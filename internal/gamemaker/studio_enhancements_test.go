package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddLibraryInstallsVendorFilesWithoutOverwritingSource(t *testing.T) {
	s := newTestService(t)
	s.opts.MaxProjectBytes = 100 << 20
	s.opts.MaxFilesPerProject = 250
	s.opts.JobTimeout = time.Minute
	project := createTestProject(t, s, "2d")
	const customHTML = "<!doctype html><html><body><canvas id=\"c\"></canvas><script type=\"module\" src=\"src/main.ts\"></script></body></html>"
	const customTS = "import { playSfx } from '../vendor/zzfx.js';\nconsole.log(typeof playSfx);\n"

	s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "index.html", customHTML, ""); err != nil {
			return err
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", customTS, ""); err != nil {
			return err
		}
		for _, lib := range []string{"zzfx", "phaser", "three"} {
			res, err := s.AddLibrary(ctx, run.Job.ID, lib)
			if err != nil {
				return err
			}
			if res["library"] != lib {
				t.Fatalf("AddLibrary(%s) returned library=%v", lib, res["library"])
			}
		}
		if _, err := s.AddLibrary(ctx, run.Job.ID, "nonexistent"); err == nil {
			t.Fatal("AddLibrary accepted unknown library")
		}
		gotHTML, err := s.ReadJobFile(ctx, run.Job.ID, "index.html")
		if err != nil || gotHTML != customHTML {
			t.Fatalf("AddLibrary overwrote index.html: %v", err)
		}
		gotTS, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil || gotTS != customTS {
			t.Fatalf("AddLibrary overwrote src/main.ts: %v", err)
		}
		if _, err := s.ReadJobFile(ctx, run.Job.ID, "vendor/zzfx.js"); err != nil {
			t.Fatalf("vendor/zzfx.js not installed: %v", err)
		}
		if _, err := s.ReadJobFile(ctx, run.Job.ID, "vendor/phaser-4.2.1.min.js"); err != nil {
			t.Fatalf("vendor/phaser-4.2.1.min.js not installed: %v", err)
		}
		if _, err := s.ReadJobFile(ctx, run.Job.ID, "vendor/three-"+ThreeVersion+".module.min.js"); err != nil {
			t.Fatalf("three module not installed: %v", err)
		}
		if res := s.FinishJob(ctx, run.Job.ID); !res.OK {
			t.Fatalf("FinishJob failed: %+v", res.Diagnostics)
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "add libraries"})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("job status=%s error=%s", done.Status, done.Error)
	}
}

func TestUseTemplateCanvasAndGuided3D(t *testing.T) {
	s := newTestService(t)
	s.opts.MaxProjectBytes = 100 << 20
	s.opts.MaxFilesPerProject = 250
	s.opts.JobTimeout = time.Minute
	project := createTestProject(t, s, "2d")

	s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
		// Seed a TypeScript file first to verify canvas template clears stale TypeScript.
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", "const stale = 1;", ""); err != nil {
			return err
		}
		if err := s.UseTemplate(ctx, run.Job.ID, "canvas"); err != nil {
			return err
		}
		if _, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); err == nil {
			t.Fatal("canvas template retained stale src/main.ts")
		}
		gameJS, err := s.ReadJobFile(ctx, run.Job.ID, "src/game.js")
		if err != nil || !strings.Contains(gameJS, "window.__gameDebug") || !strings.Contains(gameJS, "../vendor/zzfx.js") {
			t.Fatalf("canvas src/game.js unexpected: err=%v", err)
		}
		if res := s.FinishJob(ctx, run.Job.ID); !res.OK {
			t.Fatalf("canvas template failed build/finish: %+v", res.Diagnostics)
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "canvas starter"})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("canvas job status=%s error=%s", done.Status, done.Error)
	}

	// Export must include vendor/zzfx.js even without src/main.ts.
	var exported bytes.Buffer
	if _, err := s.WriteExport(context.Background(), project.ID, &exported); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(exported.Bytes()), int64(exported.Len()))
	if err != nil {
		t.Fatal(err)
	}
	hasZzfx, hasGameJS := false, false
	for _, f := range zr.File {
		if f.Name == "vendor/zzfx.js" {
			hasZzfx = true
		}
		if f.Name == "src/game.js" {
			hasGameJS = true
		}
	}
	if !hasZzfx || !hasGameJS {
		t.Fatalf("canvas export missing files: zzfx=%v game.js=%v", hasZzfx, hasGameJS)
	}

	// Guided 3D template should work without failing when models are staged or skipped.
	p3d := createTestProject(t, s, "3d")
	s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
		if err := s.UseTemplate(ctx, run.Job.ID, "exploration"); err != nil {
			return err
		}
		if res := s.FinishJob(ctx, run.Job.ID); !res.OK {
			t.Fatalf("guided 3d build failed: %+v", res.Diagnostics)
		}
		return nil
	}))
	job3d, err := s.StartJob(context.Background(), p3d.ID, StartJobRequest{Prompt: "guided 3d"})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job3d.ID); done.Status != "ready" {
		t.Fatalf("guided 3d job status=%s error=%s", done.Status, done.Error)
	}
}

func TestCraftRecipeTopicsAndFrameworkReference(t *testing.T) {
	topics := CraftRecipeTopics()
	if len(topics) < 8 {
		t.Fatalf("expected at least 8 craft recipe topics, got %v", topics)
	}
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
		ctxInfo := s.FrameworkReference(ctx, run.Job.ID, "")
		if ctxInfo["available_templates"] == nil || ctxInfo["available_libraries"] == nil || ctxInfo["available_topics"] == nil {
			t.Fatalf("omitted framework reference missing toolbox catalogs: %+v", ctxInfo)
		}
		for _, topic := range topics {
			ref := s.FrameworkReference(ctx, run.Job.ID, "", topic)
			recipe, ok := ref["recipe"].(map[string]any)
			if !ok || strings.TrimSpace(recipe["snippet"].(string)) == "" {
				t.Fatalf("topic %q returned invalid recipe: %+v", topic, ref)
			}
		}
		if res := s.FinishJob(ctx, run.Job.ID); !res.OK {
			return nil
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "inspect recipes"})
	if err != nil {
		t.Fatal(err)
	}
	_ = waitJob(t, s, job.ID)
}

func TestSynthesizeSFXAndContactSheet(t *testing.T) {
	for _, prompt := range []string{"retro coin pickup", "laser blast", "big explosion", "hero jump", "level clear victory"} {
		wav, preset, params := SynthesizeSFX(prompt, 0.25, 42)
		if len(wav) <= 44 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
			t.Fatalf("invalid WAV for %q: len=%d", prompt, len(wav))
		}
		if preset == "" || len(params) < 10 {
			t.Fatalf("missing preset/zzfx params for %q: preset=%q len=%d", prompt, preset, len(params))
		}
	}

	// Deterministic output with identical prompt & seed.
	a, _, _ := SynthesizeSFX("coin chime", 0.2, 7)
	b, _, _ := SynthesizeSFX("coin chime", 0.2, 7)
	if !bytes.Equal(a, b) {
		t.Fatal("SynthesizeSFX with fixed seed was not deterministic")
	}

	// Contact sheet stitches multiple JPEG frames into one valid JPEG under 2 MiB.
	var frames [][]byte
	colors := []color.RGBA{
		{R: 220, G: 60, B: 60, A: 255},
		{R: 60, G: 220, B: 60, A: 255},
		{R: 60, G: 120, B: 230, A: 255},
		{R: 230, G: 200, B: 60, A: 255},
	}
	for _, c := range colors {
		img := image.NewRGBA(image.Rect(0, 0, 320, 240))
		for y := 0; y < 240; y++ {
			for x := 0; x < 320; x++ {
				img.Set(x, y, c)
			}
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, buf.Bytes())
	}
	sheet, err := stitchContactSheet(frames)
	if err != nil {
		t.Fatalf("stitchContactSheet failed: %v", err)
	}
	if len(sheet) == 0 || len(sheet) > 2<<20 {
		t.Fatalf("contact sheet size out of bounds: %d", len(sheet))
	}
	decoded, err := jpeg.Decode(bytes.NewReader(sheet))
	if err != nil {
		t.Fatalf("contact sheet is not a valid JPEG: %v", err)
	}
	if decoded.Bounds().Dx() <= 320 || decoded.Bounds().Dy() <= 240 {
		t.Fatalf("contact sheet was not laid out as a grid: %v", decoded.Bounds())
	}

	// Viewport parsing checks.
	if w, h, mobile, spec, err := parsePreviewViewport("portrait"); err != nil || !spec || !mobile || w != 430 || h != 932 {
		t.Fatalf("portrait viewport parse failed: %d×%d mobile=%v spec=%v err=%v", w, h, mobile, spec, err)
	}
	if w, h, mobile, spec, err := parsePreviewViewport("1280x720"); err != nil || !spec || mobile || w != 1280 || h != 720 {
		t.Fatalf("1280x720 viewport parse failed: %d×%d mobile=%v spec=%v err=%v", w, h, mobile, spec, err)
	}
	if _, _, _, _, err := parsePreviewViewport("9999x10"); err == nil {
		t.Fatal("invalid viewport dimensions accepted")
	}
}

func TestTemplateNamesStayAlignedAcrossSchemaManualAndSkills(t *testing.T) {
	manualPath := filepath.Join("..", "..", "prompts", "tools_manuals", "game_maker.md")
	manualBytes, err := os.ReadFile(manualPath)
	if err != nil {
		t.Fatal(err)
	}
	manual := string(manualBytes)
	for _, name := range StudioTemplateNames() {
		if !strings.Contains(manual, name) {
			t.Errorf("prompts/tools_manuals/game_maker.md does not mention template %q", name)
		}
	}
	for _, topic := range CraftRecipeTopics() {
		if !strings.Contains(manual, topic) {
			t.Errorf("prompts/tools_manuals/game_maker.md does not mention recipe topic %q", topic)
		}
	}
}
