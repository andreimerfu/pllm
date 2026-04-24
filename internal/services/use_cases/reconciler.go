package use_cases

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// Reconciler walks a use case's dependency graph and computes the union
// of declared data_classifications across every reachable component.
// Result is persisted on UseCase.EffectiveClassifications.
//
// Graph traversal rules:
//   - A UseCase owns edges with OwnerKind="use_case"; targets are servers,
//     agents, skills, prompts, LLMs.
//   - An Agent owns edges (composition): agent → agent, agent → server,
//     etc. We follow those transitively up to a depth cap.
//   - LLMs have no outgoing edges in the registry today, but their own
//     declared classifications on UserModel are picked up at the leaf.
//
// Depth is capped at 6 by default — production graphs rarely exceed
// 4 levels; the cap is a safeguard against cycles in user-authored data.
type Reconciler struct {
	db       *gorm.DB
	logger   *zap.Logger
	maxDepth int
}

// NewReconciler builds a Reconciler. maxDepth <= 0 applies the default.
func NewReconciler(db *gorm.DB, logger *zap.Logger) *Reconciler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Reconciler{db: db, logger: logger, maxDepth: 6}
}

// ComputeOne walks the graph for a single use case and persists the
// resulting effective classifications + last_graph_computed_at timestamp.
func (r *Reconciler) ComputeOne(ctx context.Context, useCaseID uuid.UUID) error {
	var uc models.UseCase
	if err := r.db.WithContext(ctx).First(&uc, "id = ?", useCaseID).Error; err != nil {
		return err
	}

	set := map[string]struct{}{}
	// Seed with the use case's own declared classifications.
	for _, c := range uc.DataClassifications {
		set[c] = struct{}{}
	}

	// BFS through dependency edges.
	visited := map[string]struct{}{
		nodeKey(models.RegistryKindUseCase, uc.ID.String()): {},
	}
	type frontierItem struct {
		kind  models.RegistryKind
		id    string // owner ID for registry rows, or resolved name for LLMs (no id)
		depth int
	}
	frontier := []frontierItem{}

	// Enqueue direct refs of the use case.
	refs, err := r.loadOutgoingRefs(ctx, models.RegistryKindUseCase, uc.ID)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err := r.absorbTargetClassifications(ctx, ref, set, visited); err != nil {
			return err
		}
		if ref.TargetKind == models.RegistryKindAgent {
			if id := r.resolveAgentID(ctx, ref.TargetName, ref.TargetVersion); id != uuid.Nil {
				key := nodeKey(models.RegistryKindAgent, id.String())
				if _, seen := visited[key]; !seen {
					visited[key] = struct{}{}
					frontier = append(frontier, frontierItem{kind: models.RegistryKindAgent, id: id.String(), depth: 1})
				}
			}
		}
	}

	for len(frontier) > 0 {
		item := frontier[0]
		frontier = frontier[1:]
		if item.depth >= r.maxDepth {
			continue
		}
		itemUUID, err := uuid.Parse(item.id)
		if err != nil {
			continue
		}
		subRefs, err := r.loadOutgoingRefs(ctx, item.kind, itemUUID)
		if err != nil {
			return err
		}
		for _, ref := range subRefs {
			if err := r.absorbTargetClassifications(ctx, ref, set, visited); err != nil {
				return err
			}
			if ref.TargetKind == models.RegistryKindAgent {
				if id := r.resolveAgentID(ctx, ref.TargetName, ref.TargetVersion); id != uuid.Nil {
					key := nodeKey(models.RegistryKindAgent, id.String())
					if _, seen := visited[key]; !seen {
						visited[key] = struct{}{}
						frontier = append(frontier, frontierItem{kind: models.RegistryKindAgent, id: id.String(), depth: item.depth + 1})
					}
				}
			}
		}
	}

	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)

	now := time.Now()
	err = r.db.WithContext(ctx).Model(&models.UseCase{}).
		Where("id = ?", useCaseID).
		Updates(map[string]any{
			"effective_classifications": pq.StringArray(out),
			"last_graph_computed_at":    &now,
		}).Error
	if err != nil {
		return err
	}
	r.logger.Debug("use-case classifications reconciled",
		zap.String("use_case", uc.Slug),
		zap.Int("count", len(out)))
	return nil
}

// ComputeAll walks every use case. Used on config boot and on a periodic
// refresh schedule. Errors on individual rows are logged and skipped.
func (r *Reconciler) ComputeAll(ctx context.Context) error {
	var rows []models.UseCase
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if err := r.ComputeOne(ctx, row.ID); err != nil {
			r.logger.Warn("reconcile use case failed",
				zap.String("slug", row.Slug),
				zap.Error(err))
		}
	}
	return nil
}

// absorbTargetClassifications resolves a ref target to its row (if any)
// and merges that row's data_classifications into the running set.
func (r *Reconciler) absorbTargetClassifications(
	ctx context.Context,
	ref models.RegistryRef,
	set map[string]struct{},
	visited map[string]struct{},
) error {
	// Skip if we already visited this exact (kind, name, version) target —
	// cheap cycle guard at the target level.
	key := nodeKey(ref.TargetKind, ref.TargetName+"@"+ref.TargetVersion)
	if _, seen := visited[key]; seen {
		return nil
	}
	visited[key] = struct{}{}

	switch ref.TargetKind {
	case models.RegistryKindServer:
		var row models.RegistryServer
		if err := r.latestVersionOrPinned(ctx, "registry_servers", ref, &row); err == nil {
			_ = row // servers don't carry classifications yet; column will be added in Phase 1.5
		}
	case models.RegistryKindAgent:
		// Agent classifications column doesn't exist yet either.
	case models.RegistryKindSkill:
		var row models.RegistrySkill
		_ = r.latestVersionOrPinned(ctx, "registry_skills", ref, &row)
	case models.RegistryKindPrompt:
		var row models.RegistryPrompt
		_ = r.latestVersionOrPinned(ctx, "registry_prompts", ref, &row)
	case models.RegistryKindLLM:
		// Resolve by model name → UserModel. UserModel.DataClassifications
		// is the field we add here; for now we look for the config value if set.
		var um models.UserModel
		err := r.db.WithContext(ctx).
			Where("model_name = ?", ref.TargetName).
			First(&um).Error
		if err == nil {
			for _, c := range um.DataClassifications {
				set[c] = struct{}{}
			}
		}
	}
	return nil
}

// latestVersionOrPinned loads the row matching TargetVersion (if set) or
// the is_latest row otherwise. Kept generic via Table + Scan.
func (r *Reconciler) latestVersionOrPinned(
	ctx context.Context,
	table string,
	ref models.RegistryRef,
	dest any,
) error {
	q := r.db.WithContext(ctx).Table(table).Where("name = ?", ref.TargetName)
	if ref.TargetVersion != "" && ref.TargetVersion != "latest" {
		q = q.Where("version = ?", ref.TargetVersion)
	} else {
		q = q.Where("is_latest = ?", true)
	}
	return q.First(dest).Error
}

// loadOutgoingRefs returns the edges where the given (kind, id) is owner.
func (r *Reconciler) loadOutgoingRefs(
	ctx context.Context,
	kind models.RegistryKind,
	id uuid.UUID,
) ([]models.RegistryRef, error) {
	var out []models.RegistryRef
	err := r.db.WithContext(ctx).
		Where("owner_kind = ? AND owner_id = ?", kind, id).
		Find(&out).Error
	return out, err
}

// resolveAgentID looks up an agent row by name (+ optional version) and
// returns its primary key. Zero UUID means "not found" — which may just
// mean the ref points at an agent that hasn't been published to pllm yet;
// we silently skip those.
func (r *Reconciler) resolveAgentID(ctx context.Context, name, version string) uuid.UUID {
	var row models.RegistryAgent
	q := r.db.WithContext(ctx).Where("name = ?", name)
	if version != "" && version != "latest" {
		q = q.Where("version = ?", version)
	} else {
		q = q.Where("is_latest = ?", true)
	}
	if err := q.First(&row).Error; err != nil {
		return uuid.Nil
	}
	return row.ID
}

func nodeKey(kind models.RegistryKind, id string) string {
	return string(kind) + ":" + id
}
