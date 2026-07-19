package bluetooth

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ErrorUnavailable                = "BLUETOOTH_UNAVAILABLE"
	ErrorReadOnly                   = "BLUETOOTH_READ_ONLY"
	ErrorPlaybackDisabled           = "BLUETOOTH_PLAYBACK_DISABLED"
	ErrorDeviceNotFound             = "BLUETOOTH_DEVICE_NOT_FOUND"
	ErrorDeviceAmbiguous            = "BLUETOOTH_DEVICE_AMBIGUOUS"
	ErrorDeviceNotPaired            = "BLUETOOTH_DEVICE_NOT_PAIRED"
	ErrorAudioTargetUnavailable     = "BLUETOOTH_AUDIO_TARGET_UNAVAILABLE"
	ErrorPairingInteractionRequired = "PAIRING_INTERACTION_REQUIRED"
	ErrorInvalidArgument            = "BLUETOOTH_INVALID_ARGUMENT"
)

var bluetoothAddressPattern = regexp.MustCompile(`(?i)^[0-9a-f]{2}([:-][0-9a-f]{2}){5}$`)

// Options controls Bluetooth access without depending on the config package.
type Options struct {
	Enabled            bool
	ReadOnly           bool
	AllowPlayback      bool
	ScanTimeout        time.Duration
	DefaultDevice      string
	AudioBackend       string
	IsDocker           bool
	WorkspaceDirectory string
	DataDirectory      string
}

// AdapterStatus describes the selected BlueZ adapter.
type AdapterStatus struct {
	Path    string `json:"path,omitempty"`
	Address string `json:"address,omitempty"`
	Name    string `json:"name,omitempty"`
	Powered bool   `json:"powered"`
}

// AudioStatus describes the per-stream audio backend detected for Bluetooth.
type AudioStatus struct {
	Usable  bool   `json:"usable"`
	Backend string `json:"backend,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Status is the runtime-only Bluetooth capability snapshot.
type Status struct {
	Supported    bool          `json:"supported"`
	Usable       bool          `json:"usable"`
	Reason       string        `json:"reason,omitempty"`
	Adapter      AdapterStatus `json:"adapter"`
	Audio        AudioStatus   `json:"audio"`
	LastProbedAt time.Time     `json:"last_probed_at"`
}

// Device is a BlueZ device suitable for display or explicit selection.
type Device struct {
	Address   string   `json:"address"`
	Name      string   `json:"name,omitempty"`
	Alias     string   `json:"alias,omitempty"`
	Paired    bool     `json:"paired"`
	Connected bool     `json:"connected"`
	Trusted   bool     `json:"trusted"`
	Audio     bool     `json:"audio"`
	RSSI      *int16   `json:"rssi,omitempty"`
	UUIDs     []string `json:"uuids,omitempty"`
}

// PlaybackStatus is the asynchronous state of AuraGo-owned Bluetooth audio.
type PlaybackStatus struct {
	ID            string    `json:"playback_id,omitempty"`
	State         string    `json:"state"`
	Source        string    `json:"source,omitempty"`
	DeviceAddress string    `json:"device_address,omitempty"`
	DeviceName    string    `json:"device_name,omitempty"`
	Backend       string    `json:"backend,omitempty"`
	Target        string    `json:"target,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// CodedError exposes stable machine-readable errors to the tool and admin API.
type CodedError struct {
	Code    string
	Message string
	Err     error
}

func (e *CodedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *CodedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func codedError(code, message string, err error) error {
	return &CodedError{Code: code, Message: message, Err: err}
}

// ErrorCode returns a stable Bluetooth error code.
func ErrorCode(err error) string {
	var coded *CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// NormalizeAddress validates and canonicalizes a Bluetooth MAC address.
func NormalizeAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if !bluetoothAddressPattern.MatchString(address) {
		return "", codedError(ErrorInvalidArgument, fmt.Sprintf("invalid Bluetooth address %q", address), nil)
	}
	return strings.ToUpper(strings.ReplaceAll(address, "-", ":")), nil
}

func normalizeOptions(options Options) Options {
	if options.ScanTimeout <= 0 {
		options.ScanTimeout = 10 * time.Second
	}
	switch strings.ToLower(strings.TrimSpace(options.AudioBackend)) {
	case "", "auto":
		options.AudioBackend = "auto"
	case "pipewire":
		options.AudioBackend = "pipewire"
	case "pulse", "pulseaudio":
		options.AudioBackend = "pulse"
	default:
		options.AudioBackend = "auto"
	}
	options.DefaultDevice = strings.TrimSpace(options.DefaultDevice)
	return options
}

func isAudioDevice(device Device) bool {
	for _, uuid := range device.UUIDs {
		normalized := strings.ToLower(strings.TrimSpace(uuid))
		if strings.Contains(normalized, "0000110b-") ||
			strings.Contains(normalized, "0000184e-") ||
			strings.Contains(normalized, "00001850-") ||
			strings.Contains(normalized, "00001853-") {
			return true
		}
	}
	return device.Audio
}
