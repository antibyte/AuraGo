package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/remote"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshUpgrader is the WebSocket upgrader for SSH terminal connections.
var sshUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return sameHostWebSocketOrigin(r)
	},
}

const (
	remoteProxyMaxSessionDuration = 60 * time.Minute
	remoteProxyIdleTimeout        = 5 * time.Minute
	remoteProxyWriteTimeout       = 10 * time.Second
	remoteProxyReadLimit          = 1 << 20
)

// sshControlMessage is a JSON control message from the client.
type sshControlMessage struct {
	Type   string `json:"type"`
	Cols   int    `json:"cols,omitempty"`
	Rows   int    `json:"rows,omitempty"`
	Accept bool   `json:"accept,omitempty"`
}

// sshStatusMessage is a JSON status message from the server.
type sshStatusMessage struct {
	Type        string `json:"type"`
	Message     string `json:"message,omitempty"`
	Code        string `json:"code,omitempty"`
	Host        string `json:"host,omitempty"`
	KeyType     string `json:"key_type,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type sshConfigResult struct {
	Config          *ssh.ClientConfig
	InsecureHostKey bool
}

type sshHostKeyPrompter func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) (bool, error)

// wsMessage wraps a WebSocket message for channel transport.
type wsMessage struct {
	msgType int
	data    []byte
	err     error
}

// wsBinaryWriter wraps a WebSocket connection for safe concurrent binary writes.
type wsBinaryWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsBinaryWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(remoteProxyWriteTimeout))
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsBinaryWriter) writeText(msg sshStatusMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(remoteProxyWriteTimeout))
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

func sendError(conn *websocket.Conn, message string) {
	data, _ := json.Marshal(sshStatusMessage{Type: "error", Message: message})
	_ = conn.SetWriteDeadline(time.Now().Add(remoteProxyWriteTimeout))
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

// HandleSSHProxy returns an http.HandlerFunc that upgrades to WebSocket
// and proxies an interactive SSH session to the requested device.
func HandleSSHProxy(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, options ...RemoteProxyOptions) http.HandlerFunc {
	proxyOptions := normalizeRemoteProxyOptions(options...)
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		if deviceID == "" {
			http.Error(w, "missing device_id", http.StatusBadRequest)
			return
		}

		cols := 80
		rows := 24
		if c := r.URL.Query().Get("cols"); c != "" {
			if v, err := strconv.Atoi(c); err == nil && v > 0 {
				cols = v
			}
		}
		if rr := r.URL.Query().Get("rows"); rr != "" {
			if v, err := strconv.Atoi(rr); err == nil && v > 0 {
				rows = v
			}
		}

		conn, err := sshUpgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Warn("SSH proxy WebSocket upgrade failed", "error", err)
			return
		}
		defer conn.Close()
		conn.SetReadLimit(remoteProxyReadLimit)

		device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
		if err != nil {
			sendError(conn, fmt.Sprintf("Device not found: %v", err))
			return
		}

		host, port, username, secret, err := resolveSSHAccess(device, inventoryDB, vault)
		if err != nil {
			sendError(conn, fmt.Sprintf("Failed to resolve SSH access: %v", err))
			return
		}

		configResult, err := buildSSHConfig(username, secret, logger, func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) (bool, error) {
			return promptSSHHostKey(conn, hostname, key)
		})
		if err != nil {
			sendError(conn, fmt.Sprintf("SSH configuration error: %v", err))
			return
		}
		if configResult.InsecureHostKey {
			_ = conn.WriteJSON(sshStatusMessage{Type: "warning", Code: "insecure_host_key", Message: "SSH host key verification is disabled because known_hosts is unavailable."})
		}

		addr := fmt.Sprintf("%s:%d", host, port)
		client, err := ssh.Dial("tcp", addr, configResult.Config)
		if err != nil {
			sendError(conn, fmt.Sprintf("SSH connection failed: %v", err))
			return
		}
		defer client.Close()

		session, err := client.NewSession()
		if err != nil {
			sendError(conn, fmt.Sprintf("SSH session failed: %v", err))
			return
		}
		defer session.Close()

		modes := ssh.TerminalModes{
			ssh.ECHO:          1,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}
		if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
			sendError(conn, fmt.Sprintf("PTY request failed: %v", err))
			return
		}

		stdinPipe, err := session.StdinPipe()
		if err != nil {
			sendError(conn, fmt.Sprintf("SSH stdin pipe failed: %v", err))
			return
		}
		defer stdinPipe.Close()

		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			sendError(conn, fmt.Sprintf("SSH stdout pipe failed: %v", err))
			return
		}

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			sendError(conn, fmt.Sprintf("SSH stderr pipe failed: %v", err))
			return
		}

		if err := session.Shell(); err != nil {
			sendError(conn, fmt.Sprintf("SSH shell failed: %v", err))
			return
		}

		writer := &wsBinaryWriter{conn: conn}
		if err := writer.writeText(sshStatusMessage{Type: "connected", Message: "SSH session established"}); err != nil {
			logger.Warn("failed to send connected status", "error", err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), proxyOptions.MaxSessionDuration)
		defer cancel()

		// Forward SSH stdout/stderr to WebSocket; cancel context when either ends.
		go func() {
			_, _ = io.Copy(writer, stdoutPipe)
			cancel()
		}()
		go func() {
			_, _ = io.Copy(writer, stderrPipe)
			cancel()
		}()

		// Watch for SSH session end and cancel context.
		go func() {
			_ = session.Wait()
			cancel()
		}()

		// Read WebSocket messages in a goroutine so context cancellation can interrupt.
		msgCh := make(chan wsMessage, 1)
		go func() {
			defer close(msgCh)
			for {
				_ = conn.SetReadDeadline(time.Now().Add(proxyOptions.IdleTimeout))
				mt, data, err := conn.ReadMessage()
				select {
				case <-ctx.Done():
					return
				case msgCh <- wsMessage{msgType: mt, data: data, err: err}:
					if err != nil {
						return
					}
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				_ = writer.writeText(sshStatusMessage{Type: "disconnected", Message: "SSH session closed"})
				return
			case msg, ok := <-msgCh:
				if !ok || msg.err != nil {
					if msg.err != nil && websocket.IsUnexpectedCloseError(msg.err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
						logger.Warn("SSH proxy WebSocket read error", "error", msg.err)
					}
					return
				}

				if msg.msgType == websocket.BinaryMessage {
					if _, werr := stdinPipe.Write(msg.data); werr != nil {
						logger.Warn("SSH proxy stdin write error", "error", werr)
						return
					}
				} else if msg.msgType == websocket.TextMessage {
					var ctrl sshControlMessage
					if jerr := json.Unmarshal(msg.data, &ctrl); jerr != nil {
						continue
					}
					if ctrl.Type == "resize" && ctrl.Cols > 0 && ctrl.Rows > 0 {
						if werr := session.WindowChange(ctrl.Rows, ctrl.Cols); werr != nil {
							logger.Warn("SSH proxy window change failed", "error", werr)
						}
					}
				}
			}
		}
	}
}

func promptSSHHostKey(conn *websocket.Conn, hostname string, key ssh.PublicKey) (bool, error) {
	host := knownhosts.Normalize(hostname)
	fingerprint := ssh.FingerprintSHA256(key)
	message := fmt.Sprintf("Unknown SSH host key for %s (%s, %s). Trust and save this host key?", host, key.Type(), fingerprint)
	_ = conn.SetWriteDeadline(time.Now().Add(remoteProxyWriteTimeout))
	if err := conn.WriteJSON(sshStatusMessage{
		Type:        "host_key_prompt",
		Code:        "unknown_host_key",
		Message:     message,
		Host:        host,
		KeyType:     key.Type(),
		Fingerprint: fingerprint,
	}); err != nil {
		return false, fmt.Errorf("send host key prompt: %w", err)
	}

	deadline := time.Now().Add(2 * time.Minute)
	for {
		_ = conn.SetReadDeadline(deadline)
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return false, fmt.Errorf("read host key decision: %w", err)
		}
		if msgType != websocket.TextMessage {
			continue
		}

		var ctrl sshControlMessage
		if err := json.Unmarshal(data, &ctrl); err != nil {
			continue
		}
		if ctrl.Type == "host_key_decision" {
			return ctrl.Accept, nil
		}
	}
}

// resolveSSHAccess resolves host, port, username and secret for a device,
// following the same logic as the agent's resolveDeviceSSHAccess.
func resolveSSHAccess(device inventory.DeviceRecord, inventoryDB *sql.DB, vault *security.Vault) (host string, port int, username string, secret []byte, err error) {
	host = strings.TrimSpace(device.IPAddress)
	if host == "" {
		host = strings.TrimSpace(device.Name)
	}
	username = strings.TrimSpace(device.Username)
	port = device.Port
	if port <= 0 {
		port = 22
	}
	secretID := strings.TrimSpace(device.VaultSecretID)

	if strings.TrimSpace(device.CredentialID) != "" {
		cred, err := credentials.GetByID(inventoryDB, device.CredentialID)
		if err != nil {
			return "", 0, "", nil, fmt.Errorf("linked credential %q could not be loaded: %w", device.CredentialID, err)
		}
		if strings.TrimSpace(cred.Host) != "" {
			host = strings.TrimSpace(cred.Host)
		}
		if strings.TrimSpace(cred.Username) != "" {
			username = strings.TrimSpace(cred.Username)
		}
		switch {
		case strings.TrimSpace(cred.CertificateVaultID) != "":
			secretID = strings.TrimSpace(cred.CertificateVaultID)
		case strings.TrimSpace(cred.PasswordVaultID) != "":
			secretID = strings.TrimSpace(cred.PasswordVaultID)
		default:
			return "", 0, "", nil, fmt.Errorf("linked credential %q has neither password nor certificate stored in the vault", cred.Name)
		}
	}

	if host == "" {
		return "", 0, "", nil, fmt.Errorf("device host is missing")
	}
	if username == "" {
		return "", 0, "", nil, fmt.Errorf("SSH username is missing")
	}
	if secretID == "" {
		return "", 0, "", nil, fmt.Errorf("SSH secret is missing")
	}
	if vault == nil {
		return "", 0, "", nil, fmt.Errorf("vault is not available")
	}

	secretStr, err := vault.ReadSecret(secretID)
	if err != nil {
		return "", 0, "", nil, fmt.Errorf("read vault secret %q: %w", secretID, err)
	}

	return host, port, username, []byte(secretStr), nil
}

// buildSSHConfig creates an ssh.ClientConfig, auto-detecting password vs private-key auth.
// Host-key verification follows the desktop Quick Connect policy: prefer
// known_hosts and prompt the user before storing a new host key.
func buildSSHConfig(user string, secret []byte, logger *slog.Logger, prompter sshHostKeyPrompter) (sshConfigResult, error) {
	var auth []ssh.AuthMethod

	signer, err := ssh.ParsePrivateKey(secret)
	if err == nil {
		auth = append(auth, ssh.PublicKeys(signer))
	} else {
		auth = append(auth, ssh.Password(string(secret)))
	}

	var hostKeyCallback ssh.HostKeyCallback
	insecureHostKey := false
	if remote.InsecureHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
		insecureHostKey = true
	} else {
		knownHostsFile, err := defaultKnownHostsFile()
		if err != nil {
			return sshConfigResult{}, err
		}
		hostKeyCallback, err = knownHostsCallback(knownHostsFile, prompter, logger)
		if err != nil {
			return sshConfigResult{}, err
		}
	}

	return sshConfigResult{Config: &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}, InsecureHostKey: insecureHostKey}, nil
}

func defaultKnownHostsFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("SSH host key verification failed: find user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".ssh", "known_hosts"), nil
}

func knownHostsCallback(knownHostsFile string, prompter sshHostKeyPrompter, logger *slog.Logger) (ssh.HostKeyCallback, error) {
	cb, err := knownhosts.New(knownHostsFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && prompter != nil {
			return func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
				return promptAndStoreHostKey(knownHostsFile, prompter, logger, hostname, remoteAddr, key)
			}, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("SSH host key verification failed: no known_hosts file found at ~/.ssh/known_hosts. Add the host key with 'ssh-keyscan <host> >> ~/.ssh/known_hosts' or connect through Quick Connect and approve the host key prompt")
		}
		return nil, fmt.Errorf("SSH host key verification failed: load known_hosts: %w", err)
	}
	if prompter == nil {
		return cb, nil
	}

	return func(hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remoteAddr, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
			return promptAndStoreHostKey(knownHostsFile, prompter, logger, hostname, remoteAddr, key)
		}
		return err
	}, nil
}

func promptAndStoreHostKey(knownHostsFile string, prompter sshHostKeyPrompter, logger *slog.Logger, hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
	accepted, err := prompter(hostname, remoteAddr, key)
	if err != nil {
		return fmt.Errorf("host key prompt failed: %w", err)
	}
	if !accepted {
		return fmt.Errorf("SSH host key for %s was rejected by the user", knownhosts.Normalize(hostname))
	}
	if err := appendKnownHostKey(knownHostsFile, hostname, remoteAddr, key); err != nil {
		return fmt.Errorf("store SSH host key: %w", err)
	}
	if logger != nil {
		logger.Info("stored SSH host key exception", "host", knownhosts.Normalize(hostname), "key_type", key.Type())
	}
	return nil
}

func appendKnownHostKey(knownHostsFile string, hostname string, remoteAddr net.Addr, key ssh.PublicKey) error {
	if err := os.MkdirAll(filepath.Dir(knownHostsFile), 0o700); err != nil {
		return fmt.Errorf("create .ssh directory: %w", err)
	}

	addresses := []string{hostname}
	if remoteAddr != nil {
		if remote := strings.TrimSpace(remoteAddr.String()); remote != "" && remote != hostname {
			addresses = append(addresses, remote)
		}
	}

	file, err := os.OpenFile(knownHostsFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, knownhosts.Line(addresses, key)); err != nil {
		return err
	}
	return nil
}
