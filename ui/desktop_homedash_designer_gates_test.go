package ui

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// homeDashGateProbeJS measures the designer chrome of the window WIN (a selector). It returns {contrast, ui, hits, anim, clip}; every row has `pass`.
//   - contrast: every chrome text against EVERY colour its background can show (a gradient material is measured at each of its stops and at its
//     opaque -color token, translucent layers are composited over what lies under them); the worst case must reach 4.5:1. A text that something
//     outside it (an open context menu, another window) covers completely is not measured: it is recorded as skip "occluded" (not a pass); a text
//     that is covered only in part is measured as before (the background stack is the same at every point of the text).
//   - ui: selection outline, handles, guides, ghost, marquee and focus rings against the page canvas (3:1, worst case), switch tracks against the panel,
//     field, select, device box, segmented and swatch borders against their own surface (3:1, Q17), the focus ring of the focused element
//     against its surroundings (3:1 and at least 2 px when it is an outline).
//   - hits: 24 px (fine) / 44 px (coarse) targets, handles 12 / 44; card hit buttons are measured on their card.
//   - anim: running animations (an infinite one fails).
//   - clip: chrome text that is cut off, ellipsised or spills out of its box or out of the window (German texts are the usual cause).
const homeDashGateProbeJS = `(WIN)=>{
 const win=document.querySelector(WIN),root=win&&win.querySelector('.hd-root'),res={contrast:[],ui:[],hits:[],anim:[],clip:[]};if(!root)return res;
 const app=root.closest('.hdx-app'),wr=win.getBoundingClientRect();
 const cv=document.createElement('canvas');cv.width=cv.height=1;const g=cv.getContext('2d',{willReadFrequently:true});
 const rgba=v=>{g.clearRect(0,0,1,1);g.fillStyle='rgba(0,0,0,0)';g.fillStyle=v;g.fillRect(0,0,1,1);const d=g.getImageData(0,0,1,1).data;return [d[0],d[1],d[2],d[3]/255];};
 const over=(a,b)=>{const al=a[3]+b[3]*(1-a[3]);return al?[0,1,2].map(i=>(a[i]*a[3]+b[i]*b[3]*(1-a[3]))/al).concat([al]):[0,0,0,0];};
 const same=(a,b)=>Math.abs(a[0]-b[0])+Math.abs(a[1]-b[1])+Math.abs(a[2]-b[2])<2&&Math.abs(a[3]-b[3])<.02;
 const uniq=l=>{const o=[];for(const c of l)if(!o.some(x=>same(x,c)))o.push(c);return o;};
 const SURF=[['.hd-f-input,.hd-f-select,.hd-ep-box,.hd-bp,.hd-f-seg','--vd-mat-field-color'],['.hd-btn,.hd-save-action,.hd-bp-opt[aria-checked="true"],.hd-f-seg-opt[aria-checked="true"]','--vd-mat-btn-color'],
  ['.hd-eb-done','--vd-mat-primary-color'],['.hd-ptab[aria-selected="true"],.hd-ep-opt[aria-selected="true"]','--vd-mat-selected-color'],['.hd-sheet,.hd-sel-chip,.hd-toast,.hd-pal-card,.hd-insp-variant,.hd-ep-pop,.hd-pal-avatar,.hd-draft-badge','--vd-mat-card-color'],
  ['.hd-palette,.hd-inspector','--vd-mat-side-color'],['.hd-editbar,.hdx-bar','--vd-mat-bar-color'],['.hd-page','--hd-canvas']];
 const STOP=/(rgba?\([^)]*\)|color\((?:[^()]|\([^)]*\))*\)|oklch\([^)]*\)|oklab\([^)]*\)|lab\([^)]*\)|lch\([^)]*\)|hsla?\([^)]*\)|color-mix\((?:[^()]|\([^)]*\))*\))/g;
 const stopsOf=img=>/gradient\(/.test(img)?(img.match(STOP)||[]).map(rgba):[];
 // every opaque colour the paint behind el can have: walk the ancestors up to the first opaque layer, outermost first
 const cands=el=>{const layers=[];for(let n=el;n&&n.nodeType===1;n=n.parentElement){const cs=getComputedStyle(n),c=rgba(cs.backgroundColor),img=cs.backgroundImage;let stops=[],tok=null;
   if(img!=='none'&&/gradient\(/.test(img)){stops=stopsOf(img);const s=SURF.find(x=>n.matches(x[0]));if(s){const v=cs.getPropertyValue(s[1]).trim();if(v){const t=rgba(v);if(t[3]>=.99)tok=t;}}}
   if(c[3]>0||stops.length||tok){layers.push({c,stops,tok});if(tok||(!stops.length&&c[3]>=.99))break;}}
  let list=[[255,255,255,1]];
  for(let i=layers.length-1;i>=0;i--){const L=layers[i];let next;
   if(L.tok)next=[L.tok].concat(L.stops.map(s=>over(s,L.tok)));
   else{const under=list.map(b=>(L.c[3]>0?over(L.c,b):b));next=under.slice();for(const s of L.stops)for(const b of under)next.push(over(s,b));}
   list=uniq(next).slice(0,32);}
  return list;};
 const lum=c=>{const f=v=>{v/=255;return v<=.03928?v/12.92:Math.pow((v+.055)/1.055,2.4);};return .2126*f(c[0])+.7152*f(c[1])+.0722*f(c[2]);};
 const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05);};
 const worst=(fg,list)=>{let w=1e9;for(const b of list){const r=ratio(over(fg,b),b);if(r<w)w=r;}return Math.round(w*100)/100;};
 const inView=el=>{const r=el.getBoundingClientRect();for(let n=el.parentElement;n&&n!==win;n=n.parentElement){const s=getComputedStyle(n);if(s.overflowX!=='visible'||s.overflowY!=='visible'){const p=n.getBoundingClientRect();if(r.bottom<=p.top||r.top>=p.bottom||r.right<=p.left||r.left>=p.right)return false;}}return true;};
 const shown=el=>{const r=el.getBoundingClientRect();if(!r.width||!r.height)return false;const s=getComputedStyle(el);return s.visibility!=='hidden'&&Number(s.opacity)>.05&&!el.closest('[hidden],[inert],.hd-thumb,[data-hd-drawer="closed"]')&&inView(el);};
 const covered=el=>{const rs=[];for(const n of el.childNodes){if(n.nodeType!==3||!n.textContent.trim())continue;const rg=document.createRange();rg.selectNodeContents(n);for(const r of rg.getClientRects())if(r.width>0&&r.height>0)rs.push(r);}
  if(!rs.length)return false;const L=Math.min(...rs.map(r=>r.left)),R=Math.max(...rs.map(r=>r.right)),T=Math.min(...rs.map(r=>r.top)),B=Math.max(...rs.map(r=>r.bottom)),cy=(T+B)/2;let seen=0,free=0;
  for(const [x,y] of [[(L+R)/2,cy],[L+(R-L)*.15,cy],[L+(R-L)*.85,cy],[(L+R)/2,T+(B-T)*.3],[(L+R)/2,T+(B-T)*.7]]){const hit=document.elementFromPoint(x,y);if(!hit)continue;seen++;if(hit===el||el.contains(hit)||hit.contains(el))free++;}
  return seen>0&&free===0;};
 const path=el=>{const own=[...el.classList].slice(0,2).join('.'),up=el.closest('[class*="hd-"]:not(span)');return el.tagName.toLowerCase()+'.'+(own||(up&&up!==el?'in '+[...up.classList][0]:''))+(el.dataset.hdWid?'#'+el.dataset.hdWid:'');};
 const snip=el=>[...el.childNodes].filter(n=>n.nodeType===3).map(n=>n.textContent.trim()).join(' ').slice(0,32);
 const CHROME='.hdx-bar,.hdx-banner,.hd-editbar,.hd-conflict-bar,.hd-palette,.hd-inspector,.hd-sheet-layer:not([data-kind]) .hd-sheet,.hd-sel-chip,.hd-toast,.hd-drop-avatar,.hd-draft-badge,.hdx-elsewhere,.hdx-save';
 const coarse=matchMedia('(pointer: coarse)').matches,hosts=[...app.querySelectorAll(CHROME)];
 for(const host of hosts)for(const el of [host,...host.querySelectorAll('*')]){
  if(!shown(el)||el.closest('[aria-disabled="true"],:disabled'))continue;const txt=snip(el);if(!txt)continue;
  if(covered(el)){res.contrast.push({el:path(el),name:txt,skip:'occluded',pass:false});continue;}
  const list=cands(el),r=worst(rgba(getComputedStyle(el).color),list);res.contrast.push({el:path(el),name:txt,ratio:r,pass:r>=4.5});
  if(el.closest('[data-entity],.hd-sr-only'))continue;
  const cs=getComputedStyle(el),box=el.getBoundingClientRect(),rg=document.createRange();rg.selectNodeContents(el);const tr=rg.getBoundingClientRect();
  const cx=el.scrollWidth>el.clientWidth+1&&cs.overflowX!=='visible',cy=el.scrollHeight>el.clientHeight+1&&cs.overflowY!=='visible',ell=cs.textOverflow==='ellipsis'&&el.scrollWidth>el.clientWidth+1;
  const spill=tr.width>0&&(tr.right>box.right+1.5||tr.left<box.left-1.5),out=tr.width>0&&(tr.right>wr.right+1||tr.left<wr.left-1||tr.bottom>wr.bottom+1);
  res.clip.push({el:path(el),name:txt,what:cx?'cut-x':cy?'cut-y':ell?'ellipsis':spill?'spill':out?'outside-window':'',w:cy&&!cx?el.scrollHeight:el.scrollWidth,h:cy&&!cx?el.clientHeight:el.clientWidth,pass:!(cx||cy||ell||spill||out)});}
 const page=root.querySelector('.hd-page'),canvas=page?cands(page):null;
 const ui=(el,what,col,against)=>{if(!el||!shown(el)||!against)return;const r=worst(rgba(col),against);res.ui.push({el:path(el),what,ratio:r,pass:r>=3});};
 for(const w of root.querySelectorAll('.hd-page > .hd-widget[aria-selected="true"]'))ui(w,'selection',getComputedStyle(w).outlineColor,canvas);
 for(const hd of root.querySelectorAll('.hd-handle'))ui(hd,'handle',getComputedStyle(hd,'::before').borderTopColor,canvas);
 for(const x of root.querySelectorAll('.hd-guide'))ui(x,'guide',getComputedStyle(x).backgroundColor,canvas);
 for(const x of root.querySelectorAll('.hd-ghost,.hd-marquee'))ui(x,x.className,getComputedStyle(x).borderTopColor,canvas);
 for(const s of app.querySelectorAll('.hd-f-switch')){const tr=s.querySelector('.hd-f-switch-track'),cs=getComputedStyle(tr),on=s.getAttribute('aria-checked')==='true';ui(s,on?'switch-on':'switch-off',on?cs.backgroundColor:cs.borderTopColor,cands(s.parentElement));}
 for(const f of app.querySelectorAll('.hd-f-input,.hd-f-select,.hd-ep-box,.hd-f-seg,.hd-f-swatch')){if(!shown(f)||f.matches('.hd-ep-input'))continue;ui(f,'field-border',getComputedStyle(f).borderTopColor,cands(f));}
 const act=document.activeElement;if(act&&root.contains(act)&&act!==document.body&&act.matches(':focus-visible')){const cs=getComputedStyle(act),back=cands(act.parentElement),rings=[];
  if(cs.outlineStyle!=='none'&&parseFloat(cs.outlineWidth)>0)rings.push({col:cs.outlineColor,w:parseFloat(cs.outlineWidth)});
  for(const m of cs.boxShadow.matchAll(/(rgba?\([^)]*\))\s+(-?[\d.]+)px\s+(-?[\d.]+)px\s+([\d.]+)px\s+([\d.]+)px/g))if(parseFloat(m[5])>0&&!/inset/.test(m[0]))rings.push({col:m[1],w:parseFloat(m[5])});
  let best={r:0,w:0};for(const x of rings){const r=worst(rgba(x.col),back);if(x.w>=2&&r>best.r||(!best.w&&x.w>best.w))best={r,w:x.w};}
  res.ui.push({el:path(act),what:'focus',ratio:best.r,w:Math.round(best.w*10)/10,pass:best.r>=3&&best.w>=2});}
 for(const el of [...root.querySelectorAll('button,input:not([type=file]),select,[role=tab],[role=radio],[role=switch],.hd-handle'),...app.querySelectorAll('.hdx-elsewhere .hdx-btn,.hdx-save .hdx-btn')]){if(!shown(el)||el.closest('.hd-page > .hd-widget'))continue;
  const box=(el.matches('.hd-pal-hit,.hd-insp-vhit,.hd-look-fhit')?el.parentElement:el).getBoundingClientRect(),min=el.matches('.hd-handle')?(coarse?44:12):(coarse?44:24);
  res.hits.push({el:path(el),name:snip(el)||el.getAttribute('aria-label')||'',w:Math.round(box.width),h:Math.round(box.height),pass:box.width>=min-.5&&box.height>=min-.5});}
 for(const a of document.getAnimations()){const tg=a.effect&&a.effect.target;if(!tg||!root.contains(tg)||a.playState!=='running')continue;const it=a.effect.getComputedTiming().iterations;res.anim.push({el:path(tg),name:a.animationName||a.transitionProperty||'',pass:it!==Infinity});}
 return res;}`

type hdProbeRow struct {
	El    string  `json:"el"`
	What  string  `json:"what,omitempty"`
	Ratio float64 `json:"ratio,omitempty"`
	W     float64 `json:"w,omitempty"`
	H     int     `json:"h,omitempty"`
	Name  string  `json:"name,omitempty"`
	Skip  string  `json:"skip,omitempty"`
	Pass  bool    `json:"pass"`
}

type hdProbe struct {
	Contrast []hdProbeRow `json:"contrast"`
	UI       []hdProbeRow `json:"ui"`
	Hits     []hdProbeRow `json:"hits"`
	Anim     []hdProbeRow `json:"anim"`
	Clip     []hdProbeRow `json:"clip"`
}

// failures lists every failed row as "kind element detail" in a stable order; a probe that measured no chrome text at all is a failure too
// (a probe that measures nothing must not pass: the M2 lesson).
func (p hdProbe) failures() []string {
	var out []string
	measured := 0
	for _, r := range p.Contrast {
		if r.Skip == "" {
			measured++
		}
	}
	if measured == 0 {
		out = append(out, "probe measured no chrome text")
	}
	for _, k := range []struct {
		kind string
		rows []hdProbeRow
	}{{"contrast", p.Contrast}, {"ui", p.UI}, {"hits", p.Hits}, {"anim", p.Anim}, {"clip", p.Clip}} {
		for _, r := range k.rows {
			if r.Pass || r.Skip != "" {
				continue
			}
			switch k.kind {
			case "contrast":
				out = append(out, fmt.Sprintf("contrast %s %q ratio=%v (needs 4.5)", r.El, r.Name, r.Ratio))
			case "ui":
				out = append(out, fmt.Sprintf("ui %s %s ratio=%v width=%v (needs 3)", r.El, r.What, r.Ratio, r.W))
			case "hits":
				out = append(out, fmt.Sprintf("hits %s %q %vx%v", r.El, r.Name, r.W, r.H))
			case "anim":
				out = append(out, fmt.Sprintf("anim %s %s runs forever", r.El, r.Name))
			default:
				out = append(out, fmt.Sprintf("clip %s %q %s scroll=%v client=%v", r.El, r.Name, r.What, r.W, r.H))
			}
		}
	}
	return out
}

// skipped lists every row the probe did not measure (a text that another element covers completely), as "contrast element "text" occluded".
func (p hdProbe) skipped() []string {
	var out []string
	for _, k := range []struct {
		kind string
		rows []hdProbeRow
	}{{"contrast", p.Contrast}, {"ui", p.UI}, {"hits", p.Hits}, {"anim", p.Anim}, {"clip", p.Clip}} {
		for _, r := range k.rows {
			if r.Skip != "" {
				out = append(out, fmt.Sprintf("%s %s %q %s", k.kind, r.El, r.Name, r.Skip))
			}
		}
	}
	return out
}

func (h *hdDesigner) probe() hdProbe {
	h.t.Helper()
	var p hdProbe
	if err := json.Unmarshal([]byte(h.str("(w)=>JSON.stringify(("+homeDashGateProbeJS+")(w))", h.winSel)), &p); err != nil {
		h.t.Fatal(err)
	}
	return p
}

// rects returns the bounding boxes of the widgets, the edit bar and the inspector (layout-shift probe: equal before and after a selection).
func (h *hdDesigner) rects() [][4]float64 {
	var out [][4]float64
	if err := json.Unmarshal([]byte(h.str(`()=>JSON.stringify([...document.querySelectorAll('.hd-root .hd-page > .hd-widget,.hd-root .hd-editbar,.hd-root .hd-inspector')].map(e=>{const r=e.getBoundingClientRect();return [Math.round(r.x*2)/2,Math.round(r.y*2)/2,Math.round(r.width*2)/2,Math.round(r.height*2)/2];}))`)), &out); err != nil {
		h.t.Fatal(err)
	}
	return out
}

func rectDiff(a, b [][4]float64) string {
	if len(a) != len(b) {
		return fmt.Sprintf("%d boxes became %d", len(a), len(b))
	}
	var parts []string
	for i := range a {
		if a[i] != b[i] {
			parts = append(parts, fmt.Sprintf("box %d %v -> %v", i, a[i], b[i]))
		}
	}
	return strings.Join(parts, "; ")
}

// hdGateState is one staged editor state of gate E-S/E-T. enter drives the real UI into it (pointer and keyboard through the helpers, every step
// waits for what it caused, and enter fails when the state is not what it names); leave undoes it. A held state keeps a mouse button down while it is
// shot, so leave cancels with Escape before the button is released (a release would drop or commit). The runner also resets everything before the next
// state, so a leave only has to be correct, not complete.
type hdGateState struct {
	name         string
	held         bool
	enter, leave func(h *hdDesigner)
}

// hdHold ends a held gesture or an open menu: Escape cancels, then the button goes up.
func hdHold(h *hdDesigner) {
	h.page.Keyboard.MustType(input.Escape)
	_ = h.page.Mouse.Up(proto.InputMouseButtonLeft, 1)
}

func hdSheetOpen(kind string) string {
	return `()=>!!document.querySelector('.hd-root .hd-sheet-layer:not([hidden]) .hd-sheet` + kind + `')`
}

// hdDirty makes the open dashboard dirty the way a user does: select a widget and nudge it one column right with the arrow key.
func (h *hdDesigner) hdDirty(id string) {
	h.selectOne(id)
	h.page.Keyboard.MustType(input.ArrowRight)
	h.until(5*time.Second, "nudge", `()=>document.querySelector('.hd-editbar [data-hd-cmd="undo"]').getAttribute('aria-disabled')!=='true'`)
}

func (h *hdDesigner) hdTab(tab string) {
	h.mouseClick(`.hd-inspector .hd-f-tab[data-tab="` + tab + `"]`)
	h.until(5*time.Second, "inspector tab "+tab, `(t)=>{const b=document.querySelector('.hd-inspector .hd-f-tab[data-tab="'+t+'"]');return !!b&&b.getAttribute('aria-selected')==='true';}`, tab)
}

// hdMoreItem opens the More menu of the edit bar and presses the item with the given words key.
func (h *hdDesigner) hdMoreItem(key string) {
	h.mouseClick(`.hd-eb-more`)
	h.until(5*time.Second, "More menu", `()=>!!document.querySelector('.vd-context-menu')`)
	h.mouseText(`.vd-context-menu .vd-context-item`, h.word(key))
}

func hdGateStates() []hdGateState {
	doc := func(h *hdDesigner) map[string]any { return hdDesignerDoc(h.fixture) }
	design := func(h *hdDesigner) { h.freshDoc(doc(h), true) }
	// A change that is not saved yet, then another window saves: the lane cannot be saved. The conflict bar always shows; the sheet opens by itself when the
	// refused PUT is what finds out (409) and by the bar's Resolve button when the stream event does, so both states are brought to what they name.
	conflict := func(h *hdDesigner, sheet bool) {
		design(h)
		h.hdDirty("mem")
		h.fixture.foreignSave(h.t, h.id, "user", func(d map[string]any) { d["name"] = "Changed elsewhere" })
		h.until(20*time.Second, "conflict bar or sheet", `()=>{const b=document.querySelector('.hd-conflict-bar');return (!!b&&!b.hidden)||!!document.querySelector('.hd-root .hd-sheet-layer:not([hidden]) .hd-sheet[data-kind="conflict"]');}`)
		h.pause(300)
		open := h.yes(hdSheetOpen(`[data-kind="conflict"]`))
		if sheet && !open {
			h.mouseClick(`.hd-conflict-bar button`)
			h.until(10*time.Second, "conflict sheet", hdSheetOpen(`[data-kind="conflict"]`))
		}
		if !sheet && open {
			h.press("later")
		}
	}
	saveFailure := func(phase string, script func(h *hdDesigner)) func(h *hdDesigner) {
		return func(h *hdDesigner) {
			design(h)
			script(h)
			h.hdDirty("mem")
			h.until(20*time.Second, "save phase "+phase, `(p)=>{const s=document.querySelector('.hd-save');return !!s&&s.dataset.phase===p;}`, phase)
		}
	}
	// A look sheet: the Look button of the edit bar.
	look := func(h *hdDesigner) {
		design(h)
		h.mouseClick(`.hd-eb-look`)
		h.until(10*time.Second, "look editor", hdSheetOpen(` .hd-look`))
		h.pause(200)
	}
	drawers := func(h *hdDesigner) {
		h.freshDoc(doc(h), true)
		h.resizeWindow(860, 720)
		h.until(5*time.Second, "drawers mode", `()=>document.querySelector('.hd-root').dataset.hdDrawers==='true'`)
	}
	menu := func(h *hdDesigner) {
		h.until(5*time.Second, "desktop menu", `()=>!!document.querySelector('.vd-context-menu')`)
	}
	return []hdGateState{
		{name: "01-empty-page", enter: func(h *hdDesigner) { h.freshDoc(hdGateEmptyDoc(h.fixture), true); h.park() }},
		{name: "02-palette-widgets", enter: func(h *hdDesigner) {
			design(h)
			h.until(10*time.Second, "palette cards", `()=>document.querySelectorAll('.hd-pal-card').length>8`)
			h.park()
		}},
		{name: "03-palette-devices", enter: func(h *hdDesigner) {
			design(h)
			h.mouseClick(`.hd-palette .hd-f-tab[data-tab="entities"]`)
			h.until(15*time.Second, "device rows", `()=>document.querySelectorAll('.hd-ent-row').length>0&&document.querySelector('.hd-pal-status[data-state]').dataset.state==='ready'`)
			h.pause(300)
			h.park()
		}},
		{name: "04-drop-ghost-reflow", held: true, enter: func(h *hdDesigner) {
			design(h)
			cx, cy := h.center(`.hd-pal-card`)
			h.page.Mouse.MustMoveTo(cx, cy)
			h.center(hdFrame("cpu"))
			tx, ty := h.center(hdFrame("cpu"))
			h.drag(cx, cy, tx, ty, 14, func() bool { return false })
			h.until(5*time.Second, "drop ghost over the page", `()=>!!document.querySelector('.hd-root .hd-ghost')&&!!document.querySelector('.hd-root > .hd-drop-avatar[data-hd-drop="add"]')`)
		}, leave: hdHold},
		{name: "05-move-guides", held: true, enter: func(h *hdDesigner) {
			design(h)
			h.center(hdFrame("li"))
			h.center(hdFrame("mem"))
			li, mem := h.rect(hdFrame("li")), h.rect(hdFrame("mem"))
			x, y := mem[0]+mem[2]/2, mem[1]+mem[3]/2
			h.drag(x, y, x, y+(mem[1]-li[1]), 12, func() bool { return false })
			h.until(5*time.Second, "guides while moving", `()=>!!document.querySelector('.hd-root .hd-guide')&&!!document.querySelector('.hd-root[data-hd-gesture="move"]')`)
		}, leave: hdHold},
		{name: "06-resize-limit", held: true, enter: func(h *hdDesigner) {
			design(h)
			h.selectOne("sw")
			fw := h.rect(hdFrame("sw"))[2]
			hx, hy := h.center(`.hd-root .hd-sel .hd-handle[data-edge="se"]`)
			h.drag(hx, hy, hx-fw, hy, 10, func() bool { return false })
			h.until(5*time.Second, "resize at its limit", `()=>!!document.querySelector('.hd-root .hd-handle[data-hd-limit]')&&document.querySelector('.hd-root').dataset.hdGesture==='resize'`)
		}, leave: hdHold},
		{name: "07-marquee", held: true, enter: func(h *hdDesigner) {
			design(h)
			x, y := h.emptyPoint()
			tx, ty := h.center(hdFrame("sw"))
			h.drag(x, y, tx, ty, 14, func() bool { return false })
			h.until(5*time.Second, "marquee over several widgets", `()=>!!document.querySelector('.hd-root .hd-marquee')&&document.querySelectorAll('.hd-root .hd-page > .hd-widget[aria-selected="true"]').length>=2`)
		}, leave: hdHold},
		{name: "08-multi-select", enter: func(h *hdDesigner) {
			design(h)
			h.selectOne("sw")
			h.withKey(input.ShiftLeft, func() { h.mouseClick(hdFrame("li")) })
			h.until(5*time.Second, "two selected", `()=>document.querySelectorAll('.hd-root .hd-page > .hd-widget[aria-selected="true"]').length===2`)
			h.park()
		}, leave: func(h *hdDesigner) { h.page.Keyboard.MustType(input.Escape) }},
		{name: "09-inspector-data", enter: func(h *hdDesigner) {
			design(h)
			h.selectOne("li")
			h.hdTab("data")
			h.until(5*time.Second, "device field", `()=>!!document.querySelector('.hd-inspector .hd-insp-body .hd-ep-box')`)
			h.park()
		}},
		{name: "10-inspector-look", enter: func(h *hdDesigner) {
			design(h)
			h.selectOne("li")
			h.hdTab("look")
			h.until(5*time.Second, "variant gallery", `()=>document.querySelectorAll('.hd-inspector .hd-insp-vhit').length>1`)
			h.park()
		}},
		{name: "11-inspector-options", enter: func(h *hdDesigner) {
			design(h)
			h.selectOne("cpu")
			h.hdTab("options")
			h.until(5*time.Second, "option fields", `()=>document.querySelectorAll('.hd-inspector .hd-insp-body .hd-f').length>1`)
			h.park()
		}},
		{name: "12-inspector-actions", enter: func(h *hdDesigner) {
			design(h)
			h.selectOne("sw")
			h.hdTab("actions")
			h.until(5*time.Second, "action fields", `()=>document.querySelectorAll('.hd-inspector .hd-insp-body .hd-f-select').length>=2`)
			h.park()
		}},
		{name: "13-inspector-multi", enter: func(h *hdDesigner) {
			design(h)
			h.selectOne("sw")
			h.withKey(input.ControlLeft, func() { h.page.Keyboard.MustType(input.KeyA) })
			h.until(5*time.Second, "all four selected", `()=>document.querySelectorAll('.hd-root .hd-page > .hd-widget[aria-selected="true"]').length===4`)
			h.until(5*time.Second, "multi inspector", `()=>!!document.querySelector('.hd-inspector .hd-insp-foot button')&&document.querySelector('.hd-inspector .hd-f-tabs').hidden`)
			h.park()
		}, leave: func(h *hdDesigner) { h.page.Keyboard.MustType(input.Escape) }},
		{name: "14-look-family", enter: func(h *hdDesigner) { look(h); h.park() }},
		{name: "15-look-custom", enter: func(h *hdDesigner) {
			look(h)
			h.mouseText(`.hd-look .hd-f-seg-opt`, h.word("homedash.look.mode_custom"))
			h.until(5*time.Second, "custom colours", `()=>{const c=document.querySelector('.hd-look-custom');return !!c&&!c.hidden;}`)
			h.park()
		}},
		{name: "16-look-surface-grid", enter: func(h *hdDesigner) {
			look(h)
			x, y := h.center(`.hd-look .hd-f-sec`)
			h.page.Mouse.MustMoveTo(x, y)
			h.page.Mouse.MustScroll(0, 4000)
			h.until(5*time.Second, "look sheet scrolled", `()=>[...document.querySelectorAll('.hd-sheet-layer:not([hidden]) *')].some(e=>e.scrollTop>40)`)
			h.park()
		}},
		{name: "17-conflict-sheet", enter: func(h *hdDesigner) { conflict(h, true); h.pause(200); h.park() }},
		{name: "18-save-saving", enter: saveFailure("saving", func(h *hdDesigner) { h.fixture.setPutDelay(4 * time.Second) }), leave: func(h *hdDesigner) { h.fixture.setPutDelay(0) }},
		{name: "19-save-retrying", enter: saveFailure("retrying", func(h *hdDesigner) {
			h.fixture.answerPut(503, map[string]any{"status": "error", "code": "storage_unavailable", "message": "later"}, 99)
		})},
		{name: "20-save-invalid", enter: saveFailure("invalid", func(h *hdDesigner) {
			h.fixture.answerPut(422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid", "issues": []any{map[string]any{"code": "too_long", "path": "/name"}}}, 1)
		})},
		{name: "21-conflict-bar", enter: func(h *hdDesigner) {
			conflict(h, false)
			h.until(5*time.Second, "conflict bar", `()=>{const b=document.querySelector('.hd-conflict-bar');return !!b&&!b.hidden&&!document.querySelector('.hd-root .hd-sheet-layer:not([hidden])');}`)
			h.park()
		}},
		{name: "22-preview-md-custom", enter: func(h *hdDesigner) {
			design(h)
			h.mouseClick(`.hd-bp-opt[data-bp="md"]`)
			h.until(5*time.Second, "tablet preview", `()=>document.querySelector('.hd-bp-opt[data-bp="md"]').getAttribute('aria-checked')==='true'`)
			h.hdDirty("mem")
			h.until(5*time.Second, "custom tablet layout", `()=>{const c=document.querySelector('.hd-bp-chip');return !!c&&!c.hidden&&c.dataset.state==='custom';}`)
			h.park()
		}},
		{name: "23-preview-sm-auto", enter: func(h *hdDesigner) {
			design(h)
			h.mouseClick(`.hd-bp-opt[data-bp="sm"]`)
			h.until(5*time.Second, "phone preview, automatic", `()=>{const c=document.querySelector('.hd-bp-chip');return document.querySelector('.hd-bp-opt[data-bp="sm"]').getAttribute('aria-checked')==='true'&&!!c&&!c.hidden&&c.dataset.state==='auto';}`)
			h.park()
		}},
		{name: "24-page-tabs-menu", held: true, enter: func(h *hdDesigner) { design(h); h.mouseRight(`.hd-ptab[aria-selected="true"]`); menu(h) }, leave: hdHold},
		{name: "25-context-widget", held: true, enter: func(h *hdDesigner) { design(h); h.mouseRight(hdFrame("li")); menu(h) }, leave: hdHold},
		{name: "26-context-canvas", held: true, enter: func(h *hdDesigner) {
			design(h)
			x, y := h.emptyPoint()
			h.mouseRightAt(x, y)
			menu(h)
		}, leave: hdHold},
		{name: "27-drawers-palette", enter: func(h *hdDesigner) {
			drawers(h)
			h.mouseClick(`.hd-eb-drawer[data-hd-cmd="toggle_palette"]`)
			h.until(5*time.Second, "palette drawer", `()=>document.querySelector('.hd-root').dataset.hdPanel==='palette'`)
			h.park()
		}},
		{name: "28-drawers-inspector", enter: func(h *hdDesigner) {
			drawers(h)
			h.selectOne("li")
			h.mouseClick(`.hd-eb-drawer[data-hd-cmd="toggle_inspector"]`)
			h.until(5*time.Second, "inspector drawer", `()=>document.querySelector('.hd-root').dataset.hdPanel==='inspector'`)
			h.park()
		}},
		{name: "29-readonly", enter: func(h *hdDesigner) {
			design(h)
			h.page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
			h.until(10*time.Second, "read-only view", `()=>document.querySelector('.hd-root').dataset.hdMode==='view'&&!!document.querySelector('.hdx-banner[data-banner="readonly"]')`)
			h.park()
		}},
		{name: "30-edit-elsewhere", enter: func(h *hdDesigner) {
			design(h)
			h.page.MustEval(`(id)=>auditTest.openApp('homedash',{dashboard_id:id,forceNew:true})`, h.id)
			h.until(10*time.Second, "second window", `()=>document.querySelectorAll('.vd-window[data-app-id="homedash"]').length===2`)
			h.page.MustEval(`()=>{const l=[...document.querySelectorAll('.vd-window[data-app-id="homedash"]')];l[l.length-1].setAttribute('data-hd-shot','1');}`)
			h.winSel = hdWinSel + `[data-hd-shot]`
			h.until(20*time.Second, "edit elsewhere note", `(w)=>{const e=document.querySelector(w+' .hdx-elsewhere');return !!e&&!e.hidden&&!!document.querySelector(w+' .hd-page');}`, h.winSel)
			h.pause(300)
			h.park()
		}},
		{name: "31-import-issues", enter: func(h *hdDesigner) {
			design(h)
			path := filepath.Join(h.t.TempDir(), "broken.json")
			broken := `{"schema_version":1,"name":"Broken","pages":[{"id":"main","name":"Main","icon":"dashboard","compact":true,"widgets":[` +
				`{"id":"a","type":"switch","variant":"pill","bindings":{"primary":"not an id"},"options":{},"actions":{}},` +
				`{"id":"b","type":"switch","variant":"pill","bindings":{"primary":"system:cpu"},"options":{"nope":1},"actions":{}},` +
				`{"id":"c","type":"light","variant":"slider","bindings":{},"options":{},"actions":{}},` +
				`{"id":"d","type":"hologram","variant":"x","bindings":{},"options":{},"actions":{}}],` +
				`"layouts":{"lg":[{"id":"a","x":0,"y":0,"w":2,"h":1},{"id":"b","x":1,"y":0,"w":2,"h":1},{"id":"c","x":0,"y":4,"w":99,"h":1}],"md":null,"sm":null}}]}`

			if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
				h.t.Fatal(err)
			}
			chosen := h.page.Timeout(20 * time.Second).MustHandleFileDialog()
			h.hdMoreItem("homedash.menu.import")
			chosen(path)
			h.until(15*time.Second, "import issues", `()=>!!document.querySelector('.hd-root .hd-sheet-layer:not([hidden]) .hd-dlg-issues li')`)
			h.pause(200)
			h.park()
		}},
		{name: "32-new-sheet", enter: func(h *hdDesigner) {
			design(h)
			h.mouseClick(`.vd-window-menubar .vd-window-menu-button[data-window-menu="homedash-dashboard"]`)
			h.until(5*time.Second, "Dashboard menu", `()=>!!document.querySelector('.vd-window-menu.open')`)
			h.mouseText(`.vd-window-menu.open .vd-window-menu-item`, h.word("homedash.menu.new"))
			h.until(10*time.Second, "new sheet", hdSheetOpen(` .hd-newcards`))
			h.pause(200)
			h.park()
		}},
		{name: "33-rename-sheet", enter: func(h *hdDesigner) {
			design(h)
			h.hdMoreItem("homedash.menu.rename")
			h.until(10*time.Second, "rename sheet", hdSheetOpen(` input`))
			h.pause(200)
			h.park()
		}},
	}
}

// hdGateEmptyDoc is the dashboard of state 01: the designer document with one empty page.
func hdGateEmptyDoc(f *homeDashFixture) map[string]any {
	d := hdDesignerDoc(f)
	d["name"], d["pages"] = "Empty page", []any{map[string]any{"id": "main", "name": "Main", "icon": "dashboard", "compact": true, "widgets": []any{}, "layouts": map[string]any{"lg": []any{}, "md": nil, "sm": nil}}}
	return d
}

type hdGateResult struct {
	State, Variant string
	DPR            int
	Failures       []string
	Skipped        []string // rows the probe did not measure (reported, never counted as passed)
}

// stage runs fn as a subtest so that one state that cannot be staged (a failed wait, a missing element, a panic of a rod call) is reported and
// the other states still run. h.t is the subtest's T while fn runs.
func (h *hdDesigner) stage(t *testing.T, name string, fn func()) bool {
	parent := h.t
	ok := t.Run(name, func(t *testing.T) {
		h.t = t
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic while staging: %v\n%s", r, h.safeDiag())
			}
		}()
		fn()
	})
	h.t = parent
	return ok
}

func (h *hdDesigner) safeDiag() (s string) {
	defer func() {
		if recover() != nil {
			s = "(no diagnostics)"
		}
	}()
	return h.diag()
}

// hdSafeOutDir resolves an environment-given output directory to an absolute path and refuses one inside the repository (the gate sheets are
// large and must not land in the working tree).
func hdSafeOutDir(t *testing.T, raw string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	abs, err := hdResolveOutDir(raw, wd)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// hdResolveOutDir makes raw absolute (relative to wd) and walks up from wd to the directory holding go.mod; a path at or under it is refused.
func hdResolveOutDir(raw, wd string) (string, error) {
	abs := raw
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(wd, abs)
	}
	abs = filepath.Clean(abs)
	for repo := wd; ; repo = filepath.Dir(repo) {
		if _, err := os.Stat(filepath.Join(repo, "go.mod")); err == nil {
			if rel, err := filepath.Rel(repo, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("output directory %s is inside the repository %s; use a directory outside it", abs, repo)
			}
			break
		}
		if filepath.Dir(repo) == repo {
			break
		}
	}
	return abs, nil
}

func TestHDResolveOutDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd := filepath.Join(root, "ui")
	if err := os.MkdirAll(wd, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	for _, bad := range []string{root, filepath.Join(root, "shots"), "shots", filepath.Join("..", "shots"), ".", filepath.Join(wd, "..", "ui", "x")} {
		if got, err := hdResolveOutDir(bad, wd); err == nil {
			t.Errorf("%q resolved to %s, want a refusal", bad, got)
		}
	}
	for _, good := range []string{outside, filepath.Join(outside, "a", "..", "b")} {
		got, err := hdResolveOutDir(good, wd)
		if err != nil || !filepath.IsAbs(got) {
			t.Errorf("%q: got %q, %v", good, got, err)
		}
	}
	if got, err := hdResolveOutDir(filepath.Join("..", "..", "elsewhere"), wd); err != nil && filepath.Dir(root) != root {
		t.Errorf("a path outside the repository was refused: %s, %v", got, err)
	}
}

func hdTileLabel(name string) string { return strings.Replace(name, "-", " ", 1) }

// TestDesktopHomeDashDesignerGatesBrowser stages every editor state and measures it (gates E-S, E-T, E-F). HOMEDASH_M4_SHOTS=1 runs E-S and E-F,
// =all adds E-T (16 variants at 1x and 2x); HOMEDASH_M4_DIR is required; HOMEDASH_M4_LANG defaults to en (run it with de for the German pass);
// HOMEDASH_M4_STATES is an optional regexp over the state names (for development: only those states run, then the test stops). CI skips it.
func TestDesktopHomeDashDesignerGatesBrowser(t *testing.T) {
	mode, dir, lang := os.Getenv("HOMEDASH_M4_SHOTS"), os.Getenv("HOMEDASH_M4_DIR"), os.Getenv("HOMEDASH_M4_LANG")
	if mode != "1" && mode != "all" {
		t.Skip("set HOMEDASH_M4_SHOTS=1 (E-S, E-F) or all (also E-T) to write the gate sheets")
	}
	if dir == "" {
		t.Fatal("HOMEDASH_M4_DIR is required")
	}
	dir = hdSafeOutDir(t, dir)
	if lang == "" {
		lang = "en"
	}
	only, filtered := regexp.MustCompile(`.`), os.Getenv("HOMEDASH_M4_STATES") != ""
	if filtered {
		only = regexp.MustCompile(os.Getenv("HOMEDASH_M4_STATES"))
	}
	h := newHDDesigner(t, lang)
	var results []hdGateResult
	var notStaged []string
	variants := hdThemeVariants(t)
	pick := func(name string) hdThemeVariant {
		for _, v := range variants {
			if v.Name == name {
				return v
			}
		}
		t.Fatalf("no theme variant %s", name)
		return hdThemeVariant{}
	}
	run := func(group, root string, vs []hdThemeVariant, dprs []int, scale map[int]float64) {
		for _, dpr := range dprs {
			h.page.MustSetViewport(1600, 1000, float64(dpr), false)
			for _, v := range vs {
				h.hardReset()
				h.theme(v)
				var tiles []hdTile
				for i, st := range hdGateStates() {
					if !only.MatchString(st.name) {
						continue
					}
					started := time.Now()
					var p hdProbe
					entered := false
					h.stage(t, fmt.Sprintf("%s/%s/%dx/%s", group, v.Name, dpr, st.name), func() {
						if st.enter == nil {
							h.t.Fatalf("NOT STAGED")
						}
						st.enter(h)
						entered = true
						tiles = append(tiles, hdTile{Label: fmt.Sprintf("%02d %s", i+1, strings.SplitN(st.name, "-", 2)[1]), File: h.capture(filepath.Join(dir, root, v.Name, fmt.Sprintf("%dx", dpr)), st.name)})
						p = h.probe()
						for _, f := range p.failures() {
							h.t.Errorf("%s", f)
						}
						if st.leave != nil {
							st.leave(h)
						}
					})
					if !entered {
						notStaged = append(notStaged, st.name+" "+v.Name)
					}
					results = append(results, hdGateResult{State: st.name, Variant: v.Name, DPR: dpr, Failures: p.failures(), Skipped: p.skipped()})
					t.Logf("%s %s %dx %s staged=%v in %v, %d probe failures", group, v.Name, dpr, st.name, entered, time.Since(started).Round(10*time.Millisecond), len(p.failures()))
				}
				name := fmt.Sprintf("%s-%s-%dx.png", group, v.Name, dpr)
				if group == "ES" {
					name = fmt.Sprintf("%s-%s.png", group, v.Name)
				}
				writeHDSheet(t, filepath.Join(dir, name), tiles, 3, scale[dpr])
			}
		}
	}
	run("ES", "ES", []hdThemeVariant{pick("fruity-light"), pick("fruity-dark")}, []int{1}, map[int]float64{1: 0.5})
	if filtered {
		writeHDIndex(dir, results, notStaged)
		return
	}
	h.theme(pick("fruity-light"))
	h.stage(t, "layout-shift", func() {
		for _, bad := range h.hdLayoutShift() {
			h.t.Errorf("layout shift: %s", bad)
		}
	})
	h.stage(t, "focus-walk", func() {
		for _, bad := range h.hdFocusWalk(60) {
			h.t.Errorf("focus ring: %s", bad)
		}
	})
	h.stage(t, "families", func() { h.runFamilies(h.t, dir, pick) })
	if mode == "all" {
		run("ET", "ET", variants, []int{1, 2}, map[int]float64{1: 0.5, 2: 0.25})
	}
	h.stage(t, "coarse", func() { h.coarseProbe(h.t, dir) })
	if errs := h.safeStr(`()=>window.hdErrs.join('\n')`); errs != "" {
		t.Errorf("page errors during the gates:\n%s", errs)
	}
	raw, _ := json.MarshalIndent(results, "", " ")
	_ = os.WriteFile(filepath.Join(dir, "probes.json"), raw, 0o644)
	writeHDIndex(dir, results, notStaged)
}

func (h *hdDesigner) safeStr(js string) (s string) {
	defer func() {
		if recover() != nil {
			s = ""
		}
	}()
	return h.str(js)
}

// writeHDIndex writes index.html: the sheets, the states that could not be staged and every failed probe by state and variant.
func writeHDIndex(dir string, results []hdGateResult, notStaged []string) {
	kinds := map[string]int{}
	var rows, skips strings.Builder
	for _, r := range results {
		for _, f := range r.Skipped {
			skips.WriteString("<tr><td>" + html.EscapeString(r.State+" "+r.Variant+fmt.Sprintf(" %dx", r.DPR)) + "</td><td>" + html.EscapeString(f) + "</td></tr>\n")
		}
		for _, f := range r.Failures {
			kinds[strings.SplitN(f, " ", 2)[0]]++
			rows.WriteString("<tr><td>" + html.EscapeString(r.State+" "+r.Variant+fmt.Sprintf(" %dx", r.DPR)) + "</td><td>" + html.EscapeString(f) + "</td></tr>\n")
		}
	}
	var counts []string
	for k, n := range kinds {
		counts = append(counts, fmt.Sprintf("%s: %d", k, n))
	}
	sort.Strings(counts)
	var sheets strings.Builder
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".png") {
			sheets.WriteString(`<li><a href="` + html.EscapeString(e.Name()) + `">` + html.EscapeString(e.Name()) + "</a></li>\n")
		}
	}
	ns := "none"
	if len(notStaged) > 0 {
		ns = html.EscapeString(strings.Join(notStaged, ", "))
	}
	index := "<!doctype html><meta charset=utf-8><title>HomeDash M4 gates</title><h1>Gate sheets</h1><ul>" + sheets.String() + "</ul><p>Not staged: " + ns + "</p><h2>Failing probes (" +
		html.EscapeString(strings.Join(counts, ", ")) + ")</h2><table>" + rows.String() + "</table>\n"
	if skips.Len() > 0 {
		index += "<h2>Skipped probes (not measured, not passed)</h2><table>" + skips.String() + "</table>\n"
	}
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte(index), 0o644)
}

// runFamilies is gate E-F: the designer on each of the four family canvases in fruity-light and fruity-dark, twice: a widget selected (outline, chip and handles
// against that family's canvas) and a palette card held over empty cells (ghost, avatar). Shots go to EF/<family>-<kind>-<variant>.png and the sheet to EF.png
// (light | dark per row; per family the selection row first, then the drag row).
func (h *hdDesigner) runFamilies(t *testing.T, dir string, pick func(string) hdThemeVariant) {
	t.Helper()
	var tiles []hdTile
	for _, family := range []string{"glass", "tactile", "minimal", "instrument"} {
		byKind := map[string][]hdTile{}
		for _, name := range []string{"fruity-light", "fruity-dark"} {
			h.hardReset()
			h.theme(pick(name))
			id := h.fixture.seedDashboard(hdDesignerDoc(h.fixture))
			h.fixture.setFamily(t, id, family)
			h.id = id
			h.openDashboard(id)
			h.enterEdit()
			h.pause(200)
			h.selectOne("li")
			h.park()
			byKind["select"] = append(byKind["select"], hdTile{Label: family + " selected " + name, File: h.capture(filepath.Join(dir, "EF"), family+"-selected-"+name)})
			for _, f := range h.probe().failures() {
				t.Errorf("EF %s %s selected: %s", family, name, f)
			}
			h.page.Keyboard.MustType(input.Escape)
			cx, cy := h.center(`.hd-pal-card`)
			x, y := h.emptyPoint()
			h.drag(cx, cy, x, y, 14, func() bool { return false })
			h.until(5*time.Second, "drop ghost on empty cells", `()=>!!document.querySelector('.hd-root .hd-ghost')`)
			byKind["drag"] = append(byKind["drag"], hdTile{Label: family + " drag " + name, File: h.capture(filepath.Join(dir, "EF"), family+"-drag-"+name)})
			for _, f := range h.probe().failures() {
				t.Errorf("EF %s %s drag: %s", family, name, f)
			}
			hdHold(h)
		}
		tiles = append(tiles, byKind["select"]...)
		tiles = append(tiles, byKind["drag"]...)
	}
	writeHDSheet(t, filepath.Join(dir, "EF.png"), tiles, 2, 0.5)
}

// coarseProbe re-runs the hit-size probe with touch emulation on (UNVERIFIED that emulation flips (pointer: coarse); if it does not, it logs and skips).
func (h *hdDesigner) coarseProbe(t *testing.T, dir string) {
	t.Helper()
	five := 5
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &five}).Call(h.page); err != nil {
		t.Logf("touch emulation unavailable: %v", err)
		return
	}
	defer func() { _ = (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(h.page) }()
	if !h.yes(`()=>matchMedia('(pointer: coarse)').matches`) {
		t.Log("touch emulation does not flip (pointer: coarse); coarse hit pass skipped")
		return
	}
	for _, name := range []string{"02-palette-widgets", "06-resize-limit", "09-inspector-data"} {
		for _, st := range hdGateStates() {
			if st.name != name || st.enter == nil {
				continue
			}
			st.enter(h)
			p := h.probe()
			t.Logf("coarse %s: %d hit rows", name, len(p.Hits))
			for _, r := range p.Hits {
				if !r.Pass {
					t.Errorf("coarse %s: hits %s %q %vx%v", name, r.El, r.Name, r.W, r.H)
				}
			}
			if st.leave != nil {
				st.leave(h)
			}
		}
	}
	_ = dir
}

// hdFocusWalk presses Tab n times from the Done button of a designer with a widget selected and returns the failed focus probes of the stops inside the
// window (each stop needs a 3:1 ring of at least 2 px).
func (h *hdDesigner) hdFocusWalk(n int) []string {
	h.freshDoc(hdDesignerDoc(h.fixture), true)
	h.selectOne("li")
	h.page.MustEval(`()=>document.querySelector('.hd-eb-done').focus()`)
	var bad []string
	stops := map[string]bool{}
	for i := 0; i < n; i++ {
		h.page.Keyboard.MustType(input.Tab)
		for _, r := range h.probe().UI {
			if r.What != "focus" {
				continue
			}
			stops[r.El] = true
			if !r.Pass {
				bad = append(bad, fmt.Sprintf("stop %d %s ratio=%v width=%v", i, r.El, r.Ratio, r.W))
			}
		}
	}
	h.t.Logf("focus walk: %d distinct stops in %d tabs", len(stops), n)
	if len(stops) < 10 {
		bad = append(bad, fmt.Sprintf("the walk reached only %d distinct elements", len(stops)))
	}
	return bad
}

// hdLayoutShift selects and deselects every widget of the designer document with the pointer and returns where the rects of widgets, edit bar or
// inspector differ from the unselected layout.
func (h *hdDesigner) hdLayoutShift() []string {
	h.freshDoc(hdDesignerDoc(h.fixture), true)
	var bad []string
	for _, id := range []string{"sw", "li", "cpu", "mem"} {
		before := h.rects()
		h.selectOne(id)
		h.pause(150)
		if d := rectDiff(before, h.rects()); d != "" {
			bad = append(bad, "selecting "+id+": "+d)
		}
		h.page.Keyboard.MustType(input.Escape)
		h.until(5*time.Second, "deselected "+id, `()=>document.querySelectorAll('.hd-root .hd-page > .hd-widget[aria-selected="true"]').length===0`)
		h.pause(150)
		if d := rectDiff(before, h.rects()); d != "" {
			bad = append(bad, "deselecting "+id+": "+d)
		}
	}
	return bad
}
