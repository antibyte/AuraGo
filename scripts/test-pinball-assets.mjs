import fs from 'node:fs/promises';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

process.chdir(fileURLToPath(new URL('../',import.meta.url)));
const root='ui/3d/pinball/v1',manifest=JSON.parse(await fs.readFile(root+'/manifest.json','utf8'));
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
assert.equal(manifest.version,2);
assert.deepEqual(manifest.tables.map(t=>t.id),['neon','leviathan','temple']);
for(const t of manifest.tables){
    assert.equal(t.underlay,t.id+'-playfield.webp');assert.ok(t.backglass&&t.preview);
    for(const key of ['model','physics','normal','ao','playfield'])assert.ok(!(key in t),t.id+': legacy field '+key);
}
for(const file of manifest.files){const bytes=await fs.readFile(root+'/'+file.path);assert.equal(bytes.length,file.bytes,file.path);assert.equal(sha(bytes),file.sha256,file.path);}
assert.ok(manifest.files.reduce((n,f)=>n+f.bytes,0)<=20*1024*1024,'payload cap');

// No orphans: every delivered file is listed in the manifest.
const listed=new Set([...manifest.files.map(f=>f.path),'manifest.json']);
const walk=async dir=>{const out=[];for(const e of await fs.readdir(root+'/'+dir,{withFileTypes:true})){const rel=dir?dir+'/'+e.name:e.name;if(e.isDirectory())out.push(...await walk(rel));else out.push(rel);}return out;};
for(const rel of await walk(''))assert.ok(listed.has(rel),'orphan file not in manifest: '+rel);
for(const path of listed)await fs.access(root+'/'+path);
for(const t of manifest.tables)for(const key of ['underlay','backglass','preview'])assert.ok(listed.has(t[key]),t.id+': '+key+' not in manifest');

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

// Retained artwork masters and prompts.
const art=JSON.parse(await fs.readFile('assets/pinball/artwork.json','utf8'));
const prompts=JSON.parse(await fs.readFile('assets/pinball/prompts.json','utf8'));
assert.equal(art.assets.length,9);assert.equal(prompts.prompts.length,9);
for(const asset of art.assets){
    const bytes=await fs.readFile('assets/pinball/'+asset.file);
    assert.equal(sha(bytes),asset.sha256);
    assert.ok(prompts.prompts.some(p=>p.id===asset.table&&p.kind===asset.role),'retained prompt for '+asset.file);
}

// Locale coverage.
const english=JSON.parse(await fs.readFile('ui/lang/desktop/en.json','utf8')),keys=Object.keys(english).filter(k=>k.startsWith('pinball.'));
assert.ok(keys.length>=80);
for(const lang of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']){
    const words=JSON.parse(await fs.readFile('ui/lang/desktop/'+lang+'.json','utf8'));
    for(const key of keys)assert.ok(typeof words[key]==='string'&&words[key].trim(),lang+': '+key);
}
console.log('Pinball assets, no orphans, CC0 provenance, artwork retention and all sixteen locale sets passed');
