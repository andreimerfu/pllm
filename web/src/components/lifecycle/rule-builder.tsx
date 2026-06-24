// Rule builder dialog — operator-facing workflow for creating and
// editing custom lifecycle rules. The user picks a template, fills in
// a typed param form, sees a live "X rows would be affected" preview,
// and saves. We never expose CEL: the backend resolves the template +
// params into a CEL expression at save time.
//
// The dialog has three logical phases:
//   1. Template picker (only shown for new rules)
//   2. Form (params + name + dry-run + enabled)
//   3. Preview + save
//
// Steps 2 and 3 share the form view — the preview button just calls
// the backend and updates a side panel, no navigation needed.

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import {
  createCustomRule,
  listLifecycleTemplates,
  previewCustomRule,
  updateCustomRule,
} from "@/lib/api";
import { icons } from "@/lib/icons";
import { cn } from "@/lib/utils";
import type {
  CustomRulePreviewResponse,
  CustomRuleUpsertRequest,
  LifecycleCustomRule,
  LifecycleTemplate,
  LifecycleTemplateParam,
} from "@/types/api";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";

// ──────────────────────── Helpers ────────────────────────

const CATEGORY_META: Record<
  LifecycleTemplate["category"],
  { label: string; icon: string; tone: string }
> = {
  keys: {
    label: "API Keys",
    icon: "solar:key-minimalistic-linear",
    tone: "text-amber-600 dark:text-amber-400",
  },
  servers: {
    label: "MCP Servers",
    icon: "mingcute:mcp-line",
    tone: "text-violet-600 dark:text-violet-400",
  },
  use_cases: {
    label: "Use Cases",
    icon: "solar:target-linear",
    tone: "text-emerald-600 dark:text-emerald-400",
  },
  users: {
    label: "Users",
    icon: "solar:user-linear",
    tone: "text-sky-600 dark:text-sky-400",
  },
};

const ACTION_META: Record<
  LifecycleTemplate["action"],
  { label: string; tone: string; description: string }
> = {
  revoke: {
    label: "Revoke",
    tone:
      "bg-rose-500/10 text-rose-700 dark:text-rose-300 ring-rose-500/30",
    description: "Marks the key inactive and prevents further use.",
  },
  block: {
    label: "Block",
    tone:
      "bg-amber-500/10 text-amber-700 dark:text-amber-300 ring-amber-500/30",
    description: "Sets the server status to blocked. Operators clear manually.",
  },
  flag: {
    label: "Flag",
    tone:
      "bg-violet-500/10 text-violet-700 dark:text-violet-300 ring-violet-500/30",
    description: "Sets the compliance flag with the rule's reason.",
  },
  notify: {
    label: "Notify",
    tone: "bg-sky-500/10 text-sky-700 dark:text-sky-300 ring-sky-500/30",
    description: "Logs an audit row and fires a webhook. No mutation.",
  },
};

// slugifyName turns the user-entered free-form name into the rule
// identifier the backend accepts (lowercase, underscores, alnum only).
// Operators see this preview live as they type, which makes the
// validation rules feel discoverable.
function slugifyName(input: string): string {
  return input
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9_]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 64);
}

// defaultParamsForTemplate seeds the params object from each template
// param's `default` value so the preview is meaningful out of the box.
function defaultParamsForTemplate(t: LifecycleTemplate): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const p of t.params) {
    if (p.default !== undefined) out[p.key] = p.default;
  }
  return out;
}

// ──────────────────────── Component ────────────────────────

export interface RuleBuilderProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // existing is non-null on edit; null on create.
  existing: LifecycleCustomRule | null;
}

export function RuleBuilder({ open, onOpenChange, existing }: RuleBuilderProps) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const isEdit = !!existing;

  const { data: tmplData } = useQuery({
    queryKey: ["lifecycle-templates"],
    queryFn: listLifecycleTemplates,
    staleTime: 60_000,
  });
  const templates = useMemo(
    () => tmplData?.templates ?? [],
    [tmplData],
  );

  // ---- form state ----
  const [templateId, setTemplateId] = useState<string>("");
  const [name, setName] = useState<string>("");
  const [description, setDescription] = useState<string>("");
  const [params, setParams] = useState<Record<string, unknown>>({});
  const [reason, setReason] = useState<string>("");
  const [enabled, setEnabled] = useState<boolean>(true);
  const [dryRunOverride, setDryRunOverride] = useState<"inherit" | "on" | "off">(
    "inherit",
  );
  const [preview, setPreview] = useState<CustomRulePreviewResponse | null>(
    null,
  );

  // Reset state whenever the dialog opens with a different rule (or
  // none, on create). This is the simpler form of "uncontrolled until
  // open" — we don't try to preserve in-flight edits across opens.
  useEffect(() => {
    if (!open) return;
    if (existing) {
      setTemplateId(existing.template_id);
      setName(existing.name);
      setDescription(existing.description ?? "");
      setParams((existing.params as Record<string, unknown>) ?? {});
      setReason(existing.reason ?? "");
      setEnabled(existing.enabled);
      setDryRunOverride(
        existing.dry_run === null || existing.dry_run === undefined
          ? "inherit"
          : existing.dry_run
            ? "on"
            : "off",
      );
    } else {
      setTemplateId("");
      setName("");
      setDescription("");
      setParams({});
      setReason("");
      setEnabled(true);
      setDryRunOverride("inherit");
    }
    setPreview(null);
  }, [open, existing]);

  const selectedTemplate = useMemo(
    () => templates.find((t) => t.id === templateId) ?? null,
    [templates, templateId],
  );

  // When the user picks a fresh template, seed params with defaults
  // and clear the prior preview so they're never looking at stale data.
  function pickTemplate(t: LifecycleTemplate) {
    setTemplateId(t.id);
    setParams(defaultParamsForTemplate(t));
    setPreview(null);
  }

  function setParam(key: string, value: unknown) {
    setParams((prev) => ({ ...prev, [key]: value }));
    setPreview(null);
  }

  // ---- mutations ----
  const previewMut = useMutation({
    mutationFn: (req: { template_id: string; params: Record<string, unknown> }) =>
      previewCustomRule(req),
    onSuccess: (data) => setPreview(data),
    onError: (e: { response?: { data?: { error?: string } }; message?: string }) =>
      toast({
        title: "Preview failed",
        description: e?.response?.data?.error ?? e?.message ?? "unknown error",
        variant: "destructive",
      }),
  });

  const saveMut = useMutation({
    mutationFn: (req: CustomRuleUpsertRequest) =>
      isEdit && existing
        ? updateCustomRule(existing.id, req)
        : createCustomRule(req),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["lifecycle-custom-rules"] });
      qc.invalidateQueries({ queryKey: ["lifecycle-status"] });
      toast({
        title: isEdit ? "Rule updated" : "Rule created",
        description: name,
      });
      onOpenChange(false);
    },
    onError: (e: { response?: { data?: { error?: string } }; message?: string }) =>
      toast({
        title: "Save failed",
        description: e?.response?.data?.error ?? e?.message ?? "unknown error",
        variant: "destructive",
      }),
  });

  const finalName = slugifyName(name);
  const canPreview = !!templateId;
  const canSave =
    !!templateId && finalName.length >= 3 && !saveMut.isPending;

  function handleSave() {
    if (!selectedTemplate) return;
    const req: CustomRuleUpsertRequest = {
      name: finalName,
      description,
      template_id: selectedTemplate.id,
      params,
      enabled,
      reason: reason.trim() || undefined,
    };
    if (dryRunOverride === "on") req.dry_run = true;
    else if (dryRunOverride === "off") req.dry_run = false;
    saveMut.mutate(req);
  }

  // ──────────── render ────────────
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? "Edit hygiene rule" : "New hygiene rule"}
          </DialogTitle>
          <DialogDescription>
            Pick a control, fill in the details, and preview against current data
            before saving.
          </DialogDescription>
        </DialogHeader>

        {/* Step 1 — template picker. Hidden on edit since template is fixed. */}
        {!isEdit && (
          <section className="space-y-2">
            <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              Step 1 · Pick a control
            </p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
              {templates.map((t) => {
                const cat = CATEGORY_META[t.category];
                const sel = templateId === t.id;
                return (
                  <button
                    key={t.id}
                    onClick={() => pickTemplate(t)}
                    className={cn(
                      "text-left rounded-md border p-3 transition-colors hover:bg-accent/50 group",
                      sel
                        ? "border-primary bg-primary/5 ring-1 ring-primary/40"
                        : "border-border",
                    )}
                  >
                    <div className="flex items-start gap-2.5">
                      <Icon
                        icon={cat.icon}
                        className={cn("h-5 w-5 shrink-0 mt-0.5", cat.tone)}
                      />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-sm font-medium tracking-tight">
                            {t.title}
                          </span>
                          <span
                            className={cn(
                              "inline-flex items-center h-[18px] px-1.5 rounded text-[10px] font-mono uppercase tracking-wider ring-1 shrink-0",
                              ACTION_META[t.action].tone,
                            )}
                          >
                            {ACTION_META[t.action].label}
                          </span>
                        </div>
                        <p className="text-[12px] text-muted-foreground mt-1 leading-snug">
                          {t.description}
                        </p>
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          </section>
        )}

        {/* Step 2 — fill in the form. */}
        {selectedTemplate && (
          <section className="space-y-4">
            <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
              {isEdit ? "Settings" : "Step 2 · Configure"}
            </p>

            {/* Per-template parameter fields */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {selectedTemplate.params.map((p) => (
                <ParamField
                  key={p.key}
                  param={p}
                  value={params[p.key]}
                  onChange={(v) => setParam(p.key, v)}
                />
              ))}
            </div>

            {/* Free-form fields */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="rule-name">
                  Rule name
                  <span className="text-muted-foreground font-normal ml-1">
                    · used in audit logs and webhooks
                  </span>
                </Label>
                <Input
                  id="rule-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={
                    selectedTemplate.id === "rotate_aged_keys"
                      ? "prod_key_rotation"
                      : "my_custom_rule"
                  }
                  disabled={isEdit}
                />
                {name && finalName !== name && (
                  <p className="text-[10px] text-muted-foreground">
                    Saved as <code className="font-mono">{finalName}</code>
                  </p>
                )}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="rule-description">
                  Description
                  <span className="text-muted-foreground font-normal ml-1">
                    · optional
                  </span>
                </Label>
                <Input
                  id="rule-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="e.g. CTRL-042: prod keys rotate every 90 days"
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="rule-reason">
                Audit reason
                <span className="text-muted-foreground font-normal ml-1">
                  · what shows up in the audit row when this rule fires
                </span>
              </Label>
              <Textarea
                id="rule-reason"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder={
                  preview?.reason ?? "Leave blank to use the template default"
                }
                rows={2}
              />
            </div>

            {/* Toggles */}
            <div className="flex items-center gap-6 flex-wrap">
              <label className="flex items-center gap-2 cursor-pointer">
                <Switch checked={enabled} onCheckedChange={setEnabled} />
                <span className="text-sm">Enabled</span>
              </label>

              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">Dry-run:</span>
                <div className="inline-flex rounded-md border border-border overflow-hidden text-[12px] font-mono">
                  {(["inherit", "on", "off"] as const).map((v) => (
                    <button
                      key={v}
                      onClick={() => setDryRunOverride(v)}
                      className={cn(
                        "px-2.5 py-1 transition-colors",
                        dryRunOverride === v
                          ? "bg-primary text-primary-foreground"
                          : "hover:bg-accent",
                      )}
                    >
                      {v}
                    </button>
                  ))}
                </div>
                <span className="text-[11px] text-muted-foreground">
                  {dryRunOverride === "inherit"
                    ? "follows global setting"
                    : dryRunOverride === "on"
                      ? "log only, never mutate"
                      : "enforce always"}
                </span>
              </div>
            </div>

            {/* Step 3 — preview */}
            <div className="rounded-md border border-border bg-card/40 p-3 space-y-2">
              <div className="flex items-center justify-between gap-2">
                <p className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                  {isEdit ? "Preview" : "Step 3 · Preview"}
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() =>
                    previewMut.mutate({
                      template_id: selectedTemplate.id,
                      params,
                    })
                  }
                  disabled={!canPreview || previewMut.isPending}
                  className="gap-1.5"
                >
                  <Icon icon={icons.eye} className="h-3.5 w-3.5" />
                  {previewMut.isPending ? "Checking…" : "Preview matches"}
                </Button>
              </div>

              {preview ? (
                <PreviewPanel preview={preview} />
              ) : (
                <p className="text-[12px] text-muted-foreground italic">
                  Click <span className="font-mono">Preview matches</span> to see
                  what this rule would affect against current data. No changes
                  are made.
                </p>
              )}
            </div>
          </section>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={!canSave}>
            {saveMut.isPending
              ? "Saving…"
              : isEdit
                ? "Save changes"
                : "Create rule"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ──────────────────────── Param field ────────────────────────

function ParamField({
  param,
  value,
  onChange,
}: {
  param: LifecycleTemplateParam;
  value: unknown;
  onChange: (v: unknown) => void;
}) {
  if (param.type === "duration_days" || param.type === "int") {
    const n = typeof value === "number" ? value : Number(value ?? param.default ?? 0);
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`p-${param.key}`}>
          {param.label}
          {param.required && <span className="text-rose-600 ml-0.5">*</span>}
          {param.type === "duration_days" && (
            <span className="text-muted-foreground font-normal ml-1">· days</span>
          )}
        </Label>
        <Input
          id={`p-${param.key}`}
          type="number"
          inputMode="numeric"
          min={param.min}
          max={param.max}
          value={Number.isFinite(n) ? n : ""}
          onChange={(e) => {
            const v = e.target.value;
            onChange(v === "" ? null : Number(v));
          }}
        />
        {param.help && (
          <p className="text-[11px] text-muted-foreground">{param.help}</p>
        )}
      </div>
    );
  }
  if (param.type === "string") {
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`p-${param.key}`}>
          {param.label}
          {param.required && <span className="text-rose-600 ml-0.5">*</span>}
        </Label>
        <Input
          id={`p-${param.key}`}
          value={(value as string) ?? ""}
          placeholder={param.placeholder}
          onChange={(e) => onChange(e.target.value)}
        />
        {param.help && (
          <p className="text-[11px] text-muted-foreground">{param.help}</p>
        )}
      </div>
    );
  }
  if (param.type === "string[]") {
    const list = Array.isArray(value)
      ? (value as string[])
      : typeof value === "string"
        ? (value as string).split(/[\s,]+/).filter(Boolean)
        : [];
    return (
      <div className="space-y-1.5">
        <Label htmlFor={`p-${param.key}`}>
          {param.label}
          {param.required && <span className="text-rose-600 ml-0.5">*</span>}
          <span className="text-muted-foreground font-normal ml-1">
            · comma or space separated
          </span>
        </Label>
        <Textarea
          id={`p-${param.key}`}
          rows={2}
          value={list.join(", ")}
          placeholder={param.placeholder}
          onChange={(e) =>
            onChange(e.target.value.split(/[\s,]+/).filter(Boolean))
          }
        />
        {param.help && (
          <p className="text-[11px] text-muted-foreground">{param.help}</p>
        )}
      </div>
    );
  }
  return null;
}

// ──────────────────────── Preview panel ────────────────────────

function PreviewPanel({ preview }: { preview: CustomRulePreviewResponse }) {
  const matched = preview.matched;
  const total = preview.total_rows;
  const tone =
    matched === 0
      ? "text-emerald-600 dark:text-emerald-400"
      : matched > 50
        ? "text-rose-600 dark:text-rose-400"
        : "text-amber-600 dark:text-amber-400";
  return (
    <div className="space-y-2">
      <div className="flex items-baseline gap-2 flex-wrap">
        <span className={cn("text-2xl font-semibold tabular-nums", tone)}>
          {matched}
        </span>
        <span className="text-sm text-muted-foreground">
          of {total} rows would match
          {matched > 0 ? " — these are the records this rule would act on." : "."}
        </span>
      </div>
      {preview.sample.length > 0 && (
        <ul className="rounded-md border border-border max-h-48 overflow-y-auto divide-y divide-border/50">
          {preview.sample.map((m) => (
            <li
              key={m.resource_id}
              className="px-2.5 py-1.5 text-[12px] flex items-center gap-2"
            >
              <Icon
                icon="solar:point-on-map-perspective-linear"
                className="h-3 w-3 text-muted-foreground shrink-0"
              />
              <code className="font-mono truncate">{m.subject}</code>
            </li>
          ))}
          {preview.capped && (
            <li className="px-2.5 py-1.5 text-[11px] italic text-muted-foreground">
              showing first {preview.sample.length} matches; {matched - preview.sample.length} more not shown
            </li>
          )}
        </ul>
      )}
    </div>
  );
}
