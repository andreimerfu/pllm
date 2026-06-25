import { useMemo } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  XAxis,
  YAxis,
} from "recharts";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { SectionLabel } from "@/components/common/status-chip";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import type { AuditLog } from "@/types/api";

type Outcome = "success" | "failure" | "error" | "warning" | "other";

// Semantic chart colors — driven entirely by the token system. Success maps to
// the primary/good accent, warning to warn, error/failure to bad.
const OUTCOME_COLOR: Record<Outcome, string> = {
  success: "hsl(var(--primary))",
  failure: "hsl(var(--bad))",
  error: "hsl(var(--bad))",
  warning: "hsl(var(--warn))",
  other: "hsl(var(--muted-foreground))",
};

const chartConfig = {
  success: { label: "Success", color: "hsl(var(--primary))" },
  warning: { label: "Warning", color: "hsl(var(--warn))" },
  error: { label: "Error", color: "hsl(var(--bad))" },
  failure: { label: "Failure", color: "hsl(var(--bad))" },
} satisfies ChartConfig;

function classify(r: string): Outcome {
  if (r === "success" || r === "failure" || r === "error" || r === "warning")
    return r;
  return "other";
}

// Shared card shell — editorial-catalog header with a mono eyebrow + title.
function ChartCard({
  eyebrow,
  title,
  sub,
  icon,
  legend,
  children,
}: {
  eyebrow: React.ReactNode;
  title: React.ReactNode;
  sub?: React.ReactNode;
  icon?: string;
  legend?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-[14px] border border-border bg-card shadow-sm">
      <div className="flex items-start justify-between gap-3 border-b py-4 px-5">
        <div className="min-w-0">
          <SectionLabel>{eyebrow}</SectionLabel>
          <div className="mt-1 flex items-center gap-2">
            {icon && (
              <Icon icon={icon} className="h-4 w-4 shrink-0 text-text-dim" />
            )}
            <h3 className="text-base font-semibold text-foreground">{title}</h3>
          </div>
          {sub && (
            <div className="mt-0.5 text-xs text-muted-foreground">{sub}</div>
          )}
        </div>
        {legend && <div className="shrink-0 pt-0.5">{legend}</div>}
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

// Tiny dot + label legend item, tinted by semantic token.
function LegendDot({
  color,
  label,
  count,
}: {
  color: string;
  label: string;
  count?: number;
}) {
  return (
    <div className="flex items-center gap-1.5">
      <span
        className="h-2 w-2 shrink-0 rounded-full"
        style={{ backgroundColor: color }}
      />
      <span className="text-[11px] text-muted-foreground">{label}</span>
      {count !== undefined && (
        <span className="font-mono text-[11px] tabular-nums text-foreground">
          {count}
        </span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Timeline — area chart of events per hour bucket over the loaded window.
// Stacked by outcome so the eye immediately spots error spikes.
// ---------------------------------------------------------------------------

export function EventsTimeline({ logs }: { logs: AuditLog[] }) {
  const { data, total } = useMemo(() => {
    if (logs.length === 0) return { data: [], total: 0 };
    // Collect into 24 hourly buckets over the span of the loaded logs.
    // If the span is narrow (< 24h), each bucket represents less than 1h.
    const ts = logs
      .map((l) => new Date(l.timestamp).getTime())
      .filter((t) => !isNaN(t))
      .sort((a, b) => a - b);
    if (ts.length === 0) return { data: [], total: 0 };
    const minT = ts[0];
    const maxT = ts[ts.length - 1];
    const span = Math.max(maxT - minT, 3_600_000);
    const buckets = 24;
    const width = span / buckets;
    const arr = Array.from({ length: buckets }, (_, i) => {
      const t = new Date(minT + i * width);
      return {
        label:
          span <= 24 * 3_600_000
            ? `${t.getHours().toString().padStart(2, "0")}:00`
            : `${t.getMonth() + 1}/${t.getDate()}`,
        success: 0,
        failure: 0,
        error: 0,
        warning: 0,
      };
    });
    for (const l of logs) {
      const t = new Date(l.timestamp).getTime();
      if (isNaN(t)) continue;
      const idx = Math.min(buckets - 1, Math.floor((t - minT) / width));
      const o = classify(l.event_result);
      if (o !== "other") arr[idx][o]++;
    }
    return { data: arr, total: logs.length };
  }, [logs]);

  return (
    <ChartCard
      eyebrow="Signal"
      title="Events over time"
      sub="Last 24 hours"
      icon={icons.trendingUp}
      legend={
        <div className="flex items-center gap-3">
          <LegendDot color="hsl(var(--primary))" label="Success" />
          <LegendDot color="hsl(var(--warn))" label="Warning" />
          <LegendDot color="hsl(var(--bad))" label="Error" />
        </div>
      }
    >
      <div className="h-36">
        {total === 0 ? (
          <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
            No events in range
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="aspect-auto h-full w-full">
            <AreaChart data={data} margin={{ top: 6, right: 8, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id="a-ok" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--color-success)" stopOpacity={0.5} />
                  <stop offset="100%" stopColor="var(--color-success)" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="a-warn" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--color-warning)" stopOpacity={0.5} />
                  <stop offset="100%" stopColor="var(--color-warning)" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="a-err" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="var(--color-error)" stopOpacity={0.5} />
                  <stop offset="100%" stopColor="var(--color-error)" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="2 4" stroke="hsl(var(--border))" vertical={false} />
              <XAxis
                dataKey="label"
                stroke="hsl(var(--muted-foreground))"
                fontSize={10}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
              />
              <YAxis
                stroke="hsl(var(--muted-foreground))"
                fontSize={10}
                tickLine={false}
                axisLine={false}
                allowDecimals={false}
                width={32}
              />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Area
                type="monotone"
                dataKey="success"
                stackId="1"
                stroke="var(--color-success)"
                strokeWidth={1.5}
                fill="url(#a-ok)"
              />
              <Area
                type="monotone"
                dataKey="warning"
                stackId="1"
                stroke="var(--color-warning)"
                strokeWidth={1.5}
                fill="url(#a-warn)"
              />
              <Area
                type="monotone"
                dataKey="error"
                stackId="1"
                stroke="var(--color-error)"
                strokeWidth={1.5}
                fill="url(#a-err)"
              />
              <Area
                type="monotone"
                dataKey="failure"
                stackId="1"
                stroke="var(--color-failure)"
                strokeWidth={1.5}
                fill="url(#a-err)"
              />
            </AreaChart>
          </ChartContainer>
        )}
      </div>
    </ChartCard>
  );
}

// ---------------------------------------------------------------------------
// Outcome donut
// ---------------------------------------------------------------------------

export function OutcomeDonut({ logs }: { logs: AuditLog[] }) {
  const counts = useMemo(() => {
    const c: Record<Outcome, number> = {
      success: 0,
      failure: 0,
      error: 0,
      warning: 0,
      other: 0,
    };
    for (const l of logs) c[classify(l.event_result)]++;
    return c;
  }, [logs]);

  const total = logs.length;
  const successPct = total === 0 ? 0 : (counts.success / total) * 100;
  const successLabel = total === 0 ? "0" : successPct.toFixed(1);
  const pieData = (Object.keys(counts) as Outcome[])
    .filter((k) => counts[k] > 0)
    .map((k) => ({ name: k, value: counts[k] }));

  return (
    <ChartCard eyebrow="Outcomes" title="Success rate" icon={icons.auditLogs}>
      <div className="flex items-center gap-5">
        <div className="relative h-28 w-28 shrink-0">
          {total === 0 ? (
            <div className="h-full w-full rounded-full border-4 border-muted/50" />
          ) : (
            <ChartContainer config={chartConfig} className="aspect-square h-full w-full">
              <PieChart>
                <Pie
                  data={pieData}
                  dataKey="value"
                  innerRadius={34}
                  outerRadius={52}
                  paddingAngle={2}
                  stroke="hsl(var(--card))"
                  strokeWidth={2}
                >
                  {pieData.map((d) => (
                    <Cell key={d.name} fill={OUTCOME_COLOR[d.name as Outcome]} />
                  ))}
                </Pie>
              </PieChart>
            </ChartContainer>
          )}
          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
            <span
              className={cn(
                "font-mono text-xl font-bold leading-none tabular-nums",
                successPct >= 95
                  ? "text-good"
                  : successPct >= 80
                    ? "text-warn"
                    : "text-bad",
              )}
            >
              {successLabel}
              <span className="text-base">%</span>
            </span>
            <span className="mt-0.5 text-[9px] font-semibold uppercase tracking-[0.08em] text-text-dim">
              Success
            </span>
          </div>
        </div>
        <div className="min-w-0 flex-1 space-y-2">
          {(["success", "warning", "failure", "error"] as Outcome[]).map((k) => (
            <div key={k} className="flex items-center gap-2 text-[11px]">
              <span
                className="h-2 w-2 shrink-0 rounded-full"
                style={{ backgroundColor: OUTCOME_COLOR[k] }}
              />
              <span className="truncate capitalize text-muted-foreground">{k}</span>
              <span
                className={cn(
                  "ml-auto font-mono tabular-nums text-foreground",
                  counts[k] === 0 && "text-muted-foreground/40",
                )}
              >
                {counts[k]}
              </span>
            </div>
          ))}
        </div>
      </div>
    </ChartCard>
  );
}

// ---------------------------------------------------------------------------
// Top bar charts
// ---------------------------------------------------------------------------

export function TopBars({
  logs,
  kind,
  onPick,
}: {
  logs: AuditLog[];
  kind: "action" | "resource" | "user";
  onPick?: (value: string) => void;
}) {
  const rows = useMemo(() => {
    const map = new Map<string, { count: number; label: string }>();
    for (const l of logs) {
      let key = "";
      let label = "";
      if (kind === "action") {
        key = l.event_action;
        label = l.event_action;
      } else if (kind === "resource") {
        key = l.resource_type ?? "";
        label = l.resource_type ?? "";
      } else {
        key = l.user?.email ?? l.user?.name ?? (l.user ? "unknown" : "system");
        label = l.user?.name ?? l.user?.email ?? (l.user ? "—" : "System");
      }
      if (!key) continue;
      const cur = map.get(key) ?? { count: 0, label };
      cur.count++;
      map.set(key, cur);
    }
    return Array.from(map.entries())
      .map(([key, v]) => ({ key, ...v }))
      .sort((a, b) => b.count - a.count)
      .slice(0, 5);
  }, [logs, kind]);

  const icon =
    kind === "action" ? icons.bolt : kind === "resource" ? icons.folder : icons.user;
  const title =
    kind === "action" ? "Top actions" : kind === "resource" ? "Top resources" : "Top actors";
  const eyebrow =
    kind === "action" ? "What" : kind === "resource" ? "Where" : "Who";

  const max = Math.max(1, ...rows.map((r) => r.count));

  return (
    <ChartCard eyebrow={eyebrow} title={title} icon={icon}>
      {rows.length === 0 ? (
        <div className="flex h-24 items-center justify-center text-xs text-muted-foreground">
          No data
        </div>
      ) : (
        <div className="space-y-2.5">
          {rows.map((r) => (
            <button
              key={r.key}
              onClick={() => onPick?.(r.key)}
              disabled={!onPick}
              className="group w-full space-y-1 text-left"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="truncate text-[12px] text-foreground transition-colors group-hover:text-primary">
                  {r.label}
                </span>
                <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
                  {r.count}
                </span>
              </div>
              <div className="h-1 w-full overflow-hidden rounded-full bg-muted/60">
                <div
                  className="h-full bg-primary transition-all"
                  style={{ width: `${(r.count / max) * 100}%` }}
                />
              </div>
            </button>
          ))}
        </div>
      )}
    </ChartCard>
  );
}
