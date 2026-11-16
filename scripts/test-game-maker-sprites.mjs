import assert from 'node:assert/strict';
import fs from 'node:fs';

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
  on(){return this;}
  once(){return this;}
}
const objects=[];
const scene={
  textures:{exists:()=>true,get:()=>({has:frame=>Number.isInteger(frame)&&frame>=0&&frame<meta.frames.length})},
  anims:{exists:key=>definitions.has(key),create:definition=>definitions.set(definition.key,definition)},
  add:{sprite:(...args)=>{const o=new Sprite(...args);objects.push(o);return o;},container:(x,y)=>{const o=new Sprite(x,y,'container',0);objects.push(o);return o;}},
  children:{list:objects}
};
helpers.registerAnimations(scene,meta);const count=definitions.size;helpers.registerAnimations(scene,meta);assert.equal(definitions.size,count);
const loadedSheets=[];
const preloadScene={textures:{exists:()=>false},load:{spritesheet:(...args)=>loadedSheets.push(args)}};
helpers.preloadPack(preloadScene,meta,'sheet.png');
helpers.preloadPack(preloadScene,recipes,'sheet.png');
assert.equal(loadedSheets.length,2,'both expanded and original grids must load');
for(const broken of [{...meta,rows:10},{...meta,rows:12.5},{...meta,rows:33},{...meta,columns:0},{...meta,frame_width:32}]){
  assert.throws(()=>helpers.preloadPack(preloadScene,broken,'sheet.png'),/complete 10-column grid/);
}
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
for(const packID of ['human-characters-animated','human-characters-top-down']){
  const humans=JSON.parse(fs.readFileSync(new URL('../internal/gamemaker/asset_packs/'+packID+'/sheet.json',import.meta.url)));
  helpers.registerAnimations(scene,humans);
  for(const entity of ['ranger','knight']){
    const hero=helpers.createAsset(scene,humans,entity+(packID.endsWith('top-down')?'_idle_down':'_idle'),0,0);
    for(const [dx,dy] of (packID.endsWith('top-down')?[[0,-1],[1,0],[0,1],[-1,0]]:[[1,0],[-1,0]])){
      helpers.setFacing(hero,dx,dy);
      for(const action of ['climb_ladder','climb_rope']){
        helpers.playAction(hero,action);
        const clip=definitions.get(hero.current);
        assert.equal(hero.current,humans.id+'@'+humans.version+':'+entity+'_'+action);
        assert.equal(clip.repeat,-1);
        assert.equal(clip.frameRate,8);
        assert.ok(clip.frames.every(f=>f.frame>=100));
      }
      helpers.playAction(hero,'walk');
    }
  }
}
assert.throws(()=>helpers.playAction(ranger,'invented'),/unavailable/);
assert.throws(()=>helpers.playAction(ranger,'walk/right'),error=>{
  assert.match(error.message,/available actions:.*"walk"/);
  assert.match(error.message,/setFacing\(object, dx, dy\)/);
  assert.doesNotMatch(error.message.split('available actions:')[1],/"walk\/right"/);
  return true;
});
// Reproduce the atlas failure from the live maritime evaluation. Valid repair
// hints must name actions for this asset, not animation IDs or another asset.
const atlas=JSON.parse(fs.readFileSync(new URL('../internal/gamemaker/asset_packs/aurago-pirates-topdown/manifest.json',import.meta.url)));
const atlasScene={...scene,textures:{exists:()=>true,get:()=>({has:()=>true})}};
helpers.registerAnimations(atlasScene,atlas);
const island=helpers.createAsset(atlasScene,atlas,'coast-island',0,0);
helpers.playAction(island,'idle');
assert.throws(()=>helpers.playAction(island,'idle/heading-0'),error=>{
  assert.match(error.message,/available actions: "idle"\./);
  assert.match(error.message,/Pass only the action to playAction/);
  assert.doesNotMatch(error.message,/available actions:.*"sailing"/);
  return true;
});
helpers.playAction(island,'idle');
objects.splice(objects.indexOf(island),1);
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
