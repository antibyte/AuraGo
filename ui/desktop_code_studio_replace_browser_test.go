package ui

import "testing"

// Project-wide replace in the Code Studio search panel: the preview, Replace All writing a closed file
// through PUT, an open tab with unsaved edits taking the change in its buffer (one undo step, no PUT),
// Replace All switched off for a truncated result list, and read-only mode switching replace off.
// The fixture UI runs in German, so every check uses data attributes and state, not labels.
func TestDesktopCodeStudioReplaceBrowser(t *testing.T) {
	const alpha = "/workspace/replace-alpha.txt"
	const beta = "/workspace/replace-beta.txt"
	page, backend := newCodeStudioBrowser(t)
	backend.mu.Lock()
	backend.files[alpha] = "quokka one\nquokka quokka two\nplain\n"
	backend.files[beta] = "keep quokka here\n"
	backend.mu.Unlock()
	browserErrors := func() string {
		return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str()
	}
	scenario := func(name, js string, args ...interface{}) {
		t.Helper()
		result, err := page.Eval(js, args...)
		if err != nil {
			t.Fatalf("%s: eval failed: %v; browser errors: %s", name, err, browserErrors())
		}
		if !result.Value.Get("ok").Bool() {
			t.Fatalf("%s: %s; browser errors: %s", name, result.Value.JSON("", ""), browserErrors())
		}
	}
	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v; errors=%s", err, browserErrors())
	}

	// beta.txt gets an unsaved edit that shifts its lines; then main.go becomes the active tab, so beta
	// lives on as a background tab whose editor state sits in its snapshot.
	scenario("dirty background tab", `async(beta)=>{
		await CodeStudioApp.openFile(beta);
		const tab=await csWait(()=>csState().openTabs.find(item=>item.path===beta&&item.view&&item.view.state),10000);
		tab.view.dispatch({changes:{from:0,insert:'// local\n'}});
		await csWait(()=>tab.modified,2000);
		await CodeStudioApp.openFile('/workspace/main.go');
		await csWait(()=>{const active=csState().openTabs[csState().activeTabIndex];return active&&active.path==='/workspace/main.go';},10000);
		return {ok:tab.modified&&!tab.view&&tab.content==='// local\nkeep quokka here\n',modified:tab.modified,hasView:!!tab.view,content:tab.content};
	}`, beta)

	// Search, open the replace row and type the replacement: every match shows old and new text, the
	// second match on a line has its own row, and beta's row uses the buffer's line number (2, not 1).
	scenario("preview", `async()=>{
		csRoot().querySelector('[data-activity="search"]').click();
		const form=csRoot().querySelector('[data-search-form]');
		form.elements.q.value='quokka';
		form.requestSubmit();
		await csWait(()=>csState().searchPerformed&&csState().searchQuery==='quokka'&&csRoot().querySelectorAll('.cs-search-result').length>0,10000);
		csRoot().querySelector('[data-search-replace-toggle]').click();
		const input=await csWait(()=>csRoot().querySelector('[data-search-replace-input]'),2000);
		input.value='wombat';
		input.dispatchEvent(new Event('input',{bubbles:true}));
		await csWait(()=>csRoot().querySelectorAll('.cs-replace-new').length===4,4000);
		const olds=[...csRoot().querySelectorAll('.cs-replace-old')].map(el=>el.textContent);
		const news=[...csRoot().querySelectorAll('.cs-replace-new')].map(el=>el.textContent);
		const second=csRoot().querySelector('[data-replace-one][data-replace-path="/workspace/replace-alpha.txt"][data-replace-line="2"][data-replace-occurrence="1"]');
		const row=second&&second.closest('.cs-replace-match');
		const rowText=row?row.querySelector('code').textContent:'';
		const betaRow=csRoot().querySelector('[data-replace-one][data-replace-path="/workspace/replace-beta.txt"]');
		const all=csRoot().querySelector('[data-search-replace-all]');
		return {
			ok:olds.length===4&&olds.every(text=>text==='quokka')&&news.every(text=>text==='wombat')&&rowText==='quokka quokkawombat two'&&!!betaRow&&betaRow.dataset.replaceLine==='2'&&csRoot().querySelectorAll('[data-replace-file-action]').length===2&&!!all&&!all.disabled,
			olds,news,rowText,betaLine:betaRow&&betaRow.dataset.replaceLine,allDisabled:all&&all.disabled
		};
	}`)

	// Replace All asks with the counts, writes the closed file and changes the open tab only in its buffer.
	scenario("replace all", `async()=>{
		csRoot().querySelector('[data-search-replace-all]').click();
		const confirm=await csWait(()=>document.querySelector('.cs-modal-backdrop [data-confirm]'),4000);
		const message=document.querySelector('.cs-modal-backdrop p').textContent;
		confirm.click();
		const report=await csWait(()=>csState().projectReplaceReport,10000);
		return {
			ok:/4/.test(message)&&/2/.test(message)&&report.count===4&&report.files===2&&report.buffers===1&&report.skipped.length===0,
			message,count:report.count,files:report.files,buffers:report.buffers,skipped:report.skipped
		};
	}`)
	backend.mu.Lock()
	gotAlpha, gotBeta := backend.files[alpha], backend.files[beta]
	backend.mu.Unlock()
	if want := "wombat one\nwombat wombat two\nplain\n"; gotAlpha != want {
		t.Errorf("Replace All wrote %q to the closed file, want %q", gotAlpha, want)
	}
	if gotBeta != "keep quokka here\n" {
		t.Errorf("the open tab's file was written to disk: %q", gotBeta)
	}

	// The background tab stays dirty, shows the change when it is activated, and one undo takes it back.
	scenario("dirty tab got the change in its buffer", `async(beta)=>{
		const tab=csState().openTabs.find(item=>item.path===beta);
		const want='// local\nkeep wombat here\n';
		const before={modified:tab.modified,content:tab.content};
		await CodeStudioApp.openFile(beta);
		await csWait(()=>tab.view&&tab.view.state,10000);
		const shown=tab.view.state.doc.toString();
		const undone=csState().cmModule.undo(tab.view);
		const afterUndo=tab.view.state.doc.toString();
		return {ok:before.modified&&before.content===want&&shown===want&&undone&&afterUndo==='// local\nkeep quokka here\n'&&tab.modified,before,shown,undone,afterUndo};
	}`, beta)

	// A truncated result list switches Replace All off with a hint; Replace in File stays usable.
	backend.mu.Lock()
	backend.searchTruncated = true
	backend.mu.Unlock()
	scenario("Replace All is off for a truncated list", `async()=>{
		const form=csRoot().querySelector('[data-search-form]');
		form.elements.q.value='plain';
		form.requestSubmit();
		await csWait(()=>csState().searchQuery==='plain'&&csState().searchTruncated&&!csState().projectReplaceBusy&&csRoot().querySelector('[data-replace-file="/workspace/replace-alpha.txt"]'),10000);
		const all=csRoot().querySelector('[data-search-replace-all]');
		const file=csRoot().querySelector('[data-replace-file-action][data-replace-path="/workspace/replace-alpha.txt"]');
		const hint=csRoot().querySelector('[data-search-replace-hint]').textContent.trim();
		return {ok:all.disabled&&!!file&&!file.disabled&&hint.length>0,allDisabled:all.disabled,fileDisabled:file&&file.disabled,hint};
	}`)

	// Read-only mode switches the replace input and every replace button off.
	scenario("read-only turns replace off", `async()=>{
		document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}));
		await csWait(()=>csState().readOnly,2000);
		const all=csRoot().querySelector('[data-search-replace-all]');
		const input=csRoot().querySelector('[data-search-replace-input]');
		const buttons=[...csRoot().querySelectorAll('[data-replace-file-action],[data-replace-one]')];
		return {ok:all.disabled&&input.disabled&&buttons.length>0&&buttons.every(button=>button.disabled),allDisabled:all.disabled,inputDisabled:input.disabled,buttons:buttons.map(button=>button.disabled)};
	}`)

	scenario("no browser errors", `()=>({ok:fixtureErrors.length===0,errors:fixtureErrors})`)
}
