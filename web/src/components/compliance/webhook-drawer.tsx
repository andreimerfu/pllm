import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import {
  createWebhook,
  listWebhookDestinations,
  updateWebhook,
} from "@/lib/api";
import type {
  Webhook,
  WebhookDestinationConfigKey,
  WebhookDestinationType,
  WebhookDestinationView,
  WebhookEventType,
  WebhookUpsertRequest,
} from "@/types/api";

import { FormDrawer } from "@/components/common/FormDrawer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/hooks/use-toast";
import {
  DESTINATION_VISUALS,
  visualFor,
} from "@/components/compliance/destination-visuals";

// ---------------------------------------------------------------------------
// Event catalog
// ---------------------------------------------------------------------------

const ALL_EVENT_TYPES: {
  value: WebhookEventType;
  label: string;
  group: string;
  hint: string;
}[] = [
  {
    value: "scope.denied",
    label: "scope.denied",
    group: "Runtime",
    hint: "Blocked call — policy said no",
  },
  {
    value: "scope.flagged",
    label: "scope.flagged",
    group: "Runtime",
    hint: "Out-of-scope but allowed (flag mode)",
  },
  {
    value: "use_case.created",
    label: "use_case.created",
    group: "Lifecycle",
    hint: "New use case added",
  },
  {
    value: "use_case.updated",
    label: "use_case.updated",
    group: "Lifecycle",
    hint: "Metadata or refs changed",
  },
  {
    value: "use_case.deleted",
    label: "use_case.deleted",
    group: "Lifecycle",
    hint: "Use case removed",
  },
  {
    value: "use_case.compliance_flag",
    label: "use_case.compliance_flag",
    group: "Lifecycle",
    hint: "Approved posture invalidated",
  },
  {
    value: "registry.server.saved",
    label: "registry.server.saved",
    group: "Registry",
    hint: "New version published",
  },
];

// ---------------------------------------------------------------------------
// Destination inference for edit mode — guess the preset by URL when
// the persisted webhook predates the destination_type column.
// ---------------------------------------------------------------------------

function inferDestination(url: string): WebhookDestinationType {
  const u = url.toLowerCase();
  if (u.includes("hooks.slack.com")) return "slack";
  if (u.includes("outlook.office.com/webhook") || u.includes(".office.com"))
    return "teams";
  if (u.includes("service-now.com")) return "servicenow";
  if (u.includes("datadoghq.com") || u.includes("datadoghq.eu")) return "datadog";
  if (u.includes("/services/collector")) return "splunk";
  return "generic";
}

// ---------------------------------------------------------------------------
// Drawer
// ---------------------------------------------------------------------------

export function WebhookDrawer({
  open,
  onOpenChange,
  existing,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  existing: Webhook | null;
}) {
  const { toast } = useToast();
  const qc = useQueryClient();
  const isEdit = !!existing;

  // Catalog drives what destinations are pickable + what config form to
  // render. Fetched once per drawer open; cached at the query layer.
  const { data: catalog } = useQuery({
    queryKey: ["webhook-destinations"],
    queryFn: listWebhookDestinations,
    staleTime: 60_000,
    enabled: open,
  });
  const destinations = catalog?.destinations ?? [];

  const [destType, setDestType] = useState<WebhookDestinationType>("slack");
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");
  const [events, setEvents] = useState<WebhookEventType[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [secretRevealed, setSecretRevealed] = useState(false);
  // config holds the raw values the user has entered. On edit we keep
  // existing-but-redacted ("***") sentinels so the user knows a value
  // is set without us showing it; the backend strips them on merge.
  const [config, setConfig] = useState<Record<string, string>>({});

  const dest = useMemo<WebhookDestinationView | null>(
    () => destinations.find((d) => d.type === destType) ?? null,
    [destinations, destType],
  );
  const visual = visualFor(destType);

  // Rehydrate when opening or when existing changes.
  useEffect(() => {
    if (!open) return;
    if (existing) {
      const inferred = existing.destination_type || inferDestination(existing.url);
      setDestType(inferred);
      setName(existing.name);
      setUrl(existing.url);
      setEvents(existing.events ?? []);
      setEnabled(existing.enabled);
      setSecret(existing.secret ?? "");
      // Surface masked summary so the user sees "set" indicators.
      const summary = (existing.config_summary ?? {}) as Record<string, string | null>;
      const cfg: Record<string, string> = {};
      for (const [k, v] of Object.entries(summary)) {
        if (v == null) continue;
        cfg[k] = String(v);
      }
      setConfig(cfg);
    } else {
      setDestType("slack");
      const v = visualFor("slack");
      setName(v.defaultName);
      setUrl("");
      setEvents([]);
      setEnabled(true);
      setSecret("");
      setConfig({});
    }
    setSecretRevealed(false);
    // events are derived from destination on first apply, see effect
    // below — keep this hook focused on identity hydration so the
    // initial pick can also feed off the catalog when it lands.
  }, [open, existing]);

  // When the catalog arrives in create mode, seed default events from
  // the picked destination so the form is usable without further input.
  useEffect(() => {
    if (isEdit) return;
    if (!dest) return;
    setEvents((cur) => (cur.length === 0 ? dest.default_events ?? [] : cur));
  }, [dest, isEdit]);

  // When the user picks a new destination (create mode), apply its
  // visual + event defaults — but don't clobber fields the user has
  // already edited.
  const applyDestination = (id: WebhookDestinationType) => {
    setDestType(id);
    if (isEdit) return;
    const v = visualFor(id);
    const knownDefaults = Object.values(DESTINATION_VISUALS)
      .map((x) => x.defaultName)
      .filter(Boolean);
    if (!name || knownDefaults.includes(name)) {
      setName(v.defaultName);
    }
    const next = destinations.find((d) => d.type === id);
    setEvents(next?.default_events ?? []);
    setConfig({});
  };

  const setConfigValue = (key: string, value: string) => {
    setConfig((prev) => {
      const next = { ...prev, [key]: value };
      // Some destinations want URL pre-filled from a config selector
      // (Datadog site → events API URL). Run the suggestion on every
      // change but only auto-fill if the user hasn't typed a custom URL.
      if (visual.suggestUrlOn) {
        const suggested = visual.suggestUrlOn(next);
        if (suggested) {
          setUrl((cur) => {
            // Only overwrite if URL is empty or matches a previously
            // suggested value — never trample a hand-typed URL.
            if (!cur) return suggested;
            const allSuggestions = ["us1", "us3", "us5", "eu", "ap1"]
              .map((s) =>
                visual.suggestUrlOn?.({ site: s }) ?? "",
              )
              .filter(Boolean);
            if (allSuggestions.includes(cur)) return suggested;
            return cur;
          });
        }
      }
      return next;
    });
  };

  const mutation = useMutation({
    mutationFn: (body: WebhookUpsertRequest) =>
      existing ? updateWebhook(existing.id, body) : createWebhook(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["webhooks"] });
      toast({ title: existing ? "Subscription updated" : "Subscription created" });
      onOpenChange(false);
    },
    onError: (e: { response?: { data?: { error?: string } }; message?: string }) => {
      toast({
        title: "Save failed",
        description: e?.response?.data?.error ?? e.message,
        variant: "destructive",
      });
    },
  });

  // Required config check — UI mirrors what the backend will enforce.
  const missingRequired = useMemo(() => {
    const keys = dest?.config_keys ?? [];
    return keys
      .filter((k) => k.required)
      .filter((k) => {
        const v = config[k.key];
        return !v || v === "";
      })
      .map((k) => k.label);
  }, [config, dest]);

  const canSubmit =
    name.trim().length > 0 &&
    url.trim().length > 0 &&
    missingRequired.length === 0;

  const handleSubmit = () => {
    if (!canSubmit) {
      toast({
        title: "Missing required fields",
        description:
          missingRequired.length > 0
            ? `Required: ${missingRequired.join(", ")}`
            : "Name and URL are required.",
        variant: "destructive",
      });
      return;
    }
    // Strip masked sentinels — the backend's merge logic does the same
    // server-side, but cleaning here keeps PUT bodies free of "***".
    const cfgOut: Record<string, string> = {};
    for (const [k, v] of Object.entries(config)) {
      if (v === "***") continue;
      if (v === "" && !isEdit) continue;
      cfgOut[k] = v;
    }
    const body: WebhookUpsertRequest = {
      name: name.trim(),
      url: url.trim(),
      events: events.length ? events : undefined,
      enabled,
      destination_type: destType,
      config: Object.keys(cfgOut).length > 0 ? cfgOut : undefined,
    };
    if (!existing && secret.trim()) body.secret = secret.trim();
    mutation.mutate(body);
  };

  const eventGroups = useMemo(() => {
    const g: Record<string, typeof ALL_EVENT_TYPES> = {};
    for (const e of ALL_EVENT_TYPES) {
      (g[e.group] ||= []).push(e);
    }
    return g;
  }, []);

  // Order picker by catalog. Generic last — feels less "default" when
  // it's not at the front. Visual logos handle the rest of the prominence.
  const orderedDestinations = useMemo(() => {
    const order: WebhookDestinationType[] = [
      "slack",
      "teams",
      "splunk",
      "datadog",
      "servicenow",
      "generic",
    ];
    return order
      .map((t) => destinations.find((d) => d.type === t))
      .filter((d): d is WebhookDestinationView => !!d);
  }, [destinations]);

  return (
    <FormDrawer
      open={open}
      onOpenChange={onOpenChange}
      size="lg"
      title={isEdit ? "Edit subscription" : "New subscription"}
      description={
        isEdit
          ? "Rotate the HMAC secret by deleting and re-creating the subscription."
          : "Pipe pLLM events to Slack, Teams, a SIEM, or any HTTPS endpoint. Vendor-native formatting and auth are handled per destination."
      }
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!canSubmit || mutation.isPending}
          >
            {mutation.isPending
              ? "Saving…"
              : isEdit
                ? "Save changes"
                : "Create subscription"}
          </Button>
        </>
      }
    >
      <div className="space-y-6">
        {/* ────────── Destination picker ────────── */}
        {!isEdit && (
          <section className="space-y-2">
            <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
              Destination
            </Label>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {orderedDestinations.map((d) => {
                const v = visualFor(d.type);
                const active = d.type === destType;
                return (
                  <button
                    key={d.type}
                    type="button"
                    onClick={() => applyDestination(d.type)}
                    className={cn(
                      "group rounded-lg border p-3 text-left transition-all",
                      "flex items-center gap-2.5",
                      "hover:border-primary/40",
                      active
                        ? "border-primary ring-1 ring-primary bg-primary/[0.04]"
                        : "border-border",
                    )}
                  >
                    <div
                      className={cn(
                        "h-8 w-8 rounded-md flex items-center justify-center shrink-0",
                        v.colored ? "bg-background" : "bg-muted",
                        v.tint,
                      )}
                    >
                      <Icon icon={v.icon} className="h-5 w-5" />
                    </div>
                    <span
                      className={cn(
                        "text-[12px] font-medium leading-tight truncate",
                        active ? "text-foreground" : "text-muted-foreground",
                      )}
                    >
                      {d.label}
                    </span>
                  </button>
                );
              })}
            </div>
          </section>
        )}

        {isEdit && (
          <section className="rounded-lg border border-border bg-muted/20 p-3 flex items-center gap-3">
            <div
              className={cn(
                "h-10 w-10 rounded-md flex items-center justify-center shrink-0",
                visual.colored ? "bg-background" : "bg-muted",
                visual.tint,
              )}
            >
              <Icon icon={visual.icon} className="h-6 w-6" />
            </div>
            <div className="min-w-0">
              <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                Destination
              </div>
              <div className="text-sm font-semibold">
                {dest?.label ?? destType}
              </div>
            </div>
          </section>
        )}

        {/* ────────── Name ────────── */}
        <section className="space-y-2">
          <Label htmlFor="wh-name">Name</Label>
          <Input
            id="wh-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={visual.defaultName || "My webhook"}
          />
        </section>

        {/* ────────── URL ────────── */}
        <section className="space-y-2">
          <Label htmlFor="wh-url">Endpoint URL</Label>
          <Input
            id="wh-url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={visual.urlPlaceholder}
            className="font-mono text-xs"
          />
          {visual.urlHint && (
            <p className="text-[11px] text-muted-foreground flex items-center gap-1">
              <Icon icon={icons.info} className="h-3 w-3" />
              {visual.urlHint}
            </p>
          )}
        </section>

        {/* ────────── Per-destination config form ────────── */}
        {dest?.config_keys && dest.config_keys.length > 0 && (
          <section className="space-y-3 rounded-lg border border-border p-3 bg-muted/10">
            <div className="flex items-center justify-between">
              <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
                {dest.label} credentials
              </Label>
              {isEdit && (
                <span
                  className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground"
                  title="Leave a credential blank to keep the existing value."
                >
                  edit-only fields
                </span>
              )}
            </div>
            {dest.config_keys.map((k) => (
              <ConfigField
                key={k.key}
                schema={k}
                value={config[k.key] ?? ""}
                onChange={(v) => setConfigValue(k.key, v)}
                isEdit={isEdit}
              />
            ))}
          </section>
        )}

        {/* ────────── Generic-only HMAC secret ────────── */}
        {!isEdit && destType === "generic" && (
          <section className="space-y-2">
            <Label htmlFor="wh-secret" className="flex items-center gap-2">
              HMAC secret
              <span className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                auto-generated if blank
              </span>
            </Label>
            <div className="relative">
              <Input
                id="wh-secret"
                type={secretRevealed ? "text" : "password"}
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder="(auto-generate a 32-byte hex secret)"
                className="font-mono text-xs pr-20"
              />
              <div className="absolute right-1 top-1/2 -translate-y-1/2 flex items-center">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => setSecretRevealed((v) => !v)}
                  title={secretRevealed ? "Hide" : "Show"}
                >
                  <Icon
                    icon={secretRevealed ? icons.eyeOff : icons.eye}
                    className="h-3.5 w-3.5"
                  />
                </Button>
              </div>
            </div>
            <p className="text-[11px] text-muted-foreground">
              Receivers verify with{" "}
              <code className="font-mono text-foreground/80">
                X-PLLM-Signature: sha256=HMAC(body, secret)
              </code>
            </p>
          </section>
        )}

        {/* ────────── Events ────────── */}
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <Label className="text-[11px] font-mono uppercase tracking-widest text-muted-foreground">
              Events
            </Label>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() =>
                  setEvents(ALL_EVENT_TYPES.map((e) => e.value))
                }
                className="text-[11px] text-muted-foreground hover:text-foreground"
              >
                All
              </button>
              <span className="text-muted-foreground/40">·</span>
              <button
                type="button"
                onClick={() => setEvents(dest?.default_events ?? [])}
                className="text-[11px] text-muted-foreground hover:text-foreground"
                title="Restore the destination's smart defaults"
              >
                Defaults
              </button>
              <span className="text-muted-foreground/40">·</span>
              <button
                type="button"
                onClick={() => setEvents([])}
                className="text-[11px] text-muted-foreground hover:text-foreground"
              >
                None
              </button>
            </div>
          </div>
          {events.length === 0 && (
            <p className="text-[11px] text-amber-600 dark:text-amber-400 flex items-center gap-1.5">
              <Icon icon={icons.warning} className="h-3 w-3" />
              Empty subscription — receiver will get every event type.
            </p>
          )}
          {Object.entries(eventGroups).map(([group, entries]) => (
            <div key={group}>
              <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground mb-1.5">
                {group}
              </div>
              <div className="space-y-1">
                {entries.map((e) => {
                  const on = events.includes(e.value);
                  return (
                    <label
                      key={e.value}
                      className={cn(
                        "flex items-start gap-2 rounded-md border px-2.5 py-2 cursor-pointer transition-colors",
                        on
                          ? "border-primary/40 bg-primary/[0.04]"
                          : "border-border hover:bg-muted/40",
                      )}
                    >
                      <input
                        type="checkbox"
                        checked={on}
                        onChange={() =>
                          setEvents((prev) =>
                            on
                              ? prev.filter((v) => v !== e.value)
                              : [...prev, e.value],
                          )
                        }
                        className="h-4 w-4 mt-0.5 shrink-0"
                      />
                      <div className="min-w-0 flex-1">
                        <code className="text-[12px] font-mono block truncate">
                          {e.label}
                        </code>
                        <p className="text-[11px] text-muted-foreground">
                          {e.hint}
                        </p>
                      </div>
                    </label>
                  );
                })}
              </div>
            </div>
          ))}
        </section>

        {/* ────────── Enabled switch ────────── */}
        <section className="rounded-lg border border-border bg-muted/20 p-3 flex items-center justify-between">
          <div>
            <Label htmlFor="wh-enabled" className="cursor-pointer text-sm">
              Deliver events
            </Label>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              Disable to pause without losing the subscription.
            </p>
          </div>
          <Switch
            id="wh-enabled"
            checked={enabled}
            onCheckedChange={setEnabled}
          />
        </section>
      </div>
    </FormDrawer>
  );
}

// ---------------------------------------------------------------------------
// ConfigField — renders one entry from the destination's config_keys
// schema. Sensitive values arrive as "***" on edit; we treat that as
// "value already set, leave alone unless user retypes" so the backend
// can keep the persisted credential.
// ---------------------------------------------------------------------------

function ConfigField({
  schema,
  value,
  onChange,
  isEdit,
}: {
  schema: WebhookDestinationConfigKey;
  value: string;
  onChange: (v: string) => void;
  isEdit: boolean;
}) {
  const masked = value === "***";
  const placeholder = schema.required
    ? `${schema.label} (required)`
    : schema.label;
  const inputId = `wh-cfg-${schema.key}`;

  if (schema.type === "select") {
    const options = schema.options ?? [];
    return (
      <div className="space-y-1.5">
        <Label htmlFor={inputId} className="text-[11px] font-medium">
          {schema.label}
          {schema.required && (
            <span className="text-rose-500 ml-1">*</span>
          )}
        </Label>
        <select
          id={inputId}
          value={value || schema.default || options[0] || ""}
          onChange={(e) => onChange(e.target.value)}
          className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
        >
          {options.map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
        {schema.help && (
          <p className="text-[11px] text-muted-foreground">{schema.help}</p>
        )}
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      <Label htmlFor={inputId} className="text-[11px] font-medium flex items-center gap-1.5">
        {schema.label}
        {schema.required && <span className="text-rose-500">*</span>}
        {isEdit && masked && (
          <span className="ml-auto text-[10px] font-mono uppercase tracking-widest text-emerald-600 dark:text-emerald-400 inline-flex items-center gap-1">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            stored
          </span>
        )}
      </Label>
      <Input
        id={inputId}
        type={schema.type === "password" ? "password" : "text"}
        value={masked ? "" : value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={masked ? "(leave blank to keep stored value)" : placeholder}
        className="font-mono text-xs"
        autoComplete={schema.type === "password" ? "new-password" : "off"}
      />
      {schema.help && (
        <p className="text-[11px] text-muted-foreground">{schema.help}</p>
      )}
    </div>
  );
}
