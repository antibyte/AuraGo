package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopEasyDragPackBrowser covers local-pack review, explicit import, uncertain status,
// attachment export and the unchanged plain-flow path in the real Desktop shell.
func TestDesktopEasyDragPackBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-pack-fixture.js")
	p := s.page
	p.MustEval(`()=>fixtureTheme('standard')`)
	p.MustEval(`()=>{window.packSource=edPackFixture.makePack({hostile:true,long:true,reordered:true});window.packSourceID=edPackFixture.seed(window.packSource);aurora.handleDesktopEvent({type:'flows_changed',payload:{flow_id:window.packSourceID,reason:'created'}});}`)
	s.wait(`()=>!!document.querySelector('[data-ed-flow="flow_packsource"]')`)
	s.noRawKeys("pack source library")

	setFile := func(selector, name, contents string) {
		t.Helper()
		p.MustEval(`arg=>{const transfer=new DataTransfer();transfer.items.add(new File([arg.contents],arg.name,{type:'application/json'}));const input=document.querySelector(arg.selector);if(!input)throw Error('missing file input '+arg.selector);input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));}`,
			map[string]string{"selector": selector, "name": name, "contents": contents})
	}
	dropFile := func(name, contents string) {
		t.Helper()
		p.MustEval(`arg=>{const transfer=new DataTransfer();transfer.items.add(new File([arg.contents],arg.name,{type:'application/json'}));const event=new Event('drop',{bubbles:true,cancelable:true});Object.defineProperty(event,'dataTransfer',{value:transfer});document.querySelector('.ed-home').dispatchEvent(event);}`,
			map[string]string{"name": name, "contents": contents})
	}
	choosePack := func(options string) {
		t.Helper()
		raw := p.MustEval(`raw=>JSON.stringify(edPackFixture.makePack(JSON.parse(raw)))`, options).Str()
		setFile(".ed-home [data-ed-pack-import-file]", "portable.easydrag-pack.json", raw)
	}
	clickContextLabel := func(labelKey string) {
		t.Helper()
		p.MustEval(`key=>{const label=t('easydrag.ui.'+key);const button=[...document.querySelectorAll('.vd-context-menu [role="menuitem"]')].find(item=>item.textContent.includes(label));if(!button)throw Error('context action not found: '+label);button.click();}`, labelKey)
	}
	clickFlowCard := func(id string) {
		t.Helper()
		p.MustEval(`id=>{const card=document.querySelector('[data-ed-flow="'+id+'"]');if(!card)throw Error('flow card missing '+id);card.click();}`, id)
	}
	goHome := func() {
		t.Helper()
		p.MustElement(`.ed-editor [data-ed-cmd="home"]`).MustClick()
		s.wait(`()=>!!document.querySelector('.ed-home')&&!edFixture.editor()`)
	}
	openPreview := func() {
		t.Helper()
		s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="preview"]')`)
		p.MustElement(`.ed-modal--pack [data-ed-action="preview"]`).MustClick()
		s.wait(`()=>{const d=document.querySelector('.ed-modal--pack');const b=d&&d.querySelector('[data-ed-action="import"]');return !!d&&!!d.querySelector('.ed-pack-doc-list')&&!!b&&!b.hidden&&!b.disabled;}`)
	}
	assertImportCounts := func(want int) {
		t.Helper()
		if got := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='import').length`).Int(); got != want {
			t.Fatalf("local-pack Import count = %d, want %d", got, want)
		}
	}

	// A newer invalid file selection supersedes a still-reading older pack file.
	previewBeforeSelection := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='preview').length`).Int()
	packRaw := p.MustEval(`()=>JSON.stringify(edPackFixture.makePack({empty:true}))`).Str()
	p.MustEval(`raw=>{const input=document.querySelector('.ed-home [data-ed-pack-import-file]'),file=new File(['pending'],'slow.easydrag-pack.json',{type:'application/json'}),transfer=new DataTransfer();Object.defineProperty(file,'text',{value:()=>new Promise(resolve=>window.releaseSlowPack=()=>resolve(raw))});transfer.items.add(file);input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));}`,
		packRaw)
	setFile(".ed-home [data-ed-pack-import-file]", "newer.easydrag-pack.json", "{")
	s.wait(`()=>[...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(t('easydrag.ui.pack_invalid')))`)
	p.MustEval(`async()=>{window.releaseSlowPack();await new Promise(resolve=>setTimeout(resolve,0));await new Promise(resolve=>requestAnimationFrame(resolve));}`)
	s.wait(`()=>!document.querySelector('.ed-modal--pack')&&edPackFixture.state.requests.filter(r=>r.kind==='preview').length===` + fmt.Sprint(previewBeforeSelection))
	assertImportCounts(0)

	// A generic filename with the LocalPack marker is recognized on drop and remains read-only.
	collectionsBeforeDrop := p.MustEval(`()=>edFixture.state.requests.filter(r=>r==='POST /api/desktop/flows').length`).Int()
	previewBeforeDrop := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='preview').length`).Int()
	dropRaw := p.MustEval(`()=>JSON.stringify(edPackFixture.makePack({empty:true}))`).Str()
	dropFile("portable.json", dropRaw)
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="preview"]')`)
	p.MustElement(`.ed-modal--pack [data-ed-action="preview"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack .ed-pack-doc-list')`)
	if got := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='preview').length`).Int(); got != previewBeforeDrop+1 {
		t.Fatalf("generic JSON drop did not use LocalPack preview: count %d -> %d", previewBeforeDrop, got)
	}
	assertImportCounts(0)
	if got := p.MustEval(`()=>edFixture.state.requests.filter(r=>r==='POST /api/desktop/flows').length`).Int(); got != collectionsBeforeDrop {
		t.Fatalf("generic JSON pack drop fell through to legacy create: POST count %d -> %d", collectionsBeforeDrop, got)
	}
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--pack')`)

	requestsBeforeReview := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='preview'||r.kind==='import').length`).Int()
	choosePack(`{"hostile":true,"long":true,"reordered":true}`)
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="preview"]')`)
	if got := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='preview'||r.kind==='import').length`).Int(); got != requestsBeforeReview {
		t.Fatalf("opening pack review sent %d API requests before explicit preview", got-requestsBeforeReview)
	}
	openPreview()
	if got := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='import').length`).Int(); got != 0 {
		t.Fatalf("read-only preview sent %d Import requests", got)
	}
	if got := p.MustEval(`()=>JSON.stringify({parent:document.querySelector('.ed-pack-parent')?.textContent,rows:[...document.querySelectorAll('.ed-pack-doc-list li')].map(x=>x.textContent),script:!!document.querySelector('.ed-modal--pack script'),truncated:document.querySelector('.ed-pack-document')?.textContent.includes(t('easydrag.ui.output_truncated')),injected:!!window.packInjected})`).Str(); !strings.Contains(got, `<script>window.packInjected=true</script>`) || !strings.Contains(got, `$& $'`) || !strings.Contains(got, `"rows":[`) || strings.Contains(got, `"script":true`) || strings.Contains(got, `"injected":true`) || !strings.Contains(got, `"truncated":true`) {
		t.Fatalf("hostile or bounded preview rendering = %s", got)
	}
	s.noRawKeys("local-pack preview")

	for _, tc := range []struct {
		theme string
		width int
	}{
		{"standard", 320}, {"standard", 390}, {"fruity-light", 320},
		{"fruity-light", 390}, {"fruity-dark", 390}, {"fruity-dark", 1440},
	} {
		p.MustSetViewport(tc.width, 844, 1, tc.width < 760)
		p.MustEval(`theme=>fixtureTheme(theme)`, tc.theme)
		p.MustEval(`()=>document.querySelector('.ed-modal--pack .ed-modal-body').scrollTop=0`)
		s.wait(`()=>{const d=document.querySelector('.ed-modal--pack'),r=d&&d.getBoundingClientRect(),b=d&&d.querySelector('[data-ed-action="import"]'),f=d&&d.querySelector('.ed-modal-foot');return !!r&&r.left>=-1&&r.right<=innerWidth+1&&d.scrollWidth<=d.clientWidth+1&&!!b&&!b.hidden&&!!f&&[...f.querySelectorAll('[data-ed-action]:not([hidden])')].every(x=>{const q=x.getBoundingClientRect();return q.left>=-1&&q.right<=innerWidth+1;});}`)
		p.MustEval(`()=>{const b=document.querySelector('.ed-modal--pack [data-ed-action="import"]'),cs=getComputedStyle(b),rgb=s=>{const c=s.match(/[0-9.]+/g).slice(0,3).map(Number);return s.startsWith('color(srgb ')?c.map(x=>x*255):c;},lin=x=>{x/=255;return x<=.04045?x/12.92:((x+.055)/1.055)**2.4;},lum=s=>{const c=rgb(s).map(lin);return .2126*c[0]+.7152*c[1]+.0722*c[2];},fg=lum(cs.color),stops=cs.backgroundImage.match(/rgba?\([^)]+\)/g)||[cs.backgroundColor],ratio=Math.min(...stops.map(s=>{const bg=lum(s);return (Math.max(fg,bg)+.05)/(Math.min(fg,bg)+.05);}));if(ratio<4.5)throw Error('Import contrast below 4.5:1: '+ratio+' foreground='+cs.color+' backgrounds='+JSON.stringify(stops));}`)
		s.shot(fmt.Sprintf("easydrag-pack-preview-top-%s-%d", tc.theme, tc.width))
		if tc.width == 320 && tc.theme == "standard" {
			p.MustEval(`()=>{const body=document.querySelector('.ed-modal--pack .ed-modal-body'),content=document.querySelector('.ed-pack-content');body.scrollTop=content.offsetTop;}`)
			s.shot("easydrag-pack-preview-content-standard-320")
		}
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)

	// A fresh ready modal proves the complete keyboard path: Tab to preview, Enter to preview,
	// then Tab to the now-visible Import action and Enter exactly once.
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--pack')`)
	choosePack(`{"hostile":true,"long":true,"reordered":true}`)
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="preview"]')&&!document.querySelector('.ed-modal--pack [data-ed-action="preview"]').hidden`)
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustFocus()
	p.Keyboard.MustType(input.Tab)
	p.MustEval(`()=>{const b=document.querySelector('.ed-modal--pack [data-ed-action="preview"]');if(document.activeElement!==b)throw Error('Tab did not focus Review pack');}`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="import"]')&&!document.querySelector('.ed-modal--pack [data-ed-action="import"]').hidden`)
	p.Keyboard.MustType(input.Tab)
	p.MustEval(`()=>{const b=document.querySelector('.ed-modal--pack [data-ed-action="import"]');if(document.activeElement!==b)throw Error('Tab did not focus Import after preview');}`)
	p.MustSetViewport(320, 844, 1, true)
	p.MustEval(`()=>fixtureTheme('fruity-light')`)
	p.MustEval(`()=>document.querySelector('.ed-modal--pack .ed-modal-body').scrollTop=0`)
	if got := p.MustEval(`()=>document.querySelector('.ed-pack-document').textContent.includes(t('easydrag.ui.output_truncated'))`).Bool(); !got {
		t.Fatal("bounded preview does not disclose truncation at 320px")
	}
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>!!document.querySelector('.ed-modal--pack .ed-pack-result')`)
	assertImportCounts(1)
	if got := p.MustEval(`()=>{const req=edPackFixture.state.requests.filter(r=>r.kind==='import').at(-1),id=req.body.proposal.parent_id,e=edFixture.flow(id);return !!e&&!e.enabled&&!e.rec.live&&e.rec.live_revision===0&&e.rec.draft_revision===1&&edBlockFixture.state.blocks.size>=2}`).Bool(); !got {
		t.Fatal("acknowledged import did not create an inactive draft with local blocks")
	}
	p.MustEval(`()=>document.querySelector('.ed-modal--pack .ed-modal-body').scrollTop=0`)
	s.shot("easydrag-pack-result-first-320-fruity-light")
	p.MustElement(`.ed-modal--pack [data-ed-action="open"]`).MustClick()
	s.wait(`()=>!!edFixture.editor()&&edFixture.editor().flow.id===edPackFixture.state.requests.filter(r=>r.kind==='import')[0].body.proposal.parent_id`)
	goHome()

	// A lost reply is never retried. Reopening through the banner performs only read-only reconcile;
	// the proposal-only operation renders from compact summaries and can open the confirmed draft.
	p.MustEval(`()=>edPackFixture.state.modes.import='lost'`)
	choosePack(`{"empty":true}`)
	openPreview()
	p.MustElement(`.ed-modal--pack [data-ed-action="import"]`).MustClick()
	s.wait(`()=>{const b=document.querySelector('.ed-modal--pack [data-ed-action="reconcile"]');return !!b&&!b.hidden;}`)
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-pack-reconcile]')`)
	assertImportCounts(2)
	if got := p.MustEval(`()=>edPackFixture.state.requests.filter(r=>r.kind==='reconcile').length`).Int(); got != 0 {
		t.Fatalf("closing uncertain import sent %d reconciliation requests", got)
	}
	p.MustElement(`[data-ed-pack-reconcile]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="reconcile"]')`)
	p.MustElement(`.ed-modal--pack [data-ed-action="reconcile"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack .ed-pack-result')&&!!document.querySelector('.ed-modal--pack [data-ed-action="open"]')&&!document.querySelector('.ed-modal--pack [data-ed-action="open"]').hidden`)
	assertImportCounts(2)
	p.MustElement(`.ed-modal--pack [data-ed-action="open"]`).MustClick()
	s.wait(`()=>!!edFixture.editor()&&edFixture.editor().flow.id===edPackFixture.state.requests.filter(r=>r.kind==='import')[1].body.proposal.parent_id`)
	goHome()

	// Unknown reconciliation remains pending until the second, explicit finish confirmation.
	p.MustEval(`()=>{edPackFixture.state.modes.import='lost';edPackFixture.state.modes.reconcile='unknown';}`)
	choosePack(`{"empty":true}`)
	openPreview()
	p.MustElement(`.ed-modal--pack [data-ed-action="import"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="reconcile"]')`)
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustClick()
	p.MustElement(`[data-ed-pack-reconcile]`).MustClick()
	p.MustElement(`.ed-modal--pack [data-ed-action="reconcile"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="finish"]')&&!document.querySelector('.ed-modal--pack [data-ed-action="finish"]').hidden`)
	p.MustElement(`.ed-modal--pack [data-ed-action="finish"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-modal--pack .ed-pack-warning')?.textContent.includes(t('easydrag.ui.pack_finish_status_text'))===true`)
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--pack')&&!!document.querySelector('[data-ed-pack-reconcile]')`)
	assertImportCounts(3)
	// Reopen and explicitly repeat the read-only check before accepting the local finish action.
	p.MustElement(`[data-ed-pack-reconcile]`).MustClick()
	p.MustElement(`.ed-modal--pack [data-ed-action="reconcile"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack [data-ed-action="finish"]')&&!document.querySelector('.ed-modal--pack [data-ed-action="finish"]').hidden`)
	p.MustElement(`.ed-modal--pack [data-ed-action="finish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack .ed-pack-warning')`)
	p.MustElement(`.ed-modal--pack [data-ed-action="finish"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--pack')&&!document.querySelector('[data-ed-pack-reconcile]')`)
	assertImportCounts(3)

	// Closing an accepted, held Import fences late UI effects. The server fixture still commits it;
	// the window retains a proposal for read-only reconciliation and never paints the late result.
	p.MustEval(`()=>{edPackFixture.state.modes.import='ok';edPackFixture.state.modes.reconcile='confirmed';edPackFixture.hold('import');}`)
	choosePack(`{"empty":true}`)
	openPreview()
	p.MustElement(`.ed-modal--pack [data-ed-action="import"]`).MustClick()
	s.wait(`()=>edPackFixture.state.requests.filter(r=>r.kind==='import').length===4`)
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--pack')&&!!document.querySelector('[data-ed-pack-reconcile]')`)
	p.MustEval(`()=>edPackFixture.release('import')`)
	s.wait(`()=>!document.querySelector('.ed-modal--pack')&&!!document.querySelector('[data-ed-pack-reconcile]')`)
	assertImportCounts(4)
	p.MustEval(`()=>{const id=edPackFixture.state.requests.filter(r=>r.kind==='import').at(-1).body.proposal.parent_id;if(!edFixture.flow(id))throw Error('late import did not commit in fixture');}`)
	p.MustElement(`[data-ed-pack-reconcile]`).MustClick()
	p.MustElement(`.ed-modal--pack [data-ed-action="reconcile"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal--pack .ed-pack-result')`)
	assertImportCounts(4)
	p.MustElement(`.ed-modal--pack [data-ed-action="cancel"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--pack')&&!document.querySelector('[data-ed-pack-reconcile]')`)

	// Library export returns one bare pack download. Editor export first saves a pending name edit,
	// checks the acknowledged source revision and model version, then downloads that same response.
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	p.MustEval(`id=>{const card=document.querySelector('[data-ed-flow="'+id+'"]');card.querySelector('[data-ed-card-menu]').click();}`, "flow_packsource")
	clickContextLabel("pack_export")
	s.wait(`()=>edPackFixture.state.downloads.length===1&&edPackFixture.state.downloads[0].text.length>0`)
	p.MustEval(`()=>{const d=edPackFixture.state.downloads[0],pack=JSON.parse(d.text);if(!d.name.endsWith('.easydrag-pack.json')||pack.format!=='aurago.easydrag.local-pack'||pack.parent.flow.id!=='flow_packsource')throw Error('library export is not a bare source pack');}`)
	clickFlowCard("flow_packsource")
	s.wait(`()=>!!edFixture.editor()&&edFixture.editor().flow.id==='flow_packsource'`)
	p.MustEval(`()=>{const n=document.querySelector('[data-ed-name]');n.value='Saved before local-pack export';n.dispatchEvent(new Event('change',{bubbles:true}));}`)
	p.MustElement(`.ed-editor [data-ed-cmd="more"]`).MustClick()
	clickContextLabel("pack_export")
	s.wait(`()=>edPackFixture.state.downloads.length===2&&edPackFixture.state.downloads[1].text.length>0`)
	p.MustEval(`()=>{const text=edPackFixture.state.downloads[1].text,pack=JSON.parse(text),entry=edFixture.flow('flow_packsource');if(pack.parent.source_revision!==entry.rec.draft_revision||pack.parent.flow.name!=='Saved before local-pack export')throw Error('editor export did not use the saved model/revision');if(edFixture.state.saves.at(-1).name!=='Saved before local-pack export')throw Error('pending editor name was not flushed before export');}`)
	p.MustEval(`()=>edPackFixture.state.exportRevisionOffset=1`)
	p.MustElement(`.ed-editor [data-ed-cmd="more"]`).MustClick()
	clickContextLabel("pack_export")
	s.wait(`()=>!!document.querySelector('.vd-toast')`)
	if got := p.MustEval(`()=>edPackFixture.state.downloads.length`).Int(); got != 2 {
		t.Fatalf("stale editor snapshot produced %d downloads, want 2", got)
	}
	goHome()

	// A recognized unsupported pack never falls through to the old flow-create route. Ordinary
	// v1 JSON still uses that route and its existing download action.
	collectionsBefore := p.MustEval(`()=>edFixture.state.requests.filter(r=>r==='POST /api/desktop/flows').length`).Int()
	unsupported := p.MustEval(`()=>{const pack=edPackFixture.makePack({empty:true});pack.version=2;return JSON.stringify(pack);}`).Str()
	setFile(".ed-home [data-ed-pack-import-file]", "future.easydrag-pack.json", unsupported)
	s.wait(`()=>[...document.querySelectorAll('.vd-toast')].some(x=>x.textContent.includes(t('easydrag.ui.error_flow_pack_unsupported')))`)
	if got := p.MustEval(`()=>edFixture.state.requests.filter(r=>r==='POST /api/desktop/flows').length`).Int(); got != collectionsBefore {
		t.Fatalf("unsupported local pack fell through to legacy flow creation: POST count %d -> %d", collectionsBefore, got)
	}
	legacy := `{"schema":1,"kind":"flow","name":"Legacy v1 import","settings":{},"nodes":[],"edges":[]}`
	setFile(".ed-home [data-ed-import-file]", "legacy.easydrag.json", legacy)
	s.wait(`()=>!!edFixture.editor()&&edFixture.editor().model.doc.name==='Legacy v1 import'`)
	goHome()
	s.wait(`()=>[...document.querySelectorAll('.ed-flow-card')].some(c=>c.textContent.includes('Legacy v1 import'))`)
	legacyID := p.MustEval(`()=>[...document.querySelectorAll('.ed-flow-card')].find(c=>c.textContent.includes('Legacy v1 import')).dataset.edFlow`).Str()
	p.MustEval(`id=>{const card=document.querySelector('[data-ed-flow="'+id+'"]');card.querySelector('[data-ed-card-menu]').click();}`, legacyID)
	p.MustEval(`()=>{window.__legacyDownloads=[];window.__anchorClick=HTMLAnchorElement.prototype.click;HTMLAnchorElement.prototype.click=function(){if(this.href.includes('/api/desktop/flows/')&&this.href.endsWith('/export')){window.__legacyDownloads.push({href:this.href,name:this.download});return;}return window.__anchorClick.call(this);};}`)
	clickContextLabel("home_export")
	s.wait(`()=>window.__legacyDownloads.length===1`)
	p.MustEval(`async()=>{const item=window.__legacyDownloads[0];if(!item.name.endsWith('.easydrag.json'))throw Error('legacy export filename changed');const response=await fetch(item.href);if(!response.ok)throw Error('legacy export endpoint failed');window.__legacyExport=await response.json();HTMLAnchorElement.prototype.click=window.__anchorClick;}`)
	s.wait(`()=>edFixture.state.exports.some(doc=>doc.name==='Legacy v1 import')`)
	s.noRawKeys("local-pack journeys")
	s.failOnPageErrors("local-pack journeys")
}
