package ui

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// TestEasyDragSmokeSetsReducedMotion: a page that startEasyDragSmoke opens reports
// prefers-reduced-motion: reduce, so no test depends on the machine's animation setting.
func TestEasyDragSmokeSetsReducedMotion(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	if !s.page.MustEval(`()=>matchMedia('(prefers-reduced-motion: reduce)').matches`).Bool() {
		t.Fatal("the page does not report prefers-reduced-motion: reduce")
	}
}

// easyDragRunningAnimations lists the animations that run inside .ed-app right now ("name on element"
// joined by " | "). A finished animation with fill-mode both stays in getAnimations(), so only
// playState "running" counts.
const easyDragRunningAnimations = `()=>{
	const app=document.querySelector('.ed-app');
	return document.getAnimations().filter(a=>a.playState==='running' && a.effect && a.effect.target && app.contains(a.effect.target))
		.map(a=>(a.animationName||a.transitionProperty||a.constructor.name)+' on '+a.effect.target.tagName+'.'+String(a.effect.target.getAttribute('class')||'').split(' ')[0]).join(' | ');
}`

// easyDragSettle waits long enough for every one-shot entrance (the longest is 360 ms) to end.
func easyDragSettle(page *rod.Page) {
	page.MustEval(`()=>new Promise(r=>setTimeout(()=>requestAnimationFrame(()=>requestAnimationFrame(r)),1000))`)
}

// TestEasyDragMotionIsCalmWhenIdle checks the motion gate in the real editor (spec 6.4):
//  1. normal motion, idle editor: nothing runs;
//  2. normal motion, a step running: something runs (so the first check is not vacuous);
//  3. Ruhig, chosen through the editor's more menu, the same step still running: nothing runs.
func TestEasyDragMotionIsCalmWhenIdle(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page

	// The helper leaves the media query alone; say "no preference" explicitly, and turn the desktop's
	// animations on (the fixture shell sets windows.animations to false).
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "no-preference"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(p); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	p.MustEval(`()=>{document.body.dataset.animations='true';}`)

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelector('[data-ed-state]').textContent.length>0`)
	s.wait(`()=>document.querySelector('.ed-app').dataset.motion==='normal'`)
	if !p.MustEval(`()=>!matchMedia('(prefers-reduced-motion: reduce)').matches && document.body.dataset.animations==='true'`).Bool() {
		t.Fatal("the page still asks for reduced motion; the idle check would be vacuous")
	}

	// 1. Idle editor, normal motion.
	easyDragSettle(p)
	if got := p.MustEval(easyDragRunningAnimations).Str(); got != "" {
		t.Fatalf("idle editor, normal motion: running animations: %s", got)
	}

	// 2. A step runs, normal motion: at least one animation runs.
	p.MustEval(`()=>{edFixture.state.delay=600000}`)
	s.fit()
	p.KeyActions().Press(input.ControlLeft).Type(input.Enter).MustDo()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]') && document.querySelectorAll('.ed-modal .ed-effects li').length>=1`)
	p.MustEval(`()=>{const s=document.querySelector('.ed-modal [data-ed-test-mode]');s.value='real';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node.status-running').length===1`)
	easyDragSettle(p)
	if got := p.MustEval(easyDragRunningAnimations).Str(); got == "" {
		t.Fatal("normal motion with a running step: no animation runs; the gate test would be vacuous")
	} else {
		t.Logf("normal motion, running step: %s", got)
	}

	// 3. Ruhig through the real menu entry; the step keeps running, nothing moves.
	p.MustElement(`[data-ed-cmd="more"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.vd-context-menu')`)
	clicked := p.MustEval(`()=>{
		const parent=[...document.querySelectorAll('.vd-context-menu .vd-context-item')].find(b=>b.textContent.includes('Lebendigkeit'));
		const calm=[...document.querySelectorAll('.vd-context-submenu-popover .vd-context-item')].find(b=>b.textContent.trim().startsWith('Ruhig'));
		if(!parent||!calm)return 'the Lebendigkeit entry or its Ruhig choice is missing from the more menu';
		calm.click();
		return 'ok';
	}`).Str()
	if clicked != "ok" {
		t.Fatal(clicked)
	}
	s.wait(`()=>document.querySelector('.ed-app').dataset.motion==='calm' && localStorage.getItem('aurago.easydrag.liveliness')==='calm'`)
	easyDragSettle(p)
	if p.MustEval(`()=>document.querySelectorAll('.ed-node.status-running').length`).Int() != 1 {
		t.Fatal("the step stopped running; the Ruhig check would be vacuous")
	}
	if got := p.MustEval(easyDragRunningAnimations).Str(); got != "" {
		t.Fatalf("Ruhig with a running step: running animations: %s", got)
	}
	s.failOnPageErrors("motion")
}
