import { GuardrailDiscoveryResponse } from '@/types/discovery'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import {
  CheckCircle2,
  Clock,
  Gauge,
  Globe,
  Shield,
  Zap,
  ExternalLink,
} from 'lucide-react'

interface DiscoveryPreviewProps {
  discovery: GuardrailDiscoveryResponse
}

export function DiscoveryPreview({ discovery }: DiscoveryPreviewProps) {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="space-y-2">
        <div className="flex items-start justify-between">
          <div>
            <h3 className="text-lg font-semibold">{discovery.name}</h3>
            <p className="text-sm text-muted-foreground">{discovery.description}</p>
          </div>
          <Badge variant="outline">{discovery.version}</Badge>
        </div>

        <div className="flex items-center gap-2">
          <Badge variant="secondary">{discovery.category}</Badge>
          {discovery.tags?.map((tag) => (
            <Badge key={tag} variant="outline">
              {tag}
            </Badge>
          ))}
        </div>
      </div>

      <Separator />

      {/* Provider Info */}
      <div>
        <h4 className="text-sm font-medium mb-2 flex items-center gap-2">
          <Shield className="h-4 w-4" />
          Provider
        </h4>
        <div className="text-sm space-y-1">
          <p>
            <span className="font-medium">{discovery.provider.name}</span>
            {discovery.provider.organization && (
              <span className="text-muted-foreground"> • {discovery.provider.organization}</span>
            )}
          </p>
          {discovery.provider.website && (
            <a
              href={discovery.provider.website}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline inline-flex items-center gap-1"
            >
              <Globe className="h-3 w-3" />
              Website
              <ExternalLink className="h-3 w-3" />
            </a>
          )}
        </div>
      </div>

      {/* Capabilities */}
      <div>
        <h4 className="text-sm font-medium mb-3 flex items-center gap-2">
          <Zap className="h-4 w-4" />
          Capabilities
        </h4>
        <div className="grid gap-3 sm:grid-cols-2">
          <CapabilityItem
            label="Execution Modes"
            value={discovery.capabilities.execution_modes.join(', ')}
          />
          <CapabilityItem
            label="Languages"
            value={discovery.capabilities.supported_languages.join(', ')}
          />
          <CapabilityItem
            label="Streaming"
            value={discovery.capabilities.supports_streaming ? 'Yes' : 'No'}
            icon={discovery.capabilities.supports_streaming ? CheckCircle2 : undefined}
          />
          <CapabilityItem
            label="Batch Processing"
            value={
              discovery.capabilities.supports_batch
                ? `Yes (max ${discovery.capabilities.max_batch_size || '∞'})`
                : 'No'
            }
            icon={discovery.capabilities.supports_batch ? CheckCircle2 : undefined}
          />
          {discovery.capabilities.requires_api_key && (
            <CapabilityItem label="API Key" value="Required" icon={Shield} />
          )}
        </div>
      </div>

      {/* Performance */}
      {discovery.performance && (
        <div>
          <h4 className="text-sm font-medium mb-3 flex items-center gap-2">
            <Gauge className="h-4 w-4" />
            Performance Characteristics
          </h4>
          <div className="grid gap-3 sm:grid-cols-2">
            {discovery.performance.avg_latency_ms !== undefined && (
              <PerformanceItem
                label="Avg Latency"
                value={`${discovery.performance.avg_latency_ms}ms`}
                icon={Clock}
              />
            )}
            {discovery.performance.max_latency_ms !== undefined && (
              <PerformanceItem
                label="Max Latency"
                value={`${discovery.performance.max_latency_ms}ms`}
                icon={Clock}
              />
            )}
            {discovery.performance.throughput_req_per_sec !== undefined && (
              <PerformanceItem
                label="Throughput"
                value={`${discovery.performance.throughput_req_per_sec} req/s`}
              />
            )}
            {discovery.performance.accuracy !== undefined && (
              <PerformanceItem
                label="Accuracy"
                value={`${(discovery.performance.accuracy * 100).toFixed(1)}%`}
              />
            )}
          </div>
        </div>
      )}

      {/* Configuration Schema Preview */}
      <div>
        <h4 className="text-sm font-medium mb-3">Configuration Options</h4>
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">
              {discovery.configuration_schema.title || 'Configuration Schema'}
            </CardTitle>
            {discovery.configuration_schema.description && (
              <CardDescription className="text-xs">
                {discovery.configuration_schema.description}
              </CardDescription>
            )}
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {Object.entries(discovery.configuration_schema.properties).map(([key, prop]) => (
                <div key={key} className="flex items-start justify-between text-sm">
                  <div className="flex items-center gap-2">
                    <code className="text-xs bg-muted px-1.5 py-0.5 rounded">{key}</code>
                    {discovery.configuration_schema.required?.includes(key) && (
                      <Badge variant="destructive" className="h-5 text-xs">
                        required
                      </Badge>
                    )}
                  </div>
                  <span className="text-muted-foreground text-xs">
                    {Array.isArray(prop.type) ? prop.type.join(' | ') : prop.type}
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Links */}
      {(discovery.documentation || discovery.homepage || discovery.repository) && (
        <div>
          <h4 className="text-sm font-medium mb-2">Resources</h4>
          <div className="flex flex-wrap gap-2">
            {discovery.documentation && (
              <a
                href={discovery.documentation}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-primary hover:underline inline-flex items-center gap-1"
              >
                Documentation
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
            {discovery.homepage && (
              <a
                href={discovery.homepage}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-primary hover:underline inline-flex items-center gap-1"
              >
                Homepage
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
            {discovery.repository && (
              <a
                href={discovery.repository}
                target="_blank"
                rel="noopener noreferrer"
                className="text-sm text-primary hover:underline inline-flex items-center gap-1"
              >
                Repository
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function CapabilityItem({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: string
  icon?: any
}) {
  return (
    <div className="flex items-start gap-2 text-sm">
      {Icon && <Icon className="h-4 w-4 mt-0.5 text-muted-foreground" />}
      <div>
        <p className="font-medium text-muted-foreground">{label}</p>
        <p className="text-foreground">{value}</p>
      </div>
    </div>
  )
}

function PerformanceItem({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: string
  icon?: any
}) {
  return (
    <div className="flex items-center gap-2 text-sm">
      {Icon && <Icon className="h-4 w-4 text-muted-foreground" />}
      <div>
        <p className="font-medium text-muted-foreground">{label}</p>
        <p className="text-foreground">{value}</p>
      </div>
    </div>
  )
}
