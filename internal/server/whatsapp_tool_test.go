package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
	"aurago/internal/whatsapp"
)

// probeWhatsAppSender is a comparable stand-in that always reports ready.
type probeWhatsAppSender struct{ _ byte }

func (*probeWhatsAppSender) Ready() bool { return true }
func (*probeWhatsAppSender) SendOwnerMessage(context.Context, string, string) (tools.WhatsAppSendResult, error) {
	return tools.WhatsAppSendResult{}, nil
}

func TestWhatsAppOwnerSenderFollowsTheRunningBot(t *testing.T) {
	w := whatsAppOwnerSender{s: &Server{}}
	if w.Ready() {
		t.Fatal("a server without a Bot reports ready")
	}
	if res, err := w.SendOwnerMessage(context.Background(), "hi", ""); !errors.Is(err, whatsapp.ErrNotConnected) || res != (tools.WhatsAppSendResult{}) {
		t.Fatalf("send without a Bot = %+v, %v; want ErrNotConnected", res, err)
	}
}

// The server publishes its sender once, in configureWhatsAppBot, and never again after shutdown;
// stopWhatsApp withdraws only a sender that is still its own. The hook is process-wide state, so
// the test does not run in parallel.
func TestWhatsAppRuntimePublishesTheOwnerSender(t *testing.T) {
	probe, newer := &probeWhatsAppSender{}, &probeWhatsAppSender{}
	tools.SetWhatsAppOwnerSender(probe)
	t.Cleanup(func() { tools.SetWhatsAppOwnerSender(nil) })
	s := whatsAppRuntimeServer(t, nil)

	s.configureWhatsAppBot()
	if tools.WhatsAppAvailable() {
		t.Fatal("configureWhatsAppBot left the earlier sender published; the server's own (not ready without a Bot) must replace it")
	}

	tools.SetWhatsAppOwnerSender(newer)
	s.stopWhatsApp()
	if !tools.WhatsAppAvailable() {
		t.Fatal("stopWhatsApp withdrew a sender that is not the server's")
	}

	tools.SetWhatsAppOwnerSender(probe)
	s.configureWhatsAppBot()
	if !tools.WhatsAppAvailable() {
		t.Fatal("configureWhatsAppBot published a sender after shutdown")
	}
}

// productionServerWhatsAppOwner is the seam as shipped, captured before any test swaps it.
var productionServerWhatsAppOwner = serverWhatsAppOwner

// fakeWhatsAppOwner stands in for the running Bot in the server-side WhatsApp features (camera
// alerts and newspaper delivery). It records what was sent and fails on request.
type fakeWhatsAppOwner struct {
	mu             sync.Mutex
	ready          bool
	attachErr      error
	attachTextSent bool
	msgErrs        []error  // consumed one per SendOwnerMessage call; nil once used up
	attachments    []string // name|caption|len
	messages       []string
}

var _ whatsAppOwnerAPI = (*fakeWhatsAppOwner)(nil)

func (f *fakeWhatsAppOwner) OwnerSendReady() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ready
}

func (f *fakeWhatsAppOwner) SendOwnerMessage(_ context.Context, text, _ string) (tools.WhatsAppSendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, text)
	var err error
	if len(f.msgErrs) > 0 {
		err, f.msgErrs = f.msgErrs[0], f.msgErrs[1:]
	}
	return tools.WhatsAppSendResult{Kind: "text", TextSent: err == nil}, err
}

func (f *fakeWhatsAppOwner) SendOwnerAttachment(_ context.Context, caption string, att whatsapp.OwnerAttachment) (tools.WhatsAppSendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attachments = append(f.attachments, fmt.Sprintf("%s|%s|%d", att.Name, caption, len(att.Data)))
	return tools.WhatsAppSendResult{Kind: "image", File: att.Name, TextSent: f.attachTextSent}, f.attachErr
}

// useFakeWhatsAppOwner swaps serverWhatsAppOwner for the test; a nil fake means no running Bot.
// The seam is process-wide, so tests that use it do not run in parallel.
func useFakeWhatsAppOwner(t *testing.T, f *fakeWhatsAppOwner) {
	t.Helper()
	previous := serverWhatsAppOwner
	serverWhatsAppOwner = func(*Server) whatsAppOwnerAPI {
		if f == nil {
			return nil
		}
		return f
	}
	t.Cleanup(func() { serverWhatsAppOwner = previous })
}

func TestWhatsAppOwnerReadyNeedsEnabledWritableAndRunning(t *testing.T) {
	s := &Server{}
	cfg := func(enabled, readOnly bool) *config.Config {
		c := &config.Config{}
		c.WhatsApp.Enabled = enabled
		c.WhatsApp.ReadOnly = readOnly
		return c
	}
	cases := []struct {
		name  string
		cfg   *config.Config
		fake  *fakeWhatsAppOwner
		ready bool
	}{
		{"no configuration", nil, &fakeWhatsAppOwner{ready: true}, false},
		{"disabled", cfg(false, false), &fakeWhatsAppOwner{ready: true}, false},
		{"read-only", cfg(true, true), &fakeWhatsAppOwner{ready: true}, false},
		{"not linked or not running", cfg(true, false), &fakeWhatsAppOwner{ready: false}, false},
		{"no running Bot", cfg(true, false), nil, false},
		{"enabled, writable and ready", cfg(true, false), &fakeWhatsAppOwner{ready: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useFakeWhatsAppOwner(t, tc.fake)
			if got := s.whatsAppOwnerReady(tc.cfg); got != tc.ready {
				t.Fatalf("whatsAppOwnerReady = %v, want %v", got, tc.ready)
			}
		})
	}

	// The production seam returns an untyped nil without a Bot, so the nil checks hold.
	if api := productionServerWhatsAppOwner(s); api != nil {
		t.Fatalf("production seam without a Bot = %#v, want nil (no typed nil)", api)
	}
	prev := serverWhatsAppOwner
	serverWhatsAppOwner = productionServerWhatsAppOwner
	t.Cleanup(func() { serverWhatsAppOwner = prev })
	if s.whatsAppOwnerReady(cfg(true, false)) {
		t.Fatal("whatsAppOwnerReady is true without a running Bot")
	}
}
