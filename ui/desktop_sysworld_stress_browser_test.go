package ui

import (
	"github.com/go-rod/rod"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func verifySystemWorldStress(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	page.MustSetViewport(1920, 1080, 1, false)
	page.MustEval(`()=>{
      document.body.dataset.animations='true';window.worldStressFetch=fetch;window.worldActionCalls=0;window.worldStressState='idle';window.worldGap=false;
      window.worldGraph=Array.from({length:10000},(_,i)=>({id:'scale-'+i,label:'Knowledge '+i,type:'entity'}));
      window.fetch=(url,opts={})=>{
        const path=String(url),now=Date.now(),respond=data=>Promise.resolve(new Response(JSON.stringify(data),{headers:{'Content-Type':'application/json'}}));
        const entities=[{id:'agent',kind:'district',district:'agent',state:'idle',at:now,model:'Recorded model'},
          {id:'graph',kind:'district',district:'graph',state:'idle',at:now,values:{nodes:10000}},
          {id:'mission:test',kind:'mission',district:'missions',label:'Review mission',state:worldStressState,at:now,actions:['start']},
          {id:'container:confirm',kind:'container',district:'infra',label:'Review container',state:'running',at:now,actions:['stop','restart']}];
        if(path==='/api/containers')return respond(Array.from({length:1000},(_,i)=>({id:'scale-'+i,name:'Container '+i,state:'running'})));
        if(path==='/api/knowledge-graph/nodes?limit=300')return respond({nodes:worldGraph.slice(0,300),total:worldGraph.length});
        if(path==='/api/knowledge-graph/node?id=scale-299')return respond({edges:[{source:'scale-299',target:'scale-9999',relation:'related'}]});
        if(path==='/api/desktop/system-world/snapshot')return respond({at:now,metrics:{cpu:14,ram:38,disk:18},entities});
        if(path.startsWith('/api/desktop/system-world/snapshot?')){if(worldGap)return Promise.resolve(new Response('{}',{status:404}));return respond({at:now-600000,metrics:{cpu:0,ram:28},entities:entities.map(e=>({...e,actions:[]}))});}
        if(path.startsWith('/api/desktop/system-world/events?'))return respond([{id:1,at:now-610000,entity:entities[2]}]);
        if(path==='/api/desktop/system-world/history')return respond(Array.from({length:1440},(_,i)=>({at:now-(1440-i)*60000,key:'cpu',average:i%70,count:6})));
        if(path==='/api/desktop/system-world/actions'){const r=JSON.parse(opts.body);if(r.entity!=='container:confirm'||r.action!=='restart'||r.confirmed!==true)throw Error('Wrong target/action');worldActionCalls++;return respond({status:'completed'});}
        return worldStressFetch(url,opts);
      };SysWorld.data.refresh();
    }`)
	page.Timeout(15 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).entityCount>1300`)
	page.MustEval(`()=>{const q=document.querySelector('.sw-search');q.value='Container 999';q.dispatchEvent(new Event('input'));}`)
	page.MustElement(`.sw-results [data-entity="container:scale-999"]`).MustClick()
	page.MustEval(`()=>{document.querySelector('.sw-search').value='';document.querySelector('.sw-search').dispatchEvent(new Event('input'));document.querySelector('[data-sw-action="close"]').click();document.querySelector('[data-sw-mode="orbit"]').click();}`)
	// Only a changed, fresh mission state may move cargo.
	page.MustEval(`()=>{window.beforeFreight=SysWorldApp.inspect(cityId).experience.freightEvents;worldStressState='running';SysWorld.data.refresh();}`)
	page.Timeout(15 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.freightEvents===beforeFreight+1`)
	page.MustEval(`()=>{document.querySelector('.sw-world').open=true;const details=document.querySelectorAll('.sw-world-section');details[1].open=true;const target=document.querySelector('.sw-world-target');target.value='container:confirm';target.dispatchEvent(new Event('change'));const buttons=details[1].querySelectorAll('.sw-world-verbs button');buttons[1].click();buttons[1].click();if(document.querySelectorAll('.vd-modal').length!==1)throw Error('Double confirmation');}`)
	page.MustEval(`()=>{if(!document.querySelector('.vd-modal-copy').textContent.includes('Review container')||worldActionCalls!==0)throw Error('Action lacks target confirmation');}`)
	page.MustElement(`.vd-modal [type="submit"]`).MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>worldActionCalls===1`)
	page.MustEval(`()=>{const timeline=document.querySelectorAll('.sw-world-section')[2];timeline.open=true;const r=timeline.querySelector('input');r.value=-10;r.dispatchEvent(new Event('change'));}`)
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('[data-sw-stat="cpu"]').textContent.includes('0')`)
	page.MustEval(`()=>{if([...document.querySelectorAll('.sw-world-verbs button')].filter(b=>/Start|Stopp|Neustart/.test(b.textContent)).some(b=>!b.disabled))throw Error('Replay permits actions');worldGap=true;const r=document.querySelector('.sw-world-section>input');r.value=-20;r.dispatchEvent(new Event('change'));}`)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).entityCount===0`)
	page.MustScreenshot(filepath.Join(dir, "world2-history-gap.png"))
	page.MustEval(`()=>{document.querySelectorAll('.sw-world-section')[2].querySelector('.sw-world-verbs button').click();document.querySelector('.sw-world').open=false;}`)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).entityCount>1300`)
	for _, tier := range []string{"high", "low"} {
		page.MustEval(`tier=>{const q=document.querySelector('.sw-quality');q.value=tier;q.dispatchEvent(new Event('change'));}`, tier)
		time.Sleep(1800 * time.Millisecond)
		result := page.MustEval(`async()=>{let last=performance.now();const ms=[];for(let i=0;i<200;i++){await new Promise(requestAnimationFrame);const now=performance.now();if(i>20)ms.push(now-last);last=now;}ms.sort((a,b)=>a-b);return JSON.stringify({meanMS:ms.reduce((a,b)=>a+b)/ms.length,p95MS:ms[Math.floor(ms.length*.95)],graphDataset:worldGraph.length,containers:1000,state:SysWorldApp.inspect(cityId)},null,2);}`).Str()
		if err := os.WriteFile(filepath.Join(dir, "world2-stress-"+tier+".json"), []byte(result), 0600); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(dir, "world2-stress-"+tier+".png"))
	}
	for i := 0; i < 3; i++ {
		page.MustEval(`async()=>{await aurora.closeWindow(cityId);}`)
		page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId)===null`)
		page.MustEval(`async()=>{for(const callbacks of cityHandlers.values())if(callbacks.size)throw Error('Leaked subscription');await fixtureOpen('system-world');window.cityId=aurora.state.activeWindowId;}`)
		page.Timeout(30 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId)?.frames>3`)
	}
	page.MustEval(`async()=>{await aurora.closeWindow(cityId);}`)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId)===null`)
	page.MustEval(`()=>{for(const c of cityAudioContexts)if(c.state!=='closed')throw Error('Audio leak');}`)
	if errs := page.MustEval(`()=>JSON.stringify(cityErrors)`).Str(); errs != "[]" {
		t.Fatal(errs)
	}
}
