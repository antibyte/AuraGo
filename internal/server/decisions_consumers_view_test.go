package server

import (
	"testing"

	"aurago/internal/config"
)

func TestDecisionConsumersView(t *testing.T) {
	on := config.DecisionConsumerSwitch{Enabled: true, AllowExternalImages: true}
	cfg := config.DecisionConsumersConfig{MailTriage: on, PersonWake: on, GameMakerVisual: config.DecisionConsumerSwitch{Enabled: true}}
	view := decisionConsumersView(cfg)
	if len(view) != 8 {
		t.Fatalf("want 8 keys, got %d: %v", len(view), view)
	}
	if _, ok := view["webhook_questions"]; !ok {
		t.Fatal("missing webhook_questions key")
	}
	for _, k := range []string{"person_wake", "indexer_images", "game_maker_visual"} {
		if _, ok := view[k]["allow_external_images"]; !ok {
			t.Errorf("%s lacks explicit allow_external_images", k)
		}
	}
	for _, k := range []string{"memory_rerank", "appraisal", "salience_gate", "mail_triage", "webhook_questions"} {
		if _, ok := view[k]["allow_external_images"]; ok {
			t.Errorf("%s must not carry allow_external_images", k)
		}
	}
	if !view["person_wake"]["allow_external_images"] || view["indexer_images"]["allow_external_images"] || view["game_maker_visual"]["allow_external_images"] {
		t.Errorf("image permissions wrong: %v", view)
	}
	if !view["mail_triage"]["enabled"] || !view["person_wake"]["enabled"] || !view["game_maker_visual"]["enabled"] || view["appraisal"]["enabled"] || view["indexer_images"]["enabled"] {
		t.Errorf("enabled flags wrong: %v", view)
	}
}
