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
	SharedKey     string // hex-encoded
	LastHeartbeat time.Time
	Status        string // "connected" | "idle" | "busy" | "error"
	Telemetry     HeartbeatPayload
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

	// Callbacks (set by the server layer)
	OnConnect    func(nestID, eggID string)
	OnDisconnect func(nestID, eggID string)
	OnHeartbeat  func(nestID string, hb HeartbeatPayload)
	OnResult     func(nestID string, result ResultPayload)
}

// NewEggHub creates a new hub for managing egg connections.
func NewEggHub(logger *slog.Logger) *EggHub {
	return &EggHub{
		connections: make(map[string]*EggConnection),
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

// HandleMessages reads messages from an egg connection and dispatches them.
// Blocks until the connection closes or an error occurs.
func (h *EggHub) HandleMessages(conn *EggConnection) {
	defer h.Unregister(conn.NestID)

	// Rate limit: max 100 messages per second per connection
	const rateLimit = 100
	const rateBurst = 150
	tokens := rateBurst
	lastRefill := time.Now()

	for {
		_, data, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Warn("Egg connection error", "nest_id", conn.NestID, "error", err)
			}
			return
		}

		// Token bucket rate limiting
		now := time.Now()
		elapsed := now.Sub(lastRefill)
		tokens += int(elapsed.Seconds() * float64(rateLimit))
		if tokens > rateBurst {
			tokens = rateBurst
		}
		lastRefill = now
		if tokens <= 0 {
			h.logger.Warn("Rate limit exceeded for egg", "nest_id", conn.NestID)
			continue
		}
		tokens--

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			h.logger.Warn("Invalid message from egg", "nest_id", conn.NestID, "error", err)
			continue
		}

		// Verify HMAC
		ok, err := VerifyMessage(msg, conn.SharedKey)
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
		case MsgAck:
			// Ack received — logged for tracing
			h.logger.Debug("Ack received from egg", "nest_id", conn.NestID, "msg_id", msg.ID)
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
