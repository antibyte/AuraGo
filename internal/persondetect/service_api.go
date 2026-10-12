package persondetect

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"time"
)

// changesRowLimit caps the events and the arm-log rows of one Changes call each.
const changesRowLimit = 200

// runningLocked reports whether detection runs: enabled, started and not shut down.
func (s *Service) runningLocked() bool {
	return !s.closed && s.settings.Enabled && s.runCancel != nil
}

func (s *Service) running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runningLocked()
}

// Changes returns the current status plus event and arm-log rows.
//
// A cursor of 0 (or below) asks for a snapshot: the newest events and arm-log
// rows (at most changesRowLimit each, ascending) with Cursor = Status.Cursor, so a
// first load never replays old rows as new. A cursor above 0 returns the rows
// written after it; when a list was capped, Cursor is the last delivered revision,
// so the next call continues there (rows may repeat; clients merge by id).
//
// Changes is not limited to a running service: a switched-off service with an
// open database returns its rows like Events does, so the end that switching
// detection off writes to open events, late alert results and the arm history
// still reach clients. Without a database (ErrDisabled, ErrUnavailable) it returns
// the status, which says why, with empty lists and Cursor = Status.Cursor (0);
// only a failing database read is an error.
func (s *Service) Changes(ctx context.Context, cursor int64) (Changes, error) {
	if cursor < 0 {
		cursor = 0
	}
	status := s.Status()
	out := Changes{Status: status, Events: []EventSummary{}, ArmLog: []ArmLogEntry{}, Cursor: status.Cursor}
	st, err := s.apiStore()
	if err != nil {
		return out, nil // ErrDisabled or ErrUnavailable: the status reports it
	}
	if cursor == 0 {
		events, err := st.EventsLatest(ctx, status.Cursor, changesRowLimit)
		if err != nil {
			return Changes{}, err
		}
		armLog, err := st.ArmLogLatest(ctx, status.Cursor, changesRowLimit)
		if err != nil {
			return Changes{}, err
		}
		out.Events, out.ArmLog = events, armLog
		return out, nil
	}
	if cursor >= status.Cursor {
		return out, nil
	}
	events, eventsLast, eventsFull, err := st.EventsBetween(ctx, cursor, status.Cursor, changesRowLimit)
	if err != nil {
		return Changes{}, err
	}
	armLog, armLast, armFull, err := st.ArmLogBetween(ctx, cursor, status.Cursor, changesRowLimit)
	if err != nil {
		return Changes{}, err
	}
	out.Events, out.ArmLog = events, armLog
	if eventsFull && eventsLast < out.Cursor {
		out.Cursor = eventsLast
	}
	if armFull && armLast < out.Cursor {
		out.Cursor = armLast
	}
	return out, nil
}

// SetArmMode sets off, on or schedule; actor is user or agent. A change is stored
// and written to the arm log; setting the current mode again changes nothing.
func (s *Service) SetArmMode(ctx context.Context, mode ArmMode, actor string) (ArmState, error) {
	if !ValidArmMode(string(mode)) {
		return ArmState{}, fmt.Errorf("%w: %q", ErrInvalidArmMode, mode)
	}
	if actor != ActorUser && actor != ActorAgent {
		return ArmState{}, fmt.Errorf("%w: actor %q", ErrInvalidArmMode, actor)
	}
	s.armMu.Lock()
	defer s.armMu.Unlock()
	s.mu.Lock()
	running := s.runningLocked()
	loc := s.locationLocked()
	schedule := s.settings.Schedule
	current := s.arm.mode
	s.mu.Unlock()
	if !running {
		return ArmState{}, ErrDisabled
	}
	st, err := s.apiStore()
	if err != nil {
		return ArmState{}, err
	}
	now := s.now()
	if mode != current {
		armed, _ := EffectiveArmed(now.In(loc), mode, schedule)
		if err := st.RecordArm(ctx, armRecord{Mode: mode, Armed: armed, ChangedAt: now, ChangedBy: actor}); err != nil {
			return ArmState{}, fmt.Errorf("persondetect: record arm mode: %w", err)
		}
		s.mu.Lock()
		s.arm = armCache{mode: mode, armed: armed, changedAt: now, changedBy: actor}
		s.mu.Unlock()
		s.logger.Info("[PersonDetect] Arm mode changed", "mode", mode, "armed", armed, "actor", actor)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.armStateLocked(now), nil
}

// armLoop evaluates the schedule every ArmCheckInterval, after every Configure and
// when the database was opened.
func (s *Service) armLoop(ctx context.Context) {
	defer s.wg.Done()
	s.evaluateArming(ctx, s.now())
	ticker := time.NewTicker(ArmCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluateArming(ctx, s.now())
		case <-s.armNow:
			s.evaluateArming(ctx, s.now())
		}
	}
}

// evaluateArming logs a scheduled transition (actor schedule) when the effective
// armed state of mode schedule changed since the last evaluation.
func (s *Service) evaluateArming(ctx context.Context, now time.Time) {
	s.armMu.Lock()
	defer s.armMu.Unlock()
	s.mu.Lock()
	st := s.store
	running := s.runningLocked()
	loc := s.locationLocked()
	schedule := s.settings.Schedule
	current := s.arm
	s.mu.Unlock()
	if st == nil || !running {
		return
	}
	armed, _ := EffectiveArmed(now.In(loc), current.mode, schedule)
	if armed == current.armed {
		return
	}
	if current.mode != ArmSchedule {
		s.mu.Lock()
		s.arm.armed = armed
		s.mu.Unlock()
		return
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmSchedule, Armed: armed, ChangedAt: now, ChangedBy: ActorSchedule}); err != nil {
		if ctx.Err() == nil {
			s.logger.Warn("[PersonDetect] Could not record a scheduled arm change", "error", err)
		}
		return
	}
	s.mu.Lock()
	s.arm = armCache{mode: ArmSchedule, armed: armed, changedAt: now, changedBy: ActorSchedule}
	s.mu.Unlock()
	s.logger.Info("[PersonDetect] Schedule changed the arm state", "armed", armed)
}

// DeleteEvent deletes an event with its frames and files. A matching open event
// is closed in its tracker first, so the next hit starts a new event. A frame of
// the event that is still being written is dropped when its record finds the
// event gone.
func (s *Service) DeleteEvent(ctx context.Context, id int64) error {
	if !s.running() {
		return ErrDisabled
	}
	st, err := s.apiStore()
	if err != nil {
		return err
	}
	s.trackMu.Lock()
	for cameraID, open := range s.openEvents {
		if open != id {
			continue
		}
		delete(s.openEvents, cameraID)
		delete(s.openSeq, cameraID)
		if tr := s.trackers[cameraID]; tr != nil {
			tr.Close()
		}
	}
	freed, err := st.RemoveEvent(ctx, id)
	s.trackMu.Unlock()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storageBytes -= freed
	if s.storageBytes < 0 {
		s.storageBytes = 0
	}
	for cameraID, last := range s.lastEvents {
		if last.id == id {
			delete(s.lastEvents, cameraID)
		}
	}
	for _, rt := range s.cameras {
		if rt.lastEventID == id {
			rt.lastEventID, rt.lastEventAt = 0, time.Time{}
		}
	}
	return nil
}

// Test analyzes one current frame of a camera with threshold TestMinScore and marks
// every box with the camera's verdict. override (zones, threshold) replaces the saved
// detection settings; without it the camera must be detecting. Test needs only the
// detector, not the database: ErrDisabled while detection is off, ErrNotReady
// while the detector is not ready.
func (s *Service) Test(ctx context.Context, cameraID string, override *Camera) (TestResult, error) {
	s.mu.Lock()
	running := s.runningLocked()
	det := s.det
	var cam Camera
	found := false
	if override != nil {
		cam, found = normalizeCamera(*override), true
		cam.ID = cameraID
	} else if rt := s.cameras[cameraID]; rt != nil {
		cam, found = rt.cam, true
	}
	s.mu.Unlock()
	if !running {
		return TestResult{}, ErrDisabled
	}
	if det == nil {
		return TestResult{}, ErrNotReady
	}
	if !found {
		return TestResult{}, fmt.Errorf("%w: %s", ErrUnknownCamera, cameraID)
	}
	if s.frames == nil {
		return TestResult{}, fmt.Errorf("%w: no frame source", ErrFrameUnavailable)
	}
	fetchCtx, cancel := context.WithTimeout(ctx, FrameFetchTimeout)
	data, err := s.frames.Frame(fetchCtx, cameraID)
	cancel()
	if err != nil {
		return TestResult{}, fmt.Errorf("%w: %v", ErrFrameUnavailable, err)
	}
	s.detectMu.Lock()
	res, err := det.Detect(ctx, data, TestMinScore)
	s.detectMu.Unlock()
	if err != nil {
		switch classifyDetectError(ctx, err) {
		case detectFailDetector:
			s.detectorFailed(det, err)
			return TestResult{}, fmt.Errorf("%w: %v", ErrNotReady, err)
		case detectSkipFrame:
			if ctxErr := ctx.Err(); ctxErr != nil {
				return TestResult{}, ctxErr
			}
			return TestResult{}, fmt.Errorf("%w: %v", ErrNotReady, err)
		default:
			return TestResult{}, fmt.Errorf("%w: %v", ErrFrameUnavailable, err)
		}
	}
	boxes := make([]TestBox, 0, len(res.Boxes))
	for _, b := range res.Boxes {
		counts, zones := BoxCounts(b, cam.MinScore, cam.Zones)
		if zones == nil {
			zones = []string{}
		}
		boxes = append(boxes, TestBox{Box: b, Counts: counts, Zones: zones})
	}
	width, height := res.Width, res.Height
	if width <= 0 || height <= 0 {
		if cfg, err := jpeg.DecodeConfig(bytes.NewReader(data)); err == nil {
			width, height = cfg.Width, cfg.Height
		}
	}
	return TestResult{
		Image:   "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data),
		Width:   width,
		Height:  height,
		Backend: det.Info().Active,
		WallMS:  res.WallMS,
		Boxes:   boxes,
	}, nil
}
