import { useEffect, useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Icon } from "@iconify/react";

import { cn } from "@/lib/utils";
import { icons } from "@/lib/icons";
import { createWebhook, updateWebhook } from "@/lib/api";
import type {
  Webhook,
  WebhookEventType,
  WebhookUpsertRequest,
} from "@/types/api";

import { FormDrawer } from "@/components/common/FormDrawer";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/hooks/use-toast";

// ---------------------------------------------------------------------------
// Destination presets — what pllm webhooks commonly point at.
//
// Each preset supplies:
//   • brand logo (Iconify id from logos: / simple-icons:)
//   • URL placeholder + (optional) regex hint
//   • a suggested default event subscription
//   • suggested name, inserted only when the user hasn't typed one
//
// Picking a preset is purely convenience — the backend doesn't care, and
// "Generic" leaves every field free.
// ---------------------------------------------------------------------------

type Destination = {
  id: string;
  label: string;
  icon: string;
  tint: string;
  urlPlaceholder: string;
  urlHint?: string;
  defaultName: string;
  defaultEvents: WebhookEventType[] | null; // null = all
  colored: boolean; // true when icon is a full-color logos:* glyph
};

const DESTINATIONS: Destination[] = [
  {
    id: "slack",
    label: "Slack",
    icon: "logos:slack-icon",
    tint: "",
    urlPlaceholder: "https://hooks.slack.com/services/T…/B…/…",
    urlHint: "Incoming webhook URL from Slack app config",
    defaultName: "Slack — #compliance",
    defaultEvents: ["scope.denied", "scope.flagged", "use_case.compliance_flag"],
    colored: true,
  },
  {
    id: "teams",
    label: "Microsoft Teams",
    icon: "logos:microsoft-teams",
    tint: "",
    urlPlaceholder: "https://outlook.office.com/webhook/…",
    urlHint: "Teams connector URL",
    defaultName: "Teams — compliance",
    defaultEvents: ["scope.denied", "use_case.compliance_flag"],
    colored: true,
  },
  {
    id: "pagerduty",
    label: "PagerDuty",
    icon: "logos:pagerduty-icon",
    tint: "",
    urlPlaceholder: "https://events.pagerduty.com/v2/enqueue",
    urlHint: "Events API v2 endpoint",
    defaultName: "PagerDuty",
    defaultEvents: ["scope.denied"],
    colored: true,
  },
  {
    id: "servicenow",
    label: "ServiceNow",
    icon: "simple-icons:servicenow",
    tint: "text-emerald-600 dark:text-emerald-400",
    urlPlaceholder: "https://<instance>.service-now.com/api/now/table/…",
    urlHint: "Scripted REST or table API endpoint",
    defaultName: "ServiceNow — GRC",
    defaultEvents: ["use_case.compliance_flag", "scope.denied"],
    colored: false,
  },
  {
    id: "datadog",
    label: "Datadog",
    icon: "logos:datadog",
    tint: "",
    urlPlaceholder: "https://api.datadoghq.com/api/v1/events",
    defaultName: "Datadog — events",
    defaultEvents: null,
    colored: true,
  },
  {
    id: "splunk",
    label: "Splunk",
    icon: "logos:splunk",
    tint: "",
    urlPlaceholder: "https://<host>:8088/services/collector",
    urlHint: "HTTP Event Collector (HEC)",
    defaultName: "Splunk — HEC",
    defaultEvents: null,
    colored: true,
  },
  {
    id: "discord",
    label: "Discord",
    icon: "logos:discord-icon",
    tint: "",
    urlPlaceholder: "https://discord.com/api/webhooks/…",
    defaultName: "Discord",
    defaultEvents: ["scope.denied", "scope.flagged"],
    colored: true,
  },
  {
    id: "generic",
    label: "Generic webhook",
    icon: icons.link,
    tint: "text-muted-foreground",
    urlPlaceholder: "https://example.com/hooks/pllm",
    defaultName: "",
    defaultEvents: null,
    colored: false,
  },
];

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
// Destination inference for edit mode — guess the preset by URL
// ---------------------------------------------------------------------------

function inferDestination(url: string): string {
  const u = url.toLowerCase();
  if (u.includes("hooks.slack.com")) return "slack";
  if (u.includes("outlook.office.com/webhook") || u.includes("office.com"))
    return "teams";
  if (u.includes("events.pagerduty.com")) return "pagerduty";
  if (u.includes("service-now.com")) return "servicenow";
  if (u.includes("datadoghq.com")) return "datadog";
  if (u.includes("/services/collector")) return "splunk";
  if (u.includes("discord.com/api/webhooks")) return "discord";
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

  const [destId, setDestId] = useState<string>("slack");
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");
  const [events, setEvents] = useState<WebhookEventType[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [secretRevealed, setSecretRevealed] = useState(false);

  const dest = useMemo(
    () => DESTINATIONS.find((d) => d.id === destId) ?? DESTINATIONS[0],
    [destId],
  );

  // Rehydrate when opening or when existing changes.
  useEffect(() => {
    if (!open) return;
    if (existing) {
      const inferred = inferDestination(existing.url);
      setDestId(inferred);
      setName(existing.name);
      setUrl(existing.url);
      setEvents(existing.events ?? []);
      setEnabled(existing.enabled);
      setSecret(existing.secret ?? "");
    } else {
      setDestId("slack");
      const d = DESTINATIONS[0];
      setName(d.defaultName);
      setUrl("");
      setEvents(d.defaultEvents ?? []);
      setEnabled(true);
      setSecret("");
    }
    setSecretRevealed(false);
  }, [open, existing]);

  // When the user picks a new destination (create mode), apply its defaults
  // — but don't clobber fields the user has already edited.
  const applyDestination = (id: string) => {
    setDestId(id);
    if (isEdit) return;
    const d = DESTINATIONS.find((x) => x.id === id) ?? DESTINATIONS[0];
    // Replace the name only if the user hasn't typed a custom one, OR if
    // it still matches a prior preset's default name.
    const prevDefaults = DESTINATIONS.map((x) => x.defaultName).filter(Boolean);
    if (!name || prevDefaults.includes(name)) {
      setName(d.defaultName);
    }
    setEvents(d.defaultEvents ?? []);
  };

  const mutation = useMutation({
    mutationFn: (body: WebhookUpsertRequest) =>
      existing ? updateWebhook(existing.id, body) : createWebhook(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["webhooks"] });
      toast({ title: existing ? "Webhook updated" : "Webhook created" });
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

  const canSubmit = name.trim().length > 0 && url.trim().length > 0;

  const handleSubmit = () => {
    if (!canSubmit) {
      toast({ title: "Name and URL are required", variant: "destructive" });
      return;
    }
    const body: WebhookUpsertRequest = {
      name: name.trim(),
      url: url.trim(),
      events: events.length ? events : undefined,
      enabled,
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

  return (
    <FormDrawer
      open={open}
      onOpenChange={onOpenChange}
      size="lg"
      title={isEdit ? "Edit webhook" : "New webhook"}
      description={
        isEdit
          ? "Rotate the secret by deleting and re-creating."
          : "Pipe scope events to Slack, PagerDuty, a SIEM — anything that accepts HMAC-signed POSTs."
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
            {mutation.isPending ? "Saving…" : isEdit ? "Save changes" : "Create webhook"}
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
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {DESTINATIONS.map((d) => {
                const active = d.id === destId;
                return (
                  <button
                    key={d.id}
                    type="button"
                    onClick={() => applyDestination(d.id)}
                    className={cn(
                      "group rounded-lg border p-3 text-left transition-all",
                      "flex flex-col items-center gap-2",
                      "hover:border-primary/40",
                      active
                        ? "border-primary ring-1 ring-primary bg-primary/[0.04]"
                        : "border-border",
                    )}
                  >
                    <div
                      className={cn(
                        "h-8 w-8 rounded-md flex items-center justify-center shrink-0",
                        d.colored ? "bg-background" : "bg-muted",
                        d.tint,
                      )}
                    >
                      <Icon icon={d.icon} className="h-5 w-5" />
                    </div>
                    <span
                      className={cn(
                        "text-[11px] font-medium text-center leading-tight",
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
                dest.colored ? "bg-background" : "bg-muted",
                dest.tint,
              )}
            >
              <Icon icon={dest.icon} className="h-6 w-6" />
            </div>
            <div className="min-w-0">
              <div className="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                Destination
              </div>
              <div className="text-sm font-semibold">{dest.label}</div>
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
            placeholder={dest.defaultName || "My webhook"}
          />
        </section>

        {/* ────────── URL ────────── */}
        <section className="space-y-2">
          <Label htmlFor="wh-url">Endpoint URL</Label>
          <Input
            id="wh-url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={dest.urlPlaceholder}
            className="font-mono text-xs"
          />
          {dest.urlHint && (
            <p className="text-[11px] text-muted-foreground flex items-center gap-1">
              <Icon icon={icons.info} className="h-3 w-3" />
              {dest.urlHint}
            </p>
          )}
        </section>

        {/* ────────── Secret ────────── */}
        {!isEdit && (
          <section className="space-y-2">
            <Label htmlFor="wh-secret" className="flex items-center gap-2">
              Secret
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
              Empty subscription — you'll get every event type.
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
