package homedash

import "time"

// Kind is the entity category a widget binds to.
type Kind string

const (
	KindSwitch        Kind = "switch"
	KindLight         Kind = "light"
	KindSensor        Kind = "sensor"
	KindBinarySensor  Kind = "binary_sensor"
	KindClimate       Kind = "climate"
	KindCover         Kind = "cover"
	KindFan           Kind = "fan"
	KindLock          Kind = "lock"
	KindMediaPlayer   Kind = "media_player"
	KindCamera        Kind = "camera"
	KindScene         Kind = "scene"
	KindScript        Kind = "script"
	KindButton        Kind = "button"
	KindNumber        Kind = "number"
	KindSelect        Kind = "select"
	KindHumidifier    Kind = "humidifier"
	KindVacuum        Kind = "vacuum"
	KindPresence      Kind = "presence"
	KindWeather       Kind = "weather"
	KindSun           Kind = "sun"
	KindUpdate        Kind = "update"
	KindServiceStatus Kind = "service_status"
	KindPrinter       Kind = "printer"
	KindGeneric       Kind = "generic"
)

// AllKinds returns every Kind in declaration order.
func AllKinds() []Kind {
	return []Kind{
		KindSwitch, KindLight, KindSensor, KindBinarySensor, KindClimate, KindCover, KindFan, KindLock,
		KindMediaPlayer, KindCamera, KindScene, KindScript, KindButton, KindNumber, KindSelect,
		KindHumidifier, KindVacuum, KindPresence, KindWeather, KindSun, KindUpdate, KindServiceStatus,
		KindPrinter, KindGeneric,
	}
}

// CapabilitySpec is one capability of one entity, with its gate state and value bounds.
type CapabilitySpec struct {
	Name      Capability `json:"name"`
	Allowed   bool       `json:"allowed"`              // false for read caps is meaningless; always true there
	BlockedBy string     `json:"blocked_by,omitempty"` // config key or "kiosk.view_only", "kiosk.denied", "virtual_desktop.readonly"
	Sensitive bool       `json:"sensitive,omitempty"`  // UI must confirm; server requires "confirmed": true
	Min       *float64   `json:"min,omitempty"`
	Max       *float64   `json:"max,omitempty"`
	Step      *float64   `json:"step,omitempty"`
	Options   []string   `json:"options,omitempty"` // <= 64 entries, each <= 64 runes
}

// MediaInfo describes camera-like media access of an entity.
type MediaInfo struct {
	Snapshot   bool   `json:"snapshot"`
	Live       string `json:"live,omitempty"`        // "go2rtc_viewer" | "mjpeg" | ""
	ViewerPath string `json:"viewer_path,omitempty"` // desktop only, e.g. /api/desktop/integrations/go2rtc/viewer/<id>
}

// State is the current state of an entity.
type State struct {
	Value      any            `json:"value"`                // string | float64 | bool | nil
	Text       string         `json:"text,omitempty"`       // raw state text, <= 255 runes
	Attributes map[string]any `json:"attributes,omitempty"` // allowlisted, <= 8 KiB JSON
	Available  bool           `json:"available"`
	Stale      bool           `json:"stale,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// Entity is one controllable or readable thing of a source.
type Entity struct {
	ID             EntityID         `json:"id"`
	Source         SourceID         `json:"source"`
	Kind           Kind             `json:"kind"`
	Name           string           `json:"name"` // external text, <= 120 runes
	Area           string           `json:"area,omitempty"`
	AreaID         string           `json:"area_id,omitempty"`
	DeviceClass    string           `json:"device_class,omitempty"`
	Unit           string           `json:"unit,omitempty"`
	IconHint       string           `json:"icon_hint,omitempty"` // catalog icon id, never a URL
	Capabilities   []CapabilitySpec `json:"capabilities"`
	ReadOnly       bool             `json:"readonly"`
	ReadOnlyReason string           `json:"readonly_reason,omitempty"`
	Media          *MediaInfo       `json:"media,omitempty"`
	State          State            `json:"state"`
}

// StateUpdate is one state change reported by a source; Native is the source-local id.
type StateUpdate struct {
	Native string
	State  State
}

// SourceStatus is the runtime status of one source.
type SourceStatus struct {
	Source SourceID  `json:"source"`
	State  string    `json:"state"`            // disabled|idle|starting|live|polling|degraded|unavailable
	Reason string    `json:"reason,omitempty"` // fixed code: auth_failed, unreachable, timeout, not_configured, ...
	At     time.Time `json:"at"`
}

const (
	StatusDisabled    = "disabled"
	StatusIdle        = "idle"
	StatusStarting    = "starting"
	StatusLive        = "live"
	StatusPolling     = "polling"
	StatusDegraded    = "degraded"
	StatusUnavailable = "unavailable"

	ReasonCatalogFailed = "catalog_failed"
	ReasonCrashed       = "crashed"
	ReasonStopped       = "stopped"
)
