package ui

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func verifySystemWorldExpansion(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	testStart := time.Now()
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{document.body.dataset.animations='true';aurora.state.bootstrap.settings['appearance.animations']=true;localStorage.removeItem('aurago.desktop.sysworld.discoveries');}`)
	page.MustSetViewport(1920, 1080, 1, false)
	page.Timeout(40 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.residents===19&&SysWorldApp.inspect(cityId).experience.trams===2`)
	size := page.MustEval(`()=>SysWorldApp.inspect(cityId).loadedBytes`).Int()
	t.Logf("loaded at first display: %d bytes", size)
	if size > 20*1024*1024 {
		t.Fatal("initial load exceeds 20 MiB", size)
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
	// holdFrames presses a key until the app has drawn at least n more frames, so a step does not depend on the frame
	// rate (a press shorter than one frame moves nothing). A dead page still fails after five seconds.
	holdFrames := func(key input.Key, n int) {
		t.Helper()
		page.MustElement(".sysworld-gl").MustFocus()
		start := page.MustEval(`()=>SysWorldApp.inspect(cityId).frames`).Num()
		if err := page.Keyboard.Press(key); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for page.MustEval(`()=>SysWorldApp.inspect(cityId).frames`).Num() < start+float64(n) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
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
			holdFrames(key, int(math.Min(6, 1+math.Abs(p-target)/2)))
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
			// Enter the lift through its open LEFT side, not through its guard rails. Every leg's
			// ±0.35 m stop window is free: x 0.65–1.35 runs between the hologram (x<0.6) and the
			// divider pillar (x>1.5 near z 73.15); z 74.15–74.85 lies past the pillar and short of
			// the console (z>75.2); x 1.65–2.35 clears the console (x<1.15) and the lift (x≥2.4).
			axis(0, 1, input.KeyA, input.KeyD)
			axis(2, 74.5, input.KeyW, input.KeyS)
			axis(0, 2, input.KeyA, input.KeyD)
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
	// Phase 3: up the spire stairs, through the sliding doors into the agent lobby and out again. The door
	// state is asserted through inspect (a resident standing closer can win the interaction prompt).
	page.MustEval(`()=>{document.querySelector('[data-sw-mode="street"]').click();document.querySelector('[data-sw-district="agent"]').click();}`)
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).lobbies.shown.includes('lobby-agent')`)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.lobbies.ready.includes('agent')`)
	axis(2, -3.2, input.KeyS, input.KeyW)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.lobbies.open.includes('lobby:agent:0')`)
	axis(2, -8, input.KeyS, input.KeyW)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.lobby==='agent'`)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId),k=Math.tan(Math.abs(s.pitch))/Math.tan(s.fov*Math.PI/360);if(!(k<=1/3))throw Error('The lobby view must look into the hall (horizon in the middle third): '+JSON.stringify({pitch:s.pitch,fov:s.fov}));}`)
	page.MustScreenshot(filepath.Join(dir, "world2-lobby-agent.png"))
	axis(2, 4, input.KeyS, input.KeyW)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.lobby===null`)
	t.Logf("loaded after the lobby visit: %d bytes", page.MustEval(`()=>SysWorldApp.inspect(cityId).loadedBytes`).Int())
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
	// The tram wait explains itself: every five seconds the trams' traffic poses (x, z, blocked, obstruction), the visitor
	// and the experience state are sampled; if the ride has not started after 150 s the samples are the failure message.
	tramState := `()=>{const s=SysWorldApp.inspect(cityId),e=s.experience;return JSON.stringify({t:+(s.traffic.time||0).toFixed(1),frames:s.frames,visitor:s.position.map(v=>+v.toFixed(1)),ride:e.ride,station:e.station,interaction:e.interactions,trams:s.traffic.poses.filter(p=>/^tram-/.test(p.id)).map(p=>({id:p.id,x:+p.x.toFixed(1),z:+p.z.toFixed(1),blocked:p.blocked,obstruction:p.obstruction})),near:s.traffic.poses.filter(p=>!/^tram-/.test(p.id)&&s.traffic.poses.some(q=>/^tram-/.test(q.id)&&Math.hypot(p.x-q.x,p.z-q.z)<14)).map(p=>({id:p.id,x:+p.x.toFixed(1),z:+p.z.toFixed(1),blocked:p.blocked,obstruction:p.obstruction})),residents:e.society.residents.filter(r=>s.traffic.poses.some(q=>/^tram-/.test(q.id)&&Math.hypot(r.x-q.x,r.z-q.z)<14)).map(r=>({id:r.id,state:r.state,x:+r.x.toFixed(1),z:+r.z.toFixed(1)})),soundTrams:s.sound&&s.sound.soundscape&&s.sound.soundscape.layers&&s.sound.soundscape.layers.movers?s.sound.soundscape.layers.movers.trams:null,trafficStops:s.traffic.stops,trafficSteps:s.traffic.steps});}`
	tramBegan := time.Now()
	t.Logf("tram wait starts %.0fs into the test: %s", time.Since(testStart).Seconds(), page.MustEval(tramState).Str())
	var tramSamples []string
	for !page.MustEval(`()=>SysWorldApp.inspect(cityId).experience.ride==='tram'`).Bool() {
		if time.Since(tramBegan) > 150*time.Second {
			t.Fatalf("tram ride did not start within 150 s (test time %.0fs); samples every 5 s:\n%s\nfinal: %s", time.Since(testStart).Seconds(), strings.Join(tramSamples, "\n"), page.MustEval(tramState).Str())
		}
		tramSamples = append(tramSamples, fmt.Sprintf("+%.0fs %s", time.Since(tramBegan).Seconds(), page.MustEval(tramState).Str()))
		time.Sleep(5 * time.Second)
	}
	t.Logf("tram wait took %.1fs", time.Since(tramBegan).Seconds())
	page.MustScreenshot(filepath.Join(dir, "world2-tram-interior.png"))
	hold(input.KeyW, 400*time.Millisecond)
	page.MustEval(`()=>{window.world2Stops=new Set();window.world2RideAt=performance.now();}`)
	page.Timeout(190 * time.Second).MustWait(`()=>{const s=SysWorldApp.inspect(cityId).experience;world2Stops.add(s.station);return world2Stops.size===7;}`)
	page.MustElement(".sw-interaction").MustClick()
	page.Timeout(45 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.ride===null`)
	// Hover pack at the southern dock (hoverPads[0], front towards the boulevard): strap it on, climb, fly,
	// glide down and find the pack back in its dock. All seven discoveries are done, so the operations stop
	// arrives at (0, 64.5) facing south: W walks south (+Z), A east (+X), D west (-X), S north (-Z).
	page.Timeout(20 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.hover.pads===5`)
	page.MustEval(`()=>{document.querySelector('.sw-world').open=true;document.querySelector('[data-world-station="operations"]').click();}`)
	page.Timeout(10 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.ride===null&&Math.abs(SysWorldApp.inspect(cityId).position[2]-64.5)<.5`)
	// West off the platform, north to the dock's row, west again: the dock itself fills x -8.5..-7.1, z 63.25..64.25.
	axis(0, -5.3, input.KeyA, input.KeyD)
	axis(2, 62.35, input.KeyW, input.KeyS)
	axis(0, -7.8, input.KeyA, input.KeyD)
	page.Timeout(10 * time.Second).MustWait(`()=>document.querySelector('.sw-interaction').dataset.kind==='hoverpack'`)
	hold(input.KeyE, 60*time.Millisecond)
	page.Timeout(5 * time.Second).MustWait(`()=>{const s=SysWorldApp.inspect(cityId).experience;return s.ride==='hover'&&s.hover.docked===4&&s.hover.flying;}`)
	hold(input.Space, 1500*time.Millisecond)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId),f=document.querySelector('.sw-flight'),k=document.querySelector('[data-sw-action="flight-up"]');
        if(s.experience.hover.altitude<3||s.position[1]<5)throw Error('Space must lift the visitor: '+JSON.stringify(s.experience.hover));
        if(f.hidden||!f.getBoundingClientRect().width||!/ m$/.test(f.querySelector('.sw-flight-stat strong').textContent))throw Error('The flight readout must show while flying');
        if(getComputedStyle(k).display==='none'||k.getBoundingClientRect().width<30)throw Error('Climb and descend buttons belong to the flight');
        window.hoverFrom=s.position;}`)
	hold(input.KeyW, 600*time.Millisecond)
	page.MustEval(`()=>{const p=SysWorldApp.inspect(cityId).position,q=window.hoverFrom;if(Math.hypot(p[0]-q[0],p[2]-q[2])<2.5)throw Error('W must fly forward: '+JSON.stringify([q,p]));window.hoverFrom=p;}`)
	// The touch climb button is the same pointer-capture pad as the arrows.
	up := page.MustEval(`()=>{const r=document.querySelector('[data-sw-action="flight-up"]').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}`).Arr()
	page.Mouse.MustMoveTo(up[0].Num(), up[1].Num()).MustDown(proto.InputMouseButtonLeft)
	time.Sleep(700 * time.Millisecond)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	page.MustEval(`()=>{if(SysWorldApp.inspect(cityId).position[1]<window.hoverFrom[1]+1)throw Error('The climb button must lift the visitor');}`)
	page.MustScreenshot(filepath.Join(dir, "city-hover-flight.png"))
	hold(input.KeyE, 60*time.Millisecond)
	page.Timeout(5 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.hover.landing&&document.querySelector('.sw-flight-state').textContent!==''&&!document.querySelector('.sw-interaction').offsetParent`)
	page.Timeout(60 * time.Second).MustWait(`()=>SysWorldApp.inspect(cityId).experience.ride===null`)
	page.MustEval(`()=>{const s=SysWorldApp.inspect(cityId),y=s.position[1];
        if(s.experience.hover.docked!==5||s.experience.hover.flying)throw Error('The pack must be back in its dock: '+JSON.stringify(s.experience.hover));
        if(y<2.3||y>3.4)throw Error('Landing must end at street eye height: '+y);
        if(!document.querySelector('.sw-flight').hidden||getComputedStyle(document.querySelector('[data-sw-action="flight-up"]')).display!=='none')throw Error('The flight panel and its buttons must be gone');}`)
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
