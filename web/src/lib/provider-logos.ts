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
};

export function getProviderLogo(provider: string): string {
  const key = provider.toLowerCase().replace(/[\s_]+/g, '-');
  return providerLogos[key] ?? 'solar:cpu-bolt-linear';
}

export function getProviderColor(provider: string): string {
  const key = provider.toLowerCase().replace(/[\s_]+/g, '-');
  return providerColors[key] ?? '#6B7280';
}
