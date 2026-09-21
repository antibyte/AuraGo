package rtlsdr

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRTLSDRDurationBoundsCannotOverflow(t *testing.T) {
	s := newTestService(t, &testReceiver{}, enabled, nil)
	for _, duration := range []int{7201, int(^uint(0) >> 1)} {
		if _, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: duration}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("record accepted duration %d: %v", duration, err)
		}
		if _, err := s.SaveSchedule(Schedule{Tuning: DefaultTuning(), Duration: duration, Start: time.Now().Add(time.Hour), Timezone: "UTC", Repeat: "once", Enabled: true}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("schedule accepted duration %d: %v", duration, err)
		}
	}
}

func TestRTLSDRAnnouncesBeforeTakingReceiver(t *testing.T) {
	base := time.Date(2030, 1, 1, 11, 59, 0, 0, time.UTC)
	var now atomic.Int64
	now.Store(base.Unix())
	var notices atomic.Int32
	f := &testReceiver{}
	s := newTestService(t, f, enabled, func(o *Options) {
		o.Now = func() time.Time { return time.Unix(now.Load(), 0) }
		o.NotifyUpcoming = func() { notices.Add(1) }
	})
	_, err := s.SaveSchedule(Schedule{Name: "Noon news", Tuning: DefaultTuning(), Start: base.Add(time.Minute), Repeat: "daily", Timezone: "UTC", Duration: 600, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	s.tick()
	s.tick()
	if notices.Load() != 1 || f.count() != 0 {
		t.Fatalf("warning must precede receiver claim: notices=%d tunes=%d", notices.Load(), f.count())
	}
	now.Store(base.Add(31 * time.Second).Unix())
	s.tick()
	await(t, func() bool { return f.count() == 1 })
	if notices.Load() != 1 {
		t.Fatal("warmup repeated the announcement")
	}
}

type unplugReceiver struct {
	*testReceiver
	connected atomic.Bool
}

func (f *unplugReceiver) DeviceConnected() bool { return f.connected.Load() }

func TestRTLSDRUSBRemovalFinalizesPartialAudio(t *testing.T) {
	f := &testReceiver{captureGate: make(chan struct{}), started: make(chan struct{}, 1)}
	backend := &unplugReceiver{testReceiver: f}
	backend.connected.Store(true)
	s := newTestService(t, f, enabled, func(o *Options) { o.Backend = backend })
	r, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: 600})
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	backend.connected.Store(false)
	s.tick()
	await(t, func() bool { r, _ = s.Recording(r.ID); return r.Status == "partial" })
	if r.Error != "sdr_device_lost" || r.Bytes == 0 || r.Gaps != 1 {
		t.Fatalf("lost device: %+v", r)
	}
	file, err := s.Audio(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if s.Snapshot().Active != "" {
		t.Fatal("lost receiver remained reserved")
	}
}

func TestRTLSDRExplicitStopDoesNotReportAnOutage(t *testing.T) {
	f := &testReceiver{captureGate: make(chan struct{}), started: make(chan struct{}, 1)}
	var issues atomic.Int32
	s := newTestService(t, f, enabled, func(o *Options) {
		o.Issue = func(string, bool) { issues.Add(1) }
	})
	r, err := s.Record(Recording{Tuning: DefaultTuning(), Duration: 600})
	if err != nil {
		t.Fatal(err)
	}
	<-f.started
	if err = s.StopRecording(r.ID); err != nil {
		t.Fatal(err)
	}
	await(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.finishing[r.ID] })
	r, _ = s.Recording(r.ID)
	if r.Status != "partial" || r.Error != "sdr_stopped" || issues.Load() != 0 {
		t.Fatalf("intentional stop must retain audio without an outage: %+v, notices=%d", r, issues.Load())
	}
}

func TestRTLSDRContainerPathsStayInsideOwnedMount(t *testing.T) {
	mounts := []runtimeMount{
		{Type: "volume", Source: "/var/lib/docker/volumes/data/_data", Destination: "/app/data", RW: true},
		{Type: "bind", Source: "/radio", Destination: "/app/data/rtl-sdr", RW: true},
		{Type: "bind", Source: "/readonly", Destination: "/settings", RW: false},
	}
	for _, test := range []struct{ input, want string }{
		{"/app/data/rtl-sdr/run", "/radio/run"},
		{"/app/data/other", "/var/lib/docker/volumes/data/_data/other"},
		{"/app/database/rtl-sdr", ""},
		{"/app/data/../../etc", ""},
		{"/settings/rtl-sdr", ""},
	} {
		got, err := runtimeHostPath(test.input, mounts)
		if got != test.want || (err != nil) != (test.want == "") {
			t.Errorf("%s: %s / %v", test.input, got, err)
		}
	}
}
