import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import { createUseCase, updateUseCase } from "@/lib/api";
import type {
  UseCaseRef,
  UseCaseRefKind,
  UseCaseRiskTier,
  UseCaseStatus,
  UseCaseUpsertRequest,
  UseCaseWithRefs,
} from "@/types/api";

import { FormDrawer } from "@/components/common/FormDrawer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Static catalogs
// ---------------------------------------------------------------------------

const RISK_TIERS: {
  value: UseCaseRiskTier;
  label: string;
  tone: string;
  description: string;
}[] = [
  {
    value: "tier1",
    label: "Tier 1",
    tone: "bg-rose-500",
    description: "Customer-facing decisions, material financial exposure",
  },
  {
    value: "tier2",
    label: "Tier 2",
    tone: "bg-amber-500",
    description: "Internal decisions, significant operational reliance",
  },
  {
    value: "tier3",
    label: "Tier 3",
    tone: "bg-sky-500",
    description: "Productivity, summarization, drafting",
  },
  {
    value: "tier4",
    label: "Tier 4",
    tone: "bg-slate-400",
    description: "Sandbox, experimentation",
  },
];

const STATUSES: { value: UseCaseStatus; label: string }[] = [
  { value: "draft", label: "Draft" },
  { value: "under_review", label: "Under review" },
  { value: "approved", label: "Approved" },
  { value: "retired", label: "Retired" },
];

const REF_KIND_ORDER: UseCaseRefKind[] = [
  "server",
  "agent",
  "llm",
  "skill",
  "prompt",
];

const REF_KIND_META: Record<
  UseCaseRefKind,
  { icon: string; tint: string; label: string; placeholder: string }
> = {
  server: {
    icon: icons.mcp,
    tint: "text-slate-600 dark:text-slate-300",
    label: "MCP server",
    placeholder: "io.modelcontextprotocol/server-everything",
  },
  agent: {
    icon: icons.agent,
    tint: "text-indigo-600 dark:text-indigo-300",
    label: "Agent",
    placeholder: "acme/triage-agent",
  },
  llm: {
    icon: icons.llm,
    tint: "text-violet-600 dark:text-violet-300",
    label: "LLM",
    placeholder: "anthropic/claude-sonnet-4-5",
  },
  skill: {
    icon: icons.skill,
    tint: "text-emerald-600 dark:text-emerald-300",
    label: "Skill",
    placeholder: "acme/ticket-triage",
  },
  prompt: {
    icon: icons.prompt,
    tint: "text-amber-600 dark:text-amber-300",
    label: "Prompt",
    placeholder: "acme/classifier-system",
  },
};

const DEFAULT_CLASSIFICATIONS = [
  "pii",
  "pci",
  "phi",
  "customer-financial",
  "confidential",
  "internal",
  "public",
];

const CLASS_TONE: Record<string, string> = {
  pii: "bg-rose-500",
  pci: "bg-rose-500",
  phi: "bg-rose-500",
  "customer-financial": "bg-amber-500",
  confidential: "bg-amber-500",
  internal: "bg-sky-500",
  public: "bg-emerald-500",
};

function classTone(c: string): string {
  return CLASS_TONE[c] ?? "bg-slate-500";
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function gatherRefs(existing: UseCaseWithRefs | null): UseCaseRef[] {
  if (!existing) return [];
  const all: UseCaseRef[] = [
    ...(existing.servers ?? []),
    ...(existing.agents ?? []),
    ...(existing.llms ?? []),
    ...(existing.skills ?? []),
    ...(existing.prompts ?? []),
  ];
  return all.map((r) => ({ ...r }));
}

// ---------------------------------------------------------------------------
// Drawer
// ---------------------------------------------------------------------------

export function UseCaseDrawer({
  open,
  onOpenChange,
  existing,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  existing: UseCaseWithRefs | null;
}) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const isEdit = !!existing;

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [riskTier, setRiskTier] = useState<UseCaseRiskTier>("tier4");
  const [status, setStatus] = useState<UseCaseStatus>("draft");
  const [classifications, setClassifications] = useState<string[]>([]);
  const [refs, setRefs] = useState<UseCaseRef[]>([]);
  const [enforceScope, setEnforceScope] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(existing?.use_case.name ?? "");
    setDescription(existing?.use_case.description ?? "");
    setRiskTier(existing?.use_case.risk_tier ?? "tier4");
    setStatus(existing?.use_case.status ?? "draft");
    setClassifications(existing?.use_case.data_classifications ?? []);
    setRefs(gatherRefs(existing));
    setEnforceScope(existing?.use_case.enforce_scope ?? false);
  }, [open, existing]);

  const mutation = useMutation({
    mutationFn: (body: UseCaseUpsertRequest) => {
      if (isEdit && existing) {
        return updateUseCase(existing.use_case.id, body);
      }
      return createUseCase(body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["use-cases"] });
      qc.invalidateQueries({ queryKey: ["use-cases-all"] });
      qc.invalidateQueries({ queryKey: ["use-case-detail"] });
      toast({ title: isEdit ? "Use case updated" : "Use case created" });
      onOpenChange(false);
    },
    onError: (e: any) => {
      toast({
        title: "Save failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  const handleSubmit = () => {
    if (!name.trim()) {
      toast({ title: "Name is required", variant: "destructive" });
      return;
    }
    mutation.mutate({
      name: name.trim(),
      description: description.trim() || undefined,
      risk_tier: riskTier,
      status,
      data_classifications: classifications.length ? classifications : undefined,
      refs: refs.filter((r) => r.target_name.trim()),
      enforce_scope: enforceScope,
    });
  };

  return (
    <FormDrawer
      open={open}
      onOpenChange={onOpenChange}
      size="xl"
      title={isEdit ? "Edit use case" : "New use case"}
      description={
        isEdit
          ? "Changes replace the current ref set wholesale."
          : "Name the business activity, declare what it depends on, pick the risk tier."
      }
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={mutation.isPending}>
            {mutation.isPending
              ? "Saving…"
              : isEdit
                ? "Save changes"
                : "Create use case"}
          </Button>
        </>
      }
    >
      <div className="space-y-6">
        {/* ────────── Basics ────────── */}
        <section className="space-y-3">
          <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
            Basics
          </Label>
          <div className="space-y-1.5">
            <Label htmlFor="uc-name" className="text-xs">
              Name
            </Label>
            <Input
              id="uc-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Mortgage Adverse Action Agent"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="uc-desc" className="text-xs">
              Description{" "}
              <span className="text-[10px] font-mono text-muted-foreground">
                optional
              </span>
            </Label>
            <textarea
              id="uc-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
              className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              placeholder="What business activity does this represent?"
            />
          </div>
        </section>

        {/* ────────── Risk tier ────────── */}
        <section className="space-y-2">
          <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
            Risk tier
          </Label>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
            {RISK_TIERS.map((t) => {
              const active = t.value === riskTier;
              return (
                <button
                  key={t.value}
                  type="button"
                  onClick={() => setRiskTier(t.value)}
                  className={cn(
                    "rounded-lg border p-3 text-left flex flex-col gap-1 transition-all",
                    "hover:border-primary/40",
                    active
                      ? "border-primary ring-1 ring-primary bg-primary/[0.04]"
                      : "border-border",
                  )}
                >
                  <div className="flex items-center gap-1.5">
                    <span className={cn("h-2 w-2 rounded-full", t.tone)} />
                    <span className="text-[12px] font-semibold">{t.label}</span>
                  </div>
                  <span className="text-[10px] text-muted-foreground line-clamp-2">
                    {t.description}
                  </span>
                </button>
              );
            })}
          </div>
        </section>

        {/* ────────── Status ────────── */}
        <section className="space-y-2">
          <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
            Lifecycle
          </Label>
          <Select
            value={status}
            onValueChange={(v) => setStatus(v as UseCaseStatus)}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {STATUSES.map((s) => (
                <SelectItem key={s.value} value={s.value}>
                  {s.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </section>

        {/* ────────── Classifications ────────── */}
        <section className="space-y-2">
          <Label className="flex items-baseline justify-between text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
            Data classifications
            <span className="text-[10px] font-mono text-muted-foreground/70 normal-case tracking-normal">
              propagate to dependents
            </span>
          </Label>
          <div className="flex flex-wrap gap-1.5">
            {DEFAULT_CLASSIFICATIONS.map((c) => {
              const on = classifications.includes(c);
              return (
                <button
                  key={c}
                  type="button"
                  onClick={() =>
                    setClassifications((prev) =>
                      on ? prev.filter((x) => x !== c) : [...prev, c],
                    )
                  }
                  className={cn(
                    "inline-flex items-center gap-1.5 text-[11px] font-mono uppercase tracking-wider h-7 px-2.5 rounded-md ring-1 transition",
                    on
                      ? "bg-foreground text-background ring-foreground"
                      : "bg-muted text-muted-foreground ring-border hover:bg-foreground/10",
                  )}
                >
                  <span
                    className={cn(
                      "h-1.5 w-1.5 rounded-full",
                      on ? "bg-background/60" : classTone(c),
                    )}
                  />
                  {c}
                </button>
              );
            })}
          </div>
        </section>

        {/* ────────── Components ────────── */}
        <section className="space-y-2">
          <Label className="flex items-baseline justify-between text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
            Components
            <span className="text-[10px] font-mono text-muted-foreground/70 normal-case tracking-normal">
              {refs.length} declared
            </span>
          </Label>
          <ComponentRefsEditor refs={refs} onChange={setRefs} />
        </section>

        {/* ────────── Runtime enforcement ────────── */}
        <section
          className={cn(
            "rounded-lg border p-3",
            enforceScope
              ? "border-rose-500/40 bg-rose-500/[0.04]"
              : "border-border bg-muted/20",
          )}
        >
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <Label
                htmlFor="enforce-scope"
                className="cursor-pointer text-sm font-semibold flex items-center gap-2"
              >
                <Icon
                  icon={enforceScope ? icons.shield : icons.shieldAlert}
                  className={cn(
                    "h-4 w-4",
                    enforceScope
                      ? "text-rose-600 dark:text-rose-400"
                      : "text-muted-foreground",
                  )}
                />
                Runtime enforcement
                <span
                  className={cn(
                    "inline-flex items-center h-5 px-1.5 rounded-md ring-1 text-[10px] font-mono uppercase tracking-wider",
                    enforceScope
                      ? "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/40"
                      : "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
                  )}
                >
                  {enforceScope ? "blocking" : "flag mode"}
                </span>
              </Label>
              <p className="text-[12px] text-muted-foreground mt-1.5">
                {enforceScope
                  ? "Out-of-scope calls are blocked with a 403 / JSON-RPC error."
                  : "Out-of-scope calls are logged to the audit trail but still proceed. Flip to enforcing once the log is clean."}
              </p>
            </div>
            <Switch
              id="enforce-scope"
              checked={enforceScope}
              onCheckedChange={setEnforceScope}
            />
          </div>
        </section>
      </div>
    </FormDrawer>
  );
}

// ---------------------------------------------------------------------------
// Components editor
// ---------------------------------------------------------------------------

function ComponentRefsEditor({
  refs,
  onChange,
}: {
  refs: UseCaseRef[];
  onChange: (refs: UseCaseRef[]) => void;
}) {
  const update = (i: number, patch: Partial<UseCaseRef>) => {
    const next = refs.slice();
    next[i] = { ...next[i], ...patch };
    onChange(next);
  };
  const remove = (i: number) => {
    onChange(refs.filter((_, j) => j !== i));
  };
  const addOfKind = (kind: UseCaseRefKind) => {
    onChange([...refs, { target_kind: kind, target_name: "" }]);
  };

  return (
    <div className="space-y-2">
      {refs.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border p-6 text-center">
          <Icon
            icon={icons.layers}
            className="h-6 w-6 mx-auto text-muted-foreground/50 mb-2"
          />
          <p className="text-xs text-muted-foreground">
            No components yet — pick a kind below to add the first one.
          </p>
        </div>
      ) : (
        <ul className="space-y-1.5">
          {refs.map((r, i) => {
            const meta = REF_KIND_META[r.target_kind];
            return (
              <li
                key={i}
                className="rounded-md border border-border bg-card p-2 flex items-center gap-2"
              >
                <Select
                  value={r.target_kind}
                  onValueChange={(v) =>
                    update(i, { target_kind: v as UseCaseRefKind })
                  }
                >
                  <SelectTrigger className="h-8 w-[130px] shrink-0 gap-1.5">
                    <div className="flex items-center gap-1.5">
                      <Icon
                        icon={meta.icon}
                        className={cn("h-3.5 w-3.5", meta.tint)}
                      />
                      <span className="text-xs font-mono capitalize">
                        {r.target_kind}
                      </span>
                    </div>
                  </SelectTrigger>
                  <SelectContent>
                    {REF_KIND_ORDER.map((k) => {
                      const m = REF_KIND_META[k];
                      return (
                        <SelectItem key={k} value={k}>
                          <div className="flex items-center gap-1.5">
                            <Icon
                              icon={m.icon}
                              className={cn("h-3.5 w-3.5", m.tint)}
                            />
                            <span>{m.label}</span>
                          </div>
                        </SelectItem>
                      );
                    })}
                  </SelectContent>
                </Select>
                <Input
                  value={r.target_name}
                  onChange={(e) => update(i, { target_name: e.target.value })}
                  placeholder={meta.placeholder}
                  className="h-8 font-mono text-xs flex-1"
                />
                <Input
                  value={r.target_version ?? ""}
                  onChange={(e) =>
                    update(i, {
                      target_version: e.target.value || undefined,
                    })
                  }
                  placeholder="version"
                  className="h-8 font-mono text-xs w-[100px] shrink-0"
                />
                <Input
                  value={r.local_name ?? ""}
                  onChange={(e) =>
                    update(i, { local_name: e.target.value || undefined })
                  }
                  placeholder="alias"
                  className="h-8 font-mono text-xs w-[90px] shrink-0"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 text-muted-foreground hover:text-destructive"
                  onClick={() => remove(i)}
                  title="Remove"
                >
                  <Icon icon={icons.delete} className="h-3.5 w-3.5" />
                </Button>
              </li>
            );
          })}
        </ul>
      )}

      <div className="flex items-center gap-1.5 flex-wrap pt-1">
        <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mr-1">
          add
        </span>
        {REF_KIND_ORDER.map((k) => {
          const m = REF_KIND_META[k];
          return (
            <Button
              key={k}
              type="button"
              variant="outline"
              size="sm"
              onClick={() => addOfKind(k)}
              className="h-7 gap-1.5 text-xs"
            >
              <Icon icon={m.icon} className={cn("h-3.5 w-3.5", m.tint)} />
              {m.label}
            </Button>
          );
        })}
      </div>
    </div>
  );
}
