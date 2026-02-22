import { useState, useEffect, useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
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
import { getAdminModels, updateModel, testModelConnection } from "@/lib/api";
import type { UpdateModelRequest, ProviderConfig, AdminModel, AdminModelsResponse } from "@/types/api";

const PROVIDER_OPTIONS = [
  { value: "openai", label: "OpenAI", icon: "logos:openai-icon", color: "text-emerald-600 dark:text-emerald-400", bgColor: "bg-emerald-50 dark:bg-emerald-950/30", borderColor: "border-emerald-200 dark:border-emerald-800" },
  { value: "anthropic", label: "Anthropic", icon: "logos:anthropic", color: "text-orange-600 dark:text-orange-400", bgColor: "bg-orange-50 dark:bg-orange-950/30", borderColor: "border-orange-200 dark:border-orange-800" },
  { value: "azure", label: "Azure OpenAI", icon: "logos:microsoft-azure", color: "text-blue-700 dark:text-blue-300", bgColor: "bg-blue-50 dark:bg-blue-950/30", borderColor: "border-blue-200 dark:border-blue-800" },
  { value: "bedrock", label: "AWS Bedrock", icon: "logos:aws", color: "text-yellow-600 dark:text-yellow-400", bgColor: "bg-yellow-50 dark:bg-yellow-950/30", borderColor: "border-yellow-200 dark:border-yellow-800" },
  { value: "vertex", label: "Google Vertex AI", icon: "logos:google-cloud", color: "text-red-600 dark:text-red-400", bgColor: "bg-red-50 dark:bg-red-950/30", borderColor: "border-red-200 dark:border-red-800" },
  { value: "openrouter", label: "OpenRouter", icon: "lucide:globe", color: "text-purple-600 dark:text-purple-400", bgColor: "bg-purple-50 dark:bg-purple-950/30", borderColor: "border-purple-200 dark:border-purple-800" },
];

export default function EditModel() {
  const { modelId } = useParams<{ modelId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [loaded, setLoaded] = useState(false);

  // Form state
  const [selectedProvider, setSelectedProvider] = useState("");
  const [modelName, setModelName] = useState("");
  const [providerModel, setProviderModel] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [azureDeployment, setAzureDeployment] = useState("");
  const [azureEndpoint, setAzureEndpoint] = useState("");
  const [apiVersion, setApiVersion] = useState("");
  const [awsRegion, setAwsRegion] = useState("");
  const [awsAccessKeyId, setAwsAccessKeyId] = useState("");
  const [awsSecretKey, setAwsSecretKey] = useState("");
  const [vertexProject, setVertexProject] = useState("");
  const [vertexLocation, setVertexLocation] = useState("");
  const [supportsStreaming, setSupportsStreaming] = useState(true);
  const [supportsVision, setSupportsVision] = useState(false);
  const [supportsFunctions, setSupportsFunctions] = useState(true);
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

  // Fetch admin models to find the one we're editing
  const { data: adminModelsData, isLoading } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  });

  const adminModel: AdminModel | undefined = (() => {
    const data = adminModelsData as AdminModelsResponse | undefined;
    return data?.models?.find((m) => m.id === modelId || m.model_name === modelId);
  })();

  // Populate form from fetched model
  useEffect(() => {
    if (adminModel && !loaded) {
      setSelectedProvider(adminModel.provider?.type || "");
      setModelName(adminModel.model_name || "");
      setProviderModel(adminModel.provider?.model || "");
      setBaseUrl(adminModel.provider?.base_url || "");
      setAzureDeployment(adminModel.provider?.azure_deployment || "");
      setAzureEndpoint(adminModel.provider?.azure_endpoint || "");
      setApiVersion(adminModel.provider?.api_version || "");
      setAwsRegion(adminModel.provider?.aws_region_name || "");
      setVertexProject(adminModel.provider?.vertex_project || "");
      setVertexLocation(adminModel.provider?.vertex_location || "");
      setRpm(adminModel.rpm ? String(adminModel.rpm) : "");
      setTpm(adminModel.tpm ? String(adminModel.tpm) : "");
      setPriority(adminModel.priority ? String(adminModel.priority) : "");
      setWeight(adminModel.weight ? String(adminModel.weight) : "");
      setInputCost(adminModel.input_cost_per_token ? String(adminModel.input_cost_per_token) : "");
      setOutputCost(adminModel.output_cost_per_token ? String(adminModel.output_cost_per_token) : "");
      setTimeoutSeconds(adminModel.timeout_seconds ? String(adminModel.timeout_seconds) : "");
      setTags(adminModel.tags?.join(", ") || "");
      setDefaultReasoningEffort(adminModel.provider?.reasoning_effort || "");
      setSupportsStreaming(adminModel.model_info?.supports_streaming ?? true);
      setSupportsVision(adminModel.model_info?.supports_vision ?? false);
      setSupportsFunctions(adminModel.model_info?.supports_functions ?? true);
      setLoaded(true);
    }
  }, [adminModel, loaded]);

  // Test connection state
  const [testResult, setTestResult] = useState<{
    success: boolean;
    message: string;
    latency?: string;
  } | null>(null);
  const [testLoading, setTestLoading] = useState(false);

  // Build provider config from current form state
  const buildProviderConfig = useCallback((): ProviderConfig => {
    const provider: ProviderConfig = { type: selectedProvider, model: providerModel || "test" };
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

  // Determine if the provider requires a key that hasn't been entered
  const needsKeyForTest = (() => {
    if (!selectedProvider) return false;
    switch (selectedProvider) {
      case "anthropic":
      case "openrouter":
      case "vertex":
      case "openai":
        return !apiKey;
      case "bedrock":
        return !awsAccessKeyId || !awsSecretKey;
      default:
        return false;
    }
  })();

  const handleTestConnection = async () => {
    if (!selectedProvider) return;
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
    mutationFn: (data: UpdateModelRequest) => updateModel(modelId!, data),
    onSuccess: () => {
      toast({ title: "Model updated", description: `Model "${modelName}" has been updated.` });
      queryClient.invalidateQueries({ queryKey: ["models"] });
      queryClient.invalidateQueries({ queryKey: ["admin-models"] });
      navigate("/models");
    },
    onError: (error: any) => {
      const message = error.response?.data?.error || error.message || "Failed to update model";
      toast({ title: "Error", description: message, variant: "destructive" });
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!modelId || !selectedProvider || !modelName || !providerModel) return;

    const provider = buildProviderConfig();
    provider.model = providerModel;
    if (defaultReasoningEffort) provider.reasoning_effort = defaultReasoningEffort;

    const request: UpdateModelRequest = {
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
    if (tags) request.tags = tags.split(",").map((t) => t.trim()).filter(Boolean);

    mutation.mutate(request);
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin" />
        <span className="ml-2">Loading model...</span>
      </div>
    );
  }

  if (!adminModel) {
    return (
      <div className="text-center py-8">
        <p className="text-muted-foreground">Model not found</p>
        <Button variant="outline" className="mt-4" onClick={() => navigate("/models")}>
          Back to Models
        </Button>
      </div>
    );
  }

  if (adminModel.source !== "user") {
    return (
      <div className="text-center py-8">
        <p className="text-muted-foreground">System models cannot be edited. They are managed via config.yaml.</p>
        <Button variant="outline" className="mt-4" onClick={() => navigate("/models")}>
          Back to Models
        </Button>
      </div>
    );
  }

  // For edit, provider-specific fields are only required if they're being changed
  // (existing credentials are preserved server-side if fields are left empty)
  const providerValid = (() => {
    if (!selectedProvider) return false;
    switch (selectedProvider) {
      case "azure":
        return !!azureEndpoint;
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
          <h1 className="text-2xl font-bold">Edit Model</h1>
          <p className="text-muted-foreground">
            Update configuration for <span className="font-medium">{adminModel.model_name}</span>
          </p>
        </div>
      </div>

      <form onSubmit={handleSubmit}>
        {/* Provider display */}
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Sparkles className="h-5 w-5" />
              Provider
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
              {PROVIDER_OPTIONS.map((p) => {
                const isSelected = selectedProvider === p.value;
                return (
                  <button
                    key={p.value}
                    type="button"
                    onClick={() => setSelectedProvider(p.value)}
                    className={`relative flex flex-col items-center gap-3 p-4 rounded-xl border-2 transition-all hover:shadow-md ${
                      isSelected
                        ? `${p.borderColor} ${p.bgColor} ring-2 ring-primary/50`
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

        {/* Model Configuration */}
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Settings2 className="h-5 w-5" />
              Model Configuration
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-2">
              <Label className="text-sm font-medium">Model Name <span className="text-destructive">*</span></Label>
              <Input value={modelName} onChange={(e) => setModelName(e.target.value)} className="max-w-md" required />
            </div>
            <div className="space-y-2">
              <Label className="text-sm font-medium">Provider Model ID <span className="text-destructive">*</span></Label>
              <Input value={providerModel} onChange={(e) => setProviderModel(e.target.value)} className="max-w-md" required />
            </div>
          </CardContent>
        </Card>

        {/* Authentication */}
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Key className="h-5 w-5" />
              Authentication
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-2">
              <Label className="text-sm font-medium">API Key</Label>
              <Input
                type="password"
                placeholder="Leave empty to keep current key"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="max-w-md font-mono"
              />
              <div className="flex items-start gap-2 text-xs text-muted-foreground">
                <Info className="h-3 w-3 mt-0.5 flex-shrink-0" />
                <p>A key is currently set. Leave empty to keep it unchanged.</p>
              </div>
            </div>
            {selectedProvider !== "bedrock" && selectedProvider !== "vertex" && selectedProvider !== "azure" && (
              <div className="space-y-2">
                <Label className="text-sm font-medium">Base URL <span className="text-muted-foreground font-normal">(optional)</span></Label>
                <Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} className="max-w-md" />
              </div>
            )}

            {selectedProvider === "azure" && (
              <div className="space-y-4 p-4 rounded-lg border bg-muted/30">
                <div className="flex items-center gap-2">
                  <Icon icon="logos:microsoft-azure" width="16" height="16" />
                  <span className="text-sm font-medium">Azure Configuration</span>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label className="text-sm">Endpoint URL <span className="text-destructive">*</span></Label>
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
                    <Input value={azureDeployment} onChange={(e) => setAzureDeployment(e.target.value)} />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-sm">API Version</Label>
                    <Input placeholder="2024-06-01" value={apiVersion} onChange={(e) => setApiVersion(e.target.value)} />
                  </div>
                </div>
              </div>
            )}
            {selectedProvider === "bedrock" && (
              <div className="space-y-4 p-4 rounded-lg border bg-muted/30">
                <div className="flex items-center gap-2">
                  <Icon icon="logos:aws" width="20" height="14" />
                  <span className="text-sm font-medium">AWS Configuration</span>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="space-y-2">
                    <Label className="text-sm">Region</Label>
                    <Input value={awsRegion} onChange={(e) => setAwsRegion(e.target.value)} />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-sm">Access Key ID <span className="text-destructive">*</span></Label>
                    <Input type="password" placeholder="Leave empty to keep current" value={awsAccessKeyId} onChange={(e) => setAwsAccessKeyId(e.target.value)} className="font-mono" />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-sm">Secret Access Key <span className="text-destructive">*</span></Label>
                    <Input type="password" placeholder="Leave empty to keep current" value={awsSecretKey} onChange={(e) => setAwsSecretKey(e.target.value)} className="font-mono" />
                  </div>
                </div>
              </div>
            )}
            {selectedProvider === "vertex" && (
              <div className="space-y-4 p-4 rounded-lg border bg-muted/30">
                <div className="flex items-center gap-2">
                  <Icon icon="logos:google-cloud" width="20" height="16" />
                  <span className="text-sm font-medium">Vertex AI Configuration</span>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label className="text-sm">Project ID</Label>
                    <Input value={vertexProject} onChange={(e) => setVertexProject(e.target.value)} />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-sm">Location</Label>
                    <Input value={vertexLocation} onChange={(e) => setVertexLocation(e.target.value)} />
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
                disabled={needsKeyForTest || testLoading}
                onClick={handleTestConnection}
              >
                {testLoading ? (
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                ) : (
                  <Wifi className="h-4 w-4 mr-2" />
                )}
                Test Connection
              </Button>
              {needsKeyForTest && (
                <span className="text-xs text-muted-foreground">
                  Enter the API key to test the connection (existing keys are masked for security)
                </span>
              )}
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

        {/* Capabilities */}
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Sparkles className="h-5 w-5" />
              Capabilities
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="flex items-center justify-between p-3 rounded-lg border">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-4 w-4 text-blue-500" />
                  <Label className="text-sm cursor-pointer">Streaming</Label>
                </div>
                <Switch checked={supportsStreaming} onCheckedChange={setSupportsStreaming} />
              </div>
              <div className="flex items-center justify-between p-3 rounded-lg border">
                <div className="flex items-center gap-2">
                  <Icon icon="lucide:eye" width="16" height="16" className="text-purple-500" />
                  <Label className="text-sm cursor-pointer">Vision</Label>
                </div>
                <Switch checked={supportsVision} onCheckedChange={setSupportsVision} />
              </div>
              <div className="flex items-center justify-between p-3 rounded-lg border">
                <div className="flex items-center gap-2">
                  <Icon icon="lucide:wrench" width="16" height="16" className="text-green-500" />
                  <Label className="text-sm cursor-pointer">Function Calling</Label>
                </div>
                <Switch checked={supportsFunctions} onCheckedChange={setSupportsFunctions} />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Advanced Settings */}
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
                    <CardDescription>Rate limits, priority, pricing, and tags</CardDescription>
                  </div>
                  <ChevronRight className={`h-5 w-5 text-muted-foreground transition-transform ${advancedOpen ? "rotate-90" : ""}`} />
                </div>
              </CardHeader>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <CardContent className="space-y-6 pt-0">
                <Separator />
                <div>
                  <h4 className="text-sm font-medium flex items-center gap-2 mb-3"><Gauge className="h-4 w-4" />Rate Limits</h4>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label className="text-sm">RPM <span className="text-muted-foreground font-normal">(Requests/min)</span></Label>
                      <Input type="number" placeholder="100" value={rpm} onChange={(e) => setRpm(e.target.value)} />
                    </div>
                    <div className="space-y-2">
                      <Label className="text-sm">TPM <span className="text-muted-foreground font-normal">(Tokens/min)</span></Label>
                      <Input type="number" placeholder="100,000" value={tpm} onChange={(e) => setTpm(e.target.value)} />
                    </div>
                  </div>
                </div>
                <div>
                  <h4 className="text-sm font-medium flex items-center gap-2 mb-3"><Settings2 className="h-4 w-4" />Load Balancing</h4>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div className="space-y-2">
                      <Label className="text-sm">Priority (1-100)</Label>
                      <Input type="number" placeholder="50" min="1" max="100" value={priority} onChange={(e) => setPriority(e.target.value)} />
                    </div>
                    <div className="space-y-2">
                      <Label className="text-sm">Weight</Label>
                      <Input type="number" step="0.1" placeholder="1.0" value={weight} onChange={(e) => setWeight(e.target.value)} />
                    </div>
                    <div className="space-y-2">
                      <Label className="text-sm">Timeout (s)</Label>
                      <Input type="number" placeholder="60" value={timeoutSeconds} onChange={(e) => setTimeoutSeconds(e.target.value)} />
                    </div>
                  </div>
                </div>
                <div>
                  <h4 className="text-sm font-medium flex items-center gap-2 mb-3"><DollarSign className="h-4 w-4" />Pricing</h4>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label className="text-sm">Input Cost / Token</Label>
                      <Input type="number" step="0.000001" value={inputCost} onChange={(e) => setInputCost(e.target.value)} />
                    </div>
                    <div className="space-y-2">
                      <Label className="text-sm">Output Cost / Token</Label>
                      <Input type="number" step="0.000001" value={outputCost} onChange={(e) => setOutputCost(e.target.value)} />
                    </div>
                  </div>
                </div>
                <div>
                  <h4 className="text-sm font-medium flex items-center gap-2 mb-3"><Tag className="h-4 w-4" />Tags</h4>
                  <Input placeholder="production, fast, custom" value={tags} onChange={(e) => setTags(e.target.value)} className="max-w-md" />
                </div>
                <div>
                  <h4 className="text-sm font-medium flex items-center gap-2 mb-3"><Sparkles className="h-4 w-4" />Default Reasoning Effort</h4>
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
                    Default reasoning effort for reasoning models (o1, o3, GPT-5, etc.)
                  </p>
                </div>
              </CardContent>
            </CollapsibleContent>
          </Collapsible>
        </Card>

        {/* Actions */}
        <div className="flex items-center justify-between py-4 border-t">
          <Button type="button" variant="ghost" onClick={() => navigate("/models")}>
            Cancel
          </Button>
          <Button type="submit" disabled={mutation.isPending || !canSubmit} className="min-w-[140px]">
            {mutation.isPending ? "Saving..." : (
              <><Check className="h-4 w-4 mr-2" />Save Changes</>
            )}
          </Button>
        </div>
      </form>
    </div>
  );
}
