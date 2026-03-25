package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/truenas"
)

// registerTrueNASHandlers registers all TrueNAS HTTP endpoints.
// This function is called from server_routes.go
func registerTrueNASHandlers(mux *http.ServeMux, s *Server) {
	// Overview and health
	mux.HandleFunc("/api/truenas/status", handleTrueNASStatus(s))
	mux.HandleFunc("/api/truenas/health", handleTrueNASHealth(s))

	// Pools
	mux.HandleFunc("/api/truenas/pools", handleTrueNASPools(s))
	mux.HandleFunc("/api/truenas/pools/", handleTrueNASPoolDetail(s))

	// Datasets
	mux.HandleFunc("/api/truenas/datasets", handleTrueNASDatasets(s))

	// Snapshots
	mux.HandleFunc("/api/truenas/snapshots", handleTrueNASSnapshots(s))
	mux.HandleFunc("/api/truenas/snapshots/", handleTrueNASSnapshotActions(s))

	// Shares
	mux.HandleFunc("/api/truenas/shares/smb", handleTrueNASSMBShares(s))
	mux.HandleFunc("/api/truenas/shares/smb/", handleTrueNASSMBShareDetail(s))
	mux.HandleFunc("/api/truenas/shares/nfs", handleTrueNASNFSShares(s))
}

func handleTrueNASStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": false,
				"status":  "disabled",
			})
			return
		}

		// Try to connect and get basic info
		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": true,
				"status":  "error",
				"error":   err.Error(),
			})
			return
		}
		defer client.Close()

		info, err := client.Health(r.Context())
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": true,
				"status":  "offline",
				"error":   err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":  true,
			"status":   "online",
			"version":  info.Version,
			"hostname": info.Hostname,
			"model":    info.Model,
		})
	}
}

func handleTrueNASHealth(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "TrueNAS integration is disabled",
			})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		defer client.Close()

		ctx := r.Context()

		// Get all health data
		info, err := client.Health(ctx)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		pools, _ := client.ListPools(ctx)
		alerts, _ := client.ListAlerts(ctx)

		// Determine overall status
		status := "healthy"
		for _, p := range pools {
			if p.Status != "ONLINE" {
				status = "degraded"
				break
			}
		}

		for _, a := range alerts {
			if !a.Dismissed && (a.Level == "CRITICAL" || a.Level == "ERROR") {
				status = "alert"
				break
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       status,
			"system":       info,
			"pools":        pools,
			"alerts":       alerts,
			"pool_count":   len(pools),
			"alert_count":  len(alerts),
		})
	}
}

func handleTrueNASPools(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "TrueNAS integration is disabled"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		pools, err := client.ListPools(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"pools": pools,
		})
	}
}

func handleTrueNASPoolDetail(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		if cfg.ReadOnly {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
			return
		}

		// Extract pool ID from path /api/truenas/pools/{id}/scrub
		path := strings.TrimPrefix(r.URL.Path, "/api/truenas/pools/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "scrub" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid path"})
			return
		}

		poolID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid pool ID"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		if err := client.ScrubPool(r.Context(), poolID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Scrub started successfully",
		})
	}
}

func handleTrueNASDatasets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "TrueNAS integration is disabled"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		ctx := r.Context()

		switch r.Method {
		case http.MethodGet:
			pool := r.URL.Query().Get("pool")
			var datasets []truenas.Dataset
			if pool != "" {
				datasets, err = client.ListDatasetsByPool(ctx, pool)
			} else {
				datasets, err = client.ListDatasets(ctx)
			}
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"datasets": datasets})

		case http.MethodPost:
			if cfg.ReadOnly {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
				return
			}

			var req struct {
				Name        string `json:"name"`
				Compression string `json:"compression"`
				QuotaGB     int64  `json:"quota_gb"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request body"})
				return
			}

			if req.Name == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Dataset name is required"})
				return
			}

			createReq := truenas.CreateDatasetRequest{
				Name:        req.Name,
				Compression: req.Compression,
			}
			if req.QuotaGB > 0 {
				createReq.Quota = req.QuotaGB * 1024 * 1024 * 1024
			}

			dataset, err := client.CreateDataset(ctx, createReq)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{"dataset": dataset})
		}
	}
}

func handleTrueNASDatasetDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		if cfg.ReadOnly {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
			return
		}

		if !cfg.AllowDestructive {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Destructive operations not allowed"})
			return
		}

		// Extract name from path /api/truenas/datasets/{name}
		name := strings.TrimPrefix(r.URL.Path, "/api/truenas/datasets/")
		name = strings.TrimSuffix(name, "/")

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		recursive := r.URL.Query().Get("recursive") == "true"

		if err := client.DeleteDataset(r.Context(), name, recursive); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Dataset deleted successfully",
		})
	}
}

func handleTrueNASSnapshots(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "TrueNAS integration is disabled"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		ctx := r.Context()

		switch r.Method {
		case http.MethodGet:
			dataset := r.URL.Query().Get("dataset")
			snapshots, err := client.ListSnapshots(ctx, dataset)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"snapshots": snapshots})

		case http.MethodPost:
			if cfg.ReadOnly {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
				return
			}

			var req struct {
				Dataset       string `json:"dataset"`
				Name          string `json:"name"`
				Recursive     bool   `json:"recursive"`
				RetentionDays int    `json:"retention_days"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request body"})
				return
			}

			if req.Dataset == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Dataset is required"})
				return
			}

			createReq := truenas.CreateSnapshotRequest{
				Dataset:   req.Dataset,
				Name:      req.Name,
				Recursive: req.Recursive,
				Retention: req.RetentionDays,
			}

			snapshot, err := client.CreateSnapshot(ctx, createReq)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{"snapshot": snapshot})
		}
	}
}

// handleTrueNASSnapshotActions handles DELETE for snapshot deletion and POST for rollback
func handleTrueNASSnapshotActions(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		// Extract path after /api/truenas/snapshots/
		path := strings.TrimPrefix(r.URL.Path, "/api/truenas/snapshots/")
		path = strings.TrimSuffix(path, "/")

		// Check if it's a rollback request
		isRollback := strings.HasSuffix(path, "/rollback")
		name := path
		if isRollback {
			name = strings.TrimSuffix(path, "/rollback")
		}

		// Check if it's a delete request
		isDelete := r.Method == http.MethodDelete
		isPostRollback := r.Method == http.MethodPost && isRollback

		if !isDelete && !isPostRollback {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Method not allowed"})
			return
		}

		if cfg.ReadOnly {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
			return
		}

		if !cfg.AllowDestructive {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Destructive operations not allowed"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		if isDelete {
			// Delete snapshot
			if err := client.DeleteSnapshot(r.Context(), name); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Snapshot deleted successfully",
			})
		} else {
			// Rollback
			var req struct {
				Force bool `json:"force"`
			}
			json.NewDecoder(r.Body).Decode(&req)

			if err := client.RollbackSnapshot(r.Context(), name, req.Force); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Rollback completed successfully",
			})
		}
	}
}

func handleTrueNASSMBShares(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "TrueNAS integration is disabled"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		ctx := r.Context()

		switch r.Method {
		case http.MethodGet:
			shares, err := client.ListSMBShares(ctx)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"shares": shares})

		case http.MethodPost:
			if cfg.ReadOnly {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
				return
			}

			var req struct {
				Name        string `json:"name"`
				Path        string `json:"path"`
				GuestOK     bool   `json:"guest_ok"`
				Timemachine bool   `json:"timemachine"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request body"})
				return
			}

			if req.Name == "" || req.Path == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Name and path are required"})
				return
			}

			createReq := truenas.CreateSMBShareRequest{
				Name:        req.Name,
				Path:        req.Path,
				Enabled:     true,
				GuestOK:     req.GuestOK,
				Timemachine: req.Timemachine,
				Browseable:  true,
				ShadowCopy:  true,
				RecycleBin:  true,
			}

			share, err := client.CreateSMBShare(ctx, createReq)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{"share": share})
		}
	}
}

func handleTrueNASSMBShareDetail(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		if cfg.ReadOnly {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
			return
		}

		// Extract ID from path /api/truenas/shares/smb/{id}
		idStr := strings.TrimPrefix(r.URL.Path, "/api/truenas/shares/smb/")
		idStr = strings.TrimSuffix(idStr, "/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid share ID"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		if err := client.DeleteSMBShare(r.Context(), id); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "SMB share deleted successfully",
		})
	}
}

func handleTrueNASNFSShares(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		cfg := s.Cfg.TrueNAS
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		if !cfg.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "TrueNAS integration is disabled"})
			return
		}

		client, err := truenas.NewClient(cfg, s.Vault)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer client.Close()

		ctx := r.Context()

		switch r.Method {
		case http.MethodGet:
			shares, err := client.ListNFSShares(ctx)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"shares": shares})

		case http.MethodPost:
			if cfg.ReadOnly {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Read-only mode"})
				return
			}

			var req truenas.CreateNFSShareRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request body"})
				return
			}

			if req.Path == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": "Path is required"})
				return
			}

			share, err := client.CreateNFSShare(ctx, req)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}

			json.NewEncoder(w).Encode(map[string]interface{}{"share": share})
		}
	}
}
