package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestDesktopEasyDragNestedBrowser exercises version choice and simulation using the real editor modules.
func TestDesktopEasyDragNestedBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-blocks-fixture.js", "/testdata/easydrag-nested-fixture.js")
	p := s.page
	ids := p.MustEval(`async()=>{
        const base='/api/desktop/flow-blocks', req=async(path,method,body)=>{const r=await fetch(path,{method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});if(!r.ok)throw Error(method+' '+path+' '+r.status);return r.json();};
        const leaf=(await req(base,'POST',{name:'Begrüßung'})).block, lp=base+'/'+leaf.id, ld=leaf.draft;
        for(const n of ld.nodes){if(n.type==='block.input')n.params.fields=[{name:'name',type:'string',required:true,nullable:false}];if(n.type==='block.output'){n.params.fields=[{name:'greeting',type:'string',required:true,nullable:false}];n.params.values={greeting:'Hello {{block_input.data.name}}'};}}
        await req(lp,'PUT',{base_revision:1,doc:ld});await req(lp+'/publish','POST',{base_revision:2});
        const outer=(await req(base,'POST',{name:'Zusammengesetzter Baustein'})).block, op=base+'/'+outer.id, od=outer.draft;
        for(const n of od.nodes){if(n.type==='block.input')n.params.fields=[{name:'name',type:'string',required:true,nullable:false}];if(n.type==='block.output'){n.params.fields=[{name:'greeting',type:'string',required:true,nullable:false}];n.params.values={greeting:'{{child.greeting}}'};}}
        od.nodes.push({id:'n_nestedaa',key:'child',type:'component.call',type_version:1,params:{flow_id:leaf.id,revision:1,inputs:{name:'{{block_input.data.name}}'}},settings:{retry:{count:0}},position:{x:180,y:180}});
        od.edges=[{id:'e_nestedaa',source:{node:od.nodes[0].id,port:'out'},target:{node:'n_nestedaa',port:'in'}},{id:'e_nestedab',source:{node:'n_nestedaa',port:'out'},target:{node:od.nodes[1].id,port:'in'}}];
        await req(op,'PUT',{base_revision:1,doc:od});await req(op+'/publish','POST',{base_revision:2});
        ld.nodes.find(n=>n.type==='block.output').params.values.greeting='Welcome {{block_input.data.name}}';await req(lp,'PUT',{base_revision:2,doc:ld});await req(lp+'/publish','POST',{base_revision:3});
        EasyDragApp.open(Array.from(EasyDragApp._instances.values())[0].windowId,{blockId:outer.id});return {leaf:leaf.id,outer:outer.id};
    }`).Map()
	leaf, outer := ids["leaf"].Str(), ids["outer"].Str()
	s.wait(`()=>edFixture.editor()?.isBlock && edFixture.editor().flow.id==='` + outer + `'`)
	simulate := func(expected string) {
		t.Helper()
		p.MustElement(`[data-ed-cmd="test"]`).MustClick()
		s.wait(`()=>!!document.querySelector('[data-ed-block-inputs]')`)
		p.MustEval(`()=>{const e=document.querySelector('.ed-field[data-param="name"] [data-ed-block-presence]');e.value='value';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
		p.MustElement(`.ed-field[data-param="name"] input`).MustSelectAllText().MustInput("Ada")
		p.Keyboard.MustType(input.Tab)
		p.MustElement(`[data-ed-action="simulate"]`).MustClick()
		s.wait(`()=>!document.querySelector('.ed-modal') && !!document.querySelector('.ed-block-result') && document.querySelector('.ed-block-result').textContent.includes('` + expected + `')`)
	}
	simulate("Hello Ada")
	p.MustElement(`.ed-node[data-node-id="n_nestedaa"] .ed-node-label`).MustDoubleClick()
	s.wait(`()=>!!document.querySelector('[data-ed-call-block]') && !!document.querySelector('[data-ed-nested-call-help]')`)
	p.MustEval(`id=>{const select=document.querySelector('[data-ed-call-block]');if([...select.options].some(o=>o.value===id && !o.disabled))throw Error('self block offered');}`, outer)
	s.wait(`()=>[...document.querySelector('[data-ed-call-version]').options].some(o=>o.value==='2')`)
	p.MustEval(`()=>{const e=document.querySelector('[data-ed-call-version]');e.value='2';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
	p.MustElement(`[data-ed-call-apply]`).MustClick()
	s.wait(`()=>!!document.querySelector('[data-ed-action="apply"]') && document.querySelector('.ed-modal--block')?.textContent.includes(edFixture.editor().t('easydrag.ui.block_dependencies')+': 1')`)
	s.shot("nested-explicit-upgrade")
	p.MustElement(`[data-ed-action="apply"]`).MustClick()
	s.wait(`()=>edFixture.editor().model.doc.nodes.find(n=>n.id==='n_nestedaa').params.revision===2`)
	p.MustElement(`[data-ed-detail-close]`).MustClick()
	simulate("Welcome Ada")
	p.MustEval(`(outer,leaf)=>{const old=edBlockFixture.invokeVersion(outer,1,{name:'Ada'},'2026-01-02T03:04:05Z');if(old.status!=='success'||old.outputs.greeting!=='Hello Ada')throw Error('published parent changed');if(!edBlockFixture.state.versions.has(leaf+':2'))throw Error('leaf v2 missing');}`, outer, leaf)
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		p.MustEval(`theme=>fixtureTheme(theme)`, theme)
		p.MustSetViewport(390, 844, 1, true)
		s.shot("nested-simulation-" + theme + "-390")
	}
	s.noRawKeys("nested blocks")
	s.failOnPageErrors("nested blocks")
}
