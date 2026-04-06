"use client"

import * as React from "react"
import {
  ColumnDef,
  ColumnFiltersState,
  SortingState,
  VisibilityState,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table"
import { Icon } from "@iconify/react"
import { icons } from "@/lib/icons"

import { Button } from "../ui/button"
import { Input } from "../ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "../ui/table"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select"
import { ApiKey, ExpandedRowContent } from "./columns"
import { cn } from "@/lib/utils"

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[]
  data: TData[]
  teams?: { id: string; name: string }[]
}

const statusFilters = [
  { value: "all", label: "All" },
  { value: "active", label: "Active" },
  { value: "inactive", label: "Inactive" },
  { value: "revoked", label: "Revoked" },
]

export function DataTable<TData extends ApiKey, TValue>({
  columns,
  data,
  teams,
}: DataTableProps<TData, TValue>) {
  const [sorting, setSorting] = React.useState<SortingState>([])
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>([])
  const [columnVisibility, setColumnVisibility] = React.useState<VisibilityState>({})
  const [globalFilter, setGlobalFilter] = React.useState("")
  const [statusFilter, setStatusFilter] = React.useState("all")
  const [teamFilter, setTeamFilter] = React.useState("all")
  const [expandedRows, setExpandedRows] = React.useState<Record<string, boolean>>({})

  const table = useReactTable({
    data,
    columns,
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
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
  })

  // Apply status filter
  React.useEffect(() => {
    table.getColumn("status")?.setFilterValue(statusFilter === "all" ? undefined : statusFilter)
  }, [statusFilter, table])

  // Apply team filter
  React.useEffect(() => {
    table.getColumn("team")?.setFilterValue(teamFilter === "all" ? undefined : teamFilter)
  }, [teamFilter, table])

  const toggleRowExpansion = (rowId: string) => {
    setExpandedRows(prev => ({
      ...prev,
      [rowId]: !prev[rowId],
    }))
  }

  const hasActiveFilters = globalFilter.length > 0 || statusFilter !== "all" || teamFilter !== "all"

  // Get unique teams from data for filter dropdown
  const uniqueTeams = React.useMemo(() => {
    if (teams && teams.length > 0) return teams
    const teamMap = new Map<string, string>()
    data.forEach(item => {
      if (item.team) {
        teamMap.set(item.team.id, item.team.name)
      }
    })
    return Array.from(teamMap, ([id, name]) => ({ id, name }))
  }, [data, teams])

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        {/* Search */}
        <div className="relative flex-1 max-w-sm">
          <Icon icon={icons.search} className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search keys..."
            value={globalFilter}
            onChange={(e) => setGlobalFilter(e.target.value)}
            className="pl-9 h-9"
          />
          {globalFilter && (
            <Button
              variant="ghost"
              size="sm"
              className="absolute right-1 top-1/2 h-6 w-6 -translate-y-1/2 p-0"
              onClick={() => setGlobalFilter("")}
            >
              <Icon icon={icons.close} className="h-3 w-3" />
            </Button>
          )}
        </div>

        <div className="flex items-center gap-2">
          {/* Status segmented toggle */}
          <div className="inline-flex h-9 items-center rounded-lg border bg-muted/50 p-0.5">
            {statusFilters.map((filter) => (
              <button
                key={filter.value}
                onClick={() => setStatusFilter(filter.value)}
                className={cn(
                  "inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-xs font-medium transition-all",
                  statusFilter === filter.value
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {filter.value !== "all" && (
                  <span className={cn(
                    "mr-1.5 inline-block h-1.5 w-1.5 rounded-full",
                    filter.value === "active" && "bg-emerald-500",
                    filter.value === "inactive" && "bg-zinc-400",
                    filter.value === "revoked" && "bg-red-500",
                  )} />
                )}
                {filter.label}
              </button>
            ))}
          </div>

          {/* Team filter */}
          {uniqueTeams.length > 0 && (
            <Select value={teamFilter} onValueChange={setTeamFilter}>
              <SelectTrigger className="h-9 w-[150px]">
                <Icon icon={icons.teams} className="mr-2 h-3.5 w-3.5 text-muted-foreground" />
                <SelectValue placeholder="Team" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Teams</SelectItem>
                <SelectItem value="personal">Personal</SelectItem>
                {uniqueTeams.map((team) => (
                  <SelectItem key={team.id} value={team.id}>
                    {team.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}

          {/* Clear Filters */}
          {hasActiveFilters && (
            <Button
              variant="ghost"
              size="sm"
              className="h-9 px-2 text-muted-foreground"
              onClick={() => {
                setGlobalFilter("")
                setStatusFilter("all")
                setTeamFilter("all")
                table.resetColumnFilters()
              }}
            >
              <Icon icon={icons.close} className="mr-1 h-3.5 w-3.5" />
              Clear
            </Button>
          )}
        </div>
      </div>

      {/* Table */}
      <div className="rounded-lg border bg-card">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="hover:bg-transparent border-border/50">
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} className="whitespace-nowrap text-[11px] uppercase tracking-wider font-medium text-muted-foreground h-10">
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext()
                        )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => {
                const isExpanded = expandedRows[row.id]
                return (
                  <React.Fragment key={row.id}>
                    <TableRow
                      data-state={isExpanded && "expanded"}
                      className={cn(
                        "cursor-pointer transition-colors hover:bg-muted/50",
                        isExpanded && "bg-muted/30"
                      )}
                      onClick={() => toggleRowExpansion(row.id)}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <TableCell key={cell.id} className="py-3">
                          {flexRender(
                            cell.column.columnDef.cell,
                            cell.getContext()
                          )}
                        </TableCell>
                      ))}
                    </TableRow>
                    {isExpanded && (
                      <TableRow className="hover:bg-transparent">
                        <TableCell colSpan={row.getVisibleCells().length} className="p-0">
                          <ExpandedRowContent apiKey={row.original} />
                        </TableCell>
                      </TableRow>
                    )}
                  </React.Fragment>
                )
              })
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-32 text-center"
                >
                  <div className="flex flex-col items-center gap-2 text-muted-foreground">
                    <Icon icon={icons.keys} className="h-8 w-8 opacity-50" />
                    <p className="text-sm">No API keys found.</p>
                    {hasActiveFilters && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setGlobalFilter("")
                          setStatusFilter("all")
                          setTeamFilter("all")
                          table.resetColumnFilters()
                        }}
                      >
                        Clear filters
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {table.getRowModel().rows.length > 0 && (
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="text-xs text-muted-foreground">
            Showing {table.getRowModel().rows.length} of{" "}
            {table.getFilteredRowModel().rows.length} key(s)
          </div>
          <div className="flex items-center space-x-2">
            <div className="flex items-center space-x-2">
              <p className="text-xs text-muted-foreground">Per page</p>
              <Select
                value={`${table.getState().pagination.pageSize}`}
                onValueChange={(value) => {
                  table.setPageSize(Number(value))
                }}
              >
                <SelectTrigger className="h-8 w-[65px]">
                  <SelectValue placeholder={table.getState().pagination.pageSize} />
                </SelectTrigger>
                <SelectContent side="top">
                  {[10, 20, 30, 50].map((pageSize) => (
                    <SelectItem key={pageSize} value={`${pageSize}`}>
                      {pageSize}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center space-x-1">
              <Button
                variant="outline"
                size="sm"
                className="h-8"
                onClick={() => table.previousPage()}
                disabled={!table.getCanPreviousPage()}
              >
                <Icon icon={icons.chevronLeft} className="h-4 w-4" />
              </Button>
              <div className="flex items-center justify-center text-xs font-medium min-w-[80px]">
                {table.getState().pagination.pageIndex + 1} / {table.getPageCount()}
              </div>
              <Button
                variant="outline"
                size="sm"
                className="h-8"
                onClick={() => table.nextPage()}
                disabled={!table.getCanNextPage()}
              >
                <Icon icon={icons.chevronRight} className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
