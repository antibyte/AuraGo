package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"

	"aurago/internal/videostudio"
)

// effectsPagePrelude finds the editor of the effects browser test ('kf', the keyframe fixture's window id) and adds a
// one-pixel capture of the settled preview at (canvas/8, canvas/8): inside clip c1, a quarter into its picture.
const effectsPagePrelude = `const s=window.VideoStudioApp.instances.get('kf'),P=window.VideoStudioParts,T=window.VideoStudioTimeline;
const capture=async()=>{s.preview.pause();s.preview.seek(s.frame);if(!await s.preview.settle(15000))throw new Error('the preview did not settle');const canvas=document.querySelector('[data-preview]');const shot=window.VideoStudioApp.capturePreview('kf',{x:Math.floor(canvas.width/8),y:Math.floor(canvas.height/8),width:1,height:1});return shot?Array.from(shot.data):[];};
`

// videoStudioBuiltinLUTRoute serves /api/desktop/video-studio/luts/builtin/<id>.cube from videostudio.BuiltinLookCube, like
// the server's route (index §4.20), so the fixture's preview can load the built-in looks.
func videoStudioBuiltinLUTRoute(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/desktop/video-studio/luts/builtin/"), ".cube")
	cube, ok := videostudio.BuiltinLookCube(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(cube)
}

// effectsCapture is the preview's RGBA pixel at (canvas/8, canvas/8) once the preview has settled.
func effectsCapture(t *testing.T, page *rod.Page) [4]int {
	t.Helper()
	values := page.MustEval(`async () => {` + effectsPagePrelude + `return await capture(); }`).Arr()
	var out [4]int
	if len(values) != 4 {
		t.Fatalf("the capture returned %d values", len(values))
	}
	for i, value := range values {
		out[i] = value.Int()
	}
	return out
}

// effectsDiff is the summed absolute RGB difference of two captures.
func effectsDiff(a, b [4]int) int {
	sum := 0
	for i := 0; i < 3; i++ {
		sum += int(math.Abs(float64(a[i] - b[i])))
	}
	return sum
}

// effectsEntry is one effect entry of clip c1 as the app holds it.
type effectsEntry struct {
	ID        string                           `json:"id"`
	On        bool                             `json:"on"`
	Params    map[string]any                   `json:"params"`
	Keyframes map[string][]videoStudioKeyframe `json:"keyframes"`
}

// effectsClip is clip c1 of the open fixture.
func effectsClip(t *testing.T, page *rod.Page) map[string]any {
	t.Helper()
	raw := page.MustEval(`() => { const found = window.VideoStudioTimeline.clipFor(window.VideoStudioApp.instances.get('kf').project, 'c1'); return JSON.stringify(found ? found.clip : null); }`).Str()
	var clip map[string]any
	if err := json.Unmarshal([]byte(raw), &clip); err != nil || clip == nil {
		t.Fatalf("clip c1: %v (%s)", err, raw)
	}
	return clip
}

// effectsStack is c1's effect stack.
func effectsStack(t *testing.T, page *rod.Page) []effectsEntry {
	t.Helper()
	raw, _ := json.Marshal(effectsClip(t, page)["effects"])
	var stack []effectsEntry
	if err := json.Unmarshal(raw, &stack); err != nil {
		t.Fatalf("effects of c1: %v (%s)", err, raw)
	}
	return stack
}

// effectsIDs lists the stack's effect ids, comma-separated.
func effectsIDs(stack []effectsEntry) string {
	ids := make([]string, len(stack))
	for i, entry := range stack {
		ids[i] = entry.ID
	}
	return strings.Join(ids, ",")
}

// effectsFocused reports whether the focused element is the one selector names.
func effectsFocused(page *rod.Page, selector string) bool {
	return page.MustEval(`sel => document.activeElement === document.querySelector(sel)`, selector).Bool()
}

// effectsAdd adds an effect to the selected clip through "+ Effect" and its menu, with real clicks.
func effectsAdd(t *testing.T, page *rod.Page, id string) {
	t.Helper()
	count := len(effectsStack(t, page))
	page.MustElement(`[data-inspector] [data-fx-action="add"]`).MustClick()
	page.MustElement(`.vs-context-menu [data-fx-add="` + id + `"]`).MustClick()
	waitForJSBool(t, page, fmt.Sprintf(`() => document.querySelectorAll('[data-inspector] [data-fx-entry]').length === %d`, count+1))
}

// TestDesktopVideoStudioEffectsBrowser drives the 2c effect stack in the real app under the SwiftShader launcher, so the
// GL painter draws the effects. Part 1 (Task 14) is effectsStackBrowserPart; Tasks 15 to 17 append parts 2 to 4.
func TestDesktopVideoStudioEffectsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := newVideoStudioKeyframesServerWith(t, videoStudioKeyframesProject(), map[string]http.HandlerFunc{"/api/desktop/video-studio/luts/builtin/": videoStudioBuiltinLUTRoute})
	page := openVideoStudioKeyframes(t, newWebGLSmokeBrowser(t), server)
	shots := videoStudioKeyframesShotDir(t)
	effectsStackBrowserPart(t, page, shots)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors after the effect stack: %s", errs)
	}
	colorWheelBrowserPart(t, page, shots)
	if errs := page.MustEval(`() => JSON.stringify(window.kfErrors)`).Str(); errs != "[]" {
		t.Fatalf("script errors after the colour wheels: %s", errs)
	}
}

// Part 1 of TestDesktopVideoStudioEffectsBrowser (Task 14), with real clicks and keys:
//   - "+ Effect" adds Color and its header takes focus; exposure +1 EV brightens the GL capture; the switch turns it off.
//   - A vignette moves up and back with Alt+ArrowUp / Alt+ArrowDown on its header (focus follows it, the playhead stays),
//     then is removed.
//   - Flip horizontally mirrors the picture in the capture, a crop field sets crop.l, and a built-in look recolours it.
//   - The save carries crop, flip and the stack; the timeline stays valid.
func effectsStackBrowserPart(t *testing.T, page *rod.Page, shots string) {
	t.Helper()
	const app = `window.VideoStudioApp.instances.get('kf')`
	entry := func(index int, selector string) string {
		return fmt.Sprintf(`[data-inspector] [data-fx-entry="%d"] %s`, index, selector)
	}
	attr := func(selector, name string) string {
		t.Helper()
		return page.MustEval(`(sel, name) => { const el = document.querySelector(sel); return el ? String(el.getAttribute(name)) : 'missing'; }`, selector, name).Str()
	}
	page.MustEval(`() => { const s = ` + app + `; s.timelineOptions.onSelect('c1'); s.preview.pause(); s.preview.seek(0); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-inspector] [data-fx-section] [data-fx-action="add"]')`)
	if painter := page.MustEval(`() => ` + app + `.preview.painterKind()`).Str(); painter != "gl" {
		t.Fatalf("the SwiftShader launcher must draw with GL, got %q", painter)
	}
	history := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.history.length`).Int() }
	base := effectsCapture(t, page)

	// Add Color through the menu; the new entry's header takes focus.
	effectsAdd(t, page, "color")
	if got := effectsIDs(effectsStack(t, page)); got != "color" {
		t.Fatalf("stack after adding Color: %q", got)
	}
	if !effectsFocused(page, entry(0, "[data-fx-head]")) {
		t.Error("the new entry's header did not take focus")
	}

	// Exposure +1 EV: one undo step, the output text and a brighter GL capture.
	steps := history()
	page.MustEval(`sel => { const input = document.querySelector(sel); input.value = '1'; input.dispatchEvent(new Event('input', { bubbles: true })); input.dispatchEvent(new Event('change', { bubbles: true })); }`, entry(0, `input[data-fx-param="exposure"]`))
	if got := effectsStack(t, page)[0].Params["exposure"]; got != 1.0 {
		t.Fatalf("exposure = %v, want 1", got)
	}
	if got := history() - steps; got != 1 {
		t.Errorf("the slider gesture made %d undo steps, want 1", got)
	}
	if got := page.MustEval(`sel => document.querySelector(sel).parentElement.querySelector('output').textContent`, entry(0, `input[data-fx-param="exposure"]`)).Str(); got != "+1.00 EV" {
		t.Errorf("exposure output %q, want +1.00 EV", got)
	}
	bright := effectsCapture(t, page)
	if (bright[0]+bright[1]+bright[2])-(base[0]+base[1]+base[2]) < 120 {
		t.Fatalf("exposure +1 EV did not brighten the GL capture: %v -> %v", base, bright)
	}

	// The switch turns the entry off: the capture is the plain picture again.
	page.MustElement(entry(0, `[data-fx-action="on"]`)).MustClick()
	if effectsStack(t, page)[0].On {
		t.Fatal("the switch did not turn Color off")
	}
	if got := attr(entry(0, `[data-fx-action="on"]`), "aria-checked"); got != "false" {
		t.Errorf("switch aria-checked = %q", got)
	}
	if off := effectsCapture(t, page); effectsDiff(off, base) > 12 {
		t.Fatalf("Color switched off still changes the picture: %v vs %v", off, base)
	}

	// Alt+ArrowUp / Alt+ArrowDown on an entry header move it; focus follows; the playhead stays.
	effectsAdd(t, page, "vignette")
	page.MustElement(entry(1, "[data-fx-head]")).MustFocus()
	page.KeyActions().Press(input.AltLeft).Type(input.ArrowUp).MustDo()
	if got := effectsIDs(effectsStack(t, page)); got != "vignette,color" {
		t.Fatalf("Alt+ArrowUp on the vignette header: %q", got)
	}
	if !effectsFocused(page, entry(0, "[data-fx-head]")) {
		t.Error("focus did not follow the moved entry up")
	}
	page.KeyActions().Press(input.AltLeft).Type(input.ArrowDown).MustDo()
	if got := effectsIDs(effectsStack(t, page)); got != "color,vignette" {
		t.Fatalf("Alt+ArrowDown on the vignette header: %q", got)
	}
	if !effectsFocused(page, entry(1, "[data-fx-head]")) {
		t.Error("focus did not follow the moved entry down")
	}
	if f := page.MustEval(`() => ` + app + `.frame`).Int(); f != 0 {
		t.Errorf("Alt+arrows on an entry header moved the playhead to %d", f)
	}

	// Remove the vignette; focus moves to the entry before it.
	page.MustElement(entry(1, `[data-fx-action="remove"]`)).MustClick()
	if got := effectsIDs(effectsStack(t, page)); got != "color" {
		t.Fatalf("stack after removing the vignette: %q", got)
	}
	if !effectsFocused(page, entry(0, "[data-fx-head]")) {
		t.Error("focus did not move to the remaining entry")
	}

	// Flip horizontally: the capture shows the other side of the picture (red grows with x in the fixture PNG).
	before := effectsCapture(t, page)
	page.MustElement(`[data-inspector] [data-flip="h"]`).MustClick()
	if effectsClip(t, page)["flip_h"] != true {
		t.Fatal("the flip toggle did not set flip_h")
	}
	if got := attr(`[data-inspector] [data-flip="h"]`, "aria-pressed"); got != "true" {
		t.Errorf("flip aria-pressed = %q", got)
	}
	if flipped := effectsCapture(t, page); math.Abs(float64(flipped[0]-before[0])) < 30 {
		t.Fatalf("flip_h did not mirror the picture: %v -> %v", before, flipped)
	}

	// The crop field takes percent of the source picture.
	page.MustEval(`() => { const input = document.querySelector('[data-inspector] [data-crop="l"]'); input.value = '10'; input.dispatchEvent(new Event('change', { bubbles: true })); }`)
	crop, _ := effectsClip(t, page)["crop"].(map[string]any)
	if crop == nil || math.Abs(crop["l"].(float64)-0.1) > 1e-9 || crop["t"] != 0.0 || crop["r"] != 0.0 || crop["b"] != 0.0 {
		t.Fatalf("crop after the left field: %v", crop)
	}

	// A built-in look recolours the picture (the preview loads its .cube from the fixture's route).
	effectsAdd(t, page, "lut")
	plain := effectsCapture(t, page)
	page.MustEval(`sel => { const select = document.querySelector(sel); select.value = 'builtin:teal_orange'; select.dispatchEvent(new Event('change', { bubbles: true })); }`, entry(1, `select[data-fx-param="lut"]`))
	if got := effectsStack(t, page)[1].Params["lut"]; got != "builtin:teal_orange" {
		t.Fatalf("lut = %v", got)
	}
	if look := effectsCapture(t, page); effectsDiff(look, plain) < 12 {
		t.Fatalf("the teal and orange look did not recolour the GL capture: %v -> %v", plain, look)
	}

	// The save carries crop, flip and the stack with complete params; the timeline stays valid.
	saved := page.MustEval(`async () => { const s = ` + app + `; await window.VideoStudioParts.saveUntilClean(s); const body = window.kfSaves[window.kfSaves.length - 1]; return JSON.stringify(body.tracks.flatMap(track => track.clips).find(clip => clip.id === 'c1')); }`).Str()
	var savedClip struct {
		Crop    map[string]float64 `json:"crop"`
		FlipH   bool               `json:"flip_h"`
		Effects []effectsEntry     `json:"effects"`
	}
	if err := json.Unmarshal([]byte(saved), &savedClip); err != nil {
		t.Fatalf("saved c1: %v (%s)", err, saved)
	}
	if !savedClip.FlipH || math.Abs(savedClip.Crop["l"]-0.1) > 1e-9 || effectsIDs(savedClip.Effects) != "color,lut" || len(savedClip.Effects[0].Params) != 14 || savedClip.Effects[0].On {
		t.Fatalf("the save does not carry the picture fields: %s", saved)
	}
	if !page.MustEval(`() => window.VideoStudioTimeline.validTimeline(` + app + `.project)`).Bool() {
		t.Error("the effect edits left an invalid timeline")
	}
	page.MustEval(`() => document.querySelector('[data-inspector] [data-fx-section]').scrollIntoView({ block: 'start' })`)
	page.MustScreenshot(filepath.Join(shots, "effects-inspector-1280x900.png"))
}

// Part 2 of TestDesktopVideoStudioEffectsBrowser (Task 15): the colour wheels of a fresh Color entry on c1.
//   - A real pointer drag moves the gain wheel's point toward orange (30°, 80 % of the disc). gain_x and gain_y follow,
//     the screen-reader text says so, and the GL capture warms (red up against blue).
//   - On the focused lift wheel, ArrowRight three times and Shift+ArrowUp give (0.03, 0.1), one undo step each.
//   - Delete resets the wheel and leaves the clip in place; a double-click resets the gain wheel.
func colorWheelBrowserPart(t *testing.T, page *rod.Page, shots string) {
	t.Helper()
	const app = `window.VideoStudioApp.instances.get('kf')`
	wheel := func(name string) string { return `[data-inspector] [data-fx-entry="0"] [data-wheel="` + name + `"]` }
	page.MustEval(`() => {` + effectsPagePrelude + `
		if (!P.mutate(s, 'wheel fixture', project => { const clip = T.clipFor(project, 'c1').clip; clip.effects = [{id: 'color', on: true, params: window.VideoStudioEffects.defaults('color')}]; delete clip.crop; delete clip.flip_h; })) throw new Error('the wheel fixture was refused');
		P.setSelection(s, ['c1'], 'c1'); s.preview.pause(); s.preview.seek(0); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('`+wheel("gain")+`')`)
	params := func() map[string]any {
		t.Helper()
		stack := effectsStack(t, page)
		if len(stack) != 1 || stack[0].ID != "color" {
			t.Fatalf("the wheel fixture's stack changed: %q", effectsIDs(stack))
		}
		return stack[0].Params
	}
	near := func(step string, got any, want, tolerance float64) {
		t.Helper()
		value, ok := got.(float64)
		if !ok || math.Abs(value-want) > tolerance {
			t.Fatalf("%s: %v, want %v ± %v", step, got, want, tolerance)
		}
	}
	valueText := func(name string) string {
		t.Helper()
		return page.MustEval(`sel => document.querySelector(sel).getAttribute('aria-valuetext')`, wheel(name)).Str()
	}
	history := func() int { t.Helper(); return page.MustEval(`() => ` + app + `.history.length`).Int() }
	before := effectsCapture(t, page)

	// Drag the gain wheel toward orange.
	page.MustElement(wheel("gain")).MustScrollIntoView()
	box := page.MustEval(`sel => { const r = document.querySelector(sel).getBoundingClientRect(); return {x: r.left + r.width / 2, y: r.top + r.height / 2, r: Math.min(r.width, r.height) / 2 / 1.2}; }`, wheel("gain"))
	cx, cy, radius := box.Get("x").Num(), box.Get("y").Num(), box.Get("r").Num()
	tx, ty := cx+0.8*radius*math.Cos(math.Pi/6), cy-0.8*radius*math.Sin(math.Pi/6)
	page.Mouse.MustMoveTo(cx, cy).MustDown(proto.InputMouseButtonLeft).MustMoveTo((cx+tx)/2, (cy+ty)/2).MustMoveTo(tx, ty).MustUp(proto.InputMouseButtonLeft)
	got := params()
	near("gain_x after the drag", got["gain_x"], 0.8*math.Cos(math.Pi/6), 0.05)
	near("gain_y after the drag", got["gain_y"], 0.4, 0.05)
	if text := valueText("gain"); !regexp.MustCompile(`^Hue (2[6-9]|3[0-4])°, strength (7[5-9]|8[0-5]) %$`).MatchString(text) {
		t.Errorf("gain wheel aria-valuetext %q", text)
	}
	warm := effectsCapture(t, page)
	if (warm[0]-warm[2])-(before[0]-before[2]) < 12 {
		t.Fatalf("the gain wheel toward orange did not warm the GL capture: %v -> %v", before, warm)
	}

	// Keys on the lift wheel: one undo step each; focus stays on the wheel.
	steps := history()
	page.MustElement(wheel("lift")).MustFocus()
	for i := 0; i < 3; i++ {
		page.KeyActions().Type(input.ArrowRight).MustDo()
	}
	page.KeyActions().Press(input.ShiftLeft).Type(input.ArrowUp).MustDo()
	got = params()
	near("lift_x after three arrows", got["lift_x"], 0.03, 1e-9)
	near("lift_y after Shift+ArrowUp", got["lift_y"], 0.1, 1e-9)
	if n := history() - steps; n != 4 {
		t.Errorf("four wheel keys made %d undo steps", n)
	}
	if !effectsFocused(page, wheel("lift")) {
		t.Error("the lift wheel lost focus after its keys")
	}
	if text := valueText("lift"); text != "Hue 73°, strength 10 %" {
		t.Errorf("lift wheel aria-valuetext %q", text)
	}
	page.KeyActions().Type(input.Delete).MustDo()
	got = params()
	near("lift_x after Delete", got["lift_x"], 0, 1e-9)
	near("lift_y after Delete", got["lift_y"], 0, 1e-9)
	effectsClip(t, page) // fails when Delete removed the clip

	// A double-click resets the gain wheel.
	page.MustEval(`sel => document.querySelector(sel).dispatchEvent(new MouseEvent('dblclick', {bubbles: true}))`, wheel("gain"))
	got = params()
	near("gain_x after the double-click", got["gain_x"], 0, 1e-9)
	near("gain_y after the double-click", got["gain_y"], 0, 1e-9)
	if !page.MustEval(`() => window.VideoStudioTimeline.validTimeline(` + app + `.project)`).Bool() {
		t.Error("the wheel edits left an invalid timeline")
	}
	page.MustEval(`sel => document.querySelector(sel).scrollIntoView({ block: 'center' })`, wheel("gain"))
	page.MustScreenshot(filepath.Join(shots, "effects-color-wheels-1280x900.png"))
	page.MustEval(`sel => document.querySelector(sel).scrollIntoView({ block: 'center' })`, `[data-inspector] [data-fx-entry="0"] [data-fx-param="tint"]`)
	page.MustScreenshot(filepath.Join(shots, "effects-color-sliders-1280x900.png"))
}
