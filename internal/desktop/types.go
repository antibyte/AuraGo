package desktop

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

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
	IconCatalog        IconCatalogInfo   `json:"icon_catalog"`
}

// IconCatalogInfo tells agents and generated apps which semantic icons are safe
// to use for Papirus-first desktop surfaces.
type IconCatalogInfo struct {
	Theme              string            `json:"theme"`
	DefaultTheme       string            `json:"default_theme"`
	Preferred          []string          `json:"preferred"`
	Aliases            map[string]string `json:"aliases"`
	LegacySpritePrefix string            `json:"legacy_sprite_prefix"`
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
		{Key: "appearance.wallpaper", Default: "aurora", Values: []string{"aurora", "midnight", "slate", "ember", "forest", "alpine_dawn", "city_rain", "ocean_cliff", "aurora_glass", "nebula_flow", "paper_waves"}},
		{Key: "appearance.accent", Default: "teal", Values: []string{"teal", "orange", "blue", "violet", "green"}},
		{Key: "appearance.density", Default: "comfortable", Values: []string{"comfortable", "compact"}},
		{Key: "appearance.icon_theme", Default: "papirus", Values: []string{"papirus", "aurago"}},
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

var desktopPreferredIconNames = []string{
	"apps",
	"archive",
	"audio",
	"browser",
	"calendar",
	"calculator",
	"code",
	"css",
	"database",
	"desktop",
	"documents",
	"downloads",
	"editor",
	"folder",
	"go",
	"html",
	"image",
	"javascript",
	"json",
	"markdown",
	"network",
	"notes",
	"pdf",
	"python",
	"settings",
	"spreadsheet",
	"terminal",
	"text",
	"trash",
	"video",
	"weather",
	"xml",
	"yaml",
}

var desktopIconAliases = map[string]string{
	"agent_chat":   "apps",
	"binary":       "code",
	"calc":         "calculator",
	"calculator":   "calculator",
	"cloud":        "network",
	"csv":          "spreadsheet",
	"edit":         "editor",
	"executable":   "code",
	"file":         "text",
	"music":        "audio",
	"music-player": "audio",
	"note":         "notes",
	"pictures":     "image",
	"presentation": "documents",
	"search":       "folder",
	"sparkles":     "apps",
	"tasks":        "notes",
	"todo":         "notes",
	"widgets":      "apps",
}

var desktopIconTokenPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// DesktopIconCatalog returns a copy of the public icon catalog for generated apps.
func DesktopIconCatalog(settings map[string]string) IconCatalogInfo {
	defaultTheme := DesktopSettingDefaults()["appearance.icon_theme"]
	theme := defaultTheme
	if settings != nil {
		if value := settings["appearance.icon_theme"]; value != "" {
			if err := validateDesktopSetting("appearance.icon_theme", value); err == nil {
				theme = value
			}
		}
	}
	aliases := make(map[string]string, len(desktopIconAliases))
	for key, value := range desktopIconAliases {
		aliases[key] = value
	}
	return IconCatalogInfo{
		Theme:              theme,
		DefaultTheme:       defaultTheme,
		Preferred:          append([]string(nil), desktopPreferredIconNames...),
		Aliases:            aliases,
		LegacySpritePrefix: "sprite:",
	}
}

// NormalizeDesktopIconName canonicalizes generated app/widget icon names against
// the semantic Papirus catalog while preserving explicit legacy sprite icons.
func NormalizeDesktopIconName(raw, label string) (string, error) {
	icon := strings.ToLower(strings.TrimSpace(raw))
	if icon == "" {
		return "", fmt.Errorf("%s icon is required", label)
	}
	icon = strings.ReplaceAll(icon, " ", "_")
	if strings.HasPrefix(icon, "papirus:") {
		icon = strings.TrimPrefix(icon, "papirus:")
	}
	if strings.HasPrefix(icon, "sprite:") {
		spriteName := strings.TrimPrefix(icon, "sprite:")
		if !desktopIconTokenPattern.MatchString(spriteName) {
			return "", fmt.Errorf("%s icon must use icon_catalog.preferred, icon_catalog.aliases, or sprite:<name>", label)
		}
		return "sprite:" + spriteName, nil
	}
	if alias, ok := desktopIconAliases[icon]; ok {
		icon = alias
	}
	for _, preferred := range desktopPreferredIconNames {
		if icon == preferred {
			return icon, nil
		}
	}
	return "", fmt.Errorf("%s icon must use icon_catalog.preferred, icon_catalog.aliases, or sprite:<name>", label)
}

// InferDesktopIconName chooses the first catalog icon mentioned by an app or
// widget identity. It keeps agent-generated desktop items usable when the icon
// field is omitted while still returning only catalog-approved names.
func InferDesktopIconName(candidates ...string) string {
	for _, raw := range candidates {
		for _, candidate := range desktopIconInferenceCandidates(raw) {
			if icon, err := NormalizeDesktopIconName(candidate, "desktop"); err == nil {
				return icon
			}
		}
	}
	return "apps"
}

func desktopIconInferenceCandidates(raw string) []string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return nil
	}
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		normalized = normalized[idx+1:]
	}
	candidates := []string{strings.ReplaceAll(normalized, " ", "_")}
	var token strings.Builder
	flush := func() {
		if token.Len() == 0 {
			return
		}
		candidates = append(candidates, token.String())
		token.Reset()
	}
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			token.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return candidates
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
		{ID: "calculator", Name: "Calculator", Version: "1.0.0", Icon: "calculator", Entry: "builtin://calculator", Runtime: BuiltinRuntime, Description: "Scientific calculator with standard and advanced modes."},
		{ID: "todo", Name: "Todo", Version: "1.0.0", Icon: "notes", Entry: "builtin://todo", Runtime: BuiltinRuntime, Description: "Task management connected to the backend planner."},
		{ID: "music-player", Name: "Music Player", Version: "1.0.0", Icon: "audio", Entry: "builtin://music-player", Runtime: BuiltinRuntime, Description: "Winamp-style music player for workspace audio files."},
		{ID: "agent-chat", Name: "Agent Chat", Version: "1.0.0", Icon: "sparkles", Entry: "builtin://agent-chat", Runtime: BuiltinRuntime, Description: "Ask AuraGo to create apps, widgets, and files."},
	}
}
