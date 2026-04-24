import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import {
  axiosInstance,
  getMCPServers,
  getRegistryAgent,
  getRegistryPrompt,
  listRegistryAgents,
} from "@/lib/api";
import type {
  MCPServer,
  MCPServerTool,
  RegistryAgent,
  RegistryAgentWithRefs,
  RegistryRef,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/common/EmptyState";
import { KindIcon } from "@/components/registry/kind-icon";
import {
  TargetCombobox,
  useChatTargets,
} from "@/components/playground/target-combobox";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type ChatRole = "system" | "user" | "assistant" | "tool";
type OpenAIToolCall = {
  id: string;
  type: "function";
  function: { name: string; arguments: string };
};
type ChatMsg = {
  role: ChatRole;
  content: string | null;
  tool_calls?: OpenAIToolCall[];
  tool_call_id?: string;
  name?: string;
};

type TraceEntry =
  | { kind: "llm_request"; ts: number; model: string; tool_count: number; msg_count: number }
  | { kind: "llm_response"; ts: number; latency_ms: number; has_tool_calls: boolean; content_preview?: string }
  | { kind: "tool_call"; ts: number; tool: string; args_preview: string }
  | { kind: "tool_result"; ts: number; tool: string; latency_ms: number; ok: boolean; preview?: string }
  | { kind: "error"; ts: number; message: string };

// ---------------------------------------------------------------------------
// Tool naming — we prefix tool names with `<slug>__` so a single assistant
// can route a tool call back to the right MCP backend.
// ---------------------------------------------------------------------------

function qualifyTool(slug: string, name: string): string {
  return `${slug}__${name}`;
}
function splitQualified(qualified: string): { slug: string; name: string } | null {
  const i = qualified.indexOf("__");
  if (i < 0) return null;
  return { slug: qualified.slice(0, i), name: qualified.slice(i + 2) };
}

// ---------------------------------------------------------------------------
// Ref → MCPServer resolution. We try (in order): local_name match, last
// path segment match, full name match. Mirrors backend ScopeResolver.
// ---------------------------------------------------------------------------

function resolveServer(
  ref: RegistryRef,
  servers: MCPServer[],
): MCPServer | null {
  const tryMatch = (v: string) =>
    servers.find((s) => s.slug === v || s.name === v) ?? null;
  if (ref.local_name) {
    const m = tryMatch(ref.local_name);
    if (m) return m;
  }
  const tail = ref.target_name.split("/").pop();
  if (tail) {
    const m = tryMatch(tail);
    if (m) return m;
  }
  return tryMatch(ref.target_name);
}

// ---------------------------------------------------------------------------
// Mode
// ---------------------------------------------------------------------------

export function AgentMode({
  onOpenRegistry,
}: {
  onOpenRegistry: () => void;
}) {
  const { data: agentsData } = useQuery({
    queryKey: ["registry-agents", { latest: true }],
    queryFn: () => listRegistryAgents({ latest: true }),
  });
  const agents: RegistryAgent[] = agentsData?.agents ?? [];

  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [selectedVersion, setSelectedVersion] = useState<string | null>(null);
  const [trace, setTrace] = useState<TraceEntry[]>([]);

  // Auto-pick first agent when list loads.
  useEffect(() => {
    if (!selectedName && agents.length > 0) {
      setSelectedName(agents[0].name);
      setSelectedVersion(agents[0].version);
    }
  }, [agents, selectedName]);

  const { data: agentDetail } = useQuery({
    queryKey: ["registry-agent", selectedName, selectedVersion],
    queryFn: () => getRegistryAgent(selectedName!, selectedVersion ?? undefined),
    enabled: !!selectedName,
  });

  // Reset trace when switching agents.
  useEffect(() => {
    setTrace([]);
  }, [selectedName, selectedVersion]);

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[260px_1fr_320px] h-full min-h-0">
      {/* Agent list */}
      <aside className="border-r border-border bg-muted/20 flex flex-col min-h-0">
        <div className="px-3 py-2 border-b border-border flex items-center justify-between">
          <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Agents
          </span>
          <button
            onClick={onOpenRegistry}
            className="text-[10px] text-primary hover:underline"
          >
            registry
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {agents.length === 0 && (
            <EmptyState
              icon={icons.agent}
              title="No agents yet"
              description="Publish an agent to the registry to test it here."
            />
          )}
          {agents.map((a) => {
            const active = a.name === selectedName;
            return (
              <button
                key={a.id}
                onClick={() => {
                  setSelectedName(a.name);
                  setSelectedVersion(a.version);
                }}
                className={cn(
                  "w-full text-left rounded-md px-2.5 py-2 space-y-0.5 transition-colors",
                  active
                    ? "bg-primary/10 ring-1 ring-primary/30"
                    : "hover:bg-muted/60",
                )}
              >
                <div className="flex items-center gap-1.5 min-w-0">
                  <KindIcon kind="agent" name={a.name} size="sm" className="!h-6 !w-6 !rounded-md" />
                  <span className="text-[13px] font-semibold truncate">
                    {a.title || a.name.split("/").pop()}
                  </span>
                </div>
                <div className="pl-7 flex items-center gap-1.5">
                  <code className="text-[10px] font-mono text-muted-foreground truncate">
                    v{a.version}
                  </code>
                  {a.model_name && (
                    <>
                      <span className="text-[10px] text-muted-foreground">·</span>
                      <span className="text-[10px] font-mono text-muted-foreground truncate">
                        {a.model_name}
                      </span>
                    </>
                  )}
                </div>
              </button>
            );
          })}
        </div>
      </aside>

      {/* Agent playground */}
      <main className="flex flex-col min-h-0 overflow-hidden">
        {!agentDetail ? (
          <div className="flex-1 flex items-center justify-center p-8">
            <EmptyState
              icon={icons.agent}
              title="Pick an agent"
              description="Select an agent on the left. Its prompt, tools, and LLM auto-wire into the chat below."
            />
          </div>
        ) : (
          <AgentChat detail={agentDetail} setTrace={setTrace} />
        )}
      </main>

      {/* Trace rail */}
      <aside className="border-l border-border bg-muted/20 flex flex-col min-h-0 overflow-hidden">
        <div className="px-3 py-2 border-b border-border">
          <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Trace
          </span>
        </div>
        <div className="flex-1 overflow-y-auto p-2 space-y-1.5">
          {trace.length === 0 ? (
            <p className="text-xs text-muted-foreground p-2 text-center">
              Start a run to see trace entries here.
            </p>
          ) : (
            trace.map((e, i) => <TraceRow key={i} entry={e} />)
          )}
        </div>
      </aside>
    </div>
  );
}

// ---------------------------------------------------------------------------
// AgentChat — builds messages, calls /v1/chat/completions, handles tool loop
// ---------------------------------------------------------------------------

function AgentChat({
  detail,
  setTrace,
}: {
  detail: RegistryAgentWithRefs;
  setTrace: React.Dispatch<React.SetStateAction<TraceEntry[]>>;
}) {
  const { toast } = useToast();
  const { agent } = detail;

  const { data: serversData } = useQuery({
    queryKey: ["mcp-servers"],
    queryFn: getMCPServers,
    refetchInterval: 10_000,
  });
  const mcpServers: MCPServer[] = (serversData ?? []) as MCPServer[];

  const { data: targets } = useChatTargets();
  const targetList = targets ?? [];

  // Resolve the linked prompt (first one). We fetch its content lazily.
  const firstPromptRef = detail.prompts?.[0];
  const { data: promptData } = useQuery({
    queryKey: [
      "registry-prompt",
      firstPromptRef?.target_name,
      firstPromptRef?.target_version,
    ],
    queryFn: () =>
      getRegistryPrompt(
        firstPromptRef!.target_name,
        firstPromptRef?.target_version,
      ),
    enabled: !!firstPromptRef,
  });

  // Pick target: prefer the agent's declared model, fall back to first
  // available target (routes first since they're the policy-enforced path).
  const [model, setModel] = useState<string>("");
  useEffect(() => {
    const preferred = agent.model_name ?? "";
    if (preferred && targetList.some((t) => t.id === preferred)) {
      setModel(preferred);
      return;
    }
    if (!model && targetList.length > 0) setModel(targetList[0].id);
  }, [agent.model_name, targetList, model]);

  // Resolve server refs → live MCPServer rows + build tools.
  const resolvedServers = useMemo(() => {
    return (detail.servers ?? []).map((ref) => {
      const server = resolveServer(ref, mcpServers);
      return { ref, server };
    });
  }, [detail.servers, mcpServers]);

  const openAITools = useMemo(() => {
    const tools: any[] = [];
    for (const { server } of resolvedServers) {
      if (!server || !server.enabled) continue;
      for (const t of server.tools ?? []) {
        tools.push({
          type: "function",
          function: {
            name: qualifyTool(server.slug, t.name),
            description: t.description,
            parameters: t.input_schema ?? { type: "object", properties: {} },
          },
        });
      }
    }
    return tools;
  }, [resolvedServers]);

  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [input, setInput] = useState("");
  const [pending, setPending] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Reset when agent changes. Parent clears `trace` — we only clear messages.
  useEffect(() => {
    setMessages([]);
  }, [agent.id]);

  useEffect(() => {
    scrollRef.current?.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: "smooth",
    });
  }, [messages, pending]);

  const systemMessage: string = useMemo(() => {
    const parts: string[] = [];
    if (promptData?.template) parts.push(promptData.template);
    if (detail.skills && detail.skills.length > 0) {
      parts.push(
        `\n\nAvailable skills:\n${detail.skills.map((s) => `- ${s.target_name}`).join("\n")}`,
      );
    }
    return parts.join("").trim();
  }, [promptData, detail.skills]);

  const addTrace = (e: TraceEntry) => setTrace((prev) => [...prev, e]);

  const runLoop = async (initialMessages: ChatMsg[]) => {
    let loopMessages = initialMessages;
    for (let turn = 0; turn < 8; turn++) {
      const reqStart = performance.now();
      addTrace({
        kind: "llm_request",
        ts: Date.now(),
        model,
        tool_count: openAITools.length,
        msg_count: loopMessages.length,
      });

      const res = await fetch(
        `${import.meta.env.DEV ? "http://localhost:8080" : ""}/v1/chat/completions`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization:
              "Bearer " +
              (localStorage.getItem("masterKey") ??
                "sk-master-dev-key-change-in-production"),
          },
          body: JSON.stringify({
            model,
            messages: loopMessages.map(serializeMsg),
            tools: openAITools.length ? openAITools : undefined,
            stream: false,
          }),
        },
      );
      const latency_ms = Math.round(performance.now() - reqStart);

      if (!res.ok) {
        const text = await res.text();
        addTrace({
          kind: "error",
          ts: Date.now(),
          message: `HTTP ${res.status}: ${text.slice(0, 200)}`,
        });
        throw new Error(text);
      }
      const body = await res.json();
      const choice = body?.choices?.[0];
      const assistantRaw = choice?.message;
      const content: string | null = assistantRaw?.content ?? null;
      const toolCalls: OpenAIToolCall[] = assistantRaw?.tool_calls ?? [];

      addTrace({
        kind: "llm_response",
        ts: Date.now(),
        latency_ms,
        has_tool_calls: toolCalls.length > 0,
        content_preview: content?.slice(0, 120) ?? undefined,
      });

      const assistantMsg: ChatMsg = {
        role: "assistant",
        content: content,
        tool_calls: toolCalls.length ? toolCalls : undefined,
      };
      loopMessages = [...loopMessages, assistantMsg];
      setMessages((prev) => [...prev, assistantMsg]);

      if (toolCalls.length === 0) {
        return;
      }

      // Invoke each tool, append results.
      for (const tc of toolCalls) {
        const split = splitQualified(tc.function.name);
        const toolPreview = tc.function.arguments.slice(0, 100);
        addTrace({
          kind: "tool_call",
          ts: Date.now(),
          tool: tc.function.name,
          args_preview: toolPreview,
        });
        const tStart = performance.now();
        try {
          if (!split) throw new Error(`malformed tool name ${tc.function.name}`);
          let parsedArgs: any = {};
          try {
            parsedArgs = tc.function.arguments
              ? JSON.parse(tc.function.arguments)
              : {};
          } catch {
            /* pass through as-is */
          }
          const result = (await axiosInstance.post(
            "/api/admin/mcp/tools/invoke",
            {
              server_slug: split.slug,
              tool_name: split.name,
              arguments: parsedArgs,
            },
          )) as { content?: any[]; isError?: boolean; error?: any };
          const textParts =
            (result.content ?? [])
              .filter((c: any) => c.type === "text")
              .map((c: any) => c.text)
              .join("\n\n") ?? "";
          const toolMsg: ChatMsg = {
            role: "tool",
            content: textParts || JSON.stringify(result.content ?? result),
            tool_call_id: tc.id,
            name: tc.function.name,
          };
          loopMessages = [...loopMessages, toolMsg];
          setMessages((prev) => [...prev, toolMsg]);
          addTrace({
            kind: "tool_result",
            ts: Date.now(),
            tool: tc.function.name,
            latency_ms: Math.round(performance.now() - tStart),
            ok: !result.isError && !result.error,
            preview: textParts.slice(0, 140),
          });
        } catch (e: any) {
          const toolMsg: ChatMsg = {
            role: "tool",
            content: `Tool invocation failed: ${e?.response?.data?.error ?? e.message}`,
            tool_call_id: tc.id,
            name: tc.function.name,
          };
          loopMessages = [...loopMessages, toolMsg];
          setMessages((prev) => [...prev, toolMsg]);
          addTrace({
            kind: "tool_result",
            ts: Date.now(),
            tool: tc.function.name,
            latency_ms: Math.round(performance.now() - tStart),
            ok: false,
            preview: e?.response?.data?.error ?? e.message,
          });
        }
      }
      // Loop again with tool results.
    }
    addTrace({
      kind: "error",
      ts: Date.now(),
      message: "Max turns reached (8) — loop halted to avoid runaway cost.",
    });
  };

  const send = async () => {
    if (pending) return;
    if (!input.trim() || !model) return;
    setPending(true);
    const userMsg: ChatMsg = { role: "user", content: input.trim() };
    const sys: ChatMsg[] = systemMessage
      ? [{ role: "system", content: systemMessage }]
      : [];
    const next = messages.length === 0
      ? [...sys, userMsg]
      : [...messages, userMsg];
    setMessages(next);
    setInput("");
    try {
      await runLoop(next);
    } catch (e: any) {
      toast({
        title: "Run failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    } finally {
      setPending(false);
    }
  };

  const canSend = !pending && input.trim().length > 0 && !!model;

  const missingServers = resolvedServers.filter((r) => !r.server);

  return (
    <>
      {/* Header */}
      <div className="border-b border-border px-5 py-3 space-y-2">
        <div className="flex items-center gap-3">
          <KindIcon kind="agent" name={agent.name} size="md" />
          <div className="min-w-0 flex-1">
            <h2 className="text-sm font-semibold truncate">
              {agent.title || agent.name.split("/").pop()}
            </h2>
            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground min-w-0">
              <code className="font-mono truncate">{agent.name}</code>
              <span>·</span>
              <span className="font-mono">v{agent.version}</span>
            </div>
          </div>
        </div>

        {/* Agent-wired summary */}
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              Target
            </span>
            <TargetCombobox
              targets={targetList}
              value={model}
              onChange={setModel}
              widthClass="w-[220px]"
            />
          </div>
          <span
            className={cn(
              "inline-flex items-center gap-1 h-6 px-2 rounded-md ring-1 text-[11px] font-mono",
              firstPromptRef
                ? "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30"
                : "bg-muted text-muted-foreground ring-border",
            )}
            title={
              firstPromptRef
                ? `System prompt: ${firstPromptRef.target_name}`
                : "No prompt linked"
            }
          >
            <Icon icon={icons.prompt} className="h-3 w-3" />
            {firstPromptRef
              ? firstPromptRef.target_name.split("/").pop()
              : "no prompt"}
          </span>
          <span
            className={cn(
              "inline-flex items-center gap-1 h-6 px-2 rounded-md ring-1 text-[11px] font-mono",
              openAITools.length > 0
                ? "bg-primary/10 text-primary ring-primary/30"
                : "bg-muted text-muted-foreground ring-border",
            )}
          >
            <Icon icon={icons.mcp} className="h-3 w-3" />
            {openAITools.length} tools
            {missingServers.length > 0 && (
              <span className="text-rose-600 dark:text-rose-400 ml-1">
                · {missingServers.length} unresolved
              </span>
            )}
          </span>
          <span className="inline-flex items-center gap-1 h-6 px-2 rounded-md ring-1 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 ring-emerald-500/30 text-[11px] font-mono">
            <Icon icon={icons.skill} className="h-3 w-3" />
            {detail.skills?.length ?? 0} skills
          </span>
          <button
            onClick={() => {
              setMessages([]);
              setTrace([]);
            }}
            disabled={messages.length === 0}
            className="ml-auto text-[11px] text-muted-foreground hover:text-foreground disabled:opacity-40"
          >
            Reset
          </button>
        </div>
        {missingServers.length > 0 && (
          <div className="text-[11px] text-rose-600 dark:text-rose-400 flex items-center gap-1.5">
            <Icon icon={icons.warning} className="h-3 w-3" />
            Couldn't resolve:{" "}
            {missingServers.map((r) => r.ref.target_name).join(", ")}
          </div>
        )}
      </div>

      {/* Body: messages + trace (trace rendered in portal target) */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto p-5 space-y-3">
        {messages.filter((m) => m.role !== "system").length === 0 && (
          <div className="text-center text-xs text-muted-foreground py-8">
            Type below to run this agent.
          </div>
        )}
        {messages.map((m, i) =>
          m.role === "system" ? null : <AgentMsgBubble key={i} msg={m} />,
        )}
        {pending && (
          <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
            <div className="animate-spin rounded-full h-3 w-3 border-b-2 border-primary" />
            running…
          </div>
        )}
      </div>

      {/* Composer */}
      <div className="border-t border-border p-3 flex items-end gap-2">
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send();
            }
          }}
          placeholder="Ask the agent something — Enter to send"
          className="flex-1"
        />
        <Button onClick={send} disabled={!canSend} className="gap-1.5 shrink-0">
          <Icon icon={icons.send} className="h-3.5 w-3.5" />
          Run
        </Button>
      </div>

    </>
  );
}

function serializeMsg(m: ChatMsg): any {
  const out: any = { role: m.role };
  if (m.content !== null && m.content !== undefined) out.content = m.content;
  if (m.tool_calls) out.tool_calls = m.tool_calls;
  if (m.tool_call_id) out.tool_call_id = m.tool_call_id;
  if (m.name) out.name = m.name;
  return out;
}

function AgentMsgBubble({ msg }: { msg: ChatMsg }) {
  if (msg.role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[80%] rounded-lg bg-primary text-primary-foreground px-3 py-2 text-[13px]">
          {msg.content}
        </div>
      </div>
    );
  }
  if (msg.role === "assistant") {
    return (
      <div className="flex gap-2 items-start">
        <div className="h-6 w-6 rounded-full bg-muted ring-1 ring-border flex items-center justify-center shrink-0">
          <Icon icon={icons.agent} className="h-3.5 w-3.5 text-primary" />
        </div>
        <div className="flex-1 space-y-1.5 min-w-0">
          {msg.content && (
            <div className="rounded-lg bg-muted/40 px-3 py-2 text-[13px] whitespace-pre-wrap break-words">
              {msg.content}
            </div>
          )}
          {msg.tool_calls?.map((tc) => (
            <div
              key={tc.id}
              className="rounded-md border border-primary/30 bg-primary/5 px-2.5 py-1.5 text-[11px] font-mono flex items-center gap-1.5"
            >
              <Icon icon={icons.mcp} className="h-3 w-3 text-primary" />
              <span className="text-primary font-semibold">
                {tc.function.name}
              </span>
              <span className="text-muted-foreground truncate">
                ({tc.function.arguments.slice(0, 80)}
                {tc.function.arguments.length > 80 ? "…" : ""})
              </span>
            </div>
          ))}
        </div>
      </div>
    );
  }
  if (msg.role === "tool") {
    return (
      <div className="flex gap-2 items-start pl-8">
        <div className="h-5 w-5 rounded-full bg-emerald-500/10 ring-1 ring-emerald-500/30 flex items-center justify-center shrink-0">
          <Icon
            icon={icons.check}
            className="h-3 w-3 text-emerald-600 dark:text-emerald-400"
          />
        </div>
        <div className="flex-1 rounded-md bg-emerald-500/5 ring-1 ring-emerald-500/20 px-2.5 py-1.5 text-[11px] font-mono whitespace-pre-wrap break-words text-muted-foreground">
          <span className="text-emerald-700 dark:text-emerald-400 font-semibold">
            {msg.name}
          </span>{" "}
          →{" "}
          {(msg.content ?? "").length > 200
            ? (msg.content ?? "").slice(0, 200) + "…"
            : msg.content}
        </div>
      </div>
    );
  }
  return null;
}

// ---------------------------------------------------------------------------
// TracePortal — renders into the <div id="agent-trace-rail"> target.
// ---------------------------------------------------------------------------

function TraceRow({ entry }: { entry: TraceEntry }) {
  const time = formatDistanceToNow(new Date(entry.ts), { addSuffix: true });
  if (entry.kind === "llm_request") {
    return (
      <div className="rounded-md border border-border bg-card p-2 text-[11px]">
        <div className="flex items-center gap-1.5 font-mono">
          <Icon icon={icons.bolt} className="h-3 w-3 text-primary" />
          <span className="font-semibold">LLM request</span>
          <span className="ml-auto text-muted-foreground tabular-nums">
            {time}
          </span>
        </div>
        <div className="mt-1 text-muted-foreground">
          {entry.msg_count} msgs · {entry.tool_count} tools ·{" "}
          <code>{entry.model}</code>
        </div>
      </div>
    );
  }
  if (entry.kind === "llm_response") {
    return (
      <div className="rounded-md border border-border bg-card p-2 text-[11px]">
        <div className="flex items-center gap-1.5 font-mono">
          <Icon icon={icons.bolt} className="h-3 w-3 text-muted-foreground" />
          <span className="font-semibold">LLM response</span>
          <span className="ml-auto text-muted-foreground tabular-nums">
            {entry.latency_ms}ms
          </span>
        </div>
        {entry.has_tool_calls ? (
          <div className="mt-1 text-primary font-mono">→ wants tool calls</div>
        ) : entry.content_preview ? (
          <div className="mt-1 text-muted-foreground line-clamp-2">
            "{entry.content_preview}"
          </div>
        ) : null}
      </div>
    );
  }
  if (entry.kind === "tool_call") {
    return (
      <div className="rounded-md border border-primary/30 bg-primary/5 p-2 text-[11px]">
        <div className="flex items-center gap-1.5 font-mono">
          <Icon icon={icons.mcp} className="h-3 w-3 text-primary" />
          <span className="font-semibold truncate">{entry.tool}</span>
        </div>
        <div className="mt-1 text-muted-foreground font-mono truncate">
          {entry.args_preview}
        </div>
      </div>
    );
  }
  if (entry.kind === "tool_result") {
    return (
      <div
        className={cn(
          "rounded-md border p-2 text-[11px]",
          entry.ok
            ? "border-emerald-500/30 bg-emerald-500/5"
            : "border-rose-500/30 bg-rose-500/5",
        )}
      >
        <div className="flex items-center gap-1.5 font-mono">
          <Icon
            icon={entry.ok ? icons.check : icons.error}
            className={cn(
              "h-3 w-3",
              entry.ok
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-rose-600 dark:text-rose-400",
            )}
          />
          <span className="font-semibold truncate">{entry.tool}</span>
          <span className="ml-auto text-muted-foreground tabular-nums">
            {entry.latency_ms}ms
          </span>
        </div>
        {entry.preview && (
          <div className="mt-1 text-muted-foreground line-clamp-2 font-mono">
            {entry.preview}
          </div>
        )}
      </div>
    );
  }
  if (entry.kind === "error") {
    return (
      <div className="rounded-md border border-rose-500/30 bg-rose-500/5 p-2 text-[11px]">
        <div className="flex items-center gap-1.5 font-mono">
          <Icon
            icon={icons.error}
            className="h-3 w-3 text-rose-600 dark:text-rose-400"
          />
          <span className="font-semibold">Error</span>
        </div>
        <div className="mt-1 text-rose-700 dark:text-rose-400 line-clamp-3">
          {entry.message}
        </div>
      </div>
    );
  }
  return null;
}

// Silence unused-type warnings on imports we only need for typing.
export type { MCPServerTool };
