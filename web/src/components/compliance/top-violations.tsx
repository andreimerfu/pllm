import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { detectProvider } from "@/lib/providers";
import { cn } from "@/lib/utils";
import type { ScopeEvent, UseCase } from "@/types/api";

type UCEntry = {
  id: string;
  name: string;
  slug?: string;
  count: number;
  denied: number;
  flagged: number;
};

type TargetEntry = {
  target: string;
  kind: "mcp_tool" | "llm";
  count: number;
  denied: number;
  flagged: number;
};

export function topUseCases(
  events: ScopeEvent[],
  useCases: UseCase[],
  n = 5,
): UCEntry[] {
  const map = new Map<string, UCEntry>();
  const ucById = new Map(useCases.map((u) => [u.id, u]));
  for (const e of events) {
    const cur = map.get(e.use_case_id);
    if (cur) {
      cur.count++;
      if (e.outcome === "denied") cur.denied++;
      else cur.flagged++;
    } else {
      const uc = ucById.get(e.use_case_id);
      map.set(e.use_case_id, {
        id: e.use_case_id,
        name: uc?.name ?? e.use_case_id.slice(0, 8),
        slug: uc?.slug,
        count: 1,
        denied: e.outcome === "denied" ? 1 : 0,
        flagged: e.outcome === "flagged" ? 1 : 0,
      });
    }
  }
  return Array.from(map.values())
    .sort((a, b) => b.count - a.count)
    .slice(0, n);
}

export function topTargets(events: ScopeEvent[], n = 5): TargetEntry[] {
  const map = new Map<string, TargetEntry>();
  for (const e of events) {
    const key = `${e.kind}::${e.target_name}`;
    const cur = map.get(key);
    if (cur) {
      cur.count++;
      if (e.outcome === "denied") cur.denied++;
      else cur.flagged++;
    } else {
      map.set(key, {
        target: e.target_name,
        kind: e.kind,
        count: 1,
        denied: e.outcome === "denied" ? 1 : 0,
        flagged: e.outcome === "flagged" ? 1 : 0,
      });
    }
  }
  return Array.from(map.values())
    .sort((a, b) => b.count - a.count)
    .slice(0, n);
}

export function UseCaseBars({
  items,
  onSelect,
}: {
  items: UCEntry[];
  onSelect?: (id: string) => void;
}) {
  const max = Math.max(1, ...items.map((i) => i.count));
  if (items.length === 0) {
    return (
      <div className="h-32 flex items-center justify-center text-xs text-muted-foreground">
        No use case violations yet
      </div>
    );
  }
  return (
    <div className="space-y-2.5">
      {items.map((it) => (
        <button
          key={it.id}
          onClick={() => onSelect?.(it.id)}
          className="group w-full text-left space-y-1"
        >
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1.5 min-w-0">
              <Icon
                icon={icons.useCase}
                className="h-3 w-3 text-primary shrink-0"
              />
              <span className="text-[12px] font-medium truncate group-hover:text-primary transition-colors">
                {it.name}
              </span>
            </div>
            <span className="text-[11px] font-mono tabular-nums text-muted-foreground">
              {it.count}
            </span>
          </div>
          <div className="flex h-1.5 w-full rounded-full overflow-hidden bg-muted/60">
            <div
              className="bg-rose-500"
              style={{ width: `${(it.denied / max) * 100}%` }}
            />
            <div
              className="bg-amber-500"
              style={{ width: `${(it.flagged / max) * 100}%` }}
            />
          </div>
        </button>
      ))}
    </div>
  );
}

export function TargetBars({ items }: { items: TargetEntry[] }) {
  const max = Math.max(1, ...items.map((i) => i.count));
  if (items.length === 0) {
    return (
      <div className="h-32 flex items-center justify-center text-xs text-muted-foreground">
        No target violations yet
      </div>
    );
  }
  return (
    <div className="space-y-2.5">
      {items.map((it) => {
        const providerInfo =
          it.kind === "llm" ? detectProvider(it.target, "") : null;
        return (
        <div key={`${it.kind}-${it.target}`} className="space-y-1">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-1.5 min-w-0">
              <Icon
                icon={
                  providerInfo?.icon ??
                  (it.kind === "mcp_tool" ? icons.mcp : icons.llm)
                }
                className={cn(
                  "h-3.5 w-3.5 shrink-0",
                  providerInfo
                    ? null
                    : it.kind === "mcp_tool"
                      ? "text-slate-500"
                      : "text-violet-500",
                )}
              />
              <code className="text-[11px] font-mono truncate">{it.target}</code>
            </div>
            <span className="text-[11px] font-mono tabular-nums text-muted-foreground">
              {it.count}
            </span>
          </div>
          <div className="flex h-1.5 w-full rounded-full overflow-hidden bg-muted/60">
            <div
              className="bg-rose-500"
              style={{ width: `${(it.denied / max) * 100}%` }}
            />
            <div
              className="bg-amber-500"
              style={{ width: `${(it.flagged / max) * 100}%` }}
            />
          </div>
        </div>
        );
      })}
    </div>
  );
}
