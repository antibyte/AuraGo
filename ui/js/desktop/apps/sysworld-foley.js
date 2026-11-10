import {surfaceAt} from './sysworld-zones.js';

// Procedural robot foley. Every articulated resident answers what it does with a
// quiet, positional sound: a servo settle, a footfall, a parcel rustle, a tool
// tap, a greeting chirp, a steering whine. Nothing is sampled and nothing is
// fetched; the whole set is synthesized from oscillators and filtered noise.
//
// The set is bounded on purpose: at most MAX_VOICES sounds are alive, a sound
// dies beyond MAX_DISTANCE of the acoustic origin, and one resident cannot
// retrigger itself faster than every RETRIGGER seconds. The nearest residents
// win a voice. Nothing sounds while the city is muted, hidden, unfocused or in
// map mode.
// Footfalls by surface (phase 5). The emitter's position decides the surface (sysworld-zones.js surfaceAt), so every
// kind in SURFACE_KINDS needs a voice here. Gains stay >= .06: a footfall beside the listener must clear the bed.
// `body` is the surface's short resonance under the heel strike (a narrow noise band at 100-400 Hz, 70 ms; its level is high because so narrow a band passes only a few tens of Hz of the noise): what makes the
// strike read as a footstep on that surface. None for garden and grating, which have their own character.
export const STEP_VOICES=Object.freeze({
  road:{gain:.049,body:{f:150,q:8,lvl:1.8},layers:[{wave:'noise',f:900,q:.8,dur:.06,lvl:.5},{wave:'noise',f:2600,q:1.4,dur:.03,lvl:.25},{wave:'tone',f:90,f2:70,dur:.08,lvl:.25}]},
  pavement:{gain:.049,body:{f:200,q:9,lvl:1.9},layers:[{wave:'noise',f:1900,q:1.7,dur:.05,lvl:.5},{wave:'noise',f:520,q:.9,dur:.1,lvl:.4},{wave:'tone',f:104,f2:74,dur:.12,lvl:.3}]},
  kerb:{gain:.0504,body:{f:260,q:9,lvl:1.8},layers:[{wave:'noise',f:2400,q:2,dur:.03,lvl:.6},{wave:'noise',f:700,q:1,dur:.07,lvl:.3},{wave:'tone',f:180,f2:120,dur:.06,lvl:.35}]},
  stairs:{gain:.0525,body:{f:170,q:10,lvl:2.2},layers:[{wave:'noise',f:1700,q:1.3,dur:.04,lvl:.5},{wave:'noise',f:600,q:.8,dur:.11,lvl:.4},{wave:'tone',f:140,f2:95,dur:.09,lvl:.35},{wave:'noise',f:3800,q:1,dur:.05,lvl:.15,delay:.06}]},
  quay:{gain:.0525,body:{f:130,q:8,lvl:2},layers:[{wave:'tone',f:170,f2:130,dur:.14,lvl:.45,type:'triangle'},{wave:'noise',f:700,q:2.5,dur:.12,lvl:.35},{wave:'noise',f:2000,q:1,dur:.03,lvl:.2}]},
  lobby:{gain:.0476,body:{f:300,q:10,lvl:1.7},layers:[{wave:'noise',f:3200,q:2.2,dur:.035,lvl:.55},{wave:'noise',f:1100,q:1.4,dur:.07,lvl:.3},{wave:'tone',f:220,f2:160,dur:.07,lvl:.2}]},
  grating:{gain:.049,layers:[{wave:'noise',f:3000,q:3,dur:.025,lvl:.45},{wave:'tone',f:820,dur:.18,lvl:.18},{wave:'tone',f:1370,dur:.14,lvl:.12},{wave:'tone',f:95,f2:70,dur:.08,lvl:.25}]},
  garden:{gain:.0434,layers:[{wave:'noise',f:4200,q:.7,dur:.09,lvl:.4},{wave:'noise',f:1800,q:.6,dur:.07,lvl:.3},{wave:'noise',f:600,q:.8,dur:.06,lvl:.15}]},
});

// A footfall is a heel strike followed by the sole (the toe): the heel is the surface voice with longer decays (low
// tone 120 ms, noise 80-90 ms; the garden rustles for ~200 ms; these are the times the layer takes to fall 20 dB, a short
// tail follows), the toe is the surface's noise layers only, a little brighter (x1.25), 55 % of the heel's level, 80 ms long, 70-95 ms after the heel. Both live in one voice's node chain,
// so a footfall still takes one voice. PAIRS[surface] is the layer list; layers with `toe` start after the gap. A surface with a
// `body` adds one narrow resonance layer (`body:true`) to the heel; the toe never copies it.
export const PAIR={toeGap:Object.freeze([.07,.095]),toeLevel:.55,toeBright:1.25,toeDur:.08,heelTone:.12,heelNoise:Object.freeze([.08,.09]),rustle:Object.freeze([.2,.16,.12])};
export const PAIRS=Object.freeze(Object.fromEntries(Object.entries(STEP_VOICES).map(([name,spec])=>{
  let rustle=0;const heel=spec.layers.map(l=>{
    if(l.wave==='noise'&&name==='garden')return {...l,dur:PAIR.rustle[Math.min(rustle++,PAIR.rustle.length-1)]};
    if(l.wave==='noise')return {...l,dur:Math.max(PAIR.heelNoise[0],Math.min(PAIR.heelNoise[1],l.dur*1.6))};
    return l.f<300?{...l,dur:PAIR.heelTone}:l;});
  const toe=spec.layers.filter(l=>l.wave==='noise'&&!l.delay).map(l=>({...l,f:l.f*PAIR.toeBright,lvl:l.lvl*PAIR.toeLevel,dur:PAIR.toeDur,toe:true}));
  return [name,[...heel,...(spec.body?[{wave:'noise',f:spec.body.f,q:spec.body.q,dur:.07,lvl:spec.body.lvl,body:true}]:[]),...toe]];})));

export function createCityFoley(context,destination,ambience=destination) {
  const MAX_DISTANCE=48,RETRIGGER=.25,CARRY={pitch:.85,length:1.2,scale:1.07},FRONT={scale:.48,pitch:1.6,length:.6},HIND={scale:.41,pitch:1.5,length:.6,delay:.025};
  let maxVoices=10,companionBeat=null,companionSteps=0;const bySurface=Object.fromEntries(Object.keys(STEP_VOICES).map(k=>[k,0]));
  // The robot layer has to be heard over the city bed. The bed sits at a mixed
  // RMS of roughly .01, so a footfall that stays below that is swallowed by the
  // drones and the murmur. LAYER lifts the whole robot layer above the bed, the
  // table below keeps the balance between the single sounds.
  const LAYER=4.5;
  const voices=new Set(),gaits=new Map(),recent=new Map();
  const listener={x:0,y:0,z:0,fx:0,fz:-1};
  // The acoustic origin is where the view rests, not where the camera floats:
  // on the street ahead of you, or on the district the overview is centred on.
  const origin={x:0,z:0};
  let active=false,inside=false,played=0,steps=0,peak=0;
  const rejected={kind:0,inactive:0,distant:0,pool:0,retrigger:0,quiet:0};

  const noise=context.createBuffer(1,context.sampleRate*2,context.sampleRate);
  let seed=9371,toeSeed=4417;const toeRandom=()=>(toeSeed=(Math.imul(toeSeed,1664525)+1013904223)>>>0)/4294967296;
  const samples=noise.getChannelData(0);
  for(let i=0;i<samples.length;i++){seed=Math.imul(seed,1664525)+1013904223|0;samples[i]=(seed>>>0)/2147483648-1;}

  // A sound is a short stack of layers: noise taps carry the transient, tones
  // carry the body. The gains are relative to each other; LAYER sets the level
  // of the whole robot layer against the city bed.
  const SOUNDS={
    step:{gain:.07,layers:[
      {wave:'noise',f:1900,q:1.7,dur:.05,lvl:.5},
      {wave:'noise',f:520,q:.9,dur:.1,lvl:.4},
      {wave:'tone',f:104,f2:74,dur:.12,lvl:.3}]},
    step_carry:{gain:.075,layers:[
      {wave:'noise',f:1250,q:1.2,dur:.06,lvl:.45},
      {wave:'noise',f:340,q:.8,dur:.13,lvl:.5},
      {wave:'tone',f:88,f2:62,dur:.15,lvl:.36}]},
    glide:{gain:.028,layers:[
      {wave:'noise',f:700,q:.6,dur:.26,lvl:.8},
      {wave:'tone',f:150,f2:120,dur:.24,lvl:.18}]},
    greet:{gain:.05,layers:[
      {wave:'tone',f:520,f2:740,dur:.11,lvl:.5,type:'triangle'},
      {wave:'tone',f:790,f2:960,dur:.13,lvl:.3,type:'triangle',delay:.1}]},
    work:{gain:.045,layers:[
      {wave:'noise',f:2700,q:2.6,dur:.05,lvl:.6},
      {wave:'tone',f:240,f2:175,dur:.22,lvl:.26,type:'sawtooth'}]},
    carry:{gain:.034,layers:[
      {wave:'noise',f:950,q:.7,dur:.12,lvl:.7},
      {wave:'noise',f:2300,q:1.5,dur:.07,lvl:.25}]},
    idle:{gain:.02,layers:[{wave:'noise',f:3400,q:3.2,dur:.028,lvl:.7}]},
    turn:{gain:.026,layers:[{wave:'tone',f:900,f2:1440,dur:.18,lvl:.4,type:'sawtooth'}]},
  };

  function voice(kind,x,y,z,id,surface,mod){
    const name=kind.startsWith('robot_')?kind.slice(6):kind;
    const spec=surface?STEP_VOICES[surface]:SOUNDS[name];if(!spec){rejected.kind++;return false;}if(!active){rejected.inactive++;return false;}
    // Distance runs on the ground: a resident is on the street, and the listener
    // either stands on it or looks down at the district it belongs to.
    const dx=x-origin.x,dz=z-origin.z,distance=Math.hypot(dx,dz);
    if(distance>MAX_DISTANCE){rejected.distant++;return false;}
    // A farther resident never takes a voice from a nearer one.
    if(voices.size>=maxVoices){
      let farthest=null;
      for(const v of voices)if(!farthest||v.distance>farthest.distance)farthest=v;
      if(!farthest||distance>=farthest.distance){rejected.pool++;return false;}
      farthest.release();
    }
    const now=context.currentTime;
    if(id!=null){const last=recent.get(id);if(last!=null&&now-last<RETRIGGER){rejected.retrigger++;return false;}recent.set(id,now);}
    const height=Math.abs(y-listener.y);
    // The distance falls gently: a resident a few metres away has to be audible,
    // one at the far edge of the district only murmurs.
    const level=spec.gain*LAYER*(mod?.scale??1)/(1+distance*.12)/(1+height*.0045)*(inside?.5:1);
    if(level<.002){rejected.quiet++;return false;}
    // The pan follows the picture, so the direction stays true from any view.
    const cx=x-listener.x,cz=z-listener.z;
    const pan=Math.max(-1,Math.min(1,(cx*-listener.fz+cz*listener.fx)/Math.max(1,Math.hypot(cx,cz))));
    const bus=context.createGain(),panner=context.createStereoPanner(),send=context.createGain();
    bus.gain.value=level;panner.pan.value=pan;send.gain.value=inside?.14:.26;
    // Distance dulls the sound, so a far resident is a murmur and not a click.
    const tone=context.createBiquadFilter();tone.type='lowpass';tone.Q.value=.6;
    tone.frequency.value=Math.max(700,Math.min(11000,11000-distance*130));
    tone.connect(bus);bus.connect(panner).connect(destination);panner.connect(send).connect(ambience);
    const nodes=[tone];
    const entry={distance,release(){if(!voices.delete(entry))return;for(const n of nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}for(const n of[bus,panner,send]){try{n.disconnect();}catch(_){}}}};
    voices.add(entry);
    let latest=null,latestEnd=0;
    const toeGap=surface?(PAIR.toeGap[0]+(PAIR.toeGap[1]-PAIR.toeGap[0])*toeRandom())*(mod?.length??1):0;
    for(const layer of surface?PAIRS[surface]:spec.layers) {
      const at=now+(layer.toe?toeGap:layer.delay||0)+(mod?.delay||0),dur=Math.max(layer.dur*(mod?.length??1),(layer.attack||.006)+.02);
      const levelNode=context.createGain();
      levelNode.gain.setValueAtTime(0,at);
      levelNode.gain.linearRampToValueAtTime(layer.lvl,at+(layer.attack||.006));
      // Footfall layers fall 20 dB over `dur` (the pair reads as one step), then die away over half as long again.
      if(surface){levelNode.gain.exponentialRampToValueAtTime(layer.lvl*.1,at+dur);levelNode.gain.exponentialRampToValueAtTime(.0001,at+dur*1.5);}
      else levelNode.gain.exponentialRampToValueAtTime(.0001,at+dur);
      levelNode.connect(tone);
      let source;
      if(layer.wave==='noise') {
        source=context.createBufferSource();source.buffer=noise;source.loop=true;
        const band=context.createBiquadFilter();band.type='bandpass';band.frequency.value=layer.f*(mod?.pitch??1);band.Q.value=layer.q||1;
        source.connect(band).connect(levelNode);nodes.push(band);
      } else {
        source=context.createOscillator();source.type=layer.type||'sine';
        source.frequency.setValueAtTime(layer.f*(mod?.pitch??1),at);
        if(layer.f2)source.frequency.exponentialRampToValueAtTime(Math.max(20,layer.f2*(mod?.pitch??1)),at+dur);
        source.connect(levelNode);
      }
      nodes.push(source,levelNode);
      const end=at+dur*(surface?1.5:1)+.02;if(end>latestEnd){latestEnd=end;latest=source;}
      source.start(at);source.stop(end);
    }
    // Only the longest layer ends the voice, so a two-note greeting is not cut.
    if(latest)latest.onended=entry.release;
    played++;if(surface)bySurface[surface]++;
    if(level>peak)peak=level;
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
    else voice('step',x,y,z,id,surfaceAt(x,y,z),carrying?CARRY:null);
    steps++;
  }

  function clear(){
    for(const v of [...voices])v.release();
    for(const [,g]of gaits){g.x=listener.x;g.z=listener.z;g.phase=0;}
    recent.clear();
  }

  // The companion trots (phase 5): when a diagonal pair lands (phase passes a multiple of pi), its front and hind foot
  // sound 25 ms apart. A hidden or resumed companion resyncs silently.
  function companion(p){
    if(!active||!p||!p.on){companionBeat=null;return;}
    const beat=Math.floor(p.phase/Math.PI);
    if(companionBeat===null||beat<companionBeat||beat-companionBeat>2){companionBeat=beat;return;}
    if(beat===companionBeat)return;
    companionBeat=beat;companionSteps++;const surface=surfaceAt(p.x,0,p.z);
    voice('step',p.x,0,p.z,'companion-front',surface,FRONT);voice('step',p.x,0,p.z,'companion-hind',surface,HIND);
  }

  return {
    setActive(value){if(active===value)return;active=value;if(!value)clear();},
    environment(value){inside=!!value;},
    listener(x,y,z,fx,fz,ox,oz){Object.assign(listener,{x,y,z,fx,fz});origin.x=ox??x;origin.z=oz??z;},
    action(kind,x,y,z,id){return voice(kind,x,y,z,id);},
    gait,
    step(x,y,z,id,mod){return voice('step',x,y,z,id,surfaceAt(x,y,z),mod);},companion,setTier(value){maxVoices=value==='low'?6:10;},
    reset(){gaits.clear();recent.clear();},
    stats:()=>({voices:voices.size,played,steps,companionSteps,bySurface:{...bySurface},gaits:gaits.size,inside,peak:+peak.toFixed(4),rejected:{...rejected}}),
    dispose(){active=false;clear();},
  };
}
