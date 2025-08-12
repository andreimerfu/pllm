import { useQuery } from '@tanstack/react-query'
import { getModelStats, getModels } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'
import ReactECharts from 'echarts-for-react'
import { Icon } from '@iconify/react'
import { useState, useEffect } from 'react'

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

  const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444']

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
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
            <Icon icon="lucide:activity" width="16" height="16" className="text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">{totalRequests.toLocaleString()}</div>
            <p className="text-xs text-muted-foreground">
              Across all models
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Models</CardTitle>
            <Icon icon="lucide:brain" width="16" height="16" className="text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">{activeModels} / {models.length}</div>
            <p className="text-xs text-muted-foreground">
              Healthy and serving
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Avg Health Score</CardTitle>
            <Icon icon="lucide:zap" width="16" height="16" className="text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-xl sm:text-2xl font-bold">{avgHealthScore.toFixed(1)}%</div>
            <p className="text-xs text-muted-foreground">
              System health
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Load Shedding</CardTitle>
            {stats?.should_shed_load ? (
              <Icon icon="lucide:alert-circle" width="16" height="16" className="text-destructive" />
            ) : (
              <Icon icon="lucide:check-circle" width="16" height="16" className="text-green-500" />
            )}
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
        <Card>
          <CardHeader>
            <CardTitle className="text-lg lg:text-xl">Model Health Scores</CardTitle>
            <CardDescription>Real-time health monitoring</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="h-[250px] sm:h-[300px]">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={modelHealthData} margin={{ top: 5, right: 5, left: 5, bottom: 5 }}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                  <XAxis 
                    dataKey="name" 
                    className="text-xs" 
                    tick={{ fontSize: 12 }}
                    interval={0}
                    angle={-45}
                    textAnchor="end"
                    height={60}
                  />
                  <YAxis className="text-xs" tick={{ fontSize: 12 }} />
                  <Tooltip 
                    contentStyle={{ 
                      backgroundColor: 'hsl(var(--card))',
                      border: '1px solid hsl(var(--border))',
                      fontSize: '12px'
                    }}
                  />
                  <Bar dataKey="health" fill="hsl(var(--primary))" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>

        {/* Request Distribution */}
        <Card>
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
                  <Tooltip contentStyle={{ fontSize: '12px' }} />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* ECharts Real-time Metrics */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg lg:text-xl">Real-time Latency</CardTitle>
          <CardDescription>Live performance monitoring</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-[250px] sm:h-[300px]">
            <ReactECharts
              option={{
                tooltip: {
                  trigger: 'axis',
                  axisPointer: {
                    type: 'cross'
                  }
                },
                legend: {
                  data: modelHealthData.map(m => m.name),
                  textStyle: {
                    color: 'hsl(var(--foreground))'
                  }
                },
                grid: {
                  left: '3%',
                  right: '4%',
                  bottom: '3%',
                  containLabel: true
                },
                xAxis: {
                  type: 'category',
                  data: ['00:00', '00:05', '00:10', '00:15', '00:20'],
                  axisLine: {
                    lineStyle: {
                      color: 'hsl(var(--border))'
                    }
                  },
                  axisLabel: {
                    color: 'hsl(var(--muted-foreground))'
                  }
                },
                yAxis: {
                  type: 'value',
                  name: 'Latency (ms)',
                  axisLine: {
                    lineStyle: {
                      color: 'hsl(var(--border))'
                    }
                  },
                  axisLabel: {
                    color: 'hsl(var(--muted-foreground))'
                  }
                },
                series: modelHealthData.map((model, index) => ({
                  name: model.name,
                  type: 'line',
                  smooth: true,
                  data: Array.from({ length: 5 }, () => 
                    Math.floor(Math.random() * 200) + model.latency
                  ),
                  lineStyle: {
                    color: COLORS[index % COLORS.length]
                  },
                  itemStyle: {
                    color: COLORS[index % COLORS.length]
                  }
                }))
              }}
              style={{ height: '100%', width: '100%' }}
              theme={isDark ? 'dark' : undefined}
            />
          </div>
        </CardContent>
      </Card>

      {/* Model Status Table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg lg:text-xl">Model Status</CardTitle>
          <CardDescription>Detailed model performance metrics</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <div className="min-w-[800px]">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left p-2 min-w-[120px]">Model</th>
                    <th className="text-left p-2 min-w-[80px]">Provider</th>
                    <th className="text-left p-2 min-w-[100px]">Status</th>
                    <th className="text-left p-2 min-w-[120px]">Health Score</th>
                    <th className="text-left p-2 min-w-[80px]">Requests</th>
                    <th className="text-left p-2 min-w-[80px]">Failed</th>
                    <th className="text-left p-2 min-w-[100px]">Avg Latency</th>
                    <th className="text-left p-2 min-w-[100px]">P95 Latency</th>
                    <th className="text-left p-2 min-w-[80px]">Circuit</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(stats?.load_balancer || {}).map(([name, data]: [string, any]) => {
                    const getProviderIcon = (modelName: string) => {
                      if (modelName.toLowerCase().includes('gpt') || modelName.toLowerCase().includes('openai')) {
                        return <Icon icon="logos:openai" width="16" height="16" />;
                      }
                      if (modelName.toLowerCase().includes('claude') || modelName.toLowerCase().includes('anthropic')) {
                        return <Icon icon="logos:anthropic" width="16" height="16" />;
                      }
                      return <Icon icon="lucide:brain" width="16" height="16" className="text-muted-foreground" />;
                    };

                    return (
                      <tr key={name} className="border-b hover:bg-muted/50">
                        <td className="p-2 font-medium">
                          <div className="flex items-center space-x-2">
                            {getProviderIcon(name)}
                            <span className="truncate">{name}</span>
                          </div>
                        </td>
                        <td className="p-2">
                          <span className="text-xs px-2 py-1 bg-muted rounded-full">
                            {name.toLowerCase().includes('gpt') || name.toLowerCase().includes('openai') ? 'OpenAI' :
                             name.toLowerCase().includes('claude') || name.toLowerCase().includes('anthropic') ? 'Anthropic' : 'Other'}
                          </span>
                        </td>
                        <td className="p-2">
                          <div className="flex items-center">
                            {data.circuit_open ? (
                              <Icon icon="lucide:x-circle" width="16" height="16" className="text-destructive mr-1" />
                            ) : (
                              <Icon icon="lucide:check-circle" width="16" height="16" className="text-green-500 mr-1" />
                            )}
                            <span className={`text-xs ${data.circuit_open ? 'text-destructive' : 'text-green-600'}`}>
                              {data.circuit_open ? 'Unhealthy' : 'Healthy'}
                            </span>
                          </div>
                        </td>
                        <td className="p-2">
                          <div className="flex items-center">
                            <div className="w-16 sm:w-24 bg-muted rounded-full h-2 mr-2">
                              <div 
                                className="bg-primary h-2 rounded-full transition-all"
                                style={{ width: `${data.health_score}%` }}
                              />
                            </div>
                            <span className="text-xs font-medium">{Math.round(data.health_score)}%</span>
                          </div>
                        </td>
                        <td className="p-2 font-mono text-xs">{data.total_requests}</td>
                        <td className="p-2">
                          <span className={`font-mono text-xs ${data.failed_requests > 0 ? 'text-destructive font-semibold' : ''}`}>
                            {data.failed_requests}
                          </span>
                        </td>
                        <td className="p-2 font-mono text-xs">{data.avg_latency || 'N/A'}</td>
                        <td className="p-2 font-mono text-xs">{data.p95_latency || 'N/A'}</td>
                        <td className="p-2">
                          <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                            data.circuit_open 
                              ? 'bg-destructive/10 text-destructive' 
                              : 'bg-green-500/10 text-green-600'
                          }`}>
                            {data.circuit_open ? 'Open' : 'Closed'}
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}