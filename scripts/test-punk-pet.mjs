import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
import http from 'node:http';
import { dirname, extname, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';

const root=resolve(dirname(fileURLToPath(import.meta.url)),'..'),ui=resolve(root,'ui');
const catalog=JSON.parse(readFileSync(resolve(root,'assets/desktop-pets/catalog.json'),'utf8'));
const pet=process.argv[2]?.replace(/^--pet=/,'')||'punk';
assert.ok(process.argv.length<=3&&Object.hasOwn(catalog,pet),'Unknown pet test argument');
const {sprite_id:spritePetId,id:modelPetId,joints:jointCount,kind,name}=catalog[pet];
const humanoid=kind==='humanoid';
const scratch=resolve(root,'disposable/_'+pet+'-pet');mkdirSync(scratch,{recursive:true});
const model=readFileSync(resolve(ui,'3d/pets',pet,'model.glb'));
const length=model.readUInt32LE(12),doc=JSON.parse(model.subarray(20,20+length));
assert.equal(doc.skins[0].joints.length,jointCount);
const jointSet=new Set(doc.skins[0].joints),parents=new Map();
doc.nodes.forEach((n,i)=>(n.children||[]).forEach(c=>parents.set(c,i)));
assert.equal([...jointSet].filter(i=>!jointSet.has(parents.get(i))).length,1,'All limbs and fingers must share the body root');
const indexByName=new Map(doc.nodes.map((n,i)=>[n.name,i]));
if(humanoid)assert.equal(parents.get(indexByName.get('DEF-thumb.01.L')),indexByName.get('DEF-hand.L'));
else {
    const required=['DEF-spine','DEF-spine.003','DEF-spine.006'];
    if(kind==='dinosaur')required.push('Tail.04');
    if(['tentacle','blob'].includes(kind))required.push('TentacleTip');
    if(['bird','bird-tail'].includes(kind))required.push('Tail.03','DEF-foot.L','DEF-foot.R');
    if(kind==='bird-tail')for(let i=1;i<=7;i++)required.push('Fan.'+String(i).padStart(2,'0'));
    if(kind==='tracked')required.push('Tread.L','Tread.R');
    if(kind==='quadruped')required.push('DEF-root','DEF-hand.L','DEF-hand.R','DEF-foot.L','DEF-foot.R','Tail.03');
    if(kind==='spider')for(const side of ['L','R'])for(let i=2;i<=4;i++)required.push('Leg.'+String(i).padStart(2,'0')+'.tip.'+side);
    for(const name of required)assert.ok(indexByName.has(name),'Missing anatomical joint '+name);
}
const binary=model.subarray(28+length);
if(pet==='punk'){
const eyes=doc.nodes[indexByName.get('PunkPersona_Eyes')];
assert.ok(eyes?.mesh!==undefined,'The model must contain the authored eyes');
assert.equal(parents.get(indexByName.get('PunkPersona_Eyes')),indexByName.get('DEF-spine.006'),'Eyes must follow the animated head');
assert.equal(eyes.skin,undefined,'Rigid head-bound eyes must not introduce another skin');
assert.deepEqual(doc.meshes[eyes.mesh].primitives.map(p=>doc.materials[p.material].name).sort(),['Eye catchlight','Eye iris','Eye pupil','Eye sclera']);
function eyeWidth(material,side){
    const primitive=doc.meshes[eyes.mesh].primitives.find(p=>doc.materials[p.material].name===material);
    const a=doc.accessors[primitive.attributes.POSITION],v=doc.bufferViews[a.bufferView],positions=[];
    assert.equal(a.componentType,5126);assert.equal(a.type,'VEC3');
    for(let i=0;i<a.count;i++){const x=binary.readFloatLE((v.byteOffset||0)+(a.byteOffset||0)+i*(v.byteStride||12));if(x*side>0)positions.push(x);}
    assert.ok(positions.length>0);
    return Math.max(...positions)-Math.min(...positions);
}
for(const side of [-1,1]){const ratio=eyeWidth('Eye iris',side)/eyeWidth('Eye sclera',side);assert.ok(ratio>.58&&ratio<.68,'Reference-sized irises must leave restrained sclera');}
}
for(const node of doc.nodes.filter(n=>n.skin!==undefined))for(const primitive of doc.meshes[node.mesh].primitives){const a=doc.accessors[primitive.attributes.WEIGHTS_0],v=doc.bufferViews[a.bufferView];assert.equal(a.type,'VEC4');assert.equal(a.componentType,5126);for(let i=0;i<a.count;i++){const offset=(v.byteOffset||0)+(a.byteOffset||0)+i*(v.byteStride||16);let sum=0;for(let j=0;j<4;j++)sum+=binary.readFloatLE(offset+j*4);assert.ok(Math.abs(sum-1)<.0001);}}
if(['zombert','snoopy','hana'].includes(pet)||['bird','bird-tail','clip','spider','blob','tracked'].includes(kind)){
    const head=doc.skins[0].joints.indexOf(indexByName.get('DEF-spine.006'));
    const modelHeight=Math.max(...doc.meshes[0].primitives.map(p=>doc.accessors[p.attributes.POSITION].max[1]));
    for(const primitive of doc.meshes[0].primitives){
    const position=doc.accessors[primitive.attributes.POSITION],joints=doc.accessors[primitive.attributes.JOINTS_0],weights=doc.accessors[primitive.attributes.WEIGHTS_0];
    const offset=(a,i,size)=>(doc.bufferViews[a.bufferView].byteOffset||0)+(a.byteOffset||0)+i*(doc.bufferViews[a.bufferView].byteStride||size);
    assert.equal(joints.componentType,5121);
    for(let i=0;i<position.count;i++){
        const x=binary.readFloatLE(offset(position,i,12)),y=binary.readFloatLE(offset(position,i,12)+4);
        let headWeight=0;
        for(let j=0;j<4;j++)if(binary.readUInt8(offset(joints,i,4)+j)===head)headWeight+=binary.readFloatLE(offset(weights,i,16)+j*4);
        const depth=binary.readFloatLE(offset(position,i,12)+8);
        if(['zombert','snoopy'].includes(pet)&&Math.abs(x)>.6&&(pet==='zombert'||(y>.82&&y<1&&Math.abs(depth)<.06)))assert.ok(headWeight<.00001,'Raised fingertips must not be bound to the skull');
        if(pet==='snoopy'&&y>1.2)assert.ok(headWeight>.999,'The upper skull and long ears must not follow the arms');
        if(pet==='hana'&&Math.abs(x)>.65&&y>.8&&y<1.2)assert.ok(headWeight<.00001,'The hair repair must not capture raised hands');
        if(pet==='hana'&&y>1.4)assert.ok(headWeight>.999,'Hair and its supporting skull must follow the head');
        if(pet==='hana'&&y>.75&&y<.9&&Math.abs(x)<.2&&depth>.05)assert.ok(headWeight<.001,'The cyan skirt must remain independent of the hair');
        if(pet==='hana'&&y>.9&&y<1.1&&Math.abs(x)<.2&&depth>.05)assert.ok(headWeight<.1,'The cardigan must remain independent of the hair');
        if(['bird','clip'].includes(kind)&&y>position.max[1]*.62)assert.ok(headWeight>.999,'Eyes and their supporting head surface must move rigidly together');
        if(['bird-tail','spider'].includes(kind)&&y>position.max[1]*.62&&depth>(position.max[2]-position.min[2])*.1)assert.ok(headWeight>.999,'The face must stay rigid while the rear anatomy moves');
        if(kind==='blob'&&y>position.max[1]*.4&&y<position.max[1]*.75&&Math.abs(x)<(position.max[0]-position.min[0])*.25&&depth>0)assert.ok(headWeight>.999,'The slime face must remain together while the base wobbles');
        if(kind==='tracked'&&y>modelHeight*.8)assert.ok(headWeight>.999,'Camera lenses and their supporting housing must remain rigid');
    }
    }
}
if(pet==='wall-e')assert.ok(doc.meshes[0].primitives[0].attributes.COLOR_0!==undefined,'The mechanical reconstruction must retain its vertex colors');
for(const image of doc.images||[]){const view=doc.bufferViews[image.bufferView],bytes=binary.subarray(view.byteOffset,view.byteOffset+view.byteLength);assert.equal(bytes.toString('ascii',1,4),'PNG');assert.ok(bytes.readUInt32BE(16)<=1024&&bytes.readUInt32BE(20)<=1024,'Runtime textures must actually be resized');}
const pack=JSON.parse(readFileSync(resolve(ui,'3d/pets',pet,'motions.json'),'utf8'));
assert.equal(pack.clips.length,30);
assert.equal(pack.model_sha256,createHash('sha256').update(model).digest('hex'));
if(humanoid){
    for(const item of pack.clips){const source=readFileSync(resolve(root,'assets/desktop-pets/mesh2motion/animations',item.id+'.glb'));assert.equal(item.source_sha256,createHash('sha256').update(source).digest('hex'));}
}else{
    assert.equal(pack.procedural,true);assert.equal(pack.license,'GPL-3.0-only');
    assert.equal(pack.source_sha256,createHash('sha256').update(readFileSync(resolve(root,'scripts/build-creature-pet-motions.mjs'))).update(readFileSync(resolve(root,'assets/desktop-pets',pet,'rig.json'))).digest('hex'));
}
for(const {clip} of pack.clips){
    assert.ok(clip.tracks.every(t=>t.values.every(Number.isFinite)));
    assert.ok(clip.tracks.some(t=>{const size=t.type==='quaternion'?4:3;return t.values.some((v,i)=>i>=size&&Math.abs(v-t.values[i%size])>.0001);}), 'Every clip must animate');
}
const browser=[process.env.CHROME_BIN,process.env.PROGRAMFILES&&resolve(process.env.PROGRAMFILES,'Google/Chrome/Application/chrome.exe'),process.env['PROGRAMFILES(X86)']&&resolve(process.env['PROGRAMFILES(X86)'],'Microsoft/Edge/Application/msedge.exe'),'/usr/bin/google-chrome','/usr/bin/chromium'].filter(Boolean).find(existsSync);
assert.ok(browser,'Chrome/Edge is required for the real WebGL pet test');
if(pet==='hana'){
    const THREE=await import('three');const {loadSkeleton,reset}=await import('./pet-skeleton.mjs');
    const target=await loadSkeleton(resolve(ui,'3d/pets',pet,'model.glb'));let surface;
    target.gltf.scene.traverse(o=>{if(o.isSkinnedMesh)surface=o;});
    const head=target.skeleton.bones.find(b=>b.name.replace(/[^a-z0-9]/gi,'').toLowerCase()==='defspine006');
    const geometry=surface.geometry,index=geometry.index,point=new THREE.Vector3();
    const skin=()=>{target.gltf.scene.updateMatrixWorld(true);surface.skeleton.update();const positions=new Float32Array(geometry.attributes.position.count*3);for(let i=0;i<geometry.attributes.position.count;i++){point.fromBufferAttribute(geometry.attributes.position,i);surface.applyBoneTransform(i,point).applyMatrix4(surface.matrixWorld);point.toArray(positions,i*3);}return positions;};
    reset(target);const rest=skin(),edgeChecks=[];
    const distance=(positions,a,b)=>Math.hypot(positions[a*3]-positions[b*3],positions[a*3+1]-positions[b*3+1],positions[a*3+2]-positions[b*3+2]);
    for(const id of pack.clips.map(item=>item.id))for(const fraction of [.25,.6,.9]){
        reset(target);const clip=THREE.AnimationClip.parse(pack.clips.find(item=>item.id===id).clip),mixer=new THREE.AnimationMixer(target.gltf.scene);
        mixer.clipAction(clip).play();mixer.setTime(clip.duration*fraction);
        if(!['sleeping','freefall'].includes(id))head.quaternion.multiply(new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0,1,0),.38));
        const positions=skin(),examples=[],badTriangles=new Set();let spikes=0,maxStretch=0;
        for(let i=0;i<index.count;i+=3)for(const [a,b] of [[index.getX(i),index.getX(i+1)],[index.getX(i+1),index.getX(i+2)],[index.getX(i+2),index.getX(i)]]){
            const before=distance(rest,a,b),after=distance(positions,a,b);
            if(before<.025&&after>.12&&after>before*12){
                spikes++;maxStretch=Math.max(maxStretch,after);
                badTriangles.add(i/3);
                if(examples.length<3)examples.push([a,b].map(vertex=>({position:Array.from(rest.subarray(vertex*3,vertex*3+3)),influences:['getX','getY','getZ','getW'].map(method=>({bone:surface.skeleton.bones[geometry.attributes.skinIndex[method](vertex)].name,weight:geometry.attributes.skinWeight[method](vertex)}))})));
            }
        }
        edgeChecks.push({id,fraction,spikes,maxStretch,badTriangles:[...badTriangles],examples});
        mixer.stopAllAction();mixer.uncacheRoot(target.gltf.scene);
    }
    writeFileSync(resolve(scratch,'hair-edge-check.json'),JSON.stringify({model_sha256:pack.model_sha256,samples:edgeChecks},null,2)+'\n');
    assert.ok(edgeChecks.every(check=>check.spikes===0),'Short surface edges must not turn into hair/clothing spikes; see hair-edge-check.json');
}
const labels=JSON.parse(readFileSync(resolve(ui,'lang/desktop/de.json'),'utf8'));
const html=`<!doctype html><html lang="de"><head><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><style>body{margin:0;background:#252331;color:white;font:16px system-ui}.vd-taskbar{position:fixed;bottom:0;width:100%;height:48px;background:#131219}#title{padding:20px}button{margin:8px}#test-window{position:absolute;inset:0;background:#252331;pointer-events:auto}</style></head><body class="desktop-body" data-animations="true" data-theme="standard" data-scheme="dark"><main class="vd-shell"><section class="vd-workspace" id="vd-workspace"><div class="vd-window-layer" id="test-window-layer"><div id="test-window"></div></div></section></main><div id="title">AuraGo - ${name} 3D</div><div class="vd-taskbar"></div><script>
window.I18N=${JSON.stringify(labels)};window.BUILD_VERSION='pet-test';
window.state={bootstrap:{pets:[{id:'${modelPetId}',display_name:'${pet} 3D',subcategory:'3d',spritesheet:'spritesheet.webp'},{id:'${spritePetId}',display_name:'${pet}',spritesheet:'spritesheet.webp'}],active_pet_id:'${modelPetId}',settings:{'pet.enabled':'true','pet.scale':'2','pet.position_x':'350','pet.position_y':'240','pet.gravity':'false'}}};
window.settingValue=k=>state.bootstrap.settings[k];window.t=k=>I18N[k]||k;window.shellLayout=()=> 'taskbar';window.notifications=[];window.showDesktopNotification=x=>notifications.push(x);window.showContextMenu=(x,y,items)=>window.menuItems=items;window.openApp=()=>{};
window.api=async(url,options)=>{if(options&&options.body){const {key,value}=JSON.parse(options.body);state.bootstrap.settings[key]=value;}return {pets:state.bootstrap.pets,active_pet_id:state.bootstrap.active_pet_id,settings:state.bootstrap.settings};};
window.until=async(predicate,message)=>{const end=Date.now()+30000;while(Date.now()<end){if(predicate())return;await new Promise(r=>setTimeout(r,30));}throw Error('Timed out: '+message+' '+JSON.stringify({notifications,hidden:document.hidden,model:window.PetRuntime?.inspect3D()}));};
window.changePet=id=>{PetRuntime.syncBootstrap({active_pet_id:id});PetRuntime.load();};
</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/pet-model-catalog.js"></script><script src="/js/desktop/core/pet-runtime.js"></script></body></html>`;
let delay=0,fail=false;
const server=http.createServer((request,response)=>{
    try{const pathname=new URL(request.url,'http://localhost').pathname;
        if(pathname==='/'){response.setHeader('Content-Type','text/html; charset=utf-8');response.end(html);return;}
        if(pathname.includes('/files/desktop/Pets/')){response.setHeader('Content-Type','image/webp');response.end(readFileSync(resolve(root,'internal/desktop/pets_assets',spritePetId,'spritesheet.webp')));return;}
        const file=resolve(ui,'.'+decodeURIComponent(pathname));if(!file.startsWith(ui+sep))throw Error('path');
        const send=()=>{if(fail&&pathname.endsWith('motions.json')){response.statusCode=503;response.end();return;}response.setHeader('Content-Type',{'.json':'application/json','.js':'text/javascript','.css':'text/css','.png':'image/png'}[extname(file)]||'application/octet-stream');response.end(readFileSync(file));};
        if(delay&&pathname.endsWith('motions.json'))setTimeout(send,delay);else send();
    }catch{response.statusCode=404;response.end();}
});
await new Promise(r=>server.listen(0,'127.0.0.1',r));
const profile=mkdtempSync(resolve(scratch,'profile-'));
const child=spawn(browser,['--headless=new','--no-sandbox','--no-first-run','--use-gl=angle','--use-angle=swiftshader','--enable-unsafe-swiftshader','--remote-debugging-port=0','--user-data-dir='+profile,'about:blank'],{windowsHide:true,stdio:['ignore','ignore','pipe']});
let socket;
try{
    const debug=await new Promise((res,rej)=>{let text='';const timer=setTimeout(()=>rej(Error('Browser start timed out')),20000);child.stderr.on('data',v=>{text+=v;const m=text.match(/DevTools listening on (ws:\/\/\S+)/);if(m){clearTimeout(timer);res(m[1]);}});child.once('error',rej);});
    const target=await fetch('http://'+new URL(debug).host+'/json/new?about:blank',{method:'PUT'}).then(r=>r.json());
    socket=new WebSocket(target.webSocketDebuggerUrl);
    await new Promise((res,rej)=>{socket.addEventListener('open',res,{once:true});socket.addEventListener('error',rej,{once:true});});
    let id=0,onLoad=null;const pending=new Map(),errors=[];
    socket.addEventListener('message',event=>{const m=JSON.parse(event.data);if(m.method==='Runtime.exceptionThrown')errors.push(m.params);if(m.method==='Page.loadEventFired')onLoad?.();const p=pending.get(m.id);if(p){pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result);}});
    const call=(method,params={})=>new Promise((res,rej)=>{const next=++id;pending.set(next,{resolve:res,reject:rej});socket.send(JSON.stringify({id:next,method,params}));});
    const evaluate=async expression=>{const r=await call('Runtime.evaluate',{expression,awaitPromise:true,returnByValue:true});assert.equal(r.exceptionDetails,undefined,JSON.stringify(r.exceptionDetails));return r.result.value;};
    const shot=async name=>{const clip=await evaluate(`(()=>{const r=document.querySelector('.vd-pet-layer').getBoundingClientRect();const x=Math.max(0,r.left-16),y=Math.max(0,r.top-16);return {x,y,width:Math.min(innerWidth-x,r.width+32),height:Math.min(innerHeight-y,r.height+32),scale:1};})()`);const r=await call('Page.captureScreenshot',{format:'png',clip});writeFileSync(resolve(scratch,name+'.png'),Buffer.from(r.data,'base64'));};
    await call('Runtime.enable');await call('Page.enable');await call('Emulation.setDeviceMetricsOverride',{width:1280,height:850,deviceScaleFactor:1,mobile:false});
    await call('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-motion',value:'no-preference'}]});
    const loaded=new Promise(r=>onLoad=r);
    await call('Page.navigate',{url:'http://127.0.0.1:'+server.address().port});await loaded;
    await call('Page.bringToFront');
    const initial=await evaluate(`(async()=>{await until(()=>window.PetRuntime&&PetRuntime.inspect3D()?.frames>3,'3D load');return PetRuntime.inspect3D();})()`);
    assert.equal(initial.bones,jointCount);assert.equal(initial.clips,30);assert.ok(initial.triangles>40000&&initial.triangles<=60000);
    const checkLayering=async()=>{
        for(const theme of ['standard','fruity'])for(const onTop of [false,true,false]){
            await evaluate(`document.body.dataset.theme=${JSON.stringify(theme)};PetRuntime.syncBootstrap({settings:{'pet.always_on_top':'${onTop}'}});PetRuntime.load();`);
            const hit=await evaluate(`(()=>{const r=document.querySelector('.vd-pet-sprite').getBoundingClientRect();const el=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return {pet:!!el?.closest('.vd-pet-layer'),window:el?.id==='test-window'};})()`);
            assert.deepEqual(hit,{pet:onTop,window:!onTop},'Pet foreground setting must control real window occlusion in '+theme);
        }
        await evaluate(`document.getElementById('test-window-layer').hidden=true;document.body.dataset.theme='standard';`);
        assert.equal(await evaluate(`(()=>{const r=document.querySelector('.vd-pet-sprite').getBoundingClientRect();return !!document.elementFromPoint(r.left+r.width/2,r.top+r.height/2)?.closest('.vd-pet-layer');})()`),true,'Unpinned pet must remain clickable on the desktop');
    };
    await checkLayering();
    await shot('idle');
    await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:10,y:150});
    await evaluate(`until(()=>PetRuntime.inspect3D().gaze[0]<-.2,'left pointer gaze')`);const left=await evaluate('PetRuntime.inspect3D().gaze[0]');
    await call('Input.dispatchMouseEvent',{type:'mouseMoved',x:1250,y:150});
    await evaluate(`until(()=>PetRuntime.inspect3D().gaze[0]>.2,'right pointer gaze')`);const right=await evaluate('PetRuntime.inspect3D().gaze[0]');assert.ok(right-left>.3,'Head must follow live pointer position');
    for(const [reaction,file] of [['waving','wave'],['thinking','thinking'],['celebrating','celebrate'],['editing','working']]){
        await evaluate(`PetRuntime.setReaction(${JSON.stringify(reaction)})`);await new Promise(r=>setTimeout(r,750));const pose=await evaluate('PetRuntime.inspect3D()');assert.ok(pose.gazeAngle<.5,'Additive gaze must never accumulate on constant motion tracks');await shot(file);
    }
    await evaluate(`document.querySelector('.vd-pet-layer').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:500,clientY:300}));menuItems.find(x=>x.label==='Tanzen').action();`);
    assert.equal(await evaluate('PetRuntime.inspect3D().state'),'dancing');await new Promise(r=>setTimeout(r,700));await shot('dance');
    await evaluate(`menuItems.find(x=>x.label==='Hinsetzen').action()`);await new Promise(r=>setTimeout(r,700));const seated=await evaluate('PetRuntime.inspect3D()');
    assert.ok(seated.gazeAngle<.5&&[...seated.headWorld,...seated.pelvisWorld,...seated.skinTip].every(Number.isFinite),'Rest pose must keep bounded head tracking and finite skinning');
    if(humanoid)assert.ok(seated.headWorld[1]>seated.pelvisWorld[1]+.3&&seated.skinTip[1]>seated.headWorld[1]+.15,'Seated humanoid must keep an upright body');
    else assert.ok(Math.hypot(...seated.headWorld.map((v,i)=>v-seated.pelvisWorld[i]))>.1&&seated.skinTip[1]>.5,'Creature rest pose must retain its body volume');
    await shot('sit');
    await evaluate(`menuItems.find(x=>x.label==='Aufstehen').action();PetRuntime.say('Hallo! Ich bin Dein neues 3D-Pet.','info')`);
    assert.equal(await evaluate(`document.querySelector('.vd-pet-bubble').hidden`),false);
    const rect=await evaluate(`(()=>{const r=document.querySelector('.vd-pet-sprite').getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height*.55,left:document.querySelector('.vd-pet-layer').offsetLeft,top:document.querySelector('.vd-pet-layer').offsetTop};})()`);
    const mouse=(type,x,y,down=false)=>call('Input.dispatchMouseEvent',{type,x,y,button:type==='mouseMoved'?'none':'left',buttons:down?1:0,clickCount:type==='mouseMoved'?0:1});
    const inputChecks={};
    await mouse('mousePressed',rect.x,rect.y,true);
    inputChecks.clickDoesNotStartDrag=await evaluate(`!document.querySelector('.vd-pet-layer').classList.contains('dragging')&&PetRuntime.inspect3D().state!=='dragging'`);
    await mouse('mouseMoved',rect.x+2,rect.y+2,true);await mouse('mouseReleased',rect.x+2,rect.y+2);
    assert.deepEqual(await evaluate(`({left:document.querySelector('.vd-pet-layer').offsetLeft,top:document.querySelector('.vd-pet-layer').offsetTop})`),{left:rect.left,top:rect.top},'A greeting click must not move the pet');
    await mouse('mousePressed',rect.x,rect.y,true);await mouse('mouseMoved',rect.x+90,rect.y-120,true);
    await evaluate(`until(()=>PetRuntime.inspect3D().state==='dragging','pickup animation')`);await shot('drag');
    await mouse('mouseReleased',rect.x+90,rect.y-120);
    await evaluate(`until(()=>PetRuntime.inspect3D().state==='idle','drop settles')`);
    const beginDrag=async reason=>{
        await evaluate(`document.querySelector('.vd-pet-layer').getAnimations().forEach(a=>a.finish())`);
        const point=await evaluate(`(()=>{const layer=document.querySelector('.vd-pet-layer');layer.addEventListener('pointerdown',e=>window.gesturePointer=e.pointerId,{once:true});const r=document.querySelector('.vd-pet-sprite').getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height*.3};})()`);
        // Move onto the newly positioned pet before pressing, as a real pointer does.
        await mouse('mouseMoved',point.x,point.y);
        await mouse('mousePressed',point.x,point.y,true);
        assert.equal(await evaluate(`document.querySelector('.vd-pet-layer').hasPointerCapture(gesturePointer)`),true,'Pointer must reach the pet for '+reason);
        await mouse('mouseMoved',point.x+40,point.y-40,true);
        await evaluate(`until(()=>PetRuntime.inspect3D().state==='dragging','drag starts '+${JSON.stringify(reason)})`);return point;
    };
    for(const reason of ['captureLoss','outsideRelease','blur','missingRelease']){
        // Let the preceding release publish its position before measuring the next press.
        await evaluate(`new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`);
        const point=await beginDrag(reason);
        if(reason==='captureLoss')await evaluate(`document.querySelector('.vd-pet-layer').dispatchEvent(new PointerEvent('lostpointercapture',{pointerId:gesturePointer,bubbles:true}))`);
        if(reason==='outsideRelease'){await evaluate(`document.querySelector('.vd-pet-layer').releasePointerCapture(gesturePointer)`);await mouse('mouseReleased',20,20);}
        if(reason==='blur')await evaluate(`window.dispatchEvent(new Event('blur'))`);
        if(reason==='missingRelease')await mouse('mouseMoved',point.x+42,point.y-42);
        await evaluate(`new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))`);
        inputChecks[reason]=await evaluate(`PetRuntime.inspect3D().state!=='dragging'&&!document.querySelector('.vd-pet-layer').classList.contains('dragging')`);
        await mouse('mouseReleased',point.x+42,point.y-42);
        await evaluate(`until(()=>PetRuntime.inspect3D().state==='idle','cancelled drag settles')`);
    }
    await evaluate(`document.querySelector('.vd-pet-layer').style.top='40px';PetRuntime.syncBootstrap({settings:{'pet.gravity':'true','pet.position_y':'0'}});PetRuntime.load()`);
    await evaluate(`until(()=>Math.abs(document.querySelector('.vd-pet-layer').offsetTop-(document.querySelector('.vd-taskbar').getBoundingClientRect().top-416))<1&&PetRuntime.inspect3D().state==='landing','dock landing')`);
    assert.equal(await evaluate('PetRuntime.inspect3D().state'),'landing','A real fall must still play its landing animation');
    await evaluate(`until(()=>PetRuntime.inspect3D().state==='idle','landing finishes')`);
    const refreshed=await evaluate(`(async()=>{const states=[];for(let i=0;i<5;i++){PetRuntime.load();await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));states.push(PetRuntime.inspect3D().state);}document.querySelector('.vd-taskbar').style.width='95%';await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));states.push(PetRuntime.inspect3D().state);return states;})()`);
    assert.deepEqual(refreshed,Array(6).fill('idle'),'Window/bootstrap refreshes and dock width changes must not trigger a landing while grounded');
    await shot('dock');
    const background=await fetch('http://'+new URL(debug).host+'/json/new?about:blank',{method:'PUT'}).then(r=>r.json());
    await fetch('http://'+new URL(debug).host+'/json/activate/'+background.id);
    await new Promise(r=>setTimeout(r,150));assert.equal(await evaluate('document.hidden'),true);
    const hiddenFrames=await evaluate('PetRuntime.inspect3D().frames');await new Promise(r=>setTimeout(r,200));assert.equal(await evaluate('PetRuntime.inspect3D().frames'),hiddenFrames,'Hidden tabs must not render');
    await call('Page.bringToFront');await fetch('http://'+new URL(debug).host+'/json/close/'+background.id);
    await evaluate(`until(()=>PetRuntime.inspect3D().frames>${hiddenFrames},'visible tab resumes')`);
    await call('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-motion',value:'reduce'}]});
    await evaluate(`(async()=>{await until(()=>PetRuntime.inspect3D().reducedMotion,'reduced motion media change');await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));})()`);const reduced=await evaluate('PetRuntime.inspect3D().frames');await new Promise(r=>setTimeout(r,200));assert.equal(await evaluate('PetRuntime.inspect3D().frames'),reduced,'Reduced motion must stop continuous rendering');
    await call('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-motion',value:'no-preference'}]});
    const otherPet=pet==='punk'?'aurago-zombie-3d':'aurago-punk-3d',otherJoints=pet==='punk'?53:71;
    await evaluate(`(async()=>{state.bootstrap.pets.push({id:'${otherPet}',subcategory:'3d',spritesheet:'spritesheet.webp'});changePet('${otherPet}');await until(()=>PetRuntime.inspect3D()?.bones===${otherJoints},'switch between 3D pets');})()`);
    delay=300;
    await evaluate(`(async()=>{changePet('${modelPetId}');await new Promise(r=>requestAnimationFrame(r));changePet('${otherPet}');await until(()=>PetRuntime.inspect3D()?.bones===${otherJoints},'cancel stale 3D model');})()`);
    delay=0;
    await evaluate(`(async()=>{changePet('${modelPetId}');await until(()=>PetRuntime.inspect3D()?.bones===${jointCount},'restore selected 3D pet');let rejected=false;try{await AuraPunkPet3D.create(document.createElement('div'),{petId:'constructor'});}catch{rejected=true;}if(!rejected)throw Error('Unknown model id accepted');})()`);
    await evaluate(`window.oldCanvas=document.querySelector('.vd-pet-sprite canvas');changePet('${spritePetId}');`);
    assert.equal(await evaluate('PetRuntime.inspect3D()'),null);assert.equal(await evaluate('document.querySelectorAll(".vd-pet-sprite canvas").length'),0);assert.equal(await evaluate('oldCanvas.getContext("webgl2").isContextLost()'),true);
    await evaluate(`document.getElementById('test-window-layer').hidden=false;`);
    await checkLayering();
    delay=400;
    await evaluate(`changePet('${modelPetId}');`);
    await evaluate(`new Promise(r=>requestAnimationFrame(r))`);
    inputChecks.loadingHidesSprite=await evaluate(`getComputedStyle(document.querySelector('.vd-pet-sprite')).backgroundImage==='none'&&getComputedStyle(document.querySelector('.vd-pet-sprite')).pointerEvents==='none'`);
    await evaluate(`changePet('${spritePetId}');`);await new Promise(r=>setTimeout(r,600));assert.equal(await evaluate('document.querySelectorAll(".vd-pet-sprite canvas").length'),0,'Late load must not revive a removed pet');
    delay=0;fail=true;
    await evaluate(`changePet('${modelPetId}')`);await evaluate(`until(()=>notifications.length===1,'sprite fallback')`);
    assert.equal(await evaluate('document.querySelectorAll(".vd-pet-sprite canvas").length'),0);
    assert.equal(await evaluate(`getComputedStyle(document.querySelector('.vd-pet-sprite')).backgroundImage!=='none'&&getComputedStyle(document.querySelector('.vd-pet-sprite')).pointerEvents==='auto'`),true,'The sprite fallback must remain visible and interactive after failure');
    fail=false;await evaluate(`PetRuntime.load();`);await evaluate(`until(()=>PetRuntime.inspect3D()?.frames>2,'retry after failure')`);
    await evaluate(`document.querySelector('.vd-pet-sprite canvas').getContext('webgl2').getExtension('WEBGL_lose_context').loseContext()`);
    await evaluate(`until(()=>notifications.length===2&&!PetRuntime.inspect3D(),'context-loss fallback')`);
    const posters=await evaluate(`(async()=>{
        await new Promise((resolve,reject)=>{const s=document.createElement('script');s.src='/js/desktop/apps/pet-picker.js';s.onload=resolve;s.onerror=reject;document.head.appendChild(s);});
        const host=document.createElement('section');host.hidden=true;document.body.appendChild(host);
        for(const item of ${JSON.stringify(Object.values(catalog))})if(!state.bootstrap.pets.some(p=>p.id===item.id))state.bootstrap.pets.push({id:item.id,subcategory:'3d',spritesheet:'spritesheet.webp'});
        state.bootstrap.pets.push({id:'constructor',subcategory:'3d',spritesheet:'spritesheet.webp'});
        PetPickerApp.render(host,'pet-test',{api:async()=>state.bootstrap,t:window.t});
        await until(()=>host.querySelector('[data-id="constructor"]'),'picker catalog');
        const result=${JSON.stringify([...Object.values(catalog).map(p=>p.id),'constructor'])}.map(id=>{const thumb=host.querySelector('[data-id="'+id+'"] .vd-pet-picker-thumb');return {id,model:thumb.classList.contains('is-model'),image:thumb.style.backgroundImage};});
        host.remove();return result;
    })()`);
    for(const [folder,item] of Object.entries(catalog)){const poster=posters.find(p=>p.id===item.id);assert.ok(poster.model&&poster.image.includes('/3d/pets/'+folder+'/thumbnail.png'));}
    assert.ok(!posters.at(-1).model&&posters.at(-1).image.includes('/files/desktop/Pets/constructor/'));
    assert.deepEqual(errors,[]);
    assert.deepEqual(inputChecks,{clickDoesNotStartDrag:true,captureLoss:true,outsideRelease:true,blur:true,missingRelease:true,loadingHidesSprite:true});
    console.log(JSON.stringify({passed:true,initial,inputChecks,gazeRange:right-left,checks:['actual skin render','normalized weights','connected anatomical skeleton',humanoid?'30 pinned Mesh2Motion clips':'30 original creature motions','3D and sprite foreground toggles in Standard and Fruity','desktop pet hit testing','pointer gaze without drift','agent reactions','manual gestures','greeting without position jump','pickup and drop','real fall and landing','no landing on grounded refresh or dock width change','speech bubbles','hidden-tab pause and resume','reduced motion','GPU disposal','late-load cancellation','asset failure fallback','context loss fallback']},null,2));
}finally{
    socket?.close();
    if(child.exitCode===null){if(process.platform==='win32'){const stop=spawn('taskkill',['/PID',String(child.pid),'/T','/F'],{windowsHide:true,stdio:'ignore'});await new Promise(r=>stop.once('exit',r));}else child.kill();}
    server.closeAllConnections();await new Promise(r=>server.close(r));if(child.exitCode===null)await new Promise(r=>child.once('exit',r));
    assert.ok(profile.startsWith(scratch+sep));try{rmSync(profile,{recursive:true,force:true,maxRetries:5,retryDelay:200});}catch(error){if(error.code!=='EPERM'&&error.code!=='EBUSY')throw error;}
}
