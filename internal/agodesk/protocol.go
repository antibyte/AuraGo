package agodesk

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const ProtocolVersion = "agodesk.v1"

type MessageType string

const (
	TypeSystemConnected       MessageType = "system.connected"
	TypeSystemPing            MessageType = "system.ping"
	TypeSystemPong            MessageType = "system.pong"
	TypeSessionStart          MessageType = "session.start"
	TypeSessionAccepted       MessageType = "session.accepted"
	TypeChatMessage           MessageType = "chat.message"
	TypeChatResponse          MessageType = "chat.response"
	TypeChatError             MessageType = "chat.error"
	TypeChatChunk             MessageType = "chat.response.chunk"
	TypeChatPlanUpdate        MessageType = "chat.plan_update"
	TypeChatSessionsList      MessageType = "chat.sessions.list"
	TypeChatSessions          MessageType = "chat.sessions"
	TypeChatSessionCreate     MessageType = "chat.session.create"
	TypeChatSessionLoad       MessageType = "chat.session.load"
	TypeChatSession           MessageType = "chat.session"
	TypeChatCancel            MessageType = "chat.cancel"
	TypeChatCancelled         MessageType = "chat.cancelled"
	TypeChatAudio             MessageType = "chat.audio"
	TypeChatVoiceOutputStatus MessageType = "chat.voice_output.status"
	TypeDesktopCommand        MessageType = "desktop.command"
	TypeDesktopResult         MessageType = "desktop.result"
	TypePersonaAssetsRequest  MessageType = "persona.assets.request"
	TypePersonaAssets         MessageType = "persona.assets"
)

const (
	ErrorAgentTimeout           = "AGENT_TIMEOUT"
	ErrorInvalidMessage         = "INVALID_MESSAGE"
	ErrorSessionNotFound        = "SESSION_NOT_FOUND"
	ErrorInternal               = "INTERNAL_ERROR"
	ErrorAuthRequired           = "AUTH_REQUIRED"
	ErrorAuthFailed             = "AUTH_FAILED"
	ErrorPairingRequired        = "PAIRING_REQUIRED"
	ErrorDeviceNotApproved      = "DEVICE_NOT_APPROVED"
	ErrorRemoteReadOnly         = "REMOTE_READ_ONLY"
	ErrorRemotePermissionDenied = "REMOTE_PERMISSION_DENIED"
	ErrorRemoteUnavailable      = "REMOTE_UNAVAILABLE"
	ErrorUnsupportedCapability  = "UNSUPPORTED_CAPABILITY"
	ErrorControlSessionActive   = "CONTROL_SESSION_ACTIVE"
)

var DefaultCapabilities = []string{
	"chat.full_response",
	"chat.streaming",
	"chat.server_push",
	"chat.agent_metadata",
	"chat.plan_updates",
	"chat.sessions",
	"chat.cancel",
	"chat.audio_events",
	"chat.voice_output_status",
	"pairing.remotehub",
	"remote.desktop.capture",
	"remote.desktop.permission_request",
	"remote.desktop.input",
	"remote.desktop.discovery",
	"remote.desktop.ui_automation",
	"remote.desktop.browser",
	"remote.files.read",
	"remote.files.write",
	"persona.assets",
}

const PersonaAssetVersion = "20260502-persona-refresh"

var corePersonaAssetKeys = map[string]bool{
	"evil":         true,
	"friend":       true,
	"mcp":          true,
	"mistress":     true,
	"neutral":      true,
	"professional": true,
	"psycho":       true,
	"punk":         true,
	"secretary":    true,
	"servant":      true,
	"terminator":   true,
	"thinker":      true,
}

type Envelope struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type SystemConnectedPayload struct {
	ProtocolVersion string   `json:"protocol_version"`
	ServerVersion   string   `json:"server_version"`
	SessionID       string   `json:"session_id"`
	AuthRequired    bool     `json:"auth_required"`
	PairingRequired bool     `json:"pairing_required"`
	Capabilities    []string `json:"capabilities"`
}

type SessionStartPayload struct {
	ClientVersion      string             `json:"client_version"`
	DeviceID           string             `json:"device_id,omitempty"`
	PairingToken       string             `json:"pairing_token,omitempty"`
	SharedKeyProof     *SharedKeyProof    `json:"shared_key_proof,omitempty"`
	ClientCapabilities []string           `json:"client_capabilities,omitempty"`
	FileAccess         *FileAccessPayload `json:"file_access,omitempty"`
	Host               SessionStartHost   `json:"host"`
	Metadata           map[string]string  `json:"metadata,omitempty"`
}

type FileAccessPayload struct {
	Enabled       bool             `json:"enabled"`
	Roots         []FileAccessRoot `json:"roots,omitempty"`
	MaxReadBytes  int64            `json:"max_read_bytes,omitempty"`
	MaxWriteBytes int64            `json:"max_write_bytes,omitempty"`
}

type FileAccessRoot struct {
	RootID      string   `json:"root_id"`
	Label       string   `json:"label,omitempty"`
	PathDisplay string   `json:"path_display,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

type SessionStartHost struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	IP       string `json:"ip,omitempty"`
}

type SharedKeyProof struct {
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	HMAC      string `json:"hmac"`
}

type SessionAcceptedPayload struct {
	SessionID              string   `json:"session_id"`
	DeviceID               string   `json:"device_id"`
	Approved               bool     `json:"approved"`
	ReadOnly               bool     `json:"read_only"`
	Capabilities           []string `json:"capabilities"`
	AdvertisedCapabilities []string `json:"advertised_capabilities,omitempty"`
	SharedKey              string   `json:"shared_key,omitempty"`
}

type ChatMessagePayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Text           string `json:"text"`
	Role           string `json:"role"`
	VoiceOutput    bool   `json:"voice_output,omitempty"`
}

type ChatResponsePayload struct {
	SessionID      string                 `json:"session_id"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	RequestID      string                 `json:"request_id"`
	Text           string                 `json:"text"`
	Role           string                 `json:"role"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type ChatErrorPayload struct {
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type ChatChunkPayload struct {
	SessionID      string                 `json:"session_id"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	RequestID      string                 `json:"request_id"`
	Delta          string                 `json:"delta"`
	Done           bool                   `json:"done"`
	Sequence       int                    `json:"sequence"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type ChatPlanUpdatePayload struct {
	SessionID      string          `json:"session_id"`
	ConversationID string          `json:"conversation_id,omitempty"`
	RequestID      string          `json:"request_id,omitempty"`
	Plan           json.RawMessage `json:"plan"`
}

type ChatSessionSummary struct {
	ID           string `json:"id"`
	Preview      string `json:"preview"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
	MessageCount int    `json:"message_count"`
}

type ChatHistoryMessagePayload struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

type ChatSessionsListPayload struct {
	SessionID string `json:"session_id"`
	Limit     int    `json:"limit,omitempty"`
}

type ChatSessionsPayload struct {
	SessionID string               `json:"session_id"`
	Sessions  []ChatSessionSummary `json:"sessions"`
}

type ChatSessionCreatePayload struct {
	SessionID string `json:"session_id"`
}

type ChatSessionLoadPayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id"`
}

type ChatSessionPayload struct {
	SessionID      string                      `json:"session_id"`
	ConversationID string                      `json:"conversation_id"`
	Session        ChatSessionSummary          `json:"session"`
	Messages       []ChatHistoryMessagePayload `json:"messages,omitempty"`
}

type ChatCancelPayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
}

type ChatCancelledPayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	Status         string `json:"status,omitempty"`
}

type ChatAudioPayload struct {
	SessionID      string                 `json:"session_id"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	RequestID      string                 `json:"request_id,omitempty"`
	Path           string                 `json:"path,omitempty"`
	Title          string                 `json:"title,omitempty"`
	MimeType       string                 `json:"mime_type,omitempty"`
	Filename       string                 `json:"filename,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type ChatVoiceOutputStatusPayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	SpeakerMode    bool   `json:"speaker_mode"`
	Mode           string `json:"mode,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Status         string `json:"status,omitempty"`
}

type PersonaAssetsRequestPayload struct {
	SessionID string `json:"session_id"`
}

type PersonaAssetsPayload struct {
	SessionID      string `json:"session_id"`
	Persona        string `json:"persona"`
	IconKey        string `json:"icon_key"`
	AvatarImageURL string `json:"avatar_image_url"`
	IconURL        string `json:"icon_url"`
	PersonaPrompt  string `json:"persona_prompt"`
	AssetVersion   string `json:"asset_version"`
}

type DesktopCommandPayload struct {
	CommandID string                 `json:"command_id"`
	Operation string                 `json:"operation"`
	Params    map[string]interface{} `json:"params,omitempty"`
}

type DesktopResultPayload struct {
	CommandID string                 `json:"command_id"`
	OK        bool                   `json:"ok,omitempty"`
	Success   *bool                  `json:"success,omitempty"`
	Status    string                 `json:"status,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	DeviceID  string                 `json:"device_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	ErrorCode string                 `json:"error_code,omitempty"`
}

func (p DesktopResultPayload) Succeeded() bool {
	if p.Success != nil {
		return *p.Success
	}
	if status := strings.TrimSpace(p.Status); status != "" {
		return strings.EqualFold(status, "ok")
	}
	return p.OK
}

func NegotiateCapabilities(clientCapabilities, serverCapabilities []string) []string {
	if len(clientCapabilities) == 0 || len(serverCapabilities) == 0 {
		return nil
	}
	server := make(map[string]struct{}, len(serverCapabilities))
	for _, capability := range serverCapabilities {
		capability = strings.TrimSpace(capability)
		if capability != "" {
			server[capability] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(clientCapabilities))
	negotiated := make([]string, 0, len(clientCapabilities))
	for _, capability := range clientCapabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		if _, ok := server[capability]; !ok {
			continue
		}
		if _, ok := seen[capability]; ok {
			continue
		}
		seen[capability] = struct{}{}
		negotiated = append(negotiated, capability)
	}
	return negotiated
}

func NewEnvelope(messageType MessageType, payload interface{}) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal payload: %w", err)
	}
	return Envelope{
		ID:        newMessageID(),
		Type:      messageType,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:   raw,
	}, nil
}

func NewPersonaAssetsRequest(sessionID string) (Envelope, error) {
	return NewEnvelope(TypePersonaAssetsRequest, PersonaAssetsRequestPayload{
		SessionID: strings.TrimSpace(sessionID),
	})
}

func DecodeEnvelope(data []byte, maxBytes int) (Envelope, error) {
	if maxBytes > 0 && len(data) > maxBytes {
		return Envelope{}, fmt.Errorf("message too large: %d bytes exceeds %d", len(data), maxBytes)
	}
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	if strings.TrimSpace(env.ID) == "" {
		return Envelope{}, fmt.Errorf("id is required")
	}
	if strings.TrimSpace(string(env.Type)) == "" {
		return Envelope{}, fmt.Errorf("type is required")
	}
	if strings.TrimSpace(env.Timestamp) == "" {
		return Envelope{}, fmt.Errorf("timestamp is required")
	}
	if _, err := time.Parse(time.RFC3339Nano, env.Timestamp); err != nil {
		if _, fallbackErr := time.Parse(time.RFC3339, env.Timestamp); fallbackErr != nil {
			return Envelope{}, fmt.Errorf("timestamp is invalid: %w", err)
		}
	}
	if env.Payload == nil {
		env.Payload = json.RawMessage(`{}`)
	}
	return env, nil
}

func NewSharedKeyProof(sharedKey, envelopeID, deviceID string, now time.Time) (SharedKeyProof, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return SharedKeyProof{}, fmt.Errorf("generate nonce: %w", err)
	}
	proof := SharedKeyProof{
		Nonce:     hex.EncodeToString(nonceBytes),
		Timestamp: now.UTC().Format(time.RFC3339Nano),
	}
	proof.HMAC = signSharedKeyProof(sharedKey, envelopeID, deviceID, proof.Nonce, proof.Timestamp)
	return proof, nil
}

func VerifySharedKeyProof(sharedKey, envelopeID, deviceID string, proof SharedKeyProof, now time.Time, maxSkew time.Duration) bool {
	if strings.TrimSpace(sharedKey) == "" ||
		strings.TrimSpace(envelopeID) == "" ||
		strings.TrimSpace(deviceID) == "" ||
		strings.TrimSpace(proof.Nonce) == "" ||
		strings.TrimSpace(proof.Timestamp) == "" ||
		strings.TrimSpace(proof.HMAC) == "" {
		return false
	}
	proofTime, err := time.Parse(time.RFC3339Nano, proof.Timestamp)
	if err != nil {
		proofTime, err = time.Parse(time.RFC3339, proof.Timestamp)
		if err != nil {
			return false
		}
	}
	if maxSkew > 0 {
		delta := now.Sub(proofTime)
		if delta < 0 {
			delta = -delta
		}
		if delta > maxSkew {
			return false
		}
	}
	want := signSharedKeyProof(sharedKey, envelopeID, deviceID, proof.Nonce, proof.Timestamp)
	return hmac.Equal([]byte(strings.ToLower(want)), []byte(strings.ToLower(strings.TrimSpace(proof.HMAC))))
}

func NewPersonaAssetsPayload(sessionID, personaName string, core bool, personaPrompt string) PersonaAssetsPayload {
	persona := strings.TrimSpace(personaName)
	if persona == "" {
		persona = "custom"
	}
	iconKey := PersonaAssetKey(persona, core)
	return PersonaAssetsPayload{
		SessionID:      strings.TrimSpace(sessionID),
		Persona:        persona,
		IconKey:        iconKey,
		AvatarImageURL: "/img/personas/" + iconKey + ".png?v=" + PersonaAssetVersion,
		IconURL:        "/img/persona-icons/" + iconKey + ".png?v=" + PersonaAssetVersion,
		PersonaPrompt:  strings.TrimSpace(personaPrompt),
		AssetVersion:   PersonaAssetVersion,
	}
}

func PersonaAssetKey(personaName string, core bool) string {
	key := strings.ToLower(strings.TrimSpace(personaName))
	if core && corePersonaAssetKeys[key] {
		return key
	}
	return "custom"
}

func signSharedKeyProof(sharedKey, envelopeID, deviceID, nonce, timestamp string) string {
	mac := hmac.New(sha256.New, sharedKeyProofBytes(sharedKey))
	mac.Write([]byte(ProtocolVersion))
	mac.Write([]byte("\nsession.start\n"))
	mac.Write([]byte(envelopeID))
	mac.Write([]byte("\n"))
	mac.Write([]byte(deviceID))
	mac.Write([]byte("\n"))
	mac.Write([]byte(nonce))
	mac.Write([]byte("\n"))
	mac.Write([]byte(timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}

func sharedKeyProofBytes(sharedKey string) []byte {
	if decoded, err := hex.DecodeString(strings.TrimSpace(sharedKey)); err == nil && len(decoded) > 0 {
		return decoded
	}
	return []byte(sharedKey)
}

func newMessageID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf)
}
