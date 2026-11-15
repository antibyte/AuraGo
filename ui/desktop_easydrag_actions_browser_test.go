package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// easyDragActionsHelper installs window.__acts: collectors that read every surface an action appears on (the window menus, the
// "more" menu, the card menu, the canvas menu, the card tools, the header buttons and the shortcut table) into one list of
// {surface, id, label, keys, enabled, reason}, and analyse(), which compares them. An entry's id is the registry row it maps to
// (a menu item id is the row's id or its legacy window-menu id).
const easyDragActionsHelper = `()=>{
	const ED=window.EasyDrag, A=ED.actions;
	const strip=x=>String(x==null?'':x).replace(/\s+/g,' ').trim();
	const rowOf=id=>A.row(id)||A.ROWS.find(r=>r.menu===id)||null;
	const ed=()=>edFixture.editor();
	const list=[];
	const push=(surface,row,label,keys,enabled,reason)=>{if(row)list.push({surface,id:row.id,label:strip(label),keys:strip(keys),enabled:!!enabled,reason:strip(reason)});};
	const titleKeys=(title,label)=>{const m=/^ \(([^)]*)\)/.exec(strip(title).slice(label.length));return m?m[1]:'';};
	window.__acts={
		// menusFresh is true once every window-menu entry shows the registry's availability: the editor rebuilds the window
		// menus 120 ms after a selection change (setMenusSoon), so a read right after selecting would see the old state.
		menusFresh(){
			let fresh=true;
			const visit=item=>{
				if(item.type==='separator')return;
				if(item.type==='submenu'){item.items.forEach(visit);return;}
				const row=rowOf(item.id);
				if(row&&!item.disabled!==A.state(row.id,{host:ed()}).enabled)fresh=false;
			};
			aurora.state.windowMenus.get(ed().windowId).renderedMenus.filter(m=>m.id!=='window').forEach(m=>m.items.forEach(visit));
			return fresh;
		},
		windowMenus(){
			const menus=aurora.state.windowMenus.get(ed().windowId).renderedMenus.filter(m=>m.id!=='window');
			const visit=item=>{
				if(item.type==='separator')return;
				if(item.type==='submenu'){item.items.forEach(visit);return;}
				push('window',rowOf(item.id),item.label||ed().t(item.labelKey),item.shortcut||item.shortcutHint,!item.disabled,item.disabled?item.disabledHint:'');
			};
			menus.forEach(m=>m.items.forEach(visit));
		},
		context(surface){
			document.querySelectorAll('.vd-context-menu [data-context-action]').forEach(b=>{
				const kbd=b.querySelector('.vd-context-shortcut');
				push(surface,rowOf(b.dataset.contextAction.split('/').pop()),b.querySelector('.vd-context-label').textContent,kbd?kbd.textContent:'',!b.disabled,b.disabled?b.getAttribute('title'):'');
			});
		},
		tools(){
			const card=document.querySelector('.ed-node.is-selected');
			if(!card)return;
			card.querySelectorAll('[data-ed-node-tool]').forEach(b=>{
				const found=A.TOOLS.find(([tool])=>tool===b.dataset.edNodeTool);
				const label=b.getAttribute('aria-label');
				push('tool',rowOf(found[1]),label,titleKeys(b.title,label),b.getAttribute('aria-disabled')!=='true'&&!b.disabled,b.getAttribute('aria-description'));
			});
		},
		header(){
			const ids={test:'flow.test',cancel:'flow.stop',publish:'flow.publish','run-now':'flow.run_now',active:'flow.active'};
			document.querySelectorAll('.ed-head [data-ed-cmd]').forEach(b=>{
				const id=ids[b.dataset.edCmd];
				if(!id||b.hidden||!b.getClientRects().length)return;
				const label=strip(b.textContent);
				push('header',rowOf(id),label,titleKeys(b.title,label),b.getAttribute('aria-disabled')!=='true'&&!b.disabled,b.getAttribute('aria-description'));
			});
			document.querySelectorAll('.ed-head [data-ed-history]').forEach(b=>{
				if(!b.getClientRects().length)return;
				const label=b.getAttribute('aria-label');
				push('header',rowOf(b.dataset.edHistory==='undo'?'edit.undo':'edit.redo'),label,titleKeys(b.title,label),b.getAttribute('aria-disabled')!=='true',b.getAttribute('aria-description'));
			});
		},
		table(){
			const dialog=ED.dialogs.shortcuts(ed());
			dialog.el.parentNode.querySelectorAll('.ed-keys-row').forEach(r=>{
				const keys=[...r.querySelectorAll('dt kbd')].map(k=>k.textContent).join('+');
				const row=A.ROWS.find(x=>x.shortcut&&A.display(x.shortcut,ed().t)===keys);
				if(row)list.push({surface:'table',id:row.id,label:strip(r.querySelector('dd').textContent),keys,enabled:true,reason:''});
			});
			dialog.close(null);
		},
		reset(){list.length=0;},
		// analyse compares the entries of one state: the same label, displayed shortcut, availability and reason on every
		// surface that lists an action, a reason on every unavailable entry. need maps an action id to the fewest surfaces it
		// must appear on.
		analyse(need){
			const problems=[], byId=new Map();
			list.forEach(e=>{
				const row=A.row(e.id), marked=e.id.startsWith('view.liveliness');
				const want=A.display(row.shortcut,ed().t);
				if(!marked&&e.surface!=='table'&&e.keys!==want)problems.push(e.surface+' '+e.id+': shortcut "'+e.keys+'", the registry says "'+want+'"');
				if(e.surface==='table'&&e.label!==strip(A.label(e.id,{host:ed()})))problems.push('table '+e.id+': label "'+e.label+'"');
				if(e.surface!=='table'&&!e.enabled&&!e.reason)problems.push(e.surface+' '+e.id+': unavailable without a reason');
				if(!byId.has(e.id))byId.set(e.id,[]);
				byId.get(e.id).push(e);
			});
			byId.forEach((entries,id)=>{
				const live=entries.filter(e=>e.surface!=='table');
				live.slice(1).forEach(e=>{
					if(e.label!==live[0].label)problems.push(id+': label "'+e.label+'" on '+e.surface+', "'+live[0].label+'" on '+live[0].surface);
					if(e.enabled!==live[0].enabled)problems.push(id+': '+(e.enabled?'available':'unavailable')+' on '+e.surface+', '+(live[0].enabled?'available':'unavailable')+' on '+live[0].surface);
					if(e.reason!==live[0].reason)problems.push(id+': reason "'+e.reason+'" on '+e.surface+', "'+live[0].reason+'" on '+live[0].surface);
				});
			});
			Object.keys(need).forEach(id=>{const n=new Set((byId.get(id)||[]).map(e=>e.surface)).size;if(n<need[id])problems.push(id+' is listed on '+n+' surfaces, expected at least '+need[id]);});
			const out={problems,compared:byId.size,facts:Object.fromEntries([...byId].map(([id,entries])=>[id,entries.filter(e=>e.surface!=='table').map(e=>e.enabled?'on':e.reason)[0]||'']))};
			list.length=0;
			return JSON.stringify(out);
		}
	};
}`

type easyDragActionsReport struct {
	Problems []string          `json:"problems"`
	Compared int               `json:"compared"`
	Facts    map[string]string `json:"facts"`
}

// actionsMenu opens a context menu (by a click or a JS expression), collects it as a surface and closes it.
func (s *easyDragSmoke) actionsMenu(surface, open string) {
	s.t.Helper()
	s.page.MustEval(open)
	s.wait(`()=>!!document.querySelector('.vd-context-menu [data-context-action]')`)
	s.page.MustEval(`(surface)=>window.__acts.context(surface)`, surface)
	s.page.MustEval(`()=>aurora.closeContextMenu()`)
	s.wait(`()=>!document.querySelector('.vd-context-menu')`)
}

// actionsAgree collects the surfaces of one state and fails on the first disagreement. A surface is one of
// window, more, card, canvas, tools, header, table. need maps action ids to the fewest surfaces they must appear on.
func (s *easyDragSmoke) actionsAgree(state string, surfaces []string, need map[string]int) easyDragActionsReport {
	s.t.Helper()
	p := s.page
	p.MustEval(`()=>window.__acts.reset()`)
	for _, surface := range surfaces {
		switch surface {
		case "window":
			s.wait(`()=>window.__acts.menusFresh()`)
			p.MustEval(`()=>window.__acts.windowMenus()`)
		case "more":
			s.actionsMenu("more", `()=>document.querySelector('[data-ed-cmd="more"]').click()`)
		case "card":
			s.actionsMenu("card", `()=>{const c=document.querySelector('.ed-node.is-selected')||document.querySelector('.ed-node');const r=c.getBoundingClientRect();c.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:r.left+20,clientY:r.top+20}));}`)
		case "canvas":
			s.actionsMenu("canvas", `()=>{const c=document.querySelector('.ed-canvas');const r=c.getBoundingClientRect();c.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:r.left+6,clientY:r.bottom-6}));}`)
		case "tools":
			p.MustEval(`()=>window.__acts.tools()`)
		case "header":
			p.MustEval(`()=>window.__acts.header()`)
		case "table":
			p.MustEval(`()=>window.__acts.table()`)
			s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)
		default:
			s.t.Fatalf("unknown surface %q", surface)
		}
	}
	needJSON, err := json.Marshal(need)
	if err != nil {
		s.t.Fatal(err)
	}
	var report easyDragActionsReport
	if err := json.Unmarshal([]byte(p.MustEval(`(need)=>window.__acts.analyse(JSON.parse(need))`, string(needJSON)).Str()), &report); err != nil {
		s.t.Fatal(err)
	}
	if len(report.Problems) > 0 {
		s.t.Fatalf("%s: the surfaces disagree:\n\t%s", state, strings.Join(report.Problems, "\n\t"))
	}
	s.t.Logf("%s: %d actions compared on %v", state, report.Compared, surfaces)
	return report
}

// TestEasyDragActionsAgreeBrowser opens every surface of the action registry in four states (normal, nothing selected, a run in
// progress, a past run open) and checks that each action shows the same label, displayed shortcut, availability and reason on
// every surface that lists it, that every unavailable entry carries a reason, and that each registry shortcut fires its action once.
func TestEasyDragActionsAgreeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()
	p.MustEval(easyDragActionsHelper)
	select1 := `()=>{const ed=edFixture.editor();const id=ed.model.doc.nodes.find(n=>!ed.model.info(n.type).trigger).id;ed.bus.emit('focus-node',id);}`
	s.settle()

	// 1. Normal: the published, switched-on flow with one step selected. Every surface is open.
	p.MustEval(select1)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1`)
	normal := s.actionsAgree("normal", []string{"window", "more", "card", "canvas", "tools", "header", "table"}, map[string]int{
		"edit.duplicate": 4, "edit.delete": 4, "step.test": 2, "step.toggle": 3, "flow.test": 3, "flow.run_now": 3, "edit.undo": 3, "edit.select_all": 3, "view.zoom_fit": 3,
	})
	if normal.Facts["flow.test"] != "on" || normal.Facts["edit.duplicate"] != "on" {
		t.Fatalf("normal: Test and Duplicate must be available, got %v", normal.Facts)
	}
	if !strings.Contains(normal.Facts["edit.undo"], "Nichts") {
		t.Fatalf("normal: Undo has nothing to undo and says so, got %q", normal.Facts["edit.undo"])
	}

	// 2. Nothing selected.
	p.MustEval(`()=>{const ed=edFixture.editor();ed.selection.clear();ed.bus.emit('selection',ed.selection);}`)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===0`)
	s.settle()
	none := s.actionsAgree("nothing selected", []string{"window", "more", "canvas", "header", "table"}, map[string]int{
		"edit.cut": 1, "edit.copy": 1, "edit.duplicate": 1, "edit.extract": 2, "flow.test": 3, "view.find_step": 3,
	})
	if !strings.Contains(none.Facts["edit.cut"], "Schritt") || !strings.Contains(none.Facts["edit.duplicate"], "Schritt") {
		t.Fatalf("nothing selected: Cut and Duplicate say to select a step first, got %v", none.Facts)
	}

	// 3. A run in progress: Test is off ("Ein Lauf läuft gerade."), Stop is on, edits stay on.
	p.MustEval(`()=>{edFixture.state.delay=2500;}`)
	p.MustEval(select1)
	p.MustEval(`()=>window.EasyDrag.actions.run('flow.test',{host:edFixture.editor()})`)
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="run"]')`)
	// The flow has effects, so Test opens in Simulation, which finishes at once; the real test runs the 2.5 s steps.
	p.MustEval(`()=>{const s=document.querySelector('.ed-modal [data-ed-test-mode]');s.value='real';s.dispatchEvent(new Event('change',{bubbles:true}));}`)
	p.MustElement(`.ed-modal [data-ed-action="run"]`).MustClick()
	s.wait(`()=>{const stop=document.querySelector('[data-ed-cmd="cancel"]');return !!stop && !stop.hidden && !document.querySelector('.ed-modal-backdrop')}`)
	p.MustEval(select1)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1`)
	running := s.actionsAgree("run in progress", []string{"window", "more", "card", "canvas", "tools", "header", "table"}, map[string]int{
		"flow.test": 3, "step.test": 2, "flow.stop": 1, "edit.duplicate": 4,
	})
	if !strings.Contains(running.Facts["flow.test"], "Lauf") || !strings.Contains(running.Facts["step.test"], "Lauf") || running.Facts["flow.stop"] != "on" {
		t.Fatalf("run in progress: Test and Test this step say a run is going and Stop is on, got %v", running.Facts)
	}
	p.MustElement(`[data-ed-cmd="cancel"]`).MustClick()
	s.wait(`()=>{const stop=document.querySelector('[data-ed-cmd="cancel"]');return !stop || stop.hidden}`)
	p.MustEval(`()=>{edFixture.state.delay=140;}`)

	// 4. A past run open: nothing can be changed, and every entry says so.
	p.MustEval(`()=>{const id=edFixture.addWaitingRun();Array.from(window.EasyDragApp._instances.values())[0].screen.showRun(id);}`)
	s.wait(`()=>!document.querySelector('.ed-runview-banner').hidden`)
	s.settle()
	// One step selected, as in the other states: the card menu selects its card when it opens, so without a selection
	// the window menus (read first) and the card menu would see two different selections.
	p.MustEval(select1)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1`)
	view := s.actionsAgree("past run", []string{"window", "more", "card", "canvas", "header", "table"}, map[string]int{
		"flow.save": 2, "flow.test": 3, "edit.duplicate": 2, "step.test": 1, "view.palette": 1,
	})
	if !strings.Contains(view.Facts["flow.save"], "Lauf-Rückblick") || !strings.Contains(view.Facts["edit.duplicate"], "Lauf-Rückblick") {
		t.Fatalf("past run: Save and Duplicate say a past run is open, got %v", view.Facts)
	}
	p.MustElement(`[data-ed-cmd="exit-run-view"]`).MustClick()
	s.wait(`()=>document.querySelector('.ed-runview-banner').hidden`)
	s.settle()

	// 5. Each registry shortcut fires its action once, or is refused with its reason (nothing runs). The key goes to the canvas, as
	// a user's would; observe() counts the runs of the registry.
	ids := p.MustEval(`()=>window.EasyDrag.actions.ROWS.filter(r=>r.shortcut).map(r=>r.id)`).Arr()
	fired := 0
	for _, raw := range ids {
		id := raw.Str()
		p.MustEval(select1)
		s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1 && !document.querySelector('.ed-modal-backdrop') && !document.querySelector('.ed-detail-backdrop') && !document.querySelector('.ed-quick')`)
		got := p.MustEval(`(id)=>{
			const ED=window.EasyDrag, A=ED.actions, ed=edFixture.editor(), row=A.row(id);
			const node=ed.model.doc.nodes[1].id;
			if(id==='edit.undo'||id==='edit.redo'){ed.model.moveNodes([node],40,0);if(id==='edit.redo')ed.model.undo();}
			const canvas=document.querySelector('.ed-canvas');canvas.focus();
			const expected=A.state(id,{host:ed}).enabled?1:0;
			const parts=row.shortcut.split('+');let key=parts[parts.length-1];
			if(row.shortcut==='+'){key='+';}
			const shift=parts.includes('Shift')||['+','?'].includes(row.shortcut), alt=parts.includes('Alt'), ctrl=parts.includes('Ctrl');
			if(row.shortcut==='Shift+1')key='!';
			if(key.length===1&&(shift&&/[a-z]/i.test(key)))key=key.toUpperCase();else if(key.length===1&&/[A-Z]/.test(key))key=key.toLowerCase();
			let count=0;const off=A.observe(()=>count++);
			canvas.dispatchEvent(new KeyboardEvent('keydown',{key,ctrlKey:ctrl,shiftKey:shift,altKey:alt,bubbles:true,cancelable:true,composed:true}));
			off();
			return JSON.stringify({id,expected,count});
		}`, id).Str()
		var res struct {
			ID       string `json:"id"`
			Expected int    `json:"expected"`
			Count    int    `json:"count"`
		}
		if err := json.Unmarshal([]byte(got), &res); err != nil {
			t.Fatal(err)
		}
		if res.Count != res.Expected {
			t.Fatalf("shortcut of %s: ran %d times, expected %d", id, res.Count, res.Expected)
		}
		fired += res.Count
		// What the shortcut opened closes before the next one: the step view with its close button (Escape never closes it from
		// an active field, and its name field has the focus), a dialog or quick-add with an Escape sent to the overlay itself
		// (each listens on its own element, and the focus may sit on the desktop body).
		for i := 0; i < 4 && p.MustEval(`()=>!!(document.querySelector('.ed-modal-backdrop')||document.querySelector('.ed-detail-backdrop')||document.querySelector('.ed-quick'))`).Bool(); i++ {
			p.MustEval(`()=>{const close=document.querySelector('.ed-detail-backdrop:not(.is-closing) [data-ed-detail-close]');if(close){close.click();return;}const el=document.querySelector('.ed-modal-backdrop')||document.querySelector('.ed-quick');const target=(el&&el.contains(document.activeElement))?document.activeElement:el;if(target)target.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true,composed:true}));}`)
			p.MustEval(`()=>new Promise(r=>setTimeout(r,200))`)
		}
		s.wait(`()=>!document.querySelector('.ed-modal-backdrop') && !document.querySelector('.ed-detail-backdrop') && !document.querySelector('.ed-quick')`)
	}
	if fired < 17 {
		t.Fatalf("only %d of %d registry shortcuts fired an action", fired, len(ids))
	}
	s.t.Logf("shortcuts: %d of %d fired once, the rest were refused with their reason", fired, len(ids))

	// 6. The start page's card menu: the card's flow is the context; its entries carry the registry's labels.
	p.MustElement(`[data-ed-cmd="home"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustEval(`()=>document.querySelector('[data-ed-card-menu]').click()`)
	s.wait(`()=>!!document.querySelector('.vd-context-menu [data-context-action]')`)
	home := p.MustEval(`()=>{
		const A=window.EasyDrag.actions, t=Array.from(window.EasyDragApp._instances.values())[0].t, out=[];
		document.querySelectorAll('.vd-context-menu [data-context-action]').forEach(b=>{
			const row=A.row(b.dataset.contextAction)||A.ROWS.find(r=>r.menu===b.dataset.contextAction);
			const label=b.querySelector('.vd-context-label').textContent;
			if(!row)out.push('unknown entry '+b.dataset.contextAction);
			else if(label!==t(row.label))out.push(row.id+': label "'+label+'", the registry says "'+t(row.label)+'"');
			if(b.disabled&&!b.getAttribute('title'))out.push((row&&row.id)+': unavailable without a reason');
		});
		return JSON.stringify({problems:out,count:document.querySelectorAll('.vd-context-menu [data-context-action]').length});
	}`).Str()
	var homeReport struct {
		Problems []string `json:"problems"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(home), &homeReport); err != nil {
		t.Fatal(err)
	}
	if len(homeReport.Problems) > 0 || homeReport.Count < 7 {
		t.Fatalf("start page card menu: %d entries, problems %v", homeReport.Count, homeReport.Problems)
	}
	p.MustEval(`()=>aurora.closeContextMenu()`)
	s.failOnPageErrors("actions agree")
}

// TestEasyDragActionsMenuClicksBrowser clicks real entries of the "more" menu and of a card menu while the desktop animates:
// a menu that is closing stays in the DOM with vd-context-menu-closing, and its action must still run.
func TestEasyDragActionsMenuClicksBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Motion: true})
	p := s.page
	// The fixture shell sets windows.animations to false; make the desktop animate so the closing class really appears.
	feature := &proto.EmulationMediaFeature{Name: "prefers-reduced-motion", Value: "no-preference"}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{feature}}).Call(p); err != nil {
		t.Fatalf("emulate prefers-reduced-motion: %v", err)
	}
	p.MustEval(`()=>{document.body.dataset.animations='true';}`)

	s.wait(`()=>document.querySelectorAll('.ed-flow-card').length===1`)
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4 && document.querySelectorAll('.ed-edge').length===3`)
	s.settle()

	// The "more" menu: Flow settings opens its dialog.
	p.MustElement(`[data-ed-cmd="more"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.vd-context-menu [data-context-action$="settings"]')`)
	p.MustElement(`.vd-context-menu [data-context-action$="settings"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal')`)
	p.MustEval(`()=>{const m=document.querySelector('.ed-modal-backdrop');m.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true,composed:true}));}`)
	s.wait(`()=>!document.querySelector('.ed-modal-backdrop')`)
	s.settle()

	// A card menu: Duplicate on the selected card adds a card, and Undo stays available.
	p.MustEval(`()=>{const ed=edFixture.editor();const id=ed.model.doc.nodes.find(n=>!ed.model.info(n.type).trigger).id;ed.bus.emit('focus-node',id);}`)
	s.wait(`()=>document.querySelectorAll('.ed-node.is-selected').length===1`)
	s.settle()
	p.MustEval(`()=>{const c=document.querySelector('.ed-node.is-selected');const r=c.getBoundingClientRect();c.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:r.left+20,clientY:r.top+20}));}`)
	s.wait(`()=>!!document.querySelector('.vd-context-menu [data-context-action$="duplicate"]')`)
	p.MustElement(`.vd-context-menu [data-context-action$="duplicate"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===5`)
	s.wait(`()=>!document.querySelector('.vd-context-menu')`)
	s.settle()
	if p.MustEval(`()=>document.querySelector('.ed-head [data-ed-history="undo"]').getAttribute('aria-disabled')==='true'`).Bool() {
		t.Fatal("Undo is unavailable after the card menu's Duplicate")
	}
	s.failOnPageErrors("actions menu clicks")
}
