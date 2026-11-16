package ui

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// easyDragControlBudget is one row of the control budget (W1b spec section 5).
//
//   - Ceiling is what the region may show today. The test fails above it and asks to lower it when the
//     count is below.
//   - Target is the number a later sub-project must reach; 0 means the region has no target.
//   - Gate names the sub-project that enforces the target ("" when there is no target).
//   - Enforced is flipped by that sub-project when it lands; from then on Ceiling equals Target.
type easyDragControlBudget struct {
	Region   string
	Ceiling  int
	Target   int
	Gate     string
	Enforced bool
}

// easyDragControlBudgets is the table. Regions are measured at 1440x900 with the standard fixture
// and a published, idle, enabled flow (see TestEasyDragControlBudgetsBrowser).
var easyDragControlBudgets = []easyDragControlBudget{
	// The editor header (.ed-head): Back, name, pill, Test, Undo, Redo and More, and Publish while there is something to publish (counted in that state)
	// to publish (W1c). Enforced.
	{Region: "header", Ceiling: 8, Target: 8, Gate: "W1c", Enforced: true},
	// The editor footer (.ed-foot). No target: it must not grow.
	{Region: "footer", Ceiling: 2},
	// The entries of the header "..." menu, separators not counted. No target: it must not grow, so
	// W1c cannot reach its header target by moving controls into the menu.
	{Region: "more-menu", Ceiling: 12},
	// Every control on the start page outside the flow cards and template cards (radio groups count
	// as one, a collapsed disclosure as its head). W1f rebuilds the page and must reach 6.
	{Region: "home", Ceiling: 15, Target: 6, Gate: "W1f"},
	// One published flow card, the card element itself not counted. W1f must reach 3.
	{Region: "card", Ceiling: 4, Target: 3, Gate: "W1f"},
}

// easyDragControlCount counts the controls under a root by the rule of spec section 5.1 and lists
// them, so a changed ceiling can be reviewed. Arguments: the root selector and a selector for
// descendants to skip ("" for none). It returns JSON {count, items}; count is -1 when the root is
// missing.
//
// Counted: button, a[href], input (not hidden), select, textarea and the roles button, switch,
// checkbox, tab, link, option, menuitem and radio, and tabindex="0". Disabled controls count.
// Counted as one: a radio group (role=radiogroup, or inputs with the same name). A collapsed
// disclosure shows only its head, because its body carries [hidden]. Not counted: [hidden] and
// [inert] ancestors, display none, visibility hidden, a zero-size box, the screen-reader-only list,
// and the root itself.
const easyDragControlCount = `(rootSel,skipSel)=>{
	const root=document.querySelector(rootSel);
	if(!root)return JSON.stringify({count:-1,items:[]});
	const picks='button,a[href],input,select,textarea,[role="button"],[role="switch"],[role="checkbox"],[role="tab"],[role="link"],[role="option"],[role="menuitem"],[role="radio"],[tabindex="0"]';
	const seen=new Set();
	const items=[];
	root.querySelectorAll(picks).forEach(el=>{
		if(el.matches('input[type="hidden"]'))return;
		if(skipSel&&el.closest(skipSel))return;
		if(el.closest('[hidden],[inert],.ed-sr-only'))return;
		const cs=getComputedStyle(el),r=el.getBoundingClientRect();
		if(cs.display==='none'||cs.visibility==='hidden'||(r.width===0&&r.height===0))return;
		let key=el;
		if(el.matches('input[type="radio"][name]'))key='radio:'+el.name;
		else if(el.matches('[role="radio"]')){const g=el.closest('[role="radiogroup"]');if(g)key=g;}
		if(seen.has(key))return;
		seen.add(key);
		const label=(el.getAttribute('aria-label')||el.textContent||'').replace(/\s+/g,' ').trim().slice(0,24);
		const data=[...el.attributes].map(a=>a.name).find(n=>n.startsWith('data-ed'))||'';
		items.push(el.tagName.toLowerCase()+(data?'['+data+']':'')+(label?' "'+label+'"':''));
	});
	return JSON.stringify({count:items.length,items:items});
}`

// easyDragMenuCount counts the entries of the open desktop context menu: a button, or the parent
// button of a submenu (its entries are not counted). Separators are no entries. Disabled entries
// count.
const easyDragMenuCount = `()=>{
	const menu=document.querySelector('.vd-context-menu');
	if(!menu)return JSON.stringify({count:-1,items:[]});
	const items=[...menu.querySelectorAll(':scope > .vd-context-item, :scope > .vd-context-submenu > .vd-context-item')]
		.map(b=>(b.querySelector('.vd-context-label')||b).textContent.replace(/\s+/g,' ').trim());
	return JSON.stringify({count:items.length,items:items});
}`

type easyDragControlReading struct {
	Count int      `json:"count"`
	Items []string `json:"items"`
}

func (s *easyDragSmoke) readControls(region, script string, args ...interface{}) int {
	s.t.Helper()
	var reading easyDragControlReading
	if err := json.Unmarshal([]byte(s.page.MustEval(script, args...).Str()), &reading); err != nil {
		s.t.Fatalf("%s: read the control count: %v", region, err)
	}
	s.t.Logf("CONTROLS region=%s count=%d: %s", region, reading.Count, strings.Join(reading.Items, " | "))
	return reading.Count
}

// countControls counts the controls under root, skipping the descendants that match skip.
func (s *easyDragSmoke) countControls(region, root, skip string) int {
	s.t.Helper()
	return s.readControls(region, easyDragControlCount, root, skip)
}

// countMenuEntries counts the entries of the open context menu.
func (s *easyDragSmoke) countMenuEntries(region string) int {
	s.t.Helper()
	return s.readControls(region, easyDragMenuCount)
}

// TestEasyDragControlBudgetsBrowser counts the controls of the header, the footer, the "..." menu,
// the start page and one published flow card at 1440x900 and compares each count with its ceiling in
// easyDragControlBudgets. A count above the ceiling fails; a count below it logs "lower the ceiling".
// Every region logs one BUDGET line.
func TestEasyDragControlBudgetsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	page := s.page
	counts := map[string]int{}

	// The start page with the seeded, published and enabled flow and the three templates.
	s.wait(`()=>document.querySelectorAll('.ed-home .ed-flow-card').length===1 && document.querySelectorAll('.ed-home .ed-template-card').length===3`)
	counts["home"] = s.countControls("home", ".ed-home", ".ed-flow-card, .ed-template-card")
	counts["card"] = s.countControls("card", ".ed-home .ed-flow-card", "")

	// The editor of that flow: published, idle and switched on; the pill says so.
	page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	s.wait(`()=>{const ed=edFixture.editor();const pill=document.querySelector('[data-ed-pill]');return !!ed && ed.flowEnabled===true && !!pill && pill.getAttribute('data-ed-pill-kind')==='live'}`)
	counts["header"] = s.countControls("header", ".ed-editor > .ed-head", "")
	counts["footer"] = s.countControls("footer", ".ed-editor > .ed-foot", "")

	// The "..." menu: open it, count its entries, close it again.
	page.MustElement(`[data-ed-cmd="more"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.vd-context-menu .vd-context-item')`)
	counts["more-menu"] = s.countMenuEntries("more-menu")
	page.MustEval(`()=>aurora.closeContextMenu()`)
	s.wait(`()=>!document.querySelector('.vd-context-menu')`)

	// The header's fullest state: with something to publish, Publish stays visible (owner decision on
	// W1c review finding 6), so the header shows one control more. The region's count is the larger one.
	page.MustEval(`()=>{edFixture.state.changes={steps:1,connections:0};const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed for the budget')}`)
	s.wait(`()=>edFixture.editor().saver.state==='saved'`)
	s.wait(`()=>{const b=document.querySelector('[data-ed-cmd="publish"]');return !!b && !b.hidden}`)
	if withPublish := s.countControls("header-with-publish", ".ed-editor > .ed-head", ""); withPublish > counts["header"] {
		counts["header"] = withPublish
	}

	for _, b := range easyDragControlBudgets {
		count, ok := counts[b.Region]
		if !ok {
			t.Errorf("budget region %q is not measured by this test", b.Region)
			continue
		}
		if count <= 0 {
			t.Errorf("budget region %q: found %d controls; the region selector no longer matches", b.Region, count)
			continue
		}
		target, gate := "-", "-"
		if b.Target > 0 {
			target = strconv.Itoa(b.Target)
		}
		if b.Gate != "" {
			gate = b.Gate
		}
		t.Logf("BUDGET region=%s count=%d ceiling=%d target=%s gate=%s", b.Region, count, b.Ceiling, target, gate)
		if count > b.Ceiling {
			t.Errorf("budget region %q: %d controls, ceiling %d (target %s, gate %s). A control was added; remove it or move it out of this region", b.Region, count, b.Ceiling, target, gate)
		}
		if count < b.Ceiling {
			t.Logf("BUDGET region=%s: lower the ceiling to %d", b.Region, count)
		}
	}
	s.failOnPageErrors("control budgets")
}
