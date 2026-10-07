//go:build !remote_minimal

package remote

import (
	"strings"
	"testing"
	"time"
)

func TestGetSSHConfigNamesTheRealInsecureHostKeySetting(t *testing.T) {
	prior := InsecureHostKey
	InsecureHostKey = false
	defer func() { InsecureHostKey = prior }()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	knownHostsCache.mu.Lock()
	knownHostsCache.callback, knownHostsCache.path, knownHostsCache.expiresAt = nil, "", time.Time{}
	knownHostsCache.mu.Unlock()

	_, err := GetSSHConfig("fixture", []byte("fixture"))
	if err == nil {
		t.Fatal("GetSSHConfig succeeded without known_hosts")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "SSH host key verification failed: known_hosts file not found at ") ||
		!strings.Contains(msg, "'remote_control.ssh_insecure_host_key: true' in config to disable host verification (not recommended)") ||
		strings.Contains(msg, "'ssh.insecure_host_key") {
		t.Fatalf("message = %q, want the real setting remote_control.ssh_insecure_host_key", msg)
	}
}
