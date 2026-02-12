import { useParams, Link } from "react-router-dom";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ExternalLink, Settings, Activity, DollarSign, Zap, Clock, AlertCircle, CheckCircle, XCircle, Loader2 } from "lucide-react";
import { Icon } from "@iconify/react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { ModelWithUsage } from "@/types/api";
import { detectProvider } from "@/lib/providers";
import { SparklineChart, MetricCard } from "@/components/models/ModelCharts";
import { EditableFallbackDiagram } from "@/components/models/EditableFallbackDiagram";
import EditModelDialog from "@/components/models/EditModelDialog";
import DeleteModelDialog from "@/components/models/DeleteModelDialog";
import { getSystemConfig, getModelMetrics, getModelTrends, getAdminModels, getModelsHealth, updateConfig } from "@/lib/api";
import { fillTimeGaps } from "@/lib/date-utils";
import type { AdminModel, AdminModelsResponse, ModelsHealthResponse } from "@/types/api";

const formatNumber = (num: number): string => {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`;
  if (num >= 1000) return `${(num / 1000).toFixed(1)}K`;
  return num.toString();
};

const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
  }).format(amount);
};

export default function ModelDetail() {
  const { modelId } = useParams<{ modelId: string }>();
  const decodedModelId = modelId ? decodeURIComponent(modelId) : "";
  const queryClient = useQueryClient();

  // Use React Query for admin models so the cache is shared with EditModel
  const { data: adminModelsData } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
    refetchInterval: 60000,
  });

  const { data: healthData } = useQuery({
    queryKey: ["models-health"],
    queryFn: getModelsHealth,
    refetchInterval: 60000,
  });

  const { data: metricsRaw, isLoading: metricsLoading, error: metricsError } = useQuery({
    queryKey: ["model-metrics", decodedModelId],
    queryFn: () => getModelMetrics(decodedModelId),
    refetchInterval: 60000,
    enabled: !!modelId,
  });

  const [trendTimeRange, setTrendTimeRange] = useState("30d");

  const trendParams = useMemo(() => {
    switch (trendTimeRange) {
      case "24h":
        return { hours: 24, interval: "hourly" };
      case "7d":
        return { days: 7, interval: "daily" };
      case "30d":
      default:
        return { days: 30, interval: "daily" };
    }
  }, [trendTimeRange]);

  const { data: trendsRaw } = useQuery({
    queryKey: ["model-trends", decodedModelId, trendParams],
    queryFn: () => getModelTrends(decodedModelId, trendParams),
    refetchInterval: 60000,
    enabled: !!modelId,
  });

  const { data: configRaw } = useQuery({
    queryKey: ["system-config"],
    queryFn: getSystemConfig,
    enabled: !!modelId,
  });

  const adminModel: AdminModel | null = (() => {
    if (!modelId) return null;
    const decodedId = decodeURIComponent(modelId);
    const data = adminModelsData as AdminModelsResponse | undefined;
    const matches = data?.models?.filter((m) =>
      m.model_name === decodedId || m.id === decodedId
    ) || [];
    // Prefer the entry with provider details (database version) over registry-only entries
    return matches.find((m) => m.provider) || matches[0] || null;
  })();

  // Derive model, configuration, and fallbacks from query data
  const { model, configuration, allFallbacksConfig } = useMemo(() => {
    if (!metricsRaw) return { model: null, configuration: null, allFallbacksConfig: {} as Record<string, string[]> };

    const metricsValue = metricsRaw as any;
    const metrics = metricsValue.data || metricsValue;
    const providerInfo = detectProvider(decodedModelId, decodedModelId.includes("claude") ? "anthropic" :
                                        decodedModelId.includes("gpt") ? "openai" :
                                        decodedModelId.includes("gemini") ? "google" : "openrouter");

    // Get trend data if available, filling time gaps
    let trendData: number[] = [];
    if (trendsRaw) {
      const trendsValue = trendsRaw as any;
      const trends = trendsValue?.data ? (Array.isArray(trendsValue.data) ? trendsValue.data : []) :
                    (Array.isArray(trendsValue) ? trendsValue : []);
      const currentInterval = (trendParams.interval || "daily") as "hourly" | "daily";
      const range = trendParams.hours || trendParams.days || 30;
      const filled = fillTimeGaps(trends, currentInterval, range);
      trendData = filled.map((t: any) => t.requests || 0);
    }

    // Calculate health score from success rate and latency
    const healthScore = metrics.success_rate >= 95 && metrics.avg_latency < 1000 ? 100 :
                       metrics.success_rate >= 90 && metrics.avg_latency < 2000 ? 85 :
                       metrics.success_rate >= 80 && metrics.avg_latency < 3000 ? 70 : 50;

    const modelData: ModelWithUsage = {
      id: decodedModelId,
      object: "model",
      created: Math.floor(Date.now() / 1000),
      owned_by: providerInfo.name.toLowerCase(),
      provider: providerInfo.name.toLowerCase(),
      is_active: metrics.total_requests > 0,
      usage_stats: {
        requests_today: 0,
        requests_total: metrics.total_requests || 0,
        tokens_today: 0,
        tokens_total: metrics.total_tokens || 0,
        cost_today: 0,
        cost_total: metrics.total_cost || 0,
        avg_latency: metrics.avg_latency || 0,
        error_rate: metrics.success_rate ? (100 - metrics.success_rate) : 0,
        cache_hit_rate: metrics.cache_hit_rate || 0,
        health_score: healthScore,
        trend_data: trendData,
        last_used: metrics.last_used || null,
      }
    };

    // Add health status
    (modelData as any).health = {
      status: metrics.success_rate >= 95 ? 'healthy' : metrics.success_rate >= 80 ? 'degraded' : 'unhealthy',
      uptime: metrics.success_rate || 0,
      errorRate: metrics.success_rate ? (100 - metrics.success_rate) : 100,
      p99Latency: metrics.avg_latency || 0
    };

    const configData = {
      provider: providerInfo.name,
      endpoint: "Configured via system config",
      maxTokens: 4096,
      temperature: 0.7,
      topP: 1.0,
      timeout: 30000,
      retries: 3
    };

    (modelData as any).configuration = configData;

    // Handle config data for fallbacks
    let fallbacksConfig: Record<string, string[]> = {};
    if (configRaw) {
      const config = configRaw as any;
      if (config.config?.router?.fallbacks || config.router?.fallbacks) {
        fallbacksConfig = config.config?.router?.fallbacks || config.router?.fallbacks;
      }
    }

    return { model: modelData, configuration: configData, allFallbacksConfig: fallbacksConfig };
  }, [metricsRaw, trendsRaw, configRaw, decodedModelId]);

  const loading = metricsLoading;
  const error = metricsError ? 'Failed to load model metrics' : (!modelId ? 'Model ID is required' : null);
  const modelPricing: any = null;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin" />
        <span className="ml-2">Loading model details...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-8">
        <p className="text-red-500">{error}</p>
        <Button variant="outline" asChild className="mt-4">
          <Link to="/models">Back to Models</Link>
        </Button>
      </div>
    );
  }

  if (!model) {
    return <div>Model not found</div>;
  }

  // Use real provider type from admin model when available (e.g., "azure" instead of guessed "openai")
  const providerInfo = detectProvider(model.id, adminModel?.provider?.type || model.owned_by);
  const usage = model.usage_stats;

  // Derive health from real health-check data when available
  const modelHealthData = (() => {
    const hd = healthData as ModelsHealthResponse | undefined;
    if (!hd?.models || !model) return null;
    return hd.models[model.id] ?? null;
  })();

  const health = (() => {
    if (modelHealthData && modelHealthData.total_count > 0) {
      const ratio = modelHealthData.healthy_count / modelHealthData.total_count;
      const status = ratio >= 1 ? 'healthy' as const :
                     ratio > 0 ? 'degraded' as const : 'unhealthy' as const;
      return {
        status,
        uptime: ratio * 100,
        errorRate: (1 - ratio) * 100,
        p99Latency: modelHealthData.avg_latency_ms,
        lastChecked: modelHealthData.last_checked_at,
      };
    }
    // Fallback to usage-metric-derived health
    if (usage) {
      return {
        status: usage.health_score >= 90 ? 'healthy' as const :
                usage.health_score >= 70 ? 'degraded' as const : 'unhealthy' as const,
        uptime: usage.health_score,
        errorRate: usage.error_rate,
        p99Latency: usage.avg_latency * 1.5,
        lastChecked: null as string | null,
      };
    }
    // No data at all — neutral "unknown" state
    return {
      status: 'unknown' as const,
      uptime: 0,
      errorRate: 0,
      p99Latency: 0,
      lastChecked: null as string | null,
    };
  })();

  const getHealthIcon = (status: string) => {
    switch (status) {
      case 'healthy':
        return <CheckCircle className="h-5 w-5 text-green-500" />;
      case 'degraded':
        return <AlertCircle className="h-5 w-5 text-yellow-500" />;
      case 'unhealthy':
        return <XCircle className="h-5 w-5 text-red-500" />;
      case 'unknown':
        return <Clock className="h-5 w-5 text-gray-400" />;
      default:
        return <AlertCircle className="h-5 w-5 text-gray-500" />;
    }
  };

  const getHealthBadgeVariant = (status: string) => {
    switch (status) {
      case 'healthy':
        return "default";
      case 'degraded':
        return "secondary";
      case 'unhealthy':
        return "destructive";
      default:
        return "outline";
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className={`p-3 rounded-xl border ${providerInfo.bgColor} ${providerInfo.borderColor}`}>
            <Icon
              icon={providerInfo.icon}
              width="32"
              height="32"
              className={providerInfo.color}
            />
          </div>
          <div>
            <h1 className="text-2xl font-bold">{model.id}</h1>
            <p className={`text-lg ${providerInfo.color}`}>
              {providerInfo.name}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {adminModel && (
            <Badge variant={adminModel.source === "user" ? "outline" : "secondary"}>
              {adminModel.source === "user" ? "User" : "System"}
            </Badge>
          )}
          <Badge variant={getHealthBadgeVariant(health.status)} className="gap-1">
            {getHealthIcon(health.status)}
            {health.status}
          </Badge>
          {adminModel?.source === "user" && (
            <EditModelDialog model={adminModel} trigger={
              <Button variant="outline" size="sm">
                <Settings className="h-4 w-4 mr-1" />
                Edit
              </Button>
            } />
          )}
          {adminModel && (
            <DeleteModelDialog
              modelId={adminModel.id}
              modelName={adminModel.model_name}
              trigger={
                <Button variant="outline" size="sm" className="text-destructive hover:text-destructive">
                  Delete
                </Button>
              }
            />
          )}
        </div>
      </div>

      {/* Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <MetricCard
          label="Total Requests"
          value={formatNumber(usage?.requests_total || 0)}
          trend={usage?.trend_data?.slice(-7)}
          icon={<Activity className="h-4 w-4 text-blue-500" />}
          color="#3b82f6"
        />
        <MetricCard
          label="Total Tokens"
          value={formatNumber(usage?.tokens_total || 0)}
          icon={<Zap className="h-4 w-4 text-purple-500" />}
          color="#8b5cf6"
        />
        <MetricCard
          label="Total Cost"
          value={formatCurrency(usage?.cost_total || 0)}
          icon={<DollarSign className="h-4 w-4 text-green-500" />}
          color="#10b981"
        />
        <MetricCard
          label="Avg Latency"
          value={`${usage?.avg_latency || 0}ms`}
          icon={<Clock className="h-4 w-4 text-orange-500" />}
          color="#f59e0b"
        />
      </div>

      {/* Main Content Tabs */}
      <Tabs defaultValue="overview" className="space-y-6">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="metrics">Metrics</TabsTrigger>
          <TabsTrigger value="fallbacks">Fallbacks</TabsTrigger>
          <TabsTrigger value="configuration">Configuration</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Usage Trend */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0">
                <div>
                  <CardTitle>
                    Usage Trend ({trendTimeRange === "24h" ? "24h" : trendTimeRange === "7d" ? "7 Days" : "30 Days"})
                  </CardTitle>
                  <CardDescription>Request volume over time</CardDescription>
                </div>
                <Select value={trendTimeRange} onValueChange={setTrendTimeRange}>
                  <SelectTrigger className="w-[140px]" aria-label="Select time range">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="30d">Last 30 days</SelectItem>
                    <SelectItem value="7d">Last 7 days</SelectItem>
                    <SelectItem value="24h">Last 24 hours</SelectItem>
                  </SelectContent>
                </Select>
              </CardHeader>
              <CardContent>
                {usage?.trend_data && (
                  <SparklineChart 
                    data={usage.trend_data}
                    type="area"
                    color="#3b82f6"
                    className="h-32 w-full"
                  />
                )}
              </CardContent>
            </Card>

            {/* Health Status */}
            <Card>
              <CardHeader>
                <CardTitle>Health Status</CardTitle>
                <CardDescription>Current model health metrics</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Status</span>
                  <div className="flex items-center gap-2">
                    {getHealthIcon(health.status)}
                    <span className="capitalize">{health.status}</span>
                  </div>
                </div>
                <Separator />
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Uptime</span>
                  <span>{health.uptime.toFixed(2)}%</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">Error Rate</span>
                  <span>{health.errorRate.toFixed(2)}%</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium">P99 Latency</span>
                  <span>{health.p99Latency}ms</span>
                </div>
                {health.lastChecked && (
                  <>
                    <Separator />
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium">Last Checked</span>
                      <span className="text-sm text-muted-foreground">
                        {new Date(health.lastChecked).toLocaleString()}
                      </span>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="metrics" className="space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Request Volume</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <span>Today</span>
                    <span className="font-medium">{formatNumber(usage?.requests_today || 0)}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span>Total</span>
                    <span className="font-medium">{formatNumber(usage?.requests_total || 0)}</span>
                  </div>
                </div>
              </CardContent>
            </Card>
            
            <Card>
              <CardHeader>
                <CardTitle>Token Usage</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <span>Today</span>
                    <span className="font-medium">{formatNumber(usage?.tokens_today || 0)}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span>Total</span>
                    <span className="font-medium">{formatNumber(usage?.tokens_total || 0)}</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="fallbacks" className="space-y-6">
          <EditableFallbackDiagram
            primaryModel={model.id}
            allFallbacksConfig={allFallbacksConfig}
            availableModels={
              (adminModelsData as AdminModelsResponse | undefined)?.models?.map(m => m.model_name) ?? []
            }
            onSave={async (newConfig) => {
              try {
                await updateConfig({ router: { fallbacks: newConfig } });
                queryClient.invalidateQueries({ queryKey: ["system-config"] });
              } catch (err) {
                console.error('Failed to save fallback config:', err);
              }
            }}
          />
        </TabsContent>

        <TabsContent value="configuration" className="space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Provider Configuration</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <span>Provider</span>
                  <span className="font-medium">{configuration?.provider || 'N/A'}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Endpoint</span>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm truncate max-w-48">
                      {configuration?.endpoint || 'N/A'}
                    </span>
                    {configuration?.endpoint && <ExternalLink className="h-3 w-3" />}
                  </div>
                </div>
                <div className="flex items-center justify-between">
                  <span>Timeout</span>
                  <span className="font-medium">{configuration?.timeout ? `${configuration.timeout / 1000}s` : 'N/A'}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Max Retries</span>
                  <span className="font-medium">{configuration?.retries ?? 'N/A'}</span>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Model Parameters</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <span>Max Tokens</span>
                  <span className="font-medium">{modelPricing?.max_tokens || configuration?.maxTokens || 'N/A'}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Provider</span>
                  <span className="font-medium capitalize">{modelPricing?.provider || model.owned_by || 'N/A'}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Input Cost (per token)</span>
                  <span className="font-medium">
                    {modelPricing?.input_cost_per_token ? `$${modelPricing.input_cost_per_token.toFixed(6)}` : 'N/A'}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Output Cost (per token)</span>
                  <span className="font-medium">
                    {modelPricing?.output_cost_per_token ? `$${modelPricing.output_cost_per_token.toFixed(6)}` : 'N/A'}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Input Cost (per 1K tokens)</span>
                  <span className="font-medium">
                    {modelPricing?.input_cost_per_token ? `$${(modelPricing.input_cost_per_token * 1000).toFixed(4)}` : 'N/A'}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Output Cost (per 1K tokens)</span>
                  <span className="font-medium">
                    {modelPricing?.output_cost_per_token ? `$${(modelPricing.output_cost_per_token * 1000).toFixed(4)}` : 'N/A'}
                  </span>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}