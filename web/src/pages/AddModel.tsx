import { useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import {
  ArrowLeft,
  Check,
  CheckCircle2,
  ChevronRight,
  Info,
  Key,
  Loader2,
  Settings2,
  Sparkles,
  DollarSign,
  Gauge,
  Tag,
  Wifi,
  XCircle,
} from "lucide-react";

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
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/hooks/use-toast";
import { createModel, testModelConnection } from "@/lib/api";
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
    models: ["claude-sonnet-4-20250514", "claude-opus-4-20250514", "claude-3-5-haiku-20241022"],
    keyPlaceholder: "sk-ant-... or ${ANTHROPIC_API_KEY}",
    keyHint: "Found in console.anthropic.com/settings/keys",
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
    icon: "lucide:globe",
    color: "text-purple-600 dark:text-purple-400",
    bgColor: "bg-purple-50 dark:bg-purple-950/30",
    borderColor: "border-purple-200 dark:border-purple-800",
    ringColor: "ring-purple-500",
    models: ["openai/gpt-4o", "anthropic/claude-sonnet-4-20250514", "google/gemini-2.0-flash-exp"],
    keyPlaceholder: "sk-or-... or ${OPENROUTER_API_KEY}",
    keyHint: "Found in openrouter.ai/keys",
  },
];

export default function AddModel() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  // Step tracking
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null);

  // Form state
  const [modelName, setModelName] = useState("");
  const [providerModel, setProviderModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");

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

  // Advanced
  const [advancedOpen, setAdvancedOpen] = useState(false);
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
    if (apiKey) provider.api_key = apiKey;
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
  }, [selectedProvider, providerModel, apiKey, baseUrl, azureDeployment, azureEndpoint, apiVersion, awsRegion, awsAccessKeyId, awsSecretKey, vertexProject, vertexLocation]);

  // Clear test result when provider config changes
  useEffect(() => {
    setTestResult(null);
  }, [selectedProvider, apiKey, baseUrl, azureDeployment, azureEndpoint, apiVersion, awsAccessKeyId, awsSecretKey, awsRegion, vertexProject, vertexLocation]);

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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
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
  const canSubmit = selectedProvider && modelName && providerModel && providerValid;

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate("/models")}>
          <ArrowLeft className="h-5 w-5" />
        </Button>
        <div>
          <h1 className="text-2xl font-bold">Add Model</h1>
          <p className="text-muted-foreground">
            Configure a new model instance. It will be persisted and survive restarts.
          </p>
        </div>
      </div>

      <form onSubmit={handleSubmit}>
        {/* Step 1: Provider Selection */}
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Sparkles className="h-5 w-5" />
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
                    className={`relative flex flex-col items-center gap-3 p-4 rounded-xl border-2 transition-all hover:shadow-md ${
                      isSelected
                        ? `${p.borderColor} ${p.bgColor} ring-2 ${p.ringColor}`
                        : "border-border hover:border-muted-foreground/30"
                    }`}
                  >
                    {isSelected && (
                      <div className="absolute top-2 right-2">
                        <Check className="h-4 w-4 text-primary" />
                      </div>
                    )}
                    <div className={`p-2 rounded-lg ${p.bgColor}`}>
                      <Icon icon={p.icon} width="28" height="28" className={p.color} />
                    </div>
                    <span className="text-sm font-medium">{p.label}</span>
                  </button>
                );
              })}
            </div>
          </CardContent>
        </Card>

        {/* Step 2: Model Configuration (visible after provider selected) */}
        {selectedProvider && providerInfo && (
          <>
            <Card className="mb-6">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Settings2 className="h-5 w-5" />
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
                    <code className="text-xs bg-muted px-1 py-0.5 rounded">
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
                    placeholder={
                      providerInfo.models[0] || "Model identifier"
                    }
                    value={providerModel}
                    onChange={(e) => setProviderModel(e.target.value)}
                    className="max-w-md"
                    required
                  />
                  {providerInfo.models.length > 0 && (
                    <div className="flex flex-wrap gap-1.5 mt-2">
                      {providerInfo.models.map((m) => (
                        <Badge
                          key={m}
                          variant="outline"
                          className="cursor-pointer hover:bg-accent transition-colors text-xs"
                          onClick={() => setProviderModel(m)}
                        >
                          {m}
                        </Badge>
                      ))}
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>

            {/* Step 3: Authentication */}
            <Card className="mb-6">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Key className="h-5 w-5" />
                  Authentication
                </CardTitle>
                <CardDescription>
                  API credentials for {providerInfo.label}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-2">
                  <Label htmlFor="apiKey" className="text-sm font-medium">
                    API Key
                    {["anthropic", "openrouter", "vertex"].includes(selectedProvider) && (
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
                    required={["anthropic", "openrouter", "vertex"].includes(selectedProvider)}
                  />
                  <div className="flex items-start gap-2 text-xs text-muted-foreground">
                    <Info className="h-3 w-3 mt-0.5 flex-shrink-0" />
                    <div>
                      <p>{providerInfo.keyHint}</p>
                      <p className="mt-1">
                        Supports environment variable references:{" "}
                        <code className="bg-muted px-1 py-0.5 rounded">
                          {"${OPENAI_API_KEY}"}
                        </code>
                      </p>
                    </div>
                  </div>
                </div>

                {selectedProvider !== "bedrock" &&
                  selectedProvider !== "vertex" &&
                  selectedProvider !== "azure" && (
                    <div className="space-y-2">
                      <Label htmlFor="baseUrl" className="text-sm font-medium">
                        Base URL{" "}
                        <span className="text-muted-foreground font-normal">
                          (optional)
                        </span>
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
                          Found in Azure Portal → your OpenAI resource → Keys and Endpoint
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
                      <span className="text-sm font-medium">
                        Vertex AI Configuration
                      </span>
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

                {/* Test Connection */}
                <Separator />
                <div className="flex items-center gap-3 flex-wrap">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={!providerValid || testLoading}
                    onClick={handleTestConnection}
                  >
                    {testLoading ? (
                      <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    ) : (
                      <Wifi className="h-4 w-4 mr-2" />
                    )}
                    Test Connection
                  </Button>
                  {testResult && (
                    <div className={`flex items-center gap-2 text-sm ${testResult.success ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400"}`}>
                      {testResult.success ? (
                        <CheckCircle2 className="h-4 w-4 flex-shrink-0" />
                      ) : (
                        <XCircle className="h-4 w-4 flex-shrink-0" />
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

            {/* Step 4: Capabilities */}
            <Card className="mb-6">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-lg">
                  <Sparkles className="h-5 w-5" />
                  Capabilities
                </CardTitle>
                <CardDescription>
                  Declare what this model supports
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="flex items-center justify-between p-3 rounded-lg border">
                    <div className="flex items-center gap-2">
                      <Sparkles className="h-4 w-4 text-blue-500" />
                      <Label className="text-sm cursor-pointer">Streaming</Label>
                    </div>
                    <Switch
                      checked={supportsStreaming}
                      onCheckedChange={setSupportsStreaming}
                    />
                  </div>
                  <div className="flex items-center justify-between p-3 rounded-lg border">
                    <div className="flex items-center gap-2">
                      <Icon icon="lucide:eye" width="16" height="16" className="text-purple-500" />
                      <Label className="text-sm cursor-pointer">Vision</Label>
                    </div>
                    <Switch
                      checked={supportsVision}
                      onCheckedChange={setSupportsVision}
                    />
                  </div>
                  <div className="flex items-center justify-between p-3 rounded-lg border">
                    <div className="flex items-center gap-2">
                      <Icon icon="lucide:wrench" width="16" height="16" className="text-green-500" />
                      <Label className="text-sm cursor-pointer">
                        Function Calling
                      </Label>
                    </div>
                    <Switch
                      checked={supportsFunctions}
                      onCheckedChange={setSupportsFunctions}
                    />
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Step 5: Advanced Settings (collapsible) */}
            <Card className="mb-6">
              <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
                <CollapsibleTrigger asChild>
                  <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors rounded-t-lg">
                    <div className="flex items-center justify-between">
                      <div>
                        <CardTitle className="flex items-center gap-2 text-lg">
                          <Settings2 className="h-5 w-5" />
                          Advanced Settings
                        </CardTitle>
                        <CardDescription>
                          Rate limits, priority, pricing, and tags
                        </CardDescription>
                      </div>
                      <ChevronRight
                        className={`h-5 w-5 text-muted-foreground transition-transform ${
                          advancedOpen ? "rotate-90" : ""
                        }`}
                      />
                    </div>
                  </CardHeader>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <CardContent className="space-y-6 pt-0">
                    <Separator />

                    {/* Rate Limits */}
                    <div>
                      <h4 className="text-sm font-medium flex items-center gap-2 mb-3">
                        <Gauge className="h-4 w-4" />
                        Rate Limits
                      </h4>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="space-y-2">
                          <Label className="text-sm">
                            RPM{" "}
                            <span className="text-muted-foreground font-normal">
                              (Requests/min)
                            </span>
                          </Label>
                          <Input
                            type="number"
                            placeholder="100"
                            value={rpm}
                            onChange={(e) => setRpm(e.target.value)}
                          />
                        </div>
                        <div className="space-y-2">
                          <Label className="text-sm">
                            TPM{" "}
                            <span className="text-muted-foreground font-normal">
                              (Tokens/min)
                            </span>
                          </Label>
                          <Input
                            type="number"
                            placeholder="100,000"
                            value={tpm}
                            onChange={(e) => setTpm(e.target.value)}
                          />
                        </div>
                      </div>
                    </div>

                    {/* Load Balancing */}
                    <div>
                      <h4 className="text-sm font-medium flex items-center gap-2 mb-3">
                        <Settings2 className="h-4 w-4" />
                        Load Balancing
                      </h4>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div className="space-y-2">
                          <Label className="text-sm">Priority (1-100)</Label>
                          <Input
                            type="number"
                            placeholder="50"
                            min="1"
                            max="100"
                            value={priority}
                            onChange={(e) => setPriority(e.target.value)}
                          />
                          <p className="text-xs text-muted-foreground">
                            Higher = preferred
                          </p>
                        </div>
                        <div className="space-y-2">
                          <Label className="text-sm">Weight</Label>
                          <Input
                            type="number"
                            step="0.1"
                            placeholder="1.0"
                            value={weight}
                            onChange={(e) => setWeight(e.target.value)}
                          />
                          <p className="text-xs text-muted-foreground">
                            For weighted round-robin
                          </p>
                        </div>
                        <div className="space-y-2">
                          <Label className="text-sm">Timeout (s)</Label>
                          <Input
                            type="number"
                            placeholder="60"
                            value={timeoutSeconds}
                            onChange={(e) => setTimeoutSeconds(e.target.value)}
                          />
                          <p className="text-xs text-muted-foreground">
                            Request timeout
                          </p>
                        </div>
                      </div>
                    </div>

                    {/* Pricing */}
                    <div>
                      <h4 className="text-sm font-medium flex items-center gap-2 mb-3">
                        <DollarSign className="h-4 w-4" />
                        Pricing
                      </h4>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="space-y-2">
                          <Label className="text-sm">Input Cost / Token</Label>
                          <Input
                            type="number"
                            step="0.000001"
                            placeholder="0.000005"
                            value={inputCost}
                            onChange={(e) => setInputCost(e.target.value)}
                          />
                        </div>
                        <div className="space-y-2">
                          <Label className="text-sm">Output Cost / Token</Label>
                          <Input
                            type="number"
                            step="0.000001"
                            placeholder="0.000015"
                            value={outputCost}
                            onChange={(e) => setOutputCost(e.target.value)}
                          />
                        </div>
                      </div>
                    </div>

                    {/* Tags */}
                    <div>
                      <h4 className="text-sm font-medium flex items-center gap-2 mb-3">
                        <Tag className="h-4 w-4" />
                        Tags
                      </h4>
                      <Input
                        placeholder="production, fast, custom"
                        value={tags}
                        onChange={(e) => setTags(e.target.value)}
                        className="max-w-md"
                      />
                      <p className="text-xs text-muted-foreground mt-1">
                        Comma-separated labels for filtering and grouping
                      </p>
                    </div>

                    {/* Default Reasoning Effort */}
                    <div>
                      <h4 className="text-sm font-medium flex items-center gap-2 mb-3">
                        <Sparkles className="h-4 w-4" />
                        Default Reasoning Effort
                      </h4>
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
                      <p className="text-xs text-muted-foreground mt-1">
                        Default reasoning effort for reasoning models (o1, o3, GPT-5, etc.). Applied when callers don't specify one.
                      </p>
                    </div>
                  </CardContent>
                </CollapsibleContent>
              </Collapsible>
            </Card>

            {/* Actions */}
            <div className="flex items-center justify-between py-4 border-t">
              <Button
                type="button"
                variant="ghost"
                onClick={() => navigate("/models")}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={mutation.isPending || !canSubmit}
                className="min-w-[140px]"
              >
                {mutation.isPending ? (
                  "Creating..."
                ) : (
                  <>
                    <Check className="h-4 w-4 mr-2" />
                    Create Model
                  </>
                )}
              </Button>
            </div>
          </>
        )}
      </form>
    </div>
  );
}
