package voice

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBridgeDropsOldestFrame(t *testing.T) {
	bridge := NewBridge(1)
	if err := bridge.Send(context.Background(), PCMFrame{Samples: []int16{1}, SampleRate: 8000}); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Send(context.Background(), PCMFrame{Samples: []int16{2}, SampleRate: 8000}); err != nil {
		t.Fatal(err)
	}
	frame, err := bridge.NextSend(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := frame.Samples[0]; got != 2 {
		t.Fatalf("expected newest frame, got %d", got)
	}
	select {
	case event := <-bridge.Events():
		if event.Type != "output_queue_overrun" {
			t.Fatalf("unexpected event %q", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("expected overrun event")
	}
}

type testRecognizer struct{ text string }

func (r testRecognizer) Recognize(_ context.Context, wav []byte, rate int, _ string) (string, error) {
	if len(wav) < 44 || rate != 16000 {
		return "", context.DeadlineExceeded
	}
	return r.text, nil
}

type testSynthesizer struct{}

func (testSynthesizer) Synthesize(context.Context, string, string) ([]int16, int, error) {
	return make([]int16, 320), 16000, nil
}

func loudPCMFrame(samples int) []int16 {
	frame := make([]int16, samples)
	for index := range frame {
		frame[index] = 2000
	}
	return frame
}

type testVoiceRunner struct {
	cancelled    atomic.Int32
	ended        atomic.Int32
	internalEnds atomic.Int32
	internalWhy  atomic.Value
	response     string
}

func (r *testVoiceRunner) RunVoiceTurn(context.Context, CallContext, string) (string, error) {
	if r.response != "" {
		return r.response, nil
	}
	return "Antwort", nil
}
func (r *testVoiceRunner) CancelVoiceTurn(string) { r.cancelled.Add(1) }
func (r *testVoiceRunner) EndVoiceCall(string)    { r.ended.Add(1) }
func (r *testVoiceRunner) EndVoiceCallInternal(_ string, reason string) {
	r.internalWhy.Store(reason)
	r.internalEnds.Add(1)
}

type recordingSynthesizer struct {
	texts chan string
	err   error
}

type stagedStreamingSynthesizer struct {
	secondStarted chan struct{}
	releaseSecond chan struct{}
	done          chan struct{}
}

func (*stagedStreamingSynthesizer) Synthesize(context.Context, string, string) ([]int16, int, error) {
	return nil, 0, errors.New("non-streaming synthesis must not be used")
}

func (s *stagedStreamingSynthesizer) SynthesizeStream(ctx context.Context, _, _ string) <-chan SpeechSynthesisChunk {
	chunks := make(chan SpeechSynthesisChunk, 1)
	go func() {
		defer close(chunks)
		defer close(s.done)
		select {
		case chunks <- SpeechSynthesisChunk{Samples: make([]int16, 320), SampleRate: 8000}:
		case <-ctx.Done():
			return
		}
		close(s.secondStarted)
		select {
		case <-s.releaseSecond:
		case <-ctx.Done():
			return
		}
		select {
		case chunks <- SpeechSynthesisChunk{Samples: make([]int16, 160), SampleRate: 8000}:
		case <-ctx.Done():
		}
	}()
	return chunks
}

type scriptedStreamingSynthesizer struct {
	chunks []SpeechSynthesisChunk
}

func (*scriptedStreamingSynthesizer) Synthesize(context.Context, string, string) ([]int16, int, error) {
	return nil, 0, errors.New("non-streaming synthesis must not be used")
}

func (s *scriptedStreamingSynthesizer) SynthesizeStream(ctx context.Context, _, _ string) <-chan SpeechSynthesisChunk {
	chunks := make(chan SpeechSynthesisChunk, 1)
	go func() {
		defer close(chunks)
		for _, chunk := range s.chunks {
			select {
			case chunks <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()
	return chunks
}

func (s recordingSynthesizer) Synthesize(_ context.Context, text, _ string) ([]int16, int, error) {
	if s.texts != nil {
		s.texts <- text
	}
	if s.err != nil {
		return nil, 0, s.err
	}
	return make([]int16, 160), 8000, nil
}

type failingRecognizer struct{}

func (failingRecognizer) Recognize(context.Context, []byte, int, string) (string, error) {
	return "", errors.New("recognizer unavailable")
}

func TestClassicBackendASRAgentTTSPipeline(t *testing.T) {
	runner := &testVoiceRunner{}
	backend := &ClassicBackend{Recognizer: testRecognizer{text: "Hallo"}, Synthesizer: testSynthesizer{}, Runner: runner}
	bridge := NewBridge(4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &classicSession{
		ctx: ctx, cancel: cancel, call: CallContext{CallID: "call-1", Language: "de"}, audio: bridge,
		backend: backend, events: make(chan VoiceEvent, 8), framePeriod: time.Millisecond,
	}
	session.handleUtterance(make([]int16, 160), 8000)
	frame, err := bridge.NextSend(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if frame.SampleRate != 8000 || len(frame.Samples) == 0 {
		t.Fatalf("unexpected telephone frame: rate=%d samples=%d", frame.SampleRate, len(frame.Samples))
	}
}

func TestClassicSpeechStreamingStartsPlaybackBeforeNextBlockCompletes(t *testing.T) {
	synthesizer := &stagedStreamingSynthesizer{
		secondStarted: make(chan struct{}), releaseSecond: make(chan struct{}), done: make(chan struct{}),
	}
	bridge := NewBridge(8)
	session := &classicSession{
		call: CallContext{Language: "de"}, audio: bridge,
		backend: &ClassicBackend{Synthesizer: synthesizer}, events: make(chan VoiceEvent, 4), framePeriod: time.Millisecond,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- session.speakText(context.Background(), "Erster Satz. Zweiter Satz.", "response") }()
	select {
	case <-synthesizer.secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second synthesis did not start")
	}
	readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
	frame, err := bridge.NextSend(readCtx)
	cancelRead()
	if err != nil || len(frame.Samples) == 0 {
		t.Fatalf("first audio was not played while second synthesis was pending: frame=%+v err=%v", frame, err)
	}
	close(synthesizer.releaseSecond)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("streaming playback did not complete")
	}
	select {
	case <-synthesizer.done:
	case <-time.After(time.Second):
		t.Fatal("streaming synthesis goroutine leaked")
	}
}

func TestClassicSpeechStreamingFailsClosedOnRateChangeAndLaterError(t *testing.T) {
	tests := []struct {
		name   string
		chunks []SpeechSynthesisChunk
		want   string
	}{
		{
			name: "sample rate change",
			chunks: []SpeechSynthesisChunk{
				{Samples: make([]int16, 160), SampleRate: 8000},
				{Samples: make([]int16, 160), SampleRate: 16000},
			},
			want: "sample rate changed",
		},
		{
			name: "later provider failure",
			chunks: []SpeechSynthesisChunk{
				{Samples: make([]int16, 160), SampleRate: 8000},
				{Err: errors.New("provider failed on block two")},
			},
			want: "provider failed on block two",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := NewBridge(4)
			session := &classicSession{
				call: CallContext{Language: "de"}, audio: bridge,
				backend: &ClassicBackend{Synthesizer: &scriptedStreamingSynthesizer{chunks: tt.chunks}},
				events:  make(chan VoiceEvent, 4), framePeriod: time.Millisecond,
			}
			err := session.speakText(context.Background(), "Zwei Blöcke", "response")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
			frame, readErr := bridge.NextSend(readCtx)
			cancelRead()
			if readErr != nil || len(frame.Samples) == 0 {
				t.Fatalf("audio completed before the later failure was lost: frame=%+v err=%v", frame, readErr)
			}
		})
	}
}

func TestClassicSpeechStreamingBargeInCancelsAndFlushesOutput(t *testing.T) {
	bridge := NewBridge(8)
	runner := &testVoiceRunner{}
	synthesizer := &scriptedStreamingSynthesizer{chunks: []SpeechSynthesisChunk{{Samples: make([]int16, 1600), SampleRate: 8000}}}
	turnCtx, turnCancel := context.WithCancel(context.Background())
	session := &classicSession{
		call: CallContext{CallID: "stream-barge-in", Language: "de"}, audio: bridge,
		backend: &ClassicBackend{Synthesizer: synthesizer, Runner: runner},
		events:  make(chan VoiceEvent, 4), framePeriod: 10 * time.Millisecond, turnCancel: turnCancel,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- session.speakText(turnCtx, "Lange Antwort", "response") }()
	readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
	if _, err := bridge.NextSend(readCtx); err != nil {
		cancelRead()
		t.Fatalf("stream did not begin playback: %v", err)
	}
	cancelRead()
	session.Interrupt()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("barge-in error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("barge-in did not stop streaming playback")
	}
	if runner.cancelled.Load() != 1 {
		t.Fatalf("voice runner cancel count = %d", runner.cancelled.Load())
	}
	emptyCtx, cancelEmpty := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelEmpty()
	if frame, err := bridge.NextSend(emptyCtx); err == nil {
		t.Fatalf("audio remained queued after barge-in: %+v", frame)
	}
}

func TestClassicBackendUsesSnapshotAndConfiguredGreeting(t *testing.T) {
	texts := make(chan string, 2)
	runner := &testVoiceRunner{}
	backend := &ClassicBackend{
		Recognizer: testRecognizer{text: "Hallo"}, Synthesizer: recordingSynthesizer{texts: texts}, Runner: runner,
		MaxDuration: time.Second, IdleTimeout: time.Second, AgentProviderID: "phone-llm",
		AdditionalPrompt: "Never disclose secrets.", Greeting: "Guten Tag.",
	}
	session, err := backend.Start(context.Background(), CallContext{CallID: "call-snapshot"}, NewBridge(4))
	if err != nil {
		t.Fatal(err)
	}
	classic, ok := session.(*classicSession)
	if !ok {
		t.Fatalf("session type = %T", session)
	}
	if classic.call.AgentProviderID != "phone-llm" || classic.call.AdditionalPrompt != "Never disclose secrets." {
		t.Fatalf("telephone call snapshot = %+v", classic.call)
	}
	select {
	case got := <-texts:
		if got != "Guten Tag." {
			t.Fatalf("greeting = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("configured greeting was not synthesized")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClassicGreetingBargeInCancelsAnnouncementAndFlushesAudio(t *testing.T) {
	bridge := NewBridge(8)
	runner := &testVoiceRunner{}
	synthesizer := &stagedStreamingSynthesizer{
		secondStarted: make(chan struct{}), releaseSecond: make(chan struct{}), done: make(chan struct{}),
	}
	backend := &ClassicBackend{
		Recognizer: testRecognizer{text: "Hallo"}, Synthesizer: synthesizer, Runner: runner,
		MaxDuration: time.Second, IdleTimeout: time.Second, Greeting: "Guten Tag.",
	}
	session, err := backend.Start(context.Background(), CallContext{CallID: "greeting-barge"}, bridge)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	select {
	case <-synthesizer.secondStarted:
	case <-time.After(time.Second):
		t.Fatal("greeting synthesis did not start")
	}
	readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
	if _, err := bridge.NextSend(readCtx); err != nil {
		cancelRead()
		t.Fatalf("greeting playback did not start: %v", err)
	}
	cancelRead()
	for index := 0; index < 6; index++ {
		if err := bridge.PushReceive(PCMFrame{Samples: loudPCMFrame(160), SampleRate: 8000}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-synthesizer.done:
	case <-time.After(time.Second):
		t.Fatal("barge-in did not cancel greeting synthesis")
	}
	if runner.cancelled.Load() == 0 {
		t.Fatal("barge-in did not cancel the active telephone turn")
	}
	emptyCtx, cancelEmpty := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelEmpty()
	if frame, err := bridge.NextSend(emptyCtx); err == nil {
		t.Fatalf("greeting audio remained queued after barge-in: %+v", frame)
	}
}

func TestClassicTurnTimeoutAnnouncesFailureAndEndsCall(t *testing.T) {
	texts := make(chan string, 1)
	runner := &blockingVoiceRunner{started: make(chan struct{}), release: make(chan struct{})}
	session := &classicSession{
		ctx: context.Background(), cancel: func() {}, call: CallContext{CallID: "turn-timeout", FailureMessage: "Technischer Fehler."},
		audio: NewBridge(4), backend: &ClassicBackend{
			Recognizer: testRecognizer{text: "Lange Aufgabe"}, Synthesizer: recordingSynthesizer{texts: texts}, Runner: runner,
			TurnTimeout: 20 * time.Millisecond,
		},
		events: make(chan VoiceEvent, 8), framePeriod: time.Millisecond,
	}
	done := make(chan struct{})
	go func() {
		session.handleUtterance(make([]int16, 160), 8000)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed-out voice turn did not return")
	}
	select {
	case text := <-texts:
		if text != "Technischer Fehler." {
			t.Fatalf("timeout announcement = %q", text)
		}
	default:
		t.Fatal("turn timeout did not play the failure announcement")
	}
	if runner.internalEnds.Load() != 1 || runner.internalWhy.Load() != "voice_backend_error" {
		t.Fatalf("turn timeout ended call %d times with reason %v", runner.internalEnds.Load(), runner.internalWhy.Load())
	}
}

func TestClassicBackendInactivitySaysGoodbyeAndEndsCall(t *testing.T) {
	texts := make(chan string, 2)
	runner := &testVoiceRunner{}
	backend := &ClassicBackend{
		Recognizer: testRecognizer{text: "Hallo"}, Synthesizer: recordingSynthesizer{texts: texts}, Runner: runner,
		MaxDuration: time.Second, IdleTimeout: 20 * time.Millisecond, GoodbyeMessage: "Auf Wiederhören.",
	}
	session, err := backend.Start(context.Background(), CallContext{CallID: "call-idle"}, NewBridge(4))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	select {
	case got := <-texts:
		if got != "Auf Wiederhören." {
			t.Fatalf("goodbye = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("inactivity did not synthesize the farewell")
	}
	deadline := time.Now().Add(time.Second)
	for runner.internalEnds.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if runner.internalEnds.Load() != 1 || runner.ended.Load() != 0 || runner.internalWhy.Load() != "inactivity_timeout" {
		t.Fatalf("internal ends = %d reason = %v, agent ends = %d", runner.internalEnds.Load(), runner.internalWhy.Load(), runner.ended.Load())
	}
}

func TestClassicBackendFailureAnnouncesAndEndsWithoutFallback(t *testing.T) {
	texts := make(chan string, 2)
	runner := &testVoiceRunner{}
	session := &classicSession{
		ctx: context.Background(), cancel: func() {}, call: CallContext{CallID: "call-failure", FailureMessage: "Technischer Fehler."},
		audio: NewBridge(4), backend: &ClassicBackend{
			Recognizer: failingRecognizer{}, Synthesizer: recordingSynthesizer{texts: texts}, Runner: runner,
		},
		events: make(chan VoiceEvent, 8), framePeriod: time.Millisecond,
	}
	session.handleUtterance(make([]int16, 160), 8000)
	select {
	case got := <-texts:
		if got != "Technischer Fehler." {
			t.Fatalf("failure announcement = %q", got)
		}
	default:
		t.Fatal("pipeline failure did not use the configured technical announcement")
	}
	if runner.internalEnds.Load() != 1 || runner.ended.Load() != 0 || runner.internalWhy.Load() != "voice_backend_error" {
		t.Fatalf("internal ends = %d reason = %v, agent ends = %d", runner.internalEnds.Load(), runner.internalWhy.Load(), runner.ended.Load())
	}
}

func TestClassicBackendStripsEndMarkerAndEndsAfterSpeaking(t *testing.T) {
	texts := make(chan string, 1)
	runner := &testVoiceRunner{response: "Das kann ich nicht erledigen. " + EndCallResponseMarker}
	session := &classicSession{
		ctx: context.Background(), cancel: func() {}, call: CallContext{CallID: "call-explain-end"},
		audio: NewBridge(4), backend: &ClassicBackend{
			Recognizer: testRecognizer{text: "Tu etwas Unerlaubtes"}, Synthesizer: recordingSynthesizer{texts: texts}, Runner: runner,
		},
		events: make(chan VoiceEvent, 8), framePeriod: time.Millisecond,
	}
	session.handleUtterance(make([]int16, 160), 8000)
	select {
	case spoken := <-texts:
		if spoken != "Das kann ich nicht erledigen." || strings.Contains(spoken, EndCallResponseMarker) {
			t.Fatalf("spoken response = %q", spoken)
		}
	default:
		t.Fatal("final explanation was not spoken")
	}
	if runner.ended.Load() != 1 {
		t.Fatalf("EndVoiceCall count = %d", runner.ended.Load())
	}
}

type blockingVoiceRunner struct {
	testVoiceRunner
	started chan struct{}
	release chan struct{}
}

func (r *blockingVoiceRunner) RunVoiceTurn(ctx context.Context, _ CallContext, _ string) (string, error) {
	close(r.started)
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-r.release:
		return "Fertig.", nil
	}
}

func TestClassicBackendPausesInactivityWhileTurnRuns(t *testing.T) {
	runner := &blockingVoiceRunner{started: make(chan struct{}), release: make(chan struct{})}
	backend := &ClassicBackend{
		Recognizer: testRecognizer{text: "Lange Aufgabe"}, Synthesizer: testSynthesizer{}, Runner: runner,
		MaxDuration: time.Second, IdleTimeout: 20 * time.Millisecond,
	}
	session, err := backend.Start(context.Background(), CallContext{CallID: "call-busy"}, NewBridge(4))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	classic := session.(*classicSession)
	classic.startUtterance(make([]int16, 160), 8000)
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("voice turn did not start")
	}
	time.Sleep(60 * time.Millisecond)
	if runner.internalEnds.Load() != 0 {
		t.Fatal("inactivity ended the call while the agent turn was running")
	}
	close(runner.release)
	deadline := time.Now().Add(time.Second)
	for runner.internalEnds.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if runner.internalEnds.Load() != 1 || runner.internalWhy.Load() != "inactivity_timeout" {
		t.Fatalf("inactivity did not resume after the turn, internal ends=%d reason=%v", runner.internalEnds.Load(), runner.internalWhy.Load())
	}
}

func TestClassicInterruptCancelsTurnAndFlushesOutput(t *testing.T) {
	runner := &testVoiceRunner{}
	bridge := NewBridge(2)
	_ = bridge.Send(context.Background(), PCMFrame{Samples: []int16{1}, SampleRate: 8000})
	_, turnCancel := context.WithCancel(context.Background())
	session := &classicSession{
		call: CallContext{CallID: "call-1"}, audio: bridge,
		backend: &ClassicBackend{Runner: runner}, turnCancel: turnCancel,
	}
	session.Interrupt()
	if runner.cancelled.Load() != 1 {
		t.Fatal("barge-in did not cancel the active agent turn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := bridge.NextSend(ctx); err == nil {
		t.Fatal("barge-in did not flush queued output")
	}
}

func TestTruncateVoiceResponseUsesBoundary(t *testing.T) {
	text, truncated := truncateVoiceResponse("Ein kurzer Satz. Danach kommt deutlich mehr Text.", 22)
	if !truncated || text != "Ein kurzer Satz." {
		t.Fatalf("truncateVoiceResponse = %q/%v", text, truncated)
	}
}

func TestResamplerSupportedRatesAndContinuity(t *testing.T) {
	resampler, err := NewResampler(8000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	one := resampler.Process([]int16{0, 1000, 2000})
	two := resampler.Process([]int16{3000, 4000})
	if len(one) == 0 || len(two) == 0 || math.Abs(float64(two[0]-one[len(one)-1])) > 1500 {
		t.Fatalf("unexpected discontinuity: %v then %v", one, two)
	}
	if _, err := NewResampler(44100, 8000); err == nil {
		t.Fatal("expected unsupported rate error")
	}
	providerResampler, err := NewSourceResampler(32000, 8000)
	if err != nil {
		t.Fatalf("provider sample rate rejected: %v", err)
	}
	if got := providerResampler.Process(make([]int16, 320)); len(got) == 0 {
		t.Fatal("provider sample rate produced no telephone audio")
	}
}

func TestWAVRoundTrip(t *testing.T) {
	want := []int16{-32768, -1, 0, 1, 32767}
	data, err := EncodeWAVPCM16(want, 16000)
	if err != nil {
		t.Fatal(err)
	}
	got, rate, err := DecodeWAVPCM16(data)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 16000 || len(got) != len(want) {
		t.Fatalf("unexpected WAV metadata rate=%d samples=%d", rate, len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d: want %d got %d", i, want[i], got[i])
		}
	}
}

func TestDecodeWAVPCM16SourceAcceptsProviderRate(t *testing.T) {
	data, err := EncodeWAVPCM16([]int16{1, 2, 3, 4}, 16000)
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(data[24:28], 22050)
	binary.LittleEndian.PutUint32(data[28:32], 44100)
	samples, rate, err := DecodeWAVPCM16Source(data)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 22050 || len(samples) != 4 {
		t.Fatalf("provider WAV rate=%d samples=%d", rate, len(samples))
	}
}

func TestTurnDetectorCompletesSpeechAfterSilence(t *testing.T) {
	detector := NewTurnDetector(20, 40, 60, 20)
	silence := make([]int16, 160)
	speech := make([]int16, 160)
	for i := range speech {
		speech[i] = 4000
	}
	detector.Push(silence)
	if started, _ := detector.Push(speech); started {
		t.Fatal("speech should require two frames")
	}
	if started, _ := detector.Push(speech); !started {
		t.Fatal("expected speech start")
	}
	var utterance []int16
	for range 3 {
		_, utterance = detector.Push(silence)
	}
	if len(utterance) == 0 {
		t.Fatal("expected completed utterance")
	}
}

func TestTurnDetectorDropsOldestAudioAtBound(t *testing.T) {
	detector := newTurnDetector(20, 20, 20, 0, 60, true)
	speech := make([]int16, 160)
	for i := range speech {
		speech[i] = 4000
	}
	if started, _ := detector.Push(speech); !started {
		t.Fatal("expected immediate speech start")
	}
	for range 10 {
		detector.Push(speech)
	}
	if !detector.TakeOverflow() {
		t.Fatal("expected bounded detector to report discarded audio")
	}
	if detector.TakeOverflow() {
		t.Fatal("overflow must be reported only once per utterance")
	}
	_, utterance := detector.Push(make([]int16, 160))
	if len(utterance) > 3*len(speech) {
		t.Fatalf("utterance exceeded configured bound: %d samples", len(utterance))
	}
}

func TestActivityDetectorDoesNotRetainUtteranceAudio(t *testing.T) {
	detector := NewActivityDetector(20, 20, 20, 0)
	speech := make([]int16, 160)
	for i := range speech {
		speech[i] = 4000
	}
	detector.Push(speech)
	for range 1000 {
		detector.Push(speech)
	}
	if len(detector.utterance) != 0 {
		t.Fatalf("activity detector retained %d samples", len(detector.utterance))
	}
	_, ended := detector.Push(make([]int16, 160))
	if ended == nil {
		t.Fatal("expected non-nil end-of-activity marker")
	}
}
