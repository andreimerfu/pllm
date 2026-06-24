package use_cases

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/amerfu/pllm/internal/core/models"
)

// fakeLoader stands in for the gorm-backed use-case loader so the PDP
// client is testable without a database.
type fakeLoader struct {
	uc  *models.UseCase
	err error
}

func (f fakeLoader) loadUseCase(_ context.Context, _ uuid.UUID) (*models.UseCase, error) {
	return f.uc, f.err
}

func sampleUseCase() *models.UseCase {
	uc := &models.UseCase{
		Name:                     "Fraud Review",
		Slug:                     "fraud-review",
		RiskTier:                 models.UseCaseRiskTier1,
		Status:                   models.UseCaseStatusApproved,
		EnforceScope:             true,
		DataClassifications:      pq.StringArray{"PII"},
		EffectiveClassifications: pq.StringArray{"PII", "Financial"},
	}
	uc.ID = uuid.New()
	return uc
}

// newTestPDP wires a PDPDecider at the given test server with a fake
// loader returning uc.
func newTestPDP(srv *httptest.Server, uc *models.UseCase) *PDPDecider {
	return &PDPDecider{
		loader:   fakeLoader{uc: uc},
		client:   srv.Client(),
		endpoint: srv.URL,
		token:    "tok-123",
	}
}

func boolResponder(allow bool, enforce *bool, reason string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"allow":   allow,
			"enforce": enforce,
			"reason":  reason,
		})
	}
}

func TestPDPDecider_NilUseCaseBypasses(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
	}))
	defer srv.Close()
	b := newTestPDP(srv, sampleUseCase())

	d, err := b.AllowLLM(context.Background(), nil, "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed {
		t.Fatalf("nil use case should bypass to allowed; got %+v", d)
	}
	if hits != 0 {
		t.Fatalf("nil use case must not call the PDP; got %d hits", hits)
	}
}

func TestPDPDecider_AllowTrue(t *testing.T) {
	srv := httptest.NewServer(boolResponder(true, nil, "ok"))
	defer srv.Close()
	b := newTestPDP(srv, sampleUseCase())

	d, err := b.AllowAgent(context.Background(), ptr(uuid.New()), "io.github.org/agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Allowed || d.Reason != "ok" {
		t.Fatalf("expected allowed with reason ok; got %+v", d)
	}
}

func TestPDPDecider_EnforceDefaultsToUseCase(t *testing.T) {
	srv := httptest.NewServer(boolResponder(false, nil, "denied")) // enforce omitted
	defer srv.Close()
	uc := sampleUseCase() // EnforceScope = true
	b := newTestPDP(srv, uc)

	d, _ := b.AllowMCPTool(context.Background(), ptr(uc.ID), "filesystem")
	if d.Allowed {
		t.Fatalf("expected denied; got %+v", d)
	}
	if !d.EnforceMode {
		t.Fatalf("enforce should default to use case EnforceScope=true; got %+v", d)
	}
}

func TestPDPDecider_EnforceExplicitOverrides(t *testing.T) {
	no := false
	srv := httptest.NewServer(boolResponder(false, &no, "flag only")) // explicit enforce=false
	defer srv.Close()
	uc := sampleUseCase() // EnforceScope = true
	b := newTestPDP(srv, uc)

	d, _ := b.AllowPrompt(context.Background(), ptr(uc.ID), "greeting")
	if d.EnforceMode {
		t.Fatalf("explicit enforce=false should override use case; got %+v", d)
	}
}

func TestPDPDecider_BuildsInputDocument(t *testing.T) {
	var got struct {
		Action  string `json:"action"`
		UseCase struct {
			ID                       string   `json:"id"`
			RiskTier                 string   `json:"risk_tier"`
			Status                   string   `json:"status"`
			EnforceScope             bool     `json:"enforce_scope"`
			DataClassifications      []string `json:"data_classifications"`
			EffectiveClassifications []string `json:"effective_classifications"`
		} `json:"use_case"`
		Target struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"target"`
	}
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_ = json.NewEncoder(w).Encode(map[string]any{"allow": true})
	}))
	defer srv.Close()
	uc := sampleUseCase()
	b := newTestPDP(srv, uc)

	if _, err := b.AllowAgent(context.Background(), ptr(uc.ID), "io.github.org/agent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Action != "invoke_agent" {
		t.Errorf("action = %q, want invoke_agent", got.Action)
	}
	if got.Target.Kind != "agent" || got.Target.Name != "io.github.org/agent" {
		t.Errorf("target = %+v, want {agent io.github.org/agent}", got.Target)
	}
	if got.UseCase.ID != uc.ID.String() {
		t.Errorf("use_case.id = %q, want %q", got.UseCase.ID, uc.ID.String())
	}
	if got.UseCase.RiskTier != "tier1" || got.UseCase.Status != "approved" {
		t.Errorf("use_case tier/status = %q/%q, want tier1/approved", got.UseCase.RiskTier, got.UseCase.Status)
	}
	if !got.UseCase.EnforceScope {
		t.Errorf("use_case.enforce_scope = false, want true")
	}
	if len(got.UseCase.EffectiveClassifications) != 2 {
		t.Errorf("effective_classifications = %v, want 2 entries", got.UseCase.EffectiveClassifications)
	}
	if authHeader != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want Bearer tok-123", authHeader)
	}
}

func TestPDPDecider_HTTPErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	uc := sampleUseCase()
	b := newTestPDP(srv, uc)

	if _, err := b.AllowLLM(context.Background(), ptr(uc.ID), "gpt-4"); err == nil {
		t.Fatalf("expected error on non-200 PDP response")
	}
}

func ptr(id uuid.UUID) *uuid.UUID { return &id }
