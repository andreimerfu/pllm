"use client"

import * as React from "react"
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { format, formatDistanceToNow } from "date-fns"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Calendar as CalendarComponent } from "@/components/ui/calendar"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { AuditLog } from "@/types/api"
import { useAuditLogs } from "@/hooks/useAuditLogs"
import { getStatusBadge } from "@/components/audit-logs/columns"
import { DetailItem } from "@/components/common/DetailItem"
import { JsonViewer } from "@/components/common/JsonViewer"

// Status dot color
const getResultDot = (result: string) => {
  switch (result) {
    case 'success':
      return 'bg-emerald-400'
    case 'failure':
      return 'bg-red-400'
    case 'error':
      return 'bg-red-500'
    case 'warning':
      return 'bg-amber-400'
    default:
      return 'bg-zinc-400'
  }
}

// Pill status options
const statusOptions = [
  { value: '', label: 'All', dot: 'bg-zinc-400' },
  { value: 'success', label: 'Success', dot: 'bg-emerald-400' },
  { value: 'failure', label: 'Failure', dot: 'bg-red-400' },
  { value: 'error', label: 'Error', dot: 'bg-red-500' },
  { value: 'warning', label: 'Warning', dot: 'bg-amber-400' },
]

export default function AuditLogs() {
  const {
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
    refetch,
    // pageCount is available but unused in the timeline layout
  } = useAuditLogs()

  // Drawer state
  const [selectedAuditLog, setSelectedAuditLog] = React.useState<AuditLog | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = React.useState(false)

  // Expanded rows (inline detail)
  const [expandedRows, setExpandedRows] = React.useState<Set<string>>(new Set())

  // Search input
  const [searchInput, setSearchInput] = React.useState("")

  // Determine if any filters are active
  const hasActiveFilters = React.useMemo(() => {
    return !!(filters.action || filters.resource || filters.result || filters.user_id || searchInput)
  }, [filters, searchInput])

  // Debounced search
  React.useEffect(() => {
    const timeout = setTimeout(() => {
      setFilters(prev => ({ ...prev, action: searchInput }))
    }, 300)
    return () => clearTimeout(timeout)
  }, [searchInput, setFilters])

  // Collect active filter pills
  const activeFilterPills = React.useMemo(() => {
    const pills: { key: string; label: string; onClear: () => void }[] = []
    if (filters.resource) {
      pills.push({
        key: 'resource',
        label: `Resource: ${filters.resource}`,
        onClear: () => setFilters(prev => ({ ...prev, resource: '' })),
      })
    }
    if (filters.result) {
      pills.push({
        key: 'result',
        label: `Result: ${filters.result}`,
        onClear: () => setFilters(prev => ({ ...prev, result: '' })),
      })
    }
    if (filters.user_id) {
      pills.push({
        key: 'user_id',
        label: `User: ${filters.user_id}`,
        onClear: () => setFilters(prev => ({ ...prev, user_id: '' })),
      })
    }
    return pills
  }, [filters, setFilters])

  const handleClearAll = () => {
    setSearchInput("")
    clearFilters()
  }

  const handleExportAll = () => {
    const blob = new Blob([JSON.stringify(auditLogs, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `audit-logs-${format(new Date(), 'yyyy-MM-dd')}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  const toggleRow = (id: string) => {
    setExpandedRows(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  return (
    <div className="space-y-0">
      {/* ── Header ── */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Audit Logs</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              System events and security activity
            </p>
          </div>
          {!isLoading && (
            <Badge variant="secondary" className="font-mono text-xs tabular-nums h-6 px-2.5">
              {total.toLocaleString()}
            </Badge>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={handleExportAll} disabled={auditLogs.length === 0}>
            <Icon icon={icons.download} className="h-3.5 w-3.5 mr-1.5" />
            Export
          </Button>
          <Button size="sm" onClick={refetch} disabled={isLoading}>
            <Icon icon={icons.refresh} className={`h-3.5 w-3.5 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* ── Filter Bar ── */}
      <div className="rounded-xl border bg-card/60 backdrop-blur-sm px-3 py-2.5 mb-4">
        <div className="flex items-center gap-2 flex-wrap">
          {/* Search */}
          <div className="relative flex-1 min-w-[220px] max-w-md">
            <Icon icon={icons.search} className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground pointer-events-none" />
            <Input
              placeholder="Search actions, events..."
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="h-8 pl-8 text-sm bg-background/60 border-muted"
            />
          </div>

          {/* Separator */}
          <div className="h-5 w-px bg-border hidden sm:block" />

          {/* Date Range */}
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" size="sm" className="h-8 text-xs font-normal gap-1.5 bg-background/60 border-muted">
                <Icon icon={icons.calendar} className="h-3.5 w-3.5 text-muted-foreground" />
                {dateRange?.from ? (
                  dateRange.to ? (
                    <>
                      {format(dateRange.from, "MMM d")} - {format(dateRange.to, "MMM d")}
                    </>
                  ) : (
                    format(dateRange.from, "MMM d, yyyy")
                  )
                ) : (
                  "Date range"
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0" align="start">
              <CalendarComponent
                initialFocus
                mode="range"
                defaultMonth={dateRange?.from}
                selected={dateRange}
                onSelect={setDateRange}
                numberOfMonths={2}
              />
            </PopoverContent>
          </Popover>

          {/* Resource Type */}
          <Select
            value={filters.resource || "all-resources"}
            onValueChange={(value) => setFilters(prev => ({ ...prev, resource: value === "all-resources" ? "" : value }))}
          >
            <SelectTrigger className="h-8 w-[130px] text-xs bg-background/60 border-muted">
              <SelectValue placeholder="Resource" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all-resources">All resources</SelectItem>
              <SelectItem value="user">User</SelectItem>
              <SelectItem value="team">Team</SelectItem>
              <SelectItem value="key">Key</SelectItem>
              <SelectItem value="api">API</SelectItem>
              <SelectItem value="llm">LLM</SelectItem>
              <SelectItem value="session">Session</SelectItem>
              <SelectItem value="budget">Budget</SelectItem>
            </SelectContent>
          </Select>

          {/* Separator */}
          <div className="h-5 w-px bg-border hidden sm:block" />

          {/* Status Pill Toggles */}
          <div className="flex items-center gap-1 rounded-lg bg-muted/50 p-0.5">
            {statusOptions.map((opt) => {
              const isActive = filters.result === opt.value
              return (
                <button
                  key={opt.value}
                  onClick={() => setFilters(prev => ({ ...prev, result: opt.value }))}
                  className={`
                    flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium transition-all
                    ${isActive
                      ? 'bg-background shadow-sm text-foreground'
                      : 'text-muted-foreground hover:text-foreground'
                    }
                  `}
                >
                  <span className={`h-1.5 w-1.5 rounded-full ${opt.dot}`} />
                  {opt.label}
                </button>
              )
            })}
          </div>

          {/* User ID Filter */}
          <Input
            placeholder="User ID..."
            value={filters.user_id}
            onChange={(e) => setFilters(prev => ({ ...prev, user_id: e.target.value }))}
            className="h-8 w-[130px] text-xs bg-background/60 border-muted"
          />

          {/* Clear Filters */}
          {hasActiveFilters && (
            <Button variant="ghost" size="sm" className="h-8 text-xs text-muted-foreground hover:text-foreground ml-auto" onClick={handleClearAll}>
              <Icon icon={icons.close} className="h-3.5 w-3.5 mr-1" />
              Clear
            </Button>
          )}
        </div>
      </div>

      {/* ── Active Filter Chips ── */}
      {activeFilterPills.length > 0 && (
        <div className="flex items-center gap-1.5 flex-wrap mb-4">
          {activeFilterPills.map((pill) => (
            <Badge
              key={pill.key}
              variant="secondary"
              className="text-xs gap-1 pl-2.5 pr-1.5 py-0.5 cursor-pointer hover:bg-destructive/10 hover:text-destructive transition-colors"
              onClick={pill.onClear}
            >
              {pill.label}
              <Icon icon={icons.close} className="h-3 w-3" />
            </Badge>
          ))}
        </div>
      )}

      {/* ── Timeline Feed ── */}
      <div className="rounded-xl border bg-card overflow-hidden">
        {/* Column header */}
        <div className="grid grid-cols-[auto_1fr_minmax(0,200px)_minmax(0,140px)_minmax(0,120px)_40px] gap-3 px-4 py-2.5 border-b bg-muted/30 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          <div className="w-14">Time</div>
          <div>Event</div>
          <div className="hidden lg:block">User</div>
          <div className="hidden md:block">Resource</div>
          <div className="hidden sm:block">IP</div>
          <div />
        </div>

        {/* Rows */}
        <div className="divide-y divide-border/50">
          {isLoading ? (
            Array(Math.min(pageSize, 10)).fill(0).map((_, i) => (
              <div key={i} className="px-4 py-3">
                <div className="flex items-center gap-3">
                  <Skeleton className="h-2 w-2 rounded-full" />
                  <Skeleton className="h-4 w-16" />
                  <Skeleton className="h-4 w-48" />
                  <Skeleton className="h-4 w-24 ml-auto" />
                </div>
              </div>
            ))
          ) : auditLogs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
              <Icon icon={icons.auditLogs} className="h-10 w-10 opacity-30 mb-3" />
              <span className="text-sm font-medium">No audit logs found</span>
              <span className="text-xs mt-1">Try adjusting your filters or date range</span>
            </div>
          ) : (
            auditLogs.map((log, index) => {
              const isExpanded = expandedRows.has(log.id)
              const timestamp = new Date(log.timestamp)
              const relative = formatDistanceToNow(timestamp, { addSuffix: true })
              const initial = log.user?.name?.[0] || log.user?.email?.[0]?.toUpperCase() || null

              return (
                <div key={log.id}>
                  {/* Row */}
                  <div
                    className={`
                      grid grid-cols-[auto_1fr_minmax(0,200px)_minmax(0,140px)_minmax(0,120px)_40px] gap-3 items-center px-4 py-2.5
                      cursor-pointer transition-colors group
                      ${index % 2 === 0 ? 'bg-transparent' : 'bg-muted/20'}
                      ${isExpanded ? 'bg-accent/40' : 'hover:bg-accent/30'}
                    `}
                    onClick={() => toggleRow(log.id)}
                  >
                    {/* Time */}
                    <div className="w-14 font-mono text-[11px] text-muted-foreground whitespace-nowrap" title={format(timestamp, "yyyy-MM-dd HH:mm:ss")}>
                      {relative.replace(' ago', '').replace('about ', '~').replace('less than a minute', '<1m')}
                    </div>

                    {/* Event */}
                    <div className="flex items-center gap-2.5 min-w-0">
                      <span className={`h-2 w-2 rounded-full shrink-0 ${getResultDot(log.event_result)}`} />
                      <div className="min-w-0">
                        <span className="text-sm font-semibold truncate block">{log.event_action}</span>
                        <span className="text-[11px] text-muted-foreground capitalize truncate block">
                          {log.event_type.replace(/_/g, ' ')}
                        </span>
                      </div>
                      {log.resource_type && (
                        <Badge variant="outline" className="text-[10px] px-1.5 py-0 h-4 capitalize font-normal shrink-0 hidden xl:inline-flex border-muted-foreground/20 text-muted-foreground">
                          {log.resource_type}
                        </Badge>
                      )}
                    </div>

                    {/* User */}
                    <div className="hidden lg:flex items-center gap-2 min-w-0">
                      {log.user ? (
                        <>
                          <div className="h-6 w-6 rounded-full bg-primary/10 flex items-center justify-center shrink-0 ring-1 ring-primary/20">
                            <span className="text-[10px] font-semibold text-primary">{initial}</span>
                          </div>
                          <span className="text-xs truncate">{log.user.name || log.user.email || 'Unknown'}</span>
                        </>
                      ) : (
                        <span className="text-[11px] text-muted-foreground italic">System</span>
                      )}
                    </div>

                    {/* Resource */}
                    <div className="hidden md:block min-w-0">
                      {log.resource_type ? (
                        <div>
                          <span className="text-xs capitalize">{log.resource_type}</span>
                          {log.resource_id && (
                            <div className="text-[10px] font-mono text-muted-foreground truncate">
                              {log.resource_id.slice(0, 12)}
                            </div>
                          )}
                        </div>
                      ) : (
                        <span className="text-xs text-muted-foreground">--</span>
                      )}
                    </div>

                    {/* IP */}
                    <div className="hidden sm:block font-mono text-[11px] text-muted-foreground truncate">
                      {log.ip_address || "--"}
                    </div>

                    {/* Expand chevron */}
                    <div className="flex justify-end">
                      <Icon
                        icon={icons.chevronDown}
                        className={`h-4 w-4 text-muted-foreground transition-transform duration-200 ${isExpanded ? 'rotate-180' : ''}`}
                      />
                    </div>
                  </div>

                  {/* Expanded inline detail */}
                  {isExpanded && (
                    <div className={`border-t bg-muted/10 ${index % 2 === 0 ? '' : 'bg-muted/30'}`}>
                      <div className="px-4 py-4">
                        {/* Quick summary bar */}
                        <div className="flex items-center gap-2 mb-4 flex-wrap">
                          {getStatusBadge(log.event_result)}
                          <Badge variant="outline" className="text-xs capitalize font-normal">
                            {log.event_type.replace(/_/g, ' ')}
                          </Badge>
                          {log.resource_type && (
                            <Badge variant="outline" className="text-xs capitalize font-normal">
                              {log.resource_type}
                            </Badge>
                          )}
                          <span className="text-[11px] font-mono text-muted-foreground ml-auto">
                            {format(timestamp, "yyyy-MM-dd HH:mm:ss")}
                          </span>
                          <div className="flex items-center gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-6 px-2 text-[11px] text-muted-foreground"
                              onClick={(e: React.MouseEvent) => {
                                e.stopPropagation()
                                setSelectedAuditLog(log)
                                setIsDrawerOpen(true)
                              }}
                            >
                              <Icon icon={icons.maximize} className="h-3 w-3 mr-1" />
                              Full view
                            </Button>
                          </div>
                        </div>

                        <Tabs defaultValue="overview">
                          <TabsList className="h-7 p-0.5 bg-muted/60">
                            <TabsTrigger value="overview" className="text-[11px] h-6 px-3">Overview</TabsTrigger>
                            <TabsTrigger value="request" className="text-[11px] h-6 px-3">Request</TabsTrigger>
                            <TabsTrigger value="changes" className="text-[11px] h-6 px-3">Changes</TabsTrigger>
                            <TabsTrigger value="technical" className="text-[11px] h-6 px-3">Technical</TabsTrigger>
                          </TabsList>

                          <div className="mt-3">
                            <TabsContent value="overview" className="mt-0">
                              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                                <DetailItem label="User">
                                  {log.user ? (
                                    <div className="flex items-center gap-2">
                                      <div className="h-5 w-5 rounded-full bg-primary/10 flex items-center justify-center ring-1 ring-primary/20">
                                        <span className="text-[9px] font-semibold text-primary">{initial}</span>
                                      </div>
                                      <span className="text-xs">{log.user.name || log.user.email}</span>
                                    </div>
                                  ) : (
                                    <span className="text-xs text-muted-foreground italic">System</span>
                                  )}
                                </DetailItem>
                                {log.resource_type && (
                                  <DetailItem label="Resource" value={`${log.resource_type}${log.resource_id ? ' / ' + log.resource_id.slice(0, 8) + '...' : ''}`} />
                                )}
                                {log.message && (
                                  <div className="col-span-2">
                                    <DetailItem label="Message">
                                      <div className="bg-muted/50 rounded px-2 py-1.5 text-xs font-mono leading-relaxed">{log.message}</div>
                                    </DetailItem>
                                  </div>
                                )}
                                {log.error_code && (
                                  <DetailItem label="Error Code">
                                    <Badge variant="destructive" className="text-[10px] font-mono">{log.error_code}</Badge>
                                  </DetailItem>
                                )}
                              </div>
                            </TabsContent>

                            <TabsContent value="request" className="mt-0">
                              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                                {log.method && log.path && (
                                  <div className="col-span-2">
                                    <DetailItem label="Endpoint">
                                      <div className="flex items-center gap-1.5">
                                        <Badge variant="outline" className={`text-[10px] px-1.5 py-0 ${
                                          log.method === 'GET' ? "bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-300" :
                                          log.method === 'POST' ? "bg-green-50 text-green-700 border-green-200 dark:bg-green-950 dark:text-green-300" :
                                          log.method === 'PUT' ? "bg-yellow-50 text-yellow-700 border-yellow-200 dark:bg-yellow-950 dark:text-yellow-300" :
                                          log.method === 'DELETE' ? "bg-red-50 text-red-700 border-red-200 dark:bg-red-950 dark:text-red-300" : ""
                                        }`}>
                                          {log.method}
                                        </Badge>
                                        <code className="text-[11px] font-mono bg-muted px-1.5 py-0.5 rounded break-all">{log.path}</code>
                                      </div>
                                    </DetailItem>
                                  </div>
                                )}
                                <DetailItem label="IP Address">
                                  <span className="font-mono text-xs">{log.ip_address || 'N/A'}</span>
                                </DetailItem>
                                {log.duration !== undefined && (
                                  <DetailItem label="Duration">
                                    <span className="font-mono text-xs">{log.duration}ms</span>
                                  </DetailItem>
                                )}
                                {log.user_agent && log.user_agent !== 'Not recorded' && (
                                  <div className="col-span-full">
                                    <DetailItem label="User Agent">
                                      <p className="font-mono text-[11px] break-all bg-muted/50 rounded px-2 py-1.5">{log.user_agent}</p>
                                    </DetailItem>
                                  </div>
                                )}
                              </div>
                            </TabsContent>

                            <TabsContent value="changes" className="mt-0">
                              {(!log.old_values && !log.new_values) ? (
                                <div className="flex items-center justify-center py-6 text-muted-foreground">
                                  <Icon icon={icons.file} className="h-4 w-4 mr-2 opacity-40" />
                                  <span className="text-xs">No changes recorded</span>
                                </div>
                              ) : (
                                <div className="grid md:grid-cols-2 gap-3">
                                  {log.old_values && (
                                    <JsonViewer
                                      title="Old Values"
                                      data={typeof log.old_values === 'string' ? JSON.parse(log.old_values) : log.old_values}
                                    />
                                  )}
                                  {log.new_values && (
                                    <JsonViewer
                                      title="New Values"
                                      data={typeof log.new_values === 'string' ? JSON.parse(log.new_values) : log.new_values}
                                    />
                                  )}
                                </div>
                              )}
                            </TabsContent>

                            <TabsContent value="technical" className="mt-0">
                              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                                <DetailItem label="Event ID">
                                  <code className="text-[11px] font-mono bg-muted px-1.5 py-0.5 rounded break-all">{log.id}</code>
                                </DetailItem>
                                {log.auth_method && (
                                  <DetailItem label="Auth Method">
                                    <Badge variant="secondary" className="capitalize text-[10px]">{log.auth_method}</Badge>
                                  </DetailItem>
                                )}
                                {log.auth_provider && (
                                  <DetailItem label="Auth Provider">
                                    <Badge variant="secondary" className="capitalize text-[10px]">{log.auth_provider}</Badge>
                                  </DetailItem>
                                )}
                                {log.user_id && (
                                  <DetailItem label="User ID">
                                    <code className="text-[11px] font-mono bg-muted px-1.5 py-0.5 rounded">{log.user_id}</code>
                                  </DetailItem>
                                )}
                              </div>
                              {log.metadata && (
                                <div className="mt-3">
                                  <JsonViewer
                                    title="Metadata"
                                    data={typeof log.metadata === 'string' ? JSON.parse(log.metadata) : log.metadata}
                                  />
                                </div>
                              )}
                            </TabsContent>
                          </div>
                        </Tabs>
                      </div>
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* ── Pagination ── */}
      {total > pageSize && (
        <div className="flex items-center justify-between pt-4">
          <div className="text-xs text-muted-foreground font-mono tabular-nums">
            {currentPage * pageSize + 1}-{Math.min((currentPage + 1) * pageSize, total)} of {total}
          </div>
          <div className="flex items-center gap-1.5">
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={() => handlePageChange(Math.max(0, currentPage - 1))}
              disabled={currentPage === 0 || isLoading}
            >
              <Icon icon={icons.chevronLeft} className="h-3.5 w-3.5 mr-1" />
              Prev
            </Button>
            <span className="text-xs text-muted-foreground px-2 font-mono tabular-nums">
              {currentPage + 1} / {Math.ceil(total / pageSize)}
            </span>
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage >= Math.ceil(total / pageSize) - 1 || isLoading}
            >
              Next
              <Icon icon={icons.chevronRight} className="h-3.5 w-3.5 ml-1" />
            </Button>
          </div>
        </div>
      )}

      {/* ── Slide-over Detail Panel ── */}
      <Sheet open={isDrawerOpen} onOpenChange={setIsDrawerOpen}>
        <SheetContent className="w-full sm:max-w-2xl p-0 flex flex-col gap-0">
          {selectedAuditLog && (() => {
            const ts = new Date(selectedAuditLog.timestamp)
            const userInitial = selectedAuditLog.user?.name?.[0] || selectedAuditLog.user?.email?.[0]?.toUpperCase() || 'S'
            return (
              <>
                {/* Dark header */}
                <SheetHeader className="p-5 pr-12 bg-zinc-900 dark:bg-zinc-950 text-white">
                  <div className="flex items-start gap-3">
                    <span className={`inline-block h-3 w-3 rounded-full mt-1 shrink-0 ring-2 ring-white/20 ${getResultDot(selectedAuditLog.event_result)}`} />
                    <div className="flex-1 min-w-0">
                      <SheetTitle className="text-base font-semibold text-white">
                        {selectedAuditLog.event_action}
                      </SheetTitle>
                      <SheetDescription className="mt-1 text-xs font-mono text-zinc-400">
                        {format(ts, "yyyy-MM-dd HH:mm:ss")}
                        <span className="ml-2 text-zinc-500">
                          ({formatDistanceToNow(ts, { addSuffix: true })})
                        </span>
                      </SheetDescription>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 mt-3 flex-wrap">
                    {getStatusBadge(selectedAuditLog.event_result)}
                    <Badge variant="outline" className="text-xs capitalize font-normal border-zinc-700 text-zinc-300">
                      {selectedAuditLog.event_type.replace(/_/g, ' ')}
                    </Badge>
                    {selectedAuditLog.resource_type && (
                      <Badge variant="outline" className="text-xs capitalize font-normal border-zinc-700 text-zinc-300">
                        {selectedAuditLog.resource_type}
                      </Badge>
                    )}
                    <div className="flex items-center gap-1.5 ml-auto">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-6 text-[11px] bg-transparent border-zinc-700 text-zinc-300 hover:bg-zinc-800 hover:text-white"
                        onClick={() => navigator.clipboard.writeText(selectedAuditLog.id)}
                      >
                        <Icon icon={icons.copy} className="h-3 w-3 mr-1" />
                        ID
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-6 text-[11px] bg-transparent border-zinc-700 text-zinc-300 hover:bg-zinc-800 hover:text-white"
                        onClick={() => {
                          const blob = new Blob([JSON.stringify(selectedAuditLog, null, 2)], { type: 'application/json' })
                          const url = URL.createObjectURL(blob)
                          const a = document.createElement('a')
                          a.href = url
                          a.download = `audit-${selectedAuditLog.id}.json`
                          a.click()
                          URL.revokeObjectURL(url)
                        }}
                      >
                        <Icon icon={icons.download} className="h-3 w-3 mr-1" />
                        JSON
                      </Button>
                    </div>
                  </div>
                </SheetHeader>

                <div className="flex-grow overflow-y-auto">
                  <Tabs defaultValue="overview" className="p-4">
                    <TabsList className="grid w-full grid-cols-4 h-8">
                      <TabsTrigger value="overview" className="text-xs">Overview</TabsTrigger>
                      <TabsTrigger value="request" className="text-xs">Request</TabsTrigger>
                      <TabsTrigger value="changes" className="text-xs">Changes</TabsTrigger>
                      <TabsTrigger value="technical" className="text-xs">Technical</TabsTrigger>
                    </TabsList>

                    <div className="mt-4 space-y-6">
                      <TabsContent value="overview">
                        <dl className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-5">
                          <DetailItem label="User">
                            {selectedAuditLog.user ? (
                              <div className="flex items-center gap-2.5">
                                <div className="h-7 w-7 rounded-full bg-primary/15 flex items-center justify-center">
                                  <span className="text-[10px] font-semibold text-primary">{userInitial}</span>
                                </div>
                                <div>
                                  <div className="text-sm font-medium">{selectedAuditLog.user.name || selectedAuditLog.user.email}</div>
                                  {selectedAuditLog.user.name && selectedAuditLog.user.email && (
                                    <div className="text-[11px] text-muted-foreground font-mono">{selectedAuditLog.user.email}</div>
                                  )}
                                </div>
                              </div>
                            ) : (
                              <div className="flex items-center gap-2.5">
                                <div className="h-7 w-7 rounded-full bg-muted flex items-center justify-center">
                                  <span className="text-[10px] font-medium text-muted-foreground">SYS</span>
                                </div>
                                <span className="text-sm text-muted-foreground">System Event</span>
                              </div>
                            )}
                          </DetailItem>

                          {selectedAuditLog.resource_type && (
                            <>
                              <DetailItem label="Resource Type" value={selectedAuditLog.resource_type} />
                              {selectedAuditLog.resource_id && (
                                <DetailItem label="Resource ID">
                                  <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded">{selectedAuditLog.resource_id}</code>
                                </DetailItem>
                              )}
                            </>
                          )}

                          {selectedAuditLog.message && (
                            <DetailItem label="Message">
                              <div className="bg-muted/50 rounded-md p-2.5 text-xs font-mono leading-relaxed">{selectedAuditLog.message}</div>
                            </DetailItem>
                          )}

                          {selectedAuditLog.error_code && (
                            <DetailItem label="Error">
                              <Badge variant="destructive" className="font-mono text-xs">{selectedAuditLog.error_code}</Badge>
                            </DetailItem>
                          )}
                        </dl>
                      </TabsContent>

                      <TabsContent value="request">
                        <dl className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-5">
                          {selectedAuditLog.method && selectedAuditLog.path && (
                            <DetailItem label="Endpoint">
                              <div className="flex items-center gap-2">
                                <Badge variant="outline" className={
                                  selectedAuditLog.method === 'GET' ? "bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-300" :
                                  selectedAuditLog.method === 'POST' ? "bg-green-50 text-green-700 border-green-200 dark:bg-green-950 dark:text-green-300" :
                                  selectedAuditLog.method === 'PUT' ? "bg-yellow-50 text-yellow-700 border-yellow-200 dark:bg-yellow-950 dark:text-yellow-300" :
                                  selectedAuditLog.method === 'DELETE' ? "bg-red-50 text-red-700 border-red-200 dark:bg-red-950 dark:text-red-300" : ""
                                }>
                                  {selectedAuditLog.method}
                                </Badge>
                                <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded break-all">
                                  {selectedAuditLog.path}
                                </code>
                              </div>
                            </DetailItem>
                          )}

                          <DetailItem label="IP Address">
                            <span className="font-mono text-xs">{selectedAuditLog.ip_address || 'Not recorded'}</span>
                          </DetailItem>

                          {selectedAuditLog.duration !== undefined && (
                            <DetailItem label="Duration">
                              <span className="font-mono text-xs">{selectedAuditLog.duration}ms</span>
                            </DetailItem>
                          )}

                          {selectedAuditLog.user_agent && selectedAuditLog.user_agent !== 'Not recorded' && (
                            <DetailItem label="User Agent">
                              <p className="font-mono text-xs break-all bg-muted/50 rounded-md p-2">
                                {selectedAuditLog.user_agent}
                              </p>
                            </DetailItem>
                          )}
                        </dl>
                      </TabsContent>

                      <TabsContent value="changes" className="space-y-4">
                        {(!selectedAuditLog.old_values && !selectedAuditLog.new_values) ? (
                          <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
                            <Icon icon={icons.file} className="h-6 w-6 mb-2 opacity-40" />
                            <p className="text-sm">No changes recorded for this event.</p>
                          </div>
                        ) : (
                          <div className="grid md:grid-cols-2 gap-4">
                            {selectedAuditLog.old_values && (
                              <JsonViewer
                                title="Old Values"
                                data={typeof selectedAuditLog.old_values === 'string'
                                  ? JSON.parse(selectedAuditLog.old_values)
                                  : selectedAuditLog.old_values
                                }
                              />
                            )}
                            {selectedAuditLog.new_values && (
                              <JsonViewer
                                title="New Values"
                                data={typeof selectedAuditLog.new_values === 'string'
                                  ? JSON.parse(selectedAuditLog.new_values)
                                  : selectedAuditLog.new_values
                                }
                              />
                            )}
                          </div>
                        )}
                      </TabsContent>

                      <TabsContent value="technical" className="space-y-4">
                        <dl className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-5">
                          <DetailItem label="Event ID">
                            <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded break-all">
                              {selectedAuditLog.id}
                            </code>
                          </DetailItem>

                          {selectedAuditLog.auth_method && (
                            <DetailItem label="Auth Method">
                              <Badge variant="secondary" className="capitalize text-xs">{selectedAuditLog.auth_method}</Badge>
                            </DetailItem>
                          )}

                          {selectedAuditLog.auth_provider && (
                            <DetailItem label="Auth Provider">
                              <Badge variant="secondary" className="capitalize text-xs">{selectedAuditLog.auth_provider}</Badge>
                            </DetailItem>
                          )}

                          {selectedAuditLog.user_id && (
                            <DetailItem label="User ID">
                              <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded">{selectedAuditLog.user_id}</code>
                            </DetailItem>
                          )}
                        </dl>

                        {selectedAuditLog.metadata && (
                          <JsonViewer
                            title="Metadata"
                            data={typeof selectedAuditLog.metadata === 'string'
                              ? JSON.parse(selectedAuditLog.metadata)
                              : selectedAuditLog.metadata
                            }
                          />
                        )}
                      </TabsContent>
                    </div>
                  </Tabs>
                </div>
              </>
            )
          })()}
        </SheetContent>
      </Sheet>
    </div>
  )
}
