import {test} from 'node:test';
import assert from 'node:assert/strict';
import {CsiSimulator} from '../../ui/js/vendor/ruview/pose-fusion/js/csi-simulator.js';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import {controlResult,trainingPayload} from '../../ui/js/ruview/control-contracts.mjs';

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
 const document={createElement:()=>({children:[],appendChild(c){this.children.push(c)},addEventListener(){},set innerHTML(value){assert.ok(!value.includes('<img'),'upstream value became executable markup')},set textContent(value){this.text=String(value)}})};
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
