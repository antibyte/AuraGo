import fs from 'node:fs/promises';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { validateBytes } from 'gltf-validator';
import { fileURLToPath } from 'node:url';

process.chdir(fileURLToPath(new URL('../',import.meta.url)));
const root='ui/3d/pinball/v1',manifest=JSON.parse(await fs.readFile(root+'/manifest.json','utf8'));
assert.deepEqual(manifest.tables.map(t=>t.id),['neon','leviathan','temple']);
for(const file of manifest.files){const bytes=await fs.readFile(root+'/'+file.path);assert.equal(bytes.length,file.bytes);assert.equal(createHash('sha256').update(bytes).digest('hex'),file.sha256);}
for(const table of manifest.tables){
    const bytes=await fs.readFile(root+'/'+table.model);
    const report=await validateBytes(new Uint8Array(bytes),{uri:table.model});
    assert.equal(report.issues.numErrors,0,table.id+': '+JSON.stringify(report.issues.messages.filter(m=>m.severity===0)));
    const length=bytes.readUInt32LE(12),gltf=JSON.parse(bytes.subarray(20,20+length).toString());
    const physicsBytes=await fs.readFile(root+'/'+table.physics);
    assert.equal(gltf.scenes[0].extras.pinball_collision_sha256,createHash('sha256').update(physicsBytes).digest('hex'),'GLB and physics come from the same Blender scene');
    const names=new Set(gltf.nodes.map(n=>n.name));
    assert.ok(names.has('flipper-left')&&names.has('flipper-right')&&names.has('gate')&&names.has('plunger'));
    assert.ok(!gltf.images?.some(i=>i.uri&&/^(https?:)?\/\//.test(i.uri)));
    if(table.id==='temple')assert.ok(names.has('flipper-upper'));
    assert.ok(names.has('shot-lamp-left-0')&&names.has('shot-lamp-right-0'));
    if(table.id==='temple')assert.ok(names.has('temple-door-left')&&names.has('temple-door-right'));
    const physics=JSON.parse(await fs.readFile(root+'/'+table.physics,'utf8'));
    assert.equal(physics.flippers.length,table.id==='temple'?3:2);assert.equal(physics.bumpers.length,3);
    assert.ok(physics.sensors.some(s=>s.type==='lock'&&s.at[1]<0),'real below-playfield lock pocket');
    console.log(table.id+': valid GLB, actor IDs, matching collision payload and local textures');
}
for(const entry of manifest.audio)assert.equal(entry.license,'CC0-1.0');
const art=JSON.parse(await fs.readFile('assets/pinball/artwork.json','utf8'));
const prompts=JSON.parse(await fs.readFile('assets/pinball/prompts.json','utf8'));
assert.equal(art.assets.length,9);assert.equal(prompts.prompts.length,9);
for(const asset of art.assets){
    const bytes=await fs.readFile('assets/pinball/'+asset.file);
    assert.equal(createHash('sha256').update(bytes).digest('hex'),asset.sha256);
    assert.ok(prompts.prompts.some(p=>p.id===asset.table&&p.kind===asset.role),'retained prompt for '+asset.file);
}
const english=JSON.parse(await fs.readFile('ui/lang/desktop/en.json','utf8')),keys=Object.keys(english).filter(k=>k.startsWith('pinball.'));
assert.ok(keys.length>=80);
for(const lang of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']){
    const words=JSON.parse(await fs.readFile('ui/lang/desktop/'+lang+'.json','utf8'));
    for(const key of keys)assert.ok(typeof words[key]==='string'&&words[key].trim(),lang+': '+key);
}
console.log('Pinball assets, provenance, actor contracts and all sixteen locale sets passed');
