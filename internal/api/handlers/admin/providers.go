package admin

import (
	"net/http"

	"go.uber.org/zap"
)

type ProviderHandler struct {
	baseHandler
}

func NewProviderHandler(logger *zap.Logger) *ProviderHandler {
	return &ProviderHandler{
		baseHandler: baseHandler{logger: logger},
	}
}

func (h *ProviderHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "List providers")
}

func (h *ProviderHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Create provider")
}

func (h *ProviderHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get provider")
}

func (h *ProviderHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update provider")
}

func (h *ProviderHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Delete provider")
}

func (h *ProviderHandler) TestProvider(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Test provider")
}

func (h *ProviderHandler) GetProviderModels(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get provider models")
}

func (h *ProviderHandler) AddModel(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Add model")
}

func (h *ProviderHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Update model")
}

func (h *ProviderHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Delete model")
}

func (h *ProviderHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, "Get health")
}
