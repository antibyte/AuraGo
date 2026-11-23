package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// IDE package 1A: folding, Go to Line and Go to Symbol, two files in the split view, the editor settings and
// Format Document, driven through the real Code Studio shell and CodeMirror bundle.
func TestDesktopCodeStudioEditorBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	check := func(name, js string) {
		t.Helper()
		result, err := page.Eval(js)
		if err != nil {
			t.Errorf("%s: eval failed: %v; browser errors: %s", name, err, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
			return
		}
		if !result.Value.Bool() {
			t.Errorf("%s; browser errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	snapshot := func(name string) {
		t.Helper()
		dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v", err)
	}
	page.MustEval(`async()=>{[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/main.go').click();await csWait(()=>csRoot().querySelector('.cm-editor'),10000);}`)

	// A1: the fold gutter shows a marker for func main.
	check("the editor shows a fold gutter with a fold marker", `async()=>{await csWait(()=>csRoot().querySelector('.cm-editor .cm-foldGutter .cm-gutterElement span[title]'),5000);return true;}`)

	// A3: Ctrl+G opens the palette with ':' and line:column jumps there.
	page.MustEval(`()=>{const content=csRoot().querySelector('.cm-content');content.focus();csKey(content,{key:'g',ctrlKey:true});}`)
	check("Ctrl+G opens the palette in go-to-line mode", `async()=>{const input=await csWait(()=>document.querySelector('.cs-command-palette-backdrop input'),3000);return input.value===':';}`)
	page.MustEval(`()=>{const input=document.querySelector('.cs-command-palette-backdrop input');input.value=':5:2';input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}));}`)
	check("line:column jumps in the active editor", `async()=>{await csWait(()=>!document.querySelector('.cs-command-palette-backdrop'),3000);const view=csState().openTabs[csState().activeTabIndex].view;const head=view.state.selection.main.head;const line=view.state.doc.lineAt(head);return line.number===5&&head-line.from===1;}`)

	// A3: '@' lists the outline symbols of the active file.
	page.MustEval(`()=>{window.CodeStudioApp.command('openGoToSymbol',[],csState().windowId);}`)
	check("@ lists the symbols of the active file", `async()=>{const input=await csWait(()=>document.querySelector('.cs-command-palette-backdrop input'),3000);input.value='@mai';input.dispatchEvent(new Event('input',{bubbles:true}));return [...document.querySelectorAll('.cs-command-palette-backdrop .cs-cp-item-label')].some(el=>el.textContent==='main');}`)
	page.MustEval(`()=>{const input=document.querySelector('.cs-command-palette-backdrop input');if(input)input.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}));}`)

	// A1/A7: the fold command folds func main at the cursor (line 5) and unfold opens it again.
	page.MustEval(`()=>{window.CodeStudioApp.command('runEditorCommand',['foldCode'],csState().windowId);}`)
	check("Fold collapses func main", `async()=>{await csWait(()=>csRoot().querySelector('.cm-foldPlaceholder'),3000);return true;}`)
	page.MustEval(`()=>{window.CodeStudioApp.command('runEditorCommand',['unfoldCode'],csState().windowId);}`)
	check("Unfold opens it again", `()=>!csRoot().querySelector('.cm-foldPlaceholder')`)

	// A4: Open to the Side puts README.md next to main.go; focusing a pane makes its file the active tab.
	page.MustEval(`async()=>{await window.CodeStudioApp.command('openToSide',['/workspace/README.md'],csState().windowId);await csWait(()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===2,10000);}`)
	snapshot("cs-editor-01-split")
	check("Open to the Side shows two different files", `()=>{const primary=csRoot().querySelector('[data-split-pane="primary"] .cm-content');const secondary=csRoot().querySelector('[data-split-pane="secondary"] .cm-content');return !!primary&&!!secondary&&primary.textContent.includes('package main')&&secondary.textContent.includes('# Demo');}`)
	check("the side file is the active tab", `()=>csState().openTabs[csState().activeTabIndex].path==='/workspace/README.md'`)
	page.MustEval(`()=>{const content=csRoot().querySelector('[data-split-pane="primary"] .cm-content');content.focus();content.dispatchEvent(new FocusEvent('focusin',{bubbles:true}));}`)
	check("focusing the other pane makes its file the active tab", `()=>csState().openTabs[csState().activeTabIndex].path==='/workspace/main.go'&&csRoot().querySelectorAll('[data-editor] .cm-editor').length===2`)
	page.MustEval(`()=>{const content=csRoot().querySelector('[data-split-pane="primary"] .cm-content');csKey(content,{key:'\\',code:'Backslash',ctrlKey:true});}`)
	check("Ctrl+Backslash closes the split", `()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===1&&!csState().splitMode`)
	// Closing the primary pane's tab while the secondary pane has focus ends the split and keeps the other file.
	page.MustEval(`async()=>{await window.CodeStudioApp.command('openToSide',['/workspace/README.md'],csState().windowId);await csWait(()=>csRoot().querySelectorAll('[data-editor] .cm-editor').length===2,10000);const content=csRoot().querySelector('[data-split-pane="secondary"] .cm-content');content.focus();content.dispatchEvent(new FocusEvent('focusin',{bubbles:true}));await window.CodeStudioApp.command('closeTab',[csState().openTabs.findIndex(tab=>tab.path==='/workspace/main.go')],csState().windowId);}`)
	check("closing the primary tab ends the split and keeps the other file", `async()=>{await csWait(()=>!csState().splitMode&&csRoot().querySelectorAll('[data-editor] .cm-editor').length===1,5000);return csState().openTabs[csState().activeTabIndex].path==='/workspace/README.md';}`)
	page.MustEval(`async()=>{await window.CodeStudioApp.openFile('/workspace/main.go',true,csState().windowId);}`)

	// A5: the gear opens the settings; tab size 2 reaches the open editor live and is stored.
	page.MustEval(`()=>{csRoot().querySelector('[data-activity="settings"]').click();}`)
	check("the gear opens the settings dialog", `async()=>{await csWait(()=>document.querySelector('[data-cs-settings]'),3000);return true;}`)
	page.MustEval(`()=>{const select=document.querySelector('[data-cs-settings] [data-setting="tabSize"]');select.value='2';select.dispatchEvent(new Event('change',{bubbles:true}));}`)
	snapshot("cs-editor-02-settings")
	check("tab size 2 applies to the open editor", `()=>{const view=csState().openTabs[csState().activeTabIndex].view;return view.state.tabSize===2&&view.state.facet(csState().cmModule.indentUnit)==='  ';}`)
	check("the settings are stored", `()=>JSON.parse(localStorage.getItem('aurago.codeStudio.settings.v1')).tabSize===2`)
	check("the status bar shows the indentation", `()=>{const item=csRoot().querySelector('[data-status-indent]');return !!item&&item.textContent.includes('2');}`)
	page.MustEval(`()=>{document.querySelector('[data-cs-settings] [data-settings-close]').click();}`)
	check("the settings dialog closes", `()=>!document.querySelector('[data-cs-settings]')`)

	// A6: Shift+Alt+F formats JSON in the browser with the configured indent, as one undo step.
	page.MustEval(`async()=>{await window.CodeStudioApp.openFile('/workspace/package.json',true,csState().windowId);await csWait(()=>{const tab=csState().openTabs[csState().activeTabIndex];return tab&&tab.path==='/workspace/package.json'&&tab.view;},10000);}`)
	page.MustEval(`()=>{const view=csState().openTabs[csState().activeTabIndex].view;view.dispatch({changes:{from:0,to:view.state.doc.length,insert:'{"name":"demo","n":12345678901234567890}'}});const content=csRoot().querySelector('.cm-content');content.focus();csKey(content,{key:'F',code:'KeyF',shiftKey:true,altKey:true});}`)
	check("Shift+Alt+F formats the JSON document", `async()=>{const view=csState().openTabs[csState().activeTabIndex].view;await csWait(()=>view.state.doc.toString().includes('\n'),5000);return view.state.doc.toString()==='{\n  "name": "demo",\n  "n": 12345678901234567890\n}';}`)
	check("formatting is a single undo step", `()=>{const view=csState().openTabs[csState().activeTabIndex].view;csState().cmModule.undo(view);return view.state.doc.toString()==='{"name":"demo","n":12345678901234567890}';}`)
	snapshot("cs-editor-03-format")

	check("no browser errors after the editor tour", `()=>fixtureErrors.length===0`)
}
