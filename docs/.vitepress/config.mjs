import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

export default withMermaid(defineConfig({
  title: 'pllm',
  description: 'Blazing Fast LLM Gateway - Documentation',
  base: '/docs/',
  ignoreDeadLinks: true,
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'API Reference', link: '/api/' },
      { text: 'Admin UI', link: '/ui', target: '_blank' }
    ],
    sidebar: [
      {
        text: 'Introduction',
        items: [
          { text: 'What is pllm?', link: '/' },
          { text: 'Getting Started', link: '/guide/getting-started' },
          { text: 'Quick Start', link: '/guide/quickstart' }
        ]
      },
      {
        text: 'Architecture',
        items: [
          { text: 'System Overview', link: '/guide/architecture' },
          { text: 'Circuit Breakers', link: '/guide/architecture#circuit-breaker' },
          { text: 'Load Balancing', link: '/guide/architecture#load-balancing' },
          { text: 'Rate Limiting', link: '/guide/architecture#rate-limiting' },
          { text: 'Caching Strategy', link: '/guide/architecture#caching-strategy' },
          { text: 'Streaming', link: '/guide/architecture#streaming-implementation' }
        ]
      },
      {
        text: 'Features',
        items: [
          { text: 'Multi-Provider Support', link: '/features/providers' },
          { text: 'Load Balancing', link: '/features/load-balancing' },
          { text: 'Rate Limiting', link: '/features/rate-limiting' },
          { text: 'Caching', link: '/features/caching' },
          { text: 'Streaming', link: '/features/streaming' },
          { text: 'Adaptive Routing', link: '/features/adaptive-routing' }
        ]
      },
      {
        text: 'API Reference',
        items: [
          { text: 'OpenAI Compatible', link: '/api/' },
          { text: 'Chat Completions', link: '/api/chat-completions' },
          { text: 'Embeddings', link: '/api/embeddings' },
          { text: 'Models', link: '/api/models' },
          { text: 'Authentication', link: '/api/authentication' }
        ]
      },
      {
        text: 'Configuration',
        items: [
          { text: 'Environment Variables', link: '/config/environment' },
          { text: 'YAML Configuration', link: '/config/yaml' },
          { text: 'Model Configuration', link: '/config/models' },
          { text: 'Database Setup', link: '/config/database' }
        ]
      },
      {
        text: 'Deployment',
        items: [
          { text: 'Docker', link: '/deployment/docker' },
          { text: 'Kubernetes', link: '/deployment/kubernetes' },
          { text: 'Production Best Practices', link: '/deployment/production' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/amerfu/pllm' }
    ],
    search: {
      provider: 'local'
    }
  }
}))