import fs from 'node:fs/promises';
import validator from 'gltf-validator';
const id=process.argv[2]||'aurago-low-poly';
if(!/^[a-z0-9-]+$/.test(id))throw Error('Expected a catalog pack ID');
const root='internal/gamemaker/asset_packs/'+id+'/';
const manifest=JSON.parse(await fs.readFile(root+'manifest.json','utf8'));
const files=new Set(manifest.assets.flatMap(a=>[...a.lods,...(a.animation_library?[a.animation_library]:[])].map(f=>f.file)));
const results=[];
for(const file of files){
    const bytes=await fs.readFile(root+file);
    const result=await validator.validateBytes(bytes,{uri:file,maxIssues:100,externalResourceFunction:()=>Promise.reject(Error('external resource forbidden'))});
    results.push({file,...result.issues});
}
const report=id==='aurago-low-poly'?'reports/low-poly':'reports/game-maker-worlds';
await fs.mkdir(report,{recursive:true});
await fs.writeFile(report+'/gltf-validator.json',JSON.stringify({validator:validator.version(),files:results},null,2)+'\n');
const errors=results.reduce((n,r)=>n+r.numErrors,0),warnings=results.reduce((n,r)=>n+r.numWarnings,0);
console.log(files.size+' GLBs: '+errors+' errors, '+warnings+' warnings ('+validator.version()+')');
if(errors)process.exitCode=1;
