package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/desktop"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type codeStudioSSHFixture struct {
	addr           string
	host           string
	port           int
	hostKey        ssh.PublicKey
	user, password string
	mu             sync.Mutex
	execs          []string
	windows        [][2]int // cols, rows (pty-req and window-change)
	exec           func(cmd string, stdin []byte) (string, uint32)
}

func startCodeStudioSSHFixture(t *testing.T) *codeStudioSSHFixture {
	t.Helper()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(key)
	f := &codeStudioSSHFixture{hostKey: signer.PublicKey(), user: "dev", password: "fixture-pw",
		exec: func(cmd string, _ []byte) (string, uint32) {
			if strings.Contains(cmd, "uname") {
				return "os=Linux\narch=x86_64\nshell=/bin/bash\nhas=git\nhas=grep\nhas=gofmt\nhas=gopls\n", 0
			}
			return "", 0
		}}
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
		if c.User() == f.user && string(pw) == f.password {
			return nil, nil
		}
		return nil, fmt.Errorf("denied")
	}}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.addr = ln.Addr().String()
	f.host, f.port = "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
	fs := sftp.InMemHandler()
	var wg sync.WaitGroup
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() { defer wg.Done(); f.serve(conn, cfg, fs) }()
		}
	}()
	t.Cleanup(func() { _ = ln.Close(); wg.Wait() })
	return f
}

func (f *codeStudioSSHFixture) serve(raw net.Conn, cfg *ssh.ServerConfig, fs sftp.Handlers) {
	defer raw.Close()
	sc, chans, reqs, err := ssh.NewServerConn(raw, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs) // keepalive@openssh.com gets "false", which is not an error
	for nc := range chans {
		if nc.ChannelType() != "session" {
			_ = nc.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		ch, creqs, err := nc.Accept()
		if err != nil {
			return
		}
		go f.session(ch, creqs, fs)
	}
}

func (f *codeStudioSSHFixture) session(ch ssh.Channel, reqs <-chan *ssh.Request, fs sftp.Handlers) {
	pty := false
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			var p struct {
				Term       string
				Cols, Rows uint32
				W, H       uint32
				Modes      string
			}
			_ = ssh.Unmarshal(req.Payload, &p)
			f.mu.Lock()
			f.windows = append(f.windows, [2]int{int(p.Cols), int(p.Rows)})
			f.mu.Unlock()
			pty = true
			_ = req.Reply(true, nil)
		case "window-change":
			var p struct{ Cols, Rows, W, H uint32 }
			_ = ssh.Unmarshal(req.Payload, &p)
			f.mu.Lock()
			f.windows = append(f.windows, [2]int{int(p.Cols), int(p.Rows)})
			f.mu.Unlock()
		case "subsystem":
			_ = req.Reply(true, nil)
			srv := sftp.NewRequestServer(ch, fs)
			go func() { _ = srv.Serve(); _ = srv.Close() }()
		case "exec":
			var p struct{ Command string }
			_ = ssh.Unmarshal(req.Payload, &p)
			f.mu.Lock()
			f.execs = append(f.execs, p.Command)
			f.mu.Unlock()
			_ = req.Reply(true, nil)
			go func(cmd string, echo bool) {
				var code uint32
				if echo { // PTY shell or StartProcess("aurago-echo"): echo until stdin EOF
					_, _ = io.Copy(ch, ch)
				} else {
					stdin, _ := io.ReadAll(ch)
					out, c := f.exec(cmd, stdin)
					_, _ = io.WriteString(ch, out)
					code = c
				}
				_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{code}))
				_ = ch.Close()
			}(p.Command, pty || strings.Contains(p.Command, "aurago-echo"))
		default:
			if req.WantReply {
				_ = req.Reply(req.Type == "signal", nil)
			}
		}
	}
}

func (f *codeStudioSSHFixture) lastExec() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.execs[len(f.execs)-1]
}

// unwrapShC reverses codeStudioSSHCommandLine for assertions.
func unwrapShC(cmd string) string {
	return strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(cmd, "sh -c '"), "'"), `'"'"'`, "'")
}

func (f *codeStudioSSHFixture) knownHostsFile(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(p, []byte(knownhosts.Line([]string{f.addr}, f.hostKey)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// testDial dials the fixture for any device id (no inventory).
func (f *codeStudioSSHFixture) testDial(t *testing.T) codeStudioSSHDialFunc {
	kh := f.knownHostsFile(t)
	return func(ctx context.Context, deviceID, trust string) (*ssh.Client, codeStudioSSHTarget, error) {
		target := codeStudioSSHTarget{DeviceID: deviceID, Name: "fixture", Host: f.host, Port: f.port, User: f.user}
		c, err := desktop.DialSSH(ctx, desktop.SSHAccess{Host: f.host, Port: f.port, Username: f.user, Secret: []byte(f.password)},
			desktop.SSHDialOptions{KnownHostsFile: kh, TrustFingerprint: trust})
		if err != nil {
			return nil, target, codeStudioClassifyDialError(err, target)
		}
		return c, target, nil
	}
}

// adminSFTP gives the test a direct client (host key ignored: test only) to seed files.
func (f *codeStudioSSHFixture) adminSFTP(t *testing.T) *sftp.Client {
	c, err := ssh.Dial("tcp", f.addr, &ssh.ClientConfig{User: f.user, Auth: []ssh.AuthMethod{ssh.Password(f.password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey()}) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	sc, err := sftp.NewClient(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sc.Close(); _ = c.Close() })
	return sc
}

func newCodeStudioTestSSHBackend(t *testing.T, f *codeStudioSSHFixture, root string) (*codeStudioSSHBackend, *codeStudioRemote) {
	t.Helper()
	if err := f.adminSFTP(t).MkdirAll(root); err != nil {
		t.Fatal(err)
	}
	remote := newCodeStudioRemote(f.testDial(t), slog.Default())
	t.Cleanup(remote.pool.closeAll)
	conn, err := remote.pool.get(context.Background(), "dev1", "")
	if err != nil {
		t.Fatal(err)
	}
	return newCodeStudioSSHBackend(codeStudioWorkspaceRecord{ID: "ssh_test", DeviceID: "dev1", Root: root}, remote.pool, conn.target), remote
}
