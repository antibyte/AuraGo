package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	hf "aurago/internal/huggingface"
	"aurago/internal/tools"
)

func handleHuggingFaceStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := huggingFaceServerConfig(s)
		status := "disabled"
		if cfg.Enabled {
			status = "public_read_only"
			if strings.TrimSpace(cfg.Token) != "" {
				status = "ready"
			}
		}
		writeHuggingFaceJSON(w, map[string]interface{}{
			"status":                      status,
			"configured":                  strings.TrimSpace(cfg.Token) != "",
			"enabled":                     cfg.Enabled,
			"read_only":                   cfg.ReadOnly,
			"allow_writes":                cfg.AllowWrites,
			"allow_delete":                cfg.AllowDelete,
			"allow_jobs":                  cfg.AllowJobs,
			"allow_scheduled_jobs":        cfg.AllowScheduledJobs,
			"max_download_mb":             cfg.MaxDownloadMB,
			"max_dataset_rows":            cfg.MaxDatasetRows,
			"job_default_timeout_minutes": cfg.JobDefaultTimeoutMinutes,
			"job_max_runtime_minutes":     cfg.JobMaxRuntimeMinutes,
		}, http.StatusOK)
	}
}

func handleHuggingFaceTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := huggingFaceServerConfig(s)
		if !cfg.Enabled {
			writeHuggingFaceJSON(w, map[string]interface{}{"status": "error", "message": "Hugging Face integration is disabled"}, http.StatusBadRequest)
			return
		}
		client := hf.NewClient(hf.ClientConfig{
			HubBaseURL:            cfg.HubBaseURL,
			DatasetBaseURL:        cfg.DatasetBaseURL,
			JobsBaseURL:           cfg.JobsBaseURL,
			Token:                 cfg.Token,
			RequestTimeoutSeconds: cfg.RequestTimeoutSeconds,
			MaxResultBytes:        cfg.MaxResultBytes,
		})
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(maxHuggingFaceTimeout(cfg.RequestTimeoutSeconds))*time.Second)
		defer cancel()
		if strings.TrimSpace(cfg.Token) != "" {
			identity, err := client.WhoAmI(ctx)
			if err != nil {
				writeHuggingFaceJSON(w, map[string]interface{}{"status": "error", "message": err.Error()}, http.StatusBadGateway)
				return
			}
			writeHuggingFaceJSON(w, map[string]interface{}{"status": "ok", "message": "Connection successful", "identity": identity}, http.StatusOK)
			return
		}
		models, err := client.SearchModels(ctx, hf.SearchOptions{Limit: 1})
		if err != nil {
			writeHuggingFaceJSON(w, map[string]interface{}{"status": "error", "message": err.Error()}, http.StatusBadGateway)
			return
		}
		writeHuggingFaceJSON(w, map[string]interface{}{"status": "ok", "message": "Public Hub access successful", "models": models}, http.StatusOK)
	}
}

func handleHuggingFaceJobs(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		dataDir := "data"
		if s != nil && s.Cfg != nil && strings.TrimSpace(s.Cfg.Directories.DataDir) != "" {
			dataDir = s.Cfg.Directories.DataDir
		}
		limit := 100
		jobs, err := tools.HuggingFaceLedgerJobs(r.Context(), dataDir, limit)
		if err != nil {
			writeHuggingFaceJSON(w, map[string]interface{}{"status": "error", "message": err.Error()}, http.StatusInternalServerError)
			return
		}
		writeHuggingFaceJSON(w, map[string]interface{}{"jobs": jobs}, http.StatusOK)
	}
}

func huggingFaceServerConfig(s *Server) config.HuggingFaceConfig {
	if s == nil || s.Cfg == nil {
		return config.HuggingFaceConfig{}
	}
	cfg := s.Cfg.HuggingFace
	if strings.TrimSpace(cfg.Token) == "" && s.Vault != nil {
		if token, err := s.Vault.ReadSecret("huggingface_token"); err == nil {
			cfg.Token = token
		}
	}
	return cfg
}

func maxHuggingFaceTimeout(seconds int) int {
	if seconds <= 0 {
		return 60
	}
	return seconds
}

func writeHuggingFaceJSON(w http.ResponseWriter, payload interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
