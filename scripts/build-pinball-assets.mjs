import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { SAMPLE_FILES } from '../ui/js/desktop/apps/pinball/audio/samples.js';

process.chdir(fileURLToPath(new URL('../', import.meta.url)));
// Audio comes from two CC0 packs already in the repo; each pack's manifest is the provenance and the hash authority.
const check=process.argv.includes('--check'),root='ui/3d/pinball/v1',packs={minigolf:'ui/3d/minigolf/audio','aura-drift':'ui/3d/aura-drift/audio'};
const manifests={};
for(const [pack,dir] of Object.entries(packs))manifests[pack]=JSON.parse(await fs.readFile(dir+'/manifest.json','utf8'));
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
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
const tables=['neon','leviathan','temple'].map(id=>({id,backglass:id+'-backglass.webp',preview:id+'-concept.webp',underlay:id+'-playfield.webp'}));
const paths=[...tables.flatMap(t=>[t.underlay,t.backglass,t.preview]),...audio.map(a=>a.path)];
const files=[];
for(const path of paths){const bytes=await fs.readFile(root+'/'+path);files.push({path,bytes:bytes.length,sha256:hash(bytes)});}
if(files.reduce((n,f)=>n+f.bytes,0)>20*1024*1024)throw Error('Pinball assets exceed 20 MiB');
const text=JSON.stringify({version:2,tables,files,audio,sources},null,2)+'\n';
if(check){if(await fs.readFile(root+'/manifest.json','utf8')!==text)throw Error('Rebuild Pinball asset manifest');}
else await fs.writeFile(root+'/manifest.json',text);
console.log('Pinball assets '+(check?'verified':'built')+': '+files.length+' files, '+files.reduce((n,f)=>n+f.bytes,0)+' bytes');
