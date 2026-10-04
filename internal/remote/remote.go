//go:build !remote_minimal

package remote

import (
	"aurago/internal/fileutil"
	"aurago/internal/uid"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// InsecureHostKey disables SSH host key verification when true.
// Set at startup based on config (remote_control.ssh_insecure_host_key).
// When false (default) AuraGo uses the user's known_hosts file if available.
var InsecureHostKey bool

// knownHostsCache caches the parsed known_hosts callback so every SSH
// connection does not re-parse the file. The callback is refreshed every
// 5 minutes to pick up newly added host keys.
var knownHostsCache = struct {
	mu        sync.RWMutex
	path      string
	callback  ssh.HostKeyCallback
	expiresAt time.Time
}{}

const knownHostsCacheTTL = 5 * time.Minute

func getKnownHostsCallback() (ssh.HostKeyCallback, error) {
	knownHostsCache.mu.RLock()
	if knownHostsCache.callback != nil && knownHostsCache.path != "" && time.Now().Before(knownHostsCache.expiresAt) {
		cb := knownHostsCache.callback
		knownHostsCache.mu.RUnlock()
		return cb, nil
	}
	knownHostsCache.mu.RUnlock()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home dir: %w", err)
	}
	knownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")
	if _, statErr := os.Stat(knownHostsFile); statErr != nil {
		return nil, fmt.Errorf("known_hosts file not found at %s", knownHostsFile)
	}

	cb, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("parse known_hosts: %w", err)
	}

	knownHostsCache.mu.Lock()
	knownHostsCache.path = knownHostsFile
	knownHostsCache.callback = cb
	knownHostsCache.expiresAt = time.Now().Add(knownHostsCacheTTL)
	knownHostsCache.mu.Unlock()
	return cb, nil
}

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
		cb, err := getKnownHostsCallback()
		if err != nil {
			return nil, fmt.Errorf("SSH host key verification failed: %w. "+
				"Add the host key with 'ssh-keyscan <host> >> ~/.ssh/known_hosts' or enable "+
				"'ssh.insecure_host_key: true' in config to disable host verification (not recommended)", err)
		}
		hostKeyCallback = cb
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}, nil
}

// ExecuteRemoteCommand runs a command on a remote host via SSH and returns the combined output.
func ExecuteRemoteCommand(ctx context.Context, host string, port int, user string, secret []byte, cmd string, input ...io.Reader) (string, error) {
	config, err := GetSSHConfig(user, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get ssh config: %w", err)
	}

	addr := net.JoinHostPort(host, fmt.Sprint(port))

	// Use a dialer that supports context for the connection phase
	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial: %w", err)
	}

	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	deadline := time.Now().Add(config.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("ssh handshake failed: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})
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
	if len(input) > 0 {
		session.Stdin = input[0]
	}
	output, err := session.CombinedOutput(cmd)
	if ctx.Err() != nil {
		return string(output), fmt.Errorf("command cancelled: %w", ctx.Err())
	}
	if err != nil {
		return string(output), fmt.Errorf("command execution failed: %w", err)
	}

	return string(output), nil
}

// ExecuteRemoteScript runs a bash script on a remote host via SSH.
// The script is sent over stdin so secret material does not appear in the
// local process arguments. If the SSH user is not root, passwordless sudo is
// required.
func ExecuteRemoteScript(ctx context.Context, host string, port int, user string, secret []byte, script string) (string, error) {
	config, err := GetSSHConfig(user, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get ssh config: %w", err)
	}

	addr := net.JoinHostPort(host, fmt.Sprint(port))
	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial: %w", err)
	}

	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	deadline := time.Now().Add(config.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("ssh handshake failed: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to open stdin pipe: %w", err)
	}
	go func() {
		_, _ = io.WriteString(stdin, script)
		_ = stdin.Close()
	}()

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

	output, err := session.CombinedOutput(`if [ "$(id -u)" -eq 0 ]; then bash -s; else sudo -n bash -s; fi`)
	if ctx.Err() != nil {
		return string(output), fmt.Errorf("script cancelled: %w", ctx.Err())
	}
	if err != nil {
		return string(output), fmt.Errorf("script execution failed: %w", err)
	}
	return string(output), nil
}

// TransferFile handles file uploads and downloads via SFTP.
func TransferFile(ctx context.Context, host string, port int, user string, secret []byte, localPath, remotePath, direction string, allowedRoot ...string) error {
	rootPath := filepath.Dir(localPath)
	if len(allowedRoot) > 0 {
		rootPath = allowedRoot[0]
	}
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return err
	}
	absLocal, err := filepath.Abs(localPath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absLocal)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return fmt.Errorf("local path is outside transfer root")
	}
	root, err := os.OpenRoot(absRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	config, err := GetSSHConfig(user, secret)
	if err != nil {
		return fmt.Errorf("failed to get ssh config: %w", err)
	}

	addr := net.JoinHostPort(host, fmt.Sprint(port))

	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	deadline := time.Now().Add(config.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return fmt.Errorf("ssh handshake failed: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})
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
			errCh <- uploadFile(ctx, root, rel, remotePath, sftpClient)
		case "download":
			errCh <- downloadFile(ctx, root, rel, remotePath, sftpClient)
		default:
			errCh <- fmt.Errorf("invalid direction: %s", direction)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		_ = client.Close() // unblock SSH, SFTP and every transfer read/write
		_ = sftpClient.Close()
		<-errCh // the worker must finish before its rooted handle is closed
		return fmt.Errorf("transfer cancelled: %w", ctx.Err())
	}
}

func uploadFile(ctx context.Context, root *os.Root, localPath, remotePath string, client *sftp.Client) error {
	localFile, err := root.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer localFile.Close()
	if info, err := localFile.Stat(); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("transfer requires a regular file")
	}
	tmp := remotePath + ".aurago-" + uid.New()
	remoteFile, err := client.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fmt.Errorf("stage remote file: %w", err)
	}
	defer client.Remove(tmp)
	_, err = io.Copy(remoteFile, localFile)
	closeErr := remoteFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// POSIX rename is atomic and replaces the destination. A server without
	// this extension fails closed; never unlink the last good destination.
	return client.PosixRename(tmp, remotePath)
}

func downloadFile(ctx context.Context, root *os.Root, localPath, remotePath string, client *sftp.Client) error {
	remoteFile, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer remoteFile.Close()
	return publishTransferDownload(ctx, root, localPath, remoteFile)
}

func publishTransferDownload(ctx context.Context, root *os.Root, localPath string, source io.Reader) error {
	tmp := filepath.Join(filepath.Dir(localPath), ".aurago-transfer-"+uid.New())
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, err = io.Copy(f, source)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return fileutil.RenameRootContext(ctx, root, tmp, localPath)
}
