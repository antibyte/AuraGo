package goldenframe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/videostudio"
)

// caseGroups are the allowed name prefixes of a golden case (phase 2 index §4.12).
var caseGroups = map[string]bool{"geometry": true, "opacity": true, "fit": true, "transition": true, "keyframe": true, "ramp": true,
	"color": true, "lut": true, "crop": true, "flip": true, "blur": true, "sharpen": true, "vignette": true, "key": true,
	"blend": true, "mask": true, "anim": true}

// fakeMedia mirrors the assets MakeMedia produces (durations at 30 fps, as Probe reports them) without files.
func fakeMedia() Media {
	video := func(id string, width, height int, seconds int64) videostudio.Asset {
		return videostudio.Asset{ID: id, Name: id + ".mp4", Kind: videostudio.AssetVideo, DurationFrames: int(seconds) * 30,
			DurationUs: seconds * 1_000_000, VideoDurationUs: seconds * 1_000_000, Width: width, Height: height, SourceFPS: "25/1", Codec: "h264"}
	}
	still := func(id string) videostudio.Asset {
		return videostudio.Asset{ID: id, Name: id + ".png", Kind: videostudio.AssetImage, Width: 640, Height: 360, Codec: "png"}
	}
	return Media{Assets: map[string]videostudio.Asset{
		MediaGradient: video(MediaGradient, 1280, 720, 3), MediaPortrait: video(MediaPortrait, 720, 1280, 2),
		MediaLogo: still(MediaLogo), MediaRed: still(MediaRed), MediaBlue: still(MediaBlue),
		MediaTicker: video(MediaTicker, 320, 180, 8),
		MediaChart:  still(MediaChart), MediaDetail: still(MediaDetail),
		MediaGreen: still(MediaGreen), MediaLayer: still(MediaLayer),
		MediaCube: {ID: MediaCube, Name: "cube.cube", Kind: videostudio.AssetLUT, LUTSize: 17}}}
}

func TestProjectBuilderAndRegisteredCasesValidate(t *testing.T) {
	m := fakeMedia()
	project := m.Project("Builder", VideoTrack("video", NewClip("a", MediaGradient, 0, 75)), OverlayTrack("logo", NewClip("b", MediaLogo, 10, 40)))
	if err := videostudio.Validate(project); err != nil {
		t.Fatalf("builder project invalid: %v", err)
	}
	if project.Width != ProjectWidth || project.Height != ProjectHeight || project.FPS != ProjectFPS || len(project.Assets) != len(m.Assets) || project.Assets[0].ID != MediaBlue {
		t.Fatalf("builder project = %+v", project)
	}
	// Assets are sorted by id (blue, chart, cube, detail, gradient, …), so the gradient is looked up.
	var gradient videostudio.Asset
	for _, asset := range project.Assets {
		if asset.ID == MediaGradient {
			gradient = asset
		}
	}
	if gradient.ID != MediaGradient || gradient.DurationFrames != 75 {
		t.Fatalf("assets are not expressed at %d fps: %+v", ProjectFPS, gradient)
	}
	seen := map[string]bool{}
	for _, c := range Cases(m) {
		group, _, found := strings.Cut(c.Name, "_")
		if !found || !caseGroups[group] || seen[c.Name] {
			t.Errorf("case name %q must be a unique <group>_<detail> with a known group", c.Name)
		}
		seen[c.Name] = true
		if err := videostudio.Validate(c.Project); err != nil {
			t.Errorf("%s: invalid project: %v", c.Name, err)
		}
		if c.Frame < 0 || c.Frame >= videostudio.EnabledDuration(c.Project) {
			t.Errorf("%s: frame %d is outside the timeline", c.Name, c.Frame)
		}
		if c.Threshold <= 0 || c.Threshold > 1 {
			t.Errorf("%s: threshold %v", c.Name, c.Threshold)
		}
		if len(c.Painters) == 0 {
			t.Errorf("%s: no painters", c.Name)
		}
		for _, painter := range c.Painters {
			if painter != PainterGL && painter != Painter2D {
				t.Errorf("%s: unknown painter %q", c.Name, painter)
			}
		}
	}
}

func TestMakeMediaProbesTaggedAssets(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m, err := MakeMedia(ctx, ffmpeg, filepath.Join(t.TempDir(), "media"))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]struct {
		kind          videostudio.AssetKind
		width, height int
		fps           string
	}{
		MediaGradient: {videostudio.AssetVideo, 1280, 720, "25/1"},
		MediaPortrait: {videostudio.AssetVideo, 720, 1280, "25/1"},
		MediaLogo:     {videostudio.AssetImage, 640, 360, ""},
		MediaRed:      {videostudio.AssetImage, 640, 360, ""},
		MediaBlue:     {videostudio.AssetImage, 640, 360, ""},
		MediaTicker:   {videostudio.AssetVideo, 320, 180, "25/1"},
		MediaChart:    {videostudio.AssetImage, 640, 360, ""},
		MediaDetail:   {videostudio.AssetImage, 640, 360, ""},
		MediaCube:     {videostudio.AssetLUT, 0, 0, ""},
		MediaGreen:    {videostudio.AssetImage, 640, 360, ""},
		MediaLayer:    {videostudio.AssetImage, 640, 360, ""},
	} {
		asset := m.Assets[name]
		if asset.ID != name || asset.Kind != want.kind || asset.Width != want.width || asset.Height != want.height || asset.SourceFPS != want.fps {
			t.Errorf("%s probed as %+v", name, asset)
		}
		if info, err := os.Stat(m.Paths[name]); err != nil || !info.Mode().IsRegular() || filepath.Base(m.Paths[name]) != m.Files[name] {
			t.Errorf("%s: path %q, file %q: %v", name, m.Paths[name], m.Files[name], err)
		}
	}
	// cube.cube is inspected like the server's probe job does (InspectLUTFile), not probed by FFmpeg.
	if cube := m.Assets[MediaCube]; cube.LUTSize != 17 || cube.SizeBytes <= 0 || cube.DurationFrames != 0 {
		t.Errorf("cube.cube inspected as %+v, want kind lut, lut_size 17 and its size", cube)
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	for name, frames := range map[string]int{MediaGradient: 75, MediaPortrait: 50, MediaTicker: 200} {
		tags, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0",
			"-show_entries", "stream=color_space,color_primaries,color_transfer,color_range", "-of", "default=noprint_wrappers=1", m.Paths[name]).Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"color_space=bt709", "color_primaries=bt709", "color_transfer=bt709", "color_range=tv"} {
			if !strings.Contains(string(tags), want) {
				t.Errorf("%s lacks %s: %s", name, want, tags)
			}
		}
		types, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "frame=pict_type", "-of", "csv=p=0", m.Paths[name]).Output()
		if err != nil {
			t.Fatal(err)
		}
		var count int
		for _, field := range strings.Fields(string(types)) {
			kind := strings.Trim(field, ",")
			if kind == "" {
				continue
			}
			count++
			if kind != "I" {
				t.Fatalf("%s frame %d is %q; golden media must be all-intra", name, count-1, kind)
			}
		}
		if count != frames {
			t.Errorf("%s has %d frames, want %d", name, count, frames)
		}
	}
}
