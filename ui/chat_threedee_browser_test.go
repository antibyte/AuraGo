package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestThreeDeeCombatBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	shader := readDesktopAssetText(t, "js/chat/threedee-shader.js")
	// Expose closure state only in this fixture, never in the shipped script.
	shader = strings.Replace(shader, "    window.AuraGoThreeDee = {", `
    window.__battle = {
        step: render, hit: applyRobotDamage, shot: spawnEnergyProjectile,
        volley: spawnRobotVolley, blink: startRobotBlink, ghost: spawnBlinkGhost,
        nova: spawnNovaClashExplosion, particles: updateParticles,
        smoke: createSmokeSprite, clock: updateWorldTimeScale,
        get bots() { return robotFleet; }, get sprites() { return sprites; },
        get shots() { return energyProjectiles; }, get ghosts() { return blinkGhosts; },
        get scene() { return scene; }, get renderer() { return renderer; },
        get time() { return globalTime; }, get realTime() { return realTime; },
        tick(dt) { realTime += dt; updateWorldTimeScale(dt); globalTime += dt * worldTimeScale; },
        effects(dt) { updateEnergyProjectiles(dt, globalTime); updateParticles(dt, globalTime); },
        pulse: updateRobotCounterpulse, clear: clearEnergyProjectiles, aim: robotAimPoint,
        duel(dt) { updateFloatingRobot(dt, globalTime); }
    };
    window.AuraGoThreeDee = {`, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html data-theme="threedee"><head><style>body{margin:0;background:#070b14}</style></head><body>
<script>window.requestAnimationFrame=()=>1;window.cancelAnimationFrame=()=>{};window.__errors=[];window.addEventListener('error',e=>__errors.push(e.message));
console.warn=(...args)=>__errors.push(args.join(' '));console.error=(...args)=>__errors.push(args.join(' '));
const nativeMatchMedia=window.matchMedia.bind(window);window.matchMedia=q=>q==='(prefers-reduced-motion: reduce)'?{matches:false,addEventListener(){}}:nativeMatchMedia(q);</script>
<script src="/js/vendor/three.min.js"></script><script src="/js/vendor/GLTFLoader.min.js"></script><script src="/js/vendor/DRACOLoader.min.js"></script><script src="/battle.js"></script></body></html>`))
			return
		}
		if r.URL.Path == "/battle.js" {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(shader))
			return
		}
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	}))
	defer server.Close()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).
		Set("enable-unsafe-swiftshader")
	url := launch.MustLaunch()
	defer launch.Cleanup()
	browser := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer browser.Close()
	page := browser.MustPage("about:blank")
	page.MustSetViewport(1280, 800, 1, false)
	page.MustNavigate(server.URL).MustWaitLoad()
	// Rod's JS wait polls via RAF; this deterministic fixture owns RAF itself.
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if page.MustEval(`() => !!window.AuraGoThreeDee && AuraGoThreeDee.debugState().loadedRobots === 2`).Bool() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !page.MustEval(`() => !!window.AuraGoThreeDee && AuraGoThreeDee.debugState().loadedRobots === 2`).Bool() {
		t.Fatalf("robots failed to load: %s", page.MustEval(`() => ({errors:window.__errors, state:window.AuraGoThreeDee?.debugState()})`).String())
	}
	result := page.MustEval(`() => {
        const b = __battle, a = b.bots[0], target = b.bots[1], v = new THREE.Vector3(1,0,0);
        const lights = () => { let n=0; b.scene.traverse(o=>{if(o.isPointLight)n++}); return n; };
        b.step(16.67); b.step(33.34);
        const before = lights(), programsBefore = b.renderer.info.programs.length;
        const samples=[];
        for(let i=0;i<12;i++) {
            const start=performance.now();
            b.shot(a,target,b.time,i%3===0);
            if(i%3===0) b.hit(target,target.group.position.clone(),v,true);
            b.ghost(a);
            b.step(50+i*16.67);
            samples.push(performance.now()-start);
        }
        const peakLights=lights(), programsAfter=b.renderer.info.programs.length;
        const ghostLit=b.ghosts.some(g=>g.materials.some(m=>m.isMeshStandardMaterial));
        a.state.blinkReadyAt=0;
        const firstBlink=b.blink(a,b.time,1,0);
        for(let i=0;i<60;i++) b.tick(1/60);
        const repeatedBlink=b.blink(a,b.time,1,0);
        b.nova(new THREE.Vector3(0,0,-5),0x22d3ee,0xff4400,b.time);
        for(let i=0;i<180;i++) b.tick(1/60);
        const scaleAfterThreeSeconds=AuraGoThreeDee.debugState().worldTimeScale;
        for(let i=0;i<1200;i++) b.smoke(0,0,-5,0xffffff,0.1,1,{kind:'superTrail'});
        const peakSprites=b.sprites.length;
        b.particles(20,b.time);
        const remainingSprites=b.sprites.length;
        const errors=window.__errors;
        samples.sort((x,y)=>x-y);
        return {before,peakLights,programsBefore,programsAfter,ghostLit,firstBlink,repeatedBlink,
            scaleAfterThreeSeconds,peakSprites,remainingSprites,errors,
            p50ms:samples[6],maxMs:samples[11]};
    }`).Map()
	t.Logf("combat rendering: %s", result)
	if result["peakLights"].Int() != result["before"].Int() {
		t.Error("combat changed the point-light count, causing shader variants")
	}
	if result["ghostLit"].Bool() {
		t.Error("blink ghosts must use unlit materials")
	}
	if !result["firstBlink"].Bool() || result["repeatedBlink"].Bool() {
		t.Error("all blink entry points must enforce the cooldown")
	}
	if result["scaleAfterThreeSeconds"].Num() < 0.98 {
		t.Error("cinematic slow motion did not recover on wall-clock time")
	}
	if result["peakSprites"].Int() > 240 || result["remainingSprites"].Int() > 1 {
		t.Error("particles exceeded the shared budget or failed to expire")
	}
	if len(result["errors"].Arr()) != 0 {
		t.Errorf("browser errors: %v", result["errors"])
	}
	weapons := page.MustEval(`() => {
        const b=__battle, a=b.bots[0], target=b.bots[1];
        b.clear(); b.particles(20,b.time);
        a.state.blinkUntilReal=-999; a.state.blinkStartReal=-999;
        target.state.blinkUntilReal=-999; target.state.shieldUntil=-999;
        a.group.position.set(-2,0,-5); target.group.position.set(2,0,-5);
        const oldAim=b.aim(a); a.group.position.x+=1;
        const aimTracksMove=Math.abs(b.aim(a).x-oldAim.x-1)<0.001;
        a.group.position.x-=1;
        const random=Math.random;
        try {
            Math.random=()=>0.1;
            b.volley(a,target,b.time,false);
            b.effects(1/60);
            const helix=b.shots.length===2 && b.shots.every(p=>p.helixOffset?.length()>0) &&
                b.shots[0].helixOffset.dot(b.shots[1].helixOffset)<0;
            const beforePaused=b.sprites.length;
            Math.random=()=>0.4;
            for(let i=0;i<200;i++) b.effects(0);
            const pausedEmission=b.sprites.length-beforePaused;
            b.clear();
            b.hit(a,a.group.position.clone(),new THREE.Vector3(1,0,0),false);
            const smoke=b.sprites.filter(p=>p.kind==='robotHitSmoke' && p.sprite.material.blending===THREE.NormalBlending).length;
            a.state.hits=5; a.state.empHits=0; a.state.empReadyAt=0;
            target.state.hits=0; target.state.isAiming=true; target.state.isSuperweaponCharging=true;
            b.shot(target,a,b.time,false); const normal=b.shots.at(-1);
            b.shot(target,a,b.time,true); const superShot=b.shots.at(-1);
            b.shot(target,a,b.time,true); const clash=b.shots.at(-1);
            clash.clashTarget=new THREE.Vector3(0,0,-5);
            for(const shot of b.shots) shot.mesh.position.copy(a.group.position).add(new THREE.Vector3(1,0.6,0));
            b.pulse(b.time);
            const emp=normal.source===a && normal.target===target && normal.ricocheted &&
                !b.shots.includes(superShot) && b.shots.includes(clash) &&
                !target.state.isAiming && a.state.empReadyAt>b.time;
            const deadline=a.state.empReadyAt;
            a.state.hits+=5; b.pulse(b.time+0.1);
            const cooldown=a.state.empReadyAt===deadline;
            return {helix,pausedEmission,smoke,emp,cooldown,aimTracksMove};
        } finally { Math.random=random; }
    }`).Map()
	if !weapons["helix"].Bool() || !weapons["emp"].Bool() || !weapons["cooldown"].Bool() || !weapons["aimTracksMove"].Bool() ||
		weapons["smoke"].Int() < 6 || weapons["pausedEmission"].Int() != 0 {
		t.Errorf("weapon, smoke or emission contract failed: %v", weapons)
	}
	// Exercise normal scheduling long enough to cover charges, shields and reloads.
	natural := page.MustEval(`() => {
        const b=__battle;
        for(let i=0;i<1800;i++) { b.tick(1/60); b.duel(1/60); b.particles(1/60,b.time); }
        b.step(1000);
        return {finite:b.bots.every(bot=>Number.isFinite(bot.state.x+bot.state.z+bot.group.position.y)),
            sprites:b.sprites.length,projectiles:b.shots.length,errors:window.__errors};
    }`).Map()
	if !natural["finite"].Bool() || natural["sprites"].Int() > 240 || natural["projectiles"].Int() > 18 || len(natural["errors"].Arr()) != 0 {
		t.Errorf("sustained battle failed: %v", natural)
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		page.MustEval(`() => {
            const b=__battle, a=b.bots[0], target=b.bots[1];
            b.clear(); b.particles(20,b.time);
            b.bots.forEach((bot,i)=>{
                bot.state.x=i===0?-1.8:1.8; bot.state.z=0; bot.state.flightLift=0;
                bot.state.blinkUntilReal=-999; bot.state.blinkStartReal=-999;
                bot.group.position.set(bot.state.x,-1.4,-5.3); bot.velocity.set(0,0);
            });
            a.state.hits+=5; a.state.empReadyAt=0; b.pulse(b.time);
            b.hit(target,b.aim(target),new THREE.Vector3(1,0,0),true);
            const random=Math.random;
            try { Math.random=()=>0.1; b.volley(a,target,b.time,false); } finally { Math.random=random; }
            b.effects(0.18); b.step(1100);
        }`)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "threedee-combat.png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	page.MustEval(`() => AuraGoThreeDee.stop()`)
	if page.MustEval(`() => document.querySelectorAll('#threedee-overlay').length`).Int() != 0 {
		t.Fatal("theme stop left its canvas behind")
	}
	page.MustEval(`() => AuraGoThreeDee.start()`)
	if !page.MustEval(`() => AuraGoThreeDee.debugState().active && __battle.sprites.length === 0 && __battle.shots.length === 0 && __battle.ghosts.length === 0`).Bool() {
		t.Fatal("scene restart retained battle resources")
	}
	page.MustEval(`() => { window.matchMedia=()=>({matches:true}); AuraGoThreeDee.sync(); }`)
	if page.MustEval(`() => AuraGoThreeDee.debugState().active`).Bool() {
		t.Fatal("reduced motion did not stop the scene")
	}
}
