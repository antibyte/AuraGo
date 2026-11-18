package newspaper

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Drive the clock and scheduler explicitly to exercise restart and grace boundaries.
func audioTestService(t *testing.T) (*Service, *time.Time, *Policy) {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "newspaper.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	policy := Policy{Enabled: true, Telegram: true, WhatsApp: true, Cast: true}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{store: store, ctx: ctx, cancel: cancel, now: func() time.Time { return now }, policy: func() Policy { return policy }, audioJobs: make(map[string]*audioJob), audioSlot: make(chan struct{}, 1), wake: make(chan struct{}, 1)}
	s.destination = func(context.Context, Profile, string) (string, error) { return "owner", nil }
	s.deliver = func(context.Context, Edition, Profile, string) (string, error) { return "edition", nil }
	s.narrate = func(context.Context, Edition) (AudioFile, error) {
		return AudioFile{Data: []byte("one recording"), MIME: "audio/mpeg"}, nil
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s, &now, &policy
}

func audioReceipts(t *testing.T, s *Service, e Edition, expected int) []Delivery {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		rows, err := s.Deliveries(ctx, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		pending := false
		for _, d := range rows {
			pending = pending || d.Status == "sending" || d.Status == "preparing"
		}
		if len(rows) == expected && !pending {
			return rows
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("audio delivery did not finish")
	return nil
}

func TestAudioRecordingSharedByPublicationArchiveAndDelivery(t *testing.T) {
	s, now, policy := audioTestService(t)
	ctx := context.Background()
	e := publishedTestEdition(t, s, *now)
	var calls, sends atomic.Int32
	release := make(chan struct{})
	s.narrate = func(ctx context.Context, got Edition) (AudioFile, error) {
		calls.Add(1)
		select {
		case <-release:
		case <-ctx.Done():
			return AudioFile{}, ctx.Err()
		}
		return AudioFile{MIME: "audio/mpeg", Data: []byte(got.Hash)}, nil
	}
	s.deliverAudio = func(_ context.Context, _ Edition, _ Profile, channel string, both bool, file AudioFile) (string, error) {
		if string(file.Data) != e.Hash || both != (channel == "telegram") {
			t.Error("delivery lost its revision or format")
		}
		sends.Add(1)
		return "ok", nil
	}
	yes := true
	s.afterPublication(ctx, e, DefaultProfile(), Creation{Audio: &yes, Channels: map[string]string{"telegram": "both", "whatsapp": "audio", "email": "off"}})
	a, err := s.GenerateAudio(ctx, e.ID)
	if err != nil || a.Status != "generating" {
		t.Fatalf("in-flight: %+v, %v", a, err)
	}
	close(release)
	receipts := audioReceipts(t, s, e, 2)
	for _, row := range receipts {
		if row.Status != "sent" || row.Format == "edition" || row.Kind != "manual" {
			t.Fatalf("receipt %+v", row)
		}
	}
	if _, err = s.SendFormat(ctx, e.ID, "telegram", "manual", "on-publication", "both"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SendFormat(ctx, e.ID, "telegram", "manual", "on-publication", "audio"); !errors.Is(err, ErrConflict) {
		t.Fatalf("mismatched replay: %v", err)
	}
	if calls.Load() != 1 || sends.Load() != 2 {
		t.Fatalf("renders=%d sends=%d", calls.Load(), sends.Load())
	}
	policy.ReadOnly = true
	if _, err = s.GenerateAudio(ctx, e.ID); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	file, err := s.store.Recording(ctx, e.ID)
	if err != nil || string(file.Data) != e.Hash {
		t.Fatalf("read-only archive: %v", err)
	}
}

func TestAudioBothFallsBackToEditionAndRetriesAudioAlone(t *testing.T) {
	ctx := context.Background()
	s, now, _ := audioTestService(t)
	e := publishedTestEdition(t, s, *now)
	var text, audio, renders atomic.Int32
	s.narrate = func(context.Context, Edition) (AudioFile, error) {
		if renders.Add(1) == 1 {
			return AudioFile{}, errors.New("tts_failed")
		}
		return AudioFile{MIME: "audio/wav", Data: []byte("wave")}, nil
	}
	s.deliver = func(context.Context, Edition, Profile, string) (string, error) { text.Add(1); return "printed", nil }
	s.deliverAudio = func(_ context.Context, _ Edition, _ Profile, _ string, both bool, _ AudioFile) (string, error) {
		if both {
			t.Error("retry would repeat text")
		}
		audio.Add(1)
		return "audio", nil
	}
	first, err := s.SendFormat(ctx, e.ID, "telegram", "manual", "first-with-audio", "both")
	if err != nil || first.Status != "preparing" {
		t.Fatalf("%+v %v", first, err)
	}
	if got := audioReceipts(t, s, e, 1)[0]; got.Status != "audio_pending" || got.ProviderID != "printed" {
		t.Fatalf("%+v", got)
	}
	if _, err = s.SendFormat(ctx, e.ID, "telegram", "manual", "retry-audio-only", "audio"); err != nil {
		t.Fatal(err)
	}
	rows := audioReceipts(t, s, e, 2)
	if got := rows[0]; got.Status != "sent" || got.Format != "audio" || rows[1].Status != "sent" {
		t.Fatalf("%+v", got)
	}
	if text.Load() != 1 || audio.Load() != 1 || renders.Load() != 2 {
		t.Fatalf("text=%d audio=%d renders=%d", text.Load(), audio.Load(), renders.Load())
	}
}

func TestCastScheduleClaimsPlaybackAndHonorsGrace(t *testing.T) {
	s, now, policy := audioTestService(t)
	ctx := context.Background()
	e := publishedTestEdition(t, s, *now)
	p, err := s.Profile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.CastDevice, p.CastTime, p.CastDaily, p.TimeZone = "kitchen", "08:01", true, "UTC"
	if _, err = s.SaveProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err = s.awaitRecording(ctx, e, true); err != nil {
		t.Fatal(err)
	}
	online, ambiguous := false, false
	loads := 0
	s.cast = func(_ context.Context, device string, file AudioFile, before func() error) error {
		if device != "kitchen" || string(file.Data) != "one recording" {
			t.Error("wrong playback target or audio")
		}
		if !online {
			return errors.New("offline")
		}
		if err := before(); err != nil {
			return err
		}
		loads++
		if ambiguous {
			return errors.New("connection lost after load")
		}
		return nil
	}
	schedule := func(key, clock string) CastBooking {
		t.Helper()
		b, err := s.ScheduleCast(ctx, e.ID, e.LocalDate, clock, key)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	manual := schedule("requested-once-0001", "08:02")
	if b := schedule("requested-once-0001", "08:02"); b.Key != manual.Key {
		t.Fatal("lost idempotency")
	}
	state := func(key string) string {
		t.Helper()
		var status string
		if err := s.store.db.QueryRow("SELECT status FROM newspaper_casts WHERE request_key=?", key).Scan(&status); err != nil {
			t.Fatal(err)
		}
		return status
	}
	s.processCasts(p)
	*now = now.Add(3 * time.Minute)
	s.processCasts(p)
	if loads != 0 || state(manual.Key) != "pending" {
		t.Fatal("offline playback was consumed")
	}
	online = true
	s.processCasts(p)
	s.processCasts(p)
	if state("daily:"+e.LocalDate) != "played" || state(manual.Key) != "played" || loads != 2 {
		t.Fatal("daily and one-time playback must each run once")
	}
	ambiguous = true
	b := schedule("ambiguous-once-02", "08:04")
	*now = now.Add(time.Minute)
	s.processCasts(p)
	s.processCasts(p)
	if state(b.Key) != "uncertain" || loads != 3 {
		t.Fatal("replayed an uncertain LOAD")
	}
	offline := schedule("too-late-once-0001", "08:05")
	*now = now.Add(17 * time.Minute)
	s.processCasts(p)
	if state(offline.Key) != "missed" || loads != 3 {
		t.Fatal("expired playback issued a LOAD")
	}
	policy.Cast = false
	if _, err = s.ScheduleCast(ctx, e.ID, "", "", "blocked-now-00001"); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
}

func TestCastDailyAlonePreparesTheEditionAndKeepsThePrintWindow(t *testing.T) {
	for _, tc := range []struct {
		name    string
		clock   int
		daily   bool
		wantRun bool
	}{
		{"cast only, before airtime", 7, false, true},
		{"cast only, grace expired", 9, false, false},
		{"print still due after missed cast", 12, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, now, _ := audioTestService(t)
			*now = time.Date(2026, 9, 25, tc.clock, 0, 0, 0, time.UTC)
			ctx := context.Background()
			p, err := s.Profile(ctx)
			if err != nil {
				t.Fatal(err)
			}
			p.TimeZone, p.ReadyTime, p.Daily = "UTC", "12:05", tc.daily
			p.CastDevice, p.CastTime, p.CastDaily = "speaker", "07:05", true
			if _, err = s.SaveProfile(ctx, p); err != nil {
				t.Fatal(err)
			}
			s.cast = func(context.Context, string, AudioFile, func() error) error { return errors.New("offline") }
			s.research = func(context.Context, Profile, time.Time, func(Progress)) (Draft, error) { return testDraft(*now), nil }
			s.tick()
			s.wg.Wait()
			issues, err := s.List(ctx, 10)
			if err != nil || (len(issues) > 0) != tc.wantRun {
				t.Fatalf("issue count %d, %v", len(issues), err)
			}
			if tc.wantRun {
				ready, err := s.store.AudioState(ctx, issues[0].ID)
				if err != nil || ready.Status != "ready" {
					t.Fatalf("recording: %+v %v", ready, err)
				}
				s.tick()
				s.wg.Wait()
				again, _ := s.List(ctx, 10)
				if len(again) != 1 {
					t.Fatal("duplicate edition")
				}
			}
		})
	}
}

func TestAudioAndCastRecoverWithoutReplayingInterruptedLoads(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "newspaper.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	e := publishedTestEdition(t, &Service{store: store}, time.Now().UTC())
	if _, err = store.db.Exec("INSERT INTO newspaper_audio(edition_id,hash,status) VALUES(?,?,'generating')", e.ID, e.Hash); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"starting", "pending", "played"} {
		if _, err = store.db.Exec("INSERT INTO newspaper_casts(request_key,edition_id,day,device,due,daily,status) VALUES(?,?,?,'receiver',123,0,?)", status, e.ID, e.LocalDate, status); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	state, err := store.AudioState(ctx, e.ID)
	if err != nil || state.Status != "failed" || state.Reason != "interrupted" {
		t.Fatal(state, err)
	}
	bookings, err := store.CastBookings(ctx, e.ID)
	if err != nil || len(bookings) != 3 {
		t.Fatal(bookings, err)
	}
	for _, b := range bookings {
		want := b.Key
		if b.Key == "starting" {
			want = "uncertain"
		}
		if b.Status != want {
			t.Fatalf("%q resumed as %q", b.Key, b.Status)
		}
	}
}

func TestAudioMigrationPreservesTextReceiptsAndPrunesMedia(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st}
	e := publishedTestEdition(t, s, time.Now().UTC())
	if _, _, err = st.ClaimDelivery(ctx, e, "telegram", "manual", strings.Repeat("b", 64), "previous-edition-key"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.Exec("DROP TABLE newspaper_audio; DROP TABLE newspaper_casts; ALTER TABLE newspaper_deliveries DROP COLUMN format; PRAGMA user_version=2"); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	matches, _ := filepath.Glob(path + ".pre-v3-*.db")
	if len(matches) != 1 {
		t.Fatalf("backups: %v", matches)
	}
	deliveries, err := st.Deliveries(ctx, e.ID)
	if err != nil || len(deliveries) != 1 || deliveries[0].Format != "edition" || deliveries[0].Status != "uncertain" {
		t.Fatalf("old receipt: %+v %v", deliveries, err)
	}
	if _, err = st.db.Exec("INSERT INTO newspaper_audio(edition_id,hash,status,data) VALUES(?,?,'ready',?)", e.ID, e.Hash, []byte("test")); err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.Exec("DELETE FROM newspaper_editions WHERE id=?", e.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = st.db.QueryRow("SELECT COUNT(*) FROM newspaper_audio").Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphaned media %d, %v", count, err)
	}
}
