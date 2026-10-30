package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDecisionConsumersConfig(t *testing.T) {
	var a, b DecisionConsumersConfig
	_ = a == b // must stay comparable

	var empty Config
	if err := yaml.Unmarshal([]byte("{}"), &empty); err != nil {
		t.Fatal(err)
	}
	if empty.DecisionConsumers != (DecisionConsumersConfig{}) {
		t.Fatalf("defaults not off: %+v", empty.DecisionConsumers)
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte("decision_consumers: {person_wake: {enabled: true}}"), &cfg); err != nil {
		t.Fatal(err)
	}
	want := DecisionConsumersConfig{PersonWake: DecisionConsumerSwitch{Enabled: true}}
	if cfg.DecisionConsumers != want {
		t.Fatalf("got %+v", cfg.DecisionConsumers)
	}

	c := DecisionConsumersConfig{WebhookQuestions: DecisionConsumerSwitch{Enabled: true}}
	if !c.Switch("webhook_question").Enabled || c.Switch("mail_triage").Enabled {
		t.Fatal("webhook_question mapping wrong")
	}
	if c.Switch("nope") != (DecisionConsumerSwitch{}) {
		t.Fatal("unknown id not off")
	}
}

func TestDecisionConsumersNormalize(t *testing.T) {
	on := DecisionConsumerSwitch{Enabled: true, AllowExternalImages: true}
	got := DecisionConsumersConfig{
		MemoryRerank: on, Appraisal: on, SalienceGate: on, MailTriage: on, WebhookQuestions: on,
		PersonWake: on, IndexerImages: on, GameMakerVisual: on,
	}.Normalize()
	for _, id := range []string{"memory_rerank", "appraisal", "salience_gate", "mail_triage", "webhook_question"} {
		if s := got.Switch(id); !s.Enabled || s.AllowExternalImages {
			t.Errorf("%s: %+v", id, s)
		}
	}
	for _, id := range []string{"person_wake", "indexer_images", "game_maker_visual"} {
		if s := got.Switch(id); !s.Enabled || !s.AllowExternalImages {
			t.Errorf("%s: %+v", id, s)
		}
	}
}

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
