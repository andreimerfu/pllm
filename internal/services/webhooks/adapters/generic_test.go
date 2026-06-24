package adapters

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
)

func TestGeneric_BuildRequest_HMACAndHeaders(t *testing.T) {
	hook := sampleHook(t, models.WebhookDestGeneric, nil, "https://example.com/hooks/pllm")
	g := &Generic{}
	req, err := g.BuildRequest(sampleEvent(), hook)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	if got := req.Header.Get("X-PLLM-Event"); got != "scope.denied" {
		t.Errorf("X-PLLM-Event = %q", got)
	}
	if got := req.Header.Get("X-PLLM-Delivery-ID"); got != "delivery-abc-123" {
		t.Errorf("X-PLLM-Delivery-ID = %q", got)
	}
	sigHeader := req.Header.Get("X-PLLM-Signature")
	if !strings.HasPrefix(sigHeader, "sha256=") {
		t.Fatalf("X-PLLM-Signature missing sha256 prefix: %q", sigHeader)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(hook.Secret))
	_, _ = mac.Write(body)
	wantSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sigHeader != wantSig {
		t.Errorf("signature mismatch:\n got: %s\nwant: %s", sigHeader, wantSig)
	}
}

func TestGeneric_Success(t *testing.T) {
	g := &Generic{}
	cases := []struct {
		status int
		want   bool
	}{
		{200, true},
		{201, true},
		{299, true},
		{300, false},
		{500, false},
	}
	for _, c := range cases {
		if got := g.Success(c.status, nil); got != c.want {
			t.Errorf("Success(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestGeneric_DefaultsAndConfig(t *testing.T) {
	g := &Generic{}
	if d := g.DefaultEvents(); d != nil {
		t.Errorf("expected nil default events for generic, got %v", d)
	}
	if k := g.ConfigKeys(); len(k) != 0 {
		t.Errorf("expected no config keys for generic, got %v", k)
	}
	if err := g.ValidateConfig(map[string]any{"random": "value"}); err != nil {
		t.Errorf("ValidateConfig should ignore unknown keys, got error: %v", err)
	}
}
