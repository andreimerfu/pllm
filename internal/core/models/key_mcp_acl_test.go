package models

import (
	"testing"

	"github.com/lib/pq"
)

func TestIsMCPToolAllowed(t *testing.T) {
	cases := []struct {
		name    string
		allowed []string
		blocked []string
		call    string
		want    bool
	}{
		// Empty allow list = allow all
		{"default allow", nil, nil, "fs/read_file", true},
		{"default allow wire-separator", nil, nil, "fs__read_file", true},

		// Exact matches
		{"exact allow", []string{"fs/read_file"}, nil, "fs/read_file", true},
		{"exact deny other", []string{"fs/read_file"}, nil, "fs/write_file", false},
		{"mixed separator in pattern", []string{"fs__read_file"}, nil, "fs/read_file", true},

		// Wildcards
		{"slug wildcard", []string{"fs/*"}, nil, "fs/read_file", true},
		{"slug wildcard denies other slug", []string{"fs/*"}, nil, "github/list", false},
		{"tool wildcard any slug", []string{"*/read_file"}, nil, "fs/read_file", true},
		{"tool wildcard any slug — miss", []string{"*/read_file"}, nil, "fs/write_file", false},
		{"full wildcard", []string{"*"}, nil, "anything/goes", true},

		// Blocks take precedence
		{"block beats allow", []string{"*"}, []string{"fs/delete_*"}, "fs/delete_file", false},
		{"block specific, allow wildcard", []string{"fs/*"}, []string{"fs/delete_file"}, "fs/delete_file", false},
		{"block specific — other allowed", []string{"fs/*"}, []string{"fs/delete_file"}, "fs/read_file", true},

		// Substring wildcard inside segment
		{"middle wildcard", []string{"fs/*_file"}, nil, "fs/read_file", true},
		{"middle wildcard no match", []string{"fs/*_file"}, nil, "fs/list_dir", false},

		// Pattern without slash: matches tool portion OR full string
		{"bare tool pattern", []string{"read_file"}, nil, "fs/read_file", true},
		{"bare tool pattern — tool mismatch", []string{"read_file"}, nil, "fs/write_file", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &Key{
				AllowedMCPTools: pq.StringArray(tc.allowed),
				BlockedMCPTools: pq.StringArray(tc.blocked),
			}
			if got := k.IsMCPToolAllowed(tc.call); got != tc.want {
				t.Fatalf("IsMCPToolAllowed(%q) allow=%v block=%v = %v, want %v",
					tc.call, tc.allowed, tc.blocked, got, tc.want)
			}
		})
	}
}
