import {createHash} from 'node:crypto';
import {versionOf} from './catalog.mjs';
import {CHARACTER_IDS,ENVIRONMENT_IDS,PROP_PACKS} from './raster-catalog.mjs';
import {NAMES,nameKey} from './names.mjs';
export function validateRasterOutputs({files,manifest,entries,rig}){
 const errors=[],check=(ok,message)=>{if(!ok)errors.push(message);};
 check(manifest.schema==='storymaker-catalog/2','Wrong raster schema');
 check(manifest.version===versionOf(files),'Wrong catalog version');
 const expected={character:CHARACTER_IDS,environment:[...ENVIRONMENT_IDS,'board'],prop:Object.values(PROP_PACKS).flat()};
 for(const [kind,ids]of Object.entries(expected))check(entries.filter(e=>e.kind===kind).length===ids.length,`Wrong ${kind} count`);
 const delivery=new Map(manifest.files.map(f=>[f.path,f]));
 check(delivery.size===manifest.files.length,'Duplicate delivery file');
 check(delivery.size===files.size-1,'Incomplete delivery inventory');
 check(manifest.files.reduce((n,f)=>n+f.bytes,0)<=256*1048576,'Catalog exceeds 256 MiB');
 for(const [file,value]of files){
  if(file==='catalog.json')continue;
  const bytes=Buffer.from(value),m=delivery.get(file);
  check(m&&m.bytes===bytes.length&&m.sha256===createHash('sha256').update(bytes).digest('hex'),`Incorrect inventory: ${file}`);
  check(m&&m.bytes<=8*1048576,`Oversized file: ${file}`);
  if(m?.type==='image/png')check(m.width===bytes.readUInt32BE(16)&&m.height===bytes.readUInt32BE(20),`Incorrect image dimensions: ${file}`);
 }
 check(delivery.has(manifest.icons?.file),`Unlisted icons file: ${manifest.icons?.file}`);
 const json=file=>JSON.parse(files.get(file));
 function region(image,where){
  const m=delivery.get(image?.file),r=image?.rect;
  check(m?.type==='image/png',`Unlisted image: ${where}`);
  if(r)check(r.length===4&&r.every(Number.isInteger)&&r[0]>=0&&r[1]>=0&&r[2]>0&&r[3]>0&&r[0]+r[2]<=m?.width&&r[1]+r[3]<=m?.height,`Invalid region: ${where}`);
  if(image?.box)check(image.box.length===4&&image.box.every(Number.isFinite)&&image.box[2]>0&&image.box[3]>0,`Invalid image placement: ${where}`);
 }
 const bones=new Set(rig.bones.map(b=>b.name));check(bones.size===13,'Shared rig must have 13 bones');
 const ids=new Set();
 for(const e of entries){
  check(!ids.has(e.id),`Duplicate entry: ${e.id}`);ids.add(e.id);
  check(e.name_key===nameKey(e.id)&&NAMES[e.id]?.length===16&&NAMES[e.id].every(n=>typeof n==='string'&&n.length),`Missing names: ${e.id}`);
  for(const f of [e.file,e.thumb,...(e.resources||[]),...(e.takes||[]).map(t=>t.file)].filter(Boolean))check(delivery.has(f),`Unlisted dependency: ${e.id}/${f}`);
  if(e.kind==='character'){
   const d=json(e.file);check(d.schema==='storymaker-character-3d/1',`Bad character schema: ${e.id}`);
   check(delivery.get(d.model)?.type==='model/gltf-binary',`Unlisted model: ${e.id}`);
   check(e.resources.includes(d.model)&&e.resources.includes(d.motions),`Missing model dependencies: ${e.id}`);
   const motions=json(d.motions);check(motions.schema==='storymaker-model-motions/1',`Bad motion schema: ${e.id}`);
   for(const pose of entries.filter(e=>e.kind==='pose'))check(!!motions.poses[pose.name],`Missing model pose: ${e.id}/${pose.name}`);
   for(const alias of e.aliases||[]){check(/^char\.[a-z0-9-]+$/.test(alias)&&!ids.has(alias),`Invalid character alias: ${alias}`);ids.add(alias);}
  }
  if(e.kind==='prop')region(json(e.file).image,e.id);
  if(e.kind==='environment'&&e.drawn==='board'){
   const d=json(e.file);check(e.id==='env.board'&&d.drawn==='board','Unknown drawn environment: '+e.id);
   for(const variant of ['day','night'])check(Array.isArray(d.variants?.[variant])&&d.variants[variant].length===0&&d.tokens?.[variant]?.text,'Bad drawn environment variant: '+e.id+'/'+variant);
  }else if(e.kind==='environment')for(const variant of ['day','night']){
   const layers=json(e.file).variants[variant];check(layers?.length>=2,`Missing depth layers: ${e.id}/${variant}`);
   for(const layer of layers||[])region(layer.image,e.id+'/'+variant);
  }
 }
 return errors;
}
