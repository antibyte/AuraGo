package tools

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
)

const (
	emailMaxAttachments     = 10
	emailMaxAttachmentBytes = 20 << 20 // total size of all attachments
)

// EmailAttachment is one file attached to an outgoing email.
type EmailAttachment struct {
	Name        string
	ContentType string
	Data        []byte
}

// LoadEmailAttachments resolves and reads attachment files (workspace and documents folder
// only, at most 10 files and 20 MB together).
func LoadEmailAttachments(cfg *config.Config, paths []string) ([]EmailAttachment, error) {
	if len(paths) > emailMaxAttachments {
		return nil, fmt.Errorf("at most %d attachments are allowed", emailMaxAttachments)
	}
	var total int64
	out := make([]EmailAttachment, 0, len(paths))
	for _, p := range paths {
		resolved, err := ResolveOutgoingAttachmentPath(p, cfg)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("read attachment %q: %w", p, err)
		}
		total += info.Size()
		if total > emailMaxAttachmentBytes {
			return nil, fmt.Errorf("attachments are larger than 20 MB together")
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("read attachment %q: %w", p, err)
		}
		name := filepath.Base(resolved)
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		out = append(out, EmailAttachment{Name: name, ContentType: contentType, Data: data})
	}
	return out, nil
}

// SendEmailWithAttachments sends a plain-text email with attachments via STARTTLS.
func SendEmailWithAttachments(smtpHost string, smtpPort int, username, password, from, to, subject, body string, attachments []EmailAttachment, logger *slog.Logger) error {
	return sendEmailWithAttachments(smtpHost, smtpPort, username, password, from, to, subject, body, attachments, false, logger)
}

// SendEmailTLSWithAttachments sends a plain-text email with attachments via implicit TLS (port 465).
func SendEmailTLSWithAttachments(smtpHost string, smtpPort int, username, password, from, to, subject, body string, attachments []EmailAttachment, logger *slog.Logger) error {
	return sendEmailWithAttachments(smtpHost, smtpPort, username, password, from, to, subject, body, attachments, true, logger)
}

func sendEmailWithAttachments(smtpHost string, smtpPort int, username, password, from, to, subject, body string, attachments []EmailAttachment, implicitTLS bool, logger *slog.Logger) error {
	if from == "" {
		from = username
	}
	msg := buildEmailMessage(from, to, subject, body, time.Now(), attachments, newEmailBoundary())
	if err := deliverSMTP(smtpHost, smtpPort, username, password, from, to, msg, implicitTLS); err != nil {
		return err
	}
	if logger != nil {
		logger.Info("[Email] Message with attachments sent", "from", from, "to", to, "subject", subject, "attachments", len(attachments))
	}
	return nil
}

func newEmailBoundary() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return "aurago-" + hex.EncodeToString(buf)
}

// writeBase64Lines writes data as base64 in lines of at most 76 characters (RFC 2045).
func writeBase64Lines(b *strings.Builder, data []byte) {
	encoded := base64.StdEncoding.EncodeToString(data)
	for len(encoded) > 76 {
		b.WriteString(encoded[:76])
		b.WriteString("\r\n")
		encoded = encoded[76:]
	}
	if encoded != "" {
		b.WriteString(encoded)
		b.WriteString("\r\n")
	}
}
