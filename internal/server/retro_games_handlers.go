package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/retrogames"
)

const retroGamesPrefix = "/api/desktop/retro-games/"

func registerRetroGamesRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc(retroGamesPrefix, s.handleRetroGames)
}

func retroGamesJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func retroGamesError(w http.ResponseWriter, status int, code, message string) {
	retroGamesJSON(w, status, map[string]string{"error": message, "code": code})
}

func retroGamesMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	retroGamesError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
}

// retroGamesReadMethod reports whether the request only reads. HEAD is served
// like GET; net/http drops the body.
func retroGamesReadMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

func (s *Server) handleRetroGames(w http.ResponseWriter, r *http.Request) {
	scope := desktopScopeRead
	if !retroGamesReadMethod(r) {
		scope = desktopScopeWrite
	}
	if !authenticateDesktopPermission(s, w, r, scope) {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		retroGamesError(w, http.StatusServiceUnavailable, "desktop_unavailable", "Desktop unavailable")
		return
	}
	svc := s.RetroGames
	if !cfg.RetroGames.Enabled || svc == nil || !svc.Enabled() {
		retroGamesError(w, http.StatusServiceUnavailable, "disabled", "Retro Games is switched off")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, retroGamesPrefix), "/")
	route := parts[0]
	id := ""
	if len(parts) == 2 {
		id = parts[1]
	} else if len(parts) > 2 {
		retroGamesError(w, http.StatusNotFound, "not_found", "Not found")
		return
	}
	ctx := r.Context()
	switch {
	case route == "status" && id == "":
		if !retroGamesReadMethod(r) {
			retroGamesMethodNotAllowed(w, "GET, HEAD")
			return
		}
		status, err := svc.Status(ctx)
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		retroGamesJSON(w, http.StatusOK, status)
	case route == "sync" && id == "":
		if r.Method != http.MethodPost {
			retroGamesMethodNotAllowed(w, "POST")
			return
		}
		if !checkDesktopOperation(s, w, r, desktopWrite) {
			return
		}
		if err := svc.StartSync(); err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		retroGamesJSON(w, http.StatusAccepted, map[string]bool{"started": true})
	case route == "search" && id == "":
		if !retroGamesReadMethod(r) {
			retroGamesMethodNotAllowed(w, "GET, HEAD")
			return
		}
		q, err := retroGamesQueryFromRequest(r)
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		res, err := svc.Search(ctx, q)
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		retroGamesJSON(w, http.StatusOK, res)
	case route == "hall-of-fame" && id == "":
		if !retroGamesReadMethod(r) {
			retroGamesMethodNotAllowed(w, "GET, HEAD")
			return
		}
		games, err := svc.HallOfFame(ctx, r.URL.Query().Get("platform"))
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		retroGamesJSON(w, http.StatusOK, map[string]any{"games": games})
	case route == "library" && id == "":
		if !retroGamesReadMethod(r) {
			retroGamesMethodNotAllowed(w, "GET, HEAD")
			return
		}
		lib, err := svc.Library(ctx)
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		retroGamesJSON(w, http.StatusOK, lib)
	case route == "games" && id != "":
		if !retroGamesReadMethod(r) {
			retroGamesMethodNotAllowed(w, "GET, HEAD")
			return
		}
		g, err := svc.Game(ctx, id)
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		retroGamesJSON(w, http.StatusOK, map[string]any{"game": g})
	case (route == "favorites" || route == "broken") && id != "":
		on := r.Method == http.MethodPut
		if !on && r.Method != http.MethodDelete {
			retroGamesMethodNotAllowed(w, "PUT, DELETE")
			return
		}
		if !checkDesktopOperation(s, w, r, desktopWrite) {
			return
		}
		kind := retrogames.LibraryFavorite
		if route == "broken" {
			kind = retrogames.LibraryBroken
		}
		// Opening the index, resolving the game and waiting for the store's
		// write lock (a running sync transaction holds it for seconds) all
		// happen here, before the desktop publication gate: only the single
		// write, which no longer waits for anything, runs inside it.
		commit, release, err := svc.PrepareLibraryChange(r.Context(), kind, id, on)
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		defer release()
		if err := publishDesktopResult(r.Context(), func() error { return commit(r.Context()) }); err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		key := "favorite"
		if route == "broken" {
			key = "broken"
		}
		retroGamesJSON(w, http.StatusOK, map[string]bool{key: on})
	case route == "plays" && id != "":
		if r.Method != http.MethodPost {
			retroGamesMethodNotAllowed(w, "POST")
			return
		}
		if cfg.VirtualDesktop.ReadOnly {
			retroGamesJSON(w, http.StatusOK, map[string]bool{"recorded": false})
			return
		}
		if !checkDesktopOperation(s, w, r, desktopWrite) {
			return
		}
		// Only the write runs inside the publication gate (see above).
		commit, release, err := svc.PrepareLibraryChange(r.Context(), retrogames.LibraryPlay, id, true)
		if err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		defer release()
		if err := publishDesktopResult(r.Context(), func() error { return commit(r.Context()) }); err != nil {
			s.writeRetroGamesError(w, err)
			return
		}
		retroGamesJSON(w, http.StatusOK, map[string]bool{"recorded": true})
	default:
		retroGamesError(w, http.StatusNotFound, "not_found", "Not found")
	}
}

func retroGamesQueryFromRequest(r *http.Request) (retrogames.Query, error) {
	v := r.URL.Query()
	q := retrogames.Query{
		Text:          v.Get("q"),
		Platform:      v.Get("platform"),
		System:        v.Get("system"),
		Sort:          v.Get("sort"),
		IncludeBroken: v.Get("include_broken") == "1" || v.Get("include_broken") == "true",
	}
	for name, dst := range map[string]*int{"decade": &q.Decade, "offset": &q.Offset, "limit": &q.Limit} {
		raw := strings.TrimSpace(v.Get(name))
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return retrogames.Query{}, retrogames.ErrInvalidQuery
		}
		*dst = n
	}
	return q.Normalize()
}

func (s *Server) writeRetroGamesError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		// The run was revoked (read-only switch, config reload) or the client
		// left; nothing failed, so nothing is logged.
		writeDesktopPolicyError(w, "desktop_readonly", "The desktop action was revoked.")
	case errors.Is(err, context.DeadlineExceeded):
		retroGamesError(w, http.StatusServiceUnavailable, "timeout", "Retro Games timed out")
	case errors.Is(err, retrogames.ErrDisabled):
		retroGamesError(w, http.StatusServiceUnavailable, "disabled", "Retro Games is switched off")
	case errors.Is(err, retrogames.ErrInvalidQuery):
		retroGamesError(w, http.StatusBadRequest, "invalid_request", "Invalid search request")
	case errors.Is(err, retrogames.ErrInvalidIdentifier):
		retroGamesError(w, http.StatusBadRequest, "invalid_identifier", "Invalid game identifier")
	case errors.Is(err, retrogames.ErrUnknownGame):
		retroGamesError(w, http.StatusNotFound, "game_not_found", "Game not found")
	case errors.Is(err, retrogames.ErrLibraryFull):
		retroGamesError(w, http.StatusConflict, "library_full", "This list is full")
	case errors.Is(err, retrogames.ErrSyncRunning):
		retroGamesError(w, http.StatusConflict, "sync_running", "The index is already being updated")
	default:
		if s != nil && s.Logger != nil {
			s.Logger.Warn("[RetroGames] Request failed", "error", err)
		}
		retroGamesError(w, http.StatusInternalServerError, "retro_games_failed", "Retro Games failed")
	}
}
