package videostudio

import (
	"errors"
	"fmt"
)

// MaxVideoAspect bounds the shape of a video source: its longer side may be at most this many times its shorter side.
// Cover-fit scales a source until it covers the clip box, so an extreme shape makes an intermediate frame that is many
// boxes wide: a 16384x16 video covering a 1920x1080 box is scaled to about 1.1 million pixels wide, gigabytes per frame.
// With the rule, the intermediate of a static cover clip is at most this many boxes long on one side. Stills are exempt,
// because a title is a cropped text block that can be far wider than tall; cover clips clamp the picture aspect
// themselves: the keyed ones (maxCoverAspect) and the static ones (staticCoverClamp).
const MaxVideoAspect = 16

// maxCoverAspect clamps the picture aspect of a cover clip with keyframed geometry, in both directions (1/16 to 16). It
// is the import limit, so every video that can be imported keeps its own aspect; only a source beyond the limit (a still,
// or a video imported before the rule) and the boxes of a clip that animates to a sliver are held to it. coverBound
// documents the effect. A static cover clip uses the same factor around its one box aspect (staticCoverClamp).
const maxCoverAspect = MaxVideoAspect

// minClampSide is the fewest pixels the clamp crop of a static cover clip leaves across. A source of a few pixels in a
// box a few pixels wide would otherwise be cropped to nothing (a 10x10 picture in a 2x1080 box: 10*(16*2/1080) < 1 px),
// and FFmpeg refuses a crop without area.
const minClampSide = 2

// aspectClampCrop is the crop filter that holds the picture aspect to low..high: it trims the longer side around the
// centre (crop's default position) and leaves a picture inside the range as it is, because both min() terms then give
// the picture's own size. FFmpeg evaluates the terms on the frames it receives, so the crop is right in whatever
// orientation autorotation delivers. A minSide above zero keeps at least that many pixels across. Keyed cover clips
// (animatedTransform) and static ones (staticCoverClamp) share it.
func aspectClampCrop(low, high float64, minSide int) string {
	width, height := "ih*"+exprNum(high).String(), "iw/"+exprNum(low).String()
	if minSide > 0 {
		width, height = fmt.Sprintf("max(%s,%d)", width, minSide), fmt.Sprintf("max(%s,%d)", height, minSide)
	}
	return fmt.Sprintf("crop=w='min(iw,%s)':h='min(ih,%s)'", width, height)
}

// staticCoverClamp returns the crop that a static cover clip puts in front of its cover scale, or "" when the clip
// needs none. Cover scales the source until the box is covered, so a source whose aspect is more than maxCoverAspect
// times the box aspect, either way round, made an intermediate of any size: a 16384x16 still in a 1920x1080 box was
// scaled to about 1.1 million pixels wide. The crop holds the source aspect to within that factor of the box aspect, so
// each side of the intermediate is at most maxCoverAspect box sides (plus rounding), and the box shows the same picture:
// cover trims what the crop removes. A source within the factor keeps its chain; so does a source of unknown size
// (sourceAspect 0).
//
// Both orientations count, as in coverBound: FFmpeg's autorotation may turn the source (a phone video, a JPEG with an
// EXIF orientation), and the asset keeps the size it was stored with. The crop is a no-op for the orientation whose
// aspect is within the factor.
func staticCoverClamp(boxWidth, boxHeight int, sourceAspect float64) string {
	if sourceAspect <= 0 || boxWidth <= 0 || boxHeight <= 0 {
		return ""
	}
	box := float64(boxWidth) / float64(boxHeight)
	low, high := box/maxCoverAspect, box*maxCoverAspect
	within := func(aspect float64) bool { return aspect >= low && aspect <= high }
	if within(sourceAspect) && within(1/sourceAspect) {
		return ""
	}
	return aspectClampCrop(low, high, minClampSide)
}

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
