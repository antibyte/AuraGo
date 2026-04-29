package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchHomeAssistantBlocksUnlistedService(t *testing.T) {
	cfg := &config.Config{}
	cfg.HomeAssistant.Enabled = true
	cfg.HomeAssistant.AllowedServices = []string{"light.turn_on"}

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "home_assistant",
		Operation: "call_service",
		Domain:    "switch",
		Service:   "turn_off",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})

	if !ok {
		t.Fatal("expected dispatchServices to handle home_assistant")
	}
	if !strings.Contains(out, "not allowed") {
		t.Fatalf("output = %s, want allowed_services denial", out)
	}
}

func TestDispatchHomeAssistantRequiresAllowedServicesForCallService(t *testing.T) {
	cfg := &config.Config{}
	cfg.HomeAssistant.Enabled = true

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "home_assistant",
		Operation: "call_service",
		Domain:    "light",
		Service:   "turn_on",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})

	if !ok {
		t.Fatal("expected dispatchServices to handle home_assistant")
	}
	if !strings.Contains(out, "allowed_services") {
		t.Fatalf("output = %s, want allowed_services denial", out)
	}
}

func TestDispatchHomeAssistantBlocksExplicitlyBlockedService(t *testing.T) {
	cfg := &config.Config{}
	cfg.HomeAssistant.Enabled = true
	cfg.HomeAssistant.BlockedServices = []string{"lock.unlock"}

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "home_assistant",
		Operation: "call_service",
		Domain:    "lock",
		Service:   "unlock",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})

	if !ok {
		t.Fatal("expected dispatchServices to handle home_assistant")
	}
	if !strings.Contains(out, "blocked") {
		t.Fatalf("output = %s, want blocked_services denial", out)
	}
}
