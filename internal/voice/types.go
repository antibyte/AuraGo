package voice

import (
	"context"
	"time"
)

// PCMFrame carries signed mono PCM samples at one of AuraGo's supported rates.
// Samples are owned by the receiver and may be reused after Send returns.
type PCMFrame struct {
	Samples    []int16
	SampleRate int
	Timestamp  time.Duration
}

// CallContext contains the privacy-safe metadata exposed to voice backends.
type CallContext struct {
	CallID       string
	Direction    string
	RemoteParty  string
	Language     string
	SessionID    string
	AllowedTools []string
}

// VoiceEvent reports bounded, structured backend and media state changes.
type VoiceEvent struct {
	Type      string
	Message   string
	Timestamp time.Time
	Data      map[string]any
}

// DuplexAudio is the media boundary shared by SIP and future browser peers.
type DuplexAudio interface {
	Receive() <-chan PCMFrame
	Send(context.Context, PCMFrame) error
	FlushOutput()
}

// VoiceBackend binds a voice intelligence implementation to one call.
type VoiceBackend interface {
	Start(context.Context, CallContext, DuplexAudio) (VoiceSession, error)
}

// VoiceSession controls an active voice intelligence session.
type VoiceSession interface {
	Interrupt()
	Events() <-chan VoiceEvent
	Close() error
}

// SpeechRecognizer converts one complete telephone utterance into text.
type SpeechRecognizer interface {
	Recognize(context.Context, []byte, int, string) (string, error)
}

// SpeechSynthesizer returns signed mono PCM for a response.
type SpeechSynthesizer interface {
	Synthesize(context.Context, string, string) ([]int16, int, error)
}

// VoiceActionRunner runs one isolated user turn through AuraGo's agent path.
type VoiceActionRunner interface {
	RunVoiceTurn(context.Context, CallContext, string) (string, error)
	CancelVoiceTurn(string)
	EndVoiceCall(string)
}
