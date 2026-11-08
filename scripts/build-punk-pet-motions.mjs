import assert from 'node:assert/strict';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { build } from 'esbuild';
import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const source=path.join(root,'assets/desktop-pets/mesh2motion');
const out=path.join(root,'ui/3d/pets/punk');
const scratch=path.join(root,'disposable/_punk-pet');
await mkdir(scratch,{recursive:true});
const vendor=path.join(source,'retarget');
await build({stdin:{contents:`export { Rig } from './Rig.ts'; export { HumanClipRetargeter } from './HumanClipRetargeter.ts'; export { RigifyMapper } from './RigifyMapper.ts';`,resolveDir:vendor},outfile:path.join(scratch,'retarget.mjs'),bundle:true,platform:'node',format:'esm',packages:'external',logLevel:'silent'});
const {Rig,HumanClipRetargeter,RigifyMapper}=await import(pathToFileURL(path.join(scratch,'retarget.mjs')));

// Node needs only geometry and joints for retargeting, not browser texture decoding.
async function loadSkeleton(file){
    const raw=await readFile(file);
    assert.equal(raw.toString('ascii',0,4),'glTF');
    const length=raw.readUInt32LE(12);
    const doc=JSON.parse(raw.subarray(20,20+length));
    for(const mesh of doc.meshes||[])for(const primitive of mesh.primitives)delete primitive.material;
    delete doc.images; delete doc.textures; delete doc.materials;
    const text=Buffer.from(JSON.stringify(doc));
    const padded=Buffer.alloc(Math.ceil(text.length/4)*4,32); text.copy(padded);
    const bin=raw.subarray(20+length);
    const header=Buffer.alloc(20);
    header.write('glTF');header.writeUInt32LE(2,4);header.writeUInt32LE(20+padded.length+bin.length,8);header.writeUInt32LE(padded.length,12);header.writeUInt32LE(0x4e4f534a,16);
    const bytes=Buffer.concat([header,padded,bin]);
    const gltf=await new GLTFLoader().setMeshoptDecoder(MeshoptDecoder).parseAsync(bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength),'');
    gltf.scene.updateMatrixWorld(true);
    const bones=[];gltf.scene.traverse(o=>{if(o.isBone)bones.push(o);});
    assert.ok(bones.length>40);
    return {gltf,skeleton:new THREE.Skeleton(bones),rest:bones.map(b=>({p:b.position.clone(),q:b.quaternion.clone(),s:b.scale.clone()})),hash:createHash('sha256').update(raw).digest('hex')};
}
function reset(data){data.skeleton.bones.forEach((b,i)=>{b.position.copy(data.rest[i].p);b.quaternion.copy(data.rest[i].q);b.scale.copy(data.rest[i].s);});data.gltf.scene.updateMatrixWorld(true);}
const input=JSON.parse(await readFile(path.join(source,'source.json'),'utf8'));
const src=await loadSkeleton(path.join(source,'skeleton.glb'));
const target=await loadSkeleton(path.join(out,'model.glb'));
assert.equal(target.skeleton.bones.filter(b=>!b.parent?.isBone).length,1,'The deform skeleton must be connected');
const hand=target.skeleton.bones.find(b=>b.name==='DEF-handL');
const shoulder=target.skeleton.bones.find(b=>b.name==='DEF-upper_armL');
assert.ok(Math.abs(hand.getWorldPosition(new THREE.Vector3()).y-shoulder.getWorldPosition(new THREE.Vector3()).y)<.03,'Retargeting requires the baked T-pose');
const mapping=RigifyMapper.map_rigify_bones(src.skeleton.bones,target.skeleton.bones);
assert.ok(mapping.size>=50);
const reverse=new Map([...mapping].map(([t,s])=>[s,t]));
const config={pelvis:['pelvis'],spine:['spine_01','spine_02','spine_03'],head:['neck_01','head'],armL:['upperarm_l','lowerarm_l','hand_l'],armR:['upperarm_r','lowerarm_r','hand_r'],legL:['thigh_l','calf_l','foot_l'],legR:['thigh_r','calf_r','foot_r']};
for(const [side,suffix] of [['L','l'],['R','r']])for(const f of ['Thumb','Index','Middle','Ring','Pinky'])config['fingers'+f+side]=[1,2,3].map(i=>`${f.toLowerCase()}_0${i}_${suffix}`);
const targetConfig=Object.fromEntries(Object.entries(config).map(([k,v])=>[k,v.map(n=>reverse.get(n)||'')]));
const clips=[];
for(const motion of input.animations){
    reset(src);reset(target);
    const file=await loadSkeleton(path.join(source,'animations',motion.id+'.glb'));
    assert.equal(file.gltf.animations.length,1);
    const baked=HumanClipRetargeter.retarget(file.gltf.animations[0],new Rig(src.skeleton).fromConfig(config),new Rig(target.skeleton).fromConfig(targetConfig),{clip_name:motion.id,fps:24});
    baked.optimize();
    assert.ok(baked.tracks.length>=60);
    assert.ok(baked.tracks.every(t=>[...t.values].every(Number.isFinite)));
    assert.ok(baked.tracks.some(t=>t.values.some((v,i)=>i>=t.getValueSize()&&Math.abs(v-t.values[i%t.getValueSize()])>.01)));
    clips.push({id:motion.id,name:motion.name,loop:motion.loop,source_sha256:file.hash,clip:THREE.AnimationClip.toJSON(baked)});
}
const body=JSON.stringify({version:1,repository:input.repository,revision:input.revision,license:'CC0-1.0',skeleton_sha256:src.hash,model_sha256:target.hash,clips},(k,v)=>typeof v==='number'?Math.round(v*1e5)/1e5:v)+'\n';
assert.ok(Buffer.byteLength(body)<10*1024*1024);
await writeFile(path.join(out,'motions.json'),body);
await writeFile(path.join(out,'motions-manifest.json'),JSON.stringify({revision:input.revision,skeleton_sha256:src.hash,clips:clips.map(({id,name,loop,source_sha256,clip})=>({id,name,loop,source_sha256,duration:clip.duration})),bytes:Buffer.byteLength(body),sha256:createHash('sha256').update(body).digest('hex')},null,2)+'\n');
console.log(`Punk pet: ${clips.length} retargeted Mesh2Motion clips, ${Buffer.byteLength(body)} bytes`);
