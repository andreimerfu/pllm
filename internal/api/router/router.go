package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/amerfu/pllm/internal/core/auth"
	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/api/docs"
	"github.com/amerfu/pllm/internal/api/handlers"
	"github.com/amerfu/pllm/internal/api/handlers/admin"
	a2ahandlers "github.com/amerfu/pllm/internal/api/handlers/a2a"
	deploymenthandlers "github.com/amerfu/pllm/internal/api/handlers/deployment"
	dischandlers "github.com/amerfu/pllm/internal/api/handlers/discovery"
	lifecyclehandlers "github.com/amerfu/pllm/internal/api/handlers/lifecycle"
	mcphandlers "github.com/amerfu/pllm/internal/api/handlers/mcp"
	otelhandlers "github.com/amerfu/pllm/internal/api/handlers/otel"
	registryhandlers "github.com/amerfu/pllm/internal/api/handlers/registry"
	usecasehandlers "github.com/amerfu/pllm/internal/api/handlers/use_cases"
	webhookhandlers "github.com/amerfu/pllm/internal/api/handlers/webhooks"
	"github.com/amerfu/pllm/internal/infrastructure/middleware"
	"github.com/amerfu/pllm/internal/services/monitoring/metrics"
	"github.com/amerfu/pllm/internal/services/data/budget"
	"github.com/amerfu/pllm/internal/services/data/cache"
	"github.com/amerfu/pllm/internal/services/integrations/guardrails"
	"github.com/amerfu/pllm/internal/services/integrations/key"
	coremodels "github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/services/llm/models"
	"github.com/amerfu/pllm/internal/services/llm/realtime"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
	deploymentsvc "github.com/amerfu/pllm/internal/services/deployment"
	deploymentmetrics "github.com/amerfu/pllm/internal/services/deployment/metrics"
	discoverysvc "github.com/amerfu/pllm/internal/services/discovery"
	lifecyclesvc "github.com/amerfu/pllm/internal/services/lifecycle"
	mcpgateway "github.com/amerfu/pllm/internal/services/mcp/gateway"
	mcpregistryserver "github.com/amerfu/pllm/internal/services/mcp/registryserver"
	"github.com/amerfu/pllm/internal/services/registry/enrichment"
	"github.com/amerfu/pllm/internal/services/registry/importer"
	registryservice "github.com/amerfu/pllm/internal/services/registry/service"
	usecasesvc "github.com/amerfu/pllm/internal/services/use_cases"
	"github.com/amerfu/pllm/internal/services/webhooks"
	redisService "github.com/amerfu/pllm/internal/services/data/redis"
	"github.com/amerfu/pllm/internal/services/integrations/team"
	"github.com/amerfu/pllm/internal/api/ui"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// mcpScopeAdapter bridges the internal use_cases.ScopeResolver to the
// narrow ScopeChecker interface the MCP handler expects — keeps the
// handlers package independent of use_cases for easier testing.
type mcpScopeAdapter struct {
	inner *usecasesvc.ScopeResolver
}

func (a mcpScopeAdapter) AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, serverSlug string) mcphandlers.ScopeDecision {
	d := a.inner.AllowMCPTool(ctx, useCaseID, serverSlug)
	return mcphandlers.ScopeDecision{
		Allowed:     d.Allowed,
		EnforceMode: d.EnforceMode,
		Reason:      d.Reason,
	}
}

// llmScopeAdapter does the same bridge for the chat/LLM handler.
type llmScopeAdapter struct {
	inner *usecasesvc.ScopeResolver
}

func (a llmScopeAdapter) AllowLLM(ctx context.Context, useCaseID *uuid.UUID, modelName string) handlers.LLMScopeDecision {
	d := a.inner.AllowLLM(ctx, useCaseID, modelName)
	return handlers.LLMScopeDecision{
		Allowed:     d.Allowed,
		EnforceMode: d.EnforceMode,
		Reason:      d.Reason,
	}
}

// a2aScopeAdapter bridges to the narrow ScopeChecker the a2a handler expects.
type a2aScopeAdapter struct {
	inner *usecasesvc.ScopeResolver
}

func (a a2aScopeAdapter) AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) a2ahandlers.ScopeDecision {
	d := a.inner.AllowAgent(ctx, useCaseID, agentName)
	return a2ahandlers.ScopeDecision{
		Allowed:     d.Allowed,
		EnforceMode: d.EnforceMode,
		Reason:      d.Reason,
	}
}

// allowAllScope is used when the UseCase layer is disabled: A2A handler
// still needs a ScopeChecker, but every call is accepted.
type allowAllScope struct{}

func (allowAllScope) AllowAgent(_ context.Context, _ *uuid.UUID, _ string) a2ahandlers.ScopeDecision {
	return a2ahandlers.ScopeDecision{Allowed: true}
}

// scopeEventAdapter bridges the EventRecorder to the narrow interfaces
// expected by the MCP gateway + chat handler. Concentrates the enum
// translation in one place.
type scopeEventAdapter struct {
	inner *usecasesvc.EventRecorder
}

// RecordMCPDenial implements mcphandlers.ScopeEventRecorder.
func (a scopeEventAdapter) RecordMCPDenial(
	ctx context.Context,
	useCaseID uuid.UUID,
	keyID *uuid.UUID, keyName, target, reason string, enforced bool,
	remoteAddr, requestID string,
) {
	a.write(ctx, coremodels.ScopeEventKindMCPTool, useCaseID, keyID, keyName, target, reason, enforced, remoteAddr, requestID)
}

// RecordLLMDenial implements handlers.LLMScopeEventRecorder.
func (a scopeEventAdapter) RecordLLMDenial(
	ctx context.Context,
	useCaseID uuid.UUID,
	keyID *uuid.UUID, keyName, target, reason string, enforced bool,
	remoteAddr, requestID string,
) {
	a.write(ctx, coremodels.ScopeEventKindLLM, useCaseID, keyID, keyName, target, reason, enforced, remoteAddr, requestID)
}

// RecordAgentDenial implements a2ahandlers.ScopeEventRecorder.
func (a scopeEventAdapter) RecordAgentDenial(
	ctx context.Context,
	useCaseID uuid.UUID,
	keyID *uuid.UUID, keyName, target, reason string, enforced bool,
	remoteAddr, requestID string,
) {
	a.write(ctx, coremodels.ScopeEventKindAgent, useCaseID, keyID, keyName, target, reason, enforced, remoteAddr, requestID)
}

// write is the shared body of both Record* methods; takes the already-
// translated ScopeEventKind and persists via the EventRecorder.
func (a scopeEventAdapter) write(
	ctx context.Context,
	kind coremodels.ScopeEventKind,
	useCaseID uuid.UUID,
	keyID *uuid.UUID, keyName, target, reason string, enforced bool,
	remoteAddr, requestID string,
) {
	if a.inner == nil {
		return
	}
	outcome := coremodels.ScopeOutcomeFlagged
	if enforced {
		outcome = coremodels.ScopeOutcomeDenied
	}
	a.inner.Record(ctx, usecasesvc.RecordInput{
		UseCaseID:  useCaseID,
		KeyID:      keyID,
		KeyName:    keyName,
		Kind:       kind,
		TargetName: target,
		Outcome:    outcome,
		Reason:     reason,
		RemoteAddr: remoteAddr,
		RequestID:  requestID,
	})
}

// webhookEmitterAdapter translates the narrow use_cases.WebhookEmitter
// interface into concrete webhooks.Dispatcher.Emit calls. The string
// type gets mapped to models.WebhookEventType.
type webhookEmitterAdapter struct {
	inner *webhooks.Dispatcher
}

func (a webhookEmitterAdapter) Emit(evt usecasesvc.WebhookEventEnvelope) {
	if a.inner == nil {
		return
	}
	a.inner.Emit(webhooks.Event{
		Type: coremodels.WebhookEventType(evt.Type),
		Data: evt.Data,
	})
}

// deploymentWebhookEmitter bridges the deployment package's
// WebhookEmitter interface into the same webhooks.Dispatcher. Structurally
// identical to webhookEmitterAdapter; distinct type keeps imports clean.
type deploymentWebhookEmitter struct {
	inner *webhooks.Dispatcher
}

func (a deploymentWebhookEmitter) Emit(evt deploymentsvc.WebhookEventEnvelope) {
	if a.inner == nil {
		return
	}
	a.inner.Emit(webhooks.Event{
		Type: coremodels.WebhookEventType(evt.Type),
		Data: evt.Data,
	})
}

// lifecycleWebhookAdapter bridges the lifecycle service's
// WebhookEmitter onto the shared webhooks.Dispatcher. The lifecycle
// envelope already carries a typed models.WebhookEventType, so the
// adapter is a straight-through reshape — no string parsing, no
// silent fan-out for typos.
type lifecycleWebhookAdapter struct {
	inner *webhooks.Dispatcher
}

func (a *lifecycleWebhookAdapter) Emit(env lifecyclesvc.WebhookEventEnvelope) {
	if a == nil || a.inner == nil {
		return
	}
	a.inner.Emit(webhooks.Event{
		Type: env.Type,
		Data: env.Data,
	})
}

// --- discovery bridges ----------------------------------------------------
//
// Same narrow-interface pattern as the use-case + webhook adapters above:
// the discovery package never imports use_cases / webhooks / key services
// directly; we bridge through tiny structs defined here.

type discoveryScopeBridge struct{ inner *usecasesvc.ScopeResolver }

func (b *discoveryScopeBridge) AllowLLM(ctx context.Context, ucID *uuid.UUID, model string) discoverysvc.OracleDecision {
	d := b.inner.AllowLLM(ctx, ucID, model)
	return discoverysvc.OracleDecision{Allowed: d.Allowed, Reason: d.Reason}
}

func (b *discoveryScopeBridge) AllowMCPTool(ctx context.Context, ucID *uuid.UUID, slug string) discoverysvc.OracleDecision {
	d := b.inner.AllowMCPTool(ctx, ucID, slug)
	return discoverysvc.OracleDecision{Allowed: d.Allowed, Reason: d.Reason}
}

func (b *discoveryScopeBridge) AllowAgent(ctx context.Context, ucID *uuid.UUID, agentName string) discoverysvc.OracleDecision {
	d := b.inner.AllowAgent(ctx, ucID, agentName)
	return discoverysvc.OracleDecision{Allowed: d.Allowed, Reason: d.Reason}
}

type discoveryRecorderBridge struct{ inner *usecasesvc.EventRecorder }

func (b *discoveryRecorderBridge) Record(ctx context.Context, in discoverysvc.RecordInput) {
	if b == nil || b.inner == nil {
		return
	}
	b.inner.Record(ctx, usecasesvc.RecordInput{
		UseCaseID:  in.UseCaseID,
		KeyID:      in.KeyID,
		KeyName:    in.KeyName,
		UserID:     in.UserID,
		Kind:       in.Kind,
		TargetName: in.TargetName,
		Outcome:    in.Outcome,
		Reason:     in.Reason,
	})
}

type discoveryKeyLoader struct{ db *gorm.DB }

func (l *discoveryKeyLoader) GetKey(ctx context.Context, id uuid.UUID) (*coremodels.Key, error) {
	var k coremodels.Key
	if err := l.db.WithContext(ctx).First(&k, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

type discoveryKeyRevoker struct {
	db     *gorm.DB
	logger *zap.Logger
}

func (r *discoveryKeyRevoker) Revoke(ctx context.Context, keyID uuid.UUID, reason string) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&coremodels.Key{}).
		Where("id = ? AND (revoked_at IS NULL OR is_active = TRUE)", keyID).
		Updates(map[string]any{
			"revoked_at":        &now,
			"is_active":         false,
			"revocation_reason": reason,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		r.logger.Info("key auto-revoked by discovery",
			zap.String("key_id", keyID.String()),
			zap.String("reason", reason))
	}
	return nil
}

type discoveryAgentResolver struct {
	db     *gorm.DB
	logger *zap.Logger
}

func (a *discoveryAgentResolver) PublishFromDiscovery(ctx context.Context, in discoverysvc.PublishInput) (uuid.UUID, error) {
	refs, descSuffix := translateDiscoveryRefs(in.Refs)
	description := in.Description
	if descSuffix != "" {
		description = description + "\n\n" + descSuffix
	}

	// If a soft-deleted RegistryAgent already exists at this (name, version),
	// restore it instead of trying to insert a duplicate. The unique index
	// on idx_reg_agent_name_version is not partial — soft-deleted rows still
	// hold the slot, so a fresh Upsert would fail with a constraint error.
	var existing coremodels.RegistryAgent
	err := a.db.WithContext(ctx).Unscoped().
		Where("name = ? AND version = ?", in.Name, in.Version).
		First(&existing).Error
	if err == nil && existing.DeletedAt.Valid {
		now := time.Now()
		updates := map[string]any{
			"deleted_at":   nil,
			"title":        in.Title,
			"description":  description,
			"status":       coremodels.RegistryStatusActive,
			"published_at": now,
			"updated_at":   now,
		}
		if err := a.db.WithContext(ctx).Unscoped().
			Model(&coremodels.RegistryAgent{}).
			Where("id = ?", existing.ID).
			Updates(updates).Error; err != nil {
			return uuid.Nil, err
		}
		// Replace refs wholesale on restore, same as Upsert would.
		if err := a.replaceRefs(ctx, existing.ID, refs); err != nil {
			a.logger.Warn("failed to replace refs on restored agent", zap.Error(err))
		}
		a.logger.Info("re-promoted: restored soft-deleted RegistryAgent",
			zap.String("name", in.Name),
			zap.String("version", in.Version),
			zap.Int("refs", len(refs)))
		return existing.ID, nil
	}

	svc := registryservice.NewAgentService(a.db, a.logger)
	out, err := svc.Upsert(ctx, &registryservice.UpsertInput{
		Agent: coremodels.RegistryAgent{
			Name:        in.Name,
			Version:     in.Version,
			Title:       in.Title,
			Description: description,
		},
		Refs: refs,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return out.Agent.ID, nil
}

// observedDepsLoader implements registryhandlers.ObservedDepsLoader.
// Looks up the DiscoveredAgent that was promoted into the given
// RegistryAgent, then loads its DiscoveredCalls.
type observedDepsLoader struct{ db *gorm.DB }

func (l *observedDepsLoader) LoadForAgent(ctx context.Context, agentID uuid.UUID) (*registryhandlers.ObservedDeps, error) {
	var da coremodels.DiscoveredAgent
	err := l.db.WithContext(ctx).
		Where("promoted_agent_id = ?", agentID).
		Order("last_seen_at DESC").
		First(&da).Error
	if err != nil {
		// No DiscoveredAgent points at this RegistryAgent — agent was
		// published manually or never re-promoted. Return empty.
		return &registryhandlers.ObservedDeps{Calls: []registryhandlers.ObservedCall{}}, nil
	}
	var calls []coremodels.DiscoveredCall
	if err := l.db.WithContext(ctx).
		Where("discovered_agent_id = ?", da.ID).
		Order("last_seen_at DESC").
		Find(&calls).Error; err != nil {
		return nil, err
	}
	out := &registryhandlers.ObservedDeps{
		DiscoveredAgentID: &da.ID,
		ServiceName:       da.ServiceName,
		LastSeenAt:        &da.LastSeenAt,
		FlaggedCount:      da.FlaggedCount,
		Calls:             make([]registryhandlers.ObservedCall, 0, len(calls)),
	}
	for _, c := range calls {
		out.Calls = append(out.Calls, registryhandlers.ObservedCall{
			Kind:        string(c.Kind),
			TargetName:  c.TargetName,
			Count:       c.Count,
			LastOutcome: c.LastOutcome,
			LastReason:  c.LastReason,
			FirstSeenAt: c.FirstSeenAt,
			LastSeenAt:  c.LastSeenAt,
		})
	}
	return out, nil
}

// translateDiscoveryRefs maps the discovery package's CallKindStr enum
// onto RegistryRef.TargetKind. LLM and MCP targets fit cleanly; vector
// DBs, raw HTTP, and unknowns don't have a registry kind yet, so we
// surface them as a human-readable description suffix instead of
// dropping the information entirely.
func translateDiscoveryRefs(in []discoverysvc.PublishRef) ([]coremodels.RegistryRef, string) {
	refs := make([]coremodels.RegistryRef, 0, len(in))
	var notes []string
	for _, r := range in {
		switch r.Kind {
		case discoverysvc.CallKindLLM:
			refs = append(refs, coremodels.RegistryRef{
				TargetKind: coremodels.RegistryKindLLM,
				TargetName: r.TargetName,
			})
		case discoverysvc.CallKindMCP:
			refs = append(refs, coremodels.RegistryRef{
				TargetKind: coremodels.RegistryKindServer,
				TargetName: r.TargetName,
			})
		case discoverysvc.CallKindAgent:
			refs = append(refs, coremodels.RegistryRef{
				TargetKind: coremodels.RegistryKindAgent,
				TargetName: r.TargetName,
			})
		case discoverysvc.CallKindVectorDB:
			notes = append(notes, fmt.Sprintf("vector_db: %s (×%d)", r.TargetName, r.Count))
		case discoverysvc.CallKindHTTP:
			notes = append(notes, fmt.Sprintf("http: %s (×%d)", r.TargetName, r.Count))
		default:
			notes = append(notes, fmt.Sprintf("unknown: %s (×%d)", r.TargetName, r.Count))
		}
	}
	suffix := ""
	if len(notes) > 0 {
		suffix = "Observed external dependencies (not in registry kinds):\n  - " +
			strings.Join(notes, "\n  - ")
	}
	return refs, suffix
}

// replaceRefs deletes the agent's existing RegistryRefs and inserts the
// provided set. Mirrors what AgentService.Upsert does internally.
func (a *discoveryAgentResolver) replaceRefs(ctx context.Context, agentID uuid.UUID, refs []coremodels.RegistryRef) error {
	if err := a.db.WithContext(ctx).
		Where("owner_kind = ? AND owner_id = ?", coremodels.RegistryKindAgent, agentID).
		Delete(&coremodels.RegistryRef{}).Error; err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	for i := range refs {
		refs[i].OwnerKind = coremodels.RegistryKindAgent
		refs[i].OwnerID = agentID
	}
	return a.db.WithContext(ctx).Create(&refs).Error
}

func NewRouter(cfg *config.Config, logger *zap.Logger, modelManager *models.ModelManager, db *gorm.DB, pricingManager *config.ModelPricingManager) http.Handler {
	r := chi.NewRouter()

	// Initialize Redis client
	opt, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		logger.Fatal("Failed to parse Redis URL", zap.Error(err))
	}

	// Override with explicit password and DB if provided
	if cfg.Redis.Password != "" {
		opt.Password = cfg.Redis.Password
	}
	if cfg.Redis.DB != 0 {
		opt.DB = cfg.Redis.DB
	}

	redisClient := redis.NewClient(opt)

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize auth services
	masterKeyService := auth.NewMasterKeyService(&auth.MasterKeyConfig{
		DB:          db,
		MasterKey:   cfg.Auth.MasterKey,
		JWTSecret:   []byte(cfg.JWT.SecretKey),
		JWTIssuer:   "pllm",
		TokenExpiry: 24 * time.Hour,
	})

	// Prepare Dex config if enabled
	var dexConfig *auth.DexConfig
	if cfg.Auth.Dex.Enabled {
		dexConfig = &auth.DexConfig{
			Issuer:       cfg.Auth.Dex.Issuer,
			PublicIssuer: cfg.Auth.Dex.PublicIssuer,
			ClientID:     cfg.Auth.Dex.ClientID,
			ClientSecret: cfg.Auth.Dex.ClientSecret,
			RedirectURL:  cfg.Auth.Dex.RedirectURL,
			Scopes:       cfg.Auth.Dex.Scopes,
		}
	}

	// Initialize team and key services for auth auto-provisioning
	teamService := team.NewTeamService(db)
	keyService := key.NewService(db, logger)

	authService, err := auth.NewAuthService(&auth.AuthConfig{
		DB:               db,
		DexConfig:        dexConfig,
		JWTSecret:        cfg.JWT.SecretKey,
		JWTIssuer:        "pllm",
		TokenExpiry:      cfg.JWT.AccessTokenDuration,
		MasterKeyService: masterKeyService,
		TeamService:      teamService,
		KeyService:       keyService,
	})
	if err != nil {
		logger.Fatal("Failed to initialize auth service", zap.Error(err))
	}

	// Initialize metrics service if database and Redis are available
	var metricsService *metrics.MetricsService
	var metricsEmitter *metrics.MetricEventEmitter
	if db != nil {
		metricsConfig := &metrics.MetricsServiceConfig{
			DB:                db,
			Redis:             redisClient,
			Logger:            logger,
			WorkerCount:       4,
			BatchSize:         100,
			BatchTimeout:      5 * time.Second,
			AggregateInterval: 1 * time.Minute,
			EnableMonitoring:  false,
			MonitoringPort:    8082,
		}

		metricsService, err = metrics.NewMetricsService(metricsConfig)
		if err != nil {
			logger.Warn("Failed to initialize metrics service", zap.Error(err))
		} else {
			// Start metrics service
			if err := metricsService.Start(context.Background()); err != nil {
				logger.Warn("Failed to start metrics service", zap.Error(err))
				metricsService = nil
			} else {
				metricsEmitter = metricsService.GetEmitter()
				logger.Info("Metrics service started successfully")
			}
		}
	}

	// Initialize historical metrics collector to aggregate existing usage data
	logger.Info("Checking metrics collector prerequisites",
		zap.Bool("db_exists", db != nil),
		zap.Bool("model_manager_exists", modelManager != nil))
	if db != nil && modelManager != nil {
		historicalCollector := metrics.NewMetricsCollector(db, logger, modelManager)
		historicalCollector.Start()
		logger.Info("Historical metrics collector started")
	} else {
		logger.Warn("Cannot start historical metrics collector - missing prerequisites")
	}

	// Initialize Registry services (require DB — skipped in lite mode)
	var registryHandler *registryhandlers.Handler
	var registryImportHandler *registryhandlers.ImportHandler
	var registryMCPHandler *mcphandlers.RegistryMCPHandler
	if db != nil {
		serverSvc := registryservice.NewServerService(db, logger)
		agentSvc := registryservice.NewAgentService(db, logger)
		skillSvc := registryservice.NewSkillService(db, logger)
		promptSvc := registryservice.NewPromptService(db, logger)
		registryHandler = registryhandlers.NewHandler(
			logger, serverSvc, agentSvc, skillSvc, promptSvc,
			registryservice.NewRefService(db),
		)
		// Wire observed-deps loader so the registry's Dependencies tab
		// can show runtime-discovered targets alongside declared refs.
		registryHandler.Observed = &observedDepsLoader{db: db}
		// Enrichment runner: OSV scanner only in phase 2. Started in a
		// goroutine; shuts down when context is canceled (process exit).
		runner := enrichment.NewRunner(db, logger)
		registryHandler.Enrichment = runner
		go runner.Start(context.Background())

		// Importer (npm + PyPI). Safe to init without network — sources are
		// only hit when /api/admin/registry/import is called.
		registryImportHandler = registryhandlers.NewImportHandler(
			logger, importer.NewService(serverSvc, logger))

		// Expose the registry over MCP so agents can discover artifacts
		// via tool calls — same surface, different protocol.
		registryMCPHandler = mcphandlers.NewRegistryMCPHandler(
			logger, mcpregistryserver.New(serverSvc, agentSvc, skillSvc, promptSvc))

		logger.Info("Registry services + enrichment + importer + MCP surface initialized")
	}

	// Initialize MCP Gateway manager (safe even without DB — runs with empty backend set)
	var mcpManager *mcpgateway.Manager
	if cfg.MCP.Enabled {
		mcpManager = mcpgateway.NewManager(logger, db)
		if db != nil {
			if err := mcpManager.Load(context.Background()); err != nil {
				logger.Warn("Failed to load MCP servers from DB", zap.Error(err))
			}
		}
		mcpManager.Start(context.Background())
		logger.Info("MCP Gateway initialized", zap.Bool("persistent", db != nil))
	}

	// Webhooks dispatcher — one per process. Runs a worker pool and
	// consumes events pushed by the UseCase service + scope-event
	// recorder. No-op when DB is missing.
	var webhookDispatcher *webhooks.Dispatcher
	var webhookHandler *webhookhandlers.Handler
	if db != nil {
		webhookDispatcher = webhooks.NewDispatcher(db, logger)
		webhookDispatcher.Start(context.Background())
		webhookSvc := webhooks.NewService(db, logger)
		webhookHandler = webhookhandlers.NewHandler(logger, webhookSvc, webhookDispatcher)
	}

	// Use Case service — the top-level "business activity" layer. Requires
	// DB only; no config toggle needed, always on when DB is present.
	var useCaseHandler *usecasehandlers.Handler
	var useCaseScope *usecasesvc.ScopeResolver
	var useCaseEvents *usecasesvc.EventRecorder
	if db != nil {
		ucSvc := usecasesvc.NewService(db, logger)
		if webhookDispatcher != nil {
			ucSvc = ucSvc.WithWebhooks(webhookEmitterAdapter{inner: webhookDispatcher})
		}
		useCaseHandler = usecasehandlers.NewHandler(logger, ucSvc)
		useCaseScope = ucSvc.Scope()
		useCaseEvents = ucSvc.Events()
		logger.Info("UseCase service initialized")
	}

	// Discovery service — OTel-driven agent fingerprinting + governance.
	// Requires DB + use-case scope/event surfaces (so reuses the existing
	// audit + webhook plumbing). Mounted as POST /v1/traces below; admin
	// CRUD lives under /api/admin/discovery.
	var discoveryService *discoverysvc.Service
	var otelHandler *otelhandlers.Handler
	var discoveryHandler *dischandlers.Handler
	if db != nil && cfg.Discovery.Enabled && useCaseScope != nil && useCaseEvents != nil {
		policy := discoverysvc.NewPolicy(
			logger,
			&discoveryScopeBridge{inner: useCaseScope},
			&discoveryRecorderBridge{inner: useCaseEvents},
			&discoveryKeyLoader{db: db},
		)
		threshold := cfg.Discovery.FlagThreshold
		if !cfg.Discovery.AutoRevoke {
			threshold = 0
		}
		revoker := discoverysvc.NewRevoker(
			logger,
			&discoveryKeyRevoker{db: db, logger: logger},
			threshold,
		)
		resolver := &discoveryAgentResolver{db: db, logger: logger}
		discoveryService = discoverysvc.NewService(db, logger, policy, revoker, resolver)
		otelHandler = otelhandlers.New(logger, discoveryService)
		discoveryHandler = dischandlers.New(logger, discoveryService)
		logger.Info("Discovery service initialized",
			zap.Int("flag_threshold", cfg.Discovery.FlagThreshold),
			zap.Bool("auto_revoke", cfg.Discovery.AutoRevoke))
	}

	// Lifecycle hygiene daemon. Always-on when DB is present + feature
	// enabled in config. Defaults to DryRun=true so a fresh deploy never
	// revokes anything before the operator reviews the audit log.
	var lifecycleHandler *lifecyclehandlers.Handler
	if db != nil && cfg.Lifecycle.Enabled {
		lifeCfg := lifecyclesvc.Config{
			DryRun:                cfg.Lifecycle.DryRun,
			Interval:              cfg.Lifecycle.Interval,
			InactiveKeyDays:       cfg.Lifecycle.InactiveKeyDays,
			OwnerlessKeysEnabled:  cfg.Lifecycle.OwnerlessKeysEnabled,
			RiskyServerScoreFloor: cfg.Lifecycle.RiskyServerScoreFloor,
		}
		lifeSvc := lifecyclesvc.NewService(db, logger, lifeCfg)
		lifeSvc.Register(&lifecyclesvc.InactiveKeysRule{})
		lifeSvc.Register(&lifecyclesvc.OwnerlessKeysRule{})
		lifeSvc.Register(&lifecyclesvc.RiskyServersRule{})
		if webhookDispatcher != nil {
			lifeSvc = lifeSvc.WithWebhooks(&lifecycleWebhookAdapter{inner: webhookDispatcher})
		}
		// Custom rules (CEL) load fresh each tick so UI edits take
		// effect on the next interval without process restart.
		lifeSvc = lifeSvc.WithCustomLoader(lifecyclesvc.NewDBCustomLoader(db, logger))
		lifeSvc.Start(context.Background())
		lifecycleHandler = lifecyclehandlers.NewHandler(logger, lifeSvc).
			WithCustomRules(lifecyclehandlers.NewCustomRulesHandler(logger, db))
		logger.Info("Lifecycle hygiene daemon initialized",
			zap.Bool("dry_run", lifeCfg.DryRun),
			zap.Duration("interval", lifeCfg.Interval))
	}

	// Deployment service — opt-in via config.deployment.enabled. Falls
	// back gracefully if credentials can't be loaded. Depends on mcpManager
	// so it must be initialized after MCP setup.
	var deployHandler *deploymenthandlers.Handler
	var agentDeployHandler *deploymenthandlers.AgentHandler
	if db != nil && cfg.Deployment.Enabled {
		adapter, err := deploymentsvc.NewK8sAdapter(cfg.Deployment.K8s, logger)
		if err != nil {
			logger.Warn("Deployment disabled: K8s client unavailable", zap.Error(err))
		} else {
			bridge := deploymentsvc.NewMCPBridge(db, mcpManager, logger)
			agentAdapter := deploymentsvc.NewAgentK8sAdapter(adapter.Client(), cfg.Deployment.K8s, logger)
			agentSecrets := deploymentsvc.NewAgentSecretHelper(db, adapter.Client(), logger)

			// In-cluster URL agent pods use to reach pllm itself. Defaults
			// to the canonical skaffold layout (pllm Service in the pllm
			// namespace); operators override via cfg.Agents.InternalURL
			// when pllm runs elsewhere.
			internalURL := cfg.Agents.InternalURL
			if internalURL == "" {
				internalURL = fmt.Sprintf("http://pllm.pllm.svc.cluster.local:%d", cfg.Server.Port)
			}
			agentEnv := map[string]string{
				"PLLM_BASE_URL": internalURL,
				"PLLM_MCP_URL":  internalURL + "/v1/mcp",
			}

			deploySvc := deploymentsvc.NewService(db, logger,
				adapter,
				agentAdapter,
				agentSecrets,
				bridge,
				cfg.Deployment.K8s.Namespace,
				cfg.Deployment.K8s.AgentNamespace,
				agentEnv,
			).WithMemoryBaselines(deploymentsvc.K8sMemoryBaselines{
				ServerMemoryLimit: cfg.Deployment.K8s.DefaultMemLimit,
				AgentMemoryLimit:  cfg.Deployment.K8s.DefaultMemLimit,
			}).WithOOMAutoscale(
				deploymentsvc.OOMAutoscaleConfig{
					Enabled:           cfg.Deployment.OOMAutoscale.Enabled,
					Multiplier:        cfg.Deployment.OOMAutoscale.Multiplier,
					MaxAttempts:       cfg.Deployment.OOMAutoscale.MaxAttempts,
					MaxMemory:         cfg.Deployment.OOMAutoscale.MaxMemory,
					ResetAfterHealthy: cfg.Deployment.OOMAutoscale.ResetAfterHealthy,
					PollInterval:      cfg.Deployment.OOMAutoscale.PollInterval,
				},
				deploymentWebhookEmitter{inner: webhookDispatcher},
				deploymentmetrics.NewSink(),
			)
			serverSvc := registryservice.NewServerService(db, logger)
			deployHandler = deploymenthandlers.NewHandler(logger, deploySvc, serverSvc)
			agentDeployHandler = deploymenthandlers.NewAgentHandler(logger, deploySvc)
			logger.Info("Deployment service initialized",
				zap.String("namespace", cfg.Deployment.K8s.Namespace),
				zap.String("agent_namespace", cfg.Deployment.K8s.AgentNamespace),
				zap.Bool("in_cluster", cfg.Deployment.K8s.InCluster))
			go deploySvc.ReconcileAll(context.Background())
			deploySvc.StartOOMReconciler(context.Background())
		}
	}

	// Initialize guardrails
	var guardrailsExecutor *guardrails.Executor
	guardrailsFactory := guardrails.NewFactory(cfg, logger)
	
	// Validate guardrails configuration
	if err := guardrailsFactory.ValidateConfiguration(); err != nil {
		logger.Error("Invalid guardrails configuration", zap.Error(err))
	} else {
		// Create guardrails executor
		var err error
		guardrailsExecutor, err = guardrailsFactory.CreateExecutor()
		if err != nil {
			logger.Error("Failed to create guardrails executor", zap.Error(err))
		} else {
			logger.Info("Guardrails executor initialized successfully",
				zap.Bool("enabled", cfg.Guardrails.Enabled),
				zap.Int("guardrails_count", len(cfg.Guardrails.Guardrails)))
		}
	}

	// Legacy synchronous budget/usage systems removed in favor of async Redis-based system

	// Basic middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.Logger(logger))

	// Metrics middleware - use advanced metrics if available, otherwise basic
	if metricsEmitter != nil {
		r.Use(middleware.NewAsyncMetricsMiddleware(metricsEmitter, logger).Middleware)
	} else {
		r.Use(middleware.MetricsMiddleware(logger))
	}

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.CORS.ExposedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
	}))

	// Global rate limiting
	if cfg.RateLimit.Enabled {
		rateLimitMiddleware := middleware.NewRateLimitMiddleware(cfg, logger)
		r.Use(rateLimitMiddleware.Handler)
	}

	// Caching middleware
	if cfg.Cache.Enabled {
		cacheMiddleware := middleware.NewCacheMiddleware(cfg, logger)
		r.Use(cacheMiddleware.Handler)
	}

	// Health check
	r.Get("/health", handlers.Health)
	r.Get("/ready", handlers.Ready)

	// Prometheus metrics endpoint
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// Initialize specialized handlers
	var (
		chatHandler          *handlers.ChatHandler
		messagesHandler      *handlers.MessagesHandler
		embeddingsHandler    *handlers.EmbeddingsHandler
		modelsHandler        *handlers.ModelsHandler
		filesHandler         *handlers.FilesHandler
		imagesHandler        *handlers.ImagesHandler
		audioHandler         *handlers.AudioHandler
		moderationHandler    *handlers.ModerationHandler
		adminHandler         *handlers.AdminHandler
		modelMgmtHandler     *handlers.ModelManagementHandler
		realtimeHandler      *handlers.RealtimeHandler
	)

	if metricsEmitter != nil {
		chatHandler = handlers.NewChatHandlerWithMetrics(logger, modelManager, metricsEmitter)
		messagesHandler = handlers.NewMessagesHandlerWithMetrics(logger, modelManager, metricsEmitter)
		embeddingsHandler = handlers.NewEmbeddingsHandlerWithMetrics(logger, modelManager, metricsEmitter)
		modelsHandler = handlers.NewModelsHandlerWithMetrics(logger, modelManager, pricingManager, metricsEmitter)
		filesHandler = handlers.NewFilesHandlerWithMetrics(logger, modelManager, metricsEmitter)
		imagesHandler = handlers.NewImagesHandlerWithMetrics(logger, modelManager, metricsEmitter)
		audioHandler = handlers.NewAudioHandlerWithMetrics(logger, modelManager, metricsEmitter)
		moderationHandler = handlers.NewModerationHandlerWithMetrics(logger, modelManager, metricsEmitter)
		adminHandler = handlers.NewAdminHandlerWithMetrics(logger, modelManager, metricsEmitter)
	} else {
		chatHandler = handlers.NewChatHandler(logger, modelManager)
		messagesHandler = handlers.NewMessagesHandler(logger, modelManager)
		embeddingsHandler = handlers.NewEmbeddingsHandler(logger, modelManager)
		modelsHandler = handlers.NewModelsHandler(logger, modelManager, pricingManager)
		filesHandler = handlers.NewFilesHandler(logger, modelManager)
		imagesHandler = handlers.NewImagesHandler(logger, modelManager)
		audioHandler = handlers.NewAudioHandler(logger, modelManager)
		moderationHandler = handlers.NewModerationHandler(logger, modelManager)
		adminHandler = handlers.NewAdminHandler(logger, modelManager)
	}
	modelMgmtHandler = handlers.NewModelManagementHandler(pricingManager)

	// Attach use-case scope enforcement to the chat handler. Other LLM
	// surfaces (completions, messages, embeddings) share the same handler
	// or inherit the same model resolution path, so we gate at the chat
	// entrypoint for now — expand coverage as needed.
	if useCaseScope != nil {
		chatHandler = chatHandler.WithScope(llmScopeAdapter{inner: useCaseScope})
	}
	if useCaseEvents != nil {
		chatHandler = chatHandler.WithRecorder(scopeEventAdapter{inner: useCaseEvents})
	}

	// Initialize realtime session manager and handler
	sessionConfig := &realtime.SessionConfig{
		MaxSessions:     100,
		SessionTimeout:  30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
	sessionManager := realtime.NewSessionManager(logger, db, sessionConfig)
	
	handlerConfig := &handlers.RealtimeHandlerConfig{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		HandshakeTimeout:  45 * time.Second,
		CheckOrigin:       false,
		EnableCompression: true,
	}
	realtimeHandler = handlers.NewRealtimeHandler(logger, sessionManager, modelManager, handlerConfig)
	authHandler := handlers.NewAuthHandler(logger, authService, masterKeyService, db)

	// Initialize system handler for auth config
	systemHandler := admin.NewSystemHandler(logger, db)

	// Public routes
	r.Group(func(r chi.Router) {
		r.Post("/v1/register", authHandler.Register)
		r.Post("/v1/login", authHandler.Login)
		r.Post("/v1/refresh", authHandler.RefreshToken)
		r.Get("/api/auth/config", systemHandler.GetAuthConfig) // Public auth config

		// OTLP/HTTP traces receiver — agents identify themselves via the
		// pllm.key_id resource attribute, not via the Bearer header, so
		// this endpoint is unauthenticated. Posting noise is harmless:
		// it only creates DiscoveredAgent rows the operator can ignore.
		if otelHandler != nil {
			r.Post("/v1/traces", otelHandler.Traces)
		}
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		// Authentication middleware
		authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
			Logger:           logger,
			AuthService:      authService,
			MasterKeyService: masterKeyService,
			RequireAuth:      true,
		})
		r.Use(authMiddleware.Authenticate)

		// Guardrails middleware (after auth, before budget)
		if guardrailsExecutor != nil {
			guardrailsMiddleware := middleware.NewGuardrailsMiddleware(guardrailsExecutor, logger)
			r.Use(guardrailsMiddleware.Middleware)
		}

		// Initialize pricing cache for better performance
		pricingCache := cache.NewPricingCache(redisClient, logger, pricingManager)
		
		// Load all pricing data to Redis cache on startup
		if err := pricingCache.LoadAllPricingToCache(context.Background()); err != nil {
			logger.Warn("Failed to load pricing data to cache", zap.Error(err))
		}

		// Use async budget middleware with Redis for high performance
		asyncBudgetMiddleware := middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
			Logger:         logger,
			AuthService:    authService,
			BudgetCache:    redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			EventPub:       redisService.NewEventPublisher(redisClient, logger),
			UsageQueue:     redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
			PricingManager: pricingManager,
			PricingCache:   pricingCache,
		})
		r.Use(asyncBudgetMiddleware.EnforceBudgetAsync)

		// Async system handles both budget + usage tracking via Redis

		// OpenAI-compatible endpoints
		r.Route("/v1", func(r chi.Router) {
			// Chat completions - use a custom handler that preserves Flusher
			r.HandleFunc("/chat/completions", chatHandler.ChatCompletions)

			// Anthropic Messages API format (LiteLLM compatible)
			r.HandleFunc("/messages", messagesHandler.AnthropicMessages)

			// Completions (legacy)
			r.Post("/completions", chatHandler.Completions)

			// Embeddings
			r.Post("/embeddings", embeddingsHandler.Embeddings)

			// Models
			r.Get("/models", modelsHandler.ListModels)
			r.Get("/models/{model}", modelsHandler.GetModel)
			
			// Model Management (LiteLLM-compatible)
			r.Get("/model/info", modelMgmtHandler.GetModelInfo)
			r.Post("/model/register", modelMgmtHandler.RegisterModel)
			r.Post("/model/calculate-cost", modelMgmtHandler.CalculateCost)
			r.Get("/model/{model_name}/cost", modelMgmtHandler.GetModelCost)
			r.Patch("/model/{model_name}/pricing", modelMgmtHandler.UpdateModelPricing)

			// Files (for fine-tuning, not implemented yet)
			r.Post("/files", filesHandler.UploadFile)
			r.Get("/files", filesHandler.ListFiles)
			r.Get("/files/{file_id}", filesHandler.GetFile)
			r.Delete("/files/{file_id}", filesHandler.DeleteFile)

			// Images
			r.Post("/images/generations", imagesHandler.GenerateImage)
			r.Post("/images/edits", imagesHandler.EditImage)
			r.Post("/images/variations", imagesHandler.CreateImageVariation)

			// Audio
			r.Post("/audio/transcriptions", audioHandler.CreateTranscription)
			r.Post("/audio/translations", audioHandler.CreateTranslation)
			r.Post("/audio/speech", audioHandler.CreateSpeech)

			// Moderations
			r.Post("/moderations", moderationHandler.CreateModeration)

			// Realtime API (WebSocket)
			r.Get("/realtime", realtimeHandler.ConnectRealtime)
			r.Route("/realtime/sessions", func(r chi.Router) {
				r.Post("/", realtimeHandler.CreateSession)
				r.Get("/", realtimeHandler.ListSessions)
				r.Get("/{id}", realtimeHandler.GetSession)
			})

			// MCP Gateway: streamable-HTTP endpoint for Claude Desktop / Cursor / VS Code.
			if mcpManager != nil {
				gatewayHandler := mcphandlers.NewGatewayHandler(logger, mcpManager)
				if useCaseScope != nil {
					gatewayHandler = gatewayHandler.WithScope(mcpScopeAdapter{inner: useCaseScope})
				}
				if useCaseEvents != nil {
					gatewayHandler = gatewayHandler.WithRecorder(scopeEventAdapter{inner: useCaseEvents})
				}
				r.Handle("/mcp", gatewayHandler)
			}

			// Registry exposed as an MCP server — agents can call list_servers,
			// get_agent, etc. as tool calls instead of the REST API.
			if registryMCPHandler != nil {
				r.Handle("/mcp/registry", registryMCPHandler)
			}

			// A2A: external invocation of deployed agents.
			if db != nil {
				a2aManager := a2asvc.NewManager(db)
				a2aProxy := a2asvc.NewProxy(cfg.Agents.InvokeTimeout)
				var scopeChk a2ahandlers.ScopeChecker
				var evRec a2ahandlers.ScopeEventRecorder
				if useCaseScope != nil {
					scopeChk = a2aScopeAdapter{inner: useCaseScope}
				} else {
					scopeChk = allowAllScope{}
				}
				if useCaseEvents != nil {
					evRec = scopeEventAdapter{inner: useCaseEvents}
				}
				gw := a2ahandlers.NewGatewayHandler(logger, a2aManager, a2aProxy, scopeChk, evRec)
				card := a2ahandlers.NewCardHandler(logger, a2aManager, db, cfg.Agents.PublicURL)

				r.Post("/agents/{slug}/invoke", gw.InvokePlain)
				r.Post("/a2a/{slug}", gw.JSONRPC)
				r.Get("/a2a/{slug}/.well-known/agent.json", card.Card)
			}

			// Registry: read-only catalog endpoints.
			if registryHandler != nil {
				r.Route("/registry", func(rr chi.Router) {
					rr.Get("/servers", registryHandler.ListServers)
					rr.Get("/servers/{name}", registryHandler.GetServer)
					rr.Get("/servers/{name}/versions", registryHandler.ListServerVersions)
					rr.Get("/servers/{name}/versions/{version}", registryHandler.GetServerVersion)
					rr.Get("/agents", registryHandler.ListAgents)
					rr.Get("/agents/{name}", registryHandler.GetAgent)
					rr.Get("/agents/{name}/versions", registryHandler.ListAgentVersions)
					rr.Get("/agents/{name}/versions/{version}", registryHandler.GetAgentVersion)
					rr.Get("/agents/{name}/versions/{version}/observed", registryHandler.GetAgentObservedDeps)
					rr.Get("/skills", registryHandler.ListSkills)
					rr.Get("/skills/{name}", registryHandler.GetSkill)
					rr.Get("/skills/{name}/versions", registryHandler.ListSkillVersions)
					rr.Get("/skills/{name}/versions/{version}", registryHandler.GetSkillVersion)
					rr.Get("/prompts", registryHandler.ListPrompts)
					rr.Get("/prompts/{name}", registryHandler.GetPrompt)
					rr.Get("/prompts/{name}/versions", registryHandler.ListPromptVersions)
					rr.Get("/prompts/{name}/versions/{version}", registryHandler.GetPromptVersion)
					rr.Get("/refs", registryHandler.ListDependents)
				})
			}
		})

		// User management
		r.Route("/v1/user", func(r chi.Router) {
			r.Get("/profile", authHandler.GetProfile)
			r.Put("/profile", authHandler.UpdateProfile)
			r.Put("/password", authHandler.ChangePassword)

			// API Keys
			r.Get("/keys", authHandler.ListAPIKeys)
			r.Post("/keys", authHandler.CreateAPIKey)
			r.Delete("/keys/{key_id}", authHandler.DeleteAPIKey)

			// Usage
			r.Get("/usage", authHandler.GetUsage)
			r.Get("/usage/daily", authHandler.GetDailyUsage)
			r.Get("/usage/monthly", authHandler.GetMonthlyUsage)
			r.Get("/budget", authHandler.GetBudgetStatus)
			r.Get("/teams", authHandler.GetUserTeams)

		})

		// Admin routes for monitoring
		r.Route("/v1/admin", func(r chi.Router) {
			// Model performance statistics
			r.Get("/models/stats", adminHandler.ModelStats)
		})
	})

	// API Key authentication routes
	r.Group(func(r chi.Router) {
		// Authentication middleware
		authMiddleware := middleware.NewAuthMiddleware(&middleware.AuthConfig{
			Logger:           logger,
			AuthService:      authService,
			MasterKeyService: masterKeyService,
			RequireAuth:      true,
		})
		r.Use(authMiddleware.Authenticate)

		// Guardrails middleware (after auth, before budget)
		if guardrailsExecutor != nil {
			guardrailsMiddleware := middleware.NewGuardrailsMiddleware(guardrailsExecutor, logger)
			r.Use(guardrailsMiddleware.Middleware)
		}

		// Initialize pricing cache for better performance
		pricingCache := cache.NewPricingCache(redisClient, logger, pricingManager)
		
		// Load all pricing data to Redis cache on startup
		if err := pricingCache.LoadAllPricingToCache(context.Background()); err != nil {
			logger.Warn("Failed to load pricing data to cache", zap.Error(err))
		}

		// Use async budget middleware with Redis for high performance
		asyncBudgetMiddleware := middleware.NewAsyncBudgetMiddleware(&middleware.AsyncBudgetConfig{
			Logger:         logger,
			AuthService:    authService,
			BudgetCache:    redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			EventPub:       redisService.NewEventPublisher(redisClient, logger),
			UsageQueue:     redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
			PricingManager: pricingManager,
			PricingCache:   pricingCache,
		})
		r.Use(asyncBudgetMiddleware.EnforceBudgetAsync)

		// Async system handles both budget + usage tracking via Redis

		// OpenAI-compatible endpoints with API key
		r.Route("/api/v1", func(r chi.Router) {
			// Chat completions
			r.Post("/chat/completions", chatHandler.ChatCompletions)

			// Anthropic Messages API format (LiteLLM compatible)
			r.Post("/messages", messagesHandler.AnthropicMessages)

			// Completions (legacy)
			r.Post("/completions", chatHandler.Completions)

			// Embeddings
			r.Post("/embeddings", embeddingsHandler.Embeddings)

			// Models
			r.Get("/models", modelsHandler.ListModels)
			r.Get("/models/{model}", modelsHandler.GetModel)

			// Images
			r.Post("/images/generations", imagesHandler.GenerateImage)

			// Audio
			r.Post("/audio/transcriptions", audioHandler.CreateTranscription)
			r.Post("/audio/translations", audioHandler.CreateTranslation)
			r.Post("/audio/speech", audioHandler.CreateSpeech)

			// Moderations
			r.Post("/moderations", moderationHandler.CreateModeration)

			// Realtime API (WebSocket) - authenticated
			r.Get("/realtime", realtimeHandler.ConnectRealtime)
			r.Route("/realtime/sessions", func(r chi.Router) {
				r.Post("/", realtimeHandler.CreateSession)
				r.Get("/", realtimeHandler.ListSessions)
				r.Get("/{id}", realtimeHandler.GetSession)
			})
		})
	})

	// Admin routes - mount if database is available
	if db != nil {
		// Create unified budget service using the existing Redis components
		budgetService := budget.NewUnifiedService(&budget.UnifiedServiceConfig{
			DB:          db,
			Logger:      logger,
			BudgetCache: redisService.NewBudgetCache(redisClient, logger, 5*time.Minute),
			UsageQueue: redisService.NewUsageQueue(&redisService.UsageQueueConfig{
				Client:     redisClient,
				Logger:     logger,
				QueueName:  "usage_processing_queue",
				BatchSize:  50,
				MaxRetries: 3,
			}),
			EventPub: redisService.NewEventPublisher(redisClient, logger),
		})

		// Create admin sub-router configuration
		adminConfig := &AdminRouterConfig{
			Config:              cfg,
			Logger:              logger,
			DB:                  db,
			AuthService:         authService,
			MasterKeyService:    masterKeyService,
			ModelManager:        modelManager,
			BudgetService:       budgetService,
			GuardrailsExecutor:    guardrailsExecutor,
			MCPManager:            mcpManager,
			RegistryHandler:       registryHandler,
			RegistryImportHandler: registryImportHandler,
			DeploymentHandler:     deployHandler,
			AgentDeployHandler:    agentDeployHandler,
			UseCaseHandler:        useCaseHandler,
			WebhookHandler:        webhookHandler,
			LifecycleHandler:      lifecycleHandler,
			DiscoveryHandler:      discoveryHandler,
		}

		// Mount admin routes at /api/admin
		adminSubRouter := NewAdminSubRouter(adminConfig)
		r.Mount("/api/admin", adminSubRouter)

		logger.Info("Admin routes mounted at /api/admin")
	}

	// Documentation routes
	docsHandler, err := docs.NewHandler(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize docs handler", zap.Error(err))
	} else if docsHandler.IsEnabled() {
		// Mount docs at /docs with a wildcard to catch all subpaths
		r.Mount("/docs", http.StripPrefix("/docs", docsHandler))

		// Also handle /docs without trailing slash
		r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
		})

		logger.Info("Documentation routes enabled")
	}

	// UI routes (if enabled)
	uiHandler, err := ui.NewHandler(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize UI handler", zap.Error(err))
	} else if uiHandler.IsEnabled() {
		// Mount UI at /ui with a wildcard to catch all subpaths
		r.Mount("/ui", http.StripPrefix("/ui", uiHandler))

		// Also handle /ui without trailing slash
		r.Get("/ui", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
		})

		// Redirect root to UI
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusFound)
		})

		logger.Info("UI routes enabled")
	}

	// Static file serving for uploads
	r.Get("/files/{fileID}", filesHandler.GetFile)

	// Not found handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"error": {"message": "Not found", "type": "invalid_request_error", "code": "not_found"}}`)); err != nil {
			logger.Error("Failed to write 404 response", zap.Error(err))
		}
	})

	return r
}
