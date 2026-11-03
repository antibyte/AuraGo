import assert from 'node:assert/strict';
import {createCityFoley} from '../ui/js/desktop/apps/sysworld-foley.js';
import {createCityAmbience,citySoundscapeEvents} from '../ui/js/desktop/apps/sysworld-audio.js';

// A deterministic Web Audio double. Only the parts the city sound engine uses are
// modelled: parameters, sources, filters, panners and the event timers.
const native={setTimeout,clearTimeout,window:globalThis.window,localStorage:globalThis.localStorage};
let tasks=new Map(),id=0,nodes=[];
const parameter=()=>({value:0,ramp:[],
  setValueAtTime(value,time){this.value=value;this.ramp.push(['set',value,time]);return this;},
  linearRampToValueAtTime(value,time){this.value=value;this.ramp.push(['linear',value,time]);return this;},
  exponentialRampToValueAtTime(value,time){this.value=value;this.ramp.push(['exp',value,time]);return this;},
  setTargetAtTime(value,time,constant){this.value=value;this.ramp.push(['target',value,time]);return this;},
  cancelScheduledValues(){this.ramp.length=0;}});
function node(kind){
  const n={kind,links:[],gain:parameter(),frequency:parameter(),Q:parameter(),pan:parameter(),delayTime:parameter(),detune:parameter(),type:'',
    connect(other){this.links.push(other);return other;},disconnect(){this.disconnected=true;},start(){this.started=true;},stop(){this.stopped=true;},
    getFloatTimeDomainData(target){target.fill(0);}};
  nodes.push(n);return n;
}
const context={currentTime:0,state:'running',sampleRate:8000,destination:node('destination'),
  createGain:()=>node('gain'),createBiquadFilter:()=>node('filter'),createBufferSource:()=>node('source'),
  createOscillator:()=>node('oscillator'),
  createDelay:()=>node('delay'),createConvolver:()=>node('reverb'),createWaveShaper:()=>node('limit'),
  createStereoPanner:()=>node('pan'),createAnalyser:()=>node('analyser'),
  createBuffer:(channels,length)=>({channels,length,data:Array.from({length:channels},()=>new Float32Array(length)),getChannelData(c){return this.data[c];}}),
  decodeAudioData:async()=>({duration:1,numberOfChannels:1,getChannelData:()=>new Float32Array([0,.8,-.8,0])}),
  resume:()=>Promise.resolve(),suspend:()=>Promise.resolve(),close:()=>Promise.resolve()};
// A source ends when the engine stops it. The real API reports that asynchronously.
const flush=()=>{for(const n of nodes)if(n.onended&&!n.ended){n.ended=true;n.onended();}};
const kinds=()=>nodes.filter(n=>n.kind==='pan').map(n=>n.pan.value);

try{
  globalThis.setTimeout=(fn,ms)=>(tasks.set(++id,{fn,ms}),id);
  globalThis.clearTimeout=handle=>tasks.delete(handle);
  globalThis.window={AudioContext:function(){return context;}};
  globalThis.localStorage={getItem:()=>null,setItem:()=>{}};

  // --- Robot foley -------------------------------------------------------
  const foley=createCityFoley(context,node('out'),node('ambience'));
  // The listener sits on the street; the origin is the place the view rests on.
  foley.listener(0,0,0,0,-1,0,0);
  assert.equal(foley.stats().voices,0,'No voice before the city is audible');
  assert.equal(foley.action('robot_greet',2,0,0),false,'A quiet city makes no robot sound');
  foley.setActive(true);
  assert.equal(foley.action('robot_greet',2,0,0),true,'An audible city lets a resident answer');
  assert.equal(foley.stats().played,1);
  // The robot layer has to be heard over the city bed, whose mixed RMS is about
  // .01. A sound that stays below that is swallowed by the drones and the murmur.
  assert.ok(foley.stats().peak>.15,'A resident beside the listener is loud enough to hear');
  assert.ok(kinds().at(-1)>.9,'A resident beside the listener sits to the right of the view');
  assert.equal(foley.action('robot_greet',100,0,100),false,'A distant resident stays silent');
  // A resident of the district you look at stays audible even from far above.
  foley.listener(0,0,0,0,-1,0,0);
  assert.equal(foley.action('robot_greet',40,0,20),true,'A resident of the district in view is heard');
  flush();
  assert.equal(foley.stats().voices,0,'A finished sound releases its voices');
  for(let i=0;i<24;i++)foley.action('robot_work',3,0,0,'r'+i);
  assert.equal(foley.stats().voices,10,'The voice pool stays bounded');
  flush();
  // A nearer resident takes the voice from a farther one.
  for(let i=0;i<10;i++)foley.action('robot_work',40,0,0,'far'+i);
  assert.equal(foley.stats().voices,10);
  assert.equal(foley.action('robot_work',2,0,0,'near'),true,'A nearer resident gets a voice');
  flush();
  // One resident answers at most every RETRIGGER seconds.
  context.currentTime=1;foley.action('robot_greet',3,0,0,'same');
  context.currentTime=1.1;
  assert.equal(foley.action('robot_greet',3,0,0,'same'),false,'One resident cannot retrigger itself');
  context.currentTime=2;
  assert.equal(foley.action('robot_greet',3,0,0,'same'),true,'The same resident may answer again later');
  flush();

  // The footfall cadence follows the travelled distance, never the frame rate.
  foley.reset();
  foley.gait('a',0,0,0,false,false);
  foley.gait('a',.3,0,0,false,false);
  assert.equal(foley.stats().steps,0,'A standing resident is silent');
  foley.gait('a',.3,0,0,true,false);
  assert.equal(foley.stats().steps,0,'Starting to move without distance makes no footfall');
  foley.gait('a',1.3,0,0,true,false);
  assert.equal(foley.stats().steps,1,'One metre of travel produces one footfall');
  foley.gait('a',2.1,0,0,true,false);
  assert.equal(foley.stats().steps,2,'The cadence follows the travelled distance');
  foley.gait('a',2.1,0,0,false,false);
  foley.gait('a',12,0,0,true,false);
  assert.equal(foley.stats().steps,2,'A respawn is not a stride');
  foley.gait('b',0,0,0,true,true);
  foley.gait('b',.9,0,0,true,true);
  assert.equal(foley.stats().steps,2,'A carried parcel steps on a longer stride');
  foley.gait('b',1.9,0,0,true,true);
  assert.equal(foley.stats().steps,3,'The carried stride completes');
  foley.gait('c',0,0,0,true,false,'glide');
  foley.gait('c',1.7,0,0,true,false,'glide');
  assert.equal(foley.stats().steps,4,'A gliding resident keeps its own longer stride');
  // A footfall beside the listener is the quietest sound of the layer and still
  // has to be louder than the city bed.
  assert.ok(foley.stats().peak>.2,'A footfall beside the listener is loud enough to hear');
  flush();
  foley.setActive(false);
  assert.equal(foley.stats().voices,0,'A muted city releases every robot voice');

  // --- City soundscape ----------------------------------------------------
  const ambience=createCityAmbience(()=>{});
  ambience.toggle();
  ambience.setActive(true);
  ambience.setListener(0,2.4,0,0,-1);
  assert.ok(ambience.stats().soundscape.scheduled,'The soundscape schedules while audible');
  ambience.effect('robot_greet',2,0,0,'resident-1');
  assert.equal(ambience.stats().foley.played,1,'A robot action reaches the foley');
  ambience.effect('step',2,0,0);
  assert.equal(ambience.stats().foley.played,1,'A world interaction never reaches the foley');
  ambience.gait('resident-1',0,0,0,false,false);
  ambience.gait('resident-1',1,0,0,true,false);
  assert.equal(ambience.stats().foley.steps,1,'The stride reaches the foley');
  ambience.setListener(0,2.4,76,0,-1,0,76);
  assert.ok(ambience.stats().soundscape.beds.water>.04,'Harbour water is loud at the quay');
  ambience.setListener(0,2.4,0,0,-1,0,0);
  assert.ok(ambience.stats().soundscape.beds.water<.01,'Water fades inland');
  ambience.setActive(false);
  assert.equal(ambience.stats().soundscape.scheduled,false,'A hidden city schedules nothing');
  ambience.dispose();

  // --- The event table ----------------------------------------------------
  const offered=mood=>new Set(citySoundscapeEvents(mood).map(e=>e.kind));
  const day=offered({day:1,evening:0,weather:'clear'});
  const night=offered({day:0,evening:0,weather:'clear'});
  const rain=offered({day:1,evening:0,weather:'rain'});
  const evening=offered({day:0,evening:1,busy:true,weather:'clear'});
  assert.ok(day.has('birds')&&day.has('gull')&&day.has('pigeon'),'A clear day answers with birds, gulls and pigeons');
  assert.ok(!day.has('cricket')&&!night.has('birds'),'Day and night do not share the same animals');
  assert.ok(night.has('cricket')&&night.has('horn')&&night.has('bell'),'A clear night answers with crickets, the harbour horn and the civic bell');
  assert.ok(rain.has('drip')&&!rain.has('cricket')&&!rain.has('birds'),'Rain replaces the animals with drips');
  assert.ok(evening.has('siren'),'A busy evening carries a distant siren');
  assert.ok([...day,...night,...rain,...evening].length>=13,'The soundscape offers many different events');
  console.log('System World city sound: robot foley, gait cadence, bounded voices, spatial beds and the varied event table verified');
}finally{
  Object.assign(globalThis,{setTimeout:native.setTimeout,clearTimeout:native.clearTimeout,window:native.window,localStorage:native.localStorage});
}
