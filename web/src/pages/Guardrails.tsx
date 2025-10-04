import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { Shield, Activity, Clock, AlertTriangle, CheckCircle, XCircle, Plus, Settings, Trash2, Edit3, MoreHorizontal, ArrowUpDown } from "lucide-react";
import { Bar, BarChart } from "recharts";
import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  SortingState,
  useReactTable,
} from "@tanstack/react-table";

import { getGuardrails, getGuardrailStats, checkGuardrailHealth } from "@/lib/api";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ChartConfig, ChartContainer } from "@/components/ui/chart";


interface GuardrailInfo {
  name: string;
  provider: string;
  mode: string[];
  enabled: boolean;
  default_on: boolean;
  config: Record<string, any>;
  stats?: {
    total_executions: number;
    total_passed: number;
    total_blocked: number;
    total_errors: number;
    average_latency: number;
    last_executed: string;
    block_rate: number;
    error_rate: number;
  };
  healthy: boolean;
}



// Microsoft icon component for Presidio
const MicrosoftIcon = () => (
  <svg viewBox="0 0 24 24" className="w-5 h-5">
    <path d="M11.4 24H0V12.6h11.4V24zM24 24H12.6V12.6H24V24zM11.4 11.4H0V0h11.4v11.4zM24 11.4H12.6V0H24v11.4z" fill="#00BCF2"/>
  </svg>
);

const PII_ENTITIES = [
  { value: "PERSON", label: "Person Names" },
  { value: "EMAIL_ADDRESS", label: "Email Addresses" },
  { value: "PHONE_NUMBER", label: "Phone Numbers" },
  { value: "CREDIT_CARD", label: "Credit Cards" },
  { value: "SSN", label: "Social Security Numbers" },
  { value: "IP_ADDRESS", label: "IP Addresses" },
  { value: "US_DRIVER_LICENSE", label: "US Driver Licenses" },
  { value: "US_PASSPORT", label: "US Passports" },
  { value: "US_BANK_NUMBER", label: "US Bank Numbers" }
];

// Microchart component for performance visualization
const PerformanceMicroChart = ({ stats }: { stats: any }) => {
  if (!stats) return <div className="w-20 h-8 bg-muted rounded" />;
  
  const chartData = [
    { name: "Passed", value: stats.total_passed, fill: "#22c55e" },
    { name: "Blocked", value: stats.total_blocked, fill: "#ef4444" },
    { name: "Errors", value: stats.total_errors, fill: "#f59e0b" },
  ];
  
  const chartConfig = {
    value: {
      label: "Count",
    },
  } satisfies ChartConfig;
  
  return (
    <ChartContainer config={chartConfig} className="h-8 w-20">
      <BarChart data={chartData}>
        <Bar dataKey="value" radius={1} />
      </BarChart>
    </ChartContainer>
  );
};

// Latency trend microchart
const LatencyMicroChart = ({ latency }: { latency: number }) => {
  // Generate sample trend data (in real app, this would come from API)
  const trendData = Array.from({ length: 7 }, (_, i) => ({
    day: i,
    latency: latency + (Math.random() - 0.5) * 20,
  }));
  
  const chartConfig = {
    latency: {
      label: "Latency",
      color: "#3b82f6",
    },
  } satisfies ChartConfig;
  
  return (
    <ChartContainer config={chartConfig} className="h-8 w-20">
      <BarChart data={trendData}>
        <Bar dataKey="latency" fill="var(--color-latency)" radius={1} />
      </BarChart>
    </ChartContainer>
  );
};

// Define table columns
const createColumns = (
  onConfigure: (guardrail: GuardrailInfo) => void,
  onEdit: (guardrail: GuardrailInfo) => void,
  onDelete: (guardrail: GuardrailInfo) => void,
  formatLatency: (ms: number) => string,
  formatRate: (rate: number) => string,
  getModeColor: (mode: string) => string
): ColumnDef<GuardrailInfo>[] => [
  {
    accessorKey: "name",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          className="h-auto p-0 font-semibold"
        >
          Name
          <ArrowUpDown className="ml-2 h-4 w-4" />
        </Button>
      );
    },
    cell: ({ row }) => {
      const guardrail = row.original;
      return (
        <div className="flex items-center gap-3">
          {guardrail.provider === 'presidio' ? (
            <div className="p-2 bg-blue-50 rounded-lg">
              <MicrosoftIcon />
            </div>
          ) : (
            <div className="p-2 bg-gray-50 rounded-lg">
              <Shield className="h-5 w-5 text-gray-600" />
            </div>
          )}
          <div>
            <div className="font-semibold flex items-center gap-2">
              {guardrail.name}
              {guardrail.healthy ? (
                <CheckCircle className="h-4 w-4 text-green-500" />
              ) : (
                <XCircle className="h-4 w-4 text-red-500" />
              )}
            </div>
            <div className="text-sm text-muted-foreground">
              {guardrail.provider === 'presidio' ? 'Microsoft Presidio' : guardrail.provider}
            </div>
          </div>
        </div>
      );
    },
  },
  {
    accessorKey: "enabled",
    header: "Status",
    cell: ({ row }) => {
      const guardrail = row.original;
      return (
        <div className="flex flex-col gap-1">
          <Badge variant={guardrail.enabled ? "default" : "secondary"}>
            {guardrail.enabled ? "Enabled" : "Disabled"}
          </Badge>
          {guardrail.default_on && (
            <Badge variant="outline" className="text-xs">
              Default On
            </Badge>
          )}
        </div>
      );
    },
  },
  {
    accessorKey: "mode",
    header: "Execution Modes",
    cell: ({ row }) => {
      const modes = row.original.mode;
      return (
        <div className="flex flex-wrap gap-1">
          {modes.map((mode: string) => (
            <Badge key={mode} className={`${getModeColor(mode)} text-xs`}>
              {mode.replace('_', ' ')}
            </Badge>
          ))}
        </div>
      );
    },
  },
  {
    accessorKey: "stats",
    header: "Performance",
    cell: ({ row }) => {
      const stats = row.original.stats;
      return (
        <div className="flex items-center gap-3">
          <PerformanceMicroChart stats={stats} />
          {stats && (
            <div className="text-xs text-muted-foreground">
              <div>{stats.total_executions.toLocaleString()} exec</div>
              <div>{formatRate(stats.block_rate)} blocked</div>
            </div>
          )}
        </div>
      );
    },
  },
  {
    accessorKey: "average_latency",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          className="h-auto p-0 font-semibold"
        >
          Latency
          <ArrowUpDown className="ml-2 h-4 w-4" />
        </Button>
      );
    },
    cell: ({ row }) => {
      const stats = row.original.stats;
      return (
        <div className="flex items-center gap-3">
          {stats && <LatencyMicroChart latency={stats.average_latency} />}
          <div className="text-sm font-medium">
            {stats ? formatLatency(stats.average_latency) : 'N/A'}
          </div>
        </div>
      );
    },
  },
  {
    accessorKey: "config",
    header: "Configuration",
    cell: ({ row }) => {
      const config = row.original.config;
      return (
        <div className="text-sm">
          {config?.threshold && (
            <div>Threshold: {config.threshold}</div>
          )}
          {config?.entities && (
            <div className="text-muted-foreground">
              {config.entities.length} PII types
            </div>
          )}
        </div>
      );
    },
  },
  {
    id: "actions",
    header: "Actions",
    cell: ({ row }) => {
      const guardrail = row.original;
      return (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" className="h-8 w-8 p-0">
              <span className="sr-only">Open menu</span>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Actions</DropdownMenuLabel>
            <DropdownMenuItem onClick={() => onConfigure(guardrail)}>
              <Settings className="mr-2 h-4 w-4" />
              Configure
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => onEdit(guardrail)}>
              <Edit3 className="mr-2 h-4 w-4" />
              Edit
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem 
              onClick={() => onDelete(guardrail)}
              className="text-red-600"
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      );
    },
  },
];

// Guardrails Table Component
function GuardrailsTable({ 
  guardrails, 
  onConfigure, 
  onEdit, 
  onDelete, 
  formatLatency, 
  formatRate, 
  getModeColor 
}: {
  guardrails: GuardrailInfo[];
  onConfigure: (guardrail: GuardrailInfo) => void;
  onEdit: (guardrail: GuardrailInfo) => void;
  onDelete: (guardrail: GuardrailInfo) => void;
  formatLatency: (ms: number) => string;
  formatRate: (rate: number) => string;
  getModeColor: (mode: string) => string;
}) {
  const [sorting, setSorting] = useState<SortingState>([]);
  
  const columns = createColumns(onConfigure, onEdit, onDelete, formatLatency, formatRate, getModeColor);
  
  const table = useReactTable({
    data: guardrails,
    columns,
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: {
      sorting,
    },
  });
  
  if (guardrails.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          <Shield className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-semibold mb-2">No Guardrails Configured</h3>
          <p className="text-muted-foreground text-center max-w-md">
            No guardrails are currently configured. Add guardrails to your configuration to start protecting your LLM interactions.
          </p>
        </CardContent>
      </Card>
    );
  }
  
  return (
    <div className="space-y-4">
      <div className="overflow-hidden rounded-md border">
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
                  );
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(
                      cell.column.columnDef.cell,
                      cell.getContext()
                    )}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}




export default function Guardrails() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState("overview");
  const [selectedGuardrail, setSelectedGuardrail] = useState<GuardrailInfo | null>(null);
  
  const handleConfigure = (guardrail: GuardrailInfo) => {
    setSelectedGuardrail(guardrail);
  };
  
  const handleEdit = (guardrail: GuardrailInfo) => {
    // TODO: Implement edit functionality
    console.log('Edit guardrail:', guardrail.name);
  };
  
  const handleDelete = (guardrail: GuardrailInfo) => {
    // TODO: Implement delete functionality
    console.log('Delete guardrail:', guardrail.name);
  };

  const { data: guardrailsData, isLoading } = useQuery({
    queryKey: ["guardrails"],
    queryFn: getGuardrails,
  });

  const { data: statsData } = useQuery({
    queryKey: ["guardrails-stats"],
    queryFn: getGuardrailStats,
  });

  const { data: healthData } = useQuery({
    queryKey: ["guardrails-health"],
    queryFn: checkGuardrailHealth,
    refetchInterval: 30000, // Check health every 30 seconds
  });

  const guardrails = (guardrailsData as any)?.guardrails || [];
  const systemEnabled = (guardrailsData as any)?.enabled ?? false;
  const stats = (statsData as any)?.stats || {};
  const health = (healthData as any)?.health || {};
  const allHealthy = (healthData as any)?.all_healthy ?? false;
  const checkedAt = (healthData as any)?.checked_at;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  const formatLatency = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  };

  const formatRate = (rate: number) => {
    return `${(rate * 100).toFixed(1)}%`;
  };

  const getHealthIcon = (healthy: boolean) => {
    return healthy ? (
      <CheckCircle className="h-4 w-4 text-green-500" />
    ) : (
      <XCircle className="h-4 w-4 text-red-500" />
    );
  };

  const getModeColor = (mode: string) => {
    switch (mode) {
      case "pre_call":
        return "bg-blue-100 text-blue-800";
      case "post_call":
        return "bg-green-100 text-green-800";
      case "during_call":
        return "bg-yellow-100 text-yellow-800";
      case "logging_only":
        return "bg-gray-100 text-gray-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  };

  return (
    <div className="space-y-4 lg:space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">Guardrails</h1>
          <p className="text-sm lg:text-base text-muted-foreground">
            Monitor and manage content safety guardrails
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={systemEnabled ? "default" : "secondary"}>
            {systemEnabled ? "Enabled" : "Disabled"}
          </Badge>
          <Button 
            variant="outline" 
            onClick={() => navigate("/guardrails/marketplace")}
          >
            <Shield className="h-4 w-4 mr-2" />
            Browse Marketplace
          </Button>
          <Button 
            onClick={() => navigate("/guardrails/config/new")}
          >
            <Plus className="h-4 w-4 mr-2" />
            Add Guardrail
          </Button>
        </div>
      </div>

      {/* System Status Alert */}
      {!systemEnabled && (
        <Card className="border-yellow-200 bg-yellow-50">
          <CardContent className="pt-6">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 text-yellow-600" />
              <p className="text-sm text-yellow-800">
                Guardrails system is currently disabled. Enable it in the configuration to start protecting your LLM interactions.
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="statistics">Statistics</TabsTrigger>
          <TabsTrigger value="health">Health</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <GuardrailsTable 
            guardrails={guardrails}
            onConfigure={handleConfigure}
            onEdit={handleEdit}
            onDelete={handleDelete}
            formatLatency={formatLatency}
            formatRate={formatRate}
            getModeColor={getModeColor}
          />
        </TabsContent>

        <TabsContent value="statistics" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Activity className="h-5 w-5" />
                Execution Statistics
              </CardTitle>
              <CardDescription>
                Performance metrics for all guardrails
              </CardDescription>
            </CardHeader>
            <CardContent>
              {!stats || Object.keys(stats).length === 0 ? (
                <div className="text-center text-muted-foreground py-8">
                  No statistics available yet
                </div>
              ) : (
                <div className="space-y-6">
                  {Object.entries(stats as Record<string, any>).map(([name, stats]) => (
                    <div key={name} className="border-b pb-4 last:border-b-0">
                      <h4 className="font-medium mb-3">{name}</h4>
                      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
                        <div className="text-center">
                          <div className="text-2xl font-bold text-blue-600">{stats.total_executions.toLocaleString()}</div>
                          <div className="text-muted-foreground">Total Executions</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold text-green-600">{stats.total_passed.toLocaleString()}</div>
                          <div className="text-muted-foreground">Passed</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold text-red-600">{stats.total_blocked.toLocaleString()}</div>
                          <div className="text-muted-foreground">Blocked</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold text-yellow-600">{stats.total_errors.toLocaleString()}</div>
                          <div className="text-muted-foreground">Errors</div>
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-4 mt-4 text-sm">
                        <div className="text-center">
                          <div className="text-xl font-semibold">{formatRate(stats.block_rate)}</div>
                          <div className="text-muted-foreground">Block Rate</div>
                        </div>
                        <div className="text-center">
                          <div className="text-xl font-semibold">{formatLatency(stats.average_latency)}</div>
                          <div className="text-muted-foreground">Avg Latency</div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="health" className="space-y-4">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center gap-2">
                    <Activity className="h-5 w-5" />
                    Health Status
                  </CardTitle>
                  <CardDescription>
                    Real-time health monitoring for all guardrails
                  </CardDescription>
                </div>
                <Button variant="outline" onClick={() => window.location.reload()}>
                  Refresh
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {!health || Object.keys(health).length === 0 ? (
                <div className="text-center text-muted-foreground py-8">
                  Health check unavailable
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="flex items-center gap-2 mb-4">
                    {allHealthy ? (
                      <CheckCircle className="h-5 w-5 text-green-500" />
                    ) : (
                      <XCircle className="h-5 w-5 text-red-500" />
                    )}
                    <span className={`font-medium ${allHealthy ? 'text-green-700' : 'text-red-700'}`}>
                      {allHealthy ? 'All Guardrails Healthy' : 'Some Guardrails Unhealthy'}
                    </span>
                  </div>
                  
                  <div className="space-y-3">
                    {Object.entries(health as Record<string, any>).map(([name, status]) => (
                      <div key={name} className="flex items-center justify-between p-3 border rounded-md">
                        <div className="flex items-center gap-2">
                          {getHealthIcon(status.healthy)}
                          <span className="font-medium">{name}</span>
                        </div>
                        <div className="text-sm text-muted-foreground">
                          {status.healthy ? 'Healthy' : status.error}
                        </div>
                      </div>
                    ))}
                  </div>
                  
                  {checkedAt && (
                    <div className="flex items-center gap-1 text-xs text-muted-foreground mt-4">
                      <Clock className="h-3 w-3" />
                      Last checked: {new Date(checkedAt).toLocaleString()}
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Guardrail Configuration Sheet */}
      <Sheet open={!!selectedGuardrail} onOpenChange={() => setSelectedGuardrail(null)}>
        <SheetContent className="sm:max-w-2xl overflow-y-auto">
          {selectedGuardrail && (
            <>
              <SheetHeader>
                <div className="flex items-center gap-3">
                  {selectedGuardrail.provider === 'presidio' ? (
                    <div className="p-2 bg-blue-50 rounded-lg">
                      <MicrosoftIcon />
                    </div>
                  ) : (
                    <div className="p-2 bg-gray-50 rounded-lg">
                      <Shield className="h-5 w-5 text-gray-600" />
                    </div>
                  )}
                  <div>
                    <SheetTitle>{selectedGuardrail.name}</SheetTitle>
                    <SheetDescription>
                      {selectedGuardrail.provider === 'presidio' ? 'Microsoft Presidio' : selectedGuardrail.provider} Configuration
                    </SheetDescription>
                  </div>
                </div>
              </SheetHeader>

              <div className="space-y-6 mt-6">
                {/* Status & Basic Info */}
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm font-medium">Status</label>
                    <div className="flex items-center gap-2">
                      <Badge variant={selectedGuardrail.enabled ? "default" : "secondary"}>
                        {selectedGuardrail.enabled ? "Enabled" : "Disabled"}
                      </Badge>
                      {getHealthIcon(selectedGuardrail.healthy)}
                      <span className="text-sm text-muted-foreground">
                        {selectedGuardrail.healthy ? 'Healthy' : 'Unhealthy'}
                      </span>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium">Provider</label>
                    <div className="flex items-center gap-2">
                      {selectedGuardrail.provider === 'presidio' ? (
                        <>
                          <MicrosoftIcon />
                          <span className="text-sm">Microsoft Presidio</span>
                        </>
                      ) : (
                        <span className="text-sm">{selectedGuardrail.provider}</span>
                      )}
                    </div>
                  </div>
                </div>

                <Separator />

                {/* Execution Modes */}
                <div className="space-y-3">
                  <label className="text-sm font-medium">Execution Modes</label>
                  <div className="grid grid-cols-2 gap-2">
                    {selectedGuardrail.mode.map((mode: string) => (
                      <Badge key={mode} className={getModeColor(mode)}>
                        {mode.replace('_', ' ')}
                      </Badge>
                    ))}
                  </div>
                </div>

                <Separator />

                {/* Configuration Details */}
                <div className="space-y-4">
                  <label className="text-sm font-medium">Configuration Details</label>
                  {selectedGuardrail.config && (
                    <div className="space-y-4">
                      {/* Service URLs */}
                      {selectedGuardrail.config.analyzer_url && (
                        <div className="grid grid-cols-3 gap-2 text-sm">
                          <span className="text-muted-foreground">Analyzer URL:</span>
                          <span className="col-span-2 font-mono text-xs bg-muted p-2 rounded">
                            {selectedGuardrail.config.analyzer_url}
                          </span>
                        </div>
                      )}
                      {selectedGuardrail.config.anonymizer_url && (
                        <div className="grid grid-cols-3 gap-2 text-sm">
                          <span className="text-muted-foreground">Anonymizer URL:</span>
                          <span className="col-span-2 font-mono text-xs bg-muted p-2 rounded">
                            {selectedGuardrail.config.anonymizer_url}
                          </span>
                        </div>
                      )}

                      {/* Detection Settings */}
                      <div className="grid grid-cols-2 gap-4">
                        {selectedGuardrail.config.threshold && (
                          <div>
                            <span className="text-sm text-muted-foreground">Threshold</span>
                            <div className="text-lg font-semibold">{selectedGuardrail.config.threshold}</div>
                          </div>
                        )}
                        {selectedGuardrail.config.language && (
                          <div>
                            <span className="text-sm text-muted-foreground">Language</span>
                            <div className="text-lg font-semibold uppercase">{selectedGuardrail.config.language}</div>
                          </div>
                        )}
                      </div>

                      {/* Anonymization Settings */}
                      <div className="grid grid-cols-2 gap-4">
                        {selectedGuardrail.config.anonymize_method && (
                          <div>
                            <span className="text-sm text-muted-foreground">Anonymization Method</span>
                            <div className="text-lg font-semibold capitalize">{selectedGuardrail.config.anonymize_method}</div>
                          </div>
                        )}
                        {selectedGuardrail.config.mask_pii !== undefined && (
                          <div>
                            <span className="text-sm text-muted-foreground">Mask PII</span>
                            <div className="text-lg font-semibold">{selectedGuardrail.config.mask_pii ? 'Yes' : 'No'}</div>
                          </div>
                        )}
                      </div>

                      {/* PII Entities */}
                      {selectedGuardrail.config.entities && (
                        <div className="space-y-2">
                          <span className="text-sm text-muted-foreground">Monitored PII Types</span>
                          <div className="flex flex-wrap gap-2">
                            {selectedGuardrail.config.entities.map((entity: string) => (
                              <Badge key={entity} variant="outline" className="text-xs">
                                {PII_ENTITIES.find(e => e.value === entity)?.label || entity}
                              </Badge>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* Custom Anonymizers */}
                      {selectedGuardrail.config.anonymizers && (
                        <div className="space-y-2">
                          <span className="text-sm text-muted-foreground">Custom Anonymization Rules</span>
                          <div className="space-y-2">
                            {Object.entries(selectedGuardrail.config.anonymizers).map(([entity, method]) => (
                              <div key={entity} className="flex justify-between items-center text-sm bg-muted p-2 rounded">
                                <span className="font-medium">{entity}</span>
                                <Badge variant="secondary">{method as string}</Badge>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                <Separator />

                {/* Performance Stats */}
                {selectedGuardrail.stats && (
                  <div className="space-y-4">
                    <label className="text-sm font-medium">Performance Statistics</label>
                    <div className="grid grid-cols-2 gap-4">
                      <div className="text-center p-3 bg-blue-50 rounded-lg">
                        <div className="text-2xl font-bold text-blue-600">
                          {selectedGuardrail.stats.total_executions.toLocaleString()}
                        </div>
                        <div className="text-xs text-blue-600">Total Executions</div>
                      </div>
                      <div className="text-center p-3 bg-green-50 rounded-lg">
                        <div className="text-2xl font-bold text-green-600">
                          {formatLatency(selectedGuardrail.stats.average_latency)}
                        </div>
                        <div className="text-xs text-green-600">Avg Latency</div>
                      </div>
                      <div className="text-center p-3 bg-red-50 rounded-lg">
                        <div className="text-2xl font-bold text-red-600">
                          {formatRate(selectedGuardrail.stats.block_rate)}
                        </div>
                        <div className="text-xs text-red-600">Block Rate</div>
                      </div>
                      <div className="text-center p-3 bg-yellow-50 rounded-lg">
                        <div className="text-2xl font-bold text-yellow-600">
                          {formatRate(selectedGuardrail.stats.error_rate)}
                        </div>
                        <div className="text-xs text-yellow-600">Error Rate</div>
                      </div>
                    </div>
                    {selectedGuardrail.stats.last_executed && (
                      <div className="text-xs text-muted-foreground text-center">
                        Last executed: {new Date(selectedGuardrail.stats.last_executed).toLocaleString()}
                      </div>
                    )}
                  </div>
                )}

                <div className="flex gap-2 pt-4">
                  <Button variant="outline" className="flex-1">
                    <Edit3 className="h-4 w-4 mr-2" />
                    Edit Configuration
                  </Button>
                  <Button variant="outline" className="flex-1">
                    <Settings className="h-4 w-4 mr-2" />
                    Test Guardrail
                  </Button>
                </div>
              </div>
            </>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}