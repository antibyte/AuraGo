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
