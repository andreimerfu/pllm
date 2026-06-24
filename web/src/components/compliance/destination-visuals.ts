// Visual metadata for webhook destinations. The list of destinations
// itself is data-driven from the backend
// (/api/admin/webhooks/destinations) so adding a new vendor adapter is
// one Go change. This module only carries the per-vendor logo + tinting
// + URL hint that doesn't make sense to surface from Go.

import { icons } from "@/lib/icons";
import type { WebhookDestinationType } from "@/types/api";

export type DestinationVisual = {
  icon: string;
  // colored=true uses logos:* glyph on a neutral background; false uses
  // a tinted icon on muted background (no first-party Solar logo
  // available for some vendors).
  colored: boolean;
  tint?: string;
  label: string;
  urlPlaceholder: string;
  urlHint?: string;
  defaultName: string;
  // suggestUrlOn lets a config-key change pre-fill the URL for vendors
  // whose endpoint depends on a config selector (e.g. Datadog site).
  suggestUrlOn?: (cfg: Record<string, string>) => string | null;
};

export const DESTINATION_VISUALS: Record<WebhookDestinationType, DestinationVisual> = {
  generic: {
    icon: icons.link,
    colored: false,
    tint: "text-muted-foreground",
    label: "Generic",
    urlPlaceholder: "https://example.com/hooks/pllm",
    urlHint: "Any HTTPS endpoint that accepts an HMAC-signed JSON POST.",
    defaultName: "",
  },
  slack: {
    icon: "logos:slack-icon",
    colored: true,
    label: "Slack",
    urlPlaceholder: "https://hooks.slack.com/services/T…/B…/…",
    urlHint: "Incoming webhook URL from Slack app config.",
    defaultName: "Slack — #compliance",
  },
  teams: {
    icon: "logos:microsoft-teams",
    colored: true,
    label: "Teams",
    urlPlaceholder:
      "https://outlook.office.com/webhook/… or https://….office.com/workflows/…",
    urlHint: "Incoming webhook or Workflow connector URL.",
    defaultName: "Teams — compliance",
  },
  splunk: {
    icon: "logos:splunk",
    colored: true,
    label: "Splunk",
    urlPlaceholder: "https://splunk.example.com:8088/services/collector/event",
    urlHint:
      "HEC endpoint — usually port 8088, path /services/collector/event.",
    defaultName: "Splunk — HEC",
  },
  datadog: {
    icon: "logos:datadog",
    colored: true,
    label: "Datadog",
    urlPlaceholder: "https://api.datadoghq.com/api/v1/events",
    urlHint:
      "Events API v1 endpoint. The site selector below pre-fills this.",
    defaultName: "Datadog — events",
    suggestUrlOn: (cfg) => {
      const site = (cfg.site || "us1").toLowerCase();
      switch (site) {
        case "us1":
          return "https://api.datadoghq.com/api/v1/events";
        case "us3":
          return "https://api.us3.datadoghq.com/api/v1/events";
        case "us5":
          return "https://api.us5.datadoghq.com/api/v1/events";
        case "eu":
          return "https://api.datadoghq.eu/api/v1/events";
        case "ap1":
          return "https://api.ap1.datadoghq.com/api/v1/events";
      }
      return null;
    },
  },
  servicenow: {
    icon: "simple-icons:servicenow",
    colored: false,
    tint: "text-emerald-600 dark:text-emerald-400",
    label: "ServiceNow",
    urlPlaceholder:
      "https://<instance>.service-now.com/api/now/table/incident",
    urlHint:
      "Table API endpoint for the target table. Custom u_pllm_* fields are set on the row.",
    defaultName: "ServiceNow — GRC",
  },
};

const DEFAULT_VISUAL = DESTINATION_VISUALS.generic;

export function visualFor(t: WebhookDestinationType | undefined): DestinationVisual {
  if (!t) return DEFAULT_VISUAL;
  return DESTINATION_VISUALS[t] ?? DEFAULT_VISUAL;
}
