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
            const cavity=rig.meshes.find(m=>{const a=m.geometry.attributes.position;return a.count===66&&a.getX(0)===a.getX(1);});
            const center=cavity.getVertexPosition(0,new T.Vector3()).applyMatrix4(cavity.matrixWorld);
            const normal=cavity.applyBoneTransform(0,new T.Vector3(0,0,1)).sub(cavity.applyBoneTransform(0,new T.Vector3())).transformDirection(cavity.matrixWorld);
            const face=rig.meshes.filter(m=>m.geometry.attributes.position.count>66);
            face.forEach(m=>{if(m.isSkinnedMesh)m.computeBoundingSphere();else m.geometry.computeBoundingSphere();});
            const hit=new T.Raycaster(center.clone().add(normal),normal.clone().negate()).intersectObjects(face,false)[0];
            assert.ok(hit&&hit.distance>1.001,id+'/'+mouth+' mouth cavity must cover the deformed face');
        }
        const block={id:'spoken:n',kind:'narration',speaker:'actor',duration_ms:1000,spans:[[0,1000]],speech_ok:true,levels:[150,210],pieces:[{hash:'a'.repeat(64),ext:'wav',at_ms:0,in_ms:0,out_ms:1000,chars:10}]};
        const compiled=core.compile({cast:[{id:'actor',character:'char.'+id}],scenes:[{id:'spoken',speaker:'actor',narration:'Hallo Welt',actors:[{cast:'actor',at:'center'}]}]},{narration:{spoken:{blocks:[block]}}},readJSON('ui/img/storymaker/catalog.json'));
        const actor=compiled.timeline.scenes[0].entities.find(e=>e.type==='actor'),speaking=core.scene.evalEntity(actor,.5),shapes=Array.from(core.lipsync.track(block));
        core.modelRig.apply(rig,speaking,.5,{...presentation,mouth:speaking.mouth},actor);
        for(const mesh of rig.meshes)for(const name of shapes)assert.equal(mesh.morphTargetInfluences[mesh.morphTargetDictionary[name]],.5,id+' interpolates compiled speech shapes');
        const quiet=core.scene.evalEntity(actor,1.1);core.modelRig.apply(rig,quiet,1.1,presentation,actor);
        for(const mesh of rig.meshes)for(const name of shapes)assert.equal(mesh.morphTargetInfluences[mesh.morphTargetDictionary[name]],0,id+' speech ends at rest');
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
