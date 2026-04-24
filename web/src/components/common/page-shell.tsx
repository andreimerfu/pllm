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

type Tone = "default" | "primary" | "emerald" | "amber" | "rose";

const toneMap: Record<Tone, string> = {
  default: "text-foreground",
  primary: "text-primary",
  emerald: "text-emerald-600 dark:text-emerald-400",
  amber: "text-amber-600 dark:text-amber-400",
  rose: "text-rose-600 dark:text-rose-400",
};

// StatTile — framed, for dashboard-style pages where the tile IS the
// content (Dashboard, Use Cases, Compliance). 4–5 across on lg.
export function StatTile({
  label,
  value,
  tone = "default",
  hint,
  icon,
  className,
}: {
  label: string;
  value: ReactNode;
  tone?: Tone;
  hint?: ReactNode;
  icon?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-lg border border-border/70 bg-card p-3 flex flex-col gap-1 min-w-0",
        className,
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground truncate">
          {label}
        </span>
        {icon && <span className="text-muted-foreground text-[11px]">{icon}</span>}
      </div>
      <div className={cn("text-2xl font-semibold tabular-nums leading-none", toneMap[tone])}>
        {value}
      </div>
      {hint && <div className="text-[11px] text-muted-foreground truncate">{hint}</div>}
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
