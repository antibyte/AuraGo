package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/codegraph"
	"aurago/internal/memory"
)

func registerLearningRoutes(mux *http.ServeMux, s *Server) {
	for _, route := range []string{"/api/learnings", "/api/desktop/learnings"} {
		mux.Handle(route, requireAdmin(s, handleLearnings(s)))
	}
	for _, route := range []string{"/api/code-graph", "/api/desktop/code-graph"} {
		mux.Handle(route, requireAdmin(s, handleCodeGraph(s)))
	}
}

func knowledgeHTTPPermission(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/desktop/") && !requireDesktopOperation(s, w, r, desktopMethodScope(r.Method), desktopMethodOperation(r.Method)) {
		return false
	}
	if r.Method != http.MethodGet {
		_, bearer := bearerCredential(r.Header.Get("Authorization"))
		if !bearer && !checkCSRFOriginWithPolicy(r, true) {
			jsonError(w, "Origin denied", http.StatusForbidden)
			return false
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	return true
}

func handleLearnings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !knowledgeHTTPPermission(s, w, r) {
			return
		}
		if s.ShortTermMem == nil {
			jsonError(w, "Memory unavailable", http.StatusServiceUnavailable)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "Configuration unavailable", http.StatusServiceUnavailable)
			return
		}
		project := r.URL.Query().Get("project")
		scope := memory.LearningScope{}
		if project != "" && project != "global" {
			_, resolved, _, err := s.resolveKnowledgeProject(r.Context(), project, r.URL.Query().Get("resource"))
			if err != nil {
				jsonError(w, "Project unavailable", http.StatusNotFound)
				return
			}
			scope = resolved
			project = resolved.Project
		}
		id := r.URL.Query().Get("id")
		if r.Method == http.MethodGet {
			if id != "" {
				l, err := s.ShortTermMem.LearningForAdmin(r.Context(), id)
				if err != nil || !learningAdminScopeAllows(scope, project, l.Scope) {
					jsonError(w, "Learning unavailable", http.StatusNotFound)
					return
				}
				history, err := s.ShortTermMem.LearningHistory(r.Context(), id, l.Scope)
				if err != nil {
					jsonError(w, "History unavailable", http.StatusInternalServerError)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"learning": l, "history": history})
				return
			}
			list, err := s.ShortTermMem.ListLearningsForAdmin(r.Context(), project, r.URL.Query().Get("query"), r.URL.Query().Get("status"))
			if err != nil {
				jsonError(w, "Learning query failed", http.StatusInternalServerError)
				return
			}
			metrics, err := s.ShortTermMem.LearningMetrics(r.Context())
			if err != nil {
				jsonError(w, "Learning metrics unavailable", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"learnings": list, "enabled": cfg.Learnings.Enabled, "writable": cfg.Learnings.Enabled && cfg.Tools.Memory.Enabled && !cfg.Tools.Memory.ReadOnly && (!strings.HasPrefix(r.URL.Path, "/api/desktop/") || !cfg.VirtualDesktop.ReadOnly), "metrics": metrics, "overlaps": s.knowledge.targets.Snapshot(scope.Project), "limited": len(list) == 200})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !cfg.Learnings.Enabled || !cfg.Tools.Memory.Enabled || cfg.Tools.Memory.ReadOnly {
			jsonError(w, "Learning writes disabled", http.StatusForbidden)
			return
		}
		var req struct {
			ID        string `json:"id"`
			Revision  int    `json:"revision"`
			Action    string `json:"action"`
			Statement string `json:"statement"`
			Reason    string `json:"reason"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		dec.DisallowUnknownFields()
		if dec.Decode(&req) != nil || dec.Decode(&struct{}{}) != io.EOF {
			jsonError(w, "Invalid learning request", http.StatusBadRequest)
			return
		}
		l, err := s.ShortTermMem.LearningForAdmin(r.Context(), req.ID)
		if err != nil || !learningAdminScopeAllows(scope, project, l.Scope) {
			jsonError(w, "Learning unavailable", http.StatusNotFound)
			return
		}
		// Hold publication against config replacement only for the bounded SQLite transaction.
		s.CfgMu.RLock()
		current := s.Cfg
		if current == nil || !current.Learnings.Enabled || !current.Tools.Memory.Enabled || current.Tools.Memory.ReadOnly || (strings.HasPrefix(r.URL.Path, "/api/desktop/") && (!current.VirtualDesktop.Enabled || current.VirtualDesktop.ReadOnly)) {
			s.CfgMu.RUnlock()
			jsonError(w, "Learning authorization revoked", http.StatusForbidden)
			return
		}
		err = publishDesktopResult(r.Context(), func() error {
			var reviewErr error
			l, reviewErr = s.ShortTermMem.ReviewLearning(r.Context(), req.ID, req.Revision, l.Scope, req.Action, req.Statement, req.Reason)
			return reviewErr
		})
		s.CfgMu.RUnlock()
		if err != nil {
			jsonError(w, "Learning changed or action is not applicable; reload and review", http.StatusConflict)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"learning": l})
	}
}

func learningAdminScopeAllows(scope memory.LearningScope, project string, target memory.LearningScope) bool {
	if project == "" {
		return true
	}
	if project == "global" {
		return target.Project == ""
	}
	return scope.Project == target.Project && (scope.Resource == "" || scope.Resource == target.Resource)
}

func handleCodeGraph(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !knowledgeHTTPPermission(s, w, r) {
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.CodeGraph.Enabled {
			jsonError(w, "Code graph disabled", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("project")
		if id == "" {
			id = "workspace"
		}
		p, _, _, err := s.resolveKnowledgeProject(r.Context(), id, "")
		if err != nil {
			jsonError(w, "Project unavailable", http.StatusNotFound)
			return
		}
		q := codegraph.Request{Operation: r.URL.Query().Get("operation"), Query: r.URL.Query().Get("query"), File: strings.TrimPrefix(r.URL.Query().Get("file"), "/workspace/"), Symbol: r.URL.Query().Get("symbol")}
		q.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
		q.Depth, _ = strconv.Atoi(r.URL.Query().Get("depth"))
		if q.Operation == "" {
			q.Operation = "status"
		}
		if q.Operation == "rescan" && r.Method != http.MethodPost {
			jsonError(w, "Rescan requires POST", http.StatusMethodNotAllowed)
			return
		}
		manager := s.codeGraphService()
		if manager == nil {
			jsonError(w, "Code graph unavailable", http.StatusServiceUnavailable)
			return
		}
		result, err := manager.Query(r.Context(), p, q)
		if err != nil {
			jsonError(w, "Code graph unavailable or symbol is ambiguous", http.StatusConflict)
			return
		}
		if current := s.ConfigSnapshot(); current == nil || !current.CodeGraph.Enabled {
			jsonError(w, "Code graph disabled", http.StatusForbidden)
			return
		}
		json.NewEncoder(w).Encode(result)
	}
}
