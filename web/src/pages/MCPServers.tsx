import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

import {
  getMCPServers,
  deleteMCPServer,
  probeMCPServer,
} from "@/lib/api";
import type { MCPHealth, MCPServer } from "@/types/api";

import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { useToast } from "@/hooks/use-toast";
import { PageShell } from "@/components/common/page-shell";
import { cn } from "@/lib/utils";
import {
  HealthDonut,
  HealthDot,
  TransportBadge,
  TransportMix,
} from "@/components/mcp/mcp-overview";
import {
  MCPServerDrawer,
  emptyForm,
  toFormState,
  type FormState,
} from "@/components/mcp/mcp-server-drawer";
import { formatDistanceToNow } from "date-fns";

export default function MCPServers() {
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [form, setForm] = useState<FormState | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<MCPServer | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["mcp-servers"],
    queryFn: getMCPServers,
    refetchInterval: 10_000,
  });

  const servers = (data ?? []) as MCPServer[];

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteMCPServer(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      setDeleteTarget(null);
      toast({ title: "MCP server deleted" });
    },
    onError: (e: any) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const probeMutation = useMutation({
    mutationFn: (id: string) => probeMCPServer(id),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({
        title: `Probe: ${res.health}`,
        description:
          res.error ?? res.reason ?? (res.tool_count != null ? `${res.tool_count} tools cached` : undefined),
        variant: res.health === "healthy" ? "default" : "destructive",
      });
    },
    onError: (e: any) => {
      toast({ title: "Probe failed", description: e.message, variant: "destructive" });
    },
  });

  const healthy = useMemo(
    () => servers.filter((s) => (s.live_health ?? s.health_status) === "healthy").length,
    [servers],
  );
  const totalTools = useMemo(
    () => servers.reduce((n, s) => n + (s.tool_count ?? 0), 0),
    [servers],
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  const enabledCount = servers.filter((s) => s.enabled).length;

  return (
    <PageShell
      title="MCP servers"
      description="Backends proxied through /v1/mcp. Tools are aggregated as <slug>__<tool>."
      actions={
        <Button onClick={() => setForm({ ...emptyForm })} className="gap-2">
          <Icon icon={icons.plus} className="h-4 w-4" />
          Add MCP server
        </Button>
      }
    >
      {/* ────────────── Hero ────────────── */}
      <div className="rounded-lg border border-border bg-card p-5">
        <div className="grid grid-cols-1 md:grid-cols-[auto_1fr_1fr] gap-6 items-center">
          <HealthDonut servers={servers} />

          <div className="space-y-3 min-w-0">
            <div>
              <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                Gateway backends
              </div>
              <div className="text-2xl font-bold tabular-nums tracking-tight leading-none mt-1">
                {servers.length}
                <span className="text-xs font-normal font-mono uppercase tracking-widest text-muted-foreground ml-2">
                  registered
                </span>
              </div>
              <div className="text-[11px] text-muted-foreground mt-1.5">
                {enabledCount} enabled · {totalTools} aggregated tools
              </div>
            </div>
            <TransportMix servers={servers} />
          </div>

          <div className="grid grid-cols-2 gap-2">
            <HeroStat
              icon={icons.bolt}
              label="Healthy"
              value={healthy}
              tone={
                servers.length === 0
                  ? "muted"
                  : healthy === servers.length
                    ? "emerald"
                    : "amber"
              }
            />
            <HeroStat
              icon={icons.terminal}
              label="Tools"
              value={totalTools}
              tone="muted"
            />
            <HeroStat
              icon={icons.power}
              label="Enabled"
              value={enabledCount}
              tone={enabledCount > 0 ? "emerald" : "muted"}
            />
            <HeroStat
              icon={icons.error}
              label="Unhealthy"
              value={servers.length - healthy}
              tone={servers.length - healthy > 0 ? "rose" : "muted"}
            />
          </div>
        </div>
      </div>

      {servers.length === 0 ? (
        <EmptyState
          icon={icons.mcp}
          title="No MCP servers yet"
          description="Register a backend — npx, uvx, Docker, or a remote HTTP/SSE MCP server — and it will appear in /v1/mcp."
          action={
            <Button onClick={() => setForm({ ...emptyForm })} variant="outline" className="gap-2">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Add first server
            </Button>
          }
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {servers.map((s) => {
            const health = (s.live_health ?? s.health_status) as MCPHealth;
            const endpoint =
              s.transport === "stdio"
                ? `${s.command ?? ""} ${(s.args ?? []).join(" ")}`.trim()
                : s.endpoint ?? "";
            const topTools = s.tools?.slice(0, 3) ?? [];
            return (
              <div
                key={s.id}
                className="group relative rounded-lg border border-border bg-card hover:border-primary/40 hover:shadow-sm transition-all"
              >
                <div className="p-4 space-y-3">
                  <div className="flex items-start gap-3">
                    <div
                      className={cn(
                        "h-10 w-10 rounded-lg flex items-center justify-center shrink-0",
                        health === "healthy" &&
                          "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20",
                        health === "unhealthy" &&
                          "bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20",
                        health === "unknown" &&
                          "bg-muted text-muted-foreground ring-1 ring-border",
                      )}
                    >
                      <Icon icon={icons.mcp} className="h-5 w-5" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1.5">
                        <HealthDot health={health} withPulse />
                        <h3 className="font-semibold text-[15px] leading-tight truncate">
                          {s.name}
                        </h3>
                      </div>
                      <code className="text-[11px] font-mono text-muted-foreground block truncate mt-0.5">
                        {s.slug}
                      </code>
                    </div>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 opacity-60 group-hover:opacity-100"
                        >
                          <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setForm(toFormState(s))}>
                          <Icon icon={icons.edit} className="mr-2 h-4 w-4" />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => probeMutation.mutate(s.id)}>
                          <Icon icon={icons.refresh} className="mr-2 h-4 w-4" />
                          Probe health
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          className="text-destructive"
                          onClick={() => setDeleteTarget(s)}
                        >
                          <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>

                  {s.description && (
                    <p className="text-[13px] text-muted-foreground line-clamp-2 leading-relaxed">
                      {s.description}
                    </p>
                  )}

                  <div className="flex items-center gap-1.5 flex-wrap">
                    <TransportBadge transport={s.transport} />
                    {!s.enabled && (
                      <span className="inline-flex items-center h-5 px-1.5 rounded bg-muted text-muted-foreground ring-1 ring-border text-[10px] font-mono uppercase tracking-wider">
                        disabled
                      </span>
                    )}
                    {s.tool_count != null && s.tool_count > 0 && (
                      <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded bg-primary/10 text-primary ring-1 ring-primary/20 text-[10px] font-mono uppercase tracking-wider">
                        <Icon icon={icons.terminal} className="h-2.5 w-2.5" />
                        {s.tool_count} tools
                      </span>
                    )}
                    {s.last_seen_at && (
                      <span className="text-[10px] text-muted-foreground font-mono ml-auto">
                        {formatDistanceToNow(new Date(s.last_seen_at), {
                          addSuffix: true,
                        })}
                      </span>
                    )}
                  </div>

                  <div className="text-[11px] font-mono text-muted-foreground truncate bg-muted/40 rounded-md px-2 py-1">
                    {endpoint || "—"}
                  </div>

                  {topTools.length > 0 && (
                    <div className="pt-1 border-t border-border/50 space-y-1">
                      <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                        Tools
                      </div>
                      <div className="flex flex-wrap gap-1">
                        {topTools.map((t) => (
                          <code
                            key={t.name}
                            className="text-[10px] font-mono bg-muted rounded px-1.5 py-0.5 truncate max-w-[14ch]"
                            title={t.description}
                          >
                            {t.name}
                          </code>
                        ))}
                        {(s.tool_count ?? 0) > topTools.length && (
                          <span className="text-[10px] font-mono text-muted-foreground">
                            +{(s.tool_count ?? 0) - topTools.length}
                          </span>
                        )}
                      </div>
                    </div>
                  )}

                  {s.last_error && (
                    <p className="text-[11px] text-rose-600 dark:text-rose-400 line-clamp-2 bg-rose-500/5 rounded-md px-2 py-1 border border-rose-500/20">
                      {s.last_error}
                    </p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      <MCPServerDrawer
        open={!!form}
        onOpenChange={(open) => !open && setForm(null)}
        initial={form}
      />

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete MCP Server</AlertDialogTitle>
            <AlertDialogDescription>
              Remove <span className="font-mono">{deleteTarget?.slug}</span> from the gateway? Active
              sessions using its tools will start failing immediately.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}

function HeroStat({
  icon,
  label,
  value,
  tone = "muted",
}: {
  icon: string;
  label: string;
  value: number;
  tone?: "emerald" | "amber" | "rose" | "muted";
}) {
  return (
    <div className="rounded-md border border-border/70 bg-background/40 px-3 py-2.5 flex items-center gap-2.5">
      <div
        className={cn(
          "h-8 w-8 rounded-md flex items-center justify-center shrink-0",
          tone === "emerald" && "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
          tone === "amber" && "bg-amber-500/10 text-amber-600 dark:text-amber-400",
          tone === "rose" && "bg-rose-500/10 text-rose-600 dark:text-rose-400",
          tone === "muted" && "bg-muted text-muted-foreground",
        )}
      >
        <Icon icon={icon} className="h-4 w-4" />
      </div>
      <div className="min-w-0">
        <div className="text-[9px] font-mono uppercase tracking-widest text-muted-foreground leading-none">
          {label}
        </div>
        <div
          className={cn(
            "text-lg font-bold tabular-nums leading-none mt-1",
            tone === "emerald" && "text-emerald-600 dark:text-emerald-400",
            tone === "amber" && "text-amber-600 dark:text-amber-400",
            tone === "rose" && "text-rose-600 dark:text-rose-400",
          )}
        >
          {value}
        </div>
      </div>
    </div>
  );
}

