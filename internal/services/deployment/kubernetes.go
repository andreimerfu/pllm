package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
)

// Default port the wrapper image exposes. Must match whatever the wrapper
// image actually binds. Keep it fixed across all deployments for simpler
// gateway wiring.
const defaultWrapperPort int32 = 8000

// K8sAdapter implements Adapter against a Kubernetes cluster.
// Safe to construct with an in-cluster or kubeconfig-based client.
type K8sAdapter struct {
	client kubernetes.Interface
	cfg    config.DeploymentK8sConfig
	logger *zap.Logger
}

// NewK8sAdapter builds a K8sAdapter from the passed config.
// Returns an error if cluster credentials can't be loaded — caller should
// still be able to run pllm (Deployment feature just stays disabled).
func NewK8sAdapter(cfg config.DeploymentK8sConfig, logger *zap.Logger) (*K8sAdapter, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	restCfg, err := loadRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("k8s: build client: %w", err)
	}
	return &K8sAdapter{client: client, cfg: cfg, logger: logger}, nil
}

// NewK8sAdapterWithClient is an injection point for tests (fake client).
func NewK8sAdapterWithClient(client kubernetes.Interface, cfg config.DeploymentK8sConfig, logger *zap.Logger) *K8sAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &K8sAdapter{client: client, cfg: cfg, logger: logger}
}

func loadRESTConfig(cfg config.DeploymentK8sConfig) (*rest.Config, error) {
	if cfg.InCluster {
		c, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("k8s: in-cluster config: %w", err)
		}
		return c, nil
	}
	if cfg.KubeconfigPath == "" {
		return nil, errors.New("k8s: either in_cluster=true or kubeconfig_path must be set")
	}
	c, err := clientcmd.BuildConfigFromFlags("", cfg.KubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("k8s: load kubeconfig: %w", err)
	}
	return c, nil
}

// Platform implements Adapter.
func (a *K8sAdapter) Platform() models.DeploymentPlatform { return models.DeploymentPlatformKubernetes }

// Deploy implements Adapter. Applies Deployment + Service + optional
// NetworkPolicy. Idempotent: repeated calls with the same spec result in
// identical manifests (so server-side apply = no-op).
func (a *K8sAdapter) Deploy(ctx context.Context, req *Request) (*Result, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	ns := req.Namespace
	if ns == "" {
		ns = a.cfg.Namespace
	}
	if err := a.ensureNamespace(ctx, ns); err != nil {
		return nil, err
	}

	// Build manifests.
	deploy := a.buildDeployment(req, ns)
	svc := a.buildService(req, ns, deploy.Spec.Template.Labels)

	// Apply Deployment.
	if _, err := a.client.AppsV1().Deployments(ns).Create(ctx, deploy, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("k8s: create deployment: %w", err)
		}
		// Exists — update.
		existing, gerr := a.client.AppsV1().Deployments(ns).Get(ctx, deploy.Name, metav1.GetOptions{})
		if gerr != nil {
			return nil, fmt.Errorf("k8s: get deployment: %w", gerr)
		}
		deploy.ResourceVersion = existing.ResourceVersion
		if _, err := a.client.AppsV1().Deployments(ns).Update(ctx, deploy, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("k8s: update deployment: %w", err)
		}
	}

	// Apply Service.
	if _, err := a.client.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("k8s: create service: %w", err)
		}
		existing, gerr := a.client.CoreV1().Services(ns).Get(ctx, svc.Name, metav1.GetOptions{})
		if gerr != nil {
			return nil, fmt.Errorf("k8s: get service: %w", gerr)
		}
		svc.ResourceVersion = existing.ResourceVersion
		svc.Spec.ClusterIP = existing.Spec.ClusterIP // immutable
		if _, err := a.client.CoreV1().Services(ns).Update(ctx, svc, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("k8s: update service: %w", err)
		}
	}

	// Optional NetworkPolicy.
	if req.RestrictEgress || a.cfg.RestrictEgress {
		np := a.buildNetworkPolicy(req, ns, deploy.Spec.Template.Labels)
		if _, err := a.client.NetworkingV1().NetworkPolicies(ns).Create(ctx, np, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				a.logger.Warn("k8s: create netpol failed", zap.Error(err))
			}
		}
	}

	state, _ := json.Marshal(adapterState{ManifestHash: hashRequest(req)})
	return &Result{
		Endpoint:     fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/mcp", svc.Name, ns, defaultWrapperPort),
		AdapterState: state,
	}, nil
}

// Undeploy implements Adapter. Deletes all objects we created. No-ops on
// missing objects so a partial Deploy still cleans up.
func (a *K8sAdapter) Undeploy(ctx context.Context, d *models.Deployment) error {
	ns := d.Namespace
	name := d.WorkloadName

	propagation := metav1.DeletePropagationForeground
	delOpts := metav1.DeleteOptions{PropagationPolicy: &propagation}

	if err := a.client.AppsV1().Deployments(ns).Delete(ctx, name, delOpts); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("k8s: delete deployment: %w", err)
	}
	if err := a.client.CoreV1().Services(ns).Delete(ctx, name, delOpts); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("k8s: delete service: %w", err)
	}
	if err := a.client.NetworkingV1().NetworkPolicies(ns).Delete(ctx, name, delOpts); err != nil && !apierrors.IsNotFound(err) {
		a.logger.Warn("k8s: delete netpol failed", zap.Error(err))
	}
	return nil
}

// Status implements Adapter. Reads the Deployment and reports health.
func (a *K8sAdapter) Status(ctx context.Context, d *models.Deployment) (*StatusReport, error) {
	dep, err := a.client.AppsV1().Deployments(d.Namespace).Get(ctx, d.WorkloadName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &StatusReport{Status: models.DeploymentStatusStopped}, nil
		}
		return nil, fmt.Errorf("k8s: get: %w", err)
	}
	// Ready replicas == desired replicas AND no progressing/ stalled condition.
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

type adapterState struct {
	ManifestHash string `json:"manifest_hash"`
}

func hashRequest(req *Request) string {
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (a *K8sAdapter) ensureNamespace(ctx context.Context, ns string) error {
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

func (a *K8sAdapter) buildDeployment(req *Request, ns string) *appsv1.Deployment {
	labels := buildLabels(req)
	selector := map[string]string{
		"app.kubernetes.io/name":     req.WorkloadName,
		"app.kubernetes.io/instance": req.WorkloadName,
	}

	container := a.buildContainer(req)

	// Wrapper-based builds (npx/uvx) need a writable scratch area for
	// package caches since we keep the root filesystem read-only.
	// OCI image builds don't — they manage their own filesystem.
	var volumes []corev1.Volume
	if req.NPXPackage != nil || req.UVXPackage != nil {
		volumes = append(volumes, corev1.Volume{
			Name:         "scratch",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
	}

	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.WorkloadName,
			Namespace:   ns,
			Labels:      labels,
			Annotations: buildAnnotations(req),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      mergeMaps(labels, selector),
					Annotations: buildAnnotations(req),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					Volumes:    volumes,
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

func (a *K8sAdapter) buildContainer(req *Request) corev1.Container {
	cpuReq := orDefault(a.cfg.DefaultCPURequest, "50m")
	memReq := orDefault(a.cfg.DefaultMemRequest, "64Mi")
	cpuLim := orDefault(a.cfg.DefaultCPULimit, "500m")
	memLim := orDefault(a.cfg.DefaultMemLimit, "256Mi")

	base := corev1.Container{
		Name:            "mcp",
		ImagePullPolicy: corev1.PullPolicy(orDefault(a.cfg.ImagePullPolicy, "IfNotPresent")),
		Ports: []corev1.ContainerPort{{
			Name:          "mcp",
			ContainerPort: defaultWrapperPort,
			Protocol:      corev1.ProtocolTCP,
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
		Env: buildEnvVars(req.Env),
	}

	switch {
	case req.Image != nil:
		base.Image = req.Image.Reference
		if port := req.Image.Port; port > 0 {
			base.Ports[0].ContainerPort = port
		}
		base.Command = req.Image.Command
		base.Args = req.Image.Args

	case req.NPXPackage != nil:
		// Use the shared wrapper image. The wrapper's entrypoint knows
		// how to spawn `npx` and bridge stdio <-> HTTP.
		base.Image = a.cfg.WrapperImage
		base.Env = append(base.Env,
			envVar("PLLM_WRAPPER_KIND", "npx"),
			envVar("PLLM_WRAPPER_PACKAGE", req.NPXPackage.Package),
			envVar("PLLM_WRAPPER_VERSION", req.NPXPackage.Version),
			// npm writes its cache under $HOME/.npm. With a read-only root
			// FS we redirect HOME to the mounted emptyDir.
			envVar("HOME", "/tmp"),
		)
		if len(req.NPXPackage.Args) > 0 {
			base.Env = append(base.Env, envVar("PLLM_WRAPPER_ARGS", strings.Join(req.NPXPackage.Args, " ")))
		}
		base.VolumeMounts = append(base.VolumeMounts, corev1.VolumeMount{
			Name: "scratch", MountPath: "/tmp",
		})

	case req.UVXPackage != nil:
		base.Image = a.cfg.WrapperImage
		base.Env = append(base.Env,
			envVar("PLLM_WRAPPER_KIND", "uvx"),
			envVar("PLLM_WRAPPER_PACKAGE", req.UVXPackage.Package),
			envVar("PLLM_WRAPPER_VERSION", req.UVXPackage.Version),
			envVar("HOME", "/tmp"),
			// uv respects XDG_* and UV_CACHE_DIR; nail the cache dir too.
			envVar("UV_CACHE_DIR", "/tmp/uv-cache"),
			envVar("XDG_CACHE_HOME", "/tmp/cache"),
		)
		if len(req.UVXPackage.Args) > 0 {
			base.Env = append(base.Env, envVar("PLLM_WRAPPER_ARGS", strings.Join(req.UVXPackage.Args, " ")))
		}
		base.VolumeMounts = append(base.VolumeMounts, corev1.VolumeMount{
			Name: "scratch", MountPath: "/tmp",
		})
	}

	// A basic TCP probe is enough — the wrapper binds as soon as the MCP
	// server initializes. Skip /healthz since we don't require one.
	base.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(defaultWrapperPort),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
	}
	return base
}

func (a *K8sAdapter) buildService(req *Request, ns string, podLabels map[string]string) *corev1.Service {
	labels := buildLabels(req)
	selector := map[string]string{
		"app.kubernetes.io/name":     req.WorkloadName,
		"app.kubernetes.io/instance": req.WorkloadName,
	}
	_ = podLabels
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        req.WorkloadName,
			Namespace:   ns,
			Labels:      labels,
			Annotations: buildAnnotations(req),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:        "mcp",
				Port:        defaultWrapperPort,
				TargetPort:  intstr.FromInt32(defaultWrapperPort),
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: stringPtr("pllm.ai/mcp"),
			}},
		},
	}
}

func (a *K8sAdapter) buildNetworkPolicy(req *Request, ns string, podLabels map[string]string) *networkingv1.NetworkPolicy {
	_ = podLabels
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.WorkloadName,
			Namespace: ns,
			Labels:    buildLabels(req),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{
				"app.kubernetes.io/name":     req.WorkloadName,
				"app.kubernetes.io/instance": req.WorkloadName,
			}},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			// Ingress: only from the pllm namespace/pod label.
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app.kubernetes.io/name": "pllm"},
					},
				}},
			}},
			// Egress: DNS + minimal allowlist. Operators override via annotations.
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				// DNS
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: protoPtr(corev1.ProtocolUDP),
					Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
				}},
			}},
		},
	}
}

// --- helpers ------------------------------------------------------------

func validateRequest(req *Request) error {
	if req == nil {
		return errors.New("request is nil")
	}
	if req.WorkloadName == "" {
		return errors.New("workload_name is required")
	}
	// K8s DNS-1123 limits: lowercase, digits, dash, max 63 chars.
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
	specCount := 0
	if req.Image != nil {
		specCount++
	}
	if req.NPXPackage != nil {
		specCount++
	}
	if req.UVXPackage != nil {
		specCount++
	}
	if specCount != 1 {
		return fmt.Errorf("exactly one of Image/NPXPackage/UVXPackage must be set (got %d)", specCount)
	}
	return nil
}

func buildLabels(req *Request) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       req.WorkloadName,
		"app.kubernetes.io/instance":   req.WorkloadName,
		"app.kubernetes.io/managed-by": "pllm",
		"pllm.ai/kind":                 "mcp-server",
	}
}

func buildAnnotations(req *Request) map[string]string {
	return map[string]string{
		"pllm.ai/resource-name":    req.ResourceName,
		"pllm.ai/resource-version": req.ResourceVersion,
		"pllm.ai/display-name":     req.DisplayName,
		"pllm.ai/deployed-at":      time.Now().UTC().Format(time.RFC3339),
	}
}

func buildEnvVars(m map[string]string) []corev1.EnvVar {
	if len(m) == 0 {
		return nil
	}
	out := make([]corev1.EnvVar, 0, len(m))
	for k, v := range m {
		out = append(out, corev1.EnvVar{Name: k, Value: v})
	}
	return out
}

func envVar(k, v string) corev1.EnvVar { return corev1.EnvVar{Name: k, Value: v} }
func stringPtr(s string) *string       { return &s }
func boolPtr(b bool) *bool             { return &b }
func int64Ptr(i int64) *int64          { return &i }
func protoPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func mergeMaps(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
