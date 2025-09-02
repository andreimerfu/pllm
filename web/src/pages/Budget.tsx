import { useQuery } from "@tanstack/react-query";
import { getBudgetSummary, getUserBreakdown } from "@/lib/api";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { Separator } from "@/components/ui/separator";
import { Button } from "@/components/ui/button";
import { 
  Pie, 
  PieChart, 
  Area,
  AreaChart,
  CartesianGrid, 
  XAxis,
  Label
} from "recharts";
import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { 
  TrendingUp, 
  TrendingDown,
  PiggyBank, 
  AlertTriangle, 
  Users, 
  Key, 
  Building, 
  Target,
  Settings,
  DollarSign,
  Wallet,
  Activity,
  BarChart3
} from "lucide-react";
import { EmptyState } from "@/components/EmptyState";

export default function Budget() {
  const { data: budgetData, isLoading } = useQuery({
    queryKey: ["budget-summary"],
    queryFn: getBudgetSummary,
    refetchInterval: 30000,
  });

  const { data: userBreakdownData, isLoading: isLoadingUsers } = useQuery({
    queryKey: ["user-breakdown"],
    queryFn: getUserBreakdown,
    refetchInterval: 30000,
  });

  if (isLoading || isLoadingUsers) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  const budget = budgetData as any;
  const userBreakdown = userBreakdownData as any;
  
  if (!budget) {
    return (
      <div className="flex items-center justify-center h-64">
        <EmptyState
          icon="lucide:wallet"
          title="No budget data available"
          description="Budget information will appear here once budgets are configured."
          action={{ label: "Configure Budgets", href: "/ui/settings" }}
        />
      </div>
    );
  }

  const { summary, team_budgets, key_budgets, usage_by_period, charts } = budget;
  const { user_breakdown = [], team_breakdown = [], summary: userSummary } = userBreakdown || {};

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(amount);
  };

  const chartConfig: ChartConfig = {
    teams: { label: "Teams", color: "hsl(var(--chart-1))" },
    keys: { label: "API Keys", color: "hsl(var(--chart-2))" },
    budget: { label: "Budget", color: "hsl(var(--chart-3))" },
    spent: { label: "Spent", color: "hsl(var(--chart-4))" },
    remaining: { label: "Remaining", color: "hsl(var(--chart-5))" },
  };

  const getUsageBadge = (percent: number, isExceeded: boolean) => {
    if (isExceeded) return <Badge variant="destructive">Exceeded</Badge>;
    if (percent >= 80) return <Badge variant="secondary">Near Limit</Badge>;
    if (percent >= 50) return <Badge variant="outline">Active</Badge>;
    return <Badge variant="default">Healthy</Badge>;
  };


  // Prepare chart data
  const budgetDistributionData = charts?.budget_distribution?.map((item: any, index: number) => ({
    ...item,
    fill: `hsl(var(--chart-${(index % 5) + 1}))`,
  })) || [];

  const spendingDistributionData = charts?.spending_distribution?.map((item: any, index: number) => ({
    ...item,
    fill: `hsl(var(--chart-${(index % 5) + 1}))`,
  })) || [];

  const periodData = usage_by_period?.map((period: any) => ({
    period: period.period.charAt(0).toUpperCase() + period.period.slice(1),
    budget: period.budget,
    spent: period.spent,
    remaining: Math.max(0, period.budget - period.spent),
    count: period.count,
  })) || [];

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
        <Button variant="outline" asChild>
          <a href="/ui/settings">
            <Settings className="h-4 w-4 mr-2" />
            Manage Budgets
          </a>
        </Button>
      </div>

      {/* Summary Cards */}
      <div className="*:data-[slot=card]:from-primary/5 *:data-[slot=card]:to-card dark:*:data-[slot=card]:bg-card grid grid-cols-1 gap-4 *:data-[slot=card]:bg-gradient-to-t *:data-[slot=card]:shadow-xs md:grid-cols-2 lg:grid-cols-4">
        <Card className="@container/card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
            <div className="space-y-1">
              <CardDescription>Total Budget</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                {formatCurrency(summary.total_budget)}
              </CardTitle>
            </div>
            <Badge variant="outline">
              <Target className="h-3 w-3" />
              {summary.total_entities} entities
            </Badge>
          </CardHeader>
          <CardFooter className="flex-col items-start gap-1.5 text-sm">
            <div className="line-clamp-1 flex gap-2 font-medium">
              Total allocated budget <Wallet className="size-4" />
            </div>
            <div className="text-muted-foreground">
              Across all teams and keys
            </div>
          </CardFooter>
        </Card>

        <Card className="@container/card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
            <div className="space-y-1">
              <CardDescription>Total Spent</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                {formatCurrency(summary.total_spent)}
              </CardTitle>
            </div>
            <Badge variant="outline" className={summary.total_spent / summary.total_budget > 0.8 ? "border-orange-200 text-orange-700" : ""}>
              {summary.total_spent > summary.total_budget ? (
                <TrendingDown className="h-3 w-3" />
              ) : (
                <TrendingUp className="h-3 w-3" />
              )}
              {((summary.total_spent / summary.total_budget) * 100).toFixed(1)}%
            </Badge>
          </CardHeader>
          <CardFooter className="flex-col items-start gap-1.5 text-sm">
            <Progress 
              value={Math.min((summary.total_spent / summary.total_budget) * 100, 100)} 
              className="w-full h-2"
            />
            <div className="flex w-full justify-between text-xs text-muted-foreground">
              <span>Used: {((summary.total_spent / summary.total_budget) * 100).toFixed(1)}%</span>
              <span>{formatCurrency(summary.total_budget - summary.total_spent)} left</span>
            </div>
          </CardFooter>
        </Card>

        <Card className="@container/card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
            <div className="space-y-1">
              <CardDescription>Remaining Budget</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                {formatCurrency(summary.total_remaining)}
              </CardTitle>
            </div>
            <Badge variant="outline">
              <PiggyBank className="h-3 w-3" />
              Available
            </Badge>
          </CardHeader>
          <CardFooter className="flex-col items-start gap-1.5 text-sm">
            <div className="line-clamp-1 flex gap-2 font-medium">
              Ready to spend <DollarSign className="size-4" />
            </div>
            <div className="text-muted-foreground">
              {((summary.total_remaining / summary.total_budget) * 100).toFixed(1)}% of total budget
            </div>
          </CardFooter>
        </Card>

        <Card className="@container/card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
            <div className="space-y-1">
              <CardDescription>Budget Alerts</CardDescription>
              <CardTitle className="text-2xl font-semibold tabular-nums @[250px]/card:text-3xl">
                {summary.alerting_count + summary.exceeded_count}
              </CardTitle>
            </div>
            <Badge variant={summary.exceeded_count > 0 ? "destructive" : summary.alerting_count > 0 ? "secondary" : "outline"}>
              <AlertTriangle className="h-3 w-3" />
              {summary.exceeded_count > 0 ? "Critical" : summary.alerting_count > 0 ? "Warning" : "Healthy"}
            </Badge>
          </CardHeader>
          <CardFooter className="flex-col items-start gap-1.5 text-sm">
            <div className="line-clamp-1 flex gap-2 font-medium">
              {summary.exceeded_count > 0 ? (
                <>{summary.exceeded_count} exceeded <TrendingUp className="size-4" /></>
              ) : summary.alerting_count > 0 ? (
                <>{summary.alerting_count} near limit <Activity className="size-4" /></>
              ) : (
                <>All budgets healthy <Activity className="size-4" /></>
              )}
            </div>
            <div className="text-muted-foreground">
              {summary.exceeded_count} exceeded, {summary.alerting_count} warning
            </div>
          </CardFooter>
        </Card>
      </div>

      {/* Charts Grid */}
      <div className="grid gap-4 md:grid-cols-2">
        {/* Budget Distribution */}
        <Card className="flex flex-col">
          <CardHeader className="items-center pb-0">
            <CardTitle>Budget Distribution</CardTitle>
            <CardDescription>
              How budgets are allocated across entity types
            </CardDescription>
          </CardHeader>
          <CardContent className="flex-1 pb-0">
            {budgetDistributionData.length === 0 ? (
              <EmptyState
                variant="chart"
                icon="lucide:pie-chart"
                title="No budget data"
                description="Budget distribution will appear here once budgets are configured."
                action={{ label: "Configure Budgets", href: "/ui/settings" }}
              />
            ) : (
              <ChartContainer
                config={chartConfig}
                className="mx-auto aspect-square max-h-[250px]"
              >
                <PieChart>
                  <ChartTooltip
                    cursor={false}
                    content={<ChartTooltipContent hideLabel />}
                  />
                  <Pie
                    data={budgetDistributionData}
                    dataKey="value"
                    nameKey="name"
                    innerRadius={60}
                    strokeWidth={5}
                  >
                    <Label
                      content={({ viewBox }) => {
                        if (viewBox && "cx" in viewBox && "cy" in viewBox) {
                          const totalBudget = budgetDistributionData.reduce(
                            (acc: number, curr: any) => acc + curr.value,
                            0
                          );
                          return (
                            <text
                              x={viewBox.cx}
                              y={viewBox.cy}
                              textAnchor="middle"
                              dominantBaseline="middle"
                            >
                              <tspan
                                x={viewBox.cx}
                                y={viewBox.cy}
                                className="fill-foreground text-2xl font-bold"
                              >
                                {formatCurrency(totalBudget)}
                              </tspan>
                              <tspan
                                x={viewBox.cx}
                                y={(viewBox.cy || 0) + 24}
                                className="fill-muted-foreground"
                              >
                                Total Budget
                              </tspan>
                            </text>
                          );
                        }
                      }}
                    />
                  </Pie>
                </PieChart>
              </ChartContainer>
            )}
          </CardContent>
          {budgetDistributionData.length > 0 && (
            <CardFooter className="flex-col gap-2 text-sm">
              <div className="flex flex-wrap justify-center gap-4">
                {budgetDistributionData.map((item: any) => (
                  <div key={item.name} className="flex items-center gap-2">
                    <div 
                      className="w-3 h-3 rounded-full" 
                      style={{ backgroundColor: item.fill }}
                    />
                    <span className="text-sm font-medium">{item.name}</span>
                    <span className="text-sm text-muted-foreground">
                      ({formatCurrency(item.value)})
                    </span>
                  </div>
                ))}
              </div>
            </CardFooter>
          )}
        </Card>

        {/* Spending Distribution */}
        <Card className="flex flex-col">
          <CardHeader className="items-center pb-0">
            <CardTitle>Spending Distribution</CardTitle>
            <CardDescription>
              Current spending breakdown by entity type
            </CardDescription>
          </CardHeader>
          <CardContent className="flex-1 pb-0">
            {spendingDistributionData.length === 0 ? (
              <EmptyState
                variant="chart"
                icon="lucide:pie-chart"
                title="No spending data"
                description="Spending distribution will appear here once there's usage activity."
                action={{ label: "View Usage", href: "/ui/dashboard" }}
              />
            ) : (
              <ChartContainer
                config={chartConfig}
                className="mx-auto aspect-square max-h-[250px]"
              >
                <PieChart>
                  <ChartTooltip
                    cursor={false}
                    content={<ChartTooltipContent hideLabel />}
                  />
                  <Pie
                    data={spendingDistributionData}
                    dataKey="value"
                    nameKey="name"
                    innerRadius={60}
                    strokeWidth={5}
                    activeIndex={0}
                  >
                    <Label
                      content={({ viewBox }) => {
                        if (viewBox && "cx" in viewBox && "cy" in viewBox) {
                          const totalSpent = spendingDistributionData.reduce(
                            (acc: number, curr: any) => acc + curr.value,
                            0
                          );
                          return (
                            <text
                              x={viewBox.cx}
                              y={viewBox.cy}
                              textAnchor="middle"
                              dominantBaseline="middle"
                            >
                              <tspan
                                x={viewBox.cx}
                                y={viewBox.cy}
                                className="fill-foreground text-2xl font-bold"
                              >
                                {formatCurrency(totalSpent)}
                              </tspan>
                              <tspan
                                x={viewBox.cx}
                                y={(viewBox.cy || 0) + 24}
                                className="fill-muted-foreground"
                              >
                                Total Spent
                              </tspan>
                            </text>
                          );
                        }
                      }}
                    />
                  </Pie>
                </PieChart>
              </ChartContainer>
            )}
          </CardContent>
          {spendingDistributionData.length > 0 && (
            <CardFooter className="flex-col gap-2 text-sm">
              <div className="flex flex-wrap justify-center gap-4">
                {spendingDistributionData.map((item: any) => (
                  <div key={item.name} className="flex items-center gap-2">
                    <div 
                      className="w-3 h-3 rounded-full" 
                      style={{ backgroundColor: item.fill }}
                    />
                    <span className="text-sm font-medium">{item.name}</span>
                    <span className="text-sm text-muted-foreground">
                      ({formatCurrency(item.value)})
                    </span>
                  </div>
                ))}
              </div>
            </CardFooter>
          )}
        </Card>
      </div>

      {/* Budget Usage by Period - Enhanced Area Chart */}
      {periodData.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Budget Usage Trends</CardTitle>
            <CardDescription>
              Spending patterns and budget utilization over time
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ChartContainer config={chartConfig} className="aspect-auto h-[300px] w-full">
              <AreaChart
                accessibilityLayer
                data={periodData}
                margin={{
                  left: 12,
                  right: 12,
                }}
              >
                <defs>
                  <linearGradient id="fillBudget" x1="0" y1="0" x2="0" y2="1">
                    <stop
                      offset="5%"
                      stopColor="var(--color-budget)"
                      stopOpacity={0.8}
                    />
                    <stop
                      offset="95%"
                      stopColor="var(--color-budget)"
                      stopOpacity={0.1}
                    />
                  </linearGradient>
                  <linearGradient id="fillSpent" x1="0" y1="0" x2="0" y2="1">
                    <stop
                      offset="5%"
                      stopColor="var(--color-spent)"
                      stopOpacity={0.8}
                    />
                    <stop
                      offset="95%"
                      stopColor="var(--color-spent)"
                      stopOpacity={0.1}
                    />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey="period"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                />
                <ChartTooltip
                  cursor={false}
                  content={
                    <ChartTooltipContent
                      labelFormatter={(value) => {
                        return `${value} Period`;
                      }}
                      indicator="dot"
                    />
                  }
                />
                <Area
                  dataKey="budget"
                  type="natural"
                  fill="url(#fillBudget)"
                  fillOpacity={0.4}
                  stroke="var(--color-budget)"
                  stackId="a"
                />
                <Area
                  dataKey="spent"
                  type="natural"
                  fill="url(#fillSpent)"
                  fillOpacity={0.4}
                  stroke="var(--color-spent)"
                  stackId="a"
                />
                <ChartLegend content={<ChartLegendContent />} />
              </AreaChart>
            </ChartContainer>
          </CardContent>
          <CardFooter>
            <div className="flex w-full items-start gap-2 text-sm">
              <div className="grid gap-2">
                <div className="flex items-center gap-2 leading-none font-medium">
                  Budget utilization trends <BarChart3 className="h-4 w-4" />
                </div>
                <div className="text-muted-foreground flex items-center gap-2 leading-none">
                  Tracking spending across reset periods
                </div>
              </div>
            </div>
          </CardFooter>
        </Card>
      )}

      {/* Budget Details */}
      <div className="grid gap-4 md:grid-cols-2">
        {/* Team Budgets */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Users className="h-5 w-5" />
              Team Budgets
            </CardTitle>
            <CardDescription>
              {team_budgets.length} teams with configured budgets
            </CardDescription>
          </CardHeader>
          <CardContent>
            {team_budgets.length === 0 ? (
              <EmptyState
                icon="lucide:users"
                title="No team budgets"
                description="Team budget tracking will appear here once budgets are configured."
                action={{ label: "Configure Teams", href: "/ui/settings" }}
              />
            ) : (
              <div className="space-y-4">
                {team_budgets.map((teamBudget: any) => (
                  <Card key={teamBudget.id} className="border-l-4 border-l-primary">
                    <CardContent className="p-4">
                      <div className="flex items-start justify-between">
                        <div className="space-y-1 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="font-semibold">{teamBudget.name}</h3>
                            {getUsageBadge(teamBudget.usage_percent, teamBudget.is_exceeded)}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            Period: {teamBudget.period} • Budget: {formatCurrency(teamBudget.max_budget)}
                          </div>
                        </div>
                        <div className="text-right">
                          <div className="text-lg font-bold">
                            {formatCurrency(teamBudget.current_spend)}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {teamBudget.usage_percent.toFixed(1)}% used
                          </div>
                        </div>
                      </div>
                      
                      <Separator className="my-3" />
                      
                      <div className="space-y-2">
                        <Progress 
                          value={Math.min(teamBudget.usage_percent, 100)}
                          className="h-2"
                        />
                        <div className="flex justify-between text-sm text-muted-foreground">
                          <span>{formatCurrency(teamBudget.current_spend)} spent</span>
                          <span>{formatCurrency(teamBudget.max_budget - teamBudget.current_spend)} remaining</span>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* API Key Budgets */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Key className="h-5 w-5" />
              API Key Budgets
            </CardTitle>
            <CardDescription>
              {key_budgets.length} keys with configured budgets
            </CardDescription>
          </CardHeader>
          <CardContent>
            {key_budgets.length === 0 ? (
              <EmptyState
                icon="lucide:key"
                title="No key budgets"
                description="API key budget tracking will appear here once budgets are configured."
                action={{ label: "Configure Keys", href: "/ui/settings" }}
              />
            ) : (
              <div className="space-y-4">
                {key_budgets.map((keyBudget: any) => (
                  <Card key={keyBudget.id} className="border-l-4 border-l-secondary">
                    <CardContent className="p-4">
                      <div className="flex items-start justify-between">
                        <div className="space-y-1 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="font-semibold">{keyBudget.name}</h3>
                            {getUsageBadge(keyBudget.usage_percent, keyBudget.current_spend >= keyBudget.max_budget)}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            Budget: {formatCurrency(keyBudget.max_budget)} • {keyBudget.usage_count.toLocaleString()} requests
                          </div>
                        </div>
                        <div className="text-right">
                          <div className="text-lg font-bold">
                            {formatCurrency(keyBudget.current_spend)}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {keyBudget.usage_percent.toFixed(1)}% used
                          </div>
                        </div>
                      </div>
                      
                      <Separator className="my-3" />
                      
                      <div className="space-y-2">
                        <Progress 
                          value={Math.min(keyBudget.usage_percent, 100)}
                          className="h-2"
                        />
                        <div className="flex justify-between text-sm text-muted-foreground">
                          <span>{formatCurrency(keyBudget.current_spend)} spent</span>
                          <span>{formatCurrency(keyBudget.max_budget - keyBudget.current_spend)} remaining</span>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* User Analytics */}
      {userBreakdown && (user_breakdown.length > 0 || team_breakdown.length > 0) && (
        <div className="grid gap-4 md:grid-cols-2">
          {/* Top Users */}
          {user_breakdown.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Users className="h-5 w-5" />
                  Top Users by Spending
                </CardTitle>
                <CardDescription>
                  {userSummary?.total_users || 0} active users this month
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {user_breakdown.slice(0, 8).map((user: any) => (
                    <div
                      key={user.user_id}
                      className="flex items-center justify-between p-3 rounded-lg border"
                    >
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="font-medium truncate">
                            {user.user_name || user.user_email}
                          </p>
                          {user.team_requests > 0 && user.user_requests > 0 && (
                            <Badge variant="outline" className="text-xs">Mixed</Badge>
                          )}
                          {user.team_requests > 0 && user.user_requests === 0 && (
                            <Badge variant="secondary" className="text-xs">Team</Badge>
                          )}
                          {user.user_requests > 0 && user.team_requests === 0 && (
                            <Badge variant="default" className="text-xs">Personal</Badge>
                          )}
                        </div>
                        <div className="flex items-center gap-4 text-sm text-muted-foreground">
                          <span>Cost: {formatCurrency(user.cost)}</span>
                          <span>{user.requests.toLocaleString()} requests</span>
                          <span>{user.tokens.toLocaleString()} tokens</span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}

          {/* Team Activity */}
          {team_breakdown.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Building className="h-5 w-5" />
                  Team Activity
                </CardTitle>
                <CardDescription>
                  User activity breakdown by teams
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {team_breakdown.map((team: any) => (
                    <Card key={team.team_id} className="border">
                      <CardContent className="p-4">
                        <div className="flex items-center justify-between mb-3">
                          <h3 className="font-semibold">{team.team_name}</h3>
                          <Badge variant="outline">
                            {team.active_members}/{team.member_count} active
                          </Badge>
                        </div>
                        
                        <div className="grid grid-cols-3 gap-2 text-sm mb-3">
                          <div>
                            <span className="text-muted-foreground">Cost</span>
                            <p className="font-medium">{formatCurrency(team.cost)}</p>
                          </div>
                          <div>
                            <span className="text-muted-foreground">Requests</span>
                            <p className="font-medium">{team.requests.toLocaleString()}</p>
                          </div>
                          <div>
                            <span className="text-muted-foreground">Tokens</span>
                            <p className="font-medium">{team.tokens.toLocaleString()}</p>
                          </div>
                        </div>
                        
                        {team.user_breakdown && Object.values(team.user_breakdown).length > 0 && (
                          <>
                            <Separator className="my-3" />
                            <div>
                              <p className="text-sm font-medium text-muted-foreground mb-2">Top Users</p>
                              <div className="space-y-1">
                                {Object.values(team.user_breakdown).slice(0, 3).map((user: any) => (
                                  <div key={user.user_id} className="flex justify-between text-sm">
                                    <span className="truncate">{user.user_name || user.user_email}</span>
                                    <span className="font-medium ml-2">{formatCurrency(user.cost)}</span>
                                  </div>
                                ))}
                              </div>
                            </div>
                          </>
                        )}
                      </CardContent>
                    </Card>
                  ))}
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      )}
    </div>
  );
}