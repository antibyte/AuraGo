package tools

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"aurago/internal/config"
)

// whatsAppNotifyBudget bounds one WhatsApp notification, as telegramSendTotalBudget does for
// Telegram. A notification is sent from a goroutine of the caller's, which must not hang on a
// stuck connection.
const whatsAppNotifyBudget = 30 * time.Second

var (
	errWhatsAppDisabled   = errors.New("whatsapp is not enabled")
	errWhatsAppReadOnly   = errors.New("whatsapp is in read-only mode")
	errWhatsAppNotRunning = errors.New("whatsapp is not linked or not running")
)

// SendWhatsAppMessage is the body of the send_whatsapp tool: it sends text and/or one workspace or
// documents file to the owner's WhatsApp chat through the sender the server published, and
// returns the JSON the model reads. It never names a recipient; the Bot sends to the owners and
// nobody else. It refuses while WhatsApp is disabled or read-only and while no linked generation
// runs, so a caller other than the dispatch gets the same gates.
//
// The result mirrors SendTelegramFile: {"status", "kind", "results":[{"channel","status"}]}, plus
// "document" (the base name) when a file went out. "text_sent": true says the text went out as a
// message of its own; it is also set on an error, where it means only the file failed, so a retry
// sends the text again. Errors come masked from the Bot and carry no phone numbers.
func SendWhatsAppMessage(ctx context.Context, cfg *config.Config, text, filePath string) string {
	switch {
	case cfg == nil || !cfg.WhatsApp.Enabled:
		return whatsAppFailure(errWhatsAppDisabled, false)
	case cfg.WhatsApp.ReadOnly:
		return whatsAppFailure(errWhatsAppReadOnly, false)
	}
	sender := currentWhatsAppOwnerSender()
	if sender == nil || !sender.Ready() {
		return whatsAppFailure(errWhatsAppNotRunning, false)
	}
	res, err := sender.SendOwnerMessage(ctx, text, filePath)
	if err != nil {
		return whatsAppFailure(err, res.TextSent)
	}
	out := map[string]any{
		"status":  "success",
		"kind":    res.Kind,
		"results": []map[string]string{{"channel": "whatsapp", "status": "sent"}},
	}
	if res.File != "" {
		out["document"] = res.File
	}
	if res.TextSent {
		out["text_sent"] = true
	}
	return whatsAppJSON(out)
}

// whatsAppFailure encodes an error result; textSent marks that the text was delivered first.
func whatsAppFailure(err error, textSent bool) string {
	out := map[string]any{
		"status":  "error",
		"message": err.Error(),
		"results": []map[string]string{{"channel": "whatsapp", "status": "error", "detail": err.Error()}},
	}
	if textSent {
		out["text_sent"] = true
	}
	return whatsAppJSON(out)
}

func whatsAppJSON(v map[string]any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ErrorJSON("whatsapp result could not be encoded")
	}
	return string(data)
}

// sendWhatsAppNotification sends a notification to the owner's WhatsApp chat: the title in bold
// above the message, which the Bot converts to WhatsApp formatting. The caller has checked
// enabled and read-only. The error comes masked from the Bot.
func sendWhatsAppNotification(ctx context.Context, title, message string) error {
	sender := currentWhatsAppOwnerSender()
	if sender == nil || !sender.Ready() {
		return errWhatsAppNotRunning
	}
	ctx, cancel := context.WithTimeout(ctx, whatsAppNotifyBudget)
	defer cancel()
	_, err := sender.SendOwnerMessage(ctx, "**"+title+"**\n"+message, "")
	return err
}
