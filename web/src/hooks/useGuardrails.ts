import { useQuery } from '@tanstack/react-query'
import { getGuardrails, getGuardrailStats, checkGuardrailHealth } from '@/lib/api'

export interface GuardrailInfo {
  name: string
  provider: string
  mode: string[]
  enabled: boolean
  default_on: boolean
  config: Record<string, any>
  healthy?: boolean
  stats?: {
    total_executions: number
    total_passed: number
    total_blocked: number
    total_errors: number
    blocked_count: number
    passed_count: number
    error_count: number
    average_latency: number
    avg_latency_ms: number
    last_executed: string
    block_rate: number
    error_rate: number
  }
}

export interface GuardrailStats {
  total_executions: number
  blocked_requests: number
  avg_latency_ms: number
  block_rate: number
}

export interface GuardrailHealth {
  [key: string]: {
    healthy: boolean
    last_check: string
    error?: string
  }
}

export function useGuardrails() {
  const { data: guardrailsData, isLoading, refetch } = useQuery({
    queryKey: ['guardrails'],
    queryFn: getGuardrails,
  })

  const { data: statsData, isLoading: isStatsLoading } = useQuery({
    queryKey: ['guardrails-stats'],
    queryFn: getGuardrailStats,
  })

  const { data: healthData, isLoading: isHealthLoading } = useQuery({
    queryKey: ['guardrails-health'],
    queryFn: checkGuardrailHealth,
    refetchInterval: 30000, // Check health every 30 seconds
  })

  const guardrails = (guardrailsData as any)?.guardrails || []
  const systemEnabled = (guardrailsData as any)?.enabled ?? false
  const stats = (statsData as any)?.stats || {}
  const health = (healthData as any)?.health || {}
  const allHealthy = (healthData as any)?.all_healthy ?? false
  const checkedAt = (healthData as any)?.checked_at

  return {
    guardrails: guardrails as GuardrailInfo[],
    systemEnabled,
    stats: stats as GuardrailStats,
    health: health as GuardrailHealth,
    allHealthy,
    checkedAt,
    isLoading: isLoading || isStatsLoading || isHealthLoading,
    refetch,
  }
}

// Utility functions for Guardrails UI
export const formatLatency = (ms: number) => {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

export const formatRate = (rate: number) => {
  return `${(rate * 100).toFixed(1)}%`
}

export const getModeColor = (mode: string) => {
  switch (mode) {
    case 'pre_call':
      return 'bg-blue-100 text-blue-800'
    case 'post_call':
      return 'bg-green-100 text-green-800'
    case 'during_call':
      return 'bg-yellow-100 text-yellow-800'
    case 'logging_only':
      return 'bg-gray-100 text-gray-800'
    default:
      return 'bg-gray-100 text-gray-800'
  }
}
