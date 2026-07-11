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
	TypeSystemConnected                  MessageType = "system.connected"
	TypeSystemPing                       MessageType = "system.ping"
	TypeSystemPong                       MessageType = "system.pong"
	TypeSessionStart                     MessageType = "session.start"
	TypeSessionAccepted                  MessageType = "session.accepted"
	TypeChatMessage                      MessageType = "chat.message"
	TypeChatResponse                     MessageType = "chat.response"
	TypeChatError                        MessageType = "chat.error"
	TypeChatChunk                        MessageType = "chat.response.chunk"
	TypeChatPlanUpdate                   MessageType = "chat.plan_update"
	TypeAgentActivity                    MessageType = "agent.activity"
	TypeChatSessionsList                 MessageType = "chat.sessions.list"
	TypeChatSessions                     MessageType = "chat.sessions"
	TypeChatSessionCreate                MessageType = "chat.session.create"
	TypeChatSessionLoad                  MessageType = "chat.session.load"
	TypeChatSession                      MessageType = "chat.session"
	TypeChatCancel                       MessageType = "chat.cancel"
	TypeChatCancelled                    MessageType = "chat.cancelled"
	TypeChatAudio                        MessageType = "chat.audio"
	TypeChatMedia                        MessageType = "chat.media"
	TypeChatAttachmentPrepare            MessageType = "chat.attachment.prepare"
	TypeChatAttachmentPrepared           MessageType = "chat.attachment.prepared"
	TypeChatAttachmentAccepted           MessageType = "chat.attachment.accepted"
	TypeChatVoiceOutputStatus            MessageType = "chat.voice_output.status"
	TypeIntegrationsWebhostsList         MessageType = "integrations.webhosts.list"
	TypeIntegrationsWebhosts             MessageType = "integrations.webhosts"
	TypeSystemWarningsList               MessageType = "system.warnings.list"
	TypeSystemWarnings                   MessageType = "system.warnings"
	TypeSystemWarningAcknowledge         MessageType = "system.warning.acknowledge"
	TypeDesktopCommand                   MessageType = "desktop.command"
	TypeDesktopResult                    MessageType = "desktop.result"
	TypePersonaAssetsRequest             MessageType = "persona.assets.request"
	TypePersonaAssets                    MessageType = "persona.assets"
	TypeConfigProviderCatalogList        MessageType = "config.provider.catalog.list"
	TypeConfigProviderCatalogDetail      MessageType = "config.provider.catalog.detail"
	TypeConfigProviderCatalog            MessageType = "config.provider.catalog"
	TypeConfigProvidersList              MessageType = "config.providers.list"
	TypeConfigProviders                  MessageType = "config.providers"
	TypeConfigProviderGet                MessageType = "config.provider.get"
	TypeConfigProvider                   MessageType = "config.provider"
	TypeConfigProviderUpsert             MessageType = "config.provider.upsert"
	TypeConfigProviderDelete             MessageType = "config.provider.delete"
	TypeConfigProviderTest               MessageType = "config.provider.test"
	TypeConfigProviderTestResult         MessageType = "config.provider.test_result"
	TypeConfigProviderOAuthStart         MessageType = "config.provider.oauth.start"
	TypeConfigProviderOAuthStarted       MessageType = "config.provider.oauth.started"
	TypeConfigProviderOAuthComplete      MessageType = "config.provider.oauth.complete"
	TypeConfigProviderOAuthStatusRequest MessageType = "config.provider.oauth.status"
	TypeConfigProviderOAuthStatus        MessageType = "config.provider.oauth.status"
	TypeConfigProviderOAuthRevoke        MessageType = "config.provider.oauth.revoke"
)

const (
	ErrorAgentTimeout             = "AGENT_TIMEOUT"
	ErrorInvalidMessage           = "INVALID_MESSAGE"
	ErrorSessionNotFound          = "SESSION_NOT_FOUND"
	ErrorInternal                 = "INTERNAL_ERROR"
	ErrorAuthRequired             = "AUTH_REQUIRED"
	ErrorAuthFailed               = "AUTH_FAILED"
	ErrorPairingRequired          = "PAIRING_REQUIRED"
	ErrorDeviceNotApproved        = "DEVICE_NOT_APPROVED"
	ErrorRemoteReadOnly           = "REMOTE_READ_ONLY"
	ErrorRemotePermissionDenied   = "REMOTE_PERMISSION_DENIED"
	ErrorRemoteUnavailable        = "REMOTE_UNAVAILABLE"
	ErrorUnsupportedCapability    = "UNSUPPORTED_CAPABILITY"
	ErrorControlSessionActive     = "CONTROL_SESSION_ACTIVE"
	ErrorAttachmentRejected       = "ATTACHMENT_REJECTED"
	ErrorAttachmentTooLarge       = "ATTACHMENT_TOO_LARGE"
	ErrorAttachmentMimeNotAllowed = "ATTACHMENT_MIME_NOT_ALLOWED"
	ErrorAttachmentNotFound       = "ATTACHMENT_NOT_FOUND"
	ErrorAttachmentNotReady       = "ATTACHMENT_NOT_READY"
	ErrorAttachmentExpired        = "ATTACHMENT_EXPIRED"
)

var DefaultCapabilities = []string{
	"chat.full_response",
	"chat.server_push",
	"chat.agent_metadata",
	"chat.plan_updates",
	"chat.agent_activity",
	"chat.sessions",
	"chat.cancel",
	"chat.audio_events",
	"chat.media_events",
	"chat.voice_output_status",
	"integrations.webhosts",
	"system.warnings",
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

const (
	CapabilityConfigProvidersRead  = "config.providers.read"
	CapabilityConfigProvidersWrite = "config.providers.write"
	CapabilityConfigProvidersOAuth = "config.providers.oauth"
)

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
	SessionID              string                   `json:"session_id"`
	DeviceID               string                   `json:"device_id"`
	Approved               bool                     `json:"approved"`
	ReadOnly               bool                     `json:"read_only"`
	Capabilities           []string                 `json:"capabilities"`
	AdvertisedCapabilities []string                 `json:"advertised_capabilities,omitempty"`
	SharedKey              string                   `json:"shared_key,omitempty"`
	AttachmentLimits       *AttachmentLimitsPayload `json:"attachment_limits,omitempty"`
}

type ChatMessagePayload struct {
	SessionID      string               `json:"session_id"`
	ConversationID string               `json:"conversation_id,omitempty"`
	Text           string               `json:"text"`
	Role           string               `json:"role"`
	VoiceOutput    bool                 `json:"voice_output,omitempty"`
	Attachments    []ChatAttachmentItem `json:"attachments,omitempty"`
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

type AgentActivityPayload struct {
	ActivityID       string                        `json:"activity_id"`
	ParentActivityID string                        `json:"parent_activity_id,omitempty"`
	SessionID        string                        `json:"session_id"`
	ConversationID   string                        `json:"conversation_id,omitempty"`
	RequestID        string                        `json:"request_id,omitempty"`
	CommandID        string                        `json:"command_id,omitempty"`
	Kind             string                        `json:"kind"`
	Phase            string                        `json:"phase"`
	Title            string                        `json:"title,omitempty"`
	Summary          string                        `json:"summary,omitempty"`
	Risk             string                        `json:"risk,omitempty"`
	Progress         *AgentActivityProgressPayload `json:"progress,omitempty"`
}

type AgentActivityProgressPayload struct {
	Current int64  `json:"current,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Unit    string `json:"unit,omitempty"`
}

type ChatSessionSummary struct {
	ID           string `json:"id"`
	Preview      string `json:"preview"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
	MessageCount int    `json:"message_count"`
}

type ChatHistoryMessagePayload struct {
	Role        string               `json:"role"`
	Content     string               `json:"content"`
	Timestamp   string               `json:"timestamp,omitempty"`
	Attachments []ChatAttachmentItem `json:"attachments,omitempty"`
}

type AttachmentLimitsPayload struct {
	MaxFileBytes  int64    `json:"max_file_bytes"`
	MaxFiles      int      `json:"max_files"`
	MaxTotalBytes int64    `json:"max_total_bytes"`
	AllowedMime   []string `json:"allowed_mime,omitempty"`
}

type ChatAttachmentPreparePayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	Filename       string `json:"filename"`
	MimeType       string `json:"mime_type,omitempty"`
	SizeBytes      int64  `json:"size_bytes"`
	SHA256         string `json:"sha256,omitempty"`
}

type ChatAttachmentPreparedPayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	PrepareID      string `json:"prepare_id"`
	AttachmentID   string `json:"attachment_id"`
	UploadURL      string `json:"upload_url"`
	Method         string `json:"method"`
	UploadField    string `json:"upload_field"`
	ExpiresAt      string `json:"expires_at"`
	MaxBytes       int64  `json:"max_bytes"`
}

type ChatAttachmentAcceptedPayload struct {
	SessionID      string               `json:"session_id"`
	ConversationID string               `json:"conversation_id,omitempty"`
	Attachments    []ChatAttachmentItem `json:"attachments,omitempty"`
}

type ChatAttachmentItem struct {
	AttachmentID string                 `json:"attachment_id"`
	Kind         string                 `json:"kind,omitempty"`
	Path         string                 `json:"path,omitempty"`
	PreviewURL   string                 `json:"preview_url,omitempty"`
	URL          string                 `json:"url,omitempty"`
	Title        string                 `json:"title,omitempty"`
	Caption      string                 `json:"caption,omitempty"`
	MimeType     string                 `json:"mime_type,omitempty"`
	Filename     string                 `json:"filename,omitempty"`
	SizeBytes    int64                  `json:"size_bytes,omitempty"`
	SHA256       string                 `json:"sha256,omitempty"`
	OpenMode     string                 `json:"open_mode,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
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

type ChatMediaPayload struct {
	SessionID      string                 `json:"session_id"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	RequestID      string                 `json:"request_id,omitempty"`
	Kind           string                 `json:"kind"`
	Path           string                 `json:"path,omitempty"`
	PreviewURL     string                 `json:"preview_url,omitempty"`
	URL            string                 `json:"url,omitempty"`
	EmbedURL       string                 `json:"embed_url,omitempty"`
	VideoID        string                 `json:"video_id,omitempty"`
	Title          string                 `json:"title,omitempty"`
	Caption        string                 `json:"caption,omitempty"`
	MimeType       string                 `json:"mime_type,omitempty"`
	Filename       string                 `json:"filename,omitempty"`
	Format         string                 `json:"format,omitempty"`
	Provider       string                 `json:"provider,omitempty"`
	StartSeconds   int                    `json:"start_seconds,omitempty"`
	DurationMs     int64                  `json:"duration_ms,omitempty"`
	OpenMode       string                 `json:"open_mode,omitempty"`
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

type WebhostIntegrationPayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	Icon        string `json:"icon,omitempty"`
}

type IntegrationsWebhostsListPayload struct {
	SessionID string `json:"session_id"`
}

type IntegrationsWebhostsPayload struct {
	SessionID string                      `json:"session_id"`
	Status    string                      `json:"status"`
	Webhosts  []WebhostIntegrationPayload `json:"webhosts"`
}

type SystemWarningsListPayload struct {
	SessionID string `json:"session_id"`
}

type SystemWarningPayload struct {
	ID           string `json:"id"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Category     string `json:"category"`
	Timestamp    string `json:"timestamp"`
	Acknowledged bool   `json:"acknowledged"`
}

type SystemWarningsPayload struct {
	SessionID      string                 `json:"session_id"`
	Warnings       []SystemWarningPayload `json:"warnings"`
	Total          int                    `json:"total"`
	Unacknowledged int                    `json:"unacknowledged"`
}

type SystemWarningAcknowledgePayload struct {
	SessionID string `json:"session_id"`
	ID        string `json:"id,omitempty"`
	All       bool   `json:"all,omitempty"`
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

type ConfigProviderCatalogListPayload struct {
	SessionID     string `json:"session_id"`
	IncludeModels bool   `json:"include_models,omitempty"`
}

type ConfigProviderCatalogDetailPayload struct {
	SessionID     string `json:"session_id"`
	ProviderID    string `json:"provider_id,omitempty"`
	ProviderType  string `json:"provider_type,omitempty"`
	IncludeModels bool   `json:"include_models,omitempty"`
}

type ConfigProviderCatalogPayload struct {
	SessionID string                           `json:"session_id"`
	Status    string                           `json:"status"`
	Enabled   bool                             `json:"enabled"`
	Metadata  ProviderCatalogMetadataPayload   `json:"metadata"`
	Providers []ProviderCatalogProviderPayload `json:"providers,omitempty"`
	Models    []ProviderCatalogModelPayload    `json:"models,omitempty"`
}

type ProviderCatalogMetadataPayload struct {
	PackageName   string   `json:"package_name,omitempty"`
	Version       string   `json:"version,omitempty"`
	TarballURL    string   `json:"tarball_url,omitempty"`
	SyncedAt      string   `json:"synced_at,omitempty"`
	RepositoryURL string   `json:"repository_url,omitempty"`
	License       string   `json:"license,omitempty"`
	Copyright     string   `json:"copyright,omitempty"`
	SourceFiles   []string `json:"source_files,omitempty"`
}

type ProviderCatalogProviderPayload struct {
	ID                         string                            `json:"id"`
	AuraProviderType           string                            `json:"aura_provider_type"`
	Name                       string                            `json:"name,omitempty"`
	DefaultModel               string                            `json:"default_model,omitempty"`
	EnvVars                    []string                          `json:"env_vars,omitempty"`
	OAuthProvider              string                            `json:"oauth_provider,omitempty"`
	OAuthSetup                 *ProviderCatalogOAuthSetupPayload `json:"oauth_setup,omitempty"`
	AllowUnauthenticated       bool                              `json:"allow_unauthenticated"`
	DynamicModelsAuthoritative bool                              `json:"dynamic_models_authoritative"`
	CatalogOnly                bool                              `json:"catalog_only"`
	Available                  bool                              `json:"available"`
	Availability               string                            `json:"availability"`
	ModelsCount                int                               `json:"models_count"`
}

type ProviderCatalogOAuthSetupPayload struct {
	Source            string   `json:"source,omitempty"`
	SourcePackage     string   `json:"source_package,omitempty"`
	SourceProvider    string   `json:"source_provider,omitempty"`
	Flow              string   `json:"flow,omitempty"`
	SetupURL          string   `json:"setup_url,omitempty"`
	DocsURL           string   `json:"docs_url,omitempty"`
	ConsoleLabel      string   `json:"console_label,omitempty"`
	RedirectURIField  string   `json:"redirect_uri_field,omitempty"`
	ClientIDField     string   `json:"client_id_field,omitempty"`
	ClientSecretField string   `json:"client_secret_field,omitempty"`
	ClientID          string   `json:"client_id,omitempty"`
	AuthURL           string   `json:"auth_url,omitempty"`
	TokenURL          string   `json:"token_url,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	CallbackPort      int      `json:"callback_port,omitempty"`
	CallbackPath      string   `json:"callback_path,omitempty"`
}

type ProviderCatalogModelPayload struct {
	ID            string                           `json:"id"`
	Provider      string                           `json:"provider"`
	Name          string                           `json:"name,omitempty"`
	API           string                           `json:"api,omitempty"`
	BaseURL       string                           `json:"base_url,omitempty"`
	ContextWindow int                              `json:"context_window,omitempty"`
	MaxTokens     int                              `json:"max_tokens,omitempty"`
	Capabilities  ProviderModelCapabilitiesPayload `json:"capabilities"`
	Cost          ProviderCatalogCostPayload       `json:"cost"`
	CatalogOnly   bool                             `json:"catalog_only"`
}

type ProviderCatalogCostPayload struct {
	Input      float64 `json:"input,omitempty"`
	Output     float64 `json:"output,omitempty"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
}

type ProviderModelCapabilitiesPayload struct {
	ToolCalling       bool `json:"tool_calling"`
	StructuredOutputs bool `json:"structured_outputs"`
	Multimodal        bool `json:"multimodal"`
	Reasoning         bool `json:"reasoning,omitempty"`
}

type ConfigProvidersListPayload struct {
	SessionID string `json:"session_id"`
}

type ConfigProvidersPayload struct {
	SessionID string                       `json:"session_id"`
	Status    string                       `json:"status"`
	Providers []ConfigProviderEntryPayload `json:"providers"`
}

type ConfigProviderGetPayload struct {
	SessionID  string `json:"session_id"`
	ProviderID string `json:"provider_id"`
}

type ConfigProviderPayload struct {
	SessionID string                     `json:"session_id"`
	Status    string                     `json:"status"`
	Provider  ConfigProviderEntryPayload `json:"provider"`
}

type ConfigProviderUpsertPayload struct {
	SessionID string                         `json:"session_id"`
	Mode      string                         `json:"mode"`
	Provider  ConfigProviderEntryPayload     `json:"provider"`
	Secrets   ConfigProviderSecretOpsPayload `json:"secrets,omitempty"`
}

type ConfigProviderDeletePayload struct {
	SessionID  string `json:"session_id"`
	ProviderID string `json:"provider_id"`
	Force      bool   `json:"force,omitempty"`
}

type ConfigProviderTestPayload struct {
	SessionID  string `json:"session_id"`
	ProviderID string `json:"provider_id"`
}

type ConfigProviderTestResultPayload struct {
	SessionID  string   `json:"session_id"`
	ProviderID string   `json:"provider_id"`
	Status     string   `json:"status"`
	OK         bool     `json:"ok"`
	Message    string   `json:"message,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

type ConfigProviderEntryPayload struct {
	ID                    string                           `json:"id"`
	Name                  string                           `json:"name"`
	Type                  string                           `json:"type"`
	BaseURL               string                           `json:"base_url"`
	Model                 string                           `json:"model"`
	AccountID             string                           `json:"account_id,omitempty"`
	AuthType              string                           `json:"auth_type"`
	OAuthAuthURL          string                           `json:"oauth_auth_url,omitempty"`
	OAuthTokenURL         string                           `json:"oauth_token_url,omitempty"`
	OAuthClientID         string                           `json:"oauth_client_id,omitempty"`
	OAuthScopes           string                           `json:"oauth_scopes,omitempty"`
	Models                []ProviderModelCostPayload       `json:"models,omitempty"`
	Capabilities          *ProviderCapabilitiesPayload     `json:"capabilities,omitempty"`
	EffectiveCapabilities ProviderCapabilitiesPayload      `json:"effective_capabilities,omitempty"`
	Secrets               ConfigProviderSecretsPayload     `json:"secrets"`
	OAuth                 ConfigProviderOAuthStatusPayload `json:"oauth"`
	References            []ProviderReferencePayload       `json:"references,omitempty"`
}

type ProviderModelCostPayload struct {
	Name             string  `json:"name"`
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
}

type ProviderCapabilitiesPayload struct {
	Auto              bool   `json:"auto"`
	ToolCalling       bool   `json:"tool_calling"`
	StructuredOutputs bool   `json:"structured_outputs"`
	Multimodal        bool   `json:"multimodal"`
	DetectedModel     string `json:"detected_model,omitempty"`
	Source            string `json:"source,omitempty"`
	Known             bool   `json:"known,omitempty"`
}

type ConfigProviderSecretsPayload struct {
	APIKey            SecretPresencePayload `json:"api_key"`
	OAuthClientSecret SecretPresencePayload `json:"oauth_client_secret"`
}

type SecretPresencePayload struct {
	Present bool `json:"present"`
}

type ConfigProviderSecretOpsPayload struct {
	APIKey            SecretOperationPayload `json:"api_key,omitempty"`
	OAuthClientSecret SecretOperationPayload `json:"oauth_client_secret,omitempty"`
}

type SecretOperationPayload struct {
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
}

type ProviderReferencePayload struct {
	Path string `json:"path"`
	Role string `json:"role,omitempty"`
}

type ConfigProviderOAuthStartPayload struct {
	SessionID   string `json:"session_id"`
	ProviderID  string `json:"provider_id"`
	RedirectURI string `json:"redirect_uri"`
}

type ConfigProviderOAuthStartedPayload struct {
	SessionID     string   `json:"session_id"`
	ProviderID    string   `json:"provider_id"`
	AuthURL       string   `json:"auth_url"`
	Mode          string   `json:"mode"`
	OAuthState    string   `json:"oauth_state"`
	ExpiresAt     string   `json:"expires_at"`
	FallbackModes []string `json:"fallback_modes,omitempty"`
	RedirectURI   string   `json:"redirect_uri"`
}

type ConfigProviderOAuthCompletePayload struct {
	SessionID   string `json:"session_id"`
	ProviderID  string `json:"provider_id,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`
	RedirectURI string `json:"redirect_uri,omitempty"`
	Code        string `json:"code,omitempty"`
	State       string `json:"state,omitempty"`
}

type ConfigProviderOAuthStatusRequestPayload struct {
	SessionID   string `json:"session_id"`
	ProviderID  string `json:"provider_id"`
	RedirectURI string `json:"redirect_uri,omitempty"`
}

type ConfigProviderOAuthRevokePayload struct {
	SessionID  string `json:"session_id"`
	ProviderID string `json:"provider_id"`
}

type ConfigProviderOAuthStatusPayload struct {
	SessionID       string   `json:"session_id,omitempty"`
	ProviderID      string   `json:"provider_id"`
	Status          string   `json:"status,omitempty"`
	Configured      bool     `json:"configured"`
	Authorized      bool     `json:"authorized"`
	Expired         bool     `json:"expired,omitempty"`
	Expiry          string   `json:"expiry,omitempty"`
	HasRefreshToken bool     `json:"has_refresh_token"`
	MissingFields   []string `json:"missing_fields,omitempty"`
	RedirectURI     string   `json:"redirect_uri,omitempty"`
	Mode            string   `json:"mode,omitempty"`
	Message         string   `json:"message,omitempty"`
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
