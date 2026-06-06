package desktopstore

import "fmt"

const dashboardIconPNGBase = "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png"

// DefaultCatalog returns the fixed allowlist of Docker web apps that the store
// is allowed to install.
func DefaultCatalog() []CatalogEntry {
	return []CatalogEntry{
		{
			ID:          "homarr",
			Name:        "Homarr",
			Description: "Dashboard for home-lab services and quick links.",
			Image:       "ghcr.io/homarr-labs/homarr:latest",
			Icon:        "home",
			LogoSlug:    "homarr",
			LogoURL:     logoURL("homarr"),
			PrimaryPort: PortSpec{ContainerPort: 7575, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "appdata", ContainerPath: "/appdata"},
			},
			GeneratedSecrets: []GeneratedSecret{
				{Key: "secret_encryption_key", Env: "SECRET_ENCRYPTION_KEY", Label: "Encryption key"},
			},
		},
		{
			ID:          "n8n",
			Name:        "n8n",
			Description: "Workflow automation with integrations, triggers, and visual flows.",
			Image:       "ghcr.io/n8n-io/n8n:latest",
			Icon:        "workflow",
			LogoSlug:    "n8n",
			LogoURL:     logoURL("n8n"),
			PrimaryPort: PortSpec{ContainerPort: 5678, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "data", ContainerPath: "/home/node/.n8n"},
			},
			Env: []string{
				"TZ=UTC",
				"N8N_SECURE_COOKIE=false",
			},
		},
		{
			ID:          "node-red",
			Name:        "Node-RED",
			Description: "Low-code automation flows for devices, APIs, and services.",
			Image:       "ghcr.io/node-red/node-red:latest",
			Icon:        "workflow",
			LogoSlug:    "node-red",
			LogoURL:     logoURL("node-red"),
			PrimaryPort: PortSpec{ContainerPort: 1880, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "data", ContainerPath: "/data"},
			},
		},
		{
			ID:          "open-webui",
			Name:        "Open WebUI",
			Description: "Self-hosted chat interface for local and remote LLM providers.",
			Image:       "ghcr.io/open-webui/open-webui:main",
			Icon:        "chat",
			LogoSlug:    "open-webui",
			LogoURL:     logoURL("open-webui"),
			PrimaryPort: PortSpec{ContainerPort: 8080, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "data", ContainerPath: "/app/backend/data"},
			},
		},
		{
			ID:          "bytestash",
			Name:        "ByteStash",
			Description: "Self-hosted snippet manager for storing and searching code.",
			Image:       "ghcr.io/jordan-dalby/bytestash:latest",
			Icon:        "code",
			LogoSlug:    "bytestash",
			LogoURL:     logoURL("bytestash"),
			PrimaryPort: PortSpec{ContainerPort: 5000, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "snippets", ContainerPath: "/data/snippets"},
			},
			Env: []string{
				"BASE_PATH=",
				"TOKEN_EXPIRY=24h",
				"ALLOW_NEW_ACCOUNTS=true",
				"DEBUG=false",
				"DISABLE_ACCOUNTS=false",
				"DISABLE_INTERNAL_ACCOUNTS=false",
				"OIDC_ENABLED=false",
			},
			GeneratedSecrets: []GeneratedSecret{
				{Key: "jwt_secret", Env: "JWT_SECRET", Label: "JWT secret"},
			},
		},
		{
			ID:          "it-tools",
			Name:        "IT Tools",
			Description: "Collection of handy browser-based tools for developers and IT work.",
			Image:       "ghcr.io/corentinth/it-tools:latest",
			Icon:        "tools",
			LogoSlug:    "it-tools",
			LogoURL:     logoURL("it-tools"),
			PrimaryPort: PortSpec{ContainerPort: 80, Protocol: "tcp"},
		},
		{
			ID:          "filebrowser-quantum",
			Name:        "FileBrowser Quantum",
			Description: "Modern web file manager for browsing, uploading, and sharing files.",
			Image:       "ghcr.io/gtsteffaniak/filebrowser:stable",
			Icon:        "folder",
			LogoSlug:    "filebrowser-quantum",
			LogoURL:     logoURL("filebrowser-quantum"),
			PrimaryPort: PortSpec{ContainerPort: 80, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "files", ContainerPath: "/folder"},
				{NameSuffix: "data", ContainerPath: "/home/filebrowser/data"},
			},
		},
		{
			ID:          "olivetin",
			Name:        "OliveTin",
			Description: "Web UI for running predefined shell automation actions.",
			Image:       "ghcr.io/olivetin/olivetin:latest",
			Icon:        "terminal",
			LogoSlug:    "olivetin",
			LogoURL:     logoURL("olivetin"),
			PrimaryPort: PortSpec{ContainerPort: 1337, Protocol: "tcp"},
			WorkspaceBinds: []WorkspaceBind{
				{WorkspacePath: "Shared/OliveTin", ContainerPath: "/config"},
			},
			SeedFiles: []SeedFile{
				{Path: "/config/config.yaml", Content: oliveTinDefaultConfig},
			},
		},
		{
			ID:          "adguard-home",
			Name:        "AdGuard Home",
			Description: "Network-wide ad blocking and DNS filtering; v1 exposes only the setup web UI. Keep the admin web port on 3000 during setup.",
			Image:       "adguard/adguardhome",
			Icon:        "network",
			LogoSlug:    "adguard-home",
			LogoURL:     logoURL("adguard-home"),
			PrimaryPort: PortSpec{ContainerPort: 3000, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "work", ContainerPath: "/opt/adguardhome/work"},
				{NameSuffix: "conf", ContainerPath: "/opt/adguardhome/conf"},
			},
		},
		{
			ID:          "excalidraw",
			Name:        "Excalidraw",
			Description: "Collaborative sketching and diagramming canvas.",
			Image:       "excalidraw/excalidraw:latest",
			Icon:        "editor",
			LogoSlug:    "excalidraw",
			LogoURL:     logoURL("excalidraw"),
			PrimaryPort: PortSpec{ContainerPort: 80, Protocol: "tcp"},
		},
		{
			ID:          "uptime-kuma",
			Name:        "Uptime Kuma",
			Description: "Friendly uptime monitoring, alerting, and status pages.",
			Image:       "ghcr.io/louislam/uptime-kuma:2",
			Icon:        "monitor",
			LogoSlug:    "uptime-kuma",
			LogoURL:     logoURL("uptime-kuma"),
			PrimaryPort: PortSpec{ContainerPort: 3001, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "data", ContainerPath: "/app/data"},
			},
			Env: []string{
				"UPTIME_KUMA_DISABLE_FRAME_SAMEORIGIN=true",
			},
		},
		{
			ID:          "stirling-pdf",
			Name:        "Stirling PDF",
			Description: "Local PDF toolkit for merging, splitting, converting, signing, and OCR workflows.",
			Image:       "ghcr.io/stirling-tools/stirling-pdf:latest",
			Icon:        "pdf",
			LogoSlug:    "stirling-pdf",
			LogoURL:     logoURL("stirling-pdf"),
			PrimaryPort: PortSpec{ID: "web", Name: "Web UI", ContainerPort: 8080, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "configs", ContainerPath: "/configs"},
				{NameSuffix: "logs", ContainerPath: "/logs"},
				{NameSuffix: "pipeline", ContainerPath: "/pipeline"},
				{NameSuffix: "tessdata", ContainerPath: "/usr/share/tessdata"},
			},
		},
		{
			ID:          "quakejs-rootless",
			Name:        "QuakeJS Rootless",
			Description: "Browser-playable QuakeJS server packaged for rootless container deployments.",
			Image:       "docker.io/awakenedpower/quakejs-rootless:latest",
			Icon:        "run",
			LogoSlug:    "quakejs",
			LogoURL:     logoURL("quakejs"),
			PrimaryPort: PortSpec{ID: "web", Name: "Game", ContainerPort: 8080, Protocol: "tcp"},
			Metadata: map[string]string{
				"open_maximized": "true",
				"frame_features": "game",
			},
		},
		{
			ID:          "romm",
			Name:        "RomM",
			Description: "ROM library manager with metadata, browser players, saves, and collection management.",
			Image:       "ghcr.io/rommapp/romm:latest",
			Icon:        "run",
			LogoSlug:    "romm",
			LogoURL:     logoURL("romm"),
			PrimaryPort: PortSpec{ID: "web", Name: "Web UI", ContainerPort: 8080, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "resources", ContainerPath: "/romm/resources"},
				{NameSuffix: "redis-data", ContainerPath: "/redis-data"},
				{NameSuffix: "library", ContainerPath: "/romm/library"},
				{NameSuffix: "assets", ContainerPath: "/romm/assets"},
				{NameSuffix: "config", ContainerPath: "/romm/config"},
			},
			Env: []string{
				"TZ=Etc/UTC",
				"DB_HOST=aurago-store-romm-db",
				"DB_PORT=3306",
				"DB_NAME=romm",
				"DB_USER=romm-user",
				"ROMM_BASE_URL=${APP_URL}",
				"ROMM_PORT=8080",
			},
			GeneratedSecrets: []GeneratedSecret{
				{Key: "db_password", Env: "DB_PASSWD", Label: "Database password"},
				{Key: "db_root_password", Label: "Database root password"},
				{Key: "auth_secret_key", Env: "ROMM_AUTH_SECRET_KEY", Label: "Authentication secret"},
			},
			Metadata: map[string]string{
				"open_external": "true",
			},
			Companions: []CompanionTemplate{
				{
					ID:          "db",
					Name:        "RomM MariaDB",
					Image:       "ghcr.io/linuxserver/mariadb:latest",
					NetworkMode: "aurago-store-romm-net",
					Env: []string{
						"PUID=1000",
						"PGID=1000",
						"TZ=Etc/UTC",
						"MYSQL_DATABASE=romm",
						"MYSQL_USER=romm-user",
						"MYSQL_PASSWORD=${SECRET:desktop_store_romm_db_password}",
						"MYSQL_ROOT_PASSWORD=${SECRET:desktop_store_romm_db_root_password}",
					},
					Volumes: []VolumeTemplate{
						{NameSuffix: "db", ContainerPath: "/config"},
					},
				},
			},
		},
		{
			ID:          "beszel",
			Name:        "Beszel",
			Description: "Lightweight server monitoring hub with an optional local host agent.",
			Image:       "ghcr.io/henrygd/beszel/beszel:latest",
			Icon:        "monitor",
			LogoSlug:    "beszel",
			LogoURL:     logoURL("beszel"),
			PrimaryPort: PortSpec{ID: "hub", Name: "Hub", ContainerPort: 8090, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "data", ContainerPath: "/beszel_data"},
				{NameSuffix: "socket", ContainerPath: "/beszel_socket"},
			},
			Companions: []CompanionTemplate{
				{
					ID:          "agent",
					Name:        "Beszel Agent",
					Image:       "ghcr.io/henrygd/beszel/beszel-agent:latest",
					NetworkMode: "host",
					Env: []string{
						"LISTEN=/beszel_socket/beszel.sock",
						"HUB_URL=${APP_URL}",
						"KEY=${SECRET:desktop_store_beszel_agent_key}",
						"TOKEN=${SECRET:desktop_store_beszel_agent_token}",
					},
					Volumes: []VolumeTemplate{
						{NameSuffix: "socket", ContainerPath: "/beszel_socket"},
						{NameSuffix: "agent-data", ContainerPath: "/var/lib/beszel-agent"},
					},
					HostBinds: []HostBindTemplate{
						{HostPath: "/var/run/docker.sock", ContainerPath: "/var/run/docker.sock", ReadOnly: true},
					},
				},
			},
		},
		{
			ID:          "dozzle",
			Name:        "Dozzle",
			Description: "Real-time Docker log viewer for local containers.",
			Image:       "ghcr.io/amir20/dozzle:latest",
			Icon:        "terminal",
			LogoSlug:    "dozzle",
			LogoURL:     logoURL("dozzle"),
			PrimaryPort: PortSpec{ID: "web", Name: "Logs", ContainerPort: 8080, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "data", ContainerPath: "/data"},
			},
			HostBinds: []HostBindTemplate{
				{HostPath: "/var/run/docker.sock", ContainerPath: "/var/run/docker.sock", ReadOnly: true},
			},
		},
		{
			ID:          "code-server",
			Name:        "code-server",
			Description: "Browser-based VS Code development environment.",
			Image:       "ghcr.io/linuxserver/code-server:latest",
			Icon:        "code",
			LogoSlug:    "code-server",
			LogoURL:     logoURL("code-server"),
			PrimaryPort: PortSpec{ID: "web", Name: "IDE", ContainerPort: 8443, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "config", ContainerPath: "/config"},
			},
			Env: []string{
				"PUID=1000",
				"PGID=1000",
				"TZ=Etc/UTC",
			},
			GeneratedSecrets: []GeneratedSecret{
				{Key: "password", Env: "PASSWORD", Label: "Password", Expose: true},
			},
		},
		{
			ID:          "termix",
			Name:        "Termix",
			Description: "Self-hosted SSH and remote desktop management platform with RDP, VNC, and Telnet support.",
			Image:       "ghcr.io/lukegus/termix:latest",
			Icon:        "terminal",
			LogoSlug:    "termix",
			LogoURL:     logoURL("termix"),
			PrimaryPort: PortSpec{ID: "web", Name: "Web UI", ContainerPort: 8080, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "data", ContainerPath: "/app/data"},
			},
			Env: []string{
				"PORT=8080",
				"GUACD_HOST=aurago-store-termix-guacd",
				"GUACD_PORT=4822",
				"ENABLE_GUACAMOLE=true",
			},
			Metadata: map[string]string{
				"open_external": "true",
			},
			Companions: []CompanionTemplate{
				{
					ID:          "guacd",
					Name:        "Termix Guacamole (RDP/VNC)",
					Image:       "guacamole/guacd:1.6.0",
					NetworkMode: "aurago-store-termix-net",
				},
			},
		},
		{
			ID:          "commandcode",
			Name:        "CommandCode",
			Description: "Console-first development workspace with Command Code and full-stack toolchains preinstalled. Installation can take several minutes because AuraGo may build the image locally. Command Code requires login or an API key; browser auth shows a key you can paste into the terminal.",
			Image:       "ghcr.io/antibyte/aurago-commandcode:latest",
			Icon:        "terminal",
			LogoSlug:    "terminal",
			LogoURL:     logoURL("terminal"),
			PrimaryPort: PortSpec{ID: "web", Name: "Preview", ContainerPort: 80, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "home", ContainerPath: "/home/developer"},
			},
			WorkspaceBinds: []WorkspaceBind{
				{WorkspacePath: "Shared/CommandCode", ContainerPath: "/workspace"},
			},
			Metadata: map[string]string{
				"store_ui":         "terminal-preview",
				"terminal_enabled": "true",
				"preview_port_id":  "web",
				"open_maximized":   "true",
			},
		},
	}
}

const oliveTinDefaultConfig = `actions:
  - title: "Hello world!"
    shell: echo 'Hello World!'
`

func logoURL(slug string) string {
	return fmt.Sprintf("%s/%s.png", dashboardIconPNGBase, slug)
}
