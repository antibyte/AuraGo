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
	r.check("6-home", labSpec{Names: true, Probes: labStatusProbes(0)}) // the home page shows no status disc (probe: 0 on HOME), so Min is 0
	s.checkTargets(r.theme + "/6-home")
}
