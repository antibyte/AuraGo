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
	Assets  []Asset `json:"assets"`
	Tracks  []Track `json:"tracks"`
}

type Asset struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Path           string    `json:"path,omitempty"`
	Kind           AssetKind `json:"kind"`
	DurationFrames int       `json:"duration_frames"`
	Width          int       `json:"width,omitempty"`
	Height         int       `json:"height,omitempty"`
	HasAudio       bool      `json:"has_audio"`
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
	FramesPerSecond     = 30
	MaxDurationFrames   = 10 * 60 * FramesPerSecond
	MaxVideoTracks      = 4
	MaxAudioTracks      = 4
	MaxOverlayTracks    = 4
	MaxClipsPerProject  = 256
	MaxAssetsPerProject = 512
	MaxRenderInputs     = 32
	MaxSourceDimension  = 16384
	MaxSourcePixels     = 64_000_000
)

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
