// Package bridge implements the WebSocket communication protocol between
// the AuraGo master and its deployed egg workers. All messages are HMAC-signed
// with a per-nest shared key; secret payloads are additionally AES-256-GCM encrypted.
package bridge

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// ── Message types ───────────────────────────────────────────────────────────

const (
	MsgAuth            = "auth"             // egg → master: initial authentication
	MsgHeartbeat       = "heartbeat"        // egg → master: periodic health report
	MsgTask            = "task"             // master → egg: work assignment
	MsgResult          = "result"           // egg → master: task completion
	MsgMissionSync     = "mission_sync"     // master → egg: create/update a mission
	MsgMissionRun      = "mission_run"      // master → egg: run a synced mission
	MsgMissionDelete   = "mission_delete"   // master → egg: delete a synced mission
	MsgMissionResult   = "mission_result"   // egg → master: synced mission completion
	MsgStatus          = "status"           // master → egg: status request
	MsgSecret          = "secret"           // master → egg: encrypted vault secret
	MsgRekey           = "rekey"            // master → egg: shared key rotation
	MsgSafeReconfigure = "safe_reconfigure" // master → egg: safe config patch + restart
	MsgAck             = "ack"              // both directions: acknowledgement
	MsgError           = "error"            // both directions: error notification
	MsgStop            = "stop"             // master → egg: graceful shutdown
)

// Message is the wire format for all egg↔master communication.
type Message struct {
	Type      string          `json:"type"`
	EggID     string          `json:"egg_id"`
	NestID    string          `json:"nest_id"`
	ID        string          `json:"id,omitempty"`      // unique message ID for ack correlation
	Payload   json.RawMessage `json:"payload,omitempty"` // type-specific data
	Timestamp string          `json:"timestamp"`         // ISO 8601
	HMAC      string          `json:"hmac"`              // SHA-256 HMAC of Type+EggID+NestID+Timestamp+Payload
}

// ── Payload types ───────────────────────────────────────────────────────────

// AuthPayload is sent by the egg during the WebSocket handshake.
type AuthPayload struct {
	Version string `json:"version"` // AuraGo version
}

// HeartbeatPayload is sent periodically by the egg.
type HeartbeatPayload struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float64 `json:"mem_percent"`
	Uptime      int64   `json:"uptime_seconds"`
	ActiveTasks int     `json:"active_tasks"`
	Status      string  `json:"status"` // "idle" | "busy" | "error"
}

// TaskPayload is sent by the master to assign work.
type TaskPayload struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description"` // natural language task for the egg's agent loop
	Timeout     int    `json:"timeout"`     // seconds; 0 = no limit
}

// ResultPayload is sent by the egg after completing a task.
type ResultPayload struct {
	TaskID      string          `json:"task_id"`
	Status      string          `json:"status"` // "success" | "partial" | "failure"
	Output      string          `json:"output"`
	Error       string          `json:"error,omitempty"`
	Tokens      int             `json:"tokens_used"`
	ArtifactIDs []string        `json:"artifact_ids,omitempty"`
	Message     string          `json:"message,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// MissionSyncPayload installs or updates a mission on an egg.
type MissionSyncPayload struct {
	Revision       string          `json:"revision"`
	MissionID      string          `json:"mission_id"`
	Name           string          `json:"name"`
	PromptSnapshot string          `json:"prompt_snapshot"`
	ExecutionType  string          `json:"execution_type"`
	Schedule       string          `json:"schedule,omitempty"`
	TriggerType    string          `json:"trigger_type,omitempty"`
	TriggerConfig  json.RawMessage `json:"trigger_config,omitempty"`
	Priority       string          `json:"priority,omitempty"`
	Enabled        bool            `json:"enabled"`
	Locked         bool            `json:"locked,omitempty"`
	AutoPrepare    bool            `json:"auto_prepare,omitempty"`
	CreatedAt      time.Time       `json:"created_at,omitempty"`
}

// MissionRunPayload requests execution of a synced mission on an egg.
type MissionRunPayload struct {
	MissionID   string `json:"mission_id"`
	TriggerType string `json:"trigger_type,omitempty"`
	TriggerData string `json:"trigger_data,omitempty"`
}

// MissionDeletePayload removes a synced mission from an egg.
type MissionDeletePayload struct {
	MissionID string `json:"mission_id"`
}

// MissionResultPayload reports completion of a synced mission from an egg.
type MissionResultPayload struct {
	MissionID   string          `json:"mission_id"`
	Result      string          `json:"result"`
	Output      string          `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	ArtifactIDs []string        `json:"artifact_ids,omitempty"`
	Message     string          `json:"message,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// SecretPayload carries an encrypted vault secret from master to egg.
// The Value field is AES-256-GCM encrypted with the shared key (hex-encoded ciphertext).
type SecretPayload struct {
	Key            string `json:"key"`
	EncryptedValue string `json:"encrypted_value"` // hex-encoded AES-256-GCM ciphertext
}

// RekeyPayload carries a new shared key, encrypted with the current shared key.
type RekeyPayload struct {
	NewKeyEncrypted string `json:"new_key_encrypted"` // hex-encoded AES-256-GCM ciphertext of new hex key
	KeyVersion      int    `json:"key_version"`       // incrementing version number
}

// AckPayload acknowledges receipt of a specific message.
type AckPayload struct {
	RefID   string `json:"ref_id"` // ID of the acknowledged message
	Success bool   `json:"success"`
	Detail  string `json:"detail,omitempty"`
}

// ErrorPayload reports protocol-level errors.
type ErrorPayload struct {
	Code    string `json:"code"` // "auth_failed" | "invalid_hmac" | "unknown_type" | "internal"
	Message string `json:"message"`
}

// ReconfigurePayload carries a safe config patch from master to egg.
// The egg applies the patch to its local config.yaml and restarts.
type ReconfigurePayload struct {
	RevisionID string `json:"revision_id"` // safe_config_revisions ID for ack correlation
	PatchJSON  string `json:"patch_json"`  // JSON-serialized SafeConfigPatch
	ConfigHash string `json:"config_hash"` // SHA-256 hash of the resulting full config
}

// ── HMAC signing ────────────────────────────────────────────────────────────

// SignMessage computes and sets the HMAC field on a Message.
func SignMessage(msg *Message, sharedKeyHex string) error {
	key, err := hex.DecodeString(sharedKeyHex)
	if err != nil {
		return fmt.Errorf("invalid shared key: %w", err)
	}
	msg.HMAC = "" // clear before signing
	data := msg.Type + msg.EggID + msg.NestID + msg.Timestamp + string(msg.Payload)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	msg.HMAC = hex.EncodeToString(mac.Sum(nil))
	return nil
}

// VerifyMessage checks the HMAC signature of a Message.
func VerifyMessage(msg Message, sharedKeyHex string) (bool, error) {
	key, err := hex.DecodeString(sharedKeyHex)
	if err != nil {
		return false, fmt.Errorf("invalid shared key: %w", err)
	}
	expected := msg.HMAC
	data := msg.Type + msg.EggID + msg.NestID + msg.Timestamp + string(msg.Payload)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	computed := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(computed)), nil
}

var bridgeMessageCounter atomic.Uint64

func newMessageID(now time.Time) string {
	return fmt.Sprintf("%d-%d", now.UnixNano(), bridgeMessageCounter.Add(1))
}

// NewMessage creates a signed Message with the given payload.
func NewMessage(msgType, eggID, nestID, sharedKeyHex string, payload interface{}) (*Message, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		raw = b
	}
	now := time.Now().UTC()
	msg := &Message{
		Type:      msgType,
		EggID:     eggID,
		NestID:    nestID,
		ID:        newMessageID(now),
		Payload:   raw,
		Timestamp: now.Format(time.RFC3339),
	}
	if err := SignMessage(msg, sharedKeyHex); err != nil {
		return nil, err
	}
	return msg, nil
}

// ── Shared-key encryption for secret payloads ──────────────────────────────

// EncryptWithSharedKey encrypts plaintext using AES-256-GCM with the shared key.
// Returns the ciphertext as a hex-encoded string (nonce prepended).
func EncryptWithSharedKey(plaintext []byte, sharedKeyHex string) (string, error) {
	key, err := hex.DecodeString(sharedKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid shared key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(sealed), nil
}

// DecryptWithSharedKey decrypts a hex-encoded ciphertext using AES-256-GCM with the shared key.
func DecryptWithSharedKey(ciphertextHex, sharedKeyHex string) ([]byte, error) {
	key, err := hex.DecodeString(sharedKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid shared key: %w", err)
	}
	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext hex: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
