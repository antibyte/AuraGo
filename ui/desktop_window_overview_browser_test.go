package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func desktopWindowOverviewIDs(page *rod.Page) []string {
	page.MustWait(`()=>!!document.querySelector('#vd-window-overview [data-overview-window]')`)
	raw := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('#vd-window-overview [data-overview-window]')].map(node=>node.dataset.overviewWindow))`).Str()
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		panic(err)
	}
	return ids
}

func desktopWindowOverviewActivate(page *rod.Page, id string) {
	page.MustEval(`id=>{
		const button=[...document.querySelectorAll('[data-window-activate]')].find(node=>node.dataset.windowActivate===id);
		if(!button)throw new Error('missing activate control for '+id);
		button.click();
	}`, id)
}

func TestDesktopWindowOverviewGroupingAndSpacesBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
		const a=auditTest,manager=window.AuraDesktopWorkspaceManager;
		a.state.bootstrap.settings['appearance.theme']='standard';
		a.state.bootstrap.settings['windows.animations']='true';
		const before=manager.snapshot(),remote=before.layout.workspaces.find(item=>item.id!==String(a.state.activeSpaceId));
		if(!remote)throw new Error('second workspace fixture is missing');
		window.overviewLocalSpace=String(a.state.activeSpaceId);
		window.overviewRemoteSpace=remote.id;
		manager.adopt({layout:{...before.layout,revision:before.layout.revision+1,workspaces:before.layout.workspaces.map(item=>item.id===remote.id?{...item,name:'Research desk'}:item)},settings:before.settings});
		a.refreshSpacesForViewport();
		a.state.bootstrap.settings['appearance.dock_pins']=JSON.stringify(['calculator','files','writer']);
		a.applyDesktopSettings();
		for(let i=0;i<3;i++){a.openApp('calculator',{forceNew:true});await new Promise(resolve=>requestAnimationFrame(resolve));}
		window.overviewGroupWindows=[...a.state.windows.values()].filter(win=>win.appId==='calculator').map(win=>win.id);
		a.openApp('editor',{forceNew:true});
		window.overviewRemoteWindow=[...a.state.windows.values()].find(win=>win.appId==='editor').id;
		a.moveWindowToSpace(overviewRemoteWindow,overviewRemoteSpace);
	}`)
	page.MustWait(`()=>window.overviewGroupWindows?.length===3&&!!document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]')`)
	page.MustEval(`async()=>{
		const a=auditTest,group=document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]');
		window.overviewGroupButton=group;
		const pins=[...document.querySelectorAll('#vd-taskbar-apps [data-pin-app-id]')].map(node=>node.dataset.pinAppId);
		if(group.dataset.windowCount!=='3')throw new Error('taskbar app group did not report all three windows');
		if(pins.slice(0,3).join(',')!=='calculator,files,writer')throw new Error('pinned app order was not retained: '+pins.join(','));
		a.applyDesktopSettings();
		if(document.querySelectorAll('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]').length!==1)throw new Error('taskbar contains duplicate app groups');
		if(document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]')!==overviewGroupButton)throw new Error('taskbar reconciliation replaced the live group button');
		const hidden=a.state.windows.get(overviewGroupWindows[1]);
		hidden.element.querySelector('[data-action="minimize"]').click();
		window.overviewMinimizedID=hidden.id;
	}`)
	page.MustWait(`()=>auditTest.state.windows.get(overviewMinimizedID)?.minimizing===true`)
	page.MustEval(`()=>overviewGroupButton.click()`)
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelector('#vd-window-overview')?.dataset.overviewMode==='app-group'`)
	ids := desktopWindowOverviewIDs(page)
	if len(ids) != 3 {
		t.Fatalf("app group overview lists %d windows, want 3: %v", len(ids), ids)
	}
	if !page.MustEval(`()=>{
		const id=overviewMinimizedID,card=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===id);
		return !!card&&(card.dataset.overviewMinimized==='true'||!!card.querySelector('[data-overview-minimized="true"]'));
	}`).Bool() {
		t.Fatal("minimized instance is not marked in its app-group overview card")
	}
	page.MustEval(`()=>{
		const id=overviewMinimizedID,button=[...document.querySelectorAll('[data-window-activate]')].find(node=>node.dataset.windowActivate===id);
		if(!button)throw new Error('minimized window has no activation control');
		button.click();
	}`)
	page.MustWait(`()=>auditTest.state.activeWindowId===overviewMinimizedID&&auditTest.state.windows.get(overviewMinimizedID)?.element.style.display!=='none'&&!AuraDesktopWindowOverview.isOpen()`)
	// The minimize completion callback fires after its theme fallback. Restoring during that
	// interval must cancel the old callback rather than hiding the now-active window again.
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,800))`)
	if !page.MustEval(`()=>{
		const item=auditTest.state.windows.get(overviewMinimizedID);
		return !!item&&item.element.style.display!=='none'&&!item.minimizing&&auditTest.state.activeWindowId===overviewMinimizedID;
	}`).Bool() {
		t.Fatal("the old minimize completion hid or deactivated a window restored from the overview")
	}
	// Minimize, open the group and activate within one browser turn. The active minimize
	// animation callback must see the activation's revision fence when its timer completes.
	page.MustEval(`()=>{
		const id=overviewGroupWindows[2],item=auditTest.state.windows.get(id);
		item.element.querySelector('[data-action="minimize"]').click();
		window.overviewRaceID=id;
		window.overviewRaceStarted=item.minimizing;
		AuraDesktopWindowOverview.open({mode:'app-group',appId:'calculator',scope:'current',opener:overviewGroupButton});
		const card=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===id);
		window.overviewRaceCardMinimized=!!card&&(card.dataset.overviewMinimized==='true'||!!card.querySelector('[data-overview-minimized="true"]'));
		const activate=[...document.querySelectorAll('[data-window-activate]')].find(node=>node.dataset.windowActivate===id);
		if(!activate)throw new Error('minimized animation target has no activation control');
		activate.click();
	}`)
	page.MustWait(`()=>overviewRaceStarted&&overviewRaceCardMinimized&&auditTest.state.activeWindowId===overviewRaceID&&auditTest.state.windows.get(overviewRaceID)?.element.style.display!=='none'&&!AuraDesktopWindowOverview.isOpen()`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,800))`)
	if !page.MustEval(`()=>{
		const item=auditTest.state.windows.get(overviewRaceID);
		return !!item&&item.element.style.display!=='none'&&!item.minimizing&&auditTest.state.activeWindowId===overviewRaceID;
	}`).Bool() {
		t.Fatal("minimize animation completion hid the immediately activated overview window")
	}

	page.MustEval(`()=>{
		const group=document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]');
		group.focus();
		AuraDesktopWindowOverview.open({mode:'switcher',opener:group});
	}`)
	page.MustWait(`()=>!!document.querySelector('#vd-window-overview [data-overview-scope="current"]')&&document.querySelectorAll('#vd-window-overview [data-overview-window]').length===3`)
	if page.MustEval(`()=>[...document.querySelectorAll('[data-overview-window]')].some(node=>node.dataset.overviewWindow===overviewRemoteWindow)`).Bool() {
		t.Fatal("current scope included a window from another workspace")
	}
	page.MustEval(`()=>{
		const all=[...document.querySelectorAll('#vd-window-overview [data-overview-scope]')].find(node=>node.dataset.overviewScope==='all');
		if(!all)throw new Error('all-workspaces scope control is missing');
		all.click();
	}`)
	page.MustWait(`()=>[...document.querySelectorAll('#vd-window-overview [data-overview-window]')].some(node=>node.dataset.overviewWindow===overviewRemoteWindow)`)
	if !page.MustEval(`()=>{
		const card=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===overviewRemoteWindow);
		return !!card&&card.textContent.includes('Research desk')&&!!card.querySelector('[data-overview-workspace-label]');
	}`).Bool() {
		t.Fatal("all scope did not label the remote workspace in its window card")
	}
	captureEverydayPanel(t, page, "window-overview-standard-all-spaces")
	desktopWindowOverviewActivate(page, page.MustEval(`()=>overviewRemoteWindow`).Str())
	page.MustWait(`()=>auditTest.state.activeSpaceId===overviewRemoteSpace&&auditTest.state.activeWindowId===overviewRemoteWindow&&!AuraDesktopWindowOverview.isOpen()`)
	if !page.MustEval(`()=>{
		const win=auditTest.state.windows.get(overviewRemoteWindow);
		return !!win&&win.element.style.display!=='none'&&!win.element.classList.contains('vd-space-hidden');
	}`).Bool() {
		t.Fatal("activating a remote workspace window did not restore its visibility")
	}
	page.MustEval(`()=>auditTest.switchSpace(overviewLocalSpace)`)
	page.MustWait(`()=>auditTest.state.activeSpaceId===overviewLocalSpace`)
	page.MustSetViewport(640, 450, 2, false)
	page.MustEval(`async()=>{
		await document.fonts.ready;
		await new Promise(resolve=>requestAnimationFrame(resolve));
		await new Promise(resolve=>requestAnimationFrame(resolve));
	}`)
	page.MustEval(`()=>{
		const opener=document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]');
		if(!document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]'))throw new Error('compact taskbar omitted the visible remote editor');
		AuraDesktopWindowOverview.open({mode:'switcher',scope:'current',opener});
	}`)
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelectorAll('#vd-window-overview [data-overview-window]').length===3`)
	if page.MustEval(`()=>[...document.querySelectorAll('#vd-window-overview [data-overview-window]')].some(node=>node.dataset.overviewWindow===overviewRemoteWindow)`).Bool() {
		t.Fatal("compact Current scope included a window from a different logical workspace")
	}
	page.MustEval(`()=>document.querySelector('#vd-window-overview [data-overview-scope="all"]').click()`)
	page.MustWait(`()=>[...document.querySelectorAll('#vd-window-overview [data-overview-window]')].some(node=>node.dataset.overviewWindow===overviewRemoteWindow)`)
	desktopWindowOverviewActivate(page, page.MustEval(`()=>overviewRemoteWindow`).Str())
	page.MustWait(`()=>auditTest.state.activeSpaceId===overviewRemoteSpace&&auditTest.state.activeWindowId===overviewRemoteWindow&&!AuraDesktopWindowOverview.isOpen()`)
	if !page.MustEval(`()=>!!document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]')&&!auditTest.state.windows.get(overviewRemoteWindow).element.classList.contains('vd-space-hidden')`).Bool() {
		t.Fatal("compact activation did not select the remote logical workspace while keeping its window visible")
	}
	page.MustSetViewport(1280, 900, 1, false)
	page.MustEval(`async()=>{
		await document.fonts.ready;
		await new Promise(resolve=>requestAnimationFrame(resolve));
		await new Promise(resolve=>requestAnimationFrame(resolve));
	}`)
	page.MustWait(`()=>innerWidth===1280&&auditTest.state.activeSpaceId===overviewRemoteSpace&&!auditTest.state.windows.get(overviewRemoteWindow).element.classList.contains('vd-space-hidden')&&auditTest.state.windows.get(overviewGroupWindows[0]).element.classList.contains('vd-space-hidden')`)
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatalf("window overview browser errors: %s", got)
	}
}

func TestDesktopWindowOverviewCloseLifecycleBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
		const a=auditTest;
		a.state.bootstrap.settings['appearance.theme']='standard';
		a.state.bootstrap.settings['windows.animations']='false';
		a.state.bootstrap.settings['appearance.dock_pins']=JSON.stringify(['editor','files','writer']);
		a.applyDesktopSettings();
		for(let i=0;i<4;i++){a.openApp('editor',{forceNew:true});await new Promise(resolve=>requestAnimationFrame(resolve));}
		window.overviewInitialEditorIDs=[...a.state.windows.values()].filter(win=>win.appId==='editor').map(win=>win.id);
	}`)
	page.MustWait(`()=>overviewInitialEditorIDs?.length===4&&!!document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]')`)
	page.MustEval(`async()=>{
		const button=document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]');
		button.focus();
		AuraDesktopWindowOverview.open({mode:'app-group',appId:'editor',scope:'current',opener:button});
		const cancelID=overviewInitialEditorIDs[0],cancelWindow=auditTest.state.windows.get(cancelID);
		cancelWindow.beforeClose=async()=>{window.overviewCancelResolved=true;return false;};
		window.overviewCancelID=cancelID;
	}`)
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&!!document.querySelector('[data-window-close]')`)
	page.MustEval(`()=>{
		[...document.querySelectorAll('[data-window-close]')].find(node=>node.dataset.windowClose===overviewCancelID).click();
	}`)
	page.MustWait(`()=>window.overviewCancelResolved===true&&auditTest.state.windows.has(overviewCancelID)`)
	if !page.MustEval(`()=>{
		const panel=document.querySelector('#vd-window-overview');
		return AuraDesktopWindowOverview.isOpen()&&panel?.contains(document.activeElement)&&!![...panel.querySelectorAll('[data-window-close]')].find(node=>node.dataset.windowClose===overviewCancelID);
	}`).Bool() {
		t.Fatal("cancelled close did not preserve the window, overview and usable focus")
	}

	page.MustEval(`()=>{
		const acceptID=overviewInitialEditorIDs[1],item=auditTest.state.windows.get(acceptID);
		item.beforeClose=async()=>true;
		window.overviewAcceptID=acceptID;
		[...document.querySelectorAll('[data-window-close]')].find(node=>node.dataset.windowClose===acceptID).click();
	}`)
	page.MustWait(`()=>!auditTest.state.windows.has(overviewAcceptID)`)
	if !page.MustEval(`()=>{
		const panel=document.querySelector('#vd-window-overview');
		return document.activeElement!==document.body&&(!AuraDesktopWindowOverview.isOpen()||panel?.contains(document.activeElement));
	}`).Bool() {
		t.Fatal("accepted close left keyboard focus without a live overview or opener")
	}

	page.MustEval(`()=>{
		AuraDesktopWindowOverview.open({mode:'app-group',appId:'editor',scope:'current',opener:document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]')});
		window.overviewBatchInitial=[...auditTest.state.windows.values()].filter(win=>win.appId==='editor').map(win=>win.id);
		window.overviewBatchCalls=[];
		window.overviewBatchNewID='';
		overviewBatchInitial.forEach(id=>{
			const item=auditTest.state.windows.get(id);
			item.beforeClose=async()=>{
				overviewBatchCalls.push(id);
				if(overviewBatchCalls.length===1){
					auditTest.openApp('editor',{forceNew:true});
					overviewBatchNewID=[...auditTest.state.windows.values()].find(win=>win.appId==='editor'&&!overviewBatchInitial.includes(win.id)).id;
					return true;
				}
				if(overviewBatchCalls.length===2)return false;
				return true;
			};
		});
	}`)
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&!!document.querySelector('[data-overview-close-app]')`)
	page.MustEval(`()=>document.querySelector('[data-overview-close-app]').click()`)
	page.MustWait(`()=>overviewBatchCalls.length>=2&&!!overviewBatchNewID`)
	if !page.MustEval(`()=>{
		const a=auditTest,calls=overviewBatchCalls,initial=overviewBatchInitial;
		return calls.length===2&&calls.every(id=>initial.includes(id))
			&&!a.state.windows.has(calls[0])&&a.state.windows.has(calls[1])
			&&initial.filter(id=>id!==calls[0]).every(id=>a.state.windows.has(id))
			&&a.state.windows.has(overviewBatchNewID)&&!calls.includes(overviewBatchNewID);
	}`).Bool() {
		t.Fatalf("close-all did not stop at cancellation or closed a newly opened window: %s", page.MustEval(`()=>JSON.stringify({calls:overviewBatchCalls,initial:overviewBatchInitial,newID:overviewBatchNewID,live:[...auditTest.state.windows.keys()]})`).Str())
	}

	page.MustEval(`()=>{
		const pendingID=[...auditTest.state.windows.values()].find(win=>win.appId==='editor').id,item=auditTest.state.windows.get(pendingID);
		window.overviewPendingID=pendingID;
		window.overviewPendingPromise=new Promise(resolve=>window.resolveOverviewPending=resolve);
		item.beforeClose=()=>{window.overviewPendingStarted=true;return overviewPendingPromise.then(value=>{window.overviewPendingResolved=true;return value;});};
		AuraDesktopWindowOverview.open({mode:'app-group',appId:'editor',scope:'current',opener:document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]')});
		[...document.querySelectorAll('[data-window-close]')].find(node=>node.dataset.windowClose===overviewPendingID).click();
	}`)
	page.MustWait(`()=>window.overviewPendingStarted===true`)
	page.MustEval(`()=>document.querySelector('[data-overview-dismiss]').click()`)
	page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()`)
	page.MustEval(`()=>AuraDesktopWindowOverview.open({mode:'app-group',appId:'editor',scope:'current',opener:document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]')})`)
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&!!document.querySelector('[data-overview-window]')`)
	page.MustEval(`()=>resolveOverviewPending(false)`)
	page.MustWait(`()=>window.overviewPendingResolved===true`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,0))`)
	if !page.MustEval(`()=>AuraDesktopWindowOverview.isOpen()&&[...document.querySelectorAll('[data-overview-window]')].some(node=>node.dataset.overviewWindow===overviewPendingID)&&document.querySelector('#vd-window-overview').dataset.overviewMode==='app-group'`).Bool() {
		t.Fatal("a late cancelled close changed the newly reopened overview")
	}
	if !page.MustEval(`()=>document.querySelector('#vd-window-overview')?.contains(document.activeElement)`).Bool() {
		t.Fatal("a late cancelled close moved focus outside the reopened overview")
	}
	page.MustEval(`()=>{
		const opener=document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="editor"]');
		opener.focus();
		window.overviewModalOpener=opener;
		AuraDesktopWindowOverview.open({mode:'app-group',appId:'editor',scope:'current',opener});
		window.overviewModalBeforeCount=document.querySelectorAll('#vd-window-overview [data-overview-window]').length;
		window.overviewModalResult='pending';
		auditTest.modalDialog({title:'Overview focus fixture',message:'Modal focus must remain in front.',input:true,value:'still open'}).then(value=>overviewModalResult=value);
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal-input')&&document.activeElement===document.querySelector('.vd-modal-input')`)
	page.MustEval(`async()=>{
		await new Promise(resolve=>requestAnimationFrame(resolve));
		auditTest.openApp('editor',{forceNew:true});
	}`)
	page.MustWait(`()=>document.querySelectorAll('#vd-window-overview [data-overview-window]').length===overviewModalBeforeCount+1&&document.activeElement===document.querySelector('.vd-modal-input')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
	page.MustWait(`()=>overviewModalResult===null&&!document.querySelector('.vd-modal')&&AuraDesktopWindowOverview.isOpen()`)
	page.MustEval(`()=>AuraDesktopWindowOverview.finish()`)
	page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()&&!document.querySelector('.vd-modal-backdrop')&&document.activeElement===overviewModalOpener`)
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatalf("window overview close-flow errors: %s", got)
	}
}

func TestDesktopWindowOverviewPreviewAndAccessibilityBrowser(t *testing.T) {
	var previewRequests atomic.Int32
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/overview-probe": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			previewRequests.Add(1)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("invalid-image-fixture"))
		}),
	})
	page.MustEval(`async()=>{
		const a=auditTest;
		await Promise.all([
			window.AuraDesktopModules.loadAppAssets('calculator'),
			window.AuraDesktopModules.loadAppAssets('editor')
		]);
		a.state.bootstrap.settings['windows.animations']='false';
		a.state.bootstrap.settings['appearance.theme']='standard';
		a.state.bootstrap.settings['appearance.dock_pins']=JSON.stringify(['calculator','files','writer']);
		a.applyDesktopSettings();
		a.openApp('calculator',{forceNew:true});
		window.overviewSafeWindow=[...a.state.windows.values()].find(win=>win.appId==='calculator');
		await new Promise(resolve=>requestAnimationFrame(resolve));
		a.openApp('calculator',{forceNew:true});
		await new Promise(resolve=>requestAnimationFrame(resolve));
		a.openApp('editor',{forceNew:true});
		window.overviewSensitiveWindow=[...a.state.windows.values()].find(win=>win.appId==='editor');
	}`)
	page.MustWait(`()=>!!overviewSafeWindow?.element.querySelector('.vd-calc')&&!!overviewSensitiveWindow?.element.querySelector('.vd-editor textarea')&&!overviewSensitiveWindow.element.querySelector('.vd-editor textarea').readOnly`)
	page.MustEval(`()=>{
		const content=overviewSafeWindow.element.querySelector('.vd-window-content');
		content.innerHTML='<div data-preview-fixture="remove" onclick="window.previewExecuted=true"><a href="/api/desktop/no-preview-link">Safe preview</a><img alt="loaded only by its owner" src="/api/desktop/overview-probe"></div>';
		window.overviewUnsafeImage=content.querySelector('img');
		const sensitiveContent=overviewSensitiveWindow.element.querySelector('.vd-window-content');
		sensitiveContent.innerHTML='<iframe title="owner frame" src="/api/desktop/overview-probe?owner=frame"></iframe>';
		window.overviewSensitiveFrame=sensitiveContent.querySelector('iframe');
	}`)
	resourceDiagnostic := page.MustEval(`()=>new Promise(resolve=>{
		const deadline=performance.now()+8000;
		function inspect(){
			const image=window.overviewUnsafeImage,frame=window.overviewSensitiveFrame;
			const resources=performance.getEntriesByType('resource').map(entry=>entry.name).filter(name=>name.includes('/api/desktop/overview-probe'));
			const imageConnected=!!image&&image.isConnected,imageComplete=!!image&&image.complete;
			const frameConnected=!!frame&&frame.isConnected;
			const imageRequest=resources.some(name=>!name.includes('?owner=frame'));
			const frameRequest=resources.some(name=>name.includes('?owner=frame'));
			const result={ready:imageConnected&&imageComplete&&frameConnected&&imageRequest&&frameRequest,imageConnected,imageComplete,frameConnected,imageRequest,frameRequest,resources};
			if(result.ready||performance.now()>=deadline){resolve(JSON.stringify(result));return;}
			setTimeout(inspect,25);
		}
		inspect();
	})`).Str()
	var resourceState struct {
		Ready          bool     `json:"ready"`
		ImageConnected bool     `json:"imageConnected"`
		ImageComplete  bool     `json:"imageComplete"`
		FrameConnected bool     `json:"frameConnected"`
		ImageRequest   bool     `json:"imageRequest"`
		FrameRequest   bool     `json:"frameRequest"`
		Resources      []string `json:"resources"`
	}
	if err := json.Unmarshal([]byte(resourceDiagnostic), &resourceState); err != nil {
		t.Fatalf("could not decode owner resource diagnostic: %v (%s)", err, resourceDiagnostic)
	}
	if !resourceState.Ready {
		t.Fatalf("owner resource fixture did not load within 8s: %s; server requests=%d", resourceDiagnostic, previewRequests.Load())
	}
	if previewRequests.Load() != 2 {
		t.Fatalf("owner windows started %d probe requests, want two", previewRequests.Load())
	}
	page.MustEval(`()=>AuraDesktopWindowOverview.open({mode:'switcher',scope:'all',opener:document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]')})`)
	page.MustWait(`()=>{
		const safe=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===overviewSafeWindow.id);
		const sensitive=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===overviewSensitiveWindow.id);
		return AuraDesktopWindowOverview.isOpen()&&!!safe?.querySelector('.vd-window-overview-preview.is-ready')
			&&!!sensitive?.querySelector('.vd-window-overview-preview.is-ready');
	}`)
	if !page.MustEval(`()=>{
		const safe=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===overviewSafeWindow.id);
		const safePreview=safe?.querySelector('.vd-window-overview-preview');
		const sensitive=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===overviewSensitiveWindow.id);
		const sensitivePreview=sensitive?.querySelector('.vd-window-overview-preview');
		return !!safePreview&&!safePreview.querySelector('img[src],iframe,script,[onclick],[data-preview-fixture],[href],[src]')
			&&!!sensitivePreview&&sensitivePreview.textContent.includes(window.t('desktop.taskbar_thumb_live'))
			&&!sensitivePreview.querySelector('iframe,textarea,input,canvas,video,audio');
	}`).Bool() {
		t.Fatal("overview preview cloned unsafe content instead of sanitizing it or showing the sensitive fallback")
	}
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,300))`)
	if previewRequests.Load() != 2 {
		t.Fatalf("opening the overview triggered %d extra preview resource requests", previewRequests.Load()-2)
	}

	page.MustEval(`()=>{AuraDesktopWindowOverview.finish();}`)
	page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()`)
	page.MustSetViewport(640, 450, 2, false)
	page.MustEval(`async()=>{
		await document.fonts.ready;
		await new Promise(resolve=>requestAnimationFrame(resolve));
		await new Promise(resolve=>requestAnimationFrame(resolve));
	}`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "forced-colors", Value: "active"}, {Name: "prefers-reduced-motion", Value: "reduce"},
	}}).Call(page); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`theme=>{
			const a=auditTest;
			a.state.bootstrap.settings['appearance.theme']=theme;
			a.state.bootstrap.settings['windows.animations']='false';
			a.applyDesktopSettings();
		}`, theme)
		page.MustWait(`theme=>document.body.dataset.theme===theme`, theme)
		page.MustEval(`async()=>{
			await document.fonts.ready;
			await new Promise(resolve=>requestAnimationFrame(resolve));
			await new Promise(resolve=>requestAnimationFrame(resolve));
		}`)
		page.MustEval(`()=>{
			const selector=document.body.dataset.theme==='fruity'?'.vd-dock-button[data-app-id="calculator"]':'.vd-task-button[data-app-id="calculator"]';
			const opener=document.querySelector(selector);
			if(!opener)throw new Error('visible app overview launcher is missing');
			opener.focus();
			window.overviewA11yOpener=opener;
		}`)
		modifier := input.ControlLeft
		if theme == "fruity" {
			modifier = input.MetaLeft
		}
		if err := page.Keyboard.Press(modifier); err != nil {
			t.Fatal(err)
		}
		if err := page.Keyboard.Press(input.Tab); err != nil {
			t.Fatal(err)
		}
		if err := page.Keyboard.Release(input.Tab); err != nil {
			t.Fatal(err)
		}
		page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelector('#vd-window-overview')?.dataset.overviewMode==='switcher'`)
		page.MustEval(`()=>{
			const selected=document.querySelector('#vd-window-overview [data-window-activate][aria-pressed="true"]');
			if(!selected)throw new Error('Ctrl/Meta+Tab did not select a switcher window');
			window.overviewSwitchSelected=selected.dataset.windowActivate;
		}`)
		if err := page.Keyboard.Release(modifier); err != nil {
			t.Fatal(err)
		}
		page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()&&auditTest.state.activeWindowId===overviewSwitchSelected`)
		// Ctrl/Meta+Alt+W opens a persistent switcher for focus trapping and Escape checks.
		page.MustEval(`()=>overviewA11yOpener.focus()`)
		page.KeyActions().Press(modifier).Press(input.AltLeft).Type(input.KeyW).Release(input.AltLeft).Release(modifier).MustDo()
		page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelector('#vd-window-overview')?.dataset.overviewMode==='switcher'`)
		page.MustEval(`async()=>{
			await document.fonts.ready;
			await new Promise(resolve=>requestAnimationFrame(resolve));
			await new Promise(resolve=>requestAnimationFrame(resolve));
		}`)
		captureEverydayPanel(t, page, fmt.Sprintf("window-overview-%s-640x450-dpr2-forced-colors", theme))
		if !page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-window-overview');
			const state=element=>{
				if(!element)return null;
				const style=getComputedStyle(element),rect=element.getBoundingClientRect();
				return {text:(element.textContent||'').trim(),display:style.display,visibility:style.visibility,opacity:style.opacity,color:style.color,
					textFill:style.getPropertyValue('-webkit-text-fill-color'),rect:{left:rect.left,right:rect.right,top:rect.top,bottom:rect.bottom,width:rect.width,height:rect.height}};
			};
			const visible=item=>!!item&&!!item.text&&item.display!=='none'&&item.visibility!=='hidden'&&Number(item.opacity)>0
				&&item.rect.width>=24&&item.rect.height>=9&&item.color!=='transparent'&&item.textFill!=='transparent';
			return !!dialog&&visible(state(dialog.querySelector('#vd-window-overview-title')))
				&&visible(state(dialog.querySelector('.vd-window-overview-card-title')))
				&&visible(state(dialog.querySelector('.vd-window-overview-card-meta')));
		}`).Bool() {
			diagnostic := page.MustEval(`()=>{
				const dialog=document.querySelector('#vd-window-overview');
				const state=element=>{
					if(!element)return null;
					const style=getComputedStyle(element),rect=element.getBoundingClientRect();
					return {text:(element.textContent||'').trim(),display:style.display,visibility:style.visibility,opacity:style.opacity,color:style.color,
						textFill:style.getPropertyValue('-webkit-text-fill-color'),rect:{left:rect.left,right:rect.right,top:rect.top,bottom:rect.bottom,width:rect.width,height:rect.height}};
				};
				return JSON.stringify({heading:state(dialog?.querySelector('#vd-window-overview-title')),title:state(dialog?.querySelector('.vd-window-overview-card-title')),
					meta:state(dialog?.querySelector('.vd-window-overview-card-meta')),viewport:{width:innerWidth,height:innerHeight},theme:document.body.dataset.theme});
			}`).Str()
			t.Fatalf("%s overview text is hidden or clipped after fonts/layout settled: %s", theme, diagnostic)
		}
		page.MustEval(`()=>{
			auditTest.state.bootstrap.settings['accessibility.text_size']='150';
			auditTest.applyDesktopSettings();
		}`)
		page.MustWait(`()=>document.body.dataset.textSize==='150'`)
		page.MustEval(`async()=>{
			await document.fonts.ready;
			await new Promise(resolve=>requestAnimationFrame(resolve));
			await new Promise(resolve=>requestAnimationFrame(resolve));
		}`)
		if !page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-window-overview');
			const heading=dialog?.querySelector('#vd-window-overview-title');
			const close=dialog?.querySelector('button[data-overview-dismiss]');
			return !!heading&&!!close&&parseFloat(getComputedStyle(heading).fontSize)>=20.9&&parseFloat(getComputedStyle(close).fontSize)>=20.9;
		}`).Bool() {
			diagnostic := page.MustEval(`()=>JSON.stringify({heading:getComputedStyle(document.querySelector('#vd-window-overview-title')).fontSize,
				close:getComputedStyle(document.querySelector('#vd-window-overview button[data-overview-dismiss]')).fontSize,textSize:document.body.dataset.textSize})`).Str()
			t.Fatalf("%s overview heading/close control did not reach 150%% shell text size: %s", theme, diagnostic)
		}
		page.MustEval(`()=>{
			auditTest.state.bootstrap.settings['accessibility.text_size']='100';
			auditTest.applyDesktopSettings();
		}`)
		page.MustWait(`()=>document.body.dataset.textSize==='100'`)
		if !page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-window-overview'),rect=dialog.getBoundingClientRect();
			const backdrop=dialog.querySelector('.vd-window-overview-backdrop');
			return dialog.getAttribute('role')==='dialog'&&dialog.getAttribute('aria-modal')==='true'
				&&dialog.contains(document.activeElement)&&matchMedia('(forced-colors: active)').matches
				&&matchMedia('(prefers-reduced-motion: reduce)').matches&&rect.width>0&&rect.height>0
				&&!!backdrop&&getComputedStyle(backdrop).backdropFilter==='none'
				&&rect.left>=0&&rect.top>=0&&rect.right<=innerWidth+1&&rect.bottom<=innerHeight+1;
		}`).Bool() {
			t.Fatalf("%s switcher is not usable within the forced-colors 640x450 viewport", theme)
		}
		page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-window-overview');
			const controls=[...dialog.querySelectorAll('button:not(:disabled)')].filter(node=>node.getClientRects().length>0&&!node.closest('[hidden]'));
			if(!controls.length)throw new Error('overview has no keyboard-operable controls');
			controls.at(-1).focus();
		}`)
		if err := page.Keyboard.Press(input.Tab); err != nil {
			t.Fatal(err)
		}
		if err := page.Keyboard.Release(input.Tab); err != nil {
			t.Fatal(err)
		}
		if !page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-window-overview');
			const first=[...dialog.querySelectorAll('button:not(:disabled)')].find(node=>node.getClientRects().length>0&&!node.closest('[hidden]'));
			return !!first&&document.activeElement===first;
		}`).Bool() {
			t.Fatalf("Tab did not wrap focus inside the %s window overview", theme)
		}
		if err := page.Keyboard.Press(input.Escape); err != nil {
			t.Fatal(err)
		}
		if err := page.Keyboard.Release(input.Escape); err != nil {
			t.Fatal(err)
		}
		if err := page.Keyboard.Release(modifier); err != nil {
			t.Fatal(err)
		}
		page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()&&document.activeElement===overviewA11yOpener`)
		// The grouped taskbar entry remains a visible route to the same overview after Escape.
		page.MustEval(`()=>overviewA11yOpener.click()`)
		page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelector('#vd-window-overview')?.dataset.overviewMode==='app-group'`)
		page.MustEval(`()=>AuraDesktopWindowOverview.finish()`)
		page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()`)
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatalf("window overview preview/accessibility errors: %s", got)
	}
}

func TestDesktopWindowOverviewContextEntriesBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
		const a=auditTest;
		a.state.bootstrap.settings['appearance.theme']='standard';
		a.state.bootstrap.settings['appearance.dock_pins']=JSON.stringify(['calculator','files','writer']);
		a.state.bootstrap.settings['windows.animations']='false';
		a.applyDesktopSettings();
		a.openApp('calculator',{forceNew:true});await new Promise(resolve=>requestAnimationFrame(resolve));
		a.openApp('calculator',{forceNew:true});
	}`)
	page.MustWait(`()=>document.querySelectorAll('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]').length===1&&!!document.querySelector('#vd-taskbar-apps [data-pin-app-id="calculator"]')`)
	page.MustEval(`async()=>{
		const group=document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]');
		group.focus();
		document.querySelector('.vd-taskbar').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:80,clientY:80}));
	}`)
	page.MustElement(`.vd-context-menu [data-context-action="window-overview"]`).MustClick()
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelector('#vd-window-overview')?.dataset.overviewMode==='switcher'`)
	if err := page.Keyboard.Press(input.Escape); err != nil {
		t.Fatal(err)
	}
	if err := page.Keyboard.Release(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()&&document.activeElement===document.querySelector('#vd-taskbar-apps .vd-task-button[data-app-id="calculator"]')`)
	page.MustEval(`()=>{
		const pin=document.querySelector('#vd-taskbar-apps [data-pin-app-id="calculator"]');
		pin.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:100,clientY:100}));
	}`)
	page.MustElement(`.vd-context-menu [data-context-action="app-windows"]`).MustClick()
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelector('#vd-window-overview')?.dataset.overviewMode==='app-group'`)
	if err := page.Keyboard.Press(input.Escape); err != nil {
		t.Fatal(err)
	}
	if err := page.Keyboard.Release(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()&&document.activeElement===document.querySelector('#vd-taskbar-apps [data-pin-app-id="calculator"]')`)
	page.MustEval(`()=>{
		window.contextCloseIDs=[...auditTest.state.windows.values()].filter(win=>win.appId==='calculator').map(win=>win.id);
		const pin=document.querySelector('#vd-taskbar-apps [data-pin-app-id="calculator"]');
		pin.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:100,clientY:100}));
	}`)
	page.MustElement(`.vd-context-menu [data-context-action="close-app-windows"]`).MustClick()
	page.MustWait(`()=>contextCloseIDs.every(id=>!auditTest.state.windows.has(id))`)
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatalf("window overview context-entry errors: %s", got)
	}
}

func TestDesktopWindowOverviewSnapPartnerBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
		const a=auditTest;
		a.state.bootstrap.settings['appearance.theme']='standard';
		a.state.bootstrap.settings['windows.animations']='false';
		a.applyDesktopSettings();
		for(let i=0;i<2;i++){a.openApp('files',{forceNew:true});await new Promise(resolve=>requestAnimationFrame(resolve));}
		window.overviewSnapWindows=[...a.state.windows.values()].filter(win=>win.appId==='files');
		window.overviewSnapSource=overviewSnapWindows[0];
		window.overviewSnapMinimized=overviewSnapWindows[1];
		window.overviewSnapOriginalSize={width:overviewSnapMinimized.element.style.width,height:overviewSnapMinimized.element.style.height};
		if(!a.applyWindowSnap(overviewSnapSource.element,'left-half',{showPartnerChooser:false}))throw new Error('could not snap source to the left');
		overviewSnapMinimized.element.querySelector('[data-action="minimize"]').click();
		a.focusWindow(overviewSnapSource.id);
	}`)
	page.MustWait(`()=>overviewSnapMinimized.element.style.display==='none'&&!overviewSnapMinimized.minimizing&&overviewSnapSource.snapped==='left-half'`)
	page.MustEval(`()=>AuraDesktopWindowOverview.open({mode:'snap-partner',sourceId:overviewSnapSource.id,partnerZone:'right-half',opener:overviewSnapSource.element.querySelector('[data-action="maximize"]')})`)
	page.MustWait(`()=>AuraDesktopWindowOverview.isOpen()&&document.querySelector('#vd-window-overview')?.dataset.overviewMode==='snap-partner'`)
	if !page.MustEval(`()=>{
		const ids=[...document.querySelectorAll('#vd-window-overview [data-overview-window]')].map(node=>node.dataset.overviewWindow);
		const card=[...document.querySelectorAll('#vd-window-overview [data-overview-window]')].find(node=>node.dataset.overviewWindow===overviewSnapMinimized.id);
		return ids.length===1&&!ids.includes(overviewSnapSource.id)&&ids.includes(overviewSnapMinimized.id)
			&&!!card&&(card.dataset.overviewMinimized==='true'||!!card.querySelector('[data-overview-minimized="true"]'));
	}`).Bool() {
		t.Fatal("snap partner overview omitted a minimized but eligible candidate")
	}
	page.MustEval(`()=>{
		const card=[...document.querySelectorAll('[data-overview-window]')].find(node=>node.dataset.overviewWindow===overviewSnapMinimized.id);
		const before=JSON.stringify({active:auditTest.state.activeWindowId,snapped:overviewSnapMinimized.snapped,space:auditTest.state.activeSpaceId});
		card.dispatchEvent(new MouseEvent('mouseover',{bubbles:true}));
		card.dispatchEvent(new MouseEvent('pointerover',{bubbles:true}));
		window.overviewSnapHoverBefore=before;
	}`)
	if !page.MustEval(`()=>JSON.stringify({active:auditTest.state.activeWindowId,snapped:overviewSnapMinimized.snapped,space:auditTest.state.activeSpaceId})===overviewSnapHoverBefore&&AuraDesktopWindowOverview.isOpen()`).Bool() {
		t.Fatal("hovering a snap candidate changed selection, window state or overview visibility")
	}
	page.MustEval(`()=>{
		const button=[...document.querySelectorAll('[data-window-activate]')].find(node=>node.dataset.windowActivate===overviewSnapMinimized.id);
		if(!button)throw new Error('minimized snap partner has no activation control');
		button.click();
	}`)
	page.MustWait(`()=>!AuraDesktopWindowOverview.isOpen()&&auditTest.state.activeWindowId===overviewSnapMinimized.id&&overviewSnapMinimized.snapped==='right-half'&&overviewSnapMinimized.element.style.display!=='none'&&overviewSnapMinimized.element.contains(document.activeElement)`)
	if !page.MustEval(`()=>{
		const source=overviewSnapSource.element,partner=overviewSnapMinimized.element;
		const bounds=auditTest.workspaceBoundsForWindow();
		return Math.abs(parseFloat(source.style.left))<2&&Math.abs(parseFloat(source.style.width)-bounds.width/2)<3
			&&Math.abs(parseFloat(partner.style.left)-bounds.width/2)<3&&Math.abs(parseFloat(partner.style.width)-bounds.width/2)<3
	}`).Bool() {
		t.Fatal("activating a snap partner did not fill the complementary right half")
	}
	page.MustEval(`()=>auditTest.restoreWindowFromSnap(overviewSnapMinimized.id)`)
	page.MustWait(`()=>overviewSnapMinimized.snapped===''&&overviewSnapMinimized.element.style.width===overviewSnapOriginalSize.width&&overviewSnapMinimized.element.style.height===overviewSnapOriginalSize.height`)
	page.MustEval(`()=>{
		if(!auditTest.applyWindowSnap(overviewSnapMinimized.element,'right-half',{showPartnerChooser:false}))throw new Error('could not place the visible occupant in the right half');
		window.overviewOccupiedAccepted=AuraDesktopWindowOverview.open({mode:'snap-partner',sourceId:overviewSnapSource.id,partnerZone:'right-half',opener:overviewSnapSource.element.querySelector('[data-action="maximize"]')});
	}`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,50))`)
	if page.MustEval(`()=>AuraDesktopWindowOverview.isOpen()||overviewOccupiedAccepted===true`).Bool() {
		t.Fatal("a visible window already occupying the partner half did not suppress the overview")
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatalf("snap partner overview errors: %s", got)
	}
}
