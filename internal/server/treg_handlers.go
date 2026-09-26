package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

var newTregServerClient = tools.NewTregClient

// Config routes are administrative. Tests and catalog reads use only saved
// credentials/config; these routes never proxy /call or accept a token or URL.
func handleTreg(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		op := strings.TrimPrefix(r.URL.Path, "/api/treg/")
		method := http.MethodGet
		if op == "test-connection" {
			method = http.MethodPost
		}
		if r.Method != method {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "Configuration unavailable", http.StatusServiceUnavailable)
			return
		}
		token := ""
		if s.Vault != nil {
			token, _ = s.Vault.ReadSecret("treg_token")
		}
		if op == "status" {
			state := "configured"
			if !cfg.Treg.Enabled {
				state = "disabled"
			} else if !cfg.Agent.AllowNetworkRequests {
				state = "network_disabled"
			} else if token == "" {
				state = "needs_setup"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": state, "key_present": token != "", "readonly": cfg.Treg.ReadOnly, "max_call_cost_micro": cfg.Treg.MaxCallCostMicro, "allowed_endpoints": len(cfg.Treg.AllowedEndpoints)})
			return
		}
		if op != "catalog" && op != "endpoint" && op != "balance" && op != "test-connection" {
			jsonError(w, "Not found", http.StatusNotFound)
			return
		}
		if !cfg.Agent.AllowNetworkRequests {
			jsonError(w, "Network requests are disabled", http.StatusForbidden)
			return
		}
		if token == "" {
			jsonError(w, "Save an organization-scoped treg token in the Vault first", http.StatusBadRequest)
			return
		}
		client, err := newTregServerClient(token)
		if err != nil {
			jsonError(w, security.Scrub(err.Error()), http.StatusBadGateway)
			return
		}
		client.Authorize = func() (config.TregConfig, error) {
			current := s.ConfigSnapshot()
			if current == nil || !current.Agent.AllowNetworkRequests {
				return config.TregConfig{}, fmt.Errorf("network requests are disabled")
			}
			return current.Treg, nil
		}
		var data any
		switch op {
		case "catalog":
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			data, err = client.Search(r.Context(), r.URL.Query().Get("q"), limit)
		case "endpoint":
			_, data, err = client.Endpoint(r.Context(), r.URL.Query().Get("id"))
		case "balance", "test-connection":
			data, err = client.Balance(r.Context())
		}
		if err != nil {
			jsonError(w, security.Scrub(err.Error()), http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": data})
	}
}

func injectTregDefaults(raw map[string]interface{}, cfg *config.Config) {
	if cfg == nil {
		return
	}
	b, _ := json.Marshal(cfg.Treg)
	var section map[string]interface{}
	_ = json.Unmarshal(b, &section)
	raw["treg"] = section
}
