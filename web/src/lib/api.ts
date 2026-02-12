import axios from "axios";
import type { StatsResponse, ModelsResponse, CreateModelRequest, UpdateModelRequest, AdminModelsResponse, ProviderConfig, ModelsHealthResponse, RoutesResponse, Route, RouteStatsResponse } from "@/types/api";

const API_BASE = import.meta.env.DEV ? "http://localhost:8080" : "";

const axiosInstance = axios.create({
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
