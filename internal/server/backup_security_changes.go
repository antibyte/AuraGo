package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/cydcreator"
	"aurago/internal/restorepending"
	"aurago/internal/security"

	"gopkg.in/yaml.v3"
)

const (
	securityChangeExampleCap = 8
	securityChangeTokenCap   = 20
)

// restoreDirs returns the configured directories of the ACTIVE config. Only
// this config decides where restored files go; directory values inside the
// archive's own config.yaml are never used.
func (s *Server) restoreDirs() restorepending.Dirs {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	d := s.Cfg.Directories
	return restorepending.Dirs{
		VectorDB:    d.VectorDBDir,
		Prompts:     d.PromptsDir,
		Skills:      d.SkillsDir,
		AgentSkills: d.AgentSkillsDir,
		Tools:       d.ToolsDir,
		Workspace:   d.WorkspaceDir,
	}
}

// validateRestoredTokenScopes rejects a staged token store with scopes that the
// server never checks. The CYD device scope is minted for flashed boards and is
// legitimately present in own backups.
func validateRestoredTokenScopes(tokens []security.Token) error {
	for _, t := range tokens {
		for _, scope := range t.Scopes {
			if scope == cydcreator.TokenScopeDevice || isKnownTokenScope(scope) {
				continue
			}
			return fmt.Errorf("unknown token scope %q in token %q", scope, t.ID)
		}
	}
	return nil
}

// readTokenFile decrypts a vault-encrypted token store file. A missing or empty
// file is an empty store.
func readTokenFile(vault *security.Vault, p string) ([]security.Token, error) {
	if vault == nil {
		return nil, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	plain, err := vault.DecryptBytes(data)
	if err != nil {
		return nil, err
	}
	var tokens []security.Token
	if err := json.Unmarshal(plain, &tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

type stagedConfigSecurity struct {
	Auth struct {
		Enabled             bool `yaml:"enabled"`
		TOTPEnabled         bool `yaml:"totp_enabled"`
		RequireOriginHeader bool `yaml:"require_origin_header"`
	} `yaml:"auth"`
	Server struct {
		Host string `yaml:"host"`
	} `yaml:"server"`
}

func sameScopes(a, b []string) bool {
	x, y := append([]string{}, a...), append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "\x00") == strings.Join(y, "\x00")
}

type cronJobLite struct {
	ID         string `json:"id"`
	CronExpr   string `json:"cron_expr"`
	TaskPrompt string `json:"task_prompt"`
	Disabled   bool   `json:"disabled"`
}

func readCronJobs(p string) map[string]cronJobLite {
	out := map[string]cronJobLite{}
	data, err := os.ReadFile(p)
	if err != nil {
		return out
	}
	var jobs []cronJobLite
	if json.Unmarshal(data, &jobs) != nil {
		return out
	}
	for _, j := range jobs {
		out[j.ID] = j
	}
	return out
}

// crontabChange counts new or changed cron jobs: their task prompts run
// autonomously.
func crontabChange(entries []agoImportEntry) *restorepending.FileChange {
	for _, e := range entries {
		if e.isDir || filepath.ToSlash(e.clean) != "data/crontab.json" {
			continue
		}
		fc := &restorepending.FileChange{Category: "crontab"}
		current := readCronJobs(e.dest)
		for id, job := range readCronJobs(e.stage) {
			prev, ok := current[id]
			switch {
			case !ok:
				fc.New++
			case prev != job:
				fc.Changed++
			default:
				continue
			}
			if len(fc.Examples) < securityChangeExampleCap {
				fc.Examples = append(fc.Examples, id)
			}
		}
		if fc.New+fc.Changed > 0 {
			return fc
		}
	}
	return nil
}

// credentialDatabases hold connection targets, credentials or remote-control
// endpoints; a changed copy is reported (not every database change is).
var credentialDatabases = map[string]bool{
	"sql_connections.db": true, "remote_control.db": true, "inventory.db": true, "invasion.db": true, "mailer.db": true,
}

func backupFileDigest(p string) (string, bool) {
	f, err := os.Open(p)
	if err != nil {
		return "", false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false
	}
	return string(h.Sum(nil)), true
}

func databaseChange(entries []agoImportEntry) *restorepending.FileChange {
	fc := &restorepending.FileChange{Category: "databases"}
	for _, e := range entries {
		base := strings.ToLower(filepath.Base(e.clean))
		if e.isDir || !credentialDatabases[base] {
			continue
		}
		staged, ok := backupFileDigest(e.stage)
		if !ok {
			continue
		}
		cur, exists := backupFileDigest(e.dest)
		switch {
		case exists && cur == staged:
			continue
		case exists:
			fc.Changed++
		default:
			fc.New++
		}
		fc.Examples = append(fc.Examples, base)
	}
	if fc.New+fc.Changed == 0 {
		return nil
	}
	return fc
}

func restoreFileCategory(clean string) string {
	slash := filepath.ToSlash(clean)
	switch {
	case strings.HasPrefix(slash, "prompts/"):
		return "prompts"
	case strings.HasPrefix(slash, "agent_workspace/skills/"):
		return "skills"
	case strings.HasPrefix(slash, "agent_workspace/agent_skills/"):
		return "agent_skills"
	case strings.HasPrefix(slash, "agent_workspace/tools/"):
		return "tools"
	}
	return ""
}

// agoComputeSecurityChanges compares the staged restore with the current state.
// stagedVault is the decrypted vault snapshot (nil when none was staged);
// stagedTokens the staged token store (nil when none).
func (s *Server) agoComputeSecurityChanges(entries []agoImportEntry, stagedVault map[string]string, stagedTokens []security.Token, currentTokenFile string) *restorepending.SecurityChanges {
	ch := &restorepending.SecurityChanges{}

	s.CfgMu.RLock()
	curAuth := s.Cfg.Auth.Enabled
	curHost := strings.TrimSpace(s.Cfg.Server.Host)
	curHash := s.Cfg.Auth.PasswordHash
	curTOTP := s.Cfg.Auth.TOTPEnabled
	curRequireOrigin := s.Cfg.Auth.RequireOriginHeader
	curTOTPSecret := s.Cfg.Auth.TOTPSecret
	s.CfgMu.RUnlock()

	for _, e := range entries {
		if e.isDir || filepath.ToSlash(e.clean) != "config.yaml" {
			continue
		}
		raw, err := os.ReadFile(e.stage)
		if err != nil {
			break
		}
		var staged stagedConfigSecurity
		if yaml.Unmarshal(raw, &staged) != nil {
			break
		}
		if curAuth && !staged.Auth.Enabled {
			ch.AuthDisabled = true
		}
		if curTOTP && !staged.Auth.TOTPEnabled {
			ch.TOTPDisabled = true
		}
		if curRequireOrigin && !staged.Auth.RequireOriginHeader {
			ch.RequireOriginOff = true
		}
		if h := strings.TrimSpace(staged.Server.Host); h != curHost {
			ch.HostChanged, ch.HostFrom, ch.HostTo = true, curHost, h
		}
		break
	}

	// Vault-only auth values; names come from vaultAuthKeys. Only booleans leave
	// this function, never the values.
	differs := func(field, current string) bool {
		staged := stagedVault[vaultAuthKeys[field]]
		if staged == "" {
			return false
		}
		if current == "" && s.Vault != nil {
			current, _ = s.Vault.ReadSecret(vaultAuthKeys[field])
		}
		return staged != current
	}
	ch.PasswordHashChanged = differs("password_hash", curHash)
	ch.TOTPSecretChanged = differs("totp_secret", curTOTPSecret)
	ch.WebAuthnChanged = differs("webauthn_state", "")

	if len(stagedTokens) > 0 {
		ch.TokenCount = len(stagedTokens)
		type tokenState struct {
			hash   string
			scopes []string
		}
		current := map[string]tokenState{}
		if existing, err := readTokenFile(s.Vault, currentTokenFile); err == nil {
			for _, t := range existing {
				current[t.ID] = tokenState{t.TokenHash, t.Scopes}
			}
		}
		for _, t := range stagedTokens {
			if prev, ok := current[t.ID]; ok && prev.hash == t.TokenHash && sameScopes(prev.scopes, t.Scopes) {
				continue
			}
			ch.NewTokenCount++
			if len(ch.NewTokens) < securityChangeTokenCap {
				ch.NewTokens = append(ch.NewTokens, restorepending.TokenInfo{ID: t.ID, Name: t.Name, Scopes: append([]string{}, t.Scopes...)})
			}
		}
	}

	byCat := map[string]*restorepending.FileChange{}
	for _, e := range entries {
		cat := restoreFileCategory(e.clean)
		if cat == "" || e.isDir {
			continue
		}
		staged, err := os.ReadFile(e.stage)
		if err != nil {
			continue
		}
		fc := byCat[cat]
		if fc == nil {
			fc = &restorepending.FileChange{Category: cat}
			byCat[cat] = fc
		}
		current, err := os.ReadFile(e.dest)
		switch {
		case err != nil && os.IsNotExist(err):
			fc.New++
		case err != nil:
			fc.Changed++
		case bytes.Equal(current, staged):
			continue
		default:
			fc.Changed++
		}
		if len(fc.Examples) < securityChangeExampleCap {
			fc.Examples = append(fc.Examples, filepath.ToSlash(e.clean))
		}
	}
	if fc := crontabChange(entries); fc != nil {
		byCat[fc.Category] = fc
	}
	if fc := databaseChange(entries); fc != nil {
		byCat[fc.Category] = fc
	}
	cats := make([]string, 0, len(byCat))
	for cat, fc := range byCat {
		if fc.New+fc.Changed > 0 {
			cats = append(cats, cat)
		}
	}
	sort.Strings(cats)
	for _, cat := range cats {
		ch.Files = append(ch.Files, *byCat[cat])
	}
	return ch
}

// ── B10: rebinding-safe host for the backup endpoints ───────────────────────

func hostnameOnly(authority string) string {
	authority = strings.TrimSpace(authority)
	if h, _, err := net.SplitHostPort(authority); err == nil {
		return strings.ToLower(strings.Trim(h, "[]"))
	}
	return strings.ToLower(strings.Trim(authority, "[]"))
}

func configuredHostMatches(host, configured string) bool {
	configured = strings.ToLower(strings.TrimSpace(configured))
	if configured == "" {
		return false
	}
	if strings.Contains(configured, "://") {
		configured = strings.TrimPrefix(configured, configured[:strings.Index(configured, "://")+3])
	}
	configured = hostnameOnly(strings.SplitN(configured, "/", 2)[0])
	return configured != "" && host == configured
}

// rebindingSafeHost reports whether the Host the browser used cannot have been
// chosen by a DNS-rebinding page: IP literals, localhost names, or a name the
// operator configured.
func (s *Server) rebindingSafeHost(r *http.Request) bool {
	host := hostnameOnly(requestHost(r))
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if name, err := os.Hostname(); err == nil && name != "" && !strings.Contains(name, ".") {
		name = strings.ToLower(name)
		if host == name || host == name+".local" {
			return true
		}
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	c := s.Cfg
	for _, configured := range []string{
		c.Server.HTTPS.Domain, c.SecurityProxy.Domain, c.Server.OAuthRedirectBaseURL, c.Server.PreviewDomain,
	} {
		if configuredHostMatches(host, configured) {
			return true
		}
	}
	// Tailscale: only when tsnet is enabled, the bare single-label name or the
	// exact MagicDNS name reported by the running node. Never a prefix match: the
	// default name is "aurago", and "aurago.attacker.test" must not pass.
	if c.Tailscale.TsNet.Enabled {
		if ts := strings.ToLower(strings.TrimSpace(c.Tailscale.TsNet.Hostname)); ts != "" && !strings.Contains(ts, ".") && host == ts {
			return true
		}
		if s.TsNetManager != nil {
			if dns := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s.TsNetManager.GetStatus().DNS)), "."); dns != "" && host == dns {
				return true
			}
		}
	}
	return false
}

// requireRebindingSafeHostWithoutLogin protects the backup endpoints (vault
// export is a decryption oracle for the chosen password) while login is
// disabled: a DNS-rebinding page reaches the server under its own host name and
// would otherwise pass the Origin/Host comparison. With login enabled, or for
// an authenticated admin Bearer token, nothing changes.
func requireRebindingSafeHostWithoutLogin(s *Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, isBearer := bearerCredential(r.Header.Get("Authorization")); isBearer {
			next.ServeHTTP(w, r)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		s.CfgMu.RUnlock()
		if authEnabled || s.rebindingSafeHost(r) {
			next.ServeHTTP(w, r)
			return
		}
		s.Logger.Warn("[Backup] Refused backup endpoint without login on an untrusted host name", "host", requestHost(r), "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "login_required_for_host",
			"message": "Backup und Wiederherstellung sind ohne Login nur über eine IP-Adresse oder localhost erreichbar. Aktivieren Sie den Login oder öffnen Sie AuraGo über die IP-Adresse (enable login or open AuraGo by IP address).",
		})
	})
}
