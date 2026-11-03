package server

import (
	"context"

	"aurago/internal/config"
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

// whatsAppOwnerAPI is the part of the running Bot that server features other than the agent
// tools send through: camera alert photos and newspaper editions. Every method passes the
// Bot's own gate (ownerSendAllowed: live readonly, egg mode, generation) and reaches the owners
// only.
type whatsAppOwnerAPI interface {
	OwnerSendReady() bool
	SendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error)
	SendOwnerAttachment(ctx context.Context, caption string, att whatsapp.OwnerAttachment) (tools.WhatsAppSendResult, error)
}

var _ whatsAppOwnerAPI = (*whatsapp.Bot)(nil)

// serverWhatsAppOwner returns the running Bot, or nil (never a typed nil). Tests replace it.
var serverWhatsAppOwner = func(s *Server) whatsAppOwnerAPI {
	if bot := s.whatsAppBot.Load(); bot != nil {
		return bot
	}
	return nil
}

// whatsAppOwnerReady reports that a server feature may send to WhatsApp now: the section is
// enabled and writable and a linked generation runs. The owner API checks readonly again on
// the live configuration, so a toggle between this check and the send still wins.
func (s *Server) whatsAppOwnerReady(cfg *config.Config) bool {
	if cfg == nil || !cfg.WhatsApp.Enabled || cfg.WhatsApp.ReadOnly {
		return false
	}
	api := serverWhatsAppOwner(s)
	return api != nil && api.OwnerSendReady()
}
