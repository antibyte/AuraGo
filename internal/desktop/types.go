package desktop

import "time"

const (
	ControlConfirmDestructive = "confirm_destructive"
	ControlTrusted            = "trusted"

	SourceAgent = "agent"
	SourceUser  = "user"

	AuraDesktopRuntime = "aura-desktop-sdk@1"
	BuiltinRuntime     = "builtin"
	WidgetTypeCustom   = "custom"
)

// Config describes the runtime settings needed by the desktop service.
type Config struct {
	Enabled            bool
	ReadOnly           bool
	AllowAgentControl  bool
	AllowGeneratedApps bool
	AllowPythonJobs    bool
	WorkspaceDir       string
	DBPath             string
	MaxFileSizeMB      int
	ControlLevel       string
	MaxWSClients       int
}

// WorkspaceInfo is the public workspace state returned to the browser.
type WorkspaceInfo struct {
	Root        string   `json:"root"`
	Directories []string `json:"directories"`
	MaxFileSize int64    `json:"max_file_size"`
}

// BootstrapPayload is the initial state used by the virtual desktop UI.
type BootstrapPayload struct {
	Enabled            bool              `json:"enabled"`
	ReadOnly           bool              `json:"readonly"`
	AllowAgentControl  bool              `json:"allow_agent_control"`
	AllowGeneratedApps bool              `json:"allow_generated_apps"`
	AllowPythonJobs    bool              `json:"allow_python_jobs"`
	ControlLevel       string            `json:"control_level"`
	Workspace          WorkspaceInfo     `json:"workspace"`
	BuiltinApps        []AppManifest     `json:"builtin_apps"`
	InstalledApps      []AppManifest     `json:"installed_apps"`
	Widgets            []Widget          `json:"widgets"`
	Settings           map[string]string `json:"settings"`
}

// FileEntry describes one file or directory in the desktop workspace.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Type    string    `json:"type"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// AppManifest describes a browser-side desktop application.
type AppManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Icon        string            `json:"icon"`
	Entry       string            `json:"entry"`
	Runtime     string            `json:"runtime,omitempty"`
	Description string            `json:"description,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty"`
}

// Widget describes a pinned desktop widget backed by a built-in or installed app.
type Widget struct {
	ID          string                 `json:"id"`
	AppID       string                 `json:"app_id,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Title       string                 `json:"title"`
	Icon        string                 `json:"icon,omitempty"`
	Entry       string                 `json:"entry,omitempty"`
	Runtime     string                 `json:"runtime,omitempty"`
	Permissions []string               `json:"permissions,omitempty"`
	X           int                    `json:"x"`
	Y           int                    `json:"y"`
	W           int                    `json:"w"`
	H           int                    `json:"h"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at,omitempty"`
}

// Event is sent over WebSocket/SSE when the desktop state changes.
type Event struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

// SettingDefinition describes one safe, user-editable desktop setting.
type SettingDefinition struct {
	Key     string
	Default string
	Values  []string
}

// DesktopSettingDefinitions returns all settings that may be persisted by the desktop UI.
func DesktopSettingDefinitions() []SettingDefinition {
	return []SettingDefinition{
		{Key: "appearance.wallpaper", Default: "aurora", Values: []string{"aurora", "midnight", "slate", "ember", "forest"}},
		{Key: "appearance.accent", Default: "teal", Values: []string{"teal", "orange", "blue", "violet", "green"}},
		{Key: "appearance.density", Default: "comfortable", Values: []string{"comfortable", "compact"}},
		{Key: "desktop.icon_size", Default: "medium", Values: []string{"small", "medium", "large"}},
		{Key: "desktop.show_widgets", Default: "true", Values: []string{"true", "false"}},
		{Key: "windows.animations", Default: "true", Values: []string{"true", "false"}},
		{Key: "windows.default_size", Default: "balanced", Values: []string{"compact", "balanced", "large"}},
		{Key: "files.confirm_delete", Default: "true", Values: []string{"true", "false"}},
		{Key: "files.default_folder", Default: "Documents", Values: []string{"Desktop", "Documents", "Downloads", "Pictures", "Shared"}},
		{Key: "agent.show_chat_button", Default: "true", Values: []string{"true", "false"}},
	}
}

// DesktopSettingDefaults returns default values for all safe desktop settings.
func DesktopSettingDefaults() map[string]string {
	defaults := map[string]string{}
	for _, def := range DesktopSettingDefinitions() {
		defaults[def.Key] = def.Default
	}
	return defaults
}

// DefaultDirectories returns the persistent workspace folders exposed by the desktop.
func DefaultDirectories() []string {
	return []string{"Desktop", "Documents", "Downloads", "Apps", "Widgets", "Data", "Pictures", "Trash", "Shared"}
}

// BuiltinApps returns the first-party applications always available in the shell.
func BuiltinApps() []AppManifest {
	return []AppManifest{
		{ID: "files", Name: "Files", Version: "1.0.0", Icon: "folder", Entry: "builtin://files", Runtime: BuiltinRuntime, Description: "Browse and manage desktop workspace files."},
		{ID: "editor", Name: "Editor", Version: "1.0.0", Icon: "edit", Entry: "builtin://editor", Runtime: BuiltinRuntime, Description: "Edit workspace text files."},
		{ID: "settings", Name: "Settings", Version: "1.0.0", Icon: "settings", Entry: "builtin://settings", Runtime: BuiltinRuntime, Description: "Inspect virtual desktop settings."},
		{ID: "calendar", Name: "Calendar", Version: "1.0.0", Icon: "calendar", Entry: "builtin://calendar", Runtime: BuiltinRuntime, Description: "Local calendar surface for the desktop."},
		{ID: "agent-chat", Name: "Agent Chat", Version: "1.0.0", Icon: "sparkles", Entry: "builtin://agent-chat", Runtime: BuiltinRuntime, Description: "Ask AuraGo to create apps, widgets, and files."},
	}
}
