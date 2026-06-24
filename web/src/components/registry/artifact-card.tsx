import { formatDistanceToNow } from "date-fns";
import { Icon } from "@iconify/react";
import { frameworkInfo } from "@/lib/frameworks";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import { KindIcon } from "./kind-icon";
import { PackageChip } from "./package-chip";
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
};

// ArtifactCard is the uniform card used across all four kinds. Visual
// hierarchy, left-to-right: kind icon → title stack → metadata tail. Click
// opens the detail drawer. Tiny strip across the bottom carries status +
// install command when relevant.
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

  // Collect unique package types (so a single card with npm + oci shows both).
  const pkgTypes = Array.from(
    new Set((extras?.packages ?? []).map((p) => p.registry_type?.toLowerCase()).filter(Boolean)),
  ) as string[];

  // Agent rows carry an optional framework slug — render its vendor logo
  // alongside the title so LangGraph / CrewAI etc. are recognizable at a glance.
  const fw =
    kind === "agent"
      ? frameworkInfo((row as RegistryAgent).framework)
      : null;

  return (
    <button
      onClick={onClick}
      className={cn(
        "group relative text-left w-full",
        "rounded-xl border border-border/80 bg-card",
        "hover:border-primary/40 hover:shadow-sm transition-all",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary",
        "p-4 space-y-3",
      )}
    >
      {/* Top row: icon + title + version */}
      <div className="flex items-start gap-3 min-w-0">
        <KindIcon kind={kind} name={row.name} size="md" />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 min-w-0">
            <h3 className="font-semibold text-[15px] leading-tight truncate">
              {row.title || row.name.split("/").slice(-1)[0]}
            </h3>
            {fw && (
              <span
                className="inline-flex items-center gap-1 rounded-full border px-1.5 py-0 h-[18px] text-[10px] font-medium text-muted-foreground shrink-0"
                title={`Framework: ${fw.label}`}
              >
                <Icon
                  icon={fw.icon}
                  className={`h-3 w-3 ${fw.tone ?? ""}`}
                />
                {fw.label}
              </span>
            )}
          </div>
          <code
            className="text-[11px] font-mono text-muted-foreground block truncate mt-0.5"
            title={row.name}
          >
            {row.name}
          </code>
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          <span className="inline-flex items-center h-[18px] px-1.5 rounded bg-foreground/5 ring-1 ring-border/60 font-mono text-[10px] tabular-nums">
            v{row.version}
          </span>
          {row.is_latest && (
            <span className="text-[9px] font-mono uppercase tracking-widest text-emerald-600 dark:text-emerald-400">
              latest
            </span>
          )}
        </div>
      </div>

      {/* Description */}
      {row.description && (
        <p className="text-[13px] leading-relaxed text-muted-foreground line-clamp-2">
          {row.description}
        </p>
      )}

      {/* Tagline — e.g. framework/language for agents, image ref for skills */}
      {extras?.tagline && (
        <p className="text-[11px] font-mono text-muted-foreground/80 truncate">
          {extras.tagline}
        </p>
      )}

      {/* Metadata rail */}
      <div className="flex items-center justify-between gap-2 text-[11px] text-muted-foreground pt-1">
        <div className="flex items-center gap-1.5 min-w-0 flex-wrap">
          {pkgTypes.map((t) => (
            <PackageChip key={t} type={t} />
          ))}
          {row.status === "deprecated" && (
            <span className="inline-flex items-center h-[18px] px-1.5 rounded bg-amber-500/10 ring-1 ring-amber-500/20 font-mono text-[10px] uppercase tracking-wider text-amber-700 dark:text-amber-400">
              deprecated
            </span>
          )}
          {row.source_peer_id && (
            <span
              className="inline-flex items-center gap-1 h-[18px] px-1.5 rounded bg-violet-500/10 ring-1 ring-violet-500/20 font-mono text-[10px] uppercase tracking-wider text-violet-700 dark:text-violet-300"
              title={`Federated from ${row.source_peer_id}`}
            >
              <Icon icon={icons.federation} className="h-3 w-3" />
              {row.source_peer_id}
            </span>
          )}
          {extras?.deployStatus && extras.deployStatus !== "stopped" && (
            <DeployBadge status={extras.deployStatus} />
          )}
        </div>
        {when && (
          <span className="inline-flex items-center gap-1 shrink-0">
            <Icon icon={icons.clock} className="h-3 w-3" />
            {when}
          </span>
        )}
      </div>
    </button>
  );
}

function DeployBadge({ status }: { status: string }) {
  const palette: Record<string, string> = {
    running: "text-emerald-700 dark:text-emerald-300 bg-emerald-500/10 ring-emerald-500/30",
    deploying: "text-blue-700 dark:text-blue-300 bg-blue-500/10 ring-blue-500/30",
    pending: "text-muted-foreground bg-muted ring-border",
    failed: "text-rose-700 dark:text-rose-300 bg-rose-500/10 ring-rose-500/30",
    terminating: "text-amber-700 dark:text-amber-300 bg-amber-500/10 ring-amber-500/30",
  };
  const tone = palette[status] ?? palette.pending;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 h-[18px] px-1.5 rounded font-mono text-[10px] uppercase tracking-wider ring-1",
        tone,
      )}
    >
      <span className="h-1.5 w-1.5 rounded-full bg-current animate-pulse" />
      {status}
    </span>
  );
}
