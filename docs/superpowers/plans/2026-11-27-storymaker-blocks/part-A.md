# Part A: Stage 1: reliable and reviewable code scenes

> Generated from the drafted and tested commits 70e65ce41..f8bc11664. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Task A1: free fill, check and revise get the build output budget

**Files:**
- Modify: `internal/server/storymaker_agent.go`
- Create: `internal/server/storymaker_agent_budget_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/storymaker_agent_budget_test.go`:

```go
package server

import (
	"testing"

	"aurago/internal/llm"
	"aurago/internal/storymaker"
)

func TestStorymakerAgentOutputTokens(t *testing.T) {
	limits := func(ctx int) llm.ModelLimits { return llm.ModelLimits{ContextWindow: ctx, MaxOutputTokens: 131072} }
	cases := []struct {
		name        string
		limits      llm.ModelLimits
		mode, phase string
		want        int
	}{
		{"free fill 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseFill, 32768},
		{"free check 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseCheck, 32768},
		{"free revise 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseRevise, 32768},
		{"free fill 256k", limits(262144), storymaker.ModeFree, storymaker.PhaseFill, 65536},
		{"free outline 200k", limits(204800), storymaker.ModeFree, storymaker.PhaseOutline, 20480},
		{"catalog fill 200k", limits(204800), storymaker.ModeCatalog, storymaker.PhaseFill, 20480},
		{"catalog fill 256k", limits(262144), storymaker.ModeCatalog, storymaker.PhaseFill, 20480},
		{"free fill small", llm.ModelLimits{ContextWindow: 32768, MaxOutputTokens: 32768}, storymaker.ModeFree, storymaker.PhaseFill, 0},
	}
	for _, tc := range cases {
		if got := storymakerAgentOutputTokens(tc.limits, tc.mode, tc.phase); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run '(?i)(TestStorymakerAgentOutputTokens)' -count=1`
Expected: build failure, undefined storymakerAgentOutputTokens

- [ ] **Step 3: Implement**

In `internal/server/storymaker_agent.go`, replace:

```go
	return phase == storymaker.PhaseOutline || phase == storymaker.PhaseCast || phase == storymaker.PhaseFill
}

// storymakerAgentLoop is one model loop of a job.
type storymakerAgentLoop struct {
	phase   string
```

with:

```go
	return phase == storymaker.PhaseOutline || phase == storymaker.PhaseCast || phase == storymaker.PhaseFill
}

// storymakerAgentOutputTokens is max_tokens of one loop. The free mode's fill, check and revise phases write code and get
// Game Maker's build reserve; every other phase keeps the general reserve.
func storymakerAgentOutputTokens(limits llm.ModelLimits, mode, phase string) int {
	if storymaker.EffectiveMode(mode) == storymaker.ModeFree {
		switch phase {
		case storymaker.PhaseFill, storymaker.PhaseCheck, storymaker.PhaseRevise:
			return gameMakerOutputTokens(limits, gameMakerBuildAnswerTokens)
		}
	}
	return gameMakerOutputTokens(limits, llm.ConservativeOutputTokens)
}

// storymakerAgentLoop is one model loop of a job.
type storymakerAgentLoop struct {
	phase   string
```

In `internal/server/storymaker_agent.go`, replace:

```go
	if !j.route.text && storymakerAgentNeedsToolCall(loop.phase) {
		req.ToolChoice = "required"
	}
	if tokens := gameMakerOutputTokens(gameMakerRouteLimits(&cfg), llm.ConservativeOutputTokens); tokens > 0 {
		req.MaxTokens = tokens
	}
	callCtx := llm.WithStreamAttemptTimeout(ctx, time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
```

with:

```go
	if !j.route.text && storymakerAgentNeedsToolCall(loop.phase) {
		req.ToolChoice = "required"
	}
	if tokens := storymakerAgentOutputTokens(gameMakerRouteLimits(&cfg), j.studioJob.Mode, loop.phase); tokens > 0 {
		req.MaxTokens = tokens
	}
	callCtx := llm.WithStreamAttemptTimeout(ctx, time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run '(?i)(TestStorymakerAgentOutputTokens)' -count=1`
Expected: PASS (8 table rows)

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_agent.go internal/server/storymaker_agent_budget_test.go
git commit -m "storymaker: free fill, check and revise get the build output budget"
```

### Task A2: fill pass 1 tries every scene, pass 2 retries the unfilled ones

**Files:**
- Modify: `internal/server/storymaker_agent.go`
- Create: `internal/server/storymaker_agent_fill_order_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/storymaker_agent_fill_order_test.go`:

```go
package server

import (
	"regexp"
	"slices"
	"testing"

	"aurago/internal/storymaker"

	"github.com/sashabaranov/go-openai"
)

var smFillSceneRe = regexp.MustCompile(`"scene":\{"id":"([a-z0-9-]+)"`)

// A scene that does not get filled in pass 1 is retried only after every other scene had its turn.
func TestStorymakerAgentFillPassesRetryAfterTheOthers(t *testing.T) {
	turns := smScriptHappyTurns()
	turns[storymaker.PhaseFill] = []smScriptTurn{
		{prose: "Ich plane."},
		{prose: "Noch nicht."},
		smScriptStoryCall("put_scene", "s2", smScriptSceneTwo()),
		smScriptStoryCall("put_scene", "s1", smScriptSceneOne()),
	}
	client := &smScriptClient{turns: turns}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	smScriptRequireReady(t, job)
	smScriptRequireFilled(t, smScriptStory(t, env), "s1", "s2")

	var order []string
	for _, req := range client.seen() {
		if smScriptPhase(req) != storymaker.PhaseFill {
			continue
		}
		for _, m := range req.Messages {
			if m.Role != openai.ChatMessageRoleUser {
				continue
			}
			if match := smFillSceneRe.FindStringSubmatch(m.Content); match != nil && (len(order) == 0 || order[len(order)-1] != match[1]) {
				order = append(order, match[1])
			}
		}
	}
	if want := []string{"s1", "s2", "s1"}; !slices.Equal(order, want) {
		t.Fatalf("fill order = %v, want %v", order, want)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run '(?i)(TestStorymakerAgentFillPassesRetryAfterTheOthers)' -count=1`
Expected: job ends incomplete, scene s2 unfilled (order [s1 s2])

- [ ] **Step 3: Implement**

In `internal/server/storymaker_agent.go`, replace:

```go
	advice bool
	// visual marks the one check round in which nothing is open and a model that can see looks at the film.
	visual bool
}

// storymakerAgentJob is the state of one running agent job.
```

with:

```go
	advice bool
	// visual marks the one check round in which nothing is open and a model that can see looks at the film.
	visual bool
	// pass is the fill pass (1..storymakerFillPasses) of a scene loop. 0 means this is not a fill pass.
	pass int
}

// storymakerAgentJob is the state of one running agent job.
```

In `internal/server/storymaker_agent.go`, replace:

```go
// storymakerAgentAttempts bounds one loop: it is restarted at most once, after a model error or an empty end.
const storymakerAgentAttempts = 2

// storymakerAgentStopped is the error of a run that lost its context. The manager turns a requested
// cancel or a shutdown into its own end state, whatever this returns.
func storymakerAgentStopped() *render.JobError {
```

with:

```go
// storymakerAgentAttempts bounds one loop: it is restarted at most once, after a model error or an empty end.
const storymakerAgentAttempts = 2

// storymakerFillPasses is the number of turns a scene gets in the fill order: one in pass 1, one in pass 2 if still unfilled.
const storymakerFillPasses = 2

// storymakerAgentStopped is the error of a run that lost its context. The manager turns a requested
// cancel or a shutdown into its own end state, whatever this returns.
func storymakerAgentStopped() *render.JobError {
```

In `internal/server/storymaker_agent.go`, replace:

```go
// counts as a model error, because advice never blocks the outcome.
func (j *storymakerAgentJob) runPhase(ctx context.Context, loop storymakerAgentLoop) (string, bool) {
	j.enterPhase(loop.phase, loop.sceneID)
	for attempt := 1; attempt <= storymakerAgentAttempts; attempt++ {
		fields := map[string]any{"phase": loop.phase, "attempt": attempt}
		if loop.sceneID != "" {
			fields["scene_id"] = loop.sceneID
		}
```

with:

```go
// counts as a model error, because advice never blocks the outcome.
func (j *storymakerAgentJob) runPhase(ctx context.Context, loop storymakerAgentLoop) (string, bool) {
	j.enterPhase(loop.phase, loop.sceneID)
	attempts := storymakerAgentAttempts
	if loop.pass > 0 {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		number := attempt
		if loop.pass > 0 {
			number = loop.pass
		}
		fields := map[string]any{"phase": loop.phase, "attempt": number}
		if loop.sceneID != "" {
			fields["scene_id"] = loop.sceneID
		}
```

In `internal/server/storymaker_agent.go`, replace:

```go
			return answer, true
		}
		if err != nil {
			j.logger.Warn("[Storymaker] An agent loop failed", "job_id", j.job.ID, "phase", loop.phase, "attempt", attempt, "error", err)
			if attempt == storymakerAgentAttempts && !loop.advice {
				j.noteModelError(err)
			}
		}
```

with:

```go
			return answer, true
		}
		if err != nil {
			j.logger.Warn("[Storymaker] An agent loop failed", "job_id", j.job.ID, "phase", loop.phase, "attempt", number, "error", err)
			last := attempt == storymakerAgentAttempts
			if loop.pass > 0 {
				last = loop.pass == storymakerFillPasses
			}
			if last && !loop.advice {
				j.noteModelError(err)
			}
		}
```

In `internal/server/storymaker_agent.go`, replace:

```go
		j.noteModelError(err)
		return
	}
	for _, sceneID := range progress.Unfilled {
		if ctx.Err() != nil {
			return
		}
		j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseFill, sceneID: sceneID})
		j.refreshProgress(ctx)
	}
	j.runCheckRound(ctx)
}

// publishOutcome decides the outcome and writes it to the job. A story that never got an outline
```

with:

```go
		j.noteModelError(err)
		return
	}
	j.runFillPasses(ctx, progress.Unfilled)
	j.runCheckRound(ctx)
}

// runFillPasses fills the scenes of first in order, once each (pass 1). The scenes that are still unfilled
// then get one more turn in the same order (pass 2), after every other scene had its own.
func (j *storymakerAgentJob) runFillPasses(ctx context.Context, first []string) {
	ids := first
	for pass := 1; pass <= storymakerFillPasses; pass++ {
		for _, sceneID := range ids {
			if ctx.Err() != nil {
				return
			}
			j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseFill, sceneID: sceneID, pass: pass})
			j.refreshProgress(ctx)
		}
		if pass == storymakerFillPasses {
			return
		}
		progress, err := j.studio.Progress(ctx)
		if err != nil {
			j.logger.Warn("[Storymaker] The agent progress could not be read", "job_id", j.job.ID, "error", err)
			j.noteModelError(err)
			return
		}
		ids = nil
		for _, sceneID := range first {
			if slices.Contains(progress.Unfilled, sceneID) {
				ids = append(ids, sceneID)
			}
		}
	}
}

// publishOutcome decides the outcome and writes it to the job. A story that never got an outline
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run '(?i)(TestStorymakerAgentFillPassesRetryAfterTheOthers)' -count=1`
Expected: PASS (order [s1 s2 s1]); (?i)(TestStorymakerAgent|Storymaker.*Agent) group ok

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_agent.go internal/server/storymaker_agent_fill_order_test.go
git commit -m "storymaker: fill pass 1 tries every scene, pass 2 retries the unfilled ones"
```

### Task A3: the JS narration parser ignores closing markers

**Files:**
- Modify: `scripts/storymaker-tests/16-narration.mjs`
- Modify: `ui/js/storymaker/narration.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/16-narration.mjs`, replace:

```js
        assert.equal(SM.ranges.width(got.plain), row.width, 'width of ' + row.input);
    }
});
```

with:

```js
        assert.equal(SM.ranges.width(got.plain), row.width, 'width of ' + row.input);
    }
});

check('narration: {/n} closing markers vanish and set no cue', () => {
    const got = SM.narration.parse('Daten{/1}. Mehr {1}Text {/1}hier');
    assert.equal(got.plain, 'Daten. Mehr Text hier');
    assert.deepEqual(plain(got.cues), { 1: 12 });
    assert.deepEqual(plain(SM.narration.parse('a {/2} b').cues), {});
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: narration: {/n} closing markers vanish and set no cue (plain kept {/1})

- [ ] **Step 3: Implement**

In `ui/js/storymaker/narration.js`, replace:

```js
    // the position in the plain text where it stood.
    function parse(text) {
        var src = String(text === undefined || text === null ? '' : text);
        var plain = '', cues = {}, re = /\{(\d+)\}/g, last = 0, m;
        function append(chunk) {
            chunk = chunk.replace(/\s+/g, ' ');
            if (!plain || plain.charAt(plain.length - 1) === ' ') chunk = chunk.replace(/^ /, '');
```

with:

```js
    // the position in the plain text where it stood.
    function parse(text) {
        var src = String(text === undefined || text === null ? '' : text);
        var plain = '', cues = {}, re = /\{(\/?)(\d+)\}/g, last = 0, m;
        function append(chunk) {
            chunk = chunk.replace(/\s+/g, ' ');
            if (!plain || plain.charAt(plain.length - 1) === ' ') chunk = chunk.replace(/^ /, '');
```

In `ui/js/storymaker/narration.js`, replace:

```js
        }
        while ((m = re.exec(src)) !== null) {
            append(src.slice(last, m.index));
            var n = parseInt(m[1], 10);
            if (cues[n] === undefined) cues[n] = plain.length;
            last = m.index + m[0].length;
        }
        append(src.slice(last));
```

with:

```js
        }
        while ((m = re.exec(src)) !== null) {
            append(src.slice(last, m.index));
            // A closing marker {/n} is dropped like an opening one but records no cue.
            if (m[1] === '') {
                var n = parseInt(m[2], 10);
                if (cues[n] === undefined) cues[n] = plain.length;
            }
            last = m.index + m[0].length;
        }
        append(src.slice(last));
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 268 passed, 0 failed; build:ui + check:ui ok

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/16-narration.mjs ui/js/storymaker/narration.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: the JS narration parser ignores closing markers"
```

### Task A4: the Go parser ignores closing markers

**Files:**
- Modify: `internal/storymaker/cuephrase.go`
- Modify: `internal/storymaker/narration.go`
- Modify: `internal/storymaker/narration_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/narration_test.go`, replace:

```go
		t.Fatalf("got %+v", got)
	}
}
```

with:

```go
		t.Fatalf("got %+v", got)
	}
}

func TestStripMarkersDropsClosingMarkers(t *testing.T) {
	plain, cues := parseMarkers("Daten{/1}. Mehr {1}Text {/1}hier")
	if plain != "Daten. Mehr Text hier" || len(cues) != 1 || cues[1] != 12 {
		t.Fatalf("plain = %q, cues = %v", plain, cues)
	}
	if _, cues := parseMarkers("a {/2} b"); len(cues) != 0 {
		t.Fatalf("a closing marker set a cue: %v", cues)
	}
	var words []string
	for _, tok := range cueTokens("Daten{/1} hier") {
		words = append(words, tok.text)
	}
	if !strings.EqualFold(strings.Join(words, ","), "daten,hier") {
		t.Fatalf("tokens = %v, want daten and hier only", words)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestStripMarkersDropsClosingMarkers)' -count=1`
Expected: plain kept {/1}; cueTokens held the closing marker digits

- [ ] **Step 3: Implement**

In `internal/storymaker/cuephrase.go`, replace:

```go
// without spaces (ucdCJKWord) is a unit of its own, so a phrase may start or end inside such a run.
func cueTokens(text string) []cueToken {
	skip := map[int]int{} // start of a marker -> its end
	for _, m := range narrationMarker.FindAllStringIndex(text, -1) {
		skip[m[0]] = m[1]
	}
	var out []cueToken
```

with:

```go
// without spaces (ucdCJKWord) is a unit of its own, so a phrase may start or end inside such a run.
func cueTokens(text string) []cueToken {
	skip := map[int]int{} // start of a marker -> its end
	for _, m := range narrationAnyMarker.FindAllStringIndex(text, -1) {
		skip[m[0]] = m[1]
	}
	var out []cueToken
```

In `internal/storymaker/narration.go`, replace:

```go
// narrationMarker is the marker regex of narration.js. cuePattern in validate.go is narrower, so it cannot be reused.
var narrationMarker = regexp.MustCompile(`\{(\d+)\}`)

// Blocks lists the spoken blocks of a story in scene order, inside a scene the narration first and the say blocks in
// beat order. silent silences only the scene narration. A say whose actor is off stage still makes a block.
func Blocks(s *Story) []NarrationBlock {
```

with:

```go
// narrationMarker is the marker regex of narration.js. cuePattern in validate.go is narrower, so it cannot be reused.
var narrationMarker = regexp.MustCompile(`\{(\d+)\}`)

// narrationAnyMarker also matches the closing marker {/n}. The parsers drop both; only an opening marker records a cue.
var narrationAnyMarker = regexp.MustCompile(`\{(/?)(\d+)\}`)

// Blocks lists the spoken blocks of a story in scene order, inside a scene the narration first and the say blocks in
// beat order. silent silences only the scene narration. A say whose actor is off stage still makes a block.
func Blocks(s *Story) []NarrationBlock {
```

In `internal/storymaker/narration.go`, replace:

```go
		b.WriteString(chunk)
	}
	last := 0
	for _, m := range narrationMarker.FindAllStringSubmatchIndex(text, -1) {
		appendChunk(text[last:m[0]])
		if n, err := strconv.Atoi(text[m[2]:m[3]]); err == nil {
			if _, seen := cues[n]; !seen {
				cues[n] = UTF16Len(b.String())
			}
		}
		last = m[1]
```

with:

```go
		b.WriteString(chunk)
	}
	last := 0
	for _, m := range narrationAnyMarker.FindAllStringSubmatchIndex(text, -1) {
		appendChunk(text[last:m[0]])
		if m[3] == m[2] { // group 1 empty: an opening marker
			if n, err := strconv.Atoi(text[m[4]:m[5]]); err == nil {
				if _, seen := cues[n]; !seen {
					cues[n] = UTF16Len(b.String())
				}
			}
		}
		last = m[1]
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestStripMarkersDropsClosingMarkers)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/cuephrase.go internal/storymaker/narration.go internal/storymaker/narration_test.go
git commit -m "storymaker: the Go parser ignores closing markers"
```

### Task A5: the Studio removes closing markers when it saves

**Files:**
- Create: `internal/storymaker/studio_markers_test.go`
- Modify: `internal/storymaker/studio_umlauts.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/studio_markers_test.go`:

```go
package storymaker

import "testing"

func TestPutSceneRemovesClosingMarkers(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, nil)
	studio.job.Language = "en"
	studio.SetPhase(PhaseFill, "s1")
	out := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{"environment":"env.office","narration":"Daten{/1}. Mehr {1}Text"}}`)
	if out["status"] == "refused" {
		t.Fatalf("put_scene = %v", out)
	}
	if got := storedScene(t, ps, projectID, "s1").Narration; got != "Daten. Mehr {1}Text" {
		t.Fatalf("stored narration %q", got)
	}

	studio, ps, projectID = newStudioWriteRig(t, nil)
	studio.job.Language = "en"
	studio.SetPhase(PhaseOutline, "")
	out = studioWriteCall(t, studio, ToolStory, "outline",
		`{"data":{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"Eine Firewall pruft jedes Paket."},{"id":"s2","narration":"Sie darf Pakete{/2} durchlassen, die erlaubt sind."},{"id":"s3","narration":"Das ist der ganze Tag."}]}}`)
	if out["status"] != "ok" {
		t.Fatalf("outline = %v", out)
	}
	if got := storedScene(t, ps, projectID, "s2").Narration; got != "Sie darf Pakete durchlassen, die erlaubt sind." {
		t.Fatalf("stored outline narration %q", got)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestPutSceneRemovesClosingMarkers)' -count=1`
Expected: stored narration kept {/1}

- [ ] **Step 3: Implement**

In `internal/storymaker/studio_umlauts.go`, replace:

```go
import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"unicode"
```

with:

```go
import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"unicode"
```

In `internal/storymaker/studio_umlauts.go`, replace:

```go
	return append(list, fix)
}

// repairText restores the umlauts of one text in the job's language and remembers what it repaired for the call's answer.
func (s *Studio) repairText(text string) string {
	fixed, fixes := RepairUmlauts(text, s.job.Language)
	for _, f := range fixes {
		s.pendingRepairs = addUmlautFix(s.pendingRepairs, f)
```

with:

```go
	return append(list, fix)
}

// closingMarker is the {/n} that a model sometimes writes after a cue phrase; the format has no such marker, so no text keeps it.
var closingMarker = regexp.MustCompile(`\{/\d+\}`)

// repairText removes closing markers in every language, then restores the umlauts of one text in the job's language and remembers what it repaired for the call's answer.
func (s *Studio) repairText(text string) string {
	text = closingMarker.ReplaceAllString(text, "")
	fixed, fixes := RepairUmlauts(text, s.job.Language)
	for _, f := range fixes {
		s.pendingRepairs = addUmlautFix(s.pendingRepairs, f)
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestPutSceneRemovesClosingMarkers)' -count=1`
Expected: PASS; Umlaut tests ok

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_markers_test.go internal/storymaker/studio_umlauts.go
git commit -m "storymaker: the Studio removes closing markers when it saves"
```

### Task A6: Go computes the safe box per format

**Files:**
- Create: `internal/storymaker/safebox.go`
- Create: `internal/storymaker/safebox_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/safebox_test.go`:

```go
package storymaker

import "testing"

func TestSafeBoxForPinsTheThreeFormats(t *testing.T) {
	for _, tc := range []struct {
		aspect string
		want   SafeBox
	}{
		{"16:9", SafeBox{96, 54, 1728, 803}},
		{"9:16", SafeBox{691, 125, 493, 581}},
		{"1:1", SafeBox{474, 54, 972, 803}},
		{"", SafeBox{96, 54, 1728, 803}},
		{"4:3", SafeBox{96, 54, 1728, 803}},
	} {
		if got := SafeBoxFor(tc.aspect); got != tc.want {
			t.Errorf("SafeBoxFor(%q) = %+v, want %+v", tc.aspect, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestSafeBoxForPinsTheThreeFormats)' -count=1`
Expected: undefined: SafeBox

- [ ] **Step 3: Implement**

Create `internal/storymaker/safebox.go`:

```go
package storymaker

import "math"

// SafeBox is the drawing area of a code scene in 1920x1080 design pixels for an aspect.
type SafeBox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

const (
	captionBandTop         = 0.794 // top of the two-line caption box, 16:9 and 1:1 (layout.go comment, measured 2026-11-09)
	captionBandTopPortrait = 0.654 // 0.78 box bottom - 0.126 two-line box height; caption sizes are fractions of the height
)

// SafeBoxFor is the part of the 1920x1080 design frame that stays visible in the aspect's cover fit, inside the safe zone and
// above the captions. An unknown aspect is 16:9. ui/js/storymaker/stage.js safeBox repeats it; change both together.
func SafeBoxFor(aspect string) SafeBox {
	w, h, top := 1920.0, 1080.0, captionBandTop
	switch aspect {
	case "9:16":
		w, h, top = 1080, 1920, captionBandTopPortrait
	case "1:1":
		w, h = 1080, 1080
	default:
		aspect = "16:9"
	}
	scale := math.Max(w/1920, h/1080)
	visW, visH := math.Min(1920, w/scale), math.Min(1080, h/scale)
	visX, visY := 960-visW/2, (1080-visH)/2
	z := SafeZone(aspect)
	left := visX + z.Left*visW
	right := visX + (1-z.Right)*visW
	top0 := visY + z.Top*visH
	bottom := math.Min(visY+(1-z.Bottom)*visH, visY+top*visH)
	x := int(math.Ceil(left - 1e-6))
	y := int(math.Ceil(top0 - 1e-6))
	return SafeBox{X: x, Y: y, W: int(math.Floor(right+1e-6)) - x, H: int(math.Floor(bottom+1e-6)) - y}
}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestSafeBoxForPinsTheThreeFormats)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/safebox.go internal/storymaker/safebox_test.go
git commit -m "storymaker: Go computes the safe box per format"
```

### Task A7: SM.stage.safeBox returns the same boxes

**Files:**
- Modify: `scripts/storymaker-tests/02-stage-parts-text.mjs`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/stage.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/02-stage-parts-text.mjs`, replace:

```js
    assert.deepEqual(plain(wide.matrix(win)), [2, 0, 0, 2, 0, 0]);
});

check('parts: shapes become paths with fills, strokes, gradients and token resolution', () => {
    SM.parts.resetUnresolved();
    const ctx = makeContext(null);
```

with:

```js
    assert.deepEqual(plain(wide.matrix(win)), [2, 0, 0, 2, 0, 0]);
});

check('stage: safeBox pins 16:9, 9:16, 1:1 like Go', () => {
    assert.deepEqual(plain(SM.stage.safeBox('16:9')), { x: 96, y: 54, w: 1728, h: 803 });
    assert.deepEqual(plain(SM.stage.safeBox('9:16')), { x: 691, y: 125, w: 493, h: 581 });
    assert.deepEqual(plain(SM.stage.safeBox('1:1')), { x: 474, y: 54, w: 972, h: 803 });
    assert.deepEqual(plain(SM.stage.safeBox('4:3')), { x: 96, y: 54, w: 1728, h: 803 });
});

check('parts: shapes become paths with fills, strokes, gradients and token resolution', () => {
    SM.parts.resetUnresolved();
    const ctx = makeContext(null);
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: SM.stage.safeBox is not a function (268 passed, 1 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/stage.js`, replace:

```js
        return stage;
    }

    SM.stage = {
        create: create, sizeFor: sizeFor, DESIGN_W: DESIGN_W, DESIGN_H: DESIGN_H, SIZES: SIZES,
        DEFAULT_SAFE: DEFAULT_SAFE, DEFAULT_FLOOR: DEFAULT_FLOOR, DEFAULT_SPOTS: DEFAULT_SPOTS, V2_FEET_Y: V2_FEET_Y
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        return stage;
    }

    // Safe zone and top of the caption box per aspect; repeat internal/storymaker/safebox.go and layout.go SafeZone, change both together.
    var SAFE_ZONES = { '9:16': { top: 0.115, right: 0.130, bottom: 0.219, left: 0.056 } };
    var SAFE_ZONE_DEFAULT = { top: 0.05, right: 0.05, bottom: 0.05, left: 0.05 };
    var CAPTION_TOP = { '9:16': 0.654, 'default': 0.794 };

    // safeBox(aspect) = {x, y, w, h}: the drawing area of a code scene in design pixels, inside the visible cover-fit slice and
    // the safe zone and above the captions. An unknown aspect is 16:9. Same numbers and rounding as Go SafeBoxFor.
    function safeBox(aspect) {
        var key = SIZES[aspect] ? aspect : '16:9', size = SIZES[key];
        var scale = Math.max(size[0] / DESIGN_W, size[1] / DESIGN_H);
        var visW = Math.min(DESIGN_W, size[0] / scale), visH = Math.min(DESIGN_H, size[1] / scale);
        var visX = DESIGN_W / 2 - visW / 2, visY = (DESIGN_H - visH) / 2;
        var z = SAFE_ZONES[key] || SAFE_ZONE_DEFAULT, capTop = CAPTION_TOP[key] || CAPTION_TOP['default'];
        var left = visX + z.left * visW, right = visX + (1 - z.right) * visW, top = visY + z.top * visH;
        var bottom = Math.min(visY + (1 - z.bottom) * visH, visY + capTop * visH);
        var x = Math.ceil(left - 1e-6), y = Math.ceil(top - 1e-6);
        return { x: x, y: y, w: Math.floor(right + 1e-6) - x, h: Math.floor(bottom + 1e-6) - y };
    }

    SM.stage = {
        create: create, sizeFor: sizeFor, safeBox: safeBox, DESIGN_W: DESIGN_W, DESIGN_H: DESIGN_H, SIZES: SIZES,
        DEFAULT_SAFE: DEFAULT_SAFE, DEFAULT_FLOOR: DEFAULT_FLOOR, DEFAULT_SPOTS: DEFAULT_SPOTS, V2_FEET_Y: V2_FEET_Y
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 269 passed, 0 failed; build:ui + check:ui ok

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/02-stage-parts-text.mjs ui/js/storymaker/player.bundle.js ui/js/storymaker/stage.js
git commit -m "storymaker: SM.stage.safeBox returns the same boxes"
```

### Task A8: api.safe() in code scenes

**Files:**
- Modify: `scripts/storymaker-tests/06-codescene.mjs`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/06-codescene.mjs`, replace:

```js
    const C = ctx.StorymakerCore;
    const plan = C.planTiming(extra && extra.story || firewall, {}, catalog);
    const sceneId = extra && extra.id || 's3';
    const opts = { sceneId, index: plan.scenes[sceneId].index, catalog, aspect: '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[sceneId].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe };
    return { ctx, C, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}
```

with:

```js
    const C = ctx.StorymakerCore;
    const plan = C.planTiming(extra && extra.story || firewall, {}, catalog);
    const sceneId = extra && extra.id || 's3';
    const opts = { sceneId, index: plan.scenes[sceneId].index, catalog, aspect: extra && extra.aspect || '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[sceneId].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe };
    return { ctx, C, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}
```

In `scripts/storymaker-tests/06-codescene.mjs`, replace:

```js
    assert.equal(more.duration, Math.max(2, 1.3) + 0.5, 'without api.duration: the end of the last motion + 0.5 s');
});

check('code scene API: api.random is seeded, api.cue maps to cue times, errors name the fix', () => {
    const { C } = runScene('export default function scene() {}', 'e');
    const r1 = C.codescene.random(42), r2 = C.codescene.random(42), r3 = C.codescene.random('42');
```

with:

```js
    assert.equal(more.duration, Math.max(2, 1.3) + 0.5, 'without api.duration: the end of the last motion + 0.5 s');
});

check('code scene API: api.safe() returns the safe box of the format, as a fresh object each call', () => {
    const src = `export default function scene(api) {
      const a = api.safe(); a.x = 0; a.w = 1;
      api.text(JSON.stringify(api.safe()), 'title');
    }`;
    const box = aspect => JSON.parse(runScene(src, 'safe', { aspect }).build().entities[0].text);
    assert.deepEqual(box('9:16'), { x: 691, y: 125, w: 493, h: 581 });
    assert.deepEqual(box('16:9'), { x: 96, y: 54, w: 1728, h: 803 });
});

check('code scene API: api.random is seeded, api.cue maps to cue times, errors name the fix', () => {
    const { C } = runScene('export default function scene() {}', 'e');
    const r1 = C.codescene.random(42), r2 = C.codescene.random(42), r3 = C.codescene.random('42');
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker`
Expected: api.safe is not a function (269 passed, 1 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/codescene.js`, replace:

```js
// Storymaker player: the code-scene API. A code scene is `export default function scene(api)` bundled into a classic
// script; it runs once, in its own worker, to build a timeline of tracks (like a GSAP master timeline), and every frame
// evaluates that timeline. API: duration, add, text, shape, tween, camera, random(seed), cue(n), onFrame(fn); chart and
// diagram exist and report not_supported_yet in phase 1. Handle methods take {start, dur, ease}; `start` is seconds
// (or api.cue(n)) from the scene start (the start of the entry transition, like cues and beats). Positions are spot
// names or {x, y} in percent of the safe area. Errors are thrown with a message that names the fix.
```

with:

```js
// Storymaker player: the code-scene API. A code scene is `export default function scene(api)` bundled into a classic
// script; it runs once, in its own worker, to build a timeline of tracks (like a GSAP master timeline), and every frame
// evaluates that timeline. API: duration, safe, add, text, shape, tween, camera, random(seed), cue(n), onFrame(fn); chart and
// diagram exist and report not_supported_yet in phase 1. Handle methods take {start, dur, ease}; `start` is seconds
// (or api.cue(n)) from the scene start (the start of the entry transition, like cues and beats). Positions are spot
// names or {x, y} in percent of the safe area. Errors are thrown with a message that names the fix.
```

In `ui/js/storymaker/codescene.js`, replace:

```js
            if (sec <= 0 || sec > MAX_DURATION) throw new RangeError('duration must be between 0 and ' + MAX_DURATION + ' seconds, got ' + sec);
            state.duration = sec;
        };
        api.add = function (id, o) {
            o = o || {};
            var entry = cat.byId[id];
```

with:

```js
            if (sec <= 0 || sec > MAX_DURATION) throw new RangeError('duration must be between 0 and ' + MAX_DURATION + ' seconds, got ' + sec);
            state.duration = sec;
        };
        // api.safe() = {x, y, w, h}: the drawing area of this format in design pixels, above the captions (a new object per call).
        api.safe = function () { return SM.stage.safeBox(opts.aspect); };
        api.add = function (id, o) {
            o = o || {};
            var entry = cat.byId[id];
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker`
Expected: 270 passed, 0 failed; build:ui + check:ui ok

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/06-codescene.mjs ui/js/storymaker/codescene.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: api.safe() in code scenes"
```

### Task A9: the free fill packet carries code_rev, safe_box and the API card

**Files:**
- Create: `internal/storymaker/studio_apicard.go`
- Modify: `internal/storymaker/studio_free_packet_test.go`
- Modify: `internal/storymaker/studio_packet.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_free_packet_test.go`, replace:

```go
		t.Fatalf("next_scene = %v", packet["next_scene"])
	}
}
```

with:

```go
		t.Fatalf("next_scene = %v", packet["next_scene"])
	}
}

func TestFreeFillPacketCarriesCodeRevCardAndSafeBox(t *testing.T) {
	ctx := context.Background()
	s, ps, projectID := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")
	packet, err := s.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := ps.LoadStory(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := DecodeStory(story, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	ApplyDefaults(&st)
	if packet["code_rev"] != MustNotExist {
		t.Fatalf("code_rev = %v, want %q", packet["code_rev"], MustNotExist)
	}
	if box, _ := packet["safe_box"].(SafeBox); box != SafeBoxFor(st.Format.Aspect) {
		t.Fatalf("safe_box = %v, want %+v", packet["safe_box"], SafeBoxFor(st.Format.Aspect))
	}
	card, _ := packet["api_card"].(string)
	if !strings.Contains(card, "api.safe()") || !strings.Contains(card, "Palette of this film: $bg #") || len(card) > 1800 {
		t.Fatalf("api_card has %d bytes: %s", len(card), card)
	}
	if next, _ := packet["next"].(string); !strings.HasPrefix(next, "put_code with expected_rev = code_rev") {
		t.Fatalf("next = %q", next)
	}

	put := studioWriteCall(t, s, ToolStory, "put_code", `{"scene_id":"s1","expected_rev":"`+MustNotExist+`","data":{"source":"export default function scene(api) { api.duration(5); }"}}`)
	if put["status"] != "ok" || put["rev"] == nil {
		t.Fatalf("put_code with code_rev and no get_code = %v", put)
	}
	again, err := s.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if again["code_rev"] != put["rev"] {
		t.Fatalf("code_rev = %v, want the rev of the put_code answer %v", again["code_rev"], put["rev"])
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeFillPacketCarriesCodeRevCardAndSafeBox|TestFreePacketsCarryNoCatalogOrCast)' -count=1`
Expected: code_rev = <nil>, want "-" (fields missing)

- [ ] **Step 3: Implement**

Create `internal/storymaker/studio_apicard.go`:

```go
package storymaker

import (
	"fmt"
	"strings"
)

// freeAPICardText is the code API in short, for the context of a free fill. It must stay within 1800 bytes with the two lines
// FreeAPICard adds; the code reference holds the long form.
const freeAPICardText = `API card for this code scene. Read storymaker_catalog reference topic code only when this card is not enough.
export default function scene(api) { ... } runs once and builds the scene; each frame is a pure function of t.
timing = {start, dur, ease}; ease is linear, ease-in, ease-out, ease-in-out, back-out or bounce-out. api.cue(n) is the second of narration marker {n}.
api.duration(seconds)
api.safe() returns {x, y, w, h}: the drawing area of this format in 1920x1080 design pixels, above the captions.
api.text(text, 'title' or 'label', {at}) returns a handle.
api.shape('rect', 'circle' or 'line', {at, w, h, fill, stroke}) returns a handle; w and h are design pixels.
handle.moveTo(at, timing), fadeIn(timing), fadeOut(timing), scaleTo(scale, timing), rotateTo(degrees, timing)
api.tween(handle, {opacity, x, y, scale, rotation, width, height}, timing)
api.camera.zoom(factor, timing), api.camera.pan(at, timing), api.camera.shake(amount, timing)
api.random('seed') returns a seeded random function: call it while building, never in onFrame.
api.onFrame(function (t, g) { ... }): g.save, restore, translate, rotate, scale, alpha, fill, stroke, rect, strokeRect, circle, line, path(svgPath, 'fill', 'stroke' or 'both'), text(text, x, y, {size, weight, align, color}). g draws in design pixels.
at is {x, y} in percent of the safe area, or a spot: left, center, right.
Colours: the palette tokens $bg, $bg2, $fg, $accent, $accent2, $text, $outline, or #rrggbb.
Never: api.add, Math.random, Date, timers, imports, the DOM, or state that changes from frame to frame.`

// FreeAPICard is the card of a free fill packet: freeAPICardText, a palette line and a safe_box line, joined by "\n". The
// palette line names the tokens of the film's palette in catalog order and is left out when there are none.
func FreeAPICard(tokens map[string]string, box SafeBox) string {
	lines := []string{freeAPICardText}
	var parts []string
	for _, name := range paletteTokenNames {
		if hex := tokens[name]; hex != "" {
			parts = append(parts, "$"+name+" "+hex)
		}
	}
	if len(parts) > 0 {
		lines = append(lines, "Palette of this film: "+strings.Join(parts, ", "))
	}
	lines = append(lines, fmt.Sprintf("safe_box: x %d, y %d, w %d, h %d", box.X, box.Y, box.W, box.H))
	return strings.Join(lines, "\n")
}
```

In `internal/storymaker/studio_packet.go`, replace:

```go

import (
	"context"
	"fmt"
	"math"
	"time"
)
```

with:

```go

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"time"
)
```

In `internal/storymaker/studio_packet.go`, replace:

```go
	}
	packet["max_code_bytes"] = StudioCodeBytes
	packet["open_issues"] = packetIssues(story, deps, sc.ID, job.Mode)
	packet["next"] = `get_code, then put_code with expected_rev and data.source, then put_scene with data {"kind":"code"}`
	if !job.VoiceAvailable {
		packet["note"] = packetFreeNoVoiceNote
	}
```

with:

```go
	}
	packet["max_code_bytes"] = StudioCodeBytes
	packet["open_issues"] = packetIssues(story, deps, sc.ID, job.Mode)
	codeRev := MustNotExist
	if _, etag, err := deps.Projects.LoadCodeSource(ctx, job.ProjectID, sceneID); err == nil {
		codeRev = studioRev(etag)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	box := SafeBoxFor(story.Format.Aspect)
	var tokens map[string]string
	paletteName := story.Style.Palette
	if paletteName == "" {
		paletteName = DefaultPalette // ApplyDefaults has not run on a story that never named one
	}
	if deps.Catalog != nil {
		// Catalog.Palettes is not filled by LoadCatalog; the palette entry keeps its manifest row.
		if e, ok := deps.Catalog.Lookup("palette." + paletteName); ok {
			var p Palette
			if json.Unmarshal(e.raw, &p) == nil {
				tokens = p.Tokens
			}
		}
	}
	packet["code_rev"] = codeRev
	packet["safe_box"] = box
	packet["api_card"] = FreeAPICard(tokens, box)
	packet["next"] = `put_code with expected_rev = code_rev and data.source (get_code only to read a saved source or after rev_mismatch), then put_scene with data {"kind":"code"}`
	if !job.VoiceAvailable {
		packet["note"] = packetFreeNoVoiceNote
	}
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestFreeFillPacketCarriesCodeRevCardAndSafeBox|TestFreePacketsCarryNoCatalogOrCast)' -count=1`
Expected: both PASS; go test ./internal/storymaker ok

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_apicard.go internal/storymaker/studio_free_packet_test.go internal/storymaker/studio_packet.go
git commit -m "storymaker: the free fill packet carries code_rev, safe_box and the API card"
```

### Task A10: the free profile reads the card instead of the reference

**Files:**
- Modify: `internal/agent/native_tools_storymaker.go`
- Modify: `internal/server/storymaker_agent_profile.go`
- Modify: `internal/server/storymaker_agent_profile_free.go`
- Modify: `internal/server/storymaker_agent_profile_free_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/server/storymaker_agent_profile_free_test.go`, replace:

```go
	if !strings.Contains(fill.SystemPrompt(), "## Fill one scene") || !strings.Contains(fill.SystemPrompt(), "put_code") {
		t.Error("the free fill workflow must keep its heading and name put_code")
	}
	if !strings.Contains(fill.Name(), "storymaker/v5/free/fill/") {
		t.Errorf("profile name = %q", fill.Name())
	}
	catalog, _ := storymakerAgentProfile(storymaker.PhaseFill, false, false, names)
	if strings.Contains(catalog.SystemPrompt(), "no catalog") || strings.Contains(catalog.Name(), "/free/") {
		t.Error("the catalog profile must not change")
	}
	nudge := storymakerRecoveryNudgeFor(storymaker.ModeFree, storymaker.PhaseFill)(false)
	if !strings.Contains(nudge, "get_code") || strings.Contains(nudge, "search") {
		t.Errorf("free fill nudge = %q", nudge)
	}
}
```

with:

```go
	if !strings.Contains(fill.SystemPrompt(), "## Fill one scene") || !strings.Contains(fill.SystemPrompt(), "put_code") {
		t.Error("the free fill workflow must keep its heading and name put_code")
	}
	if !strings.Contains(fill.Name(), "storymaker/v6/free/fill/") {
		t.Errorf("profile name = %q", fill.Name())
	}
	if prompt := fill.SystemPrompt(); !strings.Contains(prompt, "code_rev") || !strings.Contains(prompt, "api_card") || strings.Contains(prompt, "before the first scene") {
		t.Error("the free fill prompt must name code_rev and api_card and must not send the model to the reference first")
	}
	catalog, _ := storymakerAgentProfile(storymaker.PhaseFill, false, false, names)
	if strings.Contains(catalog.SystemPrompt(), "no catalog") || strings.Contains(catalog.Name(), "/free/") {
		t.Error("the catalog profile must not change")
	}
	nudge := storymakerRecoveryNudgeFor(storymaker.ModeFree, storymaker.PhaseFill)(false)
	if !strings.Contains(nudge, "put_code") || !strings.Contains(nudge, "code_rev") || strings.Contains(nudge, "search") {
		t.Errorf("free fill nudge = %q", nudge)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run '(?i)(TestStorymakerFreeProfilesTellTheFreeStory)' -count=1`
Expected: profile name v5, prompt without code_rev/api_card, nudge shows get_code

- [ ] **Step 3: Implement**

In `internal/agent/native_tools_storymaker.go`, replace:

```go
func storymakerFreeSceneProperties() map[string]interface{} {
	return map[string]interface{}{
		"kind":       storymakerEnum("code"),
		"source":     map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes, "description": "put_code only; first read get_code and the code reference"},
		"narration":  storymakerTyped("string"),
		"transition": storymakerEnum("cut", "fade", "slide-left", "wipe"),
		"duration":   prop("string", "auto or seconds"),
```

with:

```go
func storymakerFreeSceneProperties() map[string]interface{} {
	return map[string]interface{}{
		"kind":       storymakerEnum("code"),
		"source":     map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes, "description": "put_code only; expected_rev is code_rev from the context"},
		"narration":  storymakerTyped("string"),
		"transition": storymakerEnum("cut", "fade", "slide-left", "wipe"),
		"duration":   prop("string", "auto or seconds"),
```

In `internal/server/storymaker_agent_profile.go`, replace:

```go
	free := storymaker.EffectiveMode(mode) == storymaker.ModeFree
	workflows, ground, rules, style, guide, name := storymakerPhaseWorkflows, storymakerGroundRules, storymakerHouseRules, storymakerHouseStyle, storymakerAnimationGuide, "storymaker/v5/"+phase
	if free {
		workflows, ground, rules, style, guide, name = storymakerFreePhaseWorkflows, storymakerFreeGroundRules, storymakerFreeHouseRules, storymakerFreeHouseStyle, storymakerFreeCodeGuide, "storymaker/v5/free/"+phase
	}
	workflow, ok := workflows[phase]
	if !ok {
```

with:

```go
	free := storymaker.EffectiveMode(mode) == storymaker.ModeFree
	workflows, ground, rules, style, guide, name := storymakerPhaseWorkflows, storymakerGroundRules, storymakerHouseRules, storymakerHouseStyle, storymakerAnimationGuide, "storymaker/v5/"+phase
	if free {
		workflows, ground, rules, style, guide, name = storymakerFreePhaseWorkflows, storymakerFreeGroundRules, storymakerFreeHouseRules, storymakerFreeHouseStyle, storymakerFreeCodeGuide, "storymaker/v6/free/"+phase
	}
	workflow, ok := workflows[phase]
	if !ok {
```

In `internal/server/storymaker_agent_profile_free.go`, replace:

```go
- Banned: bullet lists, three equal cards in a row, glow and neon looks, a title that keeps moving after it has landed.`

const storymakerFreeCodeGuide = `## Drawing and animation in code
- Read storymaker_catalog operation reference, topic code before the first scene: it is the only reference you need.
- get_code reads the bound scene's source and code rev. put_code takes expected_rev from that answer and data.source only, within 8 KiB. Invalid code keeps the saved source; a stale rev needs a fresh read.
- Code uses export default function scene(api), handles and tweens, and the restricted onFrame(t,g) drawing facade. Draw with shapes, text and g.path SVG path data. Use explicit scene time and fixed seeded constants, never counters or mutable state across frames, clocks, timers, imports, DOM, network, raw PixiJS or GSAP. Keep loops bounded. chart and diagram helpers are unsupported.
- The {1}, {2} markers of the narration give api.cue times: start a shape's motion on its cue, so the picture lands on the word.
- After changes run storymaker_check and repair its errors. A compile failure is fixed in the source; never bypass isolation or relax deadlines.`
```

with:

```go
- Banned: bullet lists, three equal cards in a row, glow and neon looks, a title that keeps moving after it has landed.`

const storymakerFreeCodeGuide = `## Drawing and animation in code
- The fill context has api_card (the code API in short), code_rev and safe_box. Read storymaker_catalog operation reference, topic code only when the card is not enough.
- put_code takes expected_rev and data.source only, within 8 KiB. In a fill, expected_rev is code_rev from the context; get_code reads a saved source, and a fresh rev after rev_mismatch. Invalid code keeps the saved source.
- Keep every drawing inside safe_box (api.safe()); the captions sit below it.
- Code uses export default function scene(api), handles and tweens, and the restricted onFrame(t,g) drawing facade. Draw with shapes, text and g.path SVG path data. Use explicit scene time and fixed seeded constants, never counters or mutable state across frames, clocks, timers, imports, DOM, network, raw PixiJS or GSAP. Keep loops bounded. chart and diagram helpers are unsupported.
- The {1}, {2} markers of the narration give api.cue times: start a shape's motion on its cue, so the picture lands on the word.
- After changes run storymaker_check and repair its errors. A compile failure is fixed in the source; never bypass isolation or relax deadlines.`
```

In `internal/server/storymaker_agent_profile_free.go`, replace:

```go
4. If the answer says the text is too long, shorten it and call outline again. An accepted outline ends this phase: stop.`

const storymakerFreeFillWorkflow = `## Fill one scene
The context names one scene, plus previous_scene, next_scene, the aspect and max_code_bytes. Draw this scene in code.
1. Call storymaker_story with operation get_code and the scene_id. Its rev is the expected_rev of put_code.
2. Call put_code with the scene_id, expected_rev and data.source: export default function scene(api) { ... } within max_code_bytes. Choose the one motif of this scene from its narration, draw it with shapes, text and g.path, and move it on the cue markers.
3. If put_code refuses, fix the listed lines and call it again with the same rev.
4. Activate the scene with put_scene, the scene_id and data {"kind":"code"}. The narration stays. An optional dry_run checks without saving.
A saved scene ends this phase: stop.`

const storymakerFreeCheckWorkflow = `## Check and fix
```

with:

```go
4. If the answer says the text is too long, shorten it and call outline again. An accepted outline ends this phase: stop.`

const storymakerFreeFillWorkflow = `## Fill one scene
The context names one scene, plus previous_scene, next_scene, the aspect, code_rev, safe_box, api_card and max_code_bytes. Draw this scene in code.
1. Call storymaker_story with operation put_code, the scene_id, expected_rev set to code_rev and data.source: export default function scene(api) { ... } within max_code_bytes. Choose the one motif of this scene from its narration, draw it inside safe_box and move it on the cue markers.
2. If put_code refuses, fix the listed lines and call it again; after rev_mismatch call get_code for the fresh rev.
3. Activate the scene with put_scene, the scene_id and data {"kind":"code"}. The narration stays. An optional dry_run checks without saving.
A saved scene ends this phase: stop.`

const storymakerFreeCheckWorkflow = `## Check and fix
```

In `internal/server/storymaker_agent_profile_free.go`, replace:

```go
var storymakerFreeToolExamples = map[string][]string{
	storymaker.PhaseOutline: storymakerToolExamples[storymaker.PhaseOutline],
	storymaker.PhaseFill: {
		`{"action":"storymaker_story","operation":"get_code","scene_id":"s1"}`,
		`{"action":"storymaker_story","operation":"put_code","scene_id":"s1","expected_rev":"<rev of get_code>","data":{"source":"export default function scene(api) { api.duration(6); }"}}`,
		`{"action":"storymaker_story","operation":"put_scene","scene_id":"s1","data":{"kind":"code"}}`,
	},
	storymaker.PhaseCheck: {
		`{"action":"storymaker_check","operation":"validate"}`,
```

with:

```go
var storymakerFreeToolExamples = map[string][]string{
	storymaker.PhaseOutline: storymakerToolExamples[storymaker.PhaseOutline],
	storymaker.PhaseFill: {
		`{"action":"storymaker_story","operation":"put_code","scene_id":"s1","expected_rev":"<code_rev of the context>","data":{"source":"export default function scene(api) { api.duration(6); }"}}`,
		`{"action":"storymaker_story","operation":"put_scene","scene_id":"s1","data":{"kind":"code"}}`,
		`{"action":"storymaker_story","operation":"get_code","scene_id":"s1"}`,
	},
	storymaker.PhaseCheck: {
		`{"action":"storymaker_check","operation":"validate"}`,
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run '(?i)(TestStorymakerFreeProfilesTellTheFreeStory)' -count=1`
Expected: PASS; go test ./internal/server ./internal/agent -run (?i)storymaker ok

- [ ] **Step 5: Commit**

```bash
git add internal/agent/native_tools_storymaker.go internal/server/storymaker_agent_profile.go internal/server/storymaker_agent_profile_free.go internal/server/storymaker_agent_profile_free_test.go
git commit -m "storymaker: the free profile reads the card instead of the reference"
```

### Task A11: the code references name the safe area, not x=710..1210

**Files:**
- Modify: `internal/storymaker/reference/code-free.md`
- Modify: `internal/storymaker/reference/code.md`
- Modify: `internal/storymaker/studio_free_reference_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_free_reference_test.go`, replace:

```go
		}
	}
}
```

with:

```go
		}
	}
}

func TestCodeReferencesNameTheSafeArea(t *testing.T) {
	for name, text := range map[string]string{"code.md": codeReference, "code-free.md": codeReferenceFree} {
		if strings.Contains(text, "710..1210") {
			t.Errorf("%s still names the fixed range x=710..1210", name)
		}
		if !strings.Contains(text, "api.safe()") {
			t.Errorf("%s does not name api.safe()", name)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestCodeReferencesNameTheSafeArea|TestFree.*Reference)' -count=1`
Expected: both references contain 710..1210 and lack api.safe()

- [ ] **Step 3: Implement**

In `internal/storymaker/reference/code-free.md`, replace:

```markdown
1. Pick the one concrete motif of the scene from its narration and design that specific object, organism or process element, not a generic card with its name.
2. Author compact SVG **path data** as constant strings, for example `const shell = 'M-80-60H80Q100-60 100-40V60H-100V-40Q-100-60-80-60Z';`. Draw with `g.path(shell, 'fill')`, `'stroke'` or `'both'`; set colour/width with `g.fill` and `g.stroke`. This uses SVG's M/L/H/V/C/Q/A/Z commands. It does not load `<svg>` markup, viewBox, CSS, filters, images, external URLs or .svg files. Flatten transforms into path coordinates or apply g.translate/scale/rotate inside balanced save/restore calls.
3. Give the motif a recognisable silhouette, consistent line weights and a small palette that the whole film shares. Layer an offset low-opacity shadow, the body, inset surfaces and a few functional details. Keep fine detail readable at output size. Keep labels as api.text/g.text in the job language, not outlined paths.
4. Use api.shape for backdrops, bars and rules, and g.path for the main motif. onFrame draws above retained shapes and text, so leave clear space and do not paint a full-screen rectangle over them. For all three formats with a stationary camera, keep key drawing roughly inside x=710..1210, y=110..800 in design coordinates; leave the lower region for subtitles. api.shape and api.text use safe-area percentages, while g uses design pixels. A camera move changes this crop and needs review.
5. Reveal the main object when named: use `api.cue(n)` for existing markers and derive alpha, position or flow directly from t; use fixed times when the narration has no marker. Keep loops small and all path constants immutable. Match the narration duration. Save with get_code/put_code, activate with patch_scene and {"kind":"code"}, then check.

The executable example below is one complete free scene; change its labels to the job language. Layout still needs visual review in the intended format; a successful check is not pixel-level approval.
```

with:

```markdown
1. Pick the one concrete motif of the scene from its narration and design that specific object, organism or process element, not a generic card with its name.
2. Author compact SVG **path data** as constant strings, for example `const shell = 'M-80-60H80Q100-60 100-40V60H-100V-40Q-100-60-80-60Z';`. Draw with `g.path(shell, 'fill')`, `'stroke'` or `'both'`; set colour/width with `g.fill` and `g.stroke`. This uses SVG's M/L/H/V/C/Q/A/Z commands. It does not load `<svg>` markup, viewBox, CSS, filters, images, external URLs or .svg files. Flatten transforms into path coordinates or apply g.translate/scale/rotate inside balanced save/restore calls.
3. Give the motif a recognisable silhouette, consistent line weights and a small palette that the whole film shares. Layer an offset low-opacity shadow, the body, inset surfaces and a few functional details. Keep fine detail readable at output size. Keep labels as api.text/g.text in the job language, not outlined paths.
4. Use api.shape for backdrops, bars and rules, and g.path for the main motif. onFrame draws above retained shapes and text, so leave clear space and do not paint a full-screen rectangle over them. For all three formats with a stationary camera, keep key drawing inside api.safe() (the safe_box of the context), which is the visible part of the format above the captions. api.shape and api.text use safe-area percentages, while g uses design pixels. A camera move changes this crop and needs review.
5. Reveal the main object when named: use `api.cue(n)` for existing markers and derive alpha, position or flow directly from t; use fixed times when the narration has no marker. Keep loops small and all path constants immutable. Match the narration duration. Save with get_code/put_code, activate with patch_scene and {"kind":"code"}, then check.

The executable example below is one complete free scene; change its labels to the job language. Layout still needs visual review in the intended format; a successful check is not pixel-level approval.
```

In `internal/storymaker/reference/code.md`, replace:

```markdown
1. Search the catalog for the concrete object and a synonym. Use an existing asset when it conveys the intended meaning. Spelling alternatives are not evidence that a motif is appropriate. For a missing device, organism or process element, design that specific motif rather than a generic card with its name.
2. Author compact SVG **path data** as constant strings, for example `const shell = 'M-80-60H80Q100-60 100-40V60H-100V-40Q-100-60-80-60Z';`. Draw with `g.path(shell, 'fill')`, `'stroke'` or `'both'`; set colour/width with `g.fill` and `g.stroke`. This uses SVG's M/L/H/V/C/Q/A/Z commands. It does not load `<svg>` markup, viewBox, CSS, filters, images, external URLs or .svg files. Flatten transforms into path coordinates or apply g.translate/scale/rotate inside balanced save/restore calls.
3. Give the motif a recognisable silhouette, consistent line weights and a small palette suited to the scene. Layer an offset low-opacity shadow, the body, inset surfaces and a few functional details. Keep fine detail readable at output size; avoid photorealistic claims for a small vector illustration. Keep labels as api.text/g.text in the job language, not outlined paths.
4. Combine the new drawing with relevant `api.add` catalog props. `onFrame` draws above those props, so leave clear space and do not paint a full-screen rectangle over them. For all three formats with a stationary camera, keep key custom drawing roughly inside x=710..1210, y=110..800 in design coordinates; leave the lower region for subtitles. Catalog handles use safe-area percentages, while g uses design pixels. A camera move changes this crop and needs review.
5. Reveal the main object when named. In a narrated scene use `api.cue(n)` for existing markers, and derive alpha, position or flow directly from t; for silent examples use fixed times. Keep loops small and all path constants immutable. Match the narration duration. Save with get_code/put_code, activate with put_scene kind code, then check. The paths remain editable in the saved scene source and use the same renderer for preview and export.

The following two executable examples show motion and SVG asset composition. The SVG example uses a custom charge controller with catalog solar-panel and battery props; it is a narrated illustration, not an on-screen speaking character. Change labels to the job language and IDs only to verified catalog IDs. Preserve existing dialogue scenes and user code during revisions. Code-scene layout still needs visual review in the intended format; a successful check is not pixel-level approval.
```

with:

```markdown
1. Search the catalog for the concrete object and a synonym. Use an existing asset when it conveys the intended meaning. Spelling alternatives are not evidence that a motif is appropriate. For a missing device, organism or process element, design that specific motif rather than a generic card with its name.
2. Author compact SVG **path data** as constant strings, for example `const shell = 'M-80-60H80Q100-60 100-40V60H-100V-40Q-100-60-80-60Z';`. Draw with `g.path(shell, 'fill')`, `'stroke'` or `'both'`; set colour/width with `g.fill` and `g.stroke`. This uses SVG's M/L/H/V/C/Q/A/Z commands. It does not load `<svg>` markup, viewBox, CSS, filters, images, external URLs or .svg files. Flatten transforms into path coordinates or apply g.translate/scale/rotate inside balanced save/restore calls.
3. Give the motif a recognisable silhouette, consistent line weights and a small palette suited to the scene. Layer an offset low-opacity shadow, the body, inset surfaces and a few functional details. Keep fine detail readable at output size; avoid photorealistic claims for a small vector illustration. Keep labels as api.text/g.text in the job language, not outlined paths.
4. Combine the new drawing with relevant `api.add` catalog props. `onFrame` draws above those props, so leave clear space and do not paint a full-screen rectangle over them. For all three formats with a stationary camera, keep key custom drawing inside api.safe() (the safe_box of the context), which is the visible part of the format above the captions. Catalog handles use safe-area percentages, while g uses design pixels. A camera move changes this crop and needs review.
5. Reveal the main object when named. In a narrated scene use `api.cue(n)` for existing markers, and derive alpha, position or flow directly from t; for silent examples use fixed times. Keep loops small and all path constants immutable. Match the narration duration. Save with get_code/put_code, activate with put_scene kind code, then check. The paths remain editable in the saved scene source and use the same renderer for preview and export.

The following two executable examples show motion and SVG asset composition. The SVG example uses a custom charge controller with catalog solar-panel and battery props; it is a narrated illustration, not an on-screen speaking character. Change labels to the job language and IDs only to verified catalog IDs. Preserve existing dialogue scenes and user code during revisions. Code-scene layout still needs visual review in the intended format; a successful check is not pixel-level approval.
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestCodeReferencesNameTheSafeArea|TestFree.*Reference)' -count=1`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/reference/code-free.md internal/storymaker/reference/code.md internal/storymaker/studio_free_reference_test.go
git commit -m "storymaker: the code references name the safe area, not x=710..1210"
```

### Task A12: code scenes get a layout block with a hold frame

**Files:**
- Modify: `internal/storymaker/layout.go`
- Modify: `internal/storymaker/layout_test.go`
- Modify: `internal/storymaker/stills.go`
- Modify: `internal/storymaker/studio_check.go`
- Modify: `scripts/storymaker-tests/30-layout.mjs`
- Modify: `ui/js/storymaker/layout.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/layout_test.go`, replace:

```go
		t.Fatal("no layout, no findings")
	}
}
```

with:

```go
		t.Fatal("no layout, no findings")
	}
}

func TestLayoutIssuesSkipStaticForCodeScenes(t *testing.T) {
	code := LayoutScene{ID: "c", Index: 1, HoldS: 2, Code: true, Static: true, Actors: []LayoutActor{}, Props: []LayoutProp{}, Texts: []LayoutText{}}
	if got := LayoutIssues([]LayoutScene{code}, LayoutOptions{}); len(got) != 0 {
		t.Fatalf("a code scene reported %v", layoutCodes(got))
	}
	code.Code = false
	if got := layoutCodes(LayoutIssues([]LayoutScene{code}, LayoutOptions{})); !slices.Equal(got, []string{CodeSceneStatic}) {
		t.Fatalf("a storyboard scene that is static reported %v", got)
	}
}
```

In `scripts/storymaker-tests/30-layout.mjs`, replace:

```js
    assert.ok(hero.scale > 1, 'and carries the scale that makes it so');
});

check('layout: no captions, no caption box; a scene without motion is static; code scenes are left out', () => {
    assert.equal(measure(film({ captions: false }))[0].caption, null);
    const quiet = measure(film({}, { beats: [] }))[0];
    assert.equal(quiet.static, true, 'an actor that only stands there');
    const two = film();
    two.scenes.push({ id: 's2', kind: 'code', narration: 'Ein Code.' });
    const tl = SM.compile(two, {}, t.catalog, { codeScenes: { s2: { duration: 3 } } }).timeline;
    const blocks = SM.layout.measure(tl, renderAssets());
    assert.deepEqual(plain(blocks.map(b => b.id)), ['s1']);
});

check('layout: the compile report is unchanged by the layout run (the golden stays valid)', () => {
```

with:

```js
    assert.ok(hero.scale > 1, 'and carries the scale that makes it so');
});

check('layout: no captions, no caption box; a scene without motion is static', () => {
    assert.equal(measure(film({ captions: false }))[0].caption, null);
    const quiet = measure(film({}, { beats: [] }))[0];
    assert.equal(quiet.static, true, 'an actor that only stands there');
});

check('layout: code scenes get a block with code true', () => {
    const two = film();
    two.scenes.push({ id: 's2', kind: 'code', narration: 'Ein Code.' });
    const tl = SM.compile(two, {}, t.catalog, { codeScenes: { s2: { duration: 3 } } }).timeline;
    const blocks = SM.layout.measure(tl, renderAssets());
    assert.equal(blocks.length, 2);
    const [story, code] = blocks;
    assert.equal(story.code, false);
    assert.equal(code.id, 's2');
    assert.equal(code.code, true);
    assert.equal(code.static, false);
    assert.deepEqual(plain([code.actors, code.props, code.texts]), [[], [], []]);
    const sc = tl.scenes[1], D = (sc.end_frame - sc.start_frame) / tl.fps;
    near(code.hold_s, 0.7 * D, 0.05, 'the code hold frame is 70 % of the scene');
});

check('layout: the compile report is unchanged by the layout run (the golden stays valid)', () => {
```

- [ ] **Step 2: Run it and see it fail**

Run: `npm run test:storymaker; go test ./internal/storymaker -run '(?i)(TestLayoutIssuesSkipStaticForCodeScenes)' -count=1`
Expected: JS: layout: code scenes get a block with code true failed (1 block); Go: unknown field Code in LayoutScene (build failed)

- [ ] **Step 3: Implement**

In `internal/storymaker/layout.go`, replace:

```go
	Box LayoutBox `json:"box"`
}

// LayoutScene is the measured layout of one storyboard scene at its hold frame. Caption is the burned caption of that
// moment, nil when none is on screen. Missing counts boxes the player could not measure for lack of art. Static says that
// nothing on the stage changed between the first frame and the hold frame, apart from the idle cycle.
type LayoutScene struct {
```

with:

```go
	Box LayoutBox `json:"box"`
}

// LayoutScene is the measured layout of one scene at its hold frame. A code scene (Code) has no boxes of its own, because its
// raw drawing is never measured; it is listed so that stills, the contact sheet and the checks see its hold frame. Caption is the burned caption of that
// moment, nil when none is on screen. Missing counts boxes the player could not measure for lack of art. Static says that
// nothing on the stage changed between the first frame and the hold frame, apart from the idle cycle.
type LayoutScene struct {
```

In `internal/storymaker/layout.go`, replace:

```go
	Caption *LayoutBox    `json:"caption"`
	Missing int           `json:"missing"`
	Static  bool          `json:"static"`
}

// The thresholds of the layout findings, in fractions of the frame. They come from the geometry measured on 2026-11-09 and
```

with:

```go
	Caption *LayoutBox    `json:"caption"`
	Missing int           `json:"missing"`
	Static  bool          `json:"static"`
	Code    bool          `json:"code"`
}

// The thresholds of the layout findings, in fractions of the frame. They come from the geometry measured on 2026-11-09 and
```

In `internal/storymaker/layout.go`, replace:

```go
				}
			}
		}
		if sc.Static {
			add(CodeSceneStatic, fmt.Sprintf("Nothing shows, moves or changes in scene %s.", sc.ID),
				`Add a show beat with a prop and a gesture beat with "with":"prev" using patch_scene and the whole beats list.`)
		}
```

with:

```go
				}
			}
		}
		if sc.Static && !sc.Code {
			add(CodeSceneStatic, fmt.Sprintf("Nothing shows, moves or changes in scene %s.", sc.ID),
				`Add a show beat with a prop and a gesture beat with "with":"prev" using patch_scene and the whole beats list.`)
		}
```

In `internal/storymaker/stills.go`, replace:

```go
// LookAlikeIssues finds runs of three or more stills of neighbouring scenes whose neighbours differ in at most
// lookAlikeMaxDistance bits and reports each run once, at its third scene: that is the one to change. The fix asks for another
// environment, another framing and the actor on the other side, at every second scene of the run. The list of stills leaves out
// the scenes without a measured layout (code scenes, scenes whose hold frame is out of range), so two successive stills are
// neighbours only when their story indexes are consecutive: a gap ends the run, because the scenes across it are not seen next
// to each other in the film.
func LookAlikeIssues(stills []StillInfo) []Issue {
```

with:

```go
// LookAlikeIssues finds runs of three or more stills of neighbouring scenes whose neighbours differ in at most
// lookAlikeMaxDistance bits and reports each run once, at its third scene: that is the one to change. The fix asks for another
// environment, another framing and the actor on the other side, at every second scene of the run. The list of stills leaves out
// the scenes without a measured layout (scenes whose hold frame is out of range), so two successive stills are
// neighbours only when their story indexes are consecutive: a gap ends the run, because the scenes across it are not seen next
// to each other in the film.
func LookAlikeIssues(stills []StillInfo) []Issue {
```

In `internal/storymaker/studio_check.go`, replace:

```go
	CompileReason               string // the closed code of why the compile check was unavailable, "" otherwise
	StoryboardScenes            int    // storyboard scenes of the film, from the compile run
	CodeScenes                  int    // code scenes of the film (their layout is never measured)
	LayoutScenes                int    // storyboard scenes whose layout the player measured
	RoundsLeft                  int
}
```

with:

```go
	CompileReason               string // the closed code of why the compile check was unavailable, "" otherwise
	StoryboardScenes            int    // storyboard scenes of the film, from the compile run
	CodeScenes                  int    // code scenes of the film (their layout is never measured)
	LayoutScenes                int    // scenes (storyboard and code) whose layout block the player reported
	RoundsLeft                  int
}
```

In `ui/js/storymaker/layout.js`, replace:

```js
// its last cue arrival and its last motion, plus half a second), and reports where the actors, the shown props, the texts and
// the burned caption are, as boxes in fractions of the output frame. Go turns the boxes into findings. It reads the compiled
// timeline and the loaded art and draws nothing; the one canvas it uses only measures text. Classic script; publishes
// StorymakerCore.layout.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var HOLD_AFTER = 0.5;       // seconds after the last arrival or motion
    var VISIBLE = 0.05;         // an entity with less alpha is not on the stage
    var EPS = 0.5;              // design pixels: a smaller move is no move
```

with:

```js
// its last cue arrival and its last motion, plus half a second), and reports where the actors, the shown props, the texts and
// the burned caption are, as boxes in fractions of the output frame. Go turns the boxes into findings. It reads the compiled
// timeline and the loaded art and draws nothing; the one canvas it uses only measures text. Classic script; publishes
// StorymakerCore.layout. A code scene gets a block with code true and no boxes of its own.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var HOLD_AFTER = 0.5;       // seconds after the last arrival or motion
    var CODE_HOLD_SHARE = 0.7;  // a code scene is sampled at this share of its length (compileshots.go codeHoldShare)
    var VISIBLE = 0.05;         // an entity with less alpha is not on the stage
    var EPS = 0.5;              // design pixels: a smaller move is no move
```

In `ui/js/storymaker/layout.js`, replace:

```js

    // holdSeconds(timeline, index): the scene-local time of the sample, clamped to the last frame in which the scene is the
    // one on screen (the next scene's entry transition overlaps its end).
    function holdSeconds(timeline, index) {
        var sc = timeline.scenes[index], next = timeline.scenes[index + 1], fps = timeline.fps, last = 0;
        var cues = sc.narration && sc.narration.cueList || [];
        for (var i = 0; i < cues.length; i++) if (cues[i].at_ms / 1000 > last) last = cues[i].at_ms / 1000;
        for (var k = 0; k < sc.entities.length; k++) { var end = SM.scene.entityEnd(sc.entities[k]); if (end > last) last = end; }
        var lastFrame = Math.min(sc.end_frame, next ? next.start_frame : sc.end_frame) - 1;
        return Math.min(last + HOLD_AFTER, Math.max(0, (lastFrame - sc.start_frame) / fps));
    }

    // frameBox(box, win, stage, W, H): a design box under a camera window as [x, y, w, h] in fractions of the canvas.
```

with:

```js

    // holdSeconds(timeline, index): the scene-local time of the sample, clamped to the last frame in which the scene is the
    // one on screen (the next scene's entry transition overlaps its end).
    // A code scene has no stage entities of the storyboard: it is sampled at the larger of 70 % of its length and half a second
    // after its last cue, and never after its last frame.
    function holdSeconds(timeline, index) {
        var sc = timeline.scenes[index], next = timeline.scenes[index + 1], fps = timeline.fps, last = 0;
        var cues = sc.narration && sc.narration.cueList || [];
        for (var i = 0; i < cues.length; i++) if (cues[i].at_ms / 1000 > last) last = cues[i].at_ms / 1000;
        var lastFrame = Math.min(sc.end_frame, next ? next.start_frame : sc.end_frame) - 1;
        var limit = Math.max(0, (lastFrame - sc.start_frame) / fps);
        if (sc.kind === 'code') return Math.min(Math.max(CODE_HOLD_SHARE * (sc.end_frame - sc.start_frame) / fps, last + HOLD_AFTER), limit);
        for (var k = 0; k < sc.entities.length; k++) { var end = SM.scene.entityEnd(sc.entities[k]); if (end > last) last = end; }
        return Math.min(last + HOLD_AFTER, limit);
    }

    // frameBox(box, win, stage, W, H): a design box under a camera window as [x, y, w, h] in fractions of the canvas.
```

In `ui/js/storymaker/layout.js`, replace:

```js
        var sc = timeline.scenes[index], W = timeline.width, H = timeline.height, fps = timeline.fps;
        var t = holdSeconds(timeline, index), stage = SM.render.stageFor(sc, rc);
        var win = stage.cameraWindow(SM.camera.at(sc.camera, t)), base = stage.baseWindow();
        var block = { id: sc.id, index: sc.index, hold_s: round4(t), actors: [], props: [], texts: [], caption: null, missing: 0, static: false };
        for (var i = 0; i < sc.entities.length; i++) {
            var e = sc.entities[i], st = SM.scene.evalEntity(e, t);
            if (!(st.alpha > VISIBLE)) continue;
            if (e.type === 'actor' || e.type === 'prop') {
```

with:

```js
        var sc = timeline.scenes[index], W = timeline.width, H = timeline.height, fps = timeline.fps;
        var t = holdSeconds(timeline, index), stage = SM.render.stageFor(sc, rc);
        var win = stage.cameraWindow(SM.camera.at(sc.camera, t)), base = stage.baseWindow();
        var code = sc.kind === 'code';
        var block = { id: sc.id, index: sc.index, hold_s: round4(t), code: code, actors: [], props: [], texts: [], caption: null, missing: 0, static: false };
        for (var i = 0; i < sc.entities.length && !code; i++) {
            var e = sc.entities[i], st = SM.scene.evalEntity(e, t);
            if (!(st.alpha > VISIBLE)) continue;
            if (e.type === 'actor' || e.type === 'prop') {
```

In `ui/js/storymaker/layout.js`, replace:

```js
            var cap = SM.captions.at(timeline.captions, sc.start_frame * 1000 / fps + t * 1000);
            if (cap) block.caption = pixelBox(SM.render.captionBox(ctx, cap, rc), W, H);
        }
        block.static = snapshot(sc, 0) === snapshot(sc, t);
        return block;
    }

    // measure(timeline, assets): one block per storyboard scene, in story order. assets is what the renderer gets (rig, props,
    // envs, canvas(name, w, h)). A scene with missing art still reports; its missing count says how many boxes are absent.
    function measure(timeline, assets) {
        var W = timeline.width, H = timeline.height, rc = SM.render.makeContext(timeline, assets, W, H);
        var holder = assets && typeof assets.canvas === 'function' ? assets.canvas('layout', W, H) : null, ctx = holder && holder.ctx || null;
        var out = [];
        for (var i = 0; i < timeline.scenes.length; i++) {
            if (timeline.scenes[i].kind === 'code') continue;
            out.push(measureScene(timeline, i, rc, ctx));
        }
        return out;
```

with:

```js
            var cap = SM.captions.at(timeline.captions, sc.start_frame * 1000 / fps + t * 1000);
            if (cap) block.caption = pixelBox(SM.render.captionBox(ctx, cap, rc), W, H);
        }
        if (!code) block.static = snapshot(sc, 0) === snapshot(sc, t);
        return block;
    }

    // measure(timeline, assets): one block per scene, in story order. assets is what the renderer gets (rig, props,
    // envs, canvas(name, w, h)). A scene with missing art still reports; its missing count says how many boxes are absent.
    function measure(timeline, assets) {
        var W = timeline.width, H = timeline.height, rc = SM.render.makeContext(timeline, assets, W, H);
        var holder = assets && typeof assets.canvas === 'function' ? assets.canvas('layout', W, H) : null, ctx = holder && holder.ctx || null;
        var out = [];
        for (var i = 0; i < timeline.scenes.length; i++) {
            out.push(measureScene(timeline, i, rc, ctx));
        }
        return out;
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `npm run test:storymaker; go test ./internal/storymaker -run '(?i)(TestLayoutIssuesSkipStaticForCodeScenes)' -count=1`
Expected: 271 passed, 0 failed; Go PASS; build:ui + check:ui ok

- [ ] **Step 6: Commit**

```bash
git add internal/storymaker/layout.go internal/storymaker/layout_test.go internal/storymaker/stills.go internal/storymaker/studio_check.go scripts/storymaker-tests/30-layout.mjs ui/js/storymaker/layout.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: code scenes get a layout block with a hold frame"
```

### Task A13: pass 2 skips scenes that another loop filled; stop before the progress read on cancel

**Files:**
- Modify: `internal/server/storymaker_agent.go`
- Modify: `internal/server/storymaker_agent_fill_order_test.go`
- Modify: `internal/server/storymaker_agent_script_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/server/storymaker_agent_fill_order_test.go`, replace:

```go
package server

import (
	"regexp"
	"slices"
	"testing"
```

with:

```go
package server

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"testing"
```

In `internal/server/storymaker_agent_fill_order_test.go`, replace:

```go
		t.Fatalf("fill order = %v, want %v", order, want)
	}
}
```

with:

```go
		t.Fatalf("fill order = %v, want %v", order, want)
	}
}

// A scene that another writer filled while pass 2 ran gets no request of its own any more.
func TestStorymakerAgentFillPassesSkipScenesAnotherLoopFilled(t *testing.T) {
	turns := smScriptHappyTurns()
	turns[storymaker.PhaseFill] = []smScriptTurn{
		{prose: "Ich plane."},
		{prose: "Noch nicht."},
		{prose: "Ich plane auch hier."},
		{prose: "Hier noch nicht."},
		smScriptStoryCall("put_scene", "s1", smScriptSceneOne()),
	}
	client := &smScriptClient{turns: turns}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	fillRequests := 0
	client.beforeReply = func(req openai.ChatCompletionRequest) {
		if smScriptPhase(req) != storymaker.PhaseFill {
			return
		}
		fillRequests++
		if fillRequests != 5 { // the first request of pass 2, scene s1
			return
		}
		raw, etag, err := env.rt.projects.LoadStory(context.Background(), env.id)
		if err != nil {
			t.Error(err)
			return
		}
		story, _, err := storymaker.DecodeStory(raw, storymaker.DefaultLimits())
		if err != nil {
			t.Error(err)
			return
		}
		data, _ := json.Marshal(smScriptSceneTwo())
		var filled storymaker.Scene
		if err := json.Unmarshal(data, &filled); err != nil {
			t.Error(err)
			return
		}
		for i := range story.Scenes {
			if story.Scenes[i].ID == "s2" {
				story.Scenes[i].Environment, story.Scenes[i].Actors = filled.Environment, filled.Actors
				story.Scenes[i].Beats, story.Scenes[i].Transition = filled.Beats, filled.Transition
			}
		}
		if raw, err = json.Marshal(story); err == nil {
			_, err = env.rt.projects.SaveStory(context.Background(), env.id, raw, etag)
		}
		if err != nil {
			t.Error(err)
		}
	}
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	smScriptRequireReady(t, job)
	smScriptRequireFilled(t, smScriptStory(t, env), "s1", "s2")

	var order []string
	for _, req := range client.seen() {
		if smScriptPhase(req) != storymaker.PhaseFill {
			continue
		}
		for _, m := range req.Messages {
			if m.Role != openai.ChatMessageRoleUser {
				continue
			}
			if match := smFillSceneRe.FindStringSubmatch(m.Content); match != nil && (len(order) == 0 || order[len(order)-1] != match[1]) {
				order = append(order, match[1])
			}
		}
	}
	if want := []string{"s1", "s2", "s1"}; !slices.Equal(order, want) {
		t.Fatalf("fill order = %v, want %v", order, want)
	}
	// The scene is skipped before its loop starts, so pass 2 announces no phase for it.
	for _, line := range env.rt.agentLog.after(job.ID, 0) {
		var ev struct {
			Type    string `json:"type"`
			Phase   string `json:"phase"`
			SceneID string `json:"scene_id"`
			Attempt int    `json:"attempt"`
		}
		if json.Unmarshal(line.Data, &ev) == nil && ev.Type == agentEventPhase && ev.Phase == storymaker.PhaseFill && ev.SceneID == "s2" && ev.Attempt == 2 {
			t.Fatalf("pass 2 started a loop for s2, which was already filled: %s", line.Data)
		}
	}
}
```

In `internal/server/storymaker_agent_script_test.go`, replace:

```go
type smScriptClient struct {
	textMode bool
	onBlock  func()

	mu       sync.Mutex
	turns    map[string][]smScriptTurn
```

with:

```go
type smScriptClient struct {
	textMode bool
	onBlock  func()
	// beforeReply runs before each reply is chosen, without the client lock held.
	beforeReply func(req openai.ChatCompletionRequest)

	mu       sync.Mutex
	turns    map[string][]smScriptTurn
```

In `internal/server/storymaker_agent_script_test.go`, replace:

```go
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	c.mu.Lock()
	req.Messages = slices.Clone(req.Messages)
	c.requests = append(c.requests, req)
```

with:

```go
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionMessage{}, "", err
	}
	if c.beforeReply != nil {
		c.beforeReply(req)
	}
	c.mu.Lock()
	req.Messages = slices.Clone(req.Messages)
	c.requests = append(c.requests, req)
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/server -run '(?i)(TestStorymakerAgentFillPasses)' -count=1`
Expected: pass 2 announced a fill phase (attempt 2) for s2 that was already filled

- [ ] **Step 3: Implement**

In `internal/server/storymaker_agent.go`, replace:

```go
			if ctx.Err() != nil {
				return
			}
			j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseFill, sceneID: sceneID, pass: pass})
			j.refreshProgress(ctx)
		}
		if pass == storymakerFillPasses {
			return
		}
		progress, err := j.studio.Progress(ctx)
```

with:

```go
			if ctx.Err() != nil {
				return
			}
			if pass > 1 {
				// Another loop may have filled this scene since the list was made.
				current, err := j.studio.Progress(ctx)
				if err != nil {
					j.logger.Warn("[Storymaker] The agent progress could not be read", "job_id", j.job.ID, "error", err)
					j.noteModelError(err)
					return
				}
				if !slices.Contains(current.Unfilled, sceneID) {
					continue
				}
			}
			j.runPhase(ctx, storymakerAgentLoop{phase: storymaker.PhaseFill, sceneID: sceneID, pass: pass})
			j.refreshProgress(ctx)
		}
		if pass == storymakerFillPasses || ctx.Err() != nil {
			return
		}
		progress, err := j.studio.Progress(ctx)
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/server -run '(?i)(TestStorymakerAgentFillPasses)' -count=1`
Expected: PASS (2 tests); the cancel guard has no separate test

- [ ] **Step 5: Commit**

```bash
git add internal/server/storymaker_agent.go internal/server/storymaker_agent_fill_order_test.go internal/server/storymaker_agent_script_test.go
git commit -m "storymaker: pass 2 skips scenes that another loop filled; stop before the progress read on cancel"
```

### Task A14: code scenes get no caption-zone or camera advice; look-alike speaks code

**Files:**
- Modify: `internal/storymaker/layout.go`
- Modify: `internal/storymaker/layout_test.go`
- Modify: `internal/storymaker/stills.go`
- Modify: `internal/storymaker/stills_test.go`
- Modify: `internal/storymaker/studio_check.go`
- Modify: `ui/js/storymaker/layout.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/layout_test.go`, replace:

```go
		t.Fatalf("a storyboard scene that is static reported %v", got)
	}
}
```

with:

```go
		t.Fatalf("a storyboard scene that is static reported %v", got)
	}
}

func TestLayoutIssuesCodeSceneGetsNoCaptionFindings(t *testing.T) {
	caption := LayoutBox{0.2, 0.8, 0.6, 0.1} // inside the bottom margin of 9:16
	sc := layoutScene("c", 0)
	sc.Caption, sc.Code = &caption, true
	opts := LayoutOptions{CaptionRule: true, Aspect: "9:16"}
	if got := LayoutIssues([]LayoutScene{sc}, opts); len(got) != 0 {
		t.Fatalf("a code scene reported %v", layoutCodes(got))
	}
	sc.Code = false
	if got := layoutCodes(LayoutIssues([]LayoutScene{sc}, opts)); !slices.Equal(got, []string{CodeTextUnsafe}) {
		t.Fatalf("a storyboard scene reported %v", got)
	}
}
```

In `internal/storymaker/stills_test.go`, replace:

```go
		t.Errorf("16 million pixels exactly: %v", err)
	}
}
```

with:

```go
		t.Errorf("16 million pixels exactly: %v", err)
	}
}

func TestLookAlikeIssuesCodeSceneGetsTheCodeFix(t *testing.T) {
	run := func(code ...bool) []StillInfo {
		var out []StillInfo
		for i, c := range code {
			out = append(out, StillInfo{SceneID: fmt.Sprintf("s%d", i+1), Index: i, Hash: 0, Code: c})
		}
		return out
	}
	got := LookAlikeIssues(run(false, false, true))
	if len(got) != 1 || !strings.Contains(got[0].Fix, "put_code") || strings.Contains(got[0].Fix, "environment") || strings.Contains(got[0].Fix, "framing") {
		t.Fatalf("a run with a code scene: %+v", got)
	}
	got = LookAlikeIssues(run(false, false, false))
	if len(got) != 1 || !strings.Contains(got[0].Fix, "framing") || strings.Contains(got[0].Fix, "put_code") {
		t.Fatalf("a storyboard run keeps the camera advice: %+v", got)
	}
	// The pair is the changed scene and its predecessor: a code scene farther away leaves the camera advice alone.
	if got = LookAlikeIssues(run(true, false, false)); len(got) != 1 || strings.Contains(got[0].Fix, "put_code") {
		t.Fatalf("a code scene outside the pair: %+v", got)
	}
	if got = LookAlikeIssues(run(false, true, false)); len(got) != 1 || !strings.Contains(got[0].Fix, "put_code") {
		t.Fatalf("a code scene as the predecessor: %+v", got)
	}
}

func TestMarkCodeStillsCopiesTheLayoutFlag(t *testing.T) {
	stills := []StillInfo{{SceneID: "a"}, {SceneID: "b"}}
	got := markCodeStills(stills, []LayoutScene{{ID: "b", Code: true}})
	if got[0].Code || !got[1].Code || stills[1].Code {
		t.Fatalf("stills = %+v, input = %+v", got, stills)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestLayoutIssues|TestLookAlike|TestMarkCode)' -count=1`
Expected: test files did not build against the old code (no StillInfo.Code, no markCodeStills); the caption-zone case is covered by the same run

- [ ] **Step 3: Implement**

In `internal/storymaker/layout.go`, replace:

```go
		add := func(code, message, fix string) {
			out = append(out, Issue{Severity: SeverityWarning, SceneID: sc.ID, Path: path, Code: code, Message: message, Fix: fix})
		}
		if opts.CaptionRule && sc.Caption != nil && !opts.CaptionExempt[sc.ID] {
			for _, a := range sc.Actors {
				if footInCaption(a.Box, *sc.Caption) {
					add(CodeCaptionOverlap, fmt.Sprintf("The caption covers the feet of %s in scene %s.", a.ID, sc.ID),
```

with:

```go
		add := func(code, message, fix string) {
			out = append(out, Issue{Severity: SeverityWarning, SceneID: sc.ID, Path: path, Code: code, Message: message, Fix: fix})
		}
		if opts.CaptionRule && sc.Caption != nil && !sc.Code && !opts.CaptionExempt[sc.ID] {
			for _, a := range sc.Actors {
				if footInCaption(a.Box, *sc.Caption) {
					add(CodeCaptionOverlap, fmt.Sprintf("The caption covers the feet of %s in scene %s.", a.ID, sc.ID),
```

In `internal/storymaker/layout.go`, replace:

```go
				add(CodeTextOffframe, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		if sc.Caption != nil && zoneShare(*sc.Caption, zone) > layoutZoneShare {
			add(CodeTextUnsafe, fmt.Sprintf("The caption lies in the margin that platform overlays cover in scene %s.", sc.ID),
				`Shorten the narration of the scene so the caption has fewer lines.`)
		}
```

with:

```go
				add(CodeTextOffframe, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		if sc.Caption != nil && !sc.Code && zoneShare(*sc.Caption, zone) > layoutZoneShare {
			add(CodeTextUnsafe, fmt.Sprintf("The caption lies in the margin that platform overlays cover in scene %s.", sc.ID),
				`Shorten the narration of the scene so the caption has fewer lines.`)
		}
```

In `internal/storymaker/stills.go`, replace:

```go
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

with:

```go
)

// StillInfo is one captured still: the scene, its index in the story, the difference hash of the picture and the measured
// contrast of its text boxes (empty when none was measured). Code marks the still of a code scene.
type StillInfo struct {
	SceneID string
	Index   int
	Hash    uint64
	Texts   []TextContrast
	Code    bool
}

// ErrSheetTooLarge says that a contact sheet would exceed the pixel budget even with the narrowest thumbnails.
```

In `internal/storymaker/stills.go`, replace:

```go
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
```

with:

```go
			ids[i] = s.SceneID
		}
		var change []string
		codeScene := false
		for i := 2; i < len(run); i += 2 {
			change = append(change, run[i].SceneID)
			codeScene = codeScene || run[i].Code || run[i-1].Code
		}
		fix := fmt.Sprintf(`Change scene %s so it differs from its neighbours: another environment with "transition":"wipe", a framing such as {"camera":{"framing":"medium"}}, and the actor on the other side ("at":"right" or "left") with the whole actors list.`,
			strings.Join(change, " and "))
		if codeScene {
			// A code scene has no environment, camera or actors: only its own source changes the picture.
			fix = "Give this scene its own picture: another block layout, motif or position in its source (put_code)."
		}
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: run[2].SceneID, Path: jp("scenes", run[2].Index), Code: CodeScenesLookAlike,
			Message: fmt.Sprintf("Scenes %s look alike: the same stage and nearly the same picture.", strings.Join(ids, ", ")),
			Fix:     fix,
		})
	}
	for i := 1; i <= len(stills); i++ {
```

In `internal/storymaker/studio_check.go`, replace:

```go
	})
}

func (s *Studio) runCheck(ctx context.Context) (CheckReport, string, error) {
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
```

with:

```go
	})
}

// markCodeStills copies the code flag of the measured layout onto the stills, so the look-alike finding can tell a code scene.
func markCodeStills(stills []StillInfo, layout []LayoutScene) []StillInfo {
	code := map[string]bool{}
	for _, sc := range layout {
		code[sc.ID] = sc.Code
	}
	out := slices.Clone(stills)
	for i := range out {
		out[i].Code = code[out[i].SceneID]
	}
	return out
}

func (s *Studio) runCheck(ctx context.Context) (CheckReport, string, error) {
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
```

In `internal/storymaker/studio_check.go`, replace:

```go
	summary, state, environment := s.compileForCheck(ctx, etag)
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LayoutIssues(summary.Layout, StoryLayoutOptions(&story))...)
	found = append(found, LookAlikeIssues(summary.Stills)...)
	found = append(found, ContrastIssues(summary.Stills)...)
	found = append(found, LintForJobMode(&story, s.job.Mode)...)
	found = append(found, s.craftIssues(ctx, &story)...)
```

with:

```go
	summary, state, environment := s.compileForCheck(ctx, etag)
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LayoutIssues(summary.Layout, StoryLayoutOptions(&story))...)
	found = append(found, LookAlikeIssues(markCodeStills(summary.Stills, summary.Layout))...)
	found = append(found, ContrastIssues(summary.Stills)...)
	found = append(found, LintForJobMode(&story, s.job.Mode)...)
	found = append(found, s.craftIssues(ctx, &story)...)
```

In `ui/js/storymaker/layout.js`, replace:

```js
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var HOLD_AFTER = 0.5;       // seconds after the last arrival or motion
    var CODE_HOLD_SHARE = 0.7;  // a code scene is sampled at this share of its length (compileshots.go codeHoldShare)
    var VISIBLE = 0.05;         // an entity with less alpha is not on the stage
    var EPS = 0.5;              // design pixels: a smaller move is no move
```

with:

```js
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var HOLD_AFTER = 0.5;       // seconds after the last arrival or motion
    var CODE_HOLD_SHARE = 0.7;  // a code scene is sampled at this share of its length (render/compileshots.go shotDefaultFraction)
    var VISIBLE = 0.05;         // an entity with less alpha is not on the stage
    var EPS = 0.5;              // design pixels: a smaller move is no move
```

- [ ] **Step 4: Rebuild the generated files**

Run: npm run build:ui, then npm run check:ui (git checkout ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)

- [ ] **Step 5: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestLayoutIssues|TestLookAlike|TestMarkCode)' -count=1`
Expected: PASS (15 tests); npm run test:storymaker 271 passed; build:ui + check:ui ok

- [ ] **Step 6: Commit**

```bash
git add internal/storymaker/layout.go internal/storymaker/layout_test.go internal/storymaker/stills.go internal/storymaker/stills_test.go internal/storymaker/studio_check.go ui/js/storymaker/layout.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: code scenes get no caption-zone or camera advice; look-alike speaks code"
```

### Task A15: removing a closing marker leaves no stray space

**Files:**
- Modify: `internal/storymaker/studio_markers_test.go`
- Modify: `internal/storymaker/studio_umlauts.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/studio_markers_test.go`, replace:

```go
		t.Fatalf("stored outline narration %q", got)
	}
}
```

with:

```go
		t.Fatalf("stored outline narration %q", got)
	}
}

func TestRepairTextClosingMarkerSpacing(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	studio.job.Language = "en"
	for in, want := range map[string]string{
		"Daten {/1}.":            "Daten.",
		"Daten{/1}. Mehr":        "Daten. Mehr",
		"Mehr {1}Text {/1}hier":  "Mehr {1}Text hier",
		"a {/2} b":               "a b",
		"Pakete{/2} durchlassen": "Pakete durchlassen",
		"Ende {/3}":              "Ende",
		"Wort{/1}Wort":           "WortWort",
	} {
		if got := studio.repairText(in); got != want {
			t.Errorf("repairText(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker -run '(?i)(TestPutSceneRemovesClosingMarkers|TestRepairText)' -count=1`
Expected: repairText("Daten {/1}.") = "Daten ." and "a {/2} b" = "a  b"

- [ ] **Step 3: Implement**

In `internal/storymaker/studio_umlauts.go`, replace:

```go
	"strings"
	"sync"
	"unicode"
)

//go:embed data/de_umlauts.json
```

with:

```go
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

//go:embed data/de_umlauts.json
```

In `internal/storymaker/studio_umlauts.go`, replace:

```go
}

// closingMarker is the {/n} that a model sometimes writes after a cue phrase; the format has no such marker, so no text keeps it.
var closingMarker = regexp.MustCompile(`\{/\d+\}`)

// repairText removes closing markers in every language, then restores the umlauts of one text in the job's language and remembers what it repaired for the call's answer.
func (s *Studio) repairText(text string) string {
	text = closingMarker.ReplaceAllString(text, "")
	fixed, fixes := RepairUmlauts(text, s.job.Language)
	for _, f := range fixes {
		s.pendingRepairs = addUmlautFix(s.pendingRepairs, f)
```

with:

```go
}

// closingMarker is the {/n} that a model sometimes writes after a cue phrase; the format has no such marker, so no text keeps it.
var closingMarker = regexp.MustCompile(`([ 	]*)\{/\d+\}([ 	]*)`)

// removeClosingMarkers cuts every {/n} with the blanks around it. Where the marker sat between two words, one space stays;
// before the end of the text or a closing punctuation mark, and where no blank touched it, nothing stays.
func removeClosingMarkers(text string) string {
	var out strings.Builder
	last := 0
	for _, m := range closingMarker.FindAllStringSubmatchIndex(text, -1) {
		out.WriteString(text[last:m[0]])
		last = m[1]
		blanks := m[3] > m[2] || m[5] > m[4]
		next, _ := utf8.DecodeRuneInString(text[m[1]:])
		if blanks && m[0] > 0 && m[1] < len(text) && !strings.ContainsRune(".,!?;:\u2026)", next) {
			out.WriteByte(' ')
		}
	}
	out.WriteString(text[last:])
	return out.String()
}

// repairText removes closing markers in every language, then restores the umlauts of one text in the job's language and remembers what it repaired for the call's answer.
func (s *Studio) repairText(text string) string {
	text = removeClosingMarkers(text)
	fixed, fixes := RepairUmlauts(text, s.job.Language)
	for _, f := range fixes {
		s.pendingRepairs = addUmlautFix(s.pendingRepairs, f)
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker -run '(?i)(TestPutSceneRemovesClosingMarkers|TestRepairText)' -count=1`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/studio_markers_test.go internal/storymaker/studio_umlauts.go
git commit -m "storymaker: removing a closing marker leaves no stray space"
```

