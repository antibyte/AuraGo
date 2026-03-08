package invasion

import (
	"aurago/internal/remote"
	"context"
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
	baseDir := fmt.Sprintf("/opt/aurago-egg-%s", nest.ID[:8])

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
	// Use a heredoc via SSH to write config content
	configEscaped := strings.ReplaceAll(string(payload.ConfigYAML), "'", "'\\''")
	writeCmd := fmt.Sprintf("cat > %s << 'EOFCFG'\n%s\nEOFCFG", remoteConfig, configEscaped)
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
		vaultWriteCmd := fmt.Sprintf("echo '%s' | base64 -d > %s", encodeBase64(payload.VaultData), remoteVault)
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
	baseDir := fmt.Sprintf("/opt/aurago-egg-%s", nest.ID[:8])
	serviceName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])

	// Try systemd first
	stopCmd := fmt.Sprintf("systemctl stop %s 2>/dev/null || true", serviceName)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, stopCmd); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	// Also kill any running process
	killCmd := fmt.Sprintf("pkill -f '%s/aurago' 2>/dev/null || true", baseDir)
	_, _ = remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, killCmd)

	return nil
}

func (c *SSHConnector) Status(ctx context.Context, nest NestRecord, secret []byte) (string, error) {
	baseDir := fmt.Sprintf("/opt/aurago-egg-%s", nest.ID[:8])
	serviceName := fmt.Sprintf("aurago-egg-%s", nest.ID[:8])

	// Check systemd service first
	output, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret,
		fmt.Sprintf("systemctl is-active %s 2>/dev/null || echo 'inactive'", serviceName))
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

	writeCmd := fmt.Sprintf("cat > /etc/systemd/system/%s.service << 'EOF'\n%s\nEOF", serviceName, unitFile)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, writeCmd); err != nil {
		return fmt.Errorf("failed to write service unit: %w", err)
	}

	startCmd := fmt.Sprintf("systemctl daemon-reload && systemctl enable %s && systemctl start %s", serviceName, serviceName)
	if _, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func (c *SSHConnector) startProcess(ctx context.Context, nest NestRecord, secret []byte, baseDir string) error {
	// Start in background with nohup, redirect output to log
	startCmd := fmt.Sprintf("cd %s && source .env && nohup ./aurago > log/egg.log 2>&1 & echo $!", baseDir)
	output, err := remote.ExecuteRemoteCommand(ctx, nest.Host, nest.Port, nest.Username, secret, startCmd)
	if err != nil {
		return fmt.Errorf("failed to start egg process: %w", err)
	}
	_ = output // PID returned but not stored (we use process detection for status)
	return nil
}

// encodeBase64 is a helper to base64-encode bytes for transfer.
func encodeBase64(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte
	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		result = append(result, base64Chars[(b0>>2)&0x3f])
		result = append(result, base64Chars[((b0<<4)|(b1>>4))&0x3f])
		if i+1 < len(data) {
			result = append(result, base64Chars[((b1<<2)|(b2>>6))&0x3f])
		} else {
			result = append(result, '=')
		}
		if i+2 < len(data) {
			result = append(result, base64Chars[b2&0x3f])
		} else {
			result = append(result, '=')
		}
	}
	return string(result)
}
