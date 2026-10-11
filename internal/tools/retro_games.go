package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/retrogames"
	"aurago/internal/security"
)

const (
	retroGamesDefaultLimit = 10
	retroGamesMaxLimit     = 20

	// The tool output has to fit the 4000-byte preview the agent loop keeps, so
	// the result list is clipped per field and by total size.
	retroGamesTitleRunes  = 80
	retroGamesMakerRunes  = 60
	retroGamesResultBytes = 2800
)

var (
	retroGamesMu      sync.RWMutex
	retroGamesService *retrogames.Service
)

// SetRetroGamesService publishes the server-owned service to the agent tool;
// nil withdraws it.
func SetRetroGamesService(svc *retrogames.Service) {
	retroGamesMu.Lock()
	defer retroGamesMu.Unlock()
	retroGamesService = svc
}

// WithdrawRetroGamesService clears the published service only while it is svc.
func WithdrawRetroGamesService(svc *retrogames.Service) {
	if svc == nil {
		return
	}
	retroGamesMu.Lock()
	defer retroGamesMu.Unlock()
	if retroGamesService == svc {
		retroGamesService = nil
	}
}

func currentRetroGamesService() *retrogames.Service {
	retroGamesMu.RLock()
	defer retroGamesMu.RUnlock()
	return retroGamesService
}

// RetroGamesAvailable reports whether a published service grants agent access.
func RetroGamesAvailable() bool {
	svc := currentRetroGamesService()
	return svc != nil && svc.AgentAccess()
}

// RetroGamesRequest is a decoded retro_games tool call.
type RetroGamesRequest struct {
	Operation  string
	Query      string
	Platform   string
	Decade     int
	Limit      int
	Identifier string
}

// ExecuteRetroGames runs the retro_games tool. A successful play carries an
// open_app desktop event for the Retro Games app.
func ExecuteRetroGames(ctx context.Context, cfg *config.Config, req RetroGamesRequest) VirtualDesktopExecution {
	if cfg == nil || !cfg.RetroGames.Enabled || !cfg.RetroGames.AgentAccess {
		return retroGamesFailure("policy_denied", "retro_games_disabled", "Retro Games is not enabled for agent access.")
	}
	svc := currentRetroGamesService()
	if svc == nil || !svc.AgentAccess() {
		return retroGamesFailure("error", "retro_games_unavailable", "Retro Games is not available right now.")
	}
	switch strings.ToLower(strings.TrimSpace(req.Operation)) {
	case "search":
		return retroGamesSearch(ctx, svc, req)
	case "play":
		return retroGamesPlay(ctx, cfg, svc, req)
	default:
		return retroGamesFailure("error", "invalid_request", "operation must be search or play.")
	}
}

func retroGamesSearch(ctx context.Context, svc *retrogames.Service, req RetroGamesRequest) VirtualDesktopExecution {
	limit := req.Limit
	if limit <= 0 {
		limit = retroGamesDefaultLimit
	}
	if limit > retroGamesMaxLimit {
		limit = retroGamesMaxLimit
	}
	query := strings.TrimSpace(req.Query)
	if _, err := (retrogames.Query{Text: query, Platform: req.Platform, Decade: req.Decade}).Normalize(); err != nil {
		return retroGamesErrorFor(err)
	}
	if query == "" {
		games, err := svc.HallOfFame(ctx, req.Platform)
		if err != nil {
			return retroGamesErrorFor(err)
		}
		games = retroGamesDecade(games, req.Decade)
		return retroGamesList("hall_of_fame", len(games), retroGamesHead(games, limit), "")
	}
	res, err := svc.Search(ctx, retrogames.Query{Text: query, Platform: req.Platform, Decade: req.Decade, Limit: limit})
	if err != nil {
		return retroGamesErrorFor(err)
	}
	if res.Total == 0 {
		// Only an index that holds no games at all falls back to the Hall of
		// Fame; a miss on a populated index is an honest empty result.
		if status, statusErr := svc.Status(ctx); statusErr == nil && status.TotalGames == 0 {
			hof, hofErr := svc.HallOfFame(ctx, req.Platform)
			if hofErr != nil {
				return retroGamesErrorFor(hofErr)
			}
			hits := retroGamesMatchTitles(retroGamesDecade(hof, req.Decade), query)
			return retroGamesList("hall_of_fame", len(hits), retroGamesHead(hits, limit),
				"index_building: the full game index is still being built; only matching Hall of Fame games are listed. Search again in a minute or two.")
		}
	}
	return retroGamesList("index", res.Total, res.Games, "")
}

func retroGamesPlay(ctx context.Context, cfg *config.Config, svc *retrogames.Service, req RetroGamesRequest) VirtualDesktopExecution {
	if !cfg.VirtualDesktop.Enabled {
		return retroGamesFailure("error", "desktop_unavailable", "The virtual desktop is switched off, so no game can be opened.")
	}
	if !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled {
		return retroGamesFailure("policy_denied", "desktop_agent_control_disabled", "The user has not allowed the agent to control the desktop, so the game cannot be opened.")
	}
	g, err := svc.Game(ctx, strings.TrimSpace(req.Identifier))
	if err != nil {
		return retroGamesErrorFor(err)
	}
	event := virtualDesktopEvent("open_app", map[string]interface{}{"app_id": "retro-games", "game": g.Identifier})
	return retroGamesResult(map[string]any{
		"status":     "success",
		"operation":  "play",
		"identifier": g.Identifier,
		"title":      security.IsolateExternalData(retroGamesClip(g.Title, retroGamesTitleRunes)),
		"system":     g.System,
		"message":    "Opened in the Retro Games desktop app. The user starts the emulator with the power button on the game screen.",
	}, event)
}

func retroGamesDecade(games []retrogames.Game, decade int) []retrogames.Game {
	if decade == 0 {
		return games
	}
	out := []retrogames.Game{}
	for _, g := range games {
		if g.Year >= decade && g.Year <= decade+9 {
			out = append(out, g)
		}
	}
	return out
}

// retroGamesMatchTitles keeps the games whose title contains every query word.
// Words are split like the index splits them: letters and digits only.
func retroGamesMatchTitles(games []retrogames.Game, query string) []retrogames.Game {
	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := []retrogames.Game{}
	if len(words) == 0 {
		return out
	}
	for _, g := range games {
		title := strings.ToLower(g.Title)
		match := true
		for _, w := range words {
			if !strings.Contains(title, w) {
				match = false
				break
			}
		}
		if match {
			out = append(out, g)
		}
	}
	return out
}

// retroGamesClip puts s on one line and cuts it to at most maxRunes runes,
// marking the cut with an ellipsis.
func retroGamesClip(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(strings.ToValidUTF8(s, "")), " ")
	if maxRunes < 1 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "\u2026"
}

func retroGamesHead(games []retrogames.Game, limit int) []retrogames.Game {
	if len(games) > limit {
		return games[:limit]
	}
	return games
}

func retroGamesList(source string, total int, games []retrogames.Game, note string) VirtualDesktopExecution {
	var lines strings.Builder
	returned := 0
	used := 0
	truncated := false
	for _, g := range games {
		year := "?"
		if g.Year > 0 {
			year = strconv.Itoa(g.Year)
		}
		line := fmt.Sprintf("%d. %s | %s | %s | %s", returned+1, g.Identifier, retroGamesClip(g.Title, retroGamesTitleRunes), g.System, year)
		if maker := retroGamesClip(g.Creator, retroGamesMakerRunes); maker != "" {
			line += " | " + maker
		}
		// The budget counts the escaped size, which bounds the isolated text
		// (and so the raw text) even when a title is all quotes and ampersands.
		cost := len(html.EscapeString(line)) + 1
		if used+cost > retroGamesResultBytes {
			truncated = true
			break
		}
		used += cost
		lines.WriteString(line + "\n")
		returned++
	}
	payload := map[string]any{
		"status":    "success",
		"operation": "search",
		"source":    source,
		"total":     total,
		"returned":  returned,
		"format":    "n. identifier | title | system | year | maker",
	}
	if returned > 0 {
		payload["results"] = security.IsolateExternalData(strings.TrimRight(lines.String(), "\n"))
	}
	if truncated {
		payload["truncated"] = true
	}
	if note != "" {
		payload["note"] = note
	}
	return retroGamesResult(payload, nil)
}

func retroGamesErrorFor(err error) VirtualDesktopExecution {
	switch {
	case errors.Is(err, retrogames.ErrDisabled):
		return retroGamesFailure("policy_denied", "retro_games_disabled", "Retro Games is not enabled for agent access.")
	case errors.Is(err, retrogames.ErrInvalidQuery), errors.Is(err, retrogames.ErrInvalidIdentifier):
		return retroGamesFailure("error", "invalid_request", "Invalid arguments: platform must be msdos, arcade, console, c64, amiga, apple2, zx or win3; decade 1950-2020 in steps of 10; identifier from a search result.")
	case errors.Is(err, retrogames.ErrUnknownGame):
		return retroGamesFailure("error", "game_not_found", "No such game. Search first and pass an identifier from the results.")
	default:
		return retroGamesFailure("error", "retro_games_failed", "Retro Games could not answer; try again shortly.")
	}
}

func retroGamesFailure(status, code, message string) VirtualDesktopExecution {
	return retroGamesResult(map[string]any{"status": status, "code": code, "message": message}, nil)
}

// retroGamesResult encodes without HTML escaping so the isolation tags stay
// readable; IsolateExternalData has already escaped archive.org text.
func retroGamesResult(payload map[string]any, event *desktop.Event) VirtualDesktopExecution {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return VirtualDesktopExecution{Output: `{"status":"error","code":"retro_games_failed","message":"result could not be encoded"}`}
	}
	return VirtualDesktopExecution{Output: strings.TrimRight(buf.String(), "\n"), Event: event}
}
