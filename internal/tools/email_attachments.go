package tools

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"
	"time"
	"unicode"

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

// emailMaxAttachmentNameBytes bounds the name an attachment is sent under. It is the usual
// file system limit, and keeps the RFC 2231-encoded header line (three characters per byte)
// below the 998 characters RFC 5322 allows.
const emailMaxAttachmentNameBytes = 255

// errEmailAttachmentsTooLarge reports attachments over emailMaxAttachmentBytes together.
var errEmailAttachmentsTooLarge = errors.New("attachments are larger than 20 MB together")

// LoadEmailAttachments resolves and reads attachment files (workspace and documents folder
// only, at most 10 files and 20 MB together).
//
// Each file is opened once with OpenOutgoingAttachment and read through that handle, so it
// cannot be swapped for another file after the checks. The 20 MB limit holds for the bytes
// actually read, also when a file grows after its size was taken.
func LoadEmailAttachments(cfg *config.Config, paths []string) ([]EmailAttachment, error) {
	if len(paths) > emailMaxAttachments {
		return nil, fmt.Errorf("at most %d attachments are allowed", emailMaxAttachments)
	}
	remaining := int64(emailMaxAttachmentBytes)
	out := make([]EmailAttachment, 0, len(paths))
	for _, p := range paths {
		attachment, err := loadEmailAttachment(cfg, p, remaining)
		if err != nil {
			return nil, err
		}
		remaining -= int64(len(attachment.Data))
		out = append(out, attachment)
	}
	return out, nil
}

// loadEmailAttachment reads one attachment of at most remaining bytes and closes it again.
// Error messages repeat at most maxEchoedAttachmentPathRunes runes of p.
func loadEmailAttachment(cfg *config.Config, p string, remaining int64) (EmailAttachment, error) {
	f, resolved, err := OpenOutgoingAttachment(p, cfg)
	if err != nil {
		return EmailAttachment{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return EmailAttachment{}, fmt.Errorf("read attachment %q: %w", truncateStr(p, maxEchoedAttachmentPathRunes), withoutPathError(err))
	}
	data, err := readEmailAttachmentData(f, info.Size(), remaining)
	if errors.Is(err, errEmailAttachmentsTooLarge) {
		return EmailAttachment{}, err
	}
	if err != nil {
		return EmailAttachment{}, fmt.Errorf("read attachment %q: %w", truncateStr(p, maxEchoedAttachmentPathRunes), err)
	}
	name := filepath.Base(resolved)
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return EmailAttachment{Name: name, ContentType: contentType, Data: data}, nil
}

// readEmailAttachmentData reads r, a file whose Stat gave size, within a budget of remaining
// bytes. size only sizes the buffer and refuses a file that is too large before reading it;
// the limit itself is enforced on the bytes read, so a file that grows after its Stat fails
// with errEmailAttachmentsTooLarge as well. A file that shrinks is read as it now is. Read
// errors carry no path.
func readEmailAttachmentData(r io.Reader, size, remaining int64) ([]byte, error) {
	if size > remaining {
		return nil, errEmailAttachmentsTooLarge
	}
	buf := bytes.NewBuffer(make([]byte, 0, int(max(size, 0))+bytes.MinRead))
	if _, err := buf.ReadFrom(io.LimitReader(r, remaining+1)); err != nil {
		return nil, withoutPathError(err)
	}
	if int64(buf.Len()) > remaining {
		return nil, errEmailAttachmentsTooLarge
	}
	return buf.Bytes(), nil
}

// emailAttachmentHeaders returns the values of the Content-Type and Content-Disposition
// headers of an attachment part. Both are always one line that names the attachment:
//   - The name is emailAttachmentName(a.Name). mime.FormatMediaType quotes it, or encodes it
//     per RFC 2231 when it holds non-ASCII characters.
//   - The content type keeps its parameters: mime.TypeByExtension gives text types a
//     charset, a form mime.FormatMediaType cannot take as type.
//   - A type that does not parse, has no subtype, or is composite (multipart/*, message/*,
//     which must not be base64-encoded) is sent as application/octet-stream.
//   - Should mime.FormatMediaType still give no value, both headers use
//     emailAttachmentFallbackName.
func emailAttachmentHeaders(a EmailAttachment) (contentType, disposition string) {
	name := emailAttachmentName(a.Name)
	mediaType, params, err := mime.ParseMediaType(a.ContentType)
	if err != nil || !strings.Contains(mediaType, "/") ||
		strings.HasPrefix(mediaType, "multipart/") || strings.HasPrefix(mediaType, "message/") {
		mediaType, params = "application/octet-stream", map[string]string{}
	}
	params["name"] = name
	contentType = mime.FormatMediaType(mediaType, params)
	disposition = mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if contentType == "" || disposition == "" {
		fallback := emailAttachmentFallbackName(name)
		contentType = mime.FormatMediaType("application/octet-stream", map[string]string{"name": fallback})
		disposition = mime.FormatMediaType("attachment", map[string]string{"filename": fallback})
	}
	return contentType, disposition
}

// emailAttachmentName is the name an attachment is sent under: valid UTF-8, every control
// character (CR and LF included) replaced by "_", and at most emailMaxAttachmentNameBytes
// bytes, cut before a short extension so the extension survives. An empty name, "." and
// ".." become "attachment".
func emailAttachmentName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '_'
		}
		return r
	}, strings.ToValidUTF8(name, "�"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "attachment"
	}
	if len(name) <= emailMaxAttachmentNameBytes {
		return name
	}
	ext := ""
	if i := strings.LastIndexByte(name, '.'); i > 0 && len(name)-i <= 16 {
		ext = name[i:]
	}
	return truncateUTF8Safe(name[:len(name)-len(ext)], emailMaxAttachmentNameBytes-len(ext)) + ext
}

// emailAttachmentFallbackName is "attachment" plus the extension of name when that is short
// plain ASCII (letters and digits, lower-cased), else just "attachment".
func emailAttachmentFallbackName(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return "attachment"
	}
	ext := strings.ToLower(name[i+1:])
	if ext == "" || len(ext) > 10 || strings.ContainsFunc(ext, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		return "attachment"
	}
	return "attachment." + ext
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
		logger.Info("[Email] Message with attachments sent", "from", from, "to", truncateStr(to, smtpMaxEchoRunes),
			"subject", truncateStr(subject, smtpMaxEchoRunes), "attachments", len(attachments))
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
