import * as React from "react"
import { useNavigate } from "react-router-dom"
import { Area, AreaChart, CartesianGrid, XAxis, ResponsiveContainer, PieChart, Pie, Cell } from "recharts"
import { useQuery } from "@tanstack/react-query"
import { formatDistanceToNow } from "date-fns"
import { Icon } from "@iconify/react"
import { icons } from "@/lib/icons"
import { getProviderLogo } from "@/lib/provider-logos"
import { detectProvider } from "@/lib/providers"
import {
  getDashboardMetrics,
  getUsageTrends,
  getAdminModels,
  getModelsHealth,
  getAuditLogs,
  listDeployments,
  listRegistryServers,
  listRegistryAgents,
  listRegistrySkills,
  listRegistryPrompts,
} from "@/lib/api"
import { PageShell, StatTile } from "@/components/common/page-shell"
import type { AdminModelsResponse, ModelsHealthResponse, AuditLog } from "@/types/api"
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

// ─── Time range model ─────────────────────────────────────────────────────────
//
// Shared by every dashboard widget. Three values matching the FE pill and
// the backend allow-list in dashboard.go (parseDashboardTimeRange).
type TimeRange = "24h" | "7d" | "30d"

// trendParams maps a TimeRange to the (days/hours, interval) tuple every
// trends-style API call accepts. Single source of truth — both the
// sparkline queries in HeroMetrics and the area chart in
// RequestVolumeChart go through this so the cache keys line up.
function trendParams(range: TimeRange): { hours?: number; days?: number; interval: "hourly" | "daily" } {
  switch (range) {
    case "24h": return { hours: 24, interval: "hourly" }
    case "7d":  return { days: 7,  interval: "daily" }
    case "30d": return { days: 30, interval: "daily" }
  }
}

// metricsParams maps a TimeRange to the (hours XOR days) tuple
// getDashboardMetrics accepts. No interval — the snapshot endpoint
// only cares about the window.
function metricsParams(range: TimeRange): { hours?: number; days?: number } {
  switch (range) {
    case "24h": return { hours: 24 }
    case "7d":  return { days: 7 }
    case "30d": return { days: 30 }
  }
}

// rangeLabel is the short label used in hero card titles ("Requests (24h)").
function rangeLabel(range: TimeRange): string {
  return range
}

// ─── Time range pill ──────────────────────────────────────────────────────────

function TimeRangePill({
  value,
  onChange,
}: {
  value: TimeRange
  onChange: (v: TimeRange) => void
}) {
  const tabs: { value: TimeRange; label: string }[] = [
    { value: "24h", label: "24h" },
    { value: "7d", label: "7d" },
    { value: "30d", label: "30d" },
  ]
  return (
    <div className="flex items-center gap-0.5 rounded-[9px] border border-border bg-surface-2 p-[3px]">
      {tabs.map((tab) => (
        <button
          key={tab.value}
          onClick={() => onChange(tab.value)}
          className={`h-[26px] rounded-md px-3 text-xs font-semibold transition-colors ${
            value === tab.value
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {tab.label}
        </button>
      ))}
    </div>
  )
}

// ─── Hero Metrics Row ─────────────────────────────────────────────────────────

function fmtCompact(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function HeroMetrics({ range }: { range: TimeRange }) {
  const mp = metricsParams(range)
  const tp = trendParams(range)

  const { data: rawData, isLoading: metricsLoading } = useQuery({
    queryKey: ["dashboard-metrics", mp],
    queryFn: () => getDashboardMetrics(mp),
    refetchInterval: 60000,
  })

  const { data: rawTrends } = useQuery({
    queryKey: ["usage-trends", tp],
    queryFn: () => getUsageTrends(tp),
    refetchInterval: 60000,
  })

  const { data: adminModelsData } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  })

  // Registry composition + deployments — the registry-first KPIs the
  // "pLLM Console" dashboard leads with. Counts come straight from the
  // list endpoints (latest-only), never fabricated.
  const { data: registryCounts } = useRegistryCounts()

  const { data: deploymentsData } = useQuery({
    queryKey: ["deployments"],
    queryFn: listDeployments,
    refetchInterval: 10000,
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
    if (!rawTrends) return []
    const raw = (rawTrends as any).data || rawTrends || []
    return (raw as any[]).map((d: any) => d.requests || 0)
  }, [rawTrends])

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

  if (metricsLoading || !metrics) {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {[1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="rounded-[14px] border border-border bg-card p-4 shadow-sm">
            <div className="mb-3.5 h-[34px] w-[34px] animate-pulse rounded-[9px] bg-muted" />
            <div className="mb-2 h-7 w-24 animate-pulse rounded bg-muted" />
            <div className="h-3 w-20 animate-pulse rounded bg-muted" />
          </div>
        ))}
      </div>
    )
  }

  const c = registryCounts
  const registeredAssets = c ? c.servers + c.agents + c.skills + c.prompts : null
  const deployments = deploymentsData?.deployments ?? []
  const running = deployments.filter((d) => d.status === "running").length
  const reqTrend = computeTrend(sparkRequests)

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
      <StatTile
        label="Registered assets"
        value={registeredAssets ?? "—"}
        icon={<Icon icon={icons.registry} />}
        iconColor="agent"
        sub={c ? `${c.agents} agents · ${c.servers} MCP · ${c.skills + c.prompts} others` : "Across the catalog"}
        className="cursor-pointer"
      />
      <StatTile
        label="Live deployments"
        value={deployments.length}
        icon={<Icon icon={icons.rocket} />}
        iconColor="mcp"
        sub={`${running} running`}
      />
      <StatTile
        label={`Requests (${rangeLabel(range)})`}
        value={fmtCompact(metrics.totalRequests)}
        icon={<Icon icon={icons.trendingUp} />}
        iconColor="primary"
        delta={reqTrend !== null ? `${reqTrend >= 0 ? "+" : ""}${reqTrend.toFixed(0)}%` : undefined}
        deltaPositive={(reqTrend ?? 0) >= 0}
        sub="Routed through the gateway"
      />
      <StatTile
        label="Active models"
        value={activeModelCount}
        icon={<Icon icon={icons.models} />}
        iconColor="model"
        sub={`${metrics.activeKeys} active keys`}
      />
      <StatTile
        label={`Spend (${rangeLabel(range)})`}
        value={`$${metrics.totalCost.toFixed(2)}`}
        icon={<Icon icon={icons.budget} />}
        iconColor="prompt"
        sub={`${fmtCompact(metrics.totalTokens)} tokens`}
      />
    </div>
  )
}

// Shared registry-count query — agents/MCP/skills/prompts totals. Used by
// both the KPI row and the composition donut so it fires once and caches.
function useRegistryCounts() {
  return useQuery({
    queryKey: ["dashboard-registry-counts"],
    queryFn: async () => {
      const [s, a, sk, p] = await Promise.all([
        listRegistryServers({ latest: true }),
        listRegistryAgents({ latest: true }),
        listRegistrySkills({ latest: true }),
        listRegistryPrompts({ latest: true }),
      ])
      return {
        servers: s.total ?? s.servers?.length ?? 0,
        agents: a.total ?? a.agents?.length ?? 0,
        skills: sk.total ?? sk.skills?.length ?? 0,
        prompts: p.total ?? p.prompts?.length ?? 0,
      }
    },
    refetchInterval: 60000,
  })
}

// ─── Request Volume Chart ─────────────────────────────────────────────────────

const chartConfig = {
  requests: {
    label: "Requests",
    color: "hsl(var(--primary))",
  },
  tokens: {
    label: "Tokens (100s)",
    color: "hsl(var(--kind-mcp))",
  },
} satisfies ChartConfig

function RequestVolumeChart({ range }: { range: TimeRange }) {
  const tp = trendParams(range)
  const currentInterval = tp.interval

  const { data: rawTrends, isLoading: loading } = useQuery({
    queryKey: ["usage-trends", tp],
    queryFn: () => getUsageTrends(tp),
    refetchInterval: 60000,
  })

  const chartData = React.useMemo(() => {
    if (!rawTrends) return []
    const raw = (rawTrends as any).data || rawTrends
    const data = raw || []
    const span = tp.hours ?? tp.days ?? 30
    return fillTimeGaps(data, currentInterval, span)
  }, [rawTrends, currentInterval, tp])

  const filteredData = chartData.map((item: any) => ({
    date: item.date,
    requests: item.requests || 0,
    tokens: item.tokens ? Math.floor(item.tokens / 100) : 0,
  }))

  return (
    <Card className="pt-0 flex-1 min-w-0">
      <CardHeader className="flex items-center gap-2 space-y-0 border-b py-4 sm:flex-row">
        <CardTitle className="flex-1 text-base">Request Volume</CardTitle>
        <span className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground">
          {rangeLabel(range)}
        </span>
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
                  <stop offset="5%" stopColor="var(--color-requests)" stopOpacity={0.4} />
                  <stop offset="95%" stopColor="var(--color-requests)" stopOpacity={0.02} />
                </linearGradient>
                <linearGradient id="fillTokens" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-tokens)" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="var(--color-tokens)" stopOpacity={0.02} />
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
                stroke="var(--color-tokens)"
                strokeWidth={1.5}
                stackId="a"
              />
              <Area
                dataKey="requests"
                type="monotone"
                fill="url(#fillRequests)"
                stroke="var(--color-requests)"
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

function ProviderBreakdown({ range }: { range: TimeRange }) {
  const mp = metricsParams(range)
  const { data: rawData, isLoading: metricsLoading } = useQuery({
    queryKey: ["dashboard-metrics", mp],
    queryFn: () => getDashboardMetrics(mp),
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
      <CardHeader className="border-b py-4 flex items-center gap-2 sm:flex-row">
        <CardTitle className="text-base flex-1">Provider Breakdown</CardTitle>
        <span className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground">
          {rangeLabel(range)}
        </span>
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
                      className="h-full rounded-full bg-primary transition-all"
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

function ModelsActivityTable({ range }: { range: TimeRange }) {
  const navigate = useNavigate()
  const mp = metricsParams(range)

  const { data: rawData, isLoading: metricsLoading } = useQuery({
    queryKey: ["dashboard-metrics", mp],
    queryFn: () => getDashboardMetrics(mp),
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
        <span className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground">
          {rangeLabel(range)}
        </span>
        <Button
          variant="ghost"
          size="sm"
          className="text-primary hover:text-primary/80 text-xs"
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
                      <span className={`h-1.5 w-1.5 rounded-full ${model.enabled ? "bg-good" : "bg-text-dim"}`} />
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

// ─── Catalog Composition Donut ────────────────────────────────────────────────

function CompositionDonut() {
  const { data: c } = useRegistryCounts()
  const { data: adminModelsData } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  })
  const modelCount = (adminModelsData as AdminModelsResponse | undefined)?.models?.length ?? 0

  const segs = [
    { key: "agent", name: "Agents", value: c?.agents ?? 0, color: "hsl(var(--kind-agent))" },
    { key: "mcp", name: "MCP Servers", value: c?.servers ?? 0, color: "hsl(var(--kind-mcp))" },
    { key: "skill", name: "Skills", value: c?.skills ?? 0, color: "hsl(var(--kind-skill))" },
    { key: "prompt", name: "Prompts", value: c?.prompts ?? 0, color: "hsl(var(--kind-prompt))" },
    { key: "model", name: "Models", value: modelCount, color: "hsl(var(--kind-model))" },
  ]
  const data = segs.filter((s) => s.value > 0)
  const total = segs.reduce((a, s) => a + s.value, 0)

  return (
    <Card className="w-full flex-shrink-0 pt-0 lg:w-[360px] xl:w-[400px]">
      <CardHeader className="border-b py-4">
        <CardTitle className="text-base">Catalog composition</CardTitle>
      </CardHeader>
      <CardContent className="pt-5">
        {total === 0 ? (
          <p className="py-10 text-center text-sm text-muted-foreground">Nothing published yet.</p>
        ) : (
          <div className="flex items-center gap-5">
            <div className="relative h-[150px] w-[150px] shrink-0">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie data={data} dataKey="value" nameKey="name" innerRadius={48} outerRadius={70} paddingAngle={2} stroke="none">
                    {data.map((s) => (
                      <Cell key={s.key} fill={s.color} />
                    ))}
                  </Pie>
                </PieChart>
              </ResponsiveContainer>
              <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                <span className="font-mono text-2xl font-semibold leading-none">{total}</span>
                <span className="text-[10.5px] uppercase tracking-wider text-text-dim">assets</span>
              </div>
            </div>
            <div className="flex-1 space-y-2">
              {segs.map((s) => (
                <div key={s.key} className="flex items-center gap-2.5 text-sm">
                  <span className="h-2.5 w-2.5 rounded-[3px]" style={{ background: s.color }} />
                  <span className="flex-1 text-muted-foreground">{s.name}</span>
                  <span className="font-mono tabular-nums">{s.value}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ─── Recent Activity Feed ─────────────────────────────────────────────────────

function RecentActivity() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["dashboard-recent-activity"],
    queryFn: () => getAuditLogs({ limit: 7 }),
    refetchInterval: 30000,
    retry: false,
  })
  const logs = (((data as any)?.audit_logs as AuditLog[] | undefined) ?? [])
  const dotClass = (r: AuditLog["event_result"]) =>
    r === "success" ? "bg-good" : r === "warning" ? "bg-warn" : r === "failure" || r === "error" ? "bg-bad" : "bg-info"

  return (
    <Card className="flex-1 pt-0">
      <CardHeader className="border-b py-4">
        <CardTitle className="text-base">Recent activity</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <div className="space-y-3 p-5">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="h-8 animate-pulse rounded bg-muted" />
            ))}
          </div>
        ) : isError || logs.length === 0 ? (
          <p className="py-10 text-center text-sm text-muted-foreground">No recent events.</p>
        ) : (
          <ul className="divide-y divide-border">
            {logs.map((l) => (
              <li key={l.id} className="flex items-start gap-3 px-5 py-3">
                <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${dotClass(l.event_result)}`} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[13px] font-medium">{l.event_action || l.event_type}</p>
                  <p className="truncate text-[11.5px] text-text-dim">
                    {[l.resource_type, l.message].filter(Boolean).join(" · ") || l.user?.email || "—"}
                  </p>
                </div>
                {l.created_at && (
                  <span className="shrink-0 text-[11px] text-text-dim">
                    {formatDistanceToNow(new Date(l.created_at))}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

// ─── Main Dashboard ───────────────────────────────────────────────────────────

export default function Dashboard() {
  const [range, setRange] = React.useState<TimeRange>("7d")
  return (
    <PageShell
      title="Dashboard"
      description="Registry health, governance, and gateway traffic at a glance."
      actions={<TimeRangePill value={range} onChange={setRange} />}
    >
      <SystemHealthStrip />
      <HeroMetrics range={range} />
      <div className="flex flex-col gap-4 xl:flex-row">
        <RequestVolumeChart range={range} />
        <CompositionDonut />
      </div>
      <div className="flex flex-col gap-4 lg:flex-row">
        <RecentActivity />
        <ProviderBreakdown range={range} />
      </div>
      <ModelsActivityTable range={range} />
    </PageShell>
  )
}
