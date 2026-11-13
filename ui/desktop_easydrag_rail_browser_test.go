package ui

import (
	"testing"
)

// TestEasyDragRailCaptionsBrowser checks the labelled rail: seven buttons, 64 px wide, each with an
// unclipped caption under its icon that stays inside its button, at 1440, 900 and 390 px (the width
// where the editor is a phone layout). The rail is a column at every width, so nothing may scroll.
func TestEasyDragRailCaptionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-rail button').length===7`)

	measure := `()=>{
		const rail=document.querySelector('.ed-rail'), rr=rail.getBoundingClientRect(), out=[];
		const buttons=[...rail.querySelectorAll('button')];
		buttons.forEach(b=>{
			const id=b.dataset.edCmd||b.getAttribute('aria-label');
			const cap=b.querySelector('.ed-rail-caption'), icon=b.querySelector('.ed-icon');
			if(!cap||!cap.textContent.trim()){out.push(id+': no caption');return;}
			if(getComputedStyle(cap).display==='none')out.push(id+': caption hidden');
			const r=cap.getBoundingClientRect(), br=b.getBoundingClientRect(), ir=icon.getBoundingClientRect();
			if(cap.scrollWidth>cap.clientWidth)out.push(id+': caption clipped, scrollWidth '+cap.scrollWidth+' > clientWidth '+cap.clientWidth);
			if(r.left<br.left-0.5||r.right>br.right+0.5||r.bottom>br.bottom+0.5)out.push(id+': caption leaves its button');
			if(r.top<ir.bottom-0.5)out.push(id+': caption overlaps its icon');
			if(br.left<rr.left-0.5||br.right>rr.right+0.5)out.push(id+': button leaves the rail');
			if(br.height<44)out.push(id+': button is only '+Math.round(br.height)+' px tall');
			if(parseFloat(getComputedStyle(cap).fontSize)<10.9)out.push(id+': caption font is '+getComputedStyle(cap).fontSize);
		});
		if(rail.scrollHeight>rail.clientHeight+1)out.push('the rail scrolls: '+rail.scrollHeight+' > '+rail.clientHeight);
		return JSON.stringify({n:buttons.length,width:Math.round(rr.width),problems:out});
	}`

	for _, vp := range []struct {
		name          string
		width, height int
		mobile        bool
	}{{"1440", 1440, 900, false}, {"900", 900, 700, false}, {"390", 390, 844, true}} {
		p.MustSetViewport(vp.width, vp.height, 1, vp.mobile)
		p.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,250))))`)
		got := p.MustEval(measure).Str()
		if got != `{"n":7,"width":64,"problems":[]}` {
			t.Fatalf("rail at %s px: %s", vp.name, got)
		}
		s.shot("rail-captions-" + vp.name)
	}

	// Each rail button is named by its visible caption (WCAG 2.5.3): no aria-label, a caption that
	// screen readers see, and no other text in the button.
	if got := p.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.ed-rail button')].filter(b=>{
		const cap=b.querySelector('.ed-rail-caption');
		return b.hasAttribute('aria-label')||b.hasAttribute('aria-labelledby')||!cap||cap.closest('[aria-hidden="true"]')||b.textContent.trim()!==cap.textContent.trim()||!b.title;
	}).map(b=>b.dataset.edCmd||b.title))`).Str(); got != "[]" {
		t.Fatalf("rail buttons whose accessible name is not their caption: %s", got)
	}

	// At 900 and 390 px the palette and the outline float over the canvas. They start right of the
	// rail, and the end of every caption still receives the click.
	const open = `(which)=>{
		const ed=edFixture.editor();
		if(which==='palette'){const b=document.querySelector('.ed-rail [data-ed-cmd="palette"]');if(b.getAttribute('aria-pressed')!=='true')b.click();return 'ok';}
		const label=ed.t('easydrag.ui.node_list');
		const b=[...document.querySelectorAll('.ed-rail button')].find(x=>x.title===label);
		if(!b)return 'no outline button';
		if(b.getAttribute('aria-pressed')!=='true')b.click();
		return 'ok';
	}`
	const check = `(sel)=>{
		const panel=document.querySelector(sel), rail=document.querySelector('.ed-rail');
		if(!panel||panel.getBoundingClientRect().width===0)return sel+' is not shown';
		const pr=panel.getBoundingClientRect(), rr=rail.getBoundingClientRect(), out=[];
		if(pr.left<rr.right-0.5)out.push(sel+' left '+pr.left+' < rail right '+rr.right);
		rail.querySelectorAll('button').forEach(b=>{
			const cap=b.querySelector('.ed-rail-caption').getBoundingClientRect();
			const hit=document.elementFromPoint(cap.right-1,(cap.top+cap.bottom)/2);
			if(!hit||!b.contains(hit))out.push((b.title||b.dataset.edCmd)+': caption end is covered by '+(hit?hit.className||hit.tagName:'nothing'));
		});
		return out.join('; ');
	}`
	for _, vp := range []struct {
		name          string
		width, height int
		mobile        bool
	}{{"900", 900, 700, false}, {"390", 390, 844, true}} {
		p.MustSetViewport(vp.width, vp.height, 1, vp.mobile)
		p.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,250))))`)
		for _, panel := range []struct{ which, sel string }{{"palette", ".ed-palette"}, {"outline", ".ed-node-region.is-outline"}} {
			if got := p.MustEval(open, panel.which).Str(); got != "ok" {
				t.Fatalf("%s at %s px: %s", panel.which, vp.name, got)
			}
			p.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(r,300))))`)
			if got := p.MustEval(check, panel.sel).Str(); got != "" {
				t.Fatalf("%s at %s px: %s", panel.which, vp.name, got)
			}
		}
	}
	s.noRawKeys("rail")
	s.failOnPageErrors("rail")
}
