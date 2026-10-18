package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// On-canvas mask handles (Task 15). The test checks:
//   - the wiring the stage relies on and the handle strings in every locale;
//   - the DOM-free math of video-studio-mask.js: the overlay geometry under a turned clip and a turned mask (law L of
//     plan index §4.22), drags of every handle with the opposite edge fixed in mask space, the rotate and feather knobs,
//     the registry clamps, the screen-to-box delta and the key steps.
func TestVideoStudioMaskHandles(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("js", "desktop", "apps", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	app, stage := read("video-studio.js"), read("video-studio-stage.js")
	for _, want := range []string{"P.wireMaskBox(s);", "action === 'mask-mode'", "maskMode: !!s.maskMode"} {
		if !strings.Contains(app, want) {
			t.Errorf("video-studio.js lacks %s", want)
		}
	}
	crop, mask := strings.Index(app, "P.handleCropKeys(s, event)"), strings.Index(app, "P.handleMaskKeys(s, event)")
	textGuard := strings.Index(app, "if (!sourceSlider && target.closest('input,textarea,select,[contenteditable=\"true\"]')) return;")
	if crop < 0 || mask < crop || textGuard < mask {
		t.Error("handleKeys must try P.handleMaskKeys after P.handleCropKeys and before the text-field guard")
	}
	start, end := strings.Index(stage, "function updateTransform(s) {"), strings.Index(stage, "function syncPreviewNotice(s) {")
	if start < 0 || end < start || !strings.Contains(stage[start:end], "P.updateMaskBox(s)") || !strings.Contains(stage[start:end], "!s.maskMode") {
		t.Error("updateTransform must keep the mask overlay in step and hide the transform box in mask mode")
	}
	if !strings.Contains(stage, "if (found && s.maskMode && P.setMaskMode) P.setMaskMode(s, false);") {
		t.Error("crop mode must end mask mode")
	}
	keys := []string{"maskBoxHint", "maskBoxLabel", "maskHandle_nw", "maskHandle_n", "maskHandle_ne", "maskHandle_e", "maskHandle_se", "maskHandle_s", "maskHandle_sw", "maskHandle_w", "maskHandle_rotate", "maskHandle_feather"}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		for _, mark := range []string{"{{x}}", "{{y}}", "{{w}}", "{{h}}", "{{r}}", "{{f}}"} {
			if !strings.Contains(values["videoStudio.maskBoxLabel"], mark) {
				t.Errorf("%s videoStudio.maskBoxLabel lost %s: %q", lang, mark, values["videoStudio.maskBoxLabel"])
			}
		}
		if lang != "en" && values["videoStudio.maskBoxHint"] == english["videoStudio.maskBoxHint"] {
			t.Errorf("%s copies the English videoStudio.maskBoxHint", lang)
		}
	}
	runVideoStudioEditScript(t, `
require('./js/desktop/apps/video-studio-keyframe-edit.js');
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-inspector.js');
require('./js/desktop/apps/video-studio-core.js');
require('./js/desktop/apps/video-studio-stage.js');
require('./js/desktop/apps/video-studio-mask.js');
const P = window.VideoStudioParts;
const near = (a, b, message, eps) => assert(Math.abs(a - b) < (eps || 1e-9), message + ': ' + a + ' vs ' + b);
const same = (got, want, message, eps) => Object.keys(want).forEach(key => near(got[key], want[key], message + ' ' + key, eps));
const size = {width: 1280, height: 720};
const box = {w: 640, h: 360};
const state = mask => ({x: 0.25, y: 0.25, width: 0.5, height: 0.5, rotation: 30, opacity: 1, fit: 'contain', crop: {l: 0, t: 0, r: 0, b: 0}, flipH: false, flipV: false, blend: 'normal', mask, effects: []});
const MASK = {shape: 'ellipse', invert: true, x: 0.25, y: 0.2, w: 0.5, h: 0.6, rotation: 15, feather: 0.04};

// Geometry: the clip box at the playhead (VideoStudioEffects.placement) turned by the clip, and the mask in box pixels
// turned about its own centre.
let g = P.maskGeometry(state(MASK), size);
same(g.box, {x: 320, y: 180, w: 640, h: 360}, 'box');
near(g.angle, Math.PI / 6, 'clip angle');
same(g, {cx: 320, cy: 180, rx: 160, ry: 108, theta: 15 * Math.PI / 180, ramp: 14.4}, 'mask frame');
assert.equal(g.mask.shape, 'ellipse'); assert.equal(g.mask.invert, true);
same(g.mask, {x: 0.25, y: 0.2, w: 0.5, h: 0.6, rotation: 15, feather: 0.04}, 'mask values');
// The same frame as law L in VideoStudioPicture (plan index §4.4a).
same(window.VideoStudioPicture.maskFrame(MASK, {width: 640, height: 360}), {cx: g.cx, cy: g.cy, rx: g.rx, ry: g.ry, ramp: g.ramp}, 'maskFrame agrees');
// Feather 0 keeps the 1 px floor.
g = P.maskGeometry(state(Object.assign({}, MASK, {shape: 'rect', invert: false, feather: 0})), size);
near(g.ramp, 1, 'feather floor'); assert.equal(g.mask.shape, 'rect'); assert.equal(g.mask.invert, false);

// Moves and size handles on an unturned mask; dx and dy are box pixels in box directions.
const FLAT = {shape: 'rect', x: 0.25, y: 0.2, w: 0.5, h: 0.6, rotation: 0, feather: 0.04, invert: false};
same(P.maskDrag(FLAT, 'move', 64, -36, box), {x: 0.35, y: 0.1, w: 0.5, h: 0.6, rotation: 0, feather: 0.04}, 'move');
same(P.maskDrag(FLAT, 'move', 1e5, -1e5, box), {x: 1, y: -1, w: 0.5, h: 0.6}, 'move clamps to -1..1');
same(P.maskDrag(FLAT, 'e', 64, 30, box), {x: 0.25, y: 0.2, w: 0.6, h: 0.6}, 'the right edge ignores dy');
same(P.maskDrag(FLAT, 'w', 64, 0, box), {x: 0.35, w: 0.4}, 'left edge');
same(P.maskDrag(FLAT, 'se', 64, 36, box), {x: 0.25, y: 0.2, w: 0.6, h: 0.7}, 'corner');
same(P.maskDrag(FLAT, 'n', 0, 36, box), {y: 0.3, h: 0.5}, 'top edge');
same(P.maskDrag(FLAT, 'e', -1000, 0, box), {x: 0.25, w: 0.01}, 'the width stops at 1 % and the left edge stays');
same(P.maskDrag(FLAT, 'e', 1e5, 0, box), {x: 0.25, w: 2}, 'the width stops at 200 %');
// A turned mask (30°): whatever the drag, the opposite edge or corner stays in place in box pixels, and the dragged
// size follows the drag projected on the mask's axes.
const TURNED = Object.assign({}, FLAT, {rotation: 30});
const point = (m, ux, uy) => { const t = m.rotation * Math.PI / 180, cx = (m.x + m.w / 2) * box.w, cy = (m.y + m.h / 2) * box.h, lx = ux * m.w * box.w / 2, ly = uy * m.h * box.h / 2; return [cx + lx * Math.cos(t) - ly * Math.sin(t), cy + lx * Math.sin(t) + ly * Math.cos(t)]; };
const OPPOSITE = {nw: [1, 1], n: [0, 1], ne: [-1, 1], e: [-1, 0], se: [-1, -1], s: [0, -1], sw: [1, -1], w: [1, 0]};
Object.entries(OPPOSITE).forEach(([handle, [ux, uy]]) => {
  const out = Object.assign({}, TURNED, P.maskDrag(TURNED, handle, 23, -17, box)), a = point(TURNED, ux, uy), b = point(out, ux, uy);
  assert(Math.hypot(a[0] - b[0], a[1] - b[1]) < 0.002, handle + ' keeps the opposite edge: ' + a + ' vs ' + b);
  near(out.rotation, 30, handle + ' rotation'); near(out.feather, 0.04, handle + ' feather');
});
const lx = 23 * Math.cos(Math.PI / 6) - 17 * Math.sin(Math.PI / 6), ly = -23 * Math.sin(Math.PI / 6) - 17 * Math.cos(Math.PI / 6);
near(P.maskDrag(TURNED, 'e', 23, -17, box).w, (320 + lx) / 640, 'turned right edge', 1e-6);
near(P.maskDrag(TURNED, 'n', 23, -17, box).h, (216 - ly) / 360, 'turned top edge', 1e-6);

// The rotate knob starts 24 screen px above the top edge (box px = screen px / scale). A quarter turn of the pointer
// around the centre turns the mask by 90°, wrapping inside ±360°.
same(P.maskDrag(FLAT, 'rotate', 132, 132, {w: 640, h: 360, scale: 1}), {rotation: 90, x: 0.25, y: 0.2, w: 0.5, h: 0.6}, 'quarter turn', 1e-6);
same(P.maskDrag(FLAT, 'rotate', 120, 120, {w: 640, h: 360, scale: 2}), {rotation: 90}, 'the knob distance follows the screen scale', 1e-6);
const t350 = 350 * Math.PI / 180, p0 = [132 * Math.sin(t350), -132 * Math.cos(t350)], p1 = [-p0[1], p0[0]];
near(P.maskDrag(Object.assign({}, FLAT, {rotation: 350}), 'rotate', p1[0] - p0[0], p1[1] - p0[1], box).rotation, 80, 'wraps past 360', 1e-6);
// The feather knob: a drag of d box px outward (along the mask's own down axis) widens the ramp F by 2d.
near(P.maskDrag(FLAT, 'feather', 0, 10, box).feather, (14.4 + 20) / 360, 'feather drag', 1e-6);
near(P.maskDrag(Object.assign({}, FLAT, {rotation: 90}), 'feather', -10, 0, box).feather, (14.4 + 20) / 360, 'feather on a turned mask', 1e-6);
near(P.maskDrag(FLAT, 'feather', 0, 1000, box).feather, 0.5, 'feather max');
near(P.maskDrag(FLAT, 'feather', 0, -1000, box).feather, 0, 'feather min');

// Screen pixels to box pixels: the delta turns back by the clip's rotation.
same(P.maskDragDelta(64, -36, 0), {dx: 64, dy: -36}, 'unturned');
same(P.maskDragDelta(0, 64, Math.PI / 2), {dx: 64, dy: 0}, 'on a clip turned 90°, dragging down moves along the box x axis');

// Keys:
// - on the mask, 1 % of the box per press (Shift 10 %);
// - on a size handle, its edges move by the same steps in mask space;
// - on the rotate knob, 1° (Shift 15°); on the feather knob, 0.001 (Shift 0.01);
// - any key but an arrow is no nudge.
same(P.maskNudge(FLAT, 'move', 'ArrowRight', false, box), {x: 0.26, y: 0.2, w: 0.5}, 'ArrowRight');
same(P.maskNudge(FLAT, 'move', 'ArrowDown', true, box), {x: 0.25, y: 0.3}, 'Shift+ArrowDown');
same(P.maskNudge(FLAT, 'e', 'ArrowRight', false, box), {x: 0.25, w: 0.51}, 'right handle');
same(P.maskNudge(FLAT, 'e', 'ArrowLeft', true, box), {x: 0.25, w: 0.4}, 'Shift on the right handle');
same(P.maskNudge(FLAT, 'n', 'ArrowUp', false, box), {y: 0.19, h: 0.61}, 'top handle');
const nudged = Object.assign({}, TURNED, P.maskNudge(TURNED, 'e', 'ArrowRight', false, box)), left = point(TURNED, -1, 0), kept = point(nudged, -1, 0);
assert(Math.hypot(left[0] - kept[0], left[1] - kept[1]) < 0.002, 'a turned mask keeps its left edge');
near(nudged.w, 0.51, 'a turned mask grows along its own axis', 1e-6);
near(P.maskNudge(FLAT, 'rotate', 'ArrowUp', false, box).rotation, 1, 'rotate up');
near(P.maskNudge(FLAT, 'rotate', 'ArrowLeft', true, box).rotation, -15, 'Shift rotate left');
near(P.maskNudge(Object.assign({}, FLAT, {rotation: 360}), 'rotate', 'ArrowRight', false, box).rotation, 360, 'rotation stays within 360');
near(P.maskNudge(FLAT, 'feather', 'ArrowUp', false, box).feather, 0.041, 'feather up');
near(P.maskNudge(FLAT, 'feather', 'ArrowDown', true, box).feather, 0.03, 'Shift feather down');
near(P.maskNudge(Object.assign({}, FLAT, {feather: 0}), 'feather', 'ArrowDown', false, box).feather, 0, 'feather stays at 0');
assert.equal(P.maskNudge(FLAT, 'move', 'Delete', false, box), null);
`)
}
