import fs from 'node:fs';
import path from 'node:path';
import {createHash} from 'node:crypto';
import {MODEL_CHARACTERS} from '../assets/storymaker/lib/model-characters.mjs';
import {authorFace,FACE_NAMES,FACE_LANDMARKS} from '../assets/storymaker/lib/model-face.mjs';
import {authorMotions} from '../assets/storymaker/lib/model-motions.mjs';
const root=path.resolve(import.meta.dirname,'..'),out=path.join(root,'assets/storymaker/models');
const hash=bytes=>createHash('sha256').update(bytes).digest('hex');
const sourceHash=file=>hash(fs.readFileSync(file,'utf8').replace(/\r\n?/g,'\n'));
const write=(file,bytes)=>{const target=path.join(out,file);if(process.argv.includes('--check')){if(!fs.existsSync(target)||!fs.readFileSync(target).equals(Buffer.from(bytes)))throw new Error('Stale Storymaker model: '+file);}else{fs.mkdirSync(path.dirname(target),{recursive:true});fs.writeFileSync(target,bytes);}};
for(const {id} of MODEL_CHARACTERS){
 const base=path.join(root,'ui/3d/pets',id),model=fs.readFileSync(path.join(base,'model.glb')),motion=fs.readFileSync(path.join(base,'motions.json'));
 const derived=authorFace(model,id),poses=await authorMotions(path.join(base,'model.glb'),JSON.parse(motion));
 if(derived.length>8*1048576)throw new Error('Model exceeds 8 MiB: '+id);
 write(id+'.glb',derived);write(id+'-motions.json',JSON.stringify(poses)+'\n');
 write(id+'-provenance.json',JSON.stringify({schema:'storymaker-model-source/1',source:'ui/3d/pets/'+id,model_sha256:hash(model),motions_sha256:hash(motion),derived_sha256:hash(derived),landmarks:FACE_LANDMARKS[id],morphs:FACE_NAMES,authoring:['model-face.mjs','model-motions.mjs'].map(file=>({file:'assets/storymaker/lib/'+file,sha256:sourceHash(path.join(root,'assets/storymaker/lib',file))})),geometry_notice:fs.readFileSync(path.join(base,'NOTICE.txt'),'utf8'),motion_license:'CC0-1.0 for reused clips; GPL-3.0-only for original explainer poses and facial deformation.'},null,2)+'\n');
 console.log(id+': '+derived.length+' bytes, '+FACE_NAMES.length+' facial shapes, '+Object.keys(poses.poses).length+' authored poses');
}
