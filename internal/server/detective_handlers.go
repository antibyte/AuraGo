package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/detective"
)

func registerDetectiveRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/detective/", s.handleDetective)
}

func detectiveJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func detectiveError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, detective.ErrNotFound) {
		status = 404
	}
	if errors.Is(err, detective.ErrConflict) || errors.Is(err, detective.ErrBudget) {
		status = 409
	}
	detectiveJSON(w, status, map[string]string{"error": err.Error()})
}
func detectiveDecode(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

// detectiveBlockReason names why the research surface is unavailable, so the UI can
// explain disabled actions instead of showing a generic notice. Empty means ready.
func detectiveBlockReason(cfg *config.Config, ready bool) string {
	if cfg == nil || !cfg.Detective.Enabled {
		return "disabled"
	}
	if !ready {
		return "initializing"
	}
	if cfg.Detective.ReadOnly || cfg.VirtualDesktop.ReadOnly || !cfg.VirtualDesktop.AllowAgentControl {
		return "read_only"
	}
	return ""
}

func (s *Server) handleDetective(w http.ResponseWriter, r *http.Request) {
	// Cases can contain selected private material. Desktop read-only bearer scopes
	// never grant access to this administrative research surface.
	if !authenticateDesktopPermission(s, w, r, desktopScopeAdmin) {
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/detective/"), "/")
	parts := strings.Split(path, "/")
	var runRequest struct {
		Action string `json:"action"`
		Effort string `json:"effort"`
		Key    string `json:"idempotency_key"`
		Answer string `json:"answer"`
	}
	operation := desktopMethodOperation(r.Method)
	var autoUpdate detective.AutoUpdateSettings
	if len(parts) == 3 && parts[0] == "cases" && parts[2] == "auto-update" && r.Method == http.MethodPut {
		if err := detectiveDecode(w, r, &autoUpdate); err != nil {
			detectiveError(w, err)
			return
		}
		if !autoUpdate.Enabled {
			operation = desktopStop
		}
	}
	if len(parts) == 3 && parts[0] == "cases" && parts[2] == "run" && r.Method == http.MethodPost {
		if err := detectiveDecode(w, r, &runRequest); err != nil {
			detectiveError(w, err)
			return
		}
		if runRequest.Action == "stop" {
			operation = desktopStop
		}
	}
	if !checkDesktopOperation(s, w, r, operation) {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		jsonError(w, "Desktop unavailable", 503)
		return
	}
	if path == "capabilities" && r.Method == http.MethodGet {
		tools := []string{}
		for _, schema := range detectiveSchemas(cfg, s, detective.Request{}) {
			tools = append(tools, schema.Function.Name)
		}
		private := []string{}
		for name := range cfg.Detective.ExtraReadOperations {
			private = append(private, name)
		}
		sort.Strings(private)
		providers := []map[string]string{}
		for _, p := range cfg.Providers {
			if strings.TrimSpace(p.ID) == "" || !config.ProviderUsageAllowsGeneralChat(&p) {
				continue
			}
			usage := config.NormalizeProviderUsage(p.Usage)
			if usage == "" {
				usage = config.ProviderUsageGeneral
			}
			providers = append(providers, map[string]string{"id": p.ID, "name": p.Name, "model": p.Model, "usage": usage})
		}
		profiles := detective.Profiles()
		ready := s.Detective != nil && s.Detective.Ready()
		if s.Detective != nil {
			profiles = s.Detective.Profiles()
		}
		detectiveJSON(w, 200, map[string]any{"enabled": cfg.Detective.Enabled, "ready": ready, "block_reason": detectiveBlockReason(cfg, ready), "read_only": cfg.Detective.ReadOnly || cfg.VirtualDesktop.ReadOnly || !cfg.VirtualDesktop.AllowAgentControl, "profiles": profiles, "tools": tools, "private_sources": private, "providers": providers, "provider_id": cfg.LLM.Provider, "model": cfg.LLM.Model})
		return
	}
	if s.Detective == nil || !cfg.Detective.Enabled {
		jsonError(w, "Detective unavailable", 503)
		return
	}
	if operation != desktopRead && operation != desktopStop && (cfg.Detective.ReadOnly || !cfg.VirtualDesktop.AllowAgentControl) {
		jsonError(w, "Detective is read-only", 403)
		return
	}
	if parts[0] != "cases" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			cases, err := s.Detective.ListSummaries()
			if err != nil {
				detectiveError(w, err)
				return
			}
			for i := range cases {
				cases[i].Sources = nil
				cases[i].Findings = nil
				cases[i].Reports = nil
				cases[i].Answers = nil
				cases[i].Plan = nil
			}
			detectiveJSON(w, 200, map[string]any{"cases": cases})
		case http.MethodPost:
			var req detective.Request
			if err := detectiveDecode(w, r, &req); err != nil {
				detectiveError(w, err)
				return
			}
			for _, name := range req.PrivateSources {
				if len(cfg.Detective.ExtraReadOperations[name]) == 0 {
					detectiveError(w, errors.New("private source is not approved for research"))
					return
				}
			}
			if providerID := strings.TrimSpace(req.ProviderID); providerID != "" {
				provider := cfg.FindProvider(providerID)
				if provider == nil {
					detectiveError(w, errors.New("selected provider is unavailable"))
					return
				}
				if !config.ProviderUsageAllowsGeneralChat(provider) {
					detectiveError(w, errors.New("selected provider is not available for general chat"))
					return
				}
			}
			c, err := s.Detective.Create(req)
			if err != nil {
				detectiveError(w, err)
				return
			}
			detectiveJSON(w, 201, c)
		default:
			jsonError(w, "Method not allowed", 405)
		}
		return
	}
	key := parts[1]
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			c, err := s.Detective.Get(key)
			if err != nil {
				detectiveError(w, err)
				return
			}
			detectiveJSON(w, 200, c)
		case http.MethodDelete:
			if err := s.Detective.Delete(key); err != nil {
				detectiveError(w, err)
				return
			}
			detectiveJSON(w, 200, map[string]bool{"deleted": true})
		default:
			jsonError(w, "Method not allowed", 405)
		}
		return
	}
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	switch parts[2] {
	case "auto-update":
		if r.Method != http.MethodPut {
			jsonError(w, "Method not allowed", 405)
			return
		}
		var c detective.Case
		var err error
		if !autoUpdate.Enabled && (cfg.Detective.ReadOnly || cfg.VirtualDesktop.ReadOnly || !cfg.VirtualDesktop.AllowAgentControl) {
			c, err = s.Detective.Get(key)
			if err == nil {
				autoUpdate = c.AutoUpdate.AutoUpdateSettings
				autoUpdate.Enabled = false
				c, err = s.Detective.SetAutoUpdate(key, autoUpdate)
			}
		} else {
			ctx, release, grantErr := s.beginDesktopRun(r.Context())
			if grantErr != nil {
				detectiveError(w, grantErr)
				return
			}
			defer release()
			err = publishDesktopResult(ctx, func() error { c, err = s.Detective.SetAutoUpdate(key, autoUpdate); return err })
		}
		if err != nil {
			detectiveError(w, err)
			return
		}
		detectiveJSON(w, 200, c)
	case "reports", "sources", "findings":
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		before, err := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
		if r.URL.Query().Get("before") == "" {
			before, err = 0, nil
		}
		if err != nil {
			detectiveError(w, errors.New("invalid archive cursor"))
			return
		}
		limit := 20
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
		if err != nil {
			detectiveError(w, errors.New("invalid archive limit"))
			return
		}
		page, err := s.Detective.Archive(key, parts[2], before, limit, r.URL.Query().Get("query"))
		if err != nil {
			detectiveError(w, err)
			return
		}
		detectiveJSON(w, 200, page)
	case "report":
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
		if err != nil || revision < 1 {
			detectiveError(w, errors.New("report revision required"))
			return
		}
		report, err := s.Detective.ReportRevision(key, revision)
		if err != nil {
			detectiveError(w, err)
			return
		}
		detectiveJSON(w, 200, report)
	case "source":
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		source, err := s.Detective.Source(key, r.URL.Query().Get("id"))
		if err != nil {
			detectiveError(w, err)
			return
		}
		detectiveJSON(w, 200, source)
	case "events":
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		ok, err := s.Detective.Exists(key)
		if err != nil {
			detectiveError(w, err)
			return
		}
		if !ok {
			detectiveError(w, detective.ErrNotFound)
			return
		}
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		events, err := s.Detective.Events(key, after)
		if err != nil {
			detectiveError(w, err)
			return
		}
		detectiveJSON(w, 200, map[string]any{"events": events})
	case "live":
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		view, err := s.Detective.Live(key)
		if err != nil {
			detectiveError(w, err)
			return
		}
		detectiveJSON(w, 200, view)
	case "run":
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		req := runRequest
		if req.Action == "stop" || req.Action == "finish" {
			if err := s.Detective.Stop(key, req.Action == "finish"); err != nil {
				detectiveError(w, err)
				return
			}
			c, _ := s.Detective.Get(key)
			detectiveJSON(w, 200, c)
			return
		}
		c, err := s.Detective.Start(key, req.Action, req.Effort, req.Key, req.Answer)
		if err != nil {
			detectiveError(w, err)
			return
		}
		detectiveJSON(w, 202, c)
	case "export", "autor":
		autor := parts[2] == "autor"
		if (!autor && r.Method != http.MethodGet) || (autor && r.Method != http.MethodPost) {
			jsonError(w, "Method not allowed", 405)
			return
		}
		revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
		if err != nil {
			detectiveError(w, errors.New("report revision required"))
			return
		}
		format := r.URL.Query().Get("format")
		if autor {
			format = "docx"
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		artifact, err := s.Detective.ExportRevision(ctx, key, revision, format)
		if err != nil {
			detectiveError(w, err)
			return
		}
		if autor {
			if s.DesktopService == nil {
				jsonError(w, "Desktop files unavailable", 503)
				return
			}
			path := fmt.Sprintf("Documents/Detective/%s-r%d-%d.docx", key, revision, time.Now().UnixNano())
			_, err = s.DesktopService.WriteFileBytesConditional(ctx, path, artifact.Data, desktop.SourceUser, func(state desktop.FileWriteState) error {
				if state.Exists {
					return errors.New("export copy already exists")
				}
				return nil
			})
			if err != nil {
				detectiveError(w, err)
				return
			}
			detectiveJSON(w, 201, map[string]string{"path": path})
			return
		}
		w.Header().Set("Content-Type", artifact.MIME)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("ETag", `"`+artifact.SHA256+`"`)
		w.Header().Set("Content-Disposition", `attachment; filename="`+artifact.Filename+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(artifact.Data)))
		_, _ = w.Write(artifact.Data)
	default:
		http.NotFound(w, r)
	}
}
