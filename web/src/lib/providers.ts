export interface ProviderInfo {
  icon: string;
  name: string;
  color: string;
  bgColor: string;
  borderColor: string;
}

export const PROVIDERS: Record<string, ProviderInfo> = {
  openai: {
    icon: "logos:openai-icon",
    name: "OpenAI",
    color: "text-emerald-600 dark:text-emerald-400",
    bgColor: "bg-emerald-50 dark:bg-emerald-950/30",
    borderColor: "border-emerald-200 dark:border-emerald-800",
  },
  anthropic: {
    icon: "logos:anthropic",
    name: "Anthropic", 
    color: "text-orange-600 dark:text-orange-400",
    bgColor: "bg-orange-50 dark:bg-orange-950/30",
    borderColor: "border-orange-200 dark:border-orange-800",
  },
  mistral: {
    icon: "logos:mistral",
    name: "Mistral AI",
    color: "text-blue-600 dark:text-blue-400", 
    bgColor: "bg-blue-50 dark:bg-blue-950/30",
    borderColor: "border-blue-200 dark:border-blue-800",
  },
  meta: {
    icon: "logos:meta",
    name: "Meta",
    color: "text-indigo-600 dark:text-indigo-400",
    bgColor: "bg-indigo-50 dark:bg-indigo-950/30",
    borderColor: "border-indigo-200 dark:border-indigo-800",
  },
  google: {
    icon: "logos:google",
    name: "Google",
    color: "text-red-600 dark:text-red-400",
    bgColor: "bg-red-50 dark:bg-red-950/30",
    borderColor: "border-red-200 dark:border-red-800",
  },
  azure: {
    icon: "logos:microsoft-azure",
    name: "Azure OpenAI",
    color: "text-blue-700 dark:text-blue-300",
    bgColor: "bg-blue-50 dark:bg-blue-950/30",
    borderColor: "border-blue-200 dark:border-blue-800",
  },
  microsoft: {
    icon: "logos:microsoft",
    name: "Microsoft",
    color: "text-blue-700 dark:text-blue-300",
    bgColor: "bg-blue-50 dark:bg-blue-950/30",
    borderColor: "border-blue-200 dark:border-blue-800",
  },
  aws: {
    icon: "logos:aws",
    name: "AWS",
    color: "text-yellow-600 dark:text-yellow-400",
    bgColor: "bg-yellow-50 dark:bg-yellow-950/30",
    borderColor: "border-yellow-200 dark:border-yellow-800",
  },
  openrouter: {
    icon: "lucide:globe",
    name: "OpenRouter",
    color: "text-purple-600 dark:text-purple-400",
    bgColor: "bg-purple-50 dark:bg-purple-950/30",
    borderColor: "border-purple-200 dark:border-purple-800",
  },
  cohere: {
    icon: "simple-icons:cohere",
    name: "Cohere",
    color: "text-green-600 dark:text-green-400",
    bgColor: "bg-green-50 dark:bg-green-950/30",
    borderColor: "border-green-200 dark:border-green-800",
  },
  unknown: {
    icon: "lucide:brain",
    name: "Unknown",
    color: "text-muted-foreground",
    bgColor: "bg-muted/30",
    borderColor: "border-muted",
  },
};

export function detectProvider(modelId: string, ownedBy: string): ProviderInfo {
  const id = modelId?.toLowerCase() || "";
  const owner = ownedBy?.toLowerCase() || "";

  // Azure detection (must come before OpenAI since Azure models often contain "gpt")
  if (owner.includes("azure") || id.includes("azure") || owner.includes("microsoft")) {
    return PROVIDERS.azure;
  }

  // OpenAI detection
  if (id.includes("gpt") || owner.includes("openai") || id.includes("openai")) {
    return PROVIDERS.openai;
  }

  // Anthropic detection
  if (id.includes("claude") || owner.includes("anthropic") || id.includes("anthropic")) {
    return PROVIDERS.anthropic;
  }

  // Mistral detection
  if (id.includes("mistral") || owner.includes("mistral")) {
    return PROVIDERS.mistral;
  }

  // Meta detection
  if (id.includes("llama") || owner.includes("meta") || id.includes("meta")) {
    return PROVIDERS.meta;
  }

  // Google detection
  if (id.includes("gemini") || owner.includes("google") || id.includes("google")) {
    return PROVIDERS.google;
  }

  // AWS detection
  if (id.includes("bedrock") || owner.includes("aws") || id.includes("aws")) {
    return PROVIDERS.aws;
  }

  // OpenRouter detection
  if (owner.includes("openrouter") || id.includes("openrouter")) {
    return PROVIDERS.openrouter;
  }

  // Unknown provider
  return {
    ...PROVIDERS.unknown,
    name: ownedBy || "Unknown",
  };
}

export function getAllProviders(): string[] {
  return Object.keys(PROVIDERS).filter(key => key !== 'unknown');
}

export function getProviderByName(name: string): ProviderInfo {
  const key = name.toLowerCase();
  return PROVIDERS[key] || { ...PROVIDERS.unknown, name };
}