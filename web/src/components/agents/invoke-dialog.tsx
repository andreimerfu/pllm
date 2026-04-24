import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { invokeAgent } from "@/lib/api";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";

import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";

type Props = {
  slug: string | null;
  onClose: () => void;
};

// Runtime state — drives the status pill on the right side of the header
// and the busy styling on the output panel.
type RunState = "idle" | "running" | "success" | "error";

/**
 * InvokeDialog — right-side "ops console" drawer for ad-hoc agent invocations.
 *
 * Shape:
 *   [sticky header]   icon + slug + live status pill
 *   [input]            textarea with ⌘+Enter affordance
 *   [output]           terminal-style panel with subtle JSON syntax highlight,
 *                      copy + clear buttons, status badge
 *   [sticky footer]    Close + Invoke
 *
 * Intentional deviations from the old modal:
 *   - Sheet (side="right") uses `bg-background` so it renders correctly
 *     in dark mode without the portal-theme-bleed artifact the modal had.
 *   - Output is always-present-but-empty on first render so the layout
 *     doesn't jump when a result comes in.
 *   - ⌘/Ctrl+Enter fires an invocation from the textarea.
 */
export function InvokeDialog({ slug, onClose }: Props) {
  const { toast } = useToast();
  const [input, setInput] = useState("");
  const [output, setOutput] = useState<string | null>(null);
  const [runState, setRunState] = useState<RunState>("idle");
  const [durationMs, setDurationMs] = useState<number | null>(null);
  const startedAtRef = useRef<number | null>(null);

  const invokeMut = useMutation({
    mutationFn: async () => {
      startedAtRef.current = performance.now();
      setRunState("running");
      return invokeAgent(slug!, input);
    },
    onSuccess: (res: any) => {
      const str =
        typeof res.output === "string"
          ? res.output
          : JSON.stringify(res.output, null, 2);
      setOutput(str);
      setRunState(res.error ? "error" : "success");
      if (startedAtRef.current != null) {
        setDurationMs(Math.round(performance.now() - startedAtRef.current));
      }
    },
    onError: (e: any) => {
      setRunState("error");
      if (startedAtRef.current != null) {
        setDurationMs(Math.round(performance.now() - startedAtRef.current));
      }
      const desc = e?.response?.data?.error ?? e.message;
      setOutput(typeof desc === "string" ? desc : JSON.stringify(desc, null, 2));
      toast({
        title: "Invoke failed",
        description: typeof desc === "string" ? desc : undefined,
        variant: "destructive",
      });
    },
  });

  function handleOpenChange(open: boolean) {
    if (!open) {
      reset();
      onClose();
    }
  }

  function reset() {
    setInput("");
    setOutput(null);
    setRunState("idle");
    setDurationMs(null);
    startedAtRef.current = null;
  }

  const canRun = !!input.trim() && runState !== "running";

  // Keyboard shortcut — ⌘/Ctrl+Enter fires invocation from anywhere in the drawer.
  useEffect(() => {
    if (!slug) return;
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter" && canRun) {
        e.preventDefault();
        invokeMut.mutate();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [slug, canRun, invokeMut]);

  const outputBytes = output?.length ?? 0;

  return (
    <Sheet open={!!slug} onOpenChange={handleOpenChange}>
      <SheetContent
        side="right"
        className="w-full sm:max-w-none sm:w-[min(720px,90vw)] lg:w-[860px] p-0 flex flex-col gap-0 bg-background border-l border-border"
      >
        {/* ── Header ─────────────────────────────────────────────── */}
        <SheetHeader className="px-6 pt-5 pb-4 border-b border-border/70 space-y-0 text-left bg-gradient-to-b from-card to-background">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20 flex items-center justify-center shrink-0">
              <Icon icon={icons.play} className="h-4 w-4" />
            </div>
            <div className="min-w-0 flex-1">
              <SheetTitle className="text-[15px] font-semibold leading-tight tracking-tight">
                Invoke agent
              </SheetTitle>
              <code className="text-[11px] font-mono text-muted-foreground block truncate mt-0.5">
                {slug}
              </code>
            </div>
            <RunStatePill state={runState} durationMs={durationMs} />
          </div>
        </SheetHeader>

        {/* ── Body ───────────────────────────────────────────────── */}
        <div className="flex-1 overflow-y-auto px-6 py-5 space-y-5">
          <InputBlock
            value={input}
            onChange={setInput}
            disabled={runState === "running"}
            onSubmit={() => canRun && invokeMut.mutate()}
          />

          <OutputBlock
            output={output}
            runState={runState}
            durationMs={durationMs}
            onClear={reset}
            onCopy={() => {
              if (!output) return;
              void navigator.clipboard.writeText(output);
              toast({ title: "Copied to clipboard" });
            }}
          />
        </div>

        {/* ── Footer ─────────────────────────────────────────────── */}
        <div className="px-6 py-3.5 border-t border-border/70 bg-card/60 flex items-center gap-3">
          <KbdHint />
          <div className="ml-auto flex items-center gap-2">
            <Button variant="ghost" onClick={() => handleOpenChange(false)}>
              Close
            </Button>
            <Button
              onClick={() => invokeMut.mutate()}
              disabled={!canRun}
              className="min-w-[108px]"
            >
              {runState === "running" ? (
                <>
                  <Icon
                    icon={icons.refresh}
                    className="mr-2 h-3.5 w-3.5 animate-spin"
                  />
                  Invoking…
                </>
              ) : (
                <>
                  <Icon icon={icons.play} className="mr-2 h-3.5 w-3.5" />
                  Invoke
                </>
              )}
            </Button>
          </div>
        </div>

        {/* Hidden count for a11y screen-reader summary of output size */}
        <span className="sr-only" aria-live="polite">
          {output ? `Result received, ${outputBytes} characters.` : ""}
        </span>
      </SheetContent>
    </Sheet>
  );
}

// ───────────────────────────────────────────────────────────────────
// Input block
// ───────────────────────────────────────────────────────────────────

function InputBlock({
  value,
  onChange,
  disabled,
  onSubmit,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  onSubmit: () => void;
}) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <label className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Input
        </label>
        <span className="text-[10px] font-mono tabular-nums text-muted-foreground/70">
          {value.length} chars
        </span>
      </div>
      <Textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        rows={5}
        placeholder="Prompt or structured input for the agent"
        onKeyDown={(e) => {
          if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
            e.preventDefault();
            onSubmit();
          }
        }}
        className="font-mono text-[12.5px] leading-relaxed resize-y bg-card"
      />
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────
// Output block — terminal-style panel with subtle JSON highlighting
// ───────────────────────────────────────────────────────────────────

function OutputBlock({
  output,
  runState,
  durationMs,
  onClear,
  onCopy,
}: {
  output: string | null;
  runState: RunState;
  durationMs: number | null;
  onClear: () => void;
  onCopy: () => void;
}) {
  const highlighted = useMemo(() => (output ? highlightJSON(output) : null), [output]);
  const isErr = runState === "error";

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <label className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
          Output
        </label>
        {output !== null && (
          <div className="flex items-center gap-3">
            {durationMs != null && (
              <span className="inline-flex items-center gap-1 text-[10px] font-mono tabular-nums text-muted-foreground/80">
                <Icon icon={icons.clock} className="h-2.5 w-2.5" />
                {formatMs(durationMs)}
              </span>
            )}
            <span className="text-[10px] font-mono tabular-nums text-muted-foreground/70">
              {output.length} bytes
            </span>
            <button
              type="button"
              onClick={onCopy}
              className="inline-flex items-center gap-1 text-[10px] font-mono uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors"
              title="Copy to clipboard"
            >
              <Icon icon={icons.copy} className="h-3 w-3" />
              Copy
            </button>
            <button
              type="button"
              onClick={onClear}
              className="inline-flex items-center gap-1 text-[10px] font-mono uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors"
              title="Clear"
            >
              <Icon icon={icons.delete} className="h-3 w-3" />
              Clear
            </button>
          </div>
        )}
      </div>

      <div
        className={cn(
          "relative rounded-md border overflow-hidden",
          isErr
            ? "border-rose-500/30 bg-rose-500/[0.03]"
            : "border-border/70 bg-[hsl(var(--muted))]/40",
        )}
      >
        {/* Terminal-style gutter bar — thin vertical accent on the left */}
        <div
          className={cn(
            "absolute inset-y-0 left-0 w-[2px]",
            runState === "success" && "bg-emerald-500/60",
            runState === "error" && "bg-rose-500/70",
            runState === "running" &&
              "bg-sky-500/70 animate-pulse",
            runState === "idle" && "bg-border",
          )}
        />
        {output === null ? (
          <div className="px-5 py-8 text-center">
            <div className="inline-flex items-center gap-2 text-[11px] font-mono uppercase tracking-widest text-muted-foreground/70">
              <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/40" />
              awaiting invocation
            </div>
          </div>
        ) : (
          <pre
            className={cn(
              "max-h-[min(56vh,480px)] overflow-auto pl-5 pr-3 py-3 text-[12px] leading-[1.55] font-mono tabular-nums whitespace-pre-wrap break-words",
              isErr && "text-rose-600 dark:text-rose-300",
            )}
          >
            {highlighted}
          </pre>
        )}
      </div>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────
// Run-state pill + keyboard hint
// ───────────────────────────────────────────────────────────────────

function RunStatePill({
  state,
  durationMs,
}: {
  state: RunState;
  durationMs: number | null;
}) {
  const cls: Record<RunState, string> = {
    idle: "bg-muted text-muted-foreground ring-border",
    running: "bg-sky-500/10 text-sky-700 dark:text-sky-400 ring-sky-500/30",
    success:
      "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 ring-emerald-500/30",
    error: "bg-rose-500/10 text-rose-700 dark:text-rose-400 ring-rose-500/30",
  };
  const label: Record<RunState, string> = {
    idle: "ready",
    running: "running",
    success: "ok",
    error: "error",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 h-6 px-2 rounded ring-1 text-[10px] font-mono uppercase tracking-widest shrink-0",
        cls[state],
      )}
    >
      {state === "running" && (
        <span className="relative flex h-1.5 w-1.5">
          <span className="absolute inset-0 rounded-full bg-sky-500/60 animate-ping" />
          <span className="relative rounded-full h-1.5 w-1.5 bg-sky-500" />
        </span>
      )}
      {state === "success" && <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />}
      {state === "error" && <span className="h-1.5 w-1.5 rounded-full bg-rose-500" />}
      {label[state]}
      {state !== "idle" && state !== "running" && durationMs != null && (
        <span className="text-muted-foreground/70 ml-0.5">· {formatMs(durationMs)}</span>
      )}
    </span>
  );
}

function KbdHint() {
  const isMac = useMemo(
    () => typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform),
    [],
  );
  return (
    <div className="inline-flex items-center gap-1.5 text-[10.5px] font-mono text-muted-foreground/80">
      <Kbd>{isMac ? "⌘" : "Ctrl"}</Kbd>
      <Kbd>↵</Kbd>
      <span className="text-muted-foreground/60">to invoke</span>
    </div>
  );
}

function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="inline-flex items-center justify-center min-w-[1.4rem] h-5 px-1 rounded bg-muted/60 ring-1 ring-border text-[10px] font-mono text-foreground/80">
      {children}
    </kbd>
  );
}

// ───────────────────────────────────────────────────────────────────
// Lightweight JSON syntax highlighter — no external dep.
// Falls back to plain text if input isn't parseable JSON.
// ───────────────────────────────────────────────────────────────────

function highlightJSON(raw: string): React.ReactNode {
  // Validate — if not JSON, render as plain text so non-JSON error strings
  // still display cleanly.
  try {
    JSON.parse(raw);
  } catch {
    return raw;
  }

  // Token regex: matches a JSON key, a quoted string value, a boolean/null,
  // or a number. Anything unmatched is emitted as-is (braces, commas, WS).
  const parts: React.ReactNode[] = [];
  let last = 0;
  const re =
    /"(?:\\.|[^"\\])*"(\s*:)?|\b(true|false|null)\b|-?\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/g;
  let m: RegExpExecArray | null;
  let i = 0;
  while ((m = re.exec(raw)) != null) {
    if (m.index > last) {
      parts.push(<span key={`p${i++}`}>{raw.slice(last, m.index)}</span>);
    }
    const tok = m[0];
    const isKey = !!m[1];
    if (isKey) {
      const key = tok.slice(0, -m[1].length);
      parts.push(
        <span key={`p${i++}`} className="text-sky-600 dark:text-sky-300">
          {key}
        </span>,
      );
      parts.push(<span key={`p${i++}`}>{m[1]}</span>);
    } else if (tok.startsWith('"')) {
      parts.push(
        <span key={`p${i++}`} className="text-emerald-700 dark:text-emerald-300/95">
          {tok}
        </span>,
      );
    } else if (tok === "true" || tok === "false") {
      parts.push(
        <span key={`p${i++}`} className="text-amber-600 dark:text-amber-300">
          {tok}
        </span>,
      );
    } else if (tok === "null") {
      parts.push(
        <span key={`p${i++}`} className="text-muted-foreground italic">
          {tok}
        </span>,
      );
    } else {
      parts.push(
        <span key={`p${i++}`} className="text-violet-600 dark:text-violet-300">
          {tok}
        </span>,
      );
    }
    last = m.index + tok.length;
  }
  if (last < raw.length) parts.push(<span key={`p${i++}`}>{raw.slice(last)}</span>);
  return parts;
}

function formatMs(ms: number) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(ms < 10_000 ? 2 : 1)}s`;
}
