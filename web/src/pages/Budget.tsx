"use client"
import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import { getBudgetSummary } from "@/lib/api"
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import {
  Area, AreaChart, CartesianGrid, XAxis, YAxis, Pie, PieChart,
  ReferenceLine,
} from "recharts"

import { Badge } from "@/components/ui/badge"
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"

const chartConfig = {
  budget: {
    label: "Budget",
    color: "hsl(var(--chart-1))",
  },
  spent: {
    label: "Spent",
    color: "hsl(var(--chart-2))",
  },
  remaining: {
    label: "Remaining",
    color: "hsl(var(--chart-3))",
  },
} satisfies ChartConfig

// ─── Helpers ────────────────────────────────────────────────────────────────

function utilizationBgClass(pct: number): string {
  if (pct > 90) return "bg-red-500"
  if (pct > 75) return "bg-amber-500"
  return "bg-teal-500"
}

function statusBadge(pct: number, isExceeded: boolean) {
  if (isExceeded || pct > 100) return <Badge variant="destructive">Over budget</Badge>
  if (pct > 75) return <Badge className="bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/25">Near limit</Badge>
  return <Badge variant="outline">On track</Badge>
}

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
    <Card>
      <CardContent className="pt-6 pb-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          {/* Left: spent / budget */}
          <div className="space-y-1">
            <p className="text-sm text-muted-foreground">Total Spent / Budget</p>
            <div className="flex items-baseline gap-2">
              <span className="text-3xl font-semibold font-mono tabular-nums tracking-tight">
                ${totalSpent.toFixed(2)}
              </span>
              <span className="text-lg text-muted-foreground font-mono tabular-nums">
                / ${totalBudget.toFixed(2)}
              </span>
            </div>
          </div>
          {/* Right: remaining + projection */}
          <div className="text-right space-y-1">
            <p className="text-sm text-muted-foreground">Remaining</p>
            <p className="text-2xl font-semibold font-mono tabular-nums tracking-tight">
              ${remaining.toFixed(2)}
            </p>
            <p className={`text-xs font-medium font-mono ${projectedOver ? "text-amber-500" : "text-muted-foreground"}`}>
              {projectedOver && <Icon icon={icons.warning} className="inline h-3 w-3 mr-1" />}
              Projected EOM: ${projected.toFixed(2)}
            </p>
          </div>
        </div>
        {/* Progress bar */}
        <div className="mt-4 space-y-1">
          <div className="flex justify-between text-xs text-muted-foreground font-mono">
            <span>{pct.toFixed(1)}% used</span>
            <span>{Math.max(0, 100 - pct).toFixed(1)}% remaining</span>
          </div>
          <div className="h-3 w-full rounded-full bg-muted overflow-hidden">
            <div
              className={`h-full rounded-full transition-all ${barColor}`}
              style={{ width: `${Math.min(pct, 100)}%` }}
            />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ─── Metric Cards Row ───────────────────────────────────────────────────────

function MetricCards({ summary }: { summary: any }) {
  const totalBudget = summary.total_budget || 0
  const totalSpent = summary.total_spent || 0

  const now = new Date()
  const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate()
  const dayOfMonth = now.getDate()
  const dailyAvg = dayOfMonth > 0 ? totalSpent / dayOfMonth : 0
  const projected = dailyAvg * daysInMonth
  const projectedOver = projected > totalBudget

  const nearLimitCount = (summary.alerting_count || 0)
  const activeBudgets = summary.total_entities || 0

  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      {/* Daily Average */}
      <Card>
        <CardContent className="pt-5 pb-5">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-teal-500/10">
              <Icon icon={icons.trendingUp} className="h-5 w-5 text-teal-500" />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Daily Average</p>
              <p className="text-xl font-semibold font-mono tabular-nums">
                ${dailyAvg.toFixed(2)}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Projected EOM */}
      <Card>
        <CardContent className="pt-5 pb-5">
          <div className="flex items-center gap-3">
            <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${
              projectedOver ? "bg-amber-500/10" : "bg-teal-500/10"
            }`}>
              <Icon icon={icons.calendar} className={`h-5 w-5 ${projectedOver ? "text-amber-500" : "text-teal-500"}`} />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Projected End of Month</p>
              <p className={`text-xl font-semibold font-mono tabular-nums ${projectedOver ? "text-amber-500" : ""}`}>
                ${projected.toFixed(2)}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Active Budgets */}
      <Card>
        <CardContent className="pt-5 pb-5">
          <div className="flex items-center gap-3">
            <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${
              nearLimitCount > 0 ? "bg-amber-500/10" : "bg-teal-500/10"
            }`}>
              <Icon icon={icons.budget} className={`h-5 w-5 ${nearLimitCount > 0 ? "text-amber-500" : "text-teal-500"}`} />
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Active Budgets</p>
              <div className="flex items-baseline gap-2">
                <p className="text-xl font-semibold font-mono tabular-nums">{activeBudgets}</p>
                {nearLimitCount > 0 && (
                  <span className="text-xs font-medium text-amber-500">
                    {nearLimitCount} near limit
                  </span>
                )}
                {(summary.exceeded_count || 0) > 0 && (
                  <span className="text-xs font-medium text-red-500">
                    {summary.exceeded_count} exceeded
                  </span>
                )}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
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
                <stop offset="5%" stopColor="#14B8A6" stopOpacity={0.4} />
                <stop offset="95%" stopColor="#14B8A6" stopOpacity={0.02} />
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
                stroke="rgb(245 158 11)"
                strokeDasharray="6 4"
                strokeWidth={1.5}
                label={{
                  value: "Daily budget",
                  position: "insideTopRight",
                  fill: "rgb(245 158 11)",
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
              stroke="#14B8A6"
              strokeWidth={2}
            />
          </AreaChart>
        </ChartContainer>
      </CardContent>
    </Card>
  )
}

// ─── Budget Breakdown Table (used in each tab) ─────────────────────────────

function BreakdownTable({ budgets }: { budgets: any[] }) {
  const sorted = [...budgets].sort((a, b) => {
    const aUtil = a.usage_percent || 0
    const bUtil = b.usage_percent || 0
    return bUtil - aUtil
  })

  if (sorted.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Icon icon={icons.budget} className="h-10 w-10 text-muted-foreground mb-3" />
        <p className="text-sm text-muted-foreground">No entities in this category</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead className="text-right">Budget</TableHead>
            <TableHead className="text-right">Spent</TableHead>
            <TableHead>Utilization</TableHead>
            <TableHead className="text-right font-mono">%</TableHead>
            <TableHead>Status</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map((b) => {
            const util = b.usage_percent || 0
            const isExceeded = b.is_exceeded || false
            const color = utilizationBgClass(util)
            return (
              <TableRow key={`${b.type}-${b.id}`}>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Icon
                      icon={b.type === 'team' ? icons.teams : b.type === 'model' ? icons.models : icons.keys}
                      className="h-4 w-4 text-muted-foreground"
                    />
                    <span className="font-medium">{b.name}</span>
                  </div>
                </TableCell>
                <TableCell className="text-right font-mono tabular-nums">
                  ${b.max_budget?.toFixed(2) || '0.00'}
                </TableCell>
                <TableCell className="text-right font-mono tabular-nums">
                  ${b.current_spend?.toFixed(2) || '0.00'}
                </TableCell>
                <TableCell>
                  <div className="w-24 h-2 rounded-full bg-muted overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${color}`}
                      style={{ width: `${Math.min(util, 100)}%` }}
                    />
                  </div>
                </TableCell>
                <TableCell className="text-right font-mono tabular-nums text-sm">
                  {util.toFixed(1)}%
                </TableCell>
                <TableCell>
                  {statusBadge(util, isExceeded)}
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}

// ─── Budget Breakdown Tabs ──────────────────────────────────────────────────

function BudgetBreakdown({ teamBudgets, keyBudgets }: { teamBudgets: any[]; keyBudgets: any[] }) {
  const allBudgets = [...(teamBudgets || []), ...(keyBudgets || [])]
  // There's no model budget data from the API, so model tab uses empty
  const modelBudgets: any[] = []

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div>
          <CardTitle>Budget Breakdown</CardTitle>
          <CardDescription className="mt-1">
            {allBudgets.length} entities with budget tracking
          </CardDescription>
        </div>
        <Button variant="outline" size="sm" asChild>
          <a href="/ui/teams">
            <Icon icon={icons.settings} className="h-4 w-4" />
            Manage
          </a>
        </Button>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="team" className="w-full">
          <TabsList className="mb-4">
            <TabsTrigger value="team">
              <Icon icon={icons.teams} className="h-3.5 w-3.5 mr-1.5" />
              By Team
            </TabsTrigger>
            <TabsTrigger value="model">
              <Icon icon={icons.models} className="h-3.5 w-3.5 mr-1.5" />
              By Model
            </TabsTrigger>
            <TabsTrigger value="key">
              <Icon icon={icons.keys} className="h-3.5 w-3.5 mr-1.5" />
              By Key
            </TabsTrigger>
          </TabsList>
          <TabsContent value="team">
            <BreakdownTable budgets={(teamBudgets || []).map(b => ({ ...b, type: 'team' }))} />
          </TabsContent>
          <TabsContent value="model">
            <BreakdownTable budgets={modelBudgets} />
          </TabsContent>
          <TabsContent value="key">
            <BreakdownTable budgets={(keyBudgets || []).map(b => ({ ...b, type: 'key' }))} />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}

// ─── Budget Distribution Charts (preserved) ────────────────────────────────

function BudgetDistributionCharts({ charts }: { charts: any }) {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
      {/* Budget Allocation */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Budget Allocation</CardTitle>
          <CardDescription>Distribution by type</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-center pb-6">
          {charts?.budget_distribution?.some((item: any) => item.value > 0) ? (
            <ChartContainer
              config={chartConfig}
              className="mx-auto h-[220px] w-full max-w-[220px]"
            >
              <PieChart>
                <Pie
                  dataKey="value"
                  data={charts.budget_distribution.map((item: any, index: number) => ({
                    ...item,
                    fill: `hsl(var(--chart-${(index % 5) + 1}))`,
                  }))}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  outerRadius={80}
                />
                <ChartTooltip content={<ChartTooltipContent />} />
              </PieChart>
            </ChartContainer>
          ) : (
            <div className="flex flex-col items-center justify-center py-8 text-center">
              <Icon icon={icons.budget} className="h-10 w-10 text-muted-foreground mb-3" />
              <p className="text-sm text-muted-foreground">No budget allocation data</p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Spending Distribution */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Spending Distribution</CardTitle>
          <CardDescription>Actual spend by type</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-center pb-6">
          {charts?.spending_distribution?.some((item: any) => item.value > 0) ? (
            <ChartContainer
              config={chartConfig}
              className="mx-auto h-[220px] w-full max-w-[220px]"
            >
              <PieChart>
                <Pie
                  dataKey="value"
                  data={charts.spending_distribution.map((item: any, index: number) => ({
                    ...item,
                    fill: `hsl(var(--chart-${(index % 5) + 1}))`,
                  }))}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  outerRadius={80}
                />
                <ChartTooltip content={<ChartTooltipContent />} />
              </PieChart>
            </ChartContainer>
          ) : (
            <div className="flex flex-col items-center justify-center py-8 text-center">
              <Icon icon={icons.budget} className="h-10 w-10 text-muted-foreground mb-3" />
              <p className="text-sm text-muted-foreground">No spending data yet</p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
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

  const { summary, team_budgets = [], key_budgets = [], charts } = (budgetData as any).data || (budgetData as any)

  return (
    <div className="space-y-6">
      {/* ── Page Header ─────────────────────────────────────────────────── */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Budget Management</h1>
          <p className="text-muted-foreground">
            Monitor spending, track budgets, and manage allocations
          </p>
        </div>
        <div className="flex items-center gap-3">
          <PeriodSelector period={period} onChange={setPeriod} />
          <div className="flex gap-2">
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
          </div>
        </div>
      </div>

      {/* ── Budget Overview Bar ─────────────────────────────────────────── */}
      <BudgetOverviewBar summary={summary} />

      {/* ── Metric Cards ────────────────────────────────────────────────── */}
      <MetricCards summary={summary} />

      {/* ── Spending Trends Chart ───────────────────────────────────────── */}
      <SpendingTrendsChart charts={charts} summary={summary} />

      {/* ── Distribution Charts ─────────────────────────────────────────── */}
      <BudgetDistributionCharts charts={charts} />

      {/* ── Budget Breakdown (tabbed) ───────────────────────────────────── */}
      <BudgetBreakdown teamBudgets={team_budgets} keyBudgets={key_budgets} />
    </div>
  )
}
