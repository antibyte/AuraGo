package render

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/storymaker"
)

func TestRasterReferenceExportBrowser(t *testing.T) {
	fx := newRealFixture(t, browserSmoke(t))
	fx.settings.TimeoutSeconds = 900
	fx.settings.Transport = TransportPNG
	var browserPID, chromePeak, heapPeak, firstFrame, lastFrame, frameBytes atomic.Int64
	fx.deps.OpenPage = func(ctx context.Context, options sessionOptions) (browserPage, error) {
		page, err := openSession(ctx, options)
		if session, ok := page.(*session); ok && err == nil {
			browserPID.Store(int64(session.proc.PID()))
			t.Logf("render guard: %s", session.guard.Mode())
		}
		return page, err
	}
	if os.Getenv("AURAGO_STORYMAKER_REVIEW_DIR") != "" {
		fx.deps.FrameObserver = func(n int, _ string, data []byte) {
			now := time.Now().UnixNano()
			firstFrame.CompareAndSwap(0, now)
			lastFrame.Store(now)
			frameBytes.Add(int64(len(data)))
			if n%15 == 0 {
				chromePeak.Store(max(chromePeak.Load(), treeRSS(int(browserPID.Load()))))
				var memory runtime.MemStats
				runtime.ReadMemStats(&memory)
				heapPeak.Store(max(heapPeak.Load(), int64(memory.HeapInuse)))
			}
		}
	}
	if root := os.Getenv("AURAGO_STORYMAKER_REFERENCE_UI"); root != "" {
		fx.deps.UI = os.DirFS(root)
		var err error
		fx.deps.Catalog, err = storymaker.LoadCatalog(fx.deps.UI, "img/storymaker/catalog.json")
		if err != nil {
			t.Fatal(err)
		}
	}
	fx.deps.Library = func(ctx context.Context, slug string, reader io.Reader, size int64) (string, error) {
		target := "Videos/Storymaker/" + slug + ".mp4"
		return target, fx.pub.WriteStream(ctx, target, reader, size)
	}
	fx.rebuild(t)
	ctx := context.Background()
	story, err := os.ReadFile("../testdata/stories/sprite-reference.json")
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := storymaker.DecodeStory(story, fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&decoded)
	for _, issue := range storymaker.Validate(&decoded, fx.deps.Catalog, fx.settings.Limits) {
		if issue.Severity == storymaker.SeverityError {
			t.Errorf("reference fixture: %s: %s", issue.Path, issue.Message)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, story, ""); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	if _, result, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", source, ""); err != nil || len(result.Errors) != 0 {
		t.Fatalf("code scene: %v %+v", err, result)
	}
	fx.seedStoredNarration(t)
	started := time.Now()
	job := fx.renderWith(t, RenderOptions{SRT: true, VTT: true, Stems: true, Quality: "standard"})
	elapsed := time.Since(started)
	if job.Status != StatusSucceeded || job.Output == nil {
		t.Fatalf("render: %+v %+v", job, job.Error)
	}
	if job.FramesTotal != 900 || job.FramesDone != 900 {
		t.Fatalf("expected 30 seconds / 900 frames: %+v", job)
	}
	for _, name := range []string{job.Output.Path, job.Output.Library, job.Output.SRT, job.Output.VTT} {
		if data, ok := fx.pub.get(name); !ok || len(data) == 0 {
			t.Fatalf("missing export artifact %q", name)
		}
	}
	srt, _ := fx.pub.get(job.Output.SRT)
	if !bytes.Contains(srt, []byte("Unsere Geräte")) {
		t.Fatalf("missing German subtitle: %s", srt)
	}
	if job.Output.Stems == nil || job.Output.Stems.Narration == "" {
		t.Fatal("missing narration stem")
	}
	video, _ := fx.pub.get(job.Output.Path)
	file := filepath.Join(t.TempDir(), "reference.mp4")
	if err := os.WriteFile(file, video, 0o600); err != nil {
		t.Fatal(err)
	}
	probe := ffprobeFile(t, fx.settings.FFprobePath, file)
	videoOK, audioOK := false, false
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			videoOK = stream.Width == 1920 && stream.Height == 1080 && stream.NbFrames == "900" && stream.AvgFrameRate == "30/1"
		}
		if stream.CodecType == "audio" {
			audioOK = true
		}
	}
	if !videoOK || !audioOK {
		t.Fatalf("MP4 streams: %+v", probe.Streams)
	}
	if dir := os.Getenv("AURAGO_STORYMAKER_REVIEW_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range fx.pub.names() {
			if strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".srt") || strings.HasSuffix(name, ".vtt") {
				data, _ := fx.pub.get(name)
				if err := os.WriteFile(filepath.Join(dir, filepath.Base(name)), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
		data, _ := json.MarshalIndent(job, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "reference-job.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		metrics := map[string]any{"platform": runtime.GOOS, "catalog_schema": fx.deps.Catalog.Schema, "frames": job.FramesTotal, "job_seconds": elapsed.Seconds(), "render_fps": float64(job.FramesTotal-1) / (float64(lastFrame.Load()-firstFrame.Load()) / float64(time.Second)), "chrome_peak_rss_mib": chromePeak.Load() >> 20, "go_heap_peak_mib": heapPeak.Load() >> 20, "frame_bytes_avg": frameBytes.Load() / int64(job.FramesTotal)}
		data, err := json.MarshalIndent(metrics, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "reference-metrics.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("reference metrics: %s", data)
	}
}
