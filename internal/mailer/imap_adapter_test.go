package mailer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap"
)

func TestIMAPMetadataPreviewAndAttachmentCountStayOutOfTheBody(t *testing.T) {
	section := &imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: imap.TextSpecifier}, Partial: []int{0, 2048}}
	if section.FetchItem() != "BODY.PEEK[TEXT]<0.2048>" {
		t.Fatalf("snippet fetch = %q", section.FetchItem())
	}
	count := imapAttachmentCount(&imap.BodyStructure{MIMEType: "multipart", MIMESubType: "mixed", Parts: []*imap.BodyStructure{
		{MIMEType: "text", MIMESubType: "plain"},
		{MIMEType: "image", MIMESubType: "png", Disposition: "inline"},
		{MIMEType: "application", MIMESubType: "pdf", Disposition: "attachment"},
	}})
	if count != 1 {
		t.Fatalf("attachment count = %d", count)
	}
	snippet := imapListSnippet([]byte("Content-Type: text/plain; charset=utf-8\r\n\r\nHello <b>there</b> friend"))
	if snippet != "Hello there friend" {
		t.Fatalf("snippet = %q", snippet)
	}
	plain := imapListSnippet([]byte("See <https://example.com/a> and <ada@example.test> today. 5 > 3"))
	if plain != "See <https://example.com/a> and <ada@example.test> today. 5 > 3" {
		t.Fatalf("plain snippet = %q", plain)
	}
	set, cursorUID, exhausted, err := imapSearchPageLimit("9")
	if err != nil || exhausted || cursorUID != 9 || set == nil || set.String() != "1:8" {
		t.Fatalf("page limit = %v uid=%d exhausted=%v err=%v", set, cursorUID, exhausted, err)
	}
	if _, _, exhausted, err = imapSearchPageLimit("1"); err != nil || !exhausted {
		t.Fatalf("cursor 1 = exhausted %v err %v", exhausted, err)
	}
	if _, _, _, err = imapSearchPageLimit("nope"); err == nil {
		t.Fatal("invalid cursor was accepted")
	}
	if messagePreview(Message{Snippet: "from snippet", Text: "from text", Subject: "subject"}) != "from snippet" {
		t.Fatal("preview ignored the snippet")
	}
	if messagePreview(Message{Text: "from text", Subject: "subject"}) != "from text" || messagePreview(Message{Subject: "subject"}) != "subject" {
		t.Fatal("preview fallback order changed")
	}
}

func TestIMAPSyncDropsBrokenSessionAndDialsAgain(t *testing.T) {
	serverTLS, clientTLS := smtpFixtureTLS(t)
	firstClient, firstServer := net.Pipe()
	secondClient, secondServer := net.Pipe()
	defer firstClient.Close()
	defer secondClient.Close()
	firstServer = tls.Server(firstServer, serverTLS)
	secondServer = tls.Server(secondServer, serverTLS)
	defer firstServer.Close()
	defer secondServer.Close()
	serve := func(conn net.Conn, failSelect bool, done chan error) {
		go func() {
			_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
			if _, err := io.WriteString(conn, "* OK [CAPABILITY IMAP4rev1] fixture IMAP ready\r\n"); err != nil {
				done <- err
				return
			}
			reader := bufio.NewReader(conn)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					done <- err
					return
				}
				line = strings.TrimRight(line, "\r\n")
				parts := strings.SplitN(line, " ", 3)
				if len(parts) < 2 {
					done <- fmt.Errorf("invalid IMAP command: %q", line)
					return
				}
				tag, command := parts[0], strings.ToUpper(parts[1])
				switch command {
				case "CAPABILITY":
					_, err = fmt.Fprintf(conn, "* CAPABILITY IMAP4rev1\r\n%s OK CAPABILITY completed\r\n", tag)
				case "LOGIN":
					_, err = fmt.Fprintf(conn, "%s OK [CAPABILITY IMAP4rev1] LOGIN completed\r\n", tag)
				case "LIST":
					_, err = io.WriteString(conn, "* LIST () \"/\" INBOX\r\n")
					if err == nil {
						_, err = fmt.Fprintf(conn, "%s OK LIST completed\r\n", tag)
					}
				case "EXAMINE", "SELECT":
					if failSelect {
						_, err = fmt.Fprintf(conn, "%s NO select failed\r\n", tag)
					} else {
						_, err = io.WriteString(conn, "* FLAGS (\\Seen)\r\n* 0 EXISTS\r\n* 0 RECENT\r\n* OK [UIDVALIDITY 3] valid\r\n")
						if err == nil {
							_, err = fmt.Fprintf(conn, "%s OK [READ-ONLY] selected\r\n", tag)
						}
					}
				case "UID":
					_, err = fmt.Fprintf(conn, "* SEARCH\r\n%s OK SEARCH completed\r\n", tag)
				case "LOGOUT":
					_, _ = io.WriteString(conn, "* BYE logging out\r\n")
					_, err = fmt.Fprintf(conn, "%s OK LOGOUT completed\r\n", tag)
					done <- err
					return
				default:
					_, err = fmt.Fprintf(conn, "%s OK completed\r\n", tag)
				}
				if err != nil {
					done <- err
					return
				}
			}
		}()
	}
	done1 := make(chan error, 1)
	done2 := make(chan error, 1)
	serve(firstServer, true, done1)
	serve(secondServer, false, done2)
	var dials int
	var dialMu sync.Mutex
	adapter := &imapAdapter{cfg: TransportConfig{ID: "fixture", Source: "email", IMAPHost: "127.0.0.1", ReceiveTLS: "tls", Username: "user", Password: "password", TLSConfig: clientTLS,
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialMu.Lock()
			defer dialMu.Unlock()
			dials++
			switch dials {
			case 1:
				return firstClient, nil
			case 2:
				return secondClient, nil
			default:
				return nil, errors.New("unexpected IMAP dial")
			}
		}}}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	stop, err := adapter.StartSyncSession(ctx)
	if err != nil {
		t.Fatalf("start sync session: %v", err)
	}
	defer stop()
	if _, err := adapter.Sync(ctx, "INBOX", ""); err == nil {
		t.Fatal("broken select succeeded")
	}
	if adapter.currentSyncSession() != nil {
		t.Fatal("broken sync session was kept")
	}
	folders, err := adapter.Folders(ctx)
	if err != nil {
		t.Fatalf("folders after reconnect: %v", err)
	}
	if len(folders) != 1 || folders[0].Name != "INBOX" {
		t.Fatalf("folders = %#v", folders)
	}
	if err := <-done2; err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	dialMu.Lock()
	gotDials := dials
	dialMu.Unlock()
	if gotDials != 2 {
		t.Fatalf("IMAP dials = %d", gotDials)
	}
}

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

func TestIMAPSyncSearchesOnlyNewUIDsAndReusesOneSession(t *testing.T) {
	serverTLS, clientTLS := smtpFixtureTLS(t)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	serverConn = tls.Server(serverConn, serverTLS)
	defer serverConn.Close()
	var commandMu sync.Mutex
	var commands []string
	serverDone := make(chan error, 1)
	go func() {
		_ = serverConn.SetDeadline(time.Now().Add(15 * time.Second))
		if _, err := io.WriteString(serverConn, "* OK [CAPABILITY IMAP4rev1] fixture IMAP ready\r\n"); err != nil {
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
			commandMu.Lock()
			commands = append(commands, line)
			commandMu.Unlock()
			tag, command := parts[0], strings.ToUpper(parts[1])
			var errWrite error
			switch command {
			case "CAPABILITY":
				_, errWrite = fmt.Fprintf(serverConn, "* CAPABILITY IMAP4rev1\r\n%s OK CAPABILITY completed\r\n", tag)
			case "LOGIN":
				_, errWrite = fmt.Fprintf(serverConn, "%s OK [CAPABILITY IMAP4rev1] LOGIN completed\r\n", tag)
			case "LIST":
				_, errWrite = io.WriteString(serverConn, "* LIST () \"/\" INBOX\r\n* LIST () \"/\" Archive\r\n")
				if errWrite == nil {
					_, errWrite = fmt.Fprintf(serverConn, "%s OK LIST completed\r\n", tag)
				}
			case "EXAMINE", "SELECT":
				_, errWrite = io.WriteString(serverConn, "* FLAGS (\\Answered \\Flagged \\Deleted \\Seen \\Draft)\r\n* 1 EXISTS\r\n* 0 RECENT\r\n* OK [UIDVALIDITY 7] valid\r\n")
				if errWrite == nil {
					_, errWrite = fmt.Fprintf(serverConn, "%s OK [READ-ONLY] selected\r\n", tag)
				}
			case "UID":
				_, errWrite = fmt.Fprintf(serverConn, "* SEARCH\r\n%s OK SEARCH completed\r\n", tag)
			case "LOGOUT":
				_, _ = io.WriteString(serverConn, "* BYE logging out\r\n")
				_, errWrite = fmt.Fprintf(serverConn, "%s OK LOGOUT completed\r\n", tag)
				serverDone <- errWrite
				return
			default:
				_, errWrite = fmt.Fprintf(serverConn, "%s OK completed\r\n", tag)
			}
			if errWrite != nil {
				serverDone <- errWrite
				return
			}
		}
	}()

	var dials int
	var dialMu sync.Mutex
	adapter := &imapAdapter{cfg: TransportConfig{ID: "fixture", Source: "email", IMAPHost: "127.0.0.1", ReceiveTLS: "tls", Username: "user", Password: "password", TLSConfig: clientTLS,
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			dialMu.Lock()
			defer dialMu.Unlock()
			dials++
			if dials != 1 {
				return nil, errors.New("second IMAP dial")
			}
			return clientConn, nil
		}}}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	stop, err := adapter.StartSyncSession(ctx)
	if err != nil {
		t.Fatalf("start sync session: %v", err)
	}
	folders, err := adapter.Folders(ctx)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("folders = %#v", folders)
	}
	cursorData, _ := json.Marshal(imapCursor{Validity: 7, LastUID: 12})
	cursor := base64.RawURLEncoding.EncodeToString(cursorData)
	incremental, err := adapter.Sync(ctx, folders[0].ID, cursor)
	if err != nil {
		t.Fatalf("incremental sync: %v", err)
	}
	if !incremental.RemoteSnapshotComplete || incremental.HasMore || len(incremental.Messages) != 0 || incremental.NextCursor != cursor {
		t.Fatalf("incremental result = %+v", incremental)
	}
	full, err := adapter.Sync(ctx, folders[1].ID, "")
	if err != nil {
		t.Fatalf("full sync: %v", err)
	}
	if !full.RemoteSnapshotComplete || full.HasMore || full.ResetSnapshot {
		t.Fatalf("full result = %+v", full)
	}
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(full.NextCursor)
	var stored imapCursor
	if decodeErr != nil || json.Unmarshal(decoded, &stored) != nil || stored.Validity != 7 || stored.LastUID != 0 {
		t.Fatalf("empty mailbox cursor = %q (%v)", full.NextCursor, decodeErr)
	}
	stop()
	if err := <-serverDone; err != nil && !errors.Is(err, io.ErrClosedPipe) && !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	dialMu.Lock()
	gotDials := dials
	dialMu.Unlock()
	if gotDials != 1 {
		t.Fatalf("IMAP dials = %d", gotDials)
	}
	commandMu.Lock()
	got := append([]string(nil), commands...)
	commandMu.Unlock()
	logins := 0
	ranged := 0
	fullSearches := 0
	for _, line := range got {
		upper := strings.ToUpper(line)
		parts := strings.SplitN(upper, " ", 3)
		if len(parts) >= 2 && parts[1] == "LOGIN" {
			logins++
		}
		if strings.Contains(upper, "SEARCH") && strings.Contains(upper, "13:*") {
			ranged++
		}
		if strings.Contains(upper, "SEARCH") && !strings.Contains(upper, "13:*") {
			fullSearches++
		}
	}
	if logins != 1 || ranged != 1 || fullSearches != 1 {
		t.Fatalf("LOGIN=%d ranged=%d full=%d commands=%v", logins, ranged, fullSearches, got)
	}
}
