// Package the raster catalog; prototype output is isolated from the installed catalog.
import fs from 'node:fs';
import path from 'node:path';
import { buildRasterOutputs } from '../assets/storymaker/lib/raster-catalog.mjs';
const root=path.resolve(import.meta.dirname,'..');
const prototype=process.argv.includes('--prototype'),check=process.argv.includes('--check');
if(!prototype){
    await import('./build-storymaker-catalog.mjs');
}else{
const directory=path.join(root,prototype?'ui/reports/storymaker-raster/prototype':'ui/img/storymaker');
const built=buildRasterOutputs({prototype});
for(const [rel,value]of built.files){
    const file=path.join(directory,rel),bytes=Buffer.isBuffer(value)?value:Buffer.from(value);
    if(check){if(!fs.existsSync(file)||!fs.readFileSync(file).equals(bytes))throw new Error(`Stale raster output: ${rel}`);}
    else{fs.mkdirSync(path.dirname(file),{recursive:true});fs.writeFileSync(file,bytes);}
}
console.log(`Storymaker raster ${built.manifest.version}: ${built.entries.filter(e=>e.kind==='character').length} characters, ${built.entries.filter(e=>e.kind==='environment').length} environments, ${built.entries.filter(e=>e.kind==='prop').length} props; ${directory}`);
}
