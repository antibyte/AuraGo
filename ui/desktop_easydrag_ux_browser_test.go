package ui

import (
	"fmt"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopEasyDragUXBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	page := s.page
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)

	page.MustElement(`[data-ed-home-search]`).MustInput("no matching flow")
	s.wait(`()=>!!document.querySelector('[data-ed-home-clear-search]') && !document.querySelector('.ed-flow-card')`)
	page.MustElement(`[data-ed-home-clear-search]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	page.MustEval(`()=>{edFixture.state.failLists.push({status:500,code:'FLOW_INTERNAL'});document.dispatchEvent(new CustomEvent('aurago:flows-changed',{detail:{reason:'published'}}));}`)
	s.wait(`()=>!!document.querySelector('.ed-home-stale') && document.querySelectorAll('.ed-flow-card').length===1 && document.querySelector('[data-ed-new]').disabled`)
	page.MustElement(`[data-ed-home-retry]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-home-stale') && !document.querySelector('[data-ed-new]').disabled`)
	page.MustElement(`.ed-flow-card h3`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-canvas')`)

	// Standard has one appearance; Fruity supplies the existing light/dark variants.
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		for _, width := range []int{320, 390, 768, 1024, 1440} {
			t.Run(fmt.Sprintf("%s/%d", theme, width), func(t *testing.T) {
				parent := s.t
				s.t = t
				defer func() { s.t = parent }()
				defer func() {
					if t.Failed() {
						s.shot(fmt.Sprintf("ux-failed-%s-%d", theme, width))
						t.Log(page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.ed-editor,.ed-body,.ed-canvas,.ed-detail-backdrop,.ed-detail')].map(e=>({class:e.className,width:e.getBoundingClientRect().width,left:e.getBoundingClientRect().left,right:e.getBoundingClientRect().right,scroll:e.scrollWidth,client:e.clientWidth})))`).Str())
					}
				}()
				if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: width < 760}).Call(page); err != nil {
					t.Fatal(err)
				}
				page.MustSetViewport(width, 900, 1, width < 760)
				page.MustEval(`theme=>fixtureTheme(theme)`, theme)
				s.wait(`()=>{const app=document.querySelector('.ed-app'),head=document.querySelector('.ed-head'),box=head.getBoundingClientRect();return app && box.width>0 && box.left>=-1 && box.right<=innerWidth+1 && head.scrollWidth<=head.clientWidth+1 && document.querySelector('.ed-canvas').getBoundingClientRect().width>100;}`)
				s.wait(`()=>['test','publish'].every(cmd=>{const b=document.querySelector('[data-ed-cmd="'+cmd+'"]'),label=b.querySelector('span'),r=b.getBoundingClientRect();return r.width>0 && r.left>=-1 && r.right<=innerWidth+1 && getComputedStyle(label).display!=='none';})`)
				if width < 760 {
					s.wait(`()=>['test','publish','more'].every(cmd=>{const r=document.querySelector('[data-ed-cmd="'+cmd+'"]').getBoundingClientRect();return r.width>=44 && r.height>=44;})`)
				}
				// Position is fixture setup; opening and closing use real keyboard/buttons.
				page.MustEval(`()=>{const ed=edFixture.editor();ed.bus.emit('focus-node',ed.model.doc.nodes[0].id);document.querySelector('.ed-canvas').focus();}`)
				page.Keyboard.MustType(input.Enter)
				s.wait(`()=>{const panel=document.querySelector('.ed-detail');if(!panel)return false;const r=panel.getBoundingClientRect();return panel.getAttribute('role')==='region' && !panel.hasAttribute('aria-modal') && r.width>0 && r.left>=-1 && r.right<=innerWidth+1 && panel.scrollWidth<=panel.clientWidth+1;}`)
				page.MustElement(`.ed-detail [data-ed-narrow="input"]`).MustClick()
				s.wait(`()=>document.querySelector('.ed-detail [data-col="input"]').classList.contains('is-active')`)
				page.MustElement(`.ed-detail [data-ed-narrow="params"]`).MustClick()
				s.shot(fmt.Sprintf("ux-%s-%d", theme, width))
				s.noRawKeys("responsive inspector")
				page.MustElement(`.ed-detail [data-ed-detail-close]`).MustClick()
				s.wait(`()=>!document.querySelector('.ed-detail') && document.querySelector('.ed-canvas').getBoundingClientRect().width>100`)
			})
		}
	}
	page.MustSetViewport(1440, 900, 1, false)
	_ = (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	page.MustEval(`()=>fixtureTheme('standard')`)
	page.MustElement(`[data-ed-cmd="home"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-home .ed-flow-card').length===1`)
	page.MustEval(`()=>{edFixture.state.flows.clear();document.dispatchEvent(new CustomEvent('aurago:flows-changed',{detail:{reason:'deleted'}}));}`)
	s.wait(`()=>!!document.querySelector('[data-ed-home-templates]') && !!document.querySelector('.ed-empty-actions [data-ed-new]') && !!document.querySelector('.ed-empty-actions [data-ed-import]')`)
	page.MustElement(`[data-ed-home-templates]`).MustClick()
	s.wait(`()=>edFixture.state.flows.size===0 && document.activeElement.matches('[data-ed-template]')`)
	s.failOnPageErrors("responsive UX and home recovery")

	// Initial load failure is not an empty list or a stale successful list.
	page.MustNavigate(page.MustInfo().URL).MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;edFixture.state.failLists.push({status:500,code:'FLOW_INTERNAL'});await fixtureOpen('easydrag');}`)
	s.wait(`()=>!!document.querySelector('[data-ed-home-retry]') && !document.querySelector('.ed-flow-card') && !document.querySelector('.ed-home-stale') && !document.querySelector('[data-ed-home-templates]')`)
	page.MustElement(`[data-ed-home-retry]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	s.failOnPageErrors("initial load recovery")
}
