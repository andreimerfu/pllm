import axios from 'axios'

const API_BASE = import.meta.env.DEV ? 'http://localhost:8080' : ''

export const api = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
})

// API endpoints
export const healthCheck = () => api.get('/health')
export const getDatabaseStatus = () => api.get('/v1/admin/database/status')
export const getModels = () => api.get('/v1/models')
export const getModelStats = () => api.get('/v1/admin/models/stats')
export const getMetrics = () => api.get('/metrics')

// User management
export const getUsers = () => api.get('/v1/admin/users')
export const createUser = (data: any) => api.post('/v1/admin/users', data)
export const updateUser = (id: string, data: any) => api.put(`/v1/admin/users/${id}`, data)
export const deleteUser = (id: string) => api.delete(`/v1/admin/users/${id}`)

// API Keys
export const getAPIKeys = () => api.get('/v1/admin/api-keys')
export const createAPIKey = (data: any) => api.post('/v1/admin/api-keys', data)
export const revokeAPIKey = (id: string) => api.delete(`/v1/admin/api-keys/${id}`)

// Usage tracking
export const getUsageStats = (params?: any) => api.get('/v1/admin/usage', { params })
export const getUserUsage = (userId: string) => api.get(`/v1/admin/users/${userId}/usage`)

// Providers
export const getProviders = () => api.get('/v1/admin/providers')
export const updateProvider = (id: string, data: any) => api.put(`/v1/admin/providers/${id}`, data)

// Groups
export const getGroups = () => api.get('/v1/admin/groups')
export const createGroup = (data: any) => api.post('/v1/admin/groups', data)
export const updateGroup = (id: string, data: any) => api.put(`/v1/admin/groups/${id}`, data)
export const deleteGroup = (id: string) => api.delete(`/v1/admin/groups/${id}`)

// Budgets
export const getBudgets = () => api.get('/v1/admin/budgets')
export const createBudget = (data: any) => api.post('/v1/admin/budgets', data)
export const updateBudget = (id: string, data: any) => api.put(`/v1/admin/budgets/${id}`, data)
export const deleteBudget = (id: string) => api.delete(`/v1/admin/budgets/${id}`)