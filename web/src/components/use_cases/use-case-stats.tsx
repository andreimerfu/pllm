import { Fragment, useMemo } from "react";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { UseCase, UseCaseRiskTier, UseCaseStatus } from "@/types/api";

// ---------------------------------------------------------------------------
// Shared chart colors. These are the one place raw hex is permitted —
// chart stroke/fill only — to faithfully match the decoded mockup. Tier
// colors run hottest (T1) to coolest (T4); classification colors group by
// sensitivity tier.
// ---------------------------------------------------------------------------

const TIER_HEX: Record<UseCaseRiskTier, string> = {
  tier1: "#E0484D",
  tier2: "#D97706",
  tier3: "#2563EB",
  tier4: "#8B95A2",
};

const CLASS_HEX: Record<string, string> = {
  pii: "#E0484D",
  pci: "#E0484D",
  phi: "#E0484D",
  "customer-financial": "#D97706",
  financial: "#D97706",
  confidential: "#D97706",
  internal: "#2563EB",
  public: "#0E9F6E",
};

function classHex(c: string): string {
  return CLASS_HEX[c.toLowerCase()] ?? "#586273";
}

// rgba helper for the soft cell/bar tints (8% / 45% of the tier hue).
function softHex(hex: string, a: number): string {
  const n = parseInt(hex.slice(1), 16);
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  return `rgba(${r}, ${g}, ${b}, ${a})`;
}

// ---------------------------------------------------------------------------
// Tier × Lifecycle matrix — the risk officer's default view: how much work
// sits in each (tier, status) cell. Each cell is a tappable filter. Cells
// are 70px tall, rounded-[11px], with an 18px mono count; non-zero cells
// pick up a soft tier tint + a tier-colored border.
// ---------------------------------------------------------------------------

const TIERS: { key: UseCaseRiskTier; label: string }[] = [
  { key: "tier1", label: "Tier 1" },
  { key: "tier2", label: "Tier 2" },
  { key: "tier3", label: "Tier 3" },
  { key: "tier4", label: "Tier 4" },
];

const MATRIX_STATUSES: { key: UseCaseStatus; label: string }[] = [
  { key: "draft", label: "Draft" },
  { key: "under_review", label: "Under review" },
  { key: "approved", label: "Approved" },
  { key: "retired", label: "Retired" },
];

export function RiskMatrix({
  useCases,
  onSelect,
  activeCell,
}: {
  useCases: UseCase[];
  onSelect: (tier: UseCaseRiskTier | "", status: UseCaseStatus | "") => void;
  activeCell?: { tier: UseCaseRiskTier | ""; status: UseCaseStatus | "" };
}) {
  const { counts, rowTotals, colTotals } = useMemo(() => {
    const counts = new Map<string, number>();
    const rowTotals: Record<UseCaseRiskTier, number> = {
      tier1: 0, tier2: 0, tier3: 0, tier4: 0,
    };
    const colTotals: Record<UseCaseStatus, number> = {
      draft: 0, under_review: 0, approved: 0, retired: 0,
    };
    for (const uc of useCases) {
      const key = `${uc.risk_tier}|${uc.status}`;
      counts.set(key, (counts.get(key) ?? 0) + 1);
      rowTotals[uc.risk_tier]++;
      colTotals[uc.status]++;
    }
    return { counts, rowTotals, colTotals };
  }, [useCases]);

  const filterActive =
    !!activeCell && (activeCell.tier !== "" || activeCell.status !== "");

  return (
    <div className="rounded-[14px] border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-baseline justify-between">
        <div>
          <div className="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
            Risk × lifecycle
          </div>
          <div className="mt-0.5 text-sm font-semibold tracking-tight">
            {useCases.length} use{" "}
            {useCases.length === 1 ? "case" : "cases"} across the portfolio
          </div>
        </div>
        {filterActive && (
          <button
            onClick={() => onSelect("", "")}
            className="flex items-center gap-1 text-[11px] text-primary hover:underline"
          >
            <Icon icon={icons.close} className="h-3 w-3" />
            Clear filter
          </button>
        )}
      </div>

      {/* grid: tier-label column + 4 status columns + a Σ total column */}
      <div
        className="grid items-center gap-[9px]"
        style={{ gridTemplateColumns: "1.5fr repeat(4, 1fr) 52px" }}
      >
        {/* header row — inline "Label N", sentence case */}
        <div />
        {MATRIX_STATUSES.map((s) => (
          <button
            key={s.key}
            onClick={() => onSelect("", s.key)}
            className={cn(
              "flex items-baseline justify-center gap-1.5 pb-1 text-[12px] transition-colors hover:text-foreground",
              activeCell?.status === s.key && activeCell?.tier === ""
                ? "text-foreground"
                : "text-muted-foreground",
            )}
          >
            <span className="font-semibold">{s.label}</span>
            <span className="font-mono tabular-nums text-text-dim">
              {colTotals[s.key]}
            </span>
          </button>
        ))}
        <div className="flex items-end justify-center pb-1 text-[14px] text-text-dim">
          Σ
        </div>

        {/* tier rows */}
        {TIERS.map((t) => {
          const hex = TIER_HEX[t.key];
          return (
            <Fragment key={t.key}>
              <button
                onClick={() => onSelect(t.key, "")}
                className="flex items-center gap-2.5 self-center transition-opacity hover:opacity-80"
              >
                <span
                  className="h-[9px] w-[9px] shrink-0 rounded-full"
                  style={{ background: hex }}
                />
                <span
                  className="font-mono text-[13px] font-semibold"
                  style={{ color: hex }}
                >
                  {t.label}
                </span>
              </button>
              {MATRIX_STATUSES.map((s) => {
                const count = counts.get(`${t.key}|${s.key}`) ?? 0;
                const active =
                  activeCell?.tier === t.key && activeCell?.status === s.key;
                return (
                  <button
                    key={s.key}
                    onClick={() => onSelect(t.key, s.key)}
                    className={cn(
                      "flex h-[70px] items-center justify-center rounded-[11px] border font-mono text-[18px] font-semibold transition-all",
                      count > 0
                        ? "text-foreground"
                        : "border-border bg-card text-text-dim",
                      count > 0 && "hover:brightness-105",
                      active && "ring-2 ring-primary ring-offset-1 ring-offset-card",
                    )}
                    style={
                      count > 0
                        ? {
                            background: softHex(hex, 0.12),
                            borderColor: softHex(hex, 0.45),
                          }
                        : undefined
                    }
                  >
                    {count}
                  </button>
                );
              })}
              <div className="flex items-center justify-center font-mono text-[13px] tabular-nums text-text-dim">
                {rowTotals[t.key]}
              </div>
            </Fragment>
          );
        })}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Classification mix — what classifications are in the portfolio and how
// many use cases touch each. Pulls from both declared and effective
// classifications so propagated labels show up alongside hand-tagged ones.
// Each row: colored dot + uppercase label + "count · pct%" and a track with
// a colored bar (solid = declared share, faded = inherited share).
// ---------------------------------------------------------------------------

export function ClassificationMix({ useCases }: { useCases: UseCase[] }) {
  const rows = useMemo(() => {
    const counts = new Map<string, { declared: number; inherited: number }>();
    for (const uc of useCases) {
      const declared = new Set(uc.data_classifications ?? []);
      const effective = new Set(uc.effective_classifications ?? []);
      for (const c of declared) {
        const cur = counts.get(c) ?? { declared: 0, inherited: 0 };
        cur.declared++;
        counts.set(c, cur);
      }
      for (const c of effective) {
        if (declared.has(c)) continue;
        const cur = counts.get(c) ?? { declared: 0, inherited: 0 };
        cur.inherited++;
        counts.set(c, cur);
      }
    }
    return Array.from(counts.entries())
      .map(([label, v]) => ({ label, ...v, total: v.declared + v.inherited }))
      .sort((a, b) => b.total - a.total)
      .slice(0, 8);
  }, [useCases]);

  const total = useCases.length || 1;

  return (
    <div className="rounded-[14px] border border-border bg-card p-4 shadow-sm">
      <div className="mb-3">
        <div className="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
          Classification mix
        </div>
        <div className="mt-0.5 text-sm font-semibold tracking-tight">
          Data classes in scope
        </div>
      </div>
      {rows.length === 0 ? (
        <div className="flex h-32 items-center justify-center text-xs text-muted-foreground">
          No classifications declared
        </div>
      ) : (
        <div className="space-y-3">
          {rows.map((r) => {
            const hex = classHex(r.label);
            const pct = Math.round((r.total / total) * 100);
            const declaredPct = Math.round((r.declared / total) * 100);
            const inheritedPct = pct - declaredPct;
            return (
              <div key={r.label} className="space-y-1.5">
                <div className="flex items-center justify-between gap-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <span
                      className="h-2 w-2 shrink-0 rounded-full"
                      style={{ background: hex }}
                    />
                    <span className="truncate font-mono text-[12px] font-semibold uppercase tracking-wide text-foreground">
                      {r.label}
                    </span>
                  </div>
                  <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
                    {r.total}
                    <span className="ml-1 text-muted-foreground/60">· {pct}%</span>
                  </span>
                </div>
                <div className="flex h-1.5 w-full overflow-hidden rounded-full bg-surface-2">
                  <div
                    className="h-1.5 rounded-full"
                    style={{ width: `${declaredPct}%`, background: hex }}
                  />
                  {inheritedPct > 0 && (
                    <div
                      className="h-1.5 rounded-full"
                      style={{
                        width: `${inheritedPct}%`,
                        background: softHex(hex, 0.5),
                      }}
                    />
                  )}
                </div>
              </div>
            );
          })}
          <p className="pt-1 text-[10px] text-muted-foreground">
            Solid = declared · Faded = inherited from components
          </p>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Blocking gauge — "blocking coverage" = of approved use cases, how many are
// in blocking mode. `blocking` MUST already be filtered to approved-only;
// historically callers passed the unfiltered enforce count, which let drafts
// push the gauge past 100%. Rendered as an SVG donut (r=40) with the percent
// in the center and the "{blocking} of {approved} approved are blocking"
// sub-line beside it.
// ---------------------------------------------------------------------------

export function CoverageRing({
  blocking,
  approved,
}: {
  blocking: number;
  approved: number;
}) {
  const safeBlocking = Math.min(blocking, approved);
  const pct = approved === 0 ? 0 : Math.round((safeBlocking / approved) * 100);
  const tone = pct >= 80 ? "text-good" : pct >= 50 ? "text-warn" : "text-bad";

  const r = 40;
  const c = 2 * Math.PI * r;
  const dash = (pct / 100) * c;

  return (
    <div className="relative h-[92px] w-[92px] shrink-0">
      <svg viewBox="0 0 100 100" className="h-full w-full -rotate-90">
        <circle
          cx="50"
          cy="50"
          r={r}
          stroke="currentColor"
          strokeWidth="9"
          fill="none"
          className="text-surface-2"
        />
        <circle
          cx="50"
          cy="50"
          r={r}
          stroke="currentColor"
          strokeWidth="9"
          fill="none"
          strokeDasharray={`${dash} ${c}`}
          strokeLinecap="round"
          className={cn(tone, "transition-all duration-700 ease-out")}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className={cn("font-mono text-[19px] font-bold leading-none tabular-nums", tone)}>
          {pct}
          <span className="text-[13px]">%</span>
        </span>
        <span className="mt-[3px] font-mono text-[8px] uppercase tracking-[0.08em] text-text-dim">
          blocking
        </span>
      </div>
    </div>
  );
}
