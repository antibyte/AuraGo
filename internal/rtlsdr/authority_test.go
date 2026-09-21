package rtlsdr

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRTLSDRNewScheduleCannotInterruptCurrentRecording(t *testing.T) {
	f := &testReceiver{captureGate: make(chan struct{}), started: make(chan struct{}, 1)}
	s := newTestService(t, f, enabled, nil)
	_, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: 600})
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	_, err = s.SaveSchedule(Schedule{Tuning: DefaultTuning(), Start: time.Now().Add(time.Minute), Repeat: "once", Enabled: true, Duration: 60})
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("overlapping active recording accepted: %v", err)
	}
}

func TestRTLSDRAgentRetryStopsWhenGrantIsRevoked(t *testing.T) {
	var revoked atomic.Bool
	asr := &testASR{gate: make(chan struct{})}
	s := newTestService(t, &testReceiver{}, func() Policy { p := enabled(); p.AllowAgent = !revoked.Load(); return p }, func(o *Options) { o.NewTranscriber = func(context.Context) (Transcriber, error) { return asr, nil } })
	r, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: 5})
	if err != nil {
		t.Fatal(err)
	}
	await(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.finishing[r.ID] })
	if err = s.RetryTranscriptionAs(context.Background(), r.ID, true); err != nil {
		t.Fatal(err)
	}
	await(t, func() bool { return asr.calls.Load() > 0 })
	revoked.Store(true)
	s.tick()
	await(t, func() bool { v, _ := s.Recording(r.ID); return v.ASRError != "" })
	if err = s.RetryTranscriptionAs(context.Background(), r.ID, true); !errors.Is(err, ErrDisabled) {
		t.Fatalf("revoked retry: %v", err)
	}
}

func TestRTLSDRTuneRechecksAfterWaitingForDevice(t *testing.T) {
	var revoked atomic.Bool
	f := &testReceiver{}
	s := newTestService(t, f, func() Policy { p := enabled(); p.ReadOnly = revoked.Load(); return p }, nil)
	s.deviceMu.Lock()
	done := make(chan error, 1)
	go func() { done <- s.Tune(context.Background(), "browser", DefaultTuning()) }()
	revoked.Store(true)
	s.deviceMu.Unlock()
	if err := <-done; !errors.Is(err, ErrReadOnly) {
		t.Fatalf("revoked tune: %v", err)
	}
	if f.count() != 0 {
		t.Fatal("receiver changed after revocation")
	}
}

func TestRTLSDRSnapshotDoesNotExposeTranscripts(t *testing.T) {
	s := newTestService(t, &testReceiver{}, enabled, nil)
	s.mu.Lock()
	s.state.Recordings = append(s.state.Recordings, Recording{ID: "a", Segments: []Segment{{Text: "external text"}}})
	s.mu.Unlock()
	if len(s.Snapshot().Recordings[0].Segments) != 0 {
		t.Fatal("poll response contains transcript")
	}
	r, _ := s.Recording("a")
	if len(r.Segments) != 1 {
		t.Fatal("snapshot mutated durable result")
	}
}
