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
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
  { id: 1, label: "Provider", icon: "solar:star-shine-linear" },
  { id: 2, label: "Authentication", icon: icons.keys },
  { id: 3, label: "Model", icon: icons.models },
  { id: 4, label: "Capabilities", icon: icons.settings },
  { id: 5, label: "Review & Test", icon: icons.check },
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

  // ─── Step Indicator ─────────────────────────────────────────────────
  const StepIndicator = () => (
    <div className="flex items-center justify-between mb-8">
      {STEPS.map((s, i) => {
        const isCompleted = step > s.id;
        const isCurrent = step === s.id;
        const isClickable = s.id < step || (s.id === step);
        // Allow jumping back to completed steps
        const canJump = s.id < step;

        return (
          <div key={s.id} className="flex items-center flex-1 last:flex-none">
            {/* Step circle + label */}
            <button
              type="button"
              disabled={!isClickable && !canJump}
              onClick={() => { if (canJump) setStep(s.id); }}
              className={`flex flex-col items-center gap-1.5 transition-all ${canJump ? "cursor-pointer" : "cursor-default"}`}
            >
              <div
                className={`relative flex items-center justify-center w-10 h-10 rounded-full border-2 transition-all ${
                  isCurrent
                    ? "border-teal-500 bg-teal-500/10 text-teal-500 ring-4 ring-teal-500/20"
                    : isCompleted
                    ? "border-teal-500 bg-teal-500 text-white"
                    : "border-muted-foreground/30 text-muted-foreground/50"
                }`}
              >
                {isCompleted ? (
                  <Icon icon={icons.check} className="h-5 w-5" />
                ) : (
                  <Icon icon={s.icon} className="h-4 w-4" />
                )}
              </div>
              <span
                className={`text-xs font-medium hidden sm:block ${
                  isCurrent
                    ? "text-teal-500"
                    : isCompleted
                    ? "text-foreground"
                    : "text-muted-foreground/50"
                }`}
              >
                {s.label}
              </span>
            </button>
            {/* Connector line */}
            {i < STEPS.length - 1 && (
              <div className="flex-1 mx-2 sm:mx-3">
                <div
                  className={`h-0.5 rounded-full transition-colors ${
                    step > s.id ? "bg-teal-500" : "bg-muted-foreground/20"
                  }`}
                />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );

  // ─── Step 1: Provider ───────────────────────────────────────────────
  const renderProviderStep = () => (
    <Card className="border-muted-foreground/10">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <Icon icon="solar:star-shine-linear" className="h-5 w-5 text-teal-500" />
          Choose Provider
        </CardTitle>
        <CardDescription>
          Select the LLM provider for this model instance
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
          {PROVIDERS.map((p) => {
            const isSelected = selectedProvider === p.value;
            return (
              <button
                key={p.value}
                type="button"
                onClick={() => setSelectedProvider(p.value)}
                className={`relative flex flex-col items-center gap-3 p-5 rounded-xl border-2 transition-all hover:shadow-md ${
                  isSelected
                    ? `${p.borderColor} ${p.bgColor} ring-2 ring-teal-500/50`
                    : "border-border hover:border-muted-foreground/30"
                }`}
              >
                {isSelected && (
                  <div className="absolute top-2 right-2">
                    <Icon icon={icons.check} className="h-4 w-4 text-teal-500" />
                  </div>
                )}
                <div className={`p-2.5 rounded-lg ${p.bgColor}`}>
                  <Icon icon={p.icon} width="32" height="32" className={p.color} />
                </div>
                <span className="text-sm font-medium">{p.label}</span>
              </button>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );

  // ─── Step 2: Authentication ─────────────────────────────────────────
  const renderAuthStep = () => {
    if (!providerInfo) return null;
    return (
      <Card className="border-muted-foreground/10">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Icon icon={icons.keys} className="h-5 w-5 text-teal-500" />
            Authentication
          </CardTitle>
          <CardDescription>
            API credentials for{" "}
            <span className={providerInfo.color}>{providerInfo.label}</span>
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Anthropic auth mode toggle */}
          {selectedProvider === "anthropic" && (
            <div className="flex gap-2 p-1 bg-muted rounded-lg w-fit">
              <button
                type="button"
                onClick={() => setAnthropicAuthMode("api_key")}
                className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                  anthropicAuthMode === "api_key"
                    ? "bg-background shadow-sm font-medium"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                API Key
              </button>
              <button
                type="button"
                onClick={() => setAnthropicAuthMode("oauth_token")}
                className={`px-3 py-1.5 text-sm rounded-md transition-colors ${
                  anthropicAuthMode === "oauth_token"
                    ? "bg-background shadow-sm font-medium"
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
                className="max-w-md font-mono"
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
                className="max-w-md font-mono"
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
                  className="max-w-md"
                />
              </div>
            )}

          {/* Azure-specific */}
          {selectedProvider === "azure" && (
            <div className="space-y-4 p-4 rounded-lg border bg-muted/30">
              <div className="flex items-center gap-2">
                <Icon icon="logos:microsoft-azure" width="16" height="16" />
                <span className="text-sm font-medium">Azure Configuration</span>
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
            <div className="space-y-4 p-4 rounded-lg border bg-muted/30">
              <div className="flex items-center gap-2">
                <Icon icon="logos:aws" width="20" height="14" />
                <span className="text-sm font-medium">AWS Configuration</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="space-y-2">
                  <Label className="text-sm">Region</Label>
                  <Input
                    placeholder="us-east-1"
                    value={awsRegion}
                    onChange={(e) => setAwsRegion(e.target.value)}
                  />
                </div>
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
          )}

          {/* Vertex-specific */}
          {selectedProvider === "vertex" && (
            <div className="space-y-4 p-4 rounded-lg border bg-muted/30">
              <div className="flex items-center gap-2">
                <Icon icon="logos:google-cloud" width="20" height="16" />
                <span className="text-sm font-medium">Vertex AI Configuration</span>
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
        </CardContent>
      </Card>
    );
  };

  // ─── Step 3: Model ──────────────────────────────────────────────────
  const renderModelStep = () => {
    if (!providerInfo) return null;
    return (
      <Card className="border-muted-foreground/10">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Icon icon={icons.models} className="h-5 w-5 text-teal-500" />
            Model Configuration
          </CardTitle>
          <CardDescription>
            Configure the model instance for{" "}
            <span className={providerInfo.color}>{providerInfo.label}</span>
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
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
              className="max-w-md"
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
              className="max-w-md"
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
                      providerModel === m ? "border-teal-500 bg-teal-500/10 text-teal-600 dark:text-teal-400" : ""
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
                  <span className="text-sm text-teal-600 dark:text-teal-400">
                    {discoveredModels.length} models fetched — click a badge above to select
                  </span>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    );
  };

  // ─── Step 4: Capabilities & Config ──────────────────────────────────
  const renderCapabilitiesStep = () => (
    <div className="space-y-6">
      {/* Capabilities */}
      <Card className="border-muted-foreground/10">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Icon icon="solar:star-shine-linear" className="h-5 w-5 text-teal-500" />
            Capabilities
          </CardTitle>
          <CardDescription>Declare what this model supports</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="flex items-center justify-between p-3 rounded-lg border">
              <div className="flex items-center gap-2">
                <Icon icon="solar:star-shine-linear" className="h-4 w-4 text-blue-500" />
                <Label className="text-sm cursor-pointer">Streaming</Label>
              </div>
              <Switch checked={supportsStreaming} onCheckedChange={setSupportsStreaming} />
            </div>
            <div className="flex items-center justify-between p-3 rounded-lg border">
              <div className="flex items-center gap-2">
                <Icon icon={icons.eye} className="h-4 w-4 text-purple-500" />
                <Label className="text-sm cursor-pointer">Vision</Label>
              </div>
              <Switch checked={supportsVision} onCheckedChange={setSupportsVision} />
            </div>
            <div className="flex items-center justify-between p-3 rounded-lg border">
              <div className="flex items-center gap-2">
                <Icon icon="solar:settings-minimalistic-linear" className="h-4 w-4 text-green-500" />
                <Label className="text-sm cursor-pointer">Function Calling</Label>
              </div>
              <Switch checked={supportsFunctions} onCheckedChange={setSupportsFunctions} />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Rate Limits */}
      <Card className="border-muted-foreground/10">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Icon icon="solar:speed-linear" className="h-5 w-5 text-teal-500" />
            Rate Limits
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
        </CardContent>
      </Card>

      {/* Load Balancing */}
      <Card className="border-muted-foreground/10">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Icon icon={icons.settings} className="h-5 w-5 text-teal-500" />
            Load Balancing
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
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
        </CardContent>
      </Card>

      {/* Pricing */}
      <Card className="border-muted-foreground/10">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Icon icon="solar:dollar-minimalistic-linear" className="h-5 w-5 text-teal-500" />
            Pricing
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="text-sm">Input Cost / Token</Label>
              <Input type="number" step="0.000001" placeholder="0.000005" value={inputCost} onChange={(e) => setInputCost(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label className="text-sm">Output Cost / Token</Label>
              <Input type="number" step="0.000001" placeholder="0.000015" value={outputCost} onChange={(e) => setOutputCost(e.target.value)} />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Tags + Reasoning Effort */}
      <Card className="border-muted-foreground/10">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Icon icon="solar:tag-linear" className="h-5 w-5 text-teal-500" />
            Tags & Reasoning
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label className="text-sm">Tags</Label>
            <Input
              placeholder="production, fast, custom"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              className="max-w-md"
            />
            <p className="text-xs text-muted-foreground">Comma-separated labels for filtering and grouping</p>
          </div>
          <div className="space-y-2">
            <Label className="text-sm">Default Reasoning Effort</Label>
            <Select value={defaultReasoningEffort} onValueChange={setDefaultReasoningEffort}>
              <SelectTrigger className="max-w-md">
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
        </CardContent>
      </Card>
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
      <div className="space-y-6">
        <Card className="border-muted-foreground/10">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Icon icon={icons.check} className="h-5 w-5 text-teal-500" />
              Review Configuration
            </CardTitle>
            <CardDescription>
              Confirm all settings before creating the model
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-0 divide-y divide-border rounded-lg border overflow-hidden">
              {summaryRows.map((row, i) => (
                <div key={i} className="flex items-center justify-between px-4 py-3 hover:bg-muted/30">
                  <span className="text-sm text-muted-foreground">{row.label}</span>
                  <span className={`text-sm font-medium ${row.mono ? "font-mono" : ""}`}>
                    {row.value || <span className="text-muted-foreground/50">--</span>}
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Test Connection */}
        <Card className="border-muted-foreground/10">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Icon icon="solar:wifi-linear" className="h-5 w-5 text-teal-500" />
              Test Connection
            </CardTitle>
            <CardDescription>
              Verify the provider is reachable before creating
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
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
          </CardContent>
        </Card>
      </div>
    );
  };

  // ─── Render ─────────────────────────────────────────────────────────
  return (
    <div className="max-w-4xl mx-auto">
      {/* Header */}
      <div className="flex items-center gap-4 mb-6">
        <Button variant="ghost" size="icon" onClick={() => navigate("/models")}>
          <Icon icon={icons.arrowLeft} className="h-5 w-5" />
        </Button>
        <div>
          <h1 className="text-2xl font-bold">Add Model</h1>
          <p className="text-muted-foreground">
            Configure a new model instance. It will be persisted and survive restarts.
          </p>
        </div>
      </div>

      {/* Step Indicator */}
      <StepIndicator />

      {/* Step Content */}
      <div className="mb-8">
        {step === 1 && renderProviderStep()}
        {step === 2 && renderAuthStep()}
        {step === 3 && renderModelStep()}
        {step === 4 && renderCapabilitiesStep()}
        {step === 5 && renderReviewStep()}
      </div>

      {/* Navigation */}
      <div className="flex items-center justify-between py-4 border-t">
        <div>
          {step === 1 ? (
            <Button type="button" variant="ghost" onClick={() => navigate("/models")}>
              Cancel
            </Button>
          ) : (
            <Button type="button" variant="ghost" onClick={handleBack}>
              <Icon icon={icons.arrowLeft} className="h-4 w-4 mr-2" />
              Back
            </Button>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground mr-2">
            Step {step} of 5
          </span>
          {step < 5 ? (
            <Button
              type="button"
              disabled={!stepValid(step)}
              onClick={handleNext}
              className="min-w-[120px] bg-teal-600 hover:bg-teal-700 text-white"
            >
              Next
              <Icon icon={icons.arrowRight} className="h-4 w-4 ml-2" />
            </Button>
          ) : (
            <Button
              type="button"
              disabled={mutation.isPending || !stepValid(5)}
              onClick={handleSubmit}
              className="min-w-[140px] bg-teal-600 hover:bg-teal-700 text-white"
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
  );
}
