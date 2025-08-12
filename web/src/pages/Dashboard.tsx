import { useQuery } from '@tanstack/react-query'
import { getModelStats, getModels } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import ReactECharts from 'echarts-for-react'
import { Icon } from '@iconify/react'
import { useState, useEffect } from 'react'
import { Tooltip as UITooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

export default function Dashboard() {
  const [isDark, setIsDark] = useState(false)
  
  useEffect(() => {
    const checkTheme = () => {
      setIsDark(document.documentElement.classList.contains('dark'))
    }
    checkTheme()
    
    const observer = new MutationObserver(checkTheme)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class']
    })
    
    return () => observer.disconnect()
  }, [])

  const { data: statsData, isLoading: statsLoading } = useQuery({
    queryKey: ['model-stats'],
    queryFn: getModelStats,
    refetchInterval: 5000, // Refresh every 5 seconds
  })

  const { data: modelsData } = useQuery({
    queryKey: ['models'],
    queryFn: getModels,
  })

  const stats = statsData?.data
  const models = modelsData?.data?.data || []

  // Calculate summary metrics
  const totalRequests = Object.values(stats?.load_balancer || {}).reduce(
    (sum: number, model: any) => sum + (model.total_requests || 0),
    0
  )

  const activeModels = Object.values(stats?.load_balancer || {}).filter(
    (model: any) => !model.circuit_open
  ).length

  const avgHealthScore = Object.values(stats?.load_balancer || {}).reduce(
    (sum: number, model: any, _, arr) => sum + model.health_score / arr.length,
    0
  )

  // Prepare chart data
  const modelHealthData = Object.entries(stats?.load_balancer || {}).map(([name, data]: [string, any]) => ({
    name: name.replace('my-', ''),
    health: Math.round(data.health_score),
    requests: data.total_requests,
    latency: parseInt(data.avg_latency) || 0,
  }))

  const pieData = modelHealthData.map(m => ({
    name: m.name,
    value: m.requests,
  }))

  const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#06b6d4', '#84cc16', '#f97316']

  // Enhanced provider detection function
  const getProviderInfo = (modelName: string) => {
    const name = modelName.toLowerCase();
    
    if (name.includes('gpt') || name.includes('openai')) {
      return {
        icon: 'logos:openai',
        name: 'OpenAI',
        color: 'text-emerald-600 dark:text-emerald-400'
      };
    }
    if (name.includes('claude') || name.includes('anthropic')) {
      return {
        icon: 'logos:anthropic', 
        name: 'Anthropic',
        color: 'text-orange-600 dark:text-orange-400'
      };
    }
    if (name.includes('mistral')) {
      return {
        icon: 'logos:mistral',
        name: 'Mistral AI', 
        color: 'text-blue-600 dark:text-blue-400'
      };
    }
    if (name.includes('llama') || name.includes('meta')) {
      return {
        icon: 'logos:meta',
        name: 'Meta',
        color: 'text-indigo-600 dark:text-indigo-400'
      };
    }
    if (name.includes('gemini') || name.includes('google')) {
      return {
        icon: 'logos:google',
        name: 'Google',
        color: 'text-red-600 dark:text-red-400'
      };
    }
    if (name.includes('azure') || name.includes('microsoft')) {
      return {
        icon: 'logos:microsoft',
        name: 'Microsoft',
        color: 'text-blue-700 dark:text-blue-300'
      };
    }
    if (name.includes('bedrock') || name.includes('aws')) {
      return {
        icon: 'logos:aws',
        name: 'AWS',
        color: 'text-yellow-600 dark:text-yellow-400'
      };
    }
    
    return {
      icon: 'lucide:brain',
      name: 'Unknown',
      color: 'text-muted-foreground'
    };
  };

  if (statsLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground">Real-time monitoring and analytics</p>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card className="transition-theme border-l-4 border-l-blue-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
            <div className="h-8 w-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
              <Icon icon="lucide:activity" width="16" height="16" className="text-blue-500" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">{totalRequests.toLocaleString()}</div>
            <p className="text-xs text-muted-foreground">
              Across all models
            </p>
          </CardContent>
        </Card>

        <Card className="transition-theme border-l-4 border-l-green-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Models</CardTitle>
            <div className="h-8 w-8 rounded-lg bg-green-500/10 flex items-center justify-center">
              <Icon icon="lucide:brain" width="16" height="16" className="text-green-500" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">{activeModels} / {models.length}</div>
            <p className="text-xs text-muted-foreground">
              Healthy and serving
            </p>
          </CardContent>
        </Card>

        <Card className="transition-theme border-l-4 border-l-yellow-500">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Avg Health Score</CardTitle>
            <div className="h-8 w-8 rounded-lg bg-yellow-500/10 flex items-center justify-center">
              <Icon icon="lucide:zap" width="16" height="16" className="text-yellow-500" />
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">{avgHealthScore.toFixed(1)}%</div>
            <p className="text-xs text-muted-foreground">
              System health
            </p>
          </CardContent>
        </Card>

        <Card className={`transition-theme border-l-4 ${
          stats?.should_shed_load ? 'border-l-red-500' : 'border-l-green-500'
        }`}>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Load Shedding</CardTitle>
            <div className={`h-8 w-8 rounded-lg flex items-center justify-center ${
              stats?.should_shed_load ? 'bg-red-500/10' : 'bg-green-500/10'
            }`}>
              {stats?.should_shed_load ? (
                <Icon icon="lucide:alert-circle" width="16" height="16" className="text-red-500" />
              ) : (
                <Icon icon="lucide:check-circle" width="16" height="16" className="text-green-500" />
              )}
            </div>
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">
              {stats?.should_shed_load ? 'Active' : 'Inactive'}
            </div>
            <p className="text-xs text-muted-foreground">
              System protection status
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 lg:gap-6">
        {/* Model Health Chart */}
        <Card className="transition-theme">
          <CardHeader>
            <CardTitle className="text-lg lg:text-xl">Model Health Scores</CardTitle>
            <CardDescription>Real-time health monitoring</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px] sm:h-[300px]">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={modelHealthData} margin={{ top: 5, right: 5, left: 5, bottom: 5 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke={isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)'} />
                  <XAxis 
                    dataKey="name" 
                    className="text-xs" 
                    tick={{ fontSize: 12, fill: isDark ? 'hsl(217.9, 10.6%, 64.9%)' : 'hsl(220, 8.9%, 46.1%)' }}
                    interval={0}
                    angle={-45}
                    textAnchor="end"
                    height={60}
                  />
                  <YAxis className="text-xs" tick={{ fontSize: 12, fill: isDark ? 'hsl(217.9, 10.6%, 64.9%)' : 'hsl(220, 8.9%, 46.1%)' }} />
                  <Tooltip 
                    contentStyle={{ 
                      backgroundColor: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(0, 0%, 100%)',
                      border: `1px solid ${isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)'}`,
                      fontSize: '12px',
                      color: isDark ? 'hsl(210, 20%, 98%)' : 'hsl(224, 71.4%, 4.1%)',
                      borderRadius: '8px'
                    }}
                  />
                  <Bar 
                    dataKey="health" 
                    fill="hsl(var(--primary))" 
                    radius={[6, 6, 0, 0]}
                    className="transition-all duration-200 hover:opacity-80"
                  />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Request Distribution */}
        <Card className="transition-theme">
          <CardHeader>
            <CardTitle className="text-lg lg:text-xl">Request Distribution</CardTitle>
            <CardDescription>Load balancing across models</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px] sm:h-[300px]">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={pieData}
                    cx="50%"
                    cy="50%"
                    labelLine={false}
                    label={({ name, percent }) => `${name}: ${(percent * 100).toFixed(0)}%`}
                    outerRadius={window.innerWidth < 640 ? 60 : 80}
                    fill="#8884d8"
                    dataKey="value"
                  >
                    {pieData.map((_, index) => (
                      <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip 
                    contentStyle={{ 
                      fontSize: '12px',
                      backgroundColor: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(0, 0%, 100%)',
                      border: `1px solid ${isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)'}`,
                      borderRadius: '8px',
                      color: isDark ? 'hsl(210, 20%, 98%)' : 'hsl(224, 71.4%, 4.1%)'
                    }} 
                  />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* ECharts Real-time Metrics */}
      <Card className="transition-theme">
        <CardHeader>
          <CardTitle className="text-lg lg:text-xl">Real-time Latency</CardTitle>
          <CardDescription>Live performance monitoring</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-[250px] sm:h-[300px]">
            <ReactECharts
              option={{
                backgroundColor: 'transparent',
                tooltip: {
                  trigger: 'axis',
                  axisPointer: {
                    type: 'cross',
                    lineStyle: {
                      color: isDark ? 'hsl(217.9, 10.6%, 64.9%)' : 'hsl(220, 8.9%, 46.1%)'
                    }
                  },
                  backgroundColor: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(0, 0%, 100%)',
                  borderColor: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)',
                  textStyle: {
                    color: isDark ? 'hsl(210, 20%, 98%)' : 'hsl(224, 71.4%, 4.1%)'
                  }
                },
                legend: {
                  data: modelHealthData.map(m => m.name),
                  textStyle: {
                    color: isDark ? 'hsl(210, 20%, 98%)' : 'hsl(224, 71.4%, 4.1%)'
                  },
                  padding: [10, 0]
                },
                grid: {
                  left: '3%',
                  right: '4%',
                  bottom: '8%',
                  top: '15%',
                  containLabel: true,
                  borderColor: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)'
                },
                xAxis: {
                  type: 'category',
                  data: ['00:00', '00:05', '00:10', '00:15', '00:20'],
                  axisLine: {
                    lineStyle: {
                      color: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)'
                    }
                  },
                  axisLabel: {
                    color: isDark ? 'hsl(217.9, 10.6%, 64.9%)' : 'hsl(220, 8.9%, 46.1%)'
                  },
                  splitLine: {
                    lineStyle: {
                      color: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)',
                      type: 'dashed'
                    }
                  }
                },
                yAxis: {
                  type: 'value',
                  name: 'Latency (ms)',
                  nameTextStyle: {
                    color: isDark ? 'hsl(217.9, 10.6%, 64.9%)' : 'hsl(220, 8.9%, 46.1%)'
                  },
                  axisLine: {
                    lineStyle: {
                      color: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)'
                    }
                  },
                  axisLabel: {
                    color: isDark ? 'hsl(217.9, 10.6%, 64.9%)' : 'hsl(220, 8.9%, 46.1%)'
                  },
                  splitLine: {
                    lineStyle: {
                      color: isDark ? 'hsl(215, 27.9%, 16.9%)' : 'hsl(220, 13%, 91%)',
                      type: 'dashed'
                    }
                  }
                },
                series: modelHealthData.map((model, index) => ({
                  name: model.name,
                  type: 'line',
                  smooth: true,
                  symbol: 'circle',
                  symbolSize: 6,
                  data: Array.from({ length: 5 }, () => 
                    Math.floor(Math.random() * 200) + model.latency
                  ),
                  lineStyle: {
                    color: COLORS[index % COLORS.length],
                    width: 3
                  },
                  itemStyle: {
                    color: COLORS[index % COLORS.length]
                  },
                  areaStyle: {
                    color: {
                      type: 'linear',
                      x: 0,
                      y: 0,
                      x2: 0,
                      y2: 1,
                      colorStops: [{
                        offset: 0,
                        color: COLORS[index % COLORS.length] + '20'
                      }, {
                        offset: 1,
                        color: COLORS[index % COLORS.length] + '00'
                      }]
                    }
                  }
                }))
              }}
              style={{ height: '100%', width: '100%' }}
              opts={{ renderer: 'svg' }}
            />
          </div>
        </CardContent>
      </Card>

      {/* Model Status Table */}
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
                    <th className="text-left p-3 font-semibold min-w-[140px]">Model</th>
                    <th className="text-left p-3 font-semibold min-w-[90px]">Provider</th>
                    <th className="text-left p-3 font-semibold min-w-[100px]">Status</th>
                    <th className="text-left p-3 font-semibold min-w-[140px]">Health Score</th>
                    <th className="text-left p-3 font-semibold min-w-[90px]">Requests</th>
                    <th className="text-left p-3 font-semibold min-w-[80px]">Failed</th>
                    <th className="text-left p-3 font-semibold min-w-[100px]">Avg Latency</th>
                    <th className="text-left p-3 font-semibold min-w-[100px]">P95 Latency</th>
                    <th className="text-left p-3 font-semibold min-w-[90px]">Circuit</th>
                  </tr>
                </thead>
                <tbody>
                  <TooltipProvider>
                    {Object.entries(stats?.load_balancer || {}).map(([name, data]: [string, any]) => {
                      const providerInfo = getProviderInfo(name);

                      return (
                        <tr key={name} className="border-b border-border/50 hover:bg-muted/30 transition-colors duration-200">
                          <td className="p-3 font-medium">
                            <div className="flex items-center space-x-3">
                              <div className="flex-shrink-0">
                                <Icon icon={providerInfo.icon} width="24" height="24" className={providerInfo.color} />
                              </div>
                              <span className="truncate font-medium">{name}</span>
                            </div>
                          </td>
                          <td className="p-3">
                            <UITooltip>
                              <TooltipTrigger asChild>
                                <div className="flex items-center justify-center w-10 h-8 rounded-lg bg-muted/50 hover:bg-muted transition-colors cursor-help">
                                  <Icon icon={providerInfo.icon} width="20" height="20" className={providerInfo.color} />
                                </div>
                              </TooltipTrigger>
                              <TooltipContent>
                                <p className="font-medium">{providerInfo.name}</p>
                              </TooltipContent>
                            </UITooltip>
                          </td>
                          <td className="p-3">
                            <div className="flex items-center space-x-2">
                              <div className={`w-2 h-2 rounded-full ${
                                data.circuit_open ? 'bg-red-500' : 'bg-green-500'
                              }`} />
                              <span className={`text-sm font-medium ${
                                data.circuit_open ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'
                              }`}>
                                {data.circuit_open ? 'Unhealthy' : 'Healthy'}
                              </span>
                            </div>
                          </td>
                          <td className="p-3">
                            <div className="flex items-center space-x-3">
                              <div className="flex-1 max-w-20">
                                <div className="w-full bg-muted rounded-full h-2.5 overflow-hidden">
                                  <div 
                                    className={`h-2.5 rounded-full transition-all duration-500 ${
                                      data.health_score >= 80 ? 'bg-green-500' :
                                      data.health_score >= 60 ? 'bg-yellow-500' : 'bg-red-500'
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
                            <span className={`font-mono text-sm font-medium ${
                              data.failed_requests > 0 
                                ? 'text-red-600 dark:text-red-400 font-bold' 
                                : 'text-muted-foreground'
                            }`}>
                              {data.failed_requests}
                            </span>
                          </td>
                          <td className="p-3">
                            <span className="font-mono text-sm">
                              {data.avg_latency ? `${data.avg_latency}ms` : 'N/A'}
                            </span>
                          </td>
                          <td className="p-3">
                            <span className="font-mono text-sm">
                              {data.p95_latency ? `${data.p95_latency}ms` : 'N/A'}
                            </span>
                          </td>
                          <td className="p-3">
                            <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium border ${
                              data.circuit_open 
                                ? 'bg-red-50 dark:bg-red-950/30 text-red-700 dark:text-red-400 border-red-200 dark:border-red-800' 
                                : 'bg-green-50 dark:bg-green-950/30 text-green-700 dark:text-green-400 border-green-200 dark:border-green-800'
                            }`}>
                              {data.circuit_open ? 'Open' : 'Closed'}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </TooltipProvider>
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}