package persondetect_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/onnxrt"
	"aurago/internal/persondetect"
	"aurago/internal/persondetect/persondetecttest"
)

func TestServiceRebuildsDetectorOnlyForModelBackendOrEnabled(t *testing.T) {
	h := newSvcHarness(t, false)
	cam := svcCamera("front-door")
	h.start(cam)
	if h.factory.Calls() != 1 {
		t.Fatalf("setups = %d", h.factory.Calls())
	}
	opts := h.factory.LastOptions()
	if opts.ModelDir != filepath.Join(h.dataDir, "persondetect", "models") || opts.RuntimeCacheDir != filepath.Join(h.dataDir, "embeddings") ||
		opts.Model != persondetect.ModelStandard || opts.Backend != persondetect.BackendAuto || opts.OnProgress == nil {
		t.Fatalf("detector options = %+v", opts)
	}
	tuned := cam
	tuned.Interval, tuned.MinScore = 5*time.Second, 0.7
	settings := h.settings(tuned)
	settings.RetentionDays = 3
	settings.Schedule = []persondetect.Window{{Days: []time.Weekday{time.Monday}, From: 0, To: 60}}
	h.svc.Configure(settings)
	if h.factory.Calls() != 1 {
		t.Fatalf("a camera, retention or schedule change rebuilt the detector (%d setups)", h.factory.Calls())
	}
	settings.Backend = persondetect.BackendCPU
	h.svc.Configure(settings)
	persondetecttest.WaitFor(t, 5*time.Second, "second detector setup", func() bool { return h.factory.Calls() == 2 })
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	created := h.factory.Created()
	persondetecttest.WaitFor(t, 5*time.Second, "replaced detector closed", func() bool { return created[0].Closed() })

	disabled := settings
	disabled.Enabled = false
	h.svc.Configure(disabled)
	if st := h.svc.Status(); st.Enabled || st.Detector.State != persondetect.DetectorOff || len(st.Cameras) != 0 {
		t.Fatalf("disabled status = %+v", st)
	}
	persondetecttest.WaitFor(t, 5*time.Second, "detector closed on disable", func() bool { return h.factory.Created()[1].Closed() })
	h.svc.Configure(settings)
	persondetecttest.WaitFor(t, 5*time.Second, "third detector setup", func() bool { return h.factory.Calls() == 3 })
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
}

func TestServiceSetupPhasesErrorAndRetry(t *testing.T) {
	h := newSvcHarness(t, false)
	gate := make(chan struct{})
	h.factory.SetPhases(persondetect.SetupProgress{Phase: "downloading_model", Download: onnxrt.DownloadStatus{Asset: "dfine-n", Downloaded: 5, Total: 10, Percent: 50}})
	h.factory.SetGate(gate)
	h.svc.Configure(h.settings(svcCamera("front-door")))
	persondetecttest.WaitFor(t, 5*time.Second, "download progress", func() bool {
		d := h.svc.Status().Detector
		return d.State == persondetect.DetectorDownloading && d.Phase == "downloading_model" && d.Download.Percent == 50
	})
	if d := h.svc.Status().Detector; d.Model != persondetect.ModelStandard || d.ModelName != "D-FINE-N" || d.RequestedBackend != persondetect.BackendAuto {
		t.Fatalf("setup status = %+v", d)
	}
	if delay := h.svc.FetchOnceForTest(context.Background(), "front-door"); delay != 2*time.Second || h.frames.Calls("front-door") != 0 {
		t.Fatalf("a camera requested frames before the detector was ready (delay %v, calls %d)", delay, h.frames.Calls("front-door"))
	}
	if st := h.svc.Status().Cameras[0]; st.State != persondetect.CameraWaiting {
		t.Fatalf("camera state = %q, want waiting", st.State)
	}
	h.factory.SetError(errors.New("sha256 mismatch for dfine-n"))
	close(gate)
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorError)
	if d := h.svc.Status().Detector; !strings.Contains(d.Error, "sha256 mismatch") {
		t.Fatalf("error = %q", d.Error)
	}
	h.factory.SetError(nil)
	h.factory.SetGate(nil)
	h.factory.SetPhases()
	if err := h.svc.Retry(); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	if h.factory.Calls() != 2 {
		t.Fatalf("setups after retry = %d", h.factory.Calls())
	}
}

func TestServiceDegradedWhenGPURequestedButRunningOnCPU(t *testing.T) {
	h := newSvcHarness(t, false)
	info := persondetecttest.CPUInfo()
	info.Requested = persondetect.BackendGPU
	info.FallbackReason = "GPU not accessible: /dev/dri/renderD128 permission denied"
	h.factory.SetInfo(info)
	settings := h.settings(svcCamera("front-door"))
	settings.Backend = persondetect.BackendGPU
	h.svc.Configure(settings)
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorDegraded)
	d := h.svc.Status().Detector
	if d.Backend != persondetect.BackendCPU || d.RequestedBackend != persondetect.BackendGPU || d.FallbackReason == "" ||
		d.SelfTest == nil || !d.SelfTest.Passed || d.Runtime == "" || len(d.Providers) != 1 {
		t.Fatalf("degraded status = %+v", d)
	}
}

func TestServiceFrameFetchBackoffAndCameraStates(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	h.frames.Fail("front-door", errors.New("go2rtc: stream offline"))
	for i, want := range []time.Duration{4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, 60 * time.Second, 60 * time.Second} {
		if got := h.svc.FetchOnceForTest(ctx, "front-door"); got != want {
			t.Fatalf("failure %d: delay %v, want %v", i+1, got, want)
		}
	}
	cam := h.svc.Status().Cameras[0]
	if cam.State != persondetect.CameraUnreachable || !strings.Contains(cam.LastError, "stream offline") {
		t.Fatalf("camera = %+v", cam)
	}
	h.frames.Fail("front-door", nil)
	h.frames.Set("front-door", persondetecttest.JPEG(64, 36))
	if got := h.svc.FetchOnceForTest(ctx, "front-door"); got != 2*time.Second {
		t.Fatalf("delay after recovery = %v", got)
	}
	if cam := h.svc.Status().Cameras[0]; cam.State != persondetect.CameraWaiting || cam.LastError != "" {
		t.Fatalf("recovered camera = %+v", cam)
	}
	if h.svc.QueuedFramesForTest() != 1 {
		t.Fatalf("queued frames = %d", h.svc.QueuedFramesForTest())
	}
}

// A stopped go2rtc sidecar makes every frame request fail. The cameras then show
// unreachable and back off, while the detector stays ready: that is the expected
// state until go2rtc runs again, not a detector fault.
func TestServiceStoppedGo2RTCLeavesCamerasUnreachable(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"), svcCamera("garage"))
	ctx := context.Background()
	stopped := fmt.Errorf("%w: go2rtc is not running", persondetect.ErrFrameUnavailable)
	for _, id := range []string{"front-door", "garage"} {
		h.frames.Fail(id, stopped)
		if got := h.svc.FetchOnceForTest(ctx, id); got != 4*time.Second {
			t.Fatalf("%s: first delay %v, want 4s", id, got)
		}
		if got := h.svc.FetchOnceForTest(ctx, id); got != 8*time.Second {
			t.Fatalf("%s: second delay %v, want 8s", id, got)
		}
	}
	st := h.svc.Status()
	for _, cam := range st.Cameras {
		if cam.State != persondetect.CameraUnreachable || !strings.Contains(cam.LastError, "go2rtc is not running") {
			t.Fatalf("camera = %+v", cam)
		}
	}
	if st.Detector.State != persondetect.DetectorReady || h.svc.QueuedFramesForTest() != 0 {
		t.Fatalf("detector = %q, queued = %d", st.Detector.State, h.svc.QueuedFramesForTest())
	}
}

func TestServiceStatusListsCamerasInOrderWithStorageLimit(t *testing.T) {
	h := newSvcHarness(t, false)
	settings := h.settings(svcCamera("garage"), svcCamera("front-door"))
	settings.MaxStorageMB = 100
	h.svc.Configure(settings)
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	st := h.svc.Status()
	if !st.Enabled || len(st.Cameras) != 2 || st.Cameras[0].ID != "garage" || st.Cameras[1].Name != "Front-door" {
		t.Fatalf("cameras = %+v", st.Cameras)
	}
	if st.Storage.LimitBytes != 100<<20 || st.Storage.Bytes != 0 || st.Storage.Warning != "" {
		t.Fatalf("storage = %+v", st.Storage)
	}
	if st.Arm.Mode != persondetect.ArmOff || st.Arm.Armed || st.Arm.NextChange != nil || st.Arm.ChangedAt != nil {
		t.Fatalf("fresh arm state = %+v", st.Arm)
	}
	settings.Cameras = settings.Cameras[:1]
	h.svc.Configure(settings)
	if st := h.svc.Status(); len(st.Cameras) != 1 || st.Cameras[0].ID != "garage" {
		t.Fatalf("cameras after removal = %+v", st.Cameras)
	}
}

func TestServiceOpensStorageOnlyWhenEnabledOrPresent(t *testing.T) {
	dir := t.TempDir()
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	svc := persondetect.NewManualServiceForTest(persondetect.Deps{Logger: svcDiscardLogger(), NewDetector: factory.New})
	persondetecttest.StopOnCleanup(t, svc)
	svc.Configure(persondetect.Settings{DataDir: dir})
	if !svc.WaitBackgroundForTest(5 * time.Second) {
		t.Fatal("background work of a disabled service did not finish")
	}
	dbPath, _ := persondetect.DetectionsPaths(dir)
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("a disabled service created %s (%v)", dbPath, err)
	}
	if err := svc.StoreErrForTest(); !errors.Is(err, persondetect.ErrDisabled) {
		t.Fatalf("store of a disabled service without a database: %v", err)
	}
	svc.Configure(persondetect.Settings{Enabled: true, DataDir: dir})
	persondetecttest.WaitDetector(t, svc, persondetect.DetectorReady)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("enabled service has no database: %v", err)
	}
	if err := svc.StoreErrForTest(); err != nil {
		t.Fatalf("store of an enabled service: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := svc.StoreErrForTest(); !errors.Is(err, persondetect.ErrDisabled) {
		t.Fatalf("store after Shutdown: %v", err)
	}

	// A disabled service opens an existing database, so stored events stay readable.
	reopened := persondetect.NewManualServiceForTest(persondetect.Deps{Logger: svcDiscardLogger(), NewDetector: factory.New})
	persondetecttest.StopOnCleanup(t, reopened)
	reopened.Configure(persondetect.Settings{DataDir: dir})
	persondetecttest.WaitFor(t, 5*time.Second, "existing database opened", func() bool { return reopened.StoreErrForTest() == nil })
	if st := reopened.Status(); st.Enabled || st.Detector.State != persondetect.DetectorOff || factory.Calls() != 1 {
		t.Fatalf("disabled service with a database: status %+v, setups %d", st, factory.Calls())
	}

	noDir := persondetect.NewManualServiceForTest(persondetect.Deps{Logger: svcDiscardLogger(), NewDetector: factory.New})
	persondetecttest.StopOnCleanup(t, noDir)
	noDir.Configure(persondetect.Settings{Enabled: true})
	persondetecttest.WaitDetector(t, noDir, persondetect.DetectorError)
	if d := noDir.Status().Detector; !strings.Contains(d.Error, "storage") {
		t.Fatalf("missing data directory = %+v", d)
	}
	if err := noDir.StoreErrForTest(); !errors.Is(err, persondetect.ErrUnavailable) {
		t.Fatalf("store without a data directory: %v", err)
	}
}

// OpenEventStore runs an integrity check that takes seconds on a large database
// or a slow disk. Configure runs under the server's config lock, so the database is
// opened in the background, ahead of the detector setup; Configure and Status
// never wait for it, and one open serves every setup that needs the database.
func TestServiceOpensTheStoreOutsideConfigure(t *testing.T) {
	h := newSvcHarness(t, false)
	gate := make(chan struct{})
	var opens atomic.Int32
	h.svc.SetStoreOpenerForTest(func(dataDir string, logger *slog.Logger) (*persondetect.EventStore, error) {
		opens.Add(1)
		<-gate
		return persondetect.OpenEventStore(dataDir, logger)
	})
	var released atomic.Bool
	release := func() {
		if released.CompareAndSwap(false, true) {
			close(gate)
		}
	}
	t.Cleanup(release) // runs before the harness's Shutdown

	configure := func(settings persondetect.Settings) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			h.svc.Configure(settings)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Configure waited for the database to open")
		}
	}
	configure(h.settings(svcCamera("front-door")))
	persondetecttest.WaitFor(t, 5*time.Second, "database open started", func() bool { return opens.Load() == 1 })
	st := h.svc.Status()
	if !st.Enabled || st.Detector.State != persondetect.DetectorPreparing || len(st.Cameras) != 1 || st.Cameras[0].State != persondetect.CameraWaiting {
		t.Fatalf("status while the database opens = %+v", st)
	}
	if err := h.svc.StoreErrForTest(); !errors.Is(err, persondetect.ErrUnavailable) {
		t.Fatalf("store while it opens: %v", err)
	}
	if delay := h.svc.FetchOnceForTest(context.Background(), "front-door"); delay != 2*time.Second || h.frames.Calls("front-door") != 0 {
		t.Fatalf("a camera requested frames before the database was open (delay %v, calls %d)", delay, h.frames.Calls("front-door"))
	}
	if h.factory.Calls() != 0 {
		t.Fatalf("the detector was set up before the database was open (%d setups)", h.factory.Calls())
	}

	rebuilt := h.settings(svcCamera("front-door"))
	rebuilt.Backend = persondetect.BackendCPU
	configure(rebuilt)
	release()
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	if opens.Load() != 1 {
		t.Fatalf("database opened %d times", opens.Load())
	}
	if h.factory.Calls() != 1 || h.factory.LastOptions().Backend != persondetect.BackendCPU {
		t.Fatalf("setups = %d, last backend %q", h.factory.Calls(), h.factory.LastOptions().Backend)
	}
	if err := h.svc.StoreErrForTest(); err != nil {
		t.Fatalf("store after the open: %v", err)
	}
}

// An enabled service whose detections.db cannot be opened is unavailable, not
// switched off; Configure and Retry try the open again.
func TestServiceUnavailableWhileTheDatabaseCannotBeOpened(t *testing.T) {
	h := newSvcHarness(t, false)
	var broken atomic.Bool
	broken.Store(true)
	var opens atomic.Int32
	h.svc.SetStoreOpenerForTest(func(dataDir string, logger *slog.Logger) (*persondetect.EventStore, error) {
		opens.Add(1)
		if broken.Load() {
			return nil, errors.New("database disk image is malformed")
		}
		return persondetect.OpenEventStore(dataDir, logger)
	})
	settings := h.settings(svcCamera("front-door"))
	h.svc.Configure(settings)
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorError)
	if d := h.svc.Status().Detector; !strings.Contains(d.Error, "storage") || !strings.Contains(d.Error, "malformed") {
		t.Fatalf("detector = %+v", d)
	}
	err := h.svc.StoreErrForTest()
	if !errors.Is(err, persondetect.ErrUnavailable) || errors.Is(err, persondetect.ErrDisabled) || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("store error = %v", err)
	}
	if h.factory.Calls() != 0 {
		t.Fatalf("setups without a database = %d", h.factory.Calls())
	}

	h.svc.Configure(settings)
	persondetecttest.WaitFor(t, 5*time.Second, "second open attempt", func() bool { return opens.Load() == 2 })
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorError)

	broken.Store(false)
	if err := h.svc.Retry(); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	if opens.Load() != 3 || h.factory.Calls() != 1 {
		t.Fatalf("opens = %d, setups = %d", opens.Load(), h.factory.Calls())
	}
	if err := h.svc.StoreErrForTest(); err != nil {
		t.Fatalf("store after Retry: %v", err)
	}
}

// Worker restarts, a replaced detector and a stopping service are service state:
// they drop the frame and leave the camera alone. Other Detect errors concern the
// frame and mark the camera; ErrDetectorFailed takes the detector out of service.
func TestServiceWorkerErrorsDoNotMarkTheCamera(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"), svcCamera("garage"))
	h.svc.SetDetectorRetryForTest(time.Hour, time.Hour)
	ctx := context.Background()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	camera := func(id string) persondetect.CameraStatus {
		t.Helper()
		for _, cam := range h.svc.Status().Cameras {
			if cam.ID == id {
				return cam
			}
		}
		t.Fatalf("camera %s missing", id)
		return persondetect.CameraStatus{}
	}
	for _, tc := range []struct {
		ctx context.Context
		err error
	}{
		{ctx, persondetect.ErrWorkerRestarting},
		{ctx, fmt.Errorf("%w in 2s: %w", persondetect.ErrWorkerRestarting, errors.New("worker exited: exit status 2"))},
		{ctx, persondetect.ErrDetectorClosed},
		{cancelled, errors.New("detect: write request: file already closed")},
	} {
		h.svc.HandleDetectErrorForTest(tc.ctx, "front-door", tc.err)
		if cam := camera("front-door"); cam.State != persondetect.CameraWaiting || cam.LastError != "" {
			t.Fatalf("%v marked the camera: %+v", tc.err, cam)
		}
		if d := h.svc.Status().Detector; d.State != persondetect.DetectorReady {
			t.Fatalf("%v changed the detector: %+v", tc.err, d)
		}
	}

	h.svc.HandleDetectErrorForTest(ctx, "front-door", errors.New("detect: frame is not a JPEG"))
	if cam := camera("front-door"); cam.State != persondetect.CameraError || !strings.Contains(cam.LastError, "not a JPEG") {
		t.Fatalf("frame error: camera = %+v", cam)
	}
	if d := h.svc.Status().Detector; d.State != persondetect.DetectorReady {
		t.Fatalf("a frame error changed the detector: %+v", d)
	}

	failed := fmt.Errorf("%w after 5 consecutive failures: %w", persondetect.ErrDetectorFailed,
		fmt.Errorf("%w: worker exited", persondetect.ErrWorkerRestarting))
	h.svc.HandleDetectErrorForTest(ctx, "garage", failed)
	if d := h.svc.Status().Detector; d.State != persondetect.DetectorError || !strings.Contains(d.Error, "consecutive failures") {
		t.Fatalf("detector after ErrDetectorFailed = %+v", d)
	}
	if cam := camera("garage"); cam.State != persondetect.CameraWaiting || cam.LastError != "" {
		t.Fatalf("ErrDetectorFailed marked the camera: %+v", cam)
	}
	created := h.factory.Created()
	persondetecttest.WaitFor(t, 5*time.Second, "failed detector closed", func() bool { return created[0].Closed() })
}

// After ErrDetectorFailed the service runs NewDetector again on its own, with a
// growing delay; Retry, a settings change or a later setup supersede a scheduled one.
func TestServiceSetsTheDetectorUpAgainAfterItFailed(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	failed := fmt.Errorf("%w after 5 consecutive failures: %w", persondetect.ErrDetectorFailed, errors.New("worker exited: signal: killed"))
	setups := func(n int) {
		t.Helper()
		persondetecttest.WaitFor(t, 5*time.Second, fmt.Sprintf("%d detector setups", n), func() bool { return h.factory.Calls() == n })
		persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	}

	h.svc.SetDetectorRetryForTest(time.Hour, time.Hour)
	if delay, ok := h.svc.FailDetectorForTest(failed); !ok || delay != time.Hour {
		t.Fatalf("scheduled setup in %v (%v), want 1h", delay, ok)
	}
	if d := h.svc.Status().Detector; d.State != persondetect.DetectorError || !strings.Contains(d.Error, "consecutive failures") {
		t.Fatalf("detector = %+v", d)
	}
	h.frames.Set("front-door", persondetecttest.JPEG(64, 36))
	if h.svc.FetchOnceForTest(context.Background(), "front-door"); h.frames.Calls("front-door") != 0 {
		t.Fatal("a camera requested frames while the detector was failed")
	}
	if err := h.svc.Retry(); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	setups(2)

	// A settings change supersedes a scheduled setup.
	h.svc.SetDetectorRetryForTest(300*time.Millisecond, 10*time.Second)
	if delay, ok := h.svc.FailDetectorForTest(failed); !ok || delay != 300*time.Millisecond {
		t.Fatalf("scheduled setup in %v (%v), want 300ms", delay, ok)
	}
	settings := h.settings(svcCamera("front-door"))
	settings.Backend = persondetect.BackendCPU
	h.svc.Configure(settings)
	setups(3)
	time.Sleep(600 * time.Millisecond)
	if h.factory.Calls() != 3 {
		t.Fatalf("a superseded automatic setup ran (%d setups)", h.factory.Calls())
	}

	// Automatic setups back off and continue after a failing setup.
	h.svc.SetDetectorRetryForTest(20*time.Millisecond, 10*time.Second)
	if delay, ok := h.svc.FailDetectorForTest(failed); !ok || delay != 20*time.Millisecond {
		t.Fatalf("first automatic setup in %v (%v), want 20ms", delay, ok)
	}
	setups(4)
	if opts := h.factory.LastOptions(); opts.Backend != persondetect.BackendCPU || opts.Model != persondetect.ModelStandard {
		t.Fatalf("automatic setup options = %+v", opts)
	}
	h.factory.SetError(errors.New("worker did not start"))
	if delay, ok := h.svc.FailDetectorForTest(failed); !ok || delay != 40*time.Millisecond {
		t.Fatalf("second automatic setup in %v (%v), want 40ms", delay, ok)
	}
	persondetecttest.WaitFor(t, 5*time.Second, "failing automatic setup", func() bool {
		return h.factory.Calls() >= 5 && strings.Contains(h.svc.Status().Detector.Error, "did not start")
	})
	h.factory.SetError(nil)
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	if h.factory.Calls() < 6 {
		t.Fatalf("setups = %d, want a further automatic setup after the failed one", h.factory.Calls())
	}
	if _, ok := h.svc.FailDetectorForTest(failed); !ok {
		t.Fatal("no detector to fail")
	}
	if _, ok := h.svc.FailDetectorForTest(failed); ok {
		t.Fatal("a detector that is already out of service failed again")
	}
}

func TestServiceShutdownIsIdempotentAndDefaultIsPublished(t *testing.T) {
	svc := persondetect.NewService(persondetect.Deps{Logger: svcDiscardLogger()})
	persondetecttest.StopOnCleanup(t, svc)
	t.Cleanup(func() { persondetect.SetDefault(nil) })
	persondetect.SetDefault(svc)
	if persondetect.Default() != svc {
		t.Fatal("Default did not return the published service")
	}
	persondetect.SetDefault(nil)
	if persondetect.Default() != nil {
		t.Fatal("Default kept a withdrawn service")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
	svc.Configure(persondetect.Settings{Enabled: true, DataDir: t.TempDir()})
	if svc.Status().Enabled {
		t.Fatal("a shut-down service accepted new settings")
	}
	if err := svc.Retry(); !errors.Is(err, persondetect.ErrDisabled) {
		t.Fatalf("Retry after Shutdown = %v", err)
	}
}

func TestServiceRealCameraLoopsFetchFrames(t *testing.T) {
	dir := t.TempDir()
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	frames := persondetecttest.NewFrames()
	frames.Set("front-door", persondetecttest.JPEG(64, 36))
	svc := persondetect.NewService(persondetect.Deps{Logger: svcDiscardLogger(), Frames: frames, NewDetector: factory.New})
	persondetecttest.StopOnCleanup(t, svc)
	cam := svcCamera("front-door")
	cam.Interval = 20 * time.Millisecond
	svc.Configure(persondetect.Settings{Enabled: true, DataDir: dir, Cameras: []persondetect.Camera{cam}, Location: time.UTC})
	persondetecttest.WaitFor(t, 5*time.Second, "three frame requests", func() bool { return frames.Calls("front-door") >= 3 })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	calls := frames.Calls("front-door")
	time.Sleep(60 * time.Millisecond)
	if frames.Calls("front-door") != calls {
		t.Fatal("a camera loop kept running after Shutdown")
	}
}

// Camera loops start staggered: camera i of n waits i × interval / n before its
// first frame request, so cameras with the same interval do not hit go2rtc and the
// detector at the same moment.
func TestServiceStaggersCameraStarts(t *testing.T) {
	dir := t.TempDir()
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	frames := persondetecttest.NewFrames()
	svc := persondetect.NewService(persondetect.Deps{Logger: svcDiscardLogger(), Frames: frames, NewDetector: factory.New})
	persondetecttest.StopOnCleanup(t, svc)
	settings := persondetect.Settings{Enabled: true, DataDir: dir, Location: time.UTC}
	svc.Configure(settings)
	persondetecttest.WaitDetector(t, svc, persondetect.DetectorReady)

	const interval = 400 * time.Millisecond
	for _, id := range []string{"a", "b"} {
		frames.Set(id, persondetecttest.JPEG(32, 18))
		cam := svcCamera(id)
		cam.Interval = interval
		settings.Cameras = append(settings.Cameras, cam)
	}
	before := time.Now()
	svc.Configure(settings)
	persondetecttest.WaitFor(t, 5*time.Second, "first frame of both cameras", func() bool {
		return frames.Calls("a") > 0 && frames.Calls("b") > 0
	})
	firstA, firstB := frames.CallTimes("a")[0], frames.CallTimes("b")[0]
	if got := firstB.Sub(before); got < interval/2 {
		t.Fatalf("camera b (index 1 of 2) first requested a frame after %v, want at least %v", got, interval/2)
	}
	if !firstA.Before(firstB) {
		t.Fatalf("camera a (no delay) first requested at %v, after camera b at %v", firstA, firstB)
	}
}
