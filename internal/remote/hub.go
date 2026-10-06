//go:build !remote_minimal

package remote

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
	"aurago/internal/uid"

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

// CommandTransport dispatches RemoteHub commands over non-RemoteMessage
// transports such as the agodesk desktop companion WebSocket.
type CommandTransport interface {
	IsConnected(deviceID string) bool
	SendCommand(deviceID string, cmd CommandPayload, timeout time.Duration) (ResultPayload, error)
}

// RemoteAuditEvent is a normalized remote-control event emitted to the supervisor audit sink.
type RemoteAuditEvent struct {
	DeviceID   string
	DeviceName string
	EventType  string
	Status     string
	Summary    string
	Detail     string
	DurationMS int64
	Metadata   map[string]interface{}
}

// Send writes a signed message to the remote. The websocket write stays under
// rc.mu because gorilla/websocket permits only one concurrent writer.
// Serialization happens first so marshal failures do not hold the write lock.
func (rc *RemoteConnection) Send(msg *RemoteMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal remote message: %w", err)
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.Conn.WriteMessage(websocket.TextMessage, data)
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
	mu            sync.RWMutex
	connections   map[string]*RemoteConnection   // device_id → conn
	connIndex     map[*websocket.Conn]string     // websocket conn → device_id
	transports    map[string]CommandTransport    // name → alternate command transport
	pending       map[string]chan *RemoteMessage // cmd_id → result channel
	pendingMu     sync.Mutex
	pendingOwners map[string]*RemoteConnection
	disabled      bool
	disableCh     chan struct{}
	monitorMu     sync.Mutex
	monitorCancel context.CancelFunc
	monitorDone   chan struct{}
	enrollmentMu  sync.Mutex // serializes bounded unauthenticated pending registrations
	// enrollmentKeyMu serializes token issuance with the sweep of enrollment
	// MAC keys, so the sweep never sees a new key before its row is committed.
	enrollmentKeyMu sync.Mutex
	db              *sql.DB
	vault           *security.Vault
	logger          *slog.Logger

	// Config-driven defaults (set by caller after construction)
	DefaultReadOnly bool // default read-only setting for newly enrolled devices
	AutoApprove     bool // auto-approve devices with no enrollment token
	MaxFileSizeMB   int
	AuditLogEnabled bool
	// DefaultAllowedPaths returns the global remote_control.allowed_paths,
	// which a device whose own list is empty uses instead (see
	// effectiveAllowedPaths). It is called per authentication, shell command
	// and config push, so the hub always evaluates the current default; call
	// PushDefaultAllowedPaths after it changes so connected agents get it too.
	// Nil means no default.
	DefaultAllowedPaths func() []string

	nonceCache *nonceReplayCache

	// Callbacks
	OnConnect    func(deviceID, name string)
	OnDisconnect func(deviceID, name string)
	OnHeartbeat  func(deviceID string, hb HeartbeatPayload)
	OnAudit      func(event RemoteAuditEvent)
}

// NewRemoteHub creates a new hub for managing remote connections.
func NewRemoteHub(db *sql.DB, vault *security.Vault, logger *slog.Logger) *RemoteHub {
	return &RemoteHub{
		connections:     make(map[string]*RemoteConnection),
		connIndex:       make(map[*websocket.Conn]string),
		transports:      make(map[string]CommandTransport),
		pending:         make(map[string]chan *RemoteMessage),
		pendingOwners:   make(map[string]*RemoteConnection),
		disableCh:       make(chan struct{}),
		db:              db,
		vault:           vault,
		logger:          logger,
		MaxFileSizeMB:   DefaultMaxFileSizeMB,
		AuditLogEnabled: true,
		// ValidateTimestamp accepts ±MaxTimestampDrift, so a nonce must stay
		// cached for the full window.
		nonceCache: newNonceReplayCache(NonceReplayTTL, 10000),
	}
}

// DB returns the underlying database handle.
func (h *RemoteHub) DB() *sql.DB {
	return h.db
}

// Register adds an authenticated remote connection to the hub.
func (h *RemoteHub) Register(deviceID string, conn *RemoteConnection) {
	var old *RemoteConnection
	h.mu.Lock()
	if h.disabled {
		h.mu.Unlock()
		if conn != nil && conn.Conn != nil {
			_ = conn.Conn.Close()
		}
		return
	}
	if h.connections == nil {
		h.connections = make(map[string]*RemoteConnection)
	}
	if h.connIndex == nil {
		h.connIndex = make(map[*websocket.Conn]string)
	}
	if existing, ok := h.connections[deviceID]; ok && existing != conn {
		old = existing
		h.logger.Warn("Replacing existing remote connection", "device_id", deviceID)
		if old.Conn != nil {
			delete(h.connIndex, old.Conn)
		}
	}
	h.connections[deviceID] = conn
	if conn != nil && conn.Conn != nil {
		h.connIndex[conn.Conn] = deviceID
	}
	if h.db != nil {
		_ = UpdateDeviceStatus(h.db, deviceID, "connected")
	}
	h.mu.Unlock()

	if old != nil && old.Conn != nil && (conn == nil || old.Conn != conn.Conn) {
		_ = old.Conn.Close()
	}

	h.logger.Info("Remote connected", "device_id", deviceID, "name", conn.Name)
	h.emitAudit(RemoteAuditEvent{
		DeviceID:   deviceID,
		DeviceName: conn.Name,
		EventType:  "remote_connect",
		Status:     "success",
		Summary:    "Remote device connected",
	})
	if h.OnConnect != nil {
		h.OnConnect(deviceID, conn.Name)
	}
}

// Unregister removes a remote connection from the hub.
func (h *RemoteHub) Unregister(deviceID string) {
	h.unregisterConnection(deviceID, nil)
}

func (h *RemoteHub) unregisterConnection(deviceID string, expected *RemoteConnection) {
	h.mu.Lock()
	conn, ok := h.connections[deviceID]
	if ok {
		if expected != nil && conn != expected {
			h.mu.Unlock()
			return
		}
		delete(h.connections, deviceID)
		if conn.Conn != nil {
			delete(h.connIndex, conn.Conn)
		}
		if h.db != nil {
			_ = UpdateDeviceStatus(h.db, deviceID, "offline")
		}
	}
	h.mu.Unlock()

	if ok {
		if conn.Conn != nil {
			_ = conn.Conn.Close()
		}
		h.logger.Info("Remote disconnected", "device_id", deviceID, "name", conn.Name)

		h.emitAudit(RemoteAuditEvent{
			DeviceID:   deviceID,
			DeviceName: conn.Name,
			EventType:  "remote_disconnect",
			Status:     "warning",
			Summary:    "Remote device disconnected",
		})
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
	_, ok := h.connections[deviceID]
	h.mu.RUnlock()
	if ok {
		return true
	}
	return h.hasConnectedCommandTransport(deviceID)
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

// FindByConn atomically finds the device ID whose connection matches the
// given websocket.Conn pointer. Returns ("", nil) if not found.
func (h *RemoteHub) FindByConn(wsConn *websocket.Conn) (string, *RemoteConnection) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	id, ok := h.connIndex[wsConn]
	if !ok {
		return "", nil
	}
	conn := h.connections[id]
	if conn == nil || conn.Conn != wsConn {
		return "", nil
	}
	return id, conn
}

// ConnectionCount returns the number of connected remotes.
func (h *RemoteHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// RegisterCommandTransport adds an alternate command transport.
func (h *RemoteHub) RegisterCommandTransport(name string, transport CommandTransport) {
	if h == nil || strings.TrimSpace(name) == "" || transport == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.transports == nil {
		h.transports = make(map[string]CommandTransport)
	}
	h.transports[name] = transport
}

// UnregisterCommandTransport removes an alternate command transport.
func (h *RemoteHub) UnregisterCommandTransport(name string) {
	if h == nil || strings.TrimSpace(name) == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.transports, name)
}

// ── Command dispatch ────────────────────────────────────────────────────────

// SendCommand sends a command to a remote and waits for the result.
func (h *RemoteHub) SendCommand(deviceID string, cmd CommandPayload, timeout time.Duration) (ResultPayload, error) {
	h.mu.RLock()
	disabled, disableCh := h.disabled, h.disableCh
	h.mu.RUnlock()
	if disabled {
		return ResultPayload{}, fmt.Errorf("remote control is disabled")
	}
	cmd = prepareCommandPayload(cmd, timeout)
	conn := h.GetConnection(deviceID)
	if h.commandBlockedByReadOnly(deviceID, conn, cmd.Operation) {
		return ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "denied",
			ErrorCode: "REMOTE_READ_ONLY",
			Error:     "device is in read-only mode",
		}, nil
	}
	if h.commandBlockedByMissingAllowedPaths(conn, cmd.Operation) {
		return ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "denied",
			ErrorCode: ShellRequiresAllowedPathsCode,
			Error:     ShellRequiresAllowedPathsMessage,
		}, nil
	}
	if conn == nil {
		if transport := h.connectedCommandTransport(deviceID); transport != nil {
			return transport.SendCommand(deviceID, cmd, timeout)
		}
		return ResultPayload{}, fmt.Errorf("no active connection for device %s", deviceID)
	}

	msg, err := NewMessage(MsgCommand, deviceID, conn.SharedKey, conn.NextSeq(), cmd)
	if err != nil {
		return ResultPayload{}, fmt.Errorf("failed to create command message: %w", err)
	}

	// Create result channel
	resultCh := make(chan *RemoteMessage, 1)
	h.pendingMu.Lock()
	if _, exists := h.pending[cmd.CommandID]; exists {
		h.pendingMu.Unlock()
		return ResultPayload{}, fmt.Errorf("command ID is already pending")
	}
	h.pending[cmd.CommandID] = resultCh
	h.pendingOwners[cmd.CommandID] = conn
	h.pendingMu.Unlock()

	defer func() {
		h.pendingMu.Lock()
		delete(h.pending, cmd.CommandID)
		delete(h.pendingOwners, cmd.CommandID)
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
	case <-disableCh:
		return ResultPayload{}, fmt.Errorf("remote control was disabled")
	case <-time.After(timeout):
		return ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "timeout",
			Error:     fmt.Sprintf("command timed out after %v", timeout),
		}, nil
	}
}

func prepareCommandPayload(cmd CommandPayload, timeout time.Duration) CommandPayload {
	if strings.TrimSpace(cmd.CommandID) == "" {
		if nonce, err := GenerateNonce(); err == nil {
			cmd.CommandID = "cmd-" + nonce
		} else {
			cmd.CommandID = fmt.Sprintf("cmd-%d", time.Now().UnixNano())
		}
	}
	if timeout > 0 && cmd.TimeoutSec <= 0 {
		cmd.TimeoutSec = int((timeout + time.Second - time.Nanosecond) / time.Second)
	}
	return cmd
}

func (h *RemoteHub) commandBlockedByReadOnly(deviceID string, conn *RemoteConnection, operation string) bool {
	if ReadOnlySafe(operation) {
		return false
	}
	if conn != nil {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return conn.ReadOnly
	}
	if h == nil || h.db == nil {
		return false
	}
	device, err := GetDevice(h.db, deviceID)
	return err == nil && device.ReadOnly
}

// commandBlockedByMissingAllowedPaths reports whether a shell operation must be
// refused because the connected device has no effective allowed paths. The
// connection holds the device's own list; the global default is evaluated
// here, live, so clearing it refuses shell at once. Without a RemoteConnection
// the check stays out of the way: dispatch either reports the missing
// connection or hands the command to a command transport (AgoDesk), which
// never receives allowed_paths and gates shell access by its own advertised
// capability and locally configured working directories.
func (h *RemoteHub) commandBlockedByMissingAllowedPaths(conn *RemoteConnection, operation string) bool {
	if conn == nil || !IsShellOperation(operation) {
		return false
	}
	conn.mu.Lock()
	ownPaths := conn.AllowedPaths
	conn.mu.Unlock()
	return len(h.effectiveAllowedPaths(ownPaths)) == 0
}

// EffectiveAllowedPaths returns the allowed paths that apply to deviceID: its
// own list (from the live connection, else the stored record), or the global
// default when that is empty. AgoDesk records get only their own list. It
// never returns nil.
func (h *RemoteHub) EffectiveAllowedPaths(deviceID string) []string {
	if h == nil {
		return []string{}
	}
	var ownPaths []string
	if conn := h.GetConnection(deviceID); conn != nil {
		conn.mu.Lock()
		ownPaths = conn.AllowedPaths
		conn.mu.Unlock()
	} else if h.db != nil {
		if device, err := GetDevice(h.db, deviceID); err == nil {
			// An AgoDesk companion never receives allowed_paths, so the
			// global default does not apply to it.
			if isAgodeskDevice(device) {
				return cleanAllowedPaths(device.AllowedPaths)
			}
			ownPaths = device.AllowedPaths
		}
	}
	return h.effectiveAllowedPaths(ownPaths)
}

// isAgodeskDevice reports whether the record is an AgoDesk desktop companion,
// which pairing stores with the "agodesk" tag.
func isAgodeskDevice(device DeviceRecord) bool {
	for _, tag := range device.Tags {
		if strings.EqualFold(strings.TrimSpace(tag), "agodesk") {
			return true
		}
	}
	return false
}

// PushDefaultAllowedPaths sends a full allowed_paths snapshot to every
// connected device without a list of its own (per its stored record, else its
// connection), so an agent that authenticated before the global default
// changed receives the current effective list without reconnecting. Call it
// after DefaultAllowedPaths starts returning a different list.
func (h *RemoteHub) PushDefaultAllowedPaths() {
	if h == nil {
		return
	}
	h.mu.RLock()
	conns := make([]*RemoteConnection, 0, len(h.connections))
	for _, conn := range h.connections {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()
	for _, conn := range conns {
		conn.mu.Lock()
		ownPaths := conn.AllowedPaths
		conn.mu.Unlock()
		if h.db != nil {
			if device, err := GetDevice(h.db, conn.DeviceID); err == nil {
				ownPaths = device.AllowedPaths
			}
		}
		if len(cleanAllowedPaths(ownPaths)) > 0 {
			continue
		}
		if err := h.SendConfigUpdate(conn.DeviceID, ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
			h.logger.Warn("Failed to push the default allowed paths", "device_id", conn.DeviceID, "error", err)
		}
	}
}

// effectiveAllowedPaths returns the paths a device may use: a copy of its own
// list when that names any path, else a copy of DefaultAllowedPaths. Blank
// entries are dropped and the rest trimmed. The result is never nil, so an
// empty one still reaches the agent as an explicit revocation.
func (h *RemoteHub) effectiveAllowedPaths(devicePaths []string) []string {
	if paths := cleanAllowedPaths(devicePaths); len(paths) > 0 {
		return paths
	}
	if h == nil || h.DefaultAllowedPaths == nil {
		return []string{}
	}
	return cleanAllowedPaths(h.DefaultAllowedPaths())
}

func cleanAllowedPaths(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		if path = strings.TrimSpace(path); path != "" {
			cleaned = append(cleaned, path)
		}
	}
	return cleaned
}

func (h *RemoteHub) hasConnectedCommandTransport(deviceID string) bool {
	return h.connectedCommandTransport(deviceID) != nil
}

func (h *RemoteHub) connectedCommandTransport(deviceID string) CommandTransport {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	transports := make([]CommandTransport, 0, len(h.transports))
	for _, transport := range h.transports {
		if transport != nil {
			transports = append(transports, transport)
		}
	}
	h.mu.RUnlock()
	for _, transport := range transports {
		if transport.IsConnected(deviceID) {
			return transport
		}
	}
	return nil
}

// SendConfigUpdate pushes config changes to a remote device and updates the
// in-memory connection state so server-side enforcement reflects the new values.
// A full snapshot's allowed_paths (non-nil) is the device's own list: the
// connection keeps it, and the agent receives the effective list, so a cleared
// device list falls back to DefaultAllowedPaths and only an empty result
// revokes.
func (h *RemoteHub) SendConfigUpdate(deviceID string, update ConfigUpdatePayload) error {
	conn := h.GetConnection(deviceID)
	if conn == nil {
		return fmt.Errorf("no active connection for device %s", deviceID)
	}
	ownPaths := update.AllowedPaths
	if ownPaths != nil {
		update.AllowedPaths = h.effectiveAllowedPaths(ownPaths)
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
	if ownPaths != nil {
		conn.AllowedPaths = append([]string{}, ownPaths...)
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

// maxConsecutiveBadFrames is how many frames in a row may fail authentication,
// nonce-format, timestamp or replay checks before HandleMessages hangs up. Each
// rejection is answered with a freshly signed error frame, which an on-path
// attacker could relay to the agent, so the run must stay short.
const maxConsecutiveBadFrames = 8

// HandleMessages reads messages from a remote connection and dispatches them.
// Blocks until the connection closes or an error occurs.
func (h *RemoteHub) HandleMessages(conn *RemoteConnection) {
	defer h.unregisterConnection(conn.DeviceID, conn)

	badFrames := 0
	// rejectFrame answers a frame that failed a check with a signed error
	// frame. Once maxConsecutiveBadFrames have arrived in a row it closes the
	// connection without answering and reports false; the caller must return.
	rejectFrame := func(code, message string) bool {
		badFrames++
		if badFrames >= maxConsecutiveBadFrames {
			h.logger.Warn("Closing remote connection after repeated bad frames",
				"device_id", conn.DeviceID, "count", badFrames)
			_ = conn.Conn.Close()
			return false
		}
		errMsg, _ := NewMessage(MsgError, conn.DeviceID, conn.SharedKey, conn.NextSeq(),
			ErrorPayload{Code: code, Message: message})
		if errMsg != nil {
			_ = conn.Send(errMsg)
		}
		return true
	}

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

		if msg.DeviceID != conn.DeviceID || h.GetConnection(conn.DeviceID) != conn {
			continue
		}

		// Verify HMAC
		ok, err := VerifyMessage(msg, conn.SharedKey)
		if err != nil || !ok {
			h.logger.Warn("HMAC verification failed", "device_id", conn.DeviceID)
			if !rejectFrame("invalid_hmac", "HMAC verification failed") {
				return
			}
			continue
		}

		// A cheap sanity check before the replay cache; the canonical HMAC
		// form already keeps sequence digits out of the nonce.
		if !ValidNonce(msg.Nonce) {
			h.logger.Warn("Invalid nonce format", "device_id", conn.DeviceID)
			if !rejectFrame("replay", "invalid nonce format") {
				return
			}
			continue
		}

		// Validate timestamp (anti-replay)
		if err := ValidateTimestamp(msg.Timestamp); err != nil {
			h.logger.Warn("Replay detection", "device_id", conn.DeviceID, "error", err)
			if !rejectFrame("replay", err.Error()) {
				return
			}
			continue
		}
		if h.nonceCache != nil && h.nonceCache.Seen(conn.DeviceID, msg.Nonce, time.Now().UTC()) {
			h.logger.Warn("Nonce replay detected", "device_id", conn.DeviceID, "nonce", msg.Nonce)
			if !rejectFrame("replay", "nonce replay detected") {
				return
			}
			continue
		}
		badFrames = 0

		switch msg.Type {
		case MsgHeartbeat:
			var hb HeartbeatPayload
			if err := json.Unmarshal(msg.Payload, &hb); err == nil {
				conn.mu.Lock()
				conn.LastHeartbeat = time.Now()
				conn.Telemetry = hb
				conn.Version = hb.Version
				conn.mu.Unlock()

				h.mu.Lock()
				current := h.connections[conn.DeviceID] == conn
				if current && h.db != nil {
					_ = UpdateDeviceStatus(h.db, conn.DeviceID, "connected")
				}
				h.mu.Unlock()
				if !current {
					continue
				}
				if h.OnHeartbeat != nil {
					h.OnHeartbeat(conn.DeviceID, hb)
				}
				h.emitAudit(RemoteAuditEvent{
					DeviceID:   conn.DeviceID,
					DeviceName: conn.Name,
					EventType:  "remote_heartbeat",
					Status:     "success",
					Summary:    "Remote heartbeat received",
					Detail:     fmt.Sprintf("CPU %.1f%%, memory %.1f%%, version %s", hb.CPUPercent, hb.MemPercent, hb.Version),
					Metadata: map[string]interface{}{
						"hostname": hb.Hostname,
						"os":       hb.OS,
						"arch":     hb.Arch,
						"version":  hb.Version,
					},
				})
			}
		case MsgResult:
			var result ResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err == nil {
				h.mu.RLock()
				h.pendingMu.Lock()
				ch, ok := h.pending[result.CommandID]
				ok = ok && h.pendingOwners[result.CommandID] == conn && h.connections[conn.DeviceID] == conn && !h.disabled
				if ok {
					select {
					case ch <- &msg:
					default:
					}
				}
				h.pendingMu.Unlock()
				h.mu.RUnlock()
				if !ok {
					continue
				}

				// Audit log
				if h.db != nil && h.AuditLogEnabled {
					_ = LogAudit(h.db, conn.DeviceID, "result", result.CommandID, result.Status, result.DurationMs)
				}
				h.emitAudit(RemoteAuditEvent{
					DeviceID:   conn.DeviceID,
					DeviceName: conn.Name,
					EventType:  "remote_command",
					Status:     remoteAuditStatus(result.Status),
					Summary:    "Remote command result: " + result.Status,
					Detail:     result.Error,
					DurationMS: result.DurationMs,
					Metadata: map[string]interface{}{
						"command_id": result.CommandID,
						"status":     result.Status,
					},
				})
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

func (h *RemoteHub) emitAudit(event RemoteAuditEvent) {
	if h == nil || h.OnAudit == nil {
		return
	}
	h.OnAudit(event)
}

func remoteAuditStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "ok", "completed", "done":
		return "success"
	case "denied", "timeout", "error", "failed", "failure":
		return "error"
	case "":
		return "warning"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

// ── Enrollment ──────────────────────────────────────────────────────────────

// HandleEnrollment processes an auth message from a new or returning remote.
func (h *RemoteHub) HandleEnrollment(wsConn *websocket.Conn, msg RemoteMessage) error {
	if !h.Enabled() {
		return fmt.Errorf("remote control is disabled")
	}
	var auth AuthPayload
	if err := json.Unmarshal(msg.Payload, &auth); err != nil {
		return fmt.Errorf("invalid auth payload: %w", err)
	}

	// Unversioned frames come from agents built before the canonical HMAC
	// form. Nothing in them is verified or accepted. A pre-upgrade enrollment
	// frame, or a reconnect naming a known device, gets a refusal that agent
	// can read; every other one is refused unsigned. The agent binary must be
	// replaced (an enrolled device keeps its device key, so the new binary
	// reconnects without re-enrolling).
	if msg.Version != FrameVersion {
		if msg.Version < FrameVersion && auth.DeviceID == "" && auth.TokenHash != "" {
			return h.rejectPreUpgradeEnrollment(wsConn, msg, auth)
		}
		h.logger.Warn("Refusing remote auth frame with an unsupported frame version; replace the agent binary",
			"version", msg.Version, "device_id", auth.DeviceID, "hostname", auth.Hostname)
		if msg.Version < FrameVersion && auth.DeviceID != "" {
			return h.rejectPreUpgradeReconnect(wsConn, msg, auth.DeviceID)
		}
		return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "unsupported frame version", nil, nil)
	}

	// ── Case 1: Reconnection (existing device) ──
	if auth.DeviceID != "" {
		device, err := GetDevice(h.db, auth.DeviceID)
		if err != nil {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "unknown device", nil, nil)
		}
		if device.Status == "revoked" {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "device has been revoked", nil, nil)
		}

		// Verify shared key by looking up vault
		storedKey, err := h.vault.ReadSecret("remote_shared_key_" + device.ID)
		if err != nil {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "missing shared key", nil, nil)
		}

		// Verify the incoming message HMAC using stored key. The refusal of
		// a frame that fails is unsigned: a signed one would hand an
		// unauthenticated requester a frame only the supervisor can make.
		ok, err := VerifyMessage(msg, storedKey)
		if err != nil || !ok {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "authentication failed", nil, nil)
		}
		if err := ValidateTimestamp(msg.Timestamp); err != nil || !ValidNonce(msg.Nonce) || h.nonceCache.Seen(device.ID, msg.Nonce, time.Now().UTC()) {
			return h.sendAuthResponse(wsConn, msg.Nonce, storedKey, "", "", "rejected", "stale or replayed authentication", nil, nil)
		}
		if err := UpdateDeviceStatus(h.db, device.ID, "connected"); err != nil {
			return h.sendAuthResponse(wsConn, msg.Nonce, storedKey, "", "", "rejected", "device status update failed", nil, nil)
		}

		// Authenticated — register connection. The connection keeps the
		// device's own list; the agent receives the effective one.
		allowedPaths := h.effectiveAllowedPaths(device.AllowedPaths)
		readOnly := device.ReadOnly
		conn := &RemoteConnection{
			Conn:          wsConn,
			DeviceID:      device.ID,
			Name:          device.Name,
			SharedKey:     storedKey,
			LastHeartbeat: time.Now(),
			Status:        "connected",
			ReadOnly:      readOnly,
			AllowedPaths:  append([]string{}, device.AllowedPaths...),
			Version:       auth.Version,
		}
		h.Register(device.ID, conn)

		// Do NOT echo back the shared key — the client already has it (it just used it to sign
		// the auth message). Sending it here would transmit the key over the wire unnecessarily.
		return h.sendAuthResponse(wsConn, msg.Nonce, storedKey, "", device.ID, "authenticated", "", &readOnly, allowedPaths)
	}

	// ── Case 2: Token-based enrollment ──
	// auth.TokenHash is only the lookup hash and travels in the clear. The MAC
	// key comes from the vault, never from the frame, so whoever sees this
	// frame cannot sign the answer. Refusals before the frame has verified
	// are unsigned: signing them with the MAC key would hand the requester a
	// signed answer it could not have made itself.
	if auth.TokenHash != "" {
		if auth.KDF != EnrollmentKDFVersion {
			return h.rejectPreUpgradeEnrollment(wsConn, msg, auth)
		}
		// Rows issued before the key split are stored under the plain token
		// hash, which a current agent's lookup hash never matches, so they
		// land here too.
		enrollment, err := GetEnrollmentByTokenHash(h.db, auth.TokenHash)
		if err != nil {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", unknownEnrollmentMessage, nil, nil)
		}
		// The MAC key of a consumed token is gone, so this is decided first.
		if enrollment.Used {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "enrollment token already used", nil, nil)
		}
		if msg.HMAC == "" {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "HMAC required for token enrollment", nil, nil)
		}
		if h.vault == nil {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "credential storage unavailable", nil, nil)
		}
		authKey, err := h.vault.ReadSecret(enrollmentAuthKeyName(enrollment.ID))
		if errors.Is(err, security.ErrSecretNotFound) {
			h.logger.Warn("Remote enrollment token has no MAC key in the vault", "enrollment_id", enrollment.ID)
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", keylessEnrollmentMessage, nil, nil)
		}
		if err != nil {
			h.logger.Error("Failed to read remote enrollment key from vault", "enrollment_id", enrollment.ID, "error", err)
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "credential storage unavailable", nil, nil)
		}
		if ok, err := VerifyMessage(msg, authKey); err != nil || !ok {
			return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "authentication failed", nil, nil)
		}
		if err := ValidateTimestamp(msg.Timestamp); err != nil || !ValidNonce(msg.Nonce) || h.nonceCache.Seen(enrollment.ID, msg.Nonce, time.Now().UTC()) {
			return h.sendAuthResponse(wsConn, msg.Nonce, authKey, "", "", "rejected", "stale or replayed authentication", nil, nil)
		}
		// Check expiry
		expiry, err := time.Parse(time.RFC3339, enrollment.ExpiresAt)
		if err != nil || time.Now().After(expiry) {
			return h.sendAuthResponse(wsConn, msg.Nonce, authKey, "", "", "rejected", "enrollment token expired", nil, nil)
		}

		return h.completeEnrollment(wsConn, msg.Nonce, auth, enrollment.ID, enrollment.DeviceName, authKey)
	}

	// ── Case 3: Auto-approve or manual-approval (pending) ──
	// A private peer address can be a reverse proxy or NAT. It is not identity.
	// Tokenless joins always need an explicit administrator-issued token.

	deviceName := auth.Hostname
	if deviceName == "" {
		deviceName = "Unknown Device"
	}
	h.enrollmentMu.Lock()
	defer h.enrollmentMu.Unlock()
	peerHost, _, _ := net.SplitHostPort(wsConn.RemoteAddr().String())
	var deviceID string
	err := h.db.QueryRow(`SELECT id FROM remote_devices WHERE status='pending' AND hostname=? AND ip_address=? LIMIT 1`, auth.Hostname, peerHost).Scan(&deviceID)
	if err == nil {
		return h.sendAuthResponse(wsConn, msg.Nonce, "", "", deviceID, "pending", "awaiting approval in AuraGo UI", nil, nil)
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("read pending remote enrollment: %w", err)
	}
	var pendingCount int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM remote_devices WHERE status='pending'`).Scan(&pendingCount); err != nil {
		return fmt.Errorf("count pending remote enrollments: %w", err)
	}
	if pendingCount >= 100 {
		return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "pending enrollment limit reached", nil, nil)
	}
	deviceID, err = CreateDevice(h.db, DeviceRecord{
		Name:      deviceName,
		Hostname:  auth.Hostname,
		OS:        auth.OS,
		Arch:      auth.Arch,
		IPAddress: peerHost,
		Status:    "pending",
		ReadOnly:  h.DefaultReadOnly,
	})
	if err != nil {
		return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "internal error", nil, nil)
	}

	h.logger.Info("New device pending approval", "device_id", deviceID, "hostname", auth.Hostname, "ip", auth.IP)
	return h.sendAuthResponse(wsConn, msg.Nonce, "", "", deviceID, "pending", "awaiting approval in AuraGo UI", nil, nil)
}

// completeEnrollment generates shared key, creates device, and sends credentials.
// requestNonce is the Nonce of the auth frame being answered.
func (h *RemoteHub) completeEnrollment(wsConn *websocket.Conn, requestNonce string, auth AuthPayload, enrollmentID, deviceName, bootstrapSigningKey string) error {
	sharedKey, err := GenerateSharedKey()
	if err != nil {
		return h.sendAuthResponse(wsConn, requestNonce, bootstrapSigningKey, "", "", "rejected", "key generation failed", nil, nil)
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
		return h.sendAuthResponse(wsConn, requestNonce, bootstrapSigningKey, "", "", "rejected", "device registration failed", nil, nil)
	}
	cleanupRejectedEnrollment := func() {
		if err := h.vault.DeleteSecret("remote_shared_key_" + deviceID); err != nil {
			h.logger.Error("Failed to clean up rejected remote key", "device_id", deviceID, "error", err)
		}
		if err := DeleteDevice(h.db, deviceID); err != nil {
			h.logger.Error("Failed to clean up rejected remote device", "device_id", deviceID, "error", err)
		}
	}

	// Store shared key in vault
	if err := h.vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil {
		h.logger.Error("Failed to store shared key in vault", "device_id", deviceID, "error", err)
		cleanupRejectedEnrollment()
		return h.sendAuthResponse(wsConn, requestNonce, bootstrapSigningKey, "", "", "rejected", "credential storage failed", nil, nil)
	}

	if err := finalizeEnrollment(h.db, h.vault, enrollmentID, deviceID); err != nil {
		h.logger.Error("Failed to finalize remote enrollment", "device_id", deviceID, "error", err)
		cleanupRejectedEnrollment()
		return h.sendAuthResponse(wsConn, requestNonce, bootstrapSigningKey, "", "", "rejected", "device registration failed", nil, nil)
	}

	// Register connection. A new device has no list of its own yet, so the
	// agent receives the global default.
	allowedPaths := h.effectiveAllowedPaths(nil)
	readOnly := h.DefaultReadOnly
	conn := &RemoteConnection{
		Conn:          wsConn,
		DeviceID:      deviceID,
		Name:          name,
		SharedKey:     sharedKey,
		LastHeartbeat: time.Now(),
		Status:        "connected",
		ReadOnly:      readOnly,
		AllowedPaths:  []string{},
		Version:       auth.Version,
	}
	h.Register(deviceID, conn)

	return h.sendAuthResponse(wsConn, requestNonce, bootstrapSigningKey, sharedKey, deviceID, "enrolled", "", &readOnly, allowedPaths)
}

// ApproveDevice replaces a pending observation with a fresh, single-use token.
// Only the administrator receives this token; no key is delivered to an
// unauthenticated socket and previously consumed tokens remain invalid.
func (h *RemoteHub) ApproveDevice(deviceID string) (string, string, error) {
	if !h.Enabled() {
		return "", "", fmt.Errorf("remote control is disabled")
	}
	device, err := GetDevice(h.db, deviceID)
	if err != nil {
		return "", "", fmt.Errorf("device not found: %w", err)
	}
	if device.Status != "pending" {
		return "", "", fmt.Errorf("device is not pending approval")
	}
	token, err := GenerateSharedKey()
	if err != nil {
		return "", "", err
	}
	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	_, err = h.issueEnrollment(token, device.Name, expires, func(tx *sql.Tx) error {
		result, err := tx.Exec(`DELETE FROM remote_devices WHERE id = ? AND status = 'pending'`, deviceID)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return fmt.Errorf("pending enrollment changed")
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	security.RegisterSensitive(token)
	return token, expires, nil
}

// IssueEnrollmentToken registers a raw one-time enrollment token and returns
// its enrollment ID. remote_enrollments stores only the token's lookup hash;
// its MAC key goes to the vault. If the key cannot be stored, no token is
// created. expiresAt is RFC3339 in any offset; it is stored in UTC.
func (h *RemoteHub) IssueEnrollmentToken(token, deviceName, expiresAt string) (string, error) {
	return h.issueEnrollment(token, deviceName, expiresAt, nil)
}

// SweepExpiredEnrollments removes expired enrollments and every MAC key that
// no longer belongs to a live enrollment (see CleanExpiredEnrollments).
func (h *RemoteHub) SweepExpiredEnrollments() error {
	h.enrollmentKeyMu.Lock()
	defer h.enrollmentKeyMu.Unlock()
	return CleanExpiredEnrollments(h.db, h.vault)
}

// DiscardEnrollmentKey deletes the MAC key of an enrollment consumed outside
// HandleEnrollment (agodesk pairing). A failure is only logged; the consumed
// row already refuses the token and the next sweep removes the key.
func (h *RemoteHub) DiscardEnrollmentKey(enrollmentID string) {
	if h == nil || h.vault == nil || enrollmentID == "" {
		return
	}
	if err := h.vault.DeleteSecret(enrollmentAuthKeyName(enrollmentID)); err != nil {
		h.logger.Warn("Failed to delete consumed remote enrollment key", "enrollment_id", enrollmentID, "error", err)
	}
}

// issueEnrollment inserts the enrollment row, runs alsoInTx (may be nil) and
// writes the MAC key inside one DB transaction. A vault failure rolls the row
// back; a failed commit deletes the key again. Expired enrollments and their
// keys are swept first, so leftovers do not accumulate.
func (h *RemoteHub) issueEnrollment(token, deviceName, expiresAt string, alsoInTx func(*sql.Tx) error) (string, error) {
	if h.vault == nil {
		return "", fmt.Errorf("enrollment key storage unavailable")
	}
	// The sweep compares RFC3339 strings, which only works in one offset.
	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "", fmt.Errorf("invalid enrollment expiry: %w", err)
	}
	expiresAt = expiry.UTC().Format(time.RFC3339)
	h.enrollmentKeyMu.Lock()
	defer h.enrollmentKeyMu.Unlock()
	if err := CleanExpiredEnrollments(h.db, h.vault); err != nil {
		h.logger.Warn("Failed to sweep expired remote enrollments", "error", err)
	}

	id := uid.New()
	tx, err := h.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO remote_enrollments (id, token_hash, device_name, created_at, expires_at, used, used_by_device) VALUES (?, ?, ?, ?, ?, 0, '')`,
		id, DeriveEnrollmentLookupHash(token), deviceName, time.Now().UTC().Format(time.RFC3339), expiresAt); err != nil {
		return "", fmt.Errorf("create enrollment: %w", err)
	}
	if alsoInTx != nil {
		if err := alsoInTx(tx); err != nil {
			return "", err
		}
	}
	keyName := enrollmentAuthKeyName(id)
	if err := h.vault.WriteSecret(keyName, DeriveEnrollmentAuthKey(token)); err != nil {
		return "", fmt.Errorf("store enrollment key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		if delErr := h.vault.DeleteSecret(keyName); delErr != nil {
			h.logger.Error("Failed to delete key of uncommitted remote enrollment", "enrollment_id", id, "error", delErr)
		}
		return "", fmt.Errorf("commit enrollment: %w", err)
	}
	return id, nil
}

// Refusal reasons around the lookup-hash/MAC-key split.
const (
	// A current-version enrollment frame without kdf 2.
	preUpgradeEnrollmentMessage = "enrollment token predates the upgrade; create a new one"
	// An unversioned enrollment frame: the agent binary is outdated whatever
	// the token's age, and downloading the agent again also issues a token.
	// ASCII only: old agents print it raw, and legacy code pages garble the rest.
	preUpgradeAgentMessage = "agent predates the supervisor upgrade; download the agent again from AuraGo (Remote Control), which also issues a new token"
	// No row for the lookup hash. Rows issued before the key split are stored
	// under the plain token hash and end up here as well.
	unknownEnrollmentMessage = "invalid or pre-upgrade enrollment token; create a new one"
	// A row without a vault MAC key.
	keylessEnrollmentMessage = "enrollment token has no key on this supervisor; create a new one"
	// An enrolled device whose agent binary predates the canonical HMAC form.
	preUpgradeReconnectMessage = "unsupported frame version; replace the agent binary with a download from the upgraded supervisor"
)

// rejectPreUpgradeEnrollment refuses an enrollment frame from an agent that
// predates the key split (no kdf 2) or the canonical HMAC form (no version).
// Nothing in the frame is verified. A pre-upgrade agent sends an unversioned
// frame and checks the answer in the old form with the plain token hash it
// sent, so it gets a refusal built that way (legacyRefusal) and can show the
// reason. A current-version frame without kdf 2 is refused unsigned.
func (h *RemoteHub) rejectPreUpgradeEnrollment(wsConn *websocket.Conn, msg RemoteMessage, auth AuthPayload) error {
	h.logger.Warn("Refusing enrollment from an agent or token that predates the upgrade; download the agent again from Remote Control",
		"frame_version", msg.Version, "kdf", auth.KDF, "hostname", auth.Hostname)
	if msg.Version < FrameVersion {
		if refusal, err := legacyRefusal(msg.Nonce, auth.TokenHash, preUpgradeAgentMessage); err == nil {
			return wsConn.WriteJSON(refusal)
		}
		return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", preUpgradeAgentMessage, nil, nil)
	}
	return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", preUpgradeEnrollmentMessage, nil, nil)
}

// rejectPreUpgradeReconnect refuses an unversioned reconnect frame. When it
// names a known, non-revoked device whose key is in the vault, the fixed
// refusal is built in the old form and signed with that device key, which the
// old agent verifies with, so it can show what to do. The frame itself is
// never verified and nothing is granted or changed. Otherwise the refusal is
// unsigned.
func (h *RemoteHub) rejectPreUpgradeReconnect(wsConn *websocket.Conn, msg RemoteMessage, deviceID string) error {
	if device, err := GetDevice(h.db, deviceID); err == nil && device.Status != "revoked" && h.vault != nil {
		if key, err := h.vault.ReadSecret("remote_shared_key_" + device.ID); err == nil {
			if refusal, err := legacyRefusal(msg.Nonce, key, preUpgradeReconnectMessage); err == nil {
				return wsConn.WriteJSON(refusal)
			}
		}
	}
	return h.sendAuthResponse(wsConn, msg.Nonce, "", "", "", "rejected", "unsupported frame version", nil, nil)
}

// legacyRefusal builds the only frames still signed in the undelimited
// pre-version-2 form (see hmacData): fixed refusals for pre-upgrade agents,
// signed with the key that agent verifies with (the plain token hash it sent
// when enrolling, or its device key when reconnecting) so it can show the
// reason. They grant nothing, and current agents reject unversioned frames
// outright. The payload holds only status, message and request nonce: older
// agents apply bootstrap settings from an auth response before they look at
// its status. The request nonce is echoed only if it is well formed, so the
// signed bytes stay fixed apart from server-chosen values. It fails if keyHex
// is not a usable key.
func legacyRefusal(requestNonce, keyHex, message string) (*RemoteMessage, error) {
	key, err := decodeSharedKey(keyHex)
	if err != nil {
		return nil, err
	}
	if !ValidNonce(requestNonce) {
		requestNonce = ""
	}
	payload, err := json.Marshal(struct {
		Status       string `json:"status"`
		Message      string `json:"message"`
		RequestNonce string `json:"request_nonce,omitempty"`
	}{Status: "rejected", Message: message, RequestNonce: requestNonce})
	if err != nil {
		return nil, err
	}
	nonce, err := GenerateNonce()
	if err != nil {
		return nil, err
	}
	msg := &RemoteMessage{
		Type:      MsgAuthResponse,
		MessageID: fmt.Sprintf("%d", time.Now().UnixNano()),
		Nonce:     nonce,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(legacyHMACData(msg))
	msg.HMAC = hex.EncodeToString(mac.Sum(nil))
	return msg, nil
}

// legacyHMACData is the undelimited pre-version-2 HMAC input. It is ambiguous
// across field boundaries and is used only by legacyRefusal.
func legacyHMACData(msg *RemoteMessage) []byte {
	return []byte(msg.Type + msg.DeviceID + msg.MessageID +
		strconv.FormatUint(msg.Sequence, 10) + msg.Nonce +
		msg.Timestamp + string(msg.Payload))
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

// sendAuthResponse answers the auth frame whose Nonce is requestNonce. The
// answer echoes that nonce under its HMAC, so the agent accepts a signed
// answer only for the request it just sent and a captured one cannot be
// replayed on a later connect. A malformed request nonce is not echoed, so a
// requester cannot place arbitrary bytes under a signature.
func (h *RemoteHub) sendAuthResponse(wsConn *websocket.Conn, requestNonce, signingKeyHex, sharedKey, deviceID, status, message string, readOnly *bool, allowedPaths []string) error {
	if !ValidNonce(requestNonce) {
		requestNonce = ""
	}
	resp := AuthResponsePayload{
		Status:        status,
		DeviceID:      deviceID,
		SharedKey:     sharedKey,
		Message:       message,
		ReadOnly:      readOnly,
		AllowedPaths:  allowedPaths,
		MaxFileSizeMB: h.effectiveMaxFileSizeMB(),
		RequestNonce:  requestNonce,
	}
	msg, err := NewAuthResponseMessage(deviceID, signingKeyHex, resp)
	if err != nil {
		return err
	}
	return wsConn.WriteJSON(msg)
}

// hashTokenSHA256 is the plain SHA-256 hex stored as a device's
// shared_key_hash. Enrollment tokens use the separate derivations in
// protocol.go instead.
func hashTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (h *RemoteHub) effectiveMaxFileSizeMB() int {
	if h.MaxFileSizeMB <= 0 {
		return DefaultMaxFileSizeMB
	}
	return h.MaxFileSizeMB
}

func isTrustedAutoApproveRemoteAddr(addr net.Addr) bool {
	// Kept for callers migrating from the legacy heuristic. An address alone
	// can never authenticate a device, including loopback behind a proxy.
	return false
}

func (h *RemoteHub) Enabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return !h.disabled
}

// SetEnabled serializes lifecycle transitions and drains the heartbeat monitor.
func (h *RemoteHub) SetEnabled(enabled bool) {
	h.monitorMu.Lock()
	defer h.monitorMu.Unlock()
	h.mu.Lock()
	changed := h.disabled == enabled
	h.disabled = !enabled
	if changed {
		if !enabled && h.disableCh != nil {
			close(h.disableCh)
		}
		if enabled {
			h.disableCh = make(chan struct{})
		}
	}
	var conns []*RemoteConnection
	if !enabled {
		for _, conn := range h.connections {
			conns = append(conns, conn)
		}
	}
	h.mu.Unlock()
	if !enabled {
		if h.monitorCancel != nil {
			h.monitorCancel()
			<-h.monitorDone
			h.monitorCancel = nil
		}
		for _, conn := range conns {
			h.unregisterConnection(conn.DeviceID, conn)
		}
	}
}

// ── Heartbeat monitor ───────────────────────────────────────────────────────

// StartHeartbeatMonitor periodically checks all connections for stale heartbeats.
func (h *RemoteHub) StartHeartbeatMonitor(interval, maxAge time.Duration) {
	h.monitorMu.Lock()
	defer h.monitorMu.Unlock()
	if !h.Enabled() || h.monitorCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.monitorCancel = cancel
	done := make(chan struct{})
	h.monitorDone = done
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			h.mu.RLock()
			var stale []*RemoteConnection
			for _, conn := range h.connections {
				conn.mu.Lock()
				lastHeartbeat := conn.LastHeartbeat
				conn.mu.Unlock()
				if !lastHeartbeat.IsZero() && time.Since(lastHeartbeat) > maxAge {
					stale = append(stale, conn)
				}
			}
			h.mu.RUnlock()

			for _, conn := range stale {
				h.logger.Warn("Remote heartbeat stale, disconnecting", "device_id", conn.DeviceID)
				h.unregisterConnection(conn.DeviceID, conn)
			}
		}
	}()
}
