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
	Enabled                  bool
	ReadOnly                 bool
	AllowAgentControl        bool
	AllowGeneratedApps       bool
	AllowPythonJobs          bool
	WorkspaceDir             string
	DockerHost               string
	DBPath                   string
	DataDir                  string
	DocumentDir              string
	MediaRegistryPath        string
	ImageGalleryPath         string
	MaxFileSizeMB            int
	ControlLevel             string
	MaxWSClients             int
	RemoteMaxSessionMinutes  int
	RemoteIdleTimeoutMinutes int
	CodeStudio               CodeStudioConfig
	OpenSCAD                 OpenSCADConfig
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

// OpenSCADConfig controls the lazy OpenSCAD compiler container.
type OpenSCADConfig struct {
	Enabled                 bool
	Image                   string
	AutoStart               bool
	AutoStopMinutes         int
	MaxMemoryMB             int
	MaxCPUCores             int
	MaxConcurrentJobs       int
	GeometryBackend         string
	DefaultExports          []string
	MaxSourceKB             int
	MaxOutputMB             int
	RenderTimeoutSeconds    int
	MaxRenderTimeoutSeconds int
	JobRetentionDays        int
}

// WorkspaceInfo is the public workspace state returned to the browser.
type WorkspaceInfo struct {
	Root        string   `json:"root"`
	Directories []string `json:"directories"`
	MaxFileSize int64    `json:"max_file_size"`
}

// ProviderOption is a safe, secret-free LLM provider choice for desktop UI.
type ProviderOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Model string `json:"model"`
}

// PetManifest describes one desktop pet available in the workspace.
type PetManifest struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Subcategory string `json:"subcategory,omitempty"`
	Spritesheet string `json:"spritesheet,omitempty"`
	Builtin     bool   `json:"builtin"`
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
	AllWidgets         []Widget          `json:"all_widgets"`
	Settings           map[string]string `json:"settings"`
	Providers          []ProviderOption  `json:"providers,omitempty"`
	IconCatalog        IconCatalogInfo   `json:"icon_catalog"`
	Pets               []PetManifest     `json:"pets"`
	ActivePetID        string            `json:"active_pet_id,omitempty"`
}

// IconCatalogInfo tells agents and generated apps which semantic icons are safe
// to use for Papirus-first desktop surfaces.
type IconCatalogInfo struct {
	Theme              string              `json:"theme"`
	DefaultTheme       string              `json:"default_theme"`
	Preferred          []string            `json:"preferred"`
	Categories         map[string][]string `json:"categories"`
	Aliases            map[string]string   `json:"aliases"`
	LegacySpritePrefix string              `json:"legacy_sprite_prefix"`
}

// FileEntry describes one file or directory in the desktop workspace.
type FileEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	Modified  time.Time `json:"modified"` // UI compatibility field
	WebPath   string    `json:"web_path,omitempty"`
	MediaKind string    `json:"media_kind,omitempty"`
	MIMEType  string    `json:"mime_type,omitempty"`
	Mount     string    `json:"mount,omitempty"`
	Mode      string    `json:"mode,omitempty"`
	Created   time.Time `json:"created,omitempty"`
}

// AppManifest describes a browser-side desktop application.
type AppManifest struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Icon         string            `json:"icon"`
	Entry        string            `json:"entry"`
	Runtime      string            `json:"runtime,omitempty"`
	Description  string            `json:"description,omitempty"`
	Permissions  []string          `json:"permissions,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Builtin      bool              `json:"builtin"`
	Deletable    bool              `json:"deletable"`
	Internal     bool              `json:"internal,omitempty"`
	DockVisible  bool              `json:"dock_visible"`
	StartVisible bool              `json:"start_visible"`
	Health       string            `json:"health,omitempty"`
	HealthReason string            `json:"health_reason,omitempty"`
	EntryPath    string            `json:"entry_path,omitempty"`
	Integrity    *IntegrityData    `json:"integrity,omitempty"`
	CreatedAt    time.Time         `json:"created_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty"`
}

// IntegrityData records signed file hashes for generated desktop assets.
type IntegrityData struct {
	Hashes    map[string]string   `json:"hashes,omitempty"`
	Signature *IntegritySignature `json:"signature,omitempty"`
}

// IntegritySignature stores the local AuraGo signature over IntegrityData.Hashes.
type IntegritySignature struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
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
	ID           string                 `json:"id"`
	AppID        string                 `json:"app_id,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Title        string                 `json:"title"`
	Icon         string                 `json:"icon,omitempty"`
	Entry        string                 `json:"entry,omitempty"`
	Runtime      string                 `json:"runtime,omitempty"`
	Permissions  []string               `json:"permissions,omitempty"`
	X            int                    `json:"x"`
	Y            int                    `json:"y"`
	W            int                    `json:"w"`
	H            int                    `json:"h"`
	Visible      bool                   `json:"visible"`
	Builtin      bool                   `json:"builtin"`
	Health       string                 `json:"health,omitempty"`
	HealthReason string                 `json:"health_reason,omitempty"`
	EntryPath    string                 `json:"entry_path,omitempty"`
	Integrity    *IntegrityData         `json:"integrity,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at,omitempty"`
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
		{Key: "appearance.wallpaper", Default: "groupshoot", Values: []string{"groupshoot", "aurora", "midnight", "slate", "ember", "forest", "alpine_dawn", "city_rain", "ocean_cliff", "aurora_glass", "nebula_flow", "paper_waves"}},
		{Key: "appearance.theme", Default: "standard", Values: []string{"standard", "fruity"}},
		{Key: "appearance.accent", Default: "teal", Values: []string{"teal", "orange", "blue", "violet", "green"}},
		{Key: "appearance.density", Default: "comfortable", Values: []string{"comfortable", "compact"}},
		{Key: "appearance.icon_theme", Default: "papirus", Values: []string{"papirus", "whitesur"}},
		{Key: "appearance.fruity_mode", Default: "light", Values: []string{"light", "dark"}},
		{Key: "desktop.icon_size", Default: "medium", Values: []string{"small", "medium", "large"}},
		{Key: "desktop.show_widgets", Default: "true", Values: []string{"true", "false"}},
		{Key: "windows.animations", Default: "true", Values: []string{"true", "false"}},
		{Key: "windows.default_size", Default: "balanced", Values: []string{"compact", "balanced", "large"}},
		{Key: "files.confirm_delete", Default: "true", Values: []string{"true", "false"}},
		{Key: "files.default_folder", Default: "Documents", Values: []string{"Desktop", "Documents", "Downloads", "Pictures", "Shared"}},
		{Key: "agent.show_chat_button", Default: "true", Values: []string{"true", "false"}},
		{Key: "agent.provider", Default: ""},
		{Key: "pet.enabled", Default: "true", Values: []string{"true", "false"}},
		{Key: "pet.active_id", Default: "openpets-default"},
		{Key: "pet.scale", Default: "1.0"},
		{Key: "pet.position_x", Default: "24"},
		{Key: "pet.position_y", Default: "24"},
		{Key: "pet.always_on_top", Default: "false", Values: []string{"true", "false"}},
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
	"agent-chat",
	"apps",
	"apps-symbolic",
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
	"code-studio",
	"commandcode",
	"galaxa-deluxe",
	"copy",
	"css",
	"database",
	"desktop",
	"desktop-symbolic",
	"documents",
	"dozzle",
	"download",
	"downloads",
	"editor",
	"eye",
	"file-plus",
	"folder",
	"folder-open",
	"folder-plus",
	"folder-symbolic",
	"forms",
	"gallery",
	"gallery-action-delete",
	"gallery-action-download",
	"gallery-action-edit",
	"gallery-action-preview",
	"globe",
	"go",
	"grid",
	"heart",
	"help",
	"home",
	"html",
	"image",
	"info",
	"info-symbolic",
	"javascript",
	"json",
	"key",
	"launchpad",
	"list",
	"looper",
	"mail",
	"markdown",
	"map",
	"maximize",
	"minus",
	"monitor",
	"monitor-symbolic",
	"n8n",
	"network",
	"node-red",
	"notes",
	"olivetin",
	"open-webui",
	"openscad",
	"package",
	"pdf",
	"phone",
	"pixel",
	"printer",
	"python",
	"nasscad",
	"quakejs",
	"radio",
	"redo",
	"refresh",
	"romm",
	"run",
	"save",
	"scissors",
	"search",
	"server",
	"settings",
	"settings-symbolic",
	"sort",
	"software-store",
	"spreadsheet",
	"stop",
	"teevee",
	"terminal",
	"termix",
	"text",
	"trash",
	"trash-empty",
	"trash-full",
	"tools",
	"undo",
	"upload",
	"users",
	"video",
	"weather",
	"workflow",
	"writer",
	"x",
	"xml",
	"yaml",
	"zipper",
	"zoom-in",
	"zoom-out",
	"zoom-reset",
}

var desktopIconAliases = map[string]string{
	"agent":            "agent-chat",
	"agent_chat":       "agent-chat",
	"app-store":        "software-store",
	"arcade":           "run",
	"arrow-left":       "chevron-left",
	"arrow-right":      "chevron-right",
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
	"file-zip":         "zipper",
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
	"game":             "run",
	"games":            "run",
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
	"node_red":         "node-red",
	"note":             "notes",
	"open_webui":       "open-webui",
	"openwebui":        "open-webui",
	"openscad":         "openscad",
	"open_scad":        "openscad",
	"password":         "key",
	"paint":            "pixel",
	"pictures":         "image",
	"phone":            "phone",
	"photo-editor":     "pixel",
	"player":           "audio-player",
	"presentation":     "documents",
	"print":            "printer",
	"printer":          "printer",
	"quick-launch":     "launchpad",
	"radio":            "radio",
	"rom-manager":      "romm",
	"run":              "run",
	"screen":           "monitor",
	"share":            "globe",
	"sparkles":         "apps",
	"space":            "run",
	"space-invaders":   "run",
	"stats":            "analytics",
	"store":            "software-store",
	"support":          "help",
	"tasks":            "notes",
	"teevee":           "teevee",
	"television":       "teevee",
	"todo":             "notes",
	"tool":             "tools",
	"toolbox":          "tools",
	"tools":            "tools",
	"trash":            "trash-empty",
	"utilities":        "tools",
	"user-trash":       "trash-empty",
	"people":           "users",
	"tv":               "teevee",
	"weather":          "weather",
	"widgets":          "apps",
	"workflow":         "workflow",
	"workflows":        "workflow",
	"word":             "writer",
	"word-processor":   "writer",
	"writer":           "writer",
	"zip":              "zipper",
}

var desktopIconCategories = map[string][]string{
	"games":        {"run", "video", "apps", "terminal", "monitor", "heart"},
	"office":       {"writer", "spreadsheet", "calendar", "documents", "printer", "mail"},
	"productivity": {"notes", "check-square", "workflow", "calendar", "clipboard", "search"},
	"tools":        {"tools", "settings", "terminal", "code", "openscad", "database", "network", "zipper"},
	"media":        {"gallery", "pixel", "image", "video", "teevee", "radio", "audio", "audio-player", "camera"},
	"internet":     {"browser", "globe", "cloud", "mail", "network", "download"},
	"system":       {"monitor", "server", "settings", "backup", "key", "software-store", "trash-empty"},
	"documents":    {"documents", "text", "markdown", "pdf", "html", "archive", "zipper"},
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
	categories := make(map[string][]string, len(desktopIconCategories))
	for key, value := range desktopIconCategories {
		categories[key] = append([]string(nil), value...)
	}
	return IconCatalogInfo{
		Theme:              theme,
		DefaultTheme:       defaultTheme,
		Preferred:          append([]string(nil), desktopPreferredIconNames...),
		Categories:         categories,
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
	return []string{"Desktop", "Documents", "Downloads", "Apps", "Widgets", "Pets", "Data", "Pictures", "Trash", "Shared"}
}

// BuiltinApps returns the first-party applications always available in the shell.
func BuiltinApps() []AppManifest {
	apps := []AppManifest{
		{ID: "files", Name: "Files", Version: "1.0.0", Icon: "folder", Entry: "builtin://files", Runtime: BuiltinRuntime, Description: "Browse and manage desktop workspace files."},
		{ID: "editor", Name: "Editor", Version: "1.0.0", Icon: "edit", Entry: "builtin://editor", Runtime: BuiltinRuntime, Description: "Edit workspace text files."},
		{ID: "writer", Name: "Writer", Version: "1.0.0", Icon: "writer", Entry: "builtin://writer", Runtime: BuiltinRuntime, Description: "Create and edit basic word-processing documents.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "sheets", Name: "Sheets", Version: "1.0.0", Icon: "spreadsheet", Entry: "builtin://sheets", Runtime: BuiltinRuntime, Description: "Create and edit basic spreadsheets.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "settings", Name: "Settings", Version: "1.0.0", Icon: "settings", Entry: "builtin://settings", Runtime: BuiltinRuntime, Description: "Inspect virtual desktop settings."},
		{ID: "calendar", Name: "Calendar", Version: "1.0.0", Icon: "calendar", Entry: "builtin://calendar", Runtime: BuiltinRuntime, Description: "Local calendar surface for the desktop."},
		{ID: "calculator", Name: "Calculator", Version: "1.0.0", Icon: "calculator", Entry: "builtin://calculator", Runtime: BuiltinRuntime, Description: "Scientific calculator with standard and advanced modes."},
		{ID: "todo", Name: "Todo", Version: "1.0.0", Icon: "notes", Entry: "builtin://todo", Runtime: BuiltinRuntime, Description: "Task management connected to the backend planner."},
		{ID: "gallery", Name: "Gallery", Version: "1.0.0", Icon: "gallery", Entry: "builtin://gallery", Runtime: BuiltinRuntime, Description: "Browse AuraGo photos and videos."},
		{ID: "music-player", Name: "Music Player", Version: "1.0.0", Icon: "audio-player", Entry: "builtin://music-player", Runtime: BuiltinRuntime, Description: "Winamp-style music player for workspace audio files."},
		{ID: "radio", Name: "Radio", Version: "1.0.0", Icon: "radio", Entry: "builtin://radio", Runtime: BuiltinRuntime, Description: "Stream popular internet radio stations by category and search."},
		{ID: "teevee", Name: "TeeVee", Version: "1.0.0", Icon: "teevee", Entry: "builtin://teevee", Runtime: BuiltinRuntime, Description: "Watch public IPTV channels from iptv-org with German-first filtering and global search."},
		{ID: "agent-chat", Name: "Agent Chat", Version: "1.0.0", Icon: "agent-chat", Entry: "builtin://agent-chat", Runtime: BuiltinRuntime, Description: "Ask AuraGo to create apps, widgets, and files."},
		{ID: "quick-connect", Name: "Quick Connect", Version: "1.0.0", Icon: "terminal", Entry: "builtin://quick-connect", Runtime: BuiltinRuntime, Description: "Connect to SSH and VNC servers with an interactive terminal or remote desktop viewer."},
		{ID: "code-studio", Name: "Code Studio", Version: "1.0.0", Icon: "code-studio", Entry: "builtin://code-studio", Runtime: BuiltinRuntime, Description: "Full-featured coding IDE with file browser, editor, and terminal.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "launchpad", Name: "Launchpad", Version: "1.0.0", Icon: "launchpad", Entry: "builtin://launchpad", Runtime: BuiltinRuntime, Description: "Quick-access launcher for local and remote web links."},
		{ID: "software-store", Name: "Software Store", Version: "1.0.0", Icon: "software-store", Entry: "builtin://software-store", Runtime: BuiltinRuntime, Description: "Install allowlisted Docker web apps on the virtual desktop."},
		{ID: "looper", Name: "Looper", Version: "1.0.0", Icon: "looper", Entry: "builtin://looper", Runtime: BuiltinRuntime, Description: "Iterative agent loop with prepare, plan, action, test and exit condition.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "cheater", Name: "Cheater", Version: "1.0.0", Icon: "cheater", Entry: "builtin://cheater", Runtime: BuiltinRuntime, Description: "Create, search, and maintain cheat sheets for missions and operator workflows.", Permissions: []string{"notifications"}, Metadata: map[string]string{"logo_path": "/img/chat-ui-icons/cheater.svg"}},
		{ID: "camera", Name: "Camera", Version: "1.0.0", Icon: "camera", Entry: "builtin://camera", Runtime: BuiltinRuntime, Description: "Capture photos with your camera and save or analyze them.", Permissions: []string{"files:write", "notifications"}},
		{ID: "zipper", Name: "Zipper", Version: "1.0.0", Icon: "zipper", Entry: "builtin://zipper", Runtime: BuiltinRuntime, Description: "ZIP archive manager — browse, extract, and create archives.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "pixel", Name: "Pixel", Version: "1.0.0", Icon: "pixel", Entry: "builtin://pixel", Runtime: BuiltinRuntime, Description: "AI-powered image editor — create, edit, and enhance images.", Permissions: []string{"files:read", "files:write", "notifications"}},
		{ID: "people", Name: "People", Version: "1.0.0", Icon: "users", Entry: "builtin://people", Runtime: BuiltinRuntime, Description: "Address book with knowledge graph integration and birthdays."},
		{ID: "galaxa-deluxe", Name: "Galaxa Deluxe", Version: "1.0.0", Icon: "galaxa-deluxe", Entry: "builtin://galaxa-deluxe", Runtime: BuiltinRuntime, Description: "Classic arcade space shooter — destroy enemy formations and beat the high score!"},
		{ID: "chess", Name: "Chess", Version: "1.0.0", Icon: "run", Entry: "builtin://chess", Runtime: BuiltinRuntime, Description: "Play chess against Stockfish or the AuraGo agent.", Permissions: []string{"notifications"}},
		{ID: "mission-control", Name: "Mission Control", Version: "1.0.0", Icon: "workflow", Entry: "builtin://mission-control", Runtime: BuiltinRuntime, Description: "Create, plan, and manage agent missions with triggers and schedules.", Permissions: []string{"notifications"}},
		{ID: "homepage-studio", Name: "Homepage Studio", Version: "1.0.0", Icon: "globe", Entry: "builtin://homepage-studio", Runtime: BuiltinRuntime, Description: "AI-powered website builder with live preview.", Permissions: []string{"notifications"}, Metadata: map[string]string{"open_maximized": "true"}},
		{ID: "openscad", Name: "OpenSCAD", Version: "1.0.0", Icon: "openscad", Entry: "builtin://openscad", Runtime: BuiltinRuntime, Description: "Script-based parametric CAD compiler with preview, STL export, and downloadable artifacts.", Permissions: []string{"files:read", "files:write", "notifications"}, Metadata: map[string]string{"open_maximized": "true"}},
		{ID: "nasscad", Name: "NASSCAD", Version: "4.2.7", Icon: "nasscad", Entry: "builtin://nasscad", Runtime: BuiltinRuntime, Description: "Offline browser-based 3D parametric CAD bundled locally — model parts, run booleans, and export STL, OBJ, or 3MF.", Metadata: map[string]string{"open_maximized": "true", "workspace_entry": "Apps/nasscad/index.html"}},
		{ID: "viewer", Name: "Viewer", Version: "1.0.0", Icon: "eye", Entry: "builtin://viewer", Runtime: BuiltinRuntime, Description: "Read-only viewer for documents, spreadsheets, PDFs and markdown.", Permissions: []string{"files:read"}, Internal: true},
		{ID: "pet-picker", Name: "Pet Picker", Version: "1.0.0", Icon: "heart", Entry: "builtin://pet-picker", Runtime: BuiltinRuntime, Description: "Choose and manage your desktop pet companions."},
	}
	for i := range apps {
		apps[i].Builtin = true
		apps[i].Deletable = false
		apps[i].DockVisible = true
		apps[i].StartVisible = true
		if apps[i].ID == "viewer" || apps[i].ID == "openscad" {
			apps[i].DockVisible = false
			apps[i].StartVisible = false
		}
	}
	return apps
}
