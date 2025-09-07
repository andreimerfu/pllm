import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Shield, Activity, Clock, AlertTriangle, CheckCircle, XCircle } from "lucide-react";

import { getGuardrails, getGuardrailStats, checkGuardrailHealth } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

interface GuardrailInfo {
  name: string;
  provider: string;
  mode: string[];
  enabled: boolean;
  default_on: boolean;
  config: Record<string, any>;
  stats?: {
    total_executions: number;
    total_passed: number;
    total_blocked: number;
    total_errors: number;
    average_latency: number;
    last_executed: string;
    block_rate: number;
    error_rate: number;
  };
  healthy: boolean;
}


export default function Guardrails() {
  const [activeTab, setActiveTab] = useState("overview");

  const { data: guardrailsData, isLoading } = useQuery({
    queryKey: ["guardrails"],
    queryFn: getGuardrails,
  });

  const { data: statsData } = useQuery({
    queryKey: ["guardrails-stats"],
    queryFn: getGuardrailStats,
  });

  const { data: healthData } = useQuery({
    queryKey: ["guardrails-health"],
    queryFn: checkGuardrailHealth,
    refetchInterval: 30000, // Check health every 30 seconds
  });

  const guardrails = (guardrailsData as any)?.guardrails || [];
  const systemEnabled = (guardrailsData as any)?.enabled ?? false;
  const stats = (statsData as any)?.stats || {};
  const health = (healthData as any)?.health || {};
  const allHealthy = (healthData as any)?.all_healthy ?? false;
  const checkedAt = (healthData as any)?.checked_at;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  const formatLatency = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  };

  const formatRate = (rate: number) => {
    return `${(rate * 100).toFixed(1)}%`;
  };

  const getHealthIcon = (healthy: boolean) => {
    return healthy ? (
      <CheckCircle className="h-4 w-4 text-green-500" />
    ) : (
      <XCircle className="h-4 w-4 text-red-500" />
    );
  };

  const getModeColor = (mode: string) => {
    switch (mode) {
      case "pre_call":
        return "bg-blue-100 text-blue-800";
      case "post_call":
        return "bg-green-100 text-green-800";
      case "during_call":
        return "bg-yellow-100 text-yellow-800";
      case "logging_only":
        return "bg-gray-100 text-gray-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  };

  return (
    <div className="space-y-4 lg:space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">Guardrails</h1>
          <p className="text-sm lg:text-base text-muted-foreground">
            Monitor and manage content safety guardrails
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={systemEnabled ? "default" : "secondary"}>
            {systemEnabled ? "Enabled" : "Disabled"}
          </Badge>
        </div>
      </div>

      {/* System Status Alert */}
      {!systemEnabled && (
        <Card className="border-yellow-200 bg-yellow-50">
          <CardContent className="pt-6">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-yellow-600" />
              <p className="text-sm text-yellow-800">
                Guardrails system is currently disabled. Enable it in the configuration to start protecting your LLM interactions.
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="statistics">Statistics</TabsTrigger>
          <TabsTrigger value="health">Health</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          {guardrails.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-12">
                <Shield className="h-12 w-12 text-muted-foreground mb-4" />
                <h3 className="text-lg font-semibold mb-2">No Guardrails Configured</h3>
                <p className="text-muted-foreground text-center max-w-md">
                  No guardrails are currently configured. Add guardrails to your configuration to start protecting your LLM interactions.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-4">
              {guardrails.map((guardrail: GuardrailInfo) => (
                <Card key={guardrail.name}>
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Shield className="h-5 w-5" />
                        <CardTitle className="text-lg">{guardrail.name}</CardTitle>
                        {getHealthIcon(guardrail.healthy)}
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge variant={guardrail.enabled ? "default" : "secondary"}>
                          {guardrail.enabled ? "Enabled" : "Disabled"}
                        </Badge>
                        {guardrail.default_on && (
                          <Badge variant="outline">Default On</Badge>
                        )}
                      </div>
                    </div>
                    <CardDescription>
                      Provider: {guardrail.provider}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-4">
                      {/* Execution Modes */}
                      <div>
                        <h4 className="text-sm font-medium mb-2">Execution Modes</h4>
                        <div className="flex flex-wrap gap-2">
                          {guardrail.mode.map((mode: string) => (
                            <Badge key={mode} className={getModeColor(mode)}>
                              {mode.replace('_', ' ')}
                            </Badge>
                          ))}
                        </div>
                      </div>

                      {/* Statistics */}
                      {guardrail.stats && (
                        <div>
                          <h4 className="text-sm font-medium mb-2">Quick Stats</h4>
                          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
                            <div>
                              <div className="text-muted-foreground">Executions</div>
                              <div className="font-semibold">{guardrail.stats.total_executions.toLocaleString()}</div>
                            </div>
                            <div>
                              <div className="text-muted-foreground">Block Rate</div>
                              <div className="font-semibold">{formatRate(guardrail.stats.block_rate)}</div>
                            </div>
                            <div>
                              <div className="text-muted-foreground">Error Rate</div>
                              <div className="font-semibold">{formatRate(guardrail.stats.error_rate)}</div>
                            </div>
                            <div>
                              <div className="text-muted-foreground">Avg Latency</div>
                              <div className="font-semibold">{formatLatency(guardrail.stats.average_latency)}</div>
                            </div>
                          </div>
                        </div>
                      )}

                      {/* Configuration Preview */}
                      {guardrail.config && Object.keys(guardrail.config).length > 0 && (
                        <div>
                          <h4 className="text-sm font-medium mb-2">Configuration</h4>
                          <div className="text-xs bg-muted p-3 rounded-md font-mono">
                            {Object.entries(guardrail.config).slice(0, 3).map(([key, value]) => (
                              <div key={key}>
                                <span className="text-muted-foreground">{key}:</span> {JSON.stringify(value)}
                              </div>
                            ))}
                            {Object.keys(guardrail.config).length > 3 && (
                              <div className="text-muted-foreground">
                                ... and {Object.keys(guardrail.config).length - 3} more
                              </div>
                            )}
                          </div>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="statistics" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Activity className="h-5 w-5" />
                Execution Statistics
              </CardTitle>
              <CardDescription>
                Performance metrics for all guardrails
              </CardDescription>
            </CardHeader>
            <CardContent>
              {!stats || Object.keys(stats).length === 0 ? (
                <div className="text-center text-muted-foreground py-8">
                  No statistics available yet
                </div>
              ) : (
                <div className="space-y-6">
                  {Object.entries(stats as Record<string, any>).map(([name, stats]) => (
                    <div key={name} className="border-b pb-4 last:border-b-0">
                      <h4 className="font-medium mb-3">{name}</h4>
                      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
                        <div className="text-center">
                          <div className="text-2xl font-bold text-blue-600">{stats.total_executions.toLocaleString()}</div>
                          <div className="text-muted-foreground">Total Executions</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold text-green-600">{stats.total_passed.toLocaleString()}</div>
                          <div className="text-muted-foreground">Passed</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold text-red-600">{stats.total_blocked.toLocaleString()}</div>
                          <div className="text-muted-foreground">Blocked</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold text-yellow-600">{stats.total_errors.toLocaleString()}</div>
                          <div className="text-muted-foreground">Errors</div>
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-4 mt-4 text-sm">
                        <div className="text-center">
                          <div className="text-xl font-semibold">{formatRate(stats.block_rate)}</div>
                          <div className="text-muted-foreground">Block Rate</div>
                        </div>
                        <div className="text-center">
                          <div className="text-xl font-semibold">{formatLatency(stats.average_latency)}</div>
                          <div className="text-muted-foreground">Avg Latency</div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="health" className="space-y-4">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center gap-2">
                    <Activity className="h-5 w-5" />
                    Health Status
                  </CardTitle>
                  <CardDescription>
                    Real-time health monitoring for all guardrails
                  </CardDescription>
                </div>
                <Button variant="outline" onClick={() => window.location.reload()}>
                  Refresh
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {!health || Object.keys(health).length === 0 ? (
                <div className="text-center text-muted-foreground py-8">
                  Health check unavailable
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="flex items-center gap-2 mb-4">
                    {allHealthy ? (
                      <CheckCircle className="h-5 w-5 text-green-500" />
                    ) : (
                      <XCircle className="h-5 w-5 text-red-500" />
                    )}
                    <span className={`font-medium ${allHealthy ? 'text-green-700' : 'text-red-700'}`}>
                      {allHealthy ? 'All Guardrails Healthy' : 'Some Guardrails Unhealthy'}
                    </span>
                  </div>
                  
                  <div className="space-y-3">
                    {Object.entries(health as Record<string, any>).map(([name, status]) => (
                      <div key={name} className="flex items-center justify-between p-3 border rounded-md">
                        <div className="flex items-center gap-2">
                          {getHealthIcon(status.healthy)}
                          <span className="font-medium">{name}</span>
                        </div>
                        <div className="text-sm text-muted-foreground">
                          {status.healthy ? 'Healthy' : status.error}
                        </div>
                      </div>
                    ))}
                  </div>
                  
                  {checkedAt && (
                    <div className="flex items-center gap-1 text-xs text-muted-foreground mt-4">
                      <Clock className="h-3 w-3" />
                      Last checked: {new Date(checkedAt).toLocaleString()}
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}