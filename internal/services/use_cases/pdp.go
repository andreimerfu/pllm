package use_cases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/amerfu/pllm/internal/core/models"
)

// useCaseLoader loads a UseCase row for OPA input-document construction.
// Split out from gorm so the PDP client is unit-testable without a DB.
type useCaseLoader interface {
	loadUseCase(ctx context.Context, id uuid.UUID) (*models.UseCase, error)
}

// gormUseCaseLoader is the production loader backed by the database.
type gormUseCaseLoader struct{ db *gorm.DB }

func (l gormUseCaseLoader) loadUseCase(ctx context.Context, id uuid.UUID) (*models.UseCase, error) {
	var uc models.UseCase
	if err := l.db.WithContext(ctx).First(&uc, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &uc, nil
}

// PDPDecider asks an external OPA Policy Decision Point (e.g. Beacon) for an
// allow/deny verdict. It is the "remote" half of CompositeDecider; its
// methods return (Decision, error) so the composite can fall back on
// failure.
type PDPDecider struct {
	loader   useCaseLoader
	client   *http.Client
	endpoint string
	token    string
}

// NewPDPDecider builds a PDP client posting to endpoint with a bearer
// token and per-call timeout, loading use cases from db.
func NewPDPDecider(db *gorm.DB, endpoint, token string, timeout time.Duration) *PDPDecider {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &PDPDecider{
		loader:   gormUseCaseLoader{db: db},
		client:   &http.Client{Timeout: timeout},
		endpoint: endpoint,
		token:    token,
	}
}

// pdpRequest is the OPA input document sent to the remote PDP. The schema
// is a stable contract the PDP's Rego policies read as `input.*`.
type pdpRequest struct {
	Action  string     `json:"action"`
	UseCase pdpUseCase `json:"use_case"`
	Target  pdpTarget  `json:"target"`
}

type pdpUseCase struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	Slug                     string   `json:"slug"`
	RiskTier                 string   `json:"risk_tier"`
	Status                   string   `json:"status"`
	EnforceScope             bool     `json:"enforce_scope"`
	DataClassifications      []string `json:"data_classifications"`
	EffectiveClassifications []string `json:"effective_classifications"`
	ComplianceFlag           bool     `json:"compliance_flag"`
}

type pdpTarget struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// pdpResponse is the remote PDP's verdict. Enforce is a pointer so an omitted
// field falls back to the use case's own EnforceScope.
type pdpResponse struct {
	Allow   bool   `json:"allow"`
	Enforce *bool  `json:"enforce"`
	Reason  string `json:"reason"`
}

func (b *PDPDecider) AllowLLM(ctx context.Context, useCaseID *uuid.UUID, modelName string) (Decision, error) {
	return b.decide(ctx, useCaseID, "call_llm", "llm", modelName)
}

func (b *PDPDecider) AllowMCPTool(ctx context.Context, useCaseID *uuid.UUID, serverSlug string) (Decision, error) {
	return b.decide(ctx, useCaseID, "call_mcp_tool", "server", serverSlug)
}

func (b *PDPDecider) AllowAgent(ctx context.Context, useCaseID *uuid.UUID, agentName string) (Decision, error) {
	return b.decide(ctx, useCaseID, "invoke_agent", "agent", agentName)
}

func (b *PDPDecider) AllowPrompt(ctx context.Context, useCaseID *uuid.UUID, name string) (Decision, error) {
	return b.decide(ctx, useCaseID, "fetch_prompt", "prompt", name)
}

// decide builds the OPA input document, calls the PDP, and translates the
// verdict. Unscoped keys (nil useCaseID) bypass without a PDP call,
// matching ScopeResolver. Any transport/HTTP/parse failure returns a
// non-nil error so CompositeDecider can apply its OnError policy.
func (b *PDPDecider) decide(ctx context.Context, useCaseID *uuid.UUID, action, targetKind, targetName string) (Decision, error) {
	if useCaseID == nil {
		return Decision{Allowed: true}, nil
	}

	uc, err := b.loader.loadUseCase(ctx, *useCaseID)
	if err != nil {
		return Decision{}, fmt.Errorf("load use case: %w", err)
	}

	body, err := json.Marshal(pdpRequest{
		Action: action,
		UseCase: pdpUseCase{
			ID:                       uc.ID.String(),
			Name:                     uc.Name,
			Slug:                     uc.Slug,
			RiskTier:                 string(uc.RiskTier),
			Status:                   string(uc.Status),
			EnforceScope:             uc.EnforceScope,
			DataClassifications:      []string(uc.DataClassifications),
			EffectiveClassifications: []string(uc.EffectiveClassifications),
			ComplianceFlag:           uc.ComplianceFlag,
		},
		Target: pdpTarget{Kind: targetKind, Name: targetName},
	})
	if err != nil {
		return Decision{}, fmt.Errorf("marshal pdp request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint, bytes.NewReader(body))
	if err != nil {
		return Decision{}, fmt.Errorf("build pdp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if b.token != "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return Decision{}, fmt.Errorf("pdp call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Decision{}, fmt.Errorf("pdp returned status %d", resp.StatusCode)
	}

	var out pdpResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Decision{}, fmt.Errorf("decode pdp response: %w", err)
	}

	enforce := uc.EnforceScope
	if out.Enforce != nil {
		enforce = *out.Enforce
	}
	return Decision{Allowed: out.Allow, EnforceMode: enforce, Reason: out.Reason}, nil
}
