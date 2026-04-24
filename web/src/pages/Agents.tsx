import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { listAgents, deleteAgent, refreshAgentStatus } from "@/lib/api";
import { InvokeDialog } from "@/components/agents/invoke-dialog";
import type { AgentDeployment } from "@/types/api";

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
import { useToast } from "@/hooks/use-toast";

export default function Agents() {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState<AgentDeployment | null>(null);
  const [invokeFor, setInvokeFor] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["agent-deployments"],
    queryFn: listAgents,
    refetchInterval: 5_000,
  });
  const agents: AgentDeployment[] = data?.agents ?? [];

  const refreshMutation = useMutation({
    mutationFn: (id: string) => refreshAgentStatus(id),
    onSuccess: (a) => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({ title: `Status: ${a.status}`, description: a.status_reason });
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
    mutationFn: (id: string) => deleteAgent(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      setConfirmDelete(null);
      toast({ title: "Agent undeployed" });
    },
    onError: (e: any) => {
      toast({
        title: "Undeploy failed",
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

  return (
    <PageShell
      title="Agents"
      description="Deployed agent workloads, scoped to approved use cases. Deploy via the admin API; status refreshes every 5 s."
    >
      {agents.length === 0 ? (
        <EmptyState
          icon={icons.agents}
          title="No agents deployed"
          description="Publish an agent to the registry, then deploy it via the admin API."
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {agents.map((a) => (
            <AgentCard
              key={a.id}
              agent={a}
              onRefresh={() => refreshMutation.mutate(a.id)}
              onDelete={() => setConfirmDelete(a)}
              onInvoke={() => setInvokeFor(a.workload_name)}
            />
          ))}
        </div>
      )}

      <InvokeDialog slug={invokeFor} onClose={() => setInvokeFor(null)} />

      <AlertDialog
        open={!!confirmDelete}
        onOpenChange={(open) => !open && setConfirmDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Undeploy this agent?</AlertDialogTitle>
            <AlertDialogDescription>
              The pod, Service, and scoped-key Secret will be removed for{" "}
              <span className="font-mono">{confirmDelete?.workload_name}</span>.
              The scoped API key is revoked.
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

function AgentCard({
  agent: a,
  onRefresh,
  onDelete,
  onInvoke,
}: {
  agent: AgentDeployment;
  onRefresh: () => void;
  onDelete: () => void;
  onInvoke: () => void;
}) {
  const isFailed = a.status === "failed";
  return (
    <div
      className={cn(
        "group relative rounded-lg border bg-card hover:shadow-sm transition-all",
        isFailed
          ? "border-rose-500/30"
          : "border-border hover:border-primary/40",
      )}
    >
      <div className="p-4 space-y-3">
        <div className="flex items-start gap-3">
          <div
            className={cn(
              "h-10 w-10 rounded-lg flex items-center justify-center shrink-0",
              a.status === "running" &&
                "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20",
              a.status === "deploying" &&
                "bg-sky-500/10 text-sky-600 dark:text-sky-400 ring-1 ring-sky-500/20",
              a.status === "failed" &&
                "bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20",
              (a.status === "pending" ||
                a.status === "terminating" ||
                a.status === "stopped") &&
                "bg-muted text-muted-foreground ring-1 ring-border",
            )}
          >
            <Icon icon={icons.agents} className="h-5 w-5" />
          </div>
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold text-[15px] leading-tight truncate">
              {a.resource_name}
            </h3>
            <code className="text-[11px] font-mono text-muted-foreground block truncate mt-0.5">
              {a.namespace}/{a.workload_name}
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
              <DropdownMenuItem onClick={onInvoke}>
                <Icon icon={icons.play} className="mr-2 h-4 w-4" />
                Invoke
              </DropdownMenuItem>
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
          <StatusPill status={a.status} />
          <span className="inline-flex items-center h-5 px-1.5 rounded bg-muted text-muted-foreground ring-1 ring-border text-[10px] font-mono uppercase tracking-wider capitalize">
            {a.platform}
          </span>
          <span className="inline-flex items-center h-5 px-1.5 rounded bg-muted text-muted-foreground ring-1 ring-border text-[10px] font-mono tabular-nums">
            v{a.resource_version}
          </span>
          <span className="inline-flex items-center h-5 px-1.5 rounded bg-muted text-muted-foreground ring-1 ring-border text-[10px] font-mono tabular-nums">
            {a.replicas}×
          </span>
        </div>

        {a.endpoint && (
          <div className="text-[11px] font-mono text-muted-foreground truncate bg-muted/40 rounded-md px-2 py-1">
            → {a.endpoint}
          </div>
        )}

        {a.status_reason && (
          <p
            className={cn(
              "text-[11px] line-clamp-2 leading-relaxed",
              isFailed
                ? "text-rose-600 dark:text-rose-400 bg-rose-500/5 rounded-md px-2 py-1 border border-rose-500/20"
                : "text-muted-foreground",
            )}
          >
            {a.status_reason}
          </p>
        )}

        {a.last_applied_at && (
          <div className="flex items-center gap-1 text-[10px] font-mono text-muted-foreground pt-1 border-t border-border/50">
            <Icon icon={icons.clock} className="h-3 w-3" />
            applied{" "}
            {formatDistanceToNow(new Date(a.last_applied_at), {
              addSuffix: true,
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function StatusPill({ status }: { status: AgentDeployment["status"] }) {
  const cls: Record<string, string> = {
    running:
      "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-emerald-500/20",
    deploying:
      "bg-sky-500/10 text-sky-700 dark:text-sky-400 ring-sky-500/20",
    failed: "bg-rose-500/10 text-rose-700 dark:text-rose-400 ring-rose-500/20",
    pending: "bg-muted text-muted-foreground ring-border",
    terminating: "bg-amber-500/10 text-amber-700 dark:text-amber-400 ring-amber-500/20",
    stopped: "bg-muted text-muted-foreground ring-border",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center h-5 px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
        cls[status] ?? "bg-muted text-muted-foreground ring-border",
      )}
    >
      {status}
    </span>
  );
}
