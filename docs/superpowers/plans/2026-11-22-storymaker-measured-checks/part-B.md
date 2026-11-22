# Part B: Text contrast on the stills

> Generated from the drafted and tested commits e84044a7a..2a529bde3. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task B1: the contrast of a box on a picture from its luminance percentiles

**Files:**
- Create: `internal/storymaker/contrast.go`
- Create: `internal/storymaker/contrast_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/contrast_test.go`:

```go
package storymaker

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// stripedImage is a w x h picture of the background grey with vertical strokes of the stroke grey, two pixels wide at every
// fourth column, inside the pixel rectangle box. Outside the box the picture is black.
func stripedImage(w, h int, box image.Rectangle, bg, stroke uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(0)
			if image.Pt(x, y).In(box) {
				v = bg
				if x%4 < 2 {
					v = stroke
				}
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBoxContrastReadsStrokesAgainstTheGround(t *testing.T) {
	full := image.Rect(0, 0, 200, 100)
	box := LayoutBox{0, 0, 1, 1}
	cases := []struct {
		name       string
		bg, stroke uint8
		min, max   float64
	}{
		{"black strokes on white", 255, 0, 15, 22},
		{"mid grey on slightly lighter grey", 150, 128, 1.15, 3},
		{"uniform grey", 128, 128, 1, 1.0001},
	}
	for _, c := range cases {
		ratio, ok := boxContrast(stripedImage(200, 100, full, c.bg, c.stroke), box)
		if !ok || ratio < c.min || ratio > c.max {
			t.Errorf("%s: ratio %.3f ok %v, want %.2f..%.2f", c.name, ratio, ok, c.min, c.max)
		}
	}
}

func TestBoxContrastClampsTheBoxAndSkipsATinyOne(t *testing.T) {
	img := stripedImage(200, 100, image.Rect(0, 0, 100, 100), 255, 0)
	// The box sticks out of the frame on the right: only the part inside is measured, and it is the striped half.
	if ratio, ok := boxContrast(img, LayoutBox{0.2, 0.2, 2, 0.5}); !ok || ratio < 15 {
		t.Errorf("a clamped box: ratio %.3f ok %v", ratio, ok)
	}
	if _, ok := boxContrast(img, LayoutBox{0.2, 0.2, 0.004, 0.5}); ok {
		t.Error("a box under 2 pixels wide must be skipped")
	}
	if _, ok := boxContrast(img, LayoutBox{1.5, 0.2, 0.2, 0.2}); ok {
		t.Error("a box outside the frame must be skipped")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run '(?i)BoxContrast' -count=1 -v`
Expected: internal\storymaker\contrast_test.go:52:16: undefined: boxContrast

- [ ] **Step 3: Implement**

Create `internal/storymaker/contrast.go`:

```go
package storymaker

import (
	"image"
	"math"
	"sort"
)

const (
	// contrastMin is the ratio below which a text is reported: WCAG's 3:1 for large text, which every Storymaker text is.
	contrastMin = 3.0
	// contrastMasked is the ratio below which the box is taken for empty (the text is masked or not drawn) and not reported.
	contrastMasked = 1.15
	// contrastMaxSamples bounds the sample grid inside one box.
	contrastMaxSamples = 20000
)

// linearChannel turns an sRGB channel (0..1) into linear light.
func linearChannel(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// relativeLuminance is the WCAG relative luminance of a 16 bit sRGB colour.
func relativeLuminance(r, g, b uint32) float64 {
	return 0.2126*linearChannel(float64(r)/65535) + 0.7152*linearChannel(float64(g)/65535) + 0.0722*linearChannel(float64(b)/65535)
}

// boxContrast measures the contrast inside a box given as fractions of the picture: the 3rd and 97th percentile of the relative
// luminance over an evenly spaced grid of at most contrastMaxSamples samples give (L97 + 0.05) / (L3 + 0.05). The box is clamped
// to the picture; ok is false when less than 2 x 2 pixels of it lie inside.
func boxContrast(img image.Image, box LayoutBox) (ratio float64, ok bool) {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	x0 := max(b.Min.X, int(math.Floor(box[0]*w))+b.Min.X)
	y0 := max(b.Min.Y, int(math.Floor(box[1]*h))+b.Min.Y)
	x1 := min(b.Max.X, int(math.Ceil((box[0]+box[2])*w))+b.Min.X)
	y1 := min(b.Max.Y, int(math.Ceil((box[1]+box[3])*h))+b.Min.Y)
	if x1-x0 < 2 || y1-y0 < 2 {
		return 0, false
	}
	step := max(1, int(math.Ceil(math.Sqrt(float64(x1-x0)*float64(y1-y0)/contrastMaxSamples))))
	var lum []float64
	for y := y0; y < y1; y += step {
		for x := x0; x < x1; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			lum = append(lum, relativeLuminance(r, g, bl))
		}
	}
	sort.Float64s(lum)
	last := len(lum) - 1
	low := lum[int(math.Round(0.03*float64(last)))]
	high := lum[int(math.Round(0.97*float64(last)))]
	return (high + 0.05) / (low + 0.05), true
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run '(?i)BoxContrast' -count=1 -v`
Expected: --- PASS: TestBoxContrastClampsTheBoxAndSkipsATinyOne (0.00s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/contrast.go internal/storymaker/contrast_test.go
git commit -m "storymaker: the contrast of a box on a picture from its luminance percentiles"
```

### Task B2: measure the texts and the caption of a still

**Files:**
- Modify: `internal/storymaker/contrast.go`
- Modify: `internal/storymaker/contrast_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/contrast_test.go`, replace:

```go
		t.Error("a box outside the frame must be skipped")
	}
}
```

with:

```go
		t.Error("a box outside the frame must be skipped")
	}
}

func TestMeasureStillContrastMeasuresTextsAndCaption(t *testing.T) {
	// Left half: black strokes on white (the text "title"); right half uniform grey (the text "note"); the caption band is striped
	// grey on slightly lighter grey.
	img := stripedImage(200, 100, image.Rect(0, 0, 100, 60), 255, 0)
	for y := 0; y < 60; y++ {
		for x := 100; x < 200; x++ {
			img.Set(x, y, color.RGBA{128, 128, 128, 255})
		}
	}
	band := stripedImage(200, 100, image.Rect(0, 70, 200, 100), 150, 128)
	for y := 70; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, band.At(x, y))
		}
	}
	scene := layoutScene("s1", 0)
	scene.Texts = []LayoutText{{ID: "title", Box: LayoutBox{0, 0, 0.5, 0.6}}, {ID: "note", Box: LayoutBox{0.5, 0, 0.5, 0.6}}}
	caption := LayoutBox{0, 0.7, 1, 0.3}
	scene.Caption = &caption
	got, err := MeasureStillContrast(encodePNG(t, img), scene)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != "title" || got[1].ID != "note" || got[2].ID != "caption" {
		t.Fatalf("measured %+v, want title, note, caption in order", got)
	}
	if got[0].Ratio < 15 || got[1].Ratio > 1.0001 || got[2].Ratio < 1.15 || got[2].Ratio >= contrastMin {
		t.Errorf("ratios %+v", got)
	}
}

func TestMeasureStillContrastSkipsATinyBoxAndRefusesANonPNG(t *testing.T) {
	scene := layoutScene("s1", 0)
	scene.Texts = []LayoutText{{ID: "dot", Box: LayoutBox{0.5, 0.5, 0.001, 0.001}}}
	got, err := MeasureStillContrast(encodePNG(t, stripedImage(200, 100, image.Rect(0, 0, 200, 100), 255, 0)), scene)
	if err != nil || len(got) != 0 {
		t.Errorf("a tiny box: %+v, %v", got, err)
	}
	if _, err := MeasureStillContrast([]byte("not a png"), scene); err == nil {
		t.Error("a non-PNG must be refused")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run '(?i)(MeasureStillContrast|BoxContrast)' -count=1 -v`
Expected: internal\storymaker\contrast_test.go:92:14: undefined: MeasureStillContrast

- [ ] **Step 3: Implement**

In `internal/storymaker/contrast.go`, replace:

```go
	high := lum[int(math.Round(0.97*float64(last)))]
	return (high + 0.05) / (low + 0.05), true
}
```

with:

```go
	high := lum[int(math.Round(0.97*float64(last)))]
	return (high + 0.05) / (low + 0.05), true
}

// TextContrast is the measured contrast of one text box on a still; ID is the text id or "caption".
type TextContrast struct {
	ID    string  `json:"id"`
	Ratio float64 `json:"ratio"`
}

// MeasureStillContrast decodes one still (through stillConfig and decodeStill, so the size caps hold) and measures every text
// box and the caption box of its layout scene. Boxes are fractions of the frame, as in LayoutScene. A box too small to measure
// is left out.
func MeasureStillContrast(data []byte, scene LayoutScene) ([]TextContrast, error) {
	img, err := decodeStill(data)
	if err != nil {
		return nil, err
	}
	var out []TextContrast
	for _, text := range scene.Texts {
		if ratio, ok := boxContrast(img, text.Box); ok {
			out = append(out, TextContrast{ID: text.ID, Ratio: ratio})
		}
	}
	if scene.Caption != nil {
		if ratio, ok := boxContrast(img, *scene.Caption); ok {
			out = append(out, TextContrast{ID: "caption", Ratio: ratio})
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run '(?i)(MeasureStillContrast|BoxContrast)' -count=1 -v`
Expected: --- PASS: TestMeasureStillContrastSkipsATinyBoxAndRefusesANonPNG (0.00s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/contrast.go internal/storymaker/contrast_test.go
git commit -m "storymaker: measure the texts and the caption of a still"
```

### Task B3: turn low measured contrast into text_contrast warnings

**Files:**
- Modify: `internal/storymaker/contrast.go`
- Modify: `internal/storymaker/contrast_test.go`
- Modify: `internal/storymaker/stills.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/contrast_test.go`, replace:

```go
	"image"
	"image/color"
	"image/png"
	"testing"
)
```

with:

```go
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)
```

In `internal/storymaker/contrast_test.go`, replace:

```go
		t.Error("a non-PNG must be refused")
	}
}
```

with:

```go
		t.Error("a non-PNG must be refused")
	}
}

func TestContrastIssuesReportsLowButNotMaskedTexts(t *testing.T) {
	stills := []StillInfo{
		{SceneID: "s1", Index: 0, Texts: []TextContrast{{ID: "title", Ratio: 4.2}, {ID: "note", Ratio: 1.0}}},
		{SceneID: "s2", Index: 3, Texts: []TextContrast{{ID: "claim", Ratio: 2.04}, {ID: "caption", Ratio: 1.15}, {ID: "edge", Ratio: 3.0}}},
	}
	got := ContrastIssues(stills)
	if len(got) != 2 {
		t.Fatalf("want the claim and the caption, got %+v", got)
	}
	claim, caption := got[0], got[1]
	if claim.Severity != SeverityWarning || claim.Code != CodeTextContrast || claim.SceneID != "s2" || claim.Path != "/scenes/3" {
		t.Errorf("claim issue %+v", claim)
	}
	if !strings.Contains(claim.Message, "claim") || !strings.Contains(claim.Message, "2.0") || claim.Fix == "" {
		t.Errorf("claim message %q fix %q", claim.Message, claim.Fix)
	}
	if !strings.Contains(caption.Message, "the caption") || !strings.Contains(caption.Message, "1.1") {
		t.Errorf("caption message %q", caption.Message)
	}
	if ContrastIssues(nil) != nil {
		t.Error("no stills, no issues")
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run '(?i)ContrastIssues' -count=1 -v`
Expected: internal\storymaker\contrast_test.go:119:29: unknown field Texts in struct literal of type StillInfo

- [ ] **Step 3: Implement**

In `internal/storymaker/contrast.go`, replace:

```go
package storymaker

import (
	"image"
	"math"
	"sort"
)

const (
```

with:

```go
package storymaker

import (
	"fmt"
	"image"
	"math"
	"sort"
	"strconv"
)

const (
```

In `internal/storymaker/contrast.go`, replace:

```go
	}
	return out, nil
}
```

with:

```go
	}
	return out, nil
}

// ContrastIssues turns the measured ratios into text_contrast warnings: a ratio from contrastMasked up to contrastMin. A lower
// one means the box is empty (the text is masked or not drawn), which is not a contrast problem of the picture.
func ContrastIssues(stills []StillInfo) []Issue {
	var out []Issue
	for _, still := range stills {
		for _, text := range still.Texts {
			if text.Ratio < contrastMasked || text.Ratio >= contrastMin {
				continue
			}
			name := "the text " + strconv.Quote(text.ID)
			if text.ID == "caption" {
				name = "the caption"
			}
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: still.SceneID, Path: jp("scenes", still.Index), Code: CodeTextContrast,
				Message: fmt.Sprintf("In scene %s %s has a contrast of %.1f:1 against the picture behind it, under the %.0f:1 a viewer needs.", still.SceneID, name, text.Ratio, contrastMin),
				Fix:     "Put the text on a calmer, darker or lighter area of the picture, or change the environment or the actors so the background behind it is calm.",
			})
		}
	}
	return out
}
```

In `internal/storymaker/stills.go`, replace:

```go
	lookAlikeMaxDistance = 16
)

// StillInfo is one captured still: the scene, its index in the story and the difference hash of the picture.
type StillInfo struct {
	SceneID string
	Index   int
	Hash    uint64
}

// ErrSheetTooLarge says that a contact sheet would exceed the pixel budget even with the narrowest thumbnails.
```

with:

```go
	lookAlikeMaxDistance = 16
)

// StillInfo is one captured still: the scene, its index in the story, the difference hash of the picture and the measured
// contrast of its text boxes (empty when none was measured).
type StillInfo struct {
	SceneID string
	Index   int
	Hash    uint64
	Texts   []TextContrast
}

// ErrSheetTooLarge says that a contact sheet would exceed the pixel budget even with the narrowest thumbnails.
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run '(?i)ContrastIssues' -count=1 -v`
Expected: --- PASS: TestContrastIssuesReportsLowButNotMaskedTexts (0.00s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/contrast.go internal/storymaker/contrast_test.go internal/storymaker/stills.go
git commit -m "storymaker: turn low measured contrast into text_contrast warnings"
```

### Task B4: captureStills measures the text contrast of each still

**Files:**
- Modify: `internal/storymaker/render/compilestills.go`
- Modify: `internal/storymaker/render/compilestills_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/render/compilestills_test.go`, replace:

```go
		t.Fatalf("captured %d, summary %+v", captured, got)
	}
}
```

with:

```go
		t.Fatalf("captured %d, summary %+v", captured, got)
	}
}

// lowContrastStill is a 64x36 picture of grey strokes on a slightly lighter grey: a text box over it measures about 1.4:1.
func lowContrastStill(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			v := uint8(150)
			if x%4 < 2 {
				v = 128
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The stills carry the contrast of the text boxes of their own scene, measured before the PNG bytes are dropped.
func TestCompileCheckMeasuresTheTextContrastOfEachStill(t *testing.T) {
	const twoScenes = `{"schema":"storymaker/1","title":"Stills","scenes":[{"id":"s1","narration":"One."},{"id":"s2","narration":"Two."}]}`
	page := &fakePage{crashed: make(chan struct{})}
	page.capture = func(n int) []byte { return lowContrastStill(t) }
	page.script = play(t, playOptions{frames: 90, mutateRep: func(rep map[string]any) {
		withText := map[string]any{"id": "s1", "index": 0, "hold_s": 1.0, "actors": []any{}, "props": []any{}, "missing": 0, "static": false,
			"texts": []any{map[string]any{"id": "title", "box": []any{0.1, 0.1, 0.8, 0.3}}}, "caption": []any{0.1, 0.6, 0.8, 0.3}}
		plain := map[string]any{"id": "s2", "index": 1, "hold_s": 1.0, "actors": []any{}, "props": []any{}, "texts": []any{}, "caption": nil, "missing": 0, "static": false}
		rep["layout"] = []any{withText, plain}
	}})
	runner, id := compileCheckRunner(t, page)
	if _, err := runner.deps.Projects.SaveStory(context.Background(), id, []byte(twoScenes), ""); err != nil {
		t.Fatal(err)
	}
	got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits(), Stills: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Stills) != 2 {
		t.Fatalf("stills %+v", got.Stills)
	}
	texts := got.Stills[0].Texts
	if len(texts) != 2 || texts[0].ID != "title" || texts[1].ID != "caption" {
		t.Fatalf("the first still measured %+v, want title and caption", texts)
	}
	for _, text := range texts {
		if text.Ratio < 1.15 || text.Ratio >= 3 {
			t.Errorf("%s measured %.2f, want a low but drawn contrast", text.ID, text.Ratio)
		}
	}
	if len(got.Stills[1].Texts) != 0 {
		t.Errorf("a scene without texts measures none: %+v", got.Stills[1].Texts)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/render/ -run '(?i)MeasuresTheTextContrast' -count=1`
Expected: compilestills_test.go:216: the first still measured [], want title and caption

- [ ] **Step 3: Implement**

In `internal/storymaker/render/compilestills.go`, replace:

```go
	if len(sheet) == 0 {
		r.deps.Logger.Warn("[Storymaker] The contact sheet would be too large and was left out; the hashes stay", "stills", len(stills))
	}
	summary.Sheet, summary.Stills = sheet, stills
}
```

with:

```go
	if len(sheet) == 0 {
		r.deps.Logger.Warn("[Storymaker] The contact sheet would be too large and was left out; the hashes stay", "stills", len(stills))
	}
	// The contrast of the text boxes is measured here, because the PNG bytes are gone after this function. A measuring error drops
	// the ratios of that one still and nothing else.
	layoutByID := make(map[string]storymaker.LayoutScene, len(summary.Layout))
	for _, scene := range summary.Layout {
		layoutByID[scene.ID] = scene
	}
	for i := range stills {
		texts, err := storymaker.MeasureStillContrast(pngs[i], layoutByID[stills[i].SceneID])
		if err != nil {
			r.deps.Logger.Warn("[Storymaker] The text contrast of a still could not be measured", "scene", stills[i].SceneID, "error", err)
			continue
		}
		stills[i].Texts = texts
	}
	summary.Sheet, summary.Stills = sheet, stills
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/render/ -run '(?i)MeasuresTheTextContrast' -count=1`
Expected: --- PASS: TestCompileCheckMeasuresTheTextContrastOfEachStill (0.23s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/compilestills.go internal/storymaker/render/compilestills_test.go
git commit -m "storymaker: captureStills measures the text contrast of each still"
```

### Task B5: the Studio check reports low text contrast as advice

**Files:**
- Modify: `internal/storymaker/studio_check.go`
- Modify: `internal/storymaker/studio_layout_check_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_layout_check_test.go`, replace:

```go
		t.Error("a camera beat is exempt")
	}
}
```

with:

```go
		t.Error("a camera beat is exempt")
	}
}

// A measured low contrast on a still is advice: it is listed for the model, never an issue that blocks, and a masked box
// (a ratio under 1.15) is not reported at all.
func TestCheckReportsTextContrastAsAdvice(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{
			Available: true, DurationMS: 8000, StoryboardScenes: 2,
			Layout: []LayoutScene{layoutScene("s1", 0), layoutScene("s2", 1)},
			Stills: []StillInfo{
				{SceneID: "s1", Index: 0, Hash: 1, Texts: []TextContrast{{ID: "title", Ratio: 1.8}}},
				{SceneID: "s2", Index: 1, Hash: 0xFFFF0000FFFF0000, Texts: []TextContrast{{ID: "note", Ratio: 1.0}}},
			},
		}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	contrast := func(list []Issue) (found []Issue) {
		for _, is := range list {
			if is.Code == CodeTextContrast {
				found = append(found, is)
			}
		}
		return found
	}
	advice := contrast(report.Advice)
	if len(advice) != 1 || advice[0].SceneID != "s1" || advice[0].Path != "/scenes/0" {
		t.Fatalf("advice %+v, want one text_contrast for s1 only", report.Advice)
	}
	if len(contrast(report.Issues)) != 0 || len(contrast(report.Environment)) != 0 {
		t.Fatalf("a contrast finding is advice only: issues %+v, environment %+v", report.Issues, report.Environment)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run '(?i)TextContrastAsAdvice' -count=1 -v`
Expected: studio_layout_check_test.go:187: advice [], want one text_contrast for s1 only

- [ ] **Step 3: Implement**

In `internal/storymaker/studio_check.go`, replace:

```go
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LayoutIssues(summary.Layout, StoryLayoutOptions(&story))...)
	found = append(found, LookAlikeIssues(summary.Stills)...)
	found = append(found, LintForJob(&story)...)
	found = append(found, s.craftIssues(ctx, &story)...)
	found = append(found, LintWords(&story, s.job.Language)...)
```

with:

```go
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LayoutIssues(summary.Layout, StoryLayoutOptions(&story))...)
	found = append(found, LookAlikeIssues(summary.Stills)...)
	found = append(found, ContrastIssues(summary.Stills)...)
	found = append(found, LintForJob(&story)...)
	found = append(found, s.craftIssues(ctx, &story)...)
	found = append(found, LintWords(&story, s.job.Language)...)
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run '(?i)TextContrastAsAdvice' -count=1 -v`
Expected: --- PASS: TestCheckReportsTextContrastAsAdvice (0.03s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_check.go internal/storymaker/studio_layout_check_test.go
git commit -m "storymaker: the Studio check reports low text contrast as advice"
```

