import {test} from 'node:test';
import assert from 'node:assert/strict';
import {CsiSimulator} from '../../ui/js/vendor/ruview/pose-fusion/js/csi-simulator.js';
import {readFileSync, readdirSync} from 'node:fs';
import vm from 'node:vm';
import {controlResult,trainingPayload} from '../../ui/js/ruview/control-contracts.mjs';

test('pose stream uses the pinned parameter-free WebSocket contract',async()=>{
 const source=readFileSync('ui/js/vendor/ruview/services/pose.service.js','utf8').replace(/^import .*;\r?\n/gm,'').replaceAll('export ','');
 let request;
 const Service=vm.runInNewContext(source+'\nPoseService',{console:{debug(){},info(){},warn(){},error(){}},
  API_CONFIG:{ENDPOINTS:{STREAM:{WS_POSE:'/api/v1/stream/pose'}}},apiService:{authToken:'fixture-token'},
  wsService:{connect:async(path,params)=>{request={path,params};return 'pose-test'},onConnectionStateChange(){}}});
 const service=new Service();
 assert.equal(await service.startPoseStream({zoneIds:['zone_1'],minConfidence:0.3,maxFps:30,token:'fixture-token'}),'pose-test');
 assert.equal(request.path,'/api/v1/stream/pose');
 assert.deepEqual(Object.keys(request.params),[],'unsupported filters and browser credentials must not enter the socket URL');
});

test('embedded tour completion survives reopening without persisting other upstream settings',()=>{
 const source=readFileSync('ui/js/ruview/upstream-adapter.js','utf8');
 const saved=new Map([['host-preference','keep']]),key='ruview-onboarding-done',savedKey='aurago.ruview.onboarding.done';
 const native={getItem:k=>saved.get(k)??null,setItem:(k,v)=>saved.set(k,v),removeItem:k=>saved.delete(k)};
 const open=(storage=native)=>{
  const window={fetch(){},WebSocket:class {}};
  Object.defineProperty(window,'localStorage',{configurable:true,get(){if(storage instanceof Error)throw storage;return storage;}});
  vm.runInNewContext(source,{window,location:{pathname:'/api/ruview/ui/index.html'},AbortController,addEventListener(){},document:{addEventListener(){}}});
  return window.localStorage;
 };
 let storage=open();assert.equal(storage.getItem(key),null);
 for(const name of ['token','simulation','server','url']){storage.setItem(name,'blocked');assert.equal(storage.getItem(name),null);}
 storage.setItem('theme','dark');storage.setItem(key,'true');
 assert.deepEqual([...saved],[['host-preference','keep'],[savedKey,'true']]);
 storage=open();assert.equal(storage.getItem(key),'true');assert.equal(storage.getItem('theme'),null);
 assert.equal(storage.getItem('host-preference'),null);
 storage.setItem(key,'not-a-boolean');assert.equal(open().getItem(key),null);
 storage.setItem(key,'true');storage.removeItem(key);assert.equal(open().getItem(key),null);
 storage.setItem(key,'true');storage.clear();assert.deepEqual([...saved],[['host-preference','keep']]);
 for(const blocked of [new Error('storage unavailable'),{getItem(){throw Error('blocked')},setItem(){throw Error('quota')},removeItem(){throw Error('blocked')}}]){
  storage=open(blocked);storage.setItem(key,'true');assert.equal(storage.getItem(key),'true');storage.clear();assert.equal(storage.getItem(key),null);
 }
});

test('training requests follow each pinned operation contract and reject free model paths',()=>{
 const config={selectedRecordings:['rec_1'],epochs:12,batch_size:4,learning_rate:0.002,patience:3,base_model:'model_1',lora_profile_name:'room_1'};
 assert.deepEqual(trainingPayload('startTraining',config),{dataset_ids:['rec_1'],config:{epochs:12,batch_size:4,learning_rate:0.002,early_stopping_patience:3}});
 assert.deepEqual(trainingPayload('startPretraining',config),{dataset_ids:['rec_1'],epochs:12,lr:0.002});
 assert.deepEqual(trainingPayload('startLoraTraining',config,[{id:'model_1'}]),{dataset_ids:['rec_1'],base_model_id:'model_1',profile_name:'room_1',epochs:12,rank:8});
 for(const base_model of ['../secret','C:\\secret','unlisted']) assert.throws(()=>trainingPayload('startLoraTraining',{...config,base_model},[{id:base_model==='unlisted'?'model_1':base_model}]));
 for(const changed of [{epochs:NaN},{batch_size:0},{patience:-1},{learning_rate:Infinity},{selectedRecordings:[]}]) assert.throws(()=>trainingPayload('startTraining',{...config,...changed}));
 assert.throws(()=>trainingPayload('startLoraTraining',{...config,lora_profile_name:'../room'},[{id:'model_1'}]));
});

test('control responses require explicit operation-specific success',()=>{
 for(const response of [{success:false,error:'disk failure'},{status:'error',message:'busy'},{error:'failed'},{},null]) {
  assert.throws(()=>controlResult(response,'success'));assert.throws(()=>controlResult(response,'started'));
 }
 assert.equal(controlResult({success:true},'success').success,true);
 assert.equal(controlResult({status:'stopping'},'stopping').status,'stopping');
 assert.throws(()=>controlResult({status:'stopping'},'started'));
});

test('recording service shares acknowledged state, reports unknown failures and never retries writes',async()=>{
 const source=readFileSync('ui/js/vendor/ruview/services/training.service.js','utf8').replace(/^import .*;\r?\n/gm,'').replaceAll('export ','');
 let calls=0,response={success:true,recording_id:'test'},pending;
 const Service=vm.runInNewContext(source+'\nTrainingService',{controlResult,console:{debug(){},info(){},warn(){},error(){}},apiService:{post:async()=>{calls++;if(pending)return pending;if(response instanceof Error)throw response;return response},delete:async()=>response}});
 const service=new Service();let started=0,stopped=0;service.on('recording-started',()=>started++);service.on('recording-stopped',()=>stopped++);
 assert.equal(service.recordingState,'unknown');await service.startRecording({id:'test'});assert.equal(service.recordingState,'recording');assert.equal(started,1);
 response={success:false,error:'cannot stop'};await assert.rejects(service.stopRecording());assert.equal(service.recordingState,'unknown');assert.equal(stopped,0);
 response=new Error('network lost');await assert.rejects(service.startRecording());assert.equal(calls,3);assert.equal(started,1);assert.equal(service.recordingBusy,false);
 response={success:true};await service.stopRecording();assert.equal(service.recordingState,'idle');assert.equal(stopped,1);
 let resolve;pending=new Promise(r=>resolve=r);const start=service.startRecording();await assert.rejects(service.startRecording(),/pending/);resolve({success:true});await start;assert.equal(calls,5);
 response={success:false,error:'delete failed'};await assert.rejects(service.deleteRecording('test'));assert.equal(service.recordingState,'recording');
});

function sensingFixture(fetch = async()=>({ok:true,json:async()=>({source:'esp32',source_state:'disconnected'})})) {
 const source=readFileSync('ui/js/vendor/ruview/services/sensing.service.js','utf8').replace(/^import .*;\r?\n/gm,'').replaceAll('export ','');
 const timers=new Map();let next=0;
 const Service=vm.runInNewContext(source+'\nSensingService',{fetch,apiService:{getHeaders:()=>({})},console,
  setTimeout(fn){const id=++next;timers.set(id,fn);return id},clearTimeout(id){timers.delete(id)},clearInterval(){}});
 const service=new Service();service._setState('connected');return {service,timers};
}

test('connected ESP32 without measurements waits instead of claiming synthetic data',async()=>{
 const {service}=sensingFixture();await service._detectServerSource();assert.equal(service.dataSource,'waiting');
 for(const [state,expected] of [['stale','stale'],['synthetic','server-simulated'],['live_verified','live'],['live_unverified','live'],['unknown','waiting'],['__proto__','waiting']]){
  service._applyServerSource('esp32',state);assert.equal(service.dataSource,expected,state);
 }
 for(const fetch of [async()=>({ok:false}),async()=>{throw Error('offline')},async()=>({ok:true,json:async()=>({source:'esp32'})})]){
  const {service}=sensingFixture(fetch);await service._detectServerSource();assert.equal(service.dataSource,'waiting');
 }
});

test('frames recover an unchanged source and expire without inventing data',async()=>{
 const {service,timers}=sensingFixture();await service._detectServerSource();
 service._handleData({source:'esp32'});assert.equal(service.dataSource,'live');assert.equal(timers.size,1);
 const expire=[...timers.values()][0];expire();assert.equal(service.dataSource,'stale');
 service._handleData({source:'esp32'});assert.equal(service.dataSource,'live');
 service._handleData({source:'simulated'});assert.equal(service.dataSource,'server-simulated');
 service._handleData({source:'wifi:wlan0'});assert.equal(service.dataSource,'live');
 service.stop();assert.equal(timers.size,0);expire();assert.notEqual(service.dataSource,'stale');
});

test('live source status without arriving frames also expires',async()=>{
 const {service,timers}=sensingFixture(async()=>({ok:true,json:async()=>({source:'esp32',source_state:'live_unverified'})}));
 await service._detectServerSource();assert.equal(service.dataSource,'live');
 [...timers.values()][0]();assert.equal(service.dataSource,'stale');service.stop();assert.equal(timers.size,0);
});

test('late source probes cannot overwrite a new frame or a stopped connection',async()=>{
 let resolve;const {service}=sensingFixture(()=>new Promise(r=>{resolve=r}));
 const probe=service._detectServerSource();service._handleData({source:'esp32'});
 resolve({ok:true,json:async()=>({source:'esp32',source_state:'disconnected'})});await probe;
 assert.equal(service.dataSource,'live');
 const stopped=service._detectServerSource();service.stop();
 resolve({ok:true,json:async()=>({source:'simulated',source_state:'synthetic'})});await stopped;
 assert.notEqual(service.dataSource,'server-simulated');
});
test('Pose-Fusion requires real CSI and expires live frames',()=>{
 const c=new CsiSimulator();assert.equal(c.isLive,false);
 c._handleJsonFrame({source:'simulated',nodes:[{amplitude:[1,2,3]}]});assert.equal(c.isLive,false);
 c._handleJsonFrame({source:'wifi',nodes:[{amplitude:[1,2,3]}]});assert.equal(c.isLive,false);
 c._handleJsonFrame({source:'esp32',nodes:[{amplitude:[1,2,3]}]});assert.equal(c.isLive,true);
 c.auragoFrameAt=performance.now()-10001;assert.equal(c.isLive,false);
 c.disconnect();assert.equal(c.isLive,false);
});

test('ModelPanel treats upstream counters as text',()=>{
 const source=readFileSync('ui/js/vendor/ruview/components/ModelPanel.js','utf8').replace(/^import .*;\r?\n/gm,'').replace('export default class ModelPanel','class ModelPanel');
 const document={createElement:()=>({dataset:{},children:[],appendChild(c){this.children.push(c)},addEventListener(){},setAttribute(){},set innerHTML(value){assert.ok(!value.includes('<img'),'upstream value became executable markup')},set textContent(value){this.text=String(value)}})};
 const Panel=vm.runInNewContext(source+'\nModelPanel',{document});
 const panel=Object.create(Panel.prototype);
 panel.state={activeModel:{avg_inference_ms:1,frames_processed:'<img src=x onerror="alert(1)">'},models:[],loraProfiles:[]};
 panel._renderActive();
});

test('Observatory refuses implicit demo and expires stalled data',()=>{
 const source=readFileSync('ui/js/vendor/ruview/observatory/js/main.js','utf8').replace(/^import .*;\r?\n/gm,'').replace('new Observatory();','');
 let demo=false,generated=0,cleared=0,updated=0,now=0;
 const window={auragoRuViewSimulationAllowed:()=>demo};
 const document={getElementById:()=>({textContent:'',style:{}}),querySelectorAll:()=>[]};
 const Observatory=vm.runInNewContext(source+'\nObservatory',{window,document,performance:{now:()=>now},requestAnimationFrame(){}});
 const o=Object.create(Observatory.prototype);
 const noop=()=>{};
 Object.assign(o,{settings:{dataSource:'ws'},_clock:{getDelta:()=>0.1,getElapsedTime:()=>0},_demoData:{update(){generated++;return {}}},_renderer:{clear(){cleared++}},_hud:{updateSourceBadge:noop,updateHUD:noop,updateSparkline:noop},_nebula:{update:noop},_figurePool:{update(){updated++}},_scenarioProps:{update:noop},_routerLed:{material:{}},_routerLight:{},_controls:{update:noop},_postProcessing:{update:noop,render:noop},_updateDotMatrixMist:noop,_updateParticleTrail:noop,_updateWifiWaves:noop,_updateSignalField:noop,_updateFPS:noop});
 o._animate();assert.equal(generated,0);assert.equal(updated,0);assert.equal(cleared,1);
 o._liveData={source:'esp32',nodes:[{amplitude:[1,2]}]};o._auragoFrameAt=now;o._animate();assert.equal(updated,1);
 now=10001;o._animate();assert.equal(updated,1);assert.equal(generated,0);
 demo=true;o._animate();assert.equal(generated,1);
 demo=false;o._liveData={source:'simulated'};o._auragoFrameAt=now;o._animate();assert.equal(generated,1);assert.equal(updated,2);
});

test('legacy visualization does not synthesize frames without explicit demo',()=>{
 const html=readFileSync('ui/js/vendor/ruview/viz.html','utf8');
 const code=html.match(/    function mainUpdate\(delta, elapsed\) \{[\s\S]*?\n    \}/)[0];
 let demo=false,generated=0,now=0,hud;
 const noop=()=>{};
 const state={isDemoMode:true,scene:{renderer:{domElement:{style:{}}}},dataProcessor:{generateDemoData(){generated++;return {persons:[],zoneOccupancy:{}}},generateConfidenceHeatmap:noop},signalViz:{updateSignalData:noop,update:noop},bodyModelManager:{update:noop,getActiveCount:()=>0,getAverageConfidence:()=>0},environment:{update:noop,updateZoneOccupancy:noop,updateConfidenceHeatmap:noop},wsClient:{state:'disconnected',getMetrics:()=>({})},hud:{tickFPS:noop,updateState(s){hud=s}}};
 const update=vm.runInNewContext(code+'\nmainUpdate',{state,window:{auragoRuViewSimulationAllowed:()=>demo},performance:{now:()=>now},SignalVisualization:{generateDemoData:noop}});
 update(0.1,0);assert.equal(generated,0);assert.equal(hud.sensingMode,'Unavailable');
 demo=true;update(0.1,0);assert.equal(generated,1);
 demo=false;state.isDemoMode=false;state.auragoFrameAt=now;update(0.1,0);assert.equal(state.scene.renderer.domElement.style.visibility,'');
 now=10001;update(0.1,0);assert.equal(state.scene.renderer.domElement.style.visibility,'hidden');
});

test('opaque relay matches the reviewed endpoint list and refuses other paths',async()=>{
 const {endpoint,streams,reads,writes,cleanPath,tourFrameName}=await import('../../ui/js/ruview/relay.mjs');
 const go=readFileSync('internal/ruview/policy.go','utf8');
 const fn=go.slice(go.indexOf('func Endpoint'),go.indexOf('var readOperations'));
 const groups=[...fn.matchAll(/\[\]string\{([^}]*)\}/g)].map(match=>[...match[1].matchAll(/"(\/[^"]*)"/g)].map(item=>item[1]));
 assert.deepEqual(groups[0],[...streams]);
 assert.deepEqual(groups[1],[...reads]);
 assert.deepEqual(groups[2],[...writes]);
 assert.deepEqual(groups[3],['/api/v1/recording/','/api/v1/models/']);
 assert.equal(endpoint('GET','/api/config'),false);
 assert.equal(endpoint('GET','/api/v1/status?view=raw'),false);
 assert.equal(endpoint('GET','/api/v1/nodes/../sync'),false);
 assert.equal(endpoint('GET','/api/../config'),false);
 assert.equal(endpoint('GET','/api/v1/stream/pose'),'stream');
 assert.equal(endpoint('GET','/ws/sensing'),'stream');
 assert.equal(endpoint('GET','/health/live'),'read');
 assert.equal(endpoint('GET','/api/v1/nodes/board_1/sync'),'read');
 assert.equal(endpoint('POST','/api/v1/recording/stop'),'write');
 assert.equal(endpoint('POST','/api/v1/recording/start'),'write');
 assert.equal(endpoint('DELETE','/api/v1/recording/rec_fixture'),'write');
 assert.equal(endpoint('DELETE','/api/v1/recording/../secret'),false);
 assert.equal(cleanPath('/api/v1/status/'),'/api/v1/status');
 const adapter=readFileSync('ui/js/ruview/upstream-adapter.js','utf8');
 // The source checkout may be CRLF (core.autocrlf); the vendored copy is always LF.
 assert.equal(readFileSync('ui/js/vendor/ruview/aurago-adapter.js','utf8').replaceAll('\r\n','\n'),adapter.replaceAll('\r\n','\n'));
 assert.match(adapter,/postMessage/);
 assert.match(adapter,/origin === 'null'/);
 assert.equal(tourFrameName,'aurago-tour-done');
 const app=readFileSync('ui/js/desktop/apps/ruview.js','utf8');
 assert.match(app,/frame\.sandbox='allow-scripts'/);
 assert.doesNotMatch(app,/allow-same-origin/);
 assert.match(app,/tourFrameName/);
});

test('pose demo needs the simulation gate and stops with the gate, the stream or the tab',()=>{
 const strip=name=>readFileSync('ui/js/vendor/ruview/components/'+name,'utf8').replace(/^import .*;\r?\n/gm,'').replaceAll('export ','');
 let allowed=false,frames=0,rendered=0;const cancelled=[],noop=()=>{};
 const buttons={'demo-btn-c':{style:{},textContent:'Demo'},'run-offline-demo':{style:{display:'none'},textContent:'Demo'}};
 const context={window:{auragoRuViewSimulationAllowed:()=>allowed},document:{getElementById:id=>buttons[id]??null},
  requestAnimationFrame(){return ++frames},cancelAnimationFrame:id=>cancelled.push(id),setInterval:()=>1,clearInterval:noop,poseService:{stopPoseStream:noop}};
 const Canvas=vm.runInNewContext(strip('PoseDetectionCanvas.js')+'\nPoseDetectionCanvas',context);
 const logger={info:noop,debug:noop,warn:noop,error:noop};
 const canvas=Object.assign(Object.create(Canvas.prototype),{containerId:'c',logger,state:{isActive:true,connectionState:'connected'},renderer:{config:{},updateConfig:noop,clearCanvas:noop},showDemoNotification:noop,notifyCallback:noop,updateConnectionIndicator:noop,renderPoseData(){rendered++}});
 // Gate closed: no demo, no frames, no visible entry points.
 canvas.runDemo();canvas.toggleDemo();
 assert.equal(canvas.demoState,undefined);assert.equal(frames,0);assert.equal(rendered,0);
 canvas.updateControls();assert.equal(buttons['demo-btn-c'].style.display,'none');assert.equal(buttons['run-offline-demo'].style.display,'none');
 // Gate open: the demo starts; closing the gate stops it on the next animation step.
 allowed=true;canvas.updateControls();assert.equal(buttons['run-offline-demo'].style.display,'');
 canvas.toggleDemo();assert.equal(canvas.demoState.isRunning,true);assert.equal(frames,1);assert.equal(rendered,1);assert.equal(buttons['run-offline-demo'].textContent,'Stop Demo');
 allowed=false;canvas.startDemoAnimation();
 assert.equal(canvas.demoState.isRunning,false);assert.deepEqual(cancelled,[1]);assert.equal(frames,1);assert.equal(rendered,1);assert.equal(buttons['run-offline-demo'].textContent,'Demo');
 // stop() cancels a running demo.
 allowed=true;canvas.toggleDemo();assert.equal(canvas.demoState.isRunning,true);const frame=canvas.demoAnimationFrame;
 canvas.stop();assert.equal(canvas.demoState.isRunning,false);assert.deepEqual(cancelled,[1,frame]);
 // Leaving the Live Demo tab stops an offline demo even though detection never started.
 canvas.toggleDemo();assert.equal(canvas.demoState.isRunning,true);
 const Tab=vm.runInNewContext(strip('LiveDemoTab.js')+'\nLiveDemoTab',{readError:String});
 const tab=Object.assign(Object.create(Tab.prototype),{state:{isActive:false},components:{poseCanvas:canvas},logger});
 tab.stopDemo();assert.equal(canvas.demoState.isRunning,false);assert.equal(buttons['run-offline-demo'].textContent,'Demo');
});

test('vendored output keeps AuraGo gate strings',()=>{
 const root='ui/js/vendor/ruview/',read=name=>readFileSync(root+name,'utf8');
 const need=(name,...parts)=>{const text=read(name);for(const part of parts)assert.ok(text.includes(part),name+' lost: '+part);return text;};
 need('pose-fusion/js/main.js','if(!csiSimulator.isLive&&!window.auragoRuViewSimulationAllowed?.()){',"statusLabel.textContent = 'EXPERIMENTAL CSI';");
 need('pose-fusion/js/csi-simulator.js','this.verifiedFrame === true && performance.now()-this.auragoFrameAt<10000;',"if(msg.source!=='esp32'||msg._simulated){this.verifiedFrame=false;");
 need('viz.html','const demoAllowed = window.auragoRuViewSimulationAllowed?.() === true;','state.isDemoMode && demoAllowed');
 need('observatory/js/main.js','const demoAllowed = window.auragoRuViewSimulationAllowed?.() === true;');
 need('services/sensing.service.js','function clientSimulationAllowed() { return false; }');
 need('app.js','const useMock = false;','if (false) {');
 const pages=readdirSync(root,{recursive:true}).map(name=>name.replaceAll('\\','/')).filter(name=>name.endsWith('.html'));
 assert.ok(pages.length>=5,'vendored pages');
 for(const page of pages)assert.match(read(page),/<head[^>]*><script src="\.\/aurago-adapter\.js"><\/script>/,page+' lost the adapter script');
 assert.doesNotMatch(read('components/HardwareTab.js'),/Math\.random|set(Interval|Timeout)\(|requestAnimationFrame/,'Hardware view must not invent CSI values');
 for(const name of ['components/HardwareTab.js','components/PoseDetectionCanvas.js'])new vm.Script(read(name).replace(/^import .*;\r?\n/gm,'').replaceAll('export ',''),{filename:name});
 need('components/PoseDetectionCanvas.js','if (window.auragoRuViewSimulationAllowed?.() !== true) return;','style="display:none">&#9881; Demo</button>');
 need('components/LiveDemoTab.js','id="run-offline-demo" style="display:none"');
 const index=need('index.html','<h4>CSI data (not measured in this view)</h4>');
 assert.doesNotMatch(index,/Real-time CSI Data/);
 assert.match(index,/<span id="signalStrength">[^<]*<\/span>/);assert.doesNotMatch(index,/-45 dBm/,'#signalStrength no longer contains -45 dBm');
 assert.match(index,/<span id="latency">—<\/span>/,'#latency is never written by a script');
 const claim='<p class="aurago-upstream-claim">Upstream RuView project claims — not measured or verified by AuraGo.</p>';
 for(const heading of ['<h2 data-i18n="dashboard.title">Revolutionary WiFi-Based Human Pose Detection</h2>','<h2>Performance Analysis</h2>','<h2>Real-World Applications</h2>']){
  const at=index.indexOf(heading);assert.ok(at>=0,heading);
  assert.ok(index.slice(at+heading.length).trimStart().startsWith(claim),'unlabelled claims under '+heading);
 }
 for(const section of ['<div class="key-benefits">','<div class="system-stats">'])assert.ok(index.includes(claim+'\n                '+section),'unlabelled claims before '+section);
 assert.ok(read('style.css').includes('.aurago-upstream-claim {'));
});

test('a hung status read drops the simulation gate',async()=>{
 const source=readFileSync('ui/js/ruview/upstream-adapter.js','utf8');
 const settle=()=>new Promise(resolve=>setImmediate(resolve));
 const timeouts=new Map(),intervals=[],listeners={};let timerID=0,calls=0;
 // Call 1 is the status read (a demo source), call 2 the capabilities; every later status read never settles unless its signal aborts.
 const fetch=(url,options={})=>{
  calls++;
  if(calls===1)return Promise.resolve({ok:true,json:async()=>({enabled:true,readonly:false,source:'simulated'})});
  if(calls===2)return Promise.resolve({ok:true,json:async()=>({unavailable:[]})});
  return new Promise((resolve,reject)=>options.signal.addEventListener('abort',()=>reject(Object.assign(Error('aborted'),{name:'AbortError'})),{once:true}));
 };
 const window={fetch,WebSocket:class {},origin:'http://aurago.test'};
 vm.runInNewContext(source,{window,location:{pathname:'/api/ruview/ui/index.html',href:'http://aurago.test/api/ruview/ui/index.html'},AbortController,
  addEventListener:(type,fn)=>{listeners[type]=fn},document:{addEventListener(){},querySelectorAll:()=>[],body:{}},MutationObserver:class {observe(){}disconnect(){}},
  setTimeout:fn=>{const id=++timerID;timeouts.set(id,fn);return id},clearTimeout:id=>{timeouts.delete(id)},setInterval:fn=>{intervals.push(fn);return ++timerID},clearInterval(){}});
 listeners.DOMContentLoaded();await settle();
 assert.equal(window.auragoRuViewSimulationAllowed(),true);
 assert.equal(timeouts.size,0,'a settled read clears its timeout');
 intervals[0]();await settle();
 assert.equal(calls,3);assert.equal(timeouts.size,1,'the hung read arms a 10 s timeout');
 assert.equal(window.auragoRuViewSimulationAllowed(),true,'still waiting');
 [...timeouts.values()][0]();await settle();
 assert.equal(window.auragoRuViewSimulationAllowed(),false,'a timed-out read withdraws the demo gate');
 intervals[0]();await settle();
 assert.equal(calls,4,'the busy flag was released, so the next poll runs');
});

test('honesty patches fail loudly when an upstream anchor disappears',async()=>{
 const {patchHonesty}=await import('./patch-honesty.mjs');
 const patch=(text,before,after)=>{if(!text.includes(before))throw Error('Pinned RuView adapter patch no longer matches');return text.replace(before,after);};
 for(const name of ['components/HardwareTab.js','components/PoseDetectionCanvas.js','components/LiveDemoTab.js','index.html','style.css'])assert.throws(()=>patchHonesty(name,'// short dummy text without the upstream anchors\n',patch),/no longer matches/,name);
 assert.equal(patchHonesty('components/Unrelated.js','unchanged',patch),'unchanged');
});
