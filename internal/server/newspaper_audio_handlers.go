package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/inventory"
	"aurago/internal/newspaper"
)

func newspaperCastAllowed(cfg *config.Config) bool {
	return cfg != nil && cfg.Newspaper.AllowChromecast && cfg.Chromecast.Enabled && !cfg.Chromecast.ReadOnly
}

func (s *Server) newspaperCastDevices(cfg *config.Config) []map[string]string {
	if !newspaperCastAllowed(cfg) || s.InventoryDB == nil {
		return nil
	}
	devices, err := inventory.QueryDevices(s.InventoryDB, "", "chromecast", "")
	if err != nil {
		return nil
	}
	out := make([]map[string]string, 0, len(devices))
	for _, d := range devices {
		out = append(out, map[string]string{"id": d.ID, "name": d.Name})
	}
	return out
}

func (s *Server) castNewspaper(ctx context.Context, id string, file newspaper.AudioFile, before func() error) error {
	cfg := s.ConfigSnapshot()
	if !newspaperAudioWritable(cfg) || !newspaperCastAllowed(cfg) {
		return newspaper.ErrDisabled
	}
	device, ok := chromecastInventoryDevice(s, id)
	if !ok {
		return errors.New("Cast device unavailable")
	}
	return agent.PlayCastRecording(ctx, cfg, device.IPAddress, device.Port, s.InventoryDB, s.Logger, file.Data, file.MIME, func() error {
		live := s.ConfigSnapshot()
		current, ok := chromecastInventoryDevice(s, id)
		if !newspaperAudioWritable(live) || !newspaperCastAllowed(live) || !ok || current.IPAddress != device.IPAddress || current.Port != device.Port {
			return newspaper.ErrDisabled
		}
		return before()
	})
}

func (s *Server) newspaperAudioRoute(w http.ResponseWriter, r *http.Request, id, op string) {
	ctx := r.Context()
	w.Header().Set("Cache-Control", "private, no-store")
	switch {
	case op == "audio" && r.Method == http.MethodGet:
		a, err := s.Newspaper.Store().AudioState(ctx, id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		b, err := s.Newspaper.Store().CastBookings(ctx, id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 200, map[string]any{"audio": a, "casts": b})
	case op == "audio" && r.Method == http.MethodPost:
		a, err := s.Newspaper.GenerateAudio(ctx, id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 202, a)
	case op == "audio/file" && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		e, err := s.Newspaper.Get(ctx, id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		a, err := s.Newspaper.Store().Recording(ctx, id)
		if err != nil {
			newspaperError(w, err)
			return
		}
		w.Header().Set("Content-Type", a.MIME)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, a.Name(e), e.CreatedAt, bytes.NewReader(a.Data))
	case op == "cast" && r.Method == http.MethodPost:
		var body struct {
			Date string `json:"date"`
			Time string `json:"time"`
		}
		if err := detectiveDecode(w, r, &body); err != nil {
			newspaperError(w, err)
			return
		}
		b, err := s.Newspaper.ScheduleCast(ctx, id, body.Date, body.Time, strings.TrimSpace(r.Header.Get("Idempotency-Key")))
		if err != nil {
			newspaperError(w, err)
			return
		}
		newspaperJSON(w, 202, b)
	default:
		jsonError(w, "Method not allowed", 405)
	}
}
