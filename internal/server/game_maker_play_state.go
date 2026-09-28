package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net/http"

	"aurago/internal/gamemaker"
)

func handleGameMakerPlayer(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	if !requireDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	p, err := s.GameMaker.GetProject(r.Context(), projectID)
	if err != nil || p.Variant != "voxel" || p.CurrentRevision <= 0 {
		http.NotFound(w, r)
		return
	}
	t, err := template.ParseFS(uiFiles, "game-maker-player.html")
	if err != nil {
		http.Error(w, "Game player resources unavailable", http.StatusServiceUnavailable)
		return
	}
	data := uiTemplateData(normalizeLang(s.ConfigSnapshot().Server.UILanguage), "desktop")
	setTemplateDataJSON(data, map[string]any{"projectID": p.ID}, "desktop")
	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		http.Error(w, "Game player unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; frame-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'self'")
	w.Write(body.Bytes())
}

func handleGameMakerPlayState(w http.ResponseWriter, r *http.Request, s *Server, projectID string) {
	scope := desktopScopeRead
	if r.Method != http.MethodGet {
		scope = desktopScopeWrite
	}
	if !requireDesktopPermission(s, w, r, scope) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	token := r.Header.Get("X-Game-Maker-Play")
	var result gamemaker.PlayState
	var err error
	switch r.Method {
	case http.MethodGet:
		result, err = s.GameMaker.GetPlayState(r.Context(), projectID, token)
	case http.MethodPut, http.MethodDelete:
		r.Body = http.MaxBytesReader(w, r.Body, gamemaker.MaxPlayStateBytes)
		var request struct {
			Version int64           `json:"version"`
			State   json.RawMessage `json:"state"`
		}
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if err = d.Decode(&request); err != nil {
			var large *http.MaxBytesError
			if errors.As(err, &large) {
				jsonError(w, "Voxel save exceeds 4 MiB", http.StatusRequestEntityTooLarge)
			} else {
				jsonError(w, "Invalid voxel save request", http.StatusBadRequest)
			}
			return
		}
		if d.Decode(new(any)) != io.EOF {
			jsonError(w, "Expected one voxel save request", http.StatusBadRequest)
			return
		}
		result, err = s.GameMaker.WritePlayState(r.Context(), projectID, token, request.Version, request.State, r.Method == http.MethodDelete)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, gamemaker.ErrPlayStateConflict):
			status = http.StatusConflict
		case errors.Is(err, gamemaker.ErrInvalidToken), errors.Is(err, gamemaker.ErrReadOnly):
			status = http.StatusForbidden
		case errors.Is(err, gamemaker.ErrDisabled), errors.Is(err, gamemaker.ErrNotFound):
			status = http.StatusNotFound
		}
		jsonError(w, err.Error(), status)
		return
	}
	writeGameMakerJSON(w, http.StatusOK, result)
}
