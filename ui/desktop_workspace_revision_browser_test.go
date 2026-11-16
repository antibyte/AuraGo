package ui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

type desktopWorkspaceBrowserFixture struct {
	mu    sync.Mutex
	state desktop.WorkspaceState
}

func (fixture *desktopWorkspaceBrowserFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(fixture.state)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "method_not_allowed"})
		return
	}
	var mutation desktop.WorkspaceMutation
	if err := json.NewDecoder(r.Body).Decode(&mutation); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "invalid_json"})
		return
	}
	if mutation.Revision != fixture.state.Layout.Revision {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": "workspace_revision_conflict", "state": fixture.state})
		return
	}
	layout := fixture.state.Layout
	layout.Workspaces = append([]desktop.Workspace(nil), layout.Workspaces...)
	layout.RemovedTo = copyWorkspaceRedirects(layout.RemovedTo)
	switch mutation.Operation {
	case "add":
		layout.Workspaces = append(layout.Workspaces, desktop.Workspace{ID: jsonNumber(layout.NextID), Name: mutation.Name})
		layout.NextID++
	case "rename":
		index := findWorkspace(layout.Workspaces, mutation.ID)
		if index < 0 {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "workspace_not_found"})
			return
		}
		layout.Workspaces[index].Name = mutation.Name
	case "reorder":
		ordered := make([]desktop.Workspace, 0, len(mutation.Order))
		for _, id := range mutation.Order {
			index := findWorkspace(layout.Workspaces, id)
			if index < 0 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": "workspace_invalid"})
				return
			}
			ordered = append(ordered, layout.Workspaces[index])
		}
		layout.Workspaces = ordered
	case "delete":
		index := findWorkspace(layout.Workspaces, mutation.ID)
		if index < 0 || findWorkspace(layout.Workspaces, mutation.MoveTo) < 0 || mutation.MoveTo == mutation.ID {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "workspace_invalid"})
			return
		}
		for removed, target := range layout.RemovedTo {
			if target == mutation.ID {
				layout.RemovedTo[removed] = mutation.MoveTo
			}
		}
		layout.RemovedTo[mutation.ID] = mutation.MoveTo
		layout.Workspaces = append(layout.Workspaces[:index], layout.Workspaces[index+1:]...)
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "workspace_invalid"})
		return
	}
	layout.Revision++
	fixture.state.Layout = layout
	_ = json.NewEncoder(w).Encode(fixture.state)
}

func copyWorkspaceRedirects(source map[string]string) map[string]string {
	copy := make(map[string]string, len(source)+1)
	for id, target := range source {
		copy[id] = target
	}
	return copy
}

func findWorkspace(workspaces []desktop.Workspace, id string) int {
	for index, workspace := range workspaces {
		if workspace.ID == id {
			return index
		}
	}
	return -1
}

func jsonNumber(value uint64) string { return strconv.FormatUint(value, 10) }

func TestDesktopWorkspaceManagerRevisionBrowser(t *testing.T) {
	fixture := &desktopWorkspaceBrowserFixture{state: desktop.WorkspaceState{
		Layout: desktop.DefaultWorkspaceLayout(),
		Settings: map[string]string{
			"session.windows":               "",
			"appearance.wallpaper_by_space": "{}",
			"appearance.wallpaper":          "groupshoot",
		},
	}}
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/workspaces": http.HandlerFunc(fixture.serveHTTP)})
	page.MustEval(`()=>{
        const a=auditTest;
        a.openApp('files');
        window.workspaceWindowId=a.state.activeWindowId;
        a.moveWindowToSpace(workspaceWindowId,'2');
		window.workspaceWindowObject=a.state.windows.get(workspaceWindowId);
        a.switchSpace('2');
        window.workspaceManager=window.AuraDesktopWorkspaceManager;
        workspaceManager.open();
    }`)
	page.MustWait(`()=>!!document.querySelector('#vd-workspace-manager [role="dialog"]')`)
	page.MustWait(`()=>{
		const win=auditTest.state.windows.get(workspaceWindowId);
		return !![...document.querySelectorAll('.file-manager[data-fm-window]')].find(node=>node.dataset.fmWindow===workspaceWindowId)
			&&!!win?.context?.storage;
	}`)
	page.MustEval(`()=>{
		const context=workspaceWindowObject.context||{};
		window.workspaceWindowContext=JSON.stringify({path:context.path??null,storage:context.storage??null});
	}`)
	if !page.MustEval(`()=>document.querySelectorAll('#vd-workspace-manager [data-workspace-row]').length===3&&!!document.querySelector('#vd-workspace-manager [role="dialog"][aria-modal="true"]')`).Bool() {
		t.Fatal("workspace manager did not show the three stable workspace identities")
	}

	page.MustEval(`()=>{
        const input=document.querySelector('[data-workspace-add-name]');
        input.value='Research';
        input.closest('form').dispatchEvent(new Event('submit',{bubbles:true,cancelable:true}));
    }`)
	page.MustWait(`()=>workspaceManager.snapshot().layout.workspaces.some(item=>item.id==='4'&&item.name==='Research')`)

	page.MustEval(`()=>{
        const draft=document.querySelector('[data-workspace-rename="2"]');
        draft.value='Unsaved name draft';draft.focus();
        const input=document.querySelector('[data-workspace-rename="1"]');input.value='Primary';
        document.querySelector('[data-workspace-save="1"]').click();
    }`)
	page.MustWait(`()=>workspaceManager.name('1')==='Primary'`)
	if !page.MustEval(`()=>{
        const draft=document.querySelector('[data-workspace-rename="2"]');
        return draft?.value==='Unsaved name draft'&&document.activeElement?.dataset.workspaceSave==='1';
    }`).Bool() {
		t.Fatal("unrelated workspace revision discarded an unsaved rename or keyboard focus")
	}

	page.MustEval(`()=>{
        const input=document.querySelector('[data-workspace-rename="4"]');input.value='Archive';
        document.querySelector('[data-workspace-save="4"]').click();
    }`)
	page.MustWait(`()=>workspaceManager.name('4')==='Archive'`)
	page.MustEval(`()=>document.querySelector('[data-workspace-up="4"]').click()`)
	page.MustWait(`()=>workspaceManager.snapshot().layout.workspaces[2]?.id==='4'`)
	if !page.MustEval(`()=>document.activeElement?.dataset.workspaceUp==='4'`).Bool() {
		t.Fatal("keyboard focus was lost after workspace reordering")
	}

	page.MustEval(`()=>{
        document.querySelector('[data-workspace-delete="2"]').click();
        const target=document.querySelector('[data-workspace-target="2"]');target.value='3';target.dispatchEvent(new Event('change',{bubbles:true}));
    }`)
	if !page.MustEval(`()=>document.querySelector('[data-workspace-delete-message="2"]').textContent.includes(window.t('desktop.space_n').replace('{{n}}','3'))&&!document.querySelector('[data-workspace-delete-message="2"]').textContent.includes('{{')`).Bool() {
		t.Fatal("delete confirmation did not resolve the selected target label")
	}
	page.MustEval(`()=>document.querySelector('[data-workspace-confirm-delete="2"]').click()`)
	page.MustWait(`()=>workspaceManager.resolveId('2')==='3'&&document.querySelector('.vd-workspace-manager-status')?.textContent===window.t('desktop.workspaces_saved')`)
	deleteState := page.MustEval(`()=>{
        const win=auditTest.state.windows.get(workspaceWindowId);
        const focused=document.activeElement;
		return JSON.stringify({
			windowSame:win===workspaceWindowObject,
			contextSame:JSON.stringify({path:win?.context?.path??null,storage:win?.context?.storage??null})===workspaceWindowContext,
			windowSpace:win?.spaceId,activeSpace:auditTest.state.activeSpaceId,
			focusTag:focused?.tagName,focusData:focused?Object.assign({},focused.dataset):{},
			focusRow:focused?.closest('[data-workspace-row]')?.dataset.workspaceRow||'',
			focusInDialog:!!focused?.closest('#vd-workspace-manager'),status:document.querySelector('.vd-workspace-manager-status')?.textContent||''
		});
	}`).Str()
	if !page.MustEval(`()=>{
		const win=auditTest.state.windows.get(workspaceWindowId),focused=document.activeElement;
		const context={path:win?.context?.path??null,storage:win?.context?.storage??null};
		return win===workspaceWindowObject&&JSON.stringify(context)===workspaceWindowContext&&win?.spaceId==='3'
			&&auditTest.state.activeSpaceId==='3'&&!!focused?.closest('#vd-workspace-manager')
			&&focused.closest('[data-workspace-row]')?.dataset.workspaceRow==='4';
	}`).Bool() {
		t.Fatalf("deleting a workspace did not relocate its live window and focus the nearest surviving row; observed %s", deleteState)
	}

	page.MustEval(`()=>{
        const manager=workspaceManager,base=manager.snapshot();
        const only={version:1,revision:20,next_id:5,workspaces:[{id:'4',name:'Only'}],removed_to:{'1':'4','2':'4','3':'4'}};
        manager.adopt({layout:only,settings:base.settings});
    }`)
	if !page.MustEval(`()=>document.querySelector('.vd-workspace-manager-limit')?.textContent===window.t('desktop.workspaces_minimum')&&!document.querySelector('[data-workspace-delete]')`).Bool() {
		t.Fatal("the one-workspace limit is not visible in the manager")
	}
	page.MustEval(`()=>{
        const manager=workspaceManager,base=manager.snapshot(),workspaces=[];
        for(let id=1;id<=8;id++)workspaces.push({id:String(id),name:'Space '+id});
        manager.adopt({layout:{version:1,revision:21,next_id:9,workspaces,removed_to:{}},settings:base.settings});
    }`)
	if !page.MustEval(`()=>document.querySelectorAll('#vd-workspace-manager [data-workspace-row]').length===8&&[...document.querySelectorAll('.vd-workspace-manager-limit')].some(node=>node.textContent===window.t('desktop.workspaces_maximum'))&&document.querySelector('[data-workspace-add-name]').disabled`).Bool() {
		t.Fatal("the eight-workspace limit is not visible or enforced in the manager")
	}
	page.MustEval(`()=>workspaceManager.close()`)
	page.MustSetViewport(640, 450, 2, false)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "forced-colors", Value: "active"}, {Name: "prefers-reduced-motion", Value: "reduce"},
	}}).Call(page); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`async theme=>{
			await auditTest.saveDesktopPreference('appearance.theme',theme);
			if(document.body.dataset.theme!==theme)throw new Error('theme was not applied: '+theme);
		}`, theme)
		page.MustEval(`()=>{
			const launcher=document.querySelector('#vd-quick-settings-button');
			launcher.focus();workspaceManager.open();
		}`)
		page.MustWait(`()=>!!document.querySelector('#vd-workspace-manager [role="dialog"]')
			&&document.querySelector('#vd-workspace-manager [role="dialog"]').contains(document.activeElement)`)
		if !page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-workspace-manager [role="dialog"]');
			const panel=document.querySelector('.vd-workspace-manager-panel');
			const rect=panel.getBoundingClientRect();
			const controls=[...dialog.querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled)')]
				.filter(node=>!node.closest('[hidden]')&&node.getClientRects().length>0);
			return matchMedia('(forced-colors: active)').matches&&matchMedia('(prefers-reduced-motion: reduce)').matches
				&&rect.left>=0&&rect.top>=0&&rect.right<=innerWidth+1&&rect.bottom<=innerHeight+1
				&&getComputedStyle(document.querySelector('.vd-workspace-manager-backdrop')).backdropFilter==='none'
			&&controls.length>=3;
		}`).Bool() {
			t.Fatalf("workspace controls must remain visible at 640x450 CSS pixels with forced colors and reduced motion in %s: %s", theme, page.MustEval(`()=>JSON.stringify({viewport:[innerWidth,innerHeight],theme:document.body.dataset.theme,panel:document.querySelector('.vd-workspace-manager-panel')?.getBoundingClientRect().toJSON(),controls:[...document.querySelectorAll('#vd-workspace-manager button:not(:disabled),#vd-workspace-manager input:not(:disabled),#vd-workspace-manager select:not(:disabled)')].map(node=>({tag:node.tagName,data:Object.assign({},node.dataset),rect:node.getBoundingClientRect().toJSON()})),active:document.activeElement?.outerHTML,backdrop:getComputedStyle(document.querySelector('.vd-workspace-manager-backdrop')).backdropFilter})`).Str())
		}
		page.MustEval(`()=>document.querySelector('[data-workspace-row="1"]').scrollIntoView({block:'nearest'})`)
		if !page.MustEval(`()=>{
			const panel=document.querySelector('.vd-workspace-manager-panel');
			const row=document.querySelector('[data-workspace-row="1"]'),panelRect=panel.getBoundingClientRect(),rowRect=row.getBoundingClientRect();
			const clipTop=panelRect.top+panel.clientTop,clipBottom=clipTop+panel.clientHeight;
			const rowControls=[...row.querySelectorAll('input,button')].filter(node=>node.getClientRects().length>0);
			return rowRect.top>=clipTop-1&&rowRect.bottom<=clipBottom+1&&rowControls.length>=3
				&&rowControls.every(node=>{const rect=node.getBoundingClientRect();return rect.top>=clipTop-1&&rect.bottom<=clipBottom+1;});
		}`).Bool() {
			t.Fatalf("the first workspace row and its actions were clipped by the dialog scroll area in %s: %s", theme, page.MustEval(`()=>JSON.stringify({panel:document.querySelector('.vd-workspace-manager-panel').getBoundingClientRect().toJSON(),clientHeight:document.querySelector('.vd-workspace-manager-panel').clientHeight,scrollTop:document.querySelector('.vd-workspace-manager-panel').scrollTop,row:document.querySelector('[data-workspace-row="1"]').getBoundingClientRect().toJSON(),controls:[...document.querySelectorAll('[data-workspace-row="1"] input,[data-workspace-row="1"] button')].map(node=>({data:Object.assign({},node.dataset),rect:node.getBoundingClientRect().toJSON()}))})`).Str())
		}
		captureEverydayPanel(t, page, "workspaces-"+theme+"-640x450-dpr2")
		page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-workspace-manager [role="dialog"]');
			const controls=[...dialog.querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled)')]
				.filter(node=>!node.closest('[hidden]')&&node.getClientRects().length>0);
			controls.at(-1).focus();
		}`)
		if err := page.Keyboard.Press(input.Tab); err != nil {
			t.Fatal(err)
		}
		if !page.MustEval(`()=>{
			const dialog=document.querySelector('#vd-workspace-manager [role="dialog"]');
			const first=[...dialog.querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled)')]
				.filter(node=>!node.closest('[hidden]')&&node.getClientRects().length>0)[0];
			return document.activeElement===first;
		}`).Bool() {
			t.Fatalf("Tab did not wrap keyboard focus to the first visible manager control in %s", theme)
		}
		if err := page.Keyboard.Press(input.Escape); err != nil {
			t.Fatal(err)
		}
		page.MustWait(`()=>!document.querySelector('#vd-workspace-manager')&&document.activeElement?.id==='vd-quick-settings-button'`)
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatalf("workspace manager browser errors: %s", got)
	}
}
