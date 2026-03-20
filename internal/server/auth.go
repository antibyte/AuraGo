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

// ClearSessionCookie expires the session cookie immediately.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
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

// IsLockedOut returns true if the IP is currently in a lockout period.
func IsLockedOut(ip string) bool {
	r := getLoginRecord(ip)
	r.mu.Lock()
	defer r.mu.Unlock()
	return time.Now().Before(r.lockedUntil)
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

// ClearLoginRecord removes login failure tracking for an IP (on successful login).
func ClearLoginRecord(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginRecords, ip)
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
	"/auth/",
	"/api/auth/status",
	"/api/security/status",
	"/api/setup",
	"/api/openrouter/models",
	"/api/oauth/callback",
	"/api/ui-language",
	"/api/remote/ws",       // Remote agent WebSocket — has its own key-based auth
	"/api/remote/download", // Personalized binary download — generates an enrollment token
	"/api/invasion/ws",     // Egg WebSocket — has its own HMAC-based auth handshake
	"/mcp",
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

// authMiddleware wraps a handler and enforces session authentication when
// auth.enabled is true. API paths return 401 JSON; browser paths redirect to /auth/login.
func authMiddleware(s *Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		enabled := s.Cfg.Auth.Enabled
		secret := s.Cfg.Auth.SessionSecret
		passwordHash := s.Cfg.Auth.PasswordHash
		s.CfgMu.RUnlock()

		// Safety: if auth is enabled but no password has been set, bypass auth to prevent lockout.
		// This can happen when a user enables auth without first setting a password.
		if enabled && passwordHash == "" {
			s.Logger.Warn("[Auth] auth.enabled is true but no password_hash is set — auth bypassed to prevent lockout. Set a password in the Config UI.")
			enabled = false
		}

		if !enabled || isAuthBypassed(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Internal self-calls (missions, cron follow-ups) originate from loopback and carry
		// the X-Internal-FollowUp header. Let them bypass auth — external clients cannot
		// spoof the loopback source address.
		if r.Header.Get("X-Internal-FollowUp") == "true" {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			if host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.") {
				next.ServeHTTP(w, r)
				return
			}
		}

		if IsAuthenticated(r, secret) {
			// Prevent browser from caching authenticated pages.
			// Without this, pressing Back after logout shows the cached page.
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")
			next.ServeHTTP(w, r)
			return
		}

		// JSON API: return 401
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") {
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

// GenerateRandomHex returns a cryptographically random hex string of n bytes (2n hex chars).
func GenerateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
