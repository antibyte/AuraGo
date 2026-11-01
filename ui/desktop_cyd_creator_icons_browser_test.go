package ui

import (
	"testing"

	"aurago/internal/cydcreator"
	"github.com/go-rod/rod/lib/input"
)

func TestDesktopCYDCreatorIconsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	for _, theme := range []string{"standard", "fruity"} {
		for _, motion := range []string{"no-preference", "reduce"} {
			t.Run(theme+"/"+motion, func(t *testing.T) {
				s := startCYDDesignerSmoke(t, motion, "de", theme)
				page := s.page
				page.MustEval(`catalog=>{
            const f=cydFixture;
            const P=CYDCreatorParts, asset={id:'A-mask_1',w:1e9,h:32,encoding:'a8'};
            const fallback=P.iconLibrary.node(asset,{icon_animate:true,icon_motion:'frames',icon_frames:['missing']},id=>'/fixture-asset/'+id,'#ffffff');
            if(fallback.attrs.class.includes('is-animated')||fallback.attrs.style.width!=='2048px'||fallback.children.length!==1)throw new Error('Invalid frame data must retain a bounded still image');
            f.inst.root=document.querySelector('.cydc-app');
            f.iconRequests=[];f.failIcon=false;
            f.inst.iconCatalog=async()=>catalog;
            f.inst.assetUrl=id=>{const a=f.store.get().assets.find(a=>a.id===id);return a&&a.icon?'/img/cyd-icons/'+a.icon.slice(2):'/fixture-asset/'+id};
            f.inst.importIcon=async r=>{
                f.iconRequests.push({...r});
                if(f.failIcon)throw Object.assign(new Error('budget'),{code:'CYD_ASSET_TOO_LARGE'});
                if(f.delayIcon)await new Promise(resolve=>{f.resolveIcon=resolve});
                const m=catalog.motifs.find(m=>m.id===r.motif);
                const assets=Array.from({length:r.animated&&m.motion==='frames'?6:1},(_,n)=>{
                    const id='a_'+[r.style,r.tone,r.motif,r.size,n].join('_');
                    return {id,file:'assets/'+id+'.png',w:r.size,h:r.size,alpha:true,encoding:r.tone==='mono'?'a8':'',icon:'1/'+[r.style,r.tone,r.motif,r.size,n+'.png'].join('/')};
                });
                const p=JSON.parse(JSON.stringify(f.store.get()));
                for(const a of assets)if(!p.assets.some(old=>old.id===a.id))p.assets.push(a);
                f.store.replace(p,{resetHistory:false});
                return {asset:assets[0],assets,frames:assets.map(a=>a.id),motion:m.motion,version:2};
            };
            f.state.select(['w_btn']);
        }`, cydcreator.Icons())
				page.MustElement(`[data-cc-focus="icon-library"]`).MustClick()
				s.wait(`()=>document.querySelectorAll('[data-cc-icon]').length===110`)
				page.MustEval(`()=>{const el=document.querySelector('[data-cc-icon-filter="category"]');el.value='characters';el.dispatchEvent(new Event('change',{bubbles:true}))}`)
				s.wait(`()=>document.querySelectorAll('[data-cc-icon]').length===10`)
				page.MustElement(`[data-cc-focus="icons:search"]`).MustInput("Roboter")
				s.wait(`()=>document.querySelectorAll('[data-cc-icon]').length===1&&!!document.querySelector('[data-cc-icon="robot"]')`)
				page.MustElement(`[data-cc-icon="robot"]`).MustFocus()
				page.Keyboard.MustType(input.Enter)
				s.wait(`()=>document.querySelector('[data-cc-icon="robot"]').getAttribute('aria-pressed')==='true'`)
				page.MustEval(`()=>{const el=document.querySelector('[data-cc-focus="icons:search"]');el.value='';el.dispatchEvent(new Event('input',{bubbles:true}))}`)
				page.MustEval(`()=>{const el=document.querySelector('[data-cc-icon-filter="category"]');el.value='';el.dispatchEvent(new Event('change',{bubbles:true}))}`)
				page.MustEval(`()=>{for(const [key,value] of [['tone','mono'],['animated','true']]){const el=document.querySelector('[data-cc-icon-filter="'+key+'"]');el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}))}}`)
				page.MustElement(`[data-cc-icon="gopher"]`).MustClick()
				s.wait(`()=>document.querySelectorAll('.cydc-icon-detail .cydc-icon-frame').length===12`)
				s.wait(`()=>{const card=document.querySelector('.cydc-icon-choice');return getComputedStyle(card.querySelector('.cydc-icon-art')).color===getComputedStyle(card).color}`)
				if motion == "reduce" {
					s.wait(`()=>[...document.querySelectorAll('.cydc-icon-detail .cydc-icon-frame')].every(el=>getComputedStyle(el).animationName==='none')`)
				}
				s.noRawKeys("icon picker")
				s.shot("icons-" + theme)
				page.MustElement(`[data-cc-dialog-action="insert"]`).MustClick()
				s.wait(`()=>!document.querySelector('.cydc-icon-dialog')&&cydFixture.widget('w_btn').props.icon_frames.length===6`)
				s.wait(`()=>document.querySelectorAll('[data-cc-widget-id="w_btn"] .cydc-icon-frame').length===6`)
				page.MustEval(`()=>{const el=document.querySelector('[data-cc-field="prop:icon_tint"]');el.value='#20a060';el.dispatchEvent(new Event('input',{bubbles:true}))}`)
				s.wait(`()=>getComputedStyle(document.querySelector('[data-cc-widget-id="w_btn"] .cydc-icon-frame')).backgroundColor==='rgb(32, 160, 96)'`)
				page.MustElement(`[data-cc-focus="icon-library"]`).MustClick()
				s.wait(`()=>document.querySelector('[data-cc-icon="gopher"]').getAttribute('aria-pressed')==='true'`)
				page.MustEval(`()=>{const el=document.querySelector('[data-cc-icon-filter="style"]');el.value='comic';el.dispatchEvent(new Event('change',{bubbles:true}))}`)
				page.MustElement(`[data-cc-dialog-action="insert"]`).MustClick()
				s.wait(`()=>cydFixture.widget('w_btn').props.icon_origin.includes('/comic/')`)
				page.MustEval(`()=>cydFixture.store.undo()`)
				s.wait(`()=>cydFixture.widget('w_btn').props.icon_origin.includes('/modern/')`)
				page.MustEval(`()=>cydFixture.store.redo()`)
				s.wait(`()=>cydFixture.widget('w_btn').props.icon_origin.includes('/comic/')`)
				page.MustElement(`[data-cc-focus="icon-library"]`).MustClick()
				s.wait(`()=>document.querySelectorAll('[data-cc-icon]').length===110`)
				page.MustEval(`()=>{cydFixture.failIcon=true;const el=document.querySelector('[data-cc-icon-filter="size"]');el.value='64';el.dispatchEvent(new Event('change',{bubbles:true}))}`)
				page.MustElement(`[data-cc-dialog-action="insert"]`).MustClick()
				s.wait(`()=>document.querySelector('.cydc-dialog-error').textContent.includes('kleinere Größe')`)
				page.MustEval(`()=>{cydFixture.failIcon=false;for(const [key,value] of [['size','24'],['animated','false']]){const el=document.querySelector('[data-cc-icon-filter="'+key+'"]');el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}))}}`)
				page.MustElement(`[data-cc-dialog-action="insert"]`).MustClick()
				s.wait(`()=>!document.querySelector('.cydc-icon-dialog')&&cydFixture.widget('w_btn').props.icon_animate===false`)
				page.MustEval(`()=>{cydFixture.state.setScreen('s_details');cydFixture.state.select(['w_img'])}`)
				page.MustElement(`[data-cc-focus="icon-library"]`).MustClick()
				s.wait(`()=>document.querySelectorAll('[data-cc-icon]').length===110`)
				page.MustElement(`[data-cc-focus="icons:search"]`).MustInput("Kaffeemaschine")
				s.wait(`()=>document.querySelectorAll('[data-cc-icon]').length===1`)
				page.MustElement(`[data-cc-icon="coffee_machine"]`).MustFocus()
				page.Keyboard.MustType(input.Enter)
				s.wait(`()=>document.querySelector('[data-cc-icon="coffee_machine"]').getAttribute('aria-pressed')==='true'`)
				page.MustEval(`()=>cydFixture.delayIcon=true`)
				page.MustElement(`[data-cc-dialog-action="insert"]`).MustClick()
				s.wait(`()=>typeof cydFixture.resolveIcon==='function'`)
				page.MustEval(`()=>{const el=document.querySelector('[data-cc-icon-filter="size"]');el.value='64';el.dispatchEvent(new Event('change',{bubbles:true}));cydFixture.state.commands.run('resize',[{op:'update_widget',id:'w_img',set:{w:80,h:48}}]);cydFixture.resolveIcon()}`)
				s.wait(`()=>{const w=cydFixture.widget('w_img');return !document.querySelector('.cydc-icon-dialog')&&w.w===80&&w.h===48&&w.props.icon_origin==='1/modern/color/coffee_machine/32/0.png'}`)
				s.wait(`()=>{const art=document.querySelector('[data-cc-widget-id="w_img"] .cydc-icon-art');return getComputedStyle(art).width==='80px'&&getComputedStyle(art).height==='48px'&&getComputedStyle(art.firstChild).objectFit==='contain'}`)
				page.MustEval(`()=>cydFixture.store.undo()`)
				s.wait(`()=>cydFixture.widget('w_img').props.asset==='a_sun'`)
				page.MustEval(`()=>cydFixture.store.redo()`)
				s.wait(`()=>cydFixture.widget('w_img').props.icon_origin.includes('/coffee_machine/32/')`)
				// A real handle drag must resize the artwork before committing the gesture.
				page.MustEval(`()=>{cydFixture.delayIcon=false;cydFixture.state.setZoom(1)}`)
				x, y := s.center(`[data-cc-handle="se"]`)
				if !s.drag(x, y, x+48, y+32, `()=>{const art=document.querySelector('[data-cc-widget-id="w_img"] .cydc-icon-art');return getComputedStyle(art).width==='128px'&&getComputedStyle(art).height==='80px'&&cydFixture.widget('w_img').w===80}`) {
					t.Fatal("icon artwork did not follow the live resize")
				}
				s.wait(`()=>cydFixture.widget('w_img').w===128&&cydFixture.widget('w_img').h===80`)
				page.MustElement(`[data-cc-focus="icon-library"]`).MustClick()
				s.wait(`()=>document.querySelectorAll('[data-cc-icon]').length===110`)
				page.MustEval(`()=>{for(const [key,value] of [['tone','mono'],['animated','true']]){const el=document.querySelector('[data-cc-icon-filter="'+key+'"]');el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}))}}`)
				page.MustElement(`[data-cc-icon="gopher"]`).MustClick()
				page.MustElement(`[data-cc-dialog-action="insert"]`).MustClick()
				s.wait(`()=>{const w=cydFixture.widget('w_img'),art=document.querySelector('[data-cc-widget-id="w_img"] .cydc-icon-art');return !document.querySelector('.cydc-icon-dialog')&&w.w===128&&w.h===80&&art.children.length===6&&getComputedStyle(art).width==='128px'&&getComputedStyle(art.firstChild).maskSize==='contain'}`)
				s.shot("icons-resized-" + theme)
				page.MustEval(`()=>{for(const [key,value] of [['geo:w','16'],['geo:h','16']]){const el=cydFixture.byFocus(key);el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}))}}`)
				s.wait(`()=>{const art=document.querySelector('[data-cc-widget-id="w_img"] .cydc-icon-art');return getComputedStyle(art).width==='16px'&&getComputedStyle(art).height==='16px'}`)
				page.MustEval(`()=>cydFixture.setReadonly(true)`)
				s.wait(`()=>document.querySelector('[data-cc-focus="icon-library"]').matches(':disabled')`)
				s.noErrors("icons " + theme)
			})
		}
	}
}
