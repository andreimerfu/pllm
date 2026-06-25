import { type ReactNode } from "react";
import { cn } from "@/lib/utils";

// PageShell is the single page-body template. One header row: title +
// optional description on the left, primary CTA(s) on the right. The
// navbar no longer carries CTAs — pages own their own action area.
//
// Contract:
//   title         plain page title
//   description   one-line subtitle (muted)
//   actions       right-aligned primary CTA(s) — buttons, pill-switches
//   stats         optional KPI row — use <StatTile> for framed, <StatStrip> for flat
//   toolbar       optional search/filter/view-switcher row below stats
//
// The old "editorial" props (eyebrow, accent, titleTail) are accepted
// but ignored so the italic-accent treatment goes away everywhere
// without touching page files. They'll be stripped in Phase 2 cleanup.
export function PageShell({
  title,
  description,
  actions,
  stats,
  toolbar,
  children,
  className,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  stats?: ReactNode;
  toolbar?: ReactNode;
  children?: ReactNode;
  className?: string;
  // Deprecated — accepted for back-compat, not rendered.
  eyebrow?: string;
  accent?: string;
  titleTail?: string;
}) {
  return (
    <div className={cn("space-y-5 pb-10", className)}>
      <header className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight leading-tight truncate">
            {title}
          </h1>
          {description && (
            <p className="text-[13px] text-muted-foreground max-w-2xl">{description}</p>
          )}
        </div>
        {actions && (
          <div className="flex items-center gap-2 shrink-0">{actions}</div>
        )}
      </header>

      {stats && (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-3">
          {stats}
        </div>
      )}

      {toolbar && <div className="flex flex-wrap items-center gap-3">{toolbar}</div>}

      {children}
    </div>
  );
}

// Value text color. Keeps the legacy emerald/amber/rose aliases working
// while adding the new semantic tones.
type Tone =
  | "default" | "primary"
  | "good" | "warn" | "bad" | "info"
  | "emerald" | "amber" | "rose";

const toneMap: Record<Tone, string> = {
  default: "text-foreground",
  primary: "text-primary",
  good: "text-good",
  warn: "text-warn",
  bad: "text-bad",
  info: "text-info",
  emerald: "text-good",
  amber: "text-warn",
  rose: "text-bad",
};

// Color of the icon tile (soft tint bg + solid fg). Kind colors + semantics.
export type IconColor =
  | "primary" | "agent" | "mcp" | "skill" | "prompt" | "model"
  | "good" | "warn" | "bad" | "info";

const tileMap: Record<IconColor, string> = {
  primary: "bg-[var(--accent-soft)] text-primary",
  agent: "bg-kind-agent/12 text-kind-agent",
  mcp: "bg-kind-mcp/12 text-kind-mcp",
  skill: "bg-kind-skill/12 text-kind-skill",
  prompt: "bg-kind-prompt/12 text-kind-prompt",
  model: "bg-kind-model/12 text-kind-model",
  good: "bg-good/12 text-good",
  warn: "bg-warn/12 text-warn",
  bad: "bg-bad/12 text-bad",
  info: "bg-info/12 text-info",
};

// StatTile — the "pLLM Console" KPI card: colored icon tile + optional
// delta chip, big mono value, label, and a dim sub line. Framed; 4–5
// across on lg. Legacy callers passing only {label,value,tone,hint,icon}
// still render correctly (icon lands in the tile, hint becomes the sub).
export function StatTile({
  label,
  value,
  tone = "default",
  hint,
  sub,
  icon,
  iconColor = "primary",
  delta,
  deltaPositive = true,
  className,
}: {
  label: string;
  value: ReactNode;
  tone?: Tone;
  hint?: ReactNode;
  sub?: ReactNode;
  icon?: ReactNode;
  iconColor?: IconColor;
  delta?: ReactNode;
  deltaPositive?: boolean;
  className?: string;
}) {
  const subLine = sub ?? hint;
  return (
    <div
      className={cn(
        "flex flex-col rounded-[14px] border border-border bg-card p-4 shadow-sm min-w-0",
        className,
      )}
    >
      {(icon || delta != null) && (
        <div className="mb-3.5 flex items-center justify-between gap-2">
          {icon ? (
            <span
              className={cn(
                "inline-flex h-[34px] w-[34px] items-center justify-center rounded-[9px] [&_svg]:h-[18px] [&_svg]:w-[18px]",
                tileMap[iconColor],
              )}
            >
              {icon}
            </span>
          ) : (
            <span />
          )}
          {delta != null && (
            <span
              className={cn(
                "inline-flex items-center gap-1 rounded-md px-[7px] py-[3px] text-[11px] font-semibold",
                deltaPositive ? "bg-good/12 text-good" : "bg-bad/12 text-bad",
              )}
            >
              {delta}
            </span>
          )}
        </div>
      )}
      <div
        className={cn(
          "font-mono text-[26px] font-semibold leading-none tracking-[-0.02em] tabular-nums",
          toneMap[tone],
        )}
      >
        {value}
      </div>
      <div className="mt-[3px] truncate text-[12.5px] text-muted-foreground">{label}</div>
      {subLine && <div className="mt-0.5 truncate text-[11.5px] text-text-dim">{subLine}</div>}
    </div>
  );
}

// Back-compat alias. New code should use StatTile; KPIStat will be
// removed once every caller is migrated.
export const KPIStat = StatTile;

// StatStrip — flat, borderless, for list pages where the stats are just
// context (Keys, Models, Routes, Deployments, MCP Servers). Use
// <StatStripItem> inside.
export function StatStrip({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-stretch divide-x divide-border/70 border-y border-border/70 bg-card/40",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function StatStripItem({
  label,
  value,
  tone = "default",
  hint,
  className,
}: {
  label: string;
  value: ReactNode;
  tone?: Tone;
  hint?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-0.5 px-4 py-2.5 min-w-[7.5rem]", className)}>
      <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground truncate">
        {label}
      </span>
      <span className={cn("text-xl font-semibold tabular-nums leading-tight", toneMap[tone])}>
        {value}
      </span>
      {hint && <span className="text-[11px] text-muted-foreground truncate">{hint}</span>}
    </div>
  );
}

// Shell for a standard "tool strip" above a table: search on the left,
// right-aligned filters/view toggle. Pages can still just compose their
// own — this is convenience.
export function PageToolbar({
  left,
  right,
  className,
}: {
  left?: ReactNode;
  right?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-wrap items-center gap-3", className)}>
      <div className="flex items-center gap-2 min-w-0 flex-1">{left}</div>
      {right && <div className="flex items-center gap-2 shrink-0">{right}</div>}
    </div>
  );
}
