package deployment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/amerfu/pllm/internal/core/config"
	"github.com/amerfu/pllm/internal/core/models"
)

func newTestAdapter() *K8sAdapter {
	return NewK8sAdapterWithClient(fake.NewSimpleClientset(), config.DeploymentK8sConfig{
		Namespace:    "pllm-mcp",
		WrapperImage: "ghcr.io/pllm/mcp-wrapper:0.1",
	}, nil)
}

func TestDeployOCI(t *testing.T) {
	adapter := newTestAdapter()
	res, err := adapter.Deploy(context.Background(), &Request{
		Namespace:       "pllm-mcp",
		WorkloadName:    "server-everything-0-6-2",
		DisplayName:     "Everything",
		ResourceName:    "io.mcp/server-everything",
		ResourceVersion: "0.6.2",
		Image:           &ImageSpec{Reference: "mcp/everything:0.6.2", Port: 3001},
	})
	require.NoError(t, err)
	require.Contains(t, res.Endpoint, "server-everything-0-6-2.pllm-mcp.svc.cluster.local")

	// Deployment exists with expected image + port.
	dep, err := adapter.client.AppsV1().Deployments("pllm-mcp").
		Get(context.Background(), "server-everything-0-6-2", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "mcp/everything:0.6.2", dep.Spec.Template.Spec.Containers[0].Image)
	require.Equal(t, int32(3001), dep.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)

	// Service labels include pllm.ai/kind.
	svc, err := adapter.client.CoreV1().Services("pllm-mcp").
		Get(context.Background(), "server-everything-0-6-2", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "mcp-server", svc.Labels["pllm.ai/kind"])
	require.NotNil(t, svc.Spec.Ports[0].AppProtocol)
	require.Equal(t, "pllm.ai/mcp", *svc.Spec.Ports[0].AppProtocol)
}

func TestDeployNPXUsesWrapper(t *testing.T) {
	adapter := newTestAdapter()
	_, err := adapter.Deploy(context.Background(), &Request{
		Namespace:       "pllm-mcp",
		WorkloadName:    "srv-everything",
		ResourceName:    "io.mcp/server-everything",
		ResourceVersion: "0.6.2",
		NPXPackage:      &NPXSpec{Package: "@modelcontextprotocol/server-everything", Version: "0.6.2"},
	})
	require.NoError(t, err)

	dep, err := adapter.client.AppsV1().Deployments("pllm-mcp").
		Get(context.Background(), "srv-everything", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/pllm/mcp-wrapper:0.1", dep.Spec.Template.Spec.Containers[0].Image)

	// Env tells the wrapper what to run.
	envs := map[string]string{}
	for _, e := range dep.Spec.Template.Spec.Containers[0].Env {
		envs[e.Name] = e.Value
	}
	require.Equal(t, "npx", envs["PLLM_WRAPPER_KIND"])
	require.Equal(t, "@modelcontextprotocol/server-everything", envs["PLLM_WRAPPER_PACKAGE"])
	require.Equal(t, "0.6.2", envs["PLLM_WRAPPER_VERSION"])
	require.Equal(t, "/tmp", envs["HOME"])

	// Scratch emptyDir mounted at /tmp so npm cache has somewhere to live.
	mounts := dep.Spec.Template.Spec.Containers[0].VolumeMounts
	require.Len(t, mounts, 1)
	require.Equal(t, "scratch", mounts[0].Name)
	require.Equal(t, "/tmp", mounts[0].MountPath)
	vols := dep.Spec.Template.Spec.Volumes
	require.Len(t, vols, 1)
	require.NotNil(t, vols[0].EmptyDir)
}

func TestDeployOCIHasNoScratchMount(t *testing.T) {
	// OCI builds should NOT get the scratch volume — their images manage
	// their own filesystem.
	adapter := newTestAdapter()
	_, err := adapter.Deploy(context.Background(), &Request{
		Namespace: "pllm-mcp", WorkloadName: "srv-oci",
		Image: &ImageSpec{Reference: "mcp/x:1"},
	})
	require.NoError(t, err)
	dep, _ := adapter.client.AppsV1().Deployments("pllm-mcp").
		Get(context.Background(), "srv-oci", metav1.GetOptions{})
	require.Empty(t, dep.Spec.Template.Spec.Volumes)
	require.Empty(t, dep.Spec.Template.Spec.Containers[0].VolumeMounts)
}

func TestDeployIsIdempotent(t *testing.T) {
	adapter := newTestAdapter()
	req := &Request{
		Namespace:    "pllm-mcp",
		WorkloadName: "srv-x",
		Image:        &ImageSpec{Reference: "mcp/x:1"},
	}
	_, err := adapter.Deploy(context.Background(), req)
	require.NoError(t, err)
	// Second call must succeed (update path).
	_, err = adapter.Deploy(context.Background(), req)
	require.NoError(t, err)

	// Exactly one Deployment + one Service.
	deps, _ := adapter.client.AppsV1().Deployments("pllm-mcp").List(context.Background(), metav1.ListOptions{})
	require.Len(t, deps.Items, 1)
	svcs, _ := adapter.client.CoreV1().Services("pllm-mcp").List(context.Background(), metav1.ListOptions{})
	require.Len(t, svcs.Items, 1)
}

func TestUndeployCleansUp(t *testing.T) {
	adapter := newTestAdapter()
	_, err := adapter.Deploy(context.Background(), &Request{
		Namespace:    "pllm-mcp",
		WorkloadName: "srv-y",
		Image:        &ImageSpec{Reference: "mcp/y:1"},
	})
	require.NoError(t, err)

	err = adapter.Undeploy(context.Background(), &models.Deployment{
		Namespace: "pllm-mcp", WorkloadName: "srv-y",
	})
	require.NoError(t, err)

	deps, _ := adapter.client.AppsV1().Deployments("pllm-mcp").List(context.Background(), metav1.ListOptions{})
	require.Len(t, deps.Items, 0)
	svcs, _ := adapter.client.CoreV1().Services("pllm-mcp").List(context.Background(), metav1.ListOptions{})
	require.Len(t, svcs.Items, 0)
}

func TestUndeployTolerantOfMissing(t *testing.T) {
	adapter := newTestAdapter()
	// Should not error even though nothing exists.
	require.NoError(t, adapter.Undeploy(context.Background(), &models.Deployment{
		Namespace: "pllm-mcp", WorkloadName: "never-existed",
	}))
}

func TestStatusReportsRunningWhenReady(t *testing.T) {
	adapter := newTestAdapter()
	_, err := adapter.Deploy(context.Background(), &Request{
		Namespace:    "pllm-mcp",
		WorkloadName: "srv-z",
		Image:        &ImageSpec{Reference: "mcp/z:1"},
	})
	require.NoError(t, err)

	// Simulate a Ready Deployment: fake clientset doesn't fill Status; do
	// it manually.
	dep, _ := adapter.client.AppsV1().Deployments("pllm-mcp").
		Get(context.Background(), "srv-z", metav1.GetOptions{})
	dep.Status.ReadyReplicas = 1
	_, _ = adapter.client.AppsV1().Deployments("pllm-mcp").
		UpdateStatus(context.Background(), dep, metav1.UpdateOptions{})

	rep, err := adapter.Status(context.Background(), &models.Deployment{
		Namespace: "pllm-mcp", WorkloadName: "srv-z",
	})
	require.NoError(t, err)
	require.Equal(t, models.DeploymentStatusRunning, rep.Status)
	require.True(t, rep.Healthy)
}

func TestValidateRequestRejectsBadNames(t *testing.T) {
	cases := []struct {
		name    string
		wl      string
		wantErr bool
	}{
		{"ok", "server-x-1", false},
		{"uppercase", "ServerX", true},
		{"underscore", "server_x", true},
		{"slash", "org/x", true},
		{"too-long", strings_repeat("a", 64), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRequest(&Request{
				WorkloadName: tc.wl,
				Image:        &ImageSpec{Reference: "x"},
			})
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateRequestRequiresExactlyOneSpec(t *testing.T) {
	err := validateRequest(&Request{WorkloadName: "a"})
	require.Error(t, err)

	err = validateRequest(&Request{
		WorkloadName: "a",
		Image:        &ImageSpec{Reference: "x"},
		NPXPackage:   &NPXSpec{Package: "y", Version: "1"},
	})
	require.Error(t, err)
}

// stdlib trick — Go has no strings.Repeat alias; use the actual one.
func strings_repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// Smoke test wired into the fake namespaces API: Deploy creates the
// namespace if absent.
func TestDeployCreatesNamespace(t *testing.T) {
	adapter := newTestAdapter()
	_, err := adapter.Deploy(context.Background(), &Request{
		Namespace:    "fresh-ns",
		WorkloadName: "srv-w",
		Image:        &ImageSpec{Reference: "mcp/w:1"},
	})
	require.NoError(t, err)
	ns, err := adapter.client.CoreV1().Namespaces().Get(context.Background(), "fresh-ns", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "pllm", ns.Labels["app.kubernetes.io/managed-by"])

	// And the service lands in that namespace.
	_, err = adapter.client.CoreV1().Services("fresh-ns").Get(context.Background(), "srv-w", metav1.GetOptions{})
	require.NoError(t, err)
	_ = corev1.ServiceTypeClusterIP // reference import
}
