import { ColumnDef } from "@tanstack/react-table";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { StatusChip } from "@/components/common/status-chip";
import { SparklineChart } from "./ModelCharts";
import { ModelWithUsage } from "@/types/api";
import { detectProvider } from "@/lib/providers";

// Sparkline stroke per model state — concrete colors (recharts strokes are
// SVG attributes, which don't resolve CSS vars). Matches the status chips:
// healthy = emerald, degraded = amber, paused/unhealthy = dim/red.
function sparkColor(model: ModelWithUsage): string {
  if (model.is_active === false) return "#94A3B8"; // paused
  const health = model.usage_stats?.health_score ?? 100;
  if (health >= 90) return "#0E9F6E"; // healthy
  if (health >= 70) return "#D97706"; // degraded
  return "#E0484D"; // unhealthy
}

export const formatNumber = (num: number): string => {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`;
  if (num >= 1000) return `${(num / 1000).toFixed(1)}K`;
  return num.toString();
};

export const formatCurrency = (amount: number): string => {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
  }).format(amount);
};

// Per-million-token price, the way model cards are usually quoted. Falls
// back to an em-dash when the model has no pricing configured.
const formatPricePerM = (perToken?: number): string | null => {
  if (perToken == null || perToken <= 0) return null;
  const perM = perToken * 1_000_000;
  const digits = perM >= 1 ? 2 : 3;
  return `$${perM.toFixed(digits)}`;
};

const formatContext = (maxTokens?: number): string | null => {
  if (!maxTokens || maxTokens <= 0) return null;
  if (maxTokens >= 1000) return `${Math.round(maxTokens / 1000)}K`;
  return String(maxTokens);
};

export const columns: ColumnDef<ModelWithUsage>[] = [
  {
    accessorKey: "id",
    header: "Model",
    cell: ({ row }) => {
      const model = row.original;
      return (
        <span className="font-mono text-[13px] font-semibold text-foreground">
          {model.id}
        </span>
      );
    },
  },
  {
    accessorKey: "owned_by",
    header: "Provider",
    cell: ({ row }) => {
      const model = row.original;
      const providerInfo = detectProvider(model.id, model.owned_by);
      return (
        <div className="flex items-center gap-2 min-w-0">
          <Icon
            icon={providerInfo.icon}
            width="16"
            height="16"
            className={`shrink-0 ${providerInfo.color}`}
          />
          <span className="truncate text-[13px] text-foreground">
            {providerInfo.name}
          </span>
        </div>
      );
    },
  },
  {
    accessorKey: "max_tokens",
    header: "Context",
    cell: ({ row }) => {
      const ctx = formatContext(row.original.max_tokens);
      return ctx ? (
        <span className="font-mono text-[13px] text-foreground">{ctx}</span>
      ) : (
        <span className="text-text-dim">—</span>
      );
    },
  },
  {
    id: "price",
    header: "Price in / out",
    cell: ({ row }) => {
      const model = row.original;
      const priceIn = formatPricePerM(model.input_cost_per_token);
      const priceOut = formatPricePerM(model.output_cost_per_token);
      if (!priceIn && !priceOut) {
        return <span className="text-text-dim">—</span>;
      }
      return (
        <span className="font-mono text-[13px]">
          <span className="text-foreground">{priceIn ?? "—"}</span>
          <span className="text-text-dim"> / {priceOut ?? "—"}</span>
        </span>
      );
    },
  },
  {
    accessorKey: "usage_stats.avg_latency",
    header: ({ column }) => (
      <button
        type="button"
        onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        className="inline-flex items-center gap-1 hover:text-foreground"
      >
        Latency
        <Icon icon={icons.arrowUpDown} className="h-3 w-3" />
      </button>
    ),
    cell: ({ row }) => {
      const latency = row.original.usage_stats?.avg_latency || 0;
      return latency > 0 ? (
        <span className="font-mono text-[13px] text-foreground">{latency}ms</span>
      ) : (
        <span className="text-text-dim">—</span>
      );
    },
  },
  {
    accessorKey: "usage_stats.requests_today",
    header: ({ column }) => (
      <button
        type="button"
        onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        className="inline-flex items-center gap-1 hover:text-foreground"
      >
        Requests 24h
        <Icon icon={icons.arrowUpDown} className="h-3 w-3" />
      </button>
    ),
    cell: ({ row }) => {
      const model = row.original;
      const requests = model.usage_stats?.requests_today || 0;
      const trend = model.usage_stats?.trend_data ?? [];
      return (
        <div className="flex items-center gap-3">
          {trend.length > 1 ? (
            <SparklineChart data={trend} color={sparkColor(model)} className="h-7 w-24" />
          ) : (
            <span className="h-7 w-24" />
          )}
          <span className="font-mono text-[13px] tabular-nums text-foreground">
            {requests > 0 ? formatNumber(requests) : <span className="text-text-dim">0</span>}
          </span>
        </div>
      );
    },
  },
  {
    accessorKey: "usage_stats.health_score",
    header: "Status",
    cell: ({ row }) => {
      const model = row.original;
      const isActive = model.is_active !== false;
      const health = model.usage_stats?.health_score ?? 100;
      if (!isActive) {
        return <StatusChip tone="neutral" label="Paused" />;
      }
      if (health >= 90) {
        return <StatusChip tone="good" label="Healthy" />;
      }
      if (health >= 70) {
        return <StatusChip tone="warn" label="Degraded" />;
      }
      return <StatusChip tone="bad" label="Unhealthy" />;
    },
  },
  {
    id: "actions",
    enableHiding: false,
    cell: ({ row, table }) => {
      const model = row.original;
      const isUserModel = model.source === "user";
      const meta = table.options.meta as { onEdit?: (model: ModelWithUsage) => void; onDelete?: (model: ModelWithUsage) => void } | undefined;

      return (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              className="h-8 w-8 p-0 text-muted-foreground"
              onClick={(e) => e.stopPropagation()}
            >
              <span className="sr-only">Open menu</span>
              <Icon icon={icons.moreVertical} className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
            <DropdownMenuLabel>Actions</DropdownMenuLabel>
            <DropdownMenuItem
              onClick={() => navigator.clipboard.writeText(model.id)}
            >
              Copy model ID
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            {isUserModel && (
              <DropdownMenuItem onClick={() => meta?.onEdit?.(model)}>
                <Icon icon={icons.edit} className="h-3 w-3 mr-2" />
                Edit model
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              className="text-destructive"
              onClick={() => meta?.onDelete?.(model)}
            >
              <Icon icon={icons.delete} className="h-3 w-3 mr-2" />
              Delete model
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      );
    },
  },
];
