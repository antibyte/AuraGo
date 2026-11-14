# Part B

Tasks of batch B: composition v2 in the player, `Beat.scale`, `style.compose`, and the agent projects that get it. Rules, commands and the order are in the plan index. Decisions are in [design.md](design.md), table "Batch B".

Batch B is independent of batch A and of batch C. It touches the Go model and validator, one agent project creator, the agent tool schema, the player (JavaScript, CRLF files) and the two generated player bundles.

**JavaScript edits.** The player parts are CRLF files. A multi-line edit with the Edit tool would mix line endings, so every JS edit below is a small script for the helper `crlf-edit.mjs` of the index ("Setup"). Write the script into the scratch folder with the Write tool, then run it from the worktree root with `node <scratch>/<name>.mjs`. Each `edit(file, old, new)` replaces exactly one occurrence and throws when it finds none or more than one.

<!-- group B1 -->

### Task B1: `Beat.scale` and `style.compose` in the model
**Batch:** B  **Depends on:** none
**Files:**
Modify `internal/storymaker/model.go`
Modify `internal/storymaker/validate.go`
Test `internal/storymaker/compose_model_test.go`

`style.compose` is `""` (the current look) or `"v2"`. It is never written by `ApplyDefaults`, so every existing story stays as it is. `Beat.scale` is the size of a shown prop; it uses the same range as `Actor.Scale` and `Prop.Scale` (0.5 to 2, `validate.go:597`).

- [ ] **Step 1: Impact:** `Style`, `Beat`, `validator.style`, `validator.beat`. `Beat` and `Style` have many readers: report the risk, expect HIGH for `Beat` (strict decode, canonical JSON, editor), and keep the change additive (a new `omitempty` field).
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/model.go`
Old:
```go
	AutoSFX  bool   `json:"auto_sfx,omitempty"`
```
New:
```go
	AutoSFX  bool   `json:"auto_sfx,omitempty"`
	Compose  string `json:"compose,omitempty"` // "" | v2: the composition rules of the player (caption-safe floor, hero prop, push-in); only agent projects set it
```

**Edit** `internal/storymaker/model.go`
Old:
```go
	Entry      string     `json:"entry,omitempty"`
```
New:
```go
	Entry      string     `json:"entry,omitempty"`
	Scale      float64    `json:"scale,omitempty"` // 0.5 to 2, a show beat with an item only; 0 = unset
```

**Edit** `internal/storymaker/validate.go`: the compose value, at the top of `style()`.
Old:
```go
	st := &v.s.Style
```
New:
```go
	st := &v.s.Style
	switch st.Compose {
	case "", "v2":
	default:
		v.errorf(CodeInvalidValue, "/style/compose", fmt.Sprintf("The composition %q is not supported.", clip(st.Compose, 20)), `Leave "compose" out, or use "v2".`)
	}
```
The line `st := &v.s.Style` is unique in `validate.go`; if the file has more than one, use the one inside `func (v *validator) style()`.

**Edit** `internal/storymaker/validate.go`: the beat scale, before `needWho` in `beat()`.
Old:
```go
	needWho := func(enter bool) {
```
New:
```go
	if b.Scale != 0 {
		v.scale(bb+"/scale", b.Scale)
		if b.Do != "show" || b.Item == "" {
			v.warnf(CodeInvalidValue, bb+"/scale", `A beat scale only applies to a "show" beat with an "item".`, `Remove "scale", or use it on a show beat: {"do":"show","item":"prop.lock","at":"center","entry":"pop","scale":1.5}.`)
		}
	}
	needWho := func(enter bool) {
```
- [ ] **Step 3: Test.** Create `internal/storymaker/compose_model_test.go` (it builds its story inline, so batch B does not depend on the fixtures of batch A):

{% raw %}
```go
package storymaker

import (
	"strings"
	"testing"
)

// composeStory has a show beat (index 0) and a gesture beat (index 1) in one scene.
const composeStory = `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},
"style":{"palette":"sunny","font":"default","captions":true},
"cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],
"scenes":[{"id":"s1","environment":"env.office","narration":"Eine {1}Firewall schuetzt das Netz.","speaker":"anna",
"actors":[{"cast":"anna","at":"left","pose":"present"}],
"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1},{"do":"gesture","who":"anna","gesture":"point","with":"prev"}]}]}`

func TestBeatScaleRoundTripsAndIsRangeChecked(t *testing.T) {
	cat := loadReal(t)
	s := mustDecode(t, composeStory)
	s.Scenes[0].Beats[0].Scale = 1.5
	raw, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"scale": 1.5`) {
		t.Fatalf("canonical JSON lacks the beat scale:\n%s", raw)
	}
	back := mustDecode(t, string(raw))
	if back.Scenes[0].Beats[0].Scale != 1.5 {
		t.Fatalf("scale after a round trip = %v", back.Scenes[0].Beats[0].Scale)
	}
	if issues := Validate(&back, cat, DefaultLimits()); HasErrors(issues) {
		t.Fatalf("a scale of 1.5 is valid: %s", dump(issues))
	}

	for _, bad := range []float64{0.2, 2.5, -1} {
		s.Scenes[0].Beats[0].Scale = bad
		found := false
		for _, is := range Validate(&s, cat, DefaultLimits()) {
			if is.Path == "/scenes/0/beats/0/scale" && is.Severity == SeverityError {
				found = true
			}
		}
		if !found {
			t.Errorf("scale %v must be refused at /scenes/0/beats/0/scale", bad)
		}
	}

	// A scale on a gesture beat is a warning, not an error.
	s.Scenes[0].Beats[0].Scale = 0
	s.Scenes[0].Beats[1].Scale = 1.2
	warned := false
	for _, is := range Validate(&s, cat, DefaultLimits()) {
		if is.Path == "/scenes/0/beats/1/scale" {
			warned = is.Severity == SeverityWarning
		}
	}
	if !warned {
		t.Error("a scale on a gesture beat must be a warning")
	}
}

func TestStyleComposeIsWrittenOnlyWhenSetAndAcceptsOnlyV2(t *testing.T) {
	cat := loadReal(t)
	s := mustDecode(t, composeStory)
	raw, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "compose") {
		t.Fatalf("compose must not be written when it is not set:\n%s", raw)
	}

	s.Style.Compose = "v2"
	raw, err = CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"compose": "v2"`) {
		t.Fatalf("canonical JSON lacks compose:\n%s", raw)
	}
	back := mustDecode(t, string(raw))
	if back.Style.Compose != "v2" {
		t.Fatalf("compose after a round trip = %q", back.Style.Compose)
	}
	if issues := Validate(&back, cat, DefaultLimits()); HasErrors(issues) {
		t.Fatalf("v2 is valid: %s", dump(issues))
	}

	s.Style.Compose = "v3"
	found := false
	for _, is := range Validate(&s, cat, DefaultLimits()) {
		if is.Path == "/style/compose" && is.Severity == SeverityError {
			found = true
		}
	}
	if !found {
		t.Error("compose v3 must be refused at /style/compose")
	}

	// ApplyDefaults never sets it.
	plain := Story{Schema: Schema, Title: "x", Format: Format{Aspect: "16:9", FPS: 30}, Scenes: []Scene{{ID: "s1"}}}
	ApplyDefaults(&plain)
	if plain.Style.Compose != "" {
		t.Errorf("ApplyDefaults set compose to %q", plain.Style.Compose)
	}
}
```
{% endraw %}
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestBeatScale|TestStyleCompose)' -count=1 -v`. Expected: PASS. Then the shape tests that this change could touch: `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '(Decode|Validate|Canonical|Style|Defaults)' -count=1`. Expected: PASS.
- [ ] **Step 5: Commit:** `gofmt -w internal/storymaker/model.go internal/storymaker/validate.go internal/storymaker/compose_model_test.go`; `git add` the three files; message `feat(storymaker): beat scale and style.compose in the model and the validator`.

<!-- group B2 -->

### Task B2: Agent projects get composition v2
**Batch:** B  **Depends on:** B1
**Files:**
Modify `internal/storymaker/voices.go`
Modify `internal/server/storymaker_agent_handlers.go`
Test `internal/storymaker/compose_create_test.go`

`CreateWithVoice` is also the editor's path (`storymaker_handlers.go:384`, `project.go:144`). A new options struct lets the agent path set `compose` without changing the editor.

- [ ] **Step 1: Impact:** `CreateWithVoice`, `storymakerAgentCreateProject`. Callers of the first: `Create`, the editor handler, the agent creator.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/voices.go`
Old:
```go
func (p *ProjectStore) CreateWithVoice(ctx context.Context, title, aspect string, fps int, voice *Voice) (Project, Story, string, error) {
```
New:
```go
func (p *ProjectStore) CreateWithVoice(ctx context.Context, title, aspect string, fps int, voice *Voice) (Project, Story, string, error) {
	return p.CreateWithOptions(ctx, title, aspect, fps, voice, CreateOptions{})
}

// CreateOptions are the choices of a new project that the editor does not make. Compose is the story's style.compose: the
// agent sets "v2", the editor leaves it empty, so existing and hand-made films keep their look.
type CreateOptions struct {
	Compose string
}

// CreateWithOptions is CreateWithVoice with options.
func (p *ProjectStore) CreateWithOptions(ctx context.Context, title, aspect string, fps int, voice *Voice, opts CreateOptions) (Project, Story, string, error) {
```

**Edit** `internal/storymaker/voices.go`
Old:
```go
		Style:  Style{Palette: DefaultPalette, Font: DefaultFont, Captions: true},
```
New:
```go
		Style:  Style{Palette: DefaultPalette, Font: DefaultFont, Captions: true, Compose: opts.Compose},
```

**Edit** `internal/server/storymaker_agent_handlers.go`
Old:
```go
	project, _, _, err := acc.rt.projects.CreateWithVoice(ctx, title, aspect, 0, nil)
```
New:
```go
	// An agent project gets the composition rules of the player: captions clear of the floor, a readable hero prop, a slow push-in.
	project, _, _, err := acc.rt.projects.CreateWithOptions(ctx, title, aspect, 0, nil, storymaker.CreateOptions{Compose: "v2"})
```
`storymaker_agent_handlers.go` already imports `aurago/internal/storymaker`; if it does not, add the import.
- [ ] **Step 3: Test.** Create `internal/storymaker/compose_create_test.go`:

```go
package storymaker

import (
	"context"
	"testing"
)

func TestCreateWithOptionsSetsComposeOnlyWhenAsked(t *testing.T) {
	ctx := context.Background()
	ps := newTestStore(t)

	_, plain, _, err := ps.Create(ctx, "Editor film", "16:9", 0)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Style.Compose != "" {
		t.Errorf("the editor path must not set compose, got %q", plain.Style.Compose)
	}

	project, v2, _, err := ps.CreateWithOptions(ctx, "Agent film", "16:9", 0, nil, CreateOptions{Compose: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	if v2.Style.Compose != "v2" {
		t.Fatalf("compose = %q, want v2", v2.Style.Compose)
	}
	raw, _, err := ps.LoadStory(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved := mustDecode(t, string(raw))
	if saved.Style.Compose != "v2" {
		t.Fatalf("the saved story lost compose: %q", saved.Style.Compose)
	}
}
```
`newTestStore(t)` is the helper the other store tests use (`newStudioWriteRig` calls it).
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^(TestCreateWithOptions|TestProject|TestCreate)' -count=1`, and `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go build ./internal/server`. Expected: PASS and a clean build.
- [ ] **Step 5: Commit:** `gofmt -w` the three files; `git add internal/storymaker/voices.go internal/server/storymaker_agent_handlers.go internal/storymaker/compose_create_test.go`; message `feat(storymaker): agent projects are created with composition v2`.

<!-- group B3 -->

### Task B3: Tell the agent about `scale`
**Batch:** B  **Depends on:** B1
**Files:**
Modify `internal/agent/native_tools_storymaker.go`
Modify `internal/storymaker/reference/animation.md`
Modify `internal/agent/native_tools_storymaker_test.go` (only if a test pins the beat property set)

The prompt texts are not touched here: batch A bumps the profile to v3, and a published text is never edited in place. The agent learns about `scale` from the tool schema and from the reference document (`storymaker_catalog` operation `reference`, topic `animation`).

- [ ] **Step 1: Impact:** `storymakerBeatSchema`. Callers: the scene schema and both tool modes.
- [ ] **Step 2: Implement.**

**Edit** `internal/agent/native_tools_storymaker.go`
Old:
```go
	properties["ease"] = storymakerEnum("linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out")
```
New:
```go
	properties["ease"] = storymakerEnum("linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out")
	properties["scale"] = storymakerTyped("number")
```

**Edit** `internal/storymaker/reference/animation.md` (LF file): the show row of the beat table.
Old:
```
| show | item or text, at, style, entry pop/fade/slide/none, optional id |
```
New:
```
| show | item or text, at, style, entry pop/fade/slide/none, optional id, optional scale 0.5 to 2 for an item |
```

**Edit** `internal/storymaker/reference/animation.md`: composition rules, inserted before the layout bullet of "Stage and composition".
Old:
```
- Check 16:9, 9:16 and 1:1 composition: keep faces
```
New:
```
- In a film the agent makes (`style.compose` is `v2`), the player keeps actors and shown props above the caption band, enlarges the first cue-bound prop of a scene to about a quarter of the frame height, and moves the camera in slowly onto the speaker. A scene with its own camera (a framing other than wide, a camera target or a camera beat) is left alone. Give a show beat `scale` (0.5 to 2) when a prop should have another size: an explicit scale always wins.
- Check 16:9, 9:16 and 1:1 composition: keep faces
```
- [ ] **Step 3: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/agent -run Storymaker -count=1`. Expected: PASS. If a test pins the exact beat property set (`native_tools_storymaker_test.go`, around line 153) or the compact schema size (900 estimated tokens per tool, `internal/storymaker/AGENTS.md`), add `scale` to that list and keep the size under the budget. Then `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '(Reference|Studio)' -count=1`. Expected: PASS.
- [ ] **Step 4: Commit:** `gofmt -w internal/agent/native_tools_storymaker.go`; `git add internal/agent/native_tools_storymaker.go internal/storymaker/reference/animation.md` (plus the test file if changed); message `feat(storymaker): beat scale in the agent tool schema and the animation reference`.

<!-- group B4 -->

### Task B4: Caption-safe floor in the stage
**Batch:** B  **Depends on:** B1
**Files:**
Modify `ui/js/storymaker/stage.js`
Modify `ui/js/storymaker/scene.js`
Modify `ui/js/storymaker/compile.js`
Modify `ui/js/storymaker/render.js`
Create `ui/js/storymaker/compose.js`
Modify `scripts/storymaker-bundle-parts.js`
Create `scripts/storymaker-tests/29-compose-v2.mjs`

With captions on, the bottom caption box covers 79.4 to 92 % of the frame height in 16:9 and 1:1 (two lines), and every base spot has its feet at 88 %. In v2 the feet of the base spots move to 74 % (the spec said 76; the push-in reaches zoom 1.08, which moves the feet down on screen by up to 2 %, and 74 keeps at least 3 % between the feet and the band at the end of a scene). The depth of the old feet line stays, so actors keep their size, and the x position does not change. 9:16 needs no move: its band is 69.3 to 82 % and the feet are at 88 %. Spots that are not base spots (`desk`, `door`, `window`, `board`) and explicit `{x, y}` points stay as they are.

A prop at a spot did not keep the depth of that spot, because `createProp` does not store it (`scene.js:72-80`). The player now stores it, so a prop at a v2 base spot keeps the size it has today. No spot of the starter catalog has a `depth`, so no v1 output changes.

- [ ] **Step 1: Impact:** `create` in `stage.js` (`SM.stage.create`), `createProp`, `makeContext`, `stageFor`. Callers of `SM.stage.create`: `compile.js:93`, `codescene.js:38`, `render.js` `stageFor`. Run the impact check from the main checkout and report it.
- [ ] **Step 2: Create `ui/js/storymaker/compose.js`** (the Write tool writes LF; keep the file LF, the build normalises line endings):

```js
// Storymaker player: composition v2. A story with style.compose "v2" (the agent's projects) gets three rules on top of the
// stage: the base spots stand above the caption band (stage.js), the first cue-bound prop of a scene is enlarged to about a
// quarter of the frame height, and a scene without its own camera pushes in slowly. A story without compose keeps every
// position, size and camera as before. Classic script; publishes StorymakerCore.compose.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var HERO_HEIGHT = 0.24;                 // of the frame height: the height of a scene's hero prop
    var HERO_MIN = 0.5, HERO_MAX = 3;       // the scale that makes it so is clamped to this range
    var PUSH_IN = 1.08, PUSH_IN_CALM = 1.04;

    // isV2(story): true for style.compose "v2".
    function isV2(story) { return !!(story && story.style && story.style.compose === 'v2'); }

    // heroScale(entry, depth): the scale that makes a prop of the catalog entry HERO_HEIGHT of the frame height at the given
    // depth scale: size.h * depth * scale = HERO_HEIGHT. An entry without size.h counts 0.15, as the renderers do.
    function heroScale(entry, depth) {
        var h = entry && entry.size && entry.size.h > 0 ? entry.size.h : 0.15;
        var d = depth > 0 ? depth : 1;
        var s = Math.round(HERO_HEIGHT / (h * d) * 1000) / 1000;
        return s < HERO_MIN ? HERO_MIN : (s > HERO_MAX ? HERO_MAX : s);
    }

    // hasOwnCamera(scene): the author chose a camera, so the automatic push-in stays off. ApplyDefaults writes framing wide
    // into every scene, so only a framing other than wide, a target or a camera beat counts.
    function hasOwnCamera(scene) {
        var cam = scene && scene.camera;
        if (cam && ((cam.framing && cam.framing !== 'wide') || cam.target)) return true;
        var beats = scene && scene.beats || [];
        for (var i = 0; i < beats.length; i++) if (beats[i] && beats[i].do === 'camera') return true;
        return false;
    }

    // pushInAmount(style): the zoom a scene reaches by its end; rhythm calm moves less.
    function pushInAmount(style) { return style && style.rhythm === 'calm' ? PUSH_IN_CALM : PUSH_IN; }

    SM.compose = {
        isV2: isV2, heroScale: heroScale, hasOwnCamera: hasOwnCamera, pushInAmount: pushInAmount,
        HERO_HEIGHT: HERO_HEIGHT, HERO_MIN: HERO_MIN, HERO_MAX: HERO_MAX, PUSH_IN: PUSH_IN, PUSH_IN_CALM: PUSH_IN_CALM
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```
- [ ] **Step 3: Register the part.** Write `<scratch>/edit-B4-parts.mjs`:

```js
import { edit } from './crlf-edit.mjs';
edit('scripts/storymaker-bundle-parts.js',
  "  'ui/js/storymaker/beats.js',",
  "  'ui/js/storymaker/compose.js',\n  'ui/js/storymaker/beats.js',");
console.log('part list extended');
```
- [ ] **Step 4: The stage.** Write `<scratch>/edit-B4-stage.mjs`:

```js
import { edit } from './crlf-edit.mjs';
const F = 'ui/js/storymaker/stage.js';

edit(F,
  "    var SLICE_MARGIN = 0.03;   // a spot closer than this fraction of the slice width to its edge counts as outside",
  "    var SLICE_MARGIN = 0.03;   // a spot closer than this fraction of the slice width to its edge counts as outside\n" +
  "    // Composition v2 (style.compose): with captions on, the feet line of the base spots moves above the caption band, which\n" +
  "    // starts at 79.4 percent of the height in 16:9 and 1:1 (9:16 has its band higher and needs no move). The depth scale of\n" +
  "    // the old feet line is kept, so actors and props keep their size.\n" +
  "    var V2_FEET_Y = 74;");

edit(F,
  "    // create({aspect, width, height, env, safe}): env is a catalog environment entry (spots, floor, aspects) or null.",
  "    // create({aspect, width, height, env, safe, compose, captions}): env is a catalog environment entry (spots, floor, aspects)\n" +
  "    // or null. compose 'v2' with captions on (not false) lifts the base spots above the caption band, except in 9:16.");

edit(F,
  "        var floor = env && env.floor ? env.floor : DEFAULT_FLOOR;\n",
  "        var floor = env && env.floor ? env.floor : DEFAULT_FLOOR;\n" +
  "        var captionSafe = opts.compose === 'v2' && opts.captions !== false && aspect !== '9:16';\n");

edit(F,
  "            cropped: cropped, focusX: visX + visW / 2, env: env\n",
  "            cropped: cropped, focusX: visX + visW / 2, env: env, captionSafe: captionSafe\n");

edit(F,
  "        stage.hasSpot = function (name) {\n" +
  "            return !!((env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name]);\n" +
  "        };\n",
  "        stage.hasSpot = function (name) {\n" +
  "            return !!((env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name]);\n" +
  "        };\n" +
  "\n" +
  "        // floorT(yPct): the depth of a feet line, 0 at y_far to 1 at y_near.\n" +
  "        function floorT(yPct) { var span = floor.y_near - floor.y_far; return span === 0 ? 1 : clamp((yPct - floor.y_far) / span, 0, 1); }\n");

edit(F,
  "            var face = (o && o.face) || src.face || null;\n" +
  "            return { name: name, x: x, y: yPct / 100 * DESIGN_H, face: face, depth: typeof src.depth === 'number' ? src.depth : null };\n",
  "            var face = (o && o.face) || src.face || null;\n" +
  "            var depth = typeof src.depth === 'number' ? src.depth : null;\n" +
  "            if (captionSafe && yPct > V2_FEET_Y && BASE_SPOTS.indexOf(name) >= 0) {\n" +
  "                // the depth of the old feet line becomes explicit before the feet move up, so the size stays\n" +
  "                if (depth === null) depth = floorT(yPct);\n" +
  "                yPct = V2_FEET_Y;\n" +
  "            }\n" +
  "            return { name: name, x: x, y: yPct / 100 * DESIGN_H, face: face, depth: depth };\n");

edit(F,
  "        DEFAULT_SAFE: DEFAULT_SAFE, DEFAULT_FLOOR: DEFAULT_FLOOR, DEFAULT_SPOTS: DEFAULT_SPOTS\n",
  "        DEFAULT_SAFE: DEFAULT_SAFE, DEFAULT_FLOOR: DEFAULT_FLOOR, DEFAULT_SPOTS: DEFAULT_SPOTS, V2_FEET_Y: V2_FEET_Y\n");
console.log('stage.js edited');
```
If one `edit` throws "old text not found", read that region of `stage.js` and adjust the Old text: the first line of a pair is the part that must match exactly.
- [ ] **Step 5: Props keep their spot's depth, and compile and render pass the flags.** Write `<scratch>/edit-B4-plumbing.mjs`:

```js
import { edit } from './crlf-edit.mjs';

// scene.js: a prop keeps the depth of its spot, as an actor does (no spot of the starter catalog has one, so v1 is unchanged)
edit('ui/js/storymaker/scene.js',
  "    // createProp({id, item, x, y, scale, layer, hidden})",
  "    // createProp({id, item, x, y, scale, layer, hidden, depth})");
edit('ui/js/storymaker/scene.js',
  "        e.layer = def.layer === 'back' ? 'back' : 'front';\n",
  "        e.layer = def.layer === 'back' ? 'back' : 'front';\n" +
  "        e.depth = typeof def.depth === 'number' ? def.depth : null;\n");

// compile.js: the compose flag reaches the stage of every storyboard scene and the timeline style
edit('ui/js/storymaker/compile.js',
  "        var captionsOn = !(story.style && story.style.captions === false);\n",
  "        var captionsOn = !(story.style && story.style.captions === false);\n" +
  "        var composeStyle = SM.compose.isV2(story) ? 'v2' : '';\n");
edit('ui/js/storymaker/compile.js',
  "env: envEntry, safe: catalog && catalog.stage && catalog.stage.safe });",
  "env: envEntry, safe: catalog && catalog.stage && catalog.stage.safe, compose: composeStyle, captions: captionsOn });");
edit('ui/js/storymaker/compile.js',
  "style: { font: styleFont, captions: !(story.style && story.style.captions === false) },",
  "style: { font: styleFont, captions: !(story.style && story.style.captions === false), compose: composeStyle },");

// render.js: the render context carries them to stageFor (code scenes never get v2)
edit('ui/js/storymaker/render.js',
  "styleFont: timeline.style.font, safe: timeline.safe || SM.stage.DEFAULT_SAFE,",
  "styleFont: timeline.style.font, safe: timeline.safe || SM.stage.DEFAULT_SAFE,\n" +
  "            compose: timeline.style.compose || '', captions: timeline.style.captions !== false,");
edit('ui/js/storymaker/render.js',
  "sc.stages[key] = SM.stage.create({ aspect: rc.aspect, width: rc.W, height: rc.H, env: sc.env, safe: rc.safe });",
  "sc.stages[key] = SM.stage.create({ aspect: rc.aspect, width: rc.W, height: rc.H, env: sc.env, safe: rc.safe, compose: sc.kind === 'code' ? '' : rc.compose, captions: rc.captions });");
console.log('plumbing edited');
```
Run `node <scratch>/edit-B4-parts.mjs`, `node <scratch>/edit-B4-stage.mjs`, `node <scratch>/edit-B4-plumbing.mjs`. If `captionsOn` is declared after the line that creates the stage in your copy of `compile.js`, move the two `var` lines above `scenesIn.forEach` (they must come before the scene loop).
- [ ] **Step 6: The test.** Create `scripts/storymaker-tests/29-compose-v2.mjs` (the numerical acceptance of the floor):

```js
// Composition v2 (style.compose "v2"): the base spots above the caption band with the old depth, the hero prop at a quarter of
// the frame height, the automatic push-in, and every v1 story untouched.
import assert from 'node:assert/strict';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();

// film(): one scene on plain-stage with an actor at left and two props that show on cue marker 1.
function film(extraStyle, extraScene) {
    const s = {
        schema: 'storymaker/1', title: 'Compose', format: { aspect: '16:9', fps: 30 },
        style: Object.assign({ palette: 'sunny', font: 'default', captions: true }, extraStyle),
        cast: [{ id: 'anna', name: 'Anna', character: 'char.office-woman-1' }],
        scenes: [{
            id: 's1', environment: 'env.plain-stage', narration: 'Eine {1}Firewall prueft jedes Paket am Eingang.', speaker: 'anna',
            actors: [{ cast: 'anna', at: 'left', pose: 'stand' }],
            beats: [
                { do: 'show', item: 'prop.lock', at: 'center', entry: 'pop', cue: 1 },
                { do: 'show', item: 'prop.shield', at: 'right', entry: 'pop', cue: 1 }
            ]
        }]
    };
    return Object.assign(s.scenes[0], extraScene) && s;
}
const compile = story => SM.compile(story, {}, t.catalog);
const entitiesOf = out => out.timeline.scenes[0].entities;
const plainEnv = () => t.byId('env.plain-stage');

check('compose v2: the base spots lift above the caption band and keep the old depth scale', () => {
    const v1 = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const v2 = SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: true });
    for (const name of ['far-left', 'left', 'center', 'right', 'far-right']) {
        const a = v1.spot(name), b = v2.spot(name);
        near(b.x, a.x, 1e-9, name + ' x');
        near(b.y, 0.74 * 1080, 1e-9, name + ' feet line');
        assert.ok(b.y < 0.794 * 1080, name + ': the feet stand above the top of the two-line caption band (79.4 percent)');
        near(v2.depthScale(b.y, b.depth), v1.depthScale(a.y, a.depth), 1e-9, name + ' depth scale');
    }
    near(v2.spot('board').y, v1.spot('board').y, 1e-9, 'the board is no base spot');
    assert.deepEqual(v2.resolve({ x: 50, y: 90 }), v1.resolve({ x: 50, y: 90 }), 'an explicit point stays where it is');
    assert.equal(v2.captionSafe, true);
});

check('compose v2: no change without captions, in 9:16, without compose, or for another value', () => {
    const base = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const cases = [
        SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: false }),
        SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: '', captions: true }),
        SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v3', captions: true })
    ];
    for (const stage of cases) near(stage.spot('left').y, base.spot('left').y, 1e-9, 'unchanged');
    const tall1 = SM.stage.create({ aspect: '9:16', env: plainEnv() });
    const tall2 = SM.stage.create({ aspect: '9:16', env: plainEnv(), compose: 'v2', captions: true });
    near(tall2.spot('left').y, tall1.spot('left').y, 1e-9, '9:16 has its caption band higher and needs no move');
    assert.equal(tall2.captionSafe, false);
    const square = SM.stage.create({ aspect: '1:1', env: plainEnv(), compose: 'v2', captions: true });
    near(square.spot('left').y, 0.74 * 1080, 1e-9, '1:1 has the same caption band as 16:9');
});

check('compose v2: a compiled story keeps x and the actor size and lifts the feet; a story without compose is unchanged', () => {
    const a = compile(film()), b = compile(film({ compose: 'v2' }));
    const actorA = entitiesOf(a).find(e => e.type === 'actor'), actorB = entitiesOf(b).find(e => e.type === 'actor');
    near(actorB.x.at(0), actorA.x.at(0), 1e-9, 'x');
    near(actorA.y.at(0), 0.88 * 1080, 1e-9, 'v1 feet');
    near(actorB.y.at(0), 0.74 * 1080, 1e-9, 'v2 feet');
    const s1 = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const s2 = SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: true });
    near(s2.depthScale(actorB.y.at(0), actorB.depth), s1.depthScale(actorA.y.at(0), actorA.depth), 1e-9, 'the same size');
    assert.equal(a.timeline.style.compose, '');
    assert.equal(b.timeline.style.compose, 'v2');
});

check('compose v2: a prop at a base spot keeps the size it has in v1 (createProp stores the spot depth)', () => {
    const a = compile(film()), b = compile(film({ compose: 'v2' }));
    const propA = entitiesOf(a).find(e => e.item === 'prop.shield'), propB = entitiesOf(b).find(e => e.item === 'prop.shield');
    const s1 = SM.stage.create({ aspect: '16:9', env: plainEnv() });
    const s2 = SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: true });
    near(s2.depthScale(propB.y.at(0), propB.depth), s1.depthScale(propA.y.at(0), propA.depth), 1e-9);
});
```
(The file continues in Step 7 with the tests of the hero prop and the push-in, after Task B5 has written their code. Write the file as it stands now.)
- [ ] **Step 7: Build and run what exists.** `npm run build:ui` (rebuilds `ui/js/storymaker/player.bundle.js` with the new part), then `node scripts/test-storymaker-player.mjs`. Expected: all checks `ok`, including the four new ones; `10-static-guards` accepts the new part (under 600 lines, no timers). If an older check fails because a prop now has `depth`, read it: only a story with a spot that carries `depth` can change, and the starter catalog has none, so the failure points to a real difference worth understanding before going on.
- [ ] **Step 8: Commit:** `git add ui/js/storymaker/stage.js ui/js/storymaker/scene.js ui/js/storymaker/compile.js ui/js/storymaker/render.js ui/js/storymaker/compose.js ui/js/storymaker/player.bundle.js scripts/storymaker-bundle-parts.js scripts/storymaker-tests/29-compose-v2.mjs`; message `feat(storymaker): caption-safe floor for composition v2`.

<!-- group B5 -->

### Task B5: Hero prop and push-in
**Batch:** B  **Depends on:** B4
**Files:**
Modify `ui/js/storymaker/beats.js`
Modify `scripts/storymaker-tests/29-compose-v2.mjs`

`beats.js` has 528 of its 600 allowed lines. The three hooks below add about 25 lines; the logic stays in `compose.js`.

The hero prop is the first show beat of a scene that has a cue and an item. It gets `scale = 0.24 / (size.h * depth)` (clamped to 0.5 to 3) so that it stands about a quarter of the frame height high at any spot. An explicit `scale` on that beat wins, and the beat still counts as the hero, so a later show beat is not enlarged.

The push-in zooms from 1.0 to 1.08 (1.04 for rhythm `calm`) over the scene, eased, and pans sideways to the speaker, or to the hero prop when nobody speaks; the vertical focus stays, so the feet do not slide down toward the caption band. It is off for a scene with its own camera.

- [ ] **Step 1: Impact:** `buildStoryboard`, `showTarget` (a closure inside it). Callers of `buildStoryboard`: `compile.js:106`.
- [ ] **Step 2: Implement.** Write `<scratch>/edit-B5-beats.mjs`:

```js
import { edit } from './crlf-edit.mjs';
const F = 'ui/js/storymaker/beats.js';

// 1. the compose state of the scene
edit(F,
  "        var f = SM.motion.factor(c.story && c.story.style), DEF = SM.motion.DEFAULTS;\n",
  "        var f = SM.motion.factor(c.story && c.story.style), DEF = SM.motion.DEFAULTS;\n" +
  "        var composeV2 = SM.compose.isV2(c.story), hero = null;   // hero: the prop that composition v2 enlarges\n");

// 2. the hero prop, in showTarget
edit(F,
  "            var prop = S.createProp({ id: instanceId(b.item, b.id), item: b.item, x: at.x, y: at.y, scale: b.scale, layer: b.layer, hidden: true, depth: at.depth });\n" +
  "            register(prop, b.item);\n",
  "            // composition v2: the first cue-bound prop of the scene is the hero; an explicit scale wins\n" +
  "            var isHero = composeV2 && !hero && !!b.cue, scale = b.scale;\n" +
  "            if (isHero && !(scale > 0)) scale = SM.compose.heroScale(entry, stage.depthScale(at.y, at.depth));\n" +
  "            var prop = S.createProp({ id: instanceId(b.item, b.id), item: b.item, x: at.x, y: at.y, scale: scale, layer: b.layer, hidden: true, depth: at.depth });\n" +
  "            if (isHero) hero = prop;\n" +
  "            register(prop, b.item);\n");

// 3. the push-in, before the motion end is measured
edit(F,
  "        var motionEnd = SM.camera.end(cam);\n",
  "        // composition v2: a scene without its own camera pushes in slowly onto the speaker (or the hero prop)\n" +
  "        if (composeV2 && !SM.compose.hasOwnCamera(scene)) {\n" +
  "            var pushEnd = Math.max(beatsEnd, plan.narration.start + plan.narration.seconds), pushRef = speakerActor || hero;\n" +
  "            var pushFocus = pushRef ? focusOf(pushRef.id, 0) : null;\n" +
  "            if (pushEnd > 0) {\n" +
  "                SM.camera.addZoom(cam, 0, pushEnd, SM.compose.pushInAmount(c.story.style), 'ease-in-out');\n" +
  "                if (pushFocus) SM.camera.addPan(cam, 0, pushEnd, pushFocus.x, cam.cy.at(0), 'ease-in-out');\n" +
  "            }\n" +
  "        }\n" +
  "        var motionEnd = SM.camera.end(cam);\n");
console.log('beats.js edited');
```
Run it with `node <scratch>/edit-B5-beats.mjs`. The part must stay under 600 lines: `wc -l ui/js/storymaker/beats.js` (expect about 553).
- [ ] **Step 3: Extend the test.** Append to `scripts/storymaker-tests/29-compose-v2.mjs`:

```js

check('compose v2: heroScale puts a prop at a quarter of the frame height and clamps', () => {
    near(SM.compose.heroScale({ size: { h: 0.14 } }, 0.973), 0.24 / (0.14 * 0.973), 0.001);
    assert.equal(SM.compose.heroScale({ size: { h: 0.5 } }, 1), 0.5, 'clamped at the bottom');
    assert.equal(SM.compose.heroScale({ size: { h: 0.05 } }, 1), 3, 'clamped at the top');
    near(SM.compose.heroScale({}, 1), 0.24 / 0.15, 0.001, 'an entry without size.h counts 0.15');
    near(SM.compose.heroScale({ size: { h: 0.1 } }, 0), 2.4, 0.001, 'no depth counts 1');
});

check('compose v2: the first cue-bound prop is the hero, an explicit scale wins, v1 is untouched', () => {
    const stage = SM.stage.create({ aspect: '16:9', env: plainEnv(), compose: 'v2', captions: true });
    const props = entitiesOf(compile(film({ compose: 'v2' }))).filter(e => e.type === 'prop');
    const lock = props.find(e => e.item === 'prop.lock'), shield = props.find(e => e.item === 'prop.shield');
    near(t.byId('prop.lock').size.h * stage.depthScale(lock.y.at(0), lock.depth) * lock.baseScale, 0.24, 0.002, 'the hero stands a quarter of the frame height high');
    assert.equal(shield.baseScale, 1, 'only the first cue-bound show is the hero');

    const explicit = film({ compose: 'v2' });
    explicit.scenes[0].beats[0].scale = 1.2;
    const heroWithScale = entitiesOf(compile(explicit)).find(e => e.item === 'prop.lock');
    assert.equal(heroWithScale.baseScale, 1.2, 'an explicit scale wins');
    assert.equal(entitiesOf(compile(explicit)).find(e => e.item === 'prop.shield').baseScale, 1, 'and still counts as the hero');

    for (const e of entitiesOf(compile(film())).filter(x => x.type === 'prop')) assert.equal(e.baseScale, 1, 'v1 props keep their size');

    const uncued = film({ compose: 'v2' });
    delete uncued.scenes[0].beats[0].cue; delete uncued.scenes[0].beats[1].cue;
    for (const e of entitiesOf(compile(uncued)).filter(x => x.type === 'prop')) assert.equal(e.baseScale, 1, 'a show without a cue is no hero');
});

check('compose v2: a scene without its own camera pushes in onto the speaker, an own camera switches it off, v1 stays still', () => {
    const zoomAt = (out, s) => SM.camera.at(out.timeline.scenes[0].camera, s).zoom;
    const push = compile(film({ compose: 'v2' }));
    near(zoomAt(push, 0), 1, 1e-9, 'starts wide');
    near(zoomAt(push, 60), 1.08, 1e-6, 'ends at 1.08');
    assert.ok(zoomAt(push, 2) > 1 && zoomAt(push, 2) < 1.08, 'and moves in between');
    near(zoomAt(compile(film({ compose: 'v2', rhythm: 'calm' })), 60), 1.04, 1e-6, 'calm moves less');

    const framed = compile(film({ compose: 'v2' }, { camera: { framing: 'medium' } }));
    near(zoomAt(framed, 0), 1.35, 1e-9, 'an own framing wins');
    near(zoomAt(framed, 60), 1.35, 1e-9, 'and nothing pushes in');
    const targeted = compile(film({ compose: 'v2' }, { camera: { framing: 'wide', target: 'left' } }));
    near(zoomAt(targeted, 60), 1, 1e-9, 'a camera target counts as an own camera');
    const withBeat = film({ compose: 'v2' });
    withBeat.scenes[0].beats.push({ do: 'camera', op: 'zoom', amount: 1.25 });
    near(zoomAt(compile(withBeat), 60), 1.25, 1e-6, 'a camera beat switches the push-in off');

    near(zoomAt(compile(film()), 60), 1, 1e-9, 'a story without compose never pushes in');
    assert.equal(SM.compose.hasOwnCamera({ camera: { framing: 'wide' }, beats: [] }), false, 'the default wide framing is no choice');
});
```
`film(extraStyle, extraScene)`: the second argument is merged into the scene by `Object.assign(s.scenes[0], extraScene)`; the helper's `return Object.assign(...) && s` returns the story.
- [ ] **Step 4: Run:** `npm run build:ui` then `node scripts/test-storymaker-player.mjs`. Expected: every check `ok`. If the clamped zoom check fails because the camera zoom track interpolates in log space, compare with `1e-6` as written; if `zoomAt(push, 60)` is not 1.08, print `out.timeline.scenes[0].camera.zoom` and check that `pushEnd` is greater than 0 (the scene has narration, so it is).
- [ ] **Step 5: Commit:** `git add ui/js/storymaker/beats.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/29-compose-v2.mjs`; message `feat(storymaker): hero prop and slow push-in for composition v2`.

<!-- group B6 -->

### Task B6: Documents and the batch end
**Batch:** B  **Depends on:** B5
**Files:**
Modify `docs/superpowers/specs/2026-11-01-storymaker-contract.md` (git-ignored: commit with `-f` through the temp index)
Modify `internal/storymaker/AGENTS.md`
Modify `ui/js/storymaker/AGENTS.md`

- [ ] **Step 1: The contract.** In `docs/superpowers/specs/2026-11-01-storymaker-contract.md`:

**Edit** the style line (line 29)
Old:
```
Style       {palette:string, font:string, captions:bool, rhythm?:"calm"|"standard"|"lively", auto_sfx?:bool}
```
New:
```
Style       {palette:string, font:string, captions:bool, rhythm?:"calm"|"standard"|"lively", auto_sfx?:bool, compose?:"v2"}
```
Then run `grep -n "style.rhythm. is not written when absent" docs/superpowers/specs/2026-11-01-storymaker-contract.md` and add after that line: `- \`style.compose\` is not written when absent. \`"v2"\` switches on the composition rules of the player: with captions on, the feet line of the base spots stands at 74 percent of the height instead of 88 in 16:9 and 1:1 (the depth scale of the old line is kept), the first cue-bound show of a scene with an item is enlarged to about 24 percent of the frame height unless it has a \`scale\`, and a scene without its own camera zooms from 1.0 to 1.08 (1.04 for rhythm calm). A story without \`compose\` renders exactly as before. Only agent-created projects carry it.`
Then run `grep -n "scale" docs/superpowers/specs/2026-11-01-storymaker-contract.md | head` and, in the line that lists the beat fields (near line 42 to 44), add `scale?:number` (0.5 to 2, a show beat with an item only). If the beat is described in a table, add a row.
- [ ] **Step 2: The package documents.** In `internal/storymaker/AGENTS.md` add one bullet to the model section (find the bullet that mentions `style.rhythm` or `auto_sfx`, around lines 34 to 36): `- \`style.compose\` (\`"v2"\`) and \`Beat.scale\` (0.5 to 2, show with item) are additive, never defaulted and never written when absent. Only \`ProjectStore.CreateWithOptions\` with \`CreateOptions{Compose: "v2"}\` sets compose, and only the agent's project creator calls it.` In `ui/js/storymaker/AGENTS.md` add to the part list or the support matrix (lines 34 and 35): `- \`compose.js\` holds the composition v2 rules (hero scale, push-in, camera test); \`stage.js\` applies the caption-safe floor. Both are inert without \`style.compose: "v2"\`.`
- [ ] **Step 3: Commit the code docs:** `git add internal/storymaker/AGENTS.md ui/js/storymaker/AGENTS.md`; message `docs(storymaker): composition v2 and beat scale`. Then commit the contract through the temp index (see the index, "Commits"), message `docs(storymaker): contract for style.compose and beat scale`.

## Batch B end

Run the batch matrix of the index from the batch B worktree. The numerical acceptance of the floor is in test `29-compose-v2.mjs`. The visual acceptance (feet above the caption band, the hero prop at about a quarter of the frame height, a push-in that does not cut a prop off) needs a rendered still, which batch E provides: task E6 renders the firewall run with and without `compose: "v2"` and is where `V2_FEET_Y` (74) and `HERO_HEIGHT` (0.24) are tuned if a still shows a problem. A tuning change is one small commit in `stage.js` or `compose.js` with the bundle rebuilt.
