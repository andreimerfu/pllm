package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	a2asvc "github.com/amerfu/pllm/internal/services/a2a"
)

func TestAgentCardDerivedFromRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel:   models.BaseModel{ID: agentID},
		Name:        "pllm.io/sales-research",
		Version:     "0.1.0",
		Title:       "Sales Research",
		Description: "Summarize customer docs",
		IsLatest:    true,
	}).Error)
	require.NoError(t, db.Create(&models.AgentDeployment{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		RegistryAgentID: agentID,
		ResourceName:    "pllm.io/sales-research",
		ResourceVersion: "0.1.0",
		UseCaseID:       uuid.New(),
		Platform:        models.DeploymentPlatformKubernetes,
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
		Status:          models.DeploymentStatusRunning,
	}).Error)
	require.NoError(t, db.Create(&models.RegistryRef{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		OwnerKind:  models.RegistryKindAgent,
		OwnerID:    agentID,
		TargetKind: models.RegistryKindSkill,
		TargetName: "summarize",
	}).Error)

	h := NewCardHandler(nil, a2asvc.NewManager(db), db, "https://pllm.example.com")
	r := chi.NewRouter()
	r.Get("/v1/a2a/{slug}/.well-known/agent.json", h.Card)

	req := httptest.NewRequest(http.MethodGet, "/v1/a2a/sales-research-0-1-0/.well-known/agent.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var card a2asvc.AgentCard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &card))
	require.Equal(t, "pllm.io/sales-research", card.Name)
	require.Equal(t, "0.1.0", card.Version)
	require.Equal(t, "https://pllm.example.com/v1/a2a/sales-research-0-1-0", card.URL)
	require.Len(t, card.Skills, 1)
	require.Equal(t, "summarize", card.Skills[0].Name)
	require.False(t, card.Capabilities.Streaming)
}
