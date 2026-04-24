import { useMemo } from "react";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { UseCase, UseCaseRiskTier, UseCaseStatus } from "@/types/api";

// ---------------------------------------------------------------------------
// Tier × Status matrix — the risk officer's default view: how much work
// sits in each (tier, status) cell. Tier1/under_review is the hot cell —
// the one they paged on last time — so we want it immediately visible.
// ---------------------------------------------------------------------------

const TIERS: { key: UseCaseRiskTier; label: string }[] = [
  { key: "tier1", label: "Tier 1" },
  { key: "tier2", label: "Tier 2" },
  { key: "tier3", label: "Tier 3" },
  { key: "tier4", label: "Tier 4" },
];

const STATUSES: { key: UseCaseStatus; label: string }[] = [
  { key: "draft", label: "Draft" },
  { key: "under_review", label: "Under review" },
  { key: "approved", label: "Approved" },
  { key: "retired", label: "Retired" },
];

const TIER_COLOR: Record<UseCaseRiskTier, string> = {
  tier1: "bg-rose-500",
  tier2: "bg-amber-500",
  tier3: "bg-sky-500",
  tier4: "bg-slate-400",
};

const TIER_TEXT: Record<UseCaseRiskTier, string> = {
  tier1: "text-rose-600 dark:text-rose-400",
  tier2: "text-amber-600 dark:text-amber-400",
  tier3: "text-sky-600 dark:text-sky-400",
  tier4: "text-slate-500 dark:text-slate-400",
};

export function RiskMatrix({
  useCases,
  onSelect,
  activeCell,
}: {
  useCases: UseCase[];
  onSelect: (tier: UseCaseRiskTier | "", status: UseCaseStatus | "") => void;
  activeCell?: { tier: UseCaseRiskTier | ""; status: UseCaseStatus | "" };
}) {
  const { counts, rowTotals, colTotals, max } = useMemo(() => {
    const counts = new Map<string, number>();
    const rowTotals: Record<UseCaseRiskTier, number> = {
      tier1: 0, tier2: 0, tier3: 0, tier4: 0,
    };
    const colTotals: Record<UseCaseStatus, number> = {
      draft: 0, under_review: 0, approved: 0, retired: 0,
    };
    let max = 1;
    for (const uc of useCases) {
      const key = `${uc.risk_tier}|${uc.status}`;
      const next = (counts.get(key) ?? 0) + 1;
      counts.set(key, next);
      rowTotals[uc.risk_tier]++;
      colTotals[uc.status]++;
      if (next > max) max = next;
    }
    return { counts, rowTotals, colTotals, max };
  }, [useCases]);

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-baseline justify-between mb-3">
        <div>
          <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Risk × lifecycle
          </div>
          <div className="text-sm font-semibold tracking-tight mt-0.5">
            {useCases.length} use cases across the portfolio
          </div>
        </div>
        {activeCell &&
          (activeCell.tier !== "" || activeCell.status !== "") && (
            <button
              onClick={() => onSelect("", "")}
              className="text-[11px] text-primary hover:underline flex items-center gap-1"
            >
              <Icon icon={icons.close} className="h-3 w-3" />
              Clear filter
            </button>
          )}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-[11px]">
          <thead>
            <tr>
              <th className="text-left font-mono uppercase tracking-widest text-muted-foreground pb-2 pr-3"></th>
              {STATUSES.map((s) => (
                <th
                  key={s.key}
                  className="font-mono uppercase tracking-widest text-muted-foreground pb-2 px-2 text-left"
                >
                  <button
                    onClick={() => onSelect("", s.key)}
                    className={cn(
                      "hover:text-foreground transition-colors",
                      activeCell?.status === s.key &&
                        activeCell?.tier === "" &&
                        "text-foreground",
                    )}
                  >
                    {s.label}
                    <span className="ml-1.5 font-mono tabular-nums text-foreground/70">
                      {colTotals[s.key]}
                    </span>
                  </button>
                </th>
              ))}
              <th className="font-mono uppercase tracking-widest text-muted-foreground pb-2 pl-3 text-right">
                Σ
              </th>
            </tr>
          </thead>
          <tbody>
            {TIERS.map((t) => {
              const rowTotal = rowTotals[t.key];
              return (
                <tr key={t.key}>
                  <th
                    scope="row"
                    className="py-1 pr-3 text-left font-mono uppercase tracking-widest"
                  >
                    <button
                      onClick={() => onSelect(t.key, "")}
                      className={cn(
                        "inline-flex items-center gap-1.5 hover:opacity-80 transition-opacity",
                        TIER_TEXT[t.key],
                      )}
                    >
                      <span
                        className={cn(
                          "h-2 w-2 rounded-full",
                          TIER_COLOR[t.key],
                        )}
                      />
                      {t.label}
                    </button>
                  </th>
                  {STATUSES.map((s) => {
                    const count = counts.get(`${t.key}|${s.key}`) ?? 0;
                    const active =
                      activeCell?.tier === t.key && activeCell?.status === s.key;
                    const intensity = count / max;
                    return (
                      <td key={s.key} className="p-1">
                        <button
                          onClick={() => onSelect(t.key, s.key)}
                          className={cn(
                            "relative w-full h-12 rounded-md border transition-all overflow-hidden",
                            count === 0
                              ? "border-border/40 bg-muted/20 text-muted-foreground/50"
                              : "border-border hover:border-primary/50 cursor-pointer",
                            active && "ring-2 ring-primary border-primary",
                          )}
                        >
                          {count > 0 && (
                            <div
                              className={cn(
                                "absolute inset-0",
                                TIER_COLOR[t.key],
                              )}
                              style={{
                                opacity: 0.08 + intensity * 0.25,
                              }}
                            />
                          )}
                          <span
                            className={cn(
                              "relative text-base font-semibold tabular-nums",
                              count > 0 && TIER_TEXT[t.key],
                            )}
                          >
                            {count}
                          </span>
                        </button>
                      </td>
                    );
                  })}
                  <td className="py-1 pl-3 text-right font-mono tabular-nums text-foreground/70">
                    {rowTotal}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Classification mix — what classifications are in the portfolio, how
// many use cases touch each. Pulls from both declared and effective
// classifications so propagated labels show up alongside hand-tagged ones.
// ---------------------------------------------------------------------------

const CLASS_COLOR: Record<string, string> = {
  pii: "bg-rose-500",
  pci: "bg-rose-500",
  phi: "bg-rose-500",
  "customer-financial": "bg-amber-500",
  confidential: "bg-amber-500",
  internal: "bg-sky-500",
  public: "bg-emerald-500",
};

function classTone(c: string): string {
  return CLASS_COLOR[c] ?? "bg-slate-500";
}

export function ClassificationMix({
  useCases,
}: {
  useCases: UseCase[];
}) {
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

  const max = Math.max(1, ...rows.map((r) => r.total));
  const total = useCases.length || 1;

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3">
        <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Classification mix
        </div>
        <div className="text-sm font-semibold tracking-tight mt-0.5">
          Data classes in scope
        </div>
      </div>
      {rows.length === 0 ? (
        <div className="h-32 flex items-center justify-center text-xs text-muted-foreground">
          No classifications declared
        </div>
      ) : (
        <div className="space-y-2.5">
          {rows.map((r) => (
            <div key={r.label} className="space-y-1">
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-1.5 min-w-0">
                  <span
                    className={cn("h-2 w-2 rounded-full shrink-0", classTone(r.label))}
                  />
                  <span className="text-[11px] font-mono uppercase tracking-wider truncate">
                    {r.label}
                  </span>
                </div>
                <span className="text-[11px] font-mono tabular-nums text-muted-foreground">
                  {r.total}
                  <span className="ml-1 text-muted-foreground/60">
                    · {Math.round((r.total / total) * 100)}%
                  </span>
                </span>
              </div>
              <div className="flex h-1.5 w-full rounded-full overflow-hidden bg-muted/60">
                <div
                  className={cn(classTone(r.label))}
                  style={{ width: `${(r.declared / max) * 100}%` }}
                />
                <div
                  className={cn(classTone(r.label), "opacity-50")}
                  style={{ width: `${(r.inherited / max) * 100}%` }}
                />
              </div>
            </div>
          ))}
          <p className="text-[10px] text-muted-foreground pt-1">
            Solid = declared · Faded = inherited from components
          </p>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Hero ring — "approval coverage" of approved UCs in block mode.
// ---------------------------------------------------------------------------

export function CoverageRing({
  enforcing,
  approved,
}: {
  enforcing: number;
  approved: number;
}) {
  const pct = approved === 0 ? 0 : Math.round((enforcing / approved) * 100);
  const tone = pct >= 80 ? "emerald" : pct >= 50 ? "amber" : "rose";
  const color =
    tone === "emerald"
      ? "text-emerald-500"
      : tone === "amber"
        ? "text-amber-500"
        : "text-rose-500";

  const r = 42;
  const c = 2 * Math.PI * r;
  const offset = c - (pct / 100) * c;

  return (
    <div className="relative w-24 h-24 shrink-0">
      <svg viewBox="0 0 100 100" className="w-full h-full -rotate-90">
        <circle
          cx="50"
          cy="50"
          r={r}
          stroke="currentColor"
          strokeWidth="6"
          fill="none"
          className="text-muted/50"
        />
        <circle
          cx="50"
          cy="50"
          r={r}
          stroke="currentColor"
          strokeWidth="6"
          fill="none"
          strokeDasharray={c}
          strokeDashoffset={offset}
          strokeLinecap="round"
          className={cn(color, "transition-all duration-700 ease-out")}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className={cn("text-2xl font-bold tabular-nums leading-none", color)}>
          {pct}
          <span className="text-base">%</span>
        </span>
        <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground mt-0.5">
          enforced
        </span>
      </div>
    </div>
  );
}
