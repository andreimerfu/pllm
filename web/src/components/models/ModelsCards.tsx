import { Icon } from "@iconify/react";
import { Activity, DollarSign, Zap, Clock } from "lucide-react";
import { useNavigate } from "react-router-dom";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ModelWithUsage } from "@/types/api";
import { detectProvider } from "@/lib/providers";
import { SparklineChart, MetricCard } from "./ModelCharts";
import ModelTags from "./ModelTags";
import ModelCapabilities from "./ModelCapabilities";

interface ModelsCardsProps {
  models: ModelWithUsage[];
}

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

export default function ModelsCards({ models }: ModelsCardsProps) {
  const navigate = useNavigate();

  const handleModelClick = (modelId: string) => {
    navigate(`/models/${encodeURIComponent(modelId)}`);
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4 gap-6">
      {models.map((model) => {
        const providerInfo = detectProvider(model.id, model.owned_by);
        const usage = model.usage_stats;
        const isActive = model.is_active !== false;

        return (
          <Card 
            key={model.id} 
            className="transition-all hover:shadow-md group cursor-pointer relative overflow-hidden"
            onClick={() => handleModelClick(model.id)}
          >
            {/* Background gradient overlay */}
            <div
              className={`absolute inset-0 opacity-5 group-hover:opacity-10 transition-opacity ${providerInfo.bgColor}`}
            />

            <CardHeader className="pb-4 relative z-10">
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-start gap-4 min-w-0">
                  {/* Provider icon */}
                  <div
                    className={`flex-shrink-0 p-3 rounded-xl border ${providerInfo.bgColor} ${providerInfo.borderColor} shadow-sm`}
                  >
                    <Icon
                      icon={providerInfo.icon}
                      width="32"
                      height="32"
                      className={providerInfo.color}
                    />
                  </div>

                  <div className="min-w-0 flex-1">
                    <CardTitle className="text-base font-bold leading-tight">
                      <span className="block truncate">{model.id}</span>
                    </CardTitle>
                    <CardDescription className={`mt-1 font-medium ${providerInfo.color}`}>
                      {providerInfo.name}
                    </CardDescription>
                  </div>
                </div>

                <div className="flex flex-col gap-2 items-end">
                  <Badge
                    variant={model.object ? "default" : "secondary"}
                    className="flex-shrink-0 font-medium text-xs"
                  >
                    {model.object}
                  </Badge>
                  {usage && (
                    <div className="flex items-center gap-1">
                      <div className={`w-2 h-2 rounded-full ${
                        usage.health_score >= 90 
                          ? 'bg-green-500' 
                          : usage.health_score >= 70 
                            ? 'bg-yellow-500' 
                            : 'bg-red-500'
                      }`} />
                      <span className="text-xs text-muted-foreground">
                        {usage.health_score}%
                      </span>
                    </div>
                  )}
                </div>
              </div>
            </CardHeader>

            <CardContent className="space-y-4 relative z-10">
              {/* Tags and Capabilities */}
              {(model.tags?.length || model.capabilities) && (
                <div className="space-y-3 pb-4 border-b border-border/50">
                  {model.tags && model.tags.length > 0 && (
                    <div>
                      <div className="text-xs text-muted-foreground mb-2 font-medium">
                        Tags
                      </div>
                      <ModelTags tags={model.tags} maxVisible={3} />
                    </div>
                  )}
                  {model.capabilities && (
                    <div>
                      <div className="text-xs text-muted-foreground mb-2 font-medium">
                        Capabilities
                      </div>
                      <ModelCapabilities capabilities={model.capabilities} maxVisible={6} />
                    </div>
                  )}
                </div>
              )}
              {/* Usage Metrics */}
              {usage ? (
                <div className="grid grid-cols-2 gap-3">
                  <MetricCard
                    label="Total Requests"
                    value={formatNumber(usage.requests_total)}
                    trend={usage.trend_data?.slice(-7)} // Last 7 days
                    icon={<Activity className="h-4 w-4 text-blue-500" />}
                    color="#3b82f6"
                  />
                  
                  <MetricCard
                    label="Total Tokens"
                    value={formatNumber(usage.tokens_total)}
                    icon={<Zap className="h-4 w-4 text-purple-500" />}
                    color="#8b5cf6"
                  />
                  
                  <MetricCard
                    label="Total Cost"
                    value={formatCurrency(usage.cost_total)}
                    icon={<DollarSign className="h-4 w-4 text-green-500" />}
                    color="#10b981"
                  />
                  
                  <MetricCard
                    label="Avg Latency"
                    value={`${usage.avg_latency}ms`}
                    icon={<Clock className="h-4 w-4 text-orange-500" />}
                    color="#f59e0b"
                  />
                </div>
              ) : (
                <div className="flex items-center justify-center p-8 text-muted-foreground">
                  <div className="text-center">
                    <Activity className="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p className="text-sm">No usage data available</p>
                  </div>
                </div>
              )}

              {/* Usage Trend Chart */}
              {usage?.trend_data && usage.trend_data.length > 0 && (
                <div className="pt-2 border-t border-border/50">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-xs text-muted-foreground">Usage Trend (30 days)</span>
                    <span className="text-xs text-muted-foreground">
                      Total: {formatNumber(usage.requests_total)}
                    </span>
                  </div>
                  <SparklineChart 
                    data={usage.trend_data} 
                    type="area"
                    color={providerInfo.color.includes('emerald') ? '#10b981' : 
                           providerInfo.color.includes('orange') ? '#f59e0b' :
                           providerInfo.color.includes('blue') ? '#3b82f6' :
                           providerInfo.color.includes('indigo') ? '#6366f1' :
                           providerInfo.color.includes('red') ? '#ef4444' : '#3b82f6'}
                    className="h-16 w-full"
                  />
                </div>
              )}

              {/* Status Footer */}
              <div className="pt-2 border-t border-border/50">
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span className="flex items-center gap-1">
                    <div className={`w-2 h-2 rounded-full ${isActive ? 'bg-green-500' : 'bg-red-500'}`} />
                    {isActive ? 'Ready to serve' : 'Inactive'}
                  </span>
                  <span>
                    {new Date(model.created * 1000).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                </div>
                
                {usage?.last_used && (
                  <div className="mt-1 text-xs text-muted-foreground">
                    Last used: {new Date(usage.last_used).toLocaleDateString("en-US", {
                      month: "short", 
                      day: "numeric",
                      hour: "numeric",
                      minute: "2-digit",
                    })}
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}