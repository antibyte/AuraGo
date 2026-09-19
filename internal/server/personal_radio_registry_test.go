package server

import (
	"aurago/internal/config"
	"aurago/internal/personalradio"
	"aurago/internal/tools"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func radioRegistryFixture(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	db, err := tools.InitMediaRegistryDB(filepath.Join(cfg.Directories.DataDir, "registry.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &Server{Cfg: cfg, MediaRegistryDB: db}
}

func TestPersonalRadioRegistryMetadataSelectionAndPaths(t *testing.T) {
	s := radioRegistryFixture(t)
	write := func(name string, seed int) string {
		path := filepath.Join(s.Cfg.Directories.DataDir, name)
		if err := os.WriteFile(path, personalRadioTestWave(8000, 40, seed), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a, b := write("lofi.wav", 1), write("lofi2.wav", 2)
	add := func(path, prompt, style string, tags []string) int64 {
		id, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{MediaType: "music", SourceTool: "generate_music", Filename: filepath.Base(path), FilePath: path, Description: "Existing music", Prompt: prompt, Style: style, Tags: tags})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := add(a, "warm lofi beats", "", []string{"instrumental"})
	second := add(b, "", "Lo Fi, mellow", []string{"instrumental"})
	// Older matches beyond the first two pages must remain discoverable.
	for i := 0; i < 420; i++ {
		add(a, fmt.Sprintf("orchestral metal %d", i), "", nil)
	}
	add(filepath.Join(t.TempDir(), "outside.wav"), "lofi", "", nil)
	add(a, "lofi", "", []string{"vocals"})
	if _, err := s.MediaRegistryDB.Exec("UPDATE media_items SET created_at=printf('%09d',id)"); err != nil {
		t.Fatal(err)
	}
	p := personalradio.DefaultStation()
	var found []int64
	err := s.personalRadioLibrary(context.Background(), p, func(track personalradio.LibraryTrack, r io.ReadSeeker) error {
		found = append(found, track.MediaID)
		if track.Genre != "Lo-fi" || track.Origin != "generated" {
			t.Error(track)
		}
		header := make([]byte, 4)
		if _, err := io.ReadFull(r, header); err != nil || string(header) != "RIFF" {
			t.Fatal("original not readable", err)
		}
		return nil
	})
	if err != nil || len(found) != 2 || !slices.Contains(found, first) || !slices.Contains(found, second) {
		t.Fatal(found, err)
	}
	p.Genres = []personalradio.Genre{{Name: "Rock", Weight: 1}}
	if _, score := radioRegistryMatch(p, tools.MediaItem{MediaType: "music", Prompt: "A song about rockets"}); score != 0 {
		t.Fatal("genre matched inside unrelated word")
	}
}

func TestPersonalRadioRegistryStartsWithoutWaitingForNewGeneration(t *testing.T) {
	s := radioRegistryFixture(t)
	for i := 1; i <= 2; i++ {
		path := filepath.Join(s.Cfg.Directories.DataDir, fmt.Sprintf("track-%d.wav", i))
		if err := os.WriteFile(path, personalRadioTestWave(8000, 40, i), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{MediaType: "music", SourceTool: "generate_music", Filename: filepath.Base(path), FilePath: path, Prompt: "Lo-fi instrumental", Tags: []string{"instrumental"}}); err != nil {
			t.Fatal(err)
		}
	}
	entered := make(chan struct{}, 1)
	svc, err := personalradio.New(personalradio.Options{Directory: filepath.Join(s.Cfg.Directories.DataDir, "radio"), Manual: true, Adapters: personalradio.Adapters{Library: s.personalRadioLibrary, Generate: func(ctx context.Context, _ personalradio.Station, _, _ string) (personalradio.Production, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return personalradio.Production{}, ctx.Err()
	}}})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	p := personalradio.DefaultStation()
	p.Moderation = "off"
	p.LibraryMinutes = 1
	p.RepeatTracks = 0
	p, err = svc.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.Start(p.ID, "registry-device-0001", false)
	if err != nil {
		t.Fatal(err)
	}
	svc.Tick()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && svc.Snapshot().LibraryStatus != "ready" {
		time.Sleep(5 * time.Millisecond)
	}
	svc.Tick()
	got := svc.Snapshot()
	if !got.MusicReady || got.TrackCount != 2 || got.RequiredMS != 0 || got.BufferMS != 80000 || len(got.Queue) < 2 {
		t.Fatalf("existing music did not start promptly: %+v", got)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("fresh matching music disabled by full existing library")
	}
	if err = svc.Playback("registry-device-0001", st.Epoch, got.Queue[0].ID, "started"); err != nil {
		t.Fatal(err)
	}
	if got = svc.Snapshot(); got.Status != "playing" || !got.MusicBusy {
		t.Fatal("slow generation blocked playback", got)
	}
}

func TestPersonalRadioRegistrationPreservesCompleteMetadataOnRetry(t *testing.T) {
	s := radioRegistryFixture(t)
	path := filepath.Join(s.Cfg.Directories.DataDir, "new.wav")
	if err := os.WriteFile(path, personalRadioTestWave(8000, 1, 7), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := tools.ComputeMediaFileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{MediaType: "music", SourceTool: "generate_music", Filename: "new.wav", FilePath: path, Hash: hash, Tags: []string{"favorite"}})
	if err != nil {
		t.Fatal(err)
	}
	production := personalradio.Production{Path: path, Title: "New original", Genre: "Lo-fi", MediaID: id, Prompt: "An original piano arrangement", Style: "Lo-fi, warm, 80 BPM", Lyrics: "Original lyrics", Language: "de", Provider: "acestep", Model: "test-model", Tags: []string{"vocals", "station:test"}, DurationMS: 1000, GenerationTimeMS: 60000, CostEstimate: .05}
	// A metadata failure leaves the original record available for the durable retry.
	if _, err = s.MediaRegistryDB.Exec("CREATE TRIGGER fail_radio_metadata BEFORE UPDATE OF style ON media_items BEGIN SELECT RAISE(FAIL,'temporary failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.personalRadioRegister(context.Background(), production); err == nil {
		t.Fatal("metadata failure silently accepted")
	}
	s.MediaRegistryDB.Exec("DROP TRIGGER fail_radio_metadata")
	for i := 0; i < 2; i++ {
		result, e := s.personalRadioRegister(context.Background(), production)
		if e != nil || result.MediaID != id {
			t.Fatal(result, e)
		}
	}
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Prompt != production.Prompt || item.Style != production.Style || item.Lyrics != production.Lyrics || item.Language != "de" || item.Provider != "acestep" || item.Model != "test-model" || item.DurationMs != 1000 || item.GenerationTimeMs != 60000 || item.CostEstimate != .05 || !slices.Contains(item.Tags, "favorite") || !slices.Contains(item.Tags, "Lo-fi") || !slices.Contains(item.Tags, "station:test") {
		t.Fatalf("metadata lost: %+v", item)
	}
	_, total, err := tools.SearchMedia(s.MediaRegistryDB, "", "music", nil, 10, 0)
	if err != nil || total != 1 {
		t.Fatal("duplicate generation registration", total, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.personalRadioRegister(ctx, production); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
