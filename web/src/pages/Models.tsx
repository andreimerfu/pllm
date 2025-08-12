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

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4 gap-6">
        {models.map((model: any) => {
          const getProviderInfo = (modelId: string, ownedBy: string) => {
            const id = modelId.toLowerCase()
            const owner = ownedBy?.toLowerCase() || ''
            
            if (id.includes('gpt') || owner.includes('openai') || id.includes('openai')) {
              return {
                icon: 'logos:openai',
                name: 'OpenAI',
                color: 'text-emerald-600 dark:text-emerald-400',
                bgColor: 'bg-emerald-50 dark:bg-emerald-950/30',
                borderColor: 'border-emerald-200 dark:border-emerald-800'
              }
            }
            if (id.includes('claude') || owner.includes('anthropic') || id.includes('anthropic')) {
              return {
                icon: 'logos:anthropic',
                name: 'Anthropic',
                color: 'text-orange-600 dark:text-orange-400',
                bgColor: 'bg-orange-50 dark:bg-orange-950/30',
                borderColor: 'border-orange-200 dark:border-orange-800'
              }
            }
            if (id.includes('mistral') || owner.includes('mistral')) {
              return {
                icon: 'logos:mistral',
                name: 'Mistral AI',
                color: 'text-blue-600 dark:text-blue-400',
                bgColor: 'bg-blue-50 dark:bg-blue-950/30',
                borderColor: 'border-blue-200 dark:border-blue-800'
              }
            }
            if (id.includes('llama') || owner.includes('meta') || id.includes('meta')) {
              return {
                icon: 'logos:meta',
                name: 'Meta',
                color: 'text-indigo-600 dark:text-indigo-400',
                bgColor: 'bg-indigo-50 dark:bg-indigo-950/30',
                borderColor: 'border-indigo-200 dark:border-indigo-800'
              }
            }
            if (id.includes('gemini') || owner.includes('google') || id.includes('google')) {
              return {
                icon: 'logos:google',
                name: 'Google',
                color: 'text-red-600 dark:text-red-400',
                bgColor: 'bg-red-50 dark:bg-red-950/30',
                borderColor: 'border-red-200 dark:border-red-800'
              }
            }
            if (id.includes('azure') || owner.includes('microsoft') || id.includes('microsoft')) {
              return {
                icon: 'logos:microsoft',
                name: 'Microsoft',
                color: 'text-blue-700 dark:text-blue-300',
                bgColor: 'bg-blue-50 dark:bg-blue-950/30',
                borderColor: 'border-blue-200 dark:border-blue-800'
              }
            }
            if (id.includes('bedrock') || owner.includes('aws') || id.includes('aws')) {
              return {
                icon: 'logos:aws',
                name: 'AWS',
                color: 'text-yellow-600 dark:text-yellow-400',
                bgColor: 'bg-yellow-50 dark:bg-yellow-950/30',
                borderColor: 'border-yellow-200 dark:border-yellow-800'
              }
            }
            return {
              icon: 'lucide:brain',
              name: ownedBy || 'Unknown',
              color: 'text-muted-foreground',
              bgColor: 'bg-muted/30',
              borderColor: 'border-muted'
            }
          }
          
          const providerInfo = getProviderInfo(model.id, model.owned_by)
          
          return (
            <Card key={model.id} className="transition-theme group relative overflow-hidden">
              {/* Background gradient overlay */}
              <div className={`absolute inset-0 opacity-5 group-hover:opacity-10 transition-opacity ${providerInfo.bgColor}`} />
              
              <CardHeader className="pb-4 relative z-10">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-4 min-w-0">
                    {/* Larger provider icon */}
                    <div className={`flex-shrink-0 p-3 rounded-xl border ${providerInfo.bgColor} ${providerInfo.borderColor} shadow-sm`}>
                      <Icon icon={providerInfo.icon} width="40" height="40" className={providerInfo.color} />
                    </div>
                    
                    <div className="min-w-0 flex-1">
                      <CardTitle className="text-base lg:text-lg font-bold leading-tight">
                        <span className="block truncate">{model.id}</span>
                      </CardTitle>
                      <CardDescription className={`mt-1 font-medium ${providerInfo.color}`}>
                        {providerInfo.name}
                      </CardDescription>
                    </div>
                  </div>
                  
                  <Badge 
                    variant={model.created ? 'default' : 'secondary'} 
                    className="flex-shrink-0 font-medium"
                  >
                    {model.object}
                  </Badge>
                </div>
              </CardHeader>
              
              <CardContent className="space-y-4 relative z-10">
                <div className="grid grid-cols-1 gap-3">
                  <div className="flex items-center text-sm">
                    <div className="w-8 h-8 rounded-lg bg-muted/50 flex items-center justify-center mr-3 flex-shrink-0">
                      <Icon icon="lucide:server" width="16" height="16" className="text-muted-foreground" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <span className="font-medium text-foreground">Provider</span>
                      <p className={`text-sm truncate ${providerInfo.color}`}>{providerInfo.name}</p>
                    </div>
                  </div>
                  
                  <div className="flex items-center text-sm">
                    <div className="w-8 h-8 rounded-lg bg-green-500/10 flex items-center justify-center mr-3 flex-shrink-0">
                      <Icon icon="lucide:activity" width="16" height="16" className="text-green-500" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <span className="font-medium text-foreground">Status</span>
                      <p className="text-sm text-green-600 dark:text-green-400 font-medium">Active & Ready</p>
                    </div>
                  </div>
                  
                  <div className="flex items-center text-sm">
                    <div className="w-8 h-8 rounded-lg bg-muted/50 flex items-center justify-center mr-3 flex-shrink-0">
                      <Icon icon="lucide:calendar" width="16" height="16" className="text-muted-foreground" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <span className="font-medium text-foreground">Created</span>
                      <p className="text-sm text-muted-foreground">
                        {new Date(model.created * 1000).toLocaleDateString('en-US', {
                          year: 'numeric',
                          month: 'short',
                          day: 'numeric'
                        })}
                      </p>
                    </div>
                  </div>
                </div>
                
                {/* Action buttons */}
                <div className="pt-2 border-t border-border/50">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span className="inline-flex items-center gap-1">
                      <div className="w-2 h-2 rounded-full bg-green-500" />
                      Ready to serve
                    </span>
                    <Icon icon="lucide:chevron-right" width="16" height="16" className="opacity-50 group-hover:opacity-100 transition-opacity" />
                  </div>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>
    </div>
  )
}