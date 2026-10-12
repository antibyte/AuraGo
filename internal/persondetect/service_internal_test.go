package persondetect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func discardServiceLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// The service clock truncates to milliseconds UTC at the source, so in-memory
// timestamps equal what detections.db returns after a restart, and it carries no
// monotonic reading, so the tracker sees wall-clock steps.
func TestServiceClockIsMillisecondUTC(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 10, 14, 14, 0, 0, 123456789, berlin)
	s := NewService(Deps{Logger: discardServiceLogger(), Now: func() time.Time { return fixed }})
	got := s.now()
	if want := time.Date(2026, 10, 14, 12, 0, 0, 123000000, time.UTC); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("now = %v, want %v", got, want)
	}
	wall := NewService(Deps{Logger: discardServiceLogger()}).now()
	if wall.Location() != time.UTC || wall.Nanosecond()%int(time.Millisecond) != 0 || strings.Contains(wall.String(), "m=") {
		t.Fatalf("default clock = %s, want milliseconds UTC without a monotonic reading", wall)
	}
	var lastSent map[string]LastSent = s.lastSent
	if lastSent == nil {
		t.Fatal("lastSent is nil")
	}
}

func TestServiceStatusTimestampsAreMillisecondUTC(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(Deps{Logger: discardServiceLogger(), Now: func() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) }})
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	cam := normalizeCamera(Camera{ID: "front-door"})
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.settings = normalizeSettings(Settings{Enabled: true, Cameras: []Camera{cam}, Location: time.UTC})
	s.runCtx, s.runCancel = ctx, cancel
	s.cameras[cam.ID] = &cameraRuntime{
		cam: cam, cancel: func() {}, state: CameraActive,
		lastEventID: 7, lastEventAt: time.Date(2026, 10, 14, 13, 30, 0, 987654321, berlin),
	}
	s.arm = armCache{mode: ArmOn, armed: true, changedAt: time.Date(2026, 10, 14, 13, 0, 0, 555555555, berlin), changedBy: ActorUser}
	s.mu.Unlock()

	st := s.Status()
	if len(st.Cameras) != 1 || st.Cameras[0].LastEventAt == nil || st.Cameras[0].LastEventID == nil || *st.Cameras[0].LastEventID != 7 {
		t.Fatalf("cameras = %+v", st.Cameras)
	}
	if got, want := *st.Cameras[0].LastEventAt, time.Date(2026, 10, 14, 11, 30, 0, 987000000, time.UTC); !got.Equal(want) || got.Location() != time.UTC || got.Nanosecond() != want.Nanosecond() {
		t.Fatalf("last_event_at = %v, want %v", got, want)
	}
	if st.Arm.ChangedAt == nil {
		t.Fatal("changed_at missing")
	}
	if got, want := *st.Arm.ChangedAt, time.Date(2026, 10, 14, 11, 0, 0, 555000000, time.UTC); !got.Equal(want) || got.Location() != time.UTC || got.Nanosecond() != want.Nanosecond() {
		t.Fatalf("changed_at = %v, want %v", got, want)
	}
}

func TestCameraStartDelay(t *testing.T) {
	for _, tc := range []struct {
		index, count int
		interval     time.Duration
		want         time.Duration
	}{
		{0, 1, 2 * time.Second, 0},
		{0, 3, 3 * time.Second, 0},
		{1, 3, 3 * time.Second, time.Second},
		{2, 3, 3 * time.Second, 2 * time.Second},
		{3, 4, 30 * time.Second, 22500 * time.Millisecond},
		{1, 0, 2 * time.Second, 0},
		{-1, 2, 2 * time.Second, 0},
		{2, 2, 2 * time.Second, 0},
		{1, 2, 0, 0},
	} {
		if got := cameraStartDelay(tc.index, tc.count, tc.interval); got != tc.want {
			t.Errorf("cameraStartDelay(%d, %d, %v) = %v, want %v", tc.index, tc.count, tc.interval, got, tc.want)
		}
	}
}

// The tracker's single-hit start reads AnalyzedFrame.Score: the best score among
// the boxes that count, not among all boxes.
func TestServiceAnalyzedFrameScoreIsTheBestHit(t *testing.T) {
	cam := normalizeCamera(Camera{ID: "front-door", MinScore: 0.5, Zones: []Zone{{Name: "Left", Points: []Point{{X: 0, Y: 0}, {X: 0.5, Y: 0}, {X: 0.5, Y: 1}, {X: 0, Y: 1}}}}})
	inLow := Box{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.6}
	outside := Box{X1: 0.6, Y1: 0.2, X2: 0.9, Y2: 0.9, Score: 0.95}
	inHigh := Box{X1: 0.2, Y1: 0.2, X2: 0.4, Y2: 0.9, Score: 0.7}
	at := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	f := analyzeFrame(frameJob{cameraID: cam.ID, jpeg: []byte("jpeg"), at: at},
		DetectResult{Boxes: []Box{inLow, outside, inHigh}, Width: 640, Height: 360}, cam)
	if f.Score != 0.7 || len(f.Hits) != 2 || len(f.Boxes) != 3 || len(f.Zones) != 1 || f.Zones[0] != "Left" || !f.At.Equal(at) || f.Width != 640 {
		t.Fatalf("analyzed frame = %+v, want score 0.7 from the two hits", f)
	}
	if none := analyzeFrame(frameJob{at: at}, DetectResult{Boxes: []Box{outside}, Width: 640, Height: 360}, cam); none.Score != 0 || none.IsHit() {
		t.Fatalf("frame without hits = %+v", none)
	}
}

func TestClassifyDetectError(t *testing.T) {
	ctx := context.Background()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cause := errors.New("worker exited: exit status 2")
	for _, tc := range []struct {
		name string
		ctx  context.Context
		err  error
		want detectErrorAction
	}{
		{"restart pending", ctx, ErrWorkerRestarting, detectSkipFrame},
		{"restart after a failure", ctx, fmt.Errorf("%w in 2s: %w", ErrWorkerRestarting, cause), detectSkipFrame},
		{"detector closed", ctx, ErrDetectorClosed, detectSkipFrame},
		{"service stopping", cancelled, errors.New("detect: broken pipe"), detectSkipFrame},
		{"caller gave up", cancelled, context.Canceled, detectSkipFrame},
		{"detector failed", ctx, fmt.Errorf("%w after 5 consecutive failures: %w", ErrDetectorFailed, cause), detectFailDetector},
		{"detector failed with a restart cause", ctx, fmt.Errorf("%w after 5 consecutive failures: %w", ErrDetectorFailed, fmt.Errorf("%w: %w", ErrWorkerRestarting, cause)), detectFailDetector},
		{"bad frame", ctx, errors.New("frame is empty"), detectFailCamera},
		{"frame error from the worker", ctx, errors.New("detect: invalid JPEG"), detectFailCamera},
	} {
		if got := classifyDetectError(tc.ctx, tc.err); got != tc.want {
			t.Errorf("%s: classifyDetectError = %d, want %d", tc.name, got, tc.want)
		}
	}
}
