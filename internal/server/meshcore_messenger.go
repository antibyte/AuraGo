package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"aurago/internal/config"
	"aurago/internal/meshcore"
	"gopkg.in/yaml.v3"
)

// The parent MeshCore router applies requireAdmin to reads and writes alike.
func (s *Server) handleMeshCoreMessenger(w http.ResponseWriter, r *http.Request, action string) {
	w.Header().Set("Cache-Control", "no-store")
	if s.MeshCore == nil {
		jsonError(w, "unavailable", 503)
		return
	}
	read := action == "bootstrap" || action == "conversations" || action == "messages"
	if (read && r.Method != "GET") || (!read && r.Method != "POST") {
		jsonError(w, "method_not_allowed", 405)
		return
	}
	if !read && !sameOriginOrNoOrigin(r) {
		jsonError(w, "same_origin_required", 403)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var body struct {
		meshcore.EditRequest
		ID              string `json:"id"`
		Text            string `json:"text"`
		Read            int64  `json:"read"`
		Favorite        *bool  `json:"favorite"`
		Muted           *bool  `json:"muted"`
		Clear           bool   `json:"clear"`
		HistoryDays     int    `json:"history_days"`
		HistoryMessages int    `json:"history_messages"`
	}
	if !read {
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if d.Decode(&body) != nil || d.Decode(&struct{}{}) != io.EOF {
			jsonError(w, "invalid_request", 400)
			return
		}
	}
	fail := func(err error) {
		code := "operation_failed"
		switch err.Error() {
		case "invalid_request", "invalid_text", "invalid_target", "invalid_contact", "invalid_channel", "invalid_invitation", "unsupported_invitation", "contact_exists", "channels_full", "binding_required", "not_connected", "busy", "idempotency_conflict", "send_ledger_full", "outcome_unknown", "config_unavailable", "message_unavailable":
			code = err.Error()
		}
		jsonError(w, code, 409)
	}
	switch action {
	case "bootstrap", "conversations":
		items, err := s.MeshCore.Conversations()
		if err != nil {
			fail(err)
			return
		}
		cfg := s.ConfigSnapshot().MeshCore
		writeJSON(w, map[string]interface{}{"conversations": items, "status": s.MeshCore.Status(), "enabled": cfg.Enabled, "history_days": cfg.HistoryDays, "history_messages": cfg.HistoryMessages, "channel_text_limit": s.MeshCore.ChannelTextLimit()})
	case "messages":
		before, err := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
		if r.URL.Query().Get("before") == "" {
			before = 0
			err = nil
		}
		if err != nil {
			jsonError(w, "invalid_request", 400)
			return
		}
		items, err := s.MeshCore.ChatMessages(r.URL.Query().Get("conversation"), before, r.URL.Query().Get("q"))
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]interface{}{"messages": items})
	case "send":
		id, err := s.MeshCore.SendManual(ctx, body.ID, body.Conversation, body.Text)
		if err != nil {
			fail(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]string{"id": id})
	case "conversation":
		if err := s.MeshCore.UpdateConversation(body.Conversation, body.Read, body.Favorite, body.Muted, body.Clear); err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case "reveal":
		text, err := s.MeshCore.RevealMessage(body.ID)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]string{"text": text})
	case "invitation":
		link, err := s.MeshCore.Invitation(ctx, body.Identity, body.Conversation)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]string{"invitation": link})
	case "manage":
		s.CfgSaveMu.Lock()
		err := s.MeshCore.Edit(ctx, body.EditRequest, s.persistMeshCoreSection)
		s.CfgSaveMu.Unlock()
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case "settings":
		s.CfgSaveMu.Lock()
		cfg := s.ConfigSnapshot()
		next := cfg.MeshCore
		next.TrustedNodes = slices.Clone(next.TrustedNodes)
		next.SendNodes = slices.Clone(next.SendNodes)
		next.Channels = slices.Clone(next.Channels)
		next.HistoryDays = body.HistoryDays
		next.HistoryMessages = body.HistoryMessages
		err := next.Normalize()
		if err == nil {
			err = s.persistMeshCoreSection(next)
		}
		if err == nil {
			err = s.MeshCore.Configure(next, cfg.Runtime.IsDocker)
		}
		s.CfgSaveMu.Unlock()
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		jsonError(w, "not_found", 404)
	}
}

// Caller owns CfgSaveMu. Preserve unrelated YAML and the live configuration;
// channel secrets and invitations are never members of this configuration type.
func (s *Server) persistMeshCoreSection(next meshcore.Config) error {
	cfg := s.ConfigSnapshot()
	b, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		return err
	}
	var raw map[string]interface{}
	if err = yaml.Unmarshal(b, &raw); err != nil {
		return err
	}
	raw = normalizeConfigYAMLMap(raw)
	section, err := yaml.Marshal(next)
	if err != nil {
		return err
	}
	var value map[string]interface{}
	if err = yaml.Unmarshal(section, &value); err != nil {
		return err
	}
	raw["meshcore"] = value
	b, err = yaml.Marshal(raw)
	if err != nil {
		return err
	}
	if err = config.WriteFileAtomic(cfg.ConfigPath, b, 0600); err != nil {
		return err
	}
	s.CfgMu.Lock()
	updated := *cfg
	updated.MeshCore = next
	s.replaceConfigSnapshot(&updated)
	s.CfgMu.Unlock()
	return nil
}
