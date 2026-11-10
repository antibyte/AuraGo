// Room tones (System World beauty pass, phase 5): one synthesised bed per kind of room plus sparse room events on the
// audio clock. Only the tone of the room the listener is in, or of the hall whose open door it stands at, is alive;
// a switch fades over FADE seconds and releases the old tone when it has ended. Low tier: the first bed of a tone, no
// events. Levels are first estimates, tuned by the sound survey (allowed range x0.5..x2 per tone).
export const FADE=.6,MAX_ROOM_EVENTS=6;
export const ROOM_TONES=Object.freeze({
  // Agent core: reactor hum that breathes (stronger while the agent is busy), soft interface tones.
  agent:{beds:[{wave:'sine',f:49,gain:.05,lfo:[.11,.3],busy:1.4},{wave:'sine',f:98,gain:.025,busy:1.4},{wave:'sine',f:147,gain:.012},{wave:'noise',filter:['lowpass',300,.5],gain:.04}],
    events:[{kind:'blip',every:[3,8],gain:.012,notes:[1318.5,1568,1760,2093],dur:.08,double:.3}]},
  // Archive: near silence, page-like ticks in pairs.
  archive:{beds:[{wave:'noise',filter:['lowpass',220,.5],gain:.02},{wave:'sine',f:60,gain:.004}],
    events:[{kind:'tick',every:[1.5,6],gain:.015,f:3500,q:1.2,dur:.015,count:[2,2],spacing:[.06,.12]}]},
  // Gate halls: mains hum, relay clicks, patch tones.
  gate:{beds:[{wave:'sine',f:60,gain:.012},{wave:'sine',f:120,gain:.008},{wave:'noise',filter:['bandpass',1000,.7],gain:.008}],
    events:[{kind:'tick',every:[1,4],gain:.02,f:4000,q:2,dur:.005,count:[2,4],spacing:[.04,.09]},{kind:'blip',every:[4,10],gain:.01,notes:[660,880,990],dur:.12,double:0}]},
  // Departure hall: belt rumble, hall murmur, split-flap boards, a chime before (wordless) announcements.
  departure:{beds:[{wave:'noise',filter:['lowpass',120,.7],gain:.05,lfo:[.8,.3]},{wave:'sine',f:31,gain:.02},{wave:'noise',filter:['bandpass',500,.4],gain:.015}],
    events:[{kind:'tick',every:[12,25],gain:.014,f:2200,q:1.5,dur:.006,count:[8,20],spacing:[.04,.04]},{kind:'chime',every:[40,70],gain:.02,notes:[1046.5,1318.5,1568],step:.35,murmur:2.2}]},
  // Machine hall: fans, blade pass, coolant flow, mains, an occasional clunk.
  machine:{beds:[{wave:'noise',filter:['bandpass',180,1],gain:.06},{wave:'sine',f:87,gain:.02},{wave:'sine',f:174,gain:.01},{wave:'noise',filter:['bandpass',900,.6],gain:.02,lfo:[.3,.5]},{wave:'sine',f:50,gain:.015}],
    events:[{kind:'clunk',every:[8,20],gain:.03,dur:.06}]},
  // Garden court (open to the sky; leaves come from the green zone): water trickle and drops.
  court:{beds:[{wave:'noise',filter:['bandpass',2400,2],gain:.012,lfo:[5.3,.4]},{wave:'noise',filter:['bandpass',1400,1.2],gain:.006,lfo:[7.1,.3]}],
    events:[{kind:'drop',every:[.7,2.5],gain:.01,from:[1200,1800],to:600,dur:.06}]},
  // Waterfront pavilions: quiet air handling.
  pavilion:{beds:[{wave:'noise',filter:['lowpass',400,.5],gain:.015},{wave:'sine',f:120,gain:.003}],events:[]},
});
export function createRoomTones(context,{room,send,noise,random=Math.random,tier='high'}){
  let level=tier,current=null,busy=false,fired=0,switches=0;const events=new Set(),fading=new Set(),span=([a,b])=>a+(b-a)*random();
  function build(name,now){
    const spec=ROOM_TONES[name],nodes=[],own=n=>(nodes.push(n),n),out=own(context.createGain()),wet=own(context.createGain());
    out.gain.setValueAtTime(0,now);out.gain.linearRampToValueAtTime(1,now+FADE);out.connect(room);wet.gain.value=.5;out.connect(wet).connect(send);
    const beds=[];
    for(const b of level==='low'?spec.beds.slice(0,1):spec.beds){
      let src;if(b.wave==='noise'){src=own(context.createBufferSource());src.buffer=noise;src.loop=true;src.playbackRate.value=.8+.4*random();}
      else{src=own(context.createOscillator());src.type=b.wave==='saw'?'sawtooth':'sine';src.frequency.value=b.f;}
      const g=own(context.createGain());g.gain.value=b.gain*(busy&&b.busy?b.busy:1);let head=src;
      if(b.filter){const f=own(context.createBiquadFilter());f.type=b.filter[0];f.frequency.value=b.filter[1];f.Q.value=b.filter[2];head.connect(f);head=f;}
      head.connect(g).connect(out);
      if(b.lfo){const o=own(context.createOscillator()),d=own(context.createGain());o.frequency.value=b.lfo[0];d.gain.value=b.gain*b.lfo[1];o.connect(d).connect(g.gain);o.start(now);}
      src.start(now);beds.push({g,spec:b});
    }
    const next=new Float64Array(level==='low'?0:spec.events.length);for(let i=0;i<next.length;i++)next[i]=now+span(spec.events[i].every);
    switches++;return {name,spec,out,nodes,beds,next};
  }
  function drop(tone,now){const g=tone.out.gain;g.cancelScheduledValues(now);g.setValueAtTime(g.value,now);g.linearRampToValueAtTime(0,now+FADE);fading.add(tone);
    let last=null;for(const n of tone.nodes)if(n.stop){try{n.stop(now+FADE+.05);}catch(_){}last=n;}
    const finish=()=>{if(!fading.delete(tone))return;for(const n of tone.nodes)try{n.disconnect();}catch(_){}};if(last)last.onended=finish;else finish();}
  function voice(list){const entry={release(){if(!events.delete(entry))return;for(const n of list){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}}};events.add(entry);return entry;}
  function tone(f,at,dur,gain,out,list,to=0){const o=context.createOscillator(),g=context.createGain();o.type='sine';o.frequency.setValueAtTime(f,at);if(to)o.frequency.exponentialRampToValueAtTime(to,at+dur);
    g.gain.setValueAtTime(0,at);g.gain.linearRampToValueAtTime(gain,at+.005);g.gain.exponentialRampToValueAtTime(.0001,at+dur+.05);o.connect(g).connect(out);o.start(at);o.stop(at+dur+.08);list.push(o,g);return o;}
  function burst(f,q,at,dur,gain,out,list,type='bandpass'){const s=context.createBufferSource(),b=context.createBiquadFilter(),g=context.createGain();s.buffer=noise;s.loop=true;b.type=type;b.frequency.value=f;b.Q.value=q;
    g.gain.setValueAtTime(0,at);g.gain.linearRampToValueAtTime(gain,at+.001);g.gain.exponentialRampToValueAtTime(.0001,at+dur);s.connect(b).connect(g).connect(out);s.start(at);s.stop(at+dur+.02);list.push(s,b,g);return s;}
  function fire(e,at,out){
    const list=[];let end=null;fired++;
    if(e.kind==='blip'){end=tone(e.notes[Math.floor(random()*e.notes.length)],at,e.dur,e.gain,out,list);if(random()<e.double)end=tone(e.notes[Math.floor(random()*e.notes.length)],at+e.dur+.04,e.dur,e.gain*.8,out,list);}
    else if(e.kind==='tick'){const n=e.count[0]+Math.floor(random()*(e.count[1]-e.count[0]+1));let t=at;for(let i=0;i<n;i++){end=burst(e.f,e.q,t,e.dur,e.gain,out,list);t+=span(e.spacing);}}
    else if(e.kind==='chime'){e.notes.forEach((f,i)=>{end=tone(f,at+i*e.step,.9,e.gain,out,list);});
      if(e.murmur){const t=at+e.notes.length*e.step+.2,s=context.createBufferSource(),b=context.createBiquadFilter(),g=context.createGain(),am=context.createOscillator(),depth=context.createGain();
        s.buffer=noise;s.loop=true;b.type='bandpass';b.frequency.value=700;b.Q.value=4;am.frequency.value=4;depth.gain.value=e.gain*.2;
        g.gain.setValueAtTime(0,t);g.gain.linearRampToValueAtTime(e.gain*.4,t+.3);g.gain.setValueAtTime(e.gain*.4,t+e.murmur-.3);g.gain.linearRampToValueAtTime(0,t+e.murmur);
        am.connect(depth).connect(g.gain);s.connect(b).connect(g).connect(out);s.start(t);am.start(t);s.stop(t+e.murmur+.05);am.stop(t+e.murmur+.05);list.push(s,b,g,am,depth);end=s;}}
    else if(e.kind==='clunk')end=burst(300,.7,at,e.dur,e.gain,out,list,'lowpass');
    else if(e.kind==='drop')end=tone(e.from[0]+(e.from[1]-e.from[0])*random(),at,e.dur,e.gain,out,list,e.to);
    if(end)end.onended=voice(list).release;
  }
  return {
    // w: tracker weights (tone inside, leakTone at an open door); now: context seconds.
    update(w,now){
      const want=w.tone||w.leakTone||null;
      if((current?current.name:null)!==want){if(current){drop(current,now);current=null;}if(want&&ROOM_TONES[want])current=build(want,now);}
      if(!current)return;
      for(let i=0;i<current.next.length;i++)if(now>=current.next[i]){const e=current.spec.events[i];current.next[i]=now+span(e.every);if(events.size<MAX_ROOM_EVENTS)fire(e,now,current.out);}
    },
    setMood(m){const b=!!m?.busy;if(b===busy)return;busy=b;if(current)for(const bed of current.beds)if(bed.spec.busy)bed.g.gain.setTargetAtTime(bed.spec.gain*(busy?bed.spec.busy:1),context.currentTime,1);},
    setTier(value){if(value===level||!['low','medium','high','ultra'].includes(value))return;const change=(value==='low')!==(level==='low');level=value;
      if(change&&current){const name=current.name;drop(current,context.currentTime);current=build(name,context.currentTime);}},
    clear(){for(const e of[...events])e.release();},
    stats:()=>({tier:level,tone:current?current.name:null,events:events.size,fired,switches,fading:fading.size,nodes:(current?current.nodes.length:0)+[...fading].reduce((n,t)=>n+t.nodes.length,0)}),
    dispose(){for(const e of[...events])e.release();for(const t of[current,...fading])if(t)for(const n of t.nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}current=null;fading.clear();},
  };
}
