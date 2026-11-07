import { createTowerVoice } from './sysworld-voice.js';
import { createCityEffects } from './sysworld-effects.js';
import { createCityFoley } from './sysworld-foley.js';
// Synthesized city soundscape: a sub drone, an evolving reverberant chord pad, filtered air with
// wind gusts, harbour water and a low population murmur as spatial beds, plus sparse distant
// events (data chimes, traffic swooshes, harbour gulls and horn, night crickets, rain drips, a
// civic bell, construction hammering, evening sirens). Local Web Audio only; nothing is scheduled
// while the city is muted, hidden, unfocused or in map mode.
const CHORDS=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]];
const CHIMES=[440,523.25,587.33,659.25,783.99,880,1046.5];
const MAX_EVENTS=12;
// Fixed places the city owns. An event either comes from one of these anchors or from a ring
// around the listener, so the pan always matches the direction the sound arrives from.
const ANCHORS={harbour:{x:0,z:79},quay:{x:0,z:72},tower:{x:0,z:-12},square:{x:8,z:40}};
// Weighted event table. Every entry carries its own mood condition, so the same city sounds
// different at noon, at night, in rain and while the agent is busy.
const EVENTS=[
  {kind:'chime',weight:15},{kind:'swoosh',weight:13},{kind:'gust',weight:7},
  {kind:'birds',weight:8,when:m=>m.day>.45&&m.weather==='clear'},
  {kind:'gull',weight:6,when:m=>m.day>.2&&m.weather==='clear'},
  {kind:'pigeon',weight:4,when:m=>m.day>.6&&!m.busy},
  {kind:'cricket',weight:6,when:m=>m.day<.25&&m.weather!=='rain'},
  {kind:'horn',weight:4,when:m=>m.day<.45&&m.weather!=='rain'},
  {kind:'bell',weight:3,when:m=>m.day<.7},
  {kind:'hammer',weight:4,when:m=>m.day>.3},
  {kind:'drip',weight:10,when:m=>m.weather==='rain'},
  {kind:'windchime',weight:4,when:m=>m.weather==='clear'&&!m.busy},
  {kind:'siren',weight:2,when:m=>m.evening>.3&&m.busy},
];
// Exported for the acceptance check: the weighted event table and its mood filter.
export function citySoundscapeEvents(mood={}) {
  const state={busy:!!mood.busy,day:Number.isFinite(mood.day)?mood.day:0,evening:Number.isFinite(mood.evening)?mood.evening:0,
    weather:['clear','rain','fog'].includes(mood.weather)?mood.weather:'clear'};
  return EVENTS.filter(e=>!e.when||e.when(state)).map(e=>({kind:e.kind,weight:e.weight}));
}
function impulse(context,seconds,decay){
  const length=Math.floor(context.sampleRate*seconds),buffer=context.createBuffer(2,length,context.sampleRate);
  let seed=977;
  for(let c=0;c<2;c++){const data=buffer.getChannelData(c);for(let i=0;i<length;i++){seed=Math.imul(seed,1664525)+1013904223|0;data[i]=((seed>>>0)/2147483648-1)*Math.pow(1-i/length,decay);}}
  return buffer;
}
export function createCityAmbience(onState=()=>{}) {
  let context=null, master=null, analyser=null, disposed=false, active=false, enabled=false, fadeTimer=0, voice=null;
  const nodes=[], sources=[], samples=new Float32Array(256), transient=new Set();
  let volume=.18,effects=null,foley=null,ambientBus=null,effectsBus=null,voiceBus=null,inside=false,weather='clear';
  let pad=[],padFilter=null,padLevel=null,airFilter=null,airGain=null,verb=null,noise=null;
  let waterGain=null,murmurGain=null,lampGain=null,bedWater='0.000',bedMurmur='0.000',bedLamp='0.000';
  let eventTimer=0,chordTimer=0,chord=0,events=0;
  const listener={x:0,y:2.4,z:0,fx:0,fz:-1,origin:{x:0,z:0}};
  const mood={busy:false,day:0,evening:0,weather:'clear'};
  const levels={ambience:1,effects:1,voice:1};
  try {enabled=localStorage.getItem('aurago.desktop.sysworld.sound')==='true';}catch(_){}
  function ensure() {
    if(context||disposed)return;
    context=new (window.AudioContext||window.webkitAudioContext)();
    const node=n=>(nodes.push(n),n), source=n=>(sources.push(n),node(n));
    master=node(context.createGain());master.gain.value=0;
    analyser=node(context.createAnalyser());analyser.fftSize=512;master.connect(analyser);analyser.connect(context.destination);
    ambientBus=node(context.createGain());ambientBus.gain.value=levels.ambience;ambientBus.connect(master);
    effectsBus=node(context.createGain());effectsBus.gain.value=levels.effects;effectsBus.connect(master);
    voiceBus=node(context.createGain());voiceBus.gain.value=levels.voice;voiceBus.connect(master);
    verb=node(context.createConvolver());verb.buffer=impulse(context,3.4,2.6);
    const verbReturn=node(context.createGain());verbReturn.gain.value=.55;verb.connect(verbReturn).connect(ambientBus);
    voice=createTowerVoice(context,voiceBus);effects=createCityEffects(context,effectsBus,ambientBus);effects.environment(inside,weather);
    foley=createCityFoley(context,effectsBus,ambientBus);foley.environment(inside);foley.listener(listener.x,listener.y,listener.z,listener.fx,listener.fz,listener.origin.x,listener.origin.z);
    for(const [frequency,gain] of [[55,.07],[82.4069,.022],[110,.01]]) {
      const oscillator=source(context.createOscillator()), level=node(context.createGain());
      oscillator.frequency.value=frequency;level.gain.value=gain;
      oscillator.connect(level).connect(ambientBus);oscillator.start();
    }
    // Warm detuned saw pad; chords glide slowly and the filter follows the city's mood.
    padFilter=node(context.createBiquadFilter());padFilter.type='lowpass';padFilter.frequency.value=650;padFilter.Q.value=.6;
    padLevel=node(context.createGain());padLevel.gain.value=.042;
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
    for(let i=0;i<data.length;i++){brown=(brown+(Math.random()*2-1)*.018)/1.018;data[i]=brown;}
    const seam=data[data.length-1]-data[0];
    for(let i=0;i<data.length;i++)data[i]-=seam*i/(data.length-1);
    const air=source(context.createBufferSource());airFilter=node(context.createBiquadFilter());airGain=node(context.createGain());
    air.buffer=buffer;air.loop=true;airFilter.type='lowpass';airFilter.frequency.value=680;airFilter.Q.value=.25;airGain.gain.value=.32;
    air.connect(airFilter).connect(airGain).connect(ambientBus);air.start();
    const drift=source(context.createOscillator()), depth=node(context.createGain());
    drift.frequency.value=.075;depth.gain.value=160;drift.connect(depth).connect(airFilter.frequency);drift.start();
    noise=context.createBuffer(1,context.sampleRate*2,context.sampleRate);
    const white=noise.getChannelData(0);for(let i=0;i<white.length;i++)white[i]=Math.random()*2-1;
    // Two spatial beds: harbour water near the quay and a low population murmur.
    // Both are continuous, so they stay far below the transient events.
    const wash=source(context.createBufferSource());wash.buffer=noise;wash.loop=true;
    const washBand=node(context.createBiquadFilter());washBand.type='bandpass';washBand.frequency.value=620;washBand.Q.value=.5;
    const swell=source(context.createOscillator()),swellDepth=node(context.createGain());
    swell.frequency.value=.08;swellDepth.gain.value=140;swell.connect(swellDepth).connect(washBand.frequency);swell.start();
    waterGain=node(context.createGain());waterGain.gain.value=0;
    wash.connect(washBand).connect(waterGain).connect(ambientBus);wash.start();
    const crowd=source(context.createBufferSource());crowd.buffer=noise;crowd.loop=true;
    const crowdBand=node(context.createBiquadFilter());crowdBand.type='bandpass';crowdBand.frequency.value=320;crowdBand.Q.value=.7;
    const crowdDrift=source(context.createOscillator()),crowdDepth=node(context.createGain());
    crowdDrift.frequency.value=.13;crowdDepth.gain.value=90;crowdDrift.connect(crowdDepth).connect(crowdBand.frequency);crowdDrift.start();
    murmurGain=node(context.createGain());murmurGain.gain.value=0;
    crowd.connect(crowdBand).connect(murmurGain).connect(ambientBus);crowd.start();
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
    lampTrem.connect(lampFilter).connect(lampGain).connect(ambientBus);
    lamp.start();harmonic.start();
    applyMood();
    updateBeds();
  }
  const running=()=>!!context&&enabled&&active&&!disposed&&volume>0;
  // Every transient voice releases its nodes when its first source ends or the city goes quiet.
  function track(list){
    const entry={release(){if(!transient.delete(entry))return;for(const n of list){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}}};
    transient.add(entry);list[0].onended=entry.release;events++;return entry;
  }
  function out(level,pan,send){
    const panner=context.createStereoPanner(),reverb=context.createGain();panner.pan.value=pan;reverb.gain.value=send;
    level.connect(panner).connect(ambientBus);panner.connect(reverb).connect(verb);return [panner,reverb];
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
    if(kind==='horn'||kind==='gull')return ANCHORS.harbour;
    if(kind==='windchime')return ANCHORS.quay;
    if(kind==='bell')return ANCHORS.tower;
    if(kind==='pigeon')return ANCHORS.square;
    // The ring surrounds the place the view rests on, so the city stays around you.
    const angle=Math.random()*Math.PI*2,distance=24+Math.random()*44;
    return {x:listener.origin.x+Math.sin(angle)*distance,z:listener.origin.z+Math.cos(angle)*distance};
  }
  function chime(position){
    const t=context.currentTime,f=CHIMES[Math.floor(Math.random()*CHIMES.length)];
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
    const t=context.currentTime,base=weather==='rain'?.36:.32;
    airFilter.frequency.cancelScheduledValues(t);airGain.gain.cancelScheduledValues(t);
    airFilter.frequency.setTargetAtTime(1200+Math.random()*700,t,.8);airFilter.frequency.setTargetAtTime(680,t+2.6,1.4);
    airGain.gain.setTargetAtTime(base*1.7,t,.7);airGain.gain.setTargetAtTime(base,t+2.6,1.5);events++;
  }
  function horn(position){
    const t=context.currentTime,low=context.createBiquadFilter(),level=context.createGain();
    const {pan,fade}=reach(position);
    low.type='lowpass';low.frequency.value=420;
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.028*fade,t+.9);level.gain.setValueAtTime(.028*fade,t+3.2);level.gain.exponentialRampToValueAtTime(.0001,t+5.5);
    const tones=[69.3,103.83].map(f=>{const o=context.createOscillator();o.type='sawtooth';o.frequency.value=f;o.detune.value=Math.random()*8-4;o.connect(low);return o;});
    low.connect(level);track([...tones,low,level,...out(level,pan,1.6)]);tones.forEach(o=>{o.start(t);o.stop(t+5.6);});
  }
  function birds(position){
    const t=context.currentTime,count=2+Math.floor(Math.random()*3);
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.12+Math.random()*.1),o=context.createOscillator(),level=context.createGain(),f=2600+Math.random()*900;
      o.frequency.setValueAtTime(f,at);o.frequency.exponentialRampToValueAtTime(f*1.5,at+.07);
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.006*fade,at+.015);level.gain.exponentialRampToValueAtTime(.0001,at+.13);
      o.connect(level);track([o,level,...out(level,pan,.6)]);o.start(at);o.stop(at+.15);
    }
  }
  function gull(position){
    const t=context.currentTime,count=2+Math.floor(Math.random()*3),base=760+Math.random()*280;
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.2+Math.random()*.14),o=context.createOscillator(),form=context.createBiquadFilter(),level=context.createGain();
      o.type='sawtooth';o.frequency.setValueAtTime(base,at);o.frequency.exponentialRampToValueAtTime(base*.5,at+.26);
      form.type='bandpass';form.frequency.value=1500;form.Q.value=3.6;
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.012*fade,at+.04);level.gain.exponentialRampToValueAtTime(.0001,at+.34);
      o.connect(form).connect(level);track([o,form,level,...out(level,pan,.9)]);o.start(at);o.stop(at+.36);
    }
  }
  function pigeon(position){
    const t=context.currentTime,count=2+Math.floor(Math.random()*2),base=420+Math.random()*90;
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.17+Math.random()*.08),o=context.createOscillator(),form=context.createBiquadFilter(),level=context.createGain();
      o.type='triangle';o.frequency.setValueAtTime(base,at);o.frequency.linearRampToValueAtTime(base*.72,at+.12);
      form.type='lowpass';form.frequency.value=1100;
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.01*fade,at+.03);level.gain.exponentialRampToValueAtTime(.0001,at+.16);
      o.connect(form).connect(level);track([o,form,level,...out(level,pan,.5)]);o.start(at);o.stop(at+.18);
    }
  }
  function cricket(position){
    const t=context.currentTime,count=3+Math.floor(Math.random()*4),f=3900+Math.random()*900;
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
    const t=context.currentTime,count=2+Math.floor(Math.random()*3);
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.5+Math.random()*.35),src=context.createBufferSource(),band=context.createBiquadFilter(),level=context.createGain();
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
    const t=context.currentTime,count=2+Math.floor(Math.random()*4);
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.06+Math.random()*.14),o=context.createOscillator(),level=context.createGain();
      o.type='sine';o.frequency.setValueAtTime(900+Math.random()*700,at);o.frequency.exponentialRampToValueAtTime(320,at+.06);
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.008*fade,at+.005);level.gain.exponentialRampToValueAtTime(.0001,at+.12);
      o.connect(level);track([o,level,...out(level,pan,.4)]);o.start(at);o.stop(at+.14);
    }
  }
  function windchime(position){
    const t=context.currentTime,count=3+Math.floor(Math.random()*4),scale=[1046.5,1174.7,1396.9,1567.98,1760,2093];
    const {pan,fade}=reach(position);
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.09+Math.random()*.13),f=scale[Math.floor(Math.random()*scale.length)];
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
  function pickEvent(){
    const pool=EVENTS.filter(e=>!e.when||e.when(mood));
    if(!pool.length)return 'chime';
    let total=0;for(const e of pool)total+=e.weight;
    let roll=Math.random()*total;
    for(const e of pool){roll-=e.weight;if(roll<=0)return e.kind;}
    return pool[pool.length-1].kind;
  }
  function event(){
    if(transient.size>=MAX_EVENTS)return;
    const kind=pickEvent(),position=positionFor(kind);
    VOICES[kind]?.(position);
  }
  function schedule(){
    clearTimeout(eventTimer);eventTimer=0;if(!running())return;
    eventTimer=setTimeout(()=>{eventTimer=0;if(!running())return;event();schedule();},(mood.busy?1900:3600)+Math.random()*9000);
  }
  function progress(){
    clearTimeout(chordTimer);chordTimer=0;if(!running())return;
    chordTimer=setTimeout(()=>{
      chordTimer=0;if(!running())return;chord=(chord+1)%CHORDS.length;
      pad.forEach((pair,i)=>pair.forEach(o=>o.frequency.setTargetAtTime(CHORDS[chord][i],context.currentTime,1.8)));progress();
    },16000);
  }
  function quiet(){clearTimeout(eventTimer);clearTimeout(chordTimer);eventTimer=chordTimer=0;for(const v of [...transient])v.release();}
  function applyMood(){
    if(!context)return;const t=context.currentTime;
    padFilter.frequency.setTargetAtTime(mood.busy?1500:650+mood.day*450+mood.evening*150,t,2.5);
    padLevel.gain.setTargetAtTime((mood.busy?.055:.042)*(mood.weather==='rain'?.75:1),t,2);
  }
  // The beds follow the listener: water grows near the quay, the murmur follows the
  // city's population. Values are quantized so a per-frame listener update does not
  // queue a new ramp every frame.
  function updateBeds(){
    if(!context||!waterGain||!murmurGain||!lampGain)return;
    // The harbour is a bed right at the quay and a faint wash across the city.
    const quayX=Math.max(-76,Math.min(76,listener.origin.x)),distance=Math.hypot(listener.origin.x-quayX,listener.origin.z-77);
    const near=Math.max(0,1-(distance-6)/42);
    const wash=Math.max(0,1-(distance-25)/110)*.18;
    const water=(inside?0:Math.min(1,near+wash)*.05*(weather==='rain'?1.2:1)).toFixed(3);
    const crowd=((mood.busy?.03:.012)*(.35+mood.day*.65)*(inside?.25:1)).toFixed(3);
    // The lantern hum follows the same quay proximity and fades out by noon;
    // indoors it is completely silent. Its ceiling is far below the city bed.
    const lampDistance=Math.hypot(listener.origin.x-quayX,listener.origin.z-75.5);
    const lampNear=Math.max(0,1-(lampDistance-8)/46);
    const lamp=(inside?0:Math.min(1,lampNear)*.012*(1-mood.day)*(weather==='rain'?.6:1)).toFixed(3);
    if(water===bedWater&&crowd===bedMurmur&&lamp===bedLamp)return;
    bedWater=water;bedMurmur=crowd;bedLamp=lamp;
    const t=context.currentTime;
    waterGain.gain.setTargetAtTime(Number(water),t,.5);
    murmurGain.gain.setTargetAtTime(Number(crowd),t,.8);
    lampGain.gain.setTargetAtTime(Number(lamp),t,.8);
  }
  function sync(gesture=false) {
    clearTimeout(fadeTimer);
    if(disposed)return;
    if(gesture&&enabled){try{ensure();}catch(_){enabled=false;onState(false);return;}}
    if(!context)return;
    voice?.setActive(enabled&&active&&volume>0&&levels.voice>0);effects?.setActive(enabled&&active&&volume>0);
    foley?.setActive(enabled&&active&&volume>0&&levels.effects>0);
    if(enabled&&active) {
      // First unlock always comes from a user gesture; later resumes follow focus.
      void context.resume().catch(()=>{});
      master.gain.cancelScheduledValues(context.currentTime);
      master.gain.setTargetAtTime(volume,context.currentTime,.25);
      if(!eventTimer)schedule();if(!chordTimer)progress();
    } else {
      quiet();
      master.gain.cancelScheduledValues(context.currentTime);
      master.gain.setTargetAtTime(0,context.currentTime,.035);
      fadeTimer=setTimeout(()=>{if(!disposed&&(!enabled||!active))void context.suspend().catch(()=>{});},180);
    }
  }
  return {
    enabled:()=>enabled,
    toggle() {enabled=!enabled;try{localStorage.setItem('aurago.desktop.sysworld.sound',String(enabled));}catch(_){}sync(true);onState(enabled);},
    unlock() {sync(true);},
    setActive(value) {if(active===value)return;active=value;sync();},
    setVolume(value) {if(Number.isFinite(value)){volume=Math.max(0,Math.min(.35,value));sync();}},
    setListener(x,y,z,fx,fz,ox,oz) {Object.assign(listener,{x,y,z,fx,fz});listener.origin.x=ox??x;listener.origin.z=oz??z;
      voice?.setListener(x,y,z,fx,fz);effects?.listener(x,y,z,fx,fz);foley?.listener(x,y,z,fx,fz,listener.origin.x,listener.origin.z);updateBeds();},
    setChannel(key,value){if(!(key in levels)||!Number.isFinite(value))return;levels[key]=Math.max(0,Math.min(1,value));const bus={ambience:ambientBus,effects:effectsBus,voice:voiceBus}[key];if(bus)bus.gain.setTargetAtTime(levels[key],context.currentTime,.05);sync();},
    // World interactions keep their own channel; robot actions are routed to the foley.
    effect(kind,x,y,z,id){if(String(kind).startsWith('robot_')){foley?.action(kind,x,y,z,id);return;}effects?.play(kind,x,y,z);},
    gait(id,x,y,z,moving,carrying,style){foley?.gait(id,x,y,z,moving,carrying,style);},
    setEnvironment(interior,condition){inside=!!interior;if(['clear','rain','fog'].includes(condition))weather=condition;effects?.environment(inside,weather);foley?.environment(inside);updateBeds();},
    setMood(value={}){Object.assign(mood,{busy:!!value.busy,day:Number.isFinite(value.day)?value.day:mood.day,evening:Number.isFinite(value.evening)?value.evening:mood.evening,
      weather:['clear','rain','fog'].includes(value.weather)?value.weather:mood.weather});applyMood();updateBeds();},
    // Rolling thunder after a lightning flash; the delay (s) doubles as a distance cue.
    thunder(delay=1){
      if(!running()||inside||transient.size>=MAX_EVENTS)return;
      const wait=Math.max(0,Math.min(5,delay)),t=context.currentTime+wait,far=Math.min(1,wait/4);
      const src=context.createBufferSource(),low=context.createBiquadFilter(),level=context.createGain();
      src.buffer=noise;src.loop=true;low.type='lowpass';low.Q.value=.7;
      low.frequency.setValueAtTime(900-far*500,t);low.frequency.exponentialRampToValueAtTime(70,t+3.5);
      level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.09*(1-far*.5),t+.06+far*.25);
      level.gain.exponentialRampToValueAtTime(.025,t+1.3);level.gain.exponentialRampToValueAtTime(.0001,t+5.5);
      src.connect(low).connect(level);track([src,low,level,...out(level,Math.random()*.8-.4,.8)]);src.start(t);src.stop(t+5.6);
    },
    stats() {let rms=0;if(analyser&&context.state==='running'){analyser.getFloatTimeDomainData(samples);for(const x of samples)rms+=x*x;}
      return {voice:voice?.stats(),effects:effects?.stats(),foley:foley?.stats(),channels:{...levels},enabled,active,state:context?.state||'uninitialized',volume,rms:Math.sqrt(rms/samples.length),
        soundscape:{chord,events,transient:transient.size,scheduled:!!eventTimer,mood:{...mood},beds:{water:Number(bedWater),murmur:Number(bedMurmur),lamp:Number(bedLamp)},listener:{x:listener.x,z:listener.z}}};},
    dispose() {if(disposed)return;disposed=true;quiet();foley?.dispose();effects?.dispose();voice?.dispose();clearTimeout(fadeTimer);sources.forEach(n=>{try{n.stop();}catch(_){}});nodes.forEach(n=>n.disconnect());if(context)void context.close().catch(()=>{});},
  };
}
