package mailer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
)

func TestIMAPRemoteIDsBindUIDValidityAndBodiesUsePeek(t *testing.T) {
	id := imapRemoteID(7, 12)
	uid, err := parseIMAPRemoteID(id, 7)
	if err != nil || uid != 12 {
		t.Fatalf("valid IMAP ID failed: uid=%d err=%v", uid, err)
	}
	if _, err := parseIMAPRemoteID(id, 8); err == nil {
		t.Fatal("stale UIDVALIDITY was accepted")
	}
	body := (&imap.BodySectionName{Peek: true}).FetchItem()
	headers := (&imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: imap.HeaderSpecifier, Fields: []string{"Reply-To", "Message-ID", "In-Reply-To", "References"}}}).FetchItem()
	if body != "BODY.PEEK[]" || headers != "BODY.PEEK[HEADER.FIELDS (Reply-To Message-ID In-Reply-To References)]" {
		t.Fatalf("IMAP fetches could mutate Seen state: body=%q headers=%q", body, headers)
	}
}

func TestIMAPRawFetchUsesPeekInTLSProtocolSession(t *testing.T) {
	serverTLS, clientTLS := smtpFixtureTLS(t)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	serverConn = tls.Server(serverConn, serverTLS)
	defer serverConn.Close()
	commands := make(chan string, 8)
	serverDone := make(chan error, 1)
	raw := []byte("From: sender@example.test\r\nReply-To: Support <support@example.test>\r\nTo: recipient@example.test\r\nSubject: fixture\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello\r\n")
	go func() {
		_ = serverConn.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := io.WriteString(serverConn, "* OK fixture IMAP ready\r\n"); err != nil {
			serverDone <- err
			return
		}
		reader := bufio.NewReader(serverConn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverDone <- err
				return
			}
			line = strings.TrimRight(line, "\r\n")
			parts := strings.SplitN(line, " ", 3)
			if len(parts) < 2 {
				serverDone <- fmt.Errorf("invalid IMAP command: %q", line)
				return
			}
			tag, command := parts[0], strings.ToUpper(parts[1])
			commands <- line
			switch command {
			case "LOGIN":
				_, err = fmt.Fprintf(serverConn, "%s OK LOGIN completed\r\n", tag)
			case "EXAMINE", "SELECT":
				_, err = io.WriteString(serverConn, "* FLAGS (\\Answered \\Flagged \\Deleted \\Seen \\Draft)\r\n* 1 EXISTS\r\n* 0 RECENT\r\n* OK [UIDVALIDITY 7] valid\r\n")
				if err == nil {
					_, err = fmt.Fprintf(serverConn, "%s OK [READ-ONLY] selected\r\n", tag)
				}
			case "UID":
				if len(parts) < 3 || !strings.Contains(strings.ToUpper(parts[2]), "FETCH") || !strings.Contains(strings.ToUpper(parts[2]), "BODY.PEEK[]") {
					_, err = fmt.Fprintf(serverConn, "%s BAD expected UID FETCH with BODY.PEEK[]\r\n", tag)
					break
				}
				_, err = fmt.Fprintf(serverConn, "* 1 FETCH (UID 12 BODY[] {%d}\r\n", len(raw))
				if err == nil {
					_, err = serverConn.Write(raw)
				}
				if err == nil {
					_, err = fmt.Fprintf(serverConn, ")\r\n%s OK FETCH completed\r\n", tag)
				}
			case "LOGOUT":
				_, _ = io.WriteString(serverConn, "* BYE logging out\r\n")
				_, err = fmt.Fprintf(serverConn, "%s OK LOGOUT completed\r\n", tag)
				serverDone <- err
				return
			default:
				_, err = fmt.Fprintf(serverConn, "%s BAD unsupported command\r\n", tag)
			}
			if err != nil {
				serverDone <- err
				return
			}
		}
	}()

	adapter := &imapAdapter{cfg: TransportConfig{ID: "fixture", Source: "email", IMAPHost: "127.0.0.1", ReceiveTLS: "tls", Username: "user", Password: "password", TLSConfig: clientTLS,
		DialContext: func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }}}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	got, err := adapter.Raw(ctx, "", "7:12")
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("IMAP PEEK fetch failed: err=%v body=%q", err, got)
	}
	message, err := adapter.messageFromRaw(raw, 12, "INBOX", nil, time.Time{})
	if err != nil || len(message.ReplyTo) != 1 || message.ReplyTo[0] != "support@example.test" {
		t.Fatalf("IMAP Reply-To was not parsed: %#v err=%v", message.ReplyTo, err)
	}
	if err := <-serverDone; err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if len(commands) < 3 {
		t.Fatalf("incomplete IMAP session commands: %v", commands)
	}
}
