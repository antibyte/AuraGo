package render

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// testBinary finds ffmpeg or ffprobe: AURAGO_TEST_FFMPEG_DIR, PATH, then the WinGet package folder.
func testBinary(t *testing.T, name string) string {
	t.Helper()
	exe := name
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if dir := os.Getenv("AURAGO_TEST_FFMPEG_DIR"); dir != "" {
		if p := filepath.Join(dir, exe); fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			matches, _ := filepath.Glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", "*FFmpeg*", "*", "bin", exe))
			if len(matches) > 0 {
				return matches[0]
			}
		}
	}
	t.Skipf("%s is not available (set AURAGO_TEST_FFMPEG_DIR)", name)
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func solidPNG(t *testing.T, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type probeDoc struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		PixFmt       string `json:"pix_fmt"`
		AvgFrameRate string `json:"avg_frame_rate"`
		NbFrames     string `json:"nb_frames"`
		ColorSpace   string `json:"color_space"`
		ColorRange   string `json:"color_range"`
		SampleRate   string `json:"sample_rate"`
		Duration     string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func ffprobeFile(t *testing.T, ffprobe, file string) probeDoc {
	t.Helper()
	out, err := exec.Command(ffprobe, "-v", "error", "-show_streams", "-show_format", "-of", "json", file).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var doc probeDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
