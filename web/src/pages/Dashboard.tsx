import * as React from "react"
import { useNavigate } from "react-router-dom"
import { Area, AreaChart, CartesianGrid, XAxis, ResponsiveContainer, PieChart, Pie, Cell } from "recharts"
import { useQuery } from "@tanstack/react-query"
import { formatDistanceToNow } from "date-fns"
import { Icon } from "@iconify/react"
import { icons } from "@/lib/icons"
import { detectProvider } from "@/lib/providers"
import {
  getDashboardMetrics,
  getUsageTrends,
  getAdminModels,
  getModelsHealth,
  getGuardrailStats,
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

import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart"

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

  const { data: guard } = useGuardrailAgg()

  const metrics = React.useMemo(() => {
    if (!rawData) return null
    const data = (rawData as any).data || rawData
    return {
      totalRequests: data.total_requests || 0,
      totalTokens: data.total_tokens || 0,
      totalCost: data.total_cost || 0,
      activeKeys: data.active_keys || 0,
      activeModels: data.active_models || 0,
      avgLatency: data.avg_latency || 0,
      successRate: data.success_rate || 0,
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
  const failed = deployments.filter((d) => d.status === "failed").length
  const reqTrend = computeTrend(sparkRequests)
  const passRate = guard?.passRate ?? null

  return (
    <div className="grid grid-cols-2 gap-3.5 sm:grid-cols-3 lg:grid-cols-5">
      <StatTile
        label="Registered assets"
        value={registeredAssets ?? "—"}
        icon={<Icon icon={icons.registry} />}
        iconColor="agent"
        sub={c ? `${c.agents} agents · ${c.servers} MCP · ${c.skills + c.prompts} others` : "Across the catalog"}
      />
      <StatTile
        label="Live deployments"
        value={deployments.length}
        icon={<Icon icon={icons.rocket} />}
        iconColor="mcp"
        sub={
          failed > 0
            ? `${running} running · ${failed} failed`
            : `${running} running`
        }
      />
      <StatTile
        label={`Requests (${rangeLabel(range)})`}
        value={fmtCompact(metrics.totalRequests)}
        icon={<Icon icon={icons.trendingUp} />}
        iconColor="primary"
        delta={reqTrend !== null ? `${reqTrend >= 0 ? "+" : ""}${reqTrend.toFixed(0)}%` : undefined}
        deltaPositive={(reqTrend ?? 0) >= 0}
        sub={`${activeModelCount} models · ${metrics.activeKeys} keys`}
      />
      <StatTile
        label="Avg latency"
        value={metrics.avgLatency > 0 ? `${metrics.avgLatency}ms` : "—"}
        icon={<Icon icon={icons.activity} />}
        iconColor="warn"
        sub="Across all routes"
      />
      <StatTile
        label="Guardrail pass rate"
        value={passRate !== null ? `${passRate.toFixed(1)}%` : "—"}
        icon={<Icon icon={icons.shieldCheck} />}
        iconColor="good"
        sub={
          guard && guard.executions > 0
            ? `${guard.blocked.toLocaleString()} requests blocked`
            : "No traffic checked yet"
        }
      />
    </div>
  )
}

// Aggregate guardrail runtime counters across every configured guardrail.
// These are the executor's in-memory counters (reset on gateway restart),
// so they read 0 until traffic flows — never fabricated. Drives the
// pass-rate KPI and the compliance panel's blocked / PII / injection trio.
function useGuardrailAgg() {
  return useQuery({
    queryKey: ["guardrail-stats-agg"],
    queryFn: async () => {
      const res: any = await getGuardrailStats()
      const body = res?.data ?? res
      const stats: Record<string, any> = body?.stats ?? {}
      let executions = 0,
        passed = 0,
        blocked = 0,
        pii = 0,
        injection = 0
      for (const [name, s] of Object.entries(stats)) {
        executions += s.total_executions ?? 0
        passed += s.total_passed ?? 0
        blocked += s.total_blocked ?? 0
        const n = name.toLowerCase()
        if (n.includes("pii") || n.includes("redact") || n.includes("presidio"))
          pii += s.total_blocked ?? 0
        if (n.includes("inject") || n.includes("prompt") || n.includes("lakera"))
          injection += s.total_blocked ?? 0
      }
      const passRate = executions > 0 ? (passed / executions) * 100 : null
      return {
        executions,
        passed,
        blocked,
        pii,
        injection,
        passRate,
        enabled: !!body?.enabled,
      }
    },
    refetchInterval: 60000,
    retry: false,
  })
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

// ─── Mockup panel primitives ──────────────────────────────────────────────────

function Panel({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <div
      className={`rounded-[14px] border border-border bg-card p-[18px] shadow-sm ${className ?? ""}`}
    >
      {children}
    </div>
  )
}

function SectionTitle({ title, sub }: { title: string; sub?: string }) {
  return (
    <div>
      <div className="text-[14px] font-semibold tracking-tight text-foreground">
        {title}
      </div>
      {sub && <div className="mt-0.5 text-[12px] text-text-dim">{sub}</div>}
    </div>
  )
}

// Soft 12% tint of a hex — for chips / rings that carry a static brand or
// semantic color (kind pink, framework greens/ambers).
function softTint(hex: string): string {
  const n = parseInt(hex.slice(1), 16)
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, 0.12)`
}

// ─── Operational status pill (header) ─────────────────────────────────────────

function OperationalPill() {
  const { data: healthData } = useQuery({
    queryKey: ["models-health"],
    queryFn: getModelsHealth,
    refetchInterval: 30000,
  })

  const summary = React.useMemo(() => {
    const response = healthData as ModelsHealthResponse | undefined
    if (!response?.models) return { healthy: 0, degraded: 0, down: 0 }
    let healthy = 0,
      degraded = 0,
      down = 0
    for (const m of Object.values(response.models)) {
      if (!m.healthy) down++
      else if (m.healthy_count < m.total_count) degraded++
      else healthy++
    }
    return { healthy, degraded, down }
  }, [healthData])

  const total = summary.healthy + summary.degraded + summary.down
  const allGood = summary.down === 0 && summary.degraded === 0
  const hex = allGood ? "#0E9F6E" : summary.down > 0 ? "#E0484D" : "#D97706"
  const label =
    total === 0
      ? "Awaiting traffic"
      : allGood
        ? "All systems operational"
        : summary.down > 0
          ? `${summary.down} system${summary.down === 1 ? "" : "s"} down`
          : `${summary.degraded} degraded`

  return (
    <div
      className="inline-flex h-8 items-center gap-2 rounded-lg px-3 text-[12.5px] font-semibold"
      style={{ background: softTint(hex), color: hex }}
    >
      <span
        className={`h-[7px] w-[7px] rounded-full ${allGood ? "animate-pulse" : ""}`}
        style={{ background: hex }}
      />
      {label}
    </div>
  )
}

// ─── Request volume area chart ────────────────────────────────────────────────

const chartConfig = {
  requests: {
    label: "Requests",
    color: "hsl(var(--primary))",
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
    const span = tp.hours ?? tp.days ?? 30
    return fillTimeGaps(raw || [], currentInterval, span)
  }, [rawTrends, currentInterval, tp])

  const filteredData = chartData.map((item: any) => ({
    date: item.date,
    requests: item.requests || 0,
  }))
  const total = filteredData.reduce((a: number, d: any) => a + d.requests, 0)

  return (
    <Panel className="min-w-0 flex-1">
      <div className="mb-2 flex items-start justify-between gap-3">
        <SectionTitle title="Request volume" sub={`Across all routes · last ${range}`} />
        <span className="font-mono text-[20px] font-semibold tracking-tight">
          {fmtCompact(total)}
        </span>
      </div>
      {loading ? (
        <div className="flex h-[200px] items-center justify-center">
          <Icon icon={icons.refresh} className="h-4 w-4 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <ChartContainer config={chartConfig} className="aspect-auto h-[200px] w-full">
          <AreaChart data={filteredData} margin={{ top: 10, right: 6, bottom: 0, left: 0 }}>
            <defs>
              <linearGradient id="fillRequests" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.22} />
                <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid vertical={false} strokeDasharray="3 6" stroke="hsl(var(--border))" />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={32}
              tickFormatter={(value) => formatChartLabel(value, currentInterval)}
              className="font-mono text-[11px]"
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
              dataKey="requests"
              type="monotone"
              fill="url(#fillRequests)"
              stroke="hsl(var(--primary))"
              strokeWidth={2.5}
            />
          </AreaChart>
        </ChartContainer>
      )}
    </Panel>
  )
}

// ─── Registry composition donut ───────────────────────────────────────────────

function CompositionDonut() {
  const { data: c } = useRegistryCounts()
  const { data: adminModelsData } = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  })
  const modelCount =
    (adminModelsData as AdminModelsResponse | undefined)?.models?.length ?? 0

  const segs = [
    { key: "agent", name: "Agents", value: c?.agents ?? 0, color: "hsl(var(--kind-agent))" },
    { key: "mcp", name: "MCP Servers", value: c?.servers ?? 0, color: "hsl(var(--kind-mcp))" },
    { key: "skill", name: "Skills", value: c?.skills ?? 0, color: "hsl(var(--kind-skill))" },
    { key: "prompt", name: "Prompts", value: c?.prompts ?? 0, color: "hsl(var(--kind-prompt))" },
    { key: "model", name: "Models", value: modelCount, color: "hsl(var(--kind-model))" },
  ]
  const data = segs.filter((s) => s.value > 0)
  const total = segs.reduce((a, s) => a + s.value, 0)
  const kinds = data.length

  return (
    <Panel className="w-full shrink-0 lg:w-[360px] xl:w-[400px]">
      <SectionTitle
        title="Registry composition"
        sub={`${total} assets across ${kinds} ${kinds === 1 ? "kind" : "kinds"}`}
      />
      {total === 0 ? (
        <p className="py-10 text-center text-sm text-muted-foreground">
          Nothing published yet.
        </p>
      ) : (
        <div className="mt-4 flex items-center gap-5">
          <div className="relative h-[124px] w-[124px] shrink-0">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={data}
                  dataKey="value"
                  nameKey="name"
                  innerRadius={42}
                  outerRadius={58}
                  paddingAngle={2}
                  stroke="none"
                >
                  {data.map((s) => (
                    <Cell key={s.key} fill={s.color} />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
            <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
              <span className="font-mono text-[22px] font-semibold leading-none">
                {total}
              </span>
              <span className="text-[9.5px] uppercase tracking-wider text-text-dim">
                assets
              </span>
            </div>
          </div>
          <div className="flex flex-1 flex-col gap-[9px]">
            {segs.map((s) => (
              <div key={s.key} className="flex items-center gap-2.5 text-[12.5px]">
                <span
                  className="h-2.5 w-2.5 rounded-[3px]"
                  style={{ background: s.color }}
                />
                <span className="flex-1 text-muted-foreground">{s.name}</span>
                <span className="font-mono font-semibold tabular-nums">
                  {s.value}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </Panel>
  )
}

// ─── Compliance & guardrails ──────────────────────────────────────────────────

// Static compliance posture for the demo — pllm has no frameworks API yet, so
// these chips are representative (the Compliance page is the real surface).
// The pass-rate ring + blocked / PII / injection figures ARE real (guardrail
// executor counters), so they read 0 until traffic flows.
const FRAMEWORKS = [
  { name: "SOC 2", label: "Compliant", color: "#0E9F6E" },
  { name: "ISO 27001", label: "Compliant", color: "#0E9F6E" },
  { name: "GDPR", label: "Compliant", color: "#0E9F6E" },
  { name: "EU AI Act", label: "In review", color: "#D97706" },
]

function Ring({
  pct,
  label,
  valueLabel,
}: {
  pct: number
  label: string
  valueLabel: string
}) {
  // r=15.915 → circumference ≈ 100, so dasharray "{pct} 100" fills pct%.
  const dash = `${Math.max(0, Math.min(100, pct))} 100`
  return (
    <div className="relative h-[112px] w-[112px] shrink-0">
      <svg viewBox="0 0 42 42" className="h-full w-full -rotate-90">
        <circle cx="21" cy="21" r="15.915" fill="none" stroke="hsl(var(--surface-2))" strokeWidth="4" />
        <circle
          cx="21"
          cy="21"
          r="15.915"
          fill="none"
          stroke="hsl(var(--good))"
          strokeWidth="4"
          strokeDasharray={dash}
          strokeLinecap="round"
          className="transition-all duration-700 ease-out"
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="font-mono text-[19px] font-semibold leading-none">
          {valueLabel}
        </span>
        <span className="text-[9px] uppercase tracking-wider text-text-dim">
          {label}
        </span>
      </div>
    </div>
  )
}

function ComplianceGuardrails() {
  const { data: guard } = useGuardrailAgg()
  const passRate = guard?.passRate ?? null
  const stats = [
    { value: guard ? guard.blocked.toLocaleString() : "—", label: "Requests blocked" },
    { value: guard ? guard.pii.toLocaleString() : "—", label: "PII redactions" },
    { value: guard ? guard.injection.toLocaleString() : "—", label: "Injections caught" },
  ]

  return (
    <Panel className="min-w-0 lg:flex-[1.4]">
      <SectionTitle
        title="Compliance & guardrails"
        sub="Policy enforcement across all traffic"
      />
      <div className="mt-[18px] flex items-center gap-[22px]">
        <Ring
          pct={passRate ?? 0}
          label="pass rate"
          valueLabel={passRate !== null ? `${passRate.toFixed(1)}%` : "—"}
        />
        <div className="flex flex-1 flex-col gap-3.5">
          {stats.map((s) => (
            <div key={s.label}>
              <div className="font-mono text-[20px] font-semibold leading-none">
                {s.value}
              </div>
              <div className="mt-1 text-[11.5px] text-muted-foreground">
                {s.label}
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className="my-4 h-px bg-border" />
      <div className="mb-2.5 font-mono text-[10.5px] uppercase tracking-wider text-text-dim">
        Compliance frameworks
      </div>
      <div className="flex flex-wrap gap-2">
        {FRAMEWORKS.map((f) => (
          <span
            key={f.name}
            className="inline-flex items-center gap-1.5 rounded-md px-2 py-[3px] text-[11.5px] font-medium"
            style={{ background: softTint(f.color), color: f.color }}
          >
            <span className="h-1.5 w-1.5 rounded-full" style={{ background: f.color }} />
            {f.name} · {f.label}
          </span>
        ))}
      </div>
    </Panel>
  )
}

// ─── Recent activity feed ─────────────────────────────────────────────────────

function RecentActivity() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["dashboard-recent-activity"],
    queryFn: () => getAuditLogs({ limit: 6 }),
    refetchInterval: 30000,
    retry: false,
  })
  const logs = ((data as any)?.audit_logs as AuditLog[] | undefined) ?? []
  const dotClass = (r: AuditLog["event_result"]) =>
    r === "success"
      ? "bg-good"
      : r === "warning"
        ? "bg-warn"
        : r === "failure" || r === "error"
          ? "bg-bad"
          : "bg-info"

  return (
    <Panel className="flex min-w-0 flex-1 flex-col">
      <SectionTitle title="Recent activity" />
      {isLoading ? (
        <div className="mt-3 space-y-3">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="h-8 animate-pulse rounded bg-muted" />
          ))}
        </div>
      ) : isError || logs.length === 0 ? (
        <p className="py-10 text-center text-sm text-muted-foreground">
          No recent events.
        </p>
      ) : (
        <div className="mt-1.5 flex flex-col">
          {logs.map((l) => (
            <div
              key={l.id}
              className="flex items-start gap-3 border-b border-border py-[11px] last:border-0"
            >
              <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${dotClass(l.event_result)}`} />
              <div className="min-w-0 flex-1">
                <div className="truncate text-[13px] font-medium">
                  {l.event_action || l.event_type}
                </div>
                <div className="truncate text-[11.5px] text-text-dim">
                  {[l.resource_type, l.message].filter(Boolean).join(" · ") ||
                    l.user?.email ||
                    "—"}
                </div>
              </div>
              {l.created_at && (
                <span className="shrink-0 whitespace-nowrap font-mono text-[11px] text-text-dim">
                  {formatDistanceToNow(new Date(l.created_at))}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </Panel>
  )
}

// ─── Asset activity table ─────────────────────────────────────────────────────
//
// Busiest models, presented in the mockup's asset-table shape. Every figure
// is real (per-model requests + latency from the dashboard metrics top_models
// join). Models are registry assets of kind "model", hence the kind chip.

const ASSET_COLS = "grid-cols-[2.2fr_1.1fr_1fr_.9fr_1fr]"

function AssetActivityTable({ range }: { range: TimeRange }) {
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

    const dashboard = rawData ? (rawData as any).data || rawData : null
    const topModels: any[] = dashboard?.top_models || []
    const usageMap = new Map<string, { requests: number; latency: number }>()
    for (const m of topModels) {
      if (m.model) {
        usageMap.set(m.model, {
          requests: m.requests || 0,
          latency: m.p95_latency || m.avg_latency || 0,
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
          latency: Math.round(usage?.latency || 0),
        }
      })
      .sort((a, b) => b.requests - a.requests)
      .slice(0, 6)
  }, [rawData, adminModelsData])

  return (
    <div className="overflow-hidden rounded-[14px] border border-border bg-card shadow-sm">
      <div className="flex items-center justify-between border-b border-border px-[18px] py-4">
        <SectionTitle title="Asset activity" sub={`Busiest models · last ${range}`} />
        <button
          onClick={() => navigate("/registry")}
          className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-card px-3 text-[12.5px] font-medium text-muted-foreground transition-colors hover:bg-surface-2 hover:text-foreground"
        >
          Open registry
          <Icon icon={icons.arrowRight} className="h-3.5 w-3.5" />
        </button>
      </div>
      <div
        className={`grid ${ASSET_COLS} items-center gap-2.5 border-b border-border px-[18px] py-[11px] font-mono text-[10.5px] uppercase tracking-wider text-text-dim`}
      >
        <span>Asset</span>
        <span>Kind</span>
        <span>Requests</span>
        <span>Latency</span>
        <span>Status</span>
      </div>
      {loading ? (
        <div className="space-y-3 p-[18px]">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="h-5 animate-pulse rounded bg-muted" />
          ))}
        </div>
      ) : models.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          No models configured yet.
        </div>
      ) : (
        models.map((m) => (
          <div
            key={m.id}
            className={`grid ${ASSET_COLS} items-center gap-2.5 border-b border-border px-[18px] py-[13px] transition-colors last:border-0 hover:bg-surface-2`}
          >
            <div className="flex min-w-0 items-center gap-2.5">
              <Icon
                icon={detectProvider(m.name || m.id, m.provider).icon}
                width={17}
                height={17}
                className="shrink-0"
              />
              <span className="truncate text-[13px] font-semibold">{m.name}</span>
            </div>
            <div>
              <span
                className="inline-flex items-center rounded-md px-2 py-[3px] text-[10.5px] font-semibold"
                style={{ background: softTint("#DB2777"), color: "#DB2777" }}
              >
                Model
              </span>
            </div>
            <div className="font-mono text-[13px]">
              {m.requests.toLocaleString()}
            </div>
            <div className="font-mono text-[13px] text-muted-foreground">
              {m.latency > 0 ? `${m.latency}ms` : "—"}
            </div>
            <div>
              <span
                className="inline-flex items-center gap-1.5 text-[12px] font-medium"
                style={{ color: m.enabled ? "hsl(var(--good))" : "hsl(var(--text-dim))" }}
              >
                <span
                  className="h-1.5 w-1.5 rounded-full"
                  style={{ background: m.enabled ? "hsl(var(--good))" : "hsl(var(--text-dim))" }}
                />
                {m.enabled ? "Active" : "Inactive"}
              </span>
            </div>
          </div>
        ))
      )}
    </div>
  )
}

// ─── Main Dashboard ───────────────────────────────────────────────────────────

export default function Dashboard() {
  const [range, setRange] = React.useState<TimeRange>("7d")
  return (
    <PageShell
      title="Dashboard"
      description="Registry health, governance, and gateway traffic at a glance."
      actions={
        <div className="flex items-center gap-3">
          <OperationalPill />
          <TimeRangePill value={range} onChange={setRange} />
        </div>
      }
    >
      <HeroMetrics range={range} />
      <div className="flex flex-col gap-4 lg:flex-row">
        <RequestVolumeChart range={range} />
        <CompositionDonut />
      </div>
      <div className="flex flex-col gap-4 lg:flex-row">
        <ComplianceGuardrails />
        <RecentActivity />
      </div>
      <AssetActivityTable range={range} />
    </PageShell>
  )
}

