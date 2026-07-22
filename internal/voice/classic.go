package voice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ClassicBackend struct {
	Recognizer  SpeechRecognizer
	Synthesizer SpeechSynthesizer
	Runner      VoiceActionRunner
	MaxDuration time.Duration
}

func (b *ClassicBackend) Start(ctx context.Context, call CallContext, audio DuplexAudio) (VoiceSession, error) {
	if b.Recognizer == nil || b.Synthesizer == nil || b.Runner == nil {
		return nil, fmt.Errorf("classic voice backend dependencies are incomplete")
	}
	if b.MaxDuration <= 0 {
		b.MaxDuration = time.Hour
	}
	sessionCtx, cancel := context.WithTimeout(ctx, b.MaxDuration)
	session := &classicSession{
		ctx:         sessionCtx,
		cancel:      cancel,
		call:        call,
		audio:       audio,
		backend:     b,
		events:      make(chan VoiceEvent, 32),
		detector:    NewTurnDetector(20, 120, 600, 200),
		inputRate:   8000,
		framePeriod: 20 * time.Millisecond,
	}
	go session.run()
	return session, nil
}

type classicSession struct {
	ctx         context.Context
	cancel      context.CancelFunc
	call        CallContext
	audio       DuplexAudio
	backend     *ClassicBackend
	events      chan VoiceEvent
	detector    *TurnDetector
	inputRate   int
	framePeriod time.Duration
	mu          sync.Mutex
	turnCancel  context.CancelFunc
	closed      bool
}

func (s *classicSession) run() {
	defer close(s.events)
	s.emit("backend_started", "")
	for {
		select {
		case <-s.ctx.Done():
			s.Interrupt()
			s.emit("backend_stopped", "")
			return
		case frame := <-s.audio.Receive():
			if frame.SampleRate != 0 {
				s.inputRate = frame.SampleRate
			}
			started, utterance := s.detector.Push(frame.Samples)
			if started {
				s.Interrupt()
				s.emit("barge_in", "")
			}
			if len(utterance) > 0 {
				go s.handleUtterance(utterance, s.inputRate)
			}
		}
	}
}

func (s *classicSession) handleUtterance(samples []int16, sampleRate int) {
	turnCtx, turnCancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		turnCancel()
		return
	}
	if s.turnCancel != nil {
		s.turnCancel()
	}
	s.turnCancel = turnCancel
	s.mu.Unlock()
	defer turnCancel()

	resampler, err := NewResampler(sampleRate, 16000)
	if err != nil {
		s.emitData("voice_backend_error", map[string]any{"stage": "resampling"})
		return
	}
	wav, err := EncodeWAVPCM16(resampler.Process(samples), 16000)
	if err != nil {
		s.emitData("voice_backend_error", map[string]any{"stage": "wav_encoding"})
		return
	}
	text, err := s.backend.Recognizer.Recognize(turnCtx, wav, 16000, s.call.Language)
	if err != nil {
		if turnCtx.Err() == nil {
			s.emitData("voice_backend_error", map[string]any{"stage": "asr"})
		}
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.emitData("transcript", map[string]any{"direction": "input", "text": text})

	response, err := s.backend.Runner.RunVoiceTurn(turnCtx, s.call, text)
	if err != nil {
		if turnCtx.Err() == nil {
			s.emitData("voice_backend_error", map[string]any{"stage": "agent"})
		}
		return
	}
	response = strings.TrimSpace(response)
	if response == "" {
		return
	}
	s.emitData("transcript", map[string]any{"direction": "output", "text": response})

	pcm, rate, err := s.backend.Synthesizer.Synthesize(turnCtx, response, s.call.Language)
	if err != nil {
		if turnCtx.Err() == nil {
			s.emitData("voice_backend_error", map[string]any{"stage": "tts"})
		}
		return
	}
	toTelephone, err := NewResampler(rate, 8000)
	if err != nil {
		s.emitData("voice_backend_error", map[string]any{"stage": "resampling"})
		return
	}
	telephonePCM := toTelephone.Process(pcm)
	frameSamples := 160
	for offset := 0; offset < len(telephonePCM); offset += frameSamples {
		end := min(offset+frameSamples, len(telephonePCM))
		frame := PCMFrame{Samples: telephonePCM[offset:end], SampleRate: 8000}
		if err := s.audio.Send(turnCtx, frame); err != nil {
			return
		}
		select {
		case <-turnCtx.Done():
			return
		case <-time.After(s.framePeriod):
		}
	}
	s.emit("turn_complete", "")
}

func (s *classicSession) Interrupt() {
	s.mu.Lock()
	cancel := s.turnCancel
	s.turnCancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.backend.Runner.CancelVoiceTurn(s.call.CallID)
	}
	s.audio.FlushOutput()
}

func (s *classicSession) Events() <-chan VoiceEvent { return s.events }

func (s *classicSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	return nil
}

func (s *classicSession) emit(eventType, message string) {
	s.emitDataMessage(eventType, message, nil)
}

func (s *classicSession) emitData(eventType string, data map[string]any) {
	s.emitDataMessage(eventType, "", data)
}

func (s *classicSession) emitDataMessage(eventType, message string, data map[string]any) {
	event := VoiceEvent{Type: eventType, Message: message, Data: data, Timestamp: time.Now().UTC()}
	select {
	case s.events <- event:
	default:
	}
}
