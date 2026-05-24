package desktop

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"aurago/internal/credentials"
	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

// vncUpgrader is the WebSocket upgrader for VNC connections.
var vncUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && strings.EqualFold(u.Host, r.Host)
	},
}

func sendVNCError(conn *websocket.Conn, message string) {
	data, _ := json.Marshal(sshStatusMessage{Type: "error", Message: message})
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

// HandleVNCProxy returns an http.HandlerFunc that upgrades to WebSocket
// and proxies a raw VNC (RFB) session to the requested device.
func HandleVNCProxy(inventoryDB *sql.DB, vault *security.Vault, logger *slog.Logger) http.HandlerFunc {
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

		device, err := inventory.GetDeviceByID(inventoryDB, deviceID)
		if err != nil {
			sendVNCError(conn, fmt.Sprintf("Device not found: %v", err))
			return
		}

		if device.Protocol != "vnc" {
			sendVNCError(conn, fmt.Sprintf("Device protocol is %q, expected vnc", device.Protocol))
			return
		}

		host, port, _, err := resolveVNCAccess(device, inventoryDB, vault)
		if err != nil {
			sendVNCError(conn, fmt.Sprintf("Failed to resolve VNC access: %v", err))
			return
		}

		addr := fmt.Sprintf("%s:%d", host, port)
		vncConn, err := net.Dial("tcp", addr)
		if err != nil {
			sendVNCError(conn, fmt.Sprintf("VNC connection failed: %v", err))
			return
		}
		defer vncConn.Close()

		writer := &wsBinaryWriter{conn: conn}
		if err := writer.writeText(sshStatusMessage{Type: "connected", Message: "VNC session established"}); err != nil {
			logger.Warn("failed to send VNC connected status", "error", err)
			return
		}

		// Bidirectional copy between WebSocket and VNC TCP connection.
		var wg sync.WaitGroup
		wg.Add(2)

		// VNC server -> WebSocket
		go func() {
			defer wg.Done()
			_, _ = io.Copy(writer, vncConn)
		}()

		// WebSocket -> VNC server
		go func() {
			defer wg.Done()
			for {
				mt, data, readErr := conn.ReadMessage()
				if readErr != nil {
					if websocket.IsUnexpectedCloseError(readErr, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
						logger.Warn("VNC proxy WebSocket read error", "error", readErr)
					}
					return
				}
				if mt == websocket.BinaryMessage {
					if _, werr := vncConn.Write(data); werr != nil {
						logger.Warn("VNC proxy TCP write error", "error", werr)
						return
					}
				}
				// Text messages from client are ignored (noVNC uses binary for RFB).
			}
		}()

		wg.Wait()
		_ = writer.writeText(sshStatusMessage{Type: "disconnected", Message: "VNC session closed"})
	}
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
