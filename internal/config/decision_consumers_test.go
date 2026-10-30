package config

import (
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
