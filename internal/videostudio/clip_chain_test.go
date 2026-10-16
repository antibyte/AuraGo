package videostudio

import (
	"encoding/json"
	"fmt"
	"testing"
)

// clipChainSnapshot pins one clip's export filter chain byte for byte: the clip_chain.go split must not change a single
// character of the graph in 2a (plan index §4.10).
type clipChainSnapshot struct {
	name          string
	track, index  int
	item          layerClip
	width, height int
	rate          Rate
	want          string
}

func clipChainSnapshots() []clipChainSnapshot {
	clip := func(edit func(*Clip)) Clip {
		c := Clip{ID: "c", AssetID: "a", Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain}
		edit(&c)
		return c
	}
	thirty, ntsc, pal := Rate{Num: 30, Den: 1}, Rate{Num: 30000, Den: 1001}, Rate{Num: 25, Den: 1}
	return []clipChainSnapshot{
		{name: "static", track: 0, index: 0, width: 1280, height: 720, rate: thirty,
			item: layerClip{clip: clip(func(c *Clip) { c.Duration = 60 })},
			want: `[0:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=30,setpts='max(PTS-0,-60)',tpad=stop_mode=clone:stop=120,trim=start_pts=0:end_pts=60,setpts=PTS-STARTPTS,scale=w=1280:h=720:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=1280:h=720:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba[vs_layer_0_0];color=c=black@0:s=1280x720:r=30:d=2.000000,trim=end_frame=60,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_0];[vs_clip_bg_0_0][vs_layer_0_0]overlay=x=0:y=0:format=auto:shortest=1[vs_clip_0_0]`},
		{name: "rotated", track: 1, index: 2, width: 1280, height: 720, rate: thirty,
			item: layerClip{clip: clip(func(c *Clip) {
				c.X, c.Y, c.Width, c.Height, c.Rotation, c.Offset, c.Duration = 0.25, 0.1, 0.5, 0.5, 30, 10, 45
			}), from: 10, first: 10, input: 3},
			want: `[3:V:0]setpts='if(eq(N,0),min(PTS,floor(0.333333333/TB)),PTS)',fps=30,setpts='max(PTS-10,-45)',tpad=stop_mode=clone:stop=90,trim=start_pts=0:end_pts=45,setpts=PTS-STARTPTS,scale=w=640:h=360:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=640:h=360:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba,rotate=(30.000000*PI/180):ow=rotw((30.000000*PI/180)):oh=roth((30.000000*PI/180)):c=black@0[vs_layer_1_2];color=c=black@0:s=1280x720:r=30:d=1.500000,trim=end_frame=45,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_1_2];[vs_clip_bg_1_2][vs_layer_1_2]overlay=x='320+640/2-overlay_w/2':y='72+360/2-overlay_h/2':format=auto:shortest=1[vs_clip_1_2]`},
		{name: "half_opacity_ntsc", track: 0, index: 1, width: 1920, height: 1080, rate: ntsc,
			item: layerClip{clip: clip(func(c *Clip) { c.X, c.Y, c.Width, c.Height, c.Opacity, c.Duration = 0.1, 0.2, 0.35, 0.4, 0.5, 30 }), input: 1},
			want: `[1:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=30000/1001,setpts='max(PTS-0,-30)',tpad=stop_mode=clone:stop=60,trim=start_pts=0:end_pts=30,setpts=PTS-STARTPTS,scale=w=672:h=432:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=672:h=432:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba,colorchannelmixer=aa=0.500000[vs_layer_0_1];color=c=black@0:s=1920x1080:r=30000/1001:d=1.001000,trim=end_frame=30,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_1];[vs_clip_bg_0_1][vs_layer_0_1]overlay=x=192:y=216:format=auto:shortest=1[vs_clip_0_1]`},
		{name: "speed_two", track: 0, index: 0, width: 1280, height: 720, rate: pal,
			item: layerClip{clip: clip(func(c *Clip) { c.Offset, c.Duration, c.Speed = 20, 50, 2 }), from: 30, first: 25},
			want: `[0:V:0]setpts='if(eq(N,0),min(PTS,floor(1.200000000/TB)),PTS)',settb=AVTB,setpts='(20+ceil(((PTS*TB-0.001)*25/1-20)/2))*1/25/TB',fps=25,setpts='max(PTS-25,-50)',tpad=stop_mode=clone:stop=100,trim=start_pts=0:end_pts=50,setpts=PTS-STARTPTS,scale=w=1280:h=720:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=1280:h=720:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba[vs_layer_0_0];color=c=black@0:s=1280x720:r=25:d=2.000000,trim=end_frame=50,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_0];[vs_clip_bg_0_0][vs_layer_0_0]overlay=x=0:y=0:format=auto:shortest=1[vs_clip_0_0]`},
		{name: "cover_portrait", track: 2, index: 0, width: 720, height: 1280, rate: thirty,
			item: layerClip{clip: clip(func(c *Clip) { c.X, c.Y, c.Width, c.Height, c.Fit, c.Duration = 0.05, 0.25, 0.9, 0.5, FitCover, 90 }), input: 2},
			want: `[2:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=30,setpts='max(PTS-0,-90)',tpad=stop_mode=clone:stop=180,trim=start_pts=0:end_pts=90,setpts=PTS-STARTPTS,scale=w=648:h=640:force_original_aspect_ratio=increase:force_divisible_by=2,crop=w=648:h=640,format=rgba[vs_layer_2_0];color=c=black@0:s=720x1280:r=30:d=3.000000,trim=end_frame=90,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_2_0];[vs_clip_bg_2_0][vs_layer_2_0]overlay=x=36:y=320:format=auto:shortest=1[vs_clip_2_0]`},
		{name: "image", track: 3, index: 1, width: 1280, height: 720, rate: thirty,
			item: layerClip{clip: clip(func(c *Clip) { c.X, c.Y, c.Width, c.Height, c.Duration = 0.6, 0.6, 0.3, 0.3, 75 }), image: true, input: 4},
			want: `[4:V:0]setpts=PTS-STARTPTS,fps=30,setpts='max(PTS-0,-75)',tpad=stop_mode=clone:stop=150,trim=start_pts=0:end_pts=75,setpts=PTS-STARTPTS,scale=w=384:h=216:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=384:h=216:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba[vs_layer_3_1];color=c=black@0:s=1280x720:r=30:d=2.500000,trim=end_frame=75,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_3_1];[vs_clip_bg_3_1][vs_layer_3_1]overlay=x=768:y=432:format=auto:shortest=1[vs_clip_3_1]`},
	}
}

func TestClipChainSnapshots(t *testing.T) {
	for _, c := range clipChainSnapshots() {
		output := fmt.Sprintf("vs_clip_%d_%d", c.track, c.index)
		got := clipChain{track: c.track, index: c.index, item: c.item, canvasWidth: c.width, canvasHeight: c.height, rate: c.rate}.filter(output)
		if got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

// TestParityGeometry checks clipBox against the shared fixture that the JS preview's placement runs too (index §4.13).
func TestParityGeometry(t *testing.T) {
	for _, c := range readParityCases(t, "geometry") {
		var input struct {
			Clip struct {
				X      float64 `json:"x"`
				Y      float64 `json:"y"`
				Width  float64 `json:"width"`
				Height float64 `json:"height"`
			} `json:"clip"`
			Canvas struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"canvas"`
		}
		var want struct {
			X int `json:"x"`
			Y int `json:"y"`
			W int `json:"w"`
			H int `json:"h"`
		}
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatalf("%s input: %v", c.Name, err)
		}
		if err := json.Unmarshal(c.Want, &want); err != nil {
			t.Fatalf("%s want: %v", c.Name, err)
		}
		clip := Clip{X: input.Clip.X, Y: input.Clip.Y, Width: input.Clip.Width, Height: input.Clip.Height}
		x, y, w, h := clipBox(clip, input.Canvas.Width, input.Canvas.Height)
		if x != want.X || y != want.Y || w != want.W || h != want.H {
			t.Errorf("%s: clipBox = (%d, %d, %d, %d), want (%d, %d, %d, %d)", c.Name, x, y, w, h, want.X, want.Y, want.W, want.H)
		}
	}
}

// TestClipChainKeyframedSnapshots pins the keyframed chains (index §4.11 as refined by plan 2b). The expression texts
// come from the builders, which TestKeyframeExprMatchesValueAt checks frame by frame; the stage order, filter names,
// canvas sizes and command files are pinned here.
func TestClipChainKeyframedSnapshots(t *testing.T) {
	pal := Rate{Num: 25, Den: 1}
	chain := func(track, index, input int, clip Clip) clipChain {
		return clipChain{track: track, index: index, item: layerClip{clip: clip, whole: clip, at: clip.Start, input: input, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: pal}
	}
	head := func(input, duration int) string {
		return fmt.Sprintf("[%d:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=25,setpts='max(PTS-0,-%d)',tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS,", input, duration, duration*2, duration)
	}
	tail := func(track, index, duration int, overlay string) string {
		return fmt.Sprintf("[vs_layer_%[1]d_%[2]d];color=c=black@0:s=1280x720:r=25:d=%[3]s,trim=end_frame=%[4]d,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_%[1]d_%[2]d];[vs_clip_bg_%[1]d_%[2]d][vs_layer_%[1]d_%[2]d]%[5]s[vs_clip_%[1]d_%[2]d]", track, index, pal.SecondsArg(duration), duration, overlay)
	}
	place := func(track, index int, b boxExpr, half int) string {
		return fmt.Sprintf("overlay@vs_place_%d_%d=x='%s':y='%s':eval=frame:format=auto:shortest=1", track, index,
			exprOp("-", b.cx, exprNum(float64(half))).String(), exprOp("-", b.cy, exprNum(float64(half))).String())
	}

	// Contain, width keyframed 320 → 640 px: the pad canvas is the largest box (640x360), no rotate, no commands.
	contain := Clip{ID: "c", AssetID: "a", Duration: 50, X: 0.1, Y: 0.2, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{"width": {{F: 0, V: 0.25, E: EaseLinear}, {F: 40, V: 0.5, E: EaseLinear}}}}
	c := chain(1, 2, 3, contain)
	b := boxExprs(contain, 0, nil, 1280, 720, pal)
	want := head(3, 50) +
		fmt.Sprintf("scale@vs_scale_1_2=w='%s':h='%s':force_original_aspect_ratio=decrease:force_divisible_by=2:eval=frame,pad@vs_pad_1_2=w=640:h=360:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame,format=rgba", b.w.String(), b.h.String()) +
		tail(1, 2, 50, fmt.Sprintf("overlay@vs_place_1_2=x='%s':y='%s':eval=frame:format=auto:shortest=1",
			exprOp("-", b.cx, exprNum(320)).String(), exprOp("-", b.cy, exprNum(180)).String()))
	if got := c.filter("vs_clip_1_2"); got != want {
		t.Fatalf("contain:\n got %s\nwant %s", got, want)
	}
	if _, ok := c.commandFile(); ok {
		t.Fatal("contain geometry needs no command file")
	}
	// The layer canvas sits around the box centre in whole pixels: frame 0 has a 320 px box at x 128, frame 45 a 640 px box.
	x := exprOp("-", b.cx, exprNum(320))
	if got := x.eval(map[string]float64{"t": 0}); got != -32 {
		t.Fatalf("overlay x at frame 0 = %v, want -32", got)
	}
	if got := x.eval(map[string]float64{"t": float64(45) * (1.0 / 25)}); got != 128 {
		t.Fatalf("overlay x at frame 45 = %v, want 128", got)
	}

	// Cover with rotation: aspect clamp, cover scale, bound pad, commanded crop, canvas pad with a 2 px ring, rotate.
	cover := Clip{ID: "v", AssetID: "a", Duration: 30, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitCover,
		Keyframes: map[string][]Keyframe{
			"width":    {{F: 0, V: 0.25, E: EaseLinear}, {F: 20, V: 0.5, E: EaseLinear}},
			"rotation": {{F: 0, V: 0, E: EaseLinear}, {F: 20, V: 90, E: EaseOut}},
		}}
	cc := chain(0, 0, 0, cover)
	g := cc.geometry()
	if !g.rotated || g.widths[0] != 320 || g.widths[1] != 336 || g.widths[29] != 640 || g.heights[0] != 360 || g.padWidth != 642 || g.padHeight != 362 || g.layerWidth != 738 || g.layerHeight != 738 {
		t.Fatalf("cover geometry = %+v", g)
	}
	boundWidth, boundHeight, low, high := coverBound(g.widths, g.heights, 16.0/9)
	want = head(0, 30) + "sendcmd=f=vs_cmd_0_0.txt," +
		fmt.Sprintf("crop=w='min(iw,ih*%s)':h='min(ih,iw/%s)',scale@vs_scale_0_0=w='%s':h='%s':force_original_aspect_ratio=increase:force_divisible_by=2:eval=frame,pad=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame,crop@vs_crop_0_0=w=320:h=360,pad@vs_pad_0_0=w=642:h=362:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame,format=rgba,rotate@vs_rotate_0_0=a='%s':ow=738:oh=738:c=black@0",
			exprNum(high).String(), exprNum(low).String(), g.box.w.String(), g.box.h.String(), boundWidth, boundHeight, g.box.angle.String()) +
		tail(0, 0, 30, place(0, 0, g.box, 369))
	if got := cc.filter("vs_clip_0_0"); got != want {
		t.Fatalf("cover:\n got %s\nwant %s", got, want)
	}
	file, ok := cc.commandFile()
	if !ok || file.name != "vs_cmd_0_0.txt" || len(file.tracks) != 2 || file.tracks[0].Target != "crop@vs_crop_0_0" || file.tracks[0].Option != "w" ||
		file.tracks[1].Option != "h" || file.tracks[0].Values[1] != 336 || file.tracks[1].Values[29] != 360 {
		t.Fatalf("cover command file = %+v, %v", file, ok)
	}

	// Opacity only keeps the static transform and adds the named, command-driven colorchannelmixer.
	fade := Clip{ID: "c", AssetID: "a", Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{"opacity": {{F: 0, V: 0, E: EaseLinear}, {F: 25, V: 1, E: EaseLinear}}}}
	oc := clipChain{item: layerClip{clip: fade, whole: fade}, canvasWidth: 1280, canvasHeight: 720, rate: Rate{Num: 30, Den: 1}}
	want = `[0:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=30,setpts='max(PTS-0,-60)',tpad=stop_mode=clone:stop=120,trim=start_pts=0:end_pts=60,setpts=PTS-STARTPTS,sendcmd=f=vs_cmd_0_0.txt,scale=w=1280:h=720:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=1280:h=720:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba,colorchannelmixer@vs_opacity_0_0=aa=0[vs_layer_0_0];color=c=black@0:s=1280x720:r=30:d=2.000000,trim=end_frame=60,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_0];[vs_clip_bg_0_0][vs_layer_0_0]overlay=x=0:y=0:format=auto:shortest=1[vs_clip_0_0]`
	if got := oc.filter("vs_clip_0_0"); got != want {
		t.Fatalf("opacity only:\n got %s\nwant %s", got, want)
	}
	file, ok = oc.commandFile()
	if !ok || len(file.tracks) != 1 || file.tracks[0].Target != "colorchannelmixer@vs_opacity_0_0" || file.tracks[0].Option != "aa" || len(file.tracks[0].Values) != 60 {
		t.Fatalf("opacity command file = %+v, %v", file, ok)
	}
	if values := file.tracks[0].Values; values[0] != 0 || values[25] != 1 || values[59] != 1 || values[10] < 0.4-1e-12 || values[10] > 0.4+1e-12 {
		t.Fatalf("opacity values %v", values)
	}
}

func TestCoverBound(t *testing.T) {
	// A constant box aspect: the clamp makes the bound the largest box plus the margin (1/8 and 4 px), whatever the source.
	for _, aspect := range []float64{2, 0.5, 16.0 / 9, 0} {
		if w, h, low, high := coverBound([]int{400, 200}, []int{200, 100}, aspect); w != 454 || h != 230 || low != 2 || high != 2 {
			t.Fatalf("aspect %v: bound %dx%d, range %v..%v; want 454x230, 2..2", aspect, w, h, low, high)
		}
	}
	// Boxes from 1:1 to 4:1 with a 2:1 source (1:2 after autorotation): the picture aspect lies in 1..2.
	if w, h, low, high := coverBound([]int{200, 400}, []int{200, 100}, 2); w != 454 || h != 454 || low != 1 || high != 4 {
		t.Fatalf("2:1 source: bound %dx%d, range %v..%v; want 454x454, 1..4", w, h, low, high)
	}
	// Without a source aspect the whole clamp range counts.
	if w, h, _, _ := coverBound([]int{200, 400}, []int{200, 100}, 0); w != 904 || h != 454 {
		t.Fatalf("no source aspect: bound %dx%d, want 904x454", w, h)
	}
}

func TestClipChainRampSnapshot(t *testing.T) {
	ramp := []RampPoint{{F: 0, Speed: 1, E: EaseLinear}, {F: 50, Speed: 3, E: EaseLinear}}
	clip := Clip{ID: "r", AssetID: "a", Offset: 20, Duration: 50, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Ramp: ramp}
	pieces := rampPieces(ramp, 0, 50)
	pal := Rate{Num: 25, Den: 1}
	item := layerClip{clip: clip, whole: clip, from: 20, first: 20, pieces: pieces}
	got := clipChain{item: item, canvasWidth: 1280, canvasHeight: 720, rate: pal}.filter("vs_clip_0_0")
	want := "[0:V:0]setpts='if(eq(N,0),min(PTS,floor(0.800000000/TB)),PTS)',settb=AVTB,setpts='st(0,(PTS*TB-0.001)*25/1-20);(20+ceil(" +
		rampInverseExpr(pieces, exprLd(0)).String() +
		"))*1/25/TB',fps=25,setpts='max(PTS-20,-50)',tpad=stop_mode=clone:stop=100,trim=start_pts=0:end_pts=50,setpts=PTS-STARTPTS," +
		"scale=w=1280:h=720:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=1280:h=720:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba[vs_layer_0_0];" +
		"color=c=black@0:s=1280x720:r=25:d=2.000000,trim=end_frame=50,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_0];[vs_clip_bg_0_0][vs_layer_0_0]overlay=x=0:y=0:format=auto:shortest=1[vs_clip_0_0]"
	if got != want {
		t.Fatalf("ramp chain:\n got %s\nwant %s", got, want)
	}
	// Without ramp pieces the clip keeps its phase 1 retime.
	plain := item
	plain.pieces = nil
	if got := (clipChain{item: plain, rate: pal}).retime(); got != videoResample(20, 20, 1, pal) {
		t.Fatalf("unramped retime = %s", got)
	}
}
