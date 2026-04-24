import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import { axiosInstance, getMCPServers } from "@/lib/api";
import type { MCPServer, MCPServerTool } from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/common/EmptyState";
import { useToast } from "@/hooks/use-toast";
import { TransportBadge } from "@/components/mcp/mcp-overview";

// ---------------------------------------------------------------------------
// MCP mode — tool inspector
// ---------------------------------------------------------------------------

type ToolResult = {
  content?: { type: string; text?: string; data?: string }[];
  isError?: boolean;
  error?: { code: number; message: string; data?: unknown };
};

async function invokeTool(body: {
  server_slug: string;
  tool_name: string;
  arguments: Record<string, unknown> | unknown[];
}): Promise<ToolResult> {
  const res = (await axiosInstance.post(
    "/api/admin/mcp/tools/invoke",
    body,
  )) as ToolResult;
  return res;
}

export function McpMode() {
  const { toast } = useToast();
  const { data: serversData, isLoading } = useQuery({
    queryKey: ["mcp-servers"],
    queryFn: getMCPServers,
    refetchInterval: 10_000,
  });
  const servers = (serversData ?? []) as MCPServer[];

  const [serverId, setServerId] = useState<string | null>(null);
  const [toolName, setToolName] = useState<string | null>(null);
  const [argsText, setArgsText] = useState("{}");
  const [argsError, setArgsError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [result, setResult] = useState<ToolResult | null>(null);
  const [elapsedMs, setElapsedMs] = useState<number | null>(null);

  const selectedServer = useMemo(
    () => servers.find((s) => s.id === serverId) ?? null,
    [servers, serverId],
  );
  const selectedTool: MCPServerTool | null = useMemo(() => {
    if (!selectedServer || !toolName) return null;
    return (selectedServer.tools ?? []).find((t) => t.name === toolName) ?? null;
  }, [selectedServer, toolName]);

  const mutation = useMutation({
    mutationFn: invokeTool,
    onSuccess: (res) => {
      setResult(res);
    },
    onError: (e: any) => {
      toast({
        title: "Tool invocation failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const onInvoke = () => {
    if (!selectedServer || !toolName) return;
    let parsed: any = {};
    const trimmed = argsText.trim();
    if (trimmed) {
      try {
        parsed = JSON.parse(trimmed);
      } catch (e: any) {
        setArgsError(e.message);
        return;
      }
    }
    setArgsError(null);
    setResult(null);
    const t0 = performance.now();
    mutation.mutate(
      {
        server_slug: selectedServer.slug,
        tool_name: toolName,
        arguments: parsed,
      },
      {
        onSettled: () => setElapsedMs(Math.round(performance.now() - t0)),
      },
    );
  };

  const filteredServers = servers.filter((s) =>
    !search ? true : (s.name + s.slug).toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[280px_1fr] h-full min-h-0">
      {/* ── Server rail ── */}
      <aside className="border-r border-border bg-muted/20 flex flex-col min-h-0">
        <div className="p-3 border-b border-border sticky top-0 bg-inherit">
          <div className="relative">
            <Icon
              icon={icons.search}
              className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground"
            />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter servers…"
              className="h-8 pl-8 text-xs"
            />
          </div>
        </div>
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {isLoading && (
            <div className="text-xs text-muted-foreground p-2">Loading…</div>
          )}
          {!isLoading && filteredServers.length === 0 && (
            <div className="text-xs text-muted-foreground p-2">
              {servers.length === 0
                ? "No MCP servers registered."
                : "No matches."}
            </div>
          )}
          {filteredServers.map((s) => {
            const active = s.id === serverId;
            const health = s.live_health ?? s.health_status;
            return (
              <button
                key={s.id}
                onClick={() => {
                  setServerId(s.id);
                  setToolName(null);
                  setResult(null);
                }}
                className={cn(
                  "w-full text-left rounded-md px-2.5 py-2 space-y-0.5 transition-colors",
                  active
                    ? "bg-primary/10 ring-1 ring-primary/30"
                    : "hover:bg-muted/60",
                )}
              >
                <div className="flex items-center gap-1.5 min-w-0">
                  <span
                    className={cn(
                      "h-1.5 w-1.5 rounded-full shrink-0",
                      health === "healthy"
                        ? "bg-emerald-500"
                        : health === "unhealthy"
                          ? "bg-rose-500"
                          : "bg-muted-foreground/40",
                    )}
                  />
                  <span className="text-[13px] font-semibold truncate">
                    {s.name}
                  </span>
                </div>
                <div className="flex items-center gap-1.5 min-w-0 pl-3">
                  <code className="text-[10px] font-mono text-muted-foreground truncate">
                    {s.slug}
                  </code>
                  <span className="text-[10px] font-mono text-muted-foreground">
                    ·
                  </span>
                  <span className="text-[10px] font-mono text-muted-foreground">
                    {s.tool_count ?? 0} tools
                  </span>
                </div>
              </button>
            );
          })}
        </div>
      </aside>

      {/* ── Main ── */}
      <main className="flex flex-col min-h-0 overflow-hidden">
        {!selectedServer ? (
          <div className="flex-1 flex items-center justify-center p-8">
            <EmptyState
              icon={icons.mcp}
              title="Pick a server to start"
              description="Select any registered MCP backend on the left. You'll see its live tool list and can call one with JSON arguments."
            />
          </div>
        ) : (
          <>
            {/* Selected server header */}
            <div className="border-b border-border px-5 py-3 flex items-center gap-3">
              <div className="h-10 w-10 rounded-lg bg-slate-500/10 text-slate-700 dark:text-slate-300 ring-1 ring-slate-500/20 flex items-center justify-center shrink-0">
                <Icon icon={icons.mcp} className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h2 className="text-sm font-semibold truncate">
                    {selectedServer.name}
                  </h2>
                  <TransportBadge transport={selectedServer.transport} />
                </div>
                <code className="text-[11px] font-mono text-muted-foreground truncate block">
                  {selectedServer.slug} · {(selectedServer.tools ?? []).length} tools
                </code>
              </div>
            </div>

            {/* Split pane: tools list + invocation */}
            <div className="grid grid-cols-1 lg:grid-cols-[240px_1fr] flex-1 min-h-0">
              {/* Tools */}
              <div className="border-r border-border overflow-y-auto p-2 space-y-1 bg-muted/10">
                {(selectedServer.tools ?? []).length === 0 && (
                  <p className="text-xs text-muted-foreground p-2">
                    No tools cached. Probe the server to refresh.
                  </p>
                )}
                {(selectedServer.tools ?? []).map((t) => {
                  const active = t.name === toolName;
                  return (
                    <button
                      key={t.name}
                      onClick={() => {
                        setToolName(t.name);
                        setArgsText(buildArgsTemplate(t.input_schema));
                        setArgsError(null);
                        setResult(null);
                      }}
                      className={cn(
                        "w-full text-left rounded-md px-2.5 py-1.5 transition-colors",
                        active
                          ? "bg-primary/10 ring-1 ring-primary/30"
                          : "hover:bg-muted/60",
                      )}
                    >
                      <code className="text-[12px] font-mono font-semibold truncate block">
                        {t.name}
                      </code>
                      {t.description && (
                        <p className="text-[11px] text-muted-foreground line-clamp-2 mt-0.5">
                          {t.description}
                        </p>
                      )}
                    </button>
                  );
                })}
              </div>

              {/* Invocation */}
              <div className="flex flex-col min-h-0 overflow-hidden">
                {!selectedTool ? (
                  <div className="flex-1 flex items-center justify-center p-8">
                    <p className="text-sm text-muted-foreground">
                      Pick a tool from the list.
                    </p>
                  </div>
                ) : (
                  <div className="flex flex-col min-h-0 h-full">
                    <div className="px-5 py-3 border-b border-border">
                      <code className="text-sm font-mono font-semibold block truncate">
                        {selectedTool.name}
                      </code>
                      {selectedTool.description && (
                        <p className="text-[12px] text-muted-foreground mt-1 leading-relaxed">
                          {selectedTool.description}
                        </p>
                      )}
                    </div>
                    <div className="flex-1 min-h-0 grid grid-rows-[1fr_auto_1fr] overflow-hidden">
                      {/* Args editor */}
                      <div className="p-5 flex flex-col min-h-0">
                        <div className="flex items-center justify-between mb-1.5">
                          <label className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                            Arguments (JSON)
                          </label>
                          {argsError && (
                            <span className="text-[11px] text-rose-600 dark:text-rose-400 font-mono">
                              {argsError}
                            </span>
                          )}
                        </div>
                        <textarea
                          value={argsText}
                          onChange={(e) => {
                            setArgsText(e.target.value);
                            setArgsError(null);
                          }}
                          className={cn(
                            "flex-1 rounded-md border bg-background px-3 py-2 text-[12px] font-mono leading-relaxed resize-none",
                            argsError
                              ? "border-rose-500/50"
                              : "border-border",
                          )}
                          spellCheck={false}
                        />
                      </div>
                      {/* Invoke bar */}
                      <div className="px-5 py-3 border-t border-b border-border bg-muted/20 flex items-center justify-between">
                        <div className="text-[11px] text-muted-foreground">
                          {elapsedMs != null && (
                            <span>
                              Last call{" "}
                              <span className="font-mono tabular-nums">
                                {elapsedMs}ms
                              </span>
                            </span>
                          )}
                        </div>
                        <Button
                          size="sm"
                          onClick={onInvoke}
                          disabled={mutation.isPending}
                          className="gap-1.5"
                        >
                          <Icon
                            icon={mutation.isPending ? icons.refresh : icons.play}
                            className={cn(
                              "h-3.5 w-3.5",
                              mutation.isPending && "animate-spin",
                            )}
                          />
                          {mutation.isPending ? "Running…" : "Invoke"}
                        </Button>
                      </div>
                      {/* Result */}
                      <div className="p-5 flex flex-col min-h-0 overflow-hidden">
                        <label className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-1.5">
                          Result
                        </label>
                        <div className="flex-1 min-h-0 overflow-auto rounded-md border border-border bg-muted/20 p-3">
                          {!result ? (
                            <p className="text-xs text-muted-foreground">
                              Invoke the tool to see its output here.
                            </p>
                          ) : result.error ? (
                            <div>
                              <div className="text-[11px] font-mono uppercase tracking-widest text-rose-600 dark:text-rose-400 mb-1">
                                Error {result.error.code}
                              </div>
                              <p className="text-[13px] text-rose-700 dark:text-rose-300">
                                {result.error.message}
                              </p>
                            </div>
                          ) : (
                            <ResultContent content={result.content} isError={result.isError} />
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function ResultContent({
  content,
  isError,
}: {
  content?: { type: string; text?: string; data?: string }[];
  isError?: boolean;
}) {
  if (!content || content.length === 0) {
    return (
      <p className="text-xs text-muted-foreground italic">
        (empty result)
      </p>
    );
  }
  return (
    <div className="space-y-2">
      {isError && (
        <div className="text-[11px] font-mono uppercase tracking-widest text-rose-600 dark:text-rose-400">
          Tool returned an error flag
        </div>
      )}
      {content.map((c, i) => {
        if (c.type === "text") {
          return (
            <pre
              key={i}
              className="text-[12px] font-mono whitespace-pre-wrap break-words leading-relaxed"
            >
              {c.text}
            </pre>
          );
        }
        return (
          <div
            key={i}
            className="text-[11px] font-mono text-muted-foreground italic"
          >
            ({c.type} block — {c.data ? `${c.data.length} bytes` : "empty"})
          </div>
        );
      })}
    </div>
  );
}

// Seed the JSON editor with a skeleton based on the tool's input schema so
// the user doesn't start from an empty object every time.
function buildArgsTemplate(schema: unknown): string {
  if (!schema || typeof schema !== "object") return "{}";
  const s = schema as Record<string, any>;
  const props = s.properties as Record<string, any> | undefined;
  if (!props) return "{}";
  const out: Record<string, any> = {};
  for (const [key, prop] of Object.entries(props)) {
    out[key] = sampleValue(prop);
  }
  try {
    return JSON.stringify(out, null, 2);
  } catch {
    return "{}";
  }
}

function sampleValue(prop: any): any {
  if (!prop) return null;
  const t = prop.type;
  if (t === "string") return "";
  if (t === "number" || t === "integer") return 0;
  if (t === "boolean") return false;
  if (t === "array") return [];
  if (t === "object") return {};
  return null;
}
