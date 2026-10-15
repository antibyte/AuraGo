package videostudio

import (
	"fmt"
	"sort"
)

// Segment is a timeline range [Start, End) in project frames.
type Segment struct {
	Start int
	End   int
}

// span forbids segment boundaries strictly between start and end.
type span struct{ start, end int }

// PlanVideoSegments splits [start, end) into segments of about target frames. Boundaries avoid transitions
// so segments rarely need pre-roll, and each segment opens at most MaxSegmentInputs clip inputs.
func PlanVideoSegments(project Project, start, end, target int) ([]Segment, error) {
	spans := videoForbiddenSpans(project)
	assets := assetMap(project)
	count := func(seg Segment) int {
		return countClips(project, assets, expandRange(seg, spans), func(track Track, _ Clip, _ Asset) bool { return visualClip(track) })
	}
	return planSegments(start, end, target, spans, clipEdges(project), count)
}

// PlanAudioSegments splits [start, end) like PlanVideoSegments; boundaries avoid audio fades.
func PlanAudioSegments(project Project, start, end, target int) ([]Segment, error) {
	assets := assetMap(project)
	spans := audioForbiddenSpans(project, assets)
	count := func(seg Segment) int { return countClips(project, assets, expandRange(seg, spans), audibleClip) }
	return planSegments(start, end, target, spans, clipEdges(project), count)
}

func planSegments(start, end, target int, spans []span, edges []int, inputs func(Segment) int) ([]Segment, error) {
	if end <= start {
		return nil, fmt.Errorf("render range %d–%d is empty", start, end)
	}
	if target <= 0 {
		target = end - start
	}
	var cuts []int
	for f := start + target; f < end; {
		g := f
		for {
			next, inside := containingSpan(spans, g)
			if !inside {
				break
			}
			g = next
		}
		if g >= end {
			break
		}
		cuts = append(cuts, g)
		f = g + target
	}
	var out []Segment
	var split func(Segment) error
	split = func(seg Segment) error {
		if inputs(seg) <= MaxSegmentInputs {
			out = append(out, seg)
			return nil
		}
		cut, ok := splitPoint(seg, spans, edges)
		if !ok {
			return fmt.Errorf("%w between frames %d and %d", ErrTooManyInputs, seg.Start, seg.End)
		}
		if err := split(Segment{seg.Start, cut}); err != nil {
			return err
		}
		return split(Segment{cut, seg.End})
	}
	previous := start
	for _, cut := range append(cuts, end) {
		if err := split(Segment{previous, cut}); err != nil {
			return nil, err
		}
		previous = cut
	}
	return out, nil
}

// containingSpan returns the end of a span that strictly contains frame.
func containingSpan(spans []span, frame int) (int, bool) {
	for _, s := range spans {
		if s.start < frame && frame < s.end {
			return s.end, true
		}
	}
	return 0, false
}

// splitPoint prefers an allowed frame (the middle or a clip edge) closest to the middle of seg. Without one it
// splits in the middle anyway; the renderers add pre-roll around spans.
func splitPoint(seg Segment, spans []span, edges []int) (int, bool) {
	mid := seg.Start + (seg.End-seg.Start)/2
	if mid <= seg.Start {
		return 0, false
	}
	best, found := 0, false
	consider := func(frame int) {
		if frame <= seg.Start || frame >= seg.End {
			return
		}
		if _, inside := containingSpan(spans, frame); inside {
			return
		}
		if !found || absInt(frame-mid) < absInt(best-mid) {
			best, found = frame, true
		}
	}
	consider(mid)
	for _, edge := range edges {
		consider(edge)
	}
	if !found {
		return mid, true
	}
	return best, true
}

// expandRange widens seg until neither boundary lies strictly inside a span.
func expandRange(seg Segment, spans []span) Segment {
	for changed := true; changed; {
		changed = false
		for _, s := range spans {
			if s.start < seg.Start && seg.Start < s.end {
				seg.Start, changed = s.start, true
			}
			if s.start < seg.End && seg.End < s.end {
				seg.End, changed = s.end, true
			}
		}
	}
	return seg
}

func videoForbiddenSpans(project Project) []span {
	var spans []span
	for _, track := range project.Tracks {
		if !visualClip(track) {
			continue
		}
		clips := sortedClips(track.Clips)
		for i, clip := range clips {
			if clip.Transition != nil && i+1 < len(clips) {
				spans = append(spans, span{clips[i+1].Start, clip.Start + clip.Duration})
			}
		}
	}
	return spans
}

func audioForbiddenSpans(project Project, assets map[string]Asset) []span {
	var spans []span
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			if !audibleClip(track, clip, assets[clip.AssetID]) {
				continue
			}
			end := clip.Start + clip.Duration
			if clip.FadeIn > 0 {
				spans = append(spans, span{clip.Start, clip.Start + clip.FadeIn})
			}
			if clip.FadeOut > 0 {
				spans = append(spans, span{end - clip.FadeOut, end})
			}
		}
	}
	return spans
}

func countClips(project Project, assets map[string]Asset, window Segment, include func(Track, Clip, Asset) bool) int {
	count := 0
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			if clip.Start < window.End && clip.Start+clip.Duration > window.Start && include(track, clip, assets[clip.AssetID]) {
				count++
			}
		}
	}
	return count
}

func clipEdges(project Project) []int {
	var edges []int
	for _, track := range project.Tracks {
		for _, clip := range track.Clips {
			edges = append(edges, clip.Start, clip.Start+clip.Duration)
		}
	}
	sort.Ints(edges)
	return edges
}

func assetMap(project Project) map[string]Asset {
	assets := make(map[string]Asset, len(project.Assets))
	for _, asset := range project.Assets {
		assets[asset.ID] = asset
	}
	return assets
}

// visualClip reports whether clips on track are composed into the picture.
func visualClip(track Track) bool { return track.Kind != TrackAudio && !track.Hidden }

// audibleClip mirrors the mix rules: visible, unmuted tracks; overlay tracks never play sound.
func audibleClip(track Track, clip Clip, asset Asset) bool {
	if track.Hidden || track.Muted || track.Kind == TrackOverlay || clip.Volume <= 0 || !asset.HasAudio {
		return false
	}
	return track.Kind == TrackVideo || asset.Kind == AssetAudio || asset.Kind == AssetVideo
}

func sortedClips(clips []Clip) []Clip {
	sorted := append([]Clip(nil), clips...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Start == sorted[j].Start {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Start < sorted[j].Start
	})
	return sorted
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
