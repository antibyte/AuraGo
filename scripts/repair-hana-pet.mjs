import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadSkeleton } from './pet-skeleton.mjs';

const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const folder=path.join(root,'ui/3d/pets/hana'),file=path.join(folder,'model.glb');
const model=await readFile(file),report=JSON.parse(await readFile(path.join(root,'disposable/_hana-pet/hair-edge-check.json'),'utf8'));
const hash=data=>createHash('sha256').update(data).digest('hex');
assert.equal(report.model_sha256,hash(model),'Run the current Hana motion check before repairing its mesh');
assert.equal(model.readUInt32LE(0),0x46546c67);assert.equal(model.readUInt32LE(4),2);assert.equal(model.readUInt32LE(8),model.length);
const jsonLength=model.readUInt32LE(12),doc=JSON.parse(model.subarray(20,20+jsonLength));
assert.equal(model.readUInt32LE(24+jsonLength),0x004e4942);
const binary=Buffer.from(model.subarray(28+jsonLength));assert.equal(binary.length,model.readUInt32LE(20+jsonLength));
assert.equal(doc.meshes.length,1);assert.equal(doc.meshes[0].primitives.length,1);
const primitive=doc.meshes[0].primitives[0];assert.ok(primitive.mode===undefined||primitive.mode===4);
const indices=doc.accessors[primitive.indices],view=doc.bufferViews[indices.bufferView];
assert.ok([5121,5123,5125].includes(indices.componentType));assert.equal(indices.count%3,0);assert.ok(!view.byteStride);
const size={5121:1,5123:2,5125:4}[indices.componentType],offset=(view.byteOffset||0)+(indices.byteOffset||0);
const read={1:'readUInt8',2:'readUInt16LE',4:'readUInt32LE'}[size],write={1:'writeUInt8',2:'writeUInt16LE',4:'writeUInt32LE'}[size];
const bad=new Set(report.samples.flatMap(sample=>sample.badTriangles));
assert.ok([...bad].every(index=>Number.isSafeInteger(index)&&index>=0&&index<indices.count/3));
const target=await loadSkeleton(file);let surface;target.gltf.scene.traverse(object=>{if(object.isSkinnedMesh)surface=object;});
const geometry=surface.geometry,position=geometry.attributes.position,parent=new Int32Array(position.count),keys=new Map();
assert.equal(geometry.index.count,indices.count);
const find=index=>{while(parent[index]!==index){parent[index]=parent[parent[index]];index=parent[index];}return index;};
const join=(a,b)=>{a=find(a);b=find(b);if(a!==b)parent[b]=a;};
for(let index=0;index<position.count;index++){
    parent[index]=index;
    const signature=['getX','getY','getZ','getW'].map(method=>[geometry.attributes.skinIndex[method](index),Math.round(geometry.attributes.skinWeight[method](index)*10000)]).filter(pair=>pair[1]).sort((a,b)=>a[0]-b[0]);
    const key=[Math.round(position.getX(index)*100000),Math.round(position.getY(index)*100000),Math.round(position.getZ(index)*100000),JSON.stringify(signature)].join(',');
    if(keys.has(key))join(index,keys.get(key));else keys.set(key,index);
}
for(let face=0;face<indices.count/3;face++)if(!bad.has(face)){const start=face*3;join(geometry.index.getX(start),geometry.index.getX(start+1));join(geometry.index.getX(start),geometry.index.getX(start+2));}
const components=new Map();
for(let face=0;face<indices.count/3;face++)if(!bad.has(face)){
    const start=face*3,root=find(geometry.index.getX(start));let component=components.get(root);
    if(!component)components.set(root,component={faces:[],minY:Infinity,head:true});component.faces.push(face);
    for(let corner=0;corner<3;corner++){
        const vertex=geometry.index.getX(start+corner);component.minY=Math.min(component.minY,position.getY(vertex));
        for(const method of ['getX','getY','getZ','getW'])if(geometry.attributes.skinWeight[method](vertex)>.001&&!surface.skeleton.bones[geometry.attributes.skinIndex[method](vertex)].name.includes('spine006'))component.head=false;
    }
}
let islandFaces=0;
for(const component of components.values())if((!component.head&&component.faces.length<256)||(component.faces.length<32&&!(component.head&&component.minY>1.2)))for(const face of component.faces){bad.add(face);islandFaces++;}
if(!bad.size){console.log('No stretched faces or disconnected fragments need repair');process.exit(0);}
assert.ok(bad.size<indices.count/3*.05,'Repair only isolated damaged faces; widespread damage needs a new rig');
const kept=[];for(let face=0;face<indices.count/3;face++)if(!bad.has(face))for(let corner=0;corner<3;corner++)kept.push(binary[read](offset+(face*3+corner)*size));
kept.forEach((value,index)=>binary[write](value,offset+index*size));indices.count=kept.length;
indices.min=[kept.reduce((value,index)=>Math.min(value,index),Infinity)];indices.max=[kept.reduce((value,index)=>Math.max(value,index),0)];
const json=Buffer.from(JSON.stringify(doc)),padded=Buffer.alloc(Math.ceil(json.length/4)*4,0x20);json.copy(padded);
const header=Buffer.alloc(20);header.writeUInt32LE(0x46546c67,0);header.writeUInt32LE(2,4);header.writeUInt32LE(28+padded.length+binary.length,8);header.writeUInt32LE(padded.length,12);header.writeUInt32LE(0x4e4f534a,16);
const binHeader=Buffer.alloc(8);binHeader.writeUInt32LE(binary.length,0);binHeader.writeUInt32LE(0x004e4942,4);
const output=Buffer.concat([header,padded,binHeader,binary]);
const metadata=JSON.parse(await readFile(path.join(folder,'model.json'),'utf8'));
metadata.triangles=kept.length/3;metadata.model_sha256=hash(output);
metadata.skin_cleanup={source_model_sha256:metadata.skin_cleanup?.source_model_sha256||report.model_sha256,input_model_sha256:report.model_sha256,removed_triangles:(metadata.skin_cleanup?.removed_triangles||0)+bad.size,removed_island_triangles:(metadata.skin_cleanup?.removed_island_triangles||0)+islandFaces,sampled_poses:report.samples.length,method:'Remove stretched LOD faces and small detached body fragments; preserve rigid facial details'};
await writeFile(file,output);await writeFile(path.join(folder,'model.json'),JSON.stringify(metadata,null,2)+'\n');
console.log(JSON.stringify({removed:bad.size,triangles:metadata.triangles,model_sha256:metadata.model_sha256}));
