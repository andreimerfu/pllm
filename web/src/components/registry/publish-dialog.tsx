import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";
import { icons } from "@/lib/icons";

import {
  upsertRegistryServer,
  upsertRegistryAgent,
  upsertRegistrySkill,
  upsertRegistryPrompt,
} from "@/lib/api";
import type { RegistryRef } from "@/types/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/hooks/use-toast";

type Kind = "servers" | "agents" | "skills" | "prompts";

type Form = {
  name: string;
  version: string;
  title: string;
  description: string;
  websiteUrl: string;
  // server
  packages: string;
  remotes: string;
  // agent
  language: string;
  framework: string;
  modelProvider: string;
  modelName: string;
  refs: string;
  // skill
  image: string;
  manifest: string;
  // prompt
  template: string;
  arguments: string;
};

const emptyForm: Form = {
  name: "",
  version: "",
  title: "",
  description: "",
  websiteUrl: "",
  packages: "",
  remotes: "",
  language: "",
  framework: "",
  modelProvider: "",
  modelName: "",
  refs: "",
  image: "",
  manifest: "",
  template: "",
  arguments: "",
};

// parseJSONOrThrow returns parsed JSON or throws with a clear field-specific
// error. Empty string maps to undefined (field omitted).
function parseJSONOrThrow(label: string, src: string): unknown {
  const trimmed = src.trim();
  if (!trimmed) return undefined;
  try {
    return JSON.parse(trimmed);
  } catch (e: any) {
    throw new Error(`${label}: invalid JSON — ${e.message}`);
  }
}

// parseAgentRefs takes one dep per line: "<kind> <name> [version] [as <local>]"
// e.g. "server io.example/fs 2.0.0 as fs", "skill acme/triage".
function parseAgentRefs(src: string): RegistryRef[] {
  const out: RegistryRef[] = [];
  for (const raw of src.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    // Tokenize simply on whitespace; no nested quoting needed here.
    const toks = line.split(/\s+/);
    if (toks.length < 2) {
      throw new Error(`refs: malformed line "${line}"`);
    }
    const kind = toks[0].toLowerCase();
    if (kind !== "server" && kind !== "skill" && kind !== "prompt") {
      throw new Error(`refs: unknown kind "${kind}" (expected server|skill|prompt)`);
    }
    const ref: RegistryRef = { target_kind: kind, target_name: toks[1] };
    let i = 2;
    if (i < toks.length && toks[i].toLowerCase() !== "as") {
      ref.target_version = toks[i];
      i++;
    }
    if (i < toks.length && toks[i].toLowerCase() === "as" && i + 1 < toks.length) {
      ref.local_name = toks[i + 1];
    }
    out.push(ref);
  }
  return out;
}

export function PublishDialog({
  kind,
  open,
  onOpenChange,
}: {
  kind: Kind;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [form, setForm] = useState<Form>({ ...emptyForm });
  const { toast } = useToast();
  const qc = useQueryClient();

  const reset = () => setForm({ ...emptyForm });

  const mutation = useMutation({
    mutationFn: async () => {
      if (!form.name.trim() || !form.version.trim()) {
        throw new Error("Name and version are required");
      }
      const base = {
        name: form.name.trim(),
        version: form.version.trim(),
        title: form.title.trim() || undefined,
        description: form.description.trim() || undefined,
        website_url: form.websiteUrl.trim() || undefined,
      };
      switch (kind) {
        case "servers": {
          return upsertRegistryServer({
            ...base,
            packages: parseJSONOrThrow("packages", form.packages),
            remotes: parseJSONOrThrow("remotes", form.remotes),
          } as any);
        }
        case "agents": {
          return upsertRegistryAgent({
            ...base,
            language: form.language.trim() || undefined,
            framework: form.framework.trim() || undefined,
            model_provider: form.modelProvider.trim() || undefined,
            model_name: form.modelName.trim() || undefined,
            refs: form.refs.trim() ? parseAgentRefs(form.refs) : undefined,
          } as any);
        }
        case "skills": {
          return upsertRegistrySkill({
            ...base,
            image: form.image.trim() || undefined,
            manifest: parseJSONOrThrow("manifest", form.manifest),
          } as any);
        }
        case "prompts": {
          if (!form.template.trim()) {
            throw new Error("Template is required for prompts");
          }
          return upsertRegistryPrompt({
            ...base,
            template: form.template,
            arguments: parseJSONOrThrow("arguments", form.arguments),
          } as any);
        }
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [`registry-${kind}`] });
      toast({ title: `Published ${kind.slice(0, -1)}` });
      reset();
      onOpenChange(false);
    },
    onError: (e: any) => {
      toast({
        title: "Publish failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) reset(); onOpenChange(v); }}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Publish {kindLabel(kind)}</DialogTitle>
          <DialogDescription>
            New (name, version) pairs are inserted; re-publishing the same pair overwrites the
            existing row and promotes it to latest.
          </DialogDescription>
        </DialogHeader>

        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2 col-span-2">
            <Label>Kind</Label>
            <div className="flex items-center gap-2 text-sm">
              <Icon icon={kindIcon(kind)} className="h-4 w-4" />
              <span className="capitalize">{kind.slice(0, -1)}</span>
            </div>
          </div>

          <div className="space-y-2 col-span-2 sm:col-span-1">
            <Label htmlFor="reg-name">Name</Label>
            <Input
              id="reg-name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="io.github.org/my-thing"
              className="font-mono"
            />
          </div>
          <div className="space-y-2 col-span-2 sm:col-span-1">
            <Label htmlFor="reg-version">Version</Label>
            <Input
              id="reg-version"
              value={form.version}
              onChange={(e) => setForm({ ...form, version: e.target.value })}
              onFocus={(e) => e.currentTarget.select()}
              placeholder="e.g. 1.0.0"
              className="font-mono"
            />
          </div>

          <div className="space-y-2 col-span-2">
            <Label htmlFor="reg-title">Title</Label>
            <Input
              id="reg-title"
              value={form.title}
              onChange={(e) => setForm({ ...form, title: e.target.value })}
              placeholder="Optional human-readable name"
            />
          </div>
          <div className="space-y-2 col-span-2">
            <Label htmlFor="reg-desc">Description</Label>
            <textarea
              id="reg-desc"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              rows={2}
              className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              placeholder="Short summary shown on cards"
            />
          </div>
          <div className="space-y-2 col-span-2">
            <Label htmlFor="reg-url">Website URL</Label>
            <Input
              id="reg-url"
              value={form.websiteUrl}
              onChange={(e) => setForm({ ...form, websiteUrl: e.target.value })}
              placeholder="https://..."
              className="font-mono"
            />
          </div>

          {kind === "servers" && <ServerFields form={form} setForm={setForm} />}
          {kind === "agents" && <AgentFields form={form} setForm={setForm} />}
          {kind === "skills" && <SkillFields form={form} setForm={setForm} />}
          {kind === "prompts" && <PromptFields form={form} setForm={setForm} />}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={() => mutation.mutate()} disabled={mutation.isPending}>
            {mutation.isPending ? "Publishing…" : "Publish"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type FieldProps = { form: Form; setForm: (f: Form) => void };

function ServerFields({ form, setForm }: FieldProps) {
  return (
    <>
      <div className="space-y-2 col-span-2">
        <Label htmlFor="reg-packages">Packages (JSON)</Label>
        <textarea
          id="reg-packages"
          value={form.packages}
          onChange={(e) => setForm({ ...form, packages: e.target.value })}
          rows={4}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
          placeholder={`[{"registry_type":"npm","identifier":"@org/server","version":"1.2.3"}]`}
        />
      </div>
      <div className="space-y-2 col-span-2">
        <Label htmlFor="reg-remotes">Remotes (JSON)</Label>
        <textarea
          id="reg-remotes"
          value={form.remotes}
          onChange={(e) => setForm({ ...form, remotes: e.target.value })}
          rows={3}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
          placeholder={`[{"type":"http","url":"https://..."}]`}
        />
      </div>
    </>
  );
}

function AgentFields({ form, setForm }: FieldProps) {
  return (
    <>
      <div className="space-y-2 col-span-2 sm:col-span-1">
        <Label>Language</Label>
        <Input
          value={form.language}
          onChange={(e) => setForm({ ...form, language: e.target.value })}
          placeholder="python"
        />
      </div>
      <div className="space-y-2 col-span-2 sm:col-span-1">
        <Label>Framework</Label>
        <Input
          value={form.framework}
          onChange={(e) => setForm({ ...form, framework: e.target.value })}
          placeholder="langgraph"
        />
      </div>
      <div className="space-y-2 col-span-2 sm:col-span-1">
        <Label>Model provider</Label>
        <Select
          value={form.modelProvider}
          onValueChange={(v) => setForm({ ...form, modelProvider: v })}
        >
          <SelectTrigger><SelectValue placeholder="Optional" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="openai">openai</SelectItem>
            <SelectItem value="anthropic">anthropic</SelectItem>
            <SelectItem value="google">google</SelectItem>
            <SelectItem value="azure">azure</SelectItem>
            <SelectItem value="bedrock">bedrock</SelectItem>
            <SelectItem value="vertex">vertex</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2 col-span-2 sm:col-span-1">
        <Label>Model name</Label>
        <Input
          value={form.modelName}
          onChange={(e) => setForm({ ...form, modelName: e.target.value })}
          placeholder="gpt-4o-mini"
          className="font-mono"
        />
      </div>
      <div className="space-y-2 col-span-2">
        <Label>Dependencies</Label>
        <textarea
          value={form.refs}
          onChange={(e) => setForm({ ...form, refs: e.target.value })}
          rows={4}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
          placeholder={`server io.example/fs 2.0.0 as fs
skill acme/triage
prompt acme/classifier`}
        />
        <p className="text-xs text-muted-foreground">
          One per line: <code>kind name [version] [as localName]</code>. Version empty = latest.
        </p>
      </div>
    </>
  );
}

function SkillFields({ form, setForm }: FieldProps) {
  return (
    <>
      <div className="space-y-2 col-span-2">
        <Label htmlFor="reg-image">OCI Image</Label>
        <Input
          id="reg-image"
          value={form.image}
          onChange={(e) => setForm({ ...form, image: e.target.value })}
          placeholder="ghcr.io/org/my-skill:1.0.0"
          className="font-mono"
        />
      </div>
      <div className="space-y-2 col-span-2">
        <Label htmlFor="reg-manifest">Manifest (JSON)</Label>
        <textarea
          id="reg-manifest"
          value={form.manifest}
          onChange={(e) => setForm({ ...form, manifest: e.target.value })}
          rows={4}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
          placeholder={`{"entries":[{"path":"SKILL.md"}]}`}
        />
      </div>
    </>
  );
}

function PromptFields({ form, setForm }: FieldProps) {
  return (
    <>
      <div className="space-y-2 col-span-2">
        <Label htmlFor="reg-template">Template</Label>
        <textarea
          id="reg-template"
          value={form.template}
          onChange={(e) => setForm({ ...form, template: e.target.value })}
          rows={6}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
          placeholder="You are a helpful assistant for {{role}}. Respond in {{tone}}."
        />
      </div>
      <div className="space-y-2 col-span-2">
        <Label htmlFor="reg-args">Arguments (JSON)</Label>
        <textarea
          id="reg-args"
          value={form.arguments}
          onChange={(e) => setForm({ ...form, arguments: e.target.value })}
          rows={3}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
          placeholder={`[{"name":"role","required":true},{"name":"tone"}]`}
        />
      </div>
    </>
  );
}

function kindLabel(k: Kind): string {
  return { servers: "MCP Server", agents: "Agent", skills: "Skill", prompts: "Prompt" }[k];
}
function kindIcon(k: Kind): string {
  return { servers: icons.mcp, agents: icons.agent, skills: icons.skill, prompts: icons.prompt }[k];
}
