import { build } from 'esbuild';
import { readFile, writeFile, mkdir, readdir } from 'node:fs/promises';
import path from 'node:path';
import { createHash } from 'node:crypto';

const out='ui/js/vendor/sheets',check=process.argv.includes('--check'),outputs=new Map(),packages=new Set();
const presets=['core','filter','sort','data-validation','conditional-formatting','find-replace','note','hyper-link','table'];
const localeNames=['en-US','de-DE','es-ES','fr-FR','it-IT','ja-JP','pl-PL','pt-BR','zh-CN'];
const entry=await readFile('scripts/sheets-engine-entry.js','utf8');
let localeCode=presets.map(p=>"import '@univerjs/preset-sheets-"+p+"/lib/index.css';").join('\n')+'\n',localeExports=[];
for(const locale of localeNames){
    const imports=presets.map((preset,index)=>{const name='l'+locale.replaceAll('-','')+index;localeCode+="import "+name+" from '@univerjs/preset-sheets-"+preset+"/locales/"+locale+"';\n";return name;});
    localeExports.push(JSON.stringify(locale)+': mergeLocales('+imports.join(',')+')');
}
localeCode+='export const locales={'+localeExports.join(',')+'};\n';
for(const [name,options] of [['engine',{stdin:{contents:entry+'\n'+localeCode,resolveDir:process.cwd(),sourcefile:'sheets-engine-entry.js'}}],['worker',{entryPoints:['scripts/sheets-worker-entry.js']}]]){
    const result=await build({...options,bundle:true,write:false,metafile:true,format:name==='worker'?'iife':'esm',target:'es2022',minify:true,legalComments:'inline',outfile:out+'/'+name+'.js',define:{'process.env.NODE_ENV':'"production"'}});
    for(const item of result.outputFiles)outputs.set(path.basename(item.path),Buffer.from(item.contents));
    for(const file of Object.keys(result.metafile.inputs)){
        const rest=file.replaceAll('\\','/').split('node_modules/')[1];
        if(rest)packages.add(rest.startsWith('@')?rest.split('/').slice(0,2).join('/'):rest.split('/')[0]);
    }
}
const versions={},notices=['# Tabellen — third-party licenses\n\nAuraGo host code remains MIT. Univer OSS is pinned to 0.25.1; no Pro packages are included.\n'];
for(const name of [...packages].sort()){
    const root='node_modules/'+name,pkg=JSON.parse(await readFile(root+'/package.json','utf8'));
    const licenseFiles=(await readdir(root,{withFileTypes:true})).filter(f=>f.isFile()&&/^(license|copying|notice)/i.test(f.name));
    const licenseTexts=await Promise.all(licenseFiles.map(f=>readFile(root+'/'+f.name,'utf8')));
    const license=pkg.license||(licenseTexts.some(text=>/Apache License\s+Version 2\.0/.test(text))?'Apache-2.0':null);
    if(name.startsWith('@univerjs-pro/')||!new Set(['MIT','Apache-2.0','ISC','BSD-3-Clause','0BSD']).has(license))throw Error('Unverified or disallowed dependency license: '+name);
    if(name.startsWith('@univerjs/') && name!=='@univerjs/icons' && pkg.version!=='0.25.1')throw Error('Mixed Univer versions: '+name);
    versions[name]={version:pkg.version,license,...(!pkg.license?{licenseSource:licenseFiles.map(f=>f.name)}:{})};notices.push('\n## '+name+' '+pkg.version+' — '+license+'\n',...licenseTexts);
}
outputs.set('LICENSES.md',Buffer.from(notices.join('\n').replaceAll('\r\n','\n')));
const hashes={};for(const [name,data]of outputs)hashes[name]={bytes:data.length,sha256:createHash('sha256').update(data).digest('hex')};
outputs.set('manifest.json',Buffer.from(JSON.stringify({version:1,packages:versions,files:hashes},null,2)+'\n'));
await mkdir(out,{recursive:true});
for(const [name,data]of outputs){const file=out+'/'+name;if(check){if(!data.equals(await readFile(file)))throw Error('Stale Sheets asset: '+file);}else await writeFile(file,data);}
console.log('Sheets vendor '+(check?'verified':'built')+': '+outputs.size+' assets, '+packages.size+' permissive packages');
