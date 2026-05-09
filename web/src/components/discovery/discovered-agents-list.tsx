import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { useToast } from "@/hooks/use-toast";
import {
  ignoreDiscoveredAgent,
  listDiscoveredAgents,
  promoteDiscoveredAgent,
} from "@/lib/api";
import { icons } from "@/lib/icons";
import type { DiscoveredAgent } from "@/types/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

import { DiscoveredAgentDrawer } from "./discovered-agent-drawer";

const fmt = (iso: string) =>
  new Date(iso).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

export function DiscoveredAgentsList() {
  const qc = useQueryClient();
  const { toast } = useToast();
  const [openId, setOpenId] = useState<string | null>(null);

  // Show pending + promoted both — once an operator clicks Promote we
  // shouldn't make the row disappear. Ignored stays hidden.
  const { data: pendingData, isLoading: pendingLoading } = useQuery({
    queryKey: ["discovered-agents", "pending"],
    queryFn: () => listDiscoveredAgents("pending"),
    refetchInterval: 5000,
  });
  const { data: promotedData } = useQuery({
    queryKey: ["discovered-agents", "promoted"],
    queryFn: () => listDiscoveredAgents("promoted"),
    refetchInterval: 5000,
  });
  const data = {
    items: [...(pendingData?.items ?? []), ...(promotedData?.items ?? [])],
  };
  const isLoading = pendingLoading;

  const promote = useMutation({
    mutationFn: (id: string) => promoteDiscoveredAgent(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["discovered-agents"] });
      qc.invalidateQueries({ queryKey: ["registry-agents"] });
      toast({ title: "Promoted to RegistryAgent" });
    },
    onError: (e: any) =>
      toast({
        title: "Promote failed",
        description: e?.message ?? "unknown",
        variant: "destructive",
      }),
  });

  const ignore = useMutation({
    mutationFn: (id: string) => ignoreDiscoveredAgent(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["discovered-agents"] });
    },
  });

  const items = (data?.items ?? []) as DiscoveredAgent[];

  return (
    <div className="rounded-xl border bg-card">
      <div className="flex items-center gap-2 px-4 py-3 border-b">
        <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          OTel
        </span>
        <h3 className="text-sm font-semibold">
          Discovered <span className="italic font-serif">agents</span>
        </h3>
        {items.length > 0 ? (
          <span
            className="ml-2 inline-flex h-2 w-2 animate-pulse rounded-full bg-emerald-500"
            aria-label="live"
          />
        ) : null}
        <span className="ml-auto text-xs text-muted-foreground">
          {items.length} pending
        </span>
      </div>
      <div className="px-4 py-3">
        {isLoading ? (
          <p className="text-xs text-muted-foreground">Loading…</p>
        ) : items.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            No pending agents. Run an OTel-instrumented agent pointed at{" "}
            <code className="font-mono">/v1/traces</code>.
          </p>
        ) : (
          <ul className="divide-y">
            {items.map((a) => (
              <li
                key={a.id}
                className="flex items-center justify-between gap-3 py-3"
              >
                <div className="space-y-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-mono text-sm truncate">
                      {a.service_name}
                    </span>
                    {a.service_version && (
                      <Badge variant="outline" className="font-mono text-xs">
                        {a.service_version}
                      </Badge>
                    )}
                    {a.status === "promoted" && (
                      <Badge className="bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200">
                        promoted
                      </Badge>
                    )}
                    {a.flagged_count > 0 && (
                      <Badge variant="destructive" className="font-mono">
                        {a.flagged_count} flagged
                      </Badge>
                    )}
                    {!a.key_id && (
                      <Badge variant="outline" className="text-xs">
                        no bound key
                      </Badge>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    First seen {fmt(a.first_seen_at)} · last {fmt(a.last_seen_at)}
                  </p>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setOpenId(a.id)}
                  >
                    <Icon icon={icons.eye ?? "lucide:eye"} className="h-3.5 w-3.5" />
                    <span className="ml-1">Details</span>
                  </Button>
                  {a.status === "pending" ? (
                    <>
                      <Button
                        size="sm"
                        disabled={promote.isPending}
                        onClick={() => promote.mutate(a.id)}
                      >
                        Promote
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={ignore.isPending}
                        onClick={() => ignore.mutate(a.id)}
                      >
                        Ignore
                      </Button>
                    </>
                  ) : (
                    // Promoted rows still get a re-promote in case the
                    // RegistryAgent was deleted out from under us; the
                    // backend restores soft-deleted rows when found.
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={promote.isPending}
                      onClick={() => promote.mutate(a.id)}
                    >
                      Re-promote
                    </Button>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
      <DiscoveredAgentDrawer agentId={openId} onClose={() => setOpenId(null)} />
    </div>
  );
}
