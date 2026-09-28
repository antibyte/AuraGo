import { createTowerVoice } from './sysworld-voice.js';
import { createCityEffects } from './sysworld-effects.js';
// Synthesized city soundscape: a sub drone, an evolving reverberant chord pad, filtered air with
// wind gusts and sparse distant events (traffic swooshes, data chimes, birds by day, a harbour
// horn at night, thunder after lightning). Local Web Audio only; nothing is scheduled while the
// city is muted, hidden, unfocused or in map mode.
const CHORDS=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]];
const CHIMES=[440,523.25,587.33,659.25,783.99,880,1046.5];
const MAX_EVENTS=12;
function impulse(context,seconds,decay){
  const length=Math.floor(context.sampleRate*seconds),buffer=context.createBuffer(2,length,context.sampleRate);
  let seed=977;
  for(let c=0;c<2;c++){const data=buffer.getChannelData(c);for(let i=0;i<length;i++){seed=Math.imul(seed,1664525)+1013904223|0;data[i]=((seed>>>0)/2147483648-1)*Math.pow(1-i/length,decay);}}
  return buffer;
}
export function createCityAmbience(onState=()=>{}) {
  let context=null, master=null, analyser=null, disposed=false, active=false, enabled=false, fadeTimer=0, voice=null;
  const nodes=[], sources=[], samples=new Float32Array(256), transient=new Set();
  let volume=.18,effects=null,ambientBus=null,effectsBus=null,voiceBus=null,inside=false,weather='clear';
  let pad=[],padFilter=null,padLevel=null,airFilter=null,airGain=null,verb=null,noise=null;
  let eventTimer=0,chordTimer=0,chord=0,events=0;
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
    applyMood();
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
  function chime(){
    const t=context.currentTime,f=CHIMES[Math.floor(Math.random()*CHIMES.length)];
    const a=context.createOscillator(),b=context.createOscillator(),partial=context.createGain(),level=context.createGain();
    a.frequency.value=f;b.frequency.value=f*2.76;partial.gain.value=.22;
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(mood.busy?.02:.014,t+.008);level.gain.exponentialRampToValueAtTime(.0001,t+2.8);
    a.connect(level);b.connect(partial).connect(level);
    track([a,b,partial,level,...out(level,Math.random()*1.4-.7,1.4)]);a.start(t);b.start(t);a.stop(t+2.9);b.stop(t+2.9);
  }
  function swoosh(){
    const t=context.currentTime,src=context.createBufferSource(),band=context.createBiquadFilter(),level=context.createGain(),side=Math.random()<.5?-1:1;
    src.buffer=noise;src.loop=true;band.type='bandpass';band.Q.value=1.4;
    band.frequency.setValueAtTime(260,t);band.frequency.exponentialRampToValueAtTime(1100,t+1.6);band.frequency.exponentialRampToValueAtTime(380,t+3.4);
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.03,t+1.6);level.gain.exponentialRampToValueAtTime(.0001,t+3.6);
    src.connect(band).connect(level);const [panner,reverb]=out(level,-.9*side,.5);
    panner.pan.setValueAtTime(-.9*side,t);panner.pan.linearRampToValueAtTime(.9*side,t+3.4);
    track([src,band,level,panner,reverb]);src.start(t);src.stop(t+3.7);
  }
  function gust(){
    const t=context.currentTime,base=weather==='rain'?.36:.32;
    airFilter.frequency.cancelScheduledValues(t);airGain.gain.cancelScheduledValues(t);
    airFilter.frequency.setTargetAtTime(1200+Math.random()*700,t,.8);airFilter.frequency.setTargetAtTime(680,t+2.6,1.4);
    airGain.gain.setTargetAtTime(base*1.7,t,.7);airGain.gain.setTargetAtTime(base,t+2.6,1.5);events++;
  }
  function horn(){
    const t=context.currentTime,low=context.createBiquadFilter(),level=context.createGain();
    low.type='lowpass';low.frequency.value=420;
    level.gain.setValueAtTime(0,t);level.gain.linearRampToValueAtTime(.028,t+.9);level.gain.setValueAtTime(.028,t+3.2);level.gain.exponentialRampToValueAtTime(.0001,t+5.5);
    const tones=[69.3,103.83].map(f=>{const o=context.createOscillator();o.type='sawtooth';o.frequency.value=f;o.detune.value=Math.random()*8-4;o.connect(low);return o;});
    low.connect(level);track([...tones,low,level,...out(level,-.55,1.6)]);tones.forEach(o=>{o.start(t);o.stop(t+5.6);});
  }
  function birds(){
    const t=context.currentTime,count=2+Math.floor(Math.random()*3),pan=Math.random()*1.2-.6;
    for(let i=0;i<count&&transient.size<MAX_EVENTS;i++){
      const at=t+i*(.12+Math.random()*.1),o=context.createOscillator(),level=context.createGain(),f=2600+Math.random()*900;
      o.frequency.setValueAtTime(f,at);o.frequency.exponentialRampToValueAtTime(f*1.5,at+.07);
      level.gain.setValueAtTime(0,at);level.gain.linearRampToValueAtTime(.006,at+.015);level.gain.exponentialRampToValueAtTime(.0001,at+.13);
      o.connect(level);track([o,level,...out(level,pan,.6)]);o.start(at);o.stop(at+.15);
    }
  }
  function event(){
    if(transient.size>=MAX_EVENTS)return;
    const r=Math.random(),night=1-mood.day;
    if(r<.34)chime();else if(r<.62)swoosh();else if(r<.8)gust();
    else if(night>.5&&mood.weather!=='rain'&&r<.88)horn();
    else if(mood.day>.4&&mood.weather==='clear')birds();else chime();
  }
  function schedule(){
    clearTimeout(eventTimer);eventTimer=0;if(!running())return;
    eventTimer=setTimeout(()=>{eventTimer=0;if(!running())return;event();schedule();},(mood.busy?2200:4200)+Math.random()*6000);
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
  function sync(gesture=false) {
    clearTimeout(fadeTimer);
    if(disposed)return;
    if(gesture&&enabled){try{ensure();}catch(_){enabled=false;onState(false);return;}}
    if(!context)return;
    voice?.setActive(enabled&&active&&volume>0&&levels.voice>0);effects?.setActive(enabled&&active&&volume>0);
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
    setListener(x,y,z,fx,fz) {voice?.setListener(x,y,z,fx,fz);effects?.listener(x,y,z,fx,fz);},
    setChannel(key,value){if(!(key in levels)||!Number.isFinite(value))return;levels[key]=Math.max(0,Math.min(1,value));const bus={ambience:ambientBus,effects:effectsBus,voice:voiceBus}[key];if(bus)bus.gain.setTargetAtTime(levels[key],context.currentTime,.05);sync();},
    effect(kind,x,y,z){effects?.play(kind,x,y,z);},setEnvironment(interior,condition){inside=!!interior;if(['clear','rain','fog'].includes(condition))weather=condition;effects?.environment(inside,weather);},
    setMood(value={}){Object.assign(mood,{busy:!!value.busy,day:Number.isFinite(value.day)?value.day:mood.day,evening:Number.isFinite(value.evening)?value.evening:mood.evening,
      weather:['clear','rain','fog'].includes(value.weather)?value.weather:mood.weather});applyMood();},
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
      return {voice:voice?.stats(),effects:effects?.stats(),channels:{...levels},enabled,active,state:context?.state||'uninitialized',volume,rms:Math.sqrt(rms/samples.length),
        soundscape:{chord,events,transient:transient.size,scheduled:!!eventTimer,mood:{...mood}}};},
    dispose() {if(disposed)return;disposed=true;quiet();effects?.dispose();voice?.dispose();clearTimeout(fadeTimer);sources.forEach(n=>{try{n.stop();}catch(_){}});nodes.forEach(n=>n.disconnect());if(context)void context.close().catch(()=>{});},
  };
}
