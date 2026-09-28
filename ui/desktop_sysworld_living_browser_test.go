package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func verifySystemWorldLiving(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustSetViewport(1366, 900, 1, false)
	page.Timeout(40 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.society.places.length===6`)
	page.MustEval(`()=>{document.body.dataset.animations='true';document.querySelector('[data-sw-action="close"]').click();const time=document.querySelectorAll('.sw-world select')[0];time.value='day';time.dispatchEvent(new Event('change'));}`)
	for _, id := range []string{"repair-bay", "parcel-sorter", "relay-mast", "kinetic-fountain", "glass-garden", "meeting-charge"} {
		page.MustEval(`id=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-place="'+id+'"]').click();}`, id)
		page.Timeout(20*time.Second).MustWait(`id=>SysWorldApp.inspect(cityId).experience.details.some(p=>p.id===id&&p.level===0)`, id)
		page.Timeout(10 * time.Second).MustWait(`()=>!document.querySelector('.sw-interaction').hidden`)
		page.MustElement(".sw-interaction").MustClick()
		page.MustEval(`()=>{if(/sysworld\.world\./.test(document.querySelector('.sw-world').innerText))throw Error('Untranslated local interaction');}`)
		page.MustEval(`()=>{const panel=document.querySelector('.sw-world').getBoundingClientRect(),help=document.querySelector('.sw-street-help').getBoundingClientRect();if(panel.bottom>help.top-6)throw Error('Exploration panel covers walking controls');}`)
		page.MustEval(`()=>{if(getComputedStyle(document.querySelector('.sw-interaction')).display!=='none')throw Error('Interaction prompt overlays the open panel');}`)
		time.Sleep(600 * time.Millisecond)
		page.MustScreenshot(filepath.Join(dir, "living-"+id+".png"))
		page.MustElement("[data-world-close]").MustClick()
		page.MustEval(`()=>{if(document.querySelector('.sw-world').open||document.activeElement!==document.querySelector('.sysworld-gl'))throw Error('Closing exploration must restore walking focus');}`)
	}
	// Meet a real resident using the same E/touch controls as the user.
	page.MustEval(`()=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-place="meeting-charge"]').click();}`)
	// Approach the nearest worker through normal walking and pointer controls;
	// loading time must not determine whether one happens to pass the visitor.
	page.MustElement(`[data-sw-action="pointer-lock"]`).MustClick()
	page.Timeout(5 * time.Second).MustWait(`()=>document.pointerLockElement===document.querySelector('.sysworld-gl')`)
	page.MustEval(`()=>{window.livingYaw=0;}`)
	if err := page.Timeout(40 * time.Second).Wait(rod.Eval(`async()=>{
		const s=SysWorldApp.inspect(cityId),canvas=document.querySelector('.sysworld-gl'),near=s.experience.interactions;
		if(near?.kind==='resident'&&near.id.startsWith('resident-')){canvas.dispatchEvent(new KeyboardEvent('keydown',{key:'e',code:'KeyE',bubbles:true}));return true;}
		const distance=r=>Math.hypot(r.x-s.position[0],r.z-s.position[2]);
		const worker=s.experience.society.residents.filter(r=>r.id.startsWith('resident-')).sort((a,b)=>distance(a)-distance(b))[0];
		if(!worker)return false;
		const yaw=Math.atan2(s.position[0]-worker.x,s.position[2]-worker.z),delta=Math.atan2(Math.sin(yaw-livingYaw),Math.cos(yaw-livingYaw)),dx=Math.round(-delta/.0025);
		canvas.dispatchEvent(new PointerEvent('pointermove',{movementX:dx}));livingYaw-=dx*.0025;
		canvas.dispatchEvent(new KeyboardEvent('keydown',{key:'w',code:'KeyW',bubbles:true}));
		await new Promise(resolve=>setTimeout(resolve,120));
		canvas.dispatchEvent(new KeyboardEvent('keyup',{key:'w',code:'KeyW',bubbles:true}));return false;
	}`).ByPromise()); err != nil {
		page.MustScreenshot(filepath.Join(dir, "living-encounter-failed.png"))
		t.Fatalf("Approach resident: %v; %s", err, page.MustEval(`()=>JSON.stringify({position:SysWorldApp.inspect(cityId).position,near:SysWorldApp.inspect(cityId).experience.interactions,residents:SysWorldApp.inspect(cityId).experience.society.residents,yaw:livingYaw,lock:!!document.pointerLockElement,errors:cityErrors})`).Str())
	}
	page.Timeout(5 * time.Second).MustWait(`()=>document.pointerLockElement===null&&!!document.querySelector('.sw-society select')`)
	page.MustEval(`()=>{document.querySelector('.sw-society select').value='missions';}`)
	page.MustElement(".sw-society .sw-world-verbs button").MustClick()
	page.MustEval(`()=>{if(!SysWorldApp.inspect(cityId).experience.society.residents.some(r=>r.state==='greet'))throw Error('Greeting did not stop a resident');}`)
	page.MustEval(`()=>{if(document.activeElement.dataset.worldSocial!=='greet'||document.querySelector('.sw-society select').value!=='missions')throw Error('Greeting lost keyboard focus or guide destination');}`)
	page.MustEval(`()=>{const s=document.querySelector('.sw-society select');if(s.getBoundingClientRect().width<s.parentElement.getBoundingClientRect().width-2)throw Error('Guide destination should fill the field');}`)
	page.MustScreenshot(filepath.Join(dir, "living-greeting.png"))
	page.MustSetViewport(1366, 768, 1, false)
	page.MustEval(`()=>{const panel=document.querySelector('.sw-world').getBoundingClientRect(),help=document.querySelector('.sw-street-help').getBoundingClientRect();if(panel.bottom>help.top-6)throw Error('Short desktop panel covers walking controls');}`)
	page.MustEval(`()=>{const s=document.querySelector('.sw-society select');if(!s)throw Error('Missing guide destinations');s.value='missions';document.querySelector('.sw-society>button').click();window.livingCamera=JSON.stringify(SysWorldApp.inspect(cityId).position);}`)
	page.Timeout(5 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.society.residents.some(r=>r.guided)`)
	time.Sleep(700 * time.Millisecond)
	page.MustEval(`()=>{if(JSON.stringify(SysWorldApp.inspect(cityId).position)!==livingCamera)throw Error('Guide took camera control');}`)
	page.MustEval(`()=>{document.querySelector('[data-sw-mode="orbit"]').click();document.querySelector('.sw-world').open=true;document.querySelector('[data-world-place="kinetic-fountain"]').click();fixtureTheme('fruity-light');}`)
	page.MustSetViewport(430, 932, 2, true)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{const key=document.querySelector('.sw-interaction kbd');if(getComputedStyle(key).display!=='none')throw Error('Touch prompt should not require a keyboard');}`)
	page.MustScreenshot(filepath.Join(dir, "living-touch.png"))
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{window.livingStill=SysWorldApp.inspect(cityId).experience.machinery.time;}`)
	time.Sleep(400 * time.Millisecond)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.experience.machinery.time!==livingStill||s.experience.society.signals.active!==0)throw Error('Reduced motion did not freeze local effects');}`)
	page.MustElement(".sw-interaction").MustClick()
	page.MustEval(`()=>{const panel=document.querySelector('.sw-world').getBoundingClientRect(),help=document.querySelector('.sw-street-help').getBoundingClientRect();if(panel.bottom>help.top-6)throw Error('Touch exploration covers walking controls');}`)
	page.MustScreenshot(filepath.Join(dir, "living-reduced.png"))
	page.MustEval(`()=>{document.querySelector('[data-world-close]').focus();document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));if(document.querySelector('.sw-world').open||document.activeElement!==document.querySelector('.sysworld-gl'))throw Error('Escape did not return to exploration');}`)
	if errs := page.MustEval(`()=>JSON.stringify(cityErrors)`).Str(); errs != "[]" {
		t.Fatal(errs)
	}
	report := page.MustEval(`()=>JSON.stringify(SysWorldApp.inspect(cityId),null,2)`).Str()
	if err := os.WriteFile(filepath.Join(dir, "living-acceptance.json"), []byte(report), 0600); err != nil {
		t.Fatal(err)
	}
}
