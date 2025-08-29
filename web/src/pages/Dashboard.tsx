import { useQuery } from "@tanstack/react-query";
import { getModelStats, getModels, getBudgetSummary, getHistoricalModelLatencies } from "@/lib/api";
import api from "@/lib/api";
import type { StatsResponse, ModelsResponse } from "@/types/api";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import ReactECharts from "echarts-for-react";
import { Icon } from "@iconify/react";
import { useState, useEffect } from "react";
import { useAuth } from "@/contexts/OIDCAuthContext";
import { Link } from "react-router-dom";
import {
  Tooltip as UITooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Settings } from "lucide-react";
import { EmptyState } from "@/components/EmptyState";

export default function Dashboard() {
  const [isDark, setIsDark] = useState(false);
  const { user } = useAuth();
  
  // Check if user is admin based on groups or profile
  const isAdmin = (user?.profile?.groups && Array.isArray(user.profile.groups) && user.profile.groups.includes('admin')) || 
                  user?.profile?.role === 'admin' || 
                  user?.profile?.sub === 'master-key-user';

  useEffect(() => {
    const checkTheme = () => {
      setIsDark(document.documentElement.classList.contains("dark"));
    };
    checkTheme();

    const observer = new MutationObserver(checkTheme);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    return () => observer.disconnect();
  }, []);

  const { data: statsData, isLoading: statsLoading } = useQuery({
    queryKey: ["model-stats"],
    queryFn: getModelStats,
    refetchInterval: 5000, // Refresh every 5 seconds
  });

  const { data: modelsData } = useQuery({
    queryKey: ["models"],
    queryFn: getModels,
  });

  const { data: budgetData } = useQuery({
    queryKey: ["budget-summary"],
    queryFn: getBudgetSummary,
    enabled: !!isAdmin, // Only fetch budget data for admins
  });

  const { data: userBudgetData } = useQuery({
    queryKey: ["user-budget"],
    queryFn: () => api.userProfile.getBudgetStatus(),
    enabled: !isAdmin, // Only fetch for non-admin users
  });

  // Historical data queries - removed unused historicalHealthData

  const stats = statsData as StatsResponse;

  const { data: historicalLatencyData } = useQuery({
    queryKey: ["historical-model-latencies", Object.keys(stats?.load_balancer || {})],
    queryFn: () => {
      const models = Object.keys(stats?.load_balancer || {});
      return models.length > 0 ? getHistoricalModelLatencies(models, "hourly", 24) : null;
    },
    enabled: !!stats?.load_balancer && Object.keys(stats.load_balancer).length > 0,
    refetchInterval: 5 * 60 * 1000, // Refresh every 5 minutes
  });
  const models = (modelsData as ModelsResponse)?.data || [];
  const budget = budgetData as any; // axios interceptor extracts data
  const userBudget = userBudgetData as any; // axios interceptor extracts data

  // Calculate summary metrics
  const totalRequests = Object.values(stats?.load_balancer || {}).reduce(
    (sum: number, model: any) => sum + (model.total_requests || 0),
    0,
  );

  const activeModels = Object.values(stats?.load_balancer || {}).filter(
    (model: any) => !model.circuit_open,
  ).length;

  const avgHealthScore = Object.values(stats?.load_balancer || {}).reduce(
    (sum: number, model: any, _, arr) => sum + model.health_score / arr.length,
    0,
  );

  // Prepare chart data
  const modelHealthData = Object.entries(stats?.load_balancer || {}).map(
    ([name, data]: [string, any]) => ({
      name: name.replace("my-", ""),
      health: Math.round(data.health_score),
      requests: data.total_requests,
      latency: parseInt(data.avg_latency) || 0,
    }),
  );

  const pieData = modelHealthData.map((m) => ({
    name: m.name,
    value: m.requests,
  }));

  const COLORS = [
    "#3b82f6",
    "#10b981",
    "#f59e0b",
    "#ef4444",
    "#8b5cf6",
    "#06b6d4",
    "#84cc16",
    "#f97316",
  ];

  // Enhanced provider detection function
  const getProviderInfo = (modelName: string) => {
    const name = modelName?.toLowerCase() || "";

    if (name.includes("gpt") || name.includes("openai")) {
      return {
        icon: "logos:openai-icon",
        name: "OpenAI",
        color: "text-emerald-600 dark:text-emerald-400",
      };
    }
    if (name.includes("claude") || name.includes("anthropic")) {
      return {
        icon: "logos:anthropic",
        name: "Anthropic",
        color: "text-orange-600 dark:text-orange-400",
      };
    }
    if (name.includes("mistral")) {
      return {
        icon: "logos:mistral",
        name: "Mistral AI",
        color: "text-blue-600 dark:text-blue-400",
      };
    }
    if (name.includes("llama") || name.includes("meta")) {
      return {
        icon: "logos:meta",
        name: "Meta",
        color: "text-indigo-600 dark:text-indigo-400",
      };
    }
    if (name.includes("gemini") || name.includes("google")) {
      return {
        icon: "logos:google",
        name: "Google",
        color: "text-red-600 dark:text-red-400",
      };
    }
    if (name.includes("azure") || name.includes("microsoft")) {
      return {
        icon: "logos:microsoft",
        name: "Microsoft",
        color: "text-blue-700 dark:text-blue-300",
      };
    }
    if (name.includes("bedrock") || name.includes("aws")) {
      return {
        icon: "logos:aws",
        name: "AWS",
        color: "text-yellow-600 dark:text-yellow-400",
      };
    }

    return {
      icon: "lucide:brain",
      name: "Unknown",
      color: "text-muted-foreground",
    };
  };

  if (statsLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-3xl font-bold">Dashboard</h1>
          <p className="text-muted-foreground">
            {isAdmin ? 'Admin dashboard with system monitoring and management' : 'Real-time monitoring and analytics'}
          </p>
        </div>
        {isAdmin && (
          <div className="flex space-x-2">
            <Button asChild variant="outline" size="sm">
              <Link to="/settings">
                <Settings className="mr-2 h-4 w-4" />
                Settings
              </Link>
            </Button>
          </div>
        )}
      </div>

      {/* User Budget Summary Cards */}
      {!isAdmin && userBudget && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <Card className="transition-theme border-l-4 border-l-blue-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Personal Budget
              </CardTitle>
              <div className="h-8 w-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:wallet"
                  width="16"
                  height="16"
                  className="text-blue-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                ${userBudget.user?.max_budget?.toLocaleString() || '∞'}
              </div>
              <p className="text-xs text-muted-foreground">Monthly limit</p>
            </CardContent>
          </Card>

          <Card className="transition-theme border-l-4 border-l-orange-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Current Spend</CardTitle>
              <div className="h-8 w-8 rounded-lg bg-orange-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:trending-up"
                  width="16"
                  height="16"
                  className="text-orange-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                ${userBudget.user?.current_spend?.toLocaleString() || '0'}
              </div>
              <p className="text-xs text-muted-foreground">
                {userBudget.user?.max_budget > 0 
                  ? `${((userBudget.user?.current_spend / userBudget.user?.max_budget) * 100).toFixed(1)}% used`
                  : 'No limit set'
                }
              </p>
            </CardContent>
          </Card>

          <Card className="transition-theme border-l-4 border-l-purple-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Team Budgets
              </CardTitle>
              <div className="h-8 w-8 rounded-lg bg-purple-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:users"
                  width="16"
                  height="16"
                  className="text-purple-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                {userBudget.teams?.length || 0}
              </div>
              <p className="text-xs text-muted-foreground">
                Teams with budgets
              </p>
            </CardContent>
          </Card>

          <Card className="transition-theme border-l-4 border-l-emerald-500">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                API Keys
              </CardTitle>
              <div className="h-8 w-8 rounded-lg bg-emerald-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:key"
                  width="16"
                  height="16"
                  className="text-emerald-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                {userBudget.keys?.length || 0}
              </div>
              <p className="text-xs text-muted-foreground">
                Active keys
              </p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Admin Budget Summary Cards */}
      {isAdmin && budget?.summary && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <Link to="/ui/budget" className="group">
            <Card className="transition-theme border-l-4 border-l-purple-500 hover:shadow-lg hover:shadow-purple-500/10 cursor-pointer group-hover:scale-[1.02]">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">
                  Total Budget
                </CardTitle>
              <div className="h-8 w-8 rounded-lg bg-purple-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:wallet"
                  width="16"
                  height="16"
                  className="text-purple-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                ${budget?.summary.total_budget?.toLocaleString() || '0'}
              </div>
              <p className="text-xs text-muted-foreground">Across all entities</p>
            </CardContent>
            </Card>
          </Link>

          <Link to="/ui/budget" className="group">
            <Card className="transition-theme border-l-4 border-l-orange-500 hover:shadow-lg hover:shadow-orange-500/10 cursor-pointer group-hover:scale-[1.02]">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Total Spent</CardTitle>
              <div className="h-8 w-8 rounded-lg bg-orange-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:trending-up"
                  width="16"
                  height="16"
                  className="text-orange-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                ${budget?.summary.total_spent?.toLocaleString() || '0'}
              </div>
              <p className="text-xs text-muted-foreground">
                {budget?.summary.total_budget > 0 
                  ? `${((budget?.summary.total_spent / budget?.summary.total_budget) * 100).toFixed(1)}% used`
                  : 'No budget set'
                }
              </p>
            </CardContent>
          </Card>
          </Link>

          <Link to="/ui/budget" className="group">
            <Card className="transition-theme border-l-4 border-l-emerald-500 hover:shadow-lg hover:shadow-emerald-500/10 cursor-pointer group-hover:scale-[1.02]">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">
                  Budget Alerts
                </CardTitle>
                <div className="h-8 w-8 rounded-lg bg-emerald-500/10 flex items-center justify-center">
                  <Icon
                    icon="lucide:alert-triangle"
                    width="16"
                    height="16"
                    className="text-emerald-500"
                  />
                </div>
              </CardHeader>
              <CardContent>
                <div className="text-xl sm:text-2xl font-bold">
                  {(budget?.summary.alerting_count || 0) + (budget?.summary.exceeded_count || 0)}
                </div>
                <p className="text-xs text-muted-foreground">
                  {budget?.summary.exceeded_count || 0} exceeded, {budget?.summary.alerting_count || 0} warning
                </p>
              </CardContent>
            </Card>
          </Link>

          <Link to="/ui/budget" className="group">
            <Card className="transition-theme border-l-4 border-l-cyan-500 hover:shadow-lg hover:shadow-cyan-500/10 cursor-pointer group-hover:scale-[1.02]">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">
                  Budget Entities
                </CardTitle>
                <div className="h-8 w-8 rounded-lg bg-cyan-500/10 flex items-center justify-center">
                  <Icon
                    icon="lucide:database"
                    width="16"
                    height="16"
                    className="text-cyan-500"
                  />
                </div>
              </CardHeader>
              <CardContent>
                <div className="text-xl sm:text-2xl font-bold">
                  {budget?.summary.total_entities || 0}
                </div>
                <p className="text-xs text-muted-foreground">
                  Teams & API Keys
                </p>
              </CardContent>
            </Card>
          </Link>
        </div>
      )}

      {/* System Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <Link to="/ui/dashboard" className="group">
          <Card className="transition-theme border-l-4 border-l-blue-500 hover:shadow-lg hover:shadow-blue-500/10 cursor-pointer group-hover:scale-[1.02]">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Total Requests
              </CardTitle>
              <div className="h-8 w-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:activity"
                  width="16"
                  height="16"
                  className="text-blue-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                {totalRequests.toLocaleString()}
              </div>
              <p className="text-xs text-muted-foreground">Across all models</p>
            </CardContent>
          </Card>
        </Link>

        <Link to="/ui/models" className="group">
          <Card className="transition-theme border-l-4 border-l-green-500 hover:shadow-lg hover:shadow-green-500/10 cursor-pointer group-hover:scale-[1.02]">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Active Models</CardTitle>
              <div className="h-8 w-8 rounded-lg bg-green-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:brain"
                  width="16"
                  height="16"
                  className="text-green-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                {activeModels} / {models.length}
              </div>
              <p className="text-xs text-muted-foreground">Healthy and serving</p>
            </CardContent>
          </Card>
        </Link>

        <Link to="/ui/models" className="group">
          <Card className="transition-theme border-l-4 border-l-yellow-500 hover:shadow-lg hover:shadow-yellow-500/10 cursor-pointer group-hover:scale-[1.02]">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Avg Health Score
              </CardTitle>
              <div className="h-8 w-8 rounded-lg bg-yellow-500/10 flex items-center justify-center">
                <Icon
                  icon="lucide:zap"
                  width="16"
                  height="16"
                  className="text-yellow-500"
                />
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                {avgHealthScore.toFixed(1)}%
              </div>
              <p className="text-xs text-muted-foreground">System health</p>
            </CardContent>
          </Card>
        </Link>

        <Link to="/ui/settings" className="group">
          <Card
            className={`transition-theme border-l-4 cursor-pointer group-hover:scale-[1.02] ${
              stats?.should_shed_load ? "border-l-red-500 hover:shadow-lg hover:shadow-red-500/10" : "border-l-green-500 hover:shadow-lg hover:shadow-green-500/10"
            }`}
          >
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Load Shedding</CardTitle>
              <div
                className={`h-8 w-8 rounded-lg flex items-center justify-center ${
                  stats?.should_shed_load ? "bg-red-500/10" : "bg-green-500/10"
                }`}
              >
                {stats?.should_shed_load ? (
                  <Icon
                    icon="lucide:alert-circle"
                    width="16"
                    height="16"
                    className="text-red-500"
                  />
                ) : (
                  <Icon
                    icon="lucide:check-circle"
                    width="16"
                    height="16"
                    className="text-green-500"
                  />
                )}
              </div>
            </CardHeader>
            <CardContent>
              <div className="text-xl sm:text-2xl font-bold">
                {stats?.should_shed_load ? "Active" : "Inactive"}
              </div>
              <p className="text-xs text-muted-foreground">
                System protection status
              </p>
            </CardContent>
          </Card>
        </Link>
      </div>

      {/* TODO: Add admin quick actions later */}

      {/* Charts Row */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 lg:gap-6">
        {/* Model Health Overview - Summary Cards */}
        <Card className="transition-theme">
          <CardHeader>
            <CardTitle className="text-lg lg:text-xl">
              Model Health Overview
            </CardTitle>
            <CardDescription>Quick health status across all models</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px] sm:h-[300px]">
              {Object.keys(stats?.load_balancer || {}).length === 0 ? (
                <EmptyState
                  variant="chart"
                  icon="lucide:brain"
                  title="No models configured"
                  description="Add and configure models to see their health status here."
                  action={{
                    label: "Configure Models",
                    href: "/ui/models"
                  }}
                />
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 h-full overflow-y-auto">
                  {Object.entries(stats?.load_balancer || {}).map(([name, data]: [string, any]) => {
                    const providerInfo = getProviderInfo(name);
                    return (
                      <div key={name} className="bg-muted/30 rounded-lg p-4 hover:bg-muted/50 transition-colors">
                        <div className="flex items-center space-x-3 mb-2">
                          <Icon
                            icon={providerInfo.icon}
                            width="20"
                            height="20"
                            className={providerInfo.color}
                          />
                          <h3 className="font-medium truncate">{name}</h3>
                        </div>
                        <div className="space-y-2">
                          <div className="flex items-center justify-between">
                            <span className="text-sm text-muted-foreground">Health</span>
                            <div className="flex items-center space-x-2">
                              <div className="w-12 h-2 bg-muted rounded-full overflow-hidden">
                                <div
                                  className={`h-full transition-all duration-300 ${
                                    data.health_score >= 80 ? "bg-green-500" :
                                    data.health_score >= 60 ? "bg-yellow-500" : "bg-red-500"
                                  }`}
                                  style={{ width: `${data.health_score}%` }}
                                />
                              </div>
                              <span className="text-sm font-bold">{Math.round(data.health_score)}%</span>
                            </div>
                          </div>
                          <div className="flex items-center justify-between">
                            <span className="text-sm text-muted-foreground">Requests</span>
                            <span className="text-sm font-mono">{data.total_requests.toLocaleString()}</span>
                          </div>
                          <div className="flex items-center justify-between">
                            <span className="text-sm text-muted-foreground">Status</span>
                            <div className="flex items-center space-x-1">
                              <div className={`w-2 h-2 rounded-full ${data.circuit_open ? "bg-red-500" : "bg-green-500"}`} />
                              <span className="text-xs">{data.circuit_open ? "Unhealthy" : "Healthy"}</span>
                            </div>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Request Distribution */}
        <Card className="transition-theme">
          <CardHeader>
            <CardTitle className="text-lg lg:text-xl">
              Request Distribution
            </CardTitle>
            <CardDescription>Load balancing across models</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px] sm:h-[300px]">
              {pieData.every(item => item.value === 0) ? (
                <EmptyState
                  variant="chart"
                  icon="lucide:pie-chart"
                  title="No requests yet"
                  description="Start sending requests to see the distribution across your models. View our API documentation to get started."
                  action={{
                    label: "View API Documentation",
                    href: "/docs"
                  }}
                />
              ) : (
                <ReactECharts
                option={{
                  backgroundColor: "transparent",
                  tooltip: {
                    trigger: "item",
                    formatter: "{b}: {c} ({d}%)",
                    backgroundColor: isDark
                      ? "hsl(215, 27.9%, 16.9%)"
                      : "hsl(0, 0%, 100%)",
                    borderColor: isDark
                      ? "hsl(215, 27.9%, 16.9%)"
                      : "hsl(220, 13%, 91%)",
                    textStyle: {
                      color: isDark
                        ? "hsl(210, 20%, 98%)"
                        : "hsl(224, 71.4%, 4.1%)",
                    },
                  },
                  legend: {
                    orient: "vertical",
                    left: "left",
                    data: pieData.map((d) => d.name),
                    textStyle: {
                      color: isDark
                        ? "hsl(210, 20%, 98%)"
                        : "hsl(224, 71.4%, 4.1%)",
                      fontSize: 13, // Increased from 11 for better readability
                      fontWeight: "500",
                    },
                    itemWidth: 18, // Increased legend icon size
                    itemHeight: 14,
                  },
                  series: [
                    {
                      name: "Requests",
                      type: "pie",
                      radius: ["40%", "70%"],
                      center: ["60%", "50%"],
                      avoidLabelOverlap: true,
                      itemStyle: {
                        borderRadius: 10,
                        borderColor: isDark ? "#030712" : "#fff",
                        borderWidth: 2,
                      },
                      label: {
                        show: true,
                        formatter: "{d}%",
                        position: "outside",
                        color: isDark
                          ? "hsl(210, 20%, 98%)"
                          : "hsl(224, 71.4%, 4.1%)",
                        fontSize: 13, // Increased from 11 for better readability
                        fontWeight: "bold",
                      },
                      labelLine: {
                        show: true,
                        lineStyle: {
                          color: isDark
                            ? "hsl(215, 27.9%, 16.9%)"
                            : "hsl(220, 13%, 91%)",
                        },
                      },
                      emphasis: {
                        itemStyle: {
                          shadowBlur: 10,
                          shadowOffsetX: 0,
                          shadowColor: "rgba(0, 0, 0, 0.5)",
                        },
                        label: {
                          show: true,
                          fontSize: 16, // Increased from 14 for better accessibility
                          fontWeight: "bold",
                        },
                      },
                      data: pieData.map((item, index) => ({
                        name: item.name,
                        value: item.value,
                        itemStyle: {
                          color: COLORS[index % COLORS.length],
                        },
                      })),
                    },
                  ],
                }}
                style={{ height: "100%", width: "100%" }}
                opts={{ renderer: "svg" }}
              />
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* ECharts Real-time Metrics */}
      <Card className="transition-theme">
        <CardHeader>
          <CardTitle className="text-lg lg:text-xl">
            Real-time Latency
          </CardTitle>
          <CardDescription>Live performance monitoring</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-[250px] sm:h-[300px]">
            {!historicalLatencyData || Object.keys(historicalLatencyData || {}).length === 0 ? (
              <EmptyState
                variant="chart"
                icon="lucide:activity"
                title="No latency data available"
                description="Real-time latency monitoring will appear here once your models start processing requests."
                action={{
                  label: "Test API Endpoint",
                  href: "/ui/chat"
                }}
              />
            ) : (
              <ReactECharts
              option={{
                backgroundColor: "transparent",
                tooltip: {
                  trigger: "axis",
                  axisPointer: {
                    type: "cross",
                    lineStyle: {
                      color: isDark
                        ? "hsl(217.9, 10.6%, 64.9%)"
                        : "hsl(220, 8.9%, 46.1%)",
                    },
                  },
                  backgroundColor: isDark
                    ? "hsl(215, 27.9%, 16.9%)"
                    : "hsl(0, 0%, 100%)",
                  borderColor: isDark
                    ? "hsl(215, 27.9%, 16.9%)"
                    : "hsl(220, 13%, 91%)",
                  textStyle: {
                    color: isDark
                      ? "hsl(210, 20%, 98%)"
                      : "hsl(224, 71.4%, 4.1%)",
                  },
                },
                legend: {
                  data: modelHealthData.map((m) => m.name),
                  textStyle: {
                    color: isDark
                      ? "hsl(210, 20%, 98%)"
                      : "hsl(224, 71.4%, 4.1%)",
                    fontSize: 13, // Increased font size for better readability
                    fontWeight: "500",
                  },
                  padding: [10, 0],
                  itemWidth: 18, // Increased legend icon size
                  itemHeight: 14,
                },
                grid: {
                  left: "3%",
                  right: "4%",
                  bottom: "8%",
                  top: "15%",
                  containLabel: true,
                  borderColor: isDark
                    ? "hsl(215, 27.9%, 16.9%)"
                    : "hsl(220, 13%, 91%)",
                },
                xAxis: {
                  type: "category",
                  data: (() => {
                    if (!(historicalLatencyData as any)?.models) {
                      return ["00:00", "00:05", "00:10", "00:15", "00:20"];
                    }
                    
                    // Get timestamps from first model's data
                    const firstModel = Object.values((historicalLatencyData as any).models)[0] as any[];
                    return firstModel?.map(point => {
                      const date = new Date(point.timestamp);
                      return date.toLocaleTimeString('en-US', { 
                        hour: '2-digit', 
                        minute: '2-digit',
                        hour12: false 
                      });
                    }) || ["00:00", "00:05", "00:10", "00:15", "00:20"];
                  })(),
                  axisLine: {
                    lineStyle: {
                      color: isDark
                        ? "hsl(215, 27.9%, 16.9%)"
                        : "hsl(220, 13%, 91%)",
                    },
                  },
                  axisLabel: {
                    color: isDark
                      ? "hsl(217.9, 10.6%, 64.9%)"
                      : "hsl(220, 8.9%, 46.1%)",
                    fontSize: 12, // Increased font size for better readability
                    fontWeight: "500",
                  },
                  splitLine: {
                    lineStyle: {
                      color: isDark
                        ? "hsl(215, 27.9%, 16.9%)"
                        : "hsl(220, 13%, 91%)",
                      type: "dashed",
                    },
                  },
                },
                yAxis: {
                  type: "value",
                  name: "Latency (ms)",
                  nameTextStyle: {
                    color: isDark
                      ? "hsl(217.9, 10.6%, 64.9%)"
                      : "hsl(220, 8.9%, 46.1%)",
                    fontSize: 13, // Increased font size for Y-axis label
                    fontWeight: "600",
                  },
                  axisLine: {
                    lineStyle: {
                      color: isDark
                        ? "hsl(215, 27.9%, 16.9%)"
                        : "hsl(220, 13%, 91%)",
                    },
                  },
                  axisLabel: {
                    color: isDark
                      ? "hsl(217.9, 10.6%, 64.9%)"
                      : "hsl(220, 8.9%, 46.1%)",
                    fontSize: 12, // Increased font size for better readability
                    fontWeight: "500",
                  },
                  splitLine: {
                    lineStyle: {
                      color: isDark
                        ? "hsl(215, 27.9%, 16.9%)"
                        : "hsl(220, 13%, 91%)",
                      type: "dashed",
                    },
                  },
                },
                series: (() => {
                  if (!(historicalLatencyData as any)?.models) {
                    // Fallback to synthetic data
                    return modelHealthData.map((model, index) => ({
                      name: model.name,
                      type: "line",
                      smooth: true,
                      symbol: "circle",
                      symbolSize: 6,
                      data: Array.from(
                        { length: 5 },
                        () => Math.floor(Math.random() * 200) + model.latency,
                      ),
                      lineStyle: {
                        color: COLORS[index % COLORS.length],
                        width: 3,
                      },
                      itemStyle: {
                        color: COLORS[index % COLORS.length],
                      },
                      areaStyle: {
                        color: {
                          type: "linear",
                          x: 0,
                          y: 0,
                          x2: 0,
                          y2: 1,
                          colorStops: [
                            {
                              offset: 0,
                              color: COLORS[index % COLORS.length] + "20",
                            },
                            {
                              offset: 1,
                              color: COLORS[index % COLORS.length] + "00",
                            },
                          ],
                        },
                      },
                    }));
                  }
                  
                  // Use real historical latency data
                  return Object.entries((historicalLatencyData as any).models).map(([modelName, modelData], index) => ({
                    name: modelName.replace("my-", ""),
                    type: "line",
                    smooth: true,
                    symbol: "circle",
                    symbolSize: 6,
                    data: (modelData as any[]).map(point => point.avg_latency || 0),
                    lineStyle: {
                      color: COLORS[index % COLORS.length],
                      width: 3,
                    },
                    itemStyle: {
                      color: COLORS[index % COLORS.length],
                    },
                    areaStyle: {
                      color: {
                        type: "linear",
                        x: 0,
                        y: 0,
                        x2: 0,
                        y2: 1,
                        colorStops: [
                          {
                            offset: 0,
                            color: COLORS[index % COLORS.length] + "20",
                          },
                          {
                            offset: 1,
                            color: COLORS[index % COLORS.length] + "00",
                          },
                        ],
                      },
                    },
                  }));
                })(),
              }}
              style={{ height: "100%", width: "100%" }}
              opts={{ renderer: "svg" }}
            />
            )}
          </div>
        </CardContent>
      </Card>

      {/* TODO: Add admin recent activity later */}

      {/* Model Status Table - Full Width */}
      <Card className="transition-theme">
        <CardHeader>
          <CardTitle className="text-lg lg:text-xl">Model Status</CardTitle>
          <CardDescription>Detailed model performance metrics</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <div className="min-w-[900px]">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border">
                    <th className="text-left p-3 font-semibold min-w-[140px]">
                      Model
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[90px]">
                      Provider
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[100px]">
                      Status
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[140px]">
                      Health Score
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[90px]">
                      Requests
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[80px]">
                      Failed
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[100px]">
                      Avg Latency
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[100px]">
                      P95 Latency
                    </th>
                    <th className="text-left p-3 font-semibold min-w-[90px]">
                      Circuit
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <TooltipProvider>
                    {Object.entries(stats?.load_balancer || {}).map(
                      ([name, data]: [string, any]) => {
                        const providerInfo = getProviderInfo(name);

                        return (
                          <tr
                            key={name}
                            className="border-b border-border/50 hover:bg-muted/30 transition-colors duration-200"
                          >
                            <td className="p-3 font-medium">
                              <div className="flex items-center space-x-3">
                                <div className="flex-shrink-0">
                                  <Icon
                                    icon={providerInfo.icon}
                                    width="24"
                                    height="24"
                                    className={providerInfo.color}
                                  />
                                </div>
                                <span className="truncate font-medium">
                                  {name}
                                </span>
                              </div>
                            </td>
                            <td className="p-3">
                              <UITooltip>
                                <TooltipTrigger asChild>
                                  <div className="flex items-center justify-center w-10 h-8 rounded-lg bg-muted/50 hover:bg-muted transition-colors cursor-help">
                                    <Icon
                                      icon={providerInfo.icon}
                                      width="20"
                                      height="20"
                                      className={providerInfo.color}
                                    />
                                  </div>
                                </TooltipTrigger>
                                <TooltipContent>
                                  <p className="font-medium">
                                    {providerInfo.name}
                                  </p>
                                </TooltipContent>
                              </UITooltip>
                            </td>
                            <td className="p-3">
                              <div className="flex items-center space-x-2">
                                <div
                                  className={`w-2 h-2 rounded-full ${
                                    data.circuit_open
                                      ? "bg-red-500 dark:bg-red-400"
                                      : "bg-green-500 dark:bg-green-400"
                                  }`}
                                />
                                <span
                                  className={`text-sm font-medium ${
                                    data.circuit_open
                                      ? "text-red-600 dark:text-red-400"
                                      : "text-green-600 dark:text-green-400"
                                  }`}
                                >
                                  {data.circuit_open ? "Unhealthy" : "Healthy"}
                                </span>
                              </div>
                            </td>
                            <td className="p-3">
                              <div className="flex items-center space-x-3">
                                <div className="flex-1 max-w-20">
                                  <div className="w-full bg-muted dark:bg-muted/50 rounded-full h-2.5 overflow-hidden">
                                    <div
                                      className={`h-2.5 rounded-full transition-all duration-500 ${
                                        data.health_score >= 80
                                          ? "bg-green-500 dark:bg-green-400"
                                          : data.health_score >= 60
                                            ? "bg-yellow-500 dark:bg-yellow-400"
                                            : "bg-red-500 dark:bg-red-400"
                                      }`}
                                      style={{ width: `${data.health_score}%` }}
                                    />
                                  </div>
                                </div>
                                <span className="text-sm font-bold min-w-[3ch] text-right">
                                  {Math.round(data.health_score)}%
                                </span>
                              </div>
                            </td>
                            <td className="p-3">
                              <span className="font-mono text-sm font-medium">
                                {data.total_requests.toLocaleString()}
                              </span>
                            </td>
                            <td className="p-3">
                              <span
                                className={`font-mono text-sm font-medium ${
                                  data.failed_requests > 0
                                    ? "text-red-600 dark:text-red-400 font-bold"
                                    : "text-muted-foreground"
                                }`}
                              >
                                {data.failed_requests}
                              </span>
                            </td>
                            <td className="p-3">
                              <span className="font-mono text-sm">
                                {data.avg_latency
                                  ? `${data.avg_latency}ms`
                                  : "N/A"}
                              </span>
                            </td>
                            <td className="p-3">
                              <span className="font-mono text-sm">
                                {data.p95_latency
                                  ? `${data.p95_latency}ms`
                                  : "N/A"}
                              </span>
                            </td>
                            <td className="p-3">
                              <span
                                className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium border ${
                                  data.circuit_open
                                    ? "bg-red-50 dark:bg-red-950/30 text-red-700 dark:text-red-400 border-red-200 dark:border-red-800"
                                    : "bg-green-50 dark:bg-green-950/30 text-green-700 dark:text-green-400 border-green-200 dark:border-green-800"
                                }`}
                              >
                                {data.circuit_open ? "Open" : "Closed"}
                              </span>
                            </td>
                          </tr>
                        );
                      },
                    )}
                  </TooltipProvider>
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
