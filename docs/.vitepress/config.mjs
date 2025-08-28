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
          text: "Getting Started",
          items: [
            { text: "What is pllm?", link: "/" },
            { text: "Installation & Setup", link: "/guide/getting-started" },
            { text: "Quick Start Guide", link: "/guide/quickstart" },
          ],
        },
        {
          text: "Core Features",
          items: [
            { text: "System Architecture", link: "/guide/architecture" },
            { text: "Multi-Provider Support", link: "/providers" },
            { text: "Authentication", link: "/auth" },
            { text: "Configuration", link: "/config" },
          ],
        },
        {
          text: "API Reference",
          items: [
            { text: "OpenAI Compatible API", link: "/api" },
            { text: "Chat Completions", link: "/api#chat-completions" },
            { text: "Models", link: "/api#models" },
            { text: "Health Checks", link: "/api#health-checks" },
          ],
        },
        {
          text: "Deployment",
          items: [
            { text: "Docker Deployment", link: "/deployment" },
            { text: "Kubernetes", link: "/deployment#kubernetes" },
            { text: "Production Setup", link: "/deployment#production" },
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
