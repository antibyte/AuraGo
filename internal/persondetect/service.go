package persondetect

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/onnxrt"
)

// Deps are the service's collaborators; nil fields get the noted defaults.
type Deps struct {
	Logger      *slog.Logger
	Frames      FrameSource
	Alerts      AlertDispatcher                                          // nil = every fired action reported as skipped "alerts unavailable"
	NewDetector func(context.Context, DetectorOptions) (Detector, error) // nil = NewDetector
	Now         func() time.Time                                         // nil = time.Now; the service truncates every reading to milliseconds UTC
}

var defaultService atomic.Pointer[Service]

// SetDefault publishes the server-owned service to the agent tool.
func SetDefault(s *Service) { defaultService.Store(s) }

// Default returns the server-owned service, or nil.
func Default() *Service { return defaultService.Load() }

// millisUTC is t in UTC truncated to milliseconds, without a monotonic reading.
// That is how detections.db keeps times (see fromUnixMillis), so the values the
// service holds in memory equal what the store returns after a restart.
func millisUTC(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

type cameraRuntime struct {
	cam         Camera
	cancel      context.CancelFunc
	state       string
	lastError   string
	failures    int
	rate        analysisRate
	lastEventID int64
	lastEventAt time.Time
}

type armCache struct {
	mode      ArmMode
	armed     bool
	changedAt time.Time
	changedBy string
}

// openedStore is a freshly opened event store with the state the service reads
// from it at start.
type openedStore struct {
	store      *EventStore
	arm        armCache
	armLoaded  bool
	lastEvents map[string]lastEventRef
	totalBytes int64
}

// Service runs person detection: a frame loop per camera, one detection loop over
// the single shared detector, event tracking, storage, retention, arming and alert
// dispatch.
//
// Lock order: armMu before mu; trackMu before mu. Code holding mu never takes
// trackMu or armMu. detectMu is held only around the detector's Detect and Close.
// Nothing slow runs under mu: opening detections.db and the detector setup run in
// spawned goroutines that publish their outcome under mu. Frame files, thumbnails
// and alert dispatches run in per-camera storage goroutines (storeQueues), outside
// trackMu and off the detection loop.
type Service struct {
	logger      *slog.Logger
	frames      FrameSource
	alerts      AlertDispatcher
	newDetector func(context.Context, DetectorOptions) (Detector, error)
	now         func() time.Time                                               // milliseconds UTC, see millisUTC
	storeOpener func(dataDir string, logger *slog.Logger) (*EventStore, error) // OpenEventStore; tests replace it before the first Configure

	mu           sync.Mutex
	settings     Settings
	cameras      map[string]*cameraRuntime
	lastEvents   map[string]lastEventRef
	store        *EventStore
	storeOpening chan struct{} // closed when the running open of detections.db ends; nil while none runs
	storeErr     error         // outcome of the latest failed open; nil after a successful one
	runCtx       context.Context
	runCancel    context.CancelFunc
	closed       bool
	manual       bool // tests: no background loops (export_service_test.go)
	det          Detector
	detGen       uint64
	detStatus    DetectorStatus
	setupCancel  context.CancelFunc
	retryCancel  context.CancelFunc // the scheduled automatic setup after a detector failure
	detReadyAt   time.Time          // when det became ready (monotonic)
	detRetries   int                // automatic setups since the last other setup
	detRetryBase time.Duration      // DetectorRetryDelay; tests shorten it
	detRetryMax  time.Duration      // MaxDetectorRetryDelay
	timing       timingAverage
	arm          armCache
	lastSent     map[string]LastSent // per camera; in memory only, so a restart resets the cooldowns
	storageBytes int64
	storageWarn  string
	storeQueues  map[string]*storeQueue       // per camera, present while its storage goroutine runs
	thumbnailer  func([]byte) ([]byte, error) // makeThumbnail; tests replace it
	sweepEvery   time.Duration                // SweepInterval; tests shorten it before the first Configure
	sweeps       atomic.Int64                 // sweeps run, for tests

	wg           sync.WaitGroup
	slots        *frameSlots
	wake         chan struct{}
	retentionNow chan struct{}
	armNow       chan struct{}

	armMu      sync.Mutex
	trackMu    sync.Mutex
	trackers   map[string]*tracker
	openEvents map[string]int64
	openSeq    map[string]int

	detectMu sync.Mutex
}

// NewService creates a stopped service; Configure starts it.
func NewService(deps Deps) *Service {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	newDetector := deps.NewDetector
	if newDetector == nil {
		newDetector = NewDetector
	}
	clock := deps.Now
	if clock == nil {
		clock = time.Now
	}
	return &Service{
		logger: logger, frames: deps.Frames, alerts: deps.Alerts, newDetector: newDetector,
		now:          func() time.Time { return millisUTC(clock()) },
		storeOpener:  OpenEventStore,
		settings:     normalizeSettings(Settings{}),
		cameras:      map[string]*cameraRuntime{},
		lastEvents:   map[string]lastEventRef{},
		detStatus:    offDetectorStatus(),
		detRetryBase: DetectorRetryDelay,
		detRetryMax:  MaxDetectorRetryDelay,
		arm:          armCache{mode: ArmOff},
		lastSent:     map[string]LastSent{},
		storeQueues:  map[string]*storeQueue{},
		thumbnailer:  makeThumbnail,
		sweepEvery:   SweepInterval,
		slots:        newFrameSlots(),
		wake:         make(chan struct{}, 1),
		retentionNow: make(chan struct{}, 1),
		armNow:       make(chan struct{}, 1),
		trackers:     map[string]*tracker{},
		openEvents:   map[string]int64{},
		openSeq:      map[string]int{},
	}
}

func normalizeCamera(c Camera) Camera {
	c.ID = strings.TrimSpace(c.ID)
	if strings.TrimSpace(c.Name) == "" {
		c.Name = c.ID
	}
	if c.Interval <= 0 {
		c.Interval = DefaultCameraInterval
	}
	if c.MinScore <= 0 {
		c.MinScore = DefaultCameraMinScore
	}
	if c.Cooldown < 0 {
		c.Cooldown = 0
	}
	zones := make([]Zone, 0, len(c.Zones))
	for _, zone := range c.Zones {
		zones = append(zones, Zone{Name: zone.Name, Points: append([]Point(nil), zone.Points...)})
	}
	c.Zones = zones
	return c
}

func normalizeSettings(in Settings) Settings {
	out := in
	out.DataDir = strings.TrimSpace(in.DataDir)
	if out.Model == "" {
		out.Model = ModelStandard
	}
	if out.Backend == "" {
		out.Backend = BackendAuto
	}
	if out.RetentionDays <= 0 {
		out.RetentionDays = DefaultRetentionDays
	}
	if out.MaxStorageMB <= 0 {
		out.MaxStorageMB = DefaultMaxStorageMB
	}
	if out.Location == nil {
		out.Location = time.Local
	}
	out.Schedule = make([]Window, 0, len(in.Schedule))
	for _, w := range in.Schedule {
		out.Schedule = append(out.Schedule, Window{Days: append([]time.Weekday(nil), w.Days...), From: w.From, To: w.To})
	}
	out.Cameras = make([]Camera, 0, len(in.Cameras))
	seen := make(map[string]bool, len(in.Cameras))
	for _, c := range in.Cameras {
		c = normalizeCamera(c)
		if c.ID == "" || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		out.Cameras = append(out.Cameras, c)
	}
	return out
}

func offDetectorStatus() DetectorStatus {
	return DetectorStatus{State: DetectorOff, Providers: []string{}}
}

func detectorStateForPhase(phase string) string {
	if strings.HasPrefix(phase, "downloading") {
		return DetectorDownloading
	}
	return DetectorPreparing
}

func detectorStatusFromInfo(info DetectorInfo) DetectorStatus {
	state := DetectorReady
	if info.Requested == BackendGPU && info.Active != BackendGPU {
		state = DetectorDegraded
	}
	selfTest := info.SelfTest
	return DetectorStatus{
		State: state, Model: info.Model, ModelName: info.ModelName,
		RequestedBackend: info.Requested, Backend: info.Active, Runtime: info.Runtime,
		Providers: append([]string{}, info.Providers...), FallbackReason: info.FallbackReason,
		SelfTest: &selfTest,
	}
}

func detectionsDBExists(dataDir string) bool {
	dbPath, _ := DetectionsPaths(dataDir)
	_, err := os.Stat(dbPath)
	return err == nil
}

func pokeChannel(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// spawn runs fn in a goroutine that Shutdown waits for. Call it with mu held or
// from a spawned goroutine, so it never races with Shutdown's wait.
func (s *Service) spawn(fn func()) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fn()
	}()
}

func (s *Service) storeRef() *EventStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store
}

// apiStore returns the event store for an API call, or why there is none:
// ErrDisabled when the service was shut down, or when detection is switched off
// and no database is open (a switched-off service opens an existing detections.db,
// so stored events stay readable); ErrUnavailable when detection is enabled but
// detections.db is not open, because opening it failed (the error carries the
// cause) or the open still runs.
func (s *Service) apiStore() (*EventStore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		return nil, ErrDisabled
	case s.store != nil:
		return s.store, nil
	case !s.settings.Enabled:
		return nil, ErrDisabled
	case s.storeOpening == nil && s.storeErr != nil:
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, s.storeErr)
	default:
		return nil, fmt.Errorf("%w: the detections database is not open yet", ErrUnavailable)
	}
}

func (s *Service) locationLocked() *time.Location {
	if s.settings.Location == nil {
		return time.Local
	}
	return s.settings.Location
}

// closeDetectorAsync closes det after any Detect call in flight.
func (s *Service) closeDetectorAsync(det Detector) {
	s.spawn(func() {
		s.detectMu.Lock()
		defer s.detectMu.Unlock()
		if err := det.Close(); err != nil && !errors.Is(err, ErrDetectorClosed) {
			s.logger.Debug("[PersonDetect] Closing the detector failed", "error", err)
		}
	})
}

// Configure applies settings. It is idempotent and never waits for running work:
// it starts, reconfigures or stops the service. The detector is rebuilt only when
// Enabled, Model or Backend change, or when the last setup could not open
// detections.db. The database is opened in the background (see ensureStore), only
// when detection is enabled or detections.db already exists, so a disabled
// install never gets the file; once open it stays open until Shutdown.
func (s *Service) Configure(settings Settings) {
	next := normalizeSettings(settings)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	prev := s.settings
	running := s.runCancel != nil
	s.settings = next
	if !next.Enabled {
		if running {
			s.stopLocked()
		}
		if s.store == nil && s.storeOpening == nil && next.DataDir != "" && detectionsDBExists(next.DataDir) {
			s.spawn(func() {
				if _, err := s.ensureStore(context.Background()); err != nil && !errors.Is(err, ErrDisabled) {
					s.logger.Warn("[PersonDetect] Could not open the detections database", "error", err)
				}
			})
		}
		return
	}
	rebuild := !running || prev.Model != next.Model || prev.Backend != next.Backend
	if !running {
		s.startLocked()
	}
	if !rebuild && s.store == nil && s.setupCancel == nil {
		// The last setup could not open detections.db: try again.
		rebuild = true
	}
	if rebuild {
		s.startDetectorLocked(false)
	}
	s.syncCamerasLocked()
	pokeChannel(s.armNow)
}

// ensureStore returns the event store and opens it when there is none.
//
// OpenEventStore runs dbutil's integrity check: about 1.65 s for a 184 MB database
// with 1M frames, longer with a cold cache on a Raspberry Pi. The open and the
// start-up reads therefore run without mu, so Configure (called under the
// server's config lock) and Status never wait for them. One open runs at a time:
// a caller that finds one running waits for it (or until ctx ends) and gets its
// outcome instead of starting another.
func (s *Service) ensureStore(ctx context.Context) (*EventStore, error) {
	waited := false
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, ErrDisabled
		}
		if st := s.store; st != nil {
			s.mu.Unlock()
			return st, nil
		}
		if opening := s.storeOpening; opening != nil {
			s.mu.Unlock()
			select {
			case <-opening:
				waited = true
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if waited && s.storeErr != nil {
			err := s.storeErr
			s.mu.Unlock()
			return nil, err
		}
		done := make(chan struct{})
		s.storeOpening = done
		dataDir := s.settings.DataDir
		s.mu.Unlock()
		return s.openAndPublishStore(dataDir, done)
	}
}

// openAndPublishStore runs one open for ensureStore and publishes its outcome.
// After Shutdown the fresh store is closed again.
func (s *Service) openAndPublishStore(dataDir string, done chan struct{}) (*EventStore, error) {
	opened, err := s.openStore(dataDir)
	s.mu.Lock()
	s.storeOpening = nil
	close(done)
	if err == nil && s.closed {
		err = ErrDisabled
	}
	if err != nil {
		s.storeErr = err
		s.mu.Unlock()
		if opened.store != nil {
			if closeErr := opened.store.Close(); closeErr != nil {
				s.logger.Debug("[PersonDetect] Closing the detections database failed", "error", closeErr)
			}
		}
		return nil, err
	}
	s.publishStoreLocked(opened)
	s.mu.Unlock()
	return opened.store, nil
}

// openStore opens detections.db below dataDir, ends the events a crash left open
// and reads the arm state, the newest event per camera and the stored bytes.
func (s *Service) openStore(dataDir string) (openedStore, error) {
	st, err := s.storeOpener(dataDir, s.logger)
	if err != nil {
		return openedStore{}, err
	}
	out := openedStore{store: st, lastEvents: map[string]lastEventRef{}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if closed, err := st.CloseOpenEvents(ctx); err != nil {
		s.logger.Warn("[PersonDetect] Could not close events left open", "error", err)
	} else if closed > 0 {
		s.logger.Info("[PersonDetect] Closed events left open by the previous run", "count", closed)
	}
	if rec, err := st.LoadArm(ctx); err == nil {
		out.arm = armCache{mode: rec.Mode, armed: rec.Armed, changedAt: rec.ChangedAt, changedBy: rec.ChangedBy}
		out.armLoaded = true
	} else {
		s.logger.Warn("[PersonDetect] Could not read the arm state; starting disarmed", "error", err)
	}
	if last, err := st.LastEvents(ctx); err == nil {
		out.lastEvents = last
	} else {
		s.logger.Warn("[PersonDetect] Could not read the last event per camera", "error", err)
	}
	if total, err := st.TotalBytes(ctx); err == nil {
		out.totalBytes = total
	} else {
		s.logger.Warn("[PersonDetect] Could not read the stored bytes", "error", err)
	}
	return out, nil
}

func (s *Service) publishStoreLocked(o openedStore) {
	s.store, s.storeErr = o.store, nil
	if o.armLoaded {
		s.arm = o.arm
	}
	s.lastEvents = o.lastEvents
	s.storageBytes = o.totalBytes
	for id, rt := range s.cameras {
		if last, ok := o.lastEvents[id]; ok {
			rt.lastEventID, rt.lastEventAt = last.id, last.at
		}
	}
	pokeChannel(s.armNow)
}

func (s *Service) startLocked() {
	ctx, cancel := context.WithCancel(context.Background())
	s.runCtx, s.runCancel = ctx, cancel
	if s.manual {
		return
	}
	s.wg.Add(3)
	go s.detectLoop(ctx)
	go s.retentionLoop(ctx)
	go s.armLoop(ctx)
}

func (s *Service) stopLocked() {
	s.runCancel()
	s.runCtx, s.runCancel = nil, nil
	if s.setupCancel != nil {
		s.setupCancel()
		s.setupCancel = nil
	}
	if s.retryCancel != nil {
		s.retryCancel()
		s.retryCancel = nil
	}
	s.detGen++
	if det := s.det; det != nil {
		s.det = nil
		s.closeDetectorAsync(det)
	}
	s.detStatus = offDetectorStatus()
	s.timing = timingAverage{}
	for id, rt := range s.cameras {
		rt.cancel()
		delete(s.cameras, id)
	}
	s.slots.setOrder(nil)
	s.storageWarn = ""
	s.spawn(func() { s.closeOpenEvents(context.Background()) })
}

// startDetectorLocked replaces the detector: it cancels a running setup and a
// scheduled automatic one, closes the current detector and starts a new setup in
// the background. The setup first makes sure detections.db is open (ensureStore),
// so cameras, which wait for the detector, never analyze frames without storage;
// then it runs NewDetector. auto marks the automatic setup after a detector
// failure; any other setup (settings, Retry) restarts the retry delays.
func (s *Service) startDetectorLocked(auto bool) {
	if s.setupCancel != nil {
		s.setupCancel()
		s.setupCancel = nil
	}
	if s.retryCancel != nil {
		s.retryCancel()
		s.retryCancel = nil
	}
	if det := s.det; det != nil {
		s.det = nil
		s.closeDetectorAsync(det)
	}
	if !auto {
		s.detRetries = 0
	}
	s.detGen++
	gen := s.detGen
	settings := s.settings
	status := DetectorStatus{State: DetectorPreparing, Model: settings.Model, RequestedBackend: settings.Backend, Providers: []string{}}
	if spec, ok := Model(settings.Model); ok {
		status.ModelName = spec.Name
	}
	s.detStatus = status
	s.timing = timingAverage{}
	ctx, cancel := context.WithCancel(s.runCtx)
	s.setupCancel = cancel
	opts := DetectorOptions{
		ModelDir:        filepath.Join(settings.DataDir, "persondetect", "models"),
		RuntimeCacheDir: filepath.Join(settings.DataDir, "embeddings"),
		Model:           settings.Model,
		Backend:         settings.Backend,
		Logger:          s.logger,
		OnProgress:      func(p SetupProgress) { s.setupProgress(gen, p) },
	}
	s.spawn(func() {
		if _, err := s.ensureStore(ctx); err != nil {
			s.setupDone(gen, nil, fmt.Errorf("storage: %w", err), auto)
			return
		}
		if err := ctx.Err(); err != nil {
			s.setupDone(gen, nil, err, auto)
			return
		}
		det, err := s.newDetector(ctx, opts)
		s.setupDone(gen, det, err, auto)
	})
}

func (s *Service) setupProgress(gen uint64, p SetupProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.detGen {
		return
	}
	s.detStatus.State = detectorStateForPhase(p.Phase)
	s.detStatus.Phase = p.Phase
	s.detStatus.Download = p.Download
}

// setupDone publishes the outcome of setup gen. A failed automatic setup schedules
// the next one; any other failed setup waits for Retry or a settings change.
func (s *Service) setupDone(gen uint64, det Detector, err error, auto bool) {
	s.mu.Lock()
	if gen != s.detGen || s.runCancel == nil {
		s.mu.Unlock()
		if det != nil {
			s.closeDetectorAsync(det)
		}
		return
	}
	s.setupCancel = nil
	if err != nil {
		s.detStatus.State = DetectorError
		s.detStatus.Phase = ""
		s.detStatus.Download = onnxrt.DownloadStatus{}
		s.detStatus.Error = err.Error()
		if !auto {
			s.mu.Unlock()
			s.logger.Warn("[PersonDetect] Detector setup failed", "error", err)
			return
		}
		delay := s.scheduleDetectorRetryLocked()
		s.mu.Unlock()
		s.logger.Warn("[PersonDetect] Detector setup failed; trying again", "retry_in", delay, "error", err)
		return
	}
	s.det = det
	s.detReadyAt = time.Now()
	s.detStatus = detectorStatusFromInfo(det.Info())
	state := s.detStatus.State
	s.mu.Unlock()
	s.logger.Info("[PersonDetect] Detector ready", "state", state, "backend", det.Info().Active)
}

// scheduleDetectorRetryLocked schedules an automatic detector setup after a
// detector failure and returns its delay: the retry base (DetectorRetryDelay) for
// the first, doubling per further automatic setup up to MaxDetectorRetryDelay. Any
// other setup, a stop and Shutdown cancel it.
func (s *Service) scheduleDetectorRetryLocked() time.Duration {
	if s.retryCancel != nil {
		s.retryCancel()
	}
	s.detRetries++
	delay := backoffDelay(s.detRetryBase, s.detRetryMax, s.detRetries)
	gen := s.detGen
	ctx, cancel := context.WithCancel(s.runCtx)
	s.retryCancel = cancel
	s.spawn(func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed || s.runCancel == nil || ctx.Err() != nil || gen != s.detGen {
			return
		}
		s.retryCancel = nil
		cancel()
		s.startDetectorLocked(true)
	})
	return delay
}

// detectorFailed takes a detector that failed for good (its error wraps
// ErrDetectorFailed: the worker kept failing) out of service and schedules a new
// setup, which runs NewDetector again and may land on the CPU. The detector state
// is error until then. It returns the delay, or false when det is not the current
// detector (it was replaced, or the service stopped).
func (s *Service) detectorFailed(det Detector, err error) (time.Duration, bool) {
	s.mu.Lock()
	if det == nil || s.det != det || s.runCancel == nil {
		s.mu.Unlock()
		return 0, false
	}
	s.det = nil
	s.detGen++
	s.detStatus.State = DetectorError
	s.detStatus.Phase = ""
	s.detStatus.Error = err.Error()
	if time.Since(s.detReadyAt) >= s.detRetryMax {
		s.detRetries = 0 // it served long enough to count as healthy
	}
	delay := s.scheduleDetectorRetryLocked()
	// Spawned under mu: a caller outside the service's goroutines (an API request)
	// must not add to the wait group while Shutdown waits on it.
	s.closeDetectorAsync(det)
	s.mu.Unlock()
	s.logger.Error("[PersonDetect] Detector failed; setting it up again", "retry_in", delay, "error", err)
	return delay, true
}

// detectErrorAction is how a Detect error is handled.
type detectErrorAction int

const (
	detectSkipFrame    detectErrorAction = iota // service state: drop the frame, keep the camera state
	detectFailDetector                          // ErrDetectorFailed: take the detector out of service
	detectFailCamera                            // about this frame: mark the camera
)

// classifyDetectError sorts a Detect error. Worker failures wrap
// ErrWorkerRestarting and resolve on their own, ErrDetectorClosed means the
// detector was replaced, and a done ctx means the service stops: none of them is a
// fault of the camera that sent the frame. ErrDetectorFailed is checked first: its
// cause may wrap a restart.
func classifyDetectError(ctx context.Context, err error) detectErrorAction {
	switch {
	case errors.Is(err, ErrDetectorFailed):
		return detectFailDetector
	case ctx.Err() != nil, errors.Is(err, ErrWorkerRestarting), errors.Is(err, ErrDetectorClosed):
		return detectSkipFrame
	default:
		return detectFailCamera
	}
}

// handleDetectError applies an error of det.Detect for a frame of cameraID (the
// detection loop's policy, see classifyDetectError).
func (s *Service) handleDetectError(ctx context.Context, det Detector, cameraID string, err error) {
	switch classifyDetectError(ctx, err) {
	case detectFailDetector:
		s.detectorFailed(det, err)
	case detectFailCamera:
		s.setCameraState(cameraID, CameraError, err.Error())
	}
}

// cameraStartDelay is how long camera index (in Settings.Cameras order) of count
// cameras waits before its first frame request: index × interval / count. That
// spreads the first requests, and with them the detector load and the frame age,
// over one interval instead of firing every camera at once. Only the start is
// staggered; FetchBackoff stays deterministic.
func cameraStartDelay(index, count int, interval time.Duration) time.Duration {
	if count <= 0 || index <= 0 || index >= count || interval <= 0 {
		return 0
	}
	return interval * time.Duration(index) / time.Duration(count)
}

func (s *Service) syncCamerasLocked() {
	count := len(s.settings.Cameras)
	ids := make([]string, 0, count)
	keep := make(map[string]bool, count)
	intervalChanged := false
	for i, cam := range s.settings.Cameras {
		ids = append(ids, cam.ID)
		keep[cam.ID] = true
		if rt, ok := s.cameras[cam.ID]; ok {
			intervalChanged = intervalChanged || rt.cam.Interval != cam.Interval
			rt.cam = cam
			continue
		}
		ctx, cancel := context.WithCancel(s.runCtx)
		rt := &cameraRuntime{cam: cam, cancel: cancel, state: CameraWaiting}
		if last, ok := s.lastEvents[cam.ID]; ok {
			rt.lastEventID, rt.lastEventAt = last.id, last.at
		}
		s.cameras[cam.ID] = rt
		if !s.manual {
			s.wg.Add(1)
			go s.cameraLoop(ctx, cam.ID, cameraStartDelay(i, count, cam.Interval))
		}
	}
	var removed []string
	for id, rt := range s.cameras {
		if keep[id] {
			continue
		}
		rt.cancel()
		delete(s.cameras, id)
		removed = append(removed, id)
	}
	s.slots.setOrder(ids)
	if len(removed) > 0 {
		s.spawn(func() { s.closeCameraEvents(context.Background(), removed) })
	}
	if intervalChanged {
		s.spawn(s.applyEndAfter)
	}
}

// closeOpenEvents ends every open event at its last hit (stop and shutdown).
func (s *Service) closeOpenEvents(ctx context.Context) {
	s.trackMu.Lock()
	defer s.trackMu.Unlock()
	for cameraID, tr := range s.trackers {
		if at, ok := tr.Close(); ok {
			s.endEventLocked(ctx, cameraID, at)
		}
	}
	s.trackers = map[string]*tracker{}
}

// closeCameraEvents ends the open events of removed cameras.
func (s *Service) closeCameraEvents(ctx context.Context, cameraIDs []string) {
	s.trackMu.Lock()
	defer s.trackMu.Unlock()
	for _, cameraID := range cameraIDs {
		tr := s.trackers[cameraID]
		if tr == nil {
			continue
		}
		if at, ok := tr.Close(); ok {
			s.endEventLocked(ctx, cameraID, at)
		}
		delete(s.trackers, cameraID)
	}
}

// endEventLocked records the end of a camera's open event. The caller holds trackMu.
func (s *Service) endEventLocked(ctx context.Context, cameraID string, at time.Time) {
	id := s.openEvents[cameraID]
	delete(s.openEvents, cameraID)
	delete(s.openSeq, cameraID)
	st := s.storeRef()
	if id == 0 || st == nil {
		return
	}
	if err := st.EndEvent(ctx, id, at); err != nil && !s.noteStorageFull(err) {
		s.logger.Warn("[PersonDetect] Could not end an event", "event_id", id, "error", err)
	}
}

// Status returns the current state for the app and the agent tool.
func (s *Service) Status() Status {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{
		Enabled:  s.settings.Enabled,
		Detector: s.detStatus,
		Arm:      s.armStateLocked(now),
		Cameras:  []CameraStatus{},
		Storage:  StorageStatus{Bytes: s.storageBytes, LimitBytes: s.settings.MaxStorageBytes(), Warning: s.storageWarn},
	}
	st.Detector.Providers = append([]string{}, s.detStatus.Providers...)
	if s.detStatus.SelfTest != nil {
		selfTest := *s.detStatus.SelfTest
		st.Detector.SelfTest = &selfTest
	}
	st.Detector.WallMS, st.Detector.CPUMS = s.timing.average()
	if s.runCancel != nil {
		for _, cam := range s.settings.Cameras {
			cs := CameraStatus{ID: cam.ID, Name: cam.Name, State: CameraWaiting}
			if rt := s.cameras[cam.ID]; rt != nil {
				cs.State, cs.LastError = rt.state, rt.lastError
				cs.AnalysesPerMinute = rt.rate.perMinute(now)
				if rt.lastEventID != 0 {
					id, at := rt.lastEventID, millisUTC(rt.lastEventAt)
					cs.LastEventID, cs.LastEventAt = &id, &at
				}
			}
			st.Cameras = append(st.Cameras, cs)
		}
	}
	if s.store != nil {
		st.Cursor = s.store.Cursor()
	}
	return st
}

func (s *Service) armStateLocked(now time.Time) ArmState {
	local := now.In(s.locationLocked())
	armed, next := EffectiveArmed(local, s.arm.mode, s.settings.Schedule)
	zone, _ := local.Zone()
	state := ArmState{Mode: s.arm.mode, Armed: armed, ChangedBy: s.arm.changedBy, TimeZone: zone, UTCOffset: local.Format("-07:00")}
	if !next.IsZero() {
		n := millisUTC(next)
		state.NextChange = &n
	}
	if !s.arm.changedAt.IsZero() {
		c := millisUTC(s.arm.changedAt)
		state.ChangedAt = &c
	}
	return state
}

// Retry leaves the error state: it sets the detector up again after a setup or
// detector failure, which also opens detections.db again when that failed. It
// replaces a scheduled automatic setup and restarts the retry delays. A detector
// that is not in the error state is left alone.
func (s *Service) Retry() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !s.settings.Enabled || s.runCancel == nil {
		return ErrDisabled
	}
	if s.detStatus.State == DetectorError {
		s.startDetectorLocked(false)
	}
	return nil
}

// Shutdown stops all loops, closes open events at their last hit, closes the
// detector and the database. It is idempotent. An open of detections.db that is
// still running cannot be interrupted; Shutdown waits for it (or until ctx ends),
// and the database it opened is closed again.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.runCancel != nil {
		s.stopLocked()
	}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return fmt.Errorf("persondetect: shutdown: %w", ctx.Err())
	}
	s.mu.Lock()
	st := s.store
	s.store = nil
	s.mu.Unlock()
	if st != nil {
		return st.Close()
	}
	return nil
}
