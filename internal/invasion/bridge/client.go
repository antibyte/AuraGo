package bridge

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// EggClient implements the egg-side WebSocket connection to the master.
// It auto-reconnects with exponential backoff and sends periodic heartbeats.
type EggClient struct {
	MasterURL string
	EggID     string
	NestID    string
	SharedKey string // hex-encoded
	Version   string

	conn   *websocket.Conn
	mu     sync.Mutex
	logger *slog.Logger
	done   chan struct{}

	// Callbacks (set by the egg runtime)
	OnTask   func(task TaskPayload)     // called when master sends a task
	OnSecret func(secret SecretPayload) // called when master sends a secret
	OnStop   func()                     // called when master sends stop
}

// NewEggClient creates a new client for connecting to the master.
func NewEggClient(masterURL, eggID, nestID, sharedKey, version string, logger *slog.Logger) *EggClient {
	return &EggClient{
		MasterURL: masterURL,
		EggID:     eggID,
		NestID:    nestID,
		SharedKey: sharedKey,
		Version:   version,
		logger:    logger,
		done:      make(chan struct{}),
	}
}

// Start connects to the master and enters the read loop.
// Blocks until Stop() is called. Auto-reconnects on failure.
func (c *EggClient) Start() {
	backoff := 5 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-c.done:
			return
		default:
		}

		if err := c.connect(); err != nil {
			c.logger.Warn("Failed to connect to master", "url", c.MasterURL, "error", err, "retry_in", backoff)
			select {
			case <-time.After(backoff):
				backoff = backoff * 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			case <-c.done:
				return
			}
			continue
		}

		// Reset backoff on successful connection
		backoff = 5 * time.Second
		c.logger.Info("Connected to master", "url", c.MasterURL)

		// Start heartbeat sender
		heartbeatDone := make(chan struct{})
		go c.heartbeatLoop(heartbeatDone)

		// Read loop (blocks until disconnect)
		c.readLoop()

		close(heartbeatDone)
		c.logger.Warn("Disconnected from master, will reconnect...")
	}
}

// Stop gracefully closes the connection.
func (c *EggClient) Stop() {
	select {
	case <-c.done:
		return // already stopped
	default:
		close(c.done)
	}
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "egg shutting down"))
		_ = c.conn.Close()
	}
	c.mu.Unlock()
}

// SendResult sends a task result back to the master.
func (c *EggClient) SendResult(result ResultPayload) error {
	msg, err := NewMessage(MsgResult, c.EggID, c.NestID, c.SharedKey, result)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteJSON(msg)
}

func (c *EggClient) connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.MasterURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	// Send auth message
	authMsg, err := NewMessage(MsgAuth, c.EggID, c.NestID, c.SharedKey, AuthPayload{
		Version: c.Version,
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create auth message: %w", err)
	}

	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send auth: %w", err)
	}

	// Wait for ack
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("auth response timeout: %w", err)
	}
	conn.SetReadDeadline(time.Time{}) // clear deadline

	var ackMsg Message
	if err := json.Unmarshal(data, &ackMsg); err != nil {
		conn.Close()
		return fmt.Errorf("invalid auth response: %w", err)
	}

	if ackMsg.Type == MsgError {
		conn.Close()
		var errPayload ErrorPayload
		_ = json.Unmarshal(ackMsg.Payload, &errPayload)
		return fmt.Errorf("auth rejected: %s", errPayload.Message)
	}

	if ackMsg.Type != MsgAck {
		conn.Close()
		return fmt.Errorf("unexpected auth response type: %s", ackMsg.Type)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	return nil
}

func (c *EggClient) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("Read error from master", "error", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Warn("Invalid message from master", "error", err)
			continue
		}

		// Verify HMAC
		ok, err := VerifyMessage(msg, c.SharedKey)
		if err != nil || !ok {
			c.logger.Warn("HMAC verification failed for master message")
			continue
		}

		switch msg.Type {
		case MsgTask:
			var task TaskPayload
			if err := json.Unmarshal(msg.Payload, &task); err == nil && c.OnTask != nil {
				// Send ack
				c.sendAck(msg.ID, true, "")
				go c.OnTask(task)
			}
		case MsgSecret:
			var secret SecretPayload
			if err := json.Unmarshal(msg.Payload, &secret); err == nil && c.OnSecret != nil {
				c.OnSecret(secret)
				c.sendAck(msg.ID, true, "secret stored")
			}
		case MsgStop:
			c.logger.Info("Stop command received from master")
			c.sendAck(msg.ID, true, "stopping")
			if c.OnStop != nil {
				c.OnStop()
			}
			return
		case MsgAck:
			// Ack from master — nothing to do
		case MsgError:
			var errPayload ErrorPayload
			_ = json.Unmarshal(msg.Payload, &errPayload)
			c.logger.Warn("Error from master", "code", errPayload.Code, "msg", errPayload.Message)
		default:
			c.logger.Warn("Unknown message type from master", "type", msg.Type)
		}
	}
}

func (c *EggClient) heartbeatLoop(done chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-c.done:
			return
		case <-ticker.C:
			var cpuP float64
			if cpP, err := cpu.Percent(0, false); err == nil && len(cpP) > 0 {
				cpuP = cpP[0]
			}
			var memP float64
			if v, err := mem.VirtualMemory(); err == nil {
				memP = v.UsedPercent
			}
			var upS int64
			if h, err := host.Info(); err == nil {
				upS = int64(h.Uptime)
			}

			hb := HeartbeatPayload{
				Status:     "idle",
				CPUPercent: cpuP,
				MemPercent: memP,
				Uptime:     upS,
			}
			msg, err := NewMessage(MsgHeartbeat, c.EggID, c.NestID, c.SharedKey, hb)
			if err != nil {
				continue
			}
			c.mu.Lock()
			if c.conn != nil {
				_ = c.conn.WriteJSON(msg)
			}
			c.mu.Unlock()
		}
	}
}

func (c *EggClient) sendAck(refID string, success bool, detail string) {
	ack, err := NewMessage(MsgAck, c.EggID, c.NestID, c.SharedKey, AckPayload{
		RefID:   refID,
		Success: success,
		Detail:  detail,
	})
	if err != nil {
		return
	}
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.WriteJSON(ack)
	}
	c.mu.Unlock()
}
