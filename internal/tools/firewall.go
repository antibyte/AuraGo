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

// FirewallGetRules returns the active firewall rules using iptables or ufw (Linux only).
func FirewallGetRules() (string, error) {
	// Try iptables first
	cmd := exec.Command("sudo", "iptables", "-S")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), nil
	}

	// Fallback to ufw
	cmd = exec.Command("sudo", "ufw", "status", "verbose")
	out, err = cmd.CombinedOutput()
	if err == nil {
		return string(out), nil
	}

	return "", fmt.Errorf("failed to get firewall rules: no supported firewall found or missing sudo privileges. Output: %s", string(out))
}

// FirewallModifyRule executes a firewall modification command (Linux only).
func FirewallModifyRule(command string) (string, error) {
	// Simple security check to avoid command injection although the LLM is trusted
	if !strings.HasPrefix(command, "iptables ") && !strings.HasPrefix(command, "ufw ") {
		return "", fmt.Errorf("invalid firewall command: must start with 'iptables' or 'ufw'")
	}

	args := strings.Fields(command)
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command("sudo", cmdArgs...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("firewall modification failed: %v\nOutput: %s", err, string(out))
	}

	return string(out), nil
}

// StartFirewallGuard runs a background loop checking for firewall changes.
func StartFirewallGuard(ctx context.Context, cfg *config.Config, logger *slog.Logger, triggerPrompt func(prompt string)) {
	if !cfg.Firewall.Enabled || cfg.Firewall.Mode != "guard" {
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
	initialRules, err := FirewallGetRules()
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
			currentRules, err := FirewallGetRules()
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
