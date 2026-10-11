package agent

import (
	"slices"
	"testing"
)

// retroGamesProbeQueries maps a probe message to whether it names retro games.
var retroGamesProbeQueries = map[string]bool{
	"Lass uns ein Retro-Spiel spielen":    true,
	"play an old DOS game":                true,
	"Starte etwas aus der Spielhalle":     true,
	"Hast du Amiga Spiele?":               true,
	"ich will ein altes Spiel spielen":    true,
	"C64er Spiele":                        true,
	"Was sagt die Wikipedia über Amiga?":  true,
	"Wie wird das Wetter morgen?":         false,
	"schreib eine Mail an Anna":           false,
	"zeige mir die cpu auslastung":        false,
	"starte den docker container neu":     false,
	"search the web for golang news":      false,
	"starte den Android Emulator":         false,
	"schick mir den archive.org Link":     false,
	"read the server logs and summarise":  false,
	"ddos game server logs":               false,
	"offline wikipedia":                   false,
	"schick mir die Spiele Mail an Anna":  false,
	"sha256:3fc64a9d image digest prüfen": false,
}

// TestRetroGamesOnlyAddsToTheAdaptiveSelection: with retro_games enabled a
// query is offered exactly the tools it gets without it, plus retro_games when
// it names retro games and the budget has room. The baseline does not depend
// on the flag, so it is the selection before the tool existed.
func TestRetroGamesOnlyAddsToTheAdaptiveSelection(t *testing.T) {
	offered := 0
	for flagsName, enabled := range localWikipediaProbeFlagSets() {
		enabled.RetroGamesEnabled = true
		baseline := enabled
		baseline.RetroGamesEnabled = false
		for _, b := range adaptiveProbeBudgets {
			for query, intent := range retroGamesProbeQueries {
				label := flagsName + " " + b.name + " " + query
				baseInitial, baseRefreshed := adaptiveSelectionProbe(t, query, baseline, b, nil)
				initial, refreshed := adaptiveSelectionProbe(t, query, enabled, b, nil)
				for stage, pair := range map[string][2][]string{"initial": {baseInitial, initial}, "refreshed": {baseRefreshed, refreshed}} {
					if stage == "initial" && !b.initFiltered {
						continue
					}
					without := slices.DeleteFunc(slices.Clone(pair[1]), func(name string) bool { return name == "retro_games" })
					if !slices.Equal(without, pair[0]) {
						t.Errorf("%s %s: enabling retro_games changed the other tools\nenabled  %v\nbaseline %v", label, stage, pair[1], pair[0])
					}
					if !intent && len(without) != len(pair[1]) {
						t.Errorf("%s %s: retro_games offered without a retro games intent", label, stage)
					}
					if b.total > 0 && len(pair[1]) > b.total && len(pair[1]) > len(pair[0]) {
						t.Errorf("%s %s: %d tools exceed the cap of %d", label, stage, len(pair[1]), b.total)
					}
				}
				if intent && b.total >= 40 && b.token == 0 {
					if !containsName(refreshed, "retro_games") {
						t.Errorf("%s: retro_games not offered although the budget has room", label)
					} else {
						offered++
					}
				}
			}
		}
	}
	if offered == 0 {
		t.Error("retro_games was never offered; the probe checks nothing")
	}
}
