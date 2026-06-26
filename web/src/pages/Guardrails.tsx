import { useNavigate } from "react-router-dom";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

import {
  useGuardrails,
  formatLatency,
  GuardrailInfo,
} from "@/hooks/useGuardrails";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { PageShell, StatTile } from "@/components/common/page-shell";
import { getBrandLogo, isColoredLogo } from "@/lib/brand-logos";
import {
  StatusChip,
  Tag,
  SectionLabel,
  type StatusTone,
} from "@/components/common/status-chip";

// ── Derivations ─────────────────────────────────────────────────────────────
// There is no explicit type/action field on a guardrail — derive a display
// "type" from the provider, and read action/scope from config only when present.

type KindColor = "mcp" | "agent" | "skill" | "prompt" | "model";

interface DerivedType {
  label: string;
  kind: KindColor;
}

// Map a provider to a human-facing safety category + kind tint.
const deriveType = (provider: string): DerivedType => {
  const p = provider.toLowerCase();
  if (p.includes("presidio")) return { label: "PII", kind: "mcp" };
  if (p.includes("lakera")) return { label: "Injection", kind: "agent" };
  if (p.includes("openai") || p.includes("azure"))
    return { label: "Content", kind: "skill" };
  if (p.includes("toxic")) return { label: "Toxicity", kind: "prompt" };
  if (p.includes("secret") || p.includes("custom"))
    return { label: "Secrets", kind: "model" };
  return { label: "Custom", kind: "model" };
};

// Built-in vs external. Hosted/SaaS providers are external integrations.
const isExternal = (provider: string): boolean => {
  const p = provider.toLowerCase();
  return ["presidio", "openai", "lakera", "azure"].some((k) => p.includes(k));
};

// Human label for a provider.
const providerLabel = (provider: string): string =>
  provider === "presidio" ? "Microsoft Presidio" : provider;

// Token-based tone for an execution mode. Color reserved for meaning.
const modeTone = (mode: string): StatusTone => {
  switch (mode) {
    case "pre_call":
      return "info";
    case "post_call":
      return "good";
    case "during_call":
      return "warn";
    default:
      return "neutral";
  }
};

// Static dot color per tone — keeps Tailwind's JIT scanner happy (no runtime
// class concatenation).
const toneDotClass: Record<StatusTone, string> = {
  good: "bg-good",
  warn: "bg-warn",
  bad: "bg-bad",
  info: "bg-info",
  neutral: "bg-text-dim",
};

const modeLabel = (mode: string): string => {
  switch (mode) {
    case "pre_call":
      return "Pre-call";
    case "post_call":
      return "Post-call";
    case "during_call":
      return "During-call";
    default:
      return mode.replace(/_/g, " ");
  }
};

// Soft tile tint per derived kind.
const kindTileClass: Record<KindColor, string> = {
  mcp: "bg-kind-mcp/12 text-kind-mcp",
  agent: "bg-kind-agent/12 text-kind-agent",
  skill: "bg-kind-skill/12 text-kind-skill",
  prompt: "bg-kind-prompt/12 text-kind-prompt",
  model: "bg-kind-model/12 text-kind-model",
};

const kindChipClass: Record<KindColor, string> = {
  mcp: "bg-kind-mcp/12 text-kind-mcp",
  agent: "bg-kind-agent/12 text-kind-agent",
  skill: "bg-kind-skill/12 text-kind-skill",
  prompt: "bg-kind-prompt/12 text-kind-prompt",
  model: "bg-kind-model/12 text-kind-model",
};

// Colored type chip — same shape as Tag but kind-tinted.
const TypeChip = ({ type }: { type: DerivedType }) => (
  <span
    className={`inline-flex items-center rounded-md px-2 py-[3px] text-[10.5px] font-semibold tracking-[0.02em] ${kindChipClass[type.kind]}`}
  >
    {type.label}
  </span>
);

// The three enforcement stages, in pipeline order.
const STAGES: { mode: string; label: string }[] = [
  { mode: "pre_call", label: "Pre-call" },
  { mode: "during_call", label: "During-call" },
  { mode: "post_call", label: "Post-call" },
];

export default function Guardrails() {
  const navigate = useNavigate();

  // Data management hook — untouched.
  const {
    guardrails,
    systemEnabled,
    isLoading,
    refetch,
  } = useGuardrails();

  // Toggle a guardrail's enabled state. No backend mutation endpoint exists
  // yet — keep the existing stub data-flow (log + refetch) so the Switch is
  // wired and ready for when the mutation lands.
  const handleToggle = (guardrail: GuardrailInfo, next: boolean) => {
    console.log("Toggle guardrail:", guardrail.name, "→", next);
    refetch();
  };

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
      </div>
    );
  }

  // ── KPI values, computed only from data already present on the rows ──
  const totalGuardrails = guardrails.length;
  const enabledCount = guardrails.filter((g) => g.enabled).length;
  const pausedCount = totalGuardrails - enabledCount;

  const rowsWithStats = guardrails.filter((g) => g.stats);
  const totalScreened = rowsWithStats.reduce(
    (sum, g) => sum + (g.stats?.total_executions ?? 0),
    0,
  );
  const totalBlocked = rowsWithStats.reduce(
    (sum, g) => sum + (g.stats?.total_blocked ?? 0),
    0,
  );
  const latencyRows = rowsWithStats.filter(
    (g) => typeof g.stats?.average_latency === "number",
  );
  const avgLatency =
    latencyRows.length > 0
      ? latencyRows.reduce((sum, g) => sum + (g.stats?.average_latency ?? 0), 0) /
        latencyRows.length
      : null;

  // ── Stage grouping for the enforcement pipeline ──
  const stageGroups = STAGES.map((stage) => ({
    ...stage,
    items: guardrails.filter((g) => g.mode?.includes(stage.mode)),
  })).filter((s) => s.items.length > 0);

  return (
    <PageShell
      title="Guardrails"
      description="Safety policies applied on every request and response — PII redaction, content moderation, prompt-injection screens. Use predefined providers or connect your own."
      actions={
        <>
          <Button
            variant="outline"
            onClick={() => navigate("/guardrails/marketplace")}
          >
            <Icon icon={icons.registry} className="mr-2 h-4 w-4" />
            Browse marketplace
          </Button>
          <Button onClick={() => navigate("/guardrails/config/new")}>
            <Icon icon={icons.plus} className="mr-2 h-4 w-4" />
            Add guardrail
          </Button>
        </>
      }
    >
      {/* System disabled alert */}
      {!systemEnabled && (
        <div className="rounded-[14px] border border-warn/30 bg-warn/12 p-4 shadow-sm">
          <div className="flex items-center gap-2">
            <Icon icon={icons.warning} className="h-4 w-4 text-warn" />
            <p className="text-sm text-warn">
              Guardrails system is currently disabled. Enable it in the
              configuration to start protecting your LLM interactions.
            </p>
          </div>
        </div>
      )}

      {/* ────────────── KPI row ────────────── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        <StatTile
          label="Active guardrails"
          value={enabledCount}
          tone={enabledCount > 0 ? "good" : "default"}
          icon={<Icon icon={icons.shield} />}
          iconColor="good"
          sub={`${enabledCount} enabled · ${pausedCount} paused`}
        />
        {totalScreened > 0 && (
          <StatTile
            label="Requests screened"
            value={totalScreened.toLocaleString()}
            icon={<Icon icon={icons.activity} />}
            iconColor="info"
            sub="Last 24h"
          />
        )}
        {totalBlocked > 0 && (
          <StatTile
            label="Violations handled"
            value={totalBlocked.toLocaleString()}
            tone="warn"
            icon={<Icon icon={icons.shieldAlert} />}
            iconColor="warn"
            sub="Across all guardrails"
          />
        )}
        {avgLatency !== null && (
          <StatTile
            label="Avg added latency"
            value={formatLatency(Math.round(avgLatency))}
            tone="warn"
            icon={<Icon icon={icons.clock} />}
            iconColor="warn"
            sub={`p95 over ${latencyRows.length} active`}
          />
        )}
      </div>

      {/* ────────────── Enforcement pipeline ────────────── */}
      <div className="rounded-[14px] border border-border bg-card p-5 shadow-sm">
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            <h2 className="text-base font-semibold">Enforcement pipeline</h2>
            <p className="text-[13px] text-text-dim">
              How every request flows through your active guardrails
            </p>
          </div>
          <StatusChip tone="good" label={`${enabledCount} enabled`} />
        </div>

        <div className="flex items-stretch gap-3 overflow-x-auto">
          {/* Request start tile */}
          <div className="flex min-w-[88px] flex-col items-center justify-center gap-1.5 rounded-[10px] bg-surface-2 px-4 py-3">
            <Icon icon={icons.arrowRight} className="h-4 w-4 text-text-dim" />
            <span className="text-[12px] font-medium text-muted-foreground">
              Request
            </span>
          </div>

          {stageGroups.map((stage) => (
            <div key={stage.mode} className="flex items-stretch gap-3">
              <div className="flex items-center">
                <Icon
                  icon={icons.chevronRight}
                  className="h-4 w-4 text-text-dim"
                />
              </div>
              <div className="min-w-[200px] rounded-[10px] border border-border bg-surface-3 p-3">
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-[12px] font-semibold">{stage.label}</span>
                  <span className="rounded-full bg-surface-2 px-2 py-0.5 font-mono text-[10.5px] text-text-dim">
                    {stage.items.length}
                  </span>
                </div>
                <div className="space-y-1.5">
                  {stage.items.map((g) => {
                    const action =
                      typeof g.config?.action === "string"
                        ? (g.config.action as string)
                        : null;
                    return (
                      <div
                        key={g.name}
                        className={`flex items-center justify-between gap-2 ${g.enabled ? "" : "opacity-50"}`}
                      >
                        <div className="flex min-w-0 items-center gap-1.5">
                          <span
                            className={`h-1.5 w-1.5 shrink-0 rounded-full ${toneDotClass[modeTone(stage.mode)]}`}
                          />
                          <span className="truncate text-[12px]">{g.name}</span>
                        </div>
                        {action ? (
                          <Tag label={action} />
                        ) : (
                          <StatusChip
                            tone={modeTone(stage.mode)}
                            dot={false}
                            label={stage.label}
                          />
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          ))}

          <div className="flex items-center">
            <Icon icon={icons.chevronRight} className="h-4 w-4 text-text-dim" />
          </div>

          {/* Response end tile */}
          <div className="flex min-w-[88px] flex-col items-center justify-center gap-1.5 rounded-[10px] bg-[var(--accent-soft)] px-4 py-3">
            <Icon icon={icons.check} className="h-4 w-4 text-primary" />
            <span className="text-[12px] font-medium text-primary">
              Response
            </span>
          </div>
        </div>
      </div>

      {/* ────────────── Active guardrails ────────────── */}
      <div className="space-y-3">
        <SectionLabel>Active guardrails</SectionLabel>

        {guardrails.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-[14px] border border-border bg-card py-12 shadow-sm">
            <span className="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-[12px] bg-kind-skill/12 text-kind-skill">
              <Icon icon={icons.shield} className="h-6 w-6" />
            </span>
            <h3 className="mb-2 text-lg font-semibold">No guardrails configured</h3>
            <p className="max-w-md text-center text-muted-foreground">
              No guardrails are currently configured. Add guardrails to your
              configuration to start protecting your LLM interactions.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {guardrails.map((g) => {
              const type = deriveType(g.provider);
              const action =
                typeof g.config?.action === "string"
                  ? (g.config.action as string)
                  : null;
              const scope =
                typeof g.config?.scope === "string"
                  ? (g.config.scope as string)
                  : "All routes";
              return (
                <div
                  key={g.name}
                  className={`rounded-[14px] border border-border bg-card p-4 shadow-sm ${g.enabled ? "" : "opacity-70"}`}
                >
                  {/* Top row */}
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex min-w-0 items-start gap-3">
                      {(() => {
                        // Prefer the real provider logo (Microsoft, OpenAI…);
                        // fall back to a type-tinted shield.
                        const brand = getBrandLogo(providerLabel(g.provider)) || getBrandLogo(g.provider);
                        const colored = brand ? isColoredLogo(brand) : false;
                        return (
                          <span
                            className={`inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] ${colored ? "bg-surface-2" : kindTileClass[type.kind]}`}
                          >
                            <Icon icon={brand ?? icons.shield} className="h-5 w-5" />
                          </span>
                        );
                      })()}
                      <div className="min-w-0">
                        <div className="truncate font-semibold">{g.name}</div>
                        <div className="truncate text-[13px] text-text-dim">
                          {providerLabel(g.provider)}
                        </div>
                      </div>
                    </div>
                    <Switch
                      checked={g.enabled}
                      onCheckedChange={(next) => handleToggle(g, next)}
                    />
                  </div>

                  {/* Chip row */}
                  <div className="mt-3 flex flex-wrap items-center gap-1.5">
                    <TypeChip type={type} />
                    <Tag label={isExternal(g.provider) ? "External" : "Built-in"} />
                    {g.mode?.map((m) => (
                      <Tag key={m} label={modeLabel(m)} />
                    ))}
                    {action && <Tag label={action} />}
                  </div>

                  {/* Footer */}
                  <div className="mt-3 flex items-center justify-between border-t border-border pt-3 text-[11px] text-text-dim">
                    <span>
                      {scope}
                      {g.stats &&
                        ` · ${g.stats.total_blocked.toLocaleString()} blocked`}
                    </span>
                    {g.stats && (
                      <span className="font-mono">
                        +{formatLatency(g.stats.average_latency)}
                      </span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* ────────────── Discover more guardrails banner ────────────── */}
      <div className="flex items-center justify-between gap-4 rounded-[14px] border border-primary/30 bg-[var(--accent-soft)] p-5">
        <div className="flex min-w-0 items-start gap-3">
          <span className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-primary/12 text-primary">
            <Icon icon={icons.registry} className="h-5 w-5" />
          </span>
          <div className="min-w-0">
            <div className="font-semibold">Discover more guardrails</div>
            <p className="text-[13px] text-text-dim">
              Browse 14 integrations from Microsoft, Lakera, OpenAI, NVIDIA and the
              community — or connect your own endpoint.
            </p>
          </div>
        </div>
        <Button
          className="shrink-0"
          onClick={() => navigate("/guardrails/marketplace")}
        >
          Open marketplace
          <Icon icon={icons.arrowRight} className="ml-2 h-4 w-4" />
        </Button>
      </div>
    </PageShell>
  );
}
