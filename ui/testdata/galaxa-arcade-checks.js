async () => {
 const assert=(value,message)=>{if(!value)throw Error(message)},results=[];
 async function fresh(mode='classic',ship='classic'){
  GalaxaDeluxe.dispose('test');localStorage.setItem('galaxa_settings',JSON.stringify({mode,ship,vol:0,musicVol:0,sfxVol:0,mute:true,crt:false}));
  mount();for(let i=0;i<200&&game.G.st!=='TITLE';i++)await new Promise(r=>setTimeout(r,5));
  assert(game.G.st==='TITLE', 'asset preload failed: '+game.state.loadError);
  cancelAnimationFrame(game.rafId);game.GalagaMusic.stop();game.resetRun(91421);game.startStage();return game;
 }
 let baseline;
 const broken=structuredClone(game.spriteAssets.manifest);delete broken.animations['player.heavy.super'];let rejected=false;
 try{GalaxaCore.validateAtlasManifest(broken)}catch(_){rejected=true}assert(rejected,'incomplete atlas accepted before play');
 for(const fps of [30,60,144]){
  const c=await fresh();c.G.p.inv=1e8;c.G.kb.f=true;
  for(let i=0;i<fps*10;i++){c.G.kb.r=i<fps/2;c.stepFrame(1/fps);c.renderFrame();}
  const signature=JSON.stringify({x:c.G.p.x,y:c.G.p.y,time:c.G.simTime,score:c.G.score,shots:c.G.stageAccuracyShots,enemies:c.G.enemies.map(e=>[e.type,e.st,e.hp,e.x,e.y]),bul:c.G.bul.map(b=>[b.x,b.y])});
  if(baseline)assert(signature===baseline,'simulation differs at '+fps+' FPS');else baseline=signature;
  results.push('fixed step '+fps);
 }
 let c=await fresh(),g=c.G;g.st='PLAYING';g.p.inv=0;g.startShieldHits=0;
 assert(c.hit({x:100,y:100,prevX:100,prevY:300,w:2,h:4},{x:95,y:195,w:12,h:12}),'swept collision misses fast projectile');
 assert(!c.hit({x:150,y:100,prevX:150,prevY:300,w:2,h:4},{x:95,y:195,w:12,h:12}),'swept collision false positive');
 g.parryActive=180;g.ebul=[{x:g.p.x,y:g.p.y-8,vx:0,vy:120,w:3,h:3}];g.bul=[];c.updateBul(1/60);
 assert(g.p.alive&&g.parryCount===1&&g.ebul.length===0&&g.bul.some(b=>b._parried&&b.dmg===4),'parry did not reflect into player bullets');
 const paused=g.simTime;c.pauseForFocus();c.stepFrame(.2);assert(g.simTime===paused&&g.st==='PAUSED','pause advanced simulation');
 let fired=false;c.scheduleGame(()=>fired=true,100);c.stepFrame(.2);assert(!fired,'scheduled gameplay ran while paused');
 g.st='PLAYING';g.hitstopT=0;g.timeScale=1;for(let i=0;i<8;i++)c.stepFrame(1/60);assert(fired,'scheduled gameplay did not resume');
 results.push('collision, parry, pause and game clock');
 c=await fresh();g=c.G;g.st='PLAYING';g.weaponEvo='cannon';const armored=c.newEnemy('hunter',200,300,0,20);armored.x=200;armored.y=300;g.enemies=[armored];g.bul=[{x:200,y:325,w:4,h:8,vx:0,vy:-1500,dmg:8}];c.updateBul(1/60);
 assert(armored.hp===12,'weapon evolution replaced explicit projectile damage');results.push('weapon evolution preserves super/projectile damage');
 for(const stage of [5,10,15,20,25,30]){
  c=await fresh();g=c.G;g.stage=stage;c.startStage();g.st='PLAYING';const boss=g.encounterBoss;boss.bossState='recover';boss.invulnerable=false;
  for(const phase of [2,3]){
   g.ebul=[{x:1,y:1}];g.bossLasers=[{left:1000}];
   for(let n=0;n<10;n++)c.damageEnemy(boss,boss.maxHp);
   c.updateSectorBoss(boss,1/60);
   assert(boss.bossPhase===phase&&boss.invulnerable&&boss.hp>0&&g.ebul.length===0&&g.bossLasers.length===0,'unsafe boss phase '+stage+'/'+phase);
   const hp=boss.hp;c.damageEnemy(boss,99999);assert(boss.hp===hp,'boss transition accepted damage');
   for(let n=0;n<61;n++)c.updateSectorBoss(boss,1/60);
  }
  for(let n=0;n<10;n++)c.damageEnemy(boss,boss.maxHp);
  const score=g.score,kills=g.bossKillTotal;c.updateCampaign(1/60);assert(g.bossDeath&&g.p.inv>=1400&&g.score>score&&g.bossKillTotal===kills+1,'boss death did not reward/clear/protect');
  const awarded=g.score;c.updateCampaign(1/60);assert(g.score===awarded&&g.bossKillTotal===kills+1,'boss reward duplicated');
  c.renderFrame();
 }
 results.push('six bosses, both phase boundaries and death');
 c=await fresh();g=c.G;g.stage=30;g.st='PLAYING';g.score=12345;g.weaponLv=4;g.weaponEvo='cannon';g.activePU={type:'rapid'};g.puTimer=7000;g.p.dual=true;g.stageClearLock=0;
 c.advanceToNextStage(false);assert(g.stage===31&&g.st==='LOOP_CLEAR','stage 30 did not reach loop summary');
 c.openShop();g.credits=1000;const lives=g.lives;c.shopClick(0);c.closeShop();
 assert(g.stage===31&&g.st==='STAGE_INTRO'&&g.score===12345&&g.weaponEvo==='cannon'&&g.p.dual&&g.activePU.type==='rapid'&&g.lives===lives+1,'loop/shop lost run equipment');
 results.push('loop, shop and equipment retention');
 c=await fresh();g=c.G;g.st='PLAYING';g.startShieldHits=0;g.shieldHits=0;g.p.inv=0;g.lives=1;c.killP();
 for(let i=0;i<181;i++)c.stepFrame(1/60);
 assert(g.st==='GAME_OVER','last death did not enter game over');g.kb.s=true;c.stepFrame(1/60);g.kb.s=false;
 assert(g.st==='PLAYING'&&g.p.alive&&g.lives>0,'continue failed');
 c.resetRun(1);c.startStage();assert(g.stage===1&&g.score===0&&g.weaponEvo===null&&g.superPhase==='idle','restart retained stale combat state');
 results.push('death, continue and restart');
 for(const ship of ['classic','interceptor','heavy','stealth']){
  c=await fresh('classic',ship);g=c.G;g.st='PLAYING';g.superMeter=100;assert(c.startSuper(),'super activation '+ship);let shots=0;
  for(let n=0;n<660;n++){c.tickGameSchedule(1000/60);c.updateSupers(1/60);shots=Math.max(shots,g.bul.length+(g.clones||[]).length);}
  assert(shots>0&&g.superPhase==='idle'&&g.superMeter<1,'super lifecycle '+ship+' '+g.superPhase+' '+shots+' '+g.superMeter);
 }
 results.push('four ship supers');
 for(const mode of ['classic','endless','boss_rush','gauntlet','hyperdrive','mirror']){
  c=await fresh(mode);g=c.G;
  for(let stage=1;stage<=12;stage++){
   g.stage=stage;c.startStage();g.p.inv=1e8;
   for(let n=0;n<180;n++)c.stepFrame(1/60);c.renderFrame();
   assert(g.enemies.every(e=>e.type&&Number.isFinite(e.x)&&Number.isFinite(e.y)),'invalid enemy in '+mode+' stage '+stage);
   if(mode==='boss_rush')assert(g.encounterBoss.sectorBoss===GalaxaCore.SECTORS[(stage-1)%6].id,'boss rush order');
  }
  if(mode==='gauntlet'){g.st='PLAYING';g.stageClearLock=0;c.advanceToNextStage(false);assert(g.gauntletComplete&&g.st==='RUN_CLEAR','gauntlet completion');}
  if(mode==='boss_rush'){g.stage=6;g.st='PLAYING';g.stageClearLock=0;c.advanceToNextStage(false);assert(g.st==='RUN_CLEAR','boss rush completion');}
 }
 results.push('all six modes and 12 gauntlet waves');
 let daily;
 for(let run=0;run<2;run++){
  c=await fresh();c.startDailyChallenge();g=c.G;g.p.inv=1e8;g.kb.f=true;
  for(let i=0;i<1200;i++)c.stepFrame(1/60);
  const signature=JSON.stringify({seed:g.runSeed,mods:g.dailyMods.map(m=>m.id),score:g.score,enemies:g.enemies.map(e=>[e.type,e.x,e.y,e.hp]),bullets:g.ebul.map(b=>[b.x,b.y,b.vx,b.vy])});
  if(daily)assert(daily===signature,'daily simulation is not reproducible');else daily=signature;
 }
 results.push('daily deterministic replay');
 const prev=c;c=await fresh();assert(prev.state.disposed&&prev.audioStats().effects===0&&prev.audioStats().music===0,'reopen leaked audio');
 assert(c.settings.vol===0&&c.settings.musicVol===0&&c.settings.sfxVol===0,'saved volume zero lost');
 c.startStage();c.G.st='PLAYING';c.G.kb.r=true;window.active=false;c.pauseForFocus();
 const before=c.G.p.x;c.onKey(new KeyboardEvent('keydown',{key:'ArrowRight'}));c.stepFrame(.1);
 assert(c.G.st==='PAUSED'&&c.G.p.x===before&&!c.G.kb.r,'inactive window consumed input');window.active=true;
 results.push('reopen, volume zero and focus isolation');
 c.G.st='HIGH_SCORE';c.G.ne={ch:[65,65,65],pos:0,done:false};c.showHSOverlay();
 const field=c.overlayEl.querySelector('input');field.value='bob';field.dispatchEvent(new Event('input'));
 c.api=async()=>{throw Error('test offline')};c.overlayEl.querySelector('form').requestSubmit();await new Promise(r=>setTimeout(r,0));
 assert(c.G.st==='HIGH_SCORE'&&!c.G.ne.done&&c.overlayEl.querySelector('[role="alert"]').textContent,'failed highscore silently discarded');
 let saved;c.api=async(path,options)=>{saved={path,body:JSON.parse(options.body)};return []};c.overlayEl.querySelector('form').requestSubmit();await new Promise(r=>setTimeout(r,0));
 assert(saved.path==='/api/desktop/galaxa/highscore/submit'&&saved.body.name==='BOB'&&c.G.st==='TITLE','touch initials or existing highscore endpoint changed');
 let complete;c.api=()=>new Promise(resolve=>complete=resolve);const request=c.submitHS('ACE',10,1);GalaxaDeluxe.dispose('test');complete([]);await request;
 assert(c.state.disposed&&c.MusicEngine.playing===null&&!c.GalagaMusic._shouldPlay,'late score response restarted disposed audio');
 results.push('native initials, highscore retry and late disposal');
 assert(errors.length===0,errors.join('\n'));return results;
}
