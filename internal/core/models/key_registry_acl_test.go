package models

import (
	"testing"

	"github.com/lib/pq"
)

func TestHasRegistryPermission(t *testing.T) {
	cases := []struct {
		name    string
		perms   []string
		kind    string
		rname   string
		action  RegistryAction
		want    bool
	}{
		// Star grants all
		{"star grants all", []string{"*"}, "server", "anything", RegistryActionPublish, true},
		{"star grants delete", []string{"*"}, "agent", "any", RegistryActionDelete, true},

		// Action-level globs
		{"publish on glob", []string{"publish:io.github.me/*"}, "server", "io.github.me/x", RegistryActionPublish, true},
		{"publish on glob — miss", []string{"publish:io.github.me/*"}, "server", "io.other/x", RegistryActionPublish, false},
		{"delete needs delete perm", []string{"publish:io.github.me/*"}, "server", "io.github.me/x", RegistryActionDelete, false},

		// Kind-scoped
		{"kind-scoped — match", []string{"publish:server/io.github.me/*"}, "server", "io.github.me/x", RegistryActionPublish, true},
		{"kind-scoped — wrong kind", []string{"publish:server/io.github.me/*"}, "agent", "io.github.me/x", RegistryActionPublish, false},

		// Wildcard kind
		{"any-kind", []string{"publish:*/io.github.me/*"}, "agent", "io.github.me/x", RegistryActionPublish, true},

		// Wildcard action
		{"any action", []string{"*:io.github.me/*"}, "server", "io.github.me/x", RegistryActionEdit, true},

		// No permissions = deny
		{"empty denies", nil, "server", "any", RegistryActionPublish, false},

		// Multiple entries
		{"union", []string{"publish:io.github.me/*", "delete:io.github.me/private"}, "server", "io.github.me/private", RegistryActionDelete, true},
		{"union — other", []string{"publish:io.github.me/*", "delete:io.github.me/private"}, "server", "io.github.me/other", RegistryActionDelete, false},

		// Bad patterns ignored silently
		{"bad pattern ignored", []string{"not-a-pattern"}, "server", "io.github.me/x", RegistryActionPublish, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &Key{RegistryPermissions: pq.StringArray(tc.perms)}
			if got := k.HasRegistryPermission(tc.kind, tc.rname, tc.action); got != tc.want {
				t.Fatalf("HasRegistryPermission(%q,%q,%q) perms=%v = %v, want %v",
					tc.kind, tc.rname, tc.action, tc.perms, got, tc.want)
			}
		})
	}
}
