package ui

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func verifySystemWorldExpansion(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{document.body.dataset.animations='true';aurora.state.bootstrap.settings['appearance.animations']=true;localStorage.removeItem('aurago.desktop.sysworld.discoveries');}`)
	page.MustSetViewport(1920, 1080, 1, false)
	page.Timeout(40 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.residents===19&&SysWorldApp.inspect(cityId).experience.trams===2`)
	if size := page.MustEval(`()=>SysWorldApp.inspect(cityId).loadedBytes`).Int(); size > 12*1024*1024 {
		t.Fatal("initial load exceeds 12 MiB", size)
	}
	hold := func(key input.Key, d time.Duration) {
		t.Helper()
		page.MustElement(".sysworld-gl").MustFocus()
		if err := page.Keyboard.Press(key); err != nil {
			t.Fatal(err)
		}
		time.Sleep(d)
		if err := page.Keyboard.Release(key); err != nil {
			t.Fatal(err)
		}
	}
	axis := func(index int, target float64, positive, negative input.Key) {
		t.Helper()
		for n := 0; n < 40; n++ {
			p := page.MustEval(`i=>SysWorldApp.inspect(cityId).position[i]`, index).Num()
			if p > target-.35 && p < target+.35 {
				return
			}
			key := positive
			if p > target {
				key = negative
			}
			hold(key, time.Duration(math.Min(65, math.Max(20, math.Abs(p-target)/11*700)))*time.Millisecond)
		}
		page.MustScreenshot(filepath.Join(dir, "world2-navigation-failure.png"))
		t.Fatalf("cannot reach axis %d %.1f: %s", index, target, page.MustEval(`()=>JSON.stringify(SysWorldApp.inspect(cityId))`).Str())
	}
	page.MustEval(`()=>{document.querySelector('[data-sw-action="close"]').click();document.querySelector('.sw-world').open=true;const t=document.querySelectorAll('.sw-world select')[0];t.value='day';t.dispatchEvent(new Event('change'));}`)
	time.Sleep(300 * time.Millisecond)
	page.MustScreenshot(filepath.Join(dir, "world2-day.png"))
	page.MustEval(`()=>{const t=document.querySelectorAll('.sw-world select')[0];t.value='night';t.dispatchEvent(new Event('change'));}`)
	time.Sleep(300 * time.Millisecond)
	page.MustScreenshot(filepath.Join(dir, "world2-night.png"))
	for _, id := range []string{"agent", "memory", "missions"} {
		page.MustEval(`id=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-room="'+id+'"]').click();}`, id)
		page.Timeout(20*time.Second).MustWait(`id=>SysWorldApp.inspect(cityId).experience.rooms.includes(id)`, id)
		doorZ := 67.0
		axis(2, doorZ-.4, input.KeyW, input.KeyS)
		page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='door'`)
		page.MustElement(".sw-interaction").MustClick()
		time.Sleep(650 * time.Millisecond)
		axis(2, doorZ+2, input.KeyW, input.KeyS)
		page.Timeout(10*time.Second).MustWait(`id=>SysWorldApp.inspect(cityId).experience.inside===id`, id)
		page.MustScreenshot(filepath.Join(dir, "world2-interior-"+id+".png"))
		if id == "agent" {
			// Enter through the open LEFT side, not through the lift's guard rails.
			axis(0, 1.5, input.KeyA, input.KeyD)
			axis(2, 76, input.KeyW, input.KeyS)
			axis(0, 4, input.KeyA, input.KeyD)
			page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='lift'`)
			page.MustElement(".sw-interaction").MustClick()
			page.Timeout(12 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).position[1]>6.3&&SysWorldApp.inspect(cityId).experience.ride===null`)
			// Arrival alone missed the original defect. Leave the lift, walk across
			// the actual upper floor and return before requesting the descent.
			axis(0, -2.5, input.KeyA, input.KeyD)
			if y := page.MustEval(`()=>SysWorldApp.inspect(cityId).position[1]`).Num(); y < 6.5 {
				t.Fatal("lost gallery floor after exiting lift", y)
			}
			page.Mouse.MustMoveTo(550, 400).MustDown(proto.InputMouseButtonLeft).MustMoveTo(1806, 600).MustUp(proto.InputMouseButtonLeft)
			time.Sleep(150 * time.Millisecond)
			page.MustScreenshot(filepath.Join(dir, "world2-lift-balcony.png"))
			page.Mouse.MustDown(proto.InputMouseButtonLeft).MustMoveTo(550, 400).MustUp(proto.InputMouseButtonLeft)
			axis(0, 4, input.KeyA, input.KeyD)
			page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='lift'`)
			page.MustElement(".sw-interaction").MustClick()
			page.Timeout(12 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).position[1]<2.6&&SysWorldApp.inspect(cityId).experience.ride===null`)
		}
	}
	// Sky deck: arrive from the exploration panel, use a viewer, then ride the tower lift down and up.
	skyDeck := `()=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-sky]').click();}`
	page.MustEval(skyDeck)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).position[1]>74&&document.querySelector('.sw-interaction').dataset.kind==='telescope'`)
	page.MustScreenshot(filepath.Join(dir, "world2-sky-deck.png"))
	page.MustElement(".sw-interaction").MustClick()
	page.Timeout(10 * time.Second).MustWait(`()=>{const s=SysWorldApp.inspect(cityId).experience.scope;return !!s?.target&&s.fov<16&&!document.querySelector('.sw-scope').hidden&&!document.querySelector('.sw-scope-tag').classList.contains('sw-idle');}`)
	page.MustEval(`()=>{const c=document.querySelector('.sysworld-gl');c.focus();for(let i=0;i<3;i++)c.dispatchEvent(new WheelEvent('wheel',{deltaY:-200,bubbles:true,cancelable:true}));}`)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.scope?.fov<10`)
	page.MustScreenshot(filepath.Join(dir, "world2-telescope.png"))
	page.MustEval(`()=>{const c=document.querySelector('.sysworld-gl');window.scopeHead=document.querySelector('.sw-scope-head span:last-child').textContent;c.dispatchEvent(new KeyboardEvent('keydown',{code:'ArrowLeft',key:'ArrowLeft',bubbles:true}));}`)
	time.Sleep(500 * time.Millisecond)
	page.MustEval(`()=>{const c=document.querySelector('.sysworld-gl');c.dispatchEvent(new KeyboardEvent('keyup',{code:'ArrowLeft',key:'ArrowLeft',bubbles:true}));if(document.querySelector('.sw-scope-head span:last-child').textContent===scopeHead)throw Error('Arrow keys must pan the viewer');c.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));
        const s=SysWorldApp.inspect(cityId);if(s.mode!=='street'||s.experience.scope||!document.querySelector('.sw-scope').hidden)throw Error('Escape must only leave the viewer');}`)
	page.MustEval(skyDeck)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).position[1]>74`)
	// The arrival faces north: S walks south (+Z), D walks east (+X) towards the lift landing.
	axis(2, -12.8, input.KeyS, input.KeyW)
	axis(0, 7.4, input.KeyD, input.KeyA)
	page.Timeout(10 * time.Second).MustWait(`()=>['skycall','skydown'].includes(document.querySelector('.sw-interaction').dataset.kind)`)
	if page.MustEval(`()=>document.querySelector('.sw-interaction').dataset.kind`).Str() == "skycall" {
		page.MustElement(".sw-interaction").MustClick()
		page.Timeout(20 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='skydown'`)
	}
	page.MustElement(".sw-interaction").MustClick()
	time.Sleep(6 * time.Second)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId);if(s.experience.ride!=='sky'||s.position[1]<10||s.position[1]>70)throw Error('Lift ride is not travelling: '+JSON.stringify(s.position));}`)
	page.MustScreenshot(filepath.Join(dir, "world2-sky-lift.png"))
	page.Timeout(25 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).position[1]<4&&SysWorldApp.inspect(cityId).experience.ride===null`)
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='skyup'`)
	page.MustElement(".sw-interaction").MustClick()
	page.Timeout(25 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).position[1]>74&&SysWorldApp.inspect(cityId).experience.ride===null`)
	for _, id := range []string{"agent", "infra", "memory", "missions", "graph", "integrations", "operations"} {
		page.MustEval(`id=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-station="'+id+'"]').click();}`, id)
		page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='discover'`)
		page.MustElement(".sw-interaction").MustClick()
		page.MustEval(`()=>document.querySelector('.sw-world').open=false`)
	}
	page.MustEval(`()=>{if(SysWorldApp.inspect(cityId).experience.discovered.length!==7)throw Error('Missing discovery');document.querySelector('.sw-world').open=true;document.querySelector('[data-world-station="infra"]').click();}`)
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='tram'`)
	page.MustElement(".sw-interaction").MustClick()
	page.Timeout(150 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.ride==='tram'`)
	page.MustScreenshot(filepath.Join(dir, "world2-tram-interior.png"))
	hold(input.KeyW, 400*time.Millisecond)
	page.MustEval(`()=>{window.world2Stops=new Set();window.world2RideAt=performance.now();}`)
	page.Timeout(190 * time.Second).MustWait(`()=>{const s=SysWorldApp.inspect(cityId).experience;world2Stops.add(s.station);return world2Stops.size===7;}`)
	page.MustElement(".sw-interaction").MustClick()
	page.Timeout(45 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.ride===null`)
	page.MustEval(`()=>{document.querySelector('.sw-world').open=true;[...document.querySelectorAll('.sw-world-destinations>button')][0].click();}`)
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='drone'`)
	page.MustElement(".sw-interaction").MustClick()
	time.Sleep(600 * time.Millisecond)
	page.MustEval(`()=>{if(SysWorldApp.inspect(cityId).experience.ride!=='drone')throw Error('No drone ride');}`)
	page.MustScreenshot(filepath.Join(dir, "world2-drone-tour.png"))
	page.MustElement(".sw-interaction").MustClick()
	page.MustEval(`()=>{if(SysWorldApp.inspect(cityId).experience.ride)throw Error('Tour cannot abort');document.querySelector('[data-sw-mode="orbit"]').click();document.querySelector('.sw-world').open=true;const w=document.querySelectorAll('.sw-world select')[1];w.value='rain';w.dispatchEvent(new Event('change'));}`)
	time.Sleep(500 * time.Millisecond)
	page.MustScreenshot(filepath.Join(dir, "world2-rain.png"))
	page.MustEval(`()=>{const s=document.querySelector('.sw-quality');s.value='low';s.dispatchEvent(new Event('change'));}`)
	page.Timeout(20 * time.Second).MustWait(`()=>{const s=SysWorldApp.inspect(cityId);return s.experience.residents===3&&s.experience.trams===1&&s.drones.drones===2}`)
	if errs := page.MustEval(`()=>JSON.stringify(cityErrors)`).Str(); errs != "[]" {
		t.Fatal(errs)
	}
	report := page.MustEval(`()=>JSON.stringify({renderer:SysWorldApp.inspect(cityId).renderer,rideSeconds:(performance.now()-world2RideAt)/1000,stops:[...world2Stops],final:SysWorldApp.inspect(cityId)},null,2)`).Str()
	if err := os.WriteFile(filepath.Join(dir, "world2-exploration.json"), []byte(report), 0600); err != nil {
		t.Fatal(err)
	}
}
