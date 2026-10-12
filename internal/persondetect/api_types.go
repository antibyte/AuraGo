package persondetect

import (
	"time"

	"aurago/internal/onnxrt"
)

// Status is the service state shown by the app and the agent tool.
type Status struct {
	Enabled  bool           `json:"enabled"`
	Detector DetectorStatus `json:"detector"`
	Arm      ArmState       `json:"arm"`
	Cameras  []CameraStatus `json:"cameras"`
	Storage  StorageStatus  `json:"storage"`
	Cursor   int64          `json:"cursor"`
}

// DetectorStatus describes setup progress and the running detector. WallMS and
// CPUMS are moving averages over the last TimingAverageWindow detections.
type DetectorStatus struct {
	State            string                `json:"state"` // off | downloading | preparing | ready | degraded | error
	Phase            string                `json:"phase"`
	Download         onnxrt.DownloadStatus `json:"download"`
	Error            string                `json:"error"`
	Model            ModelTier             `json:"model"`
	ModelName        string                `json:"model_name"`
	RequestedBackend Backend               `json:"requested_backend"`
	Backend          Backend               `json:"backend"`
	Runtime          string                `json:"runtime"`
	Providers        []string              `json:"providers"`
	FallbackReason   string                `json:"fallback_reason"`
	SelfTest         *SelfTest             `json:"self_test"`
	WallMS           float64               `json:"wall_ms"`
	CPUMS            float64               `json:"cpu_ms"`
}

// ArmState is the arm mode, whether alerts are armed now and the next scheduled change.
type ArmState struct {
	Mode       ArmMode    `json:"mode"`
	Armed      bool       `json:"armed"`
	NextChange *time.Time `json:"next_change"`
	ChangedAt  *time.Time `json:"changed_at"`
	ChangedBy  string     `json:"changed_by"` // user | agent | schedule | ""
}

// CameraStatus is the runtime state of one detecting camera.
type CameraStatus struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	State             string     `json:"state"` // active | unreachable | waiting | error
	AnalysesPerMinute float64    `json:"analyses_per_minute"`
	LastEventID       *int64     `json:"last_event_id"`
	LastEventAt       *time.Time `json:"last_event_at"`
	LastError         string     `json:"last_error"`
}

// StorageStatus reports the bytes held by stored frames and the configured limit.
type StorageStatus struct {
	Bytes      int64  `json:"bytes"`
	LimitBytes int64  `json:"limit_bytes"`
	Warning    string `json:"warning"` // "" | storage_full
}

// Changes are the rows written after a cursor plus the current status.
type Changes struct {
	Status Status         `json:"status"`
	Events []EventSummary `json:"events"`
	ArmLog []ArmLogEntry  `json:"arm_log"`
	Cursor int64          `json:"cursor"`
}

// EventSummary is one detection event. EndedAt is nil while the event is open.
type EventSummary struct {
	ID           int64                  `json:"id"`
	CameraID     string                 `json:"camera_id"`
	CameraName   string                 `json:"camera_name"`
	StartedAt    time.Time              `json:"started_at"`
	EndedAt      *time.Time             `json:"ended_at"`
	MaxScore     float64                `json:"max_score"`
	MaxPersons   int                    `json:"max_persons"`
	Zones        []string               `json:"zones"`
	Armed        bool                   `json:"armed"`
	CoverFrameID *int64                 `json:"cover_frame_id"`
	FrameCount   int                    `json:"frame_count"`
	Alerts       map[string]AlertResult `json:"alerts"`
}

// EventFrame is one stored frame of an event with all boxes at or above the camera threshold.
type EventFrame struct {
	ID         int64     `json:"id"`
	Seq        int       `json:"seq"`
	CapturedAt time.Time `json:"captured_at"`
	Score      float64   `json:"score"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Boxes      []Box     `json:"boxes"`
}

// EventDetail is an event with its frames.
type EventDetail struct {
	EventSummary
	Frames []EventFrame `json:"frames"`
}

// ArmLogEntry is one arm-mode change or scheduled transition.
type ArmLogEntry struct {
	ID    int64     `json:"id"`
	At    time.Time `json:"at"`
	Mode  ArmMode   `json:"mode"`
	Armed bool      `json:"armed"`
	Actor string    `json:"actor"`
}

// EventQuery filters the event list: From inclusive and To exclusive on the start
// time, Before an event id for paging. Limit defaults to 50, at most 100.
type EventQuery struct {
	CameraID string
	From, To time.Time
	Before   int64
	Limit    int
}

// EventPage is one page of events, newest first.
type EventPage struct {
	Events     []EventSummary `json:"events"`
	NextBefore *int64         `json:"next_before"`
}

// TestBox is a detected box with its verdict under a camera's zones and threshold.
type TestBox struct {
	Box
	Counts bool     `json:"counts"`
	Zones  []string `json:"zones"`
}

// TestResult is the outcome of analyzing one current frame on demand.
type TestResult struct {
	Image   string    `json:"image"` // data:image/jpeg;base64,…
	Width   int       `json:"width"`
	Height  int       `json:"height"`
	Backend Backend   `json:"backend"`
	WallMS  float64   `json:"wall_ms"`
	Boxes   []TestBox `json:"boxes"`
}
