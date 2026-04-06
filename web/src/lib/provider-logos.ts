// Maps LLM provider names to Iconify logo identifiers.
// These render as full-color brand logos.
// Usage: import { getProviderLogo } from '@/lib/provider-logos'; <Icon icon={getProviderLogo('openai')} />

const providerLogos: Record<string, string> = {
  openai: 'logos:openai-icon',
  anthropic: 'simple-icons:anthropic',
  azure: 'logos:azure-icon',
  'azure-openai': 'logos:azure-icon',
  bedrock: 'logos:aws',
  aws: 'logos:aws',
  vertex: 'logos:google-cloud',
  'vertex-ai': 'logos:google-cloud',
  google: 'logos:google-cloud',
  cohere: 'simple-icons:cohere',
  grok: 'simple-icons:x',
  mistral: 'simple-icons:mistral',
  qwen: 'logos:qwen-icon',
  nvidia: 'simple-icons:nvidia',
  kimi: 'hugeicons:kimi-ai',
};

export const providerColors: Record<string, string> = {
  openai: '#000000',
  anthropic: '#C4956A',
  azure: '#0078D4',
  'azure-openai': '#0078D4',
  bedrock: '#FF9900',
  aws: '#FF9900',
  vertex: '#4285F4',
  'vertex-ai': '#4285F4',
  google: '#4285F4',
  cohere: '#39594D',
  grok: '#000000',
  mistral: '#F54E42',
  qwen: '#6D28D9',
  nvidia: '#76B900',
  kimi: '#0EA5E9',
};

// Maps model identifier substrings to logos (checked before provider lookup).
const modelLogos: Record<string, string> = {
  qwen: 'logos:qwen-icon',
  nvidia: 'simple-icons:nvidia',
  kimi: 'hugeicons:kimi-ai',
};

export function getProviderLogo(provider: string): string {
  const key = provider.toLowerCase().replace(/[\s_]+/g, '-');
  // Check if the identifier matches a known model family
  for (const [pattern, logo] of Object.entries(modelLogos)) {
    if (key.includes(pattern)) return logo;
  }
  return providerLogos[key] ?? 'solar:cpu-bolt-linear';
}

export function getProviderColor(provider: string): string {
  const key = provider.toLowerCase().replace(/[\s_]+/g, '-');
  return providerColors[key] ?? '#6B7280';
}
