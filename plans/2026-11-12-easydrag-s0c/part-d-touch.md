[← Plan index](README.md) · Spec: [plans/2026-11-12-easydrag-s0c-spec.md](../2026-11-12-easydrag-s0c-spec.md)

## Part D: Touch audit (FD-15)

Spec: `plans/2026-11-12-easydrag-s0c-spec.md` §6 (design), §7 rows 17+ (ledger), §8 (tests), §11 (risk: touch emulation is not a real finger). The tasks run in the order D.1 to D.7. D.1 stops the part if Part B has not landed.

**Shell.** Git Bash. Every command starts with `cd <worktree> && ...` because the working directory resets between calls. Set `MSYS_NO_PATHCONV=1` for any argument that starts with `//`.

**Starts after A and B have landed on main.** Create the worktree only when the coordinator says so.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-s0c-touch -b feat/easydrag-s0c-touch main
mkdir -p C:/disposable/easydrag-s0c D:/tmp/go-build D:/tmp/easydrag-s0c-shots
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && git log -1 --format=%h && git status --short
```

Expected: the worktree is created from `main` (b52103d51 or later, with A and B in it); `git status --short` prints nothing.

**Baseline.** Run both once before D.1. A failure that is already on main is reported to the coordinator, not fixed here.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m 2>&1 | tail -15
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && node scripts/test-easydrag.mjs 2>&1 | tail -3
```

Expected: `ok  	aurago/ui	<seconds>s` (the real top-level tests are `TestDesktopEasyDrag*Browser`, `TestConfigEasyDragDurableSwitchBrowser` and the non-browser `TestEasyDrag*`/`TestDesktopEasyDrag*` source tests, so the pattern is `(?i)easydrag`, not `TestDesktopEasyDrag`); the Node script ends with `all EasyDrag checks passed`. Run the browser command with the Bash tool's sandbox disabled: Chrome cannot start from a test binary under the repo, and `GOTMPDIR='D:\tmp\go-build'` puts the binary on D: (`reference_chrome_launch_from_agent`).

**Conventions of this part.**
- **Browser command for one gesture** (sandbox disabled):
  `cd <WT> && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouch<Name>' -count=1 -v -timeout 10m`
  A test that fails alone is re-run alone before it counts as a result.
- **Go gates before every commit that touches Go:** `GOTMPDIR='D:\tmp\go-build' go vet ./ui` and `gofmt -l ui/desktop_easydrag_touch_browser_test.go` (prints nothing).
- **JS/CSS gates before every commit that touches `ui/js` or `ui/css`:** `node scripts/test-easydrag.mjs` (ends with `all EasyDrag checks passed`), `node scripts/test-easydrag-render-baseline.mjs` (prints `PASS render baseline (unchanged)`) and `GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1` without the browser variable (the browser tests skip; the module line ratchet and `TestShippedWebAssetsLoadNothingExternal` run).
- **Line budget.** `easydrag-canvas.js` (928 lines) must not change at all. `easydrag-wires.js` (308) and `easydrag-interact.js` (735) are far below 900 and may take the fix of D.2; the ratchet logs above 900 and fails above 1,100.
- **No locale keys** are added by this part (the fixes reuse `easydrag.ui.wire_insert` and `easydrag.ui.wire_delete`).
- **Escapes.** The Write and Edit tools turn backslash-u sequences into characters; this part has none. Keep it that way.
- **IMPACT.** Run it from the main checkout path (command in D.2). HIGH or CRITICAL is reported to the coordinator; UNKNOWN is confirmed with `grep -rn`. GitNexus cannot index a worktree on this machine.
- **Commits.** One per task: `git add <exact paths>`, then `git commit -F C:/disposable/easydrag-s0c/<message file>`. Write each message file with the Write tool. Every message ends with the GitNexus line and the Co-Authored-By line; the implementer replaces `<your model line>` with the model it runs as.

**What the part depends on.** Part B's helper `startEasyDragSmokeWith(t, easyDragSmokeOptions{Width, Height})` (brief, "The T1 helper API"), `easyDragSmoke` with its fields `t`, `page`, `dir` and the methods `wait`, `settle`, `shot`, `clearToasts`, `failOnPageErrors`, and `requirePrecisionBrowserSmoke` (`ui/precision_workspace_browser_test.go:13`). Part B's guard test `TestEasyDragBrowserTestsUseTheSmokeHelper` reads every `ui/*easydrag*_browser_test.go`, so the touch file must not contain `launcher.New`, `rod.New` or `newSmokeBrowser`.

### What the code says, gesture by gesture (read before D.2)

The prediction for each gesture comes from reading the code on main (b52103d51); each task states it again and says what to do when the test disagrees.

| Gesture | Events it listens to | Blocks touch? | Prediction |
|---|---|---|---|
| Insert on a wire | `pointerover`/`pointerout` on the wire layer show and hide the actions (`easydrag-wires.js:172-181`); `click` on the layer selects the wire (`:234-240`) | `onPointerDown` (`easydrag-interact.js:161`) turns every touch that is not on a card into a pan before it looks at the wire, and the pan captures the pointer, so Chrome sends the click to the canvas and the layer's `click` handler never runs. A touch has no hover: `pointerout` fires after `pointerup` and hides the actions after 220 ms. | **Small finding (F1).** The wire cannot be selected by tap and its Insert/Delete buttons are gone before a second tap can reach them. Both buttons are 24 px and the wire's hit stroke is 16 px (14 px on screen at zoom 0.86), under the 44 px rules the other controls have in `@media (pointer: coarse)`. Fix: `easydrag-interact.js` (a tapped wire is told to `wires`), `easydrag-wires.js` (`tapEdge`, pinned actions), CSS (44 px). No design work needed. |
| Delete a wire | the same buttons; `Delete` key (`easydrag-interact.js:626`) is the only other path | same as above | **Same finding F1**, fixed once in D.2. D.3 is its own test and passes after D.2. |
| Card tools | `pointerdown` on a card selects it (`startNodeDrag`); the tools show on `:hover`, `.is-selected` and `:focus-within` (`desktop-app-easydrag.css:772`); `click` runs them (`easydrag-interact.js:421-425`) | no: `onPointerDown` returns early for `.ed-node-tools`; the canvas has `touch-action: none` (`:656`) | **Works.** A tap selects the card, and `.is-selected` shows the tools; under `@media (pointer: coarse)` and `@container (max-width: 760px)` each tool is 44 px (`:1394-1405`). Residual risk: the toolbar sits 36 px above the card and could be clipped at the canvas top; the test hit-tests each tool, and D.4 has the CSS fix ready. Long press (550 ms, `easydrag-interact.js:235-238`) opens the card menu. |
| Mapping chip into a field | `click` on a tree row calls `onInsert` (`easydrag-mapping.js:178-179`, `easydrag-detail.js:312-322`); HTML5 `dragstart` is the mouse path and is never started by a tap | no; there is no `mouse*` or `touch*` listener anywhere in the EasyDrag modules except one overlay `mousedown` | **Works.** The tap fallback is the row's `click`; `lastField` is set by the field's `focusin` (`easydrag-detail.js:736`), and the tapped `.ed-tpl-view` has `tabindex="0"`. Rows are 28 px high (`:993`), the narrow tabs 44 px (`:884`). |
| Box select | `pointerdown` on empty canvas (`easydrag-interact.js:164`, `startBoxSelect` `:391-415`) | by design: `pointerType === 'touch' && !nodeEl` is a pan (`:161`) | **Design finding.** A finger on empty canvas always pans; no select mode exists. Committed with `t.Skip("Wave 1: ...")` and a ledger row. |

---

### Task D.1: The touch helper

**Files:**
- Create: `WT/ui/desktop_easydrag_touch_browser_test.go`
- Test: `WT/ui/desktop_easydrag_touch_browser_test.go` (`TestEasyDragTouchHelper`)

`WT` is `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch`.

**Design.** Everything is a method of `*easyDragSmoke` or a plain function in the touch file; nothing launches a browser. `startEasyDragTouch(t, w, h)` calls `startEasyDragSmokeWith` with the viewport, then switches the page to a mobile device with 5 touch points, closes and reopens EasyDrag (as `phone()` in `desktop_easydrag_browser_test.go:725` does, because the shell picks its phone layout when the window opens), checks that the page now reports a coarse pointer, and opens the seeded flow with a tap, so the first gesture of every test is already a touch.

- [ ] **Step 1: Verify that Part B landed and the proto names are real.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && grep -n "^type easyDragSmokeOptions\|^func startEasyDragSmokeWith\|^func startEasyDragSmoke(\|^func sharedEasyDragBrowser\|^func setEasyDragReducedMotion" ui/desktop_easydrag_browser_test.go
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && grep -n "Width\|Height" ui/desktop_easydrag_browser_test.go | sed -n 1,12p
cd C:/Users/Andi/go/pkg/mod/github.com/go-rod/rod@v0.116.2/lib/proto && grep -n "^type InputDispatchTouchEvent struct\|^type InputTouchPoint struct\|InputDispatchTouchEventTypeTouchStart \|InputDispatchTouchEventTypeTouchMove \|InputDispatchTouchEventTypeTouchEnd \|^type EmulationSetDeviceMetricsOverride struct\|^type EmulationSetTouchEmulationEnabled struct" input.go emulation.go
```

Expected: the first command prints five lines (`easyDragSmokeOptions`, `startEasyDragSmokeWith`, `startEasyDragSmoke`, `sharedEasyDragBrowser`, `setEasyDragReducedMotion`); the second shows `opts.Width`/`opts.Height` used to set the viewport; the third prints seven lines (verified on rod v0.116.2: `InputDispatchTouchEvent{Type, TouchPoints []*InputTouchPoint}`, `InputTouchPoint{X, Y float64}`, `EmulationSetDeviceMetricsOverride{Width, Height int; DeviceScaleFactor float64; Mobile bool}`, `EmulationSetTouchEmulationEnabled{Enabled bool; MaxTouchPoints *int}`).
If the first command prints nothing, **stop**: Part B has not landed, and this part cannot start. If the helper exists with different field names, use the real names in Step 2 and say so in the report. If a proto field name differs, fix the code below to the real one before running it.

- [ ] **Step 2: Write the helper and its smoke test.** Create `WT/ui/desktop_easydrag_touch_browser_test.go`:

```go
package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// easyDragTouchViewports are the two sizes every touch gesture runs at: a small tablet and a phone.
var easyDragTouchViewports = []struct {
	name string
	w, h int
}{
	{"1024x768", 1024, 768},
	{"390x844", 390, 844},
}

// touchPoint is a point in page (client) coordinates, in CSS pixels.
type touchPoint struct{ x, y float64 }

// startEasyDragTouch opens the seeded flow of the main editor fixture in a w×h touch device: a mobile
// viewport with 5 touch points and a coarse pointer. The flow is opened by a tap.
func startEasyDragTouch(t *testing.T, w, h int) *easyDragSmoke {
	t.Helper()
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Width: w, Height: h})
	page := s.page
	points := 5
	if err := (proto.EmulationSetDeviceMetricsOverride{Width: w, Height: h, DeviceScaleFactor: 1, Mobile: true}).Call(page); err != nil {
		t.Fatalf("device metrics %dx%d: %v", w, h, err)
	}
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &points}).Call(page); err != nil {
		t.Fatalf("touch emulation: %v", err)
	}
	// The shell and the editor choose their layout when the window opens: reopen it as a phone does.
	page.MustEval(`()=>fixtureCloseAll()`)
	s.wait(`()=>!document.querySelector('.ed-app')`)
	page.MustEval(`async()=>{await fixtureOpen('easydrag')}`)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	if !page.MustEval(`()=>matchMedia('(pointer: coarse)').matches && navigator.maxTouchPoints===5`).Bool() {
		t.Fatalf("touch emulation is on, but the page reports no coarse pointer with 5 touch points")
	}
	s.touchTapElement(`.ed-flow-card h3`)
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	return s
}

// touchSend dispatches one Input.dispatchTouchEvent. The list is never nil: touchEnd carries an
// empty array, and a nil slice would be sent as null, which Chrome rejects.
func (s *easyDragSmoke) touchSend(kind proto.InputDispatchTouchEventType, points ...touchPoint) {
	s.t.Helper()
	list := make([]*proto.InputTouchPoint, 0, len(points))
	for _, p := range points {
		list = append(list, &proto.InputTouchPoint{X: p.x, Y: p.y})
	}
	if err := (proto.InputDispatchTouchEvent{Type: kind, TouchPoints: list}).Call(s.page); err != nil {
		s.t.Fatalf("touch %s at %v: %v", kind, points, err)
	}
}

// touchTap puts a finger down at (x, y) and lifts it 50 ms later.
func (s *easyDragSmoke) touchTap(x, y float64) {
	s.t.Helper()
	s.touchSend(proto.InputDispatchTouchEventTypeTouchStart, touchPoint{x, y})
	time.Sleep(50 * time.Millisecond)
	s.touchSend(proto.InputDispatchTouchEventTypeTouchEnd)
}

// touchLongPress holds a finger at (x, y) for ms milliseconds.
func (s *easyDragSmoke) touchLongPress(x, y float64, ms int) {
	s.t.Helper()
	s.touchSend(proto.InputDispatchTouchEventTypeTouchStart, touchPoint{x, y})
	time.Sleep(time.Duration(ms) * time.Millisecond)
	s.touchSend(proto.InputDispatchTouchEventTypeTouchEnd)
}

// touchDrag moves one finger from one point to another in the given number of equal moves, 16 ms apart.
func (s *easyDragSmoke) touchDrag(from, to touchPoint, steps int) {
	s.t.Helper()
	if steps < 1 {
		steps = 1
	}
	s.touchSend(proto.InputDispatchTouchEventTypeTouchStart, from)
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		s.touchSend(proto.InputDispatchTouchEventTypeTouchMove, touchPoint{from.x + (to.x-from.x)*f, from.y + (to.y-from.y)*f})
		time.Sleep(16 * time.Millisecond)
	}
	s.touchSend(proto.InputDispatchTouchEventTypeTouchEnd)
}

// touchCentre returns the centre of the part of the first element matching selector that lies in the
// viewport. It does not scroll: a canvas card scrolled into view would move the overflow:hidden canvas.
func (s *easyDragSmoke) touchCentre(selector string) touchPoint {
	s.t.Helper()
	v := s.page.MustEval(`sel=>{
		const el=document.querySelector(sel);
		if(!el)throw Error('no element matches '+sel);
		const r=el.getBoundingClientRect();
		const l=Math.max(r.left,0),rt=Math.min(r.right,innerWidth),tp=Math.max(r.top,0),b=Math.min(r.bottom,innerHeight);
		if(rt-l<2||b-tp<2)throw Error(sel+' is outside the viewport: '+JSON.stringify(r));
		return [(l+rt)/2,(tp+b)/2];
	}`, selector)
	return touchPoint{v.Get("0").Num(), v.Get("1").Num()}
}

// touchScrollTo scrolls an element of a panel (a detail column, a list) into the middle of its
// scroller. Never use it on canvas cards.
func (s *easyDragSmoke) touchScrollTo(selector string) {
	s.t.Helper()
	s.page.MustEval(`sel=>{const el=document.querySelector(sel);if(!el)throw Error('no element matches '+sel);el.scrollIntoView({block:'center',inline:'nearest'});}`, selector)
}

// touchAssertHit fails when the element at p is not selector or one of its descendants: a finger would
// press something else.
func (s *easyDragSmoke) touchAssertHit(selector string, p touchPoint) {
	s.t.Helper()
	got := s.page.MustEval(`(sel,x,y)=>{
		const e=document.elementFromPoint(x,y),t=document.querySelector(sel);
		if(e&&t&&t.contains(e))return '';
		return e?(e.tagName+'.'+String(e.getAttribute('class')||'')):'nothing';
	}`, selector, p.x, p.y).Str()
	if got != "" {
		s.t.Fatalf("a finger at (%.0f, %.0f) would press %s, not %s", p.x, p.y, got, selector)
	}
}

// touchTapElement taps the visible centre of the first element matching selector, after checking that
// the finger would hit it. It returns the point.
func (s *easyDragSmoke) touchTapElement(selector string) touchPoint {
	s.t.Helper()
	p := s.touchCentre(selector)
	s.touchAssertHit(selector, p)
	s.touchTap(p.x, p.y)
	return p
}

// touchEdgePoint returns a point on the wire's hit stroke that a finger can reach: the first of the
// middle fractions of the path at which the stroke, and not a port button or a card, is on top.
func (s *easyDragSmoke) touchEdgePoint(edgeID string) touchPoint {
	s.t.Helper()
	v := s.page.MustEval(`id=>{
		const hit=document.querySelector('.ed-edge[data-edge-id="'+id+'"] .ed-wire-hit');
		if(!hit)throw Error('no wire '+id);
		const len=hit.getTotalLength(),m=hit.getScreenCTM();
		for(const f of [0.5,0.4,0.6,0.3,0.7]){
			const p=hit.getPointAtLength(len*f),s=new DOMPoint(p.x,p.y).matrixTransform(m);
			if(document.elementFromPoint(s.x,s.y)===hit)return [s.x,s.y,f];
		}
		throw Error('no point of wire '+id+' is on screen and on top');
	}`, edgeID)
	if f := v.Get("2").Num(); f != 0.5 {
		s.t.Logf("wire %s: the middle is covered or off screen; using the point at %.0f%% of its length", edgeID, f*100)
	}
	return touchPoint{v.Get("0").Num(), v.Get("1").Num()}
}

// touchEmptyPoint returns a point of the canvas with no card, wire, control or group on it.
func (s *easyDragSmoke) touchEmptyPoint() touchPoint {
	s.t.Helper()
	v := s.page.MustEval(`()=>{
		const c=document.querySelector('.ed-canvas').getBoundingClientRect();
		for(let row=5;row>=0;row--)for(let col=0;col<6;col++){
			const x=c.left+12+col*(c.width-24)/5,y=c.top+12+row*(c.height-24)/5;
			const e=document.elementFromPoint(x,y);
			if(e&&e.closest('.ed-canvas')&&!e.closest('.ed-node,.ed-edge,.ed-zoom,.ed-minimap,.ed-empty,.ed-wire-actions,.ed-port-tip,.ed-group-frame,.ed-quick'))return [x,y];
		}
		throw Error('no empty point on the canvas');
	}`)
	return touchPoint{v.Get("0").Num(), v.Get("1").Num()}
}

// TestEasyDragTouchHelper proves the helper itself at both viewports: the page is a touch device, a
// tap is a touch pointer, and a finger dragged over empty canvas pans the view.
func TestEasyDragTouchHelper(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			s.page.MustEval(`()=>{window.__touchTypes=[];document.addEventListener('pointerdown',e=>window.__touchTypes.push(e.pointerType),true);}`)
			from := s.touchEmptyPoint()
			x0 := s.page.MustEval(`()=>edFixture.editor().view.x`).Num()
			s.touchDrag(from, touchPoint{from.x + 60, from.y}, 6)
			s.wait(fmt.Sprintf(`()=>Math.abs(edFixture.editor().view.x-(%f)-60)<4`, x0))
			s.wait(`()=>window.__touchTypes.length===1 && window.__touchTypes[0]==='touch'`)
			s.failOnPageErrors("touch helper")
		})
	}
}
```

- [ ] **Step 3: Gate and run.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && gofmt -l ui/desktop_easydrag_touch_browser_test.go && GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchHelper' -count=1 -v -timeout 10m 2>&1 | tail -15
```

Expected: `gofmt` and `go vet` print nothing; the test prints `--- PASS: TestEasyDragTouchHelper/1024x768`, `--- PASS: TestEasyDragTouchHelper/390x844`, `--- PASS: TestEasyDragTouchHelper`, `ok  	aurago/ui`.

If it fails:
- `touch emulation is on, but the page reports no coarse pointer`: the override did not reach the page; print `matchMedia('(pointer: coarse)').matches` and `navigator.maxTouchPoints` and report; do not continue with a helper that cannot make a touch device.
- `no empty point on the canvas` or the pan assertion fails at 390: print `document.elementsFromPoint` at the chosen point; a control over the canvas corner is a finding for D.7 (a first guess is the zoom bar).
- `Invalid parameters` from `Input.dispatchTouchEvent` on touchEnd: the slice was nil; `touchSend` builds it with `make`, keep it that way.

- [ ] **Step 4: The guard test accepts the file.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragBrowserTestsUseTheSmokeHelper' -count=1 -v 2>&1 | tail -5
```

Expected: `--- PASS: TestEasyDragBrowserTestsUseTheSmokeHelper` and `ok`. If it names the touch file, a forbidden call is in it; remove the call.

- [ ] **Step 5: Commit.** Write `C:/disposable/easydrag-s0c/d1-message.txt`:

```
test(easydrag): touch helper for CDP touch emulation

startEasyDragTouch opens the seeded flow in a 1024x768 or 390x844 touch device
(Emulation.setDeviceMetricsOverride with mobile, setTouchEmulationEnabled with 5
points) through startEasyDragSmokeWith, and touchTap, touchDrag, touchLongPress
send Input.dispatchTouchEvent. Small helpers find an element's visible centre, a
reachable point on a wire and an empty canvas point, and refuse a tap that would
hit another element. TestEasyDragTouchHelper proves the device, the touch pointer
and a finger pan at both sizes.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && git add ui/desktop_easydrag_touch_browser_test.go && git commit -F C:/disposable/easydrag-s0c/d1-message.txt
```

---

### Task D.2: Insert a step on a wire by touch (finding F1: small, fixed)

**Files:**
- Modify: `WT/ui/js/desktop/apps/easydrag-interact.js:159-163` (the pan decision) and `:182-190` (`startPan`)
- Modify: `WT/ui/js/desktop/apps/easydrag-wires.js:19-21` (state), `:157-172` (`hideActions`), `:178-189` (pointer listeners), `:234-241` (click), `:299-303` (export)
- Modify: `WT/ui/css/desktop-app-easydrag.css` (a coarse-pointer block next to `:1394`)
- Test: `WT/ui/desktop_easydrag_touch_browser_test.go` (`TestEasyDragTouchInsertOnWire`)

**Prediction.** The test fails on today's code (see the table above): a tap on a wire is a pan start, so the wire is not selected through its `click`, and the actions the `pointerover` showed disappear 220 ms after the finger lifts. This is a **small finding**: no design is needed, because the wire already has `selectedEdge`, the actions and the insert popover; only the touch path to them is missing. If the test passes on today's code (the first wait and the 400 ms wait both hold), there is no finding F1; skip Steps 4 to 6, keep the test, and say so in D.7.

- [ ] **Step 1: Write the failing test.** Append to `WT/ui/desktop_easydrag_touch_browser_test.go`:

```go
// TestEasyDragTouchInsertOnWire taps the wire between the trigger and the first step: the wire is
// selected, its actions stay open for the next tap, Insert opens the quick-add list, and the step
// chosen there lands between the two cards.
func TestEasyDragTouchInsertOnWire(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			page := s.page
			p := s.touchEdgePoint("e_1")
			s.touchTap(p.x, p.y)
			s.wait(`()=>{const a=document.querySelector('.ed-wire-actions');return !!a && !a.hidden && edFixture.editor().selectedEdge==='e_1' && !!document.querySelector('.ed-edge.is-selected[data-edge-id="e_1"]')}`)
			// A finger has no hover to keep the actions open: they must outlive the 220 ms hide delay.
			time.Sleep(450 * time.Millisecond)
			s.wait(`()=>{const a=document.querySelector('.ed-wire-actions');const b=a && a.querySelector('[data-ed-wire="insert"]');if(!a||a.hidden||!b)return false;const r=b.getBoundingClientRect();return r.width>=24 && r.height>=24}`)
			s.shot("touch-wire-actions-" + vp.name)
			s.touchTapElement(`.ed-wire-actions [data-ed-wire="insert"]`)
			s.wait(`()=>!!document.querySelector('.ed-quick [data-ed-index="0"]') && document.querySelector('.ed-wire-actions').hidden`)
			s.touchTapElement(`.ed-quick [data-ed-index="0"]`)
			s.wait(`()=>{const m=edFixture.editor().model;if(m.doc.nodes.length!==5||m.doc.edges.length!==4)return false;const added=m.doc.nodes.find(n=>!['n_zeitplan','n_suche','n_ki','n_pdf'].includes(n.id));return !!added && m.incoming(added.id).map(e=>e.source.node).join()==='n_zeitplan' && m.incoming('n_suche').map(e=>e.source.node).join()===added.id && !document.querySelector('.ed-quick')}`)
			s.shot("touch-wire-inserted-" + vp.name)
			page.MustEval(`()=>edFixture.editor().model.undo()`)
			s.wait(`()=>edFixture.editor().model.doc.nodes.length===4 && edFixture.editor().model.doc.edges.length===3`)
			s.failOnPageErrors("touch insert on wire")
		})
	}
}
```

- [ ] **Step 2: Run it and see it fail for the stated reason.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchInsertOnWire' -count=1 -v -timeout 10m 2>&1 | grep -v "^=== " | head -20
```

Expected: `--- FAIL: TestEasyDragTouchInsertOnWire/1024x768` and `.../390x844` with `condition did not become true: ()=>{const a=document.querySelector('.ed-wire-actions')...selectedEdge==='e_1'...` (the first wait: no selection after the tap), or, if Chrome did deliver the click to the layer, at the `.ed-wire-actions [data-ed-wire="insert"]` wait after the 450 ms sleep (the actions vanished). Note which of the two it was; it goes into the D.7 evidence. Any other failure (for example `no point of wire e_1 is on screen and on top`) is a helper problem: fix the helper, not the product.

- [ ] **Step 3: IMPACT before editing.**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "startPan" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "hideActions" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
cd C:/Users/Andi/Documents/repo/AuraGo && grep -rn "startPan\|hideActions\|\.tapEdge" ui/js/desktop/apps/*.js | grep -v "^ui/js/desktop/apps/easydrag-wires.js\|^ui/js/desktop/apps/easydrag-interact.js"
```

Expected: `startPan` has one caller (`onPointerDown`, same file); `hideActions` is called inside `easydrag-wires.js` and through `wires.hideActions` from `easydrag-interact.js`; the grep prints nothing (no other module uses them). Warn on HIGH or CRITICAL; confirm UNKNOWN with the grep.

- [ ] **Step 4: The wires module.** In `WT/ui/js/desktop/apps/easydrag-wires.js`:

(a) State, after `let dropTarget = null;`:

```js
        let dropTarget = null;
        // pinned: a tapped wire keeps its actions open until the next press elsewhere (a finger has no hover).
        let pinned = false;
```

(b) `hideActions` clears it:

```js
            const hide = () => {
                actions.hidden = true;
                pinned = false;
                hoverEdge = null;
```

(c) The pointer listeners stay out of the way of a pinned wire. Replace the `pointerout` listener and the `pointerleave` listener, and add the dismiss listener:

```js
        bag.listen(layer, 'pointerout', (event) => {
            if (pinned) return;
            if (!event.relatedTarget || !event.relatedTarget.closest || !event.relatedTarget.closest('.ed-wire-actions, .ed-edge')) hideActions(false);
        });
        bag.listen(actions, 'pointerenter', () => clearTimeout(hideTimer));
        bag.listen(actions, 'pointerleave', () => { if (!pinned) hideActions(false); });
        bag.listen(actions, 'pointerdown', event => event.stopPropagation());
        // A press anywhere else on the canvas puts a pinned wire's actions away (a press on a wire
        // pins it again when the tap ends).
        bag.listen(canvas.el, 'pointerdown', (event) => {
            if (pinned && !event.target.closest('.ed-wire-actions')) hideActions(true);
        }, true);
```

(d) One function selects a wire; the layer's `click` and the new `tapEdge` use it. Replace the `layer` `click` listener:

```js
        function selectEdge(edgeId) {
            ed.selectedEdge = edgeId;
            ed.selection.clear();
            ed.bus.emit('selection', ed.selection); // marks the wire through markSelected
        }

        // tapEdge is a finger's tap on a wire (easydrag-interact.js): the wire is selected and its actions stay
        // open, so Insert and Delete can be tapped.
        function tapEdge(edgeId) {
            if (!ed.model.edge(edgeId)) return;
            selectEdge(edgeId);
            showActions(edgeId);
            pinned = !actions.hidden;
        }

        bag.listen(layer, 'click', (event) => {
            const g = event.target.closest('.ed-edge');
            if (!g) return;
            event.stopPropagation();
            selectEdge(g.dataset.edgeId);
        });
```

(e) Export it:

```js
        return {
            render, renderFor, highlight, preview, markDropTarget, ends, tapEdge,
```

- [ ] **Step 5: The pan decision and `startPan`.** In `WT/ui/js/desktop/apps/easydrag-interact.js`, `onPointerDown`:

```js
            const panGesture = event.button === 1 || spaceDown || (event.pointerType === 'touch' && !nodeEl);
            if (panGesture) { startPan(event, event.pointerType === 'touch' ? event.target.closest('.ed-edge') : null); return; }
```

and replace `startPan`:

```js
        // startPan moves the view with the pointer. A finger that lands on a wire and does not travel
        // (tapped) selects the wire: the pan captures the pointer, so the wire never gets its own click.
        function startPan(event, tapped) {
            event.preventDefault();
            const start = { x: event.clientX, y: event.clientY, view: Object.assign({}, ed.view) };
            let moved = false;
            el.classList.add('is-panning');
            capture(event, ev => {
                if (pinch) return;
                if (Math.hypot(ev.clientX - start.x, ev.clientY - start.y) >= DRAG_THRESHOLD) moved = true;
                canvas.setView({ x: start.view.x + ev.clientX - start.x, y: start.view.y + ev.clientY - start.y, zoom: start.view.zoom });
            }, (ev, cancelled) => {
                el.classList.remove('is-panning');
                if (tapped && !moved && !cancelled) wires.tapEdge(tapped.dataset.edgeId);
            });
        }
```

- [ ] **Step 6: Touch sizes in CSS.** In `WT/ui/css/desktop-app-easydrag.css`, directly after the first `@container (max-width: 760px)` block that follows `@media (pointer: coarse)` at about line 1394-1407 (the block that ends with `.ed-app .ed-error { font-size: 14px; line-height: 1.45; }` and a closing brace), add:

```css
/* Wires under a finger: a 44 px stroke to hit and 44 px Insert and Delete buttons (the wire's 16 px
   stroke and 24 px buttons stay for a mouse). */
@media (pointer: coarse) {
    .ed-app .ed-wire-hit { stroke-width: 44px; }
    .ed-app .ed-wire-btn { width: 44px; height: 44px; }
}
@container (max-width: 760px) {
    .ed-app .ed-wire-hit { stroke-width: 44px; }
    .ed-app .ed-wire-btn { width: 44px; height: 44px; }
}
```

(The wire layer is scaled with the zoom, so 44 user units are 38 px at the automatic 0.86 zoom: above the 24 px floor.)

- [ ] **Step 7: Run the test and see it pass, then the gates.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchInsertOnWire' -count=1 -v -timeout 10m 2>&1 | grep -v "^=== " | head -12
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && node scripts/test-easydrag.mjs 2>&1 | tail -2 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -2
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && gofmt -l ui/desktop_easydrag_touch_browser_test.go && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -3
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && wc -l ui/js/desktop/apps/easydrag-wires.js ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-canvas.js && git diff --stat -- ui/js/desktop/apps/easydrag-canvas.js
```

Expected: two `--- PASS` lines (and the screenshots `touch-wire-actions-*.png`, `touch-wire-inserted-*.png` in `D:/tmp/easydrag-s0c-shots`; open one at 390 and check that the 44 px buttons sit over the wire); `all EasyDrag checks passed`; `PASS render baseline (unchanged)`; `gofmt` prints nothing; `ok  	aurago/ui`; `easydrag-wires.js` about 330 lines, `easydrag-interact.js` about 745, `easydrag-canvas.js` 928, and the `git diff --stat` for canvas.js prints nothing.

If the 390 case fails at `no point of wire e_1 is on screen and on top`, the wire between the trigger and the first step is not reachable at the automatic view: report the numbers (`ed.view`, the canvas rect) in D.7 as a second finding; do not change the test to another wire. If it fails at the `.ed-quick [data-ed-index="0"]` tap with `would press ...`, the quick-add popover is covered at that size: read the named element, and fix by CSS if it is a z-index of the wire actions (`.ed-wire-actions` has `z-index: 2`), otherwise report.

- [ ] **Step 8: Commit.** Write `C:/disposable/easydrag-s0c/d2-message.txt`:

```
fix(easydrag): a tap on a wire selects it and keeps Insert and Delete open

A finger on a wire started a canvas pan before the wire was looked at, and the pan
captured the pointer, so the wire's click never ran; the hover-driven actions were
hidden 220 ms after the finger lifted. A tapped wire (a touch that does not travel)
is now told to the wires module, which selects it and keeps its actions open until
the next press elsewhere. Under a coarse pointer or a narrow window the wire's hit
stroke and its two buttons are 44 px. TestEasyDragTouchInsertOnWire proves it at
1024x768 and 390x844.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && git add ui/desktop_easydrag_touch_browser_test.go ui/js/desktop/apps/easydrag-interact.js ui/js/desktop/apps/easydrag-wires.js ui/css/desktop-app-easydrag.css && git commit -F C:/disposable/easydrag-s0c/d2-message.txt
```

---

### Task D.3: Delete a wire by touch

**Files:**
- Test: `WT/ui/desktop_easydrag_touch_browser_test.go` (`TestEasyDragTouchDeleteWire`)

**Prediction.** **Works after D.2** (the fix of F1 is the whole path: tap selects, the pinned Delete button removes). Before D.2 it fails for the same reason as D.2's test. If it fails after D.2, the failing wait names the step; a delete that removes the wire but leaves the actions on screen is a bug in the `click` handler of the actions (`easydrag-wires.js`, `hideActions(true)` after `core.removeConnection`) and is fixed there.

- [ ] **Step 1: Write the test.** Append to `WT/ui/desktop_easydrag_touch_browser_test.go`:

```go
// TestEasyDragTouchDeleteWire taps a wire and then its Delete button: the wire is gone, its
// neighbours stay, the actions leave, and undo brings the wire back.
func TestEasyDragTouchDeleteWire(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			page := s.page
			p := s.touchEdgePoint("e_1")
			s.touchTap(p.x, p.y)
			time.Sleep(450 * time.Millisecond)
			s.wait(`()=>{const a=document.querySelector('.ed-wire-actions');const b=a && a.querySelector('[data-ed-wire="delete"]');if(!a||a.hidden||!b)return false;const r=b.getBoundingClientRect();return r.width>=24 && r.height>=24 && edFixture.editor().selectedEdge==='e_1'}`)
			s.touchTapElement(`.ed-wire-actions [data-ed-wire="delete"]`)
			s.wait(`()=>{const m=edFixture.editor().model;return m.doc.edges.length===2 && !m.edge('e_1') && m.doc.nodes.length===4 && document.querySelector('.ed-wire-actions').hidden && !document.querySelector('.ed-edge[data-edge-id="e_1"]')}`)
			s.shot("touch-wire-deleted-" + vp.name)
			s.clearToasts()
			page.MustEval(`()=>edFixture.editor().model.undo()`)
			s.wait(`()=>edFixture.editor().model.doc.edges.length===3 && !!edFixture.editor().model.edge('e_1')`)
			s.failOnPageErrors("touch delete wire")
		})
	}
}
```

- [ ] **Step 2: Run it.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchDeleteWire' -count=1 -v -timeout 10m 2>&1 | grep -v "^=== " | head -8
```

Expected: `--- PASS: TestEasyDragTouchDeleteWire/1024x768`, `--- PASS: TestEasyDragTouchDeleteWire/390x844`, `PASS`.

If `s.clearToasts` times out (the delete shows no toast, only a live-region text), drop that line; it is only there because deletions of steps raise an Undo toast. Do not add sleeps for any other reason.

- [ ] **Step 3: Gate and commit.** Write `C:/disposable/easydrag-s0c/d3-message.txt`:

```
test(easydrag): delete a wire by touch at both viewports

Tap the wire, tap Delete: the wire goes, the steps stay, the actions leave, undo
restores it. The path relies on the tap-to-select fix of the previous commit.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && gofmt -l ui/desktop_easydrag_touch_browser_test.go && GOTMPDIR='D:\tmp\go-build' go vet ./ui && git add ui/desktop_easydrag_touch_browser_test.go && git commit -F C:/disposable/easydrag-s0c/d3-message.txt
```

Expected: no output from `gofmt` and `go vet`; one commit.

---

### Task D.4: Card tools by tap, and the long-press menu

**Files:**
- Test: `WT/ui/desktop_easydrag_touch_browser_test.go` (`TestEasyDragTouchCardTools`)
- Modify (only if Step 2 fails at a hit check): `WT/ui/css/desktop-app-easydrag.css` (next to `:1394`)

**Prediction.** **Works.** `pointerdown` on the card selects it, `.ed-node.is-selected .ed-node-tools` makes the toolbar visible and clickable (`desktop-app-easydrag.css:772`), the tools are 44 px under a coarse pointer or a narrow window (`:1394-1405`), `onPointerDown` leaves a press on `.ed-node-tools` alone, and `click` runs the tool. The tapped card is the trigger: its whole body is on screen at both sizes (48 px from the canvas edge at 0.86, `phone()` asserts this), so no pan is needed. The long press is `setTimeout(… openNodeMenu …, 550)` for touch pointers (`easydrag-interact.js:235-238`).

- [ ] **Step 1: Write the test.** Append to `WT/ui/desktop_easydrag_touch_browser_test.go`:

```go
// TestEasyDragTouchCardTools taps a card: it is selected, its tools are on top and big enough, and the
// Disable tool works by tap (twice: off, on). A long press opens the card's menu.
func TestEasyDragTouchCardTools(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			page := s.page
			card := `.ed-node[data-node-id="n_zeitplan"]`
			s.touchTapElement(card)
			s.wait(`()=>{const c=document.querySelector('` + card + `');const tools=c.querySelector('.ed-node-tools');const cs=getComputedStyle(tools);return c.classList.contains('is-selected') && cs.opacity==='1' && cs.pointerEvents==='auto' && cs.visibility==='visible'}`)
			// Every tool is on top at its centre and not smaller than 24 px.
			for _, tool := range []string{"add", "connect", "test", "disable", "duplicate", "delete"} {
				sel := card + ` [data-ed-node-tool="` + tool + `"]`
				s.touchAssertHit(sel, s.touchCentre(sel))
				size := page.MustEval(`sel=>{const r=document.querySelector(sel).getBoundingClientRect();return Math.min(r.width,r.height)}`, sel).Num()
				if size < 24 {
					t.Fatalf("tool %s is %.0f px: under 24 px", tool, size)
				}
			}
			s.shot("touch-card-tools-" + vp.name)
			disable := card + ` [data-ed-node-tool="disable"]`
			s.touchTapElement(disable)
			s.wait(`()=>edFixture.editor().model.node('n_zeitplan').settings.disabled===true`)
			s.touchTapElement(disable)
			s.wait(`()=>!edFixture.editor().model.node('n_zeitplan').settings.disabled`)
			// A long press opens the card menu.
			p := s.touchCentre(card)
			s.touchAssertHit(card, p)
			s.touchLongPress(p.x, p.y, 700)
			s.wait(`()=>document.querySelectorAll('.vd-context-menu .vd-context-item').length>0`)
			s.shot("touch-card-menu-" + vp.name)
			page.MustEval(`()=>aurora.closeContextMenu()`)
			s.wait(`()=>!document.querySelector('.vd-context-menu')`)
			s.failOnPageErrors("touch card tools")
		})
	}
}
```

- [ ] **Step 2: Run it.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchCardTools' -count=1 -v -timeout 10m 2>&1 | grep -v "^=== " | head -10
```

Expected: both subtests `--- PASS`. Open `touch-card-tools-390x844.png`: the toolbar must be fully on the canvas.

**Branch: it works (expected).** Go to Step 4.

**Branch: a small finding.** The only failure the code lets me expect is `a finger at (x, y) would press ... not .ed-node[...] [data-ed-node-tool=...]` or a tool outside the viewport (`is outside the viewport`), meaning that the toolbar, which sits 36 px above the card (`desktop-app-easydrag.css:765-768`, `top: -36px`), is clipped by the canvas's `overflow: hidden` or covered by the canvas header at the top. Then:

- [ ] **Step 3 (only on that failure): Move the toolbar under the card for touch.** Next to the wire block of D.2 in `WT/ui/css/desktop-app-easydrag.css`:

```css
/* A finger cannot hover and the toolbar must not be clipped at the top of the canvas: under a
   coarse pointer or in a narrow window the tools sit under the card. */
@media (pointer: coarse) {
    .ed-app .ed-node-tools { top: calc(100% + 6px); }
}
@container (max-width: 760px) {
    .ed-app .ed-node-tools { top: calc(100% + 6px); }
}
```

Run Step 2 again (expected: both `--- PASS`), then the JS/CSS gates of the conventions. If the tools are covered by something other than clipping (the named element is a card or a wire), report it as a finding in D.7 with the element name and do not guess a fix.

- [ ] **Step 4: Gate and commit.** Write `C:/disposable/easydrag-s0c/d4-message.txt` (use the second paragraph only if Step 3 was needed):

```
test(easydrag): card tools reachable by tap, long press opens the menu

A tap selects the card; its six tools are on top, at least 24 px, and the Disable
tool works by tap. A 700 ms press opens the card menu. Checked at 1024x768 and
390x844.

(Step 3 only) The tool bar sits under the card for a coarse pointer or a narrow
window: above the card it was clipped at the top of the canvas.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && gofmt -l ui/desktop_easydrag_touch_browser_test.go && GOTMPDIR='D:\tmp\go-build' go vet ./ui && git add ui/desktop_easydrag_touch_browser_test.go && git status --short
```

Expected: no output from `gofmt` and `go vet`; `git status --short` shows the touch test file as staged (`M`) and, only after Step 3, the CSS file, which is added with the same `git add` (`git add ui/css/desktop-app-easydrag.css`). Then `git commit -F C:/disposable/easydrag-s0c/d4-message.txt`.

---

### Task D.5: Mapping chip into a field by the tap fallback

**Files:**
- Test: `WT/ui/desktop_easydrag_touch_browser_test.go` (`TestEasyDragTouchMappingChip`)

**Prediction.** **Works.** HTML5 drag (`dragstart` on `.ed-tree-row`, `easydrag-mapping.js:187-208`) is the mouse path and a tap never starts it. The tap fallback is the row's `click` handler (`easydrag-mapping.js:178-179`) that calls `onInsert`, which inserts into the last focused field (`lastField`, set by `focusin` at `easydrag-detail.js:736`) and switches to the Parameters tab (`:320-322`). A tap on the field's `.ed-tpl-view` (`tabindex="0"`, `easydrag-template-field.js:83`) focuses it. Tree rows are 28 px high (`desktop-app-easydrag.css:993`), the narrow-mode tabs 44 px (`:884`). The mouse version of this journey is already in `TestDesktopEasyDragBrowser` ("Detail view: type into the message and click the AI text", `desktop_easydrag_browser_test.go:95-111`), which is why the test follows the same steps. The step under test is the PDF step's `content` field (a template field), with `ki.text` from the step before it.

- [ ] **Step 1: Write the test.** Append to `WT/ui/desktop_easydrag_touch_browser_test.go`:

```go
// TestEasyDragTouchMappingChip fills a field by tap: tap the field, open the input tab, tap the
// "ki.text" row. HTML5 drag cannot be started by a finger; the row's click is the fallback.
func TestEasyDragTouchMappingChip(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			page := s.page
			page.MustEval(`()=>{const ed=edFixture.editor();ed.model.setParam('n_pdf','content','Text: ');ed.bus.emit('open-detail',{nodeId:'n_pdf'});}`)
			field := `.ed-detail .ed-field[data-param="content"]`
			s.wait(`()=>!!document.querySelector('` + field + ` .ed-tpl-view')`)
			s.touchScrollTo(field + ` .ed-tpl-view`)
			s.touchTapElement(field + ` .ed-tpl-view`)
			s.wait(`()=>!!document.querySelector('` + field + ` .ed-tpl.is-editing')`)
			s.touchTapElement(`.ed-detail [data-ed-narrow="input"]`)
			row := `.ed-detail .ed-tree-row[data-ref="ki.text"]`
			s.wait(`()=>!!document.querySelector('` + row + `') && document.querySelector('` + row + `').getClientRects().length>0`)
			s.touchScrollTo(row)
			if h := page.MustEval(`sel=>document.querySelector(sel).getBoundingClientRect().height`, row).Num(); h < 24 {
				t.Fatalf("the tree row is %.0f px high: under 24 px", h)
			}
			s.shot("touch-mapping-row-" + vp.name)
			s.touchTapElement(row)
			s.wait(`()=>edFixture.editor().model.node('n_pdf').params.content.includes('{{ki.text}}')`)
			s.wait(`()=>document.querySelectorAll('` + field + ` .ed-ref').length>=1 || !!document.querySelector('` + field + ` .ed-tpl')`)
			s.shot("touch-mapping-inserted-" + vp.name)
			s.failOnPageErrors("touch mapping chip")
		})
	}
}
```

- [ ] **Step 2: Run it.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchMappingChip' -count=1 -v -timeout 10m 2>&1 | grep -v "^=== " | head -10
```

Expected: both subtests `--- PASS`.

**Branch: it works (expected).** Go to Step 4.

**Branch: a small finding.** The code lets me expect only one failure here: the tap on the row says "choose a target" instead of inserting (the toast or live-region text `easydrag.ui.mapping_choose_target`, `easydrag-detail.js:314-319`), meaning that the tap on the field did not leave a `lastField`. Check with `page.MustEval("()=>document.querySelector('.ed-detail [data-ed-mapping-target]').value")`:
- empty: the focus of the tapped `.ed-tpl-view` did not reach the detail's `focusin` handler (`easydrag-detail.js:731-737`). Fix in `easydrag-forms.js` (550 lines, room for a few lines): in the existing `el.addEventListener('focusin', ...)` at `:429-432`, nothing is needed; the handler to extend is the detail's `focusin`, but `easydrag-detail.js` takes only a one-line hook, so add the touch path to the form: a `pointerdown` listener in `easydrag-forms.js` next to the `focusin` listener that focuses `event.target.closest('.ed-tpl-view')` when `event.pointerType === 'touch'`.
- not empty but the insert did not land: a tap-time race (`current()` at `:301` compares the field value with the one captured), not a touch problem; report it.

Run Step 2 again after any fix, then the JS gates of the conventions. If neither explanation fits, stop and report the measured values in D.7.

- [ ] **Step 3: (only after a fix) Gates.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -2
```

Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `ok  	aurago/ui`.

- [ ] **Step 4: Commit.** Write `C:/disposable/easydrag-s0c/d5-message.txt`:

```
test(easydrag): the mapping chip reaches a field by tap

HTML5 drag cannot be started by a finger. The fallback is a tap on the field and a
tap on the input row, which inserts {{ki.text}} into the field. Checked at 1024x768
and 390x844, where the input and parameter tabs alternate.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && gofmt -l ui/desktop_easydrag_touch_browser_test.go && GOTMPDIR='D:\tmp\go-build' go vet ./ui && git add ui/desktop_easydrag_touch_browser_test.go && git commit -F C:/disposable/easydrag-s0c/d5-message.txt
```

(If Step 2 needed a fix, add the fixed file to the same `git add` and describe the fix in the message instead of the second paragraph above.)

---

### Task D.6: Box select by touch (finding F2: design, handed to Wave 1)

**Files:**
- Test: `WT/ui/desktop_easydrag_touch_browser_test.go` (`TestEasyDragTouchBoxSelect`, committed with `t.Skip`)

**Prediction.** **Design finding.** `onPointerDown` decides `panGesture = ... || (event.pointerType === 'touch' && !nodeEl)` (`easydrag-interact.js:161`) before it reaches `startBoxSelect` (`:164`), so a finger on empty canvas always pans: the box never starts, and the selection stays empty. The canvas has `touch-action: none`, so nothing else competes. This is deliberate (one finger pans, two pinch), and the way to select several cards by finger is a design question (a select mode, a long press on empty canvas that starts the box, or a toolbar toggle), not a missing attribute. The test is written to the pass condition, run once to record the failure, and then committed skipped.

- [ ] **Step 1: Write the test.** Append to `WT/ui/desktop_easydrag_touch_browser_test.go`:

```go
// easyDragTouchBoxSelectSkip is the ledger text of the finding this test records (spec S0c section 7).
const easyDragTouchBoxSelectSkip = "Wave 1: a one-finger drag on empty canvas pans the view (easydrag-interact.js onPointerDown, pointerType touch and no card), so several cards cannot be selected by finger at 1024x768 or 390x844; Wave 1 adds a touch select mode that starts startBoxSelect, then this skip goes"

// TestEasyDragTouchBoxSelect drags a finger from empty canvas over the first two cards: both are
// selected, and the view has not moved.
func TestEasyDragTouchBoxSelect(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, vp := range easyDragTouchViewports {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			s := startEasyDragTouch(t, vp.w, vp.h)
			page := s.page
			box := page.MustEval(`()=>{
				const c=document.querySelector('.ed-canvas').getBoundingClientRect();
				const a=document.querySelector('.ed-node[data-node-id="n_zeitplan"]').getBoundingClientRect();
				const b=document.querySelector('.ed-node[data-node-id="n_suche"]').getBoundingClientRect();
				return [Math.max(c.left+6,a.left-20),Math.max(c.top+6,a.top-20),Math.min(c.right-6,b.left+40),a.bottom+20];
			}`)
			from := touchPoint{box.Get("0").Num(), box.Get("1").Num()}
			to := touchPoint{box.Get("2").Num(), box.Get("3").Num()}
			s.touchAssertHit(`.ed-canvas`, from)
			if empty := page.MustEval(`(x,y)=>{const e=document.elementFromPoint(x,y);return !!e && !e.closest('.ed-node,.ed-edge,.ed-zoom,.ed-minimap,.ed-wire-actions')}`, from.x, from.y).Bool(); !empty {
				t.Fatalf("the start of the box (%.0f, %.0f) is on a card or a control", from.x, from.y)
			}
			view := page.MustEval(`()=>JSON.stringify(edFixture.editor().view)`).Str()
			s.touchDrag(from, to, 8)
			s.shot("touch-box-select-" + vp.name)
			s.wait(`()=>{const sel=edFixture.editor().selection;return sel.size===2 && sel.has('n_zeitplan') && sel.has('n_suche')}`)
			if now := page.MustEval(`()=>JSON.stringify(edFixture.editor().view)`).Str(); now != view {
				t.Fatalf("the view moved during the box select: %s -> %s", view, now)
			}
			s.failOnPageErrors("touch box select")
		})
	}
}
```

- [ ] **Step 2: Run it and record the failure.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchBoxSelect' -count=1 -v -timeout 10m 2>&1 | grep -v "^=== " | head -10
```

Expected: `--- FAIL: TestEasyDragTouchBoxSelect/1024x768` and `.../390x844`, each `condition did not become true: ()=>{const sel=edFixture.editor().selection;return sel.size===2 ...`; the screenshot `touch-box-select-*.png` shows the flow moved sideways instead of a box. Copy the two failure lines and the observed view change into the D.7 report. A failure at `the start of the box ... is on a card or a control` is a test problem: adjust the start to the nearest empty point (`touchEmptyPoint`), not the product.

**Branch: it works.** If both subtests pass, there is no finding F2 (the code reading was wrong); remove the skip constant, keep the test, and say so in D.7.

- [ ] **Step 3: Commit it skipped.** Add the skip as the first statement after the gate in `TestEasyDragTouchBoxSelect`:

```go
	requirePrecisionBrowserSmoke(t)
	t.Skip(easyDragTouchBoxSelectSkip)
```

Run it again:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouchBoxSelect' -count=1 -v -timeout 10m 2>&1 | grep -v "^=== " | head -6
```

Expected: `--- SKIP: TestEasyDragTouchBoxSelect` with the text `Wave 1: a one-finger drag on empty canvas pans the view ...`. Then write `C:/disposable/easydrag-s0c/d6-message.txt`:

```
test(easydrag): box select by touch, skipped until Wave 1

A one-finger drag on empty canvas pans the view, so no card is selected by finger
at 1024x768 or 390x844. The test states the pass condition and is skipped with the
Wave 1 ledger text; Wave 1 builds a touch select mode and removes the skip.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && gofmt -l ui/desktop_easydrag_touch_browser_test.go && GOTMPDIR='D:\tmp\go-build' go vet ./ui && git add ui/desktop_easydrag_touch_browser_test.go && git commit -F C:/disposable/easydrag-s0c/d6-message.txt
```

Expected: no output from `gofmt` and `go vet`; one commit.

---

### Task D.7: The final run and the report for the coordinator

**Files:** none (no commit). **This part does not edit the spec or the roadmap.** The ledger rows below, the measured numbers and the evidence go to the coordinator in the final report; the coordinator records them on main when the part lands.

- [ ] **Step 1: Run the whole touch suite together, then the EasyDrag tiers.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0c-shots GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragTouch' -count=1 -v -timeout 20m 2>&1 | grep -E "^(--- |\s+--- |PASS|FAIL|ok)" 
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 2>&1 | tail -3
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && node scripts/test-easydrag.mjs 2>&1 | tail -1 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-touch && git diff --stat main -- ui/js/desktop/apps/easydrag-canvas.js && git log --oneline main..HEAD
```

Expected: the touch run shows `PASS` for `TestEasyDragTouchHelper`, `TestEasyDragTouchInsertOnWire`, `TestEasyDragTouchDeleteWire`, `TestEasyDragTouchCardTools`, `TestEasyDragTouchMappingChip` (each with both subtests) and `SKIP` for `TestEasyDragTouchBoxSelect`; the non-browser run is `ok`; the Node scripts end with `all EasyDrag checks passed` and `PASS render baseline (unchanged)`; `easydrag-canvas.js` has no diff; `git log` lists five or six commits (D.1 to D.6). Also run the part's whole browser set once more together with Part B's shared browser (`-run '(?i)easydrag'` with `AURAGO_RUN_BROWSER_SMOKE=1`, `-timeout 30m`) to see that the touch emulation of one test does not leak into the next: the shared browser gives each test a fresh incognito context, and the touch override lives on the page.

- [ ] **Step 2: Write the report.** The final answer to the coordinator contains:
1. the branch and the commit list;
2. the findings table below, filled in with what the runs showed (replace "expected" by "observed", and name the failing wait of D.2 Step 2);
3. the ledger rows for spec §7 (the text below, with the final commit hashes);
4. any watch item the runs raised (below).

**Findings table (the prediction; update with what was observed).**

| # | Gesture | Viewports | Finding | Kind | Outcome |
|---|---|---|---|---|---|
| F1 | Insert on a wire; delete a wire | 1024×768, 390×844 | A tap on a wire did not select it (the touch was a pan start and its pointer capture took the click) and the hover actions were hidden 220 ms after the finger lifted; the wire stroke was 14 px and the buttons 24 px on screen | small | fixed in D.2 (`<hash>`); tests `TestEasyDragTouchInsertOnWire`, `TestEasyDragTouchDeleteWire` |
| F2 | Box select | 1024×768, 390×844 | a one-finger drag on empty canvas pans; no way to select several cards by finger | design | handed to Wave 1; `TestEasyDragTouchBoxSelect` is committed with `t.Skip` (D.6, `<hash>`) |
| (none) | Card tools | both | works: a tap selects the card, the tools show and measure 44 px; long press opens the card menu | works | `TestEasyDragTouchCardTools` PASS (D.4, `<hash>`; add "tools moved under the card for touch" here only if D.4 Step 3 ran) |
| (none) | Mapping chip | both | works: the row's `click` is the tap fallback | works | `TestEasyDragTouchMappingChip` PASS (D.5, `<hash>`) |

**Ledger rows for spec §7 (rows 17+),** in the table's columns (`# | Item | Outcome | Evidence or task`):

```
| 17 | Touch F1: tapping a wire did not select it and its Insert/Delete actions vanished 220 ms after the finger lifted; the wire hit stroke was 14 px and the buttons 24 px on screen (Part D, `easydrag-interact.js` startPan, `easydrag-wires.js`) | fixed (D.2) | `<hash>`; `TestEasyDragTouchInsertOnWire`, `TestEasyDragTouchDeleteWire` at 1024×768 and 390×844 |
| 18 | Touch F2: box select by finger. At 1024×768 and 390×844 a one-finger drag on empty canvas pans the view (`easydrag-interact.js` onPointerDown: touch and no card is a pan), so several cards cannot be selected by touch | handed to Wave 1 | Wave 1 task: a touch select mode (for example a toolbar toggle or a long press on empty canvas, then drag) that starts `startBoxSelect` for touch pointers; then remove the `t.Skip` in `TestEasyDragTouchBoxSelect` (`<hash>`) |
| 19 | Touch: card tools by tap, mapping chip by the tap fallback, long-press card menu | closed (works) | `TestEasyDragTouchCardTools`, `TestEasyDragTouchMappingChip` PASS at both viewports (`<hash>`, `<hash>`) |
```

(If D.4 Step 3 ran, row 19 becomes "fixed (D.4)" with the CSS commit named. If D.2 Step 2 passed on today's code, row 17 becomes "closed (works)" with D.2 and D.3 as evidence. If D.6 Step 2 passed, row 18 becomes "closed (works)".)

**Watch items to report if the runs show them (they are observations, not tasks):**
- **Invisible port-add buttons.** `.ed-port-add` is `opacity: 0` until the card is hovered or selected but keeps `pointer-events`, and is 44 px under a coarse pointer. At 390 px it overlaps the first third of the wire leaving an output port, so `touchEdgePoint` takes a point at 60 % or 70 % of the wire (the helper logs when it does). A finger on that part of the wire opens quick-add from the port instead of selecting the wire. If `-v` output shows the "middle is covered" log line at 390 px, report it as a Wave 1 polish item (give `.ed-port-add` `pointer-events: none` until its card is selected).
- **Fit at 390 px hides the card tools** (`.ed-canvas.is-zoomed-out .ed-node-tools { visibility: hidden }`, below 70 % zoom). Not tested: no gesture of the table reaches it; the phone work in Wave 1 decides whether tools should survive a zoomed-out view.
- **Touch emulation is not a real finger** (spec §11): CDP touch events skip Chrome's gesture heuristics (scroll intervention, double-tap zoom). The tests find missing paths, not feel.

**Evidence to hand over:** the screenshots in `D:/tmp/easydrag-s0c-shots` (`touch-wire-actions-*`, `touch-wire-inserted-*`, `touch-wire-deleted-*`, `touch-card-tools-*`, `touch-card-menu-*`, `touch-mapping-row-*`, `touch-mapping-inserted-*`, `touch-box-select-*` from the red run), and the line counts of `easydrag-wires.js` and `easydrag-interact.js` after the fix.

---

## Coverage: spec item → task

| Spec item | Task |
|---|---|
| §6.1 touch helper (device metrics, touch emulation with 5 points, `touchTap`, `touchDrag`, `touchLongPress`) | D.1 |
| §6.2 insert a step on a wire | D.2 (test and fix F1) |
| §6.2 delete a wire | D.3 |
| §6.2 card tools | D.4 |
| §6.2 mapping chip into a field by the tap fallback | D.5 |
| §6.2 box select | D.6 (skipped, Wave 1) |
| §6.3 small findings fixed with the gesture test as the test | D.2 (F1); D.4 Step 3 and D.5 Step 2 are the prepared branches if the tests disagree with the prediction |
| §6.3 design findings skipped with a ledger reference | D.6 (F2) |
| §7 rows 17+ | D.7 |
| §8 browser tests: the five gestures at two viewports | D.2 to D.6 |
| §10 "five touch gestures green at both viewports, or skipped with a Wave 1 ledger row" | D.7 Step 1 |
| §10 "canvas.js has not grown" | D.7 Step 1 (`git diff --stat`) |
| §2 "No S0c change adds a line to canvas.js" | every task; D.2 puts the fix into `easydrag-wires.js`, `easydrag-interact.js` and CSS |

## Deviations from the spec

1. **Wire selection at 390 px is not a design finding.** Spec §6.3 lists "wire selection at 390 px" as a likely design finding for Wave 1. Reading the code, a tap on a wire fails for a mechanical reason (a touch on a wire is a pan start whose pointer capture swallows the wire's own click, and the hover-driven actions hide 220 ms after the finger lifts) and the existing selection, actions and insert popover only need a touch path. D.2 fixes it now (about 35 changed lines in two modules plus 10 lines of CSS) and not through `easydrag-canvas.js`. Only box select is handed to Wave 1. Reason: the spec's rule is that a finding that is a missing path is small; keeping it for Wave 1 would hand a sized, understood fix to a wave that has design work to do.
2. **Six test functions, not five.** Spec §6.2 names five gestures; D adds `TestEasyDragTouchHelper` (a check of the helper at both viewports, so a failing gesture test points at the product and not at the helper), and D.4 includes the long press (the only use of `touchLongPress`, and the way a card menu opens by touch). The delete-a-wire gesture has its own test in D.3 because it has its own row in §6.2.
3. **The card-tools and box-select tests start from the trigger and the first two cards, and the wire tests from `e_1`.** At 390 px only the first wire and the first one and a half cards are on screen at the automatic view. The tests use what is visible rather than panning, because a pan would make the gesture test depend on a second gesture.
4. **Mapping chip: the test taps the PDF step's `content` field, not the Telegram step's `message` field** that `TestDesktopEasyDragBrowser` uses, because the Telegram step exists only after that test's earlier journey. The step under test is in the seeded flow.
5. **Open point: the baseline pattern.** The brief suggests `TestDesktopEasyDrag`; the real browser tests also include `TestConfigEasyDragDurableSwitchBrowser`, so the baseline uses `(?i)easydrag`.
