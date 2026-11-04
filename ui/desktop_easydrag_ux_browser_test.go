package ui

import (
	"fmt"
	"strings"
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
		for _, width := range []int{320, 360, 390, 768, 1024, 1440} {
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
				s.headerNames()
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
	// Fix batch 5: on a phone the ⋯ menu holds Runs and Test and Paused stays in the header; a deleted
	// step offers Undo in a toast; the header's Undo takes back an edit.
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustSetViewport(390, 844, 1, true)
	s.wait(`()=>document.querySelector('.ed-app').getBoundingClientRect().width<=560 && getComputedStyle(document.querySelector('.ed-head-actions [data-ed-cmd="test"]')).display!=='none'`)
	page.MustElement(`[data-ed-cmd="more"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();const items=[...document.querySelectorAll('.vd-context-menu .vd-context-item')].map(b=>b.textContent);return items.some(x=>x.includes(ed.t('easydrag.ui.runs_title'))) && items.some(x=>x.includes(ed.t('easydrag.ui.test_draft')))}`)
	page.MustEval(`()=>aurora.closeContextMenu()`)
	page.MustEval(`()=>{edFixture.flow().enabled=false;aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:edFixture.seededID,reason:'enabled'}})}`)
	s.wait(`()=>{const chip=document.querySelector('[data-ed-state] [data-ed-paused]');return !!chip && chip.getBoundingClientRect().width>0}`)
	s.shot("ux-batch5-phone-header")
	page.MustEval(`()=>{edFixture.flow().enabled=true;aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:edFixture.seededID,reason:'enabled'}})}`)
	s.wait(`()=>!document.querySelector('[data-ed-state] [data-ed-paused]')`)
	page.MustSetViewport(1440, 900, 1, false)
	_ = (proto.EmulationSetTouchEmulationEnabled{Enabled: false}).Call(page)
	page.MustEval(`()=>{const ed=edFixture.editor();const id=ed.model.doc.nodes.find(n=>!ed.model.info(n.type).trigger).id;window.__b5Count=ed.model.doc.nodes.length;ed.bus.emit('focus-node',id);document.querySelector('.ed-node[data-node-id="'+id+'"] [data-ed-node-tool="delete"]').click();}`)
	s.wait(`()=>{const toast=document.querySelector('[data-ed-toast]');return !toast.hidden && toast.getBoundingClientRect().width>0 && edFixture.editor().model.doc.nodes.length===window.__b5Count-1}`)
	s.shot("ux-batch5-toast")
	page.MustElement(`[data-ed-toast-undo]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-toast]').hidden && edFixture.editor().model.doc.nodes.length===window.__b5Count`)
	page.MustEval(`()=>{const ed=edFixture.editor();window.__b5Label=ed.model.doc.nodes[0].label;ed.model.setLabel(ed.model.doc.nodes[0].id,'Batch 5')}`)
	page.MustElement(`.ed-head [data-ed-history="undo"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes[0].label===window.__b5Label`)
	s.failOnPageErrors("fix batch 5 header, toast and undo")
	page.MustElement(`[data-ed-cmd="home"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-home .ed-flow-card').length===1`)
	s.shot("ux-batch5-home-sort")
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

	page.MustElement(`[data-ed-library-view="list"]`).MustClick()
	page.MustElement(`[data-ed-favorite]`).MustClick()
	page.MustElement(`[data-ed-library-favorites]`).MustClick()
	page.MustElement(`.ed-library-saved > summary`).MustClick()
	page.MustElement(`[data-ed-library-save]`).MustClick()
	s.wait(`()=>!!document.querySelector('.vd-modal-input')`)
	page.MustElement(`.vd-modal-input`).MustInput("Favoriten")
	page.Keyboard.MustType(input.Enter)
	s.wait(`()=>!document.querySelector('.vd-modal-input') && !!document.querySelector('[data-ed-library-filter]').value`)
	page.MustElement(`[data-ed-filter="errors"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-flow-card')`)
	page.MustEval(`()=>{const s=document.querySelector('[data-ed-library-filter]');s.value=JSON.parse(localStorage.getItem('aurago.easydrag.library')).filters[0].id;s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && document.querySelector('[data-ed-library-favorites]').getAttribute('aria-pressed')==='true'`)
	page.MustElement(`.ed-library-saved > summary`).MustClick()
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		for _, width := range []int{390, 1440} {
			page.MustSetViewport(width, 900, 1, width < 760)
			page.MustEval(`theme=>fixtureTheme(theme)`, theme)
			s.wait(`()=>{const home=document.querySelector('.ed-home'),card=document.querySelector('.ed-flow-card'),r=card.getBoundingClientRect();return home.scrollWidth<=home.clientWidth+1 && document.querySelector('.ed-flow-grid').classList.contains('is-list') && r.left>=-1 && r.right<=innerWidth+1 && getComputedStyle(card.querySelector('.ed-flow-preview')).display==='none';}`)
			s.wait(`()=>[...document.querySelectorAll('.ed-home-actions button')].every(button=>{const r=button.getBoundingClientRect(),hero=button.closest('.ed-home-hero').getBoundingClientRect();return r.width>=44 && r.height>=30 && r.left>=hero.left && r.right<=hero.right && button.scrollWidth<=button.clientWidth+1;})`)
			s.shot(fmt.Sprintf("ux-library-%s-%d", theme, width))
		}
	}
	page.MustSetViewport(1440, 900, 1, false)
	page.MustEval(`()=>fixtureTheme('standard')`)
	s.noRawKeys("device-local library preferences")
	s.failOnPageErrors("device-local library preferences")

	// Goal-first authoring creates nothing until Apply, then the same phone Test path saves
	// and replays a private scenario with an exact expected-result check.
	page.MustElement(`[data-ed-authoring]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-authoring-goal]')`)
	page.MustElement(`[data-ed-authoring-goal]`).MustInput("Create a local greeting")
	page.MustElement(`[data-ed-action="generate"]`).MustClick()
	s.wait(`()=>{const b=document.querySelector('[data-ed-action="apply"]');return b && !b.hidden && edFixture.state.flows.size===1;}`)
	s.shot("ux-authoring-review")
	page.MustElement(`[data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal') && edFixture.state.flows.size===1`)
	page.MustElement(`[data-ed-authoring]`).MustClick()
	page.MustElement(`[data-ed-authoring-goal]`).MustInput("Create a local greeting")
	page.MustElement(`[data-ed-action="generate"]`).MustClick()
	s.wait(`()=>!document.querySelector('[data-ed-action="apply"]').hidden`)
	page.MustElement(`[data-ed-action="apply"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-editor') && edFixture.state.flows.size===2 && edFixture.editor().model.doc.nodes.length===3`)
	page.MustSetViewport(360, 844, 1, true)
	s.wait(`()=>{const r=document.querySelector('[data-ed-cmd="test"]').getBoundingClientRect();return r.width>=44 && r.height>=44 && r.right<=innerWidth;}`)
	s.shot("ux-first-result-header-360")
	page.MustElement(`[data-ed-cmd="test"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-test-mode]')`)
	page.MustEval(`()=>{const s=document.querySelector('[data-ed-test-mode]');s.value='simulation';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>!!document.querySelector('[data-ed-scenario-name]') && !document.querySelector('.ed-simulation-fields').hidden`)
	page.MustElement(`[data-ed-assertion-add]`).MustClick()
	page.MustEval(`()=>{document.querySelector('[data-ed-assertion-node]').value='n_greeting';}`)
	page.MustElement(`[data-ed-assertion-output]`).MustInput(`{"greeting":"Hallo Welt"}`)
	page.MustElement(`[data-ed-scenario-name]`).MustInput("Lokaler Gruß")
	page.MustElement(`[data-ed-scenario-save-button]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-test-data]').readOnly && !!document.querySelector('[data-ed-scenario]').value`)
	s.shot("ux-private-scenario-360")
	page.MustElement(`[data-ed-action="run"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal') && edFixture.editor().run.mode==='simulation' && edFixture.editor().run.record.assertions[0].passed`)
	s.wait(`()=>document.querySelector('.ed-run-results').open && document.querySelector('.ed-result-preview').textContent.includes('Hallo Welt')`)
	s.shot("ux-scenario-result-360")
	page.MustSetViewport(390, 844, 1, true)
	s.shot("ux-scenario-result-390")
	s.noRawKeys("goal-first authoring and private scenarios")
	s.failOnPageErrors("goal-first authoring and private scenarios")
}

// axName returns the accessible name Chrome computes for the first element that matches selector.
func (s *easyDragSmoke) axName(selector string) string {
	s.t.Helper()
	el := s.page.MustElement(selector)
	node, err := el.Describe(0, false)
	if err != nil {
		s.t.Fatal(err)
	}
	tree, err := proto.AccessibilityGetPartialAXTree{ObjectID: el.Object.ObjectID, FetchRelatives: false}.Call(s.page)
	if err != nil {
		s.t.Fatal(err)
	}
	for _, n := range tree.Nodes {
		if n.BackendDOMNodeID == node.BackendNodeID && n.Name != nil {
			return strings.TrimSpace(n.Name.Value.Str())
		}
	}
	return ""
}

// headerNames checks that Stop and the Active switch have a name at this width, where the header
// may hide their text (below 900 px). Stop only shows during a run: it is shown for the check and
// hidden again.
func (s *easyDragSmoke) headerNames() {
	s.t.Helper()
	s.page.MustEval(`()=>{document.querySelector('[data-ed-cmd="cancel"]').hidden=false}`)
	for _, cmd := range []string{"cancel", "active", "more"} {
		if name := s.axName(`[data-ed-cmd="` + cmd + `"]`); name == "" {
			s.t.Errorf("header button %s has no accessible name", cmd)
		}
	}
	s.page.MustEval(`()=>{document.querySelector('[data-ed-cmd="cancel"]').hidden=true}`)
}
