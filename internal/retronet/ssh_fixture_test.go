package retronet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// sshWindow is a PTY size as the fixture saw it on the wire.
type sshWindow struct{ Cols, Rows int }

// sshFixtureAuth selects the anonymous login the fixture accepts.
type sshFixtureAuth int

const (
	sshFixtureKeyboardInteractive sshFixtureAuth = iota // keyboard-interactive with one empty answer only
	sshFixturePassword                                  // empty password only
)

// sshFixture is an in-process SSH service with anonymous login, a PTY and an echo shell.
// The shell greets on stdout and stderr; input containing "exit" makes it print "bye",
// send exit-status 0 and close the channel.
type sshFixture struct {
	host        string
	port        int
	fingerprint string
	keyType     string
	// ecdsaFingerprint is the second host key's fingerprint (startSSHFixtureKeys withECDSA).
	ecdsaFingerprint string
	listener         net.Listener
	wg               sync.WaitGroup

	mu      sync.Mutex
	conns   []net.Conn
	users   []string
	ptyTerm string
	ptySize sshWindow
	resizes []sshWindow
}

func startSSHFixture(t *testing.T, auth sshFixtureAuth) *sshFixture {
	t.Helper()
	return startSSHFixtureKeys(t, auth, false)
}

// startSSHFixtureKeys is startSSHFixture; with withECDSA it also offers an ECDSA P-256 host
// key. fingerprint and keyType always describe the ed25519 key, ecdsaFingerprint the other.
func startSSHFixtureKeys(t *testing.T, auth sshFixtureAuth, withECDSA bool) *sshFixture {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	f := &sshFixture{fingerprint: ssh.FingerprintSHA256(signer.PublicKey()), keyType: signer.PublicKey().Type()}
	config := &ssh.ServerConfig{}
	switch auth {
	case sshFixtureKeyboardInteractive:
		config.KeyboardInteractiveCallback = func(meta ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			answers, err := challenge("", "", []string{"Password: "}, []bool{false})
			if err != nil {
				return nil, err
			}
			if len(answers) != 1 || answers[0] != "" {
				return nil, errors.New("access denied")
			}
			f.recordUser(meta.User())
			return nil, nil
		}
	case sshFixturePassword:
		config.PasswordCallback = func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if len(password) != 0 {
				return nil, errors.New("access denied")
			}
			f.recordUser(meta.User())
			return nil, nil
		}
	}
	config.AddHostKey(signer)
	if withECDSA {
		ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		ecSigner, err := ssh.NewSignerFromKey(ecKey)
		if err != nil {
			t.Fatal(err)
		}
		f.ecdsaFingerprint = ssh.FingerprintSHA256(ecSigner.PublicKey())
		config.AddHostKey(ecSigner)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	f.host, f.port, f.listener = addr.IP.String(), addr.Port, listener
	f.wg.Add(1)
	go f.acceptLoop(config)
	t.Cleanup(f.close)
	return f
}

func (f *sshFixture) acceptLoop(config *ssh.ServerConfig) {
	defer f.wg.Done()
	for {
		raw, err := f.listener.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conns = append(f.conns, raw)
		f.mu.Unlock()
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			f.serveConn(raw, config)
		}()
	}
}

func (f *sshFixture) serveConn(raw net.Conn, config *ssh.ServerConfig) {
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
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			f.serveSession(channel, channelRequests)
		}()
	}
}

func (f *sshFixture) serveSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()
	for req := range requests {
		switch req.Type {
		case "pty-req":
			var pty struct {
				Term                      string
				Cols, Rows, Width, Height uint32
				Modes                     string
			}
			if err := ssh.Unmarshal(req.Payload, &pty); err != nil {
				_ = req.Reply(false, nil)
				continue
			}
			f.mu.Lock()
			f.ptyTerm, f.ptySize = pty.Term, sshWindow{Cols: int(pty.Cols), Rows: int(pty.Rows)}
			f.mu.Unlock()
			_ = req.Reply(true, nil)
		case "window-change":
			var change struct{ Cols, Rows, Width, Height uint32 }
			if err := ssh.Unmarshal(req.Payload, &change); err == nil {
				f.mu.Lock()
				f.resizes = append(f.resizes, sshWindow{Cols: int(change.Cols), Rows: int(change.Rows)})
				f.mu.Unlock()
			}
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
		case "shell":
			_ = req.Reply(true, nil)
			f.wg.Add(1)
			go func() {
				defer f.wg.Done()
				runSSHFixtureShell(channel)
			}()
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

// runSSHFixtureShell greets on stdout and stderr and echoes input until "exit".
func runSSHFixtureShell(channel ssh.Channel) {
	_, _ = io.WriteString(channel, "welcome to the fixture\r\n")
	_, _ = io.WriteString(channel.Stderr(), "stderr-line\r\n")
	buf := make([]byte, 1024)
	for {
		n, err := channel.Read(buf)
		if n > 0 {
			if bytes.Contains(buf[:n], []byte("exit")) {
				_, _ = io.WriteString(channel, "bye\r\n")
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				_ = channel.Close()
				return
			}
			_, _ = channel.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (f *sshFixture) recordUser(user string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users = append(f.users, user)
}

func (f *sshFixture) close() {
	_ = f.listener.Close()
	f.mu.Lock()
	for _, conn := range f.conns {
		_ = conn.Close()
	}
	f.mu.Unlock()
	f.wg.Wait()
}

func (f *sshFixture) address() string {
	return net.JoinHostPort(f.host, strconv.Itoa(f.port))
}

// entry returns an SSH entry for the fixture; own selects own-entry rules.
func (f *sshFixture) entry(own bool, hostKey string) Entry {
	e := Entry{ID: "sshfixture", Name: "SSH Fixture", Category: CategoryGames, Protocol: ProtocolSSH, Host: f.host, Port: f.port, User: "guest", HostKey: hostKey, Own: own}
	if own {
		e.ID, e.Category = "own-sshfixture01", CategoryOwn
	}
	return e
}

func (f *sshFixture) dial(t *testing.T) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp4", f.address())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func (f *sshFixture) loggedInUsers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.users)
}

func (f *sshFixture) pty() (string, sshWindow) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ptyTerm, f.ptySize
}

// waitResize waits until the fixture received a window-change to want.
func (f *sshFixture) waitResize(t *testing.T, want sshWindow) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		seen := slices.Contains(f.resizes, want)
		f.mu.Unlock()
		if seen {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t.Fatalf("window-change %+v not received; got %+v", want, f.resizes)
}
