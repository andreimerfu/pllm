"use client"

import * as React from "react"
import {
  ColumnDef,
  ColumnFiltersState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  SortingState,
  useReactTable,
  VisibilityState,
} from "@tanstack/react-table"
import { ArrowUpDown, ChevronDown, Calendar, Filter, RefreshCw, Search, X } from "lucide-react"
import { format, subDays, startOfDay, endOfDay } from "date-fns"
import { DateRange } from "react-day-picker"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
import { Skeleton } from "@/components/ui/skeleton"
import { useToast } from "@/hooks/use-toast"
import { AuditLog, AuditLogsResponse } from "@/types/api"
import { getAuditLogs } from "@/lib/api"

const getStatusBadge = (result: string) => {
  switch (result) {
    case 'success':
      return <Badge variant="default" className="bg-green-100 text-green-800 border-green-200">Success</Badge>
    case 'failure':
      return <Badge variant="destructive">Failure</Badge>
    case 'error':
      return <Badge variant="destructive" className="bg-red-100 text-red-800 border-red-200">Error</Badge>
    case 'warning':
      return <Badge variant="secondary" className="bg-yellow-100 text-yellow-800 border-yellow-200">Warning</Badge>
    default:
      return <Badge variant="outline">{result}</Badge>
  }
}

const getSeverityColor = (eventType: string) => {
  const securityEvents = ['auth', 'login', 'logout', 'password_change', 'security_alert', 'access_denied']
  const highRiskEvents = ['budget_exceeded', 'key_revoke', 'user_delete']
  
  if (securityEvents.includes(eventType)) return 'text-red-600'
  if (highRiskEvents.includes(eventType)) return 'text-orange-600'
  return 'text-gray-600'
}

export const auditColumns: ColumnDef<AuditLog>[] = [
  {
    accessorKey: "timestamp",
    header: ({ column }) => (
      <Button
        variant="ghost"
        onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        className="p-0 h-auto"
      >
        Time
        <ArrowUpDown className="ml-2 h-4 w-4" />
      </Button>
    ),
    cell: ({ row }) => {
      const timestamp = new Date(row.getValue("timestamp"))
      return (
        <div className="text-sm">
          <div>{format(timestamp, "MMM dd, yyyy")}</div>
          <div className="text-muted-foreground text-xs">{format(timestamp, "HH:mm:ss")}</div>
        </div>
      )
    },
  },
  {
    accessorKey: "user",
    header: "User",
    cell: ({ row }) => {
      const auditLog = row.original
      return (
        <div className="text-sm">
          {auditLog.user ? (
            <>
              <div>{auditLog.user.name || auditLog.user.email || 'Unknown User'}</div>
              {auditLog.user.email && <div className="text-muted-foreground text-xs">{auditLog.user.email}</div>}
            </>
          ) : (
            <span className="text-muted-foreground">System</span>
          )}
        </div>
      )
    },
  },
  {
    accessorKey: "event_action",
    header: "Action",
    cell: ({ row }) => {
      const auditLog = row.original
      return (
        <div className="text-sm">
          <div className={`font-medium ${getSeverityColor(auditLog.event_type)}`}>
            {auditLog.event_action}
          </div>
          <div className="text-muted-foreground text-xs capitalize">
            {auditLog.event_type.replace(/_/g, ' ')}
          </div>
        </div>
      )
    },
  },
  {
    accessorKey: "resource_type",
    header: "Resource",
    cell: ({ row }) => {
      const auditLog = row.original
      return auditLog.resource_type ? (
        <div className="text-sm">
          <div className="font-medium capitalize">{auditLog.resource_type}</div>
          {auditLog.resource_id && (
            <div className="text-muted-foreground text-xs font-mono">
              {auditLog.resource_id.slice(0, 8)}...
            </div>
          )}
        </div>
      ) : (
        <span className="text-muted-foreground">-</span>
      )
    },
  },
  {
    accessorKey: "event_result",
    header: "Result",
    cell: ({ row }) => getStatusBadge(row.getValue("event_result")),
  },
  {
    accessorKey: "ip_address",
    header: "IP Address",
    cell: ({ row }) => (
      <div className="text-sm font-mono">{row.getValue("ip_address") || "-"}</div>
    ),
  },
  {
    accessorKey: "method",
    header: "Method",
    cell: ({ row }) => {
      const method = row.getValue("method") as string
      if (!method) return <span className="text-muted-foreground">-</span>
      
      const methodColors = {
        GET: "bg-blue-100 text-blue-800 border-blue-200",
        POST: "bg-green-100 text-green-800 border-green-200",
        PUT: "bg-yellow-100 text-yellow-800 border-yellow-200",
        DELETE: "bg-red-100 text-red-800 border-red-200",
      }
      
      return (
        <Badge variant="outline" className={methodColors[method as keyof typeof methodColors] || ""}>
          {method}
        </Badge>
      )
    },
  },
]

export default function AuditLogs() {
  const [sorting, setSorting] = React.useState<SortingState>([
    { id: "timestamp", desc: true }
  ])
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>([])
  const [columnVisibility, setColumnVisibility] = React.useState<VisibilityState>({})
  const [globalFilter, setGlobalFilter] = React.useState("")
  
  // Data and loading state
  const [auditLogs, setAuditLogs] = React.useState<AuditLog[]>([])
  const [isLoading, setIsLoading] = React.useState(true)
  const [total, setTotal] = React.useState(0)
  const [currentPage, setCurrentPage] = React.useState(0)
  const [pageSize] = React.useState(50)
  
  // Filters state
  const [filters, setFilters] = React.useState({
    action: "",
    resource: "",
    user_id: "",
    result: "",
  })
  const [dateRange, setDateRange] = React.useState<DateRange | undefined>({
    from: subDays(new Date(), 30),
    to: new Date(),
  })
  
  // Drawer state
  const [selectedAuditLog, setSelectedAuditLog] = React.useState<AuditLog | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = React.useState(false)
  
  const { toast } = useToast()

  const fetchAuditLogs = React.useCallback(async () => {
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
        if (apiFilters[key as keyof typeof apiFilters] === "" || 
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

  React.useEffect(() => {
    fetchAuditLogs()
  }, [fetchAuditLogs])

  const clearFilters = () => {
    setFilters({
      action: "",
      resource: "",
      user_id: "",
      result: "",
    })
    setDateRange({
      from: subDays(new Date(), 30),
      to: new Date(),
    })
    setCurrentPage(0)
  }

  const table = useReactTable({
    data: auditLogs,
    columns: auditColumns,
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
    pageCount: Math.ceil(total / pageSize),
  })

  const handlePageChange = (newPage: number) => {
    setCurrentPage(newPage)
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Audit Logs</h1>
          <p className="text-muted-foreground">
            View and search through system audit logs and security events
          </p>
        </div>
        <Button onClick={fetchAuditLogs} disabled={isLoading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Filter className="h-5 w-5" />
            Filters
          </CardTitle>
          <CardDescription>
            Filter audit logs by date range, action, resource, and result status
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
            {/* Date Range */}
            <div className="space-y-2">
              <Label>Date Range</Label>
              <Popover>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    className="w-full justify-start text-left font-normal"
                  >
                    <Calendar className="mr-2 h-4 w-4" />
                    {dateRange?.from ? (
                      dateRange.to ? (
                        <>
                          {format(dateRange.from, "LLL dd")} -{" "}
                          {format(dateRange.to, "LLL dd, y")}
                        </>
                      ) : (
                        format(dateRange.from, "LLL dd, y")
                      )
                    ) : (
                      "Pick a date range"
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
            </div>

            {/* Action Filter */}
            <div className="space-y-2">
              <Label>Action</Label>
              <Input
                placeholder="Filter by action..."
                value={filters.action}
                onChange={(e) => setFilters(prev => ({ ...prev, action: e.target.value }))}
              />
            </div>

            {/* Resource Filter */}
            <div className="space-y-2">
              <Label>Resource</Label>
              <Select
                value={filters.resource || undefined}
                onValueChange={(value) => setFilters(prev => ({ ...prev, resource: value === "clear-resource" ? "" : (value || "") }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="All resources" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="clear-resource">All resources</SelectItem>
                  <SelectItem value="user">User</SelectItem>
                  <SelectItem value="team">Team</SelectItem>
                  <SelectItem value="key">Key</SelectItem>
                  <SelectItem value="api">API</SelectItem>
                  <SelectItem value="llm">LLM</SelectItem>
                  <SelectItem value="session">Session</SelectItem>
                  <SelectItem value="budget">Budget</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Result Filter */}
            <div className="space-y-2">
              <Label>Result</Label>
              <Select
                value={filters.result || undefined}
                onValueChange={(value) => setFilters(prev => ({ ...prev, result: value === "clear-result" ? "" : (value || "") }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="All results" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="clear-result">All results</SelectItem>
                  <SelectItem value="success">Success</SelectItem>
                  <SelectItem value="failure">Failure</SelectItem>
                  <SelectItem value="error">Error</SelectItem>
                  <SelectItem value="warning">Warning</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* User ID Filter */}
            <div className="space-y-2">
              <Label>User ID</Label>
              <Input
                placeholder="Filter by user ID..."
                value={filters.user_id}
                onChange={(e) => setFilters(prev => ({ ...prev, user_id: e.target.value }))}
              />
            </div>
          </div>

          <div className="flex items-center justify-between pt-4 border-t">
            <div className="flex items-center gap-2">
              <Button onClick={fetchAuditLogs} disabled={isLoading}>
                <Search className="h-4 w-4 mr-2" />
                Apply Filters
              </Button>
              <Button variant="outline" onClick={clearFilters}>
                <X className="h-4 w-4 mr-2" />
                Clear All
              </Button>
            </div>
            <div className="text-sm text-muted-foreground">
              {total} total events
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Data Table */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Audit Events</CardTitle>
              <CardDescription>
                Showing {auditLogs.length} of {total} events
              </CardDescription>
            </div>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline">
                  Columns <ChevronDown className="ml-2 h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {table
                  .getAllColumns()
                  .filter((column) => column.getCanHide())
                  .map((column) => {
                    return (
                      <DropdownMenuCheckboxItem
                        key={column.id}
                        className="capitalize"
                        checked={column.getIsVisible()}
                        onCheckedChange={(value) =>
                          column.toggleVisibility(!!value)
                        }
                      >
                        {column.id.replace(/_/g, ' ')}
                      </DropdownMenuCheckboxItem>
                    )
                  })}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          <div className="rounded-md border-t">
            <Table>
              <TableHeader>
                {table.getHeaderGroups().map((headerGroup) => (
                  <TableRow key={headerGroup.id}>
                    {headerGroup.headers.map((header) => {
                      return (
                        <TableHead key={header.id}>
                          {header.isPlaceholder
                            ? null
                            : flexRender(
                                header.column.columnDef.header,
                                header.getContext()
                              )}
                        </TableHead>
                      )
                    })}
                  </TableRow>
                ))}
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  Array(pageSize).fill(0).map((_, i) => (
                    <TableRow key={i}>
                      {auditColumns.map((_, j) => (
                        <TableCell key={j}>
                          <Skeleton className="h-4 w-full" />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                ) : table.getRowModel().rows?.length ? (
                  table.getRowModel().rows.map((row) => (
                    <TableRow
                      key={row.id}
                      className="hover:bg-muted/50 cursor-pointer"
                      onClick={() => {
                        setSelectedAuditLog(row.original)
                        setIsDrawerOpen(true)
                      }}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id}>
                          {flexRender(
                            cell.column.columnDef.cell,
                            cell.getContext()
                          )}
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell
                      colSpan={auditColumns.length}
                      className="h-24 text-center"
                    >
                      No audit logs found.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Pagination */}
      {total > pageSize && (
        <div className="flex items-center justify-between">
          <div className="text-sm text-muted-foreground">
            Showing {currentPage * pageSize + 1} to{" "}
            {Math.min((currentPage + 1) * pageSize, total)} of {total} events
          </div>
          <div className="flex items-center space-x-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => handlePageChange(Math.max(0, currentPage - 1))}
              disabled={currentPage === 0 || isLoading}
            >
              Previous
            </Button>
            <div className="text-sm">
              Page {currentPage + 1} of {Math.ceil(total / pageSize)}
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => handlePageChange(currentPage + 1)}
              disabled={currentPage >= Math.ceil(total / pageSize) - 1 || isLoading}
            >
              Next
            </Button>
          </div>
        </div>
      )}

      {/* Audit Log Details Drawer */}
      <Sheet open={isDrawerOpen} onOpenChange={setIsDrawerOpen}>
        <SheetContent className="w-[500px] sm:w-[540px] overflow-y-auto">
          <SheetHeader>
            <SheetTitle>Audit Event Details</SheetTitle>
            <SheetDescription>
              Detailed information about the selected audit event
            </SheetDescription>
          </SheetHeader>
          
          {selectedAuditLog && (
            <div className="grid flex-1 auto-rows-min gap-6 px-4 py-6">
              {/* Header with action and status */}
              <div className="flex items-start justify-between pb-4 border-b">
                <div>
                  <h3 className={`text-xl font-semibold ${getSeverityColor(selectedAuditLog.event_type)}`}>
                    {selectedAuditLog.event_action}
                  </h3>
                  <p className="text-sm text-muted-foreground capitalize mt-1">
                    {selectedAuditLog.event_type.replace(/_/g, ' ')} event
                  </p>
                </div>
                {getStatusBadge(selectedAuditLog.event_result)}
              </div>

              {/* Basic Information */}
              <div className="grid gap-3">
                <Label className="text-sm font-medium">Timestamp</Label>
                <div className="text-sm">
                  {format(new Date(selectedAuditLog.timestamp), "EEEE, MMMM do, yyyy 'at' HH:mm:ss")}
                </div>
              </div>

              {/* User Information */}
              <div className="grid gap-3">
                <Label className="text-sm font-medium">User</Label>
                {selectedAuditLog.user ? (
                  <div className="flex items-center gap-3">
                    <div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
                      <span className="text-sm font-medium text-primary">
                        {selectedAuditLog.user.name?.[0] || selectedAuditLog.user.email?.[0]?.toUpperCase() || 'U'}
                      </span>
                    </div>
                    <div>
                      <div className="text-sm font-medium">{selectedAuditLog.user.name || selectedAuditLog.user.email}</div>
                      {selectedAuditLog.user.name && selectedAuditLog.user.email && (
                        <div className="text-xs text-muted-foreground">{selectedAuditLog.user.email}</div>
                      )}
                      <div className="text-xs text-muted-foreground font-mono">{selectedAuditLog.user_id}</div>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-center gap-3">
                    <div className="h-10 w-10 rounded-full bg-muted flex items-center justify-center">
                      <span className="text-sm">SYS</span>
                    </div>
                    <div className="text-sm text-muted-foreground">System Event</div>
                  </div>
                )}
              </div>

              {/* Resource Information */}
              {selectedAuditLog.resource_type && (
                <div className="grid gap-3">
                  <Label className="text-sm font-medium">Resource</Label>
                  <div>
                    <div className="text-sm capitalize">{selectedAuditLog.resource_type}</div>
                    {selectedAuditLog.resource_id && (
                      <div className="text-xs text-muted-foreground font-mono mt-1">
                        {selectedAuditLog.resource_id}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Request Information */}
              <div className="grid gap-3">
                <Label className="text-sm font-medium">Request Details</Label>
                <div className="space-y-2 text-sm">
                  {selectedAuditLog.method && (
                    <div className="flex justify-between items-center">
                      <span className="text-muted-foreground">Method</span>
                      <Badge variant="outline" className={
                        selectedAuditLog.method === 'GET' ? "bg-blue-50 text-blue-700 border-blue-200" :
                        selectedAuditLog.method === 'POST' ? "bg-green-50 text-green-700 border-green-200" :
                        selectedAuditLog.method === 'PUT' ? "bg-yellow-50 text-yellow-700 border-yellow-200" :
                        selectedAuditLog.method === 'DELETE' ? "bg-red-50 text-red-700 border-red-200" : ""
                      }>
                        {selectedAuditLog.method}
                      </Badge>
                    </div>
                  )}
                  
                  {selectedAuditLog.path && (
                    <div className="flex justify-between items-start">
                      <span className="text-muted-foreground">Path</span>
                      <code className="text-xs bg-muted px-2 py-1 rounded break-all max-w-[280px]">
                        {selectedAuditLog.path}
                      </code>
                    </div>
                  )}
                  
                  <div className="flex justify-between items-center">
                    <span className="text-muted-foreground">IP Address</span>
                    <span className="font-mono text-xs">{selectedAuditLog.ip_address || 'Not recorded'}</span>
                  </div>
                  
                  {selectedAuditLog.duration !== undefined && (
                    <div className="flex justify-between items-center">
                      <span className="text-muted-foreground">Duration</span>
                      <span className="font-mono text-xs">{selectedAuditLog.duration}ms</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Authentication */}
              {(selectedAuditLog.auth_method || selectedAuditLog.auth_provider) && (
                <div className="grid gap-3">
                  <Label className="text-sm font-medium">Authentication</Label>
                  <div className="space-y-2 text-sm">
                    {selectedAuditLog.auth_method && (
                      <div className="flex justify-between items-center">
                        <span className="text-muted-foreground">Method</span>
                        <Badge variant="secondary" className="capitalize">{selectedAuditLog.auth_method}</Badge>
                      </div>
                    )}
                    {selectedAuditLog.auth_provider && (
                      <div className="flex justify-between items-center">
                        <span className="text-muted-foreground">Provider</span>
                        <Badge variant="secondary" className="capitalize">{selectedAuditLog.auth_provider}</Badge>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Message */}
              {selectedAuditLog.message && (
                <div className="grid gap-3">
                  <Label className="text-sm font-medium">Message</Label>
                  <div className="bg-muted/50 rounded p-3 text-sm">
                    {selectedAuditLog.message}
                  </div>
                </div>
              )}

              {/* Error */}
              {selectedAuditLog.error_code && (
                <div className="grid gap-3">
                  <Label className="text-sm font-medium text-red-600">Error</Label>
                  <div className="bg-red-50 border border-red-200 rounded p-3">
                    <div className="flex justify-between items-center">
                      <span className="text-sm text-red-600">Error Code</span>
                      <Badge variant="destructive" className="font-mono">{selectedAuditLog.error_code}</Badge>
                    </div>
                  </div>
                </div>
              )}

              {/* Changes */}
              {(selectedAuditLog.old_values || selectedAuditLog.new_values) && (
                <div className="grid gap-3">
                  <Label className="text-sm font-medium">Changes</Label>
                  <div className="space-y-3">
                    {selectedAuditLog.old_values && (
                      <div>
                        <div className="text-xs font-medium text-red-600 mb-2">Previous Values</div>
                        <div className="bg-red-50 border border-red-200 rounded p-3">
                          <pre className="text-xs text-red-700 whitespace-pre-wrap break-words">
                            {typeof selectedAuditLog.old_values === 'string' 
                              ? selectedAuditLog.old_values 
                              : JSON.stringify(selectedAuditLog.old_values, null, 2)}
                          </pre>
                        </div>
                      </div>
                    )}
                    {selectedAuditLog.new_values && (
                      <div>
                        <div className="text-xs font-medium text-green-600 mb-2">New Values</div>
                        <div className="bg-green-50 border border-green-200 rounded p-3">
                          <pre className="text-xs text-green-700 whitespace-pre-wrap break-words">
                            {typeof selectedAuditLog.new_values === 'string' 
                              ? selectedAuditLog.new_values 
                              : JSON.stringify(selectedAuditLog.new_values, null, 2)}
                          </pre>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Metadata */}
              {selectedAuditLog.metadata && (
                <div className="grid gap-3">
                  <Label className="text-sm font-medium">Metadata</Label>
                  <div className="bg-muted rounded p-3">
                    <pre className="text-xs whitespace-pre-wrap break-words">
                      {typeof selectedAuditLog.metadata === 'string' 
                        ? selectedAuditLog.metadata 
                        : JSON.stringify(selectedAuditLog.metadata, null, 2)}
                    </pre>
                  </div>
                </div>
              )}

              {/* User Agent */}
              {selectedAuditLog.user_agent && selectedAuditLog.user_agent !== 'Not recorded' && (
                <div className="grid gap-3">
                  <Label className="text-sm font-medium">User Agent</Label>
                  <div className="text-xs text-muted-foreground break-all bg-muted/50 rounded p-2">
                    {selectedAuditLog.user_agent}
                  </div>
                </div>
              )}

              {/* Event ID at bottom */}
              <div className="grid gap-3 pt-4 border-t">
                <Label className="text-sm font-medium">Event ID</Label>
                <code className="text-xs font-mono bg-muted px-3 py-2 rounded break-all">
                  {selectedAuditLog.id}
                </code>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}