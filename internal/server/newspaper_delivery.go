package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"aurago/internal/agentmail"
	"aurago/internal/config"
	"aurago/internal/newspaper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (s *Server) newspaperDelivery(ctx context.Context, e newspaper.Edition, p newspaper.Profile, channel string) (string, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Newspaper.Enabled || cfg.Newspaper.ReadOnly {
		return "", newspaper.SafeDelivery(newspaper.ErrDisabled)
	}
	switch channel {
	case "email":
		if !cfg.Newspaper.AllowEmail || !p.EmailVerified {
			return "", newspaper.SafeDelivery(newspaper.ErrDisabled)
		}
		return s.newspaperSendEmail(ctx, p.EmailAccountID, p.EmailTo, e.Title+" · "+e.LocalDate, newspaper.Text(e), newspaper.HTML(e))
	case "telegram":
		if !cfg.Newspaper.AllowTelegram || cfg.Telegram.BotToken == "" || cfg.Telegram.UserID == 0 {
			return "", newspaper.SafeDelivery(newspaper.ErrDisabled)
		}
		return sendNewspaperTelegram(ctx, cfg.Telegram.BotToken, cfg.Telegram.UserID, e)
	default:
		return "", newspaper.SafeDelivery(errors.New("unsupported newspaper delivery channel"))
	}
}

func (s *Server) newspaperSendEmail(ctx context.Context, accountID, to, subject, plain, rich string) (string, error) {
	if accountID == "agentmail" {
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.AgentMail.Enabled || cfg.AgentMail.ReadOnly || cfg.AgentMail.APIKey == "" || cfg.AgentMail.InboxID == "" {
			return "", newspaper.SafeDelivery(errors.New("AgentMail sending is unavailable"))
		}
		client, err := agentmail.NewClient(agentmail.ClientConfig{BaseURL: cfg.AgentMail.BaseURL, APIKey: cfg.AgentMail.APIKey, DisableRetries: true})
		if err != nil {
			return "", newspaper.SafeDelivery(err)
		}
		message, err := client.SendMessage(ctx, cfg.AgentMail.InboxID, agentmail.SendMessageRequest{To: []string{to}, Subject: subject, Text: plain, HTML: rich})
		if err != nil {
			return "", fmt.Errorf("AgentMail send failed: %w", err)
		}
		return message.ID, nil
	}
	account, err := s.newspaperEmailAccount(accountID)
	if err != nil {
		return "", newspaper.SafeDelivery(err)
	}
	return "", sendNewspaperSMTP(ctx, account, to, subject, plain, rich)
}

func (s *Server) newspaperEmailAccount(id string) (config.EmailAccount, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return config.EmailAccount{}, errors.New("configuration unavailable")
	}
	for _, a := range cfg.EmailAccounts {
		if a.ID != id {
			continue
		}
		if a.Disabled || a.ReadOnly || a.SMTPHost == "" || a.SMTPPort <= 0 {
			return config.EmailAccount{}, errors.New("email account cannot send")
		}
		if a.Password == "" && s.Vault != nil {
			secret, err := s.Vault.ReadSecret("email_" + a.ID + "_password")
			if err == nil {
				a.Password = secret
			}
		}
		if a.Password == "" || a.Username == "" {
			return config.EmailAccount{}, errors.New("email account credentials unavailable")
		}
		return a, nil
	}
	return config.EmailAccount{}, errors.New("selected email account is unavailable")
}

func sendNewspaperSMTP(ctx context.Context, a config.EmailAccount, to, subject, plain, rich string) (outErr error) {
	potentiallySent := false
	defer func() {
		if outErr != nil && !potentiallySent {
			outErr = newspaper.SafeDelivery(outErr)
		}
	}()
	from := a.FromAddress
	if from == "" {
		from = a.Username
	}
	fromAddress, err := mail.ParseAddress(from)
	if err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}
	toAddress, err := mail.ParseAddress(to)
	if err != nil || toAddress.Address != to {
		return errors.New("invalid recipient address")
	}
	if len(plain)+len(rich) > 2<<20 {
		return errors.New("edition exceeds email size limit")
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, part := range []struct{ typ, text string }{{"text/plain", plain}, {"text/html", rich}} {
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", part.typ+"; charset=UTF-8")
		header.Set("Content-Transfer-Encoding", "base64")
		w, e := mw.CreatePart(header)
		if e != nil {
			return e
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(part.text))
		for len(encoded) > 76 {
			if _, e = io.WriteString(w, encoded[:76]+"\r\n"); e != nil {
				return e
			}
			encoded = encoded[76:]
		}
		if _, e = io.WriteString(w, encoded+"\r\n"); e != nil {
			return e
		}
	}
	if err = mw.Close(); err != nil {
		return err
	}
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", fromAddress.String(), toAddress.String(), mime.QEncoding.Encode("utf-8", subject), time.Now().Format(time.RFC1123Z), mw.Boundary())
	msg.Write(body.Bytes())
	address := net.JoinHostPort(a.SMTPHost, fmt.Sprint(a.SMTPPort))
	deadline := time.Now().Add(2 * time.Minute)
	var conn net.Conn
	if a.SMTPPort == 465 {
		conn, err = (&tls.Dialer{NetDialer: &net.Dialer{Timeout: 15 * time.Second}, Config: &tls.Config{ServerName: a.SMTPHost}}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("SMTP connection failed: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	client, err := smtp.NewClient(conn, a.SMTPHost)
	if err != nil {
		return fmt.Errorf("SMTP client failed: %w", err)
	}
	defer client.Close()
	if a.SMTPPort != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("SMTP requires STARTTLS")
		}
		if err = client.StartTLS(&tls.Config{ServerName: a.SMTPHost}); err != nil {
			return fmt.Errorf("SMTP STARTTLS failed: %w", err)
		}
	}
	if err = client.Auth(smtp.PlainAuth("", a.Username, a.Password, a.SMTPHost)); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}
	if err = client.Mail(fromAddress.Address); err != nil {
		return fmt.Errorf("SMTP MAIL failed: %w", err)
	}
	if err = client.Rcpt(toAddress.Address); err != nil {
		return fmt.Errorf("SMTP RCPT failed: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	potentiallySent = true
	if _, err = w.Write(msg.Bytes()); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("SMTP completion unknown: %w", err)
	}
	// A successful DATA completion means the server accepted the message.
	// A later QUIT failure cannot safely be interpreted as a failed send.
	_ = client.Quit()
	return nil
}

func sendNewspaperTelegram(ctx context.Context, token string, chatID int64, e newspaper.Edition) (string, error) {
	if token == "" || chatID == 0 {
		return "", newspaper.SafeDelivery(errors.New("Telegram destination unavailable"))
	}
	data, pdfErr := newspaper.PDF(e)
	var chunks []string
	if pdfErr == nil && len(data) > 20<<20 {
		return "", newspaper.SafeDelivery(errors.New("Telegram PDF exceeds delivery limit"))
	}
	if pdfErr != nil {
		var err error
		chunks, err = newspaperTelegramChunks(newspaper.Text(e), 3000)
		if err != nil {
			return "", newspaper.SafeDelivery(err)
		}
	}
	client := &http.Client{Timeout: 90 * time.Second}
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
	if err != nil {
		return "", newspaper.SafeDelivery(fmt.Errorf("Telegram client failed: %w", err))
	}
	lead := e.Stories[0]
	intro := fmt.Sprintf("%s · %s\n%s\n\n%s\n\n%d stories · %d sources", e.Title, e.LocalDate, lead.Headline, lead.Deck, len(e.Stories), len(e.Sources))
	if len([]rune(intro)) > 3000 {
		return "", newspaper.SafeDelivery(errors.New("Telegram digest exceeds message limit"))
	}
	digestMessage, err := bot.Send(tgbotapi.NewMessage(chatID, intro))
	if err != nil {
		return "", fmt.Errorf("Telegram digest failed: %w", err)
	}
	lastID := fmt.Sprint(digestMessage.MessageID)
	if pdfErr == nil {
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{Name: "newspaper-" + e.LocalDate + "-r" + fmt.Sprint(e.Revision) + ".pdf", Bytes: data})
		message, sendErr := bot.Send(doc)
		err = sendErr
		if err != nil {
			return lastID, fmt.Errorf("Telegram document outcome unknown: %w", err)
		}
		return fmt.Sprint(message.MessageID), nil
	}
	// Fonts without safe glyph coverage use full bounded text messages.
	for _, chunk := range chunks {
		if ctx.Err() != nil {
			return lastID, ctx.Err()
		}
		message, sendErr := bot.Send(tgbotapi.NewMessage(chatID, chunk))
		if sendErr != nil {
			return lastID, fmt.Errorf("Telegram text outcome unknown: %w", sendErr)
		}
		lastID = fmt.Sprint(message.MessageID)
	}
	return lastID, nil
}

func newspaperTelegramChunks(text string, maxRunes int) ([]string, error) {
	if maxRunes < 32 {
		return nil, errors.New("Telegram message limit is too small")
	}
	lines := strings.SplitAfter(text, "\n")
	out := []string{}
	var b strings.Builder
	for _, line := range lines {
		if len([]rune(line)) > maxRunes {
			// The renderer bounds URLs and paragraphs below this limit. Reject any
			// unexpected overlong line instead of silently omitting edition text.
			return nil, errors.New("edition contains a line too long for Telegram text delivery")
		}
		if len([]rune(b.String()))+len([]rune(line)) > maxRunes && b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
		b.WriteString(line)
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out, nil
}
