import { defineConfig } from "vitepress";
import { withMermaid } from "vitepress-plugin-mermaid";

export default withMermaid(
  defineConfig({
    title: "pllm",
    description: "Blazing Fast LLM Gateway - Documentation",
    base: "/docs/",
    ignoreDeadLinks: false,
    themeConfig: {
      nav: [
        { text: "Home", link: "/" },
        { text: "Guide", link: "/guide/getting-started" },
        { text: "Architecture", link: "/guide/architecture" },
        { text: "Admin UI", link: "/ui", target: "_blank" },
      ],
      sidebar: [
        {
          text: "Introduction",
          items: [
            { text: "What is PLLM?", link: "/" },
            { text: "Quick Start", link: "/guide/quickstart" },
            { text: "Installation", link: "/guide/getting-started" },
          ],
        },
        {
          text: "Configuration",
          items: [
            { text: "Configuration Guide", link: "/config" },
            { text: "Model Routing & Load Balancing", link: "/guide/routing" },
            { text: "Provider Setup", link: "/providers" },
            { text: "Authentication", link: "/auth" },
          ],
        },
        {
          text: "Architecture",
          items: [
            { text: "System Overview", link: "/guide/architecture" },
            { text: "Resilience & Reliability", link: "/guide/resilience" },
          ],
        },
        {
          text: "API Reference",
          items: [
            { text: "OpenAI Compatible API", link: "/api" },
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
