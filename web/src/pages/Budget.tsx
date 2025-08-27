import { useQuery } from "@tanstack/react-query";
import { getBudgetSummary, getUserBreakdown } from "@/lib/api";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import ReactECharts from "echarts-for-react";
import { Icon } from "@iconify/react";
import { useState, useEffect } from "react";


export default function Budget() {
  const [isDark, setIsDark] = useState(false);

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

  const { data: budgetData, isLoading } = useQuery({
    queryKey: ["budget-summary"],
    queryFn: getBudgetSummary,
    refetchInterval: 30000, // Refresh every 30 seconds
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

  const budget = budgetData as any; // axios interceptor extracts data
  const userBreakdown = userBreakdownData as any;
  
  if (!budget) {
    return <div>No budget data available</div>;
  }

  const { summary, team_budgets, key_budgets, usage_by_period, charts } = budget;
  const { user_breakdown = [], team_breakdown = [], summary: userSummary } = userBreakdown || {};

  // Format currency
  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(amount);
  };

  // Get status badge
  const getUsageBadge = (percent: number, isExceeded: boolean) => {
    if (isExceeded) {
      return <Badge variant="destructive">Exceeded</Badge>;
    } else if (percent >= 80) {
      return <Badge variant="secondary">Near Limit</Badge>;
    } else if (percent >= 50) {
      return <Badge variant="outline">In Use</Badge>;
    } else {
      return <Badge variant="default">Healthy</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-3xl font-bold">Budget Management</h1>
          <p className="text-muted-foreground">
            Monitor and manage team and API key budgets
          </p>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card className="border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Budget</CardTitle>
            <Icon icon="lucide:wallet" className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(summary.total_budget)}
            </div>
            <p className="text-xs text-muted-foreground">
              Across {summary.total_entities} entities
            </p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-orange-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Spent</CardTitle>
            <Icon icon="lucide:trending-up" className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(summary.total_spent)}
            </div>
            <p className="text-xs text-muted-foreground">
              {((summary.total_spent / summary.total_budget) * 100).toFixed(1)}% of budget
            </p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Remaining</CardTitle>
            <Icon icon="lucide:piggy-bank" className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(summary.total_remaining)}
            </div>
            <p className="text-xs text-muted-foreground">Available to spend</p>
          </CardContent>
        </Card>

        <Card className="border-l-4 border-l-red-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Alerts</CardTitle>
            <Icon icon="lucide:alert-triangle" className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {summary.alerting_count + summary.exceeded_count}
            </div>
            <p className="text-xs text-muted-foreground">
              {summary.exceeded_count} exceeded, {summary.alerting_count} warning
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* Budget Distribution */}
        <Card>
          <CardHeader>
            <CardTitle>Budget Distribution</CardTitle>
            <CardDescription>Budget allocation by entity type</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[300px]">
              <ReactECharts
                option={{
                  backgroundColor: "transparent",
                  tooltip: {
                    trigger: "item",
                    formatter: "{b}: {c} ({d}%)",
                    backgroundColor: isDark ? "hsl(215, 27.9%, 16.9%)" : "hsl(0, 0%, 100%)",
                    textStyle: {
                      color: isDark ? "hsl(210, 20%, 98%)" : "hsl(224, 71.4%, 4.1%)",
                    },
                  },
                  legend: {
                    orient: "vertical",
                    left: "left",
                    textStyle: {
                      color: isDark ? "hsl(210, 20%, 98%)" : "hsl(224, 71.4%, 4.1%)",
                    },
                  },
                  series: [
                    {
                      type: "pie",
                      radius: ["50%", "70%"],
                      center: ["65%", "50%"],
                      data: charts.budget_distribution.map((item: any, index: number) => ({
                        ...item,
                        itemStyle: {
                          color: index === 0 ? "#3b82f6" : "#10b981",
                        },
                      })),
                      emphasis: {
                        itemStyle: {
                          shadowBlur: 10,
                          shadowOffsetX: 0,
                          shadowColor: "rgba(0, 0, 0, 0.5)",
                        },
                      },
                    },
                  ],
                }}
                style={{ height: "100%", width: "100%" }}
              />
            </div>
          </CardContent>
        </Card>

        {/* Spending Distribution */}
        <Card>
          <CardHeader>
            <CardTitle>Spending Distribution</CardTitle>
            <CardDescription>Current spending by entity type</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[300px]">
              <ReactECharts
                option={{
                  backgroundColor: "transparent",
                  tooltip: {
                    trigger: "item",
                    formatter: "{b}: {c} ({d}%)",
                    backgroundColor: isDark ? "hsl(215, 27.9%, 16.9%)" : "hsl(0, 0%, 100%)",
                    textStyle: {
                      color: isDark ? "hsl(210, 20%, 98%)" : "hsl(224, 71.4%, 4.1%)",
                    },
                  },
                  legend: {
                    orient: "vertical",
                    left: "left",
                    textStyle: {
                      color: isDark ? "hsl(210, 20%, 98%)" : "hsl(224, 71.4%, 4.1%)",
                    },
                  },
                  series: [
                    {
                      type: "pie",
                      radius: ["50%", "70%"],
                      center: ["65%", "50%"],
                      data: charts.spending_distribution.map((item: any, index: number) => ({
                        ...item,
                        itemStyle: {
                          color: index === 0 ? "#f59e0b" : "#ef4444",
                        },
                      })),
                      emphasis: {
                        itemStyle: {
                          shadowBlur: 10,
                          shadowOffsetX: 0,
                          shadowColor: "rgba(0, 0, 0, 0.5)",
                        },
                      },
                    },
                  ],
                }}
                style={{ height: "100%", width: "100%" }}
              />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Budget Tables */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* Team Budgets */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Icon icon="lucide:users" className="h-5 w-5" />
              Team Budgets
            </CardTitle>
            <CardDescription>{team_budgets.length} teams with budgets configured</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {team_budgets.map((budget: any) => (
                <div
                  key={budget.id}
                  className="flex items-center justify-between p-4 rounded-lg border"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <h4 className="font-medium">{budget.name}</h4>
                      {getUsageBadge(budget.usage_percent, budget.is_exceeded)}
                    </div>
                    <div className="flex items-center gap-4 mt-2 text-sm text-muted-foreground">
                      <span>Budget: {formatCurrency(budget.max_budget)}</span>
                      <span>Spent: {formatCurrency(budget.current_spend)}</span>
                      <span>Period: {budget.period}</span>
                    </div>
                    <div className="mt-2">
                      <div className="w-full bg-muted rounded-full h-2">
                        <div
                          className={`h-2 rounded-full transition-all ${
                            budget.is_exceeded
                              ? "bg-red-500"
                              : budget.usage_percent >= 80
                              ? "bg-yellow-500"
                              : "bg-green-500"
                          }`}
                          style={{ width: `${Math.min(budget.usage_percent, 100)}%` }}
                        />
                      </div>
                      <p className="text-xs text-muted-foreground mt-1">
                        {budget.usage_percent.toFixed(1)}% used
                      </p>
                    </div>
                  </div>
                </div>
              ))}
              {team_budgets.length === 0 && (
                <p className="text-muted-foreground text-center py-8">
                  No team budgets configured
                </p>
              )}
            </div>
          </CardContent>
        </Card>

        {/* API Key Budgets */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Icon icon="lucide:key" className="h-5 w-5" />
              API Key Budgets
            </CardTitle>
            <CardDescription>{key_budgets.length} keys with budgets configured</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {key_budgets.map((budget: any) => (
                <div
                  key={budget.id}
                  className="flex items-center justify-between p-4 rounded-lg border"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <h4 className="font-medium">{budget.name}</h4>
                      {getUsageBadge(budget.usage_percent, budget.current_spend >= budget.max_budget)}
                    </div>
                    <div className="flex items-center gap-4 mt-2 text-sm text-muted-foreground">
                      <span>Budget: {formatCurrency(budget.max_budget)}</span>
                      <span>Spent: {formatCurrency(budget.current_spend)}</span>
                      <span>Requests: {budget.usage_count.toLocaleString()}</span>
                    </div>
                    <div className="mt-2">
                      <div className="w-full bg-muted rounded-full h-2">
                        <div
                          className={`h-2 rounded-full transition-all ${
                            budget.current_spend >= budget.max_budget
                              ? "bg-red-500"
                              : budget.usage_percent >= 80
                              ? "bg-yellow-500"
                              : "bg-green-500"
                          }`}
                          style={{ width: `${Math.min(budget.usage_percent, 100)}%` }}
                        />
                      </div>
                      <p className="text-xs text-muted-foreground mt-1">
                        {budget.usage_percent.toFixed(1)}% used
                      </p>
                    </div>
                  </div>
                </div>
              ))}
              {key_budgets.length === 0 && (
                <p className="text-muted-foreground text-center py-8">
                  No key budgets configured
                </p>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Period Usage Summary */}
      {usage_by_period.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Budget Usage by Period</CardTitle>
            <CardDescription>Breakdown of budgets by reset period</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {usage_by_period.map((period: any) => (
                <div key={period.period} className="p-4 rounded-lg border">
                  <h4 className="font-medium capitalize">{period.period}</h4>
                  <div className="mt-2 space-y-1">
                    <div className="flex justify-between">
                      <span className="text-sm text-muted-foreground">Count:</span>
                      <span className="text-sm font-medium">{period.count}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm text-muted-foreground">Budget:</span>
                      <span className="text-sm font-medium">{formatCurrency(period.budget)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-sm text-muted-foreground">Spent:</span>
                      <span className="text-sm font-medium">{formatCurrency(period.spent)}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* User Analytics Section */}
      {userBreakdown && (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
          {/* User Breakdown */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Icon icon="lucide:users" className="h-5 w-5" />
                Top Users by Spending
              </CardTitle>
              <CardDescription>
                {userSummary?.total_users || 0} active users this month
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                {user_breakdown.slice(0, 10).map((user: any) => (
                  <div
                    key={user.user_id}
                    className="flex items-center justify-between p-3 rounded-lg border"
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <h4 className="font-medium">
                          {user.user_name || user.user_email}
                        </h4>
                        {user.team_requests > 0 && user.user_requests > 0 && (
                          <Badge variant="outline">Mixed</Badge>
                        )}
                        {user.team_requests > 0 && user.user_requests === 0 && (
                          <Badge variant="secondary">Team User</Badge>
                        )}
                        {user.user_requests > 0 && user.team_requests === 0 && (
                          <Badge variant="default">Personal User</Badge>
                        )}
                      </div>
                      <div className="flex items-center gap-4 mt-2 text-sm text-muted-foreground">
                        <span>Cost: {formatCurrency(user.cost)}</span>
                        <span>Requests: {user.requests.toLocaleString()}</span>
                        <span>Tokens: {user.tokens.toLocaleString()}</span>
                      </div>
                      {user.team_requests > 0 && user.user_requests > 0 && (
                        <div className="flex items-center gap-4 mt-1 text-xs text-muted-foreground">
                          <span>Team: {user.team_requests.toLocaleString()} requests</span>
                          <span>Personal: {user.user_requests.toLocaleString()} requests</span>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
                {user_breakdown.length === 0 && (
                  <p className="text-muted-foreground text-center py-8">
                    No user activity this month
                  </p>
                )}
              </div>
            </CardContent>
          </Card>

          {/* Team User Breakdown */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Icon icon="lucide:building" className="h-5 w-5" />
                Team Activity
              </CardTitle>
              <CardDescription>
                User activity within teams
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {team_breakdown.map((team: any) => (
                  <div key={team.team_id} className="border rounded-lg p-4">
                    <div className="flex items-center justify-between mb-3">
                      <h4 className="font-medium">{team.team_name}</h4>
                      <Badge variant="outline">
                        {team.active_members}/{team.member_count} active
                      </Badge>
                    </div>
                    <div className="grid grid-cols-3 gap-2 text-sm mb-3">
                      <div>
                        <span className="text-muted-foreground">Cost:</span>
                        <span className="ml-1 font-medium">{formatCurrency(team.cost)}</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Requests:</span>
                        <span className="ml-1 font-medium">{team.requests.toLocaleString()}</span>
                      </div>
                      <div>
                        <span className="text-muted-foreground">Tokens:</span>
                        <span className="ml-1 font-medium">{team.tokens.toLocaleString()}</span>
                      </div>
                    </div>
                    {team.user_breakdown && Object.values(team.user_breakdown).length > 0 && (
                      <div className="space-y-2">
                        <p className="text-sm font-medium text-muted-foreground">Top Users:</p>
                        {Object.values(team.user_breakdown).slice(0, 3).map((user: any) => (
                          <div key={user.user_id} className="flex justify-between text-sm">
                            <span>{user.user_name || user.user_email}</span>
                            <span className="font-medium">{formatCurrency(user.cost)}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
                {team_breakdown.length === 0 && (
                  <p className="text-muted-foreground text-center py-8">
                    No team activity this month
                  </p>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}