package desktop

import (
	"crypto/ed25519"
	"crypto/rand"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/remote"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestRemoteProxyWebSocketOriginRejectsEmptyAndCrossOrigin(t *testing.T) {
	t.Parallel()

	for name, checkOrigin := range map[string]func(*http.Request) bool{
		"ssh": sshUpgrader.CheckOrigin,
		"vnc": vncUpgrader.CheckOrigin,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://aurago.test/api/desktop/"+name, nil)
			if checkOrigin(req) {
				t.Fatal("empty Origin was accepted")
			}

			req.Header.Set("Origin", "https://evil.test")
			if checkOrigin(req) {
				t.Fatal("cross-origin websocket was accepted")
			}

			req.Header.Set("Origin", "https://aurago.test")
			if !checkOrigin(req) {
				t.Fatal("matching Origin was rejected")
			}
		})
	}
}

func TestBuildSSHConfigRequiresKnownHostsUnlessExplicitlyInsecure(t *testing.T) {
	previous := remote.InsecureHostKey
	remote.InsecureHostKey = false
	t.Cleanup(func() { remote.InsecureHostKey = previous })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	_, err := buildSSHConfig("user", []byte("password"), slog.Default(), nil)
	if err == nil {
		t.Fatal("buildSSHConfig succeeded without known_hosts")
	}
	if !strings.Contains(err.Error(), "known_hosts") {
		t.Fatalf("error = %v, want known_hosts guidance", err)
	}
}

func TestBuildSSHConfigUsesKnownHostsWhenPresent(t *testing.T) {
	previous := remote.InsecureHostKey
	remote.InsecureHostKey = false
	t.Cleanup(func() { remote.InsecureHostKey = previous })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh public key: %v", err)
	}
	knownHostsLine := knownhosts.Line([]string{"example.test"}, sshPub)
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), []byte(knownHostsLine+"\n"), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}

	result, err := buildSSHConfig("user", []byte("password"), slog.Default(), nil)
	if err != nil {
		t.Fatalf("buildSSHConfig: %v", err)
	}
	if result.InsecureHostKey {
		t.Fatal("known_hosts path should not report insecure host key mode")
	}
}

func TestBuildSSHConfigPromptsAndStoresUnknownHostKey(t *testing.T) {
	previous := remote.InsecureHostKey
	remote.InsecureHostKey = false
	t.Cleanup(func() { remote.InsecureHostKey = previous })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh public key: %v", err)
	}

	var prompted bool
	result, err := buildSSHConfig("user", []byte("password"), slog.Default(), func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) (bool, error) {
		prompted = true
		if hostname != "example.test:2222" {
			t.Fatalf("hostname = %q, want example.test:2222", hostname)
		}
		if key.Type() != sshPub.Type() {
			t.Fatalf("key type = %q, want %q", key.Type(), sshPub.Type())
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("buildSSHConfig: %v", err)
	}

	err = result.Config.HostKeyCallback("example.test:2222", &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 2222}, sshPub)
	if err != nil {
		t.Fatalf("host key callback: %v", err)
	}
	if !prompted {
		t.Fatal("unknown host key prompt was not called")
	}

	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if text := string(content); !strings.Contains(text, "example.test") || !strings.Contains(text, sshPub.Type()) {
		t.Fatalf("known_hosts = %q, want accepted host key line", text)
	}

	result, err = buildSSHConfig("user", []byte("password"), slog.Default(), nil)
	if err != nil {
		t.Fatalf("buildSSHConfig after storing known_hosts: %v", err)
	}
	if err := result.Config.HostKeyCallback("example.test:2222", &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 2222}, sshPub); err != nil {
		t.Fatalf("stored host key callback: %v", err)
	}
}
