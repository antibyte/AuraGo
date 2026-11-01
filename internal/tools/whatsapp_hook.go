package tools

import (
	"context"
	"sync"
)

// WhatsAppSendResult describes what one owner send delivered.
type WhatsAppSendResult struct {
	Kind     string // "text", "image", "video", "audio" or "document"
	File     string // base name of the sent file; "" for text
	TextSent bool   // the text went out as a message of its own (also set when the file then failed)
}

// WhatsAppOwnerSender is the owner-only send surface of the running WhatsApp Bot. The server
// publishes it; internal/tools and internal/agent never import internal/whatsapp, which
// imports them both. Implementations must be comparable.
type WhatsAppOwnerSender interface {
	// Ready reports that a linked generation runs. Readonly is not part of it: every caller
	// checks the live configuration itself.
	Ready() bool
	// SendOwnerMessage sends text and/or the workspace or documents file at filePath to the
	// owners and nobody else. Its errors keep errors.Is for the sentinels of internal/whatsapp
	// and carry no phone numbers.
	SendOwnerMessage(ctx context.Context, text, filePath string) (WhatsAppSendResult, error)
}

var whatsAppOwner struct {
	sync.RWMutex
	sender WhatsAppOwnerSender
}

// SetWhatsAppOwnerSender publishes s, replacing any earlier sender. nil withdraws.
func SetWhatsAppOwnerSender(s WhatsAppOwnerSender) {
	whatsAppOwner.Lock()
	whatsAppOwner.sender = s
	whatsAppOwner.Unlock()
}

// WithdrawWhatsAppOwnerSender clears the published sender only while s is still the published
// one, so a late shutdown never withdraws a newer owner's sender.
func WithdrawWhatsAppOwnerSender(s WhatsAppOwnerSender) {
	whatsAppOwner.Lock()
	defer whatsAppOwner.Unlock()
	if whatsAppOwner.sender != nil && sameWhatsAppSender(whatsAppOwner.sender, s) {
		whatsAppOwner.sender = nil
	}
}

// sameWhatsAppSender compares two senders. An implementation that is not comparable (which the
// interface forbids) is never the same, instead of panicking.
func sameWhatsAppSender(a, b WhatsAppOwnerSender) (same bool) {
	defer func() {
		if recover() != nil {
			same = false
		}
	}()
	return a == b
}

// WhatsAppAvailable reports whether a sender is published and a linked generation runs. It does
// not look at readonly; the send path and the tool gates do.
func WhatsAppAvailable() bool {
	s := currentWhatsAppOwnerSender()
	return s != nil && s.Ready()
}

// currentWhatsAppOwnerSender returns the published sender, or nil. Callers invoke it without
// the lock held, so a slow Ready or send never blocks a publication.
func currentWhatsAppOwnerSender() WhatsAppOwnerSender {
	whatsAppOwner.RLock()
	defer whatsAppOwner.RUnlock()
	return whatsAppOwner.sender
}
