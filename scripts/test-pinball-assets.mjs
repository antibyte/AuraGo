import fs from 'node:fs/promises';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { TABLE_IDS } from '../ui/js/desktop/apps/pinball/config.js';
import { readGlb, triangleCount, rootNames } from './lib/glb-bounds.mjs';

process.chdir(fileURLToPath(new URL('../',import.meta.url)));
const root='ui/3d/pinball/v1',manifest=JSON.parse(await fs.readFile(root+'/manifest.json','utf8'));
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const MiB=1024*1024;
assert.equal(manifest.version,3);
// Tables are a subset of TABLE_IDS, in TABLE_IDS order (a table joins the manifest once its art exists).
const ids=manifest.tables.map(t=>t.id);
assert.ok(ids.length>0&&ids.every(id=>TABLE_IDS.includes(id)),'manifest tables are registered: '+ids);
assert.deepEqual(ids,TABLE_IDS.filter(id=>ids.includes(id)),'manifest tables follow TABLE_IDS order');
for(const file of manifest.files){const bytes=await fs.readFile(root+'/'+file.path);assert.equal(bytes.length,file.bytes,file.path);assert.equal(sha(bytes),file.sha256,file.path);}
assert.ok(manifest.files.reduce((n,f)=>n+f.bytes,0)<=20*MiB,'payload cap');

// No orphans: every delivered file is listed in the manifest.
const listed=new Set([...manifest.files.map(f=>f.path),'manifest.json']);
const walk=async dir=>{const out=[];for(const e of await fs.readdir(root+'/'+dir,{withFileTypes:true})){const rel=dir?dir+'/'+e.name:e.name;if(e.isDirectory())out.push(...await walk(rel));else out.push(rel);}return out;};
for(const rel of await walk(''))assert.ok(listed.has(rel),'orphan file not in manifest: '+rel);
for(const path of listed)await fs.access(root+'/'+path);

// Artwork records (needed for artRev) and prompts.
const art=JSON.parse(await fs.readFile('assets/pinball/artwork.json','utf8'));
const prompts=JSON.parse(await fs.readFile('assets/pinball/prompts.json','utf8'));

// WebP header: canvas size and the alpha flag of a VP8X file.
const webpInfo=bytes=>{
    assert.equal(bytes.toString('latin1',0,4),'RIFF');assert.equal(bytes.toString('latin1',8,12),'WEBP');
    const kind=bytes.toString('latin1',12,16);
    if(kind==='VP8 ')return {alpha:false,w:bytes.readUInt16LE(26)&0x3fff,h:bytes.readUInt16LE(28)&0x3fff};
    assert.equal(kind,'VP8X','VP8 or VP8X header');
    return {alpha:(bytes[20]&0x10)!==0,w:1+bytes.readUIntLE(24,3),h:1+bytes.readUIntLE(27,3)};
};
for(const t of manifest.tables){
    for(const key of ['model','physics','normal','ao','playfield'])assert.ok(!(key in t),t.id+': legacy field '+key);
    assert.equal(t.underlay,t.id+'-playfield.webp');
    for(const key of ['underlay','backglass','preview'])assert.ok(t[key]&&listed.has(t[key]),t.id+': '+key+' not in manifest');
    for(const key of ['plastics','side'])if(key in t)assert.ok(listed.has(t[key]),t.id+': '+key+' not listed');
    const playfield=art.assets.find(a=>a.table===t.id&&a.role==='playfield');
    assert.ok(playfield,t.id+': playfield artwork record');assert.equal(t.artRev,playfield.rev??1,t.id+': artRev');
    if(t.plastics){const info=webpInfo(await fs.readFile(root+'/'+t.plastics));assert.ok(info.alpha&&info.w===1024&&info.h===1536,t.id+': plastics 1024x1536 with alpha');}
    if(t.side){const info=webpInfo(await fs.readFile(root+'/'+t.side));assert.ok(info.w===2048&&info.h===682,t.id+': side 2048x682');}
}
if(manifest.room){assert.ok(listed.has(manifest.room.wall),'room wall not in manifest');}

// GLB toys: one file per table at most, listed in models.json, unchanged copies of the sources.
const GLB_EXTENSIONS=new Set(['KHR_materials_emissive_strength','KHR_texture_transform','EXT_texture_webp','KHR_materials_transmission','KHR_materials_ior']);
const REQUIRED_ROOTS=['orrery','pendulum','telescope','gears','dome'];
const models=JSON.parse(await fs.readFile('assets/pinball/models.json','utf8'));
assert.equal(models.version,1);
const listedGlb=manifest.tables.flatMap(t=>Object.entries(t.glb||{}).map(([key,file])=>({table:t.id,key,file})));
assert.equal(listedGlb.length,models.models.filter(m=>ids.includes(m.table)).length,'manifest glb entries match models.json');
let glbTotal=0;
for(const {table,key,file} of listedGlb){
    assert.equal(key,'toys',table+': glb key');assert.ok(listed.has(file),file+' not listed');
    const model=models.models.find(m=>m.table===table&&m.key===key);assert.ok(model,table+'/'+key+' in models.json');
    const bytes=await fs.readFile(root+'/'+file);glbTotal+=bytes.length;
    assert.ok(bytes.length<=1.5*MiB,file+' over 1.5 MiB');
    assert.equal(sha(bytes),model.sha256,file+' hash');assert.ok(bytes.equals(await fs.readFile('assets/pinball/'+model.file)),file+' is a copy of its source');
    const {json}=readGlb(bytes);
    for(const ext of [...(json.extensionsUsed||[]),...(json.extensionsRequired||[])])assert.ok(GLB_EXTENSIONS.has(ext),file+': extension '+ext+' not allowed (no Draco, meshopt or basisu)');
    for(const buffer of json.buffers||[])assert.ok(buffer.uri===undefined,file+': external buffer');
    assert.ok(triangleCount(json)<=40000,file+': triangle budget');
    const names=new Set(rootNames(json));
    for(const name of REQUIRED_ROOTS)assert.ok(names.has(name),file+': root node '+name);
}
assert.ok(glbTotal<=4*MiB,'GLB payload cap');

// Retained artwork masters and prompts.
assert.ok(art.assets.length>=9&&prompts.prompts.length>=9,'artwork and prompt records');
for(const asset of art.assets){
    const bytes=await fs.readFile('assets/pinball/'+asset.file);
    assert.equal(sha(bytes),asset.sha256,asset.file);
    assert.ok(prompts.prompts.some(p=>p.id===asset.table&&p.kind===asset.role),'retained prompt for '+asset.file);
}
for(const p of prompts.prompts)assert.ok(p.id&&p.kind&&p.prompt,'prompt record '+p.id+'/'+p.kind);

// CC0 audio provenance.
assert.equal(manifest.audio.length,29);
for(const id of ['drift-impact-metal-2','golf-roll-hard-1','drift-scrape-metal-1','golf-win-moon-1'])assert.ok(manifest.audio.some(a=>a.path==='audio/'+id+'.ogg'),'audio '+id);
assert.ok(manifest.audio.some(a=>a.pack==='aura-drift')&&manifest.audio.some(a=>a.pack==='minigolf'),'both CC0 packs');
assert.ok(manifest.audio.reduce((n,a)=>n+a.bytes,0)<1024*1024,'audio payload');
for(const entry of manifest.audio){
    assert.equal(entry.license,'CC0-1.0');assert.ok(entry.source&&entry.sha256,entry.path);
    assert.ok(manifest.sources.some(s=>s.id===entry.source),'source record for '+entry.path);
    assert.equal(sha(await fs.readFile(root+'/'+entry.path)),entry.sha256,entry.path);
}

// Locale coverage.
const english=JSON.parse(await fs.readFile('ui/lang/desktop/en.json','utf8')),keys=Object.keys(english).filter(k=>k.startsWith('pinball.'));
assert.ok(keys.length>=80);
for(const lang of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']){
    const words=JSON.parse(await fs.readFile('ui/lang/desktop/'+lang+'.json','utf8'));
    for(const key of keys)assert.ok(typeof words[key]==='string'&&words[key].trim(),lang+': '+key);
}
console.log('Pinball assets (manifest v3), no orphans, CC0 provenance, artwork retention, GLB rules and all sixteen locale sets passed');
