import { useMemo } from "react";
import {
  Bar,
  BarChart,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
} from "recharts";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type {
  RegistryAgent,
  RegistryBase,
  RegistryPrompt,
  RegistryServer,
  RegistrySkill,
} from "@/types/api";

type AnyArtifact =
  | RegistryServer
  | RegistryAgent
  | RegistrySkill
  | RegistryPrompt;

export type OverviewInput = {
  servers: RegistryServer[];
  agents: RegistryAgent[];
  skills: RegistrySkill[];
  prompts: RegistryPrompt[];
};

// Third-party heuristic — registry importers use `<type>.import/<id>` as
// the namespace when bulk-ingesting from npm/PyPI. Anything else is
// treated as internal.
function isThirdParty(a: RegistryBase): boolean {
  return /\.import\//i.test(a.name);
}

// Publish timeline — 30-day bar chart of total publishes across all
// kinds. Bars read better than an area when activity is sparse — a
// single spike looks like a legit spike, not a broken chart.
export function PublishActivity({ input }: { input: OverviewInput }) {
  const { data, total, peak, peakLabel, busyDays } = useMemo(() => {
    const now = Date.now();
    const buckets = Array.from({ length: 30 }, (_, i) => {
      const t = new Date(now - (29 - i) * 86_400_000);
      return {
        label: `${t.getMonth() + 1}/${t.getDate()}`,
        ts: t.getTime(),
        count: 0,
      };
    });
    const all: AnyArtifact[] = [
      ...input.servers,
      ...input.agents,
      ...input.skills,
      ...input.prompts,
    ];
    for (const a of all) {
      if (!a.published_at) continue;
      const ts = new Date(a.published_at).getTime();
      const daysAgo = Math.floor((now - ts) / 86_400_000);
      if (daysAgo < 0 || daysAgo >= 30) continue;
      buckets[29 - daysAgo].count++;
    }
    let peak = 0;
    let peakLabel = "";
    let busyDays = 0;
    for (const b of buckets) {
      if (b.count > 0) busyDays++;
      if (b.count > peak) {
        peak = b.count;
        peakLabel = b.label;
      }
    }
    const total = buckets.reduce((s, d) => s + d.count, 0);
    return { data: buckets, total, peak, peakLabel, busyDays };
  }, [input]);

  return (
    <div className="rounded-lg border border-border bg-card p-4 flex flex-col">
      <div className="mb-3">
        <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Activity
        </div>
        <div className="text-sm font-semibold tracking-tight mt-0.5">
          Publishes · last 30 days
        </div>
      </div>

      <div className="flex items-end gap-2 mb-1">
        <span className="text-4xl font-bold tabular-nums leading-none">
          {total}
        </span>
        <span className="text-[11px] font-mono text-muted-foreground pb-1">
          total
        </span>
      </div>

      <div className="text-[11px] text-muted-foreground mb-3">
        {total === 0 ? (
          "No publishes in the last 30 days"
        ) : (
          <>
            <span className="text-foreground font-semibold">{busyDays}</span>{" "}
            active {busyDays === 1 ? "day" : "days"}
            {" · peak "}
            <span className="text-foreground font-semibold">{peak}</span>
            {" on "}
            <span className="font-mono">{peakLabel}</span>
          </>
        )}
      </div>

      <div className="h-14 mt-auto">
        {total > 0 && (
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={data}
              margin={{ top: 2, right: 0, left: 0, bottom: 0 }}
              barCategoryGap={1}
            >
              <Tooltip
                cursor={{ fill: "hsl(var(--muted) / 0.4)" }}
                contentStyle={{
                  backgroundColor: "hsl(var(--popover))",
                  border: "1px solid hsl(var(--border))",
                  borderRadius: 6,
                  fontSize: 11,
                }}
              />
              <XAxis dataKey="label" hide />
              <Bar
                dataKey="count"
                fill="hsl(var(--primary))"
                radius={[2, 2, 0, 0]}
                minPointSize={2}
              />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
}

const KIND_COLOR = {
  server: "hsl(215 14% 50%)",
  agent: "hsl(232 66% 62%)",
  skill: "hsl(160 84% 39%)",
  prompt: "hsl(38 92% 50%)",
};

export function KindBreakdown({ input }: { input: OverviewInput }) {
  const rows = [
    { key: "server", label: "MCP Servers", count: input.servers.length },
    { key: "agent", label: "Agents", count: input.agents.length },
    { key: "skill", label: "Skills", count: input.skills.length },
    { key: "prompt", label: "Prompts", count: input.prompts.length },
  ] as const;
  const total = rows.reduce((s, r) => s + r.count, 0);
  const pieData = rows.filter((r) => r.count > 0);

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-3">
        <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Portfolio
        </div>
        <div className="text-sm font-semibold tracking-tight mt-0.5">
          Kind breakdown
        </div>
      </div>
      <div className="flex items-center gap-4">
        <div className="relative w-32 h-32 shrink-0">
          {total === 0 ? (
            <div className="h-full w-full rounded-full border-4 border-muted/50" />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={pieData}
                  dataKey="count"
                  innerRadius={40}
                  outerRadius={60}
                  paddingAngle={2}
                  stroke="hsl(var(--card))"
                  strokeWidth={2}
                >
                  {pieData.map((r) => (
                    <Cell key={r.key} fill={KIND_COLOR[r.key]} />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
          )}
          <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
            <span className="text-2xl font-bold tabular-nums leading-none">
              {total}
            </span>
            <span className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground mt-1">
              total
            </span>
          </div>
        </div>
        <div className="space-y-1.5 flex-1 min-w-0">
          {rows.map((r) => (
            <div key={r.key} className="flex items-center gap-1.5 text-[11px]">
              <span
                className="h-2 w-2 rounded-full shrink-0"
                style={{ backgroundColor: KIND_COLOR[r.key] }}
              />
              <span className="text-muted-foreground capitalize truncate">
                {r.label}
              </span>
              <span
                className={cn(
                  "ml-auto font-mono tabular-nums",
                  r.count === 0 && "text-muted-foreground/50",
                )}
              >
                {r.count}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// Third-party vs internal — servers + agents pooled.
export function ThirdPartyMix({ input }: { input: OverviewInput }) {
  const { thirdParty, internal } = useMemo(() => {
    const pool: AnyArtifact[] = [...input.servers, ...input.agents];
    let tp = 0;
    let in_ = 0;
    for (const a of pool) (isThirdParty(a) ? tp++ : in_++);
    return { thirdParty: tp, internal: in_ };
  }, [input]);
  const total = thirdParty + internal;
  const tpPct = total === 0 ? 0 : Math.round((thirdParty / total) * 100);

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="mb-2">
        <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Supply chain
        </div>
        <div className="text-sm font-semibold tracking-tight mt-0.5">
          Third-party vs internal
        </div>
      </div>
      <div className="space-y-3">
        <div className="flex items-baseline gap-2">
          <span className="text-3xl font-bold tabular-nums leading-none">
            {tpPct}
            <span className="text-base">%</span>
          </span>
          <span className="text-[11px] text-muted-foreground">
            third-party
          </span>
        </div>
        <div className="flex h-2 w-full rounded-full overflow-hidden bg-muted/60">
          <div
            className="bg-violet-500 transition-all"
            style={{ width: `${tpPct}%` }}
          />
        </div>
        <div className="grid grid-cols-2 gap-2">
          <div className="flex items-center gap-1.5">
            <Icon icon={icons.globe} className="h-3 w-3 text-violet-500" />
            <span className="text-[11px] text-muted-foreground">Vendor</span>
            <span className="text-[11px] font-mono tabular-nums ml-auto">
              {thirdParty}
            </span>
          </div>
          <div className="flex items-center gap-1.5">
            <Icon icon={icons.home} className="h-3 w-3 text-muted-foreground" />
            <span className="text-[11px] text-muted-foreground">Internal</span>
            <span className="text-[11px] font-mono tabular-nums ml-auto">
              {internal}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
