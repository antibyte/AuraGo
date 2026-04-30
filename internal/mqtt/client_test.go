package mqtt

import (
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/tools"
)

func TestMessageBufferLenUsesFullBufferAndPrunesByAge(t *testing.T) {
	b := newMessageBuffer()
	b.Configure(3, 1, 0)

	b.Add(tools.MQTTMessage{Topic: "home/a", Payload: "old", Timestamp: time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)})
	b.Add(tools.MQTTMessage{Topic: "home/a", Payload: "one", Timestamp: time.Now().UTC().Format(time.RFC3339)})
	b.Add(tools.MQTTMessage{Topic: "home/b", Payload: "two", Timestamp: time.Now().UTC().Format(time.RFC3339)})

	if got := b.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
	if got := len(b.Get("", 0)); got != 2 {
		t.Fatalf("Get default count = %d, want 2", got)
	}
}

func TestMessageBufferTruncatesOversizedPayloads(t *testing.T) {
	b := newMessageBuffer()
	b.Configure(10, 0, 5)

	msg := b.Add(tools.MQTTMessage{Topic: "home/a", Payload: "abcdef", Timestamp: time.Now().UTC().Format(time.RFC3339)})
	if !msg.PayloadTruncated {
		t.Fatal("expected payload to be marked truncated")
	}
	if msg.Payload != "abcde" {
		t.Fatalf("payload = %q, want abcde", msg.Payload)
	}
	if msg.PayloadBytes != 6 {
		t.Fatalf("payload bytes = %d, want 6", msg.PayloadBytes)
	}
}

func TestValidateMQTTTopics(t *testing.T) {
	if err := validatePublishTopic("home/+/temperature"); err == nil {
		t.Fatal("publish topic with wildcard was accepted")
	}
	if err := validateTopicFilter("home/#/bad"); err == nil {
		t.Fatal("topic filter with misplaced multi-level wildcard was accepted")
	}
	if err := validateTopicFilter("home/+/temperature"); err != nil {
		t.Fatalf("valid topic filter rejected: %v", err)
	}
}

func TestMissionTriggerRateLimit(t *testing.T) {
	missionTriggerMu.Lock()
	oldTriggers := missionTriggers
	missionTriggers = nil
	missionTriggerMu.Unlock()
	t.Cleanup(func() {
		missionTriggerMu.Lock()
		missionTriggers = oldTriggers
		missionTriggerMu.Unlock()
	})

	var fired int32
	RegisterMissionTrigger("home/#", "", 60, func(topic, payload string) {
		atomic.AddInt32(&fired, 1)
	})

	for _, trigger := range matchingMissionTriggers("home/sensor", "on") {
		trigger.callback("home/sensor", "on")
	}
	for _, trigger := range matchingMissionTriggers("home/sensor", "on") {
		trigger.callback("home/sensor", "on")
	}

	if got := atomic.LoadInt32(&fired); got != 1 {
		t.Fatalf("fired = %d, want 1", got)
	}
}
