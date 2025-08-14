import axios from "axios";
import type { StatsResponse, ModelsResponse } from "@/types/api";

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
      window.location.href = "/login";
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

// Virtual Keys API
const keys = {
  list: () => axiosInstance.get("/api/admin/keys"),
  generate: (data: any) => axiosInstance.post("/api/admin/keys/generate", data),
  get: (id: string) => axiosInstance.get(`/api/admin/keys/${id}`),
  update: (id: string, data: any) =>
    axiosInstance.put(`/api/admin/keys/${id}`, data),
  delete: (id: string) => axiosInstance.delete(`/api/admin/keys/${id}`),
  revoke: (id: string, data?: any) =>
    axiosInstance.post(`/api/admin/keys/${id}/revoke`, data || {}),
  getStats: (id: string) => axiosInstance.get(`/api/admin/keys/${id}/stats`),
  validate: (key: string) =>
    axiosInstance.post("/api/admin/keys/validate", { key }),
  temporaryBudgetIncrease: (id: string, data: any) =>
    axiosInstance.post(`/api/admin/keys/${id}/budget-increase`, data),
};

// Export the main API object
const api = {
  users,
  teams,
  keys,
  // Legacy exports for backward compatibility
  axios: axiosInstance,
};

export default api;

// Legacy exports for backward compatibility
export const healthCheck = () => axiosInstance.get("/health");
export const getDatabaseStatus = () =>
  axiosInstance.get("/api/admin/system/health");
export const getModels = () =>
  axiosInstance.get("/v1/models") as Promise<ModelsResponse>;
export const getModelStats = () =>
  axiosInstance.get("/api/admin/stats") as Promise<StatsResponse>;
export const getMetrics = () => axiosInstance.get("/metrics");

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
  axiosInstance.post("/api/admin/keys/generate", data);
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

// Budgets
export const getBudgets = () => axiosInstance.get("/api/admin/budgets");
export const createBudget = (data: any) =>
  axiosInstance.post("/api/admin/budgets", data);
export const getBudget = (id: string) =>
  axiosInstance.get(`/api/admin/budgets/${id}`);
export const updateBudget = (id: string, data: any) =>
  axiosInstance.put(`/api/admin/budgets/${id}`, data);
export const deleteBudget = (id: string) =>
  axiosInstance.delete(`/api/admin/budgets/${id}`);
export const resetBudget = (id: string) =>
  axiosInstance.post(`/api/admin/budgets/${id}/reset`);
export const getBudgetAlerts = () =>
  axiosInstance.get("/api/admin/budgets/alerts");

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
export const getDashboard = () => axiosInstance.get("/api/admin/dashboard");

// System
export const getConfig = () => axiosInstance.get("/api/admin/system/config");
export const updateConfig = (data: any) =>
  axiosInstance.put("/api/admin/system/config", data);
export const getSystemHealth = () =>
  axiosInstance.get("/api/admin/system/health");
export const getLogs = () => axiosInstance.get("/api/admin/system/logs");
export const getAuditLogs = () => axiosInstance.get("/api/admin/system/audit");
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
