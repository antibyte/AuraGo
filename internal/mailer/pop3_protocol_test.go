package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPOP3NoUIDLFallbackPreservesDuplicatesAfterUnrelatedRemoval(t *testing.T) {
	duplicate := []byte("From: sender@example.test\r\nReply-To: Support <support@example.test>\r\nSubject: same\r\n\r\nidentical body\r\n")
	other := []byte("From: sender@example.test\r\nSubject: other\r\n\r\nunrelated\r\n")
	first, commands, done := newPOP3Fixture(t, [][]byte{other, duplicate, duplicate}, false, nil)
	page, err := first.Sync(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 3 || !page.Messages[1].Unread || !page.Messages[2].Unread || page.Messages[1].RemoteID == page.Messages[2].RemoteID {
		t.Fatalf("POP3 no-UIDL import collapsed or marked duplicate messages read: %#v", page.Messages)
	}
	if len(page.Messages[1].ReplyTo) != 1 || page.Messages[1].ReplyTo[0] != "support@example.test" {
		t.Fatalf("POP3 Reply-To was not parsed: %#v", page.Messages[1].ReplyTo)
	}
	if err := <-done; err != nil && err != io.EOF {
		t.Fatal(err)
	}
	for _, command := range drainCommands(commands) {
		if strings.HasPrefix(strings.ToUpper(command), "DELE") {
			t.Fatal("POP3 adapter attempted server-side deletion")
		}
	}
	second, _, done2 := newPOP3Fixture(t, [][]byte{duplicate, duplicate}, false, nil)
	rescan, err := second.Sync(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done2; err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if len(rescan.Messages) != 2 || rescan.Messages[0].RemoteID != page.Messages[1].RemoteID || rescan.Messages[1].RemoteID != page.Messages[2].RemoteID {
		t.Fatalf("removing an unrelated earlier message changed duplicate IDs: old=%q,%q new=%#v", page.Messages[1].RemoteID, page.Messages[2].RemoteID, rescan.Messages)
	}
}

func TestPOP3UIDLIdentitiesRemainServerStable(t *testing.T) {
	messages := [][]byte{[]byte("From: a@example.test\r\n\r\none\r\n"), []byte("From: b@example.test\r\n\r\ntwo\r\n")}
	adapter, _, done := newPOP3Fixture(t, messages, true, nil)
	page, err := adapter.Sync(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.Messages[0].RemoteID != "uidl:stable-1" || page.Messages[1].RemoteID != "uidl:stable-2" {
		t.Fatalf("POP3 UIDLs were not used: %#v", page.Messages)
	}
	if err := <-done; err != nil && err != io.EOF {
		t.Fatal(err)
	}
}

func TestPOP3CancellationClosesBlockedTLSConnection(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	adapter, _, done := newPOP3Fixture(t, [][]byte{[]byte("From: sender@example.test\r\n\r\nbody\r\n")}, false, &pop3Block{started: started, release: release})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := adapter.Sync(ctx, "", "")
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("POP3 fixture did not reach the blocked RETR")
	}
	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("POP3 cancellation returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("POP3 sync did not stop when its context was canceled")
	}
	close(release)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("POP3 client kept the connection open after cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("POP3 TLS fixture did not observe connection closure")
	}
}

type pop3Block struct{ started, release chan struct{} }

func newPOP3Fixture(t *testing.T, messages [][]byte, uidl bool, block *pop3Block) (*pop3Adapter, <-chan string, <-chan error) {
	t.Helper()
	serverTLS, clientTLS := smtpFixtureTLS(t)
	commands := make(chan string, 64)
	done := make(chan error, 1)
	var once sync.Once
	adapter := &pop3Adapter{cfg: TransportConfig{ID: "fixture", Source: "email", POP3Host: "127.0.0.1", ReceiveTLS: "tls", Username: "user", Password: "pass", TLSConfig: clientTLS,
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			clientConn, serverConn := net.Pipe()
			serverConn = tls.Server(serverConn, serverTLS)
			go func() {
				defer serverConn.Close()
				_ = serverConn.SetDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.WriteString(serverConn, "+OK POP3 ready\r\n"); err != nil {
					done <- err
					return
				}
				reader := bufio.NewReader(serverConn)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						done <- err
						return
					}
					line = strings.TrimRight(line, "\r\n")
					commands <- line
					fields := strings.Fields(line)
					if len(fields) == 0 {
						continue
					}
					switch strings.ToUpper(fields[0]) {
					case "USER", "PASS":
						_, err = io.WriteString(serverConn, "+OK accepted\r\n")
					case "STAT":
						total := 0
						for _, raw := range messages {
							total += len(raw)
						}
						_, err = fmt.Fprintf(serverConn, "+OK %d %d\r\n", len(messages), total)
					case "UIDL":
						if !uidl {
							_, err = io.WriteString(serverConn, "-ERR not supported\r\n")
							break
						}
						_, err = io.WriteString(serverConn, "+OK unique IDs follow\r\n")
						for i := range messages {
							if err == nil {
								_, err = fmt.Fprintf(serverConn, "%d stable-%d\r\n", i+1, i+1)
							}
						}
						if err == nil {
							_, err = io.WriteString(serverConn, ".\r\n")
						}
					case "RETR":
						if len(fields) != 2 {
							_, err = io.WriteString(serverConn, "-ERR bad message number\r\n")
							break
						}
						n, parseErr := strconv.Atoi(fields[1])
						if parseErr != nil || n < 1 || n > len(messages) {
							_, err = io.WriteString(serverConn, "-ERR no message\r\n")
							break
						}
						_, err = io.WriteString(serverConn, "+OK message follows\r\n")
						if block != nil {
							once.Do(func() { close(block.started) })
							<-block.release
							_, writeErr := writePOP3DotMessage(serverConn, messages[n-1])
							done <- writeErr
							return
						}
						if err == nil {
							_, err = writePOP3DotMessage(serverConn, messages[n-1])
						}
					default:
						_, err = io.WriteString(serverConn, "-ERR unsupported\r\n")
					}
					if err != nil {
						done <- err
						return
					}
				}
			}()
			return clientConn, nil
		}}}
	return adapter, commands, done
}

func writePOP3DotMessage(w io.Writer, raw []byte) (int, error) {
	lines := strings.SplitAfter(string(raw), "\n")
	written := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ".") {
			line = "." + line
		}
		n, err := io.WriteString(w, line)
		written += n
		if err != nil {
			return written, err
		}
	}
	n, err := io.WriteString(w, ".\r\n")
	return written + n, err
}

func drainCommands(commands <-chan string) []string {
	var out []string
	for {
		select {
		case command := <-commands:
			out = append(out, command)
		default:
			return out
		}
	}
}
