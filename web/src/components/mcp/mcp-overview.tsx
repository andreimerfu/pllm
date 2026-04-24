import { useMemo } from "react";
import { Cell, Pie, PieChart, ResponsiveContainer } from "recharts";
import { Icon } from "@iconify/react";
import { cn } from "@/lib/utils";
import type { MCPHealth, MCPServer, MCPTransport } from "@/types/api";

const HEALTH_COLOR: Record<MCPHealth, string> = {
  healthy: "hsl(160 84% 39%)",
  unhealthy: "hsl(347 77% 50%)",
  unknown: "hsl(215 14% 50%)",
};

const TRANSPORT_ICON: Record<MCPTransport, string> = {
  stdio: "solar:command-linear",
  http: "solar:server-linear",
  sse: "solar:bolt-linear",
};

const TRANSPORT_COLOR: Record<MCPTransport, string> = {
  stdio: "bg-slate-500",
  http: "bg-sky-500",
  sse: "bg-violet-500",
};

// Small horizontal distribution strip — always shows all three transports
// even if zero, so operators can see the mix at a glance.
export function TransportMix({ servers }: { servers: MCPServer[] }) {
  const counts = useMemo(() => {
    const c: Record<MCPTransport, number> = { stdio: 0, http: 0, sse: 0 };
    for (const s of servers) c[s.transport]++;
    return c;
  }, [servers]);

  const total = Math.max(1, servers.length);

  return (
    <div className="space-y-2">
      <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        Transport mix
      </div>
      <div className="flex h-2 w-full rounded-full overflow-hidden bg-muted/60">
        {(["stdio", "http", "sse"] as MCPTransport[]).map((t) => (
          <div
            key={t}
            className={cn(TRANSPORT_COLOR[t], "transition-all")}
            style={{ width: `${(counts[t] / total) * 100}%` }}
          />
        ))}
      </div>
      <div className="flex items-center gap-3 flex-wrap">
        {(["stdio", "http", "sse"] as MCPTransport[]).map((t) => (
          <div key={t} className="flex items-center gap-1.5">
            <span
              className={cn("h-2 w-2 rounded-full", TRANSPORT_COLOR[t])}
            />
            <span className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground">
              {t}
            </span>
            <span className="text-[11px] font-mono tabular-nums font-semibold">
              {counts[t]}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// Health donut — compact, reuses the Compliance page aesthetic.
export function HealthDonut({ servers }: { servers: MCPServer[] }) {
  const counts = useMemo(() => {
    const c: Record<MCPHealth, number> = { healthy: 0, unhealthy: 0, unknown: 0 };
    for (const s of servers) {
      const h = (s.live_health ?? s.health_status) as MCPHealth;
      c[h]++;
    }
    return c;
  }, [servers]);

  const pieData = (Object.keys(counts) as MCPHealth[])
    .filter((k) => counts[k] > 0)
    .map((k) => ({ name: k, value: counts[k] }));

  const total = servers.length;

  return (
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
                stroke="hsl(var(--background))"
                strokeWidth={2}
              >
                {pieData.map((d) => (
                  <Cell key={d.name} fill={HEALTH_COLOR[d.name as MCPHealth]} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
        )}
        <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
          <span className="text-xl font-bold tabular-nums leading-none">
            {counts.healthy}
            <span className="text-muted-foreground/60">/{total}</span>
          </span>
          <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground">
            healthy
          </span>
        </div>
      </div>
      <div className="space-y-1.5 flex-1 min-w-0">
        {(Object.keys(counts) as MCPHealth[]).map((k) => (
          <div key={k} className="flex items-center gap-1.5 text-[11px]">
            <span
              className="h-2 w-2 rounded-full shrink-0"
              style={{ backgroundColor: HEALTH_COLOR[k] }}
            />
            <span className="text-muted-foreground capitalize truncate">{k}</span>
            <span className="ml-auto font-mono tabular-nums">{counts[k]}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function TransportBadge({ transport }: { transport: MCPTransport }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 h-5 px-1.5 rounded ring-1 text-[10px] font-mono uppercase tracking-wider",
        transport === "stdio" && "bg-slate-500/10 text-slate-700 dark:text-slate-300 ring-slate-500/30",
        transport === "http" && "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
        transport === "sse" && "bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-violet-500/30",
      )}
    >
      <Icon icon={TRANSPORT_ICON[transport]} className="h-2.5 w-2.5" />
      {transport}
    </span>
  );
}

export function HealthDot({
  health,
  withPulse = false,
}: {
  health: MCPHealth;
  withPulse?: boolean;
}) {
  const color =
    health === "healthy"
      ? "bg-emerald-500"
      : health === "unhealthy"
        ? "bg-rose-500"
        : "bg-muted-foreground/40";
  return (
    <span className="relative flex h-2 w-2 shrink-0">
      {withPulse && health === "healthy" && (
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
      )}
      <span className={cn("relative inline-flex rounded-full h-2 w-2", color)} />
    </span>
  );
}
