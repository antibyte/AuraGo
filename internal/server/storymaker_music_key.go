package server

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"

	"aurago/internal/storymaker"
	"aurago/internal/tools"
)

// storymakerMusicKey returns the key of the story's music. "" means nothing is to be played: no
// music, mode none, volume 0, or mode file with no source chosen yet. Only a file or track source
// that was set and no longer exists returns storymaker.ErrMusicSourceMissing. The film length is
// not part of any key.
func (s *Server) storymakerMusicKey(ctx context.Context, m *storymaker.Music) (string, error) {
	if m == nil || storymaker.MusicVolume(m) <= 0 {
		return "", nil
	}
	switch m.Mode {
	case "generated":
		return storymaker.MusicKeyGenerated(m.Mood, m.Prompt), nil
	case "file":
		if m.Track != "" {
			rel, err := s.storymakerTrackPath(m.Track)
			if err != nil {
				return "", err
			}
			info, err := s.storymakerMusicSource(ctx, "Music/"+rel)
			if err != nil {
				return "", err
			}
			return storymaker.MusicKeyTrack(m.Track, info.Size, info.ModTime), nil
		}
		if m.File == "" {
			return "", nil
		}
		info, err := s.storymakerMusicSource(ctx, m.File)
		if err != nil {
			return "", err
		}
		return storymaker.MusicKeyFile(m.File, info.Size, info.ModTime), nil
	}
	return "", nil
}

// storymakerTrackPath returns the path of a registered music track relative to DataDir/audio. A
// track that is gone, not music or outside that folder counts as a missing source.
func (s *Server) storymakerTrackPath(track string) (string, error) {
	id, err := strconv.ParseInt(track, 10, 64)
	if err != nil || id <= 0 {
		return "", storymaker.ErrMusicSourceMissing
	}
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil || item.MediaType != "music" {
		return "", storymaker.ErrMusicSourceMissing
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return "", errors.New("configuration unavailable")
	}
	root, err := filepath.Abs(filepath.Join(cfg.Directories.DataDir, "audio"))
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(item.FilePath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || !filepath.IsLocal(rel) {
		return "", storymaker.ErrMusicSourceMissing
	}
	return filepath.ToSlash(rel), nil
}

// storymakerMusicSource reads the size and time of a music source through the desktop store. It
// takes the desktop service directly, because a music job calls this too and a job must not call
// storymakerRuntime (frozen section 1.4).
func (s *Server) storymakerMusicSource(ctx context.Context, desktopPath string) (storymaker.FileInfo, error) {
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return storymaker.FileInfo{}, err
	}
	st := &storymakerStore{}
	st.use(svc)
	r, info, err := st.Open(ctx, desktopPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storymaker.FileInfo{}, storymaker.ErrMusicSourceMissing
		}
		return storymaker.FileInfo{}, err
	}
	_ = r.Close()
	return info, nil
}
