// Zone-shaped beds (System World beauty pass, phase 5): wind in the leaves (the atmosphere's wind), lapping at the quay
// (the sea shader's own wave), steam hiss at the foundry vents and drains, a traffic wash in the street canyons, rain
// on stone, water and leaves, and rain patter on the glass indoors. One shared noise buffer plus a generated patter
// loop; the graph is fixed per tier and update() allocates only for a lap. Low tier: traffic, rain and laps. Levels
// are first estimates, tuned by the sound survey within the ranges of plans/2026-11-10-system-world-beauty-phase-5.md.
import {seaHeight} from './sysworld-atmosphere.js';
import {steamVents} from './sysworld-particles.js';
import {bearingPan} from './sysworld-zones.js';

export const BEDS=Object.freeze({leaves:.1,laps:.16,steam:.06,traffic:.05,rain:.04,patter:.05});
export const LAP=Object.freeze({rise:.15,gap:.9,max:3,low:380,high:700,shortest:.22,longest:.4,quayZ:80});
const RICH=Object.freeze({low:false,medium:true,high:true,ultra:true}),VENTS=steamVents();
const clamp01=v=>v<0?0:v>1?1:v;
export function leafLevel(green,strength){const s=clamp01(strength);return BEDS.leaves*clamp01(green)*(.12+.88*s*s);}
export function trafficLevel(busy,w,rain){return BEDS.traffic*(busy?1.6:1)*(.2+.8*w.canyon+.3*w.plaza)*(1-.6*w.height)*(1-.5*w.harbour)*(rain?1.25:1);}
export function rainLevel(rain,w){return rain?BEDS.rain*(.5+.5*w.harbour+.4*w.green+.3*w.canyon)*(1-.7*w.height):0;}
export function patterLevel(rain,w){return rain&&!w.open?BEDS.patter*clamp01(w.interior):0;}
// Each vent adds size/(1 + (d/6)^2); the loudest vent sets the direction.
export function steamLevel(x,y,z,out={}){let sum=0,best=0;out.x=x;out.z=z;
  for(let i=0;i<VENTS.length;i++){const v=VENTS[i],d=Math.hypot(v.x-x,v.y-y,v.z-z),l=v.size/(1+(d/6)*(d/6));sum+=l;if(l>best){best=l;out.x=v.x;out.z=v.z;}}
  out.level=BEDS.steam*Math.min(1,sum);return out;}
export function lapCrossing(previous,height){return previous<LAP.rise&&height>=LAP.rise;}
// Two seconds of sparse decaying clicks (about 40 per second): rain on glass once band-passed.
export function patterBuffer(context,random){
  const n=Math.floor(context.sampleRate*2),b=context.createBuffer(1,n,context.sampleRate),d=b.getChannelData(0),len=Math.max(4,Math.floor(context.sampleRate*.004));
  for(let t=-Math.log(1-random()*.999)/40;;t+=-Math.log(1-random()*.999)/40){const i=Math.floor(t*context.sampleRate);if(i>=n-len)break;
    const a=.3+.7*random();for(let k=0;k<len;k++)d[i+k]+=a*(random()*2-1)*Math.exp(-4*k/len);}
  return b;
}
export function createSoundBeds(context,{outdoor,send,local,noise,random=Math.random,tier='high'}){
  let level=RICH[tier]===undefined?'high':tier,busy=false,rain=false,lastLap=-Infinity,previousSea=0,played=0,parts=null;
  const laps=new Set(),st={level:0,x:0,z:0},last=new Float64Array(8);
  function build(){
    const nodes=[],own=n=>(nodes.push(n),n),amp=v=>{const g=own(context.createGain());g.gain.value=v;return g;};
    const filter=(type,f,q=.7)=>{const n=own(context.createBiquadFilter());n.type=type;n.frequency.value=f;n.Q.value=q;return n;};
    const loop=(rate,buffer=noise)=>{const s=own(context.createBufferSource());s.buffer=buffer;s.loop=true;s.playbackRate.value=rate;s.start();return s;};
    const lfo=(rate,depth,target)=>{const o=own(context.createOscillator()),d=amp(depth);o.frequency.value=rate;o.connect(d).connect(target);o.start();};
    const p={nodes};
    p.traffic=amp(0);loop(.81).connect(filter('lowpass',380)).connect(amp(.7)).connect(p.traffic);
    p.hiss=filter('bandpass',900,.8);loop(.77).connect(p.hiss).connect(amp(.3)).connect(p.traffic);
    p.traffic.connect(outdoor);p.traffic.connect(amp(.3)).connect(send);
    p.rain=amp(0);loop(1.13).connect(filter('highpass',1500)).connect(filter('lowpass',7000)).connect(p.rain).connect(outdoor);
    if(RICH[level]){
      p.leaves=amp(0);p.leafBand=filter('bandpass',3200,.7);const am=amp(1);
      loop(.93).connect(p.leafBand).connect(am).connect(p.leaves).connect(outdoor);lfo(.7,.25,am.gain);lfo(1.3,.2,am.gain);p.leaves.connect(amp(.25)).connect(send);
      p.steam=amp(0);p.steamPan=own(context.createStereoPanner());
      loop(1.07).connect(filter('highpass',2500)).connect(filter('bandpass',5000,.5)).connect(p.steam).connect(p.steamPan).connect(outdoor);
      p.patter=amp(0);loop(1,patterBuffer(context,random)).connect(filter('bandpass',2200,.9)).connect(p.patter).connect(local);
    }
    last.fill(NaN);return p;
  }
  function teardown(){if(!parts)return;for(const n of parts.nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}parts=null;}
  function set(slot,param,value,tau=.3,eps=2e-4){const prev=last[slot];if(Math.abs(value-prev)<=Math.max(eps,Math.abs(prev)*.03))return;last[slot]=value;param.setTargetAtTime(value,context.currentTime,tau);}
  function lap(at,quay){
    lastLap=at;played++;
    const src=context.createBufferSource(),band=context.createBiquadFilter(),env=context.createGain(),pan=context.createStereoPanner(),wet=context.createGain();
    const d=LAP.shortest+(LAP.longest-LAP.shortest)*random();
    src.buffer=noise;src.loop=true;band.type='bandpass';band.frequency.value=LAP.low+(LAP.high-LAP.low)*random();band.Q.value=1.2;pan.pan.value=random()-.5;wet.gain.value=.4;
    env.gain.setValueAtTime(0,at);env.gain.linearRampToValueAtTime(BEDS.laps*quay*Math.sqrt(quay),at+.03);env.gain.exponentialRampToValueAtTime(.0001,at+d);
    src.connect(band).connect(env).connect(pan).connect(outdoor);pan.connect(wet).connect(send);
    const entry={release(){if(!laps.delete(entry))return;for(const n of[src,band,env,pan,wet]){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}}};
    laps.add(entry);src.onended=entry.release;src.start(at);src.stop(at+d+.05);
  }
  parts=build();
  return {
    // f: the sound frame; w: tracker weights; listener: {x,y,z,fx,fz,origin:{x,z}}; now: context seconds.
    update(f,w,listener,now){
      if(!parts)return;
      set(0,parts.traffic.gain,trafficLevel(busy,w,rain));set(1,parts.hiss.frequency,rain?1800:900,.8,1);set(2,parts.rain.gain,rainLevel(rain,w),.6);
      if(parts.leaves){const s=f&&f.wind?f.wind.strength:0;set(3,parts.leaves.gain,leafLevel(w.green,s),.25);set(4,parts.leafBand.frequency,2400+1600*clamp01(s),.4,20);
        steamLevel(listener.x,listener.y,listener.z,st);set(5,parts.steam.gain,st.level,.4);
        set(6,parts.steamPan.pan,bearingPan(st.x-listener.x,st.z-listener.z,listener.fx,listener.fz),.3,.02);set(7,parts.patter.gain,patterLevel(rain,w),.5);}
      const qx=Math.max(-80,Math.min(80,listener.origin.x)),h=seaHeight(qx,LAP.quayZ,f&&f.time||0),quay=w.quay||0;
      if(lapCrossing(previousSea,h)&&now-lastLap>LAP.gap&&quay>.05&&laps.size<LAP.max)lap(now,quay);
      previousSea=h;
    },
    setMood(m){busy=!!m?.busy;rain=m?.weather==='rain';},
    setTier(value){if(RICH[value]===undefined||value===level)return;const change=RICH[value]!==RICH[level];level=value;if(change){teardown();parts=build();}},
    clear(){for(const e of[...laps])e.release();},
    stats:()=>({tier:level,rich:!!RICH[level],laps:laps.size,played,nodes:parts?parts.nodes.length:0,
      levels:{traffic:+(last[0]||0).toFixed(4),rain:+(last[2]||0).toFixed(4),leaves:+(last[3]||0).toFixed(4),steam:+(last[5]||0).toFixed(4),patter:+(last[7]||0).toFixed(4)}}),
    dispose(){for(const e of[...laps])e.release();teardown();},
  };
}
