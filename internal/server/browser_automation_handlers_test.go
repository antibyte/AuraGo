package server

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestBrowserAutomationHealthHintForHostRuntimeMismatch(t *testing.T) {
	cfg := &config.Config{}
	cfg.BrowserAutomation.URL = "http://browser-automation:7331"
	cfg.Runtime.IsDocker = false

	suggestedURL, hint := browserAutomationHealthHint(cfg)
	if suggestedURL != "http://127.0.0.1:7331" {
		t.Fatalf("suggestedURL = %q, want http://127.0.0.1:7331", suggestedURL)
	}
	if !strings.Contains(hint, "not running in Docker") {
		t.Fatalf("hint = %q, want Docker runtime explanation", hint)
	}
}

func TestBrowserAutomationHealthHintForDockerLoopbackMismatch(t *testing.T) {
	cfg := &config.Config{}
	cfg.BrowserAutomation.URL = "http://127.0.0.1:7331"
	cfg.Runtime.IsDocker = true

	suggestedURL, hint := browserAutomationHealthHint(cfg)
	if suggestedURL != "http://browser-automation:7331" {
		t.Fatalf("suggestedURL = %q, want http://browser-automation:7331", suggestedURL)
	}
	if !strings.Contains(hint, "localhost points to the AuraGo container itself") {
		t.Fatalf("hint = %q, want Docker localhost explanation", hint)
	}
}

func TestBrowserAutomationAnnotateHealthAddsHintAndSuggestion(t *testing.T) {
	cfg := &config.Config{}
	cfg.BrowserAutomation.URL = "http://browser-automation:7331"
	cfg.Runtime.IsDocker = false

	health := browserAutomationAnnotateHealth(cfg, map[string]interface{}{
		"status":  "error",
		"message": `Get "http://browser-automation:7331/health": dial tcp: lookup browser-automation on 127.0.0.53:53: server misbehaving`,
	})

	msg, _ := health["message"].(string)
	if !strings.Contains(msg, "Hint:") {
		t.Fatalf("message = %q, want appended hint", msg)
	}
	if got, _ := health["suggested_url"].(string); got != "http://127.0.0.1:7331" {
		t.Fatalf("suggested_url = %q, want http://127.0.0.1:7331", got)
	}
}
