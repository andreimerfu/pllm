import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  listDeployments,
  refreshDeploymentStatus,
  deleteDeployment,
  bumpDeploymentOOM,
  resetDeploymentOOM,
} from "@/lib/api";
import type { Deployment, DeploymentStatus } from "@/types/api";

import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { PageShell } from "@/components/common/page-shell";
import {
  ClusterRing,
  RecentApplied,
  StatusMixBar,
  StatusPill,
} from "@/components/deployments/deployments-overview";
import { useToast } from "@/hooks/use-toast";

export default function Deployments() {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState<Deployment | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["deployments"],
    queryFn: listDeployments,
    refetchInterval: 5_000,
  });
  const deployments: Deployment[] = data?.deployments ?? [];

  const refreshMutation = useMutation({
    mutationFn: (id: string) => refreshDeploymentStatus(id),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      toast({ title: `Status: ${d.status}`, description: d.status_reason });
    },
    onError: (e: any) => {
      toast({
        title: "Refresh failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteDeployment(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      setConfirmDelete(null);
      toast({ title: "Deployment removed" });
    },
    onError: (e: any) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const bumpOOMMutation = useMutation({
    mutationFn: (id: string) => bumpDeploymentOOM(id),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      toast({
        title: "Memory bumped",
        description: `New limit: ${d.oom_memory_override ?? "baseline"}`,
      });
    },
    onError: (e: any) => {
      toast({
        title: "Bump failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const resetOOMMutation = useMutation({
    mutationFn: (id: string) => resetDeploymentOOM(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      toast({ title: "Memory override cleared" });
    },
    onError: (e: any) => {
      toast({
        title: "Reset failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  const byStatus = deployments.reduce<Record<DeploymentStatus, number>>(
    (acc, d) => {
      acc[d.status] = (acc[d.status] ?? 0) + 1;
      return acc;
    },
    {} as Record<DeploymentStatus, number>,
  );
  const withBackend = deployments.filter((d) => d.gateway_backend_id).length;

  return (
    <PageShell
      title="Deployments"
      description="Registry servers running as dedicated Kubernetes workloads. Each deployment is auto-registered as an MCP Gateway backend."
    >
      {/* ────────────── Hero ────────────── */}
      <div className="rounded-lg border border-border bg-card p-5">
        <div className="grid grid-cols-1 md:grid-cols-[auto_1fr] gap-6 items-center">
          <ClusterRing deployments={deployments} />

          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
            <HeroStat
              icon={icons.rocket}
              label="Total"
              value={deployments.length}
              tone="muted"
            />
            <HeroStat
              icon={icons.check}
              label="Running"
              value={byStatus.running ?? 0}
              tone={(byStatus.running ?? 0) > 0 ? "emerald" : "muted"}
            />
            <HeroStat
              icon={icons.error}
              label="Failed"
              value={byStatus.failed ?? 0}
              tone={(byStatus.failed ?? 0) > 0 ? "rose" : "muted"}
            />
            <HeroStat
              icon={icons.mcp}
              label="MCP linked"
              value={withBackend}
              tone={withBackend > 0 ? "emerald" : "muted"}
            />
          </div>
        </div>
        <div className="mt-4">
          <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-1.5">
            Status distribution
          </div>
          <StatusMixBar deployments={deployments} />
        </div>
      </div>

      {deployments.length === 0 ? (
        <EmptyState
          icon={icons.deployment}
          title="No active deployments"
          description="Open the Registry, click a server, and hit Deploy to spin up a Kubernetes workload that the MCP Gateway will route to."
        />
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {deployments.map((d) => (
              <DeploymentCard
                key={d.id}
                deployment={d}
                onRefresh={() => refreshMutation.mutate(d.id)}
                onDelete={() => setConfirmDelete(d)}
                onBumpOOM={() => bumpOOMMutation.mutate(d.id)}
                onResetOOM={() => resetOOMMutation.mutate(d.id)}
              />
            ))}
          </div>
          <div className="rounded-lg border border-border bg-card p-4 h-fit sticky top-4">
            <div className="mb-3">
              <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                Timeline
              </div>
              <div className="text-sm font-semibold tracking-tight mt-0.5">
                Recently applied
              </div>
            </div>
            <RecentApplied deployments={deployments} />
          </div>
        </div>
      )}

      <AlertDialog
        open={!!confirmDelete}
        onOpenChange={(open) => !open && setConfirmDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Undeploy workload?</AlertDialogTitle>
            <AlertDialogDescription>
              This will delete the Kubernetes Deployment/Service for{" "}
              <span className="font-mono">{confirmDelete?.workload_name}</span>{" "}
              and remove its MCP Gateway backend. Active sessions using it will
              fail.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() =>
                confirmDelete && deleteMutation.mutate(confirmDelete.id)
              }
            >
              Undeploy
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}

function DeploymentCard({
  deployment: d,
  onRefresh,
  onDelete,
  onBumpOOM,
  onResetOOM,
}: {
  deployment: Deployment;
  onRefresh: () => void;
  onDelete: () => void;
  onBumpOOM: () => void;
  onResetOOM: () => void;
}) {
  const isFailed = d.status === "failed";
  const hasOverride = !!d.oom_memory_override;
  const attempts = d.oom_attempts ?? 0;
  return (
    <div
      className={cn(
        "group relative rounded-lg border bg-card hover:shadow-sm transition-all",
        isFailed ? "border-rose-500/30" : "border-border hover:border-primary/40",
      )}
    >
      <div className="p-4 space-y-3">
        <div className="flex items-start gap-3">
          <div
            className={cn(
              "h-10 w-10 rounded-lg flex items-center justify-center shrink-0",
              d.status === "running" &&
                "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20",
              d.status === "deploying" &&
                "bg-sky-500/10 text-sky-600 dark:text-sky-400 ring-1 ring-sky-500/20",
              d.status === "failed" &&
                "bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20",
              (d.status === "pending" ||
                d.status === "terminating" ||
                d.status === "stopped") &&
                "bg-muted text-muted-foreground ring-1 ring-border",
            )}
          >
            <Icon icon={icons.rocket} className="h-5 w-5" />
          </div>
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold text-[15px] leading-tight truncate">
              {d.resource_name}
            </h3>
            <code className="text-[11px] font-mono text-muted-foreground block truncate mt-0.5">
              {d.namespace}/{d.workload_name}
            </code>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 opacity-60 group-hover:opacity-100"
              >
                <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={onRefresh}>
                <Icon icon={icons.refresh} className="mr-2 h-4 w-4" />
                Refresh status
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onBumpOOM}>
                <Icon icon={icons.rocket} className="mr-2 h-4 w-4" />
                Bump memory
              </DropdownMenuItem>
              {hasOverride && (
                <DropdownMenuItem onClick={onResetOOM}>
                  <Icon icon={icons.refresh} className="mr-2 h-4 w-4" />
                  Reset memory to baseline
                </DropdownMenuItem>
              )}
              <DropdownMenuItem
                className="text-destructive"
                onClick={onDelete}
              >
                <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
                Undeploy
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <div className="flex items-center gap-1.5 flex-wrap">
          <StatusPill status={d.status} withPulse />
          <span className="inline-flex items-center h-5 px-1.5 rounded bg-muted text-muted-foreground ring-1 ring-border text-[10px] font-mono uppercase tracking-wider capitalize">
            {d.platform}
          </span>
          <span className="inline-flex items-center h-5 px-1.5 rounded bg-muted text-muted-foreground ring-1 ring-border text-[10px] font-mono tabular-nums">
            v{d.resource_version}
          </span>
          {d.gateway_backend_id && (
            <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded bg-primary/10 text-primary ring-1 ring-primary/20 text-[10px] font-mono uppercase tracking-wider">
              <Icon icon={icons.mcp} className="h-2.5 w-2.5" />
              mcp-linked
            </span>
          )}
          {hasOverride && (
            <span
              className="inline-flex items-center gap-1 h-5 px-1.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-500/20 text-[10px] font-mono uppercase tracking-wider"
              title={`OOM auto-scaled — ${attempts} bump${attempts === 1 ? "" : "s"} so far`}
            >
              <Icon icon={icons.warning} className="h-2.5 w-2.5" />
              mem↑ {d.oom_memory_override}
              {attempts > 0 && ` (${attempts})`}
            </span>
          )}
        </div>

        {d.endpoint && (
          <div className="text-[11px] font-mono text-muted-foreground truncate bg-muted/40 rounded-md px-2 py-1">
            → {d.endpoint}
          </div>
        )}

        {d.status_reason && (
          <p
            className={cn(
              "text-[11px] line-clamp-2 leading-relaxed",
              isFailed
                ? "text-rose-600 dark:text-rose-400 bg-rose-500/5 rounded-md px-2 py-1 border border-rose-500/20"
                : "text-muted-foreground",
            )}
          >
            {d.status_reason}
          </p>
        )}

        {d.last_applied_at && (
          <div className="flex items-center gap-1 text-[10px] font-mono text-muted-foreground pt-1 border-t border-border/50">
            <Icon icon={icons.clock} className="h-3 w-3" />
            applied{" "}
            {formatDistanceToNow(new Date(d.last_applied_at), {
              addSuffix: true,
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function HeroStat({
  icon,
  label,
  value,
  tone = "muted",
}: {
  icon: string;
  label: string;
  value: number;
  tone?: "emerald" | "amber" | "rose" | "muted";
}) {
  return (
    <div className="rounded-md border border-border/70 bg-background/40 px-3 py-2.5 flex items-center gap-2.5">
      <div
        className={cn(
          "h-8 w-8 rounded-md flex items-center justify-center shrink-0",
          tone === "emerald" &&
            "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
          tone === "amber" &&
            "bg-amber-500/10 text-amber-600 dark:text-amber-400",
          tone === "rose" && "bg-rose-500/10 text-rose-600 dark:text-rose-400",
          tone === "muted" && "bg-muted text-muted-foreground",
        )}
      >
        <Icon icon={icon} className="h-4 w-4" />
      </div>
      <div className="min-w-0">
        <div className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground leading-none">
          {label}
        </div>
        <div
          className={cn(
            "text-lg font-bold tabular-nums leading-none mt-1",
            tone === "emerald" && "text-emerald-600 dark:text-emerald-400",
            tone === "amber" && "text-amber-600 dark:text-amber-400",
            tone === "rose" && "text-rose-600 dark:text-rose-400",
          )}
        >
          {value}
        </div>
      </div>
    </div>
  );
}
