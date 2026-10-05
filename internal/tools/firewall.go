package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// sudoFirewallTimeout bounds one `sudo -n <firewall command>` run.
const sudoFirewallTimeout = 60 * time.Second

// sudoRun runs a firewall command with `sudo -n` and a closed stdin. It first
// tries without a ticket (root, NOPASSWD rule), so those hosts never trigger
// a sudo login. Only when that run exits non-zero and a password is given
// does it validate a ticket through `sudo -S -v` (the only process that ever
// reads the password) and retry the command once.
func sudoRun(sudoPassword string, args ...string) ([]byte, error) {
	out, err := runSudoFirewallCommand(args...)
	var exitErr *exec.ExitError
	if err == nil || sudoPassword == "" || !errors.As(err, &exitErr) {
		return out, err
	}
	lease, authOut, authErr := withSudoTicket(sudoPassword, ".")
	if authErr != nil {
		return []byte(authOut), authErr
	}
	defer lease.Release()
	out, err = runSudoFirewallCommand(args...)
	return out, lease.Explain(err)
}

// runSudoFirewallCommand runs `sudo -n args...` with a closed stdin, a filtered
// environment and a bounded runtime. A timeout is not an *exec.ExitError, so
// sudoRun does not retry it.
func runSudoFirewallCommand(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sudoFirewallTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", append([]string{"-n"}, args...)...)
	cmd.Stdin = nil
	ensureFilteredEnv(cmd)
	SetupCmd(cmd)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		err = fmt.Errorf("sudo %s timed out after %s: %w", strings.Join(args, " "), sudoFirewallTimeout, ctx.Err())
	}
	return []byte(security.Scrub(string(out))), err
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
