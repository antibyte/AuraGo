package server

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"

	"aurago/internal/desktop"
)

type openSCADHandlers struct {
	server *Server
}

func registerOpenSCADRoutes(mux *http.ServeMux, s *Server) {
	handlers := openSCADHandlers{server: s}
	mux.HandleFunc("/api/openscad/status", handlers.handleStatus)
	mux.HandleFunc("/api/openscad/render", handlers.handleRender)
	mux.HandleFunc("/api/openscad/jobs/", handlers.handleJobPath)
}

func (h openSCADHandlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopPermission(h.server, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	svc, _, err := h.server.getDesktopService(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "openscad": svc.OpenSCADContainer().Status(r.Context())})
}

func (h openSCADHandlers) handleRender(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopPermission(h.server, w, r, desktopScopeWrite) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.rejectReadOnly(w) {
		return
	}
	svc, _, err := h.server.getDesktopService(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	maxSourceKB := svc.Config().OpenSCAD.MaxSourceKB
	if maxSourceKB <= 0 {
		maxSourceKB = 512
	}
	maxBody := int64(maxSourceKB+16) * 1024
	var req desktop.OpenSCADRenderRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	result, err := svc.OpenSCADContainer().Render(r.Context(), req)
	if err != nil {
		if result.JobID != "" {
			writeJSON(w, map[string]interface{}{"status": "error", "error": err.Error(), "result": result})
			return
		}
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.SaveToDesktop {
		result, err = svc.OpenSCADContainer().SaveJob(r.Context(), svc, result.JobID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "result": result})
}

func (h openSCADHandlers) handleJobPath(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/api/openscad/jobs/")
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	requiredScope := desktopScopeRead
	if len(parts) == 2 && parts[1] == "save" && r.Method == http.MethodPost {
		requiredScope = desktopScopeWrite
	}
	if !requireDesktopPermission(h.server, w, r, requiredScope) {
		return
	}
	svc, _, err := h.server.getDesktopService(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		result, err := svc.OpenSCADContainer().Job(r.Context(), parts[0])
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "result": result})
		return
	}
	if len(parts) == 2 && parts[1] == "save" && r.Method == http.MethodPost {
		if h.rejectReadOnly(w) {
			return
		}
		result, err := svc.OpenSCADContainer().SaveJob(r.Context(), svc, parts[0])
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok", "result": result})
		return
	}
	if len(parts) == 3 && parts[1] == "files" && r.Method == http.MethodGet {
		filePath, file, err := svc.OpenSCADContainer().JobFile(parts[0], path.Base(parts[2]))
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("X-AuraGo-OpenSCAD-SHA256", file.SHA256)
		disposition := "inline"
		if r.URL.Query().Get("download") == "1" {
			disposition = "attachment"
		}
		w.Header().Set("Content-Disposition", disposition+`; filename="`+strings.ReplaceAll(file.Name, `"`, "")+`"`)
		http.ServeFile(w, r, filePath)
		return
	}
	jsonError(w, "Not found", http.StatusNotFound)
}

func (h openSCADHandlers) rejectReadOnly(w http.ResponseWriter) bool {
	if h.server == nil || h.server.Cfg == nil {
		return false
	}
	h.server.CfgMu.RLock()
	readonly := h.server.Cfg.VirtualDesktop.ReadOnly
	h.server.CfgMu.RUnlock()
	if !readonly {
		return false
	}
	jsonError(w, "virtual desktop is read-only", http.StatusForbidden)
	return true
}
