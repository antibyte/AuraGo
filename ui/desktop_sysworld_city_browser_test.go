package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Reuse the real Desktop shell/asset router, with bounded read-only data fixtures.
func verifySystemWorldCity(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	page.MustEval(`()=>{
        localStorage.setItem('aurago.desktop.sysworld.quality','high');
        window.cityErrors=[];addEventListener('error',e=>cityErrors.push(e.message));
        addEventListener('unhandledrejection',e=>cityErrors.push(String(e.reason)));
        window.cityNativeFetch=window.fetch;window.cityFailures=false;window.cityCalls={};
        window.cityHandlers=new Map();
        window.AuraSSE={on:(type,fn)=>{if(!cityHandlers.has(type))cityHandlers.set(type,new Set());cityHandlers.get(type).add(fn)},
            off:(type,fn)=>cityHandlers.get(type)?.delete(fn)};
        window.cityEmit=(type,data)=>{for(const fn of cityHandlers.get(type)||[])fn(data)};
        window.fetch=(url,opts={})=>{
            const path=String(url);cityCalls[path]=(cityCalls[path]||0)+1;
            const fixtures={
                '/api/dashboard/overview':{agent:{model:'AuraGo Spark',provider:'Local',personality:'Thinker',context_window:32768,busy:false},missions:{total:12,running:2,queued:3},integrations:{home_assistant:true,docker:true,telegram:false,mqtt:true,meshcore:true,proxmox:true}},
                '/api/dashboard/memory':{vectordb_entries:4216,core_memory_facts:68,journal_entries:129,notes_count:48,chat_messages:1864},
                '/api/dashboard/activity':{coagents:[{id:'c1',name:'Research',status:'running',model:'Spark'}],cron_jobs:[{id:'cron1',name:'Nightly care',expr:'0 3 * * *'}]},
                '/api/missions/v2':{missions:[{id:'m1',name:'Morning briefing',status:'running',run_count:24,success_rate:.96},{id:'m2',name:'Library maintenance',status:'queued'}]},
                '/api/containers':[{id:'docker1',name:'Home Assistant',state:'running',image:'homeassistant:stable'}],
                '/api/daemons':[{id:'d1',name:'Indexer',status:'running',restarts:0}],
                '/api/dashboard/tool-stats':{top_tools:[{name:'web_search',count:152},{name:'read_file',count:84}]},
                '/api/budget':{spent:2.34},
                '/api/knowledge-graph/nodes?limit=300':{nodes:[{id:'n1',label:'AuraGo',type:'project',access_count:48},{id:'n2',label:'Andi',type:'person',access_count:129}]},
                '/api/knowledge-graph/edges?limit=500':{edges:[{source:'n1',target:'n2',relation:'maintained by'}]},
                '/api/operational-issues?status=open&limit=100':{items:[{id:'op1',title:'MQTT reconnect',severity:'warning',occurrence_count:2}],total:1},
            };
            if(cityFailures&&path==='/api/dashboard/overview')return Promise.resolve(new Response('{}',{status:503}));
            if(path in fixtures)return Promise.resolve(new Response(JSON.stringify(fixtures[path]),{headers:{'Content-Type':'application/json'}}));
            return cityNativeFetch(url,opts);
        };
    }`)
	page.Timeout(45 * time.Second).MustEval(`async()=>{await fixtureOpen('system-world');window.cityId=aurora.state.activeWindowId;}`)
	page.Timeout(90 * time.Second).MustWait(`()=>!!(SysWorldApp.inspect(cityId)?.frames>4)`)
	page.MustEval(`()=>{
        if(SysWorldApp.inspect(cityId).entityCount<20)throw Error('Missing real-data entities');
        if(window.THREE)throw Error('City must not install legacy/global THREE');
        const app=document.querySelector('.sysworld');
        if(/sysworld\.city\./.test(app.innerText))throw Error('Untranslated city control');
        if(app.querySelector('.sw-message:not([hidden])'))throw Error('City asset load failed');
    }`)
	page.MustScreenshot(filepath.Join(dir, "city-overview-first.png"))
	if os.Getenv("AURAGO_SYSTEM_WORLD_FIRST") == "1" {
		return
	}
	for _, size := range []struct {
		w, h int
		dpr  float64
	}{{1920, 1080, 1}, {1366, 768, 1}, {430, 932, 2}} {
		page.MustSetViewport(size.w, size.h, size.dpr, size.w < 600)
		for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
			for _, density := range []string{"comfortable", "compact"} {
				page.MustEval(`([theme,density])=>{aurora.state.bootstrap.settings['appearance.density']=density;fixtureTheme(theme)}`, []string{theme, density})
				time.Sleep(150 * time.Millisecond)
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("city-%s-%s-%dx%d.png", theme, density, size.w, size.h)))
				page.MustEval(`()=>{
                    const r=document.querySelector('.sysworld').getBoundingClientRect();
                    for(const s of ['.sw-top','.sw-bottom','.sw-info','.sw-quality']){
                        const e=document.querySelector(s);if(e.hidden)continue;const b=e.getBoundingClientRect();
                        if(b.left<r.left-1||b.right>r.right+1||b.bottom>r.bottom+1)throw Error('Outside city: '+s);
                    }
                }`)
			}
		}
	}
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`()=>{fixtureTheme('standard');document.querySelector('[data-sw-district="infra"]').click()}`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).selected==='infra'`)
	page.MustScreenshot(filepath.Join(dir, "city-infrastructure.png"))
	page.MustEval(`()=>{
        const search=document.querySelector('.sw-search');search.value='telegram';search.dispatchEvent(new Event('input'));
        document.querySelector('.sw-results [data-entity="integration:telegram"]').click();
        if(document.querySelector('.sw-state').dataset.state!=='disabled')throw Error('False online status');
        search.value='';search.dispatchEvent(new Event('input'));
        document.querySelector('[data-sw-district="integrations"]').click();
        if(!document.querySelector('.sw-info-body').textContent.includes('nicht geprüft'))throw Error('Missing connection disclaimer');
    }`)
	page.MustEval(`()=>{document.querySelector('[data-sw-action="close"]').click();document.querySelector('[data-sw-mode="street"]').click()}`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).mode==='street'`)
	page.MustScreenshot(filepath.Join(dir, "city-street.png"))
	before := page.MustEval(`()=>SysWorldApp.inspect(cityId).position`).JSON("", "")
	page.MustEval(`()=>{const c=document.querySelector('.sysworld-gl');c.focus();c.dispatchEvent(new KeyboardEvent('keydown',{code:'KeyW',key:'w',bubbles:true}));}`)
	time.Sleep(400 * time.Millisecond)
	page.MustEval(`()=>document.querySelector('.sysworld-gl').dispatchEvent(new KeyboardEvent('keyup',{code:'KeyW',key:'w',bubbles:true}))`)
	if after := page.MustEval(`()=>SysWorldApp.inspect(cityId).position`).JSON("", ""); after == before {
		t.Fatal("Street movement did not move")
	}
	page.MustEval(`()=>{document.querySelector('[data-sw-mode="map"]').click();}`)
	page.Timeout(20 * time.Second).MustWait(`()=>!SysWorldApp.inspect(cityId).raf`)
	page.MustScreenshot(filepath.Join(dir, "city-map.png"))
	page.MustEval(`()=>{document.querySelector('[data-sw-mode="orbit"]').click();aurora.minimizeWindow(cityId)}`)
	page.Timeout(20 * time.Second).MustWait(`()=>!SysWorldApp.inspect(cityId).raf`)
	paused := page.MustEval(`()=>SysWorldApp.inspect(cityId).frames`).Int()
	time.Sleep(200 * time.Millisecond)
	if got := page.MustEval(`()=>SysWorldApp.inspect(cityId).frames`).Int(); got != paused {
		t.Fatal("Hidden city still renders")
	}
	page.MustEval(`()=>aurora.focusWindow(cityId)`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).raf`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{document.querySelector('[data-sw-mode="tour"]').click();if(SysWorldApp.inspect(cityId).mode!=='orbit')throw Error('Reduced motion must suppress the tour');}`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{document.body.dataset.animations='true';aurora.state.bootstrap.settings['appearance.animations']=true;}`)
	page.MustEval(`()=>document.querySelector('[data-sw-mode="tour"]').click()`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).mode==='tour'`)
	page.MustEval(`()=>document.querySelector('[data-sw-district="memory"]').click()`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).mode==='orbit'`)
	page.MustEval(`()=>{window.citySelected=SysWorldApp.inspect(cityId).selected;cityEmit('system_metrics',{cpu:{usage_percent:63},memory:{used_percent:24}});if(SysWorldApp.inspect(cityId).selected!==citySelected)throw Error('Live update moved focus');}`)
	page.MustEval(`()=>{const select=document.querySelector('.sw-quality');select.value='low';select.dispatchEvent(new Event('change'));}`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).tier==='low'&&SysWorldApp.inspect(cityId).cachedModels>20`)
	page.MustScreenshot(filepath.Join(dir, "city-low.png"))
	page.MustEval(`()=>{const select=document.querySelector('.sw-quality');select.value='high';select.dispatchEvent(new Event('change'));document.querySelector('[data-sw-mode="orbit"]').click();}`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).tier==='high'`)
	metrics := page.MustEval(`async()=>{const frames=[];let last=performance.now();for(let i=0;i<120;i++){await new Promise(requestAnimationFrame);const now=performance.now();if(i>10)frames.push(now-last);last=now;}frames.sort((a,b)=>a-b);return JSON.stringify({meanMS:frames.reduce((a,b)=>a+b,0)/frames.length,p95MS:frames[Math.floor(frames.length*.95)],...SysWorldApp.inspect(cityId)},null,2)}`).Str()
	os.WriteFile(filepath.Join(dir, "city-performance.json"), []byte(metrics), 0644)
	page.MustEval(`()=>document.querySelector('[data-sw-mode="map"]').click()`)
	baseline := page.MustEval(`async()=>{const frames=[];let last=performance.now();for(let i=0;i<60;i++){await new Promise(requestAnimationFrame);const now=performance.now();if(i>10)frames.push(now-last);last=now;}return JSON.stringify({mapBaselineMeanMS:frames.reduce((a,b)=>a+b,0)/frames.length},null,2)}`).Str()
	os.WriteFile(filepath.Join(dir, "city-frame-baseline.json"), []byte(baseline), 0644)
	page.MustEval(`()=>document.querySelector('[data-sw-mode="orbit"]').click()`)

	page.MustEval(`()=>{const c=document.querySelector('.sysworld-gl');c.getContext('webgl2').getExtension('WEBGL_lose_context').loseContext()}`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).mode==='map'&&!SysWorldApp.inspect(cityId).raf`)
	page.MustScreenshot(filepath.Join(dir, "city-context-loss.png"))
	os.WriteFile(filepath.Join(dir, "city-diagnostics.json"), []byte(page.MustEval(`()=>JSON.stringify(SysWorldApp.inspect(cityId),null,2)`).Str()), 0644)
	page.MustEval(`async()=>{await aurora.closeWindow(cityId);}`)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId)===null`)
	page.MustEval(`()=>{for(const s of cityHandlers.values())if(s.size)throw Error('Leaked SSE handlers');}`)
	if errors := page.MustEval(`()=>JSON.stringify(cityErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
