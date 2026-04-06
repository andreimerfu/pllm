"use client"

import * as React from "react"
import {
  ColumnFiltersState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  SortingState,
  useReactTable,
  VisibilityState,
} from "@tanstack/react-table"
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { format, formatDistanceToNow } from "date-fns"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
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

// Inline column definitions for the redesigned table
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
    pageCount,
  } = useAuditLogs()

  // UI state
  const [sorting, setSorting] = React.useState<SortingState>([
    { id: "timestamp", desc: true }
  ])
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>([])
  const [columnVisibility, setColumnVisibility] = React.useState<VisibilityState>({})
  const [globalFilter, setGlobalFilter] = React.useState("")

  // Drawer state
  const [selectedAuditLog, setSelectedAuditLog] = React.useState<AuditLog | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = React.useState(false)

  // Search input for inline filter (combines action + user_id search)
  const [searchInput, setSearchInput] = React.useState("")

  // Determine if any filters are active beyond defaults
  const hasActiveFilters = React.useMemo(() => {
    return !!(filters.action || filters.resource || filters.result || filters.user_id || searchInput)
  }, [filters, searchInput])

  // Debounced search effect: maps searchInput to action filter
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

  // Columns for the redesigned table
  const columns = React.useMemo(
    () => [
      {
        accessorKey: "timestamp",
        header: ({ column }: { column: { toggleSorting: (asc: boolean) => void; getIsSorted: () => string | false } }) => (
          <Button
            variant="ghost"
            onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
            className="p-0 h-auto font-medium text-xs uppercase tracking-wider text-muted-foreground hover:text-foreground"
          >
            Time
            <Icon icon={icons.arrowUpDown} className="ml-1.5 h-3 w-3" />
          </Button>
        ),
        cell: ({ row }: { row: { getValue: (key: string) => string } }) => {
          const timestamp = new Date(row.getValue("timestamp"))
          const relative = formatDistanceToNow(timestamp, { addSuffix: true })
          return (
            <div className="font-mono text-xs text-muted-foreground" title={format(timestamp, "yyyy-MM-dd HH:mm:ss")}>
              {relative}
            </div>
          )
        },
      },
      {
        accessorKey: "event_action",
        header: () => (
          <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Event</span>
        ),
        cell: ({ row }: { row: { original: AuditLog } }) => {
          const auditLog = row.original
          return (
            <div className="flex items-center gap-2">
              <span className={`inline-block h-2 w-2 rounded-full shrink-0 ${getResultDot(auditLog.event_result)}`} />
              <div className="min-w-0">
                <div className="text-sm font-medium truncate">{auditLog.event_action}</div>
                <div className="text-xs text-muted-foreground capitalize truncate">
                  {auditLog.event_type.replace(/_/g, ' ')}
                </div>
              </div>
            </div>
          )
        },
      },
      {
        accessorKey: "user",
        header: () => (
          <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">User</span>
        ),
        cell: ({ row }: { row: { original: AuditLog } }) => {
          const auditLog = row.original
          if (!auditLog.user) {
            return <span className="text-xs text-muted-foreground">System</span>
          }
          const initial = auditLog.user.name?.[0] || auditLog.user.email?.[0]?.toUpperCase() || 'U'
          return (
            <div className="flex items-center gap-2">
              <div className="h-6 w-6 rounded-full bg-primary/15 flex items-center justify-center shrink-0">
                <span className="text-[10px] font-semibold text-primary">{initial}</span>
              </div>
              <span className="text-sm truncate">{auditLog.user.name || auditLog.user.email || 'Unknown'}</span>
            </div>
          )
        },
      },
      {
        accessorKey: "resource_type",
        header: () => (
          <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Resource</span>
        ),
        cell: ({ row }: { row: { original: AuditLog } }) => {
          const auditLog = row.original
          if (!auditLog.resource_type) {
            return <span className="text-xs text-muted-foreground">-</span>
          }
          return (
            <div>
              <span className="text-sm capitalize">{auditLog.resource_type}</span>
              {auditLog.resource_id && (
                <div className="text-[10px] font-mono text-muted-foreground truncate max-w-[120px]">
                  {auditLog.resource_id.slice(0, 12)}
                </div>
              )}
            </div>
          )
        },
      },
      {
        accessorKey: "event_result",
        header: () => (
          <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Result</span>
        ),
        cell: ({ row }: { row: { getValue: (key: string) => string } }) => getStatusBadge(row.getValue("event_result")),
      },
      {
        accessorKey: "ip_address",
        header: () => (
          <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">IP</span>
        ),
        cell: ({ row }: { row: { getValue: (key: string) => string } }) => (
          <span className="font-mono text-xs text-muted-foreground">{row.getValue("ip_address") || "-"}</span>
        ),
      },
      {
        id: "actions",
        cell: ({ row }: { row: { original: AuditLog } }) => (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
            onClick={(e: React.MouseEvent) => {
              e.stopPropagation()
              setSelectedAuditLog(row.original)
              setIsDrawerOpen(true)
            }}
          >
            <Icon icon={icons.chevronRight} className="h-4 w-4" />
          </Button>
        ),
      },
    ],
    []
  )

  const table = useReactTable({
    data: auditLogs,
    columns,
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    onColumnVisibilityChange: setColumnVisibility,
    onGlobalFilterChange: setGlobalFilter,
    globalFilterFn: "includesString",
    state: {
      sorting,
      columnFilters,
      columnVisibility,
      globalFilter,
    },
    manualPagination: true,
    pageCount,
  })

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

  return (
    <div className="space-y-4">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Audit Logs</h1>
            <p className="text-sm text-muted-foreground">
              System events and security activity
            </p>
          </div>
          <Badge variant="secondary" className="font-mono text-xs tabular-nums">
            {total.toLocaleString()}
          </Badge>
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

      {/* Compact Inline Filter Bar */}
      <div className="flex items-center gap-2 flex-wrap">
        {/* Search */}
        <div className="relative flex-1 min-w-[200px] max-w-sm">
          <Icon icon={icons.search} className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder="Search actions..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="h-8 pl-8 text-sm"
          />
        </div>

        {/* Date Range Picker */}
        <Popover>
          <PopoverTrigger asChild>
            <Button variant="outline" size="sm" className="h-8 text-xs font-normal gap-1.5">
              <Icon icon={icons.calendar} className="h-3.5 w-3.5" />
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
          <SelectTrigger className="h-8 w-[130px] text-xs">
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

        {/* Result/Status */}
        <Select
          value={filters.result || "all-results"}
          onValueChange={(value) => setFilters(prev => ({ ...prev, result: value === "all-results" ? "" : value }))}
        >
          <SelectTrigger className="h-8 w-[120px] text-xs">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all-results">All results</SelectItem>
            <SelectItem value="success">Success</SelectItem>
            <SelectItem value="failure">Failure</SelectItem>
            <SelectItem value="error">Error</SelectItem>
            <SelectItem value="warning">Warning</SelectItem>
          </SelectContent>
        </Select>

        {/* User ID Filter */}
        <Input
          placeholder="User ID..."
          value={filters.user_id}
          onChange={(e) => setFilters(prev => ({ ...prev, user_id: e.target.value }))}
          className="h-8 w-[140px] text-xs"
        />

        {/* Column Visibility */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="h-8 text-xs ml-auto">
              <Icon icon={icons.layers} className="h-3.5 w-3.5 mr-1.5" />
              Columns
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {table
              .getAllColumns()
              .filter((column) => column.getCanHide())
              .map((column) => (
                <DropdownMenuCheckboxItem
                  key={column.id}
                  className="capitalize text-xs"
                  checked={column.getIsVisible()}
                  onCheckedChange={(value) => column.toggleVisibility(!!value)}
                >
                  {column.id.replace(/_/g, ' ')}
                </DropdownMenuCheckboxItem>
              ))}
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Clear Filters */}
        {hasActiveFilters && (
          <Button variant="ghost" size="sm" className="h-8 text-xs text-muted-foreground" onClick={handleClearAll}>
            <Icon icon={icons.close} className="h-3.5 w-3.5 mr-1" />
            Clear
          </Button>
        )}
      </div>

      {/* Active Filter Pills */}
      {activeFilterPills.length > 0 && (
        <div className="flex items-center gap-1.5 flex-wrap">
          {activeFilterPills.map((pill) => (
            <Badge
              key={pill.key}
              variant="secondary"
              className="text-xs gap-1 pl-2 pr-1 py-0.5 cursor-pointer hover:bg-secondary/80"
              onClick={pill.onClear}
            >
              {pill.label}
              <Icon icon={icons.close} className="h-3 w-3" />
            </Badge>
          ))}
        </div>
      )}

      {/* Log Table */}
      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="hover:bg-transparent border-b">
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} className="h-9 px-3">
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array(pageSize > 10 ? 10 : pageSize).fill(0).map((_, i) => (
                <TableRow key={i}>
                  {columns.map((_, j) => (
                    <TableCell key={j} className="py-2.5 px-3">
                      <Skeleton className="h-4 w-full" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className="hover:bg-muted/50 cursor-pointer transition-colors"
                  onClick={() => {
                    setSelectedAuditLog(row.original)
                    setIsDrawerOpen(true)
                  }}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id} className="py-2 px-3">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-32 text-center">
                  <div className="flex flex-col items-center gap-2 text-muted-foreground">
                    <Icon icon={icons.auditLogs} className="h-8 w-8 opacity-40" />
                    <span className="text-sm">No audit logs found</span>
                  </div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {total > pageSize && (
        <div className="flex items-center justify-between">
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

      {/* Audit Log Detail Drawer */}
      <Sheet open={isDrawerOpen} onOpenChange={setIsDrawerOpen}>
        <SheetContent className="w-full sm:max-w-2xl p-0 flex flex-col">
          {selectedAuditLog && (
            <>
              <SheetHeader className="p-4 pr-12 border-b bg-muted/30">
                <div className="flex flex-col gap-3">
                  <div className="flex items-start gap-3">
                    <span className={`inline-block h-2.5 w-2.5 rounded-full mt-1.5 shrink-0 ${getResultDot(selectedAuditLog.event_result)}`} />
                    <div className="flex-1 min-w-0">
                      <SheetTitle className="text-base font-semibold">
                        {selectedAuditLog.event_action}
                      </SheetTitle>
                      <SheetDescription className="mt-0.5 text-xs font-mono">
                        {format(new Date(selectedAuditLog.timestamp), "yyyy-MM-dd HH:mm:ss")}
                        <span className="text-muted-foreground/60 ml-2">
                          ({formatDistanceToNow(new Date(selectedAuditLog.timestamp), { addSuffix: true })})
                        </span>
                      </SheetDescription>
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={() => navigator.clipboard.writeText(selectedAuditLog.id)}
                      >
                        <Icon icon={icons.copy} className="h-3 w-3 mr-1" />
                        ID
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs"
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
                  <div className="flex items-center gap-2 flex-wrap">
                    {getStatusBadge(selectedAuditLog.event_result)}
                    <Badge variant="outline" className="text-xs capitalize font-normal">
                      {selectedAuditLog.event_type.replace(/_/g, ' ')}
                    </Badge>
                    {selectedAuditLog.resource_type && (
                      <Badge variant="outline" className="text-xs capitalize font-normal">
                        {selectedAuditLog.resource_type}
                      </Badge>
                    )}
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
                                <span className="text-[10px] font-semibold text-primary">
                                  {selectedAuditLog.user.name?.[0] || selectedAuditLog.user.email?.[0]?.toUpperCase() || 'U'}
                                </span>
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
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}
