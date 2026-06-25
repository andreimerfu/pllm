import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
import { Bar, BarChart } from "recharts";
import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  SortingState,
  useReactTable,
} from "@tanstack/react-table";

import { formatDateTime } from "@/lib/date-utils";
import { useGuardrails, formatLatency, formatRate, GuardrailInfo } from "@/hooks/useGuardrails";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
import { PageShell, StatTile } from "@/components/common/page-shell";
import { StatusChip, Tag, type StatusTone } from "@/components/common/status-chip";

// GuardrailInfo interface now imported from hooks/useGuardrails

// Token-based tone for an execution mode. Replaces the legacy raw-palette
// getModeColor helper — color is reserved for meaning.
const modeTone = (mode: string): StatusTone => {
  switch (mode) {
    case 'pre_call':
      return 'info';
    case 'post_call':
      return 'good';
    case 'during_call':
      return 'warn';
    default:
      return 'neutral';
  }
};

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

// Type-tinted icon tile for a guardrail (Presidio brand mark or shield).
const GuardrailIconTile = ({ provider }: { provider: string }) =>
  provider === 'presidio' ? (
    <span className="inline-flex h-[34px] w-[34px] items-center justify-center rounded-[9px] bg-info/12">
      <MicrosoftIcon />
    </span>
  ) : (
    <span className="inline-flex h-[34px] w-[34px] items-center justify-center rounded-[9px] bg-kind-skill/12 text-kind-skill">
      <Icon icon={icons.shield} className="h-[18px] w-[18px]" />
    </span>
  );

// Microchart component for performance visualization
const PerformanceMicroChart = ({ stats }: { stats: any }) => {
  if (!stats) return <div className="w-20 h-8 bg-surface-2 rounded" />;

  const chartData = [
    { name: "Passed", value: stats.total_passed, fill: "hsl(var(--good))" },
    { name: "Blocked", value: stats.total_blocked, fill: "hsl(var(--bad))" },
    { name: "Errors", value: stats.total_errors, fill: "hsl(var(--warn))" },
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
      color: "hsl(var(--info))",
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
  formatRate: (rate: number) => string
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
          <Icon icon={icons.arrowUpDown} className="ml-2 h-4 w-4" />
        </Button>
      );
    },
    cell: ({ row }) => {
      const guardrail = row.original;
      return (
        <div className="flex items-center gap-3">
          <GuardrailIconTile provider={guardrail.provider} />
          <div>
            <div className="font-semibold flex items-center gap-2">
              {guardrail.name}
              {guardrail.healthy ? (
                <Icon icon={icons.check} className="h-4 w-4 text-good" />
              ) : (
                <Icon icon={icons.error} className="h-4 w-4 text-bad" />
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
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusChip
            tone={guardrail.enabled ? "good" : "neutral"}
            label={guardrail.enabled ? "Enabled" : "Disabled"}
          />
          {guardrail.default_on && <Tag label="Default on" />}
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
        <div className="flex flex-wrap gap-1.5">
          {modes.map((mode: string) => (
            <StatusChip key={mode} tone={modeTone(mode)} dot={false} label={mode.replace('_', ' ')} />
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
          <Icon icon={icons.arrowUpDown} className="ml-2 h-4 w-4" />
        </Button>
      );
    },
    cell: ({ row }) => {
      const stats = row.original.stats;
      return (
        <div className="flex items-center gap-3">
          {stats && <LatencyMicroChart latency={stats.average_latency} />}
          <div className="text-sm font-medium font-mono tabular-nums">
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
              <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Actions</DropdownMenuLabel>
            <DropdownMenuItem onClick={() => onConfigure(guardrail)}>
              <Icon icon={icons.settings} className="mr-2 h-4 w-4" />
              Configure
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => onEdit(guardrail)}>
              <Icon icon={icons.edit} className="mr-2 h-4 w-4" />
              Edit
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => onDelete(guardrail)}
              className="text-bad"
            >
              <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
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
}: {
  guardrails: GuardrailInfo[];
  onConfigure: (guardrail: GuardrailInfo) => void;
  onEdit: (guardrail: GuardrailInfo) => void;
  onDelete: (guardrail: GuardrailInfo) => void;
  formatLatency: (ms: number) => string;
  formatRate: (rate: number) => string;
}) {
  const [sorting, setSorting] = useState<SortingState>([]);

  const columns = createColumns(onConfigure, onEdit, onDelete, formatLatency, formatRate);

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
      <div className="flex flex-col items-center justify-center rounded-[14px] border border-border bg-card py-12 shadow-sm">
        <span className="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-[12px] bg-kind-skill/12 text-kind-skill">
          <Icon icon={icons.shield} className="h-6 w-6" />
        </span>
        <h3 className="text-lg font-semibold mb-2">No Guardrails Configured</h3>
        <p className="text-muted-foreground text-center max-w-md">
          No guardrails are currently configured. Add guardrails to your configuration to start protecting your LLM interactions.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="overflow-hidden rounded-[14px] border border-border bg-card shadow-sm">
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

  // Use the extracted hook for data management
  const {
    guardrails,
    systemEnabled,
    stats,
    health,
    allHealthy,
    checkedAt,
    isLoading,
    refetch,
  } = useGuardrails()

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

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  // ── KPI values, computed only from data already present on the rows ──
  const totalGuardrails = guardrails.length;
  const enabledCount = guardrails.filter((g) => g.enabled).length;
  const rowsWithStats = guardrails.filter((g) => g.stats);
  const totalBlocked = rowsWithStats.reduce((sum, g) => sum + (g.stats?.total_blocked ?? 0), 0);
  const latencyRows = rowsWithStats.filter((g) => typeof g.stats?.average_latency === "number");
  const avgLatency =
    latencyRows.length > 0
      ? latencyRows.reduce((sum, g) => sum + (g.stats?.average_latency ?? 0), 0) / latencyRows.length
      : null;

  const getHealthIcon = (healthy?: boolean) => {
    if (healthy === undefined) return null;
    return healthy ? (
      <Icon icon={icons.check} className="h-4 w-4 text-good" />
    ) : (
      <Icon icon={icons.error} className="h-4 w-4 text-bad" />
    );
  };

  return (
    <PageShell
      title="Guardrails"
      description="Safety policies — PII redaction, moderation, prompt-injection screens — applied on every request and response."
      actions={
        <>
          <StatusChip
            tone={systemEnabled ? "good" : "neutral"}
            label={systemEnabled ? "Enabled" : "Disabled"}
          />
          <Button variant="outline" onClick={() => navigate("/guardrails/marketplace")}>
            <Icon icon={icons.shield} className="h-4 w-4 mr-2" />
            Browse Marketplace
          </Button>
          <Button onClick={() => navigate("/guardrails/config/new")}>
            <Icon icon={icons.plus} className="h-4 w-4 mr-2" />
            Add Guardrail
          </Button>
        </>
      }
    >
      {/* ────────────── KPI cards ────────────── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        <StatTile
          label="Active guardrails"
          value={enabledCount}
          tone={enabledCount > 0 ? "good" : "default"}
          icon={<Icon icon={icons.shield} />}
          iconColor="good"
          sub={`of ${totalGuardrails} configured`}
        />
        <StatTile
          label="Configured"
          value={totalGuardrails}
          icon={<Icon icon={icons.guardrails} />}
          iconColor="skill"
          sub="Total policies"
        />
        <StatTile
          label="Violations blocked"
          value={totalBlocked.toLocaleString()}
          tone={totalBlocked > 0 ? "warn" : "default"}
          icon={<Icon icon={icons.shieldAlert} />}
          iconColor="info"
          sub="Across all guardrails"
        />
        {avgLatency !== null && (
          <StatTile
            label="Avg added latency"
            value={formatLatency(Math.round(avgLatency))}
            tone="warn"
            icon={<Icon icon={icons.clock} />}
            iconColor="warn"
            sub="Mean over active policies"
          />
        )}
      </div>

      {/* System Status Alert */}
      {!systemEnabled && (
        <div className="rounded-[14px] border border-warn/30 bg-warn/12 p-4 shadow-sm">
          <div className="flex items-center gap-2">
            <Icon icon={icons.warning} className="h-4 w-4 text-warn" />
            <p className="text-sm text-warn">
              Guardrails system is currently disabled. Enable it in the configuration to start protecting your LLM interactions.
            </p>
          </div>
        </div>
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
          />
        </TabsContent>

        <TabsContent value="statistics" className="space-y-4">
          <Card className="rounded-[14px] shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Icon icon={icons.trendingUp} className="h-5 w-5" />
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
                    <div key={name} className="border-b border-border pb-4 last:border-b-0">
                      <h4 className="font-medium mb-3">{name}</h4>
                      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
                        <div className="text-center">
                          <div className="text-2xl font-bold font-mono tabular-nums text-info">{stats.total_executions.toLocaleString()}</div>
                          <div className="text-muted-foreground">Total Executions</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold font-mono tabular-nums text-good">{stats.total_passed.toLocaleString()}</div>
                          <div className="text-muted-foreground">Passed</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold font-mono tabular-nums text-bad">{stats.total_blocked.toLocaleString()}</div>
                          <div className="text-muted-foreground">Blocked</div>
                        </div>
                        <div className="text-center">
                          <div className="text-2xl font-bold font-mono tabular-nums text-warn">{stats.total_errors.toLocaleString()}</div>
                          <div className="text-muted-foreground">Errors</div>
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-4 mt-4 text-sm">
                        <div className="text-center">
                          <div className="text-xl font-semibold font-mono tabular-nums">{formatRate(stats.block_rate)}</div>
                          <div className="text-muted-foreground">Block Rate</div>
                        </div>
                        <div className="text-center">
                          <div className="text-xl font-semibold font-mono tabular-nums">{formatLatency(stats.average_latency)}</div>
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
          <Card className="rounded-[14px] shadow-sm">
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle className="flex items-center gap-2">
                    <Icon icon={icons.trendingUp} className="h-5 w-5" />
                    Health Status
                  </CardTitle>
                  <CardDescription>
                    Real-time health monitoring for all guardrails
                  </CardDescription>
                </div>
                <Button variant="outline" onClick={() => refetch()}>
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
                    <StatusChip
                      tone={allHealthy ? "good" : "bad"}
                      label={allHealthy ? "All Guardrails Healthy" : "Some Guardrails Unhealthy"}
                    />
                  </div>

                  <div className="space-y-3">
                    {Object.entries(health as Record<string, any>).map(([name, status]) => (
                      <div key={name} className="flex items-center justify-between rounded-[12px] border border-border bg-card p-3 shadow-sm">
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
                      <Icon icon={icons.clock} className="h-3 w-3" />
                      Last checked: {formatDateTime(checkedAt)}
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
                  <GuardrailIconTile provider={selectedGuardrail.provider} />
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
                      <StatusChip
                        tone={selectedGuardrail.enabled ? "good" : "neutral"}
                        label={selectedGuardrail.enabled ? "Enabled" : "Disabled"}
                      />
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
                  <div className="flex flex-wrap gap-2">
                    {selectedGuardrail.mode.map((mode: string) => (
                      <StatusChip key={mode} tone={modeTone(mode)} dot={false} label={mode.replace('_', ' ')} />
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
                          <span className="col-span-2 font-mono text-xs bg-surface-2 p-2 rounded">
                            {selectedGuardrail.config.analyzer_url}
                          </span>
                        </div>
                      )}
                      {selectedGuardrail.config.anonymizer_url && (
                        <div className="grid grid-cols-3 gap-2 text-sm">
                          <span className="text-muted-foreground">Anonymizer URL:</span>
                          <span className="col-span-2 font-mono text-xs bg-surface-2 p-2 rounded">
                            {selectedGuardrail.config.anonymizer_url}
                          </span>
                        </div>
                      )}

                      {/* Detection Settings */}
                      <div className="grid grid-cols-2 gap-4">
                        {selectedGuardrail.config.threshold && (
                          <div>
                            <span className="text-sm text-muted-foreground">Threshold</span>
                            <div className="text-lg font-semibold font-mono tabular-nums">{selectedGuardrail.config.threshold}</div>
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
                              <Tag
                                key={entity}
                                label={PII_ENTITIES.find(e => e.value === entity)?.label || entity}
                              />
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
                              <div key={entity} className="flex justify-between items-center text-sm bg-surface-2 p-2 rounded">
                                <span className="font-medium">{entity}</span>
                                <Tag label={method as string} />
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
                      <div className="text-center p-3 rounded-[12px] border border-border bg-info/12">
                        <div className="text-2xl font-bold font-mono tabular-nums text-info">
                          {selectedGuardrail.stats.total_executions.toLocaleString()}
                        </div>
                        <div className="text-xs text-info">Total Executions</div>
                      </div>
                      <div className="text-center p-3 rounded-[12px] border border-border bg-good/12">
                        <div className="text-2xl font-bold font-mono tabular-nums text-good">
                          {formatLatency(selectedGuardrail.stats.average_latency)}
                        </div>
                        <div className="text-xs text-good">Avg Latency</div>
                      </div>
                      <div className="text-center p-3 rounded-[12px] border border-border bg-bad/12">
                        <div className="text-2xl font-bold font-mono tabular-nums text-bad">
                          {formatRate(selectedGuardrail.stats.block_rate)}
                        </div>
                        <div className="text-xs text-bad">Block Rate</div>
                      </div>
                      <div className="text-center p-3 rounded-[12px] border border-border bg-warn/12">
                        <div className="text-2xl font-bold font-mono tabular-nums text-warn">
                          {formatRate(selectedGuardrail.stats.error_rate)}
                        </div>
                        <div className="text-xs text-warn">Error Rate</div>
                      </div>
                    </div>
                    {selectedGuardrail.stats.last_executed && (
                      <div className="text-xs text-muted-foreground text-center">
                        Last executed: {formatDateTime(selectedGuardrail.stats.last_executed)}
                      </div>
                    )}
                  </div>
                )}

                <div className="flex gap-2 pt-4">
                  <Button variant="outline" className="flex-1">
                    <Icon icon={icons.edit} className="h-4 w-4 mr-2" />
                    Edit Configuration
                  </Button>
                  <Button variant="outline" className="flex-1">
                    <Icon icon={icons.settings} className="h-4 w-4 mr-2" />
                    Test Guardrail
                  </Button>
                </div>
              </div>
            </>
          )}
        </SheetContent>
      </Sheet>
    </PageShell>
  );
}
