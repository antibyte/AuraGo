package mqtt

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
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

func TestGatedMissionTriggerRefusedAdmitDoesNotConsumeInterval(t *testing.T) {
	isolateMissionTriggers(t)
	var admit atomic.Bool
	var fired atomic.Int32
	RegisterGatedMissionTriggerForKey("gated-interval", "home/#", "", 60, func(string, string) bool {
		return admit.Load()
	}, func(string, string) {
		fired.Add(1)
	})
	registered := missionTriggerByKey(t, "gated-interval")

	for delivery := 1; delivery <= 2; delivery++ {
		matched := matchingMissionTriggers("home/sensor", "on")
		if len(matched) != 1 {
			t.Fatalf("closed-gate delivery %d matched %d triggers, want 1", delivery, len(matched))
		}
		if matched[0].id != registered.id {
			t.Fatalf("closed-gate delivery %d id = %d, want %d", delivery, matched[0].id, registered.id)
		}
		matched[0].callback("home/sensor", "on")
	}
	if got := fired.Load(); got != 0 {
		t.Fatalf("closed gate fired %d times", got)
	}
	if got := missionTriggerByKey(t, "gated-interval").lastFired; !got.IsZero() {
		t.Fatalf("closed gate set lastFired to %s", got.Format(time.RFC3339Nano))
	}

	admit.Store(true)
	matched := matchingMissionTriggers("home/sensor", "on")
	if len(matched) != 1 || matched[0].id != registered.id {
		t.Fatalf("admitted match = %d triggers, id ok=%v", len(matched), len(matched) == 1 && matched[0].id == registered.id)
	}
	matched[0].callback("home/sensor", "on")
	matched[0].callback("home/sensor", "on")
	if got := fired.Load(); got != 1 {
		t.Fatalf("trigger id %d fired %d times within the interval, want 1", registered.id, got)
	}
	if missionTriggerByKey(t, "gated-interval").lastFired.IsZero() {
		t.Fatal("admitted delivery left lastFired unset")
	}
	if again := matchingMissionTriggers("home/sensor", "on"); len(again) != 0 {
		t.Fatalf("trigger id %d matched %d times inside the interval", registered.id, len(again))
	}
}

func TestDroppedMissionQueueDoesNotConsumeMissionTriggerInterval(t *testing.T) {
	isolateMissionTriggers(t)
	var fired atomic.Int32
	RegisterGatedMissionTriggerForKey("queue-drop", "home/#", "", 60, func(string, string) bool {
		return true
	}, func(string, string) {
		fired.Add(1)
	})
	matched := matchingMissionTriggers("home/sensor", "on")
	if len(matched) != 1 {
		t.Fatalf("queued trigger count = %d, want 1", len(matched))
	}

	controller := NewMQTTController(mqttTestLogger())
	ctx, cancel := context.WithCancel(context.Background())
	gen := &mqttGeneration{
		ctx:          ctx,
		cancel:       cancel,
		missionQueue: make(chan missionJob, 1),
	}
	controller.mu.Lock()
	controller.current = gen
	controller.mu.Unlock()
	t.Cleanup(func() {
		gen.closed.Store(true)
		cancel()
		controller.mu.Lock()
		if controller.current == gen {
			controller.current = nil
		}
		controller.mu.Unlock()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer stopCancel()
		if err := controller.Stop(stopCtx); err != nil {
			t.Errorf("stop mission queue controller: %v", err)
		}
	})

	controller.enqueueMission(gen, missionTriggerEntry{}, "home/sensor", "filler")
	before := atomic.LoadUint64(&stats.droppedMissionJobs)
	controller.enqueueMission(gen, matched[0], "home/sensor", "on")
	if got := atomic.LoadUint64(&stats.droppedMissionJobs); got != before+1 {
		t.Fatalf("dropped mission jobs = %d, want %d", got, before+1)
	}
	if got := fired.Load(); got != 0 {
		t.Fatalf("dropped queue job fired %d times", got)
	}
	if got := missionTriggerByKey(t, "queue-drop").lastFired; !got.IsZero() {
		t.Fatalf("dropped queue job set lastFired to %s", got.Format(time.RFC3339Nano))
	}
}

func missionTriggerByKey(t *testing.T, key string) missionTriggerEntry {
	t.Helper()
	missionTriggerMu.RLock()
	defer missionTriggerMu.RUnlock()
	for _, trigger := range missionTriggers {
		if trigger.key == key {
			return trigger
		}
	}
	t.Fatalf("missing mission trigger %q", key)
	return missionTriggerEntry{}
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
