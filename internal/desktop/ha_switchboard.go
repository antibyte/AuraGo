package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const HASwitchboardSetting = "ha_switchboard.board"
const EmptyHASwitchboard = `{"version":1,"switches":[]}`

var haSwitchEntityID = regexp.MustCompile(`^switch\.[a-z0-9_]+$`)

type HASwitchboardEntry struct {
	EntityID string `json:"entity_id"`
	Label    string `json:"label,omitempty"`
}

type HASwitchboard struct {
	Version  int                  `json:"version"`
	Switches []HASwitchboardEntry `json:"switches"`
}

func ValidHASwitchEntityID(id string) bool {
	return len(id) <= 255 && haSwitchEntityID.MatchString(id)
}

// ParseHASwitchboard validates the board without consulting live HA state.
// Missing entities remain on the board until explicitly removed by the user.
func ParseHASwitchboard(raw string) (HASwitchboard, error) {
	board := HASwitchboard{}
	invalid := fmt.Errorf("invalid HA switchboard: use version 1, at most 60 unique switches and labels up to 80 characters")
	if len(raw) > 16*1024 || !utf8.ValidString(raw) {
		return board, invalid
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&board); err != nil {
		return board, invalid
	}
	if dec.Decode(new(interface{})) != io.EOF || board.Version != 1 || board.Switches == nil || len(board.Switches) > 60 {
		return board, invalid
	}
	seen := make(map[string]bool, len(board.Switches))
	for _, entry := range board.Switches {
		if !ValidHASwitchEntityID(entry.EntityID) || seen[entry.EntityID] || utf8.RuneCountInString(entry.Label) > 80 || strings.IndexFunc(entry.Label, unicode.IsControl) >= 0 {
			return board, invalid
		}
		seen[entry.EntityID] = true
	}
	return board, nil
}

// HASwitchboard reads the validated desktop setting shared with bootstrap and saves.
func (s *Service) HASwitchboard(ctx context.Context) (HASwitchboard, error) {
	if err := s.ensureReady(ctx); err != nil {
		return HASwitchboard{}, err
	}
	settings, err := s.listSettings(ctx)
	if err != nil {
		return HASwitchboard{}, err
	}
	return ParseHASwitchboard(settings[HASwitchboardSetting])
}
