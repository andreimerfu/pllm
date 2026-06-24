import { useMemo } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { AuditLog } from "@/types/api";

type Outcome = "success" | "failure" | "error" | "warning" | "other";

const OUTCOME_COLOR: Record<Outcome, string> = {
  success: "hsl(160 84% 39%)",
  failure: "hsl(347 77% 50%)",
  error: "hsl(347 77% 40%)",
  warning: "hsl(38 92% 50%)",
  other: "hsl(215 14% 50%)",
};

function classify(r: string): Outcome {
  if (r === "success" || r === "failure" || r === "error" || r === "warning")
    return r;
  return "other";
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
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-baseline justify-between mb-2">
        <div>
          <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Signal
          </div>
          <div className="text-sm font-semibold tracking-tight mt-0.5">
            Events over time
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Legend tone="emerald" label="Success" />
          <Legend tone="amber" label="Warning" />
          <Legend tone="rose" label="Error" />
          <span className="text-[11px] font-mono tabular-nums text-muted-foreground ml-2">
            {total} shown
          </span>
        </div>
      </div>
      <div className="h-36">
        {total === 0 ? (
          <div className="h-full flex items-center justify-center text-xs text-muted-foreground">
            No events in range
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 6, right: 8, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id="a-ok" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={OUTCOME_COLOR.success} stopOpacity={0.5} />
                  <stop offset="100%" stopColor={OUTCOME_COLOR.success} stopOpacity={0} />
                </linearGradient>
                <linearGradient id="a-warn" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={OUTCOME_COLOR.warning} stopOpacity={0.5} />
                  <stop offset="100%" stopColor={OUTCOME_COLOR.warning} stopOpacity={0} />
                </linearGradient>
                <linearGradient id="a-err" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={OUTCOME_COLOR.error} stopOpacity={0.5} />
                  <stop offset="100%" stopColor={OUTCOME_COLOR.error} stopOpacity={0} />
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
              <Tooltip
                contentStyle={{
                  backgroundColor: "hsl(var(--popover))",
                  border: "1px solid hsl(var(--border))",
                  borderRadius: 6,
                  fontSize: 11,
                }}
              />
              <Area
                type="monotone"
                dataKey="success"
                stackId="1"
                stroke={OUTCOME_COLOR.success}
                strokeWidth={1.5}
                fill="url(#a-ok)"
              />
              <Area
                type="monotone"
                dataKey="warning"
                stackId="1"
                stroke={OUTCOME_COLOR.warning}
                strokeWidth={1.5}
                fill="url(#a-warn)"
              />
              <Area
                type="monotone"
                dataKey="error"
                stackId="1"
                stroke={OUTCOME_COLOR.error}
                strokeWidth={1.5}
                fill="url(#a-err)"
              />
              <Area
                type="monotone"
                dataKey="failure"
                stackId="1"
                stroke={OUTCOME_COLOR.failure}
                strokeWidth={1.5}
                fill="url(#a-err)"
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
}

function Legend({ tone, label }: { tone: "emerald" | "amber" | "rose"; label: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <span
        className={cn(
          "h-2 w-2 rounded-full",
          tone === "emerald" && "bg-emerald-500",
          tone === "amber" && "bg-amber-500",
          tone === "rose" && "bg-rose-500",
        )}
      />
      <span className="text-[11px] text-muted-foreground">{label}</span>
    </div>
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
  const successPct = total === 0 ? 0 : Math.round((counts.success / total) * 100);
  const pieData = (Object.keys(counts) as Outcome[])
    .filter((k) => counts[k] > 0)
    .map((k) => ({ name: k, value: counts[k] }));

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3">
        <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Outcomes
        </div>
        <div className="text-sm font-semibold tracking-tight mt-0.5">
          Success rate
        </div>
      </div>
      <div className="flex items-center gap-4">
        <div className="relative w-24 h-24 shrink-0">
          {total === 0 ? (
            <div className="h-full w-full rounded-full border-4 border-muted/50" />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={pieData}
                  dataKey="value"
                  innerRadius={28}
                  outerRadius={44}
                  paddingAngle={2}
                  stroke="hsl(var(--card))"
                  strokeWidth={2}
                >
                  {pieData.map((d) => (
                    <Cell key={d.name} fill={OUTCOME_COLOR[d.name as Outcome]} />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
          )}
          <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
            <span
              className={cn(
                "text-xl font-bold tabular-nums leading-none",
                successPct >= 95
                  ? "text-emerald-600 dark:text-emerald-400"
                  : successPct >= 80
                    ? "text-amber-600 dark:text-amber-400"
                    : "text-rose-600 dark:text-rose-400",
              )}
            >
              {successPct}
              <span className="text-base">%</span>
            </span>
            <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground">
              ok
            </span>
          </div>
        </div>
        <div className="space-y-1 flex-1 min-w-0">
          {(["success", "warning", "failure", "error"] as Outcome[]).map((k) => (
            <div key={k} className="flex items-center gap-1.5 text-[11px]">
              <span
                className="h-2 w-2 rounded-full shrink-0"
                style={{ backgroundColor: OUTCOME_COLOR[k] }}
              />
              <span className="text-muted-foreground capitalize truncate">{k}</span>
              <span
                className={cn(
                  "ml-auto font-mono tabular-nums",
                  counts[k] === 0 && "text-muted-foreground/40",
                )}
              >
                {counts[k]}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
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
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3">
        <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          {eyebrow}
        </div>
        <div className="text-sm font-semibold tracking-tight mt-0.5">
          {title}
        </div>
      </div>
      {rows.length === 0 ? (
        <div className="h-24 flex items-center justify-center text-xs text-muted-foreground">
          No data
        </div>
      ) : (
        <div className="space-y-2">
          {rows.map((r) => (
            <button
              key={r.key}
              onClick={() => onPick?.(r.key)}
              disabled={!onPick}
              className="group w-full text-left space-y-1"
            >
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-1.5 min-w-0">
                  <Icon icon={icon} className="h-3 w-3 shrink-0 text-muted-foreground" />
                  <span className="text-[12px] truncate group-hover:text-primary transition-colors">
                    {r.label}
                  </span>
                </div>
                <span className="text-[11px] font-mono tabular-nums text-muted-foreground">
                  {r.count}
                </span>
              </div>
              <div className="h-1 w-full rounded-full overflow-hidden bg-muted/60">
                <div
                  className="h-full bg-primary transition-all"
                  style={{ width: `${(r.count / max) * 100}%` }}
                />
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
