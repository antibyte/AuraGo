# Part A

Tasks of batch A. Rules, commands and the order are in the plan index.

<!-- group A1 -->

### Task B0: Record the test baseline
**Batch:** A  **Depends on:** none
**Files:**
Create `docs/superpowers/plans/2026-11-06-storymaker-agent-core/evidence/baseline.md`

- [ ] **Step 1: Run: write the baseline from real test runs.** Run this from the root of the batch A worktree, before any other task, with no code change. The whole block takes many minutes, so run it as a background command. A failing test is expected here. Do not fix anything.

```bash
F=docs/superpowers/plans/2026-11-06-storymaker-agent-core/evidence/baseline.md; mkdir -p "$(dirname "$F")"; printf '# Test baseline at %s\n' "$(git rev-parse HEAD)" > "$F"
note() { out=$(awk '/^--- FAIL: /{print "- " $3} /build failed/{print "- [build failed] " $2} /^panic: /{print "- [panic] " $2}' | sort -u); printf '\n## %s\n%s\n' "$1" "${out:-none}" >> "$F"; }
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/agent 2>&1 | note './internal/agent'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/server -run Storymaker 2>&1 | note './internal/server -run Storymaker'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/storymaker/... 2>&1 | note './internal/storymaker/...'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/config 2>&1 | note './internal/config'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/prompts 2>&1 | note './internal/prompts'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/security 2>&1 | note './internal/security'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./internal/audit 2>&1 | note './internal/audit'
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 -timeout 30m ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' 2>&1 | note './ui -run Storymaker|TestDesktopStorymaker|TestConfigStorymaker'
```

Expected: the file starts with the commit hash and has eight `##` headings. Each heading lists failing top-level test names, or `none`. From now on, a batch passes when its failures are a subset of this file.
- [ ] **Step 2: Commit:** stage `docs/superpowers/plans/2026-11-06-storymaker-agent-core/evidence/baseline.md` through the temp index, because `docs/` is git-ignored; message `docs(storymaker): record the test baseline of the agent core`.

### Task A1: Agent config keys
**Batch:** A  **Depends on:** none
**Files:**
Modify `internal/config/storymaker.go`
Modify `internal/config/config.go`
Modify `internal/config/provider_references.go`
Modify `internal/config/decisions.go`
Modify `config_template.yaml`
Modify `internal/config/storymaker_test.go`
Test `internal/config/storymaker_agent_test.go`

- [ ] **Step 1: Impact:** `StorymakerConfig`, `NormalizeStorymakerConfig`, `Load`, `TestStorymakerDefaultsOn`. `Load` has a wide blast radius, so report its risk before you edit.
- [ ] **Step 2: Implement.** `config.go` and `config_template.yaml` are `eol=lf`. Keep them LF.

**Edit** `internal/config/storymaker.go`
Old:
```go
	Enabled   bool                      `yaml:"enabled" json:"enabled"`
```
New:
```go
	Enabled   bool                      `yaml:"enabled" json:"enabled"`
	Agent     StorymakerAgentConfig     `yaml:"agent" json:"agent"`
```

**Edit** `internal/config/storymaker.go`
Old:
```go
// StorymakerNarrationConfig caps the characters narrate jobs send to cloud TTS providers. Local
```
New:
```go
// StorymakerAgentConfig configures the agent that writes a story from a topic. Enabled defaults
// to true in the loader.
type StorymakerAgentConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Provider is a provider id for the job; empty means the main route.
	Provider string `yaml:"provider" json:"provider"`
	// Model overrides the provider's model; empty means the provider's model.
	Model string `yaml:"model" json:"model"`
	// JobTimeoutSeconds bounds one agent job.
	JobTimeoutSeconds int `yaml:"job_timeout_seconds" json:"job_timeout_seconds"`
	// MaxLengthSeconds is the longest film the agent is asked to write.
	MaxLengthSeconds int `yaml:"max_length_seconds" json:"max_length_seconds"`
	// MaxDailyJobs limits agent starts per local day.
	MaxDailyJobs int `yaml:"max_daily_jobs" json:"max_daily_jobs"`
}

// StorymakerNarrationConfig caps the characters narrate jobs send to cloud TTS providers. Local
```

**Edit** `internal/config/storymaker.go`
Old:
```go
	storymakerDefaultMaxJobChars        = 20000
)
```
New:
```go
	storymakerDefaultMaxJobChars        = 20000
)

const (
	storymakerDefaultAgentJobTimeoutSeconds = 1800
	storymakerDefaultAgentMaxLengthSeconds  = 180
	storymakerDefaultAgentMaxDailyJobs      = 10
)
```

**Edit** `internal/config/storymaker.go`
Old:
```go
// 100 to 10000000 narration characters per job. It does not change Enabled.
```
New:
```go
// 100 to 10000000 narration characters per job, and the agent limits of normalizeStorymakerAgent.
// It does not change Enabled.
```

**Edit** `internal/config/storymaker.go`
Old:
```go
		return fmt.Errorf("storymaker: narration.max_job_chars must be 100–10000000")
	}
	return nil
}
```
New:
```go
		return fmt.Errorf("storymaker: narration.max_job_chars must be 100–10000000")
	}
	return normalizeStorymakerAgent(&c.Agent)
}

// normalizeStorymakerAgent applies the agent defaults for unset (zero) values and rejects values
// outside their range: job timeout 120 to 7200 seconds, maximum length 15 to 3600 seconds, 1 to
// 1000 jobs per day. It does not change Enabled.
func normalizeStorymakerAgent(a *StorymakerAgentConfig) error {
	a.Provider = strings.TrimSpace(a.Provider)
	a.Model = strings.TrimSpace(a.Model)
	if a.JobTimeoutSeconds == 0 {
		a.JobTimeoutSeconds = storymakerDefaultAgentJobTimeoutSeconds
	}
	if a.MaxLengthSeconds == 0 {
		a.MaxLengthSeconds = storymakerDefaultAgentMaxLengthSeconds
	}
	if a.MaxDailyJobs == 0 {
		a.MaxDailyJobs = storymakerDefaultAgentMaxDailyJobs
	}
	if a.JobTimeoutSeconds < 120 || a.JobTimeoutSeconds > 7200 {
		return fmt.Errorf("storymaker: agent.job_timeout_seconds must be 120–7200")
	}
	if a.MaxLengthSeconds < 15 || a.MaxLengthSeconds > 3600 {
		return fmt.Errorf("storymaker: agent.max_length_seconds must be 15–3600")
	}
	if a.MaxDailyJobs < 1 || a.MaxDailyJobs > 1000 {
		return fmt.Errorf("storymaker: agent.max_daily_jobs must be 1–1000")
	}
	return nil
}
```

**Edit** `internal/config/config.go`
Old:
```go
	cfg.Storymaker.Enabled = true
```
New:
```go
	cfg.Storymaker.Enabled = true
	cfg.Storymaker.Agent.Enabled = true
```

**Edit** `config_template.yaml`
Old:
```yaml
storymaker:
    enabled: true
    render:
```
New:
```yaml
storymaker:
    enabled: true
    agent:
        enabled: true               # agent that writes a story from a topic, from the app or the chat tool
        provider: ""                # provider id for agent jobs; empty = main route
        model: ""                   # model override for agent jobs; empty = the provider's model
        job_timeout_seconds: 1800   # limit of one agent job (120-7200)
        max_length_seconds: 180     # longest film the agent writes (15-3600)
        max_daily_jobs: 10          # agent starts per local day (1-1000)
    render:
```

**Edit** `internal/config/provider_references.go`
Old:
```go
	add("cyd_creator.model", "cyd_creator", c.CYDCreator.Model)
```
New:
```go
	add("cyd_creator.model", "cyd_creator", c.CYDCreator.Model)
	add("storymaker.agent.provider", "storymaker_agent", c.Storymaker.Agent.Provider)
```

**Edit** `internal/config/decisions.go`
Old:
```go
		{"cyd_creator.model", cfg.CYDCreator.Model},
```
New:
```go
		{"cyd_creator.model", cfg.CYDCreator.Model},
		{"storymaker.agent.provider", cfg.Storymaker.Agent.Provider},
```

- [ ] **Step 3: Test.** `TestStorymakerDefaultsOn` compares the whole struct, so its expected value needs the new block.

**Edit** `internal/config/storymaker_test.go`
Old:
```go
		Enabled:   true,
		Render:    StorymakerRenderConfig{TimeoutSeconds: 3600, MaxDurationSeconds: 900},
```
New:
```go
		Enabled:   true,
		Agent:     StorymakerAgentConfig{Enabled: true, JobTimeoutSeconds: 1800, MaxLengthSeconds: 180, MaxDailyJobs: 10},
		Render:    StorymakerRenderConfig{TimeoutSeconds: 3600, MaxDurationSeconds: 900},
```

**Create** `internal/config/storymaker_agent_test.go`:
```go
package config

import "testing"

func TestStorymakerAgentConfigDefaultsAndRanges(t *testing.T) {
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	want := StorymakerAgentConfig{Enabled: true, JobTimeoutSeconds: 1800, MaxLengthSeconds: 180, MaxDailyJobs: 10}
	if cfg.Storymaker.Agent != want {
		t.Fatalf("defaults = %+v, want %+v", cfg.Storymaker.Agent, want)
	}

	cfg, err = loadStorymakerYAML(t, "storymaker:\n  agent:\n    enabled: false\n    provider: \" main \"\n    model: m1\n    max_length_seconds: 600\n")
	if err != nil {
		t.Fatal(err)
	}
	want = StorymakerAgentConfig{Provider: "main", Model: "m1", JobTimeoutSeconds: 1800, MaxLengthSeconds: 600, MaxDailyJobs: 10}
	if cfg.Storymaker.Agent != want {
		t.Fatalf("explicit values = %+v, want %+v", cfg.Storymaker.Agent, want)
	}

	for _, a := range []StorymakerAgentConfig{
		{JobTimeoutSeconds: 120}, {JobTimeoutSeconds: 7200}, {MaxLengthSeconds: 15}, {MaxLengthSeconds: 3600}, {MaxDailyJobs: 1}, {MaxDailyJobs: 1000},
	} {
		c := StorymakerConfig{Agent: a}
		if err := NormalizeStorymakerConfig(&c); err != nil {
			t.Errorf("%+v rejected: %v", a, err)
		}
		if c.Agent.Enabled {
			t.Errorf("%+v: normalize changed Enabled", a)
		}
	}
	for _, a := range []StorymakerAgentConfig{
		{JobTimeoutSeconds: 119}, {JobTimeoutSeconds: 7201}, {JobTimeoutSeconds: -1},
		{MaxLengthSeconds: 14}, {MaxLengthSeconds: 3601},
		{MaxDailyJobs: 1001}, {MaxDailyJobs: -1},
	} {
		c := StorymakerConfig{Agent: a}
		if NormalizeStorymakerConfig(&c) == nil {
			t.Errorf("%+v accepted", a)
		}
	}
	if _, err := loadStorymakerYAML(t, "storymaker:\n  agent:\n    max_daily_jobs: 1001\n"); err == nil {
		t.Error("Load accepted 1001 daily jobs")
	}
}
```

- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 ./internal/config -run Storymaker`. Expected: `ok`. This run includes `TestStorymakerTemplateMatchesLoadDefaults`, which proves the template block equals the loader defaults.
- [ ] **Step 5: Commit:** stage `internal/config/storymaker.go`, `internal/config/config.go`, `config_template.yaml`, `internal/config/storymaker_test.go`, `internal/config/storymaker_agent_test.go`; message `feat(config): add the storymaker.agent keys`.

### Task A2: Codes, DecodeScene, args repair
**Batch:** A  **Depends on:** none
**Files:**
Modify `internal/storymaker/model.go`
Modify `internal/storymaker/issues.go`
Modify `internal/storymaker/decode.go`
Create `internal/storymaker/studio_args.go`
Test `internal/storymaker/studio_args_test.go`

- [ ] **Step 1: Implement.**

**Edit** `internal/storymaker/model.go`
Old:
```go
const CodeGlyphMissing = "glyph_missing"
```
New:
```go
const CodeGlyphMissing = "glyph_missing"

// Issue codes of the agent job. Only the job's lint and check emit them, never Validate.
const (
	CodeSceneUnfilled = "scene_unfilled"
	CodeOverTarget    = "over_target"
	CodeOverBudget    = "over_budget"
	CodeCompileFailed = "compile_failed"
)
```

**Edit** `internal/storymaker/issues.go`
Old:
```go
// exampleJSON renders {"field":"value"} for the fix text of an issue.
```
New:
```go
// IssueModelFixable says whether the agent can fix an issue with its story tools: every error of
// Validate, scene_unfilled, and compile_failed that names a scene. Code scenes and render deadlines
// are outside its tools.
func IssueModelFixable(i Issue) bool {
	switch i.Code {
	case CodeSceneUnfilled:
		return true
	case CodeCompileFailed:
		return i.SceneID != ""
	case CodeCodeError, CodeDeadline:
		return false
	}
	return i.Severity == SeverityError
}

// IssueAdvisory says whether a lint code earns one optional fix loop. These codes never block the
// outcome and are not listed as open checks.
func IssueAdvisory(code string) bool {
	switch code {
	case CodeFirstFrameEmpty, CodeDeadStretch, CodeFadeBetweenWorlds, CodeCueLate:
		return true
	}
	return false
}

// exampleJSON renders {"field":"value"} for the fix text of an issue.
```

**Edit** `internal/storymaker/decode.go`
Old:
```go
func syntaxIssue(msg, fix string) Issue {
```
New:
```go
// DecodeScene parses one scene object strictly, with the same unknown-field findings as DecodeStory.
// Paths start at the scene, not at the story root. A missing id is no finding, because the caller
// binds it. A failure returns a *DecodeError and its issues.
func DecodeScene(data []byte) (Scene, []Issue, error) {
	if limit := DefaultLimits().MaxStoryBytes; len(data) > limit {
		return sceneFailure(Issue{
			Severity: SeverityError, Path: "", Code: CodeTooLarge,
			Message: fmt.Sprintf("The scene is %d bytes; the limit is %d bytes.", len(data), limit),
			Fix:     "Shorten the narration or use fewer beats.",
		})
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		return sceneFailure(syntaxIssue("The scene is empty.", "Send the scene as one JSON object."))
	}
	var tree any
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&tree); err != nil {
		return sceneFailure(sceneJSONIssue(data, err))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return sceneFailure(syntaxIssue("There is more data after the scene object.", "Send exactly one JSON object."))
	}
	root, ok := tree.(map[string]any)
	if !ok {
		return sceneFailure(syntaxIssue("The scene must be a JSON object.", `Start with {"environment":"env.office","beats":[...]}.`))
	}
	var hits []unknownHit
	collectUnknown(root, reflect.TypeOf(Scene{}), "", &hits)
	if len(hits) > 0 {
		order := keyOrder(data)
		sort.SliceStable(hits, func(i, j int) bool { return order[hits[i].path] < order[hits[j].path] })
		if len(hits) > maxDecodeIssues {
			hits = hits[:maxDecodeIssues]
		}
		issues := make([]Issue, 0, len(hits))
		for _, h := range hits {
			issues = append(issues, h.issue())
		}
		return sceneFailure(issues...)
	}
	var sc Scene
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&sc); err != nil {
		return sceneFailure(sceneJSONIssue(data, err))
	}
	return sc, nil, nil
}

func sceneFailure(issues ...Issue) (Scene, []Issue, error) {
	return Scene{}, issues, &DecodeError{Code: issues[0].Code, Message: issues[0].Message, Issues: issues}
}

// sceneStoryWords turns the story wording of jsonErrorIssue into scene wording.
var sceneStoryWords = strings.NewReplacer("the story", "the scene", "The story", "The scene", "complete story", "complete scene")

func sceneJSONIssue(data []byte, err error) Issue {
	is := jsonErrorIssue(data, err)
	is.Message = sceneStoryWords.Replace(is.Message)
	is.Fix = sceneStoryWords.Replace(is.Fix)
	return is
}

func syntaxIssue(msg, fix string) Issue {
```

**Create** `internal/storymaker/studio_args.go`:
```go
package storymaker

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

var (
	// storyArgFields are the arguments of storymaker_story itself. Other keys belong to data.
	storyArgFields = nameSet("scene_id", "after", "op", "expected_rev", "data")
	// storyArgDropped never reach a handler: the operation and tool name of the call, and ids the server binds.
	storyArgDropped = nameSet("operation", "action", "tool", "project_id", "job_id")
	// storySceneAliases are accepted for scene_id when their value is text.
	storySceneAliases = []string{"id", "scene", "sceneId", "sceneID"}
)

// studioArgString returns a text argument without surrounding space, or "" for any other value.
func studioArgString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

// repairStoryArgs brings the arguments of a storymaker_story call into their canonical form. It
// wraps flat fields into data (a value already in data wins), decodes data given as a JSON string,
// accepts the scene id aliases and turns the string forms of at, from, to and duration into values
// the story format reads. Anything it cannot repair stays, so that the handler or DecodeScene names
// the problem. Nested maps of args are changed in place.
func repairStoryArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	flat := map[string]any{}
	for key, value := range args {
		switch {
		case storyArgDropped[key]:
		case storyArgFields[key]:
			out[key] = value
		case containsName(storySceneAliases, key):
			if _, isText := value.(string); !isText {
				flat[key] = value
			}
		default:
			flat[key] = value
		}
	}
	if studioArgString(out, "scene_id") == "" {
		for _, alias := range storySceneAliases {
			if id := studioArgString(args, alias); id != "" {
				out["scene_id"] = id
				break
			}
		}
	}
	raw := out["data"]
	if text, isText := raw.(string); isText {
		var decoded any
		if json.Unmarshal([]byte(text), &decoded) == nil {
			raw = decoded
		}
	}
	data, isObject := raw.(map[string]any)
	if raw != nil && !isObject {
		out["data"] = raw
		return out
	}
	if data == nil && len(flat) == 0 {
		delete(out, "data")
		return out
	}
	if data == nil {
		data = map[string]any{}
	}
	for key, value := range flat {
		if _, taken := data[key]; !taken {
			data[key] = value
		}
	}
	for _, key := range []string{"scene_id", "id"} {
		if id, isText := data[key].(string); isText {
			if studioArgString(out, "scene_id") == "" && strings.TrimSpace(id) != "" {
				out["scene_id"] = strings.TrimSpace(id)
			}
			delete(data, key)
		}
	}
	repairSceneForms(data)
	out["data"] = data
	return out
}

// repairSceneForms canonicalises the string forms of one scene object.
func repairSceneForms(data map[string]any) {
	if text, isText := data["duration"].(string); isText {
		data["duration"] = repairDuration(text)
	}
	for _, list := range []string{"actors", "props"} {
		storyArgObjects(data[list], func(obj map[string]any) { repairPosition(obj, "at") })
	}
	storyArgObjects(data["beats"], func(obj map[string]any) {
		for _, key := range []string{"at", "from", "to"} {
			repairPosition(obj, key)
		}
	})
}

func storyArgObjects(list any, visit func(map[string]any)) {
	items, _ := list.([]any)
	for _, item := range items {
		if obj, isObject := item.(map[string]any); isObject {
			visit(obj)
		}
	}
}

// repairDuration turns "6" or "6s" into the number 6. "auto" and anything unreadable stay as they are.
func repairDuration(text string) any {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "auto" {
		return "auto"
	}
	if seconds, ok := parseStoryNumber(strings.TrimSuffix(t, "s")); ok {
		return seconds
	}
	return text
}

// repairPosition turns "x,y" into a point. A spot name stays.
func repairPosition(obj map[string]any, key string) {
	text, isText := obj[key].(string)
	if !isText {
		return
	}
	left, right, found := strings.Cut(text, ",")
	if !found {
		return
	}
	x, okX := parseStoryNumber(left)
	y, okY := parseStoryNumber(right)
	if okX && okY {
		obj[key] = map[string]any{"x": x, "y": y}
	}
}

func parseStoryNumber(text string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
```

- [ ] **Step 2: Test.**

**Create** `internal/storymaker/studio_args_test.go`:
```go
package storymaker

import (
	"encoding/json"
	"testing"
)

func TestStudioArgsRepairFlatAndStringForms(t *testing.T) {
	got := repairStoryArgs(map[string]any{
		"operation": "put_scene", "action": ToolNameForArgsTest, "project_id": "someone-elses",
		"id": "s2", "environment": "env.office", "duration": "6s",
		"actors": []any{map[string]any{"cast": "anna", "at": "30,88"}},
		"props":  []any{map[string]any{"item": "prop.lock", "at": "right"}},
		"beats":  []any{map[string]any{"do": "walk", "who": "anna", "to": "70, 88"}},
	})
	if got["scene_id"] != "s2" {
		t.Fatalf("scene_id = %v", got["scene_id"])
	}
	for _, key := range []string{"operation", "action", "project_id", "id", "environment"} {
		if _, present := got[key]; present {
			t.Errorf("%s stayed at the top level: %v", key, got)
		}
	}
	raw, err := json.Marshal(got["data"])
	if err != nil {
		t.Fatal(err)
	}
	sc, issues, err := DecodeScene(raw)
	if err != nil {
		t.Fatalf("DecodeScene: %v (%+v)", err, issues)
	}
	if sc.Environment != "env.office" || sc.Duration == nil || sc.Duration.Seconds != 6 {
		t.Errorf("flat fields: %+v", sc)
	}
	if a := sc.Actors[0].At; !a.Point || a.X != 30 || a.Y != 88 {
		t.Errorf("actor at = %+v", a)
	}
	if p := sc.Props[0].At; p.Point || p.Spot != "right" {
		t.Errorf("prop at = %+v", p)
	}
	if b := sc.Beats[0].To; !b.Point || b.X != 70 || b.Y != 88 {
		t.Errorf("beat to = %+v", b)
	}

	got = repairStoryArgs(map[string]any{"scene": "s3", "data": `{"duration":"auto","narration":"Hallo"}`})
	data, isObject := got["data"].(map[string]any)
	if got["scene_id"] != "s3" || !isObject || data["duration"] != "auto" || data["narration"] != "Hallo" {
		t.Errorf("JSON-string data: %v", got)
	}

	got = repairStoryArgs(map[string]any{"scene_id": "s4", "narration": "flat", "data": map[string]any{"narration": "kept", "id": "s9"}})
	data = got["data"].(map[string]any)
	if got["scene_id"] != "s4" || data["narration"] != "kept" || data["id"] != nil {
		t.Errorf("data wins over flat fields, and the bound id stays: %v", got)
	}

	got = repairStoryArgs(map[string]any{"id": "s5"})
	if _, present := got["data"]; present || got["scene_id"] != "s5" {
		t.Errorf("a get must stay without data: %v", got)
	}

	if got = repairStoryArgs(map[string]any{"data": "no json"}); got["data"] != "no json" {
		t.Errorf("unreadable data must reach the handler: %v", got)
	}
}

// ToolNameForArgsTest is the action field a text-mode call carries.
const ToolNameForArgsTest = "storymaker_story"
```

- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 ./internal/storymaker`. Expected: `ok`, or only failures listed in `evidence/baseline.md`.
- [ ] **Step 4: Commit:** stage `internal/storymaker/model.go`, `internal/storymaker/issues.go`, `internal/storymaker/decode.go`, `internal/storymaker/studio_args.go`, `internal/storymaker/studio_args_test.go`; message `feat(storymaker): add agent issue codes, DecodeScene and story argument repair`.

### Task A3: Studio session, phase table, read-only gate
**Batch:** A  **Depends on:** A2
**Files:**
Create `internal/storymaker/studio.go`
Test `internal/storymaker/studio_readonly_test.go`

- [ ] **Step 1: Implement.**

**Create** `internal/storymaker/studio.go`:
```go
package storymaker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Tools of an agent job.
const (
	ToolStory   = "storymaker_story"
	ToolCatalog = "storymaker_catalog"
	ToolCheck   = "storymaker_check"
)

// Phases of an agent job.
const (
	PhaseOutline = "outline"
	PhaseCast    = "cast"
	PhaseFill    = "fill"
	PhaseCheck   = "check"
	PhaseRevise  = "revise"
)

// Outcomes of an agent job.
const (
	OutcomeReady           = "ready"
	OutcomeReadyUnverified = "ready_unverified"
	OutcomeIncomplete      = "incomplete"
	OutcomeFailed          = "failed"
)

// Kinds of an agent job.
const (
	AgentKindCreate = "create"
	AgentKindRevise = "revise"
)

// Budgets of an agent job.
const (
	StudioSceneChargedLimit = 3
	StudioSceneWriteLimit   = 8
	StudioDryRunsPerScene   = 2
	StudioDryRunsPerJob     = 20
	StudioCheckRounds       = 6
)

// studioDefaultTargetSeconds is the film length when a job names none.
const studioDefaultTargetSeconds = 60

// studioPhaseTools lists the tools of each phase.
var studioPhaseTools = map[string][]string{
	PhaseOutline: {ToolStory},
	PhaseCast:    {ToolCatalog, ToolStory},
	PhaseFill:    {ToolCatalog, ToolStory},
	PhaseCheck:   {ToolCheck, ToolStory},
	PhaseRevise:  {ToolCatalog, ToolStory, ToolCheck},
}

// studioPhaseOperations lists the operations of each tool in each phase.
var studioPhaseOperations = map[string]map[string][]string{
	PhaseOutline: {
		ToolStory: {"outline", "get"},
	},
	PhaseCast: {
		ToolCatalog: {"search", "describe"},
		ToolStory:   {"set_meta", "get"},
	},
	PhaseFill: {
		ToolCatalog: {"search", "describe"},
		ToolStory:   {"put_scene", "patch_scene", "dry_run", "get"},
	},
	PhaseCheck: {
		ToolCheck: {"validate"},
		ToolStory: {"patch_scene", "get"},
	},
	PhaseRevise: {
		ToolCatalog: {"search", "describe"},
		ToolStory:   {"get", "put_scene", "patch_scene", "remove_scene", "move_scene", "set_meta", "dry_run"},
		ToolCheck:   {"validate"},
	},
}

// studioWriteOps change the story, so a read-only desktop refuses them.
var studioWriteOps = nameSet("outline", "set_meta", "put_scene", "patch_scene", "remove_scene", "move_scene")

// PhaseTools returns the tool names of a phase, or nil for an unknown phase.
func PhaseTools(phase string) []string {
	return append([]string(nil), studioPhaseTools[phase]...)
}

// PhaseOperations returns the operations of a tool in a phase, or nil when the phase does not offer the tool.
func PhaseOperations(phase, tool string) []string {
	return append([]string(nil), studioPhaseOperations[phase][tool]...)
}

// OperationOwners maps every operation to its tool. The operation sets of the tools are disjoint.
func OperationOwners() map[string]string {
	owners := map[string]string{}
	for _, tools := range studioPhaseOperations {
		for tool, operations := range tools {
			for _, operation := range operations {
				owners[operation] = tool
			}
		}
	}
	return owners
}

// StudioDeps are the services of a job's studio. Catalog, Voices, Compile and ReadOnly may be nil.
// MaxDurationSeconds is the longest film the export accepts; Check reports a longer film as
// over_export_limit. Zero means no limit.
type StudioDeps struct {
	Projects           *ProjectStore
	Catalog            *Catalog
	Limits             Limits
	Voices             func(ctx context.Context) []VoiceProvider
	Compile            func(ctx context.Context, projectID string) (CompileSummary, error)
	ReadOnly           func() bool
	MaxDurationSeconds int
}

// StudioJob is what the server binds to a job. The model never names any of it.
type StudioJob struct {
	JobID, ProjectID, Kind string // Kind is AgentKindCreate or AgentKindRevise
	Language, Audience     string
	TargetSeconds          int
	VoiceAvailable         bool
}

// StudioCall is one tool call of the model.
type StudioCall struct {
	Tool, Operation string
	Args            map[string]any
}

// StudioResult is the answer to a call. JSON goes to the model, the other fields go to the event log.
type StudioResult struct {
	JSON                       []byte
	Status, Operation, SceneID string // for events: closed values only
}

// CompileSummary is the result of a compile check in the browser.
type CompileSummary struct {
	Available  bool
	Reason     string
	DurationMS int64
	Scenes     []CompileScene
	Issues     []Issue
}

// CompileScene is the measured timing of one scene.
type CompileScene struct {
	ID      string
	StartMS int64
	EndMS   int64
	CuesMS  []CompileCue
}

// CompileCue is the measured time of one narration marker.
type CompileCue struct {
	Marker int
	AtMS   int64
}

// Studio is the session of one agent job. Identity is bound at construction, so a tool call can
// only reach this job's project. The story file is the checkpoint: session state is lost with the
// job, and a later revise job finishes what is left.
type Studio struct {
	deps StudioDeps
	job  StudioJob

	// mu serialises Call and PhaseDone. Operation handlers run with mu held, so they must not call
	// a locking method of Studio.
	mu      sync.Mutex
	phase   string
	sceneID string

	outlineSaved    bool
	outlineRefusals int
	template        string
	castSaved       bool

	// Scene budget, keyed by scene id.
	drafts         map[string]map[string]any
	charged        map[string]int
	writes         map[string]int
	lastIssueCount map[string]int
	dryRunsScene   map[string]int
	dryRunsJob     int

	// Check state.
	checkRounds    int
	checkClean     bool
	compileETag    string
	compileSummary CompileSummary
}

// NewStudio returns the session of one job.
func NewStudio(deps StudioDeps, job StudioJob) (*Studio, error) {
	if deps.Projects == nil {
		return nil, fmt.Errorf("%w: a studio needs a project store", ErrInvalidArgument)
	}
	if !ValidProjectID(job.ProjectID) {
		return nil, fmt.Errorf("%w: project %q", ErrInvalidID, clip(job.ProjectID, 40))
	}
	if job.Kind != AgentKindCreate && job.Kind != AgentKindRevise {
		return nil, fmt.Errorf("%w: job kind %q", ErrInvalidArgument, clip(job.Kind, 40))
	}
	if deps.Limits == (Limits{}) {
		deps.Limits = DefaultLimits()
	}
	if job.TargetSeconds <= 0 {
		job.TargetSeconds = studioDefaultTargetSeconds
	}
	return &Studio{
		deps:           deps,
		job:            job,
		drafts:         map[string]map[string]any{},
		charged:        map[string]int{},
		writes:         map[string]int{},
		lastIssueCount: map[string]int{},
		dryRunsScene:   map[string]int{},
	}, nil
}

// SetPhase names the phase of the next loop and, for fill, its one scene.
func (s *Studio) SetPhase(phase, sceneID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
	s.sceneID = sceneID
}

// studioOp handles one operation. It runs with s.mu held, after the phase and read-only gates.
// Args of storymaker_story are already repaired.
type studioOp func(ctx context.Context, s *Studio, args map[string]any) StudioResult

// studioOps is the routing table of Call, keyed by "<tool>/<operation>". Files fill it from init.
var studioOps = map[string]studioOp{}

// registerStudioOp adds a handler to the routing table. It is only called from init functions.
func registerStudioOp(tool, operation string, op studioOp) {
	studioOps[tool+"/"+operation] = op
}

// Call runs one tool call. Every failure comes back as an answer the model can act on.
func (s *Studio) Call(ctx context.Context, call StudioCall) StudioResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	tool := strings.TrimSpace(call.Tool)
	operation := strings.ToLower(strings.TrimSpace(call.Operation))
	if operation == "" {
		operation = strings.ToLower(studioArgString(call.Args, "operation"))
	}
	if operation == "" && tool == ToolCheck {
		operation = "validate"
	}
	res := s.route(ctx, tool, operation, call.Args)
	if _, known := OperationOwners()[operation]; known {
		res.Operation = operation
	}
	return res
}

func (s *Studio) route(ctx context.Context, tool, operation string, args map[string]any) StudioResult {
	allowed := PhaseOperations(s.phase, tool)
	if len(allowed) == 0 {
		return studioRefused("operation_not_in_phase",
			fmt.Sprintf("The tool %q is not available in the %s phase.", clip(tool, 40), s.phase),
			"Use one of these tools: "+listOf(PhaseTools(s.phase))+".")
	}
	if !containsName(allowed, operation) {
		return studioRefused("operation_not_in_phase",
			fmt.Sprintf("The operation %q of %s is not available in the %s phase.", clip(operation, 40), tool, s.phase),
			"Use one of these operations: "+listOf(allowed)+".")
	}
	if studioWriteOps[operation] && s.deps.ReadOnly != nil && s.deps.ReadOnly() {
		return studioRefused("desktop_readonly", "The desktop is read-only, so the story cannot change.",
			"Stop and answer with one short sentence.")
	}
	op := studioOps[tool+"/"+operation]
	if op == nil {
		return studioRefused("operation_not_in_phase", fmt.Sprintf("The operation %q is not available.", operation),
			"Use one of these operations: "+listOf(allowed)+".")
	}
	if tool == ToolStory {
		args = repairStoryArgs(args)
	}
	return op(ctx, s, args)
}

// PhaseDone is the server-owned end of the current loop. A revise loop ends on a text reply instead.
func (s *Studio) PhaseDone() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.phase {
	case PhaseOutline:
		return s.outlineSaved
	case PhaseCast:
		return s.castSaved
	case PhaseFill:
		st, _, err := s.loadStory(context.Background())
		if err != nil {
			return false
		}
		sc := studioFindScene(st, s.sceneID)
		return sc != nil && studioSceneFilled(sc)
	case PhaseCheck:
		return s.checkClean || s.checkRounds >= StudioCheckRounds
	}
	return false
}

// loadStory reads the story of this job with the defaults applied.
func (s *Studio) loadStory(ctx context.Context) (*Story, string, error) {
	data, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return nil, "", err
	}
	st, _, err := DecodeStory(data, s.deps.Limits)
	if err != nil {
		return nil, "", err
	}
	ApplyDefaults(&st)
	return &st, etag, nil
}

// saveStory writes the story against the etag that was read and returns the new etag.
func (s *Studio) saveStory(ctx context.Context, st *Story, etag string) (string, error) {
	ApplyDefaults(st)
	data, err := CanonicalJSON(*st)
	if err != nil {
		return "", err
	}
	return s.deps.Projects.SaveStory(ctx, s.job.ProjectID, data, etag)
}

// studioFindScene returns the scene with an id, or nil.
func studioFindScene(st *Story, id string) *Scene {
	for i := range st.Scenes {
		if st.Scenes[i].ID == id {
			return &st.Scenes[i]
		}
	}
	return nil
}

// studioSceneFilled says whether a scene has more than its outline. ApplyDefaults writes an
// environment into every scene, so the environment alone does not count.
func studioSceneFilled(sc *Scene) bool {
	return sc.Kind == "code" || len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0
}

// studioRev is the revision the model sees: the first 12 hex digits of the etag.
func studioRev(etag string) string {
	rev := strings.Trim(etag, `"`)
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return rev
}

// studioAnswer encodes an answer. encoding/json escapes angle brackets, so story text cannot close
// the fence around tool output.
func studioAnswer(status string, fields map[string]any) StudioResult {
	out := map[string]any{"status": status}
	for key, value := range fields {
		out[key] = value
	}
	data, err := json.Marshal(out)
	if err != nil {
		return StudioResult{JSON: []byte(`{"status":"error","message":"The answer could not be encoded."}`), Status: "error"}
	}
	return StudioResult{JSON: data, Status: status}
}

// studioOK is a successful answer.
func studioOK(fields map[string]any) StudioResult {
	return studioAnswer("ok", fields)
}

// studioRefused is a refusal with its code and the next thing to do.
func studioRefused(code, message, nextAction string) StudioResult {
	return studioRefusedWith(code, message, nextAction, nil)
}

// studioRefusedWith is a refusal with more fields, such as issues or word numbers.
func studioRefusedWith(code, message, nextAction string, extra map[string]any) StudioResult {
	fields := map[string]any{"code": code, "message": message, "next_action": nextAction}
	for key, value := range extra {
		fields[key] = value
	}
	return studioAnswer("refused", fields)
}

// studioFailed is the answer to a storage failure. It names no path.
func studioFailed() StudioResult {
	return studioAnswer("error", map[string]any{
		"message":     "The story could not be read or saved.",
		"next_action": "Stop and answer with one short sentence.",
	})
}
```

- [ ] **Step 2: Test.**

**Create** `internal/storymaker/studio_readonly_test.go`:
```go
package storymaker

import (
	"context"
	"encoding/json"
	"testing"
)

// fixtureStudio returns a studio on a new project of 60 seconds with the mini catalog.
func fixtureStudio(t *testing.T, readOnly func() bool) (*Studio, *ProjectStore, string) {
	t.Helper()
	ps, _, _ := newTestStore(t)
	proj, _, _, err := ps.Create(context.Background(), "Was ist eine Firewall?", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewStudio(StudioDeps{Projects: ps, Catalog: loadMini(t), ReadOnly: readOnly}, StudioJob{
		JobID: "job-1", ProjectID: proj.ID, Kind: AgentKindCreate, Language: "de", Audience: "Einsteiger", TargetSeconds: 60, VoiceAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, ps, proj.ID
}

// answerOf decodes the model-facing answer and checks that it agrees with the result status.
func answerOf(t *testing.T, res StudioResult) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(res.JSON, &out); err != nil {
		t.Fatalf("answer %q: %v", res.JSON, err)
	}
	if out["status"] != res.Status {
		t.Fatalf("status field %v, result status %q", out["status"], res.Status)
	}
	return out
}

func TestStudioRefusesWritesWhenReadOnly(t *testing.T) {
	ctx := context.Background()
	s, ps, id := fixtureStudio(t, func() bool { return true })
	before, _, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ phase, operation string }{
		{PhaseOutline, "outline"},
		{PhaseCast, "set_meta"},
		{PhaseFill, "put_scene"},
		{PhaseFill, "patch_scene"},
		{PhaseCheck, "patch_scene"},
		{PhaseRevise, "remove_scene"},
		{PhaseRevise, "move_scene"},
	} {
		s.SetPhase(tc.phase, "s1")
		res := s.Call(ctx, StudioCall{Tool: ToolStory, Operation: tc.operation, Args: map[string]any{"scene_id": "s1", "data": map[string]any{}}})
		answer := answerOf(t, res)
		if res.Status != "refused" || answer["code"] != "desktop_readonly" || res.Operation != tc.operation {
			t.Errorf("%s/%s: %s", tc.phase, tc.operation, res.JSON)
		}
	}
	after, _, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("a refused write changed the story:\n%s", after)
	}
	s.SetPhase(PhaseOutline, "")
	if answer := answerOf(t, s.Call(ctx, StudioCall{Tool: ToolStory, Operation: "get"})); answer["code"] == "desktop_readonly" {
		t.Errorf("reading must stay possible: %v", answer)
	}
}
```

- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 ./internal/storymaker`. Expected: `ok`, or only failures listed in `evidence/baseline.md`.
- [ ] **Step 4: Commit:** stage `internal/storymaker/studio.go`, `internal/storymaker/studio_readonly_test.go`; message `feat(storymaker): add the agent studio session, phase table and read-only gate`.

### Task A4: get, outline, set_meta, SetAgentMeta
**Batch:** A  **Depends on:** A3
**Files:**
Create `internal/storymaker/studio_story.go`
Modify `internal/storymaker/project.go`
Test `internal/storymaker/studio_story_outline_test.go`

- [ ] **Step 1: Implement.**

**Edit** `internal/storymaker/project.go`
Old:
```go
// SaveCodeScene writes the source of a code scene and bundles it. ifMatch follows
```
New:
```go
// SetAgentMeta records the agent's length target, audience and template in project.json. These
// three live in the project record, not in story.json. It retries a write that lost a race.
func (p *ProjectStore) SetAgentMeta(ctx context.Context, id string, targetSeconds int, audience, template string) error {
	if targetSeconds < 0 {
		return fmt.Errorf("%w: the target length must not be negative", ErrInvalidArgument)
	}
	for attempt := 0; attempt < 3; attempt++ {
		pr, etag, err := p.readProject(ctx, id)
		if err != nil {
			return err
		}
		pr.TargetSeconds, pr.Audience, pr.Template = targetSeconds, audience, template
		b, err := encodeProject(pr)
		if err != nil {
			return err
		}
		_, err = p.st.WriteFile(ctx, p.file(id, "project.json"), b, etag)
		if !errors.Is(err, ErrConflict) {
			return err
		}
	}
	return ErrConflict
}

// SaveCodeScene writes the source of a code scene and bundles it. ifMatch follows
```

**Create** `internal/storymaker/studio_story.go`:
```go
package storymaker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
)

func init() {
	registerStudioOp(ToolStory, "get", studioGet)
	registerStudioOp(ToolStory, "outline", studioOutline)
	registerStudioOp(ToolStory, "set_meta", studioSetMeta)
}

// Word budgets of an outline.
const (
	outlineWordsPerSecond = 2.3
	outlineWordsPerScene  = 19
	outlineMinScenes      = 2
	outlineMaxScenes      = 24
	outlineOverLimit      = 1.15 // share of the target words above which an outline is too long
	outlineRefusalLimit   = 2    // refusals of a long outline; the next attempt is accepted
)

const (
	outlineExample = `{"template":"concept_explainer","target_seconds":60,"scenes":[{"id":"s1","narration":"Ein Passwort allein schützt dein Konto nicht."},{"id":"s2","narration":"Zwei Faktoren machen den Login sicherer."}]}`
	metaExample    = `{"title":"Sicher im Netz","cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}]}`
)

var studioTemplates = []string{"product_explainer", "how_to", "concept_explainer", "dialogue_story", "data_story", "onboarding"}

func outlineTargetWords(target int) int {
	return int(math.Round(float64(target) * outlineWordsPerSecond))
}

func outlineSuggestedScenes(target int) int {
	n := int(math.Round(float64(target) * outlineWordsPerSecond / outlineWordsPerScene))
	return max(outlineMinScenes, min(n, outlineMaxScenes))
}

func outlineWordBudget(template string, target, scenes int) int {
	if template == "product_explainer" || template == "concept_explainer" || scenes <= 0 {
		return outlineWordsPerScene
	}
	return int(math.Round(float64(target) * outlineWordsPerSecond / float64(scenes)))
}

// hasLetterOrDigit says whether text has a letter or a digit. Placeholder text such as "..." has neither.
func hasLetterOrDigit(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
}

// studioDataJSON returns the data object of a call as JSON, or the refusal for a missing one.
func studioDataJSON(args map[string]any, example string) ([]byte, *StudioResult) {
	data, isObject := args["data"].(map[string]any)
	if !isObject {
		res := studioRefused("invalid_data", "The data argument must be a JSON object.", "Send data as an object. Example: "+example)
		return nil, &res
	}
	raw, err := json.Marshal(data)
	if err != nil {
		res := studioFailed()
		return nil, &res
	}
	return raw, nil
}

// studioRevMismatch refuses a write whose expected_rev is stale. The answer carries the current
// rev and, when the call names a scene, the current scene.
func studioRevMismatch(args map[string]any, st *Story, etag string) (StudioResult, bool) {
	want := studioArgString(args, "expected_rev")
	rev := studioRev(etag)
	if want == "" || studioRev(want) == rev {
		return StudioResult{}, false
	}
	extra := map[string]any{"rev": rev}
	if sc := studioFindScene(st, studioArgString(args, "scene_id")); sc != nil {
		extra["scene_id"] = sc.ID
		extra["scene"] = sc
	}
	return studioRefusedWith("rev_mismatch", "The story has changed since that rev.", "Use the rev of this answer and repeat the change.", extra), true
}

// studioSaveFailed is the answer to a failed save.
func studioSaveFailed(err error) StudioResult {
	if errors.Is(err, ErrConflict) {
		return studioRefused("rev_mismatch", "The story changed while this call ran.", "Call get, then repeat the change.")
	}
	return studioFailed()
}

// studioIssuesAt keeps the issues whose path starts with one of the prefixes.
func studioIssuesAt(issues []Issue, prefixes ...string) []Issue {
	out := []Issue{}
	for _, is := range issues {
		for _, prefix := range prefixes {
			if strings.HasPrefix(is.Path, prefix) {
				out = append(out, is)
				break
			}
		}
	}
	return out
}

// studioNonErrors keeps warnings and minor notes. The result is never nil, so it encodes as [].
func studioNonErrors(issues []Issue) []Issue {
	out := []Issue{}
	for _, is := range issues {
		if is.Severity != SeverityError {
			out = append(out, is)
		}
	}
	return out
}

func studioSceneIDs(st *Story) []string {
	ids := make([]string, len(st.Scenes))
	for i := range st.Scenes {
		ids[i] = st.Scenes[i].ID
	}
	return ids
}

// studioGet answers with one scene, or with a summary of the story.
func studioGet(ctx context.Context, s *Studio, args map[string]any) StudioResult {
	st, etag, err := s.loadStory(ctx)
	if err != nil {
		return studioFailed()
	}
	byScene := map[string][]Issue{}
	for _, is := range Validate(st, s.deps.Catalog, s.deps.Limits) {
		if is.Severity != SeverityMinor && is.SceneID != "" {
			byScene[is.SceneID] = append(byScene[is.SceneID], is)
		}
	}
	rev := studioRev(etag)
	if id := studioArgString(args, "scene_id"); id != "" {
		sc := studioFindScene(st, id)
		if sc == nil {
			return studioRefused("scene_not_found", fmt.Sprintf("The story has no scene %q.", clip(id, 40)),
				"Use one of these scene ids: "+listOf(studioSceneIDs(st))+".")
		}
		issues := append([]Issue{}, CapIssues(byScene[sc.ID], MaxModelIssues)...)
		res := studioOK(map[string]any{"rev": rev, "scene_id": sc.ID, "scene": sc, "issues": issues})
		res.SceneID = sc.ID
		return res
	}
	rows := make([]map[string]any, 0, len(st.Scenes))
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		codes := []string{}
		if !studioSceneFilled(sc) {
			codes = append(codes, CodeSceneUnfilled)
		}
		for _, is := range byScene[sc.ID] {
			if !containsName(codes, is.Code) {
				codes = append(codes, is.Code)
			}
		}
		rows = append(rows, map[string]any{"id": sc.ID, "environment": sc.Environment, "narration": clip(sc.Narration, 80), "beats": len(sc.Beats), "issues": codes})
	}
	meta := map[string]any{"title": st.Title, "aspect": st.Format.Aspect, "palette": st.Style.Palette, "voice": st.Voice}
	return studioOK(map[string]any{"rev": rev, "meta": meta, "cast": st.Cast, "scenes": rows})
}

// outlineInput ignores unknown fields on purpose: a stray title on an outline row loses nothing, and
// a weak model should not spend a round on it. target_seconds is not read, because the length of
// the job binds and the model cannot stretch the film.
type outlineInput struct {
	Template string         `json:"template"`
	Scenes   []outlineScene `json:"scenes"`
}

type outlineScene struct {
	ID        string `json:"id"`
	Narration string `json:"narration"`
}

type outlineRow struct {
	ID         string `json:"id"`
	Words      int    `json:"words"`
	WordBudget int    `json:"word_budget"`
}

// studioOutline replaces the scenes of a story that has no filled scene yet.
func studioOutline(ctx context.Context, s *Studio, args map[string]any) StudioResult {
	raw, refused := studioDataJSON(args, outlineExample)
	if refused != nil {
		return *refused
	}
	var in outlineInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return studioRefused("invalid_data", "The outline data could not be read: "+clip(err.Error(), 200), "Send data in this form: "+outlineExample)
	}
	template := strings.TrimSpace(in.Template)
	if !containsName(studioTemplates, template) {
		return studioRefused("invalid_data", fmt.Sprintf("The template %q is not known.", clip(template, 40)), "Use one of: "+listOf(studioTemplates)+".")
	}
	if n := len(in.Scenes); n < outlineMinScenes || n > outlineMaxScenes {
		return studioRefused("invalid_data", fmt.Sprintf("The outline has %d scenes; it needs %d to %d.", n, outlineMinScenes, outlineMaxScenes),
			"Send data in this form: "+outlineExample)
	}
	scenes := make([]Scene, 0, len(in.Scenes))
	seen := map[string]bool{}
	for _, row := range in.Scenes {
		id := strings.TrimSpace(row.ID)
		switch {
		case !validSlug(id):
			return studioRefused("invalid_data", fmt.Sprintf("The scene id %q is not a slug.", clip(id, 40)), "Use short ids such as s1 and s2.")
		case seen[id]:
			return studioRefused("invalid_data", fmt.Sprintf("The scene id %q is used twice.", id), "Give each scene its own id.")
		case !hasLetterOrDigit(StripMarkers(row.Narration)):
			return studioRefused("invalid_data", fmt.Sprintf("The scene %q has no narration in letters or digits.", id), "Write the narration of every scene in real words, not dots or placeholders.")
		}
		seen[id] = true
		scenes = append(scenes, Scene{ID: id, Narration: strings.TrimSpace(row.Narration)})
	}

	st, etag, err := s.loadStory(ctx)
	if err != nil {
		return studioFailed()
	}
	if res, mismatch := studioRevMismatch(args, st, etag); mismatch {
		return res
	}
	for i := range st.Scenes {
		if studioSceneFilled(&st.Scenes[i]) {
			return studioRefused("outline_locked", "The story already has filled scenes, so the outline cannot change.",
				"Use put_scene or patch_scene for one scene.")
		}
	}

	target := s.job.TargetSeconds
	budget := outlineWordBudget(template, target, len(scenes))
	rows := make([]outlineRow, len(scenes))
	total := 0
	for i := range scenes {
		words := WordCount(StripMarkers(scenes[i].Narration))
		rows[i] = outlineRow{ID: scenes[i].ID, Words: words, WordBudget: budget}
		total += words
	}
	targetWords := outlineTargetWords(target)
	answer := map[string]any{"scenes": rows, "total_words": total, "target_words": targetWords, "suggested_scenes": outlineSuggestedScenes(target)}
	over := float64(total) > float64(targetWords)*outlineOverLimit
	if over && s.outlineRefusals < outlineRefusalLimit {
		s.outlineRefusals++
		return studioRefusedWith(CodeOverBudget,
			fmt.Sprintf("The narration has %d words; the target is %d. The text is too long.", total, targetWords),
			"Shorten each scene to its word_budget and call outline again.", answer)
	}

	st.Scenes = scenes
	ApplyDefaults(st)
	found := studioIssuesAt(Validate(st, s.deps.Catalog, s.deps.Limits), "/scenes")
	if HasErrors(found) {
		return studioRefusedWith("invalid_data", "The outline has problems.", "Fix the listed issues and call outline again.",
			map[string]any{"issues": CapIssues(found, MaxModelIssues)})
	}
	newTag, err := s.saveStory(ctx, st, etag)
	if err != nil {
		return studioSaveFailed(err)
	}
	s.outlineSaved = true
	s.template = template
	// The story is the checkpoint and this is only a record, so like touch a failure is not reported.
	_ = s.deps.Projects.SetAgentMeta(ctx, s.job.ProjectID, target, s.job.Audience, template)

	warnings := studioNonErrors(found)
	if over {
		warnings = append(warnings, Issue{
			Severity: SeverityWarning, Path: "/scenes", Code: CodeOverTarget,
			Message: fmt.Sprintf("The narration has %d words; the target is %d.", total, targetWords),
			Fix:     "Send shorter narration with put_scene when you fill a scene.",
		})
	}
	if len(warnings) > 0 {
		answer["issues"] = CapIssues(warnings, MaxModelIssues)
	}
	answer["rev"] = studioRev(newTag)
	answer["next_action"] = "The outline is saved. This phase is done: stop."
	return studioOK(answer)
}

// metaInput is the data of set_meta. There is no music field: music stays the user's choice in this
// slice, so the strict decode refuses it and the model can say so.
type metaInput struct {
	Title *string       `json:"title"`
	Style *metaStyle    `json:"style"`
	Voice *Voice        `json:"voice"`
	Cast  *[]CastMember `json:"cast"`
}

type metaStyle struct {
	Palette  *string `json:"palette"`
	Captions *bool   `json:"captions"`
}

// studioSetMeta changes the fields the data names. cast is replaced whole.
func studioSetMeta(ctx context.Context, s *Studio, args map[string]any) StudioResult {
	raw, refused := studioDataJSON(args, metaExample)
	if refused != nil {
		return *refused
	}
	var in metaInput
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return studioRefused("invalid_data", "set_meta takes only title, style, voice and cast: "+clip(err.Error(), 200),
			"Send only these fields. Music is not available to you. Example: "+metaExample)
	}
	if in.Title == nil && in.Style == nil && in.Voice == nil && in.Cast == nil {
		return studioRefused("invalid_data", "set_meta needs at least one of title, style, voice and cast.", "Example: "+metaExample)
	}
	if in.Title != nil && !hasLetterOrDigit(*in.Title) {
		return studioRefused("invalid_data", "The title has no letters or digits.", "Write a short title in real words, not dots or placeholders.")
	}
	if s.phase == PhaseCast && (in.Cast == nil || len(*in.Cast) == 0) {
		return studioRefused("invalid_data", "This phase needs a cast with at least one member.", "Example: "+metaExample)
	}

	st, etag, err := s.loadStory(ctx)
	if err != nil {
		return studioFailed()
	}
	if res, mismatch := studioRevMismatch(args, st, etag); mismatch {
		return res
	}
	if in.Title != nil {
		st.Title = strings.TrimSpace(*in.Title)
	}
	if in.Style != nil {
		if in.Style.Palette != nil {
			st.Style.Palette = strings.TrimSpace(*in.Style.Palette)
		}
		if in.Style.Captions != nil {
			st.Style.Captions = *in.Style.Captions
		}
	}
	if in.Voice != nil {
		st.Voice = in.Voice
	}
	if in.Cast != nil {
		st.Cast = *in.Cast
	}
	ApplyDefaults(st)
	issues := Validate(st, s.deps.Catalog, s.deps.Limits)
	found := studioIssuesAt(issues, "/title", "/style", "/voice", "/cast")
	blocking := append([]Issue{}, found...)
	for _, is := range issues {
		if is.Code == CodeUnknownCast && is.Severity == SeverityError && strings.HasPrefix(is.Path, "/scenes") {
			blocking = append(blocking, is) // a scene still names a cast member this call removes
		}
	}
	if HasErrors(blocking) {
		return studioRefusedWith("invalid_data", "The meta data has problems.", "Fix the listed issues and call set_meta again.",
			map[string]any{"issues": CapIssues(blocking, MaxModelIssues)})
	}
	newTag, err := s.saveStory(ctx, st, etag)
	if err != nil {
		return studioSaveFailed(err)
	}
	answer := map[string]any{"rev": studioRev(newTag), "issues": CapIssues(studioNonErrors(found), MaxModelIssues)}
	if in.Cast != nil && len(*in.Cast) > 0 {
		s.castSaved = true
		if s.phase == PhaseCast {
			answer["next_action"] = "The cast is saved. This phase is done: stop."
		}
	}
	return studioOK(answer)
}
```

- [ ] **Step 2: Test.**

**Create** `internal/storymaker/studio_story_outline_test.go`:
```go
package storymaker

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
)

func callOutline(t *testing.T, s *Studio, template string, narrations ...string) (StudioResult, map[string]any) {
	t.Helper()
	scenes := make([]any, len(narrations))
	for i, narration := range narrations {
		scenes[i] = map[string]any{"id": fmt.Sprintf("s%d", i+1), "narration": narration}
	}
	res := s.Call(context.Background(), StudioCall{Tool: ToolStory, Operation: "outline", Args: map[string]any{
		"data": map[string]any{"template": template, "target_seconds": 60, "scenes": scenes},
	}})
	return res, answerOf(t, res)
}

func outlineRowAt(answer map[string]any, i int) map[string]any {
	return answer["scenes"].([]any)[i].(map[string]any)
}

func TestStudioOutlineBudgetsAndLock(t *testing.T) {
	ctx := context.Background()
	// 60 s at 2.3 words per second is 138 target words. 138 / 19 rounds to 7 scenes.
	wantWords := math.Round(60 * 2.3)
	wantScenes := math.Round(wantWords / 19)

	s, ps, id := fixtureStudio(t, nil)
	s.SetPhase(PhaseOutline, "")
	if s.PhaseDone() {
		t.Fatal("the phase is done before an outline")
	}
	// Placeholder narration is refused, and the phase stays open.
	if res, answer := callOutline(t, s, "concept_explainer", "...", "Vier fuenf"); res.Status != "refused" || answer["code"] != "invalid_data" || s.PhaseDone() {
		t.Fatalf("placeholder narration: %s", res.JSON)
	}
	res, answer := callOutline(t, s, "concept_explainer", "Eins zwei drei", "Vier fuenf")
	if res.Status != "ok" || answer["rev"] == "" || answer["target_words"] != wantWords || answer["suggested_scenes"] != wantScenes || answer["total_words"] != float64(5) {
		t.Fatalf("outline: %s", res.JSON)
	}
	first, second := outlineRowAt(answer, 0), outlineRowAt(answer, 1)
	if first["id"] != "s1" || first["words"] != float64(3) || first["word_budget"] != float64(19) || second["words"] != float64(2) {
		t.Fatalf("rows: %s", res.JSON)
	}
	if !s.PhaseDone() {
		t.Error("an accepted outline must end the phase")
	}
	proj, err := ps.Get(ctx, id)
	if err != nil || proj.TargetSeconds != 60 || proj.Audience != "Einsteiger" || proj.Template != "concept_explainer" {
		t.Errorf("project record: %+v (%v)", proj, err)
	}

	// Other templates divide the target words by the scenes: 138 / 2 is 69.
	res, answer = callOutline(t, s, "how_to", "Eins zwei drei", "Vier fuenf")
	if res.Status != "ok" || outlineRowAt(answer, 0)["word_budget"] != math.Round(wantWords/2) {
		t.Fatalf("how_to: %s", res.JSON)
	}

	data, etag, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	st := mustDecode(t, string(data))
	if len(st.Scenes) != 2 || st.Scenes[1].ID != "s2" || st.Scenes[1].Narration != "Vier fuenf" {
		t.Fatalf("saved scenes: %+v", st.Scenes)
	}
	st.Scenes[0].Beats = []Beat{{Do: "wait", Seconds: 1}}
	filled, err := CanonicalJSON(st)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, id, filled, etag); err != nil {
		t.Fatal(err)
	}
	res, answer = callOutline(t, s, "how_to", "Eins", "Zwei")
	if res.Status != "refused" || answer["code"] != "outline_locked" {
		t.Fatalf("a filled scene must lock the outline: %s", res.JSON)
	}

	// 138 * 1.15 is 158.7 words, so 170 words are too long: refused twice, accepted the third time.
	long, ps2, id2 := fixtureStudio(t, nil)
	long.SetPhase(PhaseOutline, "")
	words := strings.TrimSpace(strings.Repeat("wort ", 85))
	for attempt := 1; attempt <= 2; attempt++ {
		res, answer = callOutline(t, long, "concept_explainer", words, words)
		if res.Status != "refused" || answer["code"] != CodeOverBudget || answer["total_words"] != float64(170) || outlineRowAt(answer, 0)["words"] != float64(85) {
			t.Fatalf("attempt %d: %s", attempt, res.JSON)
		}
		if long.PhaseDone() {
			t.Fatalf("attempt %d: a refused outline must not end the phase", attempt)
		}
	}
	data, _, err = ps2.LoadStory(ctx, id2)
	if err != nil {
		t.Fatal(err)
	}
	if got := mustDecode(t, string(data)); len(got.Scenes) != 1 {
		t.Fatalf("a refused outline changed the story: %+v", got.Scenes)
	}
	res, answer = callOutline(t, long, "concept_explainer", words, words)
	if res.Status != "ok" || !strings.Contains(string(res.JSON), `"code":"`+CodeOverTarget+`"`) {
		t.Fatalf("third attempt: %s", res.JSON)
	}
	if !long.PhaseDone() {
		t.Error("the accepted third attempt must end the phase")
	}
}
```

- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 ./internal/storymaker`. Expected: `ok`, or only failures listed in `evidence/baseline.md`.
- [ ] **Step 4: Commit:** stage `internal/storymaker/studio_story.go`, `internal/storymaker/project.go`, `internal/storymaker/studio_story_outline_test.go`; message `feat(storymaker): add the studio get, outline and set_meta operations`.

<!-- group A2 -->

### Task A5: `put_scene`, `patch_scene`, `remove_scene`, `move_scene`, `dry_run`, drafts, charging
**Batch:** A  **Depends on:** A4
**Files:**
Create `internal/storymaker/studio_story_scenes.go`
Test `internal/storymaker/studio_story_scenes_test.go`

- [ ] **Step 1: Implement the scene writes.** This is a new file because an Append cannot add imports to `studio_story.go`.

**Create** `internal/storymaker/studio_story_scenes.go`:
```go
package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// sceneWriteDryRunID names the scene of a dry run without a scene_id. It is never saved.
const sceneWriteDryRunID = "dry-run"

// Each scene operation is one handler in the routing table of studio.go. Handlers run with s.mu held.
func init() {
	registerStudioOp(ToolStory, "put_scene", studioSceneOp("put_scene"))
	registerStudioOp(ToolStory, "patch_scene", studioSceneOp("patch_scene"))
	registerStudioOp(ToolStory, "dry_run", studioSceneOp("dry_run"))
	registerStudioOp(ToolStory, "remove_scene", studioSceneOp("remove_scene"))
	registerStudioOp(ToolStory, "move_scene", studioSceneOp("move_scene"))
}

// studioSceneOp returns the handler of one scene operation.
func studioSceneOp(op string) studioOp {
	return func(ctx context.Context, s *Studio, args map[string]any) StudioResult {
		return s.runSceneOp(ctx, op, args)
	}
}

// runSceneOp runs put_scene, patch_scene, remove_scene, move_scene or dry_run. Studio.Call has
// checked the phase and the read-only gate, and has repaired the arguments.
func (s *Studio) runSceneOp(ctx context.Context, op string, args map[string]any) StudioResult {
	sceneID, refused := s.sceneWriteTarget(op, args)
	if refused != nil {
		return *refused
	}
	switch op {
	case "put_scene", "patch_scene":
		return s.writeScene(ctx, op, sceneID, args)
	case "dry_run":
		return s.dryRunScene(ctx, sceneID, args)
	case "remove_scene":
		return s.removeScene(ctx, sceneID, args)
	case "move_scene":
		return s.moveScene(ctx, sceneID, args)
	}
	return sceneWriteRefused(op, "", "invalid_data", "This operation does not write a scene.", "Use put_scene or patch_scene.", nil)
}

// DropDrafts discards every draft. Saved scenes stay.
func (s *Studio) DropDrafts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.drafts)
}

// draftIDs returns the scene ids with a draft, sorted. The caller holds s.mu.
func (s *Studio) draftIDs() []string {
	return slices.Sorted(maps.Keys(s.drafts))
}

func (s *Studio) sceneWriteTarget(op string, args map[string]any) (string, *StudioResult) {
	id := sceneWriteArg(args, "scene_id")
	if s.phase == PhaseFill {
		if id == "" {
			id = s.sceneID
		}
		if id != s.sceneID {
			r := sceneWriteRefused(op, "", "scene_not_found", fmt.Sprintf("This phase fills only scene %s.", s.sceneID),
				"Use scene_id "+s.sceneID+".", nil)
			return "", &r
		}
	}
	if id == "" {
		if op == "dry_run" {
			return "", nil
		}
		r := sceneWriteRefused(op, "", "invalid_data", "scene_id is required.", "Send the scene_id of the scene.", nil)
		return "", &r
	}
	if !validSlug(id) {
		r := sceneWriteRefused(op, "", "invalid_data", "scene_id is not a valid scene id.", "Use a short id such as s1.", nil)
		return "", &r
	}
	return id, nil
}

func (s *Studio) writeScene(ctx context.Context, op, sceneID string, args map[string]any) StudioResult {
	data, ok := args["data"].(map[string]any)
	if !ok {
		return sceneWriteRefused(op, sceneID, "invalid_data", "data must be an object with the scene fields.",
			`Send data as an object, for example {"environment":"env.office"}.`, nil)
	}
	story, etag, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	if s.writes[sceneID] >= StudioSceneWriteLimit {
		return sceneWriteRefused(op, sceneID, "corrections_exhausted", fmt.Sprintf("Scene %s used all %d writes.", sceneID, StudioSceneWriteLimit),
			"Stop working on this scene. Its last saved version stays.", nil)
	}
	fields, refused := s.sceneWriteFields(op, op, sceneID, data, &story)
	if refused != nil {
		return *refused
	}
	ev := s.evaluateSceneWrite(story, sceneID, fields)
	if ev.errors == 0 {
		s.writes[sceneID]++
		newETag, err := s.saveSceneStory(ctx, &ev.story, etag)
		if err != nil {
			return sceneWriteSaveRefused(op, sceneID, err)
		}
		s.lastIssueCount[sceneID] = 0
		delete(s.drafts, sceneID)
		empty := !studioSceneFilled(&ev.scene)
		next := ""
		if s.phase == PhaseFill {
			next = "This scene is saved. Stop."
			if empty {
				next = "The scene is saved but nothing is on stage. Add actors and beats with patch_scene."
			}
		}
		return sceneWriteOK(op, sceneID, newETag, ev.issues, next)
	}
	free := s.writes[sceneID] == 0 || ev.errors < s.lastIssueCount[sceneID]
	if !free && s.charged[sceneID] >= StudioSceneChargedLimit {
		return sceneWriteRefused(op, sceneID, "corrections_exhausted", fmt.Sprintf("Scene %s used all %d corrections.", sceneID, StudioSceneChargedLimit),
			"Send a patch_scene that removes at least one listed issue, or stop. The last saved version stays.", nil)
	}
	if !free {
		s.charged[sceneID]++
	}
	s.writes[sceneID]++
	s.lastIssueCount[sceneID] = ev.errors
	s.drafts[sceneID] = fields
	left := StudioSceneChargedLimit - s.charged[sceneID]
	next := "Fix the listed issues with patch_scene and the same scene_id."
	if left == 0 {
		next = "Fix the listed issues with patch_scene and the same scene_id. Only a patch that removes an issue is still accepted."
	}
	return sceneWriteAnswer(op, "draft", sceneID, map[string]any{
		"draft": true, "scene_id": sceneID, "issues": sceneWriteIssues(ev.issues, false), "corrections_left": left, "next_action": next,
	})
}

func (s *Studio) dryRunScene(ctx context.Context, sceneID string, args map[string]any) StudioResult {
	const op = "dry_run"
	data, ok := args["data"].(map[string]any)
	if !ok {
		return sceneWriteRefused(op, sceneID, "invalid_data", "data must be an object with the scene fields.",
			`Send data as an object, for example {"environment":"env.office"}.`, nil)
	}
	mode := sceneWriteArg(args, "op")
	if mode == "" {
		mode = "put_scene"
	}
	if mode != "put_scene" && mode != "patch_scene" {
		return sceneWriteRefused(op, sceneID, "invalid_data", "op must be put_scene or patch_scene.", "Send op put_scene or patch_scene.", nil)
	}
	target := sceneID
	if target == "" {
		target = sceneWriteDryRunID
	}
	if s.dryRunsJob >= StudioDryRunsPerJob || s.dryRunsScene[target] >= StudioDryRunsPerScene {
		return sceneWriteRefused(op, sceneID, "dry_runs_exhausted", "No dry run is left.", "Call put_scene or patch_scene directly.", nil)
	}
	story, _, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	fields, refused := s.sceneWriteFields(op, mode, target, data, &story)
	if refused != nil {
		return *refused
	}
	s.dryRunsJob++
	s.dryRunsScene[target]++
	ev := s.evaluateSceneWrite(story, target, fields)
	body := map[string]any{
		"issues":           sceneWriteIssues(ev.issues, false),
		"defaults_applied": append([]DefaultApplied{}, ev.defaults...),
	}
	if sceneID != "" {
		body["scene_id"] = sceneID
	}
	return sceneWriteAnswer(op, "ok", sceneID, body)
}

func (s *Studio) removeScene(ctx context.Context, sceneID string, args map[string]any) StudioResult {
	const op = "remove_scene"
	story, etag, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	index := sceneWriteIndex(&story, sceneID)
	_, hadDraft := s.drafts[sceneID]
	if index >= 0 && len(story.Scenes) == 1 {
		return sceneWriteRefused(op, sceneID, "invalid_data", "A story needs at least one scene.", "Change this scene instead of removing it.", nil)
	}
	delete(s.drafts, sceneID)
	delete(s.writes, sceneID)
	delete(s.charged, sceneID)
	delete(s.lastIssueCount, sceneID)
	delete(s.dryRunsScene, sceneID)
	if index < 0 {
		if hadDraft {
			return sceneWriteOK(op, sceneID, etag, nil, "")
		}
		return sceneWriteNotFound(op, sceneID, &story)
	}
	story.Scenes = slices.Delete(story.Scenes, index, index+1)
	newETag, err := s.saveSceneStory(ctx, &story, etag)
	if err != nil {
		return sceneWriteSaveRefused(op, sceneID, err)
	}
	return sceneWriteOK(op, sceneID, newETag, nil, "")
}

func (s *Studio) moveScene(ctx context.Context, sceneID string, args map[string]any) StudioResult {
	const op = "move_scene"
	story, etag, refused := s.loadSceneStory(ctx, op, sceneID, args)
	if refused != nil {
		return *refused
	}
	from := sceneWriteIndex(&story, sceneID)
	if from < 0 {
		return sceneWriteNotFound(op, sceneID, &story)
	}
	after := sceneWriteArg(args, "after")
	if after == sceneID {
		return sceneWriteOK(op, sceneID, etag, nil, "")
	}
	if after != "" && sceneWriteIndex(&story, after) < 0 {
		return sceneWriteNotFound(op, after, &story)
	}
	scene := story.Scenes[from]
	rest := slices.Delete(story.Scenes, from, from+1)
	to := 0
	if after != "" {
		to = slices.IndexFunc(rest, func(sc Scene) bool { return sc.ID == after }) + 1
	}
	story.Scenes = slices.Insert(rest, to, scene)
	newETag, err := s.saveSceneStory(ctx, &story, etag)
	if err != nil {
		return sceneWriteSaveRefused(op, sceneID, err)
	}
	return sceneWriteOK(op, sceneID, newETag, nil, "")
}

// loadSceneStory reads the current story with defaults applied and checks expected_rev.
func (s *Studio) loadSceneStory(ctx context.Context, op, sceneID string, args map[string]any) (Story, string, *StudioResult) {
	unreadable := func() (Story, string, *StudioResult) {
		r := sceneWriteRefused(op, sceneID, "invalid_data", "The story could not be read.", "Stop. The server reports this to the user.", nil)
		return Story{}, "", &r
	}
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return unreadable()
	}
	story, _, err := DecodeStory(raw, s.deps.Limits)
	if err != nil {
		return unreadable()
	}
	ApplyDefaults(&story)
	if res, mismatch := studioRevMismatch(args, &story, etag); mismatch {
		res.Operation, res.SceneID = op, sceneID
		return Story{}, "", &res
	}
	return story, etag, nil
}

// sceneWriteFields builds the JSON fields of the candidate scene. put_scene replaces the scene
// but keeps narration, kind and silent when data leaves them out. patch_scene merges into the
// draft, else into the saved scene: top-level fields replace and null removes.
func (s *Studio) sceneWriteFields(answerOp, mode, sceneID string, data map[string]any, story *Story) (map[string]any, *StudioResult) {
	index := sceneWriteIndex(story, sceneID)
	var saved map[string]any
	if index >= 0 {
		var err error
		saved, err = sceneWriteFieldsOf(story.Scenes[index])
		if err != nil {
			r := sceneWriteRefused(answerOp, sceneID, "invalid_data", "The saved scene could not be read.", "Send the whole scene with put_scene.", nil)
			return nil, &r
		}
	}
	var fields map[string]any
	if mode == "put_scene" {
		fields = maps.Clone(data)
		for _, key := range []string{"narration", "kind", "silent"} {
			if v, present := fields[key]; present && v != nil {
				continue
			}
			if kept, has := saved[key]; has {
				fields[key] = kept
			} else {
				delete(fields, key)
			}
		}
	} else {
		switch draft, hasDraft := s.drafts[sceneID]; {
		case hasDraft:
			fields = maps.Clone(draft)
		case index >= 0:
			fields = saved
		default:
			r := sceneWriteNotFound(answerOp, sceneID, story)
			return nil, &r
		}
		for key, value := range data {
			if value == nil {
				delete(fields, key)
			} else {
				fields[key] = value
			}
		}
	}
	fields["id"] = sceneID
	return fields, nil
}

// sceneWriteEval is the result of checking a candidate scene inside its story.
type sceneWriteEval struct {
	story    Story
	scene    Scene
	issues   []Issue
	defaults []DefaultApplied
	errors   int
}

// evaluateSceneWrite decodes the fields and validates the whole story around them. Only findings
// of this scene count, so a finding elsewhere never blocks this write. It owns story.Scenes.
func (s *Studio) evaluateSceneWrite(story Story, sceneID string, fields map[string]any) sceneWriteEval {
	var ev sceneWriteEval
	raw, err := json.Marshal(fields)
	if err != nil {
		ev.issues = []Issue{{Severity: SeverityError, SceneID: sceneID, Code: CodeInvalidJSON, Message: "The scene could not be encoded."}}
		ev.errors = 1
		return ev
	}
	scene, decodeIssues, err := DecodeScene(raw)
	if err != nil {
		for _, is := range decodeIssues {
			is.SceneID = sceneID
			ev.issues = append(ev.issues, is)
			if is.Severity == SeverityError {
				ev.errors++
			}
		}
		if ev.errors == 0 {
			ev.issues = append(ev.issues, Issue{Severity: SeverityError, SceneID: sceneID, Code: CodeInvalidJSON, Message: "The scene could not be read.",
				Fix: `Send the scene fields of the tool schema, for example {"environment":"env.office"}.`})
			ev.errors = 1
		}
		return ev
	}
	index := sceneWriteIndex(&story, sceneID)
	if index < 0 {
		index = len(story.Scenes)
		story.Scenes = append(story.Scenes, scene)
	} else {
		story.Scenes[index] = scene
	}
	prefix := jp("scenes", index) + "/"
	for _, d := range ApplyDefaults(&story) {
		if strings.HasPrefix(d.Path, prefix) {
			ev.defaults = append(ev.defaults, d)
		}
	}
	for _, is := range Validate(&story, s.deps.Catalog, s.deps.Limits) {
		if is.SceneID != sceneID || is.Severity == SeverityMinor {
			continue
		}
		ev.issues = append(ev.issues, is)
		if is.Severity == SeverityError {
			ev.errors++
		}
	}
	ev.story = story
	ev.scene = story.Scenes[index]
	return ev
}

// saveSceneStory saves a changed story. Every saved write clears the check state of the model's
// last validate, so the check phase cannot end on a story it has not seen.
func (s *Studio) saveSceneStory(ctx context.Context, story *Story, etag string) (string, error) {
	s.checkClean = false
	data, err := CanonicalJSON(*story)
	if err != nil {
		return "", err
	}
	return s.deps.Projects.SaveStory(ctx, s.job.ProjectID, data, etag)
}

func sceneWriteArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func sceneWriteIndex(story *Story, sceneID string) int {
	return slices.IndexFunc(story.Scenes, func(sc Scene) bool { return sc.ID == sceneID })
}

func sceneWriteFieldsOf(scene Scene) (map[string]any, error) {
	raw, err := json.Marshal(scene)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func sceneWriteIssues(issues []Issue, onlyWarnings bool) []Issue {
	out := []Issue{}
	for _, is := range issues {
		if !onlyWarnings || is.Severity == SeverityWarning {
			out = append(out, is)
		}
	}
	return CapIssues(out, MaxModelIssues)
}

func sceneWriteAnswer(op, status, sceneID string, body map[string]any) StudioResult {
	body["status"] = status
	data, err := json.Marshal(body)
	if err != nil {
		status = "refused"
		data = []byte(`{"status":"refused","code":"invalid_data","message":"The answer could not be encoded."}`)
	}
	return StudioResult{JSON: data, Status: status, Operation: op, SceneID: sceneID}
}

func sceneWriteOK(op, sceneID, etag string, issues []Issue, next string) StudioResult {
	body := map[string]any{"rev": studioRev(etag), "issues": sceneWriteIssues(issues, true)}
	if sceneID != "" {
		body["scene_id"] = sceneID
	}
	if next != "" {
		body["next_action"] = next
	}
	return sceneWriteAnswer(op, "ok", sceneID, body)
}

func sceneWriteRefused(op, sceneID, code, message, next string, extra map[string]any) StudioResult {
	body := map[string]any{"code": code, "message": message, "next_action": next}
	maps.Copy(body, extra)
	return sceneWriteAnswer(op, "refused", sceneID, body)
}

func sceneWriteNotFound(op, sceneID string, story *Story) StudioResult {
	ids := make([]string, 0, len(story.Scenes))
	for i := range story.Scenes {
		ids = append(ids, story.Scenes[i].ID)
	}
	return sceneWriteRefused(op, "", "scene_not_found", fmt.Sprintf("The scene %s does not exist.", clip(sceneID, 48)),
		"Use one of: "+strings.Join(ids, ", ")+".", nil)
}

func sceneWriteSaveRefused(op, sceneID string, err error) StudioResult {
	if errors.Is(err, ErrConflict) {
		return sceneWriteRefused(op, sceneID, "rev_mismatch", "The story changed while this call ran.", "Send the change again.", nil)
	}
	return sceneWriteRefused(op, sceneID, "invalid_data", "The story could not be saved.", "Stop. The server reports this to the user.", nil)
}
```

- [ ] **Step 2: Add the test.** It uses only the design's public API, plus `studioRev`.

**Create** `internal/storymaker/studio_story_scenes_test.go`:
```go
package storymaker

import (
	"context"
	"encoding/json"
	"testing"
)

// newStudioWriteRig makes a project with a cast and two outline scenes, s1 and s2.
func newStudioWriteRig(t *testing.T, compile func(context.Context, string) (CompileSummary, error)) (*Studio, *ProjectStore, string) {
	t.Helper()
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	proj, story, _, err := ps.Create(ctx, "Firewall", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	story.Cast = []CastMember{{ID: "anna", Name: "Anna", Character: "char.office-woman-1"}}
	story.Scenes = []Scene{
		{ID: "s1", Narration: "Eine Firewall schuetzt das Netz."},
		{ID: "s2", Narration: "Sie prueft jedes Paket."},
	}
	ApplyDefaults(&story)
	data, err := CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, proj.ID, data, ""); err != nil {
		t.Fatal(err)
	}
	if compile == nil {
		compile = func(context.Context, string) (CompileSummary, error) {
			return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
		}
	}
	studio, err := NewStudio(StudioDeps{
		Projects: ps,
		Catalog:  loadReal(t),
		Limits:   DefaultLimits(),
		Voices:   func(context.Context) []VoiceProvider { return nil },
		Compile:  compile,
		ReadOnly: func() bool { return false },
	}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: "create", Language: "de", Audience: "general", TargetSeconds: 60, VoiceAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	return studio, ps, proj.ID
}

func studioWriteCall(t *testing.T, studio *Studio, tool, op, args string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		t.Fatalf("args %s: %v", args, err)
	}
	res := studio.Call(context.Background(), StudioCall{Tool: tool, Operation: op, Args: parsed})
	var answer map[string]any
	if err := json.Unmarshal(res.JSON, &answer); err != nil {
		t.Fatalf("%s answer %s: %v", op, res.JSON, err)
	}
	return answer
}

func TestStudioDraftLifecycleAndCharging(t *testing.T) {
	ctx := context.Background()
	studio, ps, id := newStudioWriteRig(t, nil)
	_, before, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	limit := StudioSceneChargedLimit
	wantDraft := func(answer map[string]any, left int) {
		t.Helper()
		if answer["status"] != "draft" || answer["draft"] != true || answer["corrections_left"] != float64(left) {
			t.Fatalf("answer = %v, want a draft with %d corrections left", answer, left)
		}
	}
	patch := func(sceneID, data string) map[string]any {
		t.Helper()
		return studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"`+sceneID+`","data":`+data+`}`)
	}

	// Two errors: an unknown cast and an unknown asset. A first write is free.
	const bad = `"data":{"environment":"env.office","actors":[{"cast":"ghost"}],"beats":[{"do":"show","item":"prop.nope","at":"right","entry":"pop"}]}`
	studio.SetPhase(PhaseFill, "s1")
	wantDraft(studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1",`+bad+`}`), limit)
	// The same two errors are charged.
	wantDraft(patch("s1", `{"variant":"night"}`), limit-1)
	// One error fewer is free.
	wantDraft(patch("s1", `{"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]}`), limit-1)
	// The same one error is charged until the budget is gone.
	wantDraft(patch("s1", `{"variant":"day"}`), limit-2)
	wantDraft(patch("s1", `{"variant":"night"}`), limit-3)
	if answer := patch("s1", `{"variant":"day"}`); answer["code"] != "corrections_exhausted" {
		t.Fatalf("answer = %v, want corrections_exhausted", answer)
	}
	if _, now, _ := ps.LoadStory(ctx, id); now != before {
		t.Fatal("a draft reached story.json")
	}
	if studio.PhaseDone() {
		t.Fatal("the fill phase ended on a draft")
	}

	// A patch that fixes the last error is still accepted, merges into the draft and saves it.
	answer := patch("s1", `{"actors":[{"cast":"anna","at":"left"}]}`)
	data, etag, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if answer["status"] != "ok" || answer["rev"] != studioRev(etag) {
		t.Fatalf("answer = %v, want ok with the current rev", answer)
	}
	story, _, err := DecodeStory(data, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	s1 := story.Scenes[0]
	if s1.Actors[0].Cast != "anna" || s1.Beats[0].Item != "prop.lock" || s1.Narration != "Eine Firewall schuetzt das Netz." {
		t.Fatalf("saved scene = %+v", s1)
	}
	if !studio.PhaseDone() {
		t.Fatal("the fill phase did not end on the saved scene")
	}

	// A new scene starts free, and remove_scene discards its draft.
	studio.SetPhase(PhaseRevise, "")
	wantDraft(studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s3",`+bad+`}`), limit)
	if answer := studioWriteCall(t, studio, ToolStory, "remove_scene", `{"scene_id":"s3"}`); answer["status"] != "ok" {
		t.Fatalf("remove answer = %v", answer)
	}
	if answer := patch("s3", `{"variant":"day"}`); answer["code"] != "scene_not_found" {
		t.Fatalf("answer = %v, want scene_not_found after the draft was removed", answer)
	}

	// Dry runs save nothing and have their own cap.
	_, beforeDry, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	studio.SetPhase(PhaseFill, "s2")
	const dry = `{"op":"put_scene","scene_id":"s2","data":{"environment":"env.office"}}`
	for range StudioDryRunsPerScene {
		answer := studioWriteCall(t, studio, ToolStory, "dry_run", dry)
		applied, _ := answer["defaults_applied"].([]any)
		if answer["status"] != "ok" || len(applied) == 0 {
			t.Fatalf("dry run answer = %v", answer)
		}
	}
	if answer := studioWriteCall(t, studio, ToolStory, "dry_run", dry); answer["code"] != "dry_runs_exhausted" {
		t.Fatalf("answer = %v, want dry_runs_exhausted", answer)
	}
	if _, now, _ := ps.LoadStory(ctx, id); now != beforeDry {
		t.Fatal("a dry run reached story.json")
	}
}
```

- [ ] **Step 3: Run.** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestStudioDraftLifecycleAndCharging$' -count=1`. Expected: PASS.

- [ ] **Step 4: Commit.** Stage `internal/storymaker/studio_story_scenes.go` and `internal/storymaker/studio_story_scenes_test.go`. Message: `feat(storymaker): add scene writes, drafts and charging to the Studio`.

### Task A6: Catalog operations and voices
**Batch:** A  **Depends on:** A3
**Files:**
Create `internal/storymaker/studio_catalog.go`
Test `internal/storymaker/studio_catalog_test.go`

- [ ] **Step 1: Implement the catalog tool.**

**Create** `internal/storymaker/studio_catalog.go`:
```go
package storymaker

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

const (
	catalogVoiceKind      = "voice"
	catalogMissNextAction = "Use the nearest alternative, or a show beat with text."
	catalogKindList       = "character, prop, environment, pose, gesture, expression, transition, text_style, sfx or voice"
)

// catalogModelKinds are the kinds a miss may suggest when the call named no kind.
var catalogModelKinds = nameSet("character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx")

type catalogHit struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	Tags         []string        `json:"tags"`
	UsageExample json.RawMessage `json:"usage_example,omitempty"`
	Spots        []string        `json:"spots,omitempty"`
	Variants     []string        `json:"variants,omitempty"`
}

type catalogVoiceHit struct {
	Provider  string   `json:"provider"`
	Voice     string   `json:"voice"`
	Languages []string `json:"languages"`
	Gender    string   `json:"gender,omitempty"`
}

type catalogVoiceRow struct {
	hit  catalogVoiceHit
	rank int
	text string
}

func init() {
	registerStudioOp(ToolCatalog, "search", studioCatalogOp("search"))
	registerStudioOp(ToolCatalog, "describe", studioCatalogOp("describe"))
}

// studioCatalogOp returns the handler of one storymaker_catalog operation. Studio.Call has checked
// the phase and the read-only gate.
func studioCatalogOp(op string) studioOp {
	return func(ctx context.Context, s *Studio, args map[string]any) StudioResult {
		return s.runCatalog(ctx, op, args)
	}
}

// runCatalog answers a search or describe call.
func (s *Studio) runCatalog(ctx context.Context, op string, args map[string]any) StudioResult {
	query := catalogOpArg(args, "query")
	id := catalogOpArg(args, "id")
	rawKind := catalogOpArg(args, "kind")
	if strings.EqualFold(rawKind, catalogVoiceKind) {
		return s.catalogVoiceSearch(ctx, op, query, catalogOpLimit(args))
	}
	kind := NormalizeKind(rawKind)
	if rawKind != "" && kind == "" {
		return catalogOpRefused(op, "This kind does not exist.", "Use kind "+catalogKindList+".")
	}
	if op == "describe" {
		target := id
		if target == "" {
			target = query
		}
		return s.catalogDescribe(target, kind)
	}
	if query == "" {
		query = id
	}
	hits := s.deps.Catalog.Search(query, kind, catalogOpLimit(args))
	if len(hits) == 0 {
		return s.catalogMiss(op, query, kind)
	}
	results := make([]catalogHit, 0, len(hits))
	for _, e := range hits {
		results = append(results, s.catalogHitOf(e))
	}
	return catalogOpAnswer(op, "ok", map[string]any{"results": results})
}

func (s *Studio) catalogDescribe(target, kind string) StudioResult {
	const op = "describe"
	entry, found := s.deps.Catalog.Lookup(target)
	if !found && kind != "" {
		entry, found = s.deps.Catalog.Lookup(kindPrefixes[kind] + target)
	}
	if !found {
		return s.catalogMiss(op, target, kind)
	}
	return catalogOpAnswer(op, "ok", map[string]any{"entry": s.catalogHitOf(entry)})
}

func (s *Studio) catalogHitOf(e Entry) catalogHit {
	hit := catalogHit{ID: e.ID, Kind: e.Kind, Tags: e.Tags, UsageExample: e.UsageExample}
	if hit.Tags == nil {
		hit.Tags = []string{}
	}
	if e.Kind == "environment" {
		hit.Spots = s.deps.Catalog.SpotNames(e.ID)
		hit.Variants = s.deps.Catalog.Variants(e.ID)
	}
	return hit
}

func (s *Studio) catalogMiss(op, query, kind string) StudioResult {
	var pool []string
	if s.deps.Catalog != nil {
		for _, e := range s.deps.Catalog.Entries() {
			if e.Kind == kind || (kind == "" && catalogModelKinds[e.Kind]) {
				pool = append(pool, e.ID)
			}
		}
	}
	return catalogOpAnswer(op, "ok", map[string]any{
		"results": []catalogHit{}, "alternatives": closestNames(query, pool, 5), "next_action": catalogMissNextAction,
	})
}

// catalogVoiceSearch lists ready voices: the active provider first, inside that voices of the job
// language first. Query terms filter on provider, voice, name, gender and language.
func (s *Studio) catalogVoiceSearch(ctx context.Context, op, query string, limit int) StudioResult {
	var providers []VoiceProvider
	if s.deps.Voices != nil {
		providers = s.deps.Voices(ctx)
	}
	base := LanguageBase(s.job.Language)
	var rows []catalogVoiceRow
	add := func(p *VoiceProvider, voice, name, gender string, languages []string) {
		rank := 2
		if p.Active {
			rank = 0
		}
		if !catalogSpeaks(languages, base) {
			rank++
		}
		id := NormalizeProvider(p.ID)
		text := strings.ToLower(strings.Join(append([]string{id, voice, name, gender}, languages...), " "))
		rows = append(rows, catalogVoiceRow{
			hit:  catalogVoiceHit{Provider: id, Voice: voice, Languages: append([]string{}, languages...), Gender: gender},
			rank: rank, text: text,
		})
	}
	for i := range providers {
		p := &providers[i]
		if !p.Ready || (!p.FreeText && len(p.Voices) == 0) {
			continue
		}
		if len(p.Voices) == 0 {
			add(p, p.DefaultVoice, "", "", p.Languages)
			continue
		}
		for _, o := range p.Voices {
			if o.Installed != nil && !*o.Installed {
				continue
			}
			languages := p.Languages
			if o.Language != "" {
				languages = []string{o.Language}
			}
			add(p, o.ID, o.Name, o.Gender, languages)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].rank < rows[j].rank })
	terms := strings.Fields(strings.ToLower(query))
	results := []catalogVoiceHit{}
	for _, row := range rows {
		if len(results) == limit {
			break
		}
		matches := true
		for _, term := range terms {
			matches = matches && strings.Contains(row.text, term)
		}
		if matches {
			results = append(results, row.hit)
		}
	}
	body := map[string]any{"results": results}
	if len(results) == 0 {
		body["next_action"] = "No voice fits. Leave the voice as it is."
	}
	return catalogOpAnswer(op, "ok", body)
}

func catalogSpeaks(languages []string, base string) bool {
	if base == "" {
		return true
	}
	for _, language := range languages {
		if language == "*" || LanguageBase(language) == base {
			return true
		}
	}
	return false
}

func catalogOpArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func catalogOpLimit(args map[string]any) int {
	limit := 0
	switch v := args["limit"].(type) {
	case float64:
		limit = int(v)
	case int:
		limit = v
	}
	if limit <= 0 {
		return defaultSearchLimit
	}
	return min(limit, maxSearchLimit)
}

func catalogOpAnswer(op, status string, body map[string]any) StudioResult {
	body["status"] = status
	data, err := json.Marshal(body)
	if err != nil {
		status = "refused"
		data = []byte(`{"status":"refused","code":"invalid_data","message":"The answer could not be encoded."}`)
	}
	return StudioResult{JSON: data, Status: status, Operation: op}
}

func catalogOpRefused(op, message, next string) StudioResult {
	return catalogOpAnswer(op, "refused", map[string]any{"code": "invalid_data", "message": message, "next_action": next})
}
```

- [ ] **Step 2: Add the test.**

**Create** `internal/storymaker/studio_catalog_test.go`:
```go
package storymaker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestStudioCatalogSearchMissAndVoice(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	proj, _, _, err := ps.Create(ctx, "Katalog", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	notInstalled := false
	// The active provider comes last here, so the answer's order proves the sort.
	providers := []VoiceProvider{
		{ID: "elevenlabs", Ready: false, Languages: []string{"*"}, Voices: []VoiceOption{{ID: "offline"}}},
		{ID: "piper", Ready: true, Languages: []string{"de", "en"}, Voices: []VoiceOption{
			{ID: "de_DE-thorsten-high", Language: "de", Gender: "male"},
			{ID: "en_US-amy", Language: "en", Installed: &notInstalled},
		}},
		{ID: "openai", Active: true, Ready: true, FreeText: true, Languages: []string{"*"}, DefaultVoice: "alloy"},
	}
	studio, err := NewStudio(StudioDeps{
		Projects: ps,
		Catalog:  loadReal(t),
		Limits:   DefaultLimits(),
		Voices:   func(context.Context) []VoiceProvider { return providers },
		Compile: func(context.Context, string) (CompileSummary, error) {
			return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
		},
		ReadOnly: func() bool { return false },
	}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: "create", Language: "de", TargetSeconds: 60, VoiceAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	studio.SetPhase(PhaseCast, "")
	search := func(args map[string]any) map[string]any {
		t.Helper()
		res := studio.Call(ctx, StudioCall{Tool: ToolCatalog, Operation: "search", Args: args})
		var answer map[string]any
		if err := json.Unmarshal(res.JSON, &answer); err != nil {
			t.Fatalf("answer %s: %v", res.JSON, err)
		}
		return answer
	}

	found := search(map[string]any{"query": "lock", "kind": "prop"})
	results, _ := found["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("search answer = %v", found)
	}
	first := results[0].(map[string]any)
	example, _ := first["usage_example"].(map[string]any)
	if first["id"] != "prop.lock" || example["item"] != "prop.lock" {
		t.Fatalf("first result = %v", first)
	}

	miss := search(map[string]any{"query": "zebra", "kind": "prop"})
	empty, isList := miss["results"].([]any)
	alternatives, _ := miss["alternatives"].([]any)
	if !isList || len(empty) != 0 || len(alternatives) == 0 || miss["next_action"] != "Use the nearest alternative, or a show beat with text." {
		t.Fatalf("miss answer = %v", miss)
	}
	for _, alternative := range alternatives {
		if !strings.HasPrefix(alternative.(string), "prop.") {
			t.Fatalf("alternative %v is not a prop", alternative)
		}
	}

	voices := search(map[string]any{"kind": "voice"})
	rows, _ := voices["results"].([]any)
	if len(rows) != 2 {
		t.Fatalf("voice answer = %v, want the two ready voices", voices)
	}
	active, listed := rows[0].(map[string]any), rows[1].(map[string]any)
	if active["provider"] != "openai" || active["voice"] != "alloy" {
		t.Fatalf("first voice = %v, want the active provider", active)
	}
	languages, _ := listed["languages"].([]any)
	if listed["provider"] != "piper" || listed["voice"] != "de_DE-thorsten-high" || listed["gender"] != "male" || len(languages) != 1 || languages[0] != "de" {
		t.Fatalf("second voice = %v", listed)
	}
}
```

- [ ] **Step 3: Run.** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestStudioCatalogSearchMissAndVoice$' -count=1`. Expected: PASS.

- [ ] **Step 4: Commit.** Stage `internal/storymaker/studio_catalog.go` and `internal/storymaker/studio_catalog_test.go`. Message: `feat(storymaker): add Studio catalog search, describe and voices`.

### Task A7: Lint, check, compile cache
**Batch:** A  **Depends on:** A5
**Files:**
Create `internal/storymaker/studio_check.go`
Modify `internal/storymaker/model.go`
Test `internal/storymaker/studio_check_test.go`

- [ ] **Step 1: Impact.** Run the plan index's impact command for `Studio.Check`, `Studio.Progress` and `Studio.runCheck`. The check state (`checkRounds`, `checkClean`, `compileETag`, `compileSummary`) is already on `Studio` from A3, so this task adds no fields.

- [ ] **Step 2: Implement lint, check and progress.**

**Edit** `internal/storymaker/model.go`
Old:
```go
	CodeCompileFailed = "compile_failed"
)
```
New:
```go
	CodeCompileFailed = "compile_failed"
	CodeOverLimit     = "over_export_limit"
)
```

**Create** `internal/storymaker/studio_check.go`:
```go
package storymaker

import (
	"context"
	"fmt"
	"math"
	"slices"
)

const (
	checkWordsPerSecond        = 2.3
	checkSceneTailSeconds      = 0.4
	checkMinSceneSeconds       = 1.0
	checkHouseWords            = 19
	checkOverFactor            = 1.15
	checkMaxAdvice             = 3
	checkCodeCompileUnverified = "compile_unverified"
)

// StudioProgress counts the scenes of the saved story. ScenesTotal is 0 for a blank story, which
// means that no outline is saved. Rev is studioRev of ETag. Drafts lists the scenes with a draft.
type StudioProgress struct {
	ScenesTotal, ScenesDone int
	Unfilled, Drafts        []string
	Rev, ETag               string
}

// CheckReport is the result of Studio.Check and of the model's validate. Issues are model-fixable,
// Advice is optional, and Environment is not the model's to fix. Compile is "ok", "unavailable" or "failed".
type CheckReport struct {
	Issues, Advice, Environment []Issue
	Scenes                      []CheckScene
	TotalSeconds                float64
	TargetSeconds               int
	Compile                     string
	RoundsLeft                  int
}

// CheckScene is the timing of one scene in a check report.
type CheckScene struct {
	ID               string
	NarrationSeconds float64
	EndSeconds       float64
	CueTimes         []float64
}

func init() {
	registerStudioOp(ToolCheck, "validate", studioValidateOp)
}

type checkSceneAnswer struct {
	ID               string    `json:"id"`
	NarrationSeconds float64   `json:"narration_seconds"`
	EndSeconds       float64   `json:"end_seconds"`
	CueTimes         []float64 `json:"cue_times,omitempty"`
}

type checkEnvironmentAnswer struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	ModelFixable bool   `json:"model_fixable"`
}

// LintForJob adds what Validate accepts but a finished film must not have: scene_unfilled for a
// scene with nothing on stage, and over_budget for narration over the house budget of 19 words
// plus 15 percent.
func LintForJob(st *Story) []Issue {
	if st == nil {
		return nil
	}
	var out []Issue
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		if !studioSceneFilled(sc) {
			out = append(out, Issue{
				Severity: SeverityError, SceneID: sc.ID, Path: jp("scenes", i), Code: CodeSceneUnfilled,
				Message: fmt.Sprintf("Scene %s has nothing on stage.", sc.ID),
				Fix:     `Add actors and beats with patch_scene. Example: {"actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","text":"Kurz gesagt","at":"right","entry":"pop"}]}`,
			})
		}
		if sc.Kind == "code" || sc.Silent {
			continue
		}
		if words := WordCount(StripMarkers(sc.Narration)); float64(words) > checkHouseWords*checkOverFactor {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: sc.ID, Path: jp("scenes", i) + "/narration", Code: CodeOverBudget,
				Message: fmt.Sprintf("The narration of scene %s has %d words; the budget is about %d.", sc.ID, words, checkHouseWords),
				Fix:     "Shorten the narration or split the scene.",
			})
		}
	}
	return out
}

// Check is the server's own check, run between loops and before narration. It clears checkClean, so
// the check phase ends only on the model's own validate. It charges no check round.
func (s *Studio) Check(ctx context.Context) (CheckReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkClean = false
	report, _, err := s.runCheck(ctx)
	return report, err
}

// Progress counts the saved story. A blank story, as ProjectStore.Create writes it, reports zero
// scenes, so ScenesTotal 0 means that no outline is saved.
func (s *Studio) Progress(ctx context.Context) (StudioProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return StudioProgress{}, fmt.Errorf("storymaker: progress: %w", err)
	}
	story, _, err := DecodeStory(raw, s.deps.Limits)
	if err != nil {
		return StudioProgress{}, fmt.Errorf("storymaker: progress: %w", err)
	}
	out := StudioProgress{Drafts: s.draftIDs(), Rev: studioRev(etag), ETag: etag}
	if len(story.Scenes) == 1 && story.Scenes[0].Narration == "" && !studioSceneFilled(&story.Scenes[0]) {
		return out, nil
	}
	out.ScenesTotal = len(story.Scenes)
	for i := range story.Scenes {
		if !studioSceneFilled(&story.Scenes[i]) {
			out.Unfilled = append(out.Unfilled, story.Scenes[i].ID)
		} else {
			out.ScenesDone++
		}
	}
	return out, nil
}

// studioValidateOp runs storymaker_check. Studio.Call has checked the phase and the read-only gate.
// The handler runs with s.mu held, so it calls runCheck and not Check.
func studioValidateOp(ctx context.Context, s *Studio, _ map[string]any) StudioResult {
	const op = "validate"
	if s.checkRounds >= StudioCheckRounds {
		return sceneWriteRefused(op, "", "check_rounds_exhausted", fmt.Sprintf("All %d check rounds are used.", StudioCheckRounds),
			"Answer with one short sentence and stop.", nil)
	}
	s.checkRounds++

	report, etag, err := s.runCheck(ctx)
	if err != nil {
		return sceneWriteRefused(op, "", "invalid_data", "The story could not be read.", "Stop. The server reports this to the user.", nil)
	}
	clean := len(report.Issues) == 0
	s.checkClean = clean

	environment := make([]checkEnvironmentAnswer, 0, len(report.Environment))
	for _, is := range report.Environment {
		environment = append(environment, checkEnvironmentAnswer{Code: is.Code, Message: is.Message})
	}
	scenes := make([]checkSceneAnswer, 0, len(report.Scenes))
	for _, sc := range report.Scenes {
		scenes = append(scenes, checkSceneAnswer{ID: sc.ID, NarrationSeconds: sc.NarrationSeconds, EndSeconds: sc.EndSeconds, CueTimes: sc.CueTimes})
	}
	next := "Fix each issue with storymaker_story operation patch_scene, then call storymaker_check again."
	if clean {
		next = "Nothing is left to fix. Answer with one short sentence and stop."
	}
	return sceneWriteAnswer(op, "ok", "", map[string]any{
		"rev":            studioRev(etag),
		"clean":          clean,
		"issues":         append([]Issue{}, CapIssues(report.Issues, MaxModelIssues)...),
		"advice":         append([]Issue{}, CapIssues(report.Advice, checkMaxAdvice)...),
		"environment":    environment,
		"scenes":         scenes,
		"total_seconds":  report.TotalSeconds,
		"target_seconds": report.TargetSeconds,
		"compile":        report.Compile,
		"rounds_left":    report.RoundsLeft,
		"next_action":    next,
	})
}

func (s *Studio) runCheck(ctx context.Context) (CheckReport, string, error) {
	raw, etag, err := s.deps.Projects.LoadStory(ctx, s.job.ProjectID)
	if err != nil {
		return CheckReport{}, "", fmt.Errorf("storymaker: check: %w", err)
	}
	story, _, err := DecodeStory(raw, s.deps.Limits)
	if err != nil {
		return CheckReport{}, "", fmt.Errorf("storymaker: check: %w", err)
	}
	ApplyDefaults(&story)
	found := Validate(&story, s.deps.Catalog, s.deps.Limits)
	if status, statusErr := s.deps.Projects.CodeSceneStatus(ctx, s.job.ProjectID); statusErr == nil {
		found = append(found, CodeSceneIssues(&story, status)...)
	}
	found = append(found, LintForJob(&story)...)
	summary, state, environment := s.compileForCheck(ctx, etag)
	found = append(found, checkCompileIssues(summary.Issues)...)
	scenes, total := checkTiming(&story, summary)
	if over := s.checkOverTarget(total); over != nil && !slices.ContainsFunc(found, func(is Issue) bool { return is.Code == over.Code }) {
		found = append(found, *over)
	}

	report := CheckReport{
		Environment: environment, Scenes: scenes, TotalSeconds: total,
		TargetSeconds: s.job.TargetSeconds, Compile: state,
	}
	for _, is := range found {
		switch {
		case is.Code == CodeOverTarget, is.Code == CodeCompileFailed && is.SceneID == "", is.Severity == SeverityError && !IssueModelFixable(is):
			report.Environment = append(report.Environment, is)
		case IssueModelFixable(is):
			report.Issues = append(report.Issues, is)
		case IssueAdvisory(is.Code):
			report.Advice = append(report.Advice, is)
		}
	}
	report.RoundsLeft = max(0, StudioCheckRounds-s.checkRounds)
	return report, etag, nil
}

// checkCompileIssues keeps the compile findings the model may see. narration_missing is dropped:
// the job narrates after the check and reports missing narration as an open check.
func checkCompileIssues(list []Issue) []Issue {
	out := []Issue{}
	for _, is := range list {
		if is.Code != CodeNarrationMissing {
			out = append(out, is)
		}
	}
	return out
}

// compileForCheck returns the compile result of this story version. A result is kept per etag,
// and a compile error is not kept, so the next round tries again.
func (s *Studio) compileForCheck(ctx context.Context, etag string) (CompileSummary, string, []Issue) {
	summary, hit := s.compileSummary, s.compileETag == etag
	if !hit {
		if s.deps.Compile == nil {
			summary = CompileSummary{Reason: "no compile check is available"}
		} else {
			fresh, err := s.deps.Compile(ctx, s.job.ProjectID)
			if err != nil {
				return CompileSummary{}, "failed", []Issue{{
					Severity: SeverityError, Code: CodeCompileFailed,
					Message: "The film could not be compiled.",
					Fix:     "This is not a scene problem. The server reports it to the user.",
				}}
			}
			summary = fresh
			s.compileSummary, s.compileETag = fresh, etag
		}
	}
	if !summary.Available {
		message := "The film could not be compiled here, so its real length and motion were not checked."
		if summary.Reason != "" {
			message += " Reason: " + clip(summary.Reason, 120) + "."
		}
		return summary, "unavailable", []Issue{{Severity: SeverityWarning, Code: checkCodeCompileUnverified, Message: message}}
	}
	if slices.ContainsFunc(summary.Issues, func(is Issue) bool { return is.Code == CodeCompileFailed }) {
		return summary, "failed", nil
	}
	return summary, "ok", nil
}

// checkOverTarget reports a film over the export limit as a model-fixable error, so the check loop
// keeps the job until it is cut. A film only over its target is a warning, which is advice for the outcome.
func (s *Studio) checkOverTarget(total float64) *Issue {
	if limit := s.deps.MaxDurationSeconds; limit > 0 && total > float64(limit) {
		return &Issue{
			Severity: SeverityError, Path: "/scenes", Code: CodeOverLimit,
			Message: fmt.Sprintf("The film is about %.0f seconds; the export limit is %d seconds.", total, limit),
			Fix:     fmt.Sprintf("Cut about %.0f seconds: shorten narration or remove a scene.", total-float64(limit)),
		}
	}
	if target := float64(s.job.TargetSeconds); target > 0 && total > target*checkOverFactor {
		return &Issue{
			Severity: SeverityWarning, Path: "/scenes", Code: CodeOverTarget,
			Message: fmt.Sprintf("The film is about %.0f seconds; the target is %d seconds.", total, s.job.TargetSeconds),
			Fix:     "Shorten narration or remove a scene.",
		}
	}
	return nil
}

// checkTiming estimates 2.3 words per second and replaces the estimate with compiled times where
// the compile check ran.
func checkTiming(story *Story, summary CompileSummary) ([]CheckScene, float64) {
	compiled := map[string]CompileScene{}
	if summary.Available {
		for _, sc := range summary.Scenes {
			compiled[sc.ID] = sc
		}
	}
	clock := 0.0
	out := make([]CheckScene, 0, len(story.Scenes))
	for i := range story.Scenes {
		sc := &story.Scenes[i]
		speech := 0.0
		if !sc.Silent {
			speech = float64(WordCount(StripMarkers(sc.Narration))) / checkWordsPerSecond
		}
		length := max(checkMinSceneSeconds, speech+checkSceneTailSeconds)
		if d := sc.Duration; d != nil && !d.Auto && len(d.Invalid) == 0 && d.Seconds > 0 {
			length = d.Seconds
		}
		clock += length
		row := CheckScene{ID: sc.ID, NarrationSeconds: checkRound(speech), EndSeconds: checkRound(clock)}
		if c, ok := compiled[sc.ID]; ok {
			row.EndSeconds = checkRound(float64(c.EndMS) / 1000)
			for _, cue := range c.CuesMS {
				row.CueTimes = append(row.CueTimes, checkRound(float64(cue.AtMS)/1000))
			}
		}
		out = append(out, row)
	}
	total := clock
	if summary.Available && summary.DurationMS > 0 {
		total = float64(summary.DurationMS) / 1000
	}
	return out, checkRound(total)
}

func checkRound(seconds float64) float64 { return math.Round(seconds*100) / 100 }
```

- [ ] **Step 3: Add the test.**

**Create** `internal/storymaker/studio_check_test.go`:
```go
package storymaker

import (
	"context"
	"testing"
)

func TestStudioCheckFindsUnfilledAndEnvironmentNeverBlocks(t *testing.T) {
	ctx := context.Background()
	compiles := 0
	studio, _, _ := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		compiles++
		return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
	})

	studio.SetPhase(PhaseFill, "s1")
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","actors":[{"cast":"anna","at":"left","pose":"present"}]}}`)
	if put["status"] != "ok" {
		t.Fatalf("put_scene answer = %v", put)
	}

	// The outline scene s2 passes Validate, but the job lint finds it.
	report, err := studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != CodeSceneUnfilled || report.Issues[0].SceneID != "s2" {
		t.Fatalf("issues = %+v, want scene_unfilled on s2", report.Issues)
	}
	if report.Compile != "unavailable" || len(report.Environment) != 1 || report.Environment[0].Code != "compile_unverified" {
		t.Fatalf("compile = %q, environment = %+v", report.Compile, report.Environment)
	}
	if report.TotalSeconds <= 0 {
		t.Fatalf("total = %v, want the estimate", report.TotalSeconds)
	}
	progress, err := studio.Progress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ScenesTotal != 2 || progress.ScenesDone != 1 || len(progress.Unfilled) != 1 || progress.Unfilled[0] != "s2" {
		t.Fatalf("progress = %+v", progress)
	}

	// The same story version is not compiled again, and the open issue keeps the phase running.
	studio.SetPhase(PhaseCheck, "")
	open := studioWriteCall(t, studio, ToolCheck, "validate", `{}`)
	if open["clean"] != false || open["rounds_left"] != float64(StudioCheckRounds-1) {
		t.Fatalf("validate answer = %v", open)
	}
	if compiles != 1 {
		t.Fatalf("compiled %d times, want the cached result", compiles)
	}
	if studio.PhaseDone() {
		t.Fatal("the check phase ended with an open issue")
	}

	patch := studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s2","data":{"actors":[{"cast":"anna","at":"left"}]}}`)
	if patch["status"] != "ok" {
		t.Fatalf("patch_scene answer = %v", patch)
	}

	// The environment item stays listed, is not the model's to fix and does not hold the phase.
	done := studioWriteCall(t, studio, ToolCheck, "validate", `{}`)
	environment, _ := done["environment"].([]any)
	if done["clean"] != true || done["compile"] != "unavailable" || done["rounds_left"] != float64(StudioCheckRounds-2) || len(environment) != 1 {
		t.Fatalf("validate answer = %v", done)
	}
	item := environment[0].(map[string]any)
	if item["code"] != "compile_unverified" || item["model_fixable"] != false {
		t.Fatalf("environment item = %v", item)
	}
	if compiles != 2 {
		t.Fatalf("compiled %d times, want a new compile for the changed story", compiles)
	}
	if !studio.PhaseDone() {
		t.Fatal("an environment item held the check phase open")
	}
}
```

- [ ] **Step 4: Run.** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestStudioCheckFindsUnfilledAndEnvironmentNeverBlocks|TestStudioDraftLifecycleAndCharging|TestStudioCatalogSearchMissAndVoice)$' -count=1`. Expected: PASS.

- [ ] **Step 5: Commit.** Stage `internal/storymaker/studio_check.go`, `internal/storymaker/studio_check_test.go` and `internal/storymaker/model.go`. Message: `feat(storymaker): add Studio job lint, check report and compile cache`.

### Task A7a: Word check
**Batch:** A  **Depends on:** A7
**Files:**
Create `internal/storymaker/studio_words.go`
Modify `internal/storymaker/issues.go`
Modify `internal/storymaker/studio_check.go`
Test `internal/storymaker/studio_words_test.go`

- [ ] **Step 1: Implement.** The word lists are our own. A phrase hit and a dash are each one warning. Both count as advice for the check loop (decision 19), so they never block the outcome.

**Create** `internal/storymaker/studio_words.go`:
```go
package storymaker

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// CodeWordsTell marks a stock phrase of machine copy, or a dash that reads as machine text.
const CodeWordsTell = "words_tell"

// wordsTellEN and wordsTellDE are phrases a film does not need. A match ignores case and needs a
// word boundary on both sides.
var wordsTellEN = []string{
	"in a world where", "little did", "what if", "more than just", "every moment counts",
	"against all odds", "it's not about", "nothing would ever be the same", "imagine", "tapestry",
	"vibrant", "breathtaking", "game changer", "game-changer", "unlock", "seamless", "delve", "journey",
}

var wordsTellDE = []string{
	"in einer welt, in der", "stell dir vor", "mehr als nur", "jeder moment zählt",
	"allen widrigkeiten zum trotz", "es geht nicht um", "nichts würde mehr so sein", "atemberaubend",
	"bahnbrechend", "nahtlos", "auf eine reise", "auf einer reise", "tauche ein", "nicht nur",
}

var (
	wordsTellENRe = wordsTellPattern(wordsTellEN)
	wordsTellDERe = wordsTellPattern(wordsTellDE)
)

// wordsTellPattern matches any phrase of the list, case-insensitively, on word boundaries.
func wordsTellPattern(phrases []string) *regexp.Regexp {
	quoted := make([]string, len(phrases))
	for i, phrase := range phrases {
		quoted[i] = regexp.QuoteMeta(phrase)
	}
	return regexp.MustCompile(`(?i)\b(?:` + strings.Join(quoted, "|") + `)\b`)
}

// LintWords finds stock phrases and dashes in the film text: the narration of each scene and the
// text of each show and say beat. Cue markers are removed before matching. Every language gets the
// dash check. Only English and German also get their phrase list.
func LintWords(st *Story, language string) []Issue {
	if st == nil {
		return nil
	}
	var phrases *regexp.Regexp
	switch LanguageBase(language) {
	case "en":
		phrases = wordsTellENRe
	case "de":
		phrases = wordsTellDERe
	}
	var out []Issue
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		out = lintWordsText(out, sc.ID, jp("scenes", i)+"/narration", sc.Narration, phrases)
		for j := range sc.Beats {
			if b := &sc.Beats[j]; b.Do == "show" || b.Do == "say" {
				out = lintWordsText(out, sc.ID, fmt.Sprintf("%s/beats/%d/text", jp("scenes", i), j), b.Text, phrases)
			}
		}
	}
	return out
}

// lintWordsText appends one issue per phrase hit, and one issue when the text has a dash.
func lintWordsText(out []Issue, sceneID, path, text string, phrases *regexp.Regexp) []Issue {
	text = StripMarkers(text)
	if phrases != nil {
		for _, hit := range phrases.FindAllString(text, -1) {
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: sceneID, Path: path, Code: CodeWordsTell,
				Message: fmt.Sprintf("Scene %s uses the phrase %q, which reads as stock copy.", sceneID, hit),
				Fix:     "Say it concretely, or cut the line.",
			})
		}
	}
	if hasWordsDash(text) {
		out = append(out, Issue{
			Severity: SeverityWarning, SceneID: sceneID, Path: path, Code: CodeWordsTell,
			Message: fmt.Sprintf("Scene %s uses a dash. Machine text uses dashes where a comma, a full stop or a colon reads better.", sceneID),
			Fix:     "Replace the dash with a comma, a full stop or a colon.",
		})
	}
	return out
}

// hasWordsDash says whether text has an em dash, a double hyphen, or an en dash that does not join
// two numbers.
func hasWordsDash(text string) bool {
	if strings.ContainsRune(text, '—') || strings.Contains(text, "--") {
		return true
	}
	runes := []rune(text)
	for i, r := range runes {
		if r != '–' {
			continue
		}
		if i > 0 && i+1 < len(runes) && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
			continue
		}
		return true
	}
	return false
}
```

**Edit** `internal/storymaker/issues.go`
Old:
```go
	case CodeFirstFrameEmpty, CodeDeadStretch, CodeFadeBetweenWorlds, CodeCueLate:
		return true
```
New:
```go
	case CodeFirstFrameEmpty, CodeDeadStretch, CodeFadeBetweenWorlds, CodeCueLate, CodeWordsTell:
		return true
```

**Edit** `internal/storymaker/studio_check.go`
Old:
```go
	found = append(found, checkCompileIssues(summary.Issues)...)
```
New:
```go
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LintWords(&story, s.job.Language)...)
```

- [ ] **Step 2: Test.**

**Create** `internal/storymaker/studio_words_test.go`:
```go
package storymaker

import (
	"context"
	"testing"
)

func TestLintWordsFindsTellsAndDashes(t *testing.T) {
	german := &Story{Scenes: []Scene{
		{ID: "s1", Narration: "Stell dir vor, dein Büro ist leer.", Beats: []Beat{{Do: "show", Text: "Mehr als nur Papier"}}},
		{ID: "s2", Narration: "Die Seiten 1–3 sind wichtig. Das {1}ist nahtlos — fast."},
	}}
	got := LintWords(german, "de-DE")
	// s1 narration, s1 show text, s2 phrase and s2 em dash. The en dash between 1 and 3 is not flagged.
	if len(got) != 4 {
		t.Fatalf("de: %d issues, want 4: %+v", len(got), got)
	}
	for _, is := range got {
		if is.Code != CodeWordsTell || is.Severity != SeverityWarning || is.Fix == "" {
			t.Errorf("issue %+v is not a words_tell warning with a fix", is)
		}
	}

	english := &Story{Scenes: []Scene{{ID: "s1", Narration: "Little {1}did we know. Imagine the office."}}}
	if got := LintWords(english, "en"); len(got) != 2 {
		t.Fatalf("en: %d issues, want 2 (a marker inside a phrase does not hide it): %+v", len(got), got)
	}

	// French gets the dash check only, so the German phrases pass and the em dash of s2 is found.
	got = LintWords(german, "fr-FR")
	if len(got) != 1 || got[0].SceneID != "s2" || got[0].Code != CodeWordsTell {
		t.Fatalf("fr: %+v, want one dash issue on s2", got)
	}
}

func TestStudioCheckAdvisesWordsTell(t *testing.T) {
	s, _, _ := fixtureStudio(t, nil)
	s.SetPhase(PhaseOutline, "")
	if res, _ := callOutline(t, s, "concept_explainer", "Stell dir vor, das Büro ist leer.", "Ein zweiter Satz."); res.Status != "ok" {
		t.Fatalf("outline: %s", res.JSON)
	}
	report, err := s.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range report.Advice {
		if is.Code == CodeWordsTell {
			return
		}
	}
	t.Fatalf("advice = %+v, want a words_tell item", report.Advice)
}
```

- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 ./internal/storymaker -run '^(TestLintWordsFindsTellsAndDashes|TestStudioCheckAdvisesWordsTell)$'`. Expected: `ok`.
- [ ] **Step 4: Commit:** stage `internal/storymaker/studio_words.go`, `internal/storymaker/issues.go`, `internal/storymaker/studio_check.go`, `internal/storymaker/studio_words_test.go`; message `feat(storymaker): flag stock phrases and dashes in film text`.

<!-- group A3 -->

### Task A8: `DecideOutcome`, `MakeSilent`, `StoryIsBlank`
**Batch:** A  **Depends on:** A7
**Files:**
Create `internal/storymaker/agentjob.go`
Test `internal/storymaker/agentjob_test.go`

- [ ] **Step 1: Implement the job types, the outcome rule, the silent rewrite and the backup store.**

**Create** `internal/storymaker/agentjob.go`:
```go
package storymaker

import (
	"context"
	"errors"
	"io/fs"
)

// Open-check codes that are no issue codes. render.CodeModelError has the same text as a job error code.
const (
	outcomeCheckCompileUnverified      = "compile_unverified"
	outcomeCheckModelError             = "model_error"
	outcomeCheckCorrectionsExhausted   = "corrections_exhausted"
)

// silentBackupName is the copy of story.json that make-silent keeps next to it, so the speech can come back.
const silentBackupName = "story.before-silent.json"

// silentWordsPerSecond and silentPadSeconds turn the words of a silent scene into its length. Without an explicit
// length a silent scene falls to its 1 s minimum or its last beat.
const (
	silentWordsPerSecond = 2.3
	silentPadSeconds     = 0.4
)

// AgentCheck is one open item of a finished agent job.
type AgentCheck struct {
	Code    string `json:"code"`
	SceneID string `json:"scene_id,omitempty"`
}

// AgentJobInfo is the agent part of a job record. Kind is create or revise. Phase is outline, cast, fill, check,
// revise or narrating. Request holds the first 200 runes of the request, scrubbed.
type AgentJobInfo struct {
	Kind            string       `json:"kind"`
	Outcome         string       `json:"outcome,omitempty"`
	Phase           string       `json:"phase,omitempty"`
	SceneID         string       `json:"scene_id,omitempty"`
	ScenesDone      int          `json:"scenes_done"`
	ScenesTotal     int          `json:"scenes_total"`
	StoryETag       string       `json:"story_etag,omitempty"`
	OpenChecks      []AgentCheck `json:"open_checks,omitempty"`
	Summary         string       `json:"summary,omitempty"`
	Provider        string       `json:"provider,omitempty"`
	Model           string       `json:"model,omitempty"`
	Compact         bool         `json:"compact,omitempty"`
	TextMode        bool         `json:"text_mode,omitempty"`
	TargetSeconds   int          `json:"target_seconds,omitempty"`
	DurationSeconds float64      `json:"duration_seconds,omitempty"`
	Request         string       `json:"request,omitempty"`
	Tokens          int          `json:"tokens,omitempty"`
}

// OutcomeInput is what a finished job knows. Narration is "", narration_unavailable, narration_failed,
// narration_budget or narration_pending.
type OutcomeInput struct {
	Progress   StudioProgress
	Check      CheckReport
	TimedOut   bool
	ModelError bool
	Narration  string
}

// DecideOutcome applies the outcome rule. failed: no outline was saved. incomplete: a timeout, a model error, or an
// unfilled scene, a model-fixable issue, a compile failure or an environment error is left. Otherwise ready_unverified:
// layout is never verified, so this slice never answers ready. Advice is not listed. Environment items are listed.
func DecideOutcome(in OutcomeInput) (outcome string, checks []AgentCheck) {
	seen := map[AgentCheck]bool{}
	add := func(code, sceneID string) {
		check := AgentCheck{Code: code, SceneID: sceneID}
		if !seen[check] {
			seen[check] = true
			checks = append(checks, check)
		}
	}
	add(CodeLayoutUnverified, "")
	if in.Check.Compile == "unavailable" {
		add(outcomeCheckCompileUnverified, "")
	}
	if in.Narration != "" {
		add(in.Narration, "")
	}
	if in.ModelError {
		add(outcomeCheckModelError, "")
	}
	for _, sceneID := range in.Progress.Unfilled {
		add(CodeSceneUnfilled, sceneID)
	}
	// A draft was never saved, so the story keeps an older version of that scene.
	for _, sceneID := range in.Progress.Drafts {
		add(outcomeCheckCorrectionsExhausted, sceneID)
	}
	blocked := in.TimedOut || in.ModelError || len(in.Progress.Unfilled) > 0 || len(in.Progress.Drafts) > 0
	compileFailed := false
	listed := append(append([]Issue(nil), in.Check.Issues...), in.Check.Environment...)
	for _, is := range listed {
		add(is.Code, is.SceneID)
		compileFailed = compileFailed || is.Code == CodeCompileFailed
		// An error among the environment items blocks by its own code. over_target and compile_failed keep their rules.
		blocked = blocked || IssueModelFixable(is) || (is.Severity == SeverityError && is.Code != CodeOverTarget && is.Code != CodeCompileFailed)
	}
	if in.Check.Compile == "failed" && !compileFailed {
		add(CodeCompileFailed, "")
		compileFailed = true
	}
	// A blank story reports zero scenes (Progress), so zero means that no outline is saved.
	noOutline := in.Progress.ScenesTotal == 0
	switch {
	case noOutline:
		return OutcomeFailed, checks
	case blocked || compileFailed:
		return OutcomeIncomplete, checks
	}
	return OutcomeReadyUnverified, checks
}

// StoryIsBlank reports a story an agent may overwrite: at most one scene with nothing in it, no cast and no music.
// The title, the format and the voice do not count, because creating a project sets them.
func StoryIsBlank(st *Story) bool {
	if st == nil || len(st.Cast) > 0 || len(st.Scenes) > 1 {
		return false
	}
	if st.Music != nil && st.Music.Mode != "" && st.Music.Mode != "none" {
		return false
	}
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		if sc.Kind == "code" || StripMarkers(sc.Narration) != "" || len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0 {
			return false
		}
	}
	return true
}

// MakeSilent rewrites a story so that it needs no voice, and returns the number of scenes it changed. In each
// storyboard scene that still speaks, it sets silent and a length from the spoken words, removes the cues and turns
// each say with text into a show beat. Code scenes stay as they are. The caller applies the defaults again, which
// gives the new show beats their ids. A second call changes nothing.
func MakeSilent(st *Story) int {
	if st == nil {
		return 0
	}
	changed := 0
	for i := range st.Scenes {
		sc := &st.Scenes[i]
		if sc.Kind == "code" {
			continue
		}
		words := 0
		narrated := !sc.Silent && StripMarkers(sc.Narration) != ""
		if narrated {
			words = WordCount(StripMarkers(sc.Narration))
		}
		spoken := false
		for j := range sc.Beats {
			if sc.Beats[j].Do == "say" && sayHasText(sc.Beats[j].Text) {
				spoken = true
				words += WordCount(StripMarkers(sc.Beats[j].Text))
			}
		}
		if !narrated && !spoken {
			continue
		}
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if b.Do == "say" && sayHasText(b.Text) {
				*b = Beat{Do: "show", Text: StripMarkers(b.Text), At: SpotAt("center"), Entry: "fade"}
				continue
			}
			b.Cue = 0
		}
		sc.Silent = true
		sc.Duration = SecondsDuration(max(1, float64(words)/silentWordsPerSecond+silentPadSeconds))
		changed++
	}
	return changed
}

// SaveSilentBackup writes the story bytes next to story.json and replaces an older copy.
func (p *ProjectStore) SaveSilentBackup(ctx context.Context, id string, data []byte) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	_, err := p.st.WriteFile(ctx, p.file(id, silentBackupName), data, "")
	return err
}

// LoadSilentBackup returns the kept story bytes. A missing copy wraps fs.ErrNotExist.
func (p *ProjectStore) LoadSilentBackup(ctx context.Context, id string) ([]byte, error) {
	if err := p.checkID(id); err != nil {
		return nil, err
	}
	data, _, err := p.st.ReadFile(ctx, p.file(id, silentBackupName))
	return data, err
}

// HasSilentBackup reports whether a kept copy exists.
func (p *ProjectStore) HasSilentBackup(ctx context.Context, id string) (bool, error) {
	_, err := p.LoadSilentBackup(ctx, id)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// RemoveSilentBackup deletes the kept copy. A missing copy is not an error.
func (p *ProjectStore) RemoveSilentBackup(ctx context.Context, id string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	return p.st.Remove(ctx, p.file(id, silentBackupName))
}
```

- [ ] **Step 2: Add the two named tests.**

**Create** `internal/storymaker/agentjob_test.go`:
```go
package storymaker

import (
	"bytes"
	"context"
	"path"
	"reflect"
	"testing"
)

func TestDecideOutcomeTable(t *testing.T) {
	done := StudioProgress{ScenesTotal: 2, ScenesDone: 2}
	layout := AgentCheck{Code: CodeLayoutUnverified}
	cases := []struct {
		name    string
		in      OutcomeInput
		outcome string
		checks  []AgentCheck
	}{
		{name: "no outline", in: OutcomeInput{Progress: StudioProgress{ScenesTotal: 0}}, outcome: OutcomeFailed, checks: []AgentCheck{layout}},
		{name: "clean, advice is not listed", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Advice: []Issue{{Severity: SeverityWarning, Code: CodeCueLate, SceneID: "s1"}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout}},
		{name: "timeout", in: OutcomeInput{Progress: done, TimedOut: true}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout}},
		{name: "model error", in: OutcomeInput{Progress: done, ModelError: true}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: "model_error"}}},
		{name: "unfilled scene", in: OutcomeInput{Progress: StudioProgress{ScenesTotal: 2, ScenesDone: 1, Unfilled: []string{"s2"}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeSceneUnfilled, SceneID: "s2"}}},
		{name: "model-fixable issue", in: OutcomeInput{Progress: done, Check: CheckReport{Issues: []Issue{{Severity: SeverityError, Code: CodeUnknownName, SceneID: "s1"}}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeUnknownName, SceneID: "s1"}}},
		{name: "compile failed", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "failed"}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeCompileFailed}}},
		{name: "environment never downgrades", in: OutcomeInput{Progress: done, Narration: "narration_unavailable", Check: CheckReport{Compile: "unavailable", Environment: []Issue{{Severity: SeverityWarning, Code: CodeOverTarget}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout, {Code: "compile_unverified"}, {Code: "narration_unavailable"}, {Code: CodeOverTarget}}},
		{name: "environment error blocks by its code", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Environment: []Issue{{Severity: SeverityError, Code: CodeDeadline}}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeDeadline}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, checks := DecideOutcome(tc.in)
			if outcome != tc.outcome {
				t.Errorf("outcome = %q, want %q", outcome, tc.outcome)
			}
			if !reflect.DeepEqual(checks, tc.checks) {
				t.Errorf("checks = %+v, want %+v", checks, tc.checks)
			}
		})
	}
}

const silentFixture = `{"schema":"storymaker/1","title":"Firewall",
 "cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],
 "scenes":[
  {"id":"s1","environment":"env.office","narration":"A firewall {1} checks every packet.",
   "actors":[{"cast":"anna","at":"left"}],
   "beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1},
            {"do":"say","who":"anna","text":"This is {2} a packet.","style":"speech"}]},
  {"id":"s2","actors":[{"cast":"anna"}],
   "beats":[{"do":"say","who":"anna","text":"Only one line.","style":"speech"}]}]}`

func TestMakeSilentPassesTheAudioGate(t *testing.T) {
	ps, mem, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, err := ps.Create(ctx, "Firewall", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := DecodeStory([]byte(silentFixture), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	ApplyDefaults(&story)
	before, err := ps.AudioAnswer(ctx, proj.ID, &story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Missing) != 3 { // the narration of s1 and the say beats of s1 and s2
		t.Fatalf("missing before = %+v", before.Missing)
	}
	backup, err := CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.SaveSilentBackup(ctx, proj.ID, backup); err != nil {
		t.Fatal(err)
	}

	if changed := MakeSilent(&story); changed != 2 {
		t.Fatalf("changed = %d, want 2", changed)
	}
	if again := MakeSilent(&story); again != 0 {
		t.Fatalf("a second call changed %d scenes", again)
	}
	ApplyDefaults(&story)
	after, err := ps.AudioAnswer(ctx, proj.ID, &story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Missing) != 0 {
		t.Fatalf("missing after = %+v", after.Missing)
	}
	issues := Validate(&story, loadReal(t), DefaultLimits())
	if HasErrors(issues) {
		t.Fatalf("issues = %+v", issues)
	}
	for _, is := range issues {
		if is.Code == CodeUnknownCue {
			t.Errorf("a cue survived: %+v", is)
		}
	}
	wantSeconds := []float64{
		max(1, float64(WordCount("A firewall checks every packet.")+WordCount("This is a packet."))/2.3+0.4),
		max(1, float64(WordCount("Only one line."))/2.3+0.4),
	}
	for i := range story.Scenes {
		sc := &story.Scenes[i]
		if !sc.Silent || sc.Duration == nil || sc.Duration.Seconds != wantSeconds[i] {
			t.Errorf("scene %s: silent %v duration %+v, want %v seconds", sc.ID, sc.Silent, sc.Duration, wantSeconds[i])
		}
		for j := range sc.Beats {
			if sc.Beats[j].Do == "say" || sc.Beats[j].Cue != 0 {
				t.Errorf("scene %s beat %d = %+v", sc.ID, j, sc.Beats[j])
			}
		}
	}
	shown := story.Scenes[0].Beats[1]
	if shown.Do != "show" || shown.Text != "This is a packet." || shown.At.String() != "center" || shown.Entry != "fade" {
		t.Errorf("the say became %+v", shown)
	}

	// The speech can come back from the copy next to story.json.
	stored, _, err := mem.ReadFile(ctx, path.Join(testRoot, proj.ID, "story.before-silent.json"))
	if err != nil || !bytes.Equal(stored, backup) {
		t.Fatalf("stored copy: %v", err)
	}
	data, err := ps.LoadSilentBackup(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	restored, _, err := DecodeStory(data, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	spoken, err := ps.AudioAnswer(ctx, proj.ID, &restored, nil)
	if err != nil || len(spoken.Missing) != 3 {
		t.Fatalf("restored story: missing %+v, error %v", spoken.Missing, err)
	}
	if err := ps.RemoveSilentBackup(ctx, proj.ID); err != nil {
		t.Fatal(err)
	}
	if has, err := ps.HasSilentBackup(ctx, proj.ID); has || err != nil {
		t.Fatalf("has backup = %v, error %v", has, err)
	}
}
```

- [ ] **Step 3: Run the tests.**
`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run 'TestDecideOutcomeTable|TestMakeSilentPassesTheAudioGate' -count=1`
Expected: `ok  	aurago/internal/storymaker`.

- [ ] **Step 4: Commit.** Stage `internal/storymaker/agentjob.go` and `internal/storymaker/agentjob_test.go`. Message: `feat(storymaker): decide the agent outcome and make a story silent`.

### Task A8a: ChangedScenes and its fields
**Batch:** A  **Depends on:** A8
**Files:**
Modify `internal/storymaker/agentjob.go`
Test `internal/storymaker/agentjob_changed_test.go`

- [ ] **Step 1: Implement.** The three lists are what a revise job reports. A create job leaves them empty.

**Edit** `internal/storymaker/agentjob.go`
Old:
```go
import (
	"context"
	"errors"
	"io/fs"
)
```
New:
```go
import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
)
```

**Edit** `internal/storymaker/agentjob.go`
Old:
```go
	Request         string       `json:"request,omitempty"`
	Tokens          int          `json:"tokens,omitempty"`
}
```
New:
```go
	Request         string       `json:"request,omitempty"`
	Tokens          int          `json:"tokens,omitempty"`
	ChangedScenes   []string     `json:"changed_scenes,omitempty"`
	UnchangedScenes []string     `json:"unchanged_scenes,omitempty"`
	RemovedScenes   []string     `json:"removed_scenes,omitempty"`
}
```

**Edit** `internal/storymaker/agentjob.go`
Old:
```go
// MakeSilent rewrites a story so that it needs no voice, and returns the number of scenes it changed. In each
```
New:
```go
// ChangedScenes compares two versions of a story scene by scene, by id. changed holds the scenes that
// differ or are new, and unchanged the scenes that are equal. A moved scene that is otherwise equal is
// unchanged. removed holds the ids that only before has. changed and unchanged follow after, and removed
// follows before. A nil story counts as having no scenes.
func ChangedScenes(before, after *Story) (changed, unchanged, removed []string) {
	old := map[string]string{}
	if before != nil {
		for i := range before.Scenes {
			old[before.Scenes[i].ID] = sceneFingerprint(&before.Scenes[i])
		}
	}
	seen := map[string]bool{}
	if after != nil {
		for i := range after.Scenes {
			sc := &after.Scenes[i]
			seen[sc.ID] = true
			if prev, had := old[sc.ID]; had && prev == sceneFingerprint(sc) {
				unchanged = append(unchanged, sc.ID)
			} else {
				changed = append(changed, sc.ID)
			}
		}
	}
	if before != nil {
		for i := range before.Scenes {
			if id := before.Scenes[i].ID; !seen[id] {
				removed = append(removed, id)
			}
		}
	}
	return changed, unchanged, removed
}

// sceneFingerprint is the JSON encoding of a scene. The position of the scene in the story is not part of it.
func sceneFingerprint(sc *Scene) string {
	raw, _ := json.Marshal(sc)
	return string(raw)
}

// MakeSilent rewrites a story so that it needs no voice, and returns the number of scenes it changed. In each
```

- [ ] **Step 2: Test.**

**Create** `internal/storymaker/agentjob_changed_test.go`:
```go
package storymaker

import (
	"slices"
	"testing"
)

func TestChangedScenesListsChangedUnchangedRemoved(t *testing.T) {
	before := &Story{Scenes: []Scene{{ID: "s1", Narration: "a"}, {ID: "s2", Narration: "b"}, {ID: "s3", Narration: "c"}}}
	after := &Story{Scenes: []Scene{{ID: "s2", Narration: "b"}, {ID: "s1", Narration: "a, changed"}, {ID: "s4", Narration: "d"}}}
	changed, unchanged, removed := ChangedScenes(before, after)
	if !slices.Equal(changed, []string{"s1", "s4"}) || !slices.Equal(unchanged, []string{"s2"}) || !slices.Equal(removed, []string{"s3"}) {
		t.Fatalf("changed %v, unchanged %v, removed %v", changed, unchanged, removed)
	}
}
```
- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test -count=1 ./internal/storymaker -run '^TestChangedScenesListsChangedUnchangedRemoved$'`. Expected: `ok`.
- [ ] **Step 4: Commit:** stage `internal/storymaker/agentjob.go` and `internal/storymaker/agentjob_changed_test.go`; message `feat(storymaker): report the scenes a revise changed`.

### Task A9: `CompileCheck`
**Batch:** A  **Depends on:** A7
**Files:**
Create `internal/storymaker/render/compilecheck.go`
Modify `internal/storymaker/render/render.go`
Test `internal/storymaker/render/compilecheck_test.go`

- [ ] **Step 1: Impact.** Run the impact command of the plan index for `prepareStory` in `internal/storymaker/render/render.go`. Section 5 expects HIGH or CRITICAL here, so report the callers and the risk before you edit.

- [ ] **Step 2: Split `prepareStory`.** `prepareCore` takes the gate as a parameter, so the gate keeps its place before the code bundles and the order of the refusals does not change.

**Edit** `internal/storymaker/render/render.go`
Old:
```go
func (r *Runner) prepareStory(ctx context.Context, projectID, ifMatch string, st Settings, opts RenderOptions) (*Plan, error) {
	data, etag, err := r.deps.Projects.LoadStory(ctx, projectID)
```
New:
```go
func (r *Runner) prepareStory(ctx context.Context, projectID, ifMatch string, st Settings, opts RenderOptions) (*Plan, error) {
	return r.prepareCore(ctx, projectID, ifMatch, st, opts, checkAudioGates)
}

// prepareCore builds the plan. gate judges the stored audio after validation and before the code bundles, so the order
// of the refusals stays the same. A nil gate skips it: the compile check estimates what has no audio.
func (r *Runner) prepareCore(ctx context.Context, projectID, ifMatch string, st Settings, opts RenderOptions, gate func(answer storymaker.AudioAnswer, musicKey string) error) (*Plan, error) {
	data, etag, err := r.deps.Projects.LoadStory(ctx, projectID)
```

**Edit** `internal/storymaker/render/render.go`
Old:
```go
	if err := checkAudioGates(answer, musicKey); err != nil {
		return nil, err
	}
```
New:
```go
	if gate != nil {
		if err := gate(answer, musicKey); err != nil {
			return nil, err
		}
	}
```

- [ ] **Step 3: Implement the compile check.**

**Create** `internal/storymaker/render/compilecheck.go`:
```go
package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"time"

	"aurago/internal/security"
	"aurago/internal/storymaker"
)

// compileCheckLimit bounds one compile check, browser probe included. A variable, so a test can shorten it.
var compileCheckLimit = 60 * time.Second

const (
	// compileOnlyAnswer is the error of the report answer. host-render.js ends on any answer without start.
	compileOnlyAnswer = "compile_only"
	// Reasons a check could not run. None of them says anything about the story.
	compileReasonTimeout = "compile_timeout"
	compileReasonCrashed = "browser_crashed"
	compileReasonSetup   = "compile_setup"

	compileProxyConnections = 400
	compileProxyBytes       = int64(256) << 20
	compileShutdownWait     = 2 * time.Second
	compileMaxFrames        = math.MaxInt32
	compileSceneFix         = "Rewrite this scene with patch_scene or put_scene, then check again."
)

// compileCheckReport is the part of the report a compile check keeps. parseReport validates the same body.
type compileCheckReport struct {
	FrameCount int                 `json:"frame_count"`
	Scenes     []compileCheckScene `json:"scenes"`
	Issues     []storymaker.Issue  `json:"issues"`
}

type compileCheckScene struct {
	ID         string            `json:"id"`
	StartFrame int               `json:"start_frame"`
	EndFrame   int               `json:"end_frame"`
	Cues       []compileCheckCue `json:"cues"`
}

type compileCheckCue struct {
	Marker int     `json:"marker"`
	AtMS   float64 `json:"at_ms"`
}

// CompileCheck compiles the saved story in the real page and stops after the report: no FFmpeg, no frames, no download.
// A check that cannot run answers Available false with a Reason. A cancelled ctx returns its error. A page or compile
// failure is an issue with code compile_failed. Cue times are scene-local, as the report gives them.
func (r *Runner) CompileCheck(ctx context.Context, projectID string, st Settings) (storymaker.CompileSummary, error) {
	checkCtx, cancel := context.WithTimeout(ctx, compileCheckLimit)
	defer cancel()
	if r.deps.Browsers == nil {
		return compileUnavailable(ctx, CodeBrowserUnavailable)
	}
	status, bin := r.deps.Browsers.resolve(checkCtx, st.BrowserPath, false, nil)
	if !status.Ready {
		return compileUnavailable(ctx, CodeBrowserUnavailable)
	}
	plan, err := r.prepareCore(checkCtx, projectID, "", st, RenderOptions{}, nil)
	if err != nil {
		var refused *PrepareError
		if errors.As(err, &refused) {
			return compileUnavailable(ctx, refused.Code)
		}
		return storymaker.CompileSummary{}, err
	}
	return r.compileInBrowser(ctx, checkCtx, plan, bin, st)
}

// compileUnavailable is the answer of a check that could not run. A cancelled job is no such answer.
func compileUnavailable(ctx context.Context, reason string) (storymaker.CompileSummary, error) {
	if err := ctx.Err(); err != nil {
		return storymaker.CompileSummary{}, err
	}
	return storymaker.CompileSummary{Reason: reason}, nil
}

func (r *Runner) compileInBrowser(ctx, checkCtx context.Context, plan *Plan, bin string, st Settings) (storymaker.CompileSummary, error) {
	setupFailed := func(err error) (storymaker.CompileSummary, error) {
		r.deps.Logger.Warn("[Storymaker] The compile check could not start", "error", err)
		return compileUnavailable(ctx, compileReasonSetup)
	}
	root := StagingRoot(r.deps.Dir)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return setupFailed(err)
	}
	staging, err := os.MkdirTemp(root, "compile-")
	if err != nil {
		return setupFailed(err)
	}
	defer os.RemoveAll(staging)
	snap, err := (&renderRun{r: r, plan: plan}).snapshot()
	if err != nil {
		return setupFailed(err)
	}
	prefix, err := NewPrefix()
	if err != nil {
		return setupFailed(err)
	}
	token, err := NewPrefix()
	if err != nil {
		return setupFailed(err)
	}
	width, height := CanvasSize(plan.Story.Format.Aspect)
	fps := plan.Story.Format.FPS
	if fps <= 0 {
		fps = storymaker.DefaultFPS
	}
	frameLimit := int(st.Limits.MaxDurationSeconds) * fps
	if frameLimit <= 0 {
		frameLimit = st.MaxDurationSeconds * fps
	}
	reports := make(chan []byte, 1)
	pageErrors := make(chan PageError, 1)
	// No workers: helper workers would only start up for frames that never come.
	listener, err := StartListener(prefix, snap, PageParams{Width: width, Height: height, FPS: fps, Transport: DefaultTransport, FrameLimit: frameLimit, FrameDeadlineMS: frameDeadlineMS, Token: token, AudioInfo: plan.AudioJSON},
		Hooks{
			Report: func(body []byte) ReportAnswer {
				select {
				case reports <- body:
				default:
				}
				return ReportAnswer{Error: compileOnlyAnswer, Message: "This page only compiles the story."}
			},
			Error: func(e PageError) {
				select {
				case pageErrors <- e:
				default:
				}
			},
		})
	if err != nil {
		return setupFailed(err)
	}
	defer listener.Close()
	origin, err := url.Parse(listener.Origin())
	if err != nil {
		return setupFailed(err)
	}
	proxy, err := security.StartStrictCaptureProxy(origin, security.CaptureProxyLimits{
		MaxConcurrent:         16,
		MaxConnections:        compileProxyConnections,
		MaxBytes:              compileProxyBytes,
		ResponseHeaderTimeout: proxyHeaderTimeout,
	})
	if err != nil {
		return setupFailed(err)
	}
	defer proxy.Close()
	opts := sessionOptions{Bin: bin, Profile: staging + string(os.PathSeparator) + "profile", ProxyAddr: proxy.Addr(), Base: listener.Base(), Width: width, Height: height}
	if r.deps.SessionTweak != nil {
		r.deps.SessionTweak(&opts)
	}
	page, err := r.deps.OpenPage(checkCtx, opts)
	if err != nil {
		r.deps.Logger.Warn("[Storymaker] The compile check could not open a browser", "issue", launchIssue(err))
		return compileUnavailable(ctx, CodeBrowserUnavailable)
	}
	defer page.Close()
	if err := page.Navigate(checkCtx, listener.Base()+"/"); err != nil {
		return setupFailed(err)
	}

	ids := make([]string, len(plan.Story.Scenes))
	for i := range plan.Story.Scenes {
		ids[i] = plan.Story.Scenes[i].ID
	}
	blocks, captionText, maxEvents := planReportBlocks(plan)
	want := reportExpect{FPS: fps, Width: width, Height: height, MaxFrames: compileMaxFrames, CatalogVersion: r.deps.Catalog.Version(),
		Catalog: r.deps.Catalog, SFXEvents: sfxEventBound(&plan.Story, r.deps.Catalog), SceneIDs: ids,
		Blocks: blocks, CaptionText: captionText, MaxEvents: maxEvents, Prefix: prefix, Token: token}
	select {
	case <-checkCtx.Done():
		return compileUnavailable(ctx, compileReasonTimeout)
	case <-page.Crashed():
		return compileUnavailable(ctx, compileReasonCrashed)
	case e := <-pageErrors:
		sceneID := ""
		if knownCompileScene(want, e.SceneID) {
			sceneID = e.SceneID
		}
		return compileFailure(sceneID, e.Code+": "+e.Message), nil
	case body := <-reports:
		// Shutdown lets the handler finish, so the page reads its answer. Close would cut it.
		shutdownCtx, stop := context.WithTimeout(context.Background(), compileShutdownWait)
		_ = listener.srv.Shutdown(shutdownCtx)
		stop()
		return summarizeCompileReport(body, want, frameLimit), nil
	}
}

// summarizeCompileReport turns a report into the summary. An error issue keeps its scene, so the model can fix it. The
// frame limit is no refusal here: a film over it is reported as over_target.
func summarizeCompileReport(body []byte, want reportExpect, frameLimit int) storymaker.CompileSummary {
	var rep compileCheckReport
	if err := json.Unmarshal(body, &rep); err != nil {
		return compileFailure("", "The compile report is not valid JSON.")
	}
	issues := cleanCompileIssues(rep.Issues, want)
	if storymaker.HasErrors(issues) {
		return storymaker.CompileSummary{Available: true, Issues: issues}
	}
	if _, refusal := parseReport(body, want); refusal != nil {
		return compileFailure("", refusal.Message)
	}
	if frameLimit > 0 && rep.FrameCount > frameLimit {
		issues = append(issues, storymaker.Issue{
			Severity: storymaker.SeverityWarning, Path: "/scenes", Code: storymaker.CodeOverTarget,
			Message: fmt.Sprintf("The film runs %d seconds, and an export allows at most %d seconds.", rep.FrameCount/want.FPS, frameLimit/want.FPS),
			Fix:     "Shorten the narration or remove a scene.",
		})
	}
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	for _, sc := range rep.Scenes {
		scene := storymaker.CompileScene{ID: sc.ID, StartMS: compileFrameMS(sc.StartFrame, want.FPS), EndMS: compileFrameMS(sc.EndFrame, want.FPS)}
		for _, cue := range sc.Cues {
			scene.CuesMS = append(scene.CuesMS, storymaker.CompileCue{Marker: cue.Marker, AtMS: int64(math.Round(cue.AtMS))})
		}
		scenes = append(scenes, scene)
	}
	return storymaker.CompileSummary{Available: true, DurationMS: compileFrameMS(rep.FrameCount, want.FPS), Scenes: scenes, Issues: issues}
}

// cleanCompileIssues removes the render secrets from the page's text and bounds it. An error issue becomes compile_failed
// and keeps its own code in the message.
func cleanCompileIssues(list []storymaker.Issue, want reportExpect) []storymaker.Issue {
	out := make([]storymaker.Issue, 0, len(list))
	for _, is := range list {
		is.SceneID = want.clean(is.SceneID, 60)
		if !knownCompileScene(want, is.SceneID) {
			is.SceneID = ""
		}
		is.Path = want.clean(is.Path, 200)
		is.Message = want.clean(is.Message, 400)
		is.Fix = want.clean(is.Fix, 400)
		is.Code = want.clean(is.Code, 60)
		switch is.Severity {
		case storymaker.SeverityError:
			is.Message = is.Code + ": " + is.Message
			is.Code = storymaker.CodeCompileFailed
			if is.Fix == "" && is.SceneID != "" {
				is.Fix = compileSceneFix
			}
		case storymaker.SeverityWarning, storymaker.SeverityMinor:
		default:
			is.Severity = storymaker.SeverityMinor
		}
		out = append(out, is)
	}
	return storymaker.CapIssues(out, storymaker.MaxHTTPIssues)
}

func compileFailure(sceneID, message string) storymaker.CompileSummary {
	issue := storymaker.Issue{Severity: storymaker.SeverityError, SceneID: sceneID, Code: storymaker.CodeCompileFailed, Message: message}
	if sceneID != "" {
		issue.Fix = compileSceneFix
	}
	return storymaker.CompileSummary{Available: true, Issues: []storymaker.Issue{issue}}
}

func knownCompileScene(want reportExpect, id string) bool {
	for _, known := range want.SceneIDs {
		if known == id {
			return true
		}
	}
	return false
}

func compileFrameMS(frames, fps int) int64 {
	return int64(math.Round(float64(frames) * 1000 / float64(fps)))
}
```

- [ ] **Step 4: Add the two named tests.**

**Create** `internal/storymaker/render/compilecheck_test.go`:
```go
package render

import (
	"context"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/storymaker"
)

// compileCheckRunner needs neither FFmpeg nor a publisher: a check that touched them would panic on the nil publisher.
func compileCheckRunner(t *testing.T, page *fakePage) (*Runner, string) {
	t.Helper()
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	projects := storymaker.NewProjectStore(storymaker.NewMemStore(), ProjectsRoot)
	proj, _, _, err := projects.Create(context.Background(), "Compile check", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	browsers := NewBrowsersWithProbe(dir, func() (string, bool) { return "/fake/chrome", true }, func(context.Context, string) (string, error) { return "Fake/1", nil })
	return NewRunner(Deps{Dir: dir, UI: ui, Catalog: cat, Projects: projects, Browsers: browsers,
		OpenPage: func(context.Context, sessionOptions) (browserPage, error) { return page, nil }}), proj.ID
}

func TestCompileCheckStopsAfterReport(t *testing.T) {
	limits := storymaker.DefaultLimits()
	limits.MaxDurationSeconds = 1 // 30 frames, so the 60 frames below are over the export limit

	t.Run("summary", func(t *testing.T) {
		var sent atomic.Int64
		told := make(chan ReportAnswer, 1)
		page := &fakePage{crashed: make(chan struct{})}
		page.script = play(t, playOptions{frames: 60, framesSent: &sent,
			onRefused: func(a ReportAnswer) { told <- a },
			mutateRep: func(rep map[string]any) {
				rep["scenes"].([]map[string]any)[0]["cues"] = []any{map[string]any{"marker": 1, "at_ms": 700, "placed": "estimate"}}
				rep["issues"] = []any{map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "dead_stretch", "message": "nothing moves"}}
			}})
		runner, id := compileCheckRunner(t, page)
		got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: limits})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case answer := <-told:
			if answer.Error != compileOnlyAnswer {
				t.Errorf("the page was told %+v", answer)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the page never got an answer to its report")
		}
		if sent.Load() != 0 || !page.closed.Load() {
			t.Errorf("frames sent %d, page closed %v", sent.Load(), page.closed.Load())
		}
		wantScenes := []storymaker.CompileScene{{ID: "s1", StartMS: 0, EndMS: 2000, CuesMS: []storymaker.CompileCue{{Marker: 1, AtMS: 700}}}}
		if !got.Available || got.DurationMS != 2000 || !reflect.DeepEqual(got.Scenes, wantScenes) {
			t.Errorf("summary = %+v", got)
		}
		if len(got.Issues) != 2 || got.Issues[0].Code != storymaker.CodeDeadStretch || got.Issues[1].Code != storymaker.CodeOverTarget {
			t.Errorf("issues = %+v", got.Issues)
		}
		entries, err := os.ReadDir(StagingRoot(runner.deps.Dir))
		if err != nil || len(entries) != 0 {
			t.Errorf("staging left %v, error %v", entries, err)
		}
	})

	t.Run("error issue keeps its scene", func(t *testing.T) {
		page := &fakePage{crashed: make(chan struct{})}
		page.script = play(t, playOptions{frames: 24, mutateRep: func(rep map[string]any) {
			rep["issues"] = []any{map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "unknown_asset", "message": "no such asset"}}
		}})
		runner, id := compileCheckRunner(t, page)
		got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits()})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || len(got.Issues) != 1 {
			t.Fatalf("summary = %+v", got)
		}
		is := got.Issues[0]
		if is.Code != storymaker.CodeCompileFailed || is.SceneID != "s1" || !strings.Contains(is.Message, "no such asset") || !storymaker.IssueModelFixable(is) {
			t.Errorf("issue = %+v", is)
		}
	})
}

func TestCompileCheckBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	fx := newRealFixture(t, chrome)
	fx.rebuild(t)
	ctx := context.Background()
	doc := `{"schema":"storymaker/1","title":"Compile check","scenes":[
	 {"id":"s1","environment":"env.office","narration":"A firewall {1} checks every packet before it reaches your network.",
	  "beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1}]},
	 {"id":"s2","narration":"After that everything runs safely."}]}`
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(doc), ""); err != nil {
		t.Fatal(err)
	}
	got, err := fx.runner.CompileCheck(ctx, fx.projectID, fx.settings)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Available || got.DurationMS <= 0 || len(got.Scenes) != 2 {
		t.Fatalf("summary = %+v", got)
	}
	if storymaker.HasErrors(got.Issues) {
		t.Fatalf("issues = %+v", got.Issues)
	}
	first := got.Scenes[0]
	if first.ID != "s1" || first.EndMS <= first.StartMS || len(first.CuesMS) != 1 || first.CuesMS[0].Marker != 1 || first.CuesMS[0].AtMS <= 0 {
		t.Errorf("first scene = %+v", first)
	}
	if got.Scenes[1].EndMS != got.DurationMS {
		t.Errorf("the last scene ends at %d, the film at %d", got.Scenes[1].EndMS, got.DurationMS)
	}
	if len(fx.pub.names()) != 0 {
		t.Errorf("a compile check published %v", fx.pub.names())
	}
}
```

- [ ] **Step 5: Run the fake-page test and the existing Prepare tests.**
`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render -run 'TestCompileCheckStopsAfterReport|TestPrepare' -count=1`
Expected: `ok  	aurago/internal/storymaker/render`. The Prepare tests that need FFmpeg skip when it is not installed.

- [ ] **Step 6: Run the browser test.** The shell sandbox must be off.
`AURAGO_RUN_BROWSER_SMOKE=1 GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render -run '^TestCompileCheckBrowser$' -count=1 -timeout 5m`
Expected: `ok  	aurago/internal/storymaker/render`. It skips when Chrome or FFmpeg is not installed. The skip comes from the shared fixture, not from the check.

- [ ] **Step 7: Commit.** Stage `internal/storymaker/render/compilecheck.go`, `internal/storymaker/render/compilecheck_test.go` and `internal/storymaker/render/render.go`. Message: `feat(storymaker): compile a story in Chrome without rendering frames`.

### Task A10: Packets
**Batch:** A  **Depends on:** A7
**Files:**
Create `internal/storymaker/studio_packet.go`

- [ ] **Step 1: Implement the context packets.** `packetState` is the only line that reads unexported `Studio` fields. It expects the fields `phase`, `sceneID`, `job` and `deps` from task A3. If the build reports another name, change only that return line.

**Create** `internal/storymaker/studio_packet.go`:
```go
package storymaker

import (
	"context"
	"fmt"
	"math"
	"time"
)

const (
	packetNoVoiceNote = "No voice is ready, so viewers may read instead of listen. Still write the narration, and add one short show beat with text in each scene."

	packetWordsPerSecond = 2.3
	packetWordsPerScene  = 19
	packetMinScenes      = 2
	packetMaxScenes      = 24
	packetAdviceLimit    = 3
	packetNarrationRunes = 120
)

var packetTemplates = []string{"product_explainer", "how_to", "concept_explainer", "dialogue_story", "data_story", "onboarding"}

// packetState reads the session fields of the packet. It takes s.mu and releases it before it returns,
// so Packet can call Check and Progress, which lock.
func (s *Studio) packetState() (phase, sceneID string, job StudioJob, deps StudioDeps) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase, s.sceneID, s.job, s.deps
}

// Packet builds the context packet of the current phase from the saved story. The server adds request and instruction,
// because the session does not hold the user's text. Normal fill and check packets add the story summary, which compact
// ones leave out.
func (s *Studio) Packet(ctx context.Context, compact bool) (map[string]any, error) {
	phase, sceneID, job, deps := s.packetState()
	story, err := packetStory(ctx, deps, job.ProjectID)
	if err != nil {
		return nil, err
	}
	packet := map[string]any{
		"phase":           phase,
		"language":        job.Language,
		"aspect":          story.Format.Aspect,
		"voice_available": job.VoiceAvailable,
		"budget":          packetBudget(ctx, phase),
	}
	switch phase {
	case PhaseOutline:
		packet["audience"] = job.Audience
		packet["target_seconds"] = job.TargetSeconds
		packet["target_words"] = int(math.Round(float64(job.TargetSeconds) * packetWordsPerSecond))
		packet["suggested_scenes"] = packetSuggestedScenes(job.TargetSeconds)
		packet["templates"] = packetTemplates
	case PhaseCast:
		packet["story"] = packetSummary(story)
		packet["characters"] = deps.Catalog.Names("character")
		packet["environments"] = deps.Catalog.Names("environment")
	case PhaseFill:
		if err := packetFill(ctx, packet, story, sceneID, job, deps); err != nil {
			return nil, err
		}
		if !compact {
			packet["story"] = packetSummary(story)
		}
	case PhaseCheck:
		report, err := s.Check(ctx)
		if err != nil {
			return nil, err
		}
		packet["issues"] = append([]Issue{}, CapIssues(report.Issues, MaxModelIssues)...)
		packet["advice"] = append([]Issue{}, CapIssues(report.Advice, packetAdviceLimit)...)
		packet["cast"] = packetCast(story)
		packet["environments"] = packetEnvironments(deps.Catalog)
		if !compact {
			packet["story"] = packetSummary(story)
		}
	case PhaseRevise:
		progress, err := s.Progress(ctx)
		if err != nil {
			return nil, err
		}
		packet["story"] = packetSummary(story)
		packet["unfilled"] = append([]string{}, progress.Unfilled...)
		packet["issues"] = packetIssues(story, deps, "")
		packet["cast"] = packetCast(story)
		packet["environments"] = packetEnvironments(deps.Catalog)
	default:
		return nil, fmt.Errorf("storymaker: a packet needs a known phase, not %q", phase)
	}
	return packet, nil
}

// packetFillExample is the worked example of a fill scene. It names the first cast member of the story and
// leaves the actor and the gesture out when there is no cast, so it never names a member that does not exist.
// It has no cue, because a cue names a narration marker of the scene.
func packetFillExample(cast []CastMember) map[string]any {
	example := map[string]any{"environment": "env.office", "transition": "wipe"}
	beats := []any{map[string]any{"do": "show", "item": "prop.lock", "at": "right", "entry": "pop"}}
	if len(cast) > 0 {
		who := cast[0].ID
		example["actors"] = []any{map[string]any{"cast": who, "at": "left", "pose": "present"}}
		beats = append(beats, map[string]any{"do": "gesture", "who": who, "gesture": "point"})
	}
	example["beats"] = beats
	return example
}

// packetFill adds the fields of one scene's fill loop.
func packetFill(ctx context.Context, packet map[string]any, story *Story, sceneID string, job StudioJob, deps StudioDeps) error {
	index := -1
	for i := range story.Scenes {
		if story.Scenes[i].ID == sceneID {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("storymaker: the scene %q is not in the story", clip(sceneID, 40))
	}
	project, err := deps.Projects.Get(ctx, job.ProjectID)
	if err != nil {
		return err
	}
	sc := &story.Scenes[index]
	packet["scene"] = map[string]any{
		"id":          sc.ID,
		"index":       index + 1,
		"total":       len(story.Scenes),
		"narration":   sc.Narration,
		"words":       WordCount(StripMarkers(sc.Narration)),
		"word_budget": packetWordBudget(project.Template, job.TargetSeconds, len(story.Scenes)),
	}
	if index > 0 {
		packet["previous_environment"] = story.Scenes[index-1].Environment
	}
	packet["cast"] = packetCast(story)
	packet["environments"] = packetEnvironments(deps.Catalog)
	packet["open_issues"] = packetIssues(story, deps, sc.ID)
	packet["example"] = packetFillExample(story.Cast)
	if !job.VoiceAvailable {
		packet["note"] = packetNoVoiceNote
	}
	return nil
}

func packetStory(ctx context.Context, deps StudioDeps, projectID string) (*Story, error) {
	data, _, err := deps.Projects.LoadStory(ctx, projectID)
	if err != nil {
		return nil, err
	}
	story, _, err := DecodeStory(data, deps.Limits)
	if err != nil {
		return nil, err
	}
	ApplyDefaults(&story)
	return &story, nil
}

// packetBudget gives the limits of decision 14. The time left comes from the job's deadline. corrections_left is the
// limit of a fresh scene, and each put_scene and patch_scene answer carries the exact count.
func packetBudget(ctx context.Context, phase string) map[string]any {
	budget := map[string]any{}
	switch phase {
	case PhaseOutline:
		budget["tool_calls"] = 6
	case PhaseCast:
		budget["tool_calls"] = 12
	case PhaseFill:
		budget["tool_calls"] = 10
		budget["corrections_left"] = StudioSceneChargedLimit
	case PhaseCheck:
		budget["tool_calls"] = 20
		budget["corrections_left"] = StudioSceneChargedLimit
	case PhaseRevise:
		budget["tool_calls"] = 30
		budget["corrections_left"] = StudioSceneChargedLimit
	}
	if deadline, ok := ctx.Deadline(); ok {
		budget["seconds_left"] = max(0, int(time.Until(deadline).Seconds()))
	}
	return budget
}

func packetSuggestedScenes(targetSeconds int) int {
	scenes := int(math.Round(float64(targetSeconds) * packetWordsPerSecond / packetWordsPerScene))
	return max(packetMinScenes, min(packetMaxScenes, scenes))
}

// packetWordBudget is 19 for product and concept explainers, else the target words split over the scenes.
func packetWordBudget(template string, targetSeconds, scenes int) int {
	if template == "product_explainer" || template == "concept_explainer" || targetSeconds <= 0 || scenes <= 0 {
		return packetWordsPerScene
	}
	return int(math.Round(float64(targetSeconds) * packetWordsPerSecond / float64(scenes)))
}

func packetCast(story *Story) []map[string]any {
	out := make([]map[string]any, 0, len(story.Cast))
	for _, member := range story.Cast {
		out = append(out, map[string]any{"id": member.ID, "name": member.Name, "character": member.Character})
	}
	return out
}

func packetEnvironments(cat *Catalog) []map[string]any {
	ids := cat.Names("environment")
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"id": id, "spots": cat.SpotNames(id)})
	}
	return out
}

func packetSummary(story *Story) map[string]any {
	scenes := make([]map[string]any, 0, len(story.Scenes))
	total := 0
	for i := range story.Scenes {
		sc := &story.Scenes[i]
		plain := StripMarkers(sc.Narration)
		words := WordCount(plain)
		total += words
		scenes = append(scenes, map[string]any{
			"id":          sc.ID,
			"environment": sc.Environment,
			"narration":   clip(plain, packetNarrationRunes),
			"words":       words,
			"filled":      sc.Kind == "code" || len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0,
		})
	}
	return map[string]any{"title": story.Title, "scenes": scenes, "total_words": total}
}

// packetIssues lists the validation errors and the over_budget lint, for one scene or, with sceneID "", for the story.
// It starts no browser. scene_unfilled stays out: a fill packet is that scene's task, and a revise packet has unfilled.
func packetIssues(story *Story, deps StudioDeps, sceneID string) []Issue {
	all := append(Validate(story, deps.Catalog, deps.Limits), LintForJob(story)...)
	out := []Issue{}
	for _, is := range all {
		if is.Code == CodeSceneUnfilled || (is.Severity != SeverityError && is.Code != CodeOverBudget) {
			continue
		}
		if sceneID == "" || is.SceneID == sceneID {
			out = append(out, is)
		}
	}
	return CapIssues(out, MaxModelIssues)
}
```

- [ ] **Step 2: Run.** This task has no test of its own. This command only compiles the package with its tests.
`GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^$' -count=1`
Expected: `ok  	aurago/internal/storymaker` with `[no tests to run]`.

- [ ] **Step 3: Commit.** Stage `internal/storymaker/studio_packet.go`. Message: `feat(storymaker): build the agent context packets`.

