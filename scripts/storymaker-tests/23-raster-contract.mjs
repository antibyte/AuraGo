// Actual GSAP seek parity, texture integrity and deterministic ImageGen-master slicing.
import assert from 'node:assert/strict';
import vm from 'node:vm';
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { loadPlayer, loadHost, read, readJSON, root, FakeCanvas, legacyCatalog, legacyAssets, realCatalog, story } from '../storymaker-test-support.mjs';
import { decodePNG, encodePNG, cropPNG, cellRect, cellSubjectBounds, thumbnailPNG } from '../../assets/storymaker/lib/png.mjs';
import { t } from '../storymaker-test-context.mjs';

t.check('app texture stream: invalid size metadata is refused before fetching and excess bytes cancel the reader',async()=>{
    const context=vm.createContext({window:{StorymakerParts:{errors:{normalize:e=>e}}},AbortController,Uint8Array});
    vm.runInContext(read('ui/js/desktop/apps/storymaker-api.js'),context);
    let fetched=0,cancelled=0,released=0;
    const api=context.window.StorymakerParts.createApi({fetch:async()=>{fetched++;return{ok:true,body:{getReader:()=>({read:async()=>({value:new Uint8Array(3)}),cancel:async()=>{cancelled++;},releaseLock:()=>{released++;}})}};}});
    for(const bytes of [undefined,NaN,Infinity,'3',1.5,0,8*1048576+1])await assert.rejects(api.textureBuffer({path:'x.png',bytes}));
    assert.equal(fetched,0);await assert.rejects(api.textureBuffer({path:'x.png',bytes:2}));
    assert.equal(cancelled,1);assert.equal(released,1);
});

t.check('production catalog: all legacy IDs survive among twelve identities, twelve places and eighty props',()=>{
    const catalog=realCatalog();assert.equal(catalog.schema,'storymaker-catalog/2');
    for(const [kind,count]of Object.entries({character:12,environment:12,prop:80}))assert.equal(catalog.entries.filter(e=>e.kind===kind).length,count);
    const ids=new Set(catalog.entries.map(e=>e.id));for(const e of legacyCatalog().entries)assert.ok(ids.has(e.id),'Missing legacy ID '+e.id);
    for(const language of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']){
        const names=readJSON('ui/lang/storymaker_catalog/'+language+'.json');for(const e of catalog.entries)assert.ok(names[e.name_key],language+'/'+e.name_key);
    }
});

t.check('production catalog: compiler timing stays compatible and every bitmap need belongs to the inventory',()=>{
    const core=loadPlayer().StorymakerCore,catalog=realCatalog(),files=core.textures.index(catalog);
    for(const name of ['firewall','speech']){
        const input=story(name),old=core.compile(input,{},legacyCatalog()),current=core.compile(input,{},catalog);
        assert.equal(current.report.frame_count,old.report.frame_count);
        assert.deepEqual(current.report.scenes,old.report.scenes);
        for(const key of current.needs.filter(k=>k.startsWith('texture:')))assert.ok(files[key.slice(8)],key);
        assert.ok(current.needs.some(k=>k.startsWith('texture:')));
    }
});

t.check('production provenance: all 123 original masters and their reference relationships verify',()=>{
    const production=readJSON('assets/storymaker/production.json'),base=path.join(root,'assets/storymaker');
    assert.equal(production.schema,'storymaker-production/1');assert.equal(production.images.length,123);
    const files=new Set(production.images.map(e=>e.file));assert.equal(files.size,123);
    const originals=fs.readdirSync(path.join(base,'masters'),{recursive:true}).filter(file=>file.endsWith('.png')&&!file.startsWith('previews'+path.sep)).map(file=>'masters/'+file.replaceAll(path.sep,'/'));
    assert.deepEqual(new Set(originals),files,'Every retained original needs a production record');
    for(const source of production.images){
        assert.ok(source.file.startsWith('masters/')&&!source.file.includes('..'));assert.equal(source.tool,'ImageGen');assert.ok(source.prompt.trim().length>20);
        const bytes=fs.readFileSync(path.join(base,source.file));assert.equal(bytes.length,source.bytes);assert.equal(createHash('sha256').update(bytes).digest('hex'),source.sha256);
        assert.equal(bytes.readUInt32BE(16),source.width);assert.equal(bytes.readUInt32BE(20),source.height);
        for(const reference of source.references)assert.ok(files.has(reference),source.file+' → '+reference);
    }
});

t.check('textures: SHA-256 agrees with the platform on padding boundaries and a large image-sized buffer',()=>{
    const hash=t.core().textures.sha256;
    for(const length of [0,3,55,56,63,64,65,1024,1048576]){
        const bytes=Uint8Array.from({length},(_,i)=>(i*37+19)&255);
        assert.equal(hash(bytes.buffer),createHash('sha256').update(bytes).digest('hex'));
    }
});

t.check('renderer selection: known CPU rasterizers use Canvas and unavailable private GPU names retain WebGL',()=>{
    const detect=loadPlayer().StorymakerCore.raster.softwareGL;
    for(const name of ['ANGLE SwiftShader Device','Mesa llvmpipe','softpipe','swrast','Microsoft Basic Render Driver'])assert.equal(detect({getExtension:()=>({UNMASKED_RENDERER_WEBGL:1}),getParameter:()=>name}),true,name);
    for(const name of ['ANGLE NVIDIA RTX','AMD Radeon','Intel Iris'])assert.equal(detect({getExtension:()=>({UNMASKED_RENDERER_WEBGL:1}),getParameter:()=>name}),false,name);
    assert.equal(detect({getExtension:()=>null}),false);assert.equal(detect({getExtension:()=>{throw new Error('private');}}),false);
});

t.check('raster deadlines: trusted painting gets its bounded allowance while untrusted code retains 250 ms',async()=>{
    const context=loadHost(),H=context.StorymakerHost,calls=[];
    context.StorymakerCore.schedule.framePlan=()=>({layers:[{i:0,f:0}]});
    const engine=H.createEngine({limits:H.LIMITS});engine.schedule={fps:30,scenes:[{id:'code',kind:'code'}]};engine.mode='workers';engine.rendered=10;
    engine.player={call:async(m,ms)=>{calls.push(['player',m.type,ms]);return{bitmap:{}};}};
    engine.sceneChannels.code={call:async(m,ms)=>{calls.push(['scene',m.type,ms]);return{bitmap:{}};}};
    engine.rasterCatalog=true;await engine.renderNow(0,{});
    assert.ok(calls.some(c=>c[0]==='scene'&&c[1]==='frame'&&c[2]===250));
    assert.ok(calls.some(c=>c[0]==='player'&&c[1]==='render'&&c[2]===4000));
    calls.length=0;engine.rasterCatalog=false;await engine.renderNow(0,{});
    assert.ok(calls.some(c=>c[0]==='player'&&c[1]==='render'&&c[2]===250));
    let closed=0;engine.rasterCatalog=true;
    engine.sceneChannels.code.call=async m=>m.type==='frame'?{bitmap:{close(){closed++;}}}:{};
    engine.player.call=async()=>{throw{code:'cancelled',message:'disposed'};};
    await assert.rejects(engine.renderNow(1,{}),e=>e.code==='cancelled');assert.equal(closed,1);
});

t.check('raster lifecycle: a failed frame releases incoming code-scene bitmaps',async()=>{
    const core=loadPlayer().StorymakerCore;let closed=0;
    const player=new core.workerPlayer.Runtime({makeCanvas:(w,h)=>new FakeCanvas(w,h)});player.timeline={width:10,height:10};
    core.renderFrame=()=>{throw new Error('draw failed');};
    await assert.rejects(player.handle({type:'render',n:0,codeFrames:{scene:{close(){closed++;}}}}),/draw failed/);
    assert.equal(closed,1);
});
t.check('textures: delivery metadata rejects traversal, duplicate paths, bad hashes, oversized and malformed images',()=>{
    const index=t.core().textures.index;
    const valid={path:'textures/actor.png',type:'image/png',bytes:120,sha256:'a'.repeat(64),width:20,height:30};
    assert.ok(index({files:[valid]})[valid.path]);
    for(const patch of [{path:'../actor.png'},{path:'a/../b.png'},{path:'a\\b.png'},{path:'/a.png'},{sha256:'bad'},{bytes:8*1048576+1},{width:0},{width:100000},{type:'image/svg+xml'}])assert.throws(()=>index({files:[{...valid,...patch}]}));
    assert.throws(()=>index({files:[valid,valid]}));
    assert.throws(()=>index({files:[{...valid,width:2049}]}));
    assert.ok(index({files:[{...valid,path:'textures/environments/office-day.png',width:3840,height:2160}]}));
    assert.throws(()=>index({files:[{...valid,path:'textures/environments/foreground-day.png',width:2049}]}));
    assert.throws(()=>index({files:[{...valid,path:'thumbs/actor.png',width:257}]}));
});
t.check('GSAP: the pinned runtime samples overlapping numeric and logarithmic tracks in either seek direction',()=>{
    const legacy=loadPlayer().StorymakerCore;
    const context=loadPlayer({navigator:{userAgent:'Storymaker test'}});
    vm.runInContext(read('ui/js/vendor/storymaker/runtime.js'),context);
    const actual=context.StorymakerCore;
    for(const ease of ['linear','ease-in','ease-out','ease-in-out','bounce','elastic']){
        for(const log of [false,true]){
            const build=core=>new core.timeline.Track(2).add(0,3,8,ease,2,log).add(2,5,4,ease,undefined,log);
            const a=build(actual),b=build(legacy);
            for(const at of [0,0.1,1,2,2.2,2.9,3,4.9,5,2.2,0,4.9,1])assert.ok(Math.abs(a.at(at)-b.at(at))<0.00002,`${ease}/${log}/${at}: ${a.at(at)} vs ${b.at(at)}`);
        }
    }
    assert.equal(context.StorymakerVendor.gsap.version,'3.15.1');
});

t.check('GSAP lifecycle: disposing a player or rebuilt code scene detaches only its own numeric animations',async()=>{
    const context=loadPlayer({navigator:{userAgent:'Storymaker lifecycle test'}});
    vm.runInContext(read('ui/js/vendor/storymaker/runtime.js'),context);
    const core=context.StorymakerCore,gsap=context.StorymakerVendor.gsap;
    const track=()=>{const t=new core.timeline.Track(0).add(0,2,10);t.at(1);return t;};
    const retained=track(),baseline=gsap.globalTimeline.getChildren().length;
    const player=new core.workerPlayer.Runtime({});player.timeline={scenes:[{entities:[{x:track()}],camera:{zoom:track()}}]};
    assert.ok(gsap.globalTimeline.getChildren().length>baseline);
    await player.handle({type:'dispose'});assert.equal(gsap.globalTimeline.getChildren().length,baseline);
    const scene=new core.workerScene.Runtime({},{});scene.built={entities:[{x:track()}],camera:{zoom:track()}};
    await scene.handle({type:'dispose'});assert.equal(gsap.globalTimeline.getChildren().length,baseline);
    assert.equal(retained.at(1),5);retained.dispose();
});

t.check('raster lifecycle: a late bitmap or font cannot repopulate a disposed project',async()=>{
    let resolveBitmap,resolveFont,closed=0,registered=0;
    const core=loadPlayer({Blob,createImageBitmap:()=>new Promise(resolve=>{resolveBitmap=resolve;})}).StorymakerCore;
    const png=Uint8Array.from(encodePNG({width:1,height:1,pixels:Buffer.alloc(4)})).buffer;
    const meta={path:'actor.png',type:'image/png',bytes:png.byteLength,sha256:core.textures.sha256(png),width:1,height:1};
    let active=true;const held={},pending=core.textures.decode({files:[meta]},{'texture:actor.png':png},held,()=>active);
    active=false;resolveBitmap({width:1,height:1,close(){closed++;}});
    await assert.rejects(pending,/disposed/);assert.equal(closed,1);assert.deepEqual(held,{});
    const player=new core.workerPlayer.Runtime({fonts:{add(){registered++;}},FontFace:class{load(){return new Promise(resolve=>{resolveFont=resolve;});}}});
    const loading=player.handle({type:'assets',fonts:[{name:'default',family:'Geist',weight:400,data:new ArrayBuffer(1)}]});
    await player.handle({type:'dispose'});resolveFont();await assert.rejects(loading,e=>e.message.includes('disposed'));assert.equal(registered,0);
});
t.check('PNG production: lossless RGBA roundtrip, calibrated slicing and corruption rejection',()=>{
    const pixels=Buffer.from([0,0,0,0,200,40,20,255,10,20,30,128,255,255,255,255]);
    const source={width:2,height:2,pixels},encoded=encodePNG(source),decoded=decodePNG(encoded);
    assert.deepEqual(decoded,source);
    assert.deepEqual(cropPNG(decoded,[1,0,1,2]).pixels,Buffer.from([200,40,20,255,255,255,255,255]));
    assert.throws(()=>cropPNG(decoded,[1,1,2,2]));
    const broken=Buffer.from(encoded);broken[45]^=1;assert.throws(()=>decodePNG(broken));
    assert.ok(encoded.equals(encodePNG(source)));
    assert.deepEqual(cellRect({width:1536,height:1024},53,9,6),[1365,853,171,171]);
    assert.deepEqual(thumbnailPNG(source,1),{width:1,height:1,pixels:Buffer.from([255,255,255,255])});
    const sheet={width:8,height:5,pixels:Buffer.alloc(8*5*4)};
    for(let y=1;y<4;y++)for(let x=3;x<7;x++)sheet.pixels[(y*8+x)*4+3]=255;
    sheet.pixels[(2*8)*4+3]=255;
    assert.deepEqual(cellSubjectBounds(sheet,0,1,1),[3,1,4,3]);
    const crossing={width:24,height:5,pixels:Buffer.alloc(24*5*4)};
    for(let y=1;y<4;y++)for(let x=10;x<18;x++)crossing.pixels[(y*24+x)*4+3]=255;
    crossing.pixels[(2*24+6)*4+3]=255;
    assert.deepEqual(cellSubjectBounds(crossing,1,3,1),[10,1,8,3]);
});
t.check('textures: corrupted or oversized encoded dimensions are refused before allocating a bitmap',async()=>{
    let allocations=0,closed=0;
    const core=loadPlayer({Blob,createImageBitmap:async()=>{allocations++;return{width:4,height:3,close(){closed++;}};}}).StorymakerCore;
    const png=encodePNG({width:4,height:3,pixels:Buffer.alloc(48)}),bytes=Uint8Array.from(png).buffer;
    const meta={path:'actor.png',type:'image/png',bytes:bytes.byteLength,sha256:core.textures.sha256(bytes),width:4,height:3};
    const held={};await core.textures.decode({files:[meta]},{'texture:actor.png':bytes},held);
    assert.equal(allocations,1);core.textures.dispose(held);assert.equal(closed,1);
    const bomb=bytes.slice(0);new DataView(bomb).setUint32(16,100000);
    await assert.rejects(core.textures.decode({files:[{...meta,sha256:core.textures.sha256(bomb)}]},{'texture:actor.png':bomb},{}),/dimensions differ/);
    await assert.rejects(core.textures.decode({files:[meta]},{'texture:actor.png':bomb},{}),/integrity/);
    await assert.rejects(core.textures.decode({files:[meta]},{'texture:unknown.png':bytes},{}),/integrity/);
    assert.equal(allocations,1,'refusals must precede native image decoding');
});

t.check('raster lifecycle: preview canvases stay bounded and dispose releases textures, renderers and fallback fonts',async()=>{
    let deleted=0,closed=0,destroyed=0;
    const core=loadPlayer().StorymakerCore,fonts={add(){},delete(){deleted++;}};
    const runtime=new core.workerPlayer.Runtime({makeCanvas:(w,h)=>new FakeCanvas(w,h),fonts,FontFace:class{async load(){return this;}}});
    const first=runtime.canvas('main',10,10);
    for(let width=11;width<30;width++)runtime.canvas('main',width,10);
    assert.equal(Object.keys(runtime.canvases).length,9);assert.equal(first.canvas.width,1);
    await runtime.handle({type:'assets',fonts:[{name:'default',family:'Geist',weight:400,data:new ArrayBuffer(2)}]});
    const retained=runtime.canvas('main',29,10);
    runtime.bitmaps.test={close(){closed++;}};runtime.renderers.test={dispose(){destroyed++;}};runtime.textureData.test=new ArrayBuffer(2);
    await runtime.handle({type:'dispose'});
    assert.equal(deleted,1);assert.equal(closed,1);assert.equal(destroyed,1);
    assert.equal(retained.canvas.width,1);assert.equal(retained.canvas.height,1);
    assert.equal(Object.keys(runtime.textureData).length,0);assert.equal(Object.keys(runtime.canvases).length,0);assert.equal(Object.keys(runtime.fontsLoaded).length,0);
});

t.check('raster preparation: decoder readiness does not invoke the frame renderer or a code-scene callback',async()=>{
    const core=loadPlayer().StorymakerCore;let prepared=0,drawn=0;
    core.raster={get:async()=>({}),prepare:async()=>{prepared++;}};
    core.render.makeContext=()=>({});core.renderFrame=()=>{drawn++;};core.codescene.drawFrame=()=>{drawn++;};core.schedule.framePlan=()=>({layers:[]});
    const env={makeCanvas:(w,h)=>new FakeCanvas(w,h)},catalog={schema:'storymaker-catalog/2',entries:[],fonts:[]};
    const player=new core.workerPlayer.Runtime(env);player.catalog=catalog;player.timeline={width:10,height:10,fps:30,schedule:{},scenes:[]};
    await player.handle({type:'prepare_frame',n:4});
    const scene=new core.workerScene.Runtime(env,{});scene.catalog=catalog;scene.opts={catalog,width:10,height:10,fps:30};scene.built={entities:[]};
    await scene.handle({type:'prepare_frame',t:1});assert.equal(prepared,2);assert.equal(drawn,0);
});

t.check('sprite gait: planted heels stay at the same ground point for every body type and either walking direction',()=>{
    const context=loadPlayer({navigator:{userAgent:'Storymaker gait test'}});vm.runInContext(read('ui/js/vendor/storymaker/runtime.js'),context);const core=context.StorymakerCore;
    for(const body of ['average','broad','slim','elder','child'])for(const facing of [1,-1]){
        const catalog=legacyCatalog();catalog.entries.find(e=>e.id==='char.office-woman-1').params.body=body;
        const out=core.compile({format:{aspect:'16:9',fps:30},cast:[{id:'a',character:'char.office-woman-1'}],scenes:[{id:'walk',environment:'env.plain-stage',duration:12,actors:[{cast:'a',at:facing>0?'left':'right'}],beats:[{do:'walk',who:'a',to:facing>0?'right':'left',duration:10}]}]}, {},catalog);
        const sc=out.timeline.scenes[0],entity=sc.entities.find(e=>e.type==='actor'),assets=legacyAssets(),rig=assets.rig;
        const rc=core.render.makeContext(out.timeline,assets,1920,1080),stage=core.render.stageFor(sc,rc),b=core.rig.bodyOf(rig,body),rest=core.rig.boneMatrices(rig,'profile',b,{});
        const first=Math.hypot(rest.lower_leg_r[4]-rest.upper_leg_r[4],rest.lower_leg_r[5]-rest.upper_leg_r[5]);
        const start=core.scene.evalEntity(entity,0),scale=core.rig.scaleFor(rig,1080,stage.depthScale(start.y,start.depth),start.scale,b),stride=(first+205)*0.7*scale;
        const speed=Math.abs(entity.x.at(10)-entity.x.at(0))/10;let planted;
        for(const phase of [0.2,0.3,0.45,0.3,0.2]){
            const time=phase*stride/speed,state=core.scene.evalEntity(entity,time),pose=core.render.rigState(state,time,rc,rig);
            core.rasterRig.adjust(pose,state,entity,time,rc,stage);
            const world=core.rig.boneMatrices(rig,'profile',b,pose.offsets),ankle=core.rasterRig.slotMatrix(world.lower_leg_r,{pivot:[0,205],upright:true});
            const x=state.x+scale*facing*ankle[4],y=state.y+scale*(pose.offsetY+ankle[5]+20);
            if(planted===undefined)planted=x;assert.ok(Math.abs(x-planted)<0.001,`${body}/${facing}: heel slid by ${x-planted}`);
            assert.ok(Math.abs(y-state.y)<0.001,`${body}/${facing}: heel left the floor by ${y-state.y}`);
            assert.ok(Math.abs(ankle[1])<0.000001,'the shoe sole stays horizontal');
        }
        const idle=core.scene.evalEntity(entity,11),pose=core.render.rigState(idle,11,rc,rig);core.rasterRig.adjust(pose,idle,entity,11,rc,stage);assert.equal(pose.offsetY,0);
    }
});
