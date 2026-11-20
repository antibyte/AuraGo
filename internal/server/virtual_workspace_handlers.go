package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
	"aurago/internal/virtualcomputers"
)

func registerVirtualWorkspaceRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/virtual-computers/workspaces", handleVirtualWorkspaces(s))
	mux.HandleFunc("/api/virtual-computers/workspaces/", handleVirtualWorkspace(s))
	mux.HandleFunc("/api/virtual-computers/legacy-volume-imports", handleLegacyVolumeImports(s))
	mux.HandleFunc("/api/virtual-computers/legacy-volume-imports/", handleLegacyVolumeImport(s))
}

func handleLegacyVolumeImports(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := virtualWorkspaceManager(s)
		if manager == nil {
			writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "workspace_manager_unavailable", Message: "virtual workspace manager is unavailable"})
			return
		}
		var request struct {
			SourceVolumeID string   `json:"source_volume_id"`
			Paths          []string `json:"paths"`
		}
		if err := decodeWorkspaceJSON(r, &request); err != nil {
			jsonError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		preview, err := manager.BeginLegacyVolumeImport(r.Context(), virtualComputersConfigSnapshot(s), virtualWorkspaceIdentity(), request.SourceVolumeID, request.Paths)
		if err == nil {
			virtualComputersRecordAction(s, r, "scan_legacy_workspace_volume", "volume", request.SourceVolumeID, map[string]interface{}{
				"import_id": preview.ID, "paths": preview.Paths, "file_count": preview.FileCount, "total_bytes": preview.TotalBytes, "expires_at": preview.ExpiresAt,
			})
		}
		writeVirtualWorkspaceResult(w, map[string]interface{}{"preview": preview, "requires_user_confirmation": true}, err)
	}
}

func handleLegacyVolumeImport(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		manager := virtualWorkspaceManager(s)
		if manager == nil {
			writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "workspace_manager_unavailable", Message: "virtual workspace manager is unavailable"})
			return
		}
		remainder := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/virtual-computers/legacy-volume-imports/"), "/")
		parts := strings.Split(remainder, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			jsonError(w, "legacy volume import id is required", http.StatusBadRequest)
			return
		}
		importID := strings.TrimSpace(parts[0])
		cfg := virtualComputersConfigSnapshot(s)
		identity := virtualWorkspaceIdentity()
		if r.Method == http.MethodDelete && len(parts) == 1 {
			err := manager.CancelLegacyVolumeImport(r.Context(), cfg, identity, importID)
			if err == nil {
				virtualComputersRecordAction(s, r, "cancel_legacy_workspace_volume_import", "volume_import", importID, nil)
			}
			writeVirtualWorkspaceResult(w, nil, err)
			return
		}
		if r.Method != http.MethodPost || len(parts) != 2 || parts[1] != "confirm" {
			jsonError(w, "Not found", http.StatusNotFound)
			return
		}
		var request struct {
			TTLSeconds int `json:"ttl_seconds"`
		}
		if err := decodeWorkspaceJSON(r, &request); err != nil {
			jsonError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		volume, err := manager.ConfirmLegacyVolumeImport(r.Context(), cfg, identity, importID, request.TTLSeconds)
		if err == nil {
			virtualComputersRecordAction(s, r, "confirm_legacy_workspace_volume_import", "volume", volume.ID, map[string]interface{}{
				"import_id": importID, "format": virtualcomputers.WorkspaceVolumeFormat,
			})
		}
		writeVirtualWorkspaceResult(w, map[string]interface{}{"volume": volume}, err)
	}
}

func virtualWorkspaceIdentity() virtualcomputers.WorkspaceIdentity {
	return virtualcomputers.WorkspaceIdentity{SessionID: "desktop-admin", Actor: "authenticated_user", Admin: true}
}

func virtualWorkspaceManager(s *Server) *virtualcomputers.WorkspaceManager {
	if s != nil && s.VirtualWorkspaceManager != nil {
		return s.VirtualWorkspaceManager
	}
	return virtualcomputers.DefaultWorkspaceManager()
}

func handleVirtualWorkspaces(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
			return
		}
		manager := virtualWorkspaceManager(s)
		if manager == nil {
			writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "workspace_manager_unavailable", Message: "virtual workspace manager is unavailable"})
			return
		}
		cfg := virtualComputersConfigSnapshot(s)
		switch r.Method {
		case http.MethodGet:
			items, err := manager.List(r.Context(), virtualWorkspaceIdentity(), r.URL.Query().Get("include_closed") == "true")
			if err != nil {
				writeVirtualWorkspaceError(w, err)
				return
			}
			summaries := make([]map[string]interface{}, 0, len(items))
			for _, workspace := range items {
				jobs, _ := manager.ListJobs(r.Context(), virtualWorkspaceIdentity(), workspace.ID)
				var currentOutput *virtualcomputers.WorkspaceJobOutput
				for _, job := range jobs {
					if job.State == virtualcomputers.JobStateRunning || job.State == virtualcomputers.JobStateQueued {
						if output, outputErr := manager.JobOutput(r.Context(), cfg, virtualWorkspaceIdentity(), workspace.ID, job.ID, 0, 64<<10); outputErr == nil {
							currentOutput = &output
						}
						break
					}
				}
				browserSessions, _ := manager.ListBrowserSessions(r.Context(), virtualWorkspaceIdentity(), workspace.ID)
				grants, _ := manager.ListCredentialGrants(r.Context(), virtualWorkspaceIdentity(), workspace.ID)
				events, _ := manager.ListEvents(r.Context(), virtualWorkspaceIdentity(), workspace.ID, 0, 100)
				summaries = append(summaries, map[string]interface{}{
					"workspace": workspace, "jobs": jobs, "job_output": currentOutput, "browser_sessions": browserSessions, "credential_grants": grants, "events": events,
				})
			}
			writeJSON(w, map[string]interface{}{"status": "ok", "workspaces": items, "workspace_summaries": summaries})
		case http.MethodPost:
			var request virtualcomputers.WorkspaceOpenRequest
			if err := decodeWorkspaceJSON(r, &request); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			workspace, err := manager.Open(r.Context(), cfg, virtualWorkspaceIdentity(), request)
			if err != nil {
				writeVirtualWorkspaceError(w, err)
				return
			}
			writeJSON(w, map[string]interface{}{"status": "ok", "workspace": workspace})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleVirtualWorkspace(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, tail := splitVirtualWorkspacePath(r.URL.Path)
		if workspaceID == "" {
			jsonError(w, "workspace id is required", http.StatusBadRequest)
			return
		}
		if tail == "events" {
			handleVirtualWorkspaceEvents(s, workspaceID).ServeHTTP(w, r)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopMethodScope(r.Method)) {
			return
		}
		manager := virtualWorkspaceManager(s)
		if manager == nil {
			writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "workspace_manager_unavailable", Message: "virtual workspace manager is unavailable"})
			return
		}
		cfg := virtualComputersConfigSnapshot(s)
		identity := virtualWorkspaceIdentity()
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) == 1 && parts[0] == "" {
			parts = nil
		}
		if len(parts) == 0 {
			handleVirtualWorkspaceRoot(w, r, manager, cfg, identity, workspaceID)
			return
		}
		switch parts[0] {
		case "desktop":
			if r.Method != http.MethodGet || len(parts) != 2 || parts[1] != "status" {
				jsonError(w, "Not found", http.StatusNotFound)
				return
			}
			status, err := manager.DesktopStatus(r.Context(), cfg, identity, workspaceID)
			writeVirtualWorkspaceResult(w, map[string]interface{}{"desktop": status}, err)
		case "jobs":
			handleVirtualWorkspaceJobs(w, r, manager, cfg, identity, workspaceID, parts[1:])
		case "browser-sessions":
			handleVirtualWorkspaceBrowser(w, r, manager, cfg, identity, workspaceID, parts[1:])
		case "credential-grants":
			handleVirtualWorkspaceGrants(w, r, s, manager, cfg, identity, workspaceID, parts[1:])
		case "checkpoint":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var request struct {
				TTLSeconds int    `json:"ttl_seconds"`
				Name       string `json:"name"`
				Persistent bool   `json:"persistent"`
				VolumeID   string `json:"volume_id"`
			}
			if err := decodeWorkspaceJSON(r, &request); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			name, nameErr := virtualComputersNormalizeLabel(request.Name)
			if nameErr != nil {
				writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "invalid_name", Message: nameErr.Error()})
				return
			}
			if request.TTLSeconds > cfg.VolumeMaxTTL() {
				writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "volume_ttl_too_long", Message: "volume retention exceeds the configured maximum"})
				return
			}
			if request.Persistent {
				client, clientErr := virtualComputersClient(s)
				if clientErr != nil || !virtualComputersVolumeRetention(r.Context(), client) {
					writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "volume_retention_requires_repair", Message: virtualComputersRetentionRepairMessage})
					return
				}
			}
			volume, err := manager.CheckpointWithOptions(r.Context(), cfg, identity, workspaceID, virtualcomputers.CheckpointOptions{
				TTLSeconds: request.TTLSeconds, Name: name, Persistent: request.Persistent, VolumeID: strings.TrimSpace(request.VolumeID), AllowLocked: true,
			})
			writeVirtualWorkspaceResult(w, map[string]interface{}{"volume": volume}, err)
		case "control":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var request struct {
				Owner string `json:"owner"`
			}
			if err := decodeWorkspaceJSON(r, &request); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			if request.Owner != virtualcomputers.ControlOwnerHuman && request.Owner != virtualcomputers.ControlOwnerAgent {
				writeVirtualWorkspaceError(w, virtualcomputers.WorkspaceRPCError{Code: "invalid_control_owner", Message: "control owner must be human or agent"})
				return
			}
			workspace, err := manager.TakeControl(r.Context(), identity, workspaceID, request.Owner == virtualcomputers.ControlOwnerHuman)
			writeVirtualWorkspaceResult(w, map[string]interface{}{"workspace": workspace}, err)
		default:
			jsonError(w, "Not found", http.StatusNotFound)
		}
	}
}

func handleVirtualWorkspaceRoot(w http.ResponseWriter, r *http.Request, manager *virtualcomputers.WorkspaceManager, cfg virtualcomputers.ToolConfig, identity virtualcomputers.WorkspaceIdentity, workspaceID string) {
	switch r.Method {
	case http.MethodGet:
		workspace, err := manager.Get(r.Context(), identity, workspaceID)
		writeVirtualWorkspaceResult(w, map[string]interface{}{"workspace": workspace}, err)
	case http.MethodDelete:
		released := false
		if workspace, err := manager.Get(r.Context(), identity, workspaceID); err == nil {
			released = workspace.Origin == virtualcomputers.WorkspaceOriginAdopted
		}
		writeVirtualWorkspaceResult(w, map[string]interface{}{"released": released}, manager.CloseWorkspace(r.Context(), cfg, identity, workspaceID))
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleVirtualWorkspaceJobs(w http.ResponseWriter, r *http.Request, manager *virtualcomputers.WorkspaceManager, cfg virtualcomputers.ToolConfig, identity virtualcomputers.WorkspaceIdentity, workspaceID string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := manager.ListJobs(r.Context(), identity, workspaceID)
			writeVirtualWorkspaceResult(w, map[string]interface{}{"jobs": items}, err)
		case http.MethodPost:
			var request virtualcomputers.WorkspaceStartJobRequest
			if err := decodeWorkspaceJSON(r, &request); err != nil {
				jsonError(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			job, err := manager.StartJob(r.Context(), cfg, identity, workspaceID, request)
			writeVirtualWorkspaceResult(w, map[string]interface{}{"job": job}, err)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	jobID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch action {
	case "":
		if r.Method == http.MethodDelete {
			writeVirtualWorkspaceResult(w, nil, manager.CancelJob(r.Context(), cfg, identity, workspaceID, jobID))
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		job, err := manager.JobStatus(r.Context(), cfg, identity, workspaceID, jobID)
		writeVirtualWorkspaceResult(w, map[string]interface{}{"job": job}, err)
	case "output":
		cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		output, err := manager.JobOutput(r.Context(), cfg, identity, workspaceID, jobID, cursor, limit)
		writeVirtualWorkspaceResult(w, map[string]interface{}{"output": output}, err)
	case "input":
		var request struct {
			Input string `json:"input"`
			Rows  uint16 `json:"rows"`
			Cols  uint16 `json:"cols"`
		}
		if r.Method != http.MethodPost || decodeWorkspaceJSON(r, &request) != nil {
			jsonError(w, "Invalid job input request", http.StatusBadRequest)
			return
		}
		writeVirtualWorkspaceResult(w, nil, manager.JobInput(r.Context(), cfg, identity, workspaceID, jobID, request.Input, request.Rows, request.Cols))
	default:
		jsonError(w, "Not found", http.StatusNotFound)
	}
}

func handleVirtualWorkspaceBrowser(w http.ResponseWriter, r *http.Request, manager *virtualcomputers.WorkspaceManager, cfg virtualcomputers.ToolConfig, identity virtualcomputers.WorkspaceIdentity, workspaceID string, parts []string) {
	if r.Method == http.MethodGet && len(parts) == 0 {
		items, err := manager.ListBrowserSessions(r.Context(), identity, workspaceID)
		writeVirtualWorkspaceResult(w, map[string]interface{}{"browser_sessions": items}, err)
		return
	}
	var request virtualcomputers.BrowserActionRequest
	if err := decodeWorkspaceJSON(r, &request); err != nil {
		jsonError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(parts) > 0 {
		request.SessionID = parts[0]
	}
	if r.Method == http.MethodDelete {
		request.Operation = "close"
	} else if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.Operation == "" {
		request.Operation = "open"
	}
	var result virtualcomputers.BrowserActionResult
	var err error
	if request.Operation == "credential_fill" {
		result, err = manager.UseBrowserGrant(r.Context(), cfg, identity, workspaceID, request)
	} else {
		result, err = manager.BrowserAction(r.Context(), cfg, identity, workspaceID, request)
	}
	writeVirtualWorkspaceResult(w, map[string]interface{}{"result": result}, err)
}

func handleVirtualWorkspaceGrants(w http.ResponseWriter, r *http.Request, s *Server, manager *virtualcomputers.WorkspaceManager, cfg virtualcomputers.ToolConfig, identity virtualcomputers.WorkspaceIdentity, workspaceID string, parts []string) {
	if len(parts) == 0 {
		if r.Method == http.MethodGet {
			items, err := manager.ListCredentialGrants(r.Context(), identity, workspaceID)
			writeVirtualWorkspaceResult(w, map[string]interface{}{"credential_grants": items}, err)
			return
		}
		var request virtualcomputers.CredentialGrantRequest
		if r.Method != http.MethodPost || decodeWorkspaceJSON(r, &request) != nil {
			jsonError(w, "Invalid credential grant request", http.StatusBadRequest)
			return
		}
		grant, err := manager.RequestCredentialGrant(r.Context(), cfg, identity, workspaceID, request)
		writeVirtualWorkspaceResult(w, map[string]interface{}{"credential_grant": grant, "requires_user_approval": true}, err)
		return
	}
	grantID := parts[0]
	action := "revoke"
	if len(parts) > 1 {
		action = parts[1]
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if action == "approve" {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		grant, err := manager.ApproveCredentialGrant(r.Context(), cfg, identity, grantID)
		if err == nil {
			virtualComputersRecordAction(s, r, "approve_credential_grant", "workspace", workspaceID, map[string]interface{}{
				"grant_id": grant.ID, "credential_id": grant.CredentialID, "usage_type": grant.UsageType,
				"origin": grant.Origin, "job_id": grant.JobID, "field_names": grant.FieldNames, "expires_at": grant.ExpiresAt,
			})
		}
		writeVirtualWorkspaceResult(w, map[string]interface{}{"credential_grant": grant}, err)
		return
	}
	err := manager.RevokeCredentialGrant(r.Context(), cfg, identity, workspaceID, grantID)
	if err == nil {
		virtualComputersRecordAction(s, r, "revoke_credential_grant", "workspace", workspaceID, map[string]interface{}{"grant_id": grantID})
	}
	writeVirtualWorkspaceResult(w, nil, err)
}

func handleVirtualWorkspaceEvents(s *Server, workspaceID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := virtualWorkspaceManager(s)
		if manager == nil {
			jsonError(w, "virtual workspace manager is unavailable", http.StatusServiceUnavailable)
			return
		}
		if _, err := manager.Get(r.Context(), virtualWorkspaceIdentity(), workspaceID); err != nil {
			writeVirtualWorkspaceError(w, err)
			return
		}
		connection, err := virtualComputersWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		cursor, _ := strconv.ParseInt(r.URL.Query().Get("after_id"), 10, 64)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			events, err := manager.ListEvents(r.Context(), virtualWorkspaceIdentity(), workspaceID, cursor, 100)
			if err != nil {
				_ = connection.WriteJSON(map[string]interface{}{"status": "error", "error": err.Error()})
				return
			}
			for _, event := range events {
				if err := connection.WriteJSON(event); err != nil {
					return
				}
				cursor = event.ID
			}
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
			}
		}
	}
}

func splitVirtualWorkspacePath(path string) (string, string) {
	remainder := strings.TrimPrefix(path, "/api/virtual-computers/workspaces/")
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), strings.Trim(parts[1], "/")
}

func decodeWorkspaceJSON(r *http.Request, target interface{}) error {
	if r.Body == nil {
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, (8<<20)+1))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func writeVirtualWorkspaceResult(w http.ResponseWriter, payload map[string]interface{}, err error) {
	if err != nil {
		writeVirtualWorkspaceError(w, err)
		return
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["status"] = "ok"
	writeJSON(w, payload)
}

func writeVirtualWorkspaceError(w http.ResponseWriter, err error) {
	code := "workspace_error"
	message := security.Scrub(strings.TrimSpace(err.Error()))
	status := http.StatusInternalServerError
	var rpcValue virtualcomputers.WorkspaceRPCError
	var rpcPointer *virtualcomputers.WorkspaceRPCError
	var restErr virtualcomputers.RESTError
	switch {
	case errors.As(err, &rpcValue):
		code, message, status = rpcValue.Code, rpcValue.Message, http.StatusBadRequest
	case errors.As(err, &rpcPointer) && rpcPointer != nil:
		code, message, status = rpcPointer.Code, rpcPointer.Message, http.StatusBadRequest
	case errors.As(err, &restErr):
		classified := virtualcomputers.ClassifyError(err)
		code, message, status = classified.Code, security.Scrub(classified.Message), classified.HTTPStatus
	case errors.Is(err, context.DeadlineExceeded):
		code, message, status = "workspace_timeout", "virtual workspace operation timed out", http.StatusGatewayTimeout
	default:
		slog.Default().Warn("[VirtualWorkspace] operation failed", "error", message)
		message = "virtual workspace operation failed"
	}
	switch code {
	case "volume_ttl_too_long", "invalid_name":
		status = http.StatusBadRequest
	case "machine_owned_by_workspace", "machine_not_running", "desktop_control_unconfirmed", "volume_retention_requires_repair", "volume_locked", "volume_in_use":
		status = http.StatusConflict
	case "readonly", "disabled", "agent_control_disabled":
		status = http.StatusForbidden
	case "not_found":
		status = http.StatusNotFound
	case "forbidden", "owner_mismatch":
		status = http.StatusForbidden
	case "workspace_manager_unavailable", "workspace_agent_upgrade_required", "control_plane_repair_required":
		status = http.StatusServiceUnavailable
	case "workspace_limit_reached", "job_limit_reached", "browser_session_limit_reached", "browser_human_control":
		status = http.StatusConflict
	case "legacy_import_in_progress", "legacy_import_changed":
		status = http.StatusConflict
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "error": code, "message": message})
}
