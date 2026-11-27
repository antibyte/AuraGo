package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	pathpkg "path"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Code Studio HTML previews are served from /code-studio-preview/<token>/<path>. The sandboxed page has an opaque
// origin and the session cookie is SameSite=Strict, so preview requests carry no session: the token is the
// capability. It is stateless (HMAC), names one workspace, expires after 12 hours and only grants read access to
// that workspace's files. The key is derived from the session secret with a purpose label, so a preview token can
// never pass as a desktop embed token or session cookie, and rotating the secret (password change, "log out
// everywhere") revokes every preview link. Without a session secret (login disabled) a random per-process key is
// used; links then end with a restart. There is no session binding: the sandboxed page sends no cookie to bind to.
const (
	codeStudioPreviewPrefix      = "/code-studio-preview/"
	codeStudioPreviewTokenTTL    = 12 * time.Hour
	codeStudioPreviewSSHTokenTTL = time.Hour // SSH workspaces: access can be revoked remotely, keep links short-lived
	codeStudioPreviewKeyLabel    = "aurago/code-studio-preview/v1"
	codeStudioPreviewTokenMaxLen = 512
)

var codeStudioPreviewWorkspacePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type codeStudioPreviewClaims struct {
	Version   int    `json:"v"`
	Workspace string `json:"ws"`
	Expires   int64  `json:"exp"`
}

var codeStudioPreviewProcessSecret struct {
	once  sync.Once
	value string
}

func codeStudioPreviewKey(secret string) ([]byte, error) {
	if strings.TrimSpace(secret) == "" {
		codeStudioPreviewProcessSecret.once.Do(func() {
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err == nil {
				codeStudioPreviewProcessSecret.value = hex.EncodeToString(raw)
			}
		})
		secret = codeStudioPreviewProcessSecret.value
		if secret == "" {
			return nil, errors.New("preview signing key unavailable")
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(codeStudioPreviewKeyLabel))
	return mac.Sum(nil), nil
}

func (s *Server) codeStudioPreviewSigningKey() ([]byte, error) {
	if s == nil || s.Cfg == nil {
		return nil, errors.New("server configuration unavailable")
	}
	s.CfgMu.RLock()
	secret := s.Cfg.Auth.SessionSecret
	s.CfgMu.RUnlock()
	return codeStudioPreviewKey(secret)
}

func codeStudioPreviewSignature(key []byte, payload string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func issueCodeStudioPreviewToken(key []byte, workspace string, now time.Time) (string, time.Time, error) {
	if len(key) == 0 {
		return "", time.Time{}, errors.New("preview signing key unavailable")
	}
	if !codeStudioPreviewWorkspacePattern.MatchString(workspace) {
		return "", time.Time{}, errors.New("invalid workspace id")
	}
	ttl := codeStudioPreviewTokenTTL
	if workspace != codeStudioLocalWorkspaceID {
		ttl = codeStudioPreviewSSHTokenTTL
	}
	expires := now.Add(ttl).Truncate(time.Second)
	payload, err := json.Marshal(codeStudioPreviewClaims{Version: 1, Workspace: workspace, Expires: expires.Unix()})
	if err != nil {
		return "", time.Time{}, err
	}
	part := base64.RawURLEncoding.EncodeToString(payload)
	return part + "." + codeStudioPreviewSignature(key, part), expires, nil
}

func parseCodeStudioPreviewToken(key []byte, token string, now time.Time) (codeStudioPreviewClaims, bool) {
	var claims codeStudioPreviewClaims
	if len(key) == 0 || token == "" || len(token) > codeStudioPreviewTokenMaxLen {
		return claims, false
	}
	part, signature, found := strings.Cut(token, ".")
	if !found || part == "" || signature == "" || strings.Contains(signature, ".") {
		return claims, false
	}
	// The encoded signatures are compared, so no two spellings of one MAC can both pass.
	if !hmac.Equal([]byte(signature), []byte(codeStudioPreviewSignature(key, part))) {
		return claims, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil || json.Unmarshal(raw, &claims) != nil {
		return codeStudioPreviewClaims{}, false
	}
	if claims.Version != 1 || !codeStudioPreviewWorkspacePattern.MatchString(claims.Workspace) || claims.Expires <= now.Unix() {
		return codeStudioPreviewClaims{}, false
	}
	return claims, true
}

// splitCodeStudioPreviewPath returns the token and the workspace-relative rest of a preview URL path. slash reports
// whether a "/" followed the token; /code-studio-preview/<token> alone redirects to .../<token>/.
func splitCodeStudioPreviewPath(urlPath string) (token, rel string, slash bool) {
	if !strings.HasPrefix(urlPath, codeStudioPreviewPrefix) {
		return "", "", false
	}
	return strings.Cut(urlPath[len(codeStudioPreviewPrefix):], "/")
}

// codeStudioPreviewVirtualPath maps the relative part of a preview URL to a /workspace path. A ".." segment, a
// backslash or a NUL byte is rejected before cleaning, so a decoded traversal cannot climb out of the workspace.
func codeStudioPreviewVirtualPath(rel string) (string, error) {
	if strings.ContainsAny(rel, "\\\x00") {
		return "", errors.New("invalid preview path")
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return "", errors.New("preview path escapes the workspace")
		}
	}
	return sanitizeCodeStudioPath(codeStudioWorkspaceRoot + "/" + rel)
}

// codeStudioPreviewBlocked reports paths the preview never serves. The token reaches the whole workspace and the
// previewed page runs scripts that may fetch() anything under the token and send it out (an SSH workspace can be
// rooted at ~ or /). A web page does not need dot files or private keys, so .env, .git, .ssh, .aws and friends are
// refused, as are key and keystore files anywhere. /.well-known/ stays reachable.
func codeStudioPreviewBlocked(vpath string) bool {
	rel := strings.TrimPrefix(strings.TrimPrefix(vpath, codeStudioWorkspaceRoot), "/")
	segments := strings.Split(rel, "/")
	for _, segment := range segments {
		if strings.HasPrefix(segment, ".") && segment != ".well-known" {
			return true
		}
	}
	base := strings.ToLower(segments[len(segments)-1])
	if strings.HasPrefix(base, "id_rsa") || strings.HasPrefix(base, "id_dsa") || strings.HasPrefix(base, "id_ecdsa") || strings.HasPrefix(base, "id_ed25519") {
		return true
	}
	switch pathpkg.Ext(base) {
	case ".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".kdbx", ".ppk", ".gpg", ".asc":
		return true
	}
	return false
}

func codeStudioPreviewEscapePath(rel string) string {
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
