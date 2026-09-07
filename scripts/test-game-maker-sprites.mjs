import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../internal/gamemaker/build.go', import.meta.url), 'utf8');
const guard = source.match(/const phaserSpriteGuard = `([\s\S]*?)`/)[1];
const calls = [];
class Loader {
  addFile(input) { calls.push({ loader: this, input }); return this; }
}
class TextureManager {
  exists(key) { return key === 'loaded'; }
  get(key) { return key; }
}
const context = { Phaser: { Loader: { LoaderPlugin: Loader }, Textures: { TextureManager } } };
vm.runInNewContext(guard, context);
const wrapped = Loader.prototype.addFile;
vm.runInNewContext(guard, context);
assert.equal(Loader.prototype.addFile, wrapped, 'guard must install only once');
const textures=new TextureManager(), get=textures.get;
vm.runInNewContext(guard, context);
assert.equal(textures.get,get);
assert.throws(()=>textures.get({id:'blocks-and-balls',assets:[]}),/pack JSON.*createAsset/);
assert.throws(()=>textures.get('absent'),/texture absent is not loaded/);
assert.equal(textures.get('loaded'),'loaded');
assert.equal(textures.get(undefined),undefined);
const nativeTexture={key:'loaded'};assert.equal(textures.get(nativeTexture),nativeTexture);
const loader = new Loader();
const url = 'assets/builtin/space-shooter/1/sheet.png';
const good = { type: 'spritesheet', url, config: { frameWidth: 64, frameHeight: 64 } };
assert.equal(loader.addFile(good), loader);
assert.equal(calls[0].loader, loader);
assert.equal(calls[0].input, good);
loader.addFile([good, { type: 'image', url: 'assets/custom.png' }]);
loader.addFile({ type: 'json', url: 'assets/builtin/space-shooter/1/sheet.json' });
loader.addFile({ ...good, config: { frameWidth: 64 } });
for (const file of [
  { type: 'image', url },
  { type: 'image', url: '/api/game-maker/preview/token/' + url + '?v=1' },
  { type: 'image', url: url.replace('space-shooter', 'space%2Dshooter') },
  { ...good, config: { frameWidth: 640, frameHeight: 640 } },
  { ...good, config: { frameWidth: 64, frameHeight: 32 } },
  { ...good, config: { frameWidth: 64, spacing: 1 } },
  { ...good, config: { frameWidth: 64, startFrame: 10 } },
]) {
  const before = calls.length;
  assert.throws(() => loader.addFile(file), /load\.spritesheet.*frameWidth:64/);
  assert.throws(() => loader.addFile([good, file]), /load\.spritesheet/);
  assert.equal(calls.length, before, 'invalid batches must not reach the loader');
}
vm.runInNewContext(guard, {}); // A Three.js game has no Phaser global.
console.log('PASS: built-in sprite loader contract, arrays, URLs and idempotence');

const helpers = await import('../internal/gamemaker/runtime/aurago-game-1.js');
const meta = JSON.parse(fs.readFileSync(new URL('../internal/gamemaker/asset_packs/human-characters-animated/sheet.json', import.meta.url)));
const recipes = JSON.parse(fs.readFileSync(new URL('../internal/gamemaker/asset_packs/vehicles-planes-top-down/sheet.json', import.meta.url)));
const definitions = new Map(); let animationStarts = 0;
class Sprite {
  constructor(x,y,key,frame){Object.assign(this,{x,y,width:64,height:64,texture:{key},frame:{name:frame,width:64,height:64,cutX:0,cutY:0,source:{image:{}}},rotation:0,scaleX:1,scaleY:1});this.anims={stop(){}};}
  setPosition(x,y){this.x=x;this.y=y;return this;}
  setOrigin(x,y){this.originX=x;this.originY=y;return this;}
  setRotation(value){this.rotation=value;return this;}
  setFlipX(value){this.flipX=value;return this;}
  setFrame(value){this.frame.name=value;return this;}
  setScale(x,y=x){this.scaleX=x;this.scaleY=y;return this;}
  setSize(w,h){this.width=w;this.height=h;return this;}
  play(key,ignore){assert.ok(definitions.has(key));if(!(ignore&&this.current===key))animationStarts++;this.current=key;return this;}
  add(child){(this.list ||= []).push(child);return this;}
}
const objects=[];
const scene={
  textures:{exists:()=>true,get:()=>({has:frame=>Number.isInteger(frame)&&frame>=0&&frame<100})},
  anims:{exists:key=>definitions.has(key),create:definition=>definitions.set(definition.key,definition)},
  add:{sprite:(...args)=>{const o=new Sprite(...args);objects.push(o);return o;},container:(x,y)=>{const o=new Sprite(x,y,'container',0);objects.push(o);return o;}},
  children:{list:objects}
};
helpers.registerAnimations(scene,meta);const count=definitions.size;helpers.registerAnimations(scene,meta);assert.equal(definitions.size,count);
const anim=meta.animations.find(a=>a.id==='ranger_walk');
assert.deepEqual(definitions.get(meta.id+'@'+meta.version+':ranger_walk').frames.map(f=>f.frame),anim.frames);
assert.equal(definitions.get(meta.id+'@'+meta.version+':ranger_walk').sortFrames,false);
const ranger=helpers.createAsset(scene,meta,'ranger_idle',100,120);
let reads=0;
globalThis.document={createElement:()=>({getContext:()=>({drawImage(){},getImageData(){reads++;const data=new Uint8ClampedArray(64*64*4);for(let y=20;y<36;y++)for(let x=8;x<56;x++)data[(y*64+x)*4+3]=255;return {data};}})})};
const fitted=helpers.fitVisual(ranger,120,18);
assert.equal(ranger.scaleX,ranger.scaleY,'fit must preserve pixel aspect');
assert.equal(fitted.width,54);assert.equal(fitted.height,18);
const second=helpers.createAsset(scene,meta,'ranger_idle',0,0);helpers.fitVisual(second,120,18);
assert.equal(reads,1,'bounds must be shared across instances');
assert.throws(()=>helpers.fitVisual(second,0,18),/positive finite/);
delete globalThis.document;
helpers.playAction(ranger,'walk');const starts=animationStarts;helpers.playAction(ranger,'walk');assert.equal(animationStarts,starts);
helpers.setFacing(ranger,-1,0);assert.equal(ranger.flipX,true);helpers.setFacing(ranger,0,0);assert.equal(ranger.flipX,true);
assert.throws(()=>helpers.playAction(ranger,'invented'),/unavailable/);
assert.throws(()=>helpers.setFacing(ranger,0,-1),/forbidden/);
helpers.registerAnimations(scene,recipes);
const tank=helpers.createAssembly(scene,recipes,'tank',240,150),offsets=tank.list.map(p=>[p.x,p.y]);
helpers.setFacing(tank,1,0);assert.equal(tank.rotation,Math.PI/2);assert.deepEqual(tank.list.map(p=>[p.x,p.y]),offsets);
assert.equal(helpers.inspectAssets(scene).invalid_assets,0);
tank.list[0].x+=10;assert.ok(helpers.inspectAssets(scene).invalid_assets>0);
assert.throws(()=>helpers.createAsset(scene,recipes,recipes.assemblies[0].parts[0].asset_id,0,0),/fragment/);
const beforeMissing=objects.length;
scene.textures.exists=()=>false;
assert.throws(()=>helpers.createAsset(scene,meta,'ranger_idle',0,0),/ranger_idle.*not loaded.*inside preload/);
assert.throws(()=>helpers.createAssembly(scene,recipes,'tank',0,0),/tank.*not loaded/);
scene.textures.exists=()=>true;
scene.textures.get=()=>({has:()=>false});
assert.throws(()=>helpers.createAsset(scene,meta,'ranger_idle',0,0),/missing numeric frame/);
assert.throws(()=>helpers.createAssembly(scene,recipes,'tank',0,0),/missing numeric frame/);
assert.equal(objects.length,beforeMissing,'missing textures/frames must not create placeholders or partial assemblies');
objects.length=0;objects.push(new Sprite(0,0,'__MISSING',0));
assert.equal(helpers.inspectAssets(scene).invalid_assets,1,'Phaser placeholders must fail even without helper tracking');
const shutdown=[];
const testScene={sys:{},events:{once:(event,callback)=>{assert.equal(event,'shutdown');shutdown.push(callback);}}};
const player={active:true,scene:testScene},state={score:0};
assert.throws(()=>helpers.bindGameTest(testScene,state,undefined),/assign this.player/);
assert.throws(()=>helpers.bindGameTest(testScene,state,{active:true,scene:{}}),/live object in this scene/);
helpers.bindGameTest(testScene,state,player);
assert.equal(globalThis.__AURAGO_GAME_TEST__.player,player);
helpers.bindGameTest(testScene,state,player);shutdown[0]();
assert.equal(globalThis.__AURAGO_GAME_TEST__.state,state,'old shutdown cannot erase new binding');
shutdown[1]();assert.equal(globalThis.__AURAGO_GAME_TEST__,undefined);
console.log('PASS: helper animation order/holds, idempotence, facing, missing actions and assembly geometry');

const previewWindow = {};
vm.runInNewContext(fs.readFileSync(new URL('../ui/js/desktop/apps/game-maker-studio-preview.js', import.meta.url),'utf8'), {window:previewWindow,clearTimeout});
const reports=[],sent=[],diagnostics=[];
const previewState={frame:{contentWindow:{postMessage:message=>sent.push(message)}},channelID:'channel',project:{id:'project'},previewProjectID:'project',previewReported:new Set(),previewDiagnostics:[],
  previewGrant:{token:'token',validation_id:'build',expires_at:new Date(Date.now()+60000).toISOString(),scenarios:[{id:'required_input'}]},api:{reportPreview:(id,payload)=>{reports.push({id,payload});return Promise.resolve();}},addDiagnostic:message=>diagnostics.push(message)};
const receive=(data,source=previewState.frame.contentWindow)=>previewWindow.GameMakerStudioPreview.handleMessage(previewState,{source,data:{source:'aurago-game',channel:'channel',...data}});
receive({type:'ready',boot:true,visible:true},{});receive({type:'ready',boot:false,visible:true});receive({type:'ready',boot:true,visible:true,channel:'stale'});
assert.equal(reports.length,0,'only the current iframe and server boot may establish readiness');
receive({type:'ready',boot:true,visible:true});assert.equal(reports[0].payload.canvas_visible,true);assert.equal(sent[0].type,'run-tests');
receive({type:'gameplay',observations:Array(17).fill({})});assert.equal(reports.length,1);
receive({type:'gameplay',observations:[{id:'required_input',before:{player_x:1},after:{player_x:2}}],images:['data:image/png;base64,a','b','c']});
assert.equal(reports.length,2);assert.equal(reports[1].payload.images.length,2);assert.equal(reports[1].payload.token,'token');
receive({type:'gameplay',observations:[]});assert.equal(reports.length,2,'one report per current window');
receive({type:'runtime_error',message:'new error'});receive({type:'ready',boot:true,visible:true});assert.equal(diagnostics.length,1,'ready cannot clear a current error');
let rejectOld;
previewState.api.reportPreview=()=>new Promise((_,reject)=>{rejectOld=reject;});
receive({type:'runtime_error',message:'pending report'});
previewState.previewGrant=null;const diagnosticCount=diagnostics.length;rejectOld(Error('old window failure'));await Promise.resolve();
assert.equal(diagnostics.length,diagnosticCount,'late report failures must not repopulate diagnostics after reset');
console.log('PASS: Studio bridge window/channel binding, evidence limits, replay and stale-response isolation');
