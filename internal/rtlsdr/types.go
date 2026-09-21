// Package rtlsdr owns the optional desktop receiver and durable recording jobs.
package rtlsdr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	MaxDuration        = 2 * time.Hour
	DefaultQuota int64 = 10 << 30
	Warmup             = 30 * time.Second
	LeaseTTL           = 35 * time.Second
)

var (
	ErrDisabled    = errors.New("sdr_disabled")
	ErrReadOnly    = errors.New("sdr_read_only")
	ErrBusy        = errors.New("sdr_busy")
	ErrNotFound    = errors.New("sdr_not_found")
	ErrInvalid     = errors.New("sdr_invalid_request")
	ErrQuota       = errors.New("sdr_quota_exceeded")
	ErrUnavailable = errors.New("sdr_receiver_unavailable")
)

type Tuning struct {
	Frequency int64   `json:"frequency_hz"`
	Mode      string  `json:"mode"`
	Bandwidth int     `json:"bandwidth_hz"`
	Gain      float64 `json:"gain_db"`
	AGC       bool    `json:"agc"`
	PPM       int     `json:"ppm"`
	Squelch   float64 `json:"squelch_db"`
	Stereo    bool    `json:"stereo"`
	Block     string  `json:"dab_block,omitempty"`
	ServiceID string  `json:"service_id,omitempty"`
	Label     string  `json:"label,omitempty"`
}

func DefaultTuning() Tuning {
	return Tuning{Frequency: 100000000, Mode: "wfm", Bandwidth: 180000, AGC: true, Squelch: -100, Stereo: true}
}

var DABBlocks = map[string]int64{
	"5A": 174928000, "5B": 176640000, "5C": 178352000, "5D": 180064000,
	"6A": 181936000, "6B": 183648000, "6C": 185360000, "6D": 187072000,
	"7A": 188928000, "7B": 190640000, "7C": 192352000, "7D": 194064000,
	"8A": 195936000, "8B": 197648000, "8C": 199360000, "8D": 201072000,
	"9A": 202928000, "9B": 204640000, "9C": 206352000, "9D": 208064000,
	"10A": 209936000, "10B": 211648000, "10C": 213360000, "10D": 215072000,
	"11A": 216928000, "11B": 218640000, "11C": 220352000, "11D": 222064000,
	"12A": 223936000, "12B": 225648000, "12C": 227360000, "12D": 229072000,
	"13A": 230784000, "13B": 232496000, "13C": 234208000, "13D": 235776000, "13E": 237488000, "13F": 239200000,
}

func (t *Tuning) Validate() error {
	if len(t.Label) > 120 || strings.ContainsAny(t.Label, "\r\n") || math.IsNaN(t.Gain) || math.IsInf(t.Gain, 0) || math.IsNaN(t.Squelch) || math.IsInf(t.Squelch, 0) || t.PPM < -200 || t.PPM > 200 || t.Gain < -10 || t.Gain > 60 || t.Squelch < -140 || t.Squelch > 0 {
		return ErrInvalid
	}
	switch t.Mode {
	case "dab":
		f, ok := DABBlocks[t.Block]
		if !ok {
			return ErrInvalid
		}
		t.Frequency = f
		t.Bandwidth = 1536000
		if t.ServiceID != "" {
			sid, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(t.ServiceID), "0x"), 16, 32)
			if err != nil {
				return ErrInvalid
			}
			t.ServiceID = fmt.Sprintf("0x%04x", sid)
		}
	case "wfm":
		if t.Bandwidth == 0 {
			t.Bandwidth = 180000
		}
		if t.Bandwidth < 80000 || t.Bandwidth > 250000 {
			return ErrInvalid
		}
	case "nfm", "am":
		if t.Bandwidth == 0 {
			t.Bandwidth = 12500
		}
		if t.Bandwidth < 4000 || t.Bandwidth > 25000 {
			return ErrInvalid
		}
	case "usb", "lsb":
		if t.Bandwidth == 0 {
			t.Bandwidth = 2800
		}
		if t.Bandwidth < 1000 || t.Bandwidth > 5000 {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if t.Frequency < 100000 || t.Frequency > 2200000000 {
		return ErrInvalid
	}
	return nil
}

type Station struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Tuning Tuning `json:"tuning"`
}
type Segment struct {
	Start float64 `json:"start_seconds"`
	End   float64 `json:"end_seconds"`
	Text  string  `json:"text"`
	Error string  `json:"error,omitempty"`
}
type Recording struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Tuning     Tuning    `json:"tuning"`
	Start      time.Time `json:"start_at"`
	Duration   int       `json:"duration_seconds"`
	Transcribe bool      `json:"transcribe"`
	Agent      bool      `json:"agent,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	ASRError   string    `json:"asr_error,omitempty"`
	Seconds    float64   `json:"captured_seconds"`
	Bytes      int64     `json:"bytes"`
	Gaps       int       `json:"gaps"`
	ScheduleID string    `json:"schedule_id,omitempty"`
	Segments   []Segment `json:"segments"`
	Created    time.Time `json:"created_at"`
}
type Schedule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Tuning     Tuning    `json:"tuning"`
	Start      time.Time `json:"start_at"`
	Timezone   string    `json:"timezone"`
	Repeat     string    `json:"repeat"`
	Duration   int       `json:"duration_seconds"`
	Transcribe bool      `json:"transcribe"`
	Agent      bool      `json:"agent,omitempty"`
	Enabled    bool      `json:"enabled"`
	LastSlot   time.Time `json:"last_slot,omitempty"`
	NextStart  time.Time `json:"next_start_at,omitempty"`
}
type Receiver struct {
	Ready    bool      `json:"ready"`
	Device   string    `json:"device"`
	Tuner    string    `json:"tuner"`
	Minimum  int64     `json:"minimum_hz"`
	Maximum  int64     `json:"maximum_hz"`
	Gains    []float64 `json:"gains_db"`
	Power    float64   `json:"power_db"`
	SNR      float64   `json:"snr_db"`
	Stereo   bool      `json:"stereo"`
	Label    string    `json:"label"`
	Text     string    `json:"text"`
	Error    string    `json:"error,omitempty"`
	Spectrum []float64 `json:"spectrum"`
	Center   int64     `json:"center_hz"`
	Span     int       `json:"span_hz"`
}
type CaptureInfo struct {
	Seconds float64
	Gaps    int
}
type Backend interface {
	Info(context.Context) (Receiver, error)
	Tune(context.Context, Tuning) error
	Stop(context.Context) error
	Stream(context.Context) (io.ReadCloser, error)
	Capture(context.Context, int, io.Writer) (CaptureInfo, error)
	Scan(context.Context, func([]Station, string)) error
	WAV(context.Context, string, float64, int) ([]byte, error)
}
type Transcriber interface {
	Transcribe(context.Context, []byte) (string, error)
}
type Policy struct {
	Enabled    bool
	ReadOnly   bool
	AllowAgent bool
	QuotaBytes int64
}
type Options struct {
	Directory      string
	Backend        Backend
	Policy         func() Policy
	NewTranscriber func(context.Context) (Transcriber, error)
	Register       func(context.Context, Recording, string) error
	Unregister     func(string) error
	Issue          func(string, bool)
	NotifyUpcoming func()
	Now            func() time.Time
}
type State struct {
	Version    int         `json:"version"`
	Tuning     Tuning      `json:"tuning"`
	Favorites  []Station   `json:"favorites"`
	Stations   []Station   `json:"stations"`
	Recordings []Recording `json:"recordings"`
	Schedules  []Schedule  `json:"schedules"`
}
type Snapshot struct {
	State
	Active     string `json:"active_job,omitempty"`
	Listening  bool   `json:"listening"`
	Scanning   bool   `json:"scanning"`
	ScanBlock  string `json:"scan_block,omitempty"`
	ScanError  string `json:"scan_error,omitempty"`
	UsedBytes  int64  `json:"used_bytes"`
	QuotaBytes int64  `json:"quota_bytes"`
	ReadOnly   bool   `json:"read_only"`
}
