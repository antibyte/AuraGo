package bridge

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const MaxEggWebSocketMessageBytes int64 = 10 << 20

// EggConnection represents a single connected egg worker.
type EggConnection struct {
	Conn          *websocket.Conn
	EggID         string
	NestID        string
	SharedKey     string // hex-encoded (current key)
	PreviousKey   string // hex-encoded (previous key — accepted only while a rotation awaits the egg's ack)
	PreviousKeyAt time.Time
	LastHeartbeat time.Time
	Status        string // "connected" | "idle" | "busy" | "error"
	Telemetry     HeartbeatPayload
	KeyVersion    int
	mu            sync.Mutex
	Session       *Session
	closed        atomic.Bool

	// rekeyInFlight is set while SendRekey awaits the egg's answer.
	// rekeyOutstanding holds the ID of a sent rekey whose outcome the hub has
	// not seen; after a timeout the egg may still adopt that key, so no new
	// rotation may start until its rejection arrives or the socket ends.
	rekeyInFlight    bool
	rekeyOutstanding string
}

// rekeyGraceWindow caps how long the previous key verifies egg frames.
const rekeyGraceWindow = time.Minute

// Send writes a signed message to the egg.
func (ec *EggConnection) Send(msg *Message) error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.sendLocked(msg)
}

func (ec *EggConnection) sendLocked(msg *Message) error {
	if ec.Conn == nil || ec.closed.Load() {
		return fmt.Errorf("egg connection is closed")
	}
	if err := ec.Session.Prepare(msg, ec.SharedKey); err != nil {
		return err
	}
	_ = ec.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ec.Conn.WriteJSON(msg)
}

func (ec *EggConnection) newMessage(kind string, payload interface{}) (*Message, error) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return NewMessage(kind, ec.EggID, ec.NestID, ec.SharedKey, payload)
}

func (ec *EggConnection) close() error {
	ec.closed.Store(true)
	if ec.Conn == nil {
		return nil
	}
	return ec.Conn.Close()
}

// GetTelemetry safely retrieves the latest heartbeat data.
func (ec *EggConnection) GetTelemetry() HeartbeatPayload {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.Telemetry
}

// EggHub manages all connected egg workers on the master side.
type EggHub struct {
	mu             sync.RWMutex
	lifecycleMu    sync.Mutex
	connections    map[string]*EggConnection // keyed by nest_id
	logger         *slog.Logger
	MaxConnections int // 0 = unlimited
	pendingAcks    map[string]pendingAck
	ackTimeout     time.Duration
	rotatingNests  map[string]struct{}

	// Callbacks (set by the server layer)
	OnConnect       func(nestID, eggID string)
	OnDisconnect    func(nestID, eggID string)
	OnHeartbeat     func(nestID string, hb HeartbeatPayload)
	OnResult        func(nestID string, result ResultPayload)
	OnMissionResult func(nestID string, result MissionResultPayload)
}

type pendingAck struct {
	conn *EggConnection
	ch   chan AckPayload
}

// NewEggHub creates a new hub for managing egg connections.
func NewEggHub(logger *slog.Logger) *EggHub {
	return &EggHub{
		connections:   make(map[string]*EggConnection),
		pendingAcks:   make(map[string]pendingAck),
		ackTimeout:    15 * time.Second,
		rotatingNests: make(map[string]struct{}),
		logger:        logger,
	}
}

// Register adds an authenticated egg connection to the hub.
func (h *EggHub) Register(nestID string, conn *EggConnection) error {
	if conn == nil || conn.Session == nil || conn.Session.Role != "master" || conn.Session.NestID != nestID || conn.Session.EggID != conn.EggID || conn.NestID != nestID {
		return fmt.Errorf("authenticated session required")
	}
	h.lifecycleMu.Lock()
	defer h.lifecycleMu.Unlock()
	h.mu.Lock()
	old := h.connections[nestID]
	if old == nil && h.MaxConnections > 0 && len(h.connections) >= h.MaxConnections {
		h.mu.Unlock()
		return fmt.Errorf("max connections reached (%d)", h.MaxConnections)
	}
	conn.mu.Lock()
	if conn.LastHeartbeat.IsZero() {
		conn.LastHeartbeat = time.Now()
	}
	conn.mu.Unlock()
	h.connections[nestID] = conn
	h.mu.Unlock()
	if old != nil && old != conn {
		_ = old.close()
	}
	h.logger.Info("Egg connected", "nest_id", nestID, "egg_id", conn.EggID)
	if h.OnConnect != nil {
		h.OnConnect(nestID, conn.EggID)
	}
	return nil
}

// Unregister explicitly revokes the current connection for an administrative action.
func (h *EggHub) Unregister(nestID string) {
	h.unregister(nestID, nil, true, 0, nil)
}

// Connection-owned cleanup can remove only that exact generation.
func (h *EggHub) unregister(nestID string, expected *EggConnection, notify bool, maxAge time.Duration, onStale func(string, string)) {
	h.lifecycleMu.Lock()
	defer h.lifecycleMu.Unlock()
	h.mu.Lock()
	conn := h.connections[nestID]
	if conn == nil || (expected != nil && conn != expected) {
		h.mu.Unlock()
		return
	}
	if maxAge > 0 {
		conn.mu.Lock()
		stale := !conn.LastHeartbeat.IsZero() && time.Since(conn.LastHeartbeat) > maxAge
		conn.mu.Unlock()
		if !stale {
			h.mu.Unlock()
			return
		}
	}
	delete(h.connections, nestID)
	h.mu.Unlock()
	_ = conn.close()
	if notify && h.OnDisconnect != nil {
		h.OnDisconnect(nestID, conn.EggID)
	}
	if onStale != nil {
		onStale(nestID, conn.EggID)
	}
}

// GetConnection returns the connection for a nest, or nil.
func (h *EggHub) GetConnection(nestID string) *EggConnection {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connections[nestID]
}

// IsConnected checks if a nest has an active egg connection.
func (h *EggHub) IsConnected(nestID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.connections[nestID]
	return ok
}

// ConnectedNests returns a list of all connected nest IDs.
func (h *EggHub) ConnectedNests() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.connections))
	for id := range h.connections {
		ids = append(ids, id)
	}
	return ids
}

// ConnectionCount returns the number of connected eggs.
func (h *EggHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// SendTask sends a task to a specific egg via its nest connection.
func (h *EggHub) SendTask(nestID string, task TaskPayload) error {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return fmt.Errorf("no active connection for nest %s", nestID)
	}
	msg, err := conn.newMessage(MsgTask, task)
	if err != nil {
		return fmt.Errorf("failed to create task message: %w", err)
	}
	return conn.Send(msg)
}

// SendMissionSync sends a mission definition to a specific egg and waits for acknowledgement.
func (h *EggHub) SendMissionSync(nestID string, payload MissionSyncPayload) error {
	return h.SendMissionSyncContext(context.Background(), nestID, payload)
}

// SendMissionSyncContext sends a mission definition to a specific egg and waits for acknowledgement or context cancellation.
func (h *EggHub) SendMissionSyncContext(ctx context.Context, nestID string, payload MissionSyncPayload) error {
	return h.sendMissionMessageContext(ctx, nestID, MsgMissionSync, payload)
}

// SendMissionRun asks a specific egg to run a synced mission and waits for acknowledgement.
func (h *EggHub) SendMissionRun(nestID string, payload MissionRunPayload) error {
	return h.SendMissionRunContext(context.Background(), nestID, payload)
}

// SendMissionRunContext asks a specific egg to run a synced mission and waits for acknowledgement or context cancellation.
func (h *EggHub) SendMissionRunContext(ctx context.Context, nestID string, payload MissionRunPayload) error {
	return h.sendMissionMessageContext(ctx, nestID, MsgMissionRun, payload)
}

// SendMissionDelete asks a specific egg to delete a synced mission and waits for acknowledgement.
func (h *EggHub) SendMissionDelete(nestID string, payload MissionDeletePayload) error {
	return h.SendMissionDeleteContext(context.Background(), nestID, payload)
}

// SendMissionDeleteContext asks a specific egg to delete a synced mission and waits for acknowledgement or context cancellation.
func (h *EggHub) SendMissionDeleteContext(ctx context.Context, nestID string, payload MissionDeletePayload) error {
	return h.sendMissionMessageContext(ctx, nestID, MsgMissionDelete, payload)
}

func (h *EggHub) sendMissionMessageContext(ctx context.Context, nestID, msgType string, payload interface{}) error {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return fmt.Errorf("no active connection for nest %s", nestID)
	}
	msg, err := conn.newMessage(msgType, payload)
	if err != nil {
		return fmt.Errorf("failed to create %s message: %w", msgType, err)
	}
	return h.sendWithAckContext(ctx, conn, msg)
}

func (h *EggHub) sendWithAck(conn *EggConnection, msg *Message) error {
	return h.sendWithAckContext(context.Background(), conn, msg)
}

func (h *EggHub) sendWithAckContext(ctx context.Context, conn *EggConnection, msg *Message) error {
	ackCh := h.registerPendingAck(msg.ID, conn)
	defer h.clearPendingAck(msg.ID)

	if err := conn.Send(msg); err != nil {
		return err
	}
	_, err := h.awaitAck(ctx, ackCh, conn.NestID)
	return err
}

// registerPendingAck routes the ack for msgID from conn to the returned channel.
// It takes only the hub lock; callers must not hold conn.mu (lock order is
// hub before connection, as in Register and unregister).
func (h *EggHub) registerPendingAck(msgID string, conn *EggConnection) chan AckPayload {
	ackCh := make(chan AckPayload, 1)
	h.mu.Lock()
	h.pendingAcks[msgID] = pendingAck{conn: conn, ch: ackCh}
	h.mu.Unlock()
	return ackCh
}

func (h *EggHub) clearPendingAck(msgID string) {
	h.mu.Lock()
	delete(h.pendingAcks, msgID)
	h.mu.Unlock()
}

// ackRejectedError is an explicit negative ack: the egg answered and refused.
type ackRejectedError struct{ detail string }

func (e *ackRejectedError) Error() string { return e.detail }

func ackResult(ack AckPayload) error {
	if ack.Success {
		return nil
	}
	if ack.Detail == "" {
		ack.Detail = "operation rejected by egg"
	}
	return &ackRejectedError{detail: ack.Detail}
}

// awaitAck returns the egg's ack (zero when none arrived) and its outcome.
func (h *EggHub) awaitAck(ctx context.Context, ackCh <-chan AckPayload, nestID string) (AckPayload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	h.mu.RLock()
	timeout := h.ackTimeout
	h.mu.RUnlock()
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	select {
	case ack := <-ackCh:
		return ack, ackResult(ack)
	case <-time.After(timeout):
		return AckPayload{}, fmt.Errorf("timed out waiting for ack from nest %s", nestID)
	case <-ctx.Done():
		return AckPayload{}, ctx.Err()
	}
}

func (h *EggHub) resolveAck(conn *EggConnection, ack AckPayload) {
	h.mu.RLock()
	pending := h.pendingAcks[ack.RefID]
	ch := pending.ch
	h.mu.RUnlock()
	if ch == nil || pending.conn != conn {
		return
	}
	select {
	case ch <- ack:
	default:
	}
}

func (h *EggHub) sendAck(conn *EggConnection, refID string, success bool, detail string) error {
	ack, err := conn.newMessage(MsgAck, AckPayload{
		RefID:   refID,
		Success: success,
		Detail:  detail,
	})
	if err != nil {
		return err
	}
	return conn.Send(ack)
}

// SendSecret sends an encrypted secret to a specific egg.
func (h *EggHub) SendSecret(nestID, key, encryptedValue string) error {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return fmt.Errorf("no active connection for nest %s", nestID)
	}
	payload := SecretPayload{Key: key, EncryptedValue: encryptedValue}
	msg, err := conn.newMessage(MsgSecret, payload)
	if err != nil {
		return fmt.Errorf("failed to create secret message: %w", err)
	}
	return conn.Send(msg)
}

// SendSafeReconfigure sends a safe config patch to an egg for in-place reconfiguration.
// The egg applies the patch and restarts. Returns an error if the nest is not connected.
func (h *EggHub) SendSafeReconfigure(nestID string, payload ReconfigurePayload) error {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return fmt.Errorf("no active connection for nest %s", nestID)
	}
	msg, err := conn.newMessage(MsgSafeReconfigure, payload)
	if err != nil {
		return fmt.Errorf("failed to create safe_reconfigure message: %w", err)
	}
	return conn.Send(msg)
}

// SendStop sends a graceful shutdown command to an egg.
func (h *EggHub) SendStop(nestID string) error {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return fmt.Errorf("no active connection for nest %s", nestID)
	}
	msg, err := conn.newMessage(MsgStop, nil)
	if err != nil {
		return fmt.Errorf("failed to create stop message: %w", err)
	}
	if err := conn.Send(msg); err != nil {
		return err
	}
	h.unregister(nestID, conn, true, 0, nil)
	return nil
}

// BeginKeyRotation reserves the nest for one rotation, so staging a candidate
// key, SendRekey and the vault commit cannot interleave with another rotation.
// It refuses while the connected egg still has an unresolved rotation: that egg
// may yet adopt the earlier key, and staging a new one would orphan it.
func (h *EggHub) BeginKeyRotation(nestID string) (func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rotatingNests == nil {
		h.rotatingNests = make(map[string]struct{})
	}
	if _, busy := h.rotatingNests[nestID]; busy {
		return nil, fmt.Errorf("a key rotation for nest %s is already in progress", nestID)
	}
	if conn := h.connections[nestID]; conn != nil && conn.rekeyUnresolved() {
		return nil, fmt.Errorf("an earlier key rotation for nest %s is still unconfirmed", nestID)
	}
	h.rotatingNests[nestID] = struct{}{}
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.rotatingNests, nestID)
			h.mu.Unlock()
		})
	}, nil
}

// rekeyUnresolved reports whether the nest's connection awaits the outcome of
// a rotation (in flight, or timed out without the egg's answer).
func (h *EggHub) rekeyUnresolved(nestID string) bool {
	conn := h.GetConnection(nestID)
	return conn != nil && conn.rekeyUnresolved()
}

// rekeyUnresolved takes conn.mu; callers may hold the hub lock (hub before
// connection) but not conn.mu.
func (ec *EggConnection) rekeyUnresolved() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.rekeyPendingLocked()
}

func (ec *EggConnection) rekeyPendingLocked() bool {
	return ec.rekeyInFlight || ec.rekeyOutstanding != ""
}

// SendRekey rotates the shared key in two phases: the frame leaves under the
// current key, both keys stay valid while the egg persists, and the hub
// commits only after the egg's signed ack. On rejection or timeout the hub
// rolls back so a reconnecting egg still matches.
//
// The egg acks a successful rotation under the new key and a rejection under
// the old one; both verify while the previous key is held. Frames the egg
// signed with the old key precede its ack on the ordered socket, so the
// previous key is dropped on commit. After a timeout the outcome is unknown:
// the rotation stays outstanding (blocking further rotations) until the egg's
// rejection arrives or the socket ends; an egg that adopted the key fails
// verification under the rolled-back key, reconnects, and authenticates with
// the master's staged candidate.
//
// persisted reports the ack's Persisted flag: true means the egg stored the
// key durably, so the caller may retire the old key at commit; false on a
// successful rotation means a legacy egg that may hold the key only in memory.
func (h *EggHub) SendRekey(ctx context.Context, nestID, newKeyHex string) (persisted bool, err error) {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return false, fmt.Errorf("no active connection for nest %s", nestID)
	}
	key, err := hex.DecodeString(newKeyHex)
	if err != nil || len(key) != 32 {
		return false, fmt.Errorf("invalid new shared key")
	}

	conn.mu.Lock()
	if conn.rekeyPendingLocked() {
		conn.mu.Unlock()
		return false, fmt.Errorf("an earlier key rotation for nest %s is still unconfirmed", nestID)
	}
	oldKey, oldVersion := conn.SharedKey, conn.KeyVersion
	conn.rekeyInFlight = true
	conn.mu.Unlock()
	finish := func() {
		conn.mu.Lock()
		conn.rekeyInFlight = false
		conn.mu.Unlock()
	}

	version := oldVersion + 1
	encrypted, err := EncryptWithSharedKey([]byte(newKeyHex), oldKey)
	if err != nil {
		finish()
		return false, fmt.Errorf("encrypt new shared key: %w", err)
	}
	msg, err := NewMessage(MsgRekey, conn.EggID, nestID, oldKey, RekeyPayload{NewKeyEncrypted: encrypted, KeyVersion: version})
	if err != nil {
		finish()
		return false, err
	}
	// Register before taking conn.mu: the hub lock is always taken first.
	ackCh := h.registerPendingAck(msg.ID, conn)
	defer h.clearPendingAck(msg.ID)

	conn.mu.Lock()
	if err := conn.sendLocked(msg); err != nil {
		conn.rekeyInFlight = false
		conn.mu.Unlock()
		return false, err
	}
	// Frames sent after the rekey use the new key; the egg reads them only
	// after it has processed (and persisted) the rotation.
	conn.rekeyOutstanding = msg.ID
	conn.PreviousKey, conn.PreviousKeyAt = oldKey, time.Now()
	conn.SharedKey, conn.KeyVersion = newKeyHex, version
	conn.mu.Unlock()

	ack, err := h.awaitAck(ctx, ackCh, nestID)
	var rejected *ackRejectedError
	if err != nil && !errors.As(err, &rejected) {
		// The answer may have arrived just as the wait gave up.
		select {
		case ack = <-ackCh:
			err = ackResult(ack)
		default:
		}
	}

	conn.mu.Lock()
	conn.rekeyInFlight = false
	conn.PreviousKey, conn.PreviousKeyAt = "", time.Time{}
	if err == nil {
		if conn.rekeyOutstanding == msg.ID {
			conn.rekeyOutstanding = ""
		}
		conn.mu.Unlock()
		h.logger.Info("Key rotated for egg", "nest_id", nestID, "version", version, "persisted", ack.Persisted)
		return ack.Persisted, nil
	}
	conn.SharedKey, conn.KeyVersion = oldKey, oldVersion
	if errors.As(err, &rejected) && conn.rekeyOutstanding == msg.ID {
		conn.rekeyOutstanding = ""
	}
	conn.mu.Unlock()
	h.logger.Warn("Key rotation rolled back", "nest_id", nestID, "error", err)
	return ack.Persisted, fmt.Errorf("egg did not confirm key rotation: %w", err)
}

// HandleMessages reads messages from an egg connection and dispatches them.
// Blocks until the connection closes or an error occurs.
func (h *EggHub) HandleMessages(conn *EggConnection) {
	defer h.unregister(conn.NestID, conn, true, 0, nil)
	if conn.closed.Load() || conn.Conn == nil {
		return
	}
	conn.Conn.SetReadLimit(MaxEggWebSocketMessageBytes)

	// Rate limit: max 100 messages per second per connection
	const rateLimit = 100
	const rateBurst = 150
	tokens := float64(rateBurst)
	lastRefill := time.Now()

	for {
		_, data, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Warn("Egg connection error", "nest_id", conn.NestID, "error", err)
			}
			return
		}

		// Token bucket rate limiting (float64 accumulator avoids truncation
		// that would lose sub-second token refills)
		now := time.Now()
		elapsed := now.Sub(lastRefill)
		tokens += elapsed.Seconds() * float64(rateLimit)
		if tokens > float64(rateBurst) {
			tokens = float64(rateBurst)
		}
		lastRefill = now
		if tokens < 1.0 {
			h.logger.Warn("Rate limit exceeded for egg", "nest_id", conn.NestID)
			continue
		}
		tokens--

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			h.logger.Warn("Invalid message from egg", "nest_id", conn.NestID, "error", err)
			continue
		}

		conn.mu.Lock()
		previous := ""
		if time.Since(conn.PreviousKeyAt) < rekeyGraceWindow {
			previous = conn.PreviousKey
		}
		err = conn.Session.Accept(msg, conn.SharedKey, previous)
		conn.mu.Unlock()
		if err != nil {
			h.logger.Warn("Rejected egg message", "nest_id", conn.NestID, "error", err)
			return
		}
		if h.GetConnection(conn.NestID) != conn {
			return
		}

		switch msg.Type {
		case MsgHeartbeat:
			var hb HeartbeatPayload
			if err := json.Unmarshal(msg.Payload, &hb); err == nil {
				conn.mu.Lock()
				conn.LastHeartbeat = time.Now()
				conn.Status = hb.Status
				conn.Telemetry = hb
				conn.mu.Unlock()
				if h.OnHeartbeat != nil {
					h.OnHeartbeat(conn.NestID, hb)
				}
			}
		case MsgResult:
			var result ResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err == nil {
				if h.OnResult != nil {
					h.OnResult(conn.NestID, result)
				}
			}
		case MsgMissionResult:
			var result MissionResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err == nil {
				_ = h.sendAck(conn, msg.ID, true, "mission result received")
				if h.OnMissionResult != nil {
					h.OnMissionResult(conn.NestID, result)
				}
			}
		case MsgAck:
			var ack AckPayload
			if err := json.Unmarshal(msg.Payload, &ack); err == nil {
				if !ack.Success && ack.RefID != "" {
					// A rejected rekey proves the egg kept the key the hub
					// rolled back to, even when it answers after a timeout.
					conn.mu.Lock()
					if conn.rekeyOutstanding == ack.RefID {
						conn.rekeyOutstanding = ""
					}
					conn.mu.Unlock()
				}
				h.resolveAck(conn, ack)
			}
			h.logger.Debug("Ack received from egg", "nest_id", conn.NestID, "msg_id", msg.ID)
		case MsgStatus:
			// Status messages from eggs are logged at debug level
			h.logger.Debug("Status update from egg", "nest_id", conn.NestID)
		case MsgError:
			var errPayload ErrorPayload
			if err := json.Unmarshal(msg.Payload, &errPayload); err == nil {
				h.logger.Warn("Error from egg", "nest_id", conn.NestID, "code", errPayload.Code, "msg", errPayload.Message)
			}
		default:
			h.logger.Warn("Unknown message type from egg", "nest_id", conn.NestID, "type", msg.Type)
		}
	}
}

// StartHeartbeatMonitor periodically checks all connections for stale heartbeats.
// Calls onStale for each nest whose last heartbeat exceeds maxAge.
// Stops when ctx is cancelled.
func (h *EggHub) StartHeartbeatMonitor(ctx context.Context, interval, maxAge time.Duration, onStale func(nestID, eggID string)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			h.mu.RLock()
			candidates := make([]*EggConnection, 0, len(h.connections))
			for _, conn := range h.connections {
				candidates = append(candidates, conn)
			}
			h.mu.RUnlock()
			for _, conn := range candidates {
				h.unregister(conn.NestID, conn, false, maxAge, onStale)
			}

		}
	}()
}
