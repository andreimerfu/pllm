import { useMemo } from "react";
import { Cell, Pie, PieChart, ResponsiveContainer } from "recharts";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type { Deployment, DeploymentStatus } from "@/types/api";

const STATUS_COLOR: Record<DeploymentStatus, string> = {
  running: "hsl(160 84% 39%)",
  deploying: "hsl(210 90% 55%)",
  pending: "hsl(215 14% 50%)",
  failed: "hsl(347 77% 50%)",
  terminating: "hsl(38 92% 50%)",
  stopped: "hsl(215 14% 35%)",
};

const STATUS_ICON: Record<DeploymentStatus, string> = {
  running: "solar:play-circle-bold",
  deploying: "solar:refresh-linear",
  pending: "solar:clock-circle-linear",
  failed: "solar:close-circle-bold",
  terminating: "solar:stop-circle-bold",
  stopped: "solar:pause-circle-linear",
};

// Cluster ring — status mix of all deployments. Click a slice (TODO) to
// filter. For now, static donut with legend.
export function ClusterRing({ deployments }: { deployments: Deployment[] }) {
  const counts = useMemo(() => {
    const c: Record<DeploymentStatus, number> = {
      running: 0,
      deploying: 0,
      pending: 0,
      failed: 0,
      terminating: 0,
      stopped: 0,
    };
    for (const d of deployments) c[d.status]++;
    return c;
  }, [deployments]);

  const pieData = (Object.keys(counts) as DeploymentStatus[])
    .filter((k) => counts[k] > 0)
    .map((k) => ({ name: k, value: counts[k] }));

  const total = deployments.length;
  const running = counts.running;

  return (
    <div className="flex items-center gap-5">
      <div className="relative w-32 h-32 shrink-0">
        {total === 0 ? (
          <div className="h-full w-full rounded-full border-4 border-muted/50" />
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={pieData}
                dataKey="value"
                innerRadius={40}
                outerRadius={60}
                paddingAngle={2}
                stroke="hsl(var(--card))"
                strokeWidth={2}
              >
                {pieData.map((d) => (
                  <Cell
                    key={d.name}
                    fill={STATUS_COLOR[d.name as DeploymentStatus]}
                  />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
        )}
        <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
          <span className="text-2xl font-bold tabular-nums leading-none">
            {running}
            <span className="text-muted-foreground/60">/{total}</span>
          </span>
          <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground mt-1">
            running
          </span>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1 flex-1 min-w-0">
        {(Object.keys(counts) as DeploymentStatus[]).map((k) => (
          <div
            key={k}
            className="flex items-center gap-1.5 text-[11px] min-w-0"
          >
            <span
              className="h-2 w-2 rounded-full shrink-0"
              style={{ backgroundColor: STATUS_COLOR[k] }}
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
  );
}

export function StatusIcon({
  status,
  className,
}: {
  status: DeploymentStatus;
  className?: string;
}) {
  return (
    <Icon
      icon={STATUS_ICON[status]}
      className={cn("h-4 w-4", className)}
      style={{ color: STATUS_COLOR[status] }}
    />
  );
}

export function StatusPill({
  status,
  withPulse = false,
}: {
  status: DeploymentStatus;
  withPulse?: boolean;
}) {
  const active = status === "running" || status === "deploying";
  return (
    <span
      className="inline-flex items-center gap-1.5 h-5 px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider"
      style={{
        backgroundColor: `${STATUS_COLOR[status]}1a`,
        color: STATUS_COLOR[status],
        boxShadow: `inset 0 0 0 1px ${STATUS_COLOR[status]}40`,
      }}
    >
      <span className="relative flex h-1.5 w-1.5">
        {withPulse && active && (
          <span
            className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75"
            style={{ backgroundColor: STATUS_COLOR[status] }}
          />
        )}
        <span
          className="relative inline-flex rounded-full h-1.5 w-1.5"
          style={{ backgroundColor: STATUS_COLOR[status] }}
        />
      </span>
      {status}
    </span>
  );
}

// Recent-applied timeline — simple vertical list of most-recently-applied
// workloads. Bank-demo scale: 5–20 deployments, a scrollable list is fine.
export function RecentApplied({
  deployments,
  limit = 6,
}: {
  deployments: Deployment[];
  limit?: number;
}) {
  const sorted = useMemo(() => {
    return [...deployments]
      .filter((d) => d.last_applied_at)
      .sort((a, b) => {
        const ta = new Date(a.last_applied_at!).getTime();
        const tb = new Date(b.last_applied_at!).getTime();
        return tb - ta;
      })
      .slice(0, limit);
  }, [deployments, limit]);

  if (sorted.length === 0) {
    return (
      <div className="h-32 flex items-center justify-center text-xs text-muted-foreground">
        No recent deployments
      </div>
    );
  }

  return (
    <ul className="space-y-2">
      {sorted.map((d) => (
        <li key={d.id} className="flex items-center gap-2 min-w-0">
          <StatusIcon status={d.status} className="shrink-0" />
          <div className="flex-1 min-w-0">
            <div className="text-[12px] font-medium truncate">
              {d.resource_name}
            </div>
            <div className="text-[10px] font-mono text-muted-foreground truncate">
              {d.namespace}/{d.workload_name}
            </div>
          </div>
          <span className="text-[10px] font-mono tabular-nums text-muted-foreground shrink-0">
            v{d.resource_version}
          </span>
        </li>
      ))}
    </ul>
  );
}

// Status-mix stacked bar — like the transport mix on MCP Servers.
export function StatusMixBar({
  deployments,
}: {
  deployments: Deployment[];
}) {
  const counts = useMemo(() => {
    const c: Record<DeploymentStatus, number> = {
      running: 0,
      deploying: 0,
      pending: 0,
      failed: 0,
      terminating: 0,
      stopped: 0,
    };
    for (const d of deployments) c[d.status]++;
    return c;
  }, [deployments]);

  const total = Math.max(1, deployments.length);

  return (
    <div className="flex h-1.5 w-full rounded-full overflow-hidden bg-muted/60">
      {(Object.keys(counts) as DeploymentStatus[]).map((k) => (
        <div
          key={k}
          className="transition-all"
          style={{
            width: `${(counts[k] / total) * 100}%`,
            backgroundColor: STATUS_COLOR[k],
          }}
        />
      ))}
    </div>
  );
}

export { icons };
