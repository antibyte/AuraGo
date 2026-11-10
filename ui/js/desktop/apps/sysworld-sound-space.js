// Acoustic spaces (System World beauty pass, phase 5): the glass between listener and city (a low-pass and a gain while
// a closed room is around the listener), the reverb sends of the three spaces and the room bus. The open-air
// convolver stays in sysworld-audio.js (`verb`, 3.4 s); this module adds the street canyon's early echoes and one
// interior convolver whose impulse follows the room (lobby, hall, machine, small). The interior convolver is replaced,
// never re-buffered, and only while its send is near silent. Low tier: no extra convolver (both sends feed `verb`);
// medium: interior only; high and ultra: both. Pure laws and impulses are exported for the test.
export const GLASS=Object.freeze({open:18000,shut:900,db:-12,door:.7});
export const RETURNS=Object.freeze({canyon:.45,interior:.5,low:.3,leak:.35});
export const ROOM_SPACES=Object.freeze({
  lobby:Object.freeze({seconds:1.6,decay:3.2,predelay:.012,tone:5200}),
  hall:Object.freeze({seconds:2.4,decay:2.8,predelay:.02,tone:4200}),
  machine:Object.freeze({seconds:1.1,decay:2.4,predelay:.006,tone:7000,ring:Object.freeze([310,620,1240])}),
  small:Object.freeze({seconds:.6,decay:3.6,predelay:.004,tone:3600}),
});
// Early echoes between facades: [seconds, gain, pan].
export const CANYON_TAPS=Object.freeze([[.019,.62,-.5],[.031,.5,.6],[.047,.42,-.25],[.064,.33,.4],[.088,.24,-.6],[.121,.16,.3],[.163,.1,-.2]]);
const TIERS=Object.freeze({low:{canyon:false,interior:false},medium:{canyon:false,interior:true},high:{canyon:true,interior:true},ultra:{canyon:true,interior:true}});
const clamp01=v=>v<0?0:v>1?1:v;
function lcg(seed){let s=seed>>>0;return()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;}
export function glassLaw(interior,open,door,out={}){const amount=open?0:clamp01(interior)*(1-GLASS.door*clamp01(door));
  out.amount=amount;out.cutoff=GLASS.open*Math.pow(GLASS.shut/GLASS.open,amount);out.gain=Math.pow(10,GLASS.db*amount/20);return out;}
export function sendLaw(w,out={}){const canyon=clamp01(w.canyon);out.canyon=canyon;out.open=1-canyon;out.interior=w.space?clamp01(w.interior):0;out.outdoor=1-out.interior;return out;}
export function roomLaw(interior,leak){const i=clamp01(interior);return i+(1-i)*clamp01(leak)*RETURNS.leak;}
export function impulseCanyon(sampleRate){
  const n=Math.ceil(sampleRate*.45),L=new Float32Array(n),R=new Float32Array(n),rnd=lcg(4243),burst=Math.max(4,Math.round(sampleRate*.0005));
  for(const [t,g,p]of CANYON_TAPS){const i=Math.round(t*sampleRate),gl=g*Math.sqrt((1-p)/2),gr=g*Math.sqrt((1+p)/2);
    for(let k=0;k<burst&&i+k<n;k++){const v=(rnd()*2-1)*Math.exp(-4*k/burst);L[i+k]+=v*gl;R[i+k]+=v*gr;}}
  for(let i=Math.round(.03*sampleRate);i<n;i++){const d=.05*Math.pow(1-i/n,4);L[i]+=(rnd()*2-1)*d;R[i]+=(rnd()*2-1)*d;}
  return [L,R];
}
// A room: predelay, low-passed noise with a power-law decay (and a metallic ring for the machine hall), normalised to
// the same energy for every space so a swap never jumps in level.
export function impulseRoom(name,sampleRate){
  const s=ROOM_SPACES[name];if(!s)throw Error('Unknown room space '+name);
  const n=Math.ceil(sampleRate*s.seconds),pre=Math.round(sampleRate*s.predelay),a=Math.exp(-2*Math.PI*Math.min(s.tone,sampleRate*.45)/sampleRate),out=[new Float32Array(n),new Float32Array(n)];
  let seed=977;for(let i=0;i<name.length;i++)seed=Math.imul(seed,31)+name.charCodeAt(i);const rnd=lcg(seed);
  for(let c=0;c<2;c++){const d=out[c];let lp=0,power=0;
    for(let i=pre;i<n;i++){const t=(i-pre)/(n-pre);lp=lp*a+(rnd()*2-1)*(1-a);let v=lp*Math.pow(1-t,s.decay);
      if(s.ring)for(let k=0;k<s.ring.length;k++)v+=.08*Math.sin(2*Math.PI*s.ring[k]*i/sampleRate+c)*Math.exp(-6*t);
      d[i]=v;power+=v*v;}
    const scale=.5/Math.sqrt(power||1);for(let i=0;i<n;i++)d[i]*=scale;}
  return out;
}
function buffer(context,[l,r]){const b=context.createBuffer(2,l.length,context.sampleRate);b.getChannelData(0).set(l);b.getChannelData(1).set(r);return b;}
export function createSoundSpace(context,{ambience,verb,tier='high'}){
  const nodes=new Set(),own=n=>(nodes.add(n),n),gain=value=>{const g=own(context.createGain());g.gain.value=value;return g;};
  const outdoor=gain(1),glassFilter=own(context.createBiquadFilter()),glassGain=gain(1);
  glassFilter.type='lowpass';glassFilter.frequency.value=GLASS.open;glassFilter.Q.value=.5;outdoor.connect(glassFilter).connect(glassGain).connect(ambience);
  const sendOutdoor=gain(1),openSplit=gain(1),canyonSplit=gain(0);sendOutdoor.connect(openSplit).connect(verb);sendOutdoor.connect(canyonSplit);
  const sendLocal=gain(1),interiorSplit=gain(0),outdoorSplit=gain(1);sendLocal.connect(interiorSplit);sendLocal.connect(outdoorSplit).connect(sendOutdoor);
  const roomSend=gain(1),room=gain(0),roomPan=own(context.createStereoPanner());room.connect(roomPan).connect(ambience);
  let level=TIERS[tier]?tier:'high',canyon=null,interior=null,low=null,loaded=null,waiting=0,swaps=0;
  const g={amount:0,cutoff:GLASS.open,gain:1},s={},last=new Float64Array(8).fill(NaN);
  function release(parts){for(const n of parts){try{n.disconnect();}catch(_){}nodes.delete(n);}}
  function route(){
    canyonSplit.disconnect();interiorSplit.disconnect();roomSend.disconnect();
    if(canyon){release([canyon.conv,canyon.ret]);canyon=null;}
    if(low){release([low]);low=null;}
    const t=TIERS[level];
    if(t.canyon){const conv=own(context.createConvolver()),ret=gain(RETURNS.canyon);conv.normalize=false;conv.buffer=buffer(context,impulseCanyon(context.sampleRate));
      canyonSplit.connect(conv);conv.connect(ret).connect(outdoor);canyon={conv,ret};}
    else canyonSplit.connect(verb);
    if(t.interior){if(interior){interiorSplit.connect(interior.conv);roomSend.connect(interior.conv);}}
    else{if(interior){release([interior.conv,interior.ret]);interior=null;loaded=null;}low=gain(RETURNS.low);low.connect(verb);interiorSplit.connect(low);roomSend.connect(low);}
  }
  function load(name){
    if(interior){try{interiorSplit.disconnect(interior.conv);}catch(_){}try{roomSend.disconnect(interior.conv);}catch(_){}release([interior.conv,interior.ret]);}
    const conv=own(context.createConvolver()),ret=gain(RETURNS.interior);conv.normalize=false;conv.buffer=buffer(context,impulseRoom(name,context.sampleRate));
    conv.connect(ret).connect(room);interiorSplit.connect(conv);roomSend.connect(conv);interior={conv,ret};loaded=name;swaps++;
  }
  function set(slot,param,value,tau,eps){if(Math.abs(value-last[slot])<=eps)return;last[slot]=value;param.setTargetAtTime(value,context.currentTime,tau);}
  route();
  return {outdoor,sendOutdoor,sendLocal,room,roomSend,
    // w: createZoneTracker().weights; dt: seconds since the last frame.
    update(w,dt=0){
      glassLaw(w.interior,w.open,w.door,g);sendLaw(w,s);
      set(0,glassFilter.frequency,g.cutoff,.12,g.cutoff*.02);set(1,glassGain.gain,g.gain,.12,.004);
      set(2,openSplit.gain,s.open,.2,.01);set(3,canyonSplit.gain,s.canyon,.2,.01);set(4,interiorSplit.gain,s.interior,.15,.01);set(5,outdoorSplit.gain,s.outdoor,.15,.01);
      set(6,room.gain,roomLaw(w.interior,w.leak),.2,.004);set(7,roomPan.pan,(w.leakPan||0)*(1-clamp01(w.interior)),.2,.02);
      if(!TIERS[level].interior)return;
      const want=w.space||w.leakSpace;if(!want||want===loaded){waiting=0;return;}
      waiting+=dt;if(!loaded||(w.interior<.02&&w.leak<.02)||waiting>1.5){load(want);waiting=0;}
    },
    setTier(value){if(!TIERS[value]||value===level)return;level=value;route();},
    stats:()=>({tier:level,canyon:!!canyon,interior:loaded,swaps,glass:+g.amount.toFixed(3),cutoff:Math.round(g.cutoff),nodes:nodes.size}),
    dispose(){for(const n of nodes){try{n.disconnect();}catch(_){}}nodes.clear();canyon=interior=low=null;loaded=null;},
  };
}
