package voice

import (
	"context"
	"encoding/binary"
	"math"
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

type testVoiceRunner struct {
	cancelled atomic.Int32
}

func (r *testVoiceRunner) RunVoiceTurn(context.Context, CallContext, string) (string, error) {
	return "Antwort", nil
}
func (r *testVoiceRunner) CancelVoiceTurn(string) { r.cancelled.Add(1) }
func (r *testVoiceRunner) EndVoiceCall(string)    {}

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
