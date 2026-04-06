package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ── TOTP (RFC 6238 / RFC 4226) ──────────────────────────────────────────────

// GenerateTOTPSecret generates a new random, base32-encoded TOTP secret (160-bit).
func GenerateTOTPSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// totpCode computes the 6-digit HOTP code for the given secret and counter.
func totpCode(secret string, counter uint64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("invalid TOTP secret: %w", err)
	}

	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	h := mac.Sum(nil)

	offset := h[len(h)-1] & 0x0f
	code := (uint32(h[offset])&0x7f)<<24 |
		uint32(h[offset+1])<<16 |
		uint32(h[offset+2])<<8 |
		uint32(h[offset+3])
	code = code % 1_000_000
	return fmt.Sprintf("%06d", code), nil
}

// VerifyTOTP checks code against the current and ±1 TOTP windows (90-second grace).
func VerifyTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	now := time.Now()
	for _, delta := range []int{-1, 0, 1} {
		t := now.Add(time.Duration(delta) * 30 * time.Second)
		counter := uint64(math.Floor(float64(t.Unix()) / 30))
		expected, err := totpCode(secret, counter)
		if err != nil {
			return false
		}
		// Constant-time compare
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}
	return false
}

// TOTPAuthURI returns the otpauth:// URI suitable for QR code generation.
func TOTPAuthURI(secret, issuer, account string) string {
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, account, secret, issuer,
	)
}

// ── Password Hashing (bcrypt, cost 12) ──────────────────────────────────────

const bcryptCost = 12

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies plaintext against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ── Session Cookies (HMAC-SHA256, stateless) ────────────────────────────────

const sessionCookieName = "aurago_session"

// createSessionValue produces a tamper-proof session token.
// Format: base64url(payload) + "." + hmac_hex
// Payload: "user|<unix_expires>"
func createSessionValue(secret string, expiry time.Time) string {
	payload := fmt.Sprintf("user|%d", expiry.Unix())
	payloadEnc := base64.URLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadEnc))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payloadEnc + "." + sig
}

// validateSessionValue verifies the signature and expiry of a session token.
func validateSessionValue(secret, value string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	payloadEnc, sig := parts[0], parts[1]

	// Verify HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadEnc))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return false
	}

	// Decode and check expiry
	payloadBytes, err := base64.URLEncoding.DecodeString(payloadEnc)
	if err != nil {
		return false
	}
	var expires int64
	if _, err := fmt.Sscanf(string(payloadBytes), "user|%d", &expires); err != nil {
		return false
	}
	return time.Now().Unix() < expires
}

// SetSessionCookie writes a signed, HttpOnly session cookie to the response.
// The Secure flag is automatically set when the request is over HTTPS.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, secret string, timeout time.Duration) {
	expiry := time.Now().Add(timeout)
	value := createSessionValue(secret, expiry)

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	// Set Secure flag if request is HTTPS (direct or via proxy)
	if IsSecureRequest(r) {
		cookie.Secure = true
	}

	http.SetCookie(w, cookie)
}

func clearSessionCookieVariant(w http.ResponseWriter, secure bool) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	cookie.Secure = secure
	http.SetCookie(w, cookie)
}

// ClearSessionCookie expires the session cookie immediately.
// We expire both insecure and secure variants so logout remains reliable when
// users switch between plain HTTP and HTTPS/Tailscale access on the same host.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	clearSessionCookieVariant(w, false)
	if IsSecureRequest(r) {
		clearSessionCookieVariant(w, true)
	}
}

// IsAuthenticated returns true if the request carries a valid session cookie.
func IsAuthenticated(r *http.Request, secret string) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return validateSessionValue(secret, cookie.Value)
}

// ── Rate Limiting ────────────────────────────────────────────────────────────

type loginRecord struct {
	mu          sync.Mutex
	count       int
	lockedUntil time.Time
}

var (
	loginMu      sync.Mutex
	loginRecords = make(map[string]*loginRecord)
)

func getLoginRecord(ip string) *loginRecord {
	loginMu.Lock()
	defer loginMu.Unlock()
	if r, ok := loginRecords[ip]; ok {
		return r
	}
	r := &loginRecord{}
	loginRecords[ip] = r
	return r
}

func loginScopeKey(scope, value string) string {
	return scope + ":" + value
}

// IsLockedOut returns true if the IP is currently in a lockout period.
func IsLockedOut(ip string) bool {
	r := getLoginRecord(ip)
	r.mu.Lock()
	defer r.mu.Unlock()
	return time.Now().Before(r.lockedUntil)
}

// IsLockedOutAny returns true if any provided rate-limit key is locked.
func IsLockedOutAny(keys ...string) bool {
	for _, key := range keys {
		if key != "" && IsLockedOut(key) {
			return true
		}
	}
	return false
}

// RecordFailedLogin records a failed attempt and triggers lockout when threshold is reached.
func RecordFailedLogin(ip string, maxAttempts, lockoutMinutes int) {
	r := getLoginRecord(ip)
	r.mu.Lock()
	defer r.mu.Unlock()
	// If lockout has expired, reset counter
	if !r.lockedUntil.IsZero() && time.Now().After(r.lockedUntil) {
		r.count = 0
		r.lockedUntil = time.Time{}
	}
	r.count++
	if r.count >= maxAttempts {
		r.lockedUntil = time.Now().Add(time.Duration(lockoutMinutes) * time.Minute)
		r.count = 0
	}
}

// RecordFailedLoginForKeys records a failed attempt for all provided scopes.
func RecordFailedLoginForKeys(maxAttempts, lockoutMinutes int, keys ...string) {
	for _, key := range keys {
		if key != "" {
			RecordFailedLogin(key, maxAttempts, lockoutMinutes)
		}
	}
}

// ClearLoginRecord removes login failure tracking for an IP (on successful login).
func ClearLoginRecord(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginRecords, ip)
}

// startLoginRecordCleaner starts a background goroutine that periodically removes
// expired rate-limit records to prevent unbounded map growth.
func startLoginRecordCleaner(shutdownCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				loginMu.Lock()
				now := time.Now()
				for key, r := range loginRecords {
					r.mu.Lock()
					expired := r.count == 0 && (r.lockedUntil.IsZero() || now.After(r.lockedUntil))
					r.mu.Unlock()
					if expired {
						delete(loginRecords, key)
					}
				}
				loginMu.Unlock()
			case <-shutdownCh:
				return
			}
		}
	}()
}

// ClearLoginRecords removes rate-limit tracking for all provided scopes.
func ClearLoginRecords(keys ...string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	for _, key := range keys {
		if key != "" {
			delete(loginRecords, key)
		}
	}
}

// LoginBackoffDelay returns a small progressive delay for repeated failed logins.
func LoginBackoffDelay(keys ...string) time.Duration {
	maxCount := 0
	for _, key := range keys {
		if key == "" {
			continue
		}
		r := getLoginRecord(key)
		r.mu.Lock()
		count := r.count
		locked := time.Now().Before(r.lockedUntil)
		r.mu.Unlock()
		if locked {
			return 0
		}
		if count > maxCount {
			maxCount = count
		}
	}
	if maxCount <= 0 {
		return 0
	}
	delay := time.Duration(maxCount) * 250 * time.Millisecond
	if delay > 2*time.Second {
		return 2 * time.Second
	}
	return delay
}

// ClientIP extracts the real client IP, respecting X-Forwarded-For for reverse proxies.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); ip != "" {
			return ip
		}
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		return ip[:idx]
	}
	return ip
}

// ── Auth Middleware ──────────────────────────────────────────────────────────

// authBypassPrefixes lists URL prefixes that are always accessible without a session.
// NOTE: /api/personalities is intentionally NOT in this list — personality profile
// names are internal information and the login page does not need them.
// The setup wizard (/setup) calls this endpoint but auth is auto-disabled when no
// password is configured, so the setup flow still works without a bypass here.
var authBypassPrefixes = []string{
	"/api/health",
	"/api/i18n",
	"/auth/",
	"/api/auth/status",
	"/api/auth/logout",
	"/api/security/status",
	"/api/setup",
	"/api/openrouter/models",
	"/api/oauth/callback",
	"/api/ui-language",
	"/api/remote/ws",       // Remote agent WebSocket — has its own key-based auth
	"/api/remote/download", // Personalized binary download — generates an enrollment token
	"/api/invasion/ws",     // Egg WebSocket — has its own HMAC-based auth handshake
	"/mcp",
	"/api/n8n/", // n8n endpoints have their own Bearer token auth (see n8nAuthenticate)
	"/setup",
	"/shared.css",
	"/shared.js",
	"/css/login.css",
	"/js/login/",
	// Static media files are served inline in authenticated pages (audio player, image gallery,
	// document viewer). The browser's <audio>/<img>/<iframe> sub-resource requests may not
	// carry the session cookie in all browsers/configurations (e.g. after a server restart
	// that invalidated the session, or strict SameSite edge-cases). These paths are already
	// protected by obscurity (timestamp-based filenames) and are not sensitive API endpoints.
	"/files/audio/",
	"/files/generated_images/",
	"/files/documents/",
	"/manifest.json",
	"/sw.js",
	"/tailwind.min.js",
	"/chart.min.js",
}

func isAuthBypassed(path string) bool {
	for _, prefix := range authBypassPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	// Static image/icon assets needed for the login page
	return strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".ico")
}

// noPasswordPrefixes lists the only URL prefixes accessible when auth is enabled
// but no password has been configured yet. Everything else is hard-blocked.
// This prevents the server from being openly accessible while auth is "enabled"
// but the vault is missing or the password was lost.
var noPasswordPrefixes = []string{
	"/auth/",
	"/api/auth/status",
	"/api/auth/logout",
	"/api/auth/password", // allows setting the initial password
	"/api/security/status",
	"/api/setup",
	"/api/ui-language",
	"/setup",
	"/shared.css",
	"/shared.js",
	"/css/login.css",
	"/js/login/",
}

func isAllowedWithoutPassword(path string) bool {
	for _, prefix := range noPasswordPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".ico")
}

// authMiddleware wraps a handler and enforces session authentication when
// auth.enabled is true. API paths return 401 JSON; browser paths redirect to /auth/login.
func authMiddleware(s *Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		enabled := s.Cfg.Auth.Enabled
		secret := s.Cfg.Auth.SessionSecret
		passwordHash := s.Cfg.Auth.PasswordHash
		s.CfgMu.RUnlock()

		// SECURITY: if auth is enabled but no password is set, the server is locked down.
		// Only the minimal endpoints required to set a password remain accessible.
		// There is NO bypass — auth.enabled = true means the server is protected, period.
		if enabled && passwordHash == "" {
			s.Logger.Error("[Auth] LOCKDOWN: auth.enabled is true but no password is set in the vault. " +
				"All requests are blocked except the password-setup endpoints. " +
				"Set a password via /config (local network only) or the /api/auth/password endpoint.")
			if isAllowedWithoutPassword(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":"server_locked","message":"Auth is enabled but no password is set. Access is blocked until a password is configured."}`))
				return
			}
			http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
			return
		}

		if !enabled || isAuthBypassed(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Internal self-calls (missions, cron follow-ups) originate from loopback and carry
		// the X-Internal-FollowUp header together with a per-process crypto token.
		// Both checks are required: IP prevents token leakage from outside, token prevents
		// any local process from bypassing auth without knowledge of the token.
		if r.Header.Get("X-Internal-FollowUp") == "true" {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			if host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.") {
				tok := s.internalToken
				if tok != "" && hmac.Equal([]byte(r.Header.Get("X-Internal-Token")), []byte(tok)) {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		if IsAuthenticated(r, secret) {
			// CSRF: reject state-changing requests whose Origin does not match our host.
			if !isSafeMethod(r.Method) && !checkCSRFOrigin(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"csrf_check_failed","message":"Request origin does not match server host."}`))
				return
			}
			// Prevent browser from caching authenticated pages.
			// Without this, pressing Back after logout shows the cached page.
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")
			next.ServeHTTP(w, r)
			return
		}

		// JSON API or SSE stream: return 401
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") || r.URL.Path == "/events" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","redirect":"/auth/login"}`))
			return
		}

		// Browser: redirect to login with return path
		target := "/auth/login"
		if r.URL.Path != "/" {
			target += "?redirect=" + r.URL.RequestURI()
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// isSafeMethod returns true for HTTP methods that are read-only and cannot cause state changes.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// checkCSRFOrigin validates that the request Origin header matches the server's host.
// Returns true (allow) when:
//   - The Origin header is absent (non-browser clients, or same-origin requests
//     where some browsers omit it for first-party fetches).
//   - The Origin host matches the Host header (or X-Forwarded-Host when behind a proxy).
//
// An attacker-controlled page on a different origin cannot forge matching cookies with
// SameSite=Strict, but this check adds defence-in-depth for edge cases.
func checkCSRFOrigin(r *http.Request) bool {
	originHeader := r.Header.Get("Origin")
	if originHeader == "" {
		referer := r.Header.Get("Referer")
		if referer == "" {
			return false
		}
		parsedReferer, err := url.Parse(referer)
		if err != nil || parsedReferer.Host == "" {
			return false
		}
		serverHost := r.Header.Get("X-Forwarded-Host")
		if serverHost == "" {
			serverHost = r.Host
		}
		return strings.EqualFold(parsedReferer.Host, serverHost)
	}

	parsed, err := url.Parse(originHeader)
	if err != nil || parsed.Host == "" {
		// Malformed Origin header — reject.
		return false
	}

	// Determine the canonical server host: prefer X-Forwarded-Host (set by trusted
	// reverse proxies) then fall back to the Host header.
	serverHost := r.Header.Get("X-Forwarded-Host")
	if serverHost == "" {
		serverHost = r.Host
	}
	// Keep only the first host when X-Forwarded-Host contains a comma-separated list.
	if idx := strings.IndexByte(serverHost, ','); idx >= 0 {
		serverHost = strings.TrimSpace(serverHost[:idx])
	}

	return strings.EqualFold(parsed.Host, serverHost)
}

// GenerateRandomHex returns a cryptographically random hex string of n bytes (2n hex chars).
func GenerateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
