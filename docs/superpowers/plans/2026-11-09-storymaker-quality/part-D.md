# Part D

Tasks of batch D: the player measures the layout of every storyboard scene during the compile run, Go turns the boxes into model-fixable findings, and a film with a clean layout, clean timing and nothing open can finally be `ready`. Rules, commands and the order are in the plan index. Decisions are in [design.md](design.md), table "Batch D".

**Depends on batches A and B.** B: the hero flag lives in `beats.js` next to the hero code of B5, and the caption rule is only meaningful for composition v2. A: the issue class (`IssueCraft`, `issueNeverBlocks`) and the test helpers and fixtures (`loadAgentFixture`, `saveFixture`, `agent-varied`) that the Go tests below use. Batch C is not needed, but when it has landed the lines it changed (`CheckReport`, `CHECK_KEYS`) are named in the edits.

How it works. `compile()` is synchronous, runs in the worker before the fonts and the art are loaded, and has no canvas, so it cannot measure. A new worker message `layout` runs after `prepare()`, when the rig, the props and the fonts exist. `host-render.js` sends it when `boot.layout` is true, which only the compile check sets, and adds the answer to the report body as `layout`. `compile()` output is untouched (the compile golden `04-compile.mjs:207-212` stays valid). In the layout run every storyboard scene is sampled once, at its hold frame: the moment after its last cue arrival and its last motion, plus half a second. For each visible actor, prop and text the player reports a box in fractions of the frame, and the caption box for that moment. Go decides what is wrong.

<!-- group D1 -->

### Task D1: The player measures
**Batch:** D  **Depends on:** B5, A2
**Files:**
Modify `ui/js/storymaker/render.js`
Modify `ui/js/storymaker/beats.js`
Create `ui/js/storymaker/layout.js`
Modify `ui/js/storymaker/worker-player.js`
Modify `ui/js/storymaker/host-core.js`
Modify `ui/js/storymaker/host-render.js`
Modify `scripts/storymaker-bundle-parts.js`
Create `scripts/storymaker-tests/30-layout.mjs`

- [ ] **Step 1: Impact:** `drawCaptions`, `showTarget` closure in `buildStoryboard`, `Runtime.handle`, the `LIMITS` object, `mountRender` in `host-render.js`. `SM.render` is used by `raster.js`, `codescene.js` and the tests: the change adds exports only. Report callers and risk.
- [ ] **Step 2: `render.js` exports `entityBox`, `textBox` and a caption box.** Write `<scratch>/edit-D1-render.mjs` and run it from the worktree root:

```js
import { edit } from './crlf-edit.mjs';
const F = 'ui/js/storymaker/render.js';

// drawCaptions: the layout moves into captionLayout, so the layout run measures exactly what is drawn
edit(F,
  "    function drawCaptions(ctx, timeline, n, rc) {\n" +
  "        if (!timeline.style.captions || !timeline.captions || !timeline.captions.length) return;\n" +
  "        var cap = SM.captions.at(timeline.captions, n * 1000 / timeline.fps);\n" +
  "        if (!cap) return;\n" +
  "        var style = rc.cat.textStyles.subtitle || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;\n" +
  "        var x = rc.W / 2, bottom = rc.H * (rc.aspect === '9:16' ? 0.82 : 0.92);\n" +
  "        ctx.setTransform(1, 0, 0, 1, 0, 0);\n" +
  "        var lay = SM.text.layout(ctx, cap.lines.join('\\n'), style, {\n" +
  "            canvasH: rc.H, short: Math.min(rc.W, rc.H), maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, x, style.align || 'center'), family: familyOf(rc, style)\n" +
  "        });\n" +
  "        SM.text.draw(ctx, lay, style, x, bottom - lay.height / 2 - lay.pad, SM.parts.makeResolver([rc.palette]), 1);\n" +
  "        ctx.globalAlpha = 1;\n" +
  "    }\n",
  "    function drawCaptions(ctx, timeline, n, rc) {\n" +
  "        if (!timeline.style.captions || !timeline.captions || !timeline.captions.length) return;\n" +
  "        var cap = SM.captions.at(timeline.captions, n * 1000 / timeline.fps);\n" +
  "        if (!cap) return;\n" +
  "        ctx.setTransform(1, 0, 0, 1, 0, 0);\n" +
  "        var c = captionLayout(ctx, cap, rc);\n" +
  "        SM.text.draw(ctx, c.lay, c.style, c.x, c.y, SM.parts.makeResolver([rc.palette]), 1);\n" +
  "        ctx.globalAlpha = 1;\n" +
  "    }\n" +
  "\n" +
  "    // captionLayout(ctx, cap, rc): the layout, the style and the anchor (x, y of the block centre) of a burned caption. The box\n" +
  "    // bottom sits at 92 % of the height, 82 % in 9:16. drawCaptions draws it and the layout run measures it.\n" +
  "    function captionLayout(ctx, cap, rc) {\n" +
  "        var style = rc.cat.textStyles.subtitle || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;\n" +
  "        var x = rc.W / 2, bottom = rc.H * (rc.aspect === '9:16' ? 0.82 : 0.92);\n" +
  "        var lay = SM.text.layout(ctx, cap.lines.join('\\n'), style, {\n" +
  "            canvasH: rc.H, short: Math.min(rc.W, rc.H), maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, x, style.align || 'center'), family: familyOf(rc, style)\n" +
  "        });\n" +
  "        return { style: style, lay: lay, x: x, y: bottom - lay.height / 2 - lay.pad };\n" +
  "    }\n" +
  "\n" +
  "    // captionBox(ctx, cap, rc): the box of a burned caption in canvas pixels, padding included.\n" +
  "    function captionBox(ctx, cap, rc) {\n" +
  "        var c = captionLayout(ctx, cap, rc);\n" +
  "        return { x: c.x - c.lay.width / 2 - c.lay.pad, y: c.y - c.lay.height / 2 - c.lay.pad, w: c.lay.width + c.lay.pad * 2, h: c.lay.height + c.lay.pad * 2 };\n" +
  "    }\n");

edit(F,
  "drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor, pulseOf: pulseOf\n",
  "drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor, pulseOf: pulseOf,\n" +
  "        entityBox: entityBox, textBox: textBox, captionBox: captionBox\n");
console.log('render.js edited');
```
In the file the long literal `'\n'` of `cap.lines.join('\n')` is a backslash and an n: the script writes it as `'\\n'` so that the edit text holds the two characters. If the first `edit` reports "old text not found", read `drawCaptions` in your copy (it was at lines 313 to 327 on 2026-11-09) and match its text.
- [ ] **Step 3: `beats.js` marks the hero in every story.** The hero prop is the first cue-bound show of a scene. B5 marked it only for v2; the layout run needs it for every story, and the scale still changes only for v2. Write `<scratch>/edit-D1-beats.mjs`:

```js
import { edit } from './crlf-edit.mjs';
const F = 'ui/js/storymaker/beats.js';
edit(F,
  "            var isHero = composeV2 && !hero && !!b.cue, scale = b.scale;\n" +
  "            if (isHero && !(scale > 0)) scale = SM.compose.heroScale(entry, stage.depthScale(at.y, at.depth));\n",
  "            var isHero = !hero && !!b.cue, scale = b.scale;\n" +
  "            if (isHero && composeV2 && !(scale > 0)) scale = SM.compose.heroScale(entry, stage.depthScale(at.y, at.depth));\n");
edit(F,
  "            if (isHero) hero = prop;\n",
  "            if (isHero) { hero = prop; prop.hero = true; }\n");
console.log('beats.js edited');
```
The push-in of B5 uses `hero` only inside its `composeV2` block, so v1 scenes still get no camera move.
- [ ] **Step 4: Create `ui/js/storymaker/layout.js`:**

```js
// Storymaker player: the layout run. measure(timeline, assets) samples every storyboard scene once, at its hold frame (after
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

    function round4(v) { return Math.round(v * 10000) / 10000; }

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
    function frameBox(box, win, stage, W, H) {
        var a = stage.toCanvas(box.x, box.y, win), b = stage.toCanvas(box.x + box.w, box.y + box.h, win);
        return [round4(a.x / W), round4(a.y / H), round4((b.x - a.x) / W), round4((b.y - a.y) / H)];
    }

    // pixelBox(box, W, H): a canvas-pixel box as fractions of the canvas.
    function pixelBox(box, W, H) { return [round4(box.x / W), round4(box.y / H), round4(box.w / W), round4(box.h / H)]; }

    // snapshot(sc, t): what a viewer could see change, without the idle cycle and the mouth.
    function snapshot(sc, t) {
        var out = [];
        for (var i = 0; i < sc.entities.length; i++) {
            var e = sc.entities[i];
            if (e.type !== 'actor' && e.type !== 'prop' && e.type !== 'text' && e.type !== 'shape') continue;
            var s = SM.scene.evalEntity(e, t);
            out.push([
                e.id, Math.round(s.x / EPS), Math.round(s.y / EPS), Math.round(s.alpha * 50), Math.round(s.scale * 50), Math.round(s.rot * 50),
                typeof s.pose === 'string' ? s.pose : (s.pose && s.pose.b) || '', s.expression || '', s.gesture && s.gesture.name || ''
            ]);
        }
        return JSON.stringify(out);
    }

    function measureScene(timeline, index, rc, ctx) {
        var sc = timeline.scenes[index], W = timeline.width, H = timeline.height, fps = timeline.fps;
        var t = holdSeconds(timeline, index), stage = SM.render.stageFor(sc, rc);
        var win = stage.cameraWindow(SM.camera.at(sc.camera, t)), base = stage.baseWindow();
        var block = { id: sc.id, index: sc.index, hold_s: round4(t), actors: [], props: [], texts: [], caption: null, missing: 0, static: false };
        for (var i = 0; i < sc.entities.length; i++) {
            var e = sc.entities[i], st = SM.scene.evalEntity(e, t);
            if (!(st.alpha > VISIBLE)) continue;
            if (e.type === 'actor' || e.type === 'prop') {
                var box = SM.render.entityBox(st, rc, stage);
                if (!box) { block.missing++; continue; }
                var framed = frameBox(box, win, stage, W, H);
                if (e.type === 'actor') block.actors.push({ id: e.id, box: framed });
                else block.props.push({ id: e.id, item: e.item, hero: !!e.hero, back: e.layer === 'back', scale: round4(e.baseScale || 1), box: framed });
            } else if (e.type === 'text' && st.text && ctx) {
                var tb = SM.render.textBox(ctx, st, rc, stage, base);
                if (tb) block.texts.push({ id: e.id, box: pixelBox(tb, W, H) });
            }
        }
        if (timeline.style.captions && timeline.captions && timeline.captions.length && ctx) {
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
    }

    SM.layout = { measure: measure, holdSeconds: holdSeconds, HOLD_AFTER: HOLD_AFTER };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```
- [ ] **Step 5: Register the part, the message and the boot flag.** Write `<scratch>/edit-D1-wiring.mjs`:

```js
import { edit } from './crlf-edit.mjs';

// the part, after render.js
edit('scripts/storymaker-bundle-parts.js',
  "  'ui/js/storymaker/render.js',",
  "  'ui/js/storymaker/render.js',\n  'ui/js/storymaker/layout.js',");

// the worker message
edit('ui/js/storymaker/worker-player.js',
  "            case 'dispose':\n",
  "            case 'layout': {\n" +
  "                if (!this.timeline) throw fail('code_error', 'layout before compile');\n" +
  "                var ltl = this.timeline, lself = this;\n" +
  "                var lassets = {\n" +
  "                    raster: null, bitmaps: this.bitmaps, rig: this.files[ltl.rigFile] || null, props: this.files, envs: this.files,\n" +
  "                    codeFrames: {}, codeErrors: {}, canvas: function (name, w, h) { return lself.canvas(name, w, h); }\n" +
  "                };\n" +
  "                return { payload: { layout: SM.layout.measure(ltl, lassets) } };\n" +
  "            }\n" +
  "            case 'dispose':\n");

// the deadline
edit('ui/js/storymaker/host-core.js',
  "helloDeadlineMs: 5000, assetsDeadlineMs: 20000",
  "helloDeadlineMs: 5000, assetsDeadlineMs: 20000, layoutDeadlineMs: 15000");

// the render page asks for it when boot.layout is true (only the compile check sets it), before it posts the report
edit('ui/js/storymaker/host-render.js',
  "            await loaded.prepare();\n            var rep = loaded.report;\n",
  "            await loaded.prepare();\n            var rep = loaded.report;\n" +
  "            if (boot.layout === true) {\n" +
  "                // The compile check wants the geometry of every scene. A failure costs the geometry, never the report.\n" +
  "                try {\n" +
  "                    var measured = await engine.send({ type: 'layout' }, env.limits.layoutDeadlineMs > 0 ? env.limits.layoutDeadlineMs : 15000);\n" +
  "                    rep.layout = measured.layout || [];\n" +
  "                } catch (layoutErr) {\n" +
  "                    rep.layout_error = String(layoutErr && layoutErr.message || layoutErr).slice(0, 200);\n" +
  "                }\n" +
  "            }\n");
console.log('wiring edited');
```
`render.js` is followed by `raster-rig.js` in the part list; `layout.js` uses `SM.render` at call time only, so the position after `render.js` is enough. The `'ui/js/storymaker/render.js',` entry is unique in the player list (the host list has no `render.js`).
- [ ] **Step 6: The test.** Create `scripts/storymaker-tests/30-layout.mjs`:

```js
// The layout run: boxes of actors, props, texts and the burned caption at the hold frame of each storyboard scene.
import assert from 'node:assert/strict';
import { renderAssets } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();

function film(style, scene) {
    const s = {
        schema: 'storymaker/1', title: 'Layout', format: { aspect: '16:9', fps: 30 },
        style: Object.assign({ palette: 'sunny', font: 'default', captions: true }, style),
        cast: [{ id: 'anna', name: 'Anna', character: 'char.office-woman-1' }],
        scenes: [Object.assign({
            id: 's1', environment: 'env.plain-stage', narration: 'Eine {1}Firewall prueft jedes Paket am Eingang.', speaker: 'anna',
            actors: [{ cast: 'anna', at: 'left', pose: 'stand' }],
            beats: [{ do: 'show', item: 'prop.lock', at: 'center', entry: 'pop', cue: 1 }, { do: 'show', item: 'prop.shield', at: 'right', entry: 'pop', cue: 1 }]
        }, scene)]
    };
    return s;
}
const measure = story => SM.layout.measure(SM.compile(story, {}, t.catalog).timeline, renderAssets());

check('layout: one block per storyboard scene with the boxes of what is on the stage at the hold frame', () => {
    const blocks = measure(film());
    assert.equal(blocks.length, 1);
    const b = blocks[0];
    assert.equal(b.id, 's1');
    assert.ok(b.hold_s > 0.5, 'the hold frame lies after the cue and the pop');
    assert.equal(b.actors.length, 1);
    assert.equal(b.props.length, 2);
    assert.equal(b.missing, 0);
    for (const box of [b.actors[0].box, b.props[0].box, b.props[1].box]) assert.ok(box[2] > 0 && box[3] > 0 && box[0] >= 0 && box[1] >= 0, 'a box inside the frame: ' + box);
    const lock = b.props.find(p => p.item === 'prop.lock'), shield = b.props.find(p => p.item === 'prop.shield');
    assert.equal(lock.hero, true, 'the first cue-bound show is the hero, also without compose');
    assert.equal(shield.hero, false);
    assert.equal(lock.scale, 1, 'v1 does not enlarge it');
    assert.ok(b.caption && b.caption[2] > 0, 'the burned caption is on screen at the hold frame');
    assert.equal(b.static, false, 'a prop appears');
});

check('layout: the feet of the v1 floor stand in the caption band, the feet of composition v2 stand above it', () => {
    const v1 = measure(film())[0], v2 = measure(film({ compose: 'v2' }))[0];
    const foot = a => a.box[1] + a.box[3];
    near(foot(v1.actors[0]), 0.88, 0.01, 'v1 feet line');
    near(foot(v2.actors[0]), 0.74, 0.02, 'v2 feet line (the push-in moves it a little)');
    assert.ok(foot(v1.actors[0]) > v1.caption[1], 'v1: the feet are below the top of the caption');
    assert.ok(foot(v2.actors[0]) < v2.caption[1], 'v2: the feet are above the top of the caption');
    const hero = v2.props.find(p => p.hero);
    near(hero.box[3], 0.24, 0.02, 'the v2 hero prop stands about a quarter of the frame high');
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
    assert.deepEqual(blocks.map(b => b.id), ['s1']);
});

check('layout: the compile report is unchanged by the layout run (the golden stays valid)', () => {
    const out = SM.compile(film(), {}, t.catalog);
    assert.equal(out.report.layout, undefined);
    assert.equal(out.timeline.scenes[0].entities.some(e => e.type === 'prop' && e.hero), true);
});
```
- [ ] **Step 7: Build and run.** `npm run build:ui`, then `node scripts/test-storymaker-player.mjs`. Expected: all checks `ok`. If `30-layout` fails on the feet values, print `b.actors[0].box` and compare with `0.88` and `0.74`: the mini rig's `height` and `units_height` decide the box height, not the feet line, so the feet (`y + h`) must still be at the spot's line. If an older check fails because `isHero` now marks `prop.hero` for v1, nothing else reads that field; look at the failing assertion and the test's expectation of the entity shape.
- [ ] **Step 8: Commit:** `git add ui/js/storymaker/render.js ui/js/storymaker/beats.js ui/js/storymaker/layout.js ui/js/storymaker/worker-player.js ui/js/storymaker/host-core.js ui/js/storymaker/host-render.js ui/js/storymaker/player.bundle.js ui/js/storymaker/host.bundle.js scripts/storymaker-bundle-parts.js scripts/storymaker-tests/30-layout.mjs`; message `feat(storymaker): layout run in the player (boxes of actors, props, texts and the caption)`.

<!-- group D2 -->

### Task D2: The report carries the layout
**Batch:** D  **Depends on:** D1
**Files:**
Create `internal/storymaker/layout.go`
Modify `internal/storymaker/studio.go`
Modify `internal/storymaker/render/listener.go`
Modify `internal/storymaker/render/compilecheck.go`
Test `internal/storymaker/render/compilecheck_layout_test.go`

- [ ] **Step 1: Impact:** `CompileSummary`, `summarizeCompileReport`, `PageParams`, `bootDoc`, `compileCheckReport`. `PageParams` and `bootDoc` are shared with the export: the new field is `omitempty`/false there.
- [ ] **Step 2: Create `internal/storymaker/layout.go`** (the shapes only; the findings come in D3):

```go
package storymaker

// LayoutBox is a rectangle in fractions of the output frame: x, y, width, height. The player measures it at the hold frame
// of a scene, under the scene's camera.
type LayoutBox [4]float64

// LayoutActor is the box of one actor on stage.
type LayoutActor struct {
	ID  string    `json:"id"`
	Box LayoutBox `json:"box"`
}

// LayoutProp is the box of one shown prop. Hero is the first cue-bound prop of the scene, Back a prop behind the actors,
// Scale the scale it carries (1 when none).
type LayoutProp struct {
	ID    string    `json:"id"`
	Item  string    `json:"item"`
	Hero  bool      `json:"hero"`
	Back  bool      `json:"back"`
	Scale float64   `json:"scale"`
	Box   LayoutBox `json:"box"`
}

// LayoutText is the box of one shown text card.
type LayoutText struct {
	ID  string    `json:"id"`
	Box LayoutBox `json:"box"`
}

// LayoutScene is the measured layout of one storyboard scene at its hold frame. Caption is the burned caption of that
// moment, nil when none is on screen. Missing counts boxes the player could not measure for lack of art. Static says that
// nothing on the stage changed between the first frame and the hold frame, apart from the idle cycle.
type LayoutScene struct {
	ID      string        `json:"id"`
	Index   int           `json:"index"`
	HoldS   float64       `json:"hold_s"`
	Actors  []LayoutActor `json:"actors"`
	Props   []LayoutProp  `json:"props"`
	Texts   []LayoutText  `json:"texts"`
	Caption *LayoutBox    `json:"caption"`
	Missing int           `json:"missing"`
	Static  bool          `json:"static"`
}
```
- [ ] **Step 3: The summary.**

**Edit** `internal/storymaker/studio.go`: the new fields of `CompileSummary`.
Old:
```go
	Scenes     []CompileScene
	Issues     []Issue
}
```
New:
```go
	Scenes     []CompileScene
	Issues     []Issue
	// Layout is the measured layout of the storyboard scenes (empty when the player did not measure). StoryboardScenes and
	// CodeScenes count the scenes of each kind; LayoutError is the player's reason when the measurement failed.
	Layout           []LayoutScene
	StoryboardScenes int
	CodeScenes       int
	LayoutError      string
}
```
(The first line of Old must be the end of the `CompileSummary` struct, `Scenes     []CompileScene` with the aligned spacing of that struct; `CompileScene` below has other fields.)
- [ ] **Step 4: The page asks for the layout.**

**Edit** `internal/storymaker/render/listener.go`: `PageParams`.
Old:
```go
	// Token authorises the page's POSTs (32 hex characters).
```
New:
```go
	// Layout asks the page to measure the layout of every storyboard scene and to add it to the report (only the compile check).
	Layout bool
	// Token authorises the page's POSTs (32 hex characters).
```
**Edit** `bootDoc`:
Old:
```go
json:"pre_roll,omitempty"`
```
New:
```go
json:"pre_roll,omitempty"`
	Layout          bool              `json:"layout,omitempty"`
```
**Edit** `boot()`:
Old:
```go
	doc.Workers, doc.Chunk, doc.PreRoll = l.params.Workers, l.params.Chunk, l.params.PreRoll
```
New:
```go
	doc.Workers, doc.Chunk, doc.PreRoll = l.params.Workers, l.params.Chunk, l.params.PreRoll
	doc.Layout = l.params.Layout
```
- [ ] **Step 5: The compile check reads it.**

**Edit** `internal/storymaker/render/compilecheck.go`: the page parameters.
Old:
```go
FrameDeadlineMS: frameDeadlineMS, Token: token, AudioInfo: plan.AudioJSON},
```
New:
```go
FrameDeadlineMS: frameDeadlineMS, Layout: true, Token: token, AudioInfo: plan.AudioJSON},
```
**Edit** the report structs:
Old:
```go
type compileCheckReport struct {
	FrameCount int                 `json:"frame_count"`
	Scenes     []compileCheckScene `json:"scenes"`
	Issues     []storymaker.Issue  `json:"issues"`
}

type compileCheckScene struct {
	ID         string            `json:"id"`
```
New:
```go
type compileCheckReport struct {
	FrameCount  int                       `json:"frame_count"`
	Scenes      []compileCheckScene       `json:"scenes"`
	Issues      []storymaker.Issue        `json:"issues"`
	Layout      []storymaker.LayoutScene  `json:"layout"`
	LayoutError string                    `json:"layout_error"`
}

type compileCheckScene struct {
	Kind       string            `json:"kind"`
	ID         string            `json:"id"`
```
**Edit** `summarizeCompileReport`: count the kinds and keep the layout of the scenes the story has.
Old:
```go
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	for _, sc := range rep.Scenes {
```
New:
```go
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	storyboard, code := 0, 0
	known := map[string]bool{}
	for _, sc := range rep.Scenes {
		known[sc.ID] = true
		if sc.Kind == "code" {
			code++
		} else {
			storyboard++
		}
	}
	var layout []storymaker.LayoutScene
	for _, block := range rep.Layout {
		// The page is trusted for geometry only: a block for a scene the story does not have, or a second one, is dropped.
		if known[block.ID] && !slices.ContainsFunc(layout, func(l storymaker.LayoutScene) bool { return l.ID == block.ID }) {
			layout = append(layout, block)
		}
	}
	for _, sc := range rep.Scenes {
```
**Edit** the return of `summarizeCompileReport`:
Old:
```go
	return storymaker.CompileSummary{Available: true, DurationMS: compileFrameMS(rep.FrameCount, want.FPS), Scenes: scenes, Issues: issues}
```
New:
```go
	return storymaker.CompileSummary{
		Available: true, DurationMS: compileFrameMS(rep.FrameCount, want.FPS), Scenes: scenes, Issues: issues,
		Layout: layout, StoryboardScenes: storyboard, CodeScenes: code, LayoutError: clip(rep.LayoutError, 200),
	}
```
Add `"slices"` to the imports of `compilecheck.go`. `clip` is the package's own (`render/jobs.go:906`).
**Edit** the call site, to log a failed measurement:
Old:
```go
			return summarizeCompileReport(body, want, frameLimit), nil
```
New:
```go
			summary := summarizeCompileReport(body, want, frameLimit)
			if summary.LayoutError != "" {
				r.deps.Logger.Warn("[Storymaker] The layout measurement of the compile check failed", "issue", summary.LayoutError)
			}
			return summary, nil
```
Run `gofmt -w` on `compilecheck.go` (the struct alignment above is approximate).
- [ ] **Step 6: Test.** Create `internal/storymaker/render/compilecheck_layout_test.go`:

```go
package render

import (
	"encoding/json"
	"testing"

	"aurago/internal/storymaker"
)

// goodReport() has s1 and s2 as storyboard scenes and s3 as a code scene.
func reportWithLayout(t *testing.T, extra map[string]any) []byte {
	t.Helper()
	m := goodReport()
	m["layout"] = []any{
		map[string]any{"id": "s1", "index": 0, "hold_s": 1.2, "actors": []any{map[string]any{"id": "anna", "box": []float64{0.2, 0.3, 0.1, 0.45}}},
			"props": []any{}, "texts": []any{}, "caption": []float64{0.2, 0.79, 0.6, 0.13}, "missing": 0, "static": false},
		map[string]any{"id": "s2", "index": 1, "hold_s": 2, "actors": []any{}, "props": []any{}, "texts": []any{}, "caption": nil, "missing": 0, "static": true},
		map[string]any{"id": "ghost", "index": 9, "actors": []any{}, "props": []any{}, "texts": []any{}},
		map[string]any{"id": "s1", "index": 0, "actors": []any{}, "props": []any{}, "texts": []any{}},
	}
	for k, v := range extra {
		m[k] = v
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestSummarizeCompileReportKeepsTheLayoutOfTheStoryScenes(t *testing.T) {
	got := summarizeCompileReport(reportWithLayout(t, nil), reportExpectation(), 0)
	if !got.Available {
		t.Fatalf("summary = %+v", got)
	}
	if got.StoryboardScenes != 2 || got.CodeScenes != 1 {
		t.Fatalf("scene kinds: storyboard %d code %d", got.StoryboardScenes, got.CodeScenes)
	}
	if len(got.Layout) != 2 || got.Layout[0].ID != "s1" || got.Layout[1].ID != "s2" {
		t.Fatalf("a block for an unknown scene and a repeated block are dropped: %+v", got.Layout)
	}
	if got.Layout[0].Caption == nil || got.Layout[0].Caption[1] != 0.79 || got.Layout[1].Caption != nil || !got.Layout[1].Static {
		t.Fatalf("layout = %+v", got.Layout)
	}
}

func TestSummarizeCompileReportWithoutLayoutOrWithAnError(t *testing.T) {
	plain, _ := json.Marshal(goodReport())
	got := summarizeCompileReport(plain, reportExpectation(), 0)
	if !got.Available || len(got.Layout) != 0 || got.StoryboardScenes != 2 || got.LayoutError != "" {
		t.Fatalf("a report without layout: %+v", got)
	}
	failed := summarizeCompileReport(reportWithLayout(t, map[string]any{"layout": nil, "layout_error": "The layout run took too long."}), reportExpectation(), 0)
	if failed.LayoutError != "The layout run took too long." || len(failed.Layout) != 0 {
		t.Fatalf("layout error: %+v", failed)
	}
}

func TestBootDocAsksForLayoutOnlyWhenTheParamsDo(t *testing.T) {
	var _ = storymaker.LayoutScene{}
	on, _ := json.Marshal(bootDoc{Layout: true})
	off, _ := json.Marshal(bootDoc{})
	if string(on) == string(off) {
		t.Fatal("layout must change the boot document")
	}
	var m map[string]any
	_ = json.Unmarshal(off, &m)
	if _, present := m["layout"]; present {
		t.Fatalf("an export's boot.json must not mention layout: %s", off)
	}
}
```
Delete the stray `var _ = storymaker.LayoutScene{}` line and the unused `storymaker` import if the compiler flags them; the other tests of this file do not need that import.
- [ ] **Step 7: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render -run '(TestSummarizeCompileReport|TestBootDoc|TestParseReport|TestCompileCheckStopsAfterReport|TestListener)' -count=1`. Expected: PASS (`TestCompileCheckStopsAfterReport` has a fake page whose report has no layout; its issue count must stay 2).
- [ ] **Step 8: Commit:** `gofmt -w` the touched Go files; `git add internal/storymaker/layout.go internal/storymaker/studio.go internal/storymaker/render/listener.go internal/storymaker/render/compilecheck.go internal/storymaker/render/compilecheck_layout_test.go`; message `feat(storymaker): the compile check carries the measured layout`.

<!-- group D3 -->

### Task D3: Findings from the boxes
**Batch:** D  **Depends on:** D2
**Files:**
Modify `internal/storymaker/model.go`
Modify `internal/storymaker/issues.go`
Modify `internal/storymaker/layout.go`
Test `internal/storymaker/layout_test.go`

The thresholds are constants of one file, so tuning is one small commit. They were chosen from the geometry measured on 2026-11-09 and are checked on the real run in task E6. A finding is a warning, model-fixable, and never blocks (task D4). The caption rule applies to composition v2 only: in a v1 film every base spot stands inside the caption band, so the rule could not be met.

- [ ] **Step 1: Codes.** **Edit** `internal/storymaker/model.go`:
Old:
```go
// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```
New:
```go
// Issue codes of the layout run (LayoutIssues). Only the agent job's check emits them. They are model-fixable like the craft
// codes and never block an outcome.
const (
	CodeCaptionOverlap  = "caption_overlap"
	CodePropTooSmall    = "prop_too_small"
	CodeOffscreen       = "offscreen"
	CodeActorPropOverlap = "actor_prop_overlap"
	CodeSceneStatic     = "scene_static"
)

// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```
- [ ] **Step 1b: The layout class joins the model-fixable and never-blocking sets.** `internal/storymaker/issues.go`. `IssueModelFixable` starts with `if IssueCraft(i.Code) {` and `issueNeverBlocks` returns `IssueCraft(code)` (task A2):
Old:
```go
	if IssueCraft(i.Code) {
		return true
	}
```
New:
```go
	if IssueCraft(i.Code) || IssueLayout(i.Code) {
		return true
	}
```
Old:
```go
func issueNeverBlocks(code string) bool {
	return IssueCraft(code)
}
```
New:
```go
func issueNeverBlocks(code string) bool {
	return IssueCraft(code) || IssueLayout(code)
}
```

- [ ] **Step 2: Write the failing test.** Create `internal/storymaker/layout_test.go`:

```go
package storymaker

import (
	"slices"
	"testing"
)

func layoutScene(id string, index int) LayoutScene {
	return LayoutScene{ID: id, Index: index, HoldS: 1, Actors: []LayoutActor{}, Props: []LayoutProp{}, Texts: []LayoutText{}}
}

func layoutCodes(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Code)
	}
	return out
}

func TestLayoutIssuesCaptionOverlap(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	sc := layoutScene("s1", 0)
	sc.Caption = &caption
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.25, 0.40, 0.12, 0.48}}} // feet at 0.88: inside the band
	sc.Props = []LayoutProp{{ID: "lock", Item: "prop.lock", Box: LayoutBox{0.45, 0.70, 0.12, 0.18}}}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: true})
	if !slices.Equal(layoutCodes(got), []string{CodeCaptionOverlap, CodeCaptionOverlap}) {
		t.Fatalf("issues = %v", layoutCodes(got))
	}
	for _, is := range got {
		if is.Severity != SeverityWarning || is.SceneID != "s1" || is.Path != "/scenes/0" || is.Fix == "" || !IssueLayout(is.Code) || !IssueModelFixable(is) {
			t.Errorf("a layout finding is a warning on the scene with a fix: %+v", is)
		}
	}

	// Feet above the band, or no caption on screen, or a v1 film (rule off): nothing.
	sc.Actors[0].Box = LayoutBox{0.25, 0.28, 0.12, 0.48} // feet at 0.76
	sc.Props[0].Box = LayoutBox{0.45, 0.58, 0.12, 0.18}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: true}); len(got) != 0 {
		t.Fatalf("feet above the band: %v", layoutCodes(got))
	}
	sc.Actors[0].Box = LayoutBox{0.25, 0.40, 0.12, 0.48}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: false}); slices.Contains(layoutCodes(got), CodeCaptionOverlap) {
		t.Fatalf("a v1 film is not held to the caption rule: %v", layoutCodes(got))
	}
	sc.Caption = nil
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: true}); len(got) != 0 {
		t.Fatalf("no caption on screen: %v", layoutCodes(got))
	}
}

func TestLayoutIssuesHeroPropTooSmall(t *testing.T) {
	sc := layoutScene("s2", 1)
	sc.Props = []LayoutProp{
		{ID: "small", Item: "prop.key", Hero: true, Scale: 1, Box: LayoutBox{0.5, 0.6, 0.05, 0.08}},
		{ID: "other", Item: "prop.lock", Hero: false, Scale: 1, Box: LayoutBox{0.2, 0.6, 0.05, 0.06}},
	}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	if !slices.Equal(layoutCodes(got), []string{CodePropTooSmall}) {
		t.Fatalf("only the hero counts: %v", layoutCodes(got))
	}
	if got[0].SceneID != "s2" || got[0].Fix == "" {
		t.Fatalf("issue = %+v", got[0])
	}
	sc.Props[0].Box = LayoutBox{0.5, 0.5, 0.1, 0.13}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); len(got) != 0 {
		t.Fatalf("13 percent of the height is big enough: %v", layoutCodes(got))
	}
}

func TestLayoutIssuesOffscreenAndOverlap(t *testing.T) {
	sc := layoutScene("s3", 2)
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.30, 0.30, 0.12, 0.45}}}
	sc.Props = []LayoutProp{
		{ID: "gone", Item: "prop.lock", Box: LayoutBox{0.9, 0.5, 0.2, 0.2}},          // half outside to the right
		{ID: "cover", Item: "prop.shield", Box: LayoutBox{0.32, 0.4, 0.10, 0.10}},    // inside the actor box
		{ID: "behind", Item: "prop.key", Back: true, Box: LayoutBox{0.32, 0.4, 0.1, 0.1}}, // a back prop may overlap
	}
	sc.Texts = []LayoutText{{ID: "t", Box: LayoutBox{0.7, 0.2, 0.35, 0.1}}} // sticks out of the frame
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	want := []string{CodeOffscreen, CodeOffscreen, CodeActorPropOverlap}
	slices.Sort(want)
	have := layoutCodes(got)
	slices.Sort(have)
	if !slices.Equal(have, want) {
		t.Fatalf("issues = %v, want %v", have, want)
	}
}

func TestLayoutIssuesStaticSceneAndMissingArt(t *testing.T) {
	sc := layoutScene("s4", 3)
	sc.Static = true
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.3, 0.3, 0.1, 0.4}}}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); !slices.Equal(layoutCodes(got), []string{CodeSceneStatic}) {
		t.Fatalf("issues = %v", layoutCodes(got))
	}
	sc.Missing = 1
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); slices.Contains(layoutCodes(got), CodeSceneStatic) == false {
		t.Fatalf("a missing box does not hide a static scene")
	}
	if LayoutIssues(nil, LayoutOptions{CaptionRule: true}) != nil {
		t.Fatal("no layout, no findings")
	}
}
```
- [ ] **Step 3: Run to see it fail:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestLayoutIssues' -count=1`. Expected: FAIL to build (`LayoutIssues`, `LayoutOptions`, `IssueLayout` are not defined).
- [ ] **Step 4: Implement.** Append to `internal/storymaker/layout.go`:

```go

// The thresholds of the layout findings, in fractions of the frame. They come from the geometry measured on 2026-11-09 and
// are checked on a real run (task E6): the two-line caption box covers 79.4 to 92 percent of the height in 16:9 and 1:1.
const (
	layoutFootStrip      = 0.12 // the lower part of an actor or prop box that counts as its foot or base
	layoutCaptionSlack   = 0.004
	layoutHeroMinHeight  = 0.12 // a hero prop below this share of the frame height is too small
	layoutHeroTarget     = 0.20 // the share the fix text asks for
	layoutOutsideShare   = 0.25 // actors and props: this share of the box outside the frame is too much
	layoutTextOutside    = 0.01 // texts must lie inside the frame
	layoutOverlapShare   = 0.35 // of the smaller box
	layoutScaleMax       = 2.0  // the validated range of a beat scale
)

// LayoutOptions say which rules apply. CaptionRule is true for a composition v2 film with captions on.
type LayoutOptions struct {
	CaptionRule bool
}

// IssueLayout says whether a code is a finding of the layout run. Like the craft codes they are model-fixable and never
// block an outcome.
func IssueLayout(code string) bool {
	switch code {
	case CodeCaptionOverlap, CodePropTooSmall, CodeOffscreen, CodeActorPropOverlap, CodeSceneStatic:
		return true
	}
	return false
}

// LayoutIssues turns the measured boxes into findings: an actor foot or a prop base inside the caption band, a hero prop that
// is too small, a box outside the frame, an actor that hides a prop or a text, and a scene in which nothing changes. They are
// warnings with a fix that names the field and the range.
func LayoutIssues(scenes []LayoutScene, opts LayoutOptions) []Issue {
	var out []Issue
	for i := range scenes {
		sc := &scenes[i]
		path := jp("scenes", sc.Index)
		add := func(code, message, fix string) {
			out = append(out, Issue{Severity: SeverityWarning, SceneID: sc.ID, Path: path, Code: code, Message: message, Fix: fix})
		}
		if opts.CaptionRule && sc.Caption != nil {
			for _, a := range sc.Actors {
				if footInCaption(a.Box, *sc.Caption) {
					add(CodeCaptionOverlap, fmt.Sprintf("The caption covers the feet of %s in scene %s.", a.ID, sc.ID),
						`Move the actor to a base spot ("left", "center" or "right") with patch_scene and the whole actors list, or shorten the narration so the caption has one line.`)
				}
			}
			for _, p := range sc.Props {
				if !p.Back && footInCaption(p.Box, *sc.Caption) {
					add(CodeCaptionOverlap, fmt.Sprintf("The caption covers the base of %s in scene %s.", p.ID, sc.ID),
						`Show the prop at a higher spot such as "board" or "center" with patch_scene and the whole beats list.`)
				}
			}
		}
		for _, p := range sc.Props {
			if p.Hero && p.Box[3] < layoutHeroMinHeight {
				scale := min(layoutScaleMax, round1(max(p.Scale, 0.5)*layoutHeroTarget/max(p.Box[3], 0.01)))
				add(CodePropTooSmall, fmt.Sprintf("The prop %s in scene %s is only %.0f percent of the frame height.", p.ID, sc.ID, p.Box[3]*100),
					fmt.Sprintf(`Make it bigger: send patch_scene with the whole beats list and "scale":%.1f on the show beat of %s (scale takes 0.5 to 2).`, scale, p.Item))
			}
		}
		for _, a := range sc.Actors {
			if outsideShare(a.Box) > layoutOutsideShare {
				add(CodeOffscreen, fmt.Sprintf("%s is partly outside the frame in scene %s.", a.ID, sc.ID), `Move the actor to a spot inside the frame such as "left", "center" or "right".`)
			}
		}
		for _, p := range sc.Props {
			if outsideShare(p.Box) > layoutOutsideShare {
				add(CodeOffscreen, fmt.Sprintf("The prop %s is partly outside the frame in scene %s.", p.ID, sc.ID), `Show it at a spot inside the frame such as "center" or "right".`)
			}
		}
		for _, tx := range sc.Texts {
			if outsideShare(tx.Box) > layoutTextOutside {
				add(CodeOffscreen, fmt.Sprintf("The text %s sticks out of the frame in scene %s.", tx.ID, sc.ID), `Shorten the text to five words or fewer, or show it at "center".`)
			}
		}
		for _, a := range sc.Actors {
			for _, p := range sc.Props {
				if !p.Back && overlapShare(a.Box, p.Box) > layoutOverlapShare {
					add(CodeActorPropOverlap, fmt.Sprintf("%s hides the prop %s in scene %s.", a.ID, p.ID, sc.ID), `Show the prop on a spot next to the actor, for example "center" when the actor stands at "left".`)
				}
			}
			for _, tx := range sc.Texts {
				if overlapShare(a.Box, tx.Box) > layoutOverlapShare {
					add(CodeActorPropOverlap, fmt.Sprintf("%s hides the text %s in scene %s.", a.ID, tx.ID, sc.ID), `Show the text at "center" or "board", away from the actor.`)
				}
			}
		}
		if sc.Static {
			add(CodeSceneStatic, fmt.Sprintf("Nothing shows, moves or changes in scene %s.", sc.ID),
				`Add a show beat with a prop and a gesture beat with "with":"prev" using patch_scene and the whole beats list.`)
		}
	}
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// footInCaption is true when the lower strip of a box meets the caption box. The top of the caption gets a small slack.
func footInCaption(box, caption LayoutBox) bool {
	strip := LayoutBox{box[0], box[1] + box[3]*(1-layoutFootStrip), box[2], box[3] * layoutFootStrip}
	top := LayoutBox{caption[0], caption[1] + layoutCaptionSlack, caption[2], caption[3] - layoutCaptionSlack}
	return intersection(strip, top) > 0
}

func area(b LayoutBox) float64 { return max(0, b[2]) * max(0, b[3]) }

// intersection is the area of two boxes' overlap.
func intersection(a, b LayoutBox) float64 {
	w := min(a[0]+a[2], b[0]+b[2]) - max(a[0], b[0])
	h := min(a[1]+a[3], b[1]+b[3]) - max(a[1], b[1])
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// outsideShare is the share of a box's area outside the frame [0,1] x [0,1].
func outsideShare(b LayoutBox) float64 {
	if area(b) == 0 {
		return 0
	}
	return 1 - intersection(b, LayoutBox{0, 0, 1, 1})/area(b)
}

// overlapShare is the overlap as a share of the smaller box.
func overlapShare(a, b LayoutBox) float64 {
	smaller := min(area(a), area(b))
	if smaller == 0 {
		return 0
	}
	return intersection(a, b) / smaller
}
```
and replace the import line of `layout.go`: the file starts with `package storymaker`; add `import (\n\t"fmt"\n\t"math"\n)` after it.
- [ ] **Step 5: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '^TestLayoutIssues' -count=1 -v`. Expected: PASS. `TestLayoutIssuesCaptionOverlap` uses caption box `{0.2, 0.794, 0.6, 0.126}` (79.4 to 92 percent) and an actor whose feet are at 0.88: the foot strip is `[0.88 - 0.0576, 0.88]` inside the caption, so two findings (actor and prop). 
- [ ] **Step 6: Commit:** `gofmt -w internal/storymaker/layout.go internal/storymaker/layout_test.go internal/storymaker/model.go internal/storymaker/issues.go`; `git add` the four files; message `feat(storymaker): findings from the measured layout`.

<!-- group D4 -->

### Task D4: Check and outcome `ready`
**Batch:** D  **Depends on:** D2, D3
**Files:**
Modify `internal/storymaker/studio_check.go`
Modify `internal/storymaker/agentjob.go`
Modify `internal/storymaker/agentjob_test.go`
Modify `internal/server/storymaker_agent.go`
Test `internal/storymaker/studio_layout_check_test.go`

`ready` means nothing is left open: compile `ok`, the layout measured for every storyboard scene, no code scene, narration done, and an empty open-check list. Everything else stays `ready_unverified`, as before, with `layout_unverified` listed only when the layout was not verified (a code scene, no compile, or no measurement).

- [ ] **Step 1: Impact:** `DecideOutcome`, `CheckReport`, `runCheck`, `publishOutcome`. Some of them are the symbols batch A and batch C touched; run the impact check again from the current main and report.
- [ ] **Step 2: Implement.**

**Edit** `internal/storymaker/studio_check.go`: the report counts, after `CompileReason` (or after `Compile` when batch C has not landed; the spaces align the field, run `gofmt -w`).
Old:
```go
Compile                     string
```
New:
```go
Compile                     string
	StoryboardScenes            int // storyboard scenes of the film, from the compile run
	CodeScenes                  int // code scenes of the film (their layout is never measured)
	LayoutScenes                int // storyboard scenes whose layout the player measured
```
**Edit** `runCheck`: the findings and the counts.
Old:
```go
	found = append(found, checkCompileIssues(summary.Issues)...)
```
New:
```go
	found = append(found, checkCompileIssues(summary.Issues)...)
	found = append(found, LayoutIssues(summary.Layout, LayoutOptions{CaptionRule: story.Style.Compose == "v2" && story.Style.Captions})...)
```
Old:
```go
		TargetSeconds: s.job.TargetSeconds, Compile: state,
```
New:
```go
		TargetSeconds: s.job.TargetSeconds, Compile: state, StoryboardScenes: summary.StoryboardScenes, CodeScenes: summary.CodeScenes, LayoutScenes: len(summary.Layout),
```
If batch C has landed the line reads `... Compile: state, CompileReason: compileReasonCode(summary, state),`: append the three fields to it. When the compile did not run (`summary.Available` is false) all three counts are 0, so the layout cannot count as verified.

**Edit** `internal/storymaker/agentjob.go`: `DecideOutcome`.
Old:
```go
	add(CodeLayoutUnverified, "")
```
New:
```go
	// The layout counts as verified only when the compile ran, the player measured every storyboard scene and no code scene
	// exists (the layout inside a code scene is never measured).
	layoutChecked := in.Check.Compile == "ok" && in.Check.StoryboardScenes > 0 && in.Check.CodeScenes == 0 && in.Check.LayoutScenes == in.Check.StoryboardScenes
	if !layoutChecked {
		add(CodeLayoutUnverified, "")
	}
```
Old:
```go
	return OutcomeReadyUnverified, checks
```
New:
```go
	// ready means nothing is left open: the film was checked, every item is closed, and the narration is done.
	if len(checks) == 0 {
		return OutcomeReady, checks
	}
	return OutcomeReadyUnverified, checks
```
The first `return OutcomeReadyUnverified, checks` in `DecideOutcome` is the last statement of the function; the rest of the file has no other. Update the doc comment above `DecideOutcome`: replace the sentence "Otherwise ready_unverified: layout is never verified, so this slice never answers ready." by "Otherwise ready when nothing is left open, else ready_unverified."

**Edit** `internal/server/storymaker_agent.go`: the summary of a ready job is kept too.
Old:
```go
			if outcome == storymaker.OutcomeReadyUnverified {
```
New:
```go
			if outcome == storymaker.OutcomeReadyUnverified || outcome == storymaker.OutcomeReady {
```
(The comment of `keepAnswer`, "It is released only with the outcome ready_unverified.", gets "ready or" before it.)
- [ ] **Step 3: Tests.** In `internal/storymaker/agentjob_test.go` add rows (same place as in A2):

```go
		{name: "ready when the layout was measured and nothing is open", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", StoryboardScenes: 3, LayoutScenes: 3}}, outcome: OutcomeReady, checks: nil},
		{name: "a code scene keeps layout_unverified", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", StoryboardScenes: 2, CodeScenes: 1, LayoutScenes: 2}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout}},
		{name: "an unmeasured scene keeps layout_unverified", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", StoryboardScenes: 3, LayoutScenes: 2}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout}},
		{name: "a layout finding that is left is listed and never blocks", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", StoryboardScenes: 2, LayoutScenes: 2, Issues: []Issue{{Severity: SeverityWarning, Code: CodeCaptionOverlap, SceneID: "s2"}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{{Code: CodeCaptionOverlap, SceneID: "s2"}}},
		{name: "an open narration check keeps the film from ready", in: OutcomeInput{Progress: done, Narration: "narration_pending", Check: CheckReport{Compile: "ok", StoryboardScenes: 2, LayoutScenes: 2}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{{Code: "narration_pending"}}},
```
The table compares `checks` with `reflect.DeepEqual`: for the first row, `DecideOutcome` returns a nil slice when nothing was added, which equals `nil`. If the table type compares an empty slice differently, use `[]AgentCheck(nil)`.

Create `internal/storymaker/studio_layout_check_test.go`:

```go
package storymaker

import (
	"context"
	"slices"
	"testing"
)

// A compile that measured the layout of the real run: the check turns the boxes into findings, they are model-fixable and
// the summary counts decide the outcome.
func TestCheckTurnsTheLayoutIntoFindings(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		sc := layoutScene("s1", 0)
		sc.Caption = &caption
		sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.25, 0.40, 0.12, 0.48}}}
		return CompileSummary{Available: true, DurationMS: 8000, StoryboardScenes: 2, Layout: []LayoutScene{sc, layoutScene("s2", 1)}}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, is := range report.Issues {
		codes = append(codes, is.Code)
	}
	if report.Compile != "ok" || report.StoryboardScenes != 2 || report.LayoutScenes != 2 {
		t.Fatalf("report counts: %+v", report)
	}
	// agent-varied is a v2-less fixture (no compose), so the caption rule is off; turn it on for the second half.
	if slices.Contains(codes, CodeCaptionOverlap) {
		t.Fatalf("a story without compose is not held to the caption rule: %v", codes)
	}
	var s Story
	raw, _, _ := ps.LoadStory(context.Background(), projectID)
	s = mustDecode(t, string(raw))
	s.Style.Compose = "v2"
	out, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(context.Background(), projectID, out, ""); err != nil {
		t.Fatal(err)
	}
	report, err = studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	codes = nil
	for _, is := range report.Issues {
		codes = append(codes, is.Code)
	}
	if !slices.Contains(codes, CodeCaptionOverlap) {
		t.Fatalf("a v2 story with the feet in the caption band must be flagged: %v", codes)
	}
	outcome, _ := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	if outcome == OutcomeIncomplete {
		t.Fatalf("a layout finding must not block: %s", outcome)
	}
}
```
The fixture `agent-varied` has seven scenes, so the rig reports `Layout` for only two of them: that is enough for this test; the counts come from the summary, not from the story.
- [ ] **Step 4: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker -run '(TestDecideOutcome|TestCheck|TestLayoutIssues|TestStudioCheck)' -count=1`, then `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run 'TestStorymakerAgentScripted' -count=1 -timeout 20m`. Expected: PASS. A scripted job still ends `ready_unverified`: its browser is a stub, so the compile is unavailable.
- [ ] **Step 5: Commit:** `gofmt -w` the touched Go files; `git add internal/storymaker/studio_check.go internal/storymaker/agentjob.go internal/storymaker/agentjob_test.go internal/storymaker/studio_layout_check_test.go internal/server/storymaker_agent.go`; message `feat(storymaker): layout findings never block, and a film with nothing open is ready`.

<!-- group D5 -->

### Task D5: Job card, documents and the browser acceptance
**Batch:** D  **Depends on:** D4
**Files:**
Modify `ui/js/desktop/apps/storymaker-agent.js`
Modify `ui/lang/storymaker/*.json` (16 files)
Modify `internal/storymaker/render/compilecheck_test.go`
Modify `internal/storymaker/AGENTS.md`
Modify `internal/storymaker/reference/animation.md`
Modify `prompts/tools_manuals/storymaker.md`

- [ ] **Step 1: The job card.** `OUTCOME_KEYS` gets an entry for `ready`; the five layout codes share one sentence; a `ready` job shows a text that says the layout was checked. Write `<scratch>/edit-D5-agent-js.mjs` and run it from the worktree root:

```js
import { edit } from './crlf-edit.mjs';
const F = 'ui/js/desktop/apps/storymaker-agent.js';

edit(F,
  "        ready_unverified: { title: 'storymaker.agent.outcome_ready_title', text: 'storymaker.agent.outcome_ready_text' },\n",
  "        ready: { title: 'storymaker.agent.outcome_ready_title', text: 'storymaker.agent.outcome_ready_checked_text' },\n" +
  "        ready_unverified: { title: 'storymaker.agent.outcome_ready_title', text: 'storymaker.agent.outcome_ready_text' },\n");

edit(F,
  "        over_budget: 'storymaker.agent.check_over_budget'",
  "        over_budget: 'storymaker.agent.check_over_budget',\n" +
  "        caption_overlap: 'storymaker.agent.check_layout_found',\n" +
  "        prop_too_small: 'storymaker.agent.check_layout_found',\n" +
  "        offscreen: 'storymaker.agent.check_layout_found',\n" +
  "        actor_prop_overlap: 'storymaker.agent.check_layout_found',\n" +
  "        scene_static: 'storymaker.agent.check_layout_found'");

edit(F,
  "            if (outcome !== 'incomplete') return t(OUTCOME_KEYS.ready_unverified.text);\n",
  "            if (outcome === 'ready') return t(OUTCOME_KEYS.ready.text);\n" +
  "            if (outcome !== 'incomplete') return t(OUTCOME_KEYS.ready_unverified.text);\n");
console.log('storymaker-agent.js edited');
```
If batch A or C have landed, `CHECK_KEYS` ends with their entries instead of `over_budget`: the Old line `over_budget: 'storymaker.agent.check_over_budget'` is still unique, and the new entries are inserted after it, before theirs. If the `ready_unverified` line of `OUTCOME_KEYS` differs in your copy, read lines 29 to 33 of the file and match them.
- [ ] **Step 2: Two sentences in 16 languages.** Write `<scratch>/add-D5-keys.mjs` and run it from the worktree root. It inserts two lines after the line with `"storymaker.agent.outcome_ready_text"` (the first) in each `ui/lang/storymaker/<lang>.json`:

```js
import fs from 'node:fs';
const AFTER = '"storymaker.agent.outcome_ready_text"';
const KEYS = ['storymaker.agent.outcome_ready_checked_text', 'storymaker.agent.check_layout_found'];
const T = {
  en: ['Scenes, assets, timing and the layout of every scene were checked.', 'A scene still has a layout problem: text over feet, a small prop, something off screen or overlapping. Watch it once and adjust the scene.'],
  de: ['Szenen, Assets, Timing und das Layout jeder Szene wurden geprüft.', 'Eine Szene hat noch ein Layoutproblem: Text über den Füßen, ein kleines Objekt, etwas außerhalb des Bildes oder Überlappungen. Sieh sie dir an und passe die Szene an.'],
  cs: ['Scény, podklady, časování a rozvržení každé scény byly zkontrolovány.', 'Některá scéna má ještě potíže s rozvržením: text přes nohy, malý předmět, něco mimo obraz nebo překryv. Prohlédněte si ji a upravte.'],
  da: ['Scener, filer, timing og layoutet i hver scene er kontrolleret.', 'En scene har stadig et layoutproblem: tekst over fødderne, en lille ting, noget uden for billedet eller overlap. Se den og tilpas scenen.'],
  el: ['Οι σκηνές, τα στοιχεία, ο χρονισμός και η διάταξη κάθε σκηνής ελέγχθηκαν.', 'Μια σκηνή έχει ακόμη πρόβλημα διάταξης: κείμενο πάνω από τα πόδια, μικρό αντικείμενο, κάτι εκτός κάδρου ή επικάλυψη. Δείτε την και προσαρμόστε τη σκηνή.'],
  es: ['Se revisaron las escenas, los recursos, el tiempo y la composición de cada escena.', 'Una escena aún tiene un problema de composición: texto sobre los pies, un objeto pequeño, algo fuera de plano o superpuesto. Míralo y ajusta la escena.'],
  fr: ['Les scènes, les ressources, le minutage et la mise en page de chaque scène ont été vérifiés.', 'Une scène a encore un problème de mise en page : texte sur les pieds, objet trop petit, élément hors champ ou qui se chevauche. Regardez-la et ajustez la scène.'],
  hi: ['दृश्यों, संसाधनों, समय और हर दृश्य के लेआउट की जाँच हो चुकी है।', 'किसी दृश्य में अब भी लेआउट की समस्या है: पैरों पर लिखा पाठ, छोटी वस्तु, फ़्रेम से बाहर या आपस में ढकी चीज़ें। इसे एक बार देखें और दृश्य ठीक करें।'],
  it: ['Scene, risorse, tempi e impaginazione di ogni scena sono stati controllati.', "Una scena ha ancora un problema di impaginazione: testo sui piedi, un oggetto piccolo, qualcosa fuori quadro o sovrapposto. Guardala e modifica la scena."],
  ja: ['シーン、アセット、タイミング、各シーンのレイアウトを確認しました。', 'レイアウトに問題が残るシーンがあります：足に重なる字幕、小さすぎる小道具、画面外や重なりなど。一度ご覧になり、シーンを調整してください。'],
  nl: ['Scènes, bestanden, timing en de opmaak van elke scène zijn gecontroleerd.', 'Een scène heeft nog een opmaakprobleem: tekst over de voeten, een klein voorwerp, iets buiten beeld of overlap. Bekijk haar en pas de scène aan.'],
  no: ['Scener, filer, timing og oppsettet i hver scene er kontrollert.', 'En scene har fortsatt et oppsettproblem: tekst over føttene, en liten ting, noe utenfor bildet eller overlapp. Se den og juster scenen.'],
  pl: ['Sprawdzono sceny, zasoby, czas i układ każdej sceny.', 'Któraś scena ma jeszcze problem z układem: napis na stopach, mały przedmiot, coś poza kadrem lub nakładanie się. Obejrzyj ją i popraw scenę.'],
  pt: ['As cenas, os recursos, o tempo e a composição de cada cena foram verificados.', 'Uma cena ainda tem um problema de composição: texto sobre os pés, um objeto pequeno, algo fora do quadro ou sobreposto. Veja-a e ajuste a cena.'],
  sv: ['Scener, resurser, timing och layouten i varje scen har kontrollerats.', 'En scen har fortfarande ett layoutproblem: text över fötterna, ett litet föremål, något utanför bild eller överlapp. Titta på den och justera scenen.'],
  zh: ['已检查各场景、素材、时间节奏和每个场景的版面。', '某个场景的版面仍有问题：文字盖住脚部、道具太小、内容超出画面或相互重叠。请观看一次并调整该场景。'],
};
for (const [lang, texts] of Object.entries(T)) {
  const file = `ui/lang/storymaker/${lang}.json`;
  const source = fs.readFileSync(file, 'utf8');
  if (source.includes(KEYS[0])) throw new Error(`${file} already has the keys`);
  const eol = source.includes('\r\n') ? '\r\n' : '\n';
  const lines = source.split(eol);
  const at = lines.findIndex((line) => line.includes(AFTER));
  if (at < 0) throw new Error(`${file} lacks ${AFTER}`);
  lines.splice(at + 1, 0, ...KEYS.map((key, i) => `  "${key}": ${JSON.stringify(texts[i])},`));
  fs.writeFileSync(file, lines.join(eol));
  JSON.parse(fs.readFileSync(file, 'utf8'));
}
console.log('added 2 keys to', Object.keys(T).length, 'files');
```
Expected output: `added 2 keys to 16 files`. The line after `outcome_ready_text` is `outcome_incomplete_title`, so the commas keep the JSON valid.
- [ ] **Step 3: Extend the browser test.** In `internal/storymaker/render/compilecheck_test.go`, in `TestCompileCheckBrowser`, after the check on `got.Scenes[1].EndMS` add:

```go
	// The page measured the layout of both storyboard scenes, in fractions of the frame.
	if got.LayoutError != "" || got.StoryboardScenes != 2 || got.CodeScenes != 0 || len(got.Layout) != 2 {
		t.Fatalf("layout: %+v (error %q)", got.Layout, got.LayoutError)
	}
	first := got.Layout[0]
	if first.ID != "s1" || len(first.Props) != 1 || !first.Props[0].Hero || first.Props[0].Box[3] <= 0 || first.Props[0].Box[3] > 1 {
		t.Errorf("layout of s1 = %+v", first)
	}
	if first.Caption == nil || first.Caption[1] < 0.5 {
		t.Errorf("the caption of s1 must be near the bottom: %+v", first.Caption)
	}
```
The document of that test has `s1` with a `prop.lock` show beat (cue 1) and no actors, and `s2` with narration only. If the caption is missing because the test's story has no captions, drop the last assertion.
- [ ] **Step 4: Documents.** In `internal/storymaker/AGENTS.md` replace the line `DecideOutcome is the one rule for outcomes. This slice never emits ready, and open_checks always holds layout_unverified. over_target and environment items never downgrade.` (line 109 on 2026-11-09) by: ``- `DecideOutcome` is the one rule for outcomes. `ready` means nothing is left open: the compile ran, the player measured the layout of every storyboard scene, no code scene exists, the narration is done and the open-check list is empty. `layout_unverified` is listed only when the layout was not verified (a code scene, no compile, no measurement). Craft and layout findings are model-fixable while rounds remain and never block (`issueNeverBlocks`). `over_target` and environment items never downgrade a film but keep it from `ready`.`` and add after the compile bullet: ``- The compile run measures the layout (`layout.js`, worker message `layout`, boot flag `layout`, only the compile check sets it) and `LayoutIssues` (`layout.go`) turns the boxes into `caption_overlap`, `prop_too_small`, `offscreen`, `actor_prop_overlap` and `scene_static`; the caption rule applies to `style.compose` `v2` only. The thresholds are constants in `layout.go`.`` In `internal/storymaker/reference/animation.md` replace the sentence `The agent cannot certify pixel layout; leave layout_unverified for human review.` by `The check measures where actors, props, texts and the caption stand in every storyboard scene and reports what covers what, what is too small and what lies outside the frame; fix what it reports. Code scenes are not measured.` In `prompts/tools_manuals/storymaker.md` (line 20 on 2026-11-09) replace the sentence about `ready_unverified` and `layout_unverified` by: ``- `ready` means scenes, assets, timing and the layout of every scene were checked and nothing is left open. `ready_unverified` means something is left open (see `open_checks`), for example a code scene, whose layout is never measured, or a finding the agent could not fix. Ask the user to watch the film once in that case.``
- [ ] **Step 5: Run:** `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated|TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerSourcesAreSafeAndSmall)' -count=1`, `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/prompts -count=1`, and `GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/... -count=1`. Expected: PASS. Then the browser test, one command, sandbox off (index, "Setup"): `AURAGO_RUN_BROWSER_SMOKE=1 GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render -run '^TestCompileCheckBrowser$' -count=1 -v -timeout 10m`. Expected: PASS. If the test is skipped it counts as not run.
- [ ] **Step 6: Commit:** `git add ui/js/desktop/apps/storymaker-agent.js ui/lang/storymaker internal/storymaker/render/compilecheck_test.go internal/storymaker/AGENTS.md internal/storymaker/reference/animation.md prompts/tools_manuals/storymaker.md` plus any rebuilt bundle; message `feat(storymaker): job card for a checked film and for layout findings, document the layout run`.

## Batch D end

Run the batch matrix of the index from the batch D worktree, with the browser line of the matrix (`TestCompileCheckBrowser`). The thresholds of `layout.go` are first checked against a real frame in task E6.
