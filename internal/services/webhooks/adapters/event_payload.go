package adapters

import (
	"fmt"
	"strings"

	"github.com/amerfu/pllm/internal/core/models"
)

// Severity classifies an event for vendor surfaces that distinguish
// info / warning / error styling. Maps to Slack accent color, Teams
// card theme color, Datadog alert_type, etc.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// SeverityOf classifies an event by type. Block-mode denials and
// risky-server blocks are errors; flag-mode and compliance flags are
// warnings; everything else is informational.
func SeverityOf(t models.WebhookEventType) Severity {
	switch t {
	case models.WebhookEventScopeDenied,
		models.WebhookEventRegistryServerBlockedRisky,
		models.WebhookEventDeploymentOOMCapReached,
		models.WebhookEventKeyRevokedOwnerless,
		models.WebhookEventFederationSyncFailed:
		return SeverityError
	case models.WebhookEventScopeFlagged,
		models.WebhookEventUseCaseComplianceFlag,
		models.WebhookEventDeploymentOOMBumped,
		models.WebhookEventKeyRevokedInactive,
		models.WebhookEventFederationItemOrphaned:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

// Summary is a human-readable view of an event derived from the well-
// known keys vendor adapters render into chat/ticket surfaces. The
// Generic adapter ignores this and forwards the raw envelope.
type Summary struct {
	// Title is one short line — fits a chat header, Slack message
	// header block, or ServiceNow short_description.
	Title string
	// Body is a longer paragraph form for Teams / ServiceNow long
	// description / Datadog text.
	Body string
	// Fields are key/value pairs for Slack section blocks and Teams
	// FactSet rows. Order is meaningful — the most relevant key
	// (target, then use case, then reason) comes first.
	Fields []SummaryField
	Severity Severity
}

// SummaryField is one key/value row.
type SummaryField struct {
	Label string
	Value string
}

// SummaryFor renders an Event into a vendor-neutral Summary by reading
// well-known keys from the data map. Unknown event shapes fall back to
// a generic "Event of type X" line so subscribers always see *something*
// rather than an empty alert.
func SummaryFor(evt Event) Summary {
	data := dataMap(evt.Data)
	sev := SeverityOf(evt.Type)
	switch evt.Type {
	case models.WebhookEventScopeDenied, models.WebhookEventScopeFlagged:
		return scopeEventSummary(evt, data, sev)
	case models.WebhookEventUseCaseCreated,
		models.WebhookEventUseCaseUpdated,
		models.WebhookEventUseCaseDeleted,
		models.WebhookEventUseCaseComplianceFlag:
		return useCaseEventSummary(evt, data, sev)
	case models.WebhookEventRegistryServerSaved,
		models.WebhookEventRegistryServerBlockedRisky:
		return registryEventSummary(evt, data, sev)
	case models.WebhookEventKeyRevokedInactive,
		models.WebhookEventKeyRevokedOwnerless,
		models.WebhookEventLifecycleCustom:
		return lifecycleEventSummary(evt, data, sev)
	case models.WebhookEventFederationSyncCompleted,
		models.WebhookEventFederationSyncFailed,
		models.WebhookEventFederationItemOrphaned:
		return federationEventSummary(evt, data, sev)
	default:
		return Summary{
			Title:    fmt.Sprintf("pLLM event: %s", evt.Type),
			Body:     fmt.Sprintf("Event id %s — see receiver logs for full payload.", evt.ID),
			Severity: sev,
		}
	}
}

// federationEventSummary renders federation sync events. Payloads come
// from federation.SyncResult; only well-known top-level keys are read
// here so adapter rendering stays stable across schema additions.
func federationEventSummary(evt Event, data map[string]any, sev Severity) Summary {
	peer := stringValue(data, "peer_id")
	verb := "completed"
	switch evt.Type {
	case models.WebhookEventFederationSyncFailed:
		verb = "failed"
	case models.WebhookEventFederationItemOrphaned:
		verb = "orphaned items"
	}
	title := fmt.Sprintf("Federation sync %s — %s", verb, defaultIfEmpty(peer, "(unknown peer)"))
	body := fmt.Sprintf("Peer %q sync %s.", defaultIfEmpty(peer, "(unknown)"), verb)
	if orphans := stringValue(data, "orphaned"); orphans != "" {
		body += " Orphaned: " + orphans + "."
	}
	return Summary{
		Title:    title,
		Body:     body,
		Severity: sev,
		Fields: nonEmptyFields(
			SummaryField{"Peer", peer},
			SummaryField{"Generation", stringValue(data, "generation")},
			SummaryField{"Orphaned", stringValue(data, "orphaned")},
		),
	}
}

func scopeEventSummary(evt Event, data map[string]any, sev Severity) Summary {
	target := stringValue(data, "target_name", "target")
	useCase := firstNonEmpty(stringValue(data, "use_case_name"), stringValue(data, "use_case_id"))
	reason := stringValue(data, "reason")
	keyName := stringValue(data, "key_name")
	kind := stringValue(data, "kind")
	verb := "blocked"
	if evt.Type == models.WebhookEventScopeFlagged {
		verb = "flagged"
	}
	title := fmt.Sprintf("Scope %s — %s", verb, defaultIfEmpty(target, kind))
	body := fmt.Sprintf(
		"A request from key %q to %s %q was %s because the bound use case %q does not declare it.",
		defaultIfEmpty(keyName, "(unknown key)"),
		defaultIfEmpty(kind, "target"),
		defaultIfEmpty(target, "(unknown)"),
		verb,
		defaultIfEmpty(useCase, "(unscoped)"),
	)
	if reason != "" {
		body += " Reason: " + reason
	}
	return Summary{
		Title:    title,
		Body:     body,
		Severity: sev,
		Fields: nonEmptyFields(
			SummaryField{"Target", target},
			SummaryField{"Kind", kind},
			SummaryField{"Use case", useCase},
			SummaryField{"Key", keyName},
			SummaryField{"Reason", reason},
		),
	}
}

func useCaseEventSummary(evt Event, data map[string]any, sev Severity) Summary {
	name := firstNonEmpty(stringValue(data, "name"), stringValue(data, "use_case_name"), stringValue(data, "id"))
	verb := strings.TrimPrefix(string(evt.Type), "use_case.")
	title := fmt.Sprintf("Use case %s — %s", verb, defaultIfEmpty(name, "(unnamed)"))
	body := fmt.Sprintf("Use case %q has been %s.", defaultIfEmpty(name, "(unnamed)"), verb)
	if reason := stringValue(data, "reason"); reason != "" {
		body += " Reason: " + reason
	}
	return Summary{
		Title:    title,
		Body:     body,
		Severity: sev,
		Fields: nonEmptyFields(
			SummaryField{"Use case", name},
			SummaryField{"Status", stringValue(data, "status")},
			SummaryField{"Owner", stringValue(data, "owner")},
		),
	}
}

func registryEventSummary(evt Event, data map[string]any, sev Severity) Summary {
	name := firstNonEmpty(stringValue(data, "server_name"), stringValue(data, "name"))
	version := stringValue(data, "version")
	verb := "saved"
	if evt.Type == models.WebhookEventRegistryServerBlockedRisky {
		verb = "blocked"
	}
	title := fmt.Sprintf("Registry server %s — %s", verb, defaultIfEmpty(name, "(unnamed)"))
	body := fmt.Sprintf("Server %q (version %s) has been %s.", defaultIfEmpty(name, "(unnamed)"), defaultIfEmpty(version, "—"), verb)
	if reason := stringValue(data, "reason"); reason != "" {
		body += " " + reason
	}
	return Summary{
		Title:    title,
		Body:     body,
		Severity: sev,
		Fields: nonEmptyFields(
			SummaryField{"Server", name},
			SummaryField{"Version", version},
			SummaryField{"Score", stringValue(data, "score")},
			SummaryField{"Reason", stringValue(data, "reason")},
		),
	}
}

func lifecycleEventSummary(evt Event, data map[string]any, sev Severity) Summary {
	subject := firstNonEmpty(stringValue(data, "subject"), stringValue(data, "name"), stringValue(data, "id"))
	rule := stringValue(data, "rule")
	action := firstNonEmpty(stringValue(data, "action"), strings.TrimPrefix(string(evt.Type), "lifecycle."))
	title := fmt.Sprintf("Policy %s — %s", action, defaultIfEmpty(subject, "(unspecified)"))
	body := fmt.Sprintf("Lifecycle policy %q took action %q on %q.", defaultIfEmpty(rule, "(builtin)"), action, defaultIfEmpty(subject, "(unspecified)"))
	if reason := stringValue(data, "reason"); reason != "" {
		body += " Reason: " + reason
	}
	return Summary{
		Title:    title,
		Body:     body,
		Severity: sev,
		Fields: nonEmptyFields(
			SummaryField{"Rule", rule},
			SummaryField{"Action", action},
			SummaryField{"Subject", subject},
			SummaryField{"Reason", stringValue(data, "reason")},
		),
	}
}

// --- small helpers --------------------------------------------------------

func dataMap(d any) map[string]any {
	if m, ok := d.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func stringValue(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultIfEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func nonEmptyFields(fs ...SummaryField) []SummaryField {
	out := make([]SummaryField, 0, len(fs))
	for _, f := range fs {
		if f.Value != "" {
			out = append(out, f)
		}
	}
	return out
}
