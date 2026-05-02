package seed

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// generateBankEvents produces ~8 events spread across the previous
// 7 days. Distribution is hand-tuned for the demo's posture story:
// 5 denied concentrated in the last 24h on the loan use case (drives
// "Use cases with most denials" + "Most-denied targets" panels), 3
// flagged spread over 3 days on AML (flag mode means logged-not-blocked).
//
// Returns ready-to-POST event payloads (use_case_id is already
// resolved by the caller from the seeded slugs).
func generateBankEvents(ucIDs map[string]uuid.UUID) []map[string]any {
	now := time.Now().UTC()
	events := []map[string]any{}

	loanID, hasLoan := ucIDs["loan-underwriting-assistant"]
	amlID, hasAML := ucIDs["aml-investigation-copilot"]

	// 5 denied on loan-underwriting yesterday — operator's "recent" story
	if hasLoan {
		offsets := []time.Duration{24, 22, 20, 18, 14}
		for _, h := range offsets {
			events = append(events, map[string]any{
				"use_case_id": loanID,
				"kind":        "llm_model",
				"outcome":     "denied",
				"target_name": "gpt-4-turbo",
				"key_name":    "demo-loan-officer",
				"reason":      "model gpt-4-turbo is not declared by the bound use case",
				"remote_addr": "10.42.0.7",
				"created_at":  now.Add(-h * time.Hour).Format(time.RFC3339),
			})
		}
	}

	// 3 flagged on AML — flag mode means logged-not-blocked
	if hasAML {
		offsets := []time.Duration{6, 30, 70} // hours ago
		for _, h := range offsets {
			events = append(events, map[string]any{
				"use_case_id": amlID,
				"kind":        "mcp_tool",
				"outcome":     "flagged",
				"target_name": "tavily-search__search",
				"key_name":    "demo-aml-analyst",
				"reason":      "tool tavily-search__search is not declared by the bound use case",
				"remote_addr": "10.42.0.8",
				"created_at":  now.Add(-h * time.Hour).Format(time.RFC3339),
			})
		}
	}

	return events
}

// pushEvents ships the events through the bulk endpoint.
func pushEvents(c *Client, events []map[string]any) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	body := map[string]any{"events": events}
	var got struct {
		Inserted int `json:"inserted"`
	}
	status, b, err := c.Do(http.MethodPost, "/api/admin/use-cases/events:bulk", body, &got)
	if err != nil {
		return 0, err
	}
	if err := MustOK(http.MethodPost, "/api/admin/use-cases/events:bulk", status, b); err != nil {
		return 0, err
	}
	return got.Inserted, nil
}

// upsertWebhook ensures a single demo webhook subscription exists,
// disabled by default. Operator flips it on after replacing the URL
// with their own webhook.site / SIEM endpoint.
func upsertWebhook(c *Client, url string) error {
	type webhookRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type listResp struct {
		Webhooks []webhookRow `json:"webhooks"`
	}
	var existing listResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/webhooks", nil, &existing)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/webhooks", status, body); err != nil {
		return err
	}
	const demoName = "demo-siem-stub"
	payload := map[string]any{
		"name":    demoName,
		"url":     url,
		"events":  []string{"scope.denied", "scope.flagged", "use_case.created", "use_case.updated", "use_case.compliance_flag"},
		"enabled": false,
	}
	for _, w := range existing.Webhooks {
		if w.Name == demoName {
			st, b, err := c.Do(http.MethodPut, "/api/admin/webhooks/"+w.ID.String(), payload, nil)
			if err != nil {
				return err
			}
			return MustOK(http.MethodPut, "/api/admin/webhooks/"+w.ID.String(), st, b)
		}
	}
	st, b, err := c.Do(http.MethodPost, "/api/admin/webhooks", payload, nil)
	if err != nil {
		return err
	}
	return MustOK(http.MethodPost, "/api/admin/webhooks", st, b)
}

// _ keeps fmt referenced in case future event helpers grow.
var _ = fmt.Sprintf
