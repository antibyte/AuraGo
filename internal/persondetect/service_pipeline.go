package persondetect

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"path/filepath"
	"time"

	"modernc.org/sqlite"
)

// sqliteFull is SQLITE_FULL, the primary result code of "database or disk is full".
const sqliteFull = 13

// pendingFrame is a frame the tracker chose to store, numbered within its event.
type pendingFrame struct {
	frame AnalyzedFrame
	seq   int
}

// storeJob is the storage work of one observation: the frames to write for an
// event and, at the event start, the alert to dispatch once the start frames are
// stored (the alert names the cover frame).
type storeJob struct {
	cam     Camera
	eventID int64
	frames  []pendingFrame
	alert   *AlertEvent
}

// storeQueue holds a camera's storage jobs. One goroutine at a time drains it, so
// a camera's frames are written in order and at most one of its frames is decoded
// for a thumbnail at a time.
type storeQueue struct {
	jobs []storeJob
}

// detectLoop analyzes queued frames one at a time over the shared detector and
// ends events of cameras that stopped delivering hits.
func (s *Service) detectLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.sweepEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
			s.processQueued(ctx)
		case <-ticker.C:
			s.sweep(s.now)
		}
	}
}

// processQueued analyzes at most the frames that are queued when it starts. When
// detection is slower than frames arrive the slots never empty, and draining them
// would keep the loop from ever reaching the sweep, so the events of a camera that
// stopped delivering frames would stay open. Frames left over wake the loop again
// after the select has given the sweep its turn.
func (s *Service) processQueued(ctx context.Context) {
	for n := s.slots.queued(); n > 0 && ctx.Err() == nil; n-- {
		if !s.processOne(ctx) {
			return
		}
	}
	if s.slots.queued() > 0 {
		pokeChannel(s.wake)
	}
}

// processOne analyzes the next queued frame; false when no frame was queued.
func (s *Service) processOne(ctx context.Context) bool {
	job, ok := s.slots.take()
	if !ok {
		return false
	}
	s.mu.Lock()
	rt, known := s.cameras[job.cameraID]
	det := s.det
	var cam Camera
	if known {
		cam = rt.cam
	}
	s.mu.Unlock()
	if !known {
		return true
	}
	if det == nil {
		s.setCameraState(job.cameraID, CameraWaiting, "")
		return true
	}
	s.detectMu.Lock()
	res, err := det.Detect(ctx, job.jpeg, cam.MinScore)
	s.detectMu.Unlock()
	if err != nil {
		s.handleDetectError(ctx, det, job.cameraID, err)
		return true
	}
	s.recordAnalysis(job.cameraID, job.at, res)
	s.observe(ctx, job.cameraID, analyzeFrame(job, res, cam))
	return true
}

// analyzeFrame classifies the detector's boxes for a camera. Score is the highest
// score among the hits (the boxes that count), which the tracker's single-hit
// start reads.
func analyzeFrame(job frameJob, res DetectResult, cam Camera) AnalyzedFrame {
	hits, zones, score := classifyBoxes(res.Boxes, cam.MinScore, cam.Zones)
	width, height := res.Width, res.Height
	if width <= 0 || height <= 0 {
		if cfg, err := jpeg.DecodeConfig(bytes.NewReader(job.jpeg)); err == nil {
			width, height = cfg.Width, cfg.Height
		}
	}
	return AnalyzedFrame{
		At: job.at, JPEG: job.jpeg, Width: width, Height: height,
		Boxes: append([]Box(nil), res.Boxes...), Hits: hits, Zones: zones, Score: score,
	}
}

func (s *Service) recordAnalysis(cameraID string, at time.Time, res DetectResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timing.add(res.WallMS, res.CPUMS)
	if rt := s.cameras[cameraID]; rt != nil {
		rt.rate.add(at)
		rt.state, rt.lastError = CameraActive, ""
	}
}

// configuredCamera returns the current settings of a detecting camera.
func (s *Service) configuredCamera(cameraID string) (Camera, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := s.cameras[cameraID]
	if rt == nil {
		return Camera{}, false
	}
	return rt.cam, true
}

// observe feeds one analyzed frame to the camera's tracker and records the event
// changes. The frames to store and the alert of an event start are handed to the
// camera's storage goroutine, so thumbnails and file writes never hold up the
// detection loop or trackMu.
func (s *Service) observe(ctx context.Context, cameraID string, f AnalyzedFrame) {
	s.trackMu.Lock()
	job := s.observeLocked(ctx, cameraID, f)
	s.trackMu.Unlock()
	if job != nil {
		s.queueStore(*job)
	}
}

// observeLocked is observe under trackMu.
func (s *Service) observeLocked(ctx context.Context, cameraID string, f AnalyzedFrame) *storeJob {
	if ctx.Err() != nil {
		return nil // the service stops; closeOpenEvents ends the open events
	}
	// The current settings: the camera may have changed (or been removed, and then
	// closeCameraEvents ends its event) while the frame was analyzed.
	cam, ok := s.configuredCamera(cameraID)
	if !ok {
		return nil
	}
	tr := s.trackers[cameraID]
	if tr == nil {
		tr = newTracker()
		tr.SetEndAfter(EventEndAfterFor(cam.Interval))
		s.trackers[cameraID] = tr
	}
	step := tr.Observe(f)
	if step.Ended {
		s.endEventLocked(context.Background(), cameraID, step.EndedAt)
	}
	switch {
	case step.Started:
		return s.startEventLocked(cam, tr, step)
	case step.Updated || len(step.Store) > 0:
		return s.extendEventLocked(cam, step)
	}
	return nil
}

// applyEndAfter gives every tracker the end-after time of its camera's current
// interval (EventEndAfterFor); Configure runs it when an interval changed.
func (s *Service) applyEndAfter() {
	s.trackMu.Lock()
	defer s.trackMu.Unlock()
	s.mu.Lock()
	intervals := make(map[string]time.Duration, len(s.cameras))
	for id, rt := range s.cameras {
		intervals[id] = rt.cam.Interval
	}
	s.mu.Unlock()
	for id, tr := range s.trackers {
		if interval, ok := intervals[id]; ok {
			tr.SetEndAfter(EventEndAfterFor(interval))
		}
	}
}

// startEventLocked records a new event and decides its alerts; it returns the
// storage job of the start frames, which dispatches the alert. The caller holds
// trackMu.
func (s *Service) startEventLocked(cam Camera, tr *tracker, step trackStep) *storeJob {
	st := s.storeRef()
	if st == nil {
		s.abandonStartLocked(cam.ID, tr, errors.New("the detections database is not open"))
		return nil
	}
	s.mu.Lock()
	// now is read in the same section as lastSent: an alert report that records a
	// sent time between two separate reads would look like a backward clock step
	// and fire that action during its cooldown (see decideFire).
	evaluatedAt := s.now()
	loc := s.locationLocked()
	mode := s.arm.mode
	schedule := s.settings.Schedule
	language := s.settings.Language
	lastSent := s.lastSent[cam.ID]
	s.mu.Unlock()
	armed, _ := EffectiveArmed(evaluatedAt.In(loc), mode, schedule)
	fire, skipped := decideFire(armed, evaluatedAt, lastSent, cam)
	id, err := st.CreateEvent(context.Background(), NewEvent{
		CameraID: cam.ID, CameraName: cam.Name, StartedAt: step.StartedAt, Armed: armed, Stats: step.Stats, Alerts: skipped,
	})
	if err != nil {
		s.abandonStartLocked(cam.ID, tr, err)
		return nil
	}
	s.openEvents[cam.ID] = id
	s.openSeq[cam.ID] = 0
	s.mu.Lock()
	s.lastEvents[cam.ID] = lastEventRef{id: id, at: step.StartedAt}
	if rt := s.cameras[cam.ID]; rt != nil {
		rt.lastEventID, rt.lastEventAt = id, step.StartedAt
	}
	s.mu.Unlock()
	return &storeJob{
		cam: cam, eventID: id, frames: s.numberFramesLocked(cam.ID, step.Store),
		alert: &AlertEvent{
			EventID: id, CameraID: cam.ID, CameraName: cam.Name,
			Zones: append([]string{}, step.Stats.Zones...), Score: step.Stats.MaxScore, Persons: step.Stats.MaxPersons,
			Armed: armed, StartedAt: step.StartedAt, Actions: cam.Actions, Fire: fire, Language: language,
		},
	}
}

// abandonStartLocked gives up an event start that could not be recorded. The
// tracker forgets the event, so the next hits try to start one again instead of
// extending an event that does not exist until the person leaves. The caller
// holds trackMu.
func (s *Service) abandonStartLocked(cameraID string, tr *tracker, err error) {
	tr.Close()
	delete(s.openEvents, cameraID)
	delete(s.openSeq, cameraID)
	if !s.noteStorageFull(err) {
		s.logger.Error("[PersonDetect] Could not record an event", "camera", cameraID, "error", err)
	}
}

// extendEventLocked updates the aggregates of the open event and returns the
// storage job of its throttled frames. The caller holds trackMu.
func (s *Service) extendEventLocked(cam Camera, step trackStep) *storeJob {
	id := s.openEvents[cam.ID]
	st := s.storeRef()
	if id == 0 || st == nil {
		return nil
	}
	if step.Updated {
		if err := st.UpdateEventStats(context.Background(), id, step.Stats); err != nil && !s.noteStorageFull(err) {
			s.logger.Warn("[PersonDetect] Could not update an event", "event_id", id, "error", err)
		}
	}
	if len(step.Store) == 0 {
		return nil
	}
	return &storeJob{cam: cam, eventID: id, frames: s.numberFramesLocked(cam.ID, step.Store)}
}

// numberFramesLocked gives frames the next sequence numbers of the camera's open
// event. The caller holds trackMu.
func (s *Service) numberFramesLocked(cameraID string, frames []AnalyzedFrame) []pendingFrame {
	out := make([]pendingFrame, 0, len(frames))
	for _, f := range frames {
		s.openSeq[cameraID]++
		out = append(out, pendingFrame{frame: f, seq: s.openSeq[cameraID]})
	}
	return out
}

// queueStore hands a job to its camera's storage goroutine, starting one when none
// runs. After Shutdown began, the job runs in the caller (the detection loop,
// which Shutdown waits for).
func (s *Service) queueStore(job storeJob) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.runStoreJob(job)
		return
	}
	id := job.cam.ID
	if q := s.storeQueues[id]; q != nil {
		q.jobs = append(q.jobs, job)
		s.mu.Unlock()
		return
	}
	s.storeQueues[id] = &storeQueue{jobs: []storeJob{job}}
	s.spawn(func() { s.drainStore(id) })
	s.mu.Unlock()
}

// drainStore runs a camera's storage jobs in order until its queue is empty.
func (s *Service) drainStore(cameraID string) {
	for {
		s.mu.Lock()
		q := s.storeQueues[cameraID]
		if q == nil || len(q.jobs) == 0 {
			delete(s.storeQueues, cameraID)
			s.mu.Unlock()
			return
		}
		job := q.jobs[0]
		q.jobs[0] = storeJob{}
		q.jobs = q.jobs[1:]
		s.mu.Unlock()
		s.runStoreJob(job)
	}
}

// runStoreJob writes a job's frames and then dispatches its alert with the best
// stored frame as the cover (no cover when no frame could be written).
func (s *Service) runStoreJob(job storeJob) {
	coverPath, best := "", -1.0
	var coverBoxes []Box
	if st := s.storeRef(); st != nil {
		for _, pf := range job.frames {
			path, ok := s.storeFrame(st, job.cam.ID, job.eventID, pf)
			if ok && pf.frame.Score > best {
				best, coverPath = pf.frame.Score, path
				coverBoxes = append([]Box(nil), pf.frame.Hits...)
			}
		}
	}
	if job.alert != nil {
		alert := *job.alert
		alert.CoverPath, alert.CoverBoxes = coverPath, coverBoxes
		s.dispatchIfCurrent(alert)
	}
}

// dispatchIfCurrent dispatches the alert of an event start unless it went stale
// while the start frames were stored: the event was deleted (nothing is left to
// alert about), or detection was switched off or shut down (each fired action is
// recorded as skipped "detection off"). When the check itself fails the alert is
// dispatched: a missed alert costs more than a doubtful one.
func (s *Service) dispatchIfCurrent(ev AlertEvent) {
	st := s.storeRef()
	if st == nil {
		return
	}
	exists, err := st.eventExists(context.Background(), ev.EventID)
	if err != nil {
		s.logger.Warn("[PersonDetect] Could not check an event before its alert", "event_id", ev.EventID, "error", err)
	} else if !exists {
		s.logger.Debug("[PersonDetect] Alert dropped: its event was deleted", "event_id", ev.EventID)
		return
	}
	if !s.running() {
		for _, action := range ev.Fire.Fired() {
			s.recordAlert(ev.EventID, ev.CameraID, action, AlertResult{Status: AlertSkipped, Detail: "detection off"})
		}
		return
	}
	s.dispatch(ev)
}

// storeFrame writes one frame with its thumbnail and records it; it returns the
// absolute path of the frame. Only the boxes that count are stored with it.
func (s *Service) storeFrame(st *EventStore, cameraID string, eventID int64, pf pendingFrame) (string, bool) {
	f := pf.frame
	rel, thumbRel := frameRelPaths(f.At, cameraID, eventID, pf.seq)
	thumb, err := s.frameThumbnail(f)
	if err != nil {
		s.logger.Warn("[PersonDetect] Could not make a thumbnail", "event_id", eventID, "error", err)
		return "", false
	}
	size, err := writeFrameWithThumbnail(st.Root(), rel, thumbRel, f.JPEG, thumb)
	if err != nil {
		s.noteWriteError(err)
		return "", false
	}
	if _, err := st.AddFrame(context.Background(), eventID, StoredFrame{
		Seq: pf.seq, Path: rel, ThumbPath: thumbRel, Size: size, CapturedAt: f.At,
		Width: f.Width, Height: f.Height, Boxes: f.Hits, Score: f.Score,
	}); err != nil {
		st.removeFile(rel)
		st.removeFile(thumbRel)
		// An unknown event was deleted (DeleteEvent, retention) while its frame was written.
		if !errors.Is(err, ErrUnknownEvent) && !s.noteStorageFull(err) {
			s.logger.Warn("[PersonDetect] Could not record a frame", "event_id", eventID, "error", err)
		}
		return "", false
	}
	s.noteWriteOK(size)
	return filepath.Join(st.Root(), filepath.FromSlash(rel)), true
}

// frameThumbnail returns the thumbnail of a frame. A frame at most ThumbnailWidth
// wide is its own thumbnail (makeThumbnail never scales up), so it is not decoded.
func (s *Service) frameThumbnail(f AnalyzedFrame) ([]byte, error) {
	if f.Width > 0 && f.Width <= ThumbnailWidth {
		return f.JPEG, nil
	}
	s.mu.Lock()
	build := s.thumbnailer
	s.mu.Unlock()
	return build(f.JPEG)
}

// isStorageFull reports whether a write failed because the disk is full: an OS
// error (isNoSpace) or SQLite's SQLITE_FULL ("database or disk is full").
func isStorageFull(err error) bool {
	if err == nil {
		return false
	}
	if isNoSpace(err) {
		return true
	}
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code()&0xff == sqliteFull
}

// noteStorageFull handles a write error that means a full disk: it sets the
// storage_full warning and asks for an immediate retention run. It reports
// whether err was such an error.
func (s *Service) noteStorageFull(err error) bool {
	if !isStorageFull(err) {
		return false
	}
	s.mu.Lock()
	s.storageWarn = StorageFullWarning
	s.mu.Unlock()
	pokeChannel(s.retentionNow)
	s.logger.Error("[PersonDetect] Disk full while storing detections; running retention now", "error", err)
	return true
}

func (s *Service) noteWriteError(err error) {
	if !s.noteStorageFull(err) {
		s.logger.Warn("[PersonDetect] Could not store a frame", "error", err)
	}
}

func (s *Service) noteWriteOK(size int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storageBytes += size
	if s.storageWarn == StorageFullWarning {
		s.storageWarn = ""
	}
}

// dispatch hands an event start to the alert dispatcher (outside every lock).
func (s *Service) dispatch(ev AlertEvent) {
	report := func(action string, result AlertResult) {
		s.recordAlert(ev.EventID, ev.CameraID, action, result)
	}
	if s.alerts == nil {
		for _, action := range ev.Fire.Fired() {
			report(action, AlertResult{Status: AlertSkipped, Detail: "alerts unavailable"})
		}
		return
	}
	s.alerts.Dispatch(ev, report)
}

// recordAlert stores an action result. A sent message, agent or desktop alert
// starts the cooldown of that action only; the mission trigger has none.
func (s *Service) recordAlert(eventID int64, cameraID, action string, result AlertResult) {
	if st := s.storeRef(); st != nil {
		if err := st.SetAlertResult(context.Background(), eventID, action, result); err != nil && !errors.Is(err, ErrUnknownEvent) && !s.noteStorageFull(err) {
			s.logger.Warn("[PersonDetect] Could not record an alert result", "event_id", eventID, "action", action, "error", err)
		}
	}
	if result.Status != AlertSent {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	last := s.lastSent[cameraID]
	switch action {
	case ActionMessage:
		last.Message = s.now()
	case ActionAgent:
		last.Agent = s.now()
	case ActionDesktop:
		last.Desktop = s.now()
	default:
		return
	}
	s.lastSent[cameraID] = last
}

// sweep ends events whose last hit is at least the end-after time before now, or
// after now (the clock stepped back). now is read under trackMu, which serializes
// the sweep with observe: a frame observed between an earlier reading and the lock
// would otherwise lie after now and end its own event. A camera with a frame
// waiting for analysis is left to that frame: it may hold a hit captured before
// the deadline.
func (s *Service) sweep(clock func() time.Time) {
	s.sweeps.Add(1)
	s.trackMu.Lock()
	defer s.trackMu.Unlock()
	now := clock()
	for cameraID, tr := range s.trackers {
		if s.slots.has(cameraID) {
			continue
		}
		if at, ok := tr.Expire(now); ok {
			s.endEventLocked(context.Background(), cameraID, at)
		}
	}
}

// retentionLoop runs retention at start, hourly and when a full disk asks for it.
func (s *Service) retentionLoop(ctx context.Context) {
	defer s.wg.Done()
	// The first pass needs detections.db, which is opened in the background; a
	// failed open is reported by the detector setup.
	if _, err := s.ensureStore(ctx); err == nil {
		_, _ = s.runRetention(ctx)
	}
	ticker := time.NewTicker(RetentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.runRetention(ctx)
		case <-s.retentionNow:
			_, _ = s.runRetention(ctx)
		}
	}
}

func (s *Service) runRetention(ctx context.Context) (RetentionResult, error) {
	s.mu.Lock()
	st := s.store
	days := s.settings.RetentionDays
	limit := s.settings.MaxStorageBytes()
	s.mu.Unlock()
	if st == nil {
		return RetentionResult{}, nil
	}
	res, err := applyRetention(ctx, st, s.now(), days, limit)
	if err != nil {
		if ctx.Err() == nil && !errors.Is(err, context.Canceled) {
			s.logger.Warn("[PersonDetect] Retention run failed", "error", err)
		}
		return res, err
	}
	s.mu.Lock()
	s.storageBytes = res.TotalBytes
	s.mu.Unlock()
	if res.DeletedEvents > 0 {
		s.logger.Info("[PersonDetect] Retention removed events", "events", res.DeletedEvents, "freed_bytes", res.FreedBytes)
	}
	if res.StaleTemporaries > 0 {
		s.logger.Info("[PersonDetect] Retention removed stale temporary frame files", "count", res.StaleTemporaries)
	}
	if res.StaleTemporaryError != nil {
		s.logger.Warn("[PersonDetect] Retention could not remove stale temporary frame files", "count", res.StaleTemporaryFailures, "error", res.StaleTemporaryError)
	}
	return res, nil
}
