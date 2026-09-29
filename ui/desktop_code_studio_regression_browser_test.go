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

func TestDesktopCodeStudioSaveOwnership(t *testing.T) {
	cases := []struct{ name, script string }{
		{"save keeps later edits dirty", `
            const saving=app.saveCurrentFile(id);await csWait(()=>releases.length===1);
            tab.view.dispatch({changes:{from:0,insert:'// later\n'}});
            releases.shift()();await saving;
            const disk=await app.api.file(tab.path);
            return tab.modified && tab.content.includes('// later') && !disk.content.includes('// later');`},
		{"save all keeps later edits after switching tabs", `
            const saving=app.command('saveAllFiles',[],id);await csWait(()=>releases.length===1);
            tab.view.dispatch({changes:{from:0,insert:'// later\n'}});
            await app.openFile('/workspace/README.md',true,id);
            releases.shift()();await saving;
            return tab.modified && tab.content.includes('// later') && !(await app.api.file(tab.path)).content.includes('// later');`},
		{"overlapping saves are serialized", `
            const first=app.saveCurrentFile(id);await csWait(()=>releases.length===1);
            tab.view.dispatch({changes:{from:0,insert:'// later\n'}});
            const second=app.saveCurrentFile(id);await Promise.resolve();await Promise.resolve();
            const serialized=releases.length===1;
            releases.shift()();await first;await csWait(()=>releases.length===1);
            releases.shift()();await second;
            return serialized && !tab.modified && (await app.api.file(tab.path)).content===tab.content;`},
		{"run stops after superseded save", `
            const running=app.command('runCurrentFile',[],id);await csWait(()=>releases.length===1);
            tab.view.dispatch({changes:{from:0,insert:'// later\n'}});
            releases.shift()();await running;
            return execs===0 && tab.modified;`},
		{"closed document cannot complete a run", `
            const running=app.command('runCurrentFile',[],id);await csWait(()=>releases.length===1);
            await app.command('closeTab',[0,true],id);
            releases.shift()();await running;
            return execs===0 && target.openTabs.length===0;`},
		{"rename waits for pending saves", `
            const saving=app.saveCurrentFile(id);await csWait(()=>releases.length===1);
            csKey(csRoot().querySelector('[data-file-path="/workspace/main.go"]'),{key:'F2'});
            const form=document.querySelector('.cs-modal-backdrop form');
            form.elements.value.value='renamed.go';form.requestSubmit();await csWait(()=>tab.pathMutation);
            const waited=mutations===0;
            releases.shift()();await saving;await csWait(()=>tab.path==='/workspace/renamed.go');
            return waited && (await app.api.file(tab.path)).content===tab.content && (await fetch('/api/code-studio/file?path=%2Fworkspace%2Fmain.go')).status===404;`},
		{"delete waits for pending saves", `
            const saving=app.saveCurrentFile(id);await csWait(()=>releases.length===1);
            csKey(csRoot().querySelector('[data-file-path="/workspace/main.go"]'),{key:'Delete'});
            document.querySelector('.cs-modal-backdrop [data-confirm]').click();await csWait(()=>tab.pathMutation);
            const waited=mutations===0;
            releases.shift()();await saving;await csWait(()=>!target.openTabs.includes(tab));
            return waited && (await fetch('/api/code-studio/file?path=%2Fworkspace%2Fmain.go')).status===404;`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _ := newCodeStudioBrowser(t)
			result, err := page.Eval(`async()=>{
                await csWait(()=>csState()?.terminalSessions.length);
                const app=CodeStudioApp,target=csState(),id=target.windowId;
                await app.openFile('/workspace/main.go',true,id);
                const tab=target.openTabs[0];tab.view.dispatch({changes:{from:0,insert:'// snapshot\n'}});
                const originalFetch=window.fetch,releases=[];let execs=0,mutations=0;
                window.fetch=async(url,opts)=>{
                    if(String(url)==='/api/code-studio/file'&&opts?.method==='PUT')await new Promise(r=>releases.push(r));
                    if(String(url)==='/api/code-studio/exec')execs++;
                    if(opts?.method==='PATCH'||opts?.method==='DELETE')mutations++;
                    return originalFetch(url,opts);
                };
                try {` + tc.script + `} finally {window.fetch=originalFetch;releases.forEach(r=>r());}
            }`)
			if err != nil || !result.Value.Bool() {
				t.Fatalf("save ownership failed: %v", err)
			}
		})
	}
}

func TestDesktopCodeStudioFailedSaveStopsRun(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	if !page.MustEval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const app=CodeStudioApp,id=csState().windowId;
        await app.openFile('/workspace/main.go',true,id);
        const tab=csState().openTabs[0];tab.view.dispatch({changes:{from:0,insert:'// unsaved\n'}});
        const originalFetch=window.fetch;let execs=0;
        window.fetch=async(url,opts)=>{
            if(String(url)==='/api/code-studio/file'&&opts?.method==='PUT')return new Response(JSON.stringify({error:'disk full'}),{status:500});
            if(String(url)==='/api/code-studio/exec')execs++;
            return originalFetch(url,opts);
        };
        try {await app.command('runCurrentFile',[],id);return execs===0 && tab.modified;}
        finally {window.fetch=originalFetch;}
    }`).Bool() {
		t.Fatal("Run executed after a failed save")
	}
}

func TestDesktopCodeStudioAsyncWindows(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	if !page.MustEval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const app=CodeStudioApp,first=csState(),originalFetch=window.fetch;
        let releaseList,releaseStatus,listWaiting=false,statusWaiting=false,second;
        window.fetch=async(url,opts)=>{
            if(String(url)==='/api/code-studio/files?path=%2Fworkspace%2Fsrc'&&!listWaiting){
                listWaiting=true;await new Promise(r=>{releaseList=r;});
            }
            if(String(url)==='/api/code-studio/status'){
                statusWaiting=true;await new Promise(r=>{releaseStatus=r;});
            }
            return originalFetch(url,opts);
        };
        try {
            await app.openFile('/workspace/src/util/helpers.py',true,first.windowId);
            await csWait(()=>listWaiting);codeStudioTest.openApp('code-studio',{forceNew:true});
            await csWait(()=>statusWaiting);second=[...app.instances.values()].find(item=>item!==first);
            releaseList();await csWait(()=>!!first.treeCache['/workspace/src/util']);
            return first.expandedDirs.has('/workspace/src/util')&&!second.expandedDirs.has('/workspace/src/util');
        } finally {
            window.fetch=originalFetch;releaseList?.();releaseStatus?.();
            if(second){await csWait(()=>second.terminalSessions.length>0);codeStudioTest.closeWindow(second.windowId);}
        }
    }`).Bool() {
		t.Fatal("async work modified another window")
	}
}

func TestDesktopCodeStudioIgnoresStaleRequests(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	if !page.MustEval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const app=CodeStudioApp,target=csState(),id=target.windowId,originalFetch=window.fetch;
        let release;
        window.fetch=async(url,opts)=>{
            if(String(url)==='/api/code-studio/files?path=%2Fworkspace%2Fsrc')await new Promise(r=>{release=r;});
            return originalFetch(url,opts);
        };
        try {
            const old=app.refreshFiles('/workspace/src',id);await csWait(()=>release);
            await app.refreshFiles('/workspace',id);release();await old;
            const current=target.currentPath==='/workspace'&&target.files.some(f=>f.path==='/workspace/main.go');
            release=null;
            const disposed=app.refreshFiles('/workspace/src',id);await csWait(()=>release);
            app.dispose(id);release();await disposed;
            return current && !app.instances.has(id) && target.files.some(f=>f.path==='/workspace/main.go');
        } finally {window.fetch=originalFetch;release?.();}
    }`).Bool() {
		t.Fatal("stale or disposed request changed the explorer")
	}
}
