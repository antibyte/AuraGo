import { build } from 'esbuild';
import { readFile,writeFile,mkdir,readdir } from 'node:fs/promises';
import path from 'node:path';
import { createHash } from 'node:crypto';
const out='ui/js/vendor/notes',check=process.argv.includes('--check'),outputs=new Map(),packages=new Set();
const result=await build({entryPoints:['scripts/notes-engine-entry.js'],bundle:true,write:false,metafile:true,format:'esm',target:'es2022',minify:true,legalComments:'inline',outfile:out+'/engine.js',loader:{'.woff':'dataurl','.woff2':'dataurl','.ttf':'dataurl'},define:{'process.env.NODE_ENV':'"production"'}});
for(const item of result.outputFiles)outputs.set(path.basename(item.path),Buffer.from(item.contents));
for(const file of Object.keys(result.metafile.inputs)){
 const rest=file.replaceAll('\\','/').split('node_modules/')[1];
 if(rest)packages.add(rest.startsWith('@')?rest.split('/').slice(0,2).join('/'):rest.split('/')[0]);
}
const versions={},notices=['# Notizen — third-party licenses\n\nAuraGo remains MIT. Milkdown/Crepe is pinned to 7.22.1.\n'];
for(const name of [...packages].sort()){
 const root='node_modules/'+name,pkg=JSON.parse(await readFile(root+'/package.json','utf8'));
 if(name.startsWith('@milkdown/')&&pkg.version!=='7.22.1')throw Error('Mixed Milkdown versions: '+name);
 const license=pkg.license==='(MPL-2.0 OR Apache-2.0)'?'Apache-2.0':pkg.license;
 if(!new Set(['MIT','Apache-2.0','ISC','BSD-2-Clause','BSD-3-Clause','(MIT AND BSD-3-Clause)','0BSD']).has(license))throw Error('Unverified license: '+name+' '+pkg.license);
 versions[name]={version:pkg.version,license,upstreamLicense:pkg.license};
 notices.push('\n## '+name+' '+pkg.version+' — '+pkg.license+'\n');
 for(const f of await readdir(root,{withFileTypes:true}))if(f.isFile()&&/^(license|copying|notice)/i.test(f.name))notices.push(await readFile(root+'/'+f.name,'utf8'));
}
outputs.set('LICENSES.md',Buffer.from(notices.join('\n').replaceAll('\r\n','\n').replace(/[ \t]+$/gm,'')));
const hashes={};for(const [name,data]of outputs)hashes[name]={bytes:data.length,sha256:createHash('sha256').update(data).digest('hex')};
outputs.set('manifest.json',Buffer.from(JSON.stringify({version:1,packages:versions,files:hashes},null,2)+'\n'));
await mkdir(out,{recursive:true});
for(const [name,data]of outputs){const file=out+'/'+name;if(check){if(!data.equals(await readFile(file)))throw Error('Stale Notes asset: '+file);}else await writeFile(file,data);}
console.log('Notes vendor '+(check?'verified':'built')+': '+outputs.size+' assets, '+packages.size+' permissive packages');
