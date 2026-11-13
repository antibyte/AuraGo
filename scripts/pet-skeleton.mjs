import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import * as THREE from 'three';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { MeshoptDecoder } from 'three/addons/libs/meshopt_decoder.module.js';

// Node needs only geometry and joints for retargeting, not browser texture decoding.
export async function loadSkeleton(file){
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
    assert.ok(bones.length>0);
    return {gltf,skeleton:new THREE.Skeleton(bones),rest:bones.map(b=>({p:b.position.clone(),q:b.quaternion.clone(),s:b.scale.clone()})),hash:createHash('sha256').update(raw).digest('hex')};
}
export function reset(data){data.skeleton.bones.forEach((b,i)=>{b.position.copy(data.rest[i].p);b.quaternion.copy(data.rest[i].q);b.scale.copy(data.rest[i].s);});data.gltf.scene.updateMatrixWorld(true);}
