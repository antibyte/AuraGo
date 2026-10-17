package videostudio

import (
	"os"
	"path/filepath"
	"testing"
)

// probeVideo reports the clockwise degrees FFmpeg's autorotation turns a video by:
//   - the negated display-matrix rotation;
//   - else the legacy rotate tag.
//
// The two fixtures are real ffprobe captures (plan 2c Task 6 Step 2), both of a 1920x1080 stream turned by a display
// matrix of "rotation": 90 (90° counter-clockwise, which autorotation applies as 270° clockwise; decoded 1080x1920):
//   - FFmpeg 5.1.9: the mov muxer wrote -metadata:s:v:0 rotate=90 as that matrix, and ffprobe 5.1 prints no rotate tag;
//   - FFmpeg 8.1.1: -display_rotation 90 on a stream copy.
//
// The legacy rotate tag alone is covered by the synthetic streams below. The asset keeps the coded size; rotatedSize
// gives the decoded one.
func TestProbeVideoRotation(t *testing.T) {
	for _, c := range []struct {
		file     string
		rotation int
	}{
		{"ffprobe-rotation-5.1.json", 270},
		{"ffprobe-rotation-8.json", 270},
	} {
		output, err := os.ReadFile(filepath.Join("testdata", c.file))
		if err != nil {
			t.Fatal(err)
		}
		asset, rotation, err := assetFromFFprobe(output, "phone.mov", 1234)
		if err != nil || asset.Kind != AssetVideo || asset.Width != 1920 || asset.Height != 1080 || asset.Name != "phone.mov" || asset.SizeBytes != 1234 || asset.DurationFrames != 60 || rotation != c.rotation {
			t.Fatalf("%s: asset %+v, rotation %d, %v; want rotation %d", c.file, asset, rotation, err, c.rotation)
		}
		if width, height := rotatedSize(asset.Width, asset.Height, rotation); width != 1080 || height != 1920 {
			t.Fatalf("%s: decoded size %dx%d, want 1080x1920", c.file, width, height)
		}
	}
	stream := func(extra string) []byte {
		return []byte(`{"streams":[{"codec_type":"video","codec_name":"h264","width":64,"height":36,"duration":"1.000000","avg_frame_rate":"25/1"` + extra +
			`}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"1.000000"}}`)
	}
	for _, c := range []struct {
		name, extra string
		rotation    int
	}{
		{"no rotation", "", 0},
		{"rotate tag only", `,"tags":{"rotate":"270"}`, 270},
		{"upside down", `,"side_data_list":[{"side_data_type":"Display Matrix","rotation":180}]`, 180},
		{"counter-clockwise quarter turn", `,"side_data_list":[{"side_data_type":"Display Matrix","rotation":90}]`, 270},
		{"side data wins over the tag", `,"tags":{"rotate":"0"},"side_data_list":[{"side_data_type":"Display Matrix","rotation":-90}]`, 90},
		{"fractional degrees", `,"side_data_list":[{"side_data_type":"Display Matrix","rotation":-89.6}]`, 90},
		{"rotation as a string", `,"side_data_list":[{"side_data_type":"Display Matrix","rotation":"-90"}]`, 90},
		{"other side data", `,"side_data_list":[{"side_data_type":"Stereo 3D","type":"2D"}]`, 0},
	} {
		asset, rotation, err := assetFromFFprobe(stream(c.extra), "clip.mp4", 1)
		if err != nil || asset.Width != 64 || asset.Height != 36 || rotation != c.rotation {
			t.Errorf("%s: asset %dx%d, rotation %d (%v), want %d", c.name, asset.Width, asset.Height, rotation, err, c.rotation)
		}
	}
	if width, height := rotatedSize(64, 36, 180); width != 64 || height != 36 {
		t.Fatalf("a half turn changed the size: %dx%d", width, height)
	}
}
