package tools

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// FetchEmailsByUID connects to IMAP and fetches specific emails by their UIDs.
// This is used by the EmailWatcher to fetch exactly the newly detected messages
// instead of relying on sequence-number ranges which may return wrong emails.
func FetchEmailsByUID(host string, port int, username, password, folder string, uids []uint32, logger *slog.Logger) ([]EmailMessage, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	if len(uids) > 50 {
		uids = uids[:50]
	}

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

	// Build UID set string: "123 456 789"
	parts := make([]string, len(uids))
	for i, uid := range uids {
		parts[i] = strconv.FormatUint(uint64(uid), 10)
	}
	uidSet := strings.Join(parts, ",")

	// UID FETCH by specific UIDs
	lines, status, err := ic.command("UID FETCH %s (UID BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)] BODY[TEXT])", uidSet)
	if err != nil {
		return nil, fmt.Errorf("IMAP UID FETCH failed: %w", err)
	}
	if !strings.Contains(status, "OK") {
		return nil, fmt.Errorf("IMAP UID FETCH failed: %s", status)
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

// TestIMAPConnection verifies IMAP login credentials without fetching mail.
func TestIMAPConnection(host string, port int, username, password string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("IMAP host is required")
	}
	if port <= 0 {
		port = 993
	}
	ic, err := imapDial(host, port)
	if err != nil {
		return err
	}
	defer ic.Close()
	return ic.login(username, password)
}

// TestSMTPAuth verifies SMTP credentials without sending mail.
func TestSMTPAuth(host string, port int, username, password string) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if port <= 0 {
		port = 587
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	if port == 465 {
		tlsConn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 15 * time.Second},
			"tcp", addr,
			&tls.Config{ServerName: host},
		)
		if err != nil {
			return fmt.Errorf("SMTPS TLS dial failed: %w", err)
		}
		client, err := smtp.NewClient(tlsConn, host)
		if err != nil {
			tlsConn.Close()
			return fmt.Errorf("SMTPS client creation failed: %w", err)
		}
		defer client.Close()
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTPS auth failed: %w", err)
		}
		return nil
	}

	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return fmt.Errorf("SMTP connection failed: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	} else {
		return fmt.Errorf("SMTP server %s does not support STARTTLS (use port 465 with TLS instead)", host)
	}

	auth := smtp.PlainAuth("", username, password, host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}
	return nil
}

// ── SMTP Send ───────────────────────────────────────────────────────────────

// SendEmail sends an email via SMTP with STARTTLS.
func SendEmail(smtpHost string, smtpPort int, username, password, from, to, subject, body string, logger *slog.Logger) error {
	if from == "" {
		from = username
	}
	msg := buildEmailMessage(from, to, subject, body, time.Now(), nil, "")
	if err := deliverSMTP(smtpHost, smtpPort, username, password, from, to, msg, false); err != nil {
		return err
	}
	logger.Info("[Email] Message sent", "from", from, "to", truncateStr(to, smtpMaxEchoRunes), "subject", truncateStr(subject, smtpMaxEchoRunes))
	return nil
}

// ── SMTP via TLS (port 465) ─────────────────────────────────────────────────

// SendEmailTLS sends an email via direct TLS (port 465, implicit TLS).
func SendEmailTLS(smtpHost string, smtpPort int, username, password, from, to, subject, body string, logger *slog.Logger) error {
	if from == "" {
		from = username
	}
	msg := buildEmailMessage(from, to, subject, body, time.Now(), nil, "")
	if err := deliverSMTP(smtpHost, smtpPort, username, password, from, to, msg, true); err != nil {
		return err
	}
	logger.Info("[Email] Message sent via TLS", "from", from, "to", truncateStr(to, smtpMaxEchoRunes), "subject", truncateStr(subject, smtpMaxEchoRunes))
	return nil
}

// buildEmailMessage renders an RFC 5322 message. Without attachments it is the plain
// text/plain message AuraGo always sent; with attachments it is multipart/mixed.
func buildEmailMessage(from, to, subject, body string, now time.Time, attachments []EmailAttachment, boundary string) string {
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: =?UTF-8?B?%s?=\r\n", base64.StdEncoding.EncodeToString([]byte(subject))))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", now.Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	if len(attachments) == 0 {
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(body)
		return msg.String()
	}
	msg.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n\r\n")
	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msg.WriteString(body)
	msg.WriteString("\r\n")
	for _, a := range attachments {
		contentType, disposition := emailAttachmentHeaders(a)
		msg.WriteString("--" + boundary + "\r\n")
		msg.WriteString("Content-Type: " + contentType + "\r\n")
		msg.WriteString("Content-Disposition: " + disposition + "\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		writeBase64Lines(&msg, a.Data)
	}
	msg.WriteString("--" + boundary + "--\r\n")
	return msg.String()
}

// smtpRootCAs are the roots deliverSMTP verifies the server certificate against. nil, the
// production value, means the system roots; tests set the certificate of their fake server.
var smtpRootCAs *x509.CertPool

// smtpMaxEchoRunes bounds how much of a recipient or subject an error or log line repeats.
const smtpMaxEchoRunes = 200

// The deadline of one SMTP session (see smtpSessionTimeout). Variables, so tests can lower
// them.
var (
	smtpSessionBaseTimeout     = 2 * time.Minute
	smtpSessionTimeoutPerChunk = time.Minute
)

// smtpSessionTimeoutChunkBytes is the message size that earns one more smtpSessionTimeoutPerChunk.
const smtpSessionTimeoutChunkBytes = 5 << 20

// smtpSessionTimeout is how long a whole SMTP session may take after the dial, from the
// greeting to QUIT, for a message of messageBytes: two minutes plus one minute for every
// full 5 MiB. A server that stops answering mid-session then fails the send instead of
// blocking it for ever. A plain message gets two minutes; a message at the attachment limit
// (20 MiB, about 27 MiB in base64) gets seven, which still lets a link of about 65 KB/s
// deliver it.
func smtpSessionTimeout(messageBytes int) time.Duration {
	return smtpSessionBaseTimeout + time.Duration(messageBytes/smtpSessionTimeoutChunkBytes)*smtpSessionTimeoutPerChunk
}

// checkEmailEnvelope refuses a sender or recipient value that would change the message
// headers: buildEmailMessage writes from and to raw into "From:" and "To:", so a CR or LF
// adds header lines or ends the header block, and a NUL is not allowed in a message. A
// recipient list with an empty entry is refused as well. Nothing is stripped: a value
// changed by stripping could reach another address than the caller named. It returns the
// trimmed recipients. The messages never repeat the values.
func checkEmailEnvelope(from, to string) ([]string, error) {
	if strings.ContainsAny(from, "\r\n\x00") {
		return nil, errors.New("the sender address contains a line break or a NUL character")
	}
	if strings.ContainsAny(to, "\r\n\x00") {
		return nil, errors.New("the recipient address contains a line break or a NUL character")
	}
	parts := strings.Split(to, ",")
	recipients := make([]string, 0, len(parts))
	for _, rcpt := range parts {
		rcpt = strings.TrimSpace(rcpt)
		if rcpt == "" {
			return nil, errors.New("the recipient list has an empty entry")
		}
		recipients = append(recipients, rcpt)
	}
	return recipients, nil
}

// deliverSMTP sends a rendered message over STARTTLS (implicitTLS=false) or implicit TLS.
// It first checks from and to with checkEmailEnvelope, before any connection, so every send
// path refuses header injection. The whole session after the dial is bounded by
// smtpSessionTimeout. The SMTP error texts are the ones SendEmail and SendEmailTLS always
// returned, except that a refused recipient is repeated with at most smtpMaxEchoRunes runes.
func deliverSMTP(smtpHost string, smtpPort int, username, password, from, to, message string, implicitTLS bool) error {
	recipients, err := checkEmailEnvelope(from, to)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(smtpHost, fmt.Sprintf("%d", smtpPort))
	timeout := smtpSessionTimeout(len(message))
	label := "SMTP"
	var client *smtp.Client
	if implicitTLS {
		label = "SMTPS"
		tlsConn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", addr, &tls.Config{ServerName: smtpHost, RootCAs: smtpRootCAs})
		if err != nil {
			return fmt.Errorf("SMTPS TLS dial failed: %w", err)
		}
		_ = tlsConn.SetDeadline(time.Now().Add(timeout))
		c, err := smtp.NewClient(tlsConn, smtpHost)
		if err != nil {
			tlsConn.Close()
			return fmt.Errorf("SMTPS client creation failed: %w", err)
		}
		client = c
	} else {
		conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
		if err != nil {
			return fmt.Errorf("SMTP connection failed: %w", err)
		}
		// Set on the plain connection, the deadline also bounds the session after STARTTLS:
		// the TLS layer reads and writes through this connection.
		_ = conn.SetDeadline(time.Now().Add(timeout))
		c, err := smtp.NewClient(conn, smtpHost)
		if err != nil {
			conn.Close()
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}
		client = c
	}
	defer client.Close()
	if !implicitTLS {
		// STARTTLS — required. Credentials must not be sent over unencrypted connections.
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: smtpHost, RootCAs: smtpRootCAs}); err != nil {
				return fmt.Errorf("STARTTLS failed: %w", err)
			}
		} else {
			return fmt.Errorf("SMTP server %s does not support STARTTLS: refusing to send credentials over unencrypted connection (use port 465 with TLS instead)", smtpHost)
		}
	}
	if err := client.Auth(smtp.PlainAuth("", username, password, smtpHost)); err != nil {
		return fmt.Errorf("%s auth failed: %w", label, err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("%s MAIL FROM failed: %w", label, err)
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("%s RCPT TO <%s> failed: %w", label, truncateStr(rcpt, smtpMaxEchoRunes), err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("%s DATA failed: %w", label, err)
	}
	if _, err := io.WriteString(w, message); err != nil {
		return fmt.Errorf("%s write failed: %w", label, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("%s close failed: %w", label, err)
	}
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
