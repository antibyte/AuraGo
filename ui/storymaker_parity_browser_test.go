package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-rod/rod"
)

// Storymaker visual parity: the installed bitmap catalog (ui/img/storymaker) is rendered by the shared player through
// the real render page (contract section 7, png transport) in the
// installed Chrome, and chosen frames are written as PNG files to look at: the cast sheet in 16:9, firewall.json in 16:9, 9:16
// and 1:1. The test asserts only what a machine can judge (frame size, the story reports no error, no pixel is the unresolved
// token colour #ff00ff, the frames are not flat); artwork quality is a human look at the PNGs.
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_PARITY_DIR=<dir> go test ./ui -run TestStorymakerParityBrowser -count=1 -timeout 20m
//
// AURAGO_STORYMAKER_PARITY_ONLY=castsheet (or firewall) renders only the jobs whose name contains the text.

type storymakerParityScene struct {
	ID         string
	Start, End int
}

type storymakerParityJob struct {
	name          string
	story         string // a file below internal/storymaker/testdata
	aspect        string // overrides story.format.aspect when set
	width, height int
	cols          int // columns of the contact sheets (half-size tiles)
	rows          int
	pick          func(scenes []storymakerParityScene) (frames map[int]string, order []string)
}

type storymakerParityResult struct {
	frames map[string][]byte
	report map[string]any
	errors []map[string]any
}

// parityScenes reads the scene ranges of a compile report.
func parityScenes(report map[string]any) []storymakerParityScene {
	var out []storymakerParityScene
	raw, _ := report["scenes"].([]any)
	for _, s := range raw {
		m, _ := s.(map[string]any)
		id, _ := m["id"].(string)
		start, _ := m["start_frame"].(float64)
		end, _ := m["end_frame"].(float64)
		out = append(out, storymakerParityScene{ID: id, Start: int(start), End: int(end)})
	}
	return out
}

// parityStory reads a fixture story and optionally changes its aspect.
func parityStory(t *testing.T, rel, aspect string) []byte {
	t.Helper()
	data := storymakerFile(t, "../internal/storymaker/testdata/"+rel)
	if aspect == "" {
		return data
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	format, _ := doc["format"].(map[string]any)
	format["aspect"] = aspect
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func runStorymakerParity(t *testing.T, browser *rod.Browser, job storymakerParityJob) storymakerParityResult {
	t.Helper()
	const prefix = "/fedcba9876543210fedcba9876543210"
	res := storymakerParityResult{frames: map[string][]byte{}}
	var catalog struct {
		Files []smRasterFile `json:"files"`
		Fonts []struct {
			Name, Family, File string
			Weight             int
		} `json:"fonts"`
	}
	if err := json.Unmarshal(storymakerFile(t, "img/storymaker/catalog.json"), &catalog); err != nil {
		t.Fatalf("catalog.json: %v", err)
	}
	listed := map[string]string{"catalog.json": "application/json"}
	for _, file := range catalog.Files {
		listed[file.Path] = file.Type
	}
	storyJSON := parityStory(t, job.story, job.aspect)
	var mu sync.Mutex
	var wanted map[int]string
	last, total := -1, 0
	done := make(chan struct{})
	var closeOnce sync.Once
	finish := func() { closeOnce.Do(func() { close(done) }) }
	var origin string
	mux := http.NewServeMux()
	handle := func(p string, fn http.HandlerFunc) {
		mux.HandleFunc(prefix+p, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			fn(w, r)
		})
	}
	handle("/", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", fmt.Sprintf(storymakerRenderCSP, base))
		_, _ = fmt.Fprintf(w, `<!doctype html><html data-sm-mode="render" data-sm-base="%[1]s"><head><meta charset="utf-8"></head><body><canvas id="sm-stage" width="%[2]d" height="%[3]d"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, base, job.width, job.height)
	})
	handle("/boot.json", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		fonts := []map[string]any{}
		for _, f := range catalog.Fonts {
			fonts = append(fonts, map[string]any{"name": f.Name, "family": f.Family, "weight": f.Weight, "url": base + f.File})
		}
		codeBundles := map[string]string{}
		if strings.Contains(job.story, "firewall") {
			codeBundles["s3"] = base + "/scenes/s3.js"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode": "render", "fps": 30, "width": job.width, "height": job.height, "transport": "png", "frame_limit": 4000,
			"story_url": base + "/story.json", "catalog_url": base + "/catalog/catalog.json", "assets_base": base + "/catalog/",
			"code_bundles": codeBundles, "fonts": fonts, "audio_info": map[string]any{},
		})
	})
	handle("/story.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storyJSON)
	})
	handle("/catalog/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix+"/catalog/")
		if strings.Contains(rel, "..") || listed[rel] == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", listed[rel])
		_, _ = w.Write(storymakerFile(t, "img/storymaker/"+path.Clean(rel)))
	})
	handle("/fonts/", func(w http.ResponseWriter, r *http.Request) {
		name := path.Base(r.URL.Path)
		allowed := false
		for _, f := range catalog.Fonts {
			allowed = allowed || path.Base(f.File) == name
		}
		if !allowed {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write(storymakerFile(t, "fonts/"+name))
	})
	handle("/js/vendor/storymaker/runtime.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "js/vendor/storymaker/runtime.js"))
	})
	for _, bundle := range []string{"host", "player"} {
		file := bundle + ".bundle.js"
		handle("/js/storymaker/"+file, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, _ = w.Write(storymakerFile(t, "js/storymaker/"+file))
		})
	}
	handle("/scenes/s3.js", func(w http.ResponseWriter, r *http.Request) {
		src := storymakerFile(t, "../internal/storymaker/testdata/scenes/s3.js")
		out := api.Transform(string(src), api.TransformOptions{Format: api.FormatIIFE, GlobalName: "__smSceneModule", Target: api.ES2020, LegalComments: api.LegalCommentsNone, Loader: api.LoaderJS, Sourcefile: "scenes/s3.js"})
		if len(out.Errors) != 0 {
			http.Error(w, out.Errors[0].Text, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write(append(out.Code, []byte("\nself.__smScene=__smSceneModule.default;\n")...))
	})
	handle("/report", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		mu.Lock()
		defer mu.Unlock()
		_ = json.Unmarshal(body, &res.report)
		total = int(res.report["frame_count"].(float64))
		wanted, _ = job.pick(parityScenes(res.report))
		for n := range wanted {
			last = max(last, n)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"start":true,"from_frame":0}`)
	})
	handle("/error", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
		mu.Lock()
		res.errors = append(res.errors, body)
		mu.Unlock()
		finish()
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	handle("/frame", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		var n int
		_, _ = fmt.Sscanf(r.URL.Query().Get("n"), "%d", &n)
		mu.Lock()
		defer mu.Unlock()
		if name, ok := wanted[n]; ok {
			res.frames[name] = data
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case n >= last:
			_, _ = io.WriteString(w, `{"stop":true}`)
			finish()
		case n == total-1:
			_, _ = io.WriteString(w, `{"ok":true,"done":true}`)
			finish()
		default:
			_, _ = io.WriteString(w, `{"ok":true}`)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	origin = srv.URL

	page := browser.MustPage(origin + prefix + "/").Timeout(10 * time.Minute)
	defer page.Close()
	select {
	case <-done:
	case <-time.After(9 * time.Minute):
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("%s: the render page did not finish: frames=%d errors=%v status=%s", job.name, len(res.frames), res.errors, page.MustEval(`() => JSON.stringify(window.__smStatus || null)`).Str())
	}
	mu.Lock()
	defer mu.Unlock()
	return res
}

// halve averages 2x2 pixel blocks.
func halveImage(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()/2))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			var r, g, bl, a uint32
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					pr, pg, pb, pa := src.At(b.Min.X+2*x+dx, b.Min.Y+2*y+dy).RGBA()
					r, g, bl, a = r+pr, g+pg, bl+pb, a+pa
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r >> 10), uint8(g >> 10), uint8(bl >> 10), uint8(a >> 10)})
		}
	}
	return dst
}

// writeContactSheets tiles half-size copies of the frames, cols x rows tiles per sheet, and writes a text file with the tile names.
func writeContactSheets(t *testing.T, dir, prefix string, names []string, frames map[string][]byte, w, h, cols, rows int) {
	t.Helper()
	per := cols * rows
	for sheet := 0; sheet*per < len(names); sheet++ {
		tiles := names[sheet*per : min(len(names), (sheet+1)*per)]
		img := image.NewRGBA(image.Rect(0, 0, cols*(w/2), rows*(h/2)))
		for i, name := range tiles {
			src, err := png.Decode(bytes.NewReader(frames[name]))
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			half := halveImage(src)
			at := image.Pt((i%cols)*(w/2), (i/cols)*(h/2))
			for y := 0; y < half.Bounds().Dy(); y++ {
				for x := 0; x < half.Bounds().Dx(); x++ {
					img.SetRGBA(at.X+x, at.Y+y, half.RGBAAt(x, y))
				}
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		stem := filepath.Join(dir, fmt.Sprintf("%s-sheet-%02d", prefix, sheet+1))
		if err := os.WriteFile(stem+".png", buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stem+".txt", []byte(strings.Join(tiles, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// checkParityFrame asserts what a machine can judge about one frame.
func checkParityFrame(t *testing.T, name string, data []byte, w, h int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("%s: not a PNG: %v", name, err)
		return
	}
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Errorf("%s: %dx%d, want %dx%d", name, b.Dx(), b.Dy(), w, h)
		return
	}
	colours := map[uint32]bool{}
	magenta := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 1 {
		for x := b.Min.X; x < b.Max.X; x += 1 {
			r, g, bl, _ := img.At(x, y).RGBA()
			r8, g8, b8 := r>>8, g>>8, bl>>8
			if r8 == 255 && g8 == 0 && b8 == 255 {
				magenta++
			}
			if len(colours) < 4096 {
				colours[r8<<16|g8<<8|b8] = true
			}
		}
	}
	if magenta > 0 {
		t.Errorf("%s: %d pixels have the unresolved-token colour #ff00ff", name, magenta)
	}
	if len(colours) < 64 {
		t.Errorf("%s: only %d distinct colours, the frame is nearly flat", name, len(colours))
	}
}

func TestStorymakerParityBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker parity test")
	}
	dir := os.Getenv("AURAGO_STORYMAKER_PARITY_DIR")
	if dir == "" {
		t.Skip("set AURAGO_STORYMAKER_PARITY_DIR to the folder that receives the PNG frames")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	only := os.Getenv("AURAGO_STORYMAKER_PARITY_ONLY")

	// the cast sheet: every scene lasts 1.2 s with a cut in, so 0.7 s into a scene (21 frames) is where its beats are in their held part
	castPick := func(scenes []storymakerParityScene) (map[int]string, []string) {
		frames, order := map[int]string{}, []string{}
		for i, s := range scenes {
			name := fmt.Sprintf("%02d-%s", i+1, s.ID)
			frames[s.Start+21] = name
			order = append(order, name)
		}
		return frames, order
	}
	// firewall: three frames of each storyboard scene and the middle of the code scene
	firePick := func(scenes []storymakerParityScene) (map[int]string, []string) {
		frames, order := map[int]string{}, []string{}
		for _, s := range scenes {
			for _, pct := range []int{25, 55, 85} {
				if s.ID == "s3" && pct != 55 {
					continue
				}
				name := fmt.Sprintf("%s-%d", s.ID, pct)
				frames[s.Start+(s.End-s.Start)*pct/100] = name
				order = append(order, name)
			}
		}
		return frames, order
	}
	jobs := []storymakerParityJob{
		{name: "castsheet", story: "player/castsheet.json", width: 1920, height: 1080, cols: 2, rows: 3, pick: castPick},
		{name: "firewall-16x9", story: "stories/firewall.json", width: 1920, height: 1080, cols: 2, rows: 3, pick: firePick},
		{name: "firewall-9x16", story: "stories/firewall.json", aspect: "9:16", width: 1080, height: 1920, cols: 4, rows: 1, pick: firePick},
		{name: "firewall-1x1", story: "stories/firewall.json", aspect: "1:1", width: 1080, height: 1080, cols: 3, rows: 2, pick: firePick},
	}
	browser := newSmokeBrowser(t)
	for _, job := range jobs {
		if only != "" && !strings.Contains(job.name, only) {
			continue
		}
		t.Run(job.name, func(t *testing.T) {
			started := time.Now()
			res := runStorymakerParity(t, browser, job)
			if len(res.errors) != 0 {
				t.Fatalf("page errors: %v", res.errors)
			}
			issues, _ := res.report["issues"].([]any)
			for _, raw := range issues {
				if is, _ := raw.(map[string]any); is["severity"] == "error" {
					t.Errorf("the compile report has an error issue: %v", is)
				}
			}
			_, order := job.pick(parityScenes(res.report))
			for _, name := range order {
				data, ok := res.frames[name]
				if !ok {
					t.Errorf("frame %s was never posted", name)
					continue
				}
				checkParityFrame(t, name, data, job.width, job.height)
				if err := os.WriteFile(filepath.Join(dir, job.name+"-"+name+".png"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if len(res.frames) == len(order) {
				writeContactSheets(t, dir, job.name, order, res.frames, job.width, job.height, job.cols, job.rows)
			}
			t.Logf("%s: %d frames written to %s in %s", job.name, len(res.frames), dir, time.Since(started).Round(time.Second))
		})
	}
}
