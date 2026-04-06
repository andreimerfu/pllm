import { useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/hooks/use-toast";
import { useProviderProfiles, useCreateProviderProfile } from "@/hooks/useProviders";
import { createModel, testModelConnection, discoverModels } from "@/lib/api";
import type { CreateModelRequest, ProviderConfig, ProviderProfile } from "@/types/api";

// Provider definitions with icons, colors, and placeholder hints
// logoBg provides a light-enough surface for brand SVG logos to remain visible in dark mode
const PROVIDERS = [
  {
    value: "openai",
    label: "OpenAI",
    icon: "logos:openai-icon",
    color: "text-emerald-600 dark:text-emerald-400",
    bgColor: "bg-emerald-50 dark:bg-emerald-950/30",
    borderColor: "border-emerald-200 dark:border-emerald-800",
    ringColor: "ring-emerald-500",
    logoBg: "bg-white dark:bg-zinc-200",
    accent: "#10b981",
    models: ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "o1", "o1-mini", "o3-mini"],
    keyPlaceholder: "sk-... or ${OPENAI_API_KEY}",
    keyHint: "Found in platform.openai.com/api-keys",
  },
  {
    value: "anthropic",
    label: "Anthropic",
    icon: "logos:anthropic",
    color: "text-orange-600 dark:text-orange-400",
    bgColor: "bg-orange-50 dark:bg-orange-950/30",
    borderColor: "border-orange-200 dark:border-orange-800",
    ringColor: "ring-orange-500",
    logoBg: "bg-white dark:bg-zinc-200",
    accent: "#f97316",
    models: ["claude-opus-4-5", "claude-sonnet-4-5", "claude-haiku-4-5-20251001", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"],
    authModes: ["api_key", "oauth_token"] as const,
    keyPlaceholder: "sk-ant-... or ${ANTHROPIC_API_KEY}",
    keyHint: "Found in console.anthropic.com/settings/keys",
    oauthPlaceholder: "Bearer token from Claude Max subscription",
    oauthHint: "Long-lived OAuth token from your Claude Max subscription",
  },
  {
    value: "azure",
    label: "Azure OpenAI",
    icon: "logos:microsoft-azure",
    color: "text-blue-700 dark:text-blue-300",
    bgColor: "bg-blue-50 dark:bg-blue-950/30",
    borderColor: "border-blue-200 dark:border-blue-800",
    ringColor: "ring-blue-500",
    logoBg: "bg-white dark:bg-zinc-200",
    accent: "#3b82f6",
    models: ["gpt-4o", "gpt-4", "gpt-35-turbo"],
    keyPlaceholder: "${AZURE_API_KEY}",
    keyHint: "Found in Azure Portal > your resource > Keys",
  },
  {
    value: "bedrock",
    label: "AWS Bedrock",
    icon: "logos:aws",
    color: "text-yellow-600 dark:text-yellow-400",
    bgColor: "bg-yellow-50 dark:bg-yellow-950/30",
    borderColor: "border-yellow-200 dark:border-yellow-800",
    ringColor: "ring-yellow-500",
    logoBg: "bg-white dark:bg-zinc-200",
    accent: "#f59e0b",
    models: ["anthropic.claude-3-5-sonnet-20241022-v2:0", "amazon.titan-text-premier-v1:0"],
    keyPlaceholder: "${AWS_ACCESS_KEY_ID}",
    keyHint: "Uses AWS credentials (access key + secret key)",
  },
  {
    value: "vertex",
    label: "Google Vertex AI",
    icon: "logos:google-cloud",
    color: "text-red-600 dark:text-red-400",
    bgColor: "bg-red-50 dark:bg-red-950/30",
    borderColor: "border-red-200 dark:border-red-800",
    ringColor: "ring-red-500",
    logoBg: "bg-white dark:bg-zinc-200",
    accent: "#ef4444",
    models: ["gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"],
    keyPlaceholder: "${GOOGLE_API_KEY}",
    keyHint: "Uses Google Cloud service account credentials",
  },
  {
    value: "openrouter",
    label: "OpenRouter",
    icon: "solar:global-linear",
    color: "text-purple-600 dark:text-purple-400",
    bgColor: "bg-purple-50 dark:bg-purple-950/30",
    borderColor: "border-purple-200 dark:border-purple-800",
    ringColor: "ring-purple-500",
    logoBg: "bg-purple-100 dark:bg-purple-200",
    accent: "#a855f7",
    models: ["openai/gpt-4o", "anthropic/claude-sonnet-4-20250514", "google/gemini-2.0-flash-exp"],
    keyPlaceholder: "sk-or-... or ${OPENROUTER_API_KEY}",
    keyHint: "Found in openrouter.ai/keys",
  },
];

const STEPS = [
  { id: 1, label: "Provider", description: "Choose LLM provider", icon: "solar:star-shine-linear" },
  { id: 2, label: "Authentication", description: "API credentials", icon: icons.keys },
  { id: 3, label: "Model", description: "Name & model ID", icon: icons.models },
  { id: 4, label: "Capabilities", description: "Features & limits", icon: icons.settings },
  { id: 5, label: "Review & Test", description: "Confirm & deploy", icon: icons.check },
];

export default function AddModel() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  // Wizard step
  const [step, setStep] = useState(1);

  // Provider
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null);

  // Provider profiles
  const [selectedProfileId, setSelectedProfileId] = useState<string | null>(null);
  const [saveAsProfile, setSaveAsProfile] = useState(false);
  const [profileName, setProfileName] = useState("");
  const { data: providerProfiles } = useProviderProfiles();
  const createProfileMutation = useCreateProviderProfile();

  // Form state
  const [modelName, setModelName] = useState("");
  const [providerModel, setProviderModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [oauthToken, setOauthToken] = useState("");
  const [anthropicAuthMode, setAnthropicAuthMode] = useState<"api_key" | "oauth_token">("api_key");
  const [baseUrl, setBaseUrl] = useState("");

  // Discover models state
  const [discoveredModels, setDiscoveredModels] = useState<string[]>([]);
  const [discoverLoading, setDiscoverLoading] = useState(false);

  // Azure
  const [azureDeployment, setAzureDeployment] = useState("");
  const [azureEndpoint, setAzureEndpoint] = useState("");
  const [apiVersion, setApiVersion] = useState("2024-06-01");

  // Bedrock
  const [awsRegion, setAwsRegion] = useState("us-east-1");
  const [awsAccessKeyId, setAwsAccessKeyId] = useState("");
  const [awsSecretKey, setAwsSecretKey] = useState("");

  // Vertex
  const [vertexProject, setVertexProject] = useState("");
  const [vertexLocation, setVertexLocation] = useState("us-central1");

  // Capabilities
  const [supportsStreaming, setSupportsStreaming] = useState(true);
  const [supportsVision, setSupportsVision] = useState(false);
  const [supportsFunctions, setSupportsFunctions] = useState(true);

  // Advanced / Config
  const [rpm, setRpm] = useState("");
  const [tpm, setTpm] = useState("");
  const [priority, setPriority] = useState("");
  const [weight, setWeight] = useState("");
  const [inputCost, setInputCost] = useState("");
  const [outputCost, setOutputCost] = useState("");
  const [timeoutSeconds, setTimeoutSeconds] = useState("");
  const [tags, setTags] = useState("");
  const [defaultReasoningEffort, setDefaultReasoningEffort] = useState("");

  // Test connection state
  const [testResult, setTestResult] = useState<{
    success: boolean;
    message: string;
    latency?: string;
  } | null>(null);
  const [testLoading, setTestLoading] = useState(false);

  const providerInfo = PROVIDERS.find((p) => p.value === selectedProvider);

  // Build provider config from current form state
  const buildProviderConfig = useCallback((): ProviderConfig => {
    const provider: ProviderConfig = {
      type: selectedProvider!,
      model: providerModel || "test",
    };
    if (selectedProvider === "anthropic" && anthropicAuthMode === "oauth_token") {
      if (oauthToken) provider.oauth_token = oauthToken;
    } else {
      if (apiKey) provider.api_key = apiKey;
    }
    if (baseUrl) provider.base_url = baseUrl;
    if (selectedProvider === "azure") {
      if (azureDeployment) provider.azure_deployment = azureDeployment;
      if (azureEndpoint) provider.azure_endpoint = azureEndpoint;
      if (apiVersion) provider.api_version = apiVersion;
    }
    if (selectedProvider === "bedrock") {
      if (awsRegion) provider.aws_region_name = awsRegion;
      if (awsAccessKeyId) provider.aws_access_key_id = awsAccessKeyId;
      if (awsSecretKey) provider.aws_secret_access_key = awsSecretKey;
    }
    if (selectedProvider === "vertex") {
      if (vertexProject) provider.vertex_project = vertexProject;
      if (vertexLocation) provider.vertex_location = vertexLocation;
    }
    return provider;
  }, [selectedProvider, providerModel, apiKey, oauthToken, anthropicAuthMode, baseUrl, azureDeployment, azureEndpoint, apiVersion, awsRegion, awsAccessKeyId, awsSecretKey, vertexProject, vertexLocation]);

  // Reset profile selection when provider changes
  useEffect(() => {
    setSelectedProfileId(null);
    setSaveAsProfile(false);
    setProfileName("");
  }, [selectedProvider]);

  // Clear test result and discovered models when provider config changes
  useEffect(() => {
    setTestResult(null);
    setDiscoveredModels([]);
  }, [selectedProvider, apiKey, oauthToken, anthropicAuthMode, baseUrl, azureDeployment, azureEndpoint, apiVersion, awsAccessKeyId, awsSecretKey, awsRegion, vertexProject, vertexLocation]);

  const handleDiscoverModels = async () => {
    if (!selectedProvider || !providerValid) return;
    setDiscoverLoading(true);
    try {
      const result = await discoverModels(buildProviderConfig());
      setDiscoveredModels(result.models || []);
      if (result.models?.length === 0) {
        toast({ title: "No models found", description: "The provider returned an empty model list." });
      }
    } catch (err: any) {
      toast({
        title: "Failed to fetch models",
        description: err.response?.data?.error || err.message || "Could not fetch model list",
        variant: "destructive",
      });
    } finally {
      setDiscoverLoading(false);
    }
  };

  const handleTestConnection = async () => {
    if (!selectedProvider || !providerValid) return;
    setTestLoading(true);
    setTestResult(null);
    try {
      const result = await testModelConnection(buildProviderConfig());
      setTestResult(result);
    } catch (err: any) {
      setTestResult({
        success: false,
        message: err.response?.data?.message || err.message || "Connection test failed",
      });
    } finally {
      setTestLoading(false);
    }
  };

  const mutation = useMutation({
    mutationFn: (data: CreateModelRequest) => createModel(data),
    onSuccess: () => {
      toast({
        title: "Model created",
        description: `Model "${modelName}" has been created and is ready to serve requests.`,
      });
      queryClient.invalidateQueries({ queryKey: ["models"] });
      queryClient.invalidateQueries({ queryKey: ["admin-models"] });
      navigate("/models");
    },
    onError: (error: any) => {
      const message =
        error.response?.data?.error || error.message || "Failed to create model";
      toast({ title: "Error creating model", description: message, variant: "destructive" });
    },
  });

  const handleSubmit = async () => {
    if (!selectedProvider || !modelName || !providerModel) return;

    let profileId = selectedProfileId;

    // If saving as a new profile, create it first
    if (!profileId && saveAsProfile && profileName.trim()) {
      try {
        const config: Record<string, string> = {};
        if (selectedProvider === "anthropic" && anthropicAuthMode === "oauth_token") {
          if (oauthToken) config.oauth_token = oauthToken;
        } else {
          if (apiKey) config.api_key = apiKey;
        }
        if (baseUrl) config.base_url = baseUrl;
        if (selectedProvider === "azure") {
          if (azureEndpoint) config.azure_endpoint = azureEndpoint;
          if (azureDeployment) config.azure_deployment = azureDeployment;
          if (apiVersion) config.api_version = apiVersion;
        }
        if (selectedProvider === "bedrock") {
          if (awsRegion) config.aws_region_name = awsRegion;
          if (awsAccessKeyId) config.aws_access_key_id = awsAccessKeyId;
          if (awsSecretKey) config.aws_secret_access_key = awsSecretKey;
        }
        if (selectedProvider === "vertex") {
          if (vertexProject) config.vertex_project = vertexProject;
          if (vertexLocation) config.vertex_location = vertexLocation;
        }
        const resp: any = await createProfileMutation.mutateAsync({
          name: profileName.trim(),
          type: selectedProvider,
          config,
        });
        profileId = resp?.data?.id || resp?.id;
      } catch (err: any) {
        toast({
          title: "Failed to save profile",
          description: err.response?.data?.error || err.message || "Could not save credentials profile",
          variant: "destructive",
        });
        return;
      }
    }

    const provider: ProviderConfig = {
      type: selectedProvider,
      model: providerModel,
    };

    // If using a profile, only send type + model; otherwise send full inline credentials
    if (!profileId) {
      const fullProvider = buildProviderConfig();
      Object.assign(provider, fullProvider);
    }

    provider.model = providerModel;
    if (defaultReasoningEffort) provider.reasoning_effort = defaultReasoningEffort;

    const request: CreateModelRequest = {
      model_name: modelName,
      provider,
      model_info: {
        mode: "chat",
        supports_streaming: supportsStreaming,
        supports_vision: supportsVision,
        supports_functions: supportsFunctions,
      },
    };

    if (profileId) request.provider_profile_id = profileId;

    if (rpm) request.rpm = parseInt(rpm);
    if (tpm) request.tpm = parseInt(tpm);
    if (priority) request.priority = parseInt(priority);
    if (weight) request.weight = parseFloat(weight);
    if (inputCost) request.input_cost_per_token = parseFloat(inputCost);
    if (outputCost) request.output_cost_per_token = parseFloat(outputCost);
    if (timeoutSeconds) request.timeout_seconds = parseInt(timeoutSeconds);
    if (tags)
      request.tags = tags
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean);

    mutation.mutate(request);
  };

  const providerValid = (() => {
    if (!selectedProvider) return false;
    switch (selectedProvider) {
      case "anthropic":
        return anthropicAuthMode === "oauth_token" ? !!oauthToken : !!apiKey;
      case "openrouter":
        return !!apiKey;
      case "azure":
        return !!azureEndpoint;
      case "bedrock":
        return !!awsAccessKeyId && !!awsSecretKey;
      case "vertex":
        return !!apiKey;
      default:
        return true;
    }
  })();

  // Per-step validation
  const stepValid = (s: number): boolean => {
    switch (s) {
      case 1:
        return !!selectedProvider;
      case 2:
        return !!selectedProfileId || providerValid;
      case 3:
        return !!modelName && !!providerModel;
      case 4:
        return true; // capabilities are always valid (toggles have defaults)
      case 5:
        return !!selectedProvider && !!modelName && !!providerModel && (!!selectedProfileId || providerValid);
      default:
        return false;
    }
  };

  const handleNext = () => {
    if (step < 5 && stepValid(step)) {
      setStep(step + 1);
    }
  };

  const handleBack = () => {
    if (step > 1) {
      setStep(step - 1);
    }
  };


  // ─── Step 1: Provider ───────────────────────────────────────────────
  const renderProviderStep = () => (
    <div>
      <div className="mb-8">
        <div className="flex items-center gap-3 mb-2">
          <div className="h-8 w-1 rounded-full bg-gradient-to-b from-teal-400 to-teal-600" />
          <h2 className="text-2xl font-bold tracking-tight">Choose Provider</h2>
        </div>
        <p className="text-muted-foreground ml-[19px]">
          Select the LLM provider for this model instance
        </p>
      </div>
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
        {PROVIDERS.map((p) => {
          const isSelected = selectedProvider === p.value;
          return (
            <button
              key={p.value}
              type="button"
              onClick={() => setSelectedProvider(p.value)}
              className={`group relative flex flex-col items-center gap-5 p-8 rounded-2xl border transition-all duration-200 hover:shadow-md active:scale-[0.98] ${
                isSelected
                  ? "border-teal-500 bg-teal-500/5 dark:bg-teal-500/8 shadow-lg"
                  : "border-border hover:border-border hover:bg-muted/40"
              }`}
              style={isSelected ? { boxShadow: `0 0 0 1px ${p.accent}22, 0 4px 24px ${p.accent}15` } : undefined}
            >
              {isSelected && (
                <div className="absolute top-3 right-3 w-5 h-5 rounded-full bg-teal-500 flex items-center justify-center">
                  <Icon icon={icons.check} className="h-3 w-3 text-white" />
                </div>
              )}
              {/* Logo pill — light surface so brand SVGs stay visible in dark mode */}
              <div className={`w-14 h-14 rounded-2xl flex items-center justify-center ${p.logoBg} shadow-sm`}>
                <Icon icon={p.icon} width="30" height="30" />
              </div>
              <span className={`text-sm font-semibold tracking-tight ${isSelected ? "text-foreground" : "text-muted-foreground group-hover:text-foreground"}`}>
                {p.label}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );

  // ─── Step 2: Authentication ─────────────────────────────────────────
  const renderAuthStep = () => {
    if (!providerInfo) return null;

    const matchingProfiles: ProviderProfile[] = providerProfiles?.filter((p: ProviderProfile) => p.type === selectedProvider) || [];

    return (
      <div>
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="h-8 w-1 rounded-full bg-gradient-to-b from-teal-400 to-teal-600" />
            <h2 className="text-2xl font-bold tracking-tight">Authentication</h2>
          </div>
          <p className="text-muted-foreground ml-[19px]">
            API credentials for{" "}
            <span className={providerInfo.color}>{providerInfo.label}</span>
          </p>
        </div>

        <div className="space-y-6 max-w-xl">
          {/* Saved Credentials */}
          {matchingProfiles.length > 0 && (
            <>
              <div>
                <h3 className="text-xs uppercase tracking-widest text-muted-foreground font-semibold mb-3">Saved Credentials</h3>
                <div className="grid grid-cols-1 gap-2">
                  {matchingProfiles.map((profile) => {
                    const isSelected = selectedProfileId === profile.id;
                    const maskedKey = profile.config.api_key
                      ? profile.config.api_key.slice(0, 6) + "····"
                      : profile.config.oauth_token
                      ? "oauth····"
                      : profile.config.azure_endpoint
                      ? "azure····"
                      : profile.config.aws_access_key_id
                      ? "aws····"
                      : "configured";
                    return (
                      <button
                        key={profile.id}
                        type="button"
                        onClick={() => setSelectedProfileId(isSelected ? null : profile.id)}
                        className={`flex items-center gap-4 p-4 rounded-xl border-2 transition-all duration-200 text-left ${
                          isSelected
                            ? "border-teal-500 bg-teal-500/5 dark:bg-teal-500/10 ring-2 ring-teal-500/30"
                            : "border-border/50 hover:border-muted-foreground/30 hover:bg-muted/30"
                        }`}
                      >
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-bold truncate">{profile.name}</p>
                          <p className="text-xs font-mono text-muted-foreground mt-0.5">{maskedKey}</p>
                        </div>
                        {(profile.model_count ?? 0) > 0 && (
                          <Badge variant="secondary" className="text-[10px] flex-shrink-0">
                            {profile.model_count} model{profile.model_count !== 1 ? "s" : ""}
                          </Badge>
                        )}
                        {isSelected && (
                          <div className="w-5 h-5 rounded-full bg-teal-500 flex items-center justify-center flex-shrink-0">
                            <Icon icon={icons.check} className="h-3 w-3 text-white" />
                          </div>
                        )}
                      </button>
                    );
                  })}
                </div>
              </div>

              {selectedProfileId ? (
                <button
                  type="button"
                  onClick={() => setSelectedProfileId(null)}
                  className="text-sm text-teal-600 dark:text-teal-500 hover:underline"
                >
                  Use different credentials
                </button>
              ) : (
                <div className="relative">
                  <Separator />
                  <span className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-card px-3 text-xs text-muted-foreground">
                    Or enter new credentials
                  </span>
                </div>
              )}
            </>
          )}

          {/* Credential form -- hidden when a profile is selected */}
          {!selectedProfileId && (
            <>
          {/* Anthropic auth mode toggle */}
          {selectedProvider === "anthropic" && (
            <div className="flex gap-1.5 p-1 bg-muted/60 rounded-xl w-fit">
              <button
                type="button"
                onClick={() => setAnthropicAuthMode("api_key")}
                className={`px-4 py-2 text-sm rounded-lg transition-all ${
                  anthropicAuthMode === "api_key"
                    ? "bg-background shadow-sm font-semibold text-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                API Key
              </button>
              <button
                type="button"
                onClick={() => setAnthropicAuthMode("oauth_token")}
                className={`px-4 py-2 text-sm rounded-lg transition-all ${
                  anthropicAuthMode === "oauth_token"
                    ? "bg-background shadow-sm font-semibold text-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                OAuth Token (Claude Max)
              </button>
            </div>
          )}

          {/* OAuth token input for Anthropic */}
          {selectedProvider === "anthropic" && anthropicAuthMode === "oauth_token" ? (
            <div className="space-y-2">
              <Label htmlFor="oauthToken" className="text-sm font-medium">
                OAuth Bearer Token <span className="text-destructive">*</span>
              </Label>
              <Input
                id="oauthToken"
                type="password"
                placeholder={(providerInfo as any).oauthPlaceholder || "Bearer token"}
                value={oauthToken}
                onChange={(e) => setOauthToken(e.target.value)}
                className="font-mono"
                required
              />
              <div className="flex items-start gap-2 text-xs text-muted-foreground">
                <Icon icon={icons.info} className="h-3 w-3 mt-0.5 flex-shrink-0" />
                <p>{(providerInfo as any).oauthHint || "Long-lived OAuth token"}</p>
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              <Label htmlFor="apiKey" className="text-sm font-medium">
                API Key
                {["anthropic", "openrouter", "vertex"].includes(selectedProvider!) && (
                  <span className="text-destructive"> *</span>
                )}
              </Label>
              <Input
                id="apiKey"
                type="password"
                placeholder={providerInfo.keyPlaceholder}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="font-mono"
                required={["anthropic", "openrouter", "vertex"].includes(selectedProvider!)}
              />
              <div className="flex items-start gap-2 text-xs text-muted-foreground">
                <Icon icon={icons.info} className="h-3 w-3 mt-0.5 flex-shrink-0" />
                <div>
                  <p>{providerInfo.keyHint}</p>
                  <p className="mt-1">
                    Supports environment variable references:{" "}
                    <code className="bg-muted px-1 py-0.5 rounded font-mono text-[11px]">
                      {"${OPENAI_API_KEY}"}
                    </code>
                  </p>
                </div>
              </div>
            </div>
          )}

          {selectedProvider !== "bedrock" &&
            selectedProvider !== "vertex" &&
            selectedProvider !== "azure" && (
              <div className="space-y-2">
                <Label htmlFor="baseUrl" className="text-sm font-medium">
                  Base URL{" "}
                  <span className="text-muted-foreground font-normal">(optional)</span>
                </Label>
                <Input
                  id="baseUrl"
                  placeholder="Leave empty for default endpoint"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                />
              </div>
            )}

          {/* Azure-specific */}
          {selectedProvider === "azure" && (
            <div className="space-y-4 p-5 rounded-xl border border-blue-200/40 dark:border-blue-800/40 bg-blue-50/30 dark:bg-blue-950/20">
              <div className="flex items-center gap-2">
                <Icon icon="logos:microsoft-azure" width="16" height="16" />
                <span className="text-sm font-semibold">Azure Configuration</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label className="text-sm">
                    Endpoint URL <span className="text-destructive">*</span>
                  </Label>
                  <Input
                    placeholder="https://your-resource.openai.azure.com"
                    value={azureEndpoint}
                    onChange={(e) => setAzureEndpoint(e.target.value)}
                    required
                  />
                  <p className="text-xs text-muted-foreground">
                    Found in Azure Portal &rarr; your OpenAI resource &rarr; Keys and Endpoint
                  </p>
                </div>
                <div className="space-y-2">
                  <Label className="text-sm">Deployment Name</Label>
                  <Input
                    placeholder="my-gpt4-deployment"
                    value={azureDeployment}
                    onChange={(e) => setAzureDeployment(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label className="text-sm">API Version</Label>
                  <Input
                    placeholder="2024-06-01"
                    value={apiVersion}
                    onChange={(e) => setApiVersion(e.target.value)}
                  />
                </div>
              </div>
            </div>
          )}

          {/* Bedrock-specific */}
          {selectedProvider === "bedrock" && (
            <div className="space-y-4 p-5 rounded-xl border border-yellow-200/40 dark:border-yellow-800/40 bg-yellow-50/30 dark:bg-yellow-950/20">
              <div className="flex items-center gap-2">
                <Icon icon="logos:aws" width="20" height="14" />
                <span className="text-sm font-semibold">AWS Configuration</span>
              </div>
              <div className="grid grid-cols-1 gap-4">
                <div className="space-y-2">
                  <Label className="text-sm">Region</Label>
                  <Input
                    placeholder="us-east-1"
                    value={awsRegion}
                    onChange={(e) => setAwsRegion(e.target.value)}
                  />
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label className="text-sm">
                      Access Key ID <span className="text-destructive">*</span>
                    </Label>
                    <Input
                      type="password"
                      placeholder="${AWS_ACCESS_KEY_ID}"
                      value={awsAccessKeyId}
                      onChange={(e) => setAwsAccessKeyId(e.target.value)}
                      className="font-mono"
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-sm">
                      Secret Access Key <span className="text-destructive">*</span>
                    </Label>
                    <Input
                      type="password"
                      placeholder="${AWS_SECRET_ACCESS_KEY}"
                      value={awsSecretKey}
                      onChange={(e) => setAwsSecretKey(e.target.value)}
                      className="font-mono"
                      required
                    />
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Vertex-specific */}
          {selectedProvider === "vertex" && (
            <div className="space-y-4 p-5 rounded-xl border border-red-200/40 dark:border-red-800/40 bg-red-50/30 dark:bg-red-950/20">
              <div className="flex items-center gap-2">
                <Icon icon="logos:google-cloud" width="20" height="16" />
                <span className="text-sm font-semibold">Vertex AI Configuration</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label className="text-sm">Project ID</Label>
                  <Input
                    placeholder="my-gcp-project"
                    value={vertexProject}
                    onChange={(e) => setVertexProject(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label className="text-sm">Location</Label>
                  <Input
                    placeholder="us-central1"
                    value={vertexLocation}
                    onChange={(e) => setVertexLocation(e.target.value)}
                  />
                </div>
              </div>
            </div>
          )}

          {/* Save as profile */}
          <div className="flex items-center gap-3 mt-4 pt-4 border-t border-border">
            <input
              type="checkbox"
              id="saveProfile"
              checked={saveAsProfile}
              onChange={(e) => setSaveAsProfile(e.target.checked)}
              className="rounded border-border"
            />
            <Label htmlFor="saveProfile" className="text-sm cursor-pointer">
              Save these credentials for reuse
            </Label>
          </div>
          {saveAsProfile && (
            <div className="mt-3">
              <Label className="text-sm">Profile name</Label>
              <Input
                placeholder="e.g., Our OpenRouter Account"
                value={profileName}
                onChange={(e) => setProfileName(e.target.value)}
                className="mt-1 max-w-md"
              />
            </div>
          )}
            </>
          )}
        </div>
      </div>
    );
  };

  // ─── Step 3: Model ──────────────────────────────────────────────────
  const renderModelStep = () => {
    if (!providerInfo) return null;
    return (
      <div>
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="h-8 w-1 rounded-full bg-gradient-to-b from-teal-400 to-teal-600" />
            <h2 className="text-2xl font-bold tracking-tight">Model Configuration</h2>
          </div>
          <p className="text-muted-foreground ml-[19px]">
            Configure the model instance for{" "}
            <span className={providerInfo.color}>{providerInfo.label}</span>
          </p>
        </div>

        <div className="space-y-6 max-w-xl">
          {/* Model Name */}
          <div className="space-y-2">
            <Label htmlFor="modelName" className="text-sm font-medium">
              Model Name <span className="text-destructive">*</span>
            </Label>
            <Input
              id="modelName"
              placeholder="e.g. my-gpt-4o, fast-claude"
              value={modelName}
              onChange={(e) => setModelName(e.target.value)}
              required
            />
            <p className="text-xs text-muted-foreground">
              The user-facing name used in API requests (e.g.{" "}
              <code className="text-[11px] bg-muted px-1 py-0.5 rounded font-mono">
                model: "my-gpt-4o"
              </code>
              )
            </p>
          </div>

          {/* Provider Model */}
          <div className="space-y-2">
            <Label htmlFor="providerModel" className="text-sm font-medium">
              Provider Model ID <span className="text-destructive">*</span>
            </Label>
            <Input
              id="providerModel"
              placeholder={providerInfo.models[0] || "Model identifier"}
              value={providerModel}
              onChange={(e) => setProviderModel(e.target.value)}
              required
            />
            {/* Show discovered models if available, otherwise static defaults */}
            {(discoveredModels.length > 0 ? discoveredModels : providerInfo.models).length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-2">
                {(discoveredModels.length > 0 ? discoveredModels : providerInfo.models).map((m) => (
                  <Badge
                    key={m}
                    variant="outline"
                    className={`cursor-pointer hover:bg-accent transition-colors text-xs ${
                      providerModel === m ? "border-teal-500 bg-teal-500/10 text-teal-600 dark:text-teal-500" : ""
                    }`}
                    onClick={() => setProviderModel(m)}
                  >
                    {m}
                  </Badge>
                ))}
              </div>
            )}
          </div>

          {/* Discover Models button */}
          {selectedProvider === "anthropic" && providerValid && (
            <>
              <Separator />
              <div className="flex items-center gap-3 flex-wrap">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={!providerValid || discoverLoading}
                  onClick={handleDiscoverModels}
                >
                  {discoverLoading ? (
                    <Icon icon="solar:refresh-circle-linear" className="h-4 w-4 mr-2 animate-spin" />
                  ) : (
                    <Icon icon="solar:star-shine-linear" className="h-4 w-4 mr-2" />
                  )}
                  Discover Models
                </Button>
                {discoveredModels.length > 0 && (
                  <span className="text-sm text-teal-600 dark:text-teal-500">
                    {discoveredModels.length} models fetched — click a badge above to select
                  </span>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    );
  };

  // ─── Step 4: Capabilities & Config ──────────────────────────────────
  const renderCapabilitiesStep = () => (
    <div>
      <div className="mb-8">
        <div className="flex items-center gap-3 mb-2">
          <div className="h-8 w-1 rounded-full bg-gradient-to-b from-teal-400 to-teal-600" />
          <h2 className="text-2xl font-bold tracking-tight">Capabilities & Config</h2>
        </div>
        <p className="text-muted-foreground ml-[19px]">
          Declare features, limits, and pricing for this model
        </p>
      </div>

      <div className="space-y-8">
        {/* Capabilities - toggle cards */}
        <div>
          <h3 className="text-xs uppercase tracking-widest text-muted-foreground font-semibold mb-3">Capabilities</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            {[
              { label: "Streaming", desc: "Server-sent events", icon: "solar:star-shine-linear", iconColor: "text-blue-500", checked: supportsStreaming, onChange: setSupportsStreaming },
              { label: "Vision", desc: "Image understanding", icon: icons.eye, iconColor: "text-purple-500", checked: supportsVision, onChange: setSupportsVision },
              { label: "Function Calling", desc: "Tool use support", icon: "solar:settings-minimalistic-linear", iconColor: "text-green-500", checked: supportsFunctions, onChange: setSupportsFunctions },
            ].map((cap) => (
              <div
                key={cap.label}
                className={`flex items-center justify-between p-4 rounded-xl border transition-colors ${
                  cap.checked ? "border-teal-500/30 bg-teal-500/5" : "border-border/50"
                }`}
              >
                <div className="flex items-center gap-3">
                  <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${cap.checked ? "bg-teal-500/10" : "bg-muted/50"}`}>
                    <Icon icon={cap.icon} className={`h-4 w-4 ${cap.checked ? cap.iconColor : "text-muted-foreground"}`} />
                  </div>
                  <div>
                    <Label className="text-sm cursor-pointer font-medium">{cap.label}</Label>
                    <p className="text-[11px] text-muted-foreground">{cap.desc}</p>
                  </div>
                </div>
                <Switch checked={cap.checked} onCheckedChange={cap.onChange} />
              </div>
            ))}
          </div>
        </div>

        {/* Rate Limits */}
        <div>
          <h3 className="text-xs uppercase tracking-widest text-muted-foreground font-semibold mb-3">Rate Limits</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 max-w-xl">
            <div className="space-y-2">
              <Label className="text-sm">
                RPM <span className="text-muted-foreground font-normal">(Requests/min)</span>
              </Label>
              <Input type="number" placeholder="100" value={rpm} onChange={(e) => setRpm(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label className="text-sm">
                TPM <span className="text-muted-foreground font-normal">(Tokens/min)</span>
              </Label>
              <Input type="number" placeholder="100,000" value={tpm} onChange={(e) => setTpm(e.target.value)} />
            </div>
          </div>
        </div>

        {/* Load Balancing */}
        <div>
          <h3 className="text-xs uppercase tracking-widest text-muted-foreground font-semibold mb-3">Load Balancing</h3>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 max-w-xl">
            <div className="space-y-2">
              <Label className="text-sm">Priority (1-100)</Label>
              <Input type="number" placeholder="50" min="1" max="100" value={priority} onChange={(e) => setPriority(e.target.value)} />
              <p className="text-xs text-muted-foreground">Higher = preferred</p>
            </div>
            <div className="space-y-2">
              <Label className="text-sm">Weight</Label>
              <Input type="number" step="0.1" placeholder="1.0" value={weight} onChange={(e) => setWeight(e.target.value)} />
              <p className="text-xs text-muted-foreground">For weighted round-robin</p>
            </div>
            <div className="space-y-2">
              <Label className="text-sm">Timeout (s)</Label>
              <Input type="number" placeholder="60" value={timeoutSeconds} onChange={(e) => setTimeoutSeconds(e.target.value)} />
              <p className="text-xs text-muted-foreground">Request timeout</p>
            </div>
          </div>
        </div>

        {/* Pricing */}
        <div>
          <h3 className="text-xs uppercase tracking-widest text-muted-foreground font-semibold mb-3">Pricing</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 max-w-xl">
            <div className="space-y-2">
              <Label className="text-sm">Input Cost / Token</Label>
              <Input type="number" step="0.000001" placeholder="0.000005" value={inputCost} onChange={(e) => setInputCost(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label className="text-sm">Output Cost / Token</Label>
              <Input type="number" step="0.000001" placeholder="0.000015" value={outputCost} onChange={(e) => setOutputCost(e.target.value)} />
            </div>
          </div>
        </div>

        {/* Tags + Reasoning Effort */}
        <div>
          <h3 className="text-xs uppercase tracking-widest text-muted-foreground font-semibold mb-3">Tags & Reasoning</h3>
          <div className="space-y-4 max-w-xl">
            <div className="space-y-2">
              <Label className="text-sm">Tags</Label>
              <Input
                placeholder="production, fast, custom"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">Comma-separated labels for filtering and grouping</p>
            </div>
            <div className="space-y-2">
              <Label className="text-sm">Default Reasoning Effort</Label>
              <Select value={defaultReasoningEffort} onValueChange={setDefaultReasoningEffort}>
                <SelectTrigger>
                  <SelectValue placeholder="None (use provider default)" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">Low</SelectItem>
                  <SelectItem value="medium">Medium</SelectItem>
                  <SelectItem value="high">High</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Default reasoning effort for reasoning models (o1, o3, GPT-5, etc.). Applied when callers don't specify one.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );

  // ─── Step 5: Review & Test ──────────────────────────────────────────
  const renderReviewStep = () => {
    if (!providerInfo) return null;

    const summaryRows: { label: string; value: string; mono?: boolean }[] = [
      { label: "Provider", value: providerInfo.label },
      { label: "Model Name", value: modelName, mono: true },
      { label: "Provider Model ID", value: providerModel, mono: true },
    ];

    // Auth summary (masked)
    if (selectedProfileId) {
      const profile = providerProfiles?.find((p: ProviderProfile) => p.id === selectedProfileId);
      summaryRows.push({ label: "Credentials", value: profile ? `Profile: ${profile.name}` : "Saved profile" });
    } else if (selectedProvider === "anthropic" && anthropicAuthMode === "oauth_token") {
      summaryRows.push({ label: "Auth Mode", value: "OAuth Token" });
      summaryRows.push({ label: "OAuth Token", value: oauthToken ? "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" : "Not set" });
    } else {
      summaryRows.push({ label: "API Key", value: apiKey ? "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" : "Not set" });
    }

    if (baseUrl) summaryRows.push({ label: "Base URL", value: baseUrl, mono: true });
    if (selectedProvider === "azure") {
      if (azureEndpoint) summaryRows.push({ label: "Azure Endpoint", value: azureEndpoint, mono: true });
      if (azureDeployment) summaryRows.push({ label: "Azure Deployment", value: azureDeployment, mono: true });
      if (apiVersion) summaryRows.push({ label: "API Version", value: apiVersion, mono: true });
    }
    if (selectedProvider === "bedrock") {
      if (awsRegion) summaryRows.push({ label: "AWS Region", value: awsRegion, mono: true });
      summaryRows.push({ label: "AWS Access Key", value: awsAccessKeyId ? "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" : "Not set" });
    }
    if (selectedProvider === "vertex") {
      if (vertexProject) summaryRows.push({ label: "Vertex Project", value: vertexProject, mono: true });
      if (vertexLocation) summaryRows.push({ label: "Vertex Location", value: vertexLocation, mono: true });
    }

    // Capabilities
    const caps = [];
    if (supportsStreaming) caps.push("Streaming");
    if (supportsVision) caps.push("Vision");
    if (supportsFunctions) caps.push("Functions");
    summaryRows.push({ label: "Capabilities", value: caps.join(", ") || "None" });

    // Optional fields
    if (rpm) summaryRows.push({ label: "RPM", value: rpm });
    if (tpm) summaryRows.push({ label: "TPM", value: tpm });
    if (priority) summaryRows.push({ label: "Priority", value: priority });
    if (weight) summaryRows.push({ label: "Weight", value: weight });
    if (timeoutSeconds) summaryRows.push({ label: "Timeout", value: `${timeoutSeconds}s` });
    if (inputCost) summaryRows.push({ label: "Input Cost/Token", value: inputCost });
    if (outputCost) summaryRows.push({ label: "Output Cost/Token", value: outputCost });
    if (tags) summaryRows.push({ label: "Tags", value: tags });
    if (defaultReasoningEffort) summaryRows.push({ label: "Reasoning Effort", value: defaultReasoningEffort });

    return (
      <div>
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="h-8 w-1 rounded-full bg-gradient-to-b from-teal-400 to-teal-600" />
            <h2 className="text-2xl font-bold tracking-tight">Review & Test</h2>
          </div>
          <p className="text-muted-foreground ml-[19px]">
            Confirm all settings before creating the model
          </p>
        </div>

        <div className="space-y-6">
          {/* Summary table */}
          <div className="rounded-xl border overflow-hidden">
            <div className="divide-y divide-border">
              {summaryRows.map((row, i) => (
                <div key={i} className="flex items-center justify-between px-5 py-3.5 hover:bg-muted/30 transition-colors">
                  <span className="text-sm text-muted-foreground">{row.label}</span>
                  <span className={`text-sm font-medium ${row.mono ? "font-mono" : ""}`}>
                    {row.value || <span className="text-muted-foreground/50">--</span>}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Test Connection */}
          <div className="rounded-xl border p-5 space-y-4">
            <div className="flex items-center gap-2">
              <Icon icon="solar:wifi-linear" className="h-5 w-5 text-teal-500" />
              <h3 className="text-sm font-semibold">Test Connection</h3>
              <span className="text-xs text-muted-foreground ml-1">Verify the provider is reachable</span>
            </div>
            <div className="flex items-center gap-3 flex-wrap">
              <Button
                type="button"
                variant="outline"
                disabled={(!providerValid && !selectedProfileId) || testLoading}
                onClick={handleTestConnection}
                className="border-teal-500/30 hover:border-teal-500/60 hover:bg-teal-500/5"
              >
                {testLoading ? (
                  <Icon icon="solar:refresh-circle-linear" className="h-4 w-4 mr-2 animate-spin" />
                ) : (
                  <Icon icon="solar:wifi-linear" className="h-4 w-4 mr-2" />
                )}
                Test Connection
              </Button>
              {testResult && (
                <div
                  className={`flex items-center gap-2 text-sm ${
                    testResult.success
                      ? "text-green-600 dark:text-green-400"
                      : "text-red-600 dark:text-red-400"
                  }`}
                >
                  {testResult.success ? (
                    <Icon icon={icons.check} className="h-4 w-4 flex-shrink-0" />
                  ) : (
                    <Icon icon={icons.error} className="h-4 w-4 flex-shrink-0" />
                  )}
                  <span>{testResult.message}</span>
                  {testResult.latency && (
                    <span className="text-muted-foreground">({testResult.latency})</span>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    );
  };

  // ─── Render ─────────────────────────────────────────────────────────
  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Step Content */}
      <div className="flex-1">
        {step === 1 && renderProviderStep()}
        {step === 2 && renderAuthStep()}
        {step === 3 && renderModelStep()}
        {step === 4 && renderCapabilitiesStep()}
        {step === 5 && renderReviewStep()}
      </div>

      {/* Bottom Bar — sticks to bottom of content area */}
      <div className="sticky bottom-0 z-40 -mx-6 lg:-mx-8">
        <div className="backdrop-blur-xl bg-background/90 border-t border-border">
          <div className="px-6 lg:px-8 py-3 flex items-center justify-between gap-6">
            {/* Left: Back / Cancel */}
            <div className="w-[100px] flex-shrink-0">
              {step === 1 ? (
                <Button type="button" variant="ghost" size="sm" onClick={() => navigate("/models")} className="text-muted-foreground">
                  Cancel
                </Button>
              ) : (
                <Button type="button" variant="ghost" size="sm" onClick={handleBack} className="text-muted-foreground gap-1.5">
                  <Icon icon={icons.arrowLeft} className="h-3.5 w-3.5" />
                  Back
                </Button>
              )}
            </div>

            {/* Center: Step indicators with connecting lines */}
            <div className="flex items-center">
              {STEPS.map((s, idx) => {
                const isCompleted = step > s.id;
                const isCurrent = step === s.id;
                const canJump = s.id < step;
                return (
                  <div key={s.id} className="flex items-center">
                    <button
                      type="button"
                      disabled={!canJump}
                      onClick={() => { if (canJump) setStep(s.id); }}
                      className={`flex items-center gap-1.5 transition-all ${
                        canJump ? "cursor-pointer" : ""
                      }`}
                      aria-label={`Step ${s.id}: ${s.label}`}
                    >
                      {/* Step dot */}
                      <div className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold flex-shrink-0 transition-all ${
                        isCompleted
                          ? "bg-teal-500 text-white"
                          : isCurrent
                          ? "bg-teal-500/15 text-teal-600 dark:text-teal-400 ring-2 ring-teal-500/50"
                          : "bg-muted/80 text-muted-foreground/40"
                      }`}>
                        {isCompleted ? (
                          <Icon icon={icons.check} className="h-3 w-3" />
                        ) : (
                          <span>{s.id}</span>
                        )}
                      </div>
                      {/* Label */}
                      <span className={`hidden sm:inline text-xs transition-colors ${
                        isCurrent
                          ? "text-foreground font-semibold"
                          : isCompleted
                          ? "text-muted-foreground hover:text-foreground"
                          : "text-muted-foreground/40"
                      }`}>{s.label}</span>
                    </button>
                    {/* Connecting line */}
                    {idx < STEPS.length - 1 && (
                      <div className={`hidden sm:block w-8 h-px mx-2 transition-colors ${
                        step > s.id + 1 || (step > s.id && step > idx + 1)
                          ? "bg-teal-500/50"
                          : "bg-border"
                      }`} />
                    )}
                    {idx < STEPS.length - 1 && (
                      <div className="sm:hidden w-3" />
                    )}
                  </div>
                );
              })}
            </div>

            {/* Right: Next / Create */}
            <div className="w-[140px] flex-shrink-0 flex justify-end">
              {step < 5 ? (
                <Button
                  type="button"
                  disabled={!stepValid(step)}
                  onClick={handleNext}
                  className="bg-teal-600 hover:bg-teal-700 text-white rounded-xl px-5 gap-1.5"
                >
                  Next
                  <Icon icon={icons.arrowRight} className="h-3.5 w-3.5" />
                </Button>
              ) : (
                <Button
                  type="button"
                  disabled={mutation.isPending || !stepValid(5)}
                  onClick={handleSubmit}
                  className="bg-teal-600 hover:bg-teal-700 text-white rounded-xl px-5 gap-1.5"
                >
                  {mutation.isPending ? (
                    "Creating..."
                  ) : (
                    <>
                      <Icon icon={icons.check} className="h-3.5 w-3.5" />
                      Create Model
                    </>
                  )}
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
