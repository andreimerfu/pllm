package cel

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/amerfu/pllm/internal/core/models"
)

// TestCompile_ValidExpression confirms a well-typed predicate parses
// against the keys schema and returns a CompiledRule.
func TestCompile_ValidExpression(t *testing.T) {
	r, err := Compile(models.LifecycleScanKeys,
		`key.is_active && (now - key.created_at) > duration("24h")`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if r.Target != models.LifecycleScanKeys {
		t.Fatalf("target: got %s want keys", r.Target)
	}
}

// TestCompile_NotBool rejects expressions that don't return bool — a
// common authoring mistake (e.g. forgetting the comparison).
func TestCompile_NotBool(t *testing.T) {
	_, err := Compile(models.LifecycleScanKeys, `key.name`)
	if err == nil {
		t.Fatal("expected error for non-bool expression")
	}
}

// TestCompile_UnknownField surfaces type-check errors at compile time
// rather than letting them silently fail at eval.
func TestCompile_UnknownField(t *testing.T) {
	// Reading an unknown field on a map<string,dyn> is not a compile-time
	// error in cel-go (maps are opaque). It evaluates to an error at
	// runtime — verify that path here.
	r, err := Compile(models.LifecycleScanKeys, `key.totally_made_up == 1`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	in := KeyToInput(&models.Key{BaseModel: models.BaseModel{ID: uuid.New()}})
	_, err = r.Eval(context.Background(), in, time.Now())
	if err == nil {
		t.Fatal("expected eval error for missing field")
	}
}

// TestEval_OldKeyMatches walks the typical "older than N" predicate
// against a key with LastUsedAt 100 days ago — should match a 90-day
// rule and not match a 200-day rule.
func TestEval_OldKeyMatches(t *testing.T) {
	hundredDaysAgo := time.Now().Add(-100 * 24 * time.Hour)
	k := &models.Key{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		IsActive:   true,
		Type:       models.KeyTypeAPI,
		LastUsedAt: &hundredDaysAgo,
	}
	cases := []struct {
		name   string
		expr   string
		expect bool
	}{
		{
			name:   "90d threshold matches",
			expr:   `key.last_used_at != null && (now - key.last_used_at) > duration("2160h")`,
			expect: true,
		},
		{
			name:   "200d threshold does not match",
			expr:   `key.last_used_at != null && (now - key.last_used_at) > duration("4800h")`,
			expect: false,
		},
		{
			name:   "tag membership false (no tags)",
			expr:   `"env:prod" in key.tags`,
			expect: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Compile(models.LifecycleScanKeys, tc.expr)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got, err := r.Eval(context.Background(), KeyToInput(k), time.Now())
			if err != nil {
				t.Fatalf("eval: %v", err)
			}
			if got != tc.expect {
				t.Fatalf("expr %q: got %v want %v", tc.expr, got, tc.expect)
			}
		})
	}
}

// TestEval_NullLastUsed verifies that the null-guard pattern keeps the
// expression safe for never-used keys.
func TestEval_NullLastUsed(t *testing.T) {
	k := &models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		IsActive:  true,
		Type:      models.KeyTypeAPI,
		// LastUsedAt nil
	}
	r, err := Compile(models.LifecycleScanKeys,
		`key.last_used_at != null && (now - key.last_used_at) > duration("24h")`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got, err := r.Eval(context.Background(), KeyToInput(k), time.Now())
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got {
		t.Fatal("expected false for never-used key")
	}
}

// TestEval_TagMatches confirms list membership works for tag-based
// rules, which are central to the rotate-by-tag template.
func TestEval_TagMatches(t *testing.T) {
	k := &models.Key{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Tags:      []string{"env:prod", "team:fraud"},
	}
	r, err := Compile(models.LifecycleScanKeys, `"env:prod" in key.tags`)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got, err := r.Eval(context.Background(), KeyToInput(k), time.Now())
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if !got {
		t.Fatal("expected true for tagged key")
	}
}
