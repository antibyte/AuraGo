package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aurago/internal/agentmail"
	"aurago/internal/config"
)

func handleAgentMailStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.Cfg.AgentMail
		running := false
		if s.AgentMailService != nil {
			running = s.AgentMailService.Running()
		}
		w.Header().Set("Content-Type", "application/json")
		if !cfg.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "disabled",
				"message": "AgentMail integration is not enabled",
				"running": running,
			})
			return
		}
		if cfg.APIKey == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "no_api_key",
				"message": "AgentMail API key is not configured",
				"running": running,
			})
			return
		}
		status := "ok"
		if cfg.RelayToAgent && cfg.InboxID == "" {
			status = "no_inbox"
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         status,
			"enabled":        cfg.Enabled,
			"readonly":       cfg.ReadOnly,
			"inbox_id":       cfg.InboxID,
			"relay_to_agent": cfg.RelayToAgent,
			"use_websocket":  cfg.UseWebSocket,
			"running":        running,
			"base_url":       cfg.BaseURL,
		})
	}
}

func handleAgentMailTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		cfg := agentmail.ConfigFromAppConfig(s.Cfg.AgentMail)
		if cfg.APIKey == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": "AgentMail API key is not configured"})
			return
		}
		client, err := agentmail.NewClient(agentmail.ClientConfig{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey})
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if cfg.InboxID != "" {
			inbox, err := client.GetInbox(ctx, cfg.InboxID)
			if err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "message": "Connection successful", "inbox": inbox})
			return
		}
		res, err := client.ListInboxes(ctx, agentmail.ListInboxesOptions{Limit: 10})
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "ok",
			"message":     "Connection successful",
			"inbox_count": len(res.Inboxes),
			"inboxes":     res.Inboxes,
		})
	}
}

func (s *Server) configureAgentMailRelay(cfg *config.Config) {
	s.AgentMailMu.Lock()
	defer s.AgentMailMu.Unlock()
	if s.AgentMailService != nil {
		s.AgentMailService.Stop(context.Background())
		s.AgentMailService = nil
	}
	if cfg == nil || cfg.EggMode.Enabled || !cfg.AgentMail.Enabled || !cfg.AgentMail.RelayToAgent {
		return
	}
	svc := agentmail.NewService(agentmail.ServiceConfig{
		Config:      agentmail.ConfigFromAppConfig(cfg.AgentMail),
		Logger:      s.Logger,
		Guardian:    s.Guardian,
		LLMGuardian: s.LLMGuardian,
		ScanEmails:  cfg.LLMGuardian.ScanEmails,
		Notify:      s.notifyAgentMailLoopback,
	})
	if err := svc.Start(context.Background()); err != nil {
		s.Logger.Warn("[AgentMail] Relay not started", "error", err)
		return
	}
	s.AgentMailService = svc
	s.Logger.Info("[AgentMail] Relay started", "inbox_id", cfg.AgentMail.InboxID, "websocket", cfg.AgentMail.UseWebSocket)
}

func (s *Server) notifyAgentMailLoopback(ctx context.Context, prompt string) error {
	if s == nil || s.Cfg == nil {
		return fmt.Errorf("server config is not available")
	}
	url := InternalAPIURL(s.Cfg) + "/v1/chat/completions"
	payload, err := json.Marshal(map[string]interface{}{
		"model": s.Cfg.LLM.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return err
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	client := NewInternalHTTPClient(3 * time.Minute)
	resp, err := DoInternalRequestWithStartupRetry(ctx, client, http.MethodPost, url, payload, headers, 10*time.Second)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return fmt.Errorf("loopback notification failed: %s %s", resp.Status, buf.String())
	}
	return nil
}
