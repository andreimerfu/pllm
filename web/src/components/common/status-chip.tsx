import { type ReactNode } from "react";
import { cn } from "@/lib/utils";
import type { RegistryKind } from "@/types/api";

// Status / kind chips for the "pLLM Console" look. Color is reserved for
// meaning — status tone, registry kind — never decoration.

export type StatusTone = "good" | "warn" | "bad" | "info" | "neutral";

const statusMap: Record<StatusTone, { chip: string; dot: string }> = {
  good: { chip: "bg-good/12 text-good", dot: "bg-good" },
  warn: { chip: "bg-warn/12 text-warn", dot: "bg-warn" },
  bad: { chip: "bg-bad/12 text-bad", dot: "bg-bad" },
  info: { chip: "bg-info/12 text-info", dot: "bg-info" },
  neutral: { chip: "bg-surface-2 text-text-dim", dot: "bg-text-dim" },
};

// Soft-tint pill with an optional leading status dot.
export function StatusChip({
  tone = "neutral",
  label,
  dot = true,
  className,
}: {
  tone?: StatusTone;
  label: ReactNode;
  dot?: boolean;
  className?: string;
}) {
  const t = statusMap[tone];
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-[7px] px-[9px] py-1 text-[11.5px] font-semibold",
        t.chip,
        className,
      )}
    >
      {dot && <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", t.dot)} />}
      {label}
    </span>
  );
}

const kindColorMap: Record<RegistryKind | "model", string> = {
  agent: "bg-kind-agent/12 text-kind-agent",
  server: "bg-kind-mcp/12 text-kind-mcp",
  skill: "bg-kind-skill/12 text-kind-skill",
  prompt: "bg-kind-prompt/12 text-kind-prompt",
  llm: "bg-kind-model/12 text-kind-model",
  model: "bg-kind-model/12 text-kind-model",
  use_case: "bg-[var(--accent-soft)] text-primary",
};

// Small uppercase kind tag (the catalog card label).
export function KindChip({
  kind,
  label,
  className,
}: {
  kind: RegistryKind | "model";
  label: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-[3px] text-[10.5px] font-semibold tracking-[0.02em]",
        kindColorMap[kind],
        className,
      )}
    >
      {label}
    </span>
  );
}

// Neutral tag (transport, namespace, etc.) — surface-2 fill.
export function Tag({
  label,
  className,
}: {
  label: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md bg-surface-2 px-2 py-[3px] text-[10.5px] font-medium text-text-dim",
        className,
      )}
    >
      {label}
    </span>
  );
}

// Mono-spaced eyebrow label for card/section headers.
export function SectionLabel({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "text-[10.5px] font-semibold uppercase tracking-[0.08em] text-text-dim",
        className,
      )}
    >
      {children}
    </div>
  );
}
