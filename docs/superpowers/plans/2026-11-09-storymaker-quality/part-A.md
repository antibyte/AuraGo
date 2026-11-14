# Part A

Tasks of batch A: film-level craft lints, the direction packet, the prompt texts (version v3) and the fixtures. Rules, commands and the order are in the plan index. Decisions are in [design.md](design.md), table "Batch A".

All of batch A is Go in `internal/storymaker` and `internal/server`, plus one data file, 16 locale files and `storymaker-agent.js`. It needs no browser.

<!-- group A0 -->

### Task A0: Record the test baseline
**Batch:** A  **Depends on:** none
**Files:**
Create `docs/superpowers/plans/2026-11-09-storymaker-quality/evidence/baseline.md`

- [ ] **Step 1: Run: write the baseline from real test runs.** Run this from the root of the batch A worktree before any other task, with no code change. A failing test is expected here. Do not fix anything. The block takes several minutes, so run it as a background command.

```bash
F=docs/superpowers/plans/2026-11-09-storymaker-quality/evidence/baseline.md; mkdir -p "$(dirname "$F")"; printf '# Test baseline at %s\n' "$(git rev-parse HEAD)" > "$F"
note() { out=$(awk '/^--- FAIL: /{print "- " $3} /build failed/{print "- [build failed] " $2} /^panic: /{print "- [panic] " $2}' | sort -u); printf '\n## %s\n%s\n' "$1" "${out:-none}" >> "$F"; }
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/storymaker/... 2>&1 | note './internal/storymaker/...'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/server -run Storymaker 2>&1 | note './internal/server -run Storymaker'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/agent -run Storymaker 2>&1 | note './internal/agent -run Storymaker'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/config 2>&1 | note './internal/config'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./ui -run '(?i)(storymaker|TestConfigStorymaker)' 2>&1 | note './ui -run storymaker'
node scripts/test-storymaker-player.mjs 2>&1 | awk '/FAIL|not ok|Error/{print "- " $0}' | sort -u | { out=$(cat); printf '\n## node scripts/test-storymaker-player.mjs\n%s\n' "${out:-none}" >> "$F"; }
```

Expected: the file starts with the commit hash and has six `##` headings. Each lists failing top-level test names or `none`. From now on a batch passes when its failures are a subset of this file.
- [ ] **Step 2: Commit:** stage `docs/superpowers/plans/2026-11-09-storymaker-quality/evidence/baseline.md` through the temp index, because `docs/` is git-ignored (see the index, "Commits"); message `docs(storymaker): record the test baseline of the quality work`.

<!-- group A1 -->

### Task A1: Fixtures
**Batch:** A  **Depends on:** A0
**Files:**
Create `internal/storymaker/testdata/stories/agent-firewall-run.json`
Create `internal/storymaker/testdata/stories/agent-varied.json`
Create `internal/storymaker/agent_fixtures_test.go`

The first fixture is the real story an agent wrote on aurago-test on 2026-11-08 (7 scenes, all on `env.plain-stage`, all `wide`, `point` in 5 scenes, the second cast member never on stage, broken umlauts). It is kept byte for byte. The second is a hand-written story that a good film would look like: four environments (no three scenes in a row share one), two cast members used, two framings, varied gestures.

- [ ] **Step 1: Copy the real run.**

```bash
cp docs/superpowers/plans/2026-11-09-storymaker-quality/evidence/firewall-run.story.json internal/storymaker/testdata/stories/agent-firewall-run.json
```

If the worktree has no `docs/` folder (it is git-ignored), copy from the main checkout: `cp ../../docs/superpowers/plans/2026-11-09-storymaker-quality/evidence/firewall-run.story.json ...` (adjust `../..` to the main checkout).
- [ ] **Step 2: Create `internal/storymaker/testdata/stories/agent-varied.json`:**

```json
{
  "schema": "storymaker/1",
  "title": "Wie eine Firewall dein Netz schützt",
  "format": { "aspect": "16:9", "fps": 30 },
  "style": { "palette": "calm", "font": "default", "captions": true, "rhythm": "standard" },
  "cast": [
    { "id": "anna", "name": "Anna", "character": "char.office-woman-1" },
    { "id": "ben", "name": "Ben", "character": "char.it-tech-1" }
  ],
  "scenes": [
    {
      "id": "s1", "environment": "env.office", "variant": "day",
      "narration": "Jede Sekunde erreichen dein Netz {1}tausende Datenpakete.",
      "speaker": "anna",
      "actors": [{ "cast": "anna", "at": "left", "pose": "present", "facing": "auto", "expression": "neutral" }],
      "beats": [
        { "do": "show", "item": "prop.laptop", "at": "center", "entry": "pop", "id": "laptop", "cue": 1 },
        { "do": "gesture", "who": "anna", "gesture": "wave", "with": "prev" }
      ],
      "transition": "cut", "duration": "auto", "camera": { "framing": "medium" }
    },
    {
      "id": "s2", "environment": "env.office", "variant": "day",
      "narration": "Die meisten sind harmlos, doch manche tragen {1}Schadcode.",
      "speaker": "ben",
      "actors": [{ "cast": "ben", "at": "right", "pose": "think", "facing": "auto", "expression": "worried" }],
      "beats": [
        { "do": "show", "item": "prop.warning", "at": "center", "entry": "fade", "id": "warning", "cue": 1 },
        { "do": "gesture", "who": "ben", "gesture": "shrug", "with": "prev" }
      ],
      "transition": "cut", "duration": "auto", "camera": { "framing": "wide" }
    },
    {
      "id": "s3", "environment": "env.server-room", "variant": "day",
      "narration": "Hier greift die {1}Firewall und prüft jedes Paket am Eingang.",
      "speaker": "anna",
      "actors": [{ "cast": "anna", "at": "left", "pose": "stand", "facing": "auto", "expression": "determined" }],
      "beats": [
        { "do": "show", "item": "prop.firewall", "at": "center", "entry": "slide", "id": "firewall", "cue": 1 },
        { "do": "gesture", "who": "anna", "gesture": "present", "with": "prev" }
      ],
      "transition": "wipe", "duration": "auto", "camera": { "framing": "medium" }
    },
    {
      "id": "s4", "environment": "env.server-room", "variant": "day",
      "narration": "Sie folgt deinen {1}Regeln: diese Pakete dürfen durch, jene nicht.",
      "speaker": "ben",
      "actors": [{ "cast": "ben", "at": "right", "pose": "present", "facing": "auto", "expression": "neutral" }],
      "beats": [
        { "do": "show", "item": "prop.folder", "at": "center", "entry": "pop", "id": "rules", "cue": 1 },
        { "do": "gesture", "who": "ben", "gesture": "point", "with": "prev" }
      ],
      "transition": "cut", "duration": "auto", "camera": { "framing": "wide" }
    },
    {
      "id": "s5", "environment": "env.home-office", "variant": "day",
      "narration": "Ohne Erlaubnis wird ein Paket {1}abgewiesen.",
      "speaker": "anna",
      "actors": [{ "cast": "anna", "at": "left", "pose": "think", "facing": "auto", "expression": "neutral" }],
      "beats": [
        { "do": "show", "item": "prop.shield", "at": "center", "entry": "fade", "id": "shield", "cue": 1 },
        { "do": "gesture", "who": "anna", "gesture": "thumbs-up", "with": "prev" }
      ],
      "transition": "wipe", "duration": "auto", "camera": { "framing": "wide" }
    },
    {
      "id": "s6", "environment": "env.home-office", "variant": "day",
      "narration": "Dein Gerät bleibt {1}geschützt, ganz ohne Mühe.",
      "speaker": "ben",
      "actors": [{ "cast": "ben", "at": "right", "pose": "stand", "facing": "auto", "expression": "happy" }],
      "beats": [
        { "do": "show", "item": "prop.hard-drive", "at": "center", "entry": "slide", "id": "drive", "cue": 1 },
        { "do": "gesture", "who": "ben", "gesture": "celebrate", "with": "prev" }
      ],
      "transition": "cut", "duration": "auto", "camera": { "framing": "medium" }
    },
    {
      "id": "s7", "environment": "env.living-room", "variant": "day",
      "narration": "Ein Tor, das {1}rund um die Uhr wacht.",
      "speaker": "anna",
      "actors": [{ "cast": "anna", "at": "left", "pose": "stand", "facing": "auto", "expression": "happy" }],
      "beats": [
        { "do": "show", "item": "prop.clock", "at": "center", "entry": "pop", "id": "clock", "cue": 1 },
        { "do": "gesture", "who": "anna", "gesture": "present", "with": "prev" }
      ],
      "transition": "wipe", "duration": "auto", "camera": { "framing": "wide" }
    }
  ]
}
```
- [ ] **Step 3: Create `internal/storymaker/agent_fixtures_test.go`:**

```go
package storymaker

import (
	"os"
	"testing"
)

// loadAgentFixture reads a story of testdata/stories, decodes it and applies the defaults, as the job's check sees it.
func loadAgentFixture(t *testing.T, name string) Story {
	t.Helper()
	data, err := os.ReadFile("testdata/stories/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	s := mustDecode(t, string(data))
	ApplyDefaults(&s)
	return s
}

// The two agent fixtures are real stories: they decode, and the real catalog accepts them.
func TestAgentFixturesValidateAgainstTheRealCatalog(t *testing.T) {
	cat := loadReal(t)
	for _, name := range []string{"agent-firewall-run", "agent-varied"} {
		s := loadAgentFixture(t, name)
		if issues := Validate(&s, cat, DefaultLimits()); HasErrors(issues) {
			t.Errorf("%s: %s", name, dump(issues))
		}
	}
}
```
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestAgentFixturesValidateAgainstTheRealCatalog$' -count=1 -v`. Expected: PASS. If `agent-varied` fails on a prop or asset id, replace it with an id of the same kind that `ui/img/storymaker/catalog.json` has and keep everything else.
- [ ] **Step 5: Commit:** `git add internal/storymaker/testdata/stories/agent-firewall-run.json internal/storymaker/testdata/stories/agent-varied.json internal/storymaker/agent_fixtures_test.go`; message `test(storymaker): add the real firewall run and a varied story as fixtures`.

<!-- group A2 -->

### Task A2: Craft codes, their class and the outcome rule
**Batch:** A  **Depends on:** A1
**Files:**
Modify `internal/storymaker/model.go`
Modify `internal/storymaker/issues.go`
Modify `internal/storymaker/agentjob.go`
Modify `internal/storymaker/agentjob_test.go`
Test `internal/storymaker/issues_test.go`

- [ ] **Step 1: Impact:** `IssueModelFixable`, `DecideOutcome`. Report callers, processes and risk. `IssueModelFixable` has three callers (`studio_check.go:326,328`, `agentjob.go:104`, one render test): the risk is expected to be LOW to MEDIUM.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/model.go`
Old:
```go
// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```
New:
```go
// Issue codes of the film-level craft lint (LintFilmCraft). Only the create job's check emits them. The model can fix
// them while check rounds remain, but they never block an outcome: what is left becomes an open check.
const (
	CodeEnvironmentMonotony = "environment_monotony"
	CodeFramingMonotony     = "framing_monotony"
	CodeGestureMonotony     = "gesture_monotony"
	CodeCastUnused          = "cast_unused"
)

// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```

**Edit** `internal/storymaker/issues.go`
Old:
```go
func IssueModelFixable(i Issue) bool {
```
New:
```go
func IssueModelFixable(i Issue) bool {
	if IssueCraft(i.Code) {
		return true
	}
```

**Edit** `internal/storymaker/issues.go`
Old:
```go
// IssueAdvisory says whether a lint code earns one optional fix loop. These codes never block the
```
New:
```go
// IssueCraft says whether a code is a film-level craft finding: the film is valid but looks uniform. The model can fix
// it with its story tools, so IssueModelFixable is true for it and the check loop runs while rounds remain.
func IssueCraft(code string) bool {
	switch code {
	case CodeEnvironmentMonotony, CodeFramingMonotony, CodeGestureMonotony, CodeCastUnused:
		return true
	}
	return false
}

// issueNeverBlocks says whether a model-fixable code stays an open check instead of turning the outcome into
// incomplete. Craft codes and, from batch D on, layout codes never block.
func issueNeverBlocks(code string) bool {
	return IssueCraft(code)
}

// IssueAdvisory says whether a lint code earns one optional fix loop. These codes never block the
```

**Edit** `internal/storymaker/agentjob.go`
Old:
```go
		blocked = blocked || IssueModelFixable(is) || (is.Severity == SeverityError && is.Code != CodeOverTarget && is.Code != CodeCompileFailed)
```
New:
```go
		blocked = blocked || (IssueModelFixable(is) && !issueNeverBlocks(is.Code)) || (is.Severity == SeverityError && is.Code != CodeOverTarget && is.Code != CodeCompileFailed)
```

**Edit** `internal/storymaker/agentjob.go`
Old:
```go
// unfilled scene, a model-fixable issue, a compile failure or an environment error is left. Otherwise ready_unverified:
```
New:
```go
// unfilled scene, a model-fixable issue (craft findings excepted), a compile failure or an environment error is left. Otherwise ready_unverified:
```

**Edit** `internal/storymaker/agentjob_test.go` (add a row; keep the tabs of the neighbouring rows)
Old:
{% raw %}
```go
{name: "clean, advice is not listed", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Advice: []Issue{{Severity: SeverityWarning, Code: CodeCueLate, SceneID: "s1"}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout}},
```
{% endraw %}
New:
{% raw %}
```go
{name: "clean, advice is not listed", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Advice: []Issue{{Severity: SeverityWarning, Code: CodeCueLate, SceneID: "s1"}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout}},
		{name: "a craft finding is listed and never blocks", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Issues: []Issue{{Severity: SeverityWarning, Code: CodeEnvironmentMonotony}, {Severity: SeverityWarning, Code: CodeCastUnused, SceneID: "s3"}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout, {Code: CodeEnvironmentMonotony}, {Code: CodeCastUnused, SceneID: "s3"}}},
		{name: "a real model-fixable error still blocks next to a craft finding", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Issues: []Issue{{Severity: SeverityWarning, Code: CodeGestureMonotony}, {Severity: SeverityError, Code: CodeSceneUnfilled, SceneID: "s2"}}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeGestureMonotony}, {Code: CodeSceneUnfilled, SceneID: "s2"}}},
```
{% endraw %}

**Edit** `internal/storymaker/issues_test.go`: append to the end of the file.
```go

func TestIssueCraftIsModelFixableAndWarningOnly(t *testing.T) {
	for _, code := range []string{CodeEnvironmentMonotony, CodeFramingMonotony, CodeGestureMonotony, CodeCastUnused} {
		if !IssueCraft(code) || !IssueModelFixable(Issue{Severity: SeverityWarning, Code: code}) || !issueNeverBlocks(code) {
			t.Errorf("%s must be a craft code that the model can fix and that never blocks", code)
		}
	}
	for _, code := range []string{CodeSceneUnfilled, CodeOverBudget, CodeSpotOverlap, CodeEntryMonotony} {
		if IssueCraft(code) || issueNeverBlocks(code) {
			t.Errorf("%s must not be a craft code", code)
		}
	}
}
```
If `issues_test.go` does not import `testing` yet, it does (it is a test file); keep its existing imports.
- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestDecideOutcomeTable|TestIssueCraftIsModelFixableAndWarningOnly)$' -count=1`. Expected: PASS.
- [ ] **Step 4: Commit:** run `gofmt -w` on the three Go files, then `git add internal/storymaker/model.go internal/storymaker/issues.go internal/storymaker/agentjob.go internal/storymaker/agentjob_test.go internal/storymaker/issues_test.go`; message `feat(storymaker): craft issue class that never blocks the outcome`.

<!-- group A3 -->

### Task A3: The film-level craft lint
**Batch:** A  **Depends on:** A2, A7
**Files:**
Create `internal/storymaker/studio_craft.go`
Create `internal/storymaker/studio_craft_test.go`

The fix texts name concrete scenes and ids, which come from `DirectionPlan` (task A7). That is why A6 and A7 run before A3: the execution order is A0, A1, A2, A6, A7, A3, A4, A5, A8, A9, A10.

- [ ] **Step 1: Write the failing test.** Create `internal/storymaker/studio_craft_test.go`:

```go
package storymaker

import (
	"slices"
	"strings"
	"testing"
)

func craftCodes(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Code)
	}
	return out
}

// The real run of 2026-11-08 must trip all four film-level lints. This is the red test of the batch.
func TestFilmCraftFlagsTheRealFirewallRun(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	got := LintFilmCraft(&s, "", cat)
	for _, want := range []string{CodeEnvironmentMonotony, CodeFramingMonotony, CodeGestureMonotony, CodeCastUnused} {
		if !slices.Contains(craftCodes(got), want) {
			t.Errorf("codes = %v, missing %s", craftCodes(got), want)
		}
	}
	for _, is := range got {
		if is.Severity != SeverityWarning || is.Fix == "" || !IssueCraft(is.Code) || !IssueModelFixable(is) {
			t.Errorf("%s must be a warning with a fix, a craft code and model-fixable: %+v", is.Code, is)
		}
	}
	for _, is := range got {
		switch is.Code {
		case CodeEnvironmentMonotony:
			if !strings.Contains(is.Fix, "env.") || !strings.Contains(is.Fix, "wipe") {
				t.Errorf("the environment fix must name an environment and the transition: %s", is.Fix)
			}
		case CodeGestureMonotony:
			if is.SceneID == "" || !strings.Contains(is.Message, `"point"`) {
				t.Errorf("the gesture finding must name point and the first scene over the limit: %+v", is)
			}
		case CodeCastUnused:
			if !strings.Contains(is.Message, "laura") || is.SceneID == "" {
				t.Errorf("the cast finding must name laura and a scene: %+v", is)
			}
		}
	}
}

func TestFilmCraftPassesTheVariedStory(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	if got := LintFilmCraft(&s, "", cat); len(got) != 0 {
		t.Fatalf("the varied story must pass: %s", dump(got))
	}
}

func TestFilmCraftSkipsShortFilmsAndExemptsDialogueEnvironments(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	short := s
	short.Scenes = s.Scenes[:3]
	if got := LintFilmCraft(&short, "", cat); len(got) != 0 {
		t.Fatalf("a film of 3 scenes is exempt: %s", dump(got))
	}
	if got := LintFilmCraft(&s, "dialogue_story", cat); slices.Contains(craftCodes(got), CodeEnvironmentMonotony) {
		t.Fatalf("a dialogue_story never gets the environment lint: %v", craftCodes(got))
	}
	if got := LintFilmCraft(nil, "", cat); got != nil {
		t.Fatalf("nil story = %v", got)
	}
}

// The limit is max(2, ceil(n/3)) scenes per gesture: 7 scenes allow 3, a fourth one is the finding.
func TestFilmCraftGestureLimit(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i := range s.Scenes {
		for j := range s.Scenes[i].Beats {
			if s.Scenes[i].Beats[j].Do == "gesture" {
				s.Scenes[i].Beats[j].Gesture = "point"
			}
		}
	}
	got := LintFilmCraft(&s, "", cat)
	var gesture *Issue
	for i := range got {
		if got[i].Code == CodeGestureMonotony {
			gesture = &got[i]
		}
	}
	if gesture == nil || gesture.SceneID != "s4" {
		t.Fatalf("7 scenes with point must flag s4 (the 4th), got %+v", got)
	}
	// Three scenes with the same gesture are still inside the limit.
	for i := 3; i < len(s.Scenes); i++ {
		for j := range s.Scenes[i].Beats {
			if s.Scenes[i].Beats[j].Do == "gesture" {
				s.Scenes[i].Beats[j].Gesture = []string{"", "", "", "wave", "shrug", "present", "thumbs-up"}[i]
			}
		}
	}
	if got := LintFilmCraft(&s, "", cat); slices.Contains(craftCodes(got), CodeGestureMonotony) {
		t.Fatalf("3 uses are allowed, got %v", craftCodes(got))
	}
}

// A camera beat or a framing other than wide counts as a choice.
func TestFilmCraftFramingChoices(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	if !slices.Contains(craftCodes(LintFilmCraft(&s, "", cat)), CodeFramingMonotony) {
		t.Fatal("the run has only wide scenes and must be flagged")
	}
	withFraming := s
	withFraming.Scenes = slices.Clone(s.Scenes)
	withFraming.Scenes[2].Camera = &Camera{Framing: "medium"}
	if slices.Contains(craftCodes(LintFilmCraft(&withFraming, "", cat)), CodeFramingMonotony) {
		t.Fatal("one medium scene is a choice")
	}
	withBeat := s
	withBeat.Scenes = slices.Clone(s.Scenes)
	withBeat.Scenes[4].Beats = append(slices.Clone(s.Scenes[4].Beats), Beat{Do: "camera"})
	if slices.Contains(craftCodes(LintFilmCraft(&withBeat, "", cat)), CodeFramingMonotony) {
		t.Fatal("a camera beat is a choice")
	}
	withTarget := s
	withTarget.Scenes = slices.Clone(s.Scenes)
	withTarget.Scenes[1].Camera = &Camera{Framing: "wide", Target: "left"}
	if slices.Contains(craftCodes(LintFilmCraft(&withTarget, "", cat)), CodeFramingMonotony) {
		t.Fatal("a camera target is a choice")
	}
}

func TestFilmCraftCastUnusedIgnoresTheNarrator(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	s.Scenes[6].Speaker = "narrator"
	if got := LintFilmCraft(&s, "", cat); len(got) != 0 {
		t.Fatalf("narrator is not a cast member and anna still appears: %s", dump(got))
	}
	s.Cast = append(s.Cast, CastMember{ID: "cleo", Name: "Cleo", Character: "char.office-woman-1"})
	s.Cast = append(s.Cast, CastMember{ID: "dan", Name: "Dan", Character: "char.it-tech-1"})
	got := LintFilmCraft(&s, "", cat)
	paths := map[string]bool{}
	for _, is := range got {
		if is.Code == CodeCastUnused {
			paths[is.Path] = true
		}
	}
	if len(paths) != 2 {
		t.Fatalf("two unused members must give two findings with different paths, got %s", dump(got))
	}
}
```
- [ ] **Step 2: Run to see it fail:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestFilmCraft' -count=1`. Expected: FAIL to build (`LintFilmCraft` is not defined).
- [ ] **Step 3: Create `internal/storymaker/studio_craft.go`:**

{% raw %}
```go
package storymaker

import (
	"context"
	"fmt"
	"strings"
)

const (
	craftMinScenes         = 4 // a film of fewer scenes is exempt from every craft lint
	craftEnvironmentsWants = 3 // the environments a long enough film should use
)

// craftGestureChoices are the gestures named in a fix text, all members of GestureNames.
var craftGestureChoices = []string{"present", "thumbs-up", "shrug", "wave", "think", "celebrate", "count", "point"}

// LintFilmCraft finds what Validate accepts but what makes a film look uniform: too few environments, no chosen framing,
// one gesture in too many scenes, a cast member who never appears. It needs a film of at least 4 scenes. The
// dialogue_story template is exempt from the environment rule, because its scenes follow the speaker turns. The fix
// texts name concrete scenes and ids from DirectionPlan. Every finding is a warning; see IssueCraft.
func LintFilmCraft(st *Story, template string, cat *Catalog) []Issue {
	if st == nil || len(st.Scenes) < craftMinScenes {
		return nil
	}
	plan := DirectionPlan(st, template, cat)
	var out []Issue
	if template != "dialogue_story" {
		out = append(out, lintEnvironmentMonotony(st, plan)...)
	}
	out = append(out, lintFramingMonotony(st, plan)...)
	out = append(out, lintGestureMonotony(st)...)
	out = append(out, lintCastUnused(st, plan)...)
	return out
}

// craftIssues runs LintFilmCraft for the job's story. Only a create job gets it: a revise job makes the smallest change
// the user asked for, and a film-level finding on an old story would send the model off to rewrite scenes nobody named.
func (s *Studio) craftIssues(ctx context.Context, story *Story) []Issue {
	if s.job.Kind != AgentKindCreate {
		return nil
	}
	template := ""
	if project, err := s.deps.Projects.Get(ctx, s.job.ProjectID); err == nil {
		template = project.Template
	}
	return LintFilmCraft(story, template, s.deps.Catalog)
}

func lintEnvironmentMonotony(st *Story, plan []SceneDirection) []Issue {
	n := len(st.Scenes)
	want := min(craftEnvironmentsWants, (n+2)/3) // min(3, ceil(n/3))
	have := map[string]bool{}
	for i := range st.Scenes {
		if env := st.Scenes[i].Environment; env != "" {
			have[env] = true
		}
	}
	if len(have) >= want {
		return nil
	}
	var picks []string
	used := map[string]bool{}
	for i := range st.Scenes {
		if len(have)+len(used) >= want {
			break
		}
		env := plan[i].Environment
		if env == "" || have[env] || used[env] {
			continue
		}
		used[env] = true
		picks = append(picks, st.Scenes[i].ID+" -> "+env)
	}
	fix := fmt.Sprintf(`Give some scenes another environment so the film uses at least %d. Send patch_scene with {"environment":"<id>","transition":"wipe"} for those scenes`, want)
	if len(picks) > 0 {
		fix += ", for example " + strings.Join(picks, ", ")
	}
	fix += ". A change of environment never fades: use wipe, slide-left or cut."
	return []Issue{{
		Severity: SeverityWarning, Path: "/scenes", Code: CodeEnvironmentMonotony,
		Message: fmt.Sprintf("All %d scenes play in %d environment(s); the film needs at least %d.", n, len(have), want),
		Fix:     fix,
	}}
}

func lintFramingMonotony(st *Story, plan []SceneDirection) []Issue {
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		// ApplyDefaults always writes a camera, so only a framing other than wide or a target is a choice.
		if sc.Camera != nil && ((sc.Camera.Framing != "" && sc.Camera.Framing != "wide") || sc.Camera.Target != "") {
			return nil
		}
		for j := range sc.Beats {
			if sc.Beats[j].Do == "camera" {
				return nil
			}
		}
	}
	var picks []string
	for i := range st.Scenes {
		if plan[i].Framing != "wide" && len(picks) < 3 {
			picks = append(picks, st.Scenes[i].ID)
		}
	}
	fix := `Choose a framing for the scenes that carry the point: patch_scene with {"camera":{"framing":"medium"}}. Medium follows the speaker, so keep a shown prop on the spot "center" or next to the speaker`
	if len(picks) > 0 {
		fix += "; suggested scenes: " + strings.Join(picks, ", ")
	}
	return []Issue{{
		Severity: SeverityWarning, Path: "/scenes", Code: CodeFramingMonotony,
		Message: "Every scene uses the default wide framing and no scene moves the camera.",
		Fix:     fix + ".",
	}}
}

func lintGestureMonotony(st *Story) []Issue {
	n := len(st.Scenes)
	limit := max(2, (n+2)/3) // max(2, ceil(n/3))
	scenesOf := map[string][]string{}
	var order []string
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		seen := map[string]bool{}
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if b.Do != "gesture" || b.Gesture == "" || seen[b.Gesture] {
				continue
			}
			seen[b.Gesture] = true
			if len(scenesOf[b.Gesture]) == 0 {
				order = append(order, b.Gesture)
			}
			scenesOf[b.Gesture] = append(scenesOf[b.Gesture], sc.ID)
		}
	}
	var out []Issue
	for _, gesture := range order {
		ids := scenesOf[gesture]
		if len(ids) <= limit {
			continue
		}
		var choices []string
		for _, c := range craftGestureChoices {
			if c != gesture && len(choices) < 5 {
				choices = append(choices, c)
			}
		}
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: ids[limit], Path: "/scenes", Code: CodeGestureMonotony,
			Message: fmt.Sprintf("The gesture %q is used in %d of %d scenes (%s).", gesture, len(ids), n, strings.Join(ids, ", ")),
			Fix: fmt.Sprintf("Use %q in at most %d scenes. In %s send patch_scene with the whole beats list of the scene and another gesture, one of %s.",
				gesture, limit, strings.Join(ids[limit:], ", "), strings.Join(choices, ", ")),
		})
	}
	return out
}

func lintCastUnused(st *Story, plan []SceneDirection) []Issue {
	used := map[string]bool{}
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		used[sc.Speaker] = true
		for j := range sc.Actors {
			used[sc.Actors[j].Cast] = true
		}
	}
	var out []Issue
	for k, member := range st.Cast {
		if used[member.ID] {
			continue
		}
		index := len(st.Scenes) / 2
		for i := range plan {
			if i > 0 && plan[i].Actor == member.ID {
				index = i
				break
			}
		}
		sc := &st.Scenes[index]
		at := "right"
		for j := range sc.Actors {
			if sc.Actors[j].At != nil && sc.Actors[j].At.Spot == "right" {
				at = "left"
			}
		}
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: sc.ID, Path: jp("cast", k), Code: CodeCastUnused,
			Message: fmt.Sprintf("%s (%s) is in the cast but never appears.", member.Name, member.ID),
			Fix: fmt.Sprintf(`Give %s a part in scene %s with patch_scene: send the actors list with the actors that are already there plus {"cast":%q,"at":%q,"pose":"stand"}, and set "speaker":%q when %s speaks the narration.`,
				member.Name, sc.ID, member.ID, at, member.ID, member.Name),
		})
	}
	return out
}
```
{% endraw %}
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestFilmCraft' -count=1 -v`. Expected: all PASS. If `TestFilmCraftPassesTheVariedStory` fails, read which finding fired: it is either a threshold bug in the lint or a gap in the fixture, and the fixture is hand-written, so fix the fixture only when the lint is right.
- [ ] **Step 5: Commit:** `gofmt -w internal/storymaker/studio_craft.go internal/storymaker/studio_craft_test.go`, then `git add internal/storymaker/studio_craft.go internal/storymaker/studio_craft_test.go`; message `feat(storymaker): film-level craft lint (environments, framing, gestures, cast)`.

<!-- group A4 -->

### Task A4: Repair `entry_monotony`
**Batch:** A  **Depends on:** A3
**Files:**
Modify `internal/storymaker/studio_check.go`
Modify `internal/storymaker/studio_check_test.go`

The streak is declared once before the scene loop, so it carries across scenes and fires only at exactly 3. It must reset at every scene. The Fix text also names three entry styles the validator refuses (`slide-up`, `draw`, `stamp`); the valid ones are `pop`, `fade`, `slide` and `none` (`validate.go:1104`).

- [ ] **Step 1: Impact:** `LintForJob`. Callers: `runCheck`, `packetIssues`.
- [ ] **Step 2: Implement.**

Three single-line edits in `LintForJob` (the streak variables move into the scene loop). Each Old line is unique in the file.

**Edit** `internal/storymaker/studio_check.go`
Old:
```go
lastEntry := ""
```
New: remove the text (leave nothing). `gofmt` drops the blank line.

**Edit** `internal/storymaker/studio_check.go`
Old:
```go
entryStreak := 0
```
New: remove the text (leave nothing).

**Edit** `internal/storymaker/studio_check.go`
Old:
```go
if !studioSceneFilled(sc) {
```
New:
```go
// The streak of equal entries counts inside one scene: a new scene starts a new reveal sequence.
		lastEntry, entryStreak := "", 0
		if !studioSceneFilled(sc) {
```

**Edit** `internal/storymaker/studio_check.go`
Old:
```go
						Fix:     `Vary the entry style across reveals (pop, fade, slide-up, draw or stamp).`,
```
New:
```go
						Fix:     `Vary the entry style across reveals: pop, fade or slide.`,
```

**Edit** `internal/storymaker/studio_check.go`
Old:
```go
// for three or more consecutive show beats with the same entry style, and spot_overlap when
```
New:
```go
// for three or more consecutive show beats of one scene with the same entry style, and spot_overlap when
```

**Edit** `internal/storymaker/studio_check_test.go`: the streak no longer carries from s1 to s2, so s1 gets its own third `pop` reveal. In `TestStudioCheckFindsStageOverlapMonotonyAndCompileWarnings`:
Old:
```go
{"do":"show","item":"prop.shield","at":"right","entry":"pop"}]}}`)
```
New:
```go
{"do":"show","item":"prop.shield","at":"right","entry":"pop"},{"do":"show","item":"prop.key","at":"center","entry":"pop"}]}}`)
```
(The Old line is the end of the `put_scene` call for `s1`; the call for `s2` has `"at":"left","entry":"pop"}]}}` and does not match.)

**Edit** the same file
Old:
```go
// Walking away before showing at "left" resolves spot_overlap in s2, while fade across environments and 3rd "pop" entry trigger advice.
```
New:
```go
// Walking away before showing at "left" resolves spot_overlap in s2, while fade across environments triggers advice; s1 holds the third "pop" in a row.
```
- [ ] **Step 3: Add a test for the new rule.** Append to `internal/storymaker/studio_check_test.go`:
{% raw %}
```go

func TestLintForJobEntryStreakResetsPerScene(t *testing.T) {
	show := func(entry string) Beat { return Beat{Do: "show", Item: "prop.lock", At: SpotAt("right"), Entry: entry} }
	actors := []Actor{{Cast: "anna", At: SpotAt("left")}}
	story := &Story{Scenes: []Scene{
		{ID: "s1", Actors: actors, Beats: []Beat{show("pop"), show("pop")}},
		{ID: "s2", Actors: actors, Beats: []Beat{show("pop"), show("pop")}},
		{ID: "s3", Actors: actors, Beats: []Beat{show("fade"), show("fade"), show("fade"), show("pop"), show("pop"), show("pop")}},
	}}
	var got []string
	for _, is := range LintForJob(story) {
		if is.Code == CodeEntryMonotony {
			got = append(got, is.SceneID)
		}
	}
	if len(got) != 2 || got[0] != "s3" || got[1] != "s3" {
		t.Fatalf("entry_monotony scenes = %v, want s3 twice (two streaks of three) and nothing for two scenes of two", got)
	}
}
```
{% endraw %}
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestStudioCheck|TestLintForJobEntryStreakResetsPerScene)' -count=1`. Expected: PASS.
- [ ] **Step 5: Commit:** `gofmt -w internal/storymaker/studio_check.go internal/storymaker/studio_check_test.go`; `git add` both; message `fix(storymaker): reset the entry_monotony streak per scene and name only valid entries`.

<!-- group A5 -->

### Task A5: Run the craft lint in the check
**Batch:** A  **Depends on:** A3, A4
**Files:**
Modify `internal/storymaker/studio_check.go`
Test `internal/storymaker/studio_craft_check_test.go`

- [ ] **Step 1: Impact:** `runCheck`, `Studio.Check`. Callers: `Studio.Check`, `studioValidateOp`, `Packet` (check phase), the server's `check`. Report the risk.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/studio_check.go`
Old:
```go
found = append(found, LintForJob(&story)...)
```
New:
```go
found = append(found, LintForJob(&story)...)
	found = append(found, s.craftIssues(ctx, &story)...)
```
- [ ] **Step 3: Test.** Create `internal/storymaker/studio_craft_check_test.go`:

```go
package storymaker

import (
	"context"
	"os"
	"slices"
	"testing"
)

// saveFixture replaces the story of the rig's project with a fixture file.
func saveFixture(t *testing.T, ps *ProjectStore, projectID, name string) {
	t.Helper()
	data, err := os.ReadFile("testdata/stories/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(context.Background(), projectID, data, ""); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsCraftFindingsForACreateJobOnly(t *testing.T) {
	ctx := context.Background()
	studio, ps, projectID := newStudioWriteRig(t, nil)
	saveFixture(t, ps, projectID, "agent-firewall-run")

	report, err := studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, is := range report.Issues {
		codes = append(codes, is.Code)
	}
	for _, want := range []string{CodeEnvironmentMonotony, CodeFramingMonotony, CodeGestureMonotony, CodeCastUnused} {
		if !slices.Contains(codes, want) {
			t.Errorf("issues = %v, missing %s", codes, want)
		}
	}
	outcome, checks := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	if outcome != OutcomeReadyUnverified {
		t.Errorf("outcome = %s (%v): craft findings must not block", outcome, checks)
	}

	studio.job.Kind = AgentKindRevise
	report, err = studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range report.Issues {
		if IssueCraft(is.Code) {
			t.Errorf("a revise job must not get %s", is.Code)
		}
	}
}

func TestCheckLeavesTheVariedStoryClean(t *testing.T) {
	ctx := context.Background()
	studio, ps, projectID := newStudioWriteRig(t, nil)
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range report.Issues {
		if IssueCraft(is.Code) {
			t.Errorf("the varied story must pass the craft lint, got %+v", is)
		}
	}
}
```
The rig's project id is the third return value of `newStudioWriteRig` (see `studio_story_scenes_test.go:10-48`). If it is not a project id, read the rig and use the id it passes to `NewStudio` (`StudioJob.ProjectID`).
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestCheck|TestStudioCheck|TestDecideOutcome)' -count=1`. Expected: PASS.
- [ ] **Step 5: Commit:** `gofmt -w` both files; `git add internal/storymaker/studio_check.go internal/storymaker/studio_craft_check_test.go`; message `feat(storymaker): the create job's check runs the craft lint`.

<!-- group A6 -->

### Task A6: Environment guide data
**Batch:** A  **Depends on:** A1
**Files:**
Create `internal/storymaker/data/env_direction.json`
Create `internal/storymaker/studio_direction_data.go`
Create `internal/storymaker/studio_direction_data_test.go`

The guide is curated: one line about each environment and the words that mean the environment fits a topic, in English and German (the languages of the films so far). `base: true` marks the environments that fit many topics: they rank above specific ones when no word matches.

- [ ] **Step 1: Look at the art once.** Open the 12 thumbnails `ui/img/storymaker/thumbs/env.<id>.png` with the Read tool (they are PNG) and compare each with its `about` line below. Correct a line that does not match what you see, and keep the rest. This is the only place the description is checked against a picture.
- [ ] **Step 2: Create `internal/storymaker/data/env_direction.json`:**

```json
{
  "env.plain-stage": {
    "about": "Neutral empty stage. Use it only when nothing else fits.",
    "tags": []
  },
  "env.office": {
    "about": "Open office with desk, door and window. Work, teams, meetings, business.",
    "tags": ["office", "work", "team", "meeting", "business", "company", "employee", "colleague", "büro", "arbeit", "firma", "unternehmen", "mitarbeiter", "kolleg", "besprechung", "geschäft"],
    "base": true
  },
  "env.home-office": {
    "about": "Home workplace with desk and window. Remote work, private devices, everyday users.",
    "tags": ["home", "remote", "private", "everyday", "personal", "zuhause", "homeoffice", "privat", "alltag", "persönlich", "heimnetz", "telearbeit"],
    "base": true
  },
  "env.abstract-tech": {
    "about": "Abstract digital space. Networks, data, code, cloud, security concepts.",
    "tags": ["network", "data", "code", "cloud", "digital", "internet", "security", "firewall", "encryption", "packet", "online", "software", "attack", "hacker", "virus", "malware", "netzwerk", "daten", "verschlüssel", "sicherheit", "paket", "schutz", "angriff"],
    "base": true
  },
  "env.server-room": {
    "about": "Server racks and cables. Infrastructure, hosting, backups, operations.",
    "tags": ["server", "rack", "hosting", "infrastructure", "backup", "database", "datacenter", "operations", "serverraum", "rechenzentrum", "infrastruktur", "sicherung", "datenbank", "betrieb"],
    "base": true
  },
  "env.classroom": {
    "about": "School classroom with a board. Learning, training, explaining step by step.",
    "tags": ["learn", "school", "teach", "lesson", "training", "student", "course", "explain", "lernen", "schule", "unterricht", "lehrer", "schüler", "kurs", "ausbildung", "erklär"],
    "base": true
  },
  "env.laboratory": {
    "about": "Research laboratory. Experiments, science, testing, measuring.",
    "tags": ["science", "experiment", "research", "test", "measure", "chemistry", "biology", "lab", "wissenschaft", "forschung", "versuch", "messung", "chemie", "labor", "analyse"]
  },
  "env.workshop": {
    "about": "Workshop with tools. Building, repairing, craft, hardware.",
    "tags": ["build", "repair", "tool", "craft", "hardware", "machine", "maintenance", "bauen", "reparatur", "werkzeug", "handwerk", "maschine", "wartung", "werkstatt"]
  },
  "env.medical-office": {
    "about": "Doctor's office. Health, care, patients, medicine.",
    "tags": ["health", "doctor", "patient", "medicine", "care", "clinic", "hospital", "gesundheit", "arzt", "ärztin", "medizin", "pflege", "praxis", "krank"]
  },
  "env.living-room": {
    "about": "Living room. Family, home, leisure, everyday life, consumer devices.",
    "tags": ["family", "leisure", "tv", "consumer", "comfort", "familie", "wohnzimmer", "freizeit", "fernseher", "gemütlich", "kinder", "smart", "haushalt"],
    "base": true
  },
  "env.city-street": {
    "about": "City street. Traffic, shops, mobility, public places, the outside world.",
    "tags": ["city", "street", "traffic", "shop", "mobility", "public", "outside", "stadt", "straße", "verkehr", "laden", "öffentlich", "unterwegs", "mobil", "kunden"]
  },
  "env.park": {
    "about": "Park with trees. Nature, relaxation, outdoors, a calm ending.",
    "tags": ["nature", "park", "outdoor", "relax", "environment", "green", "tree", "natur", "draußen", "entspann", "umwelt", "grün", "baum", "spazier", "erholung"]
  }
}
```
- [ ] **Step 3: Create `internal/storymaker/studio_direction_data.go`:**

```go
package storymaker

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed data/env_direction.json
var envDirectionJSON []byte

// plainStageID is the neutral environment that the direction uses only when nothing else exists.
const plainStageID = "env.plain-stage"

// EnvDirection is the curated guide of one catalog environment. About is one line for the model, Tags are the words
// (English and German, matched as whole words or as prefixes of at least 4 letters) that mean the environment fits a
// topic, and Base marks an environment that fits many topics.
type EnvDirection struct {
	About string   `json:"about"`
	Tags  []string `json:"tags"`
	Base  bool     `json:"base,omitempty"`
}

var (
	envDirectionOnce sync.Once
	envDirectionData map[string]EnvDirection
)

// EnvironmentGuide returns the curated guide of the catalog environments, keyed by id. The map is shared: do not change it.
// The file is embedded and a test ties it to the catalog, so a decode error means a broken build and leaves the guide empty.
func EnvironmentGuide() map[string]EnvDirection {
	envDirectionOnce.Do(func() {
		envDirectionData = map[string]EnvDirection{}
		_ = json.Unmarshal(envDirectionJSON, &envDirectionData)
	})
	return envDirectionData
}

// environmentAbout is the one-line guide of an environment, or "".
func environmentAbout(id string) string {
	return EnvironmentGuide()[id].About
}
```
- [ ] **Step 4: Create `internal/storymaker/studio_direction_data_test.go`:**

```go
package storymaker

import (
	"strings"
	"testing"
)

// The guide and the catalog must keep agreeing: every environment has a line, no line names a missing environment.
func TestEnvironmentGuideCoversTheCatalog(t *testing.T) {
	cat := loadReal(t)
	guide := EnvironmentGuide()
	if len(guide) == 0 {
		t.Fatal("the embedded guide did not decode")
	}
	for _, id := range cat.Names("environment") {
		g, ok := guide[id]
		if !ok || strings.TrimSpace(g.About) == "" {
			t.Errorf("the catalog environment %s has no guide line", id)
		}
		if id != plainStageID && len(g.Tags) == 0 {
			t.Errorf("%s has no tags", id)
		}
	}
	for id := range guide {
		if _, ok := cat.Lookup(id); !ok {
			t.Errorf("the guide names %s, which the catalog lacks", id)
		}
	}
}
```
- [ ] **Step 5: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestEnvironmentGuideCoversTheCatalog$' -count=1`. Expected: PASS.
- [ ] **Step 6: Commit:** `gofmt -w` the two Go files; `git add internal/storymaker/data/env_direction.json internal/storymaker/studio_direction_data.go internal/storymaker/studio_direction_data_test.go`; message `feat(storymaker): curated environment guide for the direction packet`.

<!-- group A7 -->

### Task A7: The direction plan
**Batch:** A  **Depends on:** A6
**Files:**
Create `internal/storymaker/studio_direction.go`
Create `internal/storymaker/studio_direction_test.go`

- [ ] **Step 1: Create `internal/storymaker/studio_direction.go`:**

```go
package storymaker

import (
	"hash/fnv"
	"sort"
	"strings"
	"unicode"
)

// The roles of the arc the outline asks for.
const (
	RoleHook     = "hook"
	RoleProblem  = "problem"
	RoleSolution = "solution"
	RoleHow      = "how"
	RoleBenefit  = "benefit"
	RoleClose    = "close"
)

// directionNote is the sentence that travels with every direction.
const directionNote = "A suggestion for staging this scene. Keep it unless the narration needs something else. Scenes with different roles need different looks."

// directionStyle is what a role asks for: a framing, a pose and a gesture. The values are members of the closed name sets
// (a test checks them). Medium follows the speaker; wide shows the whole stage.
var directionStyles = map[string]struct{ framing, pose, gesture string }{
	RoleHook:     {"medium", "present", "wave"},
	RoleProblem:  {"medium", "think", "shrug"},
	RoleSolution: {"wide", "stand", "present"},
	RoleHow:      {"medium", "present", "point"},
	RoleBenefit:  {"wide", "stand", "thumbs-up"},
	RoleClose:    {"medium", "stand", "celebrate"},
}

// directionEntries are the show entries the validator accepts, rotated by scene.
var directionEntries = []string{"pop", "fade", "slide"}

// SceneDirection is the staging suggestion of one scene. It is computed from the story, never stored in it.
type SceneDirection struct {
	Role        string
	Environment string
	Framing     string
	Pose        string
	Gesture     string
	Entry       string
	Actor       string // cast id; "" without a cast and in a dialogue_story
	ActorAt     string
	HeroProp    string // catalog id of the best prop for the narration; "" when none matches
	PropAt      string
}

// packet is the direction as the fill packet shows it.
func (d SceneDirection) packet() map[string]any {
	out := map[string]any{
		"role": d.Role, "environment": d.Environment, "framing": d.Framing, "pose": d.Pose, "gesture": d.Gesture,
		"entry": d.Entry, "actor_at": d.ActorAt, "prop_at": d.PropAt, "note": directionNote,
	}
	if about := environmentAbout(d.Environment); about != "" {
		out["environment_about"] = about
	}
	if d.Actor != "" {
		out["actor"] = d.Actor
	}
	if d.HeroProp != "" {
		out["hero_prop"] = d.HeroProp
	}
	return out
}

// DirectionPlan computes the direction of every scene of the story. It is a pure function of the title, the narrations, the
// cast, the template and the catalog, so every fill packet of one film sees the same plan. A film of at least 3 environments
// gets three: the best match goes to the middle act (solution and how), the second best to the first act (hook and
// problem), the third to the last act (benefit and close). plain-stage is used only when nothing else exists.
func DirectionPlan(story *Story, template string, cat *Catalog) []SceneDirection {
	if story == nil || len(story.Scenes) == 0 {
		return nil
	}
	n := len(story.Scenes)
	ranked := rankEnvironments(story, cat)
	roles := directionRoles(n)
	plan := make([]SceneDirection, n)
	for i := range plan {
		role := roles[i]
		style := directionStyles[role]
		d := SceneDirection{
			Role: role, Environment: actEnvironment(ranked, roleAct(role)),
			Framing: style.framing, Pose: style.pose, Gesture: style.gesture,
			Entry:   directionEntries[i%len(directionEntries)],
			ActorAt: "left", PropAt: "right",
		}
		if i%2 == 1 {
			d.ActorAt, d.PropAt = "right", "left"
		}
		if style.framing == "medium" {
			d.PropAt = "center" // the camera follows the speaker, so the prop stays within reach
		}
		if template != "dialogue_story" && len(story.Cast) > 0 {
			d.Actor = story.Cast[i%len(story.Cast)].ID
		}
		if props := packetSuggestedProps(cat, story.Scenes[i].Narration); len(props) > 0 {
			d.HeroProp = props[0]
		}
		plan[i] = d
	}
	return plan
}

// directionRoles gives the role of each of n scenes: the first is the hook, the last the close, and the scenes between
// follow t = i/(n-1): problem below 0.25, solution below 0.5, how below 0.75, else benefit.
func directionRoles(n int) []string {
	out := make([]string, n)
	for i := range out {
		switch {
		case i == 0:
			out[i] = RoleHook
		case i == n-1:
			out[i] = RoleClose
		default:
			switch t := float64(i) / float64(n-1); {
			case t < 0.25:
				out[i] = RoleProblem
			case t < 0.5:
				out[i] = RoleSolution
			case t < 0.75:
				out[i] = RoleHow
			default:
				out[i] = RoleBenefit
			}
		}
	}
	return out
}

// roleAct is 0 for hook and problem, 1 for solution and how, 2 for benefit and close.
func roleAct(role string) int {
	switch role {
	case RoleHook, RoleProblem:
		return 0
	case RoleSolution, RoleHow:
		return 1
	}
	return 2
}

// actEnvironment picks the environment of an act from the ranked list: the best match plays the middle act.
func actEnvironment(ranked []string, act int) string {
	switch len(ranked) {
	case 0:
		return ""
	case 1:
		return ranked[0]
	case 2:
		return [3]string{ranked[1], ranked[0], ranked[1]}[act]
	}
	return [3]string{ranked[1], ranked[0], ranked[2]}[act]
}

// rankEnvironments orders the catalog environments for this film: each tag that matches a word of the title or of the
// narrations scores 2, a base environment scores 1 more, plain-stage comes last, and ties rotate by a hash of the title so
// that two films about the same thing but with different titles do not both start on the same environment.
func rankEnvironments(story *Story, cat *Catalog) []string {
	ids := cat.Names("environment")
	if len(ids) == 0 {
		return nil
	}
	guide := EnvironmentGuide()
	words := directionWords(story)
	seed := fnv.New32a()
	_, _ = seed.Write([]byte(story.Title))
	start := int(seed.Sum32() % uint32(len(ids)))
	type row struct {
		id            string
		score, rotate int
	}
	rows := make([]row, 0, len(ids))
	for k, id := range ids {
		g := guide[id]
		score := 0
		for _, tag := range g.Tags {
			if tagMatches(words, tag) {
				score += 2
			}
		}
		if g.Base {
			score++
		}
		rows = append(rows, row{id: id, score: score, rotate: (k - start + len(ids)) % len(ids)})
	}
	sort.SliceStable(rows, func(a, b int) bool {
		plainA, plainB := rows[a].id == plainStageID, rows[b].id == plainStageID
		if plainA != plainB {
			return plainB
		}
		if rows[a].score != rows[b].score {
			return rows[a].score > rows[b].score
		}
		return rows[a].rotate < rows[b].rotate
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.id
	}
	return out
}

// directionWords are the lower-case words of at least 3 letters of the title and the narrations, markers removed.
func directionWords(story *Story) []string {
	var b strings.Builder
	b.WriteString(story.Title)
	for i := range story.Scenes {
		b.WriteByte(' ')
		b.WriteString(StripMarkers(story.Scenes[i].Narration))
	}
	fields := strings.FieldsFunc(strings.ToLower(b.String()), func(r rune) bool { return !unicode.IsLetter(r) })
	out := fields[:0]
	for _, f := range fields {
		if len([]rune(f)) >= 3 {
			out = append(out, f)
		}
	}
	return out
}

// tagMatches is true when a word equals the tag, or starts with it and the tag has at least 4 letters ("daten" matches
// "datenpakete").
func tagMatches(words []string, tag string) bool {
	tag = strings.ToLower(tag)
	long := len([]rune(tag)) >= 4
	for _, w := range words {
		if w == tag || (long && strings.HasPrefix(w, tag)) {
			return true
		}
	}
	return false
}
```
- [ ] **Step 2: Create `internal/storymaker/studio_direction_test.go`:**

{% raw %}
```go
package storymaker

import (
	"slices"
	"testing"
)

func TestDirectionRoles(t *testing.T) {
	want := map[int][]string{
		1: {RoleHook},
		2: {RoleHook, RoleClose},
		3: {RoleHook, RoleHow, RoleClose},
		7: {RoleHook, RoleProblem, RoleSolution, RoleHow, RoleHow, RoleBenefit, RoleClose},
	}
	for n, roles := range want {
		if got := directionRoles(n); !slices.Equal(got, roles) {
			t.Errorf("directionRoles(%d) = %v, want %v", n, got, roles)
		}
	}
}

// Every suggested value is a member of a closed set or a spot that every environment has.
func TestDirectionStylesUseValidNames(t *testing.T) {
	cat := loadReal(t)
	for role, style := range directionStyles {
		if !slices.Contains(PoseNames, style.pose) || !slices.Contains(GestureNames, style.gesture) || !slices.Contains([]string{"wide", "medium"}, style.framing) {
			t.Errorf("%s: %+v holds an invalid name", role, style)
		}
	}
	for _, entry := range directionEntries {
		if !slices.Contains([]string{"pop", "fade", "slide"}, entry) {
			t.Errorf("entry %q is not accepted by the validator", entry)
		}
	}
	for _, id := range cat.Names("environment") {
		spots := cat.SpotNames(id)
		for _, spot := range []string{"left", "right", "center"} {
			if !slices.Contains(spots, spot) {
				t.Errorf("%s lacks the spot %s", id, spot)
			}
		}
	}
	for _, choice := range craftGestureChoices {
		if !slices.Contains(GestureNames, choice) {
			t.Errorf("craftGestureChoices holds %q, which is not a gesture", choice)
		}
	}
}

func TestDirectionPlanForTheRealFirewallRun(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	plan := DirectionPlan(&s, "", cat)
	if len(plan) != 7 || plan[0].Role != RoleHook || plan[6].Role != RoleClose {
		t.Fatalf("plan = %+v", plan)
	}
	envs := map[string]bool{}
	for _, d := range plan {
		if d.Environment == "" || d.Environment == plainStageID {
			t.Errorf("a scene is planned on %q", d.Environment)
		}
		envs[d.Environment] = true
	}
	if len(envs) != 3 {
		t.Errorf("a film of 7 scenes gets 3 environments, got %v", envs)
	}
	if plan[2].Environment != "env.abstract-tech" {
		t.Errorf("a story about a firewall puts its solution on abstract-tech, got %s", plan[2].Environment)
	}
	if plan[3].Environment != plan[2].Environment || plan[0].Environment != plan[1].Environment || plan[5].Environment != plan[6].Environment {
		t.Errorf("an act keeps one environment: %+v", plan)
	}
	if plan[0].Actor != "tobias" || plan[1].Actor != "laura" {
		t.Errorf("two cast members alternate: %s, %s", plan[0].Actor, plan[1].Actor)
	}
	if plan[0].ActorAt != "left" || plan[1].ActorAt != "right" {
		t.Errorf("the actor spot alternates: %s, %s", plan[0].ActorAt, plan[1].ActorAt)
	}
	if again := DirectionPlan(&s, "", cat); !slices.EqualFunc(plan, again, func(a, b SceneDirection) bool { return a == b }) {
		t.Error("the plan is not deterministic")
	}
}

func TestDirectionPlanDialogueStoryHasNoActorSuggestion(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for _, d := range DirectionPlan(&s, "dialogue_story", cat) {
		if d.Actor != "" {
			t.Fatalf("a dialogue_story follows its speaker turns, got actor %q", d.Actor)
		}
	}
}

func TestDirectionPlanWithoutCatalogOrCastIsSafe(t *testing.T) {
	s := &Story{Title: "x", Scenes: []Scene{{ID: "s1", Narration: "a"}, {ID: "s2", Narration: "b"}}}
	plan := DirectionPlan(s, "", nil)
	if len(plan) != 2 || plan[0].Environment != "" || plan[0].Actor != "" {
		t.Fatalf("plan = %+v", plan)
	}
	if DirectionPlan(nil, "", nil) != nil || DirectionPlan(&Story{}, "", nil) != nil {
		t.Fatal("an empty story has no plan")
	}
}

func TestTagMatchesWholeWordsAndLongPrefixes(t *testing.T) {
	words := []string{"datenpakete", "tv", "firewall"}
	for tag, want := range map[string]bool{"daten": true, "paket": false, "firewall": true, "tv": true, "fire": true, "tvs": false} {
		if got := tagMatches(words, tag); got != want {
			t.Errorf("tagMatches(%q) = %v, want %v", tag, got, want)
		}
	}
}
```
{% endraw %}
- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestDirection|TestTagMatches|TestEnvironmentGuide)' -count=1 -v`. Expected: all PASS. If `TestDirectionPlanForTheRealFirewallRun` fails on `plan[2].Environment`, print the ranking with a temporary `t.Logf` of `rankEnvironments` and fix the tag lists in `env_direction.json`, not the test: the test states what a firewall story should get.
- [ ] **Step 4: Commit:** `gofmt -w` the two files; `git add internal/storymaker/studio_direction.go internal/storymaker/studio_direction_test.go`; message `feat(storymaker): direction plan (role, environment, framing, gesture, cast, prop spot)`.

<!-- group A8 -->

### Task A8: Direction in the packets, worked example from the direction
**Batch:** A  **Depends on:** A7
**Files:**
Modify `internal/storymaker/studio_packet.go`
Modify `internal/storymaker/studio_check_test.go`
Test `internal/storymaker/studio_direction_packet_test.go`

- [ ] **Step 1: Impact:** `Packet`, `packetFill`, `packetFillExample`, `packetEnvironments`. Callers of `packetFillExample`: `packetFill` only (grep to confirm).
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/studio_packet.go`: outline and cast get an `environment_guide` next to the unchanged `environments` (a test asserts `environments` is a `[]string`).
Old:
```go
packet["templates"] = packetTemplates
```
New:
```go
packet["templates"] = packetTemplates
		packet["environment_guide"] = packetEnvironmentGuide(deps.Catalog)
```
Old:
```go
packet["characters"] = deps.Catalog.Names("character")
```
New:
```go
packet["characters"] = deps.Catalog.Names("character")
		packet["environment_guide"] = packetEnvironmentGuide(deps.Catalog)
```

**Edit** the worked example (replace the whole function and its comment):
Old:
```go
// packetFillExample is the worked example of a fill scene. It names the first cast member of the story and
// leaves the actor and the gesture out when there is no cast, so it never names a member that does not exist.
func packetFillExample(cast []CastMember) map[string]any {
	example := map[string]any{
		"narration":   "Eine {1}Firewall prüft jedes Paket am Eingang.",
		"environment": "env.office",
		"transition":  "wipe",
	}
	beats := []any{map[string]any{"do": "show", "item": "prop.lock", "at": "right", "entry": "pop", "cue": 1}}
	if len(cast) > 0 {
		who := cast[0].ID
		example["speaker"] = who
		example["actors"] = []any{map[string]any{"cast": who, "at": "left", "pose": "present", "expression": "neutral"}}
		beats = append(beats, map[string]any{"do": "gesture", "who": who, "gesture": "point", "with": "prev"})
	}
	example["beats"] = beats
	return example
}
```
New:
```go
// packetFillExample is the worked example of a fill scene. It is built from the scene's own direction (environment,
// framing, actor, spots, pose, gesture, entry), so it can never contradict it and every scene gets a different one. It
// names the direction's actor, or the first cast member, and leaves the actor and the gesture out when there is no cast.
func packetFillExample(cast []CastMember, dir SceneDirection) map[string]any {
	environment := dir.Environment
	if environment == "" {
		environment = "env.office"
	}
	prop := dir.HeroProp
	if prop == "" {
		prop = "prop.lock"
	}
	propAt, entry := dir.PropAt, dir.Entry
	if propAt == "" {
		propAt = "right"
	}
	if entry == "" {
		entry = "pop"
	}
	example := map[string]any{
		"narration":   "Eine {1}Firewall prüft jedes Paket am Eingang.",
		"environment": environment,
		"transition":  "wipe",
	}
	if dir.Framing != "" && dir.Framing != "wide" {
		example["camera"] = map[string]any{"framing": dir.Framing}
	}
	beats := []any{map[string]any{"do": "show", "item": prop, "at": propAt, "entry": entry, "cue": 1}}
	if len(cast) > 0 {
		who := cast[0].ID
		if dir.Actor != "" {
			who = dir.Actor
		}
		actorAt, pose, gesture := dir.ActorAt, dir.Pose, dir.Gesture
		if actorAt == "" {
			actorAt = "left"
		}
		if pose == "" {
			pose = "present"
		}
		if gesture == "" {
			gesture = "present"
		}
		example["speaker"] = who
		example["actors"] = []any{map[string]any{"cast": who, "at": actorAt, "pose": pose, "expression": "neutral"}}
		beats = append(beats, map[string]any{"do": "gesture", "who": who, "gesture": gesture, "with": "prev"})
	}
	example["beats"] = beats
	return example
}
```

**Edit** `internal/storymaker/studio_packet.go` in `packetFill` (one line becomes three):
Old:
```go
packet["example"] = packetFillExample(story.Cast)
```
New:
```go
direction := DirectionPlan(story, project.Template, deps.Catalog)[index]
	packet["direction"] = direction.packet()
	packet["example"] = packetFillExample(story.Cast, direction)
```
`project` and `index` are already in scope there (`packetFill` reads the project for the word budget and finds the scene index first).

**Edit** `internal/storymaker/studio_packet.go`: the environment lists get the guide line.
Old:
```go
out = append(out, map[string]any{"id": id, "spots": cat.SpotNames(id)})
```
New:
```go
row := map[string]any{"id": id, "spots": cat.SpotNames(id)}
		if about := environmentAbout(id); about != "" {
			row["about"] = about
		}
		out = append(out, row)
```

**Edit** `internal/storymaker/studio_packet.go`: the guide list for the outline and cast packets.
Old:
```go
func packetSummary(story *Story) map[string]any {
```
New:
```go
// packetEnvironmentGuide lists what each environment is for, without the spots. The outline and cast packets carry it
// next to the plain list of names.
func packetEnvironmentGuide(cat *Catalog) []map[string]any {
	ids := cat.Names("environment")
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		if about := environmentAbout(id); about != "" {
			out = append(out, map[string]any{"id": id, "about": about})
		}
	}
	return out
}

func packetSummary(story *Story) map[string]any {
```
- [ ] **Step 3: Test.** Create `internal/storymaker/studio_direction_packet_test.go`:

```go
package storymaker

import (
	"context"
	"encoding/json"
	"testing"
)

// Every example of the fill packet is a scene the validator accepts: it is built from the scene's own direction.
func TestFillExamplesValidate(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i, d := range DirectionPlan(&s, "", cat) {
		raw, err := json.Marshal(packetFillExample(s.Cast, d))
		if err != nil {
			t.Fatal(err)
		}
		var sc Scene
		if err := json.Unmarshal(raw, &sc); err != nil {
			t.Fatalf("scene %d: %v: %s", i+1, err, raw)
		}
		sc.ID = "s1"
		one := s
		one.Scenes = []Scene{sc}
		if issues := Validate(&one, cat, DefaultLimits()); HasErrors(issues) {
			t.Errorf("example %d (%s): %s\n%s", i+1, d.Role, dump(issues), raw)
		}
	}
}

func TestFillPacketCarriesTheDirectionAndAnExampleFromIt(t *testing.T) {
	ctx := context.Background()
	studio, ps, projectID := newStudioWriteRig(t, nil)
	saveFixture(t, ps, projectID, "agent-firewall-run")

	studio.SetPhase(PhaseFill, "s3")
	packet, err := studio.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := packet["direction"].(map[string]any)
	example, _ := packet["example"].(map[string]any)
	if dir == nil || example == nil {
		t.Fatalf("packet lacks direction or example: %v", packet)
	}
	if dir["role"] != RoleSolution || dir["environment"] != "env.abstract-tech" || dir["note"] != directionNote {
		t.Errorf("direction of s3 = %v", dir)
	}
	if example["environment"] != dir["environment"] {
		t.Errorf("the example must use the direction's environment: %v vs %v", example["environment"], dir["environment"])
	}
	envs, _ := packet["environments"].([]map[string]any)
	if len(envs) == 0 || envs[0]["about"] == nil {
		t.Errorf("the environments of a fill packet carry the guide line: %v", envs)
	}
	// Different scenes get different directions and examples.
	studio.SetPhase(PhaseFill, "s1")
	other, err := studio.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	otherDir, _ := other["direction"].(map[string]any)
	if otherDir["role"] != RoleHook || otherDir["environment"] == dir["environment"] {
		t.Errorf("s1 must be the hook in another environment than s3: %v", otherDir)
	}
}

func TestOutlineAndCastPacketsCarryTheEnvironmentGuide(t *testing.T) {
	ctx := context.Background()
	studio, _, _ := newStudioWriteRig(t, nil)
	for _, phase := range []string{PhaseOutline, PhaseCast} {
		studio.SetPhase(phase, "")
		packet, err := studio.Packet(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if names, _ := packet["environments"].([]string); len(names) == 0 {
			t.Errorf("%s: environments must stay a list of names: %v", phase, packet["environments"])
		}
		if guide, _ := packet["environment_guide"].([]map[string]any); len(guide) < 12 {
			t.Errorf("%s: environment_guide = %v", phase, packet["environment_guide"])
		}
	}
}
```
The rig has `SetPhase` (used in `studio_check_test.go:96`).
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestFillPacket|TestOutlineAndCast|TestFillExamplesValidate|TestStudioPacket|TestStudioCheck|TestFilmCraft|TestDirection)' -count=1`. Expected: PASS.
- [ ] **Step 5: Commit:** `gofmt -w` the three files; `git add internal/storymaker/studio_packet.go internal/storymaker/studio_direction_packet_test.go internal/storymaker/studio_check_test.go`; message `feat(storymaker): direction packet and a worked example built from it`.

<!-- group A9 -->

### Task A9: Prompt texts, version v3
**Batch:** A  **Depends on:** A8
**Files:**
Modify `internal/server/storymaker_agent_profile.go`
Modify `internal/server/storymaker_agent_profile_test.go`

A published text is never edited in place (`storymaker_agent_profile.go:11`), so the profile name moves from `v2` to `v3`. The texts also named three entry styles the validator refuses (`slide-up`, `draw`, `stamp`).

- [ ] **Step 1: Impact:** `storymakerAgentProfile`, `storymakerFillWorkflow`, `storymakerCheckWorkflow`. Report callers.
- [ ] **Step 2: Implement.**

**Edit** `internal/server/storymaker_agent_profile.go` (house rules, rule 7)
Old:
```go
7. Vary the entries (pop, slide-up, fade, draw, stamp) across scenes. Text stops moving after it lands.
```
New:
```go
7. Vary the entries (pop, fade, slide) across scenes. Text stops moving after it lands.
```

**Edit** (house style, last line)
Old:
```go
- Vary the show entries (pop, slide-up, fade, draw, stamp) across scenes (check used_entries in the context) and use the camera sparingly, for one reason per scene.`
```
New:
```go
- Vary the show entries (pop, fade, slide) across scenes (check used_entries in the context). Use the camera for a reason: framing medium follows the speaker.
- A film looks alike when every scene has the same environment, the same framing, the same gesture and the same person. Plan the film as three acts (hook and problem, solution and how, benefit and close) and change the environment between acts.`
```

**Edit** (outline workflow, step 2)
Old:
```go
2. Build the arc hook, problem, solution, how, benefit, close, using suggested_scenes and the stageable environments from the context. The dialogue_story template uses one scene per speaker turn.
```
New:
```go
2. Build the arc hook, problem, solution, how, benefit, close, using suggested_scenes. The context lists the environments with a line about each in environment_guide. The dialogue_story template uses one scene per speaker turn.
```

**Edit** (fill workflow: replace the spot and continuity bullet and add the direction bullet)
Old:
```go
- Use spot names from the context for at, from and to. Keep actors and shown props/text on separate spots so they never overlap, and maintain visual continuity with previous_scene when staying in the same environment.
```
New:
```go
- The context has a direction for this scene: its role, a suggested environment, framing, pose, gesture, entry, actor and the spots of the actor and the prop. Follow it unless the narration needs something else. The example in the context is built from it.
- Keep the same environment only while the scene's role stays the same, and do not repeat the gesture of previous_scene. The server checks the whole film: at least three environments in a long film, a framing other than wide in at least one scene, varied gestures, and every cast member on stage in some scene.
- Use spot names from the context for at, from and to. Keep actors and shown props/text on separate spots so they never overlap.
```

**Edit** (check workflow)
Old:
```go
3. Items in advice are optional. Fix them only if it is one small patch.
```
New:
```go
3. Items in advice are optional. Fix them only if it is one small patch.
   Film items (environment_monotony, framing_monotony, gesture_monotony, cast_unused) are about the whole film. Fix them with patch_scene on the scenes the fix text names. A patch replaces a list that you send (actors, beats), so send the whole list.
```

**Edit** (profile name)
Old:
```go
	name := "storymaker/v2/" + phase + "/" + variant
```
New:
```go
	name := "storymaker/v3/" + phase + "/" + variant
```

**Edit** `internal/server/storymaker_agent_profile_test.go`: replace both `storymaker/v2/` strings by `storymaker/v3/` (two single-line edits).
Old:
```go
"storymaker/v2/"+phase+"/normal"
```
New:
```go
"storymaker/v3/"+phase+"/normal"
```
Old:
```go
"storymaker/v2/"+phase+"/compact"
```
New:
```go
"storymaker/v3/"+phase+"/compact"
```
Append to the same test file:
```go

// The prompt names only entry styles the validator accepts (pop, fade, slide, none) and mentions the direction.
func TestStorymakerProfilesNameOnlyValidEntriesAndTheDirection(t *testing.T) {
	names := agent.StorymakerNames{Poses: []string{"stand"}, Gestures: []string{"point"}, Expressions: []string{"neutral"}}
	for _, phase := range []string{storymaker.PhaseOutline, storymaker.PhaseCast, storymaker.PhaseFill, storymaker.PhaseCheck, storymaker.PhaseRevise} {
		for _, compact := range []bool{false, true} {
			profile, err := storymakerAgentProfile(phase, compact, false, names)
			if err != nil {
				t.Fatal(err)
			}
			system := profile.SystemPrompt()
			for _, banned := range []string{"slide-up", "draw, stamp", "stageable environments", "maintain visual continuity"} {
				if strings.Contains(system, banned) {
					t.Errorf("%s compact=%v still says %q", phase, compact, banned)
				}
			}
			if phase == storymaker.PhaseFill && !strings.Contains(system, "direction") {
				t.Errorf("the fill prompt must mention the direction")
			}
			if phase == storymaker.PhaseCheck && !strings.Contains(system, "cast_unused") {
				t.Errorf("the check prompt must mention the film items")
			}
		}
	}
}
```
The test of the tool examples (`storymakerToolExamples`) keeps `"gesture":"point"` and entry `pop`, which are valid.
- [ ] **Step 3: Update the docs that name the version.** Run `grep -n "v2" internal/server/AGENTS.md | grep -i storymaker`. For each line that says the prompts are `storymaker/v2`, change it to `storymaker/v3`.
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run 'TestStorymakerProfiles' -count=1`. Expected: PASS. Then the scripted jobs, which build the profiles in both tool modes: `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run 'TestStorymakerAgentScripted' -count=1 -timeout 20m`. Expected: PASS.
- [ ] **Step 5: Commit:** `gofmt -w` both Go files; `git add internal/server/storymaker_agent_profile.go internal/server/storymaker_agent_profile_test.go internal/server/AGENTS.md`; message `feat(storymaker): agent prompts v3 (direction, film items, valid entry styles)`.

<!-- group A10 -->

### Task A10: Open checks on the job card, documents
**Batch:** A  **Depends on:** A9
**Files:**
Modify `ui/js/desktop/apps/storymaker-agent.js`
Modify `ui/lang/storymaker/*.json` (16 files, by a one-off script that stays in the scratch folder)
Modify `internal/storymaker/AGENTS.md`

The four craft codes can be left open at the end of a job. The card shows one shared sentence for them.

- [ ] **Step 1: Edit `ui/js/desktop/apps/storymaker-agent.js`.** The file has CRLF line endings, so a multi-line edit with the Edit tool would mix endings. Use this script from the scratch folder (`<scratch>/edit-agent-js.mjs`), run from the worktree root. It adds four entries to `CHECK_KEYS`, keeps the file's endings and refuses to run twice:

```js
import fs from 'node:fs';
const file = 'ui/js/desktop/apps/storymaker-agent.js';
const source = fs.readFileSync(file, 'utf8');
const eol = source.includes('\r\n') ? '\r\n' : '\n';
const anchor = "        over_budget: 'storymaker.agent.check_over_budget'" + eol + '    };';
if (!source.includes(anchor)) throw new Error('CHECK_KEYS anchor not found');
if (source.includes('environment_monotony')) throw new Error('already edited');
const added = [
  "        over_budget: 'storymaker.agent.check_over_budget',",
  "        environment_monotony: 'storymaker.agent.check_craft',",
  "        framing_monotony: 'storymaker.agent.check_craft',",
  "        gesture_monotony: 'storymaker.agent.check_craft',",
  "        cast_unused: 'storymaker.agent.check_craft'",
  '    };',
].join(eol);
fs.writeFileSync(file, source.replace(anchor, added));
console.log('CHECK_KEYS extended');
```
Expected output: `CHECK_KEYS extended`.
- [ ] **Step 2: Add the key `storymaker.agent.check_craft` to all 16 locale files.** Write `add-keys.mjs` into the scratch folder (never into the repo) and run it from the worktree root: `node <scratch>/add-keys.mjs`. It inserts one line after the line that holds `"storymaker.agent.check_over_budget"` in each `ui/lang/storymaker/<lang>.json`, keeps the file's line endings and refuses to run twice.

```js
import fs from 'node:fs';
const KEY = 'storymaker.agent.check_craft';
const AFTER = '"storymaker.agent.check_over_budget"';
const TEXT = {
  en: 'The film still looks alike in one respect: environments, framing, gestures or cast. Watch it once and adjust scenes if you like.',
  de: 'Der Film wirkt in einem Punkt noch gleichförmig: Umgebungen, Kamera, Gesten oder Besetzung. Sieh ihn dir an und passe Szenen an, wenn du magst.',
  cs: 'Film působí v jednom ohledu stále jednotvárně: prostředí, záběr, gesta nebo obsazení. Prohlédněte si ho a případně upravte scény.',
  da: 'Filmen virker stadig ensartet på ét punkt: miljøer, kamera, gestus eller rollebesætning. Se den igennem og justér scener, hvis du vil.',
  el: 'Η ταινία φαίνεται ακόμη μονότονη σε ένα σημείο: χώροι, κάδρο, χειρονομίες ή διανομή. Δείτε την μία φορά και προσαρμόστε σκηνές αν θέλετε.',
  es: 'La película aún resulta uniforme en un aspecto: entornos, encuadre, gestos o reparto. Míralo una vez y ajusta escenas si quieres.',
  fr: 'Le film paraît encore uniforme sur un point : décors, cadrage, gestes ou distribution. Regardez-le une fois et ajustez des scènes si vous le souhaitez.',
  hi: 'फ़िल्म अब भी एक बात में एक जैसी लगती है: माहौल, फ़्रेमिंग, हाव-भाव या किरदार। इसे एक बार देखें और चाहें तो दृश्य बदलें।',
  it: 'Il film appare ancora uniforme sotto un aspetto: ambienti, inquadratura, gesti o cast. Guardalo una volta e modifica le scene se vuoi.',
  ja: '動画はまだ、環境・カメラ・ジェスチャー・キャストのいずれかで単調に見えます。一度ご覧になり、必要ならシーンを調整してください。',
  nl: 'De film oogt nog op één punt eentonig: omgevingen, camera, gebaren of cast. Bekijk hem eenmaal en pas scènes aan als je wilt.',
  no: 'Filmen virker fortsatt ensartet på ett punkt: omgivelser, kamera, bevegelser eller rollebesetning. Se den én gang og juster scener om du vil.',
  pl: 'Film nadal wygląda jednolicie pod jednym względem: otoczenie, kadr, gesty lub obsada. Obejrzyj go raz i w razie potrzeby popraw sceny.',
  pt: 'O filme ainda parece uniforme num aspeto: cenários, enquadramento, gestos ou elenco. Veja-o uma vez e ajuste cenas se quiser.',
  sv: 'Filmen känns fortfarande likartad på en punkt: miljöer, kamera, gester eller rollbesättning. Titta på den en gång och justera scener om du vill.',
  zh: '影片在某一方面仍显单调：场景、镜头、手势或角色。请观看一次，如有需要再调整场景。',
};
for (const [lang, text] of Object.entries(TEXT)) {
  const file = `ui/lang/storymaker/${lang}.json`;
  let source = fs.readFileSync(file, 'utf8');
  if (source.includes(`"${KEY}"`)) throw new Error(`${file} already has ${KEY}`);
  const eol = source.includes('\r\n') ? '\r\n' : '\n';
  const lines = source.split(eol);
  const at = lines.findIndex((line) => line.includes(AFTER));
  if (at < 0) throw new Error(`${file} lacks ${AFTER}`);
  lines.splice(at + 1, 0, `  "${KEY}": ${JSON.stringify(text)},`);
  fs.writeFileSync(file, lines.join(eol));
  JSON.parse(fs.readFileSync(file, 'utf8')); // still valid JSON
}
console.log('added', KEY, 'to', Object.keys(TEXT).length, 'files');
```
Expected output: `added storymaker.agent.check_craft to 16 files`. The line after `check_over_budget` is `check_unknown`, so the new line (which ends with a comma) keeps the JSON valid. JSON.stringify keeps the non-ASCII text as written.
- [ ] **Step 3: Documents.** In `internal/storymaker/AGENTS.md`, after the line that begins `- \`DecideOutcome\` is the one rule for outcomes.` add:

Old (the whole line, as read on 2026-11-09):
```
- `DecideOutcome` is the one rule for outcomes. This slice never emits `ready`, and `open_checks` always holds `layout_unverified`. `over_target` and environment items never downgrade.
```
New:
```
- `DecideOutcome` is the one rule for outcomes. This slice never emits `ready`, and `open_checks` always holds `layout_unverified`. `over_target` and environment items never downgrade.
- `LintFilmCraft` (`studio_craft.go`) is the create job's film-level lint: `environment_monotony`, `framing_monotony`, `gesture_monotony`, `cast_unused`, for films of at least 4 scenes. `IssueCraft` codes are model-fixable while check rounds remain and never block: `issueNeverBlocks` keeps them out of `incomplete`, and what is left is an open check. `DirectionPlan` (`studio_direction.go`, curated guide in `data/env_direction.json`) gives each fill packet a `direction` and builds the worked example from it. A new environment in the catalog needs a line in the guide: `TestEnvironmentGuideCoversTheCatalog` fails without it. Show entries are `pop`, `fade`, `slide` and `none`; never name another in a prompt or a fix text.
```
If the line differs (another session changed it), insert the new bullet after the `DecideOutcome` bullet by its own text.
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated|TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerSourcesAreSafeAndSmall)' -count=1`. Expected: PASS. Then `npm run build:ui` and `npm run check:ui`: if `storymaker-agent.js` is part of a bundle, commit the rebuilt bundle with this task.
- [ ] **Step 5: Commit:** `git add ui/js/desktop/apps/storymaker-agent.js ui/lang/storymaker internal/storymaker/AGENTS.md` plus any rebuilt bundle that `git status` lists (stage it by name); message `feat(storymaker): job card line for open craft checks, document the craft lint`.

## Batch A end

Run the batch matrix of the index from the batch A worktree. Then the user decides the landing.
