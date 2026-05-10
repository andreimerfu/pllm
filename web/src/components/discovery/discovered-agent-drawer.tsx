import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { getDiscoveredAgent } from "@/lib/api";
import { detectProvider } from "@/lib/providers";
import type { DiscoveredCall, DiscoveredCallKind } from "@/types/api";
import { Badge } from "@/components/ui/badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

const KIND_TONE: Record<DiscoveredCallKind, string> = {
  llm: "bg-indigo-100 text-indigo-800 dark:bg-indigo-950 dark:text-indigo-200",
  mcp: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200",
  vector_db: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200",
  agent: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-200",
  http: "bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-200",
  unknown: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200",
};

const OUTCOME_TONE: Record<string, string> = {
  allowed: "text-emerald-700 dark:text-emerald-400",
  flagged: "text-amber-700 dark:text-amber-400",
  blocked: "text-rose-700 dark:text-rose-400",
  denied: "text-rose-700 dark:text-rose-400",
};

// Vector DB / kind-specific icons. detectProvider() handles LLM model
// names; everything else gets a tailored iconify slug here.
const KIND_FALLBACK_ICON: Record<DiscoveredCallKind, string> = {
  llm: "solar:cpu-bolt-linear",
  mcp: "lucide:plug-zap",
  vector_db: "solar:database-linear",
  agent: "solar:users-group-rounded-linear",
  http: "solar:global-linear",
  unknown: "solar:question-circle-linear",
};

function vectorDbIcon(target: string): string {
  const t = target.toLowerCase();
  if (t.startsWith("qdrant")) return "logos:qdrant-icon";
  if (t.startsWith("pinecone")) return "simple-icons:pinecone";
  if (t.startsWith("weaviate")) return "simple-icons:weaviate";
  if (t.startsWith("chroma")) return "logos:chroma";
  if (t.startsWith("milvus")) return "simple-icons:milvus";
  return KIND_FALLBACK_ICON.vector_db;
}

interface CallVisual {
  icon: string;
  label: string;
  /** tailwind classes for icon foreground color */
  tone?: string;
}

function visualFor(call: DiscoveredCall): CallVisual {
  switch (call.kind) {
    case "llm": {
      const p = detectProvider(call.target_name, call.target_name);
      return { icon: p.icon, label: p.name, tone: p.color };
    }
    case "vector_db":
      return { icon: vectorDbIcon(call.target_name), label: call.target_name.split("://")[0] };
    case "mcp":
      return { icon: KIND_FALLBACK_ICON.mcp, label: "MCP server" };
    case "agent":
      return { icon: KIND_FALLBACK_ICON.agent, label: "agent (A2A)" };
    case "http":
      return { icon: KIND_FALLBACK_ICON.http, label: "HTTP" };
    default:
      return { icon: KIND_FALLBACK_ICON.unknown, label: "unknown" };
  }
}

interface Props {
  agentId: string | null;
  onClose: () => void;
}

export function DiscoveredAgentDrawer({ agentId, onClose }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ["discovered-agent", agentId],
    queryFn: () =>
      agentId
        ? getDiscoveredAgent(agentId)
        : Promise.reject(new Error("no id")),
    enabled: !!agentId,
    refetchInterval: agentId ? 3000 : false,
  });

  const summary = useMemo(() => {
    const buckets: Record<DiscoveredCallKind, number> = {
      llm: 0, mcp: 0, vector_db: 0, agent: 0, http: 0, unknown: 0,
    };
    (data?.calls ?? []).forEach((c) => {
      buckets[c.kind] = (buckets[c.kind] ?? 0) + c.count;
    });
    return buckets;
  }, [data]);

  return (
    <Sheet open={!!agentId} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[min(680px,100vw)] sm:max-w-[680px] overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-mono">
            {data?.agent.service_name ?? "Agent"}
          </SheetTitle>
          <SheetDescription>
            Calls observed via OpenTelemetry. Out-of-scope rows are flagged but
            not blocked at the gateway.
          </SheetDescription>
        </SheetHeader>

        {isLoading || !data ? (
          <p className="text-sm text-muted-foreground py-6">Loading…</p>
        ) : (
          <div className="mt-4 space-y-4">
            {/* Kind summary */}
            <div className="grid grid-cols-3 sm:grid-cols-6 gap-2">
              {(Object.keys(summary) as DiscoveredCallKind[]).map((k) => (
                <div
                  key={k}
                  className="rounded border p-2 text-center"
                  title={`${summary[k]} ${k} calls`}
                >
                  <Icon
                    icon={KIND_FALLBACK_ICON[k]}
                    className="h-5 w-5 mx-auto mb-1"
                  />
                  <div className="text-[10px] uppercase tracking-wide text-muted-foreground">
                    {k.replace("_", " ")}
                  </div>
                  <div className="font-mono text-sm tabular-nums">
                    {summary[k]}
                  </div>
                </div>
              ))}
            </div>

            {/* Per-call rows */}
            <div className="space-y-2">
              <h4 className="text-[10px] uppercase tracking-widest text-muted-foreground">
                Dependencies observed
              </h4>
              {data.calls.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  No calls recorded yet — run the agent to populate this.
                </p>
              ) : (
                data.calls.map((c: DiscoveredCall) => {
                  const v = visualFor(c);
                  return (
                    <div
                      key={c.id}
                      className="rounded border p-3 text-sm flex gap-3"
                    >
                      <Icon
                        icon={v.icon}
                        className={`h-7 w-7 shrink-0 ${v.tone ?? ""}`}
                      />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <Badge className={KIND_TONE[c.kind]}>{c.kind}</Badge>
                          <span className="text-xs text-muted-foreground">
                            {v.label}
                          </span>
                          <span className="ml-auto text-xs text-muted-foreground font-mono">
                            ×{c.count}
                          </span>
                        </div>
                        <p className="font-mono text-xs break-all mt-1">
                          {c.target_name}
                        </p>
                        {c.last_outcome && (
                          <p
                            className={`mt-1 text-xs ${OUTCOME_TONE[c.last_outcome] ?? ""}`}
                          >
                            <span className="uppercase tracking-wide">
                              {c.last_outcome}
                            </span>
                            {c.last_reason ? `: ${c.last_reason}` : ""}
                          </p>
                        )}
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
