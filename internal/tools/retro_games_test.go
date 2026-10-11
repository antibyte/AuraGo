package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/retrogames"
	"aurago/internal/retrogames/retrogamestest"
)

func retroGamesToolConfig() *config.Config {
	cfg := &config.Config{}
	cfg.RetroGames.Enabled = true
	cfg.RetroGames.AgentAccess = true
	cfg.VirtualDesktop.Enabled = true
	return cfg
}

func useRetroGamesService(t *testing.T, f retrogames.Fetcher) *retrogames.Service {
	t.Helper()
	svc := retrogamestest.NewService(t, f)
	SetRetroGamesService(svc)
	t.Cleanup(func() { SetRetroGamesService(nil) })
	return svc
}

func decodeRetroGamesOutput(t *testing.T, out string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	return m
}

func TestRetroGamesToolGating(t *testing.T) {
	SetRetroGamesService(nil)
	cfg := retroGamesToolConfig()
	cfg.RetroGames.AgentAccess = false
	if m := decodeRetroGamesOutput(t, ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "search"}).Output); m["status"] != "policy_denied" || m["code"] != "retro_games_disabled" {
		t.Fatalf("agent access off: %v", m)
	}
	if m := decodeRetroGamesOutput(t, ExecuteRetroGames(context.Background(), retroGamesToolConfig(), RetroGamesRequest{Operation: "search"}).Output); m["code"] != "retro_games_unavailable" {
		t.Fatalf("no service: %v", m)
	}
	useRetroGamesService(t, &retrogamestest.Fetcher{})
	if m := decodeRetroGamesOutput(t, ExecuteRetroGames(context.Background(), retroGamesToolConfig(), RetroGamesRequest{Operation: "dance"}).Output); m["code"] != "invalid_request" {
		t.Fatalf("bad operation: %v", m)
	}
	if RetroGamesAvailable() != true {
		t.Fatal("published enabled service must be available")
	}
}

func TestRetroGamesToolSearchHallOfFameAndIndex(t *testing.T) {
	svc := useRetroGamesService(t, &retrogamestest.Fetcher{BySource: map[string][]retrogames.Item{
		retrogames.PlatformMSDOS: {retrogamestest.Item("msdos_Prince_of_Persia_1990", "Prince of Persia", "dosbox", 1990, 99)},
	}})
	cfg := retroGamesToolConfig()
	m := decodeRetroGamesOutput(t, ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "search", Platform: "zx", Limit: 3}).Output)
	results, _ := m["results"].(string)
	if m["status"] != "success" || m["source"] != "hall_of_fame" || m["returned"].(float64) != 3 || !strings.HasPrefix(results, "<external_data>") || !strings.Contains(results, "zx_") {
		t.Fatalf("hall of fame search: %v", m)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	m = decodeRetroGamesOutput(t, ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "search", Query: "persia"}).Output)
	if m["source"] != "index" || m["total"].(float64) != 1 || !strings.Contains(m["results"].(string), "msdos_Prince_of_Persia_1990 | Prince of Persia | MS-DOS | 1990") {
		t.Fatalf("index search: %v", m)
	}
	m = decodeRetroGamesOutput(t, ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "search", Decade: 1985}).Output)
	if m["code"] != "invalid_request" {
		t.Fatalf("bad decade: %v", m)
	}
}

func TestRetroGamesToolSearchFallsBackWhileIndexIsEmpty(t *testing.T) {
	useRetroGamesService(t, &retrogamestest.Fetcher{Err: errors.New("offline")})
	m := decodeRetroGamesOutput(t, ExecuteRetroGames(context.Background(), retroGamesToolConfig(), RetroGamesRequest{Operation: "search", Query: "persia"}).Output)
	if m["source"] != "hall_of_fame" || !strings.Contains(m["note"].(string), "index_building") || !strings.Contains(m["results"].(string), "msdos_Prince_of_Persia_1990") {
		t.Fatalf("fallback: %v", m)
	}
}

func TestRetroGamesToolPlay(t *testing.T) {
	useRetroGamesService(t, &retrogamestest.Fetcher{Err: errors.New("offline")})
	cfg := retroGamesToolConfig()
	exec := ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "play", Identifier: "arcade_tron"})
	m := decodeRetroGamesOutput(t, exec.Output)
	if m["status"] != "success" || m["identifier"] != "arcade_tron" || exec.Event == nil || exec.Event.Type != "open_app" {
		t.Fatalf("play: %v event=%+v", m, exec.Event)
	}
	payload, _ := exec.Event.Payload.(map[string]interface{})
	if payload["app_id"] != "retro-games" || payload["game"] != "arcade_tron" {
		t.Fatalf("event payload = %v", payload)
	}
	if exec := ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "play", Identifier: "nope_nope"}); exec.Event != nil || decodeRetroGamesOutput(t, exec.Output)["code"] != "game_not_found" {
		t.Fatalf("unknown game: %s", exec.Output)
	}
	if exec := ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "play", Identifier: "../x"}); exec.Event != nil || decodeRetroGamesOutput(t, exec.Output)["code"] != "invalid_request" {
		t.Fatalf("invalid id: %s", exec.Output)
	}
	cfg.VirtualDesktop.Enabled = false
	if exec := ExecuteRetroGames(context.Background(), cfg, RetroGamesRequest{Operation: "play", Identifier: "arcade_tron"}); exec.Event != nil || decodeRetroGamesOutput(t, exec.Output)["code"] != "desktop_unavailable" {
		t.Fatalf("desktop off: %s", exec.Output)
	}
}

func TestWithdrawRetroGamesServiceOnlyOwn(t *testing.T) {
	a := retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	b := retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	SetRetroGamesService(a)
	t.Cleanup(func() { SetRetroGamesService(nil) })
	WithdrawRetroGamesService(b)
	if currentRetroGamesService() != a {
		t.Fatal("withdrawing another service must not clear the published one")
	}
	WithdrawRetroGamesService(a)
	if currentRetroGamesService() != nil {
		t.Fatal("own service must be withdrawn")
	}
}
