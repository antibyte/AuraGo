import assert from 'node:assert/strict';
import * as T from 'three';
import { loadSkeleton } from '../pet-skeleton.mjs';
import { loadPlayer, readJSON, read, root } from '../storymaker-test-support.mjs';
import { FACE_NAMES, FACE_LANDMARKS } from '../../assets/storymaker/lib/model-face.mjs';
import { t } from '../storymaker-test-context.mjs';
import fs from 'node:fs';
import path from 'node:path';

const buffer=bytes=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength);
t.check('3D characters: bounded self-contained GLBs, aliases and independent binary worker buffers',()=>{
    const core=loadPlayer({TextDecoder}).StorymakerCore,catalog=readJSON('ui/img/storymaker/catalog.json'),idx=core.scene.indexCatalog(catalog);
    for(const id of ['hoodie','professor','punk']){
        const entry=idx.byId['char.'+id],bytes=fs.readFileSync(path.join(root,'ui/img/storymaker/models/'+id+'.glb'));
        const model=core.models.glb(buffer(bytes));assert.ok(model.json.meshes.length>1);
        for(const alias of entry.aliases)assert.equal(idx.byId[alias],entry);
        const copy=buffer(bytes);new Uint8Array(copy)[0]=0;assert.throws(()=>core.models.glb(copy),/header/);assert.equal(bytes[0],0x67);
        const truncated=buffer(bytes.subarray(0,bytes.length-1));assert.throws(()=>core.models.glb(truncated),/header/);
        const bad=bytes.slice();bad.writeUInt32LE(0xffffffff,12);assert.throws(()=>core.models.glb(buffer(bad)),/header/);
    }
    assert.equal(catalog.entries.filter(e=>e.kind==='character').length,3);
    assert.ok(!catalog.files.some(f=>f.path.startsWith('textures/characters/')));
});

t.check('3D articulation: every pet has all poses and facial shapes; skull and seeks remain stable',async()=>{
    const core=loadPlayer({StorymakerVendor:{THREE:T},TextDecoder}).StorymakerCore;
    for(const id of ['hoodie','professor','punk']){
        const {gltf}=await loadSkeleton(path.join(root,'assets/storymaker/models/'+id+'.glb'));
        const pack=readJSON('assets/storymaker/models/'+id+'-motions.json'),rig=core.modelRig.create(gltf.scene,pack);
        const state={pose:{a:'stand',b:'stand',u:1},walkTime:null,phase:0,expression:'neutral',facing:1};
        const presentation={view:'front',mouth:'rest'};
        const snapshot=()=>JSON.stringify([...rig.objects.values()].map(o=>[o.name,o.position.toArray(),o.quaternion.toArray(),o.morphTargetInfluences]));
        const floor=core.modelRig.apply(rig,state,.7,presentation),first=snapshot();
        const head=rig.bones.get('DEF-spine006').matrixWorld.toArray();
        for(const mouth of ['ai','e','o','u','mbp','fv']){
            core.modelRig.apply(rig,state,.7,{...presentation,mouth});
            assert.deepEqual(rig.bones.get('DEF-spine006').matrixWorld.toArray(),head,id+'/'+mouth+' rigid skull');
            assert.ok(rig.meshes.some(m=>m.morphTargetInfluences[m.morphTargetDictionary[mouth]]===1));
            const cavity=rig.meshes.find(m=>m.name==='Storymaker_Mouth_Cavity'),lips=rig.meshes.find(m=>m.name==='Storymaker_Mouth_Lips');
            assert.ok(cavity&&lips&&rig.meshes.some(m=>m.name==='Storymaker_Mouth_Teeth')&&rig.meshes.some(m=>m.name==='Storymaker_Mouth_Tongue'));
            const patch=rig.meshes.find(m=>!m.geometry.index&&!m.name.startsWith('Storymaker_'));
            assert.ok(patch,id+' has a cut face patch');
            const source=patch.geometry.attributes.position,edge=lips.geometry.attributes.position;
            const key=(a,i)=>[a.getX(i),a.getY(i),a.getZ(i)].map(n=>n.toFixed(6)).join(',');
            const vertices=new Map(Array.from({length:source.count},(_,i)=>[key(source,i),i]));
            for(let i=0;i<edge.count/3;i++){
                const j=vertices.get(key(edge,i));assert.notEqual(j,undefined,id+' attached lip boundary');
                const x=lips.getVertexPosition(i,new T.Vector3()).applyMatrix4(lips.matrixWorld),y=patch.getVertexPosition(j,new T.Vector3()).applyMatrix4(patch.matrixWorld);
                assert.ok(x.distanceTo(y)<.00001,id+'/'+mouth+' lip has no skin seam');
            }
            if(['ai','e','o','u'].includes(mouth)){
                const last=cavity.geometry.attributes.position.count-1,center=cavity.getVertexPosition(last,new T.Vector3()).applyMatrix4(cavity.matrixWorld);
                const normal=cavity.applyBoneTransform(last,new T.Vector3(0,0,1)).sub(cavity.applyBoneTransform(last,new T.Vector3())).transformDirection(cavity.matrixWorld);
                const face=rig.meshes.filter(m=>!m.name.startsWith('Storymaker_Mouth_'));
                face.forEach(m=>{if(m.isSkinnedMesh)m.computeBoundingSphere();else m.geometry.computeBoundingSphere();});
                const rim=(cavity.geometry.attributes.position.count-1)/2;
                for(let sample=0;sample<=12;sample++){
                    const inside=sample===12?center:center.clone().lerp(cavity.getVertexPosition(Math.floor(sample*rim/12),new T.Vector3()).applyMatrix4(cavity.matrixWorld),.65);
                    const hit=new T.Raycaster(inside.clone().add(normal),normal.clone().negate()).intersectObjects(face,false)[0];
                    assert.ok(!hit||hit.distance>1,id+'/'+mouth+'/'+sample+' opening must not intersect the face');
                }
            }
        }
        const block={id:'spoken:n',kind:'narration',speaker:'actor',duration_ms:1000,spans:[[0,1000]],speech_ok:true,levels:[150,210],pieces:[{hash:'a'.repeat(64),ext:'wav',at_ms:0,in_ms:0,out_ms:1000,chars:10}]};
        const compiled=core.compile({cast:[{id:'actor',character:'char.'+id}],scenes:[{id:'spoken',speaker:'actor',narration:'Hallo Welt',actors:[{cast:'actor',at:'center'}]}]},{narration:{spoken:{blocks:[block]}}},readJSON('ui/img/storymaker/catalog.json'));
        const actor=compiled.timeline.scenes[0].entities.find(e=>e.type==='actor'),speaking=core.scene.evalEntity(actor,.5),shapes=Array.from(core.lipsync.track(block));
        core.modelRig.apply(rig,speaking,.5,{...presentation,mouth:speaking.mouth},actor);
        for(const mesh of rig.meshes)for(const name of shapes)assert.equal(mesh.morphTargetInfluences[mesh.morphTargetDictionary[name]],.5,id+' interpolates compiled speech shapes');
        const quiet=core.scene.evalEntity(actor,1.1);core.modelRig.apply(rig,quiet,1.1,presentation,actor);
        for(const mesh of rig.meshes)for(const name of shapes)assert.equal(mesh.morphTargetInfluences[mesh.morphTargetDictionary[name]],0,id+' speech ends at rest');
        const chatter={id:'chatter:n',duration_ms:2000,levels:Array.from({length:60},(_,i)=>i%2?210:150)};
        const speech={speech:{active:time=>time>=0&&time<2?{t0:0,t1:2,data:{block:chatter}}:null}};
        let previous=null;
        for(let frame=20;frame<40;frame++){
            core.modelRig.apply(rig,state,frame/30,presentation,speech);
            const mesh=rig.meshes[0],weights=['ai','e','o','u','mbp','fv'].map(name=>mesh.morphTargetInfluences[mesh.morphTargetDictionary[name]]);
            if(previous)assert.ok(Math.max(...weights.map((n,i)=>Math.abs(n-previous[i])))<.35,id+' suppresses frame-rate viseme chatter');
            previous=weights;
        }
        core.modelRig.apply(rig,state,1.999,presentation,speech);
        const quietMesh=rig.meshes[0];assert.ok(['ai','e','o','u','mbp','fv'].reduce((sum,name)=>sum+quietMesh.morphTargetInfluences[quietMesh.morphTargetDictionary[name]],0)<.01,id+' closes before the speech boundary');
        for(const pose of Object.keys(pack.poses))assert.ok(Number.isFinite(core.modelRig.apply(rig,{...state,pose:{a:'stand',b:pose,u:1}},.7,presentation)),id+'/'+pose);
        const hand=rig.bones.get('DEF-handL'),thumb=rig.bones.get('DEF-thumb03L'),middle=rig.bones.get('DEF-f_middle03L');
        core.modelRig.apply(rig,{...state,pose:{a:'thumbs-up',b:'thumbs-up',u:1}},.7,presentation);
        const up=thumb.getWorldPosition(new T.Vector3()).sub(hand.getWorldPosition(new T.Vector3())).normalize();
        assert.ok(up.y>.98,id+' thumb points up');
        const closed=middle.getWorldPosition(new T.Vector3()).distanceTo(hand.getWorldPosition(new T.Vector3()));
        core.modelRig.apply(rig,{...state,pose:{a:'wave',b:'wave',u:1}},.7,presentation);
        const open=middle.getWorldPosition(new T.Vector3()).distanceTo(hand.getWorldPosition(new T.Vector3()));
        assert.ok(closed<open-.001,id+' closed fingers are folded toward the palm');
        core.modelRig.apply(rig,{...state,walkTime:1.1},1.1,presentation);
        assert.equal(core.modelRig.apply(rig,state,.7,presentation),floor);assert.equal(snapshot(),first,id+' reverse seek after walking');
        for(const mesh of rig.meshes){
            assert.deepEqual(Object.keys(mesh.morphTargetDictionary),FACE_NAMES);
            // Speech must never deform the crown or upper skull.
            const pos=mesh.geometry.attributes.position;
            for(const name of ['ai','e','o','u','mbp','fv']){
                const morph=mesh.geometry.morphAttributes.position[mesh.morphTargetDictionary[name]];
                for(let i=0;i<pos.count;i++)if(pos.getY(i)>FACE_LANDMARKS[id].eyeY+.1)assert.equal(Math.abs(morph.getX(i))+Math.abs(morph.getY(i))+Math.abs(morph.getZ(i)),0,id+' crown');
            }
        }
        core.models.dispose(gltf.scene);
    }
});
