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
// From and To are minutes since midnight in 0..1439 (config validation enforces
// the range); To < From crosses midnight and To == From covers 24 hours. See
// InWindow for how a window behaves across clock changes.
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

// Camera is one go2rtc stream whose stream and detection are both enabled. Those
// are the cameras in Settings.Cameras. Service.Test additionally accepts a Camera
// as an override (zones and threshold) for a stream that has no detection enabled
// yet, so the zone editor can try a layout before it is saved; without an override
// the camera must be one of Settings.Cameras.
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
	MaxStorageMB  int // MiB, see MaxStorageBytes
	Schedule      []Window
	Cameras       []Camera
	Location      *time.Location // server local time
	Language      string         // cfg.Server.UILanguage, for alert texts
}

// MaxStorageBytes is the size limit of stored frames in bytes. MaxStorageMB is
// MiB (1 MiB = 1<<20 bytes), not megabytes.
func (s Settings) MaxStorageBytes() int64 { return int64(s.MaxStorageMB) << 20 }

// FrameSource delivers the current JPEG frame of a camera.
type FrameSource interface {
	Frame(ctx context.Context, cameraID string) ([]byte, error)
}

// Detection-logic constants (spec "Detection logic").
const (
	EventStartWindow      = 3                // analyzed frames considered by the start rule
	EventStartHits        = 2                // hits within the window that start an event
	EventSingleHitScore   = 0.8              // a single hit at or above this score starts an event
	EventEndAfter         = 30 * time.Second // an event ends after this long without a hit (minimum; slow cameras wait longer, see EventEndAfterFor)
	EventFrameInterval    = 5 * time.Second  // at most one stored frame per interval while hits continue
	EventMaxFrames        = 20               // stored frames per event
	EventPendingHitFrames = 2                // hit frames kept in memory until the start decision
)

// EventEndAfterFor is the time without a hit after which an event of a camera
// that analyzes every interval ends: EventEndAfter, or two intervals plus 5 s
// when that is longer. With the plain 30 s a person in view of a camera that
// analyzes every 30 s would start and end an event on every other frame, and one
// missed detection would end the event of a camera at 15 s. The two intervals
// plus 5 s allow one missed detection and the fetch time in between; the value
// is EventEndAfter for intervals up to 12.5 s.
func EventEndAfterFor(interval time.Duration) time.Duration {
	return max(EventEndAfter, 2*interval+5*time.Second)
}

// Service defaults and timings.
const (
	DefaultCameraInterval = 2 * time.Second  // analysis interval of a camera that sets none (or a value <= 0)
	DefaultCameraMinScore = 0.5              // person score threshold of a camera that sets none
	DefaultRetentionDays  = 14               // days ended events are kept when retention is unset
	DefaultMaxStorageMB   = 2048             // MiB of stored frames kept when the limit is unset
	MaxFetchBackoff       = 60 * time.Second // upper bound of the frame-fetch backoff (interval x 2^failures)
	FrameFetchTimeout     = 15 * time.Second // timeout of one FrameSource.Frame request
	TestMinScore          = 0.3              // threshold of Service.Test: every box from this score up is returned and judged
	TimingAverageWindow   = 20               // detections averaged into DetectorStatus.WallMS and CPUMS
	ThumbnailWidth        = 320              // width in pixels of stored thumbnails (never scaled up)
	ThumbnailQuality      = 80               // JPEG quality of stored thumbnails
	RetentionInterval     = time.Hour        // period of the retention pass (it also runs at start and when the disk is full)
	ArmLogMinAge          = 24 * time.Hour   // arm-log rows younger than this are never pruned
	ArmCheckInterval      = 10 * time.Second // period at which the arming loop evaluates the schedule
	SweepInterval         = time.Second      // period of the sweep that ends events of cameras without recent hits
	StorageTargetRatio    = 0.9              // size retention deletes the oldest ended events down to this fraction of the limit
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
