import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { getBrandLogo, isColoredLogo } from "@/lib/brand-logos";
import type { RegistryKind } from "@/types/api";

// Kind-tinted icon square. Gives each of the four registry kinds an
// immediately-recognizable identity on cards and in drawer headers.
// We reserve color usage for meaning — score, status, and kind — so the
// palette stays controlled.
//
// When a `name` prop is supplied and matches a well-known service (see
// lib/brand-logos.ts), we swap the generic kind glyph for the vendor mark.
// For colored `logos:*` icons we drop the tint fill and keep the kind
// color as a ring, so brand colors read cleanly; for monochrome
// `simple-icons:*` icons we keep the tint fill (they inherit currentColor).
// Kind tints follow the "pLLM Console" palette: agent indigo, MCP blue,
// skill purple, prompt amber, model/llm pink. Driven by the shared
// --kind-* tokens so the same colors back chips, KPI tiles, and charts.
const TINTS: Record<RegistryKind, { bg: string; fg: string; ring: string; icon: string }> = {
  server: {
    bg: "bg-kind-mcp/12",
    fg: "text-kind-mcp",
    ring: "ring-kind-mcp/20",
    icon: icons.mcp,
  },
  agent: {
    bg: "bg-kind-agent/12",
    fg: "text-kind-agent",
    ring: "ring-kind-agent/20",
    icon: icons.agent,
  },
  skill: {
    bg: "bg-kind-skill/12",
    fg: "text-kind-skill",
    ring: "ring-kind-skill/20",
    icon: icons.skill,
  },
  prompt: {
    bg: "bg-kind-prompt/12",
    fg: "text-kind-prompt",
    ring: "ring-kind-prompt/20",
    icon: icons.prompt,
  },
  // LLMs live in the existing model-registry pipeline — they're surfaced
  // here as graph nodes via dependency edges, so they deserve a tint too.
  llm: {
    bg: "bg-kind-model/12",
    fg: "text-kind-model",
    ring: "ring-kind-model/20",
    icon: icons.llm,
  },
  // UseCase is a top-level business-activity node; the primary tint
  // signals "this is where approvals live."
  use_case: {
    bg: "bg-[var(--accent-soft)]",
    fg: "text-primary",
    ring: "ring-primary/20",
    icon: icons.useCase,
  },
};

export function KindIcon({
  kind,
  name,
  size = "md",
  className,
}: {
  kind: RegistryKind;
  name?: string;
  size?: "sm" | "md" | "lg";
  className?: string;
}) {
  const t = TINTS[kind];
  const brand = name ? getBrandLogo(name) : null;
  const colored = brand ? isColoredLogo(brand) : false;
  const glyph = brand ?? t.icon;
  const dim = { sm: "h-8 w-8", md: "h-10 w-10", lg: "h-14 w-14" }[size];
  const glyphDim = { sm: "h-4 w-4", md: "h-5 w-5", lg: "h-7 w-7" }[size];
  return (
    <div
      className={cn(
        "rounded-lg flex items-center justify-center shrink-0 ring-1",
        colored ? "bg-background" : t.bg,
        colored ? "" : t.fg,
        t.ring,
        dim,
        className,
      )}
    >
      <Icon icon={glyph} className={glyphDim} />
    </div>
  );
}

export function kindAccent(kind: RegistryKind) {
  return TINTS[kind];
}
