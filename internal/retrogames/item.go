package retrogames

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

const (
	maxTitleRunes    = 200
	maxCreatorRunes  = 120
	maxTagsRunes     = 600
	maxEmulatorRunes = 40
)

// Game is one playable archive.org item.
type Game struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Year       int    `json:"year,omitempty"`
	Creator    string `json:"creator,omitempty"`
	Platform   string `json:"platform"`
	System     string `json:"system"`
	Emulator   string `json:"emulator,omitempty"`
	Downloads  int64  `json:"downloads,omitempty"`
	Tags       string `json:"-"`
}

// FlexText decodes archive.org metadata values that arrive as a string, a
// number or an (optionally nested) array of those. Objects decode to nothing.
type FlexText []string

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexText) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*f = nil
		return nil
	}
	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexText{s}
	case '[':
		var parts []json.RawMessage
		if err := json.Unmarshal(data, &parts); err != nil {
			return err
		}
		out := FlexText{}
		for _, part := range parts {
			var one FlexText
			if err := one.UnmarshalJSON(part); err != nil {
				return err
			}
			out = append(out, one...)
		}
		*f = out
	case '{':
		*f = nil
	default:
		*f = FlexText{string(data)}
	}
	return nil
}

// First returns the first non-empty value.
func (f FlexText) First() string {
	for _, v := range f {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// Join returns all non-empty values joined by sep.
func (f FlexText) Join(sep string) string {
	parts := make([]string, 0, len(f))
	for _, v := range f {
		if v = strings.TrimSpace(v); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, sep)
}

// Item is one record of an archive.org scrape page.
type Item struct {
	Identifier string   `json:"identifier"`
	Title      FlexText `json:"title"`
	Year       FlexText `json:"year"`
	Creator    FlexText `json:"creator"`
	Downloads  FlexText `json:"downloads"`
	Emulator   FlexText `json:"emulator"`
	Subject    FlexText `json:"subject"`
}

func parseCount(s string) int64 {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 && f < 1e15 {
		return int64(f)
	}
	return 0
}

// popRank is the bit length of downloads, a cheap log2 popularity bucket.
func popRank(downloads int64) int {
	rank := 0
	for d := downloads; d > 0; d >>= 1 {
		rank++
	}
	return rank
}

// normalizeItem validates and cleans one scrape record of source src.
func normalizeItem(src Source, it Item) (Game, bool) {
	id := strings.TrimSpace(it.Identifier)
	if !ValidIdentifier(id) {
		return Game{}, false
	}
	rawTitle := collapseSpace(stripControl(it.Title.Join(" ")))
	if rawTitle == "" {
		return Game{}, false
	}
	emulator := strings.ToLower(collapseSpace(stripControl(it.Emulator.First())))
	if emulator == "" {
		return Game{}, false
	}
	title, tosecYear := CleanTitle(rawTitle)
	year := ParseYear(it.Year.First())
	if year == 0 {
		year = tosecYear
	}
	tags := rawTitle
	if subject := collapseSpace(stripControl(it.Subject.Join(" "))); subject != "" {
		tags += " " + subject
	}
	return Game{
		Identifier: id,
		Title:      title,
		Year:       year,
		Creator:    truncateRunes(collapseSpace(stripControl(it.Creator.Join(", "))), maxCreatorRunes),
		Platform:   src.Key,
		System:     SystemFor(src.Key, emulator),
		Emulator:   truncateRunes(emulator, maxEmulatorRunes),
		Downloads:  parseCount(it.Downloads.First()),
		Tags:       truncateRunes(tags, maxTagsRunes),
	}, true
}
