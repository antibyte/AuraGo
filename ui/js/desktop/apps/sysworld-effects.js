// Original procedural Foley: bounded, gesture-unlocked and local Web Audio only.
export function createCityEffects(context,destination,ambience=destination){
  const transient=new Set(),position={x:0,z:0,fx:0,fz:-1};let active=false,inside=false,weather='clear';
  const noise=context.createBuffer(1,context.sampleRate*2,context.sampleRate);let seed=31;
  const samples=noise.getChannelData(0);for(let i=0;i<samples.length;i++){seed=Math.imul(seed,1664525)+1013904223|0;samples[i]=(seed>>>0)/2147483648-1;}
  const air=context.createBufferSource(),filter=context.createBiquadFilter(),gain=context.createGain();
  air.buffer=noise;air.loop=true;filter.type='lowpass';filter.frequency.value=450;gain.gain.value=0;
  air.connect(filter).connect(gain).connect(ambience);air.start();
  const spaces=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((s,i)=>{
    const source=i===3?context.createBufferSource():context.createOscillator(),level=context.createGain(),pan=context.createStereoPanner(),low=context.createBiquadFilter();
    if(i===3){source.buffer=noise;source.loop=true;}else{source.type='sine';source.frequency.value=s.f;}
    low.type='lowpass';low.frequency.value=s.f*2;level.gain.value=0;source.connect(low).connect(level).connect(pan).connect(ambience);source.start();return {...s,source,level,pan,low};
  });
  function sync(){gain.gain.setTargetAtTime(active?(weather==='rain'?.07:.015)*(inside?.12:1):0,context.currentTime,.1);filter.frequency.setTargetAtTime(inside?280:weather==='rain'?2100:600,context.currentTime,.2);}
  function clear(){for(const v of [...transient]){try{v.source.stop();}catch(_){}v.release();}}
  return {
    setActive(value){active=value;sync();if(!value){clear();for(const s of spaces)s.level.gain.setTargetAtTime(0,context.currentTime,.05);}},
    environment(value,rain){inside=value;if(rain)weather=rain;sync();},
    listener(x,y,z,fx,fz){Object.assign(position,{x,z,fx,fz});for(const s of spaces){const d=Math.hypot(x-s.x,z-s.z);s.level.gain.setTargetAtTime(active?s.g/(1+d*d*.025)*(inside?.25:1):0,context.currentTime,.1);s.pan.pan.value=Math.max(-1,Math.min(1,((s.x-x)*-fz+(s.z-z)*fx)/Math.max(1,d)));}},
    play(kind,x=0,y=0,z=0){
      if(!active||transient.size>=16)return;
      const distance=Math.hypot(x-position.x,z-position.z);if(distance>90)return;
      const spec={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08]}[kind];if(!spec)return;
      const [duration,freq,level]=spec,source=kind.startsWith('step')||kind==='door'?context.createBufferSource():context.createOscillator();
      if('buffer'in source)source.buffer=noise;else{source.type='sine';source.frequency.value=freq;}
      const low=context.createBiquadFilter(),volume=context.createGain(),pan=context.createStereoPanner();
      low.type='lowpass';low.frequency.value=inside?freq*.7:freq;
      volume.gain.setValueAtTime(0,context.currentTime);volume.gain.linearRampToValueAtTime(level/(1+distance*.08),context.currentTime+.015);volume.gain.exponentialRampToValueAtTime(.0001,context.currentTime+duration);
      pan.pan.value=Math.max(-1,Math.min(1,((x-position.x)*-position.fz+(z-position.z)*position.fx)/Math.max(distance,1)));
      source.connect(low).connect(volume).connect(pan).connect(destination);
      const entry={source,release(){if(!transient.delete(entry))return;for(const n of[source,low,volume,pan])n.disconnect();}};
      transient.add(entry);source.onended=entry.release;source.start();source.stop(context.currentTime+duration+.02);
    },
    stats:()=>({voices:transient.size,weather,inside}),
    dispose(){active=false;clear();air.stop();for(const s of spaces){s.source.stop();for(const n of[s.source,s.level,s.pan,s.low])n.disconnect();}for(const n of[air,filter,gain])n.disconnect();},
  };
}
