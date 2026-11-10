// Recordings layer (System World beauty pass, phase 5, task 14): six short CC0 loops (ui/3d/system-world/audio/) are
// the main texture of each place: water at the quay, wind in the leaves, birds, gulls, the city's hum and rain. The
// synthesis stays as the layer that reacts (laps, gusts, vents, movers, steps, room tones).
// Nothing is fetched before the engine is running and a frame arrives; then the manifest and the files load once through
// the injected load(path) (an ArrayBuffer for 'manifest.json' or '<id>.ogg', relative to the audio folder). Any failure
// (missing file, decode error, no Vorbis) leaves that recording silent, is counted and is never retried.
// Playback per recording: two looping sources of the same buffer, the second half a loop later, panned -0.6 / +0.6 at
// 0.707 each (a wide, decorrelated bed from a mono loop), into one level gain, into the outdoor bus (so glass damps them
// indoors); birds and gulls also send a little into the outdoor reverb. update() allocates nothing and schedules a
// level only when its value quantized to 1/500 changes. Low tier: water, city and rain only.
const clamp01=v=>v<0?0:v>1?1:v;
// `gain` is the starting level scale (tuned by the sound survey); `send` is the reverb send of the level gain.
export const RECORDINGS=Object.freeze({
  water:Object.freeze({gain:1.95,send:0}),leaves:Object.freeze({gain:1.2,send:0}),birds:Object.freeze({gain:.9,send:.2}),
  gulls:Object.freeze({gain:.9,send:.2}),city:Object.freeze({gain:1.5,send:0}),rain:Object.freeze({gain:1.25,send:0}),
});
export const IDS=Object.freeze(Object.keys(RECORDINGS));
export const LOW_IDS=Object.freeze(['water','city','rain']);
export const LEVEL_STEP=500,LEVEL_TAU=.6,PRESENT=.05,HALF=.707,SPREAD=.6;
// The level of one recording (its law times its gain). w: the tracker's zone weights; f: the sound frame (wind
// strength 0..1); mood: {day 0..1, weather 'clear'|'rain'|'fog', busy}.
export function recordingLevel(id,w,f,mood){
  const spec=RECORDINGS[id];if(!spec)return 0;
  const s=clamp01(f&&f.wind?+f.wind.strength||0:0),day=clamp01(+mood.day||0),rain=mood.weather==='rain',fog=mood.weather==='fog';
  const quay=w.quay||0,green=w.green||0,plaza=w.plaza||0,harbour=w.harbour||0,canyon=w.canyon||0,height=w.height||0;
  let law=0;
  if(id==='water')law=Math.pow(clamp01(quay),1.3);
  else if(id==='leaves')law=green*(.25+.75*s*s);
  else if(id==='birds')law=Math.min(1,green+.35*plaza)*day*(rain?0:fog?.6:1);
  else if(id==='gulls')law=harbour*(.35+.65*day)*(rain?.4:1);
  else if(id==='city')law=(.35+.65*Math.min(1,canyon+.6*plaza+.5*height))*(mood.busy?1.25:1)*(.7+.3*day);
  else if(id==='rain')law=rain?1-.5*height:0;
  return law*spec.gain;
}
export function createSoundRecordings(context,{outdoor,send,load,tier='high'}){
  let level=['low','medium','high','ultra'].includes(tier)?tier:'high',started=false,disposed=false,failed=0,manifest=null,loading=false;
  const mood={day:0,weather:'clear',busy:false},attempted=new Set(),buffers=new Map(),voices=new Map();
  const wanted=()=>level==='low'?LOW_IDS:IDS;
  function build(id,buffer){
    const spec=RECORDINGS[id],nodes=[],own=n=>(nodes.push(n),n),out=own(context.createGain());out.gain.value=0;out.connect(outdoor);
    if(spec.send>0){const wet=own(context.createGain());wet.gain.value=spec.send;out.connect(wet).connect(send);}
    for(let k=0;k<2;k++){
      const src=own(context.createBufferSource()),half=own(context.createGain()),pan=own(context.createStereoPanner());
      src.buffer=buffer;src.loop=true;half.gain.value=HALF;pan.pan.value=k?SPREAD:-SPREAD;src.connect(half).connect(pan).connect(out);
      src.start(0,k?buffer.duration/2:0);
    }
    const voice={id,out,nodes,quantized:NaN,level:0};voices.set(id,voice);return voice;
  }
  function drop(voice){for(const n of voice.nodes){try{n.stop?.();}catch(_){}try{n.disconnect();}catch(_){}}voices.delete(voice.id);}
  // Builds the voices of the current tier from the decoded buffers (also after a tier change).
  function sync(){
    if(disposed)return;const want=wanted();
    for(const [id,buffer]of buffers)if(want.includes(id)&&!voices.has(id))build(id,buffer);
    for(const voice of [...voices.values()])if(!want.includes(voice.id))drop(voice);
  }
  async function fetchOne(id){
    attempted.add(id);
    try{
      const entry=manifest.get(id);if(!entry)throw Error('not in the manifest');
      const bytes=await load(entry),buffer=await context.decodeAudioData(bytes);
      if(!buffer||!(buffer.duration>0))throw Error('empty');
      if(disposed)return;buffers.set(id,buffer);
    }catch(_){if(!disposed)failed++;}
  }
  async function begin(){
    if(loading)return;loading=true;
    try{
      if(!manifest){
        try{
          const list=JSON.parse(new TextDecoder().decode(await load('manifest.json'))).recordings,map=new Map();
          for(const r of Array.isArray(list)?list:[])if(r&&RECORDINGS[r.id]&&/^[a-z0-9-]+\.ogg$/.test(String(r.file)))map.set(r.id,r.file);
          manifest=map;
        }catch(_){if(!disposed){failed+=wanted().filter(id=>!attempted.has(id)).length;for(const id of wanted())attempted.add(id);}if(!manifest)manifest=new Map();return;}
      }
      for(let todo=wanted().filter(id=>!attempted.has(id));todo.length&&!disposed;todo=wanted().filter(id=>!attempted.has(id))){   // a tier change meanwhile adds ids
        await Promise.all(todo.map(fetchOne));sync();
      }
      sync();
    }finally{loading=false;}
  }
  return {
    // f: the sound frame; w: tracker weights. The first call starts the (single) load.
    update(f,w){
      if(disposed)return;
      if(!started){started=true;void begin();}
      for(const voice of voices.values()){
        const value=recordingLevel(voice.id,w,f,mood),q=Math.round(value*LEVEL_STEP);
        if(q===voice.quantized)continue;voice.quantized=q;voice.level=q/LEVEL_STEP;
        voice.out.gain.setTargetAtTime(voice.level,context.currentTime,LEVEL_TAU);
      }
    },
    setMood(m){mood.busy=!!m?.busy;mood.weather=m?.weather||'clear';mood.day=Number.isFinite(m?.day)?m.day:mood.day;},
    setTier(value){
      if(!['low','medium','high','ultra'].includes(value)||value===level)return;level=value;
      if(started&&manifest&&manifest.size&&wanted().some(id=>!attempted.has(id)))void begin();else sync();
    },
    // True while the recording of `id` is loaded and audible enough to carry its place (the synthesised events thin out).
    carries:id=>{const v=voices.get(id);return !!v&&v.level>PRESENT;},
    clear(){},
    stats(){const levels={};for(const v of voices.values())levels[v.id]=+v.level.toFixed(3);
      return {tier:level,loaded:[...voices.keys()],failed,levels,nodes:[...voices.values()].reduce((n,v)=>n+v.nodes.length,0)};},
    dispose(){disposed=true;for(const v of [...voices.values()])drop(v);buffers.clear();},
  };
}
