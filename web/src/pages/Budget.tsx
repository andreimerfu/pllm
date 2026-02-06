"use client"
import { useQuery } from "@tanstack/react-query"
import { getBudgetSummary } from "@/lib/api"
import { 
  TrendingUp, 
  AlertTriangle, 
  DollarSign,
  Users,
  Key,
  Settings,
  Plus
} from "lucide-react"
import { Pie, PieChart } from "recharts"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
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
import { Progress } from "@/components/ui/progress"

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

// Budget Health Status Component
function BudgetHealthAlert({ summary }: { summary: any }) {
  if (!summary.exceeded_count && !summary.alerting_count) return null

  const severity = summary.exceeded_count > 0 ? "destructive" : "default"
  const IconComponent = summary.exceeded_count > 0 ? AlertTriangle : TrendingUp

  return (
    <Alert variant={severity} className="border-l-4">
      <IconComponent className="h-4 w-4" />
      <AlertDescription>
        <div className="flex items-center justify-between">
          <div>
            {summary.exceeded_count > 0 && (
              <span className="font-medium">
                {summary.exceeded_count} budget{summary.exceeded_count > 1 ? 's' : ''} exceeded
              </span>
            )}
            {summary.exceeded_count > 0 && summary.alerting_count > 0 && <span> • </span>}
            {summary.alerting_count > 0 && (
              <span className="font-medium text-amber-600 dark:text-amber-400">
                {summary.alerting_count} near limit (&gt;80%)
              </span>
            )}
          </div>
          <Button variant="outline" size="sm" asChild>
            <a href="/ui/teams">Manage Budgets</a>
          </Button>
        </div>
      </AlertDescription>
    </Alert>
  )
}

// Budget Metrics Cards Component
function BudgetMetricsCards({ summary }: { summary: any }) {
  const utilizationRate = summary.total_budget > 0 ? (summary.total_spent / summary.total_budget) * 100 : 0

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
      <Card>
        <CardHeader>
          <CardDescription>Total Budget</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums">
            ${summary.total_budget?.toFixed(2) || '0.00'}
          </CardTitle>
          <div>
            <Badge variant="outline">
              <Users className="h-3 w-3" />
              {summary.total_entities || 0} entities
            </Badge>
          </div>
        </CardHeader>
        <CardFooter className="flex-col items-start gap-1.5 text-sm">
          <div className="text-muted-foreground">
            Teams: ${summary.team_budget?.toFixed(2) || '0'} • Keys: ${summary.key_budget?.toFixed(2) || '0'}
          </div>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardDescription>Total Spent</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums">
            ${summary.total_spent?.toFixed(2) || '0.00'}
          </CardTitle>
          <div>
            <Badge variant={utilizationRate > 90 ? "destructive" : utilizationRate > 70 ? "secondary" : "outline"}>
              <TrendingUp className="h-3 w-3" />
              {utilizationRate.toFixed(1)}%
            </Badge>
          </div>
        </CardHeader>
        <CardFooter className="flex-col items-start gap-1.5 text-sm">
          <div className="line-clamp-1 flex gap-2 font-medium">
            {utilizationRate > 80 ? "High utilization" : "Within limits"}
          </div>
          <div className="text-muted-foreground">
            Budget utilization rate
          </div>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardDescription>Remaining Budget</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums">
            ${summary.total_remaining?.toFixed(2) || '0.00'}
          </CardTitle>
          <div>
            <Badge variant="outline">
              <DollarSign className="h-3 w-3" />
              Available
            </Badge>
          </div>
        </CardHeader>
        <CardFooter className="flex-col items-start gap-1.5 text-sm">
          <div className="line-clamp-1 flex gap-2 font-medium">
            Budget remaining this period
          </div>
          <div className="text-muted-foreground">
            Across all teams and keys
          </div>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardDescription>Alert Status</CardDescription>
          <CardTitle className="text-2xl font-semibold tabular-nums">
            {summary.exceeded_count + summary.alerting_count}
          </CardTitle>
          <div>
            <Badge variant={summary.exceeded_count > 0 ? "destructive" : summary.alerting_count > 0 ? "secondary" : "outline"}>
              <AlertTriangle className="h-3 w-3" />
              Need attention
            </Badge>
          </div>
        </CardHeader>
        <CardFooter className="flex-col items-start gap-1.5 text-sm">
          <div className="line-clamp-1 flex gap-2 font-medium">
            {summary.exceeded_count} exceeded • {summary.alerting_count} alerting
          </div>
          <div className="text-muted-foreground">
            Entities requiring action
          </div>
        </CardFooter>
      </Card>
    </div>
  )
}

// Budget Distribution Charts
function BudgetDistributionCharts({ charts }: { charts: any }) {

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* Budget Allocation */}
      <Card>
        <CardHeader>
          <CardTitle>Budget Allocation</CardTitle>
          <CardDescription>Distribution by type</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-center pb-6">
          {charts?.budget_distribution?.some((item: any) => item.value > 0) ? (
            <ChartContainer
              config={chartConfig}
              className="mx-auto h-[250px] w-full max-w-[250px]"
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
              <DollarSign className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-muted-foreground">No budget allocation</p>
              <p className="text-sm text-muted-foreground mt-2">
                Chart will appear when budgets are allocated
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Spending Distribution */}
      <Card>
        <CardHeader>
          <CardTitle>Spending Distribution</CardTitle>
          <CardDescription>Actual spend by type</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-center pb-6">
          {charts?.spending_distribution?.some((item: any) => item.value > 0) ? (
            <ChartContainer
              config={chartConfig}
              className="mx-auto h-[250px] w-full max-w-[250px]"
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
              <DollarSign className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-muted-foreground">No spending yet</p>
              <p className="text-sm text-muted-foreground mt-2">
                Chart will appear when spending occurs
              </p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// Budget Entities Table Component  
function BudgetEntitiesTable({ teamBudgets, keyBudgets }: { teamBudgets: any[], keyBudgets: any[] }) {
  const allBudgets = [...(teamBudgets || []), ...(keyBudgets || [])]
  
  // Sort by utilization percentage (highest first)
  const sortedBudgets = allBudgets.sort((a, b) => {
    const aUtil = a.usage_percent || 0
    const bUtil = b.usage_percent || 0
    return bUtil - aUtil
  })

  if (sortedBudgets.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Budget Entities</CardTitle>
          <CardDescription>Teams and keys with budget allocation</CardDescription>
          <div>
            <Button variant="outline" size="sm" asChild>
              <a href="/ui/teams">
                <Plus className="h-4 w-4" />
                Create Team
              </a>
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <DollarSign className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">No budget entities configured</p>
            <p className="text-sm text-muted-foreground mt-2">
              Create teams or keys with budgets to track spending
            </p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Budget Entities</CardTitle>
        <CardDescription>
          {sortedBudgets.length} entities with budget tracking
        </CardDescription>
        <div>
          <Button variant="outline" size="sm" asChild>
            <a href="/ui/teams">
              <Settings className="h-4 w-4" />
              Manage
            </a>
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead className="text-right">Budget</TableHead>
                <TableHead className="text-right">Spent</TableHead>
                <TableHead className="text-right">Utilization</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Period</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedBudgets.map((budget) => {
                const utilization = budget.usage_percent || 0
                const isExceeded = budget.is_exceeded || false
                const shouldAlert = budget.should_alert || utilization > 80
                
                return (
                  <TableRow key={`${budget.type}-${budget.id}`}>
                    <TableCell className="font-medium">{budget.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {budget.type === 'team' ? <Users className="h-3 w-3" /> : <Key className="h-3 w-3" />}
                        {budget.type}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      ${budget.max_budget?.toFixed(2) || '0.00'}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      ${budget.current_spend?.toFixed(2) || '0.00'}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center gap-2">
                        <Progress value={Math.min(utilization, 100)} className="w-16" />
                        <span className="tabular-nums text-sm w-12">
                          {utilization.toFixed(1)}%
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={isExceeded ? "destructive" : shouldAlert ? "secondary" : "outline"}>
                        {isExceeded ? "Exceeded" : shouldAlert ? "Warning" : "OK"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right text-sm text-muted-foreground">
                      {budget.period || 'monthly'}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  )
}

// Main Budget Component
export default function Budget() {
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
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Budget Management</h1>
            <p className="text-muted-foreground">
              Monitor spending, track budgets, and manage allocations
            </p>
          </div>
        </div>
        <Alert variant="destructive">
          <AlertTriangle className="h-4 w-4" />
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
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">Budget Management</h1>
            <p className="text-muted-foreground">
              Monitor spending, track budgets, and manage allocations
            </p>
          </div>
        </div>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16 text-center">
            <DollarSign className="h-16 w-16 text-muted-foreground mb-4" />
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
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Budget Management</h1>
          <p className="text-muted-foreground">
            Monitor spending, track budgets, and manage allocations
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <a href="/ui/keys">
              <Plus className="h-4 w-4" />
              Add Key
            </a>
          </Button>
          <Button asChild>
            <a href="/ui/teams">
              <Plus className="h-4 w-4" />
              Add Team
            </a>
          </Button>
        </div>
      </div>

      {/* Budget Health Alert */}
      <BudgetHealthAlert summary={summary} />

      {/* Metrics Cards */}
      <BudgetMetricsCards summary={summary} />

      {/* Charts Section */}
      <BudgetDistributionCharts charts={charts} />

      {/* Budget Entities Table */}
      <BudgetEntitiesTable teamBudgets={team_budgets} keyBudgets={key_budgets} />
    </div>
  )
}