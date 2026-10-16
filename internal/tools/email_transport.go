package tools

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"aurago/internal/mailer"
	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
)

const maxLegacyEmailBodyBytes = 16 << 20

// FetchEmailsWithTransport preserves the legacy result shape while using the
// configured shared IMAP transport.
func FetchEmailsWithTransport(ctx context.Context, host string, port int, username, password, receiveTLS, folder string, count int, logger *slog.Logger) ([]EmailMessage, error) {
	if count <= 0 {
		count = 10
	}
	if count > 50 {
		count = 50
	}
	var messages []EmailMessage
	err := mailer.WithIMAPClient(ctx, legacyIMAPConfig(host, port, username, password, receiveTLS), func(c *imapclient.Client) error {
		status, err := c.Select(folder, true)
		if err != nil || status.Messages == 0 {
			return err
		}
		first := uint32(1)
		if status.Messages > uint32(count) {
			first = status.Messages - uint32(count) + 1
		}
		set := new(imap.SeqSet)
		set.AddRange(first, status.Messages)
		messages, err = fetchLegacyMessages(c, set, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Debug("Fetched email messages", "count", len(messages))
	}
	return messages, nil
}

func fetchEmailsByUIDTransportContext(parent context.Context, host string, port int, username, password, receiveTLS, folder string, uids []uint32, logger *slog.Logger) ([]EmailMessage, error) {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	if len(uids) == 0 {
		return nil, nil
	}
	if len(uids) > 50 {
		uids = uids[:50]
	}
	var messages []EmailMessage
	err := mailer.WithIMAPClient(ctx, legacyIMAPConfig(host, port, username, password, receiveTLS), func(c *imapclient.Client) error {
		if _, err := c.Select(folder, true); err != nil {
			return err
		}
		set := new(imap.SeqSet)
		set.AddNum(uids...)
		var err error
		messages, err = fetchLegacyMessages(c, set, true)
		return err
	})
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Debug("Fetched email messages by UID", "count", len(messages))
	}
	return messages, nil
}

func searchUnseenUIDsTransportContext(parent context.Context, host string, port int, username, password, receiveTLS, folder string) ([]uint32, error) {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	var uids []uint32
	err := mailer.WithIMAPClient(ctx, legacyIMAPConfig(host, port, username, password, receiveTLS), func(c *imapclient.Client) error {
		if _, err := c.Select(folder, true); err != nil {
			return err
		}
		criteria := imap.NewSearchCriteria()
		criteria.WithoutFlags = append(criteria.WithoutFlags, imap.SeenFlag)
		var err error
		uids, err = c.UidSearch(criteria)
		return err
	})
	return uids, err
}

func fetchLegacyMessages(c *imapclient.Client, set *imap.SeqSet, byUID bool) ([]EmailMessage, error) {
	headers := &imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: imap.HeaderSpecifier, Fields: []string{"From", "To", "Subject", "Date"}}}
	body := &imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: "TEXT"}}
	items := []imap.FetchItem{imap.FetchUid, headers.FetchItem(), body.FetchItem()}
	fetched := make(chan *imap.Message, 50)
	var err error
	if byUID {
		err = c.UidFetch(set, items, fetched)
	} else {
		err = c.Fetch(set, items, fetched)
	}
	if err != nil {
		return nil, err
	}
	messages := make([]EmailMessage, 0, len(fetched))
	for remote := range fetched {
		message := EmailMessage{UID: remote.Uid}
		if raw, err := readLegacyIMAPSection(remote.GetBody(headers)); err != nil {
			return nil, err
		} else {
			parseHeaders(&message, string(raw))
		}
		if raw, err := readLegacyIMAPSection(remote.GetBody(body)); err != nil {
			return nil, err
		} else {
			message.Body = decodeBodyText(string(raw))
			if len(message.Body) > 4096 {
				message.Body = truncateUTF8Safe(message.Body, 4096) + "\n[... truncated]"
			}
			message.Snippet = truncateSnippet(message.Body, 200)
		}
		messages = append(messages, message)
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].UID > messages[j].UID })
	return messages, nil
}

func readLegacyIMAPSection(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxLegacyEmailBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read IMAP message section: %w", err)
	}
	if len(data) > maxLegacyEmailBodyBytes {
		return nil, fmt.Errorf("IMAP message section exceeds size limit")
	}
	return data, nil
}

func legacyIMAPConfig(host string, port int, username, password, receiveTLS string) mailer.TransportConfig {
	return mailer.TransportConfig{ReceiveProtocol: "imap", ReceiveTLS: strings.TrimSpace(receiveTLS),
		IMAPHost: host, IMAPPort: port, Username: username, Password: password}
}

// TestIMAPConnectionWithTLS verifies credentials with the selected IMAP mode.
func TestIMAPConnectionWithTLS(ctx context.Context, host string, port int, username, password, receiveTLS string) error {
	return mailer.TestIMAPAuth(ctx, legacyIMAPConfig(host, port, username, password, receiveTLS))
}

// TestSMTPAuthWithTransport verifies SMTP credentials without sending mail.
func TestSMTPAuthWithTransport(ctx context.Context, host string, port int, username, password, transport string) error {
	return mailer.TestSMTPAuth(ctx, mailer.TransportConfig{SMTPHost: host, SMTPPort: port,
		SMTPUsername: username, SMTPPassword: password, SMTPTransport: strings.TrimSpace(transport)})
}

// SendEmailConfigured sends through the same SMTP transport used by Mailer.
func SendEmailConfigured(ctx context.Context, cfg mailer.TransportConfig, to, subject, body string, attachments []EmailAttachment, logger *slog.Logger) error {
	from := configuredEmailFrom(cfg)
	if from == "" {
		return fmt.Errorf("SMTP sender address is required")
	}
	recipients, err := checkEmailEnvelope(from, to)
	if err != nil {
		return err
	}
	cfg.FromAddress = from
	if err := checkEmailAttachmentLimits(attachments); err != nil {
		return err
	}
	draft := mailer.Draft{To: recipients, Subject: subject, Text: body}
	for _, attachment := range attachments {
		draft.Attachments = append(draft.Attachments, mailer.Attachment{Filename: attachment.Name, ContentType: attachment.ContentType, Data: attachment.Data})
	}
	result, err := mailer.SendSMTP(ctx, cfg, mailer.Outgoing{Draft: draft})
	if err != nil {
		return err
	}
	if !result.Confirmed {
		return fmt.Errorf("SMTP delivery was not confirmed")
	}
	if logger != nil {
		logger.Info("[Email] Message sent", "recipient_count", len(recipients), "attachment_count", len(attachments))
	}
	return nil
}

func configuredEmailFrom(cfg mailer.TransportConfig) string {
	for _, candidate := range []string{cfg.FromAddress, cfg.Address, cfg.SMTPUsername, cfg.Username} {
		if strings.TrimSpace(candidate) != "" {
			return strings.TrimSpace(candidate)
		}
	}
	return ""
}

// SendEmailWithTransport keeps agent email writes on Mailer's configured SMTP
// transport while preserving the legacy plain-text message fields.
func SendEmailWithTransport(ctx context.Context, host string, port int, username, password, from, to, subject, body, transport string, attachments []EmailAttachment, logger *slog.Logger) error {
	return SendEmailConfigured(ctx, mailer.TransportConfig{SMTPHost: host, SMTPPort: port, SMTPUsername: username,
		SMTPPassword: password, Username: username, Password: password, FromAddress: from, Address: from, SMTPTransport: transport},
		to, subject, body, attachments, logger)
}
