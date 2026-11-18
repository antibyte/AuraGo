package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"slices"
	"time"

	"aurago/internal/newspaper"
	"aurago/internal/security"
	"aurago/internal/whatsapp"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type newspaperMailAudio struct {
	Name, MIME string
	Data       []byte
}

func (s *Server) newspaperDeliverAudio(ctx context.Context, e newspaper.Edition, p newspaper.Profile, channel string, both bool, file newspaper.AudioFile) (string, error) {
	cfg := s.ConfigSnapshot()
	if !newspaperAudioWritable(cfg) {
		return "", newspaper.SafeDelivery(newspaper.ErrDisabled)
	}
	initial := cfg
	name := file.Name(e)
	if channel == "email" {
		if !cfg.Newspaper.AllowEmail || !p.EmailVerified {
			return "", newspaper.SafeDelivery(newspaper.ErrDisabled)
		}
		subject := e.Title + " · " + e.LocalDate
		plain, rich := subject, "<p>"+html.EscapeString(subject)+"</p>"
		if both {
			plain, rich = newspaper.Text(e), newspaper.HTML(e)
		}
		return s.newspaperSendEmail(ctx, p.EmailAccountID, p.EmailTo, subject, plain, rich, newspaperMailAudio{name, file.MIME, file.Data})
	}
	if both {
		if _, err := s.newspaperDelivery(ctx, e, p, channel); err != nil {
			return "", err
		}
	}
	// The text part has a known outcome. A pre-send audio refusal leaves only audio outstanding.
	preSend := func(err error) (string, error) {
		if both {
			return "", newspaper.ErrAudioPending
		}
		return "", newspaper.SafeDelivery(err)
	}
	cfg = s.ConfigSnapshot()
	if !newspaperAudioWritable(cfg) || ctx.Err() != nil {
		return preSend(newspaper.ErrDisabled)
	}
	switch channel {
	case "whatsapp":
		if !s.newspaperWhatsAppReady(cfg) || cfg.WhatsApp.Mode != initial.WhatsApp.Mode || !slices.Equal(cfg.WhatsApp.OwnerNumbers, initial.WhatsApp.OwnerNumbers) {
			return preSend(newspaper.ErrDisabled)
		}
		api := serverWhatsAppOwner(s)
		if api == nil {
			return preSend(newspaper.ErrDisabled)
		}
		_, err := api.SendOwnerAttachment(ctx, "", whatsapp.OwnerAttachment{Name: name, Data: file.Data})
		if errors.Is(err, whatsapp.ErrReadOnly) || errors.Is(err, whatsapp.ErrMediaTooLarge) {
			return preSend(err)
		}
		return "", newspaperWhatsAppError(err, false)
	case "telegram":
		if !cfg.Newspaper.AllowTelegram || cfg.Telegram.BotToken == "" || cfg.Telegram.UserID == 0 || cfg.Telegram.UserID != initial.Telegram.UserID || cfg.Telegram.BotToken != initial.Telegram.BotToken {
			return preSend(newspaper.ErrDisabled)
		}
		bot, err := newspaperTelegramBot(cfg.Telegram.BotToken)
		if err != nil {
			return preSend(err)
		}
		if ctx.Err() != nil {
			return preSend(ctx.Err())
		}
		bytes := tgbotapi.FileBytes{Name: name, Bytes: file.Data}
		var send tgbotapi.Chattable = tgbotapi.NewDocument(cfg.Telegram.UserID, bytes)
		if file.MIME == "audio/mpeg" {
			audio := tgbotapi.NewAudio(cfg.Telegram.UserID, bytes)
			audio.Title = e.Title + " · " + e.LocalDate
			send = audio
		}
		msg, err := bot.Send(send)
		if err != nil {
			return "", errors.New("Telegram audio outcome unknown")
		}
		return fmt.Sprint(msg.MessageID), nil
	}
	return preSend(newspaper.ErrDisabled)
}

func newspaperTelegramBot(token string) (*tgbotapi.BotAPI, error) {
	security.RegisterSensitive(token)
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return tgbotapi.NewBotAPIWithClient(token, newspaperTelegramEndpoint, client)
}

func newspaperMixedMail(body *bytes.Buffer, alternativeType string, attachment newspaperMailAudio) (string, error) {
	if len(attachment.Data) > 20<<20 {
		return "", errors.New("audio exceeds email attachment limit")
	}
	var outer bytes.Buffer
	mw := multipart.NewWriter(&outer)
	part, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {alternativeType}})
	if err != nil {
		return "", err
	}
	if _, err = body.WriteTo(part); err != nil {
		return "", err
	}
	part, err = mw.CreatePart(textproto.MIMEHeader{"Content-Type": {attachment.MIME}, "Content-Disposition": {fmt.Sprintf("attachment; filename=%q", attachment.Name)}, "Content-Transfer-Encoding": {"base64"}})
	if err != nil {
		return "", err
	}
	// 57 input bytes encode to one 76-column MIME line.
	for i := 0; i < len(attachment.Data); i += 57 {
		line := base64.StdEncoding.EncodeToString(attachment.Data[i:min(i+57, len(attachment.Data))])
		if _, err = fmt.Fprintf(part, "%s\r\n", line); err != nil {
			return "", err
		}
	}
	if err = mw.Close(); err != nil {
		return "", err
	}
	*body = outer
	return fmt.Sprintf("multipart/mixed; boundary=%q", mw.Boundary()), nil
}
