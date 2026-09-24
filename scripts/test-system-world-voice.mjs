import assert from 'node:assert/strict';
import {createTowerVoice,towerVoiceInputGain} from '../ui/js/desktop/apps/sysworld-voice.js';

function voiceBuffer(amplitude,peak){
  const data=new Float32Array(24000);
  for(let i=0;i<data.length;i++)data[i]=amplitude*Math.sin(2*Math.PI*220*i/24000);
  data[100]=peak;
  return {sampleRate:24000,numberOfChannels:1,getChannelData:()=>data};
}
const qwenGain=towerVoiceInputGain(voiceBuffer(.124,.596));
assert.ok(qwenGain>2.7&&qwenGain<3.2&&qwenGain>1/.596*1.6,'Qwen-like speech body rises beyond peak-only normalization');
assert.ok(Math.abs(towerVoiceInputGain(voiceBuffer(.5,.8))-1.25)<.001,'Already strong speech is not boosted further');
assert.equal(towerVoiceInputGain(voiceBuffer(.01,.9)),8,'Very quiet speech has a bounded boost');
assert.throws(()=>towerVoiceInputGain(voiceBuffer(0,0)),/Silent voice/);

const native={setTimeout,clearTimeout,fetch,random:Math.random};
let tasks=new Map(),id=0,nodes=[],requests=0,response,decodeResolve;
const parameter=()=>({value:0,setTargetAtTime(value){this.value=value;}});
function node(kind){
  const n={kind,links:[],gain:parameter(),frequency:parameter(),Q:parameter(),pan:parameter(),delayTime:parameter(),
    connect(other){this.links.push(other);return other;},disconnect(){this.disconnected=true;},start(){this.started=true;},stop(){this.stopped=true;}};
  nodes.push(n);return n;
}
const context={currentTime:0,state:'running',sampleRate:8000,
  createGain:()=>node('gain'),createBiquadFilter:()=>node('filter'),createBufferSource:()=>node('source'),
  createDelay:()=>node('delay'),createConvolver:()=>node('reverb'),createWaveShaper:()=>node('limit'),createStereoPanner:()=>node('pan'),
  createBuffer:(channels,length)=>({channels,length,data:Array.from({length:channels},()=>new Float32Array(length)),getChannelData(c){return this.data[c];}}),
  decodeAudioData:async()=>({duration:1,numberOfChannels:1,getChannelData:()=>new Float32Array([0,.8,-.8,0])}),
};
const flush=()=>new Promise(r=>setImmediate(r));
const run=(max=Infinity)=>{const entry=[...tasks].find(([,v])=>v.ms<=max);assert.ok(entry,'Expected scheduled work');tasks.delete(entry[0]);entry[1].fn();};
try{
  globalThis.setTimeout=(fn,ms)=>(tasks.set(++id,{fn,ms}),id);globalThis.clearTimeout=id=>tasks.delete(id);Math.random=()=>.5;
  globalThis.fetch=async(url,options)=>{
    assert.equal(url,'/api/desktop/system-world/voice');assert.equal(options.method,'POST');assert.equal(options.cache,'no-store');
    assert.equal(options.credentials,'same-origin');requests++;return response?.(options)||new Response(new Uint8Array([1,2]),{headers:{'Content-Type':'audio/wav'}});
  };
  const voice=createTowerVoice(context,node('output'));
  voice.setListener(0,2.4,2,0,-1);const near=voice.stats().gain;
  voice.setListener(85,2.4,80,0,-1);const far=voice.stats().gain;
  voice.setListener(200,150,250,0,-1);const beyondCity=voice.stats().gain;
  assert.ok(near>far&&near<.6&&far>.24&&far>beyondCity*2&&beyondCity>.04,'Speech remains audible across the road grid and falls off beyond it');
  assert.ok(near*.35*.75<.14,'Entire mixed voice remains well below full output');
  voice.setListener(NaN,0,0,0,0);assert.equal(voice.stats().gain,beyondCity);
  assert.equal(tasks.size,0,'No work before opt-in/focus');
  voice.setActive(true);assert.equal(tasks.size,1);run(5000);await flush();
  assert.equal(requests,1);assert.equal(voice.stats().state,'speaking');assert.equal(tasks.size,0);
  const source=nodes.find(n=>n.kind==='source'),delay=nodes.find(n=>n.kind==='delay'),room=nodes.find(n=>n.kind==='reverb'),limit=nodes.find(n=>n.kind==='limit');
  assert.ok(Math.abs(source.links[0].gain.value-1.25)<.001,'Playback uses the measured input gain');
  assert.equal(source.links[0].links[0].frequency.value,90,'Low voices retain their fundamental');
  assert.equal(delay.delayTime.value,.085);assert.equal(room.buffer.channels,2);assert.equal(room.buffer.length,6800);
  assert.equal(room.normalize,false,'Room impulse uses calibrated energy, not browser-dependent auto normalization');
  for(let c=0;c<2;c++){
    const data=room.buffer.getChannelData(c);
    assert.ok(data.subarray(0,120).every(x=>x===0),'Room has a short pre-delay');
    let energy=0;for(const sample of data)energy+=sample*sample;
    assert.ok(Math.abs(Math.sqrt(energy)-.65)<.001,'Room energy is calibrated');
  }
  assert.equal(room.links[0].gain.value,.28,'Room return stays audible below the direct voice');
  assert.ok(Math.max(...limit.curve)<=.75&&Math.min(...limit.curve)>=-.75);
  const panner=nodes.find(n=>n.kind==='pan');voice.setListener(30,2.4,-12,0,-1);const right=panner.pan.value;
  voice.setListener(-30,2.4,-12,0,-1);assert.ok(right*panner.pan.value<0,'Spatial direction follows tower');
  source.onended();assert.equal(voice.stats().state,'tail');run(1000);assert.equal(voice.stats().state,'idle');
  assert.equal([...tasks.values()][0].ms,30000,'Pause begins after utterance and room tail');
  assert.ok(nodes.filter(n=>n.kind!=='output').every(n=>n.disconnected),'All per-phrase nodes released');
  voice.setActive(false);assert.equal(tasks.size,0);
  // Abort work on hide, and ignore a response that arrives after abort.
  let resolve,signal;response=opts=>(signal=opts.signal,new Promise(r=>resolve=r));
  voice.setActive(true);run(5000);await flush();assert.equal(voice.stats().pending,true);
  voice.setActive(false);assert.ok(signal.aborted);resolve(new Response(new Uint8Array([1]),{headers:{'Content-Type':'audio/wav'}}));await flush();
  assert.equal(voice.stats().state,'idle');assert.equal(tasks.size,0);
  // A decode already in progress cannot resurrect speech after a lifecycle change.
  response=null;context.decodeAudioData=()=>new Promise(r=>decodeResolve=r);
  voice.setActive(true);run(5000);await flush();voice.setActive(false);
  decodeResolve({duration:1,numberOfChannels:1,getChannelData:()=>new Float32Array([1])});await flush();
  assert.equal(voice.stats().phrases,1);assert.equal(tasks.size,0);
  response=()=>new Response('',{status:503});voice.setActive(true);run(5000);await flush();
  assert.equal(voice.stats().state,'unavailable');assert.equal([...tasks.values()][0].ms,60000,'Provider backoff');
  voice.dispose();assert.equal(tasks.size,0);voice.setActive(true);assert.equal(tasks.size,0);
  console.log('System World tower speech: spatial mixer, echo/reverb, peak cap, pauses, aborts and stale decode verified');
}finally{Object.assign(globalThis,{setTimeout:native.setTimeout,clearTimeout:native.clearTimeout,fetch:native.fetch});Math.random=native.random;}
