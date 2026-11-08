import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

process.chdir(fileURLToPath(new URL('../', import.meta.url)));
const check=process.argv.includes('--check'),root='ui/3d/pinball/v1',sourceRoot='ui/3d/minigolf/audio';
const source=JSON.parse(await fs.readFile(sourceRoot+'/manifest.json','utf8'));
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const selected={flipper:'sfx/impact-metal-heavy-1.ogg',bumper:'sfx/impact-metal-medium-1.ogg',sling:'sfx/impact-soft-heavy-1.ogg',target:'sfx/impact-wood-medium-1.ogg',launch:'sfx/impact-metal-heavy-2.ogg',drain:'sfx/cup-rattle-1.ogg',lock:'sfx/impact-metal-light-1.ogg',jackpot:'sfx/win-moon-1.ogg',bonus:'sfx/win-temple-1.ogg'};
const audio=[];
for(const [id,path] of Object.entries(selected)){
    const record=source.files.find(f=>f.path===path);
    if(!record||record.license!=='CC0-1.0')throw Error('Missing CC0 audio provenance: '+path);
    const bytes=await fs.readFile(sourceRoot+'/'+path);if(hash(bytes)!==record.sha256)throw Error('Audio source drift: '+path);
    const target='audio/'+id+'.ogg';
    if(check){if(!(await fs.readFile(root+'/'+target)).equals(bytes))throw Error('Rebuild '+target);}
    else{await fs.mkdir(root+'/audio',{recursive:true});await fs.writeFile(root+'/'+target,bytes);}
    audio.push({...record,path:target});
}
const sources=source.sources.filter(s=>audio.some(a=>a.source===s.id));
const tables=['neon','leviathan','temple'].map(id=>({id,model:id+'.glb',physics:id+'-physics.json',playfield:id+'-playfield.webp',backglass:id+'-backglass.webp',preview:id+'-concept.webp',normal:id+'-normal.png',ao:id+'-ao.png'}));
const paths=[...tables.flatMap(t=>[t.model,t.physics,t.playfield,t.backglass,t.preview,t.normal,t.ao]),...audio.map(a=>a.path)];
const files=[];
for(const path of paths){const bytes=await fs.readFile(root+'/'+path);files.push({path,bytes:bytes.length,sha256:hash(bytes)});}
if(files.reduce((n,f)=>n+f.bytes,0)>110*1024*1024)throw Error('Pinball assets exceed 110 MiB');
const text=JSON.stringify({version:1,blender:'5.2.2',tables,files,audio,sources},null,2)+'\n';
if(check){if(await fs.readFile(root+'/manifest.json','utf8')!==text)throw Error('Rebuild Pinball asset manifest');}
else await fs.writeFile(root+'/manifest.json',text);
console.log('Pinball assets '+(check?'verified':'built')+': '+files.length+' files, '+files.reduce((n,f)=>n+f.bytes,0)+' bytes');
