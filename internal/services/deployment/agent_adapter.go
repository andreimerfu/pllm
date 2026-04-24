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

// NewAgentK8sAdapter constructs an AgentK8sAdapter with the given Kubernetes
// client, config, and logger. A nil logger falls back to a no-op logger.
func NewAgentK8sAdapter(client kubernetes.Interface, cfg config.DeploymentK8sConfig, logger *zap.Logger) *AgentK8sAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentK8sAdapter{client: client, cfg: cfg, logger: logger}
}

// Platform implements AgentAdapter.
func (a *AgentK8sAdapter) Platform() models.DeploymentPlatform {
	return models.DeploymentPlatformKubernetes
}

const defaultAgentPort int32 = 8000

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
	if err := a.ensureAgentNamespace(ctx, ns); err != nil {
		return nil, err
	}

	deploy := a.buildAgentDeployment(req, ns)
	svc := a.buildAgentService(req, ns)

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

// Undeploy implements AgentAdapter. Deletes all objects created for this
// agent. No-ops on missing objects so a partial Deploy still cleans up.
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

// Status implements AgentAdapter. Reads the Deployment and reports health.
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

// --- internal ------------------------------------------------------------

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

func (a *AgentK8sAdapter) ensureAgentNamespace(ctx context.Context, ns string) error {
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

func (a *AgentK8sAdapter) buildAgentDeployment(req *AgentRequest, ns string) *appsv1.Deployment {
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

func (a *AgentK8sAdapter) buildAgentService(req *AgentRequest, ns string) *corev1.Service {
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

// agentLabels returns the standard label set for an agent workload.
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

// agentAnnotations returns the standard annotation set for an agent workload.
func agentAnnotations(req *AgentRequest) map[string]string {
	return map[string]string{
		"pllm.ai/resource-name":    req.ResourceName,
		"pllm.ai/resource-version": req.ResourceVersion,
		"pllm.ai/display-name":     req.DisplayName,
		"pllm.ai/deployed-at":      time.Now().UTC().Format(time.RFC3339),
	}
}

// hashAgentRequest produces a deterministic content hash of the request for
// change-detection in AdapterState.
func hashAgentRequest(req *AgentRequest) string {
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
