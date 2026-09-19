import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import validator from 'gltf-validator';
import {interiors,canWalk,groundHeight,daylight} from '../ui/js/desktop/apps/sysworld-exploration.js';
import {districts} from '../ui/js/desktop/apps/sysworld-scene.js';

const base='ui/3d/system-world/v2',manifest=JSON.parse(await fs.readFile(base+'/manifest.json'));
assert.equal(manifest.assets.length,27);
let models=0,bytes=0,warnings=0;
for(const asset of manifest.assets){
  assert.deepEqual(asset.lods.map(l=>l.level),[0,1,2]);
  assert.ok(asset.navigation);
  let triangles=Infinity;
  for(const lod of asset.lods){
    const data=await fs.readFile(base+'/'+lod.file);models++;bytes+=data.length;
    assert.equal(data.length,lod.bytes);assert.equal(createHash('sha256').update(data).digest('hex'),lod.sha256);
    assert.ok(lod.triangles<=triangles);triangles=lod.triangles;
    const checked=await validator.validateBytes(new Uint8Array(data),{uri:lod.file,maxIssues:100});
    assert.equal(checked.issues.numErrors,0,JSON.stringify({file:lod.file,issues:checked.issues}));warnings+=checked.issues.numWarnings;
    const size=data.readUInt32LE(12),doc=JSON.parse(data.subarray(20,20+size));
    assert.ok(doc.buffers.every(b=>!b.uri));assert.ok(!doc.images?.length);
    assert.deepEqual((doc.animations||[]).map(a=>a.name).sort(),asset.animations);
    const binary=data.subarray(28+size);
    for(const clip of doc.animations||[]){
      // Each promised clip must deform articulated parts, not only the whole root.
      assert.ok(clip.channels.some(c=>c.target.node!==doc.scenes[0].nodes[0]));
      const changes=clip.samplers.some(s=>{
        const a=doc.accessors[s.output],v=doc.bufferViews[a.bufferView];if(a.componentType!==5126)return false;
        const width={SCALAR:1,VEC3:3,VEC4:4}[a.type],values=[];
        for(let i=0;i<a.count;i++){const frame=[];for(let j=0;j<width;j++)frame.push(binary.readFloatLE((v.byteOffset||0)+(a.byteOffset||0)+i*(v.byteStride||width*4)+j*4).toFixed(4));values.push(frame.join(','));}
        return new Set(values).size>1;
      });assert.ok(changes,asset.id+':'+clip.name+' has no movement');
    }
  }
}
for(const room of interiors){
  const outside={x:room.x,z:room.doorZ+room.front*2},inside={x:room.x,z:room.doorZ-room.front*.2};
  assert.ok(canWalk(outside.x,outside.z,{x:outside.x,z:outside.z+room.front},()=>false,districts),'Door approach must be outside unrelated building collisions: '+room.id);
  assert.equal(canWalk(inside.x,inside.z,outside,()=>false,districts),false);
  assert.equal(canWalk(inside.x,inside.z,{x:room.x,z:room.doorZ+room.front*.2},()=>true,districts),true);
  assert.equal(canWalk(room.x+5.9,room.z,{x:room.x+5,z:room.z},()=>true,districts),false);
}
assert.equal(groundHeight(-80,52),0);assert.equal(groundHeight(-80,46),2);assert.equal(daylight('day').amount,1);assert.equal(daylight('night').amount,0);
assert.ok(canWalk(43,-10,{x:43,z:-9},()=>false,districts),'Integration passage must stay open');
assert.ok(canWalk(0,-2,{x:0,z:0},()=>false,districts),'Spire plaza outside ground-floor solid must be usable');
assert.equal(canWalk(0,-12,{x:0,z:-11},()=>false,districts),false,'Solid spire foundation cannot be crossed');
assert.ok(canWalk(20,-55,{x:20,z:-54},()=>false,districts),'Space between towers must stay open');
async function size(dir){let total=0;for(const f of await fs.readdir(dir,{withFileTypes:true})){const p=path.join(dir,f.name);total+=f.isDirectory()?await size(p):(await fs.stat(p)).size;}return total;}
// Count the renderer, classic app modules, CSS and all sixteen translated sections,
// not only the model files. Authoring sources and browser review code are excluded.
let runtime=await size('ui/3d/system-world')+await size('ui/js/vendor/system-world');
for(const name of ['sysworld.js','sysworld-data.js','sysworld-hud.js','sysworld-controls.js'])runtime+=(await fs.stat('ui/js/desktop/apps/'+name)).size;
runtime+=(await fs.stat('ui/css/desktop-app-sysworld.css')).size;
for(const name of await fs.readdir('ui/lang/desktop')){if(!name.endsWith('.json'))continue;const locale=JSON.parse(await fs.readFile('ui/lang/desktop/'+name));runtime+=Buffer.byteLength(JSON.stringify(Object.fromEntries(Object.entries(locale).filter(([key])=>key.startsWith('sysworld.')))));}
assert.ok(runtime<48*1024*1024);
console.log(JSON.stringify({assets:manifest.assets.length,models,bytes,runtimeBytes:runtime,validatorWarnings:warnings,checks:'LOD, hashes, articulated clips, doors, navigation, time, budget passed'},null,2));
