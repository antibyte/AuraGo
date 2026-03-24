package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/meshcentral"
	"aurago/internal/security"
)

// handleRuntime returns the current runtime detection results and feature availability.
func handleRuntime(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		rt := s.Cfg.Runtime
		s.CfgMu.RUnlock()

		result := map[string]interface{}{
			"runtime":  rt,
			"features": config.ComputeFeatureAvailability(rt),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// injectFeatureAvailability adds _available and _reason fields into
// the raw config sections so the UI can gray out unavailable features.
func injectFeatureAvailability(rawCfg map[string]interface{}, rt config.Runtime) {
	avail := config.ComputeFeatureAvailability(rt)

	// Map feature keys to config section names
	mapping := map[string]string{
		"firewall":             "firewall",
		"docker":               "docker",
		"sandbox":              "sandbox",
		"sudo":                 "agent",
		"wol":                  "tools",
		"homepage_docker":      "homepage",
		"invasion_local":       "invasion_control",
		"chromecast_discovery": "chromecast",
	}

	for featureKey, fa := range avail {
		sectionName, ok := mapping[featureKey]
		if !ok {
			continue
		}

		// For "sudo" we inject into the agent section with a prefixed key
		if featureKey == "sudo" {
			if agentSec, ok := rawCfg["agent"].(map[string]interface{}); ok {
				agentSec["_sudo_available"] = fa.Available
				if fa.Reason != "" {
					agentSec["_sudo_reason"] = fa.Reason
				}
			}
			continue
		}

		// Use a sub-key for features that share a section (e.g. wol lives under tools.wol)
		if featureKey == "wol" {
			toolsSec, ok := rawCfg[sectionName].(map[string]interface{})
			if !ok {
				continue
			}
			wolSec, ok := toolsSec["wol"].(map[string]interface{})
			if ok {
				wolSec["_available"] = fa.Available
				if fa.Reason != "" {
					wolSec["_reason"] = fa.Reason
				}
			}
			continue
		}

		section, ok := rawCfg[sectionName].(map[string]interface{})
		if !ok {
			continue
		}
		section["_available"] = fa.Available
		if fa.Reason != "" {
			section["_reason"] = fa.Reason
		}
	}
}

// injectVaultIndicators adds masked placeholder values ("••••••••") into the
// raw config map for vault-only fields that have a stored secret. This ensures
// the UI shows that a value is configured even though it's not in the YAML.
func injectVaultIndicators(rawCfg map[string]interface{}, vault *security.Vault) {
	if vault == nil {
		return
	}
	for yamlPath, vaultKey := range vaultKeyMap {
		parts := strings.Split(yamlPath, ".")
		if vaultKey == "" {
			continue
		}
		val, err := vault.ReadSecret(vaultKey)
		if err != nil || val == "" {
			continue
		}
		// Navigate to the parent map, creating intermediate maps as needed
		m := rawCfg
		for i := 0; i < len(parts)-1; i++ {
			sub, ok := m[parts[i]].(map[string]interface{})
			if !ok {
				sub = make(map[string]interface{})
				m[parts[i]] = sub
			}
			m = sub
		}
		m[parts[len(parts)-1]] = "••••••••"
	}
}

func isVaultMappedPath(fullPath string) bool {
	_, ok := vaultKeyMap[fullPath]
	return ok
}

// getConfigSchema returns a JSON schema describing the config structure for the UI.
// It reflects the Config struct to produce field metadata (type, yaml key).
func handleGetConfigSchema(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		schema := buildSchema(reflect.TypeOf(*s.Cfg), "")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schema)
	}
}

// SchemaField describes a single config field for the UI renderer.
type SchemaField struct {
	Key       string        `json:"key"`
	YAMLKey   string        `json:"yaml_key"`
	Type      string        `json:"type"` // "string", "int", "float", "bool", "object", "array"
	Sensitive bool          `json:"sensitive,omitempty"`
	Children  []SchemaField `json:"children,omitempty"`
}

func buildSchema(t reflect.Type, prefix string) []SchemaField {
	var fields []SchemaField

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		yamlTag := f.Tag.Get("yaml")
		vaultTag := f.Tag.Get("vault")
		// Skip fields excluded from YAML unless they have a vault tag
		if yamlTag == "-" {
			if vaultTag == "" {
				continue
			}
			yamlTag = vaultTag // use vault key name as display key
		}
		// Skip fields explicitly excluded from JSON (hidden from UI)
		if f.Tag.Get("json") == "-" {
			continue
		}
		if yamlTag == "" {
			yamlTag = strings.ToLower(f.Name)
		}
		// Strip tag options
		if idx := strings.Index(yamlTag, ","); idx >= 0 {
			yamlTag = yamlTag[:idx]
		}

		fullKey := yamlTag
		if prefix != "" {
			fullKey = prefix + "." + yamlTag
		}

		sf := SchemaField{
			Key:     fullKey,
			YAMLKey: yamlTag,
		}

		ft := f.Type
		if ft.Kind() == reflect.Struct {
			sf.Type = "object"
			sf.Children = buildSchema(ft, fullKey)
		} else if ft.Kind() == reflect.Slice {
			sf.Type = "array"
		} else if ft.Kind() == reflect.Bool {
			sf.Type = "bool"
		} else if ft.Kind() == reflect.Int || ft.Kind() == reflect.Int64 || ft.Kind() == reflect.Int32 {
			sf.Type = "int"
		} else if ft.Kind() == reflect.Float64 || ft.Kind() == reflect.Float32 {
			sf.Type = "float"
		} else {
			sf.Type = "string"
		}

		// Mark sensitive fields. Besides explicit vault tags, also respect the
		// static vault path map so generic sections render fields like
		// rocketchat.auth_token or google_workspace.client_secret as secrets.
		if sensitiveKeys[yamlTag] || vaultTag != "" || isVaultMappedPath(fullKey) {
			sf.Sensitive = true
		}

		fields = append(fields, sf)
	}

	return fields
}

// handleRestart triggers an application restart by exiting with code 42
func handleRestart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.Logger.Info("[Config UI] Restart requested via Web UI")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "AuraGo wird neu gestartet...",
		})

		// Give the HTTP response time to flush before killing the process.
		go func() {
			time.Sleep(300 * time.Millisecond)
			os.Exit(42) // 42 → systemd Restart=on-failure / start.bat loop
		}()
	}
}

// handleOllamaModels proxies GET /api/tags on an Ollama host and
// returns the list of locally installed model names.
// Accepts an optional ?url= query parameter with the Ollama base URL.
// If omitted, falls back to the configured main LLM provider (must be ollama).
func handleOllamaModels(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		explicitURL := strings.TrimSpace(r.URL.Query().Get("url"))

		var ollamaHost string
		if explicitURL != "" {
			// Caller supplied a URL directly (e.g. from provider modal)
			ollamaHost = strings.TrimRight(explicitURL, "/")
			ollamaHost = strings.TrimSuffix(ollamaHost, "/v1")
			ollamaHost = strings.TrimRight(ollamaHost, "/")
		} else {
			// Fall back to the saved main LLM config
			s.CfgMu.RLock()
			provider := s.Cfg.LLM.ProviderType
			baseURL := s.Cfg.LLM.BaseURL
			s.CfgMu.RUnlock()

			if !strings.EqualFold(provider, "ollama") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"available": false,
					"reason":    "provider is not ollama",
				})
				return
			}
			ollamaHost = strings.TrimRight(baseURL, "/")
			ollamaHost = strings.TrimSuffix(ollamaHost, "/v1")
			ollamaHost = strings.TrimRight(ollamaHost, "/")
		}

		if ollamaHost == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    "no Ollama URL provided",
			})
			return
		}

		if !strings.HasPrefix(ollamaHost, "http://") && !strings.HasPrefix(ollamaHost, "https://") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    "invalid URL scheme (must be http:// or https://)",
			})
			return
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(ollamaHost + "/api/tags")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    "Failed to reach Ollama: " + err.Error(),
			})
			return
		}
		defer resp.Body.Close()

		var tagsResp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
			http.Error(w, "Failed to parse Ollama response: "+err.Error(), http.StatusBadGateway)
			return
		}

		names := make([]string, 0, len(tagsResp.Models))
		for _, m := range tagsResp.Models {
			names = append(names, m.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"models": names,
		})
	}
}

// handleOpenRouterModels fetches the public model list from the OpenRouter API
// and returns it as JSON. The frontend uses this to display a model browser.
// No API key is required — the endpoint is public.
func handleOpenRouterModels(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get("https://openrouter.ai/api/v1/models")
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    "Failed to reach OpenRouter: " + err.Error(),
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    fmt.Sprintf("OpenRouter returned HTTP %d", resp.StatusCode),
			})
			return
		}

		// Stream the response directly to reduce memory usage
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
	}
}

// handleMeshCentralTest attempts a login + WebSocket handshake against the MeshCentral server.
// Accepts an optional JSON body with fields {url, username, password}; any empty/omitted field
// falls back to the saved config value (password also falls back to the vault).
func handleMeshCentralTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		// Parse optional override values from request body.
		// Always attempt decode regardless of ContentLength (HTTP/2 may omit it).
		var body struct {
			URL        string `json:"url"`
			Username   string `json:"username"`
			Password   string `json:"password"`
			LoginToken string `json:"login_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Fall back to saved config
		s.CfgMu.RLock()
		url := body.URL
		if url == "" {
			url = s.Cfg.MeshCentral.URL
		}
		username := body.Username
		if username == "" {
			username = s.Cfg.MeshCentral.Username
		}
		password := body.Password
		if password == "" {
			password = s.Cfg.MeshCentral.Password
		}
		loginToken := body.LoginToken
		if loginToken == "" {
			loginToken = s.Cfg.MeshCentral.LoginToken
		}
		s.CfgMu.RUnlock()

		// Vault fallback for password / token
		if s.Vault != nil {
			if password == "" {
				if v, _ := s.Vault.ReadSecret("meshcentral_password"); v != "" {
					s.Logger.Info("[MeshCentral Test] Found password in vault")
					password = v
				}
			}
			if loginToken == "" {
				if v, _ := s.Vault.ReadSecret("meshcentral_token"); v != "" {
					s.Logger.Info("[MeshCentral Test] Found login token in vault", "tokenLength", len(v))
					loginToken = v
				} else {
					s.Logger.Info("[MeshCentral Test] No login token found in vault")
				}
			}
		} else {
			s.Logger.Info("[MeshCentral Test] Vault is nil")
		}

		// URL is always required. Username is required only when no login token is set.
		if url == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "URL is required (not set in config)",
			})
			return
		}
		if username == "" && loginToken == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Username or Login Token is required (not set in config)",
			})
			return
		}

		mc := meshcentral.NewClient(url, username, password, loginToken, s.Cfg.MeshCentral.Insecure)
		if err := mc.Connect(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
		mc.Close()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Login and WebSocket handshake successful.",
		})
	}
}
