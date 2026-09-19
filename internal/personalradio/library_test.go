package personalradio

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestEmptyRegistryStartsAfterTwoTracksAndKeepsGenerating(t *testing.T) {
	var generated atomic.Int32
	third := make(chan struct{})
	originals := t.TempDir()
	s, _ := testService(t, Adapters{
		Library: func(context.Context, Station, func(LibraryTrack, io.ReadSeeker) error) error { return nil },
		Generate: func(ctx context.Context, _ Station, genre, _ string) (Production, error) {
			n := generated.Add(1)
			if n == 3 {
				close(third)
				<-ctx.Done()
				return Production{}, ctx.Err()
			}
			path := filepath.Join(originals, newID()+".wav")
			err := os.WriteFile(path, testWave(40, int(n)), 0600)
			return Production{Path: path, Genre: genre, MediaID: int64(n)}, err
		},
	})
	p := DefaultStation()
	p.Moderation = "off"
	p, err := s.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Start(p.ID, testDevice, false)
	if err != nil {
		t.Fatal(err)
	}
	s.Tick()
	s.wg.Wait() // The empty registry has been checked before buying any music.
	for n := int32(1); n <= 2; n++ {
		s.Tick()
		s.wg.Wait()
		if generated.Load() != n || s.Snapshot().MusicReady {
			t.Fatal("music started without two prepared tracks", generated.Load(), s.Snapshot())
		}
	}
	s.Tick()
	select {
	case <-third:
	case <-time.After(time.Second):
		t.Fatal("background generation did not continue")
	}
	got := s.Snapshot()
	if !got.MusicReady || got.RequiredMS != 0 || got.TrackCount != 2 || got.BufferMS != 80000 || !got.MusicBusy {
		t.Fatal("two prepared tracks did not release playback", got)
	}
	if err := s.Playback(testDevice, st.Epoch, got.Queue[0].ID, "started"); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Status != "playing" {
		t.Fatal("background generation blocked music")
	}
}

func TestLibraryRemovalAndPerStationGenre(t *testing.T) {
	s, _ := testService(t, Adapters{Library: func(ctx context.Context, p Station, accept func(LibraryTrack, io.ReadSeeker) error) error {
		return accept(LibraryTrack{MediaID: 123, Title: "Existing", Origin: "generated", Genre: p.Genres[0].Name, Extension: "wav"}, bytes.NewReader(testWave(180, 1)))
	}})
	p := testProfile(t, s, "mixed")
	s.Start(p.ID, testDevice, false)
	s.Tick()
	s.wg.Wait()
	tracks, err := s.Tracks(p.ID)
	if err != nil || len(tracks) != 1 {
		t.Fatal(tracks, err)
	}
	id := tracks[0].ID
	// A missing owned copy is repaired from the registry, without a new identity.
	s.Control(testDevice, s.Snapshot().Epoch, "stop", "")
	path := filepath.Join(s.dir, id+".wav")
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.Start(p.ID, testDevice, false)
	s.Tick()
	s.wg.Wait()
	if info, err := os.Stat(path); err != nil || info.Size() != tracks[0].Bytes {
		t.Fatal("registry cache was not repaired", err)
	}
	// Blocking a track must survive automatic discovery and retain its settings.
	s.Control(testDevice, s.Snapshot().Epoch, "stop", "")
	if err = s.UpdateTrack(p.ID, id, true, true, 25); err != nil {
		t.Fatal(err)
	}
	s.Start(p.ID, testDevice, false)
	s.Tick()
	s.wg.Wait()
	tracks, _ = s.Tracks(p.ID)
	if len(tracks) != 1 || !tracks[0].Blocked || !tracks[0].Favorite || tracks[0].Weight != 25 {
		t.Fatal("automatic discovery undid track controls", tracks)
	}
	if err = s.RemoveTrack(p.ID, id); err != nil {
		t.Fatal(err)
	}
	s.Control(testDevice, s.Snapshot().Epoch, "stop", "")
	s.Start(p.ID, testDevice, false)
	s.Tick()
	s.wg.Wait()
	tracks, _ = s.Tracks(p.ID)
	if len(tracks) != 0 {
		t.Fatal("removed registry music silently reimported")
	}
	s.Control(testDevice, s.Snapshot().Epoch, "stop", "")
	// An explicit import can restore the association, preserving other stations.
	if _, err = s.Import(context.Background(), p.ID, bytes.NewReader(testWave(180, 1)), "wav", "Existing", "generated", "Lo-fi", 123); err != nil {
		t.Fatal(err)
	}
	other := testProfile(t, s, "mixed")
	other.Genres = []Genre{{Name: "Jazz", Weight: 1}}
	other, err = s.SaveStation(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Import(context.Background(), other.ID, bytes.NewReader(testWave(180, 1)), "wav", "Existing", "generated", "Jazz", 123); err != nil {
		t.Fatal(err)
	}
	a, _ := s.Tracks(p.ID)
	b, _ := s.Tracks(other.ID)
	if a[0].ID != b[0].ID || a[0].Genre != "Lo-fi" || b[0].Genre != "Jazz" {
		t.Fatal("station genre changed shared asset", a, b)
	}
}

func TestLegacyFactoryReserveMigration(t *testing.T) {
	s, _ := testService(t, Adapters{})
	p := DefaultStation()
	p.ReserveMinutes = 30
	p.MinTracks = 8
	p, err := s.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	custom := DefaultStation()
	custom.ReserveMinutes = 10
	custom.MinTracks = 5
	custom, err = s.SaveStation(custom)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("ALTER TABLE station_tracks DROP COLUMN genre; UPDATE schema_meta SET version=1"); err != nil {
		t.Fatal(err)
	}
	dir := s.dir
	s.Close()
	r, err := New(Options{Directory: dir, Manual: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	updated, err := r.station(p.ID)
	if err != nil || updated.ReserveMinutes != 0 || updated.MinTracks != 2 || updated.Revision != p.Revision+1 {
		t.Fatal(updated, err)
	}
	retained, err := r.station(custom.ID)
	if err != nil || retained.ReserveMinutes != 10 || retained.MinTracks != 5 || retained.Revision != custom.Revision {
		t.Fatal(retained, err)
	}
	updated.ReserveMinutes = 30
	updated.MinTracks = 8
	if _, err = r.SaveStation(updated); err != nil {
		t.Fatal(err)
	}
	r.Close()
	r, err = New(Options{Directory: dir, Manual: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	updated, err = r.station(p.ID)
	if err != nil || updated.ReserveMinutes != 30 || updated.MinTracks != 8 {
		t.Fatal("migration repeated over explicit settings", updated, err)
	}
}
