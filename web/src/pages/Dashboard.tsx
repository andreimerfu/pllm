import * as React from "react"
import { useNavigate } from "react-router-dom"
import { Area, AreaChart, CartesianGrid, XAxis, Line, LineChart, ResponsiveContainer } from "recharts"
import { useQuery } from "@tanstack/react-query"
import { Icon } from "@iconify/react"
import { icons } from "@/lib/icons"
import { getProviderLogo } from "@/lib/provider-logos"
import { detectProvider } from "@/lib/providers"
import { getDashboardMetrics, getUsageTrends, getAdminModels, getModelsHealth } from "@/lib/api"
import type { AdminModelsResponse, ModelsHealthResponse } from "@/types/api"
import { formatChartLabel, fillTimeGaps } from "@/lib/date-utils"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

// ─── Mini Sparkline ───────────────────────────────────────────────────────────

function Sparkline({ data, color = "#14B8A6" }: { data: number[]; color?: string }) {
  const chartData = data.map((v, i) => ({ i, v }))
  return (
    <ResponsiveContainer width="100%" height={32}>
      <LineChart data={chartData} margin={{ top: 2, right: 2, bottom: 2, left: 2 }}>
        <Line
          type="monotone"
          dataKey="v"
          stroke={color}
          strokeWidth={1.5}
          dot={false}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}

// ─── Trend Indicator ──────────────────────────────────────────────────────────

function TrendIndicator({ value }: { value: number | null }) {
  if (value === null || value === undefined) return null
  const isPositive = value >= 0
  return (
    <span className={`inline-flex items-center gap-0.5 text-xs font-medium ${isPositive ? "text-emerald-500" : "text-red-400"}`}>
      <Icon icon={isPositive ? icons.arrowUp : icons.arrowDown} className="h-3 w-3" />
      {Math.abs(value).toFixed(1)}%
    </span>
  )
}

// ─── Hero Metric Card ─────────────────────────────────────────────────────────

function HeroCard({
  icon,
  label,
  value,
  trend,
  sparkData,
  onClick,
}: {
  icon: string
  label: string
  value: string
  trend: number | null
  sparkData: number[]
  onClick?: () => void
}) {
  return (
    <Card
      className={`bg-card border rounded-lg p-4 transition-colors ${onClick ? "cursor-pointer hover:border-teal-500/40" : ""}`}
      onClick={onClick}
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-teal-500/10">
          <Icon icon={icon} className="h-4 w-4 text-teal-500" />
        </div>
        <TrendIndicator value={trend} />
      </div>
      <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-1">
        {label}
      </p>
      <p className="text-2xl font-bold font-mono leading-none mb-2">{value}</p>
      {sparkData.length > 1 && (
        <div className="mt-1">
          <Sparkline data={sparkData} />
        </div>
      )}
    </Card>
  )
}

// ─── Hero Metrics Row ─────────────────────────────────────────────────────────

function HeroMetrics() {
  const navigate = useNavigate()

  const { data: rawData, isLoading: metricsLoading } = useQuery({
    queryKey: ["dashboard-metrics"],
    queryFn: getDashboardMetrics,
    refetchInterval: 60000,
  })

  const { data: rawTrends7d } = useQuery({
    queryKey: ["usage-trends", { days: 7, interval: "daily" }],
    queryFn: () => getUsageTrends({ days: 7, interval: "daily" }),
    refetchInterval: 60000,
  })

  const { data: adminModelsData } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  })

  const metrics = React.useMemo(() => {
    if (!rawData) return null
    const data = (rawData as any).data || rawData
    return {
      totalRequests: data.total_requests || 0,
      totalTokens: data.total_tokens || 0,
      totalCost: data.total_cost || 0,
      activeKeys: data.active_keys || 0,
      activeModels: data.active_models || 0,
      topModels: data.top_models || [],
    }
  }, [rawData])

  const sparkRequests = React.useMemo(() => {
    if (!rawTrends7d) return []
    const raw = (rawTrends7d as any).data || rawTrends7d || []
    return (raw as any[]).map((d: any) => d.requests || 0)
  }, [rawTrends7d])

  const sparkTokens = React.useMemo(() => {
    if (!rawTrends7d) return []
    const raw = (rawTrends7d as any).data || rawTrends7d || []
    return (raw as any[]).map((d: any) => d.tokens || 0)
  }, [rawTrends7d])

  // Compute simple trend percentage (last vs first half average)
  const computeTrend = (arr: number[]): number | null => {
    if (arr.length < 4) return null
    const mid = Math.floor(arr.length / 2)
    const first = arr.slice(0, mid).reduce((a, b) => a + b, 0) / mid
    const second = arr.slice(mid).reduce((a, b) => a + b, 0) / (arr.length - mid)
    if (first === 0) return second > 0 ? 100 : 0
    return ((second - first) / first) * 100
  }

  const activeModelCount = React.useMemo(() => {
    const adminResponse = adminModelsData as AdminModelsResponse | undefined
    const models = adminResponse?.models || []
    return models.filter(m => m.enabled).length || metrics?.activeModels || 0
  }, [adminModelsData, metrics])

  // Compute error rate from top_models if available
  const errorRate = React.useMemo(() => {
    if (!metrics) return 0
    // If we have top_models with error info, compute; otherwise show 0
    return 0
  }, [metrics])

  if (metricsLoading || !metrics) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map((i) => (
          <Card key={i} className="bg-card border rounded-lg p-4">
            <div className="h-8 w-8 bg-muted animate-pulse rounded-md mb-3" />
            <div className="h-3 w-20 bg-muted animate-pulse rounded mb-2" />
            <div className="h-7 w-24 bg-muted animate-pulse rounded mb-2" />
            <div className="h-8 w-full bg-muted animate-pulse rounded" />
          </Card>
        ))}
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <HeroCard
        icon={icons.trendingUp}
        label="Total Requests (24h)"
        value={metrics.totalRequests.toLocaleString()}
        trend={computeTrend(sparkRequests)}
        sparkData={sparkRequests}
      />
      <HeroCard
        icon={icons.bolt}
        label="Tokens Used"
        value={
          metrics.totalTokens > 1000000
            ? `${(metrics.totalTokens / 1000000).toFixed(1)}M`
            : metrics.totalTokens.toLocaleString()
        }
        trend={computeTrend(sparkTokens)}
        sparkData={sparkTokens}
      />
      <HeroCard
        icon={icons.models}
        label="Active Models"
        value={String(activeModelCount)}
        trend={null}
        sparkData={[]}
        onClick={() => navigate("/models")}
      />
      <HeroCard
        icon={icons.warning}
        label="Error Rate"
        value={`${errorRate.toFixed(2)}%`}
        trend={null}
        sparkData={[]}
      />
    </div>
  )
}

// ─── Request Volume Chart ─────────────────────────────────────────────────────

const chartConfig = {
  requests: {
    label: "Requests",
    color: "#14B8A6",
  },
  tokens: {
    label: "Tokens (100s)",
    color: "#0D9488",
  },
} satisfies ChartConfig

function RequestVolumeChart() {
  const [timeRange, setTimeRange] = React.useState<"24h" | "7d" | "30d">("7d")

  const trendParams = React.useMemo(() => {
    switch (timeRange) {
      case "24h":
        return { hours: 24, interval: "hourly" }
      case "7d":
        return { days: 7, interval: "daily" }
      case "30d":
      default:
        return { days: 30, interval: "daily" }
    }
  }, [timeRange])

  const currentInterval = (trendParams.interval || "daily") as "hourly" | "daily"

  const { data: rawTrends, isLoading: loading } = useQuery({
    queryKey: ["usage-trends", trendParams],
    queryFn: () => getUsageTrends(trendParams),
    refetchInterval: 60000,
  })

  const chartData = React.useMemo(() => {
    if (!rawTrends) return []
    const raw = (rawTrends as any).data || rawTrends
    const data = raw || []
    const range = trendParams.hours || trendParams.days || 30
    return fillTimeGaps(data, currentInterval, range)
  }, [rawTrends, currentInterval, trendParams])

  const filteredData = chartData.map((item: any) => ({
    date: item.date,
    requests: item.requests || 0,
    tokens: item.tokens ? Math.floor(item.tokens / 100) : 0,
  }))

  const tabs: { value: "24h" | "7d" | "30d"; label: string }[] = [
    { value: "24h", label: "24h" },
    { value: "7d", label: "7d" },
    { value: "30d", label: "30d" },
  ]

  return (
    <Card className="pt-0 flex-1 min-w-0">
      <CardHeader className="flex items-center gap-2 space-y-0 border-b py-4 sm:flex-row">
        <CardTitle className="flex-1 text-base">Request Volume</CardTitle>
        <div className="flex items-center gap-1 rounded-lg border p-0.5">
          {tabs.map((tab) => (
            <button
              key={tab.value}
              onClick={() => setTimeRange(tab.value)}
              className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                timeRange === tab.value
                  ? "bg-teal-500/15 text-teal-400"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </CardHeader>
      <CardContent className="overflow-hidden px-2 pt-4 sm:px-6 sm:pt-6">
        {loading ? (
          <div className="aspect-auto h-[280px] w-full flex items-center justify-center">
            <div className="flex items-center gap-2">
              <Icon icon={icons.refresh} className="h-4 w-4 animate-spin" />
              <span className="text-muted-foreground text-sm">Loading chart data...</span>
            </div>
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            className="aspect-auto h-[280px] w-full"
          >
            <AreaChart data={filteredData} margin={{ top: 10, right: 10, bottom: 0, left: 0 }}>
              <defs>
                <linearGradient id="fillRequests" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#14B8A6" stopOpacity={0.4} />
                  <stop offset="95%" stopColor="#14B8A6" stopOpacity={0.02} />
                </linearGradient>
                <linearGradient id="fillTokens" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#0D9488" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#0D9488" stopOpacity={0.02} />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis
                dataKey="date"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={32}
                tickFormatter={(value) => formatChartLabel(value, currentInterval)}
                className="text-xs"
              />
              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    labelFormatter={(value) => formatChartLabel(value, currentInterval)}
                    indicator="dot"
                  />
                }
              />
              <Area
                dataKey="tokens"
                type="monotone"
                fill="url(#fillTokens)"
                stroke="#0D9488"
                strokeWidth={1.5}
                stackId="a"
              />
              <Area
                dataKey="requests"
                type="monotone"
                fill="url(#fillRequests)"
                stroke="#14B8A6"
                strokeWidth={1.5}
                stackId="a"
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}

// ─── Provider Breakdown Panel ─────────────────────────────────────────────────

function ProviderBreakdown() {
  const { data: rawData, isLoading: metricsLoading } = useQuery({
    queryKey: ["dashboard-metrics"],
    queryFn: getDashboardMetrics,
    refetchInterval: 60000,
  })

  const { data: adminModelsData, isLoading: modelsLoading } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  })

  const providers = React.useMemo(() => {
    const adminResponse = adminModelsData as AdminModelsResponse | undefined
    const adminModels = adminResponse?.models || []
    const dashboard = rawData ? ((rawData as any).data || rawData) : null
    const topModels: any[] = dashboard?.top_models || []

    // Build usage map
    const usageMap = new Map<string, number>()
    for (const m of topModels) {
      if (m.model) {
        usageMap.set(m.model, m.requests || 0)
      }
    }

    // Aggregate by provider
    const providerMap = new Map<string, number>()
    for (const am of adminModels) {
      const provider = am.provider?.type || "unknown"
      const requests = usageMap.get(am.model_name) || 0
      providerMap.set(provider, (providerMap.get(provider) || 0) + requests)
    }

    // Also count providers with no requests (so they still appear)
    for (const am of adminModels) {
      const provider = am.provider?.type || "unknown"
      if (!providerMap.has(provider)) {
        providerMap.set(provider, 0)
      }
    }

    const entries = Array.from(providerMap.entries())
      .map(([name, requests]) => ({ name, requests }))
      .sort((a, b) => b.requests - a.requests)

    const total = entries.reduce((sum, e) => sum + e.requests, 0)
    return entries.map(e => ({
      ...e,
      percentage: total > 0 ? (e.requests / total) * 100 : 0,
    }))
  }, [rawData, adminModelsData])

  const loading = metricsLoading || modelsLoading

  return (
    <Card className="pt-0 w-full lg:w-[340px] xl:w-[380px] flex-shrink-0">
      <CardHeader className="border-b py-4">
        <CardTitle className="text-base">Provider Breakdown</CardTitle>
      </CardHeader>
      <CardContent className="pt-4">
        {loading ? (
          <div className="space-y-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="flex items-center gap-3">
                <div className="h-6 w-6 bg-muted animate-pulse rounded" />
                <div className="flex-1">
                  <div className="h-3 w-20 bg-muted animate-pulse rounded mb-2" />
                  <div className="h-2 w-full bg-muted animate-pulse rounded" />
                </div>
              </div>
            ))}
          </div>
        ) : providers.length === 0 ? (
          <p className="text-sm text-muted-foreground py-8 text-center">No providers configured</p>
        ) : (
          <div className="space-y-4">
            {providers.map((provider) => (
              <div key={provider.name} className="flex items-center gap-3">
                <div className="flex h-7 w-7 items-center justify-center rounded-md bg-muted/50 flex-shrink-0">
                  <Icon icon={getProviderLogo(provider.name)} width={16} height={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium truncate capitalize">{provider.name}</span>
                    <span className="text-xs font-mono text-muted-foreground ml-2">
                      {provider.requests.toLocaleString()}
                    </span>
                  </div>
                  <div className="h-1.5 w-full rounded-full bg-muted/50 overflow-hidden">
                    <div
                      className="h-full rounded-full bg-teal-500 transition-all"
                      style={{ width: `${Math.max(provider.percentage, 2)}%` }}
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ─── System Health Strip ──────────────────────────────────────────────────────

function SystemHealthStrip() {
  const { data: healthData, isLoading } = useQuery({
    queryKey: ["models-health"],
    queryFn: getModelsHealth,
    refetchInterval: 30000,
  })

  const summary = React.useMemo(() => {
    const response = healthData as ModelsHealthResponse | undefined
    if (!response?.models) return { healthy: 0, degraded: 0, down: 0 }
    const models = Object.values(response.models)
    let healthy = 0, degraded = 0, down = 0
    for (const m of models) {
      if (!m.healthy) {
        down++
      } else if (m.healthy_count < m.total_count) {
        degraded++
      } else {
        healthy++
      }
    }
    return { healthy, degraded, down }
  }, [healthData])

  if (isLoading) return null

  const total = summary.healthy + summary.degraded + summary.down
  if (total === 0) return null

  return (
    <div className="flex items-center gap-4 px-4 py-2 rounded-lg border bg-card text-sm">
      <span className="text-muted-foreground text-xs font-medium uppercase tracking-wide">System Health</span>
      <div className="flex items-center gap-4 ml-auto">
        {summary.healthy > 0 && (
          <span className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-emerald-500" />
            <span className="font-mono text-xs">{summary.healthy}</span>
            <span className="text-muted-foreground text-xs">healthy</span>
          </span>
        )}
        {summary.degraded > 0 && (
          <span className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-amber-500" />
            <span className="font-mono text-xs">{summary.degraded}</span>
            <span className="text-muted-foreground text-xs">degraded</span>
          </span>
        )}
        {summary.down > 0 && (
          <span className="flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-red-500" />
            <span className="font-mono text-xs">{summary.down}</span>
            <span className="text-muted-foreground text-xs">down</span>
          </span>
        )}
      </div>
    </div>
  )
}

// ─── Models Activity Table ────────────────────────────────────────────────────

function ModelsActivityTable() {
  const navigate = useNavigate()

  const { data: rawData, isLoading: metricsLoading } = useQuery({
    queryKey: ["dashboard-metrics"],
    queryFn: getDashboardMetrics,
    refetchInterval: 60000,
  })

  const { data: adminModelsData, isLoading: modelsLoading } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  })

  const loading = metricsLoading || modelsLoading

  const models = React.useMemo(() => {
    const adminResponse = adminModelsData as AdminModelsResponse | undefined
    const adminModels = adminResponse?.models || []
    if (adminModels.length === 0) return []

    const dashboard = rawData ? ((rawData as any).data || rawData) : null
    const topModels: any[] = dashboard?.top_models || []

    const usageMap = new Map<string, { requests: number; cost: number; latency: number }>()
    for (const m of topModels) {
      if (m.model) {
        usageMap.set(m.model, {
          requests: m.requests || 0,
          cost: m.cost || 0,
          latency: m.avg_latency || 0,
        })
      }
    }

    return adminModels
      .map((am) => {
        const usage = usageMap.get(am.model_name)
        return {
          id: am.id,
          name: am.model_name,
          provider: am.provider?.type || "Unknown",
          enabled: am.enabled,
          requests: usage?.requests || 0,
          cost: usage?.cost || 0,
          latency: usage?.latency || 0,
        }
      })
      .sort((a, b) => b.requests - a.requests)
      .slice(0, 8)
  }, [rawData, adminModelsData])

  return (
    <Card className="pt-0">
      <CardHeader className="flex items-center gap-2 space-y-0 border-b py-4 sm:flex-row">
        <CardTitle className="flex-1 text-base">Models Activity</CardTitle>
        <Button
          variant="ghost"
          size="sm"
          className="text-teal-500 hover:text-teal-400 text-xs"
          onClick={() => navigate("/models")}
        >
          View All Models
          <Icon icon={icons.arrowRight} className="ml-1 h-3 w-3" />
        </Button>
      </CardHeader>
      <CardContent className="p-0">
        {loading ? (
          <div className="p-6 space-y-3">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="flex items-center gap-4">
                <div className="h-4 w-4 bg-muted animate-pulse rounded" />
                <div className="h-4 w-28 bg-muted animate-pulse rounded" />
                <div className="flex-1" />
                <div className="h-4 w-12 bg-muted animate-pulse rounded" />
                <div className="h-4 w-16 bg-muted animate-pulse rounded" />
                <div className="h-4 w-14 bg-muted animate-pulse rounded" />
              </div>
            ))}
          </div>
        ) : models.length === 0 ? (
          <div className="py-12 text-center text-muted-foreground text-sm">
            No models configured yet.
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Model</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Latency</TableHead>
                <TableHead className="text-right">Requests</TableHead>
                <TableHead className="text-right">Cost</TableHead>
                <TableHead className="w-[40px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {models.map((model) => (
                <TableRow key={model.id} className="group">
                  <TableCell>
                    <div className="flex items-center gap-2.5">
                      <Icon icon={detectProvider(model.name || model.id, model.provider).icon} width={18} height={18} />
                      <div>
                        <div className="font-medium text-sm">{model.name}</div>
                        <div className="text-[11px] text-muted-foreground capitalize">{model.provider}</div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1.5">
                      <span className={`h-1.5 w-1.5 rounded-full ${model.enabled ? "bg-emerald-500" : "bg-zinc-500"}`} />
                      <span className="text-xs text-muted-foreground">{model.enabled ? "Active" : "Inactive"}</span>
                    </div>
                  </TableCell>
                  <TableCell className="text-right font-mono text-sm">
                    {model.latency > 0 ? `${model.latency}ms` : "--"}
                  </TableCell>
                  <TableCell className="text-right font-mono text-sm">
                    {model.requests.toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right font-mono text-sm">
                    ${model.cost.toFixed(2)}
                  </TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button
                          variant="ghost"
                          className="h-7 w-7 p-0 opacity-0 group-hover:opacity-100 transition-opacity"
                        >
                          <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
                          <span className="sr-only">Open menu</span>
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => navigate(`/models/${model.id}`)}>
                          View Details
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => navigate("/models")}>
                          Edit
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

// ─── Main Dashboard ───────────────────────────────────────────────────────────

export default function Dashboard() {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Dashboard</h2>
        <p className="text-[13px] text-muted-foreground mt-1">
          Monitor your LLM gateway performance and usage
        </p>
      </div>

      {/* System Health Strip */}
      <SystemHealthStrip />

      {/* Hero Metrics Row */}
      <HeroMetrics />

      {/* Two-Column: Chart + Provider Breakdown */}
      <div className="flex flex-col lg:flex-row gap-6">
        <RequestVolumeChart />
        <ProviderBreakdown />
      </div>

      {/* Models Activity Table */}
      <ModelsActivityTable />
    </div>
  )
}
