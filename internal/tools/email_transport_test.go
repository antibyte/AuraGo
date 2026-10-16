package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"aurago/internal/mailer"
)

func TestSendEmailConfiguredFallsBackWhenFromAddressIsMissing(t *testing.T) {
	serverTLS, roots := c05TLSMaterial(t)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	fixtureDone := make(chan smtpTransportFixtureResult, 1)
	go serveSMTPTransportFixture(serverConn, serverTLS, fixtureDone)

	cfg := mailer.TransportConfig{
		SMTPHost: "127.0.0.1", SMTPPort: 587, SMTPTransport: "starttls",
		Username: "sender@example.test", Password: "fixture-password",
		TLSConfig:   &tls.Config{RootCAs: roots},
		DialContext: func(context.Context, string, string) (net.Conn, error) { return clientConn, nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := SendEmailConfigured(ctx, cfg, "to@example.test", "subject", "body", nil, nil)
	if err != nil {
		_ = clientConn.Close()
		t.Fatalf("send with missing FromAddress: %v", err)
	}
	fixture := <-fixtureDone
	if fixture.err != nil {
		t.Fatalf("SMTP fixture: %v", fixture.err)
	}
	if fixture.mailFrom != "MAIL FROM:<sender@example.test>" {
		t.Errorf("MAIL FROM = %q, want username fallback", fixture.mailFrom)
	}
	message, err := mail.ReadMessage(bytes.NewReader(fixture.data))
	if err != nil {
		t.Fatalf("parse transmitted MIME: %v", err)
	}
	got, err := mail.ParseAddress(message.Header.Get("From"))
	if err != nil || got.Address != "sender@example.test" {
		t.Errorf("From header = %q, want username fallback (parsed=%v, err=%v)", message.Header.Get("From"), got, err)
	}
}

type smtpTransportFixtureResult struct {
	mailFrom string
	data     []byte
	err      error
}

func serveSMTPTransportFixture(conn net.Conn, serverTLS *tls.Config, done chan<- smtpTransportFixtureResult) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	finish := func(result smtpTransportFixtureResult) { done <- result }
	reply := func(c net.Conn, lines ...string) error {
		for _, line := range lines {
			if _, err := io.WriteString(c, line+"\r\n"); err != nil {
				return err
			}
		}
		return nil
	}
	if err := reply(conn, "220 fixture ESMTP"); err != nil {
		finish(smtpTransportFixtureResult{err: err})
		return
	}
	secure := false
	mailFrom := ""
	var capturedData []byte
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			finish(smtpTransportFixtureResult{mailFrom: mailFrom, data: capturedData, err: err})
			return
		}
		command := strings.TrimRight(line, "\r\n")
		verb, _, _ := strings.Cut(command, " ")
		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			if secure {
				err = reply(conn, "250-fixture", "250 AUTH PLAIN")
			} else {
				err = reply(conn, "250-fixture", "250-STARTTLS", "250 AUTH PLAIN")
			}
		case "STARTTLS":
			err = reply(conn, "220 ready")
			if err == nil {
				tlsConn := tls.Server(conn, serverTLS)
				err = tlsConn.Handshake()
				if err == nil {
					conn, secure = tlsConn, true
					reader = bufio.NewReader(conn)
				}
			}
		case "AUTH":
			if len(strings.Fields(command)) == 2 {
				err = reply(conn, "334 ")
				if err == nil {
					_, err = reader.ReadString('\n')
				}
			}
			if err == nil {
				err = reply(conn, "235 accepted")
			}
		case "MAIL":
			mailFrom = command
			err = reply(conn, "250 sender ok")
		case "RCPT":
			err = reply(conn, "250 recipient ok")
		case "DATA":
			err = reply(conn, "354 send data")
			if err == nil {
				var data bytes.Buffer
				for {
					dataLine, readErr := reader.ReadString('\n')
					if readErr != nil {
						finish(smtpTransportFixtureResult{mailFrom: mailFrom, err: readErr})
						return
					}
					if dataLine == ".\r\n" {
						break
					}
					if strings.HasPrefix(dataLine, "..") {
						dataLine = dataLine[1:]
					}
					data.WriteString(dataLine)
				}
				capturedData = append([]byte(nil), data.Bytes()...)
				err = reply(conn, "250 queued")
			}
		case "QUIT":
			err = reply(conn, "221 bye")
			finish(smtpTransportFixtureResult{mailFrom: mailFrom, data: capturedData, err: err})
			return
		default:
			err = fmt.Errorf("unexpected SMTP command %q", command)
		}
		if err != nil {
			finish(smtpTransportFixtureResult{mailFrom: mailFrom, data: capturedData, err: err})
			return
		}
	}
}

func TestSendEmailConfiguredRejectsHeaderInjectionInFallbackSender(t *testing.T) {
	dials := 0
	cfg := mailer.TransportConfig{SMTPHost: "smtp.example.test", SMTPTransport: "plain", SMTPUsername: "sender@example.test\r\nBcc: victim@example.test",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dials++
			return nil, nil
		}}
	err := SendEmailConfigured(context.Background(), cfg, "to@example.test", "subject", "body", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "sender address contains a line break") {
		t.Fatalf("header injection in fallback sender was not rejected: %v", err)
	}
	if dials != 0 {
		t.Fatalf("invalid sender opened %d connections", dials)
	}
}
