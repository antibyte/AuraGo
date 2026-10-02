package bluetooth

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func waitForInteraction(t *testing.T, broker *interactionBroker) string {
	t.Helper()
	var id string
	waitFor(t, "interaction opens", func() bool { id = broker.current(); return id != "" })
	return id
}

func TestBrokerAskReturnsTheOperatorAnswer(t *testing.T) {
	broker := newInteractionBroker()
	result := make(chan interactionAnswer, 1)
	go func() {
		answer, err := broker.ask(context.Background(), Interaction{Kind: InteractionConfirmPasskey, DeviceAddress: testDeviceAddress, Passkey: "482913"})
		if err != nil {
			t.Errorf("ask: %v", err)
		}
		result <- answer
	}()
	id := waitForInteraction(t, broker)
	view, err := broker.get(id)
	if err != nil || view.Passkey != "482913" || view.RemainingSeconds <= 0 {
		t.Fatalf("get = %+v, %v", view, err)
	}
	if err := broker.answer(id, true, ""); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if answer := <-result; !answer.accept {
		t.Fatal("accepted answer arrived as rejection")
	}
	if _, err := broker.get(id); ErrorCode(err) != ErrorInteractionExpired {
		t.Fatalf("get after close = %v, want expired", err)
	}
	if err := broker.answer("ffffffffffffffffffffffffffffffff", true, ""); ErrorCode(err) != ErrorInteractionNotFound {
		t.Fatalf("unknown answer = %v, want not found", err)
	}
}

func TestBrokerExpiresUnansweredInteractions(t *testing.T) {
	broker := newInteractionBroker()
	broker.timeout = 20 * time.Millisecond
	answer, err := broker.ask(context.Background(), Interaction{Kind: InteractionAuthorizePairing})
	if ErrorCode(err) != ErrorInteractionExpired || answer.accept {
		t.Fatalf("ask = %+v, %v; want expired rejection", answer, err)
	}
	if broker.current() != "" {
		t.Fatal("expired interaction stayed open")
	}
}

func TestBrokerValidatesAnswers(t *testing.T) {
	for _, tc := range []struct {
		kind  InteractionKind
		value string
		ok    bool
	}{
		{InteractionEnterPasskey, "abc", false},
		{InteractionEnterPasskey, "1234567", false},
		{InteractionEnterPasskey, "000123", true},
		{InteractionEnterPIN, "", false},
		{InteractionEnterPIN, strings.Repeat("1", 17), false},
		{InteractionEnterPIN, "0000", true},
	} {
		broker := newInteractionBroker()
		done := make(chan interactionAnswer, 1)
		go func() {
			answer, _ := broker.ask(context.Background(), Interaction{Kind: tc.kind})
			done <- answer
		}()
		id := waitForInteraction(t, broker)
		err := broker.answer(id, true, tc.value)
		if tc.ok != (err == nil) {
			t.Fatalf("%s %q: err = %v, want ok=%v", tc.kind, tc.value, err, tc.ok)
		}
		if !tc.ok {
			if broker.current() != id {
				t.Fatalf("%s %q: invalid answer closed the interaction", tc.kind, tc.value)
			}
			broker.cancel()
		}
		answer := <-done
		if tc.ok && answer.value != tc.value {
			t.Fatalf("%s: value = %q, want %q", tc.kind, answer.value, tc.value)
		}
	}
}

func TestBrokerReplacesTheOpenInteraction(t *testing.T) {
	broker := newInteractionBroker()
	first := make(chan interactionAnswer, 1)
	go func() {
		answer, _ := broker.ask(context.Background(), Interaction{Kind: InteractionAuthorizePairing})
		first <- answer
	}()
	firstID := waitForInteraction(t, broker)
	go func() { _, _ = broker.ask(context.Background(), Interaction{Kind: InteractionAuthorizePairing}) }()
	waitFor(t, "second interaction", func() bool { id := broker.current(); return id != "" && id != firstID })
	if answer := <-first; answer.accept {
		t.Fatal("replaced interaction must be rejected")
	}
	broker.cancel()
}

func TestBrokerDisplayUpdatesInPlace(t *testing.T) {
	broker := newInteractionBroker()
	var mu sync.Mutex
	var announcements []bool
	broker.onChange = func(id string, isNew bool) {
		if id != "" {
			mu.Lock()
			announcements = append(announcements, isNew)
			mu.Unlock()
		}
	}
	broker.show(Interaction{Kind: InteractionDisplayPasskey, DeviceAddress: testDeviceAddress, Passkey: "000042"})
	id := broker.current()
	broker.show(Interaction{Kind: InteractionDisplayPasskey, DeviceAddress: testDeviceAddress, Passkey: "000042", Entered: 3})
	if broker.current() != id {
		t.Fatal("display update opened a new interaction")
	}
	view, _ := broker.get(id)
	if view.Entered != 3 {
		t.Fatalf("entered = %d, want 3", view.Entered)
	}
	if err := broker.answer(id, true, ""); ErrorCode(err) != ErrorInvalidArgument {
		t.Fatalf("answering a display interaction = %v, want invalid argument", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(announcements) != 2 || !announcements[0] || announcements[1] {
		t.Fatalf("announcements = %v, want [true false]", announcements)
	}
	broker.cancel()
}

func TestBrokerDropsSecretsWhenClosed(t *testing.T) {
	broker := newInteractionBroker()
	go func() {
		_, _ = broker.ask(context.Background(), Interaction{Kind: InteractionConfirmPasskey, Passkey: "123456"})
	}()
	id := waitForInteraction(t, broker)
	broker.mu.Lock()
	pending := broker.open
	broker.mu.Unlock()
	if err := broker.answer(id, false, ""); err != nil {
		t.Fatal(err)
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if pending.view.Passkey != "" || pending.view.PIN != "" {
		t.Fatalf("secrets kept after close: %+v", pending.view)
	}
}

func TestServiceNameKnowsCommonProfiles(t *testing.T) {
	if got := serviceName("0000110b-0000-1000-8000-00805f9b34fb"); got != "Audio Sink" {
		t.Fatalf("serviceName(A2DP sink) = %q", got)
	}
	if got := serviceName("12345678-0000-1000-8000-00805f9b34fb"); got != "12345678-0000-1000-8000-00805f9b34fb" {
		t.Fatalf("unknown UUID = %q", got)
	}
}
