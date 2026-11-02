## Part A: Go core (`internal/storymaker` and config)

**Scope.** The Go package `internal/storymaker` (story model, strict decoding, validation, defaults, catalog index, code-scene
bundling, project store) and the `storymaker` config section. It has no server code: the HTTP layer (part B), the player (part C),
the catalog (part D) and the app (part E) build on it. Contract: `docs/superpowers/specs/2026-11-01-storymaker-contract.md` (the interface contract of the index's Documents table), sections 2 to 4.

**Order.** A1 (config) stands alone. A2 to A7 are cumulative: each task uses only files of earlier tasks, its tests fail first
(undefined symbols) and pass after its implementation, and the whole package stays green. A8 documents the package and runs the
final check. Every task was replayed in a throw-away copy of the package to prove this.

**Pipeline for callers (part B).** `DecodeStory` (strict; failure is a `*DecodeError` carrying issues, answer 422) then
`ApplyDefaults` (returns `defaults_applied`) then `Validate` (canonicalises names in place, returns issues; cap them with
`CapIssues`) then `CanonicalJSON` then `ProjectStore.SaveStory(ctx, id, bytes, ifMatch)`. `Store.Remove` must behave like
`os.RemoveAll`. After a code-scene save, `CodeSceneIssues(story, status)` gives the `code_error` issues that block a render.

**Test conventions.** Windows checkouts may convert `testdata/scenes/*.js` to CRLF; the tests normalise line endings before
comparing bytes, and the player's Node tests must do the same.

### Task A1: Storymaker config section

**Files:**
- Create: `internal/config/storymaker.go`
- Modify: `internal/config/config_types.go:2195` (add the `Storymaker` field after `VideoStudio`)
- Modify: `internal/config/config.go:572` (default `Enabled = true`) and `internal/config/config.go:1084` (call `NormalizeStorymakerConfig`)
- Modify: `config_template.yaml:2363` (section after `video_studio:`)
- Test: `internal/config/storymaker_test.go`

Line numbers of the three existing files are those of main HEAD `561685cde` (the drafting base `0248bc133` had 2194, 571, 1083 and 2358); the Old/New blocks match both.

Line numbers of the three existing files are those of main HEAD `561685cde` (the drafting base `0248bc133` had 2194, 571, 1083 and 2358); the Old/New blocks match both.

- [ ] **Step 1: Write the failing test**

`internal/config/storymaker_test.go`:

```go
package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadStorymakerYAML(t *testing.T, content string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return Load(path)
}

func TestStorymakerDefaultsOn(t *testing.T) {
	want := StorymakerConfig{Enabled: true, Render: StorymakerRenderConfig{TimeoutSeconds: 3600, MaxDurationSeconds: 900}}
	for name, content := range map[string]string{
		"empty file":   "{}\n",
		"null section": "storymaker:\n",
		"render only":  "storymaker:\n  render:\n    transport: png\n",
	} {
		cfg, err := loadStorymakerYAML(t, content)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := cfg.Storymaker
		wantHere := want
		if name == "render only" {
			wantHere.Render.Transport = "png"
		}
		if got != wantHere {
			t.Fatalf("%s: storymaker = %+v, want %+v", name, got, wantHere)
		}
	}
	cfg, err := loadStorymakerYAML(t, "storymaker:\n  enabled: false\n  render:\n    timeout_seconds: 120\n    max_duration_seconds: 60\n    browser_path: /usr/bin/chromium\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storymaker.Enabled || cfg.Storymaker.Render.TimeoutSeconds != 120 || cfg.Storymaker.Render.MaxDurationSeconds != 60 || cfg.Storymaker.Render.BrowserPath != "/usr/bin/chromium" {
		t.Fatalf("explicit values lost: %+v", cfg.Storymaker)
	}
}

func TestNormalizeStorymakerConfigRanges(t *testing.T) {
	ok := []StorymakerRenderConfig{
		{}, {TimeoutSeconds: 60}, {TimeoutSeconds: 14400}, {MaxDurationSeconds: 10}, {MaxDurationSeconds: 3600},
		{Transport: " WebP "}, {Transport: "raw"}, {Transport: "screenshot"}, {BrowserPath: "/usr/bin/chromium"},
	}
	for _, r := range ok {
		c := StorymakerConfig{Render: r}
		if err := NormalizeStorymakerConfig(&c); err != nil {
			t.Errorf("%+v rejected: %v", r, err)
		}
		if c.Render.TimeoutSeconds == 0 || c.Render.MaxDurationSeconds == 0 {
			t.Errorf("%+v: defaults not applied: %+v", r, c.Render)
		}
	}
	bad := []StorymakerRenderConfig{
		{TimeoutSeconds: -1}, {TimeoutSeconds: 59}, {TimeoutSeconds: 14401},
		{MaxDurationSeconds: -5}, {MaxDurationSeconds: 9}, {MaxDurationSeconds: 3601},
		{Transport: "websocket"}, {BrowserPath: "chromium"}, {BrowserPath: "./chrome"}, {BrowserPath: "/usr/bin/a\nb"},
	}
	for _, r := range bad {
		c := StorymakerConfig{Render: r}
		if NormalizeStorymakerConfig(&c) == nil {
			t.Errorf("%+v accepted", r)
		}
	}
	c := StorymakerConfig{Render: StorymakerRenderConfig{Transport: " WebP ", BrowserPath: " /usr/bin/chromium "}}
	if err := NormalizeStorymakerConfig(&c); err != nil || c.Render.Transport != "webp" || c.Render.BrowserPath != "/usr/bin/chromium" {
		t.Errorf("normalisation: %+v (%v)", c.Render, err)
	}
	if _, err := loadStorymakerYAML(t, "storymaker:\n  render:\n    timeout_seconds: 5\n"); err == nil {
		t.Error("Load accepted a render timeout below 60 seconds")
	}
}

// The shipped template must describe the same defaults the loader applies.
func TestStorymakerTemplateMatchesLoadDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse template: %v", err)
	}
	node, ok := doc["storymaker"]
	if !ok {
		t.Fatal("config_template.yaml has no storymaker: block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		t.Fatal(err)
	}
	var tpl StorymakerConfig
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		t.Fatalf("template block does not decode into StorymakerConfig: %v", err)
	}
	if err := NormalizeStorymakerConfig(&tpl); err != nil {
		t.Fatalf("template values rejected: %v", err)
	}
	cfg, err := loadStorymakerYAML(t, "{}\n")
	if err != nil {
		t.Fatal(err)
	}
	if tpl != cfg.Storymaker {
		t.Fatalf("template %+v differs from loader defaults %+v", tpl, cfg.Storymaker)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/config/ -run 'Storymaker' -count=1`
Expected: FAIL (build failed: `undefined: StorymakerConfig`)

- [ ] **Step 3: Implement**

Before editing `Load` (GitNexus, CLAUDE.md): `node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact "Load" --direction upstream --repo .` was measured at base
`0248bc133`: 73 impacted symbols, risk MEDIUM, no `riskNote`. The change is additive (one default, one validation call, no signature
change). Re-run it if the index was rebuilt.

`internal/config/storymaker.go`:

```go
package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// StorymakerConfig configures the Storymaker desktop app (explainer videos from a story
// document). Enabled defaults to true in the loader; the app shows capability states for
// export when FFmpeg or a browser is missing.
type StorymakerConfig struct {
	Enabled bool                   `yaml:"enabled" json:"enabled"`
	Render  StorymakerRenderConfig `yaml:"render" json:"render"`
}

// StorymakerRenderConfig bounds the server-side render that turns a story into an MP4.
type StorymakerRenderConfig struct {
	// TimeoutSeconds is the minimum render timeout; the render job raises it for long videos.
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
	// MaxDurationSeconds is the longest video a render accepts.
	MaxDurationSeconds int `yaml:"max_duration_seconds" json:"max_duration_seconds"`
	// BrowserPath is an optional absolute path of the Chrome/Chromium used for rendering.
	BrowserPath string `yaml:"browser_path" json:"browser_path"`
	// Transport selects how frames travel from the render page: "" (automatic), png, webp, raw
	// or screenshot.
	Transport string `yaml:"transport" json:"transport"`
}

const (
	storymakerDefaultTimeoutSeconds     = 3600
	storymakerDefaultMaxDurationSeconds = 900
)

// NormalizeStorymakerConfig applies defaults for unset (zero) values and rejects values outside
// their range: timeout 60 to 14400 seconds, maximum duration 10 to 3600 seconds, a browser path
// that is absolute, and a known frame transport. It does not change Enabled.
func NormalizeStorymakerConfig(c *StorymakerConfig) error {
	r := &c.Render
	if r.TimeoutSeconds == 0 {
		r.TimeoutSeconds = storymakerDefaultTimeoutSeconds
	}
	if r.MaxDurationSeconds == 0 {
		r.MaxDurationSeconds = storymakerDefaultMaxDurationSeconds
	}
	if r.TimeoutSeconds < 60 || r.TimeoutSeconds > 14400 {
		return fmt.Errorf("storymaker: render.timeout_seconds must be 60–14400")
	}
	if r.MaxDurationSeconds < 10 || r.MaxDurationSeconds > 3600 {
		return fmt.Errorf("storymaker: render.max_duration_seconds must be 10–3600")
	}
	r.BrowserPath = strings.TrimSpace(r.BrowserPath)
	if r.BrowserPath != "" {
		// A Linux path ("/usr/bin/chromium") stays valid when the configuration is checked on Windows.
		abs := filepath.IsAbs(r.BrowserPath) || strings.HasPrefix(r.BrowserPath, "/")
		if !abs || len(r.BrowserPath) > 1024 || strings.ContainsAny(r.BrowserPath, "\r\n\x00") {
			return fmt.Errorf("storymaker: render.browser_path must be an absolute path to a Chrome or Chromium executable")
		}
	}
	r.Transport = strings.ToLower(strings.TrimSpace(r.Transport))
	switch r.Transport {
	case "", "png", "webp", "raw", "screenshot":
	default:
		return fmt.Errorf("storymaker: render.transport must be empty, png, webp, raw or screenshot")
	}
	return nil
}
```

In `internal/config/config_types.go` (line 2195), add the field after `VideoStudio`:

Old:

```go
	VideoStudio       VideoStudioConfig       `yaml:"video_studio"`
	VirtualComputers  VirtualComputersConfig  `yaml:"virtual_computers"`
```

New:

```go
	VideoStudio       VideoStudioConfig       `yaml:"video_studio"`
	Storymaker        StorymakerConfig        `yaml:"storymaker" json:"storymaker"`
	VirtualComputers  VirtualComputersConfig  `yaml:"virtual_computers"`
```

In `internal/config/config.go`, set the default next to Retro Games (line 572):

Old:

```go
	cfg.RetroGames.Enabled = true
	cfg.RetroGames.AgentAccess = true

	// Space Agent defaults: disabled by default, managed Docker sidecar when enabled.
```

New:

```go
	cfg.RetroGames.Enabled = true
	cfg.RetroGames.AgentAccess = true

	// Storymaker is on by default: the app is visible and its export shows capability states
	// (FFmpeg, browser) instead of hiding the editor.
	cfg.Storymaker.Enabled = true

	// Space Agent defaults: disabled by default, managed Docker sidecar when enabled.
```

In `internal/config/config.go`, normalise next to Video Studio (line 1084):

Old:

```go
	if err := NormalizeVideoStudioConfig(&cfg.VideoStudio); err != nil {
		return nil, err
	}
	if err := NormalizeNewspaperConfig(&cfg.Newspaper); err != nil {
```

New:

```go
	if err := NormalizeVideoStudioConfig(&cfg.VideoStudio); err != nil {
		return nil, err
	}
	if err := NormalizeStorymakerConfig(&cfg.Storymaker); err != nil {
		return nil, err
	}
	if err := NormalizeNewspaperConfig(&cfg.Newspaper); err != nil {
```

In `config_template.yaml`, add the section after the `video_studio:` block (line 2363; the unchanged lines are context):

Old:

```yaml
    hardware_encoding: auto
    hardware_device: ""

journal:
```

New:

```yaml
    hardware_encoding: auto
    hardware_device: ""

# Desktop explainer-video maker. Export renders frames in a headless Chrome/Chromium and encodes
# them with FFmpeg (video_studio.ffmpeg_path, or PATH). The editor and preview work without both.
storymaker:
    enabled: true
    render:
        timeout_seconds: 3600       # minimum render timeout (60-14400); long videos raise it
        max_duration_seconds: 900   # longest video a render accepts (10-3600)
        browser_path: ""            # optional absolute path of Chrome/Chromium; empty = system browser, then pinned download
        transport: ""               # frame transport: empty = automatic, or png, webp, raw, screenshot

journal:
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/... -count=1`
Expected: PASS (`ok  aurago/internal/config`). The new tests are `TestStorymakerDefaultsOn`, `TestNormalizeStorymakerConfigRanges` and
`TestStorymakerTemplateMatchesLoadDefaults`. Also run `go test ./ui/ -run 'TestConfigDerivedValidationRulesCoverTemplate' -count=1`:
it checks every scalar of the template (including the new keys) against the derived validation rules.

- [ ] **Step 5: Commit**

Run `node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo .` first (CLAUDE.md); expected: only the five files below.

```bash
git add internal/config/storymaker.go internal/config/storymaker_test.go internal/config/config_types.go internal/config/config.go config_template.yaml
git commit -m "feat(storymaker): config section with render limits" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A2: Story model and strict decoding

**Files:**
- Create: `internal/storymaker/model.go`
- Create: `internal/storymaker/decode.go`
- Create: `internal/storymaker/testdata/stories/minimal.json`
- Create: `internal/storymaker/testdata/stories/firewall.json`
- Test: `internal/storymaker/decode_test.go`

- [ ] **Step 1: Write the failing test**

`internal/storymaker/testdata/stories/minimal.json`:

```json
{"schema":"storymaker/1","title":"Minimal","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"voice":null,"music":null,"cast":[],"scenes":[{"id":"s1","narration":"Hallo Welt."}]}
```

`internal/storymaker/testdata/stories/firewall.json`:

```json
{"schema":"storymaker/1","title":"Was ist eine Firewall?","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"voice":null,"music":null,
 "cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],
 "scenes":[
  {"id":"s1","environment":"env.office","transition":"fade","narration":"Anna arbeitet im Homeoffice. {1}Plötzlich klopft ein Unbekannter an.","speaker":"narrator",
   "actors":[{"cast":"anna","at":"desk","pose":"typing"}],
   "beats":[{"cue":1,"do":"emote","who":"anna","expression":"surprised"},{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]},
  {"id":"s2","environment":"env.abstract-tech","narration":"Eine Firewall prüft jedes Paket.","actors":[{"cast":"anna","at":"left"}],
   "beats":[{"do":"walk","who":"anna","to":"center"},{"do":"gesture","who":"anna","gesture":"point","with":"prev"},{"do":"show","item":"prop.shield","at":"right","entry":"fade"},{"do":"camera","op":"zoom","amount":1.2}]},
  {"id":"s3","kind":"code","narration":"So sieht das in Zahlen aus."}]}
```

`internal/storymaker/decode_test.go`:

```go
package storymaker

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func mustDecode(t *testing.T, data string) Story {
	t.Helper()
	s, issues, err := DecodeStory([]byte(data), DefaultLimits())
	if err != nil {
		t.Fatalf("DecodeStory: %v (%+v)", err, issues)
	}
	if len(issues) != 0 {
		t.Fatalf("issues on success: %+v", issues)
	}
	return s
}

func decodeFail(t *testing.T, data string) []Issue {
	t.Helper()
	_, issues, err := DecodeStory([]byte(data), DefaultLimits())
	if err == nil {
		t.Fatalf("expected an error for %.80q", data)
	}
	var de *DecodeError
	if !errors.As(err, &de) || len(issues) == 0 || len(de.Issues) != len(issues) {
		t.Fatalf("error %v must be a *DecodeError with issues, got %+v", err, issues)
	}
	return issues
}

func TestDecodeFixtures(t *testing.T) {
	for _, name := range []string{"minimal", "firewall"} {
		data, err := os.ReadFile("testdata/stories/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		s, issues, err := DecodeStory(data, DefaultLimits())
		if err != nil || len(issues) != 0 {
			t.Fatalf("%s: %v %+v", name, err, issues)
		}
		if s.Schema != Schema || len(s.Scenes) == 0 {
			t.Fatalf("%s: %+v", name, s)
		}
	}
	data, _ := os.ReadFile("testdata/stories/firewall.json")
	s := mustDecode(t, string(data))
	if s.Title != "Was ist eine Firewall?" || len(s.Cast) != 1 || len(s.Scenes) != 3 {
		t.Fatalf("firewall: %+v", s)
	}
	a := s.Scenes[0].Actors[0]
	if a.At == nil || a.At.Spot != "desk" || a.Pose != "typing" {
		t.Fatalf("actor: %+v", a)
	}
	if b := s.Scenes[0].Beats[0]; b.Cue != 1 || b.Do != "emote" || b.Expression != "surprised" {
		t.Fatalf("beat: %+v", b)
	}
	if !strings.Contains(s.Scenes[0].Narration, "{1}Plötzlich") || s.Scenes[2].Kind != "code" {
		t.Fatalf("scenes: %+v", s.Scenes)
	}
}

func TestDecodeUnknownFieldsHaveHintsAndPaths(t *testing.T) {
	story := `{"schema":"storymaker/1","title":"T","scenes":[
	  {"id":"s1","text":"Hallo","actors":[{"character":"anna","position":"left"}],
	   "beats":[{"type":"walk","actor":"anna","destination":"door"},{"do":"wait","seconds":1,"pause":2}]}]}`
	issues := decodeFail(t, story)
	byPath := map[string]Issue{}
	for _, is := range issues {
		if is.Code != CodeUnknownField || is.Severity != SeverityError {
			t.Fatalf("issue %+v", is)
		}
		byPath[is.Path] = is
	}
	want := map[string]string{
		"/scenes/0/text":                `"narration"`,
		"/scenes/0/actors/0/character":  `"cast"`,
		"/scenes/0/actors/0/position":   `"at"`,
		"/scenes/0/beats/0/type":        `"do"`,
		"/scenes/0/beats/0/actor":       `"who"`,
		"/scenes/0/beats/0/destination": `"to"`,
		"/scenes/0/beats/1/pause":       "one of the fields of a beat",
	}
	if len(byPath) != len(want) {
		t.Fatalf("want %d issues, got %d: %+v", len(want), len(byPath), issues)
	}
	for path, hint := range want {
		is, ok := byPath[path]
		if !ok {
			t.Errorf("no issue at %s", path)
			continue
		}
		if !strings.Contains(is.Fix, hint) {
			t.Errorf("%s: fix %q lacks %q", path, is.Fix, hint)
		}
	}
	// Document order: scene 0 fields before beats, beat 0 before beat 1.
	if issues[len(issues)-1].Path != "/scenes/0/beats/1/pause" {
		t.Errorf("issues not in document order: %+v", issues)
	}
}

func TestDecodeKeyMatchingIsExact(t *testing.T) {
	// encoding/json would accept "Title" for "title"; the format does not.
	issues := decodeFail(t, `{"schema":"storymaker/1","Title":"x","scenes":[]}`)
	if issues[0].Code != CodeUnknownField || issues[0].Path != "/Title" {
		t.Fatalf("%+v", issues)
	}
}

func TestDecodeFailureCodes(t *testing.T) {
	cases := []struct {
		name, data, code, path, msg string
	}{
		{"empty", "  ", CodeInvalidJSON, "", "empty"},
		{"truncated", `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1"`, CodeInvalidJSON, "", ""},
		{"syntax", "{\n  \"schema\": \"storymaker/1\",\n  \"title\" \"x\"\n}", CodeInvalidJSON, "", "line 3"},
		{"two objects", `{"schema":"storymaker/1"} {"a":1}`, CodeInvalidJSON, "", "more data"},
		{"array", `[]`, CodeInvalidJSON, "", "JSON object"},
		{"no schema", `{"title":"x"}`, CodeSchemaMismatch, "/schema", "must be"},
		{"other schema", `{"schema":"storymaker/2","title":"x"}`, CodeSchemaMismatch, "/schema", "storymaker/2"},
		{"fps as string", `{"schema":"storymaker/1","title":"x","format":{"aspect":"16:9","fps":"30"},"scenes":[]}`, CodeInvalidJSON, "/format/fps", "expected a number"},
		{"scenes as object", `{"schema":"storymaker/1","title":"x","scenes":{}}`, CodeInvalidJSON, "/scenes", "an array"},
		{"scene id as number", `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1"},{"id":5}]}`, CodeInvalidJSON, "/scenes/1/id", "a string"},
		{"nested beat count", `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1","beats":[{"do":"wait"},{"do":"gesture","count":"two"}]}]}`, CodeInvalidJSON, "/scenes/0/beats/1/count", "a number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := decodeFail(t, tc.data)
			is := issues[0]
			if is.Code != tc.code || is.Path != tc.path || is.Severity != SeverityError || !strings.Contains(is.Message, tc.msg) {
				t.Fatalf("got %+v, want code %s path %q msg containing %q", is, tc.code, tc.path, tc.msg)
			}
		})
	}
}

func TestDecodeTooLarge(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxStoryBytes = 50
	_, issues, err := DecodeStory([]byte(`{"schema":"storymaker/1","title":"`+strings.Repeat("x", 100)+`","scenes":[]}`), lim)
	if err == nil || issues[0].Code != CodeTooLarge {
		t.Fatalf("%v %+v", err, issues)
	}
}

func TestDecodeStripsBOM(t *testing.T) {
	mustDecode(t, "\xef\xbb\xbf"+`{"schema":"storymaker/1","title":"x","scenes":[]}`)
}

func TestPolymorphicPositionAndDuration(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"x","scenes":[
	  {"id":"s1","duration":"auto","actors":[{"cast":"a","at":"desk"},{"cast":"b","at":{"x":12.5,"y":80}},{"cast":"c","at":{"x":1}},{"cast":"d","at":7}]},
	  {"id":"s2","duration":6.5},{"id":"s3","duration":"soon"},{"id":"s4"}]}`)
	ac := s.Scenes[0].Actors
	if ac[0].At.Spot != "desk" || ac[0].At.Point {
		t.Errorf("spot: %+v", ac[0].At)
	}
	if !ac[1].At.Point || ac[1].At.X != 12.5 || ac[1].At.Y != 80 {
		t.Errorf("point: %+v", ac[1].At)
	}
	if len(ac[2].At.Invalid) == 0 || len(ac[3].At.Invalid) == 0 {
		t.Errorf("invalid shapes must be kept: %+v %+v", ac[2].At, ac[3].At)
	}
	if !s.Scenes[0].Duration.Auto || s.Scenes[1].Duration.Seconds != 6.5 || len(s.Scenes[2].Duration.Invalid) == 0 || s.Scenes[3].Duration != nil {
		t.Errorf("durations: %+v %+v %+v %+v", s.Scenes[0].Duration, s.Scenes[1].Duration, s.Scenes[2].Duration, s.Scenes[3].Duration)
	}
	out, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"at": "desk"`, `"duration": "auto"`, `"duration": 6.5`, `"duration": "soon"`, `"at": 7`, `"x": 12.5`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("canonical JSON lacks %s:\n%s", want, out)
		}
	}
	// What was saved is read back unchanged.
	again := mustDecode(t, string(out))
	if out2, _ := CanonicalJSON(again); !bytes.Equal(out, out2) {
		t.Errorf("round trip changed the story:\n%s\n%s", out, out2)
	}
	if !reflect.DeepEqual(s.Scenes, again.Scenes) {
		t.Errorf("round trip changed the scenes:\n%+v\n%+v", s.Scenes, again.Scenes)
	}
}

func TestStyleCaptionsDefaultOnWhenOmitted(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"x","style":{"palette":"calm"},"scenes":[]}`)
	if !s.Style.Captions || s.Style.Palette != "calm" {
		t.Fatalf("%+v", s.Style)
	}
	s = mustDecode(t, `{"schema":"storymaker/1","title":"x","style":{"captions":false},"scenes":[]}`)
	if s.Style.Captions {
		t.Fatalf("explicit false lost: %+v", s.Style)
	}
	s = mustDecode(t, `{"schema":"storymaker/1","title":"x","scenes":[]}`)
	if s.Style != (Style{}) {
		t.Fatalf("absent style must stay zero so ApplyDefaults can fill it: %+v", s.Style)
	}
}

func TestCanonicalJSONIsStableAndRoundTrips(t *testing.T) {
	for _, name := range []string{"minimal", "firewall"} {
		data, _ := os.ReadFile("testdata/stories/" + name + ".json")
		s := mustDecode(t, string(data))
		a, err := CanonicalJSON(s)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := CanonicalJSON(s)
		if !bytes.Equal(a, b) {
			t.Fatalf("%s: not stable", name)
		}
		if !bytes.HasSuffix(a, []byte("}\n")) || !strings.HasPrefix(string(a), "{\n  \"schema\": \"storymaker/1\",\n  \"title\":") {
			t.Fatalf("%s: unexpected layout:\n%.200s", name, a)
		}
		again := mustDecode(t, string(a))
		if !reflect.DeepEqual(s, again) {
			t.Fatalf("%s: round trip changed the story", name)
		}
		// Same JSON value as the fixture.
		var x, y any
		_ = json.Unmarshal(data, &x)
		_ = json.Unmarshal(a, &y)
		if !reflect.DeepEqual(x, y) {
			t.Fatalf("%s: canonical JSON differs from the fixture:\n%s", name, a)
		}
	}
	// No HTML escaping, and empty collections are [] not null.
	out, _ := CanonicalJSON(Story{Schema: Schema, Title: "A & B <c>"})
	if !strings.Contains(string(out), `"A & B <c>"`) || !strings.Contains(string(out), `"cast": []`) || !strings.Contains(string(out), `"scenes": []`) {
		t.Fatalf("%s", out)
	}
}

func TestPointerAtOffsetEscapesAndIndexes(t *testing.T) {
	data := `{"a/b":{"c~d":[0,{"e":"x"}]}}`
	off := int64(strings.Index(data, `"x"`) + 3)
	if got := pointerAtOffset([]byte(data), off); got != "/a~1b/c~0d/1/e" {
		t.Fatalf("pointer = %q", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/ -run 'TestDecode|TestPolymorphic|TestStyle|TestCanonical|TestPointer' -count=1`
Expected: FAIL (build failed: `undefined: DecodeStory`)

- [ ] **Step 3: Implement**

`internal/storymaker/model.go`:

```go
// Package storymaker is the Go core of the Storymaker desktop app: the story document
// (schema storymaker/1), its validation and defaults, the catalog index, code-scene
// bundling and the project storage layout. It has no server dependency.
package storymaker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Schema is the only story schema string this package accepts.
const Schema = "storymaker/1"

// Issue severities.
const (
	SeverityError   = "error"   // blocks render
	SeverityWarning = "warning" // does not block render
	SeverityMinor   = "minor"   // not_supported_yet and canonicalisation notes
)

// Issue codes of phase 1. The desktop app localises them as storymaker.issue.<code>.
const (
	CodeInvalidJSON      = "invalid_json"
	CodeUnknownField     = "unknown_field"
	CodeSchemaMismatch   = "schema_mismatch"
	CodeTooLarge         = "too_large"
	CodeLimitExceeded    = "limit_exceeded"
	CodeMissingField     = "missing_field"
	CodeInvalidValue     = "invalid_value"
	CodeDuplicateID      = "duplicate_id"
	CodeUnknownName      = "unknown_name"
	CodeUnknownAsset     = "unknown_asset"
	CodeUnknownSpot      = "unknown_spot"
	CodeUnknownCast      = "unknown_cast"
	CodeUnknownCue       = "unknown_cue"
	CodeCanonicalised    = "canonicalised"
	CodeNotSupportedYet  = "not_supported_yet"
	CodeNarrationMissing = "narration_missing"
	CodeCodeError        = "code_error"
	CodeDeadline         = "deadline"
	CodeLayoutUnverified = "layout_unverified"
)

// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
type Issue struct {
	Severity  string   `json:"severity"`
	SceneID   string   `json:"scene_id,omitempty"`
	Path      string   `json:"path"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	AtSeconds *float64 `json:"at_seconds,omitempty"`
	Fix       string   `json:"fix,omitempty"`
}

// DefaultApplied records one default that ApplyDefaults filled in.
type DefaultApplied struct {
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// Limits are the story size limits. The zero value of a field means "no limit" for
// DecodeStory and BundleCodeScene; Validate replaces an all-zero Limits by DefaultLimits.
type Limits struct {
	MaxScenes          int
	MaxItemsPerScene   int // actors plus props
	MaxBeatsPerScene   int
	MaxNarrationChars  int // per scene, in characters
	MaxStoryBytes      int
	MaxCodeBytes       int
	MaxDurationSeconds float64
}

// DefaultLimits returns the limits of the design spec.
func DefaultLimits() Limits {
	return Limits{
		MaxScenes:          200,
		MaxItemsPerScene:   40,
		MaxBeatsPerScene:   60,
		MaxNarrationChars:  1500,
		MaxStoryBytes:      1 << 20,
		MaxCodeBytes:       64 << 10,
		MaxDurationSeconds: 15 * 60,
	}
}

// Story is the storymaker/1 document. Field order is the key order of story.json.
type Story struct {
	Schema string       `json:"schema"`
	Title  string       `json:"title"`
	Format Format       `json:"format"`
	Style  Style        `json:"style"`
	Voice  *Voice       `json:"voice"`
	Music  *Music       `json:"music"`
	Cast   []CastMember `json:"cast"`
	Scenes []Scene      `json:"scenes"`
}

// Format is the canvas: aspect 16:9 (1920x1080), 9:16 (1080x1920) or 1:1 (1080x1080).
type Format struct {
	Aspect string `json:"aspect"`
	FPS    int    `json:"fps"`
}

// Style names a catalog palette and font and switches captions.
type Style struct {
	Palette  string `json:"palette"`
	Font     string `json:"font"`
	Captions bool   `json:"captions"`
}

// UnmarshalJSON defaults captions to true when a style object omits the key.
func (s *Style) UnmarshalJSON(b []byte) error {
	if string(bytes.TrimSpace(b)) == "null" {
		return nil
	}
	type plain Style
	p := plain{Captions: true}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}
	*s = Style(p)
	return nil
}

// Voice is a narration voice. Phase 1 accepts and validates its shape only.
type Voice struct {
	Provider string `json:"provider"`
	Voice    string `json:"voice"`
	Language string `json:"language"`
}

// Music is the background music setting.
type Music struct {
	Mode   string   `json:"mode"` // none | generated | file
	Mood   string   `json:"mood,omitempty"`
	Prompt string   `json:"prompt,omitempty"`
	File   string   `json:"file,omitempty"`
	Track  string   `json:"track,omitempty"`
	Volume *float64 `json:"volume,omitempty"`
}

// CastMember is a recurring character of the story.
type CastMember struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Character string `json:"character"`
	Voice     *Voice `json:"voice,omitempty"`
}

// Scene is one scene. Kind "code" scenes keep their source in scenes/<id>.js.
type Scene struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind,omitempty"`
	Environment string    `json:"environment,omitempty"`
	Variant     string    `json:"variant,omitempty"`
	Narration   string    `json:"narration,omitempty"`
	Silent      bool      `json:"silent,omitempty"`
	Speaker     string    `json:"speaker,omitempty"`
	Actors      []Actor   `json:"actors,omitempty"`
	Props       []Prop    `json:"props,omitempty"`
	Beats       []Beat    `json:"beats,omitempty"`
	Transition  string    `json:"transition,omitempty"`
	Duration    *Duration `json:"duration,omitempty"`
	Camera      *Camera   `json:"camera,omitempty"`
}

// Camera is the starting framing of a scene.
type Camera struct {
	Framing string `json:"framing"` // wide | medium | close
	Target  string `json:"target,omitempty"`
}

// Actor places a cast member in a scene.
type Actor struct {
	Cast       string    `json:"cast"`
	At         *Position `json:"at,omitempty"`
	Pose       string    `json:"pose,omitempty"`
	Facing     string    `json:"facing,omitempty"` // left | right | auto
	Expression string    `json:"expression,omitempty"`
	Scale      float64   `json:"scale,omitempty"` // 0.5 to 2, 0 = unset (1)
}

// Prop places an item in a scene.
type Prop struct {
	ID    string    `json:"id,omitempty"`
	Item  string    `json:"item"`
	At    *Position `json:"at,omitempty"`
	Layer string    `json:"layer,omitempty"` // back | front
	Scale float64   `json:"scale,omitempty"` // 0.5 to 2, 0 = unset (1)
}

// ChartData is the data of a chart.
type ChartData struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
}

// Beat is one timed action of a storyboard scene. Per-kind required fields are checked by
// Validate. Field order is the key order of story.json.
type Beat struct {
	Do         string     `json:"do"`
	Who        string     `json:"who,omitempty"`
	Target     string     `json:"target,omitempty"`
	From       *Position  `json:"from,omitempty"`
	To         *Position  `json:"to,omitempty"`
	Pose       string     `json:"pose,omitempty"`
	Gesture    string     `json:"gesture,omitempty"`
	Count      int        `json:"count,omitempty"`
	Expression string     `json:"expression,omitempty"`
	Text       string     `json:"text,omitempty"`
	Style      string     `json:"style,omitempty"`
	Item       string     `json:"item,omitempty"`
	Icon       string     `json:"icon,omitempty"`
	Chart      string     `json:"chart,omitempty"`
	Data       *ChartData `json:"data,omitempty"`
	At         *Position  `json:"at,omitempty"`
	Entry      string     `json:"entry,omitempty"`
	ID         string     `json:"id,omitempty"`
	Op         string     `json:"op,omitempty"`
	Amount     float64    `json:"amount,omitempty"`
	Sound      string     `json:"sound,omitempty"`
	Seconds    float64    `json:"seconds,omitempty"`
	With       string     `json:"with,omitempty"` // "prev"
	Cue        int        `json:"cue,omitempty"`
	Delay      float64    `json:"delay,omitempty"`
	Duration   float64    `json:"duration,omitempty"`
	Ease       string     `json:"ease,omitempty"`
}

// Position is either a spot name of the scene's environment or a point {x, y} in percent
// of the safe area (y is the feet line). A value of a different JSON shape is kept as
// Invalid so that Validate can report it at the right path and the story can be saved
// unchanged.
type Position struct {
	Spot    string
	X, Y    float64
	Point   bool
	Invalid json.RawMessage
}

// SpotAt returns a position naming a spot.
func SpotAt(name string) *Position { return &Position{Spot: name} }

// PointAt returns a position at x, y percent.
func PointAt(x, y float64) *Position { return &Position{Point: true, X: x, Y: y} }

// MarshalJSON writes the spot name or {"x":..,"y":..}.
func (p Position) MarshalJSON() ([]byte, error) {
	switch {
	case len(p.Invalid) > 0:
		return p.Invalid, nil
	case p.Point:
		return json.Marshal(struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}{p.X, p.Y})
	default:
		return json.Marshal(p.Spot)
	}
}

// UnmarshalJSON reads a spot name or an {x, y} object.
func (p *Position) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*p = Position{}
	switch {
	case len(b) == 0 || string(b) == "null":
		return nil
	case b[0] == '"':
		if err := json.Unmarshal(b, &p.Spot); err != nil {
			p.Spot = ""
			p.Invalid = compactRaw(b)
		}
	case b[0] == '{':
		var o struct {
			X *float64 `json:"x"`
			Y *float64 `json:"y"`
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&o); err != nil || o.X == nil || o.Y == nil {
			p.Invalid = compactRaw(b)
			return nil
		}
		p.Point, p.X, p.Y = true, *o.X, *o.Y
	default:
		p.Invalid = compactRaw(b)
	}
	return nil
}

// IsZero reports whether the position is unset.
func (p *Position) IsZero() bool {
	return p == nil || (!p.Point && p.Spot == "" && len(p.Invalid) == 0)
}

// String is the spot name, or "x,y" for a point. It is used in messages.
func (p *Position) String() string {
	switch {
	case p == nil:
		return ""
	case len(p.Invalid) > 0:
		return string(p.Invalid)
	case p.Point:
		return fmt.Sprintf("%g,%g", p.X, p.Y)
	}
	return p.Spot
}

// Duration is a scene length: "auto" or a number of seconds.
type Duration struct {
	Auto    bool
	Seconds float64
	Invalid json.RawMessage
}

// AutoDuration is the default scene duration.
func AutoDuration() *Duration { return &Duration{Auto: true} }

// SecondsDuration is an explicit scene duration.
func SecondsDuration(s float64) *Duration { return &Duration{Seconds: s} }

// MarshalJSON writes "auto" or the number.
func (d Duration) MarshalJSON() ([]byte, error) {
	switch {
	case len(d.Invalid) > 0:
		return d.Invalid, nil
	case d.Auto:
		return []byte(`"auto"`), nil
	}
	return json.Marshal(d.Seconds)
}

// UnmarshalJSON reads "auto" or a number.
func (d *Duration) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*d = Duration{}
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if json.Unmarshal(b, &s) == nil && strings.EqualFold(strings.TrimSpace(s), "auto") {
			d.Auto = true
			return nil
		}
		d.Invalid = compactRaw(b)
		return nil
	}
	if err := json.Unmarshal(b, &d.Seconds); err != nil {
		d.Seconds = 0
		d.Invalid = compactRaw(b)
	}
	return nil
}

// CanonicalJSON is the stable story.json form: two-space indentation, struct key order,
// no HTML escaping and a trailing newline. Empty cast and scenes are written as [].
func CanonicalJSON(s Story) ([]byte, error) {
	if s.Cast == nil {
		s.Cast = []CastMember{}
	}
	if s.Scenes == nil {
		s.Scenes = []Scene{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, fmt.Errorf("storymaker: encode story: %w", err)
	}
	return buf.Bytes(), nil
}

// compactRaw removes insignificant whitespace, so an invalid value reads back the same from
// the canonical (indented) file.
func compactRaw(b []byte) json.RawMessage {
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return append(json.RawMessage(nil), b...)
	}
	return buf.Bytes()
}

// validSlug reports whether s matches ^[a-z0-9][a-z0-9-]{0,47}$.
func validSlug(s string) bool {
	if s == "" || len(s) > 48 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			continue
		}
		if c == '-' && i > 0 {
			continue
		}
		return false
	}
	return true
}

// Closed name sets of the story format. The validator knows all of them from phase 1; the
// catalog provides a subset until the catalog is complete.
var (
	PoseNames       = []string{"stand", "sit", "typing", "point-left", "point-right", "wave", "arms-crossed", "hands-on-hips", "shrug", "think", "present", "thumbs-up", "celebrate", "facepalm", "phone", "hold", "lean"}
	GestureNames    = []string{"point", "wave", "shrug", "think", "present", "thumbs-up", "celebrate", "facepalm", "count"}
	ExpressionNames = []string{"neutral", "happy", "laugh", "sad", "angry", "surprised", "confused", "thinking", "worried", "determined"}
	MouthNames      = []string{"rest", "ai", "e", "o", "u", "mbp", "fv"}
	TransitionNames = []string{"cut", "fade", "slide-left", "slide-right", "slide-up", "slide-down", "wipe", "zoom", "iris", "whip"}
	EaseNames       = []string{"linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out"}
	BeatKinds       = []string{"enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "say", "show", "hide", "highlight", "camera", "chart", "sfx", "wait"}
)

// Phase 1 plays these beats (show only with item or text) and transitions; everything else
// is accepted, ignored by the player and reported as a minor not_supported_yet issue.
var (
	phase1Beats       = nameSet("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "show", "hide", "camera", "wait")
	phase1Transitions = nameSet("cut", "fade", "slide-left", "wipe")
)

func nameSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

func containsName(list []string, s string) bool {
	for _, n := range list {
		if n == s {
			return true
		}
	}
	return false
}
```

`internal/storymaker/decode.go`:

```go
package storymaker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// DecodeError is returned by DecodeStory. Issues always holds at least one entry, so an
// HTTP layer can answer 422 with the issues.
type DecodeError struct {
	Code    string
	Message string
	Issues  []Issue
}

func (e *DecodeError) Error() string { return "storymaker: " + e.Message }

func decodeFailure(issues ...Issue) (Story, []Issue, error) {
	return Story{}, issues, &DecodeError{Code: issues[0].Code, Message: issues[0].Message, Issues: issues}
}

// maxDecodeIssues bounds the unknown-field findings of one decode.
const maxDecodeIssues = 20

// DecodeStory parses a story document strictly: one JSON object, the storymaker/1 schema,
// no unknown fields (each is reported with a hint for common mistakes) and the size limit.
// Findings that need the catalog or cross references are Validate's job; a successful
// decode returns no issues. A failure returns a *DecodeError and its issues.
func DecodeStory(data []byte, lim Limits) (Story, []Issue, error) {
	if lim.MaxStoryBytes > 0 && len(data) > lim.MaxStoryBytes {
		return decodeFailure(Issue{
			Severity: SeverityError, Path: "", Code: CodeTooLarge,
			Message: fmt.Sprintf("The story is %d bytes; the limit is %d bytes.", len(data), lim.MaxStoryBytes),
			Fix:     "Remove scenes or shorten narration, or move long animation into code scenes.",
		})
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(bytes.TrimSpace(data)) == 0 {
		return decodeFailure(syntaxIssue("The story is empty.", "Send the complete story JSON object."))
	}

	var tree any
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&tree); err != nil {
		return decodeFailure(jsonErrorIssue(data, err))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return decodeFailure(syntaxIssue("There is more data after the story object.", "Send exactly one JSON object."))
	}
	root, ok := tree.(map[string]any)
	if !ok {
		return decodeFailure(syntaxIssue("The story must be a JSON object.", `Start with {"schema":"storymaker/1","title":"...","scenes":[...]}.`))
	}
	if sch, _ := root["schema"].(string); sch != Schema {
		msg := fmt.Sprintf("The schema must be %q.", Schema)
		if sch != "" {
			msg = fmt.Sprintf("The schema %q is not supported; this version reads %q.", sch, Schema)
		}
		return decodeFailure(Issue{
			Severity: SeverityError, Path: "/schema", Code: CodeSchemaMismatch, Message: msg,
			Fix: `Set "schema": "storymaker/1".`,
		})
	}

	var hits []unknownHit
	collectUnknown(root, reflect.TypeOf(Story{}), "", &hits)
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
		return decodeFailure(issues...)
	}

	var s Story
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&s); err != nil {
		return decodeFailure(jsonErrorIssue(data, err))
	}
	return s, nil, nil
}

func syntaxIssue(msg, fix string) Issue {
	return Issue{Severity: SeverityError, Path: "", Code: CodeInvalidJSON, Message: msg, Fix: fix}
}

// jsonErrorIssue maps an encoding/json error to an issue with the best location.
func jsonErrorIssue(data []byte, err error) Issue {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syn):
		line, col := lineColumn(data, syn.Offset)
		return syntaxIssue(fmt.Sprintf("Invalid JSON at line %d, column %d: %s.", line, col, syn.Error()),
			"Fix the syntax (quotes, commas, brackets) and send the complete story.")
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return syntaxIssue("The JSON ends too early.", "Close every string, array and object and send the complete story.")
	case errors.As(err, &typ):
		path := pointerAtOffset(data, typ.Offset)
		if path == "" && typ.Field != "" {
			path = "/" + strings.ReplaceAll(typ.Field, ".", "/")
		}
		return Issue{
			Severity: SeverityError, Path: path, Code: CodeInvalidJSON,
			Message: fmt.Sprintf("Wrong type at %s: expected %s, got %s.", pathLabel(path), jsonTypeName(typ.Type), typ.Value),
			Fix:     "Use the value type of the story format, for example a number without quotes for fps.",
		}
	}
	return syntaxIssue("The story could not be read: "+err.Error()+".", "Check the story against the storymaker/1 format.")
}

func pathLabel(p string) string {
	if p == "" {
		return "the story root"
	}
	return p
}

func jsonTypeName(t reflect.Type) string {
	if t == nil {
		return "another type"
	}
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "an array"
	case reflect.Struct, reflect.Map:
		return "an object"
	case reflect.Pointer:
		return jsonTypeName(t.Elem())
	}
	return t.String()
}

func lineColumn(data []byte, offset int64) (line, col int) {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	if offset < 0 {
		offset = 0
	}
	line, col = 1, 1
	for _, c := range string(data[:offset]) {
		if c == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// pointerAtOffset returns the JSON Pointer of the value whose last byte lies at offset (the
// position encoding/json reports in an UnmarshalTypeError). It returns "" when it cannot tell.
func pointerAtOffset(data []byte, offset int64) string {
	type frame struct {
		obj     bool
		key     string
		haveKey bool
		idx     int
	}
	var stack []frame
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		n := len(stack)
		delim, isDelim := tok.(json.Delim)
		if isDelim && (delim == '}' || delim == ']') {
			stack = stack[:n-1]
			if n >= 2 && stack[n-2].obj {
				stack[n-2].haveKey = false
			}
			continue
		}
		if n > 0 && stack[n-1].obj && !stack[n-1].haveKey {
			key, _ := tok.(string)
			stack[n-1].key, stack[n-1].haveKey = key, true
			continue
		}
		if n > 0 && !stack[n-1].obj {
			stack[n-1].idx++
		}
		if dec.InputOffset() >= offset {
			var b strings.Builder
			for _, f := range stack {
				if f.obj {
					b.WriteString("/" + escapePointer(f.key))
				} else {
					b.WriteString("/" + strconv.Itoa(f.idx))
				}
			}
			return b.String()
		}
		if isDelim {
			stack = append(stack, frame{obj: delim == '{', idx: -1})
			continue
		}
		if n > 0 && stack[n-1].obj {
			stack[n-1].haveKey = false
		}
	}
}

// escapePointer escapes one JSON Pointer segment (RFC 6901).
func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
}

// keyOrder numbers every object key of the document by its position, keyed by its JSON Pointer.
func keyOrder(data []byte) map[string]int {
	type frame struct {
		obj     bool
		key     string
		haveKey bool
		idx     int
	}
	order := map[string]int{}
	var stack []frame
	dec := json.NewDecoder(bytes.NewReader(data))
	pointer := func() string {
		var b strings.Builder
		for _, f := range stack {
			if f.obj {
				b.WriteString("/" + escapePointer(f.key))
			} else {
				b.WriteString("/" + strconv.Itoa(f.idx))
			}
		}
		return b.String()
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return order
		}
		n := len(stack)
		delim, isDelim := tok.(json.Delim)
		if isDelim && (delim == '}' || delim == ']') {
			stack = stack[:n-1]
			if n >= 2 && stack[n-2].obj {
				stack[n-2].haveKey = false
			}
			continue
		}
		if n > 0 && stack[n-1].obj && !stack[n-1].haveKey {
			key, _ := tok.(string)
			stack[n-1].key, stack[n-1].haveKey = key, true
			order[pointer()] = len(order)
			continue
		}
		if n > 0 && !stack[n-1].obj {
			stack[n-1].idx++
		}
		if isDelim {
			stack = append(stack, frame{obj: delim == '{', idx: -1})
			continue
		}
		if n > 0 && stack[n-1].obj {
			stack[n-1].haveKey = false
		}
	}
}

// ---------------------------------------------------------------------------
// Unknown fields: a reflection walk over the generic JSON tree finds every unknown key
// with its path (the typed strict decode is the safety net behind it).

type unknownHit struct {
	path    string
	typ     string
	key     string
	allowed []string
}

func (h unknownHit) issue() Issue {
	fix := fieldHints[h.typ+"."+h.key]
	if fix == "" {
		fix = fieldHints["*."+h.key]
	}
	if fix == "" {
		fix = "Remove it, or use one of the fields of a " + typeLabel(h.typ) + ": " + strings.Join(h.allowed, ", ") + "."
	}
	return Issue{
		Severity: SeverityError, Path: h.path, Code: CodeUnknownField,
		Message: fmt.Sprintf("A %s has no field %q.", typeLabel(h.typ), h.key),
		Fix:     fix,
	}
}

func typeLabel(t string) string {
	if t == "CastMember" {
		return "cast member"
	}
	if t == "ChartData" {
		return "chart data"
	}
	return strings.ToLower(t)
}

type structFields struct {
	types map[string]reflect.Type
	order []string
}

var (
	structFieldsCache sync.Map // reflect.Type -> *structFields
	positionType      = reflect.TypeOf(Position{})
	durationType      = reflect.TypeOf(Duration{})
)

func fieldsOf(t reflect.Type) *structFields {
	if v, ok := structFieldsCache.Load(t); ok {
		return v.(*structFields)
	}
	sf := &structFields{types: map[string]reflect.Type{}}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		sf.types[name] = f.Type
		sf.order = append(sf.order, name)
	}
	structFieldsCache.Store(t, sf)
	return sf
}

func collectUnknown(v any, t reflect.Type, path string, out *[]unknownHit) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == positionType || t == durationType {
		return // polymorphic values validate their own shape
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return // the typed decode reports the wrong type
		}
		sf := fieldsOf(t)
		for k, val := range obj {
			ft, known := sf.types[k]
			p := path + "/" + escapePointer(k)
			if !known {
				*out = append(*out, unknownHit{path: p, typ: t.Name(), key: k, allowed: sf.order})
				continue
			}
			collectUnknown(val, ft, p, out)
		}
	case reflect.Slice:
		arr, ok := v.([]any)
		if !ok {
			return
		}
		for i, e := range arr {
			collectUnknown(e, t.Elem(), path+"/"+strconv.Itoa(i), out)
		}
	}
}

// fieldHints maps "<Type>.<key>" (or "*.<key>") of common model mistakes to a one-sentence fix.
var fieldHints = map[string]string{
	"Story.characters":  `Recurring people go in "cast": [{"id":"anna","name":"Anna","character":"char.office-woman-1"}].`,
	"Story.actors":      `Actors belong to a scene: scenes[].actors, each {"cast":"anna","at":"center"}.`,
	"Story.language":    `The narration language is voice.language, for example "voice":{"provider":"piper","voice":"de_DE-thorsten-high","language":"de"}.`,
	"Story.aspect":      `Put aspect and fps in "format": {"aspect":"16:9","fps":30}.`,
	"Story.fps":         `Put aspect and fps in "format": {"aspect":"16:9","fps":30}.`,
	"Story.resolution":  `The canvas is "format": {"aspect":"16:9","fps":30}; there is no free resolution.`,
	"Story.palette":     `The palette is "style": {"palette":"sunny"}.`,
	"Story.description": `A story has only a title; put the spoken text into scene narration.`,
	"Story.summary":     `A story has only a title; put the spoken text into scene narration.`,
	"Story.duration":    `Durations belong to scenes: scenes[].duration is "auto" or a number of seconds.`,
	"Story.id":          `A story has no id; the project id is kept by the app.`,
	"Story.name":        `Use "title".`,

	"Scene.text":        `The spoken text is "narration"; on-screen text is a show beat: {"do":"show","text":"...","at":"center","entry":"fade"}.`,
	"Scene.script":      `The spoken text is "narration".`,
	"Scene.dialogue":    `The spoken text is "narration"; one scene per speaker turn with "speaker":"<cast id>".`,
	"Scene.voiceover":   `The spoken text is "narration".`,
	"Scene.voice_over":  `The spoken text is "narration".`,
	"Scene.description": `The spoken text is "narration"; the place is "environment".`,
	"Scene.summary":     `The spoken text is "narration".`,
	"Scene.background":  `The place is "environment", a full id such as "env.office".`,
	"Scene.setting":     `The place is "environment", a full id such as "env.office".`,
	"Scene.location":    `The place is "environment", a full id such as "env.office".`,
	"Scene.backdrop":    `The place is "environment", a full id such as "env.office".`,
	"Scene.characters":  `People on stage are "actors": [{"cast":"anna","at":"left"}].`,
	"Scene.cast":        `People on stage are "actors": [{"cast":"anna","at":"left"}].`,
	"Scene.items":       `Objects on stage are "props": [{"item":"prop.laptop","at":"right"}].`,
	"Scene.objects":     `Objects on stage are "props": [{"item":"prop.laptop","at":"right"}].`,
	"Scene.actions":     `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.animations":  `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.events":      `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.steps":       `Timed actions are "beats": [{"do":"walk","who":"anna","to":"center"}].`,
	"Scene.code":        `A code scene has no code key: set "kind":"code"; the source is saved as scenes/<id>.js.`,
	"Scene.title":       `A scene has no title; use "narration" or a show beat with text.`,
	"Scene.name":        `A scene has no name; its "id" is a short slug such as "s1".`,
	"Scene.length":      `Use "duration": "auto" or a number of seconds.`,
	"Scene.time":        `Use "duration": "auto" or a number of seconds.`,
	"Scene.seconds":     `Use "duration": "auto" or a number of seconds.`,
	"Scene.music":       `Music is set once for the story: "music": {"mode":"none"}.`,
	"Scene.framing":     `The camera is "camera": {"framing":"wide"}.`,

	"Actor.character": `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.id":        `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.name":      `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.who":       `Use "cast" with the cast id, for example {"cast":"anna"}.`,
	"Actor.position":  `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.location":  `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.spot":      `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.x":         `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.y":         `Use "at": a spot name such as "left" or {"x":30,"y":88}.`,
	"Actor.emotion":   `Use "expression", for example "happy".`,
	"Actor.mood":      `Use "expression", for example "happy".`,
	"Actor.animation": `Use "pose", for example "typing".`,
	"Actor.action":    `Use "pose", for example "typing"; timed changes are beats.`,
	"Actor.direction": `Use "facing": "left", "right" or "auto".`,

	"Prop.name":     `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.asset":    `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.asset_id": `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.prop":     `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.object":   `Use "item" with a full asset id such as "prop.lock".`,
	"Prop.position": `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.location": `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.x":        `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.y":        `Use "at": a spot name such as "right" or {"x":70,"y":88}.`,
	"Prop.z":        `Use "layer": "back" or "front".`,
	"Prop.z_index":  `Use "layer": "back" or "front".`,

	"Beat.type":         `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.action":       `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.kind":         `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.event":        `The beat kind is "do", for example {"do":"walk","who":"anna","to":"center"}.`,
	"Beat.actor":        `The cast member is "who", for example "anna".`,
	"Beat.character":    `The cast member is "who", for example "anna".`,
	"Beat.cast":         `The cast member is "who", for example "anna".`,
	"Beat.name":         `The cast member is "who", for example "anna".`,
	"Beat.position":     `Use "to" for walk, move, enter and exit, or "at" for show.`,
	"Beat.location":     `Use "to" for walk, move, enter and exit, or "at" for show.`,
	"Beat.destination":  `Use "to" for walk, move, enter and exit.`,
	"Beat.where":        `Use "to" for walk, move, enter and exit, or "at" for show.`,
	"Beat.x":            `Use "to" or "at": a spot name or {"x":50,"y":88}.`,
	"Beat.y":            `Use "to" or "at": a spot name or {"x":50,"y":88}.`,
	"Beat.emotion":      `Use "expression" with do "emote", for example {"do":"emote","who":"anna","expression":"happy"}.`,
	"Beat.mood":         `Use "expression" with do "emote".`,
	"Beat.animation":    `Use "pose" (do "pose") or "gesture" (do "gesture").`,
	"Beat.anim":         `Use "pose" (do "pose") or "gesture" (do "gesture").`,
	"Beat.time":         `Use "delay" (seconds after the previous beat starts) or "cue" (a narration marker number).`,
	"Beat.start":        `Use "delay" (seconds after the previous beat starts) or "cue" (a narration marker number).`,
	"Beat.at_seconds":   `Use "delay" (seconds) or "cue" (a narration marker number).`,
	"Beat.offset":       `Use "delay" (seconds) or "cue" (a narration marker number).`,
	"Beat.t":            `Use "delay" (seconds) or "cue" (a narration marker number).`,
	"Beat.sfx":          `Use "sound" with an sfx id: {"do":"sfx","sound":"sfx.pop"}.`,
	"Beat.sound_effect": `Use "sound" with an sfx id: {"do":"sfx","sound":"sfx.pop"}.`,
	"Beat.image":        `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.prop":         `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.asset":        `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.object":       `Use "item" with a full asset id: {"do":"show","item":"prop.lock","at":"right","entry":"pop"}.`,
	"Beat.message":      `Use "text".`,
	"Beat.content":      `Use "text".`,
	"Beat.label":        `Use "text".`,
	"Beat.caption":      `Use "text".`,
	"Beat.speech":       `Use "text" with do "say".`,
	"Beat.direction":    `Use "from" (enter) or "to" (exit, walk).`,
	"Beat.repeat":       `Use "count" (1 to 5) with do "gesture".`,
	"Beat.times":        `Use "count" (1 to 5) with do "gesture".`,
	"Beat.easing":       `Use "ease": linear, ease-in, ease-out, ease-in-out, back-out or bounce-out.`,
	"Beat.curve":        `Use "ease": linear, ease-in, ease-out, ease-in-out, back-out or bounce-out.`,
	"Beat.wait":         `Use {"do":"wait","seconds":1}.`,

	"CastMember.char":        `Use "character" with a full asset id such as "char.office-woman-1".`,
	"CastMember.avatar":      `Use "character" with a full asset id such as "char.office-woman-1".`,
	"CastMember.asset":       `Use "character" with a full asset id such as "char.office-woman-1".`,
	"CastMember.displayname": `Use "name".`,
	"CastMember.label":       `Use "name".`,

	"Format.width":      `The canvas is "aspect": "16:9", "9:16" or "1:1"; there is no free size.`,
	"Format.height":     `The canvas is "aspect": "16:9", "9:16" or "1:1"; there is no free size.`,
	"Format.resolution": `The canvas is "aspect": "16:9", "9:16" or "1:1".`,
	"Format.framerate":  `Use "fps": 24, 25 or 30.`,

	"Style.color":     `Use "palette" with a palette name such as "sunny".`,
	"Style.colors":    `Use "palette" with a palette name such as "sunny".`,
	"Style.theme":     `Use "palette" with a palette name such as "sunny".`,
	"Style.typeface":  `Use "font": "default" or a bundled font name.`,
	"Style.subtitles": `Use "captions": true or false.`,

	"Voice.lang":   `Use "language", for example "de".`,
	"Voice.engine": `Use "provider", for example "piper".`,
	"Voice.tts":    `Use "provider", for example "piper".`,
	"Voice.id":     `Use "voice" with the voice id.`,
	"Voice.name":   `Use "voice" with the voice id.`,

	"Music.path":  `Use "file" with a desktop path and "mode":"file".`,
	"Music.url":   `Music cannot be loaded from a URL; use "mode":"file" with a desktop path.`,
	"Music.style": `Use "mood", for example "upbeat-corporate".`,
	"Music.genre": `Use "mood", for example "upbeat-corporate".`,
	"Music.gain":  `Use "volume" between 0 and 1.`,
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/storymaker/ -run 'TestDecode|TestPolymorphic|TestStyle|TestCanonical|TestPointer' -count=1`, then `go test ./internal/storymaker/ -count=1`
Expected: PASS (`ok  aurago/internal/storymaker`)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/model.go internal/storymaker/decode.go internal/storymaker/testdata/stories/minimal.json internal/storymaker/testdata/stories/firewall.json internal/storymaker/decode_test.go
git commit -m "feat(storymaker): story model with strict decoding and field hints" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A3: Catalog index

**Files:**
- Create: `internal/storymaker/catalog.go`
- Create: `internal/storymaker/testdata/catalog-mini/catalog.json`
- Test: `internal/storymaker/catalog_test.go`

- [ ] **Step 1: Write the failing test**

`internal/storymaker/testdata/catalog-mini/catalog.json`:

```json
{"schema":"storymaker-catalog/1","version":"mini00000000",
 "stage":{"width":1920,"height":1080,"safe":{"x":5,"y":5,"w":90,"h":90}},
 "rig":{"id":"rig.human/1","file":"rig/human.json"},
 "fonts":[{"name":"default","family":"Storymaker Default","weight":400,"file":"/fonts/placeholder.woff2","licence":"OFL-1.1"}],
 "palettes":[{"id":"palette.sunny","name":"sunny","name_key":"storymaker.catalog.palette_sunny","tokens":{"bg":"#fff7e6","bg2":"#ffe0a3","fg":"#1f2a44","accent":"#ff7a45","accent2":"#36cfc9","text":"#1f2a44","outline":"#1f2a44"}}],
 "entries":[
  {"id":"char.office-woman-1","kind":"character","name":"char.office-woman-1","name_key":"storymaker.catalog.char_office-woman-1","tags":["woman","office","business"],"thumb":"thumbs/char.office-woman-1.svg","usage_example":{"cast":[{"id":"anna","character":"char.office-woman-1"}]},"params":{"body":"average","skin":"medium","hair":"bun","hair_color":"dark-brown","eyes":"round","brows":"soft","facial_hair":"none","glasses":"none","outfit":"business","accessory":"none"}},
  {"id":"prop.lock","kind":"prop","name":"prop.lock","name_key":"storymaker.catalog.prop_lock","tags":["security","lock","padlock"],"thumb":"thumbs/prop.lock.svg","usage_example":{"do":"show","item":"prop.lock","at":"right","entry":"pop"},"file":"props/lock.json","size":{"h":0.14},"anchor":{"x":0.5,"y":1}},
  {"id":"prop.shield","kind":"prop","name":"prop.shield","name_key":"storymaker.catalog.prop_shield","tags":["security","shield","protection"],"thumb":"thumbs/prop.shield.svg","usage_example":{"do":"show","item":"prop.shield","at":"right","entry":"fade"},"file":"props/shield.json","size":{"h":0.16},"anchor":{"x":0.5,"y":1}},
  {"id":"env.plain-stage","kind":"environment","name":"env.plain-stage","name_key":"storymaker.catalog.env_plain-stage","tags":["plain","stage","neutral"],"thumb":"thumbs/env.plain-stage.svg","usage_example":{"environment":"env.plain-stage"},"file":"environments/plain-stage.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":74,"y":62}},"floor":{"y_near":96,"y_far":70,"scale_near":1.05,"scale_far":0.8},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50,"spots":{"far-left":{"x":38,"y":88},"far-right":{"x":62,"y":88},"left":{"x":42,"y":88},"right":{"x":58,"y":88}}},"1:1":{"focus_x":50}}},
  {"id":"env.office","kind":"environment","name":"env.office","name_key":"storymaker.catalog.env_office","tags":["office","work","desk"],"thumb":"thumbs/env.office.svg","usage_example":{"environment":"env.office"},"file":"environments/office.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"desk":{"x":36,"y":84,"face":"right"},"door":{"x":90,"y":82,"face":"left"},"board":{"x":70,"y":60},"window":{"x":18,"y":82}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":40},"1:1":{"focus_x":45}}},
  {"id":"env.abstract-tech","kind":"environment","name":"env.abstract-tech","name_key":"storymaker.catalog.env_abstract-tech","tags":["tech","abstract","network"],"thumb":"thumbs/env.abstract-tech.svg","usage_example":{"environment":"env.abstract-tech"},"file":"environments/abstract-tech.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":70,"y":58}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50},"1:1":{"focus_x":50}}},
  {"id":"pose.stand","kind":"pose","name":"stand","name_key":"storymaker.catalog.pose_stand","tags":["stand"],"thumb":"thumbs/pose.stand.svg","usage_example":{"pose":"stand"},"bones":{}},
  {"id":"pose.typing","kind":"pose","name":"typing","name_key":"storymaker.catalog.pose_typing","tags":["typing","computer","sit"],"thumb":"thumbs/pose.typing.svg","usage_example":{"pose":"typing"},"bones":{"upper_leg_l":-80,"lower_leg_l":80,"upper_leg_r":-80,"lower_leg_r":80,"upper_arm_l":-30,"lower_arm_l":-70,"upper_arm_r":-30,"lower_arm_r":-70},"hands":{"l":"open","r":"open"},"seated":true},
  {"id":"gesture.point","kind":"gesture","name":"point","name_key":"storymaker.catalog.gesture_point","tags":["point","show"],"thumb":"thumbs/gesture.point.svg","usage_example":{"do":"gesture","who":"anna","gesture":"point"},"duration":1.2,"keys":[{"t":0,"bones":{}},{"t":0.3,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":0.8,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":1,"bones":{}}]},
  {"id":"expr.neutral","kind":"expression","name":"neutral","name_key":"storymaker.catalog.expr_neutral","tags":["neutral"],"thumb":"thumbs/expr.neutral.svg","usage_example":{"expression":"neutral"},"eyes":"neutral","brows":"neutral","mouth":"rest"},
  {"id":"expr.surprised","kind":"expression","name":"surprised","name_key":"storymaker.catalog.expr_surprised","tags":["surprised","shock"],"thumb":"thumbs/expr.surprised.svg","usage_example":{"do":"emote","who":"anna","expression":"surprised"},"eyes":"wide","brows":"raised","mouth":"o"},
  {"id":"expr.happy","kind":"expression","name":"happy","name_key":"storymaker.catalog.expr_happy","tags":["happy","smile"],"thumb":"thumbs/expr.happy.svg","usage_example":{"do":"emote","who":"anna","expression":"happy"},"eyes":"smile","brows":"neutral","mouth":"smile"},
  {"id":"mouth.rest","kind":"mouth","name":"rest","name_key":"storymaker.catalog.mouth_rest","tags":["mouth"],"thumb":"thumbs/mouth.rest.svg","usage_example":{},"shape":"rest"},
  {"id":"cycle.walk","kind":"cycle","name":"walk","name_key":"storymaker.catalog.cycle_walk","tags":["walk"],"thumb":"thumbs/cycle.walk.svg","usage_example":{"do":"walk","who":"anna","to":"center"},"period":1.0,"speed":12,"keys":[{"t":0,"bones":{"upper_leg_l":20,"upper_leg_r":-20,"upper_arm_l":-15,"upper_arm_r":15}},{"t":0.5,"bones":{"upper_leg_l":-20,"upper_leg_r":20,"upper_arm_l":15,"upper_arm_r":-15},"offset_y":-6},{"t":1,"bones":{"upper_leg_l":20,"upper_leg_r":-20,"upper_arm_l":-15,"upper_arm_r":15}}]},
  {"id":"cycle.idle","kind":"cycle","name":"idle","name_key":"storymaker.catalog.cycle_idle","tags":["idle"],"thumb":"thumbs/cycle.idle.svg","usage_example":{},"period":3.0,"keys":[{"t":0,"bones":{"torso":0}},{"t":0.5,"bones":{"torso":1.5},"offset_y":-2},{"t":1,"bones":{"torso":0}}]},
  {"id":"text.title","kind":"text_style","name":"title","name_key":"storymaker.catalog.text_title","tags":["title","heading"],"thumb":"thumbs/text.title.svg","usage_example":{"do":"show","text":"Hallo","at":"center"},"font":"default","size":0.07,"weight":700,"color":"$text","align":"center","max_lines":2},
  {"id":"trans.cut","kind":"transition","name":"cut","name_key":"storymaker.catalog.trans_cut","tags":["cut"],"thumb":"thumbs/trans.cut.svg","usage_example":{"transition":"cut"},"duration":0},
  {"id":"trans.fade","kind":"transition","name":"fade","name_key":"storymaker.catalog.trans_fade","tags":["fade"],"thumb":"thumbs/trans.fade.svg","usage_example":{"transition":"fade"},"duration":0.5},
  {"id":"trans.slide-left","kind":"transition","name":"slide-left","name_key":"storymaker.catalog.trans_slide-left","tags":["slide"],"thumb":"thumbs/trans.slide-left.svg","usage_example":{"transition":"slide-left"},"duration":0.5},
  {"id":"trans.wipe","kind":"transition","name":"wipe","name_key":"storymaker.catalog.trans_wipe","tags":["wipe"],"thumb":"thumbs/trans.wipe.svg","usage_example":{"transition":"wipe"},"duration":0.5}
 ]}
```

`internal/storymaker/catalog_test.go`:

```go
package storymaker

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

var (
	miniOnce sync.Once
	miniCat  *Catalog
	miniErr  error
)

// loadMini loads the fixture catalog (shared by the tests; it is immutable).
func loadMini(t testing.TB) *Catalog {
	t.Helper()
	miniOnce.Do(func() {
		miniCat, miniErr = LoadCatalog(os.DirFS("testdata/catalog-mini"), "catalog.json")
	})
	if miniErr != nil {
		t.Fatalf("LoadCatalog(catalog-mini): %v", miniErr)
	}
	return miniCat
}

func TestLoadCatalogMini(t *testing.T) {
	c := loadMini(t)
	if c.Version() != "mini00000000" || c.Schema != CatalogSchema || c.Stage.Width != 1920 || c.Rig.ID != "rig.human/1" {
		t.Fatalf("header: %q %q %+v %+v", c.Version(), c.Schema, c.Stage, c.Rig)
	}
	if e, ok := c.Lookup("prop.lock"); !ok || e.Kind != "prop" || e.File != "props/lock.json" || e.NameKey != "storymaker.catalog.prop_lock" {
		t.Fatalf("prop.lock: %+v %v", e, ok)
	}
	if e, ok := c.Lookup("palette.sunny"); !ok || e.Kind != "palette" || e.Name != "sunny" {
		t.Fatalf("palette.sunny: %+v %v", e, ok)
	}
	if _, ok := c.Lookup("prop.nothing"); ok {
		t.Fatal("found a missing id")
	}
	e, _ := c.Lookup("pose.typing")
	var raw map[string]any
	if err := json.Unmarshal(e.Raw(), &raw); err != nil || raw["seated"] != true {
		t.Fatalf("raw entry lost kind-specific fields: %s", e.Raw())
	}
	if out, err := json.Marshal(e); err != nil || string(out) != string(e.Raw()) {
		t.Fatalf("entry must marshal as the catalog has it: %s %v", out, err)
	}
	cases := map[string][]string{
		"pose":        {"stand", "typing"},
		"expression":  {"neutral", "surprised", "happy"},
		"transition":  {"cut", "fade", "slide-left", "wipe"},
		"palette":     {"sunny"},
		"font":        {"default"},
		"environment": {"env.plain-stage", "env.office", "env.abstract-tech"},
		"trans":       {"cut", "fade", "slide-left", "wipe"}, // alias for the id prefix
	}
	for kind, want := range cases {
		if got := c.Names(kind); !reflect.DeepEqual(got, want) {
			t.Errorf("Names(%s) = %v, want %v", kind, got, want)
		}
	}
	if c.Names("banana") != nil {
		t.Error("unknown kind must return nil")
	}
	if !c.HasName("pose", "typing") || c.HasName("pose", "facepalm") || !c.HasName("text", "title") {
		t.Error("HasName")
	}
}

func TestCatalogSearch(t *testing.T) {
	c := loadMini(t)
	ids := func(es []Entry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.ID)
		}
		return out
	}
	cases := []struct {
		query, kind string
		limit       int
		want        []string
	}{
		{"padlock", "", 0, []string{"prop.lock"}},                              // tag
		{"lock", "prop", 0, []string{"prop.lock"}},                             // id and tag, kind filter
		{"security", "prop", 0, []string{"prop.lock", "prop.shield"}},          // tie keeps catalog order
		{"prop.shield", "", 0, []string{"prop.shield"}},                        // exact id first
		{"office woman", "", 0, []string{"char.office-woman-1", "env.office"}}, // two terms add up
		{"locks", "prop", 0, []string{"prop.lock"}},                            // plural
		{"office", "environment", 0, []string{"env.office"}},
		{"nothing-like-this", "", 0, nil},
		{"", "transition", 0, []string{"trans.cut", "trans.fade", "trans.slide-left", "trans.wipe"}}, // list a kind
		{"", "trans", 2, []string{"trans.cut", "trans.fade"}},                                        // alias and limit
		{"SECURITY", "prop", 1, []string{"prop.lock"}},                                               // case-insensitive
		{"x", "banana", 0, nil},                                                                      // unknown kind
	}
	for _, tc := range cases {
		got := ids(c.Search(tc.query, tc.kind, tc.limit))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Search(%q, %q, %d) = %v, want %v", tc.query, tc.kind, tc.limit, got, tc.want)
		}
	}
	if n := len(c.Search("", "", 0)); n != 6 {
		t.Errorf("default limit: %d", n)
	}
	if n := len(c.Search("", "", 100)); n != 12 {
		t.Errorf("limit cap: %d", n)
	}
}

func TestCatalogSpots(t *testing.T) {
	c := loadMini(t)
	spots, ok := c.Spots("env.office", "16:9")
	if !ok || spots["desk"].Face != "right" || spots["center"].X != 50 || len(spots) != 9 {
		t.Fatalf("office spots: %v %+v", ok, spots)
	}
	// 9:16 replaces single spots of plain-stage and keeps the others.
	tall, _ := c.Spots("env.plain-stage", "9:16")
	if tall["left"].X != 42 || tall["right"].X != 58 || tall["center"].X != 50 || tall["board"].X != 74 {
		t.Fatalf("9:16 spots: %+v", tall)
	}
	if base, _ := c.Spots("env.plain-stage", "16:9"); base["left"].X != 30 {
		t.Fatalf("the base spots changed: %+v", base)
	}
	if got := c.SpotNames("env.office"); !reflect.DeepEqual(got, []string{"far-left", "left", "center", "right", "far-right", "board", "desk", "door", "window"}) {
		t.Fatalf("SpotNames: %v", got)
	}
	bg, ok := c.Spots("my.bg.park", "16:9")
	if !ok || len(bg) != 3 || bg["center"].X != 50 {
		t.Fatalf("project background spots: %v %+v", ok, bg)
	}
	if _, ok := c.Spots("env.nowhere", "16:9"); ok {
		t.Fatal("unknown environment has spots")
	}
	if _, ok := c.Spots("prop.lock", "16:9"); ok {
		t.Fatal("a prop is not an environment")
	}
	if c.FocusX("env.office", "9:16") != 40 || c.FocusX("env.office", "16:9") != 50 {
		t.Fatalf("FocusX: %v %v", c.FocusX("env.office", "9:16"), c.FocusX("env.office", "16:9"))
	}
}

// miniWith returns a copy of the mini catalog JSON with edits applied.
func miniWith(t *testing.T, edit func(string) string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/catalog-mini/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	return []byte(edit(string(data)))
}

func TestLoadCatalogRejectsBadManifests(t *testing.T) {
	cases := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"schema", func(s string) string {
			return strings.Replace(s, `"storymaker-catalog/1"`, `"storymaker-catalog/9"`, 1)
		}, "schema"},
		{"empty version", func(s string) string { return strings.Replace(s, `"mini00000000"`, `""`, 1) }, "version"},
		{"unknown kind", func(s string) string { return strings.Replace(s, `"kind":"cycle"`, `"kind":"banana"`, 1) }, `unknown kind "banana"`},
		{"palette in entries", func(s string) string { return strings.Replace(s, `"kind":"cycle"`, `"kind":"palette"`, 1) }, "palettes list"},
		{"duplicate id", func(s string) string { return strings.Replace(s, `"id":"prop.shield"`, `"id":"prop.lock"`, 1) }, "duplicate id prop.lock"},
		{"wrong prefix", func(s string) string {
			return strings.Replace(s, `"id":"prop.shield","kind":"prop"`, `"id":"obj.shield","kind":"prop"`, 1)
		}, "must start with prop."},
		{"enum name differs from id", func(s string) string { return strings.Replace(s, `"name":"typing"`, `"name":"typed"`, 1) }, `name must be "typing"`},
		{"name key", func(s string) string {
			return strings.Replace(s, `"storymaker.catalog.prop_lock"`, `"storymaker.catalog.lock"`, 1)
		}, "name_key must be storymaker.catalog.prop_lock"},
		{"thumb path", func(s string) string {
			return strings.Replace(s, `"thumbs/prop.lock.svg"`, `"../thumbs/prop.lock.svg"`, 1)
		}, "thumb"},
		{"prop without size", func(s string) string { return strings.Replace(s, `"size":{"h":0.14},`, ``, 1) }, "size.h"},
		{"prop file escapes", func(s string) string { return strings.Replace(s, `"props/lock.json"`, `"/etc/passwd"`, 1) }, "file must be a relative path"},
		{"character param", func(s string) string { return strings.Replace(s, `"accessory":"none"`, `"accessory":""`, 1) }, "params.accessory"},
		{"environment base spot", func(s string) string {
			return strings.Replace(s, `"far-right":{"x":88,"y":88},"board":{"x":74,"y":62}`, `"board":{"x":74,"y":62}`, 1)
		}, "spots.far-right is required"},
		{"spot range", func(s string) string {
			return strings.Replace(s, `"board":{"x":74,"y":62}`, `"board":{"x":174,"y":62}`, 1)
		}, "between 0 and 100"},
		{"aspect key", func(s string) string {
			return strings.Replace(s, `"1:1":{"focus_x":50}}},`, `"4:3":{"focus_x":50}}},`, 1)
		}, `aspects key "4:3"`},
		{"expression mouth", func(s string) string {
			return strings.Replace(s, `"eyes":"wide","brows":"raised","mouth":"o"`, `"eyes":"wide","brows":"raised"`, 1)
		}, "eyes, brows and mouth"},
		{"mouth shape", func(s string) string { return strings.Replace(s, `"shape":"rest"`, `"shape":"banana"`, 1) }, "shape must be one of"},
		{"transition duration", func(s string) string { return strings.Replace(s, `"duration":0}`, `"x":0}`, 1) }, "duration"},
		{"text style", func(s string) string { return strings.Replace(s, `"max_lines":2`, `"max_lines":0`, 1) }, "max_lines"},
		{"palette token", func(s string) string { return strings.Replace(s, `"bg2":"#ffe0a3",`, ``, 1) }, "token bg2"},
		{"no default font", func(s string) string {
			return strings.Replace(s, `"name":"default","family"`, `"name":"body","family"`, 1)
		}, "font named default"},
		{"font path", func(s string) string {
			return strings.Replace(s, `"/fonts/placeholder.woff2"`, `"fonts/placeholder.woff2"`, 1)
		}, "/fonts/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := miniWith(t, tc.edit)
			if string(data) == string(miniWith(t, func(s string) string { return s })) {
				t.Fatal("the edit changed nothing")
			}
			fsys := fstest.MapFS{"catalog.json": {Data: data}}
			_, err := LoadCatalog(fsys, "catalog.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want one containing %q", err, tc.want)
			}
		})
	}
	if _, err := LoadCatalog(fstest.MapFS{}, "catalog.json"); err == nil {
		t.Error("a missing catalog must fail")
	}
	if _, err := LoadCatalog(fstest.MapFS{"catalog.json": {Data: []byte("{")}}, "catalog.json"); err == nil {
		t.Error("broken JSON must fail")
	}
}

func TestCatalogVerifyFiles(t *testing.T) {
	c := loadMini(t)
	// The mini fixture references files that part C owns; the manifest alone loads fine.
	fsys := fstest.MapFS{}
	err := c.VerifyFiles(fsys, "img/storymaker")
	if err == nil || !strings.Contains(err.Error(), "img/storymaker/rig/human.json") || !strings.Contains(err.Error(), "27 missing files") {
		t.Fatalf("missing files not reported: %v", err)
	}
	for _, e := range c.Entries() {
		if e.Kind == "palette" {
			continue
		}
		if e.File != "" {
			fsys["img/storymaker/"+e.File] = &fstest.MapFile{Data: []byte("{}")}
		}
		fsys["img/storymaker/"+e.Thumb] = &fstest.MapFile{Data: []byte("<svg/>")}
	}
	fsys["img/storymaker/rig/human.json"] = &fstest.MapFile{Data: []byte("{}")}
	if err := c.VerifyFiles(fsys, "img/storymaker"); err == nil || !strings.Contains(err.Error(), "fonts/placeholder.woff2") || strings.Contains(err.Error(), "thumbs/") {
		t.Fatalf("only the font should be missing: %v", err)
	}
	fsys["fonts/placeholder.woff2"] = &fstest.MapFile{Data: []byte("font")}
	if err := c.VerifyFiles(fsys, "img/storymaker"); err != nil {
		t.Fatalf("complete tree rejected: %v", err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/ -run 'Catalog' -count=1`
Expected: FAIL (build failed: `undefined: LoadCatalog`)

- [ ] **Step 3: Implement**

`internal/storymaker/catalog.go`:

```go
package storymaker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// CatalogSchema is the schema string of catalog.json.
const CatalogSchema = "storymaker-catalog/1"

const maxCatalogBytes = 16 << 20

// kindPrefixes maps every entry kind to its ID prefix. Palettes live in the manifest's
// palettes list and fonts in its fonts list; both are indexed here as well.
var kindPrefixes = map[string]string{
	"character":   "char.",
	"prop":        "prop.",
	"environment": "env.",
	"pose":        "pose.",
	"gesture":     "gesture.",
	"expression":  "expr.",
	"mouth":       "mouth.",
	"cycle":       "cycle.",
	"text_style":  "text.",
	"transition":  "trans.",
	"palette":     "palette.",
	"sfx":         "sfx.",
}

// enumKinds are the kinds whose name is the bare name used in story fields.
var enumKinds = nameSet("pose", "gesture", "expression", "mouth", "cycle", "text_style", "transition", "palette")

// kindAliases lets callers pass an ID prefix or a plural instead of the kind.
var kindAliases = map[string]string{
	"char": "character", "chars": "character", "characters": "character", "people": "character",
	"props": "prop", "env": "environment", "envs": "environment", "environments": "environment", "background": "environment",
	"poses": "pose", "gestures": "gesture", "expr": "expression", "expressions": "expression",
	"mouths": "mouth", "cycles": "cycle", "text": "text_style", "text_styles": "text_style",
	"trans": "transition", "transitions": "transition", "palettes": "palette", "fonts": "font",
}

// NormalizeKind returns the canonical catalog kind for a kind, alias or ID prefix, or "".
func NormalizeKind(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	k = strings.TrimSuffix(k, ".")
	if _, ok := kindPrefixes[k]; ok || k == "font" {
		return k
	}
	return kindAliases[k]
}

// Spot is a named standing position of an environment, in percent of the design frame.
type Spot struct {
	X     float64  `json:"x"`
	Y     float64  `json:"y"` // feet line
	Depth *float64 `json:"depth,omitempty"`
	Face  string   `json:"face,omitempty"` // left | right
}

// Floor describes how actor size changes with depth.
type Floor struct {
	YNear     float64 `json:"y_near"`
	YFar      float64 `json:"y_far"`
	ScaleNear float64 `json:"scale_near"`
	ScaleFar  float64 `json:"scale_far"`
}

// AspectOverride adjusts an environment for 9:16 or 1:1.
type AspectOverride struct {
	FocusX float64         `json:"focus_x"`
	Spots  map[string]Spot `json:"spots,omitempty"`
}

// Entry is one catalog entry. Common fields and the environment data are typed; Raw keeps
// the complete entry for describe answers (kind-specific fields differ and some names
// clash between kinds, for example size).
type Entry struct {
	ID           string                    `json:"id"`
	Kind         string                    `json:"kind"`
	Name         string                    `json:"name"`
	NameKey      string                    `json:"name_key"`
	Tags         []string                  `json:"tags"`
	Thumb        string                    `json:"thumb"`
	UsageExample json.RawMessage           `json:"usage_example"`
	File         string                    `json:"file,omitempty"`
	Params       map[string]string         `json:"params,omitempty"`
	Spots        map[string]Spot           `json:"spots,omitempty"`
	Floor        *Floor                    `json:"floor,omitempty"`
	Variants     []string                  `json:"variants,omitempty"`
	Aspects      map[string]AspectOverride `json:"aspects,omitempty"`
	raw          json.RawMessage
}

// MarshalJSON writes the entry exactly as the catalog has it.
func (e Entry) MarshalJSON() ([]byte, error) {
	if len(e.raw) > 0 {
		return e.raw, nil
	}
	type plain Entry
	return json.Marshal(plain(e))
}

// Raw returns the complete catalog JSON of the entry.
func (e Entry) Raw() json.RawMessage { return e.raw }

// Font is one font face of the catalog.
type Font struct {
	Name    string `json:"name"`
	Family  string `json:"family"`
	Weight  int    `json:"weight"`
	File    string `json:"file"` // /fonts/<file> in the ui tree
	Licence string `json:"licence"`
}

// Palette is a named colour palette.
type Palette struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	NameKey string            `json:"name_key"`
	Tokens  map[string]string `json:"tokens"`
}

// Stage is the design frame and its safe area (percent).
type Stage struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Safe   struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		W float64 `json:"w"`
		H float64 `json:"h"`
	} `json:"safe"`
}

// RigRef names the rig file.
type RigRef struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// Catalog is the loaded catalog.json index. It is immutable after LoadCatalog, so it can be
// shared between goroutines; callers must not modify the exported slices and maps.
type Catalog struct {
	Schema   string
	Stage    Stage
	Rig      RigRef
	Fonts    []Font
	Palettes []Palette

	version string
	entries []Entry
	byID    map[string]int
}

type catalogFile struct {
	Schema   string            `json:"schema"`
	Version  string            `json:"version"`
	Stage    Stage             `json:"stage"`
	Rig      RigRef            `json:"rig"`
	Fonts    []Font            `json:"fonts"`
	Palettes []Palette         `json:"palettes"`
	Entries  []json.RawMessage `json:"entries"`
}

var paletteTokenNames = []string{"bg", "bg2", "fg", "accent", "accent2", "text", "outline"}

var characterParamNames = []string{"body", "skin", "hair", "hair_color", "eyes", "brows", "facial_hair", "glasses", "outfit", "accessory"}

// LoadCatalog reads and checks the manifest at path (for the ui tree: "img/storymaker/catalog.json").
// It validates the manifest only: the schema string, kinds, duplicate IDs, the ID prefix of each
// kind, the name keys and the required fields of each kind. It does not open the files that
// entries reference; VerifyFiles does that.
func LoadCatalog(fsys fs.FS, path string) (*Catalog, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("storymaker: open catalog: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCatalogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("storymaker: read catalog: %w", err)
	}
	if len(data) > maxCatalogBytes {
		return nil, fmt.Errorf("storymaker: catalog %s is larger than %d bytes", path, maxCatalogBytes)
	}
	var m catalogFile
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("storymaker: parse catalog %s: %w", path, err)
	}

	var problems []string
	bad := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if m.Schema != CatalogSchema {
		bad("schema is %q, want %q", m.Schema, CatalogSchema)
	}
	if strings.TrimSpace(m.Version) == "" || len(m.Version) > 64 {
		bad("version must be a non-empty string of at most 64 characters")
	}
	if m.Stage.Width <= 0 || m.Stage.Height <= 0 || m.Stage.Safe.W <= 0 || m.Stage.Safe.H <= 0 {
		bad("stage needs a positive width, height and safe area")
	}
	if m.Rig.ID == "" || !safeRelPath(m.Rig.File) {
		bad("rig needs an id and a relative file path")
	}
	c := &Catalog{Schema: m.Schema, Stage: m.Stage, Rig: m.Rig, version: m.Version, byID: map[string]int{}}

	hasDefaultFont := false
	for i, ft := range m.Fonts {
		switch {
		case ft.Name == "" || ft.Family == "" || ft.Weight <= 0 || ft.Licence == "":
			bad("fonts[%d] needs name, family, weight and licence", i)
		case !strings.HasPrefix(ft.File, "/fonts/") || strings.Contains(ft.File, ".."):
			bad("fonts[%d] file must be a /fonts/ path", i)
		}
		hasDefaultFont = hasDefaultFont || ft.Name == "default"
	}
	if !hasDefaultFont {
		bad("fonts needs a font named default (story style.font defaults to it)")
	}
	c.Fonts = m.Fonts

	seen := map[string]bool{}
	duplicate := func(id string) bool {
		if seen[id] {
			bad("duplicate id %s", id)
			return true
		}
		seen[id] = true
		return false
	}
	add := func(e Entry) {
		c.byID[e.ID] = len(c.entries)
		c.entries = append(c.entries, e)
	}

	for i, p := range m.Palettes {
		switch {
		case !strings.HasPrefix(p.ID, "palette.") || p.Name != strings.TrimPrefix(p.ID, "palette."):
			bad("palettes[%d] id %q must be palette.<name>", i, p.ID)
		case p.NameKey != catalogNameKey(p.ID):
			bad("palette %s name_key must be %s", p.ID, catalogNameKey(p.ID))
		default:
			for _, tok := range paletteTokenNames {
				if !strings.HasPrefix(p.Tokens[tok], "#") {
					bad("palette %s needs a hex colour for token %s", p.ID, tok)
				}
			}
		}
		if duplicate(p.ID) {
			continue
		}
		raw, _ := json.Marshal(p)
		add(Entry{ID: p.ID, Kind: "palette", Name: p.Name, NameKey: p.NameKey, raw: raw})
	}

	for i, raw := range m.Entries {
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			bad("entries[%d]: %v", i, err)
			continue
		}
		e.raw = append(json.RawMessage(nil), raw...)
		if duplicate(e.ID) || !c.checkEntry(&e, raw, i, bad) {
			continue
		}
		add(e)
	}

	if len(problems) > 0 {
		shown := problems
		if len(shown) > 12 {
			shown = append(append([]string(nil), shown[:12]...), fmt.Sprintf("... and %d more", len(problems)-12))
		}
		return nil, fmt.Errorf("storymaker: catalog %s is invalid:\n - %s", path, strings.Join(shown, "\n - "))
	}
	return c, nil
}

func catalogNameKey(id string) string {
	return "storymaker.catalog." + strings.ReplaceAll(id, ".", "_")
}

// safeRelPath reports a non-empty slash path that stays inside its directory.
func safeRelPath(p string) bool {
	if p == "" || strings.ContainsAny(p, "\\:\x00") || strings.HasPrefix(p, "/") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// checkEntry validates the common and kind-specific manifest fields of one entry.
func (c *Catalog) checkEntry(e *Entry, raw json.RawMessage, i int, bad func(string, ...any)) bool {
	label := fmt.Sprintf("entries[%d]", i)
	if e.ID != "" {
		label = e.ID
	}
	n := 0
	fail := func(format string, args ...any) {
		n++
		bad("%s: "+format, append([]any{label}, args...)...)
	}
	prefix, known := kindPrefixes[e.Kind]
	switch {
	case e.Kind == "palette":
		fail("palettes belong to the palettes list, not to entries")
		return false
	case !known:
		fail("unknown kind %q", e.Kind)
		return false
	case !strings.HasPrefix(e.ID, prefix) || len(e.ID) == len(prefix):
		fail("id must start with %s for kind %s", prefix, e.Kind)
		return false
	}
	suffix := strings.TrimPrefix(e.ID, prefix)
	if enumKinds[e.Kind] && e.Name != suffix {
		fail("name must be %q (the id without its prefix)", suffix)
	}
	if !enumKinds[e.Kind] && e.Name == "" {
		fail("name is required")
	}
	if e.NameKey != catalogNameKey(e.ID) {
		fail("name_key must be %s", catalogNameKey(e.ID))
	}
	if !safeRelPath(e.Thumb) || !strings.HasSuffix(e.Thumb, ".svg") {
		fail("thumb must be a relative .svg path")
	}
	if len(e.UsageExample) == 0 || e.UsageExample[0] != '{' {
		fail("usage_example must be an object")
	}
	inRange := func(what string, v float64) {
		if v < 0 || v > 100 {
			fail("%s must be between 0 and 100", what)
		}
	}

	switch e.Kind {
	case "character":
		for _, k := range characterParamNames {
			if strings.TrimSpace(e.Params[k]) == "" {
				fail("params.%s is required", k)
			}
		}
	case "prop":
		var p struct {
			Size *struct {
				H float64 `json:"h"`
			} `json:"size"`
			Anchor *struct {
				X *float64 `json:"x"`
				Y *float64 `json:"y"`
			} `json:"anchor"`
		}
		_ = json.Unmarshal(raw, &p)
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
		if p.Size == nil || p.Size.H <= 0 {
			fail("size.h must be a positive number")
		}
		if p.Anchor == nil || p.Anchor.X == nil || p.Anchor.Y == nil {
			fail("anchor needs x and y")
		}
	case "environment":
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
		for _, base := range []string{"far-left", "left", "center", "right", "far-right"} {
			if _, ok := e.Spots[base]; !ok {
				fail("spots.%s is required", base)
			}
		}
		for name, s := range e.Spots {
			if !validSpotName(name) {
				fail("spot name %q must be lowercase letters, digits and dashes", name)
			}
			inRange("spot "+name+" x", s.X)
			inRange("spot "+name+" y", s.Y)
			if s.Face != "" && s.Face != "left" && s.Face != "right" {
				fail("spot %s face must be left or right", name)
			}
		}
		if e.Floor == nil || e.Floor.ScaleNear <= 0 || e.Floor.ScaleFar <= 0 {
			fail("floor needs y_near, y_far, scale_near and scale_far")
		}
		if len(e.Variants) == 0 {
			fail("variants is required")
		}
		for _, v := range e.Variants {
			if v != "day" && v != "night" {
				fail("variant %q must be day or night", v)
			}
		}
		for asp, ov := range e.Aspects {
			if asp != "9:16" && asp != "1:1" {
				fail("aspects key %q must be 9:16 or 1:1", asp)
			}
			for name, s := range ov.Spots {
				inRange("aspect "+asp+" spot "+name+" x", s.X)
				inRange("aspect "+asp+" spot "+name+" y", s.Y)
			}
		}
	case "sfx":
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
	case "pose":
		var p struct {
			Bones map[string]float64 `json:"bones"`
		}
		_ = json.Unmarshal(raw, &p)
		if p.Bones == nil {
			fail("bones is required (use {} for the rest pose)")
		}
	case "gesture":
		var g struct {
			Keys     []json.RawMessage `json:"keys"`
			Duration float64           `json:"duration"`
		}
		_ = json.Unmarshal(raw, &g)
		if len(g.Keys) < 2 || g.Duration <= 0 {
			fail("keys (at least two) and a positive duration are required")
		}
	case "expression":
		var x struct {
			Eyes  string `json:"eyes"`
			Brows string `json:"brows"`
			Mouth string `json:"mouth"`
		}
		_ = json.Unmarshal(raw, &x)
		if x.Eyes == "" || x.Brows == "" || x.Mouth == "" {
			fail("eyes, brows and mouth are required")
		}
	case "mouth":
		var m struct {
			Shape string `json:"shape"`
		}
		_ = json.Unmarshal(raw, &m)
		if !containsName(MouthNames, m.Shape) {
			fail("shape must be one of %s", strings.Join(MouthNames, ", "))
		}
	case "cycle":
		var cy struct {
			Period float64           `json:"period"`
			Keys   []json.RawMessage `json:"keys"`
		}
		_ = json.Unmarshal(raw, &cy)
		if cy.Period <= 0 || len(cy.Keys) < 2 {
			fail("a positive period and at least two keys are required")
		}
	case "text_style":
		var t struct {
			Font     string  `json:"font"`
			Size     float64 `json:"size"`
			Weight   int     `json:"weight"`
			Color    string  `json:"color"`
			Align    string  `json:"align"`
			MaxLines int     `json:"max_lines"`
		}
		_ = json.Unmarshal(raw, &t)
		if t.Font == "" || t.Size <= 0 || t.Weight <= 0 || t.Color == "" || t.MaxLines <= 0 ||
			(t.Align != "left" && t.Align != "center" && t.Align != "right") {
			fail("font, size, weight, color, align (left, center or right) and max_lines are required")
		}
	case "transition":
		var t struct {
			Duration *float64 `json:"duration"`
		}
		_ = json.Unmarshal(raw, &t)
		if t.Duration == nil || *t.Duration < 0 {
			fail("duration (seconds, 0 or more) is required")
		}
	}
	return n == 0
}

func validSpotName(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' && i > 0 {
			continue
		}
		return false
	}
	return true
}

// VerifyFiles checks that every file the catalog references exists: the rig, entry files and
// thumbnails (relative to dir) and the font files (absolute /fonts/ paths relative to the root
// of fsys). fsys is the ui tree and dir the catalog directory in it ("img/storymaker").
func (c *Catalog) VerifyFiles(fsys fs.FS, dir string) error {
	var missing []string
	check := func(p string) {
		if !fs.ValidPath(p) {
			missing = append(missing, p+" (invalid path)")
			return
		}
		if st, err := fs.Stat(fsys, p); err != nil || st.IsDir() {
			missing = append(missing, p)
		}
	}
	check(path.Join(dir, c.Rig.File))
	for _, e := range c.entries {
		if e.Kind == "palette" {
			continue
		}
		if e.File != "" {
			check(path.Join(dir, e.File))
		}
		check(path.Join(dir, e.Thumb))
	}
	for _, f := range c.Fonts {
		check(strings.TrimPrefix(f.File, "/"))
	}
	if len(missing) == 0 {
		return nil
	}
	shown := missing
	if len(shown) > 12 {
		shown = append(append([]string(nil), shown[:12]...), fmt.Sprintf("... and %d more", len(missing)-12))
	}
	return fmt.Errorf("storymaker: catalog references %d missing files:\n - %s", len(missing), strings.Join(shown, "\n - "))
}

// Version is the catalog version string (a hash of the output files).
func (c *Catalog) Version() string { return c.version }

// Entries returns all entries in catalog order (palettes first).
func (c *Catalog) Entries() []Entry { return append([]Entry(nil), c.entries...) }

// Lookup finds an entry by its full ID.
func (c *Catalog) Lookup(id string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	i, ok := c.byID[id]
	if !ok {
		return Entry{}, false
	}
	return c.entries[i], true
}

// Names returns the bare names of an enum-like kind (pose, gesture, expression, mouth, cycle,
// text_style, transition, palette) or the font names for "font". For the other kinds it returns
// the IDs. The order is the catalog order. An unknown kind returns nil.
func (c *Catalog) Names(kind string) []string {
	if c == nil {
		return nil
	}
	kind = NormalizeKind(kind)
	if kind == "font" {
		seen := map[string]bool{}
		var out []string
		for _, f := range c.Fonts {
			if !seen[f.Name] {
				seen[f.Name] = true
				out = append(out, f.Name)
			}
		}
		return out
	}
	var out []string
	for _, e := range c.entries {
		if e.Kind != kind {
			continue
		}
		if enumKinds[kind] {
			out = append(out, e.Name)
		} else {
			out = append(out, e.ID)
		}
	}
	return out
}

// HasName reports whether an enum-like kind has the bare name.
func (c *Catalog) HasName(kind, name string) bool {
	if c == nil {
		return false
	}
	prefix, ok := kindPrefixes[NormalizeKind(kind)]
	if !ok {
		return false
	}
	_, found := c.byID[prefix+name]
	return found
}

const (
	defaultSearchLimit = 6
	maxSearchLimit     = 12
)

// Search finds entries by terms over their id, name and tags. kind restricts the result (a kind,
// alias or ID prefix; an unknown kind finds nothing). limit defaults to 6 and is capped at 12.
// Without terms it lists the entries of the kind in catalog order. Results are sorted by score,
// then by catalog order.
func (c *Catalog) Search(query, kind string, limit int) []Entry {
	if c == nil {
		return nil
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	limit = min(limit, maxSearchLimit)
	wanted := strings.TrimSpace(kind)
	kind = NormalizeKind(kind)
	if wanted != "" && kind == "" {
		return nil
	}
	terms := strings.Fields(strings.ToLower(query))
	type scored struct {
		e     Entry
		score int
		order int
	}
	var hits []scored
	for i, e := range c.entries {
		if kind != "" && e.Kind != kind {
			continue
		}
		score := 1
		if len(terms) > 0 {
			score = 0
			for _, t := range terms {
				score += termScore(e, t)
			}
			if score == 0 {
				continue
			}
		}
		hits = append(hits, scored{e, score, i})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].order < hits[j].order
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Entry, len(hits))
	for i, h := range hits {
		out[i] = h.e
	}
	return out
}

// termScore scores one lower-case term against an entry: exact id or name, tag, then substrings.
func termScore(e Entry, term string) int {
	best := 0
	try := func(t string) {
		s := 0
		lowID, lowName := strings.ToLower(e.ID), strings.ToLower(e.Name)
		switch {
		case lowID == t:
			s = 10
		case lowName == t:
			s = 8
		}
		if s == 0 {
			for _, tag := range e.Tags {
				lt := strings.ToLower(tag)
				if lt == t {
					s = max(s, 4)
				} else if strings.Contains(lt, t) {
					s = max(s, 2)
				}
			}
			if strings.Contains(lowName, t) {
				s = max(s, 3)
			}
			if strings.Contains(lowID, t) {
				s = max(s, 2)
			}
		}
		best = max(best, s)
	}
	try(term)
	if best == 0 && len(term) > 3 && strings.HasSuffix(term, "s") {
		try(strings.TrimSuffix(term, "s"))
	}
	return best
}

// defaultBackgroundSpots are the spots of a project or library background (my.bg.*, lib.bg.*).
var defaultBackgroundSpots = map[string]Spot{
	"left": {X: 30, Y: 88}, "center": {X: 50, Y: 88}, "right": {X: 70, Y: 88},
}

// isBackgroundAsset reports a project or library background ID.
func isBackgroundAsset(id string) bool {
	return strings.HasPrefix(id, "my.bg.") || strings.HasPrefix(id, "lib.bg.")
}

// Spots returns the named spots of an environment for an aspect ("16:9", "9:16", "1:1"); the
// aspect overrides replace single spots. A project or library background (my.bg.*, lib.bg.*)
// has the default spots left, center and right. It reports false for an unknown environment.
func (c *Catalog) Spots(envID, aspect string) (map[string]Spot, bool) {
	if isBackgroundAsset(envID) {
		out := make(map[string]Spot, len(defaultBackgroundSpots))
		for k, v := range defaultBackgroundSpots {
			out[k] = v
		}
		return out, true
	}
	e, ok := c.Lookup(envID)
	if !ok || e.Kind != "environment" {
		return nil, false
	}
	out := make(map[string]Spot, len(e.Spots))
	for k, v := range e.Spots {
		out[k] = v
	}
	if ov, ok := e.Aspects[aspect]; ok {
		for k, v := range ov.Spots {
			out[k] = v
		}
	}
	return out, true
}

// SpotNames lists the spot names of an environment: the five base spots first, then the
// environment's own spots in alphabetical order.
func (c *Catalog) SpotNames(envID string) []string {
	spots, ok := c.Spots(envID, "16:9")
	if !ok {
		return nil
	}
	base := []string{"far-left", "left", "center", "right", "far-right"}
	var out, rest []string
	for _, b := range base {
		if _, ok := spots[b]; ok {
			out = append(out, b)
		}
	}
	for name := range spots {
		if !containsName(base, name) {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// FocusX is the horizontal focus (percent) of an environment for an aspect; 50 when unknown.
func (c *Catalog) FocusX(envID, aspect string) float64 {
	if e, ok := c.Lookup(envID); ok {
		if ov, ok := e.Aspects[aspect]; ok {
			return ov.FocusX
		}
	}
	return 50
}

// Variants lists the variants (day, night) an environment supports.
func (c *Catalog) Variants(envID string) []string {
	if isBackgroundAsset(envID) {
		return []string{"day", "night"}
	}
	if e, ok := c.Lookup(envID); ok {
		return e.Variants
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/storymaker/ -run 'Catalog' -count=1`, then `go test ./internal/storymaker/ -count=1`
Expected: PASS (`ok  aurago/internal/storymaker`)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/catalog.go internal/storymaker/testdata/catalog-mini/catalog.json internal/storymaker/catalog_test.go
git commit -m "feat(storymaker): catalog index with search and spot lookup" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A4: Defaults and issue helpers

**Files:**
- Create: `internal/storymaker/defaults.go`
- Create: `internal/storymaker/issues.go`
- Test: `internal/storymaker/defaults_test.go`
- Test: `internal/storymaker/issues_test.go`

- [ ] **Step 1: Write the failing test**

`internal/storymaker/defaults_test.go`:

```go
package storymaker

import (
	"reflect"
	"strings"
	"testing"
)

func appliedMap(d []DefaultApplied) map[string]any {
	m := map[string]any{}
	for _, a := range d {
		m[a.Path] = a.Value
	}
	return m
}

func TestApplyDefaultsTable(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},
	  "cast":[{"id":"anna","character":"char.office-woman-1"},{"id":"ben","name":"Ben","character":"char.office-man-1"}],
	  "scenes":[{"id":"s1","narration":"Hallo."},
	            {"id":"s2","environment":"env.office","actors":[{"cast":"anna"}],"props":[{"item":"prop.lock"}]},
	            {"id":"s3","actors":[{"cast":"ben"},{"cast":"anna"}]}]}`)
	got := ApplyDefaults(&s)
	m := appliedMap(got)
	want := map[string]any{
		// scene 1: first scene
		"/scenes/0/environment": "env.plain-stage", "/scenes/0/variant": "day", "/scenes/0/speaker": "narrator",
		"/scenes/0/transition": "cut", "/scenes/0/duration": "auto", "/scenes/0/camera": map[string]any{"framing": "wide"},
		// scene 2: one actor at center, prop layer front
		"/scenes/1/variant": "day", "/scenes/1/speaker": "narrator", "/scenes/1/transition": "fade", "/scenes/1/duration": "auto",
		"/scenes/1/camera": map[string]any{"framing": "wide"}, "/scenes/1/actors/0/at": "center", "/scenes/1/actors/0/pose": "stand",
		"/scenes/1/actors/0/facing": "auto", "/scenes/1/actors/0/expression": "neutral", "/scenes/1/actors/0/scale": 1,
		"/scenes/1/props/0/layer": "front",
		// scene 3: inherits env.office; two actors take left/right in cast order (anna first, whatever the scene order)
		"/scenes/2/environment": "env.office", "/scenes/2/variant": "day", "/scenes/2/speaker": "narrator", "/scenes/2/transition": "fade",
		"/scenes/2/duration": "auto", "/scenes/2/camera": map[string]any{"framing": "wide"},
		"/scenes/2/actors/0/at": "right", "/scenes/2/actors/0/pose": "stand", "/scenes/2/actors/0/facing": "auto", "/scenes/2/actors/0/expression": "neutral", "/scenes/2/actors/0/scale": 1,
		"/scenes/2/actors/1/at": "left", "/scenes/2/actors/1/pose": "stand", "/scenes/2/actors/1/facing": "auto", "/scenes/2/actors/1/expression": "neutral", "/scenes/2/actors/1/scale": 1,
		// cast name = id (only where omitted)
		"/cast/0/name": "anna",
	}
	if !reflect.DeepEqual(m, want) {
		for k, v := range want {
			if !reflect.DeepEqual(m[k], v) {
				t.Errorf("%s = %#v, want %#v", k, m[k], v)
			}
		}
		for k, v := range m {
			if _, ok := want[k]; !ok {
				t.Errorf("unexpected default %s = %#v", k, v)
			}
		}
	}
	if s.Scenes[1].Environment != "env.office" || s.Scenes[2].Actors[0].At.Spot != "right" || s.Cast[0].Name != "anna" || s.Cast[1].Name != "Ben" {
		t.Errorf("story not updated: %+v", s)
	}
	if !s.Scenes[0].Duration.Auto || s.Scenes[0].Camera.Framing != "wide" {
		t.Errorf("duration/camera: %+v %+v", s.Scenes[0].Duration, s.Scenes[0].Camera)
	}
}

func TestApplyDefaultsKeepsExplicitValues(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","format":{"aspect":"9:16","fps":24},"style":{"palette":"calm","font":"default","captions":false},
	  "cast":[{"id":"anna","name":"Ann","character":"char.office-woman-1"}],
	  "scenes":[{"id":"s1","environment":"env.office","variant":"night","speaker":"anna","transition":"wipe","duration":5,"camera":{"framing":"close"},
	    "actors":[{"cast":"anna","at":{"x":10,"y":90},"pose":"sit","facing":"left","expression":"happy","scale":1.5}],
	    "props":[{"item":"prop.lock","layer":"back"}]}]}`)
	before, _ := CanonicalJSON(s)
	got := ApplyDefaults(&s)
	after, _ := CanonicalJSON(s)
	if len(got) != 0 || string(before) != string(after) {
		t.Fatalf("explicit values changed: %v\n%s\n%s", got, before, after)
	}
}

func TestApplyDefaultsIsIdempotentAndNeverTouchesTiming(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","scenes":[{"id":"s1","actors":[{"cast":"a"}],"beats":[{"do":"wait","seconds":2},{"do":"show","text":"x","at":"left","entry":"fade"}]}]}`)
	first := ApplyDefaults(&s)
	if len(first) == 0 {
		t.Fatal("nothing applied")
	}
	if again := ApplyDefaults(&s); len(again) != 0 {
		t.Fatalf("second call applied %v", again)
	}
	b := s.Scenes[0].Beats
	if b[0].Delay != 0 || b[0].Duration != 0 || b[0].Ease != "" || b[1].Delay != 0 || b[1].Duration != 0 || b[1].Ease != "" || b[0].Seconds != 2 {
		t.Fatalf("timing was touched: %+v", b)
	}
	for _, a := range first {
		if !strings.Contains(a.Path, "/beats/") {
			continue
		}
		if !strings.HasSuffix(a.Path, "/id") {
			t.Fatalf("a beat default other than the show id: %v", a)
		}
	}
}

func TestApplyDefaultsFillsDocumentLevelFields(t *testing.T) {
	var s Story
	got := appliedMap(ApplyDefaults(&s))
	want := map[string]any{
		"/schema": Schema, "/format/aspect": "16:9", "/format/fps": 30,
		"/style": map[string]any{"palette": "sunny", "font": "default", "captions": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	// A style with only a palette keeps the author's captions choice (false here, since the
	// story was built in code) and fills font.
	s2 := Story{Style: Style{Palette: "calm"}}
	m := appliedMap(ApplyDefaults(&s2))
	if m["/style/font"] != "default" || s2.Style.Palette != "calm" {
		t.Fatalf("%#v", m)
	}
}

func TestApplyDefaultsActorSlots(t *testing.T) {
	cast := `"cast":[{"id":"a","character":"char.x"},{"id":"b","character":"char.x"},{"id":"c","character":"char.x"},{"id":"d","character":"char.x"},{"id":"e","character":"char.x"}]`
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T",`+cast+`,"scenes":[
	  {"id":"s1","actors":[{"cast":"a"},{"cast":"b"},{"cast":"c"},{"cast":"d"}]},
	  {"id":"s2","actors":[{"cast":"d","at":"desk"},{"cast":"b"}]},
	  {"id":"s3","actors":[{"cast":"unknown"},{"cast":"a"}]}]}`)
	ApplyDefaults(&s)
	slots := func(i int) []string {
		var out []string
		for _, a := range s.Scenes[i].Actors {
			out = append(out, a.At.Spot)
		}
		return out
	}
	if got := slots(0); !reflect.DeepEqual(got, []string{"left", "right", "far-left", "far-right"}) {
		t.Errorf("four actors: %v", got)
	}
	// Explicit positions stay; the other actor keeps the slot of its rank (b is the first by cast order).
	if got := slots(1); !reflect.DeepEqual(got, []string{"desk", "left"}) {
		t.Errorf("explicit position kept, rank slot for the rest: %v", got)
	}
	// An actor of an unknown cast member ranks last.
	if got := slots(2); !reflect.DeepEqual(got, []string{"right", "left"}) {
		t.Errorf("unknown cast last: %v", got)
	}
}

func TestApplyDefaultsShowBeatInstanceIDs(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","scenes":[{"id":"s1",
	  "props":[{"id":"lock-1","item":"prop.lock"}],
	  "beats":[
	    {"do":"show","item":"prop.lock","at":"left","entry":"pop"},
	    {"do":"show","item":"prop.lock","at":"right","entry":"pop"},
	    {"do":"show","item":"prop.shield","at":"center","entry":"fade"},
	    {"do":"show","item":"my.prop.red-box","at":"center","entry":"fade"},
	    {"do":"show","text":"Hallo","at":"center","entry":"fade"},
	    {"do":"show","icon":"Warning Sign","at":"center","entry":"fade"},
	    {"do":"show","item":"prop.shield","id":"keep","at":"center","entry":"fade"},
	    {"do":"wait","seconds":1}]}]}`)
	ApplyDefaults(&s)
	var ids []string
	for _, b := range s.Scenes[0].Beats {
		ids = append(ids, b.ID)
	}
	// lock-1 is taken by the prop, so the first shown lock is lock-2; ids count per item.
	want := []string{"lock-2", "lock-3", "shield-1", "red-box-1", "text-1", "warning-sign-1", "keep", ""}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for _, id := range ids {
		if id != "" && !validSlug(id) {
			t.Errorf("%q is not a slug", id)
		}
	}
}
```

`internal/storymaker/issues_test.go`:

```go
package storymaker

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCapIssues(t *testing.T) {
	var issues []Issue
	for i := 0; i < 5; i++ {
		issues = append(issues, Issue{Severity: SeverityMinor, Path: fmt.Sprintf("/m%d", i)})
	}
	issues = append(issues, Issue{Severity: SeverityError, Path: "/e1"}, Issue{Severity: SeverityWarning, Path: "/w1"}, Issue{Severity: SeverityError, Path: "/e2"})
	got := CapIssues(issues, 3)
	if len(got) != 3 || got[0].Path != "/e1" || got[1].Path != "/w1" || got[2].Path != "/e2" {
		t.Fatalf("errors first, then warnings, in document order: %+v", got)
	}
	if got := CapIssues(issues, 0); len(got) != len(issues) {
		t.Fatal("n <= 0 must keep everything")
	}
	if got := CapIssues(issues, MaxModelIssues); len(got) != MaxModelIssues {
		t.Fatalf("model cap: %d", len(got))
	}
}

func TestClosestNames(t *testing.T) {
	pool := []string{"stand", "sit", "typing", "point-left", "point-right", "wave", "think"}
	cases := []struct {
		input string
		want  []string // the best matches, in order; the rest is filler
	}{
		{"typin", []string{"typing"}},
		{"sitting", []string{"sit"}},
		{"pointing", []string{"point-left", "point-right"}},
		{"pose.wave", []string{"wave"}},
		{"Point_Left", []string{"point-left", "point-right"}},
	}
	for _, tc := range cases {
		got := closestNames(tc.input, pool, 3)
		if len(got) != 3 || !reflect.DeepEqual(got[:len(tc.want)], tc.want) {
			t.Errorf("closestNames(%q) = %v, want it to start with %v", tc.input, got, tc.want)
		}
	}
	if got := closestNames("x", nil, 5); len(got) != 0 {
		t.Errorf("empty pool: %v", got)
	}
	if got := closestNames("a", []string{"a", "a", "b"}, 5); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("duplicates: %v", got)
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) || HasErrors([]Issue{{Severity: SeverityWarning}, {Severity: SeverityMinor}}) || !HasErrors([]Issue{{Severity: SeverityMinor}, {Severity: SeverityError}}) {
		t.Fatal("HasErrors")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/ -run 'Defaults|CapIssues|ClosestNames|HasErrors' -count=1`
Expected: FAIL (build failed: `undefined: ApplyDefaults`)

- [ ] **Step 3: Implement**

`internal/storymaker/defaults.go`:

```go
package storymaker

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Defaults of the story format (design spec, section Defaults).
const (
	DefaultEnvironment = "env.plain-stage"
	DefaultAspect      = "16:9"
	DefaultFPS         = 30
	DefaultPalette     = "sunny"
	DefaultFont        = "default"
)

// actorSlots are the default spots of several actors, assigned in cast order.
var actorSlots = []string{"left", "right", "far-left", "far-right"}

// ApplyDefaults fills the static defaults of the spec's Defaults table into s, only for fields
// the author omitted, and lists each filled field. Timing (delay, duration, ease) is never
// defaulted: the player's compile step owns it. Calling it again changes nothing.
//
// Beyond the table it completes the document-level fields a bare story may lack: schema,
// format (16:9, 30 fps), style (sunny, default font, captions on) and the show-beat instance id.
//
// With several actors, each actor without a position takes the slot left, right, far-left,
// far-right (wrapping) that matches its rank in the story's cast list, so a character keeps its
// side across scenes; a single actor stands at center.
func ApplyDefaults(s *Story) []DefaultApplied {
	var out []DefaultApplied
	set := func(path string, value any) { out = append(out, DefaultApplied{Path: path, Value: value}) }

	if s.Schema == "" {
		s.Schema = Schema
		set("/schema", Schema)
	}
	if s.Format.Aspect == "" {
		s.Format.Aspect = DefaultAspect
		set("/format/aspect", DefaultAspect)
	}
	if s.Format.FPS == 0 {
		s.Format.FPS = DefaultFPS
		set("/format/fps", DefaultFPS)
	}
	if s.Style == (Style{}) {
		s.Style = Style{Palette: DefaultPalette, Font: DefaultFont, Captions: true}
		set("/style", map[string]any{"palette": DefaultPalette, "font": DefaultFont, "captions": true})
	} else {
		if s.Style.Palette == "" {
			s.Style.Palette = DefaultPalette
			set("/style/palette", DefaultPalette)
		}
		if s.Style.Font == "" {
			s.Style.Font = DefaultFont
			set("/style/font", DefaultFont)
		}
	}
	for i := range s.Cast {
		if s.Cast[i].Name == "" && s.Cast[i].ID != "" {
			s.Cast[i].Name = s.Cast[i].ID
			set(jp("cast", i, "name"), s.Cast[i].ID)
		}
	}

	prevEnv := ""
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		if sc.Environment == "" {
			sc.Environment = prevEnv
			if sc.Environment == "" {
				sc.Environment = DefaultEnvironment
			}
			set(jp("scenes", i, "environment"), sc.Environment)
		}
		prevEnv = sc.Environment
		if sc.Variant == "" {
			sc.Variant = "day"
			set(jp("scenes", i, "variant"), "day")
		}
		if sc.Speaker == "" {
			sc.Speaker = "narrator"
			set(jp("scenes", i, "speaker"), "narrator")
		}
		if sc.Transition == "" {
			sc.Transition = "fade"
			if i == 0 {
				sc.Transition = "cut"
			}
			set(jp("scenes", i, "transition"), sc.Transition)
		}
		if sc.Duration == nil {
			sc.Duration = AutoDuration()
			set(jp("scenes", i, "duration"), "auto")
		}
		if sc.Camera == nil {
			sc.Camera = &Camera{Framing: "wide"}
			set(jp("scenes", i, "camera"), map[string]any{"framing": "wide"})
		} else if sc.Camera.Framing == "" {
			sc.Camera.Framing = "wide"
			set(jp("scenes", i, "camera", "framing"), "wide")
		}

		ranks := actorRanks(s.Cast, sc.Actors)
		for j := range sc.Actors {
			a := &sc.Actors[j]
			if a.At.IsZero() {
				slot := "center"
				if len(sc.Actors) > 1 {
					slot = actorSlots[ranks[j]%len(actorSlots)]
				}
				a.At = SpotAt(slot)
				set(jp("scenes", i, "actors", j, "at"), slot)
			}
			if a.Pose == "" {
				a.Pose = "stand"
				set(jp("scenes", i, "actors", j, "pose"), "stand")
			}
			if a.Facing == "" {
				a.Facing = "auto"
				set(jp("scenes", i, "actors", j, "facing"), "auto")
			}
			if a.Expression == "" {
				a.Expression = "neutral"
				set(jp("scenes", i, "actors", j, "expression"), "neutral")
			}
			if a.Scale == 0 {
				a.Scale = 1
				set(jp("scenes", i, "actors", j, "scale"), 1)
			}
		}
		for j := range sc.Props {
			if sc.Props[j].Layer == "" {
				sc.Props[j].Layer = "front"
				set(jp("scenes", i, "props", j, "layer"), "front")
			}
		}
		for j, id := range deriveShowIDs(sc) {
			if id != "" {
				sc.Beats[j].ID = id
				set(jp("scenes", i, "beats", j, "id"), id)
			}
		}
	}
	return out
}

// actorRanks returns, per actor, its rank in cast order (actors of unknown cast members come
// last, in scene order).
func actorRanks(cast []CastMember, actors []Actor) []int {
	idx := make([]int, len(actors))
	for j := range idx {
		idx[j] = j
	}
	order := func(j int) int {
		for k, c := range cast {
			if c.ID == actors[j].Cast {
				return k
			}
		}
		return len(cast)
	}
	sort.SliceStable(idx, func(a, b int) bool { return order(idx[a]) < order(idx[b]) })
	ranks := make([]int, len(actors))
	for rank, j := range idx {
		ranks[j] = rank
	}
	return ranks
}

// deriveShowIDs returns, aligned with sc.Beats, the instance id a show beat without an id
// would get (<item-slug>-<n>), and "" for every other beat. It skips ids already taken by props
// and by beats with an explicit id, so ids stay unique.
func deriveShowIDs(sc *Scene) []string {
	if len(sc.Beats) == 0 {
		return nil
	}
	used := map[string]bool{}
	for _, p := range sc.Props {
		if p.ID != "" {
			used[p.ID] = true
		}
	}
	for _, b := range sc.Beats {
		if b.ID != "" {
			used[b.ID] = true
		}
	}
	counters := map[string]int{}
	out := make([]string, len(sc.Beats))
	for j, b := range sc.Beats {
		if b.Do != "show" || b.ID != "" {
			continue
		}
		slug := showSlug(b)
		n := counters[slug] + 1
		id := slug + "-" + strconv.Itoa(n)
		for used[id] {
			n++
			id = slug + "-" + strconv.Itoa(n)
		}
		counters[slug] = n
		used[id] = true
		out[j] = id
	}
	return out
}

// showSlug names what a show beat shows: the item without its prefix, or text, icon or chart.
func showSlug(b Beat) string {
	switch {
	case b.Item != "":
		return assetSlug(b.Item)
	case b.Text != "":
		return "text"
	case b.Icon != "":
		return cleanSlug(b.Icon, "icon")
	case b.Chart != "":
		return "chart"
	}
	return "show"
}

// assetSlug turns prop.lock into lock and my.prop.red-box into red-box.
func assetSlug(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) > 1 {
		if (parts[0] == "my" || parts[0] == "lib") && len(parts) > 2 {
			parts = parts[2:]
		} else {
			parts = parts[1:]
		}
	}
	return cleanSlug(strings.Join(parts, "-"), "item")
}

// cleanSlug lower-cases s, replaces runs of other characters by one dash and limits the length
// to 40 characters so that "<slug>-<n>" stays a valid id.
func cleanSlug(s, fallback string) string {
	var b strings.Builder
	dash := true
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		return fallback
	}
	return out
}

// jp builds a JSON Pointer (RFC 6901) from path segments.
func jp(parts ...any) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteByte('/')
		switch v := p.(type) {
		case int:
			b.WriteString(strconv.Itoa(v))
		case string:
			b.WriteString(escapePointer(v))
		default:
			b.WriteString(escapePointer(fmt.Sprint(v)))
		}
	}
	return b.String()
}
```

`internal/storymaker/issues.go`:

```go
package storymaker

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

// MaxModelIssues is the number of issues a model-facing answer carries.
const MaxModelIssues = 8

// MaxHTTPIssues caps the issues of an HTTP save answer.
const MaxHTTPIssues = 200

// CapIssues keeps at most n issues, preferring errors over warnings over minor notes, and
// returns them in their original (document) order. n <= 0 returns the issues unchanged.
func CapIssues(issues []Issue, n int) []Issue {
	if n <= 0 || len(issues) <= n {
		return issues
	}
	keep := make([]bool, len(issues))
	left := n
	for _, sev := range []string{SeverityError, SeverityWarning, SeverityMinor} {
		for i, is := range issues {
			if left == 0 {
				break
			}
			if is.Severity == sev && !keep[i] {
				keep[i] = true
				left--
			}
		}
	}
	out := make([]Issue, 0, n)
	for i, is := range issues {
		if keep[i] {
			out = append(out, is)
		}
	}
	return out
}

// HasErrors reports whether any issue is an error (renders are blocked).
func HasErrors(issues []Issue) bool {
	for _, is := range issues {
		if is.Severity == SeverityError {
			return true
		}
	}
	return false
}

// exampleJSON renders {"field":"value"} for the fix text of an issue.
func exampleJSON(field, value string) string {
	b, _ := json.Marshal(map[string]string{field: value})
	return string(b)
}

// closestNames returns up to n entries of pool ranked by closeness to input: equal ignoring
// case and dashes, then one containing the other, then by edit distance. Ties keep pool order.
func closestNames(input string, pool []string, n int) []string {
	norm := func(s string) string {
		s = strings.ToLower(s)
		if i := strings.LastIndexByte(s, '.'); i >= 0 {
			s = s[i+1:]
		}
		return strings.Map(func(r rune) rune {
			if r == '-' || r == '_' || r == ' ' {
				return -1
			}
			return r
		}, s)
	}
	in := norm(input)
	type cand struct {
		name  string
		score int
		order int
	}
	cands := make([]cand, 0, len(pool))
	seen := map[string]bool{}
	for i, p := range pool {
		if seen[p] {
			continue
		}
		seen[p] = true
		c := norm(p)
		score := 0
		switch {
		case c == in:
			score = 0
		case in != "" && (strings.Contains(c, in) || strings.Contains(in, c)):
			score = 1
		default:
			score = 2 + levenshtein(in, c)
		}
		cands = append(cands, cand{p, score, i})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score < cands[j].score })
	if len(cands) > n {
		cands = cands[:n]
	}
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.name
	}
	return out
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// splitTerms splits an asset id or name into lower-case search terms.
func splitTerms(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/storymaker/ -run 'Defaults|CapIssues|ClosestNames|HasErrors' -count=1`, then `go test ./internal/storymaker/ -count=1`
Expected: PASS (`ok  aurago/internal/storymaker`)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/defaults.go internal/storymaker/issues.go internal/storymaker/defaults_test.go internal/storymaker/issues_test.go
git commit -m "feat(storymaker): static defaults and issue helpers" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A5: Validation

**Files:**
- Create: `internal/storymaker/validate.go`
- Test: `internal/storymaker/validate_test.go`

- [ ] **Step 1: Write the failing test**

`internal/storymaker/validate_test.go`:

```go
package storymaker

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// storyOf wraps scene JSON into a valid story with one cast member (anna).
func storyOf(t *testing.T, scenes string) Story {
	t.Helper()
	return mustDecode(t, `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},`+
		`"style":{"palette":"sunny","font":"default","captions":true},`+
		`"cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],"scenes":[`+scenes+`]}`)
}

func validateScenes(t *testing.T, scenes string) []Issue {
	t.Helper()
	s := storyOf(t, scenes)
	return Validate(&s, loadMini(t), DefaultLimits())
}

func findIssue(issues []Issue, code, severity, path string) *Issue {
	for i := range issues {
		if issues[i].Code == code && issues[i].Severity == severity && issues[i].Path == path {
			return &issues[i]
		}
	}
	return nil
}

func dump(issues []Issue) string {
	var b strings.Builder
	for _, is := range issues {
		fmt.Fprintf(&b, "\n  %s %s %s: %s | %s", is.Severity, is.Code, is.Path, is.Message, is.Fix)
	}
	return b.String()
}

func expectIssue(t *testing.T, issues []Issue, code, severity, path string) *Issue {
	t.Helper()
	is := findIssue(issues, code, severity, path)
	if is == nil {
		t.Fatalf("no %s %s at %s in:%s", severity, code, path, dump(issues))
	}
	return is
}

func TestValidateFixturesAreClean(t *testing.T) {
	cat := loadMini(t)
	for _, name := range []string{"minimal", "firewall"} {
		data, _ := os.ReadFile("testdata/stories/" + name + ".json")
		s := mustDecode(t, string(data))
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s: %s", name, dump(issues))
		}
		ApplyDefaults(&s)
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s after defaults: %s", name, dump(issues))
		}
	}
}

func TestValidateIssueShape(t *testing.T) {
	issues := validateScenes(t, `{"id":"s1","environment":"env.offce","narration":"x","actors":[{"cast":"anna","at":"balcony"}]}`)
	for _, is := range issues {
		if is.Severity == "" || is.Code == "" || is.Message == "" || !strings.HasPrefix(is.Path, "/") {
			t.Errorf("incomplete issue %+v", is)
		}
		if is.SceneID != "s1" {
			t.Errorf("scene id missing: %+v", is)
		}
	}
}

func TestValidateReferences(t *testing.T) {
	cases := []struct {
		name, scenes, code, severity, path string
		fix                                string // substring of the fix
	}{
		{"unknown environment", `{"id":"s1","environment":"env.offce"}`, CodeUnknownAsset, SeverityError, "/scenes/0/environment", "env.office"},
		{"environment without prefix", `{"id":"s1","environment":"office"}`, CodeUnknownAsset, SeverityError, "/scenes/0/environment", "env.office"},
		{"environment wrong kind", `{"id":"s1","environment":"prop.lock"}`, CodeInvalidValue, SeverityError, "/scenes/0/environment", "env."},
		{"unknown prop item", `{"id":"s1","props":[{"item":"prop.padlock"}]}`, CodeUnknownAsset, SeverityError, "/scenes/0/props/0/item", "prop.lock"},
		{"prop missing item", `{"id":"s1","props":[{"at":"left"}]}`, CodeMissingField, SeverityError, "/scenes/0/props/0/item", ""},
		{"unknown show item", `{"id":"s1","beats":[{"do":"show","item":"prop.rocket","at":"left","entry":"pop"}]}`, CodeUnknownAsset, SeverityError, "/scenes/0/beats/0/item", "prop."},
		{"unknown spot", `{"id":"s1","environment":"env.office","actors":[{"cast":"anna","at":"balcony"}]}`, CodeUnknownSpot, SeverityError, "/scenes/0/actors/0/at", "desk"},
		{"spot of another environment", `{"id":"s1","environment":"env.plain-stage","actors":[{"cast":"anna","at":"desk"}]}`, CodeUnknownSpot, SeverityError, "/scenes/0/actors/0/at", "center"},
		{"spot inherited from the previous scene", `{"id":"s1","environment":"env.office"},{"id":"s2","actors":[{"cast":"anna","at":"desk"}]}`, "", "", "", ""},
		{"project background keeps three spots", `{"id":"s1","environment":"my.bg.park","actors":[{"cast":"anna","at":"desk"}]}`, CodeUnknownSpot, SeverityError, "/scenes/0/actors/0/at", "left"},
		{"point out of range", `{"id":"s1","actors":[{"cast":"anna","at":{"x":120,"y":50}}]}`, CodeInvalidValue, SeverityError, "/scenes/0/actors/0/at", ""},
		{"position wrong shape", `{"id":"s1","actors":[{"cast":"anna","at":{"x":50}}]}`, CodeInvalidValue, SeverityError, "/scenes/0/actors/0/at", "center"},
		{"empty spot", `{"id":"s1","actors":[{"cast":"anna","at":""}]}`, CodeInvalidValue, SeverityError, "/scenes/0/actors/0/at", ""},
		{"actor of unknown cast", `{"id":"s1","actors":[{"cast":"bob"}]}`, CodeUnknownCast, SeverityError, "/scenes/0/actors/0/cast", "anna"},
		{"actor without cast", `{"id":"s1","actors":[{"at":"left"}]}`, CodeMissingField, SeverityError, "/scenes/0/actors/0/cast", ""},
		{"same actor twice", `{"id":"s1","actors":[{"cast":"anna"},{"cast":"anna"}]}`, CodeDuplicateID, SeverityError, "/scenes/0/actors/1/cast", ""},
		{"unknown speaker", `{"id":"s1","speaker":"bob"}`, CodeUnknownCast, SeverityError, "/scenes/0/speaker", "anna"},
		{"beat of unknown cast", `{"id":"s1","beats":[{"do":"pose","who":"bob","pose":"stand"}]}`, CodeUnknownCast, SeverityError, "/scenes/0/beats/0/who", "anna"},
		{"beat of an actor who is not on stage", `{"id":"s1","beats":[{"do":"pose","who":"anna","pose":"stand"}]}`, CodeUnknownCast, SeverityWarning, "/scenes/0/beats/0/who", "enter"},
		{"enter puts the actor on stage", `{"id":"s1","beats":[{"do":"enter","who":"anna","from":"left","to":"center"},{"do":"pose","who":"anna","pose":"stand"}]}`, "", "", "", ""},
		{"exit takes the actor off stage", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"exit","who":"anna","to":"left"},{"do":"pose","who":"anna","pose":"stand"}]}`, CodeUnknownCast, SeverityWarning, "/scenes/0/beats/1/who", ""},
		{"cue without marker", `{"id":"s1","narration":"Eins. {1}Zwei.","beats":[{"do":"wait","seconds":1,"cue":2}]}`, CodeUnknownCue, SeverityError, "/scenes/0/beats/0/cue", "{1}"},
		{"cue without any narration", `{"id":"s1","beats":[{"do":"wait","seconds":1,"cue":1}]}`, CodeUnknownCue, SeverityError, "/scenes/0/beats/0/cue", "no markers"},
		{"cue with marker", `{"id":"s1","narration":"Eins. {2}Zwei.","beats":[{"do":"wait","seconds":1,"cue":2}]}`, "", "", "", ""},
		{"target outside the scene", `{"id":"s1","beats":[{"do":"hide","target":"prop.lock"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/target", "camera"},
		{"target is an item shown by a beat", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"},{"do":"hide","target":"prop.lock"}]}`, "", "", "", ""},
		{"target is the derived instance id", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"},{"do":"hide","target":"lock-1"}]}`, "", "", "", ""},
		{"ambiguous item target", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"left","entry":"pop"},{"do":"show","item":"prop.lock","at":"right","entry":"pop"},{"do":"hide","target":"prop.lock"}]}`, CodeInvalidValue, SeverityError, "/scenes/0/beats/2/target", "lock-1"},
		{"look at a spot", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"look","who":"anna","target":"center"}]}`, "", "", "", ""},
		{"duplicate scene id", `{"id":"s1"},{"id":"s1"}`, CodeDuplicateID, SeverityError, "/scenes/1/id", ""},
		{"bad scene id", `{"id":"Scene 1"}`, CodeInvalidValue, SeverityError, "/scenes/0/id", ""},
		{"missing scene id", `{"narration":"x"}`, CodeMissingField, SeverityError, "/scenes/0/id", ""},
		{"duplicate prop id", `{"id":"s1","props":[{"id":"a","item":"prop.lock"},{"id":"a","item":"prop.shield"}]}`, CodeDuplicateID, SeverityError, "/scenes/0/props/1/id", ""},
		{"show id collides with a prop id", `{"id":"s1","props":[{"id":"a","item":"prop.lock"}],"beats":[{"do":"show","id":"a","item":"prop.shield","at":"left","entry":"pop"}]}`, CodeDuplicateID, SeverityError, "/scenes/0/beats/0/id", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := validateScenes(t, tc.scenes)
			if tc.code == "" {
				for _, is := range issues {
					if is.Severity != SeverityMinor {
						t.Fatalf("unexpected issues:%s", dump(issues))
					}
				}
				return
			}
			is := expectIssue(t, issues, tc.code, tc.severity, tc.path)
			if !strings.Contains(is.Fix, tc.fix) {
				t.Fatalf("fix %q lacks %q", is.Fix, tc.fix)
			}
		})
	}
}

func TestValidateAlternativesAreLimitedAndInTheFieldForm(t *testing.T) {
	issues := validateScenes(t, `{"id":"s1","actors":[{"cast":"anna","pose":"typin","expression":"suprised"}],"beats":[{"do":"dance","who":"anna"}]}`)
	pose := expectIssue(t, issues, CodeUnknownName, SeverityError, "/scenes/0/actors/0/pose")
	if !strings.Contains(pose.Fix, `Use one of: typing, stand.`) || !strings.Contains(pose.Fix, `{"pose":"typing"}`) {
		t.Errorf("pose fix: %s", pose.Fix)
	}
	expr := expectIssue(t, issues, CodeUnknownName, SeverityError, "/scenes/0/actors/0/expression")
	if !strings.Contains(expr.Fix, "surprised") || strings.Contains(expr.Fix, "expr.") {
		t.Errorf("expression fix must offer bare names: %s", expr.Fix)
	}
	do := expectIssue(t, issues, CodeUnknownName, SeverityError, "/scenes/0/beats/0/do")
	list := strings.TrimSuffix(strings.TrimPrefix(strings.SplitN(do.Fix, ". Example:", 2)[0], "Use one of: "), ".")
	if n := len(strings.Split(list, ", ")); n != 5 {
		t.Errorf("a beat kind offers at most 5 alternatives, got %d: %s", n, do.Fix)
	}
}

func TestValidateNames(t *testing.T) {
	cases := []struct {
		name, scenes, code, severity, path string
	}{
		// Closed names the catalog lacks: minor, never an error.
		{"pose missing from the catalog", `{"id":"s1","actors":[{"cast":"anna","pose":"facepalm"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/actors/0/pose"},
		{"expression missing from the catalog", `{"id":"s1","actors":[{"cast":"anna","expression":"laugh"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/actors/0/expression"},
		{"gesture missing from the catalog", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"gesture","who":"anna","gesture":"wave"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0/gesture"},
		// Unknown names: error.
		{"unknown pose", `{"id":"s1","actors":[{"cast":"anna","pose":"flying"}]}`, CodeUnknownName, SeverityError, "/scenes/0/actors/0/pose"},
		{"unknown gesture", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"gesture","who":"anna","gesture":"dab"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/gesture"},
		{"unknown transition", `{"id":"s1","transition":"spin"}`, CodeUnknownName, SeverityError, "/scenes/0/transition"},
		{"unknown ease", `{"id":"s1","beats":[{"do":"wait","seconds":1,"ease":"bouncy"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/ease"},
		{"unknown text style", `{"id":"s1","beats":[{"do":"show","text":"Hi","style":"huge","at":"center","entry":"fade"}]}`, CodeUnknownName, SeverityError, "/scenes/0/beats/0/style"},
		// ID form is canonicalised with a note.
		{"pose in id form", `{"id":"s1","actors":[{"cast":"anna","pose":"pose.typing"}]}`, CodeCanonicalised, SeverityMinor, "/scenes/0/actors/0/pose"},
		{"expression in id form", `{"id":"s1","actors":[{"cast":"anna","expression":"expr.happy"}]}`, CodeCanonicalised, SeverityMinor, "/scenes/0/actors/0/expression"},
		{"transition in id form", `{"id":"s1","transition":"trans.fade"}`, CodeCanonicalised, SeverityMinor, "/scenes/0/transition"},
		{"ease in id form", `{"id":"s1","beats":[{"do":"wait","seconds":1,"ease":"ease.linear"}]}`, CodeCanonicalised, SeverityMinor, "/scenes/0/beats/0/ease"},
		// The phase 1 support matrix.
		{"transition not played yet", `{"id":"s1","transition":"slide-right"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
		{"transition zoom", `{"id":"s1","transition":"zoom"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/transition"},
		{"say beat", `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"say","who":"anna","style":"speech","text":"Hi"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"highlight beat", `{"id":"s1","props":[{"id":"a","item":"prop.lock"}],"beats":[{"do":"highlight","target":"a","style":"pulse"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"sfx beat", `{"id":"s1","beats":[{"do":"sfx","sound":"sfx.pop"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"chart beat", `{"id":"s1","beats":[{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"show with icon", `{"id":"s1","beats":[{"do":"show","icon":"warning","at":"center","entry":"pop"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
		{"cast speaker", `{"id":"s1","speaker":"anna"}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/speaker"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectIssue(t, validateScenes(t, tc.scenes), tc.code, tc.severity, tc.path)
		})
	}
	// Supported names raise nothing.
	clean := `{"id":"s1","transition":"wipe","actors":[{"cast":"anna","pose":"typing","expression":"happy"}],"beats":[` +
		`{"do":"gesture","who":"anna","gesture":"point","count":2,"ease":"back-out"},{"do":"show","text":"Hi","style":"title","at":"center","entry":"fade"}]}`
	if issues := validateScenes(t, clean); len(issues) != 0 {
		t.Errorf("supported names rejected:%s", dump(issues))
	}
}

func TestValidateCanonicalisesInPlace(t *testing.T) {
	s := storyOf(t, `{"id":"s1","transition":"trans.fade","actors":[{"cast":"anna","pose":"pose.typing","expression":"expression.happy"}],`+
		`"beats":[{"do":"gesture","who":"anna","gesture":"gesture.point","ease":"ease.linear"}]}`)
	Validate(&s, loadMini(t), DefaultLimits())
	sc := s.Scenes[0]
	if sc.Transition != "fade" || sc.Actors[0].Pose != "typing" || sc.Actors[0].Expression != "happy" || sc.Beats[0].Gesture != "point" || sc.Beats[0].Ease != "linear" {
		t.Fatalf("not canonicalised: %+v", sc)
	}
	// A second pass finds nothing left to note.
	for _, is := range Validate(&s, loadMini(t), DefaultLimits()) {
		if is.Code == CodeCanonicalised {
			t.Fatalf("still canonicalising: %+v", is)
		}
	}
}

func TestValidateBeatRequiredFields(t *testing.T) {
	cases := []struct {
		beat    string
		missing []string // fields reported as missing_field
	}{
		{`{"do":"enter"}`, []string{"who", "from", "to"}},
		{`{"do":"exit","who":"anna"}`, []string{"to"}},
		{`{"do":"walk","who":"anna"}`, []string{"to"}},
		{`{"do":"walk","to":"left"}`, []string{"who"}},
		{`{"do":"move","target":"anna","to":"left"}`, nil},
		{`{"do":"pose","who":"anna"}`, []string{"pose"}},
		{`{"do":"gesture","who":"anna"}`, []string{"gesture"}},
		{`{"do":"emote","who":"anna"}`, []string{"expression"}},
		{`{"do":"look","who":"anna"}`, []string{"target"}},
		{`{"do":"say","who":"anna"}`, []string{"style"}},
		{`{"do":"show","at":"left","entry":"pop"}`, []string{"item"}},
		{`{"do":"show","item":"prop.lock"}`, []string{"at", "entry"}},
		{`{"do":"hide"}`, []string{"target"}},
		{`{"do":"highlight","target":"anna"}`, []string{"style"}},
		{`{"do":"camera"}`, []string{"op"}},
		{`{"do":"chart","chart":"bar"}`, []string{"data", "at"}},
		{`{"do":"sfx"}`, []string{"sound"}},
		{`{"do":"wait"}`, []string{"seconds"}},
		{`{}`, []string{"do"}},
	}
	for _, tc := range cases {
		t.Run(tc.beat, func(t *testing.T) {
			issues := validateScenes(t, `{"id":"s1","actors":[{"cast":"anna"}],"beats":[`+tc.beat+`]}`)
			got := map[string]bool{}
			for _, is := range issues {
				if is.Code == CodeMissingField {
					got[strings.TrimPrefix(is.Path, "/scenes/0/beats/0/")] = true
					if is.Severity != SeverityError || is.Fix == "" {
						t.Errorf("missing field must be an error with a fix: %+v", is)
					}
				}
			}
			if len(got) != len(tc.missing) {
				t.Fatalf("missing fields %v, want %v:%s", got, tc.missing, dump(issues))
			}
			for _, m := range tc.missing {
				if !got[m] {
					t.Errorf("%s not reported:%s", m, dump(issues))
				}
			}
		})
	}
	// The fix of a missing field carries the example of the beat kind.
	issues := validateScenes(t, `{"id":"s1","actors":[{"cast":"anna"}],"beats":[{"do":"walk","who":"anna"}]}`)
	if is := expectIssue(t, issues, CodeMissingField, SeverityError, "/scenes/0/beats/0/to"); !strings.Contains(is.Fix, `{"do":"walk","who":"anna","to":"door"}`) {
		t.Errorf("fix: %s", is.Fix)
	}
}

func TestValidateEnumsAndRanges(t *testing.T) {
	story := func(mod string) string { return `{"id":"s1","actors":[{"cast":"anna"}],` + mod + `}` }
	cases := []struct{ name, scene, code, path string }{
		{"variant", story(`"variant":"dusk"`), CodeInvalidValue, "/scenes/0/variant"},
		{"kind", story(`"kind":"movie"`), CodeInvalidValue, "/scenes/0/kind"},
		{"camera framing", story(`"camera":{"framing":"huge"}`), CodeInvalidValue, "/scenes/0/camera/framing"},
		{"facing", `{"id":"s1","actors":[{"cast":"anna","facing":"up"}]}`, CodeInvalidValue, "/scenes/0/actors/0/facing"},
		{"actor scale", `{"id":"s1","actors":[{"cast":"anna","scale":3}]}`, CodeInvalidValue, "/scenes/0/actors/0/scale"},
		{"prop layer", `{"id":"s1","props":[{"item":"prop.lock","layer":"middle"}]}`, CodeInvalidValue, "/scenes/0/props/0/layer"},
		{"duration text", story(`"duration":"soon"`), CodeInvalidValue, "/scenes/0/duration"},
		{"duration zero", story(`"duration":0`), CodeInvalidValue, "/scenes/0/duration"},
		{"duration too long", story(`"duration":901`), CodeLimitExceeded, "/scenes/0/duration"},
		{"entry", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","at":"left","entry":"explode"}]}`, CodeInvalidValue, "/scenes/0/beats/0/entry"},
		{"with", story(`"beats":[{"do":"wait","seconds":1,"with":"next"}]`), CodeInvalidValue, "/scenes/0/beats/0/with"},
		{"count", story(`"beats":[{"do":"gesture","who":"anna","gesture":"point","count":9}]`), CodeInvalidValue, "/scenes/0/beats/0/count"},
		{"delay", story(`"beats":[{"do":"wait","seconds":1,"delay":-1}]`), CodeInvalidValue, "/scenes/0/beats/0/delay"},
		{"camera op", story(`"beats":[{"do":"camera","op":"spin"}]`), CodeInvalidValue, "/scenes/0/beats/0/op"},
		{"say style", story(`"beats":[{"do":"say","who":"anna","style":"shout"}]`), CodeInvalidValue, "/scenes/0/beats/0/style"},
		{"chart data", `{"id":"s1","beats":[{"do":"chart","chart":"bar","data":{"labels":["a","b"],"values":[1]},"at":"left"}]}`, CodeInvalidValue, "/scenes/0/beats/0/data"},
		{"wait too long", story(`"beats":[{"do":"wait","seconds":9999}]`), CodeInvalidValue, "/scenes/0/beats/0/seconds"},
		{"show two things", `{"id":"s1","beats":[{"do":"show","item":"prop.lock","text":"x","at":"left","entry":"pop"}]}`, CodeInvalidValue, "/scenes/0/beats/0/item"},
		{"first beat runs with prev", story(`"beats":[{"do":"wait","seconds":1,"with":"prev"}]`), CodeInvalidValue, "/scenes/0/beats/0/with"},
		{"code scene with beats", `{"id":"s1","kind":"code","beats":[{"do":"wait","seconds":1}]}`, CodeInvalidValue, "/scenes/0/kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := validateScenes(t, tc.scene)
			var found *Issue
			for i := range issues {
				if issues[i].Code == tc.code && issues[i].Path == tc.path {
					found = &issues[i]
				}
			}
			if found == nil {
				t.Fatalf("no %s at %s:%s", tc.code, tc.path, dump(issues))
			}
			if found.Fix == "" {
				t.Errorf("issue without a fix: %+v", found)
			}
		})
	}
}

func TestValidateStoryLevel(t *testing.T) {
	cat := loadMini(t)
	base := func() Story { return storyOf(t, `{"id":"s1"}`) }
	cases := []struct {
		name string
		mod  func(*Story)
		code string
		path string
	}{
		{"schema", func(s *Story) { s.Schema = "x" }, CodeSchemaMismatch, "/schema"},
		{"title", func(s *Story) { s.Title = "  " }, CodeMissingField, "/title"},
		{"title length", func(s *Story) { s.Title = strings.Repeat("ä", 121) }, CodeInvalidValue, "/title"},
		{"aspect", func(s *Story) { s.Format.Aspect = "4:3" }, CodeInvalidValue, "/format/aspect"},
		{"aspect missing", func(s *Story) { s.Format.Aspect = "" }, CodeMissingField, "/format/aspect"},
		{"fps", func(s *Story) { s.Format.FPS = 60 }, CodeInvalidValue, "/format/fps"},
		{"palette", func(s *Story) { s.Style.Palette = "neon" }, CodeUnknownName, "/style/palette"},
		{"font", func(s *Story) { s.Style.Font = "comic" }, CodeUnknownName, "/style/font"},
		{"no scenes", func(s *Story) { s.Scenes = nil }, CodeMissingField, "/scenes"},
		{"cast id", func(s *Story) { s.Cast[0].ID = "Anna K" }, CodeInvalidValue, "/cast/0/id"},
		{"cast character", func(s *Story) { s.Cast[0].Character = "char.nobody" }, CodeUnknownAsset, "/cast/0/character"},
		{"cast character kind", func(s *Story) { s.Cast[0].Character = "prop.lock" }, CodeInvalidValue, "/cast/0/character"},
		{"duplicate cast", func(s *Story) { s.Cast = append(s.Cast, s.Cast[0]) }, CodeDuplicateID, "/cast/1/id"},
		{"voice provider", func(s *Story) { s.Voice = &Voice{Language: "de"} }, CodeMissingField, "/voice/provider"},
		{"voice language", func(s *Story) { s.Voice = &Voice{Provider: "piper"} }, CodeMissingField, "/voice/language"},
		{"voice provider active", func(s *Story) { s.Voice = &Voice{Provider: "active", Language: "de"} }, CodeInvalidValue, "/voice/provider"},
		{"voice provider unknown", func(s *Story) { s.Voice = &Voice{Provider: "gemini", Language: "de"} }, CodeUnknownName, "/voice/provider"},
		{"voice id", func(s *Story) { s.Voice = &Voice{Provider: "piper", Voice: "../../etc/passwd", Language: "de"} }, CodeInvalidValue, "/voice/voice"},
		{"cast voice provider", func(s *Story) { s.Cast[0].Voice = &Voice{Language: "de"} }, CodeMissingField, "/cast/0/voice/provider"},
		{"cast voice provider active", func(s *Story) { s.Cast[0].Voice = &Voice{Provider: "Active", Language: "de"} }, CodeInvalidValue, "/cast/0/voice/provider"},
		{"cast voice provider unknown", func(s *Story) { s.Cast[0].Voice = &Voice{Provider: "gemini", Language: "de"} }, CodeUnknownName, "/cast/0/voice/provider"},
		{"cast voice id", func(s *Story) { s.Cast[0].Voice = &Voice{Provider: "piper", Voice: "a b", Language: "de"} }, CodeInvalidValue, "/cast/0/voice/voice"},
		{"music mode", func(s *Story) { s.Music = &Music{Mode: "radio"} }, CodeInvalidValue, "/music/mode"},
		{"music volume", func(s *Story) { v := 1.5; s.Music = &Music{Mode: "generated", Volume: &v} }, CodeInvalidValue, "/music/volume"},
		{"music file missing", func(s *Story) { s.Music = &Music{Mode: "file"} }, CodeMissingField, "/music/file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := base()
			tc.mod(&s)
			issues := Validate(&s, cat, DefaultLimits())
			var found bool
			for _, is := range issues {
				if is.Code == tc.code && is.Path == tc.path && is.Severity == SeverityError {
					found = true
				}
			}
			if !found {
				t.Fatalf("no error %s at %s:%s", tc.code, tc.path, dump(issues))
			}
		})
	}
	// Valid voice ids and a music setting that is only a note.
	s := base()
	s.Voice = &Voice{Provider: "piper", Voice: "de_DE-thorsten-high", Language: "de"}
	s.Music = &Music{Mode: "generated", Mood: "upbeat-corporate"}
	issues := Validate(&s, cat, DefaultLimits())
	if HasErrors(issues) {
		t.Fatalf("errors:%s", dump(issues))
	}
	expectIssue(t, issues, CodeNotSupportedYet, SeverityMinor, "/music")
}

func TestValidateVoiceProvider(t *testing.T) {
	cat := loadMini(t)
	voiceIssues := func(path, provider string) []Issue {
		s := storyOf(t, `{"id":"s1"}`)
		v := &Voice{Provider: provider, Language: "de"}
		if path == "/voice" {
			s.Voice = v
		} else {
			s.Cast[0].Voice = v
		}
		return Validate(&s, cat, DefaultLimits())
	}
	for _, path := range []string{"/voice", "/cast/0/voice"} {
		// The TTS dispatcher ids (internal/tools/tts.go synthesizeProvider); case and space do not matter.
		for _, p := range []string{"phonon", "piper", "supertonic", "xai", "openai", "elevenlabs", "minimax", "mistral", "resemble",
			"stepfun", "sanotts", "google", "speech_lab", "Piper ", " SPEECH_LAB"} {
			if issues := voiceIssues(path, p); len(issues) != 0 {
				t.Errorf("%s provider %q:%s", path, p, dump(issues))
			}
		}
		// "active" is the default of a new project, never a saved provider.
		for _, p := range []string{"active", "Active", " ACTIVE "} {
			is := expectIssue(t, voiceIssues(path, p), CodeInvalidValue, SeverityError, path+"/provider")
			if !strings.Contains(is.Message, "concrete") || !strings.Contains(is.Fix, `{"provider":"piper"}`) {
				t.Errorf("%s provider %q: %+v", path, p, is)
			}
		}
		// An unknown provider (also an alias of the dispatcher) gets at most five alternatives.
		for _, p := range []string{"gemini", "s2s", "tts.piper", "piper?"} {
			is := expectIssue(t, voiceIssues(path, p), CodeUnknownName, SeverityError, path+"/provider")
			list := strings.TrimSuffix(strings.TrimPrefix(strings.SplitN(is.Fix, ". Example:", 2)[0], "Use one of: "), ".")
			alts := strings.Split(list, ", ")
			if len(alts) != 5 || !strings.Contains(is.Fix, `Example: {"provider":"`+alts[0]+`"}`) || !strings.Contains(is.Message, p) {
				t.Errorf("%s provider %q: %+v", path, p, is)
			}
		}
		// The closest provider comes first.
		for p, want := range map[string]string{"pipr": "piper", "elevenlab": "elevenlabs", "Open-AI": "openai"} {
			is := expectIssue(t, voiceIssues(path, p), CodeUnknownName, SeverityError, path+"/provider")
			if !strings.HasPrefix(is.Fix, "Use one of: "+want+",") {
				t.Errorf("%s provider %q: %s", path, p, is.Fix)
			}
		}
	}
}

func TestValidateMusicFilePaths(t *testing.T) {
	for path, ok := range map[string]bool{
		"Documents/Music/theme.mp3": true, "Music/a b.wav": true,
		"https://example.com/a.mp3": false, "file:///etc/passwd": false, "C:/Users/a.mp3": false, "/etc/passwd": false,
		"\\\\server\\share\\a.mp3": false, "../secret.mp3": false, "Documents/../../a.mp3": false, "Documents//a.mp3": false,
		"Documents\\a.mp3": false, "./a.mp3": false, "a\x00.mp3": false,
	} {
		s := storyOf(t, `{"id":"s1"}`)
		s.Music = &Music{Mode: "file", File: path}
		bad := findIssue(Validate(&s, loadMini(t), DefaultLimits()), CodeInvalidValue, SeverityError, "/music/file") != nil
		if bad == ok {
			t.Errorf("music file %q: accepted=%v, want %v", path, !bad, ok)
		}
	}
}

func TestValidateLimits(t *testing.T) {
	cat := loadMini(t)
	lim := Limits{MaxScenes: 2, MaxItemsPerScene: 2, MaxBeatsPerScene: 2, MaxNarrationChars: 10, MaxDurationSeconds: 30}
	s := storyOf(t, `{"id":"s1","narration":"`+strings.Repeat("ä", 11)+`","actors":[{"cast":"anna"}],"props":[{"item":"prop.lock"},{"item":"prop.shield"}],`+
		`"beats":[{"do":"wait","seconds":1},{"do":"wait","seconds":1},{"do":"wait","seconds":1}],"duration":20},{"id":"s2","duration":20},{"id":"s3"}`)
	issues := Validate(&s, cat, lim)
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes") // scene count
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes/0/narration")
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes/0/actors")
	expectIssue(t, issues, CodeLimitExceeded, SeverityError, "/scenes/0/beats")
	n := 0
	for _, is := range issues {
		if is.Code == CodeLimitExceeded && is.Path == "/scenes" {
			n++
		}
	}
	if n != 2 { // too many scenes and explicit durations over the maximum (40 s > 30 s)
		t.Errorf("want two /scenes limit issues, got %d:%s", n, dump(issues))
	}
	// The default limits are the ones of the spec.
	if d := DefaultLimits(); d.MaxScenes != 200 || d.MaxItemsPerScene != 40 || d.MaxBeatsPerScene != 60 || d.MaxNarrationChars != 1500 ||
		d.MaxStoryBytes != 1<<20 || d.MaxCodeBytes != 64<<10 || d.MaxDurationSeconds != 900 {
		t.Errorf("default limits: %+v", d)
	}
}

func TestValidateWithoutCatalogChecksStructureOnly(t *testing.T) {
	s := storyOf(t, `{"id":"s1","environment":"env.nowhere","actors":[{"cast":"anna","at":"balcony","pose":"typing"}],"beats":[{"do":"hide","target":"x-1"},{"do":"walk","who":"anna"}]}`)
	issues := Validate(&s, nil, DefaultLimits())
	for _, is := range issues {
		switch is.Code {
		case CodeUnknownAsset, CodeUnknownSpot, CodeNotSupportedYet:
			t.Errorf("catalog-dependent issue without a catalog: %+v", is)
		}
	}
	expectIssue(t, issues, CodeMissingField, SeverityError, "/scenes/0/beats/1/to")
}

func TestValidateCueInSilentScene(t *testing.T) {
	issues := validateScenes(t, `{"id":"s1","silent":true,"narration":"Eins {1}","beats":[{"do":"wait","seconds":1,"cue":1}]}`)
	expectIssue(t, issues, CodeUnknownCue, SeverityWarning, "/scenes/0/beats/0/cue")
}

func TestCreateLikeStoryIsValid(t *testing.T) {
	s := Story{Schema: Schema, Title: "T", Format: Format{Aspect: "16:9", FPS: 30}, Cast: []CastMember{}, Scenes: []Scene{{ID: "s1"}}}
	ApplyDefaults(&s)
	if issues := Validate(&s, loadMini(t), DefaultLimits()); len(issues) != 0 {
		t.Fatalf("a fresh story must validate cleanly:%s", dump(issues))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/ -run 'TestValidate|TestCreateLike' -count=1`
Expected: FAIL (build failed: `undefined: Validate`)

- [ ] **Step 3: Implement**

`internal/storymaker/validate.go`:

```go
package storymaker

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	assetIDPattern = regexp.MustCompile(`^(?:(?:char|prop|env|sfx)\.[a-z0-9][a-z0-9.-]{0,63}|(?:my|lib)\.(?:char|prop|bg)\.[a-z0-9][a-z0-9-]{0,63})$`)
	voiceIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	cuePattern     = regexp.MustCompile(`\{(\d{1,3})\}`)
)

// voiceProviders are the provider ids a saved voice may name, sorted. They are the TTS dispatcher
// ids of internal/tools/tts.go synthesizeProvider; the dispatcher also accepts the aliases s2s and
// s2s-lab for speech_lab, but a story saves only the canonical id. This package does not import
// internal/tools, so the list is kept here and must follow the dispatcher.
var voiceProviders = []string{
	"elevenlabs", "google", "minimax", "mistral", "openai", "phonon", "piper",
	"resemble", "sanotts", "speech_lab", "stepfun", "supertonic", "xai",
}

// assetRole is the kind of asset a field takes.
type assetRole struct {
	kind     string   // catalog kind
	label    string   // for messages
	prefixes []string // accepted ID prefixes (built in, project, library)
}

var (
	roleCharacter = assetRole{"character", "character", []string{"char.", "my.char.", "lib.char."}}
	roleProp      = assetRole{"prop", "prop", []string{"prop.", "my.prop.", "lib.prop."}}
	roleEnv       = assetRole{"environment", "environment", []string{"env.", "my.bg.", "lib.bg."}}
	roleSound     = assetRole{"sfx", "sound", []string{"sfx."}}
)

// nameSpec describes an enum-like field that takes a bare name.
type nameSpec struct {
	field     string          // JSON key, for examples
	kind      string          // catalog kind, "" when the catalog has no entries for it
	known     []string        // closed set the validator knows, nil when only the catalog decides
	prefixes  []string        // ID prefixes that canonicalisation strips
	supported map[string]bool // static phase-1 support; nil means the catalog decides
}

var (
	specPose       = nameSpec{field: "pose", kind: "pose", known: PoseNames, prefixes: []string{"pose."}}
	specGesture    = nameSpec{field: "gesture", kind: "gesture", known: GestureNames, prefixes: []string{"gesture."}}
	specExpression = nameSpec{field: "expression", kind: "expression", known: ExpressionNames, prefixes: []string{"expr.", "expression."}}
	specTransition = nameSpec{field: "transition", kind: "transition", known: TransitionNames, prefixes: []string{"trans.", "transition."}, supported: phase1Transitions}
	specEase       = nameSpec{field: "ease", known: EaseNames, prefixes: []string{"ease."}}
	specPalette    = nameSpec{field: "palette", kind: "palette", prefixes: []string{"palette."}}
	specTextStyle  = nameSpec{field: "style", kind: "text_style", prefixes: []string{"text."}}
)

var beatExamples = map[string]string{
	"enter":     `{"do":"enter","who":"anna","from":"left","to":"desk"}`,
	"exit":      `{"do":"exit","who":"anna","to":"door"}`,
	"walk":      `{"do":"walk","who":"anna","to":"door"}`,
	"move":      `{"do":"move","who":"anna","to":"door"}`,
	"pose":      `{"do":"pose","who":"anna","pose":"typing"}`,
	"gesture":   `{"do":"gesture","who":"anna","gesture":"point","count":2}`,
	"emote":     `{"do":"emote","who":"anna","expression":"surprised"}`,
	"look":      `{"do":"look","who":"anna","target":"camera"}`,
	"say":       `{"do":"say","who":"anna","text":"Das ist ein Paket.","style":"speech"}`,
	"show":      `{"do":"show","item":"prop.lock","at":"right","entry":"pop"}`,
	"hide":      `{"do":"hide","target":"prop.lock"}`,
	"highlight": `{"do":"highlight","target":"prop.lock","style":"pulse"}`,
	"camera":    `{"do":"camera","op":"focus","target":"anna"}`,
	"chart":     `{"do":"chart","chart":"bar","data":{"labels":["Mo","Di"],"values":[2,5]},"at":"right"}`,
	"sfx":       `{"do":"sfx","sound":"sfx.pop"}`,
	"wait":      `{"do":"wait","seconds":1}`,
}

type validator struct {
	s       *Story
	cat     *Catalog
	lim     Limits
	out     []Issue
	cast    map[string]int
	castIDs []string
	sceneID string
}

// Validate checks a story against the format, the limits and (when cat is not nil) the
// catalog. It returns issues in document order: errors block a render, warnings do not, and
// minor issues are notes (not_supported_yet, canonicalised). It canonicalises prefixed names in
// place (pose.typing becomes typing) and reports each change. A nil catalog skips the checks
// that need it (asset ids, spots, catalog names). Validate does not fill defaults; it
// understands a story with or without ApplyDefaults.
func Validate(s *Story, cat *Catalog, lim Limits) []Issue {
	if s == nil {
		return []Issue{{Severity: SeverityError, Path: "", Code: CodeMissingField, Message: "There is no story."}}
	}
	if lim == (Limits{}) {
		lim = DefaultLimits()
	}
	v := &validator{s: s, cat: cat, lim: lim, cast: map[string]int{}}
	v.story()
	return v.out
}

func (v *validator) add(sev, code, path, msg, fix string) {
	v.out = append(v.out, Issue{Severity: sev, SceneID: v.sceneID, Path: path, Code: code, Message: msg, Fix: fix})
}
func (v *validator) errorf(code, path, msg, fix string) { v.add(SeverityError, code, path, msg, fix) }
func (v *validator) warnf(code, path, msg, fix string)  { v.add(SeverityWarning, code, path, msg, fix) }
func (v *validator) minorf(code, path, msg, fix string) { v.add(SeverityMinor, code, path, msg, fix) }
func (v *validator) notYet(path, msg string)            { v.minorf(CodeNotSupportedYet, path, msg, "") }

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func listOf(names []string) string { return strings.Join(names, ", ") }

func (v *validator) story() {
	s := v.s
	if s.Schema != Schema {
		v.errorf(CodeSchemaMismatch, "/schema", fmt.Sprintf("The schema must be %q.", Schema), `Set "schema": "storymaker/1".`)
	}
	switch title := strings.TrimSpace(s.Title); {
	case title == "":
		v.errorf(CodeMissingField, "/title", "The story needs a title.", `Add "title": "Was ist eine Firewall?".`)
	case runeLen(title) > 120:
		v.errorf(CodeInvalidValue, "/title", fmt.Sprintf("The title has %d characters; the limit is 120.", runeLen(title)), "Shorten the title.")
	}
	v.format()
	v.style()
	v.checkVoice("/voice", s.Voice)
	v.music()
	v.castList()
	v.scenes()
}

func (v *validator) format() {
	f := v.s.Format
	switch f.Aspect {
	case "16:9", "9:16", "1:1":
	case "":
		v.errorf(CodeMissingField, "/format/aspect", "The format needs an aspect.", `Add "format": {"aspect":"16:9","fps":30}.`)
	default:
		v.errorf(CodeInvalidValue, "/format/aspect", fmt.Sprintf("The aspect %q is not supported.", f.Aspect), `Use "16:9", "9:16" or "1:1": {"aspect":"16:9"}.`)
	}
	switch f.FPS {
	case 24, 25, 30:
	case 0:
		v.errorf(CodeMissingField, "/format/fps", "The format needs fps.", `Add "fps": 30.`)
	default:
		v.errorf(CodeInvalidValue, "/format/fps", fmt.Sprintf("%d fps is not supported.", f.FPS), `Use 24, 25 or 30: {"fps":30}.`)
	}
}

func (v *validator) style() {
	st := &v.s.Style
	if st.Palette == "" {
		v.errorf(CodeMissingField, "/style/palette", "The style needs a palette.", `Add "style": {"palette":"sunny","font":"default","captions":true}.`)
	} else {
		v.bareName("/style/palette", &st.Palette, specPalette)
	}
	if st.Font == "" {
		v.errorf(CodeMissingField, "/style/font", "The style needs a font.", `Add "font": "default".`)
	} else if v.cat != nil && st.Font != "default" && !containsName(v.cat.Names("font"), st.Font) {
		alts := closestNames(st.Font, append([]string{"default"}, v.cat.Names("font")...), 5)
		v.errorf(CodeUnknownName, "/style/font", fmt.Sprintf("The font %q is not in the catalog.", st.Font),
			"Use one of: "+listOf(alts)+". Example: "+exampleJSON("font", alts[0]))
	}
}

func (v *validator) checkVoice(path string, voice *Voice) {
	if voice == nil {
		return
	}
	switch p := strings.ToLower(strings.TrimSpace(voice.Provider)); {
	case p == "":
		v.errorf(CodeMissingField, path+"/provider", "The voice needs a provider.", `Add "provider": "piper".`)
	case p == "active":
		v.errorf(CodeInvalidValue, path+"/provider", `The voice provider "active" is not saved; a saved story names a concrete provider.`,
			"Use one of: "+listOf(voiceProviders)+". Example: "+exampleJSON("provider", "piper"))
	case !containsName(voiceProviders, p):
		alts := closestNames(p, voiceProviders, 5)
		v.errorf(CodeUnknownName, path+"/provider", fmt.Sprintf("The voice provider %q is not known.", clip(strings.TrimSpace(voice.Provider), 40)),
			"Use one of: "+listOf(alts)+". Example: "+exampleJSON("provider", alts[0]))
	}
	if strings.TrimSpace(voice.Language) == "" {
		v.errorf(CodeMissingField, path+"/language", "The voice needs a language.", `Add "language": "de".`)
	}
	if voice.Voice != "" && !voiceIDPattern.MatchString(voice.Voice) {
		v.errorf(CodeInvalidValue, path+"/voice", fmt.Sprintf("The voice id %q is not valid.", clip(voice.Voice, 40)),
			"A voice id has 1 to 128 letters, digits, dots, dashes or underscores, for example de_DE-thorsten-high.")
	}
}

func clip(s string, n int) string {
	if runeLen(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

func (v *validator) music() {
	m := v.s.Music
	if m == nil {
		return
	}
	switch m.Mode {
	case "none", "generated", "file":
	case "":
		v.errorf(CodeMissingField, "/music/mode", "The music needs a mode.", `Use "mode": "none", "generated" or "file".`)
		return
	default:
		v.errorf(CodeInvalidValue, "/music/mode", fmt.Sprintf("The music mode %q is not supported.", m.Mode), `Use "none", "generated" or "file": {"mode":"none"}.`)
		return
	}
	if m.Volume != nil && (*m.Volume < 0 || *m.Volume > 1 || math.IsNaN(*m.Volume)) {
		v.errorf(CodeInvalidValue, "/music/volume", "The music volume must be between 0 and 1.", `Use "volume": 0.25.`)
	}
	if m.Mode == "file" {
		switch {
		case m.File == "" && m.Track == "":
			v.errorf(CodeMissingField, "/music/file", `The music mode "file" needs a file or a track.`, `Add "file": "Documents/Music/theme.mp3".`)
		case m.File != "":
			if why := badDesktopPath(m.File); why != "" {
				v.errorf(CodeInvalidValue, "/music/file", fmt.Sprintf("The music file %q is not a desktop path: %s.", clip(m.File, 60), why),
					`Use a rooted relative path inside the desktop, for example "Documents/Music/theme.mp3".`)
			}
		}
	}
	if m.Mode != "none" {
		v.notYet("/music", "Background music is accepted but not played yet.")
	}
}

// badDesktopPath explains why p is not a rooted relative desktop path ("" when it is fine).
func badDesktopPath(p string) string {
	switch {
	case len(p) > 260:
		return "it is longer than 260 characters"
	case strings.ContainsAny(p, "\x00\r\n"):
		return "it contains control characters"
	case strings.Contains(p, "://") || strings.Contains(p, ":"):
		return "it contains a scheme or drive (colon)"
	case strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\"):
		return "it starts with a slash"
	case strings.Contains(p, "\\"):
		return "it contains a backslash"
	}
	for _, seg := range strings.Split(p, "/") {
		switch seg {
		case "":
			return "it has an empty segment"
		case ".", "..":
			return "it contains . or .."
		}
	}
	return ""
}

func (v *validator) castList() {
	for i, c := range v.s.Cast {
		base := jp("cast", i)
		switch {
		case c.ID == "":
			v.errorf(CodeMissingField, base+"/id", "A cast member needs an id.", `Add "id": "anna".`)
		case !validSlug(c.ID):
			v.errorf(CodeInvalidValue, base+"/id", fmt.Sprintf("The cast id %q is not a slug.", clip(c.ID, 40)),
				"Use lowercase letters, digits and dashes (at most 48), for example anna.")
		default:
			if first, dup := v.cast[c.ID]; dup {
				v.errorf(CodeDuplicateID, base+"/id", fmt.Sprintf("The cast id %q is used twice (also at cast[%d]).", c.ID, first), "Give each cast member its own id.")
			} else {
				v.cast[c.ID] = i
				v.castIDs = append(v.castIDs, c.ID)
			}
		}
		if c.Character == "" {
			v.errorf(CodeMissingField, base+"/character", "A cast member needs a character.", `Add "character": "char.office-woman-1".`)
		} else {
			v.checkAsset(base+"/character", "character", c.Character, roleCharacter)
		}
		if runeLen(c.Name) > 80 {
			v.errorf(CodeInvalidValue, base+"/name", "The cast name is longer than 80 characters.", "Shorten the name.")
		}
		v.checkVoice(base+"/voice", c.Voice)
	}
}

func (v *validator) scenes() {
	s := v.s
	if len(s.Scenes) == 0 {
		v.errorf(CodeMissingField, "/scenes", "The story needs at least one scene.", `Add "scenes": [{"id":"s1","narration":"..."}].`)
		return
	}
	if len(s.Scenes) > v.lim.MaxScenes {
		v.errorf(CodeLimitExceeded, "/scenes", fmt.Sprintf("The story has %d scenes; the limit is %d.", len(s.Scenes), v.lim.MaxScenes), "Remove scenes or split the story into several videos.")
	}
	seen := map[string]int{}
	prevEnv := ""
	explicit := 0.0
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		v.sceneID = sc.ID
		env := sc.Environment
		if env == "" {
			env = prevEnv
		}
		if env == "" {
			env = DefaultEnvironment
		}
		prevEnv = env
		v.scene(i, sc, env, seen)
		if d := sc.Duration; d != nil && !d.Auto && len(d.Invalid) == 0 && d.Seconds > 0 {
			explicit += d.Seconds
		}
	}
	v.sceneID = ""
	if v.lim.MaxDurationSeconds > 0 && explicit > v.lim.MaxDurationSeconds {
		v.errorf(CodeLimitExceeded, "/scenes", fmt.Sprintf("The explicit scene durations add up to %.0f seconds; the limit is %.0f seconds.", explicit, v.lim.MaxDurationSeconds),
			"Shorten scene durations or remove scenes.")
	}
}

// sceneRefs are the things a beat can point at in one scene.
type sceneRefs struct {
	instances  map[string]bool // prop ids and show-beat ids (explicit or derived)
	items      map[string][]string
	spots      map[string]Spot
	spotsKnown bool
	markers    map[int]bool
}

func (v *validator) scene(i int, sc *Scene, env string, seen map[string]int) {
	base := jp("scenes", i)
	switch {
	case sc.ID == "":
		v.errorf(CodeMissingField, base+"/id", "A scene needs an id.", `Add "id": "s1".`)
	case !validSlug(sc.ID):
		v.errorf(CodeInvalidValue, base+"/id", fmt.Sprintf("The scene id %q is not a slug.", clip(sc.ID, 40)),
			"Use lowercase letters, digits and dashes (at most 48), for example s1 or intro.")
	default:
		if first, dup := seen[sc.ID]; dup {
			v.errorf(CodeDuplicateID, base+"/id", fmt.Sprintf("The scene id %q is used twice (also at scenes[%d]).", sc.ID, first), "Give each scene its own id.")
		} else {
			seen[sc.ID] = i
		}
	}
	switch sc.Kind {
	case "", "storyboard", "code":
	default:
		v.errorf(CodeInvalidValue, base+"/kind", fmt.Sprintf("The scene kind %q is not supported.", sc.Kind), `Use "storyboard" (the default) or "code".`)
	}

	if sc.Environment != "" {
		v.checkAsset(base+"/environment", "environment", sc.Environment, roleEnv)
	}
	switch sc.Variant {
	case "", "day", "night":
		if sc.Variant != "" && v.cat != nil {
			if vs := v.cat.Variants(env); len(vs) > 0 && !containsName(vs, sc.Variant) {
				v.warnf(CodeInvalidValue, base+"/variant", fmt.Sprintf("The environment %s has no %s variant.", env, sc.Variant), "Use one of: "+listOf(vs)+".")
			}
		}
	default:
		v.errorf(CodeInvalidValue, base+"/variant", fmt.Sprintf("The variant %q is not supported.", sc.Variant), `Use "day" or "night": {"variant":"night"}.`)
	}

	refs := v.refsOf(sc, env)
	v.narration(base, sc, refs)
	v.speaker(base, sc)
	if sc.Transition != "" {
		v.bareName(base+"/transition", &sc.Transition, specTransition)
	}
	v.duration(base, sc)
	if c := sc.Camera; c != nil {
		switch c.Framing {
		case "", "wide", "medium", "close":
		default:
			v.errorf(CodeInvalidValue, base+"/camera/framing", fmt.Sprintf("The camera framing %q is not supported.", c.Framing), `Use "wide", "medium" or "close": {"framing":"wide"}.`)
		}
	}

	if items := len(sc.Actors) + len(sc.Props); items > v.lim.MaxItemsPerScene {
		v.errorf(CodeLimitExceeded, base+"/actors", fmt.Sprintf("The scene has %d actors and props; the limit is %d.", items, v.lim.MaxItemsPerScene), "Remove props or split the scene.")
	}
	if len(sc.Beats) > v.lim.MaxBeatsPerScene {
		v.errorf(CodeLimitExceeded, base+"/beats", fmt.Sprintf("The scene has %d beats; the limit is %d.", len(sc.Beats), v.lim.MaxBeatsPerScene), "Split the scene into two scenes.")
	}
	if sc.Kind == "code" && (len(sc.Actors) > 0 || len(sc.Props) > 0 || len(sc.Beats) > 0) {
		v.warnf(CodeInvalidValue, base+"/kind", "A code scene draws everything in its code; its actors, props and beats are ignored.",
			`Remove actors, props and beats, or set "kind" to "storyboard".`)
	}

	if sc.Kind != "code" {
		v.actors(i, sc, env)
		v.props(i, sc, env)
		v.beats(i, sc, env, refs)
	}
}

// refsOf collects the instance ids, items, spots and cue markers of a scene.
func (v *validator) refsOf(sc *Scene, env string) *sceneRefs {
	r := &sceneRefs{instances: map[string]bool{}, items: map[string][]string{}, markers: map[int]bool{}}
	for _, p := range sc.Props {
		if p.ID != "" {
			r.instances[p.ID] = true
		}
		if p.Item != "" {
			r.items[p.Item] = append(r.items[p.Item], p.ID)
		}
	}
	derived := deriveShowIDs(sc)
	for j, b := range sc.Beats {
		if b.Do != "show" {
			continue
		}
		id := b.ID
		if id == "" {
			id = derived[j]
		}
		if id != "" {
			r.instances[id] = true
		}
		if b.Item != "" {
			r.items[b.Item] = append(r.items[b.Item], id)
		}
	}
	if v.cat != nil {
		r.spots, r.spotsKnown = v.cat.Spots(env, v.s.Format.Aspect)
	}
	for _, m := range cuePattern.FindAllStringSubmatch(sc.Narration, -1) {
		n, _ := strconv.Atoi(m[1])
		r.markers[n] = true
	}
	return r
}

func (v *validator) narration(base string, sc *Scene, _ *sceneRefs) {
	if n := runeLen(sc.Narration); n > v.lim.MaxNarrationChars {
		v.errorf(CodeLimitExceeded, base+"/narration", fmt.Sprintf("The narration has %d characters; the limit is %d.", n, v.lim.MaxNarrationChars),
			"Shorten the narration or split the scene.")
	}
}

func (v *validator) speaker(base string, sc *Scene) {
	switch sc.Speaker {
	case "", "narrator":
	default:
		if _, ok := v.cast[sc.Speaker]; !ok {
			v.errorf(CodeUnknownCast, base+"/speaker", fmt.Sprintf("The speaker %q is not in the cast.", clip(sc.Speaker, 40)),
				v.castFix("speaker", sc.Speaker))
			return
		}
		v.notYet(base+"/speaker", "A cast speaker (lip-sync and speech bubble) is accepted but not shown yet; the scene plays like a narrator scene.")
	}
}

// castFix suggests the cast ids closest to a wrong one.
func (v *validator) castFix(field, got string) string {
	if len(v.castIDs) == 0 {
		return `The story has no cast yet. Add "cast": [{"id":"anna","character":"char.office-woman-1"}].`
	}
	alts := closestNames(got, v.castIDs, 5)
	return "Use a cast id: " + listOf(alts) + ". Example: " + exampleJSON(field, alts[0])
}

func (v *validator) duration(base string, sc *Scene) {
	d := sc.Duration
	if d == nil || d.Auto {
		return
	}
	switch {
	case len(d.Invalid) > 0:
		v.errorf(CodeInvalidValue, base+"/duration", "The duration must be \"auto\" or a number of seconds.", `Use "duration": "auto" or "duration": 6.`)
	case d.Seconds <= 0 || math.IsNaN(d.Seconds):
		v.errorf(CodeInvalidValue, base+"/duration", "The duration must be greater than 0 seconds.", `Use "duration": 6 or "duration": "auto".`)
	case v.lim.MaxDurationSeconds > 0 && d.Seconds > v.lim.MaxDurationSeconds:
		v.errorf(CodeLimitExceeded, base+"/duration", fmt.Sprintf("The duration %.0f s exceeds the limit of %.0f s.", d.Seconds, v.lim.MaxDurationSeconds), "Use a shorter duration.")
	}
}

func (v *validator) actors(si int, sc *Scene, env string) {
	seen := map[string]int{}
	for j := range sc.Actors {
		a := &sc.Actors[j]
		ab := jp("scenes", si, "actors", j)
		switch {
		case a.Cast == "":
			v.errorf(CodeMissingField, ab+"/cast", "An actor needs a cast id.", `Add "cast": "anna" (an id of the story's cast).`)
		default:
			if _, ok := v.cast[a.Cast]; !ok {
				v.errorf(CodeUnknownCast, ab+"/cast", fmt.Sprintf("The cast id %q is not in the story's cast.", clip(a.Cast, 40)), v.castFix("cast", a.Cast))
			}
			if first, dup := seen[a.Cast]; dup {
				v.errorf(CodeDuplicateID, ab+"/cast", fmt.Sprintf("%q is on stage twice (also actors[%d]).", a.Cast, first), "A cast member appears once per scene; move it with beats.")
			} else {
				seen[a.Cast] = j
			}
		}
		v.position(ab+"/at", a.At, env)
		if a.Pose != "" {
			v.bareName(ab+"/pose", &a.Pose, specPose)
		}
		switch a.Facing {
		case "", "left", "right", "auto":
		default:
			v.errorf(CodeInvalidValue, ab+"/facing", fmt.Sprintf("The facing %q is not supported.", a.Facing), `Use "left", "right" or "auto": {"facing":"auto"}.`)
		}
		if a.Expression != "" {
			v.bareName(ab+"/expression", &a.Expression, specExpression)
		}
		v.scale(ab+"/scale", a.Scale)
	}
}

func (v *validator) scale(path string, scale float64) {
	if scale != 0 && (scale < 0.5 || scale > 2 || math.IsNaN(scale)) {
		v.errorf(CodeInvalidValue, path, fmt.Sprintf("The scale %g is outside 0.5 to 2.", scale), `Use "scale": 1.`)
	}
}

func (v *validator) props(si int, sc *Scene, env string) {
	seen := map[string]bool{}
	for j := range sc.Props {
		p := &sc.Props[j]
		pb := jp("scenes", si, "props", j)
		if p.ID != "" {
			switch {
			case !validSlug(p.ID):
				v.errorf(CodeInvalidValue, pb+"/id", fmt.Sprintf("The instance id %q is not a slug.", clip(p.ID, 40)), "Use lowercase letters, digits and dashes, for example lock-1.")
			case seen[p.ID]:
				v.errorf(CodeDuplicateID, pb+"/id", fmt.Sprintf("The instance id %q is used twice.", p.ID), "Give each prop its own id.")
			default:
				seen[p.ID] = true
			}
		}
		if p.Item == "" {
			v.errorf(CodeMissingField, pb+"/item", "A prop needs an item.", `Add "item": "prop.lock".`)
		} else {
			v.checkAsset(pb+"/item", "item", p.Item, roleProp)
		}
		v.position(pb+"/at", p.At, env)
		switch p.Layer {
		case "", "back", "front":
		default:
			v.errorf(CodeInvalidValue, pb+"/layer", fmt.Sprintf("The layer %q is not supported.", p.Layer), `Use "back" or "front": {"layer":"front"}.`)
		}
		v.scale(pb+"/scale", p.Scale)
	}
}

// bareName validates an enum-like field. It strips an ID prefix with a canonicalised note,
// accepts catalog names, reports known names the catalog lacks as not_supported_yet and unknown
// names as unknown_name with alternatives.
func (v *validator) bareName(path string, val *string, spec nameSpec) {
	name := *val
	if name == "" {
		return
	}
	for _, p := range spec.prefixes {
		if strings.HasPrefix(name, p) && len(name) > len(p) {
			canon := name[len(p):]
			v.minorf(CodeCanonicalised, path, fmt.Sprintf("%q is a catalog id; the field %s takes the bare name, so %q was used.", name, spec.field, canon),
				"Write "+exampleJSON(spec.field, canon)+" next time.")
			*val, name = canon, canon
			break
		}
	}
	catalogBacked := spec.kind != ""
	known := containsName(spec.known, name)
	switch {
	case v.cat != nil && catalogBacked && v.cat.HasName(spec.kind, name):
		return
	case spec.supported != nil && known:
		if !spec.supported[name] {
			v.notYet(path, fmt.Sprintf("The %s %q is accepted but not played yet.", spec.field, name))
		}
		return
	case known && (!catalogBacked || v.cat == nil):
		return
	case known:
		v.notYet(path, fmt.Sprintf("The %s %q is not in the catalog yet; %s.", spec.field, name, fallbackText(spec.field)))
		return
	case spec.known == nil && v.cat == nil:
		return // only the catalog could judge this name, and there is none
	}
	pool := spec.known
	if v.cat != nil && catalogBacked {
		if names := v.cat.Names(spec.kind); len(names) > 0 {
			pool = names
		}
	}
	alts := closestNames(name, pool, 5)
	fix := "No names are available for this field."
	if len(alts) > 0 {
		fix = "Use one of: " + listOf(alts) + ". Example: " + exampleJSON(spec.field, alts[0])
	}
	v.errorf(CodeUnknownName, path, fmt.Sprintf("The %s %q is not known.", spec.field, clip(name, 40)), fix)
}

func fallbackText(field string) string {
	switch field {
	case "pose":
		return "the actor keeps the stand pose"
	case "expression":
		return "the actor keeps the neutral face"
	case "gesture":
		return "no gesture is played"
	}
	return "the player uses a fallback"
}

// checkAsset validates an asset id field.
func (v *validator) checkAsset(path, field, id string, role assetRole) {
	if !assetIDPattern.MatchString(id) {
		v.errorf(CodeUnknownAsset, path, fmt.Sprintf("%q is not an asset id.", clip(id, 60)), v.assetFix(field, id, role, "Use the full prefixed id."))
		return
	}
	prefixOK := false
	for _, p := range role.prefixes {
		if strings.HasPrefix(id, p) {
			prefixOK = true
			break
		}
	}
	if !prefixOK {
		v.errorf(CodeInvalidValue, path, fmt.Sprintf("%q is not a %s; this field takes a %s id.", id, wrongKindLabel(id), role.label), v.assetFix(field, id, role, ""))
		return
	}
	if strings.HasPrefix(id, "my.") || strings.HasPrefix(id, "lib.") {
		return // project and library assets are resolved when the project is opened
	}
	if v.cat == nil {
		return
	}
	if role.kind == "sfx" && len(v.cat.Names("sfx")) == 0 {
		return // the sound catalog arrives with the sound phase
	}
	if e, ok := v.cat.Lookup(id); ok && e.Kind == role.kind {
		return
	}
	v.errorf(CodeUnknownAsset, path, fmt.Sprintf("The %s %q is not in the catalog.", role.label, id), v.assetFix(field, id, role, ""))
}

func wrongKindLabel(id string) string {
	switch {
	case strings.HasPrefix(id, "char."), strings.HasPrefix(id, "my.char."), strings.HasPrefix(id, "lib.char."):
		return "character"
	case strings.HasPrefix(id, "prop."), strings.HasPrefix(id, "my.prop."), strings.HasPrefix(id, "lib.prop."):
		return "prop"
	case strings.HasPrefix(id, "env."), strings.HasPrefix(id, "my.bg."), strings.HasPrefix(id, "lib.bg."):
		return "environment"
	case strings.HasPrefix(id, "sfx."):
		return "sound"
	}
	return "different kind of asset"
}

// assetFix names the catalog assets closest to a wrong id.
func (v *validator) assetFix(field, id string, role assetRole, lead string) string {
	alts := v.assetAlternatives(id, role.kind)
	if len(alts) == 0 {
		return strings.TrimSpace(lead + " Search the catalog for a " + role.label + ".")
	}
	return strings.TrimSpace(lead + " Use one of: " + listOf(alts) + ". Example: " + exampleJSON(field, alts[0]))
}

// assetAlternatives returns at most five catalog ids of a kind that resemble id.
func (v *validator) assetAlternatives(id, kind string) []string {
	if v.cat == nil {
		return nil
	}
	suffix := id
	if parts := strings.Split(id, "."); len(parts) > 1 {
		suffix = strings.Join(parts[1:], ".")
		if (parts[0] == "my" || parts[0] == "lib") && len(parts) > 2 {
			suffix = strings.Join(parts[2:], ".")
		}
	}
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] && len(out) < 5 {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, t := range splitTerms(suffix) {
		for _, e := range v.cat.Search(t, kind, 5) {
			add(e.ID)
		}
	}
	for _, s := range closestNames(id, v.cat.Names(kind), 5) {
		add(s)
	}
	return out
}

// position validates a spot or point against the environment.
func (v *validator) position(path string, p *Position, env string) {
	if p == nil {
		return
	}
	switch {
	case len(p.Invalid) > 0:
		v.errorf(CodeInvalidValue, path, "A position is a spot name or an {x, y} object.", `Use "at": "center" or "at": {"x":50,"y":88}.`)
	case p.Point:
		if p.X < 0 || p.X > 100 || p.Y < 0 || p.Y > 100 || math.IsNaN(p.X) || math.IsNaN(p.Y) {
			v.errorf(CodeInvalidValue, path, "A point's x and y are percentages between 0 and 100.", `Use {"x":50,"y":88}.`)
		}
	case p.Spot == "":
		v.errorf(CodeInvalidValue, path, "A position needs a spot name or an {x, y} point.", `Use "center" or {"x":50,"y":88}.`)
	case v.cat != nil:
		spots, ok := v.cat.Spots(env, v.s.Format.Aspect)
		if !ok {
			return // the environment itself is unknown and already reported
		}
		if _, found := spots[p.Spot]; !found {
			names := v.cat.SpotNames(env)
			alts := closestNames(p.Spot, names, 5)
			fix := "Use a spot of " + env + ": " + listOf(names) + "."
			if len(alts) > 0 {
				fix += " Example: " + exampleJSON("at", alts[0])
			}
			v.errorf(CodeUnknownSpot, path, fmt.Sprintf("The spot %q does not exist in %s.", clip(p.Spot, 40), env), fix)
		}
	}
}

func (v *validator) beats(si int, sc *Scene, env string, refs *sceneRefs) {
	stage := map[string]bool{}
	for _, a := range sc.Actors {
		stage[a.Cast] = true
	}
	explicit := map[string]bool{}
	for _, p := range sc.Props {
		if p.ID != "" {
			explicit[p.ID] = true
		}
	}
	for j := range sc.Beats {
		b := &sc.Beats[j]
		bb := jp("scenes", si, "beats", j)
		v.beat(bb, j, b, sc, env, refs, stage, explicit)
	}
}

func (v *validator) missing(bb string, b *Beat, field string, what string) {
	v.errorf(CodeMissingField, bb+"/"+field, fmt.Sprintf("A %q beat needs %q%s.", b.Do, field, what),
		fmt.Sprintf("Add %q. Example: %s", field, beatExamples[b.Do]))
}

func (v *validator) beat(bb string, j int, b *Beat, sc *Scene, env string, refs *sceneRefs, stage, explicit map[string]bool) {
	if b.Do == "" {
		v.errorf(CodeMissingField, bb+"/do", "A beat needs a kind in \"do\".", `Add "do": for example {"do":"walk","who":"anna","to":"center"}.`)
		return
	}
	if !containsName(BeatKinds, b.Do) {
		alts := closestNames(b.Do, BeatKinds, 5)
		v.errorf(CodeUnknownName, bb+"/do", fmt.Sprintf("The beat kind %q is not known.", clip(b.Do, 40)),
			"Use one of: "+listOf(alts)+". Example: "+beatExamples[alts[0]])
		return
	}
	if !phase1Beats[b.Do] {
		v.notYet(bb, fmt.Sprintf("The %q beat is accepted but not played yet; the player skips it.", b.Do))
	}

	// common fields
	switch b.With {
	case "":
	case "prev":
		if j == 0 {
			v.warnf(CodeInvalidValue, bb+"/with", `The first beat has no previous beat to run with.`, `Remove "with" from the first beat.`)
		}
	default:
		v.errorf(CodeInvalidValue, bb+"/with", fmt.Sprintf("\"with\" must be \"prev\", not %q.", clip(b.With, 20)), `Use {"with":"prev"} to start together with the previous beat.`)
	}
	if b.Cue != 0 {
		switch {
		case sc.Silent:
			v.warnf(CodeUnknownCue, bb+"/cue", "The scene is silent, so its narration markers are ignored and this beat never starts.",
				`Remove "cue" or clear "silent".`)
		case b.Cue < 0:
			v.errorf(CodeInvalidValue, bb+"/cue", "A cue is a narration marker number, 1 or more.", `Use "cue": 1 and put {1} into the narration.`)
		case !refs.markers[b.Cue]:
			v.errorf(CodeUnknownCue, bb+"/cue", fmt.Sprintf("The narration has no marker {%d}.", b.Cue), v.cueFix(refs, b.Cue))
		}
	}
	if b.Delay < 0 || b.Delay > 600 {
		v.errorf(CodeInvalidValue, bb+"/delay", "The delay must be between 0 and 600 seconds.", `Use "delay": 0.5.`)
	}
	if b.Duration < 0 || b.Duration > 600 {
		v.errorf(CodeInvalidValue, bb+"/duration", "The duration must be between 0 and 600 seconds.", `Use "duration": 1.`)
	}
	if b.Ease != "" {
		v.bareName(bb+"/ease", &b.Ease, specEase)
	}

	needWho := func(enter bool) {
		if b.Who == "" {
			v.missing(bb, b, "who", " (a cast id)")
			return
		}
		if _, ok := v.cast[b.Who]; !ok {
			v.errorf(CodeUnknownCast, bb+"/who", fmt.Sprintf("The cast id %q is not in the story's cast.", clip(b.Who, 40)), v.castFix("who", b.Who))
			return
		}
		if !enter && !stage[b.Who] {
			v.warnf(CodeUnknownCast, bb+"/who", fmt.Sprintf("%q is not on stage here; the beat is ignored.", b.Who),
				`Add the actor to "actors" or use an enter beat before this one.`)
		}
	}
	needPos := func(field string, p *Position) {
		if p.IsZero() {
			v.missing(bb, b, field, " (a spot name or {x, y})")
			return
		}
		v.position(bb+"/"+field, p, env)
	}

	switch b.Do {
	case "enter":
		needWho(true)
		needPos("from", b.From)
		needPos("to", b.To)
		if b.Who != "" {
			stage[b.Who] = true
		}
	case "exit":
		needWho(false)
		needPos("to", b.To)
		delete(stage, b.Who)
	case "walk", "move":
		switch {
		case b.Who == "" && b.Target == "":
			v.missing(bb, b, "who", " or \"target\"")
		case b.Who != "":
			needWho(false)
		default:
			v.checkTarget(bb+"/target", b.Target, refs)
		}
		needPos("to", b.To)
	case "pose":
		needWho(false)
		if b.Pose == "" {
			v.missing(bb, b, "pose", "")
		} else {
			v.bareName(bb+"/pose", &b.Pose, specPose)
		}
	case "gesture":
		needWho(false)
		if b.Gesture == "" {
			v.missing(bb, b, "gesture", "")
		} else {
			v.bareName(bb+"/gesture", &b.Gesture, specGesture)
		}
		if b.Count < 0 || b.Count > 5 {
			v.errorf(CodeInvalidValue, bb+"/count", "The count must be between 1 and 5.", `Use "count": 2.`)
		}
		if b.Target != "" {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
	case "emote":
		needWho(false)
		if b.Expression == "" {
			v.missing(bb, b, "expression", "")
		} else {
			v.bareName(bb+"/expression", &b.Expression, specExpression)
		}
	case "look":
		needWho(false)
		if b.Target == "" {
			v.missing(bb, b, "target", " (a cast id, instance id, spot or camera)")
		} else {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
	case "say":
		needWho(false)
		switch b.Style {
		case "speech", "thought":
		case "":
			v.missing(bb, b, "style", " (speech or thought)")
		default:
			v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("A say style is \"speech\" or \"thought\", not %q.", clip(b.Style, 20)), `Use "style": "speech".`)
		}
	case "show":
		v.show(bb, b, refs, explicit, env)
	case "hide":
		if b.Target == "" {
			v.missing(bb, b, "target", " (an instance id, or an item id that is unique in the scene)")
		} else {
			v.checkTarget(bb+"/target", b.Target, refs)
			if ids := refs.items[b.Target]; len(ids) > 1 {
				v.errorf(CodeInvalidValue, bb+"/target", fmt.Sprintf("%d instances of %s are in this scene, so the item id is ambiguous.", len(ids), b.Target),
					"Give the instances ids and hide one: "+exampleJSON("target", "lock-1")+".")
			}
		}
	case "highlight":
		if b.Target == "" {
			v.missing(bb, b, "target", "")
		} else {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
		switch b.Style {
		case "pulse", "outline", "arrow":
		case "":
			v.missing(bb, b, "style", " (pulse, outline or arrow)")
		default:
			v.errorf(CodeInvalidValue, bb+"/style", fmt.Sprintf("A highlight style is pulse, outline or arrow, not %q.", clip(b.Style, 20)), `Use "style": "pulse".`)
		}
	case "camera":
		switch b.Op {
		case "zoom", "pan", "focus", "shake":
		case "":
			v.missing(bb, b, "op", " (zoom, pan, focus or shake)")
		default:
			v.errorf(CodeInvalidValue, bb+"/op", fmt.Sprintf("A camera op is zoom, pan, focus or shake, not %q.", clip(b.Op, 20)), `Use "op": "zoom".`)
		}
		if b.Target != "" {
			v.checkTarget(bb+"/target", b.Target, refs)
		}
		if b.Amount < 0 {
			v.errorf(CodeInvalidValue, bb+"/amount", "The camera amount must not be negative.", `Use "amount": 1.2.`)
		}
	case "chart":
		switch b.Chart {
		case "bar", "line", "pie", "counter":
		case "":
			v.missing(bb, b, "chart", " (bar, line, pie or counter)")
		default:
			v.errorf(CodeInvalidValue, bb+"/chart", fmt.Sprintf("A chart is bar, line, pie or counter, not %q.", clip(b.Chart, 20)), `Use "chart": "bar".`)
		}
		v.chartData(bb, b)
		needPos("at", b.At)
	case "sfx":
		if b.Sound == "" {
			v.missing(bb, b, "sound", " (an sfx id)")
		} else {
			v.checkAsset(bb+"/sound", "sound", b.Sound, roleSound)
		}
	case "wait":
		switch {
		case b.Seconds <= 0:
			v.missing(bb, b, "seconds", " greater than 0")
		case b.Seconds > 600:
			v.errorf(CodeInvalidValue, bb+"/seconds", "A wait is at most 600 seconds.", `Use "seconds": 1.`)
		}
	}
}

func (v *validator) cueFix(refs *sceneRefs, got int) string {
	if len(refs.markers) == 0 {
		return fmt.Sprintf("The scene's narration has no markers. Put {%d} at the word where the beat should start, or remove \"cue\".", got)
	}
	var ns []string
	for n := 1; n <= 999; n++ {
		if refs.markers[n] {
			ns = append(ns, "{"+strconv.Itoa(n)+"}")
		}
	}
	return "Use one of the markers in the narration: " + listOf(ns) + ", or add {" + strconv.Itoa(got) + "} to the narration."
}

func (v *validator) show(bb string, b *Beat, refs *sceneRefs, explicit map[string]bool, env string) {
	kinds := 0
	for _, set := range []bool{b.Item != "", b.Text != "", b.Icon != "", b.Chart != ""} {
		if set {
			kinds++
		}
	}
	switch {
	case kinds == 0:
		v.errorf(CodeMissingField, bb+"/item", `A "show" beat needs one of "item", "text", "icon" or "chart".`, "Add \"item\". Example: "+beatExamples["show"])
	case kinds > 1:
		v.errorf(CodeInvalidValue, bb+"/item", `A "show" beat shows one thing: use only one of "item", "text", "icon" or "chart".`, "Split it into two show beats.")
	}
	if b.Item != "" {
		v.checkAsset(bb+"/item", "item", b.Item, roleProp)
	}
	if b.Text != "" && b.Style != "" {
		v.bareName(bb+"/style", &b.Style, specTextStyle)
	}
	if b.Icon != "" || b.Chart != "" {
		v.notYet(bb, "A show beat with an icon or a chart is accepted but not shown yet.")
	}
	if b.At.IsZero() {
		v.missing(bb, b, "at", " (a spot name or {x, y})")
	} else {
		v.position(bb+"/at", b.At, env)
	}
	switch b.Entry {
	case "pop", "fade", "slide", "none":
	case "":
		v.missing(bb, b, "entry", " (pop, fade, slide or none)")
	default:
		v.errorf(CodeInvalidValue, bb+"/entry", fmt.Sprintf("An entry is pop, fade, slide or none, not %q.", clip(b.Entry, 20)), `Use "entry": "pop".`)
	}
	if b.ID != "" {
		switch {
		case !validSlug(b.ID):
			v.errorf(CodeInvalidValue, bb+"/id", fmt.Sprintf("The instance id %q is not a slug.", clip(b.ID, 40)), "Use lowercase letters, digits and dashes, for example lock-1.")
		case explicit[b.ID]:
			v.errorf(CodeDuplicateID, bb+"/id", fmt.Sprintf("The instance id %q is used twice in this scene.", b.ID), "Give each shown thing its own id.")
		default:
			explicit[b.ID] = true
		}
	}
}

func (v *validator) chartData(bb string, b *Beat) {
	d := b.Data
	switch {
	case d == nil:
		v.missing(bb, b, "data", " ({labels, values})")
	case len(d.Labels) == 0 || len(d.Labels) != len(d.Values):
		v.errorf(CodeInvalidValue, bb+"/data", "Chart data needs the same number (at least one) of labels and values.", `Use "data": {"labels":["Mo","Di"],"values":[2,5]}.`)
	case len(d.Labels) > 24:
		v.errorf(CodeLimitExceeded, bb+"/data", fmt.Sprintf("The chart has %d entries; the limit is 24.", len(d.Labels)), "Show fewer values.")
	}
}

// checkTarget validates a beat target: a cast id, an instance id, an item id of the scene, a
// spot or "camera".
func (v *validator) checkTarget(path, target string, refs *sceneRefs) {
	if target == "camera" {
		return
	}
	if _, ok := v.cast[target]; ok {
		return
	}
	if refs.instances[target] {
		return
	}
	if _, ok := refs.items[target]; ok {
		return
	}
	if refs.spotsKnown {
		if _, ok := refs.spots[target]; ok {
			return
		}
	} else if v.cat == nil && validSlug(target) {
		return // spots cannot be checked without a catalog
	}
	pool := append([]string{"camera"}, v.castIDs...)
	for id := range refs.instances {
		pool = append(pool, id)
	}
	for item := range refs.items {
		pool = append(pool, item)
	}
	for name := range refs.spots {
		pool = append(pool, name)
	}
	sort.Strings(pool)
	alts := closestNames(target, pool, 5)
	v.errorf(CodeUnknownName, path, fmt.Sprintf("The target %q is not in this scene.", clip(target, 40)),
		"Use a cast id, an instance id, an item id of this scene, a spot or camera: "+listOf(alts)+". Example: "+exampleJSON("target", alts[0]))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/storymaker/ -run 'TestValidate|TestCreateLike' -count=1`, then `go test ./internal/storymaker/ -count=1`
Expected: PASS (`ok  aurago/internal/storymaker`)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/validate.go internal/storymaker/validate_test.go
git commit -m "feat(storymaker): story validation with alternatives and the phase 1 support matrix" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A6: Code scene bundling

**Files:**
- Create: `internal/storymaker/codebundle.go`
- Create: `internal/storymaker/testdata/scenes/s3.js`
- Create: `internal/storymaker/testdata/scenes/s3.bundle.js`
- Test: `internal/storymaker/codebundle_test.go`

- [ ] **Step 1: Write the failing test**

`internal/storymaker/testdata/scenes/s3.js`:

```js
export default function scene(api) {
  api.duration(5);
  const anna = api.add('char.office-woman-1', { at: 'left', pose: 'stand' });
  const label = api.text('3 von 4 Paketen geblockt', 'title');
  anna.gesture('point', { start: 1 });
  api.tween(label, { opacity: 1 }, { start: 0.5, dur: 1, ease: 'ease-out' });
  anna.emote('happy', { start: 3 });
}
```

`internal/storymaker/codebundle_test.go`:

```go
package storymaker

import (
	"bytes"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

// readFixture reads a testdata file with line endings normalised to LF (a Windows checkout may
// convert them).
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func TestBundleAcceptsFixtureScene(t *testing.T) {
	res := BundleCodeScene(readFixture(t, "testdata/scenes/s3.js"), "s3", 0)
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	code := string(res.Code)
	for _, want := range []string{"var __smSceneModule = (() => {", "default: () => scene", "\nself.__smScene=__smSceneModule.default;\n"} {
		if !strings.Contains(code, want) {
			t.Errorf("bundle misses %q", want)
		}
	}
	if !strings.HasSuffix(code, "\nself.__smScene=__smSceneModule.default;\n") {
		t.Error("bundle must end with the registration line")
	}
	if strings.Contains(code, "import ") || strings.Contains(code, "require(") {
		t.Error("bundle must not load other modules")
	}
}

// TestBundleGolden pins the exact bytes: the player test in Node reproduces them with esbuild.
// Regenerate with STORYMAKER_UPDATE_GOLDEN=1.
func TestBundleGolden(t *testing.T) {
	res := BundleCodeScene(readFixture(t, "testdata/scenes/s3.js"), "s3", 0)
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	const golden = "testdata/scenes/s3.bundle.js"
	if os.Getenv("STORYMAKER_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, res.Code, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if want := readFixture(t, golden); !bytes.Equal(res.Code, want) {
		t.Fatalf("bundle differs from %s (rerun with STORYMAKER_UPDATE_GOLDEN=1 after an intended esbuild change)\n got: %q\nwant: %q", golden, res.Code, want)
	}
}

func TestBundleRejects(t *testing.T) {
	cases := []struct {
		name string
		src  string
		line int
		col  int
		msg  string // substring
	}{
		{"static import", "import x from './y.js';\nexport default function scene(api) {}\n", 1, 1, "import"},
		{"side-effect import", "import './y.js';\nexport default function scene(api) {}\n", 1, 1, "import"},
		{"dynamic import", "export default function scene(api) {\n  api.go(import('./y.js'));\n}\n", 2, 10, "import"},
		{"export from", "export { a } from './y.js';\nexport default function scene(api) {}\n", 1, 1, "export ... from"},
		{"export star", "export * from './y.js';\nexport default function scene(api) {}\n", 1, 1, "export ... from"},
		{"eval", "export default function scene(api) {\n  eval('1');\n}\n", 2, 3, "eval"},
		{"Function", "export default function scene(api) {\n  new Function('return 1')();\n}\n", 2, 7, "Function"},
		{"globalThis", "export default function scene(api) { globalThis.x = 1; }\n", 1, 38, "globalThis"},
		{"self", "export default function scene(api) { self.postMessage(1); }\n", 1, 38, "self"},
		{"window", "export default function scene(api) { window.x = 1; }\n", 1, 38, "window"},
		{"document", "export default function scene(api) { document.title = 'x'; }\n", 1, 38, "document"},
		{"importScripts", "export default function scene(api) { importScripts('x'); }\n", 1, 38, "importScripts"},
		{"fetch", "export default function scene(api) { fetch('/x'); }\n", 1, 38, "fetch"},
		{"XMLHttpRequest", "export default function scene(api) { new XMLHttpRequest(); }\n", 1, 42, "XMLHttpRequest"},
		{"WebSocket", "export default function scene(api) { new WebSocket('ws://x'); }\n", 1, 42, "WebSocket"},
		{"dot constructor", "export default function scene(api) {\n  const f = (() => 1).constructor;\n}\n", 2, 23, ".constructor"},
		{"spaced constructor", "export default function scene(api) {\n  const f = (() => 1)\n    . constructor;\n}\n", 3, 7, ".constructor"},
		{"bracket constructor", "export default function scene(api) {\n  const f = (() => 1)['constructor'];\n}\n", 2, 23, "constructor"},
		{"escaped identifier", "export default function scene(api) {\n  \x5cu0065val('1');\n}\n", 2, 3, "eval"},
		{"escaped bracket string", "export default function scene(api) {\n  const f = {}['constr\x5cx75ctor'];\n}\n", 2, 16, "constructor"},
		{"in template substitution", "export default function scene(api) {\n  const s = `a ${eval('1')} b`;\n}\n", 2, 18, "eval"},
		{"missing default export", "export function scene(api) {}\n", 1, 1, "export default"},
		{"empty source", "  \n", 1, 1, "export default"},
		{"syntax error", "export default function scene(api) {\n  api.duration(5;\n}\n", 2, 17, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := BundleCodeScene([]byte(tc.src), "s1", 0)
			if res.Code != nil || len(res.Errors) == 0 {
				t.Fatalf("accepted: %q", res.Code)
			}
			found := false
			for _, e := range res.Errors {
				if e.Line == tc.line && e.Column == tc.col && strings.Contains(e.Message, tc.msg) {
					found = true
				}
			}
			if !found {
				t.Fatalf("want error at %d:%d containing %q, got %+v", tc.line, tc.col, tc.msg, res.Errors)
			}
		})
	}
}

// Names inside strings, comments, templates and regular expressions are not code.
func TestBundleIgnoresForbiddenNamesInsideText(t *testing.T) {
	src := "// eval self window\n" +
		"/* fetch(document) */\n" +
		"export default function scene(api) {\n" +
		"  api.text('eval and fetch and self', 'title');\n" +
		"  const re = /window|document\\//g;\n" +
		"  const t = `the ${api.duration(1)} document is self-made`;\n" +
		"  const half = (2 + 2) / 2 / 1;\n" +
		"  const selfish = 1, fetched = 2, evaluate = 3;\n" +
		"  api.tween(api.text('x', 'title'), { opacity: 1 }, { start: 0, dur: 1, ease: 'ease-out' });\n" +
		"}\n"
	res := BundleCodeScene([]byte(src), "s1", 0)
	if len(res.Errors) != 0 {
		t.Fatalf("rejected: %+v", res.Errors)
	}
}

// A slash after ) or } may be a division or a regular expression; both readings are checked, so
// a quote inside a regex literal cannot hide the code behind it.
func TestBundleAmbiguousSlashCannotHideCode(t *testing.T) {
	src := "export default function scene(api) {\n  if (api) /\"/.test('x'); eval('1');\n}\n"
	res := BundleCodeScene([]byte(src), "s1", 0)
	if len(res.Errors) == 0 {
		t.Fatal("eval behind an ambiguous slash was accepted")
	}
}

func TestBundleSizeCap(t *testing.T) {
	src := []byte("export default function scene(api) {}\n" + strings.Repeat("// padding\n", 20))
	if res := BundleCodeScene(src, "s1", len(src)); len(res.Errors) != 0 {
		t.Fatalf("source at the limit rejected: %+v", res.Errors)
	}
	res := BundleCodeScene(src, "s1", len(src)-1)
	if res.Code != nil || len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "limit") {
		t.Fatalf("oversize source: %+v", res)
	}
}

func TestBundleReportsEveryProblemSortedAndColumnsCountCharacters(t *testing.T) {
	src := "export default function scene(api) {\n  api.text('äöü', eval('1'));\n  fetch('x');\n}\n"
	res := BundleCodeScene([]byte(src), "s1", 0)
	if len(res.Errors) != 2 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	if res.Errors[0].Line != 2 || res.Errors[0].Column != 19 || res.Errors[1].Line != 3 || res.Errors[1].Column != 3 {
		t.Fatalf("positions: %+v", res.Errors)
	}
}

func TestBundleOutputIsDeterministic(t *testing.T) {
	src := readFixture(t, "testdata/scenes/s3.js")
	a := BundleCodeScene(src, "s3", 0)
	b := BundleCodeScene(src, "s3", 0)
	if !bytes.Equal(a.Code, b.Code) {
		t.Fatal("two bundles of the same source differ")
	}
}

func TestBundleAmbiguousSlashBothDirections(t *testing.T) {
	// Read as a regular expression, "/2; eval('1'); const b = 2/" would swallow the call; read as
	// a division it is plain code. The check sees it either way.
	src := "export default function scene(api) {\n  const a = (4)/2; eval('1'); const b = 2/1;\n}\n"
	if res := BundleCodeScene([]byte(src), "s1", 0); len(res.Errors) == 0 {
		t.Fatal("eval between two divisions was accepted")
	}
	src = "export default function scene(api) {\n  let n = 1; n++ /2; eval('1'); n /1;\n}\n"
	if res := BundleCodeScene([]byte(src), "s1", 0); len(res.Errors) == 0 {
		t.Fatal("eval after a postfix increment was accepted")
	}
}

func TestPrecheckNeverPanicsOrHangs(t *testing.T) {
	alphabet := []string{"/", "*", "`", "${", "}", "{", "\"", "'", "\\", "\x5cu", "\x5cu{", "\n", " ", "(", ")", "[", "]", "eval", "x", "1", ".", "+", "-", "=>",
		"import", "export", "default", "from", "ä", "\x00", "//", "/*", "*/", "constructor", "\xff"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		rng := rand.New(rand.NewSource(1))
		for i := 0; i < 30000; i++ {
			var b strings.Builder
			for j, n := 0, rng.Intn(40); j < n; j++ {
				b.WriteString(alphabet[rng.Intn(len(alphabet))])
			}
			precheckSource([]byte(b.String()))
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the pre-check did not finish")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/ -run 'TestBundle|TestPrecheck' -count=1`
Expected: FAIL (build failed: `undefined: BundleCodeScene`)

- [ ] **Step 3: Implement**

`internal/storymaker/codebundle.go`:

```go
package storymaker

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/evanw/esbuild/pkg/api"
)

// BundleError is one problem found while bundling a code scene. Line and Column are
// 1-based and count characters (not bytes) of the author's source, so an editor can
// place a marker without conversion.
type BundleError struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

// BundleResult is the outcome of BundleCodeScene. Errors non-empty means Code is nil.
type BundleResult struct {
	Code   []byte
	Errors []BundleError
}

const (
	// bundleGlobalName is the IIFE namespace esbuild assigns the scene module to.
	bundleGlobalName = "__smSceneModule"
	// bundleSuffix registers the scene function on the fixed global the code-scene worker reads.
	// A dotted GlobalName would make esbuild emit `var self = self || {}` and assign the
	// namespace object instead of the function, so the registration is appended by Go.
	bundleSuffix = "\nself.__smScene=" + bundleGlobalName + ".default;\n"

	maxBundleErrors = 20
)

// BundleCodeScene turns the author's ES module source into one classic script. The
// script defines the scene function on `self.__smScene`.
//
// Order: a size cap, esbuild's syntax check and the pre-check run on the author's own
// source, so every position refers to what the author typed. The pre-check is defence in
// depth only: the real barriers are the player CSP and the code-scene worker realm.
func BundleCodeScene(source []byte, sceneID string, maxBytes int) BundleResult {
	if maxBytes <= 0 {
		maxBytes = DefaultLimits().MaxCodeBytes
	}
	if len(source) > maxBytes {
		return BundleResult{Errors: []BundleError{{
			Line: 1, Column: 1,
			Message: fmt.Sprintf("The code scene is %d bytes; the limit is %d bytes. Split the scene or remove unused code.", len(source), maxBytes),
		}}}
	}
	if !validSlug(sceneID) {
		sceneID = "scene"
	}

	var errs []BundleError
	res := api.Transform(string(source), api.TransformOptions{
		Format:        api.FormatIIFE,
		GlobalName:    bundleGlobalName,
		Target:        api.ES2020,
		LegalComments: api.LegalCommentsNone,
		Loader:        api.LoaderJS,
		Sourcefile:    "scenes/" + sceneID + ".js",
	})
	for _, m := range res.Errors {
		errs = append(errs, esbuildError(m))
	}
	errs = append(errs, precheckSource(source)...)
	errs = normalizeBundleErrors(errs)
	if len(errs) > 0 {
		return BundleResult{Errors: errs}
	}
	out := make([]byte, 0, len(res.Code)+len(bundleSuffix))
	out = append(out, res.Code...)
	out = append(out, bundleSuffix...)
	return BundleResult{Code: out}
}

func esbuildError(m api.Message) BundleError {
	e := BundleError{Line: 1, Column: 1, Message: strings.TrimSpace(m.Text)}
	if loc := m.Location; loc != nil {
		e.Line = max(loc.Line, 1)
		col := min(max(loc.Column, 0), len(loc.LineText))
		e.Column = utf8.RuneCountInString(loc.LineText[:col]) + 1
	}
	return e
}

func normalizeBundleErrors(errs []BundleError) []BundleError {
	if len(errs) == 0 {
		return nil
	}
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Line != errs[j].Line {
			return errs[i].Line < errs[j].Line
		}
		return errs[i].Column < errs[j].Column
	})
	out := errs[:0]
	seen := map[BundleError]bool{}
	for _, e := range errs {
		if seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
		if len(out) == maxBundleErrors {
			break
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Pre-check: a small JavaScript tokenizer. esbuild's Go API exposes no AST, so the
// check works on tokens. It skips strings, template literals, comments and regular
// expression literals, so names inside them never trigger it.

type tokKind int

const (
	tokIdent tokKind = iota
	tokPunct
	tokString
	tokNumber
	tokTemplate
	tokRegex
)

type srcTok struct {
	kind      tokKind
	text      string // identifier name (escapes decoded), punctuation character or string value
	line, col int
}

var forbiddenIdents = map[string]bool{
	"eval": true, "Function": true, "globalThis": true, "self": true, "window": true, "document": true,
	"importScripts": true, "fetch": true, "XMLHttpRequest": true, "WebSocket": true,
}

// regexKeywords are the keywords after which a slash starts a regular expression.
var regexKeywords = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true, "delete": true,
	"void": true, "throw": true, "case": true, "do": true, "else": true, "yield": true, "await": true,
}

// precheckSource runs the token checks. A slash after `)`, `}`, `++` or `--` is ambiguous
// (division or regular expression), so the source is tokenized twice, once with each
// reading, and the findings are merged: hiding a name from the check would need both
// readings to swallow it.
func precheckSource(src []byte) []BundleError {
	var errs []BundleError
	hasDefault := false
	for _, ambiguousIsRegex := range []bool{false, true} {
		toks := tokenizeJS(src, ambiguousIsRegex)
		e, d := checkTokens(toks)
		errs = append(errs, e...)
		hasDefault = hasDefault || d
	}
	if !hasDefault {
		errs = append(errs, BundleError{Line: 1, Column: 1, Message: "A code scene must export a default function: export default function scene(api) { ... }"})
	}
	return errs
}

func checkTokens(toks []srcTok) (errs []BundleError, hasDefault bool) {
	add := func(t srcTok, msg string) {
		errs = append(errs, BundleError{Line: t.line, Column: t.col, Message: msg})
	}
	for i, t := range toks {
		prev := srcTok{kind: -1}
		if i > 0 {
			prev = toks[i-1]
		}
		switch t.kind {
		case tokIdent:
			switch {
			case forbiddenIdents[t.text]:
				add(t, fmt.Sprintf("%s is not available in code scenes. Use only the api object passed to scene(api).", t.text))
			case t.text == "import":
				add(t, "import is not supported in code scenes: a scene is one file and gets everything from the api object.")
			case t.text == "constructor" && prev.kind == tokPunct && prev.text == ".":
				add(t, ".constructor is not available in code scenes.")
			case t.text == "export":
				if hasDefaultAfterExport(toks, i) {
					hasDefault = true
				}
				if exportsFromModule(toks, i) {
					add(t, "export ... from is not supported in code scenes. Use export default function scene(api) { ... }.")
				}
			}
		case tokString:
			if t.text == "constructor" && prev.kind == tokPunct && prev.text == "[" {
				add(t, `["constructor"] is not available in code scenes.`)
			} else if forbiddenIdents[t.text] && prev.kind == tokPunct && prev.text == "[" {
				add(t, fmt.Sprintf("[%q] is not available in code scenes.", t.text))
			}
		}
	}
	return errs, hasDefault
}

// hasDefaultAfterExport reports `export default ...` and `export { x as default }`.
func hasDefaultAfterExport(toks []srcTok, i int) bool {
	if i+1 >= len(toks) {
		return false
	}
	next := toks[i+1]
	if next.kind == tokIdent && next.text == "default" {
		return true
	}
	if next.kind == tokPunct && next.text == "{" {
		for j := i + 2; j < len(toks) && !(toks[j].kind == tokPunct && toks[j].text == "}"); j++ {
			if toks[j].kind == tokIdent && toks[j].text == "default" {
				return true
			}
		}
	}
	return false
}

// exportsFromModule reports `export * from ...` and `export { ... } from ...`.
func exportsFromModule(toks []srcTok, i int) bool {
	j := i + 1
	if j >= len(toks) || toks[j].kind != tokPunct {
		return false
	}
	switch toks[j].text {
	case "*":
		return true
	case "{":
		for ; j < len(toks); j++ {
			if toks[j].kind == tokPunct && toks[j].text == "}" {
				return j+1 < len(toks) && toks[j+1].kind == tokIdent && toks[j+1].text == "from"
			}
		}
	}
	return false
}

type jsScanner struct {
	src  []byte
	pos  int
	line int
	col  int
	toks []srcTok
	// braces records for every open `{` whether it opened a template substitution.
	braces []bool
	// ambiguousIsRegex selects the reading of a slash after `)`, `}`, `++` or `--`.
	ambiguousIsRegex bool
}

func tokenizeJS(src []byte, ambiguousIsRegex bool) []srcTok {
	s := &jsScanner{src: src, line: 1, col: 1, ambiguousIsRegex: ambiguousIsRegex}
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '\n':
			s.next()
		case c == ' ' || c == '\t' || c == '\r' || c == '\v' || c == '\f':
			s.next()
		case c == '/' && s.peek(1) == '/':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.next()
			}
		case c == '/' && s.peek(1) == '*':
			s.next()
			s.next()
			for s.pos < len(s.src) && !(s.src[s.pos] == '*' && s.peek(1) == '/') {
				s.next()
			}
			s.next()
			s.next()
		case c == '"' || c == '\'':
			s.scanString(c)
		case c == '`':
			start := s.mark()
			s.next()
			s.scanTemplate(start)
		case c >= '0' && c <= '9' || c == '.' && s.peek(1) >= '0' && s.peek(1) <= '9':
			s.scanNumber()
		case c == '/':
			if s.slashStartsRegex() && s.scanRegex() {
				continue
			}
			s.punct()
		case c == '{':
			s.braces = append(s.braces, false)
			s.punct()
		case c == '}':
			if n := len(s.braces); n > 0 {
				top := s.braces[n-1]
				s.braces = s.braces[:n-1]
				if top {
					// End of a template substitution: the template text continues.
					start := s.mark()
					s.next()
					s.scanTemplate(start)
					continue
				}
			}
			s.punct()
		default:
			r, _ := utf8.DecodeRune(s.src[s.pos:])
			if isIdentStart(r) || c == '\\' {
				s.scanIdent()
			} else {
				s.punct()
			}
		}
	}
	return s.toks
}

type srcMark struct{ line, col int }

func (s *jsScanner) mark() srcMark { return srcMark{s.line, s.col} }

func (s *jsScanner) peek(n int) byte {
	if s.pos+n < len(s.src) {
		return s.src[s.pos+n]
	}
	return 0
}

// next consumes one character and keeps the line and column (in characters) current.
func (s *jsScanner) next() {
	if s.pos >= len(s.src) {
		return
	}
	if s.src[s.pos] == '\n' {
		s.pos++
		s.line++
		s.col = 1
		return
	}
	_, size := utf8.DecodeRune(s.src[s.pos:])
	s.pos += size
	s.col++
}

func (s *jsScanner) emit(kind tokKind, text string, m srcMark) {
	s.toks = append(s.toks, srcTok{kind: kind, text: text, line: m.line, col: m.col})
}

func (s *jsScanner) punct() {
	m := s.mark()
	r, size := utf8.DecodeRune(s.src[s.pos:])
	s.next()
	s.emit(tokPunct, string(r), m)
	_ = size
}

func isIdentStart(r rune) bool {
	return r == '$' || r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '$' || r == '_' || r == 0x200c || r == 0x200d || unicode.IsLetter(r) || unicode.IsDigit(r) ||
		unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Pc, r)
}

// scanIdent reads an identifier. A unicode escape (backslash, u, then four hex digits or a braced
// code point) is decoded, so an escaped spelling of a forbidden name is still caught.
func (s *jsScanner) scanIdent() {
	m := s.mark()
	var b strings.Builder
	for s.pos < len(s.src) {
		r, _ := utf8.DecodeRune(s.src[s.pos:])
		if r == '\\' && s.peek(1) == 'u' {
			if v, n, ok := decodeUnicodeEscape(s.src[s.pos+2:]); ok {
				b.WriteRune(v)
				for i := 0; i < 2+n; i++ {
					s.next()
				}
				continue
			}
			break
		}
		if !isIdentPart(r) {
			break
		}
		b.WriteRune(r)
		s.next()
	}
	if b.Len() == 0 {
		// A stray backslash: consume it so the scan always moves forward.
		s.punct()
		return
	}
	s.emit(tokIdent, b.String(), m)
}

// decodeUnicodeEscape decodes the part after the backslash-u of a unicode escape: four hex digits or
// {hex...}. It returns the rune and the number of bytes consumed after the backslash-u.
func decodeUnicodeEscape(b []byte) (rune, int, bool) {
	if len(b) >= 1 && b[0] == '{' {
		end := 1
		for end < len(b) && b[end] != '}' && end < 9 {
			end++
		}
		if end >= len(b) || b[end] != '}' || end == 1 {
			return 0, 0, false
		}
		v, err := strconv.ParseUint(string(b[1:end]), 16, 32)
		if err != nil || v > unicode.MaxRune {
			return 0, 0, false
		}
		return rune(v), end + 1, true
	}
	if len(b) < 4 {
		return 0, 0, false
	}
	v, err := strconv.ParseUint(string(b[:4]), 16, 32)
	if err != nil {
		return 0, 0, false
	}
	return rune(v), 4, true
}

func (s *jsScanner) scanNumber() {
	m := s.mark()
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_', c == '.':
			s.next()
		case (c == '+' || c == '-') && s.pos > 0 && (s.src[s.pos-1] == 'e' || s.src[s.pos-1] == 'E'):
			s.next()
		default:
			s.emit(tokNumber, "", m)
			return
		}
	}
	s.emit(tokNumber, "", m)
}

// scanString reads a quoted string and records its value with simple escapes decoded.
// A string ends at its closing quote or, when unterminated, at the end of the line.
func (s *jsScanner) scanString(quote byte) {
	m := s.mark()
	s.next()
	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == quote {
			s.next()
			break
		}
		if c == '\n' {
			break
		}
		if c == '\\' {
			s.next()
			if s.pos >= len(s.src) {
				break
			}
			e := s.src[s.pos]
			switch e {
			case 'x':
				if s.pos+2 < len(s.src) {
					if v, err := strconv.ParseUint(string(s.src[s.pos+1:s.pos+3]), 16, 8); err == nil {
						b.WriteRune(rune(v))
						s.next()
						s.next()
						s.next()
						continue
					}
				}
			case 'u':
				if v, n, ok := decodeUnicodeEscape(s.src[s.pos+1:]); ok {
					b.WriteRune(v)
					for i := 0; i < 1+n; i++ {
						s.next()
					}
					continue
				}
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\r':
				// Line continuation: skip a following \n as well.
				s.next()
				if s.pos < len(s.src) && s.src[s.pos] == '\n' {
					s.next()
				}
				continue
			case '\n':
				s.next()
				continue
			default:
				b.WriteByte(e)
			}
			s.next()
			continue
		}
		r, _ := utf8.DecodeRune(s.src[s.pos:])
		b.WriteRune(r)
		s.next()
	}
	s.emit(tokString, b.String(), m)
}

// scanTemplate reads template text up to the closing backtick or the next `${`. It runs
// right after the opening backtick or after the `}` that closed a substitution.
func (s *jsScanner) scanTemplate(start srcMark) {
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '\\':
			s.next()
			s.next()
		case c == '`':
			s.next()
			s.emit(tokTemplate, "", start)
			return
		case c == '$' && s.peek(1) == '{':
			s.next()
			s.next()
			s.braces = append(s.braces, true)
			s.emit(tokTemplate, "", start)
			return
		default:
			s.next()
		}
	}
	s.emit(tokTemplate, "", start)
}

// slashStartsRegex decides how a slash is read from the previous token.
func (s *jsScanner) slashStartsRegex() bool {
	n := len(s.toks)
	if n == 0 {
		return true
	}
	last := s.toks[n-1]
	switch last.kind {
	case tokNumber, tokString, tokTemplate, tokRegex:
		return false
	case tokIdent:
		return regexKeywords[last.text]
	}
	switch last.text {
	case ")", "}":
		return s.ambiguousIsRegex
	case "]":
		return false
	case "+", "-":
		if n >= 2 {
			p := s.toks[n-2]
			if p.kind == tokPunct && p.text == last.text && p.line == last.line && p.col+1 == last.col {
				return s.ambiguousIsRegex
			}
		}
	}
	return true
}

// scanRegex reads a regular expression literal on the current line. It reports false (and
// consumes nothing) when no closing slash is found before the end of the line.
func (s *jsScanner) scanRegex() bool {
	i := s.pos + 1
	inClass := false
	for ; i < len(s.src); i++ {
		c := s.src[i]
		if c == '\n' {
			return false
		}
		if c == '\\' {
			i++
			continue
		}
		if c == '[' {
			inClass = true
		} else if c == ']' {
			inClass = false
		} else if c == '/' && !inClass {
			break
		}
	}
	if i >= len(s.src) {
		return false
	}
	m := s.mark()
	for s.pos <= i {
		s.next()
	}
	for s.pos < len(s.src) && (s.src[s.pos] >= 'a' && s.src[s.pos] <= 'z') {
		s.next()
	}
	s.emit(tokRegex, "", m)
	return true
}
```

The golden bundle is generated, not typed. Generate it once, check its hash, then cross-check it with Node esbuild (the player part
reproduces these bytes in its own tests; the repo pins esbuild 0.28.2 in `go.mod` and `package.json`):

```bash
STORYMAKER_UPDATE_GOLDEN=1 go test ./internal/storymaker/ -run 'TestBundleGolden' -count=1
sha256sum internal/storymaker/testdata/scenes/s3.bundle.js
# expected: 5a7e31916dcaa48c3aa30145cb6ea9496a489cc3e023d64999ef38bef83918d4  (1365 bytes, LF line endings)
NODE_PATH=<repo checkout with node_modules> node -e "const e=require('esbuild'),fs=require('fs');const d='internal/storymaker/testdata/scenes/';const src=fs.readFileSync(d+'s3.js','utf8').replace(/\r\n/g,'\n');const out=e.transformSync(src,{format:'iife',globalName:'__smSceneModule',target:'es2020',legalComments:'none',loader:'js',sourcefile:'scenes/s3.js'}).code+'\nself.__smScene=__smSceneModule.default;\n';const gold=fs.readFileSync(d+'s3.bundle.js','utf8').replace(/\r\n/g,'\n');console.log(e.version,out===gold?'IDENTICAL':'DIFFERENT');process.exit(out===gold?0:1)"
# expected: 0.28.2 IDENTICAL
```

Expected content of the golden file (for review; the command above produces it):

```js
var __smSceneModule = (() => {
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);
  var s3_exports = {};
  __export(s3_exports, {
    default: () => scene
  });
  function scene(api) {
    api.duration(5);
    const anna = api.add("char.office-woman-1", { at: "left", pose: "stand" });
    const label = api.text("3 von 4 Paketen geblockt", "title");
    anna.gesture("point", { start: 1 });
    api.tween(label, { opacity: 1 }, { start: 0.5, dur: 1, ease: "ease-out" });
    anna.emote("happy", { start: 3 });
  }
  return __toCommonJS(s3_exports);
})();

self.__smScene=__smSceneModule.default;
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/storymaker/ -run 'TestBundle|TestPrecheck' -count=1`, then `go test ./internal/storymaker/ -count=1`
Expected: PASS (`ok  aurago/internal/storymaker`)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/codebundle.go internal/storymaker/testdata/scenes/s3.js internal/storymaker/testdata/scenes/s3.bundle.js internal/storymaker/codebundle_test.go
git commit -m "feat(storymaker): in-process esbuild bundling of code scenes with a pre-check" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A7: Slugs, project store and in-memory store

**Files:**
- Create: `internal/storymaker/slug.go`
- Create: `internal/storymaker/memstore.go`
- Create: `internal/storymaker/project.go`
- Test: `internal/storymaker/slug_test.go`
- Test: `internal/storymaker/project_test.go`

- [ ] **Step 1: Write the failing test**

`internal/storymaker/slug_test.go`:

```go
package storymaker

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000001"
	cases := []struct{ title, want string }{
		{"Was ist eine Firewall?", "Was-ist-eine-Firewall"},
		{"Größe & Übung: Äpfel, Öl, Straße", "Groesse-Uebung-Aepfel-Oel-Strasse"},
		{"ÄÖÜ äöü ß", "AeOeUe-aeoeue-ss"},
		{"  --Hallo   Welt--  ", "Hallo-Welt"},
		{"Café Crème brûlée, señor", "Cafe-Creme-brulee-senor"},
		{"Zażółć gęślą jaźń", "Zazolc-gesla-jazn"},
		{"a/b\\c:d*e?f\"g<h>i|j", "a-b-c-d-e-f-g-h-i-j"},
		{"../../etc/passwd", "etc-passwd"},
		{"v1.2 final.mp4", "v1-2-final-mp4"},
		{"日本語のタイトル", id},
		{"???", id},
		{"", id},
		// Windows reserved device names fall back (any case), their look-alikes stay.
		{"CON", id}, {"nul", id}, {"Aux", id}, {"PRN", id}, {"com1", id}, {"LPT9", id},
		{"COM0", "COM0"}, {"console", "console"}, {"CON-1", "CON-1"}, {"COM10", "COM10"},
	}
	for _, tc := range cases {
		if got := Slugify(tc.title, id); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
	if got := Slugify("???", ""); got != "storymaker" {
		t.Errorf("empty fallback: %q", got)
	}
	long := Slugify(strings.Repeat("abcdefghi ", 20), id)
	if len(long) > 80 || strings.HasSuffix(long, "-") || !strings.HasPrefix(long, "abcdefghi-abcdefghi") {
		t.Errorf("long title: %q (%d)", long, len(long))
	}
	// Whatever the input, the result is a plain file-name stem.
	for _, in := range []string{"x\x00y", "a\nb", "tab\there", "emoji " + string(rune(0x1F600)) + " title", string(rune(0x202e)) + "evil"} {
		got := Slugify(in, id)
		for _, r := range got {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				t.Errorf("Slugify(%q) = %q has %q", in, got, r)
			}
		}
	}
}

func TestValidProjectID(t *testing.T) {
	ok := []string{"00000000-0000-4000-8000-000000000001", "9b2f1c3e-7d44-4a5b-8c1d-0123456789ab"}
	bad := []string{"", "x", "9B2F1C3E-7D44-4A5B-8C1D-0123456789AB", "9b2f1c3e-7d44-1a5b-8c1d-0123456789ab", "9b2f1c3e-7d44-4a5b-cc1d-0123456789ab",
		"9b2f1c3e7d444a5b8c1d0123456789ab", "../9b2f1c3e-7d44-4a5b-8c1d-0123456789ab", "9b2f1c3e-7d44-4a5b-8c1d-0123456789ab/",
		"9b2f1c3e-7d44-4a5b-8c1d-0123456789ab\n"}
	for _, id := range ok {
		if !ValidProjectID(id) {
			t.Errorf("%q rejected", id)
		}
	}
	for _, id := range bad {
		if ValidProjectID(id) {
			t.Errorf("%q accepted", id)
		}
	}
}
```

`internal/storymaker/project_test.go`:

```go
package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const testRoot = "Documents/Storymaker"

// newTestStore returns a ProjectStore on a MemStore with a controllable clock and sequential ids.
func newTestStore(t *testing.T) (*ProjectStore, *MemStore, *time.Time) {
	t.Helper()
	mem := NewMemStore()
	ps := NewProjectStore(mem, testRoot)
	clock := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	ps.now = func() time.Time { return clock }
	n := 0
	ps.newID = func() string {
		n++
		return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
	}
	return ps, mem, &clock
}

func TestProjectCreateLayoutAndDefaults(t *testing.T) {
	ps, mem, _ := newTestStore(t)
	ctx := context.Background()
	proj, story, etag, err := ps.Create(ctx, "  Was ist eine Firewall?  ", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidProjectID(proj.ID) || proj.Title != "Was ist eine Firewall?" || proj.Aspect != "16:9" || proj.FPS != 30 ||
		!proj.CreatedAt.Equal(proj.UpdatedAt) || proj.CreatedAt.IsZero() {
		t.Fatalf("project: %+v", proj)
	}
	if len(story.Scenes) != 1 || story.Scenes[0].ID != "s1" || story.Scenes[0].Environment != "env.plain-stage" ||
		story.Style.Palette != "sunny" || !story.Style.Captions || story.Format.FPS != 30 || story.Voice != nil || story.Music != nil {
		t.Fatalf("story: %+v", story)
	}
	if issues := Validate(&story, loadMini(t), DefaultLimits()); len(issues) != 0 {
		t.Fatalf("a new story must be valid:%s", dump(issues))
	}
	base := testRoot + "/" + proj.ID
	if got, want := mem.Paths(), []string{base + "/project.json", base + "/story.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files: %v", got)
	}
	dirs := strings.Join(mem.Dirs(), " ")
	for _, d := range []string{"assets", "audio/narration", "audio/music", "exports"} {
		if !strings.Contains(dirs, base+"/"+d) {
			t.Errorf("directory %s not created (have %s)", d, dirs)
		}
	}
	data, gotEtag, err := ps.LoadStory(ctx, proj.ID)
	if err != nil || gotEtag != etag || ETag(data) != etag {
		t.Fatalf("LoadStory: %v %q %q", err, gotEtag, etag)
	}
	again, _, err := DecodeStory(data, DefaultLimits())
	if err != nil || again.Title != story.Title {
		t.Fatalf("stored story: %v %+v", err, again)
	}
	if !strings.HasPrefix(etag, `"`) || len(etag) != 66 {
		t.Fatalf("etag must be the quoted sha256 of the notes: %s", etag)
	}

	// Format options and invalid arguments.
	p2, s2, _, err := ps.Create(ctx, "Hochformat", "9:16", 24)
	if err != nil || p2.Aspect != "9:16" || p2.FPS != 24 || s2.Format != (Format{Aspect: "9:16", FPS: 24}) {
		t.Fatalf("9:16: %v %+v", err, p2)
	}
	for _, bad := range []struct {
		title, aspect string
		fps           int
	}{{"", "", 0}, {"   ", "", 0}, {strings.Repeat("x", 121), "", 0}, {"T", "4:3", 0}, {"T", "", 60}} {
		if _, _, _, err := ps.Create(ctx, bad.title, bad.aspect, bad.fps); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("Create(%q,%q,%d) = %v", bad.title, bad.aspect, bad.fps, err)
		}
	}
}

func TestProjectGetListDelete(t *testing.T) {
	ps, mem, clock := newTestStore(t)
	ctx := context.Background()
	if got, err := ps.List(ctx); err != nil || got != nil {
		t.Fatalf("list of a missing root: %v %v", got, err)
	}
	a, _, _, _ := ps.Create(ctx, "A", "", 0)
	*clock = clock.Add(time.Hour)
	b, _, _, _ := ps.Create(ctx, "B", "", 0)
	// Junk next to the projects is skipped.
	_, _ = mem.WriteFile(ctx, testRoot+"/Library/library.json", []byte("{}"), "")
	_, _ = mem.WriteFile(ctx, testRoot+"/00000000-0000-4000-8000-999999999999/note.txt", []byte("x"), "")
	list, err := ps.List(ctx)
	if err != nil || len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("list (newest first): %v %+v", err, list)
	}
	got, err := ps.Get(ctx, a.ID)
	if err != nil || got.Title != "A" || !got.CreatedAt.Equal(a.CreatedAt) {
		t.Fatalf("get: %v %+v", err, got)
	}
	if _, err := ps.Get(ctx, "00000000-0000-4000-8000-000000000042"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing project: %v", err)
	}
	for _, id := range []string{"", "../x", "ABCDEF00-0000-4000-8000-000000000001", "00000000-0000-1000-8000-000000000001", "a/b"} {
		if _, err := ps.Get(ctx, id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Get(%q) = %v", id, err)
		}
		if err := ps.Delete(ctx, id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Delete(%q) = %v", id, err)
		}
	}
	if err := ps.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := ps.Delete(ctx, a.ID); err != nil {
		t.Fatalf("deleting a missing project must succeed: %v", err)
	}
	for _, p := range mem.Paths() {
		if strings.Contains(p, a.ID) {
			t.Errorf("left behind: %s", p)
		}
	}
	if list, _ := ps.List(ctx); len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("list after delete: %+v", list)
	}
	if _, _, err := ps.LoadStory(ctx, a.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("LoadStory after delete: %v", err)
	}
}

func TestSaveStoryETagPreconditions(t *testing.T) {
	ps, _, clock := newTestStore(t)
	ctx := context.Background()
	proj, story, etag1, _ := ps.Create(ctx, "Erste Version", "", 0)

	story.Title = "Zweite Version"
	story.Format.FPS = 25
	data, _ := CanonicalJSON(story)
	*clock = clock.Add(time.Minute)
	etag2, err := ps.SaveStory(ctx, proj.ID, data, etag1)
	if err != nil || etag2 == etag1 || etag2 != ETag(data) {
		t.Fatalf("save with the current etag: %v %q", err, etag2)
	}
	// The project record follows the story: title, fps and updated_at.
	got, _ := ps.Get(ctx, proj.ID)
	if got.Title != "Zweite Version" || got.FPS != 25 || !got.UpdatedAt.Equal(*clock) || !got.CreatedAt.Equal(proj.CreatedAt) {
		t.Fatalf("project after save: %+v", got)
	}

	// A stale etag fails and changes nothing.
	story.Title = "Dritte Version"
	data3, _ := CanonicalJSON(story)
	if _, err := ps.SaveStory(ctx, proj.ID, data3, etag1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale etag: %v", err)
	}
	if cur, etag, _ := ps.LoadStory(ctx, proj.ID); etag != etag2 || !strings.Contains(string(cur), "Zweite Version") {
		t.Fatal("a failed save changed the story")
	}
	if _, err := ps.SaveStory(ctx, proj.ID, data3, MustNotExist); !errors.Is(err, ErrConflict) {
		t.Fatalf("MustNotExist on an existing story: %v", err)
	}
	// An unconditional write is allowed.
	if _, err := ps.SaveStory(ctx, proj.ID, data3, ""); err != nil {
		t.Fatalf("unconditional save: %v", err)
	}

	// Size limit, unknown and invalid projects.
	if _, err := ps.SaveStory(ctx, proj.ID, make([]byte, DefaultLimits().MaxStoryBytes+1), ""); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize story: %v", err)
	}
	if _, err := ps.SaveStory(ctx, "00000000-0000-4000-8000-000000000099", data3, ""); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unknown project: %v", err)
	}
	if _, err := ps.SaveStory(ctx, "nope", data3, ""); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("invalid id: %v", err)
	}
}

func TestSaveCodeSceneLifecycle(t *testing.T) {
	ps, mem, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, _ := ps.Create(ctx, "Code", "", 0)
	good := readFixture(t, "testdata/scenes/s3.js")
	scenes := testRoot + "/" + proj.ID + "/scenes/"

	// Create-only, bundle written, no errors file.
	etag, res, err := ps.SaveCodeScene(ctx, proj.ID, "s3", good, MustNotExist)
	if err != nil || len(res.Errors) != 0 || res.Code == nil || etag != ETag(good) {
		t.Fatalf("first save: %v %+v %q", err, res.Errors, etag)
	}
	if got := mem.Paths(); !contains(got, scenes+"s3.js") || !contains(got, scenes+"s3.bundle.js") || contains(got, scenes+"s3.errors.json") {
		t.Fatalf("files after a good save: %v", got)
	}
	bundle, err := ps.LoadCodeBundle(ctx, proj.ID, "s3")
	if err != nil || string(bundle) != string(res.Code) {
		t.Fatalf("bundle: %v", err)
	}
	src, srcEtag, err := ps.LoadCodeSource(ctx, proj.ID, "s3")
	if err != nil || string(src) != string(good) || srcEtag != etag {
		t.Fatalf("source: %v", err)
	}
	status, err := ps.CodeSceneStatus(ctx, proj.ID)
	if err != nil || !reflect.DeepEqual(status, []CodeSceneStatus{{SceneID: "s3", OK: true, Errors: []BundleError{}}}) {
		t.Fatalf("status: %v %+v", err, status)
	}

	// Creating again with MustNotExist, or updating with a stale etag, conflicts and keeps the old state.
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s3", good, MustNotExist); !errors.Is(err, ErrConflict) {
		t.Fatalf("create over existing: %v", err)
	}
	bad := []byte("export default function scene(api) {\n  eval('1');\n}\n")
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s3", bad, `"stale"`); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale etag: %v", err)
	}
	if b, _ := ps.LoadCodeBundle(ctx, proj.ID, "s3"); string(b) != string(res.Code) {
		t.Fatal("a conflicting save changed the bundle")
	}

	// A failing update keeps the source, drops the bundle and records the errors.
	etag2, res2, err := ps.SaveCodeScene(ctx, proj.ID, "s3", bad, etag)
	if err != nil || res2.Code != nil || len(res2.Errors) != 1 || res2.Errors[0].Line != 2 || res2.Errors[0].Column != 3 || etag2 != ETag(bad) {
		t.Fatalf("failing save: %v %+v", err, res2)
	}
	if _, err := ps.LoadCodeBundle(ctx, proj.ID, "s3"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("bundle must be gone: %v", err)
	}
	if got := mem.Paths(); !contains(got, scenes+"s3.js") || contains(got, scenes+"s3.bundle.js") || !contains(got, scenes+"s3.errors.json") {
		t.Fatalf("files after a failing save: %v", got)
	}
	status, _ = ps.CodeSceneStatus(ctx, proj.ID)
	if len(status) != 1 || status[0].OK || !reflect.DeepEqual(status[0].Errors, res2.Errors) {
		t.Fatalf("status after failure: %+v", status)
	}
	var wire []map[string]any
	data, _ := json.Marshal(status)
	_ = json.Unmarshal(data, &wire)
	if _, ok := wire[0]["errors"].([]any)[0].(map[string]any)["column"]; !ok {
		t.Fatalf("status JSON: %s", data)
	}

	// Fixing it again restores the bundle and removes the errors file.
	if _, res3, err := ps.SaveCodeScene(ctx, proj.ID, "s3", good, etag2); err != nil || res3.Code == nil {
		t.Fatalf("fix: %v %+v", err, res3)
	}
	if got := mem.Paths(); !contains(got, scenes+"s3.bundle.js") || contains(got, scenes+"s3.errors.json") {
		t.Fatalf("files after the fix: %v", got)
	}

	// A second scene sorts after; sources over the limit and invalid ids are refused before writing.
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "intro", good, ""); err != nil {
		t.Fatal(err)
	}
	if status, _ := ps.CodeSceneStatus(ctx, proj.ID); len(status) != 2 || status[0].SceneID != "intro" || status[1].SceneID != "s3" {
		t.Fatalf("two scenes: %+v", status)
	}
	big := []byte("export default function scene(api) {}\n" + strings.Repeat("// x\n", 20000))
	if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "big", big, ""); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversize: %v", err)
	}
	if contains(mem.Paths(), scenes+"big.js") {
		t.Fatal("an oversize source was written")
	}
	for _, id := range []string{"", "../x", "A", "a b", strings.Repeat("a", 49)} {
		if _, _, err := ps.SaveCodeScene(ctx, proj.ID, id, good, ""); !errors.Is(err, ErrInvalidID) {
			t.Errorf("scene id %q: %v", id, err)
		}
	}
	if _, _, err := ps.SaveCodeScene(ctx, "00000000-0000-4000-8000-000000000099", "s1", good, ""); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unknown project: %v", err)
	}
}

func TestCodeSceneStatusWithoutScenes(t *testing.T) {
	ps, _, _ := newTestStore(t)
	proj, _, _, _ := ps.Create(context.Background(), "Leer", "", 0)
	status, err := ps.CodeSceneStatus(context.Background(), proj.ID)
	if err != nil || len(status) != 0 {
		t.Fatalf("%v %+v", err, status)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestMemStoreContract(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	if names, err := m.List(ctx, "a/b"); names != nil || err != nil {
		t.Fatalf("missing dir: %v %v", names, err)
	}
	if _, _, err := m.ReadFile(ctx, "a/x"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	e1, err := m.WriteFile(ctx, "a/b/x.txt", []byte("one"), MustNotExist)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.WriteFile(ctx, "a/b/x.txt", []byte("two"), MustNotExist); !errors.Is(err, ErrConflict) {
		t.Fatalf("must-not-exist: %v", err)
	}
	if _, err := m.WriteFile(ctx, "a/b/new.txt", []byte("two"), e1); !errors.Is(err, ErrConflict) {
		t.Fatalf("etag of a missing file: %v", err)
	}
	if _, err := m.WriteFile(ctx, "a/b/x.txt", []byte("two"), e1); err != nil {
		t.Fatal(err)
	}
	_, _ = m.WriteFile(ctx, "a/c.txt", []byte("c"), "")
	_ = m.MkdirAll(ctx, "a/empty/deeper")
	names, _ := m.List(ctx, "a")
	if !reflect.DeepEqual(names, []string{"b", "c.txt", "empty"}) {
		t.Fatalf("list: %v", names)
	}
	if err := m.Remove(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(ctx, "a/b"); err != nil {
		t.Fatalf("remove twice: %v", err)
	}
	if names, _ := m.List(ctx, "a"); !reflect.DeepEqual(names, []string{"c.txt", "empty"}) {
		t.Fatalf("list after remove: %v", names)
	}
}

func TestCodeSceneIssues(t *testing.T) {
	ps, _, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, _ := ps.Create(ctx, "Code", "", 0)
	s := storyOf(t, `{"id":"s1"},{"id":"missing","kind":"code"},{"id":"broken","kind":"code"},{"id":"fine","kind":"code"}`)
	good := readFixture(t, "testdata/scenes/s3.js")
	_, _, _ = ps.SaveCodeScene(ctx, proj.ID, "fine", good, "")
	_, _, _ = ps.SaveCodeScene(ctx, proj.ID, "broken", []byte("export default function scene(api) {\n  fetch('x');\n}\n"), "")
	_, _, _ = ps.SaveCodeScene(ctx, proj.ID, "unused", good, "") // not in the story: ignored
	status, err := ps.CodeSceneStatus(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	issues := CodeSceneIssues(&s, status)
	if len(issues) != 2 {
		t.Fatalf("issues: %s", dump(issues))
	}
	m := expectIssue(t, issues, CodeCodeError, SeverityError, "/scenes/1/kind")
	b := expectIssue(t, issues, CodeCodeError, SeverityError, "/scenes/2/kind")
	if m.SceneID != "missing" || !strings.Contains(m.Message, "no source") || !strings.Contains(m.Fix, "scenes/missing.js") {
		t.Errorf("missing: %+v", m)
	}
	if b.SceneID != "broken" || !strings.Contains(b.Message, "line 2, column 3") || !strings.Contains(b.Message, "fetch") {
		t.Errorf("broken: %+v", b)
	}
}

func TestSaveCodeSceneConcurrentSavesStayConsistent(t *testing.T) {
	ps, _, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, _ := ps.Create(ctx, "Code", "", 0)
	a := []byte("export default function scene(api) { api.duration(1); }\n")
	b := []byte("export default function scene(api) { api.duration(2); }\n")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		src := a
		if i%2 == 1 {
			src = b
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := ps.SaveCodeScene(ctx, proj.ID, "s1", src, ""); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	src, _, _ := ps.LoadCodeSource(ctx, proj.ID, "s1")
	bundle, err := ps.LoadCodeBundle(ctx, proj.ID, "s1")
	if err != nil || !strings.Contains(string(bundle), strings.TrimSpace(strings.TrimSuffix(strings.SplitN(string(src), "{", 2)[1], "}\n"))) {
		t.Fatalf("bundle does not belong to the last source: %v\nsource: %s\nbundle: %s", err, src, bundle)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/ -run 'TestSlugify|TestValidProjectID|TestProject|TestSaveStory|TestSaveCodeScene|TestCodeScene|TestMemStore' -count=1`
Expected: FAIL (build failed: `undefined: NewProjectStore`)

- [ ] **Step 3: Implement**

`internal/storymaker/slug.go`:

```go
package storymaker

import (
	"regexp"
	"strings"
)

var projectIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ValidProjectID reports a lowercase UUID v4, the only form project folders are named by.
func ValidProjectID(id string) bool { return projectIDPattern.MatchString(id) }

// maxSlugLen is the longest slug Slugify returns.
const maxSlugLen = 80

// translit maps letters outside ASCII to ASCII spellings (German umlauts the usual way).
var translit = buildTranslit()

func buildTranslit() map[rune]string {
	m := map[rune]string{
		'ä': "ae", 'ö': "oe", 'ü': "ue", 'Ä': "Ae", 'Ö': "Oe", 'Ü': "Ue", 'ß': "ss", 'ẞ': "SS",
		'æ': "ae", 'Æ': "AE", 'œ': "oe", 'Œ': "OE", 'þ': "th", 'Þ': "Th",
	}
	groups := map[string]string{
		"àáâãåāăąǎ": "a", "ÀÁÂÃÅĀĂĄǍ": "A",
		"çćĉċč": "c", "ÇĆĈĊČ": "C",
		"ďđð": "d", "ĎĐÐ": "D",
		"èéêëēĕėęě": "e", "ÈÉÊËĒĔĖĘĚ": "E",
		"ĝğġģ": "g", "ĜĞĠĢ": "G",
		"ĥ": "h", "Ĥ": "H",
		"ìíîïĩīĭįı": "i", "ÌÍÎÏĨĪĬĮİ": "I",
		"ĵ": "j", "Ĵ": "J",
		"ķ": "k", "Ķ": "K",
		"ĺļľł": "l", "ĹĻĽŁ": "L",
		"ñńņň": "n", "ÑŃŅŇ": "N",
		"òóôõōŏőøǒ": "o", "ÒÓÔÕŌŎŐØǑ": "O",
		"ŕŗř": "r", "ŔŖŘ": "R",
		"śŝşš": "s", "ŚŜŞŠ": "S",
		"ţťŧ": "t", "ŢŤŦ": "T",
		"ùúûũūŭůűųǔ": "u", "ÙÚÛŨŪŬŮŰŲǓ": "U",
		"ŵ": "w", "Ŵ": "W",
		"ýÿŷ": "y", "ÝŸŶ": "Y",
		"źżž": "z", "ŹŻŽ": "Z",
	}
	for letters, ascii := range groups {
		for _, r := range letters {
			m[r] = ascii
		}
	}
	return m
}

// windowsReserved are device names Windows refuses as file names (with or without extension).
func windowsReserved(name string) bool {
	n := strings.ToUpper(name)
	switch n {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(n) == 4 && (strings.HasPrefix(n, "COM") || strings.HasPrefix(n, "LPT")) && n[3] >= '1' && n[3] <= '9' {
		return true
	}
	return false
}

// Slugify turns a title into a safe file-name stem: ASCII letters, digits and single dashes,
// at most 80 characters, case kept. Umlauts and other Latin letters are transliterated
// (ä becomes ae, ß becomes ss). When nothing usable remains, or the result is a Windows
// reserved device name (CON, PRN, AUX, NUL, COM1-9, LPT1-9), it returns fallback (the project
// id), or "storymaker" when that is empty too.
//
//	Slugify("Was ist eine Firewall?", id) == "Was-ist-eine-Firewall"
func Slugify(title, fallback string) string {
	var b strings.Builder
	dash := true
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
			continue
		}
		if t, ok := translit[r]; ok {
			b.WriteString(t)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxSlugLen {
		out = strings.Trim(out[:maxSlugLen], "-")
	}
	if out == "" || windowsReserved(out) {
		if fallback == "" {
			return "storymaker"
		}
		return fallback
	}
	return out
}
```

`internal/storymaker/memstore.go`:

```go
package storymaker

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
)

// MemStore is an in-memory Store for tests of this package and of the packages that build on
// it. It follows the Store contract exactly, including etags, MustNotExist, recursive Remove
// and List of a missing directory.
type MemStore struct {
	mu    sync.Mutex
	files map[string][]byte
	dirs  map[string]bool
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{files: map[string][]byte{}, dirs: map[string]bool{}}
}

func memClean(p string) string {
	return strings.Trim(path.Clean(strings.ReplaceAll(p, "\\", "/")), "/")
}

// ReadFile implements Store.
func (m *MemStore) ReadFile(_ context.Context, p string) ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[memClean(p)]
	if !ok {
		return nil, "", fmt.Errorf("storymaker: %s: %w", p, fs.ErrNotExist)
	}
	return append([]byte(nil), data...), ETag(data), nil
}

// WriteFile implements Store.
func (m *MemStore) WriteFile(_ context.Context, p string, data []byte, ifMatch string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memClean(p)
	cur, exists := m.files[key]
	switch {
	case ifMatch == "":
	case ifMatch == MustNotExist:
		if exists {
			return "", ErrConflict
		}
	default:
		if !exists || ETag(cur) != ifMatch {
			return "", ErrConflict
		}
	}
	if m.dirs[key] {
		return "", fmt.Errorf("storymaker: %s is a directory", p)
	}
	m.mkdirs(path.Dir(key))
	m.files[key] = append([]byte(nil), data...)
	return ETag(data), nil
}

// Remove implements Store: it removes a file or a directory tree; a missing path is fine.
func (m *MemStore) Remove(_ context.Context, p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memClean(p)
	delete(m.files, key)
	delete(m.dirs, key)
	prefix := key + "/"
	for f := range m.files {
		if strings.HasPrefix(f, prefix) {
			delete(m.files, f)
		}
	}
	for d := range m.dirs {
		if strings.HasPrefix(d, prefix) {
			delete(m.dirs, d)
		}
	}
	return nil
}

// List implements Store: the base names of the direct children, sorted; (nil, nil) when the
// directory does not exist.
func (m *MemStore) List(_ context.Context, dir string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memClean(dir)
	prefix := key + "/"
	if key == "." || key == "" {
		prefix = ""
	}
	seen := map[string]bool{}
	collect := func(full string) {
		if !strings.HasPrefix(full, prefix) || full == strings.TrimSuffix(prefix, "/") {
			return
		}
		rest := strings.TrimPrefix(full, prefix)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			seen[rest] = true
		}
	}
	for f := range m.files {
		collect(f)
	}
	for d := range m.dirs {
		collect(d)
	}
	if len(seen) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// MkdirAll implements Store.
func (m *MemStore) MkdirAll(_ context.Context, dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mkdirs(memClean(dir))
	return nil
}

func (m *MemStore) mkdirs(dir string) {
	for dir != "." && dir != "" && dir != "/" {
		m.dirs[dir] = true
		dir = path.Dir(dir)
	}
}

// Paths lists every file path in the store, sorted. It is a test helper.
func (m *MemStore) Paths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.files))
	for f := range m.files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// Dirs lists every directory path in the store, sorted. It is a test helper.
func (m *MemStore) Dirs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.dirs))
	for d := range m.dirs {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
```

`internal/storymaker/project.go`:

```go
package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/desktop"
	"github.com/google/uuid"
)

// Store errors.
var (
	// ErrConflict reports a failed precondition: a stale ETag, or MustNotExist on an existing file.
	ErrConflict = errors.New("storymaker: precondition failed")
	// ErrInvalidID reports a project or scene id that is not in the accepted form.
	ErrInvalidID = errors.New("storymaker: invalid project or scene id")
	// ErrInvalidArgument reports an unusable title, aspect or frame rate.
	ErrInvalidArgument = errors.New("storymaker: invalid argument")
	// ErrTooLarge reports story or code content over its size limit.
	ErrTooLarge = errors.New("storymaker: content too large")
)

// MustNotExist is the ifMatch value of Store.WriteFile that creates a file only when it is new.
const MustNotExist = "-"

// Store is the file access ProjectStore needs. The server implements it over the desktop
// workspace; NewMemStore is the in-memory implementation for tests. Paths are rooted desktop
// paths such as "Documents/Storymaker/<id>/story.json".
//
// Semantics: an etag is ETag(bytes). ReadFile of a missing file wraps fs.ErrNotExist.
// WriteFile creates missing parent directories and takes an ifMatch of "" (unconditional),
// MustNotExist (create only) or an etag (the file must exist and still have it); a failed
// precondition returns ErrConflict. Remove behaves like os.RemoveAll: a missing path is not an
// error and a directory is removed with everything below it. List returns the base names of
// the direct children of dir (files and directories); a missing directory is (nil, nil).
type Store interface {
	ReadFile(ctx context.Context, path string) (data []byte, etag string, err error)
	WriteFile(ctx context.Context, path string, data []byte, ifMatch string) (etag string, err error)
	Remove(ctx context.Context, path string) error
	List(ctx context.Context, dir string) ([]string, error)
	MkdirAll(ctx context.Context, dir string) error
}

// ETag is the strong version of a file: the quoted SHA-256 of its bytes, the same format the
// desktop notes use.
func ETag(data []byte) string { return desktop.NoteVersion(data) }

// Project is the project record in project.json and in the API.
type Project struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Aspect        string    `json:"aspect"`
	FPS           int       `json:"fps"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	TargetSeconds int       `json:"target_seconds,omitempty"`
	Audience      string    `json:"audience,omitempty"`
	Template      string    `json:"template,omitempty"`
}

// CodeSceneStatus is the bundle state of one code scene.
type CodeSceneStatus struct {
	SceneID string        `json:"scene_id"`
	OK      bool          `json:"ok"`
	Errors  []BundleError `json:"errors"`
}

// ProjectStore keeps projects in <root>/<id>/ on top of a Store:
//
//	story.json            the story document (ETag-protected writes)
//	project.json          the project record
//	scenes/<id>.js        code-scene source
//	scenes/<id>.bundle.js the bundled classic script, written on a successful save
//	scenes/<id>.errors.json the bundle errors of the last failed save
//	assets/  audio/narration/  audio/music/  exports/   created empty by Create
type ProjectStore struct {
	st    Store
	root  string
	lim   Limits
	now   func() time.Time
	newID func() string
	// codeMu serialises SaveCodeScene, so two saves cannot leave a bundle that belongs to
	// the other save's source.
	codeMu sync.Mutex
}

// NewProjectStore returns a store rooted at root, for example "Documents/Storymaker".
func NewProjectStore(st Store, root string) *ProjectStore {
	root = strings.Trim(path.Clean(strings.ReplaceAll(root, "\\", "/")), "/")
	return &ProjectStore{
		st: st, root: root, lim: DefaultLimits(),
		now:   func() time.Time { return time.Now().UTC().Truncate(time.Second) },
		newID: func() string { return uuid.NewString() },
	}
}

func (p *ProjectStore) dir(id string) string { return path.Join(p.root, id) }

func (p *ProjectStore) file(id string, parts ...string) string {
	return path.Join(append([]string{p.root, id}, parts...)...)
}

func (p *ProjectStore) checkID(id string) error {
	if !ValidProjectID(id) {
		return fmt.Errorf("%w: project %q", ErrInvalidID, clip(id, 40))
	}
	return nil
}

func (p *ProjectStore) checkScene(id, sceneID string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	if !validSlug(sceneID) {
		return fmt.Errorf("%w: scene %q", ErrInvalidID, clip(sceneID, 40))
	}
	return nil
}

// Create makes a project. An empty aspect means 16:9 and fps 0 means 30. The new story has the
// title, the format, the default style and one empty scene s1 on env.plain-stage, so the editor
// opens on something that renders; defaults are applied before the first write. It returns the
// project, the story and the etag of story.json.
func (p *ProjectStore) Create(ctx context.Context, title, aspect string, fps int) (Project, Story, string, error) {
	title = strings.TrimSpace(title)
	if title == "" || runeLen(title) > 120 {
		return Project{}, Story{}, "", fmt.Errorf("%w: the title must have 1 to 120 characters", ErrInvalidArgument)
	}
	if aspect == "" {
		aspect = DefaultAspect
	}
	if aspect != "16:9" && aspect != "9:16" && aspect != "1:1" {
		return Project{}, Story{}, "", fmt.Errorf("%w: aspect must be 16:9, 9:16 or 1:1", ErrInvalidArgument)
	}
	if fps == 0 {
		fps = DefaultFPS
	}
	if fps != 24 && fps != 25 && fps != 30 {
		return Project{}, Story{}, "", fmt.Errorf("%w: fps must be 24, 25 or 30", ErrInvalidArgument)
	}

	id := p.newID()
	now := p.now()
	story := Story{
		Schema: Schema, Title: title,
		Format: Format{Aspect: aspect, FPS: fps},
		Style:  Style{Palette: DefaultPalette, Font: DefaultFont, Captions: true},
		Cast:   []CastMember{},
		Scenes: []Scene{{ID: "s1"}},
	}
	ApplyDefaults(&story)
	storyBytes, err := CanonicalJSON(story)
	if err != nil {
		return Project{}, Story{}, "", err
	}
	proj := Project{ID: id, Title: title, Aspect: aspect, FPS: fps, CreatedAt: now, UpdatedAt: now}
	projBytes, err := encodeProject(proj)
	if err != nil {
		return Project{}, Story{}, "", err
	}

	for _, sub := range []string{"", "assets", "audio/narration", "audio/music", "exports"} {
		if err := p.st.MkdirAll(ctx, p.file(id, sub)); err != nil {
			p.cleanup(id)
			return Project{}, Story{}, "", fmt.Errorf("storymaker: create project folders: %w", err)
		}
	}
	etag, err := p.st.WriteFile(ctx, p.file(id, "story.json"), storyBytes, MustNotExist)
	if err == nil {
		_, err = p.st.WriteFile(ctx, p.file(id, "project.json"), projBytes, MustNotExist)
	}
	if err != nil {
		p.cleanup(id)
		return Project{}, Story{}, "", fmt.Errorf("storymaker: create project: %w", err)
	}
	return proj, story, etag, nil
}

// cleanup removes a half-created project; a failure to clean up is not reported.
func (p *ProjectStore) cleanup(id string) {
	_ = p.st.Remove(context.Background(), p.dir(id))
}

func encodeProject(pr Project) ([]byte, error) {
	b, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("storymaker: encode project: %w", err)
	}
	return append(b, '\n'), nil
}

// List returns the projects, most recently updated first. Folders that are not valid project
// ids or whose project.json cannot be read are skipped.
func (p *ProjectStore) List(ctx context.Context) ([]Project, error) {
	names, err := p.st.List(ctx, p.root)
	if err != nil {
		return nil, fmt.Errorf("storymaker: list projects: %w", err)
	}
	var out []Project
	for _, name := range names {
		id := path.Base(name)
		if !ValidProjectID(id) {
			continue
		}
		pr, err := p.Get(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, pr)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Get reads project.json. A missing project wraps fs.ErrNotExist.
func (p *ProjectStore) Get(ctx context.Context, id string) (Project, error) {
	pr, _, err := p.readProject(ctx, id)
	return pr, err
}

func (p *ProjectStore) readProject(ctx context.Context, id string) (Project, string, error) {
	if err := p.checkID(id); err != nil {
		return Project{}, "", err
	}
	data, etag, err := p.st.ReadFile(ctx, p.file(id, "project.json"))
	if err != nil {
		return Project{}, "", err
	}
	var pr Project
	if err := json.Unmarshal(data, &pr); err != nil {
		return Project{}, "", fmt.Errorf("storymaker: project %s: %w", id, err)
	}
	pr.ID = id
	return pr, etag, nil
}

// LoadStory returns the bytes of story.json and their etag.
func (p *ProjectStore) LoadStory(ctx context.Context, id string) ([]byte, string, error) {
	if err := p.checkID(id); err != nil {
		return nil, "", err
	}
	return p.st.ReadFile(ctx, p.file(id, "story.json"))
}

// SaveStory writes story.json (data is stored as given: the caller decodes, validates and
// canonicalises first) and returns the new etag. ifMatch is "" for an unconditional write or
// the etag the caller read; a stale etag returns ErrConflict. A successful write updates
// project.json: updated_at, and the title, aspect and fps read from the story.
func (p *ProjectStore) SaveStory(ctx context.Context, id string, data []byte, ifMatch string) (string, error) {
	if err := p.checkID(id); err != nil {
		return "", err
	}
	if p.lim.MaxStoryBytes > 0 && len(data) > p.lim.MaxStoryBytes {
		return "", fmt.Errorf("%w: story.json is %d bytes, the limit is %d", ErrTooLarge, len(data), p.lim.MaxStoryBytes)
	}
	if _, _, err := p.st.ReadFile(ctx, p.file(id, "project.json")); err != nil {
		return "", err
	}
	etag, err := p.st.WriteFile(ctx, p.file(id, "story.json"), data, ifMatch)
	if err != nil {
		return "", err
	}
	p.touch(ctx, id, data)
	return etag, nil
}

// touch bumps updated_at and mirrors the story's title and format into project.json. It is
// best effort: the story is already saved, so a failure here is not reported.
func (p *ProjectStore) touch(ctx context.Context, id string, story []byte) {
	var head struct {
		Title  string `json:"title"`
		Format struct {
			Aspect string `json:"aspect"`
			FPS    int    `json:"fps"`
		} `json:"format"`
	}
	_ = json.Unmarshal(story, &head)
	for attempt := 0; attempt < 3; attempt++ {
		pr, etag, err := p.readProject(ctx, id)
		if err != nil {
			return
		}
		pr.UpdatedAt = p.now()
		if t := strings.TrimSpace(head.Title); t != "" && runeLen(t) <= 120 {
			pr.Title = t
		}
		if head.Format.Aspect != "" {
			pr.Aspect = head.Format.Aspect
		}
		if head.Format.FPS != 0 {
			pr.FPS = head.Format.FPS
		}
		b, err := encodeProject(pr)
		if err != nil {
			return
		}
		if _, err := p.st.WriteFile(ctx, p.file(id, "project.json"), b, etag); !errors.Is(err, ErrConflict) {
			return
		}
	}
}

// SaveCodeScene writes the source of a code scene and bundles it. ifMatch follows
// Store.WriteFile: "" unconditional, MustNotExist for a new scene, or the source etag.
// On success it writes scenes/<id>.bundle.js and removes scenes/<id>.errors.json. When bundling
// fails it keeps the source, removes the bundle and writes scenes/<id>.errors.json; the error
// list is in the result and the call still succeeds, so the editor can show it. Source over the
// code size limit is refused with ErrTooLarge before anything is written.
func (p *ProjectStore) SaveCodeScene(ctx context.Context, id, sceneID string, source []byte, ifMatch string) (string, BundleResult, error) {
	if err := p.checkScene(id, sceneID); err != nil {
		return "", BundleResult{}, err
	}
	p.codeMu.Lock()
	defer p.codeMu.Unlock()
	if p.lim.MaxCodeBytes > 0 && len(source) > p.lim.MaxCodeBytes {
		return "", BundleResult{}, fmt.Errorf("%w: the code scene is %d bytes, the limit is %d", ErrTooLarge, len(source), p.lim.MaxCodeBytes)
	}
	if _, _, err := p.st.ReadFile(ctx, p.file(id, "project.json")); err != nil {
		return "", BundleResult{}, err
	}
	if err := p.st.MkdirAll(ctx, p.file(id, "scenes")); err != nil {
		return "", BundleResult{}, fmt.Errorf("storymaker: create scenes folder: %w", err)
	}
	etag, err := p.st.WriteFile(ctx, p.codePath(id, sceneID, ".js"), source, ifMatch)
	if err != nil {
		return "", BundleResult{}, err
	}
	res := BundleCodeScene(source, sceneID, p.lim.MaxCodeBytes)
	if len(res.Errors) == 0 {
		if _, err := p.st.WriteFile(ctx, p.codePath(id, sceneID, ".bundle.js"), res.Code, ""); err != nil {
			return etag, res, fmt.Errorf("storymaker: write bundle: %w", err)
		}
		if err := p.st.Remove(ctx, p.codePath(id, sceneID, ".errors.json")); err != nil {
			return etag, res, fmt.Errorf("storymaker: remove errors: %w", err)
		}
		return etag, res, nil
	}
	if err := p.st.Remove(ctx, p.codePath(id, sceneID, ".bundle.js")); err != nil {
		return etag, res, fmt.Errorf("storymaker: remove bundle: %w", err)
	}
	b, err := json.MarshalIndent(res.Errors, "", "  ")
	if err == nil {
		_, err = p.st.WriteFile(ctx, p.codePath(id, sceneID, ".errors.json"), append(b, '\n'), "")
	}
	if err != nil {
		return etag, res, fmt.Errorf("storymaker: write errors: %w", err)
	}
	return etag, res, nil
}

func (p *ProjectStore) codePath(id, sceneID, ext string) string {
	return p.file(id, "scenes", sceneID+ext)
}

// LoadCodeSource returns the source of a code scene and its etag.
func (p *ProjectStore) LoadCodeSource(ctx context.Context, id, sceneID string) ([]byte, string, error) {
	if err := p.checkScene(id, sceneID); err != nil {
		return nil, "", err
	}
	return p.st.ReadFile(ctx, p.codePath(id, sceneID, ".js"))
}

// LoadCodeBundle returns the bundled script of a code scene; fs.ErrNotExist when the last save
// failed or the scene was never saved.
func (p *ProjectStore) LoadCodeBundle(ctx context.Context, id, sceneID string) ([]byte, error) {
	if err := p.checkScene(id, sceneID); err != nil {
		return nil, err
	}
	data, _, err := p.st.ReadFile(ctx, p.codePath(id, sceneID, ".bundle.js"))
	return data, err
}

// CodeSceneStatus lists every code scene that has a source file, sorted by scene id, with the
// result of its last save.
func (p *ProjectStore) CodeSceneStatus(ctx context.Context, id string) ([]CodeSceneStatus, error) {
	if err := p.checkID(id); err != nil {
		return nil, err
	}
	names, err := p.st.List(ctx, p.file(id, "scenes"))
	if err != nil {
		return nil, fmt.Errorf("storymaker: list scenes: %w", err)
	}
	have := map[string]bool{}
	for _, n := range names {
		have[path.Base(n)] = true
	}
	var out []CodeSceneStatus
	for name := range have {
		if !strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".bundle.js") {
			continue
		}
		sceneID := strings.TrimSuffix(name, ".js")
		if !validSlug(sceneID) {
			continue
		}
		st := CodeSceneStatus{SceneID: sceneID, Errors: []BundleError{}}
		switch {
		case have[sceneID+".bundle.js"]:
			st.OK = true
		case have[sceneID+".errors.json"]:
			if data, _, err := p.st.ReadFile(ctx, p.codePath(id, sceneID, ".errors.json")); err == nil {
				_ = json.Unmarshal(data, &st.Errors)
			}
		}
		if !st.OK && len(st.Errors) == 0 {
			st.Errors = []BundleError{{Line: 1, Column: 1, Message: "The scene has not been bundled yet; save it again."}}
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SceneID < out[j].SceneID })
	return out, nil
}

// CodeSceneIssues reports, for every scene of kind "code", a code_error issue when the scene has
// no source file or its last bundle failed (first error with line and column). Sources of scenes
// that are not in the story are ignored. A render needs this list to be empty.
func CodeSceneIssues(s *Story, status []CodeSceneStatus) []Issue {
	byID := make(map[string]CodeSceneStatus, len(status))
	for _, st := range status {
		byID[st.SceneID] = st
	}
	var out []Issue
	for i, sc := range s.Scenes {
		if sc.Kind != "code" {
			continue
		}
		st, ok := byID[sc.ID]
		switch {
		case !ok:
			out = append(out, Issue{Severity: SeverityError, SceneID: sc.ID, Path: jp("scenes", i, "kind"), Code: CodeCodeError,
				Message: "The code scene has no source yet.",
				Fix:     "Write scenes/" + sc.ID + ".js with export default function scene(api) { ... }, or set kind to \"storyboard\"."})
		case !st.OK:
			msg := "The code scene did not bundle."
			if len(st.Errors) > 0 {
				e := st.Errors[0]
				msg = fmt.Sprintf("The code scene did not bundle (line %d, column %d): %s", e.Line, e.Column, e.Message)
			}
			out = append(out, Issue{Severity: SeverityError, SceneID: sc.ID, Path: jp("scenes", i, "kind"), Code: CodeCodeError,
				Message: msg, Fix: "Fix the reported line in the code scene and save it again."})
		}
	}
	return out
}

// Delete removes a project folder with everything in it. A missing project is not an error.
func (p *ProjectStore) Delete(ctx context.Context, id string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	return p.st.Remove(ctx, p.dir(id))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/storymaker/ -run 'TestSlugify|TestValidProjectID|TestProject|TestSaveStory|TestSaveCodeScene|TestCodeScene|TestMemStore' -count=1`, then `go test ./internal/storymaker/ -count=1`
Expected: PASS (`ok  aurago/internal/storymaker`)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/slug.go internal/storymaker/memstore.go internal/storymaker/project.go internal/storymaker/slug_test.go internal/storymaker/project_test.go
git commit -m "feat(storymaker): project store with ETag preconditions and code-scene status" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A8: Package contract and final check

**Files:**
- Create: `internal/storymaker/AGENTS.md`

(A document has no failing test; the step order is adapted: write, verify, commit.)

- [ ] **Step 1: Write the package contract**

`internal/storymaker/AGENTS.md`:

```markdown
# Storymaker package contract

## Purpose

Go core of the Storymaker desktop app (explainer videos from a story document): the `storymaker/1` story model, strict
decoding, validation, defaults, the catalog index, code-scene bundling and the project storage layout. No server
dependency. Spec: `docs/superpowers/specs/2026-11-01-storymaker-design.md`. The player that renders the story is
JavaScript (`ui/js/storymaker/`); Go never recomputes the timeline.

## Ownership

- `model.go` types and JSON (polymorphic `Position` and `Duration` keep an unreadable value as `Invalid` so Validate reports
  it at the right path); `decode.go` `DecodeStory` and the field hints; `validate.go` + `issues.go` `Validate`,
  `CapIssues`; `defaults.go` `ApplyDefaults`; `catalog.go` the `storymaker-catalog/1` index; `codebundle.go`
  `BundleCodeScene`; `project.go` + `memstore.go` + `slug.go` storage, `MemStore` (test helper), `Slugify`.
- `testdata/stories/*.json`, `testdata/scenes/*` (with the golden `s3.bundle.js`) and `testdata/catalog-mini/catalog.json`
  are shared fixtures: the player tests read the same files. `catalog-mini` has only the manifest; the files it
  references belong to the player part.
- The render job, listener and browser live in `render/`; HTTP handlers in `internal/server/storymaker_*.go`. Config is
  `internal/config/storymaker.go`.

## Local Contracts

- Issue shape `{severity, scene_id, path, code, message, at_seconds, fix}`; `path` is a JSON Pointer. `error` blocks a
  render, `warning` does not, `minor` is `not_supported_yet` and `canonicalised`. Codes are the constants in `model.go`;
  the app localises them as `storymaker.issue.<code>`. Model-facing answers carry at most `MaxModelIssues` (`CapIssues`).
- `DecodeStory` fails with a `*DecodeError` that always carries issues. Unknown fields are reported with the path, a
  hint for common mistakes and the allowed fields; key matching is exact. Wrong types become `invalid_json` with the path.
- `Validate` canonicalises prefixed names in place (`pose.typing` becomes `typing`, with a `canonicalised` note), knows the
  closed pose/gesture/expression/transition/ease sets, reports names the catalog lacks as `not_supported_yet` and unknown
  names as `unknown_name` with at most 5 alternatives. A nil catalog skips asset, spot and catalog-name checks.
- The phase 1 support matrix lives in `model.go` (`phase1Beats`, `phase1Transitions`) and in `validate.go`; move a beat or
  transition out of "not supported yet" there when the player gains it.
- `ApplyDefaults` fills only the static defaults of the spec and the show-beat instance id; timing (`delay`, `duration`,
  `ease`) is never defaulted: the player's compile step owns it. It is idempotent.
- `LoadCatalog` checks the manifest only; `VerifyFiles` checks that referenced files exist. The catalog is immutable after
  loading. Search is term scoring over id, name and tags (default 6 results, cap 12).
- `BundleCodeScene`: esbuild `Transform` (IIFE, global `__smSceneModule`, ES2020, no legal comments) plus the appended
  `self.__smScene=__smSceneModule.default;`. The pre-check is a tokenizer over the author's source and is defence in depth
  only; the CSP and the worker realm are the barriers. A slash after `)`, `}`, `++` or `--` is read both ways. Positions
  are 1-based and count characters. The golden bundle is reproduced byte for byte by Node esbuild 0.28.2 in the player tests.
- `Store` is the only file access of `ProjectStore`. `Remove` behaves like `os.RemoveAll`; etags are
  `desktop.NoteVersion` (`ETag`); a failed precondition is `ErrConflict`. Project ids are lowercase UUID v4, scene ids slugs;
  every path is built from validated ids only.
- Slugs for file names: ASCII letters, digits and dashes, umlauts transliterated, Windows device names refused, 80 characters.

## Work Guidance

- Keep one responsibility per file. New issue codes or fields need the model, the validator, the hint text
  and the app's `storymaker.issue.<code>` locale keys in step.
- Do not edit the `catalog-mini` fixture or the story fixtures without updating the player's Node tests that read them.
- Regenerate the golden bundle only for an intended esbuild change: `STORYMAKER_UPDATE_GOLDEN=1 go test ./internal/storymaker -run TestBundleGolden`.

## Verification

- `go test ./internal/storymaker/... -count=1` and `go test ./internal/config/... -count=1`.

## Child DOX Index

None yet.
```

- [ ] **Step 2: Run the final checks**

```bash
gofmt -l internal/storymaker internal/config/storymaker.go internal/config/storymaker_test.go
go vet ./internal/storymaker/ ./internal/config/
go test ./internal/storymaker/... ./internal/config/... -count=1
git diff --check
```

Expected: `gofmt -l` prints nothing, `go vet` is silent, both packages print `ok`, `git diff --check` reports no whitespace errors (on Windows it may
warn that LF becomes CRLF for the `.js` fixtures; that is fine). The root `AGENTS.md`
child-DOX row for `internal/storymaker/` belongs to part E.

- [ ] **Step 3: Commit**

Run `node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo .` first (CLAUDE.md).

```bash
git add -f internal/storymaker/AGENTS.md   # .gitignore ignores nested AGENTS.md files; -f is needed for a new one
git commit -m "docs(storymaker): package contract for the Go core" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
