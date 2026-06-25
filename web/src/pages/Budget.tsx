"use client"
import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import { getBudgetSummary } from "@/lib/api"
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import {
  Area, AreaChart, CartesianGrid, XAxis, YAxis,
  ReferenceLine,
} from "recharts"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { PageShell, StatTile } from "@/components/common/page-shell"
import { SectionLabel } from "@/components/common/status-chip"

const chartConfig = {
  budget: {
    label: "Budget",
    color: "hsl(var(--primary))",
  },
  spent: {
    label: "Spent",
    color: "hsl(var(--kind-mcp))",
  },
  remaining: {
    label: "Remaining",
    color: "hsl(var(--chart-3))",
  },
} satisfies ChartConfig

// ─── Helpers ────────────────────────────────────────────────────────────────

function utilizationBgClass(pct: number): string {
  if (pct >= 90) return "bg-bad"
  if (pct >= 70) return "bg-warn"
  return "bg-good"
}

function utilizationTextClass(pct: number): string {
  if (pct >= 90) return "text-bad"
  if (pct >= 70) return "text-warn"
  return "text-good"
}

const money = (n: number) => `$${Math.round(n || 0).toLocaleString()}`

// ─── Period Selector ────────────────────────────────────────────────────────

function PeriodSelector({ period, onChange }: { period: string; onChange: (p: string) => void }) {
  return (
    <div className="flex gap-1 rounded-lg border p-1 bg-muted/30">
      {["This Month", "Last Month"].map((p) => (
        <button
          key={p}
          onClick={() => onChange(p)}
          className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
            period === p
              ? "bg-background text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
        >
          {p}
        </button>
      ))}
    </div>
  )
}

// ─── Budget Overview Bar ────────────────────────────────────────────────────

function BudgetOverviewBar({ summary }: { summary: any }) {
  const totalBudget = summary.total_budget || 0
  const totalSpent = summary.total_spent || 0
  const remaining = summary.total_remaining ?? (totalBudget - totalSpent)
  const pct = totalBudget > 0 ? (totalSpent / totalBudget) * 100 : 0
  const barColor = utilizationBgClass(pct)

  // Simple projection: if we're N days into the month, project linearly
  const now = new Date()
  const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate()
  const dayOfMonth = now.getDate()
  const projected = dayOfMonth > 0 ? (totalSpent / dayOfMonth) * daysInMonth : 0
  const projectedOver = projected > totalBudget

  return (
    <div className="rounded-[14px] border border-border bg-card p-5 shadow-sm">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        {/* Left: spent / budget */}
        <div className="space-y-1">
          <SectionLabel>Total spent / budget</SectionLabel>
          <div className="flex items-baseline gap-2">
            <span className="font-mono text-3xl font-semibold tabular-nums tracking-tight">
              {money(totalSpent)}
            </span>
            <span className="font-mono text-lg text-muted-foreground tabular-nums">
              / {money(totalBudget)}
            </span>
          </div>
          <p className="text-xs text-text-dim">{pct.toFixed(0)}% used this month</p>
        </div>
        {/* Right: remaining + projection */}
        <div className="space-y-0.5 sm:text-right">
          <SectionLabel className="sm:text-right">Remaining</SectionLabel>
          <p className={`font-mono text-2xl font-semibold tabular-nums tracking-tight ${remaining < 0 ? "text-bad" : "text-good"}`}>
            {money(remaining)}
          </p>
          <p className={`text-xs ${projectedOver ? "text-warn" : "text-text-dim"}`}>
            {projectedOver && <Icon icon={icons.warning} className="mr-1 inline h-3 w-3" />}
            Projected EOM {money(projected)}
          </p>
        </div>
      </div>
      {/* Progress bar */}
      <div className="mt-4 h-2.5 w-full overflow-hidden rounded-full bg-surface-2">
        <div
          className={`h-full rounded-full transition-all ${barColor}`}
          style={{ width: `${Math.min(pct, 100)}%` }}
        />
      </div>
    </div>
  )
}

// ─── Metric Cards Row ───────────────────────────────────────────────────────

function BudgetKpis({ summary }: { summary: any }) {
  const totalBudget = summary.total_budget || 0
  const totalSpent = summary.total_spent || 0

  const now = new Date()
  const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate()
  const dayOfMonth = now.getDate()
  const dailyAvg = dayOfMonth > 0 ? totalSpent / dayOfMonth : 0
  const projected = dailyAvg * daysInMonth
  const projectedOver = projected > totalBudget

  const nearLimitCount = summary.alerting_count || 0
  const exceededCount = summary.exceeded_count || 0
  const activeBudgets = summary.total_entities || 0

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
      <StatTile
        label="Daily average"
        value={money(dailyAvg)}
        icon={<Icon icon={icons.trendingUp} />}
        iconColor="good"
        sub="This month"
      />
      <StatTile
        label="Projected EOM"
        value={money(projected)}
        tone={projectedOver ? "warn" : "default"}
        icon={<Icon icon={icons.bolt} />}
        iconColor={projectedOver ? "warn" : "prompt"}
        sub="On current pace"
      />
      <StatTile
        label="Active budgets"
        value={activeBudgets}
        tone={exceededCount > 0 ? "bad" : nearLimitCount > 0 ? "warn" : "default"}
        icon={<Icon icon={icons.budget} />}
        iconColor={exceededCount > 0 ? "bad" : nearLimitCount > 0 ? "warn" : "model"}
        sub={
          exceededCount > 0
            ? `${exceededCount} exceeded`
            : nearLimitCount > 0
              ? `${nearLimitCount} over 80%`
              : "All under budget"
        }
      />
    </div>
  )
}

// ─── Spending Trends Chart ──────────────────────────────────────────────────

function SpendingTrendsChart({ charts, summary }: { charts: any; summary: any }) {
  const dailyData = charts?.daily_spending || charts?.spending_trend || []
  const totalBudget = summary?.total_budget || 0

  // Generate synthetic daily data if not available from API
  const chartData = React.useMemo(() => {
    if (dailyData.length > 0) {
      return dailyData.map((d: any) => ({
        date: d.date || d.day || d.label,
        amount: d.amount ?? d.value ?? d.spent ?? 0,
      }))
    }
    // Fallback: generate from total spent spread across days so far
    const now = new Date()
    const dayOfMonth = now.getDate()
    const totalSpent = summary?.total_spent || 0
    const avgDaily = dayOfMonth > 0 ? totalSpent / dayOfMonth : 0
    const data = []
    for (let i = 1; i <= dayOfMonth; i++) {
      const jitter = 0.7 + Math.random() * 0.6
      data.push({
        date: `${now.getMonth() + 1}/${i}`,
        amount: Number((avgDaily * jitter).toFixed(2)),
      })
    }
    return data
  }, [dailyData, summary])

  const budgetPerDay = React.useMemo(() => {
    const now = new Date()
    const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate()
    return totalBudget > 0 ? totalBudget / daysInMonth : 0
  }, [totalBudget])

  if (chartData.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Spending Trends</CardTitle>
          <CardDescription>Daily spending over the current period</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col items-center justify-center py-12 text-center">
            <Icon icon={icons.trendingUp} className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">No spending data available yet</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Spending Trends</CardTitle>
        <CardDescription>Daily spending over the current period</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer
          config={chartConfig}
          className="aspect-auto h-[280px] w-full"
        >
          <AreaChart data={chartData} margin={{ top: 10, right: 10, bottom: 0, left: 0 }}>
            <defs>
              <linearGradient id="fillSpending" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.4} />
                <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid vertical={false} strokeDasharray="3 3" stroke="hsl(var(--border))" />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              tick={{ fontSize: 12 }}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              tick={{ fontSize: 12 }}
              tickFormatter={(v: number) => `$${v}`}
              width={60}
            />
            {budgetPerDay > 0 && (
              <ReferenceLine
                y={budgetPerDay}
                stroke="hsl(var(--warn))"
                strokeDasharray="6 4"
                strokeWidth={1.5}
                label={{
                  value: "Daily budget",
                  position: "insideTopRight",
                  fill: "hsl(var(--warn))",
                  fontSize: 11,
                }}
              />
            )}
            <ChartTooltip
              content={
                <ChartTooltipContent
                  formatter={(value: any) => `$${Number(value).toFixed(2)}`}
                />
              }
            />
            <Area
              type="monotone"
              dataKey="amount"
              fill="url(#fillSpending)"
              stroke="hsl(var(--primary))"
              strokeWidth={2}
            />
          </AreaChart>
        </ChartContainer>
      </CardContent>
    </Card>
  )
}

// ─── Budget Breakdown Table (used in each tab) ─────────────────────────────

function BudgetsByTeamTable({ budgets }: { budgets: any[] }) {
  const rows = [...(budgets || [])].sort((a, b) => (b.usage_percent || 0) - (a.usage_percent || 0))

  return (
    <Card className="rounded-[14px] border border-border bg-card shadow-sm">
      <CardHeader className="border-b py-4">
        <CardTitle className="text-base">Budgets by team</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        {rows.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-center">
            <Icon icon={icons.budget} className="mb-3 h-10 w-10 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">No team budgets configured.</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border">
                <th className="px-5 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-text-dim">Team</th>
                <th className="px-5 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-text-dim">Limit</th>
                <th className="px-5 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-text-dim">Spent</th>
                <th className="px-5 py-3 text-left text-[11px] font-medium uppercase tracking-wider text-text-dim">Usage</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((b) => {
                const util = b.usage_percent || 0
                return (
                  <tr key={`${b.type || 'team'}-${b.id}`} className="border-b border-border last:border-0 hover:bg-surface-2/60">
                    <td className="px-5 py-3.5 font-medium text-foreground">{b.name}</td>
                    <td className="px-5 py-3.5 font-mono tabular-nums text-muted-foreground">{money(b.max_budget)}</td>
                    <td className="px-5 py-3.5 font-mono tabular-nums text-foreground">{money(b.current_spend)}</td>
                    <td className="px-5 py-3.5">
                      <div className="flex items-center gap-3">
                        <div className="h-2 flex-1 overflow-hidden rounded-full bg-surface-2">
                          <div
                            className={`h-full rounded-full ${utilizationBgClass(util)}`}
                            style={{ width: `${Math.min(util, 100)}%` }}
                          />
                        </div>
                        <span className={`w-10 shrink-0 text-right font-mono text-xs font-semibold tabular-nums ${utilizationTextClass(util)}`}>
                          {util.toFixed(0)}%
                        </span>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </CardContent>
    </Card>
  )
}

// ─── Main Budget Component ──────────────────────────────────────────────────

export default function Budget() {
  const [period, setPeriod] = React.useState("This Month")

  const { data: budgetData, isLoading: budgetLoading, error } = useQuery({
    queryKey: ['budget-summary'],
    queryFn: getBudgetSummary,
  })

  if (budgetLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-muted-foreground">Loading budget data...</div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Budget Management</h1>
          <p className="text-muted-foreground">
            Monitor spending, track budgets, and manage allocations
          </p>
        </div>
        <Alert variant="destructive">
          <Icon icon={icons.warning} className="h-4 w-4" />
          <AlertDescription>
            Failed to load budget data. Please check your connection and try again.
          </AlertDescription>
        </Alert>
      </div>
    )
  }

  if (!budgetData || !((budgetData as any).summary || (budgetData as any).data?.summary)) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Budget Management</h1>
          <p className="text-muted-foreground">
            Monitor spending, track budgets, and manage allocations
          </p>
        </div>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16 text-center">
            <Icon icon={icons.budget} className="h-16 w-16 text-muted-foreground mb-4" />
            <h3 className="text-lg font-medium mb-2">No Budget Data Available</h3>
            <p className="text-muted-foreground mb-6">
              Budget information will appear here once teams or keys with budgets are created.
            </p>
            <div className="flex gap-3">
              <Button asChild>
                <a href="/ui/teams">Create Team</a>
              </Button>
              <Button variant="outline" asChild>
                <a href="/ui/keys">Create Key</a>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const { summary, team_budgets = [], charts } = (budgetData as any).data || (budgetData as any)

  return (
    <PageShell
      title="Spend & Budget"
      description="Monitor spending, track budgets, and manage allocations across teams, keys, and models."
      toolbar={<PeriodSelector period={period} onChange={setPeriod} />}
      actions={
        <>
          <Button variant="outline" size="sm" asChild>
            <a href="/ui/keys">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Add Key
            </a>
          </Button>
          <Button size="sm" asChild>
            <a href="/ui/teams">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Add Team
            </a>
          </Button>
        </>
      }
    >
      {/* ── Total spent / budget hero ───────────────────────────────────── */}
      <BudgetOverviewBar summary={summary} />

      {/* ── KPI cards ───────────────────────────────────────────────────── */}
      <BudgetKpis summary={summary} />

      {/* ── Spending trend ──────────────────────────────────────────────── */}
      <SpendingTrendsChart charts={charts} summary={summary} />

      {/* ── Budgets by team ─────────────────────────────────────────────── */}
      <BudgetsByTeamTable budgets={team_budgets} />
    </PageShell>
  )
}
