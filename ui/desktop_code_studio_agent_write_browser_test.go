package ui

import "testing"

// When the agent writes a file and then asks Code Studio to open it, a tab that is already open for that
// path must show what is on disk now. The "agent write" is simulated by changing the fixture backend and
// then calling the entry the open_app event uses (CodeStudioApp.openPath).
func TestDesktopCodeStudioAgentWriteRefreshesOpenTab(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	browserErrors := func() string {
		return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str()
	}
	setBackendFile := func(path, content string) {
		backend.mu.Lock()
		backend.files[path] = content
		backend.mu.Unlock()
	}
	scenario := func(name, js string, args ...interface{}) {
		t.Helper()
		result, err := page.Eval(js, args...)
		if err != nil {
			t.Errorf("%s: eval failed: %v; browser errors: %s", name, err, browserErrors())
			return
		}
		if !result.Value.Get("ok").Bool() {
			t.Errorf("%s: %s; browser errors: %s", name, result.Value.JSON("", ""), browserErrors())
		}
	}

	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v; errors=%s", err, browserErrors())
	}
	page.MustEval(`()=>{
        window.csDoc=path=>{const tab=csState().openTabs.find(item=>item.path===path&&item.kind!=='diff');return tab&&tab.view&&tab.view.state&&tab.view.state.doc?tab.view.state.doc.toString():null;};
        window.csActivePath=()=>{const tab=csState().openTabs[csState().activeTabIndex];return tab?tab.path:'';};
        window.csTabsFor=path=>csState().openTabs.filter(item=>item.path===path&&item.kind!=='diff');
        window.csVisibleText=()=>{const content=csRoot().querySelector('[data-editor] .cm-content');return content?content.textContent:null;};
        window.csAgentOpen=async(path,want,marker)=>{
            await CodeStudioApp.openPath(path);
            let timedOut=false;
            try{await csWait(()=>csDoc(path)===want,4000);}catch(e){timedOut=true;}
            const tabs=csTabsFor(path),visible=csVisibleText();
            return {
                ok:!timedOut&&tabs.length===1&&!tabs[0].modified&&csActivePath()===path&&visible!==null&&visible.includes(marker)&&csRoot().querySelectorAll('[data-editor] .cm-editor').length===1,
                timedOut,doc:csDoc(path),active:csActivePath(),visible,paths:csState().openTabs.map(item=>item.path),modified:tabs.map(item=>item.modified)
            };
        };
    }`)

	// a. A clean open tab picks up the new content of its file.
	page.MustEval(`async()=>{[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/main.go').click();await csWait(()=>csRoot().querySelector('.cm-editor')&&csDoc('/workspace/main.go')!==null,10000);}`)
	goV1 := "package main\n\nfunc main() {\n\tprintln(\"written by the agent\")\n}\n"
	setBackendFile("/workspace/main.go", goV1)
	scenario("a. clean open tab shows the agent's write", `async(path,want,marker)=>csAgentOpen(path,want,marker)`, "/workspace/main.go", goV1, "written by the agent")

	// b. The reported case: an empty file is open, the agent fills it, the editor must not stay empty.
	setBackendFile("/workspace/main.cpp", "")
	scenario("b. empty file opens as an empty editor", `async(path,want,marker)=>csAgentOpen(path,want,marker)`, "/workspace/main.cpp", "", "")
	cppHello := "#include <iostream>\n\nint main() {\n    std::cout << \"Hello, World!\" << std::endl;\n    return 0;\n}\n"
	setBackendFile("/workspace/main.cpp", cppHello)
	scenario("b. empty open tab shows the agent's hello world, no duplicate tab", `async(path,want,marker)=>csAgentOpen(path,want,marker)`, "/workspace/main.cpp", cppHello, "Hello, World!")

	// c. An inactive tab becomes active and shows the new content.
	page.MustEval(`async()=>{[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/README.md').click();await csWait(()=>csActivePath()==='/workspace/README.md'&&csDoc('/workspace/README.md')!==null,10000);}`)
	goV2 := "package main\n\nfunc main() {\n\tprintln(\"second write by the agent\")\n}\n"
	setBackendFile("/workspace/main.go", goV2)
	scenario("c. inactive tab becomes active with the new content", `async(path,want,marker)=>{
        if(csActivePath()===path||csTabsFor(path).length!==1||csTabsFor(path)[0].modified)return {ok:false,precondition:'main.go must be an unmodified background tab',active:csActivePath()};
        return csAgentOpen(path,want,marker);
    }`, "/workspace/main.go", goV2, "second write by the agent")

	// d. A tab with local edits is never overwritten.
	localDoc := page.MustEval(`async()=>{
        const path='/workspace/main.go',tab=csTabsFor(path)[0];
        tab.view.dispatch({changes:{from:0,insert:'// local edit\n'}});
        await csWait(()=>tab.modified,2000);
        return csDoc(path);
    }`).Str()
	goV3 := "package main\n\nfunc main() {\n\tprintln(\"rewritten by the agent after a local edit\")\n}\n"
	setBackendFile("/workspace/main.go", goV3)
	scenario("d. a modified tab keeps the local edit", `async(path,local,remote)=>{
        await CodeStudioApp.openPath(path);
        await new Promise(resolve=>setTimeout(resolve,400));
        const tabs=csTabsFor(path),doc=csDoc(path);
        return {
            ok:tabs.length===1&&tabs[0].modified===true&&doc===local&&doc.includes('// local edit')&&!doc.includes(remote)&&csActivePath()===path,
            doc,active:csActivePath(),modified:tabs.map(item=>item.modified),paths:csState().openTabs.map(item=>item.path)
        };
    }`, "/workspace/main.go", localDoc, "rewritten by the agent after a local edit")

	// e. None of this may raise browser errors.
	scenario("e. no browser errors", `()=>({ok:fixtureErrors.length===0,errors:fixtureErrors})`)
}
