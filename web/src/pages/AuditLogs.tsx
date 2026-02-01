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
import { ChevronDown, Calendar, Filter, RefreshCw, Search, X, Copy, Download } from "lucide-react"
import { format } from "date-fns"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { AuditLog } from "@/types/api"
import { useAuditLogs } from "@/hooks/useAuditLogs"
import { createAuditColumns, getStatusBadge } from "@/components/audit-logs/columns"
import { DetailItem } from "@/components/common/DetailItem"
import { JsonViewer } from "@/components/common/JsonViewer"

export default function AuditLogs() {
  // Use the extracted hook for data management
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

  // Create columns with row click handler
  const columns = React.useMemo(
    () => createAuditColumns((log) => {
      setSelectedAuditLog(log)
      setIsDrawerOpen(true)
    }),
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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Audit Logs</h1>
          <p className="text-muted-foreground">
            View and search through system audit logs and security events
          </p>
        </div>
        <Button onClick={refetch} disabled={isLoading}>
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
              <Button onClick={refetch} disabled={isLoading}>
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
                      {columns.map((_, j) => (
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
                      colSpan={columns.length}
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

      {/* Audit Log Details Drawer - Redesigned */}
      <Sheet open={isDrawerOpen} onOpenChange={setIsDrawerOpen}>
        <SheetContent className="w-full sm:max-w-2xl p-0 flex flex-col">
          {selectedAuditLog && (
            <>
              <SheetHeader className="p-4 pr-12 border-b">
                <div className="flex flex-col gap-3">
                  <div className="flex items-start gap-3">
                    {getStatusBadge(selectedAuditLog.event_result)}
                    <div className="flex items-center gap-2 ml-auto">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => navigator.clipboard.writeText(selectedAuditLog.id)}
                      >
                        <Copy className="h-3.5 w-3.5 mr-1.5" />
                        Copy ID
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          const blob = new Blob([JSON.stringify(selectedAuditLog, null, 2)], { type: 'application/json' });
                          const url = URL.createObjectURL(blob);
                          const a = document.createElement('a');
                          a.href = url;
                          a.download = `audit-${selectedAuditLog.id}.json`;
                          a.click();
                        }}
                      >
                        <Download className="h-3.5 w-3.5 mr-1.5" />
                        Export
                      </Button>
                    </div>
                  </div>
                  <div>
                    <SheetTitle className="text-lg">
                      {selectedAuditLog.event_type.replace(/_/g, ' ')}: {selectedAuditLog.event_action}
                    </SheetTitle>
                    <SheetDescription className="mt-1">
                      {format(new Date(selectedAuditLog.timestamp), "EEEE, MMMM do, yyyy 'at' HH:mm:ss")}
                    </SheetDescription>
                  </div>
                </div>
              </SheetHeader>

              <div className="flex-grow overflow-y-auto">
                <Tabs defaultValue="overview" className="p-4">
                  <TabsList className="grid w-full grid-cols-4">
                    <TabsTrigger value="overview">Overview</TabsTrigger>
                    <TabsTrigger value="request">Request</TabsTrigger>
                    <TabsTrigger value="changes">Changes</TabsTrigger>
                    <TabsTrigger value="technical">Technical</TabsTrigger>
                  </TabsList>

                  <div className="mt-4 space-y-6">
                    <TabsContent value="overview">
                      <dl className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-6">
                        <DetailItem label="User">
                          {selectedAuditLog.user ? (
                            <div className="flex items-center gap-3">
                              <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center">
                                <span className="text-xs font-medium text-primary">
                                  {selectedAuditLog.user.name?.[0] || selectedAuditLog.user.email?.[0]?.toUpperCase() || 'U'}
                                </span>
                              </div>
                              <div>
                                <div className="text-sm font-medium">{selectedAuditLog.user.name || selectedAuditLog.user.email}</div>
                                {selectedAuditLog.user.name && selectedAuditLog.user.email && (
                                  <div className="text-xs text-muted-foreground">{selectedAuditLog.user.email}</div>
                                )}
                              </div>
                            </div>
                          ) : (
                            <div className="flex items-center gap-3">
                              <div className="h-8 w-8 rounded-full bg-muted flex items-center justify-center">
                                <span className="text-xs">SYS</span>
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
                                <code className="text-xs font-mono">{selectedAuditLog.resource_id}</code>
                              </DetailItem>
                            )}
                          </>
                        )}

                        {selectedAuditLog.message && (
                          <DetailItem label="Message">
                            <div className="bg-muted/50 rounded p-2 text-xs">{selectedAuditLog.message}</div>
                          </DetailItem>
                        )}

                        {selectedAuditLog.error_code && (
                          <DetailItem label="Error">
                            <Badge variant="destructive" className="font-mono">{selectedAuditLog.error_code}</Badge>
                          </DetailItem>
                        )}
                      </dl>
                    </TabsContent>

                    <TabsContent value="request">
                      <dl className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-6">
                        {selectedAuditLog.method && selectedAuditLog.path && (
                          <DetailItem label="Path">
                            <div className="flex items-center gap-2">
                              <Badge variant="outline" className={
                                selectedAuditLog.method === 'GET' ? "bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-300" :
                                selectedAuditLog.method === 'POST' ? "bg-green-50 text-green-700 border-green-200 dark:bg-green-950 dark:text-green-300" :
                                selectedAuditLog.method === 'PUT' ? "bg-yellow-50 text-yellow-700 border-yellow-200 dark:bg-yellow-950 dark:text-yellow-300" :
                                selectedAuditLog.method === 'DELETE' ? "bg-red-50 text-red-700 border-red-200 dark:bg-red-950 dark:text-red-300" : ""
                              }>
                                {selectedAuditLog.method}
                              </Badge>
                              <code className="text-xs bg-muted px-2 py-1 rounded break-all">
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
                            <p className="font-mono text-xs break-all bg-muted/50 rounded p-2">
                              {selectedAuditLog.user_agent}
                            </p>
                          </DetailItem>
                        )}
                      </dl>
                    </TabsContent>

                    <TabsContent value="changes" className="space-y-4">
                      {(!selectedAuditLog.old_values && !selectedAuditLog.new_values) ? (
                        <p className="text-sm text-muted-foreground">No changes recorded for this event.</p>
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
                      <dl className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-6">
                        <DetailItem label="Event ID">
                          <code className="text-xs font-mono bg-muted px-2 py-1 rounded break-all">
                            {selectedAuditLog.id}
                          </code>
                        </DetailItem>

                        {selectedAuditLog.auth_method && (
                          <DetailItem label="Auth Method">
                            <Badge variant="secondary" className="capitalize">{selectedAuditLog.auth_method}</Badge>
                          </DetailItem>
                        )}

                        {selectedAuditLog.auth_provider && (
                          <DetailItem label="Auth Provider">
                            <Badge variant="secondary" className="capitalize">{selectedAuditLog.auth_provider}</Badge>
                          </DetailItem>
                        )}

                        {selectedAuditLog.user_id && (
                          <DetailItem label="User ID">
                            <code className="text-xs font-mono">{selectedAuditLog.user_id}</code>
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