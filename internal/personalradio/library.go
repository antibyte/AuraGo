package personalradio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errLibraryFull = errors.New("radio_library_ready")

// Library reads and PCM preparation run independently of speech and music jobs.
// Playback may start as soon as two titles are ready, before the scan completes.
func (s *Service) scheduleLibraryLocked(p Station) {
	if s.adapters.Library == nil {
		s.state.LibraryStatus = "off"
		return
	}
	if s.libraryActive || s.state.LibraryStatus == "ready" || s.now().Before(s.libraryRetry) {
		return
	}
	s.libraryActive = true
	s.state.LibraryStatus = "searching"
	s.wg.Add(1)
	go s.prepareLibrary(s.runCtx, s.state.Epoch, p)
}

func (s *Service) prepareLibrary(parent context.Context, epoch string, p Station) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	err := s.adapters.Library(ctx, p, func(item LibraryTrack, input io.ReadSeeker) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.mu.Lock()
		if s.state.Epoch != epoch || s.closed {
			s.mu.Unlock()
			return context.Canceled
		}
		tracks := s.updateBufferLocked(p)
		if s.state.BufferMS >= int64(p.LibraryMinutes)*60000 && len(tracks) >= max(p.MinTracks, p.RepeatTracks+1) {
			s.mu.Unlock()
			return errLibraryFull
		}
		var known int
		err := s.db.QueryRow("SELECT 1 FROM registry_ignored WHERE station=? AND media_id=?", p.ID, item.MediaID).Scan(&known)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			s.mu.Unlock()
			return err
		}
		ignored := err == nil
		// Include blocked associations: automatic discovery must never undo a block.
		var body string
		var blocked bool
		err = s.db.QueryRow("SELECT a.body,t.blocked FROM station_tracks t JOIN assets a ON a.id=t.asset WHERE t.station=? AND json_extract(a.body,'$.media_id')=?", p.ID, item.MediaID).Scan(&body, &blocked)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			s.mu.Unlock()
			return err
		}
		cached := false
		if err == nil {
			var track Track
			if err = json.Unmarshal([]byte(body), &track); err != nil {
				s.mu.Unlock()
				return err
			}
			if info, e := os.Stat(filepath.Join(s.dir, track.ID+".wav")); e == nil && info.Size() == track.Bytes {
				cached = true
			}
		}
		s.state.LibraryStatus = "importing"
		s.mu.Unlock()
		if ignored || blocked || cached {
			return nil
		}
		_, err = s.Import(ctx, p.ID, input, item.Extension, item.Title, item.Origin, item.Genre, item.MediaID)
		if errors.Is(err, ErrLimit) {
			return errLibraryFull
		}
		// A missing/invalid individual file does not prevent using the rest.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && !strings.HasPrefix(err.Error(), "radio_invalid") && !strings.HasPrefix(err.Error(), "radio_format") && !strings.HasPrefix(err.Error(), "radio_truncated") && !strings.HasPrefix(err.Error(), "radio_empty") && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}
		return nil
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.libraryActive = false
	if s.state.Epoch != epoch || s.closed {
		return
	}
	if err != nil && !errors.Is(err, errLibraryFull) {
		s.state.LibraryStatus = "failed"
		s.state.Code = "radio_registry_unavailable"
		s.libraryRetry = s.now().Add(time.Minute)
		return
	}
	s.state.LibraryStatus = "ready"
	if s.state.Code == "radio_registry_unavailable" {
		s.state.Code = ""
	}
	s.updateBufferLocked(p)
}
