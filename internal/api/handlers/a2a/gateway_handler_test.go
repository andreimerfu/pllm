package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

func fakePod(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/invoke", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var in a2asvc.InvokeRequest
		_ = json.Unmarshal(body, &in)
		_ = json.NewEncoder(w).Encode(a2asvc.InvokeResponse{Output: "ECHO:" + in.Input.(string)})
	}))
}

type stubScope struct{}

func (stubScope) AllowAgent(_ context.Context, _ *uuid.UUID, _ string) ScopeDecision {
	return ScopeDecision{Allowed: true}
}

type stubRec struct{ denials int }

func (r *stubRec) RecordAgentDenial(
	_ context.Context, _ uuid.UUID, _ *uuid.UUID,
	_, _, _ string, _ bool, _, _ string,
) {
	r.denials++
}

func TestInvokeAliasHappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	pod := fakePod(t)
	defer pod.Close()

	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceName: "pllm.io/a", ResourceVersion: "0.1.0",
		UseCaseID: uuid.New(),
		Platform:  models.DeploymentPlatformKubernetes,
		Namespace: "pllm-agents", WorkloadName: "a-0-1-0",
		Status:   models.DeploymentStatusRunning,
		Endpoint: pod.URL,
	}).Error)

	h := NewGatewayHandler(
		nil,
		a2asvc.NewManager(db),
		a2asvc.NewProxy(2*time.Second),
		stubScope{}, &stubRec{},
	)

	r := chi.NewRouter()
	r.Post("/v1/agents/{slug}/invoke", h.InvokePlain)

	body, _ := json.Marshal(map[string]any{"input": "hello", "task_id": "t-1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/a-0-1-0/invoke", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var out a2asvc.InvokeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "ECHO:hello", out.Output)
}

type denyScope struct{}

func (denyScope) AllowAgent(_ context.Context, _ *uuid.UUID, _ string) ScopeDecision {
	return ScopeDecision{Allowed: false, EnforceMode: true, Reason: "not in use case"}
}

func TestInvokeAliasBlockedByScope(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	ucID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: ucID},
		Name: "uc", Slug: "uc",
	}).Error)
	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceName: "pllm.io/a", ResourceVersion: "0.1.0",
		UseCaseID: ucID, Platform: models.DeploymentPlatformKubernetes,
		Namespace: "pllm-agents", WorkloadName: "a-0-1-0",
		Status:   models.DeploymentStatusRunning,
		Endpoint: "http://never-called.invalid",
	}).Error)

	rec := &stubRec{}
	h := NewGatewayHandler(nil, a2asvc.NewManager(db), a2asvc.NewProxy(time.Second), denyScope{}, rec)

	r := chi.NewRouter()
	r.Post("/v1/agents/{slug}/invoke", h.InvokePlain)

	body, _ := json.Marshal(map[string]any{"input": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/a-0-1-0/invoke", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	// Note: denial is not recorded in the test path because the caller
	// has no Key in the context (no middleware wrapping); the denial
	// path still returns 403. This asserts the enforcement behavior.
}

func TestA2AJSONRPCTasksSend(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	pod := fakePod(t)
	defer pod.Close()

	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:    models.BaseModel{ID: uuid.New()},
		ResourceName: "pllm.io/a", ResourceVersion: "0.1.0",
		UseCaseID: uuid.New(),
		Platform:  models.DeploymentPlatformKubernetes,
		Namespace: "pllm-agents", WorkloadName: "a-0-1-0",
		Status:   models.DeploymentStatusRunning,
		Endpoint: pod.URL,
	}).Error)

	h := NewGatewayHandler(nil, a2asvc.NewManager(db), a2asvc.NewProxy(time.Second), stubScope{}, &stubRec{})
	r := chi.NewRouter()
	r.Post("/v1/a2a/{slug}", h.JSONRPC)

	env := map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/send",
		"params": map[string]any{
			"id": "t-xyz",
			"message": map[string]any{
				"role": "user",
				"parts": []map[string]any{{"type": "text", "text": "hi"}},
			},
		},
		"id": 1,
	}
	body, _ := json.Marshal(env)
	req := httptest.NewRequest(http.MethodPost, "/v1/a2a/a-0-1-0", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp a2asvc.JSONRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Nil(t, resp.Error)

	b, _ := json.Marshal(resp.Result)
	var task a2asvc.Task
	require.NoError(t, json.Unmarshal(b, &task))
	require.Equal(t, "t-xyz", task.ID)
	require.Equal(t, "completed", task.Status.State)
	require.Len(t, task.Messages, 1)
	require.Equal(t, "agent", task.Messages[0].Role)
}
