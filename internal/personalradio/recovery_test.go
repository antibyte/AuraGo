package personalradio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestGeneratedColdStartRecoversRegistrationWithoutRegeneration(t *testing.T) {
	sourceDir := t.TempDir()
	var generated, registered atomic.Int64
	adapters := Adapters{Generate: func(ctx context.Context, p Station, g, i string) (Production, error) {
		n := generated.Add(1)
		path := filepath.Join(sourceDir, newID()+".wav")
		err := os.WriteFile(path, testWave(180, int(n)), 0600)
		return Production{Path: path, Title: "Generated", Genre: g}, err
	}, Register: func(ctx context.Context, p Production) (Production, error) {
		if registered.Add(1) == 1 {
			return p, errors.New("temporary registry failure")
		}
		p.MediaID = registered.Load()
		return p, nil
	}}
	s, clock := testService(t, adapters)
	p := testProfile(t, s, "generated")
	p.DailyGenerations = 2
	p, _ = s.SaveStation(p)
	s.Start(p.ID, testDevice, false)
	s.Tick()
	s.wg.Wait()
	if generated.Load() != 1 || s.Snapshot().Status != "preparing" {
		t.Fatal("cold start released prematurely")
	}
	dir := s.dir
	s.Close()
	resumed, err := New(Options{Directory: dir, Manual: true, Adapters: adapters, Now: func() time.Time { return time.Unix(0, clock.Load()) }})
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	resumed.Start(p.ID, testDevice, false)
	resumed.Tick()
	resumed.wg.Wait()
	if generated.Load() != 1 {
		t.Fatal("registered retry charged a new generation")
	}
	tracks, _ := resumed.Tracks(p.ID)
	if len(tracks) != 1 || tracks[0].MediaID == 0 {
		t.Fatal("recovery missing registration", tracks)
	}
	resumed.Tick()
	resumed.wg.Wait()
	if resumed.Snapshot().Status != "preparing" {
		t.Fatal("unvalidated job counted")
	}
	resumed.Tick()
	st := resumed.Snapshot()
	if generated.Load() != 2 || st.GeneratedToday != 2 || st.Status != "ready" || st.BufferMS != 360000 {
		t.Fatal("cold start failed", st, generated.Load())
	}
}

func TestControlRetryAndExpiredSpeechResume(t *testing.T) {
	s, clock := testService(t, Adapters{})
	p := testProfile(t, s, "local")
	testImport(t, s, p, 1)
	testImport(t, s, p, 2)
	st, _ := s.Start(p.ID, testDevice, false)
	s.Tick()
	id := s.Snapshot().Queue[0].ID
	if err := s.Control(testDevice, st.Epoch, "skip", id); err != nil {
		t.Fatal(err)
	}
	first := s.Snapshot().Queue[0].ID
	if err := s.Control(testDevice, st.Epoch, "skip", id); err != nil || first != s.Snapshot().Queue[0].ID {
		t.Fatal("duplicate skip advanced again", err)
	}
	s.Playback(testDevice, st.Epoch, first, "started")
	s.Control(testDevice, st.Epoch, "pause", "")
	s.Control(testDevice, st.Epoch, "resume", "")
	if s.Snapshot().Status != "playing" {
		t.Fatal("resume lost current playback")
	}
	s.mu.Lock()
	s.state.Queue[0].Kind = "news"
	s.state.Queue[0].Expires = s.now().Add(time.Minute)
	s.mu.Unlock()
	s.Control(testDevice, st.Epoch, "pause", "")
	clock.Add(int64(2 * time.Minute))
	s.Control(testDevice, st.Epoch, "resume", "")
	if s.Snapshot().Current != "" || s.Snapshot().Queue[0].ID == first {
		t.Fatal("expired paused bulletin resumed")
	}
	if err := s.Control(testDevice, st.Epoch, "stop", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Control(testDevice, st.Epoch, "stop", ""); err != nil {
		t.Fatal("stop not idempotent", err)
	}
	next, _ := s.Start(p.ID, testDevice, false)
	if err := s.Control(testDevice, st.Epoch, "stop", ""); !errors.Is(err, ErrLease) || s.Snapshot().Epoch != next.Epoch {
		t.Fatal("old stop killed new run", err)
	}
}

func TestStrictColdStartCannotCountCoolingTitles(t *testing.T) {
	s, _ := testService(t, Adapters{})
	p := testProfile(t, s, "local")
	p.Strict = true
	p, _ = s.SaveStation(p)
	a := testImport(t, s, p, 1)
	testImport(t, s, p, 2)
	s.db.Exec("INSERT INTO plays(segment,station,asset,started) VALUES(?,?,?,?)", newID(), p.ID, a.ID, s.now().Format(time.RFC3339Nano))
	s.Start(p.ID, testDevice, false)
	s.Tick()
	if s.Snapshot().Status != "preparing" || len(s.Snapshot().Queue) != 0 {
		t.Fatal("cooling music counted toward strict reserve")
	}
}

func TestNewsTwoDeadlinesAndDeduplication(t *testing.T) {
	var clock *atomic.Int64
	var researched, spoken atomic.Int64
	adapters := Adapters{Research: func(context.Context, Station) ([]Source, error) {
		researched.Add(1)
		return []Source{{ID: "unchanged", Published: time.Unix(0, clock.Load()).Truncate(time.Hour), Retrieved: time.Unix(0, clock.Load()), Text: "Verified development", URL: "https://example.test/news"}}, nil
	}, Plan: func(_ context.Context, r EditorialRequest) (Plan, error) {
		if !r.News {
			return Plan{Theme: "Quiet music"}, nil
		}
		return Plan{NewsText: "A verified development.", SourceIDs: []string{r.Sources[0].ID}}, nil
	}, Speak: func(context.Context, Station, string) (Audio, error) {
		spoken.Add(1)
		return Audio{testWave(1, 1), "wav"}, nil
	}}
	s, c := testService(t, adapters)
	clock = c
	p := testProfile(t, s, "local")
	p.Topics = "Technology"
	p.NewsMinutes = 30
	p, _ = s.SaveStation(p)
	testImport(t, s, p, 1)
	testImport(t, s, p, 2)
	st, _ := s.Start(p.ID, testDevice, false)
	s.Tick()
	s.wg.Wait()
	var heard int
	// Pass two deadlines while research runs with a current music segment.
	for i := 0; i < 50; i++ {
		s.Tick()
		s.wg.Wait()
		q := s.Snapshot().Queue
		if len(q) == 0 {
			t.Fatal("music stopped")
		}
		x := q[0]
		if err := s.Playback(testDevice, st.Epoch, x.ID, "started"); err != nil {
			t.Fatal(err)
		}
		if x.Kind == "news" {
			heard++
		}
		clock.Add(int64(90 * time.Second))
		s.Heartbeat(testDevice, st.Epoch, x.ID, 90000)
		s.Tick()
		s.wg.Wait()
		if err := s.Playback(testDevice, st.Epoch, x.ID, "ended"); err != nil {
			t.Fatal(err)
		}
	}
	if researched.Load() != 2 || spoken.Load() != 1 || heard != 1 {
		t.Fatal("unchanged news repeated or deadlines missed", researched.Load(), spoken.Load(), heard)
	}
	if len(s.News(p.ID)) != 1 || s.News(p.ID)[0].Aired.IsZero() {
		t.Fatal("airtime history missing")
	}
}
