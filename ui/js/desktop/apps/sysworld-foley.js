// Procedural robot foley. Every articulated resident answers what it does with a
// quiet, positional sound: a servo settle, a footfall, a parcel rustle, a tool
// tap, a greeting chirp, a steering whine. Nothing is sampled and nothing is
// fetched; the whole set is synthesized from oscillators and filtered noise.
//
// The set is bounded on purpose: at most MAX_VOICES sounds are alive, a sound
// dies beyond MAX_DISTANCE, and one resident cannot retrigger itself faster
// than every RETRIGGER seconds. Nothing sounds while the city is muted,
// hidden, unfocused or in map mode.
export function createCityFoley(context,destination,ambience=destination) {
  const MAX_VOICES=10,MAX_DISTANCE=45,RETRIGGER=.25;
  const voices=new Set(),gaits=new Map(),recent=new Map();
  const listener={x:0,y:0,z:0,fx:0,fz:-1};
  let active=false,inside=false,played=0,steps=0;

  const noise=context.createBuffer(1,context.sampleRate*2,context.sampleRate);
  let seed=9371;const samples=noise.getChannelData(0);
  for(let i=0;i<samples.length;i++){seed=Math.imul(seed,1664525)+1013904223|0;samples[i]=(seed>>>0)/2147483648-1;}

  // A sound is a short stack of layers: noise taps carry the transient, tones
  // carry the body. Gains stay low because the city bed has to stay dominant.
  const SOUNDS={
    step:{gain:.05,layers:[
      {wave:'noise',f:1900,q:1.7,dur:.05,lvl:.5},
      {wave:'noise',f:520,q:.9,dur:.1,lvl:.4},
      {wave:'tone',f:104,f2:74,dur:.12,lvl:.3}]},
    step_carry:{gain:.055,layers:[
      {wave:'noise',f:1250,q:1.2,dur:.06,lvl:.45},
      {wave:'noise',f:340,q:.8,dur:.13,lvl:.5},
      {wave:'tone',f:88,f2:62,dur:.15,lvl:.36}]},
    glide:{gain:.02,layers:[
      {wave:'noise',f:700,q:.6,dur:.26,lvl:.8},
      {wave:'tone',f:150,f2:120,dur:.24,lvl:.18}]},
    greet:{gain:.038,layers:[
      {wave:'tone',f:520,f2:740,dur:.11,lvl:.5,type:'triangle'},
      {wave:'tone',f:790,f2:960,dur:.13,lvl:.3,type:'triangle',delay:.1}]},
    work:{gain:.034,layers:[
      {wave:'noise',f:2700,q:2.6,dur:.05,lvl:.6},
      {wave:'tone',f:240,f2:175,dur:.22,lvl:.26,type:'sawtooth'}]},
    carry:{gain:.026,layers:[
      {wave:'noise',f:950,q:.7,dur:.12,lvl:.7},
      {wave:'noise',f:2300,q:1.5,dur:.07,lvl:.25}]},
    idle:{gain:.016,layers:[{wave:'noise',f:3400,q:3.2,dur:.028,lvl:.7}]},
    turn:{gain:.02,layers:[{wave:'tone',f:900,f2:1440,dur:.18,lvl:.4,type:'sawtooth'}]},
  };

  function voice(kind,x,y,z,id){
    const name=kind.startsWith('robot_')?kind.slice(6):kind;
    const spec=SOUNDS[name];if(!spec||!active)return false;
    if(voices.size>=MAX_VOICES)return false;
    const dx=x-listener.x,dz=z-listener.z,distance=Math.hypot(dx,dz);
    if(distance>MAX_DISTANCE)return false;
    const now=context.currentTime;
    if(id!=null){const last=recent.get(id);if(last!=null&&now-last<RETRIGGER)return false;recent.set(id,now);}
    const level=spec.gain/(1+distance*.16)*(inside?.5:1);
    if(level<.0004)return false;
    const pan=Math.max(-1,Math.min(1,(dx*-listener.fz+dz*listener.fx)/Math.max(distance,1)));
    const bus=context.createGain(),panner=context.createStereoPanner(),send=context.createGain();
    bus.gain.value=level;panner.pan.value=pan;send.gain.value=inside?.2:.4;
    bus.connect(panner).connect(destination);panner.connect(send).connect(ambience);
    const nodes=[];
    const entry={release(){if(!voices.delete(entry))return;for(const n of nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}for(const n of[bus,panner,send]){try{n.disconnect();}catch(_){}}}};
    voices.add(entry);
    let latest=null,latestEnd=0;
    for(const layer of spec.layers) {
      const at=now+(layer.delay||0),dur=Math.max(layer.dur,(layer.attack||.006)+.02);
      const levelNode=context.createGain();
      levelNode.gain.setValueAtTime(0,at);
      levelNode.gain.linearRampToValueAtTime(layer.lvl,at+(layer.attack||.006));
      levelNode.gain.exponentialRampToValueAtTime(.0001,at+dur);
      levelNode.connect(bus);
      let source;
      if(layer.wave==='noise') {
        source=context.createBufferSource();source.buffer=noise;source.loop=true;
        const band=context.createBiquadFilter();band.type='bandpass';band.frequency.value=layer.f;band.Q.value=layer.q||1;
        source.connect(band).connect(levelNode);nodes.push(band);
      } else {
        source=context.createOscillator();source.type=layer.type||'sine';
        source.frequency.setValueAtTime(layer.f,at);
        if(layer.f2)source.frequency.exponentialRampToValueAtTime(Math.max(20,layer.f2),at+dur);
        source.connect(levelNode);
      }
      nodes.push(source,levelNode);
      const end=at+dur+.02;if(end>latestEnd){latestEnd=end;latest=source;}
      source.start(at);source.stop(end);
    }
    // Only the longest layer ends the voice, so a two-note greeting is not cut.
    if(latest)latest.onended=entry.release;
    played++;
    return true;
  }

  function gait(id,x,y,z,moving,carrying,style){
    if(!active)return;
    const g=gaits.get(id);
    if(!g){gaits.set(id,{x,z,phase:0});return;}
    const dx=x-g.x,dz=z-g.z,distance=Math.hypot(dx,dz);
    g.x=x;g.z=z;
    if(!moving){g.phase=0;return;}
    if(distance>8)return;                     // a respawn is not a stride
    g.phase+=distance;
    const stride=style==='glide'?1.6:carrying?1.05:.8;
    if(g.phase<stride)return;
    g.phase-=stride;
    if(style==='glide')voice('glide',x,y,z,id);
    else voice(carrying?'step_carry':'step',x,y,z,id);
    steps++;
  }

  function clear(){
    for(const v of [...voices])v.release();
    for(const [,g]of gaits){g.x=listener.x;g.z=listener.z;g.phase=0;}
    recent.clear();
  }

  return {
    setActive(value){if(active===value)return;active=value;if(!value)clear();},
    environment(value){inside=!!value;},
    listener(x,y,z,fx,fz){Object.assign(listener,{x,y,z,fx,fz});},
    action(kind,x,y,z,id){return voice(kind,x,y,z,id);},
    gait,
    reset(){gaits.clear();recent.clear();},
    stats:()=>({voices:voices.size,played,steps,gaits:gaits.size,inside}),
    dispose(){active=false;clear();},
  };
}
