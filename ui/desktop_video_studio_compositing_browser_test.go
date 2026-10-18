package ui

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopVideoStudioCompositingBrowser drives the 2d compositing UI in the real app on the keyframe fixture with the
// default launcher. The controls and the mask overlay are DOM, so either painter works. Subtests: inspector (Task 14)
// and handles (Task 15).
func TestDesktopVideoStudioCompositingBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	t.Run("inspector", videoStudioCompositingInspectorPart)
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
