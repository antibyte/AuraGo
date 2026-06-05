package desktop

import (
	"context"
	"crypto/des"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/bits"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

// vncUpgrader is the WebSocket upgrader for VNC connections.
var vncUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return sameHostWebSocketOrigin(r)
	},
}

func sendVNCError(conn *websocket.Conn, code, message string) {
	data, _ := json.Marshal(sshStatusMessage{Type: "error", Code: code, Message: message})
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

// HandleVNCProxy returns an http.HandlerFunc that upgrades to WebSocket
// and proxies a raw VNC (RFB) session to the requested device.
func HandleVNCProxy(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger, options ...RemoteProxyOptions) http.HandlerFunc {
	proxyOptions := normalizeRemoteProxyOptions(options...)
	return func(w http.ResponseWriter, r *http.Request) {
		deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
		if deviceID == "" {
			http.Error(w, "missing device_id", http.StatusBadRequest)
			return
		}

		conn, err := vncUpgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Warn("VNC proxy WebSocket upgrade failed", "error", err)
			return
		}
		defer conn.Close()
		conn.SetReadLimit(remoteProxyReadLimit)

		device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
		if err != nil {
			sendVNCError(conn, "device_not_found", fmt.Sprintf("Device not found: %v", err))
			return
		}

		if device.Protocol != "vnc" {
			sendVNCError(conn, "protocol_mismatch", fmt.Sprintf("Device protocol is %q, expected vnc", device.Protocol))
			return
		}

		host, port, password, err := resolveVNCAccess(device, inventoryDB, vault)
		if err != nil {
			sendVNCError(conn, "credential_unavailable", fmt.Sprintf("Failed to resolve VNC access: %v", err))
			return
		}

		addr := vncDialAddress(host, port)
		vncConn, err := net.Dial("tcp", addr)
		if err != nil {
			sendVNCError(conn, "dial_failed", fmt.Sprintf("VNC connection failed: %v", err))
			return
		}
		defer vncConn.Close()
		handshakeDeadline := time.Now().Add(remoteProxyWriteTimeout)
		_ = vncConn.SetDeadline(handshakeDeadline)
		_ = conn.SetReadDeadline(handshakeDeadline)

		writer := &wsBinaryWriter{conn: conn}
		if err := writer.writeText(sshStatusMessage{Type: "connected", Message: "VNC session established"}); err != nil {
			logger.Warn("failed to send VNC connected status", "error", err)
			return
		}
		browserRFB := &wsRFBConn{conn: conn}
		if err := performRFBHandshake(browserRFB, vncConn, password); err != nil {
			sendVNCError(conn, "auth_failed", fmt.Sprintf("VNC authentication failed: %v", err))
			return
		}
		_ = conn.SetReadDeadline(time.Time{})
		_ = vncConn.SetDeadline(time.Now().Add(proxyOptions.MaxSessionDuration))
		if err := browserRFB.drainTo(vncConn); err != nil {
			sendVNCError(conn, "init_failed", fmt.Sprintf("VNC client initialization failed: %v", err))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), proxyOptions.MaxSessionDuration)
		defer cancel()

		// Bidirectional copy between WebSocket and VNC TCP connection.
		var wg sync.WaitGroup
		wg.Add(2)

		// VNC server -> WebSocket
		go func() {
			defer wg.Done()
			_, _ = io.Copy(writer, vncConn)
			cancel()
		}()

		// WebSocket -> VNC server
		go func() {
			defer wg.Done()
			for {
				_ = conn.SetReadDeadline(time.Now().Add(proxyOptions.IdleTimeout))
				mt, data, readErr := conn.ReadMessage()
				if readErr != nil {
					if websocket.IsUnexpectedCloseError(readErr, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
						logger.Warn("VNC proxy WebSocket read error", "error", readErr)
					}
					cancel()
					return
				}
				if mt == websocket.BinaryMessage {
					if _, werr := vncConn.Write(data); werr != nil {
						logger.Warn("VNC proxy TCP write error", "error", werr)
						cancel()
						return
					}
				}
				// Text messages from client are ignored (noVNC uses binary for RFB).
			}
		}()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-ctx.Done():
			_ = vncConn.Close()
			_ = conn.Close()
			<-done
		case <-done:
		}
		_ = writer.writeText(sshStatusMessage{Type: "disconnected", Message: "VNC session closed"})
	}
}

func vncDialAddress(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

type wsRFBConn struct {
	conn   *websocket.Conn
	buffer []byte
}

func (c *wsRFBConn) Read(p []byte) (int, error) {
	for len(c.buffer) == 0 {
		mt, data, err := c.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if mt != websocket.BinaryMessage {
			continue
		}
		c.buffer = append(c.buffer[:0], data...)
	}
	n := copy(p, c.buffer)
	c.buffer = c.buffer[n:]
	return n, nil
}

func (c *wsRFBConn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	data := append([]byte(nil), p...)
	if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *wsRFBConn) drainTo(w io.Writer) error {
	if len(c.buffer) == 0 {
		return nil
	}
	_, err := w.Write(c.buffer)
	c.buffer = nil
	return err
}

type rfbVersion struct {
	raw        []byte
	major, min int
}

func performRFBHandshake(browser io.ReadWriter, server io.ReadWriter, password string) error {
	serverVersion, err := readRFBVersion(server)
	if err != nil {
		return fmt.Errorf("read server version: %w", err)
	}
	if err := writeRFB(browser, serverVersion.raw); err != nil {
		return fmt.Errorf("send server version to browser: %w", err)
	}
	browserVersion, err := readRFBVersion(browser)
	if err != nil {
		return fmt.Errorf("read browser version: %w", err)
	}
	if err := writeRFB(server, browserVersion.raw); err != nil {
		return fmt.Errorf("send browser version to server: %w", err)
	}
	if err := authenticateRemoteRFB(server, serverVersion, password); err != nil {
		return err
	}
	if err := offerBrowserNoAuth(browser, browserVersion); err != nil {
		return err
	}
	clientInit := make([]byte, 1)
	if _, err := io.ReadFull(browser, clientInit); err != nil {
		return fmt.Errorf("read browser client init: %w", err)
	}
	if err := writeRFB(server, clientInit); err != nil {
		return fmt.Errorf("send client init to server: %w", err)
	}
	return nil
}

func readRFBVersion(r io.Reader) (rfbVersion, error) {
	raw := make([]byte, 12)
	if _, err := io.ReadFull(r, raw); err != nil {
		return rfbVersion{}, err
	}
	var major, minor int
	if _, err := fmt.Sscanf(string(raw), "RFB %03d.%03d\n", &major, &minor); err != nil {
		return rfbVersion{}, fmt.Errorf("invalid RFB version %q", string(raw))
	}
	return rfbVersion{raw: raw, major: major, min: minor}, nil
}

func authenticateRemoteRFB(server io.ReadWriter, version rfbVersion, password string) error {
	if version.major == 3 && version.min <= 3 {
		securityType := make([]byte, 4)
		if _, err := io.ReadFull(server, securityType); err != nil {
			return fmt.Errorf("read RFB 3.3 security type: %w", err)
		}
		switch binary.BigEndian.Uint32(securityType) {
		case 1:
			return nil
		case 2:
			return completeVNCPasswordAuth(server, version, password)
		default:
			return fmt.Errorf("unsupported RFB 3.3 security type %d", binary.BigEndian.Uint32(securityType))
		}
	}

	countBuf := make([]byte, 1)
	if _, err := io.ReadFull(server, countBuf); err != nil {
		return fmt.Errorf("read RFB security type count: %w", err)
	}
	count := int(countBuf[0])
	if count == 0 {
		reason := readRFBReason(server)
		if reason == "" {
			reason = "server rejected connection"
		}
		return fmt.Errorf("%s", reason)
	}
	types := make([]byte, count)
	if _, err := io.ReadFull(server, types); err != nil {
		return fmt.Errorf("read RFB security types: %w", err)
	}
	choice := byte(0)
	if strings.TrimSpace(password) != "" && bytesContains(types, 2) {
		choice = 2
	} else if bytesContains(types, 1) {
		choice = 1
	} else if bytesContains(types, 2) {
		return fmt.Errorf("VNC password is required")
	}
	if choice == 0 {
		return fmt.Errorf("unsupported RFB security types %v", types)
	}
	if err := writeRFB(server, []byte{choice}); err != nil {
		return fmt.Errorf("send RFB security choice: %w", err)
	}
	switch choice {
	case 1:
		return readRFBSecurityResult(server, version)
	case 2:
		return completeVNCPasswordAuth(server, version, password)
	default:
		return fmt.Errorf("unsupported RFB security choice %d", choice)
	}
}

func completeVNCPasswordAuth(server io.ReadWriter, version rfbVersion, password string) error {
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("VNC password is required")
	}
	challenge := make([]byte, 16)
	if _, err := io.ReadFull(server, challenge); err != nil {
		return fmt.Errorf("read VNC challenge: %w", err)
	}
	response, err := vncPasswordResponse(password, challenge)
	if err != nil {
		return err
	}
	if err := writeRFB(server, response); err != nil {
		return fmt.Errorf("send VNC challenge response: %w", err)
	}
	return readRFBSecurityResult(server, version)
}

func offerBrowserNoAuth(browser io.ReadWriter, version rfbVersion) error {
	if version.major == 3 && version.min <= 3 {
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, 1)
		return writeRFB(browser, buf)
	}
	if err := writeRFB(browser, []byte{1, 1}); err != nil {
		return fmt.Errorf("send browser no-auth security type: %w", err)
	}
	choice := make([]byte, 1)
	if _, err := io.ReadFull(browser, choice); err != nil {
		return fmt.Errorf("read browser security choice: %w", err)
	}
	if choice[0] != 1 {
		return fmt.Errorf("browser rejected no-auth security type")
	}
	if err := writeRFB(browser, []byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("send browser security result: %w", err)
	}
	return nil
}

func readRFBSecurityResult(r io.Reader, version rfbVersion) error {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("read RFB security result: %w", err)
	}
	status := binary.BigEndian.Uint32(buf)
	if status == 0 {
		return nil
	}
	reason := ""
	if version.major > 3 || (version.major == 3 && version.min >= 8) {
		reason = readRFBReason(r)
	}
	if reason == "" {
		reason = "authentication failed"
	}
	return fmt.Errorf("authentication failed: %s", reason)
}

func readRFBReason(r io.Reader) string {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return ""
	}
	n := binary.BigEndian.Uint32(lenBuf)
	if n == 0 || n > 4096 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return ""
	}
	return strings.TrimSpace(string(buf))
}

func vncPasswordResponse(password string, challenge []byte) ([]byte, error) {
	if len(challenge) != 16 {
		return nil, fmt.Errorf("VNC challenge must be 16 bytes")
	}
	key := make([]byte, 8)
	copy(key, []byte(password))
	for i := range key {
		key[i] = bits.Reverse8(key[i])
	}
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create VNC DES cipher: %w", err)
	}
	response := make([]byte, 16)
	block.Encrypt(response[:8], challenge[:8])
	block.Encrypt(response[8:], challenge[8:])
	return response, nil
}

func bytesContains(values []byte, want byte) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func writeRFB(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	return err
}

// resolveVNCAccess resolves host, port and password for a VNC device.
func resolveVNCAccess(device inventory.DeviceRecord, inventoryDB *sql.DB, vault *security.Vault) (host string, port int, password string, err error) {
	host = strings.TrimSpace(device.IPAddress)
	if host == "" {
		host = strings.TrimSpace(device.Name)
	}
	port = device.Port
	if port <= 0 {
		port = 5900
	}

	if strings.TrimSpace(device.CredentialID) != "" {
		cred, credErr := credentials.GetByID(inventoryDB, device.CredentialID)
		if credErr != nil {
			return "", 0, "", fmt.Errorf("linked credential %q could not be loaded: %w", device.CredentialID, credErr)
		}
		if strings.TrimSpace(cred.Host) != "" {
			host = strings.TrimSpace(cred.Host)
		}
		if strings.TrimSpace(cred.PasswordVaultID) != "" {
			if vault == nil {
				return "", 0, "", fmt.Errorf("vault is not available")
			}
			secretStr, vaultErr := vault.ReadSecret(cred.PasswordVaultID)
			if vaultErr != nil {
				return "", 0, "", fmt.Errorf("read vault secret %q: %w", cred.PasswordVaultID, vaultErr)
			}
			password = secretStr
		}
	} else if strings.TrimSpace(device.VaultSecretID) != "" {
		if vault == nil {
			return "", 0, "", fmt.Errorf("vault is not available")
		}
		secretStr, vaultErr := vault.ReadSecret(device.VaultSecretID)
		if vaultErr != nil {
			return "", 0, "", fmt.Errorf("read vault secret %q: %w", device.VaultSecretID, vaultErr)
		}
		password = secretStr
	}

	if host == "" {
		return "", 0, "", fmt.Errorf("device host is missing")
	}

	return host, port, password, nil
}
