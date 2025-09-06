import { ColumnDef } from "@tanstack/react-table";
import { ArrowUpDown, MoreHorizontal, TrendingUp, Activity } from "lucide-react";
import { Icon } from "@iconify/react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ModelWithUsage } from "@/types/api";
import { detectProvider } from "@/lib/providers";
import { SparklineChart } from "./ModelCharts";
import ModelTags from "./ModelTags";
import ModelCapabilities from "./ModelCapabilities";

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

export const columns: ColumnDef<ModelWithUsage>[] = [
  {
    accessorKey: "id",
    header: "Model",
    cell: ({ row }) => {
      const model = row.original;
      const providerInfo = detectProvider(model.id, model.owned_by);
      
      return (
        <div className="flex items-center gap-3 min-w-0">
          <div className={`flex-shrink-0 p-2 rounded-lg border ${providerInfo.bgColor} ${providerInfo.borderColor}`}>
            <Icon
              icon={providerInfo.icon}
              width="20"
              height="20"
              className={providerInfo.color}
            />
          </div>
          <div className="min-w-0">
            <div className="font-medium truncate">{model.id}</div>
            <div className={`text-sm ${providerInfo.color}`}>
              {providerInfo.name}
            </div>
          </div>
        </div>
      );
    },
  },
  {
    accessorKey: "object",
    header: "Type",
    cell: ({ row }) => (
      <Badge variant="outline" className="font-medium">
        {row.getValue("object")}
      </Badge>
    ),
  },
  {
    accessorKey: "tags",
    header: "Tags",
    cell: ({ row }) => {
      const model = row.original;
      return (
        <div className="min-w-0 max-w-32">
          <ModelTags tags={model.tags} maxVisible={2} />
        </div>
      );
    },
  },
  {
    accessorKey: "capabilities",
    header: "Capabilities",
    cell: ({ row }) => {
      const model = row.original;
      return (
        <div className="min-w-0 max-w-40">
          <ModelCapabilities capabilities={model.capabilities} maxVisible={4} />
        </div>
      );
    },
  },
  {
    accessorKey: "usage_stats.requests_today",
    header: ({ column }) => (
      <Button
        variant="ghost"
        onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        className="gap-1"
      >
        Total Requests
        <ArrowUpDown className="h-4 w-4" />
      </Button>
    ),
    cell: ({ row }) => {
      const requests = row.original.usage_stats?.requests_total || 0;
      const trendData = row.original.usage_stats?.trend_data?.slice(-7); // Last 7 days
      
      return (
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-muted-foreground" />
            <span className="font-medium">{formatNumber(requests)}</span>
          </div>
          {trendData && trendData.length > 0 && (
            <SparklineChart 
              data={trendData} 
              color="#3b82f6" 
              className="h-6 w-16" 
            />
          )}
        </div>
      );
    },
  },
  {
    accessorKey: "usage_stats.tokens_total",
    header: ({ column }) => (
      <Button
        variant="ghost"
        onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        className="gap-1"
      >
        Total Tokens
        <ArrowUpDown className="h-4 w-4" />
      </Button>
    ),
    cell: ({ row }) => {
      const tokens = row.original.usage_stats?.tokens_total || 0;
      return (
        <div className="flex items-center gap-2">
          <TrendingUp className="h-4 w-4 text-muted-foreground" />
          <span className="font-medium">{formatNumber(tokens)}</span>
        </div>
      );
    },
  },
  {
    accessorKey: "usage_stats.cost_total",
    header: ({ column }) => (
      <Button
        variant="ghost"
        onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        className="gap-1"
      >
        Total Cost
        <ArrowUpDown className="h-4 w-4" />
      </Button>
    ),
    cell: ({ row }) => {
      const cost = row.original.usage_stats?.cost_total || 0;
      return (
        <span className="font-medium">
          {formatCurrency(cost)}
        </span>
      );
    },
  },
  {
    accessorKey: "usage_stats.health_score",
    header: "Health & Trend",
    cell: ({ row }) => {
      const health = row.original.usage_stats?.health_score || 100;
      const isHealthy = health >= 90;
      const isWarning = health >= 70 && health < 90;
      const trendData = row.original.usage_stats?.trend_data?.slice(-7); // Last 7 days
      
      return (
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <div
              className={`w-2 h-2 rounded-full ${
                isHealthy ? "bg-green-500" : isWarning ? "bg-yellow-500" : "bg-red-500"
              }`}
            />
            <span className="font-medium">{health}%</span>
          </div>
          {trendData && trendData.length > 0 && (
            <SparklineChart 
              data={trendData} 
              color={isHealthy ? "#10b981" : isWarning ? "#f59e0b" : "#ef4444"}
              className="h-6 w-16" 
            />
          )}
        </div>
      );
    },
  },
  {
    accessorKey: "created",
    header: "Created",
    cell: ({ row }) => {
      const date = new Date((row.getValue("created") as number) * 1000);
      return (
        <span className="text-muted-foreground">
          {date.toLocaleDateString("en-US", {
            year: "numeric",
            month: "short", 
            day: "numeric",
          })}
        </span>
      );
    },
  },
  {
    id: "actions",
    enableHiding: false,
    cell: ({ row }) => {
      const model = row.original;

      return (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button 
              variant="ghost" 
              className="h-8 w-8 p-0"
              onClick={(e) => e.stopPropagation()}
            >
              <span className="sr-only">Open menu</span>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Actions</DropdownMenuLabel>
            <DropdownMenuItem
              onClick={() => navigator.clipboard.writeText(model.id)}
            >
              Copy model ID
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem>View details</DropdownMenuItem>
            <DropdownMenuItem>View usage stats</DropdownMenuItem>
            <DropdownMenuItem className="text-destructive">
              Disable model
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      );
    },
  },
];