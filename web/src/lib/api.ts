import axios from "axios";
import type { StatsResponse, ModelsResponse, CreateModelRequest, UpdateModelRequest, AdminModelsResponse, ProviderConfig, ModelsHealthResponse, RoutesResponse, Route, RouteStatsResponse, MCPServer, MCPUpsertRequest, MCPHealthProbeResponse, RegistryServer, RegistryServerDetail, RegistryAgent, RegistryAgentWithRefs, RegistrySkill, RegistryPrompt, RegistryListResponse, Deployment, RegistryDependent, RegistryKind, UseCase, UseCaseWithRefs, UseCaseUpsertRequest, ScopeEvent, ScopeEventStats, Webhook, WebhookUpsertRequest, AgentDeployment, DeployAgentRequest, LifecycleSummary } from "@/types/api";

const API_BASE = import.meta.env.DEV ? "http://localhost:8080" : "";

export const axiosInstance = axios.create({
  baseURL: API_BASE,
  headers: {
    "Content-Type": "application/json",
  },
});

// Add auth token if available
axiosInstance.interceptors.request.use((config) => {
  const token =
    localStorage.getItem("token") || localStorage.getItem("authToken");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle responses and errors
axiosInstance.interceptors.response.use(
  (response) => response.data,
  (error) => {
    if (error.response?.status === 401) {
      // Handle unauthorized access
      localStorage.removeItem("token");
      localStorage.removeItem("authToken");
      window.location.href = "/ui/login";
    }
    return Promise.reject(error);
  },
);

// Users API
const users = {
  list: () => axiosInstance.get("/api/admin/users"),
  get: (id: string) => axiosInstance.get(`/api/admin/users/${id}`),
  update: (id: string, data: any) =>
    axiosInstance.put(`/api/admin/users/${id}`, data),
  delete: (id: string) => axiosInstance.delete(`/api/admin/users/${id}`),
  getStats: (id: string) => axiosInstance.get(`/api/admin/users/${id}/stats`),
};

// Teams API
const teams = {
  list: () => axiosInstance.get("/api/admin/teams"),
  get: (id: string) => axiosInstance.get(`/api/admin/teams/${id}`),
  create: (data: any) => axiosInstance.post("/api/admin/teams", data),
  update: (id: string, data: any) =>
    axiosInstance.put(`/api/admin/teams/${id}`, data),
  delete: (id: string) => axiosInstance.delete(`/api/admin/teams/${id}`),
  addMember: (teamId: string, data: any) =>
    axiosInstance.post(`/api/admin/teams/${teamId}/members`, data),
  updateMember: (teamId: string, memberId: string, data: any) =>
    axiosInstance.put(`/api/admin/teams/${teamId}/members/${memberId}`, data),
  removeMember: (teamId: string, memberId: string) =>
    axiosInstance.delete(`/api/admin/teams/${teamId}/members/${memberId}`),
  getStats: (teamId: string) =>
    axiosInstance.get(`/api/admin/teams/${teamId}/stats`),
};

// Virtual Keys API (Admin)
const adminKeys = {
  list: () => axiosInstance.get("/api/admin/keys"),
  generate: (data: any) => axiosInstance.post("/api/admin/keys", data),
  get: (id: string) => axiosInstance.get(`/api/admin/keys/${id}`),
  update: (id: string, data: any) =>
    axiosInstance.put(`/api/admin/keys/${id}`, data),
  delete: (id: string) => axiosInstance.delete(`/api/admin/keys/${id}`),
  revoke: (id: string, data?: any) =>
    axiosInstance.post(`/api/admin/keys/${id}/revoke`, data || {}),
  getStats: (id: string) => axiosInstance.get(`/api/admin/keys/${id}/stats`),
  validate: (key: string) =>
    axiosInstance.post("/api/admin/keys/validate", { key }),
};

// User Keys API
const userKeys = {
  list: () => axiosInstance.get("/v1/user/keys"),
  create: (data: any) => axiosInstance.post("/v1/user/keys", data),
  delete: (id: string) => axiosInstance.delete(`/v1/user/keys/${id}`),
  getUsage: () => axiosInstance.get("/v1/user/usage"),
  getDailyUsage: () => axiosInstance.get("/v1/user/usage/daily"),
  getMonthlyUsage: () => axiosInstance.get("/v1/user/usage/monthly"),
};

// User Profile API
const userProfile = {
  get: () => axiosInstance.get("/v1/user/profile"),
  getBudgetStatus: () => axiosInstance.get("/v1/user/budget"),
  getTeams: () => axiosInstance.get("/v1/user/teams"),
};

// Unified Keys API (switches based on role)
const keys = {
  list: (useUserEndpoint?: boolean) => 
    useUserEndpoint ? userKeys.list() : adminKeys.list(),
  generate: (data: any, useUserEndpoint?: boolean) =>
    useUserEndpoint ? userKeys.create(data) : adminKeys.generate(data),
  create: (data: any) => userKeys.create(data),
  get: (id: string) => adminKeys.get(id),
  update: (id: string, data: any) => adminKeys.update(id, data),
  delete: (id: string, useUserEndpoint?: boolean) =>
    useUserEndpoint ? userKeys.delete(id) : adminKeys.delete(id),
  revoke: (id: string, data?: any) => adminKeys.revoke(id, data),
  getStats: (id: string) => adminKeys.getStats(id),
  validate: (key: string) => adminKeys.validate(key),
};

// Export the main API object
const api = {
  users,
  teams,
  keys,
  userKeys,
  userProfile,
  adminKeys,
  // Legacy exports for backward compatibility
  axios: axiosInstance,
};

export default api;

// Auth API
export const getAuthConfig = (): Promise<any> => axiosInstance.get("/api/auth/config");

// Legacy exports for backward compatibility
export const healthCheck = () => axiosInstance.get("/health");
export const getDatabaseStatus = () =>
  axiosInstance.get("/api/admin/system/health");
export const getModels = () =>
  axiosInstance.get("/v1/models") as Promise<ModelsResponse>;
export const getModel = (modelId: string) =>
  axiosInstance.get(`/v1/models/${encodeURIComponent(modelId)}`);
export const getModelStats = () =>
  axiosInstance.get("/api/admin/stats") as Promise<StatsResponse>;
export const getSystemConfig = () => 
  axiosInstance.get("/api/admin/system/config");
export const getMetrics = () => axiosInstance.get("/metrics");

// Historical metrics API
export const getHistoricalModelHealth = (days = 30) =>
  axiosInstance.get(`/api/admin/analytics/historical/model-health?days=${days}`);
export const getHistoricalSystemMetrics = (interval = "hourly", hours = 24) =>
  axiosInstance.get(`/api/admin/analytics/historical/system-metrics?interval=${interval}&hours=${hours}`);
export const getHistoricalModelLatencies = (models: string[], interval = "hourly", hours = 24) =>
  axiosInstance.get(`/api/admin/analytics/historical/model-latencies?models=${models.join(",")}&interval=${interval}&hours=${hours}`);

// Dashboard metrics API
export const getDashboardMetrics = (): Promise<{
  total_requests: number;
  total_tokens: number; 
  total_cost: number;
  active_keys: number;
  active_models: number;
  top_models?: Array<{
    model: string;
    requests: number;
    cost: number;
  }>;
}> => axiosInstance.get("/api/admin/dashboard/metrics");
export const getModelMetrics = (model: string) => axiosInstance.get(`/api/admin/dashboard/models/${encodeURIComponent(model)}`);
export const getModelTrends = (model: string, params: { days?: number; hours?: number; interval?: string } = {}) => {
  const searchParams = new URLSearchParams();
  if (params.hours) searchParams.set("hours", String(params.hours));
  else if (params.days) searchParams.set("days", String(params.days));
  else searchParams.set("days", "30");
  if (params.interval) searchParams.set("interval", params.interval);
  return axiosInstance.get(`/api/admin/dashboard/models/${encodeURIComponent(model)}/trends?${searchParams}`);
};
export const getUsageTrends = (params: { days?: number; hours?: number; interval?: string } = {}) => {
  const searchParams = new URLSearchParams();
  if (params.hours) searchParams.set("hours", String(params.hours));
  else if (params.days) searchParams.set("days", String(params.days));
  else searchParams.set("days", "30");
  if (params.interval) searchParams.set("interval", params.interval);
  return axiosInstance.get(`/api/admin/dashboard/usage-trends?${searchParams}`);
};

// Legacy dashboard API (renamed to avoid conflict)
export const getLegacyDashboard = () => axiosInstance.get("/api/admin/dashboard");

// Users (legacy exports)
export const getUsers = () => axiosInstance.get("/api/admin/users");
export const getUser = (id: string) =>
  axiosInstance.get(`/api/admin/users/${id}`);
export const updateUser = (id: string, data: any) =>
  axiosInstance.put(`/api/admin/users/${id}`, data);
export const deleteUser = (id: string) =>
  axiosInstance.delete(`/api/admin/users/${id}`);
export const getUserStats = (id: string) =>
  axiosInstance.get(`/api/admin/users/${id}/stats`);

// Teams (legacy exports)
export const getTeams = () => axiosInstance.get("/api/admin/teams");
export const createTeam = (data: any) =>
  axiosInstance.post("/api/admin/teams", data);
export const getTeam = (id: string) =>
  axiosInstance.get(`/api/admin/teams/${id}`);
export const updateTeam = (id: string, data: any) =>
  axiosInstance.put(`/api/admin/teams/${id}`, data);
export const deleteTeam = (id: string) =>
  axiosInstance.delete(`/api/admin/teams/${id}`);
export const addTeamMember = (teamId: string, data: any) =>
  axiosInstance.post(`/api/admin/teams/${teamId}/members`, data);
export const updateTeamMember = (teamId: string, memberId: string, data: any) =>
  axiosInstance.put(`/api/admin/teams/${teamId}/members/${memberId}`, data);
export const removeTeamMember = (teamId: string, memberId: string) =>
  axiosInstance.delete(`/api/admin/teams/${teamId}/members/${memberId}`);
export const getTeamStats = (teamId: string) =>
  axiosInstance.get(`/api/admin/teams/${teamId}/stats`);

// Virtual Keys (legacy exports)
export const getKeys = () => axiosInstance.get("/api/admin/keys");
export const generateKey = (data: any) =>
  axiosInstance.post("/api/admin/keys", data);
export const getKey = (id: string) =>
  axiosInstance.get(`/api/admin/keys/${id}`);
// revealKey returns the plaintext token for a key. Admin-only on the
// backend; the playground uses it lazily so the operator can run
// scoped chat from the UI without keeping every plaintext in notes.
// The response interceptor on axiosInstance unwraps to .data already,
// so callers get { plaintext_key } directly (no extra .data hop).
export const revealKey = (id: string): Promise<{ plaintext_key: string }> =>
  axiosInstance.get(`/api/admin/keys/${id}/secret`) as unknown as Promise<{ plaintext_key: string }>;
export const updateKey = (id: string, data: any) =>
  axiosInstance.put(`/api/admin/keys/${id}`, data);
export const deleteKey = (id: string) =>
  axiosInstance.delete(`/api/admin/keys/${id}`);
export const revokeKey = (id: string) =>
  axiosInstance.post(`/api/admin/keys/${id}/revoke`);
export const getKeyStats = (id: string) =>
  axiosInstance.get(`/api/admin/keys/${id}/stats`);
export const validateKey = (key: string) =>
  axiosInstance.post("/api/keys/validate", { key });

// Budget Analytics
export const getBudgetSummary = () => axiosInstance.get("/api/admin/analytics/budget");
export const getUserBreakdown = () => axiosInstance.get("/api/admin/analytics/user-breakdown");
export const getTeamUserBreakdown = (teamId?: string) => 
  axiosInstance.get(`/api/admin/analytics/team-user-breakdown${teamId ? `?team_id=${teamId}` : ""}`);

// Analytics
export const getUsage = () => axiosInstance.get("/api/admin/analytics/usage");
export const getHourlyUsage = () =>
  axiosInstance.get("/api/admin/analytics/usage/hourly");
export const getDailyUsage = () =>
  axiosInstance.get("/api/admin/analytics/usage/daily");
export const getMonthlyUsage = () =>
  axiosInstance.get("/api/admin/analytics/usage/monthly");
export const getCosts = () => axiosInstance.get("/api/admin/analytics/costs");
export const getCostBreakdown = () =>
  axiosInstance.get("/api/admin/analytics/costs/breakdown");
export const getPerformance = () =>
  axiosInstance.get("/api/admin/analytics/performance");
export const getErrors = () => axiosInstance.get("/api/admin/analytics/errors");
export const getCacheStats = () =>
  axiosInstance.get("/api/admin/analytics/cache");
// Removed duplicate getDashboard - using getDashboardMetrics for new API

// System
export const getConfig = () => axiosInstance.get("/api/admin/system/config");
export const updateConfig = (data: any) =>
  axiosInstance.put("/api/admin/system/config", data);
export const getSystemHealth = () =>
  axiosInstance.get("/api/admin/system/health");
export const getLogs = () => axiosInstance.get("/api/admin/system/logs");
export const getAuditLogs = (filters?: {
  action?: string;
  resource?: string;
  user_id?: string;
  team_id?: string;
  start_date?: string;
  end_date?: string;
  limit?: number;
  offset?: number;
}) => {
  const params = new URLSearchParams();
  if (filters) {
    Object.entries(filters).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        params.append(key, value.toString());
      }
    });
  }
  const queryString = params.toString();
  return axiosInstance.get(`/api/admin/system/audit${queryString ? `?${queryString}` : ''}`);
};
export const clearCache = () =>
  axiosInstance.post("/api/admin/system/cache/clear");
export const setMaintenance = (enabled: boolean) =>
  axiosInstance.post("/api/admin/system/maintenance", { enabled });

// Settings
export const getSettings = () => axiosInstance.get("/api/admin/settings");
export const updateSettings = (data: any) =>
  axiosInstance.put("/api/admin/settings", data);
export const getRateLimits = () =>
  axiosInstance.get("/api/admin/settings/rate-limits");
export const updateRateLimits = (data: any) =>
  axiosInstance.put("/api/admin/settings/rate-limits", data);
export const getCacheSettings = () =>
  axiosInstance.get("/api/admin/settings/cache");
export const updateCacheSettings = (data: any) =>
  axiosInstance.put("/api/admin/settings/cache", data);

// Admin Models CRUD
export const testModelConnection = (provider: ProviderConfig): Promise<{
  success: boolean;
  message: string;
  provider: string;
  latency?: string;
}> =>
  axiosInstance.post("/api/admin/models/test-connection", { provider });

export const discoverModels = (provider: ProviderConfig): Promise<{
  models: string[];
  count: number;
}> =>
  axiosInstance.post("/api/admin/models/discover-models", { provider });
export const getAdminModels = () =>
  axiosInstance.get("/api/admin/models") as Promise<AdminModelsResponse>;
export const createModel = (data: CreateModelRequest) =>
  axiosInstance.post("/api/admin/models", data);
export const updateModel = (id: string, data: UpdateModelRequest) =>
  axiosInstance.put(`/api/admin/models/${id}`, data);
export const deleteModel = (id: string) =>
  axiosInstance.delete(`/api/admin/models/${id}`);
export const getModelsHealth = () =>
  axiosInstance.get("/api/admin/models/health") as Promise<ModelsHealthResponse>;

// Routes
export const getRoutes = () =>
  axiosInstance.get("/api/admin/routes") as Promise<RoutesResponse>;
export const getRoute = (id: string) =>
  axiosInstance.get(`/api/admin/routes/${id}`) as Promise<Route>;
export const createRoute = (data: any) =>
  axiosInstance.post("/api/admin/routes", data);
export const updateRoute = (id: string, data: any) =>
  axiosInstance.put(`/api/admin/routes/${id}`, data);
export const deleteRoute = (id: string) =>
  axiosInstance.delete(`/api/admin/routes/${id}`);
export const getRouteStats = (id: string, hours = 24) =>
  axiosInstance.get(`/api/admin/routes/${id}/stats?hours=${hours}`) as Promise<RouteStatsResponse>;

// Provider Profiles
export const getProviderProfiles = () => axiosInstance.get('/api/admin/providers').then((r: any) => r.providers || r);
export const getProviderProfile = (id: string) => axiosInstance.get(`/api/admin/providers/${id}`);
export const createProviderProfile = (data: any) => axiosInstance.post('/api/admin/providers', data);
export const updateProviderProfile = (id: string, data: any) => axiosInstance.put(`/api/admin/providers/${id}`, data);
export const deleteProviderProfile = (id: string) => axiosInstance.delete(`/api/admin/providers/${id}`);

// Guardrails
export const getGuardrails = () => axiosInstance.get("/api/admin/guardrails");
export const getGuardrail = (name: string) => 
  axiosInstance.get(`/api/admin/guardrails/${name}`);
export const getGuardrailStats = () => 
  axiosInstance.get("/api/admin/guardrails/stats");
export const checkGuardrailHealth = () => 
  axiosInstance.get("/api/admin/guardrails/health");
export const testGuardrail = (name: string, data: { text: string; messages?: any[] }) =>
  axiosInstance.post(`/api/admin/guardrails/${name}/test`, data);

// MCP Gateway
export const getMCPServers = () =>
  axiosInstance.get("/api/admin/mcp/servers") as Promise<MCPServer[]>;
export const getMCPServer = (id: string) =>
  axiosInstance.get(`/api/admin/mcp/servers/${id}`) as Promise<MCPServer>;
export const createMCPServer = (data: MCPUpsertRequest) =>
  axiosInstance.post("/api/admin/mcp/servers", data) as Promise<MCPServer>;
export const updateMCPServer = (id: string, data: MCPUpsertRequest) =>
  axiosInstance.put(`/api/admin/mcp/servers/${id}`, data) as Promise<MCPServer>;
export const deleteMCPServer = (id: string) =>
  axiosInstance.delete(`/api/admin/mcp/servers/${id}`);
export const probeMCPServer = (id: string) =>
  axiosInstance.post(`/api/admin/mcp/servers/${id}/health`, {}) as Promise<MCPHealthProbeResponse>;

// Registry (read-only catalog). Writes happen under /api/admin/registry and
// require admin.
type RegistryQuery = { search?: string; latest?: boolean; limit?: number; offset?: number };

const qs = (q?: RegistryQuery) => {
  if (!q) return "";
  const parts: string[] = [];
  if (q.search) parts.push(`search=${encodeURIComponent(q.search)}`);
  if (q.latest) parts.push("latest=true");
  if (q.limit != null) parts.push(`limit=${q.limit}`);
  if (q.offset != null) parts.push(`offset=${q.offset}`);
  return parts.length ? `?${parts.join("&")}` : "";
};

export const listRegistryServers = (q?: RegistryQuery) =>
  axiosInstance.get(`/v1/registry/servers${qs(q)}`) as Promise<RegistryListResponse<RegistryServer>>;
export const getRegistryServer = (name: string, version?: string) =>
  axiosInstance.get(`/v1/registry/servers/${encodeURIComponent(name)}${version ? `?version=${encodeURIComponent(version)}` : ""}`) as Promise<RegistryServer | RegistryServerDetail>;
export const listRegistryServerVersions = (name: string) =>
  axiosInstance.get(`/v1/registry/servers/${encodeURIComponent(name)}/versions`) as Promise<{ versions: RegistryServer[] }>;
export const listRegistryAgentVersions = (name: string) =>
  axiosInstance.get(`/v1/registry/agents/${encodeURIComponent(name)}/versions`) as Promise<{ versions: RegistryAgent[] }>;
export const listRegistrySkillVersions = (name: string) =>
  axiosInstance.get(`/v1/registry/skills/${encodeURIComponent(name)}/versions`) as Promise<{ versions: RegistrySkill[] }>;
export const listRegistryPromptVersions = (name: string) =>
  axiosInstance.get(`/v1/registry/prompts/${encodeURIComponent(name)}/versions`) as Promise<{ versions: RegistryPrompt[] }>;
export const listRegistryDependents = (targetKind: RegistryKind, targetName: string) =>
  axiosInstance.get(`/v1/registry/refs?target_kind=${encodeURIComponent(targetKind)}&target_name=${encodeURIComponent(targetName)}`) as Promise<{ dependents: RegistryDependent[] }>;
export const upsertRegistryServer = (body: Partial<RegistryServer>) =>
  axiosInstance.post(`/api/admin/registry/servers`, body) as Promise<RegistryServer>;
export const deleteRegistryServerVersion = (name: string, version: string) =>
  axiosInstance.delete(`/api/admin/registry/servers/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}`);

export const listRegistryAgents = (q?: RegistryQuery) =>
  axiosInstance.get(`/v1/registry/agents${qs(q)}`) as Promise<RegistryListResponse<RegistryAgent>>;
export const getRegistryAgent = (name: string, version?: string) =>
  axiosInstance.get(`/v1/registry/agents/${encodeURIComponent(name)}${version ? `?version=${encodeURIComponent(version)}` : ""}`) as Promise<RegistryAgentWithRefs>;
export const upsertRegistryAgent = (body: Partial<RegistryAgent> & { refs?: unknown[] }) =>
  axiosInstance.post(`/api/admin/registry/agents`, body) as Promise<RegistryAgentWithRefs>;
export const deleteRegistryAgentVersion = (name: string, version: string) =>
  axiosInstance.delete(`/api/admin/registry/agents/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}`);

export const listRegistrySkills = (q?: RegistryQuery) =>
  axiosInstance.get(`/v1/registry/skills${qs(q)}`) as Promise<RegistryListResponse<RegistrySkill>>;
export const getRegistrySkill = (name: string, version?: string) =>
  axiosInstance.get(`/v1/registry/skills/${encodeURIComponent(name)}${version ? `?version=${encodeURIComponent(version)}` : ""}`) as Promise<RegistrySkill>;
export const upsertRegistrySkill = (body: Partial<RegistrySkill>) =>
  axiosInstance.post(`/api/admin/registry/skills`, body) as Promise<RegistrySkill>;
export const deleteRegistrySkillVersion = (name: string, version: string) =>
  axiosInstance.delete(`/api/admin/registry/skills/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}`);

export const listRegistryPrompts = (q?: RegistryQuery) =>
  axiosInstance.get(`/v1/registry/prompts${qs(q)}`) as Promise<RegistryListResponse<RegistryPrompt>>;
export const getRegistryPrompt = (name: string, version?: string) =>
  axiosInstance.get(`/v1/registry/prompts/${encodeURIComponent(name)}${version ? `?version=${encodeURIComponent(version)}` : ""}`) as Promise<RegistryPrompt>;
export const upsertRegistryPrompt = (body: Partial<RegistryPrompt>) =>
  axiosInstance.post(`/api/admin/registry/prompts`, body) as Promise<RegistryPrompt>;
export const deleteRegistryPromptVersion = (name: string, version: string) =>
  axiosInstance.delete(`/api/admin/registry/prompts/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}`);

export interface ImportReport {
  source: string;
  found: number;
  imported: number;
  skipped: number;
  errors?: string[];
}
export const triggerRegistryImport = (query?: string, limit?: number) =>
  axiosInstance.post(`/api/admin/registry/import`, { query, limit }) as Promise<{ reports: ImportReport[] }>;

// Deployments
export const listDeployments = () =>
  axiosInstance.get("/api/admin/deployments") as Promise<{ deployments: Deployment[] }>;
export const getDeployment = (id: string) =>
  axiosInstance.get(`/api/admin/deployments/${id}`) as Promise<Deployment>;
export const deployRegistryServer = (body: { server_name: string; server_version?: string; namespace?: string }) =>
  axiosInstance.post(`/api/admin/deployments`, body) as Promise<Deployment>;
export const refreshDeploymentStatus = (id: string) =>
  axiosInstance.post(`/api/admin/deployments/${id}/status`, {}) as Promise<Deployment>;
export const deleteDeployment = (id: string) =>
  axiosInstance.delete(`/api/admin/deployments/${id}`);
export const bumpDeploymentOOM = (id: string) =>
  axiosInstance.post(`/api/admin/deployments/${id}/oom/bump`, {}) as Promise<Deployment>;
export const resetDeploymentOOM = (id: string) =>
  axiosInstance.post(`/api/admin/deployments/${id}/oom/reset`, {}) as Promise<Deployment>;
export const setDeploymentOOMAutoscale = (id: string, enabled: boolean | null) =>
  axiosInstance.put(`/api/admin/deployments/${id}/oom/autoscale`, { enabled });
export const listDeploymentEvents = (params?: {
  deployment_id?: string;
  target_kind?: "server" | "agent";
  kind?: string;
  since?: string;
  limit?: number;
  offset?: number;
}) =>
  axiosInstance.get(`/api/admin/deployments/events`, { params }) as Promise<{
    events: import("@/types/api").DeploymentEvent[];
  }>;

// Agents
export const listAgents = () =>
  axiosInstance.get("/api/admin/agents") as Promise<{ agents: AgentDeployment[] }>;
export const getAgent = (id: string) =>
  axiosInstance.get(`/api/admin/agents/${id}`) as Promise<AgentDeployment>;
export const deployAgent = (body: DeployAgentRequest) =>
  axiosInstance.post(`/api/admin/agents`, body) as Promise<AgentDeployment>;
export const refreshAgentStatus = (id: string) =>
  axiosInstance.post(`/api/admin/agents/${id}/status`, {}) as Promise<AgentDeployment>;
export const deleteAgent = (id: string) =>
  axiosInstance.delete(`/api/admin/agents/${id}`);
export const bumpAgentOOM = (id: string) =>
  axiosInstance.post(`/api/admin/agents/${id}/oom/bump`, {}) as Promise<AgentDeployment>;
export const resetAgentOOM = (id: string) =>
  axiosInstance.post(`/api/admin/agents/${id}/oom/reset`, {}) as Promise<AgentDeployment>;
export const setAgentOOMAutoscale = (id: string, enabled: boolean | null) =>
  axiosInstance.put(`/api/admin/agents/${id}/oom/autoscale`, { enabled });
export const invokeAgent = (slug: string, input: unknown) =>
  axiosInstance.post(`/v1/agents/${slug}/invoke`, { input }) as Promise<{ output: unknown; error?: string; task_id?: string }>;

// Use Cases
export type UseCaseListParams = {
  search?: string;
  status?: string;
  risk_tier?: string;
  classification?: string;
  effective_classification?: string;
  compliance_flag?: boolean;
  has_third_party?: boolean;
};
export const listUseCases = (params?: UseCaseListParams) => {
  const qs = new URLSearchParams();
  if (params?.search) qs.set("search", params.search);
  if (params?.status) qs.set("status", params.status);
  if (params?.risk_tier) qs.set("risk_tier", params.risk_tier);
  if (params?.classification) qs.set("classification", params.classification);
  if (params?.effective_classification)
    qs.set("effective_classification", params.effective_classification);
  if (params?.compliance_flag != null)
    qs.set("compliance_flag", String(params.compliance_flag));
  if (params?.has_third_party != null)
    qs.set("has_third_party", String(params.has_third_party));
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return axiosInstance.get(`/api/admin/use-cases${suffix}`) as Promise<{ use_cases: UseCase[] }>;
};
export const getUseCase = (idOrSlug: string) =>
  axiosInstance.get(`/api/admin/use-cases/${encodeURIComponent(idOrSlug)}`) as Promise<UseCaseWithRefs>;
export const createUseCase = (body: UseCaseUpsertRequest) =>
  axiosInstance.post(`/api/admin/use-cases`, body) as Promise<UseCaseWithRefs>;
export const updateUseCase = (id: string, body: UseCaseUpsertRequest) =>
  axiosInstance.put(`/api/admin/use-cases/${id}`, body) as Promise<UseCaseWithRefs>;
export const deleteUseCase = (id: string) =>
  axiosInstance.delete(`/api/admin/use-cases/${id}`);

// Scope-event audit log
export type ScopeEventListParams = {
  use_case_id?: string;
  kind?: "mcp_tool" | "llm" | "";
  outcome?: "denied" | "flagged" | "";
  target_name?: string;
  since?: string;
  limit?: number;
  offset?: number;
};
export const listScopeEvents = (params?: ScopeEventListParams) => {
  const qs = new URLSearchParams();
  if (params?.use_case_id) qs.set("use_case_id", params.use_case_id);
  if (params?.kind) qs.set("kind", params.kind);
  if (params?.outcome) qs.set("outcome", params.outcome);
  if (params?.target_name) qs.set("target_name", params.target_name);
  if (params?.since) qs.set("since", params.since);
  if (params?.limit != null) qs.set("limit", String(params.limit));
  if (params?.offset != null) qs.set("offset", String(params.offset));
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  return axiosInstance.get(`/api/admin/use-cases/events${suffix}`) as Promise<{
    events: ScopeEvent[];
    stats: ScopeEventStats;
  }>;
};

// Webhooks
export const listWebhooks = () =>
  axiosInstance.get(`/api/admin/webhooks`) as Promise<{ webhooks: Webhook[] }>;
export const getWebhook = (id: string) =>
  axiosInstance.get(`/api/admin/webhooks/${id}`) as Promise<Webhook>;
export const createWebhook = (body: WebhookUpsertRequest) =>
  axiosInstance.post(`/api/admin/webhooks`, body) as Promise<Webhook>;
export const updateWebhook = (id: string, body: WebhookUpsertRequest) =>
  axiosInstance.put(`/api/admin/webhooks/${id}`, body) as Promise<Webhook>;
export const deleteWebhook = (id: string) =>
  axiosInstance.delete(`/api/admin/webhooks/${id}`);
export const testWebhook = (id: string) =>
  axiosInstance.post(`/api/admin/webhooks/${id}/test`, {});

// Lifecycle hygiene
// Returns the cached summary of the most recent lifecycle daemon run.
// Empty/zero summary before the daemon has run for the first time.
export const getLifecycleStatus = () =>
  axiosInstance.get(`/api/admin/lifecycle/status`) as Promise<LifecycleSummary>;
// Triggers a synchronous run-now and returns its summary.
export const runLifecycle = () =>
  axiosInstance.post(`/api/admin/lifecycle/run`, {}) as Promise<LifecycleSummary>;
