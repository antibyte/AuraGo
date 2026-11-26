package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/agentmail"
)

func TestPlainTextDraftDeclaresUTF8AndRoundTrips(t *testing.T) {
	raw, err := BuildMIME("from@example.test", Draft{To: []string{"to@example.test"}, Subject: "s", Text: "Grüße ✓"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	head := strings.ToLower(string(raw[:strings.Index(string(raw), "\r\n\r\n")]))
	if !strings.Contains(head, "content-type: text/plain; charset=utf-8") {
		t.Fatalf("missing content type in headers: %q", head)
	}
	_, _, _, _, text, _, _, _, err := ParseMIME(raw, 0)
	if err != nil || strings.TrimSpace(text) != "Grüße ✓" {
		t.Fatalf("round trip text=%q err=%v", text, err)
	}
}

func TestParseMIMEToleratesUnknownCharsetPart(t *testing.T) {
	raw := "From: a@example.test\r\nTo: b@example.test\r\nSubject: x\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=B\r\n\r\n" +
		"--B\r\nContent-Type: text/plain; charset=x-unknown-foo\r\n\r\nodd part\r\n" +
		"--B\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>kept</p>\r\n--B--\r\n"
	_, _, _, _, text, html, _, _, err := ParseMIME([]byte(raw), 0)
	if err != nil {
		t.Fatalf("unknown charset aborted parsing: %v", err)
	}
	if !strings.Contains(text, "odd part") || !strings.Contains(html, "kept") {
		t.Fatalf("parts lost: text=%q html=%q", text, html)
	}
}

func TestSafeContentTypeAndAgentMailAttachmentGuard(t *testing.T) {
	for _, in := range []string{"", "  ", "text/plain\r\nBcc: x@example.test"} {
		if got := safeContentType(in); got != "application/octet-stream" {
			t.Fatalf("safeContentType(%q)=%q", in, got)
		}
	}
	if got := safeContentType(" image/png "); got != "image/png" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentMailMessageMarksSkippedHTMLAndFoldsDisposition(t *testing.T) {
	raw := agentmail.Message{ID: "m1", HTML: strings.Repeat("a", maxMailHTMLBytes+1), Text: "plain",
		Attachments: []agentmail.Attachment{{ID: "a1", Filename: "x.png", ContentDisposition: "Inline"}}}
	msg := agentMailMessage(TransportConfig{Source: "agentmail", ID: "acc"}, raw)
	if !msg.HTMLSkipped || msg.HTML != "" || msg.Text != "plain" {
		t.Fatalf("HTMLSkipped=%v html=%d text=%q", msg.HTMLSkipped, len(msg.HTML), msg.Text)
	}
	if len(msg.Attachments) != 1 || !msg.Attachments[0].Inline {
		t.Fatalf("inline disposition not matched case-insensitively: %+v", msg.Attachments)
	}
}

func TestSafeFilenameTruncatesOnRuneBoundary(t *testing.T) {
	name := strings.Repeat("a", 254) + "ü" + "tail" // ü straddles byte 255
	got := safeFilename(name)
	if len(got) > 255 || !utf8.ValidString(got) {
		t.Fatalf("len=%d valid=%v", len(got), utf8.ValidString(got))
	}
}

func TestPOP3AuthErrorClassification(t *testing.T) {
	rejected := pop3AuthError("POP3 authentication failed", errPOP3Rejected)
	if !errors.Is(rejected, ErrMailAuthentication) || !errors.Is(rejected, errPOP3Rejected) {
		t.Fatalf("credential rejection lost marker or cause: %v", rejected)
	}
	ioErr := pop3AuthError("POP3 authentication failed", errors.New("connection reset"))
	if errors.Is(ioErr, ErrMailAuthentication) {
		t.Fatalf("transport error marked as authentication failure: %v", ioErr)
	}
}

func TestPOP3ConnectRejectedPasswordKeepsAuthenticationMarker(t *testing.T) {
	for _, mode := range []string{"reject", "drop"} {
		t.Run(mode, func(t *testing.T) {
			serverTLS, clientTLS := smtpFixtureTLS(t)
			clientConn, serverRaw := net.Pipe()
			defer clientConn.Close()
			defer serverRaw.Close()
			go func() {
				conn := tls.Server(serverRaw, serverTLS)
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				_, _ = io.WriteString(conn, "+OK ready\r\n")
				r := textproto.NewReader(bufio.NewReader(conn))
				for {
					line, err := r.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "USER "):
						_, _ = io.WriteString(conn, "+OK\r\n")
					case strings.HasPrefix(line, "PASS "):
						if mode == "drop" {
							return
						}
						_, _ = io.WriteString(conn, "-ERR bad password\r\n")
					}
				}
			}()
			a := &pop3Adapter{cfg: TransportConfig{ID: "p", Source: "email", POP3Host: "127.0.0.1", POP3Port: 995, Username: "u", Password: "secret-pass", TLSConfig: clientTLS,
				DialContext: func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }}}
			err := a.Test(context.Background())
			if err == nil || strings.Contains(err.Error(), "secret-pass") {
				t.Fatalf("err=%v", err)
			}
			if got := errors.Is(err, ErrMailAuthentication); got != (mode == "reject") {
				t.Fatalf("mode %s: ErrMailAuthentication=%v err=%v", mode, got, err)
			}
		})
	}
}

func TestTestSMTPAuthKeepsProtocolCause(t *testing.T) {
	serverTLS, clientTLS := smtpFixtureTLS(t)
	clientConn, serverRaw := net.Pipe()
	defer clientConn.Close()
	defer serverRaw.Close()
	go func() {
		conn := tls.Server(serverRaw, serverTLS)
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = io.WriteString(conn, "220 fixture\r\n")
		r := textproto.NewReader(bufio.NewReader(conn))
		for {
			line, err := r.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(strings.ToUpper(line), "EHLO"):
				_, _ = io.WriteString(conn, "250-fixture\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(strings.ToUpper(line), "AUTH"):
				_, _ = io.WriteString(conn, "535 5.7.8 rejected\r\n")
			default:
				_, _ = io.WriteString(conn, "221 bye\r\n")
				return
			}
		}
	}()
	cfg := TransportConfig{ID: "s", Source: "email", SMTPHost: "127.0.0.1", SMTPPort: 465, SMTPTransport: "tls", Username: "u", SMTPPassword: "pw", TLSConfig: clientTLS,
		DialContext: func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }}
	err := TestSMTPAuth(context.Background(), cfg)
	var pe *textproto.Error
	if !errors.As(err, &pe) || pe.Code != 535 {
		t.Fatalf("protocol cause not preserved: %v", err)
	}
}

func TestSMTPDataDeadlineScalesWithSize(t *testing.T) {
	if smtpDataDeadline(0) != 60*time.Second || smtpDataDeadline(32<<20) <= smtpDataDeadline(1<<20) {
		t.Fatalf("deadline does not scale: %v %v", smtpDataDeadline(0), smtpDataDeadline(32<<20))
	}
}
