import { useMemo } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { ScopeEvent } from "@/types/api";

export type TimelineRange = "24h" | "7d";

type Bucket = {
  label: string;
  ts: number;
  denied: number;
  flagged: number;
};

function bucketEvents(events: ScopeEvent[], range: TimelineRange): Bucket[] {
  const now = Date.now();
  if (range === "24h") {
    const buckets: Bucket[] = Array.from({ length: 24 }, (_, i) => {
      const t = new Date(now - (23 - i) * 3_600_000);
      return {
        label: `${t.getHours().toString().padStart(2, "0")}:00`,
        ts: t.getTime(),
        denied: 0,
        flagged: 0,
      };
    });
    for (const e of events) {
      if (!e.created_at) continue;
      const ts = new Date(e.created_at).getTime();
      const hoursAgo = Math.floor((now - ts) / 3_600_000);
      if (hoursAgo < 0 || hoursAgo >= 24) continue;
      const b = buckets[23 - hoursAgo];
      if (e.outcome === "denied") b.denied++;
      else b.flagged++;
    }
    return buckets;
  }
  const buckets: Bucket[] = Array.from({ length: 7 }, (_, i) => {
    const t = new Date(now - (6 - i) * 86_400_000);
    return {
      label: t.toLocaleDateString("en-US", { weekday: "short" }),
      ts: t.getTime(),
      denied: 0,
      flagged: 0,
    };
  });
  for (const e of events) {
    if (!e.created_at) continue;
    const ts = new Date(e.created_at).getTime();
    const daysAgo = Math.floor((now - ts) / 86_400_000);
    if (daysAgo < 0 || daysAgo >= 7) continue;
    const b = buckets[6 - daysAgo];
    if (e.outcome === "denied") b.denied++;
    else b.flagged++;
  }
  return buckets;
}

export function ScopeTimeline({
  events,
  range,
}: {
  events: ScopeEvent[];
  range: TimelineRange;
}) {
  const data = useMemo(() => bucketEvents(events, range), [events, range]);
  const hasAny = data.some((d) => d.denied + d.flagged > 0);

  return (
    <div className="h-48 w-full">
      {!hasAny ? (
        <div className="h-full flex items-center justify-center text-xs text-muted-foreground">
          No scope events in the last {range === "24h" ? "24 hours" : "7 days"}
        </div>
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 8, right: 8, left: -20, bottom: 0 }}>
            <defs>
              <linearGradient id="denied-grad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="hsl(347 77% 50%)" stopOpacity={0.5} />
                <stop offset="100%" stopColor="hsl(347 77% 50%)" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="flagged-grad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="hsl(38 92% 50%)" stopOpacity={0.5} />
                <stop offset="100%" stopColor="hsl(38 92% 50%)" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid
              strokeDasharray="2 4"
              stroke="hsl(var(--border))"
              vertical={false}
            />
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
              labelStyle={{ color: "hsl(var(--foreground))" }}
            />
            <Area
              type="monotone"
              dataKey="flagged"
              stackId="1"
              stroke="hsl(38 92% 50%)"
              strokeWidth={1.5}
              fill="url(#flagged-grad)"
            />
            <Area
              type="monotone"
              dataKey="denied"
              stackId="1"
              stroke="hsl(347 77% 50%)"
              strokeWidth={1.5}
              fill="url(#denied-grad)"
            />
          </AreaChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}
