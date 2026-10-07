package videostudio

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

var supportedCanvasSizes = map[[2]int]struct{}{
	{1280, 720}: {}, {1920, 1080}: {},
	{720, 1280}: {}, {1080, 1920}: {},
	{720, 720}: {}, {1080, 1080}: {},
}

// Validate checks the persisted project model and all frame and overlap rules.
func Validate(project Project) error {
	if project.Version != 1 {
		return fmt.Errorf("unsupported project version %d", project.Version)
	}
	if strings.TrimSpace(project.Name) == "" || len([]rune(project.Name)) > 120 {
		return fmt.Errorf("project name must contain 1 to 120 characters")
	}
	if _, ok := supportedCanvasSizes[[2]int{project.Width, project.Height}]; !ok {
		return fmt.Errorf("unsupported canvas size %dx%d", project.Width, project.Height)
	}
	if project.FPS != FramesPerSecond {
		return fmt.Errorf("project frame rate must be %d fps", FramesPerSecond)
	}
	if len(project.Assets) > MaxAssetsPerProject {
		return fmt.Errorf("project exceeds %d assets", MaxAssetsPerProject)
	}

	assets := make(map[string]Asset, len(project.Assets))
	for _, asset := range project.Assets {
		if !validIdentifier(asset.ID) {
			return fmt.Errorf("asset id is required and must be at most 128 characters")
		}
		if _, exists := assets[asset.ID]; exists {
			return fmt.Errorf("duplicate asset id %q", asset.ID)
		}
		if strings.TrimSpace(asset.Name) == "" || len([]rune(asset.Name)) > 255 {
			return fmt.Errorf("asset %q must have a name of at most 255 characters", asset.ID)
		}
		if asset.DurationFrames < 0 || asset.Width < 0 || asset.Height < 0 {
			return fmt.Errorf("asset %q has negative media metadata", asset.ID)
		}
		if asset.Width > MaxSourceDimension || asset.Height > MaxSourceDimension || int64(asset.Width)*int64(asset.Height) > MaxSourcePixels {
			return fmt.Errorf("asset %q exceeds the source dimension limit", asset.ID)
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
		assets[asset.ID] = asset
	}

	var videoTracks, audioTracks, overlayTracks, clipCount int
	trackIDs := make(map[string]struct{}, len(project.Tracks))
	clipIDs := make(map[string]struct{})
	renderAssets := make(map[string]struct{})
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
			if err := validateClip(track, clip, assets, clipIDs); err != nil {
				return err
			}
			asset := assets[clip.AssetID]
			visualUsed := track.Kind != TrackAudio && !track.Hidden
			audioUsed := !track.Hidden && !track.Muted && clip.Volume > 0 &&
				((track.Kind == TrackAudio && !track.Hidden) || (track.Kind == TrackVideo && asset.HasAudio))
			if visualUsed || audioUsed {
				renderAssets[clip.AssetID] = struct{}{}
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
	if len(renderAssets) > MaxRenderInputs {
		return fmt.Errorf("project uses more than %d distinct render inputs", MaxRenderInputs)
	}
	if Duration(project) > MaxDurationFrames {
		return fmt.Errorf("project duration exceeds ten minutes")
	}
	return nil
}

func validateClip(track Track, clip Clip, assets map[string]Asset, clipIDs map[string]struct{}) error {
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
	if clip.Start < 0 || clip.Start > MaxDurationFrames || clip.Offset < 0 || clip.Duration <= 0 || clip.Duration > MaxDurationFrames || clip.Start > MaxDurationFrames-clip.Duration {
		return fmt.Errorf("clip %q has invalid frame timing", clip.ID)
	}
	if asset.Kind != AssetImage && (clip.Offset > asset.DurationFrames || clip.Duration > asset.DurationFrames-clip.Offset) {
		return fmt.Errorf("clip %q exceeds asset %q duration", clip.ID, asset.ID)
	}
	switch track.Kind {
	case TrackVideo:
		if asset.Kind != AssetVideo {
			return fmt.Errorf("video track clip %q must reference a video asset", clip.ID)
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

func validIdentifier(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsRune(value, 0)
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
