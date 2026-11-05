package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopEasyDragLibraryPauseBrowser exercises the explicit library review and pause flow
// through native browser controls and the status-only API fixture.
func TestDesktopEasyDragLibraryPauseBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-library-pause-fixture.js")
	p := s.page
	t.Cleanup(func() {
		if t.Failed() {
			if value, err := p.Eval(`()=>JSON.stringify({cards:[...document.querySelectorAll('.ed-flow-card')].map(c=>c.dataset.edFlow),requests:edPauseFixture.state.requests,errors:window.fixtureErrors})`); err == nil {
				t.Logf("library pause fixture state: %s", value.Value.Str())
			}
		}
	})
	ids := strings.Split(p.MustEval(`()=>{
		const make=(name,enabled,updated,state)=>{
			const id=edFixture.addEmptyDraft(name),entry=edFixture.flow(id),rec=entry.rec;
			rec.live=JSON.parse(JSON.stringify(rec.draft));rec.live_revision=1;rec.published_draft_revision=1;
			rec.published_at=new Date().toISOString();rec.updated_at=new Date(Date.now()-updated).toISOString();entry.enabled=enabled;
			if(state)edPauseFixture.set(id,state);
			return id;
		};
		const uncertain=make('Very long flow name for the uncertain save result at a narrow screen',true,0,'enabled');
		const next=make('Second active flow',true,10000,'enabled');
		const paused=make('Already paused flow',false,20000,'disabled');
		const mismatch=make('Identity changed flow',true,30000,'mismatch');
		edFixture.addEmptyDraft('Draft without a selection checkbox');
		return [uncertain,next,paused,mismatch].join('|');
	}`).Str(), "|")
	if len(ids) != 4 {
		t.Fatalf("fixture flow IDs = %v", ids)
	}
	uncertain, next, paused, mismatch := ids[0], ids[1], ids[2], ids[3]
	p.MustEval(`()=>aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:'',reason:'published'}})`)
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===6`)
	s.wait(fmt.Sprintf(`()=>[...document.querySelectorAll('.ed-flow-card')].some(c=>c.dataset.edFlow==='%s') && [...document.querySelectorAll('.ed-flow-card')].some(c=>c.dataset.edFlow==='%s')`, uncertain, mismatch))
	p.MustElement(`[data-ed-library-view="list"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-flow-grid').classList.contains('is-list')`)
	p.MustEval(`()=>{const card=[...document.querySelectorAll('.ed-flow-card')].find(c=>!c.classList.contains('is-bulk-selectable'));if(!card||card.querySelector('[data-ed-select-flow]'))throw Error('draft unexpectedly has a selection checkbox');if(getComputedStyle(card.querySelector('.ed-flow-info')).paddingLeft!=='14px')throw Error('draft-only list row retains the selection gutter');}`)
	p.MustElement(`[data-ed-library-view="cards"]`).MustClick()
	p.MustEval(`()=>fixtureTheme('standard')`)
	p.MustSetViewport(1440, 900, 1, false)

	// Select the long active target first with Space, then include an already-paused and a
	// mismatched identity. Both must remain visible in review but can never become candidates.
	pick := func(id string, keyboard bool) {
		t.Helper()
		e := p.MustElement(`[data-ed-select-flow="` + id + `"]`).MustFocus()
		if keyboard {
			p.Keyboard.MustType(input.Space)
		} else {
			e.MustClick()
		}
	}
	pick(uncertain, true)
	pick(next, false)
	pick(paused, false)
	pick(mismatch, false)
	p.MustEval(`()=>{if(document.querySelectorAll('[data-ed-select-flow]:checked').length!==4)throw Error('explicit selection count');}`)
	if got := p.MustEval(`()=>edPauseFixture.state.requests.filter(r=>r.method==='POST').length`).Int(); got != 0 {
		t.Fatalf("selection sent %d pause writes", got)
	}

	openReview := func(count int) {
		t.Helper()
		p.MustElement(`[data-ed-bulk-review]`).MustFocus().MustType(input.Enter)
		s.wait(fmt.Sprintf(`()=>{const d=document.querySelector('.ed-modal--bulk-pause');return !!d && !d.querySelector('.ed-bulk-pause-progress') && d.querySelectorAll('.ed-bulk-pause-row').length===%d && !!d.querySelector('[data-ed-action="confirm"]');}`, count))
	}
	keyboardConfirm := func() {
		t.Helper()
		p.MustElement(`.ed-modal--bulk-pause [data-ed-action="close"]`).MustFocus()
		p.Keyboard.MustType(input.Tab)
		p.MustEval(`()=>{if(document.activeElement!==document.querySelector('.ed-modal--bulk-pause [data-ed-action="confirm"]'))throw Error('Tab did not reach Pause active flows');}`)
		p.Keyboard.MustType(input.Enter)
	}
	openReview(4)
	preview := p.MustEval(`()=>JSON.stringify({statuses:[...document.querySelectorAll('.ed-bulk-pause-row')].map(r=>[r.dataset.state,r.textContent.trim()]),posts:edPauseFixture.state.requests.filter(r=>r.method==='POST').length})`).Str()
	if !strings.Contains(preview, `"enabled"`) || !strings.Contains(preview, `"disabled"`) || !strings.Contains(preview, `"mismatch"`) || !strings.Contains(preview, `"posts":0`) {
		t.Fatalf("mixed read-only preview = %s", preview)
	}
	// This is the supported skin matrix: Standard dark and Fruity light/dark. Narrow layouts also
	// exercise the translated long result label and keep the primary action readable.
	for _, tc := range []struct {
		theme string
		width int
	}{
		{"standard", 320}, {"standard", 390}, {"fruity-light", 320},
		{"fruity-light", 390}, {"fruity-dark", 390}, {"fruity-dark", 1440},
	} {
		p.MustSetViewport(tc.width, 844, 1, tc.width < 760)
		p.MustEval(`theme=>fixtureTheme(theme)`, tc.theme)
		s.wait(`()=>{const d=document.querySelector('.ed-modal--bulk-pause'),r=d&&d.getBoundingClientRect();return !!r && r.left>=-1 && r.right<=innerWidth+1 && d.scrollWidth<=d.clientWidth+1 && [...d.querySelectorAll('.ed-modal-foot [data-ed-action]')].every(b=>{const x=b.getBoundingClientRect();return x.left>=-1&&x.right<=innerWidth+1;});}`)
		p.MustEval(`()=>{const b=document.querySelector('.ed-modal--bulk-pause [data-ed-action="confirm"]'),cs=getComputedStyle(b),rgb=s=>{const c=s.match(/[0-9.]+/g).slice(0,3).map(Number);return s.startsWith('color(srgb ')?c.map(x=>x*255):c;},lin=x=>{x/=255;return x<=.04045?x/12.92:((x+.055)/1.055)**2.4;},lum=s=>{const c=rgb(s).map(lin);return .2126*c[0]+.7152*c[1]+.0722*c[2];},fg=lum(cs.color),stops=cs.backgroundImage.match(/rgba?\([^)]+\)/g)||[cs.backgroundColor],ratios=stops.map(s=>{const bg=lum(s);return (Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05);}),ratio=Math.min(...ratios);if(ratio<4.5)throw Error('Pause confirm contrast below 4.5:1: '+ratio+' foreground='+cs.color+' backgrounds='+JSON.stringify(stops));}`)
		s.shot(fmt.Sprintf("library-pause-preview-%s-%d", tc.theme, tc.width))
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)

	// The preview freezes candidate membership. A paused or mismatched flow becoming active later
	// does not silently enter this reviewed operation. The first POST loses its reply after applying.
	p.MustEval(fmt.Sprintf(`()=>{edPauseFixture.set('%s','enabled');edPauseFixture.set('%s','enabled');edPauseFixture.state.loseNextPost=true;}`, paused, mismatch))
	keyboardConfirm()
	s.wait(`()=>!!document.querySelector('.ed-modal--bulk-pause [data-ed-bulk-state-note]') && !!document.querySelector('.ed-modal--bulk-pause') && document.querySelector('.ed-modal--bulk-pause').textContent.includes(t('easydrag.ui.home_bulk_outcome_save_uncertain'))`)
	p.MustSetViewport(320, 844, 1, true)
	p.MustEval(`()=>fixtureTheme('fruity-light')`)
	p.MustEval(`()=>{const d=document.querySelector('.ed-modal--bulk-pause'),row=[...d.querySelectorAll('.ed-bulk-pause-row')].find(x=>x.textContent.includes(t('easydrag.ui.home_bulk_outcome_save_uncertain')));row.scrollIntoView({block:'center'});}`)
	s.wait(`()=>{const d=document.querySelector('.ed-modal--bulk-pause'),row=[...d.querySelectorAll('.ed-bulk-pause-row')].find(x=>x.textContent.includes(t('easydrag.ui.home_bulk_outcome_save_uncertain'))),state=row&&row.querySelector('.ed-bulk-pause-state'),r=state&&state.getBoundingClientRect(),b=d.querySelector('.ed-modal-body').getBoundingClientRect(),cs=state&&getComputedStyle(state),line=cs&&parseFloat(cs.lineHeight)||16;return !!state && state.textContent.trim()===t('easydrag.ui.home_bulk_outcome_save_uncertain') && state.scrollWidth<=state.clientWidth+1 && state.clientWidth>=row.clientWidth*.85 && cs.textAlign==='left' && r.height>=line-1 && r.top>=b.top-1 && r.bottom<=b.bottom+1 && d.scrollWidth<=d.clientWidth+1;}`)
	s.shot("library-pause-save-uncertain-visible-fruity-light-320")
	p.MustEval(fmt.Sprintf(`id=>{const r=edPauseFixture.state.requests,posts=r.filter(x=>x.method==='POST');if(posts.length!==1||posts[0].id!==id||posts[0].body.enabled!==false)throw Error('uncertain POST retried or broadened');if(r.filter(x=>x.method==='GET'&&x.id===id).length!==3)throw Error('expected preview, precheck and one readback');if(posts.some(x=>x.id==='%s'||x.id==='%s'))throw Error('newly active excluded identity entered frozen candidate set');}`, paused, mismatch), uncertain)
	p.MustElement(`.ed-modal--bulk-pause [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--bulk-pause')`)
	s.wait(fmt.Sprintf(`()=>{const c=document.querySelector('[data-ed-flow="%s"]');return !!c && !c.classList.contains('is-active');}`, uncertain))
	p.MustEval(`id=>document.querySelector('[data-ed-flow="'+id+'"]')?.scrollIntoView({block:'center'})`, uncertain)
	s.shot("library-pause-after-readback-card")

	// A held accepted request keeps Cancel usable. Closing stops the second candidate; releasing the
	// already-sent request allows its one readback and refresh without repainting the closed modal.
	p.MustEval(fmt.Sprintf(`()=>{edPauseFixture.set('%s','enabled');edPauseFixture.set('%s','enabled');edPauseFixture.state.holdNextPost=true;}`, uncertain, next))
	openReview(4)
	p.MustElement(`.ed-modal--bulk-pause [data-ed-action="confirm"]`).MustClick()
	s.wait(`()=>!!edPauseFixture.state.heldPost`)
	p.MustEval(`()=>{const c=document.querySelector('.ed-modal--bulk-pause [data-ed-action="cancel"]');if(!c||c.disabled||document.activeElement!==c)throw Error('Cancel lost focus or was disabled during the held POST');}`)
	beforeClose := p.MustEval(`id=>edPauseFixture.state.requests.filter(r=>r.method==='POST'&&r.id===id).length`, uncertain).Int()
	p.MustElement(`.ed-modal--bulk-pause [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--bulk-pause')`)
	p.MustEval(`()=>edPauseFixture.releaseHeldPost()`)
	s.wait(fmt.Sprintf(`()=>edPauseFixture.state.requests.filter(r=>r.method==='GET'&&r.id==='%s').length>=6`, uncertain))
	if got := p.MustEval(`id=>edPauseFixture.state.requests.filter(r=>r.method==='POST'&&r.id===id).length`, uncertain).Int(); got != beforeClose {
		t.Fatalf("held pause POST count = %d, before close %d", got, beforeClose)
	}
	if got := p.MustEval(`()=>document.activeElement===document.querySelector('[data-ed-home-search]')`).Bool(); !got {
		t.Fatal("closing the held operation did not restore focus to a visible library control")
	}
	s.wait(fmt.Sprintf(`()=>{const c=document.querySelector('[data-ed-flow="%s"]');return c&&!c.classList.contains('is-active');}`, uncertain))

	// A 403/read-only response still gets one status readback and ends the queue before the next
	// active candidate. The result keeps both target rows and never shows server-provided prose.
	p.MustEval(fmt.Sprintf(`()=>{edPauseFixture.set('%s','enabled');edPauseFixture.set('%s','enabled');edPauseFixture.state.denyNextPost=true;}`, uncertain, next))
	openReview(4)
	p.MustElement(`.ed-modal--bulk-pause [data-ed-action="confirm"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--bulk-pause [data-ed-bulk-state-note]') && !!document.querySelector('.ed-modal--bulk-pause') && document.querySelector('.ed-modal--bulk-pause').textContent.includes(t('easydrag.ui.home_bulk_permission_lost'))`)
	permission := p.MustEval(`()=>JSON.stringify({nextPosted:edPauseFixture.state.requests.some(r=>r.method==='POST'&&r.id==='` + next + `'),notAttempted:[...document.querySelectorAll('.ed-bulk-pause-row')].some(r=>r.textContent.includes(t('easydrag.ui.home_bulk_outcome_not_attempted'))),raw:document.querySelector('.ed-modal--bulk-pause').textContent.includes('desktop_readonly')})`).Str()
	if strings.Contains(permission, `"nextPosted":false`) == false || !strings.Contains(permission, `"notAttempted":true`) || !strings.Contains(permission, `"raw":false`) {
		t.Fatalf("read-only queue outcome = %s", permission)
	}
	if count := p.MustEval(`id=>edPauseFixture.state.requests.filter(r=>r.method==='POST'&&r.id===id).length`, next).Int(); count != 0 {
		t.Fatalf("read-only loss sent a later POST for %q (%d)", next, count)
	}
	p.MustElement(`.ed-modal--bulk-pause [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--bulk-pause')`)

	// When an Active-filtered operation pauses every selected flow, the refresh removes their cards
	// and hides the Review button. Closing results must move focus to the visible library search.
	seeded := p.MustEval(`()=>edFixture.seededID`).Str()
	p.MustEval(fmt.Sprintf(`()=>{edPauseFixture.set('%s','disabled');edPauseFixture.set('%s','disabled');edPauseFixture.set('%s','disabled');edPauseFixture.set('%s','mismatch');edFixture.flow('%s').enabled=false;aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:'',reason:'enabled'}});}`, seeded, uncertain, paused, mismatch, mismatch))
	s.wait(fmt.Sprintf(`()=>document.querySelectorAll('.ed-flow-card.is-active').length===1 && !!document.querySelector('.ed-flow-card.is-active[data-ed-flow="%s"]')`, next))
	p.MustElement(`[data-ed-filter="active"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && !!document.querySelector('[data-ed-flow="` + next + `"]')`)
	pick(next, true)
	openReview(1)
	p.MustEval(`()=>{const c=document.querySelector('.ed-modal--bulk-pause [data-ed-action="confirm"]');if(!c)throw Error('missing explicit confirm');c.focus();}`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>!!document.querySelector('.ed-modal--bulk-pause') && document.querySelector('.ed-modal--bulk-pause').textContent.includes(t('easydrag.ui.home_bulk_outcome_confirmed')) && document.querySelectorAll('.ed-modal--bulk-pause .ed-bulk-pause-row').length===1`)
	p.MustSetViewport(390, 844, 1, true)
	p.MustEval(`()=>fixtureTheme('fruity-dark')`)
	s.shot("library-pause-confirmed-active-390")
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===0 && document.querySelector('[data-ed-bulk-selection]').hidden`)
	p.MustElement(`.ed-modal--bulk-pause [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--bulk-pause')`)
	s.wait(`()=>document.activeElement===document.querySelector('[data-ed-home-search]')`)
	s.noRawKeys("library pause")
	s.failOnPageErrors("library pause")
}
