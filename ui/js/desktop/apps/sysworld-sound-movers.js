// Moving sources (System World beauty pass, phase 5): the trams' rail hum with a squeal in curves and rail-joint clacks,
// a whir for the nearest hover robots (the gliding street patrols of sysworld-life.js, reported through the residents'
// gait calls), the harbour launch's engine and the barge's drone (craftPoses on the atmosphere clock), and positions for
// gull calls (gullPose). A voice is built when its source comes within reach and released MOVERS.release seconds
// after it leaves, so a quiet district costs nothing; update() allocates only at those edges and for a clack. Distance
// runs from the acoustic origin, the pan from the camera (the city's convention).
import {craftPoses} from './sysworld-harbour.js';
import {gullPose,FLOCK} from './sysworld-gulls.js';
import {bearingPan} from './sysworld-zones.js';

// tram .28: the synthesis' overall level, supporting the recorded pass (phase 6; .7 without it). pass: the recorded tram pass, which has its
// own gentler law (tramLaw().pass: level at the closest point x speed x sqrt of the distance gain, half the hum's dB range, because the
// recording carries its own approach and departure); level, the distance it starts at while approaching, the minimum speed, the distance
// after which it re-arms.
export const MOVERS=Object.freeze({tram:.28,pass:Object.freeze({level:.9,reach:70,speed:4,rearm:95}),squeal:.1,hover:.05,launch:.11,barge:.08,roll:1.6,whine:.09,motor:.15,clack:.8,tick:.25,tickGap:.3,joint:12,axles:2.2,maxClacks:6,release:1.25,
  rollBands:Object.freeze([Object.freeze([.55,1.0,.6]),Object.freeze([1.1,1.4,.45]),Object.freeze([2.05,1.8,.3])]),tramFall:Object.freeze([8,.7]),hoverFall:Object.freeze([4,.45]),
  reach:Object.freeze({tram:120,hover:45,launch:120,barge:260,clack:60})});
export const VOICES=Object.freeze({low:Object.freeze({trams:1,hovers:2,barge:false}),medium:Object.freeze({trams:2,hovers:4,barge:true}),
  high:Object.freeze({trams:2,hovers:4,barge:true}),ultra:Object.freeze({trams:2,hovers:4,barge:true})});
const clamp01=v=>v<0?0:v>1?1:v;
export function distanceGain(d,ref,rolloff){return d<=ref?1:ref/(ref+rolloff*(d-ref));}
export function dopplerRatio(previous,distance,dt){if(!(dt>0)||!Number.isFinite(previous))return 1;return Math.max(.94,Math.min(1.06,343/(343+(distance-previous)/dt)));}
export function tramLaw(speed,lateral,distance,out={}){const g=distance<MOVERS.reach.tram?distanceGain(distance,MOVERS.tramFall[0],MOVERS.tramFall[1]):0;
  out.hum=MOVERS.tram*Math.pow(clamp01(speed/11),.7)*g;out.squeal=MOVERS.squeal*clamp01((lateral-1)/3)*g;out.motor=60+32*clamp01(speed/11);out.whine=380+520*clamp01(speed/11);out.roll=600+40*Math.min(11,Math.max(0,speed));out.pass=MOVERS.pass.level*Math.pow(clamp01(speed/11),.7)*Math.sqrt(g);return out;}
// Indices of the `count` nearest items with `on` within `reach` of (x, z), nearest first; -1 pads. No allocation.
export function nearestSlots(items,count,x,z,reach,out){
  for(let k=0;k<count;k++)out[k]=-1;
  for(let i=0;i<items.length;i++){const it=items[i];if(!it.on)continue;const d=Math.hypot(it.x-x,it.z-z);if(d>reach)continue;
    for(let k=0;k<count;k++){const j=out[k];if(j<0||d<Math.hypot(items[j].x-x,items[j].z-z)){for(let m=count-1;m>k;m--)out[m]=out[m-1];out[k]=i;break;}}}
  return out;
}
export function createSoundMovers(context,{outdoor,send,noise,tier='high',random=Math.random,buffer=()=>null}){
  let level=VOICES[tier]?tier:'high',hoverClock=0,clacks=0,clacksPlayed=0,ticks=0,passes=0;
  const trams=[0,1].map(()=>({voice:null,x:0,z:0,heading:0,speed:0,turn:0,distance:NaN,travelled:0,idle:0,fresh:true,tick:0,armed:true,pass:null}));
  const gliders=[],gliderIds=new Map(),slots=Array.from({length:4},()=>({voice:null,index:-1,idle:0})),picks=new Int32Array(4);
  const launch={voice:null,idle:0},barge={voice:null,idle:0},poses=craftPoses(0),law={hum:0,squeal:0,motor:60},live=new Set();
  function kit(){const v={nodes:[],last:new Float64Array(9).fill(NaN)},own=n=>(v.nodes.push(n),n);
    v.amp=x=>{const g=own(context.createGain());g.gain.value=x;return g;};
    v.filter=(type,f,q)=>{const n=own(context.createBiquadFilter());n.type=type;n.frequency.value=f;n.Q.value=q;return n;};
    v.noise=rate=>{const s=own(context.createBufferSource());s.buffer=noise;s.loop=true;s.playbackRate.value=rate;return s;};
    v.osc=(type,f)=>{const o=own(context.createOscillator());o.type=type;o.frequency.value=f;return o;};
    v.out=v.amp(0);v.pan=own(context.createStereoPanner());v.out.connect(v.pan).connect(outdoor);return v;}
  function start(v){for(const n of v.nodes)if(n.start)n.start();live.add(v);return v;}
  function release(v){if(!v)return null;for(const n of v.nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}live.delete(v);return null;}
  function tramVoice(){const v=kit();v.squeal=v.amp(0);v.squeal.connect(v.pan);const wet=v.amp(.35);v.out.connect(wet).connect(send);
    v.passGain=v.amp(0);v.passGain.connect(v.pan);v.passGain.connect(wet);   // the recorded pass: its own gain, pan and the same reverb send, not v.out
    v.noise(.6).connect(v.filter('lowpass',160,.7)).connect(v.amp(.6)).connect(v.out);
    v.motor=v.osc('sawtooth',60);v.motor.connect(v.filter('lowpass',400,.7)).connect(v.amp(MOVERS.motor)).connect(v.out);
    // Noise first, tones second: the roar (roll) is the loudest part, the motor sawtooth and the whine sit well under it (motor .15, whine .09).
    // Electric whine (380 Hz at rest to 900 Hz at full speed) and wheel-on-rail roar: three detuned noise bands that follow the speed;
    // both pass v.out, so they share the hum's speed/distance law, pan and output.
    v.whine=v.osc('sine',380);v.whine.connect(v.amp(MOVERS.whine)).connect(v.out);
    v.rolls=MOVERS.rollBands.map(([k,q,g],i)=>{const b=v.filter('bandpass',600*k,q);v.noise(.9+.07*i).connect(v.filter('highpass',220,.7)).connect(b).connect(v.amp(MOVERS.roll*g)).connect(v.out);return b;});
    v.noise(1).connect(v.filter('bandpass',2900,14)).connect(v.squeal);v.osc('sine',3150).connect(v.amp(.3)).connect(v.squeal);return start(v);}
  function hoverVoice(k){const v=kit();v.out.connect(v.amp(.2)).connect(send);
    v.osc('sine',180+7*k).connect(v.out);v.osc('sine',271+9*k).connect(v.amp(.4)).connect(v.out);v.noise(1+.05*k).connect(v.filter('bandpass',1200,3)).connect(v.amp(.5)).connect(v.out);return start(v);}
  function launchVoice(){const v=kit(),am=v.amp(.6);v.out.connect(v.amp(.3)).connect(send);
    v.osc('sawtooth',46).connect(v.filter('lowpass',260,.7)).connect(am).connect(v.out);v.osc('sine',7.5).connect(v.amp(.4)).connect(am.gain);return start(v);}
  function bargeVoice(){const v=kit(),lp=v.filter('lowpass',120,.7);lp.connect(v.out);v.osc('sine',33).connect(lp);v.osc('sine',66).connect(v.amp(.5)).connect(lp);return start(v);}
  function set(v,slot,param,value,tau,eps=2e-4){const prev=v.last[slot];if(Math.abs(value-prev)<=Math.max(eps,Math.abs(prev)*.004))return;v.last[slot]=value;param.setTargetAtTime(value,context.currentTime,tau);}
  function clack(at,gain,pan,f=1800,q=2,decay=.012){if(clacks>=MOVERS.maxClacks)return;clacks++;clacksPlayed++;
    const src=context.createBufferSource(),band=context.createBiquadFilter(),env=context.createGain();src.buffer=noise;src.loop=true;band.type='bandpass';band.frequency.value=f;band.Q.value=q;
    env.gain.setValueAtTime(0,at);env.gain.linearRampToValueAtTime(gain*MOVERS.clack,at+.002);env.gain.exponentialRampToValueAtTime(.0001,at+decay);
    src.connect(band).connect(env).connect(pan);src.onended=()=>{clacks--;for(const n of[src,band,env])try{n.disconnect();}catch(_){}};src.start(at);src.stop(at+decay+.008);}
  // One recorded pass into the voice's passGain (its own law: tramLaw().pass; the voice's pan and reverb send, not the hum's gain); it dies with the voice.
  function startPass(t,v,b,now){const src=context.createBufferSource();src.buffer=b;src.connect(v.passGain);
    v.nodes.push(src);t.pass=src;t.armed=false;passes++;
    src.onended=()=>{try{src.disconnect();}catch(_){}const i=v.nodes.indexOf(src);if(i>=0)v.nodes.splice(i,1);if(t.pass===src)t.pass=null;};src.start(now);}
  function dropTram(t){t.voice=release(t.voice);t.idle=0;t.armed=true;t.pass=null;}
  function moveTram(t,src,listener,now,dt){
    const ox=listener.origin.x,oz=listener.origin.z;
    if(src&&src.on){
      const step=Math.hypot(src.x-t.x,src.z-t.z);
      if(t.fresh||step>20){t.x=src.x;t.z=src.z;t.heading=src.heading;t.speed=0;t.turn=0;t.fresh=false;t.distance=NaN;}
      else if(dt>0){const k=1-Math.exp(-dt/.25),dh=Math.atan2(Math.sin(src.heading-t.heading),Math.cos(src.heading-t.heading));
        t.speed+=(step/dt-t.speed)*k;t.turn+=(dh/dt-t.turn)*k;t.travelled+=step;t.x=src.x;t.z=src.z;t.heading=src.heading;}
      const d=Math.hypot(t.x-ox,t.z-oz);
      if(!t.voice&&d<MOVERS.reach.tram)t.voice=tramVoice();
      if(t.voice){const v=t.voice,r=dopplerRatio(t.distance,d,dt);tramLaw(t.speed,Math.abs(t.turn)*t.speed,d,law);
        set(v,0,v.out.gain,law.hum,.15);set(v,8,v.passGain.gain,law.pass,.15);set(v,1,v.squeal.gain,law.squeal,.1);set(v,2,v.motor.frequency,law.motor*r,.2,.05);set(v,3,v.whine.frequency,law.whine*r,.2,.05);for(let i=0;i<v.rolls.length;i++)set(v,5+i,v.rolls[i].frequency,law.roll*MOVERS.rollBands[i][0]*(1+.012*(i-1)),.2,1);
        set(v,4,v.pan.pan,bearingPan(t.x-listener.x,t.z-listener.z,listener.fx,listener.fz),.1,.02);
        while(t.travelled>=MOVERS.joint){t.travelled-=MOVERS.joint;if(d<MOVERS.reach.clack&&law.hum>0){clack(now,law.hum,v.pan,1300,1.6,.035);clack(now+MOVERS.axles/Math.max(1,t.speed),law.hum,v.pan,1300,1.6,.035);}}
        // Flange ticks: short bright noise bursts, more often at speed and in curves (deterministic through the injected random).
        if(d<MOVERS.reach.clack&&law.hum>0&&t.speed>1){t.tick-=dt*(t.speed/11)*(1+2*Math.min(1,Math.abs(t.turn)));if(t.tick<=0){ticks++;clack(now,law.hum*MOVERS.tick/MOVERS.clack,v.pan,4200,6,.006);t.tick=MOVERS.tickGap*(.4+1.2*random());}}
        if(d>MOVERS.pass.rearm)t.armed=true;
        else if(t.armed&&!t.pass&&d<MOVERS.pass.reach&&t.speed>MOVERS.pass.speed&&Number.isFinite(t.distance)&&d<t.distance){const b=buffer('tram');if(b)startPass(t,v,b,now);}
        t.idle=d>MOVERS.reach.tram*1.2?t.idle+dt:0;if(t.idle>MOVERS.release)dropTram(t);}
      else t.travelled%=MOVERS.joint;
      t.distance=d;
    }else{t.fresh=true;if(t.voice){set(t.voice,0,t.voice.out.gain,0,.15);set(t.voice,8,t.voice.passGain.gain,0,.15);set(t.voice,1,t.voice.squeal.gain,0,.1);t.idle+=dt;if(t.idle>MOVERS.release)dropTram(t);}}
  }
  function moveHovers(listener,dt){
    const n=VOICES[level].hovers,ox=listener.origin.x,oz=listener.origin.z;hoverClock+=dt;
    if(hoverClock>=.2||dt===0){hoverClock=0;nearestSlots(gliders,n,ox,oz,MOVERS.reach.hover,picks);
      for(let k=0;k<slots.length;k++){const s=slots[k],j=k<n?picks[k]:-1;if(j>=0){s.index=j;s.idle=0;if(!s.voice)s.voice=hoverVoice(k);}else s.index=-1;}}
    for(let k=0;k<slots.length;k++){const s=slots[k];if(!s.voice)continue;
      if(s.index<0){set(s.voice,0,s.voice.out.gain,0,.3);s.idle+=dt;if(s.idle>MOVERS.release){s.voice=release(s.voice);s.idle=0;}continue;}
      const g=gliders[s.index],d=Math.hypot(g.x-ox,g.z-oz);
      set(s.voice,0,s.voice.out.gain,MOVERS.hover*distanceGain(d,MOVERS.hoverFall[0],MOVERS.hoverFall[1])*(g.moving?1:.6),.15);set(s.voice,1,s.voice.pan.pan,bearingPan(g.x-listener.x,g.z-listener.z,listener.fx,listener.fz),.1,.02);}
  }
  function boat(b,pose,make,gain,ref,rolloff,reach,listener,dt,allowed){
    const d=Math.hypot(pose.x-listener.origin.x,pose.z-listener.origin.z);
    if(allowed&&!b.voice&&d<reach)b.voice=make();if(!b.voice)return;
    const on=allowed&&d<reach*1.2;set(b.voice,0,b.voice.out.gain,on?gain*distanceGain(d,ref,rolloff):0,.4);
    set(b.voice,1,b.voice.pan.pan,bearingPan(pose.x-listener.x,pose.z-listener.z,listener.fx,listener.fz),.3,.02);
    b.idle=on?0:b.idle+dt;if(b.idle>MOVERS.release){b.voice=release(b.voice);b.idle=0;}
  }
  return {
    gait(id,x,y,z,moving,style){if(style!=='glide')return;let g=gliderIds.get(id);if(!g){if(gliders.length>=32)return;g={id,x,y,z,on:true,moving:false};gliderIds.set(id,g);gliders.push(g);}
      g.x=x;g.y=y;g.z=z;g.moving=!!moving;},
    update(f,listener,now,dt){
      for(let i=0;i<trams.length;i++)moveTram(trams[i],i<VOICES[level].trams&&f&&f.trams?f.trams[i]:null,listener,now,dt);
      moveHovers(listener,dt);craftPoses(f&&f.seaTime||0,poses);
      boat(launch,poses[0],launchVoice,MOVERS.launch,10,.15,MOVERS.reach.launch,listener,dt,true);
      boat(barge,poses[1],bargeVoice,MOVERS.barge,20,.08,MOVERS.reach.barge,listener,dt,VOICES[level].barge);
    },
    // A position for a gull call: one bird of the visible flock (none on low: the caller keeps its harbour anchor).
    gull(f,random,out){const n=FLOCK[level]||0;if(!n)return null;const t=f&&f.seaTime||0;craftPoses(t,poses);return gullPose(Math.floor(random()*n)%n,t,poses[1],out);},
    setTier(value){if(VOICES[value])level=value;},
    clear(){},
    stats:()=>({tier:level,trams:trams.map(t=>({voice:!!t.voice,speed:+t.speed.toFixed(2),hum:t.voice?+(t.voice.last[0]||0).toFixed(4):0,squeal:t.voice?+(t.voice.last[1]||0).toFixed(4):0,
        distance:Number.isFinite(t.distance)?+t.distance.toFixed(1):null})),
      hovers:slots.filter(s=>s.voice&&s.index>=0).map(s=>gliders[s.index].id),gliders:gliders.length,launch:!!launch.voice,barge:!!barge.voice,clacks,clacksPlayed,ticks,passes,
      nodes:[...live].reduce((n,v)=>n+v.nodes.length,0)}),
    dispose(){for(const v of[...live])release(v);for(const t of trams)t.voice=null;for(const s of slots)s.voice=null;launch.voice=barge.voice=null;},
  };
}
