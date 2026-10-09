package retronet

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// readSSHUntil reads from s until the output contains every string in want.
func readSSHUntil(t *testing.T, s *SSHSession, want ...string) string {
	t.Helper()
	got := make(chan string, 1)
	go func() {
		var acc []byte
		buf := make([]byte, 1024)
		for {
			n, err := s.Read(buf)
			acc = append(acc, buf[:n]...)
			if sshOutputHasAll(string(acc), want) || err != nil {
				got <- string(acc)
				return
			}
		}
	}()
	select {
	case text := <-got:
		if !sshOutputHasAll(text, want) {
			t.Fatalf("ssh output %q does not contain %q", text, want)
		}
		return text
	case <-time.After(5 * time.Second):
		_ = s.Close()
		t.Fatalf("timed out waiting for ssh output %q", want)
		return ""
	}
}

func sshOutputHasAll(text string, want []string) bool {
	for _, w := range want {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

func requireDialReason(t *testing.T, err error, want string) {
	t.Helper()
	var dialErr *DialError
	if !errors.As(err, &dialErr) || dialErr.Reason != want {
		t.Fatalf("error = %v, want *DialError with reason %q", err, want)
	}
}

func TestOpenSSHAnonymousKeyboardInteractiveLogin(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, fingerprint, err := OpenSSH(ctx, f.dial(t), f.entry(false, f.fingerprint), 100, 30, nil)
	if err != nil {
		t.Fatalf("OpenSSH: %v", err)
	}
	defer sess.Close()
	if fingerprint != f.fingerprint {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, f.fingerprint)
	}
	readSSHUntil(t, sess, "welcome to the fixture", "stderr-line")
	if users := f.loggedInUsers(); len(users) != 1 || users[0] != "guest" {
		t.Fatalf("logins = %q, want [guest]", users)
	}
	if term, size := f.pty(); term != "xterm-256color" || size != (sshWindow{Cols: 100, Rows: 30}) {
		t.Fatalf("pty = %q %+v, want xterm-256color {100 30}", term, size)
	}
	if _, err := sess.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	readSSHUntil(t, sess, "ping")
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestOpenSSHFallsBackToEmptyPassword(t *testing.T) {
	f := startSSHFixture(t, sshFixturePassword)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, _, err := OpenSSH(ctx, f.dial(t), f.entry(false, f.fingerprint), 80, 25, nil)
	if err != nil {
		t.Fatalf("OpenSSH: %v", err)
	}
	defer sess.Close()
	readSSHUntil(t, sess, "welcome to the fixture")
	if users := f.loggedInUsers(); len(users) != 1 || users[0] != "guest" {
		t.Fatalf("logins = %q, want [guest]", users)
	}
}

func TestOpenSSHHostKeyRules(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	wrongKey := "SHA256:" + strings.Repeat("A", 43)
	errPrompt := errors.New("prompt failed")
	type decision struct {
		accept bool
		err    error
	}
	cases := []struct {
		name       string
		own        bool
		hostKey    string
		decide     *decision
		wantReason string // empty: success
		wantErr    error
		wantAsked  bool
	}{
		{name: "catalog pinned match", hostKey: f.fingerprint},
		{name: "catalog pinned mismatch", hostKey: wrongKey, wantReason: ReasonHostKeyMismatch},
		{name: "catalog without pin", wantReason: ReasonHostKeyMismatch},
		{name: "own stored match", own: true, hostKey: f.fingerprint},
		{name: "own stored mismatch", own: true, hostKey: wrongKey, wantReason: ReasonHostKeyMismatch},
		{name: "own first contact accepted", own: true, decide: &decision{accept: true}, wantAsked: true},
		{name: "own first contact rejected", own: true, decide: &decision{accept: false}, wantReason: ReasonHostKeyRejected, wantAsked: true},
		{name: "own first contact without decider", own: true, wantReason: ReasonHostKeyRejected},
		{name: "own first contact decider error", own: true, decide: &decision{err: errPrompt}, wantErr: errPrompt, wantAsked: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var asked []string
			var decide HostKeyDecider
			if tc.decide != nil {
				decide = func(_ context.Context, keyType, fingerprint string) (bool, error) {
					mu.Lock()
					asked = append(asked, keyType+" "+fingerprint)
					mu.Unlock()
					return tc.decide.accept, tc.decide.err
				}
			}
			conn := f.dial(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sess, fingerprint, err := OpenSSH(ctx, conn, f.entry(tc.own, tc.hostKey), 80, 25, decide)
			switch {
			case tc.wantReason == "" && tc.wantErr == nil:
				if err != nil {
					t.Fatalf("OpenSSH: %v", err)
				}
				defer sess.Close()
				if fingerprint != f.fingerprint {
					t.Fatalf("fingerprint = %q, want %q", fingerprint, f.fingerprint)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
			default:
				requireDialReason(t, err, tc.wantReason)
			}
			if err != nil {
				if _, werr := conn.Write([]byte{0}); !errors.Is(werr, net.ErrClosed) {
					t.Fatalf("conn still open after failure: write error = %v", werr)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if tc.wantAsked {
				if want := f.keyType + " " + f.fingerprint; len(asked) != 1 || asked[0] != want {
					t.Fatalf("decider calls = %q, want [%q]", asked, want)
				}
			} else if len(asked) != 0 {
				t.Fatalf("decider called for %q: %q", tc.name, asked)
			}
		})
	}
}

func TestOpenSSHPrefersTheEd25519HostKey(t *testing.T) {
	f := startSSHFixtureKeys(t, sshFixtureKeyboardInteractive, true)
	if f.ecdsaFingerprint == "" || f.ecdsaFingerprint == f.fingerprint {
		t.Fatalf("fixture keys: ed25519 %q, ecdsa %q", f.fingerprint, f.ecdsaFingerprint)
	}
	var asked []string
	decide := func(_ context.Context, keyType, fingerprint string) (bool, error) {
		asked = append(asked, keyType+" "+fingerprint)
		return true, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, fingerprint, err := OpenSSH(ctx, f.dial(t), f.entry(true, ""), 80, 25, decide)
	if err != nil {
		t.Fatalf("OpenSSH: %v", err)
	}
	defer sess.Close()
	// The first-contact fingerprint must be the one ssh and known_hosts show for the server's
	// preferred key (ed25519), not whichever algorithm the library negotiates by default.
	if want := f.keyType + " " + f.fingerprint; len(asked) != 1 || asked[0] != want {
		t.Fatalf("decider asked about %q, want [%q]", asked, want)
	}
	if fingerprint != f.fingerprint {
		t.Fatalf("fingerprint = %q, want the ed25519 key %q", fingerprint, f.fingerprint)
	}
}

func TestSSHSessionResizeSendsWindowChange(t *testing.T) {
	f := startSSHFixture(t, sshFixtureKeyboardInteractive)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, _, err := OpenSSH(ctx, f.dial(t), f.entry(false, f.fingerprint), 80, 25, nil)
	if err != nil {
		t.Fatalf("OpenSSH: %v", err)
	}
	defer sess.Close()
	if err := sess.Resize(120, 40); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	f.waitResize(t, sshWindow{Cols: 120, Rows: 40})
}

func TestOpenSSHCancelsStalledHandshake(t *testing.T) {
	errStalledHandshake := errors.New("test cancelled the handshake")
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	conn, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	entry := Entry{ID: "own-stalled01", Protocol: ProtocolSSH, Host: "127.0.0.1", Port: port, User: "guest", Own: true}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	done := make(chan error, 1)
	go func() {
		_, _, err := OpenSSH(ctx, conn, entry, 80, 25, nil)
		done <- err
	}()
	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("no connection")
	}
	defer server.Close()
	// Wait for the client version line: the handshake is now waiting for a server that never answers.
	_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
	if line, err := bufio.NewReader(server).ReadString('\n'); err != nil || !strings.HasPrefix(line, "SSH-2.0-") {
		t.Fatalf("client version line = %q, %v", line, err)
	}
	cancel(errStalledHandshake)
	select {
	case err := <-done:
		if !errors.Is(err, errStalledHandshake) {
			t.Fatalf("error = %v, want the cancellation cause", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handshake did not stop after cancellation")
	}
}
