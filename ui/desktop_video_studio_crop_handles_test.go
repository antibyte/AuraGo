package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// On-canvas crop handles (Task 16): the overlay geometry for contain and cover (the whole source at the clip's scale,
// mirrored by flips), drags and nudges mapped to unflipped crop fractions with the 0.95 limits and the 0.001 step, the
// clip rotation undone before the mapping, and the wiring the stage relies on.
func TestVideoStudioCropHandleMath(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("js", "desktop", "apps", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	app, stage, inspector := read("video-studio.js"), read("video-studio-stage.js"), read("video-studio-inspector.js")
	for _, want := range []string{"P.wireCropBox(s);", "action === 'crop-mode'", "cropMode: !!s.cropMode"} {
		if !strings.Contains(app, want) {
			t.Errorf("video-studio.js lacks %s", want)
		}
	}
	keys := strings.Index(app, "P.handleCropKeys(s, event)")
	rangeKeys := strings.Index(app, "P.handleRangeKeys(s, event)")
	textGuard := strings.Index(app, "if (!sourceSlider && target.closest('input,textarea,select,[contenteditable=\"true\"]')) return;")
	if keys < 0 || rangeKeys < 0 || textGuard < 0 || keys < rangeKeys || keys > textGuard {
		t.Error("handleKeys must try P.handleCropKeys after the range keys and before the text-field guard")
	}
	start := strings.Index(stage, "function updateTransform(s) {")
	end := strings.Index(stage, "function syncPreviewNotice(s) {")
	if start < 0 || end < start || !strings.Contains(stage[start:end], "updateCropBox(s);") {
		t.Error("updateTransform must keep the crop overlay in step")
	}
	if !strings.Contains(inspector, `data-action="crop-mode"`) {
		t.Error("the inspector has no crop-mode button")
	}
	runVideoStudioEditScript(t, `
require('./js/desktop/apps/video-studio-keyframe-edit.js');
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-inspector.js');
require('./js/desktop/apps/video-studio-core.js');
require('./js/desktop/apps/video-studio-stage.js');
const P = window.VideoStudioParts;
const near = (a, b, message) => assert(Math.abs(a - b) < 1e-9, message + ': ' + a + ' vs ' + b);
const same = (got, want, message) => Object.keys(want).forEach(key => near(got[key], want[key], message + ' ' + key));
const size = {width: 1280, height: 720};
const source = {id: 'v', kind: 'video', width: 1280, height: 720};
const state = extra => Object.assign({x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain', crop: {l: 0, t: 0, r: 0, b: 0}, flipH: false, flipV: false}, extra);

// Contain: a quarter cut on the left. The cropped 960x720 picture fits the full-frame box at x 160; the whole source
// reaches out to x -160.
let g = P.cropGeometry(state({crop: {l: 0.25, t: 0, r: 0, b: 0}}), source, size);
same(g.box, {x: 0, y: 0, w: 1280, h: 720}, 'contain box');
same(g.draw, {x: 160, y: 0, w: 960, h: 720}, 'contain picture');
same(g.full, {x: -160, y: 0, w: 1280, h: 720}, 'contain source');
same(g.edges, {l: 0.25, t: 0, r: 0, b: 0}, 'contain edges');
// The same crop flipped: the cut is on the right of the screen, so the source starts at the picture.
g = P.cropGeometry(state({crop: {l: 0.25, t: 0, r: 0, b: 0}, flipH: true}), source, size);
same(g.full, {x: 160, y: 0, w: 1280, h: 720}, 'flipped source');
same(g.edges, {l: 0, t: 0, r: 0.25, b: 0}, 'flipped edges');
assert.equal(g.flipH, true);
// Cover: the top and bottom quarters cut. The 1280x360 picture covers the centred half-size box and spills sideways.
g = P.cropGeometry(state({x: 0.25, y: 0.25, width: 0.5, height: 0.5, fit: 'cover', crop: {l: 0, t: 0.25, r: 0, b: 0.25}}), source, size);
same(g.box, {x: 320, y: 180, w: 640, h: 360}, 'cover box');
same(g.draw, {x: 0, y: 180, w: 1280, h: 360}, 'cover picture');
same(g.full, {x: 0, y: 0, w: 1280, h: 720}, 'cover source');
// Without a source size the picture fills the box and the fractions place the source.
g = P.cropGeometry(state({crop: {l: 0.2, t: 0, r: 0.3, b: 0}}), {id: 'x'}, size);
same(g.full, {x: -512, y: 0, w: 2560, h: 720}, 'unknown size');
near(P.cropGeometry(state({rotation: 90}), source, size).angle, Math.PI / 2, 'angle');
// A phone video stored 1920x1080 and shown upright (its element reports 1080x1920): the overlay uses the decoded size.
const upright = P.cropGeometry(state(), {id: 'p', kind: 'video', width: 1920, height: 1080}, size, {width: 1080, height: 1920});
assert(Math.abs(upright.full.w / upright.full.h - 1080 / 1920) < 0.01, 'a rotated phone video keeps its upright aspect: ' + JSON.stringify(upright.full));

// Drags and nudges: dx and dy are fractions of the whole source in screen directions.
const drag = (crop, flips, handle, dx, dy) => P.cropDrag(Object.assign({l: 0, t: 0, r: 0, b: 0}, crop), flips.includes('h'), flips.includes('v'), handle, dx, dy);
same(drag({}, '', 'w', 0.3, 0), {l: 0.3, t: 0, r: 0, b: 0}, 'left edge');
same(drag({}, '', 'e', -0.2, 0.5), {l: 0, t: 0, r: 0.2, b: 0}, 'an edge handle ignores the other axis');
same(drag({}, '', 'se', -0.1, -0.2), {l: 0, t: 0, r: 0.1, b: 0.2}, 'corner');
same(drag({}, '', 'w', 1, 0), {l: 0.95, t: 0, r: 0, b: 0}, 'clamped to 0.95');
same(drag({r: 0.5}, '', 'w', 0.8, 0), {l: 0.45, t: 0, r: 0.5, b: 0}, 'l + r stays at most 0.95');
same(drag({l: 0.1}, '', 'w', -0.4, 0), {l: 0, t: 0, r: 0, b: 0}, 'never below 0');
same(drag({}, '', 'nw', 0.12345, 0.0004), {l: 0.123, t: 0, r: 0, b: 0}, 'the 0.001 step');
// Flips mirror the handles: the screen's left handle moves the source's right edge.
same(drag({}, 'h', 'w', 0.3, 0), {l: 0, t: 0, r: 0.3, b: 0}, 'flip_h left handle');
same(drag({l: 0.1, t: 0.1, r: 0.2}, 'h', 'nw', 0.05, 0.05), {l: 0.1, t: 0.15, r: 0.25, b: 0}, 'flip_h corner');
same(drag({t: 0.1}, 'v', 's', 0, -0.05), {l: 0, t: 0.15, r: 0, b: 0}, 'flip_v bottom handle');
same(drag({t: 0.1}, 'hv', 'n', 0, 0.2), {l: 0, t: 0.1, r: 0, b: 0.2}, 'both flips');
// Moving the crop keeps its size and stops at the source's edges.
same(drag({l: 0.1, r: 0.2}, '', 'move', 0.5, 0), {l: 0.3, t: 0, r: 0, b: 0}, 'move right to the edge');
same(drag({l: 0.1, r: 0.2, t: 0.3}, '', 'move', -0.05, -0.1), {l: 0.05, t: 0.2, r: 0.25, b: 0.1}, 'move up and left');
same(drag({l: 0.1, r: 0.2}, 'h', 'move', 0.05, 0), {l: 0.05, t: 0, r: 0.25, b: 0}, 'a flipped picture moves the other way in the source');
same(drag({}, '', 'move', 0.3, 0.3), {l: 0, t: 0, r: 0, b: 0}, 'an uncropped picture has nothing to move');

// Screen pixels to fractions: the delta turns back by the clip's rotation first.
let d = P.cropDragDelta(64, -36, 0, 256, 144);
near(d.dx, 0.25, 'dx'); near(d.dy, -0.25, 'dy');
d = P.cropDragDelta(0, 64, Math.PI / 2, 256, 144);
near(d.dx, 0.25, 'rotated 90 degrees: dragging down moves along the picture x axis');
assert(Math.abs(d.dy) < 1e-9, 'rotated dy');
assert.deepEqual(P.cropDragDelta(10, 10, 0, 0, 0), {dx: 0, dy: 0});

// A gesture writes only the sides it moves. An animated side between its keys is not on the 0.001 step, so rounding it
// would key it (and at 64 keys refuse the whole gesture): the other sides come back exactly as they were.
const BETWEEN = {l: 0.1000001, t: 0.2000002, r: 0.2333333, b: 0.3000004};
const untouched = (got, keys, message) => keys.forEach(key => assert.strictEqual(got[key], BETWEEN[key], message + ' ' + key));
untouched(P.cropDrag(BETWEEN, false, false, 'w', 0.05, 0.3), ['t', 'r', 'b'], 'the left handle');
untouched(P.cropDrag(BETWEEN, false, false, 'e', -0.02, 0.3), ['l', 't', 'b'], 'the right handle');
untouched(P.cropDrag(BETWEEN, false, false, 'n', 0.3, 0.02), ['l', 'r', 'b'], 'the top handle');
untouched(P.cropDrag(BETWEEN, false, false, 'se', -0.02, -0.02), ['l', 't'], 'the bottom right corner');
untouched(P.cropDrag(BETWEEN, false, false, 'move', 0, 0.01), ['l', 'r'], 'a vertical move');
untouched(P.cropDrag(BETWEEN, false, false, 'move', 0.01, 0), ['t', 'b'], 'a horizontal move');
untouched(P.cropDrag(BETWEEN, true, false, 'w', 0.05, 0), ['l', 't', 'b'], 'the screen left handle of a flipped picture moves the source right side');
untouched(P.cropDrag(BETWEEN, false, true, 's', 0, -0.02), ['l', 'r', 'b'], 'the screen bottom handle of a flipped picture moves the source top side');
assert.strictEqual(P.cropDrag(BETWEEN, false, false, 'w', 0.05, 0).l, 0.15, 'the moved side is on the step');
// The side that gives way at 0.95 rounds down, so the sum stays within the limit next to a side off the step.
const full = P.cropDrag({l: 0, t: 0, r: 0.3184, b: 0}, false, false, 'w', 0.8, 0);
assert.strictEqual(full.r, 0.3184); assert.strictEqual(full.l, 0.631); assert(full.l + full.r <= 0.95 + 1e-9, 'l + r stays at most 0.95: ' + (full.l + full.r));

// The same through the keys, with the real edit: crop.r animated with the playhead between its keys.
P.applyEdit = (s, label, op) => op(s.project) === true;
const FXE = window.VideoStudioEffects;
const kf = (f, v, e) => ({f, v, e: e || 'linear'});
const stage = (keys, extra) => {
  const c = clip('c1', V, 0, 200, Object.assign({keyframes: keys}, extra || {}));
  const p = project([track('v1', 'video', [c])]);
  return {p, c, s: {project: p, frame: 20, selectedClipId: 'c1', stageMode: 'timeline', readonly: false, cropMode: {clipId: 'c1'}, cropDrag: null}};
};
const press = (s, key, handle) => P.handleCropKeys(s, {key, shiftKey: false, altKey: false, ctrlKey: false, metaKey: false, target: {closest: sel => sel === '[data-crop-box]' ? {} : sel === '[data-crop-handle]' && handle ? {dataset: {cropHandle: handle}} : null}, preventDefault() {}, stopPropagation() {}});
let fix = stage({'crop.r': [kf(0, 0.1), kf(60, 0.5)]});
const rKeys = JSON.stringify(fix.c.keyframes['crop.r']);
assert(Math.abs(Math.round(FXE.valueAt(fix.c, 'crop.r', 20, fix.p) * 1000) / 1000 - FXE.valueAt(fix.c, 'crop.r', 20, fix.p)) > 1e-5, 'the fixture needs an r that is off the step');
assert.equal(press(fix.s, 'ArrowRight', 'w'), true);
assert.equal(JSON.stringify(fix.c.keyframes['crop.r']), rKeys, 'dragging the left handle leaves the keys of r alone');
near(fix.c.crop.l, 0.01, 'the left side moved');
assert.equal(fix.c.crop.r, 0, 'the static field of an animated side is never written');
// With l animated as well, the key lands on the side the handle moves.
fix = stage({'crop.l': [kf(0, 0.05), kf(60, 0.4)], 'crop.r': [kf(0, 0.1), kf(60, 0.5)]});
const both = JSON.parse(JSON.stringify(fix.c.keyframes));
press(fix.s, 'ArrowRight', 'w');
assert.equal(fix.c.keyframes['crop.l'].length, 3, 'the left handle keys l at the playhead');
assert.deepEqual(fix.c.keyframes['crop.r'], both['crop.r'], 'r keeps its keys');
// A vertical move of the whole crop leaves the animated l and r alone.
fix = stage({'crop.l': [kf(0, 0.05), kf(60, 0.4)], 'crop.r': [kf(0, 0.1), kf(60, 0.5)]}, {crop: {l: 0, t: 0.2, r: 0, b: 0.2}});
const rails = JSON.parse(JSON.stringify(fix.c.keyframes));
press(fix.s, 'ArrowDown');
assert.deepEqual(fix.c.keyframes, rails, 'a vertical move does not key l or r');
near(fix.c.crop.t, 0.21, 'the top moved'); near(fix.c.crop.b, 0.19, 'the bottom moved');
// 64 keys on r: dragging the left handle is not refused for a key that is not its own.
fix = stage({'crop.r': Array.from({length: 64}, (_, i) => kf(i * 3, 0.05 + i * 0.005))});
assert.equal(press(fix.s, 'ArrowRight', 'w'), true);
assert.equal(fix.c.keyframes['crop.r'].length, 64);
near(fix.c.crop.l, 0.01, 'l moved although r has 64 keys');
`)
}
