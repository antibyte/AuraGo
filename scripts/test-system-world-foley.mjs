import assert from 'node:assert/strict';
import {createCityFoley} from '../ui/js/desktop/apps/sysworld-foley.js';
import {createCityAmbience,citySoundscapeEvents} from '../ui/js/desktop/apps/sysworld-audio.js';
import {createFakeAudio} from './system-world-audio-fake.mjs';

// The shared deterministic Web Audio double (scripts/system-world-audio-fake.mjs).
const native={setTimeout,clearTimeout,window:globalThis.window,localStorage:globalThis.localStorage};
const fake=createFakeAudio(),{context,nodes,node,flush}=fake;
const kinds=()=>nodes.filter(n=>n.kind==='pan').map(n=>n.pan.value);

try{
  globalThis.setTimeout=fake.timers.setTimeout;
  globalThis.clearTimeout=fake.timers.clearTimeout;
  globalThis.window={AudioContext:function(){return context;}};
  globalThis.localStorage={getItem:()=>null,setItem:()=>{}};

  // --- Robot foley -------------------------------------------------------
  const foley=createCityFoley(context,node('out'),node('ambience'));
  // The listener sits on the street; the origin is the place the view rests on.
  foley.listener(0,0,0,0,-1,0,0);
  assert.equal(foley.stats().voices,0,'No voice before the city is audible');
  assert.equal(foley.action('robot_greet',2,0,0),false,'A quiet city makes no robot sound');
  foley.setActive(true);
  assert.equal(foley.action('robot_greet',2,0,0),true,'An audible city lets a resident answer');
  assert.equal(foley.stats().played,1);
  // The robot layer has to be heard over the city bed, whose mixed RMS is about
  // .01. A sound that stays below that is swallowed by the drones and the murmur.
  assert.ok(foley.stats().peak>.15,'A resident beside the listener is loud enough to hear');
  assert.ok(kinds().at(-1)>.9,'A resident beside the listener sits to the right of the view');
  assert.equal(foley.action('robot_greet',100,0,100),false,'A distant resident stays silent');
  // A resident of the district you look at stays audible even from far above.
  foley.listener(0,0,0,0,-1,0,0);
  assert.equal(foley.action('robot_greet',40,0,20),true,'A resident of the district in view is heard');
  flush();
  assert.equal(foley.stats().voices,0,'A finished sound releases its voices');
  for(let i=0;i<24;i++)foley.action('robot_work',3,0,0,'r'+i);
  assert.equal(foley.stats().voices,10,'The voice pool stays bounded');
  flush();
  // A nearer resident takes the voice from a farther one.
  for(let i=0;i<10;i++)foley.action('robot_work',40,0,0,'far'+i);
  assert.equal(foley.stats().voices,10);
  assert.equal(foley.action('robot_work',2,0,0,'near'),true,'A nearer resident gets a voice');
  flush();
  // One resident answers at most every RETRIGGER seconds.
  context.currentTime=1;foley.action('robot_greet',3,0,0,'same');
  context.currentTime=1.1;
  assert.equal(foley.action('robot_greet',3,0,0,'same'),false,'One resident cannot retrigger itself');
  context.currentTime=2;
  assert.equal(foley.action('robot_greet',3,0,0,'same'),true,'The same resident may answer again later');
  flush();

  // The footfall cadence follows the travelled distance, never the frame rate.
  foley.reset();
  foley.gait('a',0,0,0,false,false);
  foley.gait('a',.3,0,0,false,false);
  assert.equal(foley.stats().steps,0,'A standing resident is silent');
  foley.gait('a',.3,0,0,true,false);
  assert.equal(foley.stats().steps,0,'Starting to move without distance makes no footfall');
  foley.gait('a',1.3,0,0,true,false);
  assert.equal(foley.stats().steps,1,'One metre of travel produces one footfall');
  foley.gait('a',2.1,0,0,true,false);
  assert.equal(foley.stats().steps,2,'The cadence follows the travelled distance');
  foley.gait('a',2.1,0,0,false,false);
  foley.gait('a',12,0,0,true,false);
  assert.equal(foley.stats().steps,2,'A respawn is not a stride');
  foley.gait('b',0,0,0,true,true);
  foley.gait('b',.9,0,0,true,true);
  assert.equal(foley.stats().steps,2,'A carried parcel steps on a longer stride');
  foley.gait('b',1.9,0,0,true,true);
  assert.equal(foley.stats().steps,3,'The carried stride completes');
  foley.gait('c',0,0,0,true,false,'glide');
  foley.gait('c',1.7,0,0,true,false,'glide');
  assert.equal(foley.stats().steps,4,'A gliding resident keeps its own longer stride');
  // A footfall beside the listener is the quietest sound of the layer and still
  // has to be louder than the city bed.
  assert.ok(foley.stats().peak>.14,'A footfall beside the listener is loud enough to hear');
  flush();
  foley.setActive(false);
  assert.equal(foley.stats().voices,0,'A muted city releases every robot voice');

  // --- City soundscape ----------------------------------------------------
  const ambience=createCityAmbience(()=>{});
  ambience.toggle();
  ambience.setActive(true);
  ambience.setListener(0,2.4,0,0,-1);
  assert.ok(ambience.stats().soundscape.scheduled,'The soundscape schedules while audible');
  ambience.effect('robot_greet',2,0,0,'resident-1');
  assert.equal(ambience.stats().foley.played,1,'A robot action reaches the foley');
  ambience.effect('door',2,0,0);
  assert.equal(ambience.stats().foley.played,1,'A world interaction never reaches the foley');
  ambience.gait('resident-1',0,0,0,false,false);
  ambience.gait('resident-1',1,0,0,true,false);
  assert.equal(ambience.stats().foley.steps,1,'The stride reaches the foley');
  ambience.setListener(0,2.4,76,0,-1,0,76);
  assert.ok(ambience.stats().soundscape.beds.water>.04,'Harbour water is loud at the quay');
  ambience.setListener(0,2.4,0,0,-1,0,0);
  assert.ok(ambience.stats().soundscape.beds.water<.01,'Water fades inland');
  // The lantern hum is a dusk/night bed at the quay, never a daytime or inland layer.
  ambience.setMood({day:0});
  ambience.setListener(0,2.4,76,0,-1,0,76);
  assert.ok(ambience.stats().soundscape.beds.lamp>0,'Warm lantern bed opens at the quay at night');
  ambience.setMood({day:1});
  assert.equal(ambience.stats().soundscape.beds.lamp,0,'Lantern bed is silent at noon');
  ambience.setMood({day:0});
  ambience.setListener(0,2.4,0,0,-1,0,0);
  assert.equal(ambience.stats().soundscape.beds.lamp,0,'Lantern bed fades inland');
  ambience.setActive(false);
  assert.equal(ambience.stats().soundscape.scheduled,false,'A hidden city schedules nothing');
  ambience.dispose();

  // --- The event table ----------------------------------------------------
  const offered=mood=>new Set(citySoundscapeEvents(mood).map(e=>e.kind));
  const day=offered({day:1,evening:0,weather:'clear'});
  const night=offered({day:0,evening:0,weather:'clear'});
  const rain=offered({day:1,evening:0,weather:'rain'});
  const evening=offered({day:0,evening:1,busy:true,weather:'clear'});
  assert.ok(day.has('birds')&&day.has('gull')&&day.has('pigeon'),'A clear day answers with birds, gulls and pigeons');
  assert.ok(!day.has('cricket')&&!night.has('birds'),'Day and night do not share the same animals');
  assert.ok(night.has('cricket')&&night.has('horn')&&night.has('bell'),'A clear night answers with crickets, the harbour horn and the civic bell');
  assert.ok(rain.has('drip')&&!rain.has('cricket')&&!rain.has('birds'),'Rain replaces the animals with drips');
  assert.ok(evening.has('siren'),'A busy evening carries a distant siren');
  assert.ok([...day,...night,...rain,...evening].length>=13,'The soundscape offers many different events');
  // --- Phase 5: footfalls by surface, the companion, the low tier --------------------------------------------------
  {
    const {STEP_VOICES}=await import('../ui/js/desktop/apps/sysworld-foley.js');
    const {SURFACE_KINDS}=await import('../ui/js/desktop/apps/sysworld-zones.js');
    assert.deepEqual(Object.keys(STEP_VOICES).sort(),[...SURFACE_KINDS].sort(),'every surface has a footfall voice');
    for(const [k,v]of Object.entries(STEP_VOICES))assert.ok(v.gain>=.042&&v.gain<=.063&&v.layers.length>=2,'step voice '+k);
    for(const [k,v]of Object.entries(STEP_VOICES)){if(k==='garden'||k==='grating'){assert.equal(v.body,undefined,k+' has no body resonance');continue;}
      assert.ok(v.body&&v.body.f>=100&&v.body.f<=400&&v.body.q>=6,'body resonance on '+k);}
    const {PAIRS}=await import('../ui/js/desktop/apps/sysworld-foley.js');
    assert.equal(PAIRS.pavement.filter(l=>l.body).length,1,'one body layer in a pavement step');assert.ok(!PAIRS.pavement.some(l=>l.toe&&l.body),'no toe layer carries the body');
    const f=createCityFoley(context,node('out'),node('send'));f.setActive(true);
    const spots={road:[-18,-.14,-50],kerb:[-14.63,0,-50],pavement:[-13.5,0,-50],stairs:[0,.5,.3],quay:[-20,.08,77],lobby:[0,.88,-12],grating:[-43,.88,-56],garden:[-44,0,-88]};
    for(const [surface,[x,y,z]]of Object.entries(spots)){f.listener(x+2,y,z,0,-1,x+2,z);assert.equal(f.step(x,y,z,'v-'+surface),true,surface);assert.equal(f.stats().bySurface[surface],1,surface+' counted');flush();}
    // A carried stride lowers the voice by the CARRY pitch (0.85).
    context.currentTime+=1;f.listener(0,0,-50,0,-1,0,-50);const before=nodes.length;f.gait('c',0,0,-50,true,true);f.gait('c',1.2,0,-50,true,true);
    const tone=nodes.slice(before).find(n=>n.kind==='oscillator');assert.ok(tone&&Math.abs(tone.frequency.ramp[0][1]-104*.85)<1e-6,'carried stride on pavement at 88.4 Hz');flush();
    // The companion: two crossings of a diagonal pair per gait cycle, each a front and a hind foot.
    f.listener(30,0,72,0,-1,30,72);const pose={x:30,z:69.5,phase:0,on:true},played=f.stats().played,start=f.stats().companionSteps;
    for(let i=0;i<=40;i++){context.currentTime+=.03;pose.phase=i*Math.PI/10;f.companion(pose);flush();}
    assert.equal(f.stats().companionSteps-start,4,'four pair crossings over two gait cycles');assert.equal(f.stats().played-played,8,'front and hind foot each');
    pose.on=false;f.companion(pose);pose.on=true;pose.phase+=Math.PI*3;f.companion(pose);assert.equal(f.stats().companionSteps-start,4,'a hidden companion is silent and does not catch up');
    f.setTier('low');for(let i=0;i<20;i++)f.action('robot_work',32,0,72,'L'+i);assert.equal(f.stats().voices,6,'low: six voices');f.setTier('high');flush();f.setActive(false);
  }
  { // A footfall is a heel-toe pair inside ONE voice: heel layers at once, the toe's noise layers 70-95 ms later.
    const f3=createFakeAudio(),d3=f3.node('dest'),one=createCityFoley(f3.context,d3);one.setActive(true);one.listener(0,2.4,0,0,-1,0,0);
    const ok=one.step(1,0.05,0,'visitor',null);assert.equal(ok,true);assert.equal(one.stats().voices,1,'a footfall takes one voice');
    const src=f3.nodes.filter(n=>n.kind==='source'||n.kind==='oscillator'),starts=src.map(n=>n.startArgs[0]).sort((p,q)=>p-q);
    assert.ok(starts[0]===0&&starts.at(-1)>=.07-1e-9&&starts.at(-1)<=.095+1e-9,'the toe starts 70-95 ms after the heel: '+starts);
    const tones=src.filter(n=>n.kind==='oscillator');assert.ok(tones.every(n=>n.startArgs[0]===0),'the toe has no low tone');
    assert.equal(src.filter(n=>n.startArgs[0]>0).length>=1&&src.filter(n=>n.startArgs[0]>0).every(n=>n.kind==='source'),true,'the toe is noise only');
  }
  { // A recorded footfall under each hard-surface step (phase 6): one slice source per voice, never the same slice twice in a row.
    const {PAIR}=await import('../ui/js/desktop/apps/sysworld-foley.js');
    assert.deepEqual(PAIR.sample,{road:.7,pavement:.8,kerb:.8,stairs:.75,quay:.6,lobby:.85});
    const slot=.3,count=12,rec={duration:3.6},spots={road:[-18,-.14,-50],kerb:[-14.63,0,-50],pavement:[-13.5,0,-50],stairs:[0,.5,.3],quay:[-20,.08,77],lobby:[0,.88,-12],grating:[-43,.88,-56],garden:[-44,0,-88]};
    const make=withSteps=>{const fk=createFakeAudio(),d=fk.node('dest'),f=createCityFoley(fk.context,d,d,withSteps?{steps:()=>({buffer:rec,slot,count})}:undefined);f.setActive(true);return {fk,f};};
    const slices=(fk,before)=>fk.nodes.slice(before).filter(n=>n.kind==='source'&&n.startArgs?.length===3);
    const withS=make(true),without=make(false),offsets=[];
    for(const [surface,[x,y,z]]of Object.entries(spots)){
      for(const r of[withS,without]){r.fk.context.currentTime+=1;r.f.listener(x+2,y,z,0,-1,x+2,z);}
      const b1=withS.fk.nodes.length,b0=without.fk.nodes.length;assert.equal(withS.f.step(x,y,z,'v-'+surface),true);assert.equal(without.f.step(x,y,z,'v-'+surface),true);
      const hard=surface!=='garden'&&surface!=='grating',got=slices(withS.fk,b1);
      assert.equal(got.length,hard?1:0,surface+': a sample source only on the hard surfaces');assert.equal(slices(without.fk,b0).length,0,surface+': none without steps');
      assert.equal(withS.fk.nodes.length-b1,without.fk.nodes.length-b0+(hard?2:0),surface+': the sample adds exactly a source and a gain');
      if(hard){const s=got[0],g=s.links[0];assert.equal(g.gain.value,PAIR.sample[surface],surface+' sample gain');assert.ok(s.buffer===rec&&s.playbackRate.value>=.94&&s.playbackRate.value<=1.06);}
      withS.fk.flush();without.fk.flush();
    }
    for(let i=0;i<50;i++){withS.fk.context.currentTime+=1;const before=withS.fk.nodes.length;const [x,y,z]=spots.pavement;withS.f.listener(x+2,y,z,0,-1,x+2,z);
      assert.equal(withS.f.step(x,y,z,'w-'+i),true);const got=slices(withS.fk,before);assert.equal(got.length,1);withS.fk.flush();
      const [at,off,len]=got[0].startArgs,k=Math.round(off/slot);assert.ok(Math.abs(off-k*slot)<1e-9&&k>=0&&k<count,'offset is a multiple of the slot: '+off);assert.ok(len<=.28+1e-12&&at>=0);offsets.push(k);}
    for(let i=1;i<offsets.length;i++)assert.notEqual(offsets[i],offsets[i-1],'no slice twice in a row at step '+i);
    assert.ok(new Set(offsets).size>=6,'the slices vary: '+offsets.join(','));
  }
  console.log('System World city sound: robot foley, gait cadence, bounded voices, spatial beds and the varied event table verified');
}finally{
  Object.assign(globalThis,{setTimeout:native.setTimeout,clearTimeout:native.clearTimeout,window:native.window,localStorage:native.localStorage});
}
