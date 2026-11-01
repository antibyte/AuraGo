package server

import (
	"context"

	"aurago/internal/tools"
	"aurago/internal/whatsapp"
)

// whatsAppOwnerSender is the hook the server publishes to internal/tools. It follows the running
// generation on every call, so it is published once and never refers to a stopped Bot. The
// struct is comparable, as tools.WithdrawWhatsAppOwnerSender requires.
type whatsAppOwnerSender struct{ s *Server }

var _ tools.WhatsAppOwnerSender = whatsAppOwnerSender{}

func (w whatsAppOwnerSender) Ready() bool {
	bot := w.s.whatsAppBot.Load()
	return bot != nil && bot.OwnerSendReady()
}

func (w whatsAppOwnerSender) SendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error) {
	bot := w.s.whatsAppBot.Load()
	if bot == nil {
		return tools.WhatsAppSendResult{}, whatsapp.ErrNotConnected
	}
	return bot.SendOwnerMessage(ctx, text, filePath)
}
