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

// PublicLiveServers are published only when --with-deployments is set.
// They reuse standard MCP server packages. The seed only publishes the
// catalog entry — actual K8s deployment is left to the operator's
// "Deploy" button in /ui/deployments because it requires the wrapper
// image and per-cluster RBAC.
var PublicLiveServers = []ServerFixture{
	{
		Name:        "mcp.io/filesystem",
		Version:     "1.0.0",
		Description: SeedMarker + "Reference filesystem MCP server (sandboxed).",
		Score:       90,
	},
	{
		Name:        "mcp.io/server-everything",
		Version:     "1.0.0",
		Description: SeedMarker + "MCP reference server with every tool type.",
		Score:       90,
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
