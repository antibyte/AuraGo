package persondetect

import (
	"context"
	"errors"
	"time"
)

// ArmMode is the persisted arming mode. Message, agent and desktop alerts fire
// only while armed; detection and image storage always run.
type ArmMode string

const (
	ArmOff      ArmMode = "off"
	ArmOn       ArmMode = "on"
	ArmSchedule ArmMode = "schedule"
)

// ValidArmMode reports whether s names an arm mode.
func ValidArmMode(s string) bool {
	switch ArmMode(s) {
	case ArmOff, ArmOn, ArmSchedule:
		return true
	default:
		return false
	}
}

// Arm log actors.
const (
	ActorUser     = "user"
	ActorAgent    = "agent"
	ActorSchedule = "schedule"
)

// Point is a normalized frame coordinate: 0..1, origin at the top left.
type Point struct{ X, Y float64 }

// Zone is a named polygon. A person counts when the foot point of its box lies inside.
type Zone struct {
	Name   string
	Points []Point
}

// Window is a weekly arming window. Days are the weekdays the window starts on;
// From and To are minutes since midnight; To <= From crosses midnight.
type Window struct {
	Days     []time.Weekday
	From, To int
}

// Actions selects the alerts of one camera.
type Actions struct {
	Message          bool
	Channel          string
	Agent            bool
	AgentInstruction string
	Desktop          bool
}

// Camera is one go2rtc stream whose stream and detection are both enabled.
type Camera struct {
	ID, Name string
	Interval time.Duration
	MinScore float64
	Cooldown time.Duration
	Zones    []Zone
	Actions  Actions
}

// Settings is the complete runtime configuration of the service.
type Settings struct {
	Enabled       bool
	DataDir       string // <directories.data_dir>
	Model         ModelTier
	Backend       Backend
	RetentionDays int
	MaxStorageMB  int
	Schedule      []Window
	Cameras       []Camera
	Location      *time.Location // server local time
	Language      string         // cfg.Server.UILanguage, for alert texts
}

// FrameSource delivers the current JPEG frame of a camera.
type FrameSource interface {
	Frame(ctx context.Context, cameraID string) ([]byte, error)
}

// Detection-logic constants (spec "Detection logic").
const (
	EventStartWindow      = 3                // analyzed frames considered by the start rule
	EventStartHits        = 2                // hits within the window that start an event
	EventSingleHitScore   = 0.8              // a single hit at or above this score starts an event
	EventEndAfter         = 30 * time.Second // an event ends after this long without a hit
	EventFrameInterval    = 5 * time.Second  // at most one stored frame per interval while hits continue
	EventMaxFrames        = 20               // stored frames per event
	EventPendingHitFrames = 2                // hit frames kept in memory until the start decision
)

// Service defaults and timings.
const (
	DefaultCameraInterval = 2 * time.Second
	DefaultCameraMinScore = 0.5
	DefaultRetentionDays  = 14
	DefaultMaxStorageMB   = 2048
	MaxFetchBackoff       = 60 * time.Second
	FrameFetchTimeout     = 15 * time.Second
	TestMinScore          = 0.3
	TimingAverageWindow   = 20
	ThumbnailWidth        = 320
	ThumbnailQuality      = 80
	RetentionInterval     = time.Hour
	ArmLogMinAge          = 24 * time.Hour
	ArmCheckInterval      = 10 * time.Second
	SweepInterval         = time.Second
	StorageTargetRatio    = 0.9
)

// Status values used in the C4 JSON shapes.
const (
	DetectorOff         = "off"
	DetectorDownloading = "downloading"
	DetectorPreparing   = "preparing"
	DetectorReady       = "ready"
	DetectorDegraded    = "degraded"
	DetectorError       = "error"

	CameraActive      = "active"
	CameraUnreachable = "unreachable"
	CameraWaiting     = "waiting"
	CameraError       = "error"

	AlertSent    = "sent"
	AlertFailed  = "failed"
	AlertSkipped = "skipped"

	StorageFullWarning = "storage_full"
)

var (
	ErrDisabled         = errors.New("person detection is disabled")
	ErrNotReady         = errors.New("person detector is not ready")
	ErrUnknownCamera    = errors.New("camera has no person detection")
	ErrUnknownEvent     = errors.New("detection event not found")
	ErrInvalidArmMode   = errors.New("invalid arm mode")
	ErrFrameUnavailable = errors.New("camera frame unavailable")
)
