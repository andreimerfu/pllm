import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  listAgents,
  deleteAgent,
  refreshAgentStatus,
  bumpAgentOOM,
  resetAgentOOM,
  listUseCases,
} from "@/lib/api";
import { InvokeDialog } from "@/components/agents/invoke-dialog";
import { DeployDialog } from "@/components/agents/deploy-dialog";
import type { AgentDeployment, UseCase } from "@/types/api";

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
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { PageShell, StatTile } from "@/components/common/page-shell";
import { useToast } from "@/hooks/use-toast";

export default function Agents() {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState<AgentDeployment | null>(null);
  const [invokeFor, setInvokeFor] = useState<string | null>(null);
  const [deployOpen, setDeployOpen] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["agent-deployments"],
    queryFn: listAgents,
    refetchInterval: 5_000,
  });
  const agents: AgentDeployment[] = data?.agents ?? [];

  // Use cases are needed to render names on the cards; the deployment row
  // only carries a UUID. Load once when the page mounts.
  const { data: ucRes } = useQuery({
    queryKey: ["use-cases", "for-agents-page"],
    queryFn: () => listUseCases(),
  });
  const useCases: UseCase[] = ucRes?.use_cases ?? [];
  const useCaseById = useMemo(() => {
    const m = new Map<string, UseCase>();
    for (const u of useCases) m.set(u.id, u);
    return m;
  }, [useCases]);

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

  const bumpOOMMutation = useMutation({
    mutationFn: (id: string) => bumpAgentOOM(id),
    onSuccess: (a) => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({
        title: "Memory bumped",
        description: `New limit: ${a.oom_memory_override ?? "baseline"}`,
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
    mutationFn: (id: string) => resetAgentOOM(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
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

  // --- derived stats + grouping --------------------------------------------

  const stats = useMemo(() => {
    const byStatus = agents.reduce<Record<string, number>>((acc, a) => {
      acc[a.status] = (acc[a.status] ?? 0) + 1;
      return acc;
    }, {});
    const pods = agents.reduce((n, a) => n + (a.replicas || 1), 0);
    const oomBumped = agents.filter((a) => !!a.oom_memory_override).length;
    const useCasesCovered = new Set(agents.map((a) => a.use_case_id)).size;
    return {
      total: agents.length,
      running: byStatus.running ?? 0,
      failed: byStatus.failed ?? 0,
      deploying: byStatus.deploying ?? 0,
      pods,
      oomBumped,
      useCasesCovered,
    };
  }, [agents]);

  // Group agents by their bound use case — this is the editorial move.
  // Agents live under use cases in the governance model; showing that
  // grouping in the UI makes the scope-enforcement story visible.
  const grouped = useMemo(() => {
    const buckets = new Map<string, { uc: UseCase | undefined; agents: AgentDeployment[] }>();
    for (const a of agents) {
      const bucket = buckets.get(a.use_case_id) ?? {
        uc: useCaseById.get(a.use_case_id),
        agents: [],
      };
      bucket.agents.push(a);
      buckets.set(a.use_case_id, bucket);
    }
    return Array.from(buckets.values()).sort((a, b) => {
      // Stable: biggest bucket first, then by use case name
      const d = b.agents.length - a.agents.length;
      if (d !== 0) return d;
      return (a.uc?.name ?? "").localeCompare(b.uc?.name ?? "");
    });
  }, [agents, useCaseById]);

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
      description="Deployed agent workloads, each scoped to an approved use case. Every pod runs with an auto-provisioned key that cannot escape its binding."
      actions={
        <Button onClick={() => setDeployOpen(true)}>
          <Icon icon={icons.rocket} className="mr-2 size-4" />
          Deploy agent
        </Button>
      }
    >
      {agents.length === 0 ? (
        <EmptyState
          icon={icons.agents}
          title="No agents deployed"
          description="Publish an agent to the registry, bind it to a use case, and deploy. Its scoped API key is provisioned automatically."
          action={
            <Button onClick={() => setDeployOpen(true)}>
              <Icon icon={icons.rocket} className="mr-2 size-4" />
              Deploy your first agent
            </Button>
          }
        />
      ) : (
        <>
          {/* ────────────── KPI cards ────────────── */}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
            <StatTile
              label="Agents"
              value={stats.total}
              icon={<Icon icon={icons.agents} />}
              iconColor="agent"
              sub="Across all use cases"
            />
            <StatTile
              label="Running"
              value={stats.running}
              tone={stats.running > 0 ? "good" : "default"}
              icon={<Icon icon={icons.rocket} />}
              iconColor="good"
              sub={stats.deploying > 0 ? `${stats.deploying} deploying` : undefined}
            />
            <StatTile
              label="Failed"
              value={stats.failed}
              tone={stats.failed > 0 ? "bad" : "default"}
              icon={<Icon icon={icons.warning} />}
              iconColor="bad"
            />
            <StatTile
              label="Pods"
              value={stats.pods}
              icon={<Icon icon={icons.mcp} />}
              iconColor="mcp"
              sub={stats.pods === stats.total ? "1× each" : undefined}
            />
            <StatTile
              label="Use cases"
              value={stats.useCasesCovered}
              icon={<Icon icon={icons.useCase} />}
              iconColor="primary"
              sub={stats.useCasesCovered === 1 ? "single binding" : "across bindings"}
            />
            {stats.oomBumped > 0 && (
              <StatTile
                label="Mem scaled"
                value={stats.oomBumped}
                tone="warn"
                icon={<Icon icon={icons.bolt} />}
                iconColor="warn"
                sub="OOM auto-bumped"
              />
            )}
          </div>

          {/* ────────────── Grouped by use case ────────────── */}
          <div className="space-y-6">
            {grouped.map(({ uc, agents: bucket }) => (
              <section
                key={uc?.id ?? "unbound"}
                className="space-y-2.5"
                aria-label={uc?.name ?? "Unbound"}
              >
                <UseCaseHeader uc={uc} agentCount={bucket.length} />
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                  {bucket.map((a) => (
                    <AgentCard
                      key={a.id}
                      agent={a}
                      useCase={uc}
                      onRefresh={() => refreshMutation.mutate(a.id)}
                      onDelete={() => setConfirmDelete(a)}
                      onInvoke={() => setInvokeFor(a.workload_name)}
                      onBumpOOM={() => bumpOOMMutation.mutate(a.id)}
                      onResetOOM={() => resetOOMMutation.mutate(a.id)}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        </>
      )}

      <InvokeDialog slug={invokeFor} onClose={() => setInvokeFor(null)} />
      <DeployDialog open={deployOpen} onClose={() => setDeployOpen(false)} />

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
              Inbound invocations to this workload will fail.
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

// ---------------------------------------------------------------------------
// Use-case group header
// ---------------------------------------------------------------------------

function UseCaseHeader({
  uc,
  agentCount,
}: {
  uc: UseCase | undefined;
  agentCount: number;
}) {
  return (
    <div className="flex items-baseline gap-3 pb-1 border-b border-border/50">
      <Icon
        icon={icons.useCase}
        className="h-3.5 w-3.5 text-muted-foreground self-center"
      />
      <div className="flex items-baseline gap-2 min-w-0">
        <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Use case
        </span>
        <h2 className="text-[15px] font-semibold tracking-tight truncate">
          {uc?.name ?? (
            <span className="text-muted-foreground italic">Unbound</span>
          )}
        </h2>
        {uc?.slug && (
          <code className="text-[10px] font-mono text-muted-foreground tabular-nums">
            {uc.slug}
          </code>
        )}
      </div>
      <span className="ml-auto text-[10px] font-mono uppercase tracking-widest text-muted-foreground tabular-nums">
        {agentCount} {agentCount === 1 ? "agent" : "agents"}
      </span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent card
// ---------------------------------------------------------------------------

function AgentCard({
  agent: a,
  useCase,
  onRefresh,
  onDelete,
  onInvoke,
  onBumpOOM,
  onResetOOM,
}: {
  agent: AgentDeployment;
  useCase: UseCase | undefined;
  onRefresh: () => void;
  onDelete: () => void;
  onInvoke: () => void;
  onBumpOOM: () => void;
  onResetOOM: () => void;
}) {
  const isFailed = a.status === "failed";
  const isRunning = a.status === "running";
  const hasOverride = !!a.oom_memory_override;
  const attempts = a.oom_attempts ?? 0;

  // Short name: strip the namespace prefix ("pllm.io/agent-research" → "agent-research")
  const shortName = a.resource_name.includes("/")
    ? a.resource_name.slice(a.resource_name.lastIndexOf("/") + 1)
    : a.resource_name;

  return (
    <div
      className={cn(
        "group relative rounded-lg border bg-card transition-all overflow-hidden",
        isFailed
          ? "border-rose-500/30 shadow-[inset_0_0_0_1px_rgb(244_63_94_/_0.08)]"
          : "border-border hover:border-primary/40 hover:shadow-[0_0_0_1px_rgb(var(--primary)/_0.15)]",
      )}
    >
      {/* Top accent bar — status-colored, 1.5px. Replaces the square avatar. */}
      <div
        className={cn(
          "h-[2px] w-full",
          isRunning && "bg-gradient-to-r from-emerald-500/80 via-emerald-400/40 to-transparent",
          a.status === "deploying" &&
            "bg-gradient-to-r from-sky-500/80 via-sky-400/40 to-transparent animate-pulse",
          isFailed && "bg-gradient-to-r from-rose-500/80 via-rose-400/40 to-transparent",
          (a.status === "pending" ||
            a.status === "terminating" ||
            a.status === "stopped") &&
            "bg-border",
        )}
      />

      <div className="p-4 space-y-3">
        {/* Title row — name-first, actions on the right */}
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1 space-y-0.5">
            <div className="flex items-baseline gap-2 min-w-0">
              <h3 className="text-[15px] font-semibold tracking-tight leading-tight truncate">
                {shortName}
              </h3>
              <code className="text-[10px] font-mono text-muted-foreground tabular-nums shrink-0">
                v{a.resource_version}
              </code>
            </div>
            <div className="flex items-center gap-1.5 text-[10.5px] font-mono text-muted-foreground/80 truncate">
              <Icon icon={icons.globe} className="h-2.5 w-2.5 shrink-0" />
              <span className="truncate">
                {a.namespace}/{a.workload_name}
              </span>
            </div>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 opacity-60 group-hover:opacity-100 transition-opacity"
              >
                <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-44">
              <DropdownMenuItem onClick={onInvoke} disabled={!isRunning}>
                <Icon icon={icons.play} className="mr-2 h-4 w-4" />
                Invoke
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onRefresh}>
                <Icon icon={icons.refresh} className="mr-2 h-4 w-4" />
                Refresh status
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onBumpOOM}>
                <Icon icon={icons.plus} className="mr-2 h-4 w-4" />
                Bump memory
              </DropdownMenuItem>
              {hasOverride && (
                <DropdownMenuItem onClick={onResetOOM}>
                  <Icon icon={icons.refresh} className="mr-2 h-4 w-4" />
                  Reset memory
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-destructive" onClick={onDelete}>
                <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
                Undeploy
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {/* Metadata chips row */}
        <div className="flex items-center gap-1.5 flex-wrap">
          <StatusPill status={a.status} />
          <MetaChip icon={icons.deployment}>{a.platform}</MetaChip>
          <MetaChip icon={icons.rocket}>
            {a.replicas}× {a.replicas === 1 ? "pod" : "pods"}
          </MetaChip>
          {useCase?.risk_tier && (
            <TierBadge tier={useCase.risk_tier} />
          )}
          {hasOverride && (
            <span
              className="inline-flex items-center gap-1 h-5 px-1.5 rounded bg-amber-500/10 text-amber-700 dark:text-amber-400 ring-1 ring-amber-500/30 text-[10px] font-mono tabular-nums"
              title={`OOM auto-scaled — ${attempts} bump${attempts === 1 ? "" : "s"} so far`}
            >
              <Icon icon={icons.warning} className="h-2.5 w-2.5" />
              {a.oom_memory_override}
              {attempts > 0 && <span className="text-amber-500/80"> /{attempts}</span>}
            </span>
          )}
        </div>

        {/* Endpoint — subtle monospace strip */}
        {a.endpoint && (
          <div className="text-[11px] font-mono text-muted-foreground/90 bg-muted/40 rounded-md px-2 py-1 truncate border border-border/40 hover:border-border/70 transition-colors">
            <span className="text-muted-foreground/60 mr-1.5">→</span>
            {a.endpoint}
          </div>
        )}

        {/* Failure reason, if any */}
        {a.status_reason && isFailed && (
          <p className="text-[11px] leading-relaxed text-rose-600 dark:text-rose-400 bg-rose-500/5 rounded-md px-2 py-1.5 border border-rose-500/20 line-clamp-3">
            {a.status_reason}
          </p>
        )}

        {/* Footer — applied-ago */}
        {a.last_applied_at && (
          <div className="flex items-center justify-between gap-2 pt-2 border-t border-border/40 text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            <span className="inline-flex items-center gap-1">
              <Icon icon={icons.clock} className="h-3 w-3" />
              applied{" "}
              {formatDistanceToNow(new Date(a.last_applied_at), {
                addSuffix: true,
              })}
            </span>
            {a.secret_name && (
              <span
                className="inline-flex items-center gap-1 normal-case tracking-normal"
                title={`Scoped key attached as Secret: ${a.secret_name}`}
              >
                <Icon icon={icons.keys} className="h-3 w-3" />
                <span className="text-muted-foreground/80">scoped key</span>
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Small reused bits
// ---------------------------------------------------------------------------

function StatusPill({ status }: { status: AgentDeployment["status"] }) {
  const cls: Record<string, string> = {
    running:
      "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-emerald-500/20",
    deploying:
      "bg-sky-500/10 text-sky-700 dark:text-sky-400 ring-sky-500/20",
    failed: "bg-rose-500/10 text-rose-700 dark:text-rose-400 ring-rose-500/20",
    pending: "bg-muted text-muted-foreground ring-border",
    terminating:
      "bg-amber-500/10 text-amber-700 dark:text-amber-400 ring-amber-500/20",
    stopped: "bg-muted text-muted-foreground ring-border",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 h-5 px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
        cls[status] ?? "bg-muted text-muted-foreground ring-border",
      )}
    >
      {status === "running" && (
        <span className="relative flex h-1.5 w-1.5">
          <span className="absolute inset-0 rounded-full bg-emerald-500/50 animate-ping" />
          <span className="relative rounded-full h-1.5 w-1.5 bg-emerald-500" />
        </span>
      )}
      {status}
    </span>
  );
}

function MetaChip({
  icon,
  children,
}: {
  icon: string;
  children: React.ReactNode;
}) {
  return (
    <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded bg-muted/60 text-muted-foreground ring-1 ring-border text-[10px] font-mono tabular-nums">
      <Icon icon={icon} className="h-2.5 w-2.5" />
      {children}
    </span>
  );
}

function TierBadge({ tier }: { tier: string }) {
  const tones: Record<string, string> = {
    tier1: "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
    tier2: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
    tier3: "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
    tier4: "bg-muted text-muted-foreground ring-border",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center h-5 px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
        tones[tier] ?? "bg-muted text-muted-foreground ring-border",
      )}
      title={`Risk ${tier}`}
    >
      {tier.replace("tier", "T")}
    </span>
  );
}
