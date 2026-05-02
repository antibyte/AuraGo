package mqtt

import (
	"reflect"
	"testing"

	"aurago/internal/config"
)

func TestFrigateRelayTopicsFollowEnabledFlags(t *testing.T) {
	cfg := &config.Config{}
	cfg.Frigate.Enabled = true
	cfg.Frigate.MQTTTopicPrefix = "frigate/"
	cfg.Frigate.EventRelay = true
	cfg.Frigate.ReviewRelay = true

	got := FrigateRelayTopics(cfg)
	want := []string{"frigate/events", "frigate/reviews"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FrigateRelayTopics = %#v, want %#v", got, want)
	}
}

func TestFrigateRelayKindHonorsFlags(t *testing.T) {
	cfg := &config.Config{}
	cfg.Frigate.Enabled = true
	cfg.Frigate.MQTTTopicPrefix = "nvr"
	cfg.Frigate.EventRelay = true
	cfg.Frigate.ReviewRelay = false

	if kind, ok := FrigateRelayKind(cfg, "nvr/events"); !ok || kind != "event" {
		t.Fatalf("event topic kind=%q ok=%v, want event true", kind, ok)
	}
	if kind, ok := FrigateRelayKind(cfg, "nvr/reviews"); ok || kind != "" {
		t.Fatalf("review topic kind=%q ok=%v, want empty false", kind, ok)
	}
	if kind, ok := FrigateRelayKind(cfg, "other/events"); ok || kind != "" {
		t.Fatalf("unrelated topic kind=%q ok=%v, want empty false", kind, ok)
	}
}
