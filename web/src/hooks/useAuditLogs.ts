import { useState, useCallback, useEffect } from 'react'
import { subDays, startOfDay, endOfDay } from 'date-fns'
import { DateRange } from 'react-day-picker'
import { useToast } from '@/hooks/use-toast'
import { getAuditLogs } from '@/lib/api'
import { AuditLog, AuditLogsResponse } from '@/types/api'

export interface AuditLogFilters {
  action: string
  resource: string
  user_id: string
  result: string
}

export function useAuditLogs() {
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [currentPage, setCurrentPage] = useState(0)
  const [pageSize] = useState(50)

  const [filters, setFilters] = useState<AuditLogFilters>({
    action: '',
    resource: '',
    user_id: '',
    result: '',
  })

  const [dateRange, setDateRange] = useState<DateRange | undefined>({
    from: subDays(new Date(), 30),
    to: new Date(),
  })

  const { toast } = useToast()

  const fetchAuditLogs = useCallback(async () => {
    try {
      setIsLoading(true)

      const apiFilters = {
        ...filters,
        start_date: dateRange?.from ? startOfDay(dateRange.from).toISOString() : undefined,
        end_date: dateRange?.to ? endOfDay(dateRange.to).toISOString() : undefined,
        limit: pageSize,
        offset: currentPage * pageSize,
      }

      // Remove empty filters
      Object.keys(apiFilters).forEach(key => {
        if (apiFilters[key as keyof typeof apiFilters] === '' ||
            apiFilters[key as keyof typeof apiFilters] === undefined) {
          delete apiFilters[key as keyof typeof apiFilters]
        }
      })

      const response = await getAuditLogs(apiFilters) as unknown as AuditLogsResponse
      setAuditLogs(response.audit_logs)
      setTotal(response.total)
    } catch (error) {
      console.error('Failed to fetch audit logs:', error)
      toast({
        title: 'Error',
        description: 'Failed to load audit logs',
        variant: 'destructive',
      })
    } finally {
      setIsLoading(false)
    }
  }, [filters, dateRange, currentPage, pageSize, toast])

  useEffect(() => {
    fetchAuditLogs()
  }, [fetchAuditLogs])

  const clearFilters = useCallback(() => {
    setFilters({
      action: '',
      resource: '',
      user_id: '',
      result: '',
    })
    setDateRange({
      from: subDays(new Date(), 30),
      to: new Date(),
    })
    setCurrentPage(0)
  }, [])

  const handlePageChange = useCallback((newPage: number) => {
    setCurrentPage(newPage)
  }, [])

  return {
    auditLogs,
    isLoading,
    total,
    currentPage,
    pageSize,
    filters,
    dateRange,
    setFilters,
    setDateRange,
    clearFilters,
    handlePageChange,
    refetch: fetchAuditLogs,
    pageCount: Math.ceil(total / pageSize),
  }
}
