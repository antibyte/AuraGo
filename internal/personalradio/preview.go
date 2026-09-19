package personalradio

import (
	"context"
	"time"
)

// Preview uses the same provider adapter and durable TTS allowance as airtime.
func (s *Service) Preview(ctx context.Context, id, text string) (Audio, error) {
	s.mu.Lock()
	if s.closed || s.editorActive || s.state.Status != "stopped" || s.adapters.Speak == nil || s.now().Before(s.editorRetry) {
		s.mu.Unlock()
		return Audio{}, ErrConflict
	}
	p, err := s.station(id)
	if err != nil {
		s.mu.Unlock()
		return Audio{}, err
	}
	job, err := s.reserve(id, "tts_chars", len([]rune(text)), p.DailyTTSChars)
	if err != nil {
		s.mu.Unlock()
		return Audio{}, err
	}
	s.editorActive = true
	s.editorRetry = s.now().Add(10 * time.Second)
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	audio, err := s.adapters.Speak(ctx, p, text)
	s.mu.Lock()
	s.editorActive = false
	s.finishJob(job, err, "")
	s.mu.Unlock()
	return audio, err
}
