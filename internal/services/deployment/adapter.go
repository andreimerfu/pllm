// Package deployment manages turning a RegistryServer row into a running
// workload (K8s Deployment + Service in Phase 4). The Adapter interface
// lets us plug in non-k8s targets later (docker-compose, etc.) without
// changing callers.
package deployment

import (
	"context"

	"github.com/amerfu/pllm/internal/core/models"
)

// Adapter is the platform-agnostic interface the service layer calls.
// Every Adapter implementation:
//
//   - MUST be idempotent. Deploy(x) after Deploy(x) with the same spec
//     should be a no-op (or at worst re-apply the same manifests).
//   - MUST NOT return until the platform has accepted the manifest.
//     "Running" is signalled separately via Status().
//   - MUST clean up its own resources on Undeploy; pllm only deletes the
//     Deployment row after the adapter reports success.
type Adapter interface {
	// Platform returns the identifier used in DeploymentPlatform.
	Platform() models.DeploymentPlatform

	// Deploy applies the desired state. On success, Result.Endpoint is
	// populated with a URL the gateway can route to (host:port + path).
	Deploy(ctx context.Context, req *Request) (*Result, error)

	// Undeploy removes everything the adapter created for this deployment.
	Undeploy(ctx context.Context, d *models.Deployment) error

	// Status polls the platform for the current state. Used by the service
	// layer to update the DB row.
	Status(ctx context.Context, d *models.Deployment) (*StatusReport, error)
}

// Request is the normalized input the service hands to an adapter.
// The caller converts a RegistryServer into this shape.
type Request struct {
	Namespace    string
	WorkloadName string

	// Display — used for labels/annotations.
	DisplayName    string
	ResourceName   string // e.g. "io.modelcontextprotocol/server-everything"
	ResourceVersion string

	// Exactly one of Image / NPXPackage / UVXPackage / Remote is set.
	// The service picks the right one based on the RegistryServer's
	// packages/remotes fields.
	Image       *ImageSpec
	NPXPackage  *NPXSpec  // npm package to run via `npx`
	UVXPackage  *UVXSpec  // PyPI package to run via `uvx`
	RemoteURL   string    // for already-hosted MCP servers, no deploy needed

	// Optional per-deploy knobs.
	Env          map[string]string
	RestrictEgress bool

	// MemoryOverride replaces the adapter default memory limit for this
	// deployment. Set by the service layer's OOM auto-scaler; a K8s-style
	// quantity like "512Mi". Empty = use adapter default. Requests scale
	// to match so scheduling still works.
	MemoryOverride string
}

// ImageSpec deploys an OCI image directly (ideal — no runtime download).
type ImageSpec struct {
	Reference string // e.g. "ghcr.io/org/mcp-thing:1.2.3"
	Command   []string
	Args      []string
	// Port the MCP server listens on inside the container.
	// Default 8000 if zero.
	Port int32
}

// NPXSpec wraps an npm package. Adapter renders a Deployment that runs:
//
//	npx -y <Package>@<Version> [Args...]
//
// using a shared wrapper image that already has node + an HTTP bridge
// for stdio MCP servers.
type NPXSpec struct {
	Package string // e.g. "@modelcontextprotocol/server-everything"
	Version string
	Args    []string
}

// UVXSpec is the pip/pypi equivalent.
type UVXSpec struct {
	Package string
	Version string
	Args    []string
}

// Result carries the adapter's response.
type Result struct {
	// URL the gateway should use (http://svc.ns:port/mcp).
	Endpoint string
	// Opaque state the adapter wants persisted.
	AdapterState []byte
}

// StatusReport is the normalized status from a Status() call.
type StatusReport struct {
	Status  models.DeploymentStatus
	Reason  string
	Healthy bool

	// OOMKilled is true when the adapter detected an OOMKilled container
	// in the most recent (or current) pod generation. The service layer
	// uses this signal to trigger memory auto-scale. When true,
	// TerminatedMemoryLimit carries the memory limit that was active on
	// the pod at the time of death, so the reconciler can scale from the
	// right baseline instead of trusting a stale config default.
	OOMKilled             bool
	TerminatedMemoryLimit string
	// PodName + ExitCode are captured best-effort for the audit event.
	PodName  string
	ExitCode int32
}
