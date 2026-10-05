package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

const previewCookieName = "__Host-aurago-preview"
const previewLaunchPath = "/_aurago/launch/"

// Grants are deliberately independent of desktop file tickets. Credentials stay
// server-side; the browser receives only a random, resource-bound capability.
type previewGrant struct {
	resource                                 previewResource
	host, authority, parentOrigin, startPath string
	authorization, session                   string
	expires                                  time.Time
	launch                                   bool
}

type previewResource struct {
	kind, id, port string
}

type previewGrantRegistry struct {
	sync.Mutex
	entries map[[32]byte]previewGrant
}

func (s *Server) previewSettings() (string, bool) {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	if s.Cfg == nil {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(s.Cfg.Server.PreviewDomain)), s.Cfg.Server.PreviewEnabled
}

// Validate against configured identity before dispatch, not against a guest's
// Host header. A typo must never capture the existing AuraGo site.
func (s *Server) configuredPreviewDomain() (string, error) {
	s.CfgMu.RLock()
	if s.Cfg == nil {
		s.CfgMu.RUnlock()
		return "", errors.New("preview configuration unavailable")
	}
	domain := strings.ToLower(strings.TrimSpace(s.Cfg.Server.PreviewDomain))
	primary := strings.ToLower(strings.TrimSpace(s.Cfg.Server.HTTPS.Domain))
	if primary == "" {
		primary = strings.ToLower(strings.TrimSpace(s.Cfg.Server.Host))
	}
	s.CfgMu.RUnlock()
	if err := validatePreviewDomain(primary, ""); err != nil {
		return "", errors.New("isolated previews require AuraGo's primary DNS hostname in server.https.domain or server.host")
	}
	if err := validatePreviewDomain(domain, primary); err != nil {
		return "", err
	}
	return domain, nil
}

func previewRequestAuthority(r *http.Request) string {
	authority := r.Host
	if trusted, _ := r.Context().Value(trustedProxyContextKey{}).(bool); trusted && r.Header.Get("X-Forwarded-Host") != "" {
		authority = r.Header.Get("X-Forwarded-Host")
	}
	return authority
}

func previewRequestHost(r *http.Request) string {
	u, err := url.Parse("https://" + previewRequestAuthority(r))
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func previewRequestSecure(r *http.Request) bool {
	trusted, _ := r.Context().Value(trustedProxyContextKey{}).(bool)
	return r.TLS != nil || (trusted && r.Header.Get("X-Forwarded-Proto") == "https")
}

func previewResourceHost(domain string, resource previewResource) string {
	hash := sha256.Sum256([]byte(resource.kind + "\x00" + resource.id + "\x00" + resource.port))
	return "app-" + hex.EncodeToString(hash[:16]) + "." + domain
}

func validatePreviewDomain(domain, parentHost string) error {
	if domain == "" || net.ParseIP(domain) != nil || len(domain) > 200 || strings.ContainsAny(domain, "/\\:@?#% \t\r\n") || strings.HasSuffix(domain, ".") {
		return errors.New("preview_domain must be a DNS domain on a separate site")
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("invalid preview_domain")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return errors.New("invalid preview_domain")
			}
		}
	}
	site, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return errors.New("preview_domain must not be a public suffix")
	}
	parentSite, _ := publicsuffix.EffectiveTLDPlusOne(parentHost)
	if domain == parentHost || site == parentSite || strings.HasSuffix(parentHost, "."+domain) {
		return errors.New("preview_domain must be on a different registrable site from AuraGo")
	}
	return nil
}

func (s *Server) putPreviewGrant(grant previewGrant) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	registry := &s.previewGrants
	registry.Lock()
	defer registry.Unlock()
	if registry.entries == nil {
		registry.entries = make(map[[32]byte]previewGrant)
	}
	for key, existing := range registry.entries {
		if !time.Now().Before(existing.expires) {
			delete(registry.entries, key)
		}
	}
	if len(registry.entries) >= 8192 {
		return "", errors.New("preview session capacity reached")
	}
	registry.entries[sha256.Sum256([]byte(token))] = grant
	return token, nil
}

func (s *Server) getPreviewGrant(token, host string, launch bool) (previewGrant, bool) {
	registry := &s.previewGrants
	registry.Lock()
	defer registry.Unlock()
	key := sha256.Sum256([]byte(token))
	grant, ok := registry.entries[key]
	if !ok || !time.Now().Before(grant.expires) {
		delete(registry.entries, key)
		return previewGrant{}, false
	}
	if grant.host != host || grant.launch != launch {
		return previewGrant{}, false
	}
	if launch {
		delete(registry.entries, key)
	}
	return grant, true
}

func (s *Server) previewOwnerAllowed(grant previewGrant, write bool) bool {
	if !time.Now().Before(grant.expires) {
		return false
	}
	s.CfgMu.RLock()
	if s.Cfg == nil {
		s.CfgMu.RUnlock()
		return false
	}
	secret, authEnabled := s.Cfg.Auth.SessionSecret, s.Cfg.Auth.Enabled
	readonly := s.Cfg.VirtualDesktop.ReadOnly || (grant.resource.kind == "vm" && s.Cfg.VirtualComputers.ReadOnly)
	vcEnabled := s.Cfg.VirtualComputers.Enabled
	desktopEnabled := s.Cfg.VirtualDesktop.Enabled
	s.CfgMu.RUnlock()
	if !authEnabled || (write && readonly) || (grant.resource.kind == "vm" && !vcEnabled) || (grant.resource.kind == "store" && !desktopEnabled) {
		return false
	}
	if token, bearer := bearerCredential(grant.authorization); bearer {
		scope := desktopScopeRead
		if write {
			scope = desktopScopeWrite
		}
		return desktopTokenHasScope(s, token, scope)
	}
	return grant.session != "" && validateSessionValue(secret, grant.session) && !sessionIsRevoked(secret, grant.session)
}

func (s *Server) issuePreviewLaunch(r *http.Request, resource previewResource, startPath string) (string, error) {
	domain, err := s.configuredPreviewDomain()
	if err != nil {
		return "", err
	}
	if err := validatePreviewDomain(domain, previewRequestHost(r)); err != nil {
		return "", err
	}
	if !previewRequestSecure(r) {
		return "", errors.New("isolated previews require HTTPS on AuraGo and the preview host")
	}
	if !strings.HasPrefix(startPath, "/") || strings.HasPrefix(startPath, "//") || strings.ContainsAny(startPath, "\\\r\n") {
		return "", errors.New("invalid preview path")
	}
	host := previewResourceHost(domain, resource)
	authority := host
	if parent, err := url.Parse("https://" + previewRequestAuthority(r)); err == nil && parent.Port() != "" {
		authority = net.JoinHostPort(host, parent.Port())
	}
	grant := previewGrant{resource: resource, host: host, authority: authority, parentOrigin: "https://" + previewRequestAuthority(r),
		startPath: startPath, authorization: r.Header.Get("Authorization"), expires: time.Now().Add(time.Minute), launch: true}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		grant.session = cookie.Value
	}
	if !s.previewOwnerAllowed(grant, false) {
		return "", errors.New("an authenticated AuraGo session or desktop token is required")
	}
	token, err := s.putPreviewGrant(grant)
	if err != nil {
		return "", err
	}
	return "https://" + grant.authority + previewLaunchPath + token, nil
}

// This dispatch runs after trusted-proxy normalization, but before AuraGo's
// authentication, CSP and access log. Guest URLs and one-use tickets never enter
// the main-site router or its request logs.
func previewHostMiddleware(s *Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain, err := s.configuredPreviewDomain()
		host := previewRequestHost(r)
		if err != nil || !(host == domain || strings.HasSuffix(host, "."+domain)) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !previewRequestSecure(r) {
			http.Error(w, "HTTPS required", http.StatusBadRequest)
			return
		}
		if strings.HasPrefix(r.URL.Path, previewLaunchPath) {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			grant, ok := s.getPreviewGrant(strings.TrimPrefix(r.URL.Path, previewLaunchPath), host, true)
			if !ok || !s.previewOwnerAllowed(grant, false) {
				http.Error(w, "Preview launch expired", http.StatusUnauthorized)
				return
			}
			grant.launch = false
			grant.expires = time.Now().Add(12 * time.Hour)
			token, err := s.putPreviewGrant(grant)
			if err != nil {
				http.Error(w, "Preview unavailable", http.StatusServiceUnavailable)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: previewCookieName, Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteNoneMode, Partitioned: true, MaxAge: 12 * 60 * 60})
			http.Redirect(w, r, grant.startPath, http.StatusSeeOther)
			return
		}
		cookie, err := r.Cookie(previewCookieName)
		if err != nil {
			http.Error(w, "Open this app from AuraGo to start a preview session", http.StatusUnauthorized)
			return
		}
		grant, ok := s.getPreviewGrant(cookie.Value, host, false)
		write := r.Method != http.MethodGet && r.Method != http.MethodHead || strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
		if !ok || !s.previewOwnerAllowed(grant, write) {
			http.Error(w, "Preview access expired or revoked", http.StatusForbidden)
			return
		}
		// Guest requests may originate only from this resource or the authenticated
		// parent embedding it. This also protects non-partitioning browsers from CSRF.
		if origin := r.Header.Get("Origin"); origin != "" && origin != "https://"+grant.authority && origin != grant.parentOrigin {
			http.Error(w, "Invalid preview origin", http.StatusForbidden)
			return
		}
		s.servePreviewProxy(w, r, grant, write)
	})
}

func reservedPreviewCookie(name string) bool {
	return name == previewCookieName || name == sessionCookieName || strings.HasPrefix(strings.ToLower(name), "aurago_")
}

func (s *Server) servePreviewProxy(w http.ResponseWriter, r *http.Request, grant previewGrant, write bool) {
	target, err := s.previewTarget(r.Context(), grant.resource)
	if err != nil {
		http.Error(w, "Preview target unavailable", http.StatusBadGateway)
		return
	}
	if r.Context().Err() != nil || !s.previewOwnerAllowed(grant, write) {
		http.Error(w, "Preview access expired or revoked", http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if write {
		var done context.CancelFunc
		ctx, done, err = s.beginDesktopRun(ctx)
		if err != nil {
			http.Error(w, "Preview write access revoked", http.StatusForbidden)
			return
		}
		defer done()
	}
	// ReverseProxy closes upgraded transports when the request context ends.
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !s.previewOwnerAllowed(grant, write) {
					cancel()
					return
				}
			}
		}
	}()
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = grant.authority
			pr.Out.Header.Del("Cookie")
			for _, cookie := range pr.In.Cookies() {
				if !reservedPreviewCookie(cookie.Name) {
					pr.Out.AddCookie(cookie)
				}
			}
			if auth := pr.Out.Header.Get("Authorization"); auth == grant.authorization && auth != "" {
				pr.Out.Header.Del("Authorization")
			}
			if token, bearer := bearerCredential(pr.Out.Header.Get("Authorization")); bearer {
				if tm := s.currentTokenManager(); tm != nil {
					if _, valid := tm.Validate(token, ""); valid {
						pr.Out.Header.Del("Authorization")
					}
				}
			}
			pr.Out.Header.Del("X-Internal-Token")
			pr.Out.Header.Del("X-Internal-FollowUp")
			pr.Out.Header.Del("X-AuraGo-Agodesk-Dev-Token")
			pr.Out.Header.Del("Proxy-Authorization")
			pr.Out.Header.Set("X-Forwarded-Host", grant.authority)
			pr.Out.Header.Set("X-Forwarded-Proto", "https")
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Clear-Site-Data")
			resp.Header.Del("Alt-Svc")
			cookies := resp.Cookies()
			resp.Header.Del("Set-Cookie")
			for _, cookie := range cookies {
				if reservedPreviewCookie(cookie.Name) {
					continue
				}
				cookie.Domain = ""
				cookie.Secure = true
				cookie.SameSite = http.SameSiteNoneMode
				cookie.Partitioned = true
				resp.Header.Add("Set-Cookie", cookie.String())
			}
			resp.Header.Set("Referrer-Policy", "no-referrer")
			resp.Header.Set("X-Content-Type-Options", "nosniff")
			if location, err := resp.Location(); err == nil && location.Host == target.Host {
				location.Scheme, location.Host = "https", grant.authority
				location.Path = strings.TrimPrefix(location.Path, strings.TrimSuffix(target.Path, "/"))
				if location.Path == "" {
					location.Path = "/"
				}
				resp.Header.Set("Location", location.String())
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Preview connection unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r.WithContext(ctx))
}
