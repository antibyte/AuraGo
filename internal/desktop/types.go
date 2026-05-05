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

	AuraDesktopRuntime      = "aura-desktop-sdk@1"
	BuiltinRuntime          = "builtin"
	WidgetTypeCustom        = "custom"
	ShortcutTargetApp       = "app"
	ShortcutTargetDirectory = "directory"
)

// Config describes the runtime settings needed by the desktop service.
type Config struct {
	Enabled            bool
	ReadOnly           bool
	AllowAgentControl  bool
	AllowGeneratedApps bool
	AllowPythonJobs    bool
	WorkspaceDir       string
	DockerHost         string
	DBPath             string
	DataDir            string
	DocumentDir        string
	MediaRegistryPath  string
	ImageGalleryPath   string
	MaxFileSizeMB      int
	ControlLevel       string
	MaxWSClients       int
	CodeStudio         CodeStudioConfig
}

// CodeStudioConfig controls the lazy Code Studio development container.
type CodeStudioConfig struct {
	Enabled         bool
	Image           string
	AutoStart       bool
	AutoStopMinutes int
	MaxMemoryMB     int
	MaxCPUCores     int
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
	Shortcuts          []Shortcut        `json:"shortcuts"`
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
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	WebPath   string    `json:"web_path,omitempty"`
	MediaKind string    `json:"media_kind,omitempty"`
	MIMEType  string    `json:"mime_type,omitempty"`
	Mount     string    `json:"mount,omitempty"`
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

// Shortcut describes one persistent icon pinned to the desktop surface.
type Shortcut struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id,omitempty"`
	Path       string    `json:"path,omitempty"`
	Name       string    `json:"name"`
	Icon       string    `json:"icon"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
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
		{Key: "appearance.theme", Default: "standard", Values: []string{"standard", "fruity"}},
		{Key: "appearance.accent", Default: "teal", Values: []string{"teal", "orange", "blue", "violet", "green"}},
		{Key: "appearance.density", Default: "comfortable", Values: []string{"comfortable", "compact"}},
		{Key: "appearance.icon_theme", Default: "papirus", Values: []string{"papirus", "whitesur"}},
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
	"analytics",
	"apps",
	"arrow-up",
	"archive",
	"audio",
	"audio-player",
	"backup",
	"book",
	"browser",
	"calendar",
	"calculator",
	"camera",
	"check-square",
	"chevron-down",
	"chevron-left",
	"chevron-right",
	"chevron-up",
	"chat",
	"clipboard",
	"cloud",
	"code",
	"copy",
	"css",
	"database",
	"desktop",
	"documents",
	"download",
	"downloads",
	"editor",
	"file-plus",
	"folder",
	"folder-open",
	"folder-plus",
	"forms",
	"globe",
	"go",
	"grid",
	"help",
	"home",
	"html",
	"image",
	"info",
	"javascript",
	"json",
	"key",
	"launchpad",
	"list",
	"mail",
	"markdown",
	"map",
	"monitor",
	"network",
	"notes",
	"package",
	"pdf",
	"phone",
	"printer",
	"python",
	"radio",
	"refresh",
	"run",
	"save",
	"scissors",
	"search",
	"server",
	"settings",
	"sort",
	"spreadsheet",
	"stop",
	"terminal",
	"text",
	"trash",
	"tools",
	"upload",
	"video",
	"weather",
	"workflow",
	"writer",
	"x",
	"xml",
	"yaml",
}

var desktopIconAliases = map[string]string{
	"agent_chat":       "chat",
	"automation":       "workflow",
	"backup":           "backup",
	"backups":          "backup",
	"binary":           "code",
	"book":             "book",
	"books":            "book",
	"camera":           "camera",
	"chart":            "analytics",
	"calc":             "calculator",
	"calculator":       "calculator",
	"cloud":            "cloud",
	"contact":          "forms",
	"contacts":         "forms",
	"csv":              "spreadsheet",
	"edit":             "editor",
	"email":            "mail",
	"executable":       "code",
	"execute":          "run",
	"file":             "text",
	"file-archive":     "archive",
	"file-audio":       "audio",
	"file-c":           "code",
	"file-cpp":         "code",
	"file-csharp":      "code",
	"file-csv":         "spreadsheet",
	"file-css":         "css",
	"file-doc":         "documents",
	"file-docker":      "package",
	"file-go":          "go",
	"file-html":        "html",
	"file-image":       "image",
	"file-java":        "code",
	"file-js":          "javascript",
	"file-json":        "json",
	"file-kotlin":      "code",
	"file-md":          "markdown",
	"file-pdf":         "pdf",
	"file-php":         "code",
	"file-ppt":         "documents",
	"file-py":          "python",
	"file-ruby":        "code",
	"file-shell":       "terminal",
	"file-sql":         "database",
	"file-swift":       "code",
	"file-text":        "text",
	"file-video":       "video",
	"file-xls":         "spreadsheet",
	"folder-api":       "network",
	"folder-assets":    "image",
	"folder-backups":   "backup",
	"folder-build":     "package",
	"folder-cmd":       "terminal",
	"folder-config":    "settings",
	"folder-data":      "database",
	"folder-db":        "database",
	"folder-deploy":    "server",
	"folder-desktop":   "desktop",
	"folder-docs":      "documents",
	"folder-documents": "documents",
	"folder-downloads": "downloads",
	"folder-git":       "workflow",
	"folder-github":    "workflow",
	"folder-images":    "image",
	"folder-internal":  "tools",
	"folder-lib":       "package",
	"folder-logs":      "text",
	"folder-media":     "video",
	"folder-music":     "audio",
	"folder-npm":       "package",
	"folder-pictures":  "image",
	"folder-pkg":       "package",
	"folder-public":    "globe",
	"folder-reports":   "analytics",
	"folder-scripts":   "terminal",
	"folder-secrets":   "key",
	"folder-src":       "code",
	"folder-temp":      "archive",
	"folder-templates": "forms",
	"folder-tests":     "tools",
	"folder-tools":     "tools",
	"folder-ui":        "desktop",
	"folder-videos":    "video",
	"folder-workflows": "workflow",
	"folder-workspace": "home",
	"forecast":         "weather",
	"form":             "forms",
	"forms":            "forms",
	"help":             "help",
	"internet":         "globe",
	"launchpad":        "launchpad",
	"launcher":         "launchpad",
	"library":          "book",
	"location":         "map",
	"mail":             "mail",
	"map":              "map",
	"maps":             "map",
	"mobile":           "phone",
	"music":            "audio",
	"music-player":     "audio-player",
	"network":          "network",
	"note":             "notes",
	"password":         "key",
	"pictures":         "image",
	"phone":            "phone",
	"player":           "audio-player",
	"presentation":     "documents",
	"print":            "printer",
	"printer":          "printer",
	"quick-launch":     "launchpad",
	"radio":            "radio",
	"run":              "run",
	"screen":           "monitor",
	"sparkles":         "apps",
	"stats":            "analytics",
	"support":          "help",
	"tasks":            "notes",
	"todo":             "notes",
	"tool":             "tools",
	"toolbox":          "tools",
	"tools":            "tools",
	"utilities":        "tools",
	"weather":          "weather",
	"widgets":          "apps",
	"workflow":         "workflow",
	"workflows":        "workflow",
	"word":             "writer",
	"word-processor":   "writer",
	"writer":           "writer",
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
// the semantic themed icon catalog while preserving explicit legacy sprite icons.
func NormalizeDesktopIconName(raw, label string) (string, error) {
	icon := strings.ToLower(strings.TrimSpace(raw))
	if icon == "" {
		return "", fmt.Errorf("%s icon is required", label)
	}
	icon = strings.ReplaceAll(icon, " ", "_")
	if strings.HasPrefix(icon, "papirus:") {
		icon = strings.TrimPrefix(icon, "papirus:")
	}
	if strings.HasPrefix(icon, "whitesur:") {
		icon = strings.TrimPrefix(icon, "whitesur:")
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
	return append(workspaceDirectories(), "Music", "Photos", "Videos", "AuraGo Documents")
}

func workspaceDirectories() []string {
	return []string{"Desktop", "Documents", "Downloads", "Apps", "Widgets", "Data", "Pictures", "Trash", "Shared"}
}

// BuiltinApps returns the first-party applications always available in the shell.
func BuiltinApps() []AppManifest {
	return []AppManifest{
		{ID: "files", Name: "Files", Version: "1.0.0", Icon: "folder", Entry: "builtin://files", Runtime: BuiltinRuntime, Description: "Browse and manage desktop workspace files."},
		{ID: "editor", Name: "Editor", Version: "1.0.0", Icon: "edit", Entry: "builtin://editor", Runtime: BuiltinRuntime, Description: "Edit workspace text files."},
		{ID: "writer", Name: "Writer", Version: "1.0.0", Icon: "writer", Entry: "builtin://writer", Runtime: BuiltinRuntime, Description: "Create and edit basic word-processing documents.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "sheets", Name: "Sheets", Version: "1.0.0", Icon: "spreadsheet", Entry: "builtin://sheets", Runtime: BuiltinRuntime, Description: "Create and edit basic spreadsheets.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "settings", Name: "Settings", Version: "1.0.0", Icon: "settings", Entry: "builtin://settings", Runtime: BuiltinRuntime, Description: "Inspect virtual desktop settings."},
		{ID: "calendar", Name: "Calendar", Version: "1.0.0", Icon: "calendar", Entry: "builtin://calendar", Runtime: BuiltinRuntime, Description: "Local calendar surface for the desktop."},
		{ID: "calculator", Name: "Calculator", Version: "1.0.0", Icon: "calculator", Entry: "builtin://calculator", Runtime: BuiltinRuntime, Description: "Scientific calculator with standard and advanced modes."},
		{ID: "todo", Name: "Todo", Version: "1.0.0", Icon: "notes", Entry: "builtin://todo", Runtime: BuiltinRuntime, Description: "Task management connected to the backend planner."},
		{ID: "gallery", Name: "Gallery", Version: "1.0.0", Icon: "image", Entry: "builtin://gallery", Runtime: BuiltinRuntime, Description: "Browse AuraGo photos and videos."},
		{ID: "music-player", Name: "Music Player", Version: "1.0.0", Icon: "audio-player", Entry: "builtin://music-player", Runtime: BuiltinRuntime, Description: "Winamp-style music player for workspace audio files."},
		{ID: "radio", Name: "Radio", Version: "1.0.0", Icon: "radio", Entry: "builtin://radio", Runtime: BuiltinRuntime, Description: "Stream popular internet radio stations by category and search."},
		{ID: "agent-chat", Name: "Agent Chat", Version: "1.0.0", Icon: "chat", Entry: "builtin://agent-chat", Runtime: BuiltinRuntime, Description: "Ask AuraGo to create apps, widgets, and files."},
		{ID: "quick-connect", Name: "Quick Connect", Version: "1.0.0", Icon: "terminal", Entry: "builtin://quick-connect", Runtime: BuiltinRuntime, Description: "Connect to SSH servers with an interactive terminal."},
		{ID: "code-studio", Name: "Code Studio", Version: "1.0.0", Icon: "code", Entry: "builtin://code-studio", Runtime: BuiltinRuntime, Description: "Full-featured coding IDE with file browser, editor, and terminal.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "launchpad", Name: "Launchpad", Version: "1.0.0", Icon: "launchpad", Entry: "builtin://launchpad", Runtime: BuiltinRuntime, Description: "Quick-access launcher for local and remote web links."},
	}
}
