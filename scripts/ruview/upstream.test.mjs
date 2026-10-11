import {test} from 'node:test';
import assert from 'node:assert/strict';
import {CsiSimulator} from '../../ui/js/vendor/ruview/pose-fusion/js/csi-simulator.js';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
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
