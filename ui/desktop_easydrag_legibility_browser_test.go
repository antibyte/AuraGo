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
