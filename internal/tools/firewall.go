package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"aurago/internal/config"
)

// sudoRun runs a command with sudo, optionally supplying a password via stdin
// (sudo -S) when sudoPassword is non-empty.  Falls back to passwordless sudo
// (sudo -n) when no password is provided.
func sudoRun(sudoPassword string, args ...string) ([]byte, error) {
	if sudoPassword != "" {
		fullArgs := append([]string{"-S"}, args...)
		cmd := exec.Command("sudo", fullArgs...)
		cmd.Stdin = strings.NewReader(sudoPassword + "\n")
		return cmd.CombinedOutput()
	}
	// No password — try non-interactive (NOPASSWD sudoers or running as root).
	fullArgs := append([]string{"-n"}, args...)
	cmd := exec.Command("sudo", fullArgs...)
	return cmd.CombinedOutput()
}

// FirewallGetRules returns the active firewall rules using iptables or ufw (Linux only).
// sudoPassword may be empty when the process already has direct access or NOPASSWD sudo.
func FirewallGetRules(sudoPassword string) (string, error) {
	// Try iptables first
	out, err := sudoRun(sudoPassword, "iptables", "-S")
	if err == nil {
		return string(out), nil
	}

	// Fallback to ufw
	out, err = sudoRun(sudoPassword, "ufw", "status", "verbose")
	if err == nil {
		return string(out), nil
	}

	return "", fmt.Errorf("failed to get firewall rules: no supported firewall found or missing sudo privileges. Output: %s", string(out))
}

// FirewallModifyRule executes a firewall modification command (Linux only).
// sudoPassword may be empty when the process already has direct access or NOPASSWD sudo.
func FirewallModifyRule(command, sudoPassword string) (string, error) {
	// Simple security check to avoid command injection although the LLM is trusted
	if !strings.HasPrefix(command, "iptables ") && !strings.HasPrefix(command, "ufw ") {
		return "", fmt.Errorf("invalid firewall command: must start with 'iptables' or 'ufw'")
	}

	args := strings.Fields(command)
	out, err := sudoRun(sudoPassword, args...)
	if err != nil {
		return "", fmt.Errorf("firewall modification failed: %v\nOutput: %s", err, string(out))
	}

	return string(out), nil
}

// StartFirewallGuard runs a background loop checking for firewall changes.
// sudoPassword is the optional vault-stored sudo password; pass an empty string
// when the process already has direct iptables access or NOPASSWD sudo.
func StartFirewallGuard(ctx context.Context, cfg *config.Config, logger *slog.Logger, sudoPassword string, triggerPrompt func(prompt string)) {
	if !cfg.Firewall.Enabled || cfg.Firewall.Mode != "guard" {
		return
	}
	if !cfg.Runtime.FirewallAccessOK && sudoPassword == "" {
		logger.Info("Firewall Guard disabled: no firewall access (running in Docker or iptables unavailable)")
		return
	}

	logger.Info("Starting Firewall Guard", "interval", cfg.Firewall.PollIntervalSeconds)

	interval := time.Duration(cfg.Firewall.PollIntervalSeconds) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second // Enforce minimum polling interval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastHash string

	// Initial fetch to set the baseline hash
	initialRules, err := FirewallGetRules(sudoPassword)
	if err != nil {
		logger.Warn("Firewall Guard failed to fetch initial rules", "error", err)
	} else {
		lastHash = hashRules(initialRules)
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping Firewall Guard")
			return
		case <-ticker.C:
			currentRules, err := FirewallGetRules(sudoPassword)
			if err != nil {
				logger.Warn("Firewall Guard check failed", "error", err)
				continue
			}

			currentHash := hashRules(currentRules)

			// If changed and it's not the very first run
			if lastHash != "" && currentHash != lastHash {
				logger.Warn("Firewall Guard detected rule changes! Waking agent.", "old_hash", lastHash, "new_hash", currentHash)

				prompt := fmt.Sprintf(`[URGENT] The Firewall Guard mode has detected changes in the system firewall rules!

Please investigate these current active rules to ensure they align with the system's security policies. 

Current Firewall Rules:
%s
`, currentRules)

				triggerPrompt(prompt)
			}
			lastHash = currentHash
		}
	}
}

func hashRules(rules string) string {
	hasher := sha256.New()
	hasher.Write([]byte(rules))
	return hex.EncodeToString(hasher.Sum(nil))
}
