package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopMediaOwnershipBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	result := page.MustEval(`()=>{
        const handlers={},calls=[];
        Object.defineProperty(navigator,'mediaSession',{configurable:true,value:{setActionHandler:(a,h)=>handlers[a]=h}});
        const m=AuraDesktopMediaSession,radio={},tv={},webamp={},personal={};
        const claim=(owner,name,priority)=>m.claim(owner,{priority,metadata:{title:name},handlers:{play:()=>calls.push(name)}});
        const assert=(v)=>{if(!v)throw Error('media ownership failed: '+JSON.stringify(calls));};
        claim(radio,'radio',10);claim(webamp,'webamp',50);claim(tv,'tv',10);handlers.play();assert(calls.pop()==='webamp');
        m.release(tv);handlers.play();assert(calls.pop()==='webamp');
        claim(personal,'personal',100);claim(webamp,'webamp',50);handlers.play();assert(calls.pop()==='personal');
        m.release(personal);handlers.play();assert(calls.pop()==='webamp');
        m.release(webamp);handlers.play();assert(calls.pop()==='radio');
        m.release(radio);assert(handlers.play===null&&navigator.mediaSession.metadata===null);
        return true;
    }`).Bool()
	if !result {
		t.Fatal("media owners interfered")
	}
}

func TestDesktopCityRainLifecycleBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`async()=>{
        window.rainCreated=0;window.rainDestroyed=0;
        document.body.dataset.wallpaper='city_rain';
        let source=await (await fetch('/js/desktop/city-rain-droplets.js')).text();
        source=source.replace('import { createDroplets } from "/js/vendor/canvasui/droplets.js";',
          'const createDroplets=()=>{window.rainCreated++;return {resize(){},setBitmap(){},destroy(){window.rainDestroyed++;}}};');
        source=source.replace('"./wallpaper-visibility.js"',JSON.stringify(location.origin+'/js/desktop/wallpaper-visibility.js'));
        source=source.replace('bitmap = await wallpaperContent();','bitmap = await Promise.resolve({width:100,height:100});');
        source=source.replace('!webgl2Available()','false');
        const blob=URL.createObjectURL(new Blob([source],{type:'text/javascript'}));
        window.rainModule=await import(blob);URL.revokeObjectURL(blob);
    }`)
	page.MustWait(`()=>rainCreated===1`)
	for i := 0; i < 3; i++ {
		page.MustEval(`()=>dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true}))`)
		if !page.MustEval(`()=>rainDestroyed===rainCreated`).Bool() {
			t.Fatal("pagehide leaked rain")
		}
		page.MustEval(`()=>dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true}))`)
		page.MustWait(`()=>rainCreated===rainDestroyed+1`)
	}
	page.MustEval(`()=>{const cover=document.createElement('div');cover.id='audit-cover';cover.className='vd-window maximized';cover.style.cssText='position:fixed;inset:0;width:100vw;height:100vh;opacity:1';document.body.append(cover);}`)
	page.MustWait(`()=>rainCreated===rainDestroyed`)
	page.MustEval(`()=>document.getElementById('audit-cover').remove()`)
	page.MustWait(`()=>rainCreated===rainDestroyed+1`)
	page.MustEval(`()=>{document.body.dataset.wallpaper='aurora';}`)
	page.MustWait(`()=>rainCreated===rainDestroyed`)
}

func TestDesktopChessAudioCleanupBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
        const script=document.createElement('script');script.src='/js/desktop/apps/chess-fx.js';document.head.append(script);await new Promise((r,j)=>{script.onload=r;script.onerror=j;});
        window.audioOpened=0;window.audioClosed=0;
        const Original=AudioContext;
        window.AudioContext=class extends Original{constructor(){super();audioOpened++;}close(){audioClosed++;return super.close();}};
        const audio=createChessAudio();audio.check();audio.promote();audio.gameOver(true);audio.dispose();audio.dispose();audio.move();
        await new Promise(r=>setTimeout(r,500));
        if(audioOpened!==1||audioClosed!==1)throw Error('Chess audio survived dispose');
        window.AudioContext=Original;
    }`)
}

func TestDesktopGalaxaInactiveLoopBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := galaxaArcadeServer(t)
	page := newSmokeBrowser(t).MustPage().Timeout(40 * time.Second)
	defer page.Close()
	page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>window.game?.G.st==='TITLE'||errors.length>0`)
	page.MustEval(`()=>{window.active=false;dispatchEvent(new Event('blur'));}`)
	page.MustEval(`async()=>{const id=game.rafId;await new Promise(r=>setTimeout(r,150));if(id!==0||game.rafId!==0)throw Error('inactive RAF loop');}`)
	page.MustEval(`()=>{active=true;dispatchEvent(new Event('focus'));const id=game.rafId;dispatchEvent(new Event('focus'));document.dispatchEvent(new Event('focusin'));if(!id||game.rafId!==id)throw Error('duplicate RAF restart');}`)
	page.MustEval(`()=>GalaxaDeluxe.dispose('test')`)
	if got := page.MustEval(`()=>JSON.stringify(errors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}

func TestDesktopDetectiveUnsafeLinksBrowser(t *testing.T) {
	page := desktopAuditBrowser(t)
	page.MustEval(`async()=>{
        const script=document.createElement('script');script.src='/js/desktop/apps/detective-views.js';document.head.append(script);await new Promise((r,j)=>{script.onload=r;script.onerror=j;});
        const items=['javascript:alert(1)','//other.example/path','broken','https://user:pass@example.test/','https://example.test/a?b=1'].map(url=>({url,status:'ok'}));
        const el=document.createElement('div');el.innerHTML=DetectiveViews.sources(items,k=>k);
        if(el.querySelectorAll('a').length!==1||el.querySelector('a').getAttribute('href')!=='https://example.test/a?b=1')throw Error('unsafe Detective link');
    }`)
}
