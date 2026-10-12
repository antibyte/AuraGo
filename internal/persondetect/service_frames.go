package persondetect

import (
	"context"
	"errors"
	"time"
)

// maxCameraErrorBytes bounds CameraStatus.LastError (see truncateText).
const maxCameraErrorBytes = 300

// cameraLoop requests frames of one camera until its context ends. The first
// request waits first (cameraStartDelay), so cameras start staggered.
func (s *Service) cameraLoop(ctx context.Context, cameraID string, first time.Duration) {
	defer s.wg.Done()
	timer := time.NewTimer(first)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		timer.Reset(s.fetchOnce(ctx, cameraID))
	}
}

// fetchOnce requests one frame and queues it as the camera's newest frame. It
// returns the delay until the next request: the interval, or the backoff after
// failures (FetchBackoff). Nothing is requested while the detector is not ready
// (which includes the time detections.db is being opened).
//
// Every failed request counts, whatever the cause. A stopped go2rtc sidecar makes
// all of them fail: the cameras then show unreachable and back off until go2rtc
// runs again. That is the expected state, not a detector fault.
func (s *Service) fetchOnce(ctx context.Context, cameraID string) time.Duration {
	s.mu.Lock()
	rt, ok := s.cameras[cameraID]
	ready := s.det != nil && s.store != nil
	var cam Camera
	if ok {
		cam = rt.cam
		if !ready {
			rt.state, rt.lastError = CameraWaiting, ""
		}
	}
	s.mu.Unlock()
	if !ok {
		return DefaultCameraInterval
	}
	if !ready {
		return cam.Interval
	}
	var data []byte
	err := errors.New("no frame source")
	if s.frames != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, FrameFetchTimeout)
		data, err = s.frames.Frame(fetchCtx, cameraID)
		cancel()
	}
	if ctx.Err() != nil {
		return cam.Interval
	}
	if err != nil {
		return FetchBackoff(cam.Interval, s.recordFetchFailure(cameraID, err))
	}
	s.recordFetchSuccess(cameraID)
	if s.slots.put(frameJob{cameraID: cameraID, jpeg: data, at: s.now()}) {
		pokeChannel(s.wake)
	}
	return cam.Interval
}

func (s *Service) recordFetchFailure(cameraID string, err error) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := s.cameras[cameraID]
	if rt == nil {
		return 1
	}
	rt.failures++
	if rt.failures == 1 {
		s.logger.Warn("[PersonDetect] Camera frame unavailable", "camera", cameraID, "error", err)
	}
	rt.state, rt.lastError = CameraUnreachable, truncateText(err.Error(), maxCameraErrorBytes)
	return rt.failures
}

func (s *Service) recordFetchSuccess(cameraID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt := s.cameras[cameraID]
	if rt == nil {
		return
	}
	if rt.failures > 0 || rt.state == CameraUnreachable {
		rt.state, rt.lastError = CameraWaiting, ""
	}
	rt.failures = 0
}

func (s *Service) setCameraState(cameraID, state, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rt := s.cameras[cameraID]; rt != nil {
		rt.state, rt.lastError = state, truncateText(message, maxCameraErrorBytes)
	}
}
