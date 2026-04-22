package invasion

import (
	"aurago/internal/remote"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// SSHConnector deploys eggs to remote hosts via SSH/SFTP.
type SSHConnector struct{}

func (c *SSHConnector) Validate(ctx context.Context, nest NestRecord, secret []byte) error {
	output, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, "echo ok")
	if err != nil {
		return fmt.Errorf("SSH validation failed: %w", err)
	}
	if !strings.Contains(output, "ok") {
		return fmt.Errorf("unexpected validation response: %s", output)
	}
	return nil
}

func (c *SSHConnector) Deploy(ctx context.Context, nest NestRecord, secret []byte, payload EggDeployPayload) error {
	baseDir := fmt.Sprintf("~/.aurago-egg-%s", nest.ID[:8])
	backupDir := baseDir + ".bak"

	// 0. Backup existing deployment (if any)
	backupCmd := fmt.Sprintf("if [ -d %s ]; then rm -rf %s; cp -a %s %s; fi", baseDir, backupDir, baseDir, backupDir)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, backupCmd); err != nil {
		return fmt.Errorf("failed to backup existing deployment: %w", err)
	}

	// 1. Create target directory
	mkdirCmd := fmt.Sprintf("mkdir -p %s/data %s/log %s/prompts", baseDir, baseDir, baseDir)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, mkdirCmd); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// 2. Transfer binary
	remoteBinary := baseDir + "/aurago"
	if err := remote.TransferFile(ctx, nest.Host, nest.Port, nest.Username, secret, payload.BinaryPath, remoteBinary, "upload"); err != nil {
		return fmt.Errorf("failed to transfer binary: %w", err)
	}

	// chmod +x
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, "chmod +x "+remoteBinary); err != nil {
		return fmt.Errorf("failed to chmod binary: %w", err)
	}

	// 3. Write config
	remoteConfig := baseDir + "/config.yaml"
	// Use base64 encoding to safely transfer config content without shell escaping issues
	configB64 := base64.StdEncoding.EncodeToString(payload.ConfigYAML)
	writeCmd := fmt.Sprintf("echo '%s' | base64 -d > %s", configB64, remoteConfig)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, writeCmd); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// 4. Transfer resources.dat if available
	if payload.ResourcesPkg != "" {
		remoteRes := baseDir + "/resources.dat"
		if err := remote.TransferFile(ctx, nest.Host, nest.Port, nest.Username, secret, payload.ResourcesPkg, remoteRes, "upload"); err != nil {
			return fmt.Errorf("failed to transfer resources: %w", err)
		}
		// Unpack resources
		unpackCmd := fmt.Sprintf("cd %s && tar -xzf resources.dat 2>/dev/null; rm -f resources.dat", baseDir)
		if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, unpackCmd); err != nil {
			return fmt.Errorf("failed to unpack resources: %w", err)
		}
	}

	// 5. Write vault if included
	if payload.IncludeVault && len(payload.VaultData) > 0 {
		remoteVault := baseDir + "/data/vault.enc"
		// Write vault bytes via base64
		vaultWriteCmd := fmt.Sprintf("echo '%s' | base64 -d > %s", base64.StdEncoding.EncodeToString(payload.VaultData), remoteVault)
		if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, vaultWriteCmd); err != nil {
			return fmt.Errorf("failed to write vault: %w", err)
		}
	}

	// 6. Write master key to .env
	envContent := fmt.Sprintf("AURAGO_MASTER_KEY=%s", payload.MasterKey)
	envCmd := fmt.Sprintf("echo '%s' > %s/.env && chmod 600 %s/.env", envContent, baseDir, baseDir)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, envCmd); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	// 7. Start the egg
	if payload.Permanent {
		return c.installService(ctx, nest, secret, baseDir)
	}
	return c.startProcess(ctx, nest, secret, baseDir)
}

func (c *SSHConnector) Stop(ctx context.Context, nest NestRecord, secret []byte) error {
	baseDir := fmt.Sprintf("~/.aurago-egg-%s", nest.ID[:8])
	serviceName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])

	// Try systemd first
	stopCmd := fmt.Sprintf("systemctl --user stop %s 2>/dev/null || true", serviceName)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, stopCmd); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Also kill any running process
	killCmd := fmt.Sprintf("pkill -f '%s/aurago' 2>/dev/null || true", baseDir)
	_, _ = remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, killCmd)

	return nil
}

func (c *SSHConnector) Status(ctx context.Context, nest NestRecord, secret []byte) (string, error) {
	baseDir := fmt.Sprintf("~/.aurago-egg-%s", nest.ID[:8])
	serviceName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])

	// Check systemd service first
	output, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret,
		fmt.Sprintf("systemctl --user is-active %s 2>/dev/null || echo 'inactive'", serviceName))
	if err == nil && strings.TrimSpace(output) == "active" {
		return "running", nil
	}

	// Check for running process
	output, err = remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret,
		fmt.Sprintf("pgrep -f '%s/aurago' >/dev/null 2>&1 && echo running || echo stopped", baseDir))
	if err != nil {
		return "unknown", err
	}
	return strings.TrimSpace(output), nil
}

func (c *SSHConnector) installService(ctx context.Context, nest NestRecord, secret []byte, baseDir string) error {
	serviceName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
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
`, nest.ID[:8], baseDir, baseDir, baseDir)

	writeCmd := fmt.Sprintf("mkdir -p ~/.config/systemd/user && cat > ~/.config/systemd/user/%s.service << 'EOF'\n%s\nEOF", serviceName, unitFile)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, writeCmd); err != nil {
		return fmt.Errorf("failed to write service unit: %w", err)
	}

	startCmd := fmt.Sprintf("systemctl --user daemon-reload && systemctl --user enable %s && systemctl --user start %s", serviceName, serviceName)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func (c *SSHConnector) startProcess(ctx context.Context, nest NestRecord, secret []byte, baseDir string) error {
	// Start in background with nohup, redirect output to log.
	// set -a exports all sourced variables to child processes.
	startCmd := fmt.Sprintf("cd %s && set -a && source .env && set +a && nohup ./aurago > log/egg.log 2>&1 & echo $!", baseDir)
	output, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd)
	if err != nil {
		return fmt.Errorf("failed to start egg process: %w", err)
	}
	_ = output // PID returned but not stored (we use process detection for status)
	return nil
}

func (c *SSHConnector) HealthCheck(ctx context.Context, nest NestRecord, secret []byte) error {
	baseDir := fmt.Sprintf("~/.aurago-egg-%s", nest.ID[:8])
	// Check if the egg process is running
	checkCmd := fmt.Sprintf("pgrep -f '%s/aurago' >/dev/null 2>&1 && echo ok || echo fail", baseDir)
	output, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, checkCmd)
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
	baseDir := fmt.Sprintf("~/.aurago-egg-%s", nest.ID[:8])
	remoteConfig := baseDir + "/config.yaml"

	// 1. Stop the running egg
	if err := c.Stop(ctx, nest, secret); err != nil {
		return fmt.Errorf("failed to stop egg for reconfigure: %w", err)
	}

	// 2. Write the new config via base64 to avoid shell escaping issues
	configB64 := base64.StdEncoding.EncodeToString(configYAML)
	writeCmd := fmt.Sprintf("echo '%s' | base64 -d > %s", configB64, remoteConfig)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, writeCmd); err != nil {
		return fmt.Errorf("failed to write patched config: %w", err)
	}

	// 3. Restart the egg (try systemd first, fall back to nohup)
	serviceName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
	restartCmd := fmt.Sprintf("systemctl --user restart %s 2>/dev/null || (cd %s && set -a && source .env && set +a && nohup ./aurago > log/egg.log 2>&1 &)", serviceName, baseDir)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, restartCmd); err != nil {
		return fmt.Errorf("failed to restart egg after reconfigure: %w", err)
	}

	return nil
}

func (c *SSHConnector) Rollback(ctx context.Context, nest NestRecord, secret []byte) error {
	baseDir := fmt.Sprintf("~/.aurago-egg-%s", nest.ID[:8])
	backupDir := baseDir + ".bak"

	// Check if backup exists
	checkCmd := fmt.Sprintf("test -d %s && echo ok || echo missing", backupDir)
	output, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, checkCmd)
	if err != nil {
		return fmt.Errorf("failed to check backup: %w", err)
	}
	if !strings.Contains(output, "ok") {
		return fmt.Errorf("no backup found for rollback")
	}

	// Stop current egg
	_ = c.Stop(ctx, nest, secret)

	// Replace current with backup
	restoreCmd := fmt.Sprintf("rm -rf %s && mv %s %s", baseDir, backupDir, baseDir)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, restoreCmd); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	// Restart the restored egg
	serviceName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])
	startCmd := fmt.Sprintf("systemctl --user restart %s 2>/dev/null || (cd %s && set -a && source .env && set +a && nohup ./aurago > log/egg.log 2>&1 &)", serviceName, baseDir)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd); err != nil {
		return fmt.Errorf("failed to restart after rollback: %w", err)
	}

	return nil
}
