package tools

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ── Data Types ──────────────────────────────────────────────────────────────

// EmailMessage represents a single email returned by IMAP fetch.
type EmailMessage struct {
	UID     uint32 `json:"uid"`
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Body    string `json:"body"`
	Snippet string `json:"snippet,omitempty"`
}

// EmailResult is the structured JSON returned to the LLM.
type EmailResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Count   int         `json:"count,omitempty"`
}

func EncodeEmailResult(r EmailResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// ── IMAP Client (Lightweight) ───────────────────────────────────────────────

// imapConn wraps a TLS connection to an IMAP server.
type imapConn struct {
	conn   net.Conn
	reader *bufio.Reader
	tag    int
}

func imapDial(host string, port int) (*imapConn, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	tlsConn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 15 * time.Second},
		"tcp", addr,
		&tls.Config{ServerName: host},
	)
	if err != nil {
		return nil, fmt.Errorf("IMAP TLS dial failed: %w", err)
	}

	ic := &imapConn{conn: tlsConn, reader: bufio.NewReaderSize(tlsConn, 65536)}

	// Read server greeting
	if _, err := ic.readLine(); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("IMAP greeting failed: %w", err)
	}
	return ic, nil
}

func (ic *imapConn) Close() {
	ic.command("LOGOUT")
	ic.conn.Close()
}

func (ic *imapConn) readLine() (string, error) {
	ic.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	line, err := ic.reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// readUntilTag reads all lines until we get the tagged response (e.g., "A001 OK ...").
func (ic *imapConn) readUntilTag(tag string) ([]string, string, error) {
	var lines []string
	for {
		line, err := ic.readLine()
		if err != nil {
			return lines, "", err
		}
		if strings.HasPrefix(line, tag+" ") {
			return lines, line, nil
		}
		lines = append(lines, line)
	}
}

func (ic *imapConn) command(format string, args ...interface{}) ([]string, string, error) {
	ic.tag++
	tag := fmt.Sprintf("A%03d", ic.tag)
	cmd := fmt.Sprintf("%s %s\r\n", tag, fmt.Sprintf(format, args...))
	ic.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	if _, err := io.WriteString(ic.conn, cmd); err != nil {
		return nil, "", fmt.Errorf("IMAP write failed: %w", err)
	}
	return ic.readUntilTag(tag)
}

func (ic *imapConn) login(user, pass string) error {
	_, status, err := ic.command("LOGIN %s %s", quoteIMAPString(user), quoteIMAPString(pass))
	if err != nil {
		return err
	}
	if !strings.Contains(status, "OK") {
		return fmt.Errorf("IMAP LOGIN failed: %s", status)
	}
	return nil
}

func (ic *imapConn) selectFolder(folder string) (int, error) {
	lines, status, err := ic.command("SELECT %s", quoteIMAPString(folder))
	if err != nil {
		return 0, err
	}
	if !strings.Contains(status, "OK") {
		return 0, fmt.Errorf("IMAP SELECT failed: %s", status)
	}
	exists := 0
	for _, line := range lines {
		if strings.Contains(line, "EXISTS") {
			fmt.Sscanf(line, "* %d EXISTS", &exists)
		}
	}
	return exists, nil
}

// FetchEmails connects to IMAP, fetches the N most recent emails from the given folder.
func FetchEmails(host string, port int, username, password, folder string, count int, logger *slog.Logger) ([]EmailMessage, error) {
	if count <= 0 {
		count = 10
	}
	if count > 50 {
		count = 50
	}

	ic, err := imapDial(host, port)
	if err != nil {
		return nil, err
	}
	defer ic.Close()

	if err := ic.login(username, password); err != nil {
		return nil, err
	}

	exists, err := ic.selectFolder(folder)
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, nil
	}

	// Calculate range: last N messages
	from := exists - count + 1
	if from < 1 {
		from = 1
	}
	seqRange := fmt.Sprintf("%d:%d", from, exists)

	// FETCH envelope and body
	lines, status, err := ic.command("FETCH %s (UID BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)] BODY[TEXT])", seqRange)
	if err != nil {
		return nil, fmt.Errorf("IMAP FETCH failed: %w", err)
	}
	if !strings.Contains(status, "OK") {
		return nil, fmt.Errorf("IMAP FETCH failed: %s", status)
	}

	messages := parseIMAPFetch(lines, logger)

	// Sort by UID descending (newest first)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].UID > messages[j].UID
	})

	return messages, nil
}

// SearchUnseenUIDs returns UIDs of unseen messages in the given folder.
func SearchUnseenUIDs(host string, port int, username, password, folder string) ([]uint32, error) {
	ic, err := imapDial(host, port)
	if err != nil {
		return nil, err
	}
	defer ic.Close()

	if err := ic.login(username, password); err != nil {
		return nil, err
	}
	if _, err := ic.selectFolder(folder); err != nil {
		return nil, err
	}

	lines, status, err := ic.command("UID SEARCH UNSEEN")
	if err != nil {
		return nil, err
	}
	if !strings.Contains(status, "OK") {
		return nil, fmt.Errorf("IMAP SEARCH failed: %s", status)
	}

	var uids []uint32
	for _, line := range lines {
		if strings.HasPrefix(line, "* SEARCH") {
			parts := strings.Fields(line)
			for _, p := range parts[2:] {
				if uid, err := strconv.ParseUint(p, 10, 32); err == nil {
					uids = append(uids, uint32(uid))
				}
			}
		}
	}
	return uids, nil
}

// ── IMAP Response Parser ────────────────────────────────────────────────────

var reFetchStart = regexp.MustCompile(`^\* (\d+) FETCH`)
var reUID = regexp.MustCompile(`UID (\d+)`)
var reExcessiveNewlines = regexp.MustCompile(`\n{3,}`)

func parseIMAPFetch(lines []string, logger *slog.Logger) []EmailMessage {
	var messages []EmailMessage
	var current *EmailMessage
	var section string // "header" or "body"
	var buf strings.Builder
	var remaining int // literal bytes remaining

	flushSection := func() {
		if current == nil {
			return
		}
		text := buf.String()
		buf.Reset()
		switch section {
		case "header":
			parseHeaders(current, text)
		case "body":
			current.Body = decodeBodyText(text)
			// Cap body at 4KB for LLM context
			if len(current.Body) > 4096 {
				current.Body = truncateUTF8Safe(current.Body, 4096) + "\n[... truncated]"
			}
			current.Snippet = truncateSnippet(current.Body, 200)
		}
		section = ""
	}

	for _, line := range lines {
		// Handle literal continuation
		if remaining > 0 {
			buf.WriteString(line)
			buf.WriteString("\n")
			remaining -= len(line) + 2 // approximate CRLF
			if remaining <= 0 {
				remaining = 0
			}
			continue
		}

		// New fetch block
		if reFetchStart.MatchString(line) {
			flushSection()
			if current != nil {
				messages = append(messages, *current)
			}
			current = &EmailMessage{}
			if m := reUID.FindStringSubmatch(line); len(m) > 1 {
				uid, _ := strconv.ParseUint(m[1], 10, 32)
				current.UID = uint32(uid)
			}
		}

		// Detect section start
		if strings.Contains(line, "BODY[HEADER.FIELDS") {
			flushSection()
			section = "header"
			if n := parseLiteralSize(line); n > 0 {
				remaining = n
			}
			continue
		}
		if strings.Contains(line, "BODY[TEXT]") {
			flushSection()
			section = "body"
			if n := parseLiteralSize(line); n > 0 {
				remaining = n
			}
			continue
		}

		if section != "" {
			if line == ")" || line == "" && section == "header" {
				flushSection()
				continue
			}
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}

	flushSection()
	if current != nil {
		messages = append(messages, *current)
	}
	return messages
}

func parseLiteralSize(line string) int {
	idx := strings.LastIndex(line, "{")
	if idx < 0 {
		return 0
	}
	end := strings.LastIndex(line, "}")
	if end <= idx {
		return 0
	}
	n, _ := strconv.Atoi(line[idx+1 : end])
	return n
}

func parseHeaders(msg *EmailMessage, raw string) {
	reader := textproto.NewReader(bufio.NewReader(strings.NewReader(raw)))
	headers, err := reader.ReadMIMEHeader()
	if err != nil && len(headers) == 0 {
		return
	}
	msg.From = decodeHeader(headers.Get("From"))
	msg.To = decodeHeader(headers.Get("To"))
	msg.Subject = decodeHeader(headers.Get("Subject"))
	msg.Date = headers.Get("Date")
}

func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

func decodeBodyText(raw string) string {
	// Try quoted-printable first (common in emails)
	qpReader := quotedprintable.NewReader(strings.NewReader(raw))
	decoded, err := io.ReadAll(qpReader)
	if err == nil && len(decoded) > 0 {
		text := string(decoded)
		if !isGarbage(text) {
			return cleanEmailBody(text)
		}
	}

	// Try base64
	b64, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err == nil && len(b64) > 0 && !isGarbage(string(b64)) {
		return cleanEmailBody(string(b64))
	}

	return cleanEmailBody(raw)
}

func isGarbage(s string) bool {
	nonPrintable := 0
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			nonPrintable++
		}
	}
	return len(s) > 0 && float64(nonPrintable)/float64(len(s)) > 0.3
}

func cleanEmailBody(s string) string {
	// Strip HTML tags if present
	s = reTag.ReplaceAllString(s, "")
	// Collapse excessive whitespace
	s = reExcessiveNewlines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func truncateSnippet(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return truncateUTF8Safe(s, maxLen) + "..."
}

func quoteIMAPString(s string) string {
	// Double-quote the string, escaping internal quotes and backslashes
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// ── SMTP Send ───────────────────────────────────────────────────────────────

// SendEmail sends an email via SMTP with STARTTLS.
func SendEmail(smtpHost string, smtpPort int, username, password, from, to, subject, body string, logger *slog.Logger) error {
	if from == "" {
		from = username
	}

	addr := net.JoinHostPort(smtpHost, fmt.Sprintf("%d", smtpPort))

	// Build RFC 5322 message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: =?UTF-8?B?%s?=\r\n", base64.StdEncoding.EncodeToString([]byte(subject))))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// Connect and negotiate STARTTLS
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return fmt.Errorf("SMTP connection failed: %w", err)
	}

	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	// STARTTLS — required. Credentials must not be sent over unencrypted connections.
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: smtpHost}); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	} else {
		return fmt.Errorf("SMTP server %s does not support STARTTLS: refusing to send credentials over unencrypted connection (use port 465 with TLS instead)", smtpHost)
	}

	// Authenticate
	auth := smtp.PlainAuth("", username, password, smtpHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}

	// Send
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}

	recipients := strings.Split(to, ",")
	for _, rcpt := range recipients {
		rcpt = strings.TrimSpace(rcpt)
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("SMTP RCPT TO <%s> failed: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := io.WriteString(w, msg.String()); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP close failed: %w", err)
	}

	logger.Info("[Email] Message sent", "from", from, "to", to, "subject", subject)
	return client.Quit()
}

// ── SMTP via TLS (port 465) ─────────────────────────────────────────────────

// SendEmailTLS sends an email via direct TLS (port 465, implicit TLS).
func SendEmailTLS(smtpHost string, smtpPort int, username, password, from, to, subject, body string, logger *slog.Logger) error {
	if from == "" {
		from = username
	}

	addr := net.JoinHostPort(smtpHost, fmt.Sprintf("%d", smtpPort))

	// Build RFC 5322 message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: =?UTF-8?B?%s?=\r\n", base64.StdEncoding.EncodeToString([]byte(subject))))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	// Direct TLS connection
	tlsConn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 15 * time.Second},
		"tcp", addr,
		&tls.Config{ServerName: smtpHost},
	)
	if err != nil {
		return fmt.Errorf("SMTPS TLS dial failed: %w", err)
	}

	client, err := smtp.NewClient(tlsConn, smtpHost)
	if err != nil {
		tlsConn.Close()
		return fmt.Errorf("SMTPS client creation failed: %w", err)
	}
	defer client.Close()

	// Authenticate
	auth := smtp.PlainAuth("", username, password, smtpHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTPS auth failed: %w", err)
	}

	// Send
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTPS MAIL FROM failed: %w", err)
	}
	recipients := strings.Split(to, ",")
	for _, rcpt := range recipients {
		rcpt = strings.TrimSpace(rcpt)
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("SMTPS RCPT TO <%s> failed: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTPS DATA failed: %w", err)
	}
	if _, err := io.WriteString(w, msg.String()); err != nil {
		return fmt.Errorf("SMTPS write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTPS close failed: %w", err)
	}

	logger.Info("[Email] Message sent via TLS", "from", from, "to", to, "subject", subject)
	return client.Quit()
}

// ── Multipart helper ────────────────────────────────────────────────────────

// ParseMultipartBody is a helper that extracts text/plain from multipart MIME.
// Used when the raw body contains MIME boundaries.
func ParseMultipartBody(raw, boundary string) string {
	r := multipart.NewReader(strings.NewReader(raw), boundary)
	for {
		part, err := r.NextPart()
		if err != nil {
			break
		}
		ct := part.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "text/plain") {
			body, err := io.ReadAll(io.LimitReader(part, 8192))
			if err == nil {
				return string(body)
			}
		}
		part.Close()
	}
	return ""
}
