package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/invasion"
)

// ── Nest Handlers ───────────────────────────────────────────────────────────

// handleInvasionNests handles GET (list) and POST (create) for /api/invasion/nests.
func handleInvasionNests(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil {
			jsonError(w, "Invasion Control is not enabled", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			nests, err := invasion.ListNests(s.InvasionDB)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load nests", "Failed to list invasion nests", err)
				return
			}
			if nests == nil {
				nests = []invasion.NestRecord{}
			}
			type nestResponse struct {
				invasion.NestRecord
				HasSecret   bool        `json:"has_secret"`
				WSConnected bool        `json:"ws_connected"`
				Telemetry   interface{} `json:"telemetry"`
				EggName     string      `json:"egg_name,omitempty"`
			}
			resp := make([]nestResponse, 0, len(nests))
			for _, n := range nests {
				hasWS := false
				var tel interface{} = nil
				if s.EggHub != nil {
					hasWS = s.EggHub.IsConnected(n.ID)
					if c := s.EggHub.GetConnection(n.ID); c != nil {
						tel = c.GetTelemetry()
					}
				}
				eggName := ""
				if n.EggID != "" {
					if egg, err := invasion.GetEgg(s.InvasionDB, n.EggID); err == nil {
						eggName = egg.Name
					}
				}
				resp = append(resp, nestResponse{
					NestRecord: invasion.NestRecord{
						ID:               n.ID,
						Name:             n.Name,
						Notes:            n.Notes,
						AccessType:       n.AccessType,
						Host:             n.Host,
						Port:             n.Port,
						Username:         n.Username,
						Active:           n.Active,
						EggID:            n.EggID,
						HatchStatus:      n.HatchStatus,
						HatchError:       n.HatchError,
						LastHatchAt:      n.LastHatchAt,
						DeployMethod:     n.DeployMethod,
						TargetArch:       n.TargetArch,
						Route:            n.Route,
						RouteConfig:      n.RouteConfig,
						DesiredConfigRev: n.DesiredConfigRev,
						AppliedConfigRev: n.AppliedConfigRev,
						CreatedAt:        n.CreatedAt,
						UpdatedAt:        n.UpdatedAt,
					},
					HasSecret:   n.VaultSecretID != "",
					WSConnected: hasWS,
					Telemetry:   tel,
					EggName:     eggName,
				})
			}
			writeJSON(w, resp)

		case http.MethodPost:
			var req struct {
				Name         string `json:"name"`
				Notes        string `json:"notes"`
				AccessType   string `json:"access_type"`
				Host         string `json:"host"`
				Port         int    `json:"port"`
				Username     string `json:"username"`
				Secret       string `json:"secret"`
				Active       bool   `json:"active"`
				DeployMethod string `json:"deploy_method"`
				TargetArch   string `json:"target_arch"`
				Route        string `json:"route"`
				RouteConfig  string `json:"route_config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if req.Name == "" {
				jsonError(w, "Name is required", http.StatusBadRequest)
				return
			}
			if req.AccessType == "" {
				req.AccessType = "ssh"
			}
			// Validate enum fields
			switch req.AccessType {
			case "ssh", "docker", "local":
			default:
				jsonError(w, "Invalid access_type (must be ssh, docker, or local)", http.StatusBadRequest)
				return
			}
			if req.DeployMethod != "" {
				switch req.DeployMethod {
				case "ssh", "docker_remote", "docker_local":
				default:
					jsonError(w, "Invalid deploy_method (must be ssh, docker_remote, or docker_local)", http.StatusBadRequest)
					return
				}
			}
			if req.TargetArch != "" {
				switch req.TargetArch {
				case "linux/amd64", "linux/arm64":
				default:
					jsonError(w, "Invalid target_arch (must be linux/amd64 or linux/arm64)", http.StatusBadRequest)
					return
				}
			}
			if req.Route != "" {
				switch req.Route {
				case "direct", "ssh_tunnel", "tailscale", "wireguard", "custom":
				default:
					jsonError(w, "Invalid route (must be direct, ssh_tunnel, tailscale, wireguard, or custom)", http.StatusBadRequest)
					return
				}
			}
			if req.Port <= 0 {
				switch req.AccessType {
				case "docker":
					req.Port = 2375
				default:
					req.Port = 22
				}
			}

			nest := invasion.NestRecord{
				Name:         req.Name,
				Notes:        req.Notes,
				AccessType:   req.AccessType,
				Host:         req.Host,
				Port:         req.Port,
				Username:     req.Username,
				Active:       req.Active,
				DeployMethod: req.DeployMethod,
				TargetArch:   req.TargetArch,
				Route:        req.Route,
				RouteConfig:  req.RouteConfig,
			}

			// Store secret in vault if provided
			if req.Secret != "" {
				id, err := invasion.CreateNest(s.InvasionDB, nest)
				if err != nil {
					jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to create nest", "Failed to create invasion nest", err)
					return
				}
				vaultKey := "nest_" + id
				if err := s.Vault.WriteSecret(vaultKey, req.Secret); err != nil {
					// Rollback: delete the nest
					_ = invasion.DeleteNest(s.InvasionDB, id)
					jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to store nest secret", "Failed to store invasion nest secret", err, "nest_id", id)
					return
				}
				// Update nest with vault reference
				created, _ := invasion.GetNest(s.InvasionDB, id)
				created.VaultSecretID = vaultKey
				_ = invasion.UpdateNest(s.InvasionDB, created)

				writeJSON(w, map[string]interface{}{
					"id":          id,
					"name":        req.Name,
					"access_type": req.AccessType,
					"host":        req.Host,
					"port":        req.Port,
					"has_secret":  true,
					"active":      req.Active,
				})
			} else {
				id, err := invasion.CreateNest(s.InvasionDB, nest)
				if err != nil {
					jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to create nest", "Failed to create invasion nest", err)
					return
				}
				writeJSON(w, map[string]interface{}{
					"id":          id,
					"name":        req.Name,
					"access_type": req.AccessType,
					"host":        req.Host,
					"port":        req.Port,
					"has_secret":  false,
					"active":      req.Active,
				})
			}

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleInvasionNest handles GET/PUT/DELETE for /api/invasion/nests/{id}.
func handleInvasionNest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil {
			jsonError(w, "Invasion Control is not enabled", http.StatusServiceUnavailable)
			return
		}
		id := extractInvasionID(r.URL.Path, "/api/invasion/nests/")
		if id == "" {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}
		// Strip trailing sub-paths (toggle, validate)
		if strings.Contains(id, "/") {
			jsonError(w, "Invalid nest ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			nest, err := invasion.GetNest(s.InvasionDB, id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Nest not found", "Invasion nest lookup failed", err, "nest_id", id)
				return
			}
			eggName := ""
			if nest.EggID != "" {
				if egg, err := invasion.GetEgg(s.InvasionDB, nest.EggID); err == nil {
					eggName = egg.Name
				}
			}
			writeJSON(w, map[string]interface{}{
				"id":            nest.ID,
				"name":          nest.Name,
				"notes":         nest.Notes,
				"access_type":   nest.AccessType,
				"host":          nest.Host,
				"port":          nest.Port,
				"username":      nest.Username,
				"active":        nest.Active,
				"egg_id":        nest.EggID,
				"has_secret":    nest.VaultSecretID != "",
				"hatch_status":  nest.HatchStatus,
				"hatch_error":   nest.HatchError,
				"deploy_method": nest.DeployMethod,
				"target_arch":   nest.TargetArch,
				"route":         nest.Route,
				"route_config":  nest.RouteConfig,
				"created_at":    nest.CreatedAt,
				"updated_at":    nest.UpdatedAt,
				"egg_name":      eggName,
			})

		case http.MethodPut:
			var req struct {
				Name         string `json:"name"`
				Notes        string `json:"notes"`
				AccessType   string `json:"access_type"`
				Host         string `json:"host"`
				Port         int    `json:"port"`
				Username     string `json:"username"`
				Secret       string `json:"secret"`
				Active       bool   `json:"active"`
				EggID        string `json:"egg_id"`
				DeployMethod string `json:"deploy_method"`
				TargetArch   string `json:"target_arch"`
				Route        string `json:"route"`
				RouteConfig  string `json:"route_config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Validate enum fields (same rules as POST handler)
			if req.AccessType != "" {
				switch req.AccessType {
				case "ssh", "docker", "local":
				default:
					jsonError(w, "Invalid access_type (must be ssh, docker, or local)", http.StatusBadRequest)
					return
				}
			}
			if req.DeployMethod != "" {
				switch req.DeployMethod {
				case "ssh", "docker_remote", "docker_local":
				default:
					jsonError(w, "Invalid deploy_method (must be ssh, docker_remote, or docker_local)", http.StatusBadRequest)
					return
				}
			}
			if req.TargetArch != "" {
				switch req.TargetArch {
				case "linux/amd64", "linux/arm64":
				default:
					jsonError(w, "Invalid target_arch (must be linux/amd64 or linux/arm64)", http.StatusBadRequest)
					return
				}
			}
			if req.Route != "" {
				switch req.Route {
				case "direct", "ssh_tunnel", "tailscale", "wireguard", "custom":
				default:
					jsonError(w, "Invalid route (must be direct, ssh_tunnel, tailscale, wireguard, or custom)", http.StatusBadRequest)
					return
				}
			}

			existing, err := invasion.GetNest(s.InvasionDB, id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Nest not found", "Invasion nest lookup failed", err, "nest_id", id)
				return
			}

			existing.Name = req.Name
			existing.Notes = req.Notes
			existing.AccessType = req.AccessType
			existing.Host = req.Host
			existing.Port = req.Port
			existing.Username = req.Username
			existing.Active = req.Active
			existing.EggID = req.EggID
			existing.DeployMethod = req.DeployMethod
			existing.TargetArch = req.TargetArch
			existing.Route = req.Route
			existing.RouteConfig = req.RouteConfig

			// Update secret if provided (non-empty)
			if req.Secret != "" {
				vaultKey := "nest_" + id
				if err := s.Vault.WriteSecret(vaultKey, req.Secret); err != nil {
					jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to store nest secret", "Failed to update invasion nest secret", err, "nest_id", id)
					return
				}
				existing.VaultSecretID = vaultKey
			}

			if err := invasion.UpdateNest(s.InvasionDB, existing); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to update nest", "Failed to update invasion nest", err, "nest_id", id)
				return
			}
			writeJSON(w, map[string]string{"status": "updated"})

		case http.MethodDelete:
			// Clean up vault secret before deleting
			nest, err := invasion.GetNest(s.InvasionDB, id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Nest not found", "Invasion nest lookup failed", err, "nest_id", id)
				return
			}
			if nest.VaultSecretID != "" {
				_ = s.Vault.DeleteSecret(nest.VaultSecretID)
			}
			if err := invasion.DeleteNest(s.InvasionDB, id); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete nest", "Failed to delete invasion nest", err, "nest_id", id)
				return
			}
			// Fire mission trigger: nest cleared
			if s.MissionManagerV2 != nil {
				s.MissionManagerV2.NotifyInvasionEvent("nest_cleared", id, nest.Name, "", "")
			}
			writeJSON(w, map[string]string{"status": "deleted"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleInvasionNestToggle handles POST for /api/invasion/nests/{id}/toggle.
func handleInvasionNestToggle(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Path: /api/invasion/nests/{id}/toggle
		path := strings.TrimPrefix(r.URL.Path, "/api/invasion/nests/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 1 {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}
		id := parts[0]

		var req struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := invasion.ToggleNestActive(s.InvasionDB, id, req.Active); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to change nest status", "Failed to toggle invasion nest", err, "nest_id", id)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "toggled", "active": req.Active})
	}
}

// handleInvasionNestValidate handles POST for /api/invasion/nests/{id}/validate.
func handleInvasionNestValidate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/invasion/nests/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 1 {
			jsonError(w, "Missing nest ID", http.StatusBadRequest)
			return
		}
		id := parts[0]

		nest, err := invasion.GetNest(s.InvasionDB, id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusNotFound, "Nest not found", "Invasion nest lookup failed", err, "nest_id", id)
			return
		}

		start := time.Now()
		validationErr := validateNestConnection(nest, s)
		elapsed := time.Since(start)

		if validationErr != nil {
			writeJSON(w, map[string]interface{}{
				"success": false,
				"message": validationErr.Error(),
				"time_ms": elapsed.Milliseconds(),
			})
			return
		}
		writeJSON(w, map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Connection successful (%dms)", elapsed.Milliseconds()),
			"time_ms": elapsed.Milliseconds(),
		})
	}
}

// validateNestConnection tests connectivity to a nest using the appropriate
// connector based on the nest's deploy_method.
func validateNestConnection(nest invasion.NestRecord, s *Server) error {
	// Read vault secret if needed (SSH deployments require credentials)
	var secret []byte
	if nest.VaultSecretID != "" {
		sec, err := s.Vault.ReadSecret(nest.VaultSecretID)
		if err != nil {
			return fmt.Errorf("failed to read secret from vault: %w", err)
		}
		secret = []byte(sec)
	} else if nest.DeployMethod == "ssh" {
		return fmt.Errorf("no SSH secret configured for this nest")
	}

	connector := invasion.GetConnector(nest)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return connector.Validate(ctx, nest, secret)
}

// ── Egg Handlers ────────────────────────────────────────────────────────────

// handleInvasionEggs handles GET (list) and POST (create) for /api/invasion/eggs.
func handleInvasionEggs(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil {
			jsonError(w, "Invasion Control is not enabled", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			eggs, err := invasion.ListEggs(s.InvasionDB)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load eggs", "Failed to list invasion eggs", err)
				return
			}
			if eggs == nil {
				eggs = []invasion.EggRecord{}
			}
			// Strip api_key_ref from response, add has_api_key flag
			type eggResponse struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				Description  string `json:"description"`
				Model        string `json:"model"`
				Provider     string `json:"provider"`
				BaseURL      string `json:"base_url"`
				Active       bool   `json:"active"`
				HasAPIKey    bool   `json:"has_api_key"`
				Permanent    bool   `json:"permanent"`
				IncludeVault bool   `json:"include_vault"`
				InheritLLM   bool   `json:"inherit_llm"`
				EggPort      int    `json:"egg_port"`
				AllowedTools string `json:"allowed_tools"`
				CreatedAt    string `json:"created_at"`
				UpdatedAt    string `json:"updated_at"`
			}
			resp := make([]eggResponse, 0, len(eggs))
			for _, e := range eggs {
				resp = append(resp, eggResponse{
					ID:           e.ID,
					Name:         e.Name,
					Description:  e.Description,
					Model:        e.Model,
					Provider:     e.Provider,
					BaseURL:      e.BaseURL,
					Active:       e.Active,
					HasAPIKey:    e.APIKeyRef != "",
					Permanent:    e.Permanent,
					IncludeVault: e.IncludeVault,
					InheritLLM:   e.InheritLLM,
					EggPort:      e.EggPort,
					AllowedTools: e.AllowedTools,
					CreatedAt:    e.CreatedAt,
					UpdatedAt:    e.UpdatedAt,
				})
			}
			writeJSON(w, resp)

		case http.MethodPost:
			var req struct {
				Name         string `json:"name"`
				Description  string `json:"description"`
				Model        string `json:"model"`
				Provider     string `json:"provider"`
				BaseURL      string `json:"base_url"`
				APIKey       string `json:"api_key"`
				Active       bool   `json:"active"`
				Permanent    bool   `json:"permanent"`
				IncludeVault bool   `json:"include_vault"`
				InheritLLM   bool   `json:"inherit_llm"`
				EggPort      int    `json:"egg_port"`
				AllowedTools string `json:"allowed_tools"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if req.Name == "" {
				jsonError(w, "Name is required", http.StatusBadRequest)
				return
			}

			egg := invasion.EggRecord{
				Name:         req.Name,
				Description:  req.Description,
				Model:        req.Model,
				Provider:     req.Provider,
				BaseURL:      req.BaseURL,
				Active:       req.Active,
				Permanent:    req.Permanent,
				IncludeVault: req.IncludeVault,
				InheritLLM:   req.InheritLLM,
				EggPort:      req.EggPort,
				AllowedTools: req.AllowedTools,
			}

			id, err := invasion.CreateEgg(s.InvasionDB, egg)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to create egg", "Failed to create invasion egg", err)
				return
			}

			// Store API key in vault if provided
			if req.APIKey != "" {
				vaultKey := "egg_apikey_" + id
				if err := s.Vault.WriteSecret(vaultKey, req.APIKey); err != nil {
					_ = invasion.DeleteEgg(s.InvasionDB, id)
					jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to store egg API key", "Failed to store invasion egg API key", err, "egg_id", id)
					return
				}
				created, _ := invasion.GetEgg(s.InvasionDB, id)
				created.APIKeyRef = vaultKey
				_ = invasion.UpdateEgg(s.InvasionDB, created)
			}

			writeJSON(w, map[string]interface{}{
				"id":          id,
				"name":        req.Name,
				"has_api_key": req.APIKey != "",
				"active":      req.Active,
			})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleInvasionEgg handles GET/PUT/DELETE for /api/invasion/eggs/{id}.
func handleInvasionEgg(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil {
			jsonError(w, "Invasion Control is not enabled", http.StatusServiceUnavailable)
			return
		}
		id := extractInvasionID(r.URL.Path, "/api/invasion/eggs/")
		if id == "" || strings.Contains(id, "/") {
			jsonError(w, "Invalid egg ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			egg, err := invasion.GetEgg(s.InvasionDB, id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Egg not found", "Invasion egg lookup failed", err, "egg_id", id)
				return
			}
			writeJSON(w, map[string]interface{}{
				"id":            egg.ID,
				"name":          egg.Name,
				"description":   egg.Description,
				"model":         egg.Model,
				"provider":      egg.Provider,
				"base_url":      egg.BaseURL,
				"active":        egg.Active,
				"has_api_key":   egg.APIKeyRef != "",
				"permanent":     egg.Permanent,
				"include_vault": egg.IncludeVault,
				"inherit_llm":   egg.InheritLLM,
				"egg_port":      egg.EggPort,
				"allowed_tools": egg.AllowedTools,
				"created_at":    egg.CreatedAt,
				"updated_at":    egg.UpdatedAt,
			})

		case http.MethodPut:
			var req struct {
				Name         string `json:"name"`
				Description  string `json:"description"`
				Model        string `json:"model"`
				Provider     string `json:"provider"`
				BaseURL      string `json:"base_url"`
				APIKey       string `json:"api_key"`
				Active       bool   `json:"active"`
				Permanent    bool   `json:"permanent"`
				IncludeVault bool   `json:"include_vault"`
				InheritLLM   bool   `json:"inherit_llm"`
				EggPort      int    `json:"egg_port"`
				AllowedTools string `json:"allowed_tools"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			existing, err := invasion.GetEgg(s.InvasionDB, id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Egg not found", "Invasion egg lookup failed", err, "egg_id", id)
				return
			}

			existing.Name = req.Name
			existing.Description = req.Description
			existing.Model = req.Model
			existing.Provider = req.Provider
			existing.BaseURL = req.BaseURL
			existing.Active = req.Active
			existing.Permanent = req.Permanent
			existing.IncludeVault = req.IncludeVault
			existing.InheritLLM = req.InheritLLM
			existing.EggPort = req.EggPort
			existing.AllowedTools = req.AllowedTools

			if req.APIKey != "" {
				vaultKey := "egg_apikey_" + id
				if err := s.Vault.WriteSecret(vaultKey, req.APIKey); err != nil {
					jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to store egg API key", "Failed to update invasion egg API key", err, "egg_id", id)
					return
				}
				existing.APIKeyRef = vaultKey
			}

			if err := invasion.UpdateEgg(s.InvasionDB, existing); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to update egg", "Failed to update invasion egg", err, "egg_id", id)
				return
			}
			writeJSON(w, map[string]string{"status": "updated"})

		case http.MethodDelete:
			egg, err := invasion.GetEgg(s.InvasionDB, id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Egg not found", "Invasion egg lookup failed", err, "egg_id", id)
				return
			}
			if egg.APIKeyRef != "" {
				_ = s.Vault.DeleteSecret(egg.APIKeyRef)
			}
			if err := invasion.DeleteEgg(s.InvasionDB, id); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete egg", "Failed to delete invasion egg", err, "egg_id", id)
				return
			}
			writeJSON(w, map[string]string{"status": "deleted"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleInvasionEggToggle handles POST for /api/invasion/eggs/{id}/toggle.
func handleInvasionEggToggle(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.InvasionDB == nil || r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/invasion/eggs/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 1 {
			jsonError(w, "Missing egg ID", http.StatusBadRequest)
			return
		}
		id := parts[0]

		var req struct {
			Active bool `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := invasion.ToggleEggActive(s.InvasionDB, id, req.Active); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to change egg status", "Failed to toggle invasion egg", err, "egg_id", id)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "toggled", "active": req.Active})
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func extractInvasionID(urlPath, prefix string) string {
	rest := strings.TrimPrefix(urlPath, prefix)
	rest = strings.TrimSuffix(rest, "/")
	return rest
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonErrorWithDetails(w http.ResponseWriter, msg string, details string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   msg,
		"details": details,
	})
}
