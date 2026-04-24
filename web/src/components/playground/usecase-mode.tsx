import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import {
  getKeys,
  listScopeEvents,
  listUseCases,
} from "@/lib/api";
import type { ScopeEvent, UseCase } from "@/types/api";
import type { ApiKey } from "@/components/keys/columns";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/common/EmptyState";
import { useToast } from "@/hooks/use-toast";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  TargetCombobox,
  useChatTargets,
  type ChatTarget,
} from "@/components/playground/target-combobox";

// ---------------------------------------------------------------------------
// Use Case mode — chat under a bound key's scope, with live scope event tail
// ---------------------------------------------------------------------------

type ChatMsg =
  | { role: "user" | "assistant" | "system"; content: string }
  | { role: "error"; content: string };

export function UseCaseMode({
  onOpenUseCases,
}: {
  onOpenUseCases: () => void;
}) {
  const { toast } = useToast();

  const { data: ucData } = useQuery({
    queryKey: ["use-cases-all"],
    queryFn: () => listUseCases(),
  });
  const useCases = ucData?.use_cases ?? [];

  const { data: keysData } = useQuery({
    queryKey: ["keys-admin"],
    queryFn: () => getKeys() as unknown as Promise<{ keys: ApiKey[] }>,
  });
  const keys = keysData?.keys ?? [];

  const { data: targets } = useChatTargets();
  const targetList = targets ?? [];

  const [ucId, setUcId] = useState<string | null>(null);
  const [keyId, setKeyId] = useState<string | null>(null);
  const [model, setModel] = useState<string>("");

  const selectedUC: UseCase | null = useMemo(
    () => useCases.find((u) => u.id === ucId) ?? null,
    [useCases, ucId],
  );
  const boundKeys = useMemo(
    () => keys.filter((k) => k.use_case_id === ucId && k.is_active && !k.revoked_at),
    [keys, ucId],
  );
  const selectedKey: ApiKey | null = useMemo(
    () => boundKeys.find((k) => k.id === keyId) ?? null,
    [boundKeys, keyId],
  );

  // Auto-pick first key when UC changes
  useEffect(() => {
    if (!ucId) {
      setKeyId(null);
      return;
    }
    if (boundKeys.length > 0 && !boundKeys.some((k) => k.id === keyId)) {
      setKeyId(boundKeys[0].id);
    } else if (boundKeys.length === 0) {
      setKeyId(null);
    }
  }, [ucId, boundKeys, keyId]);

  useEffect(() => {
    if (!model && targetList.length > 0) setModel(targetList[0].id);
  }, [targetList, model]);

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[260px_1fr_320px] h-full min-h-0">
      {/* Use case list */}
      <aside className="border-r border-border bg-muted/20 flex flex-col min-h-0">
        <div className="px-3 py-2 border-b border-border flex items-center justify-between">
          <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Use cases
          </span>
          <button
            onClick={onOpenUseCases}
            className="text-[10px] text-primary hover:underline"
          >
            manage
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {useCases.length === 0 && (
            <EmptyState
              icon={icons.useCase}
              title="No use cases"
              description="Create one first to test scope enforcement."
            />
          )}
          {useCases.map((uc) => {
            const active = uc.id === ucId;
            return (
              <button
                key={uc.id}
                onClick={() => setUcId(uc.id)}
                className={cn(
                  "w-full text-left rounded-md px-2.5 py-2 space-y-0.5 transition-colors",
                  active
                    ? "bg-primary/10 ring-1 ring-primary/30"
                    : "hover:bg-muted/60",
                )}
              >
                <div className="flex items-center gap-1.5 min-w-0">
                  <Icon
                    icon={icons.useCase}
                    className="h-3.5 w-3.5 text-primary shrink-0"
                  />
                  <span className="text-[13px] font-semibold truncate">
                    {uc.name}
                  </span>
                  {uc.enforce_scope && (
                    <Icon
                      icon={icons.shield}
                      className="h-3 w-3 text-rose-500 shrink-0"
                    />
                  )}
                </div>
                <div className="flex items-center gap-1.5 pl-5">
                  <span
                    className={cn(
                      "inline-flex items-center h-[14px] px-1 rounded ring-1 text-[9px] font-mono uppercase tracking-wider",
                      uc.risk_tier === "tier1" &&
                        "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
                      uc.risk_tier === "tier2" &&
                        "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
                      uc.risk_tier === "tier3" &&
                        "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
                      uc.risk_tier === "tier4" &&
                        "bg-muted text-muted-foreground ring-border",
                    )}
                  >
                    {uc.risk_tier.replace("tier", "t")}
                  </span>
                  <span className="text-[10px] font-mono text-muted-foreground">
                    {uc.status.replace("_", " ")}
                  </span>
                </div>
              </button>
            );
          })}
        </div>
      </aside>

      {/* Chat */}
      <main className="flex flex-col min-h-0 overflow-hidden">
        {!selectedUC ? (
          <div className="flex-1 flex items-center justify-center p-8">
            <EmptyState
              icon={icons.useCase}
              title="Pick a use case"
              description="Select a use case on the left to chat under its bound scope. You'll see scope denials and flags in the right rail as they happen."
            />
          </div>
        ) : (
          <ScopedChat
            useCase={selectedUC}
            boundKeys={boundKeys}
            selectedKey={selectedKey}
            onSelectKey={setKeyId}
            model={model}
            targets={targetList}
            onModelChange={setModel}
            onKeyMissing={() =>
              toast({
                title: "No key bound to this use case",
                description:
                  "Create a key in the Keys page and bind it to this use case before testing.",
                variant: "destructive",
              })
            }
          />
        )}
      </main>

      {/* Events rail */}
      <aside className="border-l border-border bg-muted/20 flex flex-col min-h-0 overflow-hidden">
        <div className="px-3 py-2 border-b border-border flex items-center justify-between">
          <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Scope events
          </span>
          <span className="text-[10px] font-mono text-emerald-600 dark:text-emerald-400 flex items-center gap-1">
            <span className="relative flex h-1.5 w-1.5">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500" />
            </span>
            live
          </span>
        </div>
        <ScopeEventsTail useCaseId={ucId} />
      </aside>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Scoped chat — runs /v1/chat/completions with the bound key's token
// ---------------------------------------------------------------------------

function ScopedChat({
  useCase,
  boundKeys,
  selectedKey,
  onSelectKey,
  model,
  targets,
  onModelChange,
  onKeyMissing,
}: {
  useCase: UseCase;
  boundKeys: ApiKey[];
  selectedKey: ApiKey | null;
  onSelectKey: (id: string) => void;
  model: string;
  targets: ChatTarget[];
  onModelChange: (v: string) => void;
  onKeyMissing: () => void;
}) {
  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [input, setInput] = useState("");
  const [pending, setPending] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Reset transcript when use case changes.
  useEffect(() => {
    setMessages([]);
  }, [useCase.id]);

  useEffect(() => {
    scrollRef.current?.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: "smooth",
    });
  }, [messages, pending]);

  const canSend = !pending && input.trim().length > 0 && !!selectedKey && !!model;

  const send = async () => {
    if (pending) return;
    if (!selectedKey) {
      onKeyMissing();
      return;
    }
    if (!input.trim() || !model) return;
    const user: ChatMsg = { role: "user", content: input.trim() };
    const next = [...messages, user];
    setMessages(next);
    setInput("");
    setPending(true);
    try {
      const res = await fetch(
        `${import.meta.env.DEV ? "http://localhost:8080" : ""}/v1/chat/completions`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${selectedKey.key ?? ""}`,
          },
          body: JSON.stringify({
            model,
            messages: next
              .filter((m) => m.role !== "error")
              .map((m) => ({ role: m.role, content: m.content })),
            stream: false,
          }),
        },
      );
      if (!res.ok) {
        const text = await res.text();
        setMessages((prev) => [
          ...prev,
          { role: "error", content: `HTTP ${res.status}: ${text.slice(0, 400)}` },
        ]);
        return;
      }
      const body = await res.json();
      const content: string =
        body?.choices?.[0]?.message?.content ?? "(empty response)";
      setMessages((prev) => [...prev, { role: "assistant", content }]);
    } catch (e: any) {
      setMessages((prev) => [
        ...prev,
        { role: "error", content: e.message ?? String(e) },
      ]);
    } finally {
      setPending(false);
    }
  };

  return (
    <>
      {/* Header */}
      <div className="border-b border-border px-5 py-3 space-y-2">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-lg bg-primary/10 text-primary ring-1 ring-primary/20 flex items-center justify-center shrink-0">
            <Icon icon={icons.useCase} className="h-5 w-5" />
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="text-sm font-semibold truncate">{useCase.name}</h2>
            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <code className="font-mono truncate">{useCase.slug}</code>
              <span>·</span>
              <span>
                {useCase.enforce_scope ? (
                  <span className="text-rose-600 dark:text-rose-400 font-semibold inline-flex items-center gap-1">
                    <Icon icon={icons.shield} className="h-3 w-3" />
                    blocking
                  </span>
                ) : (
                  <span className="text-amber-600 dark:text-amber-400 font-semibold">
                    flag mode
                  </span>
                )}
              </span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              Key
            </span>
            {boundKeys.length === 0 ? (
              <span className="text-[11px] text-rose-600 dark:text-rose-400">
                no bound key — scope won't activate
              </span>
            ) : (
              <Select
                value={selectedKey?.id ?? ""}
                onValueChange={onSelectKey}
              >
                <SelectTrigger className="h-7 w-[180px] text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {boundKeys.map((k) => (
                    <SelectItem key={k.id} value={k.id}>
                      {k.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
          <div className="flex items-center gap-1.5">
            <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              Target
            </span>
            <TargetCombobox
              targets={targets}
              value={model}
              onChange={onModelChange}
              widthClass="w-[220px]"
            />
          </div>
          <button
            onClick={() => setMessages([])}
            disabled={messages.length === 0}
            className="ml-auto text-[11px] text-muted-foreground hover:text-foreground disabled:opacity-40"
          >
            Clear
          </button>
        </div>
      </div>

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto p-5 space-y-3">
        {messages.length === 0 && (
          <div className="text-center text-xs text-muted-foreground py-8">
            {boundKeys.length === 0
              ? "Bind an API key to this use case, then chat here to see scope events fire in the right rail."
              : "Type below — responses will flow under this use case's scope."}
          </div>
        )}
        {messages.map((m, i) => (
          <MessageBubble key={i} msg={m} />
        ))}
        {pending && (
          <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
            <div className="animate-spin rounded-full h-3 w-3 border-b-2 border-primary" />
            thinking…
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
          placeholder={
            boundKeys.length === 0
              ? "Bind a key first…"
              : "Say something — Enter to send"
          }
          disabled={boundKeys.length === 0}
          className="flex-1"
        />
        <Button
          onClick={send}
          disabled={!canSend}
          className="gap-1.5 shrink-0"
        >
          <Icon icon={icons.send} className="h-3.5 w-3.5" />
          Send
        </Button>
      </div>
    </>
  );
}

function MessageBubble({ msg }: { msg: ChatMsg }) {
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
          <Icon icon={icons.bolt} className="h-3.5 w-3.5 text-primary" />
        </div>
        <div className="flex-1 rounded-lg bg-muted/40 px-3 py-2 text-[13px] whitespace-pre-wrap break-words">
          {msg.content}
        </div>
      </div>
    );
  }
  if (msg.role === "error") {
    return (
      <div className="flex gap-2 items-start">
        <div className="h-6 w-6 rounded-full bg-rose-500/10 ring-1 ring-rose-500/30 flex items-center justify-center shrink-0">
          <Icon
            icon={icons.error}
            className="h-3.5 w-3.5 text-rose-600 dark:text-rose-400"
          />
        </div>
        <div className="flex-1 rounded-lg bg-rose-500/5 ring-1 ring-rose-500/20 px-3 py-2 text-[12px] text-rose-700 dark:text-rose-300 font-mono whitespace-pre-wrap break-words">
          {msg.content}
        </div>
      </div>
    );
  }
  return null;
}

// ---------------------------------------------------------------------------
// Scope events tail — polls /api/admin/use-cases/events, filtered by UC
// ---------------------------------------------------------------------------

function ScopeEventsTail({ useCaseId }: { useCaseId: string | null }) {
  const [sinceIso] = useState(() => new Date().toISOString());
  const { data } = useQuery({
    queryKey: ["scope-events-tail", useCaseId, sinceIso],
    queryFn: () =>
      listScopeEvents({
        use_case_id: useCaseId ?? undefined,
        since: sinceIso,
        limit: 50,
      }),
    refetchInterval: 2_500,
    enabled: !!useCaseId,
  });
  const events: ScopeEvent[] = data?.events ?? [];

  if (!useCaseId) {
    return (
      <div className="flex-1 flex items-center justify-center p-4 text-xs text-muted-foreground text-center">
        Pick a use case to start tailing
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center p-4 gap-2 text-center">
        <Icon icon={icons.check} className="h-6 w-6 text-emerald-500" />
        <p className="text-xs text-muted-foreground">No events since you opened this</p>
        <p className="text-[10px] text-muted-foreground/70 max-w-[22ch]">
          When chat calls hit out-of-scope targets, they'll appear here in real time.
        </p>
      </div>
    );
  }

  return (
    <ul className="flex-1 overflow-y-auto px-2 py-2 space-y-1">
      {events.map((e) => (
        <li
          key={e.id}
          className={cn(
            "rounded-md border p-2 space-y-1 text-[11px]",
            e.outcome === "denied"
              ? "border-rose-500/30 bg-rose-500/5"
              : "border-amber-500/30 bg-amber-500/5",
          )}
        >
          <div className="flex items-center gap-1.5">
            <span
              className={cn(
                "inline-flex items-center h-[14px] px-1 rounded text-[9px] font-mono uppercase tracking-wider",
                e.outcome === "denied"
                  ? "bg-rose-500/20 text-rose-700 dark:text-rose-300"
                  : "bg-amber-500/20 text-amber-700 dark:text-amber-300",
              )}
            >
              {e.outcome}
            </span>
            <Icon
              icon={e.kind === "mcp_tool" ? icons.mcp : icons.llm}
              className={cn(
                "h-3 w-3 shrink-0",
                e.kind === "mcp_tool" ? "text-slate-500" : "text-violet-500",
              )}
            />
            <code className="font-mono truncate">{e.target_name}</code>
            <span className="ml-auto font-mono tabular-nums text-[10px] text-muted-foreground">
              {e.created_at &&
                formatDistanceToNow(new Date(e.created_at), { addSuffix: true })}
            </span>
          </div>
          {e.reason && (
            <p className="text-muted-foreground line-clamp-2">{e.reason}</p>
          )}
        </li>
      ))}
    </ul>
  );
}

