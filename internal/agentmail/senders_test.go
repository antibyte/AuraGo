package agentmail

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSenderMatches(t *testing.T) {
	entries := normalizeAllowedSenders([]string{" Boss@Example.COM ", "@partner.example", "", "boss@example.com", "partner.example", "@"})
	if len(entries) != 4 {
		t.Fatalf("normalized entries = %#v, want the 3 distinct non-empty ones plus the malformed one kept as written", entries)
	}
	for _, tc := range []struct {
		from string
		want bool
	}{
		{"boss@example.com", true},
		{"BOSS@EXAMPLE.COM", true},
		{"Boss <boss@example.com>", true},
		{"alice@partner.example", true},
		{"alice@sub.partner.example", false}, // "@domain" is that domain, not its subdomains
		{"alice@evilpartner.example", false}, // a longer name that merely ends in it
		{"alice@partner.example.evil.test", false},
		{"other@example.com", false},                     // a full address is exact
		{`"boss@example.com" <evil@example.net>`, false}, // the display name is not the sender
		{"boss@example.com, evil@example.net", false},    // not a single address
		{"partner.example", false},                       // an entry without "@" never matches
		{"", false},
		{"not an address", false},
	} {
		if got := senderMatches(entries, tc.from); got != tc.want {
			t.Errorf("senderMatches(%q) = %v, want %v", tc.from, got, tc.want)
		}
	}
	if normalizeAllowedSenders([]string{"  ", ""}) != nil {
		t.Error("an all-blank list must normalize to no list")
	}
}

type relayHarness struct {
	svc    *Service
	prompt chan string
	calls  atomic.Int32
}

func newRelayHarness(t *testing.T, allowed []string, client *Client) *relayHarness {
	t.Helper()
	h := &relayHarness{prompt: make(chan string, 8)}
	h.svc = NewService(ServiceConfig{
		Config: Config{InboxID: "inbox", AllowedSenders: allowed},
		Client: client,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Notify: func(_ context.Context, prompt string) error {
			h.calls.Add(1)
			h.prompt <- prompt
			return nil
		},
	})
	return h
}

func relayMessage(id, from string) Message {
	return Message{ID: id, From: Address{Name: "RAW SENDER", Email: from}, Subject: "RAW SUBJECT", Text: "RAW BODY", Labels: []string{"unread"}}
}

func TestRelayWithAllowedSendersNotifiesOnlyAllowlistedSenders(t *testing.T) {
	var patches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			patches.Add(1)
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()
	client, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "fixture-only", HTTPClient: srv.Client(), DisableRetries: true})
	if err != nil {
		t.Fatal(err)
	}
	h := newRelayHarness(t, []string{"@Trusted.example"}, client)

	if err := h.svc.handleMessage(context.Background(), relayMessage("m-stranger", "mallory@evil.example")); err != nil {
		t.Fatalf("stranger: %v", err)
	}
	if h.calls.Load() != 0 {
		t.Fatalf("a sender outside the allowlist caused %d notifications", h.calls.Load())
	}
	if !h.svc.isSeen("m-stranger") {
		t.Fatal("an ignored message must be marked seen, or every poll logs it again")
	}
	if len(h.svc.pendingLabels) != 0 || patches.Load() != 0 {
		t.Fatalf("an ignored message changed the mailbox: pending=%v patches=%d", h.svc.pendingLabels, patches.Load())
	}

	if err := h.svc.handleMessage(context.Background(), relayMessage("m-friend", "Anna@trusted.example")); err != nil {
		t.Fatalf("allowlisted: %v", err)
	}
	if h.calls.Load() != 1 {
		t.Fatalf("an allowlisted sender caused %d notifications, want 1", h.calls.Load())
	}
	if prompt := <-h.prompt; !strings.Contains(prompt, "[AGENTMAIL NOTIFICATION]") || strings.Contains(prompt, "[QUARANTINE NOTICE]") {
		t.Fatalf("allowlisted mail did not reach the agent as a normal notification: %s", prompt)
	}
}

func TestRelayWithoutAllowedSendersDeliversOnlyTheQuarantineNotice(t *testing.T) {
	h := newRelayHarness(t, nil, nil)
	if err := h.svc.handleMessage(context.Background(), relayMessage("m-open", "anyone@example.test")); err != nil {
		t.Fatal(err)
	}
	if h.calls.Load() != 1 {
		t.Fatalf("empty allowlist caused %d notifications, want exactly the notice", h.calls.Load())
	}
	prompt := <-h.prompt
	if !strings.Contains(prompt, "[QUARANTINE NOTICE]") || !strings.Contains(prompt, "m-open") {
		t.Fatalf("not the quarantine notice with the message id: %s", prompt)
	}
	for _, leaked := range []string{"RAW SENDER", "anyone@example.test", "RAW SUBJECT", "RAW BODY", "[AGENTMAIL NOTIFICATION]"} {
		if strings.Contains(prompt, leaked) {
			t.Fatalf("notice leaked %q: %s", leaked, prompt)
		}
	}
	if !h.svc.isSeen("m-open") || len(h.svc.pendingLabels) != 0 {
		t.Fatalf("noticed message: seen=%v pending=%v, want seen without labels", h.svc.isSeen("m-open"), h.svc.pendingLabels)
	}
}

// A WebSocket event can carry only the message id. With an allowlist the relay
// asks for the sender before it decides; a failed lookup is retried, not guessed.
func TestRelayLooksUpTheSenderOfAnEventThatLacksOne(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/messages/m-event") {
			fmt.Fprint(w, `{"message_id":"m-event","from":{"email":"anna@trusted.example"},"subject":"hi"}`)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()
	client, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "fixture-only", HTTPClient: srv.Client(), DisableRetries: true})
	if err != nil {
		t.Fatal(err)
	}
	h := newRelayHarness(t, []string{"@trusted.example"}, client)
	event := Message{ID: "m-event", Subject: "hi", InboxID: "inbox"}

	fail.Store(true)
	if err := h.svc.handleMessage(context.Background(), event); err == nil {
		t.Fatal("a failed sender lookup must return an error so the message is retried")
	}
	if h.svc.isSeen("m-event") || h.calls.Load() != 0 {
		t.Fatalf("failed lookup: seen=%v notifications=%d", h.svc.isSeen("m-event"), h.calls.Load())
	}

	fail.Store(false)
	if err := h.svc.handleMessage(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if h.calls.Load() != 1 {
		t.Fatalf("event from an allowlisted sender caused %d notifications, want 1", h.calls.Load())
	}
}
