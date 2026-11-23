package ui

import "testing"

// TestDesktopCodeStudioGitWorkbenchBrowser drives the Source Control panel against stubbed git
// endpoints: stage and unstage requests, a staged-only commit, the history list, the commit tab
// and the blame gutter (including a click from blame into the commit tab).
func TestDesktopCodeStudioGitWorkbenchBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	result, err := page.Eval(`async()=>{
        let step='boot';
        const fetchBefore=window.fetch,posts=[];
        try {
            await csWait(()=>csState()?.terminalSessions.length,20000);
            const app=CodeStudioApp,target=csState(),id=target.windowId,root=csRoot();
            const hash='0123456789abcdef0123456789abcdef01234567',zero='0000000000000000000000000000000000000000';
            const changes=[{path:'main.go',status:'M',staged:'true',index:'M',worktree:'M'},{path:'src/util/format.py',status:'??',staged:'false',index:'?',worktree:'?'}];
            const json=value=>Promise.resolve(new Response(JSON.stringify(value),{headers:{'Content-Type':'application/json'}}));
            window.fetch=(url,opts)=>{
                const path=String(url);
                if(opts&&opts.method==='POST'&&path.startsWith('/api/code-studio/git/'))posts.push({path,body:JSON.parse(opts.body||'{}')});
                if(path==='/api/code-studio/git/status')return json({status:'ok',is_repo:true,branch:'main',changes,log:[]});
                if(path.startsWith('/api/code-studio/git/log?'))return json({status:'ok',log:[{hash,short:'0123456',author:'Ada Lovelace',date:'2026-11-20T10:00:00+01:00',message:'Add engine'}]});
                if(path.startsWith('/api/code-studio/git/show?'))return json({status:'ok',hash,short:'0123456',author:'Ada Lovelace',email:'ada@example.com',date:'2026-11-20T10:00:00+01:00',subject:'Add engine',body:'Longer body',files:[{path:'main.go',status:'M'}],diff:'diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n',truncated:false});
                if(path.startsWith('/api/code-studio/git/blame?'))return json({status:'ok',lines:[{line:1,hash,short:'0123456',author:'Ada Lovelace',date:'2026-11-20T10:00:00+01:00',summary:'Add engine'},{line:2,hash:zero,short:'0000000',author:'Not Committed Yet',date:'2026-11-23T10:00:00+01:00',summary:'Version of main.go from main.go'}]});
                if(['/api/code-studio/git/stage','/api/code-studio/git/unstage','/api/code-studio/git/commit'].includes(path))return json({status:'ok',hash:'deadbee'});
                return fetchBefore(url,opts);
            };
            step='open panel';
            await app.openFile('/workspace/main.go',true,id);
            app.command('toggleGitPanel',[],id);
            await csWait(()=>root.querySelector('[data-git-section="staged"] [data-git-file="main.go"]')&&root.querySelector('[data-git-section="changes"] [data-git-file="main.go"]'));
            const bothGroups=!!root.querySelector('[data-git-section="changes"] [data-git-file="src/util/format.py"]')&&!root.querySelector('[data-git-select]');
            step='stage';
            root.querySelector('[data-git-section="changes"] [data-git-stage="src/util/format.py"]').click();
            await csWait(()=>posts.some(p=>p.path.endsWith('/git/stage')));
            step='unstage';
            root.querySelector('[data-git-section="staged"] [data-git-unstage="main.go"]').click();
            await csWait(()=>posts.some(p=>p.path.endsWith('/git/unstage')));
            const bodyOf=suffix=>JSON.stringify(posts.find(p=>p.path.endsWith(suffix)).body);
            const staging=bodyOf('/git/stage')==='{"paths":["src/util/format.py"]}'&&bodyOf('/git/unstage')==='{"paths":["main.go"]}';
            step='commit';
            const message=root.querySelector('[data-git-commit-msg]');message.value='Ship it';message.dispatchEvent(new Event('input'));
            root.querySelector('[data-git-commit]').click();
            await csWait(()=>posts.some(p=>p.path.endsWith('/git/commit')));
            const commitBody=posts.find(p=>p.path.endsWith('/git/commit')).body;
            const stagedOnly=commitBody.staged_only===true&&commitBody.add_all===false&&commitBody.message==='Ship it'&&!document.querySelector('.cs-modal-backdrop');
            step='history';
            await csWait(()=>root.querySelector('[data-git-commit-hash]'));
            const history=root.querySelectorAll('[data-git-commit-hash]').length===1&&root.querySelector('[data-git-commit-hash] .cs-git-hash').textContent==='0123456';
            step='commit tab';
            root.querySelector('[data-git-commit-hash]').click();
            await csWait(()=>root.querySelector('[data-git-commit-view] .cs-diff-line.added'));
            const view=root.querySelector('[data-git-commit-view]');
            const commitTab=target.openTabs[target.activeTabIndex].kind==='commit'&&view.textContent.includes('Add engine')&&view.textContent.includes('Longer body');
            const saved=JSON.parse(localStorage.getItem('aurago.codeStudio.state.v1')).openTabs;
            const commitIsVirtual=app.isVirtualTab(target.openTabs[target.activeTabIndex])&&saved.length===1&&saved[0].path==='/workspace/main.go';
            step='blame';
            await app.openFile('/workspace/main.go',true,id);
            await app.command('toggleGitBlame',[],id);
            await csWait(()=>root.querySelectorAll('.cs-blame-gutter .cs-blame-entry.first').length>=2);
            const entries=[...root.querySelectorAll('.cs-blame-gutter .cs-blame-entry.first')];
            const blame=entries[0].textContent.includes('0123456')&&entries[0].title==='Add engine'&&entries[1].textContent===t('codeStudio.gitNotCommitted');
            step='blame click';
            const firstEntry=root.querySelector('.cs-blame-gutter .cs-blame-entry.first'),box=firstEntry.getBoundingClientRect();
            firstEntry.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true,clientX:box.left+box.width/2,clientY:box.top+box.height/2}));
            await csWait(()=>target.openTabs[target.activeTabIndex]?.kind==='commit');
            step='blame hidden on edit';
            await app.openFile('/workspace/main.go',true,id);
            const mainTab=target.openTabs.find(item=>item.path==='/workspace/main.go');
            await csWait(()=>mainTab.view&&root.querySelectorAll('.cs-blame-gutter .cs-blame-entry').length>0);
            mainTab.view.dispatch({changes:{from:0,insert:'x'}});
            await csWait(()=>mainTab.modified&&root.querySelectorAll('.cs-blame-entry').length===0);
            const blameHidden=root.querySelectorAll('.cs-blame-entry').length===0;
            return {bothGroups,staging,stagedOnly,history,commitTab,commitIsVirtual,blame,blameClick:true,blameHidden,errors:fixtureErrors};
        } catch (err) {
            return {step,error:String(err&&err.message||err),errors:fixtureErrors};
        } finally {window.fetch=fetchBefore;}
    }`)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"bothGroups", "staging", "stagedOnly", "history", "commitTab", "commitIsVirtual", "blame", "blameClick", "blameHidden"} {
		if !result.Value.Get(key).Bool() {
			t.Fatalf("Git workbench check %q failed: %s", key, result.Value.JSON("", "  "))
		}
	}
	if len(result.Value.Get("errors").Arr()) != 0 {
		t.Fatalf("browser errors: %s", result.Value.JSON("", "  "))
	}
}
