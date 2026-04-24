import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { deployAgent, listRegistryAgents, listUseCases } from "@/lib/api";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { RegistryAgent, UseCase } from "@/types/api";

import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { useToast } from "@/hooks/use-toast";

type Props = {
  open: boolean;
  onClose: () => void;
};

const REPLICAS_MIN = 1;
const REPLICAS_MAX = 5;

// Risk-tier palette — matches UseCases so an operator scanning the drawer
// sees the same tone for "Tier 1" as they do on the governance dashboards.
const TIER_TONES: Record<string, string> = {
  tier1: "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
  tier2: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
  tier3: "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
  tier4: "bg-muted text-muted-foreground ring-border",
};

/**
 * DeployDialog — right-side drawer for deploying a registry agent to K8s.
 *
 * The flow is minimal by design: agents must already be published, use
 * cases must already exist, and we just bind the two. Replicas is the
 * only numeric knob (memory / CPU come from the cluster defaults plus
 * the OOM auto-scaler).
 */
export function DeployDialog({ open, onClose }: Props) {
  const { toast } = useToast();
  const qc = useQueryClient();

  const [agentKey, setAgentKey] = useState<string>("");
  const [useCaseId, setUseCaseId] = useState<string>("");
  const [replicas, setReplicas] = useState<number>(1);

  const { data: agentsRes, isLoading: agentsLoading } = useQuery({
    queryKey: ["registry-agents", "for-deploy"],
    queryFn: () => listRegistryAgents({ latest: true }),
    enabled: open,
  });
  const agents: RegistryAgent[] = agentsRes?.agents ?? [];

  const { data: ucRes, isLoading: ucLoading } = useQuery({
    queryKey: ["use-cases", "for-deploy"],
    queryFn: () => listUseCases(),
    enabled: open,
  });
  const useCases: UseCase[] = ucRes?.use_cases ?? [];

  const selectedAgent = useMemo(() => {
    if (!agentKey) return undefined;
    const [name, version] = agentKey.split("::");
    return agents.find((a) => a.name === name && a.version === version);
  }, [agentKey, agents]);

  const selectedUseCase = useMemo(
    () => useCases.find((u) => u.id === useCaseId),
    [useCaseId, useCases],
  );

  const deployMut = useMutation({
    mutationFn: () => {
      if (!selectedAgent) throw new Error("pick an agent");
      if (!useCaseId) throw new Error("pick a use case");
      return deployAgent({
        agent_id: selectedAgent.id,
        use_case_id: useCaseId,
        replicas,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["agent-deployments"] });
      toast({ title: "Agent deployed", description: "Pod is starting up." });
      reset();
      onClose();
    },
    onError: (e: any) => {
      toast({
        title: "Deploy failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  function reset() {
    setAgentKey("");
    setUseCaseId("");
    setReplicas(1);
  }

  function handleOpenChange(o: boolean) {
    if (!o) {
      reset();
      onClose();
    }
  }

  const ready = !!selectedAgent && !!useCaseId && !deployMut.isPending;

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent
        side="right"
        className="w-full sm:max-w-none sm:w-[min(620px,90vw)] lg:w-[720px] p-0 flex flex-col gap-0 bg-background border-l border-border"
      >
        {/* ── Header ─────────────────────────────────────────────── */}
        <SheetHeader className="px-6 pt-5 pb-4 border-b border-border/70 space-y-2 text-left bg-gradient-to-b from-card to-background">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20 flex items-center justify-center shrink-0">
              <Icon icon={icons.rocket} className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <SheetTitle className="text-[15px] font-semibold leading-tight tracking-tight">
                Deploy agent
              </SheetTitle>
              <p className="text-[11.5px] leading-relaxed text-muted-foreground mt-0.5 max-w-lg">
                A scoped API key is provisioned and mounted into the pod as a
                Secret. Runtime scope enforcement tracks the bound use case.
              </p>
            </div>
          </div>
        </SheetHeader>

        {/* ── Body ───────────────────────────────────────────────── */}
        <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
          {/* Agent */}
          <Field
            label="Agent"
            hint={
              selectedAgent
                ? `v${selectedAgent.version}`
                : agents.length === 0 && !agentsLoading
                  ? "No agents have been published yet."
                  : "Pick from the registry's latest-per-name list."
            }
          >
            <Select
              value={agentKey}
              onValueChange={setAgentKey}
              disabled={agentsLoading || agents.length === 0}
            >
              <SelectTrigger className="h-10 bg-card">
                <SelectValue
                  placeholder={
                    agentsLoading
                      ? "Loading agents…"
                      : agents.length === 0
                        ? "No published agents"
                        : "Select an agent"
                  }
                />
              </SelectTrigger>
              <SelectContent className="max-h-72">
                {agents.map((a) => (
                  <SelectItem
                    key={`${a.name}::${a.version}`}
                    value={`${a.name}::${a.version}`}
                    className="py-2.5"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-sm font-medium truncate">
                        {a.title || a.name}
                      </span>
                      <code className="text-[10px] font-mono text-muted-foreground tabular-nums shrink-0">
                        v{a.version}
                      </code>
                    </div>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {selectedAgent?.description && (
              <p className="text-[11.5px] text-muted-foreground/90 leading-relaxed line-clamp-2 bg-muted/40 rounded-md px-2.5 py-1.5 border border-border/60 mt-1.5">
                {selectedAgent.description}
              </p>
            )}
          </Field>

          {/* Use case */}
          <Field
            label="Use case"
            hint={
              selectedUseCase
                ? "Keys provisioned for this agent inherit the use case's scope."
                : useCases.length === 0 && !ucLoading
                  ? "No use cases defined — create one first."
                  : "Every agent runs under a use case. The scoped key cannot escape it."
            }
          >
            <Select
              value={useCaseId}
              onValueChange={setUseCaseId}
              disabled={ucLoading || useCases.length === 0}
            >
              <SelectTrigger className="h-10 bg-card">
                <SelectValue
                  placeholder={
                    ucLoading
                      ? "Loading use cases…"
                      : useCases.length === 0
                        ? "No use cases defined"
                        : "Select a use case"
                  }
                />
              </SelectTrigger>
              <SelectContent className="max-h-72">
                {useCases.map((uc) => (
                  <SelectItem key={uc.id} value={uc.id} className="py-2.5">
                    <div className="flex items-center gap-2 min-w-0 w-full">
                      <span className="text-sm font-medium truncate">
                        {uc.name}
                      </span>
                      {uc.slug && (
                        <code className="text-[10px] font-mono text-muted-foreground tabular-nums shrink-0">
                          {uc.slug}
                        </code>
                      )}
                      <TierBadge tier={uc.risk_tier} className="ml-auto" />
                    </div>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>

          {/* Replicas */}
          <Field
            label="Replicas"
            hint={`How many pods to run. ${REPLICAS_MIN}–${REPLICAS_MAX} per deployment.`}
          >
            <Stepper
              value={replicas}
              onChange={(n) =>
                setReplicas(Math.max(REPLICAS_MIN, Math.min(REPLICAS_MAX, n)))
              }
              min={REPLICAS_MIN}
              max={REPLICAS_MAX}
            />
          </Field>

          {/* Summary */}
          {(selectedAgent || selectedUseCase) && (
            <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-3 space-y-2">
              <div className="text-[9.5px] font-mono uppercase tracking-widest text-muted-foreground">
                Summary
              </div>
              <div className="space-y-1.5">
                {selectedAgent && (
                  <SummaryRow
                    icon={icons.agents}
                    label="Agent"
                    value={selectedAgent.title || selectedAgent.name}
                    suffix={`v${selectedAgent.version}`}
                  />
                )}
                {selectedUseCase && (
                  <SummaryRow
                    icon={icons.useCase}
                    label="Use case"
                    value={selectedUseCase.name}
                    suffix={selectedUseCase.risk_tier?.replace("tier", "T")}
                  />
                )}
                <SummaryRow
                  icon={icons.rocket}
                  label="Replicas"
                  value={`${replicas} pod${replicas === 1 ? "" : "s"}`}
                />
                <SummaryRow
                  icon={icons.keys}
                  label="Scoped key"
                  value="auto-provisioned on deploy"
                  muted
                />
              </div>
            </div>
          )}
        </div>

        {/* ── Footer ─────────────────────────────────────────────── */}
        <div className="px-6 py-3.5 border-t border-border/70 bg-card/60 flex items-center gap-3">
          <div className="text-[10.5px] font-mono uppercase tracking-widest text-muted-foreground/70">
            {ready ? "ready to apply" : "fill all fields"}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <Button
              variant="ghost"
              onClick={() => handleOpenChange(false)}
              disabled={deployMut.isPending}
            >
              Cancel
            </Button>
            <Button
              onClick={() => deployMut.mutate()}
              disabled={!ready}
              className="min-w-[112px]"
            >
              {deployMut.isPending ? (
                <>
                  <Icon
                    icon={icons.refresh}
                    className="mr-2 h-3.5 w-3.5 animate-spin"
                  />
                  Deploying…
                </>
              ) : (
                <>
                  <Icon icon={icons.rocket} className="mr-2 h-3.5 w-3.5" />
                  Deploy
                </>
              )}
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

// ───────────────────────────────────────────────────────────────────
// Field + stepper + summary primitives
// ───────────────────────────────────────────────────────────────────

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <label className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        {label}
      </label>
      {children}
      {hint && (
        <p className="text-[11px] text-muted-foreground/80 leading-relaxed">
          {hint}
        </p>
      )}
    </div>
  );
}

function Stepper({
  value,
  onChange,
  min,
  max,
}: {
  value: number;
  onChange: (n: number) => void;
  min: number;
  max: number;
}) {
  return (
    <div className="flex items-center gap-0 w-fit rounded-md border border-border bg-card overflow-hidden">
      <button
        type="button"
        onClick={() => onChange(value - 1)}
        disabled={value <= min}
        className="h-9 w-9 flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-40 disabled:hover:bg-transparent transition-colors border-r border-border"
        aria-label="Decrease"
      >
        <Icon icon={icons.minus} className="h-3.5 w-3.5" />
      </button>
      <Input
        type="number"
        min={min}
        max={max}
        value={value}
        onChange={(e) => onChange(Number(e.target.value) || min)}
        className="h-9 w-14 text-center tabular-nums border-0 bg-transparent rounded-none focus-visible:ring-0 focus-visible:ring-offset-0 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
      />
      <button
        type="button"
        onClick={() => onChange(value + 1)}
        disabled={value >= max}
        className="h-9 w-9 flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-40 disabled:hover:bg-transparent transition-colors border-l border-border"
        aria-label="Increase"
      >
        <Icon icon={icons.plus} className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

function SummaryRow({
  icon,
  label,
  value,
  suffix,
  muted,
}: {
  icon: string;
  label: string;
  value: string;
  suffix?: string;
  muted?: boolean;
}) {
  return (
    <div className="flex items-baseline gap-3 text-[12px]">
      <span className="inline-flex items-center gap-1.5 w-[6.5rem] text-[10px] font-mono uppercase tracking-widest text-muted-foreground shrink-0">
        <Icon icon={icon} className="h-3 w-3" />
        {label}
      </span>
      <span className={cn("truncate flex-1 min-w-0", muted && "text-muted-foreground italic")}>
        {value}
      </span>
      {suffix && (
        <code className="text-[10px] font-mono text-muted-foreground tabular-nums shrink-0">
          {suffix}
        </code>
      )}
    </div>
  );
}

function TierBadge({ tier, className }: { tier: string; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center h-5 px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider shrink-0",
        TIER_TONES[tier] ?? "bg-muted text-muted-foreground ring-border",
        className,
      )}
    >
      {tier.replace("tier", "T")}
    </span>
  );
}
