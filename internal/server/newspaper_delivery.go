package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
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
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/agentmail"
	"aurago/internal/config"
	"aurago/internal/newspaper"
	"aurago/internal/whatsapp"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var errNewspaperAgentMailBounce = errors.New("AgentMail blocked the recipient after a bounce")

// newspaperTelegramEndpoint is the Bot API URL format of the Telegram delivery; tests
// point it to a local server.
var newspaperTelegramEndpoint = tgbotapi.APIEndpoint

// newspaperTelegramText builds one text message of the Telegram delivery with link
// previews off. The edition text carries source URLs and headlines that third-party pages
// wrote, and Telegram fetches the URL of a preview itself, so a URL that a hostile page
// got into the edition would be requested with no tool call and no approval. Every text
// message of the delivery has to be built here; the link itself stays clickable.
func newspaperTelegramText(chatID int64, text string) tgbotapi.MessageConfig {
	message := tgbotapi.NewMessage(chatID, text)
	message.DisableWebPagePreview = true
	return message
}

func newspaperAgentMailBounceRejected(apiErr *agentmail.APIError) bool {
	if apiErr == nil || apiErr.StatusCode != http.StatusForbidden {
		return false
	}
	var response struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal([]byte(apiErr.Body), &response)
	if response.Code != "" && response.Code != "message_rejected" {
		return false
	}
	message := strings.ToLower(apiErr.Message)
	return strings.Contains(message, "blocked") && strings.Contains(message, "bounced")
}

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
	case "whatsapp":
		if !s.newspaperWhatsAppReady(cfg) {
			return "", newspaper.SafeDelivery(newspaper.ErrDisabled)
		}
		return s.sendNewspaperWhatsApp(ctx, e)
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
			var apiErr *agentmail.APIError
			if errors.As(err, &apiErr) {
				if newspaperAgentMailBounceRejected(apiErr) {
					return "", newspaper.SafeDelivery(errNewspaperAgentMailBounce)
				}
				switch apiErr.StatusCode {
				case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusTooManyRequests:
					return "", newspaper.SafeDelivery(fmt.Errorf("AgentMail rejected send: %w", err))
				}
			}
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

// newspaperDigestRunes bounds the digest and each text chunk the chat deliveries send (one
// message each).
const newspaperDigestRunes = 3000

// newspaperDigest is the digest text of an edition: the introduction of the Telegram delivery and
// the caption of the WhatsApp one.
func newspaperDigest(e newspaper.Edition) string {
	leadHeadline := ""
	leadDeck := ""
	if len(e.Stories) > 0 {
		leadHeadline = e.Stories[0].Headline
		leadDeck = e.Stories[0].Deck
	}
	return fmt.Sprintf("%s · %s\n%s\n\n%s\n\n%d stories · %d sources", e.Title, e.LocalDate, leadHeadline, leadDeck, len(e.Stories), len(e.Sources))
}

// newspaperPDFName is the file name of an edition's PDF in the chat deliveries.
func newspaperPDFName(e newspaper.Edition) string {
	return "newspaper-" + e.LocalDate + "-r" + fmt.Sprint(e.Revision) + ".pdf"
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
		chunks, err = newspaperTelegramChunks(newspaper.Text(e), newspaperDigestRunes)
		if err != nil {
			return "", newspaper.SafeDelivery(err)
		}
	}
	client := &http.Client{Timeout: 90 * time.Second}
	bot, err := tgbotapi.NewBotAPIWithClient(token, newspaperTelegramEndpoint, client)
	if err != nil {
		return "", newspaper.SafeDelivery(fmt.Errorf("Telegram client failed: %w", err))
	}
	intro := newspaperDigest(e)
	if utf8.RuneCountInString(intro) > newspaperDigestRunes {
		return "", newspaper.SafeDelivery(errors.New("Telegram digest exceeds message limit"))
	}
	digestMessage, err := bot.Send(newspaperTelegramText(chatID, intro))
	if err != nil {
		return "", fmt.Errorf("Telegram digest failed: %w", err)
	}
	lastID := fmt.Sprint(digestMessage.MessageID)
	if pdfErr == nil {
		doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{Name: newspaperPDFName(e), Bytes: data})
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
		message, sendErr := bot.Send(newspaperTelegramText(chatID, chunk))
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

// newspaperWhatsAppReady reports that an edition can go to WhatsApp now: the newspaper's allow
// switch is on, WhatsApp is enabled and writable, and a linked generation runs.
func (s *Server) newspaperWhatsAppReady(cfg *config.Config) bool {
	return cfg != nil && cfg.Newspaper.AllowWhatsApp && s.whatsAppOwnerReady(cfg)
}

// sendNewspaperWhatsApp delivers an edition to the WhatsApp owners through the owner API: the PDF
// as a document with the digest as its caption (a digest over the caption limit goes first as its
// own message), or, when the edition has no safe PDF, the digest and then the full text in
// messages of at most newspaperDigestRunes runes. The owner API sends plain text without a link
// preview, so no URL a third-party page put into the edition is fetched. The provider ID stays
// empty: the owner API returns no message IDs.
func (s *Server) sendNewspaperWhatsApp(ctx context.Context, e newspaper.Edition) (string, error) {
	api := serverWhatsAppOwner(s)
	if api == nil || !api.OwnerSendReady() {
		return "", newspaper.SafeDelivery(errors.New("WhatsApp is not linked or not running"))
	}
	digest := newspaperDigest(e)
	if utf8.RuneCountInString(digest) > newspaperDigestRunes {
		return "", newspaper.SafeDelivery(errors.New("WhatsApp digest exceeds message limit"))
	}
	if data, pdfErr := newspaper.PDF(e); pdfErr == nil {
		if err := ctx.Err(); err != nil {
			return "", newspaper.SafeDelivery(err)
		}
		_, err := api.SendOwnerAttachment(ctx, digest, whatsapp.OwnerAttachment{Name: newspaperPDFName(e), Data: data})
		return "", newspaperWhatsAppError(err, false)
	}
	// Fonts without safe glyph coverage use full bounded text messages.
	chunks, err := newspaperTelegramChunks(newspaper.Text(e), newspaperDigestRunes)
	if err != nil {
		return "", newspaper.SafeDelivery(err)
	}
	sent := false
	for _, text := range append([]string{digest}, chunks...) {
		if err := ctx.Err(); err != nil {
			// This check runs before the call, so only before the first message is it known
			// that nothing of this delivery went out.
			if !sent {
				return "", newspaper.SafeDelivery(err)
			}
			return "", newspaperWhatsAppError(err, true)
		}
		if _, err := api.SendOwnerMessage(ctx, text, ""); err != nil {
			return "", newspaperWhatsAppError(err, sent)
		}
		sent = true
	}
	return "", nil
}

// newspaperWhatsAppError classifies a failure of the owner API for the ledger. Before anything of
// this delivery went out, only its pre-send refusals (read-only, a file over 50 MB) are safe to
// retry. Every other error leaves the outcome unknown, a cancellation reported by the API
// included: with several owners or a send that timed out, or when the Bot's generation ended
// mid-send, a message may have arrived, and the ledger never replays an uncertain send.
func newspaperWhatsAppError(err error, sentBefore bool) error {
	switch {
	case err == nil:
		return nil
	case !sentBefore && (errors.Is(err, whatsapp.ErrReadOnly) || errors.Is(err, whatsapp.ErrMediaTooLarge)):
		return newspaper.SafeDelivery(err)
	}
	return fmt.Errorf("WhatsApp delivery outcome unknown: %w", err)
}

// newspaperWhatsAppDestination keys the ledger to the owners without a full phone number: the
// ledger keeps SHA-256(channel, destination), and a hash of a whole number can be reversed by
// trying every number. Dedicated mode: the sorted, de-duplicated last four digits of each valid
// owner number; self-chat mode: the linked account itself. The send does not use this value; it
// goes to the Bot's own owner resolver.
func newspaperWhatsAppDestination(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", errors.New("WhatsApp destination unavailable")
	}
	if cfg.WhatsApp.Mode == config.WhatsAppModeSelfChat {
		return "self_chat", nil
	}
	var tails []string
	for _, raw := range cfg.WhatsApp.OwnerNumbers {
		number, err := config.NormalizeE164(raw)
		if err != nil {
			continue
		}
		tails = append(tails, number[len(number)-4:])
	}
	if len(tails) == 0 {
		return "", errors.New("WhatsApp destination unavailable")
	}
	slices.Sort(tails)
	return "dedicated\x00" + strings.Join(slices.Compact(tails), ","), nil
}

// newspaperWhatsAppTestText is the fixed text of the Newspaper's WhatsApp test send.
const newspaperWhatsAppTestText = "AuraGo Newspaper: WhatsApp delivery test."

var errNewspaperWhatsAppTestRateLimited = errors.New("wait a minute before sending another WhatsApp test")

// sendNewspaperWhatsAppTest checks the gates before it claims the minute, so a refused request
// does not use it up; the send passes the owner API's own read-only gate.
func (s *Server) sendNewspaperWhatsAppTest(ctx context.Context, cfg *config.Config, now time.Time) error {
	if cfg == nil || !cfg.Newspaper.Enabled || cfg.Newspaper.ReadOnly || cfg.VirtualDesktop.ReadOnly || !s.newspaperWhatsAppReady(cfg) {
		return newspaper.ErrDisabled
	}
	if !takeWhatsAppTestMinute(now) {
		return errNewspaperWhatsAppTestRateLimited
	}
	api := serverWhatsAppOwner(s)
	if api == nil {
		return newspaper.ErrDisabled
	}
	_, err := api.SendOwnerMessage(ctx, newspaperWhatsAppTestText, "")
	return err
}
