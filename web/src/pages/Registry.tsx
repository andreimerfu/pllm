import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

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
} from "@/lib/api";
import type {
  EnrichmentScore,
  RegistryAgentWithRefs,
  RegistryPrompt,
  RegistryServer,
  RegistryServerDetail,
  RegistrySkill,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { EmptyState } from "@/components/common/EmptyState";
import { PublishDialog } from "@/components/registry/publish-dialog";
import { useToast } from "@/hooks/use-toast";

type Kind = "servers" | "agents" | "skills" | "prompts";

const kindMeta: Record<Kind, { label: string; icon: string; tagline: string }> = {
  servers: {
    label: "MCP Servers",
    icon: icons.mcp,
    tagline: "Cataloged MCP servers — npx, uvx, Docker, remote endpoints.",
  },
  agents: {
    label: "Agents",
    icon: icons.agent,
    tagline: "Agent manifests with MCP server / skill / prompt dependencies.",
  },
  skills: {
    label: "Skills",
    icon: icons.skill,
    tagline: "Knowledge bundles (SKILL.md + assets) pulled from OCI images.",
  },
  prompts: {
    label: "Prompts",
    icon: icons.prompt,
    tagline: "Reusable prompt templates with argument schemas.",
  },
};

export default function Registry() {
  const [kind, setKind] = useState<Kind>("servers");
  const [search, setSearch] = useState("");
  const [latestOnly, setLatestOnly] = useState(true);
  const [detail, setDetail] = useState<{ kind: Kind; name: string; version?: string } | null>(null);
  const [publishOpen, setPublishOpen] = useState(false);

  const query = { search: search || undefined, latest: latestOnly };

  const servers = useQuery({
    queryKey: ["registry-servers", query],
    queryFn: () => listRegistryServers(query),
    enabled: kind === "servers",
  });
  const agents = useQuery({
    queryKey: ["registry-agents", query],
    queryFn: () => listRegistryAgents(query),
    enabled: kind === "agents",
  });
  const skills = useQuery({
    queryKey: ["registry-skills", query],
    queryFn: () => listRegistrySkills(query),
    enabled: kind === "skills",
  });
  const prompts = useQuery({
    queryKey: ["registry-prompts", query],
    queryFn: () => listRegistryPrompts(query),
    enabled: kind === "prompts",
  });

  return (
    <div className="space-y-4 lg:space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">Registry</h1>
          <p className="text-sm lg:text-base text-muted-foreground">
            Curated catalog of MCP servers, agents, skills, and prompts.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {kind === "servers" && <ImportButton />}
          <Button onClick={() => setPublishOpen(true)} className="gap-2">
            <Icon icon={icons.plus} className="h-4 w-4" />
            Publish {kind.slice(0, -1)}
          </Button>
        </div>
      </div>
      <PublishDialog kind={kind} open={publishOpen} onOpenChange={setPublishOpen} />

      <div className="flex flex-col sm:flex-row gap-3 items-start sm:items-center">
        <div className="relative flex-1 w-full sm:max-w-sm">
          <Icon
            icon={icons.search}
            className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground"
          />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or description"
            className="pl-9"
          />
        </div>
        <div className="flex items-center gap-2">
          <Switch id="latest-only" checked={latestOnly} onCheckedChange={setLatestOnly} />
          <Label htmlFor="latest-only" className="cursor-pointer text-sm">
            Latest versions only
          </Label>
        </div>
      </div>

      <Tabs value={kind} onValueChange={(v) => setKind(v as Kind)}>
        <TabsList className="grid grid-cols-4 w-full max-w-xl">
          <TabsTrigger value="servers" className="gap-2">
            <Icon icon={icons.mcp} className="h-4 w-4" /> Servers
          </TabsTrigger>
          <TabsTrigger value="agents" className="gap-2">
            <Icon icon={icons.agent} className="h-4 w-4" /> Agents
          </TabsTrigger>
          <TabsTrigger value="skills" className="gap-2">
            <Icon icon={icons.skill} className="h-4 w-4" /> Skills
          </TabsTrigger>
          <TabsTrigger value="prompts" className="gap-2">
            <Icon icon={icons.prompt} className="h-4 w-4" /> Prompts
          </TabsTrigger>
        </TabsList>

        <TabsContent value="servers" className="mt-4">
          <KindHeader kind="servers" />
          <ListGrid
            loading={servers.isLoading}
            items={servers.data?.servers ?? []}
            empty="No MCP servers published yet."
            onOpen={(row) => setDetail({ kind: "servers", name: row.name, version: row.version })}
            render={(row) => <BaseCard row={row} />}
          />
        </TabsContent>

        <TabsContent value="agents" className="mt-4">
          <KindHeader kind="agents" />
          <ListGrid
            loading={agents.isLoading}
            items={agents.data?.agents ?? []}
            empty="No agents published yet."
            onOpen={(row) => setDetail({ kind: "agents", name: row.name, version: row.version })}
            render={(row) => (
              <BaseCard
                row={row}
                sub={[row.language, row.framework, row.model_provider].filter(Boolean).join(" · ") || undefined}
              />
            )}
          />
        </TabsContent>

        <TabsContent value="skills" className="mt-4">
          <KindHeader kind="skills" />
          <ListGrid
            loading={skills.isLoading}
            items={skills.data?.skills ?? []}
            empty="No skills published yet."
            onOpen={(row) => setDetail({ kind: "skills", name: row.name, version: row.version })}
            render={(row) => <BaseCard row={row} sub={row.image} />}
          />
        </TabsContent>

        <TabsContent value="prompts" className="mt-4">
          <KindHeader kind="prompts" />
          <ListGrid
            loading={prompts.isLoading}
            items={prompts.data?.prompts ?? []}
            empty="No prompts published yet."
            onOpen={(row) => setDetail({ kind: "prompts", name: row.name, version: row.version })}
            render={(row) => (
              <BaseCard
                row={row}
                sub={row.template ? row.template.slice(0, 80) + (row.template.length > 80 ? "…" : "") : undefined}
              />
            )}
          />
        </TabsContent>
      </Tabs>

      <DetailDrawer target={detail} onClose={() => setDetail(null)} />
    </div>
  );
}

function DeployButton({ name, version }: { name: string; version: string }) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const mutation = useMutation({
    mutationFn: () => deployRegistryServer({ server_name: name, server_version: version }),
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: ["deployments"] });
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({
        title: "Deploy started",
        description: `workload: ${d.workload_name} · status: ${d.status}`,
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
  return (
    <Button
      variant="outline"
      size="sm"
      onClick={() => mutation.mutate()}
      disabled={mutation.isPending}
    >
      <Icon icon={icons.rocket} className="h-4 w-4 mr-1" />
      {mutation.isPending ? "Deploying…" : "Deploy"}
    </Button>
  );
}

function ImportButton() {
  const qc = useQueryClient();
  const { toast } = useToast();
  const mutation = useMutation({
    mutationFn: () => triggerRegistryImport("mcp"),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["registry-servers"] });
      const total = res.reports.reduce((a, b) => a + b.imported, 0);
      const sources = res.reports.map((r) => `${r.source}: ${r.imported}/${r.found}`).join(" · ");
      toast({
        title: `Imported ${total} packages`,
        description: sources,
      });
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
      {mutation.isPending ? "Importing…" : "Import from npm / PyPI"}
    </Button>
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

function KindHeader({ kind }: { kind: Kind }) {
  const m = kindMeta[kind];
  return (
    <p className="text-sm text-muted-foreground mb-3">
      <Icon icon={m.icon} className="inline h-4 w-4 mr-1 align-text-bottom" /> {m.tagline}
    </p>
  );
}

function ListGrid<T extends { id: string; name: string; version: string }>({
  loading,
  items,
  empty,
  onOpen,
  render,
}: {
  loading: boolean;
  items: T[];
  empty: string;
  onOpen: (row: T) => void;
  render: (row: T) => React.ReactNode;
}) {
  if (loading) {
    return (
      <div className="flex items-center justify-center h-48">
        <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
      </div>
    );
  }
  if (items.length === 0) {
    return <EmptyState icon={icons.registry} title={empty} />;
  }
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {items.map((row) => (
        <button
          key={row.id}
          onClick={() => onOpen(row)}
          className="text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded-lg"
        >
          {render(row)}
        </button>
      ))}
    </div>
  );
}

function BaseCard({
  row,
  sub,
}: {
  row: {
    name: string;
    version: string;
    title?: string;
    description?: string;
    is_latest: boolean;
    status: string;
    published_at?: string;
  };
  sub?: string;
}) {
  return (
    <Card className="h-full hover:shadow-md transition-shadow hover:border-primary/30">
      <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2 gap-3">
        <div className="space-y-1 min-w-0">
          <CardTitle className="text-base font-semibold truncate">
            {row.title || row.name}
          </CardTitle>
          <code className="text-xs font-mono text-muted-foreground bg-muted px-1.5 py-0.5 rounded truncate block max-w-full">
            {row.name}
          </code>
        </div>
        <div className="flex flex-col items-end gap-1">
          <Badge variant="secondary" className="font-mono text-[10px]">
            v{row.version}
          </Badge>
          {row.is_latest && (
            <Badge variant="default" className="text-[10px]">
              latest
            </Badge>
          )}
          {row.status === "deprecated" && (
            <Badge variant="outline" className="text-[10px] text-amber-600">
              deprecated
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="space-y-2">
        {row.description && (
          <p className="text-sm text-muted-foreground line-clamp-3">{row.description}</p>
        )}
        {sub && (
          <p className="text-xs font-mono text-muted-foreground line-clamp-1">{sub}</p>
        )}
        {row.published_at && (
          <p className="text-[11px] text-muted-foreground">
            Published {new Date(row.published_at).toLocaleDateString()}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

function DetailDrawer({
  target,
  onClose,
}: {
  target: { kind: Kind; name: string; version?: string } | null;
  onClose: () => void;
}) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const deleteMutation = useDeleteMutation(() => {
    setConfirmDelete(false);
    onClose();
  });

  const { data, isLoading } = useQuery({
    queryKey: ["registry-detail", target],
    queryFn: async () => {
      if (!target) return null;
      switch (target.kind) {
        case "servers":
          return { kind: target.kind, row: await getRegistryServer(target.name, target.version) } as const;
        case "agents":
          return { kind: target.kind, row: await getRegistryAgent(target.name, target.version) } as const;
        case "skills":
          return { kind: target.kind, row: await getRegistrySkill(target.name, target.version) } as const;
        case "prompts":
          return { kind: target.kind, row: await getRegistryPrompt(target.name, target.version) } as const;
      }
    },
    enabled: !!target,
  });

  // The server endpoint returns { server, scores } — every other kind
  // returns the row directly. Unwrap here so downstream renderers don't
  // have to branch.
  const serverEnvelope =
    target?.kind === "servers" && data
      ? (data.row as RegistryServerDetail)
      : null;
  const resolvedRow =
    serverEnvelope ? serverEnvelope.server : data?.row;
  const scores: EnrichmentScore[] = serverEnvelope?.scores ?? [];

  // Resolve the concrete version from the fetched detail (covers the
  // "latest" case where target.version is undefined).
  const resolvedVersion =
    target && data
      ? data.kind === "agents"
        ? (data.row as RegistryAgentWithRefs).agent.version
        : (resolvedRow as RegistryServer | RegistrySkill | RegistryPrompt).version
      : target?.version;

  return (
    <Sheet open={!!target} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-xl overflow-y-auto">
        <SheetHeader>
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <SheetTitle className="truncate">{target?.name}</SheetTitle>
              <SheetDescription>
                {target ? kindMeta[target.kind].label : ""} · v{resolvedVersion}
              </SheetDescription>
            </div>
            {target && resolvedVersion && (
              <div className="flex gap-2 shrink-0">
                {target.kind === "servers" && (
                  <DeployButton name={target.name} version={resolvedVersion} />
                )}
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setConfirmDelete(true)}
                  className="text-destructive hover:text-destructive"
                >
                  <Icon icon={icons.delete} className="h-4 w-4 mr-1" />
                  Delete
                </Button>
              </div>
            )}
          </div>
        </SheetHeader>

        {isLoading || !data ? (
          <div className="flex items-center justify-center h-48">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary" />
          </div>
        ) : data.kind === "agents" ? (
          <AgentDetail data={data.row as RegistryAgentWithRefs} />
        ) : (
          <>
            {scores.length > 0 && <ScoresPanel scores={scores} />}
            <RowDetail
              row={resolvedRow as RegistryServer | RegistrySkill | RegistryPrompt}
              kind={data.kind}
            />
          </>
        )}

        <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete version {resolvedVersion}?</AlertDialogTitle>
              <AlertDialogDescription>
                <span className="font-mono">{target?.name}</span> at version{" "}
                <span className="font-mono">{resolvedVersion}</span> will be marked deleted. If this
                was the latest, the next-newest active version takes over.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                onClick={() =>
                  target &&
                  resolvedVersion &&
                  deleteMutation.mutate({
                    kind: target.kind,
                    name: target.name,
                    version: resolvedVersion,
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

function ScoresPanel({ scores }: { scores: EnrichmentScore[] }) {
  return (
    <div className="mt-4 space-y-2">
      <p className="text-xs font-medium">Enrichment</p>
      <div className="grid gap-2">
        {scores.map((s) => (
          <div key={s.id} className="flex items-center justify-between p-2 bg-muted rounded">
            <div className="flex items-center gap-2 min-w-0">
              <Badge variant="outline" className="text-[10px] font-mono uppercase">
                {s.type}
              </Badge>
              <span className="text-xs text-muted-foreground truncate">{s.summary}</span>
            </div>
            <span
              className={
                "text-sm font-semibold shrink-0 ml-2 " +
                (s.score >= 80
                  ? "text-green-600 dark:text-green-400"
                  : s.score >= 50
                    ? "text-amber-600 dark:text-amber-400"
                    : "text-destructive")
              }
            >
              {s.score.toFixed(0)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function RowDetail({
  row,
  kind,
}: {
  row: RegistryServer | RegistrySkill | RegistryPrompt;
  kind: Kind;
}) {
  return (
    <div className="space-y-4 mt-4">
      {row.description && <p className="text-sm">{row.description}</p>}
      <div className="flex flex-wrap gap-2">
        <Badge variant="secondary">{row.status}</Badge>
        {row.is_latest && <Badge>latest</Badge>}
      </div>
      {kind === "prompts" && (row as RegistryPrompt).template && (
        <pre className="text-xs bg-muted p-3 rounded overflow-x-auto max-h-64 whitespace-pre-wrap">
          {(row as RegistryPrompt).template}
        </pre>
      )}
      {kind === "skills" && (row as RegistrySkill).image && (
        <div>
          <p className="text-xs font-medium mb-1">OCI Image</p>
          <code className="text-xs font-mono bg-muted p-2 rounded block break-all">
            {(row as RegistrySkill).image}
          </code>
        </div>
      )}
      {kind === "servers" && (row as RegistryServer).packages ? (
        <JSONBlock label="Packages" value={(row as RegistryServer).packages} />
      ) : null}
      {kind === "servers" && (row as RegistryServer).remotes ? (
        <JSONBlock label="Remotes" value={(row as RegistryServer).remotes} />
      ) : null}
      {row.metadata ? <JSONBlock label="Metadata" value={row.metadata} /> : null}
    </div>
  );
}

function AgentDetail({ data }: { data: RegistryAgentWithRefs }) {
  return (
    <div className="space-y-4 mt-4">
      {data.agent.description && <p className="text-sm">{data.agent.description}</p>}
      <div className="flex flex-wrap gap-2 text-xs">
        {data.agent.language && <Badge variant="secondary">{data.agent.language}</Badge>}
        {data.agent.framework && <Badge variant="secondary">{data.agent.framework}</Badge>}
        {data.agent.model_provider && <Badge variant="secondary">{data.agent.model_provider}</Badge>}
        {data.agent.model_name && <Badge variant="outline">{data.agent.model_name}</Badge>}
      </div>
      <RefList title="MCP servers" icon={icons.mcp} refs={data.servers ?? []} />
      <RefList title="Skills" icon={icons.skill} refs={data.skills ?? []} />
      <RefList title="Prompts" icon={icons.prompt} refs={data.prompts ?? []} />
    </div>
  );
}

function RefList({
  title,
  icon,
  refs,
}: {
  title: string;
  icon: string;
  refs: { target_name: string; target_version?: string; local_name?: string }[];
}) {
  if (!refs.length) return null;
  return (
    <div>
      <p className="text-xs font-medium mb-2 flex items-center gap-1">
        <Icon icon={icon} className="h-3.5 w-3.5" />
        {title} ({refs.length})
      </p>
      <div className="space-y-1">
        {refs.map((r, i) => (
          <div
            key={i}
            className="text-xs font-mono bg-muted px-2 py-1 rounded flex items-center justify-between"
          >
            <span className="truncate">{r.target_name}</span>
            <span className="text-muted-foreground shrink-0 ml-2">
              {r.target_version || "latest"}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function JSONBlock({ label, value }: { label: string; value: unknown }) {
  return (
    <div>
      <p className="text-xs font-medium mb-1">{label}</p>
      <pre className="text-xs bg-muted p-2 rounded overflow-x-auto max-h-48">
        {JSON.stringify(value, null, 2)}
      </pre>
    </div>
  );
}
