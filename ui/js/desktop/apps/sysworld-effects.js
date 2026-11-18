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
      const t=context.currentTime,att=1/(1+distance*.08)*(inside?.78:1);
      const tone=context.createBiquadFilter(),bus=context.createGain(),pan=context.createStereoPanner(),wet=context.createGain();
      tone.type='lowpass';tone.frequency.value=inside?2600:Math.max(1800,9200-distance*70);
      bus.gain.value=att;pan.pan.value=Math.max(-1,Math.min(1,((x-position.x)*-position.fz+(z-position.z)*position.fx)/Math.max(distance,1)));
      wet.gain.value=inside?.18:.28;
      tone.connect(bus).connect(pan).connect(destination);pan.connect(wet).connect(ambience);
      const nodes=[tone,bus,pan,wet],srcs=[];let endAt=0,lastSrc=null;
      const own=n=>(nodes.push(n),n),reg=(s,stopAt)=>{srcs.push(s);nodes.push(s);if(stopAt>endAt){endAt=stopAt;lastSrc=s;}s.start(t);s.stop(stopAt);};
      const env=(peak,at,attack,decay)=>{const g=own(context.createGain());g.gain.setValueAtTime(0,at);g.gain.linearRampToValueAtTime(peak,at+attack);g.gain.exponentialRampToValueAtTime(.0001,at+decay);g.connect(tone);return g;};
      const strike=(base,ratios,peak,at,decay)=>{
        ratios.forEach((r,i)=>{const o=context.createOscillator(),g=env(peak/((i+1)*.9),at,.004,decay/(1+i*.28));o.type='sine';o.frequency.value=base*r;o.connect(g);srcs.push(o);nodes.push(o);const st=at+decay+.03;if(st>endAt){endAt=st;lastSrc=o;}o.start(at);o.stop(st);});
      };
      const hiss=(f0,f1,q,peak,at,attack,decay)=>{
        const s=context.createBufferSource(),b=own(context.createBiquadFilter()),g=env(peak,at,attack,decay);
        s.buffer=noise;s.loop=true;b.type='bandpass';b.Q.value=q;b.frequency.setValueAtTime(f0,at);
        if(f1&&f1!==f0)b.frequency.exponentialRampToValueAtTime(f1,at+decay*.75);
        s.connect(b).connect(g);srcs.push(s);nodes.push(s);const st=at+decay+.03;if(st>endAt){endAt=st;lastSrc=s;}s.start(at);s.stop(st);
      };
      if(kind==='tram'||kind==='tram_bell'){
        // Two-strike bronze streetcar bell; station arrival adds a soft pneumatic brake release.
        const f=kind==='tram'?987.77:1046.5,gap=kind==='tram'?.19:.17;
        strike(f,[1,2.01,2.76,3.84],.055,t,.52);strike(f,[1,2.01,2.76,3.84],.062,t+gap,.62);
        if(kind==='tram')hiss(2600,1200,1.3,.032,t+.06,.02,.46);
      }else if(kind==='door'){
        // Brushless door glide, rubber seal air slide and damped end-stop latch.
        const o=context.createOscillator(),lp=own(context.createBiquadFilter()),g=env(.038,t,.03,.64);
        o.type='triangle';o.frequency.setValueAtTime(165,t);o.frequency.linearRampToValueAtTime(215,t+.28);o.frequency.linearRampToValueAtTime(148,t+.62);
        lp.type='lowpass';lp.frequency.value=560;o.connect(lp).connect(g);reg(o,t+.67);
        hiss(520,1080,1.2,.042,t,.025,.62);hiss(1750,1100,2.2,.028,t+.54,.003,.045);
      }else if(kind==='lift'){
        // Warm traction drive sweep with a soft two-note arrival chime.
        for(const [f0,f1,pk]of[[98,147,.036],[196,294,.016]]){const o=context.createOscillator(),g=env(pk,t,.08,1.38);o.type='sine';o.frequency.setValueAtTime(f0,t);o.frequency.exponentialRampToValueAtTime(f1,t+1.1);o.connect(g);reg(o,t+1.42);}
        strike(659.25,[1,2],.026,t+.12,.42);strike(880,[1,2],.028,t+.32,.55);
      }else if(kind==='discover'){
        // Warm harmonic discovery triad.
        [523.25,783.99,1174.66].forEach((f,i)=>strike(f,[1,2.01,3.02],.036,t+i*.055,.72));
      }else if(kind==='hover'){
        // Twin ducted-fan spin sweep and air-cushion downdraft wash.
        for(const [f0,f1,pk]of[[138,295,.036],[208,442,.02]]){const o=context.createOscillator(),g=env(pk,t,.05,1.05);o.type='sine';o.frequency.setValueAtTime(f0,t);o.frequency.exponentialRampToValueAtTime(f1,t+.65);o.frequency.exponentialRampToValueAtTime(f0*1.3,t+1.05);o.connect(g);reg(o,t+1.1);}
        hiss(340,880,.9,.048,t,.04,1.08);
      }else if(kind==='step'||kind==='step_inside'){
        hiss(kind==='step_inside'?620:280,kind==='step_inside'?420:190,1.1,kind==='step_inside'?.11:.14,t,.005,.12);
      }else{
        for(const n of nodes)try{n.disconnect();}catch(_){}return;
      }
      const entry={source:{stop(){for(const s of srcs)try{s.stop();}catch(_){}}},release(){if(!transient.delete(entry))return;for(const s of srcs)try{s.stop();}catch(_){}for(const n of nodes)try{n.disconnect();}catch(_){}}};
      transient.add(entry);if(lastSrc)lastSrc.onended=entry.release;
    },
    stats:()=>({voices:transient.size,weather,inside}),
    dispose(){active=false;clear();air.stop();for(const s of spaces){s.source.stop();for(const n of[s.source,s.level,s.pan,s.low])n.disconnect();}for(const n of[air,filter,gain])n.disconnect();},
  };
}
