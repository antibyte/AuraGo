package tools

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"math/rand/v2"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
)

// ── Fake SMTP server ────────────────────────────────────────────────────────

// c05Mail is one message the fake SMTP server received.
type c05Mail struct {
	from  string   // the MAIL command line
	rcpts []string // the RCPT command lines
	raw   string   // the DATA lines as they came over the wire, without the final ".\r\n"
	data  string   // the same, dot-unstuffed
	size  int      // the byte count of raw, also when the server discards the data
}

// c05FakeSMTP is a minimal SMTP server on 127.0.0.1. It offers STARTTLS on a plain
// listener or speaks implicit TLS, accepts any AUTH and records every message.
type c05FakeSMTP struct {
	host        string
	port        int
	implicitTLS bool
	tlsConfig   *tls.Config
	// hangAt makes the server stop answering: "greeting" right after the connection (after
	// the handshake under implicit TLS), "DOT" after the final dot of the data (the message
	// is recorded, the reply never comes), or at the first command with this verb ("MAIL").
	hangAt     string
	rejectRcpt bool
	// finalReplyDelay delays the reply to the final dot.
	finalReplyDelay time.Duration
	// discardData counts the data instead of keeping it, so the server allocates nothing
	// per message.
	discardData bool

	ln   net.Listener
	done chan struct{}
	wg   sync.WaitGroup

	mu       sync.Mutex
	accepted int
	hung     int
	mails    []c05Mail
}

// c05StartFakeSMTP starts the fake server and points deliverSMTP's TLS roots at its
// certificate until the test ends.
func c05StartFakeSMTP(t *testing.T, implicitTLS bool, configure func(*c05FakeSMTP)) *c05FakeSMTP {
	t.Helper()
	serverTLS, roots := c05TLSMaterial(t)
	s := &c05FakeSMTP{implicitTLS: implicitTLS, tlsConfig: serverTLS, done: make(chan struct{})}
	if configure != nil {
		configure(s)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.host, s.port = "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
	if implicitTLS {
		ln = tls.NewListener(ln, serverTLS)
	}
	s.ln = ln
	previous := smtpRootCAs
	smtpRootCAs = roots
	s.wg.Add(1)
	go s.acceptLoop()
	t.Cleanup(func() {
		close(s.done)
		_ = ln.Close()
		s.wg.Wait()
		smtpRootCAs = previous
	})
	return s
}

// c05TLSMaterial borrows the test certificate of net/http/httptest (valid for 127.0.0.1).
func c05TLSMaterial(t *testing.T) (*tls.Config, *x509.CertPool) {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.StartTLS()
	defer srv.Close()
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	return &tls.Config{Certificates: srv.TLS.Certificates, MinVersion: tls.VersionTLS12}, roots
}

func (s *c05FakeSMTP) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.accepted++
		s.mu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.serve(conn)
		}()
	}
}

func (s *c05FakeSMTP) hang() {
	s.mu.Lock()
	s.hung++
	s.mu.Unlock()
	<-s.done
}

func (s *c05FakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	// A connection the client leaks fails here instead of holding the test until its timeout.
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if tc, ok := conn.(*tls.Conn); ok {
		if err := tc.Handshake(); err != nil {
			return
		}
	}
	if s.hangAt == "greeting" {
		s.hang()
		return
	}
	r := bufio.NewReader(conn)
	w := io.Writer(conn)
	reply := func(lines ...string) {
		for _, line := range lines {
			if _, err := io.WriteString(w, line+"\r\n"); err != nil {
				return
			}
		}
	}
	secure := s.implicitTLS
	reply("220 fake.test ESMTP")
	var cur c05Mail
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimRight(line, "\r\n")
		verb, _, _ := strings.Cut(cmd, " ")
		verb = strings.ToUpper(verb)
		if verb == s.hangAt {
			s.hang()
			return
		}
		switch verb {
		case "EHLO":
			if secure {
				reply("250-fake.test", "250 AUTH PLAIN")
			} else {
				reply("250-fake.test", "250-STARTTLS", "250 AUTH PLAIN")
			}
		case "STARTTLS":
			reply("220 ready")
			tc := tls.Server(conn, s.tlsConfig)
			if err := tc.Handshake(); err != nil {
				return
			}
			r, w, secure = bufio.NewReader(tc), tc, true
		case "AUTH":
			reply("235 accepted")
		case "MAIL":
			cur = c05Mail{from: cmd}
			reply("250 ok")
		case "RCPT":
			if s.rejectRcpt {
				reply("550 no such user")
				continue
			}
			cur.rcpts = append(cur.rcpts, cmd)
			reply("250 ok")
		case "DATA":
			reply("354 end with .")
			if cur.raw, cur.data, cur.size, err = c05ReadData(r, !s.discardData); err != nil {
				return
			}
			s.mu.Lock()
			s.mails = append(s.mails, cur)
			s.mu.Unlock()
			if s.hangAt == "DOT" {
				s.hang()
				return
			}
			if s.finalReplyDelay > 0 {
				select {
				case <-time.After(s.finalReplyDelay):
				case <-s.done:
					return
				}
			}
			reply("250 queued")
		case "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

// c05ReadData reads SMTP DATA lines up to the final ".\r\n". It returns them as they came
// (raw), dot-unstuffed (data), and their byte count. With keep false it only counts, and
// allocates nothing.
func c05ReadData(r *bufio.Reader, keep bool) (raw, data string, size int, err error) {
	var rawB, dataB strings.Builder
	lineStart := true
	for {
		// A line longer than the reader's buffer comes in several chunks (ErrBufferFull).
		chunk, err := r.ReadSlice('\n')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return "", "", 0, err
		}
		complete := err == nil
		if lineStart && complete && string(chunk) == ".\r\n" {
			return rawB.String(), dataB.String(), size, nil
		}
		size += len(chunk)
		if keep {
			rawB.Write(chunk)
			if lineStart && chunk[0] == '.' {
				chunk = chunk[1:]
			}
			dataB.Write(chunk)
		}
		lineStart = complete
	}
}

// c05WireData is what the fake server records as data when the SMTP data writer sends msg.
func c05WireData(t *testing.T, msg string) string {
	t.Helper()
	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)
	w := textproto.NewWriter(bw).DotWriter()
	if _, err := io.WriteString(w, msg); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	_, data, _, err := c05ReadData(bufio.NewReader(&buf), true)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// c05Allocated returns the bytes the process allocated while fn ran.
func c05Allocated(fn func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func (s *c05FakeSMTP) snapshot() (accepted, hung int, mails []c05Mail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accepted, s.hung, slices.Clone(s.mails)
}

func c05Logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// c05Sender is one of the four public send functions, bound to fixed credentials and text.
type c05Sender struct {
	name        string
	implicitTLS bool
	send        func(host string, port int, from, to string) error
}

func c05Senders(attachments []EmailAttachment) []c05Sender {
	logger := c05Logger()
	return []c05Sender{
		{"SendEmail", false, func(host string, port int, from, to string) error {
			return SendEmail(host, port, "user", "pass", from, to, "Betreff", "Text", logger)
		}},
		{"SendEmailTLS", true, func(host string, port int, from, to string) error {
			return SendEmailTLS(host, port, "user", "pass", from, to, "Betreff", "Text", logger)
		}},
		{"SendEmailWithAttachments", false, func(host string, port int, from, to string) error {
			return SendEmailWithAttachments(host, port, "user", "pass", from, to, "Betreff", "Text", attachments, logger)
		}},
		{"SendEmailTLSWithAttachments", true, func(host string, port int, from, to string) error {
			return SendEmailTLSWithAttachments(host, port, "user", "pass", from, to, "Betreff", "Text", attachments, logger)
		}},
	}
}

// ── Message parsing helpers ─────────────────────────────────────────────────

// c05Part is one attachment part of a received message.
type c05Part struct {
	header      map[string][]string
	mediaType   string
	typeParams  map[string]string
	disposition string
	dispParams  map[string]string
	data        []byte
}

// c05ParseMessage reads a multipart/mixed message and returns its headers, its text part
// and its attachment parts.
func c05ParseMessage(t *testing.T, raw string) (map[string][]string, string, []c05Part) {
	t.Helper()
	c05AssertLines(t, raw)
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		t.Fatalf("content type = %q, %v", mediaType, err)
	}
	reader := multipart.NewReader(msg.Body, params["boundary"])
	textPart, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	text, _ := io.ReadAll(textPart)
	var parts []c05Part
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := io.ReadAll(part)
		data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(encoded), "\r\n", ""))
		if err != nil {
			t.Fatalf("attachment is not base64: %v", err)
		}
		p := c05Part{header: map[string][]string(part.Header), data: data}
		if p.mediaType, p.typeParams, err = mime.ParseMediaType(part.Header.Get("Content-Type")); err != nil {
			t.Fatalf("Content-Type %q: %v", part.Header.Get("Content-Type"), err)
		}
		if p.disposition, p.dispParams, err = mime.ParseMediaType(part.Header.Get("Content-Disposition")); err != nil {
			t.Fatalf("Content-Disposition %q: %v", part.Header.Get("Content-Disposition"), err)
		}
		parts = append(parts, p)
	}
	return map[string][]string(msg.Header), string(text), parts
}

// c05AssertLines checks RFC 5322 line rules: CRLF only, at most 998 characters per line.
func c05AssertLines(t *testing.T, raw string) {
	t.Helper()
	for i, line := range strings.Split(raw, "\r\n") {
		if strings.ContainsAny(line, "\r\n") {
			t.Fatalf("line %d has a bare CR or LF: %q", i, line)
		}
		if len(line) > 998 {
			t.Fatalf("line %d has %d characters", i, len(line))
		}
	}
}

func c05AssertHeaderKeys(t *testing.T, header map[string][]string, want ...string) {
	t.Helper()
	got := slices.Sorted(maps.Keys(header))
	want = slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Fatalf("headers = %v, want exactly %v", got, want)
	}
	for key, values := range header {
		if len(values) != 1 {
			t.Fatalf("header %s appears %d times", key, len(values))
		}
	}
}

// c05LegacyPlainMessage is the message SendEmail and SendEmailTLS built before task 1c-05,
// copied from their old code.
func c05LegacyPlainMessage(from, to, subject, body string, now time.Time) string {
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: =?UTF-8?B?%s?=\r\n", base64.StdEncoding.EncodeToString([]byte(subject))))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", now.Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	return msg.String()
}

func c05Workspace(t *testing.T) (*config.Config, string) {
	t.Helper()
	workspace := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	return cfg, workspace
}

func c05WriteFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// ── 1. Header injection ─────────────────────────────────────────────────────

func TestEmailSendRefusesLineBreaksAndNULInAddresses(t *testing.T) {
	attachments := []EmailAttachment{{Name: "a.txt", ContentType: "text/plain", Data: []byte("hallo")}}
	for _, sender := range c05Senders(attachments) {
		t.Run(sender.name, func(t *testing.T) {
			srv := c05StartFakeSMTP(t, sender.implicitTLS, nil)
			for _, bad := range []string{"\r", "\n", "\r\n", "\x00"} {
				// A trailing line break survives the recipient TrimSpace and would end the
				// header block early; the other forms add a header.
				for _, injected := range []string{
					"x@example.com" + bad + "Bcc: evil@example.com",
					"x@example.com" + bad,
					"x@example.com," + bad + "Bcc: evil@example.com",
				} {
					if err := sender.send(srv.host, srv.port, "a@example.com", injected); err == nil {
						t.Fatalf("to %q was sent", injected)
					}
					if err := sender.send(srv.host, srv.port, injected, "b@example.com"); err == nil {
						t.Fatalf("from %q was sent", injected)
					}
				}
			}
			for _, to := range []string{"", " ", "b@example.com,", ",b@example.com", "b@example.com, ,c@example.com"} {
				if err := sender.send(srv.host, srv.port, "a@example.com", to); err == nil {
					t.Fatalf("recipient list %q with an empty entry was sent", to)
				}
			}
			if accepted, _, mails := srv.snapshot(); accepted != 0 || len(mails) != 0 {
				t.Fatalf("refused sends reached the server: %d connections, %d messages", accepted, len(mails))
			}
			// The same server takes a valid message, so the refusals are not a broken server.
			if err := sender.send(srv.host, srv.port, "a@example.com", "b@example.com, c@example.com"); err != nil {
				t.Fatalf("valid send: %v", err)
			}
			_, _, mails := srv.snapshot()
			if len(mails) != 1 || strings.Join(mails[0].rcpts, "|") != "RCPT TO:<b@example.com>|RCPT TO:<c@example.com>" {
				t.Fatalf("mails = %+v", mails)
			}
		})
	}
}

func TestBuildEmailMessageAttachmentNameCannotAddHeaders(t *testing.T) {
	name := "Bericht \"Q3\"\r\nBcc: evil@example.com ä.pdf"
	attachments := []EmailAttachment{{Name: name, ContentType: "application/pdf", Data: []byte("%PDF-1.4")}}
	raw := buildEmailMessage("a@example.com", "b@example.com", "Bericht", "Siehe Anhang", emailTestTime, attachments, "aurago-test-boundary")
	header, text, parts := c05ParseMessage(t, raw)
	c05AssertHeaderKeys(t, header, "From", "To", "Subject", "Date", "Mime-Version", "Content-Type")
	if strings.TrimSpace(text) != "Siehe Anhang" {
		t.Fatalf("text part = %q", text)
	}
	if len(parts) != 1 {
		t.Fatalf("%d attachment parts, want 1", len(parts))
	}
	part := parts[0]
	c05AssertHeaderKeys(t, part.header, "Content-Type", "Content-Disposition", "Content-Transfer-Encoding")
	want := "Bericht \"Q3\"__Bcc: evil@example.com ä.pdf"
	if part.mediaType != "application/pdf" || part.typeParams["name"] != want {
		t.Fatalf("content type = %q %v", part.mediaType, part.typeParams)
	}
	if part.disposition != "attachment" || part.dispParams["filename"] != want {
		t.Fatalf("disposition = %q %v", part.disposition, part.dispParams)
	}
	if string(part.data) != "%PDF-1.4" {
		t.Fatalf("data = %q", part.data)
	}
}

func TestBuildEmailMessageAttachmentHeadersAreValidForAnyNameAndType(t *testing.T) {
	cases := []struct {
		label, name, contentType, wantName, wantType string
	}{
		{"empty name", "", "application/pdf", "attachment", "application/pdf"},
		{"dot", ".", "application/pdf", "attachment", "application/pdf"},
		{"only controls", "\x00\x01", "application/pdf", "__", "application/pdf"},
		{"tab", "a\tb.pdf", "application/pdf", "a_b.pdf", "application/pdf"},
		{"quotes and backslash", "a \"b\" c\\d.pdf", "application/pdf", "a \"b\" c\\d.pdf", "application/pdf"},
		{"invalid UTF-8", "\xff\xfe.pdf", "application/pdf", "\uFFFD.pdf", "application/pdf"},
		{"emoji", "Grüße 🎉.pdf", "application/pdf", "Grüße 🎉.pdf", "application/pdf"},
		{"long name keeps extension", strings.Repeat("ä", 400) + ".pdf", "application/pdf", strings.Repeat("ä", 125) + ".pdf", "application/pdf"},
		{"type with parameters", "notiz.txt", "text/plain; charset=utf-8", "notiz.txt", "text/plain"},
		{"composite type", "mail.eml", "message/rfc822", "mail.eml", "application/octet-stream"},
		{"multipart type", "x.bin", "multipart/mixed; boundary=x", "x.bin", "application/octet-stream"},
		{"no subtype", "x.bin", "text", "x.bin", "application/octet-stream"},
		{"not a type", "x.bin", "not a type", "x.bin", "application/octet-stream"},
		{"empty type", "x.bin", "", "x.bin", "application/octet-stream"},
		{"type with line break", "x.bin", "text/plain\r\nBcc: evil@example.com", "x.bin", "application/octet-stream"},
		{"bidi override", "rechnung\u202Efdp.exe", "application/pdf", "rechnung_fdp.exe", "application/pdf"},
		{"bidi isolate and mark", "a\u2066b\u200Fc.pdf", "application/pdf", "a_b_c.pdf", "application/pdf"},
		{"2000-char subtype", "a.pdf", "application/" + strings.Repeat("b", 2000), "a.pdf", "application/octet-stream"},
		{"2000-char type", "a.pdf", strings.Repeat("a", 2000) + "/pdf", "a.pdf", "application/octet-stream"},
		{"2000-char parameter", "a.pdf", "application/pdf; x=" + strings.Repeat("a", 2000), "a.pdf", "application/pdf"},
		{"2000-char charset", "a.txt", "text/plain; charset=" + strings.Repeat("u", 2000), "a.txt", "text/plain"},
		{"longest type names", "a.pdf", strings.Repeat("a", 127) + "/" + strings.Repeat("b", 127), "a.pdf",
			strings.Repeat("a", 127) + "/" + strings.Repeat("b", 127)},
		{"longest type names and name", strings.Repeat("ä", 400) + ".pdf", strings.Repeat("a", 127) + "/" + strings.Repeat("b", 127),
			strings.Repeat("ä", 125) + ".pdf", "application/octet-stream"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			data := []byte("data of " + c.label)
			attachments := []EmailAttachment{{Name: c.name, ContentType: c.contentType, Data: data}}
			raw := buildEmailMessage("a@example.com", "b@example.com", "Betreff", "Text", emailTestTime, attachments, "aurago-test-boundary")
			header, _, parts := c05ParseMessage(t, raw)
			c05AssertHeaderKeys(t, header, "From", "To", "Subject", "Date", "Mime-Version", "Content-Type")
			if len(parts) != 1 {
				t.Fatalf("%d attachment parts, want 1", len(parts))
			}
			part := parts[0]
			c05AssertHeaderKeys(t, part.header, "Content-Type", "Content-Disposition", "Content-Transfer-Encoding")
			if part.mediaType != c.wantType || part.typeParams["name"] != c.wantName {
				t.Fatalf("content type = %q %q", part.mediaType, part.typeParams)
			}
			for key := range part.typeParams {
				if key != "name" && key != "charset" {
					t.Fatalf("content type keeps parameter %q", key)
				}
			}
			if charset := part.typeParams["charset"]; len(charset) > emailMaxCharsetLen {
				t.Fatalf("content type keeps a %d-character charset", len(charset))
			}
			if part.disposition != "attachment" || part.dispParams["filename"] != c.wantName {
				t.Fatalf("disposition = %q %q", part.disposition, part.dispParams)
			}
			if len(c.wantName) > emailMaxAttachmentNameBytes {
				t.Fatalf("name has %d bytes", len(c.wantName))
			}
			if string(part.data) != string(data) {
				t.Fatalf("data = %q", part.data)
			}
		})
	}
	// The parameters of a type survive (mime.TypeByExtension gives text types a charset).
	attachments := []EmailAttachment{{Name: "notiz.txt", ContentType: "text/plain; charset=utf-8", Data: []byte("x")}}
	_, _, parts := c05ParseMessage(t, buildEmailMessage("a@example.com", "b@example.com", "s", "b", emailTestTime, attachments, "b0"))
	if parts[0].typeParams["charset"] != "utf-8" {
		t.Fatalf("charset lost: %v", parts[0].typeParams)
	}
}

func TestEmailAttachmentFallbackName(t *testing.T) {
	for name, want := range map[string]string{
		"Bericht ä.PDF":         "attachment.pdf",
		"archiv.tar.gz":         "attachment.gz",
		"ohne":                  "attachment",
		"":                      "attachment",
		"x.p d":                 "attachment",
		"x.ä":                   "attachment",
		"x.sehrlangeendung1234": "attachment",
	} {
		if got := emailAttachmentFallbackName(name); got != want {
			t.Errorf("emailAttachmentFallbackName(%q) = %q, want %q", name, got, want)
		}
	}
}

// ── 2. Bounded reads through OpenOutgoingAttachment ─────────────────────────

func TestLoadEmailAttachmentsCountsTheTotalOfAllFiles(t *testing.T) {
	cfg, workspace := c05Workspace(t)
	first := c05WriteFile(t, workspace, "first.bin", make([]byte, 15<<20))
	fits := c05WriteFile(t, workspace, "fits.bin", make([]byte, 5<<20))
	over := c05WriteFile(t, workspace, "over.bin", make([]byte, 5<<20+1))
	files, err := LoadEmailAttachments(cfg, []string{first, fits})
	if err != nil || len(files) != 2 || len(files[0].Data)+len(files[1].Data) != emailMaxAttachmentBytes {
		t.Fatalf("exactly 20 MB together must pass: %d files, %v", len(files), err)
	}
	// Each file alone is far below the limit; only the total is over it.
	if _, err := LoadEmailAttachments(cfg, []string{over}); err != nil {
		t.Fatalf("one small file: %v", err)
	}
	if _, err := LoadEmailAttachments(cfg, []string{first, over}); err == nil || !strings.Contains(err.Error(), "20 MB") {
		t.Fatalf("a total over 20 MB must be refused, got %v", err)
	}
}

// c05UnreadReader fails the test when it is read.
type c05UnreadReader struct{ t *testing.T }

func (r c05UnreadReader) Read([]byte) (int, error) {
	r.t.Error("the file was read although its size is over the budget")
	return 0, io.EOF
}

func TestReadEmailAttachmentDataLimitsTheBytesRead(t *testing.T) {
	// Stat said 5 bytes, inside the budget of 10; the file then delivers 11.
	if _, err := readEmailAttachmentData(strings.NewReader(strings.Repeat("x", 11)), 5, 10); !errors.Is(err, errEmailAttachmentsTooLarge) {
		t.Fatalf("a file that grows over the budget must be refused, got %v", err)
	}
	data, err := readEmailAttachmentData(strings.NewReader(strings.Repeat("x", 10)), 5, 10)
	if err != nil || string(data) != strings.Repeat("x", 10) {
		t.Fatalf("a file that grows up to the budget is read whole: %q, %v", data, err)
	}
	data, err = readEmailAttachmentData(strings.NewReader("abc"), 5, 10)
	if err != nil || string(data) != "abc" {
		t.Fatalf("a file that shrinks is read as it is: %q, %v", data, err)
	}
	// A size over the budget is refused without reading.
	if _, err := readEmailAttachmentData(c05UnreadReader{t}, 11, 10); !errors.Is(err, errEmailAttachmentsTooLarge) {
		t.Fatalf("a size over the budget must be refused, got %v", err)
	}
	// A read error does not repeat the path the os error carries.
	failing := iotest.ErrReader(&fs.PathError{Op: "read", Path: "/secret/dir/file", Err: errors.New("disk gone")})
	if _, err := readEmailAttachmentData(failing, 1, 10); err == nil || strings.Contains(err.Error(), "/secret/dir") {
		t.Fatalf("read error = %v", err)
	}
}

// ── 3. Bounded echoes ───────────────────────────────────────────────────────

func c05AssertBoundedEcho(t *testing.T, err error, repeated string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), strings.Repeat(repeated, 201)) || utf8.RuneCountInString(err.Error()) > 600 {
		t.Fatalf("error repeats too much of the value (%d runes)", utf8.RuneCountInString(err.Error()))
	}
}

func TestEmailErrorsBoundEchoedPathsAndRecipients(t *testing.T) {
	cfg, _ := c05Workspace(t)
	_, err := LoadEmailAttachments(cfg, []string{strings.Repeat("a", 10<<10)})
	c05AssertBoundedEcho(t, err, "a")

	srv := c05StartFakeSMTP(t, false, func(s *c05FakeSMTP) { s.rejectRcpt = true })
	recipient := strings.Repeat("r", 10<<10) + "@example.com"
	err = SendEmail(srv.host, srv.port, "user", "pass", "a@example.com", recipient, "Betreff", "Text", c05Logger())
	c05AssertBoundedEcho(t, err, "r")
	if !strings.Contains(err.Error(), "SMTP RCPT TO <rrr") || !strings.Contains(err.Error(), "no such user") {
		t.Fatalf("error = %.300s", err)
	}
}

// ── 4. Session deadline ─────────────────────────────────────────────────────

func TestSMTPSessionTimeoutGrowsWithTheMessage(t *testing.T) {
	for size, want := range map[int]time.Duration{
		0:           2 * time.Minute,
		5<<20 - 1:   2 * time.Minute,
		5 << 20:     3 * time.Minute,
		27 << 20:    7 * time.Minute,
		100<<20 + 1: 22 * time.Minute,
	} {
		if got := smtpSessionTimeout(size); got != want {
			t.Errorf("smtpSessionTimeout(%d) = %v, want %v", size, got, want)
		}
	}
}

func TestDeliverSMTPGivesUpOnAServerThatStopsAnswering(t *testing.T) {
	previous := smtpSessionBaseTimeout
	t.Cleanup(func() { smtpSessionBaseTimeout = previous })
	// The session deadline runs from the connection on, so the work before the silent point
	// must fit into it: a TLS upgrade and a few round trips, about 10 ms on loopback. Those
	// cases get 5 s (500 times that, and enough under load or the race detector). Without
	// work under the deadline (no greeting, or the implicit TLS handshake, which the dial
	// does before the deadline is set) 1 s is enough. The subtests run one after another:
	// c05StartFakeSMTP swaps the package's smtpRootCAs.
	const roomy = 5 * time.Second
	cases := []struct {
		label       string
		implicitTLS bool
		hangAt      string
		session     time.Duration
	}{
		{"STARTTLS, no greeting", false, "greeting", time.Second},
		{"STARTTLS, silent after the TLS upgrade", false, "MAIL", roomy},
		{"implicit TLS, silent after the handshake", true, "greeting", time.Second},
		{"implicit TLS, silent at DATA", true, "DATA", roomy},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			smtpSessionBaseTimeout = c.session
			srv := c05StartFakeSMTP(t, c.implicitTLS, func(s *c05FakeSMTP) { s.hangAt = c.hangAt })
			send := SendEmail
			if c.implicitTLS {
				send = SendEmailTLS
			}
			start := time.Now()
			err := send(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "Betreff", "Text", c05Logger())
			elapsed := time.Since(start)
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatalf("error = %v, want a deadline error", err)
			}
			// A negative check: the call did not hang (the production session deadline is
			// 2 minutes).
			if elapsed > 30*time.Second {
				t.Fatalf("the call took %v", elapsed)
			}
			if _, hung, _ := srv.snapshot(); hung != 1 {
				t.Fatalf("the session did not reach %q", c.hangAt)
			}
		})
	}
}

// ── Acceptance: the files and the plain message reach the server ────────────

func TestSendEmailWithAttachmentsDeliversTheFiles(t *testing.T) {
	cfg, workspace := c05Workspace(t)
	pdfData := []byte("%PDF-1.4 \x00\x01\xff binary " + strings.Repeat("0123456789", 30))
	pdf := c05WriteFile(t, workspace, "Bericht ä.pdf", pdfData)
	note := c05WriteFile(t, workspace, "notiz.txt", []byte("Zeile 1\r\n.Zeile 2\n"))
	files, err := LoadEmailAttachments(cfg, []string{pdf, note})
	if err != nil {
		t.Fatal(err)
	}
	for _, implicitTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicitTLS=%v", implicitTLS), func(t *testing.T) {
			srv := c05StartFakeSMTP(t, implicitTLS, nil)
			send := SendEmailWithAttachments
			if implicitTLS {
				send = SendEmailTLSWithAttachments
			}
			if err := send(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com, c@example.com", "Bericht", "Siehe Anhang", files, c05Logger()); err != nil {
				t.Fatal(err)
			}
			_, _, mails := srv.snapshot()
			if len(mails) != 1 {
				t.Fatalf("%d messages arrived", len(mails))
			}
			m := mails[0]
			if m.from != "MAIL FROM:<a@example.com>" || strings.Join(m.rcpts, "|") != "RCPT TO:<b@example.com>|RCPT TO:<c@example.com>" {
				t.Fatalf("envelope = %q %q", m.from, m.rcpts)
			}
			_, text, parts := c05ParseMessage(t, m.data)
			if strings.TrimSpace(text) != "Siehe Anhang" || len(parts) != 2 {
				t.Fatalf("text = %q, %d parts", text, len(parts))
			}
			for i, want := range files {
				wantType, _, _ := mime.ParseMediaType(want.ContentType)
				got := parts[i]
				if got.dispParams["filename"] != want.Name || got.mediaType != wantType || string(got.data) != string(want.Data) {
					t.Fatalf("part %d = %q %q %q, want %q %q", i, got.dispParams["filename"], got.mediaType, got.data, want.Name, wantType)
				}
			}
			if string(parts[0].data) != string(pdfData) {
				t.Fatal("the PDF did not arrive unchanged")
			}
		})
	}
}

func TestSendEmailPlainMessageIsUnchangedOnTheWire(t *testing.T) {
	for _, implicitTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicitTLS=%v", implicitTLS), func(t *testing.T) {
			srv := c05StartFakeSMTP(t, implicitTLS, nil)
			send := SendEmail
			if implicitTLS {
				send = SendEmailTLS
			}
			body := "Hallo,\r\n.Punkt am Zeilenanfang\r\nGrüße"
			if err := send(srv.host, srv.port, "user", "pass", "", "b@example.com", "Grüße", body, c05Logger()); err != nil {
				t.Fatal(err)
			}
			_, _, mails := srv.snapshot()
			if len(mails) != 1 {
				t.Fatalf("%d messages arrived", len(mails))
			}
			msg, err := mail.ReadMessage(strings.NewReader(mails[0].data))
			if err != nil {
				t.Fatal(err)
			}
			sent, err := time.Parse(time.RFC1123Z, msg.Header.Get("Date"))
			if err != nil {
				t.Fatal(err)
			}
			// An empty from falls back to the user name, as before. The SMTP client ends the
			// data with CRLF before the final dot.
			want := c05LegacyPlainMessage("user", "b@example.com", "Grüße", body, sent) + "\r\n"
			if mails[0].data != want {
				t.Fatalf("message changed:\n%q\nwant\n%q", mails[0].data, want)
			}
			if mails[0].from != "MAIL FROM:<user>" {
				t.Fatalf("envelope from = %q", mails[0].from)
			}
		})
	}
}

// ── Review round: memory ────────────────────────────────────────────────────

// c05OldWriteBase64Lines is writeBase64Lines as it was before it streamed.
func c05OldWriteBase64Lines(b *strings.Builder, data []byte) {
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

func c05RandomBytes(seed uint64, n int) []byte {
	rng := rand.New(rand.NewPCG(seed, seed+1))
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(rng.Uint32())
	}
	return data
}

func TestWriteBase64LinesMatchesTheOldOutput(t *testing.T) {
	for _, size := range []int{0, 1, 2, 3, 56, 57, 58, 113, 114, 115, 1<<20 + 3} {
		data := c05RandomBytes(uint64(size), size)
		var got, want strings.Builder
		writeBase64Lines(&got, data)
		c05OldWriteBase64Lines(&want, data)
		if got.String() != want.String() {
			t.Fatalf("size %d: output differs from the old function", size)
		}
		if got.Len() != base64LinesLen(size) {
			t.Fatalf("size %d: base64LinesLen = %d, written %d", size, base64LinesLen(size), got.Len())
		}
	}
}

func TestEmailSendOf20MiBAllocatesTheMessageAboutOnce(t *testing.T) {
	attachments := []EmailAttachment{{Name: "a.bin", ContentType: "application/octet-stream", Data: c05RandomBytes(7, emailMaxAttachmentBytes)}}
	encoded := base64LinesLen(emailMaxAttachmentBytes)
	for _, implicitTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicitTLS=%v", implicitTLS), func(t *testing.T) {
			srv := c05StartFakeSMTP(t, implicitTLS, func(s *c05FakeSMTP) { s.discardData = true })
			send := SendEmailWithAttachments
			if implicitTLS {
				send = SendEmailTLSWithAttachments
			}
			var err error
			allocated := c05Allocated(func() {
				err = send(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "Bericht", "Siehe Anhang", attachments, c05Logger())
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, mails := srv.snapshot(); len(mails) != 1 || mails[0].size < encoded {
				t.Fatalf("the message did not arrive whole: %+v", len(mails))
			}
			t.Logf("20 MiB attachment, %.1f MiB encoded: the send allocated %.1f MiB", float64(encoded)/(1<<20), float64(allocated)/(1<<20))
			// The message itself is one allocation of about the encoded size; the old code
			// allocated about 234 MiB.
			if allocated > 2*uint64(encoded) {
				t.Fatalf("the send allocated %.1f MiB, more than twice the %.1f MiB message", float64(allocated)/(1<<20), float64(encoded)/(1<<20))
			}
		})
	}
}

func TestEmailRefusedEnvelopeBuildsNoMessage(t *testing.T) {
	attachments := []EmailAttachment{{Name: "a.bin", ContentType: "application/octet-stream", Data: make([]byte, emailMaxAttachmentBytes)}}
	body := strings.Repeat("x", 10<<20)
	logger := c05Logger()
	bad := "b@example.com\r\nBcc: evil@example.com"
	sends := map[string]func() error{
		"SendEmail": func() error {
			return SendEmail("127.0.0.1", 1, "user", "pass", "a@example.com", bad, "s", body, logger)
		},
		"SendEmailTLS": func() error {
			return SendEmailTLS("127.0.0.1", 1, "user", "pass", "a@example.com", bad, "s", body, logger)
		},
		"SendEmailWithAttachments": func() error {
			return SendEmailWithAttachments("127.0.0.1", 1, "user", "pass", "a@example.com", bad, "s", "b", attachments, logger)
		},
		"SendEmailTLSWithAttachments": func() error {
			return SendEmailTLSWithAttachments("127.0.0.1", 1, "user", "pass", "a@example.com", bad, "s", "b", attachments, logger)
		},
	}
	for name, send := range sends {
		var err error
		allocated := c05Allocated(func() { err = send() })
		if err == nil {
			t.Fatalf("%s sent a refused recipient", name)
		}
		if allocated > 1<<20 {
			t.Fatalf("%s allocated %.1f MiB for a refused recipient", name, float64(allocated)/(1<<20))
		}
	}
}

// ── Review round: end of the session ────────────────────────────────────────

func TestDeliverSMTPTreatsAnAcceptedMessageAsSent(t *testing.T) {
	previous := smtpQuitTimeout
	smtpQuitTimeout = 300 * time.Millisecond
	t.Cleanup(func() { smtpQuitTimeout = previous })
	for _, implicitTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicitTLS=%v", implicitTLS), func(t *testing.T) {
			srv := c05StartFakeSMTP(t, implicitTLS, func(s *c05FakeSMTP) { s.hangAt = "QUIT" })
			send := SendEmail
			if implicitTLS {
				send = SendEmailTLS
			}
			start := time.Now()
			err := send(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "Betreff", "Text", c05Logger())
			if err != nil {
				t.Fatalf("a message the server accepted reported %v", err)
			}
			if elapsed := time.Since(start); elapsed > 10*time.Second {
				t.Fatalf("QUIT held the send for %v", elapsed)
			}
			if _, hung, mails := srv.snapshot(); hung != 1 || len(mails) != 1 {
				t.Fatalf("hung %d, %d messages", hung, len(mails))
			}
		})
	}
}

func TestDeliverSMTPWaitsForTheFinalReplyOnItsOwnDeadline(t *testing.T) {
	base, final := smtpSessionBaseTimeout, smtpFinalReplyTimeout
	t.Cleanup(func() { smtpSessionBaseTimeout, smtpFinalReplyTimeout = base, final })

	// A server that takes longer than the session deadline to accept the message is waited for.
	// The session up to the final dot (a TLS handshake and a few round trips, about 10 ms on
	// loopback) must fit into the session deadline: 5 s leaves 500 times that. The final reply
	// comes after it, and its own deadline is far beyond that.
	smtpSessionBaseTimeout, smtpFinalReplyTimeout = 5*time.Second, time.Minute
	srv := c05StartFakeSMTP(t, true, func(s *c05FakeSMTP) { s.finalReplyDelay = 6 * time.Second })
	if err := SendEmailTLS(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "Betreff", "Text", c05Logger()); err != nil {
		t.Fatalf("a slow final reply failed the send: %v", err)
	}

	// A server that never accepts it fails the send after smtpFinalReplyTimeout, not after
	// the longer session deadline.
	smtpSessionBaseTimeout, smtpFinalReplyTimeout = 2*time.Minute, time.Second
	for _, implicitTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicitTLS=%v", implicitTLS), func(t *testing.T) {
			srv := c05StartFakeSMTP(t, implicitTLS, func(s *c05FakeSMTP) { s.hangAt = "DOT" })
			send := SendEmail
			if implicitTLS {
				send = SendEmailTLS
			}
			start := time.Now()
			err := send(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "Betreff", "Text", c05Logger())
			if !errors.Is(err, os.ErrDeadlineExceeded) || !strings.Contains(err.Error(), "close failed") {
				t.Fatalf("error = %v, want a deadline error at the final dot", err)
			}
			if elapsed := time.Since(start); elapsed > 15*time.Second {
				t.Fatalf("the send took %v", elapsed)
			}
		})
	}
}

// ── Review round: bare line ends ────────────────────────────────────────────

func TestNormalizeEmailLineEndings(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"Text":                  "Text",
		"a\r\nb\r\n":            "a\r\nb\r\n",
		"a\nb":                  "a\r\nb",
		"a\rb":                  "a\r\nb",
		"\r":                    "\r\n",
		"\n":                    "\r\n",
		"\r\r\n":                "\r\n\r\n",
		"\n\r":                  "\r\n\r\n",
		"a\r.\r\nMAIL FROM:<x>": "a\r\n.\r\nMAIL FROM:<x>",
		"x\r\n.\ry\n\rz":        "x\r\n.\r\ny\r\n\r\nz",
		"ä\r\n🎉\r\x00\nend\r\n": "ä\r\n🎉\r\n\x00\r\nend\r\n",
	} {
		if got := normalizeEmailLineEndings(in); got != want {
			t.Errorf("normalizeEmailLineEndings(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmailBodyBareCRCannotEndTheData(t *testing.T) {
	body := "a\r.\r\nMAIL FROM:<x>"
	plain := buildEmailMessage("a@example.com", "b@example.com", "s", body, emailTestTime, nil, "")
	if !strings.HasSuffix(plain, "\r\n\r\na\r\n.\r\nMAIL FROM:<x>") {
		t.Fatalf("plain message = %q", plain)
	}
	attachments := []EmailAttachment{{Name: "a.txt", ContentType: "text/plain", Data: []byte("x")}}
	_, text, _ := c05ParseMessage(t, buildEmailMessage("a@example.com", "b@example.com", "s", "a\rb\nc", emailTestTime, attachments, "b0"))
	if text != "a\r\nb\r\nc" {
		t.Fatalf("text part = %q", text)
	}
	for _, implicitTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicitTLS=%v", implicitTLS), func(t *testing.T) {
			srv := c05StartFakeSMTP(t, implicitTLS, nil)
			send := SendEmail
			if implicitTLS {
				send = SendEmailTLS
			}
			if err := send(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "s", body, c05Logger()); err != nil {
				t.Fatal(err)
			}
			_, _, mails := srv.snapshot()
			if len(mails) != 1 {
				t.Fatalf("%d messages arrived", len(mails))
			}
			// The dot line is dot-stuffed on the wire, and no bare CR is left.
			if !strings.HasSuffix(mails[0].raw, "\r\n\r\na\r\n..\r\nMAIL FROM:<x>\r\n") || strings.Contains(mails[0].raw, "a\r.") {
				t.Fatalf("wire data = %q", mails[0].raw)
			}
			if !strings.HasSuffix(mails[0].data, "\r\n\r\na\r\n.\r\nMAIL FROM:<x>\r\n") {
				t.Fatalf("data = %q", mails[0].data)
			}
		})
	}
}

// c05ReplaceBareCR turns only the bare CRs of s into CRLF: the one way the wire bytes of a
// plain message differ from the old code. (A bare LF became CRLF on the wire before, too.)
func c05ReplaceBareCR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteByte(s[i])
		if s[i] == '\r' && (i+1 == len(s) || s[i+1] != '\n') {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

var c05DateHeader = regexp.MustCompile(`(?m)^Date: ([^\r\n]*)\r\n`)

func TestSendEmailPlainWireMatchesTheOldMessageExceptBareCR(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	alphabet := []string{"a", "ä", ".", "\r", "\n", "\r\n", " ", "-", "🎉", "\x00", "=", "?", "\t", "\xff"}
	gen := func(n int) string {
		var b strings.Builder
		for range n {
			b.WriteString(alphabet[rng.IntN(len(alphabet))])
		}
		return b.String()
	}
	for _, implicitTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicitTLS=%v", implicitTLS), func(t *testing.T) {
			srv := c05StartFakeSMTP(t, implicitTLS, nil)
			send := SendEmail
			if implicitTLS {
				send = SendEmailTLS
			}
			withoutBareCR := 0
			for i := range 100 {
				body := gen(rng.IntN(2000))
				switch i % 10 {
				case 0:
					body = strings.Repeat("x", 5000) + "\n." + body
				case 1:
					body = strings.ReplaceAll(body, "\r", "")
				}
				subject := gen(rng.IntN(120))
				from := []string{"", "a@example.com", "Jürgen <j@example.com>"}[rng.IntN(3)]
				to := []string{"b@example.com", "b@example.com, c@example.com", " b@example.com ,c@example.com "}[rng.IntN(3)]
				if err := send(srv.host, srv.port, "user", "pass", from, to, subject, body, c05Logger()); err != nil {
					t.Fatal(err)
				}
				_, _, mails := srv.snapshot()
				got := mails[len(mails)-1].data
				match := c05DateHeader.FindStringSubmatch(got)
				if match == nil {
					t.Fatalf("case %d: no Date header", i)
				}
				sent, err := time.Parse(time.RFC1123Z, match[1])
				if err != nil {
					t.Fatal(err)
				}
				if from == "" {
					from = "user"
				}
				want := c05WireData(t, c05LegacyPlainMessage(from, to, subject, c05ReplaceBareCR(body), sent))
				if got != want {
					t.Fatalf("case %d differs from the old message:\n%q\nwant\n%q", i, got, want)
				}
				if c05ReplaceBareCR(body) == body {
					withoutBareCR++
				}
			}
			if withoutBareCR < 10 {
				t.Fatalf("only %d bodies without a bare CR were compared unchanged", withoutBareCR)
			}
		})
	}
}

// ── Review round: attachments built outside LoadEmailAttachments ────────────

func TestSendEmailWithAttachmentsEnforcesTheLimits(t *testing.T) {
	srv := c05StartFakeSMTP(t, false, nil)
	logger := c05Logger()
	small := EmailAttachment{Name: "a.txt", ContentType: "text/plain", Data: []byte("x")}
	tooMany := slices.Repeat([]EmailAttachment{small}, emailMaxAttachments+1)
	if err := SendEmailWithAttachments(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "s", "b", tooMany, logger); err == nil {
		t.Fatal("more than 10 attachments were sent")
	}
	tooLarge := []EmailAttachment{
		{Name: "a.bin", ContentType: "application/octet-stream", Data: make([]byte, 10<<20)},
		{Name: "b.bin", ContentType: "application/octet-stream", Data: make([]byte, 10<<20+1)},
	}
	if err := SendEmailTLSWithAttachments(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "s", "b", tooLarge, logger); !errors.Is(err, errEmailAttachmentsTooLarge) {
		t.Fatalf("attachments over 20 MB together: %v", err)
	}
	if accepted, _, _ := srv.snapshot(); accepted != 0 {
		t.Fatalf("refused attachments reached the server: %d connections", accepted)
	}
	atLimit := slices.Repeat([]EmailAttachment{small}, emailMaxAttachments)
	if err := SendEmailWithAttachments(srv.host, srv.port, "user", "pass", "a@example.com", "b@example.com", "s", "b", atLimit, logger); err != nil {
		t.Fatalf("10 attachments: %v", err)
	}
	if _, _, mails := srv.snapshot(); len(mails) != 1 {
		t.Fatalf("%d messages arrived", len(mails))
	}
}
