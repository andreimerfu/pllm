import { useParams, Link } from "react-router-dom";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { getProviderLogo, getProviderColor } from "@/lib/provider-logos";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { ModelWithUsage } from "@/types/api";
import { detectProvider } from "@/lib/providers";
import { SparklineChart } from "@/components/models/ModelCharts";
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

// Pulsing status dot component
function StatusDot({ status }: { status: string }) {
  const colorMap: Record<string, string> = {
    healthy: "bg-emerald-500",
    degraded: "bg-amber-500",
    unhealthy: "bg-red-500",
    unknown: "bg-gray-400",
  };
  const glowMap: Record<string, string> = {
    healthy: "shadow-emerald-500/50",
    degraded: "shadow-amber-500/50",
    unhealthy: "shadow-red-500/50",
    unknown: "shadow-gray-400/50",
  };
  const dotColor = colorMap[status] || colorMap.unknown;
  const glowColor = glowMap[status] || glowMap.unknown;

  return (
    <span className="relative flex h-3 w-3">
      <span
        className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${dotColor}`}
      />
      <span
        className={`relative inline-flex rounded-full h-3 w-3 shadow-lg ${dotColor} ${glowColor}`}
      />
    </span>
  );
}

// Metric chip — small inline card
function MetricChip({
  icon,
  label,
  value,
  iconColor,
}: {
  icon: string;
  label: string;
  value: string;
  iconColor?: string;
}) {
  return (
    <div className="flex items-center gap-2.5 px-4 py-2 rounded-full border bg-card/60 backdrop-blur-sm">
      <Icon icon={icon} className={`h-4 w-4 flex-shrink-0 ${iconColor || "text-muted-foreground"}`} />
      <span className="text-xs text-muted-foreground whitespace-nowrap">{label}</span>
      <span className="text-sm font-semibold whitespace-nowrap">{value}</span>
    </div>
  );
}

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
        <Icon icon="solar:refresh-circle-linear" className="h-8 w-8 animate-spin" />
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
  const providerColor = getProviderColor(adminModel?.provider?.type || model.owned_by || "");
  const providerLogo = getProviderLogo(adminModel?.provider?.type || model.owned_by || "");

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

  // Extract a friendly display name from the model ID
  const displayName = (() => {
    const id = model.id;
    // Take the last segment after "/" if present, then capitalize words
    const baseName = id.includes("/") ? id.split("/").pop()! : id;
    return baseName
      .replace(/[-_]/g, " ")
      .replace(/\b\w/g, (c) => c.toUpperCase());
  })();

  // Fallback chain for this model
  const fallbackChain = allFallbacksConfig[model.id] || [];

  // Capabilities derived from model name heuristics
  const capabilities = (() => {
    const id = model.id.toLowerCase();
    const caps: string[] = [];
    if (id.includes("vision") || id.includes("4o") || id.includes("gemini")) caps.push("Vision");
    if (id.includes("code") || id.includes("codex")) caps.push("Code");
    if (id.includes("embed")) caps.push("Embeddings");
    if (!id.includes("embed")) caps.push("Chat");
    if (id.includes("gpt-4") || id.includes("claude-3") || id.includes("gemini")) caps.push("Function Calling");
    if (id.includes("whisper") || id.includes("tts")) caps.push("Audio");
    return caps;
  })();

  return (
    <div className="space-y-6">
      {/* ===== HERO SECTION ===== */}
      <div
        className="relative rounded-2xl border overflow-hidden"
        style={{
          background: `linear-gradient(135deg, ${providerColor}08 0%, ${providerColor}14 50%, transparent 100%)`,
        }}
      >
        {/* Subtle decorative circles */}
        <div
          className="absolute -top-16 -right-16 w-64 h-64 rounded-full opacity-[0.04]"
          style={{ background: providerColor }}
        />
        <div
          className="absolute -bottom-8 -left-8 w-40 h-40 rounded-full opacity-[0.03]"
          style={{ background: providerColor }}
        />

        <div className="relative p-6 md:p-8">
          <div className="flex items-start justify-between gap-4">
            {/* Left: Logo + Name */}
            <div className="flex items-start gap-5">
              <div
                className="flex items-center justify-center w-14 h-14 rounded-xl border shadow-sm"
                style={{
                  backgroundColor: `${providerColor}10`,
                  borderColor: `${providerColor}25`,
                }}
              >
                <Icon
                  icon={providerLogo}
                  width="32"
                  height="32"
                  className={providerInfo.color}
                />
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-3 mb-1">
                  <h1 className="text-2xl md:text-3xl font-bold tracking-tight truncate">
                    {displayName}
                  </h1>
                  <StatusDot status={health.status} />
                  <span className="text-xs font-medium capitalize text-muted-foreground">
                    {health.status}
                  </span>
                </div>
                <p className="font-mono text-sm text-muted-foreground truncate">
                  {model.id}
                </p>
                <div className="flex items-center gap-2 mt-2">
                  <Badge
                    variant="outline"
                    className="text-xs"
                    style={{ borderColor: `${providerColor}40`, color: providerColor }}
                  >
                    {providerInfo.name}
                  </Badge>
                  {adminModel && (
                    <Badge variant={adminModel.source === "user" ? "outline" : "secondary"} className="text-xs">
                      {adminModel.source === "user" ? "User" : "System"}
                    </Badge>
                  )}
                </div>
              </div>
            </div>

            {/* Right: Action buttons */}
            <div className="flex items-center gap-2 flex-shrink-0">
              {adminModel?.source === "user" && (
                <EditModelDialog model={adminModel} trigger={
                  <Button variant="ghost" size="sm" className="gap-1.5">
                    <Icon icon={icons.edit} className="h-4 w-4" />
                    Edit
                  </Button>
                } />
              )}
              {adminModel && (
                <DeleteModelDialog
                  modelId={adminModel.id}
                  modelName={adminModel.model_name}
                  trigger={
                    <Button variant="ghost" size="sm" className="gap-1.5 text-destructive hover:text-destructive">
                      <Icon icon={icons.delete} className="h-4 w-4" />
                      Delete
                    </Button>
                  }
                />
              )}
            </div>
          </div>

          {/* Metric Chips row */}
          <div className="flex flex-wrap items-center gap-2 mt-6">
            <MetricChip
              icon="solar:graph-up-linear"
              label="Requests"
              value={formatNumber(usage?.requests_total || 0)}
              iconColor="text-teal-500"
            />
            <MetricChip
              icon="solar:bolt-linear"
              label="Tokens"
              value={formatNumber(usage?.tokens_total || 0)}
              iconColor="text-purple-500"
            />
            <MetricChip
              icon="solar:dollar-minimalistic-linear"
              label="Cost"
              value={formatCurrency(usage?.cost_total || 0)}
              iconColor="text-emerald-500"
            />
            <MetricChip
              icon={icons.clock}
              label="Avg Latency"
              value={`${usage?.avg_latency || 0}ms`}
              iconColor="text-amber-500"
            />
            <MetricChip
              icon="solar:shield-check-linear"
              label="Success"
              value={`${(100 - (usage?.error_rate || 0)).toFixed(1)}%`}
              iconColor="text-blue-500"
            />
            {(usage?.cache_hit_rate || 0) > 0 && (
              <MetricChip
                icon="solar:database-linear"
                label="Cache Hit"
                value={`${(usage?.cache_hit_rate || 0).toFixed(1)}%`}
                iconColor="text-cyan-500"
              />
            )}
          </div>
        </div>
      </div>

      {/* ===== MAIN CONTENT: Performance (2/3) + Configuration (1/3) ===== */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Performance Panel — left 2/3 */}
        <div className="lg:col-span-2 space-y-6">
          {/* Request Volume Chart */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <div>
                <CardTitle className="text-base">Request Volume</CardTitle>
                <CardDescription>
                  {trendTimeRange === "24h" ? "Last 24 hours" : trendTimeRange === "7d" ? "Last 7 days" : "Last 30 days"}
                </CardDescription>
              </div>
              <div className="flex items-center gap-1 bg-muted rounded-full p-0.5">
                {(["24h", "7d", "30d"] as const).map((range) => (
                  <button
                    key={range}
                    onClick={() => setTrendTimeRange(range)}
                    className={`px-3 py-1 text-xs font-medium rounded-full transition-all ${
                      trendTimeRange === range
                        ? "bg-background text-foreground shadow-sm"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {range}
                  </button>
                ))}
              </div>
            </CardHeader>
            <CardContent>
              {usage?.trend_data && usage.trend_data.length > 0 ? (
                <SparklineChart
                  data={usage.trend_data}
                  type="area"
                  color="#14B8A6"
                  className="h-40 w-full"
                />
              ) : (
                <div className="h-40 flex items-center justify-center text-muted-foreground text-sm">
                  No trend data available
                </div>
              )}
            </CardContent>
          </Card>

          {/* Latency & Health Stats */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">Health &amp; Performance</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">Uptime</p>
                  <p className="text-xl font-bold">{health.uptime.toFixed(1)}%</p>
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">Error Rate</p>
                  <p className="text-xl font-bold">{health.errorRate.toFixed(2)}%</p>
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">P99 Latency</p>
                  <p className="text-xl font-bold">{Math.round(health.p99Latency)}ms</p>
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">Avg Latency</p>
                  <p className="text-xl font-bold">{usage?.avg_latency || 0}ms</p>
                </div>
              </div>
              {health.lastChecked && (
                <>
                  <Separator className="my-4" />
                  <p className="text-xs text-muted-foreground">
                    Last health check: {new Date(health.lastChecked).toLocaleString()}
                  </p>
                </>
              )}
            </CardContent>
          </Card>

          {/* Usage breakdown */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-base">Request Volume</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Today</span>
                  <span className="font-semibold">{formatNumber(usage?.requests_today || 0)}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Total</span>
                  <span className="font-semibold">{formatNumber(usage?.requests_total || 0)}</span>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-base">Token Usage</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Today</span>
                  <span className="font-semibold">{formatNumber(usage?.tokens_today || 0)}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Total</span>
                  <span className="font-semibold">{formatNumber(usage?.tokens_total || 0)}</span>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>

        {/* Configuration Panel — right 1/3, dark sidebar style */}
        <div className="space-y-6">
          <Card className="bg-zinc-950 dark:bg-zinc-950 text-zinc-100 border-zinc-800">
            <CardHeader className="pb-3">
              <CardTitle className="text-sm font-medium text-zinc-400 uppercase tracking-wider">
                Configuration
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* Key-value pairs in monospace */}
              <div className="space-y-3 font-mono text-sm">
                <div className="flex justify-between gap-2">
                  <span className="text-zinc-500">provider</span>
                  <span className="text-zinc-200 truncate">{configuration?.provider || "N/A"}</span>
                </div>
                <div className="flex justify-between gap-2">
                  <span className="text-zinc-500">max_tokens</span>
                  <span className="text-zinc-200">{modelPricing?.max_tokens || configuration?.maxTokens || "N/A"}</span>
                </div>
                <div className="flex justify-between gap-2">
                  <span className="text-zinc-500">timeout</span>
                  <span className="text-zinc-200">{configuration?.timeout ? `${configuration.timeout / 1000}s` : "N/A"}</span>
                </div>
                <div className="flex justify-between gap-2">
                  <span className="text-zinc-500">retries</span>
                  <span className="text-zinc-200">{configuration?.retries ?? "N/A"}</span>
                </div>
                <Separator className="bg-zinc-800" />
                <div className="flex justify-between gap-2">
                  <span className="text-zinc-500">input_cost</span>
                  <span className="text-zinc-200">
                    {modelPricing?.input_cost_per_token
                      ? `$${(modelPricing.input_cost_per_token * 1000).toFixed(4)}/1K`
                      : "N/A"}
                  </span>
                </div>
                <div className="flex justify-between gap-2">
                  <span className="text-zinc-500">output_cost</span>
                  <span className="text-zinc-200">
                    {modelPricing?.output_cost_per_token
                      ? `$${(modelPricing.output_cost_per_token * 1000).toFixed(4)}/1K`
                      : "N/A"}
                  </span>
                </div>
              </div>

              {/* Capabilities as colored badges */}
              {capabilities.length > 0 && (
                <>
                  <Separator className="bg-zinc-800" />
                  <div>
                    <p className="text-xs text-zinc-500 uppercase tracking-wider mb-2">Capabilities</p>
                    <div className="flex flex-wrap gap-1.5">
                      {capabilities.map((cap) => {
                        const capColors: Record<string, string> = {
                          Chat: "bg-teal-500/20 text-teal-300 border-teal-500/30",
                          Vision: "bg-violet-500/20 text-violet-300 border-violet-500/30",
                          Code: "bg-blue-500/20 text-blue-300 border-blue-500/30",
                          Embeddings: "bg-amber-500/20 text-amber-300 border-amber-500/30",
                          "Function Calling": "bg-pink-500/20 text-pink-300 border-pink-500/30",
                          Audio: "bg-cyan-500/20 text-cyan-300 border-cyan-500/30",
                        };
                        return (
                          <span
                            key={cap}
                            className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium border ${
                              capColors[cap] || "bg-zinc-800 text-zinc-400 border-zinc-700"
                            }`}
                          >
                            {cap}
                          </span>
                        );
                      })}
                    </div>
                  </div>
                </>
              )}

              {/* Fallback chain in sidebar */}
              {fallbackChain.length > 0 && (
                <>
                  <Separator className="bg-zinc-800" />
                  <div>
                    <p className="text-xs text-zinc-500 uppercase tracking-wider mb-3">Fallback Chain</p>
                    <div className="space-y-1">
                      {/* Primary */}
                      <div className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-zinc-900 border border-zinc-800">
                        <Icon icon={providerLogo} width="14" height="14" />
                        <span className="text-xs font-mono text-zinc-200 truncate">{model.id}</span>
                        <Badge variant="outline" className="ml-auto text-[10px] px-1.5 py-0 border-teal-600 text-teal-400">
                          primary
                        </Badge>
                      </div>
                      {fallbackChain.map((fbId, idx) => {
                        const fbProvider = detectProvider(fbId, "");
                        const fbLogo = getProviderLogo(fbProvider.name.toLowerCase());
                        return (
                          <div key={fbId}>
                            <div className="flex justify-center py-0.5">
                              <Icon icon={icons.arrowDown} className="h-3 w-3 text-zinc-600" />
                            </div>
                            <div className="flex items-center gap-2 px-2.5 py-1.5 rounded bg-zinc-900 border border-zinc-800">
                              <Icon icon={fbLogo} width="14" height="14" />
                              <span className="text-xs font-mono text-zinc-300 truncate">{fbId}</span>
                              <span className="ml-auto text-[10px] text-zinc-600">#{idx + 1}</span>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* ===== FALLBACK DIAGRAM (full-width) ===== */}
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
    </div>
  );
}
