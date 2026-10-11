package retrogames

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed hall_of_fame.json
var hallOfFameJSON []byte

type hallOfFameEntry struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Platform   string `json:"platform"`
	System     string `json:"system"`
	Year       int    `json:"year"`
	Why        string `json:"why"`
}

var hallOfFame = mustParseHallOfFame(hallOfFameJSON)

func mustParseHallOfFame(data []byte) []Game {
	games, err := parseHallOfFame(data)
	if err != nil {
		panic(err)
	}
	return games
}

func parseHallOfFame(data []byte) ([]Game, error) {
	var entries []hallOfFameEntry
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("retrogames: hall of fame: %w", err)
	}
	seen := map[string]bool{}
	out := make([]Game, 0, len(entries))
	for i, e := range entries {
		switch {
		case !ValidIdentifier(e.Identifier):
			return nil, fmt.Errorf("retrogames: hall of fame entry %d: invalid identifier %q", i, e.Identifier)
		case seen[e.Identifier]:
			return nil, fmt.Errorf("retrogames: hall of fame entry %d: duplicate %q", i, e.Identifier)
		case strings.TrimSpace(e.Title) == "":
			return nil, fmt.Errorf("retrogames: hall of fame entry %d: empty title", i)
		case !ValidPlatform(e.Platform):
			return nil, fmt.Errorf("retrogames: hall of fame entry %d: unknown platform %q", i, e.Platform)
		case strings.TrimSpace(e.System) == "":
			return nil, fmt.Errorf("retrogames: hall of fame entry %d: empty system", i)
		case e.Year != 0 && (e.Year < 1950 || e.Year > 2030):
			return nil, fmt.Errorf("retrogames: hall of fame entry %d: invalid year %d", i, e.Year)
		case strings.TrimSpace(e.Why) == "":
			return nil, fmt.Errorf("retrogames: hall of fame entry %d: missing why", i)
		}
		seen[e.Identifier] = true
		out = append(out, Game{Identifier: e.Identifier, Title: e.Title, Year: e.Year, Platform: e.Platform, System: e.System})
	}
	return out, nil
}

// HallOfFame returns the curated list in display order, optionally for one platform.
func HallOfFame(platform string) []Game {
	out := []Game{}
	for _, g := range hallOfFame {
		if platform == "" || g.Platform == platform {
			out = append(out, g)
		}
	}
	return out
}

func hallOfFameGame(id string) (Game, bool) {
	for _, g := range hallOfFame {
		if g.Identifier == id {
			return g, true
		}
	}
	return Game{}, false
}
