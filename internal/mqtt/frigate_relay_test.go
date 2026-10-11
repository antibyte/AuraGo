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

func TestFrigateRelayTopicsRejectsNonLiteralPrefix(t *testing.T) {
	for _, prefix := range []string{"+", "home/#", "bad\x00prefix", "home/+"} {
		cfg := &config.Config{}
		cfg.Frigate.Enabled = true
		cfg.Frigate.EventRelay = true
		cfg.Frigate.ReviewRelay = true
		cfg.Frigate.MQTTTopicPrefix = prefix
		if got := FrigateRelayTopics(cfg); got != nil {
			t.Fatalf("prefix %q topics = %#v, want nil", prefix, got)
		}
		if _, ok := FrigateRelayKind(cfg, prefix+"/events"); ok {
			t.Fatalf("prefix %q still classified a topic", prefix)
		}
	}

	cfg := &config.Config{}
	cfg.Frigate.Enabled = true
	cfg.Frigate.EventRelay = true
	cfg.Frigate.ReviewRelay = true
	cfg.Frigate.MQTTTopicPrefix = "nvr"
	got := FrigateRelayTopics(cfg)
	want := []string{"nvr/events", "nvr/reviews"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FrigateRelayTopics = %#v, want %#v", got, want)
	}

	cfg.Frigate.MQTTTopicPrefix = ""
	got = FrigateRelayTopics(cfg)
	want = []string{"frigate/events", "frigate/reviews"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty prefix topics = %#v, want %#v", got, want)
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
