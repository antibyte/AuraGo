package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/llm"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func looperRunTimeout(maxRounds int) time.Duration {
	base := 5 * time.Minute
	perRound := 2 * time.Minute
	timeout := base + time.Duration(maxRounds)*perRound
	if timeout > 4*time.Hour {
		timeout = 4 * time.Hour
	}
	return timeout
}

const looperMaxPromptLen = 10000

func validateLooperPrompts(w http.ResponseWriter, goal, work, evaluate, finish string) bool {
	type field struct {
		name, val string
		req       bool
	}
	fields := []field{
		{"goal", goal, true},
		{"work", work, true},
		{"evaluate", evaluate, true},
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
		requiredScope := desktopScopeRead
		if r.Method == http.MethodPost {
			requiredScope = desktopScopeAdmin
		}
		if !requireDesktopPermission(s, w, r, requiredScope) {
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
			if !validateLooperPrompts(w, p.Goal, p.Work, p.Evaluate, p.Finish) {
				return
			}
			desktop.NormalizeLooperPreset(&p)
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
			if !validateLooperPrompts(w, p.Goal, p.Work, p.Evaluate, p.Finish) {
				return
			}
			desktop.NormalizeLooperPreset(&p)
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

func handleLooperRuns(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requiredScope := desktopScopeRead
		if r.Method == http.MethodDelete {
			requiredScope = desktopScopeAdmin
		}
		if !requireDesktopPermission(s, w, r, requiredScope) {
			return
		}
		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			runs, err := runner.store.ListRuns(r.Context())
			if err != nil {
				jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if runs == nil {
				runs = []desktop.LooperRunRecord{}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "runs": runs})
		case http.MethodDelete:
			if err := runner.store.ClearRuns(r.Context()); err != nil {
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

func handleLooperRunByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requiredScope := desktopScopeRead
		if r.Method == http.MethodDelete {
			requiredScope = desktopScopeAdmin
		}
		if !requireDesktopPermission(s, w, r, requiredScope) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/desktop/looper/runs/")
		if path == "" {
			jsonError(w, "Missing run ID", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			jsonError(w, "Invalid run ID", http.StatusBadRequest)
			return
		}
		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			rec, err := runner.store.GetRun(r.Context(), id)
			if err != nil {
				status := http.StatusInternalServerError
				if errors.Is(err, sql.ErrNoRows) {
					status = http.StatusNotFound
				}
				jsonError(w, err.Error(), status)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "run": rec})
		case http.MethodDelete:
			if err := runner.store.DeleteRun(r.Context(), id); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				jsonError(w, err.Error(), status)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
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
		var req looperRunRequest
		if err := decodeDesktopJSON(w, r, &req, desktopMediumJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		normalizeLooperRunRequest(&req)
		if !validateLooperPrompts(w, req.Goal, req.Work, req.Evaluate, req.Finish) {
			return
		}

		cfg, client, model, dispatchCtx, toolSchemas, err := buildLooperRuntime(s, req.ProviderID, req.Model)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		loopCtx, loopCancel := context.WithTimeout(context.Background(), looperRunTimeout(req.MaxRounds))
		if err := runner.TryStart(req.MaxRounds, loopCancel); err != nil {
			loopCancel()
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		go func() {
			defer loopCancel()
			if err := runner.executeStarted(loopCtx, req.toRunConfig(model), cfg, client, toolSchemas, dispatchCtx, nil); err != nil {
				s.Logger.Error("looper execution failed", "error", err)
			}
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

func handleLooperPause(s *Server) http.HandlerFunc {
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
		runner.Pause()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "pause_requested"})
	}
}

func handleLooperResume(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req looperRunRequest
		if err := decodeDesktopJSON(w, r, &req, desktopMediumJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		normalizeLooperRunRequest(&req)
		if !validateLooperPrompts(w, req.Goal, req.Work, req.Evaluate, req.Finish) {
			return
		}

		runner, err := getLooperRunner(s)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		rs, ok := runner.ResumeState()
		if !ok {
			jsonError(w, "no paused run to resume", http.StatusConflict)
			return
		}

		cfg, client, model, dispatchCtx, toolSchemas, err := buildLooperRuntime(s, req.ProviderID, req.Model)
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		loopCtx, loopCancel := context.WithTimeout(context.Background(), looperRunTimeout(req.MaxRounds))
		if err := runner.TryStartResume(req.MaxRounds, rs.Round, loopCancel); err != nil {
			loopCancel()
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		go func() {
			defer loopCancel()
			if err := runner.Resume(loopCtx, req.toRunConfig(model), cfg, client, toolSchemas, dispatchCtx); err != nil {
				s.Logger.Error("looper resume failed", "error", err)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "resuming",
		})
	}
}

type looperRunRequest struct {
	Goal        string `json:"goal"`
	Work        string `json:"work"`
	Evaluate    string `json:"evaluate"`
	Finish      string `json:"finish"`
	MaxRounds   int    `json:"max_rounds"`
	TargetScore int    `json:"target_score"`
	StallRounds int    `json:"stall_rounds"`
	ProviderID  string `json:"provider_id"`
	Model       string `json:"model"`
	PresetName  string `json:"preset_name"`
}

func normalizeLooperRunRequest(req *looperRunRequest) {
	cfg := req.toRunConfig("")
	desktop.NormalizeLooperRunConfig(&cfg)
	req.MaxRounds = cfg.MaxRounds
	req.TargetScore = cfg.TargetScore
	req.StallRounds = cfg.StallRounds
}

func (req looperRunRequest) toRunConfig(model string) desktop.LooperRunConfig {
	cfg := desktop.LooperRunConfig{
		Goal:        req.Goal,
		Work:        req.Work,
		Evaluate:    req.Evaluate,
		Finish:      req.Finish,
		MaxRounds:   req.MaxRounds,
		TargetScore: req.TargetScore,
		StallRounds: req.StallRounds,
		ProviderID:  req.ProviderID,
		Model:       model,
		PresetName:  req.PresetName,
	}
	desktop.NormalizeLooperRunConfig(&cfg)
	if cfg.Model == "" {
		cfg.Model = req.Model
	}
	return cfg
}

func buildLooperRuntime(s *Server, providerID, model string) (*config.Config, llm.ChatClient, string, *agent.DispatchContext, []openai.Tool, error) {
	s.CfgMu.RLock()
	cfg := s.Cfg
	s.CfgMu.RUnlock()
	if cfg == nil {
		return nil, nil, "", nil, nil, fmt.Errorf("configuration not ready")
	}

	var client llm.ChatClient
	resolvedModel := model
	if providerID != "" {
		for _, p := range cfg.Providers {
			if p.ID == providerID {
				client = llm.NewClientFromProviderWithConfig(cfg, p.Type, p.BaseURL, p.APIKey, p.AccountID)
				if resolvedModel == "" {
					resolvedModel = p.Model
				}
				break
			}
		}
	}
	if client == nil {
		client = s.LLMClient
		if resolvedModel == "" {
			resolvedModel = cfg.LLM.Model
		}
	}
	if client == nil {
		return nil, nil, "", nil, nil, fmt.Errorf("no LLM client available")
	}

	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	dispatchCtx := &agent.DispatchContext{
		Cfg:                cfg,
		Logger:             s.Logger,
		LLMClient:          client,
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
		WorkspaceSearch:    s.WorkspaceSearch,
		MessageSource:      "looper",
	}
	toolSchemas := agent.GetLooperToolSchemas(cfg)
	return cfg, client, resolvedModel, dispatchCtx, toolSchemas, nil
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

		state := runner.State()
		data, _ := json.Marshal(state)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

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
				if !state.Running && !state.Paused && (state.Status == "idle" || state.Status == "stopped" || state.Status == "completed" || state.Status == "max_rounds" || state.Status == "stalled" || state.Status == "failed") {
					idleTicks++
					if idleTicks >= 3 {
						return
					}
				} else {
					idleTicks = 0
				}
			}
		}
	}
}
