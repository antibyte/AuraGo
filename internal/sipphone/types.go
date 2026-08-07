package sipphone

import (
	"context"
	"errors"
	"time"

	"aurago/internal/config"
	"aurago/internal/voice"
)

type State string

const (
	StateDisabled    State = "disabled"
	StateRegistering State = "registering"
	StateRegistered  State = "registered"
	StateRinging     State = "ringing"
	StateConnecting  State = "connecting"
	StateActive      State = "active"
	StateEnding      State = "ending"
	StateEnded       State = "ended"
	StateFailed      State = "failed"
)

const (
	MediaModeAgent   = "agent"
	MediaModeBrowser = "browser"
	mediaModeNone    = "none"
)

var (
	ErrDisabled             = errors.New("SIP endpoint is disabled")
	ErrReadOnly             = errors.New("SIP endpoint is read-only")
	ErrBusy                 = errors.New("SIP endpoint already has an active call")
	ErrSpeechLabStackChange = errors.New("Speech Lab stack activation is in progress")
	ErrPermissionDenied     = errors.New("SIP operation is not permitted")
	ErrCallNotFound         = errors.New("SIP call not found")
	ErrInvalidTarget        = errors.New("invalid SIP target")
	ErrNetworkConfiguration = errors.New("SIP advertised signaling and media hosts are required in Docker")
	ErrMediaTimeout         = errors.New("SIP RTP media timed out")
	ErrAgentDailyCallLimit  = errors.New("telephone agent daily outbound call limit reached")
)

type AgentDailyCallLimitError struct {
	Used              int       `json:"used"`
	Limit             int       `json:"limit"`
	RetryAfterSeconds int       `json:"retry_after_seconds"`
	ResetsAt          time.Time `json:"resets_at"`
}

func (e *AgentDailyCallLimitError) Error() string { return ErrAgentDailyCallLimit.Error() }
func (e *AgentDailyCallLimitError) Unwrap() error { return ErrAgentDailyCallLimit }

type DailyCallUsage struct {
	Available bool       `json:"available"`
	Date      string     `json:"date,omitempty"`
	Used      int        `json:"used"`
	Limit     int        `json:"limit"`
	Remaining int        `json:"remaining"`
	ResetsAt  *time.Time `json:"resets_at,omitempty"`
}

type CallRecord struct {
	ID                 string     `json:"id"`
	Direction          string     `json:"direction"`
	RemoteParty        string     `json:"remote_party"`
	StartedAt          time.Time  `json:"started_at"`
	AnsweredAt         *time.Time `json:"answered_at,omitempty"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	State              State      `json:"state"`
	EndReason          string     `json:"end_reason,omitempty"`
	Backend            string     `json:"backend"`
	MediaMode          string     `json:"media_mode"`
	SessionID          string     `json:"session_id,omitempty"`
	persistTranscripts bool
}

type Status struct {
	Enabled                bool        `json:"enabled"`
	ReadOnly               bool        `json:"readonly"`
	State                  State       `json:"state"`
	Registered             bool        `json:"registered"`
	RegistrationError      string      `json:"registration_error,omitempty"`
	RegistrationStatusCode int         `json:"registration_status_code,omitempty"`
	RegistrationRetryAt    *time.Time  `json:"registration_retry_at,omitempty"`
	ActiveCall             *CallRecord `json:"active_call,omitempty"`
	Transport              string      `json:"transport"`
	BindAddress            string      `json:"bind_address"`
	PasswordSet            bool        `json:"password_set"`
}

type Event struct {
	Sequence  uint64      `json:"sequence"`
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Status    *Status     `json:"status,omitempty"`
	Call      *CallRecord `json:"call,omitempty"`
	Data      any         `json:"data,omitempty"`
}

type BackendFactory func(context.Context, config.SIPVoiceConfig) (voice.VoiceBackend, error)

type IssueReporter func(context.Context, string, string)

// MediaPeer is the attachment point for future authenticated browser audio.
type MediaPeer interface {
	Attach(context.Context, string, voice.DuplexAudio) error
	Detach(string)
}

type mediaDirectionStats struct {
	receivedFrames  uint64
	sentFrames      uint64
	receivedPackets uint64
	sentPackets     uint64
}

type mediaStatsProvider interface {
	mediaStats() mediaDirectionStats
}

// IncomingCallHandler allows future voicemail routing without changing SIP.
type IncomingCallHandler interface {
	HandleIncoming(context.Context, CallRecord, voice.DuplexAudio) error
}

func validTransition(from, to State) bool {
	if from == to {
		return true
	}
	allowed := map[State]map[State]bool{
		StateDisabled:    {StateRegistering: true, StateRegistered: true},
		StateRegistering: {StateRegistered: true, StateFailed: true, StateDisabled: true},
		StateRegistered:  {StateRinging: true, StateConnecting: true, StateFailed: true, StateDisabled: true},
		StateRinging:     {StateConnecting: true, StateActive: true, StateEnding: true, StateEnded: true, StateFailed: true},
		StateConnecting:  {StateRinging: true, StateActive: true, StateEnding: true, StateEnded: true, StateFailed: true},
		StateActive:      {StateEnding: true, StateEnded: true, StateFailed: true},
		StateEnding:      {StateEnded: true, StateFailed: true},
		StateEnded:       {StateRegistered: true, StateDisabled: true, StateConnecting: true, StateRinging: true},
		StateFailed:      {StateRegistering: true, StateRegistered: true, StateDisabled: true, StateConnecting: true},
	}
	return allowed[from][to]
}
