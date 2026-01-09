import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  GuardrailDiscoveryResponse,
  GuardrailSource,
  MarketplaceGuardrail,
  DiscoverGuardrailRequest,
  AddGuardrailSourceRequest,
  ConfigureGuardrailRequest,
  TestGuardrailRequest,
  TestGuardrailResponse,
} from '@/types/discovery'
import { toast } from '@/components/ui/use-toast'

// Mock API functions - these will be replaced with real API calls
const api = {
  // Discover a guardrail from URL
  async discoverGuardrail(request: DiscoverGuardrailRequest): Promise<GuardrailDiscoveryResponse> {
    // TODO: Replace with real API call
    const response = await fetch('/api/admin/guardrails/discover', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    })
    if (!response.ok) throw new Error('Failed to discover guardrail')
    return response.json()
  },

  // Get all guardrail sources
  async getSources(): Promise<GuardrailSource[]> {
    const response = await fetch('/api/admin/guardrails/sources')
    if (!response.ok) throw new Error('Failed to fetch sources')
    return response.json()
  },

  // Add a new source
  async addSource(request: AddGuardrailSourceRequest): Promise<GuardrailSource> {
    const response = await fetch('/api/admin/guardrails/sources', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    })
    if (!response.ok) throw new Error('Failed to add source')
    return response.json()
  },

  // Remove a source
  async removeSource(sourceId: string): Promise<void> {
    const response = await fetch(`/api/admin/guardrails/sources/${sourceId}`, {
      method: 'DELETE',
    })
    if (!response.ok) throw new Error('Failed to remove source')
  },

  // Get marketplace guardrails (discovered from all sources)
  async getMarketplace(): Promise<MarketplaceGuardrail[]> {
    const response = await fetch('/api/admin/guardrails/marketplace')
    if (!response.ok) throw new Error('Failed to fetch marketplace')
    return response.json()
  },

  // Configure and install a guardrail
  async configureGuardrail(request: ConfigureGuardrailRequest): Promise<void> {
    const response = await fetch('/api/admin/guardrails', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    })
    if (!response.ok) throw new Error('Failed to configure guardrail')
  },

  // Test a guardrail configuration
  async testGuardrail(request: TestGuardrailRequest): Promise<TestGuardrailResponse> {
    const response = await fetch('/api/admin/guardrails/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    })
    if (!response.ok) throw new Error('Failed to test guardrail')
    return response.json()
  },
}

/**
 * Hook for discovering a guardrail from a URL
 */
export function useDiscoverGuardrail() {
  const [isDiscovering, setIsDiscovering] = useState(false)
  const [discovery, setDiscovery] = useState<GuardrailDiscoveryResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const discover = async (url: string, verifySsl = true) => {
    setIsDiscovering(true)
    setError(null)
    setDiscovery(null)

    try {
      const result = await api.discoverGuardrail({
        url,
        verify_ssl: verifySsl,
        timeout_seconds: 10,
      })
      setDiscovery(result)
      return result
    } catch (err: any) {
      const errorMsg = err.message || 'Failed to discover guardrail'
      setError(errorMsg)
      toast({
        title: 'Discovery Failed',
        description: errorMsg,
        variant: 'destructive',
      })
      throw err
    } finally {
      setIsDiscovering(false)
    }
  }

  const reset = () => {
    setDiscovery(null)
    setError(null)
  }

  return {
    discover,
    reset,
    isDiscovering,
    discovery,
    error,
  }
}

/**
 * Hook for managing guardrail sources
 */
export function useGuardrailSources() {
  const queryClient = useQueryClient()

  const { data: sources = [], isLoading } = useQuery({
    queryKey: ['guardrail-sources'],
    queryFn: api.getSources,
  })

  const addSourceMutation = useMutation({
    mutationFn: api.addSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['guardrail-sources'] })
      queryClient.invalidateQueries({ queryKey: ['guardrail-marketplace'] })
      toast({
        title: 'Source Added',
        description: 'Guardrail source has been added successfully',
      })
    },
    onError: (error: any) => {
      toast({
        title: 'Failed to Add Source',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  const removeSourceMutation = useMutation({
    mutationFn: api.removeSource,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['guardrail-sources'] })
      queryClient.invalidateQueries({ queryKey: ['guardrail-marketplace'] })
      toast({
        title: 'Source Removed',
        description: 'Guardrail source has been removed',
      })
    },
    onError: (error: any) => {
      toast({
        title: 'Failed to Remove Source',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  return {
    sources,
    isLoading,
    addSource: addSourceMutation.mutate,
    removeSource: removeSourceMutation.mutate,
    isAdding: addSourceMutation.isPending,
    isRemoving: removeSourceMutation.isPending,
  }
}

/**
 * Hook for marketplace guardrails
 */
export function useMarketplace() {
  const { data: guardrails = [], isLoading } = useQuery({
    queryKey: ['guardrail-marketplace'],
    queryFn: api.getMarketplace,
    refetchInterval: 60000, // Refresh every minute
  })

  const installed = guardrails.filter((g) => g.installed)
  const available = guardrails.filter((g) => !g.installed)

  return {
    guardrails,
    installed,
    available,
    isLoading,
  }
}

/**
 * Hook for configuring a guardrail
 */
export function useConfigureGuardrail() {
  const queryClient = useQueryClient()

  const configureMutation = useMutation({
    mutationFn: api.configureGuardrail,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['guardrails'] })
      queryClient.invalidateQueries({ queryKey: ['guardrail-marketplace'] })
      toast({
        title: 'Guardrail Configured',
        description: 'Guardrail has been configured and installed successfully',
      })
    },
    onError: (error: any) => {
      toast({
        title: 'Configuration Failed',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  return {
    configure: configureMutation.mutate,
    isConfiguring: configureMutation.isPending,
    error: configureMutation.error,
  }
}

/**
 * Hook for testing a guardrail configuration
 */
export function useTestGuardrail() {
  const [testResult, setTestResult] = useState<TestGuardrailResponse | null>(null)

  const testMutation = useMutation({
    mutationFn: api.testGuardrail,
    onSuccess: (result) => {
      setTestResult(result)
      if (result.success) {
        toast({
          title: 'Test Passed',
          description: `Guardrail executed in ${result.latency_ms}ms`,
        })
      } else {
        toast({
          title: 'Test Failed',
          description: result.error || 'Unknown error',
          variant: 'destructive',
        })
      }
    },
    onError: (error: any) => {
      toast({
        title: 'Test Error',
        description: error.message,
        variant: 'destructive',
      })
    },
  })

  const reset = () => {
    setTestResult(null)
  }

  return {
    test: testMutation.mutate,
    isTesting: testMutation.isPending,
    testResult,
    reset,
  }
}
