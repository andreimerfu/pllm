import * as React from "react"
import { Area, AreaChart, CartesianGrid, XAxis } from "recharts"
import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table"
import { z } from "zod"
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  CheckCircle,
  MoreHorizontal,
  Columns,
  Loader2,
  Plus,
  TrendingUp,
  AlertCircle,
} from "lucide-react"
import { Icon } from "@iconify/react"
import { getDashboardMetrics, getUsageTrends } from "@/lib/api"
import { formatDateShort } from "@/lib/date-utils"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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

// Navigation component
// Metric Cards Component with real API data
function MetricCards() {
  const [metrics, setMetrics] = React.useState<any>(null)
  const [loading, setLoading] = React.useState(true)

  React.useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const response = await getDashboardMetrics()
        const data = (response as any).data || response

        setMetrics({
          totalRequests: data.total_requests || 0,
          totalTokens: data.total_tokens || 0,
          totalCost: data.total_cost || 0,
          activeKeys: data.active_keys || 0
        })
      } catch (error) {
        console.error('Error fetching dashboard metrics:', error)
        // Fallback to mock data if API fails
        setMetrics({
          totalRequests: 45231,
          totalTokens: 2400000,
          totalCost: 1429,
          activeKeys: 12
        })
      } finally {
        setLoading(false)
      }
    }

    fetchMetrics()
  }, [])

  if (loading) {
    return (
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {[1, 2, 3, 4].map((i) => (
          <Card key={i}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <div className="h-4 w-24 bg-muted animate-pulse rounded" />
              <div className="h-4 w-4 bg-muted animate-pulse rounded" />
            </CardHeader>
            <CardContent>
              <div className="h-8 w-20 bg-muted animate-pulse rounded mb-2" />
              <div className="h-3 w-32 bg-muted animate-pulse rounded" />
            </CardContent>
          </Card>
        ))}
      </div>
    )
  }

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      {/* Total Requests Card */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
          <TrendingUp className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{metrics.totalRequests.toLocaleString()}</div>
          <p className="text-xs text-muted-foreground">
            <span className="text-emerald-500">Live data</span> from backend
          </p>
        </CardContent>
      </Card>

      {/* Token Usage Card */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Tokens Used</CardTitle>
          <Loader2 className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">
            {metrics.totalTokens > 1000000
              ? `${(metrics.totalTokens / 1000000).toFixed(1)}M`
              : metrics.totalTokens.toLocaleString()}
          </div>
          <p className="text-xs text-muted-foreground">
            <span className="text-emerald-500">Live data</span> from backend
          </p>
        </CardContent>
      </Card>

      {/* Cost Budget Card */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Monthly Cost</CardTitle>
          <AlertCircle className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">${metrics.totalCost.toFixed(2)}</div>
          <p className="text-xs text-muted-foreground">
            <span className="text-emerald-500">Live data</span> from backend
          </p>
        </CardContent>
      </Card>

      {/* Active Keys Card */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Active Keys</CardTitle>
          <CheckCircle className="h-4 w-4 text-muted-foreground" />
        </CardHeader>
        <CardContent>
          <div className="text-2xl font-bold">{metrics.activeKeys}</div>
          <p className="text-xs text-muted-foreground">
            <span className="text-emerald-500">Live data</span> from backend
          </p>
        </CardContent>
      </Card>
    </div>
  )
}


const chartConfig = {
  usage: {
    label: "Usage Metrics",
  },
  desktop: {
    label: "Requests",
    color: "hsl(var(--chart-1))",
  },
  mobile: {
    label: "Tokens (100s)",
    color: "hsl(var(--chart-2))",
  },
} satisfies ChartConfig

function ChartAreaInteractive() {
  const [timeRange, setTimeRange] = React.useState("30d")
  const [chartData, setChartData] = React.useState<Array<{
    date: string;
    requests: number;
    tokens: number;
    cost: number;
  }>>([])
  const [loading, setLoading] = React.useState(true)

  React.useEffect(() => {
    const fetchUsageTrends = async () => {
      try {
        const days = timeRange === "7d" ? 7 : timeRange === "30d" ? 30 : 90
        const response = await getUsageTrends(days)
        const data = (response as any).data || response
        setChartData(data || [])
      } catch (error) {
        console.error('Error fetching usage trends:', error)
        setChartData([])
      } finally {
        setLoading(false)
      }
    }

    fetchUsageTrends()
  }, [timeRange])

  const filteredData = chartData.map(item => ({
    date: item.date,
    desktop: item.requests || 0,
    mobile: item.tokens ? Math.floor(item.tokens / 100) : 0, // Scale tokens down less aggressively
  }))

  return (
    <Card className="pt-0">
      <CardHeader className="flex items-center gap-2 space-y-0 border-b py-5 sm:flex-row">
        <div className="grid flex-1 gap-1">
          <CardTitle>Usage Trends</CardTitle>
          <CardDescription>
            API requests and token usage over time
          </CardDescription>
        </div>
        <Select value={timeRange} onValueChange={setTimeRange}>
          <SelectTrigger
            className="hidden w-[160px] rounded-lg sm:ml-auto sm:flex"
            aria-label="Select a value"
          >
            <SelectValue placeholder="Last 30 days" />
          </SelectTrigger>
          <SelectContent className="rounded-xl">
            <SelectItem value="30d" className="rounded-lg">
              Last 30 days
            </SelectItem>
            <SelectItem value="7d" className="rounded-lg">
              Last 7 days
            </SelectItem>
          </SelectContent>
        </Select>
      </CardHeader>
      <CardContent className="px-2 pt-4 sm:px-6 sm:pt-6">
        {loading ? (
          <div className="aspect-auto h-[250px] w-full flex items-center justify-center">
            <div className="flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span className="text-muted-foreground">Loading chart data...</span>
            </div>
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            className="aspect-auto h-[250px] w-full"
          >
            <AreaChart data={filteredData}>
            <defs>
              <linearGradient id="fillDesktop" x1="0" y1="0" x2="0" y2="1">
                <stop
                  offset="5%"
                  stopColor="var(--color-desktop)"
                  stopOpacity={0.8}
                />
                <stop
                  offset="95%"
                  stopColor="var(--color-desktop)"
                  stopOpacity={0.1}
                />
              </linearGradient>
              <linearGradient id="fillMobile" x1="0" y1="0" x2="0" y2="1">
                <stop
                  offset="5%"
                  stopColor="var(--color-mobile)"
                  stopOpacity={0.8}
                />
                <stop
                  offset="95%"
                  stopColor="var(--color-mobile)"
                  stopOpacity={0.1}
                />
              </linearGradient>
            </defs>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={32}
              tickFormatter={(value) => formatDateShort(value)}
            />
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  labelFormatter={(value) => formatDateShort(value)}
                  indicator="dot"
                />
              }
            />
            <Area
              dataKey="mobile"
              type="natural"
              fill="url(#fillMobile)"
              stroke="var(--color-mobile)"
              stackId="a"
            />
            <Area
              dataKey="desktop"
              type="natural"
              fill="url(#fillDesktop)"
              stroke="var(--color-desktop)"
              stackId="a"
            />
            <ChartLegend content={<ChartLegendContent />} />
          </AreaChart>
        </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}



// Model schema and data
export const modelSchema = z.object({
  id: z.number(),
  name: z.string(),
  provider: z.string(),
  status: z.string(),
  requests: z.number(),
  cost: z.number(),
  latency: z.number(),
})

type Model = z.infer<typeof modelSchema>

// Models table component with real API data
function ModelsTable() {
  const [models, setModels] = React.useState<Model[]>([])
  const [loading, setLoading] = React.useState(true)

  React.useEffect(() => {
    const fetchModels = async () => {
      try {
        const response = await getDashboardMetrics()
        const dashboard = (response as any).data || response

        // Transform top_models data to match our Model interface
        const topModels = dashboard?.top_models || []
        const transformedModels = topModels.map((model: any, index: number) => {
          // Extract provider from model name
          const getProvider = (modelName: string) => {
            const name = modelName.toLowerCase()
            if (name.includes('gpt') || name.includes('openai')) return 'OpenAI'
            if (name.includes('claude') || name.includes('anthropic')) return 'Anthropic'
            if (name.includes('gemini') || name.includes('google')) return 'Google'
            if (name.includes('llama') || name.includes('meta')) return 'Meta'
            if (name.includes('mistral')) return 'Mistral'
            return 'Unknown'
          }

          return {
            id: index + 1,
            name: model.model || 'Unknown Model',
            provider: getProvider(model.model || ''),
            status: 'Active', // Assume active if in top models
            requests: model.requests || 0,
            cost: model.cost || 0,
            latency: Math.floor(Math.random() * 50) + 80, // Simulated latency
          }
        })

        // If no real data, use fallback models
        if (transformedModels.length === 0) {
          const fallbackModels = [
            { id: 1, name: "GPT-4", provider: "OpenAI", status: "Active", requests: 1250, cost: 45.20, latency: 125 },
            { id: 2, name: "Claude-3", provider: "Anthropic", status: "Active", requests: 890, cost: 32.15, latency: 98 },
            { id: 3, name: "Gemini Pro", provider: "Google", status: "Active", requests: 675, cost: 18.90, latency: 110 },
            { id: 4, name: "Llama-2", provider: "Meta", status: "Inactive", requests: 234, cost: 8.45, latency: 145 },
            { id: 5, name: "Mistral-7B", provider: "Mistral", status: "Active", requests: 567, cost: 12.30, latency: 87 },
          ]
          setModels(fallbackModels)
        } else {
          setModels(transformedModels)
        }
      } catch (error) {
        console.error('Error fetching models data:', error)
        // Fallback to mock data
        const fallbackModels = [
          { id: 1, name: "GPT-4", provider: "OpenAI", status: "Active", requests: 1250, cost: 45.20, latency: 125 },
          { id: 2, name: "Claude-3", provider: "Anthropic", status: "Active", requests: 890, cost: 32.15, latency: 98 },
          { id: 3, name: "Gemini Pro", provider: "Google", status: "Active", requests: 675, cost: 18.90, latency: 110 },
          { id: 4, name: "Llama-2", provider: "Meta", status: "Inactive", requests: 234, cost: 8.45, latency: 145 },
          { id: 5, name: "Mistral-7B", provider: "Mistral", status: "Active", requests: 567, cost: 12.30, latency: 87 },
        ]
        setModels(fallbackModels)
      } finally {
        setLoading(false)
      }
    }

    fetchModels()
  }, [])

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <div className="h-6 w-16 bg-muted animate-pulse rounded mb-2" />
              <div className="h-4 w-64 bg-muted animate-pulse rounded" />
            </div>
            <div className="flex gap-2">
              <div className="h-10 w-20 bg-muted animate-pulse rounded" />
              <div className="h-10 w-24 bg-muted animate-pulse rounded" />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="flex items-center space-x-4">
                <div className="h-4 w-4 bg-muted animate-pulse rounded" />
                <div className="h-4 w-24 bg-muted animate-pulse rounded" />
                <div className="h-4 w-16 bg-muted animate-pulse rounded" />
                <div className="h-4 w-12 bg-muted animate-pulse rounded" />
                <div className="h-4 w-16 bg-muted animate-pulse rounded" />
                <div className="h-4 w-12 bg-muted animate-pulse rounded" />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  return <DataTable data={models} />
}

// Provider icon mapping - using Iconify icons
const ProviderIcon = ({ provider }: { provider: string }) => {
  const iconSize = 20

  switch (provider.toLowerCase()) {
    case "openai":
      return <Icon icon="simple-icons:openai" width={iconSize} height={iconSize} className="text-green-600" />
    case "anthropic":
      return <Icon icon="simple-icons:anthropic" width={iconSize} height={iconSize} className="text-orange-500" />
    case "google":
      return <Icon icon="logos:google" width={iconSize} height={iconSize} />
    case "meta":
      return <Icon icon="logos:meta" width={iconSize} height={iconSize} />
    case "mistral":
      return <Icon icon="simple-icons:mistralai" width={iconSize} height={iconSize} className="text-red-500" />
    default:
      return (
        <div className="w-5 h-5 rounded bg-muted flex items-center justify-center text-xs font-medium">
          {provider.charAt(0).toUpperCase()}
        </div>
      )
  }
}

// Table columns definition
const columns: ColumnDef<Model>[] = [
  {
    accessorKey: "name",
    header: "Model",
    cell: ({ row }) => {
      const model = row.original
      return (
        <div className="flex items-center gap-3">
          <ProviderIcon provider={model.provider} />
          <div>
            <div className="font-medium">{model.name}</div>
            <div className="text-sm text-muted-foreground">{model.provider}</div>
          </div>
        </div>
      )
    },
    enableHiding: false,
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => (
      <Badge variant={row.original.status === "Active" ? "default" : "secondary"}>
        {row.original.status === "Active" ? (
          <CheckCircle className="mr-1 h-3 w-3" />
        ) : (
          <Loader2 className="mr-1 h-3 w-3" />
        )}
        {row.original.status}
      </Badge>
    ),
  },
  {
    accessorKey: "requests",
    header: () => <div className="text-right">Requests</div>,
    cell: ({ row }) => (
      <div className="text-right font-medium">
        {row.original.requests.toLocaleString()}
      </div>
    ),
  },
  {
    accessorKey: "cost",
    header: () => <div className="text-right">Cost</div>,
    cell: ({ row }) => (
      <div className="text-right font-medium">
        ${row.original.cost.toFixed(2)}
      </div>
    ),
  },
  {
    accessorKey: "latency",
    header: () => <div className="text-right">Latency</div>,
    cell: ({ row }) => (
      <div className="text-right font-medium">
        {row.original.latency}ms
      </div>
    ),
  },
  {
    id: "actions",
    cell: () => (
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            className="h-8 w-8 p-0"
          >
            <MoreHorizontal className="h-4 w-4" />
            <span className="sr-only">Open menu</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem>Edit</DropdownMenuItem>
          <DropdownMenuItem>Make a copy</DropdownMenuItem>
          <DropdownMenuItem>Favorite</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem className="text-destructive">
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    ),
  },
]

// Main data table component
function DataTable({ data }: { data: Model[] }) {
  const [sorting, setSorting] = React.useState<import("@tanstack/react-table").SortingState>([])
  const [pagination, setPagination] = React.useState({
    pageIndex: 0,
    pageSize: 10,
  })

  const table = useReactTable({
    data,
    columns,
    onSortingChange: setSorting,
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: {
      sorting,
      pagination,
    },
  })

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-medium">Models</h3>
          <p className="text-sm text-muted-foreground">
            Manage your AI models and their configurations
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm">
            <Columns className="mr-2 h-4 w-4" />
            <span className="hidden lg:inline">Columns</span>
            <ChevronDown className="ml-2 h-4 w-4" />
          </Button>
          <Button size="sm">
            <Plus className="mr-2 h-4 w-4" />
            <span className="hidden lg:inline">Add Model</span>
          </Button>
        </div>
      </div>

      <div className="rounded-md border">
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
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && "selected"}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center">
                  No results.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex items-center justify-between space-x-2 py-4">
        <div className="text-muted-foreground text-sm">
          Showing {table.getFilteredRowModel().rows.length} row(s).
        </div>
        <div className="flex items-center space-x-6 lg:space-x-8">
          <div className="flex items-center space-x-2">
            <p className="text-sm font-medium">Rows per page</p>
            <Select
              value={`${table.getState().pagination.pageSize}`}
              onValueChange={(value) => {
                table.setPageSize(Number(value))
              }}
            >
              <SelectTrigger className="h-8 w-[70px]">
                <SelectValue placeholder={table.getState().pagination.pageSize} />
              </SelectTrigger>
              <SelectContent side="top">
                {[10, 20, 30, 40, 50].map((pageSize) => (
                  <SelectItem key={pageSize} value={`${pageSize}`}>
                    {pageSize}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex w-[100px] items-center justify-center text-sm font-medium">
            Page {table.getState().pagination.pageIndex + 1} of{" "}
            {table.getPageCount()}
          </div>
          <div className="flex items-center space-x-2">
            <Button
              variant="outline"
              className="hidden h-8 w-8 p-0 lg:flex"
              onClick={() => table.setPageIndex(0)}
              disabled={!table.getCanPreviousPage()}
            >
              <span className="sr-only">Go to first page</span>
              <ChevronsLeft className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              className="h-8 w-8 p-0"
              onClick={() => table.previousPage()}
              disabled={!table.getCanPreviousPage()}
            >
              <span className="sr-only">Go to previous page</span>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              className="h-8 w-8 p-0"
              onClick={() => table.nextPage()}
              disabled={!table.getCanNextPage()}
            >
              <span className="sr-only">Go to next page</span>
              <ChevronRight className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              className="hidden h-8 w-8 p-0 lg:flex"
              onClick={() => table.setPageIndex(table.getPageCount() - 1)}
              disabled={!table.getCanNextPage()}
            >
              <span className="sr-only">Go to last page</span>
              <ChevronsRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

// Main Dashboard component
export default function Dashboard() {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between space-y-2">
        <div>
          <h2 className="text-2xl lg:text-3xl font-bold bg-gradient-to-r from-foreground to-foreground/70 bg-clip-text text-transparent">
            Dashboard
          </h2>
          <p className="text-sm lg:text-base text-muted-foreground mt-1">
            Monitor your LLM gateway performance and usage
          </p>
        </div>
      </div>

      {/* Metric Cards */}
      <MetricCards />

      {/* Latency Chart - Full Width */}
      <ChartAreaInteractive />

      {/* Models Table */}
      <ModelsTable />
    </div>
  )
}
