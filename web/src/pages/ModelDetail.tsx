import { useParams, Link } from "react-router-dom";
import { useEffect, useState } from "react";
import { ArrowLeft, ExternalLink, Settings, Activity, DollarSign, Zap, Clock, AlertCircle, CheckCircle, XCircle, Loader2 } from "lucide-react";
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
import { Separator } from "@/components/ui/separator";
import { ModelWithUsage } from "@/types/api";
import { detectProvider } from "@/lib/providers";
import { SparklineChart, MetricCard } from "@/components/models/ModelCharts";
import { FallbackDiagram } from "@/components/models/FallbackDiagram";
import { getModel, getSystemConfig, getModelStats } from "@/lib/api";

// Mock data - in real app this would come from API
const getMockModelDetail = (modelId: string): ModelWithUsage & {
  fallbacks?: string[];
  configuration: {
    provider: string;
    endpoint: string;
    maxTokens: number;
    temperature: number;
    topP: number;
    timeout: number;
    retries: number;
  };
  health: {
    status: 'healthy' | 'degraded' | 'unhealthy';
    uptime: number;
    errorRate: number;
    p99Latency: number;
  };
} => ({
  id: modelId,
  object: "model",
  created: Math.floor(Date.now() / 1000) - 86400,
  owned_by: modelId.includes("claude") ? "anthropic" : modelId.includes("gpt") ? "openai" : modelId.includes("gemini") ? "google" : "openrouter",
  usage_stats: {
    requests_today: Math.floor(Math.random() * 1000) + 100,
    tokens_today: Math.floor(Math.random() * 100000) + 10000,
    cost_today: Math.random() * 10 + 1,
    requests_total: Math.floor(Math.random() * 50000) + 5000,
    tokens_total: Math.floor(Math.random() * 5000000) + 500000,
    cost_total: Math.random() * 1000 + 100,
    avg_latency: Math.floor(Math.random() * 500) + 200,
    error_rate: Math.random() * 5,
    cache_hit_rate: Math.random() * 30 + 70,
    health_score: Math.floor(Math.random() * 30) + 70,
    trend_data: Array.from({ length: 30 }, () => Math.floor(Math.random() * 1000) + 100),
    last_used: new Date().toISOString()
  },
  fallbacks: modelId === "gpt-4" ? ["gpt-3.5-turbo", "claude-3-haiku"] : 
            modelId === "claude-3-opus" ? ["claude-3-sonnet", "gpt-4"] :
            [],
  configuration: {
    provider: modelId.includes("claude") ? "Anthropic" : modelId.includes("gpt") ? "OpenAI" : modelId.includes("gemini") ? "Google" : "OpenRouter",
    endpoint: "https://api.openai.com/v1/chat/completions",
    maxTokens: 4096,
    temperature: 0.7,
    topP: 1.0,
    timeout: 30000,
    retries: 3
  },
  health: {
    status: Math.random() > 0.3 ? 'healthy' : Math.random() > 0.1 ? 'degraded' : 'unhealthy',
    uptime: Math.random() * 10 + 95,
    errorRate: Math.random() * 5,
    p99Latency: Math.floor(Math.random() * 1000) + 300
  }
});

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
  const [model, setModel] = useState<ModelWithUsage | null>(null);
  const [fallbacks, setFallbacks] = useState<string[]>([]);
  const [allFallbacksConfig, setAllFallbacksConfig] = useState<Record<string, string[]>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  useEffect(() => {
    const fetchModelData = async () => {
      if (!modelId) {
        setError("Model ID is required");
        setLoading(false);
        return;
      }

      try {
        setLoading(true);
        const decodedModelId = decodeURIComponent(modelId);
        
        // Fetch model data and config in parallel
        const [modelResponse, configResponse, statsResponse] = await Promise.allSettled([
          getModel(decodedModelId),
          getSystemConfig(),
          getModelStats()
        ]);

        // Handle model data
        if (modelResponse.status === 'fulfilled') {
          setModel(modelResponse.value.data as ModelWithUsage);
        } else {
          console.warn('Failed to fetch model data:', modelResponse.reason);
          // Fallback to mock data if API fails
          setModel(getMockModelDetail(decodedModelId));
        }

        // Handle config data for fallbacks
        if (configResponse.status === 'fulfilled') {
          const config = configResponse.value as any;
          // Look for fallbacks in router configuration
          if (config.config?.router?.fallbacks || config.router?.fallbacks) {
            const fallbacksConfig = config.config?.router?.fallbacks || config.router?.fallbacks;
            const modelFallbacks = fallbacksConfig[decodedModelId] || [];
            setFallbacks(modelFallbacks);
            setAllFallbacksConfig(fallbacksConfig); // Store complete configuration for chain building
          } else {
            // No fallbacks configured in the system
            setFallbacks([]);
            setAllFallbacksConfig({});
          }
        } else {
          console.warn('Failed to fetch config:', configResponse.reason);
          // If we can't get the config, we can't show fallbacks
          setFallbacks([]);
          setAllFallbacksConfig({});
        }

        // Handle stats data - could be used for enhanced metrics
        if (statsResponse.status === 'fulfilled') {
          // Stats data could be used for enhanced metrics in the future
          // Could enhance model data with additional stats here
        }

      } catch (err) {
        console.error('Error fetching model data:', err);
        setError('Failed to load model data');
      } finally {
        setLoading(false);
      }
    };

    fetchModelData();
  }, [modelId]);

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

  const providerInfo = detectProvider(model.id, model.owned_by);
  const usage = model.usage_stats;

  // Create health data from usage stats or use defaults
  const health = {
    status: usage && usage.health_score >= 90 ? 'healthy' as const : 
            usage && usage.health_score >= 70 ? 'degraded' as const : 'unhealthy' as const,
    uptime: usage ? Math.min(usage.health_score + Math.random() * 10, 100) : 95 + Math.random() * 5,
    errorRate: usage ? usage.error_rate : Math.random() * 5,
    p99Latency: usage ? usage.avg_latency * 1.5 : 400 + Math.random() * 600
  };

  const getHealthIcon = (status: string) => {
    switch (status) {
      case 'healthy':
        return <CheckCircle className="h-5 w-5 text-green-500" />;
      case 'degraded':
        return <AlertCircle className="h-5 w-5 text-yellow-500" />;
      case 'unhealthy':
        return <XCircle className="h-5 w-5 text-red-500" />;
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
          <Button variant="ghost" size="sm" asChild>
            <Link to="/models">
              <ArrowLeft className="h-4 w-4" />
              Back to Models
            </Link>
          </Button>
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
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={getHealthBadgeVariant(health.status)} className="gap-1">
            {getHealthIcon(health.status)}
            {health.status}
          </Badge>
          <Button variant="outline" size="sm">
            <Settings className="h-4 w-4" />
            Configure
          </Button>
        </div>
      </div>

      {/* Overview Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <MetricCard
          label="Requests Today"
          value={formatNumber(usage?.requests_today || 0)}
          trend={usage?.trend_data?.slice(-7)}
          icon={<Activity className="h-4 w-4 text-blue-500" />}
          color="#3b82f6"
        />
        <MetricCard
          label="Tokens Today"
          value={formatNumber(usage?.tokens_today || 0)}
          icon={<Zap className="h-4 w-4 text-purple-500" />}
          color="#8b5cf6"
        />
        <MetricCard
          label="Cost Today"
          value={formatCurrency(usage?.cost_today || 0)}
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
              <CardHeader>
                <CardTitle>Usage Trend (30 Days)</CardTitle>
                <CardDescription>Request volume over time</CardDescription>
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
          {fallbacks && fallbacks.length > 0 ? (
            <FallbackDiagram 
              primaryModel={model.id}
              fallbacks={fallbacks}
              allFallbacksConfig={allFallbacksConfig}
            />
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>Fallback Configuration</CardTitle>
                <CardDescription>No fallback models configured</CardDescription>
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground">
                  This model doesn't have any fallback models configured. 
                  Consider adding fallbacks to improve reliability.
                </p>
                <Button className="mt-4" variant="outline">
                  <Settings className="h-4 w-4" />
                  Configure Fallbacks
                </Button>
              </CardContent>
            </Card>
          )}
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
                  <span className="font-medium">{getMockModelDetail(model.id).configuration.provider}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Endpoint</span>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm truncate max-w-48">
                      {getMockModelDetail(model.id).configuration.endpoint}
                    </span>
                    <ExternalLink className="h-3 w-3" />
                  </div>
                </div>
                <div className="flex items-center justify-between">
                  <span>Timeout</span>
                  <span className="font-medium">{getMockModelDetail(model.id).configuration.timeout / 1000}s</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Max Retries</span>
                  <span className="font-medium">{getMockModelDetail(model.id).configuration.retries}</span>
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
                  <span className="font-medium">{getMockModelDetail(model.id).configuration.maxTokens}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Temperature</span>
                  <span className="font-medium">{getMockModelDetail(model.id).configuration.temperature}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Top P</span>
                  <span className="font-medium">{getMockModelDetail(model.id).configuration.topP}</span>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}