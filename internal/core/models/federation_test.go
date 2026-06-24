package models

import "testing"

func TestFederationPeer_MatchesWhitelist(t *testing.T) {
	p := &FederationPeer{
		SyncMode:         FederationSyncModeWhitelist,
		WhitelistServers: []string{"foo/a", "foo/b"},
		WhitelistAgents:  []string{"agents/x"},
	}
	cases := []struct {
		kind RegistryKind
		name string
		want bool
	}{
		{RegistryKindServer, "foo/a", true},
		{RegistryKindServer, "foo/b", true},
		{RegistryKindServer, "foo/c", false},
		{RegistryKindAgent, "agents/x", true},
		{RegistryKindAgent, "agents/y", false},
		// Unsupported kind in whitelist mode → false (caller skips).
		{RegistryKindLLM, "anything", false},
	}
	for _, c := range cases {
		got := p.MatchesWhitelist(c.kind, c.name)
		if got != c.want {
			t.Errorf("MatchesWhitelist(%s, %s) = %v, want %v", c.kind, c.name, got, c.want)
		}
	}

	// "all" mode accepts everything regardless of lists.
	p.SyncMode = FederationSyncModeAll
	if !p.MatchesWhitelist(RegistryKindServer, "anything/at/all") {
		t.Fatalf("all-mode should accept any name")
	}
}

func TestFederationPeer_MatchesTags(t *testing.T) {
	p := &FederationPeer{SyncMode: FederationSyncModeTagFilter, TagFilters: []string{"PII", "finance"}}
	if !p.MatchesTags(`{"tags":["customer","finance"]}`) {
		t.Errorf("tag filter should match case-insensitive substring")
	}
	if !p.MatchesTags(`{"category":"pii-sensitive"}`) {
		t.Errorf("substring match should be case-insensitive")
	}
	if p.MatchesTags(`{"category":"other"}`) {
		t.Errorf("expected miss when no tag matches")
	}
	// Empty filters in tag_filter mode → match (vacuously true).
	p.TagFilters = nil
	if p.MatchesTags(`{}`) {
		// With zero filters, no tag matches and we should be conservative.
		// The implementation returns false in this case; assert that.
		t.Errorf("zero-filter tag_filter should be a miss")
	}
}

func TestRegistry_IsFederated(t *testing.T) {
	s := RegistryServer{}
	if s.IsFederated() {
		t.Errorf("empty SourcePeerID should be local")
	}
	s.SourcePeerID = "lob-a"
	if !s.IsFederated() {
		t.Errorf("non-empty SourcePeerID should be federated")
	}
}
