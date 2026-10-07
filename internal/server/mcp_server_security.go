package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
)

// Match administrator-configured names or literal local addresses, never DNS
// results for the request Host. Matching an attacker-controlled Origin to Host
// alone does not prevent DNS rebinding.
func mcpTrustedHost(r *http.Request, cfg *config.Config) bool {
	u, err := url.Parse("http://" + r.Host)
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return false
	}
	localName, _ := os.Hostname()
	trusted := []string{"localhost", localName, cfg.Server.Host, cfg.Server.HTTPS.Domain, cfg.Tailscale.TsNet.Hostname}
	if callback, err := url.Parse(cfg.Server.OAuthRedirectBaseURL); err == nil {
		trusted = append(trusted, callback.Hostname())
	}
	for _, candidate := range trusted {
		if candidate != "" && candidate != "0.0.0.0" && candidate != "::" && host == strings.ToLower(strings.TrimSuffix(candidate, ".")) {
			return true
		}
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		localHost, _, _ := net.SplitHostPort(local.String())
		if ip.Equal(net.ParseIP(localHost)) {
			return true
		}
	}
	addresses, _ := net.InterfaceAddrs()
	for _, address := range addresses {
		local, _, err := net.ParseCIDR(address.String())
		if err == nil && ip.Equal(local) {
			return true
		}
	}
	return false
}

type mcpSessionSigner struct {
	once sync.Once
	key  [32]byte
	err  error
}

type mcpSessionContextKey struct{}

func mcpSessionPrincipal(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		return auth
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		return cookie.Value
	}
	return "anonymous"
}

func (s *mcpSessionSigner) sign(payload, principal string) string {
	mac := hmac.New(sha256.New, s.key[:])
	_, _ = mac.Write([]byte(payload))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(principal))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *mcpSessionSigner) session(r *http.Request, initialize bool) (string, error) {
	s.once.Do(func() { _, s.err = rand.Read(s.key[:]) })
	if s.err != nil {
		return "", fmt.Errorf("MCP session initialization failed: %w", s.err)
	}
	principal := mcpSessionPrincipal(r)
	if supplied := r.Header.Get("Mcp-Session-Id"); supplied != "" && !initialize {
		parts := strings.Split(supplied, ".")
		if len(parts) != 3 || len(parts[0]) != 64 {
			return "", fmt.Errorf("unknown MCP session; initialize again")
		}
		expires, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || time.Now().Unix() >= expires || !hmac.Equal([]byte(parts[2]), []byte(s.sign(parts[0]+"."+parts[1], principal))) {
			return "", fmt.Errorf("unknown or expired MCP session; initialize again")
		}
		return supplied, nil
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	payload := hex.EncodeToString(nonce[:]) + "." + strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10)
	return payload + "." + s.sign(payload, principal), nil
}

func mcpDispatchSessionID(ctx context.Context) string {
	if session, ok := ctx.Value(mcpSessionContextKey{}).(string); ok && session != "" {
		digest := sha256.Sum256([]byte(session))
		return "mcp-" + hex.EncodeToString(digest[:])
	}
	// Direct/stateless invocations never inherit another client's history.
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return ""
	}
	return "mcp-" + hex.EncodeToString(nonce[:])
}

func mcpScopedConfig(s *Server, original *config.Config) *config.Config {
	scoped := *original
	scoped.AuthorizationSnapshots = func() (*config.Config, *config.Config) {
		s.CfgMu.RLock()
		current := s.Cfg
		s.CfgMu.RUnlock()
		if !mcpServerEnabled(current) || !reflect.DeepEqual(mcpEffectiveAllowedTools(original), mcpEffectiveAllowedTools(current)) {
			return nil, nil
		}
		return original, current
	}
	return &scoped
}
