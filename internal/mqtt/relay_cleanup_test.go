package mqtt

import (
	"sync/atomic"
	"testing"
)

func isolateMissionTriggers(t *testing.T) {
	t.Helper()
	missionTriggerMu.Lock()
	previous := missionTriggers
	missionTriggers = nil
	missionTriggerMu.Unlock()
	t.Cleanup(func() {
		missionTriggerMu.Lock()
		missionTriggers = previous
		missionTriggerMu.Unlock()
	})
}

func TestQueuedMissionTriggerFromReplacedRegistrationIsDropped(t *testing.T) {
	isolateMissionTriggers(t)
	var oldFired, newFired atomic.Int32
	RegisterMissionTriggerForKey("queued-replacement", "home/#", "", 60, func(string, string) {
		oldFired.Add(1)
	})
	queued := matchingMissionTriggers("home/sensor", "on")
	if len(queued) != 1 {
		t.Fatalf("queued trigger count = %d, want 1", len(queued))
	}

	RegisterMissionTriggerForKey("queued-replacement", "home/#", "", 60, func(string, string) {
		newFired.Add(1)
	})
	queued[0].callback("home/sensor", "on")
	if got := oldFired.Load(); got != 0 {
		t.Fatalf("replaced callback fired %d times", got)
	}
	if got := newFired.Load(); got != 0 {
		t.Fatalf("replaced callback consumed the new trigger interval: fired=%d", got)
	}

	fresh := matchingMissionTriggers("home/sensor", "on")
	if len(fresh) != 1 {
		t.Fatalf("fresh trigger count = %d, want 1", len(fresh))
	}
	fresh[0].callback("home/sensor", "on")
	if got := newFired.Load(); got != 1 {
		t.Fatalf("new callback fired %d times after stale callback, want 1", got)
	}
}

func TestQueuedMissionTriggerFromUnregisteredRegistrationIsDropped(t *testing.T) {
	isolateMissionTriggers(t)
	var fired atomic.Int32
	RegisterMissionTriggerForKey("queued-unregister", "home/#", "", 0, func(string, string) {
		fired.Add(1)
	})
	queued := matchingMissionTriggers("home/sensor", "on")
	if len(queued) != 1 {
		t.Fatalf("queued trigger count = %d, want 1", len(queued))
	}

	UnregisterMissionTrigger("queued-unregister")
	queued[0].callback("home/sensor", "on")
	if got := fired.Load(); got != 0 {
		t.Fatalf("unregistered callback fired %d times", got)
	}
}

func TestLegacyMessageHandlerDropsWithoutGeneration(t *testing.T) {
	defaultControllerMu.Lock()
	previous := defaultController
	defaultController = nil
	defaultControllerMu.Unlock()
	t.Cleanup(func() {
		defaultControllerMu.Lock()
		defaultController = previous
		defaultControllerMu.Unlock()
	})

	messageHandler(nil, nil)
	if controller := currentDefaultController(); controller != nil {
		t.Fatal("legacy message handler created a controller without an active generation")
	}
}

func TestMQTTSystemTopicsRequireExplicitDollarFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		topic  string
		want   bool
	}{
		{name: "hash wildcard", filter: "#", topic: "$SYS/broker/uptime", want: false},
		{name: "plus wildcard", filter: "+/broker", topic: "$SYS/broker", want: false},
		{name: "explicit system prefix", filter: "$SYS/#", topic: "$SYS/broker/uptime", want: true},
		{name: "ordinary filter", filter: "home/#", topic: "$SYS/home", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := topicMatches(tt.filter, tt.topic); got != tt.want {
				t.Fatalf("topicMatches(%q, %q) = %v, want %v", tt.filter, tt.topic, got, tt.want)
			}
		})
	}
}
