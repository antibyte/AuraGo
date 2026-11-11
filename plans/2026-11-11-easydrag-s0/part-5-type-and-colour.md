[← Plan index](README.md) · Spec: [plans/2026-11-10-easydrag-s0-spec.md](../2026-11-10-easydrag-s0-spec.md)

## Part 5: Card type and colour

Spec: `plans/2026-11-10-easydrag-s0-spec.md` §7 (S0.4) and §8 (S0.5). Work happens in `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0` on `feat/easydrag-s0`. All commands below use Git Bash syntax and run from that worktree unless a step says otherwise. Line numbers are as of main 3da58db83. Parts 1 and 2 move JS functions, so steps name functions as well as lines. This part edits no function that Parts 1 and 2 move.

**Browser test gating.** Every EasyDrag browser test calls `requirePrecisionBrowserSmoke` (`ui/precision_workspace_browser_test.go:13`), which skips unless `AURAGO_RUN_BROWSER_SMOKE=1` is set. `startEasyDragSmoke` (`ui/desktop_easydrag_browser_test.go:347`) writes screenshots to `AURAGO_BROWSER_ARTIFACT_DIR`, or to `../reports/easydrag` when that variable is unset. Every browser command in this part therefore sets `AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-shots`, so no run rewrites `reports/easydrag/*.png`. Reduced motion is set with the existing helper `setReducedMotion(t, page, "reduce")` (`ui/desktop_screensaver_browser_test.go:107`).

**Shared commands:**
- `BASELINE`: `node scripts/test-easydrag-render-baseline.mjs` (created in Part 1). Expected: PASS, counters unchanged.
- `IMPACT <name>`: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<name>" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo`. A result of UNKNOWN is unresolved; confirm it with a grep. Warn on HIGH or CRITICAL.
- `DETECT`: `node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .` (spec §9). A `partial` or `truncated` result is not clean; re-run it.

---

### Task 5.1: Readable card type (S0.4, UI-02)

**Files:**
- Modify: `ui/css/desktop-app-easydrag.css`
  - `.ed-node-label` :566
  - `.ed-node-summary` :567
  - `.ed-node-error` :596-600
  - `.ed-port-label` :614-617
  - `.ed-pill text` :668
- Modify: `ui/js/desktop/apps/easydrag-canvas.js:7-9` (`READABLE_ZOOM`, used by `fit`, `readable` and `centerOn`)
- Modify: `ui/js/desktop/apps/easydrag-wires.js:133` (`syncPill`)
- Modify: `scripts/test-easydrag-extra4.mjs:670` and `:675` (these pin the old 0.8 floor)
- Create: `ui/desktop_easydrag_legibility_browser_test.go`

**PLAN-CHECK, accessible name (settled).** The card already has `role="group"` (canvas.js:522) and an `aria-label` that `paintState` sets from `cardName()` (canvas.js:407-419 and :444). The first part of that label is the full `n.label || i.label || n.type`. The card has no `title`. CSS only cuts the visible name with an ellipsis, so no code change is needed; the test below pins the behaviour.

- [ ] **Step 1: Impact checks.** Run `IMPACT syncPill` and `IMPACT readable`. `READABLE_ZOOM` is a constant, so `readable` stands in for `fit`, `readable` and `centerOn`. Report the callers and the risk level.

- [ ] **Step 2: Write the failing test.** Create `ui/desktop_easydrag_legibility_browser_test.go`:

```go
package ui

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"strings"
	"testing"

	"aurago/internal/flows"
)

// S0.4 (UI-02): canvas text is readable at 100 % and at every automatic fit (READABLE_ZOOM 0.86).
// The run helpers here are shared with the S0.5 colour test.

// easyDragS0RunDoneJS gives the open editor a finished test run without the backend: every step succeeded and every
// wire delivered items (7, 42 and 1234 in turn, so one-, two- and four-digit pills are drawn).
const easyDragS0RunDoneJS = `()=>{const ed=edFixture.editor(),counts=[7,42,1234];
	const steps=new Map(ed.model.doc.nodes.map(n=>[n.id,{node_id:n.id,status:'success',duration_ms:120,item_count:3}]));
	const edges=new Map(ed.model.doc.edges.map((e,i)=>[e.id,{state:'delivered',count:counts[i%3]}]));
	ed.run={id:'run_s0_done',status:'success',mode:'test',steps,changes:[...steps.keys()],edges,record:null,error:''};
	ed.bus.emit('run',ed.run);}`

// easyDragS0RunErrorJS fails every step with a long message, so every card shows a two-line error caption.
const easyDragS0RunErrorJS = `()=>{const ed=edFixture.editor();
	const steps=new Map(ed.model.doc.nodes.map(n=>[n.id,{node_id:n.id,status:'error',duration_ms:80,error_code:'FLOW_NODE_FAILED',error_message:'Der Dienst hat die Anfrage abgelehnt, weil das Konto gesperrt ist und erst morgen wieder freigegeben wird.'}]));
	ed.run={id:'run_s0_error',status:'error',mode:'test',steps,changes:[...steps.keys()],edges:new Map(),record:null,error:''};
	ed.bus.emit('run',ed.run);}`

const easyDragS0OpenFlowJS = `id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`

// easyDragS0FitSizesJS reports the smallest rendered size (font size times the world transform) of card names,
// summaries, and of pills, port labels and error captions together.
const easyDragS0FitSizesJS = `()=>{const ed=edFixture.editor(),world=document.querySelector('.ed-canvas .ed-world'),m=new DOMMatrixReadOnly(getComputedStyle(world).transform);
	const shown=e=>e.getClientRects().length>0&&getComputedStyle(e).visibility==='visible'&&getComputedStyle(e).display!=='none';
	const px=sel=>[...document.querySelectorAll('.ed-canvas '+sel)].filter(shown).map(e=>parseFloat(getComputedStyle(e).fontSize)*m.a);
	const min=a=>a.length?Math.min(...a):999;
	return JSON.stringify({zoom:ed.view.zoom,rendered:m.a,label:min(px('.ed-node-label')),summary:min(px('.ed-node-summary')),
		small:min(px('.ed-pill text').concat(px('.ed-port-label'),px('.ed-node-error'))),zoomedOut:document.querySelector('.ed-canvas').classList.contains('is-zoomed-out')});}`

// easyDragS0SmallTextJS lists every visible element under the canvas whose own text is set below 12 px.
const easyDragS0SmallTextJS = `()=>{const out=[];
	document.querySelectorAll('.ed-canvas *').forEach(el=>{
		if(el.closest('.ed-sr-only,[hidden]'))return;
		if(![...el.childNodes].some(n=>n.nodeType===3&&n.textContent.trim()))return;
		const r=el.getBoundingClientRect();if(r.width<2||r.height<2)return;
		const cs=getComputedStyle(el);if(cs.visibility!=='visible'||cs.display==='none')return;
		if(parseFloat(cs.fontSize)<12)out.push((el.getAttribute('class')||el.tagName)+' '+cs.fontSize+' '+JSON.stringify(el.textContent.trim().slice(0,24)));
	});
	return JSON.stringify(out);}`

// easyDragS0PillFitJS lists the pills whose number leaves their rect.
const easyDragS0PillFitJS = `()=>JSON.stringify([...document.querySelectorAll('.ed-canvas .ed-pill')].filter(p=>{const r=p.querySelector('rect').getBBox(),t=p.querySelector('text').getBBox();
	return !(t.x>=r.x-0.5&&t.x+t.width<=r.x+r.width+0.5&&t.y>=r.y-0.5&&t.y+t.height<=r.y+r.height+0.5);}).map(p=>p.textContent))`

// easyDragS0CardChecksJS: every card keeps its whole name in its accessible name, a card with at most one output
// stays 72 px tall, and its text block fits inside it.
const easyDragS0CardChecksJS = `()=>{const out=[];
	document.querySelectorAll('.ed-canvas .ed-node').forEach(c=>{if(c.hidden)return;
		const label=c.querySelector('.ed-node-label'),name=label.textContent,aria=c.getAttribute('aria-label')||'';
		if(!aria.startsWith(name))out.push('name '+JSON.stringify(name)+' not in '+JSON.stringify(aria));
		if(c.querySelectorAll('.ed-port--out').length<=1&&c.offsetHeight!==72)out.push('height '+c.offsetHeight+' '+JSON.stringify(name));
		const body=c.querySelector('.ed-node-body');if(body.offsetHeight>c.clientHeight)out.push('body '+body.offsetHeight+'>'+c.clientHeight+' '+JSON.stringify(name));
	});
	return JSON.stringify(out);}`

// easyDragS0OverlapJS lists the pairs of cards where a card or its error caption overlaps the other card or its caption.
const easyDragS0OverlapJS = `()=>{const boxes=[...document.querySelectorAll('.ed-canvas .ed-node')].filter(c=>!c.hidden).map(c=>{const list=[c.getBoundingClientRect()];const e=c.querySelector('.ed-node-error');if(e)list.push(e.getBoundingClientRect());return {name:c.querySelector('.ed-node-label').textContent,list};});
	const hit=(a,b)=>a.left<b.right-0.5&&b.left<a.right-0.5&&a.top<b.bottom-0.5&&b.top<a.bottom-0.5;
	const out=[];for(let i=0;i<boxes.length;i++)for(let j=i+1;j<boxes.length;j++)if(boxes[i].list.some(a=>boxes[j].list.some(b=>hit(a,b))))out.push(boxes[i].name+' / '+boxes[j].name);
	return JSON.stringify({captions:document.querySelectorAll('.ed-canvas .ed-node-error').length,overlaps:out});}`

type easyDragS0Recipe struct {
	ID    string          `json:"id"`
	Count int             `json:"count"`
	Nodes json.RawMessage `json:"nodes"`
	Edges json.RawMessage `json:"edges"`
}

type easyDragS0FitSizes struct {
	Zoom      float64 `json:"zoom"`
	Rendered  float64 `json:"rendered"`
	Label     float64 `json:"label"`
	Summary   float64 `json:"summary"`
	Small     float64 `json:"small"`
	ZoomedOut bool    `json:"zoomedOut"`
}

// easyDragS0GermanWords merges every lang/*/de.json, as startEasyDragSmoke does for the page.
func easyDragS0GermanWords(t *testing.T) map[string]string {
	t.Helper()
	words := map[string]string{}
	err := fs.WalkDir(Content, "lang", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "/de.json") {
			return err
		}
		data, err := fs.ReadFile(Content, path)
		if err != nil {
			return err
		}
		part := map[string]string{}
		if json.Unmarshal(data, &part) == nil {
			for key, value := range part {
				words[key] = value
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return words
}

// easyDragS0RecipeFixtures builds the starter templates (internal/flows) in German, and the real node types they use
// as the server describes them.
func easyDragS0RecipeFixtures(t *testing.T) ([]easyDragS0Recipe, []flows.NodeTypeInfo) {
	t.Helper()
	words := easyDragS0GermanWords(t)
	tr := func(key string) string {
		if value := strings.TrimSpace(words[key]); value != "" {
			return value
		}
		return key
	}
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, nil); err != nil {
		t.Fatal(err)
	}
	var recipes []easyDragS0Recipe
	used := map[string]bool{}
	for _, info := range flows.Templates() {
		flow, err := flows.TemplateFlow(info.ID, tr)
		if err != nil {
			t.Fatal(err)
		}
		nodes, err := json.Marshal(flow.Nodes)
		if err != nil {
			t.Fatal(err)
		}
		edges, err := json.Marshal(flow.Edges)
		if err != nil {
			t.Fatal(err)
		}
		recipes = append(recipes, easyDragS0Recipe{ID: info.ID, Count: len(flow.Nodes), Nodes: nodes, Edges: edges})
		for _, node := range flow.Nodes {
			used[node.Type] = true
		}
	}
	var types []flows.NodeTypeInfo
	for _, info := range flows.DescribeNodeTypes(reg, tr) {
		if used[info.Type] {
			types = append(types, info)
		}
	}
	return recipes, types
}

func (s *easyDragSmoke) s0Frames() {
	s.t.Helper()
	s.page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,160))))`)
}

// s0OpenFlow opens a flow in the EasyDrag window and waits for its cards and its automatic fit.
func (s *easyDragSmoke) s0OpenFlow(id string, nodes int) {
	s.t.Helper()
	s.page.MustEval(easyDragS0OpenFlowJS, id)
	s.wait(fmt.Sprintf(`()=>{const ed=edFixture.editor();return !!ed && ed.flow.id===%q && document.querySelectorAll('.ed-canvas .ed-node').length===%d}`, id, nodes))
	s.settle()
	s.s0Frames()
}

// s0ZoomFull sets 100 % and waits until the world transform shows it.
func (s *easyDragSmoke) s0ZoomFull() {
	s.t.Helper()
	s.page.MustElement(`[data-ed-zoom="reset"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor(),canvas=document.querySelector('.ed-canvas'),world=canvas&&canvas.querySelector('.ed-world');if(!world||canvas.classList.contains('is-animating'))return false;const m=new DOMMatrixReadOnly(getComputedStyle(world).transform);return Math.abs(ed.view.zoom-1)<0.001&&Math.abs(m.a-1)<0.001;}`)
	s.s0Frames()
}

func (s *easyDragSmoke) s0EmptyList(where, expr string) {
	s.t.Helper()
	var got []string
	if err := json.Unmarshal([]byte(s.page.MustEval(expr).Str()), &got); err != nil {
		s.t.Fatalf("%s: %v", where, err)
	}
	if len(got) > 0 {
		s.t.Errorf("%s: %v", where, got)
	}
}

func (s *easyDragSmoke) s0NoSmallText(where string) {
	s.t.Helper()
	s.s0EmptyList(where+": canvas text under 12 px", easyDragS0SmallTextJS)
}

// s0LegibleAtFit checks the rendered sizes at the current (automatic) fit; atFloor also requires the 0.86 floor itself.
func (s *easyDragSmoke) s0LegibleAtFit(where string, atFloor bool) {
	s.t.Helper()
	const eps = 1e-3
	var got easyDragS0FitSizes
	if err := json.Unmarshal([]byte(s.page.MustEval(easyDragS0FitSizesJS).Str()), &got); err != nil {
		s.t.Fatalf("%s: %v", where, err)
	}
	if got.ZoomedOut || got.Rendered < 0.86-eps {
		s.t.Errorf("%s: the automatic fit renders at %.3f (zoomed-out tier %v), want at least 0.86", where, got.Rendered, got.ZoomedOut)
	}
	if atFloor && math.Abs(got.Zoom-0.86) > eps {
		s.t.Errorf("%s: a flow that does not fit opens at zoom %.3f, want the 0.86 floor", where, got.Zoom)
	}
	if got.Label < 13.76-eps {
		s.t.Errorf("%s: card names render at %.2f px, want at least 13.76", where, got.Label)
	}
	if got.Summary < 12.04-eps {
		s.t.Errorf("%s: summaries render at %.2f px, want at least 12.04", where, got.Summary)
	}
	if got.Small < 10.32-eps {
		s.t.Errorf("%s: pills, port labels or error captions render at %.2f px, want at least 10.32", where, got.Small)
	}
}

func (s *easyDragSmoke) s0NoOverlaps(where string) {
	s.t.Helper()
	var got struct {
		Captions int      `json:"captions"`
		Overlaps []string `json:"overlaps"`
	}
	if err := json.Unmarshal([]byte(s.page.MustEval(easyDragS0OverlapJS).Str()), &got); err != nil {
		s.t.Fatalf("%s: %v", where, err)
	}
	if len(got.Overlaps) > 0 {
		s.t.Errorf("%s: overlapping cards or error captions (%d captions): %v", where, got.Captions, got.Overlaps)
	}
}

// TestDesktopEasyDragLegibilityBrowser checks S0.4 in the Standard theme with reduced motion: no canvas text under
// 12 px at 100 %, and at the automatic fit of the seeded flow, of fifty grouped steps and of every recipe, names at
// 13.76 px, summaries at 12.04 px and pills, port labels and error captions at 10.32 px or more. Recipe cards and
// their error captions never overlap, pills hold their numbers, cards stay 72 px tall and keep their whole names.
func TestDesktopEasyDragLegibilityBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	recipes, types := easyDragS0RecipeFixtures(t)
	if len(recipes) < 12 {
		t.Fatalf("flows.Templates() lists %d recipes; S0.4 covers 12", len(recipes))
	}
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-groups-fixture.js")
	setReducedMotion(t, s.page, "reduce")
	page := s.page
	page.MustEval(`()=>fixtureTheme('standard')`)

	// The recipes use node types the fixture catalog lacks. easydrag.js loads the catalog once per app instance, so
	// register the real types and open EasyDrag again.
	typesJSON, err := json.Marshal(types)
	if err != nil {
		t.Fatal(err)
	}
	page.MustEval(`list=>{JSON.parse(list).forEach(info=>edFixture.registerNode(info));fixtureCloseAll();}`, string(typesJSON))
	page.MustEval(`async()=>{await fixtureOpen('easydrag');}`)
	s.wait(`()=>EasyDragApp._instances.size===1 && !!document.querySelector('.ed-home')`)

	// The seeded flow after a test run.
	s.s0OpenFlow(page.MustEval(`()=>edFixture.seededID`).Str(), 4)
	page.MustEval(easyDragS0RunDoneJS)
	s.wait(`()=>document.querySelectorAll('.ed-canvas .ed-edge.is-delivered').length===3 && document.querySelectorAll('.ed-canvas .ed-pill').length===3`)
	s.s0LegibleAtFit("seeded flow", false)
	s.s0ZoomFull()
	s.s0NoSmallText("seeded flow at 100 %")
	s.s0EmptyList("seeded flow pills", easyDragS0PillFitJS)
	s.s0EmptyList("seeded flow cards", easyDragS0CardChecksJS)

	// A name longer than the card ends with an ellipsis and stays whole in the card's accessible name.
	long := page.MustEval(`()=>{const id=edFixture.addDraft('Lange Namen');edFixture.flow(id).rec.draft.nodes[2].label='Zusammenfassen der wichtigsten Schlagzeilen des Tages für das Morgenbriefing';return id;}`).Str()
	s.s0OpenFlow(long, 4)
	s.wait(`()=>{const c=document.querySelector('.ed-canvas .ed-node[data-node-id="n_ki"]'),l=c.querySelector('.ed-node-label');return l.scrollWidth>l.clientWidth && c.getAttribute('aria-label').startsWith('Zusammenfassen der wichtigsten Schlagzeilen des Tages für das Morgenbriefing');}`)

	// Fifty steps open at the readable floor; their five groups keep readable heads at 100 %.
	fifty := page.MustEval(`()=>edGroupsFixture.addDraft('Fünfzig Schritte',50)`).Str()
	s.s0OpenFlow(fifty, 50)
	page.MustEval(easyDragS0RunDoneJS)
	s.wait(`()=>document.querySelectorAll('.ed-canvas .ed-pill').length===edFixture.editor().model.doc.edges.length`)
	s.s0LegibleAtFit("groups-fifty", true)
	page.MustEval(`()=>{const ed=edFixture.editor(),ids=ed.model.doc.nodes.map(n=>n.id);for(let i=0;i<5;i++)if(!ed.model.createGroup(ids.slice(i*10,i*10+10),'Gruppe '+(i+1),['blue','green','amber','rose','violet'][i],'Zehn Schritte'))throw Error('50-node grouping');}`)
	s.wait(`()=>document.querySelectorAll('.ed-group-frame').length===5`)
	s.s0ZoomFull()
	s.s0NoSmallText("groups-fifty at 100 %")

	// The recipes with every step failed: readable at the automatic fit, no overlap, whole names, 72 px cards.
	for _, recipe := range recipes {
		data, err := json.Marshal(recipe)
		if err != nil {
			t.Fatal(err)
		}
		id := page.MustEval(`r=>{r=JSON.parse(r);const id=edFixture.addEmptyDraft(r.id),d=edFixture.flow(id).rec.draft;d.nodes=r.nodes.map(n=>Object.assign({},n,{settings:n.settings||{}}));d.edges=r.edges;return id;}`, string(data)).Str()
		s.s0OpenFlow(id, recipe.Count)
		s.wait(`()=>{const ed=edFixture.editor();return ed.model.doc.nodes.every(n=>!!ed.model.info(n.type));}`)
		page.MustEval(easyDragS0RunErrorJS)
		s.wait(fmt.Sprintf(`()=>document.querySelectorAll('.ed-canvas .ed-node-error').length===%d`, recipe.Count))
		s.s0Frames()
		s.s0LegibleAtFit("recipe "+recipe.ID, false)
		s.s0NoOverlaps("recipe " + recipe.ID)
		s.s0EmptyList("recipe "+recipe.ID+" cards", easyDragS0CardChecksJS)
		if recipe.ID == "local_first_result" { // it has a logic.if step: port labels and error captions at 100 %
			s.s0ZoomFull()
			s.s0NoSmallText("recipe local_first_result at 100 %")
		}
	}
	s.failOnPageErrors("legibility")
}
```

(`flows.NodeTypeInfo.Type` is the `type` field of the catalog JSON that `catalog.go:50ff` sends. `flows.RegisterCatalog(reg, nil)` is used the same way at `internal/flows/simulation.go:133`. `ui` gains no import cycle: no non-test package under `internal/` imports `aurago/ui`.)

- [ ] **Step 3: Run the test and confirm it fails.**
  `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-shots go test ./ui/ -run 'TestDesktopEasyDragLegibilityBrowser$' -count=1 -timeout 30m`
  Expected: FAIL, with errors such as:
  - `seeded flow: card names render at 10.5x px, want at least 13.76`
  - `seeded flow at 100 %: canvas text under 12 px: [ed-node-summary 11.5px …, text 11px "7" …]`
  - `groups-fifty: a flow that does not fit opens at zoom 0.800, want the 0.86 floor`
  - error captions at `11px` in `recipe local_first_result at 100 %`

  Before going on, any overlap reported for an unchanged recipe layout must be passed to the controller. It is a finding against the acceptance item and must not be fixed by loosening the check.

- [ ] **Step 4: Implement.** Edit `ui/css/desktop-app-easydrag.css`:
  - :566. Before: `.ed-node-label { font-weight: 700; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }`
    After: `.ed-node-label { font-weight: 700; font-size: 16px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }`
  - :567. Before: `.ed-node-summary { font-size: 11.5px; color: var(--ed-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }`
    After: `.ed-node-summary { font-size: 14px; color: var(--ed-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }`
    (:568 `.ed-node-summary--muted { font-style: italic; }` stays. It is the fallback only.)
  - :598 inside `.ed-node-error`. Before: `    background: color-mix(in srgb, var(--ed-danger) 16%, var(--ed-panel-strong)); color: var(--ed-danger-ink); font-size: 11px; line-height: 1.35;`
    After: `    background: color-mix(in srgb, var(--ed-danger) 16%, var(--ed-panel-strong)); color: var(--ed-danger-ink); font-size: 12px; line-height: 1.35;`
  - :616 inside `.ed-port-label`. Before: `    font-size: 10.5px; font-weight: 700; color: var(--ed-muted); background: var(--ed-panel-strong); white-space: nowrap; pointer-events: none;`
    After: `    font-size: 12px; font-weight: 700; color: var(--ed-muted); background: var(--ed-panel-strong); white-space: nowrap; pointer-events: none;`
  - :668. Before: `.ed-pill text { fill: var(--ed-text); font: 700 11px/1 var(--ds-font-sans, system-ui, sans-serif); }`
    After: `.ed-pill text { fill: var(--ed-text); font: 700 12px/1 var(--ds-font-sans, system-ui, sans-serif); }`
  - The zoomed-out tier (:645-647) and NODE_H do not change.

  `ui/js/desktop/apps/easydrag-canvas.js:7-9`. Before:
  ```js
      // READABLE_ZOOM is the smallest zoom of an automatic fit; READABLE_PAD the room it leaves at the
      // trigger end. Below ZOOMED_OUT the cards show their labels only, in a larger size.
      const READABLE_ZOOM = 0.8;
  ```
  After:
  ```js
      // READABLE_ZOOM is the smallest zoom of an automatic fit (at 0.86 a 16 px name renders at 13.76 px, a 14 px
      // summary at 12.04 px and 12 px pills and port labels at 10.32 px); READABLE_PAD the room it leaves at the
      // trigger end. Below ZOOMED_OUT the cards show their labels only, in a larger size.
      const READABLE_ZOOM = 0.86;
  ```

  `ui/js/desktop/apps/easydrag-wires.js:133` in `syncPill`. Before: `            const w = 14 + text.length * 7;`
  After: `            const w = 16 + Math.ceil(text.length * 7.6);`

  `scripts/test-easydrag-extra4.mjs:670`. Before: `        eq('c1d14 the fallback is the readable fit from the trigger', round(h.ed.view), round({ x: 48, y: 350 - 36 * 0.8, zoom: 0.8 }));`
  After: `        eq('c1d14 the fallback is the readable fit from the trigger', round(h.ed.view), round({ x: 48, y: 350 - 36 * 0.86, zoom: 0.86 }));`

  `scripts/test-easydrag-extra4.mjs:675`. Before: `        check('c1d14 an explicit fit goes below the readable zoom on a narrow canvas', h.ed.view.zoom < 0.8, String(h.ed.view.zoom));`
  After: `        check('c1d14 an explicit fit goes below the readable zoom on a narrow canvas', h.ed.view.zoom < 0.86, String(h.ed.view.zoom));`

- [ ] **Step 5: Run the tests and confirm they pass.**
  - `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-shots go test ./ui/ -run 'TestDesktopEasyDragLegibilityBrowser$' -count=1 -timeout 30m`. Expected: PASS.
  - `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-shots go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m`. Expected: PASS. This set includes `TestDesktopEasyDragBrowser`, whose checks at :50-51 (zoom ≥ 0.8, name field 16 px) and :147 (zoomed-out tier at the explicit fit) still hold.
  - `node scripts/test-easydrag-extra4.mjs` and `node scripts/test-easydrag.mjs`. Expected: PASS.
  - `BASELINE`. Expected: PASS, counters unchanged.

- [ ] **Step 6: Commit.** Run `DETECT` first. Then:
  ```
  git add ui/css/desktop-app-easydrag.css ui/js/desktop/apps/easydrag-canvas.js ui/js/desktop/apps/easydrag-wires.js scripts/test-easydrag-extra4.mjs ui/desktop_easydrag_legibility_browser_test.go
  git commit -m "feat(easydrag): readable card type at 100 % and at the 0.86 automatic fit" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 5.2: Colour tokens, bands, groups, lint and token test (S0.5 part 1)

**Files:**
- Modify: `ui/css/desktop-app-easydrag.css`
  - comment :61
  - token values :82-91
  - insert the band rules between :105 and :107
  - alias rules :107-117
  - tiles :274, :501-504, :558-562, :742, :869
  - groups :1445-1450
- Create: `ui/desktop_easydrag_colour_test.go`
- Modify: `ui/desktop_easydrag_test.go`, in `TestEasyDragStylesheetUsesThemeTokens` (:214-231)

**Verified.** The hex lint (`desktop_easydrag_test.go:216-222`) splits at the first `.ed-app [data-cat] {` and allows hex literals only before it. Every band rule therefore goes between the end of the `.ed-app {}` block (:105) and :107.

**Decision on blue → accent** (the spec leaves the method to the plan). The test reads each context's default `--vd-accent` along a rule chain: the theme's own root rule, then `:root` in desktop-base.css. User-accent themes (`accent: 'user'` in the registry: standard, fruity, black_gloss) also count the five presets `.desktop-body[data-accent="…"]`. Blue must fall back to `var(--ed-cat-planner)` if, and only if, one of those accents comes within 0.20 OKLab of a status token. The CSS lists the fallback scopes in one rule, and the test requires that list to equal the computed set in both directions. Computed by hand for this plan:

| Accent result | Contexts |
|---|---|
| Fails | Standard #E95420 vs danger 0.10, Fruity #0a84ff vs info 0.11, Amiga #6688bb vs info 0.12, Cyberpunk #F3E600 vs warning 0.13, Black Gloss #27c7a6 vs success 0.04, Lollipop #C2186B vs danger 0.11, Organic #D4A84F vs warning, Electronic Dreams #e6a83a vs warning, Green Slime #a5d941 vs success 0.12, Orbit #5ce1ff vs info 0.15 |
| Passes | Crazy 0.21, Windows XP 0.23, Voxel World 0.24, Minimal light, Minimal dark 0.22 |

Crazy and Minimal dark are close to the 0.20 limit. If the test reports a context in the other direction, move that selector as the message says. The Go computation decides, not this table.

**Decision on neutral → `--ed-muted`.** Muted is grey text. In Standard it is about 0.14 OKLab from info and success, so it cannot pass the 0.20 rule. The rule exists to keep hues apart. The test therefore pins the neutral token to exactly `var(--ed-muted)` and checks that no group token reads a status token, but it does not measure neutral's distance. This is reported to the controller in "Notes".

**The lint arrives with a pending list.** It lists the 12 rules that Task 5.3 converts. Task 5.3's failing-test step deletes the list.

- [ ] **Step 1: Impact check.** Run `IMPACT TestEasyDragStylesheetUsesThemeTokens`. It is a test function; report the result.

- [ ] **Step 2: Write the failing tests.** Create `ui/desktop_easydrag_colour_test.go`:

```go
package ui

import (
	"math"
	"regexp"
	"strings"
	"testing"
)

// S0.5 (UI-03): category colour marks identity, never state. These tests read the tile and icon pairs of the three
// bands in desktop-app-easydrag.css and the status and accent tokens of every theme context.

var easyDragS0Categories = []string{"trigger", "logic", "ai", "web", "documents", "notify", "smart_home", "planner", "tool"}

var easyDragS0StatusTokens = []string{"--ds-color-success-fg", "--ds-color-warning-fg", "--ds-color-danger-fg", "--ds-color-info-fg"}

// easyDragS0GroupTokens render the stored group colour names (model.js:31, groups.go:42).
var easyDragS0GroupTokens = map[string]string{
	"--ed-group-blue": "var(--ed-accent)", "--ed-group-green": "var(--ed-cat-web)", "--ed-group-amber": "var(--ed-cat-smart_home)",
	"--ed-group-rose": "var(--ed-cat-notify)", "--ed-group-violet": "var(--ed-cat-trigger)", "--ed-group-neutral": "var(--ed-muted)",
}

const (
	easyDragS0DarkBand      = ".ed-app"
	easyDragS0LightBand     = `.desktop-body[data-theme="lollipop"] .ed-app`
	easyDragS0MinimalBand   = `.desktop-body[data-theme="minimal"]:not([data-scheme="dark"]) .ed-app`
	easyDragS0CyberpunkBand = `.desktop-body[data-theme="cyberpunk"] .ed-app`
	easyDragS0BlueFallback  = "var(--ed-cat-planner)"
)

var (
	easyDragS0StatusRead   = regexp.MustCompile(`--ed-(ok|warn|danger|info)\b|--ds-color-(success|warning|danger|info)-`)
	easyDragS0AccentPreset = regexp.MustCompile(`^\.desktop-body\[data-accent="([a-z]+)"\]$`)
	easyDragS0CatRead      = regexp.MustCompile(`var\(\s*--ed-cat(?:-[a-z_]+)*\s*[,)]`)
	easyDragS0RefCat       = regexp.MustCompile(`^\.ed-ref\.cat-[a-z_]+$`)
)

// easyDragS0CatIdentity are the identity marks that may carry category colour (spec §8).
var easyDragS0CatIdentity = map[string]bool{
	".ed-node-tile": true, ".ed-tile": true, ".ed-palette-tile": true, ".ed-detail-tile": true, ".ed-tree-tile": true,
	".ed-palette-cat-dot": true, ".ed-cat-dot": true, ".ed-flow-preview rect": true, ".ed-minimap-nodes rect": true, ".ed-ref": true,
}

// easyDragS0CatReadPending are the rules that still use category colour for state or decoration; Task 5.3 converts them
// and deletes this list.
var easyDragS0CatReadPending = map[string]bool{
	".ed-home-hero": true, ".ed-logo": true, ".ed-flow-triggers .ed-icon": true, ".ed-drag-ghost.is-over-canvas": true,
	".ed-node::before": true, ".ed-edge.is-delivered .ed-wire": true, ".ed-json-bool": true, ".ed-tree-group": true,
	".ed-type--bool": true, ".ed-type--file": true, `.ed-suggest-kind[data-kind="filter"]`: true, ".ed-case-port": true,
}

type easyDragS0RuleRef struct{ File, Selector string }

type easyDragS0ThemeContext struct {
	Name       string
	Band       string              // selector of the band rule that colours this context
	Scope      string              // selector a theme-scoped ".ed-app" rule uses for this context
	Chain      []easyDragS0RuleRef // where its status and accent tokens come from, most specific first
	UserAccent bool
}

type easyDragS0Band struct{ Tile, Ink map[string][3]float64 }

// easyDragOKLab converts an sRGB colour (channels 0-255) to OKLab (L, a, b).
func easyDragOKLab(rgb [3]float64) [3]float64 {
	var lin [3]float64
	for i, channel := range rgb {
		c := channel / 255
		if c <= 0.04045 {
			lin[i] = c / 12.92
		} else {
			lin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	l := math.Cbrt(0.4122214708*lin[0] + 0.5363325363*lin[1] + 0.0514459929*lin[2])
	m := math.Cbrt(0.2119034982*lin[0] + 0.6806995451*lin[1] + 0.1073969566*lin[2])
	s := math.Cbrt(0.0883024619*lin[0] + 0.2817188376*lin[1] + 0.6299787005*lin[2])
	return [3]float64{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// easyDragOKLCH returns OKLCH lightness, chroma and hue (degrees, 0-360).
func easyDragOKLCH(rgb [3]float64) (lightness, chroma, hue float64) {
	lab := easyDragOKLab(rgb)
	chroma = math.Hypot(lab[1], lab[2])
	hue = math.Atan2(lab[2], lab[1]) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return lab[0], chroma, hue
}

func easyDragOKLabDistance(a, b [3]float64) float64 {
	x, y := easyDragOKLab(a), easyDragOKLab(b)
	return math.Sqrt((x[0]-y[0])*(x[0]-y[0]) + (x[1]-y[1])*(x[1]-y[1]) + (x[2]-y[2])*(x[2]-y[2]))
}

// easyDragHueGap is the angle between two hues, 0-180.
func easyDragHueGap(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func easyDragS0Sheet(t *testing.T, sheets map[string]cssSheet, file string) cssSheet {
	t.Helper()
	if sheet, ok := sheets[file]; ok {
		return sheet
	}
	sheet := parseCSSSheet(strings.ReplaceAll(readDesktopAssetText(t, file), "\r\n", "\n"))
	sheets[file] = sheet
	return sheet
}

// easyDragS0RuleDecls merges the declarations of every top-level rule that lists selector (later rules win).
func easyDragS0RuleDecls(sheet cssSheet, selector string) map[string]string {
	want := canonicalCSSSelector(collapseCSSSpace(selector))
	decls := map[string]string{}
	for _, rule := range sheet.Rules {
		if len(rule.Context) > 0 {
			continue
		}
		for _, part := range splitCSSSelectorList(rule.Selector) {
			if canonicalCSSSelector(part) != want {
				continue
			}
			for _, decl := range rule.Decls {
				decls[decl.Property] = cssDeclarationValue(decl.Value)
			}
			break
		}
	}
	return decls
}

func easyDragS0HexToken(t *testing.T, sheets map[string]cssSheet, ctx easyDragS0ThemeContext, token string) [3]float64 {
	t.Helper()
	for _, ref := range ctx.Chain {
		value, ok := easyDragS0RuleDecls(easyDragS0Sheet(t, sheets, ref.File), ref.Selector)[token]
		if !ok {
			continue
		}
		rgb, ok := parseCSSHexColor(value)
		if !ok {
			t.Fatalf("%s: %s in %s %s is %q, not a hex colour", ctx.Name, token, ref.File, ref.Selector, value)
		}
		return rgb
	}
	t.Fatalf("%s: no rule of %v declares %s", ctx.Name, ctx.Chain, token)
	return [3]float64{}
}

func easyDragS0ReadBand(t *testing.T, sheet cssSheet, selector string) easyDragS0Band {
	t.Helper()
	decls := easyDragS0RuleDecls(sheet, selector)
	band := easyDragS0Band{Tile: map[string][3]float64{}, Ink: map[string][3]float64{}}
	for _, cat := range easyDragS0Categories {
		for token, into := range map[string]map[string][3]float64{"--ed-cat-" + cat: band.Tile, "--ed-cat-" + cat + "-ink": band.Ink} {
			rgb, ok := parseCSSHexColor(decls[token])
			if !ok {
				t.Errorf("%s: %s must be a six-digit hex colour, got %q", selector, token, decls[token])
				continue
			}
			into[cat] = rgb
		}
	}
	return band
}

// easyDragS0ThemeContexts lists the 16 contexts: the 14 registry themes, Fruity and Minimal in both schemes.
func easyDragS0ThemeContexts(t *testing.T) []easyDragS0ThemeContext {
	t.Helper()
	root := easyDragS0RuleRef{File: "css/desktop-base.css", Selector: ":root"}
	var contexts []easyDragS0ThemeContext
	for _, theme := range parseDesktopThemeRegistry(t) {
		file := "css/desktop-theme-" + strings.ReplaceAll(theme.ID, "_", "-") + ".css"
		if desktopThemeBuiltInShellThemes[theme.ID] {
			file = "css/desktop-base.css"
		}
		scope := `.desktop-body[data-theme="` + theme.ID + `"]`
		own := easyDragS0RuleRef{File: file, Selector: scope}
		ctx := easyDragS0ThemeContext{Name: theme.ID, Band: easyDragS0DarkBand, Scope: scope, Chain: []easyDragS0RuleRef{own, root}, UserAccent: theme.Accent == "user"}
		switch theme.ID {
		case "cyberpunk":
			ctx.Band = easyDragS0CyberpunkBand
		case "lollipop":
			ctx.Band = easyDragS0LightBand
		case "fruity":
			light, dark := ctx, ctx
			light.Name, dark.Name = "fruity-light", "fruity-dark"
			dark.Chain = []easyDragS0RuleRef{{File: file, Selector: scope + `[data-fruity-mode="dark"]`}, own, root}
			contexts = append(contexts, light, dark)
			continue
		case "minimal":
			light, dark := ctx, ctx
			light.Name, light.Band, light.Scope = "minimal-light", easyDragS0MinimalBand, scope+`:not([data-scheme="dark"])`
			dark.Name, dark.Scope = "minimal-dark", scope+`[data-scheme="dark"]`
			dark.Chain = []easyDragS0RuleRef{{File: file, Selector: dark.Scope}, own, root}
			contexts = append(contexts, light, dark)
			continue
		}
		contexts = append(contexts, ctx)
	}
	return contexts
}

// TestEasyDragCategoryColourTokens checks the category tiles of every band against the status colours of every theme
// context (16): a coloured tile keeps 30° of hue from each status colour, every tile and group colour keeps 0.20 OKLab
// distance from each, every icon reaches 3:1 on its tile, and group "blue" is the accent only where the accent keeps
// that distance as well.
func TestEasyDragCategoryColourTokens(t *testing.T) {
	sheets := map[string]cssSheet{}
	ed := easyDragS0Sheet(t, sheets, "css/desktop-app-easydrag.css")
	bands := map[string]easyDragS0Band{}
	for _, selector := range []string{easyDragS0DarkBand, easyDragS0LightBand, easyDragS0MinimalBand, easyDragS0CyberpunkBand} {
		bands[selector] = easyDragS0ReadBand(t, ed, selector)
	}
	if t.Failed() {
		t.FailNow()
	}

	root := easyDragS0RuleDecls(ed, easyDragS0DarkBand)
	for token, want := range easyDragS0GroupTokens {
		if got := root[token]; got != want {
			t.Errorf(".ed-app %s = %q, want %q", token, got, want)
		}
	}
	fallback := map[string]bool{}
	for _, rule := range ed.Rules {
		for _, decl := range rule.Decls {
			if !strings.HasPrefix(decl.Property, "--ed-group") {
				continue
			}
			if easyDragS0StatusRead.MatchString(decl.Value) {
				t.Errorf("line %d: %s %s reads a status colour (%s); groups use identity colours", rule.Line, rule.Selector, decl.Property, decl.Value)
			}
			if decl.Property != "--ed-group-blue" || (rule.Selector == easyDragS0DarkBand && len(rule.Context) == 0) {
				continue
			}
			if cssDeclarationValue(decl.Value) != easyDragS0BlueFallback || len(rule.Context) > 0 {
				t.Errorf("line %d: %s may only set --ed-group-blue to %s", rule.Line, rule.Selector, easyDragS0BlueFallback)
				continue
			}
			for _, part := range splitCSSSelectorList(rule.Selector) {
				fallback[strings.TrimSuffix(canonicalCSSSelector(part), " .ed-app")] = true
			}
		}
	}

	var presets [][3]float64
	for _, rule := range easyDragS0Sheet(t, sheets, "css/desktop-base.css").Rules {
		if len(rule.Context) > 0 || !easyDragS0AccentPreset.MatchString(canonicalCSSSelector(rule.Selector)) {
			continue
		}
		for _, decl := range rule.Decls {
			if rgb, ok := parseCSSHexColor(cssDeclarationValue(decl.Value)); ok && decl.Property == "--vd-accent" {
				presets = append(presets, rgb)
			}
		}
	}
	if len(presets) == 0 {
		t.Fatal(`desktop-base.css declares no accent preset (.desktop-body[data-accent="..."])`)
	}

	contexts := easyDragS0ThemeContexts(t)
	if len(contexts) != 16 {
		t.Fatalf("got %d theme contexts, want 16 (14 themes, Fruity and Minimal in both schemes)", len(contexts))
	}
	scopes := map[string]bool{}
	for _, ctx := range contexts {
		scopes[ctx.Scope] = true
		band := bands[ctx.Band]
		status := map[string][3]float64{}
		for _, token := range easyDragS0StatusTokens {
			status[token] = easyDragS0HexToken(t, sheets, ctx, token)
		}
		nearest := func(rgb [3]float64) (float64, string) {
			best, near := math.Inf(1), ""
			for token, s := range status {
				if d := easyDragOKLabDistance(rgb, s); d < best {
					best, near = d, token
				}
			}
			return best, near
		}
		for _, cat := range easyDragS0Categories {
			tile, ink := band.Tile[cat], band.Ink[cat]
			if _, chroma, hue := easyDragOKLCH(tile); chroma >= 0.05 {
				for token, s := range status {
					if _, _, statusHue := easyDragOKLCH(s); easyDragHueGap(hue, statusHue) < 30 {
						t.Errorf("%s: the %s tile (hue %.0f) is %.0f° from %s (hue %.0f), want at least 30°", ctx.Name, cat, hue, easyDragHueGap(hue, statusHue), token, statusHue)
					}
				}
			}
			if d, near := nearest(tile); d < 0.20 {
				t.Errorf("%s: the %s tile is %.3f from %s in OKLab, want at least 0.20", ctx.Name, cat, d, near)
			}
			if ratio := cssContrastRatio(ink, tile); ratio < 3 {
				t.Errorf("%s: the %s icon on its tile is %.2f:1, want at least 3:1", ctx.Name, cat, ratio)
			}
		}
		accents := [][3]float64{easyDragS0HexToken(t, sheets, ctx, "--vd-accent")}
		if ctx.UserAccent {
			accents = append(accents, presets...)
		}
		worst, worstNear := math.Inf(1), ""
		for _, accent := range accents {
			if d, near := nearest(accent); d < worst {
				worst, worstNear = d, near
			}
		}
		switch need := worst < 0.20; {
		case need && !fallback[ctx.Scope]:
			t.Errorf("%s: an accent comes within %.3f of %s; list %q in the --ed-group-blue fallback rule", ctx.Name, worst, worstNear, ctx.Scope+" .ed-app")
		case !need && fallback[ctx.Scope]:
			t.Errorf("%s: every accent keeps %.3f from the status colours; remove %q from the --ed-group-blue fallback rule", ctx.Name, worst, ctx.Scope+" .ed-app")
		}
		groups := map[string][3]float64{"green": band.Tile["web"], "amber": band.Tile["smart_home"], "rose": band.Tile["notify"], "violet": band.Tile["trigger"], "blue": accents[0]}
		if fallback[ctx.Scope] {
			groups["blue"] = band.Tile["planner"]
		}
		for name, rgb := range groups {
			if d, near := nearest(rgb); d < 0.20 {
				t.Errorf("%s: group %s is %.3f from %s in OKLab, want at least 0.20", ctx.Name, name, d, near)
			}
		}
	}
	for scope := range fallback {
		if !scopes[scope] {
			t.Errorf("the --ed-group-blue fallback names %q, which is no theme context", scope)
		}
	}
}

func easyDragS0CatReadAllowed(part string) bool {
	part = canonicalCSSSelector(collapseCSSSpace(part))
	switch {
	case part == ".ed-app", strings.HasPrefix(part, ".ed-app [data-cat"):
		return true
	case strings.HasPrefix(part, `.desktop-body[data-theme="`) && strings.HasSuffix(part, " .ed-app"):
		return true
	case easyDragS0CatIdentity[part], easyDragS0RefCat.MatchString(part):
		return true
	}
	return easyDragS0CatReadPending[part]
}

// checkEasyDragCategoryReads fails every declaration that reads --ed-cat, --ed-cat-ink or --ed-cat-<name> outside the
// token and band blocks, the [data-cat] aliases and the identity marks.
func checkEasyDragCategoryReads(t *testing.T, css string) {
	t.Helper()
	sheet := parseCSSSheet(strings.ReplaceAll(css, "\r\n", "\n"))
	if len(sheet.Problems) > 0 {
		t.Fatalf("desktop-app-easydrag.css does not parse: %v", sheet.Problems)
	}
	for _, rule := range sheet.Rules {
		for _, decl := range rule.Decls {
			if !easyDragS0CatRead.MatchString(decl.Value) {
				continue
			}
			for _, part := range splitCSSSelectorList(rule.Selector) {
				if !easyDragS0CatReadAllowed(part) {
					t.Errorf("line %d: %s reads a category colour in %s: %s; category colour marks identity only (tiles, dots, previews, reference chips)", rule.Line, part, decl.Property, decl.Value)
				}
			}
		}
	}
}
```

  In `ui/desktop_easydrag_test.go`, `TestEasyDragStylesheetUsesThemeTokens`. Before (end of the function):
  ```go
  		if !strings.Contains(css, marker) {
  			t.Errorf("desktop-app-easydrag.css misses %q", marker)
  		}
  	}
  }
  ```
  After:
  ```go
  		if !strings.Contains(css, marker) {
  			t.Errorf("desktop-app-easydrag.css misses %q", marker)
  		}
  	}
  	checkEasyDragCategoryReads(t, css)
  }
  ```

- [ ] **Step 3: Run the tests and confirm they fail.**
  `go test ./ui/ -run '(?i)(EasyDragCategoryColourTokens|EasyDragStylesheetUsesThemeTokens)' -count=1`
  Expected: FAIL.
  - `.ed-app: --ed-cat-trigger must be a six-digit hex colour, got "var(--ds-color-warning-fg)"` (and the same for the other categories).
  - `--ed-cat-trigger-ink …, got ""`.
  - The light and Cyberpunk bands report empty values.
  - `TestEasyDragStylesheetUsesThemeTokens` passes, because every current reader is either allowed or pending. A parse problem here would be a real CSS defect: report it.

- [ ] **Step 4: Implement the CSS.**

  :61-63 comment. Before: `    /* ink: a hue mixed with the text colour, like --ed-cat-ink. The status tokens are the same in light and dark themes`
  After: `    /* ink: a hue mixed with the text colour. The status tokens are the same in light and dark themes`

  :82-91. Before: the nine `--ed-cat-*` lines and `--ed-group-violet: #9c87cb;`. After (same position, keep `--ed-on-accent: #ffffff;` after it):
  ```css
      /* category identity (UI-03): a solid tile and its icon colour per category. Category colour marks identity, never
         state. This is the dark band (the 11 base themes and Minimal dark); the light and Cyberpunk bands follow the block. */
      --ed-cat-trigger: #584995;
      --ed-cat-trigger-ink: #e7e5fe;
      --ed-cat-ai: #733d89;
      --ed-cat-ai-ink: #f2e0fb;
      --ed-cat-notify: #873761;
      --ed-cat-notify-ink: #fedeec;
      --ed-cat-web: #016566;
      --ed-cat-web-ink: #d1efef;
      --ed-cat-planner: #026376;
      --ed-cat-planner-ink: #d2eef6;
      --ed-cat-smart_home: #516003;
      --ed-cat-smart_home-ink: #e4ecd0;
      --ed-cat-documents: #655543;
      --ed-cat-documents-ink: #ede7df;
      --ed-cat-logic: #4e5969;
      --ed-cat-logic-ink: #e4e8ef;
      --ed-cat-tool: #53595f;
      --ed-cat-tool-ink: #e6e8ea;
      /* group frames: the stored colour names render as identity colours, never as status colours */
      --ed-group-blue: var(--ed-accent);
      --ed-group-green: var(--ed-cat-web);
      --ed-group-amber: var(--ed-cat-smart_home);
      --ed-group-rose: var(--ed-cat-notify);
      --ed-group-violet: var(--ed-cat-trigger);
      --ed-group-neutral: var(--ed-muted);
  ```

  Insert after the closing `}` of `.ed-app` (:105) and before :107:
  ```css

  /* category tiles, light band (Lollipop, Minimal light): their status colours are dark, so the tiles are light */
  .desktop-body[data-theme="lollipop"] .ed-app,
  .desktop-body[data-theme="minimal"]:not([data-scheme="dark"]) .ed-app {
      --ed-cat-trigger: #ddd9ff;
      --ed-cat-trigger-ink: #3e316d;
      --ed-cat-ai: #f0d1ff;
      --ed-cat-ai-ink: #502a60;
      --ed-cat-notify: #fecee4;
      --ed-cat-notify-ink: #622344;
      --ed-cat-web: #b5eaea;
      --ed-cat-web-ink: #034747;
      --ed-cat-planner: #b7e7f6;
      --ed-cat-planner-ink: #014553;
      --ed-cat-smart_home: #d8e5b4;
      --ed-cat-smart_home-ink: #384302;
      --ed-cat-documents: #e7dccf;
      --ed-cat-documents-ink: #493a29;
      --ed-cat-logic: #d7dfea;
      --ed-cat-logic-ink: #333e4d;
      --ed-cat-tool: #dadee3;
      --ed-cat-tool-ink: #383e43;
  }

  /* category tiles, Cyberpunk band: hues kept away from its neon status colours */
  .desktop-body[data-theme="cyberpunk"] .ed-app {
      --ed-cat-trigger: #534b97;
      --ed-cat-trigger-ink: #e5e5fe;
      --ed-cat-ai: #6e3f8d;
      --ed-cat-ai-ink: #f0e1fc;
      --ed-cat-notify: #7c3b77;
      --ed-cat-notify-ink: #f8dff5;
      --ed-cat-web: #255c87;
      --ed-cat-web-ink: #d7ebfc;
      --ed-cat-planner: #3f568a;
      --ed-cat-planner-ink: #dee8fe;
      --ed-cat-smart_home: #575e01;
      --ed-cat-smart_home-ink: #e7ebcf;
      --ed-cat-documents: #6c5146;
      --ed-cat-documents-ink: #f0e5e0;
      --ed-cat-logic: #505966;
      --ed-cat-logic-ink: #e4e8ee;
      --ed-cat-tool: #54595d;
      --ed-cat-tool-ink: #e6e8ea;
  }

  /* group "blue" is the accent, except where a theme's accent (or an accent the user can pick) comes within 0.20 OKLab of a
     status colour: there it is the band's planner tile (TestEasyDragCategoryColourTokens computes this list) */
  .desktop-body[data-theme="standard"] .ed-app,
  .desktop-body[data-theme="fruity"] .ed-app,
  .desktop-body[data-theme="amiga"] .ed-app,
  .desktop-body[data-theme="cyberpunk"] .ed-app,
  .desktop-body[data-theme="black_gloss"] .ed-app,
  .desktop-body[data-theme="lollipop"] .ed-app,
  .desktop-body[data-theme="organic"] .ed-app,
  .desktop-body[data-theme="electronic_dreams"] .ed-app,
  .desktop-body[data-theme="green_slime"] .ed-app,
  .desktop-body[data-theme="orbit"] .ed-app { --ed-group-blue: var(--ed-cat-planner); }
  ```

  :107-117. Replace the alias block and its ink comment and rule with:
  ```css
  .ed-app [data-cat] { --ed-cat: var(--ed-cat-tool); --ed-cat-ink: var(--ed-cat-tool-ink); }
  .ed-app [data-cat="tool"] { --ed-cat: var(--ed-cat-tool); --ed-cat-ink: var(--ed-cat-tool-ink); }
  .ed-app [data-cat="trigger"] { --ed-cat: var(--ed-cat-trigger); --ed-cat-ink: var(--ed-cat-trigger-ink); }
  .ed-app [data-cat="logic"] { --ed-cat: var(--ed-cat-logic); --ed-cat-ink: var(--ed-cat-logic-ink); }
  .ed-app [data-cat="ai"] { --ed-cat: var(--ed-cat-ai); --ed-cat-ink: var(--ed-cat-ai-ink); }
  .ed-app [data-cat="web"] { --ed-cat: var(--ed-cat-web); --ed-cat-ink: var(--ed-cat-web-ink); }
  .ed-app [data-cat="documents"] { --ed-cat: var(--ed-cat-documents); --ed-cat-ink: var(--ed-cat-documents-ink); }
  .ed-app [data-cat="notify"] { --ed-cat: var(--ed-cat-notify); --ed-cat-ink: var(--ed-cat-notify-ink); }
  .ed-app [data-cat="smart_home"] { --ed-cat: var(--ed-cat-smart_home); --ed-cat-ink: var(--ed-cat-smart_home-ink); }
  .ed-app [data-cat="planner"] { --ed-cat: var(--ed-cat-planner); --ed-cat-ink: var(--ed-cat-planner-ink); }
  ```

  Solid tiles:
  - :274. After: `.ed-tile { display: inline-grid; place-items: center; width: 28px; height: 28px; border-radius: 8px; flex: none; color: var(--ed-cat-ink); background: var(--ed-cat); }`
  - :501-504. After:
    ```css
    .ed-palette-tile {
        display: grid; place-items: center; width: 32px; height: 32px; flex: none; border-radius: 9px; color: var(--ed-cat-ink);
        background: var(--ed-cat);
    }
    ```
  - :558-562. After:
    ```css
    .ed-node-tile {
        display: grid; place-items: center; width: 40px; height: 40px; flex: none; border-radius: 11px; color: var(--ed-cat-ink);
        background: var(--ed-cat);
    }
    ```
  - :742. After: `.ed-detail-tile { display: grid; place-items: center; width: 36px; height: 36px; border-radius: 10px; color: var(--ed-cat-ink); background: var(--ed-cat); }`
  - :869. After: `.ed-tree-tile { display: grid; place-items: center; width: 22px; height: 22px; border-radius: 6px; color: var(--ed-cat-ink); background: var(--ed-cat); }`

  Groups, :1445-1450. After:
  ```css
  .ed-app .ed-group-frame, .ed-app .ed-group-head { position: absolute; left: 0; top: 0; --ed-group-color: var(--ed-group-neutral); }
  .ed-app .ed-group--blue { --ed-group-color: var(--ed-group-blue); }
  .ed-app .ed-group--green { --ed-group-color: var(--ed-group-green); }
  .ed-app .ed-group--amber { --ed-group-color: var(--ed-group-amber); }
  .ed-app .ed-group--rose { --ed-group-color: var(--ed-group-rose); }
  .ed-app .ed-group--violet { --ed-group-color: var(--ed-group-violet); }
  ```

  The hex values are the spec's table (§8) and must not be changed silently. If a hue, distance or contrast check fails on a spec value, stop and report the failing context and check to the controller. Only the blue fallback list follows the test's message.

- [ ] **Step 5: Run the tests and confirm they pass.**
  - `go test ./ui/ -run '(?i)(EasyDragCategoryColourTokens|EasyDragStylesheetUsesThemeTokens|EasyDrag)' -count=1`. Expected: PASS. The browser tests skip without the smoke flag.
  - `BASELINE`. Expected: PASS, counters unchanged.

- [ ] **Step 6: Commit.** Run `DETECT` first. Then:
  ```
  git add ui/css/desktop-app-easydrag.css ui/desktop_easydrag_colour_test.go ui/desktop_easydrag_test.go
  git commit -m "feat(easydrag): category tiles in three bands, identity-only group colours, colour lint and token test" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 5.3: Category colour on identity marks only (S0.5 part 2)

**Files:**
- Modify: `ui/css/desktop-app-easydrag.css`
  - :157-160, :253, :297, :307, :348, :514
  - :548-550 and :553 (removed)
  - :659, :833, :858, :879, :881, :946, :958
- Modify: `ui/js/desktop/apps/easydrag-wires.js:103` (`drawEdge`)
- Modify: `ui/desktop_easydrag_colour_test.go` (delete the pending list; add `TestEasyDragStateMarksUseAccent`)
- Create: `ui/desktop_easydrag_colour_browser_test.go`

**PLAN-CHECK, wires (settled).** `drawEdge` sets `g.dataset.cat` at wires.js:103. A grep of `ui/*_test.go`, `ui/testdata/*.js` and `scripts/test-easydrag*.mjs` finds no test that pins `data-cat` on edge groups. The only pins are on minimap and preview rects (`test-easydrag-extra4.mjs:688/:713`, `test-easydrag.mjs:1146`). The line is removed.

**PLAN-CHECK, Publish dot (settled).** `desktop_easydrag_browser_test.go:649` pins the `.ed-chip--warn` state chip, which shows the publish-incomplete state (real status, unchanged), and the `has-changes` class. It pins no colour, so the dot can change.

- [ ] **Step 1: Impact check.** Run `IMPACT drawEdge`. Report the result.

- [ ] **Step 2: Write the failing tests.**

  In `ui/desktop_easydrag_colour_test.go`:
  - Delete the whole `easyDragS0CatReadPending` declaration with its comment.
  - In `easyDragS0CatReadAllowed`, replace `	return easyDragS0CatReadPending[part]` with `	return false`.
  - Append:

```go
// TestEasyDragStateMarksUseAccent pins the interaction and change marks to the accent and keeps category colour off
// cards and wires: the unpublished chip and the Publish dot are not warnings, the drag ghost is an interaction, cards
// carry no category stripe and wires no category attribute.
func TestEasyDragStateMarksUseAccent(t *testing.T) {
	sheet := parseCSSSheet(strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n"))
	want := map[string]map[string]string{
		".ed-chip--accent":                    {"background": "color-mix(in srgb, var(--ed-accent) 16%, transparent)", "color": "var(--ed-accent-ink)"},
		".ed-btn--primary.has-changes::after": {"background": "var(--ed-accent)"},
		".ed-drag-ghost.is-over-canvas":       {"border-color": "var(--ed-accent)"},
	}
	for selector, props := range want {
		decls := easyDragS0RuleDecls(sheet, selector)
		for prop, value := range props {
			if got := decls[prop]; got != value {
				t.Errorf("%s { %s } = %q, want %q", selector, prop, got, value)
			}
		}
	}
	for _, rule := range sheet.Rules {
		for _, part := range splitCSSSelectorList(rule.Selector) {
			if strings.HasPrefix(part, ".ed-node") && strings.HasSuffix(part, "::before") {
				t.Errorf("line %d: %s draws a card stripe; a card shows its category only on its tile", rule.Line, part)
			}
		}
	}
	if strings.Contains(readDesktopAssetText(t, "js/desktop/apps/easydrag-wires.js"), "g.dataset.cat") {
		t.Error("easydrag-wires.js marks wires with a category (g.dataset.cat); wires show state only")
	}
}
```

  Create `ui/desktop_easydrag_colour_browser_test.go`:

```go
package ui

import (
	"encoding/json"
	"fmt"
	"testing"
)

// easyDragS0ColourAuditJS compares every wire stroke and card border with the 18 category tokens (tiles and icons) of
// the current band, finds card stripes and categorised wires, and reads the web tile token and a web card's tile.
const easyDragS0ColourAuditJS = `()=>{const app=document.querySelector('.ed-app'),cats=['trigger','logic','ai','web','documents','notify','smart_home','planner','tool'];
	const probe=v=>{const p=document.createElement('span');p.style.color=v;app.appendChild(p);const c=getComputedStyle(p).color;p.remove();return c;};
	const rgb=v=>{let m=String(v).match(/^rgba?\(([^)]+)\)$/);if(m)return m[1].split(/[\s,\/]+/).filter(Boolean).map(Number).slice(0,3);
		m=String(v).match(/^color\(srgb ([^)]+)\)$/);if(m)return m[1].split(/[\s\/]+/).filter(Boolean).slice(0,3).map(x=>Number(x)*255);return null;};
	const tokens=[];cats.forEach(c=>{tokens.push(['--ed-cat-'+c,rgb(probe('var(--ed-cat-'+c+')'))]);tokens.push(['--ed-cat-'+c+'-ink',rgb(probe('var(--ed-cat-'+c+'-ink)'))]);});
	const same=(a,b)=>!!a&&!!b&&a.every((x,i)=>Math.abs(x-b[i])<1.5);
	const hit=v=>{const c=rgb(v),t=tokens.find(([,x])=>same(c,x));return t?t[0]:'';};
	const wireHits=[],borderHits=[],stripes=[];
	document.querySelectorAll('.ed-canvas .ed-edge .ed-wire').forEach(w=>{const h=hit(getComputedStyle(w).stroke);if(h)wireHits.push(w.parentNode.dataset.edgeId+' '+h);});
	document.querySelectorAll('.ed-canvas .ed-node').forEach(c=>{const cs=getComputedStyle(c);['Top','Right','Bottom','Left'].forEach(side=>{const h=hit(cs['border'+side+'Color']);if(h)borderHits.push(c.dataset.nodeId+' '+side+' '+h);});
		if(getComputedStyle(c,'::before').content!=='none')stripes.push(c.dataset.nodeId);});
	const wire=document.querySelector('.ed-canvas .ed-edge.is-delivered .ed-wire'),web=document.querySelector('.ed-canvas .ed-node[data-cat="web"] .ed-node-tile');
	return JSON.stringify({wireHits,borderHits,stripes,edgeCats:document.querySelectorAll('.ed-canvas .ed-edge[data-cat]').length,
		deliveredWidth:getComputedStyle(wire).strokeWidth,webToken:probe('var(--ed-cat-web)'),webTile:getComputedStyle(web).backgroundColor});}`

type easyDragS0ColourAudit struct {
	WireHits       []string `json:"wireHits"`
	BorderHits     []string `json:"borderHits"`
	Stripes        []string `json:"stripes"`
	EdgeCats       int      `json:"edgeCats"`
	DeliveredWidth string   `json:"deliveredWidth"`
	WebToken       string   `json:"webToken"`
	WebTile        string   `json:"webTile"`
}

// TestDesktopEasyDragColourRolesBrowser checks UI-03 on the real canvas in the run-done state: no wire stroke and no
// card border equals a category token, cards have no stripe, wires no category, delivered wires are 2.5 px, tiles are
// solid, and each theme context gets its band (dark, light, Cyberpunk).
func TestDesktopEasyDragColourRolesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	setReducedMotion(t, s.page, "reduce")
	page := s.page
	page.MustEval(`()=>fixtureTheme('standard')`)
	page.MustEval(easyDragS0OpenFlowJS, page.MustEval(`()=>edFixture.seededID`).Str())
	s.wait(`()=>!!edFixture.editor() && document.querySelectorAll('.ed-canvas .ed-node').length===4 && document.querySelectorAll('.ed-canvas .ed-edge').length===3`)
	page.MustEval(easyDragS0RunDoneJS)
	s.wait(`()=>document.querySelectorAll('.ed-canvas .ed-edge.is-delivered').length===3 && document.querySelectorAll('.ed-canvas .ed-node.status-success').length===4`)
	for _, c := range []struct{ name, apply, web string }{
		{"standard", `()=>fixtureTheme('standard')`, "rgb(1, 101, 102)"},
		{"fruity-light", `()=>fixtureTheme('fruity-light')`, "rgb(1, 101, 102)"},
		{"lollipop", `()=>fixtureTheme('lollipop')`, "rgb(181, 234, 234)"},
		{"cyberpunk", `()=>fixtureTheme('cyberpunk')`, "rgb(37, 92, 135)"},
		{"minimal-light", `()=>fixtureTheme('minimal')`, "rgb(181, 234, 234)"},
		{"minimal-dark", `()=>{Object.assign(aurora.state.bootstrap.settings,{'appearance.theme':'minimal','appearance.minimal_mode':'dark'});aurora.applyDesktopSettings();}`, "rgb(1, 101, 102)"},
	} {
		page.MustEval(c.apply)
		s.wait(fmt.Sprintf(`()=>{const p=document.createElement('span');p.style.color='var(--ed-cat-web)';document.querySelector('.ed-app').appendChild(p);const v=getComputedStyle(p).color;p.remove();return v===%q;}`, c.web))
		var got easyDragS0ColourAudit
		if err := json.Unmarshal([]byte(page.MustEval(easyDragS0ColourAuditJS).Str()), &got); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(got.WireHits) > 0 || len(got.BorderHits) > 0 {
			t.Errorf("%s: category colour on state marks: wires %v, card borders %v", c.name, got.WireHits, got.BorderHits)
		}
		if len(got.Stripes) > 0 || got.EdgeCats > 0 {
			t.Errorf("%s: card stripes on %v, %d wires with data-cat", c.name, got.Stripes, got.EdgeCats)
		}
		if got.DeliveredWidth != "2.5px" {
			t.Errorf("%s: delivered wires are %s wide, want 2.5px", c.name, got.DeliveredWidth)
		}
		if got.WebTile != c.web || got.WebToken != c.web {
			t.Errorf("%s: web tile %s and token %s, want the solid band tile %s", c.name, got.WebTile, got.WebToken, c.web)
		}
	}
	page.MustEval(`()=>{aurora.state.bootstrap.settings['appearance.minimal_mode']='light';fixtureTheme('standard');}`)
	s.failOnPageErrors("colour roles")
}
```

- [ ] **Step 3: Run the tests and confirm they fail.**
  - `go test ./ui/ -run '(?i)(EasyDragStylesheetUsesThemeTokens|EasyDragStateMarksUseAccent)' -count=1`. Expected: FAIL.
    - The lint names 12 rules: `.ed-home-hero`, `.ed-logo`, `.ed-flow-triggers .ed-icon`, `.ed-drag-ghost.is-over-canvas`, `.ed-node::before`, `.ed-edge.is-delivered .ed-wire`, `.ed-json-bool`, `.ed-tree-group`, `.ed-type--bool`, `.ed-type--file`, `.ed-suggest-kind[data-kind="filter"]`, `.ed-case-port`.
    - `TestEasyDragStateMarksUseAccent` reports the warn chip and dot, the ghost border, the `::before` stripes and `g.dataset.cat`.
  - `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-shots go test ./ui/ -run 'TestDesktopEasyDragColourRolesBrowser$' -count=1 -timeout 30m`. Expected: FAIL, reporting wire hits `… --ed-cat-*` (delivered wires use `var(--ed-cat)`), stripes on all 4 cards, and 3 wires with data-cat.

- [ ] **Step 4: Implement.** Edit `ui/css/desktop-app-easydrag.css`:
  - :159. Before: `    background: var(--ed-warn); box-shadow: 0 0 0 2px var(--ed-chrome);`
    After: `    background: var(--ed-accent); box-shadow: 0 0 0 2px var(--ed-chrome);`
  - :253. Before: `.ed-chip--accent { background: color-mix(in srgb, var(--ed-warn) 18%, transparent); color: var(--ed-warn-ink); }`
    After: `.ed-chip--accent { background: color-mix(in srgb, var(--ed-accent) 16%, transparent); color: var(--ed-accent-ink); }`
  - :297. Before: `        radial-gradient(90% 120% at 100% 0%, color-mix(in srgb, var(--ed-cat-ai) 18%, transparent), transparent 60%),`
    After: `        radial-gradient(90% 120% at 100% 0%, color-mix(in srgb, var(--ed-accent) 12%, transparent), transparent 60%),`
  - :307. Before: `    background: var(--vd-mat-primary, linear-gradient(135deg, var(--ed-accent), var(--ed-cat-ai)));`
    After: `    background: var(--vd-mat-primary, linear-gradient(135deg, var(--ed-accent), var(--ed-accent-deep)));`
  - :348. Before: `.ed-flow-triggers .ed-icon { color: var(--ed-cat-trigger); width: 14px; height: 14px; }`
    After: `.ed-flow-triggers .ed-icon { color: var(--ed-muted); width: 14px; height: 14px; }`
  - :514. Before: `.ed-drag-ghost.is-over-canvas { opacity: 1; border-color: var(--ed-cat); }`
    After: `.ed-drag-ghost.is-over-canvas { opacity: 1; border-color: var(--ed-accent); }`
  - :548-550. Delete these three lines:
    ```css
    .ed-node::before {
        content: ""; position: absolute; left: -1px; top: 12px; bottom: 12px; width: 3px; border-radius: 0 3px 3px 0; background: var(--ed-cat); opacity: 0.9;
    }
    ```
  - :553. Delete `.ed-node.is-trigger::before { display: none; }`. The rule `.ed-node.is-trigger { border-radius: 30px 14px 14px 30px; }` (:552) stays.
  - :659. Before: `.ed-edge.is-delivered .ed-wire { stroke: var(--ed-cat); stroke-width: 2.5; }`
    After: `.ed-edge.is-delivered .ed-wire { stroke: color-mix(in srgb, var(--ed-text) 60%, transparent); stroke-width: 2.5; }`
  - :833. Before: `.ed-json-bool { color: color-mix(in srgb, var(--ed-cat-ai) 30%, var(--ed-text)); }`
    After: `.ed-json-bool { color: var(--ed-muted); }`
  - :858. Before: `.ed-tree-group { border-radius: var(--ed-radius); background: color-mix(in srgb, var(--ed-cat) 5%, transparent); border: 1px solid color-mix(in srgb, var(--ed-cat) 18%, var(--ed-border)); padding: 3px; }`
    After: `.ed-tree-group { border-radius: var(--ed-radius); background: color-mix(in srgb, var(--ed-text) 4%, transparent); border: 1px solid var(--ed-border); padding: 3px; }`
  - :879. Before: `.ed-type--bool { color: color-mix(in srgb, var(--ed-cat-ai) 30%, var(--ed-text)); }`
    After: `.ed-type--bool { color: var(--ed-muted); }`
  - :881. Before: `.ed-type--file { color: color-mix(in srgb, var(--ed-cat-documents) 30%, var(--ed-text)); }`
    After: `.ed-type--file { color: var(--ed-muted); }`
  - :946. Before: `.ed-suggest-kind[data-kind="filter"] { background: var(--ed-cat-ai); border-radius: 50%; }`
    After: `.ed-suggest-kind[data-kind="filter"] { background: var(--ed-accent); border-radius: 50%; }`
  - :958. Before: `.ed-case-port { flex: none; padding: 2px 8px; border-radius: 999px; background: color-mix(in srgb, var(--ed-cat-logic) 18%, transparent); color: color-mix(in srgb, var(--ed-cat-logic) 30%, var(--ed-text)); font-size: 11px; font-weight: 700; }`
    After: `.ed-case-port { flex: none; padding: 2px 8px; border-radius: 999px; background: var(--ed-control); color: var(--ed-text); box-shadow: inset 0 0 0 1px var(--ed-border); font-size: 11px; font-weight: 700; }`

  `ui/js/desktop/apps/easydrag-wires.js`, in `drawEdge`. Delete line 103: `            g.dataset.cat = core.catOf(ed.model.info(geo.src.type));`

  The effect chips, approval pills and JSON value colours (ok, info) stay as they are: they are the status uses listed in spec §8 for Wave 1.

- [ ] **Step 5: Run the tests and confirm they pass.**
  - `go test ./ui/ -run '(?i)(EasyDrag)' -count=1`. Expected: PASS for the unit tests.
  - `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-shots go test ./ui/ -run '(?i)(EasyDrag)' -count=1 -timeout 30m`. Expected: PASS for the new colour test, the legibility test and every existing EasyDrag browser test, including :649.
  - `node scripts/test-easydrag-extra.mjs` and `node scripts/test-easydrag-extra7.mjs` (wire tests). Expected: PASS.
  - `BASELINE`. Expected: PASS, counters unchanged.

- [ ] **Step 6: Commit.** Run `DETECT` first. Then:
  ```
  git add ui/css/desktop-app-easydrag.css ui/js/desktop/apps/easydrag-wires.js ui/desktop_easydrag_colour_test.go ui/desktop_easydrag_colour_browser_test.go
  git commit -m "feat(easydrag): category colour marks identity only; neutral delivered wires, accent change marks" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

---

### Task 5.4: Contact sheet for the owner

**Existing harness.** The `reports/easydrag/*.png` files of the 2026-11-08 run came from `startEasyDragSmoke` plus `(*easyDragSmoke).shot` (`ui/desktop_easydrag_browser_test.go:347-456`):
- `TestDesktopEasyDragBrowser` wrote `home-standard`, `editor-standard`, `run-done` and `editor-fruity-light/dark`.
- `TestDesktopEasyDragGroupsBrowser` wrote `groups-fifty-standard-1440`.

The output folder is `AURAGO_BROWSER_ARTIFACT_DIR`, or `../reports/easydrag` when it is unset. This task reuses that harness.

**Self-contained by design.** The "before" run uses a checkout of the commit before Task 5.1, where the Task 5.1 helpers do not exist. The contact-sheet file therefore uses only helpers that exist there and carries its own run constant.

**Files:** Create `ui/desktop_easydrag_contact_sheet_browser_test.go`. Screenshots go to `D:/tmp/easydrag-s0-contact/{before,after}` and are not committed.

- [ ] **Step 1: Write the helper test.**

```go
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// easyDragContactRunDoneJS is a finished test run without the backend (every step succeeded, every wire delivered).
const easyDragContactRunDoneJS = `()=>{const ed=edFixture.editor(),counts=[7,42,1234];
	const steps=new Map(ed.model.doc.nodes.map(n=>[n.id,{node_id:n.id,status:'success',duration_ms:120,item_count:3}]));
	const edges=new Map(ed.model.doc.edges.map((e,i)=>[e.id,{state:'delivered',count:counts[i%3]}]));
	ed.run={id:'run_contact',status:'success',mode:'test',steps,changes:[...steps.keys()],edges,record:null,error:''};
	ed.bus.emit('run',ed.run);}`

// TestDesktopEasyDragContactSheetBrowser captures the S0 colour review for the owner: home, editor in the run-done state
// and palette for Standard dark, Fruity light and Fruity dark, and the editor in every other theme context. It runs only
// on request and only into an explicit folder outside the repository:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_EASYDRAG_CONTACT_SHEET=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-contact/after go test ./ui/ -run 'TestDesktopEasyDragContactSheetBrowser$' -count=1 -timeout 30m
func TestDesktopEasyDragContactSheetBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	if os.Getenv("AURAGO_EASYDRAG_CONTACT_SHEET") != "1" {
		t.Skip("set AURAGO_EASYDRAG_CONTACT_SHEET=1 to capture the EasyDrag contact sheet")
	}
	if os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR") == "" {
		t.Fatal("set AURAGO_BROWSER_ARTIFACT_DIR to a scratch folder outside the repository")
	}
	s := startEasyDragSmoke(t)
	setReducedMotion(t, s.page, "reduce")
	page := s.page
	owner := []string{"standard", "fruity-light", "fruity-dark"}
	for _, name := range owner {
		page.MustEval(`theme=>fixtureTheme(theme)`, name)
		s.wait(`()=>!!document.querySelector('.ed-home') && document.querySelectorAll('.ed-flow-card').length>=1`)
		s.clearToasts()
		s.shot(name + "-home")
	}
	page.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, page.MustEval(`()=>edFixture.seededID`).Str())
	s.wait(`()=>!!edFixture.editor() && document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	page.MustEval(easyDragContactRunDoneJS)
	s.wait(`()=>document.querySelectorAll('.ed-edge.is-delivered').length===3 && document.querySelectorAll('.ed-pill').length===3`)
	s.fit()
	s.clearToasts()
	for _, name := range owner {
		page.MustEval(`theme=>fixtureTheme(theme)`, name)
		s.shot(name + "-editor-run-done")
		page.MustElement(".ed-palette").MustScreenshot(filepath.Join(s.dir, name+"-palette.png"))
	}
	// Representative survey: the editor in every other registry theme, and Minimal dark.
	for _, theme := range parseDesktopThemeRegistry(t) {
		if theme.ID == "standard" || theme.ID == "fruity" {
			continue
		}
		page.MustEval(`theme=>fixtureTheme(theme)`, theme.ID)
		s.wait(fmt.Sprintf(`()=>document.querySelector('.desktop-body').dataset.theme===%q`, theme.ID))
		s.shot("survey-" + theme.ID + "-editor")
	}
	page.MustEval(`()=>{Object.assign(aurora.state.bootstrap.settings,{'appearance.theme':'minimal','appearance.minimal_mode':'dark'});aurora.applyDesktopSettings();}`)
	s.wait(`()=>document.querySelector('.desktop-body').dataset.scheme==='dark'`)
	s.shot("survey-minimal-dark-editor")
	writeEasyDragContactIndex(t, s.dir)
}

// writeEasyDragContactIndex writes <parent>/index.html with the before and after shots side by side; the "after" run
// writes it.
func writeEasyDragContactIndex(t *testing.T, dir string) {
	t.Helper()
	if filepath.Base(dir) != "after" {
		return
	}
	shots, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(shots)
	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=\"utf-8\"><title>EasyDrag S0 contact sheet</title><style>body{font:14px system-ui;background:#222;color:#eee}td{vertical-align:top;padding:6px}img{max-width:720px}</style><table><tr><th>before</th><th>after</th></tr>\n")
	for _, shot := range shots {
		name := filepath.Base(shot)
		fmt.Fprintf(&b, "<tr><td colspan=\"2\">%s</td></tr><tr><td><img src=\"before/%s\"></td><td><img src=\"after/%s\"></td></tr>\n", name, name, name)
	}
	b.WriteString("</table>\n")
	index := filepath.Join(filepath.Dir(dir), "index.html")
	if err := os.WriteFile(index, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", index)
}
```

- [ ] **Step 2: Capture "before".** This is the commit before Task 5.1.
  ```
  SHA=$(git log -1 --format=%H --grep="readable card type at 100")
  BEFORE=$(git rev-parse "$SHA~1")
  git -C C:/Users/Andi/Documents/repo/AuraGo worktree add --detach D:/tmp/easydrag-s0-before "$BEFORE"
  cp ui/desktop_easydrag_contact_sheet_browser_test.go D:/tmp/easydrag-s0-before/ui/
  cd D:/tmp/easydrag-s0-before && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_EASYDRAG_CONTACT_SHEET=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-contact/before go test ./ui/ -run 'TestDesktopEasyDragContactSheetBrowser$' -count=1 -timeout 30m
  git -C C:/Users/Andi/Documents/repo/AuraGo worktree remove --force D:/tmp/easydrag-s0-before
  ```
  Expected: PASS, with 9 owner shots and 13 survey shots in `D:/tmp/easydrag-s0-contact/before`.

- [ ] **Step 3: Capture "after".** Run from the s0 worktree:
  `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_EASYDRAG_CONTACT_SHEET=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-s0-contact/after go test ./ui/ -run 'TestDesktopEasyDragContactSheetBrowser$' -count=1 -timeout 30m`
  Expected: PASS, and `D:/tmp/easydrag-s0-contact/index.html` is written.

  Also confirm the helper stays silent in the normal suite: `go test ./ui/ -run 'TestDesktopEasyDragContactSheetBrowser$' -count=1 -v` must show SKIP. Then run `BASELINE`. Expected: PASS, counters unchanged.

- [ ] **Step 4: Commit.** Run `DETECT` first. Check `git status --short`: no PNG files are staged or modified. Then:
  ```
  git add ui/desktop_easydrag_contact_sheet_browser_test.go
  git commit -m "test(easydrag): contact sheet for the S0 colour review" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  ```

- [ ] **Step 5: Hand-off.** Give the controller `D:/tmp/easydrag-s0-contact/index.html` for the owner's review. This closes acceptance item "The owner has reviewed the contact sheet".

---

### Part 5 coverage

| Spec item | Task | Note or PLAN-CHECK answer |
|---|---|---|
| §7 sizes: label 16, summary 14 upright (italic only in the fallback), port 12, pill 12, caption 12 | 5.1 | css :566, :567, :598, :616, :668 |
| §7 pill width `16 + Math.ceil(len*7.6)` | 5.1 | `syncPill`, wires.js:133 |
| §7 `READABLE_ZOOM` 0.86; explicit Fit unchanged | 5.1 | canvas.js:9. The 0.8 pins in `scripts/test-easydrag-extra4.mjs:670/:675` are updated. |
| §7 card height stays 72 | 5.1 | Card checks: offsetHeight 72, body fits. NODE_H is untouched. |
| §7 full name in the accessible name | 5.1 | **PLAN-CHECK:** the card has `role="group"` and an `aria-label` from `cardName()` (canvas.js:407-419), set in `paintState` (:444), starting with the full name; it has no `title`. No change; pinned by the long-name and card checks. |
| §7 test: zoom 1 ≥ 12 px; fit sizes of groups-fifty and 12 recipes; no recipe overlap including error captions; pill fit; existing :51 and :147 pass; counters | 5.1 | All in `TestDesktopEasyDragLegibilityBrowser`. Recipes come from `flows.TemplateFlow` with real node types. `:51` and `:147` run in the full EasyDrag set. |
| §8 tile and ink tokens, three bands, theme keys, placed before the first `[data-cat]` rule | 5.2 | **Verified:** the hex lint (`desktop_easydrag_test.go:216-222`) allows literals only before `.ed-app [data-cat] {`. |
| §8 aliases with an explicit `tool` rule; solid tiles | 5.2 | |
| §8 groups → identity tokens; blue falls back to planner | 5.2 | Decision: the test computes the accent per context (presets included for user-accent themes) and requires the CSS fallback list to equal its result. The expected list is in Task 5.2. |
| §8 lint (parser, allowlist) | 5.2 (with a pending list) and 5.3 (list deleted) | |
| §8 token test, 16 contexts, OKLab/OKLCH, hue ≥ 30°, distance ≥ 0.20, icon ≥ 3:1 | 5.2 | |
| §8 removals and changes (stripe, ghost, delivered wires, tree group, type text, suggest kind, case port, triggers icon, hero, logo, chip, Publish dot) | 5.3 | **PLAN-CHECK wires.js:103:** no test pins `data-cat` on edges, so the line is removed. **PLAN-CHECK :649:** it pins `.ed-chip--warn` (publish-incomplete state) and the `has-changes` class, no colour, so the dot changes freely. |
| §8 browser test: no wire stroke or card border equals a category token | 5.3 | Also checks band application in 6 contexts, including Minimal light and dark. |
| §8 contact sheet (Standard dark, Fruity light and dark: editor run-done, palette, home) plus a survey of the other themes | 5.4 | It reuses `startEasyDragSmoke` and `shot`; screenshots are not committed. |
| §10 / §14: reduced motion; `-run '(?i)(EasyDrag)'` with `-timeout 30m`; counters | all tasks | |

### Notes for the other parts

1. **Two deviations for the controller and owner.**
   - Neutral → `--ed-muted` cannot meet the 0.20 OKLab rule: muted grey is about 0.14 from info and success in Standard. The token test pins it to `var(--ed-muted)` and checks that no group token reads a status token, but does not measure neutral's distance.
   - The spec says "the other 11 themes", but the registry has 12 other themes. The survey shoots all 12, plus Minimal dark.
2. **Blue fallback list.** It was computed by hand. Crazy (0.21) and Minimal dark (0.22) are close to 0.20. The token test decides; follow its message if it disagrees. Every user-accent theme (Standard, Fruity, Black Gloss) falls back, so a user's accent pick never needs special handling.
3. **Screenshots.** `startEasyDragSmoke` writes to `../reports/easydrag` unless `AURAGO_BROWSER_ARTIFACT_DIR` is set. Every part should set it to a scratch folder (here `D:/tmp/easydrag-s0-shots`), so that browser runs never change `reports/easydrag/*.png`.
4. **Independence from Parts 1, 2 and 4.**
   - Part 5 builds its run-done and error states directly (`ed.run` with `steps` and `edges` maps, then `bus.emit('run')`, as `desktop_easydrag_groups_browser_test.go:161` does). So it depends neither on the test dialog that Part 4 (S0.3) changes, nor on where Part 1 moves `edgeStates`.
   - The `home.js:380` chip and the editor's `has-changes` toggle (editor.js:240) are only touched through CSS, so the Part 2 splits do not matter.
   - If Part 1 or another part adds reduced motion inside `startEasyDragSmoke`, the explicit `setReducedMotion` calls here stay harmless.
5. **Names added to package `ui`.** All are prefixed to avoid collisions: `easyDragS0*` (constants and types), `easyDragOKLab`, `easyDragOKLCH`, `easyDragOKLabDistance`, `easyDragHueGap`, `checkEasyDragCategoryReads`, `easyDragContactRunDoneJS`, `writeEasyDragContactIndex`, and `easyDragSmoke` methods `s0Frames`, `s0OpenFlow`, `s0ZoomFull`, `s0EmptyList`, `s0NoSmallText`, `s0LegibleAtFit` and `s0NoOverlaps`. Other parts should not reuse these names.
6. **`TestEasyDragStylesheetUsesThemeTokens`.** Part 1 may also edit `ui/desktop_easydrag_test.go` (load lists, ratchet). Task 5.2 only appends one call at the end of this function; locate it by name.
7. **Imports.** `ui` tests now import `aurago/internal/flows`. There is no cycle, because no non-test `internal` package imports `aurago/ui`.
8. **Review batch.** Per spec §10, S0.4 and S0.5 are reviewed together as one batch, Critical and Important findings only, after Task 5.3. Task 5.4 is evidence for the owner, not part of the code review.
