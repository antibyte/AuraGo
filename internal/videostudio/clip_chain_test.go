package videostudio

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"regexp"
	"slices"
	"strings"
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
	if files := c.commandFiles(); len(files) != 0 {
		t.Fatalf("contain geometry needs no command file: %+v", files)
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
	files := cc.commandFiles()
	file, ok := sendCmdFile{}, len(files) == 1
	if ok {
		file = files[0]
	}
	if !ok || file.name != "vs_cmd_0_0.txt" || len(file.tracks) != 2 || file.tracks[0].Target != "crop@vs_crop_0_0" || file.tracks[0].Option != "w" ||
		file.tracks[1].Option != "h" || file.tracks[0].Values[1] != 336 || file.tracks[1].Values[29] != 360 {
		t.Fatalf("cover command file = %+v, %v", file, ok)
	}
	if built := cc.build("vs_clip_0_0"); built.filter != cc.filter("vs_clip_0_0") || !reflect.DeepEqual(built.commands, files) || built.luts != nil {
		t.Fatalf("build differs from filter and commandFiles: %+v", built)
	}

	// Opacity only keeps the static transform and adds the named, command-driven colorchannelmixer.
	fade := Clip{ID: "c", AssetID: "a", Duration: 60, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{"opacity": {{F: 0, V: 0, E: EaseLinear}, {F: 25, V: 1, E: EaseLinear}}}}
	oc := clipChain{item: layerClip{clip: fade, whole: fade}, canvasWidth: 1280, canvasHeight: 720, rate: Rate{Num: 30, Den: 1}}
	want = `[0:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=30,setpts='max(PTS-0,-60)',tpad=stop_mode=clone:stop=120,trim=start_pts=0:end_pts=60,setpts=PTS-STARTPTS,sendcmd=f=vs_cmd_0_0.txt,scale=w=1280:h=720:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=1280:h=720:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba,colorchannelmixer@vs_opacity_0_0=aa=0[vs_layer_0_0];color=c=black@0:s=1280x720:r=30:d=2.000000,trim=end_frame=60,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_0];[vs_clip_bg_0_0][vs_layer_0_0]overlay=x=0:y=0:format=auto:shortest=1[vs_clip_0_0]`
	if got := oc.filter("vs_clip_0_0"); got != want {
		t.Fatalf("opacity only:\n got %s\nwant %s", got, want)
	}
	files = oc.commandFiles()
	file, ok = sendCmdFile{}, len(files) == 1
	if ok {
		file = files[0]
	}
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

// TestClipChainPictureSnapshots pins the picture stage and the animated crop (index §4.10, plan 2c Task 7). The decoded
// picture is 1920x1080; crop rectangles follow cropRect and mirroredCrop, cover windows coverWindow.
func TestClipChainPictureSnapshots(t *testing.T) {
	pal := Rate{Num: 25, Den: 1}
	chain := func(index int, clip Clip) clipChain {
		return clipChain{track: 0, index: index, item: layerClip{clip: clip, whole: clip, at: clip.Start, input: index, pictureWidth: 1920, pictureHeight: 1080, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: pal}
	}
	head := func(input, duration int) string {
		return fmt.Sprintf("[%d:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=25,setpts='max(PTS-0,-%d)',tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS,", input, duration, duration*2, duration)
	}
	tail := func(index, duration int, overlay string) string {
		return fmt.Sprintf("[vs_layer_0_%[1]d];color=c=black@0:s=1280x720:r=25:d=%[2]s,trim=end_frame=%[3]d,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_%[1]d];[vs_clip_bg_0_%[1]d][vs_layer_0_%[1]d]%[4]s[vs_clip_0_%[1]d]", index, pal.SecondsArg(duration), duration, overlay)
	}
	staticOverlay := "overlay=x=0:y=0:format=auto:shortest=1"
	pictureFile := func(c clipChain, name string, frames ...[5]int) {
		t.Helper()
		files := c.commandFiles()
		if len(files) != 1 || files[0].name != name || len(files[0].tracks) != 4 {
			t.Fatalf("%s: command files %+v", name, files)
		}
		target := fmt.Sprintf("crop@vs_picture_0_%d", c.index)
		for i, option := range []string{"w", "h", "x", "y"} {
			if track := files[0].tracks[i]; track.Target != target || track.Option != option {
				t.Fatalf("%s: track %d is %s %s, want %s %s", name, i, track.Target, track.Option, target, option)
			}
		}
		for _, f := range frames { // {frame, w, h, x, y}
			for i := range 4 {
				if got := files[0].tracks[i].Values[f[0]]; got != float64(f[i+1]) {
					t.Fatalf("%s: frame %d option %s = %v, want %d", name, f[0], files[0].tracks[i].Option, got, f[i+1])
				}
			}
		}
	}

	// timedScale checks the rule of index risk 14 on a chain with crop keyframes: the filter right after crop@vs_picture
	// is a scale with eval=frame whose w and h both read t. FFmpeg 5.1 reconfigures its scaler only then; a
	// constant-size scale there hangs.
	timedScale := func(name, filter string) {
		t.Helper()
		_, rest, found := strings.Cut(filter, "crop@vs_picture_")
		if found {
			_, rest, found = strings.Cut(rest, ":exact=1,")
		}
		if !found {
			t.Fatalf("%s: no crop@vs_picture in %s", name, filter)
		}
		next, quoted := rest, false
		for i, r := range rest {
			if r == '\'' {
				quoted = !quoted
			} else if r == ',' && !quoted {
				next = rest[:i]
				break
			}
		}
		option := func(prefix string) string {
			_, value, _ := strings.Cut(next, prefix+"='")
			value, _, _ = strings.Cut(value, "'")
			return value
		}
		readsT := regexp.MustCompile(`\bt\b`)
		if !strings.HasPrefix(next, "scale") || !strings.Contains(next, ":eval=frame") || !readsT.MatchString(option("w")) || !readsT.MatchString(option(":h")) {
			t.Fatalf("%s: the filter after crop@vs_picture is %q; want a scale with eval=frame and t in both w and h", name, next)
		}
	}

	// Static crop with both flips: cropRect as iw/ih expressions, the flips after it, then the unchanged 2a transform.
	static := Clip{ID: "s", AssetID: "a", Duration: 50, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, FlipH: true, FlipV: true,
		Crop: &Crop{L: 0.1, T: 0, R: 0.2, B: 0.05}}
	sc := chain(0, static)
	want := head(0, 50) + staticCropFilter(*static.Crop) + ",hflip,vflip," +
		"scale=w=1280:h=720:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=1280:h=720:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0,format=rgba" +
		tail(0, 50, staticOverlay)
	if got := sc.filter("vs_clip_0_0"); got != want {
		t.Fatalf("static crop:\n got %s\nwant %s", got, want)
	}
	if files := sc.commandFiles(); len(files) != 0 {
		t.Fatalf("a static crop needs no commands: %+v", files)
	}

	// Crop keyframes on a contain clip with flip_h: the flip runs first, the mirrored crop follows the (empty) effects
	// in front of an eval=frame scale and pad.
	contain := Clip{ID: "c", AssetID: "a", Duration: 11, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, FlipH: true,
		Crop: &Crop{L: 0.1, T: 0.05, R: 0.1},
		Keyframes: map[string][]Keyframe{
			"crop.l": {{F: 0, V: 0.1, E: EaseLinear}, {F: 10, V: 0.3, E: EaseLinear}},
			"crop.b": {{F: 0, V: 0, E: EaseLinear}, {F: 10, V: 0.2, E: EaseLinear}},
		}}
	cc := chain(1, contain)
	want = head(1, 11) + "sendcmd=f=vs_cmd_0_1.txt,hflip," +
		"crop@vs_picture_0_1=w=1536:h=1026:x=192:y=54:exact=1," +
		"scale=w='1280+0*t':h='720+0*t':force_original_aspect_ratio=decrease:force_divisible_by=2:eval=frame,pad=w=1280:h=720:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame,format=rgba" +
		tail(1, 11, staticOverlay)
	if got := cc.filter("vs_clip_0_1"); got != want {
		t.Fatalf("animated contain crop:\n got %s\nwant %s", got, want)
	}
	// flip_h mirrors l and r: at frame 10 the user's l = 0.3 trims the right edge of the flipped picture.
	pictureFile(cc, "vs_cmd_0_1.txt", [5]int{0, 1536, 1026, 192, 54}, [5]int{5, 1344, 918, 192, 54}, [5]int{10, 1152, 810, 192, 54})
	timedScale("animated contain crop", cc.filter("vs_clip_0_1"))

	// Crop keyframes on a cover clip with flip_h: the crop is the cover window of the 16:9 box, scaled exactly.
	cover := Clip{ID: "v", AssetID: "a", Duration: 21, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitCover, FlipH: true,
		Crop:      &Crop{L: 0.2, B: 0.1},
		Keyframes: map[string][]Keyframe{"crop.l": {{F: 0, V: 0.2, E: EaseLinear}, {F: 20, V: 0.4, E: EaseLinear}}}}
	vc := chain(2, cover)
	want = head(2, 21) + "sendcmd=f=vs_cmd_0_2.txt,hflip,crop@vs_picture_0_2=w=1536:h=864:x=0:y=54:exact=1,scale=w='1280+0*t':h='720+0*t':eval=frame,format=rgba" + tail(2, 21, staticOverlay)
	if got := vc.filter("vs_clip_0_2"); got != want {
		t.Fatalf("animated cover crop:\n got %s\nwant %s", got, want)
	}
	pictureFile(vc, "vs_cmd_0_2.txt", [5]int{0, 1536, 864, 0, 54}, [5]int{20, 1152, 648, 0, 162})
	timedScale("animated cover crop", vc.filter("vs_clip_0_2"))

	// Keyframed geometry and crop on a cover clip: the window follows each frame's box aspect; scale@vs_scale fills the
	// box exactly; there is no aspect clamp, no cover bound and no crop@vs_crop.
	keyed := Clip{ID: "k", AssetID: "a", Duration: 30, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitCover,
		Crop: &Crop{T: 0.1},
		Keyframes: map[string][]Keyframe{
			"width":  {{F: 0, V: 0.25, E: EaseLinear}, {F: 20, V: 0.5, E: EaseLinear}},
			"crop.r": {{F: 0, V: 0, E: EaseLinear}, {F: 20, V: 0.3, E: EaseLinear}},
		}}
	kc := chain(3, keyed)
	g := kc.geometry()
	if g.padWidth != 640 || g.padHeight != 360 || g.rotated {
		t.Fatalf("keyed geometry = %+v", g)
	}
	want = head(3, 30) + "sendcmd=f=vs_cmd_0_3.txt," +
		fmt.Sprintf("crop@vs_picture_0_3=w=864:h=972:x=528:y=108:exact=1,scale@vs_scale_0_3=w='%s':h='%s':eval=frame,pad@vs_pad_0_3=w=640:h=360:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame,format=rgba", g.box.w.String(), g.box.h.String()) +
		tail(3, 30, fmt.Sprintf("overlay@vs_place_0_3=x='%s':y='%s':eval=frame:format=auto:shortest=1", exprOp("-", g.box.cx, exprNum(320)).String(), exprOp("-", g.box.cy, exprNum(180)).String()))
	if got := kc.filter("vs_clip_0_3"); got != want {
		t.Fatalf("animated crop with keyframed cover geometry:\n got %s\nwant %s", got, want)
	}
	pictureFile(kc, "vs_cmd_0_3.txt", [5]int{0, 864, 972, 528, 108}, [5]int{29, 1344, 756, 0, 216})
	timedScale("animated crop with keyframed cover geometry", kc.filter("vs_clip_0_3"))
	for _, track := range kc.commandFiles()[0].tracks {
		if strings.HasPrefix(track.Target, "crop@vs_crop") {
			t.Fatalf("the composed window replaces the cover crop, but %s is commanded", track.Target)
		}
	}

	// Keyframed position and crop on a contain clip: the unchanged 2b contain scale follows crop@vs_picture, and its box
	// expressions read t.
	moving := Clip{ID: "m", AssetID: "a", Duration: 20, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{
			"x":      {{F: 0, V: 0, E: EaseLinear}, {F: 19, V: 0.5, E: EaseLinear}},
			"crop.t": {{F: 0, V: 0, E: EaseLinear}, {F: 19, V: 0.4, E: EaseLinear}},
		}}
	timedScale("animated crop with keyframed contain position", chain(4, moving).filter("vs_clip_0_4"))
}

// TestCropRectExprMatchesCropRect evaluates the static crop's iw/ih expressions with FFmpeg's arithmetic (ffExpr.eval)
// and compares them with cropRect (index §4.20). It covers odd sizes and crops with clamped r and b. The half-pixel
// crop is exact in binary at 1024x512, so it checks the round-half-away-from-zero rule.
func TestCropRectExprMatchesCropRect(t *testing.T) {
	sizes := [][2]int{{1, 1}, {3, 3}, {1, 7}, {5, 2}, {64, 64}, {65, 97}, {640, 360}, {641, 361}, {1024, 512}, {1080, 1920}, {1919, 1081}, {3840, 2160}, {4097, 2161}}
	crops := []Crop{
		{L: 0.1, T: 0, R: 0.2, B: 0.05},
		{L: 0.95}, {R: 0.95}, {T: 0.95}, {B: 0.95},
		{L: 0.5, T: 0.6, R: 0.5, B: 0.6},
		{L: 3.0 / 1024, T: 5.0 / 512, R: 7.0 / 1024, B: 9.0 / 512},
		{L: 0.25, T: 0.25, R: 0.125, B: 0.125},
	}
	random := rand.New(rand.NewPCG(7, 11))
	for range 300 {
		crops = append(crops, Crop{L: random.Float64() * 0.95, T: random.Float64() * 0.95, R: random.Float64() * 0.95, B: random.Float64() * 0.95})
	}
	for _, size := range sizes {
		vars := map[string]float64{"iw": float64(size[0]), "ih": float64(size[1])}
		for _, crop := range crops {
			ex, ey, ew, eh := cropRectExprs(crop)
			got := [4]int{int(ex.eval(vars)), int(ey.eval(vars)), int(ew.eval(vars)), int(eh.eval(vars))}
			x, y, w, h := cropRect(crop, size[0], size[1])
			if want := [4]int{x, y, w, h}; got != want {
				t.Fatalf("%dx%d crop %+v: expressions give %v, cropRect %v", size[0], size[1], crop, got, want)
			}
		}
	}
	if got := staticCropFilter(Crop{L: 0.25, T: 0.1}); !strings.HasPrefix(got, "crop=w='min((iw-min((round(((0.25*iw)/2))*2),max(0,(iw-2)))),max(2,((iw-min((round(((0.25*iw)/2))*2),max(0,(iw-2))))-") || !strings.HasSuffix(got, ":exact=1") {
		t.Fatalf("static crop filter = %s", got)
	}
}

// TestClipChainFitOrientSplit checks the 2d split of the transform stage (index §4.10):
//   - transform() is fit() + "," + orient();
//   - stages() holds fit (with transformTracks), then the mask stage (empty without a mask), then the reveal stage
//     (empty without a wipe, 2e), then orient;
//   - fit leaves the picture in box space, with no RGBA conversion and no rotation;
//   - orient starts with format=rgba;
//   - the built chain holds fit + "," + orient.
//
// The snapshots above pin the chains themselves, which this split leaves byte-identical.
func TestClipChainFitOrientSplit(t *testing.T) {
	pal := Rate{Num: 25, Den: 1}
	var chains []clipChain
	for _, s := range clipChainSnapshots() {
		chains = append(chains, clipChain{track: s.track, index: s.index, item: s.item, canvasWidth: s.width, canvasHeight: s.height, rate: s.rate})
	}
	keyed := func(fit FitMode, rotation float64, keys map[string][]Keyframe) clipChain {
		clip := Clip{ID: "k", AssetID: "a", Duration: 30, Width: 0.5, Height: 0.5, Rotation: rotation, Opacity: 1, Volume: 1, Fit: fit, Keyframes: keys}
		return clipChain{item: layerClip{clip: clip, whole: clip, at: clip.Start, pictureWidth: 1920, pictureHeight: 1080, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: pal}
	}
	chains = append(chains,
		keyed(FitContain, 0, map[string][]Keyframe{"width": linearKeys(0.25, 0.5, 20)}),
		keyed(FitCover, 0, map[string][]Keyframe{"width": linearKeys(0.25, 0.5, 20), "rotation": linearKeys(0, 90, 20)}),
		keyed(FitContain, 15, map[string][]Keyframe{"crop.l": linearKeys(0, 0.3, 20)}),
		keyed(FitCover, 0, map[string][]Keyframe{"crop.r": linearKeys(0, 0.3, 20)}),
		keyed(FitCover, 0, map[string][]Keyframe{"width": linearKeys(0.25, 0.5, 20), "crop.r": linearKeys(0, 0.3, 20)}),
		keyed(FitContain, 0, map[string][]Keyframe{"opacity": linearKeys(0, 1, 20)}),
	)
	for i, c := range chains {
		fit, orient := c.fit(), c.orient()
		if got := c.transform(); got != fit+","+orient {
			t.Fatalf("chain %d: transform %q is not fit + orient", i, got)
		}
		if fit == "" || strings.Contains(fit, "format=rgba") || strings.Contains(fit, "rotate") || !strings.HasPrefix(orient, "format=rgba") {
			t.Fatalf("chain %d: fit %q, orient %q", i, fit, orient)
		}
		stages := c.stages()
		at := slices.IndexFunc(stages, func(s chainStage) bool { return s.filter == fit })
		if at < 0 || at+3 >= len(stages) || stages[at+3].filter != orient || stages[at+3].tracks != nil || !reflect.DeepEqual(stages[at].tracks, c.transformTracks()) || !reflect.DeepEqual(stages[at+2], chainStage{}) {
			t.Fatalf("chain %d: stages %+v do not hold fit, mask, reveal (empty) and orient in that order", i, stages)
		}
		if !strings.Contains(c.filter("vs_clip_x"), fit+","+orient) {
			t.Fatalf("chain %d: the built chain lacks %q", i, fit+","+orient)
		}
	}
}

// TestClipChainMaskSnapshots pins the mask stage between fit and orient (index §4.10, §4.22):
//   - no mask, no stage;
//   - static rect: the whole chain, including the geq expression of Go numbers;
//   - static ellipse, rotated and inverted;
//   - a keyed mask on a static box: the framed generator at a quarter of the box, upscaled to it; its frame values are
//     computed at the first pixel of every row and its pixel step works in registers 3, 4 and 6 only;
//   - a keyed, rotating box: the pad canvas, the box sizes in T, orient's rotate after the merge;
//   - cover with crop keys: the t-term scale stays in front of the mask;
//   - opacity keyframes read command group 1 behind the merge.
//
// The mask branch never holds sendcmd or gblur.
func TestClipChainMaskSnapshots(t *testing.T) {
	pal := Rate{Num: 25, Den: 1}
	chain := func(index int, clip Clip) clipChain {
		return clipChain{track: 0, index: index, item: layerClip{clip: clip, whole: clip, at: clip.Start, input: index, pictureWidth: 1920, pictureHeight: 1080, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: pal}
	}
	head := func(input, duration int) string {
		return fmt.Sprintf("[%d:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=25,setpts='max(PTS-0,-%d)',tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS,", input, duration, duration*2, duration)
	}
	tail := func(index, duration int, overlay string) string {
		return fmt.Sprintf("[vs_layer_0_%[1]d];color=c=black@0:s=1280x720:r=25:d=%[2]s,trim=end_frame=%[3]d,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_%[1]d];[vs_clip_bg_0_%[1]d][vs_layer_0_%[1]d]%[4]s[vs_clip_0_%[1]d]", index, pal.SecondsArg(duration), duration, overlay)
	}
	merge := func(index int, generator string) string {
		return fmt.Sprintf("format=gbrap[vs_mk_0_%[1]d_in];%[2]s[vs_mk_0_%[1]d_m];[vs_mk_0_%[1]d_in][vs_mk_0_%[1]d_m]blend=c0_mode=normal:c1_mode=normal:c2_mode=normal:c3_mode=multiply", index, generator)
	}
	static := func(c clipChain) string {
		plan := maskPlanOf(c)
		return fmt.Sprintf("nullsrc=s=%dx%d:r=25:d=0.040000,format=gbrap,geq=r='0':g='0':b='0':a='%s',loop=loop=-1:size=1:start=0,trim=end_frame=%d,setpts=PTS-STARTPTS",
			plan.canvasWidth, plan.canvasHeight, maskExpr(c, plan).String(), c.item.clip.Duration)
	}
	framed := func(c clipChain) string {
		plan := maskPlanOf(c)
		return fmt.Sprintf("nullsrc=s=%dx%d:r=25:d=%s,trim=end_frame=%d,setpts=PTS-STARTPTS,format=gbrap,geq=r='0':g='0':b='0':a='%s',scale=w=%d:h=%d:flags=bilinear",
			plan.gridWidth, plan.gridHeight, pal.SecondsArg(c.item.clip.Duration), c.item.clip.Duration, maskExpr(c, plan).String(), plan.canvasWidth, plan.canvasHeight)
	}
	containFit := "scale=w=640:h=360:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=640:h=360:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0"
	staticOverlay := "overlay=x=320:y=180:format=auto:shortest=1"
	clip := func(edit func(*Clip)) Clip {
		c := Clip{ID: "m", AssetID: "a", Duration: 10, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain,
			Mask: &Mask{Shape: MaskRect, X: 0.25, Y: 0.25, W: 0.5, H: 0.5}}
		edit(&c)
		return c
	}

	if got := chain(0, clip(func(c *Clip) { c.Mask = nil })).mask(); !reflect.DeepEqual(got, chainStage{}) {
		t.Fatalf("a clip without a mask has the mask stage %+v", got)
	}

	// Static rect in the 640x360 box: centre (320, 180), radii 160 and 90, rotation 0, F = 1 px (feather 0).
	rect := chain(0, clip(func(*Clip) {}))
	parts := []string{
		"st(3,((X+0.5)-320))", "st(4,((Y+0.5)-180))",
		"st(6,((ld(3)*1)+(ld(4)*0)))", "st(4,((ld(4)*1)-(ld(3)*0)))",
		"st(2,(abs(ld(6))-160))", "st(5,(abs(ld(4))-90))",
		"st(6,(hypot(max(ld(2),0),max(ld(5),0))+min(max(ld(2),ld(5)),0)))",
		"(255*clip((0.5-(ld(6)/1)),0,1))",
	}
	rectExpr := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		rectExpr = "(" + parts[i] + ";" + rectExpr + ")"
	}
	if got := maskExpr(rect, maskPlanOf(rect)).String(); got != rectExpr {
		t.Fatalf("static rect expression:\n got %s\nwant %s", got, rectExpr)
	}
	want := head(0, 10) + containFit + "," + merge(0, static(rect)) + ",format=rgba" + tail(0, 10, staticOverlay)
	if got := rect.filter("vs_clip_0_0"); got != want {
		t.Fatalf("static rect:\n got %s\nwant %s", got, want)
	}
	if stage := rect.mask(); !stage.twoInput || stage.tracks != nil || strings.Contains(stage.filter, "sendcmd") || strings.Contains(stage.filter, "gblur") {
		t.Fatalf("mask stage %+v", stage)
	}
	if files := rect.commandFiles(); len(files) != 0 {
		t.Fatalf("a static mask needs no commands: %+v", files)
	}

	// Static ellipse, rotated by 30° and inverted, F = 0.05·360 = 18 px.
	ellipse := chain(1, clip(func(c *Clip) {
		c.Mask.Shape, c.Mask.Rotation, c.Mask.Feather, c.Mask.Invert = MaskEllipse, 30, 0.05, true
	}))
	text := maskExpr(ellipse, maskPlanOf(ellipse)).String()
	degrees := 30.0 // a variable: float64 arithmetic at run time, as maskFrameOf and the export compute it (a constant would round once)
	if !strings.Contains(text, "st(3,max(hypot(ld(2),ld(5)),0.000001))") || !strings.Contains(text, "(ld(3)*"+exprNum(math.Cos(degrees*math.Pi/180)).String()+")") ||
		!strings.Contains(text, "(255*(1-clip((0.5-(ld(6)/18)),0,1)))") || strings.Contains(text, "T") {
		t.Fatalf("static ellipse expression %s", text)
	}
	if got, want := ellipse.filter("vs_clip_0_1"), head(1, 10)+containFit+","+merge(1, static(ellipse))+",format=rgba"+tail(1, 10, staticOverlay); got != want {
		t.Fatalf("static ellipse:\n got %s\nwant %s", got, want)
	}

	// A keyed mask on the static box: geq per frame at 160x90, upscaled to 640x360.
	keyed := chain(2, clip(func(c *Clip) {
		c.Keyframes = map[string][]Keyframe{"mask.x": linearKeys(0.1, 0.4, 9), "mask.rotation": linearKeys(0, 90, 9)}
	}))
	if plan := maskPlanOf(keyed); !plan.perFrame || plan.canvasWidth != 640 || plan.canvasHeight != 360 || plan.gridWidth != 160 || plan.gridHeight != 90 {
		t.Fatalf("keyed mask plan %+v", plan)
	}
	if got, want := keyed.filter("vs_clip_0_2"), head(2, 10)+containFit+","+merge(2, framed(keyed))+",format=rgba"+tail(2, 10, staticOverlay); got != want {
		t.Fatalf("keyed mask:\n got %s\nwant %s", got, want)
	}
	// The rectangle's pixel step, in the registers the row's frame values leave free: dx 3, dy 4, lx 6, ly 4, qx 3, qy 4, s 6.
	pixel := []string{
		"st(3,(((X+0.5)*4)-ld(2)))", "st(4,(((Y+0.5)*4)-ld(5)))",
		"st(6,((ld(3)*ld(1))+(ld(4)*ld(7))))", "st(4,((ld(4)*ld(1))-(ld(3)*ld(7))))",
		"st(3,(abs(ld(6))-ld(9)))", "st(4,(abs(ld(4))-ld(0)))",
		"st(6,(hypot(max(ld(3),0),max(ld(4),0))+min(max(ld(3),ld(4)),0)))",
		"(255*clip((0.5-(ld(6)/ld(8))),0,1))",
	}
	pixelExpr := pixel[len(pixel)-1]
	for i := len(pixel) - 2; i >= 0; i-- {
		pixelExpr = "(" + pixel[i] + ";" + pixelExpr + ")"
	}
	if text := maskExpr(keyed, maskPlanOf(keyed)).String(); !strings.HasPrefix(text, "(if(lt(X,0.5),(st(5,640);(st(6,360);") || !strings.HasSuffix(text, ",0);"+pixelExpr+")") {
		t.Fatalf("keyed rectangle expression:\n%s\nwant frame values once per row, then %s", text, pixelExpr)
	}
	keyedEllipse := chain(2, clip(func(c *Clip) {
		c.Mask.Shape = MaskEllipse
		c.Keyframes = map[string][]Keyframe{"mask.x": linearKeys(0.1, 0.4, 9)}
	}))
	if text := maskExpr(keyedEllipse, maskPlanOf(keyedEllipse)).String(); !strings.Contains(text, "(st(6,(ld(6)/ld(9)));(st(4,(ld(4)/ld(0)));(st(3,max(hypot(ld(6),ld(4)),0.000001));(st(6,(((ld(3)-1)*ld(3))/max(sqrt((((ld(6)*ld(6))/(ld(9)*ld(9)))+((ld(4)*ld(4))/(ld(0)*ld(0))))),0.000000000001)));") {
		t.Fatalf("keyed ellipse expression:\n%s", text)
	}

	// A keyed, rotating box: the mask fills the pad canvas with the box centred, then orient rotates both.
	box := chain(3, clip(func(c *Clip) {
		c.Keyframes = map[string][]Keyframe{"width": linearKeys(0.3, 0.5, 9), "rotation": linearKeys(0, 20, 9)}
	}))
	g := box.geometry()
	plan := maskPlanOf(box)
	if !plan.perFrame || plan.canvasWidth != g.padWidth || plan.canvasHeight != g.padHeight || plan.gridWidth != (g.padWidth+3)/4 || plan.gridHeight != (g.padHeight+3)/4 {
		t.Fatalf("keyed box plan %+v, pad %dx%d", plan, g.padWidth, g.padHeight)
	}
	if text := maskExpr(box, plan).String(); !strings.Contains(text, "max(T,0)") || strings.Contains(text, "max(t,") {
		t.Fatalf("the box sizes in the mask must read geq's T: %s", text)
	}
	want = head(3, 10) +
		fmt.Sprintf("scale@vs_scale_0_3=w='%s':h='%s':force_original_aspect_ratio=decrease:force_divisible_by=2:eval=frame,pad@vs_pad_0_3=w=%d:h=%d:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame,", g.box.w.String(), g.box.h.String(), g.padWidth, g.padHeight) +
		merge(3, framed(box)) +
		fmt.Sprintf(",format=rgba,rotate@vs_rotate_0_3=a='%s':ow=%d:oh=%d:c=black@0", g.box.angle.String(), g.layerWidth, g.layerHeight) +
		tail(3, 10, fmt.Sprintf("overlay@vs_place_0_3=x='%s':y='%s':eval=frame:format=auto:shortest=1", exprOp("-", g.box.cx, exprNum(float64(g.layerWidth/2))).String(), exprOp("-", g.box.cy, exprNum(float64(g.layerHeight/2))).String()))
	if got := box.filter("vs_clip_0_3"); got != want {
		t.Fatalf("keyed box:\n got %s\nwant %s", got, want)
	}

	// Cover with crop keys: the scale after crop@vs_picture keeps its t-term, and the static mask follows it.
	cover := chain(4, clip(func(c *Clip) {
		c.Fit = FitCover
		c.Keyframes = map[string][]Keyframe{"crop.l": linearKeys(0, 0.2, 9)}
	}))
	if maskPlanOf(cover).perFrame || !strings.Contains(cover.filter("vs_clip_0_4"), ",scale=w='640+0*t':h='360+0*t':eval=frame,"+merge(4, static(cover))+",format=rgba[vs_layer_0_4]") {
		t.Fatalf("cover with crop keys:\n%s", cover.filter("vs_clip_0_4"))
	}

	// Opacity keyframes after the mask read command group 1, right behind the merge; group 0 stays empty.
	fade := chain(5, clip(func(c *Clip) { c.Keyframes = map[string][]Keyframe{"opacity": linearKeys(0, 1, 9)} }))
	built := fade.build("vs_clip_0_5")
	if !strings.Contains(built.filter, "c3_mode=multiply,sendcmd=f=vs_cmd_0_5_1.txt,format=rgba,colorchannelmixer@vs_opacity_0_5=aa=0[vs_layer_0_5]") || strings.Contains(built.filter, "sendcmd=f=vs_cmd_0_5.txt") {
		t.Fatalf("opacity group after the mask:\n%s", built.filter)
	}
	if len(built.commands) != 1 || built.commands[0].name != "vs_cmd_0_5_1.txt" || len(built.commands[0].tracks) != 1 || built.commands[0].tracks[0].Target != "colorchannelmixer@vs_opacity_0_5" {
		t.Fatalf("command files = %+v", built.commands)
	}
}

// TestClipChainAnimSnapshots pins the 2e chains of fade, slides and pop (index §4.10, §4.24):
//   - fade only: the static transform, the named opacity filter and group 0's commands (clipOpacityAt);
//   - pop: 2b's animated-geometry chain for the whole clip, on the pad canvas of the largest scaled box (1.08× at p = 0.7);
//   - slide_left out on a masked cover clip: the cover crop commands, the per-frame mask on the pad canvas, and opacity in
//     group 1 behind the merge;
//   - an animation without an effective span leaves the chain unchanged.
func TestClipChainAnimSnapshots(t *testing.T) {
	pal := Rate{Num: 25, Den: 1}
	chain := func(index int, clip Clip) clipChain {
		return clipChain{track: 0, index: index, item: layerClip{clip: clip, whole: clip, at: clip.Start, input: index, pictureWidth: 1920, pictureHeight: 1080, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: pal}
	}
	head := func(input, duration int) string {
		return fmt.Sprintf("[%d:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=25,setpts='max(PTS-0,-%d)',tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS,", input, duration, duration*2, duration)
	}
	tail := func(index, duration int, overlay string) string {
		return fmt.Sprintf("[vs_layer_0_%[1]d];color=c=black@0:s=1280x720:r=25:d=%[2]s,trim=end_frame=%[3]d,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_%[1]d];[vs_clip_bg_0_%[1]d][vs_layer_0_%[1]d]%[4]s[vs_clip_0_%[1]d]", index, pal.SecondsArg(duration), duration, overlay)
	}
	base := func(edit func(*Clip)) Clip {
		c := Clip{ID: "a", AssetID: "a", Duration: 40, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 0.8, Volume: 1, Fit: FitContain}
		edit(&c)
		return c
	}
	plain := boxExprs(base(func(*Clip) {}), 0, nil, 1280, 720, pal)

	// Fade in 10 and out 8: the static box, opacity 0.8 × p.
	fade := chain(0, base(func(c *Clip) {
		c.AnimIn, c.AnimOut = &ClipAnim{Preset: "fade", Duration: 10}, &ClipAnim{Preset: "fade", Duration: 8}
	}))
	containFit := "scale=w=640:h=360:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=640:h=360:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0"
	want := head(0, 40) + "sendcmd=f=vs_cmd_0_0.txt," + containFit + ",format=rgba,colorchannelmixer@vs_opacity_0_0=aa=0" + tail(0, 40, "overlay=x=320:y=180:format=auto:shortest=1")
	if got := fade.filter("vs_clip_0_0"); got != want {
		t.Fatalf("fade:\n got %s\nwant %s", got, want)
	}
	files := fade.commandFiles()
	if len(files) != 1 || files[0].name != "vs_cmd_0_0.txt" || len(files[0].tracks) != 1 || files[0].tracks[0].Target != "colorchannelmixer@vs_opacity_0_0" ||
		files[0].tracks[0].Option != "aa" || len(files[0].tracks[0].Values) != 40 {
		t.Fatalf("fade command files = %+v", files)
	}
	for k, v := range files[0].tracks[0].Values {
		if want := clipOpacityAt(fade.item.whole, k); v != want {
			t.Fatalf("fade opacity at frame %d = %v, want clipOpacityAt %v", k, v, want)
		}
	}
	if v := files[0].tracks[0].Values; v[0] != 0 || v[5] != 0.4 || v[20] != 0.8 || v[39] != 0 {
		t.Fatalf("fade opacity values %v", v)
	}

	// Pop in over 10 frames: frame 7 has p = 0.7, the 1.08 peak (640·1.08 → 692, 360·1.08 → 388), which sizes the pad.
	popClip := base(func(c *Clip) { c.AnimIn = &ClipAnim{Preset: "pop", Duration: 10} })
	pop := chain(1, popClip)
	g := pop.geometry()
	if g.rotated || g.padWidth != 692 || g.padHeight != 388 || g.layerWidth != 692 || g.widths[0] != 384 || g.heights[0] != 216 ||
		g.widths[7] != 692 || g.heights[7] != 388 || g.widths[20] != 640 || g.heights[20] != 360 {
		t.Fatalf("pop geometry = %+v", g)
	}
	b := boxExprs(popClip, 0, nil, 1280, 720, pal)
	if b.cx.String() != plain.cx.String() || b.cy.String() != plain.cy.String() || b.angle.String() != plain.angle.String() ||
		!strings.Contains(b.w.String(), "st(7,") || !strings.Contains(b.h.String(), "st(8,") {
		t.Fatalf("pop scales the box only:\nw %s\ncx %s", b.w.String(), b.cx.String())
	}
	want = head(1, 40) + "sendcmd=f=vs_cmd_0_1.txt," +
		fmt.Sprintf("scale@vs_scale_0_1=w='%s':h='%s':force_original_aspect_ratio=decrease:force_divisible_by=2:eval=frame,pad@vs_pad_0_1=w=692:h=388:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0:eval=frame,format=rgba,colorchannelmixer@vs_opacity_0_1=aa=0", b.w.String(), b.h.String()) +
		tail(1, 40, fmt.Sprintf("overlay@vs_place_0_1=x='%s':y='%s':eval=frame:format=auto:shortest=1", exprOp("-", b.cx, exprNum(346)).String(), exprOp("-", b.cy, exprNum(194)).String()))
	if got := pop.filter("vs_clip_0_1"); got != want {
		t.Fatalf("pop:\n got %s\nwant %s", got, want)
	}
	files = pop.commandFiles()
	if len(files) != 1 || files[0].name != "vs_cmd_0_1.txt" || len(files[0].tracks) != 1 || files[0].tracks[0].Values[0] != 0 || files[0].tracks[0].Values[3] != 0.8 {
		t.Fatalf("pop command files = %+v", files)
	}

	// slide_left out on a masked cover clip: the box keeps its size; the centre's x moves; the mask is per frame.
	slide := chain(2, base(func(c *Clip) {
		c.Fit, c.Mask, c.AnimOut = FitCover, &Mask{Shape: MaskRect, X: 0.1, Y: 0.1, W: 0.8, H: 0.8}, &ClipAnim{Preset: "slide_left", Duration: 10}
	}))
	bs := boxExprs(slide.item.whole, 0, nil, 1280, 720, pal)
	if bs.w.String() != plain.w.String() || bs.h.String() != plain.h.String() || bs.cy.String() != plain.cy.String() || !strings.Contains(bs.cx.String(), "st(7,") {
		t.Fatalf("slide_left moves the centre's x only:\ncx %s", bs.cx.String())
	}
	if gs := slide.geometry(); gs.padWidth != 640 || gs.padHeight != 360 {
		t.Fatalf("a slide keeps the box size: pad %dx%d", gs.padWidth, gs.padHeight)
	}
	if plan := maskPlanOf(slide); !plan.perFrame || plan.canvasWidth != 640 || plan.canvasHeight != 360 {
		t.Fatalf("slide mask plan %+v", plan)
	}
	text, last := slide.filter("vs_clip_0_2"), -1
	for _, part := range []string{"sendcmd=f=vs_cmd_0_2.txt,", "crop=w='min(iw,ih*", "scale@vs_scale_0_2=", "crop@vs_crop_0_2=w=640:h=360", "pad@vs_pad_0_2=w=640:h=360",
		"format=gbrap[vs_mk_0_2_in]", "c3_mode=multiply,sendcmd=f=vs_cmd_0_2_1.txt,format=rgba,colorchannelmixer@vs_opacity_0_2=aa=0.8[vs_layer_0_2]", "overlay@vs_place_0_2="} {
		at := strings.Index(text, part)
		if at <= last {
			t.Fatalf("slide chain: %q missing or out of order in\n%s", part, text)
		}
		last = at
	}
	files = slide.commandFiles()
	if len(files) != 2 || files[0].name != "vs_cmd_0_2.txt" || files[0].tracks[0].Target != "crop@vs_crop_0_2" || files[1].name != "vs_cmd_0_2_1.txt" ||
		files[1].tracks[0].Target != "colorchannelmixer@vs_opacity_0_2" || files[1].tracks[0].Values[29] != 0.8 || files[1].tracks[0].Values[39] != 0 {
		t.Fatalf("slide command files = %+v", files)
	}

	// No effective span: a one-frame clip with a pop renders as without it.
	short := base(func(c *Clip) { c.Duration = 1 })
	animated := short
	animated.AnimIn = &ClipAnim{Preset: "pop", Duration: 10}
	if got, want := chain(3, animated).filter("vs_clip_0_3"), chain(3, short).filter("vs_clip_0_3"); got != want {
		t.Fatalf("a pop without an effective span changed the chain:\n got %s\nwant %s", got, want)
	}
}

// TestClipChainRevealSnapshots pins the wipe's reveal stage between mask and orient (index §4.10, §4.24):
//   - no stage without an effective wipe span: no animation, another preset, or a one-frame clip;
//   - static box: the whole chain, the reveal row's expression and the enable of both spans;
//   - keyed geometry: the row spans the pad canvas, and its box width is the scale's expression in T;
//   - a mask: the mask merge first, then the reveal; opacity keyframes read command group 2 behind the reveal.
//
// The stage never holds sendcmd or gblur, and nothing commands the blend.
func TestClipChainRevealSnapshots(t *testing.T) {
	pal := Rate{Num: 25, Den: 1}
	chain := func(index int, clip Clip) clipChain {
		return clipChain{track: 0, index: index, item: layerClip{clip: clip, whole: clip, at: clip.Start, input: index, pictureWidth: 1920, pictureHeight: 1080, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: pal}
	}
	head := func(input, duration int) string {
		return fmt.Sprintf("[%d:V:0]setpts='if(eq(N,0),min(PTS,floor(0.000000000/TB)),PTS)',fps=25,setpts='max(PTS-0,-%d)',tpad=stop_mode=clone:stop=%d,trim=start_pts=0:end_pts=%d,setpts=PTS-STARTPTS,", input, duration, duration*2, duration)
	}
	tail := func(index, duration int, overlay string) string {
		return fmt.Sprintf("[vs_layer_0_%[1]d];color=c=black@0:s=1280x720:r=25:d=%[2]s,trim=end_frame=%[3]d,setpts=PTS-STARTPTS,format=rgba[vs_clip_bg_0_%[1]d];[vs_clip_bg_0_%[1]d][vs_layer_0_%[1]d]%[4]s[vs_clip_0_%[1]d]", index, pal.SecondsArg(duration), duration, overlay)
	}
	// reveal is the stage text of index §4.24 for chain c.
	reveal := func(c clipChain) string {
		plan := maskPlanOf(c)
		merge := "blend=c0_mode=normal:c1_mode=normal:c2_mode=normal:c3_mode=multiply"
		if animRevealGated {
			merge += ":enable='" + animSpanExpr(c.item.whole, c.item.at, c.rate, "wipe").String() + "'"
		}
		return fmt.Sprintf("format=gbrap[vs_rv_0_%[1]d_in];nullsrc=s=%[2]dx1:r=25:d=%[3]s,trim=end_frame=%[4]d,setpts=PTS-STARTPTS,format=gbrap,geq=r='0':g='0':b='0':a='%[5]s',scale=w=%[2]d:h=%[6]d:flags=neighbor[vs_rv_0_%[1]d_m];[vs_rv_0_%[1]d_in][vs_rv_0_%[1]d_m]%[7]s",
			c.index, plan.canvasWidth, pal.SecondsArg(c.item.clip.Duration), c.item.clip.Duration, revealExpr(c, plan).String(), plan.canvasHeight, merge)
	}
	containFit := "scale=w=640:h=360:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=w=640:h=360:x=(ow-iw)/2:y=(oh-ih)/2:color=black@0"
	staticOverlay := "overlay=x=320:y=180:format=auto:shortest=1"
	clip := func(edit func(*Clip)) Clip {
		c := Clip{ID: "w", AssetID: "a", Duration: 20, X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5, Opacity: 1, Volume: 1, Fit: FitContain,
			AnimIn: &ClipAnim{Preset: "wipe", Duration: 6}, AnimOut: &ClipAnim{Preset: "wipe", Duration: 5}}
		edit(&c)
		return c
	}

	for name, c := range map[string]Clip{
		"no animation":     clip(func(c *Clip) { c.AnimIn, c.AnimOut = nil, nil }),
		"another preset":   clip(func(c *Clip) { c.AnimIn, c.AnimOut = &ClipAnim{Preset: "fade", Duration: 6}, nil }),
		"a one-frame clip": clip(func(c *Clip) { c.Duration = 1 }),
	} {
		if got := chain(0, c).animReveal(); !reflect.DeepEqual(got, chainStage{}) {
			t.Fatalf("%s: reveal stage %+v, want none", name, got)
		}
	}

	// Static box: 640x360 at (320, 180) of the 1280x720 canvas; in span [0, 6), out span [15, 20).
	static := chain(0, clip(func(*Clip) {}))
	text := revealExpr(static, maskPlanOf(static)).String()
	parts := []string{
		"(if(lt(X,0.5),(st(5,640);(st(6,(0+round(((max(T,0)*25)/1))));(st(7,((640-ld(5))/2));if(lt(ld(6),6),",
		"(st(8,ld(7));st(9,(ld(7)+round((((ld(6)-0)/6)*ld(5))))))",
		"if(gte(ld(6),15),(st(9,(ld(7)+ld(5)));st(8,(ld(9)-round((((19-ld(6))/5)*ld(5)))))),(st(8,0);st(9,640))",
	}
	if !strings.HasPrefix(text, parts[0]) || !strings.Contains(text, parts[1]) || !strings.Contains(text, parts[2]) ||
		!strings.HasSuffix(text, ",0);(255*(gte(X,ld(8))*lt(X,ld(9)))))") {
		t.Fatalf("static reveal expression:\n%s\nwant the frame values once per row (%q, %q, %q), then 255 on [ld(8), ld(9))", text, parts[0], parts[1], parts[2])
	}
	if got, want := animSpanExpr(static.item.whole, 0, pal, "wipe").String(), "(st(0,(0+round(((max(t,0)*25)/1))));max((gte(ld(0),0)*lt(ld(0),6)),(gte(ld(0),15)*lt(ld(0),20))))"; got != want {
		t.Fatalf("wipe enable:\n got %s\nwant %s", got, want)
	}
	if stage := static.animReveal(); !stage.twoInput || stage.tracks != nil || strings.Contains(stage.filter, "sendcmd") || strings.Contains(stage.filter, "gblur") || strings.Contains(stage.filter, "blend@") {
		t.Fatalf("reveal stage %+v", stage)
	}
	if got, want := static.filter("vs_clip_0_0"), head(0, 20)+containFit+","+reveal(static)+",format=rgba"+tail(0, 20, staticOverlay); got != want {
		t.Fatalf("static reveal:\n got %s\nwant %s", got, want)
	}
	if files := static.commandFiles(); len(files) != 0 {
		t.Fatalf("a wipe alone needs no commands: %+v", files)
	}

	// Keyed geometry: the row spans the pad canvas with the box centred, its width the box expression in T.
	box := chain(1, clip(func(c *Clip) { c.Keyframes = map[string][]Keyframe{"width": linearKeys(0.3, 0.5, 19)} }))
	g := box.geometry()
	plan := maskPlanOf(box)
	if plan.canvasWidth != g.padWidth || plan.canvasHeight != g.padHeight {
		t.Fatalf("keyed reveal canvas %dx%d, pad %dx%d", plan.canvasWidth, plan.canvasHeight, g.padWidth, g.padHeight)
	}
	if text := revealExpr(box, plan).String(); !strings.HasPrefix(text, "(if(lt(X,0.5),(st(5,"+renameExprVar(g.box.w, "t", "T").String()+");") {
		t.Fatalf("the keyed reveal must read the box width in T:\n%s", text)
	}
	if got, want := box.filter("vs_clip_0_1"), head(1, 20)+box.fit()+","+reveal(box)+","+box.orient()+box.place("vs_clip_0_1"); got != want {
		t.Fatalf("keyed reveal:\n got %s\nwant %s", got, want)
	}

	// A mask, then the reveal; opacity keyframes read command group 2 behind the reveal.
	masked := chain(2, clip(func(c *Clip) {
		c.Mask = &Mask{Shape: MaskEllipse, X: 0.2, Y: 0.2, W: 0.6, H: 0.6, Feather: 0.05}
		c.Keyframes = map[string][]Keyframe{"opacity": linearKeys(0.2, 1, 19)}
	}))
	built := masked.build("vs_clip_0_2")
	if want := head(2, 20) + containFit + "," + masked.mask().filter + "," + reveal(masked) + ",sendcmd=f=vs_cmd_0_2_2.txt,format=rgba," + masked.opacity() + tail(2, 20, staticOverlay); built.filter != want {
		t.Fatalf("mask then reveal:\n got %s\nwant %s", built.filter, want)
	}
	if len(built.commands) != 1 || built.commands[0].name != "vs_cmd_0_2_2.txt" || built.commands[0].tracks[0].Target != "colorchannelmixer@vs_opacity_0_2" {
		t.Fatalf("mask then reveal: command files %+v, want the opacity in group 2", built.commands)
	}
}
