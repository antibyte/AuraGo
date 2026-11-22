# Part A: Issue codes, classification, layout text checks, locale keys

> Generated from the drafted and tested commits e24358452..e84044a7a. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task A1: add the measured issue codes and classify them

**Files:**
- Modify: `internal/storymaker/issues.go`
- Create: `internal/storymaker/issues_measured_test.go`
- Modify: `internal/storymaker/model.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/issues_measured_test.go`:

```go
package storymaker

import "testing"

// The hard measured codes are model-fixable warnings that block an outcome; the advice codes never do. None of them is an
// error, so a person's export is never refused for them.
func TestMeasuredCodesClassification(t *testing.T) {
	for _, code := range []string{CodeTextOffframe, CodeFlash} {
		is := Issue{Severity: SeverityWarning, Code: code, SceneID: "s1"}
		if !IssueModelFixable(is) {
			t.Errorf("%s must be model-fixable", code)
		}
		if issueNeverBlocks(code) || !HasBlockingIssue([]Issue{is}) {
			t.Errorf("%s must block while left", code)
		}
		if IssueAdvisory(code) {
			t.Errorf("%s is not advice", code)
		}
		if HasErrors([]Issue{is}) {
			t.Errorf("%s must not be an error", code)
		}
		outcome, checks := DecideOutcome(OutcomeInput{
			Progress: StudioProgress{ScenesTotal: 2, ScenesDone: 2},
			Check:    CheckReport{Compile: "ok", StoryboardScenes: 2, LayoutScenes: 2, Issues: []Issue{is}},
		})
		if outcome != OutcomeIncomplete {
			t.Errorf("%s left: outcome = %q, want incomplete", code, outcome)
		}
		if len(checks) != 1 || checks[0].Code != code || checks[0].SceneID != "s1" {
			t.Errorf("%s left: checks = %+v", code, checks)
		}
	}
	for _, code := range []string{CodeTextContrast, CodeTextUnsafe, CodeSlowHook} {
		is := Issue{Severity: SeverityWarning, Code: code, SceneID: "s1"}
		if !IssueAdvisory(code) {
			t.Errorf("%s must be advice", code)
		}
		if HasErrors([]Issue{is}) {
			t.Errorf("%s must not be an error", code)
		}
	}
	for _, code := range []string{CodeTextOffframe, CodeTextUnsafe, CodeFlash} {
		if IssueLayout(code) {
			t.Errorf("%s is not a layout code", code)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run TestMeasuredCodesClassification -count=1`
Expected: internal\storymaker\issues_measured_test.go:8:32: undefined: CodeTextOffframe

- [ ] **Step 3: Implement**

In `internal/storymaker/issues.go`, replace:

```go
		return true
	}
	switch i.Code {
	case CodeSceneUnfilled:
		return true
	case CodeCompileFailed:
		return i.SceneID != ""
```

with:

```go
		return true
	}
	switch i.Code {
	case CodeSceneUnfilled, CodeTextOffframe, CodeFlash:
		return true
	case CodeCompileFailed:
		return i.SceneID != ""
```

In `internal/storymaker/issues.go`, replace:

```go
func IssueAdvisory(code string) bool {
	switch code {
	case CodeFirstFrameEmpty, CodeDeadStretch, CodeFadeBetweenWorlds, CodeCueLate, CodeWordsTell,
		CodeLineUnread, CodeTextTooSmall, CodeEntryMonotony, CodeOverBudget, CodeSpotOverlap:
		return true
	}
	return false
```

with:

```go
func IssueAdvisory(code string) bool {
	switch code {
	case CodeFirstFrameEmpty, CodeDeadStretch, CodeFadeBetweenWorlds, CodeCueLate, CodeWordsTell,
		CodeLineUnread, CodeTextTooSmall, CodeEntryMonotony, CodeOverBudget, CodeSpotOverlap,
		CodeTextContrast, CodeTextUnsafe, CodeSlowHook:
		return true
	}
	return false
```

In `internal/storymaker/model.go`, replace:

```go
	CodeSceneStatic      = "scene_static"
)

// CodeScenesLookAlike is the layout finding of the stills: three or more consecutive scenes show nearly the same picture.
const CodeScenesLookAlike = "scenes_look_alike"
```

with:

```go
	CodeSceneStatic      = "scene_static"
)

// Issue codes of the measured picture checks (layout boxes, stills, frame probe, player lint). The check emits them, never
// Validate. The two hard codes are model-fixable warnings that block an outcome while left; the others are advice or an open
// check.
const (
	CodeTextOffframe    = "text_offframe"    // warning, hard: a text sticks out of the frame (layout)
	CodeTextUnsafe      = "text_unsafe"      // warning, advice: a text or the caption lies in the aspect's safe zone (layout)
	CodeTextContrast    = "text_contrast"    // warning, advice: contrast below 3:1 on the still
	CodeFlash           = "flash"            // warning, hard: more than 3 flashes in a second (page probe)
	CodeFlashUnverified = "flash_unverified" // warning, open check: the probe did not reach every frame
	CodeSlowHook        = "slow_hook"        // warning, advice: scene 1 is silent or lands after 0.5 s (player lint)
)

// CodeScenesLookAlike is the layout finding of the stills: three or more consecutive scenes show nearly the same picture.
const CodeScenesLookAlike = "scenes_look_alike"
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run TestMeasuredCodesClassification -count=1`
Expected: --- PASS: TestMeasuredCodesClassification

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/issues.go internal/storymaker/issues_measured_test.go internal/storymaker/model.go
git commit -m "storymaker: add the measured issue codes and classify them"
```

### Task A2: safe zone per aspect and the aspect layout option

**Files:**
- Modify: `internal/storymaker/layout.go`
- Create: `internal/storymaker/layout_zone_test.go`
- Modify: `internal/storymaker/studio_check.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/layout_zone_test.go`:

```go
package storymaker

import "testing"

// The 9:16 zone is the union of the platform overlays; every other aspect keeps the stage's 5 percent margin.
func TestSafeZone(t *testing.T) {
	if got, want := SafeZone("9:16"), (LayoutZone{Top: 0.115, Right: 0.130, Bottom: 0.219, Left: 0.056}); got != want {
		t.Errorf("9:16 zone = %+v, want %+v", got, want)
	}
	margin := LayoutZone{Top: 0.05, Right: 0.05, Bottom: 0.05, Left: 0.05}
	for _, aspect := range []string{"16:9", "1:1", "", "4:5"} {
		if got := SafeZone(aspect); got != margin {
			t.Errorf("%q zone = %+v, want %+v", aspect, got, margin)
		}
	}
}

// The check hands the story's aspect to the layout rules.
func TestStoryLayoutOptionsCarryTheAspect(t *testing.T) {
	var s Story
	s.Format.Aspect = "9:16"
	if got := StoryLayoutOptions(&s).Aspect; got != "9:16" {
		t.Errorf("Aspect = %q, want 9:16", got)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run 'TestSafeZone|TestStoryLayoutOptionsCarryTheAspect' -count=1`
Expected: internal\storymaker\layout_zone_test.go:7:18: undefined: SafeZone

- [ ] **Step 3: Implement**

In `internal/storymaker/layout.go`, replace:

```go
	layoutHeroTarget    = 0.20 // the share the fix text asks for
	layoutOutsideShare  = 0.25 // actors and props: this share of the box outside the frame is too much
	layoutTextOutside   = 0.01 // texts must lie inside the frame
	layoutOverlapShare  = 0.35 // of the smaller box
	layoutScaleMax      = 2.0  // the validated range of a beat scale
)
```

with:

```go
	layoutHeroTarget    = 0.20 // the share the fix text asks for
	layoutOutsideShare  = 0.25 // actors and props: this share of the box outside the frame is too much
	layoutTextOutside   = 0.01 // texts must lie inside the frame
	layoutZoneShare     = 0.02 // texts and the caption: this share of the box inside the safe zone is too much
	layoutOverlapShare  = 0.35 // of the smaller box
	layoutScaleMax      = 2.0  // the validated range of a beat scale
)
```

In `internal/storymaker/layout.go`, replace:

```go
// true) whose camera the player does not make caption-safe: a close framing, a camera target or a camera beat. Their feet and
// bases may stand in the caption band, so the caption rule skips them. A medium framing is adjusted by the player and is not
// exempt.
type LayoutOptions struct {
	CaptionRule   bool
	CaptionExempt map[string]bool
}

// IssueLayout says whether a code is a finding of the layout run. Like the craft codes they are model-fixable and never
```

with:

```go
// true) whose camera the player does not make caption-safe: a close framing, a camera target or a camera beat. Their feet and
// bases may stand in the caption band, so the caption rule skips them. A medium framing is adjusted by the player and is not
// exempt.
//
// Aspect is the story's format aspect; it picks the safe zone of the text rules.
type LayoutOptions struct {
	CaptionRule   bool
	CaptionExempt map[string]bool
	Aspect        string
}

// LayoutZone is the safe zone of an aspect: fractions of the frame measured from each edge.
type LayoutZone struct{ Top, Right, Bottom, Left float64 }

// SafeZone returns the zone of an aspect; any value other than "9:16" gets the 5 % margin. The 9:16 numbers are the union of
// the TikTok, Reels and Shorts overlays. The player's caption layout (render.js captionLayout) repeats them.
func SafeZone(aspect string) LayoutZone {
	if aspect == "9:16" {
		return LayoutZone{Top: 0.115, Right: 0.130, Bottom: 0.219, Left: 0.056}
	}
	return LayoutZone{Top: 0.05, Right: 0.05, Bottom: 0.05, Left: 0.05}
}

// IssueLayout says whether a code is a finding of the layout run. Like the craft codes they are model-fixable and never
```

In `internal/storymaker/studio_check.go`, replace:

```go
	return LayoutOptions{
		CaptionRule:   story.Style.Compose == "v2" && story.Style.Captions && story.Format.Aspect != "9:16",
		CaptionExempt: captionExemptScenes(story),
	}
}
```

with:

```go
	return LayoutOptions{
		CaptionRule:   story.Style.Compose == "v2" && story.Style.Captions && story.Format.Aspect != "9:16",
		CaptionExempt: captionExemptScenes(story),
		Aspect:        story.Format.Aspect,
	}
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run 'TestSafeZone|TestStoryLayoutOptionsCarryTheAspect' -count=1`
Expected: --- PASS: TestSafeZone, --- PASS: TestStoryLayoutOptionsCarryTheAspect

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/layout.go internal/storymaker/layout_zone_test.go internal/storymaker/studio_check.go
git commit -m "storymaker: safe zone per aspect and the aspect layout option"
```

### Task A3: a text outside the frame is text_offframe

**Files:**
- Modify: `internal/storymaker/layout.go`
- Modify: `internal/storymaker/layout_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/layout_test.go`, replace:

```go
	}
	sc.Texts = []LayoutText{{ID: "t", Box: LayoutBox{0.7, 0.2, 0.35, 0.1}}} // sticks out of the frame
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	want := []string{CodeOffscreen, CodeOffscreen, CodeActorPropOverlap}
	slices.Sort(want)
	have := layoutCodes(got)
	slices.Sort(have)
```

with:

```go
	}
	sc.Texts = []LayoutText{{ID: "t", Box: LayoutBox{0.7, 0.2, 0.35, 0.1}}} // sticks out of the frame
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	want := []string{CodeOffscreen, CodeTextOffframe, CodeActorPropOverlap} // the text has its own code, the prop keeps offscreen
	slices.Sort(want)
	have := layoutCodes(got)
	slices.Sort(have)
```

In `internal/storymaker/layout_test.go`, replace:

```go
	}
}

func TestLayoutIssuesStaticSceneAndMissingArt(t *testing.T) {
	sc := layoutScene("s4", 3)
	sc.Static = true
```

with:

```go
	}
}

// A text that sticks out of the frame by more than 1 percent is text_offframe, with the text and the scene named; a text that
// is inside, or out by a hair, is not.
func TestLayoutIssuesTextOffframe(t *testing.T) {
	sc := layoutScene("s3", 2)
	sc.Texts = []LayoutText{
		{ID: "title", Box: LayoutBox{0.7, 0.2, 0.35, 0.1}},  // a seventh outside to the right
		{ID: "fine", Box: LayoutBox{0.3, 0.2, 0.3, 0.1}},    // inside
		{ID: "hair", Box: LayoutBox{0.7, 0.2, 0.3005, 0.1}}, // 0.17 percent outside
	}
	var got []Issue
	for _, is := range LayoutIssues([]LayoutScene{sc}, LayoutOptions{}) {
		if is.Code != CodeTextUnsafe { // a text at the frame edge may also lie in the margin; that is another rule
			got = append(got, is)
		}
	}
	if len(got) != 1 {
		t.Fatalf("issues = %v, want one text_offframe", layoutCodes(got))
	}
	is := got[0]
	if is.Code != CodeTextOffframe || is.Severity != SeverityWarning || is.SceneID != "s3" || is.Path != "/scenes/2" {
		t.Errorf("issue = %+v", is)
	}
	if !strings.Contains(is.Message, "title") || !strings.Contains(is.Message, "s3") || !strings.Contains(is.Fix, `"center"`) {
		t.Errorf("message %q, fix %q", is.Message, is.Fix)
	}
	if IssueLayout(is.Code) || !IssueModelFixable(is) {
		t.Errorf("text_offframe is model-fixable and not a layout code")
	}
}

func TestLayoutIssuesStaticSceneAndMissingArt(t *testing.T) {
	sc := layoutScene("s4", 3)
	sc.Static = true
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run 'TestLayoutIssuesTextOffframe|TestLayoutIssuesOffscreenAndOverlap' -count=1`
Expected: layout_test.go:141: issues = [actor_prop_overlap offscreen offscreen], want [actor_prop_overlap offscreen text_offframe]

- [ ] **Step 3: Implement**

In `internal/storymaker/layout.go`, replace:

```go
		}
		for _, tx := range sc.Texts {
			if outsideShare(tx.Box) > layoutTextOutside {
				add(CodeOffscreen, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		for _, a := range sc.Actors {
```

with:

```go
		}
		for _, tx := range sc.Texts {
			if outsideShare(tx.Box) > layoutTextOutside {
				add(CodeTextOffframe, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		for _, a := range sc.Actors {
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run 'TestLayoutIssuesTextOffframe|TestLayoutIssuesOffscreenAndOverlap' -count=1`
Expected: ok  	aurago/internal/storymaker (2 tests PASS)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/layout.go internal/storymaker/layout_test.go
git commit -m "storymaker: a text outside the frame is text_offframe"
```

### Task A4: texts and the caption in the safe zone give text_unsafe

**Files:**
- Modify: `internal/storymaker/layout.go`
- Modify: `internal/storymaker/layout_zone_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/layout_zone_test.go`, replace:

```go
package storymaker

import "testing"

// The 9:16 zone is the union of the platform overlays; every other aspect keeps the stage's 5 percent margin.
func TestSafeZone(t *testing.T) {
```

with:

```go
package storymaker

import (
	"slices"
	"strings"
	"testing"
)

// The 9:16 zone is the union of the platform overlays; every other aspect keeps the stage's 5 percent margin.
func TestSafeZone(t *testing.T) {
```

In `internal/storymaker/layout_zone_test.go`, replace:

```go
	}
}

// The check hands the story's aspect to the layout rules.
func TestStoryLayoutOptionsCarryTheAspect(t *testing.T) {
	var s Story
```

with:

```go
	}
}

// A text or the caption with more than 2 percent of its box inside the zone is text_unsafe, in the zone of the story's aspect.
func TestLayoutIssuesTextUnsafe(t *testing.T) {
	scene := func() LayoutScene {
		sc := layoutScene("s2", 1)
		sc.Texts = []LayoutText{
			{ID: "top", Box: LayoutBox{0.3, 0.02, 0.4, 0.1}},    // 16:9: three tenths of the box sit above the 5 percent margin
			{ID: "low", Box: LayoutBox{0.3, 0.70, 0.4, 0.1}},    // 9:16: one fifth of the box sits in the bottom zone (0.781)
			{ID: "mid", Box: LayoutBox{0.3, 0.40, 0.4, 0.1}},    // safe everywhere
			{ID: "edge", Box: LayoutBox{0.3, 0.0495, 0.4, 0.1}}, // 16:9: only half a percent of the box is in the margin
		}
		return sc
	}
	only := func(aspect string) []Issue {
		return LayoutIssues([]LayoutScene{scene()}, LayoutOptions{Aspect: aspect})
	}
	ids := func(issues []Issue) []string {
		var out []string
		for _, is := range issues {
			if is.Code != CodeTextUnsafe {
				t.Errorf("unexpected %s", is.Code)
			}
			for _, id := range []string{"top", "low", "mid", "edge"} {
				if strings.Contains(is.Message, "text "+id+" ") {
					out = append(out, id)
				}
			}
		}
		return out
	}
	if got := ids(only("16:9")); !slices.Equal(got, []string{"top"}) {
		t.Errorf("16:9 unsafe texts = %v, want [top]", got)
	}
	// 9:16: the top zone is 0.115, so "top" and "edge" are unsafe there, and "low" joins them.
	if got := ids(only("9:16")); !slices.Equal(got, []string{"top", "low", "edge"}) {
		t.Errorf("9:16 unsafe texts = %v, want [top low edge]", got)
	}
	is := only("16:9")[0]
	if is.Severity != SeverityWarning || is.SceneID != "s2" || is.Path != "/scenes/1" || !strings.Contains(is.Message, "s2") || is.Fix == "" {
		t.Errorf("issue = %+v", is)
	}
	if IssueLayout(is.Code) || IssueModelFixable(is) || !IssueAdvisory(is.Code) {
		t.Errorf("text_unsafe is advice, not a layout code")
	}
}

// A text that already sticks out of the frame gets text_offframe only; the caption is checked in every aspect.
func TestLayoutIssuesTextUnsafeSkipsOffframeAndChecksTheCaption(t *testing.T) {
	sc := layoutScene("s1", 0)
	sc.Texts = []LayoutText{{ID: "wide", Box: LayoutBox{0.7, 0.4, 0.4, 0.1}}} // out of the frame and in the right zone
	if got := layoutCodes(LayoutIssues([]LayoutScene{sc}, LayoutOptions{})); !slices.Equal(got, []string{CodeTextOffframe}) {
		t.Fatalf("issues = %v, want only text_offframe", got)
	}
	sc.Texts = nil
	caption := LayoutBox{0.2, 0.70, 0.6, 0.126} // bottom at 0.826
	sc.Caption = &caption
	if got := layoutCodes(LayoutIssues([]LayoutScene{sc}, LayoutOptions{Aspect: "9:16"})); !slices.Equal(got, []string{CodeTextUnsafe}) {
		t.Errorf("9:16 caption issues = %v, want [text_unsafe]", got)
	}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{Aspect: "16:9"})
	if len(got) != 0 {
		t.Errorf("16:9 caption at 0.826 is inside the 5 percent margin, got %v", layoutCodes(got))
	}
	low := LayoutBox{0.2, 0.84, 0.6, 0.126} // bottom at 0.966
	sc.Caption = &low
	got = LayoutIssues([]LayoutScene{sc}, LayoutOptions{Aspect: "16:9"})
	if len(got) != 1 || got[0].Code != CodeTextUnsafe || !strings.Contains(got[0].Message, "caption") || !strings.Contains(got[0].Message, "s1") {
		t.Errorf("16:9 caption in the margin: %+v", got)
	}
}

// The check hands the story's aspect to the layout rules.
func TestStoryLayoutOptionsCarryTheAspect(t *testing.T) {
	var s Story
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run 'TestLayoutIssuesTextUnsafe' -count=1`
Expected: layout_zone_test.go:52: 16:9 unsafe texts = [], want [top]

- [ ] **Step 3: Implement**

In `internal/storymaker/layout.go`, replace:

```go
				add(CodeOffscreen, fmt.Sprintf("The prop %s is partly outside the frame in scene %s.", p.ID, sc.ID), `Show it at a spot inside the frame such as "center" or "right".`)
			}
		}
		for _, tx := range sc.Texts {
			if outsideShare(tx.Box) > layoutTextOutside {
				add(CodeTextOffframe, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		for _, a := range sc.Actors {
			for _, p := range sc.Props {
				if !p.Back && overlapShare(a.Box, p.Box) > layoutOverlapShare {
```

with:

```go
				add(CodeOffscreen, fmt.Sprintf("The prop %s is partly outside the frame in scene %s.", p.ID, sc.ID), `Show it at a spot inside the frame such as "center" or "right".`)
			}
		}
		zone := SafeZone(opts.Aspect)
		for _, tx := range sc.Texts {
			if outsideShare(tx.Box) <= layoutTextOutside && zoneShare(tx.Box, zone) > layoutZoneShare {
				add(CodeTextUnsafe, fmt.Sprintf("The text %s lies in the margin that platform overlays cover in scene %s.", tx.ID, sc.ID),
					`Show the text at "center" or "board", or shorten it, so it keeps clear of the frame edges.`)
			}
			if outsideShare(tx.Box) > layoutTextOutside {
				add(CodeTextOffframe, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		if sc.Caption != nil && zoneShare(*sc.Caption, zone) > layoutZoneShare {
			add(CodeTextUnsafe, fmt.Sprintf("The caption lies in the margin that platform overlays cover in scene %s.", sc.ID),
				`Shorten the narration of the scene so the caption has fewer lines.`)
		}
		for _, a := range sc.Actors {
			for _, p := range sc.Props {
				if !p.Back && overlapShare(a.Box, p.Box) > layoutOverlapShare {
```

In `internal/storymaker/layout.go`, replace:

```go
	return 1 - intersection(b, LayoutBox{0, 0, 1, 1})/area(b)
}

// overlapShare is the overlap as a share of the smaller box.
func overlapShare(a, b LayoutBox) float64 {
	smaller := min(area(a), area(b))
```

with:

```go
	return 1 - intersection(b, LayoutBox{0, 0, 1, 1})/area(b)
}

// zoneShare is the share of a box's area that lies inside the frame but within the safe zone's margins. The part outside the
// frame is not counted: it is text_offframe's business.
func zoneShare(b LayoutBox, z LayoutZone) float64 {
	if area(b) == 0 {
		return 0
	}
	safe := LayoutBox{z.Left, z.Top, 1 - z.Left - z.Right, 1 - z.Top - z.Bottom}
	return (intersection(b, LayoutBox{0, 0, 1, 1}) - intersection(b, safe)) / area(b)
}

// overlapShare is the overlap as a share of the smaller box.
func overlapShare(a, b LayoutBox) float64 {
	smaller := min(area(a), area(b))
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run 'TestLayoutIssuesTextUnsafe' -count=1`
Expected: ok  	aurago/internal/storymaker (TestLayoutIssuesTextUnsafe, TestLayoutIssuesTextUnsafeSkipsOffframeAndChecksTheCaption PASS)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/layout.go internal/storymaker/layout_zone_test.go
git commit -m "storymaker: texts and the caption in the safe zone give text_unsafe"
```

### Task A5: flash_unverified is an open check, and the measured codes decide the outcome

**Files:**
- Modify: `internal/storymaker/studio_check.go`
- Create: `internal/storymaker/studio_measured_check_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/studio_measured_check_test.go`:

```go
package storymaker

import (
	"context"
	"slices"
	"testing"
)

// measuredRig runs the check on a story whose compile stub reports the given compile issues and layout scenes, and decides the
// outcome of the report the way the job does.
func measuredRig(t *testing.T, issues []Issue, layout []LayoutScene) (CheckReport, string, []AgentCheck) {
	t.Helper()
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{Available: true, DurationMS: 8000, StoryboardScenes: 2, Layout: layout, Issues: issues}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	outcome, checks := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	return report, outcome, checks
}

func reportCodes(list []Issue) []string {
	var out []string
	for _, is := range list {
		out = append(out, is.Code)
	}
	return out
}

func measuredIssue(code string) Issue {
	return Issue{Severity: SeverityWarning, SceneID: "s1", Path: "/scenes/0", Code: code, Message: code}
}

func measuredLayout() []LayoutScene { return []LayoutScene{layoutScene("s1", 0), layoutScene("s2", 1)} }

// The probe did not reach every frame: the finding is an open check, listed but never blocking.
func TestCheckListsFlashUnverifiedAsAnOpenCheck(t *testing.T) {
	report, outcome, checks := measuredRig(t, []Issue{measuredIssue(CodeFlashUnverified)}, measuredLayout())
	if !slices.Contains(reportCodes(report.Environment), CodeFlashUnverified) {
		t.Fatalf("environment = %v, want flash_unverified", reportCodes(report.Environment))
	}
	for _, list := range [][]Issue{report.Issues, report.Advice} {
		if slices.Contains(reportCodes(list), CodeFlashUnverified) {
			t.Fatalf("flash_unverified is only an open check: issues %v, advice %v", reportCodes(report.Issues), reportCodes(report.Advice))
		}
	}
	if !slices.Contains(checks, AgentCheck{Code: CodeFlashUnverified, SceneID: "s1"}) {
		t.Errorf("checks = %+v, want flash_unverified listed", checks)
	}
	if outcome != OutcomeReadyUnverified {
		t.Errorf("outcome = %q, want ready_unverified", outcome)
	}
	if HasErrors(report.Environment) {
		t.Error("flash_unverified must not be an error")
	}
}

// Contrast, safe-zone and slow-hook findings are advice: they never block and never count as open checks.
func TestCheckKeepsTheAdviceCodesAsAdvice(t *testing.T) {
	inZone := layoutScene("s2", 1)
	inZone.Texts = []LayoutText{{ID: "title", Box: LayoutBox{0.3, 0.01, 0.4, 0.1}}} // in the top margin, inside the frame
	issues := []Issue{measuredIssue(CodeTextContrast), measuredIssue(CodeSlowHook)}
	report, outcome, checks := measuredRig(t, issues, []LayoutScene{layoutScene("s1", 0), inZone})
	for _, code := range []string{CodeTextContrast, CodeSlowHook, CodeTextUnsafe} {
		if !slices.Contains(reportCodes(report.Advice), code) {
			t.Errorf("advice = %v, want %s", reportCodes(report.Advice), code)
		}
		if slices.Contains(reportCodes(report.Issues), code) || slices.Contains(reportCodes(report.Environment), code) {
			t.Errorf("%s must be advice only", code)
		}
		for _, c := range checks {
			if c.Code == code {
				t.Errorf("%s must not be listed as an open check", code)
			}
		}
	}
	if outcome == OutcomeIncomplete {
		t.Errorf("advice must not block: %s", outcome)
	}
	if HasErrors(report.Advice) {
		t.Error("advice must not be an error")
	}
}

// A text outside the frame and a flash are checks to fix: they go to the model's issues and, left over, make the job
// incomplete. Neither is an error, so a person's export is never refused.
func TestCheckBlocksOnTextOffframeAndFlash(t *testing.T) {
	out := layoutScene("s2", 1)
	out.Texts = []LayoutText{{ID: "title", Box: LayoutBox{0.7, 0.4, 0.4, 0.1}}}
	for _, tc := range []struct {
		name   string
		code   string
		issues []Issue
		layout []LayoutScene
	}{
		{"text_offframe", CodeTextOffframe, nil, []LayoutScene{layoutScene("s1", 0), out}},
		{"flash", CodeFlash, []Issue{measuredIssue(CodeFlash)}, measuredLayout()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, outcome, checks := measuredRig(t, tc.issues, tc.layout)
			if !slices.Contains(reportCodes(report.Issues), tc.code) {
				t.Fatalf("issues = %v, want %s", reportCodes(report.Issues), tc.code)
			}
			if outcome != OutcomeIncomplete {
				t.Errorf("outcome = %q, want incomplete", outcome)
			}
			if !slices.ContainsFunc(checks, func(c AgentCheck) bool { return c.Code == tc.code }) {
				t.Errorf("checks = %+v, want %s listed", checks, tc.code)
			}
			if HasErrors(report.Issues) {
				t.Errorf("%s must not be an error: %+v", tc.code, report.Issues)
			}
		})
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/ -run 'TestCheck(ListsFlash|KeepsTheAdvice|BlocksOn)' -count=1`
Expected: studio_measured_check_test.go:43: environment = [], want flash_unverified

- [ ] **Step 3: Implement**

In `internal/storymaker/studio_check.go`, replace:

```go
	}
	for _, is := range found {
		switch {
		case is.Code == CodeOverTarget, is.Code == CodeCompileFailed && is.SceneID == "", is.Severity == SeverityError && !IssueModelFixable(is):
			report.Environment = append(report.Environment, is)
		case IssueModelFixable(is):
```

with:

```go
	}
	for _, is := range found {
		switch {
		case is.Code == CodeFlashUnverified:
			// An open check: listed by DecideOutcome, never blocking, and nothing the model can fix.
			report.Environment = append(report.Environment, is)
		case is.Code == CodeOverTarget, is.Code == CodeCompileFailed && is.SceneID == "", is.Severity == SeverityError && !IssueModelFixable(is):
			report.Environment = append(report.Environment, is)
		case IssueModelFixable(is):
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/ -run 'TestCheck(ListsFlash|KeepsTheAdvice|BlocksOn)' -count=1`
Expected: --- PASS: TestCheckListsFlashUnverifiedAsAnOpenCheck, TestCheckKeepsTheAdviceCodesAsAdvice, TestCheckBlocksOnTextOffframeAndFlash (3 PASS)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_check.go internal/storymaker/studio_measured_check_test.go
git commit -m "storymaker: flash_unverified is an open check, and the measured codes decide the outcome"
```

### Task A6: locale keys for the measured issue codes in all 16 languages

**Files:**
- Modify: `ui/desktop_storymaker_i18n_test.go`
- Modify: `ui/lang/storymaker/cs.json`
- Modify: `ui/lang/storymaker/da.json`
- Modify: `ui/lang/storymaker/de.json`
- Modify: `ui/lang/storymaker/el.json`
- Modify: `ui/lang/storymaker/en.json`
- Modify: `ui/lang/storymaker/es.json`
- Modify: `ui/lang/storymaker/fr.json`
- Modify: `ui/lang/storymaker/hi.json`
- Modify: `ui/lang/storymaker/it.json`
- Modify: `ui/lang/storymaker/ja.json`
- Modify: `ui/lang/storymaker/nl.json`
- Modify: `ui/lang/storymaker/no.json`
- Modify: `ui/lang/storymaker/pl.json`
- Modify: `ui/lang/storymaker/pt.json`
- Modify: `ui/lang/storymaker/sv.json`
- Modify: `ui/lang/storymaker/zh.json`

- [ ] **Step 1: Write the failing test**

In `ui/desktop_storymaker_i18n_test.go`, replace:

```go
				t.Errorf("%s keeps the English text of %s: %q", lang, key, text)
			}
		}
	}
}
```

with:

```go
				t.Errorf("%s keeps the English text of %s: %q", lang, key, text)
			}
		}
	}
}

// The measured picture checks name their findings in every language, each in its own words.
func TestStorymakerMeasuredIssueKeysAreTranslated(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "storymaker", "en")
	for _, code := range strings.Fields("text_offframe text_unsafe text_contrast flash flash_unverified slow_hook") {
		key := "storymaker.issue." + code
		if strings.TrimSpace(english[key]) == "" {
			t.Errorf("lang/storymaker/en.json lacks %s", key)
		}
		for _, lang := range storymakerLanguages {
			text := strings.TrimSpace(storymakerLocale(t, "storymaker", lang)[key])
			if text == "" || (lang != "en" && text == english[key]) {
				t.Errorf("%s lacks a translated %s (%q)", lang, key, text)
			}
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./ui -run 'TestStorymakerMeasuredIssueKeysAreTranslated' -count=1`
Expected: desktop_storymaker_i18n_test.go:108: lang/storymaker/en.json lacks storymaker.issue.text_offframe

- [ ] **Step 3: Implement**

In `ui/lang/storymaker/cs.json`, replace:

```json
  "storymaker.issue.transition_variety": "Příliš mnoho druhů přechodů",
  "storymaker.issue.cue_late": "Značka přichází pozdě",
  "storymaker.issue.glyph_missing": "Znak není k dispozici",
  "storymaker.issue.unknown": "Problém",
  "storymaker.code.title": "Kód",
  "storymaker.code.files": "Soubory",
```

with:

```json
  "storymaker.issue.transition_variety": "Příliš mnoho druhů přechodů",
  "storymaker.issue.cue_late": "Značka přichází pozdě",
  "storymaker.issue.glyph_missing": "Znak není k dispozici",
  "storymaker.issue.text_offframe": "Text přesahuje okraj obrazu",
  "storymaker.issue.text_unsafe": "Text je příliš blízko okraje",
  "storymaker.issue.text_contrast": "Text je na obrázku špatně čitelný",
  "storymaker.issue.flash": "Příliš rychlé blikání",
  "storymaker.issue.flash_unverified": "Blikání není zcela zkontrolováno",
  "storymaker.issue.slow_hook": "Pomalý začátek",
  "storymaker.issue.unknown": "Problém",
  "storymaker.code.title": "Kód",
  "storymaker.code.files": "Soubory",
```

In `ui/lang/storymaker/da.json`, replace:

```json
  "storymaker.issue.transition_variety": "For mange slags overgange",
  "storymaker.issue.cue_late": "Markeringen kommer for sent",
  "storymaker.issue.glyph_missing": "Tegnet er ikke tilgængeligt",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

with:

```json
  "storymaker.issue.transition_variety": "For mange slags overgange",
  "storymaker.issue.cue_late": "Markeringen kommer for sent",
  "storymaker.issue.glyph_missing": "Tegnet er ikke tilgængeligt",
  "storymaker.issue.text_offframe": "Teksten stikker ud over billedet",
  "storymaker.issue.text_unsafe": "Teksten er for tæt på kanten",
  "storymaker.issue.text_contrast": "Teksten er svær at læse på billedet",
  "storymaker.issue.flash": "Blinker for hurtigt",
  "storymaker.issue.flash_unverified": "Blink er ikke helt kontrolleret",
  "storymaker.issue.slow_hook": "Langsom start",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

In `ui/lang/storymaker/de.json`, replace:

```json
  "storymaker.issue.transition_variety": "Zu viele Übergangsarten",
  "storymaker.issue.cue_late": "Einsatz kommt zu spät",
  "storymaker.issue.glyph_missing": "Zeichen nicht verfügbar",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Dateien",
```

with:

```json
  "storymaker.issue.transition_variety": "Zu viele Übergangsarten",
  "storymaker.issue.cue_late": "Einsatz kommt zu spät",
  "storymaker.issue.glyph_missing": "Zeichen nicht verfügbar",
  "storymaker.issue.text_offframe": "Text ragt aus dem Bild",
  "storymaker.issue.text_unsafe": "Text zu nah am Rand",
  "storymaker.issue.text_contrast": "Text auf dem Bild schwer lesbar",
  "storymaker.issue.flash": "Zu schnelles Blitzen",
  "storymaker.issue.flash_unverified": "Blitzen nicht vollständig geprüft",
  "storymaker.issue.slow_hook": "Langsamer Einstieg",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Dateien",
```

In `ui/lang/storymaker/el.json`, replace:

```json
  "storymaker.issue.transition_variety": "Πάρα πολλά είδη μεταβάσεων",
  "storymaker.issue.cue_late": "Το σημείο καθυστερεί",
  "storymaker.issue.glyph_missing": "Ο χαρακτήρας δεν είναι διαθέσιμος",
  "storymaker.issue.unknown": "Πρόβλημα",
  "storymaker.code.title": "Κώδικας",
  "storymaker.code.files": "Αρχεία",
```

with:

```json
  "storymaker.issue.transition_variety": "Πάρα πολλά είδη μεταβάσεων",
  "storymaker.issue.cue_late": "Το σημείο καθυστερεί",
  "storymaker.issue.glyph_missing": "Ο χαρακτήρας δεν είναι διαθέσιμος",
  "storymaker.issue.text_offframe": "Το κείμενο βγαίνει εκτός εικόνας",
  "storymaker.issue.text_unsafe": "Το κείμενο είναι πολύ κοντά στην άκρη",
  "storymaker.issue.text_contrast": "Το κείμενο διαβάζεται δύσκολα στην εικόνα",
  "storymaker.issue.flash": "Πολύ γρήγορες αναλαμπές",
  "storymaker.issue.flash_unverified": "Οι αναλαμπές δεν ελέγχθηκαν πλήρως",
  "storymaker.issue.slow_hook": "Αργή αρχή",
  "storymaker.issue.unknown": "Πρόβλημα",
  "storymaker.code.title": "Κώδικας",
  "storymaker.code.files": "Αρχεία",
```

In `ui/lang/storymaker/en.json`, replace:

```json
  "storymaker.issue.transition_variety": "Too many transition kinds",
  "storymaker.issue.cue_late": "Cue lands late",
  "storymaker.issue.glyph_missing": "Character not available",
  "storymaker.issue.unknown": "Issue",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Files",
```

with:

```json
  "storymaker.issue.transition_variety": "Too many transition kinds",
  "storymaker.issue.cue_late": "Cue lands late",
  "storymaker.issue.glyph_missing": "Character not available",
  "storymaker.issue.text_offframe": "Text sticks out of the frame",
  "storymaker.issue.text_unsafe": "Text too close to the edge",
  "storymaker.issue.text_contrast": "Text hard to read on the picture",
  "storymaker.issue.flash": "Flashing too fast",
  "storymaker.issue.flash_unverified": "Flashing not fully checked",
  "storymaker.issue.slow_hook": "Slow start",
  "storymaker.issue.unknown": "Issue",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Files",
```

In `ui/lang/storymaker/es.json`, replace:

```json
  "storymaker.issue.transition_variety": "Demasiados tipos de transición",
  "storymaker.issue.cue_late": "La marca llega tarde",
  "storymaker.issue.glyph_missing": "Carácter no disponible",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Archivos",
```

with:

```json
  "storymaker.issue.transition_variety": "Demasiados tipos de transición",
  "storymaker.issue.cue_late": "La marca llega tarde",
  "storymaker.issue.glyph_missing": "Carácter no disponible",
  "storymaker.issue.text_offframe": "El texto sobresale del encuadre",
  "storymaker.issue.text_unsafe": "Texto demasiado cerca del borde",
  "storymaker.issue.text_contrast": "Texto difícil de leer sobre la imagen",
  "storymaker.issue.flash": "Destellos demasiado rápidos",
  "storymaker.issue.flash_unverified": "Destellos no comprobados del todo",
  "storymaker.issue.slow_hook": "Inicio lento",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Archivos",
```

In `ui/lang/storymaker/fr.json`, replace:

```json
  "storymaker.issue.transition_variety": "Trop de types de transition",
  "storymaker.issue.cue_late": "Le repère arrive en retard",
  "storymaker.issue.glyph_missing": "Caractère indisponible",
  "storymaker.issue.unknown": "Problème",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Fichiers",
```

with:

```json
  "storymaker.issue.transition_variety": "Trop de types de transition",
  "storymaker.issue.cue_late": "Le repère arrive en retard",
  "storymaker.issue.glyph_missing": "Caractère indisponible",
  "storymaker.issue.text_offframe": "Le texte dépasse de l'image",
  "storymaker.issue.text_unsafe": "Texte trop près du bord",
  "storymaker.issue.text_contrast": "Texte difficile à lire sur l'image",
  "storymaker.issue.flash": "Clignotement trop rapide",
  "storymaker.issue.flash_unverified": "Clignotement pas entièrement vérifié",
  "storymaker.issue.slow_hook": "Début trop lent",
  "storymaker.issue.unknown": "Problème",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Fichiers",
```

In `ui/lang/storymaker/hi.json`, replace:

```json
  "storymaker.issue.transition_variety": "बहुत तरह के ट्रांज़िशन",
  "storymaker.issue.cue_late": "संकेत देर से आता है",
  "storymaker.issue.glyph_missing": "अक्षर उपलब्ध नहीं है",
  "storymaker.issue.unknown": "समस्या",
  "storymaker.code.title": "कोड",
  "storymaker.code.files": "फ़ाइलें",
```

with:

```json
  "storymaker.issue.transition_variety": "बहुत तरह के ट्रांज़िशन",
  "storymaker.issue.cue_late": "संकेत देर से आता है",
  "storymaker.issue.glyph_missing": "अक्षर उपलब्ध नहीं है",
  "storymaker.issue.text_offframe": "टेक्स्ट फ़्रेम से बाहर निकल रहा है",
  "storymaker.issue.text_unsafe": "टेक्स्ट किनारे के बहुत पास है",
  "storymaker.issue.text_contrast": "तस्वीर पर टेक्स्ट पढ़ना मुश्किल है",
  "storymaker.issue.flash": "बहुत तेज़ चमक",
  "storymaker.issue.flash_unverified": "चमक की पूरी जाँच नहीं हुई",
  "storymaker.issue.slow_hook": "धीमी शुरुआत",
  "storymaker.issue.unknown": "समस्या",
  "storymaker.code.title": "कोड",
  "storymaker.code.files": "फ़ाइलें",
```

In `ui/lang/storymaker/it.json`, replace:

```json
  "storymaker.issue.transition_variety": "Troppi tipi di transizione",
  "storymaker.issue.cue_late": "Il segnale arriva in ritardo",
  "storymaker.issue.glyph_missing": "Carattere non disponibile",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Codice",
  "storymaker.code.files": "File",
```

with:

```json
  "storymaker.issue.transition_variety": "Troppi tipi di transizione",
  "storymaker.issue.cue_late": "Il segnale arriva in ritardo",
  "storymaker.issue.glyph_missing": "Carattere non disponibile",
  "storymaker.issue.text_offframe": "Il testo esce dall'inquadratura",
  "storymaker.issue.text_unsafe": "Testo troppo vicino al bordo",
  "storymaker.issue.text_contrast": "Testo difficile da leggere sull'immagine",
  "storymaker.issue.flash": "Lampeggio troppo rapido",
  "storymaker.issue.flash_unverified": "Lampeggio non verificato del tutto",
  "storymaker.issue.slow_hook": "Inizio lento",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Codice",
  "storymaker.code.files": "File",
```

In `ui/lang/storymaker/ja.json`, replace:

```json
  "storymaker.issue.transition_variety": "トランジションの種類が多すぎます",
  "storymaker.issue.cue_late": "キューが遅れて着地します",
  "storymaker.issue.glyph_missing": "使用できない文字",
  "storymaker.issue.unknown": "問題",
  "storymaker.code.title": "コード",
  "storymaker.code.files": "ファイル",
```

with:

```json
  "storymaker.issue.transition_variety": "トランジションの種類が多すぎます",
  "storymaker.issue.cue_late": "キューが遅れて着地します",
  "storymaker.issue.glyph_missing": "使用できない文字",
  "storymaker.issue.text_offframe": "文字が画面からはみ出しています",
  "storymaker.issue.text_unsafe": "文字が端に近すぎます",
  "storymaker.issue.text_contrast": "画像の上で文字が読みにくい",
  "storymaker.issue.flash": "点滅が速すぎます",
  "storymaker.issue.flash_unverified": "点滅を完全には確認できていません",
  "storymaker.issue.slow_hook": "出だしが遅い",
  "storymaker.issue.unknown": "問題",
  "storymaker.code.title": "コード",
  "storymaker.code.files": "ファイル",
```

In `ui/lang/storymaker/nl.json`, replace:

```json
  "storymaker.issue.transition_variety": "Te veel soorten overgangen",
  "storymaker.issue.cue_late": "Cue komt te laat",
  "storymaker.issue.glyph_missing": "Teken niet beschikbaar",
  "storymaker.issue.unknown": "Probleem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Bestanden",
```

with:

```json
  "storymaker.issue.transition_variety": "Te veel soorten overgangen",
  "storymaker.issue.cue_late": "Cue komt te laat",
  "storymaker.issue.glyph_missing": "Teken niet beschikbaar",
  "storymaker.issue.text_offframe": "Tekst steekt buiten het beeld uit",
  "storymaker.issue.text_unsafe": "Tekst te dicht bij de rand",
  "storymaker.issue.text_contrast": "Tekst slecht leesbaar op het beeld",
  "storymaker.issue.flash": "Flitsen te snel",
  "storymaker.issue.flash_unverified": "Flitsen niet volledig gecontroleerd",
  "storymaker.issue.slow_hook": "Trage start",
  "storymaker.issue.unknown": "Probleem",
  "storymaker.code.title": "Code",
  "storymaker.code.files": "Bestanden",
```

In `ui/lang/storymaker/no.json`, replace:

```json
  "storymaker.issue.transition_variety": "For mange typer overganger",
  "storymaker.issue.cue_late": "Markeringen kommer for sent",
  "storymaker.issue.glyph_missing": "Tegnet er ikke tilgjengelig",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

with:

```json
  "storymaker.issue.transition_variety": "For mange typer overganger",
  "storymaker.issue.cue_late": "Markeringen kommer for sent",
  "storymaker.issue.glyph_missing": "Tegnet er ikke tilgjengelig",
  "storymaker.issue.text_offframe": "Teksten stikker ut av bildet",
  "storymaker.issue.text_unsafe": "Teksten er for nær kanten",
  "storymaker.issue.text_contrast": "Teksten er vanskelig å lese på bildet",
  "storymaker.issue.flash": "Blinker for raskt",
  "storymaker.issue.flash_unverified": "Blinking er ikke helt kontrollert",
  "storymaker.issue.slow_hook": "Treg start",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
```

In `ui/lang/storymaker/pl.json`, replace:

```json
  "storymaker.issue.transition_variety": "Zbyt wiele rodzajów przejść",
  "storymaker.issue.cue_late": "Znacznik wypada za późno",
  "storymaker.issue.glyph_missing": "Znak niedostępny",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Pliki",
```

with:

```json
  "storymaker.issue.transition_variety": "Zbyt wiele rodzajów przejść",
  "storymaker.issue.cue_late": "Znacznik wypada za późno",
  "storymaker.issue.glyph_missing": "Znak niedostępny",
  "storymaker.issue.text_offframe": "Tekst wystaje poza kadr",
  "storymaker.issue.text_unsafe": "Tekst zbyt blisko krawędzi",
  "storymaker.issue.text_contrast": "Tekst trudny do odczytania na obrazie",
  "storymaker.issue.flash": "Zbyt szybkie migotanie",
  "storymaker.issue.flash_unverified": "Migotanie nie w pełni sprawdzone",
  "storymaker.issue.slow_hook": "Wolny początek",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Pliki",
```

In `ui/lang/storymaker/pt.json`, replace:

```json
  "storymaker.issue.transition_variety": "Demasiados tipos de transição",
  "storymaker.issue.cue_late": "A marca chega atrasada",
  "storymaker.issue.glyph_missing": "Carácter indisponível",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Ficheiros",
```

with:

```json
  "storymaker.issue.transition_variety": "Demasiados tipos de transição",
  "storymaker.issue.cue_late": "A marca chega atrasada",
  "storymaker.issue.glyph_missing": "Carácter indisponível",
  "storymaker.issue.text_offframe": "O texto sai do enquadramento",
  "storymaker.issue.text_unsafe": "Texto demasiado perto da margem",
  "storymaker.issue.text_contrast": "Texto difícil de ler na imagem",
  "storymaker.issue.flash": "Piscar demasiado rápido",
  "storymaker.issue.flash_unverified": "Piscar não totalmente verificado",
  "storymaker.issue.slow_hook": "Início lento",
  "storymaker.issue.unknown": "Problema",
  "storymaker.code.title": "Código",
  "storymaker.code.files": "Ficheiros",
```

In `ui/lang/storymaker/sv.json`, replace:

```json
  "storymaker.issue.transition_variety": "För många sorters övergångar",
  "storymaker.issue.cue_late": "Markeringen kommer för sent",
  "storymaker.issue.glyph_missing": "Tecknet är inte tillgängligt",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Filer",
```

with:

```json
  "storymaker.issue.transition_variety": "För många sorters övergångar",
  "storymaker.issue.cue_late": "Markeringen kommer för sent",
  "storymaker.issue.glyph_missing": "Tecknet är inte tillgängligt",
  "storymaker.issue.text_offframe": "Texten sticker ut ur bilden",
  "storymaker.issue.text_unsafe": "Texten är för nära kanten",
  "storymaker.issue.text_contrast": "Texten är svår att läsa på bilden",
  "storymaker.issue.flash": "Blinkar för snabbt",
  "storymaker.issue.flash_unverified": "Blinkning inte helt kontrollerad",
  "storymaker.issue.slow_hook": "Långsam start",
  "storymaker.issue.unknown": "Problem",
  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Filer",
```

In `ui/lang/storymaker/zh.json`, replace:

```json
  "storymaker.issue.transition_variety": "转场类型过多",
  "storymaker.issue.cue_late": "提示点落后",
  "storymaker.issue.glyph_missing": "字符不可用",
  "storymaker.issue.unknown": "问题",
  "storymaker.code.title": "代码",
  "storymaker.code.files": "文件",
```

with:

```json
  "storymaker.issue.transition_variety": "转场类型过多",
  "storymaker.issue.cue_late": "提示点落后",
  "storymaker.issue.glyph_missing": "字符不可用",
  "storymaker.issue.text_offframe": "文字超出画面",
  "storymaker.issue.text_unsafe": "文字离边缘太近",
  "storymaker.issue.text_contrast": "文字在画面上难以辨认",
  "storymaker.issue.flash": "闪烁过快",
  "storymaker.issue.flash_unverified": "闪烁未完全检查",
  "storymaker.issue.slow_hook": "开头太慢",
  "storymaker.issue.unknown": "问题",
  "storymaker.code.title": "代码",
  "storymaker.code.files": "文件",
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./ui -run 'TestStorymakerMeasuredIssueKeysAreTranslated' -count=1`
Expected: --- PASS: TestStorymakerMeasuredIssueKeysAreTranslated

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_i18n_test.go ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json
git commit -m "storymaker: locale keys for the measured issue codes in all 16 languages"
```

### Task A7: the editor titles the measured issue codes

**Files:**
- Modify: `ui/desktop_storymaker_guards_test.go`
- Modify: `ui/js/desktop/apps/storymaker-core.js`

- [ ] **Step 1: Write the failing test**

In `ui/desktop_storymaker_guards_test.go`, replace:

```go
		"storymaker.export.phase_":  storymakerList(t, exporter, "PHASES"),
		"storymaker.export.source_": {"config", "system", "pinned"},
	}
	// The list of issue codes is the contract's: twenty-eight codes, no more, no less.
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified line_unread dead_stretch first_frame_empty text_too_small fade_between_worlds entry_monotony transition_variety cue_late glyph_missing")
	if strings.Join(families["storymaker.issue."], " ") != strings.Join(want, " ") {
		t.Errorf("ISSUE_CODES = %v, want the 28 codes of the contract", families["storymaker.issue."])
	}
	// A narrate job ends with these codes, so the app needs a sentence for each. Batch 2C adds more, so this is no exact list.
	jobCodes := " " + strings.Join(families["storymaker.job_error."], " ") + " "
```

with:

```go
		"storymaker.export.phase_":  storymakerList(t, exporter, "PHASES"),
		"storymaker.export.source_": {"config", "system", "pinned"},
	}
	// The list of issue codes is the contract's: thirty-four codes, no more, no less.
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified line_unread dead_stretch first_frame_empty text_too_small fade_between_worlds entry_monotony transition_variety cue_late glyph_missing text_offframe text_unsafe text_contrast flash flash_unverified slow_hook")
	if strings.Join(families["storymaker.issue."], " ") != strings.Join(want, " ") {
		t.Errorf("ISSUE_CODES = %v, want the 34 codes of the contract", families["storymaker.issue."])
	}
	// A narrate job ends with these codes, so the app needs a sentence for each. Batch 2C adds more, so this is no exact list.
	jobCodes := " " + strings.Join(families["storymaker.job_error."], " ") + " "
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./ui -run 'TestStorymakerAppUsesOnlyKeysItHas' -count=1`
Expected: desktop_storymaker_guards_test.go:163: ISSUE_CODES = [... cue_late glyph_missing], want the 34 codes of the contract

- [ ] **Step 3: Implement**

In `ui/js/desktop/apps/storymaker-core.js`, replace:

```js
        return t('storymaker.job_error.unknown');
    }

    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified', 'line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late', 'glyph_missing'];

    function issueTitle(t, code) {
        if (ISSUE_CODES.indexOf(code) >= 0) return t('storymaker.issue.' + code);
```

with:

```js
        return t('storymaker.job_error.unknown');
    }

    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified', 'line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late', 'glyph_missing', 'text_offframe', 'text_unsafe', 'text_contrast', 'flash', 'flash_unverified', 'slow_hook'];

    function issueTitle(t, code) {
        if (ISSUE_CODES.indexOf(code) >= 0) return t('storymaker.issue.' + code);
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./ui -run 'TestStorymakerAppUsesOnlyKeysItHas' -count=1`
Expected: ok  	aurago/ui (TestStorymakerAppUsesOnlyKeysItHas PASS)

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_guards_test.go ui/js/desktop/apps/storymaker-core.js
git commit -m "storymaker: the editor titles the measured issue codes"
```

