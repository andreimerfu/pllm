import { useQuery } from '@tanstack/react-query'
import { getModels } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Brain, Server, Activity, Clock } from 'lucide-react'

export default function Models() {
  const { data, isLoading } = useQuery({
    queryKey: ['models'],
    queryFn: getModels,
  })

  const models = data?.data?.data || []

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Models</h1>
        <p className="text-muted-foreground">Configure and manage LLM models</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {models.map((model: any) => (
          <Card key={model.id}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-lg flex items-center">
                  <Brain className="h-5 w-5 mr-2" />
                  {model.id}
                </CardTitle>
                <Badge variant={model.created ? 'default' : 'secondary'}>
                  {model.object}
                </Badge>
              </div>
              <CardDescription>
                {model.owned_by || 'OpenAI'}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center text-sm text-muted-foreground">
                <Server className="h-4 w-4 mr-2" />
                <span>Provider: {model.owned_by || 'OpenAI'}</span>
              </div>
              <div className="flex items-center text-sm text-muted-foreground">
                <Activity className="h-4 w-4 mr-2" />
                <span>Status: Active</span>
              </div>
              <div className="flex items-center text-sm text-muted-foreground">
                <Clock className="h-4 w-4 mr-2" />
                <span>Created: {new Date(model.created * 1000).toLocaleDateString()}</span>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}