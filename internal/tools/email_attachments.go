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
	"regexp"
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

// Limits that keep an attachment's Content-Type one valid header line.
const (
	// emailMaxMediaTypeNameLen is the RFC 6838 limit for a type and for a subtype name.
	emailMaxMediaTypeNameLen = 127
	// emailMaxCharsetLen is the length limit of IANA charset names.
	emailMaxCharsetLen = 40
	// emailMaxHeaderLineLen is the RFC 5322 limit for a line, without its CRLF.
	emailMaxHeaderLineLen = 998
)

// emailCharsetPattern matches the charset names an attachment keeps (utf-8, iso-8859-1,
// windows-1252, shift_jis, …); they need no quoting.
var emailCharsetPattern = regexp.MustCompile(`^[A-Za-z0-9._:+-]+$`)

// emailAttachmentHeaders returns the values of the Content-Type and Content-Disposition
// headers of an attachment part. Both are always one valid line that names the attachment:
//   - The name is emailAttachmentName(a.Name). mime.FormatMediaType quotes it, or encodes it
//     per RFC 2231 when it holds non-ASCII characters.
//   - Of the caller's content type only the type and a plain charset of at most 40
//     characters are kept; mime.TypeByExtension gives text types such a charset, a form
//     mime.FormatMediaType cannot take as type. Other parameters are dropped.
//   - A type that does not parse, has no subtype, has a type or subtype name over the 127
//     characters RFC 6838 allows, or is composite (multipart/*, message/*, which must not be
//     base64-encoded) is sent as application/octet-stream. So is any type whose header line
//     would still pass 998 characters.
//   - Should mime.FormatMediaType still give no value, both headers use
//     emailAttachmentFallbackName.
func emailAttachmentHeaders(a EmailAttachment) (contentType, disposition string) {
	name := emailAttachmentName(a.Name)
	params := map[string]string{"name": name}
	mediaType, original, err := mime.ParseMediaType(a.ContentType)
	major, sub, hasSub := strings.Cut(mediaType, "/")
	if err != nil || !hasSub || len(major) > emailMaxMediaTypeNameLen || len(sub) > emailMaxMediaTypeNameLen ||
		major == "multipart" || major == "message" {
		mediaType = "application/octet-stream"
	} else if charset := original["charset"]; len(charset) <= emailMaxCharsetLen && emailCharsetPattern.MatchString(charset) {
		params["charset"] = charset
	}
	contentType = mime.FormatMediaType(mediaType, params)
	if len("Content-Type: ")+len(contentType) > emailMaxHeaderLineLen {
		contentType = mime.FormatMediaType("application/octet-stream", map[string]string{"name": name})
	}
	disposition = mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if contentType == "" || disposition == "" {
		fallback := emailAttachmentFallbackName(name)
		contentType = mime.FormatMediaType("application/octet-stream", map[string]string{"name": fallback})
		disposition = mime.FormatMediaType("attachment", map[string]string{"filename": fallback})
	}
	return contentType, disposition
}

// emailAttachmentName is the name an attachment is sent under: valid UTF-8, every control
// character (CR and LF included) and every bidi control (such as U+202E, which makes
// "rechnung\u202Efdp.exe" display as "rechnungexe.pdf") replaced by "_", and at most
// emailMaxAttachmentNameBytes bytes, cut before a short extension so the extension survives.
// An empty name, "." and ".." become "attachment".
func emailAttachmentName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return '_'
		}
		return r
	}, strings.ToValidUTF8(name, "\uFFFD"))
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
	if _, err := checkEmailEnvelope(from, to); err != nil {
		return err
	}
	if err := checkEmailAttachmentLimits(attachments); err != nil {
		return err
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

// checkEmailAttachmentLimits applies the limits of LoadEmailAttachments to attachments that
// were built some other way: at most emailMaxAttachments files and emailMaxAttachmentBytes
// together.
func checkEmailAttachmentLimits(attachments []EmailAttachment) error {
	if len(attachments) > emailMaxAttachments {
		return fmt.Errorf("at most %d attachments are allowed", emailMaxAttachments)
	}
	total := 0
	for _, a := range attachments {
		total += len(a.Data)
		if total > emailMaxAttachmentBytes {
			return errEmailAttachmentsTooLarge
		}
	}
	return nil
}

func newEmailBoundary() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return "aurago-" + hex.EncodeToString(buf)
}

// writeBase64Lines writes data as base64 in lines of at most 76 characters (RFC 2045). It
// encodes 57 input bytes per line through a stack buffer, so it allocates nothing beyond b;
// 57 is a multiple of 3, so the lines are the ones the whole encoding cut every 76
// characters would give.
func writeBase64Lines(b *strings.Builder, data []byte) {
	var line [78]byte
	for len(data) > 0 {
		n := min(57, len(data))
		enc := base64.StdEncoding.EncodedLen(n)
		base64.StdEncoding.Encode(line[:enc], data[:n])
		line[enc], line[enc+1] = '\r', '\n'
		b.Write(line[:enc+2])
		data = data[n:]
	}
}

// base64LinesLen is the number of bytes writeBase64Lines writes for n bytes of data.
func base64LinesLen(n int) int {
	return base64.StdEncoding.EncodedLen(n) + 2*((n+56)/57)
}
