package remote

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

// RemoteConnection represents a single connected remote agent.
type RemoteConnection struct {
	Conn          *websocket.Conn
	DeviceID      string
	Name          string
	SharedKey     string // hex-encoded
	LastHeartbeat time.Time
	Status        string
	Telemetry     HeartbeatPayload
	ReadOnly      bool
	AllowedPaths  []string
	Version       string
	SeqCounter    uint64
	mu            sync.Mutex
}

// Send writes a signed message to the remote.
func (rc *RemoteConnection) Send(msg *RemoteMessage) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.Conn.WriteJSON(msg)
}

// NextSeq returns and increments the sequence counter.
func (rc *RemoteConnection) NextSeq() uint64 {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.SeqCounter++
	return rc.SeqCounter
}

// RemoteHub manages all connected remote agents on the supervisor side.
type RemoteHub struct {
	mu          sync.RWMutex
	connections map[string]*RemoteConnection   // device_id → conn
	pending     map[string]chan *RemoteMessage // cmd_id → result channel
	pendingMu   sync.Mutex
	db          *sql.DB
	vault       *security.Vault
	logger      *slog.Logger

	// Config-driven defaults (set by caller after construction)
	DefaultReadOnly bool // default read-only setting for newly enrolled devices
	AutoApprove     bool // auto-approve devices with no enrollment token
	MaxFileSizeMB   int
	AuditLogEnabled bool

	nonceCache *nonceReplayCache

	// Callbacks
	OnConnect    func(deviceID, name string)
	OnDisconnect func(deviceID, name string)
	OnHeartbeat  func(deviceID string, hb HeartbeatPayload)
}

// NewRemoteHub creates a new hub for managing remote connections.
func NewRemoteHub(db *sql.DB, vault *security.Vault, logger *slog.Logger) *RemoteHub {
	return &RemoteHub{
		connections:     make(map[string]*RemoteConnection),
		pending:         make(map[string]chan *RemoteMessage),
		db:              db,
		vault:           vault,
		logger:          logger,
		MaxFileSizeMB:   DefaultMaxFileSizeMB,
		AuditLogEnabled: true,
		nonceCache:      newNonceReplayCache(MaxTimestampDrift, 10000),
	}
}

// DB returns the underlying database handle.
func (h *RemoteHub) DB() *sql.DB {
	return h.db
}

// Register adds an authenticated remote connection to the hub.
func (h *RemoteHub) Register(deviceID string, conn *RemoteConnection) {
	h.mu.Lock()
	if old, ok := h.connections[deviceID]; ok {
		h.logger.Warn("Replacing existing remote connection", "device_id", deviceID)
		_ = old.Conn.Close()
	}
	h.connections[deviceID] = conn
	h.mu.Unlock()

	h.logger.Info("Remote connected", "device_id", deviceID, "name", conn.Name)
	if h.OnConnect != nil {
		h.OnConnect(deviceID, conn.Name)
	}
}

// Unregister removes a remote connection from the hub.
func (h *RemoteHub) Unregister(deviceID string) {
	h.mu.Lock()
	conn, ok := h.connections[deviceID]
	if ok {
		delete(h.connections, deviceID)
	}
	h.mu.Unlock()

	if ok {
		_ = conn.Conn.Close()
		h.logger.Info("Remote disconnected", "device_id", deviceID, "name", conn.Name)

		if h.db != nil {
			_ = UpdateDeviceStatus(h.db, deviceID, "offline")
		}
		if h.OnDisconnect != nil {
			h.OnDisconnect(deviceID, conn.Name)
		}
	}
}

// GetConnection returns the connection for a device, or nil.
func (h *RemoteHub) GetConnection(deviceID string) *RemoteConnection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connections[deviceID]
}

// IsConnected checks if a device has an active connection.
func (h *RemoteHub) IsConnected(deviceID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.connections[deviceID]
	return ok
}

// ConnectedDevices returns all connected device IDs.
func (h *RemoteHub) ConnectedDevices() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.connections))
	for id := range h.connections {
		ids = append(ids, id)
	}
	return ids
}

// ConnectionCount returns the number of connected remotes.
func (h *RemoteHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// ── Command dispatch ────────────────────────────────────────────────────────

// SendCommand sends a command to a remote and waits for the result.
func (h *RemoteHub) SendCommand(deviceID string, cmd CommandPayload, timeout time.Duration) (ResultPayload, error) {
	conn := h.GetConnection(deviceID)
	if conn == nil {
		return ResultPayload{}, fmt.Errorf("no active connection for device %s", deviceID)
	}

	// Enforce read-only
	if conn.ReadOnly && !ReadOnlySafe(cmd.Operation) {
		return ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "denied",
			Error:     "device is in read-only mode",
		}, nil
	}

	msg, err := NewMessage(MsgCommand, deviceID, conn.SharedKey, conn.NextSeq(), cmd)
	if err != nil {
		return ResultPayload{}, fmt.Errorf("failed to create command message: %w", err)
	}

	// Create result channel
	resultCh := make(chan *RemoteMessage, 1)
	h.pendingMu.Lock()
	h.pending[cmd.CommandID] = resultCh
	h.pendingMu.Unlock()

	defer func() {
		h.pendingMu.Lock()
		delete(h.pending, cmd.CommandID)
		h.pendingMu.Unlock()
	}()

	if err := conn.Send(msg); err != nil {
		return ResultPayload{}, fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for result
	select {
	case rmsg := <-resultCh:
		var result ResultPayload
		if err := json.Unmarshal(rmsg.Payload, &result); err != nil {
			return ResultPayload{}, fmt.Errorf("failed to unmarshal result: %w", err)
		}
		return result, nil
	case <-time.After(timeout):
		return ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "timeout",
			Error:     fmt.Sprintf("command timed out after %v", timeout),
		}, nil
	}
}

// SendConfigUpdate pushes config changes to a remote device and updates the
// in-memory connection state so server-side enforcement reflects the new values.
func (h *RemoteHub) SendConfigUpdate(deviceID string, update ConfigUpdatePayload) error {
	conn := h.GetConnection(deviceID)
	if conn == nil {
		return fmt.Errorf("no active connection for device %s", deviceID)
	}
	msg, err := NewMessage(MsgConfigUpdate, deviceID, conn.SharedKey, conn.NextSeq(), update)
	if err != nil {
		return fmt.Errorf("failed to create config_update message: %w", err)
	}
	if err := conn.Send(msg); err != nil {
		return err
	}
	// Keep in-memory state in sync so server-side command enforcement reflects the change.
	conn.mu.Lock()
	if update.ReadOnly != nil {
		conn.ReadOnly = *update.ReadOnly
	}
	if update.AllowedPaths != nil {
		conn.AllowedPaths = update.AllowedPaths
	}
	conn.mu.Unlock()
	return nil
}

// SendRevoke sends a revoke command and unregisters the device.
func (h *RemoteHub) SendRevoke(deviceID string) error {
	conn := h.GetConnection(deviceID)
	if conn == nil {
		return fmt.Errorf("no active connection for device %s", deviceID)
	}
	msg, err := NewMessage(MsgRevoke, deviceID, conn.SharedKey, conn.NextSeq(), nil)
	if err != nil {
		return fmt.Errorf("failed to create revoke message: %w", err)
	}
	if err := conn.Send(msg); err != nil {
		return err
	}
	h.Unregister(deviceID)
	if h.db != nil {
		_ = UpdateDeviceStatus(h.db, deviceID, "revoked")
	}
	return nil
}

// ── Message handling ────────────────────────────────────────────────────────

// HandleMessages reads messages from a remote connection and dispatches them.
// Blocks until the connection closes or an error occurs.
func (h *RemoteHub) HandleMessages(conn *RemoteConnection) {
	defer h.Unregister(conn.DeviceID)

	for {
		_, data, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Warn("Remote connection error", "device_id", conn.DeviceID, "error", err)
			}
			return
		}

		var msg RemoteMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			h.logger.Warn("Invalid message from remote", "device_id", conn.DeviceID, "error", err)
			continue
		}

		// Verify HMAC
		ok, err := VerifyMessage(msg, conn.SharedKey)
		if err != nil || !ok {
			h.logger.Warn("HMAC verification failed", "device_id", conn.DeviceID)
			errMsg, _ := NewMessage(MsgError, conn.DeviceID, conn.SharedKey, conn.NextSeq(),
				ErrorPayload{Code: "invalid_hmac", Message: "HMAC verification failed"})
			if errMsg != nil {
				_ = conn.Send(errMsg)
			}
			continue
		}

		// Validate timestamp (anti-replay)
		if err := ValidateTimestamp(msg.Timestamp); err != nil {
			h.logger.Warn("Replay detection", "device_id", conn.DeviceID, "error", err)
			errMsg, _ := NewMessage(MsgError, conn.DeviceID, conn.SharedKey, conn.NextSeq(),
				ErrorPayload{Code: "replay", Message: err.Error()})
			if errMsg != nil {
				_ = conn.Send(errMsg)
			}
			continue
		}
		if h.nonceCache != nil && h.nonceCache.Seen(conn.DeviceID, msg.Nonce, time.Now().UTC()) {
			h.logger.Warn("Nonce replay detected", "device_id", conn.DeviceID, "nonce", msg.Nonce)
			errMsg, _ := NewMessage(MsgError, conn.DeviceID, conn.SharedKey, conn.NextSeq(),
				ErrorPayload{Code: "replay", Message: "nonce replay detected"})
			if errMsg != nil {
				_ = conn.Send(errMsg)
			}
			continue
		}

		switch msg.Type {
		case MsgHeartbeat:
			var hb HeartbeatPayload
			if err := json.Unmarshal(msg.Payload, &hb); err == nil {
				conn.mu.Lock()
				conn.LastHeartbeat = time.Now()
				conn.Telemetry = hb
				conn.Version = hb.Version
				conn.mu.Unlock()

				if h.db != nil {
					_ = UpdateDeviceStatus(h.db, conn.DeviceID, "connected")
				}
				if h.OnHeartbeat != nil {
					h.OnHeartbeat(conn.DeviceID, hb)
				}
			}
		case MsgResult:
			var result ResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err == nil {
				h.pendingMu.Lock()
				ch, ok := h.pending[result.CommandID]
				h.pendingMu.Unlock()
				if ok {
					ch <- &msg
				}
				// Audit log
				if h.db != nil && h.AuditLogEnabled {
					_ = LogAudit(h.db, conn.DeviceID, "result", result.CommandID, result.Status, result.DurationMs)
				}
			}
		case MsgAck:
			h.logger.Debug("Ack received from remote", "device_id", conn.DeviceID, "msg_id", msg.MessageID)
		case MsgError:
			var errPayload ErrorPayload
			if err := json.Unmarshal(msg.Payload, &errPayload); err == nil {
				h.logger.Warn("Error from remote", "device_id", conn.DeviceID, "code", errPayload.Code, "msg", errPayload.Message)
			}
		default:
			h.logger.Warn("Unknown message type from remote", "device_id", conn.DeviceID, "type", msg.Type)
		}
	}
}

// ── Enrollment ──────────────────────────────────────────────────────────────

// HandleEnrollment processes an auth message from a new or returning remote.
func (h *RemoteHub) HandleEnrollment(wsConn *websocket.Conn, msg RemoteMessage) error {
	var auth AuthPayload
	if err := json.Unmarshal(msg.Payload, &auth); err != nil {
		return fmt.Errorf("invalid auth payload: %w", err)
	}

	// ── Case 1: Reconnection (existing device) ──
	if auth.DeviceID != "" {
		device, err := GetDevice(h.db, auth.DeviceID)
		if err != nil {
			return h.sendAuthResponse(wsConn, "", "", "", "rejected", "unknown device", nil, nil)
		}
		if device.Status == "revoked" {
			return h.sendAuthResponse(wsConn, "", "", "", "rejected", "device has been revoked", nil, nil)
		}

		// Verify shared key by looking up vault
		storedKey, err := h.vault.ReadSecret("remote_shared_key_" + device.ID)
		if err != nil {
			return h.sendAuthResponse(wsConn, "", "", "", "rejected", "missing shared key", nil, nil)
		}

		// Verify the incoming message HMAC using stored key
		ok, err := VerifyMessage(msg, storedKey)
		if err != nil || !ok {
			return h.sendAuthResponse(wsConn, storedKey, "", "", "rejected", "authentication failed", nil, nil)
		}

		// Authenticated — register connection
		conn := &RemoteConnection{
			Conn:          wsConn,
			DeviceID:      device.ID,
			Name:          device.Name,
			SharedKey:     storedKey,
			LastHeartbeat: time.Now(),
			Status:        "connected",
			ReadOnly:      device.ReadOnly,
			AllowedPaths:  device.AllowedPaths,
			Version:       auth.Version,
		}
		h.Register(device.ID, conn)
		_ = UpdateDeviceStatus(h.db, device.ID, "connected")

		// Do NOT echo back the shared key — the client already has it (it just used it to sign
		// the auth message). Sending it here would transmit the key over the wire unnecessarily.
		return h.sendAuthResponse(wsConn, storedKey, "", device.ID, "authenticated", "", &conn.ReadOnly, conn.AllowedPaths)
	}

	// ── Case 2: Token-based enrollment ──
	if auth.TokenHash != "" || auth.Token != "" {
		tokenHash := auth.TokenHash
		bootstrapKey := auth.TokenHash
		if tokenHash == "" {
			tokenHash = hashTokenSHA256(auth.Token)
			bootstrapKey = DeriveEnrollmentAuthKey(auth.Token)
		}
		enrollment, err := GetEnrollmentByTokenHash(h.db, tokenHash)
		if err != nil {
			return h.sendAuthResponse(wsConn, bootstrapKey, "", "", "rejected", "invalid enrollment token", nil, nil)
		}
		if msg.HMAC != "" {
			ok, err := VerifyMessage(msg, bootstrapKey)
			if err != nil || !ok {
				return h.sendAuthResponse(wsConn, bootstrapKey, "", "", "rejected", "authentication failed", nil, nil)
			}
		}
		if enrollment.Used {
			// Recovery path: the client lost its stored config.json but still has the
			// original personalized binary with the consumed token. Re-key the device
			// so the client can reconnect without needing a fresh binary download.
			if enrollment.UsedByDevice != "" {
				return h.reKeyDevice(wsConn, auth, enrollment.UsedByDevice, bootstrapKey)
			}
			return h.sendAuthResponse(wsConn, bootstrapKey, "", "", "rejected", "enrollment token already used", nil, nil)
		}
		// Check expiry
		expiry, err := time.Parse(time.RFC3339, enrollment.ExpiresAt)
		if err == nil && time.Now().After(expiry) {
			return h.sendAuthResponse(wsConn, bootstrapKey, "", "", "rejected", "enrollment token expired", nil, nil)
		}

		return h.completeEnrollment(wsConn, auth, enrollment.ID, enrollment.DeviceName, bootstrapKey)
	}

	// ── Case 3: Auto-approve or manual-approval (pending) ──
	if h.AutoApprove {
		// Auto-approve remains intentionally constrained to private or loopback
		// origins. Public unauthenticated joins must still enter the approval flow.
		if isTrustedAutoApproveRemoteAddr(wsConn.RemoteAddr()) {
			return h.completeEnrollment(wsConn, auth, "", auth.Hostname, "")
		}
		h.logger.Warn("Auto-approve bypassed for untrusted remote origin", "remote_addr", wsConn.RemoteAddr().String(), "hostname", auth.Hostname)
	}

	deviceName := auth.Hostname
	if deviceName == "" {
		deviceName = "Unknown Device"
	}
	deviceID, err := CreateDevice(h.db, DeviceRecord{
		Name:      deviceName,
		Hostname:  auth.Hostname,
		OS:        auth.OS,
		Arch:      auth.Arch,
		IPAddress: auth.IP,
		Status:    "pending",
		ReadOnly:  h.DefaultReadOnly,
	})
	if err != nil {
		return h.sendAuthResponse(wsConn, "", "", "", "rejected", "internal error", nil, nil)
	}

	h.logger.Info("New device pending approval", "device_id", deviceID, "hostname", auth.Hostname, "ip", auth.IP)
	return h.sendAuthResponse(wsConn, "", "", deviceID, "pending", "awaiting approval in AuraGo UI", nil, nil)
}

// completeEnrollment generates shared key, creates device, and sends credentials.
func (h *RemoteHub) completeEnrollment(wsConn *websocket.Conn, auth AuthPayload, enrollmentID, deviceName, bootstrapSigningKey string) error {
	sharedKey, err := GenerateSharedKey()
	if err != nil {
		return h.sendAuthResponse(wsConn, bootstrapSigningKey, "", "", "rejected", "key generation failed", nil, nil)
	}

	name := deviceName
	if name == "" {
		name = auth.Hostname
	}
	if name == "" {
		name = "Remote Device"
	}

	keyHash := hashTokenSHA256(sharedKey)
	deviceID, err := CreateDevice(h.db, DeviceRecord{
		Name:          name,
		Hostname:      auth.Hostname,
		OS:            auth.OS,
		Arch:          auth.Arch,
		IPAddress:     auth.IP,
		Status:        "approved",
		ReadOnly:      h.DefaultReadOnly,
		SharedKeyHash: keyHash,
	})
	if err != nil {
		return h.sendAuthResponse(wsConn, bootstrapSigningKey, "", "", "rejected", "device registration failed", nil, nil)
	}

	// Store shared key in vault
	if err := h.vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil {
		h.logger.Error("Failed to store shared key in vault", "device_id", deviceID, "error", err)
	}

	// Mark enrollment as used
	if enrollmentID != "" {
		_ = MarkEnrollmentUsed(h.db, enrollmentID, deviceID)
	}

	// Register connection
	conn := &RemoteConnection{
		Conn:          wsConn,
		DeviceID:      deviceID,
		Name:          name,
		SharedKey:     sharedKey,
		LastHeartbeat: time.Now(),
		Status:        "connected",
		ReadOnly:      h.DefaultReadOnly,
		Version:       auth.Version,
	}
	h.Register(deviceID, conn)
	_ = UpdateDeviceStatus(h.db, deviceID, "connected")

	return h.sendAuthResponse(wsConn, bootstrapSigningKey, sharedKey, deviceID, "enrolled", "", &conn.ReadOnly, conn.AllowedPaths)
}

// reKeyDevice re-generates the shared key for an existing device.
// This is the recovery path when the client's stored config was lost but the
// original binary (with the consumed enrollment token) is still available.
func (h *RemoteHub) reKeyDevice(wsConn *websocket.Conn, auth AuthPayload, deviceID, bootstrapSigningKey string) error {
	device, err := GetDevice(h.db, deviceID)
	if err != nil {
		return h.sendAuthResponse(wsConn, bootstrapSigningKey, "", "", "rejected", "original device not found", nil, nil)
	}
	if device.Status == "revoked" {
		return h.sendAuthResponse(wsConn, bootstrapSigningKey, "", "", "rejected", "device has been revoked", nil, nil)
	}

	newKey, err := GenerateSharedKey()
	if err != nil {
		return h.sendAuthResponse(wsConn, bootstrapSigningKey, "", "", "rejected", "key generation failed", nil, nil)
	}

	device.SharedKeyHash = hashTokenSHA256(newKey)
	device.Status = "approved"
	device.Hostname = auth.Hostname
	device.OS = auth.OS
	device.Arch = auth.Arch
	device.IPAddress = auth.IP
	if err := UpdateDevice(h.db, device); err != nil {
		return h.sendAuthResponse(wsConn, bootstrapSigningKey, "", "", "rejected", "device update failed", nil, nil)
	}

	if err := h.vault.WriteSecret("remote_shared_key_"+deviceID, newKey); err != nil {
		h.logger.Error("Failed to store re-keyed shared key", "device_id", deviceID, "error", err)
	}

	conn := &RemoteConnection{
		Conn:          wsConn,
		DeviceID:      deviceID,
		Name:          device.Name,
		SharedKey:     newKey,
		LastHeartbeat: time.Now(),
		Status:        "connected",
		ReadOnly:      device.ReadOnly,
		AllowedPaths:  device.AllowedPaths,
		Version:       auth.Version,
	}
	h.Register(deviceID, conn)
	_ = UpdateDeviceStatus(h.db, deviceID, "connected")

	h.logger.Info("Device re-keyed after config loss", "device_id", deviceID, "name", device.Name)
	// Send "enrolled" so the client saves the new shared key and device_id.
	return h.sendAuthResponse(wsConn, bootstrapSigningKey, newKey, deviceID, "enrolled", "", &conn.ReadOnly, conn.AllowedPaths)
}

// ApproveDevice approves a pending device and generates credentials.
// Returns the shared key for delivery to the device on next connection.
func (h *RemoteHub) ApproveDevice(deviceID string) error {
	device, err := GetDevice(h.db, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	if device.Status != "pending" {
		return fmt.Errorf("device is not pending approval (status: %s)", device.Status)
	}

	sharedKey, err := GenerateSharedKey()
	if err != nil {
		return fmt.Errorf("key generation failed: %w", err)
	}

	device.Status = "approved"
	device.SharedKeyHash = hashTokenSHA256(sharedKey)
	if err := UpdateDevice(h.db, device); err != nil {
		return fmt.Errorf("failed to update device: %w", err)
	}

	if err := h.vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil {
		return fmt.Errorf("failed to store shared key in vault: %w", err)
	}

	h.logger.Info("Device approved", "device_id", deviceID, "name", device.Name)
	return nil
}

// RejectDevice rejects a pending device.
func (h *RemoteHub) RejectDevice(deviceID string) error {
	device, err := GetDevice(h.db, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	if device.Status != "pending" {
		return fmt.Errorf("device is not pending approval (status: %s)", device.Status)
	}
	return DeleteDevice(h.db, deviceID)
}

func (h *RemoteHub) sendAuthResponse(wsConn *websocket.Conn, signingKeyHex, sharedKey, deviceID, status, message string, readOnly *bool, allowedPaths []string) error {
	resp := AuthResponsePayload{
		Status:        status,
		DeviceID:      deviceID,
		SharedKey:     sharedKey,
		Message:       message,
		ReadOnly:      readOnly,
		AllowedPaths:  allowedPaths,
		MaxFileSizeMB: h.effectiveMaxFileSizeMB(),
	}
	msg, err := NewAuthResponseMessage(deviceID, signingKeyHex, resp)
	if err != nil {
		return err
	}
	return wsConn.WriteJSON(msg)
}

func hashTokenSHA256(token string) string {
	return DeriveEnrollmentAuthKey(token)
}

func (h *RemoteHub) effectiveMaxFileSizeMB() int {
	if h.MaxFileSizeMB <= 0 {
		return DefaultMaxFileSizeMB
	}
	return h.MaxFileSizeMB
}

func isTrustedAutoApproveRemoteAddr(addr net.Addr) bool {
	if addr == nil {
		return false
	}
	host := addr.String()
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// ── Heartbeat monitor ───────────────────────────────────────────────────────

// StartHeartbeatMonitor periodically checks all connections for stale heartbeats.
func (h *RemoteHub) StartHeartbeatMonitor(interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			h.mu.RLock()
			var stale []string
			for deviceID, conn := range h.connections {
				if !conn.LastHeartbeat.IsZero() && time.Since(conn.LastHeartbeat) > maxAge {
					stale = append(stale, deviceID)
				}
			}
			h.mu.RUnlock()

			for _, id := range stale {
				h.logger.Warn("Remote heartbeat stale, disconnecting", "device_id", id)
				h.Unregister(id)
			}
		}
	}()
}

// ── Binary builder ──────────────────────────────────────────────────────────

// TrailerMagic is the magic string appended to personalized binaries.
const TrailerMagic = "AURAGO_REMOTE_CONFIG_V1\x00"

// BinaryConfig is injected into the binary trailer for personalized downloads.
type BinaryConfig struct {
	SupervisorURL string `json:"supervisor_url"`
	CACert        string `json:"ca_cert,omitempty"` // PEM-encoded
	EnrollToken   string `json:"enroll_token"`
	DeviceName    string `json:"device_name,omitempty"`
}

// BuildPersonalizedBinary reads a generic binary and appends a config trailer.
func BuildPersonalizedBinary(genericBinary []byte, cfg BinaryConfig) ([]byte, error) {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal binary config: %w", err)
	}

	payloadLen := uint32(len(payload))
	magic := []byte(TrailerMagic)

	// [binary][JSON payload][uint32 length LE][magic]
	result := make([]byte, len(genericBinary)+len(payload)+4+len(magic))
	copy(result, genericBinary)
	offset := len(genericBinary)
	copy(result[offset:], payload)
	offset += len(payload)
	result[offset] = byte(payloadLen)
	result[offset+1] = byte(payloadLen >> 8)
	result[offset+2] = byte(payloadLen >> 16)
	result[offset+3] = byte(payloadLen >> 24)
	offset += 4
	copy(result[offset:], magic)

	return result, nil
}

// ParseBinaryTrailer reads the config trailer from a binary (used by the remote client at startup).
func ParseBinaryTrailer(data []byte) (*BinaryConfig, error) {
	magic := []byte(TrailerMagic)
	magicLen := len(magic)

	if len(data) < magicLen+4 {
		return nil, fmt.Errorf("binary too small for trailer")
	}

	// Check magic at the end
	tail := data[len(data)-magicLen:]
	for i := range magic {
		if tail[i] != magic[i] {
			return nil, fmt.Errorf("no trailer found (magic mismatch)")
		}
	}

	// Read payload length (uint32 LE before magic)
	lenOffset := len(data) - magicLen - 4
	payloadLen := uint32(data[lenOffset]) |
		uint32(data[lenOffset+1])<<8 |
		uint32(data[lenOffset+2])<<16 |
		uint32(data[lenOffset+3])<<24

	if payloadLen > 1<<20 { // sanity: max 1MB
		return nil, fmt.Errorf("trailer payload too large: %d bytes", payloadLen)
	}

	payloadStart := lenOffset - int(payloadLen)
	if payloadStart < 0 {
		return nil, fmt.Errorf("invalid trailer payload length")
	}

	var cfg BinaryConfig
	if err := json.Unmarshal(data[payloadStart:lenOffset], &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse trailer config: %w", err)
	}
	return &cfg, nil
}
