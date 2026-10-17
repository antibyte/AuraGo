package videostudio

type AssetKind string

const (
	AssetVideo AssetKind = "video"
	AssetAudio AssetKind = "audio"
	AssetImage AssetKind = "image"
	// AssetLUT is a .cube colour lookup table (2c). It never sits on a track; lut effects reference it by id.
	AssetLUT AssetKind = "lut"
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
	Version  int      `json:"version"`
	Name     string   `json:"name"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	FPS      int      `json:"fps"`
	NTSC     bool     `json:"ntsc,omitempty"`
	Assets   []Asset  `json:"assets"`
	Tracks   []Track  `json:"tracks"`
	Markers  []Marker `json:"markers,omitempty"`
	InPoint  *int     `json:"in_point,omitempty"`
	OutPoint *int     `json:"out_point,omitempty"`
	Folders  []string `json:"folders,omitempty"`
}

// Marker is a named point (Duration 0) or range on the timeline. Markers never render; they guide editing.
// Colours come from the label palette (LabelColors) or are empty.
type Marker struct {
	ID       string `json:"id"`
	Frame    int    `json:"frame"`
	Duration int    `json:"duration,omitempty"`
	Name     string `json:"name,omitempty"`
	Color    string `json:"color,omitempty"`
	Note     string `json:"note,omitempty"`
}

// Asset metadata is server-owned. DurationUs is the source of truth; DurationFrames expresses it in the
// project's frame rate and is recomputed whenever the server canonicalizes a project.
type Asset struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Folder          string    `json:"folder,omitempty"`
	Path            string    `json:"path,omitempty"`
	Kind            AssetKind `json:"kind"`
	DurationFrames  int       `json:"duration_frames"`
	DurationUs      int64     `json:"duration_us,omitempty"`
	VideoDurationUs int64     `json:"video_duration_us,omitempty"`
	Width           int       `json:"width,omitempty"`
	Height          int       `json:"height,omitempty"`
	HasAudio        bool      `json:"has_audio"`
	HasPoster       bool      `json:"has_poster,omitempty"`
	SourceFPS       string    `json:"source_fps,omitempty"`
	Codec           string    `json:"codec,omitempty"`
	LUTSize         int       `json:"lut_size,omitempty"` // grid size N of a lut asset (2c), 0 for every other kind
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

// Clip is one item on a track. Model version 3 (phase 2) adds optional fields after Label in this fixed order, which
// is also the JSON order and the order of canonicalClipExtras in video-studio-core.js: keyframes (2b), ramp (2b),
// crop (2c), flip_h (2c), flip_v (2c), blend (2d), mask (2d), effects (2c), anim_in (2e), anim_out (2e).
type Clip struct {
	ID            string                `json:"id"`
	AssetID       string                `json:"asset_id"`
	Start         int                   `json:"start"`
	Offset        int                   `json:"offset"`
	Duration      int                   `json:"duration"`
	X             float64               `json:"x"`
	Y             float64               `json:"y"`
	Width         float64               `json:"width"`
	Height        float64               `json:"height"`
	Rotation      float64               `json:"rotation"`
	Opacity       float64               `json:"opacity"`
	Volume        float64               `json:"volume"`
	FadeIn        int                   `json:"fade_in"`
	FadeOut       int                   `json:"fade_out"`
	Fit           FitMode               `json:"fit"`
	Text          string                `json:"text,omitempty"`
	TextStyle     *TextStyle            `json:"text_style,omitempty"`
	Transition    *Transition           `json:"transition,omitempty"`
	Speed         float64               `json:"speed,omitempty"`
	Link          string                `json:"link,omitempty"`
	AudioDetached bool                  `json:"audio_detached,omitempty"`
	Disabled      bool                  `json:"disabled,omitempty"`
	Label         string                `json:"label,omitempty"`
	Keyframes     map[string][]Keyframe `json:"keyframes,omitempty"` // base paths (2b: x, y, width, height, rotation, opacity; 2c: crop.l|t|r|b)
	Ramp          []RampPoint           `json:"ramp,omitempty"`
	Crop          *Crop                 `json:"crop,omitempty"` // fractions of the uncropped, unflipped source (2c)
	FlipH         bool                  `json:"flip_h,omitempty"`
	FlipV         bool                  `json:"flip_v,omitempty"`
	Effects       []EffectInstance      `json:"effects,omitempty"`
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

// Crop trims a clip's source picture (2c, spec §5.3). Each side is a fraction of the uncropped, unflipped source, from 0
// to 0.95, and l+r and t+b are at most 0.95. No crop is nil: Validate refuses an all-zero crop, so the canonical form
// stays unique. The JSON always holds all four keys, in the order l, t, r, b.
type Crop struct {
	L float64 `json:"l"`
	T float64 `json:"t"`
	R float64 `json:"r"`
	B float64 `json:"b"`
}

// EffectInstance is one entry of a clip's effect stack (2c, spec §5.4); the stack order is the processing order.
//   - ID names a registry effect. An entry with On false is kept and validated, but not rendered.
//   - Params maps parameter ids to float64 (number), string (lut ref, choice, colour) or bool values. It is never nil:
//     Validate refuses a missing or null params. A missing parameter means the registry default.
//   - Keyframes maps keyframable number parameters to their keys, in source time like Clip.Keyframes.
type EffectInstance struct {
	ID        string                `json:"id"`
	On        bool                  `json:"on"`
	Params    map[string]any        `json:"params"`
	Keyframes map[string][]Keyframe `json:"keyframes,omitempty"`
}

const (
	// FramesPerSecond is the version 1 frame rate and the default for new projects.
	FramesPerSecond     = 30
	CurrentVersion      = 3
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
	// MaxProjectFolders bounds project.folders; empty folders persist there.
	MaxProjectFolders     = 128
	MaxFolderDepth        = 4
	MaxFolderSegmentRunes = 64
	MaxFolderPathRunes    = 255
	// MinClipSpeed and MaxClipSpeed bound Clip.Speed; an absent speed (0) plays at 1.
	MinClipSpeed = 0.25
	MaxClipSpeed = 16
	// MaxLinkClips bounds one link group.
	MaxLinkClips = 16
	// MaxAssetNameRunes bounds an asset's display name.
	MaxAssetNameRunes = 255
	// MaxEffectsPerClip bounds a clip's effect stack (spec §5.8).
	MaxEffectsPerClip = 8
)

// Marker limits (spec §4.1).
const (
	MaxMarkers         = 500
	MaxMarkerNameRunes = 120
	MaxMarkerNoteRunes = 2000
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

// BackfillDurationUs derives the microsecond duration of a version 1 asset from its whole frames at 30 fps. Images
// and assets that already carry a duration are returned unchanged. Upgrade and the server use this one function.
func BackfillDurationUs(asset Asset) Asset {
	if asset.DurationUs == 0 && asset.DurationFrames > 0 && asset.Kind != AssetImage {
		asset.DurationUs = int64(asset.DurationFrames) * 1_000_000 / FramesPerSecond
	}
	return asset
}

// LabelColors are the clip label colors in menu order; a clip without a label has "".
var LabelColors = []string{"red", "orange", "yellow", "green", "teal", "blue", "purple", "pink"}

// ClipSpeed returns the playback speed of a clip; an absent speed is 1.
func ClipSpeed(clip Clip) float64 {
	if clip.Speed == 0 {
		return 1
	}
	return clip.Speed
}

// EnabledDuration is the frame position after the last enabled clip. Export ranges end there; Duration keeps
// counting disabled clips because they still obey the limits.
func EnabledDuration(project Project) int {
	trimmed := project
	trimmed.Tracks = make([]Track, len(project.Tracks))
	for i, track := range project.Tracks {
		trimmed.Tracks[i] = track
		trimmed.Tracks[i].Clips = nil
		for _, clip := range track.Clips {
			if !clip.Disabled {
				trimmed.Tracks[i].Clips = append(trimmed.Tracks[i].Clips, clip)
			}
		}
	}
	return Duration(trimmed)
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
