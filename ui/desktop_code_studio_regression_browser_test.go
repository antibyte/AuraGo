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

func TestDesktopCodeStudioSuggestions(t *testing.T) {
	cases := []struct{ name, script string }{
		{"selection replacement and undo", `
            tab.view.dispatch({selection:{anchor:0,head:4}});await suggest('refactor');
            const suggestion=target.pendingSuggestion.text||target.pendingSuggestion;
            csRoot().querySelector('[data-agent-apply]').click();
            const replaced=tab.content===suggestion+before.slice(4);
            target.cmModule.undo(tab.view);
            return replaced && tab.content===before;`},
		{"selection in the second pane", `
            app.command('splitEditor',['right'],id);
            tab.secondaryView.focus();tab.secondaryView.dispatch({selection:{anchor:0,head:4}});
            await suggest('comments');const suggestion=target.pendingSuggestion.text;
            csRoot().querySelector('[data-agent-apply]').click();
            return tab.content===suggestion+before.slice(4);`},
		{"apply uses original document", `
            await suggest('refactor');const suggestion=target.pendingSuggestion.text||target.pendingSuggestion;
            await app.openFile('/workspace/styles.css',true,id);
            const other=target.openTabs[target.activeTabIndex],otherBefore=other.content;
            csRoot().querySelector('[data-agent-apply]').click();
            return tab.content===suggestion && other.content===otherBefore && !other.modified;`},
		{"editing invalidates a suggestion", `
            await suggest('refactor');tab.view.dispatch({changes:{from:0,insert:'changed '}});
            const edited=tab.content;csRoot().querySelector('[data-agent-apply]').click();
            return tab.content===edited && !!target.pendingSuggestion && !!csRoot().querySelector('[data-agent-copy]');`},
		{"tests are copy only", `
            await suggest('tests');const button=csRoot().querySelector('[data-agent-apply]');
            button?.click();return (!button||button.disabled) && tab.content===before;`},
		{"explanations are copy only", `
            await suggest('explain');const button=csRoot().querySelector('[data-agent-apply]');
            button?.click();return (!button||button.disabled) && tab.content===before;`},
		{"cancelled response cannot replace a newer suggestion", `
            const originalFetch=window.fetch;let release;
            window.fetch=async(url,opts)=>{
                if(String(url)==='/api/desktop/chat'){
                    await new Promise(r=>{release=r;});
                    return new Response(JSON.stringify({answer:'old reply'}),{status:200});
                }
                return originalFetch(url,opts);
            };
            try {
                csRoot().querySelector('[data-code-action="refactor"]').click();await csWait(()=>release);
                csRoot().querySelector('[data-agent-stop]').click();window.fetch=originalFetch;
                await suggest('comments');const newer=target.pendingSuggestion;
                release();await csWait(()=>!target.agentBusy);await new Promise(r=>requestAnimationFrame(r));
                return target.pendingSuggestion===newer && target.agentMessages.at(-1).text!=='old reply';
            } finally {window.fetch=originalFetch;release?.();}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, _ := newCodeStudioBrowser(t)
			result, err := page.Eval(`async()=>{
                await csWait(()=>csState()?.terminalSessions.length);
                const app=CodeStudioApp,target=csState(),id=target.windowId;
                await app.openFile('/workspace/README.md',true,id);
                const tab=target.openTabs[0],before=tab.content;
                app.command('toggleAgentPanel',[],id);
                const suggest=async(action)=>{
                    target.pendingSuggestion=null;
                    csRoot().querySelector('[data-code-action="'+action+'"]').click();
                    await csWait(()=>target.pendingSuggestion&&!target.agentBusy);
                };
                ` + tc.script + `
            }`)
			if err != nil || !result.Value.Bool() {
				t.Fatalf("suggestion ownership failed: %v", err)
			}
		})
	}
}

func TestDesktopCodeStudioEditorRetention(t *testing.T) {
	for _, split := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "split"}[split], func(t *testing.T) {
			page, _ := newCodeStudioBrowser(t)
			if !page.MustEval(`async(split)=>{
                await csWait(()=>csState()?.terminalSessions.length);
                const app=CodeStudioApp,target=csState(),id=target.windowId;
                await app.api.writeFile('/workspace/README.md',Array.from({length:160},(_,i)=>'line '+i).join('\n'));
                await app.openFile('/workspace/README.md',true,id);
                const tab=target.openTabs[0],before=tab.content;
                if(split)app.command('splitEditor',['right'],id);
                const view=split?tab.secondaryView:tab.view;
                view.dispatch({changes:{from:0,insert:'marker\n'},selection:{anchor:3}});
                await csWait(()=>view.scrollDOM.scrollHeight>1000).catch(()=>{throw new Error('long document did not lay out');});
                view.scrollDOM.scrollTop=500;
                await new Promise(r=>requestAnimationFrame(r));
                const scrollTop=view.scrollDOM.scrollTop;
                await app.openFile('/workspace/styles.css',true,id);
                document.body.dataset.theme='fruity';document.body.dataset.fruityMode='light';
                await app.openFile(tab.path,true,id);
                const restored=split?tab.secondaryView:tab.view;
                const selection=restored.state.selection.main.head===3;
                await csWait(()=>Math.abs(restored.scrollDOM.scrollTop-scrollTop)<2).catch(()=>{throw new Error('scroll restore '+JSON.stringify({expected:scrollTop,actual:restored.scrollDOM.scrollTop,saved:(split?tab.secondaryEditorSnapshot:tab.editorSnapshot)?.top}));});
                if(split){restored.focus();csKey(restored.contentDOM,{ctrlKey:true,key:'z',code:'KeyZ'});}
                else target.cmModule.undo(tab.view);
                const undone=tab.content===before && (!split||tab.secondaryView.state.doc.toString()===before);
                const exhausted=!target.cmModule.undo(tab.view);
                return selection && undone && exhausted;
            }`, split).Bool() {
				t.Fatal("editor selection or undo was lost across tab/theme changes")
			}
		})
	}
}

func TestDesktopCodeStudioTextareaRetention(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	if !page.MustEval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const app=CodeStudioApp,target=csState(),id=target.windowId;target.editorType='textarea';
        await app.openFile('/workspace/README.md',true,id);
        const tab=target.openTabs[0];tab.view.setValue('unsaved text');tab.view.textarea.setSelectionRange(2,5);
        await app.openFile('/workspace/styles.css',true,id);await app.openFile(tab.path,true,id);
        return tab.modified && tab.content==='unsaved text' && tab.view.textarea.selectionStart===2 && tab.view.textarea.selectionEnd===5;
    }`).Bool() {
		t.Fatal("textarea remount lost dirty state or selection")
	}
}

func TestDesktopCodeStudioDirectoryLaunch(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	if !page.MustEval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const first=csState(),originalFetch=window.fetch;let directoryReads=0;
        window.fetch=(url,opts)=>{
            if(String(url)==='/api/code-studio/file?path=%2Fworkspace%2Fsrc')directoryReads++;
            return originalFetch(url,opts);
        };
        try {
            codeStudioTest.openApp('code-studio',{path:'/workspace/src'});
            await csWait(()=>first.currentPath==='/workspace/src'&&first.files.some(f=>f.name==='app.js'));
            codeStudioTest.openApp('code-studio',{path:'/workspace/src',forceNew:true});
            await csWait(()=>[...CodeStudioApp.instances.values()].some(s=>s!==first&&s.currentPath==='/workspace/src'&&s.terminalSessions.length));
            const second=[...CodeStudioApp.instances.values()].find(s=>s!==first);
            await CodeStudioApp.openPath('/workspace/src/app.js',true,second.windowId);
            return directoryReads===0 && second.openTabs[second.activeTabIndex].path==='/workspace/src/app.js' && fixtureErrors.length===0;
        } finally {window.fetch=originalFetch;}
    }`).Bool() {
		t.Fatal("opening a directory attempted to read it as a file")
	}
}

func TestDesktopCodeStudioTerminalRouting(t *testing.T) {
	for _, closeOrigin := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch", true: "close"}[closeOrigin], func(t *testing.T) {
			page, _ := newCodeStudioBrowser(t)
			if !page.MustEval(`async(closeOrigin)=>{
                await csWait(()=>csState()?.terminalSessions[0]?.ws?.readyState===1);
                const app=CodeStudioApp,target=csState(),id=target.windowId;
                await app.openFile('/workspace/src/app.js',true,id);
                csRoot().querySelector('[data-terminal-add]').click();
                await csWait(()=>target.terminalSessions[1]?.ws?.readyState===1);
                const origin=target.terminalSessions[1],first=target.terminalSessions[0];
                const aliases=target.terminal===origin.term&&target.ws===origin.ws&&target.fitAddon===origin.fitAddon;
                const text=term=>Array.from({length:term.buffer.active.length},(_,i)=>term.buffer.active.getLine(i)?.translateToString()||'').join('\n');
                const originalFetch=window.fetch;let release;
                window.fetch=async(url,opts)=>{
                    if(String(url)==='/api/code-studio/exec'){
                        await new Promise(r=>{release=r;});
                        return new Response(JSON.stringify({output:'origin-only-output',exit_code:0}),{status:200});
                    }
                    return originalFetch(url,opts);
                };
                try {
                    const running=app.command('runCurrentFile',[],id);await csWait(()=>release);
                    csRoot().querySelector(closeOrigin?'[data-terminal-close="1"]':'[data-terminal-tab="0"]').click();
                    release();await running;await new Promise(r=>requestAnimationFrame(r));
                    if(!closeOrigin)await csWait(()=>text(origin.term).includes('origin-only-output'));
                    return aliases && !text(first.term).includes('origin-only-output') && target.terminal===first.term && fixtureErrors.length===0;
                } finally {window.fetch=originalFetch;release?.();}
            }`, closeOrigin).Bool() {
				t.Fatal("Run output or terminal aliases followed the wrong session")
			}
		})
	}
}
