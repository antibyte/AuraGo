package ui

import "testing"

func TestDesktopCodeStudioCreatePreservesExistingFile(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result := page.MustEval(`async()=>{
        await csWait(()=>csRoot()?.querySelector('.cs-tree-item'));
        const app=CodeStudioApp,id=csState().windowId;
        await app.openFile('/workspace/main.go',true,id);
        const tab=csState().openTabs[0],before=(await app.api.file(tab.path)).content;
        const creating=app.command('createNewFile',[],id);
        const form=document.querySelector('.cs-modal-backdrop form');
        form.elements.value.value='main.go';form.requestSubmit();await creating;
        return (await app.api.file(tab.path)).content===before && tab.view.state.doc.toString()===before && fixtureErrors.length===0;
    }`)
	if !result.Bool() {
		t.Fatal("New File changed an existing document")
	}
}
