// Package remote implements the secure WebSocket communication protocol between
// the AuraGo supervisor and its deployed remote-control agents. All messages are
// HMAC-SHA256 signed with a per-device shared key over an mTLS transport layer.
package remote

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ── Message types ───────────────────────────────────────────────────────────

const (
	MsgAuth         = "auth"          // remote → supervisor: enrollment or reconnect
	MsgAuthResponse = "auth_response" // supervisor → remote: enrollment result + creds
	MsgHeartbeat    = "heartbeat"     // remote → supervisor: periodic health
	MsgCommand      = "command"       // supervisor → remote: execute operation
	MsgResult       = "result"        // remote → supervisor: command result
	MsgConfigUpdate = "config_update" // supervisor → remote: push config changes
	MsgRevoke       = "revoke"        // supervisor → remote: disconnect + uninstall
	MsgAck          = "ack"           // both: receipt confirmation
	MsgError        = "error"         // both: protocol error
)

const DefaultMaxFileSizeMB = 50

// ── Operations ──────────────────────────────────────────────────────────────

const (
	OpSysinfo                  = "sysinfo"
	OpFileRead                 = "file_read"
	OpFileWrite                = "file_write"
	OpFileList                 = "file_list"
	OpFileDelete               = "file_delete"
	OpShellExec                = "shell_exec"
	OpShellExecStream          = "shell_exec_stream"
	OpShellSessionStart        = "shell_session_start"
	OpShellSessionRead         = "shell_session_read"
	OpShellSessionInput        = "shell_session_input"
	OpShellSessionStop         = "shell_session_stop"
	OpShellSessionList         = "shell_session_list"
	OpFileEdit                 = "file_edit"
	OpFilePatch                = "file_patch"
	OpJsonEdit                 = "json_edit"
	OpYamlEdit                 = "yaml_edit"
	OpXmlEdit                  = "xml_edit"
	OpFileSearch               = "file_search"
	OpFileReadAdv              = "file_read_advanced"
	OpDesktopScreenshot        = "desktop_screenshot"
	OpDesktopStreamStart       = "desktop_stream_start"
	OpDesktopStreamStop        = "desktop_stream_stop"
	OpDesktopInput             = "desktop_input"
	OpDesktopPermissionRequest = "desktop_permission_request"
	OpDesktopListDisplays      = "desktop_list_displays"
	OpDesktopListWindows       = "desktop_list_windows"
	OpDesktopActiveWindow      = "desktop_active_window"
	OpDesktopHostInfo          = "desktop_host_info"
	OpDesktopUITree            = "desktop_ui_tree"
	OpDesktopUIAction          = "desktop_ui_action"
	OpDesktopBrowserConnect    = "desktop_browser_connect"
	OpDesktopBrowserSnapshot   = "desktop_browser_snapshot"
	OpDesktopBrowserAction     = "desktop_browser_action"
	OpDesktopBrowserDisconnect = "desktop_browser_disconnect"
	OpAgoDeskChatMessage       = "agodesk_chat_message"
)

// ReadOnlySafe reports whether an operation is safe in read-only mode.
func ReadOnlySafe(op string) bool {
	switch op {
	case OpSysinfo, OpFileRead, OpFileList, OpFileSearch, OpFileReadAdv,
		OpDesktopScreenshot, OpDesktopStreamStart, OpDesktopStreamStop, OpDesktopPermissionRequest,
		OpDesktopListDisplays, OpDesktopListWindows, OpDesktopActiveWindow, OpDesktopHostInfo,
		OpDesktopUITree, OpDesktopBrowserConnect, OpDesktopBrowserSnapshot, OpDesktopBrowserDisconnect,
		OpAgoDeskChatMessage:
		return true
	default:
		return false
	}
}

// ── Wire message ────────────────────────────────────────────────────────────

// RemoteMessage is the wire format for all supervisor↔remote communication.
type RemoteMessage struct {
	Type      string          `json:"type"`
	DeviceID  string          `json:"device_id"`
	MessageID string          `json:"msg_id"`            // UUID for dedup/ack correlation
	Sequence  uint64          `json:"seq"`               // monotonic counter
	Nonce     string          `json:"nonce"`             // 16 bytes random hex
	Timestamp string          `json:"ts"`                // ISO 8601
	Payload   json.RawMessage `json:"payload,omitempty"` // type-specific data
	HMAC      string          `json:"hmac"`              // SHA-256 hex
}

// ── Payload types ───────────────────────────────────────────────────────────

// AuthPayload is sent by the remote during enrollment or reconnection.
type AuthPayload struct {
	Version  string `json:"version"`
	Hostname string `json:"hostname,omitempty"`
	OS       string `json:"os,omitempty"`
	Arch     string `json:"arch,omitempty"`
	IP       string `json:"ip,omitempty"`
	// KDF names the token derivation behind TokenHash and the frame's HMAC
	// key; enrollment requires EnrollmentKDFVersion.
	KDF int `json:"kdf,omitempty"`
	// TokenHash is DeriveEnrollmentLookupHash(token). It only selects the
	// enrollment row; it is not the key that signs this frame.
	TokenHash string `json:"token_hash,omitempty"`
	DeviceID  string `json:"device_id,omitempty"` // set on reconnection
}

// AuthResponsePayload is sent by the supervisor after enrollment/auth.
type AuthResponsePayload struct {
	Status        string   `json:"status"`              // "enrolled", "authenticated", "pending", "rejected"
	DeviceID      string   `json:"device_id,omitempty"` // assigned device UUID
	SharedKey     string   `json:"shared_key,omitempty"`
	Message       string   `json:"message,omitempty"`
	ReadOnly      *bool    `json:"read_only,omitempty"`
	AllowedPaths  []string `json:"allowed_paths"`
	MaxFileSizeMB int      `json:"max_file_size_mb,omitempty"`
	// RequestNonce echoes the Nonce of the auth frame this answers. It is
	// covered by the HMAC, so a signed answer is valid for that one request.
	RequestNonce string `json:"request_nonce,omitempty"`
}

// HeartbeatPayload is sent periodically by the remote.
type HeartbeatPayload struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float64 `json:"mem_percent"`
	DiskUsedGB  float64 `json:"disk_used_gb"`
	DiskTotalGB float64 `json:"disk_total_gb"`
	Uptime      int64   `json:"uptime_seconds"`
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	Version     string  `json:"version"`
}

// CommandPayload is sent by the supervisor to execute an operation.
type CommandPayload struct {
	CommandID  string                 `json:"cmd_id"`
	Operation  string                 `json:"op"`
	Args       map[string]interface{} `json:"args"`
	TimeoutSec int                    `json:"timeout_sec"`
}

// ResultPayload is sent by the remote after executing a command.
type ResultPayload struct {
	CommandID  string `json:"cmd_id"`
	Status     string `json:"status"` // "ok", "error", "denied", "timeout"
	Output     string `json:"output"`
	ErrorCode  string `json:"error_code,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// ConfigUpdatePayload pushes configuration changes to the remote.
type ConfigUpdatePayload struct {
	ReadOnly      *bool    `json:"read_only,omitempty"`
	AllowedPaths  []string `json:"allowed_paths,omitempty"`
	MaxFileSizeMB *int     `json:"max_file_size_mb,omitempty"`
}

// Omitted paths mean unchanged in a partial update; an explicit empty slice
// revokes access. A slice with omitempty cannot preserve that distinction.
func (p ConfigUpdatePayload) MarshalJSON() ([]byte, error) {
	type wire struct {
		ReadOnly      *bool     `json:"read_only,omitempty"`
		AllowedPaths  *[]string `json:"allowed_paths,omitempty"`
		MaxFileSizeMB *int      `json:"max_file_size_mb,omitempty"`
	}
	result := wire{ReadOnly: p.ReadOnly, MaxFileSizeMB: p.MaxFileSizeMB}
	if p.AllowedPaths != nil {
		result.AllowedPaths = &p.AllowedPaths
	}
	return json.Marshal(result)
}

// AckPayload acknowledges receipt of a message.
type AckPayload struct {
	RefID   string `json:"ref_id"`
	Success bool   `json:"success"`
	Detail  string `json:"detail,omitempty"`
}

// ErrorPayload reports protocol-level errors.
type ErrorPayload struct {
	Code    string `json:"code"` // "auth_failed", "invalid_hmac", "replay", "unknown_type", "internal"
	Message string `json:"message"`
}

// ── HMAC signing ────────────────────────────────────────────────────────────

// hmacData builds the canonical string for HMAC computation.
//
// The fields are concatenated without delimiters, so on its own the encoding is
// ambiguous: a frame signed with sequence 12 and nonce N also verifies as
// sequence 1 with nonce "2"+N. What keeps it unambiguous today are the
// receivers' checks: ValidNonce pins the nonce to exactly 32 hex characters, so
// nothing moves across either nonce boundary; the RFC3339 timestamp parse and
// the JSON payload reject characters moved across the timestamp boundary; any
// other shift leaves the nonce unchanged for the replay cache to catch. The
// long-term fix is a delimited canonical form; that is a wire change and must
// ship on the agent and the supervisor together.
func hmacData(msg *RemoteMessage) string {
	return msg.Type + msg.DeviceID + msg.MessageID +
		fmt.Sprintf("%d", msg.Sequence) + msg.Nonce +
		msg.Timestamp + string(msg.Payload)
}

// decodeSharedKey decodes a hex HMAC key. hex.DecodeString("") succeeds with a
// zero-length key, and an HMAC under the empty key is computable by anyone, so
// that case is an error too.
func decodeSharedKey(sharedKeyHex string) ([]byte, error) {
	key, err := hex.DecodeString(sharedKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid shared key: %w", err)
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("invalid shared key: empty")
	}
	return key, nil
}

// SignMessage computes and sets the HMAC field on a RemoteMessage.
func SignMessage(msg *RemoteMessage, sharedKeyHex string) error {
	key, err := decodeSharedKey(sharedKeyHex)
	if err != nil {
		return err
	}
	msg.HMAC = "" // clear before signing
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(hmacData(msg)))
	msg.HMAC = hex.EncodeToString(mac.Sum(nil))
	return nil
}

// VerifyMessage checks the HMAC signature of a RemoteMessage.
func VerifyMessage(msg RemoteMessage, sharedKeyHex string) (bool, error) {
	key, err := decodeSharedKey(sharedKeyHex)
	if err != nil {
		return false, err
	}
	expected := msg.HMAC
	msg.HMAC = ""
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(hmacData(&msg)))
	computed := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(computed)), nil
}

// ── Message construction ────────────────────────────────────────────────────

// GenerateNonce returns a 16-byte random hex string.
func GenerateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// nonceHexLen is the length of a GenerateNonce value: 16 bytes as hex.
const nonceHexLen = 32

// ValidNonce reports whether nonce has the exact GenerateNonce format: 32
// lowercase hex characters. Receivers must check it before consulting the
// replay cache, because hmacData lets sequence digits shift into the nonce.
func ValidNonce(nonce string) bool {
	if len(nonce) != nonceHexLen {
		return false
	}
	for i := 0; i < len(nonce); i++ {
		c := nonce[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// NewMessage creates a signed RemoteMessage with the given payload.
// sharedKeyHex may be empty for unauthenticated enrollment messages.
func NewMessage(msgType, deviceID, sharedKeyHex string, seq uint64, payload interface{}) (*RemoteMessage, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		raw = b
	}

	nonce, err := GenerateNonce()
	if err != nil {
		return nil, err
	}

	msg := &RemoteMessage{
		Type:      msgType,
		DeviceID:  deviceID,
		MessageID: fmt.Sprintf("%d", time.Now().UnixNano()),
		Sequence:  seq,
		Nonce:     nonce,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   raw,
	}

	if sharedKeyHex != "" {
		if err := SignMessage(msg, sharedKeyHex); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

// NewAuthResponseMessage creates an auth response and signs it when a bootstrap
// or device shared key is available. Manual approval flows may remain unsigned
// because no trusted secret exists yet on either side.
func NewAuthResponseMessage(deviceID, signingKeyHex string, payload AuthResponsePayload) (*RemoteMessage, error) {
	if payload.AllowedPaths == nil {
		payload.AllowedPaths = []string{}
	}
	return NewMessage(MsgAuthResponse, deviceID, signingKeyHex, 0, payload)
}

// MaxTimestampDrift is the maximum allowed clock drift for anti-replay.
// 15 minutes accommodates typical home-lab setups where NTP sync may lag
// (Windows re-syncs every 7 days by default, VMs can drift significantly).
// Replay attacks still require capturing and replaying within this window.
const MaxTimestampDrift = 15 * time.Minute

// ValidateTimestamp checks that a message timestamp is within acceptable drift.
func ValidateTimestamp(ts string) error {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}
	drift := time.Since(t)
	if drift < 0 {
		drift = -drift
	}
	if drift > MaxTimestampDrift {
		return fmt.Errorf("timestamp drift %v exceeds maximum %v", drift, MaxTimestampDrift)
	}
	return nil
}

// GenerateSharedKey creates a new 32-byte (64 hex char) random shared key.
func GenerateSharedKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate shared key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// EnrollmentKDFVersion is the AuthPayload.KDF value for the split derivation
// below. Enrollment frames without it come from agents that sent the plain
// SHA-256 of the token, which doubled as the HMAC key.
const EnrollmentKDFVersion = 2

// Domain separators for the two values derived from one enrollment token.
const (
	enrollmentLookupDomain = "aurago-remote-enroll-lookup\n"
	enrollmentAuthDomain   = "aurago-remote-enroll-auth\n"
)

// DeriveEnrollmentLookupHash derives the value the agent sends in the clear to
// select its enrollment row; remote_enrollments.token_hash stores it. It is
// domain-separated from DeriveEnrollmentAuthKey, so it cannot be turned into
// the MAC key.
func DeriveEnrollmentLookupHash(token string) string {
	sum := sha256.Sum256([]byte(enrollmentLookupDomain + token))
	return hex.EncodeToString(sum[:])
}

// DeriveEnrollmentAuthKey derives the HMAC key (32 bytes, hex) that signs the
// enrollment frame and the supervisor's answer. It never travels: the agent
// derives it from its token and the supervisor keeps it in the vault until the
// token is consumed or expires.
func DeriveEnrollmentAuthKey(token string) string {
	sum := sha256.Sum256([]byte(enrollmentAuthDomain + token))
	return hex.EncodeToString(sum[:])
}
