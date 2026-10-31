package server

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
	"aurago/internal/persondetect"
)

// liveRecorder wraps the real manager so the test can read the answers behind
// the consumers' coarse verdicts. Load and CacheGeneration stay promoted, which
// is what the hub's admission and identity code looks for.
type liveRecorder struct {
	*decisions.Manager
	mu       sync.Mutex
	calls    int
	last     decisions.Response
	lastErr  error
	duration time.Duration
}

func (r *liveRecorder) Evaluate(ctx context.Context, request decisions.Request) (decisions.Response, error) {
	started := time.Now()
	response, err := r.Manager.Evaluate(ctx, request)
	r.mu.Lock()
	r.calls++
	r.last, r.lastErr, r.duration = response, err, time.Since(started)
	r.mu.Unlock()
	return response, err
}

func (r *liveRecorder) snapshot() (int, decisions.Response, error, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls, r.last, r.lastErr, r.duration
}

func liveEmptyRoom(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 200, G: 196, B: 186, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 330, 640, 480), &image.Uniform{C: color.RGBA{R: 150, G: 138, B: 120, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 328, 640, 332), &image.Uniform{C: color.RGBA{R: 96, G: 88, B: 78, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func liveScreenshot(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	fill := func(r image.Rectangle, c color.RGBA) {
		draw.Draw(img, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
	fill(img.Bounds(), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	fill(image.Rect(0, 0, 800, 36), color.RGBA{R: 43, G: 87, B: 154, A: 255})
	fill(image.Rect(10, 12, 130, 24), color.RGBA{R: 235, G: 240, B: 250, A: 255})
	fill(image.Rect(760, 10, 790, 26), color.RGBA{R: 200, G: 60, B: 60, A: 255})
	for row := 0; row < 12; row++ {
		width := 300 + (row*53)%380
		fill(image.Rect(40, 70+row*28, 40+width, 78+row*28), color.RGBA{R: 40, G: 40, B: 44, A: 255})
	}
	fill(image.Rect(40, 450, 160, 486), color.RGBA{R: 43, G: 87, B: 154, A: 255})
	fill(image.Rect(180, 450, 300, 486), color.RGBA{R: 225, G: 225, B: 225, A: 255})
	fill(image.Rect(48, 464, 150, 472), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	fill(image.Rect(190, 464, 290, 472), color.RGBA{R: 60, G: 60, B: 60, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// livePersonPhoto is the repo's pedestrian reference photo (person detection self-test).
func livePersonPhoto(t *testing.T) ([]byte, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "persondetect", "selftest_reference.jpg"))
	if err != nil {
		return nil, false
	}
	return data, true
}

// TestDecisionConsumersLiveNative is opt-in like TestDecisionsLiveNative: point
// AURAGO_DECISIONS_LIVE_MODELS at the two verified model files and
// AURAGO_DECISIONS_LIVE_RUNTIME at the application root with the runtime bundle.
// Run it with TMPDIR on the volume that holds the models (they are hard-linked).
// It drives the consumers through the real hub and the real local model.
func TestDecisionConsumersLiveNative(t *testing.T) {
	models, runtimeRoot := os.Getenv("AURAGO_DECISIONS_LIVE_MODELS"), os.Getenv("AURAGO_DECISIONS_LIVE_RUNTIME")
	if models == "" || runtimeRoot == "" {
		t.Skip("requires explicitly selected local model and native runtime directories")
	}
	s, _ := decisionsServerFixture(t)
	dataDir := t.TempDir()
	modelDir := filepath.Join(dataDir, "decisions", "models")
	if err := os.MkdirAll(modelDir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := decisions.DefaultManifest()
	for _, artifact := range []decisions.Artifact{manifest.Model, manifest.Projector} {
		if err := os.Link(filepath.Join(models, artifact.Filename), filepath.Join(modelDir, artifact.Filename)); err != nil {
			t.Fatalf("hard-link local model fixture on the same volume: %v", err)
		}
	}
	s.Cfg.Directories.DataDir = dataDir
	s.Cfg.Decisions = config.DecisionsConfig{Enabled: true, Deployment: "native", CPUFallbackAllowed: false}
	manager := decisions.NewManager(s.Cfg.Decisions, dataDir, decisions.WithRuntimeDirectory(runtimeRoot))
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		if err := manager.Shutdown(stopCtx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := manager.Install(ctx); err != nil {
		t.Fatalf("install verified local artifacts: %v", err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start with CPU consent denied: %v; status=%+v", err, manager.Status())
	}
	status := manager.Status()
	t.Logf("device=%s backend=%s mode=%s gpu_offload=%v", status.Device, status.Backend, status.ExecutionMode, status.GPUOffloadVerified)
	recorder := &liveRecorder{Manager: manager}
	s.Decisions = newRoutedDecisions(s, recorder, s.Cfg)
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = s.Decisions.Shutdown(shutdownCtx)
	})
	on := config.DecisionConsumerSwitch{Enabled: true}
	s.Cfg.DecisionConsumers = config.DecisionConsumersConfig{
		MemoryRerank: on, Appraisal: on, SalienceGate: on, MailTriage: on, WebhookQuestions: on,
		PersonWake: on, IndexerImages: on, GameMakerVisual: on,
	}
	routerOn(s)
	hub := s.decisionHub()
	step := fakeHubClock(s) // the camera gate has a 2 s minimum interval

	caps, rows := acceptanceStatus(t, s)
	if caps["images"] != "supported" || caps["external"] == true {
		t.Fatalf("local backend must report images supported: %v", caps)
	}
	for _, id := range []consumers.ID{consumers.Router, consumers.PersonWake, consumers.IndexerImages, consumers.GameMakerVisual} {
		if rows[id]["available"] != true {
			t.Fatalf("%s not available: %v", id, rows[id])
		}
	}

	t.Run("camera gate", func(t *testing.T) {
		run := func(t *testing.T, name string, photo []byte) (suppressed bool) {
			t.Helper()
			cover := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(cover, photo, 0o600); err != nil {
				t.Fatal(err)
			}
			step()
			before, _, _, _ := recorder.snapshot()
			started := time.Now()
			suppress, hint := s.personWakeGate(context.Background(), persondetect.AlertEvent{EventID: 1, CameraID: "door", CameraName: "Door", Zones: []string{"Porch"}, CoverPath: cover})
			elapsed := time.Since(started)
			after, response, err, _ := recorder.snapshot()
			if after != before+1 || err != nil {
				t.Fatalf("gate did not complete one evaluation (calls %d -> %d, err=%v)", before, after, err)
			}
			t.Logf("%s: suppress=%v hint=%q latency=%s scene=%v", name, suppress, hint, elapsed.Round(time.Millisecond), response.Answers["scene"].Probabilities)
			return suppress
		}
		t.Run("empty room", func(t *testing.T) {
			run(t, "empty.jpg", liveEmptyRoom(t)) // model quality is logged, not asserted
		})
		t.Run("person", func(t *testing.T) {
			photo, ok := livePersonPhoto(t)
			if !ok {
				t.Skip("no person photo fixture found (internal/persondetect/selftest_reference.jpg)")
			}
			if run(t, "person.jpg", photo) {
				t.Fatal("an image with a person must never suppress the agent wake-up")
			}
		})
	})

	t.Run("indexer", func(t *testing.T) {
		indexer := newAcceptanceIndexer(t, s)
		allowed := map[string]bool{"": true, "photo": true, "screenshot": true, "document": true, "diagram": true, "other": true}
		cases := []struct {
			name string
			data []byte
		}{{"screenshot.png", liveScreenshot(t)}}
		if photo, ok := livePersonPhoto(t); ok {
			cases = append(cases, struct {
				name string
				data []byte
			}{"photo.jpg", photo})
		}
		for _, tc := range cases {
			before, _, _, _ := recorder.snapshot()
			started := time.Now()
			prompt := indexer.index(t, s, tc.name, tc.data)
			elapsed := time.Since(started)
			after, response, err, _ := recorder.snapshot()
			if after != before+1 || err != nil {
				t.Fatalf("%s: indexer did not complete one evaluation (calls %d -> %d, err=%v)", tc.name, before, after, err)
			}
			kind := response.Answers["kind"].Decision
			if !allowed[kind] {
				t.Fatalf("%s: unexpected kind %q", tc.name, kind)
			}
			t.Logf("%s: kind=%q has_text_p=%v latency=%s hint(text)=%v hint(photo)=%v probabilities=%v", tc.name, kind,
				response.Answers["text"].Probabilities["yes"], elapsed.Round(time.Millisecond),
				strings.Contains(prompt, acceptanceTextHint), strings.Contains(prompt, acceptancePhotoHint), response.Answers["kind"].Probabilities)
		}
	})

	t.Run("router preempts background image work", func(t *testing.T) {
		routerRequest := decisions.Request{
			State: "The user asks: what is the weather like in Berlin today?",
			Questions: []decisions.Question{{ID: "route", Type: "choice", Instructions: "Pick the best route for the message.",
				Options: []decisions.Option{{ID: "chat", Label: "Small talk"}, {ID: "tools", Label: "Needs a tool"}, {ID: "code", Label: "Coding task"}}}},
		}
		router := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
		if !router.Available {
			t.Fatalf("router unavailable: %+v", router.Code)
		}
		warm := time.Now()
		if _, err := router.Evaluate(context.Background(), routerRequest); err != nil {
			t.Fatalf("warm-up router call: %v", err)
		}
		t.Logf("router warm-up (idle): %s", time.Since(warm).Round(time.Millisecond))

		prepared, err := (taskDecisionService{s}).PrepareImage(liveScreenshot(t), 448)
		if err != nil {
			t.Fatal(err)
		}
		step()
		background := hub.Snapshot(s.Cfg, consumers.IndexerImages)
		if !background.Available {
			t.Fatalf("indexer_images unavailable: %+v", background.Code)
		}
		type outcome struct {
			err     error
			elapsed time.Duration
		}
		done := make(chan outcome, 1)
		go func() {
			started := time.Now()
			_, err := background.Evaluate(context.Background(), decisions.Request{
				State:     consumers.NewState().String(),
				Questions: []decisions.Question{{ID: "kind", Type: "choice", Instructions: "What kind of image is this?", Options: []decisions.Option{{ID: "photo", Label: "Photo"}, {ID: "screenshot", Label: "Screenshot"}, {ID: "other", Label: "Other"}}}},
				Images:    []decisions.Image{prepared},
			})
			done <- outcome{err, time.Since(started)}
		}()
		inFlight := func() bool {
			hub.mu.Lock()
			defer hub.mu.Unlock()
			return hub.background != nil
		}
		deadline := time.Now().Add(5 * time.Second)
		for !inFlight() {
			select {
			case result := <-done:
				t.Fatalf("background call ended before it could be observed in flight: err=%v after %s", result.err, result.elapsed)
			default:
			}
			if time.Now().After(deadline) {
				t.Fatal("background call was never admitted")
			}
			time.Sleep(time.Millisecond)
		}
		started := time.Now()
		_, routerErr := router.Evaluate(context.Background(), routerRequest)
		routerLatency := time.Since(started)
		t.Logf("router latency while background image work is in flight: %s (err=%v)", routerLatency.Round(time.Millisecond), routerErr)
		if routerErr != nil {
			t.Fatalf("router decision failed: %v", routerErr)
		}
		if routerLatency >= 1500*time.Millisecond {
			t.Fatalf("router latency %s exceeds 1.5 s", routerLatency)
		}
		select {
		case result := <-done:
			t.Logf("background call ended after %s: code=%q (preempted=%v)", result.elapsed.Round(time.Millisecond),
				decisions.ErrorCode(result.err), decisions.ErrorCode(result.err) == consumers.CodePreempted)
		case <-time.After(60 * time.Second):
			t.Fatal("background call never ended")
		}
		t.Logf("indexer_images stats: %+v", hub.Stats()[consumers.IndexerImages])
	})
}
