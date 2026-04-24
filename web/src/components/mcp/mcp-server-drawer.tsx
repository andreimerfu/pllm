import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import {
  createMCPServer,
  updateMCPServer,
} from "@/lib/api";
import type {
  MCPServer,
  MCPTransport,
  MCPUpsertRequest,
} from "@/types/api";

import { FormDrawer } from "@/components/common/FormDrawer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Templates — common MCP servers. Picking one pre-fills the whole form.
// ---------------------------------------------------------------------------

type Template = {
  id: string;
  label: string;
  description: string;
  icon: string;
  colored: boolean;
  transport: MCPTransport;
  name: string;
  slug: string;
  command?: string;
  args?: string;
  endpoint?: string;
};

const TEMPLATES: Template[] = [
  {
    id: "fs",
    label: "Filesystem",
    description: "Read / write local files",
    icon: "logos:nodejs-icon",
    colored: true,
    transport: "stdio",
    name: "Filesystem",
    slug: "fs",
    command: "npx",
    args: "-y @modelcontextprotocol/server-filesystem /tmp",
  },
  {
    id: "memory",
    label: "Memory",
    description: "Ephemeral knowledge graph",
    icon: "solar:database-linear",
    colored: false,
    transport: "stdio",
    name: "Memory",
    slug: "memory",
    command: "npx",
    args: "-y @modelcontextprotocol/server-memory",
  },
  {
    id: "everything",
    label: "Everything",
    description: "MCP reference / demo",
    icon: "simple-icons:anthropic",
    colored: false,
    transport: "stdio",
    name: "Everything",
    slug: "everything",
    command: "npx",
    args: "-y @modelcontextprotocol/server-everything",
  },
  {
    id: "http",
    label: "Hosted MCP",
    description: "Vendor MCP over HTTP (GitHub, Linear, etc.)",
    icon: "solar:server-linear",
    colored: false,
    transport: "http",
    name: "",
    slug: "",
    endpoint: "https://my-mcp-server.example.com/mcp",
  },
  {
    id: "sse",
    label: "Remote SSE",
    description: "Legacy MCP over SSE",
    icon: "solar:bolt-linear",
    colored: false,
    transport: "sse",
    name: "",
    slug: "",
    endpoint: "https://my-mcp-server.example.com/sse",
  },
  {
    id: "custom",
    label: "Custom stdio",
    description: "Any npx / uvx / binary",
    icon: "solar:command-linear",
    colored: false,
    transport: "stdio",
    name: "",
    slug: "",
  },
];

// ---------------------------------------------------------------------------
// Form state
// ---------------------------------------------------------------------------

export type FormState = {
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

export const emptyForm: FormState = {
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

export function toFormState(s: MCPServer): FormState {
  const env = s.env
    ? Object.entries(s.env).map(([k, v]) => `${k}=${v}`).join("\n")
    : "";
  const headers = s.headers
    ? Object.entries(s.headers).map(([k, v]) => `${k}: ${v}`).join("\n")
    : "";
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
    return {
      error: "Slug must be lowercase letters, digits, hyphen or underscore",
    };
  }
  if (form.transport === "stdio" && !form.command.trim()) {
    return { error: "Command is required for stdio transport" };
  }
  if (
    (form.transport === "http" || form.transport === "sse") &&
    !form.endpoint.trim()
  ) {
    return {
      error: "Endpoint URL is required for http/sse transport",
    };
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

// Slug sanitizer — derive a safe slug from an arbitrary string.
function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40);
}

// ---------------------------------------------------------------------------
// Drawer
// ---------------------------------------------------------------------------

export function MCPServerDrawer({
  open,
  onOpenChange,
  initial,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  initial: FormState | null;
}) {
  const { toast } = useToast();
  const qc = useQueryClient();

  const [form, setForm] = useState<FormState>(emptyForm);
  const [templateId, setTemplateId] = useState<string | null>(null);
  const [slugTouched, setSlugTouched] = useState(false);

  const isEdit = !!form.id;

  useEffect(() => {
    if (!open) return;
    if (initial) {
      setForm(initial);
      setTemplateId(null);
      setSlugTouched(true);
    } else {
      setForm({ ...emptyForm });
      setTemplateId(null);
      setSlugTouched(false);
    }
  }, [open, initial]);

  // Auto-sync slug from name unless the user has started editing slug
  // directly. This way name-first flows feel natural.
  const onNameChange = (next: string) => {
    setForm((f) => ({
      ...f,
      name: next,
      slug: slugTouched ? f.slug : slugify(next),
    }));
  };

  const applyTemplate = (t: Template) => {
    setTemplateId(t.id);
    setSlugTouched(false);
    setForm((f) => ({
      ...f,
      transport: t.transport,
      name: t.name || f.name,
      slug: t.slug || f.slug,
      command: t.command ?? "",
      argsText: t.args ?? "",
      endpoint: t.endpoint ?? "",
    }));
  };

  const createMutation = useMutation({
    mutationFn: (body: MCPUpsertRequest) => createMCPServer(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({ title: "MCP server created" });
      onOpenChange(false);
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
      qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      toast({ title: "MCP server updated" });
      onOpenChange(false);
    },
    onError: (e: any) => {
      toast({
        title: "Update failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const handleSubmit = () => {
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

  const pending = createMutation.isPending || updateMutation.isPending;

  return (
    <FormDrawer
      open={open}
      onOpenChange={onOpenChange}
      size="lg"
      title={isEdit ? "Edit MCP server" : "Add MCP server"}
      description={
        isEdit
          ? "Backend is restarted when transport details change."
          : "Pick a template to skip the typing, or configure a custom backend."
      }
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={pending}>
            {pending ? "Saving…" : isEdit ? "Save changes" : "Create server"}
          </Button>
        </>
      }
    >
      <div className="space-y-6">
        {/* ────────── Templates (create only) ────────── */}
        {!isEdit && (
          <section className="space-y-2">
            <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
              Start from a template
            </Label>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {TEMPLATES.map((t) => {
                const active = templateId === t.id;
                return (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => applyTemplate(t)}
                    className={cn(
                      "rounded-lg border p-3 text-left transition-all",
                      "flex flex-col gap-1.5",
                      "hover:border-primary/40",
                      active
                        ? "border-primary ring-1 ring-primary bg-primary/[0.04]"
                        : "border-border",
                    )}
                  >
                    <div
                      className={cn(
                        "h-8 w-8 rounded-md flex items-center justify-center",
                        t.colored ? "bg-background" : "bg-muted text-muted-foreground",
                      )}
                    >
                      <Icon icon={t.icon} className="h-5 w-5" />
                    </div>
                    <div className="min-w-0">
                      <div className="text-[12px] font-semibold leading-tight truncate">
                        {t.label}
                      </div>
                      <div className="text-[10px] text-muted-foreground line-clamp-2">
                        {t.description}
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          </section>
        )}

        {/* ────────── Identity ────────── */}
        <section className="space-y-3">
          <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
            Identity
          </Label>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="mcp-name" className="text-xs">
                Name
              </Label>
              <Input
                id="mcp-name"
                value={form.name}
                onChange={(e) => onNameChange(e.target.value)}
                placeholder="Filesystem"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="mcp-slug" className="text-xs">
                Slug{" "}
                <span className="text-[10px] font-mono text-muted-foreground">
                  lowercase, dashes
                </span>
              </Label>
              <Input
                id="mcp-slug"
                value={form.slug}
                disabled={isEdit}
                onChange={(e) => {
                  setSlugTouched(true);
                  setForm({ ...form, slug: slugify(e.target.value) });
                }}
                placeholder="fs"
                className="font-mono"
              />
            </div>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="mcp-desc" className="text-xs">
              Description{" "}
              <span className="text-[10px] font-mono text-muted-foreground">
                optional
              </span>
            </Label>
            <Input
              id="mcp-desc"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
              placeholder="What this backend does"
            />
          </div>
        </section>

        {/* ────────── Transport ────────── */}
        <section className="space-y-3">
          <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
            Transport
          </Label>
          <div className="grid grid-cols-3 gap-2">
            <TransportTab
              value="stdio"
              active={form.transport}
              icon="solar:command-linear"
              label="stdio"
              hint="Spawn process"
              onClick={() =>
                setForm({ ...form, transport: "stdio" })
              }
            />
            <TransportTab
              value="http"
              active={form.transport}
              icon="solar:server-linear"
              label="HTTP"
              hint="Streamable"
              onClick={() =>
                setForm({ ...form, transport: "http" })
              }
            />
            <TransportTab
              value="sse"
              active={form.transport}
              icon="solar:bolt-linear"
              label="SSE"
              hint="Legacy"
              onClick={() =>
                setForm({ ...form, transport: "sse" })
              }
            />
          </div>
        </section>

        {/* ────────── Config — stdio ────────── */}
        {form.transport === "stdio" && (
          <section className="space-y-3">
            <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
              Process
            </Label>
            <div className="space-y-1.5">
              <Label htmlFor="mcp-cmd" className="text-xs">
                Command
              </Label>
              <Input
                id="mcp-cmd"
                value={form.command}
                onChange={(e) =>
                  setForm({ ...form, command: e.target.value })
                }
                placeholder="npx"
                className="font-mono"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="mcp-args" className="text-xs">
                Arguments
              </Label>
              <Input
                id="mcp-args"
                value={form.argsText}
                onChange={(e) =>
                  setForm({ ...form, argsText: e.target.value })
                }
                placeholder="-y @modelcontextprotocol/server-filesystem /tmp"
                className="font-mono"
              />
              <p className="text-[11px] text-muted-foreground">
                Space-separated. Use quotes for args with spaces.
              </p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="mcp-workdir" className="text-xs">
                Working directory{" "}
                <span className="text-[10px] font-mono text-muted-foreground">
                  optional
                </span>
              </Label>
              <Input
                id="mcp-workdir"
                value={form.workingDir}
                onChange={(e) =>
                  setForm({ ...form, workingDir: e.target.value })
                }
                placeholder="/app"
                className="font-mono"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="mcp-env" className="text-xs">
                Environment variables
              </Label>
              <textarea
                id="mcp-env"
                value={form.envText}
                onChange={(e) =>
                  setForm({ ...form, envText: e.target.value })
                }
                placeholder={"GITHUB_TOKEN=ghp_…\nANTHROPIC_API_KEY=sk-…"}
                rows={3}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
              />
              <p className="text-[11px] text-muted-foreground">
                One <code className="font-mono">KEY=value</code> per line. Lines starting with <code className="font-mono">#</code> are ignored.
              </p>
            </div>
          </section>
        )}

        {/* ────────── Config — http/sse ────────── */}
        {(form.transport === "http" || form.transport === "sse") && (
          <section className="space-y-3">
            <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
              Endpoint
            </Label>
            <div className="space-y-1.5">
              <Label htmlFor="mcp-endpoint" className="text-xs">
                URL
              </Label>
              <Input
                id="mcp-endpoint"
                value={form.endpoint}
                onChange={(e) =>
                  setForm({ ...form, endpoint: e.target.value })
                }
                placeholder={
                  form.transport === "http"
                    ? "https://my-mcp-server.example.com/mcp"
                    : "https://my-mcp-server.example.com/sse"
                }
                className="font-mono"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="mcp-headers" className="text-xs">
                Headers
              </Label>
              <textarea
                id="mcp-headers"
                value={form.headersText}
                onChange={(e) =>
                  setForm({ ...form, headersText: e.target.value })
                }
                placeholder={"Authorization: Bearer …\nX-Api-Key: …"}
                rows={3}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
              />
              <p className="text-[11px] text-muted-foreground">
                One <code className="font-mono">Name: value</code> per line.
              </p>
            </div>
          </section>
        )}

        {/* ────────── Enabled ────────── */}
        <section className="rounded-lg border border-border bg-muted/20 p-3 flex items-center justify-between">
          <div>
            <Label htmlFor="mcp-enabled" className="cursor-pointer text-sm">
              Enabled
            </Label>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              When off, the backend is stored but not routed.
            </p>
          </div>
          <Switch
            id="mcp-enabled"
            checked={form.enabled}
            onCheckedChange={(v) => setForm({ ...form, enabled: v })}
          />
        </section>
      </div>
    </FormDrawer>
  );
}

function TransportTab({
  value,
  active,
  icon,
  label,
  hint,
  onClick,
}: {
  value: MCPTransport;
  active: MCPTransport;
  icon: string;
  label: string;
  hint: string;
  onClick: () => void;
}) {
  const selected = value === active;
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-lg border p-3 text-left flex flex-col gap-0.5 transition-all",
        "hover:border-primary/40",
        selected
          ? "border-primary ring-1 ring-primary bg-primary/[0.04]"
          : "border-border",
      )}
    >
      <div className="flex items-center gap-1.5">
        <Icon
          icon={icon}
          className={cn(
            "h-4 w-4",
            selected ? "text-primary" : "text-muted-foreground",
          )}
        />
        <span className="text-[13px] font-mono font-semibold">{label}</span>
      </div>
      <span className="text-[10px] text-muted-foreground">{hint}</span>
    </button>
  );
}

export { icons };
