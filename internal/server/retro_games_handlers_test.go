package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/retrogames"
	"aurago/internal/retrogames/retrogamestest"
)

type retroGamesHarness struct {
	t          *testing.T
	s          *Server
	readToken  string
	writeToken string
}

func newRetroGamesHarness(t *testing.T) *retroGamesHarness {
	t.Helper()
	s, readToken, writeToken := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.RetroGames.Enabled = true
	s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{BySource: map[string][]retrogames.Item{
		retrogames.PlatformMSDOS: {retrogamestest.Item("msdos_Prince_of_Persia_1990", "Prince of Persia", "dosbox", 1990, 99)},
	}})
	return &retroGamesHarness{t: t, s: s, readToken: readToken, writeToken: writeToken}
}

func (h *retroGamesHarness) do(method, path, token string) (*httptest.ResponseRecorder, map[string]any) {
	h.t.Helper()
	r := httptest.NewRequest(method, "/api/desktop/retro-games/"+path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.s.handleRetroGames(w, r)
	var body map[string]any
	if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		_ = json.Unmarshal(w.Body.Bytes(), &body)
	}
	return w, body
}

func (h *retroGamesHarness) waitIdle() {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.s.RetroGames.WaitIdle(ctx); err != nil {
		h.t.Fatal(err)
	}
}

func TestRetroGamesAPIGates(t *testing.T) {
	h := newRetroGamesHarness(t)
	h.s.Cfg.RetroGames.Enabled = false
	if w, body := h.do(http.MethodGet, "status", h.readToken); w.Code != http.StatusServiceUnavailable || body["code"] != "disabled" {
		t.Fatalf("disabled: %d %v", w.Code, body)
	}
	h.s.Cfg.RetroGames.Enabled = true
	h.s.Cfg.VirtualDesktop.Enabled = false
	if w, body := h.do(http.MethodGet, "status", h.readToken); w.Code != http.StatusServiceUnavailable || body["code"] != "desktop_unavailable" {
		t.Fatalf("desktop off: %d %v", w.Code, body)
	}
	h.s.Cfg.VirtualDesktop.Enabled = true
	if w, _ := h.do(http.MethodPut, "favorites/msdos_Prince_of_Persia_1990", h.readToken); w.Code != http.StatusForbidden {
		t.Fatalf("read token on PUT: %d", w.Code)
	}
	if w, body := h.do(http.MethodPost, "status", h.writeToken); w.Code != http.StatusMethodNotAllowed || body["code"] != "method_not_allowed" {
		t.Fatalf("wrong method: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodGet, "nope", h.readToken); w.Code != http.StatusNotFound || body["code"] != "not_found" {
		t.Fatalf("unknown route: %d %v", w.Code, body)
	}
}

func TestRetroGamesAPIStatusSearchAndHallOfFame(t *testing.T) {
	h := newRetroGamesHarness(t)
	w, body := h.do(http.MethodGet, "status", h.readToken)
	if w.Code != http.StatusOK || body["enabled"] != true || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status: %d %v %q", w.Code, body, w.Header().Get("Cache-Control"))
	}
	h.waitIdle()
	w, body = h.do(http.MethodGet, "search?q=persia&platform=msdos&sort=popular&limit=10", h.readToken)
	if w.Code != http.StatusOK || body["total"].(float64) != 1 {
		t.Fatalf("search: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodGet, "search?decade=1985", h.readToken); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("bad decade: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodGet, "search?decade=abc", h.readToken); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("non-numeric decade: %d %v", w.Code, body)
	}
	w, body = h.do(http.MethodGet, "hall-of-fame?platform=zx", h.readToken)
	games, _ := body["games"].([]any)
	if w.Code != http.StatusOK || len(games) != 5 {
		t.Fatalf("hall of fame zx: %d %d", w.Code, len(games))
	}
	if w, body := h.do(http.MethodGet, "hall-of-fame?platform=nes", h.readToken); w.Code != http.StatusBadRequest || body["code"] != "invalid_request" {
		t.Fatalf("bad platform: %d %v", w.Code, body)
	}
}

func TestRetroGamesAPIGames(t *testing.T) {
	h := newRetroGamesHarness(t)
	w, body := h.do(http.MethodGet, "games/arcade_tron", h.readToken)
	game, _ := body["game"].(map[string]any)
	if w.Code != http.StatusOK || game["title"] != "Tron" {
		t.Fatalf("hall of fame game: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodGet, "games/unknown_game", h.readToken); w.Code != http.StatusNotFound || body["code"] != "game_not_found" {
		t.Fatalf("unknown: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodGet, "games/.hidden", h.readToken); w.Code != http.StatusBadRequest || body["code"] != "invalid_identifier" {
		t.Fatalf("invalid: %d %v", w.Code, body)
	}
	if w, _ := h.do(http.MethodGet, "games/fake.js", h.readToken); w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("asset-like id must stay no-store: %q", w.Header().Get("Cache-Control"))
	}
}

func TestRetroGamesAPILibraryFlow(t *testing.T) {
	h := newRetroGamesHarness(t)
	if w, body := h.do(http.MethodPut, "favorites/arcade_tron", h.writeToken); w.Code != http.StatusOK || body["favorite"] != true {
		t.Fatalf("favorite: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPost, "plays/arcade_tron", h.writeToken); w.Code != http.StatusOK || body["recorded"] != true {
		t.Fatalf("play: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPut, "broken/arcade_joust", h.writeToken); w.Code != http.StatusOK || body["broken"] != true {
		t.Fatalf("broken: %d %v", w.Code, body)
	}
	w, body := h.do(http.MethodGet, "library", h.readToken)
	if w.Code != http.StatusOK || len(body["favorites"].([]any)) != 1 || len(body["recent"].([]any)) != 1 || len(body["broken"].([]any)) != 1 {
		t.Fatalf("library: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodDelete, "favorites/arcade_tron", h.writeToken); w.Code != http.StatusOK || body["favorite"] != false {
		t.Fatalf("unfavorite: %d %v", w.Code, body)
	}
	if w, body := h.do(http.MethodPut, "favorites/unknown_game", h.writeToken); w.Code != http.StatusNotFound || body["code"] != "game_not_found" {
		t.Fatalf("favorite unknown: %d %v", w.Code, body)
	}
}

func TestRetroGamesAPIReadOnlyDesktop(t *testing.T) {
	h := newRetroGamesHarness(t)
	h.s.Cfg.VirtualDesktop.ReadOnly = true
	if w, body := h.do(http.MethodPost, "plays/arcade_tron", h.writeToken); w.Code != http.StatusOK || body["recorded"] != false {
		t.Fatalf("read-only play must be skipped: %d %v", w.Code, body)
	}
	if w, _ := h.do(http.MethodPut, "favorites/arcade_tron", h.writeToken); w.Code != http.StatusForbidden {
		t.Fatalf("read-only favorite: %d", w.Code)
	}
	if w, _ := h.do(http.MethodPost, "sync", h.writeToken); w.Code != http.StatusForbidden {
		t.Fatalf("read-only sync: %d", w.Code)
	}
	_, body := h.do(http.MethodGet, "library", h.readToken)
	if len(body["recent"].([]any)) != 0 {
		t.Fatalf("read-only play was recorded: %v", body)
	}
}

func TestRetroGamesAPISync(t *testing.T) {
	h := newRetroGamesHarness(t)
	if w, body := h.do(http.MethodPost, "sync", h.writeToken); w.Code != http.StatusAccepted || body["started"] != true {
		t.Fatalf("sync: %d %v", w.Code, body)
	}
	h.waitIdle()
}
