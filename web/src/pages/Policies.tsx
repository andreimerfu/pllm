import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  getLifecycleStatus,
  runLifecycle,
  listCustomRules,
  deleteCustomRule,
  updateCustomRule,
} from "@/lib/api";
import type {
  LifecycleAction,
  LifecycleActionType,
  LifecycleCustomRule,
  LifecycleRuleResult,
  LifecycleScanTarget,
  LifecycleSummary,
} from "@/types/api";

import { PageShell, StatStrip, StatStripItem } from "@/components/common/page-shell";
import { RuleBuilder } from "@/components/lifecycle/rule-builder";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
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
import { useToast } from "@/hooks/use-toast";
import { GovernanceHelp } from "@/components/governance/help-popover";

// ---------------------------------------------------------------------------
// Policies — automated controls that act on the platform.
//
// Two flavors share the same model:
//   • built-in: ship with pLLM (inactive_keys, ownerless_keys, risky_servers)
//   • custom:   operator-authored from a CEL template
//
// Both run on the lifecycle daemon's tick, both produce LifecycleAction
// rows, both can run in dry-run. We merge them into one normalized
// shape so the catalog reads as one mental model and the visual
// language stays consistent.
// ---------------------------------------------------------------------------

type Origin = "built-in" | "custom";

interface NormalizedPolicy {
  // Stable key for React + filter selection.
  id: string;
  origin: Origin;
  title: string;
  // Slug-style identifier for the eyebrow row above the title.
  slug: string;
  description: string;
  scan: LifecycleScanTarget;
  action: LifecycleActionType;
  // Last-run telemetry from the lifecycle summary. May be missing if a
  // built-in didn't run yet or a custom rule was created after the last
  // tick — surface as "—" rather than zero so it's clear we don't know.
  ranThisTick: boolean;
  scanned: number;
  actions: LifecycleAction[];
  error?: string;
  // Built-ins are always enabled; custom rules carry a flag.
  enabled: boolean;
  // null on built-ins (always enforcing); on custom: null = inherit
  // global default, true = dry-run, false = enforcing.
  dryRun: boolean | null;
  // For edit/delete shortcuts — only present on custom rules.
  raw?: LifecycleCustomRule;
}

// Built-in metadata. Keys mirror the backend rule names from
// internal/services/lifecycle/*.
const BUILTIN_META: Record<
  string,
  {
    title: string;
    description: string;
    scan: LifecycleScanTarget;
    action: LifecycleActionType;
  }
> = {
  inactive_keys: {
    title: "Revoke inactive keys",
    description:
      "Revokes API keys with no activity for ≥90 days. Master keys are exempt — they bypass scoping by design.",
    scan: "keys",
    action: "revoke",
  },
  ownerless_keys: {
    title: "Revoke ownerless keys",
    description:
      "Revokes keys whose owner User is deactivated or soft-deleted. Team-only keys are not affected.",
    scan: "keys",
    action: "revoke",
  },
  risky_servers: {
    title: "Block risky servers",
    description:
      "Blocks registry servers with an OSV vulnerability score below the configured floor. Operators clear blocks manually.",
    scan: "registry_servers",
    action: "block",
  },
};

// Per-scan-target tone — keeps the visual language consistent with the
// rule builder's template picker.
const SCAN_META: Record<
  LifecycleScanTarget,
  { label: string; icon: string; tone: string; bg: string; ring: string }
> = {
  keys: {
    label: "Keys",
    icon: "solar:key-minimalistic-linear",
    tone: "text-amber-600 dark:text-amber-400",
    bg: "bg-amber-500/10",
    ring: "ring-amber-500/30",
  },
  registry_servers: {
    label: "MCP Servers",
    icon: "mingcute:mcp-line",
    tone: "text-violet-600 dark:text-violet-400",
    bg: "bg-violet-500/10",
    ring: "ring-violet-500/30",
  },
  use_cases: {
    label: "Use cases",
    icon: icons.useCase,
    tone: "text-emerald-600 dark:text-emerald-400",
    bg: "bg-emerald-500/10",
    ring: "ring-emerald-500/30",
  },
  users: {
    label: "Users",
    icon: "solar:user-linear",
    tone: "text-sky-600 dark:text-sky-400",
    bg: "bg-sky-500/10",
    ring: "ring-sky-500/30",
  },
};

const ACTION_BADGE: Record<LifecycleActionType, string> = {
  revoke: "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
  block: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
  flag: "bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-violet-500/30",
  notify: "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
};

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

type FilterTab = "all" | "built-in" | "custom";

export default function Policies() {
  const { toast } = useToast();
  const qc = useQueryClient();

  const { data: summary, isLoading: summaryLoading } = useQuery({
    queryKey: ["lifecycle-status"],
    queryFn: getLifecycleStatus,
  });

  const { data: customData, isLoading: customLoading } = useQuery({
    queryKey: ["lifecycle-custom-rules"],
    queryFn: listCustomRules,
  });

  const runMutation = useMutation({
    mutationFn: runLifecycle,
    onSuccess: (next: LifecycleSummary) => {
      qc.setQueryData(["lifecycle-status"], next);
      toast({
        title: "Run complete",
        description: `${next.total_actions} action${next.total_actions === 1 ? "" : "s"} across ${next.rules?.length ?? 0} polic${(next.rules?.length ?? 0) === 1 ? "y" : "ies"}.`,
      });
    },
    onError: (e: { response?: { data?: { error?: string } }; message?: string }) => {
      toast({
        title: "Run failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const policies = useMemo(
    () => buildPolicies(summary, customData?.rules ?? []),
    [summary, customData],
  );

  const stats = useMemo(() => derivePolicyStats(policies, summary), [policies, summary]);

  const [filter, setFilter] = useState<FilterTab>("all");
  const [search, setSearch] = useState("");

  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return policies.filter((p) => {
      if (filter !== "all" && p.origin !== filter) return false;
      if (!q) return true;
      return (
        p.title.toLowerCase().includes(q) ||
        p.slug.toLowerCase().includes(q) ||
        p.description.toLowerCase().includes(q)
      );
    });
  }, [policies, filter, search]);

  const [builderOpen, setBuilderOpen] = useState(false);
  const [editing, setEditing] = useState<LifecycleCustomRule | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<LifecycleCustomRule | null>(null);

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteCustomRule(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["lifecycle-custom-rules"] });
      toast({ title: "Policy deleted" });
      setConfirmDelete(null);
    },
    onError: (e: { response?: { data?: { error?: string } }; message?: string }) =>
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e?.message ?? "unknown error",
        variant: "destructive",
      }),
  });

  const toggleMut = useMutation({
    mutationFn: ({ rule, enabled }: { rule: LifecycleCustomRule; enabled: boolean }) =>
      updateCustomRule(rule.id, {
        name: rule.name,
        description: rule.description,
        template_id: rule.template_id,
        params: (rule.params as Record<string, unknown>) ?? {},
        enabled,
        dry_run: rule.dry_run ?? null,
        reason: rule.reason,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["lifecycle-custom-rules"] });
    },
  });

  const ranAt =
    summary?.ran_at && !summary.ran_at.startsWith("0001-")
      ? new Date(summary.ran_at)
      : null;
  const durationMs = summary ? Math.round(summary.duration / 1_000_000) : 0;
  const isLoading = summaryLoading || customLoading;

  return (
    <PageShell
      title="Policies"
      description="Automated controls that revoke, block, or flag based on activity. Built-in policies ship with pLLM; custom policies you author from a template."
      actions={
        <>
          <GovernanceHelp accent="audit" />
          <Button
            onClick={() => {
              setEditing(null);
              setBuilderOpen(true);
            }}
            size="sm"
            variant="outline"
            className="gap-1.5"
          >
            <Icon icon={icons.plus} className="h-3.5 w-3.5" />
            New policy
          </Button>
          <Button
            onClick={() => runMutation.mutate()}
            disabled={runMutation.isPending}
            size="sm"
            className="gap-1.5"
          >
            <Icon icon={icons.rocket} className="h-3.5 w-3.5" />
            {runMutation.isPending ? "Running…" : "Run now"}
          </Button>
        </>
      }
      stats={
        <StatStrip className="col-span-full">
          <StatStripItem label="Policies" value={stats.total} />
          <StatStripItem
            label="Built-in"
            value={stats.builtIn}
            hint="ship with pLLM"
          />
          <StatStripItem
            label="Custom"
            value={stats.custom}
            hint={stats.customDisabled > 0 ? `${stats.customDisabled} paused` : "all active"}
          />
          <StatStripItem
            label="Last run"
            value={ranAt ? formatDistanceToNow(ranAt, { addSuffix: true }) : "never"}
            hint={ranAt ? `${durationMs}ms` : "click run now"}
          />
          <StatStripItem
            label="Actions taken"
            value={summary?.total_actions ?? 0}
            tone={(summary?.total_actions ?? 0) > 0 ? "amber" : "default"}
            hint="last run"
          />
        </StatStrip>
      }
      toolbar={
        <div className="flex items-center gap-2 w-full">
          <FilterTabs value={filter} onChange={setFilter} stats={stats} />
          <div className="relative w-64 max-w-full ml-auto">
            <Icon
              icon={icons.search}
              className="h-4 w-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search policies…"
              className="h-9 pl-8"
            />
          </div>
        </div>
      }
    >
      {isLoading ? (
        <div className="rounded-lg border border-border bg-card p-12 text-center">
          <div className="animate-spin mx-auto h-6 w-6 rounded-full border-b-2 border-primary" />
          <p className="text-sm text-muted-foreground mt-3">Loading policies…</p>
        </div>
      ) : visible.length === 0 ? (
        policies.length === 0 ? (
          <NeverRunState onRun={() => runMutation.mutate()} running={runMutation.isPending} />
        ) : (
          <div className="rounded-lg border border-dashed border-border p-10 text-center">
            <p className="text-sm text-muted-foreground">
              No policies match this filter.
            </p>
          </div>
        )
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
          {visible.map((p) => (
            <PolicyCard
              key={p.id}
              policy={p}
              onEdit={() => {
                if (p.raw) {
                  setEditing(p.raw);
                  setBuilderOpen(true);
                }
              }}
              onDelete={() => p.raw && setConfirmDelete(p.raw)}
              onToggle={(enabled) =>
                p.raw && toggleMut.mutate({ rule: p.raw, enabled })
              }
            />
          ))}
        </div>
      )}

      <RuleBuilder
        open={builderOpen}
        onOpenChange={(o) => {
          setBuilderOpen(o);
          if (!o) setEditing(null);
        }}
        existing={editing}
      />

      <AlertDialog
        open={!!confirmDelete}
        onOpenChange={(o) => !o && setConfirmDelete(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this policy?</AlertDialogTitle>
            <AlertDialogDescription>
              {confirmDelete?.name} will stop running on the next tick. Past
              actions stay in the audit log.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => confirmDelete && deleteMut.mutate(confirmDelete.id)}
              className="bg-rose-600 hover:bg-rose-700 text-white"
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}

// ---------------------------------------------------------------------------
// Normalization — merge built-in + custom into one shape.
//
// Built-ins drive the order: their stable identity is the rule slug
// (`inactive_keys` etc.). If a built-in didn't appear in the last
// summary at all, we still surface it with ranThisTick=false so the
// catalog stays predictable.
// ---------------------------------------------------------------------------

function buildPolicies(
  summary: LifecycleSummary | undefined,
  customRules: LifecycleCustomRule[],
): NormalizedPolicy[] {
  const out: NormalizedPolicy[] = [];

  // Index custom rules by their stable id from the last summary so we can
  // attach last-run telemetry.
  const summaryById: Record<string, LifecycleRuleResult> = {};
  for (const r of summary?.rules ?? []) {
    summaryById[r.rule] = r;
  }

  // Built-ins first, in defined order so the page reads the same every load.
  for (const slug of Object.keys(BUILTIN_META)) {
    const meta = BUILTIN_META[slug];
    const r = summaryById[slug];
    out.push({
      id: `builtin:${slug}`,
      origin: "built-in",
      title: meta.title,
      slug,
      description: meta.description,
      scan: meta.scan,
      action: meta.action,
      ranThisTick: !!r,
      scanned: r?.scanned ?? 0,
      actions: r?.actions ?? [],
      error: r?.error,
      enabled: true,
      dryRun: null,
    });
  }

  // Custom rules — last-run info keyed on the custom-rule id which the
  // backend uses as the rule slug for custom-policy summary entries.
  for (const c of customRules) {
    const r = summaryById[c.id];
    out.push({
      id: `custom:${c.id}`,
      origin: "custom",
      title: c.name,
      slug: c.template_id || c.scan,
      description: c.description || "—",
      scan: c.scan,
      action: c.action,
      ranThisTick: !!r,
      scanned: r?.scanned ?? 0,
      actions: r?.actions ?? [],
      error: r?.error,
      enabled: c.enabled,
      dryRun: c.dry_run ?? null,
      raw: c,
    });
  }

  return out;
}

function derivePolicyStats(
  policies: NormalizedPolicy[],
  _summary: LifecycleSummary | undefined,
) {
  const builtIn = policies.filter((p) => p.origin === "built-in").length;
  const custom = policies.filter((p) => p.origin === "custom").length;
  const customDisabled = policies.filter(
    (p) => p.origin === "custom" && !p.enabled,
  ).length;
  return { total: policies.length, builtIn, custom, customDisabled };
}

// ---------------------------------------------------------------------------
// Filter tabs — pill segment control. Counts on each tab.
// ---------------------------------------------------------------------------

function FilterTabs({
  value,
  onChange,
  stats,
}: {
  value: FilterTab;
  onChange: (v: FilterTab) => void;
  stats: { total: number; builtIn: number; custom: number };
}) {
  const tabs: { id: FilterTab; label: string; count: number }[] = [
    { id: "all", label: "All", count: stats.total },
    { id: "built-in", label: "Built-in", count: stats.builtIn },
    { id: "custom", label: "Custom", count: stats.custom },
  ];
  return (
    <div className="inline-flex items-center rounded-md ring-1 ring-border overflow-hidden bg-card">
      {tabs.map((t) => (
        <button
          key={t.id}
          onClick={() => onChange(t.id)}
          className={cn(
            "text-[11px] font-mono uppercase tracking-wider h-9 px-3 transition flex items-center gap-1.5",
            value === t.id
              ? "bg-foreground text-background"
              : "bg-transparent text-muted-foreground hover:bg-foreground/5",
          )}
        >
          <span>{t.label}</span>
          <span
            className={cn(
              "tabular-nums text-[10px]",
              value === t.id ? "text-background/70" : "text-muted-foreground/70",
            )}
          >
            {t.count}
          </span>
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Card — policy at a glance.
// ---------------------------------------------------------------------------

function PolicyCard({
  policy,
  onEdit,
  onDelete,
  onToggle,
}: {
  policy: NormalizedPolicy;
  onEdit: () => void;
  onDelete: () => void;
  onToggle: (enabled: boolean) => void;
}) {
  const scan = SCAN_META[policy.scan];
  const actionsCount = policy.actions.length;
  const hasError = !!policy.error;
  const dimmed = !policy.enabled;

  return (
    <div
      className={cn(
        "group relative rounded-lg border bg-card p-4 flex flex-col gap-3 transition",
        "hover:border-foreground/30",
        dimmed
          ? "border-dashed border-border opacity-60"
          : "border-border",
      )}
    >
      {/* ── Header: scan icon + title + origin pill ── */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-start gap-2.5 min-w-0">
          <div
            className={cn(
              "h-8 w-8 rounded-md flex items-center justify-center shrink-0 ring-1",
              scan.bg,
              scan.ring,
            )}
          >
            <Icon icon={scan.icon} className={cn("h-4 w-4", scan.tone)} />
          </div>
          <div className="min-w-0 pt-0.5">
            <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              <span className="truncate">{policy.slug}</span>
              <span className="text-border">·</span>
              <span>{scan.label}</span>
            </div>
            <h3 className="text-sm font-semibold tracking-tight mt-0.5 truncate">
              {policy.title}
            </h3>
          </div>
        </div>
        <OriginPill origin={policy.origin} />
      </div>

      {/* ── Description ── */}
      <p className="text-[11.5px] leading-relaxed text-muted-foreground line-clamp-2">
        {policy.description}
      </p>

      {/* ── Status row: action verb · last run state ── */}
      <div className="flex items-center gap-1.5 flex-wrap">
        <span
          className={cn(
            "inline-flex items-center h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
            ACTION_BADGE[policy.action],
          )}
        >
          {policy.action}
        </span>
        {policy.dryRun === true && (
          <span className="inline-flex items-center h-[18px] px-1.5 rounded bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30 text-[10px] font-mono uppercase tracking-wider">
            dry-run
          </span>
        )}
        {hasError ? (
          <span
            className="inline-flex items-center gap-1 h-[18px] px-1.5 rounded bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-1 ring-rose-500/30 text-[10px] font-mono uppercase tracking-wider"
            title={policy.error}
          >
            <span className="h-1.5 w-1.5 rounded-full bg-rose-500" />
            error
          </span>
        ) : !policy.ranThisTick ? (
          <span className="inline-flex items-center h-[18px] px-1.5 rounded bg-muted text-muted-foreground ring-1 ring-border text-[10px] font-mono uppercase tracking-wider">
            not yet run
          </span>
        ) : actionsCount > 0 ? (
          <span className="inline-flex items-center h-[18px] px-1.5 rounded bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30 text-[10px] font-mono uppercase tracking-wider">
            {actionsCount} action{actionsCount === 1 ? "" : "s"}
          </span>
        ) : (
          <span className="inline-flex items-center gap-1 h-[18px] px-1.5 rounded bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-1 ring-emerald-500/30 text-[10px] font-mono uppercase tracking-wider">
            <Icon icon={icons.check} className="h-2.5 w-2.5" />
            clean
          </span>
        )}
      </div>

      {/* ── Counters: scanned · acted on ── */}
      <div className="flex items-center gap-3 text-[11px] font-mono tabular-nums border-t border-border/50 pt-2.5">
        <Counter label="scanned" value={policy.scanned} />
        <span className="text-border">·</span>
        <Counter
          label="acted on"
          value={actionsCount}
          tone={actionsCount > 0 ? "amber" : "default"}
        />
        {!policy.ranThisTick && (
          <span className="ml-auto text-[10px] text-muted-foreground/60 font-mono uppercase tracking-wider">
            awaiting tick
          </span>
        )}
      </div>

      {/* ── Recent action sample (if any) — collapsed, scrollable ── */}
      {policy.actions.length > 0 && (
        <details className="group/details">
          <summary className="cursor-pointer text-[10px] font-mono uppercase tracking-widest text-muted-foreground hover:text-foreground select-none flex items-center gap-1">
            <Icon
              icon={icons.chevronRight}
              className="h-3 w-3 transition-transform group-open/details:rotate-90"
            />
            see what was acted on
          </summary>
          <ul className="space-y-1.5 max-h-48 overflow-y-auto pr-1 mt-2">
            {policy.actions.map((a) => (
              <li
                key={a.resource_id}
                className="text-[12px] py-1 border-b border-border/30 last:border-0"
              >
                <div className="flex items-center justify-between gap-2">
                  <code className="font-mono truncate text-foreground/85">
                    {a.subject}
                  </code>
                  {a.dry_run && (
                    <span className="inline-flex items-center h-[16px] px-1.5 rounded bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-1 ring-amber-500/30 text-[9px] font-mono uppercase tracking-wider shrink-0">
                      dry-run
                    </span>
                  )}
                </div>
                <div className="text-muted-foreground text-[11px] truncate">
                  {a.reason}
                </div>
              </li>
            ))}
          </ul>
        </details>
      )}

      {/* ── Footer: toggle (custom only) + edit/delete ── */}
      <div className="flex items-center justify-between gap-2 pt-1 mt-auto">
        {policy.origin === "custom" ? (
          <label className="flex items-center gap-2 cursor-pointer">
            <Switch checked={policy.enabled} onCheckedChange={onToggle} />
            <span className="text-[11px] text-muted-foreground">
              {policy.enabled ? "Active" : "Paused"}
            </span>
          </label>
        ) : (
          <span className="text-[11px] text-muted-foreground inline-flex items-center gap-1">
            <Icon icon={icons.lock} className="h-3 w-3" />
            built-in · always active
          </span>
        )}

        {policy.origin === "custom" && (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={onEdit}
              className="h-7 gap-1 px-2"
            >
              <Icon icon={icons.edit} className="h-3.5 w-3.5" />
              <span className="text-[11px]">Edit</span>
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={onDelete}
              className="h-7 gap-1 px-2 text-rose-600 hover:text-rose-700 hover:bg-rose-500/10"
            >
              <Icon icon={icons.delete} className="h-3.5 w-3.5" />
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

function OriginPill({ origin }: { origin: Origin }) {
  return origin === "built-in" ? (
    <span className="inline-flex items-center gap-1 h-[18px] px-1.5 rounded bg-slate-500/10 text-slate-600 dark:text-slate-300 ring-1 ring-slate-500/30 text-[10px] font-mono uppercase tracking-wider shrink-0">
      <Icon icon={icons.lock} className="h-2.5 w-2.5" />
      built-in
    </span>
  ) : (
    <span className="inline-flex items-center gap-1 h-[18px] px-1.5 rounded bg-indigo-500/10 text-indigo-600 dark:text-indigo-300 ring-1 ring-indigo-500/30 text-[10px] font-mono uppercase tracking-wider shrink-0">
      <Icon icon={icons.user} className="h-2.5 w-2.5" />
      custom
    </span>
  );
}

function Counter({
  label,
  value,
  tone = "default",
}: {
  label: string;
  value: number;
  tone?: "default" | "amber";
}) {
  return (
    <div className="flex items-baseline gap-1">
      <span className="text-muted-foreground/70 uppercase tracking-wider text-[9px]">
        {label}
      </span>
      <span
        className={cn(
          "font-semibold",
          tone === "amber"
            ? "text-amber-600 dark:text-amber-400"
            : "text-foreground",
        )}
      >
        {value}
      </span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Empty state for "never run yet AND no custom rules"
// ---------------------------------------------------------------------------

function NeverRunState({
  onRun,
  running,
}: {
  onRun: () => void;
  running: boolean;
}) {
  return (
    <div className="rounded-lg border border-dashed border-border bg-card/30 p-12 text-center">
      <div className="mx-auto h-12 w-12 rounded-full bg-muted flex items-center justify-center mb-3">
        <Icon icon={icons.policies} className="h-6 w-6 text-muted-foreground" />
      </div>
      <h3 className="text-base font-semibold tracking-tight">
        No policies yet
      </h3>
      <p className="text-sm text-muted-foreground mt-1 max-w-md mx-auto">
        Click <span className="font-mono">Run now</span> to evaluate the
        built-in policies, or author your own from a template.
      </p>
      <Button
        onClick={onRun}
        disabled={running}
        size="sm"
        className="mt-5 gap-1.5"
      >
        <Icon icon={icons.rocket} className="h-3.5 w-3.5" />
        {running ? "Running…" : "Run built-in policies"}
      </Button>
    </div>
  );
}
