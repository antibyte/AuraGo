package retronet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	sshTermType      = "xterm-256color"
	sshDefaultUser   = "guest"
	sshTerminalSpeed = 14400
)

// sshHandshakeTimeout bounds the SSH handshake plus PTY and shell setup. It is a variable
// only so tests can shorten it.
var sshHandshakeTimeout = 10 * time.Second

// sshHostKeyAlgorithms is the host key algorithm preference offered to servers, in the order
// OpenSSH prefers plain keys. x/crypto's default order puts ECDSA before ed25519, so a server
// with both keys would negotiate ECDSA and the stored or first-contact fingerprint would
// differ from what ssh and known_hosts show for the same server. Certificate algorithms are
// not offered: host certificates are not supported.
var sshHostKeyAlgorithms = []string{
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoECDSA256,
	ssh.KeyAlgoECDSA384,
	ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA512,
	ssh.KeyAlgoRSASHA256,
	ssh.KeyAlgoRSA,
}

// HostKeyDecider is asked on first contact with an own SSH entry that has no stored key.
type HostKeyDecider func(ctx context.Context, keyType, fingerprint string) (bool, error)

// SSHSession is the PTY shell of one anonymous SSH connection.
type SSHSession struct {
	client    *ssh.Client
	session   *ssh.Session
	stdin     io.WriteCloser
	output    *io.PipeReader
	waitDone  chan struct{}
	closeOnce sync.Once
}

// Read returns PTY output; stdout and stderr are merged into one stream.
func (s *SSHSession) Read(p []byte) (int, error) { return s.output.Read(p) }

// Write sends keystrokes to the PTY.
func (s *SSHSession) Write(p []byte) (int, error) { return s.stdin.Write(p) }

// Close ends the shell and the connection. It is safe to call more than once and returns
// after the goroutine that feeds Read has stopped.
func (s *SSHSession) Close() error {
	s.closeOnce.Do(func() {
		_ = s.output.Close()
		_ = s.session.Close()
		_ = s.client.Close()
		<-s.waitDone
	})
	return nil
}

// Resize sends a window-change request.
func (s *SSHSession) Resize(cols, rows int) error {
	return s.session.WindowChange(rows, cols)
}

// OpenSSH performs the handshake over an already dialed conn. Auth: "none" via an empty
// method list fallback to keyboard-interactive with empty answers. PTY "xterm-256color".
// Catalog entries (e.Own == false) require e.HostKey to match, else *DialError{ReasonHostKeyMismatch}.
// Own entries with e.HostKey set must match (mismatch -> ReasonHostKeyMismatch); without it,
// decide is called; false -> ReasonHostKeyRejected. The accepted fingerprint is returned.
//
// The x/crypto client always tries "none" first, then keyboard-interactive with empty
// answers, then an empty password. Cancellation of ctx closes conn at any point of the
// handshake. Handshake, PTY and shell setup share one deadline of sshHandshakeTimeout (10 s).
// Only when the host-key decider is asked is that deadline lifted while it runs and then reset
// to a full timeout: the time the user takes to decide does not eat the remaining budget, and
// the setup after the decision gets a whole new 10 s (so the total can exceed 10 s by the time
// spent before the decision). On every error conn is closed; once ctx is done the error is
// context.Cause(ctx).
func OpenSSH(ctx context.Context, conn net.Conn, e Entry, cols, rows int, decide HostKeyDecider) (*SSHSession, string, error) {
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	fail := func(err error) (*SSHSession, string, error) {
		stop()
		_ = conn.Close()
		if ctx.Err() != nil {
			return nil, "", context.Cause(ctx)
		}
		return nil, "", err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	_ = conn.SetDeadline(time.Now().Add(sshHandshakeTimeout))
	verifier := &sshHostKeyVerifier{ctx: ctx, conn: conn, entry: e, decide: decide}
	user := e.User
	if user == "" {
		user = sshDefaultUser
	}
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.KeyboardInteractive(sshEmptyAnswers),
			ssh.Password(""),
		},
		HostKeyCallback:   verifier.check,
		HostKeyAlgorithms: sshHostKeyAlgorithms,
		Timeout:           sshHandshakeTimeout,
	}
	clientConn, channels, requests, err := ssh.NewClientConn(conn, e.Address(), config)
	if err != nil {
		if verr := verifier.failure(); verr != nil {
			return fail(verr)
		}
		return fail(fmt.Errorf("retronet: ssh handshake: %w", err))
	}
	client := ssh.NewClient(clientConn, channels, requests)
	session, err := openSSHShell(client, cols, rows)
	if err != nil {
		_ = client.Close()
		return fail(fmt.Errorf("retronet: ssh shell: %w", err))
	}
	if !stop() {
		// Cancellation already closed conn: never hand out a late client.
		_ = session.Close()
		return nil, "", context.Cause(ctx)
	}
	_ = conn.SetDeadline(time.Time{})
	return session, verifier.accepted(), nil
}

// sshEmptyAnswers answers every keyboard-interactive question with an empty string.
func sshEmptyAnswers(_, _ string, questions []string, _ []bool) ([]string, error) {
	return make([]string, len(questions)), nil
}

// openSSHShell requests an xterm-256color PTY of cols x rows and starts the login shell.
// Stdout and stderr feed one pipe so Read returns both.
func openSSHShell(client *ssh.Client, cols, rows int) (*SSHSession, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: sshTerminalSpeed,
		ssh.TTY_OP_OSPEED: sshTerminalSpeed,
	}
	if err := session.RequestPty(sshTermType, rows, cols, modes); err != nil {
		_ = session.Close()
		return nil, err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	output, writer := io.Pipe()
	session.Stdout = writer
	session.Stderr = writer
	if err := session.Shell(); err != nil {
		_ = session.Close()
		_ = output.Close()
		return nil, err
	}
	s := &SSHSession{client: client, session: session, stdin: stdin, output: output, waitDone: make(chan struct{})}
	go func() {
		defer close(s.waitDone)
		_ = session.Wait()
		_ = writer.Close()
	}()
	return s, nil
}

// sshHostKeyVerifier applies the pinned and first-contact host key rules of one connection.
type sshHostKeyVerifier struct {
	ctx    context.Context
	conn   net.Conn
	entry  Entry
	decide HostKeyDecider

	mu  sync.Mutex
	key string // fingerprint accepted on this connection; key re-exchanges must present it again
	err error  // first verification failure
}

// check is the ssh.HostKeyCallback.
func (v *sshHostKeyVerifier) check(_ string, _ net.Addr, key ssh.PublicKey) error {
	fingerprint := ssh.FingerprintSHA256(key)
	want := v.entry.HostKey
	if want == "" {
		want = v.accepted()
	}
	switch {
	case want != "" && fingerprint == want:
		return v.accept(fingerprint)
	case want != "":
		return v.reject(&DialError{Reason: ReasonHostKeyMismatch, Err: fmt.Errorf("ssh host key %s does not match the stored key", fingerprint)})
	case !v.entry.Own:
		return v.reject(&DialError{Reason: ReasonHostKeyMismatch, Err: errors.New("catalog entry has no pinned ssh host key")})
	case v.decide == nil:
		return v.reject(&DialError{Reason: ReasonHostKeyRejected, Err: errors.New("no host key decider")})
	}
	// The user may take longer than the handshake budget to decide: lift the deadline while
	// the decider runs, then reset it to a full timeout for the rest of the handshake, PTY and
	// shell setup. The time spent before the decision is deliberately not subtracted.
	_ = v.conn.SetDeadline(time.Time{})
	ok, err := v.decide(v.ctx, key.Type(), fingerprint)
	_ = v.conn.SetDeadline(time.Now().Add(sshHandshakeTimeout))
	if err != nil {
		return v.reject(err)
	}
	if !ok {
		return v.reject(&DialError{Reason: ReasonHostKeyRejected, Err: errors.New("ssh host key rejected by the user")})
	}
	return v.accept(fingerprint)
}

func (v *sshHostKeyVerifier) accept(fingerprint string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.key = fingerprint
	return nil
}

func (v *sshHostKeyVerifier) reject(err error) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.err == nil {
		v.err = err
	}
	return err
}

func (v *sshHostKeyVerifier) accepted() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.key
}

func (v *sshHostKeyVerifier) failure() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.err
}
