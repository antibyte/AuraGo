package rustfs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
)

func randomHex(n int) string {
	sum := sha256.Sum256([]byte(rand.Text() + rand.Text()))
	return hex.EncodeToString(sum[:n])
}
func registerState(s persistedState) {
	for _, value := range []string{s.RootAccess, s.RootSecret, s.ParentAccess, s.ParentSecret, s.ChildAccess, s.ChildSecret} {
		security.RegisterSensitive(value)
	}
}
func loadState(v secretStore) (persistedState, error) {
	if v == nil {
		return persistedState{}, failure("vault_unavailable", nil)
	}
	raw, err := v.ReadSecret(stateKey)
	if errors.Is(err, security.ErrSecretNotFound) {
		return persistedState{}, nil
	}
	if err != nil {
		return persistedState{}, failure("recovery_required", nil)
	}
	var s persistedState
	if len(raw) > 32<<10 || json.Unmarshal([]byte(raw), &s) != nil {
		return s, failure("recovery_required", nil)
	}
	registerState(s)
	if s.Instance == "" || s.RootAccess == "" || s.RootSecret == "" || s.ParentAccess == "" || s.ParentSecret == "" || s.ChildAccess == "" || s.ChildSecret == "" || s.PolicyName == "" || s.RootSecret == "rustfsadmin" {
		return s, failure("recovery_required", nil)
	}
	return s, nil
}
func newState() persistedState {
	id := randomHex(12)
	s := persistedState{Instance: id, DesiredRunning: true, RootAccess: "AGROOT" + strings.ToUpper(randomHex(10)), RootSecret: randomHex(32), ParentAccess: "AGPARENT" + strings.ToUpper(randomHex(10)), ParentSecret: randomHex(32), ChildAccess: "AGS3" + strings.ToUpper(randomHex(10)), ChildSecret: randomHex(32), PolicyName: "aurago-" + id}
	registerState(s)
	return s
}
func (m *Manager) saveState(s persistedState) error {
	registerState(s)
	data, err := json.Marshal(s)
	if err != nil {
		return failure("state_save_failed", nil)
	}
	if err = m.vault.WriteSecret(stateKey, string(data)); err != nil {
		return failure("state_save_failed", nil)
	}
	m.mu.Lock()
	m.state = s
	m.mu.Unlock()
	return nil
}
func hashValues(values ...any) string {
	data, _ := json.Marshal(values)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func copyConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		cfg = &config.Config{}
	}
	out := *cfg
	out.RustFS.Permissions = clonePermissions(cfg.RustFS.Permissions)
	if out.S3.Provider == "" {
		out.S3.Provider = config.S3ProviderExternal
	}
	if out.RustFS.Bucket == "" {
		out.RustFS.Bucket = "aurago"
	}
	return &out
}
func targetFingerprint(c *config.Config) string {
	if c.S3.Provider == config.S3ProviderRustFS {
		return hashValues(c.S3.Provider, c.RustFS.Bucket, c.Docker.Host, c.Runtime.IsDocker)
	}
	return hashValues(c.S3.Provider, c.S3.Endpoint, c.S3.Region, c.S3.Bucket, c.S3.UsePathStyle, c.S3.Insecure, c.S3.AccessKey, c.S3.SecretKey)
}
func localRootsFingerprint(c *config.Config) string {
	return hashValues(c.Directories.WorkspaceDir, c.Directories.DataDir)
}
func externalScopeFingerprint(c *config.Config) string {
	return hashValues(c.S3.Provider, c.S3.Endpoint, c.S3.Region, c.S3.Bucket, c.S3.UsePathStyle, c.S3.Insecure, c.S3.AccessKey, c.S3.SecretKey, c.S3.Enabled, c.S3.ReadOnly, c.Agent.AllowFilesystemWrite, localRootsFingerprint(c))
}
func configFingerprint(c *config.Config) string {
	return hashValues(targetFingerprint(c), c.S3.Enabled, c.S3.ReadOnly, c.RustFS, c.Docker.Enabled, c.Docker.ReadOnly, c.Docker.Host, c.Runtime.IsDocker, c.Agent.AllowFilesystemWrite, c.Directories.WorkspaceDir, c.Directories.DataDir)
}
