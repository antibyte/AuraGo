package invasion

import (
	"aurago/internal/remote"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// SSHConnector deploys eggs to remote hosts via SSH/SFTP.
type SSHConnector struct{}

// The SSH connector reaches a nest only through these functions; tests
// replace them to fail single deploy steps.
var (
	sshRemoteCommand = remote.ExecuteRemoteCommand
	sshTransferFile  = remote.TransferFile
)

// sshEggIDPrefix returns the nest-ID prefix for SSH egg paths and services.
func sshEggIDPrefix(nestID string) (string, error) {
	return eggIDPrefix(nestID)
}

func sshEggBaseDir(nestID string) (string, error) {
	prefix, err := sshEggIDPrefix(nestID)
	if err != nil {
		return "", err
	}
	return "~/.aurago-egg-" + prefix, nil
}

func sshEggProcessPattern(nestID string) (string, error) {
	prefix, err := sshEggIDPrefix(nestID)
	if err != nil {
		return "", err
	}
	return ".aurago-egg-" + prefix + "/aurago", nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func shellPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return "$HOME/" + shellQuote(strings.TrimPrefix(path, "~/"))
	}
	return shellQuote(path)
}

func (c *SSHConnector) Validate(ctx context.Context, nest NestRecord, secret []byte) error {
	output, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, "echo ok")
	if err != nil {
		return fmt.Errorf("SSH validation failed: %w", err)
	}
	if !strings.Contains(output, "ok") {
		return fmt.Errorf("unexpected validation response: %s", output)
	}
	return nil
}

func (c *SSHConnector) Deploy(ctx context.Context, nest NestRecord, secret []byte, payload EggDeployPayload) error {
	if key, err := hex.DecodeString(payload.MasterKey); err != nil || len(key) != 32 {
		return configNotDelivered(fmt.Errorf("deployment requires a 32-byte hexadecimal master key"))
	}
	baseDir, err := sshEggBaseDir(nest.ID)
	if err != nil {
		return configNotDelivered(err)
	}
	backupDir := baseDir + ".bak"

	// 0. Backup existing deployment (if any)
	backupCmd := fmt.Sprintf("if [ -d %s ]; then rm -rf %s; cp -a %s %s; fi", shellPath(baseDir), shellPath(backupDir), shellPath(baseDir), shellPath(backupDir))
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, backupCmd); err != nil {
		return configNotDelivered(fmt.Errorf("failed to backup existing deployment: %w", err))
	}

	// 1. Create target directory
	mkdirCmd := fmt.Sprintf("mkdir -p %s %s %s", shellPath(baseDir+"/data"), shellPath(baseDir+"/log"), shellPath(baseDir+"/prompts"))
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, mkdirCmd); err != nil {
		return configNotDelivered(fmt.Errorf("failed to create directories: %w", err))
	}

	// 2. Transfer binary
	remoteBinary := baseDir + "/aurago"
	if err := sshTransferFile(ctx, nest.Host, nest.Port, nest.Username, secret, payload.BinaryPath, remoteBinary, "upload"); err != nil {
		return configNotDelivered(fmt.Errorf("failed to transfer binary: %w", err))
	}

	// chmod +x
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, "chmod +x "+shellPath(remoteBinary)); err != nil {
		return configNotDelivered(fmt.Errorf("failed to chmod binary: %w", err))
	}

	// Everything above leaves the running egg's config.yaml and .env alone;
	// from the config write on, the new shared key may be on the nest.

	// 3. Write config
	remoteConfig := baseDir + "/config.yaml"
	if err := writeSSHDeployFile(ctx, nest, secret, remoteConfig, payload.ConfigYAML); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// 4. Transfer resources.dat if available
	if payload.ResourcesPkg != "" {
		remoteRes := baseDir + "/resources.dat"
		if err := sshTransferFile(ctx, nest.Host, nest.Port, nest.Username, secret, payload.ResourcesPkg, remoteRes, "upload"); err != nil {
			return fmt.Errorf("failed to transfer resources: %w", err)
		}
		// Unpack resources
		unpackCmd := fmt.Sprintf("cd %s && tar -xzf resources.dat 2>/dev/null; rm -f resources.dat", shellPath(baseDir))
		if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, unpackCmd); err != nil {
			return fmt.Errorf("failed to unpack resources: %w", err)
		}
	}

	// 5. Write vault if included
	if payload.IncludeVault && len(payload.VaultData) > 0 {
		remoteVault := baseDir + "/data/vault.enc"
		if err := writeSSHDeployFile(ctx, nest, secret, remoteVault, payload.VaultData); err != nil {
			return fmt.Errorf("failed to write vault: %w", err)
		}
	}

	// 6. Write master key to .env
	envContent := fmt.Sprintf("AURAGO_MASTER_KEY=%s", payload.MasterKey)
	envPath := baseDir + "/.env"
	if err := writeSSHDeployFile(ctx, nest, secret, envPath, []byte(envContent+"\n")); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	// 7. Start the egg
	if payload.Permanent {
		return c.installService(ctx, nest, secret, baseDir)
	}
	return c.startProcess(ctx, nest, secret, baseDir)
}

func sshDeployFileScript(path string, data []byte) string {
	// Only the fixed bash -s command is sent as an SSH exec argument. Payload
	// bytes travel through stdin; mktemp/rename preserve the previous valid file.
	return "set -eu\numask 077\n" +
		"target=" + shellPath(path) + "\n" +
		"tmp=$(mktemp \"${target}.tmp.XXXXXXXX\")\n" +
		"trap 'rm -f -- \"$tmp\"' EXIT HUP INT TERM\n" +
		"base64 -d > \"$tmp\" <<'AURAGO_DEPLOY_DATA'\n" + base64.StdEncoding.EncodeToString(data) + "\nAURAGO_DEPLOY_DATA\n" +
		"chmod 600 -- \"$tmp\"\nmv -f -- \"$tmp\" \"$target\"\ntrap - EXIT HUP INT TERM\n"
}

func writeSSHDeployFile(ctx context.Context, nest NestRecord, secret []byte, path string, data []byte) error {
	_, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, "bash -s", strings.NewReader(sshDeployFileScript(path, data)))
	return err
}

func (c *SSHConnector) Stop(ctx context.Context, nest NestRecord, secret []byte) error {
	prefix, err := sshEggIDPrefix(nest.ID)
	if err != nil {
		return err
	}
	processPattern, err := sshEggProcessPattern(nest.ID)
	if err != nil {
		return err
	}
	serviceName := fmt.Sprintf("aurago-egg-%s", prefix)

	// Try systemd first
	stopCmd := fmt.Sprintf("systemctl --user stop %s 2>/dev/null || true", serviceName)
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, stopCmd); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Also kill any running process
	killCmd := fmt.Sprintf("pkill -f %s 2>/dev/null || true", shellQuote(processPattern))
	_, _ = sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, killCmd)

	return nil
}

func (c *SSHConnector) Status(ctx context.Context, nest NestRecord, secret []byte) (string, error) {
	prefix, err := sshEggIDPrefix(nest.ID)
	if err != nil {
		return "unknown", err
	}
	processPattern, err := sshEggProcessPattern(nest.ID)
	if err != nil {
		return "unknown", err
	}
	serviceName := fmt.Sprintf("aurago-egg-%s", prefix)

	// Check systemd service first
	output, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret,
		fmt.Sprintf("systemctl --user is-active %s 2>/dev/null || echo 'inactive'", serviceName))
	if err == nil && strings.TrimSpace(output) == "active" {
		return "running", nil
	}

	// Check for running process
	output, err = sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret,
		fmt.Sprintf("pgrep -f %s >/dev/null 2>&1 && echo running || echo stopped", shellQuote(processPattern)))
	if err != nil {
		return "unknown", err
	}
	return strings.TrimSpace(output), nil
}

func (c *SSHConnector) installService(ctx context.Context, nest NestRecord, secret []byte, baseDir string) error {
	prefix, err := sshEggIDPrefix(nest.ID)
	if err != nil {
		return err
	}
	serviceName := fmt.Sprintf("aurago-egg-%s", prefix)
	unitFile := fmt.Sprintf(`[Unit]
Description=AuraGo Egg Worker (%s)
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
EnvironmentFile=%s/.env
ExecStart=%s/aurago
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
`, prefix, baseDir, baseDir, baseDir)

	writeCmd := fmt.Sprintf("mkdir -p ~/.config/systemd/user && cat > ~/.config/systemd/user/%s.service << 'EOF'\n%s\nEOF", serviceName, unitFile)
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, writeCmd); err != nil {
		return fmt.Errorf("failed to write service unit: %w", err)
	}

	// restart, not start: start does nothing for an active unit, so a hatch
	// over a running egg service would keep the old process and its old key.
	startCmd := fmt.Sprintf("systemctl --user daemon-reload && systemctl --user enable %s && systemctl --user restart %s", serviceName, serviceName)
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// sshEggStopRunningScript stops the egg process a previous hatch started from
// baseDir before a new one starts: SIGTERM, up to 10 s to exit, then SIGKILL.
// It finds the process by its executable (<baseDir>/aurago, or "(deleted)"
// once the upload replaced the file), not by command line: a process-mode egg
// runs as "./aurago", and the remote shell's own command line would match.
func sshEggStopRunningScript(baseDir string) string {
	return "dir=" + shellPath(baseDir) + "; pids=; " +
		`for p in $(pgrep -u "$(id -u)" -x aurago 2>/dev/null); do ` +
		`case "$(readlink "/proc/$p/exe" 2>/dev/null)" in "$dir/aurago"|"$dir/aurago (deleted)") pids="$pids $p";; esac; done; ` +
		`if [ -n "$pids" ]; then kill -TERM $pids 2>/dev/null; alive=1; i=0; ` +
		`while [ $i -lt 50 ]; do alive=; for p in $pids; do kill -0 "$p" 2>/dev/null && alive=1; done; [ -z "$alive" ] && break; sleep 0.2; i=$((i+1)); done; ` +
		`if [ -n "$alive" ]; then kill -KILL $pids 2>/dev/null; fi; fi; `
}

func (c *SSHConnector) startProcess(ctx context.Context, nest NestRecord, secret []byte, baseDir string) error {
	// Stop an egg a previous hatch left running, so only the new process (with
	// the new config and shared key) runs. Then start it in the background
	// with nohup, output to the log. set -a exports the sourced .env to it.
	// Only nohup runs in the background, with stdin from /dev/null: a
	// backgrounded "cd && ... && nohup" list kept the SSH session's
	// stdout/stderr open, so the command never returned until the deploy
	// context expired.
	startCmd := sshEggStopRunningScript(baseDir) +
		fmt.Sprintf("cd %s && set -a && . ./.env && set +a && { nohup ./aurago > log/egg.log 2>&1 < /dev/null & echo $!; }", shellPath(baseDir))
	output, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd)
	if err != nil {
		return fmt.Errorf("failed to start egg process: %w", err)
	}
	_ = output // PID returned but not stored (we use process detection for status)
	return nil
}

func (c *SSHConnector) HealthCheck(ctx context.Context, nest NestRecord, secret []byte) error {
	processPattern, err := sshEggProcessPattern(nest.ID)
	if err != nil {
		return err
	}
	// Check if the egg process is running
	checkCmd := fmt.Sprintf("pgrep -f %s >/dev/null 2>&1 && echo ok || echo fail", shellQuote(processPattern))
	output, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, checkCmd)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	if !strings.Contains(output, "ok") {
		return fmt.Errorf("egg process not running")
	}
	return nil
}

// Reconfigure writes a patched config.yaml to the remote egg and restarts it.
// The egg process/container is stopped, the config is replaced, and then restarted.
func (c *SSHConnector) Reconfigure(ctx context.Context, nest NestRecord, secret []byte, configYAML []byte) error {
	baseDir, err := sshEggBaseDir(nest.ID)
	if err != nil {
		return err
	}
	remoteConfig := baseDir + "/config.yaml"

	// 1. Stop the running egg
	if err := c.Stop(ctx, nest, secret); err != nil {
		return fmt.Errorf("failed to stop egg for reconfigure: %w", err)
	}

	// 2. Publish the private config through encrypted SSH stdin.
	if err := writeSSHDeployFile(ctx, nest, secret, remoteConfig, configYAML); err != nil {
		return fmt.Errorf("failed to write patched config: %w", err)
	}

	// 3. Restart the egg (try systemd first, fall back to nohup)
	prefix, err := sshEggIDPrefix(nest.ID)
	if err != nil {
		return err
	}
	serviceName := fmt.Sprintf("aurago-egg-%s", prefix)
	restartCmd := fmt.Sprintf("systemctl --user restart %s 2>/dev/null || (cd %s && set -a && . ./.env && set +a && nohup ./aurago > log/egg.log 2>&1 &)", serviceName, shellPath(baseDir))
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, restartCmd); err != nil {
		return fmt.Errorf("failed to restart egg after reconfigure: %w", err)
	}

	return nil
}

func (c *SSHConnector) Rollback(ctx context.Context, nest NestRecord, secret []byte) error {
	baseDir, err := sshEggBaseDir(nest.ID)
	if err != nil {
		return err
	}
	backupDir := baseDir + ".bak"

	// Check if backup exists
	checkCmd := fmt.Sprintf("test -d %s && echo ok || echo missing", shellPath(backupDir))
	output, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, checkCmd)
	if err != nil {
		return fmt.Errorf("failed to check backup: %w", err)
	}
	if !strings.Contains(output, "ok") {
		return fmt.Errorf("no backup found for rollback")
	}

	// Stop current egg
	_ = c.Stop(ctx, nest, secret)

	// Replace current with backup
	restoreCmd := fmt.Sprintf("rm -rf %s && mv %s %s", shellPath(baseDir), shellPath(backupDir), shellPath(baseDir))
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, restoreCmd); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	// Restart the restored egg
	prefix, err := sshEggIDPrefix(nest.ID)
	if err != nil {
		return err
	}
	serviceName := fmt.Sprintf("aurago-egg-%s", prefix)
	startCmd := fmt.Sprintf("systemctl --user restart %s 2>/dev/null || (cd %s && set -a && source .env && set +a && nohup ./aurago > log/egg.log 2>&1 &)", serviceName, baseDir)
	if _, err := sshRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd); err != nil {
		return fmt.Errorf("failed to restart after rollback: %w", err)
	}

	return nil
}
