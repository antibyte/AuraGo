package ui

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// easyDragTargetScan measures every pointer target inside the EasyDrag windows (WCAG 2.5.8, 24 CSS
// px). Two kinds of control are measured by their real target instead of their own box, and the
// reason sits next to each:
//   - .ed-switch--small is drawn 30x17, and its ::before widens the target to 34x25;
//   - .ed-flow-select input is a 20x20 box inside a 44 px label, and the label is the target.
//
// Skipped: hidden controls, controls with pointer-events:none, and a link that sits inline in
// running text (the text around it is not part of the link).
const easyDragTargetScan = `()=>{
	const MIN=24;
	const exceptions=[
		{selector:'.ed-switch--small',how:'before'},
		{selector:'.ed-flow-select input',how:'label'}
	];
	const squash=s=>s.replace(/\s+/g,' ').trim();
	const offenders=[];
	const viaTarget={};
	const seen=new Set();
	document.querySelectorAll('.ed-app').forEach(app=>{
		app.querySelectorAll('button,a[href],input,select,[role="button"]').forEach(el=>{
			if(seen.has(el))return;
			seen.add(el);
			if(el.matches('input[type="hidden"]'))return;
			const cs=getComputedStyle(el),r=el.getBoundingClientRect();
			if(cs.display==='none'||cs.visibility==='hidden'||cs.pointerEvents==='none'||(r.width===0&&r.height===0))return;
			if(el.matches('a[href]')&&cs.display==='inline'){
				const p=el.parentElement;
				if(p&&squash(p.textContent)!==squash(el.textContent))return;
			}
			// offsetWidth/offsetHeight are the layout box: the hover reveal of .ed-port-add scales it to 0.6 until the pointer arrives.
			let w=Math.max(r.width,el.offsetWidth),h=Math.max(r.height,el.offsetHeight);
			const ex=exceptions.find(e=>el.matches(e.selector));
			if(ex)viaTarget[ex.selector]=(viaTarget[ex.selector]||0)+1;
			if(ex&&ex.how==='before'){
				const b=getComputedStyle(el,'::before');
				w=parseFloat(b.width)||0;
				h=parseFloat(b.height)||0;
			}
			if(ex&&ex.how==='label'){
				const label=el.closest('label');
				if(label){const lr=label.getBoundingClientRect();w=lr.width;h=lr.height;}
			}
			if(w>=MIN-0.01&&h>=MIN-0.01)return;
			const data=[...el.attributes].map(a=>a.name).find(n=>n.startsWith('data-ed'))||'';
			const classes=typeof el.className==='string'?el.className.trim():'';
			offenders.push({
				selector:el.tagName.toLowerCase()+(classes?'.'+classes.split(/\s+/).join('.'):'')+(data?'['+data+']':''),
				classes:classes,
				width:Math.round(w*10)/10,
				height:Math.round(h*10)/10
			});
		});
	});
	return JSON.stringify({scanned:seen.size,viaTarget:viaTarget,offenders:offenders});
}`

// checkTargets fails the test when a visible target in an EasyDrag window is under 24x24 CSS px.
func (s *easyDragSmoke) checkTargets(step string) {
	s.t.Helper()
	var result struct {
		Scanned   int            `json:"scanned"`
		ViaTarget map[string]int `json:"viaTarget"`
		Offenders []struct {
			Selector string  `json:"selector"`
			Classes  string  `json:"classes"`
			Width    float64 `json:"width"`
			Height   float64 `json:"height"`
		} `json:"offenders"`
	}
	if err := json.Unmarshal([]byte(s.page.MustEval(easyDragTargetScan).Str()), &result); err != nil {
		s.t.Fatalf("%s: read the target scan: %v", step, err)
	}
	if result.Scanned < 10 {
		s.t.Fatalf("%s: the scan found only %d controls; it no longer sees the EasyDrag window", step, result.Scanned)
	}
	s.t.Logf("%s: %d controls scanned, measured by their real target: %v", step, result.Scanned, result.ViaTarget)
	var report []string
	for _, o := range result.Offenders {
		report = append(report, o.Selector+" is "+formatTargetSize(o.Width, o.Height)+" (classes: "+o.Classes+")")
	}
	if len(report) > 0 {
		s.t.Errorf("%s: %d of %d controls are under 24x24 CSS px:\n  %s", step, len(report), result.Scanned, strings.Join(report, "\n  "))
	}
}

func formatTargetSize(w, h float64) string {
	return strconv.FormatFloat(w, 'f', -1, 64) + "x" + strconv.FormatFloat(h, 'f', -1, 64)
}

// TestEasyDragTargetSizes is the A5 check (U9, V4): on the start page and in the editor of the
// seeded flow at 1440x900, every button, link, input, select and role=button in the EasyDrag window
// is at least 24x24 CSS px, apart from the two documented exceptions in easyDragTargetScan.
func TestEasyDragTargetSizes(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && document.querySelectorAll('.ed-template-card').length===3`)
	s.checkTargets("start page")
	s.page.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	s.checkTargets("editor")
	s.failOnPageErrors("target sizes")
}
