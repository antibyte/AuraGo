package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/agentmail"
	"aurago/internal/newspaper"
	"aurago/internal/telegram"
)

func registerNewspaperRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/newspaper/", s.handleNewspaper)
}

func newspaperJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newspaperError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, newspaper.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, newspaper.ErrConflict), errors.Is(err, newspaper.ErrBusy):
		status = http.StatusConflict
	case errors.Is(err, newspaper.ErrDisabled):
		status = http.StatusForbidden
	}
	newspaperJSON(w, status, map[string]string{"error": err.Error()})
}

func newspaperOriginOK(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		return true
	}
	return checkCSRFOriginWithPolicy(r, true)
}

func (s *Server) handleNewspaper(w http.ResponseWriter, r *http.Request) {
	if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
		return
	}
	if !newspaperOriginOK(r) {
		newspaperJSON(w, http.StatusForbidden, map[string]string{"error": "same-origin request required"})
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled || s.Newspaper == nil {
		newspaperJSON(w, 503, map[string]string{"error": "Newspaper unavailable"})
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/newspaper/"), "/")
	if path == "capabilities" && r.Method == http.MethodGet {
		p, _ := s.Newspaper.Profile(r.Context())
		accounts := []map[string]string{}
		for _, a := range cfg.EmailAccounts {
			if !a.Disabled && !a.ReadOnly && a.SMTPHost != "" {
				accounts = append(accounts, map[string]string{"id": a.ID, "name": a.Name})
			}
		}
		if cfg.AgentMail.Enabled && !cfg.AgentMail.ReadOnly && cfg.AgentMail.APIKey != "" && cfg.AgentMail.InboxID != "" {
			accounts = append(accounts, map[string]string{"id": "agentmail", "name": "AgentMail"})
		}
		var next time.Time
		if p.Daily {
			next, _ = s.Newspaper.NextRun(r.Context())
		}
		localDate := ""
		if loc, err := time.LoadLocation(p.TimeZone); err == nil {
			localDate = time.Now().In(loc).Format("2006-01-02")
		}
		accountReady := false
		for _, account := range accounts {
			if account["id"] == p.EmailAccountID {
				accountReady = true
				break
			}
		}
		researchReady := s.newspaperSkillReady && cfg.Agent.AllowNetworkRequests && ((cfg.BraveSearch.Enabled && cfg.BraveSearch.APIKey != "") || len(p.RSSFeeds) > 0) && cfg.Tools.WebScraper.Enabled && s.LLMClient != nil && cfg.LLM.Model != ""
		newspaperJSON(w, 200, map[string]any{"enabled": cfg.Newspaper.Enabled, "read_only": cfg.Newspaper.ReadOnly || cfg.VirtualDesktop.ReadOnly, "research_ready": researchReady, "email_allowed": cfg.Newspaper.AllowEmail, "email_ready": cfg.Newspaper.AllowEmail && p.EmailVerified && p.EmailTo != "" && accountReady, "telegram_allowed": cfg.Newspaper.AllowTelegram, "telegram_ready": cfg.Newspaper.AllowTelegram && cfg.Telegram.BotToken != "" && cfg.Telegram.UserID != 0, "email_accounts": accounts, "daily": p.Daily, "local_date": localDate, "next_run": next, "max_minutes": cfg.Newspaper.MaxMinutes, "max_pages": cfg.Newspaper.MaxPages, "max_editions": cfg.Newspaper.MaxEditions})
		return
	}
	if path == "profile" {
		switch r.Method {
		case http.MethodGet:
			p, err := s.Newspaper.Profile(r.Context())
			if err != nil {
				newspaperError(w, err)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf("\"%d\"", p.Version))
			newspaperJSON(w, 200, p)
		case http.MethodPut:
			if !cfg.Newspaper.Enabled || cfg.Newspaper.ReadOnly || cfg.VirtualDesktop.ReadOnly {
				newspaperError(w, newspaper.ErrDisabled)
				return
			}
			var p newspaper.Profile
			if err := detectiveDecode(w, r, &p); err != nil {
				newspaperError(w, err)
				return
			}
			if r.Header.Get("If-Match") != fmt.Sprintf("\"%d\"", p.Version) {
				newspaperError(w, newspaper.ErrConflict)
				return
			}
			out, err := s.Newspaper.SaveProfile(r.Context(), p)
			if err != nil {
				newspaperError(w, err)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf("\"%d\"", out.Version))
			newspaperJSON(w, 200, out)
		default:
			jsonError(w, "Method not allowed", 405)
		}
		return
	}
	if path == "email/challenge" && r.Method == http.MethodPost {
		if !cfg.Newspaper.Enabled || cfg.Newspaper.ReadOnly || cfg.VirtualDesktop.ReadOnly || !cfg.Newspaper.AllowEmail {
			newspaperError(w, newspaper.ErrDisabled)
			return
		}
		p, err := s.Newspaper.Profile(r.Context())
		if err != nil {
			newspaperError(w, err)
			return
		}
		if p.EmailTo == "" || p.EmailAccountID == "" {
			newspaperError(w, errors.New("save an email destination and account first"))
			return
		}
		code, err := s.Newspaper.Store().NewEmailChallenge(r.Context(), p.EmailTo, p.EmailAccountID)
		if err != nil {
			if errors.Is(err, newspaper.ErrChallengePending) {
				w.Header().Set("Retry-After", "60")
				newspaperJSON(w, http.StatusConflict, map[string]any{"code": "challenge_pending", "error": "Wait about one minute before requesting another confirmation code"})
				return
			}
			newspaperError(w, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		if _, err = s.newspaperSendEmail(ctx, p.EmailAccountID, p.EmailTo, "Confirm Newspaper delivery", "Your Newspaper confirmation code is "+code+". It expires in 10 minutes.", "<p>Your Newspaper confirmation code is <b>"+code+"</b>. It expires in 10 minutes.</p>"); err != nil {
			var safe *newspaper.SafeDeliveryError
			if errors.As(err, &safe) {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
				removed, cleanupErr := s.Newspaper.Store().CancelEmailChallenge(cleanupCtx, p.EmailTo, p.EmailAccountID, code)
				cleanupCancel()
				if cleanupErr == nil && removed {
					if errors.Is(err, errNewspaperAgentMailBounce) {
						newspaperJSON(w, http.StatusBadGateway, map[string]any{"code": "agentmail_bounce_blocked", "provider_status": http.StatusForbidden, "error": "AgentMail blocked this recipient after a bounce; request a suppression review from AgentMail support"})
						return
					}
					var apiErr *agentmail.APIError
					if errors.As(err, &apiErr) {
						newspaperJSON(w, http.StatusBadGateway, map[string]any{"code": "agentmail_rejected", "provider_status": apiErr.StatusCode, "error": fmt.Sprintf("AgentMail rejected the confirmation email (HTTP %d); check the sender configuration and limits before retrying", apiErr.StatusCode)})
						return
					}
					newspaperJSON(w, http.StatusBadGateway, map[string]any{"code": "email_send_safe", "error": "Confirmation email was not sent; check the sending account before retrying"})
					return
				}
			}
			newspaperJSON(w, http.StatusBadGateway, map[string]any{"code": "email_send_uncertain", "error": "Confirmation email delivery is uncertain; check the mailbox before requesting another code"})
			return
		}
		newspaperJSON(w, 202, map[string]string{"status": "sent"})
		return
	}
	if path == "email/confirm" && r.Method == http.MethodPost {
		if !cfg.Newspaper.Enabled || cfg.Newspaper.ReadOnly || cfg.VirtualDesktop.ReadOnly || !cfg.Newspaper.AllowEmail {
			newspaperError(w, newspaper.ErrDisabled)
			return
		}
		var req struct {
			Code string `json:"code"`
		}
		if err := detectiveDecode(w, r, &req); err != nil {
			newspaperError(w, err)
			return
		}
		p, err := s.Newspaper.Profile(r.Context())
		if err != nil {
			newspaperError(w, err)
			return
		}
		out, err := s.Newspaper.Store().ConfirmEmail(r.Context(), p.EmailTo, p.EmailAccountID, strings.ToUpper(strings.TrimSpace(req.Code)))
		if err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 200, out)
		return
	}
	if path == "telegram/test" && r.Method == http.MethodPost {
		if !cfg.Newspaper.Enabled || cfg.Newspaper.ReadOnly || cfg.VirtualDesktop.ReadOnly || !cfg.Newspaper.AllowTelegram || cfg.Telegram.BotToken == "" || cfg.Telegram.UserID == 0 {
			newspaperError(w, newspaper.ErrDisabled)
			return
		}
		if err := telegram.SendTestMessage(cfg, time.Now()); err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 200, map[string]string{"status": "sent"})
		return
	}
	if path == "editions" {
		switch r.Method {
		case http.MethodGet:
			items, err := s.Newspaper.List(r.Context(), 50)
			if err != nil {
				newspaperError(w, err)
				return
			}
			out := make([]map[string]any, 0, len(items))
			for _, e := range items {
				headlines := make([]string, 0, len(e.Stories))
				sections := make([]string, 0, len(e.Stories))
				for _, story := range e.Stories {
					headlines = append(headlines, story.Headline)
					sections = append(sections, story.Section)
				}
				out = append(out, map[string]any{"id": e.ID, "local_date": e.LocalDate, "revision": e.Revision, "title": e.Title, "created_at": e.CreatedAt, "hash": e.Hash, "partial": e.Partial, "stories": len(e.Stories), "lead": e.Stories[0].Headline, "headlines": headlines, "sections": sections})
			}
			run, _ := s.Newspaper.LatestRun(r.Context())
			newspaperJSON(w, 200, map[string]any{"editions": out, "latest_run": run})
		case http.MethodPost:
			var req struct {
				NewRevision    bool   `json:"new_revision"`
				CorrectionNote string `json:"correction_note"`
			}
			if err := detectiveDecode(w, r, &req); err != nil {
				newspaperError(w, err)
				return
			}
			run, err := s.Newspaper.StartWithCorrection(r.Context(), req.NewRevision, req.CorrectionNote)
			if err != nil {
				newspaperError(w, err)
				return
			}
			newspaperJSON(w, 202, run)
		default:
			jsonError(w, "Method not allowed", 405)
		}
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] != "editions" {
		http.NotFound(w, r)
		return
	}
	id := parts[1]
	if len(parts) == 2 && r.Method == http.MethodGet {
		e, err := s.Newspaper.Get(r.Context(), id)
		if errors.Is(err, newspaper.ErrNotFound) {
			run, runErr := s.Newspaper.Run(r.Context(), id)
			if runErr == nil {
				newspaperJSON(w, 200, map[string]any{"run": run})
				return
			}
		}
		if err != nil {
			newspaperError(w, err)
			return
		}
		w.Header().Set("ETag", "\""+e.Hash+"\"")
		newspaperJSON(w, 200, e)
		return
	}
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	switch parts[2] {
	case "events":
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		events, err := s.Newspaper.Events(r.Context(), id, after)
		if err != nil {
			newspaperError(w, err)
			return
		}
		sources, err := s.Newspaper.RunSources(r.Context(), id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 200, map[string]any{"events": events, "sources": sources})
	case "stop":
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if err := s.Newspaper.Stop(r.Context(), id); err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 202, map[string]string{"status": "stopping"})
	case "deliveries":
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", 405)
			return
		}
		d, err := s.Newspaper.Deliveries(r.Context(), id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 200, map[string]any{"deliveries": d})
	case "deliver":
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		var req struct {
			Channel string `json:"channel"`
			Kind    string `json:"kind"`
		}
		if err := detectiveDecode(w, r, &req); err != nil {
			newspaperError(w, err)
			return
		}
		if req.Kind == "" {
			req.Kind = "manual"
		}
		if req.Kind != "manual" && req.Kind != "test" {
			newspaperError(w, errors.New("invalid delivery kind"))
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(key) < 16 || len(key) > 128 || strings.ContainsAny(key, " \t\r\n") {
			newspaperError(w, errors.New("a unique Idempotency-Key header is required"))
			return
		}
		d, err := s.Newspaper.Deliver(r.Context(), id, req.Channel, req.Kind, key)
		if err != nil {
			if d.ID != 0 {
				newspaperJSON(w, http.StatusAccepted, d)
			} else {
				newspaperError(w, err)
			}
			return
		}
		newspaperJSON(w, 200, d)
	case "export":
		if r.Method != http.MethodGet || r.URL.Query().Get("format") != "pdf" {
			jsonError(w, "Unsupported export", 405)
			return
		}
		e, err := s.Newspaper.Get(r.Context(), id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		data, err := newspaper.PDF(e)
		if err != nil {
			newspaperJSON(w, 422, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"newspaper-%s-r%d.pdf\"", e.LocalDate, e.Revision))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
	default:
		http.NotFound(w, r)
	}
}
