package server

import (
	"context"
	"errors"
	"testing"

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
