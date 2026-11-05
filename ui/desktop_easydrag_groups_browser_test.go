package ui

import (
	"fmt"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopEasyDragGroupsBrowser exercises the real canvas with visual groups.
func TestDesktopEasyDragGroupsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-groups-fixture.js")
	p := s.page
	id := p.MustEval(`()=>{const id=edGroupsFixture.addDraft('Gruppierter Ablauf',4),doc=edFixture.flow(id).rec.draft;doc.nodes.forEach((node,i)=>{node.position={x:(i%2)*280,y:Math.floor(i/2)*240};});doc.edges=doc.nodes.slice(1).map((node,i)=>({id:'e_aaaaaaa'+'abc'[i],source:{node:doc.nodes[i].id,port:'out'},target:{node:node.id,port:'in'}}));return id;}`).Str()
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, id)
	s.wait(`()=>!!edFixture.editor() && edFixture.editor().model.doc.nodes.length===4`)
	p.MustEval(`()=>{const ed=edFixture.editor(),ids=ed.model.doc.nodes.map(n=>n.id);if(!ed.model.createGroup(ids.slice(0,2),'Eingangsdaten prüfen, bereinigen und für die Verarbeitung vorbereiten','blue','Eingangsdaten vorbereiten') || !ed.model.createGroup(ids.slice(2,4),'Ergebnis','amber','Ausgabe prüfen'))throw Error('Group creation failed');}`)
	s.wait(`()=>document.querySelectorAll('.ed-group-frame').length===2 && document.querySelectorAll('.ed-group-head').length===2`)
	p.MustEval(`()=>fixtureTheme('standard')`)
	p.MustSetViewport(1440, 900, 1, false)
	p.MustElement(`[data-ed-zoom="fit"]`).MustClick()
	s.wait(`()=>{const c=document.querySelector('.ed-canvas').getBoundingClientRect(),frames=[...document.querySelectorAll('.ed-group-frame')];return document.querySelectorAll('.ed-edge').length===3 && frames.length===2 && frames.every(f=>{const r=f.getBoundingClientRect();return r.left>=c.left-1 && r.right<=c.right+1 && r.top>=c.top-1 && r.bottom<=c.bottom+1;});}`)
	s.shot("groups-connected-standard-1440")
	p.MustEval(`()=>fixtureTheme('fruity-light')`)
	p.MustSetViewport(390, 844, 1, true)
	p.MustElement(`[data-ed-zoom="fit"]`).MustClick()
	s.wait(`()=>{const c=document.querySelector('.ed-canvas').getBoundingClientRect();return [...document.querySelectorAll('.ed-group-frame')].every(f=>{const r=f.getBoundingClientRect();return r.left>=c.left-1 && r.right<=c.right+1 && r.top>=c.top-1 && r.bottom<=c.bottom+1;});}`)
	s.shot("groups-connected-fruity-390")
	// The first-run port hint also stays compact and dismissible on a disconnected phone canvas.
	disconnected := p.MustEval(`async()=>{await edFixture.editor().saver.flush();localStorage.removeItem('aurago.easydrag.hint.ports');return edGroupsFixture.addDraft('Unverbunden',4);}`).Str()
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, disconnected)
	s.wait(`()=>{const hint=document.querySelector('[data-ed-port-tip]');if(!hint || hint.hidden)return false;const r=hint.getBoundingClientRect();return r.width>150 && r.width<=innerWidth && r.height<180 && r.left>=0 && r.right<=innerWidth;}`)
	s.shot("groups-disconnected-hint-fruity-390")
	p.MustElement(`[data-ed-port-tip-close]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-port-tip]').hidden`)

	// Create from a keyboard selection, then use the inspector's single-pointer controls.
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>{fixtureTheme('standard');document.querySelector('.ed-canvas').focus();}`)
	p.KeyActions().Press(input.ControlLeft).Type('a').MustDo()
	s.wait(`()=>edFixture.editor().selection.size===4`)
	p.MustElement(`[data-ed-cmd="groups"]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-group-create]:not(:disabled)')`)
	p.MustElement(`[data-ed-group-create]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-group-inspector]') && edFixture.editor().model.doc.groups?.length===1`)
	groupID := p.MustEval(`()=>edFixture.editor().model.doc.groups[0].id`).Str()
	p.MustElement(`[data-ed-group-title]`).MustSelectAllText().MustInput(`A <img src=x onerror="window.groupInjected=true"> long group title`)
	p.Keyboard.MustType(input.Tab)
	p.MustElement(`[data-ed-group-note]`).MustInput("A note for the grouped steps")
	p.Keyboard.MustType(input.Tab)
	p.MustEval(`()=>{const e=document.querySelector('[data-ed-group-color]');e.value='rose';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
	s.wait(`()=>{const g=edFixture.editor().model.doc.groups[0];return g.color==='rose' && g.title.includes('<img') && g.note==='A note for the grouped steps';}`)
	p.MustEval(`id=>{const h=document.querySelector('.ed-group-head[data-ed-group="'+id+'"]');if(h.querySelector('img') || window.groupInjected)throw Error('Group title rendered as markup');}`, groupID)
	p.MustElement(`[data-ed-group-title]`).MustSelectAllText().MustInput("Eingangsdaten prüfen und für die Verarbeitung vorbereiten")
	p.Keyboard.MustType(input.Tab)
	s.wait(`()=>edFixture.editor().model.doc.groups[0].title==='Eingangsdaten prüfen und für die Verarbeitung vorbereiten'`)
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		for _, width := range []int{320, 390, 1440} {
			p.MustSetViewport(width, 900, 1, width < 760)
			p.MustEval(`theme=>fixtureTheme(theme)`, theme)
			s.shot(fmt.Sprintf("groups-inspector-%s-%d", theme, width))
			bounds := p.MustEval(`()=>{const e=document.querySelector('[data-ed-group-inspector] .ed-detail'),r=e.getBoundingClientRect(),rect=x=>{const a=x?.getBoundingClientRect();return a?{left:a.left,right:a.right,width:a.width,client:x.clientWidth,scroll:x.scrollWidth}:null};return {left:r.left,right:r.right,viewport:innerWidth,scroll:e.scrollWidth,client:e.clientWidth,canvas:rect(document.querySelector('.ed-canvas')),app:rect(document.querySelector('.ed-app')),root:rect(document.documentElement),ancestors:(()=>{const a=[];for(let x=e;x;x=x.parentElement){const c=getComputedStyle(x);a.push({tag:x.tagName,class:x.className,id:x.id,...rect(x),minWidth:c.minWidth,cssWidth:c.width,overflow:c.overflow});}return a;})(),overflow:[...e.querySelectorAll('*')].filter(x=>x.scrollWidth>x.clientWidth+1).slice(0,10).map(x=>({tag:x.tagName,class:x.className,...rect(x)}))};}`).Map()
			if bounds["left"].Num() < -1 || bounds["right"].Num() > bounds["viewport"].Num()+1 || bounds["scroll"].Int() > bounds["client"].Int()+1 {
				t.Fatalf("inspector %s %d has horizontal overflow: %+v", theme, width, bounds)
			}
		}
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	removed := p.MustEval(`()=>edFixture.editor().model.doc.groups[0].members[3]`).Str()
	p.MustElement(`[data-ed-group-remove="` + removed + `"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.groups[0].members.length===3`)
	p.MustEval(`id=>{const e=document.querySelector('[data-ed-group-add-member]');e.value=id;e.dispatchEvent(new Event('change',{bubbles:true}));}`, removed)
	p.MustElement(`[data-ed-group-add]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.groups[0].members.length===4`)
	p.MustEval(`()=>window.__groupBeforeMove=Object.fromEntries(edFixture.editor().model.doc.nodes.map(n=>[n.id,{...n.position}]))`)
	p.MustElement(`[data-ed-group-dx]`).MustSelectAllText().MustInput("32")
	p.MustElement(`[data-ed-group-dy]`).MustSelectAllText().MustInput("16")
	p.MustElement(`[data-ed-group-move]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.every(n=>n.position.x===window.__groupBeforeMove[n.id].x+32 && n.position.y===window.__groupBeforeMove[n.id].y+16)`)
	p.MustElement(`[data-ed-group-inspector] [data-ed-detail-close]`).MustClick()
	p.MustElement(`[data-ed-history="undo"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.every(n=>n.position.x===window.__groupBeforeMove[n.id].x && n.position.y===window.__groupBeforeMove[n.id].y)`)
	// Complete-group copy/paste remaps IDs; a single undo removes the pasted group.
	p.MustElement(`[data-ed-group-drag="` + groupID + `"]`).MustClick()
	p.MustEval(`()=>document.querySelector('.ed-canvas').focus()`)
	p.KeyActions().Press(input.ControlLeft).Type('c').MustDo()
	p.KeyActions().Press(input.ControlLeft).Type('v').MustDo()
	s.wait(`()=>edFixture.editor().model.doc.nodes.length===8 && edFixture.editor().model.doc.groups.length===2`)
	p.KeyActions().Press(input.ControlLeft).Type('z').MustDo()
	s.wait(`()=>edFixture.editor().model.doc.nodes.length===4 && edFixture.editor().model.doc.groups.length===1`)
	// Collapse is local; the outline can navigate into a hidden member and expands it.
	p.MustElement(`.ed-group-head[data-ed-group="` + groupID + `"] [data-ed-group-action="collapse"]`).MustClick()
	s.wait(`()=>edFixture.editor().groups.collapsed.has('` + groupID + `') && document.querySelectorAll('.ed-node[hidden]').length===4`)
	p.MustEval(`()=>{const ed=edFixture.editor(),n=ed.model.doc.nodes[0];ed.issues=[{code:'PARAM_INVALID',severity:'error',node_id:n.id,message:'test issue'}];ed.bus.emit('issues',ed.issues);}`)
	s.wait(`()=>document.querySelector('.ed-group-counts').textContent.includes('1 Fehler')`)
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		for _, width := range []int{320, 390} {
			p.MustSetViewport(width, 900, 1, true)
			p.MustEval(`theme=>fixtureTheme(theme)`, theme)
			s.shot(fmt.Sprintf("groups-collapsed-%s-%d", theme, width))
		}
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>{const ed=edFixture.editor(),label=ed.t('easydrag.ui.node_list');[...document.querySelectorAll('.ed-rail button')].find(b=>b.getAttribute('aria-label')===label).click();}`)
	s.wait(`()=>!!document.querySelector('[data-ed-outline-group="` + groupID + `"]')`)
	for _, width := range []int{320, 390} {
		p.MustSetViewport(width, 900, 1, true)
		p.MustEval(`()=>fixtureTheme('fruity-dark')`)
		s.shot(fmt.Sprintf("groups-outline-fruity-dark-%d", width))
	}
	p.MustSetViewport(1440, 900, 1, false)
	p.MustElement(`[data-ed-focus-node="` + removed + `"]`).MustClick()
	s.wait(`()=>!edFixture.editor().groups.collapsed.has('` + groupID + `') && !document.querySelector('.ed-node[data-node-id="` + removed + `"]').hidden`)
	// A stored run remains immutable while groups can be inspected and collapsed locally.
	p.MustEval(`async()=>{const ed=edFixture.editor();await ed.saver.flush();const id='run_groupsfixture',now=new Date().toISOString(),doc=JSON.parse(JSON.stringify(ed.model.doc)),run={id,flow_id:ed.flow.id,revision:ed.saver.revision,mode:'test',trigger_node:doc.nodes[0].id,trigger_type:'test',status:'error',started_at:now,finished_at:now};const steps=[{node_id:doc.nodes[0].id,node_key:doc.nodes[0].key,status:'error',error_code:'FLOW_NODE_FAILED'}];window.__groupRunDoc=JSON.stringify(doc);edFixture.state.runs.set(id,{run,doc,steps,events:[],listeners:new Set(),done:true});EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:ed.flow.id,runId:id});}`)
	s.wait(`()=>!!edFixture.editor().runView`)
	p.MustElement(`.ed-group-head[data-ed-group="` + groupID + `"] [data-ed-group-action="edit"]`).MustClick()
	s.wait(`()=>document.querySelector('[data-ed-group-title]')?.readOnly && document.querySelector('[data-ed-group-color]')?.disabled`)
	p.MustElement(`[data-ed-group-view="collapse"]`).MustClick()
	s.wait(`()=>edFixture.editor().groups.collapsed.has('` + groupID + `') && JSON.stringify(edFixture.editor().runView.doc)===window.__groupRunDoc`)
	s.shot("groups-readonly-history")

	// Restoring an older block version clears groups added later without publishing again.
	p.MustSetViewport(1440, 900, 1, false)
	blockID := p.MustEval(`async()=>{const base='/api/desktop/flow-blocks',request=async(path,method,body)=>{const r=await fetch(path,{method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});if(!r.ok)throw Error('fixture '+method+' '+path+' '+r.status);return r.json();};const {block}=await request(base,'POST',{name:'Block mit Gruppen'}),id=block.id;await request(base+'/'+id+'/publish','POST',{base_revision:1});const doc=JSON.parse(JSON.stringify(block.draft));doc.groups=[{id:'g_aaaaaaaa',title:'Spätere Gruppe',color:'blue',members:doc.nodes.map(n=>n.id)}];await request(base+'/'+id,'PUT',{base_revision:1,doc});await request(base+'/'+id+'/publish','POST',{base_revision:2});EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{blockId:id});return id;}`).Str()
	s.wait(`()=>edFixture.editor()?.isBlock && edFixture.editor().flow.live_revision===2 && edFixture.editor().model.doc.groups?.length===1`)
	p.MustElement(`[data-ed-cmd="more"]`).MustClick()
	p.MustEval(`()=>{const label=edFixture.editor().t('easydrag.ui.versions_title');[...document.querySelectorAll('.vd-context-menu button')].find(b=>b.textContent.includes(label)).click();}`)
	s.wait(`()=>!!document.querySelector('[data-ed-block-restore="1"]')`)
	p.MustElement(`[data-ed-block-restore="1"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.vd-modal [type="submit"]')`)
	p.MustElement(`.vd-modal [type="submit"]`).MustClick()
	s.wait(`()=>{const ed=edFixture.editor();return ed.isBlock && ed.flow.live_revision===2 && (!ed.model.doc.groups || ed.model.doc.groups.length===0);}`)
	p.MustEval(`id=>{const saved=edBlockFixture.state.blocks.get(id),v2=edBlockFixture.state.versions.get(id+':2');if(saved.live_revision!==2 || v2.doc.groups.length!==1)throw Error('Restore altered published block version');}`, blockID)
	s.shot("groups-block-restored")

	// Larger fixtures retain bounded canvas elements; timings are observations on this machine.
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>fixtureTheme('standard')`)
	fifty := p.MustEval(`()=>edGroupsFixture.addDraft('Fünfzig Schritte',50)`).Str()
	p.MustEval(`id=>EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id})`, fifty)
	s.wait(`()=>!!edFixture.editor() && edFixture.editor().model.doc.nodes.length===50 && document.querySelectorAll('.ed-node').length===50`)
	p.MustEval(`()=>{const ed=edFixture.editor(),ids=ed.model.doc.nodes.map(n=>n.id);for(let i=0;i<5;i++)if(!ed.model.createGroup(ids.slice(i*10,i*10+10),'Gruppe '+(i+1),['blue','green','amber','rose','violet'][i],'Zehn Schritte'))throw Error('50-node grouping');}`)
	s.wait(`()=>document.querySelectorAll('.ed-group-frame').length===5`)
	p.MustElement(`[data-ed-zoom="fit"]`).MustClick()
	s.wait(`()=>document.querySelectorAll('[data-ed-outline-group]').length===5 && document.querySelectorAll('[data-ed-focus-node]').length===50`)
	s.clearToasts()
	s.shot("groups-fifty-standard-1440")
	p.MustElement(`[data-ed-focus-node]`).MustClick()
	s.wait(`()=>edFixture.editor().view.zoom>=0.8`)
	s.shot("groups-fifty-outline-focus-standard-1440")
	// A narrow group with every status and a two-line note reserves header space at every viewport.
	boundsID := p.MustEval(`()=>{const id=edGroupsFixture.addDraft('Kopfzeilenprüfung',4),doc=edFixture.flow(id).rec.draft;doc.nodes.forEach((node,i)=>node.position={x:100,y:200+i*240});EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id});return id;}`).Str()
	s.wait(`()=>edFixture.editor()?.flow.id==='` + boundsID + `' && document.querySelectorAll('.ed-node').length===4`)
	if p.MustEval(`()=>!!document.querySelector('.ed-node-region.is-outline')`).Bool() {
		p.MustElement(`[data-ed-outline-close]`).MustClick()
		s.wait(`()=>!document.querySelector('.ed-node-region.is-outline')`)
	}
	p.MustEval(`()=>{const ed=edFixture.editor(),nodes=ed.model.doc.nodes,ids=nodes.map(n=>n.id);nodes[3].settings.disabled=true;const id=ed.model.createGroup(ids,'Statusgruppe','green','Ein längerer Hinweis erklärt die Gruppe und erstreckt sich absichtlich über mehrere Zeilen.');if(!id)throw Error('status group');ed.run={id:'run_group_bounds',status:'running',mode:'test',steps:new Map([[ids[0],{node_id:ids[0],status:'error'}],[ids[1],{node_id:ids[1],status:'running'}],[ids[2],{node_id:ids[2],status:'success'}]]),changes:ids.slice(0,3),edges:new Map(),record:null,error:''};ed.bus.emit('run');window.__statusGroup=id;}`)
	s.wait(`()=>{const ed=edFixture.editor(),s=document.querySelector('.ed-group-counts')?.textContent||'';return ['group_errors','group_running','group_success','group_disabled'].every(k=>s.includes(ed.t('easydrag.ui.'+k,{count:1})));}`)
	for _, width := range []int{320, 390, 1440} {
		p.MustSetViewport(width, 900, 1, width < 760)
		p.MustElement(`[data-ed-zoom="reset"]`).MustClick()
		s.wait(`()=>Math.abs(edFixture.editor().view.zoom-1)<0.001`)
		s.settle()
		measure := p.MustEval(`()=>{const ed=edFixture.editor(),g=ed.model.doc.groups.find(g=>g.id===window.__statusGroup),head=document.querySelector('.ed-group-head[data-ed-group="'+g.id+'"]'),note=head.querySelector('.ed-group-note'),first=document.querySelector('.ed-node[data-node-id="'+g.members[0]+'"]');return {headBottom:head.getBoundingClientRect().bottom,nodeTop:first.getBoundingClientRect().top,noteHeight:note.getBoundingClientRect().height,line:parseFloat(getComputedStyle(note).lineHeight)};}`).Map()
		if measure["noteHeight"].Num() < measure["line"].Num()*1.8 || measure["headBottom"].Num() > measure["nodeTop"].Num()+1 {
			t.Fatalf("group header overlaps member at viewport %d: %+v", width, measure)
		}
	}
	p.MustEval(`()=>{edFixture.editor().run=null;}`)
	p.MustSetViewport(1440, 900, 1, false)
	p.MustEval(`()=>{const id=edGroupsFixture.addDraft('Fünfhundert Schritte',500);window.__groupLargeStarted=performance.now();EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{flowId:id});}`)
	s.wait(`()=>!!edFixture.editor() && edFixture.editor().model.doc.nodes.length===500 && document.querySelectorAll('.ed-node').length===500`)
	p.MustEval(`()=>{const ed=edFixture.editor();if(!ed.model.createGroup(ed.model.doc.nodes.map(n=>n.id),'Große Gruppe','blue',''))throw Error('500-node grouping');}`)
	s.wait(`()=>document.querySelectorAll('.ed-group-frame').length===1`)
	metrics := p.MustEval(`()=>({elapsed:Math.round(performance.now()-window.__groupLargeStarted),cards:document.querySelectorAll('.ed-node').length,frames:document.querySelectorAll('.ed-group-frame').length,dom:document.querySelector('.ed-canvas').querySelectorAll('*').length})`).Map()
	if metrics["cards"].Int() != 500 || metrics["frames"].Int() != 1 || metrics["dom"].Int() > 50000 {
		t.Fatalf("unexpected 500-node canvas bounds: %+v", metrics)
	}
	t.Logf("500-node local observation: %d ms, %d cards, %d group frame, %d canvas elements", metrics["elapsed"].Int(), metrics["cards"].Int(), metrics["frames"].Int(), metrics["dom"].Int())
	s.noRawKeys("visual groups")
	s.failOnPageErrors("visual groups")
}
