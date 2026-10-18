package ui

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestDesktopVideoStudioAnimBrowser drives the 2e animation and transition UI in the real app on the keyframe fixture
// with the default launcher. The controls are DOM, so either painter works. Subtests: inspector (Task 15); Task 16 adds
// picker and Task 17 library.
func TestDesktopVideoStudioAnimBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	t.Run("inspector", videoStudioAnimInspectorPart)
}

// videoStudioAnimControls is what one side of the Animation section shows.
type videoStudioAnimControls struct{ Preset, Disabled, Min, Max, Value, Output, Hint string }

// videoStudioAnimInspectorPart (Task 15) works on c1, the 512x288 still at half size on overlay o1. The clip covers
// frames 0-149 at 30 fps: lengths 6..90 frames, 15 by default, and half the clip is 75 frames.
//   - Without animations both lists show None, and both sliders are disabled at 0.5 s. The still has no speed ramp.
//   - Choosing Pop and Slide left sets each side at the default length, one undo step each, and the list keeps focus.
//   - A slider drag (input events, then change) is one undo step.
//   - 90 frames shows the hint for the 2.5 s the render plays; undo returns to the dragged length.
//   - A title keeps the section, and the narrow drawer holds it without horizontal scrolling.
//   - Screenshots for review.
func videoStudioAnimInspectorPart(t *testing.T) {
	page := openVideoStudioKeyframes(t, newSmokeBrowser(t), newVideoStudioKeyframesServer(t, videoStudioKeyframesProject()))
	shots := videoStudioKeyframesShotDir(t)
	const app = `window.VideoStudioApp.instances.get('kf')`
	const inList, outList = `[data-inspector] select[data-anim-preset="in"]`, `[data-inspector] select[data-anim-preset="out"]`
	const inLength = `[data-inspector] input[data-anim-duration="in"]`
	history := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.history.length`).Int() }
	change := func(selector, value string) {
		t.Helper()
		page.MustEval(`(sel, value) => { const el = document.querySelector(sel); if (!el) throw new Error('missing ' + sel); el.value = value; el.dispatchEvent(new Event('change', { bubbles: true })); }`, selector, value)
	}
	anim := func(side string) (string, int) {
		t.Helper()
		m, _ := effectsClip(t, page)["anim_"+side].(map[string]any)
		if m == nil {
			return "", 0
		}
		preset, _ := m["preset"].(string)
		frames, _ := m["duration"].(float64)
		return preset, int(frames)
	}
	controls := func(side string) videoStudioAnimControls {
		t.Helper()
		raw := page.MustEval(`side => { const q = sel => document.querySelector('[data-inspector] ' + sel); const list = q('[data-anim-preset="' + side + '"]'), range = q('[data-anim-duration="' + side + '"]'), hint = q('[data-anim-clamped="' + side + '"]'); if (!list || !range || !hint) return 'null'; return JSON.stringify({preset: list.value, disabled: String(range.disabled), min: range.min, max: range.max, value: range.value, output: range.parentElement.querySelector('output').textContent, hint: hint.hidden ? '' : hint.textContent.trim()}); }`, side).Str()
		var c videoStudioAnimControls
		if err := json.Unmarshal([]byte(raw), &c); err != nil || raw == "null" {
			t.Fatalf("the %s row is missing or unreadable: %v (%s)", side, err, raw)
		}
		return c
	}

	page.MustEval(`() => { const s = ` + app + `; s.timelineOptions.onSelect('c1'); s.preview.pause(); s.preview.seek(30); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('`+inList+`')`)
	for _, side := range []string{"in", "out"} {
		if got, want := controls(side), (videoStudioAnimControls{Preset: "none", Disabled: "true", Min: "6", Max: "90", Value: "15", Output: "0.5 s"}); got != want {
			t.Fatalf("%s side without animation: %+v, want %+v", side, got, want)
		}
	}
	if page.MustEval(`() => !!document.querySelector('[data-inspector] [data-ramp-preset]')`).Bool() {
		t.Error("the still on an overlay track shows speed ramp presets")
	}

	// Pop and Slide left: one undo step each at the default length; the list keeps focus.
	steps := history()
	change(inList, "pop")
	if preset, frames := anim("in"); preset != "pop" || frames != 15 {
		t.Fatalf("anim_in after choosing Pop: %s %d, want pop 15", preset, frames)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("choosing Pop made %d undo steps, want 1", got)
	}
	if !effectsFocused(page, inList) {
		t.Error("the in list lost focus after the change")
	}
	change(outList, "slide_left")
	if preset, frames := anim("out"); preset != "slide_left" || frames != 15 {
		t.Fatalf("anim_out after choosing Slide left: %s %d, want slide_left 15", preset, frames)
	}
	if got := controls("in"); got.Preset != "pop" || got.Disabled != "false" {
		t.Errorf("the in row after the change: %+v", got)
	}

	// A drag of the in length: input events change the project live, the change event ends the gesture: one undo step.
	// While the thumb passes half the clip the hint and the output follow without a re-render.
	steps = history()
	page.MustEval(`sel => { const input = document.querySelector(sel); input.value = '90'; input.dispatchEvent(new Event('input', { bubbles: true })); }`, inLength)
	if got := controls("in"); got.Output != "3 s" || got.Hint != "This clip is short: the animation plays over 2.5 s, half the clip." {
		t.Errorf("the in row while dragging to 90: %+v, want output 3 s and the clamp hint", got)
	}
	page.MustEval(`sel => { const input = document.querySelector(sel); for (const value of ['30', '45', '60']) { input.value = value; input.dispatchEvent(new Event('input', { bubbles: true })); } input.dispatchEvent(new Event('change', { bubbles: true })); }`, inLength)
	if _, frames := anim("in"); frames != 60 {
		t.Fatalf("anim_in length after the drag: %d, want 60", frames)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the slider gesture made %d undo steps, want 1", got)
	}
	if got := controls("in"); got.Output != "2 s" || got.Hint != "" {
		t.Errorf("the in row after the drag: %+v, want output 2 s and no hint", got)
	}

	// 90 frames is more than half of the 150-frame clip: the stored length stays 90 and the hint names the 2.5 s the
	// render plays.
	change(inLength, "90")
	if _, frames := anim("in"); frames != 90 {
		t.Fatalf("anim_in length after 90: %d", frames)
	}
	waitForJSBool(t, page, `() => !document.querySelector('[data-inspector] [data-anim-clamped="in"]').hidden`)
	if got := controls("in").Hint; got != "This clip is short: the animation plays over 2.5 s, half the clip." {
		t.Errorf("clamp hint %q", got)
	}
	page.MustEval(`() => document.querySelector('[data-inspector] .vs-anim-section').scrollIntoView({ block: 'center' })`)
	page.MustScreenshot(filepath.Join(shots, "anim-inspector-1280x900.png"))
	// Undo returns to the dragged 60 frames in one step.
	page.MustEval(`() => window.VideoStudioParts.undo(` + app + `)`)
	waitForJSBool(t, page, `() => document.querySelector('`+inLength+`').value === '60'`)
	if _, frames := anim("in"); frames != 60 {
		t.Errorf("anim_in length after undo: %d, want 60", frames)
	}
	if got := controls("in").Hint; got != "" {
		t.Errorf("the hint outlived the undo: %q", got)
	}

	// A title keeps the Animation section; a preset change keeps the length.
	if !page.MustEval(`() => window.VideoStudioParts.mutate(` + app + `, 'title fixture', project => { const clip = window.VideoStudioTimeline.clipFor(project, 'c1').clip; clip.text = 'Title'; clip.text_style = {font_family: 'Arial', font_size: 72, color: '#ffffff', alignment: 'center'}; })`).Bool() {
		t.Fatal("the title fixture was refused")
	}
	waitForJSBool(t, page, `() => !!document.querySelector('`+outList+`')`)
	change(outList, "fade")
	if preset, frames := anim("out"); preset != "fade" || frames != 15 {
		t.Fatalf("the title's anim_out after choosing Fade: %s %d, want fade 15", preset, frames)
	}
	page.MustEval(`() => document.querySelector('[data-inspector] .vs-anim-section').scrollIntoView({ block: 'center' })`)
	page.MustScreenshot(filepath.Join(shots, "anim-inspector-title-1280x900.png"))

	// The narrow window: the drawer holds the Animation section without horizontal scrolling.
	page.MustSetViewport(900, 900, 1, false)
	page.MustEval(`() => ` + app + `.timelineOptions.onSelect('c1')`)
	waitForJSBool(t, page, `() => document.querySelector('.vs-app').classList.contains('vs-show-inspector')`)
	page.MustEval(`() => document.querySelector('[data-inspector] .vs-anim-section').scrollIntoView({ block: 'center' })`)
	if !page.MustEval(`() => { const el = document.querySelector('.vs-inspector'); return el.scrollWidth <= el.clientWidth + 1; }`).Bool() {
		t.Error("the narrow inspector scrolls sideways")
	}
	page.MustScreenshot(filepath.Join(shots, "anim-inspector-900x900.png"))
	page.MustSetViewport(1280, 900, 1, false)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors in the animation inspector part: %s", errs)
	}
}
