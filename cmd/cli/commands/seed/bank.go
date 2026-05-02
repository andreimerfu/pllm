package seed

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// Options controls a single seed run.
type Options struct {
	BaseURL         string
	MasterKey       string
	LLMRoute        string
	WithDeployments bool
	WithEvents      bool
	WithWebhook     bool
	WebhookURL      string
	JSONOutput      bool
}

// RunBank seeds the bank demo scenario. Idempotent — re-runs update
// existing fixtures rather than failing.
func RunBank(opts Options) error {
	c := NewClient(opts.BaseURL, opts.MasterKey)
	out := newPrinter(opts.JSONOutput)

	out.Step("verify gateway + master key")
	if err := c.HealthCheck(); err != nil {
		return err
	}

	out.Step("verify LLM route exists: " + opts.LLMRoute)
	if err := ensureRoute(c, opts.LLMRoute); err != nil {
		return err
	}

	out.Step("upsert servers (catalog)")
	for _, s := range BankServers {
		if err := upsertServer(c, s); err != nil {
			return fmt.Errorf("server %s: %w", s.Name, err)
		}
		out.Item(s.Name)
	}

	if opts.WithDeployments {
		out.Step("upsert servers (live, catalog only)")
		for _, s := range PublicLiveServers {
			if err := upsertServer(c, s); err != nil {
				return fmt.Errorf("server %s: %w", s.Name, err)
			}
			out.Item(s.Name + " (catalog only — click Deploy in /ui/deployments)")
		}
	}

	out.Step("upsert skills")
	for _, k := range BankSkills {
		if err := upsertSkill(c, k); err != nil {
			return fmt.Errorf("skill %s: %w", k.Name, err)
		}
		out.Item(k.Name)
	}

	out.Step("upsert prompts")
	for _, p := range BankPrompts {
		if err := upsertPrompt(c, p); err != nil {
			return fmt.Errorf("prompt %s: %w", p.Name, err)
		}
		out.Item(p.Name)
	}

	out.Step("upsert agents")
	for _, a := range BankAgents {
		if err := upsertAgent(c, a); err != nil {
			return fmt.Errorf("agent %s: %w", a.Name, err)
		}
		out.Item(a.Name)
	}

	out.Step("upsert use cases")
	ucIDs := map[string]uuid.UUID{}
	for _, uc := range BankUseCases(opts.LLMRoute) {
		id, err := upsertUseCase(c, uc)
		if err != nil {
			return fmt.Errorf("use case %s: %w", uc.Slug, err)
		}
		ucIDs[uc.Slug] = id
		out.Item(uc.Slug + " (" + uc.Status + ")")
	}

	out.Step("upsert API keys")
	keyTokens := map[string]string{}
	for _, k := range BankKeys {
		ucID, ok := ucIDs[k.UseCaseSlug]
		if !ok {
			return fmt.Errorf("key %s: bound use case %s not found", k.Name, k.UseCaseSlug)
		}
		token, err := upsertKey(c, k, ucID)
		if err != nil {
			return fmt.Errorf("key %s: %w", k.Name, err)
		}
		keyTokens[k.Name] = token
		out.Item(k.Name + " → " + k.UseCaseSlug)
	}

	// Task 8 will append: events, webhook.
	out.Step("seeded API keys (copy these into your demo notes)")
	for _, k := range BankKeys {
		out.Result(k.Name, keyTokens[k.Name])
	}

	out.Done("bank seed complete")
	return nil
}

// CleanupBank removes everything RunBank creates. Implemented in
// cleanup.go.
func CleanupBank(opts Options) error {
	return cleanupBankInternal(opts)
}

// --- Helpers --------------------------------------------------------------

func ensureRoute(c *Client, name string) error {
	type routeRow struct {
		Name string `json:"name"`
	}
	type routesResp struct {
		Routes []routeRow `json:"routes"`
	}
	var resp routesResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/routes", nil, &resp)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}
	if err := MustOK(http.MethodGet, "/api/admin/routes", status, body); err != nil {
		return err
	}
	for _, r := range resp.Routes {
		if r.Name == name {
			return nil
		}
	}
	return fmt.Errorf(`route %q not found.
  pllm seed bank requires an LLM route. Either:
    - create one in the UI at /ui/routes
    - or pass --llm-route <existing-route-name>`, name)
}

func upsertServer(c *Client, s ServerFixture) error {
	body := map[string]any{
		"name":        s.Name,
		"version":     s.Version,
		"description": s.Description,
	}
	type out struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	var got out
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/servers", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/servers", status, b); err != nil {
		return err
	}
	c.CacheUUID(s.Name, got.ID)
	return nil
}

func upsertSkill(c *Client, a ArtifactFixture) error {
	body := map[string]any{
		"name":        a.Name,
		"version":     a.Version,
		"description": a.Description,
	}
	var got struct {
		ID uuid.UUID `json:"id"`
	}
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/skills", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/skills", status, b); err != nil {
		return err
	}
	c.CacheUUID(a.Name, got.ID)
	return nil
}

func upsertPrompt(c *Client, a ArtifactFixture) error {
	body := map[string]any{
		"name":        a.Name,
		"version":     a.Version,
		"description": a.Description,
	}
	var got struct {
		ID uuid.UUID `json:"id"`
	}
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/prompts", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/prompts", status, b); err != nil {
		return err
	}
	c.CacheUUID(a.Name, got.ID)
	return nil
}

func upsertAgent(c *Client, a AgentFixture) error {
	refs := make([]map[string]any, 0, len(a.Refs))
	for _, r := range a.Refs {
		refs = append(refs, map[string]any{
			"target_kind": r.TargetKind,
			"target_name": r.TargetName,
		})
	}
	body := map[string]any{
		"name":        a.Name,
		"version":     a.Version,
		"description": a.Description,
		"refs":        refs,
	}
	var got struct {
		ID uuid.UUID `json:"id"`
	}
	status, b, err := c.Do(http.MethodPost, "/api/admin/registry/agents", body, &got)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodPost, "/api/admin/registry/agents", status, b); err != nil {
		return err
	}
	c.CacheUUID(a.Name, got.ID)
	return nil
}

// upsertKey ensures a demo key exists and is bound to the given use case.
// The admin keys API auto-generates the key value; the seed prints it
// at the end so the demo presenter can copy it. Re-runs cannot recover
// the plaintext (it's only returned at creation time), so existing
// keys are PUT-updated to refresh the use-case binding and report
// "(unchanged — token already issued)".
func upsertKey(c *Client, k KeyFixture, useCaseID uuid.UUID) (string, error) {
	// Look for an existing key by name first.
	// ListKeys response: { keys: [{...key fields, plaintext_key, usage}], pagination }
	type keyRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type listResp struct {
		Keys []keyRow `json:"keys"`
	}
	var existing listResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/keys?limit=200", nil, &existing)
	if err != nil {
		return "", err
	}
	if err := MustOK(http.MethodGet, "/api/admin/keys", status, body); err != nil {
		return "", err
	}
	for _, row := range existing.Keys {
		if row.Name == k.Name {
			// UpdateKey uses PUT /api/admin/keys/{keyID}. The use_case_id
			// field on the update body is **uuid.UUID, so we wrap once
			// to avoid the "field absent vs JSON null" ambiguity.
			ucPtr := useCaseID
			update := map[string]any{"use_case_id": &ucPtr}
			st, b, err := c.Do(http.MethodPut, "/api/admin/keys/"+row.ID.String(), update, nil)
			if err != nil {
				return "", err
			}
			if err := MustOK(http.MethodPut, "/api/admin/keys/"+row.ID.String(), st, b); err != nil {
				return "", err
			}
			return "(unchanged — token already issued)", nil
		}
	}
	// Create. CreateKeyRequest field is `key_type`. Response shape:
	// { ...embedded models.Key fields, plaintext_key: "...", usage: {...} }
	create := map[string]any{
		"name":        k.Name,
		"key_type":    "api",
		"use_case_id": useCaseID,
	}
	var got struct {
		PlaintextKey string `json:"plaintext_key"`
	}
	st, b, err := c.Do(http.MethodPost, "/api/admin/keys", create, &got)
	if err != nil {
		return "", err
	}
	if err := MustOK(http.MethodPost, "/api/admin/keys", st, b); err != nil {
		return "", err
	}
	return got.PlaintextKey, nil
}

// upsertUseCase POSTs the use case (the Create handler is upsert-by-slug;
// existing rows return 200 with the same row). Returns the row's UUID.
func upsertUseCase(c *Client, uc UseCaseFixture) (uuid.UUID, error) {
	refs := make([]map[string]any, 0, len(uc.Refs))
	for _, r := range uc.Refs {
		refs = append(refs, map[string]any{
			"target_kind": r.TargetKind,
			"target_name": r.TargetName,
		})
	}
	body := map[string]any{
		"name":                 uc.Name,
		"slug":                 uc.Slug,
		"description":          uc.Description,
		"risk_tier":            uc.RiskTier,
		"status":               uc.Status,
		"enforce_scope":        uc.EnforceScope,
		"data_classifications": uc.DataClassifications,
		"refs":                 refs,
	}
	type ucResp struct {
		UseCase struct {
			ID uuid.UUID `json:"id"`
		} `json:"use_case"`
	}
	var got ucResp
	status, b, err := c.Do(http.MethodPost, "/api/admin/use-cases", body, &got)
	if err != nil {
		return uuid.Nil, err
	}
	if err := MustOK(http.MethodPost, "/api/admin/use-cases", status, b); err != nil {
		return uuid.Nil, err
	}
	c.CacheUUID(uc.Slug, got.UseCase.ID)
	return got.UseCase.ID, nil
}
