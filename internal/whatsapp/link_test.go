package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow"
)

func fakeLinkDeps(st *fakeStore) linkDeps {
	return linkDeps{backend: &fakeBackend{store: st}, now: fixedNow, render: func(_ context.Context, payload string) (string, error) {
		return "data:image/png;base64," + payload, nil
	}}
}

func startTestLink(t *testing.T, req LinkRequest, st *fakeStore) (*LinkSession, chan bool) {
	t.Helper()
	finished := make(chan bool, 1)
	l, err := startLink(context.Background(), testBotConfig(config.WhatsAppModeSelfChat), req, quietLogger(), func(linked bool) { finished <- linked }, fakeLinkDeps(st))
	if err != nil {
		t.Fatalf("startLink: %v", err)
	}
	t.Cleanup(func() { l.Cancel(); <-l.Done() })
	return l, finished
}

func waitLinkState(t *testing.T, l *LinkSession, state string) {
	t.Helper()
	waitFor(t, "link state "+state, func() bool { return l.Snapshot().State == state })
}

func waitFinished(t *testing.T, finished <-chan bool) bool {
	t.Helper()
	select {
	case linked := <-finished:
		return linked
	case <-time.After(5 * time.Second):
		t.Fatal("link session did not finish")
		return false
	}
}

func TestStartLinkRejectsInvalidRequests(t *testing.T) {
	cfg := testBotConfig(config.WhatsAppModeSelfChat)
	disabled := cfg.Clone()
	disabled.WhatsApp.Enabled = false
	unlinked := &fakeStore{client: &fakeClient{}}
	for _, c := range []struct {
		cfg  *config.Config
		req  LinkRequest
		want error
	}{
		{disabled, LinkRequest{Method: LinkMethodQR}, ErrNotConfigured},
		{cfg, LinkRequest{Method: "sms"}, ErrInvalidLinkMethod},
		{cfg, LinkRequest{Method: LinkMethodPhone, Phone: "01701234567"}, ErrInvalidPhone},
	} {
		if _, err := startLink(context.Background(), c.cfg, c.req, quietLogger(), nil, fakeLinkDeps(unlinked)); !errors.Is(err, c.want) {
			t.Errorf("startLink(%+v) = %v, want %v", c.req, err, c.want)
		}
	}
	linked := &fakeStore{linked: true, own: testOwn, client: &fakeClient{}}
	if _, err := startLink(context.Background(), cfg, LinkRequest{Method: LinkMethodQR}, quietLogger(), nil, fakeLinkDeps(linked)); !errors.Is(err, ErrAlreadyLinked) {
		t.Fatalf("linked store: %v, want ErrAlreadyLinked", err)
	}
	linked.mu.Lock()
	closed := linked.closed
	linked.mu.Unlock()
	if closed != 1 {
		t.Fatalf("store closed %d times after ErrAlreadyLinked, want 1", closed)
	}
}

func TestLinkSessionQRSuccessHandsOverTheDevice(t *testing.T) {
	client := &fakeClient{qr: make(chan whatsmeow.QRChannelItem, 4)}
	st := &fakeStore{client: client}
	// onFinish is where the server starts the next Bot, so the session must
	// already have disconnected and released the store by then.
	var closedAtFinish, disconnectsAtFinish int
	finished := make(chan bool, 1)
	l, err := startLink(context.Background(), testBotConfig(config.WhatsAppModeSelfChat), LinkRequest{Method: LinkMethodQR}, quietLogger(), func(linked bool) {
		st.mu.Lock()
		closedAtFinish = st.closed
		st.mu.Unlock()
		disconnectsAtFinish = clientCount(client, func(f *fakeClient) int { return f.disconnects })
		finished <- linked
	}, fakeLinkDeps(st))
	if err != nil {
		t.Fatalf("startLink: %v", err)
	}
	t.Cleanup(func() { l.Cancel(); <-l.Done() })
	if snap := l.Snapshot(); snap.State != LinkStateWaiting || snap.ExpiresAt != "2026-10-31T12:03:00Z" {
		t.Fatalf("initial snapshot = %+v", snap)
	}
	if got := integrationstatus.WhatsAppStatus().State; got != integrationstatus.WhatsAppStateLinking {
		t.Fatalf("status = %q, want linking", got)
	}
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "2@fake-ref,fake-keys", Timeout: time.Minute}
	waitLinkState(t, l, LinkStateQR)
	if snap := l.Snapshot(); snap.QRPNG != "data:image/png;base64,2@fake-ref,fake-keys" || snap.ExpiresAt != "2026-10-31T12:03:00Z" {
		t.Fatalf("qr snapshot = %+v", snap)
	}
	client.qr <- whatsmeow.QRChannelSuccess
	if !waitFinished(t, finished) {
		t.Fatal("success reported as not linked")
	}
	if closedAtFinish != 1 || disconnectsAtFinish == 0 {
		t.Fatalf("onFinish ran before the hand-over: store closed %d times, client disconnected %d times", closedAtFinish, disconnectsAtFinish)
	}
	<-l.Done()
	if l.Active() || l.Snapshot().State != LinkStateSuccess {
		t.Fatalf("the link session still holds the device: snapshot=%+v", l.Snapshot())
	}
}

func TestLinkSessionPhoneCodeTimeoutPasskeyAndCancel(t *testing.T) {
	client := &fakeClient{qr: make(chan whatsmeow.QRChannelItem, 4), pairCode: "ABCD-EFGH"}
	l, finished := startTestLink(t, LinkRequest{Method: LinkMethodPhone, Phone: "+49 170 1234567"}, &fakeStore{client: client})
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "2@first"}
	waitLinkState(t, l, LinkStateCode)
	if snap := l.Snapshot(); snap.PairingCode != "ABCD-EFGH" || snap.QRPNG != "" {
		t.Fatalf("code snapshot = %+v", snap)
	}
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "2@second"}
	client.qr <- whatsmeow.QRChannelTimeout
	if waitFinished(t, finished) {
		t.Fatal("a timeout reported a linked device")
	}
	var phones []string
	client.with(func(f *fakeClient) { phones = slices.Clone(f.pairPhones) })
	if l.Snapshot().State != LinkStateTimeout || !slices.Equal(phones, []string{"491701234567"}) {
		t.Fatalf("snapshot=%+v pairPhones=%v", l.Snapshot(), phones)
	}
	if got := integrationstatus.WhatsAppStatus().State; got != integrationstatus.WhatsAppStateNotLinked {
		t.Fatalf("status after timeout = %q", got)
	}
	passkey := &fakeClient{qr: make(chan whatsmeow.QRChannelItem, 1)}
	l2, finished2 := startTestLink(t, LinkRequest{Method: LinkMethodQR}, &fakeStore{client: passkey})
	passkey.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventPasskeyRequest}
	waitFinished(t, finished2)
	if snap := l2.Snapshot(); snap.State != LinkStateError || snap.Error != "passkey_unsupported" {
		t.Fatalf("passkey snapshot = %+v", snap)
	}
	idle := &fakeClient{qr: make(chan whatsmeow.QRChannelItem)}
	l3, finished3 := startTestLink(t, LinkRequest{Method: LinkMethodQR}, &fakeStore{client: idle})
	l3.Cancel()
	waitFinished(t, finished3)
	if snap := l3.Snapshot(); snap.State != LinkStateError || snap.Error != "cancelled" {
		t.Fatalf("cancelled snapshot = %+v", snap)
	}
}

func TestRenderQRDataURIProducesVerifiedPNG(t *testing.T) {
	payload := "2@" + strings.Repeat("Ab3+/", 30) + "," + strings.Repeat("Zx9=", 20)
	uri, err := renderQRDataURI(context.Background(), payload)
	if err != nil {
		t.Fatalf("renderQRDataURI: %v", err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("uri prefix = %q", uri[:min(len(uri), 32)])
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != linkQRSize || cfg.Height != linkQRSize {
		t.Fatalf("png = %+v, %v", cfg, err)
	}
}

func TestLinkSessionClosedQRChannelFollowsTheContext(t *testing.T) {
	cfg := testBotConfig(config.WhatsAppModeSelfChat)
	run := func(parent context.Context) LinkSnapshot {
		qr := make(chan whatsmeow.QRChannelItem)
		close(qr) // whatsmeow closes the channel when the QR codes run out or ctx ends
		finished := make(chan bool, 1)
		l, err := startLink(parent, cfg, LinkRequest{Method: LinkMethodQR}, quietLogger(), func(linked bool) { finished <- linked },
			fakeLinkDeps(&fakeStore{client: &fakeClient{qr: qr}}))
		if err != nil {
			t.Fatalf("startLink: %v", err)
		}
		waitFinished(t, finished)
		<-l.Done()
		return l.Snapshot()
	}
	if snap := run(context.Background()); snap.State != LinkStateTimeout {
		t.Fatalf("an exhausted QR channel = %+v, want timeout", snap)
	}
	// With the context already cancelled, the closed channel and ctx.Done are
	// both ready and select picks either one: the answer must not depend on it.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 30; i++ {
		if snap := run(cancelled); snap.State != LinkStateError || snap.Error != "cancelled" {
			t.Fatalf("run %d: a cancelled session = %+v, want error/cancelled", i, snap)
		}
	}
}

// syncBuffer is a log sink that other goroutines may write while a test reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestLinkSessionNeverLogsTheQRPayload(t *testing.T) {
	const payload = "2@SECRETREF9f3a,SECRETNOISEKEY,SECRETIDENTITYKEY,SECRETADVKEY"
	var logs syncBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })

	client := &fakeClient{qr: make(chan whatsmeow.QRChannelItem, 4)}
	finished := make(chan bool, 1)
	deps := fakeLinkDeps(&fakeStore{client: client})
	deps.render = renderQRDataURI
	l, err := startLink(context.Background(), testBotConfig(config.WhatsAppModeSelfChat), LinkRequest{Method: LinkMethodQR}, logger, func(linked bool) { finished <- linked }, deps)
	if err != nil {
		t.Fatalf("startLink: %v", err)
	}
	t.Cleanup(func() { l.Cancel(); <-l.Done() })
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: payload}
	waitLinkState(t, l, LinkStateQR)
	client.qr <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventError, Error: errors.New("pair error for +491701234567")}
	waitFinished(t, finished)
	out := logs.String()
	if !strings.Contains(out, "WhatsApp link failed") {
		t.Fatalf("the failure was not logged, so this test proves nothing: %q", out)
	}
	if strings.Contains(out, "SECRET") || strings.Contains(out, "491701234567") {
		t.Fatalf("the log leaks the QR payload or a full number: %s", out)
	}
}
