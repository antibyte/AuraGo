package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// EggConnection represents a single connected egg worker.
type EggConnection struct {
	Conn          *websocket.Conn
	EggID         string
	NestID        string
	SharedKey     string // hex-encoded (current key)
	PreviousKey   string // hex-encoded (previous key — valid for grace period after rotation)
	PreviousKeyAt time.Time
	LastHeartbeat time.Time
	Status        string // "connected" | "idle" | "busy" | "error"
	Telemetry     HeartbeatPayload
	KeyVersion    int
	mu            sync.Mutex
}

// Send writes a signed message to the egg.
func (ec *EggConnection) Send(msg *Message) error {
	ec.mu.Lock()
	conn := ec.Conn
	ec.mu.Unlock()
	return conn.WriteJSON(msg)
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
	connections    map[string]*EggConnection // keyed by nest_id
	logger         *slog.Logger
	MaxConnections int // 0 = unlimited
	pendingAcks    map[string]chan AckPayload
	ackTimeout     time.Duration

	// Callbacks (set by the server layer)
	OnConnect       func(nestID, eggID string)
	OnDisconnect    func(nestID, eggID string)
	OnHeartbeat     func(nestID string, hb HeartbeatPayload)
	OnResult        func(nestID string, result ResultPayload)
	OnMissionResult func(nestID string, result MissionResultPayload)
}

// NewEggHub creates a new hub for managing egg connections.
func NewEggHub(logger *slog.Logger) *EggHub {
	return &EggHub{
		connections: make(map[string]*EggConnection),
		pendingAcks: make(map[string]chan AckPayload),
		ackTimeout:  15 * time.Second,
		logger:      logger,
	}
}

// Register adds an authenticated egg connection to the hub.
func (h *EggHub) Register(nestID string, conn *EggConnection) error {
	h.mu.Lock()
	// Close existing connection for this nest if any
	if old, ok := h.connections[nestID]; ok {
		h.logger.Warn("Replacing existing egg connection", "nest_id", nestID)
		_ = old.Conn.Close()
	} else if h.MaxConnections > 0 && len(h.connections) >= h.MaxConnections {
		h.mu.Unlock()
		return fmt.Errorf("max connections reached (%d)", h.MaxConnections)
	}
	h.connections[nestID] = conn
	h.mu.Unlock()

	h.logger.Info("Egg connected", "nest_id", nestID, "egg_id", conn.EggID)
	if h.OnConnect != nil {
		h.OnConnect(nestID, conn.EggID)
	}
	return nil
}

// Unregister removes an egg connection from the hub.
func (h *EggHub) Unregister(nestID string) {
	h.mu.Lock()
	conn, ok := h.connections[nestID]
	if ok {
		delete(h.connections, nestID)
	}
	h.mu.Unlock()

	if ok {
		_ = conn.Conn.Close()
		h.logger.Info("Egg disconnected", "nest_id", nestID, "egg_id", conn.EggID)
		if h.OnDisconnect != nil {
			h.OnDisconnect(nestID, conn.EggID)
		}
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
	msg, err := NewMessage(MsgTask, conn.EggID, nestID, conn.SharedKey, task)
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
	msg, err := NewMessage(msgType, conn.EggID, nestID, conn.SharedKey, payload)
	if err != nil {
		return fmt.Errorf("failed to create %s message: %w", msgType, err)
	}
	return h.sendWithAckContext(ctx, conn, msg)
}

func (h *EggHub) sendWithAck(conn *EggConnection, msg *Message) error {
	return h.sendWithAckContext(context.Background(), conn, msg)
}

func (h *EggHub) sendWithAckContext(ctx context.Context, conn *EggConnection, msg *Message) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ackCh := make(chan AckPayload, 1)
	h.mu.Lock()
	h.pendingAcks[msg.ID] = ackCh
	timeout := h.ackTimeout
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pendingAcks, msg.ID)
		h.mu.Unlock()
	}()

	if err := conn.Send(msg); err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	select {
	case ack := <-ackCh:
		if !ack.Success {
			if ack.Detail == "" {
				ack.Detail = "operation rejected by egg"
			}
			return fmt.Errorf("%s", ack.Detail)
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for ack from nest %s", conn.NestID)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *EggHub) resolveAck(ack AckPayload) {
	h.mu.RLock()
	ch := h.pendingAcks[ack.RefID]
	h.mu.RUnlock()
	if ch == nil {
		return
	}
	select {
	case ch <- ack:
	default:
	}
}

func (h *EggHub) sendAck(conn *EggConnection, refID string, success bool, detail string) error {
	ack, err := NewMessage(MsgAck, conn.EggID, conn.NestID, conn.SharedKey, AckPayload{
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
	msg, err := NewMessage(MsgSecret, conn.EggID, nestID, conn.SharedKey, payload)
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
	msg, err := NewMessage(MsgSafeReconfigure, conn.EggID, nestID, conn.SharedKey, payload)
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
	msg, err := NewMessage(MsgStop, conn.EggID, nestID, conn.SharedKey, nil)
	if err != nil {
		return fmt.Errorf("failed to create stop message: %w", err)
	}
	if err := conn.Send(msg); err != nil {
		return err
	}
	h.Unregister(nestID)
	return nil
}

// SendRekey encrypts a new shared key with the current key and sends it to the egg.
// The hub updates the connection's key after sending; the previous key remains valid
// for a grace period (60s) to handle in-flight messages.
func (h *EggHub) SendRekey(nestID, newKeyHex string) error {
	conn := h.GetConnection(nestID)
	if conn == nil {
		return fmt.Errorf("no active connection for nest %s", nestID)
	}

	// Encrypt the new key with the current shared key
	encrypted, err := EncryptWithSharedKey([]byte(newKeyHex), conn.SharedKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt new key: %w", err)
	}

	conn.mu.Lock()
	conn.KeyVersion++
	version := conn.KeyVersion
	conn.mu.Unlock()

	payload := RekeyPayload{
		NewKeyEncrypted: encrypted,
		KeyVersion:      version,
	}
	msg, err := NewMessage(MsgRekey, conn.EggID, nestID, conn.SharedKey, payload)
	if err != nil {
		return fmt.Errorf("failed to create rekey message: %w", err)
	}
	if err := conn.Send(msg); err != nil {
		return err
	}

	// Rotate keys: current → previous, new → current
	conn.mu.Lock()
	conn.PreviousKey = conn.SharedKey
	conn.PreviousKeyAt = time.Now()
	conn.SharedKey = newKeyHex
	conn.mu.Unlock()

	h.logger.Info("Key rotated for egg", "nest_id", nestID, "version", version)
	return nil
}

// HandleMessages reads messages from an egg connection and dispatches them.
// Blocks until the connection closes or an error occurs.
func (h *EggHub) HandleMessages(conn *EggConnection) {
	defer h.Unregister(conn.NestID)

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

		// Verify HMAC (try current key, fall back to previous key within grace period)
		conn.mu.Lock()
		currentKey := conn.SharedKey
		prevKey := conn.PreviousKey
		prevKeyAt := conn.PreviousKeyAt
		conn.mu.Unlock()

		ok, err := VerifyMessage(msg, currentKey)
		if (err != nil || !ok) && prevKey != "" && time.Since(prevKeyAt) < 60*time.Second {
			ok, err = VerifyMessage(msg, prevKey)
		}
		if err != nil || !ok {
			h.logger.Warn("HMAC verification failed", "nest_id", conn.NestID)
			errMsg, _ := NewMessage(MsgError, conn.EggID, conn.NestID, conn.SharedKey,
				ErrorPayload{Code: "invalid_hmac", Message: "HMAC verification failed"})
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
				h.resolveAck(ack)
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
			var stale []struct{ nestID, eggID string }
			for nestID, conn := range h.connections {
				if !conn.LastHeartbeat.IsZero() && time.Since(conn.LastHeartbeat) > maxAge {
					stale = append(stale, struct{ nestID, eggID string }{nestID, conn.EggID})
				}
			}
			h.mu.RUnlock()

			for _, s := range stale {
				h.logger.Warn("Egg heartbeat stale", "nest_id", s.nestID, "egg_id", s.eggID)
				if onStale != nil {
					onStale(s.nestID, s.eggID)
				}
			}
		}
	}()
}
