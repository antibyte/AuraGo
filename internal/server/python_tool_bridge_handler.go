package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/tools"
)

type pythonToolBridgeGroup struct {
	Key   string   `json:"key"`   // config sidebar section key (for translated label)
	Icon  string   `json:"icon"`  // sidebar-ish icon (non-translated)
	Tools []string `json:"tools"` // native tool names to whitelist when this integration is selected
}

type pythonToolBridgeCatalogResponse struct {
	Status string                  `json:"status"`
	Count  int                     `json:"count"`
	Groups []pythonToolBridgeGroup `json:"groups"`
}

type pythonToolBridgeGroupDef struct {
	Key      string
	Icon     string
	ToolList []string
}

func pythonToolBridgeRecommendedGroups() []pythonToolBridgeGroupDef {
	// Curated allowlist for the UI: only integrations/tools that are generally
	// reasonable to call from Python skills via the internal tool bridge.
	//
	// NOTE: This does not enforce security. The actual enforcement still happens
	// in the tool-bridge handler via config.Tools.PythonToolBridge.AllowedTools.
	return []pythonToolBridgeGroupDef{
		{Key: "web_scraper", Icon: "🕷️", ToolList: []string{"web_scraper", "site_crawler"}},
		{Key: "proxmox", Icon: "🖥️", ToolList: []string{"proxmox"}},
		{Key: "docker", Icon: "🐳", ToolList: []string{"docker"}},
		{Key: "tailscale", Icon: "🔒", ToolList: []string{"tailscale"}},
		{Key: "ansible", Icon: "⚙️", ToolList: []string{"ansible"}},
		{Key: "fritzbox", Icon: "📡", ToolList: []string{"fritzbox_system", "fritzbox_network", "fritzbox_telephony", "fritzbox_smarthome", "fritzbox_storage", "fritzbox_tv"}},
		{Key: "github", Icon: "🐙", ToolList: []string{"github"}},
		{Key: "home_assistant", Icon: "🏠", ToolList: []string{"home_assistant"}},
		{Key: "sql_connections", Icon: "🔗", ToolList: []string{"sql_query"}},
		{Key: "mqtt", Icon: "📡", ToolList: []string{"mqtt_publish", "mqtt_subscribe", "mqtt_unsubscribe", "mqtt_get_messages"}},
		{Key: "email", Icon: "✉️", ToolList: []string{"fetch_email", "send_email", "list_email_accounts"}},
		{Key: "telegram", Icon: "📱", ToolList: []string{"send_telegram"}},
		{Key: "discord", Icon: "💬", ToolList: []string{"send_discord", "fetch_discord", "list_discord_channels"}},
		{Key: "webhooks", Icon: "🔗", ToolList: []string{"call_webhook", "manage_outgoing_webhooks"}},
		{Key: "netlify", Icon: "🔺", ToolList: []string{"netlify"}},
		{Key: "homepage", Icon: "🌐", ToolList: []string{"homepage"}},
		{Key: "s3", Icon: "🪣", ToolList: []string{"s3_storage"}},
		{Key: "koofr", Icon: "📦", ToolList: []string{"koofr"}},
		{Key: "onedrive", Icon: "☁️", ToolList: []string{"onedrive"}},
		{Key: "meshcentral", Icon: "🖥️", ToolList: []string{"meshcentral"}},
		{Key: "truenas", Icon: "💾", ToolList: []string{"truenas"}},
		{Key: "jellyfin", Icon: "🎬", ToolList: []string{"jellyfin"}},
		{Key: "media_conversion", Icon: "🎞️", ToolList: []string{"media_conversion"}},
	}
}

func pythonToolBridgeBuildCatalogGroups(available map[string]bool) []pythonToolBridgeGroup {
	defs := pythonToolBridgeRecommendedGroups()
	out := make([]pythonToolBridgeGroup, 0, len(defs))
	for _, def := range defs {
		var toolsOut []string
		for _, name := range def.ToolList {
			if available[name] {
				toolsOut = append(toolsOut, name)
			}
		}
		if len(toolsOut) == 0 {
			continue
		}
		sort.Strings(toolsOut)
		out = append(out, pythonToolBridgeGroup{
			Key:   def.Key,
			Icon:  def.Icon,
			Tools: toolsOut,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Key) < strings.ToLower(out[j].Key)
	})
	return out
}

// handlePythonToolBridgeTools returns a curated, translated-by-UI catalog of
// tool names that can be selected for tools.python_tool_bridge.allowed_tools.
// GET /api/python-tool-bridge/tools
func handlePythonToolBridgeTools(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		ff := mcpFeatureFlags(s)
		manifest := tools.NewManifest(cfg.Directories.ToolsDir)
		oaiTools := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, s.Logger)

		available := make(map[string]bool, len(oaiTools))
		for _, t := range oaiTools {
			if t.Function == nil || t.Function.Name == "" {
				continue
			}
			available[t.Function.Name] = true
		}

		groups := pythonToolBridgeBuildCatalogGroups(available)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pythonToolBridgeCatalogResponse{
			Status: "ok",
			Count:  len(groups),
			Groups: groups,
		})
	}
}
