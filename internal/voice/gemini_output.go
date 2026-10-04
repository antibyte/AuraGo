package voice

import "time"

// queueOutput retains a bounded burst without dropping its beginning. The
// reader remains available for interruption and tool/control messages.
func (s *geminiLiveSession) queueOutput(samples []int16) bool {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	if s.ctx.Err() != nil {
		return true
	}
	frames := (len(samples) + 159) / 160
	if frames > cap(s.output)-len(s.output) {
		return false
	}
	for offset := 0; offset < len(samples); offset += 160 {
		s.output <- PCMFrame{Samples: samples[offset:min(offset+160, len(samples))], SampleRate: 8000}
	}
	return true
}

func (s *geminiLiveSession) outputLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
		s.outputMu.Lock()
		select {
		case frame := <-s.output:
			if s.ctx.Err() == nil {
				if err := s.audio.Send(s.ctx, frame); err != nil && s.ctx.Err() == nil {
					s.outputMu.Unlock()
					s.fail("Gemini audio playback failed", false)
					return
				}
			}
		default:
		}
		s.outputMu.Unlock()
	}
}

func (s *geminiLiveSession) flushOutput() {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	for {
		select {
		case <-s.output:
		default:
			s.audio.FlushOutput()
			return
		}
	}
}
