import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Loader2, Globe, Package, FileEdit, AlertCircle, CheckCircle2 } from 'lucide-react'
import { GuardrailSourceType } from '@/types/discovery'
import { useDiscoverGuardrail, useGuardrailSources } from '@/hooks/useDiscovery'
import { DiscoveryPreview } from './DiscoveryPreview'

interface AddSourceDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AddSourceDialog({ open, onOpenChange }: AddSourceDialogProps) {
  const [sourceType, setSourceType] = useState<GuardrailSourceType>('url')
  const [sourceName, setSourceName] = useState('')
  const [sourceUrl, setSourceUrl] = useState('')
  const [discoveryUrl, setDiscoveryUrl] = useState('')
  const [showPreview, setShowPreview] = useState(false)

  const { discover, isDiscovering, discovery, error, reset } = useDiscoverGuardrail()
  const { addSource, isAdding } = useGuardrailSources()

  const handleDiscover = async () => {
    const url = sourceType === 'url' ? discoveryUrl : sourceUrl
    if (!url) return

    try {
      await discover(url)
      setShowPreview(true)
    } catch {
      // Error is handled by the hook
    }
  }

  const handleAdd = () => {
    if (!sourceName) return

    addSource({
      name: sourceName,
      type: sourceType,
      url: sourceType === 'url' ? sourceUrl : undefined,
      discovery_endpoint: sourceType === 'url' ? discoveryUrl : undefined,
    })

    handleClose()
  }

  const handleClose = () => {
    setSourceName('')
    setSourceUrl('')
    setDiscoveryUrl('')
    setShowPreview(false)
    reset()
    onOpenChange(false)
  }

  const canDiscover = sourceType === 'url' && discoveryUrl.trim() !== ''
  const canAdd = sourceName.trim() !== '' && (!showPreview || discovery !== null)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Add Guardrail Source</DialogTitle>
          <DialogDescription>
            Add a new source to discover and install guardrails from
          </DialogDescription>
        </DialogHeader>

        {!showPreview ? (
          <Tabs value={sourceType} onValueChange={(v) => setSourceType(v as GuardrailSourceType)}>
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="url" className="flex items-center gap-2">
                <Globe className="h-4 w-4" />
                URL
              </TabsTrigger>
              <TabsTrigger value="preset" className="flex items-center gap-2">
                <Package className="h-4 w-4" />
                Preset
              </TabsTrigger>
              <TabsTrigger value="manual" className="flex items-center gap-2">
                <FileEdit className="h-4 w-4" />
                Manual
              </TabsTrigger>
            </TabsList>

            <TabsContent value="url" className="space-y-4">
              <Alert>
                <AlertDescription>
                  Enter the URL of a guardrail service that implements the discovery protocol
                </AlertDescription>
              </Alert>

              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="source-name">Source Name</Label>
                  <Input
                    id="source-name"
                    placeholder="e.g., Internal PII Scanner"
                    value={sourceName}
                    onChange={(e) => setSourceName(e.target.value)}
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="source-url">Service URL</Label>
                  <Input
                    id="source-url"
                    placeholder="https://guardrail.example.com"
                    value={sourceUrl}
                    onChange={(e) => setSourceUrl(e.target.value)}
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="discovery-url">Discovery Endpoint</Label>
                  <Input
                    id="discovery-url"
                    placeholder="https://guardrail.example.com/discover"
                    value={discoveryUrl}
                    onChange={(e) => setDiscoveryUrl(e.target.value)}
                  />
                  <p className="text-sm text-muted-foreground">
                    The endpoint that returns the guardrail's capabilities and configuration schema
                  </p>
                </div>

                {error && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertDescription>{error}</AlertDescription>
                  </Alert>
                )}

                {discovery && (
                  <Alert>
                    <CheckCircle2 className="h-4 w-4" />
                    <AlertDescription>
                      Successfully discovered: {discovery.name} v{discovery.version}
                    </AlertDescription>
                  </Alert>
                )}
              </div>
            </TabsContent>

            <TabsContent value="preset" className="space-y-4">
              <Alert>
                <AlertDescription>
                  Select from a list of verified guardrail sources
                </AlertDescription>
              </Alert>

              <div className="space-y-2">
                <Label>Available Presets</Label>
                <div className="grid gap-2">
                  {PRESET_SOURCES.map((preset) => (
                    <button
                      key={preset.id}
                      onClick={() => {
                        setSourceName(preset.name)
                        setSourceUrl(preset.url)
                        setDiscoveryUrl(preset.discovery_endpoint)
                      }}
                      className="flex items-start gap-3 p-4 border rounded-lg hover:bg-accent text-left transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{preset.name}</span>
                          <Badge variant="secondary">{preset.provider}</Badge>
                          {preset.verified && (
                            <CheckCircle2 className="h-4 w-4 text-green-500" />
                          )}
                        </div>
                        <p className="text-sm text-muted-foreground mt-1">
                          {preset.description}
                        </p>
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            </TabsContent>

            <TabsContent value="manual" className="space-y-4">
              <Alert>
                <AlertDescription>
                  Manually configure a guardrail source without discovery
                </AlertDescription>
              </Alert>

              <div className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="manual-name">Source Name</Label>
                  <Input
                    id="manual-name"
                    placeholder="e.g., Custom Regex Filter"
                    value={sourceName}
                    onChange={(e) => setSourceName(e.target.value)}
                  />
                </div>

                <Alert variant="default">
                  <AlertCircle className="h-4 w-4" />
                  <AlertDescription>
                    Manual sources don't use the discovery protocol. You'll need to configure them
                    through the configuration file.
                  </AlertDescription>
                </Alert>
              </div>
            </TabsContent>
          </Tabs>
        ) : (
          <DiscoveryPreview discovery={discovery!} />
        )}

        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>
            Cancel
          </Button>
          {!showPreview && sourceType === 'url' && (
            <Button onClick={handleDiscover} disabled={!canDiscover || isDiscovering}>
              {isDiscovering && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Discover
            </Button>
          )}
          <Button onClick={handleAdd} disabled={!canAdd || isAdding}>
            {isAdding && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Add Source
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Preset sources for quick setup
const PRESET_SOURCES = [
  {
    id: 'presidio',
    name: 'Microsoft Presidio',
    provider: 'Microsoft',
    description: 'Open-source PII detection and anonymization',
    url: 'https://presidio.example.com',
    discovery_endpoint: 'https://presidio.example.com/discover',
    verified: true,
  },
  {
    id: 'aws-comprehend',
    name: 'AWS Comprehend',
    provider: 'Amazon',
    description: 'AWS managed PII and sentiment detection',
    url: 'https://comprehend.example.com',
    discovery_endpoint: 'https://comprehend.example.com/discover',
    verified: true,
  },
  {
    id: 'openai-moderation',
    name: 'OpenAI Moderation',
    provider: 'OpenAI',
    description: 'Content moderation for harmful content detection',
    url: 'https://openai-mod.example.com',
    discovery_endpoint: 'https://openai-mod.example.com/discover',
    verified: true,
  },
]
