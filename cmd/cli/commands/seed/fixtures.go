package seed

// Bank-MRM demo cast. All slugs under `bank.demo/*` are owned by the
// seed and safe to delete. Public slugs (tavily-search, mcp.io/*) are
// touched only when the existing entry's description matches the
// SeedMarker, so we never blow away an operator's pre-existing entry.

const SeedMarker = "[bank-demo-seed] "

// --- Servers (all catalog-only by default) -------------------------------

type ServerFixture struct {
	Name        string
	Version     string
	Description string
	Score       int // forced enrichment score, 0-100
	// Packages mirrors RegistryServer.Packages — when non-nil the entry
	// is deployable. Catalog-only fixtures leave this nil.
	Packages []PackageRef
}

// PackageRef matches the wire shape the deployment service decodes from
// RegistryServer.Packages / RegistryAgent.Packages.
type PackageRef struct {
	RegistryType string `json:"registry_type"` // "npm" | "pypi" | "oci"
	Identifier   string `json:"identifier"`
	Version      string `json:"version"`
}

var BankServers = []ServerFixture{
	{
		Name:        "bank.demo/policy-docs",
		Version:     "1.0.0",
		Description: SeedMarker + "Internal policy + procedure RAG for the demo bank.",
		Score:       90,
	},
	{
		Name:        "bank.demo/credit-scoring",
		Version:     "1.0.0",
		Description: SeedMarker + "Credit-score lookup against the bureau.",
		Score:       85,
	},
	{
		Name:        "bank.demo/transactions-readonly",
		Version:     "1.0.0",
		Description: SeedMarker + "Read-only transaction-history lookups.",
		Score:       80,
	},
	{
		Name:        "bank.demo/customer-360",
		Version:     "1.0.0",
		Description: SeedMarker + "Aggregated customer profile (KYC + product holdings).",
		Score:       75,
	},
	{
		Name:        "tavily-search",
		Version:     "1.0.0",
		Description: SeedMarker + "Web search via Tavily (public).",
		Score:       95,
	},
}

// PublicLiveServers are published + deployed when --with-deployments is
// set. They reuse the standard @modelcontextprotocol/* npm packages
// driven by the existing wrapper image (built via
// `make wrapper-image-minikube`). Each gets a real K8s workload + an
// auto-registered MCP gateway backend (so tools surface under
// reg-<workload>__* in /v1/mcp).
var PublicLiveServers = []ServerFixture{
	{
		Name:        "mcp.io/filesystem",
		Version:     "1.0.0",
		Description: SeedMarker + "Reference filesystem MCP server (sandboxed).",
		Score:       90,
		Packages: []PackageRef{
			{RegistryType: "npm", Identifier: "@modelcontextprotocol/server-filesystem", Version: "latest"},
		},
	},
	{
		Name:        "mcp.io/server-everything",
		Version:     "1.0.0",
		Description: SeedMarker + "MCP reference server with every tool type.",
		Score:       90,
		Packages: []PackageRef{
			{RegistryType: "npm", Identifier: "@modelcontextprotocol/server-everything", Version: "latest"},
		},
	},
}

// --- Skills, prompts ------------------------------------------------------

type ArtifactFixture struct {
	Name        string
	Version     string
	Description string
}

var BankSkills = []ArtifactFixture{
	{
		Name:        "bank.demo/pii-redactor",
		Version:     "1.0.0",
		Description: SeedMarker + "Redact PII from free-text inputs.",
	},
}

var BankPrompts = []ArtifactFixture{
	{
		Name:        "bank.demo/decision-rationale",
		Version:     "1.0.0",
		Description: SeedMarker + "Walk through an underwriting decision step by step.",
	},
	{
		Name:        "bank.demo/kyc-summary-template",
		Version:     "1.0.0",
		Description: SeedMarker + "Summarize KYC findings for an analyst.",
	},
}

// --- Agents ---------------------------------------------------------------

type AgentRefFixture struct {
	TargetKind string
	TargetName string
}

type AgentFixture struct {
	Name        string
	Version     string
	Description string
	Refs        []AgentRefFixture
	// Packages mirrors RegistryAgent.Packages — required for deployment.
	// Both bank-demo agents reuse the existing pllm/agent-research image
	// (env-configurable probe targets) so we get a real allowed-vs-denied
	// scope story without authoring net-new agent code per scenario.
	Packages []PackageRef
	// DeployForUseCase names the use case the agent should be deployed
	// against when --with-deployments is set. Agent deploys require a
	// use_case_id at deploy time (it pins the bound key's scope).
	DeployForUseCase string
	// DeployExtraEnv is merged on top of the agent adapter's defaults
	// at deploy time. Used to point the research image at the right
	// allowed / denied probe models per scenario.
	DeployExtraEnv map[string]string
}

var BankAgents = []AgentFixture{
	{
		Name:        "bank.demo/loan-officer-copilot",
		Version:     "0.1.0",
		Description: SeedMarker + "Copilot for loan officers — pulls policies and credit scores, drafts decision rationales.",
		Refs: []AgentRefFixture{
			{TargetKind: "server", TargetName: "bank.demo/policy-docs"},
			{TargetKind: "server", TargetName: "bank.demo/credit-scoring"},
			{TargetKind: "prompt", TargetName: "bank.demo/decision-rationale"},
		},
		Packages: []PackageRef{
			{RegistryType: "oci", Identifier: "pllm/agent-research", Version: "dev"},
		},
		DeployForUseCase: "loan-underwriting-assistant",
		// Loan underwriting is T1 BLOCKING and declares free-route, so
		// the allowed probe really succeeds (real LLM response from
		// OpenRouter free tier) and the denied probe gets a 403 with the
		// canonical scope.denied phrase.
		DeployExtraEnv: map[string]string{
			"PLLM_PROBE_ALLOWED": "free-route",
			"PLLM_PROBE_DENIED":  "gpt-4-turbo",
		},
	},
	{
		Name:        "bank.demo/kyc-summarizer",
		Version:     "0.1.0",
		Description: SeedMarker + "Summarizes KYC dossiers, redacting PII before producing the summary.",
		Refs: []AgentRefFixture{
			{TargetKind: "server", TargetName: "bank.demo/transactions-readonly"},
			{TargetKind: "prompt", TargetName: "bank.demo/kyc-summary-template"},
			{TargetKind: "skill", TargetName: "bank.demo/pii-redactor"},
		},
		Packages: []PackageRef{
			{RegistryType: "oci", Identifier: "pllm/agent-research", Version: "dev"},
		},
		DeployForUseCase: "aml-investigation-copilot",
		// AML is in flag mode (under_review): out-of-scope calls log
		// instead of blocking. The "denied" probe will report
		// scope_allowed=true with the routing-layer status — that's the
		// flag-mode story (logged in the audit trail, not blocked).
		DeployExtraEnv: map[string]string{
			"PLLM_PROBE_ALLOWED": "free-route",
			"PLLM_PROBE_DENIED":  "gpt-4-turbo",
		},
	},
}

// --- Use cases ------------------------------------------------------------

type UseCaseRefFixture struct {
	TargetKind string
	TargetName string
}

type UseCaseFixture struct {
	Slug                string
	Name                string
	Description         string
	RiskTier            string // "tier1".."tier4"
	Status              string // "draft" | "under_review" | "approved" | "retired"
	EnforceScope        bool
	DataClassifications []string
	Refs                []UseCaseRefFixture // LLM ref injected at runtime from --llm-route
}

func BankUseCases(llmRoute string) []UseCaseFixture {
	llmRef := UseCaseRefFixture{TargetKind: "llm", TargetName: llmRoute}
	return []UseCaseFixture{
		{
			Slug:                "loan-underwriting-assistant",
			Name:                "Loan underwriting assistant",
			Description:         SeedMarker + "T1 blocking — assists loan officers with underwriting decisions.",
			RiskTier:            "tier1",
			Status:              "approved",
			EnforceScope:        true,
			DataClassifications: []string{"pii", "customer-financial", "confidential"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "bank.demo/policy-docs"},
				{TargetKind: "server", TargetName: "bank.demo/credit-scoring"},
				{TargetKind: "agent", TargetName: "bank.demo/loan-officer-copilot"},
				{TargetKind: "prompt", TargetName: "bank.demo/decision-rationale"},
			},
		},
		{
			Slug:                "aml-investigation-copilot",
			Name:                "AML investigation copilot",
			Description:         SeedMarker + "T1 flag-mode under review — assists AML analysts during investigations.",
			RiskTier:            "tier1",
			Status:              "under_review",
			EnforceScope:        false,
			DataClassifications: []string{"pii", "customer-financial", "confidential"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "bank.demo/transactions-readonly"},
				{TargetKind: "server", TargetName: "tavily-search"},
				{TargetKind: "agent", TargetName: "bank.demo/kyc-summarizer"},
			},
		},
		{
			Slug:                "branch-staff-knowledge-search",
			Name:                "Branch-staff knowledge search",
			Description:         SeedMarker + "T2 blocking — internal policy and procedure search for branch employees.",
			RiskTier:            "tier2",
			Status:              "approved",
			EnforceScope:        true,
			DataClassifications: []string{"internal"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "bank.demo/policy-docs"},
				{TargetKind: "server", TargetName: "mcp.io/filesystem"},
			},
		},
		{
			Slug:                "marketing-copy-drafting",
			Name:                "Marketing copy drafting",
			Description:         SeedMarker + "T4 sandbox — drafts marketing copy. Flag mode while in draft.",
			RiskTier:            "tier4",
			Status:              "draft",
			EnforceScope:        false,
			DataClassifications: []string{"public"},
			Refs: []UseCaseRefFixture{
				llmRef,
				{TargetKind: "server", TargetName: "mcp.io/server-everything"},
			},
		},
	}
}

// --- API keys -------------------------------------------------------------

type KeyFixture struct {
	Name        string
	UseCaseSlug string
}

var BankKeys = []KeyFixture{
	{Name: "demo-loan-officer", UseCaseSlug: "loan-underwriting-assistant"},
	{Name: "demo-aml-analyst", UseCaseSlug: "aml-investigation-copilot"},
	{Name: "demo-marketing", UseCaseSlug: "marketing-copy-drafting"},
}
