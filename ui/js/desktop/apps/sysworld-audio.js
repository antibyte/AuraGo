import { createTowerVoice } from './sysworld-voice.js';
import { createCityEffects } from './sysworld-effects.js';
import { createCityFoley } from './sysworld-foley.js';
import {zoneWeights,createZoneTracker,doorState} from './sysworld-zones.js';
import {createSoundSpace} from './sysworld-sound-space.js';
import {createSoundBeds} from './sysworld-sound-beds.js';
import {createSoundMovers} from './sysworld-sound-movers.js';
import {createRoomTones} from './sysworld-sound-rooms.js';
import {createSoundRecordings} from './sysworld-sound-recordings.js';
// Synthesized city soundscape: a sub drone, an evolving reverberant chord pad, filtered air with
// wind gusts, harbour water and a low population murmur as spatial beds, plus sparse distant
// events (data chimes, traffic swooshes, harbour gulls and horn, night crickets, rain drips, a
// civic bell, construction hammering, evening sirens). Local Web Audio only; nothing is scheduled
// while the city is muted, hidden, unfocused or in map mode.
const CHORDS=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]];
const CHIMES=[440,523.25,587.33,659.25,783.99,880,1046.5];
const MAX_EVENTS=12;
const VISITOR_STEP=Object.freeze({scale:2.8}); // the visitor's own footfalls (phase 6: raised from 1.9 so they stand out of the bed)
// While a real recording of birds or gulls carries the place, the synthesised calls become occasional accents.
const RECORDED_KIND=Object.freeze({birds:'birds',gull:'gulls'}),RECORDED_THIN=.35;
const gullSpot={x:0,y:0,z:0,heading:0,bank:0,flap:0};
// Master protection (phase 5): a DynamicsCompressor with the design's settings, a fixed trim that cancels the gain the
// Web Audio spec adds by itself ((1/fullRangeGain)^0.6, about +4.3 dB here), and a wave-shaper ceiling that is linear
// below `linear` and never passes `ceiling` (-1 dBFS). The sound survey measures the trim (--calibrate).
export const LIMITER=Object.freeze({threshold:-10,knee:6,ratio:12,attack:.003,release:.25,trim:.607,ceiling:.89,linear:.6});
export function ceilingCurve(n=2049){const c=new Float32Array(n),{ceiling,linear}=LIMITER,span=ceiling-linear;
  for(let i=0;i<n;i++){const x=i*2/(n-1)-1,a=Math.abs(x);c[i]=Math.sign(x)*(a<=linear?a:linear+span*Math.tanh((a-linear)/span));}return c;}
export function createLimiter(context){
  const compressor=context.createDynamicsCompressor(),trim=context.createGain(),ceiling=context.createWaveShaper();
  compressor.threshold.value=LIMITER.threshold;compressor.knee.value=LIMITER.knee;compressor.ratio.value=LIMITER.ratio;
  compressor.attack.value=LIMITER.attack;compressor.release.value=LIMITER.release;
  trim.gain.value=LIMITER.trim;ceiling.curve=ceilingCurve();ceiling.oversample='4x';compressor.connect(trim).connect(ceiling);
  return {input:compressor,output:ceiling,compressor,nodes:[compressor,trim,ceiling]};
}
// Fixed places the city owns. An event either comes from one of these anchors or from a ring
// around the listener, so the pan always matches the direction the sound arrives from.
const ANCHORS={harbour:{x:0,z:79},quay:{x:0,z:72},tower:{x:0,z:-12},square:{x:8,z:40}};
// Weighted event table. Every entry carries its own mood condition, so the same city sounds different at noon, at
// night, in rain and while the agent is busy. `zone` scales the weight by where the listener stands (outdoor shares
// of sysworld-zones.js). Behind glass only `indoor` events stay (loud enough to pass it; the glass damps them), so no
// bird sings in a machine hall; the open garden court keeps the whole table.
const EVENTS=[
  {kind:'chime',weight:15,indoor:true},
  {kind:'swoosh',weight:13,zone:z=>.3+1.4*z.canyon+.4*z.plaza},
  {kind:'gust',weight:7,zone:z=>.4+z.height+.6*z.harbour+.4*z.green},
  {kind:'birds',weight:8,when:m=>m.day>.45&&m.weather==='clear',zone:z=>.15+2.2*z.green+.3*z.plaza},
  {kind:'gull',weight:6,when:m=>m.day>.2&&m.weather==='clear',zone:z=>.1+2.6*z.harbour+.6*z.height},
  {kind:'pigeon',weight:4,when:m=>m.day>.6&&!m.busy,zone:z=>.2+1.2*z.plaza+.8*z.canyon},
  {kind:'cricket',weight:6,when:m=>m.day<.25&&m.weather!=='rain',zone:z=>.1+2.4*z.green},
  {kind:'horn',weight:4,when:m=>m.day<.45&&m.weather!=='rain',indoor:true,zone:z=>.5+1.5*z.harbour},
  {kind:'bell',weight:3,when:m=>m.day<.7,indoor:true},
  {kind:'hammer',weight:4,when:m=>m.day>.3,zone:z=>.4+z.canyon+.5*z.plaza},
  {kind:'drip',weight:10,when:m=>m.weather==='rain',zone:z=>.3+z.canyon+.6*z.green},
  {kind:'windchime',weight:4,when:m=>m.weather==='clear'&&!m.busy,zone:z=>.2+1.6*z.harbour},
  {kind:'siren',weight:2,when:m=>m.evening>.3&&m.busy,indoor:true},
];
export const INDOOR_GATE=.5;
export function eventWeight(e,mood,zones){
  if(e.when&&!e.when(mood))return 0;
  if(zones){if(zones.interior>=INDOOR_GATE&&!zones.open&&!e.indoor)return 0;if(e.zone)return e.weight*e.zone(zones);}
  return e.weight;
}
// Exported for the acceptance check: the weighted event table, its mood filter and (with zones) the zone weights.
export function citySoundscapeEvents(mood={},zones=null) {
  const state={busy:!!mood.busy,day:Number.isFinite(mood.day)?mood.day:0,evening:Number.isFinite(mood.evening)?mood.evening:0,
    weather:['clear','rain','fog'].includes(mood.weather)?mood.weather:'clear'};
  return EVENTS.map(e=>({kind:e.kind,weight:eventWeight(e,state,zones)})).filter(e=>e.weight>0);
}
function impulse(context,seconds,decay){
  const length=Math.floor(context.sampleRate*seconds),buffer=context.createBuffer(2,length,context.sampleRate);
  let seed=977;
  for(let c=0;c<2;c++){const data=buffer.getChannelData(c);for(let i=0;i<length;i++){seed=Math.imul(seed,1664525)+1013904223|0;data[i]=((seed>>>0)/2147483648-1)*Math.pow(1-i/length,decay);}}
  return buffer;
}
export function createCityAmbience(onState=()=>{},options={}) {
  // Injection (phase 5): the sound survey renders this same graph through an OfflineAudioContext with audio-clock
  // timers and a seeded random. Defaults are the live browser: window.AudioContext, the global timers, Math.random,
  // localStorage. An injected context belongs to its owner (never closed here); `offline` skips resume/suspend.
  const timers=options.timers||globalThis,random=options.random||Math.random,offline=!!options.offline;
  const storage=options.storage===undefined?globalThis.localStorage:options.storage;
  // Recordings (sysworld-sound-recordings.js): load(path) returns an ArrayBuffer of 'manifest.json' or '<id>.ogg'. The default
  // fetches the app's own audio folder; options.assetURL(file) lets the app resolve it like its model files (versioned).
  const assetURL=typeof options.assetURL==='function'?options.assetURL:file=>'/3d/system-world/audio/'+file;
  const load=typeof options.load==='function'?options.load:async path=>{const response=await fetch(assetURL(path));if(!response.ok)throw Error('audio '+response.status);return response.arrayBuffer();};
  const setTimeout=(fn,ms)=>timers.setTimeout(fn,ms),clearTimeout=handle=>timers.clearTimeout(handle);
  let tier=['low','medium','high','ultra'].includes(options.tier)?options.tier:'high',frames=0,compressor=null;
  let context=null, master=null, analyser=null, disposed=false, active=false, enabled=false, fadeTimer=0, voice=null;
  const nodes=[], sources=[], samples=new Float32Array(256), transient=new Set();
  let volume=.18,effects=null,foley=null,ambientBus=null,effectsBus=null,voiceBus=null,inside=false,weather='clear';
  let pad=[],padFilter=null,padLevel=null,airFilter=null,airGain=null,verb=null,noise=null;
  let waterGain=null,murmurGain=null,lampGain=null,bedWater=0,bedMurmur=0,bedLamp=0;
  let eventTimer=0,chordTimer=0,chord=0,events=0;
  let space=null,tracker=null,lastFrame=0,soundFrame=null,beds=null,movers=null,rooms=null,recordings=null;const raw={},door={};
  const listener={x:0,y:2.4,z:0,fx:0,fz:-1,origin:{x:0,z:0}};
  const mood={busy:false,day:0,evening:0,weather:'clear'};
  const levels={ambience:1,effects:1,voice:1};
  try {enabled=options.enabled===true||storage?.getItem('aurago.desktop.sysworld.sound')==='true';}catch(_){}
  // Stored levels (phase 5): volume and the three channel sliders persist like the on/off switch.
  try {const raw=storage?.getItem('aurago.desktop.sysworld.volume'),v=Number(raw);if(raw!=null&&Number.isFinite(v))volume=Math.max(0,Math.min(.35,v));}catch(_){}
  try {const saved=JSON.parse(storage?.getItem('aurago.desktop.sysworld.channels')||'{}');for(const key of Object.keys(levels))if(Number.isFinite(saved?.[key]))levels[key]=Math.max(0,Math.min(1,saved[key]));}catch(_){}
  const save=(key,value)=>{try{storage?.setItem(key,value);}catch(_){}};
  function ensure() {
    if(context||disposed)return;
    context=options.context||new (window.AudioContext||window.webkitAudioContext)();
    const node=n=>(nodes.push(n),n), source=n=>(sources.push(n),node(n));
    master=node(context.createGain());master.gain.value=0;
    const limiter=createLimiter(context);limiter.nodes.forEach(n=>node(n));compressor=limiter.compressor;
    analyser=node(context.createAnalyser());analyser.fftSize=512;master.connect(limiter.input);limiter.output.connect(analyser);analyser.connect(context.destination);
    ambientBus=node(context.createGain());ambientBus.gain.value=levels.ambience;ambientBus.connect(master);
    effectsBus=node(context.createGain());effectsBus.gain.value=levels.effects;effectsBus.connect(master);
    voiceBus=node(context.createGain());voiceBus.gain.value=levels.voice;voiceBus.connect(master);
    verb=node(context.createConvolver());verb.buffer=impulse(context,3.4,2.6);
    space=createSoundSpace(context,{ambience:ambientBus,verb,tier});tracker=createZoneTracker();
    zoneWeights(listener.x,listener.y,listener.z,listener.origin.x,listener.origin.z,raw);   // builds the zone fields during the gesture
    const verbReturn=node(context.createGain());verbReturn.gain.value=.55;verb.connect(verbReturn).connect(space.outdoor);
    voice=options.voice===false?null:createTowerVoice(context,voiceBus);effects=createCityEffects(context,effectsBus,ambientBus);effects.environment(inside,weather);
    foley=createCityFoley(context,effectsBus,space.sendLocal,{steps:()=>{const b=recordings?.buffer('steps'),s=recordings?.slices('steps');return b&&s?{buffer:b,slot:s.slot,count:s.count}:null;}});foley.setTier(tier);foley.environment(inside);foley.listener(listener.x,listener.y,listener.z,listener.fx,listener.fz,listener.origin.x,listener.origin.z);
    for(const [frequency,gain] of [[55,.022],[82.4069,.007],[110,.0032]]) {
      const oscillator=source(context.createOscillator()), level=node(context.createGain());
      oscillator.frequency.value=frequency;level.gain.value=gain;
      oscillator.connect(level).connect(ambientBus);oscillator.start();
    }
    // Warm detuned saw pad; chords glide slowly and the filter follows the city's mood.
    padFilter=node(context.createBiquadFilter());padFilter.type='lowpass';padFilter.frequency.value=650;padFilter.Q.value=.6;
    padLevel=node(context.createGain());padLevel.gain.value=.014;
    const padSend=node(context.createGain());padSend.gain.value=.9;
    padFilter.connect(padLevel).connect(ambientBus);padLevel.connect(padSend).connect(verb);
    pad=CHORDS[0].map(frequency=>[-6,6].map(detune=>{
      const oscillator=source(context.createOscillator()),level=node(context.createGain());
      oscillator.type='sawtooth';oscillator.frequency.value=frequency;oscillator.detune.value=detune;level.gain.value=.22;
      oscillator.connect(level).connect(padFilter);oscillator.start();return oscillator;
    }));
    const sweep=source(context.createOscillator()),sweepDepth=node(context.createGain());
    sweep.frequency.value=.05;sweepDepth.gain.value=180;sweep.connect(sweepDepth).connect(padFilter.frequency);sweep.start();
    const buffer=context.createBuffer(1,context.sampleRate*4,context.sampleRate), data=buffer.getChannelData(0);
    let brown=0;
    for(let i=0;i<data.length;i++){brown=(brown+(random()*2-1)*.018)/1.018;data[i]=brown;}
    const seam=data[data.length-1]-data[0];
    for(let i=0;i<data.length;i++)data[i]-=seam*i/(data.length-1);
    const air=source(context.createBufferSource());airFilter=node(context.createBiquadFilter());airGain=node(context.createGain());
    air.buffer=buffer;air.loop=true;airFilter.type='lowpass';airFilter.frequency.value=680;airFilter.Q.value=.25;airGain.gain.value=.16;
    air.connect(airFilter).connect(airGain).connect(space.outdoor);air.start();
    const drift=source(context.createOscillator()), depth=node(context.createGain());
    drift.frequency.value=.075;depth.gain.value=160;drift.connect(depth).connect(airFilter.frequency);drift.start();
    noise=context.createBuffer(1,context.sampleRate*2,context.sampleRate);
    const white=noise.getChannelData(0);for(let i=0;i<white.length;i++)white[i]=random()*2-1;
    // Two spatial beds: harbour water near the quay and a low population murmur.
    // Both are continuous, so they stay far below the transient events.
    const wash=source(context.createBufferSource());wash.buffer=noise;wash.loop=true;
    const washBand=node(context.createBiquadFilter());washBand.type='bandpass';washBand.frequency.value=620;washBand.Q.value=.5;
    const swell=source(context.createOscillator()),swellDepth=node(context.createGain());
    swell.frequency.value=.08;swellDepth.gain.value=140;swell.connect(swellDepth).connect(washBand.frequency);swell.start();
    waterGain=node(context.createGain());waterGain.gain.value=0;
    wash.connect(washBand).connect(waterGain).connect(space.outdoor);wash.start();
    const crowd=source(context.createBufferSource());crowd.buffer=noise;crowd.loop=true;
    const crowdBand=node(context.createBiquadFilter());crowdBand.type='bandpass';crowdBand.frequency.value=320;crowdBand.Q.value=.7;
    const crowdDrift=source(context.createOscillator()),crowdDepth=node(context.createGain());
    crowdDrift.frequency.value=.13;crowdDepth.gain.value=90;crowdDrift.connect(crowdDepth).connect(crowdBand.frequency);crowdDrift.start();
    murmurGain=node(context.createGain());murmurGain.gain.value=0;
    crowd.connect(crowdBand).connect(murmurGain).connect(space.outdoor);crowd.start();
    // Warm harbour lanterns: a very quiet two-tone hum with a slow breath. It
    // is a continuous bed, only opens near the quay after dusk, and never adds
    // a transient voice to the bounded event pool.
    const lamp=source(context.createOscillator()),harmonic=source(context.createOscillator());
    lamp.type='sine';lamp.frequency.value=118;harmonic.type='sine';harmonic.frequency.value=236;
    const harmonicLevel=node(context.createGain());harmonicLevel.gain.value=.26;
    const lampTrem=node(context.createGain());lampTrem.gain.value=.9;
    const breath=source(context.createOscillator()),breathDepth=node(context.createGain());
    breath.frequency.value=.19;breathDepth.gain.value=.08;breath.connect(breathDepth).connect(lampTrem.gain);breath.start();
    const lampFilter=node(context.createBiquadFilter());lampFilter.type='lowpass';lampFilter.frequency.value=420;lampFilter.Q.value=.4;
    lampGain=node(context.createGain());lampGain.gain.value=0;
    lamp.connect(lampTrem);harmonic.connect(harmonicLevel).connect(lampTrem);
    lampTrem.connect(lampFilter).connect(lampGain).connect(space.outdoor);
    lamp.start();harmonic.start();
    // Phase 5 layers: zone-shaped beds, moving sources, room tones (outdoor ones behind the glass).
    beds=createSoundBeds(context,{outdoor:space.outdoor,send:space.sendOutdoor,local:ambientBus,noise,random,tier});beds.setMood(mood);
    movers=createSoundMovers(context,{outdoor:space.outdoor,send:space.sendOutdoor,noise,tier,random,buffer:id=>recordings?.buffer(id)??null});
    rooms=createRoomTones(context,{room:space.room,send:space.roomSend,noise,random,tier});rooms.setMood(mood);
    recordings=createSoundRecordings(context,{outdoor:space.outdoor,send:space.sendOutdoor,load,tier});recordings.setMood(mood);
    applyMood();
    updateBeds();
  }
  const running=()=>!!context&&enabled&&active&&!disposed&&volume>0;
  // Every transient voice releases its nodes when its first source ends or the city goes quiet.
  function track(list){
    const entry={release(){if(!transient.delete(entry))return;for(const n of list){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}}};
    transient.add(entry);list[0].onended=entry.release;events++;return entry;
  }
  // Outdoor events (phase 5) pass the outdoor bus, so glass damps them indoors, and send into the outdoor spaces.
  function out(level,pan,send){
    const panner=context.createStereoPanner(),reverb=context.createGain();panner.pan.value=pan;reverb.gain.value=send;
    level.connect(panner).connect(space.outdoor);panner.connect(reverb).connect(space.sendOutdoor);return [panner,reverb];
  }
  // Distance and direction of a world position against the listener frame. The
  // distance runs from the acoustic origin (where the view rests), the pan from
  // the camera, so a sound keeps the direction it has in the picture.
  function reach(position){
    const cx=position.x-listener.x,cz=position.z-listener.z;
    const ox=position.x-listener.origin.x,oz=position.z-listener.origin.z;
    const distance=Math.hypot(ox,oz),view=Math.max(1,Math.hypot(cx,cz));
    return {distance,pan:Math.max(-1,Math.min(1,(cx*-listener.fz+cz*listener.fx)/view)),fade:1/(1+distance*.02)};
  }
  function positionFor(kind){
    if(kind==='gull')return movers?.gull(soundFrame,random,gullSpot)||ANCHORS.harbour;if(kind==='horn')return ANCHORS.harbour;
    if(kind==='windchime')return ANCHORS.quay;
    if(kind==='bell')return ANCHORS.tower;
    if(kind==='pigeon')return ANCHORS.square;
    // The ring surrounds the place the view rests on, so the city stays around you.
    const angle=random()*Math.PI*2,distance=24+random()*44;
    return {x:listener.origin.x+Math.sin(angle)*distance,z:listener.origin.z+Math.cos(angle)*distance};
  }
  function chime(position){
    const t=context.currentTime,f=CHIMES[Math.floor(random()*CHIMES.length)];
    const a=context.createOscillator(),b=context.createOscillator(),partial=context.createGain(),level=context.createGain();
    const {pan,fade}=reach(position);
    a.frequency.value=f;b.frequency.value=f*2.76;partial.gain.value=.22;
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(mood.busy?.02:.014,t+.008);level.gain.exponentialRampToValueAtTime(.0001,t+2.8);
    a.connect(level);b.connect(partial).connect(level);
    track([a,b,partial,level,...out(level,pan,1.4)]);a.start(t);b.start(t);a.stop(t+2.9);b.stop(t+2.9);
  }
  function swoosh(position){
    const t=context.currentTime,src=context.createBufferSource(),band=context.createBiquadFilter(),level=context.createGain();
    const {pan,fade}=reach(position);
    src.buffer=noise;src.loop=true;band.type='bandpass';band.Q.value=1.4;
    band.frequency.setValueAtTime(260,t);band.frequency.exponentialRampToValueAtTime(1100,t+1.6);band.frequency.exponentialRampToValueAtTime(380,t+3.4);
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.03*fade,t+1.6);level.gain.exponentialRampToValueAtTime(.0001,t+3.6);
    src.connect(band).connect(level);const [panner,reverb]=out(level,pan,.5);
    // A passing vehicle sweeps across the field of view but keeps its own bearing.
    const from=Math.max(-1,pan-.6),to=Math.min(1,pan+.6);
    panner.pan.setValueAtTime(from,t);panner.pan.linearRampToValueAtTime(to,t+3.4);
    track([src,band,level,panner,reverb]);src.start(t);src.stop(t+3.7);
  }
  function gust(){
    const t=context.currentTime,base=weather==='rain'?.18:.16;
    airFilter.frequency.cancelScheduledValues(t);airGain.gain.cancelScheduledValues(t);
    airFilter.frequency.setTargetAtTime(1200+random()*700,t,.8);airFilter.frequency.setTargetAtTime(680,t+2.6,1.4);
    airGain.gain.setTargetAtTime(base*1.7,t,.7);airGain.gain.setTargetAtTime(base,t+2.6,1.5);events++;
  }
  function horn(position){
    const t=context.currentTime,low=context.createBiquadFilter(),level=context.createGain();
    const {pan,fade}=reach(position);
    low.type='lowpass';low.frequency.value=420;
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.028*fade,t+.9);level.gain.setValueAtTime(.028*fade,t+3.2);level.gain.exponentialRampToValueAtTime(.0001,t+5.5);
    const tones=[69.3,103.83].map(f=>{const o=context.createOscillator();o.type='sawtooth';o.frequency.value=f;o.detune.value=random()*8-4;o.connect(low);return o;});
    low.connect(level);track([...tones,low,level,...out(level,pan,1.6)]);tones.forEach(o=>{o.start(t);o.stop(t+5.6);});
  }
  function birds(position){
    const t=context.currentTime,count=2+Math.floor(random()*3);
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.12+random()*.1),o=context.createOscillator(),level=context.createGain(),f=2600+random()*900;
      o.frequency.setValueAtTime(f,at);o.frequency.exponentialRampToValueAtTime(f*1.5,at+.07);
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.006*fade,at+.015);level.gain.exponentialRampToValueAtTime(.0001,at+.13);
      o.connect(level);track([o,level,...out(level,pan,.6)]);o.start(at);o.stop(at+.15);
    }
  }
  function gull(position){
    const t=context.currentTime,count=2+Math.floor(random()*3),base=760+random()*280;
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.2+random()*.14),o=context.createOscillator(),form=context.createBiquadFilter(),level=context.createGain();
      o.type='sawtooth';o.frequency.setValueAtTime(base,at);o.frequency.exponentialRampToValueAtTime(base*.5,at+.26);
      form.type='bandpass';form.frequency.value=1500;form.Q.value=3.6;
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.012*fade,at+.04);level.gain.exponentialRampToValueAtTime(.0001,at+.34);
      o.connect(form).connect(level);track([o,form,level,...out(level,pan,.9)]);o.start(at);o.stop(at+.36);
    }
  }
  function pigeon(position){
    const t=context.currentTime,count=2+Math.floor(random()*2),base=420+random()*90;
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.17+random()*.08),o=context.createOscillator(),form=context.createBiquadFilter(),level=context.createGain();
      o.type='triangle';o.frequency.setValueAtTime(base,at);o.frequency.linearRampToValueAtTime(base*.72,at+.12);
      form.type='lowpass';form.frequency.value=1100;
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.01*fade,at+.03);level.gain.exponentialRampToValueAtTime(.0001,at+.16);
      o.connect(form).connect(level);track([o,form,level,...out(level,pan,.5)]);o.start(at);o.stop(at+.18);
    }
  }
  function cricket(position){
    const t=context.currentTime,count=3+Math.floor(random()*4),f=3900+random()*900;
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*.055,o=context.createOscillator(),level=context.createGain();
      o.type='square';o.frequency.value=f;
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.005*fade,at+.008);level.gain.exponentialRampToValueAtTime(.0001,at+.035);
      o.connect(level);track([o,level,...out(level,pan,.35)]);o.start(at);o.stop(at+.05);
    }
  }
  function bell(position){
    const t=context.currentTime,base=196;
    const {pan,fade}=reach(position);
    const level=context.createGain();
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.02*fade,t+.012);level.gain.exponentialRampToValueAtTime(.0001,t+4.2);
    const partials=[1,2.02,2.99,4.18,5.43].map((ratio,i)=>{
      const o=context.createOscillator(),g=context.createGain();
      o.type='sine';o.frequency.value=base*ratio;g.gain.value=.5/(i+1);
      o.connect(g).connect(level);return o;
    });
    track([...partials,level,...out(level,pan,1.8)]);partials.forEach(o=>{o.start(t);o.stop(t+4.3);});
  }
  function hammer(position){
    const t=context.currentTime,count=2+Math.floor(random()*3);
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.5+random()*.35),src=context.createBufferSource(),band=context.createBiquadFilter(),level=context.createGain();
      const ring=context.createOscillator(),ringGain=context.createGain();
      src.buffer=noise;src.loop=true;band.type='bandpass';band.frequency.value=420;band.Q.value=1.1;
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.02*fade,at+.006);level.gain.exponentialRampToValueAtTime(.0001,at+.42);
      ring.type='sine';ring.frequency.value=1180;ringGain.gain.value=.2;
      src.connect(band).connect(level);ring.connect(ringGain).connect(level);
      track([src,band,level,ring,ringGain,...out(level,pan,.9)]);
      src.start(at);src.stop(at+.44);ring.start(at);ring.stop(at+.44);
    }
  }
  function drip(position){
    const t=context.currentTime,count=2+Math.floor(random()*4);
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.06+random()*.14),o=context.createOscillator(),level=context.createGain();
      o.type='sine';o.frequency.setValueAtTime(900+random()*700,at);o.frequency.exponentialRampToValueAtTime(320,at+.06);
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.008*fade,at+.005);level.gain.exponentialRampToValueAtTime(.0001,at+.12);
      o.connect(level);track([o,level,...out(level,pan,.4)]);o.start(at);o.stop(at+.14);
    }
  }
  function windchime(position){
    const t=context.currentTime,count=3+Math.floor(random()*4),scale=[1046.5,1174.7,1396.9,1567.98,1760,2093];
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.09+random()*.13),f=scale[Math.floor(random()*scale.length)];
      const o=context.createOscillator(),level=context.createGain();
      o.type='sine';o.frequency.value=f;
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.006*fade,at+.004);level.gain.exponentialRampToValueAtTime(.0001,at+.9);
      o.connect(level);track([o,level,...out(level,pan,1.5)]);o.start(at);o.stop(at+.95);
    }
  }
  function siren(position){
    const t=context.currentTime;
    const {pan,fade}=reach(position);
    const o=context.createOscillator(),level=context.createGain();
    o.type='sine';o.frequency.setValueAtTime(420,t);o.frequency.linearRampToValueAtTime(620,t+1.8);o.frequency.linearRampToValueAtTime(430,t+3.6);
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.01*fade,t+.5);level.gain.setValueAtTime(.01*fade,t+2.8);level.gain.exponentialRampToValueAtTime(.0001,t+4.2);
    o.connect(level);track([o,level,...out(level,pan,1.2)]);o.start(t);o.stop(t+4.3);
  }
  const VOICES={chime,swoosh,gust,horn,birds,gull,pigeon,cricket,bell,hammer,drip,windchime,siren};
  // Event weight with the recordings in mind: a loaded, audible birds/gulls recording carries the place, so the matching
  // synthesised call is thinned to RECORDED_THIN (it stays as an occasional positioned accent).
  function weigh(e,z){const w=eventWeight(e,mood,z),id=RECORDED_KIND[e.kind];return w>0&&id&&recordings?.carries(id)?w*RECORDED_THIN:w;}
  function pickEvent(){
    const z=tracker?tracker.weights:null;let total=0;
    for(let i=0;i<EVENTS.length;i++)total+=weigh(EVENTS[i],z);
    if(total<=0)return null;
    let roll=random()*total;
    for(let i=0;i<EVENTS.length;i++){const w=weigh(EVENTS[i],z);if(w<=0)continue;roll-=w;if(roll<=0)return EVENTS[i].kind;}
    return null;
  }
  function event(){
    if(transient.size>=MAX_EVENTS)return;
    const kind=pickEvent();if(kind)VOICES[kind]?.(positionFor(kind));
  }
  function schedule(){
    clearTimeout(eventTimer);eventTimer=0;if(!running())return;
    eventTimer=setTimeout(()=>{eventTimer=0;if(!running())return;event();schedule();},(mood.busy?1900:3600)+random()*9000);
  }
  function progress(){
    clearTimeout(chordTimer);chordTimer=0;if(!running())return;
    chordTimer=setTimeout(()=>{
      chordTimer=0;if(!running())return;chord=(chord+1)%CHORDS.length;
      pad.forEach((pair,i)=>pair.forEach(o=>o.frequency.setTargetAtTime(CHORDS[chord][i],context.currentTime,1.8)));progress();
    },16000);
  }
  function quiet(){clearTimeout(eventTimer);clearTimeout(chordTimer);eventTimer=chordTimer=0;for(const v of [...transient])v.release();beds?.clear();movers?.clear();rooms?.clear();}
  function applyMood(){
    if(!context)return;const t=context.currentTime;
    padFilter.frequency.setTargetAtTime(mood.busy?1500:650+mood.day*450+mood.evening*150,t,2.5);
    padLevel.gain.setTargetAtTime((mood.busy?.018:.014)*(mood.weather==='rain'?.75:1),t,2);
  }
  // The beds follow the zones (sysworld-zones.js): water and the lantern hum by the quay proximity, the murmur by the
  // plaza and canyon shares. Glass (sysworld-sound-space.js) damps them indoors, so nothing here is zeroed for the
  // inside flag. Levels are quantized to 1/1000 so a per-frame listener update queues no ramp while nothing changes.
  function updateBeds(){
    if(!context||!waterGain||!murmurGain||!lampGain||!running())return;
    zoneWeights(listener.x,listener.y,listener.z,listener.origin.x,listener.origin.z,raw);
    const water=Math.round(raw.quay*.05*(weather==='rain'?1.2:1)*1000);
    const crowd=Math.round((mood.busy?.03:.012)*(.35+mood.day*.65)*(.5+.5*Math.min(1,raw.plaza+raw.canyon))*1000);
    const lamp=Math.round(raw.quay*.012*(1-mood.day)*(weather==='rain'?.6:1)*1000);
    if(water===bedWater&&crowd===bedMurmur&&lamp===bedLamp)return;
    bedWater=water;bedMurmur=crowd;bedLamp=lamp;
    const t=context.currentTime;
    waterGain.gain.setTargetAtTime(water/1000,t,.5);murmurGain.gain.setTargetAtTime(crowd/1000,t,.8);lampGain.gain.setTargetAtTime(lamp/1000,t,.8);
  }
  // The listener-driven parts, refreshed on every unmute (setListener skips them while silent).
  function refresh(){voice?.setListener(listener.x,listener.y,listener.z,listener.fx,listener.fz);effects?.listener(listener.x,listener.y,listener.z,listener.fx,listener.fz);
    foley?.listener(listener.x,listener.y,listener.z,listener.fx,listener.fz,listener.origin.x,listener.origin.z);updateBeds();}
  function sync(gesture=false) {
    clearTimeout(fadeTimer);
    if(disposed)return;
    if(gesture&&enabled){try{ensure();}catch(_){enabled=false;onState(false);return;}}
    if(!context)return;
    voice?.setActive(enabled&&active&&volume>0&&levels.voice>0);effects?.setActive(enabled&&active&&volume>0);
    foley?.setActive(enabled&&active&&volume>0&&levels.effects>0);
    if(enabled&&active) {
      // First unlock always comes from a user gesture; later resumes follow focus.
      if(!offline)void context.resume().catch(()=>{});
      master.gain.cancelScheduledValues(context.currentTime);
      master.gain.setTargetAtTime(volume,context.currentTime,.25);
      if(!eventTimer)schedule();if(!chordTimer)progress();
      refresh();
    } else {
      quiet();
      master.gain.cancelScheduledValues(context.currentTime);
      master.gain.setTargetAtTime(0,context.currentTime,.035);
      fadeTimer=setTimeout(()=>{if(!offline&&!disposed&&(!enabled||!active))void context.suspend().catch(()=>{});},180);
    }
  }
  return {
    enabled:()=>enabled,
    toggle() {enabled=!enabled;try{localStorage.setItem('aurago.desktop.sysworld.sound',String(enabled));}catch(_){}sync(true);onState(enabled);},
    unlock() {sync(true);},
    setActive(value) {if(active===value)return;active=value;sync();},
    setVolume(value) {if(Number.isFinite(value)){volume=Math.max(0,Math.min(.35,value));save('aurago.desktop.sysworld.volume',String(volume));sync();}},
    setListener(x,y,z,fx,fz,ox,oz) {listener.x=x;listener.y=y;listener.z=z;listener.fx=fx;listener.fz=fz;listener.origin.x=ox??x;listener.origin.z=oz??z;
      // A silent city does no per-frame work; sync() refreshes everything on unmute.
      if(running())refresh();},
    setChannel(key,value){if(!(key in levels)||!Number.isFinite(value))return;levels[key]=Math.max(0,Math.min(1,value));const bus={ambience:ambientBus,effects:effectsBus,voice:voiceBus}[key];if(bus)bus.gain.setTargetAtTime(levels[key],context.currentTime,.05);save('aurago.desktop.sysworld.channels',JSON.stringify(levels));sync();},
    // World interactions keep their own channel; robot actions are routed to the foley.
    effect(kind,x,y,z,id){if(String(kind).startsWith('robot_')){foley?.action(kind,x,y,z,id);return;}
      // The visitor's own steps (phase 5): a surface voice from the foley at the floor under the eye (2.4 m).
      if(kind==='step'||kind==='step_inside'){foley?.step(x,y-2.4,z,'visitor',VISITOR_STEP);return;}
      effects?.play(kind,x,y,z);},
    gait(id,x,y,z,moving,carrying,style){foley?.gait(id,x,y,z,moving,carrying,style);movers?.gait(id,x,y,z,moving,style);},
    setEnvironment(interior,condition){inside=!!interior;if(['clear','rain','fog'].includes(condition))weather=condition;effects?.environment(inside,weather);foley?.environment(inside);updateBeds();},
    setMood(value={}){Object.assign(mood,{busy:!!value.busy,day:Number.isFinite(value.day)?value.day:mood.day,evening:Number.isFinite(value.evening)?value.evening:mood.evening,
      weather:['clear','rain','fog'].includes(value.weather)?value.weather:mood.weather});beds?.setMood(mood);rooms?.setMood(mood);recordings?.setMood(mood);applyMood();updateBeds();},
    // Per-frame scene input (phase 5; sysworld-sound-frame.js). Returns at once while silent.
    frame(f){
      if(!running()||!space||!f)return;
      const now=context.currentTime,dt=lastFrame?Math.min(.25,Math.max(0,now-lastFrame)):0;lastFrame=now;frames++;soundFrame=f;
      if(raw.harbour===undefined)updateBeds();
      doorState(listener.x,listener.y,listener.z,f.doorValue,door);tracker.update(raw,door,dt,listener);space.update(tracker.weights,dt);
      beds.update(f,tracker.weights,listener,now);movers.update(f,listener,now,dt);rooms.update(tracker.weights,now);recordings.update(f,tracker.weights);foley?.companion(f.companion);
    },
    setTier(value){if(!['low','medium','high','ultra'].includes(value)||value===tier)return;tier=value;space?.setTier(value);foley?.setTier(value);beds?.setTier(value);movers?.setTier(value);rooms?.setTier(value);recordings?.setTier(value);},
    // Rolling thunder after a lightning flash; the delay (s) doubles as a distance cue.
    thunder(delay=1){
      if(!running()||inside||transient.size>=MAX_EVENTS)return;
      const wait=Math.max(0,Math.min(5,delay)),t=context.currentTime+wait,far=Math.min(1,wait/4);
      const src=context.createBufferSource(),low=context.createBiquadFilter(),level=context.createGain();
      src.buffer=noise;src.loop=true;low.type='lowpass';low.Q.value=.7;
      low.frequency.setValueAtTime(900-far*500,t);low.frequency.exponentialRampToValueAtTime(70,t+3.5);
      level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.09*(1-far*.5),t+.06+far*.25);
      level.gain.exponentialRampToValueAtTime(.025,t+1.3);level.gain.exponentialRampToValueAtTime(.0001,t+5.5);
      src.connect(low).connect(level);track([src,low,level,...out(level,random()*.8-.4,.8)]);src.start(t);src.stop(t+5.6);
    },
    stats() {let rms=0;if(analyser&&context.state==='running'){analyser.getFloatTimeDomainData(samples);for(const x of samples)rms+=x*x;}
      return {voice:voice?.stats(),effects:effects?.stats(),foley:foley?.stats(),channels:{...levels},enabled,active,state:context?.state||'uninitialized',volume,rms:Math.sqrt(rms/samples.length),limiter:compressor?{threshold:LIMITER.threshold,trim:LIMITER.trim,reduction:+(Number(compressor.reduction)||0).toFixed(2)}:null,graph:{nodes:nodes.length+(space?space.stats().nodes:0)+(beds?beds.stats().nodes:0)+(movers?movers.stats().nodes:0)+(rooms?rooms.stats().nodes:0)+(recordings?recordings.stats().nodes:0),sources:sources.length,tier,frames},
        soundscape:{chord,events,transient:transient.size,scheduled:!!eventTimer,mood:{...mood},beds:{water:bedWater/1000,murmur:bedMurmur/1000,lamp:bedLamp/1000},listener:{x:listener.x,z:listener.z},zones:tracker?{...tracker.weights}:null,space:space?space.stats():null,layers:{beds:beds?.stats()||null,movers:movers?.stats()||null,rooms:rooms?.stats()||null,recordings:recordings?.stats()||null}}};},
    dispose() {if(disposed)return;disposed=true;quiet();foley?.dispose();effects?.dispose();voice?.dispose();clearTimeout(fadeTimer);sources.forEach(n=>{try{n.stop();}catch(_){}});beds?.dispose();movers?.dispose();rooms?.dispose();recordings?.dispose();space?.dispose();nodes.forEach(n=>n.disconnect());if(context&&!options.context&&context.close)void context.close().catch(()=>{});},
  };
}
