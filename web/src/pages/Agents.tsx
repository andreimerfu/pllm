import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  listAgents,
  listRegistryAgents,
  deleteAgent,
  refreshAgentStatus,
  bumpAgentOOM,
  resetAgentOOM,
  listUseCases,
} from "@/lib/api";
import { InvokeDialog } from "@/components/agents/invoke-dialog";
import { DeployDialog } from "@/components/agents/deploy-dialog";
import { PublishDialog } from "@/components/registry/publish-dialog";
import { KindIcon } from "@/components/registry/kind-icon";
import type { AgentDeployment, RegistryAgent, UseCase } from "@/types/api";

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

// The Agents page is registry-agent-centric: it lists every *published*
// agent (from the catalog) and overlays its live deployment state. An agent
// with no deployment shows a Deploy button; a deployed one shows its
// workload status + management actions. This keeps the page in sync with the
// Catalog and the sidebar "Agents" count (both registry-based).
export default function Agents() {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [confirmDelete, setConfirmDelete] = useState<AgentDeployment | null>(null);
  const [invokeFor, setInvokeFor] = useState<string | null>(null);
  // deployKey = "<name>::<version>" preselects the agent in the deploy drawer.
  // "" opens the drawer with no preset (the header Deploy button).
  const [deployKey, setDeployKey] = useState<string | null>(null);
  const [publishOpen, setPublishOpen] = useState(false);

  const { data: regData, isLoading: regLoading } = useQuery({
    queryKey: ["registry-agents", { latest: true }],
    queryFn: () => listRegistryAgents({ latest: true }),
  });
  const registryAgents: RegistryAgent[] = regData?.agents ?? [];

  const { data: depData } = useQuery({
    queryKey: ["agent-deployments"],
    queryFn: listAgents,
    refetchInterval: 5_000,
  });
  const deployments: AgentDeployment[] = depData?.agents ?? [];

  const { data: ucRes } = useQuery({
    queryKey: ["use-cases", "for-agents-page"],
    queryFn: () => listUseCases(),
  });
  const useCaseById = useMemo(() => {
    const m = new Map<string, UseCase>();
    for (const u of ucRes?.use_cases ?? []) m.set(u.id, u);
    return m;
  }, [ucRes]);

  // Group deployments under the registry agent they were launched from.
  // Match on resource_name so any deployed version counts toward the agent.
  const deploymentsByName = useMemo(() => {
    const m = new Map<string, AgentDeployment[]>();
    for (const d of deployments) {
      const arr = m.get(d.resource_name) ?? [];
      arr.push(d);
      m.set(d.resource_name, arr);
    }
    return m;
  }, [deployments]);

  const refreshMutation = useMutation({
    mutationFn: (id: string) => refreshAgentStatus(id),
    onSuccess: (a) => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({ title: `Status: ${a.status}`, description: a.status_reason });
    },
    onError: (e: any) =>
      toast({ title: "Refresh failed", description: e?.response?.data?.error ?? e.message, variant: "destructive" }),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteAgent(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      setConfirmDelete(null);
      toast({ title: "Agent undeployed" });
    },
    onError: (e: any) =>
      toast({ title: "Undeploy failed", description: e?.response?.data?.error ?? e.message, variant: "destructive" }),
  });

  const bumpOOMMutation = useMutation({
    mutationFn: (id: string) => bumpAgentOOM(id),
    onSuccess: (a) => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({ title: "Memory bumped", description: `New limit: ${a.oom_memory_override ?? "baseline"}` });
    },
    onError: (e: any) =>
      toast({ title: "Bump failed", description: e?.response?.data?.error ?? e.message, variant: "destructive" }),
  });

  const resetOOMMutation = useMutation({
    mutationFn: (id: string) => resetAgentOOM(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({ title: "Memory override cleared" });
    },
    onError: (e: any) =>
      toast({ title: "Reset failed", description: e?.response?.data?.error ?? e.message, variant: "destructive" }),
  });

  // --- derived stats --------------------------------------------------------
  const stats = useMemo(() => {
    const deployedNames = new Set(deployments.map((d) => d.resource_name));
    const deployed = registryAgents.filter((a) => deployedNames.has(a.name)).length;
    const running = deployments.filter((d) => d.status === "running").length;
    const failed = deployments.filter((d) => d.status === "failed").length;
    const pods = deployments.reduce((n, d) => n + (d.replicas || 1), 0);
    const useCasesCovered = new Set(
      deployments.map((d) => d.use_case_id).filter(Boolean),
    ).size;
    return {
      published: registryAgents.length,
      deployed,
      running,
      failed,
      pods,
      useCasesCovered,
    };
  }, [registryAgents, deployments]);

  if (regLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
      </div>
    );
  }

  const headerActions = (
    <>
      <Button variant="outline" onClick={() => setDeployKey("")}>
        <Icon icon={icons.rocket} className="mr-2 size-4" />
        Deploy
      </Button>
      <Button onClick={() => setPublishOpen(true)}>
        <Icon icon={icons.plus} className="mr-2 size-4" />
        Add agent
      </Button>
    </>
  );

  return (
    <PageShell
      title="Agents"
      description="Published agents and their live deployments. Each running pod is scoped to an approved use case with an auto-provisioned key that cannot escape its binding."
      actions={headerActions}
    >
      {registryAgents.length === 0 ? (
        <EmptyState
          icon={icons.agents}
          title="No agents yet"
          description="Publish an agent to the registry, then deploy it bound to a use case. Its scoped API key is provisioned automatically."
          action={
            <Button onClick={() => setPublishOpen(true)}>
              <Icon icon={icons.plus} className="mr-2 size-4" />
              Add your first agent
            </Button>
          }
        />
      ) : (
        <>
          {/* ────────────── KPI cards ────────────── */}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
            <StatTile
              label="Agents"
              value={stats.published}
              icon={<Icon icon={icons.agents} />}
              iconColor="agent"
              sub="Published in the catalog"
            />
            <StatTile
              label="Deployed"
              value={stats.deployed}
              tone={stats.deployed > 0 ? "good" : "default"}
              icon={<Icon icon={icons.rocket} />}
              iconColor="good"
              sub={`${stats.published - stats.deployed} not deployed`}
            />
            <StatTile
              label="Running"
              value={stats.running}
              tone={stats.running > 0 ? "good" : "default"}
              icon={<Icon icon={icons.activity} />}
              iconColor="mcp"
              sub={stats.pods > 0 ? `${stats.pods} pods` : undefined}
            />
            <StatTile
              label="Failed"
              value={stats.failed}
              tone={stats.failed > 0 ? "bad" : "default"}
              icon={<Icon icon={icons.warning} />}
              iconColor="bad"
            />
            <StatTile
              label="Use cases"
              value={stats.useCasesCovered}
              icon={<Icon icon={icons.useCase} />}
              iconColor="primary"
              sub={stats.useCasesCovered === 1 ? "single binding" : "across bindings"}
            />
          </div>

          {/* ────────────── Agent grid ────────────── */}
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {registryAgents.map((agent) => (
              <AgentCard
                key={agent.id}
                agent={agent}
                deployments={deploymentsByName.get(agent.name) ?? []}
                useCaseById={useCaseById}
                onDeploy={() => setDeployKey(`${agent.name}::${agent.version}`)}
                onInvoke={(workload) => setInvokeFor(workload)}
                onRefresh={(id) => refreshMutation.mutate(id)}
                onDelete={(d) => setConfirmDelete(d)}
                onBumpOOM={(id) => bumpOOMMutation.mutate(id)}
                onResetOOM={(id) => resetOOMMutation.mutate(id)}
              />
            ))}
          </div>
        </>
      )}

      <InvokeDialog slug={invokeFor} onClose={() => setInvokeFor(null)} />
      <DeployDialog
        open={deployKey !== null}
        presetAgentKey={deployKey || undefined}
        onClose={() => setDeployKey(null)}
      />
      <PublishDialog kind="agents" open={publishOpen} onOpenChange={setPublishOpen} />

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
              Inbound invocations to this workload will fail. The agent stays in
              the catalog.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => confirmDelete && deleteMutation.mutate(confirmDelete.id)}
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
// Agent card — one published registry agent + its deployment state.
// ---------------------------------------------------------------------------

function AgentCard({
  agent,
  deployments,
  useCaseById,
  onDeploy,
  onInvoke,
  onRefresh,
  onDelete,
  onBumpOOM,
  onResetOOM,
}: {
  agent: RegistryAgent;
  deployments: AgentDeployment[];
  useCaseById: Map<string, UseCase>;
  onDeploy: () => void;
  onInvoke: (workload: string) => void;
  onRefresh: (id: string) => void;
  onDelete: (d: AgentDeployment) => void;
  onBumpOOM: (id: string) => void;
  onResetOOM: (id: string) => void;
}) {
  const shortName = agent.name.includes("/")
    ? agent.name.slice(agent.name.lastIndexOf("/") + 1)
    : agent.name;
  const meta = [agent.framework, agent.model_name].filter(Boolean);
  const hasFailed = deployments.some((d) => d.status === "failed");

  return (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-[14px] border bg-card p-4 shadow-sm transition-colors",
        hasFailed ? "border-bad/30" : "border-border",
      )}
    >
      {/* Header */}
      <div className="flex items-start gap-3">
        <KindIcon kind="agent" name={agent.name} size="md" />
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline gap-2">
            <h3 className="truncate text-[15px] font-semibold leading-tight">
              {agent.title || shortName}
            </h3>
            <code className="shrink-0 font-mono text-[10.5px] tabular-nums text-text-dim">
              v{agent.version}
            </code>
          </div>
          <code className="mt-0.5 block truncate font-mono text-[11px] text-text-dim">
            {agent.name}
          </code>
        </div>
      </div>

      {agent.description && (
        <p className="line-clamp-2 text-[13px] leading-relaxed text-muted-foreground">
          {agent.description}
        </p>
      )}

      {meta.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5">
          {agent.framework && <MetaChip icon={icons.deployment}>{agent.framework}</MetaChip>}
          {agent.model_name && <MetaChip icon={icons.models}>{agent.model_name}</MetaChip>}
        </div>
      )}

      {/* Deployment state */}
      <div className="mt-auto border-t border-border pt-3">
        {deployments.length === 0 ? (
          <div className="flex items-center justify-between gap-2">
            <span className="inline-flex items-center gap-1.5 text-[12px] text-text-dim">
              <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/40" />
              Not deployed
            </span>
            <Button size="sm" variant="outline" className="h-7 gap-1.5 px-2.5 text-[11px]" onClick={onDeploy}>
              <Icon icon={icons.rocket} className="h-3 w-3" />
              Deploy
            </Button>
          </div>
        ) : (
          <div className="space-y-2">
            {deployments.map((d) => (
              <DeploymentLine
                key={d.id}
                d={d}
                useCase={useCaseById.get(d.use_case_id)}
                onInvoke={() => onInvoke(d.workload_name)}
                onRefresh={() => onRefresh(d.id)}
                onDelete={() => onDelete(d)}
                onBumpOOM={() => onBumpOOM(d.id)}
                onResetOOM={() => onResetOOM(d.id)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// One deployed workload row inside an agent card.
function DeploymentLine({
  d,
  useCase,
  onInvoke,
  onRefresh,
  onDelete,
  onBumpOOM,
  onResetOOM,
}: {
  d: AgentDeployment;
  useCase: UseCase | undefined;
  onInvoke: () => void;
  onRefresh: () => void;
  onDelete: () => void;
  onBumpOOM: () => void;
  onResetOOM: () => void;
}) {
  const isRunning = d.status === "running";
  const hasOverride = !!d.oom_memory_override;
  return (
    <div className="flex items-center gap-2">
      <StatusPill status={d.status} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5 text-[11.5px] text-muted-foreground">
          <Icon icon={icons.useCase} className="h-3 w-3 shrink-0" />
          <span className="truncate">{useCase?.name ?? "unbound"}</span>
          <span className="text-text-dim">·</span>
          <span className="font-mono tabular-nums text-text-dim">
            {d.replicas}× {d.replicas === 1 ? "pod" : "pods"}
          </span>
        </div>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground">
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
  );
}

// ---------------------------------------------------------------------------
// Small reused bits
// ---------------------------------------------------------------------------

function StatusPill({ status }: { status: AgentDeployment["status"] }) {
  const cls: Record<string, string> = {
    running: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-emerald-500/20",
    deploying: "bg-sky-500/10 text-sky-700 dark:text-sky-400 ring-sky-500/20",
    failed: "bg-rose-500/10 text-rose-700 dark:text-rose-400 ring-rose-500/20",
    pending: "bg-muted text-muted-foreground ring-border",
    terminating: "bg-amber-500/10 text-amber-700 dark:text-amber-400 ring-amber-500/20",
    stopped: "bg-muted text-muted-foreground ring-border",
  };
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1 h-5 px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
        cls[status] ?? "bg-muted text-muted-foreground ring-border",
      )}
    >
      {status === "running" && (
        <span className="relative flex h-1.5 w-1.5">
          <span className="absolute inset-0 rounded-full bg-emerald-500/50 animate-ping" />
          <span className="relative h-1.5 w-1.5 rounded-full bg-emerald-500" />
        </span>
      )}
      {status}
    </span>
  );
}

function MetaChip({ icon, children }: { icon: string; children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded bg-surface-2 text-muted-foreground ring-1 ring-border text-[10.5px] font-mono">
      <Icon icon={icon} className="h-2.5 w-2.5" />
      {children}
    </span>
  );
}
