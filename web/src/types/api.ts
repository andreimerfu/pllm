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
  oauth_token?: string;
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
  reasoning_effort?: string;
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
  provider_profile_id?: string;
}

export interface ProviderProfile {
  id: string;
  name: string;
  type: string;
  config: {
    api_key?: string;
    base_url?: string;
    oauth_token?: string;
    azure_endpoint?: string;
    azure_deployment?: string;
    api_version?: string;
    aws_region_name?: string;
    aws_access_key_id?: string;
    aws_secret_access_key?: string;
    vertex_project?: string;
    vertex_location?: string;
  };
  model_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CreateProviderProfileRequest {
  name: string;
  type: string;
  config: Record<string, string>;
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

export interface RouteModel {
  id?: string;
  model_name: string;
  weight: number;
  priority: number;
  enabled: boolean;
}

export interface Route {
  id: string;
  name: string;
  slug: string;
  description?: string;
  strategy: 'priority' | 'least-latency' | 'weighted-round-robin' | 'random';
  models: RouteModel[];
  fallback_models?: string[];
  enabled: boolean;
  source: 'system' | 'user';
  created_at?: string;
  updated_at?: string;
}

export interface RoutesResponse {
  routes: Route[];
  total: number;
}

export interface AuditLogsResponse {
  audit_logs: AuditLog[];
  total: number;
  limit: number;
  offset: number;
  has_more: boolean;
}

export interface RouteModelStats {
  model: string;
  provider: string;
  requests: number;
  tokens: number;
  cost: number;
  avg_latency: number;
  percentage: number;
}

export interface RouteStatsResponse {
  total_requests: number;
  total_tokens: number;
  total_cost: number;
  models: RouteModelStats[];
}

export type MCPTransport = 'stdio' | 'sse' | 'http';
export type MCPHealth = 'unknown' | 'healthy' | 'unhealthy';

export interface MCPServerTool {
  id?: string;
  name: string;
  description?: string;
  input_schema?: unknown;
}

export interface MCPServer {
  id: string;
  name: string;
  slug: string;
  description?: string;
  enabled: boolean;
  transport: MCPTransport;
  endpoint?: string;
  headers?: Record<string, string>;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  working_dir?: string;
  health_status: MCPHealth;
  last_error?: string;
  last_seen_at?: string;
  tools?: MCPServerTool[];
  live_health?: MCPHealth;
  tool_count?: number;
  created_at?: string;
  updated_at?: string;
}

export interface MCPUpsertRequest {
  name: string;
  slug: string;
  description?: string;
  enabled?: boolean;
  transport: MCPTransport;
  endpoint?: string;
  headers?: Record<string, string>;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  working_dir?: string;
}

export interface MCPHealthProbeResponse {
  health: MCPHealth;
  error?: string;
  last_seen?: string;
  tool_count?: number;
  reason?: string;
}

export type RegistryStatus = 'active' | 'deprecated' | 'deleted';
// Both the registry resource kinds AND the owner-kinds that appear on
// RegistryRef edges. `llm` is a target kind (resolves against UserModel);
// `use_case` is an owner kind (the UseCase entity).
export type RegistryKind = 'server' | 'agent' | 'skill' | 'prompt' | 'llm' | 'use_case';

export interface RegistryBase {
  id: string;
  name: string;
  version: string;
  title?: string;
  description?: string;
  status: RegistryStatus;
  is_latest: boolean;
  published_at: string;
  published_by_user_id?: string;
  website_url?: string;
  repository?: unknown;
  metadata?: unknown;
  created_at?: string;
  updated_at?: string;
}

export interface EnrichmentScore {
  id: string;
  resource_kind: RegistryKind;
  resource_id: string;
  type: 'osv' | 'scorecard' | 'container' | 'dependencies';
  score: number;
  summary?: string;
  findings?: unknown;
  scanned_at: string;
}

export interface RegistryServer extends RegistryBase {
  packages?: unknown;
  remotes?: unknown;
}

export interface RegistryServerDetail {
  server: RegistryServer;
  scores?: EnrichmentScore[];
}

export interface RegistryRef {
  id?: string;
  owner_kind?: RegistryKind;
  owner_id?: string;
  target_kind: RegistryKind;
  target_name: string;
  target_version?: string;
  local_name?: string;
}

export interface RegistryAgent extends RegistryBase {
  language?: string;
  framework?: string;
  model_provider?: string;
  model_name?: string;
  packages?: unknown;
  remotes?: unknown;
}

export interface RegistryAgentWithRefs {
  agent: RegistryAgent;
  servers?: RegistryRef[];
  skills?: RegistryRef[];
  prompts?: RegistryRef[];
  agents?: RegistryRef[];
  llms?: RegistryRef[];
}

// Observed (OTel-derived) dependencies for an agent — what was actually
// seen at runtime, distinct from the declared `RegistryRef` rows above.
export interface ObservedAgentDeps {
  discovered_agent_id?: string | null;
  service_name?: string;
  last_seen_at?: string | null;
  flagged_count: number;
  calls: ObservedAgentCall[];
}

export interface ObservedAgentCall {
  kind: DiscoveredCallKind;
  target_name: string;
  count: number;
  last_outcome?: string;
  last_reason?: string;
  first_seen_at: string;
  last_seen_at: string;
}

export interface RegistrySkill extends RegistryBase {
  image?: string;
  manifest?: unknown;
}

export interface RegistryPrompt extends RegistryBase {
  template: string;
  arguments?: unknown;
}

export interface RegistryListResponse<T> {
  total: number;
  next: number;
  servers?: T[];
  agents?: T[];
  skills?: T[];
  prompts?: T[];
}

export type DeploymentPlatform = 'kubernetes' | 'local';
export type DeploymentStatus =
  | 'pending'
  | 'deploying'
  | 'running'
  | 'failed'
  | 'terminating'
  | 'stopped';

// OOM auto-scale fields are shared between Deployment and AgentDeployment;
// when null/absent the deployment inherits the global config.
export interface DeploymentOOMFields {
  oom_autoscale_enabled?: boolean | null;
  oom_memory_override?: string;
  oom_attempts?: number;
  oom_last_event_at?: string;
  last_healthy_at?: string;
}

export interface Deployment extends DeploymentOOMFields {
  id: string;
  registry_server_id: string;
  resource_name: string;
  resource_version: string;
  platform: DeploymentPlatform;
  namespace: string;
  workload_name: string;
  status: DeploymentStatus;
  status_reason?: string;
  endpoint?: string;
  gateway_backend_id?: string;
  last_applied_at?: string;
  created_at?: string;
  updated_at?: string;
}

export interface AgentDeployment extends DeploymentOOMFields {
  id: string;
  registry_agent_id: string;
  resource_name: string;
  resource_version: string;
  use_case_id: string;
  platform: string;
  namespace: string;
  workload_name: string;
  replicas: number;
  status: DeploymentStatus;
  status_reason?: string;
  endpoint?: string;
  provisioned_key_id?: string;
  secret_name?: string;
  created_at: string;
  updated_at: string;
  last_applied_at?: string;
}

export type DeploymentEventKind =
  | 'oom.bumped'
  | 'oom.cap_reached'
  | 'oom.reset'
  | 'oom.manual_bump';

export type DeploymentTargetKind = 'server' | 'agent';

export interface DeploymentEvent {
  id: string;
  deployment_id: string;
  target_kind: DeploymentTargetKind;
  workload_name?: string;
  namespace?: string;
  kind: DeploymentEventKind;
  reason?: string;
  data?: Record<string, unknown>;
  created_at: string;
}

export interface DeployAgentRequest {
  agent_id: string;
  use_case_id: string;
  namespace?: string;
  replicas?: number;
  image_override?: string;
  extra_env?: Record<string, string>;
}

// Wire shape of entries in RegistryServer.packages.
export interface RegistryPackage {
  registry_type?: string; // npm | pypi | oci | docker | ...
  identifier?: string;
  version?: string;
  transport?: { type?: string };
}

export interface RegistryDependent {
  owner_kind: RegistryKind;
  name: string;
  version: string;
  title?: string;
  description?: string;
  is_latest: boolean;
  target_version?: string;
  local_name?: string;
}

export type UseCaseRiskTier = 'tier1' | 'tier2' | 'tier3' | 'tier4';
export type UseCaseStatus = 'draft' | 'under_review' | 'approved' | 'retired';
// The registry kinds a use case can reference as components.
export type UseCaseRefKind = 'server' | 'agent' | 'skill' | 'prompt' | 'llm';

export interface UseCase {
  id: string;
  name: string;
  slug: string;
  description?: string;
  risk_tier: UseCaseRiskTier;
  status: UseCaseStatus;
  business_owner_user_id?: string;
  approver_user_id?: string;
  team_id?: string;
  data_classifications?: string[];
  effective_classifications?: string[];
  approval_record?: unknown;
  compliance_flag?: boolean;
  compliance_flag_reason?: string;
  last_graph_computed_at?: string;
  enforce_scope?: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface UseCaseRef {
  target_kind: UseCaseRefKind;
  target_name: string;
  target_version?: string;
  local_name?: string;
}

export interface UseCaseWithRefs {
  use_case: UseCase;
  servers?: UseCaseRef[];
  agents?: UseCaseRef[];
  skills?: UseCaseRef[];
  prompts?: UseCaseRef[];
  llms?: UseCaseRef[];
}

export interface UseCaseUpsertRequest {
  name: string;
  slug?: string;
  description?: string;
  risk_tier?: UseCaseRiskTier;
  status?: UseCaseStatus;
  business_owner_user_id?: string;
  approver_user_id?: string;
  team_id?: string;
  data_classifications?: string[];
  refs?: UseCaseRef[];
  enforce_scope?: boolean;
}

export type ScopeEventKind = 'mcp_tool' | 'llm';
export type ScopeEventOutcome = 'denied' | 'flagged';

export interface ScopeEvent {
  id: string;
  use_case_id: string;
  key_id?: string;
  key_name?: string;
  user_id?: string;
  kind: ScopeEventKind;
  target_name: string;
  outcome: ScopeEventOutcome;
  reason?: string;
  remote_addr?: string;
  request_id?: string;
  created_at?: string;
}

export interface ScopeEventStats {
  last_24h_denied: number;
  last_24h_flagged: number;
  last_7d_denied: number;
  last_7d_flagged: number;
}

export type WebhookEventType =
  | 'scope.denied'
  | 'scope.flagged'
  | 'use_case.created'
  | 'use_case.updated'
  | 'use_case.deleted'
  | 'use_case.compliance_flag'
  | 'registry.server.saved';

// Destination types correspond to backend adapters in
// internal/services/webhooks/adapters. The drawer renders the
// per-destination config form by reading the catalog from
// /api/admin/webhooks/destinations rather than hardcoding.
export type WebhookDestinationType =
  | 'generic'
  | 'slack'
  | 'teams'
  | 'splunk'
  | 'datadog'
  | 'servicenow';

export interface Webhook {
  id: string;
  name: string;
  url: string;
  secret: string;
  destination_type: WebhookDestinationType;
  // config_summary is a redacted view of the per-destination config —
  // sensitive keys (api_key, password, hec_token, ...) appear as "***"
  // when set so the UI can show "you have a credential configured"
  // without ever surfacing the raw value.
  config_summary?: Record<string, string | null>;
  events?: WebhookEventType[];
  enabled: boolean;
  team_id?: string;
  last_delivery_status?: number;
  last_delivery_error?: string;
  last_delivery_at?: string;
  failure_count: number;
  created_at?: string;
}

export interface WebhookUpsertRequest {
  name: string;
  url: string;
  secret?: string;
  events?: WebhookEventType[];
  enabled?: boolean;
  team_id?: string;
  destination_type?: WebhookDestinationType;
  // config carries the per-destination credentials/options. On update
  // the backend merges with the existing values, so the UI can ship
  // only changed fields. Pass "***" or omit to keep a sensitive value.
  config?: Record<string, string>;
}

// /api/admin/webhooks/destinations — catalog the drawer renders from.
export interface WebhookDestinationConfigKey {
  key: string;
  label: string;
  type: 'text' | 'password' | 'select';
  required: boolean;
  help?: string;
  default?: string;
  options?: string[];
}

export interface WebhookDestinationView {
  type: WebhookDestinationType;
  label: string;
  default_events?: WebhookEventType[];
  config_keys?: WebhookDestinationConfigKey[];
}

export interface WebhookDestinationsResponse {
  destinations: WebhookDestinationView[];
}

// --- Lifecycle hygiene -----------------------------------------------------
//
// Mirrors internal/services/lifecycle types. JSON tags from RunSummary,
// RuleResult, Action.

export interface LifecycleAction {
  rule: string;
  resource_id: string;
  subject: string;
  reason: string;
  dry_run: boolean;
  at: string; // ISO timestamp
}

export interface LifecycleRuleResult {
  rule: string;
  scanned: number;
  // May arrive as null when a rule scanned zero rows — Go's json.Marshal
  // serializes nil slices as null. Consumers should normalize to [].
  actions: LifecycleAction[] | null;
  error?: string;
}

export interface LifecycleSummary {
  ran_at: string; // ISO timestamp; zero value before first run
  duration: number; // nanoseconds (Go time.Duration JSON encoding)
  total_actions: number;
  rules: LifecycleRuleResult[];
}

// ─────────────────────────────────────────────────────────────────────
// Custom (CEL) lifecycle rules — operator-defined hygiene policies
// authored from a small set of templates. The UI never asks the user
// to write CEL: templates expose typed params, the backend resolves
// them into the CEL expression, and we round-trip both sides through
// these types.
// ─────────────────────────────────────────────────────────────────────

export type LifecycleScanTarget =
  | "keys"
  | "registry_servers"
  | "use_cases"
  | "users";

export type LifecycleActionType = "revoke" | "block" | "flag" | "notify";

export type TemplateParamType = "int" | "string" | "string[]" | "duration_days";

export interface LifecycleTemplateParam {
  key: string;
  label: string;
  type: TemplateParamType;
  default?: string | number | string[];
  min?: number;
  max?: number;
  placeholder?: string;
  help?: string;
  required?: boolean;
}

export interface LifecycleTemplate {
  id: string;
  title: string;
  category: "keys" | "servers" | "use_cases" | "users";
  description: string;
  scan: LifecycleScanTarget;
  action: LifecycleActionType;
  params: LifecycleTemplateParam[];
}

export interface LifecycleCustomRule {
  id: string;
  name: string;
  description: string;
  scan: LifecycleScanTarget;
  match: string; // resolved CEL expression
  action: LifecycleActionType;
  reason: string;
  enabled: boolean;
  dry_run?: boolean | null; // null = inherit global
  template_id: string;
  // params is the raw form values the user entered, kept so the rule
  // builder can re-render the dialog when editing.
  params?: Record<string, unknown> | null;
  created_at: string;
  updated_at: string;
  // Optional hydrated template metadata returned by GET endpoints.
  template?: LifecycleTemplate;
}

export interface CustomRuleUpsertRequest {
  name: string;
  description?: string;
  template_id: string;
  params: Record<string, unknown>;
  enabled?: boolean;
  dry_run?: boolean | null;
  reason?: string;
}

export interface CustomRulePreviewMatch {
  resource_id: string;
  subject: string;
  detail?: string;
}

export interface CustomRulePreviewResponse {
  template_id: string;
  scan: string;
  action: string;
  match: string;
  reason: string;
  total_rows: number;
  matched: number;
  capped: boolean;
  sample: CustomRulePreviewMatch[];
}

// ===========================
// Discovery (OTel) types
// ===========================

export type DiscoveredAgentStatus = "pending" | "promoted" | "ignored";
export type DiscoveredCallKind = "llm" | "mcp" | "vector_db" | "agent" | "http" | "unknown";

export interface DiscoveredAgent {
  id: string;
  service_name: string;
  service_version?: string;
  key_id?: string | null;
  promoted_agent_id?: string | null;
  status: DiscoveredAgentStatus;
  first_seen_at: string;
  last_seen_at: string;
  ignored_at?: string | null;
  promoted_at?: string | null;
  flagged_count: number;
  created_at: string;
  updated_at: string;
}

export interface DiscoveredCall {
  id: string;
  discovered_agent_id: string;
  kind: DiscoveredCallKind;
  target_name: string;
  count: number;
  first_seen_at: string;
  last_seen_at: string;
  last_outcome?: string;
  last_reason?: string;
}

