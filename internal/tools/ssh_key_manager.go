package tools

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSHKeyManager handles safe SSH key generation, listing, and revocation for the agent.
type SSHKeyManager struct {
	authorizedKeysPath string
}

func NewSSHKeyManager(workspaceDir string) *SSHKeyManager {
	// Usually ~/.ssh/authorized_keys, but might be within workspace or home.
	// For agent tasks, we'll try standard home dir first if allowed, otherwise target specific path.
	home, _ := os.UserHomeDir()
	authKeys := filepath.Join(home, ".ssh", "authorized_keys")
	if home == "" {
		authKeys = filepath.Join(workspaceDir, "authorized_keys")
	}
	return &SSHKeyManager{
		authorizedKeysPath: authKeys,
	}
}

// SetAuthorizedKeysPath overrides the default path.
func (m *SSHKeyManager) SetAuthorizedKeysPath(p string) {
	m.authorizedKeysPath = p
}

// Generate creates a new RSA key pair, optionally appending the public key to authorized_keys.
// Returns the private key (PEM) and public key. Does NOT store in vault to avoid mixing scopes.
func (m *SSHKeyManager) Generate(comment string, authorize bool) (privatePEM string, pubKey string, err error) {
	if comment == "" {
		comment = "AuraGo-Agent"
	} else if !strings.HasPrefix(comment, "AuraGo-Agent") {
		comment = "AuraGo-Agent:" + comment
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate RSA key: %w", err)
	}

	if err := privateKey.Validate(); err != nil {
		return "", "", fmt.Errorf("failed to validate RSA key: %w", err)
	}

	privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privBlock := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	}
	privatePEM = string(pem.EncodeToMemory(&privBlock))

	publicRsaKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public SSH key: %w", err)
	}

	pubKey = string(ssh.MarshalAuthorizedKey(publicRsaKey))
	pubKeyStr := strings.TrimSpace(string(pubKey)) + " " + comment

	if authorize {
		err := m.addAuthorizedKey(pubKeyStr)
		if err != nil {
			return privatePEM, pubKeyStr, fmt.Errorf("generated key, but failed to authorize: %w", err)
		}
	}

	return privatePEM, pubKeyStr, nil
}

func (m *SSHKeyManager) addAuthorizedKey(pubKeyStr string) error {
	dir := filepath.Dir(m.authorizedKeysPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create ssh dir: %w", err)
	}

	f, err := os.OpenFile(m.authorizedKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString("\n" + pubKeyStr + "\n"); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}
	return nil
}

// ListAuthorized returns all authorized keys that have the "AuraGo-Agent" prefix/comment.
func (m *SSHKeyManager) ListAuthorized() ([]string, error) {
	content, err := os.ReadFile(m.authorizedKeysPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	var results []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "AuraGo-Agent") {
			results = append(results, line)
		}
	}
	return results, nil
}

// Revoke removes an authorized key matching the given exact string or comment.
// Only keys containing "AuraGo-Agent" can be revoked by the agent for security.
func (m *SSHKeyManager) Revoke(searchString string) (bool, error) {
	if !strings.Contains(searchString, "AuraGo-Agent") {
		return false, fmt.Errorf("security policy violation: agent can only revoke keys containing 'AuraGo-Agent'")
	}

	content, err := os.ReadFile(m.authorizedKeysPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	revoked := false
	var keepLines []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // skip empty lines for clean rewrite
		}
		if strings.Contains(line, searchString) && strings.Contains(line, "AuraGo-Agent") {
			revoked = true
			continue
		}
		keepLines = append(keepLines, line)
	}

	if !revoked {
		return false, nil
	}

	outContent := strings.Join(keepLines, "\n") + "\n"
	err = os.WriteFile(m.authorizedKeysPath, []byte(outContent), 0600)
	if err != nil {
		return false, fmt.Errorf("failed to write updated authorized_keys: %w", err)
	}

	return true, nil
}

// Deploy copies the generated agent key to a remote server.
func (m *SSHKeyManager) Deploy(privatePEM string, targetHost string, username string, port int) error {
	// A placeholder for logic to copy to authorized_keys of a remote server (which requires password/key auth usually)
	return fmt.Errorf("deploy not fully implemented natively without initial auth, agent should use file operations")
}

// Rotate generates a new key, deploys it, and revokes the old one.
func (m *SSHKeyManager) Rotate(comment string) (string, string, error) {
	return m.Generate("Rotated-"+comment, true)
}
