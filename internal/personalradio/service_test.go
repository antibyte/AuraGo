package personalradio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func testWave(seconds, seed int) []byte {
	const rate = 8000
	b := append(waveHeader(rate, 1, int64(seconds*rate*2)), make([]byte, seconds*rate*2)...)
	for i := 44; i < len(b); i += 2 {
		binary.LittleEndian.PutUint16(b[i:], uint16((i+seed)%8000))
	}
	return b
}
func testService(t *testing.T, adapters Adapters) (*Service, *atomic.Int64) {
	t.Helper()
	clock := new(atomic.Int64)
	clock.Store(time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC).UnixNano())
	s, err := New(Options{Directory: t.TempDir(), Manual: true, Adapters: adapters, Now: func() time.Time { return time.Unix(0, clock.Load()).UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, clock
}
func testProfile(t *testing.T, s *Service, mode string) Station {
	t.Helper()
	p := DefaultStation()
	p.Mode = mode
	p.ReserveMinutes = 5
	p.MinTracks = 2
	p.LibraryMinutes = 6
	p.Moderation = "off"
	p, err := s.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func testImport(t *testing.T, s *Service, p Station, seed int) Track {
	t.Helper()
	track, err := s.Import(context.Background(), p.ID, bytes.NewReader(testWave(180, seed)), "wav", "Track", "local", "Lo-fi", 0)
	if err != nil {
		t.Fatal(err)
	}
	return track
}

const testDevice = "listener-device-0001"

func TestColdStartPlaybackLeaseAndPersistence(t *testing.T) {
	s, clock := testService(t, Adapters{})
	p := testProfile(t, s, "local")
	st, err := s.Start(p.ID, testDevice, false)
	if err != nil {
		t.Fatal(err)
	}
	s.Tick()
	if got := s.Snapshot(); got.Status != "preparing" || len(got.Queue) != 0 {
		t.Fatalf("empty start: %+v", got)
	}
	first := testImport(t, s, p, 1)
	s.Tick()
	if s.Snapshot().Status != "preparing" {
		t.Fatal("started with one title")
	}
	second := testImport(t, s, p, 2)
	s.Tick()
	if s.Snapshot().Status != "ready" {
		t.Fatal(s.Snapshot())
	}
	if _, err = s.Start(p.ID, "another-device-0002", false); !errors.Is(err, ErrLease) {
		t.Fatal(err)
	}
	if _, err = s.SaveStation(p); !errors.Is(err, ErrConflict) {
		t.Fatal("active revision editable", err)
	}
	// Ninety minutes of actual lifecycle events, with deterministic time and
	// fully decoded assets. This is a scheduler soak, not a wall-clock soak.
	previous := ""
	for i := 0; i < 30; i++ {
		s.Tick()
		q := s.Snapshot().Queue
		if len(q) < 2 {
			t.Fatal("queue depleted", i, q)
		}
		if q[0].TrackID == previous {
			t.Fatal("adjacent repeat", i)
		}
		previous = q[0].TrackID
		if err = s.Playback(testDevice, st.Epoch, q[0].ID, "started"); err != nil {
			t.Fatal(err)
		}
		if err = s.Playback(testDevice, st.Epoch, q[0].ID, "started"); err != nil {
			t.Fatal("duplicate", err)
		}
		for j := 0; j < 2; j++ {
			clock.Add(int64(90 * time.Second))
			if err = s.Heartbeat(testDevice, st.Epoch, q[0].ID, int64((j+1)*90000)); err != nil {
				t.Fatal(err)
			}
		}
		if err = s.Playback(testDevice, st.Epoch, q[0].ID, "ended"); err != nil {
			t.Fatal(err)
		}
	}
	tracks, _ := s.Tracks(p.ID)
	total := 0
	for _, x := range tracks {
		total += x.Plays
	}
	if total != 30 {
		t.Fatal("planned or duplicate plays counted", total)
	}
	if err = s.Control(testDevice, st.Epoch, "pause", ""); err != nil {
		t.Fatal(err)
	}
	s.Tick()
	if s.Snapshot().Status != "paused" {
		t.Fatal("pause lost")
	}
	clock.Add(int64(4 * time.Minute))
	s.Tick()
	if s.Snapshot().Status != "stopped" {
		t.Fatal("listener lease leaked")
	}
	if err = s.Playback(testDevice, st.Epoch, "old", "started"); !errors.Is(err, ErrLease) {
		t.Fatal("old epoch accepted")
	}
	dir := s.dir
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	resumed, err := New(Options{Directory: dir, Manual: true})
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	if resumed.Snapshot().Status != "stopped" {
		t.Fatal("restart autoplay")
	}
	tracks, _ = resumed.Tracks(p.ID)
	if len(tracks) != 2 || !slices.ContainsFunc(tracks, func(x Track) bool { return x.ID == first.ID }) || !slices.ContainsFunc(tracks, func(x Track) bool { return x.ID == second.ID }) {
		t.Fatal("library lost")
	}
	total = 0
	for _, x := range tracks {
		total += x.Plays
	}
	if total != 30 {
		t.Fatal("history lost", total)
	}
}

func TestAudioValidationWindowsAndDedup(t *testing.T) {
	s, _ := testService(t, Adapters{})
	p := testProfile(t, s, "local")
	a := testImport(t, s, p, 2)
	b := testImport(t, s, p, 2)
	if a.ID != b.ID {
		t.Fatal("duplicate copy created")
	}
	raw, err := s.AudioWindow(a.ID, 1000, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 44+8000*2*30 || binary.LittleEndian.Uint32(raw[24:]) != 8000 {
		t.Fatal("window format")
	}
	for _, bad := range []struct {
		id             string
		offset, length int64
	}{{"../../vault", 0, 1}, {a.ID, -1, 1}, {a.ID, 1 << 62, 1000}, {a.ID, 180000, 1000}, {a.ID, 0, 60001}} {
		if _, err = s.AudioWindow(bad.id, bad.offset, bad.length); err == nil {
			t.Fatal("unsafe window accepted", bad)
		}
	}
	original := testWave(1, 1)
	for name, data := range map[string][]byte{"garbage": []byte("bad"), "truncated": original[:len(original)-100], "half-frame": original[:len(original)-1]} {
		t.Run(name, func(t *testing.T) {
			if _, err := prepareAudio(context.Background(), bytes.NewReader(data), "wav", filepath.Join(t.TempDir(), "out.wav")); err == nil {
				t.Fatal("invalid WAV accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dest := filepath.Join(t.TempDir(), "out.wav")
	if _, err = prepareAudio(ctx, bytes.NewReader(original), "wav", dest); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("partial file leaked")
	}
	if _, err = s.DeleteUnused(); err != nil {
		t.Fatal(err)
	}
	if err = s.RemoveTrack(p.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := s.DeleteUnused(); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}

func TestMP3ImportPreservesSampleRateAndStereo(t *testing.T) {
	// Original synthetic 440 Hz tone, generated with FFmpeg's sine source and
	// libmp3lame at 44.1 kHz, stereo, 64 kbit/s. No external runtime is required.
	s, _ := testService(t, Adapters{})
	p := testProfile(t, s, "local")
	f, err := os.Open("testdata/tone-44100.mp3")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	track, err := s.Import(context.Background(), p.ID, f, "mp3", "Test tone", "local", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if track.Rate != 44100 || track.Channels != 2 || track.DurationMS < 1000 || track.DurationMS > 1200 {
		t.Fatal("MP3 decoded incorrectly", track)
	}
	wav, err := s.AudioWindow(track.ID, 0, 1000)
	if err != nil || len(wav) != 44+44100*4 {
		t.Fatal("stereo PCM window", err, len(wav))
	}
	nonzero := false
	for _, b := range wav[44:] {
		nonzero = nonzero || b != 0
	}
	if !nonzero {
		t.Fatal("MP3 imported silence")
	}
}

func TestRotationStrictSoftAndGenreMix(t *testing.T) {
	p := DefaultStation()
	p.RepeatMinutes = 60
	p.RepeatTracks = 20
	p.Mode = "mixed"
	p.Genres = []Genre{{"Jazz", 1}, {"Ambient", 1}}
	now := time.Now()
	var tracks []Track
	for i := 0; i < 30; i++ {
		origin := "local"
		genre := "Jazz"
		if i%2 == 0 {
			origin = "generated"
			genre = "Ambient"
		}
		tracks = append(tracks, Track{ID: newID(), Origin: origin, Genre: genre, DurationMS: 180000, Weight: 100})
	}
	var recent []string
	last := map[string]time.Time{}
	generated := 0
	for i := 0; i < 100; i++ {
		track, relaxed, ok := selectTrack(p, tracks, recent, last, now, nil)
		if !ok || relaxed {
			t.Fatal("rotation failed with enough music", i)
		}
		if slices.Contains(recent[max(0, len(recent)-20):], track.ID) {
			t.Fatal("cooldown violated")
		}
		if track.Origin == "generated" {
			generated++
		}
		recent = append(recent, track.ID)
		last[track.ID] = now
		now = now.Add(3 * time.Minute)
	}
	if generated < 45 || generated > 55 {
		t.Fatal("duration mix", generated)
	}
	tiny := tracks[:2]
	recent = []string{tiny[0].ID, tiny[1].ID}
	last = map[string]time.Time{tiny[0].ID: now, tiny[1].ID: now}
	p.Strict = true
	if _, _, ok := selectTrack(p, tiny, recent, last, now, nil); ok {
		t.Fatal("strict silently relaxed")
	}
	p.Strict = false
	if got, relaxed, ok := selectTrack(p, tiny, recent, last, now, nil); !ok || !relaxed || got.ID != tiny[0].ID {
		t.Fatal("soft fallback", got, relaxed, ok)
	}
	tiny[0].Blocked = true
	if _, _, ok := selectTrack(p, tiny, recent, last, now, nil); ok {
		t.Fatal("blocked or adjacent repeated")
	}
}

func TestProductionCancellationAndDurableQuota(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls atomic.Int64
	s, _ := testService(t, Adapters{Generate: func(ctx context.Context, p Station, g, i string) (Production, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		return Production{}, ctx.Err()
	}})
	p := testProfile(t, s, "generated")
	p.DailyGenerations = 1
	p, _ = s.SaveStation(p)
	st, _ := s.Start(p.ID, testDevice, false)
	s.Tick()
	<-entered
	for i := 0; i < 10; i++ {
		s.Tick()
	}
	if calls.Load() != 1 {
		t.Fatal("parallel duplicate production")
	}
	if err := s.Control(testDevice, st.Epoch, "stop", ""); err != nil {
		t.Fatal(err)
	}
	close(release)
	s.wg.Wait()
	if got := s.Snapshot(); got.Status != "stopped" || len(got.Queue) > 0 {
		t.Fatal("late result revived station")
	}
	dir := s.dir
	s.Close()
	other, err := New(Options{Directory: dir, Manual: true, Adapters: s.adapters})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	other.Start(p.ID, testDevice, false)
	other.Tick()
	if other.Snapshot().GeneratedToday != 1 || calls.Load() != 1 {
		t.Fatal("failed reservation refunded across restart")
	}
}

func TestNewsDeadlinesExpiryAndUntrustedSourceIDs(t *testing.T) {
	p := DefaultStation()
	p.Topics = "Technology"
	p.NewsMinutes = 30
	p.Timezone = "Europe/Berlin"
	for _, iso := range []string{"2026-03-29T00:59:00Z", "2026-10-25T00:59:00Z", "2026-09-19T08:30:00Z"} {
		now, _ := time.Parse(time.RFC3339, iso)
		due := nextNews(now, p)
		loc, _ := time.LoadLocation(p.Timezone)
		if !due.After(now) || due.Sub(now) > 30*time.Minute || due.In(loc).Minute()%30 != 0 {
			t.Fatal("DST deadline", now, due)
		}
	}
	var spoken atomic.Int64
	s, clock := testService(t, Adapters{Research: func(context.Context, Station) ([]Source, error) {
		return []Source{{ID: "verified", Published: time.Now(), Text: "source"}}, nil
	}, Plan: func(context.Context, EditorialRequest) (Plan, error) {
		return Plan{NewsText: "Invented bulletin", SourceIDs: []string{"invented"}}, nil
	}, Speak: func(context.Context, Station, string) (Audio, error) {
		spoken.Add(1)
		return Audio{testWave(1, 3), "wav"}, nil
	}})
	p = testProfile(t, s, "local")
	p.Topics = "Technology"
	p.NewsMinutes = 30
	p.NewsTopics = true
	p, _ = s.SaveStation(p)
	testImport(t, s, p, 1)
	testImport(t, s, p, 2)
	st, _ := s.Start(p.ID, testDevice, false)
	s.Tick()
	q := s.Snapshot().Queue
	s.Playback(testDevice, st.Epoch, q[0].ID, "started")
	s.wg.Wait()
	clock.Add(int64(23 * time.Minute))
	s.mu.Lock()
	s.leaseUntil = s.now().Add(time.Minute)
	s.mu.Unlock()
	s.Tick()
	s.wg.Wait()
	if spoken.Load() != 0 || s.Snapshot().NewsCode != "radio_editorial_failed" {
		t.Fatal("unverified bulletin synthesized", s.Snapshot())
	}
	s.mu.Lock()
	expired := Segment{ID: newID(), AssetID: newID(), Kind: "news", Expires: s.now().Add(-time.Second)}
	s.state.Queue = append(s.state.Queue[:1], expired)
	s.mu.Unlock()
	s.Tick()
	if slices.ContainsFunc(s.Snapshot().Queue, func(x Segment) bool { return x.ID == expired.ID }) {
		t.Fatal("expired bulletin remained")
	}
}
