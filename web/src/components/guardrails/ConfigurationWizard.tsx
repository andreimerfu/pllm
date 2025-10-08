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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Loader2,
  Settings,
  Sliders,
  TestTube,
  Rocket,
  ChevronLeft,
  ChevronRight,
  CheckCircle2,
  AlertCircle,
} from 'lucide-react'
import { GuardrailConfigurationState, GuardrailExecutionMode } from '@/types/discovery'
import { SchemaValidator } from '@/lib/schema-validator'
import { DynamicForm } from './DynamicForm'
import { useTestGuardrail, useConfigureGuardrail } from '@/hooks/useDiscovery'

interface ConfigurationWizardProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  initialState: GuardrailConfigurationState
}

export function ConfigurationWizard({
  open,
  onOpenChange,
  initialState,
}: ConfigurationWizardProps) {
  const [step, setStep] = useState(1)
  const [state, setState] = useState<GuardrailConfigurationState>(initialState)

  const { test, isTesting, testResult } = useTestGuardrail()
  const { configure, isConfiguring } = useConfigureGuardrail()

  const validator = new SchemaValidator(state.discovery.configuration_schema)

  const updateDeployment = (updates: Partial<typeof state.deployment>) => {
    setState({
      ...state,
      deployment: { ...state.deployment, ...updates },
    })
  }

  const updateConfiguration = (configuration: Record<string, any>) => {
    const errors = validator.validate(configuration)
    const errorMap = validator.getErrorMap(configuration)

    setState({
      ...state,
      configuration,
      validation_errors: errorMap,
      is_valid: errors.length === 0,
    })
  }

  const handleTest = () => {
    test({
      discovery_id: state.discovery.id,
      configuration: state.configuration,
      test_input: 'Hello, my email is john@example.com and my phone is 555-1234',
    })

    setState({
      ...state,
      test_results: {
        tested: true,
        passed: false,
        latency_ms: 0,
      },
    })
  }

  const handleDeploy = () => {
    configure({
      discovery_id: state.discovery.id,
      name: state.deployment.name,
      enabled: state.deployment.enabled,
      execution_mode: state.deployment.execution_mode,
      configuration: state.configuration,
      priority: state.deployment.priority,
      rules: state.deployment.rules,
    })

    onOpenChange(false)
  }

  const canGoNext = () => {
    if (step === 1) return state.deployment.name.trim() !== ''
    if (step === 2) return state.is_valid
    if (step === 3) return true
    return false
  }

  const canDeploy = state.is_valid && state.deployment.name.trim() !== ''

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Configure {state.discovery.name}</DialogTitle>
          <DialogDescription>Step {step} of 4</DialogDescription>
        </DialogHeader>

        {/* Progress Indicator */}
        <div className="flex items-center gap-2">
          {[1, 2, 3, 4].map((s) => (
            <div
              key={s}
              className={`h-2 flex-1 rounded-full transition-colors ${
                s <= step ? 'bg-primary' : 'bg-muted'
              }`}
            />
          ))}
        </div>

        {/* Step 1: Basic Settings */}
        {step === 1 && (
          <div className="space-y-6">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Settings className="h-5 w-5" />
              Basic Settings
            </div>

            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">
                  Deployment Name <span className="text-destructive">*</span>
                </Label>
                <Input
                  id="name"
                  placeholder="e.g., PII Detection for Production"
                  value={state.deployment.name}
                  onChange={(e) => updateDeployment({ name: e.target.value })}
                />
                <p className="text-sm text-muted-foreground">
                  A unique name to identify this guardrail instance
                </p>
              </div>

              <div className="flex items-center space-x-2">
                <Switch
                  id="enabled"
                  checked={state.deployment.enabled}
                  onCheckedChange={(enabled) => updateDeployment({ enabled })}
                />
                <Label htmlFor="enabled">Enable immediately after deployment</Label>
              </div>

              <div className="space-y-2">
                <Label htmlFor="execution-mode">Execution Mode</Label>
                <Select
                  value={state.deployment.execution_mode}
                  onValueChange={(mode) =>
                    updateDeployment({ execution_mode: mode as GuardrailExecutionMode })
                  }
                >
                  <SelectTrigger id="execution-mode">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {state.discovery.capabilities.execution_modes.map((mode) => (
                      <SelectItem key={mode} value={mode}>
                        {mode}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-sm text-muted-foreground">
                  When should this guardrail be executed in the request flow
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="priority">Priority (Optional)</Label>
                <Input
                  id="priority"
                  type="number"
                  placeholder="1-100"
                  min={1}
                  max={100}
                  value={state.deployment.priority || ''}
                  onChange={(e) =>
                    updateDeployment({
                      priority: e.target.value ? parseInt(e.target.value) : undefined,
                    })
                  }
                />
                <p className="text-sm text-muted-foreground">
                  Higher priority guardrails run first (default: 50)
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Step 2: Configuration */}
        {step === 2 && (
          <div className="space-y-6">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Sliders className="h-5 w-5" />
              Configuration
            </div>

            <Alert>
              <AlertDescription>
                Configure the guardrail parameters according to your requirements
              </AlertDescription>
            </Alert>

            <DynamicForm
              schema={state.discovery.configuration_schema}
              values={state.configuration}
              onChange={updateConfiguration}
              errors={state.validation_errors}
            />

            {!state.is_valid && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>
                  Please fix the validation errors before proceeding
                </AlertDescription>
              </Alert>
            )}
          </div>
        )}

        {/* Step 3: Rules & Targeting (Optional) */}
        {step === 3 && (
          <div className="space-y-6">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Sliders className="h-5 w-5" />
              Apply Rules (Optional)
            </div>

            <Alert>
              <AlertDescription>
                Configure when this guardrail should be applied. Leave empty to apply to all
                requests.
              </AlertDescription>
            </Alert>

            <div className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="paths">API Paths</Label>
                <Textarea
                  id="paths"
                  placeholder="/v1/chat/completions&#10;/v1/completions"
                  rows={3}
                  value={state.deployment.rules?.paths?.join('\n') || ''}
                  onChange={(e) =>
                    updateDeployment({
                      rules: {
                        ...state.deployment.rules,
                        paths: e.target.value.split('\n').filter((p) => p.trim()),
                      },
                    })
                  }
                />
                <p className="text-sm text-muted-foreground">
                  One path per line. Leave empty to apply to all paths.
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="models">Models</Label>
                <Textarea
                  id="models"
                  placeholder="gpt-4&#10;claude-3"
                  rows={3}
                  value={state.deployment.rules?.models?.join('\n') || ''}
                  onChange={(e) =>
                    updateDeployment({
                      rules: {
                        ...state.deployment.rules,
                        models: e.target.value.split('\n').filter((m) => m.trim()),
                      },
                    })
                  }
                />
                <p className="text-sm text-muted-foreground">
                  One model per line. Leave empty to apply to all models.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Step 4: Test & Deploy */}
        {step === 4 && (
          <div className="space-y-6">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <TestTube className="h-5 w-5" />
              Test & Deploy
            </div>

            <Card>
              <CardHeader>
                <CardTitle>Configuration Summary</CardTitle>
                <CardDescription>Review your configuration before deployment</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <ConfigSummaryItem label="Name" value={state.deployment.name} />
                  <ConfigSummaryItem
                    label="Status"
                    value={
                      <Badge variant={state.deployment.enabled ? 'default' : 'secondary'}>
                        {state.deployment.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    }
                  />
                  <ConfigSummaryItem
                    label="Execution Mode"
                    value={state.deployment.execution_mode}
                  />
                  <ConfigSummaryItem
                    label="Priority"
                    value={state.deployment.priority?.toString() || 'Default (50)'}
                  />
                </div>

                <Separator />

                <div>
                  <h4 className="text-sm font-medium mb-2">Configuration</h4>
                  <pre className="bg-muted p-3 rounded-md text-xs overflow-auto max-h-40">
                    {JSON.stringify(state.configuration, null, 2)}
                  </pre>
                </div>
              </CardContent>
            </Card>

            <div className="space-y-3">
              <Button
                variant="outline"
                onClick={handleTest}
                disabled={isTesting}
                className="w-full"
              >
                {isTesting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Test Configuration
              </Button>

              {testResult && (
                <Alert variant={testResult.success ? 'default' : 'destructive'}>
                  {testResult.success ? (
                    <CheckCircle2 className="h-4 w-4" />
                  ) : (
                    <AlertCircle className="h-4 w-4" />
                  )}
                  <AlertDescription>
                    {testResult.success ? (
                      <>
                        Test passed! Executed in {testResult.latency_ms}ms
                        {testResult.blocked && ' (Request blocked)'}
                      </>
                    ) : (
                      <>Test failed: {testResult.error}</>
                    )}
                  </AlertDescription>
                </Alert>
              )}
            </div>
          </div>
        )}

        <DialogFooter className="flex items-center justify-between">
          <div>
            {step > 1 && (
              <Button variant="ghost" onClick={() => setStep(step - 1)}>
                <ChevronLeft className="mr-2 h-4 w-4" />
                Back
              </Button>
            )}
          </div>

          <div className="flex gap-2">
            {step < 4 ? (
              <Button onClick={() => setStep(step + 1)} disabled={!canGoNext()}>
                Next
                <ChevronRight className="ml-2 h-4 w-4" />
              </Button>
            ) : (
              <Button onClick={handleDeploy} disabled={!canDeploy || isConfiguring}>
                {isConfiguring && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                <Rocket className="mr-2 h-4 w-4" />
                Deploy Guardrail
              </Button>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function ConfigSummaryItem({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="text-sm font-medium">{value}</p>
    </div>
  )
}
