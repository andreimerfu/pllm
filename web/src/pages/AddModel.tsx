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
import { createModel, testModelConnection, discoverModels } from "@/lib/api";
import type { CreateModelRequest, ProviderConfig } from "@/types/api";

// Provider definitions with icons, colors, and placeholder hints
const PROVIDERS = [
  {
    value: "openai",
    label: "OpenAI",
    icon: "logos:openai-icon",
    color: "text-emerald-600 dark:text-emerald-400",
    bgColor: "bg-emerald-50 dark:bg-emerald-950/30",
    borderColor: "border-emerald-200 dark:border-emerald-800",
    ringColor: "ring-emerald-500",
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

  const handleSubmit = () => {
    if (!selectedProvider || !modelName || !providerModel) return;

    const provider = buildProviderConfig();
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
        return providerValid;
      case 3:
        return !!modelName && !!providerModel;
      case 4:
        return true; // capabilities are always valid (toggles have defaults)
      case 5:
        return !!selectedProvider && !!modelName && !!providerModel && providerValid;
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

  // ─── Configuration Preview (for sidebar) ─────────────────────────────
  const configPreview = () => {
    const lines: string[] = ["{"];
    if (selectedProvider) lines.push(`  "provider": "${selectedProvider}",`);
    if (modelName) lines.push(`  "model_name": "${modelName}",`);
    if (providerModel) lines.push(`  "model_id": "${providerModel}",`);
    if (apiKey) lines.push(`  "api_key": "********",`);
    if (oauthToken && selectedProvider === "anthropic") lines.push(`  "oauth": "********",`);
    if (baseUrl) lines.push(`  "base_url": "${baseUrl}",`);
    if (selectedProvider === "azure" && azureEndpoint) lines.push(`  "endpoint": "...azure.com",`);
    if (selectedProvider === "bedrock" && awsRegion) lines.push(`  "region": "${awsRegion}",`);
    if (selectedProvider === "vertex" && vertexProject) lines.push(`  "project": "${vertexProject}",`);
    const caps = [];
    if (supportsStreaming) caps.push("stream");
    if (supportsVision) caps.push("vision");
    if (supportsFunctions) caps.push("functions");
    if (caps.length > 0) lines.push(`  "caps": [${caps.map(c => `"${c}"`).join(", ")}],`);
    if (rpm) lines.push(`  "rpm": ${rpm},`);
    if (tpm) lines.push(`  "tpm": ${tpm},`);
    lines.push("}");
    return lines.join("\n");
  };

  // Step value preview for completed steps in sidebar
  const stepPreview = (s: number): string => {
    switch (s) {
      case 1: return providerInfo?.label || "";
      case 2: {
        if (selectedProvider === "anthropic" && anthropicAuthMode === "oauth_token") return oauthToken ? "OAuth ········" : "";
        return apiKey ? "········" : (selectedProvider === "azure" && azureEndpoint ? "Azure endpoint set" : "");
      }
      case 3: return modelName ? `${modelName}` : "";
      case 4: {
        const c = [];
        if (supportsStreaming) c.push("Stream");
        if (supportsVision) c.push("Vision");
        if (supportsFunctions) c.push("Fn");
        return c.join(", ");
      }
      case 5: return "";
      default: return "";
    }
  };

  // ─── Left Sidebar: Progress Rail ────────────────────────────────────
  const ProgressRail = () => (
    <div className="w-[280px] flex-shrink-0 hidden lg:block">
      <div className="sticky top-6">
        {/* Progress steps card */}
        <div className="bg-card backdrop-blur-xl rounded-2xl border border-border p-5 shadow-lg">
          {/* Header */}
          <div className="flex items-center gap-3 mb-6">
            <div className="w-8 h-8 rounded-lg bg-teal-500/10 flex items-center justify-center">
              <Icon icon={icons.models} className="h-4 w-4 text-teal-500" />
            </div>
            <div>
              <p className="text-sm font-semibold text-foreground">New Model</p>
              <p className="text-[11px] text-muted-foreground">Step {step} of 5</p>
            </div>
          </div>

          {/* Steps */}
          <div className="space-y-1">
            {STEPS.map((s) => {
              const isCompleted = step > s.id;
              const isCurrent = step === s.id;

              const canJump = s.id < step;
              const preview = stepPreview(s.id);

              return (
                <button
                  key={s.id}
                  type="button"
                  disabled={!canJump}
                  onClick={() => { if (canJump) setStep(s.id); }}
                  className={`w-full text-left rounded-xl p-3 transition-all duration-200 group relative ${
                    isCurrent
                      ? "bg-teal-500/10 border-l-2 border-l-teal-400 shadow-[inset_0_0_20px_rgba(20,184,166,0.05)]"
                      : canJump
                      ? "hover:bg-muted/60 cursor-pointer border-l-2 border-l-transparent"
                      : "border-l-2 border-l-transparent"
                  }`}
                >
                  <div className="flex items-start gap-3">
                    {/* Step number / check */}
                    <div className={`w-6 h-6 rounded-md flex items-center justify-center flex-shrink-0 text-xs font-bold transition-all ${
                      isCompleted
                        ? "bg-teal-500 text-white"
                        : isCurrent
                        ? "bg-teal-500/20 text-teal-500 ring-1 ring-teal-500/40"
                        : "bg-muted text-muted-foreground/60"
                    }`}>
                      {isCompleted ? (
                        <Icon icon={icons.check} className="h-3.5 w-3.5" />
                      ) : (
                        <span>{s.id}</span>
                      )}
                    </div>
                    {/* Label + description */}
                    <div className="flex-1 min-w-0">
                      <p className={`text-sm font-medium leading-tight ${
                        isCurrent ? "text-teal-600 dark:text-teal-500" : isCompleted ? "text-foreground/80" : "text-muted-foreground/60"
                      }`}>
                        {s.label}
                      </p>
                      {isCurrent && (
                        <p className="text-[11px] text-muted-foreground mt-0.5">{s.description}</p>
                      )}
                      {isCompleted && preview && (
                        <p className="text-[11px] text-muted-foreground mt-0.5 font-mono truncate">{preview}</p>
                      )}
                    </div>
                  </div>
                </button>
              );
            })}
          </div>

          {/* Config preview */}
          <div className="mt-6 pt-4 border-t border-border">
            <p className="text-[10px] uppercase tracking-widest text-muted-foreground/60 font-semibold mb-2">Config Preview</p>
            <pre className="text-[11px] font-mono text-muted-foreground leading-relaxed overflow-hidden max-h-48 whitespace-pre-wrap break-all">
              {configPreview()}
            </pre>
          </div>
        </div>
      </div>
    </div>
  );

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
              className={`group relative flex flex-col items-center gap-4 p-6 rounded-2xl border-2 transition-all duration-200 hover:scale-[1.02] hover:shadow-lg active:scale-[0.98] ${
                isSelected
                  ? "border-teal-500 bg-teal-500/5 dark:bg-teal-500/10 ring-2 ring-teal-500/30 shadow-lg shadow-teal-500/10"
                  : "border-border/50 hover:border-muted-foreground/30 hover:bg-muted/30"
              }`}
            >
              {isSelected && (
                <div className="absolute top-3 right-3 w-5 h-5 rounded-full bg-teal-500 flex items-center justify-center">
                  <Icon icon={icons.check} className="h-3 w-3 text-white" />
                </div>
              )}
              <div className={`p-3 rounded-xl ${isSelected ? "bg-teal-500/10" : "bg-muted/50 group-hover:bg-muted"} transition-colors`}>
                <Icon icon={p.icon} width="36" height="36" className={isSelected ? "" : "opacity-60 group-hover:opacity-80"} />
              </div>
              <span className={`text-sm font-semibold ${isSelected ? "text-foreground" : "text-muted-foreground group-hover:text-foreground"}`}>
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
    if (selectedProvider === "anthropic" && anthropicAuthMode === "oauth_token") {
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
                disabled={!providerValid || testLoading}
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
    <div className="min-h-screen pb-24 relative">
      {/* Header */}
      <div className="flex items-center gap-4 mb-8">
        <Button variant="ghost" size="icon" onClick={() => navigate("/models")} className="rounded-xl">
          <Icon icon={icons.arrowLeft} className="h-5 w-5" />
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Add Model</h1>
          <p className="text-sm text-muted-foreground">
            Configure a new model instance
          </p>
        </div>
      </div>

      {/* Two-panel layout */}
      <div className="flex gap-8 items-start">
        {/* Left: Progress Rail */}
        <ProgressRail />

        {/* Right: Step Content */}
        <div className="flex-1 min-w-0">
          {step === 1 && renderProviderStep()}
          {step === 2 && renderAuthStep()}
          {step === 3 && renderModelStep()}
          {step === 4 && renderCapabilitiesStep()}
          {step === 5 && renderReviewStep()}
        </div>
      </div>

      {/* Floating Bottom Bar */}
      <div className="fixed bottom-0 left-0 right-0 z-50">
        <div className="backdrop-blur-xl bg-background/80 border-t border-border/50 shadow-[0_-4px_32px_rgba(0,0,0,0.08)]">
          <div className="max-w-screen-xl mx-auto px-6 py-3 flex items-center justify-between">
            {/* Left: Back / Cancel */}
            <div className="w-[140px]">
              {step === 1 ? (
                <Button type="button" variant="ghost" onClick={() => navigate("/models")} className="text-muted-foreground">
                  Cancel
                </Button>
              ) : (
                <Button type="button" variant="ghost" onClick={handleBack} className="text-muted-foreground">
                  <Icon icon={icons.arrowLeft} className="h-4 w-4 mr-2" />
                  Back
                </Button>
              )}
            </div>

            {/* Center: Step dots */}
            <div className="flex items-center gap-2">
              {STEPS.map((s) => (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => { if (s.id < step) setStep(s.id); }}
                  className={`transition-all duration-200 rounded-full ${
                    s.id === step
                      ? "w-6 h-2 bg-teal-500"
                      : s.id < step
                      ? "w-2 h-2 bg-teal-500/60 cursor-pointer hover:bg-teal-500"
                      : "w-2 h-2 bg-muted-foreground/20"
                  }`}
                  aria-label={`Step ${s.id}: ${s.label}`}
                />
              ))}
            </div>

            {/* Right: Next / Create */}
            <div className="w-[140px] flex justify-end">
              {step < 5 ? (
                <Button
                  type="button"
                  disabled={!stepValid(step)}
                  onClick={handleNext}
                  className="bg-teal-600 hover:bg-teal-700 text-white rounded-xl px-6"
                >
                  Next
                  <Icon icon={icons.arrowRight} className="h-4 w-4 ml-2" />
                </Button>
              ) : (
                <Button
                  type="button"
                  disabled={mutation.isPending || !stepValid(5)}
                  onClick={handleSubmit}
                  className="bg-teal-600 hover:bg-teal-700 text-white rounded-xl px-6"
                >
                  {mutation.isPending ? (
                    "Creating..."
                  ) : (
                    <>
                      <Icon icon={icons.check} className="h-4 w-4 mr-2" />
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
