import { useMemo, useState } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { Icon } from "@iconify/react";
import { formatDistanceToNow } from "date-fns";

import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  listRegistryServers,
  listRegistryAgents,
  listRegistrySkills,
  listRegistryPrompts,
  getRegistryAgent,
  getRegistryServer,
  getRegistrySkill,
  getRegistryPrompt,
  deleteRegistryServerVersion,
  deleteRegistryAgentVersion,
  deleteRegistrySkillVersion,
  deleteRegistryPromptVersion,
  triggerRegistryImport,
  deployRegistryServer,
  listDeployments,
  listRegistryServerVersions,
  listRegistryAgentVersions,
  listRegistrySkillVersions,
  listRegistryPromptVersions,
  listRegistryDependents,
  getObservedAgentDeps,
} from "@/lib/api";
import { detectProvider } from "@/lib/providers";
import type {
  Deployment,
  DeploymentStatus,
  EnrichmentScore,
  RegistryAgent,
  RegistryAgentWithRefs,
  RegistryKind,
  RegistryPackage,
  RegistryPrompt,
  RegistryServer,
  RegistryServerDetail,
  RegistrySkill,
  ObservedAgentCall,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EmptyState } from "@/components/common/EmptyState";
import { PublishDialog } from "@/components/registry/publish-dialog";
import { ArtifactCard } from "@/components/registry/artifact-card";
import { KindIcon, kindAccent } from "@/components/registry/kind-icon";
import { PackageChip } from "@/components/registry/package-chip";
import { InstallCommand } from "@/components/registry/install-command";
import { ScoresPanel } from "@/components/registry/score-ribbon";
import { VersionPicker } from "@/components/registry/version-picker";
import {
  KindBreakdown,
  PublishActivity,
  ThirdPartyMix,
} from "@/components/registry/registry-overview";
import { PageShell } from "@/components/common/page-shell";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Types and helpers
// ---------------------------------------------------------------------------

type Kind = "servers" | "agents" | "skills" | "prompts";
type SortMode = "recent" | "alpha" | "score";

const KIND_META: Record<
  Kind,
  { singular: RegistryKind; label: string; tagline: string; empty: string }
> = {
  servers: {
    singular: "server",
    label: "MCP Servers",
    tagline: "Cataloged MCP servers — npm, PyPI, OCI, or remote endpoints.",
    empty: "No MCP servers published yet.",
  },
  agents: {
    singular: "agent",
    label: "Agents",
    tagline: "Agent manifests bundling servers, skills, and prompts.",
    empty: "No agents published yet.",
  },
  skills: {
    singular: "skill",
    label: "Skills",
    tagline: "Knowledge bundles pulled from OCI images.",
    empty: "No skills published yet.",
  },
  prompts: {
    singular: "prompt",
    label: "Prompts",
    tagline: "Reusable prompt templates with argument schemas.",
    empty: "No prompts published yet.",
  },
};

function parsePackages(raw: unknown): RegistryPackage[] {
  if (!raw) return [];
  if (Array.isArray(raw)) return raw as RegistryPackage[];
  return [];
}

function deploymentFor(
  list: Deployment[] | undefined,
  name: string,
  version?: string,
) {
  if (!list) return undefined;
  return list.find(
    (d) => d.resource_name === name && (!version || d.resource_version === version),
  );
}

const deployInProgress = (s?: DeploymentStatus) =>
  s === "pending" || s === "deploying" || s === "terminating";

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function Registry() {
  const [kind, setKind] = useState<Kind>("servers");
  const [search, setSearch] = useState("");
  const [latestOnly, setLatestOnly] = useState(true);
  const [sort, setSort] = useState<SortMode>("recent");
  const [pkgFilter, setPkgFilter] = useState<string | null>(null);
  const [detail, setDetail] = useState<
    { kind: Kind; name: string; version?: string } | null
  >(null);
  const [publishOpen, setPublishOpen] = useState(false);

  const query = { search: search || undefined, latest: latestOnly };

  // Four parallel kind queries; only the active tab actually fires, but we
  // keep the others primed for the KPI strip.
  const servers = useQuery({
    queryKey: ["registry-servers", query],
    queryFn: () => listRegistryServers(query),
  });
  const agents = useQuery({
    queryKey: ["registry-agents", query],
    queryFn: () => listRegistryAgents(query),
  });
  const skills = useQuery({
    queryKey: ["registry-skills", query],
    queryFn: () => listRegistrySkills(query),
  });
  const prompts = useQuery({
    queryKey: ["registry-prompts", query],
    queryFn: () => listRegistryPrompts(query),
  });

  // Deployment state is polled broadly so every card can show live deploy info.
  const deployments = useQuery({
    queryKey: ["deployments"],
    queryFn: listDeployments,
    refetchInterval: 5_000,
  });

  const activeItems = useMemo(() => {
    switch (kind) {
      case "servers":
        return servers.data?.servers ?? [];
      case "agents":
        return agents.data?.agents ?? [];
      case "skills":
        return skills.data?.skills ?? [];
      case "prompts":
        return prompts.data?.prompts ?? [];
    }
  }, [kind, servers.data, agents.data, skills.data, prompts.data]);

  const filteredItems = useMemo(() => {
    let items = [...activeItems];
    // Package-type filter only makes sense for servers.
    if (kind === "servers" && pkgFilter) {
      items = items.filter((row) => {
        const pkgs = parsePackages((row as RegistryServer).packages);
        return pkgs.some(
          (p) => (p.registry_type ?? "").toLowerCase() === pkgFilter,
        );
      });
    }
    // Sort client-side; backend default is name-asc which is fine as a tiebreaker.
    switch (sort) {
      case "alpha":
        items.sort((a, b) => a.name.localeCompare(b.name));
        break;
      case "recent":
        items.sort((a, b) => {
          const ad = a.published_at ? new Date(a.published_at).getTime() : 0;
          const bd = b.published_at ? new Date(b.published_at).getTime() : 0;
          return bd - ad;
        });
        break;
      case "score":
        // Score isn't on the list row. Keep alpha tiebreak; real score sort
        // would need a backend join — a future improvement.
        items.sort((a, b) => a.name.localeCompare(b.name));
        break;
    }
    return items;
  }, [activeItems, kind, pkgFilter, sort]);

  const counts = {
    servers: servers.data?.total ?? servers.data?.servers?.length ?? 0,
    agents: agents.data?.total ?? agents.data?.agents?.length ?? 0,
    skills: skills.data?.total ?? skills.data?.skills?.length ?? 0,
    prompts: prompts.data?.total ?? prompts.data?.prompts?.length ?? 0,
  };
  const deployCount = deployments.data?.deployments?.length ?? 0;
  const runningDeployments = (deployments.data?.deployments ?? []).filter(
    (d) => d.status === "running",
  ).length;

  // Package types present among current servers (for filter chips).
  const availablePkgTypes = useMemo(() => {
    if (kind !== "servers") return [];
    const set = new Set<string>();
    for (const row of activeItems) {
      for (const p of parsePackages((row as RegistryServer).packages)) {
        if (p.registry_type) set.add(p.registry_type.toLowerCase());
      }
    }
    return Array.from(set).sort();
  }, [activeItems, kind]);

  const activeQueryLoading = {
    servers: servers.isLoading,
    agents: agents.isLoading,
    skills: skills.isLoading,
    prompts: prompts.isLoading,
  }[kind];

  const overviewInput = {
    servers: servers.data?.servers ?? [],
    agents: agents.data?.agents ?? [],
    skills: skills.data?.skills ?? [],
    prompts: prompts.data?.prompts ?? [],
  };

  return (
    <PageShell
      title="Registry"
      description="Published MCP servers, agents, skills and prompts. Reviewed, versioned, scored, and deployable as Kubernetes workloads."
      actions={
        <>
          {kind === "servers" && <ImportButton />}
          <Button onClick={() => setPublishOpen(true)} className="gap-2">
            <Icon icon={icons.plus} className="h-4 w-4" />
            Publish {kind.slice(0, -1)}
          </Button>
        </>
      }
    >
      <PublishDialog
        kind={kind}
        open={publishOpen}
        onOpenChange={setPublishOpen}
      />

      {/* ────────────────────────── Overview charts ────────────────────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="lg:col-span-1">
          <PublishActivity input={overviewInput} />
        </div>
        <KindBreakdown input={overviewInput} />
        <ThirdPartyMix input={overviewInput} />
      </div>

      {/* ────────────────────────── KPI strip ────────────────────────── */}
      <KPIStrip
        counts={counts}
        activeKind={kind}
        onSelect={setKind}
        deployCount={deployCount}
        runningDeployments={runningDeployments}
      />

      {/* ────────────────────────── Toolbar ────────────────────────── */}
      <div className="flex flex-col md:flex-row md:items-center gap-3">
        <div className="relative flex-1 md:max-w-sm">
          <Icon
            icon={icons.search}
            className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
          />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={`Search ${kind}…`}
            className="pl-9 h-9"
          />
        </div>

        <Select value={sort} onValueChange={(v) => setSort(v as SortMode)}>
          <SelectTrigger className="h-9 w-[10rem]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="recent">Most recent</SelectItem>
            <SelectItem value="alpha">Alphabetical</SelectItem>
            <SelectItem value="score">Score</SelectItem>
          </SelectContent>
        </Select>

        <div className="flex items-center gap-2 ml-auto">
          <Switch
            id="latest-only"
            checked={latestOnly}
            onCheckedChange={setLatestOnly}
          />
          <Label
            htmlFor="latest-only"
            className="cursor-pointer text-[13px] text-muted-foreground"
          >
            Latest only
          </Label>
        </div>
      </div>

      {/* Package-type filter chips (servers only) */}
      {kind === "servers" && availablePkgTypes.length > 0 && (
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mr-1">
            packages
          </span>
          <button
            onClick={() => setPkgFilter(null)}
            className={cn(
              "text-[11px] font-mono uppercase tracking-wider h-6 px-2 rounded-md ring-1 transition",
              pkgFilter === null
                ? "bg-foreground text-background ring-foreground"
                : "bg-muted text-muted-foreground ring-border hover:bg-foreground/10",
            )}
          >
            all
          </button>
          {availablePkgTypes.map((t) => (
            <button
              key={t}
              onClick={() => setPkgFilter(t === pkgFilter ? null : t)}
              className={cn(
                "text-[11px] font-mono uppercase tracking-wider h-6 px-2 rounded-md ring-1 transition",
                pkgFilter === t
                  ? "bg-foreground text-background ring-foreground"
                  : "bg-muted text-muted-foreground ring-border hover:bg-foreground/10",
              )}
            >
              {t}
            </button>
          ))}
        </div>
      )}

      {/* ────────────────────────── Tabs + Grid ────────────────────────── */}
      <Tabs value={kind} onValueChange={(v) => setKind(v as Kind)}>
        <TabsList className="grid grid-cols-4 w-full max-w-2xl">
          {(Object.keys(KIND_META) as Kind[]).map((k) => (
            <TabsTrigger key={k} value={k} className="gap-2 data-[state=active]:font-semibold">
              <KindIcon kind={KIND_META[k].singular} size="sm" className="!h-5 !w-5 !rounded" />
              <span className="hidden sm:inline">{KIND_META[k].label.replace(/^MCP /, "")}</span>
              <span className="text-[10px] font-mono tabular-nums text-muted-foreground">
                {counts[k]}
              </span>
            </TabsTrigger>
          ))}
        </TabsList>

        {(Object.keys(KIND_META) as Kind[]).map((k) => (
          <TabsContent key={k} value={k} className="mt-5">
            <p className="text-sm text-muted-foreground mb-4 flex items-center gap-2">
              <Icon
                icon={kindAccent(KIND_META[k].singular).icon}
                className="h-4 w-4"
              />
              {KIND_META[k].tagline}
            </p>

            {activeQueryLoading && k === kind ? (
              <LoadingGrid />
            ) : filteredItems.length === 0 ? (
              <EmptyState
                icon={icons.registry}
                title={KIND_META[k].empty}
                description={
                  search
                    ? `No matches for "${search}". Try clearing the filter.`
                    : undefined
                }
              />
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                {filteredItems.map((row) => (
                  <ArtifactCard
                    key={row.id}
                    kind={KIND_META[k].singular}
                    row={row}
                    extras={extrasFor(k, row, deployments.data?.deployments)}
                    onClick={() =>
                      setDetail({ kind: k, name: row.name, version: row.version })
                    }
                  />
                ))}
              </div>
            )}
          </TabsContent>
        ))}
      </Tabs>

      <DetailDrawer
        target={detail}
        onClose={() => setDetail(null)}
        deployments={deployments.data?.deployments ?? []}
      />
    </PageShell>
  );
}

// ---------------------------------------------------------------------------
// KPI strip
// ---------------------------------------------------------------------------

function KPIStrip({
  counts,
  activeKind,
  onSelect,
  deployCount,
  runningDeployments,
}: {
  counts: Record<Kind, number>;
  activeKind: Kind;
  onSelect: (k: Kind) => void;
  deployCount: number;
  runningDeployments: number;
}) {
  const tiles: {
    key: Kind | "deployments";
    label: string;
    value: number;
    secondary?: string;
    kind?: RegistryKind;
  }[] = [
    { key: "servers", label: "Servers", value: counts.servers, kind: "server" },
    { key: "agents", label: "Agents", value: counts.agents, kind: "agent" },
    { key: "skills", label: "Skills", value: counts.skills, kind: "skill" },
    { key: "prompts", label: "Prompts", value: counts.prompts, kind: "prompt" },
    {
      key: "deployments",
      label: "Deployments",
      value: deployCount,
      secondary: `${runningDeployments} running`,
    },
  ];
  return (
    <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
      {tiles.map((t) => {
        const isTab = t.key !== "deployments";
        const active = isTab && activeKind === t.key;
        return (
          <button
            key={t.key}
            onClick={() => isTab && onSelect(t.key as Kind)}
            disabled={!isTab}
            className={cn(
              "group relative text-left rounded-lg border transition-all",
              "px-3 py-3",
              active
                ? "border-foreground/40 bg-foreground/[0.03]"
                : "border-border hover:border-foreground/30",
              !isTab && "cursor-default",
            )}
          >
            <div className="flex items-center justify-between mb-2">
              {t.kind ? (
                <KindIcon kind={t.kind} size="sm" />
              ) : (
                <div className="h-8 w-8 rounded-lg bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-1 ring-sky-500/20 flex items-center justify-center">
                  <Icon icon={icons.deployment} className="h-4 w-4" />
                </div>
              )}
              {active && (
                <span className="h-1.5 w-1.5 rounded-full bg-primary" />
              )}
            </div>
            <div className="flex items-baseline gap-2">
              <span className="text-2xl font-semibold tabular-nums tracking-tight">
                {t.value}
              </span>
              {t.secondary && (
                <span className="text-[10px] font-mono text-muted-foreground">
                  · {t.secondary}
                </span>
              )}
            </div>
            <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mt-0.5">
              {t.label}
            </p>
          </button>
        );
      })}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Loading shimmer for the grid
// ---------------------------------------------------------------------------

function LoadingGrid() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="rounded-xl border border-border/60 bg-card p-4 space-y-3 animate-pulse"
        >
          <div className="flex gap-3">
            <div className="h-10 w-10 rounded-lg bg-muted" />
            <div className="flex-1 space-y-2">
              <div className="h-3 w-1/2 rounded bg-muted" />
              <div className="h-2 w-3/4 rounded bg-muted/70" />
            </div>
          </div>
          <div className="h-2 w-full rounded bg-muted/70" />
          <div className="h-2 w-4/5 rounded bg-muted/70" />
        </div>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Per-kind "extras" projector — what to show in the card metadata rail
// ---------------------------------------------------------------------------

type AnyRow = RegistryServer | RegistryAgent | RegistrySkill | RegistryPrompt;

function extrasFor(kind: Kind, row: AnyRow, deployments?: Deployment[]) {
  switch (kind) {
    case "servers": {
      const s = row as RegistryServer;
      const dep = deploymentFor(deployments, s.name, s.version);
      return {
        packages: parsePackages(s.packages),
        deployStatus: dep?.status,
      };
    }
    case "agents": {
      const a = row as RegistryAgent;
      const tagline = [a.language, a.framework, a.model_provider]
        .filter(Boolean)
        .join(" · ");
      return { tagline: tagline || undefined };
    }
    case "skills": {
      const s = row as RegistrySkill;
      return { tagline: s.image };
    }
    case "prompts": {
      const p = row as RegistryPrompt;
      if (!p.template) return {};
      return {
        tagline:
          p.template.slice(0, 80) + (p.template.length > 80 ? "…" : ""),
      };
    }
  }
}

// ---------------------------------------------------------------------------
// Detail drawer with tabs
// ---------------------------------------------------------------------------

function DetailDrawer({
  target,
  onClose,
  deployments,
}: {
  target: { kind: Kind; name: string; version?: string } | null;
  onClose: () => void;
  deployments: Deployment[];
}) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [activeTab, setActiveTab] = useState("overview");
  // Override version explicitly; `target.version` is the original click target
  // but the user can change it via the version picker.
  const [selectedVersion, setSelectedVersion] = useState<string | undefined>();

  // Reset tab + version selection when target changes.
  useMemo(() => {
    setActiveTab("overview");
    setSelectedVersion(undefined);
  }, [target?.kind, target?.name]);

  const effectiveVersion = selectedVersion ?? target?.version;

  const deleteMutation = useDeleteMutation(() => {
    setConfirmDelete(false);
    onClose();
  });

  const detailQuery = useQuery({
    queryKey: ["registry-detail", target?.kind, target?.name, effectiveVersion],
    queryFn: async () => {
      if (!target) return null;
      switch (target.kind) {
        case "servers":
          return { kind: target.kind, row: await getRegistryServer(target.name, effectiveVersion) } as const;
        case "agents":
          return { kind: target.kind, row: await getRegistryAgent(target.name, effectiveVersion) } as const;
        case "skills":
          return { kind: target.kind, row: await getRegistrySkill(target.name, effectiveVersion) } as const;
        case "prompts":
          return { kind: target.kind, row: await getRegistryPrompt(target.name, effectiveVersion) } as const;
      }
    },
    enabled: !!target,
  });

  const versionsQuery = useQuery({
    queryKey: ["registry-versions", target?.kind, target?.name],
    queryFn: async () => {
      if (!target) return [];
      switch (target.kind) {
        case "servers":
          return (await listRegistryServerVersions(target.name)).versions;
        case "agents":
          return (await listRegistryAgentVersions(target.name)).versions;
        case "skills":
          return (await listRegistrySkillVersions(target.name)).versions;
        case "prompts":
          return (await listRegistryPromptVersions(target.name)).versions;
      }
    },
    enabled: !!target,
  });

  // Server detail comes wrapped with scores when enrichment is wired.
  const serverEnvelope =
    target?.kind === "servers" && detailQuery.data
      ? (detailQuery.data.row as RegistryServerDetail)
      : null;
  const resolvedRow =
    serverEnvelope ? serverEnvelope.server : detailQuery.data?.row;
  const scores: EnrichmentScore[] = serverEnvelope?.scores ?? [];

  const kindSingular = target ? KIND_META[target.kind].singular : null;

  // The concrete version the drawer is actually showing (covers "latest").
  const actualVersion =
    detailQuery.data?.kind === "agents"
      ? (detailQuery.data.row as RegistryAgentWithRefs).agent.version
      : (resolvedRow as AnyRow | undefined)?.version;

  const associatedDeployment =
    target?.kind === "servers"
      ? deploymentFor(deployments, target.name, actualVersion)
      : undefined;

  return (
    <Sheet open={!!target} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-2xl w-full p-0 overflow-hidden flex flex-col">
        {/* ── HERO ── */}
        <SheetHeader className="px-6 pt-6 pb-4 border-b space-y-3">
          <div className="flex items-start gap-4">
            {kindSingular && <KindIcon kind={kindSingular} name={target?.name} size="lg" />}
            <div className="flex-1 min-w-0">
              <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                {target && KIND_META[target.kind].label}
              </p>
              <SheetTitle className="text-2xl font-semibold leading-tight tracking-tight truncate">
                {(resolvedRow as AnyRow | undefined)?.title ||
                  target?.name.split("/").slice(-1)[0]}
              </SheetTitle>
              <code className="text-xs font-mono text-muted-foreground block truncate mt-1">
                {target?.name}
              </code>
            </div>
          </div>
          <SheetDescription asChild>
            <div className="flex items-center gap-2 flex-wrap">
              {actualVersion && (
                <VersionPicker
                  versions={(versionsQuery.data ?? []).map((v) => ({
                    version: v.version,
                    is_latest: v.is_latest,
                    status: v.status,
                    published_at: v.published_at,
                  }))}
                  selected={actualVersion}
                  onSelect={setSelectedVersion}
                  loading={versionsQuery.isLoading}
                />
              )}
              {(resolvedRow as AnyRow | undefined)?.status === "deprecated" && (
                <Badge variant="outline" className="text-amber-600 border-amber-500/40">
                  deprecated
                </Badge>
              )}
              {(resolvedRow as AnyRow | undefined)?.published_at && (
                <span className="text-[11px] font-mono text-muted-foreground">
                  published{" "}
                  {formatDistanceToNow(
                    new Date((resolvedRow as AnyRow).published_at),
                    { addSuffix: true },
                  )}
                </span>
              )}
            </div>
          </SheetDescription>

          {/* Quick actions */}
          {target && actualVersion && detailQuery.data && (
            <QuickActions
              kind={target.kind}
              name={target.name}
              version={actualVersion}
              websiteUrl={(resolvedRow as AnyRow | undefined)?.website_url}
              deployment={associatedDeployment}
              onDelete={() => setConfirmDelete(true)}
            />
          )}
        </SheetHeader>

        {/* ── TABS ── */}
        <div className="flex-1 overflow-y-auto">
          {detailQuery.isLoading || !detailQuery.data ? (
            <div className="flex items-center justify-center h-48">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
            </div>
          ) : (
            <Tabs value={activeTab} onValueChange={setActiveTab} className="px-6 pt-4">
              <TabsList className="grid grid-cols-4 w-full">
                <TabsTrigger value="overview">Overview</TabsTrigger>
                <TabsTrigger value="versions">
                  Versions
                  <span className="text-[10px] font-mono text-muted-foreground ml-1.5">
                    {versionsQuery.data?.length ?? ""}
                  </span>
                </TabsTrigger>
                {target && (target.kind === "servers" || target.kind === "skills" || target.kind === "prompts") && (
                  <TabsTrigger value="refs">Used by</TabsTrigger>
                )}
                {target?.kind === "servers" && (
                  <TabsTrigger value="deploy">Deploy</TabsTrigger>
                )}
                {target?.kind === "agents" && (
                  <TabsTrigger value="deps">Dependencies</TabsTrigger>
                )}
              </TabsList>

              {/* Overview tab */}
              <TabsContent value="overview" className="mt-4 pb-8 space-y-5">
                {(resolvedRow as AnyRow | undefined)?.description && (
                  <p className="text-sm leading-relaxed">
                    {(resolvedRow as AnyRow).description}
                  </p>
                )}

                {/* Install command + scores side-by-side on wider screens */}
                {target?.kind === "servers" && (
                  <ServerOverview
                    row={resolvedRow as RegistryServer}
                    scores={scores}
                  />
                )}
                {target?.kind === "agents" && (
                  <AgentOverview
                    data={detailQuery.data.row as RegistryAgentWithRefs}
                  />
                )}
                {target?.kind === "skills" && (
                  <SkillOverview row={resolvedRow as RegistrySkill} />
                )}
                {target?.kind === "prompts" && (
                  <PromptOverview row={resolvedRow as RegistryPrompt} />
                )}

                {(resolvedRow as AnyRow | undefined)?.metadata ? (
                  <JSONBlock
                    label="Metadata"
                    value={(resolvedRow as AnyRow).metadata}
                  />
                ) : null}
              </TabsContent>

              {/* Versions tab */}
              <TabsContent value="versions" className="mt-4 pb-8">
                <VersionsTab
                  versions={versionsQuery.data ?? []}
                  loading={versionsQuery.isLoading}
                  selected={actualVersion}
                  onSelect={setSelectedVersion}
                />
              </TabsContent>

              {/* Used by (cross-refs) */}
              {target && target.kind !== "agents" && (
                <TabsContent value="refs" className="mt-4 pb-8">
                  <DependentsTab
                    targetKind={KIND_META[target.kind].singular}
                    targetName={target.name}
                  />
                </TabsContent>
              )}

              {/* Deploy (server only) */}
              {target?.kind === "servers" && (
                <TabsContent value="deploy" className="mt-4 pb-8 space-y-3">
                  <DeployTab
                    name={target.name}
                    version={actualVersion ?? target.version ?? ""}
                    deployment={associatedDeployment}
                  />
                </TabsContent>
              )}

              {/* Dependencies (agent only) */}
              {target?.kind === "agents" && (
                <TabsContent value="deps" className="mt-4 pb-8">
                  <DepsTab data={detailQuery.data.row as RegistryAgentWithRefs} />
                </TabsContent>
              )}
            </Tabs>
          )}
        </div>

        {/* Delete confirm */}
        <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                Delete version {actualVersion}?
              </AlertDialogTitle>
              <AlertDialogDescription>
                <span className="font-mono">{target?.name}</span> at version{" "}
                <span className="font-mono">{actualVersion}</span> will be
                marked deleted. If this was the latest, the next-newest active
                version takes over.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                onClick={() =>
                  target &&
                  actualVersion &&
                  deleteMutation.mutate({
                    kind: target.kind,
                    name: target.name,
                    version: actualVersion,
                  })
                }
              >
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </SheetContent>
    </Sheet>
  );
}

// ---------------------------------------------------------------------------
// Quick actions row (inside drawer header)
// ---------------------------------------------------------------------------

function QuickActions({
  kind,
  name,
  version,
  websiteUrl,
  deployment,
  onDelete,
}: {
  kind: Kind;
  name: string;
  version: string;
  websiteUrl?: string;
  deployment?: Deployment;
  onDelete: () => void;
}) {
  const { toast } = useToast();
  const qc = useQueryClient();

  const deployMutation = useMutation({
    mutationFn: () =>
      deployRegistryServer({ server_name: name, server_version: version }),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({
        title: deployment ? "Re-deploy started" : "Deploy started",
        description: `${d.workload_name} · ${d.status}`,
      });
    },
    onError: (e: any) => {
      toast({
        title: "Deploy failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const deployDisabled =
    deployMutation.isPending || deployInProgress(deployment?.status);
  const deployLabel = deployMutation.isPending
    ? "Deploying…"
    : deployment?.status === "deploying" || deployment?.status === "pending"
      ? "Deploying…"
      : deployment?.status === "terminating"
        ? "Terminating…"
        : deployment?.status === "running"
          ? "Re-deploy"
          : deployment?.status === "failed"
            ? "Retry"
            : "Deploy";

  return (
    <div className="flex items-center gap-2 flex-wrap pt-2">
      {kind === "servers" && (
        <Button
          size="sm"
          variant="default"
          onClick={() => deployMutation.mutate()}
          disabled={deployDisabled}
          className="gap-1.5"
        >
          <Icon icon={icons.rocket} className="h-3.5 w-3.5" />
          {deployLabel}
        </Button>
      )}
      {websiteUrl && (
        <Button asChild size="sm" variant="outline" className="gap-1.5">
          <a href={websiteUrl} target="_blank" rel="noopener noreferrer">
            <Icon icon={icons.externalLink} className="h-3.5 w-3.5" />
            Website
          </a>
        </Button>
      )}
      <div className="ml-auto">
        <Button
          size="sm"
          variant="ghost"
          onClick={onDelete}
          className="text-destructive hover:text-destructive hover:bg-destructive/10 gap-1.5"
        >
          <Icon icon={icons.delete} className="h-3.5 w-3.5" />
          Delete version
        </Button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Per-kind overview panels
// ---------------------------------------------------------------------------

function ServerOverview({
  row,
  scores,
}: {
  row: RegistryServer;
  scores: EnrichmentScore[];
}) {
  const pkgs = parsePackages(row.packages);
  return (
    <div className="space-y-4">
      {pkgs.length > 0 && (
        <section className="space-y-2">
          <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Install
          </p>
          <InstallCommand packages={pkgs} />
          {pkgs.length > 1 && (
            <div className="flex items-center gap-1.5 flex-wrap">
              {pkgs.map((p, i) => (
                <span
                  key={i}
                  className="text-[10px] font-mono text-muted-foreground inline-flex items-center gap-1"
                >
                  <PackageChip type={p.registry_type} />
                  {p.identifier}@{p.version}
                </span>
              ))}
            </div>
          )}
        </section>
      )}

      {scores.length > 0 && <ScoresPanel scores={scores} />}

      {row.remotes ? (
        <JSONBlock label="Remotes" value={row.remotes} />
      ) : null}
    </div>
  );
}

function AgentOverview({ data }: { data: RegistryAgentWithRefs }) {
  const pills = [
    { label: "Language", value: data.agent.language },
    { label: "Framework", value: data.agent.framework },
    { label: "Model", value: data.agent.model_name },
    { label: "Provider", value: data.agent.model_provider },
  ].filter((p) => p.value);
  return (
    <div className="space-y-4">
      {pills.length > 0 && (
        <div className="grid grid-cols-2 gap-x-4 gap-y-2">
          {pills.map((p) => (
            <div key={p.label}>
              <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                {p.label}
              </p>
              <p className="text-sm font-medium font-mono">{p.value}</p>
            </div>
          ))}
        </div>
      )}
      <div className="grid grid-cols-3 gap-2 text-center">
        <DepSummary count={data.servers?.length ?? 0} label="Servers" kind="server" />
        <DepSummary count={data.skills?.length ?? 0} label="Skills" kind="skill" />
        <DepSummary count={data.prompts?.length ?? 0} label="Prompts" kind="prompt" />
      </div>
    </div>
  );
}

function DepSummary({
  count,
  label,
  kind,
}: {
  count: number;
  label: string;
  kind: RegistryKind;
}) {
  return (
    <div className="rounded-lg border border-border/80 bg-card p-3 flex flex-col items-center gap-1">
      <KindIcon kind={kind} size="sm" />
      <span className="text-xl font-semibold tabular-nums leading-none">{count}</span>
      <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        {label}
      </span>
    </div>
  );
}

function SkillOverview({ row }: { row: RegistrySkill }) {
  return (
    <div className="space-y-3">
      {row.image && (
        <section>
          <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-1">
            OCI Image
          </p>
          <code className="text-xs font-mono bg-muted px-2 py-1.5 rounded block break-all">
            {row.image}
          </code>
        </section>
      )}
      {row.manifest ? (
        <JSONBlock label="Manifest" value={row.manifest} />
      ) : null}
    </div>
  );
}

function PromptOverview({ row }: { row: RegistryPrompt }) {
  return (
    <div className="space-y-3">
      {row.template && (
        <section>
          <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-1">
            Template
          </p>
          <pre className="text-xs font-mono bg-muted px-3 py-3 rounded-md whitespace-pre-wrap max-h-64 overflow-y-auto leading-relaxed">
            {row.template}
          </pre>
        </section>
      )}
      {row.arguments ? (
        <JSONBlock label="Arguments" value={row.arguments} />
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------

function VersionsTab({
  versions,
  loading,
  selected,
  onSelect,
}: {
  versions: {
    version: string;
    is_latest: boolean;
    status: string;
    published_at?: string;
    description?: string;
  }[];
  loading: boolean;
  selected?: string;
  onSelect: (v: string) => void;
}) {
  if (loading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary" />
      </div>
    );
  }
  if (!versions.length) {
    return (
      <p className="text-xs text-muted-foreground text-center py-8">
        No version history.
      </p>
    );
  }
  return (
    <div className="relative">
      <div className="absolute left-[11px] top-2 bottom-2 w-px bg-border" />
      <ul className="space-y-3">
        {versions.map((v) => {
          const active = v.version === selected;
          return (
            <li key={v.version}>
              <button
                onClick={() => onSelect(v.version)}
                className={cn(
                  "w-full text-left flex items-start gap-3 px-3 py-2 rounded-md -ml-3",
                  active ? "bg-muted/60" : "hover:bg-muted/40",
                )}
              >
                <span
                  className={cn(
                    "relative z-10 mt-1 h-2 w-2 rounded-full shrink-0",
                    v.is_latest ? "bg-emerald-500 ring-2 ring-emerald-500/30" :
                    v.status === "deleted" ? "bg-muted" : "bg-foreground/30",
                  )}
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm tabular-nums">v{v.version}</span>
                    {v.is_latest && (
                      <span className="text-[9px] font-mono uppercase tracking-widest text-emerald-600 dark:text-emerald-400">
                        latest
                      </span>
                    )}
                    {v.status === "deprecated" && (
                      <span className="text-[9px] font-mono uppercase tracking-widest text-amber-600 dark:text-amber-400">
                        deprecated
                      </span>
                    )}
                  </div>
                  {v.published_at && (
                    <p className="text-[11px] text-muted-foreground">
                      {formatDistanceToNow(new Date(v.published_at), { addSuffix: true })}
                    </p>
                  )}
                  {v.description && (
                    <p className="text-[12px] text-muted-foreground line-clamp-2 mt-0.5">
                      {v.description}
                    </p>
                  )}
                </div>
                {active && (
                  <Icon icon={icons.check} className="h-3.5 w-3.5 text-emerald-500 shrink-0 mt-1.5" />
                )}
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function DependentsTab({
  targetKind,
  targetName,
}: {
  targetKind: RegistryKind;
  targetName: string;
}) {
  const navigate = useNavigate();
  const { data, isLoading } = useQuery({
    queryKey: ["registry-dependents", targetKind, targetName],
    queryFn: () => listRegistryDependents(targetKind, targetName),
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-primary" />
      </div>
    );
  }
  const rows = data?.dependents ?? [];
  if (rows.length === 0) {
    return (
      <div className="text-center py-10">
        <Icon
          icon={icons.agent}
          className="h-10 w-10 mx-auto text-muted-foreground mb-3"
        />
        <p className="text-sm text-muted-foreground">
          No agents or use cases depend on this {targetKind} yet.
        </p>
      </div>
    );
  }

  // Split for a clearer read: business-activity dependents first, then
  // registry-internal composition.
  const useCases = rows.filter((r) => r.owner_kind === "use_case");
  const agents = rows.filter((r) => r.owner_kind === "agent");

  return (
    <div className="space-y-5">
      {useCases.length > 0 && (
        <section className="space-y-2">
          <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Use cases{" "}
            <span className="text-foreground/70">({useCases.length})</span>
          </p>
          <div className="space-y-1.5">
            {useCases.map((r, i) => (
              <button
                key={i}
                onClick={() => navigate("/use-cases")}
                className="w-full text-left flex items-start gap-3 p-3 rounded-md border border-border/70 bg-card hover:border-primary/40 transition-colors"
              >
                <div className="h-8 w-8 rounded-lg bg-primary/10 text-primary ring-1 ring-primary/20 flex items-center justify-center shrink-0">
                  <Icon icon={icons.useCase} className="h-4 w-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <span className="font-semibold text-sm truncate block">
                    {r.title || r.name}
                  </span>
                  <code className="text-[11px] font-mono text-muted-foreground block truncate">
                    {r.name}
                  </code>
                  {(r.target_version || r.local_name) && (
                    <p className="text-[11px] text-muted-foreground mt-1">
                      {r.local_name && (
                        <>alias <code className="font-mono">{r.local_name}</code></>
                      )}
                      {r.local_name && r.target_version && " · "}
                      {r.target_version && (
                        <>pinned to <code className="font-mono">v{r.target_version}</code></>
                      )}
                    </p>
                  )}
                </div>
              </button>
            ))}
          </div>
        </section>
      )}
      {agents.length > 0 && (
        <section className="space-y-2">
          <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
            Agents{" "}
            <span className="text-foreground/70">({agents.length})</span>
          </p>
          <div className="space-y-1.5">
            {agents.map((r, i) => (
              <div
                key={i}
                className="flex items-start gap-3 p-3 rounded-md border border-border/70 bg-card"
              >
                <KindIcon kind={r.owner_kind} name={r.name} size="sm" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="font-semibold text-sm truncate">
                      {r.title || r.name.split("/").slice(-1)[0]}
                    </span>
                    <span className="font-mono text-[10px] text-muted-foreground tabular-nums">
                      v{r.version}
                    </span>
                  </div>
                  <code className="text-[11px] font-mono text-muted-foreground block truncate">
                    {r.name}
                  </code>
                  {(r.target_version || r.local_name) && (
                    <p className="text-[11px] text-muted-foreground mt-1">
                      {r.local_name && (
                        <>alias <code className="font-mono">{r.local_name}</code></>
                      )}
                      {r.local_name && r.target_version && " · "}
                      {r.target_version && (
                        <>pinned to <code className="font-mono">v{r.target_version}</code></>
                      )}
                    </p>
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function DeployTab({
  name,
  version,
  deployment,
}: {
  name: string;
  version: string;
  deployment?: Deployment;
}) {
  const navigate = useNavigate();
  const { toast } = useToast();
  const qc = useQueryClient();

  const deployMutation = useMutation({
    mutationFn: () =>
      deployRegistryServer({ server_name: name, server_version: version }),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({ title: "Deploy started", description: `${d.workload_name} · ${d.status}` });
    },
    onError: (e: any) => {
      toast({
        title: "Deploy failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  if (!deployment) {
    return (
      <div className="text-center py-10 space-y-4">
        <div className="h-12 w-12 rounded-full bg-primary/10 text-primary mx-auto flex items-center justify-center">
          <Icon icon={icons.rocket} className="h-6 w-6" />
        </div>
        <p className="text-sm text-muted-foreground max-w-xs mx-auto">
          Not deployed yet. Deploy creates a dedicated Kubernetes workload in the
          <code className="font-mono text-xs"> pllm-mcp </code>
          namespace and auto-registers it with the MCP Gateway.
        </p>
        <Button
          onClick={() => deployMutation.mutate()}
          disabled={deployMutation.isPending}
          className="gap-2"
        >
          <Icon icon={icons.rocket} className="h-4 w-4" />
          {deployMutation.isPending ? "Deploying…" : "Deploy to Kubernetes"}
        </Button>
      </div>
    );
  }
  return (
    <Card className="p-0">
      <CardContent className="p-4 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
            <span className="font-mono text-sm uppercase tracking-wider">
              {deployment.status}
            </span>
          </div>
          <Badge variant="secondary" className="capitalize">
            {deployment.platform}
          </Badge>
        </div>
        <div className="space-y-2 text-[13px]">
          <Detail label="Namespace" value={deployment.namespace} mono />
          <Detail label="Workload" value={deployment.workload_name} mono />
          {deployment.endpoint && (
            <Detail label="Endpoint" value={deployment.endpoint} mono />
          )}
          {deployment.last_applied_at && (
            <Detail
              label="Last applied"
              value={formatDistanceToNow(new Date(deployment.last_applied_at), {
                addSuffix: true,
              })}
            />
          )}
          {deployment.status_reason && (
            <p className="text-xs text-muted-foreground">
              {deployment.status_reason}
            </p>
          )}
        </div>
        <div className="flex justify-end gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate("/deployments")}
            className="gap-1.5"
          >
            <Icon icon={icons.externalLink} className="h-3.5 w-3.5" />
            Open
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => deployMutation.mutate()}
            disabled={deployInProgress(deployment.status) || deployMutation.isPending}
            className="gap-1.5"
          >
            <Icon icon={icons.refresh} className="h-3.5 w-3.5" />
            Re-deploy
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function Detail({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-start gap-3">
      <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground w-24 shrink-0 pt-0.5">
        {label}
      </span>
      <span
        className={cn(
          "text-[13px] flex-1 min-w-0 break-all",
          mono && "font-mono",
        )}
      >
        {value}
      </span>
    </div>
  );
}

function DepsTab({ data }: { data: RegistryAgentWithRefs }) {
  const sections: { label: string; kind: RegistryKind; refs: RegistryAgentWithRefs["servers"] }[] = [
    { label: "LLMs", kind: "llm", refs: data.llms },
    { label: "MCP Servers", kind: "server", refs: data.servers },
    { label: "Skills", kind: "skill", refs: data.skills },
    { label: "Prompts", kind: "prompt", refs: data.prompts },
  ];
  const anyDeclared = sections.some((s) => (s.refs?.length ?? 0) > 0);

  return (
    <div className="space-y-6">
      <section className="space-y-2">
        <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
          Declared
        </h3>
        {!anyDeclared ? (
          <p className="text-sm text-muted-foreground py-3">
            No declared dependencies on this version.
          </p>
        ) : (
          <div className="space-y-5">
            {sections.map((s) =>
              !s.refs || s.refs.length === 0 ? null : (
                <div key={s.label} className="space-y-2">
                  <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                    {s.label}{" "}
                    <span className="text-foreground/70">({s.refs.length})</span>
                  </p>
                  <div className="space-y-1.5">
                    {s.refs.map((r, i) => (
                      <div
                        key={i}
                        className="flex items-center gap-3 p-2.5 rounded-md border border-border/60 bg-card"
                      >
                        <KindIcon kind={s.kind} name={r.target_name} size="sm" />
                        <div className="flex-1 min-w-0">
                          <code className="text-[12px] font-mono truncate block">
                            {r.target_name}
                          </code>
                          <p className="text-[10px] text-muted-foreground">
                            {r.local_name && (
                              <>
                                alias{" "}
                                <code className="font-mono text-foreground/80">
                                  {r.local_name}
                                </code>
                                {" · "}
                              </>
                            )}
                            pinned to{" "}
                            <code className="font-mono text-foreground/80">
                              {r.target_version ? `v${r.target_version}` : "latest"}
                            </code>
                          </p>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              ),
            )}
          </div>
        )}
      </section>

      <ObservedDepsSection
        name={data.agent.name}
        version={data.agent.version}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Observed deps panel — runtime targets fingerprinted from OTel traces.
// Decoupled from the declared-refs view above so an agent can have one,
// the other, or both. Renders nothing when no DiscoveredAgent points at
// this RegistryAgent.
// ---------------------------------------------------------------------------

const OBSERVED_KIND_TONE: Record<string, string> = {
  llm: "bg-indigo-100 text-indigo-800 dark:bg-indigo-950 dark:text-indigo-200",
  mcp: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-200",
  vector_db: "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200",
  http: "bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-200",
  unknown: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200",
};

const OBSERVED_OUTCOME_TONE: Record<string, string> = {
  allowed: "text-emerald-700 dark:text-emerald-400",
  flagged: "text-amber-700 dark:text-amber-400",
  denied: "text-rose-700 dark:text-rose-400",
};

const OBSERVED_KIND_FALLBACK: Record<string, string> = {
  llm: "solar:cpu-bolt-linear",
  mcp: "lucide:plug-zap",
  vector_db: "solar:database-linear",
  http: "solar:global-linear",
  unknown: "solar:question-circle-linear",
};

function vectorDbIconForObserved(target: string): string {
  const t = target.toLowerCase();
  if (t.startsWith("qdrant")) return "simple-icons:qdrant";
  if (t.startsWith("pinecone")) return "simple-icons:pinecone";
  if (t.startsWith("weaviate")) return "simple-icons:weaviate";
  if (t.startsWith("chroma")) return "logos:chroma";
  if (t.startsWith("milvus")) return "simple-icons:milvus";
  return OBSERVED_KIND_FALLBACK.vector_db;
}

function observedVisual(call: ObservedAgentCall): { icon: string; label: string; tone?: string } {
  switch (call.kind) {
    case "llm": {
      const p = detectProvider(call.target_name, call.target_name);
      return { icon: p.icon, label: p.name, tone: p.color };
    }
    case "vector_db":
      return {
        icon: vectorDbIconForObserved(call.target_name),
        label: call.target_name.split("://")[0],
      };
    case "mcp":
      return { icon: OBSERVED_KIND_FALLBACK.mcp, label: "MCP server" };
    case "http":
      return { icon: OBSERVED_KIND_FALLBACK.http, label: "HTTP" };
    default:
      return { icon: OBSERVED_KIND_FALLBACK.unknown, label: "unknown" };
  }
}

function ObservedDepsSection({ name, version }: { name: string; version: string }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["registry-agent-observed", name, version],
    queryFn: () => getObservedAgentDeps(name, version),
    refetchInterval: 5000,
    retry: false,
  });

  const linked = !!data?.discovered_agent_id;
  const calls = data?.calls ?? [];

  return (
    <section className="space-y-2">
      <div className="flex items-center gap-2">
        <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
          Observed in production
        </h3>
        <span className="text-[10px] font-mono text-muted-foreground">
          via OpenTelemetry
        </span>
        {linked && data.flagged_count > 0 && (
          <Badge variant="destructive" className="font-mono text-[10px]">
            {data.flagged_count} flagged
          </Badge>
        )}
      </div>
      {error ? (
        <p className="text-sm text-muted-foreground py-3">
          Couldn't load observed deps —{" "}
          <code className="font-mono text-[11px]">/v1/traces</code> may not be
          deployed on this build yet.
        </p>
      ) : isLoading ? (
        <p className="text-sm text-muted-foreground py-3">Loading…</p>
      ) : !linked ? (
        <p className="text-sm text-muted-foreground py-3">
          This agent isn't linked to any OTel-discovered fingerprint. Either it
          was published manually, or no traces have arrived yet for the
          matching <code className="font-mono">service.name</code>.
        </p>
      ) : calls.length === 0 ? (
        <p className="text-sm text-muted-foreground py-3">
          Linked but no calls observed yet — run the agent so its OTel spans
          hit <code className="font-mono">/v1/traces</code>.
        </p>
      ) : (
        <div className="space-y-1.5">
          {calls.map((c, i) => {
            const v = observedVisual(c);
            return (
              <div
                key={i}
                className="flex items-start gap-3 p-2.5 rounded-md border border-border/60 bg-card"
              >
                <Icon
                  icon={v.icon}
                  className={`h-6 w-6 mt-0.5 shrink-0 ${v.tone ?? ""}`}
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <Badge className={OBSERVED_KIND_TONE[c.kind] ?? ""}>
                      {c.kind}
                    </Badge>
                    <span className="text-[11px] text-muted-foreground">
                      {v.label}
                    </span>
                    <span className="ml-auto text-[11px] text-muted-foreground font-mono">
                      ×{c.count}
                    </span>
                  </div>
                  <code className="text-[12px] font-mono break-all block mt-1">
                    {c.target_name}
                  </code>
                  {c.last_outcome && (
                    <p
                      className={`mt-1 text-[10px] uppercase tracking-wide ${OBSERVED_OUTCOME_TONE[c.last_outcome] ?? ""}`}
                    >
                      {c.last_outcome}
                      {c.last_reason ? `: ${c.last_reason}` : ""}
                    </p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}

// ---------------------------------------------------------------------------
// Helpers (shared)
// ---------------------------------------------------------------------------

function JSONBlock({ label, value }: { label: string; value: unknown }) {
  return (
    <details className="group">
      <summary className="cursor-pointer text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-1 select-none flex items-center gap-1 hover:text-foreground">
        <Icon
          icon={icons.chevronRight}
          className="h-3 w-3 transition-transform group-open:rotate-90"
        />
        {label}
      </summary>
      <pre className="text-xs bg-muted p-2 rounded overflow-x-auto max-h-48 mt-1 font-mono leading-relaxed">
        {JSON.stringify(value, null, 2)}
      </pre>
    </details>
  );
}

function useDeleteMutation(onSuccess: () => void) {
  const qc = useQueryClient();
  const { toast } = useToast();
  return useMutation({
    mutationFn: async (t: { kind: Kind; name: string; version: string }) => {
      switch (t.kind) {
        case "servers":
          return deleteRegistryServerVersion(t.name, t.version);
        case "agents":
          return deleteRegistryAgentVersion(t.name, t.version);
        case "skills":
          return deleteRegistrySkillVersion(t.name, t.version);
        case "prompts":
          return deleteRegistryPromptVersion(t.name, t.version);
      }
    },
    onSuccess: (_d, vars) => {
      qc.invalidateQueries({ queryKey: [`registry-${vars.kind}`] });
      qc.invalidateQueries({ queryKey: ["registry-detail"] });
      qc.invalidateQueries({ queryKey: ["registry-versions"] });
      toast({ title: "Version deleted" });
      onSuccess();
    },
    onError: (e: any) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });
}

function ImportButton() {
  const qc = useQueryClient();
  const { toast } = useToast();
  const mutation = useMutation({
    mutationFn: () => triggerRegistryImport("mcp"),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["registry-servers"] });
      const total = res.reports.reduce((a, b) => a + b.imported, 0);
      const sources = res.reports
        .map((r) => `${r.source}: ${r.imported}/${r.found}`)
        .join(" · ");
      toast({ title: `Imported ${total} packages`, description: sources });
    },
    onError: (e: any) => {
      toast({
        title: "Import failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });
  return (
    <Button
      variant="outline"
      className="gap-2"
      onClick={() => mutation.mutate()}
      disabled={mutation.isPending}
    >
      <Icon icon={icons.download} className="h-4 w-4" />
      {mutation.isPending ? "Importing…" : "Import"}
    </Button>
  );
}

