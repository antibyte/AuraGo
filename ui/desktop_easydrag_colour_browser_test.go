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
