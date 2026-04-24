package deployment

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestProvisionScopedKeyStoresInK8sSecret(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	k8s := fake.NewSimpleClientset()

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "test-uc",
		Slug:      "test-uc",
	}).Error)

	h := NewAgentSecretHelper(db, k8s, nil)

	res, err := h.ProvisionScopedKey(context.Background(), ScopedKeyRequest{
		Name:       "test-agent-key",
		Namespace:  "pllm-agents",
		SecretName: "test-agent-key-secret",
		UseCaseID:  useCaseID,
	})
	require.NoError(t, err)
	require.NotNil(t, res.Key)
	require.NotNil(t, res.Key.UseCaseID)
	require.Equal(t, useCaseID, *res.Key.UseCaseID)

	sec, err := k8s.CoreV1().Secrets("pllm-agents").Get(context.Background(), "test-agent-key-secret", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, res.Plaintext, string(sec.Data["PLLM_API_KEY"]))
}

func TestRevokeScopedKeyIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	k8s := fake.NewSimpleClientset()
	h := NewAgentSecretHelper(db, k8s, nil)

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "uc", Slug: "uc",
	}).Error)

	res, err := h.ProvisionScopedKey(context.Background(), ScopedKeyRequest{
		Name:       "rk",
		Namespace:  "pllm-agents",
		SecretName: "rk-sec",
		UseCaseID:  useCaseID,
	})
	require.NoError(t, err)

	require.NoError(t, h.RevokeScopedKey(context.Background(), &res.Key.ID, "pllm-agents", "rk-sec"))
	require.NoError(t, h.RevokeScopedKey(context.Background(), &res.Key.ID, "pllm-agents", "rk-sec"))
}
