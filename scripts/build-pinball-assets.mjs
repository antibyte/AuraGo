import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { SAMPLE_FILES } from '../ui/js/desktop/apps/pinball/audio/samples.js';
import { TABLE_IDS } from '../ui/js/desktop/apps/pinball/config.js';

process.chdir(fileURLToPath(new URL('../', import.meta.url)));
// Audio comes from two CC0 packs already in the repo; each pack's manifest is the provenance and the hash authority.
const check=process.argv.includes('--check'),root='ui/3d/pinball/v1',packs={minigolf:'ui/3d/minigolf/audio','aura-drift':'ui/3d/aura-drift/audio'};
const MiB=1024*1024,CAPS={total:20*MiB,glb:1.5*MiB,glbs:4*MiB};
const manifests={};
for(const [pack,dir] of Object.entries(packs))manifests[pack]=JSON.parse(await fs.readFile(dir+'/manifest.json','utf8'));
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const exists=path=>fs.access(root+'/'+path).then(()=>true,()=>false);
const audio=[],usedSources=new Map();
for(const [id,{pack,path}] of Object.entries(SAMPLE_FILES)){
    const manifest=manifests[pack],record=manifest.files.find(f=>f.path===path);
    if(!record||record.license!=='CC0-1.0')throw Error('Missing CC0 audio provenance: '+pack+'/'+path);
    const sourceId=record.source||record.origin,source=manifest.sources.find(s=>s.id===sourceId);
    if(!source||source.license!=='CC0-1.0')throw Error('Missing CC0 source record: '+sourceId+' for '+pack+'/'+path);
    const bytes=await fs.readFile(packs[pack]+'/'+path);if(hash(bytes)!==record.sha256)throw Error('Audio source drift: '+pack+'/'+path);
    const target='audio/'+id+'.ogg';
    if(check){if(!(await fs.readFile(root+'/'+target)).equals(bytes))throw Error('Rebuild '+target);}
    else{await fs.mkdir(root+'/audio',{recursive:true});await fs.writeFile(root+'/'+target,bytes);}
    const prior=usedSources.get(sourceId);if(prior&&prior.sha256!==source.sha256)throw Error('Conflicting source id '+sourceId);
    usedSources.set(sourceId,{...source,pack});
    audio.push({path:target,bytes:record.bytes,sha256:record.sha256,seconds:record.seconds,channels:record.channels,pack,packPath:path,source:sourceId,member:record.member,license:record.license});
}
const sources=[...usedSources.values()];

// GLB toys: assets/pinball/models.json lists { table, key, file, sha256, triangles, script }; the file is copied unchanged.
const models=JSON.parse(await fs.readFile('assets/pinball/models.json','utf8')).models;
const glbFiles=[];
for(const model of models){
    const bytes=await fs.readFile('assets/pinball/'+model.file);
    if(hash(bytes)!==model.sha256)throw Error('GLB source drift: '+model.file+' (update models.json)');
    if(bytes.length>CAPS.glb)throw Error('GLB over 1.5 MiB: '+model.file);
    const name=model.file.split('/').pop();
    if(check){if(!(await fs.readFile(root+'/'+name)).equals(bytes))throw Error('Rebuild '+name);}
    else await fs.writeFile(root+'/'+name,bytes);
    glbFiles.push({...model,name,size:bytes.length});
}
if(glbFiles.reduce((n,g)=>n+g.size,0)>CAPS.glbs)throw Error('GLBs exceed 4 MiB');

// Tables in TABLE_IDS order. A table without a delivered underlay is left out until its art exists; optional keys only for delivered files.
const art=JSON.parse(await fs.readFile('assets/pinball/artwork.json','utf8')).assets;
const tables=[];
for(const id of TABLE_IDS){
    if(!await exists(id+'-playfield.webp'))continue;
    if(!await exists(id+'-backglass.webp'))throw Error(id+': backglass missing');
    const rev=art.find(a=>a.table===id&&a.role==='playfield')?.rev??1,entry={id,underlay:id+'-playfield.webp',backglass:id+'-backglass.webp'};
    if(await exists(id+'-preview.webp'))entry.preview=id+'-preview.webp';
    entry.artRev=rev;
    for(const key of ['plastics','side'])if(await exists(id+'-'+key+'.webp'))entry[key]=id+'-'+key+'.webp';
    const toys=glbFiles.filter(g=>g.table===id);
    if(toys.length)entry.glb=Object.fromEntries(toys.map(g=>[g.key,g.name]));
    tables.push(entry);
}
for(const g of glbFiles)if(!tables.some(t=>t.id===g.table))throw Error('GLB for a table that is not delivered: '+g.file);
const room=await exists('room-wall.webp')?{wall:'room-wall.webp'}:null;
const paths=[...tables.flatMap(t=>[t.underlay,t.backglass,...(t.preview?[t.preview]:[]),...(t.plastics?[t.plastics]:[]),...(t.side?[t.side]:[]),...Object.values(t.glb||{})]),...(room?[room.wall]:[]),...audio.map(a=>a.path)];
const files=[];
for(const path of paths){const bytes=await fs.readFile(root+'/'+path);files.push({path,bytes:bytes.length,sha256:hash(bytes)});}
if(files.reduce((n,f)=>n+f.bytes,0)>CAPS.total)throw Error('Pinball assets exceed 20 MiB');
const text=JSON.stringify({version:3,tables,...(room?{room}:{}),files,audio,sources},null,2)+'\n';
if(check){if(await fs.readFile(root+'/manifest.json','utf8')!==text)throw Error('Rebuild Pinball asset manifest');}
else await fs.writeFile(root+'/manifest.json',text);
console.log('Pinball assets '+(check?'verified':'built')+': '+files.length+' files, '+files.reduce((n,f)=>n+f.bytes,0)+' bytes');
