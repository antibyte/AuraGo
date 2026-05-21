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
			ID:          "olivetin",
			Name:        "OliveTin",
			Description: "Web UI for running predefined shell automation actions.",
			Image:       "ghcr.io/olivetin/olivetin:latest",
			Icon:        "terminal",
			LogoSlug:    "olivetin",
			LogoURL:     logoURL("olivetin"),
			PrimaryPort: PortSpec{ContainerPort: 1337, Protocol: "tcp"},
			Volumes: []VolumeTemplate{
				{NameSuffix: "config", ContainerPath: "/config"},
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
	}
}

const oliveTinDefaultConfig = `actions:
  - title: "Hello world!"
    shell: echo 'Hello World!'
`

func logoURL(slug string) string {
	return fmt.Sprintf("%s/%s.png", dashboardIconPNGBase, slug)
}
