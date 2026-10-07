package videostudio

import (
	"context"
	"encoding/binary"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFFmpegProbePreviewAndMultitrackRender(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	status := CheckRuntime(context.Background(), ffmpeg)
	if !status.Ready {
		t.Fatalf("CheckRuntime() = %+v", status)
	}
	if status.FFprobePath == "" {
		t.Fatal("CheckRuntime() returned no FFprobe path")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	redPath := filepath.Join(root, "red.mp4")
	bluePath := filepath.Join(root, "blue.mp4")
	audioPath := filepath.Join(root, "tone.wav")
	coverPath := filepath.Join(root, "green.png")
	coverAudioPath := filepath.Join(root, "cover.mp3")
	makeVideoFixture(t, ctx, ffmpeg, redPath, "red")
	makeVideoFixture(t, ctx, ffmpeg, bluePath, "blue")
	makeAudioFixture(t, ctx, ffmpeg, audioPath)
	makeImageFixture(t, ctx, ffmpeg, coverPath)
	makeAttachedCoverAudioFixture(t, ctx, ffmpeg, audioPath, coverPath, coverAudioPath)

	red, err := Probe(ctx, ffmpeg, redPath)
	if err != nil {
		t.Fatalf("Probe(video) error = %v", err)
	}
	if red.Kind != AssetVideo || red.Width != 64 || red.Height != 32 || red.DurationFrames != 30 {
		t.Fatalf("Probe(video) = %+v", red)
	}
	blue, err := Probe(ctx, ffmpeg, bluePath)
	if err != nil {
		t.Fatalf("Probe(second video) error = %v", err)
	}
	if blue.Kind != AssetVideo || blue.DurationFrames != 30 {
		t.Fatalf("Probe(second video) = %+v", blue)
	}
	audio, err := Probe(ctx, ffmpeg, audioPath)
	if err != nil {
		t.Fatalf("Probe(audio) error = %v", err)
	}
	if audio.Kind != AssetAudio || !audio.HasAudio || audio.DurationFrames < 59 {
		t.Fatalf("Probe(audio) = %+v", audio)
	}
	withCover, err := Probe(ctx, ffmpeg, coverAudioPath)
	if err != nil {
		t.Fatalf("Probe(audio with attached cover) error = %v", err)
	}
	if withCover.Kind != AssetAudio || !withCover.HasAudio || withCover.Width != 0 || withCover.Height != 0 {
		t.Fatalf("Probe(audio with attached cover) = %+v", withCover)
	}

	previewPath := filepath.Join(root, "preview.mp4")
	if err := PreparePreview(ctx, ffmpeg, redPath, previewPath); err != nil {
		t.Fatalf("PreparePreview() error = %v", err)
	}
	preview, err := Probe(ctx, ffmpeg, previewPath)
	if err != nil || preview.Kind != AssetVideo {
		t.Fatalf("Probe(preview) = %+v, error = %v", preview, err)
	}

	red.ID, blue.ID, audio.ID = "red", "blue", "tone"
	cover, err := Probe(ctx, ffmpeg, coverPath)
	if err != nil {
		t.Fatalf("Probe(image) error = %v", err)
	}
	cover.ID = "cover"
	project := Project{
		Version: 1, Name: "Render test", Width: 720, Height: 720, FPS: 30,
		Assets: []Asset{red, blue, audio, cover},
		Tracks: []Track{
			{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{
				{ID: "red-clip", AssetID: "red", Start: 0, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionDissolve, Duration: 10}},
				{ID: "blue-clip", AssetID: "blue", Start: 20, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionBlack, Duration: 10}},
				{ID: "red-wipe", AssetID: "red", Start: 40, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionWipeLeft, Duration: 10}},
				{ID: "blue-wipe", AssetID: "blue", Start: 60, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Transition: &Transition{Type: TransitionWipeRight, Duration: 10}},
				{ID: "red-end", AssetID: "red", Start: 80, Duration: 30, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain},
			}},
			{ID: "audio", Name: "Music", Kind: TrackAudio, Clips: []Clip{
				{ID: "audio-clip", AssetID: "tone", Start: 0, Duration: 50, Volume: 0.7, FadeIn: 3, FadeOut: 5},
			}},
			{ID: "overlay", Name: "Overlay", Kind: TrackOverlay, Clips: []Clip{
				{ID: "overlay-clip", AssetID: "cover", Start: 0, Duration: 110, X: 0.8, Y: 0.1, Width: 0.1, Height: 0.1, Opacity: 1, Volume: 1, Fit: FitContain},
			}},
		},
	}
	if err := Validate(project); err != nil {
		t.Fatalf("Validate(render project) error = %v", err)
	}
	output := filepath.Join(root, "render.mp4")
	var lastProgress float64
	if err := Render(ctx, ffmpeg, project, map[string]string{"red": redPath, "blue": bluePath, "tone": audioPath, "cover": coverPath}, output, func(value float64) {
		if value < lastProgress {
			t.Errorf("progress moved backwards from %.3f to %.3f", lastProgress, value)
		}
		lastProgress = value
	}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if lastProgress != 1 {
		t.Fatalf("final progress = %.3f, want 1", lastProgress)
	}
	rendered, err := Probe(ctx, ffmpeg, output)
	if err != nil {
		t.Fatalf("Probe(rendered) error = %v", err)
	}
	if rendered.Kind != AssetVideo || !rendered.HasAudio || rendered.Width != 720 || rendered.Height != 720 || rendered.DurationFrames < 109 || rendered.DurationFrames > 112 {
		t.Fatalf("Probe(rendered) = %+v", rendered)
	}

	before := decodedPixel(t, ctx, ffmpeg, output, "0.200000", 360, 360)
	transition := decodedPixel(t, ctx, ffmpeg, output, "0.833333", 360, 360)
	after := decodedPixel(t, ctx, ffmpeg, output, "1.000000", 360, 360)
	throughBlack := decodedPixel(t, ctx, ffmpeg, output, "1.500000", 360, 360)
	if before[0] < before[2]*3 {
		t.Fatalf("pre-transition frame is not red enough: %v", before)
	}
	if after[2] < after[0]*3 {
		t.Fatalf("post-transition frame is not blue enough: %v", after)
	}
	if int(throughBlack[0])+int(throughBlack[1])+int(throughBlack[2]) > 90 {
		t.Fatalf("black transition midpoint is not near black: %v", throughBlack)
	}
	if transition[0] < 30 || transition[2] < 30 {
		t.Fatalf("dissolve frame did not contain both clips: %v", transition)
	}
	letterbox := decodedPixel(t, ctx, ffmpeg, output, "0.200000", 360, 30)
	if letterbox[0] > 15 || letterbox[1] > 15 || letterbox[2] > 15 {
		t.Fatalf("contain fit did not preserve a black letterbox: %v", letterbox)
	}
	overlayPixel := decodedPixel(t, ctx, ffmpeg, output, "0.200000", 610, 110)
	if overlayPixel[1] < overlayPixel[0]*2 || overlayPixel[1] < overlayPixel[2]*2 {
		t.Fatalf("overlay track did not render over the base video: %v", overlayPixel)
	}
	wipeLeft := decodedPixel(t, ctx, ffmpeg, output, "2.166667", 10, 360)
	wipeLeftRight := decodedPixel(t, ctx, ffmpeg, output, "2.166667", 710, 360)
	if wipeLeft[2] < wipeLeft[0]*2 || wipeLeftRight[0] < wipeLeftRight[2]*2 {
		t.Fatalf("wipeleft did not reveal incoming video from the left: left=%v right=%v", wipeLeft, wipeLeftRight)
	}
	wipeRightLeft := decodedPixel(t, ctx, ffmpeg, output, "2.833333", 10, 360)
	wipeRight := decodedPixel(t, ctx, ffmpeg, output, "2.833333", 710, 360)
	if wipeRight[0] < wipeRight[2]*2 || wipeRightLeft[2] < wipeRightLeft[0]*2 {
		t.Fatalf("wiperight did not reveal incoming video from the right: left=%v right=%v", wipeRightLeft, wipeRight)
	}
	if got := decodedAudioRMS(t, ctx, ffmpeg, output); got < 100 {
		t.Fatalf("rendered audio RMS = %.1f, want audible output", got)
	}

	if _, err := Probe(ctx, ffmpeg, "https://example.invalid/video.mp4"); err == nil {
		t.Fatal("Probe accepted a remote URL")
	}
}

func TestRenderCoverRotationAndOpacity(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	root := t.TempDir()
	sourcePath := filepath.Join(root, "split.mp4")
	makeSplitVideoFixture(t, ctx, ffmpeg, sourcePath)
	asset, err := Probe(ctx, ffmpeg, sourcePath)
	if err != nil {
		t.Fatalf("Probe(split source) error = %v", err)
	}
	asset.ID = "split"
	project := Project{
		Version: 1, Name: "Geometry test", Width: 720, Height: 720, FPS: 30,
		Assets: []Asset{asset},
		Tracks: []Track{{ID: "video", Name: "Video", Kind: TrackVideo, Clips: []Clip{{
			ID: "clip", AssetID: "split", Start: 0, Duration: 30, X: 0.2, Y: 0.2, Width: 0.2, Height: 0.2,
			Rotation: 45, Opacity: 0.5, Volume: 1, Fit: FitCover,
		}}}},
	}
	output := filepath.Join(root, "geometry.mp4")
	if err := Render(ctx, ffmpeg, project, map[string]string{"split": sourcePath}, output, nil); err != nil {
		t.Fatalf("Render(geometry) error = %v", err)
	}
	left := decodedPixel(t, ctx, ffmpeg, output, "0.500000", 153, 216)
	right := decodedPixel(t, ctx, ffmpeg, output, "0.500000", 279, 216)
	expanded := decodedPixel(t, ctx, ffmpeg, output, "0.500000", 140, 216)
	if left[0] < 70 || left[0] > 200 || left[2] > 45 {
		t.Fatalf("cover/opacity left half = %v, want half-opacity red", left)
	}
	if right[2] < 70 || right[2] > 200 || right[0] > 45 {
		t.Fatalf("cover/opacity right half = %v, want half-opacity blue", right)
	}
	if expanded[0] < 35 && expanded[2] < 35 {
		t.Fatalf("rotation did not expand the clip outside its original bounds: %v", expanded)
	}
}

func makeVideoFixture(t *testing.T, ctx context.Context, ffmpeg, output, color string) {
	t.Helper()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=" + color + ":s=64x32:r=30:d=1",
		"-an", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-f", "mp4", output,
	}
	if output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create %s fixture: %v: %s", color, err, output)
	}
}

func makeImageFixture(t *testing.T, ctx context.Context, ffmpeg, output string) {
	t.Helper()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=green:s=32x32:d=1", "-frames:v", "1", "-f", "image2", output,
	}
	if output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create image fixture: %v: %s", err, output)
	}
}

func makeSplitVideoFixture(t *testing.T, ctx context.Context, ffmpeg, output string) {
	t.Helper()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=red:s=64x32:r=30:d=1",
		"-vf", "drawbox=x=32:y=0:w=32:h=32:color=blue:t=fill",
		"-an", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-f", "mp4", output,
	}
	if output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create split video fixture: %v: %s", err, output)
	}
}

func makeAttachedCoverAudioFixture(t *testing.T, ctx context.Context, ffmpeg, audio, image, output string) {
	t.Helper()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-i", audio, "-i", image,
		"-map", "0:a:0", "-map", "1:v:0", "-c:a", "libmp3lame", "-c:v", "copy",
		"-id3v2_version", "3", "-disposition:v:0", "attached_pic", "-f", "mp3", output,
	}
	if output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create MP3 with attached cover fixture: %v: %s", err, output)
	}
}

func makeAudioFixture(t *testing.T, ctx context.Context, ffmpeg, output string) {
	t.Helper()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=2",
		"-c:a", "pcm_s16le", "-f", "wav", output,
	}
	if output, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("create audio fixture: %v: %s", err, output)
	}
}

func decodedPixel(t *testing.T, ctx context.Context, ffmpeg, input, timestamp string, x, y int) [3]byte {
	t.Helper()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-ss", timestamp, "-i", input,
		"-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1",
	}
	data, err := exec.CommandContext(ctx, ffmpeg, args...).Output()
	if err != nil {
		t.Fatalf("decode frame at %ss: %v", timestamp, err)
	}
	index := (y*720 + x) * 3
	if len(data) < index+3 {
		t.Fatalf("decoded frame has %d bytes, need %d", len(data), index+3)
	}
	return [3]byte{data[index], data[index+1], data[index+2]}
}

func decodedAudioRMS(t *testing.T, ctx context.Context, ffmpeg, input string) float64 {
	t.Helper()
	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error", "-i", input,
		"-map", "0:a:0", "-f", "s16le", "-ac", "2", "-ar", "48000", "pipe:1",
	}
	data, err := exec.CommandContext(ctx, ffmpeg, args...).Output()
	if err != nil {
		t.Fatalf("decode rendered audio: %v", err)
	}
	var sum float64
	count := len(data) / 2
	for i := 0; i < count; i++ {
		sample := float64(int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2])))
		sum += sample * sample
	}
	return math.Sqrt(sum / float64(count))
}
