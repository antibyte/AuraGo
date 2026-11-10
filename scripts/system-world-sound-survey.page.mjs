// Sound survey page (System World beauty pass, phase 5), bundled by scripts/system-world-sound-survey.mjs. Each
// situation renders the real soundscape through an OfflineAudioContext. The city's inputs (listener, residents, trams,
// door, wind, clocks) are stepped STEP times per second with suspend()/resume(); the engine's timers run on the audio
// clock; randomness is seeded, so a render repeats exactly. The only requests go to this page's own server.
import * as THREE from 'three';
import {createCityAmbience,LIMITER,createLimiter} from '../ui/js/desktop/apps/sysworld-audio.js';
import {windAt} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {groundHeight} from '../ui/js/desktop/apps/sysworld-exploration.js';
import {streetCurve} from '../ui/js/desktop/apps/sysworld-navigation.js';
import {tramWaypoints} from '../ui/js/desktop/apps/sysworld-layout.js';
const RATE=48000,STEP=25,EYE=2.4,STRIDE=1.8;
const post=(url,body,type)=>fetch(url,{method:'POST',headers:{'Content-Type':type||'text/plain'},body});
const errors=[],log=t=>post('/__sound/log',String(t)).catch(()=>{});
addEventListener('error',e=>errors.push({where:'window',message:String(e.message)}));
addEventListener('unhandledrejection',e=>errors.push({where:'promise',message:String(e.reason&&e.reason.stack||e.reason)}));
const seeded=seed=>{let s=(seed>>>0)||1;return()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;};
function audioTimers(){const tasks=new Map();let id=0,now=0;
  return {setTimeout(fn,ms){tasks.set(++id,{fn,due:now+Math.max(0,Number(ms)||0)/1000});return id;},clearTimeout(h){tasks.delete(h);},
    run(t){now=t;for(let guard=0;guard<128;guard++){let key=0,best=null;for(const [k,v]of tasks)if(v.due<=t&&(!best||v.due<best.due)){key=k;best=v;}if(!best)return;tasks.delete(key);best.fn();}}};}
// bits 32 = IEEE float, 24 = integer PCM (config.pcm24).
function wav(buffer,bits=32){const ch=buffer.numberOfChannels,len=buffer.length,step=bits/8,bytes=44+len*ch*step,v=new DataView(new ArrayBuffer(bytes)),w=(o,s)=>{for(let i=0;i<4;i++)v.setUint8(o+i,s.charCodeAt(i));};
  w(0,'RIFF');v.setUint32(4,bytes-8,true);w(8,'WAVE');w(12,'fmt ');v.setUint32(16,16,true);v.setUint16(20,bits===32?3:1,true);v.setUint16(22,ch,true);v.setUint32(24,buffer.sampleRate,true);
  v.setUint32(28,buffer.sampleRate*ch*step,true);v.setUint16(32,ch*step,true);v.setUint16(34,bits,true);w(36,'data');v.setUint32(40,len*ch*step,true);
  const data=Array.from({length:ch},(_,c)=>buffer.getChannelData(c));let o=44;
  if(bits===32){for(let i=0;i<len;i++)for(let c=0;c<ch;c++){v.setFloat32(o,data[c][i],true);o+=4;}}
  else for(let i=0;i<len;i++)for(let c=0;c<ch;c++){const n=Math.max(-8388608,Math.min(8388607,Math.round(data[c][i]*8388608)));v.setUint8(o,n&255);v.setUint8(o+1,(n>>8)&255);v.setUint8(o+2,(n>>16)&255);o+=3;}
  return v.buffer;}
function along(points,s,out){let left=s;for(let i=1;i<points.length;i++){const [ax,az]=points[i-1],[bx,bz]=points[i],len=Math.hypot(bx-ax,bz-az);
  if(left<=len||i===points.length-1){const t=len?Math.min(1,left/len):0;out.x=ax+(bx-ax)*t;out.z=az+(bz-az)*t;if(len){out.fx=(bx-ax)/len;out.fz=(bz-az)/len;}return out;}left-=len;}return out;}
const polyLength=p=>p.slice(1).reduce((n,q,i)=>n+Math.hypot(q[0]-p[i][0],q[1]-p[i][1]),0);
async function render(s){
  const seconds=s.seconds||30,ctx=new OfflineAudioContext({numberOfChannels:2,length:Math.round(RATE*seconds),sampleRate:RATE}),timers=audioTimers(),m=s.mood||{},weather=m.weather||'clear';
  const sound=createCityAmbience(()=>{},{context:ctx,offline:true,timers,random:seeded(s.seed||1),voice:false,storage:null,enabled:true,tier:s.tier||'high'});
  sound.setTier?.(s.tier||'high');sound.setVolume(s.volume??.18);sound.setMood({day:m.day??1,evening:m.evening??0,busy:!!m.busy,weather});sound.setEnvironment(!!s.inside,weather);
  sound.unlock();sound.setActive(true);
  const L=s.listener||{},walk=s.walk||null,walkLength=walk?polyLength(walk):0,pose={x:walk?walk[0][0]:L.x??0,z:walk?walk[0][1]:L.z??0,fx:L.fx??0,fz:L.fz??-1};
  const route=streetCurve(tramWaypoints),routeLength=route.getLength(),point=new THREE.Vector3(),tangent=new THREE.Vector3();
  const trams=(s.trams||[]).slice(0,2).map(t=>({u:t.u||0,speed:t.speed??11})),walkers=s.walkers||[];
  const frame={time:0,seaTime:0,wind:{x:1,z:0,strength:0},trams:[{x:0,z:0,heading:0,on:false},{x:0,z:0,heading:0,on:false}],companion:{x:0,z:0,phase:0,on:false},doorValue:()=>s.door||0,reduced:false};
  let travelled=0,lastX=pose.x,lastZ=pose.z,steps=0;
  const track=[];
  function advance(t){
    timers.run(t);
    if(walk)along(walk,Math.min(walkLength,t*(s.walkSpeed||1.4)),pose);
    const y=L.y!=null&&!walk?L.y:groundHeight(pose.x,pose.z)+EYE,origin=s.origin||[pose.x,pose.z];
    travelled+=Math.hypot(pose.x-lastX,pose.z-lastZ);lastX=pose.x;lastZ=pose.z;
    if(walk&&travelled>=STRIDE){travelled-=STRIDE;sound.effect('step',pose.x,y,pose.z);steps++;}
    sound.setListener(pose.x,y,pose.z,pose.fx,pose.fz,origin[0],origin[1]);
    for(const w of walkers){const len=Math.hypot(w.to[0]-w.from[0],w.to[1]-w.from[1])||1,d=(t*(w.speed||1.2))%(2*len),k=(d<=len?d:2*len-d)/len,x=w.from[0]+(w.to[0]-w.from[0])*k,z=w.from[1]+(w.to[1]-w.from[1])*k;
      sound.gait(w.id,x,w.style==='glide'?1.6:groundHeight(x,z),z,true,!!w.carrying,w.style||'');}
    trams.forEach((tr,i)=>{const u=((tr.u+tr.speed*t/routeLength)%1+1)%1;route.getPointAt(u,point);route.getTangentAt(u,tangent);const f=frame.trams[i];f.x=point.x;f.z=point.z;f.heading=Math.atan2(tangent.x,tangent.z);f.on=true;
      if(i===0&&s.trace&&Math.abs(t-Math.round(t))<1e-6)track.push({t,u:+u.toFixed(4),x:+point.x.toFixed(1),z:+point.z.toFixed(1),dist:+Math.hypot(point.x-pose.x,point.z-pose.z).toFixed(1)});});
    frame.time=t;frame.seaTime=(s.seaTime||0)+t;windAt(t+(s.windOffset||0),weather,frame.wind);
    if(s.companion){const p=frame.companion,a=t/16*Math.PI*2;p.on=true;p.phase=t*1.7*Math.PI*2;p.x=30+4.5*Math.cos(a);p.z=69.5+2.5*Math.sin(a);}
    sound.frame?.(frame);
  }
  advance(0);
  const samples=[];
  for(let k=1;k<seconds*STEP;k++){const t=k/STEP;ctx.suspend(t).then(()=>{try{advance(t);if(k%(STEP*5)===0)samples.push({t,transient:sound.stats()?.soundscape?.transient??null});}catch(e){errors.push({where:s.name,message:String(e&&e.stack||e)});}return ctx.resume();});}
  const started=performance.now(),buffer=await ctx.startRendering(),renderMs=performance.now()-started,stats=sound.stats();sound.dispose();
  return {buffer,stats:{renderMs:Math.round(renderMs),realtime:+(seconds*1000/renderMs).toFixed(1),steps,sound:stats,transientSamples:samples,routeLength:+routeLength.toFixed(1),track}};
}
async function calibrate(){
  const out={trim:LIMITER.trim,ceiling:LIMITER.ceiling};
  for(const [label,amp]of[['quiet',Math.pow(10,-30/20)],['loud',2]]){
    const ctx=new OfflineAudioContext({numberOfChannels:1,length:RATE*2,sampleRate:RATE}),osc=ctx.createOscillator(),g=ctx.createGain(),lim=createLimiter(ctx);
    osc.frequency.value=1000;g.gain.value=amp;osc.connect(g).connect(lim.input);lim.output.connect(ctx.destination);osc.start();
    const d=(await ctx.startRendering()).getChannelData(0);let p=0,peak=0;for(let i=RATE;i<d.length;i++){p+=d[i]*d[i];peak=Math.max(peak,Math.abs(d[i]));}
    out[label]={gainDb:+(20*Math.log10(Math.sqrt(p/(d.length-RATE))/(amp/Math.SQRT2))).toFixed(3),peak:+peak.toFixed(4)};
  }
  out.makeupDb=+(out.quiet.gainDb-20*Math.log10(LIMITER.trim)).toFixed(3);out.suggestedTrim=+Math.pow(10,-out.makeupDb/20).toFixed(3);
  out.pass=Math.abs(out.quiet.gainDb)<=.25&&out.loud.peak<=LIMITER.ceiling+1e-3;return out;
}
async function main(){
  const config=await (await fetch('/__sound/config.json')).json();
  if(config.calibrate){try{await post('/__sound/calibration',JSON.stringify(await calibrate()),'application/json');}catch(e){errors.push({where:'calibrate',message:String(e&&e.stack||e)});}}
  for(const s of config.situations){try{log('rendering '+s.name+' ('+(s.seconds||30)+' s, '+(s.tier||'high')+')');const r=await render(s);
      const res=await post('/__sound/wav?name='+encodeURIComponent(s.name),wav(r.buffer,config.pcm24?24:32),'audio/wav');if(!res.ok)throw Error('upload failed '+res.status);
      await post('/__sound/stats?name='+encodeURIComponent(s.name),JSON.stringify(r.stats),'application/json');}
    catch(e){errors.push({where:s.name,message:String(e&&e.stack||e)});}}
}
main().catch(e=>errors.push({where:'main',message:String(e&&e.stack||e)})).finally(async()=>{
  try{await post('/__sound/report',JSON.stringify({errors}),'application/json');}catch(_){}await post('/__sound/done','done').catch(()=>{});});
