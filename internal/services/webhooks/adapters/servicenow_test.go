package adapters

import (
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
)

func TestServiceNow_BuildRequest_BasicAuthAndPayload(t *testing.T) {
	hook := sampleHook(t, models.WebhookDestServiceNow,
		map[string]any{"username": "svc.pllm", "password": "p@ss!"},
		"https://dev12345.service-now.com/api/now/table/incident",
	)
	s := &ServiceNow{}
	req, err := s.BuildRequest(sampleEvent(), hook)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		t.Fatalf("expected Basic auth, got %q", auth)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != "svc.pllm:p@ss!" {
		t.Errorf("auth payload = %q", string(decoded))
	}

	body, _ := io.ReadAll(req.Body)
	var p snPayload
	mustJSON(t, body, &p)

	if p.UPllmTarget != "anthropic/claude-3-opus" {
		t.Errorf("u_pllm_target = %q", p.UPllmTarget)
	}
	if p.UPllmUseCase != "fraud-investigation" {
		t.Errorf("u_pllm_use_case = %q", p.UPllmUseCase)
	}
	if p.UPllmEventType != "scope.denied" {
		t.Errorf("u_pllm_event_type = %q", p.UPllmEventType)
	}
	if p.UPllmDeliveryID != "delivery-abc-123" {
		t.Errorf("u_pllm_delivery_id = %q", p.UPllmDeliveryID)
	}
	if p.ShortDescription == "" {
		t.Error("short_description should be set")
	}
	if p.Priority == "" || p.Impact == "" || p.Urgency == "" {
		t.Errorf("priority triad missing: %+v", p)
	}
}

func TestServiceNow_Success(t *testing.T) {
	s := &ServiceNow{}
	cases := []struct {
		status int
		want   bool
	}{
		{201, true},
		{200, true},
		{401, false},
	}
	for _, c := range cases {
		if got := s.Success(c.status, nil); got != c.want {
			t.Errorf("Success(%d) = %v", c.status, got)
		}
	}
}

func TestServiceNow_ValidateConfig(t *testing.T) {
	s := &ServiceNow{}
	if err := s.ValidateConfig(map[string]any{}); err == nil {
		t.Error("expected error for missing username")
	}
	if err := s.ValidateConfig(map[string]any{"username": "u"}); err == nil {
		t.Error("expected error for missing password")
	}
	if err := s.ValidateConfig(map[string]any{"username": "u", "password": "p"}); err != nil {
		t.Errorf("expected ok, got %v", err)
	}
}
