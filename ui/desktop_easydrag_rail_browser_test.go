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
	s.noRawKeys("rail")
	s.failOnPageErrors("rail")
}
