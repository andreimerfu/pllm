// Discovery Protocol Types
// Based on the UI/UX design document

export type GuardrailExecutionMode = 'pre-processing' | 'post-processing' | 'streaming'

export type GuardrailCategory =
  | 'privacy'
  | 'security'
  | 'content_safety'
  | 'compliance'
  | 'custom'

export interface GuardrailProvider {
  name: string
  organization?: string
  website?: string
  contact?: string
}

export interface GuardrailCapabilities {
  execution_modes: GuardrailExecutionMode[]
  supported_languages: string[]
  supports_streaming: boolean
  supports_batch: boolean
  max_batch_size?: number
  requires_api_key?: boolean
}

export interface GuardrailPerformance {
  avg_latency_ms?: number
  max_latency_ms?: number
  throughput_req_per_sec?: number
  accuracy?: number
  false_positive_rate?: number
  false_negative_rate?: number
}

export interface GuardrailEndpoints {
  health_check?: string
  metrics?: string
  documentation?: string
  execute?: string
}

// JSON Schema types for dynamic form generation
export type JSONSchemaType =
  | 'string'
  | 'number'
  | 'integer'
  | 'boolean'
  | 'array'
  | 'object'
  | 'null'

export interface JSONSchemaProperty {
  type: JSONSchemaType | JSONSchemaType[]
  title?: string
  description?: string
  default?: any
  enum?: any[]
  minimum?: number
  maximum?: number
  minLength?: number
  maxLength?: number
  pattern?: string
  format?: string
  items?: JSONSchemaProperty
  properties?: Record<string, JSONSchemaProperty>
  required?: string[]
  additionalProperties?: boolean | JSONSchemaProperty
  // UI hints
  'ui:widget'?: string
  'ui:placeholder'?: string
  'ui:help'?: string
  'ui:disabled'?: boolean
  'ui:readonly'?: boolean
}

export interface JSONSchema {
  $schema?: string
  type: 'object'
  title?: string
  description?: string
  properties: Record<string, JSONSchemaProperty>
  required?: string[]
  additionalProperties?: boolean
}

// Main discovery response
export interface GuardrailDiscoveryResponse {
  // Core identification
  id: string
  name: string
  version: string
  description: string
  provider: GuardrailProvider
  category: GuardrailCategory
  tags?: string[]

  // Capabilities
  capabilities: GuardrailCapabilities

  // Performance characteristics
  performance?: GuardrailPerformance

  // Configuration schema (JSON Schema)
  configuration_schema: JSONSchema

  // Documentation and endpoints
  documentation?: string
  endpoints?: GuardrailEndpoints

  // Metadata
  license?: string
  homepage?: string
  repository?: string
  changelog?: string
  support?: string

  // Compatibility
  min_gateway_version?: string
  max_gateway_version?: string
  deprecated?: boolean
  deprecation_message?: string
}

// Discovery source types
export type GuardrailSourceType = 'url' | 'preset' | 'manual'

export interface GuardrailSource {
  id: string
  name: string
  type: GuardrailSourceType
  url?: string
  discovery_endpoint?: string
  enabled: boolean
  verified: boolean
  added_at: Date
  last_checked?: Date
  error?: string
}

// Marketplace guardrail (combines discovery + installation status)
export interface MarketplaceGuardrail {
  // Discovery data
  discovery: GuardrailDiscoveryResponse
  source: GuardrailSource

  // Installation status
  installed: boolean
  enabled?: boolean
  installation_date?: Date

  // Runtime data (if installed)
  health_status?: 'healthy' | 'degraded' | 'unhealthy' | 'unknown'
  last_health_check?: Date

  // Statistics (if installed)
  stats?: {
    total_requests: number
    total_blocks: number
    avg_latency_ms: number
    error_rate: number
  }
}

// Configuration state during wizard
export interface GuardrailConfigurationState {
  // Discovery data
  discovery: GuardrailDiscoveryResponse

  // User configuration (matches schema)
  configuration: Record<string, any>

  // Deployment settings
  deployment: {
    name: string
    enabled: boolean
    execution_mode: GuardrailExecutionMode
    priority?: number
    rules?: {
      paths?: string[]
      models?: string[]
      users?: string[]
      teams?: string[]
    }
  }

  // Validation state
  validation_errors?: Record<string, string>
  is_valid: boolean

  // Test results
  test_results?: {
    tested: boolean
    passed: boolean
    latency_ms?: number
    error?: string
    examples?: Array<{
      input: string
      output: any
      blocked: boolean
    }>
  }
}

// API request/response types
export interface DiscoverGuardrailRequest {
  url: string
  verify_ssl?: boolean
  timeout_seconds?: number
}

export interface AddGuardrailSourceRequest {
  name: string
  type: GuardrailSourceType
  url?: string
  discovery_endpoint?: string
}

export interface ConfigureGuardrailRequest {
  discovery_id: string
  name: string
  enabled: boolean
  execution_mode: GuardrailExecutionMode
  configuration: Record<string, any>
  priority?: number
  rules?: GuardrailConfigurationState['deployment']['rules']
}

export interface TestGuardrailRequest {
  discovery_id: string
  configuration: Record<string, any>
  test_input: string
}

export interface TestGuardrailResponse {
  success: boolean
  latency_ms: number
  output: any
  blocked: boolean
  error?: string
}
