package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// InsecureHostKey disables SSH host key verification when true.
// Set at startup based on config (remote_control.ssh_insecure_host_key).
// When false (default) AuraGo uses the user's known_hosts file if available.
var InsecureHostKey bool

// GetSSHConfig creates an ssh.ClientConfig from a username and a secret (password or private key).
func GetSSHConfig(user string, secret []byte) (*ssh.ClientConfig, error) {
	var auth []ssh.AuthMethod

	// Try to parse as private key first
	signer, err := ssh.ParsePrivateKey(secret)
	if err == nil {
		auth = append(auth, ssh.PublicKeys(signer))
	} else {
		// Fallback to password
		auth = append(auth, ssh.Password(string(secret)))
	}

	// Host key verification: use known_hosts when available.
	// If InsecureHostKey is explicitly enabled via config, skip verification (homelab opt-in).
	// Never silently fall back to insecure — require explicit opt-in or a valid known_hosts file.
	var hostKeyCallback ssh.HostKeyCallback
	if InsecureHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
	} else {
		usingKnownHosts := false
		homeDir, err := os.UserHomeDir()
		if err == nil {
			knownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")
			if _, statErr := os.Stat(knownHostsFile); statErr == nil {
				if cb, khErr := knownhosts.New(knownHostsFile); khErr == nil {
					hostKeyCallback = cb
					usingKnownHosts = true
				}
			}
		}
		if !usingKnownHosts {
			return nil, fmt.Errorf("SSH host key verification failed: no known_hosts file found at ~/.ssh/known_hosts. " +
				"Add the host key with 'ssh-keyscan <host> >> ~/.ssh/known_hosts' or enable " +
				"'ssh.insecure_host_key: true' in config to disable host verification (not recommended)")
		}
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}, nil
}

// ExecuteRemoteCommand runs a command on a remote host via SSH and returns the combined output.
func ExecuteRemoteCommand(ctx context.Context, host string, port int, user string, secret []byte, cmd string) (string, error) {
	config, err := GetSSHConfig(user, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get ssh config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	// Use a dialer that supports context for the connection phase
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("ssh handshake failed: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Propagate context cancellation to the SSH session
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Signal(ssh.SIGKILL)
			_ = session.Close()
		case <-done:
		}
	}()

	// Capture output
	output, err := session.CombinedOutput(cmd)
	if ctx.Err() != nil {
		return string(output), fmt.Errorf("command cancelled: %w", ctx.Err())
	}
	if err != nil {
		return string(output), fmt.Errorf("command execution failed: %w", err)
	}

	return string(output), nil
}

// TransferFile handles file uploads and downloads via SFTP.
func TransferFile(ctx context.Context, host string, port int, user string, secret []byte, localPath, remotePath, direction string) error {
	config, err := GetSSHConfig(user, secret)
	if err != nil {
		return fmt.Errorf("failed to get ssh config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return fmt.Errorf("ssh handshake failed: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}
	defer sftpClient.Close()

	// Monitor context cancellation for the transfer
	errCh := make(chan error, 1)
	go func() {
		switch direction {
		case "upload":
			errCh <- uploadFile(localPath, remotePath, sftpClient)
		case "download":
			errCh <- downloadFile(localPath, remotePath, sftpClient)
		default:
			errCh <- fmt.Errorf("invalid direction: %s", direction)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		_ = sftpClient.Close() // force-close to unblock the transfer goroutine
		return fmt.Errorf("transfer cancelled: %w", ctx.Err())
	}
}

func uploadFile(localPath, remotePath string, client *sftp.Client) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	remoteFile, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func downloadFile(localPath, remotePath string, client *sftp.Client) error {
	remoteFile, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}

	_, err = io.Copy(localFile, remoteFile)
	closeErr := localFile.Close()
	if err != nil {
		os.Remove(localPath) // Clean up corrupt/partial file
		return fmt.Errorf("failed to copy file: %w", err)
	}
	if closeErr != nil {
		os.Remove(localPath)
		return fmt.Errorf("failed to finalize local file: %w", closeErr)
	}

	return nil
}
