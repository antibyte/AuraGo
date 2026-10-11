package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// do sends the request with a bearer token.
func (h *retroGamesHarness) do(method, path, token string) (*httptest.ResponseRecorder, map[string]any) {
	h.t.Helper()
	return h.doAs(method, path, token)
}

// doAs sends the request with a credential: "" is anonymous, "session" is a
// valid session cookie, anything else is a bearer token. The handler is called
// directly, so the CSRF middleware in front of it is not exercised.
func (h *retroGamesHarness) doAs(method, path, credential string) (*httptest.ResponseRecorder, map[string]any) {
	h.t.Helper()
	r := httptest.NewRequest(method, "/api/desktop/retro-games/"+path, nil)
	switch credential {
	case "":
	case "session":
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(h.s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	default:
		r.Header.Set("Authorization", "Bearer "+credential)
	}
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

func TestRetroGamesAPICredentials(t *testing.T) {
	h := newRetroGamesHarness(t)
	if w, body := h.doAs(http.MethodGet, "status", ""); w.Code != http.StatusUnauthorized || body["error"] != "unauthorized" {
		t.Fatalf("anonymous GET: %d %v", w.Code, body)
	}
	if w, body := h.doAs(http.MethodPut, "favorites/arcade_tron", ""); w.Code != http.StatusUnauthorized || body["error"] != "unauthorized" {
		t.Fatalf("anonymous PUT: %d %v", w.Code, body)
	}
	if w, _ := h.doAs(http.MethodGet, "status", "not-a-token"); w.Code != http.StatusForbidden {
		t.Fatalf("unknown bearer: %d", w.Code)
	}
	if w, body := h.doAs(http.MethodGet, "status", "session"); w.Code != http.StatusOK || body["enabled"] != true {
		t.Fatalf("session GET: %d %v", w.Code, body)
	}
	if w, body := h.doAs(http.MethodPut, "favorites/arcade_tron", "session"); w.Code != http.StatusOK || body["favorite"] != true {
		t.Fatalf("session PUT: %d %v", w.Code, body)
	}
	if _, body := h.doAs(http.MethodGet, "library", "session"); len(body["favorites"].([]any)) != 1 {
		t.Fatalf("session favorite was not stored: %v", body)
	}
}

func TestRetroGamesAPIHeadAndRouting(t *testing.T) {
	h := newRetroGamesHarness(t)
	for _, path := range []string{"status", "search?q=persia", "hall-of-fame", "library", "games/arcade_tron"} {
		w, _ := h.do(http.MethodHead, path, h.readToken)
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("HEAD %s: %d cache=%q", path, w.Code, w.Header().Get("Cache-Control"))
		}
	}
	for _, path := range []string{"status", "search", "hall-of-fame", "library", "games/arcade_tron"} {
		w, body := h.do(http.MethodPost, path, h.writeToken)
		if w.Code != http.StatusMethodNotAllowed || body["code"] != "method_not_allowed" || w.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("POST %s: %d allow=%q %v", path, w.Code, w.Header().Get("Allow"), body)
		}
	}
	if w, _ := h.do(http.MethodHead, "favorites/arcade_tron", h.readToken); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "PUT, DELETE" {
		t.Fatalf("HEAD favorites: %d allow=%q", w.Code, w.Header().Get("Allow"))
	}
	for _, path := range []string{"games/a/b", "favorites/a/b", "games", "games/", "library/extra"} {
		if w, body := h.do(http.MethodGet, path, h.readToken); w.Code != http.StatusNotFound || body["code"] != "not_found" {
			t.Fatalf("GET %s: %d %v", path, w.Code, body)
		}
	}
}

func TestRetroGamesAPILibraryFull(t *testing.T) {
	h := newRetroGamesHarness(t)
	const limit = 500
	items := make([]retrogames.Item, 0, limit+1)
	for i := 0; i <= limit; i++ {
		items = append(items, retrogamestest.Item(fmt.Sprintf("msdos_game_%03d", i), fmt.Sprintf("Game %03d", i), "dosbox", 1990, int64(i+1)))
	}
	h.s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{BySource: map[string][]retrogames.Item{retrogames.PlatformMSDOS: items}})
	if w, _ := h.do(http.MethodPost, "sync", h.writeToken); w.Code != http.StatusAccepted {
		t.Fatalf("sync: %d", w.Code)
	}
	h.waitIdle()
	ctx := context.Background()
	for i := 0; i < limit; i++ {
		if err := h.s.RetroGames.SetFavorite(ctx, items[i].Identifier, true); err != nil {
			t.Fatalf("favorite %d: %v", i, err)
		}
	}
	over := "favorites/" + items[limit].Identifier
	if w, body := h.do(http.MethodPut, over, h.writeToken); w.Code != http.StatusConflict || body["code"] != "library_full" {
		t.Fatalf("full list: %d %v", w.Code, body)
	}
	// An existing favorite is not growth, and removing frees a slot.
	if w, body := h.do(http.MethodPut, "favorites/"+items[0].Identifier, h.writeToken); w.Code != http.StatusOK || body["favorite"] != true {
		t.Fatalf("re-favorite at the cap: %d %v", w.Code, body)
	}
	if w, _ := h.do(http.MethodDelete, "favorites/"+items[1].Identifier, h.writeToken); w.Code != http.StatusOK {
		t.Fatalf("unfavorite: %d", w.Code)
	}
	if w, body := h.do(http.MethodPut, over, h.writeToken); w.Code != http.StatusOK || body["favorite"] != true {
		t.Fatalf("favorite after freeing a slot: %d %v", w.Code, body)
	}
}

func TestRetroGamesWriteErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
		logged bool
	}{
		{"disabled", fmt.Errorf("wrapped: %w", retrogames.ErrDisabled), http.StatusServiceUnavailable, "disabled", false},
		{"invalid query", retrogames.ErrInvalidQuery, http.StatusBadRequest, "invalid_request", false},
		{"invalid identifier", retrogames.ErrInvalidIdentifier, http.StatusBadRequest, "invalid_identifier", false},
		{"unknown game", retrogames.ErrUnknownGame, http.StatusNotFound, "game_not_found", false},
		{"library full", fmt.Errorf("favorite: %w", retrogames.ErrLibraryFull), http.StatusConflict, "library_full", false},
		{"sync running", retrogames.ErrSyncRunning, http.StatusConflict, "sync_running", false},
		{"revoked", fmt.Errorf("publish: %w", context.Canceled), http.StatusForbidden, "desktop_readonly", false},
		{"deadline", fmt.Errorf("write: %w", context.DeadlineExceeded), http.StatusServiceUnavailable, "timeout", false},
		{"other", errors.New("disk on fire"), http.StatusInternalServerError, "retro_games_failed", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			s := &Server{Logger: slog.New(slog.NewTextHandler(&logs, nil))}
			w := httptest.NewRecorder()
			s.writeRetroGamesError(w, tc.err)
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %q: %v", w.Body.String(), err)
			}
			if w.Code != tc.status || body["code"] != tc.code {
				t.Fatalf("%v: %d %v", tc.err, w.Code, body)
			}
			if strings.Contains(w.Body.String(), "disk on fire") {
				t.Fatalf("internal error text leaked: %s", w.Body.String())
			}
			if logged := logs.Len() > 0; logged != tc.logged {
				t.Fatalf("logged = %v (%q), want %v", logged, logs.String(), tc.logged)
			}
		})
	}
}
