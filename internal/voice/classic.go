package voice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ClassicBackend struct {
	Recognizer       SpeechRecognizer
	Synthesizer      SpeechSynthesizer
	Runner           VoiceActionRunner
	MaxDuration      time.Duration
	IdleTimeout      time.Duration
	AgentProviderID  string
	AdditionalPrompt string
	Greeting         string
	FailureMessage   string
	GoodbyeMessage   string
}

func (b *ClassicBackend) Start(ctx context.Context, call CallContext, audio DuplexAudio) (VoiceSession, error) {
	if b.Recognizer == nil || b.Synthesizer == nil || b.Runner == nil {
		return nil, fmt.Errorf("classic voice backend dependencies are incomplete")
	}
	if b.MaxDuration <= 0 {
		b.MaxDuration = time.Hour
	}
	if b.IdleTimeout <= 0 {
		b.IdleTimeout = 2 * time.Minute
	}
	call.AgentProviderID = b.AgentProviderID
	call.AdditionalPrompt = b.AdditionalPrompt
	call.Greeting = b.Greeting
	call.FailureMessage = b.FailureMessage
	call.GoodbyeMessage = b.GoodbyeMessage
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
	idleTimer := time.NewTimer(s.backend.IdleTimeout)
	defer idleTimer.Stop()
	if s.call.Greeting != "" {
		if err := s.speakText(s.ctx, s.call.Greeting, "greeting"); err != nil && s.ctx.Err() == nil {
			s.fail("greeting", false)
			return
		}
	}
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
			if s.detector.TakeOverflow() {
				s.emitData("audio_queue_dropped", map[string]any{"stage": "vad", "policy": "drop_oldest"})
			}
			if started {
				resetVoiceIdleTimer(idleTimer, s.backend.IdleTimeout)
				s.Interrupt()
				s.emit("barge_in", "")
			}
			if len(utterance) > 0 {
				resetVoiceIdleTimer(idleTimer, s.backend.IdleTimeout)
				go s.handleUtterance(utterance, s.inputRate)
			}
		case <-idleTimer.C:
			if s.call.GoodbyeMessage != "" {
				_ = s.speakText(s.ctx, s.call.GoodbyeMessage, "goodbye")
			}
			s.emit("inactivity_timeout", "")
			s.backend.Runner.EndVoiceCall(s.call.CallID)
			return
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
		s.fail("resampling", true)
		return
	}
	wav, err := EncodeWAVPCM16(resampler.Process(samples), 16000)
	if err != nil {
		s.fail("wav_encoding", true)
		return
	}
	text, err := s.backend.Recognizer.Recognize(turnCtx, wav, 16000, s.call.Language)
	if err != nil {
		if turnCtx.Err() == nil {
			s.fail("asr", true)
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
			s.fail("agent", true)
		}
		return
	}
	response = strings.TrimSpace(response)
	if response == "" {
		return
	}

	if err := s.speakText(turnCtx, response, "response"); err != nil {
		if turnCtx.Err() == nil {
			s.fail("tts", false)
		}
		return
	}
	s.emit("turn_complete", "")
}

func (s *classicSession) speakText(ctx context.Context, text, kind string) error {
	pcm, rate, err := s.backend.Synthesizer.Synthesize(ctx, text, s.call.Language)
	if err != nil {
		return err
	}
	s.emitData("transcript", map[string]any{"direction": "output", "text": text, "kind": kind})
	toTelephone, err := NewSourceResampler(rate, 8000)
	if err != nil {
		return err
	}
	telephonePCM := toTelephone.Process(pcm)
	frameSamples := 160
	for offset := 0; offset < len(telephonePCM); offset += frameSamples {
		end := min(offset+frameSamples, len(telephonePCM))
		frame := PCMFrame{Samples: telephonePCM[offset:end], SampleRate: 8000}
		if err := s.audio.Send(ctx, frame); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.framePeriod):
		}
	}
	return nil
}

func (s *classicSession) fail(stage string, announce bool) {
	s.emitData("voice_backend_error", map[string]any{"stage": stage})
	if announce && s.call.FailureMessage != "" {
		ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
		_ = s.speakText(ctx, s.call.FailureMessage, "failure")
		cancel()
	}
	s.backend.Runner.EndVoiceCall(s.call.CallID)
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

func resetVoiceIdleTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}
