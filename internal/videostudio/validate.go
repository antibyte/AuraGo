package videostudio

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FrameRateOption is one frame rate a project may use. NTSC variants exist for 24, 30 and 60 fps.
type FrameRateOption struct {
	FPS  int  `json:"fps"`
	NTSC bool `json:"ntsc,omitempty"`
}

var frameRateOptions = []FrameRateOption{{FPS: 24}, {FPS: 24, NTSC: true}, {FPS: 25}, {FPS: 30}, {FPS: 30, NTSC: true}, {FPS: 50}, {FPS: 60}, {FPS: 60, NTSC: true}}

// FrameRateOptions lists the frame rates Validate accepts, in menu order.
func FrameRateOptions() []FrameRateOption { return append([]FrameRateOption(nil), frameRateOptions...) }

// supportedFrameRates and ntscFrameRates are derived from frameRateOptions, the one table of accepted rates.
var supportedFrameRates, ntscFrameRates = func() (map[int]bool, map[int]bool) {
	rates, ntsc := make(map[int]bool, len(frameRateOptions)), make(map[int]bool)
	for _, option := range frameRateOptions {
		rates[option.FPS] = true
		if option.NTSC {
			ntsc[option.FPS] = true
		}
	}
	return rates, ntsc
}()

var sourceFPSPattern = regexp.MustCompile(`^[1-9][0-9]{0,6}/[1-9][0-9]{0,6}$`)

// Validate checks a version 2 project and all frame and overlap rules. Stored or submitted
// version 1 documents must pass through Upgrade first.
func Validate(project Project) error {
	if project.Version != CurrentVersion {
		return fmt.Errorf("unsupported project version %d", project.Version)
	}
	if strings.TrimSpace(project.Name) == "" || len([]rune(project.Name)) > 120 {
		return fmt.Errorf("project name must contain 1 to 120 characters")
	}
	if !supportedCanvas(project.Width, project.Height) {
		return fmt.Errorf("unsupported canvas size %dx%d", project.Width, project.Height)
	}
	if !supportedFrameRates[project.FPS] {
		return fmt.Errorf("unsupported frame rate %d fps", project.FPS)
	}
	if project.NTSC && !ntscFrameRates[project.FPS] {
		return fmt.Errorf("NTSC timing is available for 24, 30 and 60 fps only")
	}
	if len(project.Assets) > MaxAssetsPerProject {
		return fmt.Errorf("project exceeds %d assets", MaxAssetsPerProject)
	}
	maxFrames := MaxDurationFrames(project)

	assets := make(map[string]Asset, len(project.Assets))
	for _, asset := range project.Assets {
		if err := validateAsset(asset); err != nil {
			return err
		}
		if _, exists := assets[asset.ID]; exists {
			return fmt.Errorf("duplicate asset id %q", asset.ID)
		}
		assets[asset.ID] = asset
	}
	if err := validateProjectFolders(project.Folders); err != nil {
		return err
	}

	var videoTracks, audioTracks, overlayTracks, clipCount int
	linkCounts := make(map[string]int)
	trackIDs := make(map[string]struct{}, len(project.Tracks))
	clipIDs := make(map[string]struct{})
	for _, track := range project.Tracks {
		if !validIdentifier(track.ID) {
			return fmt.Errorf("track id is required and must be at most 128 characters")
		}
		if _, exists := trackIDs[track.ID]; exists {
			return fmt.Errorf("duplicate track id %q", track.ID)
		}
		trackIDs[track.ID] = struct{}{}
		if strings.TrimSpace(track.Name) == "" || len([]rune(track.Name)) > 120 {
			return fmt.Errorf("track %q must have a name of at most 120 characters", track.ID)
		}
		switch track.Kind {
		case TrackVideo:
			videoTracks++
		case TrackAudio:
			audioTracks++
		case TrackOverlay:
			overlayTracks++
		default:
			return fmt.Errorf("track %q has unsupported kind %q", track.ID, track.Kind)
		}
		clipCount += len(track.Clips)
		if clipCount > MaxClipsPerProject {
			return fmt.Errorf("project exceeds %d clips", MaxClipsPerProject)
		}

		clips := append([]Clip(nil), track.Clips...)
		sort.Slice(clips, func(i, j int) bool {
			if clips[i].Start == clips[j].Start {
				return clips[i].ID < clips[j].ID
			}
			return clips[i].Start < clips[j].Start
		})
		for i, clip := range clips {
			if err := validateClip(track, clip, assets, clipIDs, maxFrames); err != nil {
				return err
			}
			if clip.Link != "" {
				linkCounts[clip.Link]++
				if linkCounts[clip.Link] > MaxLinkClips {
					return fmt.Errorf("link group %q holds more than %d clips", clip.Link, MaxLinkClips)
				}
			}
			if clip.Transition != nil {
				if i+1 >= len(clips) {
					return fmt.Errorf("clip %q has a transition without a following clip", clip.ID)
				}
				next := clips[i+1]
				overlap := clip.Start + clip.Duration - next.Start
				if next.Start <= clip.Start || overlap <= 0 || overlap != clip.Transition.Duration {
					return fmt.Errorf("clip %q transition must match its explicit overlap with the next clip", clip.ID)
				}
				if overlap >= clip.Duration || overlap >= next.Duration {
					return fmt.Errorf("clip %q transition must be shorter than both clips", clip.ID)
				}
			}
			if i+1 < len(clips) {
				next := clips[i+1]
				if clip.Start+clip.Duration > next.Start && clip.Transition == nil {
					return fmt.Errorf("clips %q and %q overlap without a transition", clip.ID, next.ID)
				}
				if i+2 < len(clips) && clip.Start+clip.Duration > clips[i+2].Start {
					return fmt.Errorf("clip %q overlaps more than one following clip", clip.ID)
				}
			}
		}
	}
	if videoTracks > MaxVideoTracks || audioTracks > MaxAudioTracks || overlayTracks > MaxOverlayTracks {
		return fmt.Errorf("project exceeds track limits (%d video, %d audio, %d overlay)", MaxVideoTracks, MaxAudioTracks, MaxOverlayTracks)
	}
	if err := validateMarkers(project, maxFrames); err != nil {
		return err
	}
	if Duration(project) > maxFrames {
		return fmt.Errorf("project duration exceeds %d minutes", MaxDurationSeconds/60)
	}
	return nil
}

func validateAsset(asset Asset) error {
	if !validIdentifier(asset.ID) {
		return fmt.Errorf("asset id is required and must be at most 128 characters")
	}
	if asset.Name == "" || asset.Name != strings.TrimSpace(asset.Name) || len([]rune(asset.Name)) > 255 {
		return fmt.Errorf("asset %q must have a name of at most 255 characters", asset.ID)
	}
	if folder, err := NormalizeFolder(asset.Folder); err != nil || folder != asset.Folder {
		return fmt.Errorf("asset %q has an invalid folder", asset.ID)
	}
	if asset.DurationFrames < 0 || asset.Width < 0 || asset.Height < 0 || asset.DurationUs < 0 || asset.VideoDurationUs < 0 || asset.SizeBytes < 0 {
		return fmt.Errorf("asset %q has negative media metadata", asset.ID)
	}
	if asset.Width > MaxSourceDimension || asset.Height > MaxSourceDimension || int64(asset.Width)*int64(asset.Height) > MaxSourcePixels {
		return fmt.Errorf("asset %q exceeds the source dimension limit", asset.ID)
	}
	if asset.SourceFPS != "" && !sourceFPSPattern.MatchString(asset.SourceFPS) {
		return fmt.Errorf("asset %q has an invalid source frame rate", asset.ID)
	}
	if asset.Codec != safeCodecName(asset.Codec) {
		return fmt.Errorf("asset %q has an invalid codec name", asset.ID)
	}
	switch asset.Kind {
	case AssetVideo:
		if asset.DurationFrames == 0 || asset.Width == 0 || asset.Height == 0 {
			return fmt.Errorf("video asset %q is missing duration or dimensions", asset.ID)
		}
	case AssetAudio:
		if asset.DurationFrames == 0 || !asset.HasAudio {
			return fmt.Errorf("audio asset %q is missing duration or audio stream metadata", asset.ID)
		}
	case AssetImage:
		if asset.Width == 0 || asset.Height == 0 {
			return fmt.Errorf("image asset %q is missing dimensions", asset.ID)
		}
	default:
		return fmt.Errorf("asset %q has unsupported kind %q", asset.ID, asset.Kind)
	}
	return nil
}

func validateClip(track Track, clip Clip, assets map[string]Asset, clipIDs map[string]struct{}, maxFrames int) error {
	if !validIdentifier(clip.ID) {
		return fmt.Errorf("clip id is required and must be at most 128 characters")
	}
	if _, exists := clipIDs[clip.ID]; exists {
		return fmt.Errorf("duplicate clip id %q", clip.ID)
	}
	clipIDs[clip.ID] = struct{}{}
	asset, exists := assets[clip.AssetID]
	if !exists {
		return fmt.Errorf("clip %q references unknown asset %q", clip.ID, clip.AssetID)
	}
	if clip.Start < 0 || clip.Start > maxFrames || clip.Offset < 0 || clip.Duration <= 0 || clip.Duration > maxFrames || clip.Start > maxFrames-clip.Duration {
		return fmt.Errorf("clip %q has invalid frame timing", clip.ID)
	}
	if clip.Speed != 0 && (!finite(clip.Speed) || clip.Speed < MinClipSpeed || clip.Speed > MaxClipSpeed) {
		return fmt.Errorf("clip %q speed must be between 0.25 and 16", clip.ID)
	}
	speed := ClipSpeed(clip)
	if asset.Kind == AssetImage {
		if speed != 1 {
			return fmt.Errorf("image clip %q cannot change speed", clip.ID)
		}
	} else if float64(clip.Duration-1)*speed > float64(asset.DurationFrames-1-clip.Offset)+1e-9 {
		// Source extent rule: offset + (duration - 1) * speed <= duration_frames - 1. The room left in the
		// source is computed in integers so huge offsets cannot round into range.
		return fmt.Errorf("clip %q exceeds asset %q duration", clip.ID, asset.ID)
	}
	if clip.Link != "" && !validIdentifier(clip.Link) {
		return fmt.Errorf("clip %q has an invalid link id", clip.ID)
	}
	if clip.Label != "" && !slices.Contains(LabelColors, clip.Label) {
		return fmt.Errorf("clip %q has an unsupported label %q", clip.ID, clip.Label)
	}
	switch track.Kind {
	case TrackVideo:
		if asset.Kind != AssetVideo && asset.Kind != AssetImage {
			return fmt.Errorf("video track clip %q must reference a video or image asset", clip.ID)
		}
	case TrackOverlay:
		if asset.Kind != AssetImage {
			return fmt.Errorf("overlay track clip %q must reference an image asset", clip.ID)
		}
	case TrackAudio:
		if asset.Kind != AssetAudio && (asset.Kind != AssetVideo || !asset.HasAudio) {
			return fmt.Errorf("audio track clip %q must reference an asset with audio", clip.ID)
		}
	}
	if clip.AudioDetached && (track.Kind != TrackVideo || asset.Kind != AssetVideo || !asset.HasAudio) {
		return fmt.Errorf("clip %q can detach audio only on a video track with a video asset that has audio", clip.ID)
	}
	if track.Kind != TrackAudio {
		if !finite(clip.X) || !finite(clip.Y) || !finite(clip.Width) || !finite(clip.Height) ||
			clip.X < 0 || clip.Y < 0 || clip.Width <= 0 || clip.Height <= 0 ||
			clip.Width > 1 || clip.Height > 1 || clip.X+clip.Width > 1 || clip.Y+clip.Height > 1 {
			return fmt.Errorf("clip %q has invalid normalized geometry", clip.ID)
		}
		if !finite(clip.Rotation) || clip.Rotation < -360 || clip.Rotation > 360 {
			return fmt.Errorf("clip %q rotation must be between -360 and 360 degrees", clip.ID)
		}
		if !finite(clip.Opacity) || clip.Opacity < 0 || clip.Opacity > 1 {
			return fmt.Errorf("clip %q opacity must be between 0 and 1", clip.ID)
		}
		if clip.Fit != FitContain && clip.Fit != FitCover {
			return fmt.Errorf("clip %q fit must be contain or cover", clip.ID)
		}
	}
	if !finite(clip.Volume) || clip.Volume < 0 || clip.Volume > 4 {
		return fmt.Errorf("clip %q volume must be between 0 and 4", clip.ID)
	}
	if clip.FadeIn < 0 || clip.FadeOut < 0 || clip.FadeIn > clip.Duration || clip.FadeOut > clip.Duration {
		return fmt.Errorf("clip %q has invalid audio fade length", clip.ID)
	}
	if len([]rune(clip.Text)) > 4096 {
		return fmt.Errorf("clip %q text exceeds 4096 characters", clip.ID)
	}
	if clip.TextStyle != nil {
		if !finite(clip.TextStyle.FontSize) || clip.TextStyle.FontSize < 0 || clip.TextStyle.FontSize > 256 ||
			!finite(clip.TextStyle.OutlineWidth) || clip.TextStyle.OutlineWidth < 0 || clip.TextStyle.OutlineWidth > 32 ||
			!finite(clip.TextStyle.BackgroundOpacity) || clip.TextStyle.BackgroundOpacity < 0 || clip.TextStyle.BackgroundOpacity > 1 {
			return fmt.Errorf("clip %q has invalid text style values", clip.ID)
		}
		if clip.TextStyle.Alignment != "" && clip.TextStyle.Alignment != "left" && clip.TextStyle.Alignment != "center" && clip.TextStyle.Alignment != "right" {
			return fmt.Errorf("clip %q text alignment must be left, center, or right", clip.ID)
		}
	}
	if clip.Transition != nil {
		if track.Kind == TrackAudio {
			return fmt.Errorf("audio clip %q cannot have a video transition", clip.ID)
		}
		if clip.Transition.Duration <= 0 || clip.Transition.Duration > clip.Duration {
			return fmt.Errorf("clip %q has invalid transition duration", clip.ID)
		}
		switch clip.Transition.Type {
		case TransitionDissolve, TransitionBlack, TransitionWipeLeft, TransitionWipeRight:
		default:
			return fmt.Errorf("clip %q has unsupported transition %q", clip.ID, clip.Transition.Type)
		}
	}
	return nil
}

// safeCodecName returns a lowercase FFmpeg codec name, or "" for anything that is not one.
func safeCodecName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) > 32 {
		return ""
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return ""
		}
	}
	return name
}

// NormalizeFolder returns the canonical form of a project folder path: "" for the top level, otherwise one to
// MaxFolderDepth segments joined by "/". Each segment is trimmed, has one to MaxFolderSegmentRunes runes, no control
// characters and is not "." or ".."; the whole path has at most MaxFolderPathRunes runes.
func NormalizeFolder(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", nil
	}
	parts := strings.Split(text, "/")
	if len(parts) > MaxFolderDepth {
		return "", fmt.Errorf("folder path has more than %d levels", MaxFolderDepth)
	}
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("folder path has an empty or relative segment")
		}
		if utf8.RuneCountInString(part) > MaxFolderSegmentRunes {
			return "", fmt.Errorf("folder name exceeds %d characters", MaxFolderSegmentRunes)
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return "", fmt.Errorf("folder name contains a control character")
			}
		}
		parts[i] = part
	}
	path := strings.Join(parts, "/")
	if utf8.RuneCountInString(path) > MaxFolderPathRunes {
		return "", fmt.Errorf("folder path exceeds %d characters", MaxFolderPathRunes)
	}
	return path, nil
}

// validateProjectFolders checks project.folders: canonical, non-empty, unique and at most MaxProjectFolders.
func validateProjectFolders(folders []string) error {
	if len(folders) > MaxProjectFolders {
		return fmt.Errorf("project exceeds %d folders", MaxProjectFolders)
	}
	seen := make(map[string]struct{}, len(folders))
	for _, folder := range folders {
		canonical, err := NormalizeFolder(folder)
		if err != nil || canonical == "" || canonical != folder {
			return fmt.Errorf("project folder %q is invalid", folder)
		}
		if _, dup := seen[folder]; dup {
			return fmt.Errorf("duplicate project folder %q", folder)
		}
		seen[folder] = struct{}{}
	}
	return nil
}

func validIdentifier(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsRune(value, 0)
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// validateMarkers checks the marker list and the in/out points against the timeline limit. Markers are frames of the
// project rate: a point or a range must fit inside [0, maxFrames]; the in point lies before the timeline end, the out
// point lies after the start, and in < out when both are set.
func validateMarkers(project Project, maxFrames int) error {
	if len(project.Markers) > MaxMarkers {
		return fmt.Errorf("project exceeds %d markers", MaxMarkers)
	}
	ids := make(map[string]struct{}, len(project.Markers))
	for _, marker := range project.Markers {
		if !validIdentifier(marker.ID) {
			return fmt.Errorf("marker id is required and must be at most 128 characters")
		}
		if _, exists := ids[marker.ID]; exists {
			return fmt.Errorf("duplicate marker id %q", marker.ID)
		}
		ids[marker.ID] = struct{}{}
		if marker.Frame < 0 || marker.Frame > maxFrames || marker.Duration < 0 || marker.Duration > maxFrames-marker.Frame {
			return fmt.Errorf("marker %q has invalid frame timing", marker.ID)
		}
		if len([]rune(marker.Name)) > MaxMarkerNameRunes || len([]rune(marker.Note)) > MaxMarkerNoteRunes {
			return fmt.Errorf("marker %q name or note is too long", marker.ID)
		}
		if !validMarkerColor(marker.Color) {
			return fmt.Errorf("marker %q has an unsupported colour", marker.ID)
		}
	}
	if project.InPoint != nil && (*project.InPoint < 0 || *project.InPoint >= maxFrames) {
		return fmt.Errorf("in point is outside the timeline")
	}
	if project.OutPoint != nil && (*project.OutPoint < 1 || *project.OutPoint > maxFrames) {
		return fmt.Errorf("out point is outside the timeline")
	}
	if project.InPoint != nil && project.OutPoint != nil && *project.InPoint >= *project.OutPoint {
		return fmt.Errorf("in point must come before the out point")
	}
	return nil
}

// validMarkerColor accepts an empty colour or one of the label palette names (LabelColors).
func validMarkerColor(color string) bool {
	return color == "" || slices.Contains(LabelColors, color)
}
