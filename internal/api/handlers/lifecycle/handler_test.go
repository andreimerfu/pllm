package lifecycle

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	lifesvc "github.com/amerfu/pllm/internal/services/lifecycle"
)

func TestStatusReturnsZeroSummaryBeforeAnyRun(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	svc := lifesvc.NewService(db, zap.NewNop(), lifesvc.Config{DryRun: true})
	h := NewHandler(zap.NewNop(), svc)

	r := chi.NewRouter()
	r.Get("/status", h.Status)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got lifesvc.RunSummary
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.TotalActions != 0 {
		t.Fatalf("expected 0 actions on first read, got %d", got.TotalActions)
	}
}

func TestRunNowExecutesRulesAndReturnsSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	svc := lifesvc.NewService(db, zap.NewNop(), lifesvc.Config{DryRun: true, InactiveKeyDays: 90})
	svc.Register(&lifesvc.InactiveKeysRule{})
	h := NewHandler(zap.NewNop(), svc)

	r := chi.NewRouter()
	r.Post("/run", h.RunNow)

	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var got lifesvc.RunSummary
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("expected 1 rule result, got %d", len(got.Rules))
	}
	if got.Rules[0].Rule != "inactive_keys" {
		t.Fatalf("expected inactive_keys rule, got %q", got.Rules[0].Rule)
	}
}

func TestNilServiceReturns503(t *testing.T) {
	h := NewHandler(zap.NewNop(), nil)
	r := chi.NewRouter()
	r.Get("/status", h.Status)
	r.Post("/run", h.RunNow)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{"GET", "/status"},
		{"POST", "/run"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: expected 503, got %d", tc.method, tc.path, w.Code)
			continue
		}
		var body map[string]string
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Errorf("%s %s: expected JSON body, got %v (%s)", tc.method, tc.path, err, w.Body.String())
			continue
		}
		if body["error"] == "" {
			t.Errorf("%s %s: expected non-empty error field, got %v", tc.method, tc.path, body)
		}
	}
}
