package catalog

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
	regservice "github.com/amerfu/pllm/internal/services/registry/service"
	ucservice "github.com/amerfu/pllm/internal/services/use_cases"
)

// pageSize is the per-query page used when gathering the full catalog so a
// large registry is never silently truncated.
const pageSize = 200

// RegistryResourceLister adapts the four per-kind registry services to the
// ResourceLister interface, returning all latest rows.
type RegistryResourceLister struct {
	servers *regservice.ServerService
	agents  *regservice.AgentService
	skills  *regservice.SkillService
	prompts *regservice.PromptService
}

// NewRegistryResourceLister wires the resource adapter.
func NewRegistryResourceLister(servers *regservice.ServerService, agents *regservice.AgentService, skills *regservice.SkillService, prompts *regservice.PromptService) *RegistryResourceLister {
	return &RegistryResourceLister{servers: servers, agents: agents, skills: skills, prompts: prompts}
}

// ListResources returns every latest row of one kind as neutral ResourceRows.
func (l *RegistryResourceLister) ListResources(ctx context.Context, kind string, updatedSince *time.Time) ([]ResourceRow, error) {
	f := regservice.ListFilter{LatestOnly: true, UpdatedSince: updatedSince, Limit: pageSize}
	switch kind {
	case "server":
		return pageRows(ctx, f, l.servers.List, func(s models.RegistryServer) ResourceRow {
			return ResourceRow{Kind: "server", Name: s.Name, Version: s.Version, Title: s.Title, Description: s.Description, SourcePeerID: s.SourcePeerID, UpdatedAt: s.UpdatedAt}
		})
	case "agent":
		return pageRows(ctx, f, l.agents.List, func(a models.RegistryAgent) ResourceRow {
			return ResourceRow{Kind: "agent", Name: a.Name, Version: a.Version, Title: a.Title, Description: a.Description, SourcePeerID: a.SourcePeerID, UpdatedAt: a.UpdatedAt}
		})
	case "skill":
		return pageRows(ctx, f, l.skills.List, func(s models.RegistrySkill) ResourceRow {
			return ResourceRow{Kind: "skill", Name: s.Name, Version: s.Version, Title: s.Title, Description: s.Description, SourcePeerID: s.SourcePeerID, UpdatedAt: s.UpdatedAt}
		})
	case "prompt":
		return pageRows(ctx, f, l.prompts.List, func(p models.RegistryPrompt) ResourceRow {
			return ResourceRow{Kind: "prompt", Name: p.Name, Version: p.Version, Title: p.Title, Description: p.Description, SourcePeerID: p.SourcePeerID, UpdatedAt: p.UpdatedAt}
		})
	default:
		return nil, nil
	}
}

// pageRows walks a registry List method to completion, converting each row.
func pageRows[T any](
	ctx context.Context,
	f regservice.ListFilter,
	list func(context.Context, regservice.ListFilter) (regservice.ListResult[T], error),
	conv func(T) ResourceRow,
) ([]ResourceRow, error) {
	var out []ResourceRow
	offset := 0
	for {
		f.Offset = offset
		res, err := list(ctx, f)
		if err != nil {
			return nil, err
		}
		for i := range res.Items {
			out = append(out, conv(res.Items[i]))
		}
		offset += len(res.Items)
		if len(res.Items) == 0 || int64(offset) >= res.Total {
			break
		}
	}
	return out, nil
}

// UseCaseStore adapts the use-case service to both UseCaseLister and
// BindingLister.
type UseCaseStore struct {
	svc *ucservice.Service
}

// NewUseCaseStore wires the use-case adapter.
func NewUseCaseStore(svc *ucservice.Service) *UseCaseStore { return &UseCaseStore{svc: svc} }

// ListUseCases returns every use case (paged), filtered to those updated at
// or after updatedSince when provided (the service List has no such filter,
// so it is applied here).
func (s *UseCaseStore) ListUseCases(ctx context.Context, updatedSince *time.Time) ([]models.UseCase, error) {
	var out []models.UseCase
	offset := 0
	for {
		rows, err := s.svc.List(ctx, ucservice.ListFilter{Limit: pageSize, Offset: offset})
		if err != nil {
			return nil, err
		}
		for _, uc := range rows {
			if updatedSince != nil && uc.UpdatedAt.Before(*updatedSince) {
				continue
			}
			out = append(out, uc)
		}
		if len(rows) < pageSize {
			break
		}
		offset += len(rows)
	}
	return out, nil
}

// ListBindings delegates to the use-case service.
func (s *UseCaseStore) ListBindings(ctx context.Context, useCaseID *uuid.UUID) ([]models.RegistryRef, error) {
	return s.svc.ListBindings(ctx, useCaseID)
}
