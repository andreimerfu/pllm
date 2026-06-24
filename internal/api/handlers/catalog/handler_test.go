package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/amerfu/pllm/internal/core/models"
)

// --- fakes ---

type fakeResources struct {
	byKind       map[string][]ResourceRow
	kindsAsked   []string
	updatedSince *time.Time
}

func (f *fakeResources) ListResources(_ context.Context, kind string, since *time.Time) ([]ResourceRow, error) {
	f.kindsAsked = append(f.kindsAsked, kind)
	f.updatedSince = since
	return f.byKind[kind], nil
}

type fakeUseCases struct {
	rows         []models.UseCase
	updatedSince *time.Time
}

func (f *fakeUseCases) ListUseCases(_ context.Context, since *time.Time) ([]models.UseCase, error) {
	f.updatedSince = since
	return f.rows, nil
}

type fakeBindings struct {
	refs       []models.RegistryRef
	askedForID *uuid.UUID
}

func (f *fakeBindings) ListBindings(_ context.Context, id *uuid.UUID) ([]models.RegistryRef, error) {
	f.askedForID = id
	return f.refs, nil
}

func newHandler(res ResourceLister, uc UseCaseLister, bind BindingLister) *Handler {
	return NewHandler(nil, res, uc, bind)
}

func decode(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("bad json: %v (%s)", err, body)
	}
	return m
}

// --- resources ---

func TestListResources_MapsAndDefaultsSource(t *testing.T) {
	res := &fakeResources{byKind: map[string][]ResourceRow{
		"agent": {{Kind: "agent", Name: "io.github.acme/refund-agent", Version: "1.0.0", Title: "Refund", SourcePeerID: ""}},
	}}
	h := newHandler(res, &fakeUseCases{}, &fakeBindings{})

	rr := httptest.NewRecorder()
	h.ListResources(rr, httptest.NewRequest(http.MethodGet, "/catalog/resources?kind=agent", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	m := decode(t, rr.Body.Bytes())
	items := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["name"] != "io.github.acme/refund-agent" {
		t.Errorf("name = %v", item["name"])
	}
	if item["display_name"] != "Refund" {
		t.Errorf("display_name = %v", item["display_name"])
	}
	if item["source"] != "local" {
		t.Errorf("source should default to local, got %v", item["source"])
	}
	// data_classifications must be a (possibly empty) array, never null.
	if dc, ok := item["data_classifications"].([]any); !ok || len(dc) != 0 {
		t.Errorf("data_classifications = %v, want []", item["data_classifications"])
	}
}

func TestListResources_NoKindAsksAllKinds(t *testing.T) {
	res := &fakeResources{byKind: map[string][]ResourceRow{}}
	h := newHandler(res, &fakeUseCases{}, &fakeBindings{})

	rr := httptest.NewRecorder()
	h.ListResources(rr, httptest.NewRequest(http.MethodGet, "/catalog/resources", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	want := map[string]bool{"agent": true, "server": true, "skill": true, "prompt": true}
	if len(res.kindsAsked) != len(want) {
		t.Fatalf("asked kinds = %v, want all four", res.kindsAsked)
	}
	for _, k := range res.kindsAsked {
		if !want[k] {
			t.Errorf("unexpected kind asked: %s", k)
		}
	}
}

func TestListResources_InvalidKind400(t *testing.T) {
	h := newHandler(&fakeResources{}, &fakeUseCases{}, &fakeBindings{})
	rr := httptest.NewRecorder()
	h.ListResources(rr, httptest.NewRequest(http.MethodGet, "/catalog/resources?kind=bogus", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestListResources_UpdatedSinceParsed(t *testing.T) {
	res := &fakeResources{byKind: map[string][]ResourceRow{}}
	h := newHandler(res, &fakeUseCases{}, &fakeBindings{})
	rr := httptest.NewRecorder()
	h.ListResources(rr, httptest.NewRequest(http.MethodGet, "/catalog/resources?kind=agent&updated_since=2026-06-23T10:00:00Z", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if res.updatedSince == nil {
		t.Fatalf("updated_since not parsed/forwarded")
	}
}

func TestListResources_BadUpdatedSince400(t *testing.T) {
	h := newHandler(&fakeResources{}, &fakeUseCases{}, &fakeBindings{})
	rr := httptest.NewRecorder()
	h.ListResources(rr, httptest.NewRequest(http.MethodGet, "/catalog/resources?kind=agent&updated_since=nope", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// --- use cases ---

func TestListUseCases_Maps(t *testing.T) {
	uc := models.UseCase{
		Name: "Refund", Slug: "refund", RiskTier: models.UseCaseRiskTier2,
		Status: models.UseCaseStatusApproved, DataClassifications: pq.StringArray{"PII"},
	}
	uc.ID = uuid.New()
	h := newHandler(&fakeResources{}, &fakeUseCases{rows: []models.UseCase{uc}}, &fakeBindings{})

	rr := httptest.NewRecorder()
	h.ListUseCases(rr, httptest.NewRequest(http.MethodGet, "/catalog/use-cases", nil))

	m := decode(t, rr.Body.Bytes())
	items := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("want 1 use case, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["slug"] != "refund" || item["risk_tier"] != "tier2" || item["status"] != "approved" {
		t.Errorf("use case mapped wrong: %v", item)
	}
	dc := item["data_classifications"].([]any)
	if len(dc) != 1 || dc[0] != "PII" {
		t.Errorf("data_classifications = %v", item["data_classifications"])
	}
}

// --- bindings ---

func TestListBindings_MapsEdges(t *testing.T) {
	ownerID := uuid.New()
	ref := models.RegistryRef{OwnerKind: models.RegistryKindUseCase, OwnerID: ownerID, TargetKind: models.RegistryKindAgent, TargetName: "io.github.acme/refund-agent"}
	h := newHandler(&fakeResources{}, &fakeUseCases{}, &fakeBindings{refs: []models.RegistryRef{ref}})

	rr := httptest.NewRecorder()
	h.ListBindings(rr, httptest.NewRequest(http.MethodGet, "/catalog/bindings", nil))

	m := decode(t, rr.Body.Bytes())
	items := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("want 1 binding, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["use_case"] != ownerID.String() || item["resource_kind"] != "agent" || item["resource_name"] != "io.github.acme/refund-agent" {
		t.Errorf("binding mapped wrong: %v", item)
	}
}

func TestListBindings_UseCaseFilterParsed(t *testing.T) {
	id := uuid.New()
	bind := &fakeBindings{}
	h := newHandler(&fakeResources{}, &fakeUseCases{}, bind)

	rr := httptest.NewRecorder()
	h.ListBindings(rr, httptest.NewRequest(http.MethodGet, "/catalog/bindings?use_case="+id.String(), nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if bind.askedForID == nil || *bind.askedForID != id {
		t.Fatalf("use_case filter not forwarded: %v", bind.askedForID)
	}
}

func TestListBindings_BadUseCase400(t *testing.T) {
	h := newHandler(&fakeResources{}, &fakeUseCases{}, &fakeBindings{})
	rr := httptest.NewRecorder()
	h.ListBindings(rr, httptest.NewRequest(http.MethodGet, "/catalog/bindings?use_case=not-a-uuid", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}
