package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// slimeStage opens the stage the Green Slime interaction tests and the close-up helper share: the settings
// window (lower) at 200,120 800x620 and the files window at 1100,330 700x500, both at 1920x1080, on the
// WebGL2 gel at full resolution.
func slimeStage(t *testing.T) *themeSweepSession {
	t.Helper()
	s := newSlimeSession(t, "webgl2")
	s.page.MustSetViewport(1920, 1080, 1, false)
	s.store.set(map[string]string{"appearance.theme": "green_slime", "appearance.wallpaper": "green_slime", "windows.animations": "true"})
	s.pushSettings(t)
	s.openApps(t, "settings", "files")
	s.eval(t, `()=>{for(const w of auditTest.state.windows.values()){Object.assign(w.element.style,w.appId==='settings'?{left:'200px',top:'120px',width:'800px',height:'620px'}:{left:'1100px',top:'330px',width:'700px',height:'500px'});}dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	waitSlimeRenderer(t, s, "webgl2")
	s.waitFor(t, "two slime surfaces", 10*time.Second, `()=>AuraDesktopThemeFx.inspect().surfaceCount===2`)
	// The gel oozes in over 400 ms after the build, and a fresh window oozes out of its frame.
	s.waitFor(t, "gel faded in", 10*time.Second, `()=>AuraDesktopThemeFx.inspect().fade>=1`)
	time.Sleep(1200 * time.Millisecond)
	return s
}

type slimeBox struct{ X, Y, W, H, R, B float64 }

func slimeWindowBox(t *testing.T, s *themeSweepSession, appID string) slimeBox {
	t.Helper()
	var box slimeBox
	s.evalInto(t, &box, `(id)=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId===id);const r=w.element.getBoundingClientRect();return {X:r.left,Y:r.top,W:r.width,H:r.height,R:r.right,B:r.bottom};}`, appID)
	return box
}

// Bubbles, strands, merging drops and taskbar splats on the WebGL2 gel, driven through the effect's test
// hooks and with real mouse input. One session, one stage; the subtests run in order.
func TestDesktopGreenSlimeInteractionBrowser(t *testing.T) {
	s := slimeStage(t)
	settings := slimeWindowBox(t, s, "settings")
	taskbar := slimeBox{}
	s.evalInto(t, &taskbar, `()=>{const r=document.querySelector('.vd-taskbar').getBoundingClientRect();return {X:r.left,Y:r.top,W:r.width,H:r.height,R:r.right,B:r.bottom};}`)
	windowID := s.eval(t, `()=>[...auditTest.state.windows.values()].find(w=>w.appId==='settings').id`).Value.Str()
	// The effect only reports sounds to the desktop sound system, which decides what is heard. A stub for
	// DesktopSounds.play records what it was asked for (and plays nothing) for the whole stage.
	s.eval(t, `()=>{window.slimeSounds=[];window.slimeSoundReal=DesktopSounds.play;DesktopSounds.play=function(event,options){slimeSounds.push({event,variant:options&&options.variant,strength:options&&options.strength,pan:options&&options.pan});return Promise.resolve(false);};}`)
	type slimeSoundCall struct {
		Event, Variant string
		Strength, Pan  float64
	}
	heardCalls := func(t *testing.T, event string) (calls []slimeSoundCall) {
		t.Helper()
		var all []slimeSoundCall
		s.evalInto(t, &all, `()=>slimeSounds.slice()`)
		for _, call := range all {
			if call.Event == event {
				calls = append(calls, call)
			}
		}
		return calls
	}

	t.Run("bubble lifecycle", func(t *testing.T) {
		before := s.eval(t, `()=>{const f=AuraDesktopThemeFx.inspect();return {bursts:f.bubbleBursts,spray:f.sprayCount,entries:f.bubbleStageEntries};}`).Value
		// The site is picked at random on the thick parts of the band and can be refused; ask again.
		var id int
		for attempt := 0; attempt < 25 && id <= 0; attempt++ {
			if got := s.eval(t, `(id)=>AuraDesktopThemeFx.command('bubble',{windowId:id,base:9})`, windowID).Value; !got.Nil() {
				id = got.Int()
			} else {
				time.Sleep(300 * time.Millisecond)
			}
		}
		if id <= 0 {
			t.Fatal("the forced bubble was refused 25 times")
		}
		// It runs through nucleate, grow, hold, burst and crater (the stage-entry counters prove each one)
		// and is gone when the crater has healed.
		s.waitFor(t, "the bubble to run through all stages and heal", 25*time.Second, `(id)=>!AuraDesktopThemeFx.inspect().bubbleSamples.some(b=>b.id===id)`, id)
		after := s.eval(t, `()=>{const f=AuraDesktopThemeFx.inspect();return {bursts:f.bubbleBursts,spray:f.sprayCount,entries:f.bubbleStageEntries,active:f.activeBubbles};}`).Value
		for _, stage := range []string{"nucleate", "grow", "hold", "burst", "crater"} {
			if after.Get("entries").Get(stage).Int() <= before.Get("entries").Get(stage).Int() {
				t.Fatalf("the bubble never entered %q: before %s after %s", stage, before.String(), after.String())
			}
		}
		if got := after.Get("bursts").Int() - before.Get("bursts").Int(); got < 1 {
			t.Fatalf("no burst counted: %s", after.String())
		}
		// Three to seven micro-droplets per burst (ambient bubbles may add their own bursts).
		bursts := after.Get("bursts").Int() - before.Get("bursts").Int()
		if spray := after.Get("spray").Int() - before.Get("spray").Int(); spray < 3*bursts || spray > 7*bursts {
			t.Fatalf("%d micro-droplets for %d burst(s)", spray, bursts)
		}
		// Caps: at most 5 bubbles at once and 2 per window, however many are asked for.
		got := s.eval(t, `(id)=>{
			let accepted=0;for(let i=0;i<8;i++)if(AuraDesktopThemeFx.command('bubble',{windowId:id,base:5,fast:true})!==null)accepted++;
			const f=AuraDesktopThemeFx.inspect();const per={};for(const b of f.bubbleSamples)per[b.window]=(per[b.window]||0)+1;
			return {accepted,active:f.activeBubbles,perWindow:Math.max(0,...Object.values(per))};
		}`, windowID).Value
		if got.Get("active").Int() > 5 || got.Get("perWindow").Int() > 2 {
			t.Fatalf("bubble caps escaped: %s", got.String())
		}
		s.waitFor(t, "forced bubbles to burst and heal", 20*time.Second, `()=>AuraDesktopThemeFx.inspect().activeBubbles===0`)
		// Every visible burst asked for one soft slime.pop (a size, a strength of 0.35 to 1, a gentle pan) and
		// nothing else made a sound: not the spray, the crater or the other stages.
		pops := heardCalls(t, "slime.pop")
		burstsSeen := s.eval(t, `()=>AuraDesktopThemeFx.inspect().bubbleBursts`).Value.Int() - before.Get("bursts").Int()
		if len(pops) < 1 || len(pops) > burstsSeen {
			t.Fatalf("%d slime.pop calls for %d bursts", len(pops), burstsSeen)
		}
		for _, call := range pops {
			if (call.Variant != "small" && call.Variant != "medium" && call.Variant != "large") || call.Strength < 0.35 || call.Strength > 1 || call.Pan < -0.8001 || call.Pan > 0.8001 {
				t.Fatalf("odd slime.pop request: %+v", call)
			}
		}
		if got := s.eval(t, `()=>({calls:slimeSounds.length,events:AuraDesktopThemeFx.inspect().soundEvents,other:slimeSounds.filter(c=>c.event!=='slime.pop'&&c.event!=='slime.snap').length})`).Value; got.Get("calls").Int() != len(pops) || got.Get("events").Int() != len(pops) || got.Get("other").Int() != 0 {
			t.Fatalf("sound bookkeeping off: %s", got.String())
		}
	})

	t.Run("strand with the real mouse", func(t *testing.T) {
		var press struct{ X, Y float64 }
		press.X = settings.X + settings.W/2
		press.Y = settings.B + 16
		// The press must land on the bare desktop below the window, not on a window or an icon.
		if !s.eval(t, `(x,y)=>{const el=document.elementFromPoint(x,y);return !!el && !el.closest('.vd-window, .vd-icon, .vd-widget, .vd-taskbar');}`, press.X, press.Y).Value.Bool() {
			t.Fatalf("elementFromPoint(%v,%v) is not the bare desktop", press.X, press.Y)
		}
		before := s.eval(t, `()=>{const f=AuraDesktopThemeFx.inspect();return {strands:f.strandCount,snaps:f.strandSnaps,drops:f.dropCount};}`).Value
		rectBefore := slimeWindowBox(t, s, "settings")
		if err := s.page.Mouse.MoveTo(proto.Point{X: press.X, Y: press.Y}); err != nil {
			t.Fatal(err)
		}
		if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
			t.Fatal(err)
		}
		released := false
		release := func() {
			if !released {
				released = true
				_ = s.page.Mouse.Up(proto.InputMouseButtonLeft, 1)
			}
		}
		defer release()
		for i := 1; i <= 12; i++ {
			if err := s.page.Mouse.MoveTo(proto.Point{X: press.X + float64(i)*4, Y: press.Y + float64(i)*18}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(35 * time.Millisecond)
		}
		// Past about 170 px the thread parts by itself, so it may already be gone when the drag ends.
		s.waitFor(t, "a drawn-out strand", 5*time.Second, `(n)=>AuraDesktopThemeFx.inspect().strandCount>=n+1`, before.Get("strands").Int())
		release()
		s.waitFor(t, "the strand to snap", 5*time.Second, `(n)=>AuraDesktopThemeFx.inspect().strandSnaps>=n+1`, before.Get("snaps").Int())
		s.waitFor(t, "the snapped tip to become a drop", 3*time.Second, `(n)=>AuraDesktopThemeFx.inspect().dropCount>n`, before.Get("drops").Int())
		// The parting thread asked for one slime.snap, panned by where it happened (left of the middle here).
		snaps := heardCalls(t, "slime.snap")
		if len(snaps) != 1 {
			t.Fatalf("%d slime.snap calls for one snapped strand", len(snaps))
		}
		if snap := snaps[0]; (snap.Variant != "small" && snap.Variant != "medium" && snap.Variant != "large") || snap.Strength < 0.35 || snap.Strength > 1 || snap.Pan >= 0 || snap.Pan < -0.8001 {
			t.Fatalf("odd slime.snap request: %+v", snap)
		}
		rectAfter := slimeWindowBox(t, s, "settings")
		if rectBefore != rectAfter {
			t.Fatalf("pulling the gel moved the window: %+v -> %+v", rectBefore, rectAfter)
		}
		if s.eval(t, `()=>!!document.querySelector('[data-selection-marquee]') || document.body.classList.contains('vd-desktop-selecting')`).Value.Bool() {
			t.Fatal("the strand drag left a selection marquee on the desktop")
		}
		if errors := s.eval(t, `()=>fixtureErrors`).Value.String(); errors != "[]" {
			t.Fatalf("errors while pulling a strand: %s", errors)
		}
	})

	t.Run("drops merge and splat", func(t *testing.T) {
		before := s.eval(t, `()=>{const f=AuraDesktopThemeFx.inspect();return {absorbed:f.absorbedDrops,splats:f.splatCount};}`).Value
		// A drop flung at the left flank of the lower window runs into its gel and merges.
		s.eval(t, `(x,y)=>AuraDesktopThemeFx.command('drop',{x,y,vx:170,vy:0,r:6})`, settings.X-70, settings.Y+300)
		s.waitFor(t, "the drop to merge into the gel", 4*time.Second, `(n)=>AuraDesktopThemeFx.inspect().absorbedDrops>n`, before.Get("absorbed").Int())
		// A drop falling onto the taskbar edge splats into a resting blob that clears again.
		s.eval(t, `(x,y)=>AuraDesktopThemeFx.command('drop',{x,y,vx:0,vy:260,r:9})`, 1500.0, taskbar.Y-70)
		// Dripping windows land their own splats on the taskbar now and then, so follow this one by position.
		const resting = `AuraDesktopThemeFx.inspect().splatSamples.some(p=>Math.abs(p.x-1500)<40)`
		s.waitFor(t, "a splat on the taskbar edge", 4*time.Second, `(n)=>AuraDesktopThemeFx.inspect().splatCount>n`, before.Get("splats").Int())
		s.waitFor(t, "the splat to rest at the dropped position", 4*time.Second, `()=>`+resting)
		s.waitFor(t, "the splat to clear", 8*time.Second, `()=>!`+resting)
		if errors := s.eval(t, `()=>fixtureErrors`).Value.String(); errors != "[]" {
			t.Fatalf("errors with drops: %s", errors)
		}
		// Drops, splats and spray are silent: no strand was pulled, so there is still the one snap.
		if snaps := heardCalls(t, "slime.snap"); len(snaps) != 1 {
			t.Fatalf("%d slime.snap calls after drops and splats, want the one from the strand", len(snaps))
		}
	})

	t.Run("bursts are heard through the real sound system", func(t *testing.T) {
		// The real DesktopSounds.play is back, desktop sounds are on with the sound theme left on "Follow
		// desktop theme" (Green Slime here) and a gesture has unlocked the audio context.
		s.eval(t, `()=>{DesktopSounds.play=slimeSoundReal;}`)
		s.store.set(map[string]string{"sound.enabled": "true"})
		s.pushSettings(t)
		s.eval(t, `()=>{document.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));}`)
		s.waitFor(t, "the sound context to unlock", 5*time.Second, `()=>DesktopSounds.inspect().unlocked`)
		if theme := s.eval(t, `()=>DesktopSounds.inspect().theme`).Value.Str(); theme != "green_slime" {
			t.Fatalf("the followed sound theme is %q, want green_slime", theme)
		}
		// The first request may be too old by the time the lazy sound bundle has loaded; the next burst plays.
		s.waitFor(t, "a burst to be played", 30*time.Second, `(id)=>{
			if (DesktopSounds.inspect().themeFx.slime.played >= 1) return true;
			if (AuraDesktopThemeFx.inspect().activeBubbles === 0) AuraDesktopThemeFx.command('bubble', { windowId: id, base: 9, fast: true });
			return false;
		}`, windowID)
		if event := s.eval(t, `()=>DesktopSounds.inspect().lastEvent`).Value.Str(); event != "slime.pop" {
			t.Fatalf("the last sound was %q, want slime.pop", event)
		}
		s.store.set(map[string]string{"sound.enabled": "false"})
		s.pushSettings(t)
	})

	t.Run("context loss falls back to canvas2d", func(t *testing.T) {
		s.eval(t, `()=>{document.querySelector('.vd-fx-slime-canvas').getContext('webgl2').getExtension('WEBGL_lose_context').loseContext();}`)
		s.waitFor(t, "the Canvas 2D fallback", 10*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.renderer==='canvas2d' && f.contextLost && f.ready;}`)
		got := s.eval(t, `()=>({canvases:document.querySelectorAll('.vd-fx-slime-canvas').length,layers:document.querySelectorAll('.vd-theme-fx-layer').length,errors:fixtureErrors})`).Value
		if got.Get("canvases").Int() != 1 || got.Get("layers").Int() != 1 || got.Get("errors").String() != "[]" {
			t.Fatalf("context loss left a bad canvas state: %s", got.String())
		}
	})
}
