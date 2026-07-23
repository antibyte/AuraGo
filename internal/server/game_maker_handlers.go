package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/gamemaker"
)

const gameMakerJSONLimit = 256 * 1024

func registerGameMakerRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/game-maker/capabilities", handleGameMakerCapabilities(s))
	mux.HandleFunc("/api/game-maker/projects", handleGameMakerProjects(s))
	mux.HandleFunc("/api/game-maker/projects/", handleGameMakerProjectPath(s))
	mux.HandleFunc("/api/game-maker/jobs/", handleGameMakerJobPath(s))
	mux.HandleFunc("/api/game-maker/preview/", handleGameMakerPreview(s))
}

func handleGameMakerCapabilities(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.Cfg == nil {
			jsonError(w, "server not configured", http.StatusServiceUnavailable)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			cfg = s.Cfg
		}
		var providers []gamemaker.Provider
		for _, provider := range cfg.Providers {
			if strings.TrimSpace(provider.ID) == "" {
				continue
			}
			providers = append(providers, gamemaker.Provider{
				ID: provider.ID, Name: provider.Name, Type: provider.Type, Model: provider.Model,
			})
		}
		var skills []gamemaker.SkillInfo
		skillsReady := false
		if s.GameMaker != nil {
			skills, skillsReady = s.GameMaker.SkillStatus()
		}
		writeGameMakerJSON(w, http.StatusOK, gamemaker.Capabilities{
			Enabled:              cfg.GameMaker.Enabled && s.GameMaker != nil,
			ReadOnly:             cfg.GameMaker.ReadOnly,
			AllowCreate:          cfg.GameMaker.AllowCreate && !cfg.GameMaker.ReadOnly,
			AllowEdit:            cfg.GameMaker.AllowEdit && !cfg.GameMaker.ReadOnly,
			AllowDelete:          cfg.GameMaker.AllowDelete && !cfg.GameMaker.ReadOnly,
			AllowMediaGeneration: cfg.GameMaker.AllowMediaGeneration,
			ImageGeneration:      cfg.GameMaker.AllowMediaGeneration && cfg.ImageGeneration.Enabled && cfg.ImageGeneration.APIKey != "",
			MusicGeneration:      cfg.GameMaker.AllowMediaGeneration && cfg.MusicGeneration.Enabled && cfg.MusicGeneration.APIKey != "",
			CodeStudio:           cfg.VirtualDesktop.CodeStudio.Enabled,
			PhaserVersion:        gamemaker.PhaserVersion,
			ThreeVersion:         gamemaker.ThreeVersion,
			SkillsReady:          skillsReady,
			Skills:               skills,
			Providers:            providers,
			DefaultProviderID:    cfg.LLM.Provider,
			DefaultModel:         cfg.LLM.Model,
		})
	}
}

func handleGameMakerProjects(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.GameMaker == nil {
			jsonError(w, "Game Maker service is unavailable", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if !requireDesktopPermission(s, w, r, desktopScopeRead) {
				return
			}
			projects, err := s.GameMaker.ListProjects(r.Context())
			if err != nil {
				handleGameMakerError(w, err)
				return
			}
			writeGameMakerJSON(w, http.StatusOK, map[string]any{"projects": projects})
		case http.MethodPost:
			if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
				return
			}
			var request gamemaker.CreateProjectRequest
			if err := decodeGameMakerJSON(w, r, &request); err != nil {
				return
			}
			project, err := s.GameMaker.CreateProject(r.Context(), request)
			if err != nil {
				handleGameMakerError(w, err)
				return
			}
			writeGameMakerJSON(w, http.StatusCreated, project)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleGameMakerProjectPath(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.GameMaker == nil {
			jsonError(w, "Game Maker service is unavailable", http.StatusServiceUnavailable)
			return
		}
		relative := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/game-maker/projects/"), "/")
		parts := strings.Split(relative, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			jsonError(w, "Project ID is required", http.StatusBadRequest)
			return
		}
		projectID := parts[0]
		if len(parts) == 1 {
			handleGameMakerProjectRecord(w, r, s, projectID)
			return
		}
		switch parts[1] {
		case "jobs":
			handleGameMakerStartJob(w, r, s, projectID)
		case "events":
			handleGameMakerEvents(w, r, s, projectID)
		case "revisions":
			if len(parts) == 2 {
				handleGameMakerRevisions(w, r, s, projectID)
				return
			}
			if len(parts) == 4 && parts[3] == "restore" {
				number, err := strconv.ParseInt(parts[2], 10, 64)
				if err != nil {
					jsonError(w, "Invalid revision", http.StatusBadRequest)
					return
				}
				handleGameMakerRestore(w, r, s, projectID, number)
				return
			}
			jsonError(w, "Game Maker route not found", http.StatusNotFound)
		case "preview-token":
			handleGameMakerPreviewToken(w, r, s, projectID)
		case "export":
			handleGameMakerExport(w, r, s, projectID)
		default:
			jsonError(w, "Game Maker route not found", http.StatusNotFound)
		}
	}
}

func handleGameMakerProjectRecord(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	switch r.Method {
	case http.MethodGet:
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		project, err := s.GameMaker.GetProject(r.Context(), projectID)
		if err != nil {
			handleGameMakerError(w, err)
			return
		}
		messages, err := s.GameMaker.ListMessages(r.Context(), projectID)
		if err != nil {
			handleGameMakerError(w, err)
			return
		}
		writeGameMakerJSON(w, http.StatusOK, map[string]any{"project": project, "messages": messages})
	case http.MethodPatch:
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		var request gamemaker.UpdateProjectRequest
		if err := decodeGameMakerJSON(w, r, &request); err != nil {
			return
		}
		project, err := s.GameMaker.UpdateProject(r.Context(), projectID, request)
		if err != nil {
			handleGameMakerError(w, err)
			return
		}
		writeGameMakerJSON(w, http.StatusOK, project)
	case http.MethodDelete:
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if err := s.GameMaker.DeleteProject(r.Context(), projectID); err != nil {
			handleGameMakerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGameMakerStartJob(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request gamemaker.StartJobRequest
	if err := decodeGameMakerJSON(w, r, &request); err != nil {
		return
	}
	job, err := s.GameMaker.StartJob(r.Context(), projectID, request)
	if err != nil {
		handleGameMakerError(w, err)
		return
	}
	writeGameMakerJSON(w, http.StatusAccepted, job)
}

func handleGameMakerJobPath(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.GameMaker == nil {
			jsonError(w, "Game Maker service is unavailable", http.StatusServiceUnavailable)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/game-maker/jobs/"), "/"), "/")
		if len(parts) != 2 || parts[1] != "cancel" || r.Method != http.MethodPost {
			jsonError(w, "Game Maker route not found", http.StatusNotFound)
			return
		}
		if err := s.GameMaker.CancelJob(r.Context(), parts[0]); err != nil {
			handleGameMakerError(w, err)
			return
		}
		writeGameMakerJSON(w, http.StatusAccepted, map[string]any{"status": "cancelling"})
	}
}

func handleGameMakerEvents(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	if !requireDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	afterID := int64(0)
	if raw := firstNonEmptyGameMakerString(r.Header.Get("Last-Event-ID"), r.URL.Query().Get("after")); raw != "" {
		afterID, _ = strconv.ParseInt(raw, 10, 64)
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	replay, err := s.GameMaker.EventsAfter(r.Context(), projectID, afterID, 500)
	if err != nil {
		writeGameMakerSSEError(w, err)
		return
	}
	for _, event := range replay {
		writeGameMakerSSE(w, event)
		afterID = event.ID
	}
	flusher.Flush()
	events, unsubscribe := s.GameMaker.Subscribe(projectID)
	defer unsubscribe()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if event.ID <= afterID {
				continue
			}
			writeGameMakerSSE(w, event)
			flusher.Flush()
			afterID = event.ID
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func handleGameMakerRevisions(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	if !requireDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	revisions, err := s.GameMaker.ListRevisions(r.Context(), projectID)
	if err != nil {
		handleGameMakerError(w, err)
		return
	}
	writeGameMakerJSON(w, http.StatusOK, map[string]any{"revisions": revisions})
}

func handleGameMakerRestore(w http.ResponseWriter, r *http.Request, s *Server, projectID string, revision int64) {
	if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	restored, err := s.GameMaker.RestoreRevision(r.Context(), projectID, revision)
	if err != nil {
		handleGameMakerError(w, err)
		return
	}
	writeGameMakerJSON(w, http.StatusCreated, restored)
}

func handleGameMakerPreviewToken(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	if !requireDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	grant, err := s.GameMaker.CreatePreviewGrant(projectID)
	if err != nil {
		handleGameMakerError(w, err)
		return
	}
	writeGameMakerJSON(w, http.StatusCreated, grant)
}

func handleGameMakerPreview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.GameMaker == nil {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		relative := strings.TrimPrefix(r.URL.Path, "/api/game-maker/preview/")
		parts := strings.SplitN(relative, "/", 2)
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		data, contentType, err := s.GameMaker.PreviewFile(parts[0], parts[1])
		if err != nil {
			if errors.Is(err, gamemaker.ErrInvalidToken) {
				http.Error(w, "Preview token expired", http.StatusUnauthorized)
				return
			}
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; font-src 'self'; connect-src 'none'; worker-src 'self' blob:; frame-ancestors 'self'; object-src 'none'; base-uri 'none'; form-action 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	}
}

func handleGameMakerExport(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	if !requireDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := s.GameMaker.GetProject(r.Context(), projectID)
	if err != nil {
		handleGameMakerError(w, err)
		return
	}
	exportName := project.Slug + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(exportName, `"`, "")))
	_, err = s.GameMaker.WriteExport(r.Context(), projectID, w)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("Game Maker export failed", "project_id", projectID, "error", err)
		}
		return
	}
}

func decodeGameMakerJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, gameMakerJSONLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return err
	}
	return nil
}

func writeGameMakerJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeGameMakerSSE(w http.ResponseWriter, event gamemaker.Event) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data)
}

func writeGameMakerSSEError(w http.ResponseWriter, err error) {
	data, _ := json.Marshal(map[string]any{"message": err.Error()})
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
}

func handleGameMakerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gamemaker.ErrNotFound):
		jsonError(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, gamemaker.ErrDisabled), errors.Is(err, gamemaker.ErrReadOnly):
		jsonError(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, gamemaker.ErrBusy):
		jsonError(w, err.Error(), http.StatusConflict)
	case errors.Is(err, gamemaker.ErrSkillsUnusable):
		jsonError(w, err.Error(), http.StatusPreconditionFailed)
	case errors.Is(err, gamemaker.ErrInvalidPath):
		jsonError(w, err.Error(), http.StatusBadRequest)
	default:
		jsonError(w, err.Error(), http.StatusBadRequest)
	}
}

func firstNonEmptyGameMakerString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
