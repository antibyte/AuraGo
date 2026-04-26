package bridge

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
	MasterURL     string
	EggID         string
	NestID        string
	SharedKey     string // hex-encoded
	Version       string
	TLSSkipVerify bool // skip TLS certificate verification (for self-signed certs)
	HTTPClient    *http.Client

	conn        *websocket.Conn
	mu          sync.Mutex
	logger      *slog.Logger
	done        chan struct{}
	activeTasks int

	// Callbacks (set by the egg runtime)
	OnTask          func(task TaskPayload)           // called when master sends a task
	OnSecret        func(secret SecretPayload)       // called when master sends a secret
	OnStop          func()                           // called when master sends stop
	OnReconfigure   func(payload ReconfigurePayload) // called when master sends a safe config patch
	OnMissionSync   func(payload MissionSyncPayload) error
	OnMissionRun    func(payload MissionRunPayload) error
	OnMissionDelete func(payload MissionDeletePayload) error
}

type EggArtifactUpload struct {
	MissionID      string                 `json:"mission_id,omitempty"`
	TaskID         string                 `json:"task_id,omitempty"`
	Filename       string                 `json:"filename"`
	MIMEType       string                 `json:"mime_type,omitempty"`
	ExpectedSize   int64                  `json:"expected_size,omitempty"`
	ExpectedSHA256 string                 `json:"expected_sha256,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Reader         io.Reader              `json:"-"`
}

type EggArtifactUploadResult struct {
	Status     string `json:"status"`
	ArtifactID string `json:"artifact_id"`
	WebPath    string `json:"web_path"`
	SHA256     string `json:"sha256,omitempty"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
}

type EggHostMessage struct {
	MissionID       string   `json:"mission_id,omitempty"`
	TaskID          string   `json:"task_id,omitempty"`
	Severity        string   `json:"severity,omitempty"`
	Title           string   `json:"title,omitempty"`
	Body            string   `json:"body,omitempty"`
	ArtifactIDs     []string `json:"artifact_ids,omitempty"`
	DedupKey        string   `json:"dedup_key,omitempty"`
	WakeupRequested bool     `json:"wakeup_requested,omitempty"`
}

type EggHostMessageResult struct {
	Status        string `json:"status"`
	MessageID     string `json:"message_id"`
	WakeupAllowed bool   `json:"wakeup_allowed"`
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

// SendMissionResult sends a synced mission completion result back to the master.
func (c *EggClient) SendMissionResult(result MissionResultPayload) error {
	msg, err := NewMessage(MsgMissionResult, c.EggID, c.NestID, c.SharedKey, result)
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

// UploadArtifact reserves a host-side artifact slot and streams the file to it.
func (c *EggClient) UploadArtifact(ctx context.Context, upload EggArtifactUpload) (EggArtifactUploadResult, error) {
	if upload.Reader == nil {
		return EggArtifactUploadResult{}, fmt.Errorf("artifact reader is required")
	}
	body, err := json.Marshal(upload)
	if err != nil {
		return EggArtifactUploadResult{}, fmt.Errorf("marshal artifact offer: %w", err)
	}
	baseURL := c.masterHTTPBaseURL()
	offerURL := baseURL + "/api/invasion/artifacts/offer"
	req, err := c.newSignedHTTPRequest(ctx, http.MethodPost, offerURL, body)
	if err != nil {
		return EggArtifactUploadResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return EggArtifactUploadResult{}, fmt.Errorf("artifact offer request failed: %w", err)
	}
	defer resp.Body.Close()
	var offer struct {
		Status      string `json:"status"`
		ArtifactID  string `json:"artifact_id"`
		UploadToken string `json:"upload_token"`
		UploadURL   string `json:"upload_url"`
		WebPath     string `json:"web_path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&offer); err != nil {
		return EggArtifactUploadResult{}, fmt.Errorf("decode artifact offer response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return EggArtifactUploadResult{}, fmt.Errorf("artifact offer rejected: %s", offer.Status)
	}
	uploadURL := offer.UploadURL
	if strings.HasPrefix(uploadURL, "/") {
		uploadURL = baseURL + uploadURL
	}
	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, upload.Reader)
	if err != nil {
		return EggArtifactUploadResult{}, err
	}
	if upload.MIMEType != "" {
		uploadReq.Header.Set("Content-Type", upload.MIMEType)
	}
	uploadResp, err := client.Do(uploadReq)
	if err != nil {
		return EggArtifactUploadResult{}, fmt.Errorf("artifact upload request failed: %w", err)
	}
	defer uploadResp.Body.Close()
	var result EggArtifactUploadResult
	if err := json.NewDecoder(uploadResp.Body).Decode(&result); err != nil {
		return EggArtifactUploadResult{}, fmt.Errorf("decode artifact upload response: %w", err)
	}
	if uploadResp.StatusCode != http.StatusOK {
		return EggArtifactUploadResult{}, fmt.Errorf("artifact upload rejected: %s", result.Status)
	}
	if result.ArtifactID == "" {
		result.ArtifactID = offer.ArtifactID
	}
	if result.WebPath == "" {
		result.WebPath = offer.WebPath
	}
	return result, nil
}

// SendHostMessage stores a rate-limited message on the host and optionally wakes the host agent.
func (c *EggClient) SendHostMessage(ctx context.Context, msg EggHostMessage) (EggHostMessageResult, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return EggHostMessageResult{}, fmt.Errorf("marshal egg message: %w", err)
	}
	req, err := c.newSignedHTTPRequest(ctx, http.MethodPost, c.masterHTTPBaseURL()+"/api/invasion/messages", body)
	if err != nil {
		return EggHostMessageResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return EggHostMessageResult{}, fmt.Errorf("egg message request failed: %w", err)
	}
	defer resp.Body.Close()
	var result EggHostMessageResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return EggHostMessageResult{}, fmt.Errorf("decode egg message response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return EggHostMessageResult{}, fmt.Errorf("egg message rejected: %s", result.Status)
	}
	return result, nil
}

func (c *EggClient) newSignedHTTPRequest(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	mac, err := c.requestHMAC(method, req.URL.Path, ts, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-AuraGo-Nest-ID", c.NestID)
	req.Header.Set("X-AuraGo-Egg-ID", c.EggID)
	req.Header.Set("X-AuraGo-Timestamp", ts)
	req.Header.Set("X-AuraGo-Signature", mac)
	return req, nil
}

func (c *EggClient) requestHMAC(method, path, timestamp string, body []byte) (string, error) {
	key, err := hex.DecodeString(c.SharedKey)
	if err != nil {
		return "", fmt.Errorf("decode shared key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (c *EggClient) masterHTTPBaseURL() string {
	if u, err := url.Parse(strings.TrimSpace(c.MasterURL)); err == nil && u.Scheme != "" && u.Host != "" {
		switch u.Scheme {
		case "ws":
			u.Scheme = "http"
		case "wss":
			u.Scheme = "https"
		}
		u.Path = ""
		u.RawQuery = ""
		u.Fragment = ""
		return strings.TrimSuffix(u.String(), "/")
	}
	base := strings.TrimSuffix(c.MasterURL, "/api/invasion/ws")
	base = strings.TrimSuffix(base, "/")
	base = strings.TrimPrefix(base, "ws://")
	if strings.HasPrefix(c.MasterURL, "wss://") {
		base = strings.TrimPrefix(base, "wss://")
		return "https://" + base
	}
	return "http://" + base
}

func (c *EggClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	tr := &http.Transport{}
	if c.TLSSkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-opted self-signed cert
	}
	return &http.Client{Timeout: 10 * time.Minute, Transport: tr}
}

func (c *EggClient) connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	if c.TLSSkipVerify {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-opted self-signed cert
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
				c.sendAck(msg.ID, true, "")
				c.mu.Lock()
				c.activeTasks++
				c.mu.Unlock()
				go func() {
					c.OnTask(task)
					c.mu.Lock()
					c.activeTasks--
					c.mu.Unlock()
				}()
			}
		case MsgMissionSync:
			var payload MissionSyncPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.sendAck(msg.ID, false, "invalid payload")
			} else if c.OnMissionSync == nil {
				c.sendAck(msg.ID, false, "mission sync handler unavailable")
			} else {
				c.runAckedHandler(msg.ID, "mission synced", func() error {
					return c.OnMissionSync(payload)
				})
			}
		case MsgMissionRun:
			var payload MissionRunPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.sendAck(msg.ID, false, "invalid payload")
			} else if c.OnMissionRun == nil {
				c.sendAck(msg.ID, false, "mission run handler unavailable")
			} else {
				c.runAckedHandler(msg.ID, "mission run queued", func() error {
					return c.OnMissionRun(payload)
				})
			}
		case MsgMissionDelete:
			var payload MissionDeletePayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				c.sendAck(msg.ID, false, "invalid payload")
			} else if c.OnMissionDelete == nil {
				c.sendAck(msg.ID, false, "mission delete handler unavailable")
			} else {
				c.runAckedHandler(msg.ID, "mission deleted", func() error {
					return c.OnMissionDelete(payload)
				})
			}
		case MsgSecret:
			var secret SecretPayload
			if err := json.Unmarshal(msg.Payload, &secret); err == nil && c.OnSecret != nil {
				c.OnSecret(secret)
				c.sendAck(msg.ID, true, "secret stored")
			}
		case MsgRekey:
			var rekey RekeyPayload
			if err := json.Unmarshal(msg.Payload, &rekey); err != nil {
				c.logger.Warn("Invalid rekey payload", "error", err)
				c.sendAck(msg.ID, false, "invalid payload")
				continue
			}
			newKey, err := DecryptWithSharedKey(rekey.NewKeyEncrypted, c.SharedKey)
			if err != nil {
				c.logger.Warn("Failed to decrypt new key", "error", err)
				c.sendAck(msg.ID, false, "decryption failed")
				continue
			}
			c.mu.Lock()
			c.SharedKey = string(newKey)
			c.mu.Unlock()
			c.logger.Info("Shared key rotated", "version", rekey.KeyVersion)
			c.sendAck(msg.ID, true, fmt.Sprintf("key rotated to v%d", rekey.KeyVersion))
		case MsgSafeReconfigure:
			var reconfigPayload ReconfigurePayload
			if err := json.Unmarshal(msg.Payload, &reconfigPayload); err != nil {
				c.logger.Warn("Invalid safe_reconfigure payload", "error", err)
				c.sendAck(msg.ID, false, "invalid payload")
				continue
			}
			c.sendAck(msg.ID, true, "reconfigure received")
			if c.OnReconfigure != nil {
				go c.OnReconfigure(reconfigPayload)
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

func (c *EggClient) runAckedHandler(refID, successDetail string, handler func() error) {
	c.mu.Lock()
	c.activeTasks++
	c.mu.Unlock()
	go func() {
		defer func() {
			c.mu.Lock()
			c.activeTasks--
			c.mu.Unlock()
		}()
		if err := handler(); err != nil {
			c.sendAck(refID, false, err.Error())
			return
		}
		c.sendAck(refID, true, successDetail)
	}()
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

			c.mu.Lock()
			taskCount := c.activeTasks
			c.mu.Unlock()
			status := "idle"
			if taskCount > 0 {
				status = "busy"
			}
			hb := HeartbeatPayload{
				Status:      status,
				CPUPercent:  cpuP,
				MemPercent:  memP,
				Uptime:      upS,
				ActiveTasks: taskCount,
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
		c.logger.Warn("Failed to create ack message", "ref_id", refID, "error", err)
		return
	}
	c.mu.Lock()
	if c.conn != nil {
		if err := c.conn.WriteJSON(ack); err != nil {
			c.logger.Warn("Failed to send ack", "ref_id", refID, "error", err)
		}
	}
	c.mu.Unlock()
}
