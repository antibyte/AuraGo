package rtlsdr

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testReceiver struct {
	mu          sync.Mutex
	tunes       []Tuning
	stops       int
	captureGate chan struct{}
	started     chan struct{}
	captureErr  error
	gaps        int
}

func (f *testReceiver) Info(context.Context) (Receiver, error) {
	return Receiver{Ready: true, Minimum: 24000000, Maximum: 1766000000}, nil
}
func (f *testReceiver) Tune(ctx context.Context, t Tuning) error {
	f.mu.Lock()
	f.tunes = append(f.tunes, t)
	f.mu.Unlock()
	return ctx.Err()
}
func (f *testReceiver) Stop(context.Context) error { f.mu.Lock(); f.stops++; f.mu.Unlock(); return nil }
func (f *testReceiver) Stream(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("mp3")), nil
}
func (f *testReceiver) Capture(ctx context.Context, n int, w io.Writer) (CaptureInfo, error) {
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	_, err := w.Write([]byte("fLaC-fixture-audio"))
	if err != nil {
		return CaptureInfo{}, err
	}
	if f.captureGate != nil {
		select {
		case <-f.captureGate:
		case <-ctx.Done():
			return CaptureInfo{Seconds: 2}, ctx.Err()
		}
	}
	return CaptureInfo{Seconds: float64(n), Gaps: f.gaps}, f.captureErr
}
func (f *testReceiver) Scan(ctx context.Context, progress func([]Station, string)) error {
	progress([]Station{{ID: "5A:abcd", Name: "Station", Tuning: Tuning{Mode: "dab", Block: "5A", ServiceID: "abcd"}}}, "5A")
	return ctx.Err()
}
func (f *testReceiver) WAV(context.Context, string, float64, int) ([]byte, error) {
	return []byte("wav"), nil
}
func (f *testReceiver) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.tunes) }

type testASR struct {
	calls atomic.Int32
	gate  chan struct{}
	fail  atomic.Bool
}

func (a *testASR) Transcribe(ctx context.Context, b []byte) (string, error) {
	a.calls.Add(1)
	if a.gate != nil {
		select {
		case <-a.gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if a.fail.Load() {
		return "", errors.New("provider outage")
	}
	return "News at noon.", nil
}
func await(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}
func newTestService(t *testing.T, f *testReceiver, policy func() Policy, mutate func(*Options)) *Service {
	t.Helper()
	o := Options{Directory: t.TempDir(), Backend: f, Policy: policy}
	if mutate != nil {
		mutate(&o)
	}
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func enabled() Policy { return Policy{Enabled: true, AllowAgent: true} }

func TestRTLSDRRecordingReleasesTunerBeforeASR(t *testing.T) {
	f := &testReceiver{captureGate: make(chan struct{}), started: make(chan struct{}, 1)}
	asr := &testASR{gate: make(chan struct{})}
	defer close(asr.gate)
	var factories atomic.Int32
	s := newTestService(t, f, enabled, func(o *Options) {
		o.NewTranscriber = func(context.Context) (Transcriber, error) { factories.Add(1); return asr, nil }
	})
	live := DefaultTuning()
	live.Frequency = 99000000
	if err := s.Tune(context.Background(), "browser", live); err != nil {
		t.Fatal(err)
	}
	recording := DefaultTuning()
	recording.Frequency = 101000000
	r, err := s.Record(Recording{Tuning: recording, Duration: 5, Transcribe: true})
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	if err := s.Tune(context.Background(), "other", live); !errors.Is(err, ErrBusy) {
		t.Fatalf("retune while recording: %v", err)
	}
	if _, err := s.Record(Recording{Tuning: recording, Duration: 5}); !errors.Is(err, ErrBusy) {
		t.Fatalf("duplicate capture: %v", err)
	}
	close(f.captureGate)
	await(t, func() bool { return asr.calls.Load() == 1 && s.Snapshot().Active == "" })
	if factories.Load() != 1 {
		t.Fatal("ASR route was not frozen once")
	}
	f.mu.Lock()
	last := f.tunes[len(f.tunes)-1]
	f.mu.Unlock()
	if last.Frequency != live.Frequency {
		t.Fatal("live station not restored")
	}
	if err := s.Tune(context.Background(), "browser", recording); err != nil {
		t.Fatalf("ASR held receiver: %v", err)
	}
	file, err := s.Audio(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
}
func TestRTLSDRStopListeningDoesNotStopRecording(t *testing.T) {
	f := &testReceiver{captureGate: make(chan struct{}), started: make(chan struct{}, 1)}
	s := newTestService(t, f, enabled, nil)
	if err := s.Tune(context.Background(), "browser", DefaultTuning()); err != nil {
		t.Fatal(err)
	}
	r, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: 5})
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	if err = s.Stop(context.Background(), "browser"); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Active != r.ID {
		t.Fatal("window stop cancelled capture")
	}
	close(f.captureGate)
	await(t, func() bool { return s.Snapshot().Active == "" })
	if f.count() != 2 {
		t.Fatal("stopped listener was restored")
	}
}
func TestRTLSDRQuotaAndPermissionRevocation(t *testing.T) {
	f := &testReceiver{captureGate: make(chan struct{}), started: make(chan struct{}, 1)}
	var restricted atomic.Bool
	s := newTestService(t, f, func() Policy { return Policy{Enabled: true, ReadOnly: restricted.Load(), AllowAgent: true} }, nil)
	r, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: 5})
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	restricted.Store(true)
	s.tick()
	await(t, func() bool { return s.Snapshot().Active == "" })
	result, _ := s.Recording(r.ID)
	if result.Status != "partial" || result.Bytes == 0 {
		t.Fatalf("partial audio not kept: %+v", result)
	}
	if _, err = s.Record(Recording{Tuning: DefaultTuning(), Duration: 5}); !errors.Is(err, ErrReadOnly) {
		t.Fatal(err)
	}
	small := newTestService(t, &testReceiver{}, func() Policy { return Policy{Enabled: true, QuotaBytes: 1024} }, nil)
	if _, err = small.Record(Recording{Tuning: DefaultTuning(), Duration: 5}); !errors.Is(err, ErrQuota) {
		t.Fatalf("quota: %v", err)
	}
	file, err := os.CreateTemp(t.TempDir(), "quota")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := quotaWriter{file: file, limit: 4}
	if _, err = writer.Write([]byte("12345")); !errors.Is(err, ErrQuota) {
		t.Fatal("stream quota failed")
	}
}
func TestRTLSDRPartialAndASRRetry(t *testing.T) {
	f := &testReceiver{captureErr: io.ErrUnexpectedEOF, gaps: 1}
	asr := &testASR{}
	asr.fail.Store(true)
	s := newTestService(t, f, enabled, func(o *Options) { o.NewTranscriber = func(context.Context) (Transcriber, error) { return asr, nil } })
	r, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: 5, Transcribe: true})
	if err != nil {
		t.Fatal(err)
	}
	await(t, func() bool { x, _ := s.Recording(r.ID); return x.ASRError != "" })
	x, _ := s.Recording(r.ID)
	if x.Status != "partial" || x.Gaps != 1 || len(x.Segments) != 1 {
		t.Fatalf("lost failure: %+v", x)
	}
	count := f.count()
	asr.fail.Store(false)
	if err = s.RetryTranscription(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	await(t, func() bool {
		x, _ := s.Recording(r.ID)
		return x.ASRError == "" && x.Status == "partial" && len(x.Segments) == 1 && x.Segments[0].Text != ""
	})
	if f.count() != count {
		t.Fatal("ASR retry retuned receiver")
	}
}
func TestRTLSDRPersistentScheduleClaimAndAgentRevocation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	var clock atomic.Int64
	clock.Store(now.UnixNano())
	var agent atomic.Bool
	agent.Store(true)
	f := &testReceiver{}
	s := newTestService(t, f, func() Policy { return Policy{Enabled: true, AllowAgent: agent.Load()} }, func(o *Options) { o.Now = func() time.Time { return time.Unix(0, clock.Load()) } })
	scheduled, err := s.SaveSchedule(Schedule{Start: now.Add(time.Second), Timezone: "UTC", Repeat: "once", Tuning: DefaultTuning(), Duration: 5, Enabled: true, Agent: true})
	if err != nil {
		t.Fatal(err)
	}
	agent.Store(false)
	clock.Store(now.Add(time.Second).UnixNano())
	s.tick()
	if f.count() != 0 {
		t.Fatal("revoked agent schedule ran")
	}
	agent.Store(true)
	s.tick()
	await(t, func() bool { return len(s.Snapshot().Recordings) == 1 && s.Snapshot().Active == "" })
	s.tick()
	time.Sleep(20 * time.Millisecond)
	if len(s.Snapshot().Recordings) != 1 {
		t.Fatal("schedule executed twice")
	}
	saved := s.Snapshot().Schedules[0]
	if saved.ID != scheduled.ID || !saved.LastSlot.Equal(scheduled.Start) {
		t.Fatal("claim not persisted")
	}
	if _, err := New(s.opts); !errors.Is(err, ErrBusy) {
		t.Fatalf("second service acquired dongle: %v", err)
	}
}
func TestRTLSDRRestartKeepsPartialAudio(t *testing.T) {
	dir := t.TempDir()
	db, state, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	id := newID()
	finalized := newID()
	state.Recordings = []Recording{{ID: id, Status: "recording", Duration: 60}, {ID: finalized, Status: "recording", Duration: 60}}
	raw := &Service{db: db, state: state}
	if err = raw.saveLocked(); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err = os.WriteFile(filepath.Join(dir, id+".flac.part"), []byte("fLaC-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, finalized+".flac"), []byte("fLaC-finalized-before-ledger"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{Directory: dir, Backend: &testReceiver{}, Policy: enabled})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, _ := s.Recording(id)
	if r.Status != "interrupted" || r.Bytes == 0 {
		t.Fatalf("partial lost: %+v", r)
	}
	f, err := s.Audio(id)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	recovered, _ := s.Recording(finalized)
	if recovered.Status != "interrupted" || recovered.Bytes == 0 {
		t.Fatalf("finalized audio lost after failed ledger update: %+v", recovered)
	}
	if _, err = s.Audio("../../config.yaml"); !errors.Is(err, ErrNotFound) {
		t.Fatal("invalid audio ID accepted")
	}
}
