package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopVideoStudioAnimBrowser drives the 2e animation and transition UI in the real app on the keyframe fixture
// with the default launcher. The controls are DOM, so either painter works. Subtests: inspector (Task 15), picker
// (Task 16) and library (Task 17).
func TestDesktopVideoStudioAnimBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	t.Run("inspector", videoStudioAnimInspectorPart)
	t.Run("picker", videoStudioTransitionPickerPart)
	t.Run("library", videoStudioTransitionLibraryPart)
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
//   - 2f: en, de and el in the narrow drawer: [data-inspector] does not scroll sideways (the Picture section's chips wrap).
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
	if got := controls("in"); got.Output != "3 s" || got.Hint != "This clip is short: the animation plays over 2.5\xc2\xa0s, half the clip." {
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
	if got := controls("in").Hint; got != "This clip is short: the animation plays over 2.5\xc2\xa0s, half the clip." {
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
	// Greek in the narrow drawer (2f F1, plan index §4.26.9). With c1 back as a picture, the drawer shows the whole
	// Picture section (crop, Full frame, flips, blend, mask).
	//   - The element that scrolls is [data-inspector] (.vs-inspector-body). .vs-inspector itself cannot overflow,
	//     because the body clips its content. Both are measured.
	//   - The elements whose right edge passes the body's content edge are logged for every locale and listed on
	//     failure. This is the measurement that decides the CSS fix.
	//   - A screenshot of the mask block per locale.
	if !page.MustEval(`() => window.VideoStudioParts.mutate(` + app + `, 'picture fixture', project => { const clip = window.VideoStudioTimeline.clipFor(project, 'c1').clip; clip.text = ''; clip.text_style = null; })`).Bool() {
		t.Fatal("the picture fixture was refused")
	}
	type drawerFit struct {
		Body   [2]float64 `json:"body"`
		Drawer [2]float64 `json:"drawer"`
		Wide   []string   `json:"wide"`
	}
	for _, lang := range []string{"en", "de", "el"} {
		words := map[string]string{}
		for key, value := range readDesktopLocale(t, lang) {
			if strings.HasPrefix(key, "videoStudio.") {
				words[key] = value
			}
		}
		page.MustEval(`(words, lang) => { window.kfLocaleEnglish = window.kfLocaleEnglish || window.kfLocale; window.kfLocale = Object.assign({}, window.kfLocaleEnglish, words); document.documentElement.lang = lang; window.VideoStudioParts.renderInspector(`+app+`); }`, words, lang)
		waitForJSBool(t, page, `() => !!document.querySelector('[data-inspector] [data-action="mask-mode"]') && !!document.querySelector('[data-inspector] [data-action="crop-mode"]')`)
		raw := page.MustEval(`() => {
const body = document.querySelector('.vs-inspector-body'), drawer = document.querySelector('.vs-inspector'), box = body.getBoundingClientRect();
const edge = box.left + body.clientLeft + body.clientWidth - parseFloat(getComputedStyle(body).paddingRight);
const name = el => el.tagName.toLowerCase() + (typeof el.className === 'string' && el.className.trim() ? '.' + el.className.trim().split(/\s+/).join('.') : '');
const wide = Array.from(body.querySelectorAll('*')).filter(el => el.getClientRects().length && el.getBoundingClientRect().right > edge + 0.5).map(el => name(el) + ' +' + (el.getBoundingClientRect().right - edge).toFixed(1) + 'px "' + (el.textContent || '').trim().slice(0, 48) + '"');
return JSON.stringify({body: [body.scrollWidth, body.clientWidth], drawer: [drawer.scrollWidth, drawer.clientWidth], wide: wide.slice(0, 12)});
}`).Str()
		var fit drawerFit
		if err := json.Unmarshal([]byte(raw), &fit); err != nil {
			t.Fatalf("%s drawer measure: %v (%s)", lang, err, raw)
		}
		t.Logf("%s narrow drawer: body %v, drawer %v, past the content edge: %v", lang, fit.Body, fit.Drawer, fit.Wide)
		if fit.Body[0] > fit.Body[1]+1 || fit.Drawer[0] > fit.Drawer[1]+1 {
			t.Errorf("%s: the narrow inspector scrolls sideways (body scroll/client %v, drawer %v); past the content edge: %v", lang, fit.Body, fit.Drawer, fit.Wide)
		}
		page.MustEval(`() => document.querySelector('[data-inspector] .vs-mask').scrollIntoView({ block: 'center' })`)
		page.MustScreenshot(filepath.Join(shots, "inspector-picture-"+lang+"-900x900.png"))
	}
	page.MustEval(`() => { window.kfLocale = window.kfLocaleEnglish; document.documentElement.lang = 'en'; window.VideoStudioParts.renderInspector(` + app + `); }`)
	page.MustSetViewport(1280, 900, 1, false)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors in the animation inspector part: %s", errs)
	}
}

// videoStudioTransitionsProject is the keyframe fixture with three more pictures on the empty video track v1: tx1 and
// tx2 meet at frame 150, and tx3 follows tx2 after a gap of 30 frames (Tasks 16 and 17). c1 on overlay o1 is unchanged.
func videoStudioTransitionsProject() map[string]any {
	project := videoStudioKeyframesProject()
	picture := func(id string, start int) map[string]any {
		return map[string]any{"id": id, "asset_id": "art1", "start": start, "offset": 0, "duration": 150,
			"x": 0, "y": 0, "width": 1, "height": 1, "rotation": 0, "opacity": 1, "volume": 1,
			"fade_in": 0, "fade_out": 0, "fit": "contain", "text": "", "text_style": nil, "transition": nil}
	}
	for _, entry := range project["tracks"].([]any) {
		if lane := entry.(map[string]any); lane["id"] == "v1" {
			lane["clips"] = append(lane["clips"].([]any), picture("tx1", 0), picture("tx2", 150), picture("tx3", 330))
		}
	}
	return project
}

// videoStudioTransitionPickerPart (Task 16) works on tx1 and tx2 of videoStudioTransitionsProject (they meet at frame
// 150), with real clicks and keys:
//   - the summary is collapsed by default and shows None;
//   - a click opens the panel with focus on the pressed type, and the groups follow the registry;
//   - choosing Wipe from bottom applies it in one undo step with the default length and closes the panel. Focus returns
//     to the summary, which now shows the new type, and the timeline tooltip shows the registry label;
//   - Enter opens the panel again, and Esc closes it without a change, with focus back on the summary;
//   - an unavailable type is disabled with its title, and the current one stays enabled and pressed;
//   - in the narrow window the panel fits the drawer, and Esc closes the panel but not the drawer;
//   - it takes screenshots of the open panel at both widths.
//   - 2f: in de, da, no and it the shortened labels break between words only (a screenshot per locale).
//   - 2f: arrow keys, Home and End move focus; a choice from None on a short pair applies the clamped length.
func videoStudioTransitionPickerPart(t *testing.T) {
	page := openVideoStudioKeyframes(t, newSmokeBrowser(t), newVideoStudioKeyframesServer(t, videoStudioTransitionsProject()))
	shots := videoStudioKeyframesShotDir(t)
	const app = `window.VideoStudioApp.instances.get('kf')`
	const summary = `[data-inspector] [data-action="transition-picker"]`
	const panel = `[data-inspector] [data-tx-picker]`
	history := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.history.length`).Int() }
	transition := func() string {
		t.Helper()
		return page.MustEval(`() => { const tr = window.VideoStudioTimeline.clipFor(` + app + `.project, 'tx1').clip.transition; return tr ? tr.type + '/' + tr.duration : 'none'; }`).Str()
	}
	text := func(selector string) string {
		t.Helper()
		return page.MustEval(`sel => document.querySelector(sel).textContent.trim()`, selector).Str()
	}
	opened := func() { t.Helper(); waitForJSBool(t, page, `() => !!document.querySelector('`+panel+`')`) }
	closed := func() { t.Helper(); waitForJSBool(t, page, `() => !document.querySelector('`+panel+`')`) }

	page.MustEval(`() => { const s = ` + app + `; s.timelineOptions.onSelect('tx1'); s.preview.pause(); s.preview.seek(30); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('`+summary+`')`)

	// Collapsed by default, showing None.
	if got := page.MustEval(`sel => document.querySelector(sel).getAttribute('aria-expanded')`, summary).Str(); got != "false" {
		t.Fatalf("summary aria-expanded = %q before opening", got)
	}
	if got := text(summary); got != "None" {
		t.Fatalf("summary shows %q, want None", got)
	}
	if page.MustHas(panel) {
		t.Fatal("the picker is open before anyone asked")
	}

	// A click opens the panel; focus goes to the pressed type; the groups follow the registry.
	page.MustElement(summary).MustClick()
	opened()
	if !effectsFocused(page, `[data-tx-picker] [data-transition="none"]`) {
		t.Error("opening did not move focus to the pressed type")
	}
	if got := page.MustEval(`() => JSON.stringify(Array.from(document.querySelectorAll('[data-tx-picker] .vs-tx-group'), g => g.getAttribute('aria-label') + ':' + g.querySelectorAll('[data-transition]').length))`).Str(); got != `["Basic transitions:5","Wipes:9","Push and zoom:6","Shapes:4"]` {
		t.Fatalf("picker groups %s", got)
	}
	if n := page.MustEval(`() => document.querySelectorAll('[data-tx-picker] [data-transition] svg.vs-tx-icon').length`).Int(); n != 25 {
		t.Errorf("%d types carry an icon, want 25 (none and the 24 transitions)", n)
	}

	// Choosing a type applies it with the default length (half a second) in one undo step, closes the panel and returns
	// focus to the summary.
	steps := history()
	page.MustElement(`[data-tx-picker] [data-transition="wipeup"]`).MustClick()
	closed()
	if got := transition(); got != "wipeup/15" {
		t.Fatalf("tx1 transition after choosing Wipe from bottom: %s", got)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the choice made %d undo steps, want 1", got)
	}
	if !effectsFocused(page, summary) {
		t.Error("focus did not return to the summary after a choice")
	}
	if got := text(summary); got != "Wipe from bottom" {
		t.Errorf("summary shows %q after the choice", got)
	}
	if got := page.MustEval(`() => document.querySelector('.vs-clip[data-clip-id="tx1"] .vs-transition').getAttribute('title')`).Str(); got != "Wipe from bottom" {
		t.Errorf("timeline transition tooltip %q, want the registry label", got)
	}

	// Keyboard: Enter on the summary opens the panel on the pressed type; Esc closes it without a change.
	page.MustElement(summary).MustFocus()
	page.KeyActions().Type(input.Enter).MustDo()
	opened()
	if !effectsFocused(page, `[data-tx-picker] [data-transition="wipeup"]`) {
		t.Error("Enter did not open the panel on the pressed type")
	}
	steps = history()
	page.KeyActions().Type(input.Escape).MustDo()
	closed()
	if !effectsFocused(page, summary) || history() != steps || transition() != "wipeup/15" {
		t.Errorf("Esc: focus on summary %v, undo steps +%d, transition %s", effectsFocused(page, summary), history()-steps, transition())
	}

	// Arrow keys (2f F1, spec §8.6). Enter opens the panel on Wipe from bottom, then:
	//   - ArrowRight and ArrowLeft move in DOM order;
	//   - ArrowDown moves to the same column of the next row (the columns are the layout's), ArrowUp back;
	//   - Home and End reach None and the last type.
	// No key changes the project, and Esc still closes the panel onto the summary.
	focused := func() string {
		t.Helper()
		return page.MustEval(`() => { const el = document.activeElement; return el && el.matches('[data-tx-picker] [data-transition]') ? el.dataset.transition : ''; }`).Str()
	}
	press := func(key input.Key) { t.Helper(); page.KeyActions().Type(key).MustDo() }
	page.MustElement(summary).MustFocus()
	press(input.Enter)
	opened()
	steps = history()
	if got := focused(); got != "wipeup" {
		t.Fatalf("Enter opened the panel on %q, want wipeup", got)
	}
	for _, move := range []struct {
		key  input.Key
		name string
		want string
	}{{input.ArrowRight, "ArrowRight", "wipedown"}, {input.ArrowLeft, "ArrowLeft", "wipeup"}} {
		press(move.key)
		if got := focused(); got != move.want {
			t.Errorf("%s focused %q, want %q", move.name, got, move.want)
		}
	}
	start := page.MustEval(`() => JSON.stringify([document.activeElement.offsetLeft, document.activeElement.offsetTop])`).Str()
	press(input.ArrowDown)
	if !page.MustEval(`start => { const [left, top] = JSON.parse(start), el = document.activeElement, group = el && el.closest('.vs-tx-group'); return !!group && group.getAttribute('aria-label') === 'Wipes' && el.offsetLeft === left && el.offsetTop > top; }`, start).Bool() {
		t.Errorf("ArrowDown from Wipe from bottom focused %q, want the same column one row down in Wipes", focused())
	}
	page.MustEval(`() => document.activeElement.scrollIntoView({ block: 'center' })`)
	page.MustScreenshot(filepath.Join(shots, "transition-picker-keys-1280x900.png"))
	press(input.ArrowUp)
	if got := focused(); got != "wipeup" {
		t.Errorf("ArrowUp focused %q, want wipeup back", got)
	}
	press(input.Home)
	if got := focused(); got != "none" {
		t.Errorf("Home focused %q, want none", got)
	}
	press(input.End)
	if got := focused(); got != "rectcrop" {
		t.Errorf("End focused %q, want rectcrop (the last type)", got)
	}
	if history() != steps || transition() != "wipeup/15" {
		t.Errorf("the arrow keys changed the project: undo steps +%d, transition %s", history()-steps, transition())
	}
	press(input.Escape)
	closed()
	if !effectsFocused(page, summary) {
		t.Error("Esc after the arrow keys did not return focus to the summary")
	}

	// Unavailable types are disabled with a title; the current one stays enabled and pressed.
	page.MustEval(`() => { window.VideoStudioEffects.adopt(window.VideoStudioEffects.registry(), {effects: [], transitions: ['fadegrays', 'wipeup'], blend_modes: [], anim_presets: []}); window.VideoStudioParts.renderInspector(` + app + `); }`)
	page.MustElement(summary).MustClick()
	opened()
	state := page.MustEval(`() => { const b = id => document.querySelector('[data-tx-picker] [data-transition="' + id + '"]'); return JSON.stringify([b('fadegrays').disabled, b('fadegrays').title, b('wipeup').disabled, b('wipeup').getAttribute('aria-pressed'), b('wipeup').title, b('fadewhite').disabled, b('fadewhite').title]); }`).Str()
	if want := `[true,"Not available: this server's FFmpeg lacks this transition",false,"true","Not available: this server's FFmpeg lacks this transition",false,""]`; state != want {
		t.Errorf("unavailable handling %s, want %s", state, want)
	}
	page.MustEval(`() => document.querySelector('[data-tx-picker]').scrollIntoView({ block: 'center' })`)
	page.MustScreenshot(filepath.Join(shots, "transition-picker-1280x900.png"))
	page.MustEval(`() => window.VideoStudioEffects.adopt(window.VideoStudioEffects.registry(), {effects: [], transitions: [], blend_modes: [], anim_presets: []})`)
	page.KeyActions().Type(input.Escape).MustDo()
	closed()

	// The narrow window: the panel fits the drawer; Esc closes the panel, and the drawer stays open.
	page.MustSetViewport(900, 900, 1, false)
	page.MustEval(`() => ` + app + `.timelineOptions.onSelect('tx1')`)
	waitForJSBool(t, page, `() => document.querySelector('.vs-app').classList.contains('vs-show-inspector')`)
	page.MustEval(`sel => document.querySelector(sel).click()`, summary)
	opened()
	if !page.MustEval(`() => { const el = document.querySelector('.vs-inspector'), list = document.querySelector('[data-tx-picker]'); return el.scrollWidth <= el.clientWidth + 1 && list.scrollWidth <= list.clientWidth + 1; }`).Bool() {
		t.Error("the open picker scrolls sideways in the narrow drawer")
	}
	page.MustEval(`() => document.querySelector('[data-tx-picker]').scrollIntoView({ block: 'center' })`)
	page.MustScreenshot(filepath.Join(shots, "transition-picker-900x900.png"))
	// Shortened labels (2f F1, plan index §4.26.9), in the 900 px drawer, for de, da, no and it:
	//   - The four changed types (Clock wipe, Squeeze vertically, Dissolve, Pixelate) must break between words only:
	//     a Range over each word of their label has one client rect. Words broken in other labels are logged for the
	//     screenshot review.
	//   - [data-inspector] (.vs-inspector-body, the element that scrolls) must not scroll sideways.
	//   - The English texts come back afterwards.
	for _, lang := range []string{"de", "da", "no", "it"} {
		words := map[string]string{}
		for key, value := range readDesktopLocale(t, lang) {
			if strings.HasPrefix(key, "videoStudio.") {
				words[key] = value
			}
		}
		page.MustEval(`(words, lang) => { window.kfLocaleEnglish = window.kfLocaleEnglish || window.kfLocale; window.kfLocale = Object.assign({}, window.kfLocaleEnglish, words); document.documentElement.lang = lang; }`, words, lang)
		page.MustEval(`() => window.VideoStudioParts.setTransitionPicker(` + app + `, true)`)
		opened()
		raw := page.MustEval(`() => {
const changed = ['radial', 'squeezeh', 'dissolve', 'pixelize'], out = {changed: [], other: []};
document.querySelectorAll('[data-tx-picker] [data-transition]').forEach(button => {
  const span = button.querySelector('span'), node = span && span.firstChild;
  if (!node || node.nodeType !== 3) return;
  const text = node.textContent;
  let from = 0;
  for (const word of text.split(/[\s-]+/)) {
    if (!word) continue;
    const start = text.indexOf(word, from);
    from = start + word.length;
    const range = document.createRange();
    range.setStart(node, start);
    range.setEnd(node, start + word.length);
    if (range.getClientRects().length > 1) out[changed.includes(button.dataset.transition) ? 'changed' : 'other'].push(word);
  }
});
return JSON.stringify(out);
}`).Str()
		var broken struct {
			Changed []string `json:"changed"`
			Other   []string `json:"other"`
		}
		if err := json.Unmarshal([]byte(raw), &broken); err != nil {
			t.Fatalf("%s: word check: %v (%s)", lang, err, raw)
		}
		if len(broken.Changed) > 0 {
			t.Errorf("%s: shortened picker labels still break inside a word in the 900 px drawer: %v", lang, broken.Changed)
		}
		if len(broken.Other) > 0 {
			t.Logf("%s: other picker words broken inside themselves (check the screenshot): %v", lang, broken.Other)
		}
		if !page.MustEval(`() => { const body = document.querySelector('.vs-inspector-body'); return body.scrollWidth <= body.clientWidth + 1; }`).Bool() {
			t.Errorf("%s: the open picker makes the narrow drawer scroll sideways", lang)
		}
		page.MustEval(`() => document.querySelector('[data-tx-picker]').scrollIntoView({ block: 'center' })`)
		page.MustScreenshot(filepath.Join(shots, "transition-picker-"+lang+"-900x900.png"))
	}
	page.MustEval(`() => { window.kfLocale = window.kfLocaleEnglish; document.documentElement.lang = 'en'; }`)
	page.MustEval(`() => window.VideoStudioParts.setTransitionPicker(` + app + `, true)`)
	opened()
	page.KeyActions().Type(input.Escape).MustDo()
	closed()
	if !page.MustEval(`() => document.querySelector('.vs-app').classList.contains('vs-show-inspector')`).Bool() {
		t.Error("Esc in the picker also closed the drawer")
	}
	page.MustSetViewport(1280, 900, 1, false)
	// From None on a short pair (2f F1). With tx1 and tx2 at 12 frames each, the pair allows at most 11 frames. The
	// disabled slider already holds 11, and choosing Dissolve applies those 11 in one undo step. 2e sent half a second
	// (15 frames), which applyTransition refused as too long.
	if !page.MustEval(`() => window.VideoStudioParts.mutate(` + app + `, 'short pair', project => { const at = id => window.VideoStudioTimeline.clipFor(project, id).clip; at('tx1').duration = 12; at('tx1').transition = null; at('tx2').start = 12; at('tx2').duration = 12; })`).Bool() {
		t.Fatal("the short pair fixture was refused")
	}
	page.MustEval(`() => ` + app + `.timelineOptions.onSelect('tx1')`)
	waitForJSBool(t, page, `() => { const range = document.querySelector('[data-inspector] [data-transition-duration]'); return !!range && range.disabled && range.max === '11' && range.value === '11'; }`)
	steps = history()
	page.MustElement(summary).MustClick()
	opened()
	page.MustElement(`[data-tx-picker] [data-transition="dissolve"]`).MustClick()
	closed()
	if got := transition(); got != "dissolve/11" {
		t.Errorf("tx1 after choosing Dissolve from None on the short pair: %s, want dissolve/11", got)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the choice on the short pair made %d undo steps, want 1", got)
	}
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors in the picker part: %s", errs)
	}
}

// videoStudioTransitionLibraryPart (Task 17) uses synthetic drag events, as in the 2c library part. It works on the
// video-track pictures of videoStudioTransitionsProject: tx1 and tx2 meet at frame 150, and tx3 follows after a gap of
// 30 frames.
//   - The transition groups follow the effect groups in the Effects tab.
//   - Dragging Wipe from bottom over the right half of tx1 marks the cut tx1|tx2. The drop sets that cut with the
//     default length in one undo step and selects tx1.
//   - Circle opens, dropped on the left half of tx2 (the same cut, now an overlap), replaces the type and keeps the
//     length.
//   - A drop on the right half of tx2 (a gap follows) marks nothing, changes nothing and explains the cut.
//   - A double-click on Through white sets the selected clip's cut. Enter on it with the last clip selected gives the
//     transitionNeedsNext notice.
//   - An unavailable item is aria-disabled with its reason and changes nothing.
//   - The 2c effect drop still works (Color onto tx1).
//   - Screenshots show the cut mark on a plain cut and, with the tab at the effects/transitions boundary and at the wipes, on an unselected clip.
//   - 2f: no item name is cut at 1280 px (two columns) or at 1100 px (the narrow rail).
func videoStudioTransitionLibraryPart(t *testing.T) {
	page := openVideoStudioKeyframes(t, newSmokeBrowser(t), newVideoStudioKeyframesServer(t, videoStudioTransitionsProject()))
	shots := videoStudioKeyframesShotDir(t)
	const app = `window.VideoStudioApp.instances.get('kf')`
	history := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.history.length`).Int() }
	// transition is "<type>/<duration>" of a clip's transition, or "none".
	transition := func(id string) string {
		t.Helper()
		return page.MustEval(`id => { const tr = window.VideoStudioTimeline.clipFor(`+app+`.project, id).clip.transition; return tr ? tr.type + '/' + tr.duration : 'none'; }`, id).Str()
	}
	start := func(id string) int {
		t.Helper()
		return page.MustEval(`id => window.VideoStudioTimeline.clipFor(`+app+`.project, id).clip.start`, id).Int()
	}
	notice := func() string {
		t.Helper()
		return page.MustEval(`() => { const el = document.querySelector('[data-notice]'); return el && !el.hidden ? (el.querySelector('[data-notice-text]') || el).textContent.trim() : ''; }`).Str()
	}
	clearNotice := func() { t.Helper(); page.MustEval(`() => window.VideoStudioParts.clearNotice(` + app + `)`) }
	// drag fires dragstart on a library item and dragover at a fraction of a clip's width, then the drop when finish is
	// set. It returns the mark the dragover left on the clip: 'before', 'after', 'effect' or ''.
	drag := func(key, clipID string, fraction float64, finish bool) string {
		t.Helper()
		return page.MustEval(`(key, id, fraction, finish) => {
const clip = document.querySelector('.vs-clip[data-clip-id="' + id + '"]'), r = clip.getBoundingClientRect(), dt = new DataTransfer();
document.querySelector('[data-effects-library] [data-effect-item="' + key + '"]').dispatchEvent(new DragEvent('dragstart', {dataTransfer: dt, bubbles: true, cancelable: true}));
const at = {dataTransfer: dt, clientX: r.left + r.width * fraction, clientY: r.top + r.height / 2, bubbles: true, cancelable: true};
clip.dispatchEvent(new DragEvent('dragover', at));
const mark = clip.classList.contains('is-cut-drop-before') ? 'before' : clip.classList.contains('is-cut-drop-after') ? 'after' : clip.classList.contains('is-effect-drop') ? 'effect' : '';
if (finish) clip.dispatchEvent(new DragEvent('drop', at));
return mark;
}`, key, clipID, fraction, finish).Str()
	}
	dblclick := func(key string) {
		t.Helper()
		point := page.MustEval(`key => { const el = document.querySelector('[data-effects-library] [data-effect-item="' + key + '"]'); el.scrollIntoView({block: 'center'}); const r = el.getBoundingClientRect(); return {x: r.left + r.width / 2, y: r.top + r.height / 2}; }`, key)
		page.Mouse.MustMoveTo(point.Get("x").Num(), point.Get("y").Num())
		if err := page.Mouse.Click(proto.InputMouseButtonLeft, 2); err != nil {
			t.Fatal(err)
		}
	}

	page.MustElement(`[data-tab="effects"]`).MustClick()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-effects-library] [data-effect-item="transition:wipeup"][aria-disabled="false"]')`)
	if got := page.MustEval(`() => JSON.stringify(Array.from(document.querySelectorAll('[data-effects-library] .vs-fxlib-group h3'), h => h.textContent))`).Str(); !strings.HasSuffix(got, `"Basic transitions","Wipes","Push and zoom","Shapes"]`) {
		t.Fatalf("library groups %s: the transition groups must follow the effects", got)
	}
	if !page.MustHas(`[data-effects-library] [data-effect-item="transition:wipeup"] svg.vs-tx-icon`) {
		t.Error("transition items carry no generated icon")
	}

	// A plain cut under the pointer, before anything changes: a screenshot of the mark, and dragend clears it.
	if mark := drag("transition:wipeup", "tx1", 0.9, false); mark != "after" {
		t.Fatalf("hovering the right half of tx1 marked %q, want the cut after it", mark)
	}
	page.MustScreenshot(filepath.Join(shots, "transition-library-cut-1280x900.png"))
	page.MustEval(`() => window.dispatchEvent(new DragEvent('dragend'))`)

	// Drop on the right half of tx1: the cut tx1|tx2, the default length, one undo step, tx1 selected.
	steps := history()
	if mark := drag("transition:wipeup", "tx1", 0.9, true); mark != "after" {
		t.Fatalf("dragover on the right half of tx1 marked %q, want the cut after it", mark)
	}
	if got := transition("tx1"); got != "wipeup/15" {
		t.Fatalf("tx1 after the drop: %s", got)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the drop made %d undo steps, want 1", got)
	}
	if got := page.MustEval(`() => ` + app + `.selectedClipId`).Str(); got != "tx1" {
		t.Errorf("the drop selected %q, want the outgoing clip tx1", got)
	}
	if got := start("tx2"); got != 135 {
		t.Errorf("tx2 starts at %d, want 135 (the overlap of the inspector path)", got)
	}

	// The same cut from the left half of tx2 keeps an existing length (20 frames, set through the 2a path).
	page.MustEval(`() => window.VideoStudioParts.applyTransition(` + app + `, 'wipeup', 20, 'tx1')`)
	steps = history()
	if mark := drag("transition:circleopen", "tx2", 0.25, true); mark != "before" {
		t.Fatalf("dragover on the left half of tx2 marked %q, want the cut before it", mark)
	}
	if got := transition("tx1"); got != "circleopen/20" {
		t.Fatalf("tx1 after the second drop: %s (an existing transition keeps its length)", got)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the second drop made %d undo steps, want 1", got)
	}

	// The right half of tx2: a gap follows, so there is no cut. Nothing is marked, nothing changes, and a notice explains.
	clearNotice()
	steps = history()
	if mark := drag("transition:fadewhite", "tx2", 0.9, true); mark != "" {
		t.Errorf("a clip followed by a gap was marked %q", mark)
	}
	if transition("tx2") != "none" || history() != steps {
		t.Fatalf("a drop at a gap changed the project: tx2 %s, undo steps +%d", transition("tx2"), history()-steps)
	}
	if got := notice(); got != "Drop a transition on the cut between two clips on one track." {
		t.Errorf("gap drop notice %q", got)
	}

	// A double-click sets the cut after the selected clip (tx1, selected by the drops) and keeps its length.
	steps = history()
	dblclick("transition:fadewhite")
	if got := transition("tx1"); got != "fadewhite/20" {
		t.Fatalf("tx1 after the double-click: %s", got)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the double-click made %d undo steps, want 1", got)
	}
	// Enter on a focused item with the last clip selected: no following clip, the inspector's notice, no change.
	page.MustEval(`() => ` + app + `.timelineOptions.onSelect('tx3')`)
	clearNotice()
	steps = history()
	page.MustElement(`[data-effects-library] [data-effect-item="transition:fadewhite"]`).MustFocus()
	page.KeyActions().Type(input.Enter).MustDo()
	if got := notice(); got != "Add a following clip on this track first." {
		t.Errorf("Enter without a following clip: notice %q", got)
	}
	if transition("tx3") != "none" || history() != steps {
		t.Errorf("Enter without a following clip changed the project")
	}

	// An unavailable transition is aria-disabled with its reason, and a double-click only explains.
	page.MustEval(`() => { window.VideoStudioEffects.adopt(window.VideoStudioEffects.registry(), {effects: [], transitions: ['radial'], blend_modes: [], anim_presets: []}); window.VideoStudioParts.renderEffectsLibrary(` + app + `); }`)
	radial := `[data-effects-library] [data-effect-item="transition:radial"]`
	if got := page.MustEval(`sel => { const el = document.querySelector(sel); return el.getAttribute('aria-disabled') + '|' + el.getAttribute('draggable') + '|' + el.title; }`, radial).Str(); got != "true|false|Clock wipe (Not available: this server's FFmpeg lacks this transition)" {
		t.Errorf("unavailable item %q", got)
	}
	page.MustEval(`() => ` + app + `.timelineOptions.onSelect('tx1')`)
	clearNotice()
	steps = history()
	dblclick("transition:radial")
	if got := notice(); got != "Not available: this server's FFmpeg lacks this transition" || history() != steps {
		t.Errorf("double-click on an unavailable transition: notice %q, undo steps +%d", got, history()-steps)
	}
	page.MustEval(`() => { window.VideoStudioEffects.adopt(window.VideoStudioEffects.registry(), {effects: [], transitions: [], blend_modes: [], anim_presets: []}); window.VideoStudioParts.renderEffectsLibrary(` + app + `); }`)

	// The 2c effect drop still copies an effect onto the clip under the pointer.
	if mark := drag("color", "tx1", 0.5, true); mark != "effect" {
		t.Errorf("an effect drag over tx1 marked %q, want the clip", mark)
	}
	if got := page.MustEval(`() => JSON.stringify((window.VideoStudioTimeline.clipFor(` + app + `.project, 'tx1').clip.effects || []).map(entry => entry.id))`).Str(); got != `["color"]` {
		t.Errorf("tx1 effects after the colour drop: %s", got)
	}

	// Screenshots with a cut mark on an unselected clip: the tab where the effects end and the transitions begin, and
	// the wipes below. dragend clears the mark.
	page.MustEval(`() => { window.VideoStudioParts.setSelection(` + app + `, ['tx3'], 'tx3'); window.VideoStudioParts.renderTimeline(` + app + `); }`)
	clearNotice()
	if mark := drag("transition:slideleft", "tx1", 0.9, false); mark != "after" {
		t.Errorf("screenshot drag marked %q", mark)
	}
	page.MustEval(`() => document.querySelector('[data-effects-library] h3[id$="-key"]').scrollIntoView({ block: 'start' })`)
	page.MustScreenshot(filepath.Join(shots, "transition-library-1280x900.png"))
	page.MustEval(`() => document.querySelector('[data-effects-library] h3[id$="-tx-wipe"]').scrollIntoView({ block: 'start' })`)
	page.MustScreenshot(filepath.Join(shots, "transition-library-wipes-1280x900.png"))
	page.MustEval(`() => window.dispatchEvent(new DragEvent('dragend'))`)
	if page.MustHas(`.vs-clip.is-cut-drop-before, .vs-clip.is-cut-drop-after`) {
		t.Error("dragend left a cut mark")
	}
	// Item names wrap instead of ending in an ellipsis (2f F1; 2c follow-up 2). The cut showed in the two-column list of
	// the 288 px library at 1280 px ("Teal and orange", "Black and white", "Bleach bypass"). The narrow rail at 1100 px
	// is checked too. A cut name has scrollWidth > clientWidth. A screenshot at each width.
	for _, view := range []struct {
		width int
		shot  string
	}{{1280, "effects-library-rail-1280x900.png"}, {1100, "effects-library-rail-1100x900.png"}} {
		page.MustSetViewport(view.width, 900, 1, false)
		page.MustEval(`() => document.querySelector('[data-effects-library] .vs-fxlib-group').scrollIntoView({ block: 'start' })`)
		if cut := page.MustEval(`() => JSON.stringify(Array.from(document.querySelectorAll('[data-effects-library] .vs-fxlib-item span'), span => (span.scrollWidth > span.clientWidth + 1 ? span.textContent : '')).filter(Boolean))`).Str(); cut != "[]" {
			t.Errorf("library names cut at %d px: %s", view.width, cut)
		}
		page.MustScreenshot(filepath.Join(shots, view.shot))
	}
	page.MustSetViewport(1280, 900, 1, false)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors in the library part: %s", errs)
	}
}
