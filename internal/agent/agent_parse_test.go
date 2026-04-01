package agent

import (
	"net"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/security"
)

type stubLocalIPConn struct {
	localAddr net.Addr
}

func (c *stubLocalIPConn) Read(_ []byte) (int, error) { return 0, nil }

func (c *stubLocalIPConn) Write(_ []byte) (int, error) { return 0, nil }

func (c *stubLocalIPConn) Close() error { return nil }

func (c *stubLocalIPConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *stubLocalIPConn) RemoteAddr() net.Addr { return &net.UDPAddr{} }

func (c *stubLocalIPConn) SetDeadline(_ time.Time) error { return nil }

func (c *stubLocalIPConn) SetReadDeadline(_ time.Time) error { return nil }

func (c *stubLocalIPConn) SetWriteDeadline(_ time.Time) error { return nil }

func TestDispatchToolCallOutputSanitizesThinkingTagsAndSecrets(t *testing.T) {
	raw := "prefix <thinking>internal chain of thought</thinking> password: supersecret123 suffix"
	sanitized := security.StripThinkingTags(security.RedactSensitiveInfo(raw))

	if sanitized == raw {
		t.Fatal("expected sanitized output to differ from raw output")
	}
	if contains := (len(sanitized) > 0 && (sanitized == raw)); contains {
		t.Fatal("unexpected unchanged sanitized output")
	}
	if want := "<thinking>"; containsSubstring(sanitized, want) {
		t.Fatalf("expected thinking tags to be removed, got %q", sanitized)
	}
	if want := "supersecret123"; containsSubstring(sanitized, want) {
		t.Fatalf("expected secret to be redacted, got %q", sanitized)
	}
	if want := "[redacted]"; !containsSubstring(sanitized, want) {
		t.Fatalf("expected redacted marker in %q", sanitized)
	}
}

func containsSubstring(value, want string) bool {
	return len(want) > 0 && len(value) >= len(want) && (value == want || containsRunes(value, want))
}

func containsRunes(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}

func TestToolCallParamsTruncatesCodeOnUTF8Boundary(t *testing.T) {
	tc := ToolCall{Code: strings.Repeat("界", 200)}
	params := toolCallParams(tc)
	code := params["code"]

	if code == "" {
		t.Fatal("expected code preview to be present")
	}
	if len(code) > 300 {
		t.Fatalf("code preview length = %d, want <= 300", len(code))
	}
	if !utf8.ValidString(code) {
		t.Fatalf("expected valid UTF-8 preview, got %q", code)
	}
}

func TestGetLocalIPCachesResolverResult(t *testing.T) {
	localIPCache = sync.Map{}
	originalDial := localIPDial
	defer func() {
		localIPDial = originalDial
		localIPCache = sync.Map{}
	}()

	resolverCalls := 0
	localIPDial = func(network, address string) (net.Conn, error) {
		resolverCalls++
		return &stubLocalIPConn{localAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.44"), Port: 12345}}, nil
	}

	cfg := &config.Config{}
	if got := getLocalIP(cfg); got != "192.168.1.44" {
		t.Fatalf("first getLocalIP() = %q, want 192.168.1.44", got)
	}
	if got := getLocalIP(cfg); got != "192.168.1.44" {
		t.Fatalf("second getLocalIP() = %q, want 192.168.1.44", got)
	}
	if resolverCalls != 1 {
		t.Fatalf("resolverCalls = %d, want 1", resolverCalls)
	}
}
