package models

import "testing"

func TestSubscribesAcceptsLifecycleEvents(t *testing.T) {
	w := &Webhook{Events: []string{
		string(WebhookEventKeyRevokedInactive),
		string(WebhookEventKeyRevokedOwnerless),
		string(WebhookEventRegistryServerBlockedRisky),
	}}
	for _, evt := range []WebhookEventType{
		WebhookEventKeyRevokedInactive,
		WebhookEventKeyRevokedOwnerless,
		WebhookEventRegistryServerBlockedRisky,
	} {
		if !w.Subscribes(evt) {
			t.Fatalf("expected webhook to subscribe to %q", evt)
		}
	}
}

func TestRegistryStatusBlockedDefined(t *testing.T) {
	if RegistryStatusBlocked != "blocked" {
		t.Fatalf("expected RegistryStatusBlocked == \"blocked\", got %q", RegistryStatusBlocked)
	}
}

func TestAuditEventLifecycleDefined(t *testing.T) {
	if AuditEventLifecycle != "lifecycle" {
		t.Fatalf("expected AuditEventLifecycle == \"lifecycle\", got %q", AuditEventLifecycle)
	}
}
