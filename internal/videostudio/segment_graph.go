package videostudio

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// renderSource is one staged asset of a render:
//   - videoEnd: the seconds of decodable video (0 when unknown or not a video);
//   - decodedWidth × decodedHeight: the picture FFmpeg decodes (videos autorotated, images as imported); 0 when not
//     probed;
//   - lut: the parsed table of a lut asset.
type renderSource struct {
	asset                       Asset
	path                        string
	videoEnd                    float64
	decodedWidth, decodedHeight int
	lut                         *LUT3D
}

// decodedSize is the picture size FFmpeg decodes. It falls back to the asset's size when the render did not probe it
// (for example sources built by tests).
func (s renderSource) decodedSize() (int, int) {
	if s.decodedWidth > 0 && s.decodedHeight > 0 {
		return s.decodedWidth, s.decodedHeight
	}
	return s.asset.Width, s.asset.Height
}

// segmentInput is one FFmpeg input of a segment process.
type segmentInput struct {
	path      string
	image     bool
	audioOnly bool
	seek      float64 // seconds into the source
	limit     float64 // seconds of source to read
}

// layerClip is a visual clip cut to a segment window, in window-local frames.
type layerClip struct {
	clip Clip // Start/Duration are window-local; Transition is kept only when the next clip is in the window
	// from is the source position, in project frames, shown at the clip's first window frame. Seeked video
	// inputs keep their absolute timestamps (-copyts -start_at_zero), so after fps the input's PTS count
	// project frames from the start of the source.
	from float64
	// first is the fps frame of the clip's first window frame, Offset plus the frames it entered the window late.
	// At speed 1 it equals from; a clip with a speed is retimed around Offset, so first stays a whole frame.
	first int
	image bool
	input int
	// whole is the clip as on the timeline. at is the absolute timeline frame of the clip's first window frame,
	// max(clip.Start, window.Start) with pre-roll included, so window frame k is absolute frame at+k; every per-frame
	// value (keyframes, ramp, commands) is computed from absolute frames. pieces are the ramp pieces over
	// [0, whole.Duration] in clip time, nil without a ramp (index §4.10, §4.18).
	whole  Clip
	at     int
	pieces []rampPiece
	// sourceAspect is the asset's width / height (0 when unknown); the cover bound of keyframed geometry uses it.
	sourceAspect float64
	// luts holds the table of every on lut entry with a reference, by stack index (built-in looks included); nil
	// without one. pictureWidth × pictureHeight is the decoded (autorotated) and uncropped picture: crop commands and
	// the effects' resolution rule work in these pixels (index §4.20).
	luts                        map[int]*LUT3D
	pictureWidth, pictureHeight int
}

// sendCmdFile is one command group's sendcmd script for one FFmpeg process. name is the bare file name the graph
// reads: vs_cmd_<L>_<C>.txt for group 0, vs_cmd_<L>_<C>_<g>.txt after the g-th two-input stage. stageVideoGraph writes
// it next to graph.txt in the process's working directory.
type sendCmdFile struct {
	name   string
	tracks []SendCmdTrack
}

func segmentInputArgs(inputs []segmentInput, rate Rate) []string {
	var args []string
	for _, input := range inputs {
		args = append(args, "-protocol_whitelist", allowedProtocols, "-format_whitelist", allowedDemuxers, "-threads", strconv.Itoa(inputThreadLimit))
		if input.image {
			args = append(args, "-stream_loop", "-1", "-f", "image2", "-pattern_type", "none", "-framerate", rate.String())
		} else {
			args = append(args, "-ss", fmt.Sprintf("%.6f", input.seek), "-t", fmt.Sprintf("%.6f", input.limit))
			if input.audioOnly {
				args = append(args, "-vn")
			}
		}
		args = append(args, "-i", input.path)
	}
	return args
}

func inputPaths(inputs []segmentInput) []string {
	paths := make([]string, len(inputs))
	for i, input := range inputs {
		paths[i] = input.path
	}
	return paths
}

// videoTail says how a composed video segment leaves the graph.
type videoTail struct {
	width, height int    // output size; the project size keeps the canvas
	upload        string // hardware upload (format=nv12,hwupload) appended after the YUV conversion
	rgb           bool   // RenderFrame: 8-bit RGB (rgb24) instead of the BT.709 YUV tail
}

// buildVideoSegment composes the visual tracks of seg for the encoder (BT.709 YUV tail). The graph files are the
// sendcmd scripts and LUT tables the graph reads; stageVideoGraph writes them.
func buildVideoSegment(project Project, seg Segment, sources map[string]renderSource, rate Rate, outWidth, outHeight int, upload string) ([]segmentInput, string, graphFiles, error) {
	return buildVideoSegmentWith(project, seg, sources, rate, videoTail{width: outWidth, height: outHeight, upload: upload})
}

// buildVideoSegmentWith composes the visual tracks of seg. When seg starts or ends inside a transition the graph
// covers the whole transition (pre-roll) and trims the composed frames back to seg. Each track layer composites onto
// the layers below with the effective blend mode of every frame (compositeLayer, index §4.22). A layer whose frames are
// all normal keeps the 2c overlay byte for byte.
func buildVideoSegmentWith(project Project, seg Segment, sources map[string]renderSource, rate Rate, out videoTail) ([]segmentInput, string, graphFiles, error) {
	window := expandRange(seg, videoForbiddenSpans(project))
	layers, inputs, err := segmentLayers(project, window, sources, rate)
	if err != nil {
		return nil, "", graphFiles{}, err
	}
	if count := lutInstances(layers); count > MaxSegmentLUTInstances {
		return nil, "", graphFiles{}, tooManyLUTs(seg, count)
	}
	width, height, frames := project.Width, project.Height, window.End-window.Start
	filters := []string{fmt.Sprintf("color=c=black:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[vs_base0]", width, height, rate, rate.SecondsArg(frames), frames)}
	output := "vs_base0"
	var files graphFiles
	// One numbering for the whole process: its lut effects of one table all read the same staged file.
	names := &lutNames{}
	for index, layer := range layers {
		label, layerFilters, layerFiles := buildVisualLayer(layer, index, width, height, rate, names)
		filters = append(filters, layerFilters...)
		files.commands = append(files.commands, layerFiles.commands...)
		files.addLUTs(layerFiles.luts)
		next := fmt.Sprintf("vs_base%d", index+1)
		filters = append(filters, compositeLayer(output, label, layer, index, width, height, frames, rate, next)...)
		output = next
	}
	tail := fmt.Sprintf("[%s]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS", output, seg.Start-window.Start, seg.End-window.Start)
	if out.rgb {
		// Golden frames: lossless 8-bit RGB straight from the RGBA composition, before any YUV or 4:2:0 conversion.
		finish := "format=rgb24"
		if out.width != width || out.height != height {
			finish = fmt.Sprintf("scale=w=%d:h=%d:flags=bicubic,format=rgb24", out.width, out.height)
		}
		filters = append(filters, tail+","+finish+"[vs_video_out]")
		return inputs, strings.Join(filters, ";\n"), files, nil
	}
	// Convert RGBA to limited-range BT.709 YUV here, together with the optional resize: left to the encoder's
	// automatic conversion, FFmpeg before 7.1 uses BT.601 while the stream is tagged BT.709. Hardware encoders
	// append their upload (format=nv12,hwupload) after this format.
	convert := "scale=out_color_matrix=bt709:out_range=tv"
	if out.width != width || out.height != height {
		convert = fmt.Sprintf("scale=w=%d:h=%d:flags=bicubic:out_color_matrix=bt709:out_range=tv", out.width, out.height)
	}
	filters = append(filters, tail+","+convert+",format=yuv420p"+out.upload+"[vs_video_out]")
	return inputs, strings.Join(filters, ";\n"), files, nil
}

// segmentLayers cuts every visual track to the window. Disabled clips are left out. A transition applies only when
// the clip after it in the unfiltered track order is enabled, so preview and export agree.
func segmentLayers(project Project, window Segment, sources map[string]renderSource, rate Rate) ([][]layerClip, []segmentInput, error) {
	var layers [][]layerClip
	var inputs []segmentInput
	for _, track := range project.Tracks {
		if !visualClip(track) {
			continue
		}
		ordered := sortedClips(track.Clips)
		var layer []layerClip
		for i, clip := range ordered {
			end := clip.Start + clip.Duration
			if clip.Disabled || clip.Start >= window.End || end <= window.Start {
				continue
			}
			source, ok := sources[clip.AssetID]
			if !ok {
				return nil, nil, fmt.Errorf("staged path is missing for visual asset %q", clip.AssetID)
			}
			speed := ClipSpeed(clip)
			entered := max(0, window.Start-clip.Start)
			local := clip
			local.Start = max(clip.Start, window.Start) - window.Start
			local.Duration = min(end, window.End) - max(clip.Start, window.Start)
			local.Transition = nil
			if clip.Transition != nil && i+1 < len(ordered) && !ordered[i+1].Disabled && ordered[i+1].Start < window.End {
				local.Transition = clip.Transition
			}
			item := layerClip{clip: local, whole: clip, at: max(clip.Start, window.Start), image: source.asset.Kind == AssetImage, input: len(inputs)}
			if len(clip.Ramp) > 0 {
				item.pieces = rampPieces(clip.Ramp, 0, float64(clip.Duration))
			}
			if source.asset.Height > 0 {
				item.sourceAspect = float64(source.asset.Width) / float64(source.asset.Height)
			}
			item.pictureWidth, item.pictureHeight = source.decodedSize()
			for index, ref := range clipLUTRefs(clip) {
				table, err := resolveLUTRef(ref, sources)
				if err != nil {
					return nil, nil, fmt.Errorf("clip %q effect %d: %w", clip.ID, index, err)
				}
				if item.luts == nil {
					item.luts = make(map[int]*LUT3D)
				}
				item.luts[index] = table
			}
			if clip.Crop != nil && !cropAnimated(clip) && item.pictureWidth > 0 && item.pictureHeight > 0 {
				// The transform fits the cropped picture, so the cover clamp and the cover bound use its aspect (index §4.10).
				_, _, width, height := cropRect(*clip.Crop, item.pictureWidth, item.pictureHeight)
				item.sourceAspect = float64(width) / float64(height)
			}
			if pictureSizeNeeded(clip) && (item.pictureWidth <= 0 || item.pictureHeight <= 0) {
				return nil, nil, fmt.Errorf("decoded picture size of asset %q is unknown; clip %q needs it for its crop, effects or blur animation", clip.AssetID, clip.ID)
			}
			input := segmentInput{path: source.path, image: item.image}
			if !input.image {
				// The source position of the first window frame: a clip at speed s advances s source frames per frame; a
				// ramped clip has consumed S(entered) and reads S(entered + frames) − S(entered) more (index §4.18).
				from, span := float64(clip.Offset)+float64(entered)*speed, float64(local.Duration)*speed
				if item.pieces != nil {
					consumed := rampSource(item.pieces, float64(entered))
					from, span = float64(clip.Offset)+consumed, rampSource(item.pieces, float64(entered+local.Duration))-consumed
				}
				item.from, item.first = from, clip.Offset+entered
				// Seek about a second early: the source frame that is current at from usually starts before it, and
				// accurate seeking drops every frame before the seek point. The retimed timestamps keep the single
				// pass's cadence, so every segmentation samples the same source frames.
				seekFrame := max(0, int(math.Floor(from))-int((rate.Num+rate.Den-1)/rate.Den))
				if source.videoEnd > 0 {
					// Seeking past the last video frame yields no frames; seek a second before it and let
					// tpad hold the final frame for the rest, exactly as the single pass did.
					lastSafe := int(math.Floor((source.videoEnd - 1) * float64(rate.Num) / float64(rate.Den)))
					seekFrame = min(seekFrame, max(0, lastSafe))
				}
				input.seek = rate.Seconds(seekFrame)
				input.limit = rate.SecondsFloat(from+span-float64(seekFrame)) + 1
			}
			inputs = append(inputs, input)
			layer = append(layer, item)
		}
		if len(layer) > 0 {
			layers = append(layers, layer)
		}
	}
	return layers, inputs, nil
}

// clipLUTRefs returns the LUT reference of every on lut entry of a clip's effect stack, by stack index. Entries
// without a table ("") are left out. Renders load these (collectRenderSources) and layer clips carry them.
func clipLUTRefs(clip Clip) map[int]string {
	var refs map[int]string
	for index, entry := range clip.Effects {
		if !entry.On || entry.ID != "lut" {
			continue
		}
		ref, _ := entry.Params["lut"].(string)
		if ref == "" {
			continue
		}
		if refs == nil {
			refs = make(map[int]string)
		}
		refs[index] = ref
	}
	return refs
}

// resolveLUTRef finds the table of a LUT reference: a built-in look, or a staged project lut asset.
func resolveLUTRef(ref string, sources map[string]renderSource) (*LUT3D, error) {
	builtin, assetID, ok := ParseLUTRef(ref)
	switch {
	case !ok:
		return nil, fmt.Errorf("invalid LUT reference %q", ref)
	case builtin != "":
		if table, found := BuiltinLook(builtin); found {
			return table, nil
		}
	case assetID != "":
		if source, staged := sources[assetID]; staged && source.lut != nil {
			return source.lut, nil
		}
	}
	return nil, fmt.Errorf("LUT %q is not staged", ref)
}

func buildVisualLayer(layer []layerClip, trackIndex, canvasWidth, canvasHeight int, rate Rate, names *lutNames) (string, []string, graphFiles) {
	filters := make([]string, 0, len(layer)*4)
	sequence := 0
	join := func() string {
		label := fmt.Sprintf("vs_join_%d_%d", trackIndex, sequence)
		sequence++
		return label
	}
	timebase := fmt.Sprintf("settb=%d/%d", rate.Den, rate.Num)
	labels := make([]string, len(layer))
	var files graphFiles
	for i, item := range layer {
		labels[i] = fmt.Sprintf("vs_clip_%d_%d", trackIndex, i)
		chain := clipChain{track: trackIndex, index: i, item: item, canvasWidth: canvasWidth, canvasHeight: canvasHeight, rate: rate, lutNames: names}
		built := chain.build(labels[i])
		filters = append(filters, built.filter)
		files.commands = append(files.commands, built.commands...)
		files.addLUTs(built.luts)
	}
	// concat outputs a microsecond timebase; settb after every concat and on both xfade inputs keeps xfade on the
	// frame timebase (a no-op where the input already uses it).
	combined := labels[0]
	combinedEnd := layer[0].clip.Start + layer[0].clip.Duration
	if layer[0].clip.Start > 0 {
		gap := gapLabel(trackIndex, 0)
		filters = append(filters, transparentGapFilter(gap, layer[0].clip.Start, canvasWidth, canvasHeight, rate))
		combined = join()
		filters = append(filters, concatPieces([]string{gap, labels[0]}, []int{layer[0].clip.Start, layer[0].clip.Duration}, timebase, combined))
	}
	for i := 1; i < len(layer); i++ {
		clip, previous := layer[i].clip, layer[i-1].clip
		if previous.Transition != nil {
			combined, filters = appendTransition(filters, join, combined, labels[i], clip, *previous.Transition, timebase, rate)
			combinedEnd = clip.Start + clip.Duration
			continue
		}
		if clip.Start > combinedEnd {
			gap := gapLabel(trackIndex, i)
			filters = append(filters, transparentGapFilter(gap, clip.Start-combinedEnd, canvasWidth, canvasHeight, rate))
			withGap := join()
			filters = append(filters, concatPieces([]string{combined, gap}, []int{combinedEnd, clip.Start - combinedEnd}, timebase, withGap))
			combined = withGap
		}
		out := join()
		filters = append(filters, concatPieces([]string{combined, labels[i]}, []int{clip.Start, clip.Duration}, timebase, out))
		combined = out
		combinedEnd = clip.Start + clip.Duration
	}
	return combined, filters, files
}

// concatPieces joins pieces of a layer end to end; each is frames[i] whole frames with timestamps from zero. FFmpeg's
// concat (5.1 to 8.1) measures a video input by its mean frame spacing, which a single frame lacks: a one-frame input
// counts as zero frames long, and everything after it lands one frame early (pts 0, 0, 1, 2, …). A concat with a
// one-frame input before its last therefore numbers its output frames afresh (setpts=N on the frame timebase); every
// other concat keeps its filter byte for byte.
func concatPieces(labels []string, frames []int, timebase, output string) string {
	filter := fmt.Sprintf("[%s]concat=n=%d:v=1:a=0,%s", strings.Join(labels, "]["), len(labels), timebase)
	if slices.Contains(frames[:len(frames)-1], 1) {
		filter += ",setpts=N"
	}
	return filter + "[" + output + "]"
}

// appendTransition joins incoming (a clip starting at clip.Start) to combined, whose last transition.Duration
// frames overlap it. xfade mixes all four channels, so the overlap is premultiplied to keep transparent
// surroundings from darkening the clips. Only the overlapping frames pass through premultiply, xfade and
// unpremultiply: that round trip changes pixels even at full opacity, and a frame outside a transition must
// render identically whether or not the transition shares its segment.
func appendTransition(filters []string, join func() string, combined, incoming string, clip Clip, transition Transition, timebase string, rate Rate) (string, []string) {
	overlap := transition.Duration
	var parts []string
	var lengths []int
	outgoingSource := combined
	if clip.Start > 0 {
		head, tail, before := join(), join(), join()
		filters = append(filters,
			fmt.Sprintf("[%s]split=2[%s][%s]", combined, head, tail),
			fmt.Sprintf("[%s]trim=end_frame=%d[%s]", head, clip.Start, before))
		parts, lengths = append(parts, before), append(lengths, clip.Start)
		outgoingSource = tail
	}
	incomingSource, after := incoming, ""
	if clip.Duration > overlap {
		head, tail := join(), join()
		after = join()
		filters = append(filters,
			fmt.Sprintf("[%s]split=2[%s][%s]", incoming, head, tail),
			fmt.Sprintf("[%s]trim=start_frame=%d,setpts=PTS-STARTPTS[%s]", tail, overlap, after))
		incomingSource = head
	}
	// The xfade name comes from the registry. Validate admits only registry transitions; the fallback keeps the graph
	// well-formed and equals the old default.
	xfade := "fade"
	if spec, ok := TransitionByID(transition.Type); ok {
		xfade = spec.XFade
	}
	outgoingPart, incomingPart, mixed := join(), join(), join()
	filters = append(filters,
		fmt.Sprintf("[%s]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,premultiply=inplace=1,%s[%s]", outgoingSource, clip.Start, clip.Start+overlap, timebase, outgoingPart),
		fmt.Sprintf("[%s]trim=end_frame=%d,setpts=PTS-STARTPTS,premultiply=inplace=1,%s[%s]", incomingSource, overlap, timebase, incomingPart),
		fmt.Sprintf("[%s][%s]xfade=transition=%s:duration=%s:offset=%s,unpremultiply=inplace=1,format=rgba[%s]", outgoingPart, incomingPart, xfade, rate.SecondsArg(overlap), rate.SecondsArg(0), mixed))
	parts, lengths = append(parts, mixed), append(lengths, overlap)
	if after != "" {
		parts, lengths = append(parts, after), append(lengths, clip.Duration-overlap)
	}
	if len(parts) == 1 {
		return mixed, filters
	}
	out := join()
	filters = append(filters, concatPieces(parts, lengths, timebase, out))
	return out, filters
}

// videoResample samples a video clip's source on the project's frame grid. from is the source position, in project
// frames, of the first frame the graph needs; fps works on absolute source timestamps, so fps frame offset+n is clip
// frame n. A still capture uses the same filters, so it shows the frame the renderer shows.
func videoResample(from float64, offset int, speed float64, rate Rate) string {
	start := rate.SecondsFloat(from)
	if speed == 1 {
		return fmt.Sprintf("setpts='if(eq(N,0),min(PTS,floor(%.9f/TB)),PTS)',fps=%s", start, rate)
	}
	// Retime around the clip's first source frame (Offset) before fps: clip frame n shows the last source
	// frame at or before Offset + n*s, so a 2x clip reads every second source frame. Each source frame is
	// moved onto the whole fps frame where it first shows (ceil, with 1 ms of tolerance for container
	// timestamp rounding), in microseconds: setpts truncates, and the half-frame positions of even speeds
	// would otherwise leave the choice to rounding noise. The same Offset anchor in every segment keeps the
	// fps frames whole, so every segmentation samples the same source frames.
	return fmt.Sprintf("setpts='if(eq(N,0),min(PTS,floor(%.9f/TB)),PTS)',settb=AVTB,setpts='(%d+ceil(((PTS*TB-0.001)*%d/%d-%d)/%s))*%d/%d/TB',fps=%s",
		start, offset, rate.Num, rate.Den, offset, strconv.FormatFloat(speed, 'f', -1, 64), rate.Den, rate.Num, rate)
}

// buildAudioSegment mixes every audible clip of seg into exactly SampleAt(End)-SampleAt(Start) samples.
// When seg starts or ends inside a fade the window covers the whole fade and is trimmed back. A clip above 4x is
// left out of the mix, which is silence in preview and export alike.
func buildAudioSegment(project Project, seg Segment, sources map[string]renderSource, rate Rate) ([]segmentInput, string, error) {
	assets := assetMap(project)
	window := expandRange(seg, audioForbiddenSpans(project, assets))
	base := rate.SampleAt(window.Start)
	var inputs []segmentInput
	var labels, filters []string
	for trackIndex, track := range project.Tracks {
		for clipIndex, clip := range track.Clips {
			source, ok := sources[clip.AssetID]
			if !ok || !audibleClip(track, clip, source.asset) {
				continue
			}
			end := clip.Start + clip.Duration
			if clip.Start >= window.End || end <= window.Start {
				continue
			}
			speed := ClipSpeed(clip)
			if speed > 4 {
				continue
			}
			entered := max(0, window.Start-clip.Start)
			localStart, localEnd := max(clip.Start, window.Start), min(end, window.End)
			samples := rate.SampleAt(localEnd) - rate.SampleAt(localStart)
			from := float64(clip.Offset) + float64(entered)*speed
			input := len(inputs)
			inputs = append(inputs, segmentInput{path: source.path, audioOnly: true, seek: rate.SecondsFloat(from),
				limit: rate.SecondsFloat(float64(localEnd-localStart)*speed) + 1})
			label := fmt.Sprintf("vs_audio_%d_%d", trackIndex, clipIndex)
			filter := fmt.Sprintf("[%d:a:0]asetpts=PTS-STARTPTS,aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo", input, AudioSampleRate)
			if tempo := tempoChain(speed); tempo != "" {
				filter += "," + tempo
			}
			filter += fmt.Sprintf(",atrim=end_sample=%d,volume=%.6f", samples, clip.Volume)
			if entered == 0 && clip.FadeIn > 0 {
				filter += fmt.Sprintf(",afade=t=in:st=0:d=%s", rate.SecondsArg(clip.FadeIn))
			}
			if end <= window.End && clip.FadeOut > 0 {
				filter += fmt.Sprintf(",afade=t=out:st=%s:d=%s", rate.SecondsArg(localEnd-localStart-clip.FadeOut), rate.SecondsArg(clip.FadeOut))
			}
			filter += fmt.Sprintf(",apad=whole_len=%d,adelay=%dS:all=1[%s]", samples, rate.SampleAt(localStart)-base, label)
			filters = append(filters, filter)
			labels = append(labels, "["+label+"]")
		}
	}
	windowSamples := rate.SampleAt(window.End) - base
	if len(labels) == 0 {
		filters = append(filters, fmt.Sprintf("anullsrc=r=%d:cl=stereo,atrim=end_sample=%d,aformat=sample_fmts=fltp[vs_audio_mix]", AudioSampleRate, windowSamples))
	} else {
		filters = append(filters, fmt.Sprintf("%samix=inputs=%d:duration=longest:dropout_transition=0:normalize=0[vs_audio_mix]", strings.Join(labels, ""), len(labels)))
	}
	filters = append(filters, fmt.Sprintf("[vs_audio_mix]apad=whole_len=%d,atrim=start_sample=%d:end_sample=%d,asetpts=PTS-STARTPTS[vs_audio_out]", windowSamples, rate.SampleAt(seg.Start)-base, rate.SampleAt(seg.End)-base))
	return inputs, strings.Join(filters, ";\n"), nil
}

// tempoChain returns the atempo filters for a speed. Each atempo instance takes 0.5 to 2 (the FFmpeg 5.1 range), so
// faster or slower speeds chain instances. Speed 1 needs none, and its graph stays exactly as it was.
func tempoChain(speed float64) string {
	var parts []string
	for speed > 2 {
		parts = append(parts, "atempo=2")
		speed /= 2
	}
	for speed < 0.5 {
		parts = append(parts, "atempo=0.5")
		speed /= 0.5
	}
	if math.Abs(speed-1) > 1e-9 {
		parts = append(parts, fmt.Sprintf("atempo=%.6f", speed))
	}
	return strings.Join(parts, ",")
}

func transparentGapFilter(label string, frames, width, height int, rate Rate) string {
	return fmt.Sprintf("color=c=black@0:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[%s]", width, height, rate, rate.SecondsArg(frames), frames, label)
}

func gapLabel(trackIndex, sequence int) string {
	return fmt.Sprintf("vs_gap_%d_%d", trackIndex, sequence)
}

// pictureSizeNeeded reports a clip whose export works in decoded pixels:
//   - crop keyframes (crop@vs_picture commands);
//   - an effect that is on (the resolution rule, index §4.20);
//   - a blur_in animation that plays (2e): its sigma and its masks have the processed picture's size, and maskedmerge
//     refuses a mask of another size.
func pictureSizeNeeded(clip Clip) bool {
	if cropAnimated(clip) || animKindsOf(clip).blur {
		return true
	}
	for _, entry := range clip.Effects {
		if entry.On {
			return true
		}
	}
	return false
}

// lutInstances counts the lut3d filters a process opens: the on lut entries with a table, summed over the layers'
// clips. It is the quantity the segment planner bounds with MaxSegmentLUTInstances (countLUTInstances); the build
// checks it again, because RenderFrame builds one frame without the planner.
func lutInstances(layers [][]layerClip) int {
	count := 0
	for _, layer := range layers {
		for _, item := range layer {
			count += len(item.luts)
		}
	}
	return count
}
