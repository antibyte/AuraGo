// Package personalradio owns the durable library and bounded production of a
// personal station. Providers and the browser are adapters, never the scheduler.
package personalradio

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("radio_not_found")
	ErrConflict = errors.New("radio_conflict")
	ErrLease    = errors.New("radio_device_busy")
	ErrLimit    = errors.New("radio_limit")
)

type Genre struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

type Station struct {
	ID               string  `json:"id"`
	Revision         int     `json:"revision"`
	Name             string  `json:"name"`
	Topics           string  `json:"topics"`
	Language         string  `json:"language"`
	Mode             string  `json:"mode"`
	Genres           []Genre `json:"genres"`
	Mood             string  `json:"mood"`
	Vocals           string  `json:"vocals"`
	BPM              int     `json:"bpm"`
	GeneratedPercent int     `json:"generated_percent"`
	Moderation       string  `json:"moderation"`
	ModerationStyle  string  `json:"moderation_style"`
	NewsMinutes      int     `json:"news_minutes"`
	NewsTopics       bool    `json:"news_topics"`
	International    bool    `json:"international"`
	National         bool    `json:"national"`
	Regional         bool    `json:"regional"`
	Country          string  `json:"country"`
	Region           string  `json:"region"`
	Timezone         string  `json:"timezone"`
	ReserveMinutes   int     `json:"reserve_minutes"`
	MinTracks        int     `json:"min_tracks"`
	RepeatMinutes    int     `json:"repeat_minutes"`
	RepeatTracks     int     `json:"repeat_tracks"`
	Strict           bool    `json:"strict"`
	DailyGenerations int     `json:"daily_generations"`
	DailyEditorial   int     `json:"daily_editorial"`
	DailyTTSChars    int     `json:"daily_tts_chars"`
	LibraryMinutes   int     `json:"library_minutes"`
	LibraryMB        int     `json:"library_mb"`
}

func DefaultStation() Station {
	return Station{Name: "Personal Radio", Language: "de", Mode: "mixed", Genres: []Genre{{"Lo-fi", 1}}, Vocals: "instrumental", GeneratedPercent: 50, Moderation: "balanced", NewsMinutes: 0, NewsTopics: true, Timezone: "UTC", ReserveMinutes: 0, MinTracks: 2, RepeatMinutes: 60, RepeatTracks: 20, DailyGenerations: 20, DailyEditorial: 48, DailyTTSChars: 30000, LibraryMinutes: 120, LibraryMB: 2048}
}

func (s Station) Validate() error {
	if strings.TrimSpace(s.Name) == "" || len([]rune(s.Name)) > 100 || len([]rune(s.Topics)) > 2000 || len(s.Mood) > 200 || len(s.ModerationStyle) > 500 {
		return errors.New("radio_invalid_profile")
	}
	if s.Mode != "local" && s.Mode != "generated" && s.Mode != "mixed" {
		return errors.New("radio_invalid_mode")
	}
	if s.Vocals != "instrumental" && s.Vocals != "vocals" && s.Vocals != "mixed" {
		return errors.New("radio_invalid_vocals")
	}
	if s.Moderation != "off" && s.Moderation != "little" && s.Moderation != "balanced" && s.Moderation != "much" {
		return errors.New("radio_invalid_moderation")
	}
	if s.NewsMinutes != 0 && s.NewsMinutes != 30 && s.NewsMinutes != 60 {
		return errors.New("radio_invalid_news_interval")
	}
	if s.NewsMinutes > 0 && !(s.NewsTopics || s.International || s.National || s.Regional) {
		return errors.New("radio_news_scope_required")
	}
	if s.NewsMinutes > 0 && s.NewsTopics && strings.TrimSpace(s.Topics) == "" {
		return errors.New("radio_invalid_profile")
	}
	for _, c := range s.Country {
		if !(c >= 'A' && c <= 'Z') {
			return errors.New("radio_region_required")
		}
	}
	if len(s.Country) > 2 || (s.NewsMinutes > 0 && (s.National || s.Regional) && len(s.Country) != 2) || len(s.Region) > 120 || (s.NewsMinutes > 0 && s.Regional && strings.TrimSpace(s.Region) == "") {
		return errors.New("radio_region_required")
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return errors.New("radio_invalid_timezone")
	}
	if len(s.Language) < 2 || len(s.Language) > 16 || len(s.Genres) == 0 || len(s.Genres) > 12 {
		return errors.New("radio_invalid_genres")
	}
	for _, g := range s.Genres {
		if strings.TrimSpace(g.Name) == "" || len(g.Name) > 80 || g.Weight < 1 || g.Weight > 100 {
			return errors.New("radio_invalid_genres")
		}
	}
	if s.BPM < 0 || (s.BPM > 0 && s.BPM < 30) || s.BPM > 300 || s.GeneratedPercent < 0 || s.GeneratedPercent > 100 || s.ReserveMinutes < 0 || s.ReserveMinutes > 180 || s.MinTracks < 2 || s.MinTracks > 100 || s.RepeatMinutes < 0 || s.RepeatMinutes > 1440 || s.RepeatTracks < 0 || s.RepeatTracks > 200 {
		return errors.New("radio_invalid_rotation")
	}
	if s.DailyGenerations < 0 || s.DailyGenerations > 200 || s.DailyEditorial < 0 || s.DailyEditorial > 200 || s.DailyTTSChars < 0 || s.DailyTTSChars > 200000 || s.LibraryMinutes < s.ReserveMinutes || s.LibraryMinutes > 1440 || s.LibraryMB < 100 || s.LibraryMB > 20000 {
		return errors.New("radio_invalid_limits")
	}
	return nil
}

type Track struct {
	ID         string    `json:"id"`
	MediaID    int64     `json:"media_id"`
	Title      string    `json:"title"`
	Origin     string    `json:"origin"`
	Genre      string    `json:"genre"`
	Hash       string    `json:"hash"`
	DurationMS int64     `json:"duration_ms"`
	Bytes      int64     `json:"bytes"`
	Rate       int       `json:"rate"`
	Channels   int       `json:"channels"`
	Favorite   bool      `json:"favorite"`
	Blocked    bool      `json:"blocked"`
	Weight     int       `json:"weight"`
	Plays      int       `json:"plays"`
	LastPlayed time.Time `json:"last_played"`
}

type Source struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Published time.Time `json:"published"`
	Retrieved time.Time `json:"retrieved"`
	Text      string    `json:"text,omitempty"`
}

type Segment struct {
	Opening    bool      `json:"opening,omitempty"`
	ID         string    `json:"id"`
	AssetID    string    `json:"asset_id"`
	TrackID    string    `json:"track_id,omitempty"`
	Kind       string    `json:"kind"`
	Title      string    `json:"title"`
	Text       string    `json:"text,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	Rate       int       `json:"rate"`
	Channels   int       `json:"channels"`
	Sources    []Source  `json:"sources,omitempty"`
	Due        time.Time `json:"due,omitempty"`
	Aired      time.Time `json:"aired,omitempty"`
	Expires    time.Time `json:"expires,omitempty"`
	ForTrack   string    `json:"for_track,omitempty"`
}

type Plan struct {
	Theme      string   `json:"theme"`
	TrackIDs   []string `json:"track_ids"`
	Moderation string   `json:"moderation"`
	MusicIdea  string   `json:"music_idea"`
	NewsText   string   `json:"news_text"`
	SourceIDs  []string `json:"source_ids"`
}

type EditorialRequest struct {
	Opening *OpeningContext `json:"opening,omitempty"`
	Station Station         `json:"station"`
	Tracks  []Track         `json:"tracks"`
	Recent  []string        `json:"recent"`
	Sources []Source        `json:"sources,omitempty"`
	News    bool            `json:"news"`
}

// OpeningContext is a factual startup snapshot, not an estimated completion time.
type OpeningContext struct {
	LibraryPending bool  `json:"library_pending,omitempty"`
	TrackCount     int   `json:"track_count"`
	MinTracks      int   `json:"min_tracks"`
	BufferMS       int64 `json:"buffer_ms"`
	RequiredMS     int64 `json:"required_ms"`
}

// Production identifies a durable original. A zero MediaID needs registration.
type Production struct {
	Path, Title, Genre                               string
	MediaID                                          int64
	Prompt, Style, Lyrics, Language, Provider, Model string
	Tags                                             []string
	DurationMS, GenerationTimeMS                     int64
	CostEstimate                                     float64
}

// LibraryTrack identifies existing music; providers retain ownership of the original.
type LibraryTrack struct {
	MediaID                         int64
	Title, Origin, Genre, Extension string
}
type Audio struct {
	Data      []byte
	Extension string
}
type Adapters struct {
	Library  func(context.Context, Station, func(LibraryTrack, io.ReadSeeker) error) error
	Generate func(context.Context, Station, string, string) (Production, error)
	Register func(context.Context, Production) (Production, error)
	Issue    func(string, bool)
	Plan     func(context.Context, EditorialRequest) (Plan, error)
	Research func(context.Context, Station) ([]Source, error)
	Speak    func(context.Context, Station, string) (Audio, error)
}

type State struct {
	LibraryStatus  string    `json:"library_status"`
	MusicReady     bool      `json:"music_ready"`
	OpeningStatus  string    `json:"opening_status"`
	StationID      string    `json:"station_id"`
	Status         string    `json:"status"`
	Epoch          string    `json:"epoch"`
	Owner          string    `json:"owner"`
	Queue          []Segment `json:"queue"`
	Current        string    `json:"current"`
	BufferMS       int64     `json:"buffer_ms"`
	RequiredMS     int64     `json:"required_ms"`
	TrackCount     int       `json:"track_count"`
	MusicBusy      bool      `json:"music_busy"`
	EditorBusy     bool      `json:"editor_busy"`
	Code           string    `json:"code"`
	NewsCode       string    `json:"news_code"`
	EditorialCode  string    `json:"editorial_code"`
	Theme          string    `json:"theme"`
	NextNews       time.Time `json:"next_news"`
	News           []Segment `json:"news"`
	Relaxed        bool      `json:"relaxed"`
	GeneratedToday int       `json:"generated_today"`
}
