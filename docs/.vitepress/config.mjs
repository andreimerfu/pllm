import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

export default withMermaid(
  defineConfig({
    title: "pllm",
    description:
      "Control plane for agentic systems — LLM gateway, MCP gateway, registry, deployments, and governance",
    base: "/docs/",
    ignoreDeadLinks: false,
    // Implementation plans live under docs/superpowers/plans/ for the
    // writing-plans skill convention but contain TSX/JSX snippets that
    // VitePress's Vue compiler chokes on. They're working notes, not
    // user-facing docs, so exclude them from the site build.
    srcExclude: ["**/superpowers/**"],
    themeConfig: {
      nav: [
        { text: "Home", link: "/" },
        { text: "Guide", link: "/guide/getting-started" },
        { text: "Architecture", link: "/guide/architecture" },
        { text: "API", link: "/api/" },
        { text: "Admin UI", link: "/ui", target: "_blank" },
      ],
      sidebar: [
        {
          text: "Introduction",
          items: [
            { text: "What is pllm?", link: "/" },
            { text: "Concepts", link: "/guide/concepts" },
            { text: "Quick Start", link: "/guide/quickstart" },
            { text: "Installation", link: "/guide/getting-started" },
          ],
        },
        {
          text: "LLM Gateway",
          items: [
            { text: "Overview", link: "/llm/overview" },
            { text: "Providers", link: "/providers" },
            { text: "Routing & Load Balancing", link: "/guide/routing" },
            { text: "Resilience", link: "/guide/resilience" },
            { text: "Budgets & Usage", link: "/llm/budgets" },
          ],
        },
        {
          text: "MCP Gateway",
          items: [
            { text: "Overview", link: "/mcp/overview" },
            { text: "Backends & Transports", link: "/mcp/backends" },
            { text: "Per-Tool ACL", link: "/mcp/acl" },
            { text: "Registry as MCP", link: "/mcp/registry-as-mcp" },
          ],
        },
        {
          text: "Registry",
          items: [
            { text: "Overview", link: "/registry/overview" },
            { text: "Artifacts & Versioning", link: "/registry/artifacts" },
            { text: "References & the Graph", link: "/registry/refs" },
            { text: "OSV Enrichment & Scores", link: "/registry/enrichment" },
            { text: "npm / PyPI Importers", link: "/registry/importers" },
          ],
        },
        {
          text: "Agents",
          items: [
            { text: "Overview", link: "/agents/overview" },
            { text: "Authoring & Publishing", link: "/agents/authoring" },
            { text: "Onboarding Walkthrough", link: "/agents/onboarding" },
            { text: "A2A Interoperability", link: "/agents/a2a" },
          ],
        },
        {
          text: "Deployments",
          items: [
            { text: "Overview", link: "/deployments/overview" },
            { text: "K8s Adapter", link: "/deployments/k8s" },
            { text: "Wrapper Image (npx/uvx)", link: "/deployments/wrapper" },
            { text: "OOM Autoscaling", link: "/deployments/oom" },
            { text: "Network Policy", link: "/deployments/network-policy" },
          ],
        },
        {
          text: "Governance (MRM)",
          items: [
            { text: "Overview", link: "/governance/overview" },
            { text: "Use Cases", link: "/governance/use-cases" },
            { text: "Scope Enforcement", link: "/governance/scope" },
            { text: "Audit & Scope Events", link: "/governance/audit" },
            { text: "Webhooks (SIEM/GRC)", link: "/governance/webhooks" },
            { text: "Policy Control-Plane Contract", link: "/governance/policy-control-plane-contract" },
          ],
        },
        {
          text: "Security",
          items: [
            { text: "Authentication", link: "/auth" },
            { text: "API Keys & Permissions", link: "/security/keys" },
            { text: "Guardrails", link: "/security/guardrails" },
          ],
        },
        {
          text: "Configuration",
          items: [
            { text: "Configuration Reference", link: "/config" },
            { text: "Provider Setup", link: "/providers" },
          ],
        },
        {
          text: "API Reference",
          items: [
            { text: "Overview", link: "/api/" },
            { text: "OpenAI-Compatible", link: "/api/openai" },
            { text: "Anthropic Messages", link: "/api/anthropic" },
            { text: "MCP & Registry", link: "/api/mcp-registry" },
            { text: "Admin Endpoints", link: "/api/admin" },
          ],
        },
        {
          text: "Architecture",
          items: [
            { text: "System Overview", link: "/guide/architecture" },
            { text: "Data Model", link: "/guide/data-model" },
          ],
        },
        {
          text: "Deployment",
          items: [
            { text: "Docker & Kubernetes", link: "/deployment" },
          ],
        },
      ],
      socialLinks: [
        { icon: "github", link: "https://github.com/andreimerfu/pllm" },
      ],
      search: {
        provider: "local",
      },
    },
  }),
);
