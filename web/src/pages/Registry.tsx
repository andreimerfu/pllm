import { useEffect, useMemo, useState } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
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
  getAdminModels,
} from "@/lib/api";
import { frameworkInfo } from "@/lib/frameworks";
import { detectProvider } from "@/lib/providers";
import type {
  AdminModel,
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
import { DeployDialog } from "@/components/agents/deploy-dialog";
import { ArtifactCard } from "@/components/registry/artifact-card";
import { KindIcon } from "@/components/registry/kind-icon";
import { ObservedGraph } from "@/components/registry/observed-graph";
import { PackageChip } from "@/components/registry/package-chip";
import { InstallCommand } from "@/components/registry/install-command";
import { ScoresPanel } from "@/components/registry/score-ribbon";
import { VersionPicker } from "@/components/registry/version-picker";
import { PageShell } from "@/components/common/page-shell";
import { KindChip, SectionLabel, StatusChip } from "@/components/common/status-chip";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Types and helpers
// ---------------------------------------------------------------------------

type Kind = "servers" | "agents" | "skills" | "prompts";
// The tab bar adds two non-registry tabs: a combined "all" view and the
// gateway "models" list. Registry kinds keep their plural keys.
type TabKey = "all" | "agents" | "servers" | "skills" | "prompts" | "models";
type SortMode = "recent" | "alpha" | "score";

// Ordered tab bar — drives the underline nav (label + icon per tab).
const TABS: { key: TabKey; label: string; icon: string }[] = [
  { key: "all", label: "All", icon: icons.layers },
  { key: "agents", label: "Agents", icon: icons.agent },
  { key: "servers", label: "MCP", icon: icons.mcp },
  { key: "skills", label: "Skills", icon: icons.skill },
  { key: "prompts", label: "Prompts", icon: icons.prompt },
  { key: "models", label: "Models", icon: icons.models },
];

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

const TAB_BY_KEY = TABS.reduce(
  (acc, t) => {
    acc[t.key] = t;
    return acc;
  },
  {} as Record<TabKey, { key: TabKey; label: string; icon: string }>,
);

// Map a registry singular kind back to this page's plural tab/query key.
const SINGULAR_TO_KIND: Record<"server" | "agent" | "skill" | "prompt", Kind> = {
  server: "servers",
  agent: "agents",
  skill: "skills",
  prompt: "prompts",
};

function tabTagline(tab: TabKey): string {
  switch (tab) {
    case "all":
      return "Everything in the catalog — agents, MCP servers, skills, prompts, and gateway models.";
    case "models":
      return "Gateway LLMs routed through pllm. Configured on the Models page.";
    default:
      return KIND_META[tab].tagline;
  }
}

type RegistrySingular = "server" | "agent" | "skill" | "prompt";

// One combined grid for the "all" tab: registry rows tagged with their kind,
// interleaved with gateway models.
type AllEntry =
  | { type: "registry"; kind: RegistrySingular; row: AnyRow }
  | { type: "model"; model: AdminModel };

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

// Sidebar deep-links (Skills/Prompts) land here as /registry?kind=<singular>.
// Map the registry-kind query value back to this page's tab key. No param
// falls through to the default "all" tab.
const KIND_PARAM_TO_TAB: Record<string, TabKey> = {
  skill: "skills",
  prompt: "prompts",
  agent: "agents",
  server: "servers",
  mcp: "servers",
  model: "models",
  llm: "models",
};

export default function Registry() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const kindParam = searchParams.get("kind");
  const [tab, setTab] = useState<TabKey>(KIND_PARAM_TO_TAB[kindParam ?? ""] ?? "all");
  const [search, setSearch] = useState("");
  const [latestOnly, setLatestOnly] = useState(true);
  const [sort, setSort] = useState<SortMode>("recent");
  const [pkgFilter, setPkgFilter] = useState<string | null>(null);
  const [detail, setDetail] = useState<
    { kind: Kind; name: string; version?: string } | null
  >(null);
  const [publishOpen, setPublishOpen] = useState(false);
  // "<name>::<version>" of the agent to deploy, set by a card Deploy button →
  // opens the bind-to-use-case drawer preselected. Null = drawer closed.
  const [deployAgentKey, setDeployAgentKey] = useState<string | null>(null);

  // Re-sync the active tab when the sidebar deep-link param changes while
  // we're already mounted (e.g. Skills → Prompts without a full remount).
  useEffect(() => {
    const mapped = KIND_PARAM_TO_TAB[kindParam ?? ""];
    if (mapped) setTab(mapped);
  }, [kindParam]);

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

  // Gateway models — the sixth tab. Sourced from the admin model list, not the
  // registry; no fabricated scores.
  const models = useQuery({
    queryKey: ["admin-models"],
    queryFn: getAdminModels,
  });

  // Deployment state is polled broadly so every card can show live deploy info.
  const deployments = useQuery({
    queryKey: ["deployments"],
    queryFn: listDeployments,
    refetchInterval: 5_000,
  });

  // The registry kind backing the active tab (null for "all"/"models", which
  // are handled explicitly).
  const activeKind: Kind | null =
    tab === "agents" || tab === "servers" || tab === "skills" || tab === "prompts"
      ? tab
      : null;

  const activeItems = useMemo<AnyRow[]>(() => {
    switch (activeKind) {
      case "servers":
        return servers.data?.servers ?? [];
      case "agents":
        return agents.data?.agents ?? [];
      case "skills":
        return skills.data?.skills ?? [];
      case "prompts":
        return prompts.data?.prompts ?? [];
      default:
        return [];
    }
  }, [activeKind, servers.data, agents.data, skills.data, prompts.data]);

  // Shared sort/filter applied to any registry-row array.
  const sortRows = useMemo(
    () =>
      (rows: AnyRow[], forKind: Kind | null) => {
        let items = [...rows];
        // Package-type filter only makes sense for servers.
        if (forKind === "servers" && pkgFilter) {
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
      },
    [pkgFilter, sort],
  );

  const filteredItems = useMemo(
    () => sortRows(activeItems, activeKind),
    [sortRows, activeItems, activeKind],
  );

  // Models filtered by the search term against model_name.
  const filteredModels = useMemo(() => {
    const all = models.data?.models ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return all;
    return all.filter((m) => m.model_name.toLowerCase().includes(term));
  }, [models.data, search]);

  // Combined "all" grid — every registry row tagged with its kind, plus models.
  const allEntries = useMemo<AllEntry[]>(() => {
    const reg: AllEntry[] = [
      ...sortRows(servers.data?.servers ?? [], "servers").map((row) => ({
        type: "registry" as const,
        kind: "server" as const,
        row,
      })),
      ...sortRows(agents.data?.agents ?? [], "agents").map((row) => ({
        type: "registry" as const,
        kind: "agent" as const,
        row,
      })),
      ...sortRows(skills.data?.skills ?? [], "skills").map((row) => ({
        type: "registry" as const,
        kind: "skill" as const,
        row,
      })),
      ...sortRows(prompts.data?.prompts ?? [], "prompts").map((row) => ({
        type: "registry" as const,
        kind: "prompt" as const,
        row,
      })),
    ];
    const mdls: AllEntry[] = filteredModels.map((model) => ({
      type: "model" as const,
      model,
    }));
    return [...reg, ...mdls];
  }, [servers.data, agents.data, skills.data, prompts.data, filteredModels, sortRows]);

  const countAgents = agents.data?.total ?? agents.data?.agents?.length ?? 0;
  const countServers = servers.data?.total ?? servers.data?.servers?.length ?? 0;
  const countSkills = skills.data?.total ?? skills.data?.skills?.length ?? 0;
  const countPrompts = prompts.data?.total ?? prompts.data?.prompts?.length ?? 0;
  const countModels = models.data?.models?.length ?? 0;
  const counts: Record<TabKey, number> = {
    all: countAgents + countServers + countSkills + countPrompts + countModels,
    agents: countAgents,
    servers: countServers,
    skills: countSkills,
    prompts: countPrompts,
    models: countModels,
  };
  // KPI cards are derived entirely from the four already-fetched arrays.
  const allItems = useMemo<AnyRow[]>(
    () => [
      ...(servers.data?.servers ?? []),
      ...(agents.data?.agents ?? []),
      ...(skills.data?.skills ?? []),
      ...(prompts.data?.prompts ?? []),
    ],
    [servers.data, agents.data, skills.data, prompts.data],
  );

  const registryStats = useMemo(() => {
    const total = allItems.length;
    // Third-party = federated from a peer OR pulled in via the npm/PyPI
    // importer namespace; everything else is internal/first-party.
    const isThirdParty = (r: AnyRow) =>
      !!r.source_peer_id || /\.import\//i.test(r.name);
    const vendor = allItems.filter(isThirdParty).length;
    const internal = total - vendor;
    const thirdPartyPct = total === 0 ? 0 : Math.round((vendor / total) * 100);

    // No `verified`/`signed`/score field on list rows — derive trust from
    // the fields we do have: lifecycle status and latest-version coverage.
    const active = allItems.filter((r) => r.status === "active").length;
    const latest = allItems.filter((r) => r.is_latest).length;
    const activePct = total === 0 ? 0 : Math.round((active / total) * 100);
    const latestPct = total === 0 ? 0 : Math.round((latest / total) * 100);

    // "New this week" from published_at, used as the catalog sub when > 0.
    const weekAgo = Date.now() - 7 * 86_400_000;
    const newThisWeek = allItems.filter(
      (r) => r.published_at && new Date(r.published_at).getTime() >= weekAgo,
    ).length;

    return {
      total,
      vendor,
      internal,
      thirdPartyPct,
      active,
      activePct,
      latestPct,
      newThisWeek,
    };
  }, [allItems]);

  // Package types present among current servers (for filter chips).
  const availablePkgTypes = useMemo(() => {
    if (tab !== "servers") return [];
    const set = new Set<string>();
    for (const row of activeItems) {
      for (const p of parsePackages((row as RegistryServer).packages)) {
        if (p.registry_type) set.add(p.registry_type.toLowerCase());
      }
    }
    return Array.from(set).sort();
  }, [activeItems, tab]);

  const activeQueryLoading =
    tab === "all"
      ? servers.isLoading ||
        agents.isLoading ||
        skills.isLoading ||
        prompts.isLoading ||
        models.isLoading
      : tab === "models"
        ? models.isLoading
        : {
            servers: servers.isLoading,
            agents: agents.isLoading,
            skills: skills.isLoading,
            prompts: prompts.isLoading,
          }[tab];

  // Default publish target for the "all" view and the registry tabs. The
  // models tab routes publish actions to the /models page instead.
  const publishKind: Kind = activeKind ?? "agents";

  // Per-card Deploy affordance — agents open the bind-to-use-case drawer,
  // MCP servers deploy as a K8s workload inline. Skills/prompts aren't
  // deployable on their own, so they get no action.
  const cardAction = (kind: RegistryKind, row: AnyRow) => {
    if (kind === "agent")
      return (
        <AgentDeployButton
          onDeploy={() => setDeployAgentKey(`${row.name}::${row.version}`)}
        />
      );
    if (kind === "server")
      return (
        <ServerDeployButton
          row={row as RegistryServer}
          deployments={deployments.data?.deployments}
        />
      );
    return undefined;
  };

  return (
    <PageShell
      title="Registry"
      description="The catalog of everything deployable — agents, MCP servers, skills, prompts, and models. Reviewed, versioned, scored, and ready to ship."
      actions={
        <>
          {tab === "servers" && <ImportButton />}
          {tab === "models" ? (
            <Button onClick={() => navigate("/models")} className="gap-2">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Add model
            </Button>
          ) : (
            <Button onClick={() => setPublishOpen(true)} className="gap-2">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Publish {publishKind.slice(0, -1)}
            </Button>
          )}
        </>
      }
    >
      <PublishDialog
        kind={publishKind}
        open={publishOpen}
        onOpenChange={setPublishOpen}
      />

      {/* ────────────────────────── KPI cards ────────────────────────── */}
      <RegistryKPIs counts={counts} stats={registryStats} />

      {/* ────────────────────────── Toolbar ────────────────────────── */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="relative flex-1">
          <Icon
            icon={icons.search}
            className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-dim"
          />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={`Search ${tab}…`}
            className="h-9 rounded-[9px] pl-9"
          />
        </div>

        <Select value={sort} onValueChange={(v) => setSort(v as SortMode)}>
          <SelectTrigger className="h-9 w-[10rem] rounded-[9px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="recent">Most recent</SelectItem>
            <SelectItem value="alpha">Alphabetical</SelectItem>
            <SelectItem value="score">Score</SelectItem>
          </SelectContent>
        </Select>

        <div className="flex items-center gap-2">
          <Switch
            id="latest-only"
            checked={latestOnly}
            onCheckedChange={setLatestOnly}
            className="data-[state=checked]:bg-good"
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
      {tab === "servers" && availablePkgTypes.length > 0 && (
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
      <Tabs value={tab} onValueChange={(v) => setTab(v as TabKey)}>
        <TabsList className="h-auto w-full justify-start gap-0 rounded-none border-b border-border bg-transparent p-0">
          {TABS.map((t) => (
            <TabsTrigger
              key={t.key}
              value={t.key}
              className="mr-[18px] gap-2 rounded-none border-b-2 border-transparent bg-transparent px-1 py-0 h-10 text-[13.5px] font-medium text-muted-foreground shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:font-semibold data-[state=active]:text-primary data-[state=active]:shadow-none"
            >
              <Icon icon={t.icon} className="h-[15px] w-[15px]" />
              <span>{t.label}</span>
              <span
                className={cn(
                  "rounded-full px-1.5 py-px font-mono text-[10.5px] font-semibold tabular-nums",
                  tab === t.key ? "bg-[var(--accent-soft)] text-primary" : "bg-surface-2 text-text-dim",
                )}
              >
                {counts[t.key]}
              </span>
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value={tab} className="mt-5">
          {/* Tagline rail */}
          <p className="text-sm text-muted-foreground mb-4 flex items-center gap-2">
            <Icon icon={TAB_BY_KEY[tab].icon} className="h-4 w-4" />
            {tabTagline(tab)}
          </p>

          {activeQueryLoading ? (
            <LoadingGrid />
          ) : tab === "all" ? (
            allEntries.length === 0 ? (
              <EmptyState
                icon={icons.registry}
                title="Nothing in the catalog yet."
                description={
                  search
                    ? `No matches for "${search}". Try clearing the filter.`
                    : undefined
                }
              />
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                {allEntries.map((entry) =>
                  entry.type === "model" ? (
                    <ModelCard
                      key={`model-${entry.model.id}`}
                      model={entry.model}
                      onClick={() => navigate("/models")}
                    />
                  ) : (
                    <ArtifactCard
                      key={`${entry.kind}-${entry.row.id}`}
                      kind={entry.kind}
                      row={entry.row}
                      extras={extrasFor(
                        SINGULAR_TO_KIND[entry.kind],
                        entry.row,
                        deployments.data?.deployments,
                      )}
                      action={cardAction(entry.kind, entry.row)}
                      onClick={() =>
                        setDetail({
                          kind: SINGULAR_TO_KIND[entry.kind],
                          name: entry.row.name,
                          version: entry.row.version,
                        })
                      }
                    />
                  ),
                )}
              </div>
            )
          ) : tab === "models" ? (
            filteredModels.length === 0 ? (
              <EmptyState
                icon={icons.models}
                title="No models configured yet."
                description={
                  search
                    ? `No matches for "${search}". Try clearing the filter.`
                    : "Add gateway models on the Models page."
                }
              />
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                {filteredModels.map((model) => (
                  <ModelCard
                    key={model.id}
                    model={model}
                    onClick={() => navigate("/models")}
                  />
                ))}
              </div>
            )
          ) : filteredItems.length === 0 ? (
            <EmptyState
              icon={icons.registry}
              title={activeKind ? KIND_META[activeKind].empty : "Nothing here yet."}
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
                  kind={activeKind ? KIND_META[activeKind].singular : "server"}
                  row={row}
                  extras={
                    activeKind
                      ? extrasFor(activeKind, row, deployments.data?.deployments)
                      : undefined
                  }
                  action={
                    activeKind
                      ? cardAction(KIND_META[activeKind].singular, row)
                      : undefined
                  }
                  onClick={() =>
                    activeKind &&
                    setDetail({ kind: activeKind, name: row.name, version: row.version })
                  }
                />
              ))}
            </div>
          )}
        </TabsContent>
      </Tabs>

      <DetailDrawer
        target={detail}
        onClose={() => setDetail(null)}
        deployments={deployments.data?.deployments ?? []}
      />

      <DeployDialog
        open={!!deployAgentKey}
        presetAgentKey={deployAgentKey ?? undefined}
        onClose={() => setDeployAgentKey(null)}
      />
    </PageShell>
  );
}

// ---------------------------------------------------------------------------
// KPI cards — three framed panels derived entirely from the fetched arrays
// ---------------------------------------------------------------------------

type RegistryStats = {
  total: number;
  vendor: number;
  internal: number;
  thirdPartyPct: number;
  active: number;
  activePct: number;
  latestPct: number;
  newThisWeek: number;
};

function RegistryKPIs({
  counts,
  stats,
}: {
  counts: Record<TabKey, number>;
  stats: RegistryStats;
}) {
  const legend: { kind: RegistryKind; label: string; count: number }[] = [
    { kind: "agent", label: "Agents", count: counts.agents },
    { kind: "server", label: "MCP Servers", count: counts.servers },
    { kind: "skill", label: "Skills", count: counts.skills },
    { kind: "prompt", label: "Prompts", count: counts.prompts },
  ];

  const internalPct =
    stats.total === 0 ? 0 : Math.round((stats.internal / stats.total) * 100);

  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
      {/* ── CATALOG ── */}
      <div className="rounded-[14px] border border-border bg-card p-5 shadow-sm">
        <SectionLabel>Catalog</SectionLabel>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="font-mono text-[30px] font-semibold leading-none tracking-[-0.02em] tabular-nums">
            {stats.total}
          </span>
          <span className="text-[12px] text-text-dim">
            assets
            {stats.newThisWeek > 0 && (
              <>
                {" · "}
                <span className="text-good">+{stats.newThisWeek}</span> this week
              </>
            )}
          </span>
        </div>
        <div className="mt-4 flex flex-wrap gap-x-4 gap-y-2">
          {legend.map((l) => (
            <span key={l.kind} className="inline-flex items-center gap-1.5 text-[12px]">
              <span
                className={cn(
                  "h-2 w-2 shrink-0 rounded-full",
                  KIND_DOT[l.kind],
                )}
              />
              <span className="text-muted-foreground">{l.label}</span>
              <span className="font-mono tabular-nums text-foreground">
                {l.count}
              </span>
            </span>
          ))}
        </div>
      </div>

      {/* ── SUPPLY CHAIN ── */}
      <div className="rounded-[14px] border border-border bg-card p-5 shadow-sm">
        <SectionLabel>Supply chain</SectionLabel>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="font-mono text-[30px] font-semibold leading-none tracking-[-0.02em] tabular-nums">
            {stats.thirdPartyPct}%
          </span>
          <span className="text-[12px] text-text-dim">third-party</span>
        </div>
        <div className="mt-4 flex h-2.5 w-full overflow-hidden rounded-full bg-surface-2">
          <div className="bg-good" style={{ width: `${internalPct}%` }} />
          <div
            className="bg-kind-skill"
            style={{ width: `${stats.thirdPartyPct}%` }}
          />
        </div>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-[12px] text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-good" />
            Internal
            <span className="font-mono tabular-nums text-foreground">
              {stats.internal}
            </span>
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-kind-skill" />
            Vendor
            <span className="font-mono tabular-nums text-foreground">
              {stats.vendor}
            </span>
          </span>
        </div>
      </div>

      {/* ── TRUST & SECURITY ── */}
      <div className="rounded-[14px] border border-border bg-card p-5 shadow-sm">
        <SectionLabel>Trust &amp; security</SectionLabel>
        <div className="mt-3 flex items-baseline gap-2">
          <span className="font-mono text-[30px] font-semibold leading-none tracking-[-0.02em] tabular-nums">
            {stats.activePct}
          </span>
          <span className="text-[12px] text-text-dim">/ 100 active</span>
        </div>
        <div className="mt-4 space-y-2.5">
          <MiniBar label="Active" pct={stats.activePct} tone="bg-good" />
          <MiniBar label="Latest" pct={stats.latestPct} tone="bg-info" />
        </div>
      </div>
    </div>
  );
}

const KIND_DOT: Record<RegistryKind, string> = {
  agent: "bg-kind-agent",
  server: "bg-kind-mcp",
  skill: "bg-kind-skill",
  prompt: "bg-kind-prompt",
  llm: "bg-kind-model",
  use_case: "bg-primary",
};

function MiniBar({
  label,
  pct,
  tone,
}: {
  label: string;
  pct: number;
  tone: string;
}) {
  return (
    <div>
      <div className="mb-1 flex items-center justify-between text-[12px]">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-mono tabular-nums text-foreground">{pct}%</span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface-2">
        <div className={cn("h-full rounded-full", tone)} style={{ width: `${pct}%` }} />
      </div>
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
// Model card — gateway LLMs (the "models" tab / "all" interleave). Styled to
// match ArtifactCard. No fabricated scores: only what the model row carries.
// ---------------------------------------------------------------------------

function ModelCard({
  model,
  onClick,
}: {
  model: AdminModel;
  onClick?: () => void;
}) {
  const provider = detectProvider(model.model_name, model.provider?.type ?? "");
  return (
    <button
      type="button"
      onClick={onClick}
      className="group flex w-full flex-col gap-3 rounded-[14px] border border-border bg-card p-4 text-left shadow-sm transition hover:border-primary/40 hover:shadow-md"
    >
      <div className="flex items-start gap-3">
        <KindIcon kind="llm" name={model.model_name} size="md" />
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold leading-tight">
            {model.model_name}
          </p>
          <p className="truncate text-[12px] text-text-dim">{provider.name}</p>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        <KindChip kind="llm" label="Model" />
        <StatusChip
          tone={model.enabled ? "good" : "neutral"}
          label={model.enabled ? "Enabled" : "Disabled"}
        />
      </div>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Per-card Deploy buttons (Catalog). Agents open the bind-to-use-case drawer;
// MCP servers deploy as a dedicated K8s workload inline. Both stopPropagation
// so the click doesn't also open the card's detail drawer.
// ---------------------------------------------------------------------------

function AgentDeployButton({ onDeploy }: { onDeploy: () => void }) {
  return (
    <Button
      size="sm"
      variant="outline"
      className="h-7 gap-1.5 px-2.5 text-[11px]"
      onClick={(e) => {
        e.stopPropagation();
        onDeploy();
      }}
    >
      <Icon icon={icons.rocket} className="h-3 w-3" />
      Deploy
    </Button>
  );
}

function ServerDeployButton({
  row,
  deployments,
}: {
  row: RegistryServer;
  deployments?: Deployment[];
}) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const dep = deploymentFor(deployments, row.name, row.version);

  const deployMutation = useMutation({
    mutationFn: () =>
      deployRegistryServer({ server_name: row.name, server_version: row.version }),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({
        title: dep ? "Re-deploy started" : "Deploy started",
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

  const busy = deployMutation.isPending || deployInProgress(dep?.status);
  const label = deployMutation.isPending
    ? "Deploying…"
    : deployInProgress(dep?.status)
      ? "Deploying…"
      : dep?.status === "running"
        ? "Re-deploy"
        : dep?.status === "failed"
          ? "Retry"
          : "Deploy";

  return (
    <Button
      size="sm"
      variant="outline"
      className="h-7 gap-1.5 px-2.5 text-[11px]"
      disabled={busy}
      onClick={(e) => {
        e.stopPropagation();
        deployMutation.mutate();
      }}
    >
      <Icon icon={icons.rocket} className="h-3 w-3" />
      {label}
    </Button>
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
      const deployCount = (deployments ?? []).filter(
        (d) =>
          d.resource_name === s.name &&
          d.status !== "stopped" &&
          d.status !== "terminating",
      ).length;
      return {
        packages: parsePackages(s.packages),
        deployStatus: dep?.status,
        deployCount,
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
              {target?.kind === "agents" && detailQuery.data
                ? (() => {
                    const fw = frameworkInfo(
                      (detailQuery.data.row as RegistryAgentWithRefs).agent.framework,
                    );
                    return fw ? (
                      <span
                        className="mt-2 inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-medium text-muted-foreground"
                        title={`Framework: ${fw.label}`}
                      >
                        <Icon
                          icon={fw.icon}
                          className={`h-3.5 w-3.5 ${fw.tone ?? ""}`}
                        />
                        {fw.label}
                      </span>
                    ) : null;
                  })()
                : null}
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
        <DepSummary count={data.servers?.length ?? 0} label="MCP Servers" kind="server" />
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
  agent: "bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-200",
  http: "bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-200",
  unknown: "bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-200",
};

const OBSERVED_OUTCOME_TONE: Record<string, string> = {
  allowed: "text-emerald-700 dark:text-emerald-400",
  flagged: "text-amber-700 dark:text-amber-400",
  blocked: "text-rose-700 dark:text-rose-400",
  denied: "text-rose-700 dark:text-rose-400",
};

const OBSERVED_KIND_FALLBACK: Record<string, string> = {
  llm: "solar:cpu-bolt-linear",
  mcp: "lucide:plug-zap",
  vector_db: "solar:database-linear",
  agent: "solar:users-group-rounded-linear",
  http: "solar:global-linear",
  unknown: "solar:question-circle-linear",
};

function vectorDbIconForObserved(target: string): string {
  const t = target.toLowerCase();
  if (t.startsWith("qdrant")) return "logos:qdrant-icon";
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
    case "agent":
      return { icon: OBSERVED_KIND_FALLBACK.agent, label: "agent (A2A)" };
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
        <>
          <ObservedGraph
            agentName={name}
            calls={calls}
            framework={data.framework}
          />
          <div className="mt-3 space-y-1.5">
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
        </>
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

