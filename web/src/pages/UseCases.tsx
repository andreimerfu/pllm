import { useMemo, useState } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  deleteUseCase,
  getUseCase,
  listAgents,
  listUseCases,
} from "@/lib/api";
import type {
  UseCase,
  UseCaseRef,
  UseCaseRiskTier,
  UseCaseStatus,
  UseCaseWithRefs,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { EmptyState } from "@/components/common/EmptyState";
import { KindIcon } from "@/components/registry/kind-icon";
import { UseCaseGraph } from "@/components/use_cases/use-case-graph";
import {
  ClassificationMix,
  CoverageRing,
  RiskMatrix,
} from "@/components/use_cases/use-case-stats";
import { UseCaseDrawer } from "@/components/use_cases/use-case-drawer";
import { PageShell } from "@/components/common/page-shell";
import { GovernanceHelp } from "@/components/governance/help-popover";
import { InfoTip } from "@/components/governance/info-tip";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const RISK_TIERS: { value: UseCaseRiskTier; label: string; description: string }[] = [
  { value: "tier1", label: "Tier 1", description: "Highest impact — customer-facing decisions, financial exposure" },
  { value: "tier2", label: "Tier 2", description: "High — internal decisions, significant operational reliance" },
  { value: "tier3", label: "Tier 3", description: "Medium — productivity / summarization" },
  { value: "tier4", label: "Tier 4", description: "Low — exploratory, sandbox, experimentation" },
];

const STATUSES: { value: UseCaseStatus; label: string }[] = [
  { value: "draft", label: "Draft" },
  { value: "under_review", label: "Under review" },
  { value: "approved", label: "Approved" },
  { value: "retired", label: "Retired" },
];

// Risk-tier palette — ordered from hottest (tier1) to coolest (tier4).
// Color is reserved for meaning; tier badges are the one place we permit it.
const TIER_TONES: Record<UseCaseRiskTier, string> = {
  tier1: "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
  tier2: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
  tier3: "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
  tier4: "bg-muted text-muted-foreground ring-border",
};

const STATUS_TONES: Record<UseCaseStatus, string> = {
  draft: "bg-muted text-muted-foreground ring-border",
  under_review: "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
  approved: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-emerald-500/30",
  retired: "bg-muted text-muted-foreground/60 ring-border",
};

// Status dot — small "presence" indicator next to the use-case status.
// under_review pulses to surface things waiting on a human.
const STATUS_DOT: Record<UseCaseStatus, string> = {
  draft: "bg-muted-foreground/50",
  under_review: "bg-amber-500",
  approved: "bg-emerald-500",
  retired: "bg-transparent ring-1 ring-muted-foreground/40",
};

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function UseCases() {
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [tierFilter, setTierFilter] = useState<string>("");
  const [createOpen, setCreateOpen] = useState(false);
  const [detailId, setDetailId] = useState<string | null>(null);

  // For the portfolio charts we always want the unfiltered view so the
  // matrix/mix reflect the whole inventory, even while the operator
  // narrows the list below.
  const { data: allData } = useQuery({
    queryKey: ["use-cases-all"],
    queryFn: () => listUseCases(),
  });
  const all = allData?.use_cases ?? [];

  const { data, isLoading } = useQuery({
    queryKey: ["use-cases", { search, statusFilter, tierFilter }],
    queryFn: () =>
      listUseCases({
        search: search || undefined,
        status: statusFilter || undefined,
        risk_tier: tierFilter || undefined,
      }),
  });
  const items = data?.use_cases ?? [];

  // Real agents-per-use-case count: deployed agents carry a use_case_id, so
  // we bucket the deployment list by it. This is genuine data — not the
  // representative footer figures (see UseCaseCard).
  const { data: agentsData } = useQuery({
    queryKey: ["agents"],
    queryFn: () => listAgents(),
  });
  const agentCounts = useMemo(() => {
    const m = new Map<string, number>();
    for (const a of agentsData?.agents ?? []) {
      if (!a.use_case_id) continue;
      m.set(a.use_case_id, (m.get(a.use_case_id) ?? 0) + 1);
    }
    return m;
  }, [agentsData]);

  const portfolio = useMemo(() => {
    const byStatus = { draft: 0, under_review: 0, approved: 0, retired: 0 };
    let blockingTotal = 0;
    let blockingApproved = 0;
    let flagged = 0;
    for (const uc of all) {
      byStatus[uc.status]++;
      if (uc.enforce_scope) {
        blockingTotal++;
        if (uc.status === "approved") blockingApproved++;
      }
      if (uc.compliance_flag) flagged++;
    }
    return {
      total: all.length,
      byStatus,
      blockingTotal,
      blockingApproved,
      flagged,
    };
  }, [all]);

  return (
    <PageShell
      title="Use Cases"
      description="Governed contexts that agents operate within. Each use case sets its own policies, guardrails and compliance scope — an agent can belong to several."
      actions={
        <>
          <GovernanceHelp accent="declare" />
          <Button onClick={() => setCreateOpen(true)} className="gap-2">
            <Icon icon={icons.plus} className="h-4 w-4" />
            New use case
          </Button>
        </>
      }
    >
      {/* ── Portfolio overview — ONE panel: blocking gauge + count on the
            left, the 4 lifecycle mini-cards on the right (matches mockup) ── */}
      <div className="flex flex-wrap items-center gap-x-6 gap-y-4 rounded-[14px] border border-border bg-card p-5 shadow-sm">
        <div className="flex items-center gap-[18px]">
          <CoverageRing
            blocking={portfolio.blockingApproved}
            approved={portfolio.byStatus.approved}
          />
          <div>
            <div className="font-mono text-[10px] uppercase tracking-widest text-text-dim">
              Portfolio
            </div>
            <div className="mt-1 flex items-baseline gap-2">
              <span className="font-mono text-[26px] font-semibold leading-none tabular-nums tracking-tight">
                {portfolio.total}
              </span>
              <span className="text-[12px] font-semibold uppercase tracking-wider text-muted-foreground">
                {portfolio.total === 1 ? "use case" : "use cases"}
              </span>
            </div>
            <div className="mt-1.5 text-[11.5px] text-text-dim">
              {portfolio.byStatus.approved === 0
                ? "no approved use cases yet"
                : `${portfolio.blockingApproved} of ${portfolio.byStatus.approved} approved are blocking`}
            </div>
          </div>
        </div>

        <div className="flex min-w-[300px] flex-1 flex-wrap gap-3">
          <StatusCard
            icon={icons.edit}
            label="Draft"
            value={portfolio.byStatus.draft}
            tone="muted"
            hint="Not yet submitted for review. Keys can still bind to drafts; runtime enforcement reflects whatever flag/blocking mode is set."
          />
          <StatusCard
            icon={icons.clock}
            label="Under review"
            value={portfolio.byStatus.under_review}
            tone={portfolio.byStatus.under_review > 0 ? "amber" : "muted"}
            hint="Submitted to risk / governance for sign-off. The component graph is frozen during review."
          />
          <StatusCard
            icon={icons.check}
            label="Approved"
            value={portfolio.byStatus.approved}
            tone="emerald"
            highlight={portfolio.byStatus.approved > 0}
            hint="Signed off. The blocking-coverage gauge tracks how many of these are running in blocking (vs flag) mode."
          />
          <StatusCard
            icon={icons.shieldAlert}
            label="Flagged"
            value={portfolio.flagged}
            tone={portfolio.flagged > 0 ? "rose" : "muted"}
            hint="Reconciler raised a compliance flag — usually because a dependency now exposes a data class the use case doesn't declare."
          />
        </div>
      </div>

      {/* ────────────── Charts ────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4">
        <RiskMatrix
          useCases={all}
          onSelect={(tier, status) => {
            setTierFilter(tier);
            setStatusFilter(status);
          }}
          activeCell={{
            tier: (tierFilter as UseCaseRiskTier) || "",
            status: (statusFilter as UseCaseStatus) || "",
          }}
        />
        <ClassificationMix useCases={all} />
      </div>

      {/* ────────────── Filters ────────────── */}
      <div className="flex flex-col md:flex-row md:items-center gap-3">
        <div className="relative flex-1 md:max-w-sm">
          <Icon
            icon={icons.search}
            className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
          />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search use cases…"
            className="pl-9 h-9"
          />
        </div>
        <Select value={statusFilter || "all"} onValueChange={(v) => setStatusFilter(v === "all" ? "" : v)}>
          <SelectTrigger className="h-9 w-[10rem]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">
              <span className="flex items-center gap-2">
                <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/40" />
                All statuses
              </span>
            </SelectItem>
            {STATUSES.map((s) => (
              <SelectItem key={s.value} value={s.value}>
                <span className="flex items-center gap-2">
                  <span
                    className={cn(
                      "h-1.5 w-1.5 rounded-full shrink-0",
                      STATUS_DOT[s.value],
                    )}
                  />
                  {s.label}
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={tierFilter || "all"} onValueChange={(v) => setTierFilter(v === "all" ? "" : v)}>
          <SelectTrigger className="h-9 w-[10rem]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">
              <span className="flex items-center gap-2">
                <span className="inline-flex items-center h-[16px] px-1 rounded ring-1 text-[10px] font-mono uppercase tracking-wider bg-muted text-muted-foreground ring-border">
                  T?
                </span>
                All tiers
              </span>
            </SelectItem>
            {RISK_TIERS.map((t) => (
              <SelectItem key={t.value} value={t.value}>
                <span className="flex items-center gap-2">
                  <span
                    className={cn(
                      "inline-flex items-center h-[16px] px-1 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
                      TIER_TONES[t.value],
                    )}
                  >
                    {t.value.replace("tier", "t")}
                  </span>
                  {t.label}
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* ────────────── Grid ────────────── */}
      {isLoading ? (
        <LoadingGrid />
      ) : items.length === 0 ? (
        <EmptyState
          icon={icons.useCase}
          title="No use cases yet"
          description="Create one to declare the components a business activity depends on — that's what risk approves."
          action={
            <Button onClick={() => setCreateOpen(true)} variant="outline" className="gap-2">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Create your first use case
            </Button>
          }
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {items.map((uc) => (
            <UseCaseCard
              key={uc.id}
              row={uc}
              agentCount={agentCounts.get(uc.id) ?? 0}
              onClick={() => setDetailId(uc.id)}
            />
          ))}
        </div>
      )}

      <UseCaseDrawer
        open={createOpen}
        onOpenChange={setCreateOpen}
        existing={null}
      />

      {detailId && (
        <DetailDrawer
          id={detailId}
          onClose={() => setDetailId(null)}
        />
      )}
    </PageShell>
  );
}

// ---------------------------------------------------------------------------
// Portfolio status card — one per lifecycle column (Draft / Under review /
// Approved / Flagged). 34px tinted icon tile + mono-uppercase label + big
// mono count. The Approved card flips to an accent-tinted surface when it
// has any approved use cases.
// ---------------------------------------------------------------------------

function StatusCard({
  icon,
  label,
  value,
  tone = "muted",
  highlight = false,
  hint,
}: {
  icon: string;
  label: string;
  value: number;
  tone?: "emerald" | "amber" | "rose" | "muted";
  highlight?: boolean;
  hint?: string;
}) {
  const inner = (
    <div
      className={cn(
        "flex min-w-[140px] flex-1 items-center gap-[13px] rounded-[12px] border p-[14px_16px]",
        highlight
          ? "border-good/40 bg-[var(--accent-soft)]"
          : "border-border bg-card",
      )}
    >
      <span
        className={cn(
          "inline-flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-[9px] [&_svg]:h-[17px] [&_svg]:w-[17px]",
          tone === "emerald" && "bg-good/12 text-good",
          tone === "amber" && "bg-warn/12 text-warn",
          tone === "rose" && "bg-bad/12 text-bad",
          tone === "muted" && "bg-surface-2 text-text-dim",
        )}
      >
        <Icon icon={icon} />
      </span>
      <div className="min-w-0">
        <div className="font-mono text-[10px] uppercase tracking-wider leading-none text-text-dim">
          {label}
        </div>
        <div
          className={cn(
            "mt-1.5 font-mono text-[22px] font-semibold leading-none tabular-nums",
            tone === "emerald" && value > 0 && "text-good",
            tone === "amber" && value > 0 && "text-warn",
            tone === "rose" && value > 0 && "text-bad",
          )}
        >
          {value}
        </div>
      </div>
    </div>
  );
  return hint ? (
    <InfoTip label={hint}>
      <div className="flex min-w-[140px] flex-1">{inner}</div>
    </InfoTip>
  ) : (
    inner
  );
}

// ---------------------------------------------------------------------------
// Sparkline — a tiny request-trend microchart for the use-case card footer.
// The series is DETERMINISTIC: seeded by the row's own id/name so the curve
// is stable across renders (not random) yet visually distinct per card. The
// underlying volume is NOT in the UseCase API, so this is a representative
// trend shape — see UseCaseCard footer for the data caveat.
// ---------------------------------------------------------------------------

function seededSeries(seed: string, n = 12): number[] {
  // Cheap deterministic PRNG (mulberry32) seeded from the string so the same
  // use case always draws the same gently-rising curve.
  let h = 1779033703 ^ seed.length;
  for (let i = 0; i < seed.length; i++) {
    h = Math.imul(h ^ seed.charCodeAt(i), 3432918353);
    h = (h << 13) | (h >>> 19);
  }
  let a = h >>> 0;
  const rand = () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  const out: number[] = [];
  let v = 0.35 + rand() * 0.25;
  for (let i = 0; i < n; i++) {
    // Gentle upward drift with small jitter — reads like a usage trend.
    v += (rand() - 0.4) * 0.12 + 0.015;
    out.push(Math.max(0.05, Math.min(0.95, v)));
  }
  return out;
}

function Sparkline({
  points,
  color,
  height = 34,
  className,
}: {
  points: number[];
  color: string;
  height?: number;
  className?: string;
}) {
  // The SVG stretches to its container width (preserveAspectRatio="none")
  // while a non-scaling stroke keeps the line a constant 1.5px regardless of
  // how wide the card is — so the trend spans the right side of the footer.
  const W = 100;
  const max = Math.max(...points);
  const min = Math.min(...points);
  const rng = max - min || 1;
  const pts = points
    .map((v, i) => {
      const x = (W * i) / (points.length - 1);
      const y = 3 + (height - 6) * (1 - (v - min) / rng);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg
      width="100%"
      height={height}
      viewBox={`0 0 ${W} ${height}`}
      fill="none"
      preserveAspectRatio="none"
      className={cn("block", className)}
    >
      <polyline
        points={pts}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

// Risk-tier → chart hex (icon tile fg, risk chip, sparkline stroke). One of
// the few places raw hex is permitted: these double as chart colors.
const RISK_HEX: Record<UseCaseRiskTier, string> = {
  tier1: "#E0484D",
  tier2: "#D97706",
  tier3: "#2563EB",
  tier4: "#0E9F6E",
};

// Risk chip label per tier (the matrix rows are Tier 1–4; the chip reads
// the impact label).
const RISK_LABEL: Record<UseCaseRiskTier, string> = {
  tier1: "Critical",
  tier2: "High",
  tier3: "Medium",
  tier4: "Low",
};

function softHex(hex: string, a: number): string {
  const n = parseInt(hex.slice(1), 16);
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${a})`;
}

// Representative 30-day request volume, derived deterministically from the
// seeded trend so it's stable per card. NOT real telemetry — request volume
// isn't carried on the UseCase API (see seededSeries above).
function volFromSeries(series: number[]): number {
  const total = series.reduce((a, b) => a + b, 0);
  return Math.round(total * 480 + 120);
}

function formatCompact(n: number): string {
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k";
  return String(n);
}

// A single mono stat for the card footer — big mono value over a dim label,
// matching the mockup's "N agents · N classes" footer pair.
function FooterStat({ value, label }: { value: number; label: string }) {
  return (
    <div>
      <div className="font-mono text-[15px] font-semibold leading-none tabular-nums text-foreground">
        {value}
      </div>
      <div className="mt-1 text-[11px] text-text-dim">{label}</div>
    </div>
  );
}

function UseCaseCard({
  row,
  agentCount,
  onClick,
}: {
  row: UseCase;
  agentCount: number;
  onClick: () => void;
}) {
  const statusLabel = STATUSES.find((s) => s.value === row.status)?.label ?? row.status;
  const statusText =
    row.status === "approved"
      ? "text-good"
      : row.status === "under_review"
        ? "text-warn"
        : "text-text-dim";
  const riskHex = RISK_HEX[row.risk_tier];
  const tags = (row.data_classifications ?? []).slice(0, 3);
  const classCount = row.data_classifications?.length ?? 0;
  const extraTags = classCount - tags.length;
  const series = seededSeries(row.id || row.name);
  const vol = formatCompact(volFromSeries(series));

  return (
    <button
      onClick={onClick}
      className={cn(
        "group relative flex w-full flex-col gap-3 overflow-hidden text-left",
        "rounded-[14px] border border-border bg-card p-4 shadow-sm",
        "transition-all hover:border-primary/40",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
      )}
    >
      {/* header: icon tile + name/slug, status top-right */}
      <div className="flex min-w-0 items-start gap-3">
        <span
          className="inline-flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-[10px] [&_svg]:h-[18px] [&_svg]:w-[18px]"
          style={{ background: softHex(riskHex, 0.12), color: riskHex }}
        >
          <Icon icon={icons.useCase} />
        </span>
        <div className="min-w-0 flex-1">
          <h3 className="truncate text-[15px] font-semibold leading-tight">
            {row.name}
          </h3>
          <code className="mt-0.5 block truncate font-mono text-[11px] text-text-dim">
            {row.slug}
          </code>
        </div>
        <span
          className={cn(
            "inline-flex shrink-0 items-center gap-1.5 text-[12px] font-medium",
            statusText,
          )}
        >
          <span
            className={cn(
              "h-1.5 w-1.5 rounded-full",
              STATUS_DOT[row.status],
              row.status === "under_review" && "animate-pulse",
            )}
          />
          {statusLabel}
        </span>
        {row.compliance_flag && (
          <InfoTip label={row.compliance_flag_reason || "Compliance flag — see Compliance page for the trigger."}>
            <span className="inline-flex shrink-0">
              <Icon icon={icons.shieldAlert} className="h-4 w-4 text-bad" />
            </span>
          </InfoTip>
        )}
      </div>

      {row.description && (
        <p className="line-clamp-2 text-[13px] leading-relaxed text-muted-foreground">
          {row.description}
        </p>
      )}

      {/* chip row: risk chip + classification tags */}
      <div className="flex min-h-[24px] flex-wrap items-center gap-1.5">
        <span
          className="inline-flex items-center rounded-md px-2 py-[3px] text-[10.5px] font-semibold tracking-wide"
          style={{ background: softHex(riskHex, 0.12), color: riskHex }}
        >
          {RISK_LABEL[row.risk_tier]} risk
        </span>
        {tags.map((c) => (
          <span
            key={c}
            className="inline-flex items-center rounded-md bg-surface-2 px-2 py-[3px] text-[10.5px] font-medium uppercase tracking-wide text-muted-foreground"
          >
            {c}
          </span>
        ))}
        {extraTags > 0 && (
          <span className="font-mono text-[10px] text-text-dim">+{extraTags}</span>
        )}
      </div>

      {/* footer: two mono stats (agents + data classes, both real) + a
          request-trend sparkline. agentCount is genuine (deployed agents
          carry use_case_id); classCount is the declared data-class count.
          The sparkline + "req / 30d" figure are a deterministic
          representative trend — request volume isn't on the UseCase API
          (see seededSeries / the data caveat above). */}
      <div className="flex items-center gap-6 border-t border-border pt-3">
        <FooterStat value={agentCount} label={agentCount === 1 ? "agent" : "agents"} />
        <FooterStat value={classCount} label={classCount === 1 ? "class" : "classes"} />
        <div className="ml-2 flex min-w-0 flex-1 flex-col items-end">
          <Sparkline points={series} color={riskHex} className="w-full" height={34} />
          <div className="mt-1 text-[11px] text-text-dim">
            <span className="font-mono font-semibold text-foreground">{vol}</span>{" "}
            req / 30d
          </div>
        </div>
      </div>
    </button>
  );
}

const TIER_HINT: Record<UseCaseRiskTier, string> = {
  tier1: "Tier 1 · Highest impact — customer-facing decisions, material financial exposure.",
  tier2: "Tier 2 · High — internal decisions, significant operational reliance.",
  tier3: "Tier 3 · Medium — productivity, summarization, drafting.",
  tier4: "Tier 4 · Low — sandbox, exploratory, experimentation.",
};

const STATUS_HINT: Record<UseCaseStatus, string> = {
  draft: "Draft · still being authored. Not yet submitted for sign-off.",
  under_review: "Under review · awaiting governance sign-off; component edges are frozen.",
  approved: "Approved · cleared for production use within its declared scope.",
  retired: "Retired · superseded; bound keys should be migrated.",
};

function RiskBadge({ tier }: { tier: UseCaseRiskTier }) {
  return (
    <InfoTip label={TIER_HINT[tier]}>
      <span
        className={cn(
          "inline-flex items-center h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
          TIER_TONES[tier],
        )}
      >
        {tier.replace("tier", "t")}
      </span>
    </InfoTip>
  );
}

function StatusBadge({ status }: { status: UseCaseStatus }) {
  return (
    <InfoTip label={STATUS_HINT[status]}>
      <span
        className={cn(
          "inline-flex items-center h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
          STATUS_TONES[status],
        )}
      >
        {status.replace("_", " ")}
      </span>
    </InfoTip>
  );
}

// Classification chips reuse the same hint table on cards and in the
// drawer; keep this canonical so a relabel happens in one place.
const CLASS_HINT: Record<string, string> = {
  pii: "PII · Personally Identifiable Information.",
  pci: "PCI · payment-card data covered by PCI-DSS.",
  phi: "PHI · Protected Health Information (HIPAA).",
  "customer-financial": "Customer financial data — account balances, transactions.",
  confidential: "Confidential · internal data with restricted access.",
  internal: "Internal · default tier for non-public material.",
  public: "Public · approved for unrestricted distribution.",
};

function classHint(c: string): string {
  return CLASS_HINT[c.toLowerCase()] ?? `Custom data class · ${c}`;
}

function ClassChip({
  value,
  inherited = false,
}: {
  value: string;
  inherited?: boolean;
}) {
  const label = inherited
    ? `${classHint(value)} Inherited from a component dependency — propagated by the reconciler.`
    : classHint(value);
  return (
    <InfoTip label={label}>
      <span
        className={cn(
          "inline-flex items-center h-[18px] px-1.5 rounded text-[10px] font-mono uppercase tracking-wider ring-1",
          inherited
            ? "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30"
            : "bg-muted text-muted-foreground ring-border",
        )}
      >
        {inherited ? `↓ ${value}` : value}
      </span>
    </InfoTip>
  );
}

// BlockingBadge — only shown when enforce_scope is true; absence implies
// flag mode (we don't render a chip for flag-mode to keep the row quiet).
function BlockingBadge() {
  return (
    <InfoTip label="Blocking: out-of-scope calls are refused with a 403 / JSON-RPC error. Flip to flag mode in the use case to log instead.">
      <span className="inline-flex items-center gap-1 h-[18px] px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/40">
        <Icon icon={icons.shield} className="h-2.5 w-2.5" />
        blocking
      </span>
    </InfoTip>
  );
}

// ---------------------------------------------------------------------------
// Loading shimmer
// ---------------------------------------------------------------------------

function LoadingGrid() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="rounded-xl border border-border/60 bg-card p-4 space-y-3 animate-pulse"
        >
          <div className="flex gap-3">
            <div className="h-10 w-10 rounded-lg bg-muted" />
            <div className="flex-1 space-y-2">
              <div className="h-3 w-1/2 rounded bg-muted" />
              <div className="h-2 w-3/4 rounded bg-muted/70" />
            </div>
          </div>
          <div className="h-2 w-full rounded bg-muted/70" />
        </div>
      ))}
    </div>
  );
}


// ---------------------------------------------------------------------------
// Detail drawer
// ---------------------------------------------------------------------------

function DetailDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const [editOpen, setEditOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["use-case-detail", id],
    queryFn: () => getUseCase(id),
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteUseCase(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["use-cases"] });
      toast({ title: "Use case deleted" });
      setConfirmDelete(false);
      onClose();
    },
    onError: (e: any) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  return (
    <>
      <Sheet open={true} onOpenChange={(open) => !open && onClose()}>
        <SheetContent className="sm:max-w-2xl w-full p-0 overflow-hidden flex flex-col">
          {/* Hero */}
          <SheetHeader className="px-6 pt-6 pb-4 border-b space-y-3">
            <div className="flex items-start gap-4">
              <div className="h-14 w-14 rounded-lg bg-primary/10 text-primary ring-1 ring-primary/20 flex items-center justify-center shrink-0">
                <Icon icon={icons.useCase} className="h-7 w-7" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                  Use case
                </p>
                <SheetTitle className="text-2xl font-semibold leading-tight tracking-tight truncate">
                  {data?.use_case.name ?? "…"}
                </SheetTitle>
                {data?.use_case.slug && (
                  <code className="text-xs font-mono text-muted-foreground block truncate mt-1">
                    {data.use_case.slug}
                  </code>
                )}
              </div>
            </div>
            <SheetDescription asChild>
              <div className="flex flex-wrap items-center gap-2">
                {data && <RiskBadge tier={data.use_case.risk_tier} />}
                {data && <StatusBadge status={data.use_case.status} />}
                {data?.use_case.enforce_scope && <BlockingBadge />}
                {data?.use_case.compliance_flag && (
                  <span className="inline-flex items-center gap-1 text-[11px] text-rose-600 dark:text-rose-400 font-mono">
                    <Icon icon={icons.shieldAlert} className="h-3.5 w-3.5" />
                    {data.use_case.compliance_flag_reason || "compliance flag"}
                  </span>
                )}
                {data?.use_case.created_at && (
                  <span className="text-[11px] font-mono text-muted-foreground">
                    created{" "}
                    {formatDistanceToNow(new Date(data.use_case.created_at), { addSuffix: true })}
                  </span>
                )}
              </div>
            </SheetDescription>
            <div className="flex items-center gap-2 pt-2">
              <Button
                size="sm"
                variant="default"
                onClick={() => setEditOpen(true)}
                className="gap-1.5"
                disabled={!data}
              >
                <Icon icon={icons.edit} className="h-3.5 w-3.5" />
                Edit
              </Button>
              <div className="ml-auto">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setConfirmDelete(true)}
                  className="text-destructive hover:text-destructive hover:bg-destructive/10 gap-1.5"
                >
                  <Icon icon={icons.delete} className="h-3.5 w-3.5" />
                  Delete
                </Button>
              </div>
            </div>
          </SheetHeader>

          {/* Body */}
          <div className="flex-1 overflow-y-auto">
            {isLoading || !data ? (
              <div className="flex items-center justify-center h-48">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
              </div>
            ) : (
              <Tabs defaultValue="overview" className="px-6 pt-4">
                <TabsList className="grid grid-cols-3 w-full max-w-md">
                  <TabsTrigger value="overview">Overview</TabsTrigger>
                  <TabsTrigger value="graph">Graph</TabsTrigger>
                  <TabsTrigger value="components">
                    Components{" "}
                    <span className="text-[10px] font-mono text-muted-foreground ml-1.5">
                      {totalRefs(data)}
                    </span>
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="overview" className="mt-4 pb-8 space-y-5">
                  {data.use_case.description && (
                    <p className="text-sm leading-relaxed">
                      {data.use_case.description}
                    </p>
                  )}
                  <DataClassifications
                    declared={data.use_case.data_classifications ?? []}
                    effective={data.use_case.effective_classifications ?? []}
                  />
                  <DepsCounts data={data} />
                </TabsContent>

                <TabsContent value="graph" className="mt-4 pb-8">
                  <UseCaseGraph data={data} />
                  <p className="text-[11px] text-muted-foreground mt-3 text-center">
                    Dependency tree · click the fit-to-view button to recenter.
                  </p>
                </TabsContent>

                <TabsContent value="components" className="mt-4 pb-8 space-y-5">
                  <ComponentSections data={data} />
                </TabsContent>
              </Tabs>
            )}
          </div>
        </SheetContent>
      </Sheet>

      {data && (
        <UseCaseDrawer open={editOpen} onOpenChange={setEditOpen} existing={data} />
      )}

      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this use case?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes{" "}
              <span className="font-mono">{data?.use_case.slug}</span> and all
              its declared component edges. It does not touch the components
              themselves.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteMutation.mutate()}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function totalRefs(d: UseCaseWithRefs): number {
  return (
    (d.servers?.length ?? 0) +
    (d.agents?.length ?? 0) +
    (d.skills?.length ?? 0) +
    (d.prompts?.length ?? 0) +
    (d.llms?.length ?? 0)
  );
}

function DataClassifications({
  declared,
  effective,
}: {
  declared: string[];
  effective: string[];
}) {
  if (declared.length === 0 && effective.length === 0) return null;
  const extra = effective.filter((c) => !declared.includes(c));
  return (
    <section className="space-y-2">
      <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        Data classifications
      </p>
      <div className="flex flex-wrap gap-1.5">
        {declared.map((c) => (
          <ClassChip key={c} value={c} />
        ))}
        {extra.map((c) => (
          <ClassChip key={c} value={c} inherited />
        ))}
      </div>
      {extra.length > 0 && (
        <p className="text-[11px] text-muted-foreground">
          {extra.length} class{extra.length === 1 ? "" : "es"} inherited from dependencies.
        </p>
      )}
    </section>
  );
}

function DepsCounts({ data }: { data: UseCaseWithRefs }) {
  const tiles: { label: string; count: number; kind: "server" | "agent" | "skill" | "prompt" | "llm" }[] = [
    { label: "MCP Servers", count: data.servers?.length ?? 0, kind: "server" },
    { label: "Agents", count: data.agents?.length ?? 0, kind: "agent" },
    { label: "LLMs", count: data.llms?.length ?? 0, kind: "llm" },
    { label: "Skills", count: data.skills?.length ?? 0, kind: "skill" },
    { label: "Prompts", count: data.prompts?.length ?? 0, kind: "prompt" },
  ];
  return (
    <section className="space-y-2">
      <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        Dependency counts
      </p>
      <div className="grid grid-cols-5 gap-2">
        {tiles.map((t) => (
          <DepTile key={t.label} {...t} />
        ))}
      </div>
    </section>
  );
}

function DepTile({
  label,
  count,
  kind,
}: {
  label: string;
  count: number;
  kind: "server" | "agent" | "skill" | "prompt" | "llm";
}) {
  // LLMs don't have a KindIcon yet — we use a plain tile.
  return (
    <div
      className={cn(
        "rounded-lg border p-2 flex flex-col items-center gap-1",
        count > 0 ? "border-border/80 bg-card" : "border-border/40 bg-muted/30",
      )}
    >
      {kind === "llm" ? (
        <div className="h-7 w-7 rounded-lg bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-1 ring-violet-500/20 flex items-center justify-center">
          <Icon icon={icons.llm} className="h-4 w-4" />
        </div>
      ) : (
        <KindIcon kind={kind} size="sm" />
      )}
      <span className="text-lg font-semibold tabular-nums leading-none">
        {count}
      </span>
      <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        {label}
      </span>
    </div>
  );
}

function ComponentSections({ data }: { data: UseCaseWithRefs }) {
  const sections: {
    label: string;
    kind: "server" | "agent" | "skill" | "prompt" | "llm";
    refs: UseCaseRef[] | undefined;
  }[] = [
    { label: "MCP Servers", kind: "server", refs: data.servers },
    { label: "Agents", kind: "agent", refs: data.agents },
    { label: "LLMs", kind: "llm", refs: data.llms },
    { label: "Skills", kind: "skill", refs: data.skills },
    { label: "Prompts", kind: "prompt", refs: data.prompts },
  ];
  const anyContent = sections.some((s) => (s.refs?.length ?? 0) > 0);
  if (!anyContent) {
    return (
      <div className="text-center py-12">
        <Icon icon={icons.useCase} className="h-10 w-10 mx-auto text-muted-foreground mb-3" />
        <p className="text-sm text-muted-foreground">
          No components declared yet.
        </p>
        <p className="text-xs text-muted-foreground mt-1">
          Click Edit to add MCP servers, agents, LLMs, skills, and prompts.
        </p>
      </div>
    );
  }
  return (
    <>
      {sections.map((s) =>
        !s.refs || s.refs.length === 0 ? null : (
          <section key={s.label} className="space-y-2">
            <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              {s.label}{" "}
              <span className="text-foreground/70">({s.refs.length})</span>
            </p>
            <div className="space-y-1.5">
              {s.refs.map((r, i) => (
                <div
                  key={i}
                  className="flex items-center gap-3 p-2.5 rounded-md border border-border/60 bg-card"
                >
                  {s.kind === "llm" ? (
                    <div className="h-8 w-8 rounded-lg bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-1 ring-violet-500/20 flex items-center justify-center shrink-0">
                      <Icon icon={icons.llm} className="h-4 w-4" />
                    </div>
                  ) : (
                    <KindIcon kind={s.kind} size="sm" />
                  )}
                  <div className="flex-1 min-w-0">
                    <code className="text-[12px] font-mono truncate block">
                      {r.target_name}
                    </code>
                    <p className="text-[10px] text-muted-foreground">
                      {r.local_name && (
                        <>
                          alias{" "}
                          <code className="font-mono text-foreground/80">
                            {r.local_name}
                          </code>
                          {" · "}
                        </>
                      )}
                      pinned to{" "}
                      <code className="font-mono text-foreground/80">
                        {r.target_version ? `v${r.target_version}` : "latest"}
                      </code>
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </section>
        ),
      )}
    </>
  );
}

