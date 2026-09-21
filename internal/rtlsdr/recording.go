package rtlsdr

import (
	"context"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

func (s *Service) Record(r Recording) (Recording, error) {
	r.ID = ""
	r.ScheduleID = ""
	r.Start = s.opts.Now()
	return s.startRecording(r)
}
func (s *Service) startRecording(r Recording) (Recording, error) {
	if err := s.writable(); err != nil {
		return r, err
	}
	if r.Agent && !s.policy().AllowAgent {
		return r, ErrDisabled
	}
	if err := r.Tuning.Validate(); err != nil {
		return r, err
	}
	if r.Duration == 0 {
		r.Duration = 600
	}
	if r.Duration < 5 || r.Duration > int(MaxDuration/time.Second) || len(r.Name) > 120 || (r.Tuning.Mode == "dab" && r.Tuning.ServiceID == "") {
		return r, ErrInvalid
	}
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writable(); err != nil {
		return r, err
	}
	if r.Agent && !s.policy().AllowAgent {
		return r, ErrDisabled
	}
	if s.ctx.Err() != nil {
		return r, ErrUnavailable
	}
	if s.active != "" || s.scanCancel != nil {
		return r, ErrBusy
	}
	if s.usedLocked()+int64(r.Duration)*200000 > s.policy().QuotaBytes {
		return r, ErrQuota
	}
	previous := copyState(s.state)
	if r.ScheduleID == "" {
		for _, v := range s.state.Schedules {
			next := v.Next(r.Start.Add(-time.Second))
			if v.Enabled && !next.IsZero() && next.Add(-Warmup).Before(r.Start.Add(time.Duration(r.Duration)*time.Second)) {
				return r, ErrBusy
			}
		}
	} else {
		found := false
		for i := range s.state.Schedules {
			v := &s.state.Schedules[i]
			if v.ID == r.ScheduleID && v.Enabled && v.LastSlot.Before(r.Start) {
				v.LastSlot = r.Start
				found = true
				break
			}
		}
		if !found {
			return r, ErrNotFound
		}
	}
	r.ID = newID()
	r.Created = s.opts.Now()
	r.Status = "preparing"
	r.Error = ""
	r.Segments = []Segment{}
	r.Bytes = 0
	r.Seconds = 0
	r.ASRError = ""
	s.state.Recordings = append(s.state.Recordings, r)
	if err := s.saveLocked(); err != nil {
		s.state = previous
		return r, err
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.active = r.ID
	s.recordCancel = cancel
	s.manualStop = false
	s.deviceLost = false
	s.finishing[r.ID] = true
	s.wg.Add(1)
	go s.capture(ctx, cancel, r)
	return r, nil
}
func (s *Service) updateRecording(id string, fn func(*Recording)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Recordings {
		if s.state.Recordings[i].ID == id {
			previous := s.state.Recordings[i]
			previous.Segments = append([]Segment{}, previous.Segments...)
			fn(&s.state.Recordings[i])
			err := s.saveLocked()
			if err != nil {
				s.state.Recordings[i] = previous
			}
			return err
		}
	}
	return ErrNotFound
}
func (s *Service) StopRecording(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != id {
		return ErrNotFound
	}
	s.manualStop = true
	s.recordCancel()
	return nil
}

type quotaWriter struct {
	file     *os.File
	n, limit int64
}

func (w *quotaWriter) Write(p []byte) (int, error) {
	if w.n+int64(len(p)) > w.limit {
		return 0, ErrQuota
	}
	n, err := w.file.Write(p)
	w.n += int64(n)
	return n, err
}

func (s *Service) capture(ctx context.Context, cancel context.CancelFunc, r Recording) {
	defer s.wg.Done()
	defer cancel()
	defer func() { s.mu.Lock(); delete(s.finishing, r.ID); s.mu.Unlock() }()
	var recognizer Transcriber
	var asrErr error
	if r.Transcribe {
		if s.opts.NewTranscriber != nil {
			recognizer, asrErr = s.opts.NewTranscriber(ctx)
		} else {
			asrErr = ErrUnavailable
		}
	}
	var err error
	s.deviceMu.Lock()
	err = s.writable()
	if err == nil && r.Agent && !s.policy().AllowAgent {
		err = ErrDisabled
	}
	if err == nil {
		err = s.opts.Backend.Tune(ctx, r.Tuning)
	}
	s.deviceMu.Unlock()
	if err == nil && r.ScheduleID != "" && s.opts.Now().After(r.Start.Add(3*time.Second)) {
		err = errors.New("sdr_late_tuning")
	}
	if err == nil {
		delay := r.Start.Sub(s.opts.Now())
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				err = ctx.Err()
			}
			timer.Stop()
		}
	}
	var info CaptureInfo
	var size int64
	if err == nil {
		err = s.writable()
		if err == nil {
			err = s.updateRecording(r.ID, func(v *Recording) { v.Status = "recording" })
		}
		if err == nil {
			var file *os.File
			file, err = os.OpenFile(s.audioPath(r.ID)+".part", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err == nil {
				s.mu.Lock()
				remaining := s.policy().QuotaBytes - s.usedLocked()
				s.mu.Unlock()
				writer := &quotaWriter{file: file, limit: remaining}
				captureCtx, done := context.WithTimeout(ctx, time.Duration(r.Duration)*time.Second+15*time.Second)
				info, err = s.opts.Backend.Capture(captureCtx, r.Duration, writer)
				done()
				size = writer.n
				if e := file.Sync(); err == nil {
					err = e
				}
				if e := file.Close(); err == nil {
					err = e
				}
				if size > 4 {
					if e := os.Rename(s.audioPath(r.ID)+".part", s.audioPath(r.ID)); e != nil {
						err = e
						size = 0
					}
				} else {
					_ = os.Remove(s.audioPath(r.ID) + ".part")
					size = 0
				}
			}
		}
	}
	s.mu.Lock()
	manual := s.manualStop
	lost := s.deviceLost
	s.mu.Unlock()
	_ = s.updateRecording(r.ID, func(v *Recording) { v.Bytes = size; v.Seconds = info.Seconds })
	// Probing partial audio and running ASR must not retain the receiver lease.
	s.releaseReceiver()
	if size > 4 && info.Seconds <= 0 {
		if probe, ok := s.opts.Backend.(interface {
			AudioSeconds(context.Context, string) (float64, error)
		}); ok {
			probeCtx, done := context.WithTimeout(s.ctx, 2*time.Minute)
			info.Seconds, _ = probe.AudioSeconds(probeCtx, r.ID)
			done()
		}
	}
	status, code := "complete", ""
	if err != nil {
		status = "failed"
		code = "sdr_capture_failed"
		if size > 4 {
			status = "partial"
		}
		if manual {
			code = "sdr_stopped"
		}
		if lost {
			code = "sdr_device_lost"
			info.Gaps++
		}
		if errors.Is(err, ErrQuota) {
			code = "sdr_quota_exceeded"
		}
	}
	if info.Gaps > 0 && status == "complete" {
		status = "partial"
		code = "sdr_signal_gaps"
	}
	_ = s.updateRecording(r.ID, func(v *Recording) {
		v.Status = status
		v.Error = code
		v.Bytes = size
		v.Seconds = info.Seconds
		v.Gaps = info.Gaps
		if asrErr != nil {
			v.ASRError = "sdr_asr_unavailable"
		}
	})
	if s.opts.Issue != nil && !manual && (ctx.Err() == nil || lost) {
		s.opts.Issue("capture", err == nil && info.Gaps == 0)
	}
	r, _ = s.Recording(r.ID)
	if size > 4 && s.opts.Register != nil {
		if e := s.opts.Register(s.ctx, r, s.audioPath(r.ID)); e != nil && s.opts.Issue != nil {
			s.opts.Issue("media_registration", false)
		}
	}
	if r.Transcribe && size > 4 && recognizer != nil && s.writable() == nil {
		asrCtx, asrCancel := context.WithCancel(s.ctx)
		defer asrCancel()
		s.mu.Lock()
		s.transcribing[r.ID] = asrCancel
		s.asrAgents[r.ID] = r.Agent
		s.mu.Unlock()
		s.transcribe(asrCtx, r.ID, recognizer, status, r.Agent)
	}
}

func (s *Service) releaseReceiver() {
	s.deviceMu.Lock()
	s.mu.Lock()
	s.active = ""
	s.recordCancel = nil
	resume := len(s.listeners) > 0
	t := s.state.Tuning
	s.mu.Unlock()
	restoreCtx, restoreCancel := context.WithTimeout(s.ctx, 15*time.Second)
	if s.writable() == nil && resume {
		_ = s.opts.Backend.Tune(restoreCtx, t)
	} else {
		_ = s.opts.Backend.Stop(restoreCtx)
	}
	restoreCancel()
	s.deviceMu.Unlock()
}

func (s *Service) RetryTranscription(ctx context.Context, id string) error {
	return s.RetryTranscriptionAs(ctx, id, false)
}
func (s *Service) RetryTranscriptionAs(ctx context.Context, id string, agent bool) error {
	if err := s.writable(); err != nil {
		return err
	}
	r, err := s.Recording(id)
	if err != nil {
		return err
	}
	agent = agent || r.Agent
	if agent && !s.policy().AllowAgent {
		return ErrDisabled
	}
	if r.Bytes == 0 {
		return ErrInvalid
	}
	s.mu.Lock()
	if s.active == id || s.transcribing[id] != nil || s.finishing[id] {
		s.mu.Unlock()
		return ErrBusy
	}
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return ErrUnavailable
	}
	s.wg.Add(1)
	work, cancel := context.WithCancel(s.ctx)
	s.transcribing[id] = cancel
	s.asrAgents[id] = agent
	s.mu.Unlock()
	if s.opts.NewTranscriber == nil {
		s.wg.Done()
		cancel()
		s.mu.Lock()
		delete(s.transcribing, id)
		delete(s.asrAgents, id)
		s.mu.Unlock()
		return ErrUnavailable
	}
	tr, err := s.opts.NewTranscriber(work)
	if err != nil {
		s.wg.Done()
		cancel()
		s.mu.Lock()
		delete(s.transcribing, id)
		delete(s.asrAgents, id)
		s.mu.Unlock()
		return err
	}
	go func() { defer s.wg.Done(); defer cancel(); s.transcribe(work, id, tr, r.Status, agent) }()
	return nil
}

func (s *Service) transcribe(ctx context.Context, id string, tr Transcriber, previous string, agent bool) {
	s.asrMu.Lock()
	defer s.asrMu.Unlock()
	defer func() { s.mu.Lock(); delete(s.transcribing, id); delete(s.asrAgents, id); s.mu.Unlock() }()
	if previous == "transcribing" {
		previous = "complete"
	}
	_ = s.updateRecording(id, func(v *Recording) { v.Status = "transcribing"; v.ASRError = "" })
	r, _ := s.Recording(id)
	failed := false
	duration := r.Seconds
	if duration <= 0 {
		if probe, ok := s.opts.Backend.(interface {
			AudioSeconds(context.Context, string) (float64, error)
		}); ok {
			duration, _ = probe.AudioSeconds(ctx, id)
		}
		if duration <= 0 {
			failed = true
		} else {
			_ = s.updateRecording(id, func(v *Recording) { v.Seconds = duration })
		}
	}
	for offset := 0.0; offset < duration; offset += 60 {
		if err := s.writable(); err != nil || ctx.Err() != nil || (agent && !s.policy().AllowAgent) {
			failed = true
			break
		}
		cached := false
		for _, seg := range r.Segments {
			if seg.Start == offset && seg.Error == "" {
				cached = true
				break
			}
		}
		if cached {
			continue
		}
		seconds := 60
		if duration-offset < 60 {
			seconds = int(duration - offset + 0.999)
		}
		wav, err := s.opts.Backend.WAV(ctx, id, offset, seconds)
		text := ""
		if err == nil {
			segmentCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			text, err = tr.Transcribe(segmentCtx, wav)
			cancel()
		}
		segment := Segment{Start: offset, End: offset + float64(seconds), Text: strings.TrimSpace(text)}
		segment.End = min(segment.End, duration)
		if len(segment.Text) > 24000 {
			segment.Text = ""
			err = ErrInvalid
		}
		if err != nil {
			segment.Error = "sdr_asr_failed"
			failed = true
		}
		_ = s.updateRecording(id, func(v *Recording) {
			for i, old := range v.Segments {
				if old.Start == offset {
					v.Segments = append(v.Segments[:i], v.Segments[i+1:]...)
					break
				}
			}
			v.Segments = append(v.Segments, segment)
			sort.Slice(v.Segments, func(i, j int) bool { return v.Segments[i].Start < v.Segments[j].Start })
		})
		if errors.Is(err, io.EOF) {
			break
		}
	}
	_ = s.updateRecording(id, func(v *Recording) {
		v.Status = previous
		if failed {
			v.ASRError = "sdr_asr_failed"
		}
	})
	if s.opts.Issue != nil {
		s.opts.Issue("transcription", !failed)
	}
}
