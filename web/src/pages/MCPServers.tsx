import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

import {
  getMCPServers,
  createMCPServer,
  updateMCPServer,
  deleteMCPServer,
  probeMCPServer,
} from "@/lib/api";
import type {
  MCPHealth,
  MCPServer,
  MCPTransport,
  MCPUpsertRequest,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { EmptyState } from "@/components/common/EmptyState";
import { useToast } from "@/hooks/use-toast";

const transportLabels: Record<MCPTransport, string> = {
  stdio: "stdio",
  sse: "SSE",
  http: "HTTP",
};

const healthColors: Record<MCPHealth, string> = {
  healthy: "bg-green-500/15 text-green-700 dark:text-green-400 border-green-500/30",
  unhealthy: "bg-red-500/15 text-red-700 dark:text-red-400 border-red-500/30",
  unknown: "bg-muted text-muted-foreground",
};

type FormState = {
  id?: string;
  name: string;
  slug: string;
  description: string;
  enabled: boolean;
  transport: MCPTransport;
  endpoint: string;
  command: string;
  argsText: string;
  workingDir: string;
  envText: string;
  headersText: string;
};

const emptyForm: FormState = {
  name: "",
  slug: "",
  description: "",
  enabled: true,
  transport: "stdio",
  endpoint: "",
  command: "",
  argsText: "",
  workingDir: "",
  envText: "",
  headersText: "",
};

function toFormState(s: MCPServer): FormState {
  const env = s.env ? Object.entries(s.env).map(([k, v]) => `${k}=${v}`).join("\n") : "";
  const headers = s.headers ? Object.entries(s.headers).map(([k, v]) => `${k}: ${v}`).join("\n") : "";
  return {
    id: s.id,
    name: s.name,
    slug: s.slug,
    description: s.description ?? "",
    enabled: s.enabled,
    transport: s.transport,
    endpoint: s.endpoint ?? "",
    command: s.command ?? "",
    argsText: (s.args ?? []).join(" "),
    workingDir: s.working_dir ?? "",
    envText: env,
    headersText: headers,
  };
}

function parseKVLines(src: string, sep: string): Record<string, string> | undefined {
  const out: Record<string, string> = {};
  let any = false;
  for (const raw of src.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const idx = line.indexOf(sep);
    if (idx <= 0) continue;
    out[line.slice(0, idx).trim()] = line.slice(idx + sep.length).trim();
    any = true;
  }
  return any ? out : undefined;
}

function parseArgs(src: string): string[] {
  // naive shell-ish split: supports quoted segments
  const tokens: string[] = [];
  let cur = "";
  let quote: string | null = null;
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (quote) {
      if (ch === quote) quote = null;
      else cur += ch;
    } else if (ch === "'" || ch === '"') {
      quote = ch;
    } else if (/\s/.test(ch)) {
      if (cur) {
        tokens.push(cur);
        cur = "";
      }
    } else {
      cur += ch;
    }
  }
  if (cur) tokens.push(cur);
  return tokens;
}

function toUpsertRequest(form: FormState): MCPUpsertRequest | { error: string } {
  if (!form.name.trim() || !form.slug.trim()) {
    return { error: "Name and slug are required" };
  }
  if (!/^[a-z0-9][a-z0-9_-]*$/.test(form.slug)) {
    return { error: "Slug must be lowercase letters, digits, hyphen or underscore" };
  }
  if (form.transport === "stdio" && !form.command.trim()) {
    return { error: "Command is required for stdio transport" };
  }
  if ((form.transport === "http" || form.transport === "sse") && !form.endpoint.trim()) {
    return { error: "Endpoint URL is required for http/sse transport" };
  }
  const req: MCPUpsertRequest = {
    name: form.name.trim(),
    slug: form.slug.trim(),
    description: form.description.trim() || undefined,
    enabled: form.enabled,
    transport: form.transport,
  };
  if (form.transport === "stdio") {
    req.command = form.command.trim();
    const args = parseArgs(form.argsText);
    if (args.length) req.args = args;
    if (form.workingDir.trim()) req.working_dir = form.workingDir.trim();
    const env = parseKVLines(form.envText, "=");
    if (env) req.env = env;
  } else {
    req.endpoint = form.endpoint.trim();
    const headers = parseKVLines(form.headersText, ":");
    if (headers) req.headers = headers;
  }
  return req;
}

export default function MCPServers() {
  const queryClient = useQueryClient();
  const { toast } = useToast();

  const [form, setForm] = useState<FormState | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<MCPServer | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["mcp-servers"],
    queryFn: getMCPServers,
    refetchInterval: 10_000,
  });

  const servers = (data ?? []) as MCPServer[];

  const createMutation = useMutation({
    mutationFn: (body: MCPUpsertRequest) => createMCPServer(body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      setForm(null);
      toast({ title: "MCP server created" });
    },
    onError: (e: any) => {
      toast({
        title: "Create failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, body }: { id: string; body: MCPUpsertRequest }) =>
      updateMCPServer(id, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      setForm(null);
      toast({ title: "MCP server updated" });
    },
    onError: (e: any) => {
      toast({
        title: "Update failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteMCPServer(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      setDeleteTarget(null);
      toast({ title: "MCP server deleted" });
    },
    onError: (e: any) => {
      toast({
        title: "Delete failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const probeMutation = useMutation({
    mutationFn: (id: string) => probeMCPServer(id),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({
        title: `Probe: ${res.health}`,
        description:
          res.error ?? res.reason ?? (res.tool_count != null ? `${res.tool_count} tools cached` : undefined),
        variant: res.health === "healthy" ? "default" : "destructive",
      });
    },
    onError: (e: any) => {
      toast({ title: "Probe failed", description: e.message, variant: "destructive" });
    },
  });

  const handleSubmit = () => {
    if (!form) return;
    const built = toUpsertRequest(form);
    if ("error" in built) {
      toast({ title: built.error, variant: "destructive" });
      return;
    }
    if (form.id) {
      updateMutation.mutate({ id: form.id, body: built });
    } else {
      createMutation.mutate(built);
    }
  };

  const healthy = useMemo(
    () => servers.filter((s) => (s.live_health ?? s.health_status) === "healthy").length,
    [servers],
  );
  const totalTools = useMemo(
    () => servers.reduce((n, s) => n + (s.tool_count ?? 0), 0),
    [servers],
  );

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
      </div>
    );
  }

  return (
    <div className="space-y-4 lg:space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">MCP Servers</h1>
          <p className="text-sm lg:text-base text-muted-foreground">
            Backends proxied through <code className="text-xs bg-muted px-1 rounded">/v1/mcp</code>.
            Tools are aggregated as <code className="text-xs bg-muted px-1 rounded">&lt;slug&gt;__&lt;tool&gt;</code>.
          </p>
        </div>
        <Button onClick={() => setForm({ ...emptyForm })} className="gap-2">
          <Icon icon={icons.plus} className="h-4 w-4" />
          Add MCP Server
        </Button>
      </div>

      {servers.length > 0 && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <StatCard label="Registered" value={servers.length} />
          <StatCard label="Healthy" value={healthy} tone={healthy === servers.length ? "ok" : "warn"} />
          <StatCard label="Aggregated tools" value={totalTools} />
          <StatCard label="Enabled" value={servers.filter((s) => s.enabled).length} />
        </div>
      )}

      {servers.length === 0 ? (
        <EmptyState
          icon={icons.mcp}
          title="No MCP servers yet"
          description="Register a backend — npx, uvx, Docker, or a remote HTTP/SSE MCP server — and it will appear in /v1/mcp."
          action={
            <Button onClick={() => setForm({ ...emptyForm })} variant="outline" className="gap-2">
              <Icon icon={icons.plus} className="h-4 w-4" />
              Add first server
            </Button>
          }
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {servers.map((s) => {
            const health = (s.live_health ?? s.health_status) as MCPHealth;
            return (
              <Card key={s.id}>
                <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                  <div className="space-y-1 min-w-0">
                    <CardTitle className="text-base font-semibold truncate">{s.name}</CardTitle>
                    <code className="text-xs font-mono text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                      {s.slug}
                    </code>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge className={healthColors[health]} variant="outline">
                      {health}
                    </Badge>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8">
                          <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => setForm(toFormState(s))}>
                          <Icon icon={icons.edit} className="mr-2 h-4 w-4" />
                          Edit
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => probeMutation.mutate(s.id)}>
                          <Icon icon={icons.refresh} className="mr-2 h-4 w-4" />
                          Probe health
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          className="text-destructive"
                          onClick={() => setDeleteTarget(s)}
                        >
                          <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  {s.description && (
                    <p className="text-sm text-muted-foreground line-clamp-2">{s.description}</p>
                  )}
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <Badge variant="secondary">{transportLabels[s.transport]}</Badge>
                    <Badge variant={s.enabled ? "default" : "outline"}>
                      {s.enabled ? "enabled" : "disabled"}
                    </Badge>
                    {s.tool_count != null && (
                      <span className="text-muted-foreground">{s.tool_count} tools</span>
                    )}
                  </div>
                  <div className="text-xs font-mono text-muted-foreground truncate">
                    {s.transport === "stdio"
                      ? `${s.command ?? ""} ${(s.args ?? []).join(" ")}`.trim()
                      : s.endpoint}
                  </div>
                  {s.last_error && (
                    <p className="text-xs text-destructive truncate">{s.last_error}</p>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      {/* Upsert dialog */}
      <Dialog open={!!form} onOpenChange={(open) => !open && setForm(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{form?.id ? "Edit MCP Server" : "Add MCP Server"}</DialogTitle>
            <DialogDescription>
              Backend is restarted when transport details change. Stdio processes are spawned by the
              gateway; HTTP/SSE backends connect lazily.
            </DialogDescription>
          </DialogHeader>
          {form && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2 md:col-span-1">
                <Label htmlFor="mcp-name">Name</Label>
                <Input
                  id="mcp-name"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="Filesystem"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="mcp-slug">Slug</Label>
                <Input
                  id="mcp-slug"
                  value={form.slug}
                  disabled={!!form.id}
                  onChange={(e) => setForm({ ...form, slug: e.target.value })}
                  placeholder="fs"
                />
              </div>
              <div className="space-y-2 md:col-span-2">
                <Label htmlFor="mcp-desc">Description</Label>
                <Input
                  id="mcp-desc"
                  value={form.description}
                  onChange={(e) => setForm({ ...form, description: e.target.value })}
                  placeholder="Optional"
                />
              </div>
              <div className="space-y-2">
                <Label>Transport</Label>
                <Select
                  value={form.transport}
                  onValueChange={(v) => setForm({ ...form, transport: v as MCPTransport })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="stdio">stdio (spawn process)</SelectItem>
                    <SelectItem value="http">HTTP (streamable)</SelectItem>
                    <SelectItem value="sse">SSE (legacy)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2 flex items-center gap-3 pt-7">
                <Switch
                  id="mcp-enabled"
                  checked={form.enabled}
                  onCheckedChange={(v) => setForm({ ...form, enabled: v })}
                />
                <Label htmlFor="mcp-enabled" className="cursor-pointer">
                  Enabled
                </Label>
              </div>

              {form.transport === "stdio" ? (
                <>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="mcp-cmd">Command</Label>
                    <Input
                      id="mcp-cmd"
                      value={form.command}
                      onChange={(e) => setForm({ ...form, command: e.target.value })}
                      placeholder="npx"
                      className="font-mono"
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="mcp-args">Arguments</Label>
                    <Input
                      id="mcp-args"
                      value={form.argsText}
                      onChange={(e) => setForm({ ...form, argsText: e.target.value })}
                      placeholder="-y @modelcontextprotocol/server-filesystem /tmp"
                      className="font-mono"
                    />
                    <p className="text-xs text-muted-foreground">Space-separated. Quotes supported.</p>
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="mcp-workdir">Working directory</Label>
                    <Input
                      id="mcp-workdir"
                      value={form.workingDir}
                      onChange={(e) => setForm({ ...form, workingDir: e.target.value })}
                      placeholder="Optional"
                      className="font-mono"
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="mcp-env">Environment variables</Label>
                    <textarea
                      id="mcp-env"
                      value={form.envText}
                      onChange={(e) => setForm({ ...form, envText: e.target.value })}
                      placeholder={"KEY=value\nOTHER=value"}
                      rows={3}
                      className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
                    />
                    <p className="text-xs text-muted-foreground">
                      One KEY=value per line. Stored as-is — do not put live secrets until encryption-at-rest lands.
                    </p>
                  </div>
                </>
              ) : (
                <>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="mcp-endpoint">Endpoint URL</Label>
                    <Input
                      id="mcp-endpoint"
                      value={form.endpoint}
                      onChange={(e) => setForm({ ...form, endpoint: e.target.value })}
                      placeholder="https://my-mcp-server.example.com/mcp"
                      className="font-mono"
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="mcp-headers">Headers</Label>
                    <textarea
                      id="mcp-headers"
                      value={form.headersText}
                      onChange={(e) => setForm({ ...form, headersText: e.target.value })}
                      placeholder={"Authorization: Bearer xxx\nX-Custom: value"}
                      rows={3}
                      className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
                    />
                    <p className="text-xs text-muted-foreground">One header per line: <code>Name: value</code>.</p>
                  </div>
                </>
              )}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setForm(null)}>
              Cancel
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              {form?.id ? "Save changes" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete MCP Server</AlertDialogTitle>
            <AlertDialogDescription>
              Remove <span className="font-mono">{deleteTarget?.slug}</span> from the gateway? Active
              sessions using its tools will start failing immediately.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function StatCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: number | string;
  tone?: "ok" | "warn";
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div
          className={
            "text-2xl font-semibold " +
            (tone === "warn"
              ? "text-amber-600 dark:text-amber-400"
              : tone === "ok"
                ? "text-green-600 dark:text-green-400"
                : "")
          }
        >
          {value}
        </div>
      </CardContent>
    </Card>
  );
}
