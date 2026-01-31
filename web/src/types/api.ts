export interface LoadBalancerStats {
  total_requests: number;
  circuit_open: boolean;
  health_score: number;
  average_latency?: number;
  avg_latency?: number;
  error_rate?: number;
  failed_requests?: number;
  p95_latency?: number;
}

export interface StatsResponse {
  load_balancer: Record<string, LoadBalancerStats>;
  should_shed_load: boolean;
  requests: {
    total: number;
    today: number;
    this_week: number;
    this_month: number;
  };
  tokens: {
    total: number;
    input: number;
    output: number;
  };
  costs: {
    total: number;
    today: number;
    this_week: number;
    this_month: number;
  };
  cache: {
    hits: number;
    misses: number;
    hit_rate: number;
  };
}

export interface ModelCapabilities {
  function_calling?: boolean;
  parallel_function_calling?: boolean;
  vision?: boolean;
  audio_input?: boolean;
  audio_output?: boolean;
  prompt_caching?: boolean;
  response_schema?: boolean;
  system_messages?: boolean;
  reasoning?: boolean;
  web_search?: boolean;
}

export interface Model {
  id: string;
  object: string;
  created: number;
  owned_by: string;
  tags?: string[];
  capabilities?: ModelCapabilities;
  provider?: string;
  max_tokens?: number;
  input_cost_per_token?: number;
  output_cost_per_token?: number;
  supported_regions?: string[];
  source?: 'system' | 'user';
}

export interface ProviderConfig {
  type: string;
  model: string;
  api_key?: string;
  api_secret?: string;
  base_url?: string;
  api_version?: string;
  org_id?: string;
  project_id?: string;
  region?: string;
  location?: string;
  azure_deployment?: string;
  azure_endpoint?: string;
  aws_access_key_id?: string;
  aws_secret_access_key?: string;
  aws_region_name?: string;
  vertex_project?: string;
  vertex_location?: string;
}

export interface ModelInfoConfig {
  mode?: string;
  supports_functions?: boolean;
  supports_vision?: boolean;
  supports_streaming?: boolean;
  max_tokens?: number;
  max_input_tokens?: number;
  max_output_tokens?: number;
  default_max_tokens?: number;
}

export interface CreateModelRequest {
  model_name: string;
  instance_name?: string;
  provider: ProviderConfig;
  model_info?: ModelInfoConfig;
  rpm?: number;
  tpm?: number;
  priority?: number;
  weight?: number;
  input_cost_per_token?: number;
  output_cost_per_token?: number;
  timeout_seconds?: number;
  tags?: string[];
  enabled?: boolean;
}

export interface UpdateModelRequest extends Partial<CreateModelRequest> {}

export interface AdminModel {
  id: string;
  model_name: string;
  instance_name?: string;
  source: 'system' | 'user';
  provider?: ProviderConfig;
  model_info?: ModelInfoConfig;
  owned_by?: string;
  rpm?: number;
  tpm?: number;
  priority?: number;
  weight?: number;
  input_cost_per_token?: number;
  output_cost_per_token?: number;
  timeout_seconds?: number;
  tags?: string[];
  enabled: boolean;
  created_at?: string;
  created_by_id?: string;
}

export interface AdminModelsResponse {
  models: AdminModel[];
  total: number;
}

export interface ModelUsageStats {
  requests_today: number;
  requests_total: number;
  tokens_today: number;
  tokens_total: number;
  cost_today: number;
  cost_total: number;
  avg_latency: number;
  error_rate: number;
  cache_hit_rate: number;
  health_score: number;
  trend_data: number[]; // Last 30 days request count for sparkline
  last_used: string | null;
}

export interface ModelWithUsage extends Model {
  usage_stats?: ModelUsageStats;
  is_active?: boolean;
  provider?: string;
}

export interface User {
  id: string;
  dex_id: string;
  name?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  picture?: string;
  groups?: string[];
  is_admin: boolean;
  created_at: string;
  updated_at: string;
  last_login?: string;
  last_login_at?: string;
  external_provider?: string;
  provider_icon?: string;
}

export interface ModelsResponse {
  object: string;
  data: Model[];
}

export interface Team {
  id: string;
  name: string;
  description?: string;
  owner_user_id: string;
  max_budget?: number;
  spend?: number;
  tpm_limit?: number;
  rpm_limit?: number;
  max_parallel_requests?: number;
  budget_reset_at?: string;
  members?: TeamMember[];
  created_at: string;
  updated_at: string;
}

export interface TeamMember {
  id: string;
  team_id: string;
  user_id: string;
  role: 'owner' | 'admin' | 'member';
  created_at: string;
  updated_at: string;
}

export interface VirtualKey {
  id: string;
  key: string;
  name: string;
  owner_type: 'user' | 'team';
  owner_id: string;
  max_budget?: number;
  spend?: number;
  max_parallel_requests?: number;
  tpm_limit?: number;
  rpm_limit?: number;
  ttl?: number;
  metadata?: Record<string, any>;
  models?: string[];
  revoked_at?: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

export interface HealthCheckInstance {
  instance_id: string;
  model_name: string;
  provider_type: string;
  healthy: boolean;
  latency_ms: number;
  error?: string;
  checked_at: string;
}

export interface ModelHealthSummary {
  model_name: string;
  healthy: boolean;
  healthy_count: number;
  total_count: number;
  avg_latency_ms: number;
  last_checked_at: string;
  instances: HealthCheckInstance[];
}

export interface ModelsHealthResponse {
  models: Record<string, ModelHealthSummary>;
  total: number;
}

export interface AuditLog {
  id: string;
  event_type: string;
  event_action: string;
  event_result: 'success' | 'failure' | 'error' | 'warning';
  user_id?: string;
  user?: User;
  team_id?: string;
  team?: Team;
  key_id?: string;
  key?: VirtualKey;
  ip_address: string;
  user_agent: string;
  request_id: string;
  method: string;
  path: string;
  auth_method?: string;
  auth_provider?: string;
  resource_type?: string;
  resource_id?: string;
  old_values?: any;
  new_values?: any;
  message?: string;
  error_code?: string;
  metadata?: any;
  duration?: number;
  timestamp: string;
  created_at: string;
  updated_at: string;
}

export interface AuditLogsResponse {
  audit_logs: AuditLog[];
  total: number;
  limit: number;
  offset: number;
  has_more: boolean;
}