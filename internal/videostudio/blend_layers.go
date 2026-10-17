package videostudio

import (
	"fmt"
	"slices"
	"strings"
)

// Blend modes in the export (plan 2d, index §4.10 and §4.22). A clip's blend mode decides how its track layer
// composites onto the layers below it. The clip chain is the same for every mode (place still overlays the clip on a
// transparent canvas-size layer), and buildVisualLayer joins the clips as before.
//
// Per track layer and segment window:
//   - Every window frame has one effective mode:
//     - a clip's own frames take its mode;
//     - the D overlap frames of a transition P → N take blend(P) when P and N share it, else normal;
//     - transparent frames (gaps, the frames after the last clip) join the run before them, else the run after.
//       Compositing a transparent frame is the identity in every mode.
//
//     blendRuns returns the runs of one mode.
//   - A layer whose runs are all normal keeps the 2c overlay, byte for byte (normalComposite).
//   - Otherwise the layer is padded to the window and split into one sublayer per mode (splitBlendLayers):
//     - a sublayer holds the frames of its mode's runs and transparent frames elsewhere;
//     - so it is exactly the window long, and blend's end-of-stream handling never decides a frame.
//
//     The sublayers composite onto the base in turn: normal with the overlay first, then the other modes in registry
//     order (blendComposite).
//
// FFmpeg mode names come from the registry (BlendModeByID), never from the project. No stage here takes runtime
// commands: blend and maskedmerge are two-input framesync filters (index §4.11).
//
// Labels are vs_bl_<L>_<n>_<part> for track layer L:
//   - n = 0: the pad of a layer that ends before the window (tail, window).
//   - n = r ≥ 1 for run r:
//     - its split output (in);
//     - its trimmed frames (run);
//     - the transparent stretch of sublayer m that starts at run r (gap<m>).
//   - n = m ≥ 1 for sublayer m:
//     - the sublayer (sub);
//     - its composite (b0, b1, l0, l1, lg, la, e);
//     - the composite's output when another sublayer follows (out). The last composite ends in vs_base<L+1>.

// blendRun is a window-local frame range [start, end) of one track layer with one effective blend mode (a registry id).
type blendRun struct {
	mode       string
	start, end int
}

// blendLayer is one sublayer of a track layer: the frames of one mode's runs, transparent elsewhere, the window long.
type blendLayer struct {
	mode, label string
}

// clipBlendMode is the registry id the export composites a clip with: its blend mode, or normal when the field is
// absent. A mode the registry does not know, or one without an FFmpeg blend name, counts as normal. Validate refuses
// both, so this only keeps the graph well-formed.
func clipBlendMode(clip Clip) string {
	if clip.Blend == "" {
		return BlendNormal
	}
	if spec, ok := BlendModeByID(clip.Blend); ok && spec.Blend != "" {
		return spec.ID
	}
	return BlendNormal
}

// blendRuns partitions the window frames [0, frames) of one track layer into maximal runs of one effective mode (index
// §4.22). The runs are contiguous, cover the whole window and differ from their neighbours in mode. A layer without
// visible frames is one normal run.
//
// The window is cut at every clip edge and transition overlap edge. Between two cuts the mode is constant, and
// blendModeAt gives it.
func blendRuns(layer []layerClip, frames int) []blendRun {
	frames = max(frames, 0)
	cuts := []int{0, frames}
	for i, item := range layer {
		cuts = append(cuts, item.clip.Start, item.clip.Start+item.clip.Duration)
		if item.clip.Transition != nil && i+1 < len(layer) {
			start := layer[i+1].clip.Start
			cuts = append(cuts, start, start+item.clip.Transition.Duration)
		}
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	var runs []blendRun
	for j := 0; j+1 < len(cuts); j++ {
		start, end := cuts[j], cuts[j+1]
		if start < 0 || end > frames {
			continue
		}
		mode := blendModeAt(layer, start)
		if mode == "" {
			if len(runs) > 0 {
				runs[len(runs)-1].end = end // transparent frames join the run before them
			}
			continue
		}
		if n := len(runs); n > 0 && runs[n-1].mode == mode {
			runs[n-1].end = end
			continue
		}
		runs = append(runs, blendRun{mode: mode, start: start, end: end})
	}
	if len(runs) == 0 {
		return []blendRun{{mode: BlendNormal, start: 0, end: frames}}
	}
	runs[0].start = 0 // leading transparent frames join the run after them
	return runs
}

// blendModeAt is the effective mode of one window frame of a track layer, or "" when no clip covers it. A frame in the
// overlap of a kept transition P → N takes overlapBlend(blend(P), blend(N)): the shared mode, else normal.
func blendModeAt(layer []layerClip, frame int) string {
	for i := 0; i+1 < len(layer); i++ {
		transition := layer[i].clip.Transition
		if start := layer[i+1].clip.Start; transition != nil && frame >= start && frame < start+transition.Duration {
			return overlapBlend(clipBlendMode(layer[i].whole), clipBlendMode(layer[i+1].whole))
		}
	}
	for _, item := range layer {
		if frame >= item.clip.Start && frame < item.clip.Start+item.clip.Duration {
			return clipBlendMode(item.whole)
		}
	}
	return ""
}

// layerEnd is the window frame where a track layer's stream ends. buildVisualLayer joins the clips up to the end of
// the last one, which may lie before the end of the window.
func layerEnd(layer []layerClip) int {
	end := 0
	for _, item := range layer {
		end = max(end, item.clip.Start+item.clip.Duration)
	}
	return end
}

// blendModeOrder lists the modes of runs once each, in registry order. registryBlendModes starts with normal, so
// normal comes first.
func blendModeOrder(runs []blendRun) []string {
	present := make(map[string]bool, len(runs))
	for _, run := range runs {
		present[run.mode] = true
	}
	order := make([]string, 0, len(present))
	for _, spec := range registryBlendModes {
		if present[spec.ID] {
			order = append(order, spec.ID)
		}
	}
	return order
}

// splitBlendLayers cuts the track layer label into one sublayer per mode of runs, in blendModeOrder. The layer is
// layerFrames long and starts at window frame 0; each sublayer is the window (frames) long:
//   - A layer that ends before the window is first padded with transparent frames (concatPieces).
//   - A single run needs no split: the (padded) layer is its sublayer.
//   - Otherwise split gives every run its own trim of the layer. Each mode's sublayer concatenates its runs with
//     transparent stretches for the runs of the other modes.
func splitBlendLayers(label string, layerFrames int, runs []blendRun, trackIndex, canvasWidth, canvasHeight, frames int, rate Rate) ([]string, []blendLayer) {
	name := func(n int, part string) string { return fmt.Sprintf("vs_bl_%d_%d_%s", trackIndex, n, part) }
	timebase := fmt.Sprintf("settb=%d/%d", rate.Den, rate.Num)
	var filters []string
	source := label
	if layerFrames < frames {
		tail, window := name(0, "tail"), name(0, "window")
		filters = append(filters,
			transparentGapFilter(tail, frames-layerFrames, canvasWidth, canvasHeight, rate),
			concatPieces([]string{label, tail}, []int{layerFrames, frames - layerFrames}, timebase, window))
		source = window
	}
	if len(runs) == 1 {
		return filters, []blendLayer{{mode: runs[0].mode, label: source}}
	}
	split := fmt.Sprintf("[%s]split=%d", source, len(runs))
	pieces := make([]string, len(runs))
	for r := range runs {
		split += "[" + name(r+1, "in") + "]"
		pieces[r] = name(r+1, "run")
	}
	filters = append(filters, split)
	for r, run := range runs {
		filters = append(filters, fmt.Sprintf("[%s]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS[%s]", name(r+1, "in"), run.start, run.end, pieces[r]))
	}
	modes := blendModeOrder(runs)
	layers := make([]blendLayer, 0, len(modes))
	for m, mode := range modes {
		var labels []string
		var lengths []int
		for r := 0; r < len(runs); {
			if runs[r].mode == mode {
				labels, lengths = append(labels, pieces[r]), append(lengths, runs[r].end-runs[r].start)
				r++
				continue
			}
			first := r
			for r < len(runs) && runs[r].mode != mode {
				r++
			}
			gap, length := name(first+1, fmt.Sprintf("gap%d", m+1)), runs[r-1].end-runs[first].start
			filters = append(filters, transparentGapFilter(gap, length, canvasWidth, canvasHeight, rate))
			labels, lengths = append(labels, gap), append(lengths, length)
		}
		sub := name(m+1, "sub")
		filters = append(filters, concatPieces(labels, lengths, timebase, sub))
		layers = append(layers, blendLayer{mode: mode, label: sub})
	}
	return filters, layers
}

// blendComposite composites layer onto base with a mode other than normal and ends in output (index §4.22). It is the
// form the spike measured exact on 5.1.9 and 8.1.1:
//   - base and layer go to gbrp (RGB only, no alpha conversion), and blend computes E = F(below, layer) on the three
//     colour planes at full strength. Dodge and burn take the layer first (blendSwapped); every other mode takes the
//     base first.
//   - maskedmerge mixes E into the base with the layer's alpha: (below·(255 − α) + E·α + 127) // 255. The alpha comes
//     from alphaextract, never through swscale, which adds 1 to alpha 128..254 on 5.1. The result is opaque rgba again.
//
// blend's own opacity (all_opacity) is never used: at 0 it returns the first input, which is the layer for dodge and
// burn. sequence numbers the composite within the layer's labels. A mode without an FFmpeg name composites normal.
func blendComposite(base, layer, mode, output string, trackIndex, sequence int) string {
	spec, ok := BlendModeByID(mode)
	if !ok || spec.Blend == "" {
		return normalComposite(base, layer, output)
	}
	name := func(part string) string { return fmt.Sprintf("vs_bl_%d_%d_%s", trackIndex, sequence, part) }
	first, second := name("b0"), name("lg")
	if blendSwapped[spec.ID] {
		first, second = second, first
	}
	m := spec.Blend
	return strings.Join([]string{
		fmt.Sprintf("[%s]format=gbrp,split=2[%s][%s]", base, name("b0"), name("b1")),
		fmt.Sprintf("[%s]split=2[%s][%s]", layer, name("l0"), name("l1")),
		fmt.Sprintf("[%s]format=gbrp[%s]", name("l0"), name("lg")),
		fmt.Sprintf("[%s]alphaextract[%s]", name("l1"), name("la")),
		fmt.Sprintf("[%s][%s]blend=c0_mode=%s:c1_mode=%s:c2_mode=%s[%s]", first, second, m, m, m, name("e")),
		fmt.Sprintf("[%s][%s][%s]maskedmerge=planes=7,format=rgba[%s]", name("b1"), name("e"), name("la"), output),
	}, ";\n")
}

// normalComposite is the 2c overlay of a layer onto the composite below, byte for byte. Straight alpha; frames after
// the layer's end pass the base through.
func normalComposite(base, layer, output string) string {
	return fmt.Sprintf("[%s][%s]overlay=eof_action=pass:repeatlast=0:shortest=0:format=auto[%s]", base, layer, output)
}

// compositeLayer composites track layer trackIndex (graph label label, clips layer) onto base and ends in output:
//   - a layer whose window frames are all normal keeps the 2c overlay;
//   - otherwise the sublayers of splitBlendLayers composite in turn: normal with the overlay, the other modes with
//     blendComposite.
func compositeLayer(base, label string, layer []layerClip, trackIndex, canvasWidth, canvasHeight, frames int, rate Rate, output string) []string {
	runs := blendRuns(layer, frames)
	if len(runs) == 1 && runs[0].mode == BlendNormal {
		return []string{normalComposite(base, label, output)}
	}
	filters, sublayers := splitBlendLayers(label, layerEnd(layer), runs, trackIndex, canvasWidth, canvasHeight, frames, rate)
	current := base
	for n, sub := range sublayers {
		target := output
		if n < len(sublayers)-1 {
			target = fmt.Sprintf("vs_bl_%d_%d_out", trackIndex, n+1)
		}
		if sub.mode == BlendNormal {
			filters = append(filters, normalComposite(current, sub.label, target))
		} else {
			filters = append(filters, blendComposite(current, sub.label, sub.mode, target, trackIndex, n+1))
		}
		current = target
	}
	return filters
}
