import { useQuery } from '@tanstack/react-query'
import { getModels } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Icon } from '@iconify/react'

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
    <div className="space-y-4 lg:space-y-6">
      <div>
        <h1 className="text-2xl lg:text-3xl font-bold">Models</h1>
        <p className="text-sm lg:text-base text-muted-foreground">Configure and manage LLM models</p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 lg:gap-6">
        {models.map((model: any) => {
          const getProviderInfo = (modelId: string, ownedBy: string) => {
            const id = modelId.toLowerCase()
            const owner = ownedBy?.toLowerCase() || ''
            
            if (id.includes('gpt') || owner.includes('openai') || id.includes('openai')) {
              return {
                icon: <Icon icon="logos:openai" width="20" height="20" />,
                name: 'OpenAI',
                color: 'text-emerald-600'
              }
            }
            if (id.includes('claude') || owner.includes('anthropic') || id.includes('anthropic')) {
              return {
                icon: <Icon icon="logos:anthropic" width="20" height="20" />,
                name: 'Anthropic',
                color: 'text-orange-600'
              }
            }
            if (id.includes('mistral') || owner.includes('mistral')) {
              return {
                icon: <Icon icon="logos:mistral" width="20" height="20" />,
                name: 'Mistral AI',
                color: 'text-blue-600'
              }
            }
            if (id.includes('llama') || owner.includes('meta') || id.includes('meta')) {
              return {
                icon: <Icon icon="logos:meta" width="20" height="20" />,
                name: 'Meta',
                color: 'text-blue-500'
              }
            }
            return {
              icon: <Icon icon="lucide:brain" width="20" height="20" className="text-muted-foreground" />,
              name: ownedBy || 'Unknown',
              color: 'text-muted-foreground'
            }
          }
          
          const providerInfo = getProviderInfo(model.id, model.owned_by)
          
          return (
            <Card key={model.id} className="hover:shadow-md transition-shadow">
              <CardHeader className="pb-4">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base lg:text-lg flex items-center truncate">
                    <div className="flex-shrink-0 mr-2">
                      {providerInfo.icon}
                    </div>
                    <span className="truncate">{model.id}</span>
                  </CardTitle>
                  <Badge variant={model.created ? 'default' : 'secondary'} className="ml-2 flex-shrink-0">
                    {model.object}
                  </Badge>
                </div>
                <CardDescription className={providerInfo.color}>
                  {providerInfo.name}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center text-sm text-muted-foreground">
                  <Icon icon="lucide:server" width="16" height="16" className="mr-2 flex-shrink-0" />
                  <span className="truncate">Provider: {providerInfo.name}</span>
                </div>
                <div className="flex items-center text-sm text-muted-foreground">
                  <Icon icon="lucide:activity" width="16" height="16" className="mr-2 flex-shrink-0 text-green-500" />
                  <span>Status: Active</span>
                </div>
                <div className="flex items-center text-sm text-muted-foreground">
                  <Icon icon="lucide:clock" width="16" height="16" className="mr-2 flex-shrink-0" />
                  <span className="truncate text-xs">Created: {new Date(model.created * 1000).toLocaleDateString()}</span>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>
    </div>
  )
}