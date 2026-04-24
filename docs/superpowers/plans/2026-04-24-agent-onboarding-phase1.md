# Agent Onboarding — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the minimum end-to-end path that lets a platform operator deploy a hand-written containerized agent (e.g. a LangGraph agent image) into the pllm cluster, bound to an approved UseCase, with injected env vars + a scoped key so every LLM and MCP call the agent makes is scope-checked and audited.

**Architecture:** Mirror the existing MCP deployment stack. A new `AgentDeployment` model stores the running-agent bookkeeping; a new `AgentK8sAdapter` (parallel to `K8sAdapter`) applies `Deployment + Service + Secret` in a `pllm-agents` namespace. A scoped `Key` is provisioned at deploy-time with `UseCaseID` set, stored as a K8s `Secret`, and mounted into the pod via `envFrom`. Env vars `PLLM_BASE_URL`, `PLLM_MCP_URL`, `PLLM_USE_CASE`, `PLLM_AGENT_NAME` are injected plaintext. No SDK, no A2A invocation surface yet — those land in Phases 2 and 3.

**Tech Stack:** Go 1.25+, chi/v5, GORM + Postgres, `k8s.io/client-go` (fake client for tests), React 18 + shadcn + TanStack Query on the FE. Follows existing repo conventions exactly (narrow-interface pattern at layer boundaries, three AutoMigrate lists, `testing.Short()` gate on integration tests).

---

## Scope

**In scope (Phase 1):**
- `AgentDeployment` model + migrations
- `AgentK8sAdapter` implementing a parallel deploy/undeploy/status loop for agent images
- Scoped-key provisioning: create a `Key` with `UseCaseID`, store plaintext in a K8s `Secret`, teardown on undeploy
- Env var injection into the pod: `PLLM_BASE_URL`, `PLLM_MCP_URL`, `PLLM_USE_CASE`, `PLLM_AGENT_NAME` plus `PLLM_API_KEY` via `envFrom`
- Deployment service methods: `DeployAgent`, `UndeployAgent`, `ListAgents`, `GetAgent`, `RefreshAgentStatus`
- Admin HTTP: `POST|GET /api/admin/agents`, `GET|DELETE /api/admin/agents/{id}`, `POST /api/admin/agents/{id}/status`
- Config: `deployment.kubernetes.agent_namespace` (default `pllm-agents`)
- UI: read-only `web/src/pages/Agents.tsx` (list + detail, delete action, mirrors `Deployments.tsx`)
- Smoke script: `scripts/test-agent-onboarding.sh`

**Out of scope (deferred to later phases):**
- `pllmctl` CLI additions (Phase 3)
- `pllm-sdk` Python / Node packages (Phase 3)
- Agent base images (`pllm/agent-base-*`) (Phase 3)
- A2A transport / invocation (`/v1/a2a/*`) (Phase 2)
- Agent card discovery (`.well-known/agent.json`) (Phase 2)
- Dev-time scope streaming (`pllmctl dev`) (Phase 4)
- Framework templates (`deploy/agent-templates/`) (Phase 3)

---

## File Structure

| File | Create / Modify | Responsibility |
|---|---|---|
| `internal/core/models/agent_deployment.go` | create | New GORM model `AgentDeployment` |
| `internal/core/models/agent_deployment_test.go` | create | Model sanity tests |
| `internal/core/database/migrate.go` | modify | Add `AgentDeployment` to AutoMigrate list |
| `internal/core/database/connection.go` | modify | Same |
| `internal/infrastructure/testutil/database.go` | modify | Same |
| `internal/core/config/config.go` | modify | Add `DeploymentK8sConfig.AgentNamespace` |
| `internal/services/deployment/agent_adapter.go` | create | `AgentK8sAdapter` — parallels `K8sAdapter` |
| `internal/services/deployment/agent_adapter_test.go` | create | Adapter unit tests using `fake.NewSimpleClientset` |
| `internal/services/deployment/agent_secrets.go` | create | Scoped-key provisioning + K8s Secret create/delete |
| `internal/services/deployment/agent_secrets_test.go` | create | Secret helper tests |
| `internal/services/deployment/service.go` | modify | Add `DeployAgent`, `UndeployAgent`, `ListAgents`, `GetAgent`, `RefreshAgentStatus` |
| `internal/services/deployment/service_agent_test.go` | create | Service tests for agent flow |
| `internal/api/handlers/deployment/agent_handler.go` | create | HTTP handler |
| `internal/api/handlers/deployment/agent_handler_test.go` | create | Handler tests (table-driven, HTTP recorder) |
| `internal/api/router/admin.go` | modify | Mount agent routes + add `AgentDeployHandler` field |
| `internal/api/router/router.go` | modify | Construct `AgentK8sAdapter` + `AgentSecretHelper`, wire handler |
| `internal/services/deployment/kubernetes.go` | modify | Expose `Client()` so sibling adapters share the K8s client |
| `web/src/types/api.ts` | modify | `AgentDeployment` + request/response types |
| `web/src/lib/api.ts` | modify | `listAgents` / `deployAgent` / `deleteAgent` / `refreshAgentStatus` |
| `web/src/lib/icons.ts` | modify | Add `agents` icon |
| `web/src/pages/Agents.tsx` | create | Read-only Agents page |
| `web/src/App.tsx` | modify | Route `/agents` |
| `web/src/components/app-sidebar.tsx` | modify | Sidebar entry |
| `scripts/test-agent-onboarding.sh` | create | Smoke test |
| `config.yaml` | modify | Document `deployment.kubernetes.agent_namespace` |
| `config.docker.yaml` | modify | Same |

**Design principles this structure enforces:**
- Agent deploy is a **sibling** of MCP deploy, not a fork — both live under `internal/services/deployment/`. The shared `Adapter` interface is still used for MCP; we add a new `AgentAdapter` interface (3 methods) to avoid mixing request shapes.
- `agent_secrets.go` isolates the two side effects that differ from the MCP path (key provisioning + Secret lifecycle) in one testable file.
- `service.go` keeps the *orchestration* for both kinds in one place so `ReconcileAll` etc. can be extended in later phases without bouncing between files.
- No handler imports `use_cases` directly — it needs the UseCase ID only, passed through on the request, and the service calls the Key-creation helper.

**Deviation from the spec:** the spec calls for a separate `agent_bridge.go` file parallel to `mcp_bridge.go`. In Phase 1 there's nothing to "bridge" yet — the MCPBridge exists because it registers a deployed MCP server as a live gateway backend, but agents don't have a gateway surface until A2A lands in Phase 2. Phase 1's equivalent work (provision a scoped Key + mount it as a Secret) lives in `agent_secrets.go`. When Phase 2 adds A2A, we'll introduce `agent_bridge.go` that wraps the `AgentSecretHelper` plus the A2A-backend registrar. No public-surface impact for now.

---

## Task 1: AgentDeployment model + migrations

**Files:**
- Create: `internal/core/models/agent_deployment.go`
- Create: `internal/core/models/agent_deployment_test.go`
- Modify: `internal/core/database/migrate.go`
- Modify: `internal/core/database/connection.go`
- Modify: `internal/infrastructure/testutil/database.go`

- [ ] **Step 1: Write the model struct**

Create `internal/core/models/agent_deployment.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentDeployment tracks a running instance of a RegistryAgent. Separate
// from Deployment (which is MCP-server-only) so agent-specific columns —
// UseCaseID binding, provisioned key reference, agent env vars — don't
// grow the server deployment row with nullable clutter.
//
// One row per (RegistryAgentID + Namespace). Re-deploying the same agent
// version into the same namespace updates the existing row.
type AgentDeployment struct {
	BaseModel

	// What was deployed (registry agent row).
	RegistryAgentID uuid.UUID `gorm:"type:uuid;index;not null" json:"registry_agent_id"`
	ResourceName    string    `gorm:"index;not null" json:"resource_name"`
	ResourceVersion string    `gorm:"not null" json:"resource_version"`

	// Scope binding — required. An agent always runs under a UseCase.
	UseCaseID uuid.UUID `gorm:"type:uuid;index;not null" json:"use_case_id"`

	// Target.
	Platform     DeploymentPlatform `gorm:"not null" json:"platform"`
	Namespace    string             `gorm:"index;not null" json:"namespace"`
	WorkloadName string             `gorm:"uniqueIndex:idx_agent_deploy_wl;not null" json:"workload_name"`

	// Replica count requested (1 unless overridden).
	Replicas int32 `gorm:"default:1" json:"replicas"`

	// Lifecycle.
	Status       DeploymentStatus `gorm:"default:'pending';index" json:"status"`
	StatusReason string           `json:"status_reason,omitempty"`

	// URL to reach the pod — set after Service is up. Used by Phase 2 A2A.
	Endpoint string `json:"endpoint,omitempty"`

	// Provisioned scoped key — nullable to survive teardown partial failures.
	ProvisionedKeyID *uuid.UUID `gorm:"type:uuid" json:"provisioned_key_id,omitempty"`
	SecretName       string     `json:"secret_name,omitempty"`

	// Opaque adapter-specific state (manifest hash etc.).
	AdapterState datatypes.JSON `json:"adapter_state,omitempty"`

	LastAppliedAt *time.Time `json:"last_applied_at,omitempty"`
}
```

- [ ] **Step 2: Write a sanity test**

Create `internal/core/models/agent_deployment_test.go`:

```go
package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAgentDeploymentDefaults(t *testing.T) {
	d := AgentDeployment{
		RegistryAgentID: uuid.New(),
		ResourceName:    "sales-research",
		ResourceVersion: "0.1.0",
		UseCaseID:       uuid.New(),
		Platform:        DeploymentPlatformKubernetes,
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
	}
	require.Equal(t, DeploymentPlatformKubernetes, d.Platform)
	require.Equal(t, "pllm-agents", d.Namespace)
}
```

- [ ] **Step 3: Run the test to verify compilation**

Run: `go test ./internal/core/models/ -run TestAgentDeploymentDefaults -v`
Expected: PASS

- [ ] **Step 4: Add to all three AutoMigrate lists**

Modify `internal/core/database/migrate.go` — add `&models.AgentDeployment{},` immediately after `&models.Deployment{},` on line 39:

```go
		&models.Deployment{},         // Deployment: running registry-backed workloads
		&models.AgentDeployment{},    // AgentDeployment: running registry-agent workloads
		&models.UseCase{},            // UseCase: top-level approval unit that consumes components
```

Modify `internal/core/database/connection.go` — add the same entry immediately after `&models.Deployment{}`.

Modify `internal/infrastructure/testutil/database.go` — same.

- [ ] **Step 5: Verify migrations compile**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/core/models/agent_deployment.go \
        internal/core/models/agent_deployment_test.go \
        internal/core/database/migrate.go \
        internal/core/database/connection.go \
        internal/infrastructure/testutil/database.go
git commit -m "feat(models): add AgentDeployment model with UseCase binding

One row per (registry agent, namespace). Holds the provisioned scoped-key
reference + K8s Secret name so teardown can clean both. Registered in all
three AutoMigrate lists per repo convention."
```

---

## Task 2: Config — agent namespace

**Files:**
- Modify: `internal/core/config/config.go`
- Modify: `config.yaml`
- Modify: `config.docker.yaml`

- [ ] **Step 1: Add field**

Modify `internal/core/config/config.go` — inside `type DeploymentK8sConfig struct`, add the new field after `Namespace`:

```go
	// Namespace  — where MCP server Deployments land.
	Namespace string `mapstructure:"namespace"`
	// AgentNamespace — where agent Deployments land. Defaults to "pllm-agents"
	// if empty. Kept separate from Namespace so operators can apply different
	// RBAC / NetworkPolicy defaults to each.
	AgentNamespace    string `mapstructure:"agent_namespace"`
	ImagePullPolicy   string `mapstructure:"image_pull_policy"`
```

- [ ] **Step 2: Document in sample config files**

Modify `config.yaml` — inside `deployment.kubernetes:` block, after the existing `namespace:` line, add:

```yaml
    namespace: pllm-mcp
    agent_namespace: pllm-agents
```

Modify `config.docker.yaml` the same way.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/core/config/config.go config.yaml config.docker.yaml
git commit -m "feat(config): add deployment.kubernetes.agent_namespace

Defaults to pllm-agents, separate from pllm-mcp so operators can apply
different RBAC / NetworkPolicy defaults to each."
```

---

## Task 3: AgentK8sAdapter — Deploy, Undeploy, Status

**Files:**
- Create: `internal/services/deployment/agent_adapter.go`
- Create: `internal/services/deployment/agent_adapter_test.go`

- [ ] **Step 1: Write the interface + request types**

Create `internal/services/deployment/agent_adapter.go` with this top chunk:

```go
package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
)

// AgentRequest is the input for an agent deployment. Unlike Request
// (MCP servers), an agent is always a container image — npx/uvx paths
// aren't offered.
type AgentRequest struct {
	Namespace    string
	WorkloadName string

	DisplayName     string
	ResourceName    string
	ResourceVersion string

	// Container image to run. Required.
	Image string
	// Optional overrides.
	Command []string
	Args    []string
	Port    int32 // defaults to 8000

	// UseCase binding + provisioned-key secret.
	UseCaseID  string // UUID string, used in labels only
	SecretName string // K8s Secret holding PLLM_API_KEY

	// Plaintext env vars injected into every agent pod.
	// Caller is expected to populate BaseURL / MCPURL / UseCase / AgentName.
	Env map[string]string

	Replicas int32
}

// AgentAdapter is the platform-agnostic interface for agent deploys.
// Same contract as Adapter: idempotent Deploy, clean Undeploy, Status
// reports live state.
type AgentAdapter interface {
	Platform() models.DeploymentPlatform
	Deploy(ctx context.Context, req *AgentRequest) (*Result, error)
	Undeploy(ctx context.Context, d *models.AgentDeployment) error
	Status(ctx context.Context, d *models.AgentDeployment) (*StatusReport, error)
}

// AgentK8sAdapter implements AgentAdapter against Kubernetes. Constructed
// with a kubernetes.Interface so tests can pass a fake client.
type AgentK8sAdapter struct {
	client kubernetes.Interface
	cfg    config.DeploymentK8sConfig
	logger *zap.Logger
}

func NewAgentK8sAdapter(client kubernetes.Interface, cfg config.DeploymentK8sConfig, logger *zap.Logger) *AgentK8sAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentK8sAdapter{client: client, cfg: cfg, logger: logger}
}

func (a *AgentK8sAdapter) Platform() models.DeploymentPlatform {
	return models.DeploymentPlatformKubernetes
}

const defaultAgentPort int32 = 8000
```

- [ ] **Step 2: Write the failing Deploy test**

Create `internal/services/deployment/agent_adapter_test.go`:

```go
package deployment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/config"
)

func newTestAgentAdapter() *AgentK8sAdapter {
	return NewAgentK8sAdapter(fake.NewSimpleClientset(), config.DeploymentK8sConfig{
		AgentNamespace: "pllm-agents",
	}, nil)
}

func TestAgentDeployCreatesDeploymentAndService(t *testing.T) {
	adapter := newTestAgentAdapter()
	res, err := adapter.Deploy(context.Background(), &AgentRequest{
		Namespace:       "pllm-agents",
		WorkloadName:    "sales-research-0-1-0",
		DisplayName:     "Sales Research",
		ResourceName:    "pllm.io/sales-research",
		ResourceVersion: "0.1.0",
		Image:           "ghcr.io/acme/sales-research:0.1.0",
		UseCaseID:       "11111111-1111-1111-1111-111111111111",
		SecretName:      "sales-research-0-1-0-key",
		Env: map[string]string{
			"PLLM_BASE_URL":   "http://pllm:8080",
			"PLLM_MCP_URL":    "http://pllm:8080/v1/mcp",
			"PLLM_USE_CASE":   "customer-research-approved",
			"PLLM_AGENT_NAME": "sales-research",
		},
		Replicas: 2,
	})
	require.NoError(t, err)
	require.Contains(t, res.Endpoint, "sales-research-0-1-0.pllm-agents.svc.cluster.local")

	dep, err := adapter.client.AppsV1().Deployments("pllm-agents").
		Get(context.Background(), "sales-research-0-1-0", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, int32(2), *dep.Spec.Replicas)

	container := dep.Spec.Template.Spec.Containers[0]
	require.Equal(t, "ghcr.io/acme/sales-research:0.1.0", container.Image)

	// Plaintext env vars injected.
	envMap := map[string]string{}
	for _, e := range container.Env {
		envMap[e.Name] = e.Value
	}
	require.Equal(t, "http://pllm:8080", envMap["PLLM_BASE_URL"])
	require.Equal(t, "customer-research-approved", envMap["PLLM_USE_CASE"])

	// Secret reference via envFrom for PLLM_API_KEY.
	require.Len(t, container.EnvFrom, 1)
	require.NotNil(t, container.EnvFrom[0].SecretRef)
	require.Equal(t, "sales-research-0-1-0-key", container.EnvFrom[0].SecretRef.Name)

	// Agent label set.
	svc, err := adapter.client.CoreV1().Services("pllm-agents").
		Get(context.Background(), "sales-research-0-1-0", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "agent", svc.Labels["pllm.ai/kind"])
}
```

- [ ] **Step 3: Run the test — expect compile failure (method not defined)**

Run: `go test ./internal/services/deployment/ -run TestAgentDeployCreatesDeploymentAndService -v`
Expected: FAIL — `(*AgentK8sAdapter).Deploy` undefined.

- [ ] **Step 4: Implement Deploy**

Append to `internal/services/deployment/agent_adapter.go`:

```go
// Deploy implements AgentAdapter. Idempotent on WorkloadName.
func (a *AgentK8sAdapter) Deploy(ctx context.Context, req *AgentRequest) (*Result, error) {
	if err := validateAgentRequest(req); err != nil {
		return nil, err
	}
	ns := req.Namespace
	if ns == "" {
		ns = a.cfg.AgentNamespace
	}
	if ns == "" {
		ns = "pllm-agents"
	}
	if err := a.ensureNamespace(ctx, ns); err != nil {
		return nil, err
	}

	deploy := a.buildDeployment(req, ns)
	svc := a.buildService(req, ns)

	// Deployment — create or update.
	if _, err := a.client.AppsV1().Deployments(ns).Create(ctx, deploy, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("k8s: create agent deployment: %w", err)
		}
		existing, gerr := a.client.AppsV1().Deployments(ns).Get(ctx, deploy.Name, metav1.GetOptions{})
		if gerr != nil {
			return nil, fmt.Errorf("k8s: get agent deployment: %w", gerr)
		}
		deploy.ResourceVersion = existing.ResourceVersion
		if _, err := a.client.AppsV1().Deployments(ns).Update(ctx, deploy, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("k8s: update agent deployment: %w", err)
		}
	}

	// Service — create or update.
	if _, err := a.client.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("k8s: create agent service: %w", err)
		}
		existing, gerr := a.client.CoreV1().Services(ns).Get(ctx, svc.Name, metav1.GetOptions{})
		if gerr != nil {
			return nil, fmt.Errorf("k8s: get agent service: %w", gerr)
		}
		svc.ResourceVersion = existing.ResourceVersion
		svc.Spec.ClusterIP = existing.Spec.ClusterIP
		if _, err := a.client.CoreV1().Services(ns).Update(ctx, svc, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("k8s: update agent service: %w", err)
		}
	}

	port := req.Port
	if port == 0 {
		port = defaultAgentPort
	}
	state, _ := json.Marshal(adapterState{ManifestHash: hashAgentRequest(req)})
	return &Result{
		Endpoint:     fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, ns, port),
		AdapterState: state,
	}, nil
}

func validateAgentRequest(req *AgentRequest) error {
	if req == nil {
		return errors.New("agent request is nil")
	}
	if req.WorkloadName == "" {
		return errors.New("workload_name is required")
	}
	if len(req.WorkloadName) > 63 {
		return fmt.Errorf("workload_name %q exceeds 63 chars", req.WorkloadName)
	}
	for _, ch := range req.WorkloadName {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= '0' && ch <= '9':
		case ch == '-':
		default:
			return fmt.Errorf("workload_name %q contains invalid character %q", req.WorkloadName, ch)
		}
	}
	if req.Image == "" {
		return errors.New("agent image is required")
	}
	if req.SecretName == "" {
		return errors.New("secret_name is required (scoped key)")
	}
	return nil
}

func (a *AgentK8sAdapter) ensureNamespace(ctx context.Context, ns string) error {
	_, err := a.client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("k8s: check namespace: %w", err)
	}
	_, err = a.client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ns,
			Labels: map[string]string{"app.kubernetes.io/managed-by": "pllm"},
		},
	}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("k8s: create namespace: %w", err)
	}
	return nil
}

func (a *AgentK8sAdapter) buildDeployment(req *AgentRequest, ns string) *appsv1.Deployment {
	port := req.Port
	if port == 0 {
		port = defaultAgentPort
	}
	replicas := req.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	labels := agentLabels(req)
	selector := map[string]string{
		"app.kubernetes.io/name":     req.WorkloadName,
		"app.kubernetes.io/instance": req.WorkloadName,
	}

	envVars := make([]corev1.EnvVar, 0, len(req.Env))
	for k, v := range req.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	cpuReq := orDefault(a.cfg.DefaultCPURequest, "100m")
	memReq := orDefault(a.cfg.DefaultMemRequest, "128Mi")
	cpuLim := orDefault(a.cfg.DefaultCPULimit, "1000m")
	memLim := orDefault(a.cfg.DefaultMemLimit, "512Mi")

	container := corev1.Container{
		Name:            "agent",
		Image:           req.Image,
		Command:         req.Command,
		Args:            req.Args,
		ImagePullPolicy: corev1.PullPolicy(orDefault(a.cfg.ImagePullPolicy, "IfNotPresent")),
		Ports: []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		}},
		Env: envVars,
		EnvFrom: []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: req.SecretName},
			},
		}},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuReq),
				corev1.ResourceMemory: resource.MustParse(memReq),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuLim),
				corev1.ResourceMemory: resource.MustParse(memLim),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(port)},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.WorkloadName,
			Namespace:   ns,
			Labels:      labels,
			Annotations: agentAnnotations(req),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      mergeMaps(labels, selector),
					Annotations: agentAnnotations(req),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						RunAsUser:    int64Ptr(10001),
						FSGroup:      int64Ptr(10001),
					},
				},
			},
		},
	}
}

func (a *AgentK8sAdapter) buildService(req *AgentRequest, ns string) *corev1.Service {
	port := req.Port
	if port == 0 {
		port = defaultAgentPort
	}
	selector := map[string]string{
		"app.kubernetes.io/name":     req.WorkloadName,
		"app.kubernetes.io/instance": req.WorkloadName,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.WorkloadName,
			Namespace:   ns,
			Labels:      agentLabels(req),
			Annotations: agentAnnotations(req),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       port,
				TargetPort: intstr.FromInt32(port),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

func agentLabels(req *AgentRequest) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":       req.WorkloadName,
		"app.kubernetes.io/instance":   req.WorkloadName,
		"app.kubernetes.io/managed-by": "pllm",
		"pllm.ai/kind":                 "agent",
	}
	if req.UseCaseID != "" {
		labels["pllm.ai/use-case-id"] = req.UseCaseID
	}
	return labels
}

func agentAnnotations(req *AgentRequest) map[string]string {
	return map[string]string{
		"pllm.ai/resource-name":    req.ResourceName,
		"pllm.ai/resource-version": req.ResourceVersion,
		"pllm.ai/display-name":     req.DisplayName,
		"pllm.ai/deployed-at":      time.Now().UTC().Format(time.RFC3339),
	}
}

func hashAgentRequest(req *AgentRequest) string {
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 5: Run the test — expect pass**

Run: `go test ./internal/services/deployment/ -run TestAgentDeployCreatesDeploymentAndService -v`
Expected: PASS

- [ ] **Step 6: Write the Undeploy + Status tests**

Append to `internal/services/deployment/agent_adapter_test.go`:

```go
func TestAgentUndeployIsIdempotent(t *testing.T) {
	adapter := newTestAgentAdapter()
	ctx := context.Background()
	_, err := adapter.Deploy(ctx, &AgentRequest{
		Namespace:    "pllm-agents",
		WorkloadName: "a",
		Image:        "img:1",
		SecretName:   "a-key",
	})
	require.NoError(t, err)

	row := &models.AgentDeployment{Namespace: "pllm-agents", WorkloadName: "a"}
	require.NoError(t, adapter.Undeploy(ctx, row))
	// Second Undeploy should no-op, not error.
	require.NoError(t, adapter.Undeploy(ctx, row))

	// Deployment gone.
	_, err = adapter.client.AppsV1().Deployments("pllm-agents").Get(ctx, "a", metav1.GetOptions{})
	require.Error(t, err)
}

func TestAgentStatusReportsRunningWhenReady(t *testing.T) {
	adapter := newTestAgentAdapter()
	ctx := context.Background()
	_, err := adapter.Deploy(ctx, &AgentRequest{
		Namespace:    "pllm-agents",
		WorkloadName: "a",
		Image:        "img:1",
		SecretName:   "a-key",
	})
	require.NoError(t, err)

	// Fake client doesn't populate status; patch it manually.
	dep, err := adapter.client.AppsV1().Deployments("pllm-agents").Get(ctx, "a", metav1.GetOptions{})
	require.NoError(t, err)
	dep.Status.ReadyReplicas = *dep.Spec.Replicas
	_, err = adapter.client.AppsV1().Deployments("pllm-agents").UpdateStatus(ctx, dep, metav1.UpdateOptions{})
	require.NoError(t, err)

	rep, err := adapter.Status(ctx, &models.AgentDeployment{Namespace: "pllm-agents", WorkloadName: "a"})
	require.NoError(t, err)
	require.Equal(t, models.DeploymentStatusRunning, rep.Status)
}
```

Import `"github.com/amerfu/pllm/internal/core/models"` at the top of the test file if not already present.

- [ ] **Step 7: Run — expect compile failure, Undeploy + Status undefined**

Run: `go test ./internal/services/deployment/ -run 'TestAgent.*' -v`
Expected: FAIL — `Undeploy`/`Status` undefined on `*AgentK8sAdapter`.

- [ ] **Step 8: Implement Undeploy + Status**

Append to `internal/services/deployment/agent_adapter.go`:

```go
func (a *AgentK8sAdapter) Undeploy(ctx context.Context, d *models.AgentDeployment) error {
	ns := d.Namespace
	name := d.WorkloadName

	propagation := metav1.DeletePropagationForeground
	delOpts := metav1.DeleteOptions{PropagationPolicy: &propagation}

	if err := a.client.AppsV1().Deployments(ns).Delete(ctx, name, delOpts); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("k8s: delete agent deployment: %w", err)
	}
	if err := a.client.CoreV1().Services(ns).Delete(ctx, name, delOpts); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("k8s: delete agent service: %w", err)
	}
	return nil
}

func (a *AgentK8sAdapter) Status(ctx context.Context, d *models.AgentDeployment) (*StatusReport, error) {
	dep, err := a.client.AppsV1().Deployments(d.Namespace).Get(ctx, d.WorkloadName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &StatusReport{Status: models.DeploymentStatusStopped}, nil
		}
		return nil, fmt.Errorf("k8s: get: %w", err)
	}
	ready := dep.Status.ReadyReplicas == *dep.Spec.Replicas
	for _, c := range dep.Status.Conditions {
		if c.Type == appsv1.DeploymentProgressing && c.Status == corev1.ConditionFalse {
			return &StatusReport{
				Status: models.DeploymentStatusFailed,
				Reason: c.Message,
			}, nil
		}
	}
	if ready {
		return &StatusReport{Status: models.DeploymentStatusRunning, Healthy: true}, nil
	}
	return &StatusReport{Status: models.DeploymentStatusDeploying, Reason: "waiting for replicas"}, nil
}
```

- [ ] **Step 9: Run all adapter tests**

Run: `go test ./internal/services/deployment/ -run 'TestAgent.*' -v`
Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/services/deployment/agent_adapter.go internal/services/deployment/agent_adapter_test.go
git commit -m "feat(deployment): AgentK8sAdapter for container-image agent workloads

Parallels K8sAdapter but builds a plain Deployment+Service (no stdio
wrapper), injects plaintext env vars, and mounts a K8s Secret via envFrom
for PLLM_API_KEY. Idempotent Create-or-Update on WorkloadName, Undeploy
tolerant of NotFound."
```

---

## Task 4: Scoped key + K8s Secret helpers

**Files:**
- Create: `internal/services/deployment/agent_secrets.go`
- Create: `internal/services/deployment/agent_secrets_test.go`

- [ ] **Step 1: Write the failing test for key provisioning**

Create `internal/services/deployment/agent_secrets_test.go`:

```go
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
	db, cleanup := testutil.NewTestDatabase(t)
	defer cleanup()

	k8s := fake.NewSimpleClientset()

	useCaseID := uuid.New()
	// Minimal UseCase row so the FK isn't orphaned (some tests read it back).
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

	// Plaintext landed in the Secret under PLLM_API_KEY.
	sec, err := k8s.CoreV1().Secrets("pllm-agents").Get(context.Background(), "test-agent-key-secret", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, res.Plaintext, string(sec.Data["PLLM_API_KEY"]))
}
```

- [ ] **Step 2: Run — expect compile failure**

Run: `go test ./internal/services/deployment/ -run TestProvisionScopedKey -v`
Expected: FAIL — `NewAgentSecretHelper` undefined.

- [ ] **Step 3: Implement the helper**

Create `internal/services/deployment/agent_secrets.go`:

```go
package deployment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"gorm.io/gorm"

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
// already exists with the same name, it's replaced so the key rotates
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

func (h *AgentSecretHelper) upsertSecret(ctx context.Context, ns, name, plaintext string) error {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "pllm"},
		},
		Type:       corev1.SecretTypeOpaque,
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
```

- [ ] **Step 4: Run the test — expect pass**

Run: `go test ./internal/services/deployment/ -run TestProvisionScopedKey -v`
Expected: PASS.

- [ ] **Step 5: Write the Revoke test**

Append to `internal/services/deployment/agent_secrets_test.go`:

```go
func TestRevokeScopedKeyIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDatabase(t)
	defer cleanup()

	k8s := fake.NewSimpleClientset()
	h := NewAgentSecretHelper(db, k8s, nil)

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "uc",
		Title:     "UC",
	}).Error)

	res, err := h.ProvisionScopedKey(context.Background(), ScopedKeyRequest{
		Name:       "rk",
		Namespace:  "pllm-agents",
		SecretName: "rk-sec",
		UseCaseID:  useCaseID,
	})
	require.NoError(t, err)

	require.NoError(t, h.RevokeScopedKey(context.Background(), &res.Key.ID, "pllm-agents", "rk-sec"))
	// Second call — no-op, no error.
	require.NoError(t, h.RevokeScopedKey(context.Background(), &res.Key.ID, "pllm-agents", "rk-sec"))
}
```

- [ ] **Step 6: Run the test — expect pass**

Run: `go test ./internal/services/deployment/ -run TestRevokeScopedKey -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/services/deployment/agent_secrets.go internal/services/deployment/agent_secrets_test.go
git commit -m "feat(deployment): scoped-key provisioning + K8s Secret helper

Creates a UseCase-bound Key row and writes the plaintext into an Opaque
Secret under PLLM_API_KEY. Idempotent on SecretName so redeploy rotates.
Revoke is tolerant of missing rows/secrets."
```

---

## Task 5: Service-layer DeployAgent / UndeployAgent / List / Get / RefreshStatus

**Files:**
- Modify: `internal/services/deployment/service.go`
- Create: `internal/services/deployment/service_agent_test.go`

- [ ] **Step 1: Extend the Service struct with agent dependencies**

Modify `internal/services/deployment/service.go` — replace the existing `Service`/`NewService` with:

```go
// Service manages deployment lifecycles in the DB + delegates to adapters.
type Service struct {
	db              *gorm.DB
	logger          *zap.Logger
	adapter         Adapter
	agentAdapter    AgentAdapter       // optional — agents disabled if nil
	agentSecrets    *AgentSecretHelper // optional — pairs with agentAdapter
	registry        BackendRegistrar
	ns              string
	agentNS         string
	agentEnvDefaults map[string]string // PLLM_BASE_URL, PLLM_MCP_URL; set at construction
}

// NewService constructs the deployment service. Agent support kicks in
// when agentAdapter + agentSecrets + agentEnvDefaults are all provided.
func NewService(
	db *gorm.DB,
	logger *zap.Logger,
	adapter Adapter,
	agentAdapter AgentAdapter,
	agentSecrets *AgentSecretHelper,
	registry BackendRegistrar,
	defaultNS, agentNS string,
	agentEnvDefaults map[string]string,
) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	if agentNS == "" {
		agentNS = "pllm-agents"
	}
	return &Service{
		db:               db,
		logger:           logger,
		adapter:          adapter,
		agentAdapter:     agentAdapter,
		agentSecrets:     agentSecrets,
		registry:         registry,
		ns:               defaultNS,
		agentNS:          agentNS,
		agentEnvDefaults: agentEnvDefaults,
	}
}
```

Callers pass `nil, nil, nil` for agent fields when agents aren't configured — the existing main.go does this in Step 4 of Task 7.

- [ ] **Step 2: Write a failing test for DeployAgent**

Create `internal/services/deployment/service_agent_test.go`:

```go
package deployment

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
)

func TestDeployAgentEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDatabase(t)
	defer cleanup()

	ctx := context.Background()

	// Seed: UseCase + RegistryAgent with an OCI package.
	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "customer-research-approved",
		Slug:      "customer-research-approved",
	}).Error)

	pkgs, err := json.Marshal([]map[string]string{{
		"registry_type": "oci",
		"identifier":    "ghcr.io/acme/sales-research",
		"version":       "0.1.0",
	}})
	require.NoError(t, err)
	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel:   models.BaseModel{ID: agentID},
		Name:        "pllm.io/sales-research",
		Version:     "0.1.0",
		Title:       "Sales Research",
		Description: "demo agent",
		IsLatest:    true,
		Packages:    datatypes.JSON(pkgs),
	}).Error)

	k8s := fake.NewSimpleClientset()
	agentAdapter := NewAgentK8sAdapter(k8s, config.DeploymentK8sConfig{AgentNamespace: "pllm-agents"}, nil)
	secrets := NewAgentSecretHelper(db, k8s, nil)

	svc := NewService(db, nil, nil, agentAdapter, secrets, nil, "pllm-mcp", "pllm-agents",
		map[string]string{"PLLM_BASE_URL": "http://pllm:8080", "PLLM_MCP_URL": "http://pllm:8080/v1/mcp"})

	row, err := svc.DeployAgent(ctx, DeployAgentRequest{
		AgentID:   agentID,
		UseCaseID: useCaseID,
		Replicas:  1,
	})
	require.NoError(t, err)
	require.Equal(t, models.DeploymentStatusRunning, row.Status)
	require.NotNil(t, row.ProvisionedKeyID)
	require.NotEmpty(t, row.SecretName)

	// Key row is UseCase-bound.
	var key models.Key
	require.NoError(t, db.First(&key, "id = ?", *row.ProvisionedKeyID).Error)
	require.NotNil(t, key.UseCaseID)
	require.Equal(t, useCaseID, *key.UseCaseID)

	// Deployment exists in K8s.
	_, err = k8s.AppsV1().Deployments("pllm-agents").Get(ctx, row.WorkloadName, metav1.GetOptions{})
	require.NoError(t, err)

	// Secret exists.
	_, err = k8s.CoreV1().Secrets("pllm-agents").Get(ctx, row.SecretName, metav1.GetOptions{})
	require.NoError(t, err)
}
```

- [ ] **Step 3: Run — expect compile failure (DeployAgent undefined)**

Run: `go test ./internal/services/deployment/ -run TestDeployAgentEndToEnd -v`
Expected: FAIL — `svc.DeployAgent` undefined, `DeployAgentRequest` undefined.

- [ ] **Step 4: Implement the agent service methods**

Append to `internal/services/deployment/service.go`:

```go
// DeployAgentRequest is the normalized input from the admin HTTP handler.
type DeployAgentRequest struct {
	AgentID   uuid.UUID
	UseCaseID uuid.UUID
	Namespace string // optional — defaults to service agentNS
	Replicas  int32
	// Override image if you want to run a specific tag; otherwise picks the
	// first OCI package on the RegistryAgent row.
	ImageOverride string
	// Extra env vars merged on top of the service defaults.
	ExtraEnv map[string]string
}

// DeployAgent resolves the registry agent, provisions a scoped key +
// Secret, and hands off to the agent adapter.
func (s *Service) DeployAgent(ctx context.Context, req DeployAgentRequest) (*models.AgentDeployment, error) {
	if s.agentAdapter == nil || s.agentSecrets == nil {
		return nil, errors.New("agent deployment not configured")
	}
	if req.AgentID == uuid.Nil {
		return nil, errors.New("agent_id required")
	}
	if req.UseCaseID == uuid.Nil {
		return nil, errors.New("use_case_id required")
	}
	// Resolve agent.
	var agent models.RegistryAgent
	if err := s.db.WithContext(ctx).First(&agent, "id = ?", req.AgentID).Error; err != nil {
		return nil, fmt.Errorf("resolve agent: %w", err)
	}
	image := req.ImageOverride
	if image == "" {
		pkg, err := firstOCIPackage(agent.Packages)
		if err != nil {
			return nil, fmt.Errorf("no deployable OCI package on agent %q: %w", agent.Name, err)
		}
		image = pkg
	}

	ns := req.Namespace
	if ns == "" {
		ns = s.agentNS
	}
	workload := agentWorkloadName(&agent)
	secretName := workload + "-key"

	row, err := s.findOrCreateAgentRow(ctx, &agent, req.UseCaseID, ns, workload, secretName, req.Replicas)
	if err != nil {
		return nil, err
	}
	s.updateAgentStatus(ctx, row.ID, models.DeploymentStatusDeploying, "")

	// Provision scoped key first — if this fails there's nothing to tear
	// down on the k8s side yet.
	keyRes, err := s.agentSecrets.ProvisionScopedKey(ctx, ScopedKeyRequest{
		Name:       workload + " (auto)",
		Namespace:  ns,
		SecretName: secretName,
		UseCaseID:  req.UseCaseID,
	})
	if err != nil {
		s.updateAgentStatus(ctx, row.ID, models.DeploymentStatusFailed, err.Error())
		return nil, err
	}
	row.ProvisionedKeyID = &keyRes.Key.ID
	row.SecretName = secretName
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}

	env := map[string]string{}
	for k, v := range s.agentEnvDefaults {
		env[k] = v
	}
	// Look up UseCase name for PLLM_USE_CASE so the agent can print it.
	var uc models.UseCase
	if err := s.db.WithContext(ctx).First(&uc, "id = ?", req.UseCaseID).Error; err == nil {
		env["PLLM_USE_CASE"] = uc.Name
	}
	env["PLLM_AGENT_NAME"] = agent.Name
	for k, v := range req.ExtraEnv {
		env[k] = v
	}

	res, err := s.agentAdapter.Deploy(ctx, &AgentRequest{
		Namespace:       ns,
		WorkloadName:    workload,
		DisplayName:     nonEmpty(agent.Title, agent.Name),
		ResourceName:    agent.Name,
		ResourceVersion: agent.Version,
		Image:           image,
		UseCaseID:       req.UseCaseID.String(),
		SecretName:      secretName,
		Env:             env,
		Replicas:        row.Replicas,
	})
	if err != nil {
		s.updateAgentStatus(ctx, row.ID, models.DeploymentStatusFailed, err.Error())
		// Best-effort cleanup.
		_ = s.agentSecrets.RevokeScopedKey(ctx, &keyRes.Key.ID, ns, secretName)
		return nil, err
	}

	updates := map[string]any{
		"endpoint":        res.Endpoint,
		"adapter_state":   datatypes.JSON(res.AdapterState),
		"last_applied_at": time.Now(),
		"status":          models.DeploymentStatusRunning,
		"status_reason":   "",
	}
	if err := s.db.WithContext(ctx).Model(&models.AgentDeployment{}).
		Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("persist agent deployment: %w", err)
	}
	return s.GetAgent(ctx, row.ID)
}

// UndeployAgent tears down the pod, the Service, the Secret, revokes the
// scoped key, and deletes the row.
func (s *Service) UndeployAgent(ctx context.Context, id uuid.UUID) error {
	if s.agentAdapter == nil || s.agentSecrets == nil {
		return errors.New("agent deployment not configured")
	}
	var row models.AgentDeployment
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return err
	}
	s.updateAgentStatus(ctx, id, models.DeploymentStatusTerminating, "")
	if err := s.agentAdapter.Undeploy(ctx, &row); err != nil {
		s.updateAgentStatus(ctx, id, models.DeploymentStatusFailed, err.Error())
		return err
	}
	if err := s.agentSecrets.RevokeScopedKey(ctx, row.ProvisionedKeyID, row.Namespace, row.SecretName); err != nil {
		s.logger.Warn("agent undeploy: revoke key", zap.Error(err))
	}
	return s.db.WithContext(ctx).Delete(&row).Error
}

func (s *Service) ListAgents(ctx context.Context) ([]models.AgentDeployment, error) {
	var out []models.AgentDeployment
	err := s.db.WithContext(ctx).Order("created_at desc").Find(&out).Error
	return out, err
}

func (s *Service) GetAgent(ctx context.Context, id uuid.UUID) (*models.AgentDeployment, error) {
	var row models.AgentDeployment
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) RefreshAgentStatus(ctx context.Context, id uuid.UUID) (*models.AgentDeployment, error) {
	if s.agentAdapter == nil {
		return nil, errors.New("agent deployment not configured")
	}
	row, err := s.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	rep, err := s.agentAdapter.Status(ctx, row)
	if err != nil {
		return nil, err
	}
	row.Status = rep.Status
	row.StatusReason = rep.Reason
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// --- internals -----------------------------------------------------------

func (s *Service) updateAgentStatus(ctx context.Context, id uuid.UUID, status models.DeploymentStatus, reason string) {
	if err := s.db.WithContext(ctx).Model(&models.AgentDeployment{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "status_reason": reason}).Error; err != nil {
		s.logger.Warn("agent deploy: status update", zap.Error(err))
	}
}

func (s *Service) findOrCreateAgentRow(
	ctx context.Context,
	agent *models.RegistryAgent,
	useCaseID uuid.UUID,
	ns, workload, secretName string,
	replicas int32,
) (*models.AgentDeployment, error) {
	if replicas <= 0 {
		replicas = 1
	}
	var existing models.AgentDeployment
	err := s.db.WithContext(ctx).
		Where("registry_agent_id = ? AND namespace = ?", agent.ID, ns).
		First(&existing).Error
	if err == nil {
		existing.ResourceName = agent.Name
		existing.ResourceVersion = agent.Version
		existing.UseCaseID = useCaseID
		existing.WorkloadName = workload
		existing.SecretName = secretName
		existing.Replicas = replicas
		existing.Status = models.DeploymentStatusPending
		existing.StatusReason = ""
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row := &models.AgentDeployment{
		BaseModel:        models.BaseModel{ID: uuid.New()},
		RegistryAgentID:  agent.ID,
		ResourceName:     agent.Name,
		ResourceVersion:  agent.Version,
		UseCaseID:        useCaseID,
		Platform:         s.agentAdapter.Platform(),
		Namespace:        ns,
		WorkloadName:     workload,
		SecretName:       secretName,
		Replicas:         replicas,
		Status:           models.DeploymentStatusPending,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// agentWorkloadName derives a DNS-1123 workload name from the registry
// agent. Same algorithm as workloadNameFor for servers.
func agentWorkloadName(agent *models.RegistryAgent) string {
	base := agent.Name
	if i := lastIndexSlash(base); i >= 0 {
		base = base[i+1:]
	}
	s := strings.ToLower(base + "-" + agent.Version)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

func lastIndexSlash(s string) int { return strings.LastIndex(s, "/") }

func firstOCIPackage(raw []byte) (string, error) {
	pkgs, err := decodePackages(raw)
	if err != nil {
		return "", err
	}
	for _, p := range pkgs {
		t := strings.ToLower(p.RegistryType)
		if t == "oci" || t == "docker" {
			return fmt.Sprintf("%s:%s", p.Identifier, p.Version), nil
		}
	}
	return "", errors.New("need oci package")
}
```

Ensure the `strings` import is present at the top of `service.go` (it already is). Also add `"gorm.io/datatypes"` if not present.

- [ ] **Step 5: Run the test — expect pass**

Run: `go test ./internal/services/deployment/ -run TestDeployAgentEndToEnd -v`
Expected: PASS

- [ ] **Step 6: Add an UndeployAgent test**

Append to `service_agent_test.go`:

```go
func TestUndeployAgentCleansUpKeyAndSecret(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDatabase(t)
	defer cleanup()
	ctx := context.Background()

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "uc", Slug: "uc",
	}).Error)
	pkgs, err := json.Marshal([]map[string]string{{
		"registry_type": "oci",
		"identifier":    "ghcr.io/acme/a",
		"version":       "0.1.0",
	}})
	require.NoError(t, err)
	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel: models.BaseModel{ID: agentID},
		Name:      "pllm.io/a", Version: "0.1.0",
		Packages: datatypes.JSON(pkgs),
	}).Error)

	k8s := fake.NewSimpleClientset()
	svc := NewService(db, nil, nil,
		NewAgentK8sAdapter(k8s, config.DeploymentK8sConfig{AgentNamespace: "pllm-agents"}, nil),
		NewAgentSecretHelper(db, k8s, nil),
		nil, "pllm-mcp", "pllm-agents",
		map[string]string{"PLLM_BASE_URL": "http://pllm:8080"})

	row, err := svc.DeployAgent(ctx, DeployAgentRequest{AgentID: agentID, UseCaseID: useCaseID})
	require.NoError(t, err)
	keyID := *row.ProvisionedKeyID
	secretName := row.SecretName

	require.NoError(t, svc.UndeployAgent(ctx, row.ID))

	// Key row is gone (or soft-deleted — Key uses BaseModel which has gorm.DeletedAt).
	var count int64
	require.NoError(t, db.Unscoped().Model(&models.Key{}).Where("id = ?", keyID).Count(&count).Error)
	// soft-deleted still counts here; do a live query with DeletedAt IS NULL to confirm it's gone.
	var liveCount int64
	require.NoError(t, db.Model(&models.Key{}).Where("id = ?", keyID).Count(&liveCount).Error)
	require.Equal(t, int64(0), liveCount)

	// Secret is gone.
	_, err = k8s.CoreV1().Secrets("pllm-agents").Get(ctx, secretName, metav1.GetOptions{})
	require.Error(t, err)
}
```

- [ ] **Step 7: Run — expect pass**

Run: `go test ./internal/services/deployment/ -run TestUndeployAgent -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/services/deployment/service.go internal/services/deployment/service_agent_test.go
git commit -m "feat(deployment): agent service methods (Deploy/Undeploy/List/Get/RefreshStatus)

Orchestrates key provisioning → Secret → K8s apply for DeployAgent, and
reverses that order for UndeployAgent. Key rollback on adapter failure so
we don't orphan scoped keys in the DB."
```

---

## Task 6: HTTP handler + admin router wiring

**Files:**
- Create: `internal/api/handlers/deployment/agent_handler.go`
- Create: `internal/api/handlers/deployment/agent_handler_test.go`
- Modify: `internal/api/router/admin.go`

- [ ] **Step 1: Write the handler**

Create `internal/api/handlers/deployment/agent_handler.go`:

```go
package deployment

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/amerfu/pllm/internal/services/deployment"
)

// AgentHandler exposes /api/admin/agents.
type AgentHandler struct {
	logger  *zap.Logger
	service *deployment.Service
}

func NewAgentHandler(logger *zap.Logger, svc *deployment.Service) *AgentHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentHandler{logger: logger, service: svc}
}

func (h *AgentHandler) ready() bool { return h.service != nil }

type agentDeployRequest struct {
	AgentID       uuid.UUID         `json:"agent_id"`
	UseCaseID     uuid.UUID         `json:"use_case_id"`
	Namespace     string            `json:"namespace,omitempty"`
	Replicas      int32             `json:"replicas,omitempty"`
	ImageOverride string            `json:"image_override,omitempty"`
	ExtraEnv      map[string]string `json:"extra_env,omitempty"`
}

// Deploy is POST /api/admin/agents.
func (h *AgentHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	var body agentDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.AgentID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if body.UseCaseID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "use_case_id is required")
		return
	}
	row, err := h.service.DeployAgent(r.Context(), deployment.DeployAgentRequest{
		AgentID:       body.AgentID,
		UseCaseID:     body.UseCaseID,
		Namespace:     body.Namespace,
		Replicas:      body.Replicas,
		ImageOverride: body.ImageOverride,
		ExtraEnv:      body.ExtraEnv,
	})
	if err != nil {
		if errors.Is(err, deployment.ErrAgentNotConfigured) {
			writeErr(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// List is GET /api/admin/agents.
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	rows, err := h.service.ListAgents(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": rows})
}

// Get is GET /api/admin/agents/{id}.
func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.service.GetAgent(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "agent deployment not found")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// Delete is DELETE /api/admin/agents/{id}.
func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.service.UndeployAgent(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RefreshStatus is POST /api/admin/agents/{id}/status.
func (h *AgentHandler) RefreshStatus(w http.ResponseWriter, r *http.Request) {
	if !h.ready() {
		writeErr(w, http.StatusServiceUnavailable, "agent deployment not configured")
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	row, err := h.service.RefreshAgentStatus(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, row)
}
```

- [ ] **Step 2: Add the sentinel error used above**

Add to `internal/services/deployment/service.go` (top, near existing vars):

```go
var ErrAgentNotConfigured = errors.New("agent deployment not configured")
```

Replace `return nil, errors.New("agent deployment not configured")` inside `DeployAgent`/`UndeployAgent`/`RefreshAgentStatus` with `return nil, ErrAgentNotConfigured`. (Use `return ErrAgentNotConfigured` for the one that returns `error` only.)

- [ ] **Step 3: Write a handler test**

Create `internal/api/handlers/deployment/agent_handler_test.go`:

```go
package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	deploysvc "github.com/amerfu/pllm/internal/services/deployment"
)

func TestAgentHandlerDeploy(t *testing.T) {
	if testing.Short() {
		t.Skip("needs testcontainer postgres")
	}
	db, cleanup := testutil.NewTestDatabase(t)
	defer cleanup()

	useCaseID := uuid.New()
	require.NoError(t, db.Create(&models.UseCase{
		BaseModel: models.BaseModel{ID: useCaseID},
		Name:      "uc", Slug: "uc",
	}).Error)
	pkgs, _ := json.Marshal([]map[string]string{{
		"registry_type": "oci",
		"identifier":    "ghcr.io/acme/a",
		"version":       "0.1.0",
	}})
	agentID := uuid.New()
	require.NoError(t, db.Create(&models.RegistryAgent{
		BaseModel: models.BaseModel{ID: agentID},
		Name:      "pllm.io/a", Version: "0.1.0",
		Packages: datatypes.JSON(pkgs),
	}).Error)

	k8s := fake.NewSimpleClientset()
	svc := deploysvc.NewService(db, nil, nil,
		deploysvc.NewAgentK8sAdapter(k8s, config.DeploymentK8sConfig{AgentNamespace: "pllm-agents"}, nil),
		deploysvc.NewAgentSecretHelper(db, k8s, nil),
		nil, "pllm-mcp", "pllm-agents",
		map[string]string{"PLLM_BASE_URL": "http://pllm:8080"})

	h := NewAgentHandler(nil, svc)
	body, _ := json.Marshal(map[string]any{
		"agent_id":    agentID,
		"use_case_id": useCaseID,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/agents", bytes.NewReader(body)).
		WithContext(context.Background())
	rec := httptest.NewRecorder()
	h.Deploy(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	// K8s-side artifacts exist.
	deps, err := k8s.AppsV1().Deployments("pllm-agents").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, deps.Items, 1)
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/api/handlers/deployment/ -run TestAgentHandlerDeploy -v`
Expected: PASS

- [ ] **Step 5: Wire the handler into AdminRouterConfig**

Modify `internal/api/router/admin.go`:

- Inside `AdminRouterConfig`, add the field after `DeploymentHandler`:

```go
	DeploymentHandler     *deploymenthandlers.Handler
	AgentDeployHandler    *deploymenthandlers.AgentHandler
	UseCaseHandler        *usecasehandlers.Handler
```

- Inside the `if cfg.DeploymentHandler != nil { ... }` block (around line 297-307), right after the closing brace of that block, add:

```go
			// Agent deploy routes.
			if cfg.AgentDeployHandler != nil {
				ah := cfg.AgentDeployHandler
				r.Route("/agents", func(rr chi.Router) {
					rr.Get("/", ah.List)
					rr.Post("/", ah.Deploy)
					rr.Get("/{id}", ah.Get)
					rr.Delete("/{id}", ah.Delete)
					rr.Post("/{id}/status", ah.RefreshStatus)
				})
			}
```

- [ ] **Step 6: Build + run admin tests**

Run: `go build ./... && go test ./internal/api/router/ -short`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/api/handlers/deployment/agent_handler.go \
        internal/api/handlers/deployment/agent_handler_test.go \
        internal/api/router/admin.go \
        internal/services/deployment/service.go
git commit -m "feat(admin): HTTP surface for agent deployments

POST/GET/DELETE /api/admin/agents + POST /api/admin/agents/{id}/status.
Gated off when AgentDeployHandler is nil (agent deploy feature disabled)."
```

---

## Task 7: Router wiring (main router assembles the deployment service)

**Files:**
- Modify: `internal/api/router/router.go`
- Modify: `internal/services/deployment/kubernetes.go`

**Context:** The deployment service is constructed in `router.go` around line 332–353 (search for `// Deployment service — opt-in`), not in `main.go`. We extend that block to also build the agent adapter, secret helper, and handler.

- [ ] **Step 1: Expose Client() on K8sAdapter so we can share the kubernetes client**

Add to `internal/services/deployment/kubernetes.go` right after the `NewK8sAdapterWithClient` function (around line 67):

```go
// Client returns the underlying Kubernetes client. Used by sibling
// adapters (AgentK8sAdapter) so they don't each re-parse kubeconfig.
func (a *K8sAdapter) Client() kubernetes.Interface { return a.client }
```

- [ ] **Step 2: Replace the existing deployment-wiring block in router.go**

Find the block starting at line 332 (`// Deployment service — opt-in via config.deployment.enabled.`). Replace the whole `if db != nil && cfg.Deployment.Enabled { ... }` body with:

```go
	// Deployment service — opt-in via config.deployment.enabled. Falls
	// back gracefully if credentials can't be loaded. Depends on mcpManager
	// so it must be initialized after MCP setup.
	var deployHandler *deploymenthandlers.Handler
	var agentDeployHandler *deploymenthandlers.AgentHandler
	if db != nil && cfg.Deployment.Enabled {
		adapter, err := deploymentsvc.NewK8sAdapter(cfg.Deployment.K8s, logger)
		if err != nil {
			logger.Warn("Deployment disabled: K8s client unavailable", zap.Error(err))
		} else {
			bridge := deploymentsvc.NewMCPBridge(db, mcpManager, logger)
			agentAdapter := deploymentsvc.NewAgentK8sAdapter(adapter.Client(), cfg.Deployment.K8s, logger)
			agentSecrets := deploymentsvc.NewAgentSecretHelper(db, adapter.Client(), logger)

			agentEnv := map[string]string{
				"PLLM_BASE_URL": fmt.Sprintf("http://pllm.%s.svc.cluster.local:%d", cfg.Deployment.K8s.Namespace, cfg.Server.Port),
				"PLLM_MCP_URL":  fmt.Sprintf("http://pllm.%s.svc.cluster.local:%d/v1/mcp", cfg.Deployment.K8s.Namespace, cfg.Server.Port),
			}

			deploySvc := deploymentsvc.NewService(db, logger,
				adapter,
				agentAdapter,
				agentSecrets,
				bridge,
				cfg.Deployment.K8s.Namespace,
				cfg.Deployment.K8s.AgentNamespace,
				agentEnv,
			)
			serverSvc := registryservice.NewServerService(db, logger)
			deployHandler = deploymenthandlers.NewHandler(logger, deploySvc, serverSvc)
			agentDeployHandler = deploymenthandlers.NewAgentHandler(logger, deploySvc)
			logger.Info("Deployment service initialized",
				zap.String("namespace", cfg.Deployment.K8s.Namespace),
				zap.String("agent_namespace", cfg.Deployment.K8s.AgentNamespace),
				zap.Bool("in_cluster", cfg.Deployment.K8s.InCluster))
			// Reapply orphaned deployments in the background so router
			// startup isn't blocked by K8s round-trips. A skaffold dev
			// restart that wipes the namespace will converge back here.
			go deploySvc.ReconcileAll(context.Background())
		}
	}
```

Make sure `"fmt"` is already imported at the top of `router.go` — it usually is. If it isn't, add it.

- [ ] **Step 3: Pass the new handler into AdminRouterConfig**

Find the `AdminRouterConfig{ ... }` literal (around line 780). Add the new field:

```go
			DeploymentHandler:     deployHandler,
			AgentDeployHandler:    agentDeployHandler,
```

- [ ] **Step 4: Build everything**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/api/router/router.go internal/services/deployment/kubernetes.go
git commit -m "feat(router): wire agent adapter + handler into startup

Shares the Kubernetes client with K8sAdapter via Client() so we don't
re-parse kubeconfig. Defaults PLLM_BASE_URL / PLLM_MCP_URL to the
in-cluster service DNS; operators can override per-deploy via ExtraEnv."
```

---

## Task 8: Frontend — Agents page (read-only list + delete)

**Files:**
- Modify: `web/src/types/api.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/icons.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/app-sidebar.tsx`
- Create: `web/src/pages/Agents.tsx`

- [ ] **Step 1: Add types**

Append to `web/src/types/api.ts`:

```ts
export interface AgentDeployment {
  id: string;
  registry_agent_id: string;
  resource_name: string;
  resource_version: string;
  use_case_id: string;
  platform: string;
  namespace: string;
  workload_name: string;
  replicas: number;
  status: DeploymentStatus;
  status_reason?: string;
  endpoint?: string;
  provisioned_key_id?: string;
  secret_name?: string;
  created_at: string;
  updated_at: string;
  last_applied_at?: string;
}

export interface DeployAgentRequest {
  agent_id: string;
  use_case_id: string;
  namespace?: string;
  replicas?: number;
  image_override?: string;
  extra_env?: Record<string, string>;
}
```

- [ ] **Step 2: Add API methods**

Append to `web/src/lib/api.ts`:

```ts
export async function listAgents(): Promise<{ agents: AgentDeployment[] }> {
  const { data } = await apiClient.get("/api/admin/agents");
  return data;
}

export async function deployAgent(req: DeployAgentRequest): Promise<AgentDeployment> {
  const { data } = await apiClient.post("/api/admin/agents", req);
  return data;
}

export async function deleteAgent(id: string): Promise<void> {
  await apiClient.delete(`/api/admin/agents/${id}`);
}

export async function refreshAgentStatus(id: string): Promise<AgentDeployment> {
  const { data } = await apiClient.post(`/api/admin/agents/${id}/status`);
  return data;
}
```

Make sure `AgentDeployment` and `DeployAgentRequest` are imported at the top of `api.ts` from `@/types/api` (follow the existing pattern).

- [ ] **Step 3: Add the icon**

Modify `web/src/lib/icons.ts` — add an entry under the exported icons object:

```ts
agents: "lucide:bot",
```

- [ ] **Step 4: Create the page**

Create `web/src/pages/Agents.tsx`:

```tsx
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { listAgents, deleteAgent, refreshAgentStatus } from "@/lib/api";
import type { AgentDeployment } from "@/types/api";

import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { PageShell } from "@/components/common/page-shell";
import { useToast } from "@/hooks/use-toast";

export default function Agents() {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState<AgentDeployment | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["agent-deployments"],
    queryFn: listAgents,
    refetchInterval: 5_000,
  });
  const agents: AgentDeployment[] = data?.agents ?? [];

  const refreshMutation = useMutation({
    mutationFn: (id: string) => refreshAgentStatus(id),
    onSuccess: (a) => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({ title: `Status: ${a.status}`, description: a.status_reason });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteAgent(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      setConfirmDelete(null);
      toast({ title: "Agent undeployed" });
    },
  });

  return (
    <PageShell
      eyebrow="Runtime"
      title={<>Agents</>}
      description="Deployed agent workloads, scoped to approved use cases."
      icon={<Icon icon={icons.agents} className="size-5 text-muted-foreground" />}
    >
      {isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : agents.length === 0 ? (
        <EmptyState
          icon={icons.agents}
          title="No agents deployed"
          description="Publish an agent to the registry, then deploy it via the admin API."
        />
      ) : (
        <div className="divide-y rounded-md border">
          {agents.map((a) => (
            <div key={a.id} className="flex items-center justify-between gap-4 px-4 py-3">
              <div className="min-w-0">
                <div className="truncate font-medium">{a.resource_name}</div>
                <div className="truncate text-xs text-muted-foreground">
                  v{a.resource_version} · {a.namespace}/{a.workload_name} ·{" "}
                  {formatDistanceToNow(new Date(a.created_at), { addSuffix: true })}
                </div>
              </div>
              <div className="flex items-center gap-3">
                <span
                  className={
                    a.status === "running"
                      ? "rounded bg-emerald-50 px-2 py-0.5 text-xs font-mono text-emerald-700"
                      : "rounded bg-slate-100 px-2 py-0.5 text-xs font-mono text-slate-600"
                  }
                >
                  {a.status}
                </span>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button size="sm" variant="outline">…</Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => refreshMutation.mutate(a.id)}>
                      Refresh status
                    </DropdownMenuItem>
                    <DropdownMenuItem className="text-rose-600" onClick={() => setConfirmDelete(a)}>
                      Undeploy
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          ))}
        </div>
      )}

      <AlertDialog open={!!confirmDelete} onOpenChange={(o) => !o && setConfirmDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Undeploy this agent?</AlertDialogTitle>
            <AlertDialogDescription>
              The pod, Service, and scoped-key Secret will be removed. The scoped API key is revoked.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => confirmDelete && deleteMutation.mutate(confirmDelete.id)}
            >
              Undeploy
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}
```

- [ ] **Step 5: Register the route**

Modify `web/src/App.tsx` — add a route following the existing pattern (mirror `Deployments`):

```tsx
const Agents = lazy(() => import("./pages/Agents"));
```

And inside the `<Routes>`:

```tsx
<Route path="/agents" element={<Agents />} />
```

- [ ] **Step 6: Sidebar entry**

Modify `web/src/components/app-sidebar.tsx` — add a nav item next to Deployments:

```tsx
{ title: "Agents", href: "/agents", icon: icons.agents },
```

(Adjust to match whatever the existing sidebar item shape is.)

- [ ] **Step 7: Run the frontend lint + build**

Run: `cd web && npm run lint && npm run build`
Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add web/src/types/api.ts web/src/lib/api.ts web/src/lib/icons.ts \
        web/src/pages/Agents.tsx web/src/App.tsx \
        web/src/components/app-sidebar.tsx
git commit -m "feat(ui): read-only Agents page

List deployed agents, refresh their status, undeploy with confirmation.
Deploy action is admin-API-only for Phase 1 — dialog lands in Phase 3."
```

---

## Task 9: Smoke test script

**Files:**
- Create: `scripts/test-agent-onboarding.sh`

- [ ] **Step 1: Write the script**

Create `scripts/test-agent-onboarding.sh`:

```bash
#!/usr/bin/env bash
# End-to-end smoke test for Phase 1 agent onboarding.
#
# Pre-reqs:
#   - pllm running at $PLLM_URL (default http://localhost:8080)
#   - a Kubernetes cluster the configured deployment.kubernetes can reach
#   - the master key set in $PLLM_MASTER_KEY (default: "sk-master-test")
#
# Prints FAIL + exits non-zero on the first failing step.

set -euo pipefail
PLLM_URL="${PLLM_URL:-http://localhost:8080}"
PLLM_MASTER_KEY="${PLLM_MASTER_KEY:-sk-master-test}"
AUTH=(-H "Authorization: Bearer ${PLLM_MASTER_KEY}")

step() { printf "\n== %s ==\n" "$1"; }

step "Health"
curl -fsS "${PLLM_URL}/health" >/dev/null

step "Create a minimal UseCase"
UC_ID=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/api/admin/use-cases" \
  -d '{"name":"smoke-agent-uc","slug":"smoke-agent-uc","description":"Phase 1 smoke"}' | jq -r .id)
echo "use_case_id=${UC_ID}"

step "Publish a trivial registry agent (OCI image: ghcr.io/pllm/agent-smoke:0.1)"
AGENT_ID=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/api/admin/registry/agents" \
  -d '{
    "name":"test.pllm/agent-smoke",
    "version":"0.1.0",
    "title":"Smoke Agent",
    "description":"Phase 1 smoke",
    "packages":[{"registry_type":"oci","identifier":"ghcr.io/pllm/agent-smoke","version":"0.1"}]
  }' | jq -r .id)
echo "agent_id=${AGENT_ID}"

step "Deploy the agent"
DEPLOY=$(curl -fsS "${AUTH[@]}" -H 'Content-Type: application/json' \
  "${PLLM_URL}/api/admin/agents" \
  -d "{\"agent_id\":\"${AGENT_ID}\",\"use_case_id\":\"${UC_ID}\"}")
AD_ID=$(echo "${DEPLOY}" | jq -r .id)
SECRET=$(echo "${DEPLOY}" | jq -r .secret_name)
KEY_ID=$(echo "${DEPLOY}" | jq -r .provisioned_key_id)
echo "agent_deployment_id=${AD_ID} secret_name=${SECRET} provisioned_key_id=${KEY_ID}"

step "List agents — expect our deployment"
curl -fsS "${AUTH[@]}" "${PLLM_URL}/api/admin/agents" | jq -e \
  --arg id "${AD_ID}" '.agents | map(select(.id==$id)) | length == 1' >/dev/null

step "Refresh status"
curl -fsS "${AUTH[@]}" -X POST "${PLLM_URL}/api/admin/agents/${AD_ID}/status" | jq .status

step "Undeploy"
curl -fsS "${AUTH[@]}" -X DELETE "${PLLM_URL}/api/admin/agents/${AD_ID}"

step "Re-list — expect absent"
curl -fsS "${AUTH[@]}" "${PLLM_URL}/api/admin/agents" | jq -e \
  --arg id "${AD_ID}" '.agents | map(select(.id==$id)) | length == 0' >/dev/null

echo "OK"

if [[ "${1:-}" == "--cleanup" ]]; then
  step "Cleanup test fixtures"
  curl -fsS "${AUTH[@]}" -X DELETE "${PLLM_URL}/api/admin/registry/agents/${AGENT_ID}" || true
  curl -fsS "${AUTH[@]}" -X DELETE "${PLLM_URL}/api/admin/use-cases/${UC_ID}" || true
fi
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x scripts/test-agent-onboarding.sh`

- [ ] **Step 3: Document it in the repo**

Modify `CLAUDE.md` — in the testing section, right after the existing `bash scripts/test-mcp-registry.sh` line (around line 87), add:

```bash
bash scripts/test-agent-onboarding.sh            # Phase 1 agent deploy smoke
bash scripts/test-agent-onboarding.sh --cleanup  # + delete test fixtures
```

- [ ] **Step 4: Commit**

```bash
git add scripts/test-agent-onboarding.sh CLAUDE.md
git commit -m "chore(scripts): Phase 1 agent onboarding smoke test

Covers health → UseCase create → registry agent publish → deploy → list
→ status refresh → undeploy → list-absent. Pass --cleanup to drop the
test fixtures when done."
```

---

## Completion checklist

After every task above has been committed, verify:

- [ ] `go test ./... -short` passes
- [ ] `go test ./internal/services/deployment/...` (with testcontainer DB) passes
- [ ] `cd web && npm run build` passes
- [ ] `scripts/test-agent-onboarding.sh` runs green against a dev pllm + minikube
- [ ] Agents page renders in the UI; deleting a row removes the K8s Deployment + Service + Secret

Then **milestone reached**: a hand-written LangGraph agent packaged as an OCI image can be published, deployed, seen running, and undeployed — with a UseCase-bound key + Secret lifecycle handled automatically. Scope enforcement from `ScopeResolver` already gates every `/v1/chat/completions` and `/v1/mcp` call the agent makes, because the scoped key's `UseCaseID` flows through the existing middleware.
