package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

func registerLLMRouterRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle("/api/llm-router/status", requireAdmin(s, handleLLMRouterStatus(s)))
	mux.Handle("/api/llm-router/preview", requireAdmin(s, handleLLMRouterPreview(s)))
}

func handleLLMRouterStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": cfg.LLMRouter.Enabled, "helper_available": llm.IsHelperLLMAvailable(cfg), "default_provider": cfg.LLM.Provider, "default_model": cfg.LLM.Model, "stats": agent.SnapshotTaskRouterStats()})
	}
}

func handleLLMRouterPreview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "https" && u.Scheme != "http") {
				jsonError(w, "Forbidden origin", http.StatusForbidden)
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			jsonError(w, "Forbidden origin", http.StatusForbidden)
			return
		}
		var input struct {
			Text   string `json:"text"`
			Helper bool   `json:"helper"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Text) == "" || len(input.Text) > 12000 {
			jsonError(w, "Invalid preview input", http.StatusBadRequest)
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			jsonError(w, "Invalid preview input", http.StatusBadRequest)
			return
		}
		cfg := s.ConfigSnapshot().Clone()
		if !input.Helper {
			cfg.LLMRouter.HelperFallback = false
		}
		run := agent.RunConfig{Config: cfg, Logger: s.Logger, LLMClient: s.LLMClient, BudgetTracker: s.BudgetTracker, SessionID: "router-preview", UserIntent: input.Text, TaskRoutingMode: "auto", TaskRoutingPreview: true}
		req := openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: input.Text}}}
		if err := agent.PrepareTaskRouting(r.Context(), &req, &run); err != nil {
			jsonError(w, "Preview cancelled", http.StatusRequestTimeout)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": cfg.LLMRouter.Enabled, "decision": run.TaskRouting})
	}
}

func taskRoutingHasImages(messages []openai.ChatCompletionMessage) bool {
	for _, message := range messages {
		for _, part := range message.MultiContent {
			if part.Type == openai.ChatMessagePartTypeImageURL {
				return true
			}
		}
		for _, match := range attachmentPathRe.FindAllStringSubmatch(message.Content, -1) {
			if len(match) > 1 && imageMimeType(strings.ToLower(filepath.Ext(cleanMatchedAttachmentPath(match[1])))) != "" {
				return true
			}
		}
	}
	return false
}
