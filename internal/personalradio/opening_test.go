package personalradio

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpeningBeforeMusicReserve(t *testing.T) {
	for _, mode := range []string{"local", "generated"} {
		t.Run(mode, func(t *testing.T) {
			var plans, generations atomic.Int32
			requests := make(chan EditorialRequest, 2)
			s, _ := testService(t, Adapters{
				Plan: func(_ context.Context, req EditorialRequest) (Plan, error) {
					plans.Add(1)
					requests <- req
					return Plan{Moderation: "Welcome. We are preparing your music."}, nil
				},
				Speak: func(context.Context, Station, string) (Audio, error) {
					return Audio{Data: testWave(1, 9), Extension: "wav"}, nil
				},
				Generate: func(context.Context, Station, string, string) (Production, error) {
					generations.Add(1)
					return Production{}, errors.New("provider unavailable")
				},
			})
			p := testProfile(t, s, mode)
			p.Moderation = "balanced"
			p, err := s.SaveStation(p)
			if err != nil {
				t.Fatal(err)
			}
			st, err := s.Start(p.ID, testDevice, false)
			if err != nil {
				t.Fatal(err)
			}
			s.Tick()
			s.wg.Wait()
			req := <-requests
			if req.Opening == nil || req.Opening.TrackCount != 0 || req.Opening.BufferMS != 0 || req.Opening.RequiredMS != 300000 || req.Opening.MinTracks != 2 || req.Station.Mode != mode || req.News {
				t.Fatalf("opening lacks factual startup context: %+v", req)
			}
			got := s.Snapshot()
			if got.MusicReady || got.Status != "preparing" || got.OpeningStatus != "ready" || len(got.Queue) != 1 || !got.Queue[0].Opening || generations.Load() != 0 {
				t.Fatalf("opening must precede music production: %+v", got)
			}
			intro := got.Queue[0]
			if err = s.Playback(testDevice, st.Epoch, intro.ID, "started"); err != nil {
				t.Fatal(err)
			}
			s.Tick()
			s.wg.Wait()
			got = s.Snapshot()
			if got.Status != "playing" || got.OpeningStatus != "playing" || got.BufferMS != 0 || got.TrackCount != 0 || got.MusicReady || s.plays != 0 {
				t.Fatalf("speech must not count toward music reserve/history: %+v", got)
			}
			if (mode == "generated") != (generations.Load() == 1) {
				t.Fatal("music production did not respect source mode")
			}
			if err = s.Playback(testDevice, st.Epoch, intro.ID, "ended"); err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(filepath.Join(s.dir, intro.AssetID+".wav")); !os.IsNotExist(err) {
				t.Fatal("opening audio not cleaned up", err)
			}
			if got = s.Snapshot(); got.Status != "preparing" || got.OpeningStatus != "done" || got.MusicReady {
				t.Fatal(got)
			}
			if _, err = s.Start(p.ID, testDevice, false); err != nil {
				t.Fatal(err)
			}
			s.Tick()
			s.wg.Wait()
			if plans.Load() != 1 {
				t.Fatal("opening repeated during same session")
			}
			for i := 1; i <= 2; i++ {
				if _, err = s.Import(context.Background(), p.ID, bytes.NewReader(testWave(180, i)), "wav", "Music", mode, "Lo-fi", 0); err != nil {
					t.Fatal(err)
				}
				s.Tick()
				if i == 1 && s.Snapshot().MusicReady {
					t.Fatal("one track bypassed start reserve")
				}
			}
			got = s.Snapshot()
			if !got.MusicReady || got.Status != "ready" || len(got.Queue) < 2 || got.Queue[0].Kind != "music" || got.BufferMS != 360000 {
				t.Fatal(got)
			}
			if err = s.Playback(testDevice, st.Epoch, got.Queue[0].ID, "started"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOpeningFailureReleasesMusicAndKeepsQuota(t *testing.T) {
	var spoken, generated atomic.Int32
	s, _ := testService(t, Adapters{
		Plan: func(context.Context, EditorialRequest) (Plan, error) { return Plan{Moderation: "Welcome."}, nil },
		Speak: func(context.Context, Station, string) (Audio, error) {
			spoken.Add(1)
			return Audio{}, errors.New("radio_tts_unavailable")
		},
		Generate: func(context.Context, Station, string, string) (Production, error) {
			generated.Add(1)
			return Production{}, errors.New("provider unavailable")
		},
	})
	p := testProfile(t, s, "generated")
	p.Moderation = "balanced"
	p, err := s.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Start(p.ID, testDevice, false); err != nil {
		t.Fatal(err)
	}
	s.Tick()
	s.wg.Wait()
	s.Tick()
	s.wg.Wait()
	s.Tick()
	s.wg.Wait()
	got := s.Snapshot()
	if got.OpeningStatus != "failed" || got.EditorialCode != "radio_tts_unavailable" || len(got.Queue) != 0 || spoken.Load() != 1 || generated.Load() != 1 {
		t.Fatal(got, spoken.Load(), generated.Load())
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE kind IN ('editorial','tts_chars')").Scan(&count); err != nil || count != 2 {
		t.Fatal("failed requests lost their quotas", count, err)
	}
}

func TestOpeningStopFencesLatePlanner(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var spoken atomic.Int32
	s, _ := testService(t, Adapters{
		Plan: func(ctx context.Context, _ EditorialRequest) (Plan, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 46*time.Second {
				t.Error("opening lacks bounded priority deadline")
			}
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
			}
			return Plan{Moderation: "Late welcome."}, nil
		},
		Speak: func(context.Context, Station, string) (Audio, error) {
			spoken.Add(1)
			return Audio{Data: testWave(1, 2), Extension: "wav"}, nil
		},
	})
	p := testProfile(t, s, "local")
	p.Moderation = "balanced"
	p, err := s.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Start(p.ID, testDevice, false)
	if err != nil {
		t.Fatal(err)
	}
	s.Tick()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("planner not started")
	}
	if err = s.Control(testDevice, st.Epoch, "stop", ""); err != nil {
		t.Fatal(err)
	}
	close(release)
	s.wg.Wait()
	if got := s.Snapshot(); got.Status != "stopped" || len(got.Queue) != 0 || spoken.Load() != 0 {
		t.Fatal("late welcome revived stopped station", got)
	}
}

func TestOpeningWithWarmLibraryAndExpiredPause(t *testing.T) {
	s, clock := testService(t, Adapters{
		Plan: func(_ context.Context, req EditorialRequest) (Plan, error) {
			if req.Opening == nil || req.Opening.TrackCount != 2 || req.Opening.BufferMS != 360000 {
				t.Error("warm reserve not supplied")
			}
			return Plan{Moderation: "Welcome to your station."}, nil
		},
		Speak: func(context.Context, Station, string) (Audio, error) {
			return Audio{Data: testWave(1, 7), Extension: "wav"}, nil
		},
	})
	p := testProfile(t, s, "local")
	p.Moderation = "balanced"
	p, err := s.SaveStation(p)
	if err != nil {
		t.Fatal(err)
	}
	testImport(t, s, p, 1)
	testImport(t, s, p, 2)
	st, err := s.Start(p.ID, testDevice, false)
	if err != nil {
		t.Fatal(err)
	}
	s.Tick()
	s.wg.Wait()
	s.Tick()
	got := s.Snapshot()
	if !got.MusicReady || len(got.Queue) < 3 || !got.Queue[0].Opening || got.Queue[1].Kind != "music" {
		t.Fatal(got)
	}
	intro := got.Queue[0]
	if err = s.Playback(testDevice, st.Epoch, intro.ID, "started"); err != nil {
		t.Fatal(err)
	}
	if err = s.Control(testDevice, st.Epoch, "pause", ""); err != nil {
		t.Fatal(err)
	}
	clock.Add(int64(121 * time.Second))
	if err = s.Control(testDevice, st.Epoch, "resume", ""); err != nil {
		t.Fatal(err)
	}
	got = s.Snapshot()
	if got.OpeningStatus != "done" || got.Queue[0].Kind != "music" || got.Current != "" || got.Status != "ready" {
		t.Fatal("expired welcome replayed", got)
	}
}
