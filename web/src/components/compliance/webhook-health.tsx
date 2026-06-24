import { useMemo } from "react";
import {
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
} from "recharts";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { Webhook } from "@/types/api";

type HookBucket = "healthy" | "error" | "untested" | "disabled";

const BUCKET_COLOR: Record<HookBucket, string> = {
  healthy: "hsl(160 84% 39%)",
  error: "hsl(347 77% 50%)",
  untested: "hsl(215 14% 50%)",
  disabled: "hsl(215 14% 35%)",
};

const BUCKET_LABEL: Record<HookBucket, string> = {
  healthy: "Healthy",
  error: "Error",
  untested: "Untested",
  disabled: "Disabled",
};

function classify(h: Webhook): HookBucket {
  if (!h.enabled) return "disabled";
  if (!h.last_delivery_at) return "untested";
  const s = h.last_delivery_status ?? 0;
  return s >= 200 && s < 300 ? "healthy" : "error";
}

export function WebhookHealth({
  webhooks,
  onCreate,
}: {
  webhooks: Webhook[];
  onCreate: () => void;
}) {
  const buckets = useMemo(() => {
    const b: Record<HookBucket, number> = {
      healthy: 0,
      error: 0,
      untested: 0,
      disabled: 0,
    };
    for (const h of webhooks) b[classify(h)]++;
    return b;
  }, [webhooks]);

  const pieData = (Object.keys(buckets) as HookBucket[])
    .filter((k) => buckets[k] > 0)
    .map((k) => ({ name: BUCKET_LABEL[k], value: buckets[k], bucket: k }));

  const total = webhooks.length;

  return (
    <div className="rounded-lg border border-border bg-card p-4 space-y-4">
      <div className="flex items-baseline justify-between gap-2">
        <div>
          <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Outbound
          </div>
          <div className="text-sm font-semibold tracking-tight mt-0.5">
            Webhook delivery health
          </div>
        </div>
        <button
          onClick={onCreate}
          className="text-[11px] font-medium text-primary hover:underline"
        >
          + new
        </button>
      </div>

      {total === 0 ? (
        <div className="h-40 flex flex-col items-center justify-center text-center gap-2">
          <Icon icon={icons.mail} className="h-8 w-8 text-muted-foreground/50" />
          <p className="text-xs text-muted-foreground max-w-[24ch]">
            No subscriptions yet. Pipe scope events to Slack, PagerDuty, or a SIEM.
          </p>
        </div>
      ) : (
        <div className="flex items-center gap-4">
          <div className="relative w-28 h-28 shrink-0">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={pieData}
                  dataKey="value"
                  innerRadius={32}
                  outerRadius={48}
                  paddingAngle={2}
                  stroke="hsl(var(--card))"
                  strokeWidth={2}
                >
                  {pieData.map((d) => (
                    <Cell key={d.bucket} fill={BUCKET_COLOR[d.bucket]} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    backgroundColor: "hsl(var(--popover))",
                    border: "1px solid hsl(var(--border))",
                    borderRadius: 6,
                    fontSize: 11,
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <span className="text-xl font-bold tabular-nums leading-none">
                {total}
              </span>
              <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground">
                total
              </span>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-x-3 gap-y-1.5 flex-1 min-w-0">
            {(Object.keys(buckets) as HookBucket[]).map((k) => (
              <div key={k} className="flex items-center gap-1.5 min-w-0">
                <span
                  className="h-2 w-2 rounded-full shrink-0"
                  style={{ backgroundColor: BUCKET_COLOR[k] }}
                />
                <span className="text-[11px] text-muted-foreground truncate">
                  {BUCKET_LABEL[k]}
                </span>
                <span
                  className={cn(
                    "text-[11px] font-mono tabular-nums ml-auto",
                    buckets[k] === 0 && "text-muted-foreground/50",
                  )}
                >
                  {buckets[k]}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
