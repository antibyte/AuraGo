package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MasterKeyPath returns the secure location for the master key.
// If systemd is available: /etc/aurago/master.key
// Otherwise: <installDir>/.env
func MasterKeyPath(installDir string) string {
	if HasSystemd() {
		return "/etc/aurago/master.key"
	}
	return filepath.Join(installDir, ".env")
}

// GenerateMasterKey creates a new 32-byte (256-bit) hex-encoded master key.
func GenerateMasterKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate random key: %w", err)
	}
	return hex.EncodeToString(key), nil
}

// EnsureMasterKey ensures a master key exists.
// If systemd is available, writes to /etc/aurago/master.key (via sudo).
// Otherwise writes to <installDir>/.env.
// Returns the key and whether a new key was generated.
func EnsureMasterKey(installDir string) (key string, generated bool, err error) {
	// Check existing key in both locations
	for _, path := range []string{
		"/etc/aurago/master.key",
		filepath.Join(installDir, ".env"),
	} {
		if k := ReadEnvKey(path, "AURAGO_MASTER_KEY"); k != "" {
			return k, false, nil
		}
	}

	// Generate new key
	key, err = GenerateMasterKey()
	if err != nil {
		return "", false, err
	}

	content := fmt.Sprintf("AURAGO_MASTER_KEY=%s\n", key)

	if HasSystemd() {
		if err := writeMasterKeySecure(content); err != nil {
			// Fallback to .env if /etc/aurago is not writable
			envPath := filepath.Join(installDir, ".env")
			if err2 := os.WriteFile(envPath, []byte(content), 0600); err2 != nil {
				return "", false, fmt.Errorf("write master key: %w (fallback also failed: %v)", err, err2)
			}
			return key, true, nil
		}
	} else {
		envPath := filepath.Join(installDir, ".env")
		if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
			return "", false, fmt.Errorf("write .env: %w", err)
		}
	}

	return key, true, nil
}

// writeMasterKeySecure writes the master key to /etc/aurago/master.key
// with proper ownership and permissions.
func writeMasterKeySecure(content string) error {
	// Create /etc/aurago/ directory (mode 700, root:root)
	if err := sudoExec("mkdir", "-p", "/etc/aurago"); err != nil {
		return err
	}
	if err := sudoExec("chmod", "700", "/etc/aurago"); err != nil {
		return err
	}

	// Write key file via sudo tee
	cmd := exec.Command("sudo", "tee", "/etc/aurago/master.key")
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write /etc/aurago/master.key: %w", err)
	}

	// Set permissions: 0600, root:root
	if err := sudoExec("chmod", "600", "/etc/aurago/master.key"); err != nil {
		return err
	}
	if err := sudoExec("chown", "root:root", "/etc/aurago/master.key"); err != nil {
		return err
	}
	return nil
}

// MigrateMasterKey moves the master key from .env to /etc/aurago/master.key.
// Returns true if migration was performed.
func MigrateMasterKey(installDir string) (bool, error) {
	if !HasSystemd() {
		return false, nil
	}

	// Check if already migrated
	if k := ReadEnvKey("/etc/aurago/master.key", "AURAGO_MASTER_KEY"); k != "" {
		// Already in secure location; remove .env if it exists
		envPath := filepath.Join(installDir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			os.Remove(envPath)
		}
		return false, nil
	}

	// Read from .env
	envPath := filepath.Join(installDir, ".env")
	key := ReadEnvKey(envPath, "AURAGO_MASTER_KEY")
	if key == "" {
		return false, nil
	}

	// Write to secure location
	content := fmt.Sprintf("AURAGO_MASTER_KEY=%s\n", key)
	if err := writeMasterKeySecure(content); err != nil {
		return false, fmt.Errorf("migrate master key: %w", err)
	}

	// Remove old .env
	os.Remove(envPath)
	return true, nil
}

// ReadEnvKey reads a key=value from a file.
func ReadEnvKey(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimPrefix(line, prefix)
			return strings.Trim(val, "\"'")
		}
	}
	return ""
}

// sudoExec runs a command with sudo.
func sudoExec(args ...string) error {
	cmd := exec.Command("sudo", args...)
	return cmd.Run()
}
