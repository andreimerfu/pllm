// Detects well-known services/brands from a registry artifact name and
// returns a matching Iconify icon identifier. Registry names follow
// "<owner>/<slug>" or deeper — we tokenize by `/`, `-`, `_`, `.`, space and
// probe each token (plus a few compound forms) against the table below.
//
// Preference order in the table: `logos:*` (full-color brand glyph) when
// Iconify carries the mark, else `simple-icons:*` (monochrome — inherits
// currentColor so it tints with the container).
//
// Usage:
//   import { getBrandLogo } from '@/lib/brand-logos';
//   const logo = getBrandLogo('sentry/mcp-sentry'); // → 'logos:sentry-icon'

const brandLogos: Record<string, string> = {
  // AI & LLM vendors
  openai: 'logos:openai-icon',
  anthropic: 'simple-icons:anthropic',
  claude: 'simple-icons:anthropic',
  mistral: 'simple-icons:mistralai',
  mistralai: 'simple-icons:mistralai',
  cohere: 'simple-icons:cohere',
  huggingface: 'logos:hugging-face-icon',
  hf: 'logos:hugging-face-icon',
  replicate: 'simple-icons:replicate',
  perplexity: 'simple-icons:perplexity',
  grok: 'simple-icons:x',
  xai: 'simple-icons:x',
  gemini: 'logos:google-gemini',
  groq: 'simple-icons:groq',
  nvidia: 'logos:nvidia',
  qwen: 'logos:qwen-icon',
  deepseek: 'simple-icons:deepseek',
  ollama: 'simple-icons:ollama',

  // Cloud
  google: 'logos:google-icon',
  googlecloud: 'logos:google-cloud',
  gcp: 'logos:google-cloud',
  vertex: 'logos:google-cloud',
  azure: 'logos:microsoft-azure',
  microsoft: 'logos:microsoft-icon',
  ms: 'logos:microsoft-icon',
  aws: 'logos:aws',
  amazon: 'logos:aws',
  bedrock: 'logos:aws',
  cloudflare: 'logos:cloudflare-icon',
  vercel: 'logos:vercel-icon',
  netlify: 'logos:netlify-icon',
  digitalocean: 'logos:digital-ocean',
  heroku: 'logos:heroku-icon',
  fly: 'logos:fly',
  flyio: 'logos:fly',
  railway: 'simple-icons:railway',
  render: 'simple-icons:render',
  supabase: 'logos:supabase-icon',
  firebase: 'logos:firebase',

  // Dev platforms & VCS
  github: 'logos:github-icon',
  gitlab: 'logos:gitlab',
  bitbucket: 'logos:bitbucket',
  git: 'logos:git-icon',
  atlassian: 'logos:atlassian',
  jira: 'logos:jira',
  confluence: 'logos:confluence',
  linear: 'simple-icons:linear',

  // Observability & ops
  sentry: 'logos:sentry-icon',
  datadog: 'logos:datadog',
  grafana: 'logos:grafana',
  prometheus: 'logos:prometheus',
  newrelic: 'logos:new-relic-icon',
  splunk: 'logos:splunk',
  pagerduty: 'logos:pagerduty-icon',
  opentelemetry: 'logos:opentelemetry-icon',
  otel: 'logos:opentelemetry-icon',
  jaeger: 'logos:jaeger-icon',
  elastic: 'logos:elastic',
  elasticsearch: 'logos:elasticsearch',
  kibana: 'logos:kibana',

  // Databases & data
  postgres: 'logos:postgresql',
  postgresql: 'logos:postgresql',
  mysql: 'logos:mysql-icon',
  mariadb: 'logos:mariadb-icon',
  mongodb: 'logos:mongodb-icon',
  mongo: 'logos:mongodb-icon',
  redis: 'logos:redis',
  sqlite: 'logos:sqlite',
  clickhouse: 'simple-icons:clickhouse',
  snowflake: 'logos:snowflake-icon',
  bigquery: 'logos:google-bigquery',
  databricks: 'logos:databricks-icon',
  duckdb: 'logos:duckdb',
  neo4j: 'logos:neo4j',
  cassandra: 'logos:cassandra',
  dynamodb: 'logos:aws-dynamodb',
  kafka: 'logos:kafka-icon',
  rabbitmq: 'logos:rabbitmq-icon',

  // DevOps / infra
  docker: 'logos:docker-icon',
  kubernetes: 'logos:kubernetes',
  k8s: 'logos:kubernetes',
  helm: 'logos:helm',
  terraform: 'logos:terraform-icon',
  ansible: 'logos:ansible',
  nginx: 'logos:nginx',
  traefik: 'logos:traefik-proxy-logo-orange',

  // Payments & commerce
  stripe: 'logos:stripe',
  paypal: 'logos:paypal',
  square: 'simple-icons:square',
  shopify: 'logos:shopify',
  woocommerce: 'logos:woocommerce-icon',

  // Communication & productivity
  slack: 'logos:slack-icon',
  discord: 'logos:discord-icon',
  zoom: 'logos:zoom-icon',
  teams: 'logos:microsoft-teams',
  notion: 'logos:notion-icon',
  figma: 'logos:figma',
  sketch: 'logos:sketch',
  miro: 'logos:miro',
  trello: 'logos:trello',
  asana: 'logos:asana',
  monday: 'simple-icons:mondaydotcom',
  clickup: 'simple-icons:clickup',
  intercom: 'logos:intercom-icon',
  zendesk: 'logos:zendesk-icon',
  hubspot: 'logos:hubspot',
  salesforce: 'logos:salesforce',
  mailchimp: 'logos:mailchimp-freddie',
  sendgrid: 'logos:sendgrid-icon',
  twilio: 'logos:twilio-icon',
  mailgun: 'simple-icons:mailgun',
  resend: 'simple-icons:resend',
  postmark: 'simple-icons:postmarkapp',
  servicenow: 'simple-icons:servicenow',
  opsgenie: 'simple-icons:opsgenie',
  squadcast: 'simple-icons:squadcast',
  victorops: 'simple-icons:victorops',
  statuspage: 'simple-icons:statuspage',

  // Media & streaming
  spotify: 'logos:spotify-icon',
  youtube: 'logos:youtube-icon',
  netflix: 'logos:netflix',
  twitch: 'logos:twitch',

  // Code quality / tooling
  eslint: 'logos:eslint',
  prettier: 'logos:prettier',
  biome: 'logos:biome',
  sonarqube: 'logos:sonarqube',
  typescript: 'logos:typescript-icon',
  ts: 'logos:typescript-icon',
  javascript: 'logos:javascript',
  js: 'logos:javascript',
  python: 'logos:python',
  py: 'logos:python',
  go: 'logos:go',
  golang: 'logos:go',
  rust: 'logos:rust',
  ruby: 'logos:ruby',
  php: 'logos:php',
  java: 'logos:java',
  kotlin: 'logos:kotlin-icon',
  swift: 'logos:swift',
  scala: 'logos:scala',

  // Frameworks / runtimes
  node: 'logos:nodejs-icon',
  nodejs: 'logos:nodejs-icon',
  bun: 'logos:bun',
  deno: 'logos:deno',
  react: 'logos:react',
  vue: 'logos:vue',
  angular: 'logos:angular-icon',
  svelte: 'logos:svelte-icon',
  nextjs: 'logos:nextjs-icon',
  next: 'logos:nextjs-icon',
  nuxt: 'logos:nuxt-icon',
  remix: 'logos:remix-icon',
  astro: 'logos:astro-icon',
  tailwind: 'logos:tailwindcss-icon',
  tailwindcss: 'logos:tailwindcss-icon',
  django: 'logos:django-icon',
  flask: 'logos:flask',
  fastapi: 'logos:fastapi-icon',
  rails: 'logos:rails',
  laravel: 'logos:laravel',
  spring: 'logos:spring-icon',
  express: 'logos:express',

  // Testing / browser automation
  playwright: 'logos:playwright',
  puppeteer: 'logos:puppeteer',
  cypress: 'logos:cypress-icon',
  jest: 'logos:jest',
  vitest: 'logos:vitest',
  storybook: 'logos:storybook-icon',

  // Storage / files
  dropbox: 'logos:dropbox',
  googledrive: 'logos:google-drive',
  drive: 'logos:google-drive',
  box: 'simple-icons:box',
  onedrive: 'logos:microsoft-onedrive',
  s3: 'logos:aws-s3',

  // Search / analytics
  algolia: 'logos:algolia',
  meilisearch: 'simple-icons:meilisearch',
  typesense: 'simple-icons:typesense',
  mixpanel: 'logos:mixpanel-icon',
  amplitude: 'simple-icons:amplitude',
  segment: 'simple-icons:segment',
  posthog: 'simple-icons:posthog',

  // Auth / identity
  auth0: 'logos:auth0-icon',
  okta: 'simple-icons:okta',
  clerk: 'simple-icons:clerk',
  dex: 'simple-icons:dex',
  keycloak: 'simple-icons:keycloak',

  // Misc popular SaaS
  linkedin: 'logos:linkedin-icon',
  airbnb: 'logos:airbnb',
  uber: 'logos:uber',
  lyft: 'simple-icons:lyft',
  pinterest: 'logos:pinterest',
  reddit: 'logos:reddit-icon',
  medium: 'logos:medium-icon',
  wordpress: 'logos:wordpress-icon',
  webflow: 'logos:webflow',
  framer: 'logos:framer',
  canva: 'logos:canva',
  adobe: 'logos:adobe',

  // MCP ecosystem refs that don't map to a brand
  modelcontextprotocol: 'simple-icons:anthropic',
  mcp: 'simple-icons:anthropic',
};

// Common registry prefixes / noise tokens to skip before matching. Things
// like "mcp", "server", "agent", "skill" dominate registry names and must
// never shadow a real brand match.
const NOISE = new Set([
  'mcp',
  'server',
  'servers',
  'agent',
  'agents',
  'skill',
  'skills',
  'prompt',
  'prompts',
  'gateway',
  'bridge',
  'api',
  'sdk',
  'cli',
  'tool',
  'tools',
  'client',
  'plugin',
  'integration',
  'official',
  'community',
  'demo',
  'example',
  'examples',
  'test',
  'tests',
  'dev',
  'core',
  'lib',
  'v1',
  'v2',
  'v3',
]);

// Tokenize a registry name / title: split on path/word separators and also
// compound tokens like "googlecloud" or "nextjs".
function tokenize(input: string): string[] {
  const lower = input.toLowerCase();
  const raw = lower.split(/[/\-_\s.@:]+/).filter(Boolean);
  const tokens = new Set<string>();
  for (const t of raw) {
    if (t.length < 2) continue;
    tokens.add(t);
    // Split camelCase / digit-adjacent tokens too (e.g. "azureOpenai", "nextjs").
    for (const sub of t.split(/(?=[A-Z])|(?<=\D)(?=\d)|(?<=\d)(?=\D)/g)) {
      const s = sub.toLowerCase();
      if (s.length >= 2) tokens.add(s);
    }
  }
  return [...tokens];
}

// Detect a brand logo from a registry name (or title). Returns the Iconify
// icon ID, or null when no brand matches. The first token that both
// (a) isn't noise, and (b) is present in the brand table wins — registry
// names put the vendor first ("sentry/mcp-sentry"), so this almost always
// picks up the owner segment.
export function getBrandLogo(name: string | undefined | null): string | null {
  if (!name) return null;
  const tokens = tokenize(name);
  for (const t of tokens) {
    if (NOISE.has(t)) continue;
    const hit = brandLogos[t];
    if (hit) return hit;
  }
  // Second pass: allow noise tokens as a last resort (e.g. a name that's
  // literally "mcp/server-thing" — we'd rather show MCP than nothing).
  for (const t of tokens) {
    const hit = brandLogos[t];
    if (hit) return hit;
  }
  return null;
}

// True if the matched icon lives in the `logos:` namespace (full-color).
// Used by the renderer to decide whether to drop the kind tint background
// — colored logos read better on neutral card stock.
export function isColoredLogo(icon: string): boolean {
  return icon.startsWith('logos:');
}
