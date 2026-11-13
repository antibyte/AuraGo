[← Plan index](README.md) · Spec: [plans/2026-11-13-easydrag-w1a-spec.md](../2026-11-13-easydrag-w1a-spec.md)

## Part E: Lab, first scenes (FD-14)

**Goal:** `TestEasyDragLabBrowser` renders six scenes in every theme context, measures the rendered colours, boxes and names in the page, and fails with the theme, scene, element and measured value. It closes the W1a gates "OKLab token checks across the 14 themes" and the lab's share of the acceptance list (spec §10, lab and contact sheet).

**Architecture:**
- The page JS (`labMeasureJS`) returns JSON only: rendered colours, boxes, `scrollWidth`/`clientWidth` and accessible names.
- All maths is in Go (`ui/desktop_easydrag_lab_test.go`), on the helpers that already exist: `cssContrastRatio` (`desktop_theme_css_contract_test.go:419`), `easyDragOKLab`, `easyDragOKLCH`, `easyDragOKLabDistance`, `easyDragHueGap` and `easyDragS0StatusMin` (`desktop_easydrag_colour_test.go`). Nothing has to be extracted from the colour test, so its behaviour cannot change.
- The target scan is the existing `easyDragTargetScan` through `s.checkTargets` (`desktop_easydrag_targets_browser_test.go:67`), exception list included.
- The lab lives **beside** the contact sheet (recommendation at the end of this part). It reuses `easyDragContactRunDoneJS`, and the contact sheet is not edited.
- The pure maths and the failure texts get one T0 test, `TestEasyDragLabFindings`, so the lab's failure format is proven without a browser.

**Tech stack:** Go test with rod, through `startEasyDragSmoke` only. No new JS module, no locale key, so no key file and no `module-loader.js` change. Part E touches product code only in E.5.

**Depends on:** Parts A, B, C and D on main. Spec §7 and §11.

**In-page names.** The brief writes `ED.status` and `ED.kit`. In the page they are `window.EasyDrag.status` and `window.EasyDrag.kit` (`easydrag-durable.js:5`: `const ED = window.EasyDrag = ...`). The lab code uses `window.EasyDrag`.

**Shell.** Git Bash. Every command starts with `cd <worktree> &&`. Go commands carry `GOTOOLCHAIN=go1.27.4` (the 1.27.3 `vet.exe` goes missing from time to time) and `GOTMPDIR='D:\tmp\go-build'` (backslashes). Browser runs use `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots` and the Bash sandbox disabled (Chrome cannot start from a test binary under the repo; memory note `reference_chrome_launch_from_agent.md`).

Shorthands used below:

```
WT   = C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab
GO   = GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build'
RUN  = AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots
```

Spell them out in the real commands; the shorthand is only for reading.

**Why one page per theme context.** `newEasyDragPage` opens the page with `.Timeout(150 * time.Second)` (`desktop_easydrag_browser_test.go:462`). In rod that is an absolute deadline from the moment the page is created, so one page for all contexts would die after 150 s. The lab therefore starts one page per context (a subtest each, about 12 s of work) and a second page for the pseudo run. A theme never leaks into the next one either.

---

### Preamble: worktree, contracts, baseline, DOM probe

**Files:** none changed. A scratch probe test is created and deleted again.

- [ ] **Step 1: Confirm A, B, C and D are on main**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo && git log --oneline -40 main | grep -i "w1a\|easydrag" | head -20
cd C:/Users/Andi/Documents/repo/AuraGo && git branch --list 'feat/easydrag-w1a-*' && git branch --merged main --list 'feat/easydrag-w1a-*'
```

Expected: the second command lists `feat/easydrag-w1a-status`, `-language`, `-rail` and `-kit` as merged into main. If one is missing, stop: Part E does not start before all four have landed (spec §9).

- [ ] **Step 2: Create the worktree and the scratch folders**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1a-lab -b feat/easydrag-w1a-lab main
mkdir -p D:/tmp/easydrag-w1a-shots D:/tmp/easydrag-w1a-contact/after D:/tmp/go-build C:/disposable/easydrag-w1a/keys
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git status --short | head -3 && git log --oneline -1
```

Expected: `Preparing worktree (new branch 'feat/easydrag-w1a-lab')`, an empty status and the commit of main's tip.

The tracked `AGENTS.md` assume-unchanged trap (memory `project_desktop_theme_system.md`) does not matter here, because the worktree is a fresh checkout.

- [ ] **Step 3: Check whether a bundle check needs `node_modules`** (only E.5 can change JS or CSS)

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && node scripts/build-ui-bundles.js --check 2>&1 | tail -3
```

- If it ends without an error (or says the bundles are current), nothing more is needed.
- If it fails with `Cannot find module`, copy the real folder (a junction breaks the Storymaker vendor check, memory `reference_worktree_node_modules.md`):

```bash
cp -r C:/Users/Andi/Documents/repo/AuraGo/node_modules C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab/node_modules
```

- [ ] **Step 4: Verify the contracts on main** (all greps run in the worktree)

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && ls ui/js/desktop/apps/easydrag-status.js ui/js/desktop/apps/easydrag-kit.js ui/js/desktop/apps/easydrag-format.js ui/js/desktop/apps/easydrag-icons.js ui/js/desktop/apps/easydrag-delight.js
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && grep -c "ed-status-disc\|ed-status-label\|ed-status-word" ui/css/desktop-app-easydrag.css
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && grep -n "\-\-ed-status-neutral\|\-\-ed-font-body:\|\-\-ed-radius-md:" ui/css/desktop-app-easydrag.css | head -5
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && grep -n "aurago.easydrag.pseudo" ui/js/desktop/apps/easydrag-format.js | head -3
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && grep -rn "ed-rail-caption" ui/js/desktop/apps/*.js ui/css/desktop-app-easydrag.css | head -4
```

Expected: the five files exist; the CSS count is 3 or more; the three tokens are found (name `--ed-radius-xl` instead of `--ed-radius-md`/`-lg` only if Part A reported that deviation); the pseudo switch is read in `easydrag-format.js`; `ed-rail-caption` appears in a JS file and in the CSS.

If a contract name differs, the report of that part decides; write the real name down and use it in the selectors of E.2 and E.3 (every selector sits in one place: `labStatusProbes`, `labSheetJS` and the scene functions).

Then read which character the pseudo wrapper brackets with, because E.4 asserts it:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && grep -n "pseudo" ui/js/desktop/apps/easydrag-format.js | head -12
```

Expected: a wrapper that returns `'[' + ... + ']'` (spec §5.4: "in brackets"). If it uses another bracket, change the `'['` in the E.4 assertion to that character.

- [ ] **Step 5: Baseline, T0 and one full gated browser run**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && npm run test:easydrag:t0 2>&1 | tail -8
```

Expected: ends with every step green (about 35-55 s). Anything red here is not Part E's; stop and report it.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-lab-baseline.log | tail -5
grep -c '^--- PASS' D:/tmp/easydrag-w1a-lab-baseline.log; grep -c '^--- SKIP' D:/tmp/easydrag-w1a-lab-baseline.log; grep -c '^--- FAIL' D:/tmp/easydrag-w1a-lab-baseline.log
```

Expected: `ok  	aurago/ui` and a FAIL count of 0 (6-8 minutes now that A to D added tests). Write the PASS and SKIP counts down: that is the baseline the final run of E.5 compares to. The final run must show exactly one more PASS (the lab) and the same SKIP count.

(Sandbox off for this command.)

- [ ] **Step 6: Probe the real DOM once** (scratch file, never committed)

The lab's selectors rest on the markup Parts A to D produced. This probe prints the facts the plan below depends on. Write `ui/zz_easydrag_lab_probe_browser_test.go` in the worktree:

```go
package ui

import "testing"

const labProbeJS = `()=>{
	const ED=window.EasyDrag,out={};
	out.modules=['status','kit','format','icons','delight','core'].map(k=>k+':'+typeof ED[k]).join(' ');
	out.values=ED.status?ED.status.VALUES.join(','):'none';
	const app=document.querySelector('.ed-app');
	const holder=document.createElement('div');holder.innerHTML=ED.status.disc('ok',18);
	const disc=holder.firstElementChild;app.appendChild(disc);
	const cs=getComputedStyle(disc);
	out.disc=disc.outerHTML.slice(0,420);out.discBg=cs.backgroundColor;out.discAria=disc.getAttribute('aria-label');
	disc.remove();
	const label=ED.status.label(window.t,'ok');out.labelType=typeof label;
	const c=ED.kit.callout({status:'ok',text:'probe'});
	out.calloutType=typeof c==='string'?'string':c.nodeName;
	const cn=typeof c==='string'?(()=>{const d=document.createElement('div');d.innerHTML=c;return d.firstElementChild;})():c;
	app.appendChild(cn);out.callout=cn.outerHTML.slice(0,500);out.calloutColor=getComputedStyle(cn).color;cn.remove();
	out.iconType=typeof ED.core.icon;
	out.captions=document.querySelectorAll('.ed-rail-caption').length;
	out.discsOnPage=document.querySelectorAll('.ed-app .ed-status-disc').length;
	out.nodeDiscs=document.querySelectorAll('.ed-node .ed-status-disc').length;
	out.tokens=['--ed-status-neutral','--ed-font-body','--ed-panel-strong','--ed-well-color'].map(k=>k+'='+getComputedStyle(app).getPropertyValue(k).trim()).join(' ');
	out.results=document.querySelector('.ed-run-results')?document.querySelector('.ed-run-results').outerHTML.slice(0,500):'none';
	out.disclosureHeads=document.querySelectorAll('.ed-disclosure-head').length;
	return JSON.stringify(out);
}`

func TestEasyDragLabProbe(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length>=1`)
	t.Logf("HOME   %s", p.MustEval(labProbeJS).Str())
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, p.MustEval(`()=>edFixture.seededID`).Str())
	s.wait(`()=>!!edFixture.editor() && document.querySelectorAll('.ed-node').length===4`)
	s.settle()
	t.Logf("EDITOR %s", p.MustEval(labProbeJS).Str())
	p.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.status==='success'}`)
	t.Logf("TESTED %s", p.MustEval(labProbeJS).Str())
}
```

Run it (sandbox off):

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabProbe$' -count=1 -v 2>&1 | grep -E "HOME|EDITOR|TESTED|FAIL|ok "
```

Read the three JSON lines and decide, for each fact:

| Fact | Expected | If different |
|---|---|---|
| `modules` | all six `object` | a missing module means a part did not land: stop |
| `disc` and `discBg` | the disc colour is the `background-color` of `.ed-status-disc` (not transparent), or the first svg shape of the disc is the filled circle | if the colour sits on a deeper element, change `labMarkColour` and the `svg` lookup in `labMeasureJS` (E.1) to read that element |
| `calloutType` | `DIV` or another element (the lab accepts a string as well) | none needed |
| `callout` | the text is inside the `.ed-callout` element itself or a child without its own colour | if a child span sets its own colour, change the `callout` probe selector in E.2 to that child |
| `captions` | 5 or more in `EDITOR`, 0 in `HOME` | `0` in `EDITOR`: the rail selector differs; take the name from `grep` in step 4 |
| `nodeDiscs` | 0 in `EDITOR` before a run, 4 after the run-done seed (E.3) | none needed |
| `results` and `disclosureHeads` | `TESTED` shows a visible `.ed-run-results` and 1 or more `.ed-disclosure-head` | if the panel has another class, change the `.ed-run-results` waits in E.3 and the `disclosure` probe |
| `tokens` | the status-neutral and font tokens are non-empty | empty means Part A or B is incomplete: stop |

Write the facts that differ into your notes, then delete the probe so it cannot reach a commit:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && rm ui/zz_easydrag_lab_probe_browser_test.go && git status --short | wc -l
```

Expected: `0`.

---

### Task E.1: Lab scaffolding (maths, runner, measure script, scene 6 as the first scene)

**Files:**
- Create: `ui/desktop_easydrag_lab_test.go` (types, maths, failure texts, unit test; runs in T0)
- Create: `ui/desktop_easydrag_lab_browser_test.go` (runner, theme switch, measure script, scene 6 "home")
- Test: `TestEasyDragLabFindings` (T0), `TestEasyDragLabBrowser` (T1)

Scene 6 (home) is the proof scene of the scaffolding: it needs no setup beyond the page and is the first thing a lab must show. E.3 therefore adds scenes 3 to 5 only.

- [ ] **Step 1: Write the failing unit test and the types it needs**

Create `ui/desktop_easydrag_lab_test.go`:

```go
package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// The lab (TestEasyDragLabBrowser) renders scenes in the page and measures there; every decision is made in this file,
// on the colour helpers of desktop_easydrag_colour_test.go and desktop_theme_css_contract_test.go.

const (
	labTextMin = 4.5 // status words and other text
	labMarkMin = 3.0 // discs, tiles' icons, group frames
)

// labKnown holds findings that were reported to the owner instead of fixed (spec §11: a material that blocks a surface
// token fix). The key is theme|scene|element|check, the value is the reason. A finding in this map is logged as
// LAB-KNOWN and does not fail the lab. The map starts empty and an entry without a reason fails TestEasyDragLabFindings.
var labKnown = map[string]string{}

// labProbe names a group of elements the page measures. Role decides the check:
//   - text: the element's text colour against what is behind it, 4.5:1;
//   - mark: the element's own colour (background, else its first svg shape) against what is behind it, 3:1;
//   - tile: the icon colour (the element's text colour) against the tile's own background, 3:1;
//   - edge: the left border colour against what is behind it, 3:1 (group frames);
//   - ref: only measured, used by the distance check.
//
// Min is the number of visible matches the scene must have, so a renamed class cannot make a scene pass empty.
type labProbe struct {
	Name     string `json:"name"`
	Selector string `json:"selector"`
	Role     string `json:"role"`
	Min      int    `json:"min"`
}

// labSpec is what the page measures in one scene.
type labSpec struct {
	Probes []labProbe `json:"probes"`
	Scope  string     `json:"scope"` // root selector for probes, names and clipping; "" means .ed-app
	Names  bool       `json:"names"`
	Clip   bool       `json:"clip"`
}

// labElement is one measured element. Colours are sRGB 0-255, already composited over what is behind them.
type labElement struct {
	Probe   string      `json:"probe"`
	Role    string      `json:"role"`
	Name    string      `json:"name"` // tag.class[status=..][data-ed..]@surface
	Text    string      `json:"text"`
	Status  string      `json:"status"`
	Surface string      `json:"surface"`
	Fg      *[3]float64 `json:"fg"`     // text colour over Eff
	Eff     *[3]float64 `json:"eff"`    // the element's own background over Behind
	Behind  *[3]float64 `json:"behind"` // the ancestors' backgrounds
	Own     *[3]float64 `json:"own"`    // the element's own background over Behind, nil when it has none
	Svg     *[3]float64 `json:"svg"`    // the first svg shape's fill (or stroke) over Behind
	Edge    *[3]float64 `json:"edge"`   // the left border over Behind
	Approx  bool        `json:"approx"` // a gradient or pattern lies under it; only background-color was read
}

type labBox struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type labClip struct {
	Name        string `json:"name"`
	Text        string `json:"text"`
	ScrollWidth int    `json:"sw"`
	ClientWidth int    `json:"cw"`
	Ellipsis    bool   `json:"ellipsis"`
	Title       bool   `json:"title"`
}

// labScene is the page's answer for one scene.
type labScene struct {
	Elements []labElement   `json:"elements"`
	Counts   map[string]int `json:"counts"`
	Unnamed  []labBox       `json:"unnamed"`
	Scanned  int            `json:"scanned"`
	Clipped  []labClip      `json:"clipped"`
}

type labFinding struct{ Theme, Scene, Element, Check, Value, Want string }

func (f labFinding) String() string {
	return fmt.Sprintf("LAB-FINDING theme=%s scene=%s element=%s check=%s value=%s want=%s", f.Theme, f.Scene, f.Element, f.Check, f.Value, f.Want)
}

func (f labFinding) Key() string { return f.Theme + "|" + f.Scene + "|" + f.Element + "|" + f.Check }

// labMarkColour is the colour a disc, dot or tile shows: its own background, else its svg fill.
func labMarkColour(e labElement) *[3]float64 {
	if e.Own != nil {
		return e.Own
	}
	return e.Svg
}

func labApproxNote(e labElement) string {
	if e.Approx {
		return " [a gradient or pattern lies under it; only its background-color was read]"
	}
	return ""
}

// labContrastFindings checks every measured element by its role and that each probe found its minimum.
func labContrastFindings(theme, scene string, probes []labProbe, els []labElement, counts map[string]int) []labFinding {
	var out []labFinding
	for _, p := range probes {
		if counts[p.Name] < p.Min {
			out = append(out, labFinding{theme, scene, "probe " + p.Name + " (" + p.Selector + ")", "present", fmt.Sprintf("%d visible", counts[p.Name]), fmt.Sprintf("at least %d", p.Min)})
		}
	}
	for _, e := range els {
		unmeasured := func(what string) {
			out = append(out, labFinding{theme, scene, e.Name, "measured", "no " + what, "a colour"})
		}
		ratio := func(check string, a, b [3]float64, need float64) {
			if r := cssContrastRatio(a, b); r < need {
				out = append(out, labFinding{theme, scene, e.Name, check, fmt.Sprintf("%.3f:1%s", r, labApproxNote(e)), fmt.Sprintf("at least %.1f:1", need)})
			}
		}
		switch e.Role {
		case "text":
			if e.Fg == nil || e.Eff == nil {
				unmeasured("text colour")
				continue
			}
			ratio("text-contrast", *e.Fg, *e.Eff, labTextMin)
		case "tile":
			if e.Fg == nil || e.Eff == nil {
				unmeasured("icon colour")
				continue
			}
			ratio("icon-on-tile-contrast", *e.Fg, *e.Eff, labMarkMin)
		case "mark":
			colour := labMarkColour(e)
			if colour == nil || e.Behind == nil {
				unmeasured("disc colour")
				continue
			}
			ratio("mark-contrast", *colour, *e.Behind, labMarkMin)
		case "edge":
			if e.Edge == nil || e.Behind == nil {
				unmeasured("frame colour")
				continue
			}
			ratio("frame-contrast", *e.Edge, *e.Behind, labMarkMin)
		}
	}
	return out
}

// labStatusRefs are the four coloured statuses a category colour must stay away from (spec §7). The grey statuses are
// left out on purpose: the logic and tool categories are grey by design (easyDragS0MarkSoft).
var labStatusRefs = map[string]bool{"ok": true, "failed": true, "running": true, "uncertain": true}

// labDistanceFindings keeps rendered category colours (probe "tile", their own colour; probe "group", their frame colour)
// 0.20 OKLab from every rendered status colour (probe "ref-status") and, for tiles with chroma, 30 degrees of hue.
func labDistanceFindings(theme, scene string, els []labElement) []labFinding {
	var statuses []labElement
	for _, e := range els {
		if e.Probe == "ref-status" && labStatusRefs[e.Status] && labMarkColour(e) != nil {
			statuses = append(statuses, e)
		}
	}
	var out []labFinding
	if len(statuses) < 4 {
		return append(out, labFinding{theme, scene, "probe ref-status", "present", fmt.Sprintf("%d coloured status discs", len(statuses)), "at least 4"})
	}
	for _, e := range els {
		var colour *[3]float64
		switch e.Probe {
		case "tile":
			colour = e.Own
		case "group":
			colour = e.Edge
		}
		if colour == nil {
			continue
		}
		_, chroma, hue := easyDragOKLCH(*colour)
		for _, s := range statuses {
			sc := *labMarkColour(s)
			if d := easyDragOKLabDistance(*colour, sc); d < easyDragS0StatusMin {
				out = append(out, labFinding{theme, scene, e.Name, "oklab-distance-to-status-" + s.Status, fmt.Sprintf("%.3f", d), "at least 0.20"})
			}
			if e.Probe == "tile" && chroma >= 0.05 {
				if _, _, sh := easyDragOKLCH(sc); easyDragHueGap(hue, sh) < easyDragS0MarkHueGapMin {
					out = append(out, labFinding{theme, scene, e.Name, "hue-gap-to-status-" + s.Status, fmt.Sprintf("%.0f degrees (hue %.0f against %.0f)", easyDragHueGap(hue, sh), hue, sh), "at least 30 degrees"})
				}
			}
		}
	}
	return out
}

func labNameFindings(theme, scene string, res labScene) []labFinding {
	var out []labFinding
	if res.Scanned == 0 {
		return []labFinding{{theme, scene, "name scan", "present", "0 controls scanned", "at least 1"}}
	}
	for _, b := range res.Unnamed {
		out = append(out, labFinding{theme, scene, b.Name, "accessible-name", "none (text " + fmt.Sprintf("%q", b.Text) + ")", "aria-label, label text, text content or role=img with aria-label"})
	}
	return out
}

func labClipFindings(theme, scene string, res labScene) []labFinding {
	var out []labFinding
	for _, c := range res.Clipped {
		out = append(out, labFinding{theme, scene, c.Name, "text-clipped", fmt.Sprintf("scrollWidth %d > clientWidth %d, ellipsis=%v, title=%v, text %q", c.ScrollWidth, c.ClientWidth, c.Ellipsis, c.Title, c.Text), "no overflow, or text-overflow: ellipsis plus a title"})
	}
	return out
}

// labSplit separates findings the owner accepted (labKnown) from the ones that fail the lab.
func labSplit(findings []labFinding) (failing, known []labFinding) {
	for _, f := range findings {
		if _, ok := labKnown[f.Key()]; ok {
			known = append(known, f)
		} else {
			failing = append(failing, f)
		}
	}
	return failing, known
}

// labRound keeps a measured number readable in logs.
func labRound(v float64) float64 { return math.Round(v*100) / 100 }

func TestEasyDragLabFindings(t *testing.T) {
	white, black, grey := &[3]float64{255, 255, 255}, &[3]float64{0, 0, 0}, &[3]float64{150, 150, 150}
	probes := []labProbe{{Name: "word", Selector: ".ed-status-word", Role: "text", Min: 1}}

	// a grey word on white is 2.96:1: one finding that names theme, scene, element and value
	bad := []labElement{{Probe: "word", Role: "text", Name: "span.ed-status-word[status=ok]@panel", Fg: grey, Eff: white, Behind: white}}
	got := labContrastFindings("amiga", "1-status", probes, bad, map[string]int{"word": 1})
	if len(got) != 1 {
		t.Fatalf("got %d findings %v, want 1", len(got), got)
	}
	text := got[0].String()
	for _, want := range []string{"theme=amiga", "scene=1-status", "span.ed-status-word[status=ok]@panel", "check=text-contrast", "2.9", "at least 4.5:1"} {
		if !strings.Contains(text, want) {
			t.Errorf("finding %q lacks %q", text, want)
		}
	}

	// black on white passes, and a probe that finds nothing is a finding
	good := []labElement{{Probe: "word", Role: "text", Name: "x", Fg: black, Eff: white, Behind: white}}
	if got := labContrastFindings("amiga", "1-status", probes, good, map[string]int{"word": 1}); len(got) != 0 {
		t.Errorf("black on white gave findings %v", got)
	}
	if got := labContrastFindings("amiga", "1-status", probes, nil, map[string]int{"word": 0}); len(got) != 1 || !strings.Contains(got[0].String(), "0 visible") {
		t.Errorf("an empty probe gave %v, want one finding naming 0 visible", got)
	}

	// a mark is read from its own colour, else its svg fill; 3:1 is the line (grey 150 on white is 2.96, 140 is 3.4)
	svgMark := []labElement{{Probe: "disc", Role: "mark", Name: "disc", Svg: &[3]float64{140, 140, 140}, Behind: white}}
	if got := labContrastFindings("t", "s", nil, svgMark, nil); len(got) != 0 {
		t.Errorf("a 3.4:1 svg disc gave findings %v", got)
	}
	weakMark := []labElement{{Probe: "disc", Role: "mark", Name: "disc", Own: grey, Behind: white}}
	if got := labContrastFindings("t", "s", nil, weakMark, nil); len(got) != 1 || got[0].Check != "mark-contrast" {
		t.Errorf("a 2.96:1 disc gave %v, want one mark-contrast finding", got)
	}

	// two identical colours break the OKLab distance; the grey statuses are not references
	blue := &[3]float64{40, 90, 220}
	els := []labElement{{Probe: "tile", Role: "tile", Name: "tile[cat=web]", Own: blue}}
	for _, v := range []string{"ok", "failed", "running", "uncertain"} {
		els = append(els, labElement{Probe: "ref-status", Role: "ref", Name: "disc " + v, Status: v, Own: blue})
	}
	if got := labDistanceFindings("t", "2", els); len(got) < 4 {
		t.Errorf("a tile equal to four statuses gave %d findings, want at least 4", len(got))
	}
	if got := labDistanceFindings("t", "2", els[:2]); len(got) != 1 || got[0].Check != "present" {
		t.Errorf("one status disc gave %v, want the 'present' finding", got)
	}

	// names and clipping
	if got := labNameFindings("t", "s", labScene{Scanned: 3, Unnamed: []labBox{{Name: "button.ed-icon-btn", Text: ""}}}); len(got) != 1 || got[0].Check != "accessible-name" {
		t.Errorf("an unnamed button gave %v", got)
	}
	if got := labNameFindings("t", "s", labScene{}); len(got) != 1 {
		t.Errorf("an empty scan gave %v, want one finding", got)
	}
	clip := labScene{Clipped: []labClip{{Name: "span.ed-rail-caption", Text: "[Vörlägén]", ScrollWidth: 80, ClientWidth: 60, Ellipsis: true}}}
	if got := labClipFindings("t", "s", clip); len(got) != 1 || !strings.Contains(got[0].Value, "scrollWidth 80 > clientWidth 60") {
		t.Errorf("a clipped caption gave %v", got)
	}

	// known findings are split off; an entry needs a reason
	f := labFinding{"amiga", "1-status", "x", "text-contrast", "1", "2"}
	labKnown[f.Key()] = "test"
	failing, known := labSplit([]labFinding{f})
	delete(labKnown, f.Key())
	if len(failing) != 0 || len(known) != 1 {
		t.Errorf("labSplit gave failing %v known %v", failing, known)
	}
	for key, reason := range labKnown {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("labKnown[%q] has no reason", key)
		}
	}
	if labRound(2.456) != 2.46 {
		t.Errorf("labRound(2.456) = %v", labRound(2.456))
	}
}
```

- [ ] **Step 2: Run it. It compiles and passes at once, because it tests the new maths only**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragLabFindings$' -count=1 -v 2>&1 | tail -6
```

Expected: `--- PASS: TestEasyDragLabFindings` and `ok  	aurago/ui`. (The failing-first rule is met by the browser test in step 3, which cannot compile before this file exists; if a check here fails, fix the maths before going on.)

- [ ] **Step 3: Write the runner and the measure script with scene 6**

Create `ui/desktop_easydrag_lab_browser_test.go`:

```go
package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestEasyDragLabBrowser is the EasyDrag conformance lab (FD-14, spec §7). For every theme context it renders the scenes
// of the lab in a page of its own, lets the page measure colours, boxes and names, and decides in Go. Failures read
// "LAB-FINDING theme=.. scene=.. element=.. check=.. value=.. want=..". The contact sheet
// (TestDesktopEasyDragContactSheetBrowser) stays beside it and keeps the screenshots behind AURAGO_EASYDRAG_CONTACT_SHEET.
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$' -count=1 -timeout 30m -v
func TestEasyDragLabBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	total := time.Now()
	for _, name := range labContexts(t) {
		name := name
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			r := newLabRun(t, name)
			r.scenes()
			r.report()
			t.Logf("lab %s: %s", name, time.Since(start).Round(100*time.Millisecond))
		})
	}
	t.Logf("lab total: %s", time.Since(total).Round(time.Second))
}

// labContexts are the 14 registry themes, with Fruity and Minimal in both schemes (16 contexts, as the colour test).
func labContexts(t *testing.T) []string {
	t.Helper()
	themes := parseDesktopThemeRegistry(t)
	if len(themes) != 14 {
		t.Fatalf("the theme registry lists %d themes, want 14", len(themes))
	}
	var names []string
	for _, theme := range themes {
		switch theme.ID {
		case "fruity", "minimal":
			names = append(names, theme.ID+"-light", theme.ID+"-dark")
		default:
			names = append(names, theme.ID)
		}
	}
	return names
}

type labRun struct {
	s        *easyDragSmoke
	theme    string
	findings []labFinding
}

func newLabRun(t *testing.T, name string) *labRun {
	t.Helper()
	r := &labRun{s: startEasyDragSmoke(t), theme: name}
	r.applyTheme(name)
	return r
}

// labApplyThemeJS applies a context. fixtureTheme knows the registry ids and fruity-light/-dark; Minimal's scheme is a
// setting of its own (the contact sheet sets it the same way).
const labApplyThemeJS = `name=>{
	if(name.startsWith('minimal')){
		fixtureTheme('minimal');
		Object.assign(aurora.state.bootstrap.settings,{'appearance.minimal_mode':name.endsWith('dark')?'dark':'light'});
		aurora.applyDesktopSettings();
		return;
	}
	fixtureTheme(name);
}`

func (r *labRun) applyTheme(name string) {
	r.s.t.Helper()
	r.s.page.MustEval(labApplyThemeJS, name)
	base := strings.TrimSuffix(strings.TrimSuffix(name, "-light"), "-dark")
	scheme := ""
	switch {
	case strings.HasSuffix(name, "-dark"):
		scheme = "dark"
	case strings.HasSuffix(name, "-light"):
		scheme = "light"
	}
	r.s.wait(fmt.Sprintf(`()=>{const b=document.querySelector('.desktop-body');return b.dataset.theme===%q && (%q==='' || (b.dataset.scheme==='dark')===(%q==='dark'))}`, base, scheme, scheme))
}

// scenes runs the scenes in an order that needs no navigation back: home first, then the sheets laid over it, then the
// editor scenes. Each task of Part E adds its lines here.
func (r *labRun) scenes() {
	r.sceneHome()
}

func (r *labRun) add(f []labFinding) { r.findings = append(r.findings, f...) }

// report fails the subtest for every finding that is not accepted in labKnown.
func (r *labRun) report() {
	r.s.t.Helper()
	failing, known := labSplit(r.findings)
	for _, f := range known {
		r.s.t.Logf("LAB-KNOWN %s reason=%q", f, labKnown[f.Key()])
	}
	for _, f := range failing {
		r.s.t.Error(f.String())
	}
	r.s.failOnPageErrors("lab " + r.theme)
}

// measure asks the page for the colours, names and clipping of one scene.
func (r *labRun) measure(spec labSpec) labScene {
	r.s.t.Helper()
	var res labScene
	if err := json.Unmarshal([]byte(r.s.page.MustEval(labMeasureJS, spec).Str()), &res); err != nil {
		r.s.t.Fatalf("%s: read the lab measurement: %v", r.theme, err)
	}
	return res
}

// check measures a scene and records the contrast, name and clipping findings its spec asks for.
func (r *labRun) check(scene string, spec labSpec) labScene {
	r.s.t.Helper()
	res := r.measure(spec)
	r.add(labContrastFindings(r.theme, scene, spec.Probes, res.Elements, res.Counts))
	if spec.Names {
		r.add(labNameFindings(r.theme, scene, res))
	}
	if spec.Clip {
		r.add(labClipFindings(r.theme, scene, res))
	}
	if dir := r.s.dir; dir != "" && labShots() {
		r.s.shot("lab-" + r.theme + "-" + scene)
	}
	return res
}

// labStatusProbes are the status elements any scene may show. minDisc is the number of status discs the scene must have
// outside callouts.
func labStatusProbes(minDisc int) []labProbe {
	return []labProbe{
		{Name: "disc", Selector: ".ed-status-disc:not(.ed-callout *)", Role: "mark", Min: minDisc},
		{Name: "word", Selector: ".ed-status-word", Role: "text"},
		{Name: "callout", Selector: ".ed-callout", Role: "text"},
		{Name: "callout-disc", Selector: ".ed-callout .ed-status-disc", Role: "mark"},
		{Name: "tile", Selector: ".ed-node-tile, .ed-palette-tile, .ed-detail-tile", Role: "tile"},
	}
}

// labMeasureJS measures one scene. spec is {probes:[{name,selector,role}], scope, names, clip}. It never decides; it
// returns what the page renders: colours composited over their backgrounds, the accessible names that are missing and
// the text elements whose content overflows their box.
//
// Colours: Chrome's computed colours come as rgb(), rgba() or color(srgb ...) (for color-mix); anything but rgb() goes
// through a 1px canvas. "Behind" is the ancestors' background-color layers composited from the nearest opaque one
// outwards; a gradient or pattern on the way sets approx, because only its colour is read. Opacity of the element and its
// ancestors is applied to its own colours.
const labMeasureJS = `spec=>{
	const cv=document.createElement('canvas');cv.width=cv.height=1;
	const cx=cv.getContext('2d',{willReadFrequently:true});
	const parse=css=>{
		if(!css||css==='transparent')return [0,0,0,0];
		const m=css.match(/^rgba?\(([^)]+)\)$/);
		if(m){
			const p=m[1].split(/[\s,\/]+/).filter(Boolean).map(x=>x.endsWith('%')?parseFloat(x)/100:Number(x));
			return [p[0],p[1],p[2],p.length>3?p[3]:1];
		}
		cx.clearRect(0,0,1,1);cx.fillStyle='#fe01fd';cx.fillStyle=css;
		if(cx.fillStyle==='#fe01fd')return null;
		cx.fillRect(0,0,1,1);
		const d=cx.getImageData(0,0,1,1).data;
		return d[3]===0?[0,0,0,0]:[d[0],d[1],d[2],d[3]/255];
	};
	const over=(top,under,k)=>{const a=top[3]*(k===undefined?1:k);return [0,1,2].map(i=>top[i]*a+under[i]*(1-a));};
	const round=a=>a&&a.map(v=>Math.round(v*10)/10);
	const vis=el=>{const cs=getComputedStyle(el),r=el.getBoundingClientRect();return cs.display!=='none'&&cs.visibility!=='hidden'&&(r.width>0||r.height>0);};
	const opacity=el=>{let o=1;for(let n=el;n;n=n.parentElement)o*=parseFloat(getComputedStyle(n).opacity);return o;};
	const behind=el=>{
		const layers=[];let approx=false;
		for(let n=el.parentElement;n;n=n.parentElement){
			const cs=getComputedStyle(n),c=parse(cs.backgroundColor);
			if(cs.backgroundImage!=='none')approx=true;
			if(c&&c[3]>0){layers.push(c);if(c[3]>=1)break;}
		}
		let acc=[255,255,255];
		for(let i=layers.length-1;i>=0;i--)acc=over(layers[i],acc);
		return {rgb:acc,approx:approx};
	};
	const describe=el=>{
		const cls=typeof el.className==='string'?el.className.trim().split(/\s+/).filter(Boolean):[];
		const data=[...el.attributes].map(a=>a.name).find(n=>n.startsWith('data-ed'))||'';
		const st=el.getAttribute('data-status'),surf=el.closest('[data-lab-surface]');
		return el.tagName.toLowerCase()+(cls.length?'.'+cls.join('.'):'')+(st?'[status='+st+']':'')+(data?'['+data+']':'')+(surf?'@'+surf.dataset.labSurface:'');
	};
	const name=el=>{
		const lb=el.getAttribute('aria-labelledby');
		if(lb){const s=lb.split(/\s+/).map(id=>{const n=document.getElementById(id);return n?n.textContent.trim():'';}).join(' ').trim();if(s)return s;}
		const al=(el.getAttribute('aria-label')||'').trim();if(al)return al;
		if(el.matches('input,select,textarea')&&el.labels&&[...el.labels].some(l=>l.textContent.trim()))return 'label';
		const tx=(el.matches('input[type="button"],input[type="submit"]')?el.value:el.textContent||'').trim();if(tx)return tx;
		const img=el.querySelector('img[alt]:not([alt=""])');if(img)return img.alt;
		const ti=el.querySelector('svg title');if(ti&&ti.textContent.trim())return ti.textContent.trim();
		return (el.getAttribute('title')||'').trim();
	};
	const scope=spec.scope||'.ed-app';
	const roots=[...document.querySelectorAll(scope)];
	const out={elements:[],counts:{},unnamed:[],scanned:0,clipped:[]};
	(spec.probes||[]).forEach(p=>{
		let n=0;
		roots.forEach(root=>root.querySelectorAll(p.selector).forEach(el=>{
			if(!vis(el))return;
			if(p.role==='text'&&(el.disabled||el.getAttribute('aria-disabled')==='true'))return;
			n++;
			const cs=getComputedStyle(el),op=opacity(el),b=behind(el);
			const own=parse(cs.backgroundColor),ownOver=own&&own[3]>0?over(own,b.rgb,op):null;
			const eff=ownOver||b.rgb,fgc=parse(cs.color);
			let svg=null;
			const host=el.matches('svg')?el:el.querySelector('svg');
			const shape=host&&host.querySelector('circle,path,rect,polygon,ellipse');
			if(shape){
				const sc=getComputedStyle(shape),f=parse(sc.fill),s=parse(sc.stroke);
				const pick=f&&f[3]>0?f:(s&&s[3]>0?s:null);
				if(pick)svg=over(pick,b.rgb,op);
			}
			let edge=null;
			if(parseFloat(cs.borderLeftWidth)>0){const c=parse(cs.borderLeftColor);if(c&&c[3]>0)edge=over(c,b.rgb,op);}
			const surf=el.closest('[data-lab-surface]');
			out.elements.push({probe:p.name,role:p.role,name:describe(el),text:(el.textContent||'').trim().slice(0,40),
				status:el.getAttribute('data-status')||'',cat:el.getAttribute('data-cat')||'',surface:surf?surf.dataset.labSurface:'',
				fg:fgc?round(over(fgc,eff,op)):null,eff:round(eff),behind:round(b.rgb),own:round(ownOver),svg:round(svg),edge:round(edge),approx:b.approx});
		}));
		out.counts[p.name]=n;
	});
	if(spec.names){
		const seen=new Set();
		roots.forEach(root=>root.querySelectorAll('button,a[href],input:not([type="hidden"]),select,textarea,[role="button"],.ed-status-disc').forEach(el=>{
			if(seen.has(el)||!vis(el))return;
			seen.add(el);out.scanned++;
			if(!name(el))out.unnamed.push({name:describe(el),text:(el.textContent||'').trim().slice(0,40)});
		}));
	}
	if(spec.clip){
		roots.forEach(root=>root.querySelectorAll('*').forEach(el=>{
			if(el.matches('input,textarea,select,svg,svg *,script,style')||!vis(el))return;
			const cs=getComputedStyle(el);
			if(cs.display==='inline'||cs.display==='contents'||!el.clientWidth)return;
			if(![...el.childNodes].some(n=>n.nodeType===3&&n.textContent.trim()))return;
			if(el.scrollWidth<=el.clientWidth+1)return;
			if(/auto|scroll/.test(cs.overflowX))return;
			const hasTitle=!!(el.getAttribute('title')||'').trim(),ell=cs.textOverflow==='ellipsis';
			if(ell&&hasTitle)return;
			out.clipped.push({name:describe(el),text:(el.textContent||'').trim().slice(0,40),sw:el.scrollWidth,cw:el.clientWidth,ellipsis:ell,title:hasTitle});
		}));
	}
	return JSON.stringify(out);
}`

// labShots reports whether the lab writes its screenshots too (the contact sheet's switch).
func labShots() bool { return os.Getenv("AURAGO_EASYDRAG_CONTACT_SHEET") == "1" }

// sceneHome is scene 6: the start page.
func (r *labRun) sceneHome() {
	s := r.s
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length>=1 && document.querySelectorAll('.ed-template-card').length>=3`)
	s.clearToasts()
	r.check("6-home", labSpec{Names: true, Probes: labStatusProbes(1)})
	s.checkTargets(r.theme + "/6-home")
}
```

- [ ] **Step 4: Run the lab on one theme and read what it reports**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui 2>&1 | head -5
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^(standard|amiga)$' -count=1 -v 2>&1 | grep -E "^(=== RUN|--- |\s+---|ok|FAIL)|LAB-|lab " | head -40
```

Expected: `go vet` is silent. The run lists the two subtests, a `lab <theme>: <time>` line for each (about 10 s) and either PASS or `LAB-FINDING` lines that name theme, scene `6-home`, an element and a measured value. A finding at this point is a real result, not a scaffold error, if its element is a status disc or word on the home page. A scaffold error looks like `probe disc ... 0 visible` (selector) or `no disc colour` (colour lookup): fix the probe or the lookup with the facts of preamble step 6. Do not lower `Min` to make it pass unless the probe step showed that the home page really has no disc.

- [ ] **Step 5: Prove the lab can fail** (a lab that cannot fail is worthless)

Temporarily raise the threshold, run one theme, and revert:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && sed -i 's/labTextMin = 4.5/labTextMin = 21.5/' ui/desktop_easydrag_lab_test.go
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^standard$' -count=1 -v 2>&1 | grep -E "LAB-FINDING|^--- |FAIL" | head -6
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && sed -i 's/labTextMin = 21.5/labTextMin = 4.5/' ui/desktop_easydrag_lab_test.go && grep -n "labTextMin = " ui/desktop_easydrag_lab_test.go
```

Expected: at least one `LAB-FINDING theme=standard scene=6-home element=... check=text-contrast value=... want=at least 21.5:1` line and `--- FAIL`. (If the home page shows no status word, the finding is on `callout` or `disc` with `mark-contrast`: raise `labMarkMin` the same way.) The last command prints `labTextMin = 4.5`: the revert is done.

- [ ] **Step 6: Gate and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && gofmt -l ui/desktop_easydrag_lab_test.go ui/desktop_easydrag_lab_browser_test.go
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragLabFindings|BrowserTestsUseTheSmokeHelper)' -count=1 -v 2>&1 | grep -E "^--- |^ok|FAIL"
```

Expected: `gofmt -l` prints nothing, `go vet` is silent, and both tests show `--- PASS`. The guard `TestEasyDragBrowserTestsUseTheSmokeHelper` is green because the lab opens pages only through `startEasyDragSmoke`.

If the lab found real status failures in step 4, they do not block this commit: scaffolding is committed with the scenes it has, and E.5 handles findings. To keep the branch's lab green until then, run only the T0 test above for the gate; the T1 lab is judged by its scaffold errors being zero.

Write `C:/disposable/easydrag-w1a/e1-msg.txt` with the Write tool:

```
test(easydrag): conformance lab scaffolding with the home scene

TestEasyDragLabBrowser renders a scene per theme context (14 registry
themes, Fruity and Minimal in both schemes) in a page of its own, lets the
page measure rendered colours, boxes and names, and decides in Go on the
existing OKLab and contrast helpers. Failures read LAB-FINDING with theme,
scene, element, check, value and want. labKnown holds findings reported to
the owner instead of fixed (empty).

Scene 6 (home) is the first scene: status disc and word contrast, accessible
names, and the 24 px target scan with its exception list. TestEasyDragLabFindings
proves the failure texts and the maths in T0. The contact sheet is untouched.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git add ui/desktop_easydrag_lab_test.go ui/desktop_easydrag_lab_browser_test.go && git commit -F C:/disposable/easydrag-w1a/e1-msg.txt && git status --short | wc -l
```

Expected: one commit with two files and `0` changed files left.

---

### Task E.2: Scenes 1 and 2 (status sheet, category tiles and group colours)

**Files:**
- Modify: `ui/desktop_easydrag_lab_browser_test.go` (`scenes()` and two new scene functions plus `labSheetJS`)
- Test: `TestEasyDragLabBrowser` (T1)

**Design.** A sheet is a `div.ed-lab-sheet` appended inside the open `.ed-app`, `position: fixed; inset: 0`, so every EasyDrag token resolves as in the real app. It is laid out in three columns, one per surface, each with `data-lab-surface`:
- `panel`: `var(--ed-panel-strong)`, the card colour that panels, popovers and cards use;
- `canvas`: `var(--ed-bg)`, the app background;
- `well`: `var(--ed-well-color)`, the editor canvas.

These are the same three opaque surfaces `easyDragS0Surfaces` reads statically (spec §7 names panel and canvas; the well is the third, because group frames and cards sit on it). The sheet is removed after the scene so the later editor scenes are not covered.

- [ ] **Step 1: Add the failing scenes**

In `ui/desktop_easydrag_lab_browser_test.go`, change `scenes()`:

Old:
```go
func (r *labRun) scenes() {
	r.sceneHome()
}
```

New:
```go
func (r *labRun) scenes() {
	r.sceneHome()
	r.sceneStatusSheet()
	r.sceneCategorySheet()
}
```

Append to the file:

```go
// labSheetJS lays a lab sheet over the open EasyDrag window. kind is "status" or "categories". The sheet is a real child
// of .ed-app, so the tokens, the status classes and the kit render as in the product. data-lab-surface names the
// surface a column draws on; the measure script reports it in every element name.
const labSheetJS = `(kind,args)=>{
	const ED=window.EasyDrag,tr=window.t,app=document.querySelector('.ed-app');
	const toEl=x=>{if(typeof x!=='string')return x;const d=document.createElement('div');d.innerHTML=x.trim();return d.firstElementChild;};
	const old=app.querySelector('.ed-lab-sheet');if(old)old.remove();
	const sheet=document.createElement('div');sheet.className='ed-lab-sheet';
	sheet.style.cssText='position:fixed;inset:0;z-index:99999;overflow:auto;display:flex;flex-wrap:wrap;align-content:flex-start;background:var(--ed-bg);color:var(--ed-text);font:13px/1.4 system-ui,sans-serif';
	const surfaces={panel:'var(--ed-panel-strong)',canvas:'var(--ed-bg)',well:'var(--ed-well-color)'};
	const section=name=>{const s=document.createElement('section');s.dataset.labSurface=name;s.style.cssText='box-sizing:border-box;width:33.33%;min-width:280px;padding:12px;background:'+surfaces[name]+';color:var(--ed-text)';sheet.appendChild(s);return s;};
	const row=(parent,node)=>{const r=document.createElement('div');r.style.cssText='display:flex;align-items:center;gap:8px;padding:3px 0';r.appendChild(toEl(node));parent.appendChild(r);};
	if(kind==='status'){
		Object.keys(surfaces).forEach(name=>{
			const s=section(name);
			ED.status.VALUES.forEach(v=>row(s,ED.status.label(tr,v)));
			if(name!=='well')['ok','failed','running','uncertain','skipped'].forEach(v=>row(s,ED.kit.callout({status:v,text:ED.status.word(tr,v)})));
		});
	}
	if(kind==='categories'){
		Object.keys(surfaces).forEach(name=>{
			const s=section(name);
			const tiles=document.createElement('div');tiles.style.cssText='display:flex;flex-wrap:wrap;gap:8px';
			args.cats.forEach(c=>{const tile=document.createElement('span');tile.className='ed-tile';tile.dataset.cat=c;tile.innerHTML=ED.core.icon('tool');tiles.appendChild(tile);});
			s.appendChild(tiles);
			if(name==='well'){
				const g=document.createElement('div');g.style.cssText='display:flex;flex-wrap:wrap;gap:8px;margin-top:12px';
				args.groups.forEach(n=>{const f=document.createElement('div');f.className='ed-group-frame'+(n==='neutral'?'':' ed-group--'+n);f.style.cssText='position:static;width:84px;height:28px';f.title=n;g.appendChild(f);});
				s.appendChild(g);
			}
			if(name==='panel'){
				const refs=document.createElement('div');refs.dataset.labRef='1';refs.style.cssText='display:flex;gap:8px;margin-top:12px';
				['ok','failed','running','uncertain'].forEach(v=>refs.appendChild(toEl(ED.status.disc(v,18))));
				s.appendChild(refs);
			}
		});
	}
	app.appendChild(sheet);
	return sheet.querySelectorAll('*').length;
}`

const labSheetOffJS = `()=>{document.querySelectorAll('.ed-lab-sheet').forEach(n=>n.remove());}`

// sceneStatusSheet is scene 1: every status value as a disc plus word on the panel, canvas and well surfaces, and a
// callout per status colour (ok, failed, running, uncertain and the grey skipped) on the panel and canvas surfaces.
// 12 values on three surfaces make 36 discs and 36 words; 5 callouts on two surfaces make 10.
func (r *labRun) sceneStatusSheet() {
	s := r.s
	s.page.MustEval(labSheetJS, "status", nil)
	r.check("1-status-sheet", labSpec{Scope: ".ed-lab-sheet", Names: true, Probes: []labProbe{
		{Name: "disc", Selector: ".ed-status-disc:not(.ed-callout *)", Role: "mark", Min: 36},
		{Name: "word", Selector: ".ed-status-word", Role: "text", Min: 36},
		{Name: "callout", Selector: ".ed-callout", Role: "text", Min: 10},
		{Name: "callout-disc", Selector: ".ed-callout .ed-status-disc", Role: "mark", Min: 10},
	}})
	s.page.MustEval(labSheetOffJS)
}

// sceneCategorySheet is scene 2: the nine category tiles on the three surfaces (27), the six group frames on the well,
// and a row of the four coloured status discs for the distance check. Tiles must keep their icon at 3:1 on the tile,
// group frames must reach 3:1 on the well, and every category colour keeps 30 degrees of hue (tiles) and 0.20 OKLab
// from the rendered status colours.
func (r *labRun) sceneCategorySheet() {
	s := r.s
	groups := append(append([]string{}, easyDragS0GroupNames...), "neutral")
	s.page.MustEval(labSheetJS, "categories", map[string]interface{}{"cats": easyDragS0Categories, "groups": groups})
	res := r.check("2-category-sheet", labSpec{Scope: ".ed-lab-sheet", Names: true, Probes: []labProbe{
		{Name: "tile", Selector: ".ed-tile", Role: "tile", Min: 27},
		{Name: "group", Selector: ".ed-group-frame", Role: "edge", Min: 6},
		{Name: "ref-status", Selector: "[data-lab-ref] .ed-status-disc", Role: "ref", Min: 4},
	}})
	r.add(labDistanceFindings(r.theme, "2-category-sheet", res.Elements))
	s.page.MustEval(labSheetOffJS)
}
```

- [ ] **Step 2: Run on two themes and read the result**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui 2>&1 | head -5
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^(standard|lollipop)$' -count=1 -v 2>&1 | grep -E "^--- |^\s+---|LAB-|ok|FAIL" | head -40
```

Expected: either PASS, or `LAB-FINDING` lines for scenes `1-status-sheet` and `2-category-sheet` with measured values. Scaffold errors to fix here, by their text:
- `probe disc ... 12 visible, want at least 36`: `ED.status.VALUES` has a different length, or the discs are not `.ed-status-disc`. Use the preamble probe's `values` and set the `Min` to `3 × len(VALUES)`.
- `probe callout ... 0 visible`: the callout class differs; use the probe's `callout` output.
- `no disc colour`: the disc's colour sits elsewhere; apply the table of preamble step 6.
- `probe ref-status ... 0 coloured status discs`: `labStatusRefs` keys do not match the `data-status` values; they must be `ok`, `failed`, `running`, `uncertain` (brief contract).

Look at the sheets once to be sure they show what the checks measure (screenshots only with the switch; sandbox off):

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_EASYDRAG_CONTACT_SHEET=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^lollipop$' -count=1 2>&1 | tail -2
```

Then Read `D:/tmp/easydrag-w1a-shots/lab-lollipop-1-status-sheet.png` and `lab-lollipop-2-category-sheet.png`: three columns, twelve discs with words per column, callouts under the first two, nine tiles per column, six frames in the third column and four discs at the bottom of the first.

- [ ] **Step 3: Prove the distance check can fail**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && sed -i 's/easyDragS0StatusMin     = 0.20/easyDragS0StatusMin     = 0.90/' ui/desktop_easydrag_colour_test.go
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^standard$' -count=1 -v 2>&1 | grep -E "oklab-distance" | head -3
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git checkout ui/desktop_easydrag_colour_test.go && git diff --stat ui/desktop_easydrag_colour_test.go | wc -l
```

Expected: `LAB-FINDING ... check=oklab-distance-to-status-... value=0.2xx want=at least 0.20` lines, and `0` for the last command: the colour test is back to its committed text. The lab reads `easyDragS0StatusMin` at run time, so the raised threshold shows the check can fail; the message prints the fixed text "at least 0.20", which is the real rule.

- [ ] **Step 4: Gate and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && gofmt -l ui/desktop_easydrag_lab_browser_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git status --short
```

Expected: no output from gofmt and vet; status shows only `M ui/desktop_easydrag_lab_browser_test.go`.

Write `C:/disposable/easydrag-w1a/e2-msg.txt`:

```
test(easydrag): lab scenes 1 and 2, status sheet and category tiles

Scene 1 lays every ED.status value as disc plus word on the panel, canvas
and well surfaces over the open EasyDrag window, plus one ED.kit.callout per
status colour on the panel and canvas surfaces. Words need 4.5:1, discs 3:1.

Scene 2 renders the nine category tiles on the three surfaces and the six
group frames on the well. Tile icons need 3:1 on the tile, frames 3:1 on the
well, and every rendered category colour keeps 0.20 OKLab (tiles also 30
degrees of hue) from the rendered ok, failed, running and uncertain discs.
The grey statuses are not references: logic and tool tiles are grey by design.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git add ui/desktop_easydrag_lab_browser_test.go && git commit -F C:/disposable/easydrag-w1a/e2-msg.txt
```

---

### Task E.3: Scenes 3 to 5 (idle editor with the rail, run done, test result)

**Files:**
- Modify: `ui/desktop_easydrag_lab_browser_test.go`
- Test: `TestEasyDragLabBrowser` (T1)

Scene 6 (home) came with E.1. The editor scenes follow the contact sheet's own steps: open the seeded flow through `EasyDragApp.open`, fit the view, seed a finished run with `easyDragContactRunDoneJS`.

Scene 5 runs a real test through the fixture backend (as `desktop_easydrag_decision_browser_test.go:35-39` does), so the results panel is the product's. Part B moved that panel to a disclosure; the scene needs one disclosure head. A callout is the one injected piece: the fixture's test result carries no uncertain effect, and spec §7 asks for a callout in this scene, so the scene appends one `ED.kit.callout` for `uncertain` into the results panel. This is stated in the scene's comment.

- [ ] **Step 1: Add the scenes**

Change `scenes()`:

Old:
```go
	r.sceneCategorySheet()
}
```

New:
```go
	r.sceneCategorySheet()
	r.openEditor()
	r.sceneIdleEditor()
	r.sceneRunDone()
	r.sceneTestResult()
}
```

Append:

```go
// openEditor opens the seeded flow in the editor and shows all of it (the contact sheet's steps).
func (r *labRun) openEditor() {
	s := r.s
	s.page.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, s.page.MustEval(`()=>edFixture.seededID`).Str())
	s.wait(`()=>!!edFixture.editor() && document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.fit()
	s.clearToasts()
}

// labRailProbe is the rail's captions (Part D): text on the rail surface at 4.5:1.
var labRailProbe = labProbe{Name: "rail-caption", Selector: ".ed-rail-caption", Role: "text", Min: 5}

// sceneIdleEditor is scene 3: the editor of the seeded flow with the labelled rail and nothing running.
func (r *labRun) sceneIdleEditor() {
	s := r.s
	s.wait(`()=>document.querySelectorAll('.ed-rail-caption').length>=5`)
	r.check("3-idle-editor", labSpec{Names: true, Probes: append(labStatusProbes(0), labRailProbe)})
	s.checkTargets(r.theme + "/3-idle-editor")
}

// sceneRunDone is scene 4: the same editor after a finished run, with a status disc on each of the four cards (the
// contact sheet's seed, easyDragContactRunDoneJS).
func (r *labRun) sceneRunDone() {
	s := r.s
	s.page.MustEval(easyDragContactRunDoneJS)
	s.wait(`()=>document.querySelectorAll('.ed-edge.is-delivered').length===3 && document.querySelectorAll('.ed-node .ed-status-disc').length>=4`)
	s.clearToasts()
	r.check("4-run-done", labSpec{Names: true, Probes: append(labStatusProbes(4), labRailProbe)})
	s.checkTargets(r.theme + "/4-run-done")
}

// labResultCalloutJS puts one uncertain callout into the test result panel. The fixture's run has no uncertain effect, so
// this is the one injected piece of scene 5; the disclosure and the status words are the product's.
const labResultCalloutJS = `()=>{
	const ED=window.EasyDrag,panel=document.querySelector('.ed-run-results');
	const c=ED.kit.callout({status:'uncertain',text:ED.status.word(window.t,'uncertain')});
	let el=c;if(typeof c==='string'){const d=document.createElement('div');d.innerHTML=c.trim();el=d.firstElementChild;}
	panel.appendChild(el);
}`

// sceneTestResult is scene 5: a real test run through the fixture backend, with the results panel open (disclosure) and
// a callout.
func (r *labRun) sceneTestResult() {
	s := r.s
	s.page.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	s.page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.id!=='run_contact' && ed.run.status==='success'}`)
	s.wait(`()=>{const p=document.querySelector('.ed-run-results');return !!p && !p.hidden && p.getClientRects().length>0}`)
	s.clearToasts()
	s.page.MustEval(labResultCalloutJS)
	r.check("5-test-result", labSpec{Names: true, Probes: append(labStatusProbes(1),
		labProbe{Name: "disclosure", Selector: ".ed-disclosure-head", Role: "text", Min: 1})})
	s.checkTargets(r.theme + "/5-test-result")
}
```

- [ ] **Step 2: Run on two themes**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui 2>&1 | head -5
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^(standard|fruity-dark)$' -count=1 -v 2>&1 | grep -E "^--- |^\s+---|LAB-|lab |ok|FAIL|condition did not" | head -50
```

Expected: PASS, or `LAB-FINDING` lines naming scenes `3-idle-editor`, `4-run-done`, `5-test-result`. Scaffold errors and what to do:
- `condition did not become true: ...ed-rail-caption...`: the caption class differs; use step 4 of the preamble.
- `condition did not become true: ...ed-node .ed-status-disc...`: the cards show no disc after a seeded run. Check the probe's `nodeDiscs` result; if it is 0 on a real card, that is **a Part A defect**, not a lab error: report it in the table of E.5 as "card badge not on `ED.status`" and keep the wait (do not weaken it).
- `condition did not become true: ...ed-run-results...`: apply the preamble probe's `results` row.
- `scene 5 ... probe disclosure ... 0 visible`: Part B's head class differs; use the probe's value. If the panel really has no disclosure after Part B, report it the same way.
- `checkTargets` text `the scan found only N controls`: the modal covers the window; add `s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)` before the check.

- [ ] **Step 3: Gate and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && gofmt -l ui/desktop_easydrag_lab_browser_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
```

Expected: no output.

Write `C:/disposable/easydrag-w1a/e3-msg.txt`:

```
test(easydrag): lab scenes 3 to 5, idle editor, run done, test result

Scene 3 is the idle editor with the labelled rail (captions at 4.5:1),
scene 4 the same editor after the contact sheet's run-done seed with a status
disc on every card, scene 5 a real fixture test run with the results panel
open: one disclosure head, status words and one injected uncertain callout.
Each scene checks the contrast of the status elements it shows, accessible
names on every button, link, input and status disc, and the 24 px target
scan with its exception list.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git add ui/desktop_easydrag_lab_browser_test.go && git commit -F C:/disposable/easydrag-w1a/e3-msg.txt
```

---

### Task E.4: The pseudo-locale run and the time budget

**Files:**
- Modify: `ui/desktop_easydrag_lab_browser_test.go`
- Test: `TestEasyDragLabBrowser` (T1)

**Design.** Part C's switch is read from `localStorage['aurago.easydrag.pseudo']`. The lab sets it on a fresh page and **reloads** that page, then re-opens EasyDrag. That works whether EasyDrag reads the switch at load or on every call, so the lab does not depend on where Part C reads it. The run is scene 3 again, with only the clipping check: no text element inside `.ed-app` has `scrollWidth > clientWidth` unless it has `text-overflow: ellipsis` and a `title`.

Exemptions, all in the page script and all stated in its comments: inline elements and `display: contents` (their `clientWidth` is 0), input-like elements and svg (they scroll or clip by themselves), and elements whose `overflow-x` is `auto` or `scroll` (scrolling by design, such as code blocks). Anything else that overflows is a finding.

- [ ] **Step 1: Add the pseudo run**

Add `"os"` is not needed here. Append to the file:

```go
// labPseudo is scene 3 with the pseudo-locale: every translated string comes back accented, about 35% longer and in
// brackets (spec §5.4), so a hard-coded width or a missing ellipsis shows up as a clipped text.
func labPseudo(t *testing.T, name string) {
	t.Helper()
	start := time.Now()
	s := startEasyDragSmoke(t)
	s.page.MustEval(`()=>localStorage.setItem('aurago.easydrag.pseudo','1')`)
	s.page.MustReload().MustWaitLoad()
	s.page.MustEval(`async()=>{await fixtureReady;await fixtureOpen('easydrag');}`)
	r := &labRun{s: s, theme: name + "+pseudo"}
	r.applyTheme(name)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length>=1`)
	s.clearToasts()
	r.openEditor()
	s.wait(`()=>document.querySelectorAll('.ed-rail-caption').length>=5`)
	// the switch must be on, or the run proves nothing: a caption is bracketed
	s.wait(`()=>document.querySelector('.ed-rail-caption').textContent.includes('[')`)
	r.check("3-idle-editor-pseudo", labSpec{Clip: true})
	r.report()
	t.Logf("lab %s pseudo: %s", name, time.Since(start).Round(100*time.Millisecond))
}
```

In `TestEasyDragLabBrowser`, change the loop body:

Old:
```go
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			r := newLabRun(t, name)
			r.scenes()
			r.report()
			t.Logf("lab %s: %s", name, time.Since(start).Round(100*time.Millisecond))
		})
```

New:
```go
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			r := newLabRun(t, name)
			r.scenes()
			r.report()
			t.Logf("lab %s: %s", name, time.Since(start).Round(100*time.Millisecond))
		})
		t.Run(name+"-pseudo", func(t *testing.T) { labPseudo(t, name) })
```

(If the preamble found a bracket other than `[`, use that character in the assertion.)

- [ ] **Step 2: Run one theme and look at the clip report**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui 2>&1 | head -3
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^standard-pseudo$' -count=1 -v 2>&1 | grep -E "^--- |^\s+---|LAB-|lab |ok|FAIL|condition did not" | head -30
```

Expected: PASS, or `LAB-FINDING ... check=text-clipped value=scrollWidth N > clientWidth M, ellipsis=..., title=..., text "[...]"`. A `condition did not become true` on the `'['` check means the pseudo switch is not active after the reload: check `localStorage` key and Part C's read (preamble step 4) before trusting any result.

Sanity check of the clip detector (prove it can fail): in the same run, the rail is the most likely hit. If the run is clean, force a hit once to see the finding:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && sed -i "s/if(el.scrollWidth<=el.clientWidth+1)return;/if(el.scrollWidth<=el.clientWidth-9999)return;/" ui/desktop_easydrag_lab_browser_test.go
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^standard-pseudo$' -count=1 -v 2>&1 | grep -c "text-clipped"
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && sed -i "s/if(el.scrollWidth<=el.clientWidth-9999)return;/if(el.scrollWidth<=el.clientWidth+1)return;/" ui/desktop_easydrag_lab_browser_test.go && grep -c "clientWidth+1)return" ui/desktop_easydrag_lab_browser_test.go
```

Expected: a count above 10 (every text element is now "clipped"), then `1` after the revert (the line is back). The hit is only for the detector; it is reverted before the commit.

- [ ] **Step 3: Measure the whole lab**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-lab-run1.log | grep -E "lab total|^--- (PASS|FAIL)|^ok|^FAIL"
grep -c "LAB-FINDING" D:/tmp/easydrag-w1a-lab-run1.log
```

Expected: `lab total: <time>` and the test line `--- PASS|FAIL: TestEasyDragLabBrowser (<seconds>s)`. Read `<seconds>`:
- At or below 240 s: the pseudo run stays inline. Record the number.
- Above 240 s (spec §7): split the pseudo run into its own test, as the spec says. Apply Step 3a. Record the number before and after.

The `LAB-FINDING` count is expected to be above 0 at this point: the findings are E.5's input. The lab may therefore be red when this task is committed. That is fine on this branch, because E.5 clears it; the commit gate for E.4 is that every failure is a `LAB-FINDING` (or `LAB-KNOWN`) line and none is a scaffold error (`probe ... visible`, `no ... colour`, `condition did not become true`, a panic or a page error).

- [ ] **Step 3a: Only if the lab took more than 240 s: split the pseudo run**

Remove the pseudo subtest from `TestEasyDragLabBrowser`:

Old:
```go
		t.Run(name+"-pseudo", func(t *testing.T) { labPseudo(t, name) })
```

New: delete that line.

Append this test:

```go
// TestEasyDragLabPseudoBrowser is the pseudo-locale run of the lab (scene 3 with localStorage['aurago.easydrag.pseudo']
// set), a test of its own because the whole lab took more than four minutes (spec §7).
func TestEasyDragLabPseudoBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, name := range labContexts(t) {
		name := name
		t.Run(name, func(t *testing.T) { labPseudo(t, name) })
	}
}
```

Run both:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run '(?i)TestEasyDragLab(Pseudo)?Browser$' -count=1 -timeout 30m -v 2>&1 | grep -E "^--- (PASS|FAIL)|^ok|^FAIL"
```

Expected: two top-level result lines. The final run of E.5 then expects two more PASS lines than the baseline, not one. Write that into the report.

- [ ] **Step 4: Gate and commit**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && gofmt -l ui/desktop_easydrag_lab_browser_test.go && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./ui
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragLabFindings|BrowserTestsUseTheSmokeHelper)' -count=1 2>&1 | tail -2
```

Expected: gofmt and vet silent, `ok  	aurago/ui`.

Write `C:/disposable/easydrag-w1a/e4-msg.txt` (use the second sentence of the body that matches what step 3 decided; delete the other):

```
test(easydrag): lab pseudo-locale run, clipping check

Scene 3 again with localStorage['aurago.easydrag.pseudo']='1' (page reloaded
so the switch holds wherever it is read): no text element inside .ed-app may
have scrollWidth > clientWidth unless it carries text-overflow: ellipsis and a
title. Inline elements, inputs, svg and overflow-x auto/scroll containers are
exempt, each in a comment of the measure script.

Measured: the lab takes <N> s for 16 contexts; the pseudo run stays inside
TestEasyDragLabBrowser (under the 4 minute budget of spec section 7).
Measured: the lab took <N> s with the pseudo run inside, over the 4 minute
budget of spec section 7, so the pseudo run is TestEasyDragLabPseudoBrowser.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git add ui/desktop_easydrag_lab_browser_test.go && git commit -F C:/disposable/easydrag-w1a/e4-msg.txt
```

---

### Task E.5: Fix what the lab finds

The lab will very probably report real findings: a status word on a light material, a disc on a tinted well, a rail caption that clips in the pseudo run, an icon button without a name. This task is a **procedure**, because the findings cannot be known in advance. It ends when every finding is fixed, reported, or (with the owner's reason) in `labKnown`, and the closing run is green.

**Files:**
- Modify (as findings require): `ui/css/desktop-app-easydrag.css`, the `easydrag-*.js` module that renders the element, `ui/lang/easydrag/*.json` through a key file, `ui/desktop_easydrag_lab_test.go` (only `labKnown`).
- Test: `TestEasyDragLabBrowser` (T1), plus the gates of the brief for every changed file.

**Rules (spec §11):**
- A **contrast** finding is fixed in EasyDrag's own surface or ink tokens, in the token block of `desktop-app-easydrag.css` or in the theme-scoped block `.desktop-body[data-theme="<id>"] .ed-app`. **Never change a status colour** (`--ed-ok`, `--ed-warn`, `--ed-danger`, `--ed-info`, `--ed-status-neutral`, or the `--ds-color-*-fg` tokens they read): the status colours are fixed by EasyDrag and never set by a theme.
- A **clipping** finding gets `text-overflow: ellipsis` plus a `title` on the element, or wrapping (`white-space: normal`, `overflow-wrap: anywhere`), whichever the layout allows.
- A **name** finding gets an `aria-label` from an existing key, or from a new key through a key file.
- A finding that needs a **design decision** is reported, not fixed. These are: a material whose own surface blocks a token fix; a target under 24 px that the design wants small (an exception-list entry); a status colour that fails on a surface the owner chose; a category colour that fails its distance (the static `TestEasyDragCategoryColourTokens` should have caught that, so a lab-only failure means the rendered colour differs from the token and the cause must be found first).
- The line ratchet applies: `easydrag-canvas.js` must not grow; `-model`, `-template`, `-editor`, `-home` and `-detail` change existing lines only (brief).

- [ ] **Step 1: Run the whole lab and list the findings**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run '(?i)TestEasyDragLab(Pseudo)?Browser$' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-lab-findings.log | grep -E "^--- (PASS|FAIL)|^ok|^FAIL"
grep -h "LAB-FINDING" D:/tmp/easydrag-w1a-lab-findings.log | sed 's/^ *[a-z_.]*_test.go:[0-9]*: //' | sort -u > D:/tmp/easydrag-w1a-lab-findings.txt
wc -l D:/tmp/easydrag-w1a-lab-findings.txt
```

Expected: a sorted list, one finding per line, `LAB-FINDING theme=... scene=... element=... check=... value=... want=...`.

- [ ] **Step 2: Group the findings by root cause** (one group, one fix, one commit)

```bash
awk -F'check=' '{split($2,a," "); print a[1]}' D:/tmp/easydrag-w1a-lab-findings.txt | sort | uniq -c | sort -rn
awk -F'theme=' '{split($2,a," "); print a[1]}' D:/tmp/easydrag-w1a-lab-findings.txt | sort | uniq -c | sort -rn
```

Read both counts. The usual shapes, and the first move for each:

| Pattern in the list | Likely cause | First move |
|---|---|---|
| `text-contrast` on `.ed-status-word` in one theme, every surface | the word's colour token is too close to the surface in that theme | read the word's colour rule in `desktop-app-easydrag.css`; fix the ink token under that theme's `.ed-app` block |
| `mark-contrast` on a disc, one surface only (`@well` or `@canvas`) | the surface token of that theme is close to the status colour | override the surface token (`--ed-panel-strong`, `--ed-well-color`) in the theme's `.ed-app` block, one step lighter or darker |
| `mark-contrast` with `[a gradient or pattern lies under it ...]` | the lab read only `background-color` | take a screenshot (`AURAGO_EASYDRAG_CONTACT_SHEET=1`, run only that theme); if the disc is readable, it is a lab false positive: refine the measure script, never relax the threshold |
| `icon-on-tile-contrast` | the tile ink token fails on its tile | the static `TestEasyDragCategoryColourTokens` covers it; a lab-only hit means the render differs: report |
| `text-clipped` on `.ed-rail-caption` | the 64 px rail clips a long pseudo caption | `overflow: hidden; text-overflow: ellipsis; white-space: nowrap` plus `title` set by the rail code, or `white-space: normal; overflow-wrap: anywhere; line-height` |
| `text-clipped` elsewhere | fixed width or `white-space: nowrap` without ellipsis | same two fixes, in the smallest rule |
| `accessible-name` on `button.ed-icon-btn...` | an icon-only control without `aria-label` | add `aria-label` from the key its tooltip already uses |
| `accessible-name` on `span.ed-status-disc` | a disc rendered by hand or with an empty label | it must go through `ED.status.disc`; report as a Part A leftover if it does not |
| `probe ... visible` | an element is gone | read the scene, not the colour: the surface changed or a part regressed |
| a size line from `checkTargets` | a control under 24x24 | give the control `min-width`/`min-height: 24px` in its rule; if the design wants it small, report it |

- [ ] **Step 3: Fix one group at a time, in the smallest place**

For each group, in this order:

1. Write the finding group down with an ID (`L-01`, `L-02`, ...).
2. **IMPACT before changing a function** (brief), for JS changes:

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<functionName>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

   Warn on HIGH or CRITICAL; treat UNKNOWN as unresolved and confirm callers with `grep -rn "<functionName>" ui/js/desktop/apps/`.
3. Make the change in one rule or one line. CSS example for a theme-scoped surface fix (the selector form is the one `easyDragS0ScopedSelectors` accepts, `.desktop-body[data-theme="<id>"] .ed-app`):

```css
.desktop-body[data-theme="lollipop"] .ed-app { --ed-panel-strong: #fff6fb; }
```

   Use the real token the finding points at and a value chosen so the measured ratio reaches the need with a margin; compute it with the helpers (a scratch `go test` calling `cssContrastRatio`) before editing, so the fix is not found by trial.
4. If a name needs a new key, write `C:/disposable/easydrag-w1a/keys/e-5-<id>.json` in the helper's format with the exact text for all 16 locales (German lowercase "du" where the text addresses the user) and apply it:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/e-5-<id>.json
```

   Then run `go test ./ui -run '(?i)TestEasyDragUIKeysExistInAllLocales' -count=1` with the Go prefix. Most name fixes need no key: the control already has a tooltip key.
5. Re-run only the affected themes:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run 'TestEasyDragLabBrowser$/^(<theme1>|<theme2>)(-pseudo)?$' -count=1 -v 2>&1 | grep -E "LAB-FINDING|^--- |^ok|^FAIL"
```

   Expected: the group's lines are gone and no new line appeared. A new line from the same change is a regression: undo it and choose another place.
6. Run the gates of the brief for what you changed:
   - CSS or JS: `node scripts/test-easydrag.mjs` (ends `all EasyDrag checks passed`) and `node scripts/test-easydrag-render-baseline.mjs` (ends `PASS render baseline (unchanged)`). If the render baseline moves because of the token change, update it only for that cause with `AURAGO_EASYDRAG_RENDER_BASELINE=update` and say so in the commit.
   - The static colour tests, which must stay green for any token change: `GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragCategoryColourTokens|TestEasyDragStateMarksUseChange|TestEasyDragStatus)' -count=1`.
   - JS changes also: `node scripts/build-ui-bundles.js --check` if the preamble found a bundle for EasyDrag, and `wc -l` on the touched module against the ratchet.
7. Commit, one commit per finding group. Write the message file `C:/disposable/easydrag-w1a/e5-<id>-msg.txt` and commit the exact paths:

```
fix(easydrag): <theme or "all themes"> <scene> <what was wrong> (lab L-<nn>)

Lab finding: <check> <element> measured <value>, want <want>, in <themes>.
Fix: <the one rule or line changed and why this place>. Status colours are
unchanged. <Render baseline updated for this cause only. / No baseline change.>

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && git add <exact paths> && git commit -F C:/disposable/easydrag-w1a/e5-<id>-msg.txt
```

- [ ] **Step 4: Report what is not fixed**

For a finding that needs a design decision, do not edit product code. Add it to the report table (Step 6) with the options you see. If the coordinator, on the owner's word, wants the lab green meanwhile, add one entry to `labKnown` in `ui/desktop_easydrag_lab_test.go`, in its own commit:

```go
var labKnown = map[string]string{
	"lollipop|1-status-sheet|span.ed-status-word[status=skipped]@canvas|text-contrast": "owner decision pending: Lollipop canvas surface, see lab report L-07",
}
```

The key is `theme|scene|element|check` exactly as the finding prints them; the reason must name the open decision. `TestEasyDragLabFindings` fails an entry without a reason. The entry is logged as `LAB-KNOWN` on every run, so it stays visible.

- [ ] **Step 5: Closing runs**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run '(?i)TestEasyDragLab(Pseudo)?Browser$' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-lab-final.log | grep -E "lab total|^--- (PASS|FAIL)|^ok|^FAIL"
grep -c "LAB-FINDING" D:/tmp/easydrag-w1a-lab-final.log; grep -c "LAB-KNOWN" D:/tmp/easydrag-w1a-lab-final.log
```

Expected: `--- PASS: TestEasyDragLabBrowser`, `LAB-FINDING` count 0, and a `LAB-KNOWN` count equal to the number of entries you added (0 if none).

The contact sheet must still build (spec §10, "the contact sheet still builds"); it was written before Parts A to D and waits for `.ed-pill`:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_EASYDRAG_CONTACT_SHEET=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-contact/after go test ./ui -run 'TestDesktopEasyDragContactSheetBrowser$' -count=1 -timeout 30m -v 2>&1 | grep -E "^--- |^ok|^FAIL|condition did not|wrote .*index"
```

Expected: `--- PASS` and the `wrote .../index.html` line. If it fails on a `condition did not become true` that names a selector Parts A to D replaced (for example the run-done wait of line 44), change that one wait to the element that exists now (the lab's own run-done wait in E.3 is the model: `.ed-node .ed-status-disc`), in `ui/desktop_easydrag_contact_sheet_browser_test.go`, and commit it alone as `test(easydrag): keep the contact sheet in step with the status discs`.

Then the full gated run and T0, against the baseline of the preamble:

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && npm run test:easydrag:t0 2>&1 | tail -6
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-lab && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-lab-after.log | tail -3
grep -c '^--- PASS' D:/tmp/easydrag-w1a-lab-after.log; grep -c '^--- SKIP' D:/tmp/easydrag-w1a-lab-after.log; grep -c '^--- FAIL' D:/tmp/easydrag-w1a-lab-after.log
```

Expected: T0 green; `ok  	aurago/ui`; PASS = baseline + 1 (baseline + 2 if E.4 split the pseudo run); SKIP = baseline SKIP; FAIL = 0.

- [ ] **Step 6: Report table for the coordinator**

Hand the coordinator this table, one row per finding group, in the final report of the part (not in a repository file):

```
| ID | Themes | Scene | Element | Check | Measured | Want | Disposition | Commit or decision needed |
|----|--------|-------|---------|-------|----------|------|-------------|---------------------------|
| L-01 | lollipop, amiga | 1-status-sheet | span.ed-status-word@canvas | text-contrast | 3.9:1 | 4.5:1 | fixed | abc1234 surface token --ed-bg lightened in the two theme blocks |
| L-02 | windows_xp | 3-idle-editor-pseudo | span.ed-rail-caption | text-clipped | sw 78 > cw 56 | no clip | fixed | def5678 ellipsis + title |
| L-03 | orbit | 1-status-sheet | span.ed-status-disc[status=paused]@well | mark-contrast | 2.4:1 | 3:1 | reported | the Orbit well material blocks a token fix; options: (a) lighten the well, (b) outline the disc in that theme. Recommendation: (b). In labKnown as "orbit|...|mark-contrast". |
```

Rules for the table:
- Disposition is `fixed`, `reported` (needs a decision; names the options and a recommendation) or `lab false positive` (the measure script was refined; say how).
- Add three summary lines under the table: the findings first seen (`wc -l` of the step 1 list), the findings left (the final log), and the lab's total time from the closing run.
- Add: the number of `labKnown` entries and each entry's reason; the lab's runtime; the PASS/SKIP counts before and after.

---

## Coverage of the spec (Part E)

| Spec item | Task |
|---|---|
| §7 `TestEasyDragLabBrowser` in `ui/desktop_easydrag_lab_browser_test.go`, T1 through `startEasyDragSmokeWith` | E.1 (through `startEasyDragSmoke`, which calls it) |
| §7 grows out of the contact sheet; screenshots behind `AURAGO_EASYDRAG_CONTACT_SHEET`, checks always under `AURAGO_RUN_BROWSER_SMOKE=1` | E.1 (`requirePrecisionBrowserSmoke`, `labShots`); lab beside the sheet, see Deviations |
| §7 scene 1 status sheet (every value as disc plus word on panel and canvas, one callout per status colour) | E.2 |
| §7 scene 2 category tiles and group colours | E.2 |
| §7 scene 3 idle editor with the labelled rail | E.3 |
| §7 scene 3 again with the pseudo-locale | E.4 |
| §7 scene 4 run done (the contact sheet's seed) | E.3 |
| §7 scene 5 test result with disclosures and a callout | E.3 |
| §7 scene 6 home | E.1 |
| §7 check: contrast in Go on the existing helpers; text 4.5:1, discs, tiles, icons 3:1 | E.1 (maths), E.2, E.3 |
| §7 check: distance, 30 degrees of hue and 0.20 OKLab to the rendered status colours | E.1 (`labDistanceFindings`), E.2 |
| §7 check: targets of 24 px with the documented exceptions | E.1 (home), E.3 (`checkTargets`) |
| §7 check: clipping in the pseudo run | E.4 |
| §7 check: accessible names on buttons, links, inputs and status discs | E.1, E.2, E.3 |
| §7 failures name theme, scene, element, measured value | E.1 (`LAB-FINDING`, `TestEasyDragLabFindings`) |
| §7 runtime budget of 4 minutes, split of the pseudo run | E.4 |
| §2, §10 "OKLab token checks across the 14 themes in the lab's first scenes" | E.2 |
| §10 "The lab's six scenes pass in all 14 themes, and the contact sheet still builds" | E.3, E.4, E.5 (closing runs) |
| §11 a failing theme is fixed in its EasyDrag surface tokens, never in the status colour; a blocking material goes to the owner | E.5 |

## Deviations from the spec

1. **Sixteen contexts, not fourteen.** The 14 registry themes plus the dark schemes of Fruity and Minimal, as `TestEasyDragCategoryColourTokens` uses (`easyDragS0ThemeContexts`, "16 contexts"). Each scheme paints different surfaces; two more contexts cost about 15% more time. The spec's "84 renders" becomes 96 scene renders plus 16 pseudo renders.
2. **The lab lives beside the contact sheet, it does not grow out of it.** The sheet keeps its three owner themes, its before/after index and its screenshot folder rules; the lab reuses its seed (`easyDragContactRunDoneJS`) and its open-the-flow steps. The contact sheet is not edited, except where E.5 must update one wait that Parts A to D invalidated.
3. **One page per theme context, not one page for the whole lab.** `newEasyDragPage` gives its page an absolute 150 s deadline; a single page could not carry a lab of several minutes. Subtests also isolate a failing theme.
4. **Scene 6 (home) arrives with E.1,** the scaffolding task, as its proof scene; E.3 adds scenes 3 to 5.
5. **Scenes 1 and 2 are panels injected into the open window**, as the task allows ("a fixture page or an injected panel"). They use the real `ED.status.label`, `ED.status.disc`, `ED.kit.callout` and the real tile and group classes, so the product's CSS is measured. Scene 5's callout is injected into the product's results panel, because the fixture run has no uncertain effect.
6. **Three surfaces, not two.** Panel (card colour), canvas (app background) and well (editor canvas), the three opaque surfaces `easyDragS0Surfaces` reads. The spec names panel and canvas.
7. **The distance check uses the four coloured statuses** (ok, failed, running, uncertain). The grey statuses are not references: the logic and tool tiles are grey by design and would fail 0.20 against grey by definition. (Part A's colour test must make the same cut; if it adds grey to the status set, it needs the same exception.)
8. **"Tiles" at 3:1 means the icon on its tile**, the rule `TestEasyDragCategoryColourTokens` already holds. A tile against its surface is not required (pastel tiles on light panels would fail by design); group frames against the well are, as in the static test.
9. **Contrast reads `background-color` only.** A material gradient or pattern under an element sets `approx` and the finding says so. A flagged finding is verified by screenshot before it is fixed.
10. **`labKnown` exists** so that a reported finding can be recorded without silencing it (logged as `LAB-KNOWN` on every run). It starts empty.
11. **In-page names are `window.EasyDrag.*`,** not `ED.*` (the brief's shorthand).

## Decisions needed

1. **Sixteen contexts or fourteen?** Recommendation: sixteen. The extra two are the dark schemes the static colour test already covers, and a surface that passes in light can fail in dark.
2. **May a finding that needs a design decision be parked in `labKnown` so the branch can land?** Recommendation: yes, only with the owner's word and a reason that names the decision, because an open Wave 1 gate is worse than a visible, listed exception. Every entry is printed on each run and listed in the final report.
3. **If the lab runs longer than 4 minutes, split the pseudo run (the spec's rule) or run subtests in parallel?** Recommendation: split. Parallel subtests share one Chrome, and `startEasyDragSmokeWith` drops that Chrome when one test fails, which would crash the others.
4. **Grey statuses in the distance check.** Recommendation: leave them out (Deviation 7) and make Part A's colour test do the same. Including them would force a change to the grey logic and tool tiles, which is a design decision of its own.

## Expected runtime

- A theme context: about 10 to 14 s (page start 2 s, home, two sheets, editor open, run-done seed, one test run, seven measurements, four target scans).
- The pseudo run: about 5 to 7 s per context (page, reload, editor, one scan).
- The lab: 16 contexts × about 18 s, so **about 4.5 to 5 minutes is possible**; the first full run of E.4 decides. The split of E.4 step 3a brings each test to about 3 minutes.
- Findings fixed in E.5 re-run per theme (20 to 30 s), not the whole lab.
