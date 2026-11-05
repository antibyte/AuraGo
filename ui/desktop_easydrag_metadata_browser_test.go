package ui

import (
	"fmt"
	"testing"
)

// TestDesktopEasyDragMetadataBrowser uses the real controls and an isolated metadata service.
func TestDesktopEasyDragMetadataBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-metadata-fixture.js")
	p := s.page
	t.Cleanup(func() {
		if t.Failed() {
			if value, err := p.Eval(`()=>JSON.stringify({requests:edMetadataFixture.state.requests,errors:window.fixtureErrors})`); err == nil {
				t.Logf("metadata fixture state: %s", value.Value.Str())
			}
		}
	})
	more := func(key string) {
		t.Helper()
		p.MustElement(`[data-ed-cmd="more"]`).MustClick()
		p.MustEval(`key=>{const ed=edFixture.editor(), label=ed.t(key);const item=[...document.querySelectorAll('.vd-context-menu button')].find(b=>b.textContent.includes(label));if(!item)throw Error('Missing menu action '+key);item.click();}`, key)
	}
	closeModal := func() {
		t.Helper()
		p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="close"]`).MustClick()
	}
	selectValue := func(selector, value string) {
		t.Helper()
		p.MustEval(`(selector,value)=>{const e=document.querySelector(selector);e.value=value;e.dispatchEvent(new Event('change',{bubbles:true}));}`, selector, value)
	}
	capture := func(name string) {
		t.Helper()
		for _, theme := range []string{"standard", "fruity-light"} {
			for _, width := range []int{390, 1440} {
				p.MustSetViewport(width, 900, 1, width < 760)
				p.MustEval(`theme=>fixtureTheme(theme)`, theme)
				s.wait(`()=>{const dialogs=[...document.querySelectorAll('.ed-modal')],d=dialogs.at(-1),r=d.getBoundingClientRect();return r.left>=-1 && r.right<=innerWidth+1 && d.scrollWidth<=d.clientWidth+1;}`)
				s.shot(fmt.Sprintf("metadata-%s-%s-%d", name, theme, width))
			}
		}
		p.MustSetViewport(1440, 900, 1, false)
		p.MustEval(`()=>fixtureTheme('standard')`)
	}

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1 && !!document.querySelector('[data-ed-folders-manage]')`)
	p.MustElement(`.ed-library-organization > summary`).MustClick()
	p.MustElement(`[data-ed-folders-manage]`).MustClick()
	p.MustElement(`[data-ed-folder-new]`).MustClick()
	p.MustElement(`[data-ed-folder-name]`).MustInput("Archive")
	p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="save"]`).MustClick()
	s.wait(`()=>edMetadataFixture.state.folders.size===1 && !!document.querySelector('[data-ed-folder-rename]') && !document.querySelector('[data-ed-folder-name]')`)
	folder := p.MustEval(`()=>[...edMetadataFixture.state.folders.keys()][0]`).Str()
	closeModal()
	s.wait(`()=>!document.querySelector('.ed-modal')`)

	// Organize through the card menu; the server receives a partial metadata update.
	p.MustElement(`[data-ed-card-menu]`).MustClick()
	p.MustEval(`()=>{const b=[...document.querySelectorAll('.vd-context-menu button')].find(e=>e.textContent.includes(t('easydrag.ui.organize')));if(!b)throw Error('Missing organize');b.click();}`)
	s.wait(`()=>document.querySelector('[data-ed-organization-folder]')?.options.length>1`)
	selectValue(`[data-ed-organization-folder]`, folder)
	p.MustElement(`[data-ed-tag-input]`).MustInput("local")
	p.MustElement(`[data-ed-tag-add]`).MustClick()
	for _, theme := range []string{"standard", "fruity-light"} {
		for _, width := range []int{390, 1440} {
			p.MustSetViewport(width, 900, 1, width < 760)
			p.MustEval(`theme=>fixtureTheme(theme)`, theme)
			s.wait(`()=>{const d=document.querySelector('.ed-modal--metadata'),r=d.getBoundingClientRect();return r.left>=-1 && r.right<=innerWidth+1 && d.scrollWidth<=d.clientWidth+1;}`)
			s.shot(fmt.Sprintf("metadata-organize-%s-%d", theme, width))
		}
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="save"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal') && edMetadataFixture.state.libraries.get(edFixture.seededID)?.tags.includes('local')`)
	p.MustEval(`()=>{const r=edMetadataFixture.state.requests.findLast(x=>x.method==='PUT' && x.path.endsWith('/library'));if(!r || r.body.base_revision!==0 || r.body.tags[0]!=='local' || !r.body.folder_id)throw Error('Library write contract');}`)

	// A concurrent metadata edit must leave the pending tag visible after the CAS refusal.
	s.wait(`()=>!!document.querySelector('.ed-flow-card h3')`)
	p.MustElement(`[data-ed-card-menu]`).MustClick()
	p.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu button')].find(e=>e.textContent.includes(t('easydrag.ui.organize'))).click()`)
	s.wait(`()=>!!document.querySelector('[data-ed-tag-input]') && document.querySelector('[data-ed-organization-tags]').textContent.includes('local')`)
	p.MustElement(`[data-ed-tag-input]`).MustInput("pending")
	p.MustElement(`[data-ed-tag-add]`).MustClick()
	p.MustEval(`()=>{const f=edMetadataFixture.state.libraries.get(edFixture.seededID);edMetadataFixture.setLibrary(edFixture.seededID,{folder_id:f.folder_id,tags:['elsewhere'],revision:f.revision+1});}`)
	p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="save"]`).MustClick()
	s.wait(`()=>!document.querySelector('[data-ed-metadata-error]').hidden && document.querySelector('[data-ed-organization-tags]').textContent.includes('pending') && !document.querySelector('[data-ed-metadata-reload]').hidden`)
	capture("conflict")
	closeModal()
	s.wait(`()=>!document.querySelector('.ed-modal')`)

	// New publication atomically stores its note, even when mission synchronization is partial.
	p.MustElement(`.ed-flow-card h3`).MustClick()
	s.wait(`()=>!!edFixture.editor() && !!document.querySelector('[data-ed-cmd="publish"]')`)
	p.MustEval(`()=>{const ed=edFixture.editor();ed.model.setLabel(ed.model.doc.nodes[0].id,'Changed for publication');edMetadataFixture.state.partialNext=true;}`)
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-publish-note]')`)
	p.MustElement(`[data-ed-publish-note]`).MustInput("Release note")
	capture("publish")
	p.MustElement(`.ed-modal--publish [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--publish') && edFixture.editor().publishIncomplete && edFixture.editor().flow.live_revision===2`)
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>{const d=document.querySelector('.ed-modal--publish');return d && !d.querySelector('[data-ed-publish-note]') && d.textContent.includes('Release note');}`)
	capture("repair")
	p.MustElement(`.ed-modal--publish [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--publish') && !edFixture.editor().publishIncomplete`)
	p.MustEval(`()=>{const rows=edMetadataFixture.state.requests.filter(x=>x.method==='POST' && x.path.endsWith('/publish'));if(rows.length<2 || rows.at(-2).body.note!=='Release note' || Object.hasOwn(rows.at(-1).body,'note'))throw Error('Publish/repair note contract');}`)

	// A note becoming sensitive after listing must replace the cached preview with its masked GET.
	more("easydrag.ui.versions_title")
	s.wait(`()=>!!document.querySelector('[data-ed-note-read="2"]') && document.querySelector('.ed-versions-list').textContent.includes('Release note')`)
	p.MustEval(`()=>edMetadataFixture.maskText('Release note')`)
	p.MustElement(`[data-ed-note-read="2"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-note-display]')?.textContent.includes('[redacted]')`)
	closeModal()
	s.wait(`()=>!document.querySelector('.ed-versions-list').textContent.includes('Release note')`)
	p.MustElement(`[data-ed-note-edit="2"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-note-replace]')`)
	p.MustElement(`[data-ed-note-replace]`).MustClick()
	p.MustElement(`[data-ed-note-text]`).MustInput("Replacement")
	p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="save"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-versions-list').textContent.includes('Replacement')`)
	closeModal()

	// The separate block version API has the same note editor and CAS contract.
	p.MustEval(`()=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{section:'blocks'})`)
	s.wait(`()=>!!document.querySelector('.ed-block-library')`)
	p.MustElement(`[data-ed-new]`).MustClick()
	p.MustElement(`[data-ed-block-name]`).MustInput("Reusable")
	p.MustElement(`[data-ed-action="create"]`).MustClick()
	s.wait(`()=>!!edFixture.editor() && edFixture.editor().isBlock`)
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-publish-note]')`)
	p.MustElement(`[data-ed-publish-note]`).MustInput("Block note")
	p.MustElement(`.ed-modal--publish [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal--publish') && edFixture.editor().flow.live_revision===1`)
	more("easydrag.ui.versions_title")
	s.wait(`()=>!!document.querySelector('[data-ed-note-edit="1"]')`)
	p.MustElement(`[data-ed-note-edit="1"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-note-text]')?.value==='Block note'`)
	capture("block-note")
	p.MustElement(`[data-ed-note-text]`).MustSelectAllText().MustInput("Block revised")
	p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="save"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-modal').textContent.includes('Block revised')`)
	p.MustEval(`()=>{const r=edMetadataFixture.state.requests.findLast(x=>x.method==='PUT' && /flow-blocks\/[^/]+\/versions\/1\/note$/.test(x.path));if(!r || r.body.base_revision!==1 || r.body.note!=='Block revised')throw Error('Block note CAS');}`)

	// A read-only refusal preserves the unsaved note but cannot grant another write.
	p.MustElement(`[data-ed-note-edit="1"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-note-text]')?.value==='Block revised'`)
	p.MustElement(`[data-ed-note-text]`).MustSelectAllText().MustInput("Unsaved note")
	p.MustEval(`()=>{window.__metadataProbeStart=edMetadataFixture.state.requests.length;edMetadataFixture.state.writeFailure='FLOW_PERMISSION_DENIED';}`)
	p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="save"]`).MustClick()
	s.wait(`()=>{const input=document.querySelector('[data-ed-note-text]'),save=document.querySelector('.ed-modal-backdrop:last-child [data-ed-action="save"]');return input && input.value==='Unsaved note' && save && save.disabled;}`)
	s.wait(`()=>edMetadataFixture.state.requests.slice(window.__metadataProbeStart).some(r=>r.method==='GET' && r.path.endsWith('/versions/1/note'))`)
	closeModal()

	// A later lost read scope removes cached note text rather than retaining the old version list.
	p.MustElement(`[data-ed-note-edit="1"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-note-text]')?.value==='Block revised'`)
	p.MustElement(`[data-ed-note-text]`).MustSelectAllText().MustInput("Do not save")
	p.MustEval(`()=>edMetadataFixture.revoke()`)
	p.MustElement(`.ed-modal-backdrop:last-child [data-ed-action="save"]`).MustClick()
	s.wait(`()=>!document.querySelector('.ed-modal') || !document.querySelector('.ed-modal').textContent.includes('Block revised')`)
	p.MustEval(`()=>edMetadataFixture.restore()`)
	s.noRawKeys("metadata journey")
	s.failOnPageErrors("metadata journey")
}
