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
	r.sceneStatusSheet()
	r.sceneCategorySheet()
	r.openEditor()
	r.sceneIdleEditor()
	r.sceneRunDone()
	r.sceneTestResult()
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
	const splitShadows=v=>{const out=[];let d=0,cur='';for(const ch of String(v||'')){if(ch==='(')d++;if(ch===')')d--;if(ch===','&&d===0){out.push(cur.trim());cur='';}else cur+=ch;}if(cur.trim()&&cur.trim()!=='none')out.push(cur.trim());return out;};
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
		const st=el.getAttribute('data-status'),cat=el.getAttribute('data-cat')||(/tile/.test(String(el.className))&&el.parentElement&&el.parentElement.closest('[data-cat]')?el.parentElement.closest('[data-cat]').getAttribute('data-cat'):''),surf=el.closest('[data-lab-surface]');
		return el.tagName.toLowerCase()+(cls.length?'.'+cls.join('.'):'')+(st?'[status='+st+']':'')+(cat?'[cat='+cat+']':'')+(data?'['+data+']':'')+(surf?'@'+surf.dataset.labSurface:'');
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
			const fgc=parse(cs.color);
			let svg=null;
			const host=el.matches('svg')?el:el.querySelector('svg');
			const shape=host&&host.querySelector('circle,path,rect,polygon,ellipse');
			if(shape){
				const sc=getComputedStyle(shape),f=parse(sc.fill),s=parse(sc.stroke);
				const pick=f&&f[3]>0?f:(s&&s[3]>0?s:null);
				if(pick)svg=over(pick,b.rgb,op);
			}
			// ring: the outermost solid ring a box-shadow draws (0 blur, positive spread, not inset), over what is behind the element.
			// Part A's --ed-status-edge is such a ring; a disc is told apart by its fill or by this ring (spec 7).
			let ring=null;
			splitShadows(cs.boxShadow).forEach(sh=>{
				const col=sh.match(/(rgba?\([^)]*\)|color\([^)]*\)|oklab\([^)]*\)|oklch\([^)]*\)|lab\([^)]*\)|lch\([^)]*\)|#[0-9a-f]{3,8})/i);
				if(!col||/\binset\b/.test(sh))return;
				const n=sh.replace(col[0],'').trim().split(/\s+/).map(parseFloat);
				if(n.length>=4&&n[2]===0&&n[3]>0){const c=parse(col[1]);if(c&&c[3]>0)ring=over(c,b.rgb,op);}
			});
			let edge=null;
			if(parseFloat(cs.borderLeftWidth)>0){const c=parse(cs.borderLeftColor);if(c&&c[3]>0)edge=over(c,b.rgb,op);}
			const surf=el.closest('[data-lab-surface]');
			out.elements.push({probe:p.name,role:p.role,name:describe(el),text:(el.textContent||'').trim().slice(0,40),
				status:el.getAttribute('data-status')||'',cat:el.getAttribute('data-cat')||'',surface:surf?surf.dataset.labSurface:'',
				op:op,ownRaw:own&&own[3]>0?own:null,fgRaw:fgc,behind:round(b.rgb),own:round(ownOver),svg:round(svg),ring:round(ring),edge:round(edge),approx:b.approx});
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
		// shown: a visible box that no ancestor clips away. A visually hidden ancestor (overflow hidden at 2px or less, as .ed-sr-only,
		// or clip: rect(0 0 0 0)) leaves its content to screen readers only, so a text inside it is not a text a user can see clipped.
		const shown=el=>{
			if(!vis(el))return false;
			for(let n=el.parentElement;n&&n!==document.body;n=n.parentElement){
				const cs=getComputedStyle(n);
				if(cs.visibility==='hidden'||n.inert)return false;
				const r=n.getBoundingClientRect();
				if((cs.overflowX!=='visible'||cs.overflowY!=='visible')&&(r.width<=2||r.height<=2))return false;
				if(/^rect\((0(px)?[ ,]*){4}\)$/.test(cs.clip))return false;
			}
			return true;
		};
		roots.forEach(root=>root.querySelectorAll('*').forEach(el=>{
			if(el.matches('input,textarea,select,svg,svg *,script,style')||!vis(el))return;
			const cs=getComputedStyle(el);
			if(cs.display==='inline'||cs.display==='contents'||!el.clientWidth)return;
			if(![...el.childNodes].some(n=>n.nodeType===3&&n.textContent.trim()))return;
			if(el.scrollWidth<=el.clientWidth+1)return;
			if(/auto|scroll/.test(cs.overflowX))return;
			if(!shown(el))return;
			// the title: on the element itself or on the nearest control (button, link, role button or option) around it,
			// the one the pointer hovers
			const ctl=el.closest('button,a[href],[role="button"],[role="option"]');
			const own=!!(el.getAttribute('title')||'').trim(),viaCtl=!own&&!!ctl&&!!(ctl.getAttribute('title')||'').trim();
			const hasTitle=own||viaCtl,ell=cs.textOverflow==='ellipsis';
			if(ell&&hasTitle)return;
			out.clipped.push({name:describe(el),text:(el.textContent||'').trim().slice(0,40),sw:el.scrollWidth,cw:el.clientWidth,ellipsis:ell,title:hasTitle,looked:ctl?'the element and its control '+describe(ctl):'the element (no control around it)'});
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
	r.check("6-home", labSpec{Names: true, Probes: labStatusProbes(0)}) // the home page shows no status disc (probe: 0 on HOME), so Min is 0
	s.checkTargets(r.theme + "/6-home")
}

// labSheetJS lays a lab sheet over the open EasyDrag window. kind is "status" or "categories". The sheet is a real child
// of .ed-app, so the tokens, the status classes and the kit render as in the product. data-lab-surface names the
// surface a column draws on; the measure script reports it in every element name.
const labSheetJS = `(kind,args)=>{
	const ED=window.EasyDrag,tr=window.t,app=document.querySelector('.ed-app');
	const toEl=x=>{if(typeof x!=='string')return x;const d=document.createElement('div');d.innerHTML=x.trim();return d.firstElementChild;};
	const old=app.querySelector('.ed-lab-sheet');if(old)old.remove();
	const sheet=document.createElement('div');sheet.className='ed-lab-sheet';
	sheet.style.cssText='position:fixed;inset:0;z-index:99999;overflow:auto;box-sizing:border-box;padding-top:64px;display:flex;flex-wrap:wrap;align-content:flex-start;background:var(--ed-bg);color:var(--ed-text);font:13px/1.4 system-ui,sans-serif';
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
		{Name: "group", Selector: `.ed-group-frame[class*="ed-group--"]`, Role: "edge", Min: 5},
		// The neutral frame is grey. Like the grey statuses (amendment 3) and easyDragS0GroupNames in the static colour test,
		// it is checked for contrast on the well (3:1) but not for distance to the status colours.
		{Name: "group-neutral", Selector: `.ed-group-frame:not([class*="ed-group--"])`, Role: "edge", Min: 1},
		{Name: "ref-status", Selector: "[data-lab-ref] .ed-status-disc", Role: "ref", Min: 4},
	}})
	r.add(labDistanceFindings(r.theme, "2-category-sheet", res.Elements))
	s.page.MustEval(labSheetOffJS)
}

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
	// The seeded flow has effects, so Test opens in Simulation; the lab runs the real test (as the boundary journey does).
	s.page.MustEval(`()=>{const m=document.querySelector('.ed-modal [data-ed-test-mode]');if(m){m.value='real';m.dispatchEvent(new Event('change',{bubbles:true}));}}`)
	s.page.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return !!ed.run && ed.run.id!=='run_contact' && ed.run.status==='success'}`)
	s.wait(`()=>{const p=document.querySelector('.ed-run-results');return !!p && !p.hidden && p.getClientRects().length>0}`)
	s.clearToasts()
	s.page.MustEval(labResultCalloutJS)
	r.check("5-test-result", labSpec{Names: true, Probes: append(labStatusProbes(1),
		labProbe{Name: "disclosure", Selector: ".ed-disclosure-head", Role: "text", Min: 1})})
	s.checkTargets(r.theme + "/5-test-result")
}

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

// TestEasyDragLabPseudoBrowser is the pseudo-locale run of the lab (scene 3 with localStorage['aurago.easydrag.pseudo']
// set), a test of its own because the whole lab took more than four minutes (spec §7).
func TestEasyDragLabPseudoBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, name := range labContexts(t) {
		name := name
		t.Run(name, func(t *testing.T) { labPseudo(t, name) })
	}
}
