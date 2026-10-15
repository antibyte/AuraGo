package videostudio

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// renderSource is one staged asset of a render.
type renderSource struct {
	asset    Asset
	path     string
	videoEnd float64 // seconds of decodable video; 0 when unknown or not a video
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
	from  int
	image bool
	input int
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

// buildVideoSegment composes the visual tracks of seg. When seg starts or ends inside a transition the graph
// covers the whole transition (pre-roll) and trims the composed frames back to seg.
func buildVideoSegment(project Project, seg Segment, sources map[string]renderSource, rate Rate, outWidth, outHeight int) ([]segmentInput, string, error) {
	window := expandRange(seg, videoForbiddenSpans(project))
	layers, inputs, err := segmentLayers(project, window, sources, rate)
	if err != nil {
		return nil, "", err
	}
	width, height, frames := project.Width, project.Height, window.End-window.Start
	filters := []string{fmt.Sprintf("color=c=black:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[vs_base0]", width, height, rate, rate.SecondsArg(frames), frames)}
	output := "vs_base0"
	for index, layer := range layers {
		label, layerFilters := buildVisualLayer(layer, index, width, height, rate)
		filters = append(filters, layerFilters...)
		next := fmt.Sprintf("vs_base%d", index+1)
		filters = append(filters, fmt.Sprintf("[%s][%s]overlay=eof_action=pass:repeatlast=0:shortest=0:format=auto[%s]", output, label, next))
		output = next
	}
	tail := fmt.Sprintf("[%s]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS", output, seg.Start-window.Start, seg.End-window.Start)
	// Convert RGBA to limited-range BT.709 YUV here, together with the optional resize: left to the encoder's
	// automatic conversion, FFmpeg before 7.1 uses BT.601 while the stream is tagged BT.709. Hardware encoders
	// append their upload (format=nv12,hwupload) after this format.
	convert := "scale=out_color_matrix=bt709:out_range=tv"
	if outWidth != width || outHeight != height {
		convert = fmt.Sprintf("scale=w=%d:h=%d:flags=bicubic:out_color_matrix=bt709:out_range=tv", outWidth, outHeight)
	}
	filters = append(filters, tail+","+convert+",format=yuv420p[vs_video_out]")
	return inputs, strings.Join(filters, ";\n"), nil
}

func segmentLayers(project Project, window Segment, sources map[string]renderSource, rate Rate) ([][]layerClip, []segmentInput, error) {
	var layers [][]layerClip
	var inputs []segmentInput
	for _, track := range project.Tracks {
		if !visualClip(track) {
			continue
		}
		clips := sortedClips(track.Clips)
		var layer []layerClip
		for i, clip := range clips {
			end := clip.Start + clip.Duration
			if clip.Start >= window.End || end <= window.Start {
				continue
			}
			source, ok := sources[clip.AssetID]
			if !ok {
				return nil, nil, fmt.Errorf("staged path is missing for visual asset %q", clip.AssetID)
			}
			entered := max(0, window.Start-clip.Start)
			local := clip
			local.Start = max(clip.Start, window.Start) - window.Start
			local.Duration = min(end, window.End) - max(clip.Start, window.Start)
			local.Transition = nil
			if clip.Transition != nil && i+1 < len(clips) && clips[i+1].Start < window.End {
				local.Transition = clip.Transition
			}
			item := layerClip{clip: local, image: source.asset.Kind == AssetImage, input: len(inputs)}
			input := segmentInput{path: source.path, image: item.image}
			if !input.image {
				want := clip.Offset + entered
				// Seek about a second early: the source frame that is current at want usually starts before
				// it, and accurate seeking drops every frame before the seek point. fps then picks frames on
				// absolute timestamps, so every segmentation keeps the cadence of the single pass.
				seekFrame := max(0, want-int((rate.Num+rate.Den-1)/rate.Den))
				if source.videoEnd > 0 {
					// Seeking past the last video frame yields no frames; seek a second before it and let
					// tpad hold the final frame for the rest, exactly as the single pass did.
					lastSafe := int(math.Floor((source.videoEnd - 1) * float64(rate.Num) / float64(rate.Den)))
					seekFrame = min(seekFrame, max(0, lastSafe))
				}
				item.from = want
				input.seek = rate.Seconds(seekFrame)
				input.limit = rate.Seconds(want-seekFrame+local.Duration) + 1
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

func buildVisualLayer(layer []layerClip, trackIndex, canvasWidth, canvasHeight int, rate Rate) (string, []string) {
	filters := make([]string, 0, len(layer)*4)
	sequence := 0
	join := func() string {
		label := fmt.Sprintf("vs_join_%d_%d", trackIndex, sequence)
		sequence++
		return label
	}
	timebase := fmt.Sprintf("settb=%d/%d", rate.Den, rate.Num)
	labels := make([]string, len(layer))
	for i, item := range layer {
		labels[i] = fmt.Sprintf("vs_clip_%d_%d", trackIndex, i)
		filters = append(filters, buildVisualClipFilter(trackIndex, i, item, canvasWidth, canvasHeight, rate, labels[i]))
	}
	// concat outputs a microsecond timebase; settb after every concat and on both xfade inputs keeps xfade on the
	// frame timebase (a no-op where the input already uses it).
	combined := labels[0]
	combinedEnd := layer[0].clip.Start + layer[0].clip.Duration
	if layer[0].clip.Start > 0 {
		gap := gapLabel(trackIndex, 0)
		filters = append(filters, transparentGapFilter(gap, layer[0].clip.Start, canvasWidth, canvasHeight, rate))
		combined = join()
		filters = append(filters, fmt.Sprintf("[%s][%s]concat=n=2:v=1:a=0,%s[%s]", gap, labels[0], timebase, combined))
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
			filters = append(filters, fmt.Sprintf("[%s][%s]concat=n=2:v=1:a=0,%s[%s]", combined, gap, timebase, withGap))
			combined = withGap
		}
		out := join()
		filters = append(filters, fmt.Sprintf("[%s][%s]concat=n=2:v=1:a=0,%s[%s]", combined, labels[i], timebase, out))
		combined = out
		combinedEnd = clip.Start + clip.Duration
	}
	return combined, filters
}

// appendTransition joins incoming (a clip starting at clip.Start) to combined, whose last transition.Duration
// frames overlap it. xfade mixes all four channels, so the overlap is premultiplied to keep transparent
// surroundings from darkening the clips. Only the overlapping frames pass through premultiply, xfade and
// unpremultiply: that round trip changes pixels even at full opacity, and a frame outside a transition must
// render identically whether or not the transition shares its segment.
func appendTransition(filters []string, join func() string, combined, incoming string, clip Clip, transition Transition, timebase string, rate Rate) (string, []string) {
	overlap := transition.Duration
	var parts []string
	outgoingSource := combined
	if clip.Start > 0 {
		head, tail, before := join(), join(), join()
		filters = append(filters,
			fmt.Sprintf("[%s]split=2[%s][%s]", combined, head, tail),
			fmt.Sprintf("[%s]trim=end_frame=%d[%s]", head, clip.Start, before))
		parts = append(parts, before)
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
	outgoingPart, incomingPart, mixed := join(), join(), join()
	filters = append(filters,
		fmt.Sprintf("[%s]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,premultiply=inplace=1,%s[%s]", outgoingSource, clip.Start, clip.Start+overlap, timebase, outgoingPart),
		fmt.Sprintf("[%s]trim=end_frame=%d,setpts=PTS-STARTPTS,premultiply=inplace=1,%s[%s]", incomingSource, overlap, timebase, incomingPart),
		fmt.Sprintf("[%s][%s]xfade=transition=%s:duration=%s:offset=%s,unpremultiply=inplace=1,format=rgba[%s]", outgoingPart, incomingPart, mapTransition(transition.Type), rate.SecondsArg(overlap), rate.SecondsArg(0), mixed))
	parts = append(parts, mixed)
	if after != "" {
		parts = append(parts, after)
	}
	if len(parts) == 1 {
		return mixed, filters
	}
	out := join()
	filters = append(filters, fmt.Sprintf("[%s]concat=n=%d:v=1:a=0,%s[%s]", strings.Join(parts, "]["), len(parts), timebase, out))
	return out, filters
}

func buildVisualClipFilter(trackIndex, clipIndex int, item layerClip, canvasWidth, canvasHeight int, rate Rate, output string) string {
	clip := item.clip
	boxWidth := evenPixels(clip.Width*float64(canvasWidth), canvasWidth)
	boxHeight := evenPixels(clip.Height*float64(canvasHeight), canvasHeight)
	x := evenPosition(clip.X*float64(canvasWidth), boxWidth, canvasWidth)
	y := evenPosition(clip.Y*float64(canvasHeight), boxHeight, canvasHeight)
	scale := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0", boxWidth, boxHeight, boxWidth, boxHeight)
	if clip.Fit == FitCover {
		scale = fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=%d:h=%d", boxWidth, boxHeight, boxWidth, boxHeight)
	}
	rotation := ""
	if math.Abs(clip.Rotation) > 0.000001 {
		angle := fmt.Sprintf("(%.6f*PI/180)", clip.Rotation)
		rotation = fmt.Sprintf(",rotate=%s:ow=rotw(%s):oh=roth(%s):c=black@0", angle, angle, angle)
	}
	// Images start at zero. Video keeps absolute timestamps, so fps samples it on the source's own cadence and
	// output frame from becomes the clip's first frame. When the video stream starts after the container (for
	// example behind leading audio) no source frame covers the first positions; the first decoded frame is then
	// moved back to from, so fps holds it there instead of the clip running short.
	resample := fmt.Sprintf("setpts=PTS-STARTPTS,fps=%s", rate)
	if !item.image {
		resample = fmt.Sprintf("setpts='if(eq(N,0),min(PTS,floor(%.9f/TB)),PTS)',fps=%s", rate.Seconds(item.from), rate)
	}
	// Shift into the clip and bound clone padding to a small multiple of the clip's duration.
	offsetPTS := fmt.Sprintf("setpts='max(PTS-%d,-%d)'", item.from, clip.Duration)
	source := fmt.Sprintf(
		"[%d:V:0]%s,%s,tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS,%s,format=rgba%s",
		item.input, resample, offsetPTS, clip.Duration*2, clip.Duration, scale, rotation,
	)
	if clip.Opacity < 0.999999 {
		source += fmt.Sprintf(",colorchannelmixer=aa=%.6f", clip.Opacity)
	}
	layer := fmt.Sprintf("vs_layer_%d_%d", trackIndex, clipIndex)
	background := fmt.Sprintf("vs_clip_bg_%d_%d", trackIndex, clipIndex)
	positionX, positionY := fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)
	if math.Abs(clip.Rotation) > 0.000001 {
		positionX = fmt.Sprintf("'%d+%d/2-overlay_w/2'", x, boxWidth)
		positionY = fmt.Sprintf("'%d+%d/2-overlay_h/2'", y, boxHeight)
	}
	return fmt.Sprintf("%s[%s];color=c=black@0:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[%s];[%s][%s]overlay=x=%s:y=%s:format=auto:shortest=1[%s]", source, layer, canvasWidth, canvasHeight, rate, rate.SecondsArg(clip.Duration), clip.Duration, background, background, layer, positionX, positionY, output)
}

// buildAudioSegment mixes every audible clip of seg into exactly SampleAt(End)-SampleAt(Start) samples.
// When seg starts or ends inside a fade the window covers the whole fade and is trimmed back.
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
			entered := max(0, window.Start-clip.Start)
			localStart, localEnd := max(clip.Start, window.Start), min(end, window.End)
			samples := rate.SampleAt(localEnd) - rate.SampleAt(localStart)
			input := len(inputs)
			inputs = append(inputs, segmentInput{path: source.path, audioOnly: true, seek: rate.Seconds(clip.Offset + entered), limit: rate.Seconds(localEnd-localStart) + 1})
			label := fmt.Sprintf("vs_audio_%d_%d", trackIndex, clipIndex)
			filter := fmt.Sprintf("[%d:a:0]asetpts=PTS-STARTPTS,aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo,atrim=end_sample=%d,volume=%.6f", input, AudioSampleRate, samples, clip.Volume)
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

func transparentGapFilter(label string, frames, width, height int, rate Rate) string {
	return fmt.Sprintf("color=c=black@0:s=%dx%d:r=%s:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=rgba[%s]", width, height, rate, rate.SecondsArg(frames), frames, label)
}

func mapTransition(value TransitionType) string {
	switch value {
	case TransitionBlack:
		return "fadeblack"
	case TransitionWipeLeft:
		return "wiperight"
	case TransitionWipeRight:
		return "wipeleft"
	default:
		return "fade"
	}
}

func evenPixels(value float64, bound int) int {
	pixels := int(math.Round(value/2) * 2)
	if pixels < 2 {
		return 2
	}
	if pixels > bound {
		return bound
	}
	return pixels
}

func evenPosition(value float64, box, bound int) int {
	position := int(math.Round(value/2) * 2)
	maxPosition := bound - box
	if position < 0 {
		return 0
	}
	if position > maxPosition {
		position = maxPosition
	}
	return position
}

func gapLabel(trackIndex, sequence int) string {
	return fmt.Sprintf("vs_gap_%d_%d", trackIndex, sequence)
}
