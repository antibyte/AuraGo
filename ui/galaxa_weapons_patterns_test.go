package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Galaxa Deluxe weapon evolutions (wave/flak/rail), new alien attack patterns,
// dive paths and sprite heading. Marker style as galaxa_enemy_fx_test.go.

func TestGalaxaWeaponEvolutionsPresent(t *testing.T) {
	t.Parallel()
	checks := []struct {
		file    string
		markers []string
		absent  []string
	}{
		{"js/desktop/apps/galaxa-entities.js", []string{"mode: 'wave'", "mode: 'flak'", "mode: 'rail'", "ctx.WEAPON_EVO_IDS = WEAPON_EVO_IDS"}, nil},
		{"js/desktop/apps/galaxa-entities-weapons.js", []string{"fireEvoWeapon", "fireRailLance", "flakBurst"}, nil},
		{"js/desktop/apps/galaxa-render-world.js", []string{"drawEvoBullet(", "G.railBeams", "c.rotate(rot)", "drawShip("}, nil},
		{"js/desktop/apps/galaxa-audio-sfx.js", []string{"kind === 'wave'", "kind === 'flak'", "kind === 'rail'", "kind === 'vulcan'", "kind === 'cannon'",
			"flakBurst(panX) { if (ctx.G.muted) return;", "lobShot(panX) { if (ctx.G.muted) return;", "pincerShot(panX) { if (ctx.G.muted) return;"}, nil},
		{"js/desktop/apps/galaxa-constants.js", []string{"fan_sweep:", "pincer:", "lob:", "rain:", "predictive:", "GC.ENEMY_HEADING", "loop: { stageMin"}, nil},
		{"js/desktop/apps/galaxa-enemy-motion.js", []string{"updateEnemyHeading", "e.st === 'FEINT'", "GC.ENEMY_BIOME_TINT"}, nil},
		{"js/desktop/apps/galaxa-entities-behaviors.js", []string{"k === 'loop'", "k === 'strafe'", "k === 'hover'", "P.predictive("}, []string{"rotTarget"}},
		{"lang/desktop/en.json", []string{`"galaxa.evo_rail"`}, nil},
	}
	for _, check := range checks {
		text := readEmbeddedText(t, check.file)
		for _, marker := range check.markers {
			if !strings.Contains(text, marker) {
				t.Errorf("%s missing %q", check.file, marker)
			}
		}
		for _, marker := range check.absent {
			if strings.Contains(text, marker) {
				t.Errorf("%s must no longer contain %q", check.file, marker)
			}
		}
	}
}

const galaxaWeaponsPatternsProbe = `()=>{
 const c=game,g=c.G,out={checks:{}},ok=(k,v)=>{out.checks[k]=!!v};
 const sim=n=>{for(let i=0;i<n;i++){g.p.inv=1e9;g.p.alive=true;c.stepFrame(1/60);}};
 c.settings.mode='classic';c.resize();c.resetRun(7);g.stage=7;c.startStage();g.st='PLAYING';
 for(let i=0;i<900&&!g.enemies.some(e=>e.st==='FORM');i++)sim(1);
 sim(120);
 out.formCount=g.enemies.filter(e=>e.st==='FORM').length;
 ok('formation',out.formCount>0);
 out.weapons={};
 for(const id of ['wave','flak','rail']){
  const w={};
  g.weaponEvo=id;g.weaponLv=4;g.p.alive=true;g.bul=[];g.railBeams=[];
  g.simTime+=3000;g.inp.f=true;c.fire(g.simTime);g.inp.f=false;
  w.wave=g.bul.filter(b=>b.wave).length;w.flak=g.bul.filter(b=>b.flak).length;w.rail=(g.railBeams||[]).length;w.total=g.bul.length;
  let threw=null;
  try{for(let i=0;i<40;i++){g.simTime+=16;c.updateBul(1/60);}c.renderFrame();}catch(e){threw=String(e&&e.stack||e);}
  w.threw=threw;w.after=g.bul.length;w.shards=g.bul.filter(b=>b.flakShard).length;
  out.weapons[id]=w;
  ok('weapon-'+id,!threw&&(id==='wave'?w.wave>0:id==='flak'?w.flak>0:(w.rail>0||w.total>0)));
  if(id==='flak')ok('flak-shell-or-shards',w.shards>0||g.bul.some(b=>b.flak));
 }
 g.bul=[];g.railBeams=[];
 out.rot={};
 for(const kind of ['loop','strafe','hover']){
  const e=g.enemies.find(x=>x.st==='FORM'&&!x.sectorBoss&&!['spinner','hunter','stalker','kamikaze','boss','miniboss'].includes(x.type));
  if(!e){out.rot[kind]={skipped:true};continue;}
  const r0=e.rot;let changed=false;
  e.st='DIVING';e.dPath={ph:0,kind,amp:40,vx:60};e.dTmr=3000;e.sTmr=99999;
  for(let i=0;i<90;i++){g.p.inv=1e9;c.updateE(1/60);if(e.rot!==r0)changed=true;}
  out.rot[kind]={rot:e.rot,changed,y:e.y,st:e.st};
  ok('rot-'+kind,typeof e.rot==='number'&&isFinite(e.rot)&&changed);
  e.st='RETURN';
 }
 const h=g.enemies.find(e=>e.st==='FORM'&&e.type==='hunter');
 if(h){h.st='DIVING';h.dPath={ph:0,kind:'sine',amp:20,vx:0};h.dTmr=4000;h.sTmr=99999;g.p.y=c.H-60;for(let i=0;i<60;i++){g.p.inv=1e9;c.updateE(1/60);}out.hunterRot=h.rot;ok('hunter-faces-player',Math.abs(h.rot)>0.5);}
 else out.hunterRot='skipped';
 g.stage=9;out.patterns={};
 for(const type of ['spinner','bomber','carrier']){
  let e=g.enemies.find(x=>x.type===type&&x.st!=='DEAD');
  if(!e){e=c.newEnemy(type,270,200,0,0);e.st='DIVING';g.enemies.push(e);}
  let grew=false;
  for(let i=0;i<40&&!grew;i++){g.ebul.length=Math.min(g.ebul.length,100);const m=g.ebul.length;e.shotReady=true;c.enemyFire(e);e.shotReady=false;if(g.ebul.length>m)grew=true;}
  out.patterns[type]=grew;ok('fire-'+type,grew);
 }
 ok('lob-gravity',c.ATTACK_PATTERNS.lob({x:200,y:100},3,0.5,260,Math.random).every(b=>b.ay>0));
 out.errors=window.errors.slice();
 return JSON.stringify(out);
}`

func TestGalaxaWeaponsPatternsBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	server := galaxaArcadeServer(t)
	page := newGalaxaSmokeBrowser(t).NoDefaultDevice().MustPage(server.URL + "/fixture").Timeout(120 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	waitForJSBool(t, page, `()=>game?.G.st==='TITLE' || errors.length>0`)
	page.MustEval(`()=>{cancelAnimationFrame(game.rafId);game.GalagaMusic.stop();game.G.muted=true;game.settings.crt=false;}`)

	raw := page.MustEval(galaxaWeaponsPatternsProbe).Str()
	var result struct {
		Errors []string        `json:"errors"`
		Checks map[string]bool `json:"checks"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode probe result: %v (%s)", err, raw)
	}
	t.Log(raw)
	if len(result.Errors) > 0 {
		t.Fatalf("page errors: %v", result.Errors)
	}
	if len(result.Checks) == 0 {
		t.Fatal("probe produced no checks")
	}
	for name, passed := range result.Checks {
		if !passed {
			t.Errorf("check failed: %s", name)
		}
	}

	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	os.MkdirAll(dir, 0755)
	for _, id := range []string{"wave", "flak", "rail"} {
		page.MustEval(`id=>{const c=game,g=c.G;g.st='PLAYING';g.weaponEvo=id;g.weaponLv=4;g.bul=[];g.railBeams=[];g.p.alive=true;g.p.inv=1e9;g.p.x=270;g.p.y=620;
 g.simTime+=3000;for(let k=0;k<2;k++){g.inp.f=true;c.fire(g.simTime+k*1500);g.inp.f=false;}
 for(let i=0;i<10;i++){g.simTime+=16;c.updateBul(1/60);}c.renderFrame()}`, id)
		os.WriteFile(filepath.Join(dir, "galaxa-weapons-"+id+".png"), page.MustScreenshot(), 0644)
	}
	page.MustEval(`()=>{const c=game,g=c.G;g.weaponEvo=null;g.bul=[];g.ebul=[];g.st='PLAYING';g.p.inv=1e9;
 const forms=g.enemies.filter(e=>(e.st==='FORM'||e.st==='RETURN')&&!e.sectorBoss);
 forms.slice(0,8).forEach((e,i)=>{e.st='DIVING';e.dPath={ph:i,kind:['loop','strafe','hover','sine','zigzag','arc'][i%6],amp:40,vx:(i%2?60:-60)};e.dTmr=4000;e.sTmr=99999;});
 for(let i=0;i<70;i++){g.p.inv=1e9;c.stepFrame(1/60);}c.renderFrame()}`)
	os.WriteFile(filepath.Join(dir, "galaxa-rotation.png"), page.MustScreenshot(), 0644)
	if errs := page.MustEval(`()=>errors`).String(); errs != "[]" {
		t.Fatal(errs)
	}
}
