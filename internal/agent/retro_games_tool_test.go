package agent

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/retrogames/retrogamestest"
	"aurago/internal/tools"
)

type retroGamesJSONBroker struct {
	captureBroker
	mu   sync.Mutex
	json []string
}

func (b *retroGamesJSONBroker) SendJSON(s string) {
	b.mu.Lock()
	b.json = append(b.json, s)
	b.mu.Unlock()
}

func retroGamesAgentConfig() *config.Config {
	cfg := &config.Config{}
	cfg.RetroGames.Enabled = true
	cfg.RetroGames.AgentAccess = true
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Tools.VirtualDesktop.Enabled = true
	return cfg
}

func useAgentRetroGames(t *testing.T) {
	t.Helper()
	tools.SetRetroGamesService(retrogamestest.NewService(t, &retrogamestest.Fetcher{}))
	t.Cleanup(func() { tools.SetRetroGamesService(nil) })
}

func TestRetroGamesSchemaStaysCompact(t *testing.T) {
	s := retroGamesSchema()
	if s.Function.Name != "retro_games" {
		t.Fatalf("name = %q", s.Function.Name)
	}
	if got := estimateSingleToolSchemaTokens(s); got > 220 {
		t.Fatalf("retro_games schema is %d tokens, keep it at most 220", got)
	}
}

func TestDecodeRetroGamesArgs(t *testing.T) {
	req := decodeRetroGamesArgs(ToolCall{Action: "retro_games", Params: map[string]interface{}{
		"operation": "search", "query": "prince", "platform": "msdos", "decade": float64(1990), "limit": float64(5), "identifier": "x_1",
	}})
	if req.Operation != "search" || req.Query != "prince" || req.Platform != "msdos" || req.Decade != 1990 || req.Limit != 5 || req.Identifier != "x_1" {
		t.Fatalf("decoded = %+v", req)
	}
	req = decodeRetroGamesArgs(ToolCall{Action: "retro_games", Operation: "play", Params: map[string]interface{}{"id": "y_2"}})
	if req.Operation != "play" || req.Identifier != "y_2" {
		t.Fatalf("fallbacks = %+v", req)
	}
}

func TestDispatchRetroGamesPlayBroadcastsOpenApp(t *testing.T) {
	useAgentRetroGames(t)
	broker := &retroGamesJSONBroker{}
	dc := &DispatchContext{Cfg: retroGamesAgentConfig(), Logger: testLogger, Broker: broker}
	out, ok := dispatchPlatform(context.Background(), ToolCall{Action: "retro_games", Params: map[string]interface{}{"operation": "play", "identifier": "arcade_tron"}}, dc)
	if !ok || !strings.HasPrefix(out, "Tool Output: ") || !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("dispatch = %v %s", ok, out)
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if len(broker.json) != 1 {
		t.Fatalf("broadcasts = %d", len(broker.json))
	}
	var msg struct {
		Type    string `json:"type"`
		Payload struct {
			Type    string            `json:"type"`
			Payload map[string]string `json:"payload"`
		} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(broker.json[0]), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "virtual_desktop_event" || msg.Payload.Type != "open_app" || msg.Payload.Payload["app_id"] != "retro-games" || msg.Payload.Payload["game"] != "arcade_tron" {
		t.Fatalf("broadcast = %+v", msg)
	}
}

func TestRetroGamesFeatureNeedsPublishedService(t *testing.T) {
	cfg := retroGamesAgentConfig()
	tools.SetRetroGamesService(nil)
	if buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, "")).RetroGamesEnabled {
		t.Fatal("no service: tool must stay hidden")
	}
	useAgentRetroGames(t)
	if !buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, "")).RetroGamesEnabled {
		t.Fatal("published service and both switches on: tool must show")
	}
	cfg.RetroGames.AgentAccess = false
	if buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, "")).RetroGamesEnabled {
		t.Fatal("agent_access off: tool must stay hidden")
	}
}

func TestRetroGamesIntentIsAdditive(t *testing.T) {
	for _, q := range []string{"Lass uns ein Retro-Spiel spielen", "play an old DOS game", "Starte etwas aus der Spielhalle", "Hast du Amiga Spiele?"} {
		if got := adaptiveAdditiveToolsForQuery(q); !containsString(got, "retro_games") {
			t.Errorf("%q: additive = %v", q, got)
		}
	}
	for _, q := range []string{"Wie wird das Wetter morgen?", "schreib eine Mail an Anna"} {
		if got := adaptiveAdditiveToolsForQuery(q); containsString(got, "retro_games") {
			t.Errorf("%q: unexpected retro_games", q)
		}
	}
	if got := adaptiveAdditiveToolsForQuery("Was sagt die Wikipedia über Amiga?"); !containsString(got, "local_wikipedia") || !containsString(got, "retro_games") {
		t.Errorf("both intents: %v", got)
	}
}

// TestRetroGamesAliasesRankTheToolOnlyWhileItIsOn: an exact alias message such
// as "spielhalle" ranks the catalog as before the alias existed while the tool
// is off, and ranks retro_games first while it is on.
func TestRetroGamesAliasesRankTheToolOnlyWhileItIsOn(t *testing.T) {
	off := allBuiltinToolFeatureFlags()
	off.RetroGamesEnabled = false
	offSchemas := BuildNativeToolSchemas(t.TempDir(), nil, off, nil)
	onFlags := off
	onFlags.RetroGamesEnabled = true
	onSchemas := BuildNativeToolSchemas(t.TempDir(), nil, onFlags, nil)
	aliases := discoverToolNameAliases
	t.Cleanup(func() { discoverToolNameAliases = aliases })
	found := 0
	for alias, target := range aliases {
		if target != "retro_games" {
			continue
		}
		found++
		withAlias := catalogNames(BuildToolCatalog(offSchemas, offSchemas, "").Search(alias))
		discoverToolNameAliases = nil
		before := catalogNames(BuildToolCatalog(offSchemas, offSchemas, "").Search(alias))
		discoverToolNameAliases = aliases
		if !slices.Equal(withAlias, before) {
			t.Errorf("%q with the tool off: %v, before the alias %v", alias, withAlias, before)
		}
		if on := catalogNames(BuildToolCatalog(onSchemas, onSchemas, "").Search(alias)); len(on) == 0 || on[0] != "retro_games" {
			t.Errorf("%q with the tool on ranks %v", alias, on)
		}
		if got := resolveDiscoverToolName(alias); got != "retro_games" {
			t.Errorf("discover_tools resolves %q to %q", alias, got)
		}
	}
	if found == 0 {
		t.Fatal("no retro_games alias registered")
	}
}
