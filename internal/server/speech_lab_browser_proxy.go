package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
)

const speechLabBrowserPath = "/speech-lab/"

// speechLabBrowserBackend is configuration-owned. A browser request never
// supplies the upstream address, and external deployments must set it explicitly.
func speechLabBrowserBackend(cfg *config.Config) (*url.URL, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Speech Lab configuration is unavailable")
	}
	raw := strings.TrimSpace(cfg.SpeechLab.BrowserBackendURL)
	if raw == "" {
		if cfg.SpeechLab.Deployment.Mode == "external" {
			return nil, fmt.Errorf("speech_lab.browser_backend_url is required for an external Browser Lab")
		}
		if cfg.Runtime.IsDocker {
			raw = "http://s2s-web:80"
		} else {
			raw = "http://127.0.0.1:8766"
		}
	}
	if err := config.ValidateSpeechLabBrowserBackendURL(raw); err != nil {
		return nil, err
	}
	return url.Parse(raw)
}

// speechLabBrowserTransport pins each connection to a private resolved address.
// It ignores ambient proxy settings so they cannot bypass the destination check.
func speechLabBrowserTransport(target *url.URL) *http.Transport {
	return &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			wantedPort := target.Port()
			if wantedPort == "" {
				if target.Scheme == "https" {
					wantedPort = "443"
				} else {
					wantedPort = "80"
				}
			}
			if err != nil || !strings.EqualFold(host, target.Hostname()) || port != wantedPort {
				return nil, fmt.Errorf("Speech Lab backend destination changed")
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve Speech Lab backend: %w", err)
			}
			for _, ip := range ips {
				addr, ok := netip.AddrFromSlice(ip.IP)
				if !ok {
					continue
				}
				addr = addr.Unmap()
				if !addr.IsLoopback() && !addr.IsPrivate() {
					continue
				}
				return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
			}
			return nil, fmt.Errorf("Speech Lab backend must resolve to a loopback or private address")
		},
	}
}

func handleSpeechLabBrowser(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.Auth.Enabled {
			http.Error(w, "Speech Lab browser requires AuraGo authentication", http.StatusForbidden)
			return
		}
		if !cfg.SpeechLab.Enabled {
			http.NotFound(w, r)
			return
		}
		if !IsAuthenticated(r, cfg.Auth.SessionSecret) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		if !isSafeMethod(r.Method) && !checkCSRFOriginWithPolicy(r, false) {
			http.Error(w, "Origin does not match AuraGo", http.StatusForbidden)
			return
		}
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") &&
			(strings.TrimSpace(r.Header.Get("Origin")) == "" || !sameOriginOrNoOrigin(r)) {
			http.Error(w, "WebSocket origin does not match AuraGo", http.StatusForbidden)
			return
		}
		target, err := speechLabBrowserBackend(cfg)
		if err != nil {
			http.Error(w, "Speech Lab browser backend is unavailable", http.StatusServiceUnavailable)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Transport = speechLabBrowserTransport(target)
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = target.Host
			req.URL.Path = strings.TrimPrefix(req.URL.Path, strings.TrimSuffix(speechLabBrowserPath, "/"))
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.URL.RawPath = ""
			for _, key := range []string{"Authorization", "Cookie", "Proxy-Authorization", "Origin", "X-Internal-Token", "X-Internal-FollowUp", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-For"} {
				req.Header.Del(key)
			}
		}
		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Credentials")
			if location := resp.Header.Get("Location"); location != "" {
				parsed, err := url.Parse(location)
				if err != nil || (parsed.IsAbs() && (parsed.Scheme != target.Scheme || !strings.EqualFold(parsed.Host, target.Host))) || strings.HasPrefix(location, "//") {
					return fmt.Errorf("Speech Lab backend redirect is outside the configured origin")
				}
				if parsed.IsAbs() {
					location = parsed.RequestURI()
				}
				if strings.HasPrefix(location, "/") {
					resp.Header.Set("Location", strings.TrimSuffix(speechLabBrowserPath, "/")+location)
				}
			}
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Speech Lab browser backend is unavailable", http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	})
}
