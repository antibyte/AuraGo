package server

import (
	"bufio"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"aurago/internal/config"
)

// This runs inside the built-in TLS listener's security-header middleware.
// After a successful HTTP opt-out, every remaining TLS response clears our
// previous HSTS rule, including config refreshes before the requested restart.
func (s *Server) httpsTransitionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &httpsTransitionWriter{ResponseWriter: w, server: s}
		writer.prepareHeader()
		defer writer.prepareHeader() // Also cover an implicit empty response.
		next.ServeHTTP(writer, r)
	})
}

type httpsTransitionWriter struct {
	http.ResponseWriter
	server *Server
	sent   bool
}

func (w *httpsTransitionWriter) prepareHeader() {
	if w.sent {
		return
	}
	// Use the published snapshot: a handler may hold CfgMu while writing.
	if cfg := w.server.ConfigSnapshot(); cfg != nil && !cfg.Server.HTTPS.Enabled {
		w.Header().Set("Strict-Transport-Security", "max-age=0")
	}
}

func (w *httpsTransitionWriter) WriteHeader(status int) {
	w.prepareHeader()
	w.sent = w.sent || status >= 200 || status == http.StatusSwitchingProtocols
	w.ResponseWriter.WriteHeader(status)
}

func (w *httpsTransitionWriter) Write(data []byte) (int, error) {
	w.prepareHeader()
	w.sent = true
	return w.ResponseWriter.Write(data)
}

func (w *httpsTransitionWriter) FlushError() error {
	w.prepareHeader()
	w.sent = true
	return http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *httpsTransitionWriter) Flush() { _ = w.FlushError() }

func (w *httpsTransitionWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *httpsTransitionWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.prepareHeader()
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func setupHTTPSStatus(cfg *config.Config) map[string]interface{} {
	mode := config.NormalizeCertMode(cfg.Server.HTTPS.CertMode)
	if config.UsesSelfSignedTLS(cfg) {
		mode = "selfsigned"
	}
	if mode != "selfsigned" && mode != "custom" {
		mode = "auto"
	}
	port := cfg.Server.HTTPS.HTTPSPort
	if port <= 0 {
		port = 443
	}
	return map[string]interface{}{"enabled": cfg.Server.HTTPS.Enabled, "cert_mode": mode, "port": port}
}

func configRequestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host}).String()
}

// Keep proxy/tunnel addresses under the operator's control. The browser also
// checks restart_from against its own origin, covering TLS-terminating proxies
// that have not been declared in the config.
func configRestartURL(r *http.Request, oldCfg, newCfg *config.Config) string {
	if oldCfg == nil || newCfg == nil {
		return ""
	}
	if oldCfg.Server.HTTPS.BehindProxy || newCfg.Server.HTTPS.BehindProxy || oldCfg.SecurityProxy.Enabled || newCfg.SecurityProxy.Enabled {
		return ""
	}
	u, err := url.Parse(configRequestOrigin(r))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	oldPort := oldCfg.Server.Port
	if r.TLS != nil {
		oldPort = oldCfg.Server.HTTPS.HTTPSPort
	}
	if addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		if _, port, err := net.SplitHostPort(addr.String()); err == nil {
			oldPort, _ = strconv.Atoi(port)
		}
	}
	requestPort := u.Port()
	if requestPort == "" {
		requestPort = "80"
		if r.TLS != nil {
			requestPort = "443"
		}
	}
	// A different public port may be a port mapping or another HTTPS proxy.
	// Do not replace that public endpoint with an internal listener address.
	if requestPort != strconv.Itoa(oldPort) {
		return ""
	}
	port := newCfg.Server.Port
	u.Scheme = "http"
	if newCfg.Server.HTTPS.Enabled {
		u.Scheme = "https"
		port = newCfg.Server.HTTPS.HTTPSPort
	}
	u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(port))
	u.Path = "/config"
	return u.String()
}
