package videostudio

type AssetKind string

const (
	AssetVideo AssetKind = "video"
	AssetAudio AssetKind = "audio"
	AssetImage AssetKind = "image"
)

type TrackKind string

const (
	TrackVideo   TrackKind = "video"
	TrackAudio   TrackKind = "audio"
	TrackOverlay TrackKind = "overlay"
)

type FitMode string

const (
	FitContain FitMode = "contain"
	FitCover   FitMode = "cover"
)

type TransitionType string

const (
	TransitionDissolve  TransitionType = "dissolve"
	TransitionBlack     TransitionType = "black"
	TransitionWipeLeft  TransitionType = "wipeleft"
	TransitionWipeRight TransitionType = "wiperight"
)

type Project struct {
	Version int     `json:"version"`
	Name    string  `json:"name"`
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	FPS     int     `json:"fps"`
	NTSC    bool    `json:"ntsc,omitempty"`
	Assets  []Asset `json:"assets"`
	Tracks  []Track `json:"tracks"`
}

// Asset metadata is server-owned. DurationUs is the source of truth; DurationFrames expresses it in the
// project's frame rate and is recomputed whenever the server canonicalizes a project.
type Asset struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Path            string    `json:"path,omitempty"`
	Kind            AssetKind `json:"kind"`
	DurationFrames  int       `json:"duration_frames"`
	DurationUs      int64     `json:"duration_us,omitempty"`
	VideoDurationUs int64     `json:"video_duration_us,omitempty"`
	Width           int       `json:"width,omitempty"`
	Height          int       `json:"height,omitempty"`
	HasAudio        bool      `json:"has_audio"`
	SourceFPS       string    `json:"source_fps,omitempty"`
	Codec           string    `json:"codec,omitempty"`
	SizeBytes       int64     `json:"size_bytes,omitempty"`
}

type Track struct {
	ID     string    `json:"id"`
	Kind   TrackKind `json:"kind"`
	Name   string    `json:"name"`
	Muted  bool      `json:"muted"`
	Hidden bool      `json:"hidden"`
	Locked bool      `json:"locked"`
	Clips  []Clip    `json:"clips"`
}

type Clip struct {
	ID         string      `json:"id"`
	AssetID    string      `json:"asset_id"`
	Start      int         `json:"start"`
	Offset     int         `json:"offset"`
	Duration   int         `json:"duration"`
	X          float64     `json:"x"`
	Y          float64     `json:"y"`
	Width      float64     `json:"width"`
	Height     float64     `json:"height"`
	Rotation   float64     `json:"rotation"`
	Opacity    float64     `json:"opacity"`
	Volume     float64     `json:"volume"`
	FadeIn     int         `json:"fade_in"`
	FadeOut    int         `json:"fade_out"`
	Fit        FitMode     `json:"fit"`
	Text       string      `json:"text,omitempty"`
	TextStyle  *TextStyle  `json:"text_style,omitempty"`
	Transition *Transition `json:"transition,omitempty"`
}

type TextStyle struct {
	FontFamily        string  `json:"font_family,omitempty"`
	FontSize          float64 `json:"font_size,omitempty"`
	Color             string  `json:"color,omitempty"`
	Bold              bool    `json:"bold,omitempty"`
	Italic            bool    `json:"italic,omitempty"`
	Alignment         string  `json:"alignment,omitempty"`
	OutlineColor      string  `json:"outline_color,omitempty"`
	OutlineWidth      float64 `json:"outline_width,omitempty"`
	BackgroundColor   string  `json:"background_color,omitempty"`
	BackgroundOpacity float64 `json:"background_opacity,omitempty"`
}

type Transition struct {
	Type     TransitionType `json:"type"`
	Duration int            `json:"duration"`
}

const (
	// FramesPerSecond is the version 1 frame rate and the default for new projects.
	FramesPerSecond     = 30
	CurrentVersion      = 2
	MaxDurationSeconds  = 3600
	MaxVideoTracks      = 8
	MaxAudioTracks      = 8
	MaxOverlayTracks    = 8
	MaxClipsPerProject  = 2000
	MaxAssetsPerProject = 1024
	// MaxSegmentInputs bounds the inputs of one FFmpeg process; the segment planner splits busier spans.
	MaxSegmentInputs   = 32
	MaxSourceDimension = 16384
	MaxSourcePixels    = 64_000_000
)

var canvasSizes = [][2]int{
	{1280, 720}, {1920, 1080}, {2560, 1440}, {3840, 2160},
	{720, 1280}, {1080, 1920}, {2160, 3840},
	{720, 720}, {1080, 1080}, {2160, 2160},
	{1080, 1350},
}

// CanvasSizes lists the supported canvas sizes in menu order.
func CanvasSizes() [][2]int { return append([][2]int(nil), canvasSizes...) }

func supportedCanvas(width, height int) bool {
	for _, size := range canvasSizes {
		if size[0] == width && size[1] == height {
			return true
		}
	}
	return false
}

// MaxDurationFrames is the longest timeline of a project, in its own frame rate.
func MaxDurationFrames(project Project) int {
	rate := ProjectRate(project)
	return int(int64(MaxDurationSeconds) * rate.Num / rate.Den)
}

// AssetAtRate expresses an asset's microsecond duration in frames of the given rate.
// Images and assets without a microsecond duration keep their frame count.
func AssetAtRate(asset Asset, rate Rate) Asset {
	if asset.Kind != AssetImage && asset.DurationUs > 0 {
		asset.DurationFrames = rate.FramesAt(asset.DurationUs)
	}
	return asset
}

// Duration returns the frame position immediately after the last clip.
func Duration(project Project) int {
	end := 0
	maxInt := int(^uint(0) >> 1)
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			if clip.Start < 0 || clip.Duration <= 0 {
				continue
			}
			if clip.Start > maxInt-clip.Duration {
				return maxInt
			}
			if clip.Start+clip.Duration > end {
				end = clip.Start + clip.Duration
			}
		}
	}
	return end
}
