package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/desktop"
	"aurago/internal/llm"
	"aurago/internal/tools"
)

func looperRunTimeout(maxIter int) time.Duration {
	base := 5 * time.Minute
	perIter := 2 * time.Minute
	timeout := base + time.Duration(maxIter)*perIter
	if timeout > 4*time.Hour {
		timeout = 4 * time.Hour
	}
	return timeout
}

const looperMaxPromptLen = 10000

func validateLooperPrompts(w http.ResponseWriter, prepare, plan, action, test, exitCond, finish string) bool {
	type field struct {
		name, val string
		req       bool
	}
	fields := []field{
		{"prepare", prepare, true},
		{"plan", plan, true},
		{"action", action, true},
		{"test", test, true},
		{"exit_cond", exitCond, true},
		{"finish", finish, false},
	}
	for _, f := range fields {
		if f.req && strings.TrimSpace(f.val) == "" {
			jsonError(w, fmt.Sprintf("field %q is required", f.name), http.StatusBadRequest)
			return false
		}
		if len(f.val) > looperMaxPromptLen {
			jsonError(w, fmt.Sprintf("field %q exceeds maximum length of %d characters", f.name, looperMaxPromptLen), http.StatusBadRequest)
			return false
		}
	}
	return true
}

func handleLooperPresets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			presets, err := runner.store.ListPresets(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "presets": presets})
		case http.MethodPost:
			var p desktop.LooperPreset
			if err := decodeDesktopJSON(w, r, &p, desktopMediumJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if !validateLooperPrompts(w, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish) {
				return
			}
			id, err := runner.store.SavePreset(r.Context(), p)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "id": id})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleLooperPresetByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/desktop/looper/presets/")
		if path == "" {
			jsonError(w, "Missing preset ID", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			jsonError(w, "Invalid preset ID", http.StatusBadRequest)
			return
		}
		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodPut:
			if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
				return
			}
			var p desktop.LooperPreset
			if err := decodeDesktopJSON(w, r, &p, desktopMediumJSONBodyLimit); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			p.ID = id
			if !validateLooperPrompts(w, p.Prepare, p.Plan, p.Action, p.Test, p.ExitCond, p.Finish) {
				return
			}
			_, err := runner.store.SavePreset(r.Context(), p)
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		case http.MethodDelete:
			if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
				return
			}
			if err := runner.store.DeletePreset(r.Context(), id); err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleLooperExamples(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		examples, err := runner.store.ListExamples(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "examples": examples})
	}
}

func handleLooperRun(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Prepare     string `json:"prepare"`
			Plan        string `json:"plan"`
			Action      string `json:"action"`
			Test        string `json:"test"`
			ExitCond    string `json:"exit_cond"`
			Finish      string `json:"finish"`
			ProviderID  string `json:"provider_id"`
			Model       string `json:"model"`
			MaxIter     int    `json:"max_iter"`
			ContextMode string `json:"context_mode"`
		}
		if err := decodeDesktopJSON(w, r, &req, desktopMediumJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.MaxIter <= 0 {
			req.MaxIter = 20
		}
		if req.MaxIter > 100 {
			req.MaxIter = 100
		}
		if !validateLooperPrompts(w, req.Prepare, req.Plan, req.Action, req.Test, req.ExitCond, req.Finish) {
			return
		}

		// Resolve provider and build client
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		var client llm.ChatClient
		model := req.Model
		if req.ProviderID != "" {
			for _, p := range cfg.Providers {
				if p.ID == req.ProviderID {
					client = llm.NewClientFromProviderDetails(p.Type, p.BaseURL, p.APIKey, p.AccountID)
					if model == "" {
						model = p.Model
					}
					break
				}
			}
		}
		if client == nil {
			client = s.LLMClient
			if model == "" {
				model = cfg.LLM.Model
			}
		}

		// Build tool schemas
		toolSchemas := agent.GetBuiltinToolSchemas(cfg)

		// Build dispatch context
		manifest := tools.NewManifest(cfg.Directories.ToolsDir)
		dispatchCtx := &agent.DispatchContext{
			Cfg:                cfg,
			Logger:             s.Logger,
			LLMClient:          s.LLMClient,
			Vault:              s.Vault,
			Registry:           s.Registry,
			Manifest:           manifest,
			CronManager:        s.CronManager,
			MissionManagerV2:   s.MissionManagerV2,
			LongTermMem:        s.LongTermMem,
			ShortTermMem:       s.ShortTermMem,
			KG:                 s.KG,
			InventoryDB:        s.InventoryDB,
			InvasionDB:         s.InvasionDB,
			CheatsheetDB:       s.CheatsheetDB,
			ImageGalleryDB:     s.ImageGalleryDB,
			MediaRegistryDB:    s.MediaRegistryDB,
			HomepageRegistryDB: s.HomepageRegistryDB,
			ContactsDB:         s.ContactsDB,
			PlannerDB:          s.PlannerDB,
			SQLConnectionsDB:   s.SQLConnectionsDB,
			SQLConnectionPool:  s.SQLConnectionPool,
			RemoteHub:          s.RemoteHub,
			HistoryMgr:         s.HistoryManager,
			IsMaintenance:      tools.IsBusy(),
			Guardian:           s.Guardian,
			LLMGuardian:        s.LLMGuardian,
			SessionID:          "looper",
			CoAgentRegistry:    s.CoAgentRegistry,
			BudgetTracker:      s.BudgetTracker,
			DaemonSupervisor:   s.DaemonSupervisor,
			PreparationService: s.PreparationService,
			MessageSource:      "looper",
		}

		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		loopCtx, loopCancel := context.WithTimeout(context.Background(), looperRunTimeout(req.MaxIter))
		if err := runner.TryStart(req.MaxIter, loopCancel); err != nil {
			loopCancel()
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		go func() {
			defer loopCancel()
			_ = runner.executeStarted(loopCtx, desktop.LooperRunConfig{
				Prepare:     req.Prepare,
				Plan:        req.Plan,
				Action:      req.Action,
				Test:        req.Test,
				ExitCond:    req.ExitCond,
				Finish:      req.Finish,
				ProviderID:  req.ProviderID,
				Model:       model,
				MaxIter:     req.MaxIter,
				ContextMode: desktop.NormalizeContextMode(req.ContextMode),
			}, cfg, client, toolSchemas, dispatchCtx)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "message": "Loop started"})
	}
}

func handleLooperStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		runner.Stop()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}
}

func handleLooperStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			jsonError(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send current state immediately
		state := runner.State()
		data, _ := json.Marshal(state)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		// Poll for updates
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		lastJSON := string(data)
		idleTicks := 0
		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				fmt.Fprintf(w, ":heartbeat\n\n")
				flusher.Flush()
			case <-ticker.C:
				state := runner.State()
				data, _ := json.Marshal(state)
				if string(data) != lastJSON {
					lastJSON = string(data)
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
				if !state.Running && state.CurrentStep == "idle" {
					idleTicks++
					if idleTicks >= 3 {
						return
					}
				}
			}
		}
	}
}
