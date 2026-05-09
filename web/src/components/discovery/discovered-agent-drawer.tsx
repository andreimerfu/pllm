import { useQuery } from "@tanstack/react-query";
import { getDiscoveredAgent } from "@/lib/api";
import type { DiscoveredCall } from "@/types/api";
import { Badge } from "@/components/ui/badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

const KIND_TONE: Record<string, string> = {
  llm: "bg-indigo-100 text-indigo-800 dark:bg-indigo-950 dark:text-indigo-200",
  mcp: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200",
  vector_db: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200",
  http: "bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-200",
  unknown: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200",
};

const OUTCOME_TONE: Record<string, string> = {
  allowed: "text-emerald-700 dark:text-emerald-400",
  flagged: "text-amber-700 dark:text-amber-400",
  denied: "text-rose-700 dark:text-rose-400",
};

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
  });

  return (
    <Sheet open={!!agentId} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-[min(640px,100vw)] sm:max-w-[640px] overflow-y-auto">
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
          <div className="mt-4 space-y-2">
            {data.calls.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No calls recorded yet.
              </p>
            ) : (
              data.calls.map((c: DiscoveredCall) => (
                <div key={c.id} className="rounded border p-3 text-sm">
                  <div className="flex items-center gap-2 flex-wrap">
                    <Badge className={KIND_TONE[c.kind] ?? ""}>
                      {c.kind}
                    </Badge>
                    <span className="font-mono text-xs break-all">
                      {c.target_name}
                    </span>
                    <span className="ml-auto text-xs text-muted-foreground font-mono">
                      ×{c.count}
                    </span>
                  </div>
                  {c.last_outcome && (
                    <p
                      className={`mt-1 text-xs ${OUTCOME_TONE[c.last_outcome] ?? ""}`}
                    >
                      {c.last_outcome}
                      {c.last_reason ? `: ${c.last_reason}` : ""}
                    </p>
                  )}
                </div>
              ))
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
