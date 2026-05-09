import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { detectProvider } from "@/lib/providers";
import { cn } from "@/lib/utils";
import { listScopeEvents, listUseCases, listWebhooks } from "@/lib/api";
import type { ScopeEvent, UseCase, Webhook } from "@/types/api";

import { PageShell } from "@/components/common/page-shell";
import { DiscoveredAgentsList } from "@/components/discovery/discovered-agents-list";
import { PostureBreakdown, PostureRing } from "@/components/compliance/posture-ring";
import {
  ScopeTimeline,
  type TimelineRange,
} from "@/components/compliance/scope-timeline";
import {
  TargetBars,
  UseCaseBars,
  topTargets,
  topUseCases,
} from "@/components/compliance/top-violations";
import { Button } from "@/components/ui/button";
import { GovernanceHelp } from "@/components/governance/help-popover";
import { InfoTip } from "@/components/governance/info-tip";

// ---------------------------------------------------------------------------
// Compliance — what the runtime saw.
//
// Live-refreshing dashboard surface for the Governance group. Subscriptions
// (webhooks) and Policies (lifecycle + custom rules) used to live here as
// CRUD sections; they now live on their own dedicated pages so this view
// can stay focused on telemetry. Posture rolls up enforcement coverage,
// 24h denial pressure, and webhook delivery health into one number.
// Auto-refreshes every 10s.
// ---------------------------------------------------------------------------

function computePosture(
  useCases: UseCase[],
  stats: { last_24h_denied: number } | undefined,
  webhooks: Webhook[],
) {
  const approved = useCases.filter((u) => u.status === "approved");
  const enforcing = approved.filter((u) => u.enforce_scope).length;
  const enforcementCoverage =
    approved.length === 0 ? 100 : Math.round((enforcing / approved.length) * 100);

  const denials = stats?.last_24h_denied ?? 0;
  const denialScore = Math.max(0, 100 - Math.min(100, denials * 2));

  const enabled = webhooks.filter((w) => w.enabled);
  const healthy = enabled.filter((w) => {
    const s = w.last_delivery_status ?? 0;
    return s >= 200 && s < 300;
  }).length;
  const webhookHealth =
    enabled.length === 0 ? 100 : Math.round((healthy / enabled.length) * 100);

  const score = Math.round(
    0.5 * enforcementCoverage + 0.3 * denialScore + 0.2 * webhookHealth,
  );
  return {
    score,
    enforcementCoverage,
    denialScore,
    webhookHealth,
    approved: approved.length,
    enforcing,
    healthy,
    enabled: enabled.length,
    denials,
  };
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function Compliance() {
  const [range, setRange] = useState<TimelineRange>("7d");
  const sinceISO = useMemo(() => {
    const d = new Date(Date.now() - 7 * 86_400_000);
    return d.toISOString();
  }, []);

  const { data: eventsData, isLoading: eventsLoading } = useQuery({
    queryKey: ["compliance-events-7d"],
    queryFn: () => listScopeEvents({ since: sinceISO, limit: 500 }),
    refetchInterval: 10_000,
  });
  const events: ScopeEvent[] = eventsData?.events ?? [];
  const stats = eventsData?.stats;

  const { data: ucData } = useQuery({
    queryKey: ["compliance-use-cases"],
    queryFn: () => listUseCases(),
  });
  const useCases = ucData?.use_cases ?? [];

  const { data: hookData } = useQuery({
    queryKey: ["webhooks"],
    queryFn: listWebhooks,
    refetchInterval: 10_000,
  });
  const webhooks = hookData?.webhooks ?? [];

  const posture = useMemo(
    () => computePosture(useCases, stats, webhooks),
    [useCases, stats, webhooks],
  );

  const topUCs = useMemo(() => topUseCases(events, useCases, 6), [events, useCases]);
  const topTg = useMemo(() => topTargets(events, 6), [events]);

  const flaggedUCs = useCases.filter((u) => u.compliance_flag).length;

  return (
    <PageShell
      title="Compliance"
      description="What the runtime saw — live scope events, posture across the portfolio, top violators. Manage notifications and policies on their own pages. Auto-refreshes every 10s."
      actions={
        <>
          <GovernanceHelp accent="audit" />
          <ExportCSV events={events} />
        </>
      }
    >
      {/* ────────────── Posture hero ────────────── */}
      <div className="rounded-lg border border-border bg-card p-5">
        <div className="grid grid-cols-1 md:grid-cols-[auto_1fr_auto] gap-6 items-center">
          <PostureRing score={posture.score} />

          <div className="min-w-0">
            <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-3">
              Posture breakdown
            </p>
            <PostureBreakdown
              items={[
                {
                  label: "Enforcement coverage",
                  value: posture.enforcementCoverage,
                  hint: `${posture.enforcing} of ${posture.approved} approved`,
                  tip: "Share of approved use cases running in blocking mode (vs flag mode). Drafts are excluded — only signed-off use cases count.",
                },
                {
                  label: "Runtime denial score",
                  value: posture.denialScore,
                  hint: `${posture.denials} denied in last 24h`,
                  tip: "100 with zero denials, dropping 2 points per denial in the last 24 hours, floored at 0. High denial volume usually signals key/use-case mis-binding rather than attack.",
                },
                {
                  label: "Notification delivery",
                  value: posture.webhookHealth,
                  hint:
                    posture.enabled === 0
                      ? "no subscriptions"
                      : `${posture.healthy} of ${posture.enabled} healthy`,
                  tip: "Share of enabled notification subscriptions whose last delivery returned a 2xx. 100 when no subscriptions are configured.",
                },
              ]}
            />
          </div>

          <div className="grid grid-cols-3 md:grid-cols-1 gap-2 md:w-44">
            <HeroStat
              icon={icons.useCase}
              label="Use cases"
              value={useCases.length}
              hint="Total across all statuses (drafts, under review, approved, retired)."
            />
            <HeroStat
              icon={icons.shield}
              label="Blocking"
              value={posture.enforcing}
              tone={posture.enforcing > 0 ? "emerald" : "muted"}
              hint="Approved use cases with blocking mode on — out-of-scope calls get a 403 / JSON-RPC error."
            />
            <HeroStat
              icon={icons.shieldAlert}
              label="Flagged"
              value={flaggedUCs}
              tone={flaggedUCs > 0 ? "rose" : "muted"}
              hint="Use cases the reconciler raised a compliance flag on — typically a dependency leaked a data class the use case doesn't declare."
            />
          </div>
        </div>
      </div>

      {/* ────────────── Charts row ────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Panel
          className="lg:col-span-2"
          eyebrow="Runtime"
          title="Scope events"
          tip="Each call from a key bound to a use case is checked. If the target isn't declared, the call is denied (blocking) or flagged (logged but allowed)."
          right={<RangeToggle value={range} onChange={setRange} />}
        >
          <div className="flex items-center gap-4 mb-2">
            <LegendDot tone="rose" label="Denied" />
            <LegendDot tone="amber" label="Flagged" />
            <span className="ml-auto text-[11px] font-mono tabular-nums text-muted-foreground">
              {stats?.last_24h_denied ?? 0} denied · {stats?.last_24h_flagged ?? 0} flagged (24h)
            </span>
          </div>
          {eventsLoading ? (
            <div className="h-48 flex items-center justify-center">
              <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary" />
            </div>
          ) : (
            <ScopeTimeline events={events} range={range} />
          )}
        </Panel>

        <Panel
          eyebrow="Top"
          title="Use cases with most denials"
          tip="Use cases whose bound API keys triggered the most scope denials in the selected window. Often a sign of mis-binding rather than misuse."
        >
          <UseCaseBars items={topUCs} />
        </Panel>
      </div>

      {/* ────────────── Discovery row ────────────── */}
      <DiscoveredAgentsList />

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Panel
          eyebrow="Top"
          title="Most-denied targets"
          tip="Models and tools that were most often refused — i.e. requested by a key whose use case didn't declare them."
        >
          <TargetBars items={topTg} />
        </Panel>
        <div className="lg:col-span-2">
          <LiveFeed events={events.slice(0, 10)} />
        </div>
      </div>
    </PageShell>
  );
}

// ---------------------------------------------------------------------------
// Hero stat tile
// ---------------------------------------------------------------------------

function HeroStat({
  icon,
  label,
  value,
  tone = "default",
  hint,
}: {
  icon: string;
  label: string;
  value: number;
  tone?: "default" | "emerald" | "rose" | "muted";
  hint?: string;
}) {
  const inner = (
    <div className="rounded-md border border-border/70 bg-background/40 px-3 py-2.5 flex items-center gap-2.5">
      <div
        className={cn(
          "h-8 w-8 rounded-md flex items-center justify-center shrink-0",
          tone === "emerald" && "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
          tone === "rose" && "bg-rose-500/10 text-rose-600 dark:text-rose-400",
          (tone === "default" || tone === "muted") &&
            "bg-muted text-muted-foreground",
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
            tone === "rose" && "text-rose-600 dark:text-rose-400",
          )}
        >
          {value}
        </div>
      </div>
    </div>
  );
  return hint ? <InfoTip label={hint}>{inner}</InfoTip> : inner;
}

function LegendDot({
  tone,
  label,
}: {
  tone: "rose" | "amber" | "emerald";
  label: string;
}) {
  const color =
    tone === "rose" ? "bg-rose-500" : tone === "amber" ? "bg-amber-500" : "bg-emerald-500";
  return (
    <div className="flex items-center gap-1.5">
      <span className={cn("h-2 w-2 rounded-full", color)} />
      <span className="text-[11px] text-muted-foreground">{label}</span>
    </div>
  );
}

function RangeToggle({
  value,
  onChange,
}: {
  value: TimelineRange;
  onChange: (v: TimelineRange) => void;
}) {
  return (
    <div className="inline-flex items-center rounded-md ring-1 ring-border overflow-hidden">
      {(["24h", "7d"] as TimelineRange[]).map((r) => (
        <button
          key={r}
          onClick={() => onChange(r)}
          className={cn(
            "text-[11px] font-mono uppercase tracking-wider h-6 px-2 transition",
            value === r
              ? "bg-foreground text-background"
              : "bg-transparent text-muted-foreground hover:bg-foreground/5",
          )}
        >
          {r}
        </button>
      ))}
    </div>
  );
}

function Panel({
  eyebrow,
  title,
  tip,
  right,
  children,
  className,
}: {
  eyebrow: string;
  title: string;
  tip?: string;
  right?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  const titleEl = (
    <div
      className={cn(
        "text-sm font-semibold tracking-tight mt-0.5",
        tip && "decoration-dotted decoration-muted-foreground/50 underline underline-offset-4 cursor-help",
      )}
    >
      {title}
    </div>
  );
  return (
    <div className={cn("rounded-lg border border-border bg-card p-4", className)}>
      <div className="flex items-start justify-between gap-2 mb-3">
        <div>
          <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            {eyebrow}
          </div>
          {tip ? <InfoTip label={tip}>{titleEl}</InfoTip> : titleEl}
        </div>
        {right}
      </div>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Live feed — compact trailing view of the most recent events.
// ---------------------------------------------------------------------------

function LiveFeed({ events }: { events: ScopeEvent[] }) {
  return (
    <Panel
      eyebrow="Live"
      title="Recent scope events"
      right={
        <span className="text-[10px] font-mono uppercase tracking-widest text-emerald-600 dark:text-emerald-400 flex items-center gap-1">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
          </span>
          live
        </span>
      }
    >
      {events.length === 0 ? (
        <div className="h-24 flex flex-col items-center justify-center gap-1">
          <Icon icon={icons.check} className="h-6 w-6 text-emerald-500" />
          <p className="text-xs text-muted-foreground">No scope events yet</p>
        </div>
      ) : (
        <ul className="space-y-1.5">
          {events.map((e) => {
            const providerInfo =
              e.kind === "llm" ? detectProvider(e.target_name, "") : null;
            return (
              <li
                key={e.id}
                className="flex items-center gap-2 text-[12px] py-1 border-b border-border/30 last:border-0"
              >
                <span
                  className={cn(
                    "h-1.5 w-1.5 rounded-full shrink-0",
                    e.outcome === "denied" ? "bg-rose-500" : "bg-amber-500",
                  )}
                />
                <Icon
                  icon={
                    providerInfo?.icon ??
                    (e.kind === "mcp_tool" ? icons.mcp : icons.llm)
                  }
                  className={cn(
                    "h-3.5 w-3.5 shrink-0",
                    providerInfo
                      ? null
                      : e.kind === "mcp_tool"
                        ? "text-slate-500"
                        : "text-violet-500",
                  )}
                />
                <code className="font-mono truncate max-w-[24ch]">
                  {e.target_name}
                </code>
                <span className="text-muted-foreground truncate flex-1">
                  {e.reason}
                </span>
                <span className="font-mono tabular-nums text-[10px] text-muted-foreground shrink-0">
                  {e.created_at &&
                    formatDistanceToNow(new Date(e.created_at), { addSuffix: true })}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </Panel>
  );
}

// ---------------------------------------------------------------------------
// CSV export — scope events (what auditors actually want)
// ---------------------------------------------------------------------------

function ExportCSV({ events }: { events: ScopeEvent[] }) {
  const onClick = () => {
    const header = [
      "created_at",
      "outcome",
      "kind",
      "target_name",
      "use_case_id",
      "key_name",
      "remote_addr",
      "reason",
    ];
    const lines = [header.join(",")];
    for (const e of events) {
      lines.push(
        [
          e.created_at ?? "",
          e.outcome,
          e.kind,
          csvEscape(e.target_name),
          e.use_case_id,
          csvEscape(e.key_name ?? ""),
          csvEscape(e.remote_addr ?? ""),
          csvEscape(e.reason ?? ""),
        ].join(","),
      );
    }
    const blob = new Blob([lines.join("\n")], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `compliance-events-${new Date().toISOString().slice(0, 10)}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };
  return (
    <Button
      variant="outline"
      size="sm"
      onClick={onClick}
      disabled={events.length === 0}
      className="gap-1.5"
    >
      <Icon icon={icons.download} className="h-3.5 w-3.5" />
      Export events
    </Button>
  );
}

function csvEscape(v: string): string {
  if (/[,"\n]/.test(v)) {
    return `"${v.replace(/"/g, '""')}"`;
  }
  return v;
}
