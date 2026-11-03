package tools

import (
	"context"
	"testing"
)

// fakeWhatsAppSender is comparable (a pointer), as the interface requires.
type fakeWhatsAppSender struct{ ready bool }

func (f *fakeWhatsAppSender) Ready() bool { return f.ready }
func (f *fakeWhatsAppSender) SendOwnerMessage(context.Context, string, string) (WhatsAppSendResult, error) {
	return WhatsAppSendResult{Kind: "text", TextSent: true}, nil
}

// unorderedWhatsAppSender is not comparable: comparing two of them panics.
type unorderedWhatsAppSender struct{ parts []string }

func (unorderedWhatsAppSender) Ready() bool { return true }
func (unorderedWhatsAppSender) SendOwnerMessage(context.Context, string, string) (WhatsAppSendResult, error) {
	return WhatsAppSendResult{}, nil
}

// Not parallel: the hook is process-wide state.
func TestWhatsAppOwnerSenderPublication(t *testing.T) {
	SetWhatsAppOwnerSender(nil)
	t.Cleanup(func() { SetWhatsAppOwnerSender(nil) })
	if WhatsAppAvailable() || currentWhatsAppOwnerSender() != nil {
		t.Fatal("a fresh hook reports a sender")
	}

	published, other := &fakeWhatsAppSender{ready: true}, &fakeWhatsAppSender{ready: true}
	SetWhatsAppOwnerSender(published)
	if !WhatsAppAvailable() || currentWhatsAppOwnerSender() != WhatsAppOwnerSender(published) {
		t.Fatal("a ready sender must make WhatsApp available")
	}

	WithdrawWhatsAppOwnerSender(other)
	if currentWhatsAppOwnerSender() != WhatsAppOwnerSender(published) {
		t.Fatal("withdrawing another sender removed the published one")
	}
	WithdrawWhatsAppOwnerSender(nil)
	if currentWhatsAppOwnerSender() == nil {
		t.Fatal("withdrawing nil removed the published sender")
	}

	// A sender that is published but not ready is not available; availability follows Ready.
	published.ready = false
	if WhatsAppAvailable() {
		t.Fatal("a sender that is not ready was reported available")
	}
	published.ready = true
	if !WhatsAppAvailable() {
		t.Fatal("availability did not follow Ready back to true")
	}

	// A newer sender replaces the old one; the old one's late withdrawal must not clear it.
	SetWhatsAppOwnerSender(other)
	WithdrawWhatsAppOwnerSender(published)
	if currentWhatsAppOwnerSender() != WhatsAppOwnerSender(other) {
		t.Fatal("a late withdrawal cleared the newer sender")
	}

	WithdrawWhatsAppOwnerSender(other)
	if currentWhatsAppOwnerSender() != nil || WhatsAppAvailable() {
		t.Fatal("withdrawing the published sender must clear it")
	}

	// An implementation that cannot be compared must not panic the withdrawal.
	SetWhatsAppOwnerSender(unorderedWhatsAppSender{})
	WithdrawWhatsAppOwnerSender(unorderedWhatsAppSender{})
}
