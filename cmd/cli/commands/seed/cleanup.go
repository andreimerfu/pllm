package seed

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// cleanupBankInternal removes everything RunBank creates. Idempotent —
// missing entities are silently skipped.
func cleanupBankInternal(opts Options) error {
	c := NewClient(opts.BaseURL, opts.MasterKey)
	out := newPrinter(opts.JSONOutput)

	out.Step("verify gateway + master key")
	if err := c.HealthCheck(); err != nil {
		return err
	}

	// ---- Webhook --------------------------------------------------------
	out.Step("remove webhook")
	type webhookRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type wResp struct {
		Webhooks []webhookRow `json:"webhooks"`
	}
	var ws wResp
	status, body, err := c.Do(http.MethodGet, "/api/admin/webhooks", nil, &ws)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/webhooks", status, body); err != nil {
		return err
	}
	for _, w := range ws.Webhooks {
		if w.Name == "demo-siem-stub" {
			st, b, err := c.Do(http.MethodDelete, "/api/admin/webhooks/"+w.ID.String(), nil, nil)
			if err != nil {
				return err
			}
			if err := MustOK(http.MethodDelete, "/api/admin/webhooks/"+w.ID.String(), st, b); err != nil {
				return err
			}
			out.Item("demo-siem-stub")
		}
	}

	// ---- Keys -----------------------------------------------------------
	out.Step("remove demo-* keys")
	type keyRow struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}
	type kResp struct {
		Keys []keyRow `json:"keys"`
	}
	var ks kResp
	status, body, err = c.Do(http.MethodGet, "/api/admin/keys?limit=500", nil, &ks)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/keys", status, body); err != nil {
		return err
	}
	for _, k := range ks.Keys {
		if !strings.HasPrefix(k.Name, "demo-") {
			continue
		}
		st, b, err := c.Do(http.MethodDelete, "/api/admin/keys/"+k.ID.String(), nil, nil)
		if err != nil {
			return err
		}
		if err := MustOK(http.MethodDelete, "/api/admin/keys/"+k.ID.String(), st, b); err != nil {
			return err
		}
		out.Item(k.Name)
	}

	// ---- Use cases ------------------------------------------------------
	out.Step("remove use cases")
	demoSlugs := map[string]bool{}
	for _, uc := range BankUseCases(opts.LLMRoute) {
		demoSlugs[uc.Slug] = true
	}
	type ucRow struct {
		ID   uuid.UUID `json:"id"`
		Slug string    `json:"slug"`
	}
	type ucResp struct {
		UseCases []ucRow `json:"use_cases"`
	}
	var ucs ucResp
	status, body, err = c.Do(http.MethodGet, "/api/admin/use-cases", nil, &ucs)
	if err != nil {
		return err
	}
	if err := MustOK(http.MethodGet, "/api/admin/use-cases", status, body); err != nil {
		return err
	}
	for _, uc := range ucs.UseCases {
		if !demoSlugs[uc.Slug] {
			continue
		}
		st, b, err := c.Do(http.MethodDelete, "/api/admin/use-cases/"+uc.ID.String(), nil, nil)
		if err != nil {
			return err
		}
		if err := MustOK(http.MethodDelete, "/api/admin/use-cases/"+uc.ID.String(), st, b); err != nil {
			return err
		}
		out.Item(uc.Slug)
	}

	// ---- Registry artifacts --------------------------------------------
	// Servers, agents, prompts, skills under bank.demo/* are always ours.
	// Public names (tavily-search, mcp.io/*) are deleted only if their
	// description starts with our SeedMarker so we don't blow away an
	// operator's pre-existing entry.

	allServers := append([]ServerFixture{}, BankServers...)
	allServers = append(allServers, PublicLiveServers...)
	out.Step("remove registry servers")
	for _, s := range allServers {
		if err := deleteRegistryArtifact(c, "servers", s.Name, s.Version); err != nil {
			return fmt.Errorf("delete server %s: %w", s.Name, err)
		}
		out.Item(s.Name)
	}
	out.Step("remove registry agents")
	for _, a := range BankAgents {
		if err := deleteRegistryArtifact(c, "agents", a.Name, a.Version); err != nil {
			return fmt.Errorf("delete agent %s: %w", a.Name, err)
		}
		out.Item(a.Name)
	}
	out.Step("remove registry prompts")
	for _, p := range BankPrompts {
		if err := deleteRegistryArtifact(c, "prompts", p.Name, p.Version); err != nil {
			return fmt.Errorf("delete prompt %s: %w", p.Name, err)
		}
		out.Item(p.Name)
	}
	out.Step("remove registry skills")
	for _, s := range BankSkills {
		if err := deleteRegistryArtifact(c, "skills", s.Name, s.Version); err != nil {
			return fmt.Errorf("delete skill %s: %w", s.Name, err)
		}
		out.Item(s.Name)
	}

	out.Done("bank seed cleanup complete")
	return nil
}

// deleteRegistryArtifact deletes the seeded version of name. Public-name
// safety check (description SeedMarker) is enforced by GETting first.
// The registry delete route is concrete on (name, version) — there is
// no "latest" alias — so we always pass the version we seeded.
func deleteRegistryArtifact(c *Client, kind, name, version string) error {
	if !strings.HasPrefix(name, "bank.demo/") {
		path := "/v1/registry/" + kind + "/" + EscapeName(name)
		var got struct {
			Description string `json:"description"`
		}
		status, body, err := c.Do(http.MethodGet, path, nil, &got)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return nil
		}
		if err := MustOK(http.MethodGet, path, status, body); err != nil {
			return err
		}
		if !strings.HasPrefix(got.Description, SeedMarker) {
			return nil // operator-owned; don't touch
		}
	}
	path := "/api/admin/registry/" + kind + "/" + EscapeName(name) + "/versions/" + EscapeName(version)
	st, b, err := c.Do(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if st == http.StatusNotFound {
		return nil
	}
	return MustOK(http.MethodDelete, path, st, b)
}
