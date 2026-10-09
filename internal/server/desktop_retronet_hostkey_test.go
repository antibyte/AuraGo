package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// retroNetFakeSSH is an anonymous SSH service with a PTY and a greeting shell.
type retroNetFakeSSH struct {
	listener    net.Listener
	fingerprint string
	mu          sync.Mutex
	conns       []net.Conn
}

func startRetroNetFakeSSH(t *testing.T) *retroNetFakeSSH {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fake := &retroNetFakeSSH{listener: listener, fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}
	go fake.serve(config)
	t.Cleanup(fake.close)
	return fake
}

func (f *retroNetFakeSSH) serve(config *ssh.ServerConfig) {
	for {
		raw, err := f.listener.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conns = append(f.conns, raw)
		f.mu.Unlock()
		go f.serveConn(raw, config)
	}
}

func (f *retroNetFakeSSH) serveConn(raw net.Conn, config *ssh.ServerConfig) {
	defer raw.Close()
	conn, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range channelRequests {
				switch req.Type {
				case "pty-req", "window-change":
					if req.WantReply {
						_ = req.Reply(true, nil)
					}
				case "shell":
					_ = req.Reply(true, nil)
					go func() {
						_, _ = io.WriteString(channel, "WELCOME SSH\r\n")
						_, _ = io.Copy(io.Discard, channel)
					}()
				default:
					if req.WantReply {
						_ = req.Reply(false, nil)
					}
				}
			}
		}()
	}
}

func (f *retroNetFakeSSH) port() int { return f.listener.Addr().(*net.TCPAddr).Port }

func (f *retroNetFakeSSH) close() {
	_ = f.listener.Close()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, conn := range f.conns {
		_ = conn.Close()
	}
}

// Any write-scope user may accept an unknown key for their own session, but only
// an administrator's acceptance pins it for everyone.
func TestDesktopRetroNetHostKeyPersistsOnlyForAdmins(t *testing.T) {
	service := startRetroNetFakeSSH(t)
	env := newRetroNetTestEnv(t, func(s *Server) {
		s.retroNetManager.Dialer = retroNetTestNetwork{port: service.port()}.dialer()
	})
	const entryID = "own-sshgame01"
	env.saveEntries(t, fmt.Sprintf(`{"id":%q,"name":"SSH Game","protocol":"ssh","host":"game.retronet.test","port":%d,"user":"guest"}`, entryID, service.port()))
	storedKey := func() string {
		entries := env.directory(t, env.adminToken).Entries
		last := entries[len(entries)-1]
		if last.ID != entryID {
			t.Fatalf("last directory entry = %+v, want %s", last, entryID)
		}
		return last.HostKey
	}
	for _, tc := range []struct {
		name, token string
		persisted   bool
	}{
		{"write token", env.writeToken, false},
		{"admin", env.adminToken, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			socket := dialRetroNet(t, env.httpServer.URL, tc.token, "entry="+entryID)
			socket.until(5*time.Second, socket.has("hostkey_prompt"))
			if prompt, _ := socket.control("hostkey_prompt"); prompt.Fingerprint != service.fingerprint {
				t.Fatalf("hostkey_prompt = %+v, want fingerprint %s", prompt, service.fingerprint)
			}
			socket.send(websocket.TextMessage, `{"type":"hostkey_decision","accept":true}`)
			socket.until(5*time.Second, func() bool {
				_, connected := socket.control("connected")
				return connected && strings.Contains(socket.data.String(), "WELCOME SSH")
			})
			got := storedKey()
			if tc.persisted && got != service.fingerprint {
				t.Fatalf("stored host key = %q, want %q after an admin accepted it", got, service.fingerprint)
			}
			if !tc.persisted && got != "" {
				t.Fatalf("a non-admin acceptance pinned host key %q", got)
			}
			_ = socket.conn.Close()
		})
	}
}
