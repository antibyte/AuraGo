# Part E

Tasks of batch E: one still per scene from the same compile run, a contact sheet the user can open from the job card, and the `scenes_look_alike` finding. The last task checks the thresholds of batches B, D and E on a real frame and corrects the spec. Rules, commands and the order are in the plan index. Decisions are in [design.md](design.md) and below.

**Depends on batch D** (the stills use the hold frames of its layout run and the same browser run). Batch D must have landed or be in the same line of commits.

## Decisions of batch E

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| E-1 | Where are the stills taken? | In the same browser run as the compile report. With `Settings.Stills` the page gets the screenshot transport, the report is answered with `start` instead of `compile_only`, and Go captures one frame per measured scene before it shuts the listener down. | `CompileCheck` answers the report with an error so the page stops (`compilecheck.go:124-131`); a second browser start for stills would double the time of every check. |
| E-2 | Which frame? | The hold frame of the layout block, clamped to the scene and to the frames the page offers. A scene without a layout block gets no still. | The hold frame is the moment the scene shows everything. |
| E-3 | Time limit | `compileCheckLimit` x 3 when stills are on (180 s). `WaitReady` keeps its own 60 s, each capture its 30 s. | Seven scenes plus the browser start can exceed 60 s. |
| E-4 | What is cached? | `CompileSummary` keeps the hashes (`Stills`) and, until the job saves it, the PNG sheet (`Sheet`). The job saves the sheet to `review/contact-sheet.png` of the project and clears it from the summary. | The Studio caches a summary per story version; a 1 MB sheet per version is not worth keeping. |
| E-5 | `scenes_look_alike` | A warning of the layout class (model-fixable, never blocks), from difference hashes (9x8, 64 bits). Three or more consecutive storyboard scenes whose neighbours differ by at most `lookAlikeMaxDistance` bits. The starting value 6 is tuned in task E6. | The spec asked for a perceptual hash; the tuning needs a real frame. |
| E-6 | Vision review (spec 2.3) | **Dropped from this plan.** The agent loop can carry an image part (`MultiContent`), but the policy that refuses local images for Agnes AI (`tools/vision.go:144-153`, `game_maker_visual.go:26`) applies to the provider the five real runs used. A review that never runs on the user's provider is not worth 5 to 8 files. | The spec said the item is dropped when it cannot be carried. It can be carried, but not on the user's provider. |
| E-7 | Route | `GET /api/desktop/storymaker/projects/{id}/review/contact-sheet.png`, read scope, fixed name, same inert headers as the audio route. The job document carries `agent.review` = `{sheet, scenes}`. | The only file route today is audio. |

<!-- group E1 -->

### Task E1: Hashes, contact sheet and the look-alike finding
**Batch:** E  **Depends on:** none (pure code)
**Files:**
Create `internal/storymaker/stills.go`
Test `internal/storymaker/stills_test.go`

`golang.org/x/image` is a direct dependency (`go.mod`) and `golang.org/x/image/draw` is already used by `internal/tools/vision_image.go`; nothing new is added.

- [ ] **Step 1: Write the failing test.** Create `internal/storymaker/stills_test.go`:

```go
package storymaker

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// stillPNG draws a 64x36 horizontal gradient, light to dark when rising is false. A patch of 2x2 bright pixels in the corner
// is a small difference that must not change the picture.
func stillPNG(t *testing.T, rising bool, patch bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			v := uint8(x * 255 / 63)
			if !rising {
				v = 255 - v
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	if patch {
		for y := 0; y < 2; y++ {
			for x := 62; x < 64; x++ {
				img.Set(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDifferenceHashIsStableAndTellsPicturesApart(t *testing.T) {
	a, err := DifferenceHash(stillPNG(t, true, false))
	if err != nil {
		t.Fatal(err)
	}
	same, _ := DifferenceHash(stillPNG(t, true, false))
	patched, _ := DifferenceHash(stillPNG(t, true, true))
	flipped, _ := DifferenceHash(stillPNG(t, false, false))
	if HashDistance(a, same) != 0 {
		t.Errorf("the same picture hashes the same: %d", HashDistance(a, same))
	}
	if d := HashDistance(a, patched); d > lookAlikeMaxDistance {
		t.Errorf("a 2x2 patch moved the hash by %d bits", d)
	}
	if d := HashDistance(a, flipped); d < 48 {
		t.Errorf("a flipped gradient differs by %d bits, want almost all 64", d)
	}
	if _, err := DifferenceHash([]byte("not a png")); err == nil {
		t.Error("a broken still must be an error")
	}
}

func TestContactSheetLaysStillsOutInFourColumns(t *testing.T) {
	var pngs [][]byte
	for i := 0; i < 5; i++ {
		pngs = append(pngs, stillPNG(t, i%2 == 0, false))
	}
	sheet, err := ContactSheet(pngs)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(sheet))
	if err != nil {
		t.Fatal(err)
	}
	// 64x36 stills scaled to 480 wide are 270 high: 4 columns, 2 rows, 8 pixel gaps.
	want := image.Rect(0, 0, 4*StillsThumbWidth+5*stillsGap, 2*270+3*stillsGap)
	if img.Bounds() != want {
		t.Fatalf("sheet is %v, want %v", img.Bounds(), want)
	}
	if _, err := ContactSheet(nil); err == nil {
		t.Error("a sheet without stills is an error")
	}
	short, err := ContactSheet(pngs[:2])
	if err != nil {
		t.Fatal(err)
	}
	if img, _ := png.Decode(bytes.NewReader(short)); img.Bounds().Dx() != 2*StillsThumbWidth+3*stillsGap {
		t.Errorf("two stills use two columns, got width %d", img.Bounds().Dx())
	}
}

func TestBuildStillsKeepsOrderAndRefusesAMismatch(t *testing.T) {
	pngs := [][]byte{stillPNG(t, true, false), stillPNG(t, false, false)}
	sheet, stills, err := BuildStills([]string{"s1", "s3"}, []int{0, 2}, pngs)
	if err != nil || len(sheet) == 0 {
		t.Fatalf("BuildStills = %d bytes, %v", len(sheet), err)
	}
	if len(stills) != 2 || stills[0].SceneID != "s1" || stills[1].SceneID != "s3" || stills[1].Index != 2 || HashDistance(stills[0].Hash, stills[1].Hash) < 48 {
		t.Fatalf("stills = %+v", stills)
	}
	if _, _, err := BuildStills([]string{"s1"}, []int{0, 1}, pngs); err == nil {
		t.Error("lists of different length are refused")
	}
	if _, _, err := BuildStills([]string{"s1", "s2"}, []int{0, 1}, [][]byte{pngs[0], []byte("x")}); err == nil {
		t.Error("a broken still is refused")
	}
}

func TestLookAlikeIssuesFindsRunsOfThreeOrMore(t *testing.T) {
	still := func(id string, index int, hash uint64) StillInfo { return StillInfo{SceneID: id, Index: index, Hash: hash} }
	far := ^uint64(0)
	stills := []StillInfo{
		still("s1", 0, 0), still("s2", 1, 0b1), still("s3", 2, 0b11), // a run of three
		still("s4", 3, far),
		still("s5", 4, far), still("s6", 5, far), // a run of two
		still("s7", 6, 0), still("s8", 7, 0), still("s9", 8, 0), still("s10", 9, 0), still("s11", 10, 0), // a run of five
	}
	got := LookAlikeIssues(stills)
	if len(got) != 2 {
		t.Fatalf("issues = %+v", got)
	}
	if got[0].Code != CodeScenesLookAlike || got[0].SceneID != "s3" || got[0].Path != "/scenes/2" || got[0].Severity != SeverityWarning || got[0].Fix == "" {
		t.Errorf("first run: %+v", got[0])
	}
	if got[1].SceneID != "s9" || !strings.Contains(got[1].Fix, "s9") || !strings.Contains(got[1].Fix, "s11") {
		t.Errorf("second run: %+v", got[1])
	}
	if !IssueLayout(CodeScenesLookAlike) || !IssueModelFixable(got[0]) {
		t.Error("scenes_look_alike belongs to the layout class")
	}
	if LookAlikeIssues(stills[:2]) != nil || LookAlikeIssues(nil) != nil {
		t.Error("fewer than three stills never make a run")
	}
}
```
- [ ] **Step 2: Run to see it fail:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestDifferenceHash|TestContactSheet|TestBuildStills|TestLookAlike)' -count=1`. Expected: FAIL to build.
- [ ] **Step 3: Implement.** First the code in `internal/storymaker/model.go`:
Old:
```go
// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```
New:
```go
// CodeScenesLookAlike is the layout finding of the stills: three or more consecutive scenes show nearly the same picture.
const CodeScenesLookAlike = "scenes_look_alike"

// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```
(If batch D's constants block `CodeCaptionOverlap` and so on sits just above, keep both blocks.) In `internal/storymaker/layout.go` add the new code to the class:
Old:
```go
	case CodeCaptionOverlap, CodePropTooSmall, CodeOffscreen, CodeActorPropOverlap, CodeSceneStatic:
```
New:
```go
	case CodeCaptionOverlap, CodePropTooSmall, CodeOffscreen, CodeActorPropOverlap, CodeSceneStatic, CodeScenesLookAlike:
```
Then create `internal/storymaker/stills.go`:

```go
package storymaker

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/bits"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	// StillsSheetColumns and StillsThumbWidth shape the contact sheet: four columns of 480 pixel wide thumbnails.
	StillsSheetColumns = 4
	StillsThumbWidth   = 480
	stillsGap          = 8

	lookAlikeMinRun = 3
	// lookAlikeMaxDistance is the most bits (of 64) in which the hashes of two neighbouring scenes may differ for the scenes to
	// count as alike. The value is tuned on the real run in task E6.
	lookAlikeMaxDistance = 6
)

// StillInfo is one captured still: the scene, its index in the story and the difference hash of the picture.
type StillInfo struct {
	SceneID string
	Index   int
	Hash    uint64
}

func decodeStill(data []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("storymaker: a still is not a PNG: %w", err)
	}
	return img, nil
}

func luma(c color.RGBA) int { return 299*int(c.R) + 587*int(c.G) + 114*int(c.B) }

// DifferenceHash is the 64 bit difference hash of a PNG: the picture is scaled to 9x8, and each bit says whether a pixel is
// brighter than its right neighbour. Pictures that look alike differ in few bits.
func DifferenceHash(data []byte) (uint64, error) {
	img, err := decodeStill(data)
	if err != nil {
		return 0, err
	}
	small := image.NewRGBA(image.Rect(0, 0, 9, 8))
	xdraw.CatmullRom.Scale(small, small.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	var hash uint64
	bit := uint(0)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if luma(small.RGBAAt(x, y)) > luma(small.RGBAAt(x+1, y)) {
				hash |= 1 << bit
			}
			bit++
		}
	}
	return hash, nil
}

// HashDistance is the number of bits in which two hashes differ.
func HashDistance(a, b uint64) int { return bits.OnesCount64(a ^ b) }

// ContactSheet lays PNG stills out in rows of up to four columns, each scaled to 480 pixels wide, on a dark ground, and
// returns the sheet as a PNG. The order is left to right, top to bottom. The thumbnails keep the aspect of the first still.
func ContactSheet(pngs [][]byte) ([]byte, error) {
	if len(pngs) == 0 {
		return nil, fmt.Errorf("storymaker: a contact sheet needs at least one still")
	}
	images := make([]image.Image, len(pngs))
	for i, data := range pngs {
		img, err := decodeStill(data)
		if err != nil {
			return nil, fmt.Errorf("still %d: %w", i+1, err)
		}
		images[i] = img
	}
	first := images[0].Bounds()
	thumbH := max(1, StillsThumbWidth*first.Dy()/max(first.Dx(), 1))
	cols := min(StillsSheetColumns, len(images))
	rows := (len(images) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, cols*StillsThumbWidth+(cols+1)*stillsGap, rows*thumbH+(rows+1)*stillsGap))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{C: color.RGBA{R: 24, G: 24, B: 28, A: 255}}, image.Point{}, draw.Src)
	for i, img := range images {
		x := stillsGap + (i%cols)*(StillsThumbWidth+stillsGap)
		y := stillsGap + (i/cols)*(thumbH+stillsGap)
		xdraw.CatmullRom.Scale(sheet, image.Rect(x, y, x+StillsThumbWidth, y+thumbH), img, img.Bounds(), xdraw.Over, nil)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, sheet); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// BuildStills hashes every still and lays them out as a contact sheet. The three lists have the same length and the same
// order; the stills are PNG bytes.
func BuildStills(sceneIDs []string, indexes []int, pngs [][]byte) (sheet []byte, stills []StillInfo, err error) {
	if len(sceneIDs) != len(pngs) || len(indexes) != len(pngs) {
		return nil, nil, fmt.Errorf("storymaker: %d scene ids, %d indexes and %d stills do not match", len(sceneIDs), len(indexes), len(pngs))
	}
	for i, data := range pngs {
		hash, err := DifferenceHash(data)
		if err != nil {
			return nil, nil, fmt.Errorf("still of scene %s: %w", sceneIDs[i], err)
		}
		stills = append(stills, StillInfo{SceneID: sceneIDs[i], Index: indexes[i], Hash: hash})
	}
	sheet, err = ContactSheet(pngs)
	if err != nil {
		return nil, nil, err
	}
	return sheet, stills, nil
}

// LookAlikeIssues finds runs of three or more consecutive stills whose neighbours differ in at most lookAlikeMaxDistance bits
// and reports each run once, at its third scene: that is the one to change. The fix asks for another environment, another
// framing and the actor on the other side, at every second scene of the run.
func LookAlikeIssues(stills []StillInfo) []Issue {
	var out []Issue
	start := 0
	flush := func(end int) {
		if end-start < lookAlikeMinRun {
			return
		}
		run := stills[start:end]
		ids := make([]string, len(run))
		for i, s := range run {
			ids[i] = s.SceneID
		}
		var change []string
		for i := 2; i < len(run); i += 2 {
			change = append(change, run[i].SceneID)
		}
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: run[2].SceneID, Path: jp("scenes", run[2].Index), Code: CodeScenesLookAlike,
			Message: fmt.Sprintf("Scenes %s look alike: the same stage and nearly the same picture.", strings.Join(ids, ", ")),
			Fix: fmt.Sprintf(`Change scene %s so it differs from its neighbours: another environment with "transition":"wipe", a framing such as {"camera":{"framing":"medium"}}, and the actor on the other side ("at":"right" or "left") with the whole actors list.`,
				strings.Join(change, " and ")),
		})
	}
	for i := 1; i <= len(stills); i++ {
		if i == len(stills) || HashDistance(stills[i-1].Hash, stills[i].Hash) > lookAlikeMaxDistance {
			flush(i)
			start = i
		}
	}
	return out
}
```
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestDifferenceHash|TestContactSheet|TestBuildStills|TestLookAlike)' -count=1 -v`. Expected: PASS. In `TestDifferenceHash...` a 2x2 corner patch must stay within the threshold: if CatmullRom's overshoot makes the patch flip more than 6 bits, print `HashDistance(a, patched)` and decide: the starting threshold of 6 is a guess that task E6 replaces, so raise `lookAlikeMaxDistance` only if the real frames of E6 say so, and weaken the patch in this test (1x1) otherwise.
- [ ] **Step 5: Commit:** `gofmt -w internal/storymaker/stills.go internal/storymaker/stills_test.go internal/storymaker/model.go internal/storymaker/layout.go`; `git add` the five files; message `feat(storymaker): difference hashes, contact sheet and the look-alike finding`.

<!-- group E2 -->

### Task E2: The summary carries the stills; the check uses them
**Batch:** E  **Depends on:** E1
**Files:**
Modify `internal/storymaker/studio.go`
Modify `internal/storymaker/studio_check.go`
Test `internal/storymaker/studio_stills_check_test.go`

- [ ] **Step 1: Impact:** `CompileSummary`, `runCheck`. Report the callers.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/studio.go`: the fields (after `LayoutError string`, which batch D added to `CompileSummary`).
Old:
```go
	LayoutError      string
}
```
New:
```go
	LayoutError      string
	// Stills are the hashes of the captured pictures, one per measured scene. Sheet is their contact sheet as a PNG, until the
	// job saves it; the Studio does not keep it.
	Stills []StillInfo
	Sheet  []byte
}
```
**Edit** `internal/storymaker/studio_check.go`: in `runCheck`, after the layout findings line that batch D added:
Old:
```go
	found = append(found, LayoutIssues(summary.Layout, LayoutOptions{CaptionRule: story.Style.Compose == "v2" && story.Style.Captions})...)
```
New:
```go
	found = append(found, LayoutIssues(summary.Layout, LayoutOptions{CaptionRule: story.Style.Compose == "v2" && story.Style.Captions})...)
	found = append(found, LookAlikeIssues(summary.Stills)...)
```
- [ ] **Step 3: Test.** Create `internal/storymaker/studio_stills_check_test.go`:

{% raw %}
```go
package storymaker

import (
	"context"
	"testing"
)

func TestCheckReportsScenesThatLookAlike(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{
			Available: true, DurationMS: 8000, StoryboardScenes: 3,
			Stills: []StillInfo{{SceneID: "s1", Index: 0, Hash: 0}, {SceneID: "s2", Index: 1, Hash: 1}, {SceneID: "s3", Index: 2, Hash: 3}},
		}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, is := range report.Issues {
		if is.Code == CodeScenesLookAlike && is.SceneID == "s3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues = %+v", report.Issues)
	}
	outcome, _ := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	if outcome == OutcomeIncomplete {
		t.Fatalf("a look-alike finding must not block: %s", outcome)
	}
}
```
{% endraw %}
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '(TestCheck|TestStudioCheck|TestDecideOutcome|TestLookAlike)' -count=1`. Expected: PASS.
- [ ] **Step 5: Commit:** `gofmt -w` the touched files; `git add internal/storymaker/studio.go internal/storymaker/studio_check.go internal/storymaker/studio_stills_check_test.go`; message `feat(storymaker): the check reports scenes that look alike`.

<!-- group E3 -->

### Task E3: Take the stills in the compile run
**Batch:** E  **Depends on:** E1, E2
**Files:**
Modify `internal/storymaker/render/render.go`
Modify `internal/storymaker/render/compilecheck.go`
Create `internal/storymaker/render/compilestills.go`
Test `internal/storymaker/render/compilestills_test.go`

With `Settings.Stills` the compile run keeps its page open after the report: transport screenshot, report answered with `start`, one `Capture` per measured scene at its hold frame, then the listener shuts down as before. Without `Stills` nothing changes (`TestCompileCheckStopsAfterReport` still holds).

- [ ] **Step 1: Impact:** `Runner.CompileCheck`, `compileInBrowser`, `Settings`. `Settings` is used by exports; the new field is false there.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/render/render.go`: the setting.
Old:
```go
	FreeMemoryMB       int64 // available memory in MiB; 0 means unknown, no memory bound
}
```
New:
```go
	FreeMemoryMB       int64 // available memory in MiB; 0 means unknown, no memory bound
	// Stills makes a compile check take one still per measured scene in the same browser run (only CompileCheck reads it).
	Stills bool
}
```
**Edit** `internal/storymaker/render/compilecheck.go`: the limit.
Old:
```go
	checkCtx, cancel := context.WithTimeout(ctx, compileCheckLimit)
```
New:
```go
	limit := compileCheckLimit
	if st.Stills {
		limit *= stillsLimitFactor
	}
	checkCtx, cancel := context.WithTimeout(ctx, limit)
```
**Edit** the transport and the report answer.
Old:
```go
PageParams{Width: width, Height: height, FPS: fps, Transport: DefaultTransport, FrameLimit: frameLimit,
```
New:
```go
PageParams{Width: width, Height: height, FPS: fps, Transport: compileTransport, FrameLimit: frameLimit,
```
and before the `reports := make(chan []byte, 1)` line add:
```go
	// With stills the page stays open and Go drives its screenshots; without them the report ends the run.
	compileTransport, reportAnswer := DefaultTransport, ReportAnswer{Error: compileOnlyAnswer, Message: "This page only compiles the story."}
	if st.Stills {
		compileTransport, reportAnswer = TransportScreenshot, ReportAnswer{Start: true}
	}
```
Old:
```go
				return ReportAnswer{Error: compileOnlyAnswer, Message: "This page only compiles the story."}
```
New:
```go
				return reportAnswer
```
**Edit** the report case: the stills are taken before the listener shuts down.
Old:
```go
			summary := summarizeCompileReport(body, want, frameLimit)
			if summary.LayoutError != "" {
				r.deps.Logger.Warn("[Storymaker] The layout measurement of the compile check failed", "issue", summary.LayoutError)
			}
			// Shutdown lets the handler finish, so the page reads its answer. Close would cut it.
```
If batch D changed the order of these lines, take the block that builds `summary` and edit it as follows: after the `LayoutError` log, and before the shutdown lines, add
```go
			if st.Stills && summary.Available && !storymaker.HasErrors(summary.Issues) {
				r.captureStills(checkCtx, page, &summary, fps, frameLimit)
			}
```
If batch D has not landed, the case still reads `return summarizeCompileReport(body, want, frameLimit), nil` after the shutdown: restructure it to build `summary` first, add the lines above, then shut the listener down and return `summary`. Add the constant to `compilecheck.go`:
```go
// stillsLimitFactor widens compileCheckLimit when the run also takes stills.
const stillsLimitFactor = 3
```
Create `internal/storymaker/render/compilestills.go`:

```go
package render

import (
	"context"
	"math"
	"time"

	"aurago/internal/storymaker"
)

// stillsReadyLimit bounds the wait for the page to become ready for screenshots after the report.
const stillsReadyLimit = 60 * time.Second

// stillFrames picks, for each measured scene, the frame to capture: the hold frame of its layout block, inside the scene and
// inside the frames the page offers (frameLimit is the export limit, 0 for none). A scene that is not in the summary, or whose
// hold frame lies outside, gets no still.
func stillFrames(summary storymaker.CompileSummary, fps, frameLimit int) (frames []int, ids []string, indexes []int) {
	byID := map[string]storymaker.CompileScene{}
	for _, sc := range summary.Scenes {
		byID[sc.ID] = sc
	}
	limit := int(math.Round(float64(summary.DurationMS) * float64(fps) / 1000))
	if frameLimit > 0 {
		limit = min(limit, frameLimit)
	}
	for _, block := range summary.Layout {
		sc, ok := byID[block.ID]
		if !ok {
			continue
		}
		start := int(math.Round(float64(sc.StartMS) * float64(fps) / 1000))
		end := int(math.Round(float64(sc.EndMS) * float64(fps) / 1000))
		n := min(start+int(math.Round(block.HoldS*float64(fps))), end-1)
		if n < start || n >= limit {
			continue
		}
		frames, ids, indexes = append(frames, n), append(ids, block.ID), append(indexes, block.Index)
	}
	return frames, ids, indexes
}

// captureStills takes one still per measured scene from the open page and adds the hashes and the contact sheet to the
// summary. A failure leaves the summary as it was: the stills are an extra, never a reason to fail the check.
func (r *Runner) captureStills(ctx context.Context, page browserPage, summary *storymaker.CompileSummary, fps, frameLimit int) {
	frames, ids, indexes := stillFrames(*summary, fps, frameLimit)
	if len(frames) == 0 {
		return
	}
	readyCtx, cancel := context.WithTimeout(ctx, stillsReadyLimit)
	defer cancel()
	if err := page.WaitReady(readyCtx); err != nil {
		r.deps.Logger.Warn("[Storymaker] The page was not ready for stills", "error", err)
		return
	}
	pngs := make([][]byte, 0, len(frames))
	for _, n := range frames {
		data, err := page.Capture(ctx, n)
		if err != nil {
			r.deps.Logger.Warn("[Storymaker] A still could not be captured", "frame", n, "error", err)
			return
		}
		pngs = append(pngs, data)
	}
	sheet, stills, err := storymaker.BuildStills(ids, indexes, pngs)
	if err != nil {
		r.deps.Logger.Warn("[Storymaker] The stills could not be assembled", "error", err)
		return
	}
	summary.Sheet, summary.Stills = sheet, stills
}
```
- [ ] **Step 3: Test.** Create `internal/storymaker/render/compilestills_test.go`:

{% raw %}
```go
package render

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"aurago/internal/storymaker"
)

func stillBytes(t *testing.T, seed int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 64; x++ {
			v := uint8((x*4 + seed*40) % 256)
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStillFramesPicksTheHoldFrameInsideTheScene(t *testing.T) {
	summary := storymaker.CompileSummary{
		DurationMS: 6000,
		Scenes: []storymaker.CompileScene{{ID: "s1", StartMS: 0, EndMS: 2000}, {ID: "s2", StartMS: 1800, EndMS: 6000}, {ID: "s3", StartMS: 5800, EndMS: 6000}},
		Layout: []storymaker.LayoutScene{
			{ID: "s1", Index: 0, HoldS: 1.0}, {ID: "s2", Index: 1, HoldS: 99}, {ID: "ghost", Index: 2, HoldS: 1}, {ID: "s3", Index: 3, HoldS: 0.1},
		},
	}
	frames, ids, indexes := stillFrames(summary, 30, 0)
	// s1: 30. s2: the hold is clamped to the last frame of the scene, 179. ghost: not in the summary. s3: starts at frame 174, hold 3 frames.
	if !reflect.DeepEqual(frames, []int{30, 179, 177}) || !reflect.DeepEqual(ids, []string{"s1", "s2", "s3"}) || !reflect.DeepEqual(indexes, []int{0, 1, 3}) {
		t.Fatalf("frames %v ids %v indexes %v", frames, ids, indexes)
	}
	// A page that offers only 100 frames cannot show frame 179.
	frames, ids, _ = stillFrames(summary, 30, 100)
	if !reflect.DeepEqual(frames, []int{30}) || !reflect.DeepEqual(ids, []string{"s1"}) {
		t.Fatalf("with a frame limit of 100: frames %v ids %v", frames, ids)
	}
}

func TestCompileCheckTakesStillsInTheSameRun(t *testing.T) {
	var captured []int
	page := &fakePage{crashed: make(chan struct{})}
	page.capture = func(n int) []byte {
		captured = append(captured, n)
		return stillBytes(t, n)
	}
	page.script = play(t, playOptions{frames: 90, mutateRep: func(rep map[string]any) {
		rep["layout"] = []any{map[string]any{"id": "s1", "index": 0, "hold_s": 1.0, "actors": []any{}, "props": []any{}, "texts": []any{}, "caption": nil, "missing": 0, "static": false}}
	}})
	runner, id := compileCheckRunner(t, page)
	got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits(), Stills: true})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || !reflect.DeepEqual(captured, []int{30}) {
		t.Fatalf("summary %+v, captured %v", got, captured)
	}
	if len(got.Stills) != 1 || got.Stills[0].SceneID != "s1" || len(got.Sheet) == 0 {
		t.Fatalf("stills %+v, sheet %d bytes", got.Stills, len(got.Sheet))
	}
	if _, err := png.Decode(bytes.NewReader(got.Sheet)); err != nil {
		t.Fatalf("the sheet is not a PNG: %v", err)
	}
	if !page.closed.Load() {
		t.Error("the page must be closed")
	}
}

func TestCompileCheckWithoutStillsTakesNone(t *testing.T) {
	var captured int
	page := &fakePage{crashed: make(chan struct{})}
	page.capture = func(n int) []byte { captured++; return stillBytes(t, n) }
	page.script = play(t, playOptions{frames: 90, mutateRep: func(rep map[string]any) {
		rep["layout"] = []any{map[string]any{"id": "s1", "index": 0, "hold_s": 1.0, "actors": []any{}, "props": []any{}, "texts": []any{}}}
	}})
	runner, id := compileCheckRunner(t, page)
	got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	if captured != 0 || len(got.Sheet) != 0 || len(got.Stills) != 0 || len(got.Layout) != 1 {
		t.Fatalf("captured %d, summary %+v", captured, got)
	}
}
```
{% endraw %}
In the first test `s3` starts at 5800 ms = frame 174 and ends at 6000 ms = frame 180, the hold is 0.1 s = 3 frames, so the frame is 177 (below its last frame 179). `s2` starts at 1800 ms = frame 54, ends at frame 180, so 99 s of hold clamps to frame 179. The limit of the film is 6000 ms = 180 frames, so frame 179 is inside it.
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render -run '(TestStillFrames|TestCompileCheck)' -count=1`. Expected: PASS, including `TestCompileCheckStopsAfterReport`. Then `go vet ./internal/storymaker/...`.
- [ ] **Step 5: Commit:** `gofmt -w` the touched files; `git add internal/storymaker/render/render.go internal/storymaker/render/compilecheck.go internal/storymaker/render/compilestills.go internal/storymaker/render/compilestills_test.go`; message `feat(storymaker): the compile check takes a still per scene in the same browser run`.

<!-- group E4 -->

### Task E4: Save the contact sheet and serve it
**Batch:** E  **Depends on:** E3
**Files:**
Create `internal/storymaker/review.go`
Modify `internal/storymaker/agentjob.go`
Modify `internal/server/storymaker_agent.go`
Create `internal/server/storymaker_review_handlers.go`
Modify `internal/server/storymaker_handlers.go`
Test `internal/storymaker/review_test.go`
Test `internal/server/storymaker_review_test.go`

- [ ] **Step 1: Impact:** `AgentJobInfo`, `newStorymakerAgentJob` (the `Compile:` dependency), `handleStorymaker` (the route switch). Report callers.
- [ ] **Step 2: The store.** Create `internal/storymaker/review.go`:

```go
package storymaker

import (
	"context"
	"io"
)

// ReviewSheetName is the file name of the contact sheet of a project's last agent check; its folder is review/.
const ReviewSheetName = "contact-sheet.png"

// AgentReview is the review evidence of an agent job: the sheet and how many scenes it shows.
type AgentReview struct {
	Sheet  string `json:"sheet"`
	Scenes int    `json:"scenes"`
}

// SaveReviewSheet stores the contact sheet of the latest agent check. It replaces the previous one.
func (p *ProjectStore) SaveReviewSheet(ctx context.Context, id string, data []byte) error {
	if err := checkID(id); err != nil {
		return err
	}
	_, err := p.st.WriteFile(ctx, p.file(id, "review", ReviewSheetName), data, "")
	return err
}

// OpenReviewSheet opens the stored contact sheet. A missing sheet wraps fs.ErrNotExist.
func (p *ProjectStore) OpenReviewSheet(ctx context.Context, id string) (io.ReadSeekCloser, FileInfo, error) {
	if err := checkID(id); err != nil {
		return nil, FileInfo{}, err
	}
	return p.st.Open(ctx, p.file(id, "review", ReviewSheetName))
}
```
`checkID` is what `SaveSilentBackup` uses (`agentjob.go:224-230`); if the function has another name, use the one that file uses. In `internal/storymaker/agentjob.go` add the field to `AgentJobInfo` after `Repairs` (batch C) or after `Tokens`:
Old:
```go
	Tokens          int          `json:"tokens,omitempty"`
```
New:
```go
	Tokens          int          `json:"tokens,omitempty"`
	Review          *AgentReview `json:"review,omitempty"` // the contact sheet of the latest check, when stills were taken
```
Create `internal/storymaker/review_test.go`:

```go
package storymaker

import (
	"context"
	"io"
	"testing"
)

func TestReviewSheetRoundTrips(t *testing.T) {
	ctx := context.Background()
	ps := newTestStore(t)
	project, _, _, err := ps.Create(ctx, "Review", "16:9", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ps.OpenReviewSheet(ctx, project.ID); err == nil {
		t.Fatal("no sheet yet")
	}
	if err := ps.SaveReviewSheet(ctx, project.ID, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := ps.SaveReviewSheet(ctx, project.ID, []byte("second")); err != nil {
		t.Fatal(err)
	}
	r, _, err := ps.OpenReviewSheet(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, _ := io.ReadAll(r)
	if string(data) != "second" {
		t.Fatalf("sheet = %q: a new sheet replaces the old one", data)
	}
	if err := ps.SaveReviewSheet(ctx, "../evil", []byte("x")); err == nil {
		t.Fatal("a bad id is refused")
	}
}
```
- [ ] **Step 3: The job saves it.** In `internal/server/storymaker_agent.go`:

**Edit** the dependency in `newStorymakerAgentJob`:
Old:
```go
		Compile:            s.storymakerAgentCompile(rt, cfg),
```
New:
```go
		Compile:            j.withReview(s.storymakerAgentCompile(rt, cfg)),
```
**Edit** `storymakerAgentCompile`: ask for stills.
Old:
```go
		Transport:          cfg.Storymaker.Render.Transport,
		Limits:             storymakerLimits(cfg),
	}
	return func(ctx context.Context, projectID string) (storymaker.CompileSummary, error) {
```
New:
```go
		Transport:          cfg.Storymaker.Render.Transport,
		Limits:             storymakerLimits(cfg),
		Stills:             true,
	}
	return func(ctx context.Context, projectID string) (storymaker.CompileSummary, error) {
```
**Edit** add the wrapper after `storymakerAgentCompile`:
Old:
```go
// storymakerAgentCompile runs the compile check with settings of its own: storymakerSettings fails
```
New:
```go
// withReview saves the contact sheet of a compile check as the project's review evidence, records it in the job document and
// drops it from the summary, so the Studio does not cache a picture per story version. A failed save is logged and costs the
// link only.
func (j *storymakerAgentJob) withReview(inner func(context.Context, string) (storymaker.CompileSummary, error)) func(context.Context, string) (storymaker.CompileSummary, error) {
	return func(ctx context.Context, projectID string) (storymaker.CompileSummary, error) {
		summary, err := inner(ctx, projectID)
		if err != nil || len(summary.Sheet) == 0 {
			return summary, err
		}
		if saveErr := j.rt.projects.SaveReviewSheet(ctx, projectID, summary.Sheet); saveErr != nil {
			j.logger.Warn("[Storymaker] The contact sheet could not be saved", "job_id", j.job.ID, "error", saveErr)
		} else {
			scenes := len(summary.Stills)
			j.update(func(info *storymaker.AgentJobInfo) {
				info.Review = &storymaker.AgentReview{Sheet: storymaker.ReviewSheetName, Scenes: scenes}
			})
		}
		summary.Sheet = nil
		return summary, nil
	}
}

// storymakerAgentCompile runs the compile check with settings of its own: storymakerSettings fails
```
In `newStorymakerAgentJob` the value `j` is created before `j.deps = ...` (see `j := &storymakerAgentJob{...}` earlier in the function), so `j.withReview` is available there.
- [ ] **Step 4: The route.** Create `internal/server/storymaker_review_handlers.go`:

```go
package server

import (
	"net/http"

	"aurago/internal/storymaker"
)

// handleStorymakerReviewSheet serves GET and HEAD /projects/{id}/review/contact-sheet.png: the contact sheet of the agent's
// latest check. It is an inert PNG, served with the same headers as the audio route, and not cached, because the next check
// replaces it.
func handleStorymakerReviewSheet(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeStorymakerMethodError(w, "GET, HEAD")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
	if !ok {
		return
	}
	content, info, err := acc.rt.projects.OpenReviewSheet(r.Context(), id)
	if err != nil {
		mapStorymakerError(w, err, "review_not_found")
		return
	}
	defer content.Close()
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, storymaker.ReviewSheetName, info.ModTime, content)
}
```
**Edit** `internal/server/storymaker_handlers.go`: the route, before the audio case.
Old:
```go
			case len(parts) == 5 && parts[2] == "audio":
```
New:
```go
			case len(parts) == 4 && parts[2] == "review" && parts[3] == storymaker.ReviewSheetName:
				handleStorymakerReviewSheet(s, w, r, id)
			case len(parts) == 5 && parts[2] == "audio":
```
- [ ] **Step 5: Test.** Create `internal/server/storymaker_review_test.go` with the harness of `TestStorymakerAudioRoutes` (`storymakerAPIFixture`, `createStorymakerProject`, `storymakerRequest`):

```go
package server

import (
	"context"
	"net/http"
	"testing"
)

func TestStorymakerReviewSheetRoute(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	id := createStorymakerProject(t, s, writeToken).Project.ID
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + id + "/review/contact-sheet.png"

	w := storymakerRequest(s, readToken, http.MethodGet, path, nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("no sheet yet: %d %s", w.Code, w.Body.String())
	}
	data := []byte("\x89PNG\r\n\x1a\nsheet")
	if err := rt.projects.SaveReviewSheet(context.Background(), id, data); err != nil {
		t.Fatal(err)
	}
	w = storymakerRequest(s, readToken, http.MethodGet, path, nil, nil)
	if w.Code != http.StatusOK || w.Body.String() != string(data) {
		t.Fatalf("GET = %d, %d bytes", w.Code, w.Body.Len())
	}
	for header, want := range map[string]string{
		"Content-Type": "image/png", "X-Content-Type-Options": "nosniff", "Content-Security-Policy": "sandbox; default-src 'none'", "Cache-Control": "private, no-cache",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+id+"/review/other.png", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("another file name = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodPost, path, nil, nil); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST = %d, Allow %q", w.Code, w.Header().Get("Allow"))
	}
}
```
- [ ] **Step 6: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestReviewSheet' -count=1`, `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run '(TestStorymakerReviewSheetRoute|TestStorymakerAudioRoutes|TestStorymakerAgentScripted)' -count=1 -timeout 20m`. Expected: PASS.
- [ ] **Step 7: Commit:** `gofmt -w` the touched Go files; `git add internal/storymaker/review.go internal/storymaker/review_test.go internal/storymaker/agentjob.go internal/server/storymaker_agent.go internal/server/storymaker_review_handlers.go internal/server/storymaker_review_test.go internal/server/storymaker_handlers.go`; message `feat(storymaker): save and serve the contact sheet of the agent's check`.

<!-- group E5 -->

### Task E5: The link on the job card
**Batch:** E  **Depends on:** E4
**Files:**
Modify `ui/js/desktop/apps/storymaker-api.js`
Modify `ui/js/desktop/apps/storymaker-agent.js`
Modify `ui/css/desktop-app-storymaker.css`
Modify `ui/lang/storymaker/*.json` (16 files)

- [ ] **Step 1: The address.** In `storymaker-api.js` (CRLF) the audio addresses are built in one place. Write `<scratch>/edit-E5.mjs` and run it from the worktree root:

```js
import { edit } from './crlf-edit.mjs';

// the address of the contact sheet, next to the audio addresses
edit('ui/js/desktop/apps/storymaker-api.js',
  "            musicUrl(id, key, ext) { return audioUrl(id, 'music', key, ext); },\n",
  "            musicUrl(id, key, ext) { return audioUrl(id, 'music', key, ext); },\n" +
  "            // the contact sheet of the agent's latest check; the file name is fixed by the server\n" +
  "            reviewSheetUrl(id) { return BASE + '/projects/' + encodeURIComponent(id) + '/review/contact-sheet.png'; },\n");

// the link, in the outcome card after the summary
edit('ui/js/desktop/apps/storymaker-agent.js',
  "            if (checks.length) outcomeCard.appendChild(el('ul', { class: 'stm-agent-checks' }, checks.map(checkLine)));\n",
  "            if (job.agent && job.agent.review && job.agent.review.sheet && inst.projectId) {\n" +
  "                outcomeCard.appendChild(el('a', {\n" +
  "                    class: 'stm-agent-review', href: inst.api.reviewSheetUrl(inst.projectId), target: '_blank', rel: 'noopener',\n" +
  "                    dataset: { stm: 'agent-review' }, text: t('storymaker.agent.review_link')\n" +
  "                }));\n" +
  "            }\n" +
  "            if (checks.length) outcomeCard.appendChild(el('ul', { class: 'stm-agent-checks' }, checks.map(checkLine)));\n");

// a link looks like one
edit('ui/css/desktop-app-storymaker.css',
  ".stm-agent-check code { font-family: var(--stm-mono); }\n",
  ".stm-agent-check code { font-family: var(--stm-mono); }\n.stm-agent-review { display: inline-block; font-size: 12px; color: var(--ds-color-accent, currentColor); text-decoration: underline; }\n");
console.log('E5 edited');
```
`el` is the module's own element helper; if it does not accept `href`, `target` and `rel` as attributes, read how `el` handles attributes at the top of `storymaker-agent.js` or in `storymaker-core.js` and set them with `setAttribute` through the helper's own mechanism (never `innerHTML`). The guard test requires every class literal to start with `stm-`.
- [ ] **Step 2: The sentence in 16 languages.** Write `<scratch>/add-E5-keys.mjs`, run it from the worktree root; it inserts one line after the line with `"storymaker.agent.check_unknown"` in each locale file:

```js
import fs from 'node:fs';
const AFTER = '"storymaker.agent.check_unknown"';
const KEY = 'storymaker.agent.review_link';
const T = {
  en: 'View the contact sheet of the last check',
  de: 'Kontaktbogen der letzten Prüfung ansehen',
  cs: 'Zobrazit kontaktní arch poslední kontroly',
  da: 'Se kontaktarket fra den sidste kontrol',
  el: 'Προβολή του φύλλου επαφής του τελευταίου ελέγχου',
  es: 'Ver la hoja de contactos de la última comprobación',
  fr: 'Voir la planche-contact du dernier contrôle',
  hi: 'पिछली जाँच की कॉन्टैक्ट शीट देखें',
  it: "Vedi il provino dell'ultimo controllo",
  ja: '直近のチェックのコンタクトシートを見る',
  nl: 'Bekijk het contactvel van de laatste controle',
  no: 'Se kontaktarket fra den siste kontrollen',
  pl: 'Zobacz arkusz kontaktowy ostatniej kontroli',
  pt: 'Ver a folha de contactos da última verificação',
  sv: 'Visa kontaktarket från den senaste kontrollen',
  zh: '查看最近一次检查的缩略图总览',
};
for (const [lang, text] of Object.entries(T)) {
  const file = `ui/lang/storymaker/${lang}.json`;
  const source = fs.readFileSync(file, 'utf8');
  if (source.includes(`"${KEY}"`)) throw new Error(`${file} already has ${KEY}`);
  const eol = source.includes('\r\n') ? '\r\n' : '\n';
  const lines = source.split(eol);
  const at = lines.findIndex((line) => line.includes(AFTER));
  if (at < 0) throw new Error(`${file} lacks ${AFTER}`);
  lines.splice(at + 1, 0, `  "${KEY}": ${JSON.stringify(text)},`);
  fs.writeFileSync(file, lines.join(eol));
  JSON.parse(fs.readFileSync(file, 'utf8'));
}
console.log('added', KEY, 'to', Object.keys(T).length, 'files');
```
Expected output: `added storymaker.agent.review_link to 16 files`. `check_unknown` is followed by other keys in the file, so the comma is right. If `check_unknown` is the last line before `}` in a locale, put the new line before it instead.
- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(storymaker|TestShippedWebAssetsLoadNothingExternal)' -count=1 -timeout 20m` and `npm run check:ui`. Expected: PASS. Rebuild with `npm run build:ui` if `check:ui` reports a stale bundle, and commit the rebuilt bundle with this task.
- [ ] **Step 4: Commit:** `git add ui/js/desktop/apps/storymaker-api.js ui/js/desktop/apps/storymaker-agent.js ui/css/desktop-app-storymaker.css ui/lang/storymaker` plus any rebuilt bundle; message `feat(storymaker): job card links to the contact sheet`.

<!-- group E6 -->

### Task E6: The real run in a real browser: stills, thresholds, a look at the sheet
**Batch:** E  **Depends on:** E3
**Files:**
Create `internal/storymaker/render/stills_real_test.go`
Possibly modify `internal/storymaker/stills.go`, `internal/storymaker/layout.go`, `ui/js/storymaker/stage.js`, `ui/js/storymaker/compose.js`

This is the acceptance that unit tests cannot give: the real firewall run (7 scenes, all alike) and the varied story go through the real page, the caption rule is held against measured boxes, and a contact sheet exists to look at. It also fixes the constants that were chosen from arithmetic: `V2_FEET_Y` (74), `HERO_HEIGHT` (0.24), the layout thresholds and `lookAlikeMaxDistance` (6).

- [ ] **Step 1: Create `internal/storymaker/render/stills_real_test.go`** (it uses the fixture of `TestCompileCheckBrowser`, `newRealFixture`, which needs Chrome and FFmpeg):

```go
package render

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/storymaker"
)

// TestCompileCheckStillsOfTheRealRuns renders the compile check with stills for three stories and checks the geometry that
// batches B and D promise: the real firewall run without composition v2 has the caption over the feet, the same run with v2 has
// the feet above the caption, and the varied story has no run of look-alike scenes. The contact sheets go to
// AURAGO_STILLS_OUT when it is set, so they can be looked at.
func TestCompileCheckStillsOfTheRealRuns(t *testing.T) {
	chrome := browserSmoke(t)
	fx := newRealFixture(t, chrome)
	fx.rebuild(t)
	out := os.Getenv("AURAGO_STILLS_OUT")
	ctx := context.Background()

	load := func(name string, compose string) []byte {
		raw, err := os.ReadFile(filepath.Join("..", "testdata", "stories", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		delete(doc, "voice")
		delete(doc, "music")
		style, _ := doc["style"].(map[string]any)
		if compose != "" {
			style["compose"] = compose
		} else {
			delete(style, "compose")
		}
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, tc := range []struct{ name, story, compose string }{
		{"firewall-v1", "agent-firewall-run", ""},
		{"firewall-v2", "agent-firewall-run", "v2"},
		{"varied-v2", "agent-varied", "v2"},
	} {
		if _, err := fx.projects.SaveStory(ctx, fx.projectID, load(tc.story, tc.compose), ""); err != nil {
			t.Fatal(err)
		}
		settings := fx.settings
		settings.Stills = true
		got, err := fx.runner.CompileCheck(ctx, fx.projectID, settings)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || got.LayoutError != "" || len(got.Layout) != 7 || len(got.Stills) != 7 || len(got.Sheet) == 0 {
			t.Fatalf("%s: available %v, layout error %q, %d layout blocks, %d stills, sheet %d bytes, issues %+v", tc.name, got.Available, got.LayoutError, len(got.Layout), len(got.Stills), len(got.Sheet), got.Issues)
		}
		if out != "" {
			if err := os.WriteFile(filepath.Join(out, tc.name+".png"), got.Sheet, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		for i := 1; i < len(got.Stills); i++ {
			t.Logf("%s: scenes %s and %s differ by %d of 64 bits", tc.name, got.Stills[i-1].SceneID, got.Stills[i].SceneID, storymaker.HashDistance(got.Stills[i-1].Hash, got.Stills[i].Hash))
		}
		issues := storymaker.LayoutIssues(got.Layout, storymaker.LayoutOptions{CaptionRule: true})
		overlaps, others := 0, 0
		for _, is := range issues {
			if is.Code == storymaker.CodeCaptionOverlap {
				overlaps++
			} else {
				others++
				t.Logf("%s: %s in %s: %s", tc.name, is.Code, is.SceneID, is.Message)
			}
		}
		lookAlike := storymaker.LookAlikeIssues(got.Stills)
		t.Logf("%s: %d caption overlaps, %d other layout findings, %d look-alike runs", tc.name, overlaps, others, len(lookAlike))
		switch tc.name {
		case "firewall-v1":
			if overlaps == 0 {
				t.Errorf("a story without composition v2 has its feet in the caption band: expected caption_overlap findings")
			}
			if len(lookAlike) == 0 {
				t.Errorf("the real run has seven alike scenes: expected scenes_look_alike")
			}
		case "firewall-v2":
			if overlaps != 0 {
				t.Errorf("composition v2 must keep feet and prop bases out of the caption band, got %d overlaps", overlaps)
			}
		case "varied-v2":
			if overlaps != 0 {
				t.Errorf("the varied story under v2 has %d caption overlaps", overlaps)
			}
			if len(lookAlike) != 0 {
				t.Errorf("the varied story must not be reported as look-alike: %+v", lookAlike)
			}
		}
	}
}
```
- [ ] **Step 2: Run it with the stills folder** (Chrome starts only from test binaries under `D:\tmp\go-build`, the shell sandbox off, one foreground command; the index, "Setup"): create the folder once with `mkdir -p D:/tmp/stills-out`, then `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STILLS_OUT='D:\tmp\stills-out' AURAGO_TEST_FFMPEG_DIR='<local ffmpeg bin>' GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render -run '^TestCompileCheckStillsOfTheRealRuns$' -count=1 -v -timeout 20m`. It writes `firewall-v1.png`, `firewall-v2.png` and `varied-v2.png` and logs the distances.
- [ ] **Step 3: Look at the sheets.** Open the three PNGs with the Read tool (they are images). Check, and write the answer down in the commit message:
  - `firewall-v1`: the caption covers the feet of the actor in the scenes (the defect the work removes).
  - `firewall-v2`: the feet stand clearly above the caption; the actors are about as big as in v1; the first prop of each scene is large and readable; the camera push-in does not cut a prop off at the frame edge.
  - `varied-v2`: scenes look different (three environments), the second person appears, and no scene is cut off.
- [ ] **Step 4: Tune what the pictures or the numbers say.** The distances of the log decide `lookAlikeMaxDistance`: it must be at least the largest neighbour distance inside the real run's longest alike stretch (so that run is flagged), and below the smallest neighbour distance of the varied story (so it is not). Edit the constant in `internal/storymaker/stills.go`. If a still shows feet within 1 percent of the caption, lower `V2_FEET_Y` in `ui/js/storymaker/stage.js` by 2 and the matching `0.74` values in `scripts/storymaker-tests/29-compose-v2.mjs` and `30-layout.mjs`; if the hero prop looks too big or too small, change `HERO_HEIGHT` in `compose.js` (and `0.24` in the same tests). After any JS change run `npm run build:ui` and `node scripts/test-storymaker-player.mjs`. A threshold of `layout.go` that reports a false finding on `varied-v2` is changed there, with a sentence in the commit message about the measured value. Run Step 2 again after each change until the three checks pass. If no threshold separates the real run from the varied story (the real run's neighbour distances are not clearly smaller than the varied story's), the varied fixture is the thing to change, not the lint: give it a different environment in every scene that sits between two scenes of the same environment, rerun the fixture tests of batch A, and measure again.
- [ ] **Step 5: Commit** the test and each tuned file in its own commit when more than one area changed: `git add internal/storymaker/render/stills_real_test.go`, message `test(storymaker): real-browser acceptance of stills, caption rule and look-alike scenes`; tuning commits `fix(storymaker): tune <constant> on the real run` with the measured numbers in the body.

<!-- group E7 -->

### Task E7: Documents, the spec, and the acceptance recipe
**Batch:** E  **Depends on:** E6
**Files:**
Modify `internal/storymaker/AGENTS.md`
Modify `internal/storymaker/render/AGENTS.md`
Modify `docs/superpowers/specs/2026-11-09-storymaker-quality-design.md` (git-ignored: commit with `-f` through the temp index)
Modify `documentation/` pages that describe the agent's open checks, if `grep -rn "layout_unverified" documentation` finds any

- [ ] **Step 1: The package documents.** In `internal/storymaker/AGENTS.md` add after the layout bullet of batch D: ``- With `Settings.Stills` the compile run keeps its page open (transport `screenshot`, report answered with `start`), captures the hold frame of every measured scene (`compilestills.go`), and returns `Stills` (difference hashes) and `Sheet` (a 4 column contact sheet of 480 px thumbnails). `LookAlikeIssues` reports runs of three or more consecutive scenes within `lookAlikeMaxDistance` bits as `scenes_look_alike` (layout class). The agent job saves the sheet to `review/contact-sheet.png` (`ProjectStore.SaveReviewSheet`), records `agent.review` in the job document and serves it at `/review/contact-sheet.png`.`` In `internal/storymaker/render/AGENTS.md` add under the compile check: ``A compile check with `Settings.Stills` runs up to 3 times `compileCheckLimit`, uses the screenshot transport and answers the report with `start`; without it nothing changes.``
- [ ] **Step 2: The spec.** Correct `docs/superpowers/specs/2026-11-09-storymaker-quality-design.md` to what was built: 1.2 says the new lints are a class beside `IssueModelFixable` (`IssueCraft`, `issueNeverBlocks`) and apply to create jobs only; `cast_unused` is fixed through `patch_scene` only; 1.3 says `Beat.scale` is 0.5 to 2 and the caption-safe feet line is 74 (not 76); 1.4 covers narration and the text of show and say beats; 1.5 says the gate is "not read-only" because no config key exists; 2.1 says the layout runs after `prepare()` in a worker message behind a boot flag, the caption rule applies to `compose` `v2` only, and `ready` means an empty open-check list; 2.2 says the stills are taken in the same run; 2.3 says the vision review is dropped because the policy refuses local images for Agnes AI, with the file names. Add a section "Built" with the batch commits.
- [ ] **Step 3: The acceptance recipe for the user** goes into the index (done there). Read the index recipe once more against what was built: the request text, the places to read the result, and the criteria.
- [ ] **Step 4: Commit:** `git add internal/storymaker/AGENTS.md internal/storymaker/render/AGENTS.md` and the documentation pages; message `docs(storymaker): stills, contact sheet and look-alike scenes`. Commit the spec through the temp index, message `docs(storymaker): the quality design as built`.

## Batch E end

Run the batch matrix of the index from the batch E worktree, with the browser lines (`TestCompileCheckBrowser` and `TestCompileCheckStillsOfTheRealRuns`). Batch E also runs `./internal/agent`, `./internal/prompts` and `./internal/security` (the matrix says why: the tool schema, the manual and the new route).
