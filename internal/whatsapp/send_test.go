package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

var (
	ownerJID    = types.NewJID("4917012345678", types.DefaultUserServer)
	strangerJID = types.NewJID("4917099998888", types.DefaultUserServer)
)

// newTestSender builds a sender around fake. Its sleep returns at once, so pacing never
// slows a test.
func newTestSender(cfg config.WhatsAppConfig, client waClient) (*sender, *idCache) {
	sent := newIDCache(100, time.Hour, fixedNow)
	deps := runtimeDeps{sleep: func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }, now: fixedNow}
	return newSender(cfg, func() waClient { return client }, sent, deps, quietLogger()), sent
}

func deliveryFailures() int { return integrationstatus.WhatsAppStatus().DeliveryFailures }

func sentTexts(f *fakeClient) []sentMessage {
	var out []sentMessage
	f.with(func(f *fakeClient) { out = slices.Clone(f.sent) })
	return out
}

func TestSenderRefusesNonOwnerRecipients(t *testing.T) {
	ctx := context.Background()
	fake := &fakeClient{own: testOwn, loggedIn: true}
	s, sent := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, fake)

	before := deliveryFailures()
	if err := s.deliverText(ctx, strangerJID, "hello", "reply"); !errors.Is(err, errNotOwnerRecipient) {
		t.Fatalf("send to a stranger = %v, want errNotOwnerRecipient", err)
	}
	if got := sentTexts(fake); len(got) != 0 || sent.Len() != 0 {
		t.Fatalf("a stranger received %+v", got)
	}
	if deliveryFailures() != before {
		t.Fatal("a refused recipient must not count as a delivery failure")
	}

	if err := s.deliverText(ctx, ownerJID, "hello", "reply"); err != nil {
		t.Fatalf("send to the owner: %v", err)
	}
	if got := sentTexts(fake); len(got) != 1 || got[0].to != ownerJID || got[0].text != "hello" {
		t.Fatalf("owner sends = %+v", got)
	}
}

func TestTokenBucketAllowsBurstThenTwentyPerMinute(t *testing.T) {
	now := fixedNow()
	var sleeps []time.Duration
	bucket := tokenBucket{
		now: func() time.Time { return now },
		sleep: func(_ context.Context, d time.Duration) bool {
			sleeps = append(sleeps, d)
			now = now.Add(d)
			return true
		},
	}
	ctx := context.Background()
	take := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if !bucket.take(ctx) {
				t.Fatal("take failed")
			}
		}
	}

	take(5)
	if len(sleeps) != 0 {
		t.Fatalf("the burst of five must not sleep, slept %v", sleeps)
	}
	take(1)
	if !slices.Equal(sleeps, []time.Duration{3 * time.Second}) {
		t.Fatalf("the sixth take must sleep 3s, slept %v", sleeps)
	}

	now = now.Add(time.Minute) // refills far more than the burst; the cap is five
	take(5)
	if len(sleeps) != 1 {
		t.Fatalf("five takes after a pause must not sleep, slept %v", sleeps)
	}
	take(1)
	if !slices.Equal(sleeps, []time.Duration{3 * time.Second, 3 * time.Second}) {
		t.Fatalf("the burst is capped at five, slept %v", sleeps)
	}

	// A clock that never moves (the Bot tests use a fixed time) must not stall the queue.
	frozen := tokenBucket{now: fixedNow, sleep: func(context.Context, time.Duration) bool { return true }}
	for i := 0; i < 12; i++ {
		if !frozen.take(ctx) {
			t.Fatal("take failed on a frozen clock")
		}
	}

	stopped := tokenBucket{now: func() time.Time { return now }, sleep: func(context.Context, time.Duration) bool { return false }}
	for i := 0; i < 5; i++ {
		stopped.take(ctx)
	}
	if stopped.take(ctx) {
		t.Fatal("take must report a sleep that ctx cut short")
	}
}

// cancelOnSendClient cancels the caller's context while the send is in flight.
type cancelOnSendClient struct {
	*fakeClient
	cancel context.CancelFunc
}

func (c cancelOnSendClient) SendMessage(context.Context, types.JID, *waE2E.Message, types.MessageID) (whatsmeow.SendResponse, error) {
	c.cancel()
	return whatsmeow.SendResponse{}, context.Canceled
}

func TestSenderRegistersSentIDsAndCountsFailures(t *testing.T) {
	ctx := context.Background()
	fake := &fakeClient{own: testOwn, loggedIn: true}
	s, sent := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, fake)

	if err := s.deliverText(ctx, ownerJID, "hello", "reply"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !sent.Contains("fake-1") {
		t.Fatal("the ID of a sent message must be in the echo cache")
	}

	before := deliveryFailures()
	fake.with(func(f *fakeClient) { f.sendErrs = []error{errors.New("boom")} })
	if err := s.deliverText(ctx, ownerJID, "hello", "reply"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("a failed send = %v, want the cause", err)
	}
	// The ID is registered before the call, so the failed message's ID stays in the cache.
	if deliveryFailures() != before+1 || sent.Len() != 2 || !sent.Contains("fake-2") {
		t.Fatalf("failures %d (want %d), cached IDs %d (want 2)", deliveryFailures(), before+1, sent.Len())
	}

	// A cancelled context, before the send or during it, is not a delivery failure.
	before = deliveryFailures()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.deliverText(cancelled, ownerJID, "hello", "reply"); !errors.Is(err, context.Canceled) {
		t.Fatalf("send with a cancelled context = %v", err)
	}
	inFlight, cancelInFlight := context.WithCancel(ctx)
	s2, _ := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, cancelOnSendClient{fake, cancelInFlight})
	if err := s2.deliverText(inFlight, ownerJID, "hello", "reply"); err == nil {
		t.Fatal("a send cut short by cancellation must report it")
	}
	if deliveryFailures() != before {
		t.Fatal("a cancelled send must not be counted as a delivery failure")
	}
}

// An echo of our own message can reach the inbound filter before SendMessage returns, so the
// sender fixes the ID first, registers it, and sends under exactly that ID.
func TestSenderRegistersEachIDBeforeSending(t *testing.T) {
	ctx := context.Background()
	fake := &fakeClient{own: testOwn, loggedIn: true}
	s, sent := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, fake)
	var inCache []bool // per SendMessage call: was the ID already in the cache when it began?
	fake.with(func(f *fakeClient) {
		f.beforeSend = func(id types.MessageID) { inCache = append(inCache, sent.Contains(string(id))) }
	})

	if err := s.deliverText(ctx, ownerJID, strings.Repeat("word ", 1200), "reply"); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := sentTexts(fake)
	if len(got) < 2 || len(inCache) != len(got) {
		t.Fatalf("sends = %d, hook calls = %d, want the same number, at least 2 chunks", len(got), len(inCache))
	}
	ids := map[types.MessageID]bool{}
	for i, m := range got {
		if !inCache[i] || m.id == "" || ids[m.id] {
			t.Fatalf("chunk %d: id %q, registered before the call = %v, repeated = %v", i, m.id, inCache[i], ids[m.id])
		}
		ids[m.id] = true
	}

	// A failed send keeps its ID registered.
	inCache = nil
	fake.with(func(f *fakeClient) { f.sendErrs = []error{errors.New("boom")} })
	if err := s.deliverText(ctx, ownerJID, "hello", "reply"); err == nil {
		t.Fatal("a failed send must report its error")
	}
	if len(inCache) != 1 || !inCache[0] {
		t.Fatalf("failed send: registered before the call = %v", inCache)
	}
	if want := len(got) + 1; sent.Len() != want {
		t.Fatalf("cached IDs = %d, want %d (the failed one stays)", sent.Len(), want)
	}
}

// blockingClient holds every send until release is closed.
type blockingClient struct {
	*fakeClient
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingClient) SendMessage(ctx context.Context, to types.JID, msg *waE2E.Message, id types.MessageID) (whatsmeow.SendResponse, error) {
	c.once.Do(func() { close(c.started) })
	<-c.release
	return c.fakeClient.SendMessage(ctx, to, msg, id)
}

func TestSenderQueueWaitHonoursContext(t *testing.T) {
	fake := &fakeClient{own: testOwn, loggedIn: true}
	holder := &blockingClient{fakeClient: fake, started: make(chan struct{}), release: make(chan struct{})}
	s, _ := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, holder)

	first := make(chan error, 1)
	go func() { first <- s.deliverText(context.Background(), ownerJID, "first", "reply") }()
	select {
	case <-holder.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first send never reached the client")
	}

	before := deliveryFailures()
	ctx, cancel := context.WithCancel(context.Background())
	second := make(chan error, 1)
	go func() { second <- s.deliverText(ctx, ownerJID, "second", "reply") }()
	time.Sleep(50 * time.Millisecond) // let the second send queue up behind the first
	cancel()
	select {
	case err := <-second:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("a cancelled wait = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled send stayed blocked behind the running one")
	}
	select {
	case err := <-first:
		t.Fatalf("the first send ended early: %v", err)
	default:
	}
	if deliveryFailures() != before {
		t.Fatal("a send that never got the queue slot must not count as a delivery failure")
	}

	close(holder.release)
	select {
	case err := <-first:
		if err != nil {
			t.Fatalf("first send: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the first send never finished")
	}
	if got := sentBodies(fake); !slices.Equal(got, []string{"first"}) {
		t.Fatalf("sent %q, want only the first message", got)
	}
}

func TestSenderScrubsBeforeFormatting(t *testing.T) {
	// The formatter rewrites a pipe in a table row and a doubled asterisk pair, so a
	// registered value holding one must be redacted first.
	pipeSecret, starSecret := "pass|word-secret", "tok**en-secret"
	defer security.RegisterScopedSensitiveExact(pipeSecret)()
	defer security.RegisterScopedSensitiveExact(starSecret)()

	fake := &fakeClient{own: testOwn, loggedIn: true}
	s, _ := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, fake)
	table := "| name | value |\n|---|---|\n| user | " + pipeSecret + " |"
	stars := "first " + starSecret + " and again " + starSecret + " done"
	for _, text := range []string{table, stars} {
		if err := s.deliverText(context.Background(), ownerJID, text, "reply"); err != nil {
			t.Fatalf("send: %v", err)
		}
	}
	want := []string{"• name — value\n• user — [redacted]", "first [redacted] and again [redacted] done"}
	if got := sentBodies(fake); !slices.Equal(got, want) {
		t.Fatalf("sent %q, want %q", got, want)
	}
}

func TestSenderSurfacesRecipientNotOnWhatsApp(t *testing.T) {
	fake := &fakeClient{own: testOwn, loggedIn: true,
		sendErrs: []error{fmt.Errorf("%w: no LID found", whatsmeow.ErrRecipientNotOnWhatsApp)}}
	s, _ := newTestSender(testBotConfig(config.WhatsAppModeDedicated, "+4917012345678").WhatsApp, fake)
	err := s.deliverText(context.Background(), ownerJID, "hello", "reply")
	if !errors.Is(err, ErrRecipientNotOnWhatsApp) {
		t.Fatalf("send = %v, want ErrRecipientNotOnWhatsApp", err)
	}
}

func TestSenderPrefixesEveryChunkInSelfChat(t *testing.T) {
	fake := &fakeClient{own: testOwn, loggedIn: true}
	cfg := testBotConfig(config.WhatsAppModeSelfChat).WhatsApp
	s, _ := newTestSender(cfg, fake)

	words := make([]string, 1501)
	for i := range words {
		words[i] = fmt.Sprintf("w%04d", i)
	}
	body := strings.Join(words, " ") // 9005 runes
	if err := s.deliverText(context.Background(), testOwn, body, "reply"); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := sentTexts(fake)
	if len(got) < 3 {
		t.Fatalf("a %d-rune body must take at least 3 messages, got %d", utf8.RuneCountInString(body), len(got))
	}
	var parts []string
	for i, m := range got {
		if !strings.HasPrefix(m.text, cfg.SelfChatPrefix) {
			t.Fatalf("message %d lacks the self-chat prefix: %.20q", i, m.text)
		}
		if n := utf8.RuneCountInString(m.text); n > maxChunkRunes {
			t.Fatalf("message %d has %d runes", i, n)
		}
		if m.to != testOwn {
			t.Fatalf("message %d went to %v", i, m.to)
		}
		parts = append(parts, strings.TrimPrefix(m.text, cfg.SelfChatPrefix))
	}
	if strings.Join(parts, " ") != body {
		t.Fatal("the chunks must arrive in order and restore the body")
	}
}

func TestOnlyTheSenderCallsSendMessage(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var callers []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "SendMessage" && !slices.Contains(callers, name) {
					callers = append(callers, name)
				}
			}
			return true
		})
	}
	slices.Sort(callers)
	if want := []string{"client.go", "send.go"}; !slices.Equal(callers, want) {
		t.Fatalf("SendMessage is called from %v, want only %v (the adapter and the sender)", callers, want)
	}
}
