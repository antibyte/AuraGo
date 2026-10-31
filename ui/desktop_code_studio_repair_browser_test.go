package ui

import "testing"

func TestDesktopCodeStudioLivePolicyAndDrafts(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "codemirror", true: "textarea"}[fallback], func(t *testing.T) {
			page, _ := newCodeStudioBrowser(t)
			result, err := page.Eval(`async(fallback)=>{
                await csWait(()=>csState()?.terminalSessions[0]?.ws?.readyState===1);
                const app=CodeStudioApp,target=csState(),id=target.windowId;
                if(fallback)target.editorType='textarea';
                await app.openFile('/workspace/main.go',true,id);
                app.command('splitEditor',['right'],id);
                const tab=target.openTabs[0],draft='// retained draft\n'+tab.content;
                if(fallback)tab.view.setValue(draft);else tab.view.dispatch({changes:{from:0,insert:'// retained draft\n'}});
                const oldSocket=target.ws;
                app.command('toggleGitPanel',[],id);
                await csWait(()=>target.root.querySelector('[data-git-commit-msg]'));
                const message=target.root.querySelector('[data-git-commit-msg]');message.value='retained commit draft';message.dispatchEvent(new Event('input'));
                const policy=value=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:value}}));
                policy(true);
                const locked=target.readOnly&&tab.views.length===2&&tab.views.every(v=>fallback?v.textarea.readOnly:v.state.readOnly);
                const blocked=await app.saveCurrentFile(id)===false;
                await app.command('createNewFile',[],id);await app.command('runCurrentFile',[],id);
                const runMenu=codeStudioTest.state.windowMenus.get(id).rawMenus.find(m=>m.id==='run').items.find(i=>i.id==='run-current');
                const controls=runMenu.disabled&&target.root.querySelector('[data-action="save"]').disabled&&target.root.querySelector('[data-terminal-add]').disabled&&target.root.querySelector('[data-git-commit]').disabled;
                await csWait(()=>oldSocket.readyState===3);
                const preserved=tab.content===draft&&tab.modified&&target.gitCommitMessage==='retained commit draft'&&!document.querySelector('.cs-modal-backdrop');
                await app.refreshFiles('/workspace/src',id);
                const readable=target.files.some(f=>f.name==='app.js');
                policy(false);
                const unlocked=!target.readOnly&&tab.views.every(v=>fallback?!v.textarea.readOnly:!v.state.readOnly)&&!target.root.querySelector('[data-action="save"]').disabled;
                const saved=await app.saveCurrentFile(id);
                target.root.querySelector('[data-terminal-add]').click();
                await csWait(()=>target.terminalSessions[0]?.ws?.readyState===1);
                return {locked,blocked,controls,preserved,readable,unlocked,saved,errors:fixtureErrors};
            }`, fallback)
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"locked", "blocked", "controls", "preserved", "readable", "unlocked", "saved"} {
				if !result.Value.Get(key).Bool() {
					t.Fatalf("policy/drafts: %s", result.Value.JSON("", "  "))
				}
			}
			if len(result.Value.Get("errors").Arr()) != 0 {
				t.Fatal(result.Value.JSON("", "  "))
			}
		})
	}
}

func TestDesktopCodeStudioPersistentTerminalDOMAndCleanup(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result, err := page.Eval(`async()=>{
        await csWait(()=>csState()?.terminalSessions[0]?.ws?.readyState===1);
        const app=CodeStudioApp,target=csState(),first=target.terminalSessions[0],firstElement=first.term.element,baseline=target.disposers.length;
        target.root.querySelector('[data-terminal-add]').click();
        await csWait(()=>target.terminalSessions[1]?.ws?.readyState===1);
        const second=target.terminalSessions[1],secondSocket=second.ws;
        for(let i=0;i<6;i++)target.root.querySelector('[data-terminal-tab="'+(i%2)+'"]').click();
        target.root.querySelector('[data-terminal-tab="0"]').click();
        const visible=first.term.element===firstElement&&firstElement.isConnected&&firstElement.getBoundingClientRect().height>0&&firstElement.getBoundingClientRect().width>0&&second.pane.hidden;
        target.root.querySelector('[data-terminal-close="1"]').click();
        await csWait(()=>secondSocket.readyState===3);
        const released=!second.pane.isConnected&&!second.dataSubscription&&target.disposers.length===baseline;
        const socket=first.ws;app.dispose(target.windowId);await csWait(()=>socket.readyState===3);
        return {visible,released,closed:!first.pane.isConnected,errors:fixtureErrors};
    }`)
	if err != nil || !result.Value.Get("visible").Bool() || !result.Value.Get("released").Bool() || !result.Value.Get("closed").Bool() || len(result.Value.Get("errors").Arr()) != 0 {
		t.Fatalf("terminal DOM/lifecycle: %v %v", err, result)
	}
}

func TestDesktopCodeStudioLateStatusKeepsCurrentPolicy(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result, err := page.Eval(`async()=>{
        await csWait(()=>window.CodeStudioApp&&csState()?.root.querySelector('[data-action="save"]'));
        const app=CodeStudioApp,fetchBefore=window.fetch,host=document.createElement('div');document.body.appendChild(host);
        let release,seen=false;
        window.fetch=(url,options)=>{
            if(String(url)==='/api/code-studio/status')return new Promise(resolve=>{seen=true;release=()=>resolve(new Response(JSON.stringify({status:'ok',code_studio:{enabled:true,running:true,read_only:false}}),{headers:{'Content-Type':'application/json'}}));});
            return fetchBefore(url,options);
        };
        try {
            const opening=app.render(host,'late-policy',{});
            await csWait(()=>seen);
            document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}));
            release();await opening;
            const target=app.instances.get('late-policy');
            return {locked:target.readOnly&&target.containerInfo.read_only&&target.terminalSessions.length===0&&target.root.querySelector('[data-action="save"]').disabled,errors:fixtureErrors};
        } finally {window.fetch=fetchBefore;app.dispose('late-policy');host.remove();}
    }`)
	if err != nil || !result.Value.Get("locked").Bool() || len(result.Value.Get("errors").Arr()) != 0 {
		t.Fatalf("late status overwrote current policy: %v %v", err, result)
	}
}

func TestDesktopCodeStudioDiffTabPreservesSource(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result, err := page.Eval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const app=CodeStudioApp,target=csState(),id=target.windowId;
        await app.openFile('/workspace/main.go',true,id);
        const source=target.openTabs[0],disk=(await app.api.file(source.path)).content;
        source.view.dispatch({changes:{from:0,insert:'// keep draft\n'},selection:{anchor:3}});
        const draft=source.content,fetchBefore=window.fetch,staged=[];let writes=0;
        window.fetch=(url,opts)=>{
            if(opts?.method==='PUT')writes++;
            if(String(url).startsWith('/api/code-studio/git/diff?'))staged.push(new URL(url,location.href).searchParams.get('staged'));
            return fetchBefore(url,opts);
        };
        try {
            app.command('toggleGitPanel',[],id);await csWait(()=>target.root.querySelector('[data-git-file="main.go"]'));
            target.root.querySelector('[data-git-file="main.go"]').click();
            await csWait(()=>target.openTabs.at(-1)?.kind==='diff'&&!target.openTabs.at(-1).loading);
            const diff=target.openTabs.at(-1),blocked=await app.saveCurrentFile(id)===false;
            await app.command('runCurrentFile',[],id);
            const select=target.root.querySelector('[data-diff-source]');select.value='index';select.dispatchEvent(new Event('change'));
            await csWait(()=>diff.staged&&!diff.loading);
            const safe=writes===0&&blocked&&source.content===draft&&source.modified&&(await app.api.file(source.path)).content===disk;
            await app.openFile(source.path,true,id);
            return safe&&target.openTabs.length===2&&source.view.state.doc.toString()===draft&&source.view.state.selection.main.head===3&&staged.join(',')==='false,true'&&fixtureErrors.length===0;
        } finally {window.fetch=fetchBefore;}
    }`)
	if err != nil || !result.Value.Bool() {
		t.Fatalf("diff edited or lost its source: %v %v", err, result)
	}
}

func TestDesktopCodeStudioOverlaysKeepWindowOwnership(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result, err := page.Eval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const app=CodeStudioApp,first=csState();first.selectedDir='/workspace/src';first.expandedDirs.add('/workspace/src');
        const host=document.createElement('div');host.style.cssText='width:700px;height:500px';document.body.append(host);
        await app.render(host,'second-window',{});
        const second=app.instances.get('second-window');
        CodeStudioCommandPalette.open(first.windowId);
        await app.openFile('/workspace/README.md',true,second.windowId);
        const input=document.querySelector('.cs-command-palette input');input.value=t('codeStudio.newFolder');input.dispatchEvent(new Event('input'));
        document.querySelector('.cs-cp-item').click();
        const form=document.querySelector('.cs-modal-backdrop form');
        if(!form)return false;
        form.elements.value.value='owned-by-first';form.elements.value.focus();
        csKey(form.elements.value,{key:'Tab',shiftKey:true});
        const focusTrapped=document.activeElement===form.querySelector('[type="submit"]');
        form.requestSubmit();await csWait(()=>first.treeCache['/workspace/src']?.some(f=>f.name==='owned-by-first'));
        CodeStudioCommandPalette.open(first.windowId);
        app.dispose(second.windowId);
        const kept=!!document.querySelector('.cs-command-palette-backdrop');
        app.dispose(first.windowId);
        const cleaned=!document.querySelector('.cs-command-palette-backdrop');
        return focusTrapped&&kept&&cleaned&&fixtureErrors.length===0;
    }`)
	if err != nil || !result.Value.Bool() {
		t.Fatalf("overlay changed windows or leaked: %v %v", err, result)
	}
}

func TestDesktopCodeStudioRenderCostAndWindowWidth(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result, err := page.Eval(`async()=>{
        await csWait(()=>csState()?.terminalSessions.length);
        const app=CodeStudioApp,target=csState(),id=target.windowId,root=csRoot();
        await app.openFile('/workspace/main.go',true,id);
        const tab=target.openTabs[0],tabs=root.querySelector('[data-tabs]'),observer=new MutationObserver(()=>{});
        observer.observe(tabs,{childList:true});
        for(let i=0;i<20;i++)tab.view.dispatch({changes:{from:0,insert:'x'}});
        const bounded=observer.takeRecords().length===1;observer.disconnect();
        root.style.width='600px';await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
        const narrow=innerWidth>1000&&getComputedStyle(root.querySelector('.code-studio-activity-bar')).display==='none'&&root.querySelector('[data-editor]').clientWidth>200;
        target.editorType='textarea';let highlights=0;window.hljs={highlight:(_text,options)=>{if(options.language==='css')highlights++;return {value:'preview'};}};
        await app.openFile('/workspace/styles.css',true,id);
        const fallback=target.openTabs[target.activeTabIndex];
        for(let i=0;i<20;i++)fallback.view.setValue('draft '+i);
        const immediate=fallback.content==='draft 19';
        await new Promise(r=>setTimeout(r,180));
        const bundled=highlights===1;
        fallback.view.setValue('discarded view');await app.command('closeTab',[target.activeTabIndex,true],id);
        await new Promise(r=>setTimeout(r,180));
        return bounded&&narrow&&immediate&&bundled&&highlights===1;
    }`)
	if err != nil || !result.Value.Bool() {
		t.Fatalf("render cost or app-width breakpoint: %v %v", err, result)
	}
}
