package server

import (
	"aurago/internal/personalradio"
	"aurago/internal/tools"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// Match metadata, never arbitrary audio or spoken recordings. Punctuation and
// spacing variants (lo-fi/lo fi/lofi, hip-hop/hip hop) share the same match key.
func radioMusicKey(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.NewReplacer("lo fi", "lofi", "hip hop", "hiphop", "synth pop", "synthpop").Replace(value)
}

func radioRegistryMatch(p personalradio.Station, item tools.MediaItem) (string, int) {
	if item.MediaType != "music" || item.Deleted {
		return "", 0
	}
	generated := item.SourceTool == "generate_music"
	if p.Mode == "generated" && !generated || p.Mode == "local" && generated {
		return "", 0
	}
	instrumental := slices.Contains(item.Tags, "instrumental")
	if p.Vocals == "vocals" && instrumental || p.Vocals == "instrumental" && (strings.TrimSpace(item.Lyrics) != "" || slices.Contains(item.Tags, "vocals")) {
		return "", 0
	}
	metadata := radioMusicKey(item.Style + " " + item.Prompt + " " + strings.Join(item.Tags, " ") + " " + item.Description)
	genre, score := "", 0
	for _, g := range p.Genres {
		key := radioMusicKey(g.Name)
		if key != "" && strings.Contains(" "+metadata+" ", " "+key+" ") && g.Weight > score {
			genre, score = g.Name, g.Weight
		}
	}
	if score > 0 && p.Mood != "" && strings.Contains(metadata, radioMusicKey(p.Mood)) {
		score += 100
	}
	return genre, score
}

func (s *Server) openRadioMedia(item tools.MediaItem) (*os.File, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || item.MediaType != "music" || item.Deleted {
		return nil, personalradio.ErrNotFound
	}
	base, err := filepath.Abs(cfg.Directories.DataDir)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(item.FilePath)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, personalradio.ErrNotFound
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(rel)
}

func (s *Server) personalRadioLibrary(ctx context.Context, p personalradio.Station, accept func(personalradio.LibraryTrack, io.ReadSeeker) error) error {
	if s.MediaRegistryDB == nil {
		return errors.New("radio_registry_unavailable")
	}
	type match struct {
		item  tools.MediaItem
		genre string
		score int
	}
	var matches []match
	// Metadata only: never decode the whole registry before beginning playback.
	for offset := 0; ; offset += 200 {
		if err := ctx.Err(); err != nil {
			return err
		}
		items, total, err := tools.SearchMedia(s.MediaRegistryDB, "", "music", nil, 200, offset)
		if err != nil {
			return err
		}
		for _, item := range items {
			ext := strings.ToLower(filepath.Ext(item.Filename))
			if ext != ".mp3" && ext != ".wav" {
				continue
			}
			if genre, score := radioRegistryMatch(p, item); score > 0 {
				matches = append(matches, match{item, genre, score})
			}
		}
		if offset+len(items) >= total || len(items) == 0 {
			break
		}
		if offset >= 10000 {
			return personalradio.ErrLimit
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int { return b.score - a.score })
	for _, m := range matches {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, err := s.openRadioMedia(m.item)
		if err != nil {
			continue
		}
		if info, e := file.Stat(); e != nil || !info.Mode().IsRegular() || info.Size() > personalradio.MaxImportBytes {
			file.Close()
			continue
		}
		origin := "local"
		if m.item.SourceTool == "generate_music" {
			origin = "generated"
		}
		err = accept(personalradio.LibraryTrack{MediaID: m.item.ID, Title: m.item.Description, Origin: origin, Genre: m.genre, Extension: filepath.Ext(m.item.Filename)}, file)
		file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
