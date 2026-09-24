package server

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

type trustedProxyContextKey struct{}

// trustedProxyMiddleware removes client-supplied forwarding claims unless the
// immediate peer is an explicitly configured proxy.
func trustedProxyMiddleware(s *Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trusted := false
		if s != nil && s.Cfg != nil {
			s.CfgMu.RLock()
			if s.Cfg.Server.HTTPS.BehindProxy {
				peer, err := netip.ParseAddr(remoteIP(r.RemoteAddr))
				if err == nil {
					peer = peer.Unmap()
					for _, raw := range s.Cfg.Server.HTTPS.TrustedProxyCIDRs {
						if prefix, err := netip.ParsePrefix(strings.TrimSpace(raw)); err == nil && prefix.Contains(peer) {
							trusted = true
							break
						}
						if address, err := netip.ParseAddr(strings.TrimSpace(raw)); err == nil && address.Unmap() == peer {
							trusted = true
							break
						}
					}
				}
			}
			s.CfgMu.RUnlock()
		}
		for name := range r.Header {
			lower := strings.ToLower(name)
			allowedForwarded := lower == "x-forwarded-for" || lower == "x-forwarded-host" || lower == "x-forwarded-proto"
			if lower == "forwarded" || lower == "x-real-ip" || lower == "x-scheme" || (strings.HasPrefix(lower, "x-forwarded-") && (!trusted || !allowedForwarded)) {
				r.Header.Del(name)
			}
		}
		if trusted {
			if host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); host != "" {
				if strings.ContainsAny(host, ",/\\ \t\r\n") || !validRequestAuthority(host) {
					r.Header.Del("X-Forwarded-Host")
				}
			}
			proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
			if proto != "http" && proto != "https" {
				r.Header.Del("X-Forwarded-Proto")
			}
			if raw := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); raw != "" {
				parts := strings.Split(raw, ",")
				candidate := strings.TrimSpace(parts[len(parts)-1])
				if ip, err := netip.ParseAddr(candidate); err == nil {
					r.Header.Set("X-Forwarded-For", ip.Unmap().String())
				} else {
					r.Header.Del("X-Forwarded-For")
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), trustedProxyContextKey{}, trusted)))
	})
}

func remoteIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return strings.Trim(addr, "[]")
}

func validRequestAuthority(authority string) bool {
	parsed, err := url.Parse("http://" + authority)
	return err == nil && parsed.Host == authority && parsed.Hostname() != "" && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == ""
}

func trustedForwardedRequest(r *http.Request) bool {
	return r != nil && r.Context().Value(trustedProxyContextKey{}) == true
}

func requestHost(r *http.Request) string {
	if trustedForwardedRequest(r) {
		if host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); validRequestAuthority(host) {
			return host
		}
	}
	return r.Host
}

func requestOriginMatches(r *http.Request, raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	wantScheme := "http"
	if IsSecureRequest(r) {
		wantScheme = "https"
	}
	return parsed.Scheme == wantScheme && strings.EqualFold(parsed.Host, requestHost(r))
}
