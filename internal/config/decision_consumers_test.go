package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecisionConsumersSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8088\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DecisionConsumers.PersonWake = DecisionConsumerSwitch{Enabled: true, AllowExternalImages: true}
	cfg.DecisionConsumers.MailTriage.Enabled = true
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "decision_consumers:") {
		t.Fatalf("section not added:\n%s", raw)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.DecisionConsumers != cfg.DecisionConsumers || !reloaded.DecisionConsumers.PersonWake.AllowExternalImages || !reloaded.DecisionConsumers.MailTriage.Enabled {
		t.Fatalf("round trip lost data: %+v", reloaded.DecisionConsumers)
	}
}
