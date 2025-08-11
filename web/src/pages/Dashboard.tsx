import { useQuery } from '@tanstack/react-query'
import { getModelStats, getModels } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { 
  Activity, 
  Brain, 
  Zap, 
  AlertCircle,
  CheckCircle,
  XCircle
} from 'lucide-react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'

export default function Dashboard() {
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
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalRequests.toLocaleString()}</div>
            <p className="text-xs text-muted-foreground">
              Across all models
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Models</CardTitle>
            <Brain className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activeModels} / {models.length}</div>
            <p className="text-xs text-muted-foreground">
              Healthy and serving
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Avg Health Score</CardTitle>
            <Zap className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{avgHealthScore.toFixed(1)}%</div>
            <p className="text-xs text-muted-foreground">
              System health
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Load Shedding</CardTitle>
            {stats?.should_shed_load ? (
              <AlertCircle className="h-4 w-4 text-destructive" />
            ) : (
              <CheckCircle className="h-4 w-4 text-green-500" />
            )}
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.should_shed_load ? 'Active' : 'Inactive'}
            </div>
            <p className="text-xs text-muted-foreground">
              System protection status
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Model Health Chart */}
        <Card>
          <CardHeader>
            <CardTitle>Model Health Scores</CardTitle>
            <CardDescription>Real-time health monitoring</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={modelHealthData}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                <XAxis dataKey="name" className="text-xs" />
                <YAxis className="text-xs" />
                <Tooltip 
                  contentStyle={{ 
                    backgroundColor: 'hsl(var(--card))',
                    border: '1px solid hsl(var(--border))'
                  }}
                />
                <Bar dataKey="health" fill="hsl(var(--primary))" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        {/* Request Distribution */}
        <Card>
          <CardHeader>
            <CardTitle>Request Distribution</CardTitle>
            <CardDescription>Load balancing across models</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, percent }) => `${name}: ${(percent * 100).toFixed(0)}%`}
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {pieData.map((_, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      {/* Model Status Table */}
      <Card>
        <CardHeader>
          <CardTitle>Model Status</CardTitle>
          <CardDescription>Detailed model performance metrics</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left p-2">Model</th>
                  <th className="text-left p-2">Status</th>
                  <th className="text-left p-2">Health Score</th>
                  <th className="text-left p-2">Requests</th>
                  <th className="text-left p-2">Failed</th>
                  <th className="text-left p-2">Avg Latency</th>
                  <th className="text-left p-2">P95 Latency</th>
                  <th className="text-left p-2">Circuit</th>
                </tr>
              </thead>
              <tbody>
                {Object.entries(stats?.load_balancer || {}).map(([name, data]: [string, any]) => (
                  <tr key={name} className="border-b">
                    <td className="p-2 font-medium">{name}</td>
                    <td className="p-2">
                      <div className="flex items-center">
                        {data.circuit_open ? (
                          <XCircle className="h-4 w-4 text-destructive mr-1" />
                        ) : (
                          <CheckCircle className="h-4 w-4 text-green-500 mr-1" />
                        )}
                        <span className={data.circuit_open ? 'text-destructive' : 'text-green-600'}>
                          {data.circuit_open ? 'Unhealthy' : 'Healthy'}
                        </span>
                      </div>
                    </td>
                    <td className="p-2">
                      <div className="flex items-center">
                        <div className="w-24 bg-muted rounded-full h-2 mr-2">
                          <div 
                            className="bg-primary h-2 rounded-full"
                            style={{ width: `${data.health_score}%` }}
                          />
                        </div>
                        <span className="text-xs">{Math.round(data.health_score)}%</span>
                      </div>
                    </td>
                    <td className="p-2">{data.total_requests}</td>
                    <td className="p-2">
                      <span className={data.failed_requests > 0 ? 'text-destructive' : ''}>
                        {data.failed_requests}
                      </span>
                    </td>
                    <td className="p-2">{data.avg_latency || 'N/A'}</td>
                    <td className="p-2">{data.p95_latency || 'N/A'}</td>
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
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}