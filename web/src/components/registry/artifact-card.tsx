import { formatDistanceToNow } from "date-fns";
import { Icon } from "@iconify/react";
import { frameworkInfo } from "@/lib/frameworks";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { KindIcon } from "./kind-icon";
import { KindChip, StatusChip, Tag } from "@/components/common/status-chip";
import type {
  RegistryAgent,
  RegistryKind,
  RegistryPackage,
  RegistryPrompt,
  RegistryServer,
  RegistrySkill,
} from "@/types/api";

// Union of what we might render. Only the minimum fields are used.
type AnyRow = RegistryServer | RegistryAgent | RegistrySkill | RegistryPrompt;

type Extras = {
  packages?: RegistryPackage[];
  deployStatus?: string; // server-only
  tagline?: string; // one-line secondary text (framework, image, template preview)
  score?: number; // enrichment score when known on the list row
  deployCount?: number; // running/active deployments for this row
};

const KIND_LABEL: Record<RegistryKind, string> = {
  server: "MCP",
  agent: "Agent",
  skill: "Skill",
  prompt: "Prompt",
  llm: "Model",
  use_case: "Use case",
};

// score tone: ≥88 good, ≥75 muted, else warn.
function scoreTone(score: number) {
  if (score >= 88) return "text-good";
  if (score >= 75) return "text-muted-foreground";
  return "text-warn";
}

// ArtifactCard is the uniform card used across all four kinds. Visual
// hierarchy, top-to-bottom: kind icon → title stack + version → description
// → chip row → footer (score · deployed · time). Click opens the drawer.
export function ArtifactCard({
  kind,
  row,
  extras,
  onClick,
}: {
  kind: RegistryKind;
  row: AnyRow;
  extras?: Extras;
  onClick?: () => void;
}) {
  const when = row.published_at
    ? formatDistanceToNow(new Date(row.published_at), { addSuffix: true })
    : null;

  // Third-party provenance: federated rows carry a source peer; the npm/PyPI
  // importers stamp `<type>.import/` into the namespace. Everything else is
  // first-party / internal.
  const thirdParty = !!row.source_peer_id || /\.import\//i.test(row.name);

  // Agent rows carry an optional framework slug — render its vendor logo
  // as a source Tag so LangGraph / CrewAI etc. are recognizable at a glance.
  const fw =
    kind === "agent" ? frameworkInfo((row as RegistryAgent).framework) : null;

  const deployCount = extras?.deployCount ?? 0;
  const hasScore = typeof extras?.score === "number";

  return (
    <button
      onClick={onClick}
      className={cn(
        "group relative flex w-full flex-col gap-3 text-left",
        "rounded-[14px] border border-border bg-card p-4 shadow-sm",
        "transition-all hover:border-border-strong hover:shadow",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
      )}
    >
      {/* Top row: icon + title stack + version badge */}
      <div className="flex items-start gap-3 min-w-0">
        <KindIcon kind={kind} name={row.name} size="md" />
        <div className="min-w-0 flex-1">
          <h3 className="font-semibold text-[15px] leading-tight truncate">
            {row.title || row.name.split("/").slice(-1)[0]}
          </h3>
          <code
            className="mt-0.5 block truncate font-mono text-[11px] text-text-dim"
            title={row.name}
          >
            {row.name}
          </code>
        </div>
        <span className="inline-flex shrink-0 items-center rounded-md bg-surface-2 px-1.5 py-[3px] font-mono text-[10.5px] tabular-nums text-text-dim">
          v{row.version}
        </span>
      </div>

      {/* Description (always 2 lines tall for an even grid) */}
      <p className="line-clamp-2 min-h-[2.5rem] text-[13px] leading-relaxed text-muted-foreground">
        {row.description || extras?.tagline || (
          <span className="text-text-dim">No description provided.</span>
        )}
      </p>

      {/* Chip row: kind · origin · verified/latest · source */}
      <div className="flex flex-wrap items-center gap-1.5">
        <KindChip kind={kind} label={KIND_LABEL[kind]} />
        <Tag label={thirdParty ? "Third-party" : "Internal"} />
        {row.is_latest && (
          <StatusChip tone="good" dot={false} label="✓ Latest" />
        )}
        {row.status === "deprecated" && (
          <StatusChip tone="warn" label="Deprecated" />
        )}
        {fw && <Tag label={fw.label} />}
        {row.source_peer_id && <Tag label={row.source_peer_id} />}
        {extras?.deployStatus && extras.deployStatus !== "stopped" && (
          <DeployBadge status={extras.deployStatus} />
        )}
      </div>

      {/* Footer: score · deployed · relative time */}
      <div className="mt-auto flex items-center gap-3 border-t border-border/60 pt-2.5 text-[11px] text-text-dim">
        {hasScore && (
          <span className="inline-flex items-center gap-1">
            <Icon
              icon={icons.star}
              className={cn("h-3.5 w-3.5", scoreTone(extras!.score!))}
            />
            <span
              className={cn(
                "font-mono tabular-nums",
                scoreTone(extras!.score!),
              )}
            >
              {extras!.score}
            </span>
          </span>
        )}
        {deployCount > 0 && (
          <span className="inline-flex items-center gap-1">
            <Icon icon={icons.rocket} className="h-3.5 w-3.5" />
            <span className="font-mono tabular-nums">{deployCount}</span>{" "}
            deployed
          </span>
        )}
        {when && (
          <span className="ml-auto inline-flex shrink-0 items-center gap-1">
            <Icon icon={icons.clock} className="h-3 w-3" />
            {when}
          </span>
        )}
      </div>
    </button>
  );
}

function DeployBadge({ status }: { status: string }) {
  const tone =
    status === "running"
      ? "good"
      : status === "failed"
        ? "bad"
        : status === "deploying" || status === "pending"
          ? "info"
          : "warn";
  return <StatusChip tone={tone} label={status} />;
}
