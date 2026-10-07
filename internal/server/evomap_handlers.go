package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/evomap"
	"aurago/internal/security"
)

type evomapStatusResponse struct {
	Status                string `json:"status"`
	Message               string `json:"message,omitempty"`
	Enabled               bool   `json:"enabled"`
	ReadOnly              bool   `json:"readonly"`
	BaseURL               string `json:"base_url"`
	NodeID                string `json:"node_id,omitempty"`
	NodeSecretConfigured  bool   `json:"node_secret_configured"`
	APIKeyConfigured      bool   `json:"api_key_configured"`
	KGEnabled             bool   `json:"kg_enabled"`
	AllowPublish          bool   `json:"allow_publish"`
	AllowReport           bool   `json:"allow_report"`
	AllowBounties         bool   `json:"allow_bounties"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	MaxResultBytes        int    `json:"max_result_bytes"`
}

type evomapServerClient interface {
	Status(context.Context) (evomap.StatusResult, error)
	RegisterNode(context.Context, evomap.RegisterRequest) (evomap.RegisterResponse, error)
}

var newEvomapServerClient = func(cfg config.EvomapConfig) (evomapServerClient, error) {
	return evomapClientFromServerConfig(cfg)
}

func evomapConfigSnapshot(s *Server) config.EvomapConfig {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	if s.Cfg == nil {
		return config.EvomapConfig{}
	}
	return s.Cfg.Evomap
}

func evomapStatusFromConfig(cfg config.EvomapConfig) evomapStatusResponse {
	status := "disabled"
	message := "EvoMap integration is disabled"
	if cfg.Enabled {
		status = "ready"
		message = "EvoMap integration is configured"
		if strings.TrimSpace(cfg.NodeSecret) == "" {
			status = "missing_node_secret"
			message = "EvoMap node secret is not configured"
		}
	}
	return evomapStatusResponse{
		Status:                status,
		Message:               message,
		Enabled:               cfg.Enabled,
		ReadOnly:              cfg.ReadOnly,
		BaseURL:               cfg.BaseURL,
		NodeID:                cfg.NodeID,
		NodeSecretConfigured:  strings.TrimSpace(cfg.NodeSecret) != "",
		APIKeyConfigured:      strings.TrimSpace(cfg.APIKey) != "",
		KGEnabled:             cfg.KGEnabled,
		AllowPublish:          cfg.AllowPublish,
		AllowReport:           cfg.AllowReport,
		AllowBounties:         cfg.AllowBounties,
		RequestTimeoutSeconds: cfg.RequestTimeoutSeconds,
		MaxResultBytes:        cfg.MaxResultBytes,
	}
}

func evomapClientFromServerConfig(cfg config.EvomapConfig) (*evomap.Client, error) {
	return evomap.NewClient(evomap.Config{
		BaseURL:        cfg.BaseURL,
		NodeID:         cfg.NodeID,
		NodeSecret:     cfg.NodeSecret,
		APIKey:         cfg.APIKey,
		Timeout:        time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
		MaxResultBytes: int64(cfg.MaxResultBytes),
	})
}

func handleEvomapStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(evomapStatusFromConfig(evomapConfigSnapshot(s)))
	}
}

func handleEvomapTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cfg := evomapConfigSnapshot(s)
		client, err := newEvomapServerClient(cfg)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(max(5, cfg.RequestTimeoutSeconds))*time.Second)
		defer cancel()
		status, err := client.Status(ctx)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Connection successful",
			"evomap":  json.RawMessage(status.Raw),
		})
	}
}

func handleEvomapRegister(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		nodeID, claimURL, configured, err := s.registerEvomapNode(r.Context(), evomapConfigSnapshot(s))
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": security.Scrub(err.Error())})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok", "message": "EvoMap node registered", "node_id": nodeID, "claim_url": claimURL, "node_secret_configured": configured,
		})
	}
}
