package whatsapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"slices"
	"strings"
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
	l, finished := startTestLink(t, LinkRequest{Method: LinkMethodQR}, st)
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
	<-l.Done()
	st.mu.Lock()
	closed := st.closed
	st.mu.Unlock()
	if l.Active() || l.Snapshot().State != LinkStateSuccess || clientCount(client, func(f *fakeClient) int { return f.disconnects }) == 0 || closed != 1 {
		t.Fatalf("the link session still holds the device: snapshot=%+v closed=%d", l.Snapshot(), closed)
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
