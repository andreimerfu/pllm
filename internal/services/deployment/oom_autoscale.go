package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/amerfu/pllm/internal/core/models"
)

// OOMAutoscaleConfig mirrors config.DeploymentOOMAutoscaleConfig with the
// runtime shape the service consumes. Kept in-package so the service
// doesn't depend on the config package directly — the router converts.
type OOMAutoscaleConfig struct {
	Enabled            bool
	Multiplier         float64
	MaxAttempts        int
	MaxMemory          string
	ResetAfterHealthy  time.Duration
	PollInterval       time.Duration
}

// Reasonable fallbacks if the router hands us a zero value. Matches the
// viper defaults in core/config/config.go.
func (c OOMAutoscaleConfig) withDefaults() OOMAutoscaleConfig {
	if c.Multiplier <= 1.0 {
		c.Multiplier = 2.0
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 4
	}
	if c.MaxMemory == "" {
		c.MaxMemory = "4Gi"
	}
	if c.ResetAfterHealthy <= 0 {
		c.ResetAfterHealthy = time.Hour
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 30 * time.Second
	}
	return c
}

// WebhookEmitter is the minimum surface the service needs from the
// webhooks package. Matches use_cases.WebhookEmitter by design so the
// router can reuse its bridge adapter.
type WebhookEmitter interface {
	Emit(e WebhookEventEnvelope)
}

// WebhookEventEnvelope is what the service hands to the emitter.
type WebhookEventEnvelope struct {
	Type string
	Data any
}

// MetricsSink lets the service bump Prometheus counters without a hard
// dependency on the metrics package. The concrete implementation lives
// in services/deployment/metrics and is wired by the router.
type MetricsSink interface {
	IncOOMBump(namespace, workload, newLimit string)
	SetOOMAttempts(namespace, workload string, attempts int)
}

// WithOOMAutoscale wires the OOM-autoscaler config + emitter into the
// service. Safe to pass zero-value/nil — everything degrades to "disabled".
func (s *Service) WithOOMAutoscale(cfg OOMAutoscaleConfig, hooks WebhookEmitter, metrics MetricsSink) *Service {
	s.oomCfg = cfg.withDefaults()
	s.oomHooks = hooks
	s.oomMetrics = metrics
	return s
}

// oomAutoscaleEnabled resolves the tri-state override on a deployment row
// against the global config. Used from the reconciler on both kinds.
func (s *Service) oomAutoscaleEnabled(override *bool) bool {
	if override != nil {
		return *override
	}
	return s.oomCfg.Enabled
}

// baselineMemoryLimit returns the memory limit that would be applied to a
// fresh deploy — i.e. either the adapter default (from config) or the
// namespace-specific default. It is NOT the same as the deployment row's
// current override; baseline is the starting point we escalate from.
func (s *Service) baselineMemoryLimit(target models.DeploymentTargetKind) string {
	switch target {
	case models.DeploymentTargetAgent:
		if s.agentK8sDefaults.AgentMemoryLimit != "" {
			return s.agentK8sDefaults.AgentMemoryLimit
		}
		return "512Mi"
	default:
		if s.serverK8sDefaults.ServerMemoryLimit != "" {
			return s.serverK8sDefaults.ServerMemoryLimit
		}
		return "256Mi"
	}
}

// nextMemoryAfterOOM computes the memory limit for the next deploy given
// the pod's current effective limit, the configured multiplier, and a
// hard ceiling. Returns (nextQuantity, capReached).
//
// - current is what the OOM'd pod was running with (from the probe); if
//   empty, we fall back to the baseline.
// - capReached=true means "do not bump; we've hit max_memory". The caller
//   is expected to mark the workload Failed and emit cap_reached.
func nextMemoryAfterOOM(current, baseline, ceiling string, multiplier float64) (string, bool, error) {
	if current == "" {
		current = baseline
	}
	curQ, err := resource.ParseQuantity(current)
	if err != nil {
		return "", false, fmt.Errorf("parse current memory %q: %w", current, err)
	}
	ceilQ, err := resource.ParseQuantity(ceiling)
	if err != nil {
		return "", false, fmt.Errorf("parse ceiling %q: %w", ceiling, err)
	}
	// If already at/over the ceiling, cap reached.
	if curQ.Cmp(ceilQ) >= 0 {
		return "", true, nil
	}
	// Multiply. Quantity has no Mul — we work in bytes then re-quantize so
	// the string stays readable ("512Mi" not "536870912").
	cur := curQ.Value()
	next := int64(float64(cur) * multiplier)
	if next <= cur {
		// Guard against rounding putting us at or below current.
		next = cur + 1
	}
	nextQ := resource.NewQuantity(next, resource.BinarySI)
	if nextQ.Cmp(ceilQ) > 0 {
		// Clamp to the ceiling — one last bump up to the max before we
		// refuse further escalation.
		if curQ.Cmp(ceilQ) < 0 {
			return ceilQ.String(), false, nil
		}
		return "", true, nil
	}
	return nextQ.String(), false, nil
}

// --- Reconciler tick --------------------------------------------------------

// StartOOMReconciler launches the periodic OOM-watch ticker. Cancels when
// ctx is done. Safe to call at most once; subsequent calls return the
// existing stop func.
func (s *Service) StartOOMReconciler(ctx context.Context) {
	if !s.oomCfg.Enabled && !s.anyPerRowEnabled(ctx) {
		// Global off and no per-row overrides turning it on — skip the
		// ticker entirely, save a goroutine.
		return
	}
	interval := s.oomCfg.PollInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	s.logger.Info("deployment: OOM reconciler starting", zap.Duration("interval", interval))
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		// Kick once on startup so we don't wait a full interval.
		s.oomTick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.oomTick(ctx)
			}
		}
	}()
}

// anyPerRowEnabled returns true if any deployment row has the per-row
// override set to true — used so the ticker still runs when the global
// switch is off but an operator has opted in for a specific workload.
func (s *Service) anyPerRowEnabled(ctx context.Context) bool {
	var n int64
	s.db.WithContext(ctx).Model(&models.Deployment{}).
		Where("oom_autoscale_enabled IS TRUE").Count(&n)
	if n > 0 {
		return true
	}
	s.db.WithContext(ctx).Model(&models.AgentDeployment{}).
		Where("oom_autoscale_enabled IS TRUE").Count(&n)
	return n > 0
}

// oomTick walks every deployment + agent deployment row, checks its pod
// for OOM, and bumps memory when applicable. Non-OOM rows get a healthy-
// reset check. Errors log but don't stop the loop.
func (s *Service) oomTick(ctx context.Context) {
	s.oomTickServers(ctx)
	if s.agentAdapter != nil {
		s.oomTickAgents(ctx)
	}
}

func (s *Service) oomTickServers(ctx context.Context) {
	var rows []models.Deployment
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		s.logger.Warn("oom tick: list servers", zap.Error(err))
		return
	}
	for i := range rows {
		row := &rows[i]
		if !s.oomAutoscaleEnabled(row.OOMAutoscaleEnabled) {
			continue
		}
		rep, err := s.adapter.Status(ctx, row)
		if err != nil {
			s.logger.Debug("oom tick: server status", zap.String("workload", row.WorkloadName), zap.Error(err))
			continue
		}
		s.oomReconcileServer(ctx, row, rep)
	}
}

func (s *Service) oomTickAgents(ctx context.Context) {
	var rows []models.AgentDeployment
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		s.logger.Warn("oom tick: list agents", zap.Error(err))
		return
	}
	for i := range rows {
		row := &rows[i]
		if !s.oomAutoscaleEnabled(row.OOMAutoscaleEnabled) {
			continue
		}
		rep, err := s.agentAdapter.Status(ctx, row)
		if err != nil {
			s.logger.Debug("oom tick: agent status", zap.String("workload", row.WorkloadName), zap.Error(err))
			continue
		}
		s.oomReconcileAgent(ctx, row, rep)
	}
}

// oomReconcileServer applies the OOM state machine to one server row.
// Three paths:
//
//   - OOMKilled observed → bump memory, re-apply, emit event. If the
//     bump is refused (cap reached) → mark Failed + emit cap_reached.
//   - Running + healthy + past ResetAfterHealthy → clear attempts/override.
//   - Otherwise no-op.
func (s *Service) oomReconcileServer(ctx context.Context, row *models.Deployment, rep *StatusReport) {
	if rep.OOMKilled {
		s.handleOOMBumpServer(ctx, row, rep)
		return
	}
	s.maybeResetHealthyServer(ctx, row, rep)
}

func (s *Service) oomReconcileAgent(ctx context.Context, row *models.AgentDeployment, rep *StatusReport) {
	if rep.OOMKilled {
		s.handleOOMBumpAgent(ctx, row, rep)
		return
	}
	s.maybeResetHealthyAgent(ctx, row, rep)
}

// --- bump server ------------------------------------------------------------

func (s *Service) handleOOMBumpServer(ctx context.Context, row *models.Deployment, rep *StatusReport) {
	// Debounce: if the same OOM event already triggered a bump within one
	// poll interval, skip. LastTerminationState stays sticky across
	// restarts, so without this we'd hammer the bump on every tick.
	if row.OOMLastEventAt != nil && time.Since(*row.OOMLastEventAt) < s.oomCfg.PollInterval {
		return
	}
	baseline := s.baselineMemoryLimit(models.DeploymentTargetServer)
	current := rep.TerminatedMemoryLimit
	if current == "" {
		current = row.OOMMemoryOverride
	}
	if current == "" {
		current = baseline
	}
	if row.OOMAttempts >= s.oomCfg.MaxAttempts {
		s.emitCapReached(ctx, row.ID, models.DeploymentTargetServer, row.WorkloadName, row.Namespace, current, rep)
		s.updateStatus(ctx, row.ID, models.DeploymentStatusFailed,
			fmt.Sprintf("OOMKilled: max_attempts (%d) reached at %s", s.oomCfg.MaxAttempts, current))
		return
	}
	next, capped, err := nextMemoryAfterOOM(current, baseline, s.oomCfg.MaxMemory, s.oomCfg.Multiplier)
	if err != nil {
		s.logger.Warn("oom bump: compute next", zap.Error(err), zap.String("workload", row.WorkloadName))
		return
	}
	if capped {
		s.emitCapReached(ctx, row.ID, models.DeploymentTargetServer, row.WorkloadName, row.Namespace, current, rep)
		s.updateStatus(ctx, row.ID, models.DeploymentStatusFailed,
			fmt.Sprintf("OOMKilled: memory ceiling (%s) reached", s.oomCfg.MaxMemory))
		return
	}

	now := time.Now()
	row.OOMMemoryOverride = next
	row.OOMAttempts++
	row.OOMLastEventAt = &now
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		s.logger.Warn("oom bump: persist server row", zap.Error(err))
		return
	}

	// Re-apply from the associated registry row. DeployFromServer will
	// read the updated override off the row and pass it to the adapter.
	var server models.RegistryServer
	if err := s.db.WithContext(ctx).First(&server, "id = ?", row.RegistryServerID).Error; err != nil {
		s.logger.Warn("oom bump: registry row missing", zap.Error(err))
		return
	}
	if _, err := s.DeployFromServer(ctx, &server, row.Namespace); err != nil {
		s.logger.Warn("oom bump: re-apply server", zap.String("workload", row.WorkloadName), zap.Error(err))
		return
	}

	s.emitBumped(ctx, row.ID, models.DeploymentTargetServer, row.WorkloadName, row.Namespace,
		current, next, row.OOMAttempts, rep)
}

// --- bump agent -------------------------------------------------------------

func (s *Service) handleOOMBumpAgent(ctx context.Context, row *models.AgentDeployment, rep *StatusReport) {
	if row.OOMLastEventAt != nil && time.Since(*row.OOMLastEventAt) < s.oomCfg.PollInterval {
		return
	}
	baseline := s.baselineMemoryLimit(models.DeploymentTargetAgent)
	current := rep.TerminatedMemoryLimit
	if current == "" {
		current = row.OOMMemoryOverride
	}
	if current == "" {
		current = baseline
	}
	if row.OOMAttempts >= s.oomCfg.MaxAttempts {
		s.emitCapReached(ctx, row.ID, models.DeploymentTargetAgent, row.WorkloadName, row.Namespace, current, rep)
		s.updateAgentStatus(ctx, row.ID, models.DeploymentStatusFailed,
			fmt.Sprintf("OOMKilled: max_attempts (%d) reached at %s", s.oomCfg.MaxAttempts, current))
		return
	}
	next, capped, err := nextMemoryAfterOOM(current, baseline, s.oomCfg.MaxMemory, s.oomCfg.Multiplier)
	if err != nil {
		s.logger.Warn("oom bump: compute next", zap.Error(err), zap.String("workload", row.WorkloadName))
		return
	}
	if capped {
		s.emitCapReached(ctx, row.ID, models.DeploymentTargetAgent, row.WorkloadName, row.Namespace, current, rep)
		s.updateAgentStatus(ctx, row.ID, models.DeploymentStatusFailed,
			fmt.Sprintf("OOMKilled: memory ceiling (%s) reached", s.oomCfg.MaxMemory))
		return
	}

	now := time.Now()
	row.OOMMemoryOverride = next
	row.OOMAttempts++
	row.OOMLastEventAt = &now
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		s.logger.Warn("oom bump: persist agent row", zap.Error(err))
		return
	}

	// Re-apply via a fresh DeployAgent call wouldn't work here because that
	// path re-provisions a scoped key — we just want to update the pod
	// spec with the new memory limit. Call the adapter directly with the
	// existing agent fields instead.
	if err := s.reapplyAgentWithOverride(ctx, row); err != nil {
		s.logger.Warn("oom bump: re-apply agent", zap.String("workload", row.WorkloadName), zap.Error(err))
		return
	}
	s.emitBumped(ctx, row.ID, models.DeploymentTargetAgent, row.WorkloadName, row.Namespace,
		current, next, row.OOMAttempts, rep)
}

// reapplyAgentWithOverride calls the adapter's Deploy directly to update
// an existing agent workload's spec with the row's current OOMMemoryOverride.
// Reuses stored agent metadata; does NOT re-provision the scoped key.
func (s *Service) reapplyAgentWithOverride(ctx context.Context, row *models.AgentDeployment) error {
	var agent models.RegistryAgent
	if err := s.db.WithContext(ctx).First(&agent, "id = ?", row.RegistryAgentID).Error; err != nil {
		return fmt.Errorf("resolve agent: %w", err)
	}
	image, err := firstOCIPackage(agent.Packages)
	if err != nil {
		return fmt.Errorf("agent OCI package missing: %w", err)
	}

	env := map[string]string{}
	for k, v := range s.agentEnvDefaults {
		env[k] = v
	}
	var uc models.UseCase
	if err := s.db.WithContext(ctx).First(&uc, "id = ?", row.UseCaseID).Error; err == nil {
		env["PLLM_USE_CASE"] = uc.Name
	}
	env["PLLM_AGENT_NAME"] = agent.Name

	_, err = s.agentAdapter.Deploy(ctx, &AgentRequest{
		Namespace:       row.Namespace,
		WorkloadName:    row.WorkloadName,
		DisplayName:     nonEmpty(agent.Title, agent.Name),
		ResourceName:    agent.Name,
		ResourceVersion: agent.Version,
		Image:           image,
		UseCaseID:       row.UseCaseID.String(),
		SecretName:      row.SecretName,
		Env:             env,
		Replicas:        row.Replicas,
		MemoryOverride:  row.OOMMemoryOverride,
	})
	return err
}

// --- reset on sustained health ----------------------------------------------

func (s *Service) maybeResetHealthyServer(ctx context.Context, row *models.Deployment, rep *StatusReport) {
	if rep.Status != models.DeploymentStatusRunning || !rep.Healthy {
		return
	}
	now := time.Now()
	if row.LastHealthyAt == nil {
		row.LastHealthyAt = &now
		_ = s.db.WithContext(ctx).Save(row).Error
		return
	}
	// No override / attempts → nothing to clear.
	if row.OOMAttempts == 0 && row.OOMMemoryOverride == "" {
		return
	}
	if time.Since(*row.LastHealthyAt) < s.oomCfg.ResetAfterHealthy {
		return
	}
	s.logger.Info("oom reset: server sustained healthy, clearing override",
		zap.String("workload", row.WorkloadName),
		zap.Int("attempts", row.OOMAttempts),
		zap.String("previous_override", row.OOMMemoryOverride))
	row.OOMAttempts = 0
	row.OOMMemoryOverride = ""
	row.OOMLastEventAt = nil
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		s.logger.Warn("oom reset: persist server", zap.Error(err))
	}
	if s.oomMetrics != nil {
		s.oomMetrics.SetOOMAttempts(row.Namespace, row.WorkloadName, 0)
	}
}

func (s *Service) maybeResetHealthyAgent(ctx context.Context, row *models.AgentDeployment, rep *StatusReport) {
	if rep.Status != models.DeploymentStatusRunning || !rep.Healthy {
		return
	}
	now := time.Now()
	if row.LastHealthyAt == nil {
		row.LastHealthyAt = &now
		_ = s.db.WithContext(ctx).Save(row).Error
		return
	}
	if row.OOMAttempts == 0 && row.OOMMemoryOverride == "" {
		return
	}
	if time.Since(*row.LastHealthyAt) < s.oomCfg.ResetAfterHealthy {
		return
	}
	s.logger.Info("oom reset: agent sustained healthy, clearing override",
		zap.String("workload", row.WorkloadName),
		zap.Int("attempts", row.OOMAttempts),
		zap.String("previous_override", row.OOMMemoryOverride))
	row.OOMAttempts = 0
	row.OOMMemoryOverride = ""
	row.OOMLastEventAt = nil
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		s.logger.Warn("oom reset: persist agent", zap.Error(err))
	}
	if s.oomMetrics != nil {
		s.oomMetrics.SetOOMAttempts(row.Namespace, row.WorkloadName, 0)
	}
}

// --- event recording --------------------------------------------------------

// recordEvent writes one DeploymentEvent row. Never blocks the caller on
// write failure — the audit row is best-effort.
func (s *Service) recordEvent(ctx context.Context, kind models.DeploymentEventKind,
	deploymentID uuid.UUID, target models.DeploymentTargetKind,
	workload, namespace, reason string, data map[string]any) {
	var payload datatypes.JSON
	if data != nil {
		raw, err := json.Marshal(data)
		if err == nil {
			payload = raw
		}
	}
	ev := &models.DeploymentEvent{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		DeploymentID: deploymentID,
		TargetKind:   target,
		WorkloadName: workload,
		Namespace:    namespace,
		Kind:         kind,
		Reason:       reason,
		Data:         payload,
	}
	if err := s.db.WithContext(ctx).Create(ev).Error; err != nil {
		s.logger.Warn("oom event: persist", zap.Error(err))
	}
}

// emitBumped records + fires the "auto-bumped" event on all three surfaces:
// audit table, webhook, Prometheus counter.
func (s *Service) emitBumped(ctx context.Context,
	id uuid.UUID, target models.DeploymentTargetKind,
	workload, namespace, oldLimit, newLimit string, attempts int,
	rep *StatusReport) {
	data := map[string]any{
		"old_limit":    oldLimit,
		"new_limit":    newLimit,
		"attempts":     attempts,
		"max_attempts": s.oomCfg.MaxAttempts,
		"pod_name":     rep.PodName,
		"exit_code":    rep.ExitCode,
	}
	reason := fmt.Sprintf("OOMKilled — bumped memory %s → %s (attempt %d/%d)",
		oldLimit, newLimit, attempts, s.oomCfg.MaxAttempts)
	s.recordEvent(ctx, models.DeploymentEventKindOOMBumped, id, target, workload, namespace, reason, data)
	if s.oomHooks != nil {
		envelope := WebhookEventEnvelope{
			Type: string(models.WebhookEventDeploymentOOMBumped),
			Data: map[string]any{
				"deployment_id": id,
				"target_kind":   target,
				"workload_name": workload,
				"namespace":     namespace,
				"old_limit":     oldLimit,
				"new_limit":     newLimit,
				"attempts":      attempts,
				"max_attempts":  s.oomCfg.MaxAttempts,
				"pod_name":      rep.PodName,
				"exit_code":     rep.ExitCode,
			},
		}
		s.oomHooks.Emit(envelope)
	}
	if s.oomMetrics != nil {
		s.oomMetrics.IncOOMBump(namespace, workload, newLimit)
		s.oomMetrics.SetOOMAttempts(namespace, workload, attempts)
	}
}

// emitCapReached records + fires the "ceiling reached" event. Workload
// is about to be marked Failed by the caller.
func (s *Service) emitCapReached(ctx context.Context,
	id uuid.UUID, target models.DeploymentTargetKind,
	workload, namespace, currentLimit string, rep *StatusReport) {
	data := map[string]any{
		"current_limit": currentLimit,
		"max_memory":    s.oomCfg.MaxMemory,
		"max_attempts":  s.oomCfg.MaxAttempts,
		"pod_name":      rep.PodName,
		"exit_code":     rep.ExitCode,
	}
	reason := fmt.Sprintf("OOMKilled — no further auto-scale; at %s, ceiling %s", currentLimit, s.oomCfg.MaxMemory)
	s.recordEvent(ctx, models.DeploymentEventKindOOMCapReached, id, target, workload, namespace, reason, data)
	if s.oomHooks != nil {
		s.oomHooks.Emit(WebhookEventEnvelope{
			Type: string(models.WebhookEventDeploymentOOMCapReached),
			Data: map[string]any{
				"deployment_id": id,
				"target_kind":   target,
				"workload_name": workload,
				"namespace":     namespace,
				"current_limit": currentLimit,
				"max_memory":    s.oomCfg.MaxMemory,
				"max_attempts":  s.oomCfg.MaxAttempts,
				"pod_name":      rep.PodName,
				"exit_code":     rep.ExitCode,
			},
		})
	}
}

// --- operator actions -------------------------------------------------------

// ManualBumpServer performs one escalation step outside the auto loop.
// Respects the ceiling + attempt cap. Useful when operators know an OOM
// is imminent.
func (s *Service) ManualBumpServer(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	row, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	baseline := s.baselineMemoryLimit(models.DeploymentTargetServer)
	current := row.OOMMemoryOverride
	if current == "" {
		current = baseline
	}
	next, capped, err := nextMemoryAfterOOM(current, baseline, s.oomCfg.MaxMemory, s.oomCfg.Multiplier)
	if err != nil {
		return nil, err
	}
	if capped {
		return nil, errors.New("already at memory ceiling")
	}
	now := time.Now()
	row.OOMMemoryOverride = next
	row.OOMAttempts++
	row.OOMLastEventAt = &now
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	s.recordEvent(ctx, models.DeploymentEventKindOOMManualBump, row.ID,
		models.DeploymentTargetServer, row.WorkloadName, row.Namespace,
		fmt.Sprintf("manual bump %s → %s", current, next),
		map[string]any{"old_limit": current, "new_limit": next, "attempts": row.OOMAttempts})
	var server models.RegistryServer
	if err := s.db.WithContext(ctx).First(&server, "id = ?", row.RegistryServerID).Error; err != nil {
		return row, nil
	}
	if _, err := s.DeployFromServer(ctx, &server, row.Namespace); err != nil {
		return nil, err
	}
	return s.Get(ctx, row.ID)
}

// ManualBumpAgent — same idea for agent deployments.
func (s *Service) ManualBumpAgent(ctx context.Context, id uuid.UUID) (*models.AgentDeployment, error) {
	if s.agentAdapter == nil {
		return nil, ErrAgentNotConfigured
	}
	row, err := s.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	baseline := s.baselineMemoryLimit(models.DeploymentTargetAgent)
	current := row.OOMMemoryOverride
	if current == "" {
		current = baseline
	}
	next, capped, err := nextMemoryAfterOOM(current, baseline, s.oomCfg.MaxMemory, s.oomCfg.Multiplier)
	if err != nil {
		return nil, err
	}
	if capped {
		return nil, errors.New("already at memory ceiling")
	}
	now := time.Now()
	row.OOMMemoryOverride = next
	row.OOMAttempts++
	row.OOMLastEventAt = &now
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	s.recordEvent(ctx, models.DeploymentEventKindOOMManualBump, row.ID,
		models.DeploymentTargetAgent, row.WorkloadName, row.Namespace,
		fmt.Sprintf("manual bump %s → %s", current, next),
		map[string]any{"old_limit": current, "new_limit": next, "attempts": row.OOMAttempts})
	if err := s.reapplyAgentWithOverride(ctx, row); err != nil {
		return nil, err
	}
	return s.GetAgent(ctx, row.ID)
}

// ResetServerOOM clears the override + attempt counter and re-applies.
func (s *Service) ResetServerOOM(ctx context.Context, id uuid.UUID) (*models.Deployment, error) {
	row, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	previous := row.OOMMemoryOverride
	row.OOMMemoryOverride = ""
	row.OOMAttempts = 0
	row.OOMLastEventAt = nil
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	s.recordEvent(ctx, models.DeploymentEventKindOOMReset, row.ID,
		models.DeploymentTargetServer, row.WorkloadName, row.Namespace,
		"operator reset OOM override",
		map[string]any{"previous_override": previous})
	var server models.RegistryServer
	if err := s.db.WithContext(ctx).First(&server, "id = ?", row.RegistryServerID).Error; err != nil {
		return row, nil
	}
	if _, err := s.DeployFromServer(ctx, &server, row.Namespace); err != nil {
		return nil, err
	}
	return s.Get(ctx, row.ID)
}

// ResetAgentOOM — same idea for agent deployments.
func (s *Service) ResetAgentOOM(ctx context.Context, id uuid.UUID) (*models.AgentDeployment, error) {
	if s.agentAdapter == nil {
		return nil, ErrAgentNotConfigured
	}
	row, err := s.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	previous := row.OOMMemoryOverride
	row.OOMMemoryOverride = ""
	row.OOMAttempts = 0
	row.OOMLastEventAt = nil
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	s.recordEvent(ctx, models.DeploymentEventKindOOMReset, row.ID,
		models.DeploymentTargetAgent, row.WorkloadName, row.Namespace,
		"operator reset OOM override",
		map[string]any{"previous_override": previous})
	if err := s.reapplyAgentWithOverride(ctx, row); err != nil {
		return nil, err
	}
	return s.GetAgent(ctx, row.ID)
}

// SetOOMAutoscaleEnabled writes the tri-state override for a server row.
// enabled=nil → inherit.
func (s *Service) SetOOMAutoscaleEnabled(ctx context.Context, id uuid.UUID, enabled *bool) error {
	return s.db.WithContext(ctx).Model(&models.Deployment{}).
		Where("id = ?", id).
		Update("oom_autoscale_enabled", enabled).Error
}

// SetAgentOOMAutoscaleEnabled — same for agent rows.
func (s *Service) SetAgentOOMAutoscaleEnabled(ctx context.Context, id uuid.UUID, enabled *bool) error {
	return s.db.WithContext(ctx).Model(&models.AgentDeployment{}).
		Where("id = ?", id).
		Update("oom_autoscale_enabled", enabled).Error
}

// --- query helpers ----------------------------------------------------------

// EventsFilter is the query surface for the audit log.
type EventsFilter struct {
	DeploymentID *uuid.UUID
	TargetKind   models.DeploymentTargetKind
	Kind         models.DeploymentEventKind
	Since        *time.Time
	Limit        int
	Offset       int
}

// ListEvents returns the audit rows matching the filter, newest first.
func (s *Service) ListEvents(ctx context.Context, f EventsFilter) ([]models.DeploymentEvent, error) {
	q := s.db.WithContext(ctx).Model(&models.DeploymentEvent{}).Order("created_at desc")
	if f.DeploymentID != nil {
		q = q.Where("deployment_id = ?", *f.DeploymentID)
	}
	if f.TargetKind != "" {
		q = q.Where("target_kind = ?", f.TargetKind)
	}
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
	if f.Since != nil {
		q = q.Where("created_at >= ?", *f.Since)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q = q.Limit(limit)
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	var out []models.DeploymentEvent
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

