import { createTowerVoice } from './sysworld-voice.js';
// Quiet synthesized city air and a slowly moving harmonic bed; no audio downloads.
export function createCityAmbience(onState=()=>{}) {
  let context=null, master=null, analyser=null, disposed=false, active=false, enabled=false, fadeTimer=0, voice=null;
  const nodes=[], sources=[], samples=new Float32Array(256);
  let volume=.18;
  try {enabled=localStorage.getItem('aurago.desktop.sysworld.sound')==='true';}catch(_){}
  function ensure() {
    if(context||disposed)return;
    context=new (window.AudioContext||window.webkitAudioContext)();
    const node=n=>(nodes.push(n),n), source=n=>(sources.push(n),node(n));
    master=node(context.createGain());master.gain.value=0;
    analyser=node(context.createAnalyser());analyser.fftSize=512;master.connect(analyser);analyser.connect(context.destination);
    voice=createTowerVoice(context,master);
    for(const [frequency,gain] of [[55,.1],[82.4069,.032],[110,.017],[164.8138,.009]]) {
      const oscillator=source(context.createOscillator()), level=node(context.createGain());
      oscillator.frequency.value=frequency;level.gain.value=gain;
      oscillator.connect(level).connect(master);oscillator.start();
    }
    const buffer=context.createBuffer(1,context.sampleRate*4,context.sampleRate), data=buffer.getChannelData(0);
    let brown=0;
    for(let i=0;i<data.length;i++){brown=(brown+(Math.random()*2-1)*.018)/1.018;data[i]=brown;}
    const seam=data[data.length-1]-data[0];
    for(let i=0;i<data.length;i++)data[i]-=seam*i/(data.length-1);
    const air=source(context.createBufferSource()), filter=node(context.createBiquadFilter()), airGain=node(context.createGain());
    air.buffer=buffer;air.loop=true;filter.type='lowpass';filter.frequency.value=680;filter.Q.value=.25;airGain.gain.value=.32;
    air.connect(filter).connect(airGain).connect(master);air.start();
    const drift=source(context.createOscillator()), depth=node(context.createGain());
    drift.frequency.value=.075;depth.gain.value=160;drift.connect(depth).connect(filter.frequency);drift.start();
  }
  function sync(gesture=false) {
    clearTimeout(fadeTimer);
    if(disposed)return;
    if(gesture&&enabled){try{ensure();}catch(_){enabled=false;onState(false);return;}}
    if(!context)return;
    voice?.setActive(enabled&&active&&volume>0);
    if(enabled&&active) {
      // First unlock always comes from a user gesture; later resumes follow focus.
      void context.resume().catch(()=>{});
      master.gain.cancelScheduledValues(context.currentTime);
      master.gain.setTargetAtTime(volume,context.currentTime,.25);
    } else {
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
    setListener(x,y,z,fx,fz) {voice?.setListener(x,y,z,fx,fz);},
    stats() {let rms=0;if(analyser&&context.state==='running'){analyser.getFloatTimeDomainData(samples);for(const x of samples)rms+=x*x;}
      return {voice:voice?.stats(),enabled,active,state:context?.state||'uninitialized',volume,rms:Math.sqrt(rms/samples.length)};},
    dispose() {if(disposed)return;disposed=true;voice?.dispose();clearTimeout(fadeTimer);sources.forEach(n=>{try{n.stop();}catch(_){}});nodes.forEach(n=>n.disconnect());if(context)void context.close().catch(()=>{});},
  };
}
