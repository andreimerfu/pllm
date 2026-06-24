package adapters

import (
	"github.com/amerfu/pllm/internal/core/models"
)

// registry holds one instance of each adapter, indexed by destination
// type. Adapters are stateless so a singleton per type is fine. New
// destinations register themselves at package init.
var registry = map[models.WebhookDestinationType]Adapter{}

// register slots an adapter into the registry. Called from each
// adapter's init function.
func register(a Adapter) {
	if a == nil {
		return
	}
	registry[a.Type()] = a
}

// For looks up the adapter for a destination type. Falls back to
// Generic when the type is empty (backwards compat for rows created
// before the destination_type column landed) or unknown (defensive —
// new types coming from a newer admin UI before the binary is updated).
func For(t models.WebhookDestinationType) Adapter {
	if t == "" {
		t = models.WebhookDestGeneric
	}
	if a, ok := registry[t]; ok {
		return a
	}
	return registry[models.WebhookDestGeneric]
}

// All returns a stable-ordered list of registered adapters. Used by the
// /api/admin/webhooks/destinations endpoint so the UI can render a
// data-driven picker without hardcoding the catalog twice.
func All() []Adapter {
	// Stable order matches the UI grouping: native first, vendor SaaS
	// next, SIEM/GRC last. Keeps the picker reading sensibly.
	order := []models.WebhookDestinationType{
		models.WebhookDestGeneric,
		models.WebhookDestSlack,
		models.WebhookDestTeams,
		models.WebhookDestSplunk,
		models.WebhookDestDatadog,
		models.WebhookDestServiceNow,
	}
	out := make([]Adapter, 0, len(order))
	for _, t := range order {
		if a, ok := registry[t]; ok {
			out = append(out, a)
		}
	}
	return out
}
