import fs from 'node:fs';
import assert from 'node:assert/strict';
import path from 'node:path';
import { createRequire } from 'node:module';

const root='ui/js/vendor/layerling';
const manifest=JSON.parse(fs.readFileSync(root+'/vendor.json'));
const base='/api/desktop/layerling/ui';
assert.equal(manifest.base,base);
const walk=dir=>fs.readdirSync(dir,{withFileTypes:true}).flatMap(e=>e.isDirectory()?walk(path.join(dir,e.name)):[path.join(dir,e.name)]);
const actual=walk(root).map(f=>path.relative(root,f).replaceAll('\\','/')).filter(f=>f!=='vendor.json').sort();
assert.deepEqual(actual,Object.keys(manifest.files).sort(),'unmanifested vendor resources');
const packaged=new Set(JSON.parse(fs.readFileSync('assets/web-assets.json')).roots.find(r=>r.source==='ui').extensions);
for(const file of actual)assert(packaged.has(path.extname(file))||path.basename(file).startsWith('LICENSE'),file+' excluded from the verified resource pack');
assert(!actual.includes('store.php')&&!actual.includes('sitemap.xml'),'upstream server/site files shipped');
let workers=0;
for(const file of actual.filter(f=>f.startsWith('_next/static/chunks/')&&f.endsWith('.js'))) {
    const source=fs.readFileSync(root+'/'+file,'utf8');
    if(source.includes('importScripts(')&&source.includes('static/chunks/')) {
        workers++;
        assert(source.includes('.p="'+base+'/_next/"'),file+' worker public path');
    }
}
assert(workers>=2,'CAD workers missing');
assert(actual.some(f=>f.endsWith('.wasm')),'WASM absent');
assert(actual.some(f=>f.startsWith('guide/')&&f.endsWith('.html')),'offline help absent');
const html=fs.readFileSync(root+'/index.html','utf8');
for(const match of html.matchAll(/(?:src|href)="([^"#]+)"/g)) {
    const url=match[1];
    if(!url.startsWith(base+'/'))continue;
    const asset=url.slice(base.length+1).split('?')[0];
    if(asset)assert(fs.existsSync(root+'/'+asset),asset+' referenced by entry HTML');
}
const unscoped=new Set();
for(const file of actual.filter(f=>f.endsWith('.html')))for(const match of fs.readFileSync(root+'/'+file,'utf8').matchAll(/(?:src|href)="(\/[^"#]*)"/g)){
    if(!match[1].startsWith(base+'/'))unscoped.add(match[1]);
}
assert.equal(unscoped.size,0,'unscoped static document links: '+[...unscoped].slice(0,10).join(', '));
for(const locale of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']) {
    const desktop=JSON.parse(fs.readFileSync('ui/lang/desktop/'+locale+'.json'));
    const config=JSON.parse(fs.readFileSync('ui/lang/config/'+locale+'.json'));
    const english=JSON.parse(fs.readFileSync('ui/lang/desktop/en.json'));
    for(const key of Object.keys(english).filter(k=>k.startsWith('layerling.')))assert(desktop[key],locale+' missing '+key);
    for(const key of ['enabled','help','access','access_help','off','read','write'])assert(config['config.layerling.'+key],locale+' config '+key);
    if(locale!=='en')assert.notEqual(desktop['layerling.recover_help'],english['layerling.recover_help'],locale+' copied English');
}
const extracted='disposable/_layerling/layerling-'+manifest.commit;
if(fs.existsSync(extracted+'/node_modules/fflate')) {
    const {unzipSync}=createRequire(path.resolve(extracted,'package.json'))('fflate');
    const archive=unzipSync(fs.readFileSync(root+'/source.zip'),{filter:e=>['layerling/LICENSE','layerling/package-lock.json','aurago/scripts/layerling/bridge.ts','aurago/scripts/layerling/page.tsx','AURAGO-BUILD.txt'].includes(e.name)});
    assert.equal(Object.keys(archive).length,5,'corresponding source missing');
    for(const name of ['bridge.ts','page.tsx'])assert.equal(Buffer.from(archive['aurago/scripts/layerling/'+name]).toString('utf8'),fs.readFileSync('scripts/layerling/'+name,'utf8').replaceAll('\r\n','\n'),'source archive adapter drift');
}
console.log('Layerling workers, resources, source archive and all 16 locales verified.');
