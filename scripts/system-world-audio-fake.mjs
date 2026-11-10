// Deterministic Web Audio double shared by the System World sound tests (phase 5). It models only what the city's
// sound modules use: parameters with their automation calls, node links, sources, filters, panners, convolvers,
// compressors, wave shapers, buffers and the timers the soundscape schedules with. Nothing renders audio.
export function createFakeAudio({sampleRate=8000}={}){
  const nodes=[],tasks=new Map();let id=0;
  const parameter=(value=0)=>({value,ramp:[],
    setValueAtTime(v,t){this.value=v;this.ramp.push(['set',v,t]);return this;},
    linearRampToValueAtTime(v,t){this.value=v;this.ramp.push(['linear',v,t]);return this;},
    exponentialRampToValueAtTime(v,t){this.value=v;this.ramp.push(['exp',v,t]);return this;},
    setTargetAtTime(v,t,c){this.value=v;this.ramp.push(['target',v,t,c]);return this;},
    cancelScheduledValues(){this.ramp.length=0;return this;}});
  function node(kind){
    const n={kind,links:[],type:'',buffer:null,curve:null,oversample:'none',normalize:true,loop:false,reduction:0,started:false,stopped:false,disconnected:false,onended:null,
      gain:parameter(1),frequency:parameter(350),Q:parameter(1),pan:parameter(0),delayTime:parameter(0),detune:parameter(0),playbackRate:parameter(1),
      threshold:parameter(-24),knee:parameter(30),ratio:parameter(12),attack:parameter(.003),release:parameter(.25),
      connect(other){this.links.push(other);this.disconnected=false;return other;},disconnect(){this.disconnected=true;this.links.length=0;},
      start(){this.started=true;},stop(){this.stopped=true;},getFloatTimeDomainData(target){target.fill(0);}};
    nodes.push(n);return n;
  }
  const context={currentTime:0,state:'running',sampleRate,destination:node('destination'),
    createGain:()=>node('gain'),createBiquadFilter:()=>node('filter'),createBufferSource:()=>node('source'),createOscillator:()=>node('oscillator'),
    createDelay:()=>node('delay'),createConvolver:()=>node('reverb'),createWaveShaper:()=>node('shaper'),createDynamicsCompressor:()=>node('compressor'),
    createStereoPanner:()=>node('pan'),createAnalyser:()=>node('analyser'),
    createBuffer:(channels,length,rate=sampleRate)=>({numberOfChannels:channels,length,sampleRate:rate,duration:length/rate,
      data:Array.from({length:channels},()=>new Float32Array(length)),getChannelData(c){return this.data[c];}}),
    decodeAudioData:async()=>({duration:1,numberOfChannels:1,sampleRate,getChannelData:()=>new Float32Array([0,.8,-.8,0])}),
    resume:()=>Promise.resolve(),suspend:()=>Promise.resolve(),close:()=>Promise.resolve()};
  const timers={
    setTimeout(fn,ms){tasks.set(++id,{fn,ms,due:context.currentTime+Math.max(0,Number(ms)||0)/1000});return id;},
    clearTimeout(handle){tasks.delete(handle);},
    // Runs every task due by `time` (context seconds) in due order, advancing currentTime; tasks added meanwhile run if due.
    run(time){context.currentTime=time;for(let guard=0;guard<256;guard++){let key=null,best=null;for(const [k,t]of tasks)if(t.due<=time&&(!best||t.due<best.due)){key=k;best=t;}if(!best)return;tasks.delete(key);best.fn();}},
  };
  // A source ends when the engine stops it. The real API reports that asynchronously.
  const flush=()=>{for(const n of nodes)if(n.onended&&!n.ended){n.ended=true;n.onended();}};
  const live=kind=>nodes.filter(n=>!n.disconnected&&(!kind||n.kind===kind));
  // Every node reachable downstream of `from` along current links (an AudioParam target is a leaf).
  function downstream(from){const seen=new Set(),stack=[from];while(stack.length){const n=stack.pop();if(!n||seen.has(n))continue;seen.add(n);for(const l of n.links||[])stack.push(l);}return seen;}
  return {context,nodes,node,tasks,timers,flush,live,downstream};
}
