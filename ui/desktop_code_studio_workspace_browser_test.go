package ui

import "testing"

// Plan C smoke: the workspace UI against a page-side stub of the plan A workspace API. The chip opens the menu, the
// connect dialog trusts an unknown host key through POST /remote/trust, browses and opens a folder; afterwards scoped requests and the terminal
// socket carry the workspace, the subtitle and terminal tab show user@host, git is gated, a lost connection shows the
// banner and keeps a failed save dirty, and switching back with a dirty tab asks first.
func TestDesktopCodeStudioWorkspaceBrowser(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	errs := func() string { return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str() }
	run := func(name, js string) {
		t.Helper()
		if _, err := page.Eval(js); err != nil {
			t.Fatalf("%s: %v; state: %s; browser errors: %s", name, err, page.MustEval(`()=>JSON.stringify({ws:csState()&&csState().workspaceId,status:csState()&&csState().workspaceStatus,requests:window.wsStub?wsStub.requests.slice(-8):[]})`).Str(), errs())
		}
	}

	run("boot and stub", `async()=>{
		await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);
		const stub=window.wsStub={offline:false,trusted:false,workspaces:[],requests:[],toasts:[],created:null};
		const json=(status,body)=>Promise.resolve(new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}}));
		const real=window.fetch.bind(window);
		window.fetch=(input,init)=>{
			const url=String(input),opts=init||{},headers=opts.headers||{},ws=headers['X-Code-Studio-Workspace']||'';
			stub.requests.push({url,method:opts.method||'GET',ws});
			const path=url.split('?')[0],q=new URLSearchParams(url.split('?')[1]||'');
			if(path==='/api/code-studio/remote/devices')return json(200,{devices:[{id:'dev-1',name:'Build box',host:'10.0.0.5',port:22,username:'dev'}]});
			if(path==='/api/code-studio/remote/trust'&&opts.method==='POST'){const t=JSON.parse(opts.body);if(t.device_id!=='dev-1'||t.fingerprint!=='SHA256:fixture')return json(409,{code:'host_key_mismatch',error:'mismatch'});stub.trusted=true;stub.trustCalls=(stub.trustCalls||0)+1;return json(200,{status:'ok'});}
			if(path==='/api/code-studio/remote/browse'){
				if(!stub.trusted)return json(409,{code:'unknown_host_key',error:'unknown host key',host:'10.0.0.5',key_type:'ssh-ed25519',fingerprint:'SHA256:fixture'});
				stub.trusted=true;
				const p=q.get('path')||'/home/dev';
				return json(200,{path:p,entries:p==='/home/dev'?[{name:'notes.txt',type:'file'},{name:'.cache',type:'directory'},{name:'proj',type:'directory',is_git:true}]:[{name:'src',type:'directory'}]});
			}
			if(path==='/api/code-studio/workspaces'&&opts.method==='POST'){
				const body=JSON.parse(opts.body);stub.created=body;
				const w={id:'ws-1',kind:'ssh',device_id:body.device_id,device_name:'Build box',host:'10.0.0.5',port:22,user:'dev',root:body.root,label:'',last_used_at:new Date().toISOString(),capabilities:{git:false,grep:true,lsp:{go:false,python:false,typescript:false,cpp:false}}};
				stub.workspaces=[w];return json(200,{workspace:w});
			}
			if(path==='/api/code-studio/workspaces')return json(200,{workspaces:[{id:'local',kind:'local'}].concat(stub.workspaces)});
			if(/^\/api\/code-studio\/workspaces\/[^/]+\/connect$/.test(path))return stub.offline?json(502,{code:'ssh_unavailable',error:'connection refused'}):json(200,{workspace:stub.workspaces[0]});
			if(stub.offline&&ws==='ws-1')return json(502,{code:'ssh_unavailable',error:'connection lost'});
			return real(input,init);
		};
		csState().context.notify=p=>{stub.toasts.push(p);};
		const chip=csRoot().querySelector('[data-status-remote]');
		if(!chip||chip.dataset.remoteState!=='local'||csRoot().querySelector('.code-studio-statusbar').firstElementChild!==chip)throw new Error('local chip is not first');
		if(!csRoot().querySelector('[data-ws-switcher]'))throw new Error('switcher missing');
		return true;
	}`)

	run("menu keyboard", `async()=>{
		csRoot().querySelector('[data-status-remote]').click();
		await csWait(()=>document.querySelector('.cs-ws-menu'),3000);
		if(document.activeElement.getAttribute('aria-checked')!=='true')throw new Error('current workspace not focused');
		csKey(document.activeElement,{key:'ArrowDown'});
		if(document.activeElement.dataset.wsAction!=='connect')throw new Error('ArrowDown did not reach connect');
		csKey(document.activeElement,{key:'Escape'});
		await csWait(()=>!document.querySelector('.cs-ws-menu'),2000);
		await csWait(()=>document.activeElement&&document.activeElement.matches('[data-status-remote]'),2000);
		return true;
	}`)

	run("connect dialog", `async()=>{
		csRoot().querySelector('[data-status-remote]').click();
		(await csWait(()=>document.querySelector('.cs-ws-menu [data-ws-action="connect"]'),3000)).click();
		const dialog=await csWait(()=>document.querySelector('.cs-wsd[role="dialog"][aria-modal="true"]'),3000);
		const host=await csWait(()=>dialog.querySelector('[data-wsd-host="dev-1"]'),5000);
		if(!host.textContent.includes('dev@10.0.0.5:22'))throw new Error('host address missing');
		if(!dialog.contains(document.activeElement))throw new Error('focus is not inside the dialog');
		const search=dialog.querySelector('[data-wsd-search]');
		search.value='zzz';search.dispatchEvent(new Event('input',{bubbles:true}));
		await csWait(()=>dialog.querySelector('[data-wsd-no-match]'),2000);
		search.value='';search.dispatchEvent(new Event('input',{bubbles:true}));
		(await csWait(()=>dialog.querySelector('[data-wsd-host="dev-1"]'),2000)).click();
		const trust=await csWait(()=>dialog.querySelector('[data-wsd-trust]'),5000);
		if(!dialog.querySelector('.cs-wsd-trust').textContent.includes('SHA256:fixture'))throw new Error('fingerprint missing');
		if(document.activeElement!==trust)throw new Error('trust button not focused');
		trust.click();
		const proj=await csWait(()=>dialog.querySelector('[data-wsd-entry="/home/dev/proj"]'),5000);
		const rows=[...dialog.querySelectorAll('[data-wsd-entry]')].map(r=>r.dataset.wsdEntry).join(',');
		if(rows!=='/home/dev/proj,/home/dev/.cache,/home/dev/notes.txt')throw new Error('order '+rows);
		if(!proj.querySelector('.cs-wsd-git'))throw new Error('git badge missing');
		if(dialog.querySelector('[data-wsd-entry="/home/dev/notes.txt"]').getAttribute('aria-disabled')!=='true')throw new Error('file not dimmed');
		proj.click();
		await csWait(()=>dialog.querySelector('[data-wsd-target]').textContent==='dev@10.0.0.5:/home/dev/proj',2000);
		csKey(dialog,{key:'Enter'});
		await csWait(()=>!document.querySelector('.cs-wsd')&&csState().workspaceId==='ws-1',8000);
		if(wsStub.trustCalls!==1)throw new Error('trust endpoint calls '+wsStub.trustCalls);
			if(wsStub.created.root!=='/home/dev/proj'||wsStub.created.trust_host_key!=='SHA256:fixture')throw new Error('create body '+JSON.stringify(wsStub.created));
		return true;
	}`)

	run("remote chrome, requests and sockets", `async()=>{
		await csWait(()=>csRoot().querySelectorAll('.cs-tree-item').length>0&&wsStub.requests.some(r=>r.url.startsWith('/api/code-studio/files')&&r.ws==='ws-1'),8000);
		const chip=csRoot().querySelector('[data-status-remote]'),sw=csRoot().querySelector('[data-ws-switcher]');
		if(chip.dataset.remoteState!=='connected'||!chip.textContent.includes('Build box'))throw new Error('chip '+chip.outerHTML.slice(0,160));
		if(!sw.textContent.includes('Build box · ~/proj')||!sw.textContent.includes('dev@10.0.0.5'))throw new Error('switcher '+sw.textContent);
		if(csRoot().closest('.vd-window').querySelector('.vd-window-subtitle').textContent!=='dev@10.0.0.5:~/proj')throw new Error('subtitle');
		if(JSON.parse(localStorage.getItem('aurago.codeStudio.workspace.v1')).id!=='ws-1')throw new Error('active workspace not stored');
		if(wsStub.requests.some(r=>r.url.startsWith('/api/code-studio/workspaces')&&r.ws))throw new Error('workspace API must stay unscoped');
		const socket=await csWait(()=>csState().terminalSessions[0]&&csState().terminalSessions[0].ws,5000);
		if(!socket.url.includes('ws=ws-1'))throw new Error('terminal socket '+socket.url);
		if(!csRoot().querySelector('[data-terminal-tab="0"]').textContent.includes('dev@10.0.0.5'))throw new Error('terminal tab label');
		return true;
	}`)

	run("git gated by capabilities", `async()=>{
		csRoot().querySelector('[data-activity="git"]').click();
		const box=await csWait(()=>csRoot().querySelector('[data-ws-git-missing]'),5000);
		if(!box.textContent.includes('dev@10.0.0.5'))throw new Error('host missing in the git empty state');
		if(wsStub.requests.some(r=>r.url.startsWith('/api/code-studio/git/')&&r.ws==='ws-1'))throw new Error('git was called on a host without git');
		csRoot().querySelector('[data-activity="git"]').click();
		return true;
	}`)

	run("offline save keeps the tab dirty, banner reconnects", `async()=>{
		const id=csState().windowId;
		await CodeStudioApp.openFile('/workspace/main.go',true,id);
		await csWait(()=>csState().openTabs.length===1&&csState().openTabs[0].view,8000);
		csState().openTabs[0].view.dispatch({changes:{from:0,insert:'// remote edit\n'}});
		await csWait(()=>csState().openTabs[0].modified,2000);
		wsStub.offline=true;
		await CodeStudioApp.saveCurrentFile(id);
		await csWait(()=>{const b=csRoot().querySelector('[data-ws-banner]');return b&&!b.hidden&&b.dataset.wsBannerState==='lost';},5000);
		if(!csState().openTabs[0].modified)throw new Error('a failed save cleared the dirty flag');
		if(!wsStub.toasts.length)throw new Error('no toast for the failed save');
		if(csRoot().querySelector('[data-status-remote]').dataset.remoteState!=='lost')throw new Error('chip not lost');
		wsStub.offline=false;
		const button=csRoot().querySelector('[data-ws-banner-action="reconnect"]');
		if(button)button.click();
		await csWait(()=>csRoot().querySelector('[data-ws-banner]').hidden&&csRoot().querySelector('[data-status-remote]').dataset.remoteState==='connected',12000);
		return true;
	}`)

	run("dirty switch asks first", `async()=>{
		const ask=async()=>{csRoot().querySelector('[data-ws-switcher]').click();(await csWait(()=>document.querySelector('.cs-ws-menu [data-ws-id="local"]'),3000)).click();return csWait(()=>document.querySelector('.cs-ws-dirty'),3000);};
		(await ask()).querySelector('[data-choice="cancel"]').click();
		await csWait(()=>!document.querySelector('.cs-ws-dirty'),2000);
		if(csState().workspaceId!=='ws-1')throw new Error('cancel switched anyway');
		(await ask()).querySelector('[data-choice="discard"]').click();
		await csWait(()=>csState().workspaceId==='local'&&csRoot().querySelector('[data-status-remote]').dataset.remoteState==='local',8000);
		const before=wsStub.requests.length;
		await CodeStudioApp.refreshFiles('/workspace',csState().windowId);
		const files=wsStub.requests.slice(before).filter(r=>r.url.startsWith('/api/code-studio/files'));
		if(!files.length||files.some(r=>r.ws))throw new Error('local requests still carry a workspace');
		const sessions=JSON.parse(localStorage.getItem('aurago.codeStudio.workspaceSessions.v1'));
		if(!sessions['ws-1']||!sessions['ws-1'].openTabs.includes('/workspace/main.go'))throw new Error('ws-1 session not kept');
		if(csRoot().closest('.vd-window').querySelector('.vd-window-subtitle').textContent!=='')throw new Error('subtitle not cleared');
		if(fixtureErrors.length)throw new Error('browser errors '+JSON.stringify(fixtureErrors));
		return true;
	}`)
}
