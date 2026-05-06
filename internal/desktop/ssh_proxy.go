package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && strings.EqualFold(u.Host, r.Host)
	},
}

// sshControlMessage is a JSON control message from the client.
type sshControlMessage struct {
	Type string `json:"type"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// sshStatusMessage is a JSON status message from the server.
type sshStatusMessage struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

type sshConfigResult struct {
	Config          *ssh.ClientConfig
	InsecureHostKey bool
}

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
	return w.conn.WriteMessage(websocket.TextMessage, data)
}

func sendError(conn *websocket.Conn, message string) {
	data, _ := json.Marshal(sshStatusMessage{Type: "error", Message: message})
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

// HandleSSHProxy returns an http.HandlerFunc that upgrades to WebSocket
// and proxies an interactive SSH session to the requested device.
func HandleSSHProxy(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		configResult, err := buildSSHConfig(username, secret, logger)
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

		ctx, cancel := context.WithCancel(r.Context())
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
// Host-key verification follows the desktop Quick Connect policy: prefer known_hosts,
// fall back to insecure with a warning when the file is absent.
func buildSSHConfig(user string, secret []byte, logger *slog.Logger) (sshConfigResult, error) {
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
			logger.Warn("SSH known_hosts not found, falling back to insecure host key verification for desktop Quick Connect")
			hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
			insecureHostKey = true
		}
	}

	return sshConfigResult{Config: &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}, InsecureHostKey: insecureHostKey}, nil
}
