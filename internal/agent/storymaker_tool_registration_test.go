package agent

import (
	"slices"
	"testing"

	"aurago/internal/config"
	"aurago/internal/storymaker"
)

func TestStorymakerChatToolGate(t *testing.T) {
	cfg := &config.Config{}
	cfg.Storymaker.Enabled = true
	cfg.Storymaker.Agent.Enabled = true
	cfg.VirtualDesktop.Enabled = true
	shown := func() bool {
		return buildToolFeatureFlags(RunConfig{Config: cfg}, buildToolingPolicy(cfg, "")).StorymakerEnabled
	}

	storymaker.SetAgentFacade(nil)
	if shown() {
		t.Fatal("no facade: the tool must stay hidden")
	}
	fake := &fakeStorymakerFacade{available: true}
	storymaker.SetAgentFacade(fake)
	t.Cleanup(func() { storymaker.SetAgentFacade(nil) })
	if !shown() {
		t.Fatal("published facade and all switches on: the tool must show")
	}
	fake.available = false
	if shown() {
		t.Fatal("unavailable facade: the tool must stay hidden")
	}
	fake.available = true
	for name, flag := range map[string]*bool{
		"storymaker.enabled":       &cfg.Storymaker.Enabled,
		"storymaker.agent.enabled": &cfg.Storymaker.Agent.Enabled,
		"virtual_desktop.enabled":  &cfg.VirtualDesktop.Enabled,
	} {
		*flag = false
		if shown() {
			t.Errorf("%s off: the tool must stay hidden", name)
		}
		*flag = true
	}

	if slices.Contains(builtinToolNames(ToolFeatureFlags{}), "storymaker") {
		t.Error("the flag off must not produce a schema")
	}
	if !slices.Contains(builtinToolNames(ToolFeatureFlags{StorymakerEnabled: true}), "storymaker") {
		t.Error("the flag on must produce a schema")
	}
	if got := classifyToolFamily("storymaker"); got != "media" {
		t.Errorf("family = %q, want media", got)
	}
	if got := ToolCategoryForName("storymaker"); got != "media" {
		t.Errorf("category = %q, want media", got)
	}
	if !a2aDesktopWrite(ToolCall{Action: "storymaker", Operation: "create"}) || a2aDesktopWrite(ToolCall{Action: "storymaker", Operation: "status"}) {
		t.Error("an A2A peer may only read status and list")
	}
}

func TestStorymakerIntentIsAdditive(t *testing.T) {
	for _, q := range []string{
		"Mach mir ein Erklärvideo über Backups", "Erklaervideo zu Firewalls", "ein Erklärfilm für Einsteiger", "Erklarvideo zu Backups",
		"make an explainer video about firewalls", "öffne Storymaker",
	} {
		if got := adaptiveAdditiveToolsForQuery(q); !containsString(got, "storymaker") {
			t.Errorf("%q: additive = %v", q, got)
		}
	}
	for _, q := range []string{
		"Wie wird das Wetter morgen?", "schreib eine Mail an Anna", "erklär mir das Video",
		"lade dieses YouTube Video herunter", "generate a short video of a cat", "mein ministorymaker script",
	} {
		if got := adaptiveAdditiveToolsForQuery(q); containsString(got, "storymaker") {
			t.Errorf("%q: unexpected storymaker", q)
		}
	}

	// With the tool enabled a query gets exactly the tools it gets without it, plus storymaker on an intent.
	queries := map[string]bool{
		"Mach mir ein Erklärvideo über Backups":   true,
		"make an explainer video about firewalls": true,
		"Wie wird das Wetter morgen?":             false,
		"generate a short video of a cat":         false,
	}
	offered := 0
	for flagsName, enabled := range localWikipediaProbeFlagSets() {
		enabled.StorymakerEnabled = true
		baseline := enabled
		baseline.StorymakerEnabled = false
		for _, b := range adaptiveProbeBudgets {
			for query, intent := range queries {
				label := flagsName + " " + b.name + " " + query
				baseInitial, baseRefreshed := adaptiveSelectionProbe(t, query, baseline, b, nil)
				initial, refreshed := adaptiveSelectionProbe(t, query, enabled, b, nil)
				for stage, pair := range map[string][2][]string{"initial": {baseInitial, initial}, "refreshed": {baseRefreshed, refreshed}} {
					if stage == "initial" && !b.initFiltered {
						continue
					}
					without := slices.DeleteFunc(slices.Clone(pair[1]), func(name string) bool { return name == "storymaker" })
					if !slices.Equal(without, pair[0]) {
						t.Errorf("%s %s: enabling storymaker changed the other tools\nenabled  %v\nbaseline %v", label, stage, pair[1], pair[0])
					}
					if !intent && len(without) != len(pair[1]) {
						t.Errorf("%s %s: storymaker offered without an intent", label, stage)
					}
				}
				if intent && b.total >= 40 && b.token == 0 {
					if !containsName(refreshed, "storymaker") {
						t.Errorf("%s: storymaker not offered although the budget has room", label)
					} else {
						offered++
					}
				}
			}
		}
	}
	if offered == 0 {
		t.Error("storymaker was never offered; the probe checks nothing")
	}
}
