package ui

import (
	"encoding/json"
	"math"
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopVideoStudioCompositingBrowser drives the 2d compositing UI in the real app on the keyframe fixture with the
// default launcher. The controls and the mask overlay are DOM, so either painter works. Subtests: inspector (Task 14)
// and handles (Task 15).
func TestDesktopVideoStudioCompositingBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	t.Run("inspector", videoStudioCompositingInspectorPart)
	t.Run("handles", videoStudioMaskHandlesPart)
}

// videoStudioCompositingInspectorPart (Task 14) works on c1, the 512x288 picture at half size on overlay o1:
//   - the blend list sets Multiply and then Normal (which deletes the field), one undo step each;
//   - Rectangle adds the default mask, Ellipse keeps its values, a typed width and Invert change it;
//   - the mask.x stopwatch keys frame 30, a typed value at frame 90 keys frame 90, and outside the clip the animated
//     field freezes;
//   - "+ Effect" and the effects library put chroma_key at stack index 0; the presets and the colour input set the key
//     colour;
//   - in the narrow window, the drawer shows the Picture section without horizontal scrolling.
func videoStudioCompositingInspectorPart(t *testing.T) {
	page := openVideoStudioKeyframes(t, newSmokeBrowser(t), newVideoStudioKeyframesServer(t, videoStudioKeyframesProject()))
	shots := videoStudioKeyframesShotDir(t)
	const app = `window.VideoStudioApp.instances.get('kf')`
	history := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.history.length`).Int() }
	seek := func(f int) {
		t.Helper()
		page.MustEval(`f => { const s = `+app+`; s.preview.pause(); s.preview.seek(f); }`, f)
	}
	change := func(selector, value string) {
		t.Helper()
		page.MustEval(`(sel, value) => { const el = document.querySelector(sel); if (!el) throw new Error('missing ' + sel); el.value = value; el.dispatchEvent(new Event('change', { bubbles: true })); }`, selector, value)
	}
	mask := func() map[string]any {
		t.Helper()
		m, _ := effectsClip(t, page)["mask"].(map[string]any)
		return m
	}
	number := func(m map[string]any, key string) float64 { value, _ := m[key].(float64); return value }

	page.MustEval(`() => { const s = ` + app + `; s.timelineOptions.onSelect('c1'); s.preview.pause(); s.preview.seek(30); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-inspector] select[data-blend]')`)
	if painter := page.MustEval(`() => ` + app + `.preview.painterKind()`).Str(); painter != "gl" && painter != "2d" {
		t.Fatalf("painterKind() = %q", painter)
	}

	// Blend mode: one undo step; Normal deletes the field.
	steps := history()
	change(`[data-inspector] select[data-blend]`, "multiply")
	if got := effectsClip(t, page)["blend"]; got != "multiply" {
		t.Fatalf("blend after choosing Multiply: %v", got)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the blend change made %d undo steps, want 1", got)
	}
	if got := page.MustEval(`() => document.querySelector('[data-inspector] select[data-blend]').value`).Str(); got != "multiply" {
		t.Errorf("the re-rendered list shows %q", got)
	}
	change(`[data-inspector] select[data-blend]`, "normal")
	if _, ok := effectsClip(t, page)["blend"]; ok {
		t.Fatal("Normal kept the blend field")
	}

	// Rectangle adds the registry's default mask in one undo step, and Edit mask in the preview becomes available.
	steps = history()
	page.MustElement(`[data-inspector] [data-mask-set="rect"]`).MustClick()
	m := mask()
	if m["shape"] != "rect" || number(m, "x") != 0.25 || number(m, "y") != 0.25 || number(m, "w") != 0.5 || number(m, "h") != 0.5 || number(m, "rotation") != 0 || number(m, "feather") != 0 || m["invert"] != false {
		t.Fatalf("mask after Rectangle: %v", m)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("Rectangle made %d undo steps, want 1", got)
	}
	if page.MustEval(`() => document.querySelector('[data-inspector] [data-action="mask-mode"]').disabled`).Bool() {
		t.Error("Edit mask in the preview stays disabled with a mask and the playhead in the clip")
	}
	// Ellipse keeps the values; a typed width and Invert change them.
	page.MustElement(`[data-inspector] [data-mask-set="ellipse"]`).MustClick()
	change(`[data-inspector] [data-mask-field="w"]`, "60")
	page.MustElement(`[data-inspector] [data-mask-invert]`).MustClick()
	m = mask()
	if m["shape"] != "ellipse" || math.Abs(number(m, "w")-0.6) > 1e-9 || number(m, "x") != 0.25 || m["invert"] != true {
		t.Fatalf("mask after Ellipse, width 60 %% and Invert: %v", m)
	}

	// Keyframes: the stopwatch keys frame 30; a typed value at frame 90 keys frame 90.
	page.MustElement(`[data-inspector] [data-kf-path="mask.x"][data-kf-action="stopwatch"]`).MustClick()
	requireVideoStudioKeys(t, "mask.x stopwatch", videoStudioClipKeys(t, page, "c1", "mask.x"), videoStudioKeyframe{30, 0.25, "linear"})
	seek(90)
	change(`[data-inspector] [data-mask-field="x"]`, "40")
	requireVideoStudioKeys(t, "mask.x at frame 90", videoStudioClipKeys(t, page, "c1", "mask.x"), videoStudioKeyframe{30, 0.25, "linear"}, videoStudioKeyframe{90, 0.4, "linear"})
	// Outside the clip (c1 covers frames 0–149) the animated field freezes, and so does mask mode; static fields stay
	// editable.
	seek(160)
	waitForJSBool(t, page, `() => { const field = document.querySelector('[data-inspector] [data-mask-field="x"]'), button = document.querySelector('[data-inspector] [data-action="mask-mode"]'); return !!field && field.disabled && !!button && button.disabled; }`)
	if page.MustEval(`() => document.querySelector('[data-inspector] [data-mask-field="y"]').disabled`).Bool() {
		t.Error("a static mask field froze outside the clip")
	}
	seek(30)

	// "+ Effect" puts chroma_key at the top of the stack, and its header takes focus.
	effectsAdd(t, page, "color")
	effectsAdd(t, page, "chroma_key")
	if got := effectsIDs(effectsStack(t, page)); got != "chroma_key,color" {
		t.Fatalf("stack after adding Chroma key: %q", got)
	}
	if !effectsFocused(page, `[data-inspector] [data-fx-entry="0"] [data-fx-head]`) {
		t.Error("the new chroma key's header did not take focus")
	}
	key := `[data-inspector] [data-fx-entry="0"] `
	if got := effectsStack(t, page)[0].Params["color"]; got != "#00b140" {
		t.Errorf("default key colour %v", got)
	}
	steps = history()
	page.MustElement(key + `[data-fx-key-preset="blue"]`).MustClick()
	if got := effectsStack(t, page)[0].Params["color"]; got != "#0047bb" {
		t.Fatalf("key colour after Blue screen: %v", got)
	}
	if got := page.MustEval(`sel => document.querySelector(sel).getAttribute('aria-pressed')`, key+`[data-fx-key-preset="blue"]`).Str(); got != "true" {
		t.Errorf("Blue screen aria-pressed = %q", got)
	}
	change(key+`input[type="color"][data-fx-param="color"]`, "#20c040")
	if got := effectsStack(t, page)[0].Params["color"]; got != "#20c040" {
		t.Fatalf("key colour after the colour input: %v", got)
	}
	if got := history() - steps; got != 2 {
		t.Errorf("the preset and the colour input made %d undo steps, want 2", got)
	}

	// The effects library puts chroma_key at the top too (Enter on a focused item adds it to the selected clip).
	page.MustElement(`[data-tab="effects"]`).MustClick()
	item := `[data-effects-library] [data-effect-item="chroma_key"]`
	waitForJSBool(t, page, `() => { const el = document.querySelector('`+item+`'); return !!el && el.getAttribute('aria-disabled') === 'false'; }`)
	page.MustElement(item).MustFocus()
	page.KeyActions().Type(input.Enter).MustDo()
	waitForJSBool(t, page, `() => document.querySelectorAll('[data-inspector] [data-fx-entry]').length === 3`)
	stack := effectsStack(t, page)
	if got := effectsIDs(stack); got != "chroma_key,chroma_key,color" || stack[1].Params["color"] != "#20c040" {
		t.Fatalf("stack after the library's chroma key: %q (%+v)", got, stack)
	}
	// Screenshots for review: the blend list with a mode chosen, the mask block, the key entry, and the whole Picture
	// section in a tall window.
	change(`[data-inspector] select[data-blend]`, "multiply")
	scroll := func(selector, block string) {
		t.Helper()
		page.MustEval(`(sel, block) => document.querySelector(sel).scrollIntoView({ block })`, selector, block)
	}
	scroll(`[data-inspector] .vs-blend`, "start")
	page.MustScreenshot(filepath.Join(shots, "compositing-inspector-blend-1280x900.png"))
	scroll(`[data-inspector] .vs-mask`, "center")
	page.MustScreenshot(filepath.Join(shots, "compositing-inspector-1280x900.png"))
	scroll(`[data-inspector] [data-fx-entry="0"]`, "start")
	page.MustScreenshot(filepath.Join(shots, "compositing-inspector-key-1280x900.png"))
	page.MustSetViewport(1280, 1500, 1, false)
	scroll(`[data-inspector] .vs-insp-section:has(.vs-mask)`, "start")
	page.MustScreenshot(filepath.Join(shots, "compositing-inspector-tall-1280x1500.png"))
	page.MustSetViewport(1280, 900, 1, false)

	// The narrow window: the inspector drawer holds the Picture section without horizontal scrolling.
	page.MustSetViewport(900, 900, 1, false)
	page.MustEval(`() => ` + app + `.timelineOptions.onSelect('c1')`)
	waitForJSBool(t, page, `() => document.querySelector('.vs-app').classList.contains('vs-show-inspector')`)
	if !page.MustEval(`() => { const el = document.querySelector('.vs-inspector'); return el.scrollWidth <= el.clientWidth + 1; }`).Bool() {
		t.Error("the narrow inspector scrolls sideways")
	}
	page.MustEval(`() => document.querySelector('[data-inspector] .vs-mask').scrollIntoView({ block: 'center' })`)
	page.MustScreenshot(filepath.Join(shots, "compositing-inspector-900x900.png"))
	page.MustSetViewport(1280, 900, 1, false)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors in the compositing inspector part: %s", errs)
	}
}

// videoStudioMaskHandlesPart is the handles subtest of TestDesktopVideoStudioCompositingBrowser (Task 15). It works on c1
// (the 512x288 picture at half size in the top left corner of overlay o1) with the default rectangle mask:
//   - Edit mask in the preview swaps the transform box for the mask overlay and focuses the mask; crop mode and mask
//     mode replace each other.
//   - A drag of the right handle by a tenth of the clip box widens the mask by 0.1 in one undo step; the left edge stays.
//   - Keys: 1 % per press on a size handle (Shift 10 %) and on the mask; 1° and 15° on the rotate knob; 0.001 on the
//     feather knob. Delete leaves the clip and the mask alone; Escape leaves and focuses the button.
//   - The mode ends when the playhead leaves the clip; with mask.x animated, a nudge keys mask.x at the playhead only.
//   - A title takes mask mode too.
func videoStudioMaskHandlesPart(t *testing.T) {
	page := openVideoStudioKeyframes(t, newSmokeBrowser(t), newVideoStudioKeyframesServer(t, videoStudioKeyframesProject()))
	shots := videoStudioKeyframesShotDir(t)
	const app = `window.VideoStudioApp.instances.get('kf')`
	const button = `[data-inspector] [data-action="mask-mode"]`
	const masking = `() => { const box = document.querySelector('[data-mask-box]'); return !!box && !box.hidden && document.querySelector('[data-transform]').hidden; }`
	maskOf := func() map[string]float64 {
		t.Helper()
		raw := page.MustEval(`() => { const found = window.VideoStudioTimeline.clipFor(` + app + `.project, 'c1'); const m = found && found.clip.mask; return JSON.stringify(m ? {x: m.x, y: m.y, w: m.w, h: m.h, rotation: m.rotation, feather: m.feather} : null); }`).Str()
		var out map[string]float64
		if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
			t.Fatalf("mask of c1: %v (%s)", err, raw)
		}
		return out
	}
	requireMask := func(step string, want map[string]float64, tolerance float64) {
		t.Helper()
		got := maskOf()
		for key, value := range want {
			if math.Abs(got[key]-value) > tolerance {
				t.Fatalf("%s: mask %v, want %s = %.4f", step, got, key, value)
			}
		}
	}
	history := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.history.length`).Int() }
	seek := func(f int) {
		t.Helper()
		page.MustEval(`f => { const s = `+app+`; s.preview.pause(); s.preview.seek(f); }`, f)
	}
	press := func(key input.Key, shift bool) {
		t.Helper()
		if shift {
			page.KeyActions().Press(input.ShiftLeft).Type(key).MustDo()
			return
		}
		page.KeyActions().Type(key).MustDo()
	}
	enter := func() {
		t.Helper()
		page.MustElement(button).MustClick()
		waitForJSBool(t, page, masking)
	}

	page.MustEval(`() => { const s = ` + app + `; s.timelineOptions.onSelect('c1'); s.preview.pause(); s.preview.seek(30); }`)
	if !page.MustEval(`() => window.VideoStudioParts.applyEdit(` + app + `, 'mask', project => window.VideoStudioKeyframeEdit.setMask(project, 'c1', {shape: 'rect'}, 30))`).Bool() {
		t.Fatal("setMask(rect) was refused")
	}
	waitForJSBool(t, page, `() => { const b = document.querySelector('`+button+`'); return !!b && !b.disabled; }`)
	enter()
	if !effectsFocused(page, `[data-mask-shape]`) {
		t.Error("the mask did not take keyboard focus")
	}
	if got := page.MustEval(`sel => document.querySelector(sel).getAttribute('aria-pressed')`, button).Str(); got != "true" {
		t.Errorf("mask-mode aria-pressed = %q", got)
	}

	// Crop mode replaces mask mode, and mask mode replaces crop mode.
	page.MustElement(`[data-inspector] [data-action="crop-mode"]`).MustClick()
	waitForJSBool(t, page, `() => !document.querySelector('[data-crop-box]').hidden && document.querySelector('[data-mask-box]').hidden && !`+app+`.maskMode`)
	enter()
	if !page.MustEval(`() => document.querySelector('[data-crop-box]').hidden && !` + app + `.cropMode`).Bool() {
		t.Fatal("mask mode left crop mode on")
	}

	// A drag of the right handle by a tenth of the clip box: width 0.6 in one undo step, and the left edge stays.
	at := page.MustEval(`() => { const h = document.querySelector('[data-mask-handle="e"]').getBoundingClientRect(), b = document.querySelector('[data-mask-box]').getBoundingClientRect(); return {x: h.left + h.width / 2, y: h.top + h.height / 2, w: b.width}; }`)
	x0, y0, width := at.Get("x").Num(), at.Get("y").Num(), at.Get("w").Num()
	if width < 100 {
		t.Fatalf("the clip box is %.1f px wide", width)
	}
	steps := history()
	page.Mouse.MustMoveTo(x0, y0).MustDown(proto.InputMouseButtonLeft).MustMoveTo(x0+width/20, y0).MustMoveTo(x0+width/10, y0).MustUp(proto.InputMouseButtonLeft)
	requireMask("right handle drag", map[string]float64{"x": 0.25, "y": 0.25, "w": 0.6, "h": 0.5}, 0.006)
	if got := history(); got != steps+1 {
		t.Fatalf("the drag left %d undo steps, want 1", got-steps)
	}
	if selected := page.MustEval(`() => ` + app + `.selectedClipId`).Str(); selected != "c1" {
		t.Fatalf("the drag changed the selection to %q", selected)
	}
	waitForJSBool(t, page, masking)

	// Keys: the focused right handle moves by 1 % and, with Shift, by 10 % of the box; the mask itself by 1 %.
	w0 := maskOf()["w"]
	page.MustElement(`[data-mask-handle="e"]`).MustFocus()
	press(input.ArrowLeft, false)
	requireMask("ArrowLeft on the right handle", map[string]float64{"w": w0 - 0.01, "x": 0.25}, 1e-6)
	press(input.ArrowLeft, true)
	requireMask("Shift+ArrowLeft on the right handle", map[string]float64{"w": w0 - 0.11, "x": 0.25}, 1e-6)
	if !effectsFocused(page, `[data-mask-handle="e"]`) {
		t.Error("the handle lost focus after a nudge")
	}
	page.MustElement(`[data-mask-shape]`).MustFocus()
	press(input.ArrowRight, false)
	requireMask("ArrowRight on the mask", map[string]float64{"x": 0.26, "w": w0 - 0.11}, 1e-6)
	page.MustElement(`[data-mask-handle="rotate"]`).MustFocus()
	press(input.ArrowUp, false)
	press(input.ArrowUp, true)
	requireMask("rotate knob", map[string]float64{"rotation": 16}, 1e-9)
	page.MustElement(`[data-mask-handle="feather"]`).MustFocus()
	press(input.ArrowUp, false)
	requireMask("feather knob", map[string]float64{"feather": 0.001}, 1e-9)

	// Delete stays in the overlay: the clip and its mask remain.
	page.MustElement(`[data-mask-shape]`).MustFocus()
	press(input.Delete, false)
	if page.MustEval(`() => !window.VideoStudioTimeline.clipFor(` + app + `.project, 'c1')`).Bool() {
		t.Fatal("Delete in mask mode deleted the clip")
	}
	maskOf()
	page.MustScreenshot(filepath.Join(shots, "mask-handles-1280x900.png"))

	// Escape leaves: the transform box is back and focus returns to the inspector button.
	press(input.Escape, false)
	waitForJSBool(t, page, `() => document.querySelector('[data-mask-box]').hidden && !document.querySelector('[data-transform]').hidden`)
	if !effectsFocused(page, button) {
		t.Error("Escape did not return focus to Edit mask in the preview")
	}

	// The mode ends when the playhead leaves the clip.
	enter()
	seek(160)
	waitForJSBool(t, page, `() => document.querySelector('[data-mask-box]').hidden && !`+app+`.maskMode`)
	seek(30)

	// With mask.x keyed at frame 30, a nudge at frame 90 keys mask.x there (auto-key) and nothing else; Enter leaves.
	if !page.MustEval(`() => window.VideoStudioParts.applyEdit(` + app + `, 'animate mask', project => window.VideoStudioKeyframeEdit.setAnimated(project, 'c1', 'mask.x', true, 30))`).Bool() {
		t.Fatal("setAnimated(mask.x) was refused")
	}
	x1 := maskOf()["x"]
	requireVideoStudioKeys(t, "mask.x stopwatch", videoStudioClipKeys(t, page, "c1", "mask.x"), videoStudioKeyframe{30, x1, "linear"})
	seek(90)
	enter()
	page.MustElement(`[data-mask-shape]`).MustFocus()
	press(input.ArrowRight, false)
	requireVideoStudioKeys(t, "auto-key at frame 90", videoStudioClipKeys(t, page, "c1", "mask.x"), videoStudioKeyframe{30, x1, "linear"}, videoStudioKeyframe{90, x1 + 0.01, "linear"})
	if keys := videoStudioClipKeys(t, page, "c1", "mask.y"); keys != nil {
		t.Errorf("the nudge keyed mask.y: %+v", keys)
	}

	// A drag of the mask at frame 90, between the keys of an animated mask.w (frames 30 and 120, ending at 0.99: not a round
	// value at frame 90), keys the dragged number only; w keeps its two keys.
	if !page.MustEval(`() => window.VideoStudioParts.applyEdit(` + app + `, 'animate w', project => window.VideoStudioKeyframeEdit.setAnimated(project, 'c1', 'mask.w', true, 30) === true && window.VideoStudioKeyframeEdit.setValues(project, 'c1', {'mask.w': 0.99}, 120) === true)`).Bool() {
		t.Fatal("animating mask.w was refused")
	}
	if w90 := page.MustEval(`() => window.VideoStudioEffects.valueAt(window.VideoStudioTimeline.clipFor(` + app + `.project, 'c1').clip, 'mask.w', 90, ` + app + `.project)`).Num(); math.Abs(math.Round(w90*1e6)/1e6-w90) < 1e-8 {
		t.Fatalf("mask.w at frame 90 is %v, a round value the test cannot tell from a rounded one", w90)
	}
	wKeys := videoStudioClipKeys(t, page, "c1", "mask.w")
	if len(wKeys) != 2 {
		t.Fatalf("mask.w keys %+v, want two", wKeys)
	}
	xKeys := videoStudioClipKeys(t, page, "c1", "mask.x")
	centre := page.MustEval(`() => { const r = document.querySelector('[data-mask-shape]').getBoundingClientRect(); return {x: r.left + r.width / 2, y: r.top + r.height / 2}; }`)
	cx, cy := centre.Get("x").Num(), centre.Get("y").Num()
	page.Mouse.MustMoveTo(cx, cy).MustDown(proto.InputMouseButtonLeft).MustMoveTo(cx+10, cy).MustMoveTo(cx+20, cy).MustUp(proto.InputMouseButtonLeft)
	requireVideoStudioKeys(t, "a drag between the keys of w", videoStudioClipKeys(t, page, "c1", "mask.w"), wKeys...)
	if got := videoStudioClipKeys(t, page, "c1", "mask.x"); len(got) != 2 || got[1].F != 90 || got[1].V <= xKeys[1].V+0.001 {
		t.Errorf("the drag did not move the key of mask.x at frame 90: keys %+v, before %+v", got, xKeys)
	}
	if keys := videoStudioClipKeys(t, page, "c1", "mask.y"); keys != nil {
		t.Errorf("the drag keyed mask.y: %+v", keys)
	}
	press(input.Enter, false)
	waitForJSBool(t, page, `() => document.querySelector('[data-mask-box]').hidden`)

	// A turned clip with a turned, feathered ellipse mask (screenshot for review): the overlay turns with the clip and
	// the dashed outline with the mask.
	seek(30)
	if !page.MustEval(`() => window.VideoStudioParts.applyEdit(` + app + `, 'turn', project => window.VideoStudioKeyframeEdit.setValues(project, 'c1', {rotation: 20}, 30) === true && window.VideoStudioKeyframeEdit.setMask(project, 'c1', {shape: 'ellipse', rotation: 30, feather: 0.1}, 30) === true)`).Bool() {
		t.Fatal("turning the clip and the mask was refused")
	}
	enter()
	if got := page.MustEval(`() => document.querySelector('[data-mask-box]').style.transform`).Str(); got == "" {
		t.Error("the overlay does not turn with the clip")
	}
	if got := page.MustEval(`() => document.querySelector('[data-mask-shape]').dataset.maskShape`).Str(); got != "ellipse" {
		t.Errorf("the overlay outline shows %q, want ellipse", got)
	}
	page.MustScreenshot(filepath.Join(shots, "mask-handles-turned-ellipse-1280x900.png"))
	press(input.Escape, false)
	waitForJSBool(t, page, `() => document.querySelector('[data-mask-box]').hidden`)

	// A title takes mask mode as well.
	if !page.MustEval(`() => window.VideoStudioParts.mutate(` + app + `, 'title fixture', project => { const clip = window.VideoStudioTimeline.clipFor(project, 'c1').clip; clip.text = 'Title'; clip.text_style = {font_family: 'Arial', font_size: 72, color: '#ffffff', alignment: 'center'}; })`).Bool() {
		t.Fatal("the title fixture was refused")
	}
	seek(30)
	enter()
	top := maskOf()["y"]
	page.MustElement(`[data-mask-shape]`).MustFocus()
	press(input.ArrowDown, false)
	requireMask("ArrowDown on a title's mask", map[string]float64{"y": top + 0.01}, 1e-6)
	press(input.Escape, false)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors in the mask handles part: %s", errs)
	}
}
