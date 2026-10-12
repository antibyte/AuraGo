package persondetect_test

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/persondetect"
	"aurago/internal/persondetect/persondetecttest"
)

// svcStart is Wednesday 2026-10-14 12:00 UTC.
var svcStart = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func svcDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type svcHarness struct {
	t       *testing.T
	svc     *persondetect.Service
	factory *persondetecttest.Factory
	frames  *persondetecttest.Frames
	clock   *persondetecttest.Clock
	alerts  *persondetecttest.Dispatcher
	dataDir string
}

// newSvcHarness creates a manual service (no background loops) with a fake clock,
// detector factory and frame source; withAlerts adds a recording dispatcher. The
// service is shut down when the test ends.
func newSvcHarness(t *testing.T, withAlerts bool) *svcHarness {
	t.Helper()
	h := &svcHarness{
		t:       t,
		factory: persondetecttest.NewFactory(persondetecttest.CPUInfo()),
		frames:  persondetecttest.NewFrames(),
		clock:   persondetecttest.NewClock(svcStart),
		dataDir: t.TempDir(),
	}
	deps := persondetect.Deps{Logger: svcDiscardLogger(), Frames: h.frames, NewDetector: h.factory.New, Now: h.clock.Now}
	if withAlerts {
		h.alerts = persondetecttest.NewDispatcher()
		deps.Alerts = h.alerts
	}
	h.svc = persondetect.NewManualServiceForTest(deps)
	persondetecttest.StopOnCleanup(t, h.svc)
	return h
}

func (h *svcHarness) settings(cams ...persondetect.Camera) persondetect.Settings {
	return persondetect.Settings{
		Enabled: true, DataDir: h.dataDir, Model: persondetect.ModelStandard, Backend: persondetect.BackendAuto,
		RetentionDays: 14, MaxStorageMB: 2048, Cameras: cams, Location: time.UTC, Language: "de",
	}
}

func (h *svcHarness) start(cams ...persondetect.Camera) {
	h.t.Helper()
	h.svc.Configure(h.settings(cams...))
	persondetecttest.WaitDetector(h.t, h.svc, persondetect.DetectorReady)
}

func svcCamera(id string) persondetect.Camera {
	return persondetect.Camera{
		ID: id, Name: strings.ToUpper(id[:1]) + id[1:], Interval: 2 * time.Second, MinScore: 0.5, Cooldown: 5 * time.Minute,
		Actions: persondetect.Actions{Message: true, Channel: "all", Desktop: true},
	}
}
