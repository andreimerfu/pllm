package deployment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/amerfu/pllm/internal/core/models"
)

// AgentSecretHelper is the single owner of scoped-key provisioning for
// agent deployments. Split out of the adapter so the adapter stays
// K8s-only and this file stays DB-only (testable in isolation).
type AgentSecretHelper struct {
	db     *gorm.DB
	k8s    kubernetes.Interface
	logger *zap.Logger
}

// NewAgentSecretHelper constructs an AgentSecretHelper. A nil logger is
// replaced with a no-op logger.
func NewAgentSecretHelper(db *gorm.DB, k8s kubernetes.Interface, logger *zap.Logger) *AgentSecretHelper {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentSecretHelper{db: db, k8s: k8s, logger: logger}
}

// ScopedKeyRequest describes the key to provision.
type ScopedKeyRequest struct {
	Name       string
	Namespace  string
	SecretName string
	UseCaseID  uuid.UUID
}

// ScopedKeyResult carries the created row + the plaintext (only available
// at creation time — never logged, only written into the Secret).
type ScopedKeyResult struct {
	Key       *models.Key
	Plaintext string
}

// ProvisionScopedKey creates a UseCase-bound Key row and writes the
// plaintext into a K8s Secret. Idempotent on SecretName: if a Secret
// already exists with the same name, it is replaced so the key rotates
// on redeploy.
func (h *AgentSecretHelper) ProvisionScopedKey(ctx context.Context, req ScopedKeyRequest) (*ScopedKeyResult, error) {
	if req.Name == "" || req.SecretName == "" || req.Namespace == "" {
		return nil, errors.New("scoped key: name, namespace, and secret_name required")
	}
	if req.UseCaseID == uuid.Nil {
		return nil, errors.New("scoped key: use_case_id required")
	}

	plaintext, hash, err := models.GenerateKey(models.KeyTypeAPI)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	row := &models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      req.Name,
		Key:       plaintext,
		KeyHash:   hash,
		Type:      models.KeyTypeAPI,
		IsActive:  true,
		UseCaseID: &req.UseCaseID,
	}
	if err := h.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, fmt.Errorf("persist key: %w", err)
	}

	if err := h.upsertSecret(ctx, req.Namespace, req.SecretName, plaintext); err != nil {
		// Roll back the key row on Secret failure so we don't orphan keys.
		_ = h.db.WithContext(ctx).Delete(row).Error
		return nil, err
	}

	return &ScopedKeyResult{Key: row, Plaintext: plaintext}, nil
}

// RevokeScopedKey soft-deletes the Key row and drops the Secret. Safe to
// call twice; missing pieces are no-ops.
func (h *AgentSecretHelper) RevokeScopedKey(ctx context.Context, keyID *uuid.UUID, namespace, secretName string) error {
	if keyID != nil {
		if err := h.db.WithContext(ctx).Delete(&models.Key{}, "id = ?", *keyID).Error; err != nil {
			h.logger.Warn("scoped key: delete row",
				zap.String("key_id", keyID.String()), zap.Error(err))
		}
	}
	if namespace != "" && secretName != "" {
		if err := h.k8s.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("k8s: delete secret: %w", err)
		}
	}
	return nil
}

// upsertSecret writes the plaintext into a K8s Secret, creating it when
// it doesn't exist and updating it when it does (rotate-on-redeploy).
func (h *AgentSecretHelper) upsertSecret(ctx context.Context, ns, name, plaintext string) error {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "pllm"},
		},
		Type: corev1.SecretTypeOpaque,
		// Populate both Data and StringData. The real API server converts
		// StringData → Data; the fake client does not, so we set Data directly
		// to keep tests and production behaviour consistent.
		Data:       map[string][]byte{"PLLM_API_KEY": []byte(plaintext)},
		StringData: map[string]string{"PLLM_API_KEY": plaintext},
	}
	_, err := h.k8s.CoreV1().Secrets(ns).Create(ctx, sec, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("k8s: create secret: %w", err)
	}
	existing, gerr := h.k8s.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if gerr != nil {
		return fmt.Errorf("k8s: get secret: %w", gerr)
	}
	sec.ResourceVersion = existing.ResourceVersion
	if _, err := h.k8s.CoreV1().Secrets(ns).Update(ctx, sec, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("k8s: update secret: %w", err)
	}
	return nil
}
