package videostudio

import (
	"errors"
	"fmt"
)

// MaxVideoAspect bounds the shape of a video source: its longer side may be at most this many times its shorter side.
// Cover-fit scales a source until it covers the clip box, so an extreme shape makes an intermediate frame that is many
// boxes wide: a 16384x16 video covering a 1920x1080 box is scaled to about 1.1 million pixels wide, gigabytes per frame.
// With the rule, the intermediate of a static cover clip is at most this many boxes long on one side. Stills are exempt,
// because a title is a cropped text block that can be far wider than tall; keyed cover clips clamp the picture aspect
// themselves (maxCoverAspect).
const MaxVideoAspect = 16

// maxCoverAspect clamps the picture aspect of a cover clip with keyframed geometry, in both directions (1/16 to 16). It
// is the import limit, so every video that can be imported keeps its own aspect; only a source beyond the limit (a still,
// or a video imported before the rule) and the boxes of a clip that animates to a sliver are held to it. coverBound
// documents the effect.
const maxCoverAspect = MaxVideoAspect

// ErrSourceAspect reports a video source whose longer side is more than MaxVideoAspect times its shorter side: at import
// (Probe) and when an export, a range or a frame shows such a video imported before the rule.
var ErrSourceAspect = errors.New("video aspect ratio is too extreme")

// validVideoAspect reports whether a video of this size obeys the aspect rule.
func validVideoAspect(width, height int) bool {
	return width > 0 && height > 0 && int64(max(width, height)) <= MaxVideoAspect*int64(min(width, height))
}

// checkSourceAspects refuses a render of the timeline range [start, end) that shows a video asset beyond the aspect rule.
// The asset sizes come from the project, so the check runs before any source is probed or any FFmpeg process starts.
// Disabled clips, audio tracks, hidden tracks and clips outside the range do not count, and Validate keeps accepting the
// project: the asset stays in the library and the rest of the timeline exports.
func checkSourceAspects(project Project, start, end int) error {
	assets := assetMap(project)
	for _, track := range project.Tracks {
		if !visualClip(track) {
			continue
		}
		for _, clip := range track.Clips {
			if clip.Disabled || clip.Start >= end || clip.Start+clip.Duration <= start {
				continue
			}
			asset, ok := assets[clip.AssetID]
			if !ok || asset.Kind != AssetVideo || asset.Width <= 0 || asset.Height <= 0 || validVideoAspect(asset.Width, asset.Height) {
				continue
			}
			return fmt.Errorf("%w: %q is %dx%d, its longer side may be at most %d times the shorter one", ErrSourceAspect, asset.Name, asset.Width, asset.Height, MaxVideoAspect)
		}
	}
	return nil
}
