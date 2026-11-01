package whatsapp

import (
	"strings"
	"time"

	"aurago/internal/config"

	"go.mau.fi/whatsmeow/types"
)

// ownerRecipients is the single owner-recipient resolver. Every send goes to
// the configured owners (dedicated mode) or to the linked account itself
// (self_chat mode). No API accepts arbitrary numbers.
func ownerRecipients(cfg config.WhatsAppConfig, own types.JID) ([]types.JID, error) {
	if cfg.Mode == config.WhatsAppModeSelfChat {
		if own.IsEmpty() {
			return nil, ErrNoRecipient
		}
		return []types.JID{own.ToNonAD()}, nil
	}
	out := make([]types.JID, 0, len(cfg.OwnerNumbers))
	for _, raw := range cfg.OwnerNumbers {
		number, err := config.NormalizeE164(raw)
		if err != nil {
			continue
		}
		out = append(out, types.NewJID(strings.TrimPrefix(number, "+"), types.DefaultUserServer))
	}
	if len(out) == 0 {
		// Dedicated mode needs an owner number: a settings problem, not a connection one.
		return nil, ErrNotConfigured
	}
	return out, nil
}

// testMessageText is the manual test message. It carries no self-chat prefix:
// the sender adds self_chat_prefix to every outgoing text, this one included.
func testMessageText(now time.Time) string {
	return "AuraGo WhatsApp test (manual) — " + now.UTC().Format(time.RFC3339)
}
