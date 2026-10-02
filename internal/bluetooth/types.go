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
	ErrorPoweredOff                 = "BLUETOOTH_POWERED_OFF"
	ErrorBlocked                    = "BLUETOOTH_BLOCKED"
	ErrorOperationBusy              = "BLUETOOTH_OPERATION_BUSY"
	ErrorInteractionExpired         = "BLUETOOTH_INTERACTION_EXPIRED"
	ErrorInteractionNotFound        = "BLUETOOTH_INTERACTION_NOT_FOUND"
	ErrorPairingRejected            = "PAIRING_REJECTED"
	ErrorPairingFailed              = "PAIRING_FAILED"
	ErrorDeviceUnreachable          = "DEVICE_UNREACHABLE"
	ErrorProfileUnavailable         = "BLUETOOTH_PROFILE_UNAVAILABLE"
	ErrorHeadsetBusy                = "BLUETOOTH_HEADSET_BUSY"
	ErrorHeadsetUnknown             = "BLUETOOTH_HEADSET_UNKNOWN"
	ErrorHeadsetProfileUnavailable  = "BLUETOOTH_HEADSET_PROFILE_UNAVAILABLE"
	ErrorHeadsetAudioUnavailable    = "BLUETOOTH_HEADSET_AUDIO_UNAVAILABLE"
	ErrorGeneric                    = "BLUETOOTH_ERROR"
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
	Path                string `json:"path,omitempty"`
	Address             string `json:"address,omitempty"`
	Name                string `json:"name,omitempty"`
	Powered             bool   `json:"powered"`
	PowerState          string `json:"power_state,omitempty"`
	Discoverable        bool   `json:"discoverable"`
	DiscoverableTimeout uint32 `json:"discoverable_timeout,omitempty"`
	Discovering         bool   `json:"discovering"`
	Pairable            bool   `json:"pairable"`
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
	Present      bool          `json:"present"`
	Reason       string        `json:"reason,omitempty"`
	Adapter      AdapterStatus `json:"adapter"`
	Audio        AudioStatus   `json:"audio"`
	LastProbedAt time.Time     `json:"last_probed_at"`
}

// Device is a BlueZ device suitable for display or explicit selection.
type Device struct {
	Address    string   `json:"address"`
	Name       string   `json:"name,omitempty"`
	Alias      string   `json:"alias,omitempty"`
	Paired     bool     `json:"paired"`
	Connected  bool     `json:"connected"`
	Trusted    bool     `json:"trusted"`
	Audio      bool     `json:"audio"`
	Microphone bool     `json:"microphone"`
	RSSI       *int16   `json:"rssi,omitempty"`
	UUIDs      []string `json:"uuids,omitempty"`
	Type       string   `json:"type,omitempty"`
	Battery    *int     `json:"battery,omitempty"`
	Operation  string   `json:"operation,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// TimedState describes a server-owned timed adapter mode.
type TimedState struct {
	Active           bool      `json:"active"`
	EndsAt           time.Time `json:"ends_at,omitempty"`
	RemainingSeconds int       `json:"remaining_seconds"`
}

// Snapshot is the live view rendered by the desktop app.
type Snapshot struct {
	Revision      uint64        `json:"revision"`
	Present       bool          `json:"present"`
	Reason        string        `json:"reason,omitempty"`
	Adapter       AdapterStatus `json:"adapter"`
	Devices       []Device      `json:"devices"`
	Discovery     TimedState    `json:"discovery"`
	Discoverable  TimedState    `json:"discoverable"`
	InteractionID string        `json:"interaction_id,omitempty"`
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

// hasMicrophone reports a Handsfree (HFP) or Headset (HSP) role: the classic
// Bluetooth profiles that carry a microphone towards this server.
func hasMicrophone(device Device) bool {
	for _, uuid := range device.UUIDs {
		normalized := strings.ToLower(strings.TrimSpace(uuid))
		if strings.HasPrefix(normalized, "0000111e-") || strings.HasPrefix(normalized, "00001108-") {
			return true
		}
	}
	return false
}
