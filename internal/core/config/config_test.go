package config

import "testing"

func TestDefaultLifecycleConfig(t *testing.T) {
	c := DefaultLifecycleConfig()
	if !c.Enabled {
		t.Fatal("expected Enabled=true by default")
	}
	if c.Interval.String() != "1h0m0s" {
		t.Fatalf("expected default Interval=1h, got %s", c.Interval)
	}
	if c.InactiveKeyDays != 90 {
		t.Fatalf("expected default InactiveKeyDays=90, got %d", c.InactiveKeyDays)
	}
	if c.RiskyServerScoreFloor != 40 {
		t.Fatalf("expected default RiskyServerScoreFloor=40, got %f", c.RiskyServerScoreFloor)
	}
	if !c.DryRun {
		t.Fatal("expected DryRun=true by default — operators must opt in to enforcement")
	}
	if !c.OwnerlessKeysEnabled {
		t.Fatal("expected OwnerlessKeysEnabled=true by default")
	}
}
