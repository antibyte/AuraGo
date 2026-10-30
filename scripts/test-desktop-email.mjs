import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import vm from 'node:vm';
import {adaptNasscadEmail} from './nasscad-email-adapter.mjs';

const source=await readFile(new URL('../ui/js/desktop/core/email-export-runtime.js',import.meta.url),'utf8');
const words=JSON.parse(await readFile(new URL('../ui/lang/desktop/en.json',import.meta.url),'utf8'));
const state={windows:new Map(['a','b','vault','store'].map(id=>[id,{appId:id==='vault'?'tresor':id==='store'?'custom':'writer'}])),windowCleanups:new Map()};
let body,opened,notices,requests,choice,readonly,read;
const prefs=new Map();
const document=new EventTarget();
const context={window:{},document,state,File,Blob,URL,DOMException,AbortController,queueMicrotask,
    location:{origin:'http://desktop.test'},localStorage:{getItem:key=>prefs.get(key),setItem:(key,value)=>prefs.set(key,value)},
    t:(key,values={})=>Object.entries(values).reduce((text,[name,value])=>text.replaceAll('{{'+name+'}}',value),words[key]||key),
    desktopReadonly:()=>readonly,isBuiltinApp:id=>id!=='custom',
    registerWindowCleanup:(id,fn)=>{const list=state.windowCleanups.get(id)||[];list.push(fn);state.windowCleanups.set(id,list);},
    api:async(url,options)=>{requests.push(url);if(options.signal.aborted)throw new DOMException('aborted','AbortError');return structuredClone(body);},
    openApp:(app,options)=>opened.push({app,options}),showDesktopNotification:item=>notices.push(item),modalDialog:async()=>choice,
    fetch:async(...args)=>read(...args)
};
vm.createContext(context);vm.runInContext(source,context);
const mail=context.window.AuraDesktopEmail;
function reset(){
    opened=[];notices=[];requests=[];choice='txt';readonly=false;prefs.clear();
    body={enabled:true,readonly:false,accounts:[{id:'blocked',capabilities:['read']},{id:'one',capabilities:['send']},{id:'two',capabilities:['send']}],attachment_limits:{max_file_bytes:25<<20,max_message_bytes:25<<20}};
}
const text=(name='draft.txt')=>new File(['current unsaved text'],name,{type:'text/plain;charset=utf-8'});
const options=(extra={})=>({formats:['txt'],createFiles:async()=>[text()],...extra});
let count=0;
async function check(name,fn){reset();await fn();count++;console.log('ok '+name);}

await check('opens a new composer with exact bytes, metadata and an empty recipient/body draft',async()=>{
    const files=[text('Überblick.txt'),new File([new Uint8Array([0,255,80,75])],'model.3mf',{type:'model/3mf'})];
    assert.equal(await mail.open('a',options({subject:'Report\n2026',createFiles:async()=>files})),true);
    assert.equal(opened.length,1);const launch=opened[0].options;
    assert.equal(launch.forceNew,true);assert.equal(launch.compose,true);assert.equal(launch.accountId,'one');
    assert.equal(launch.draft.subject,'Report 2026');assert.equal(launch.draft.to,undefined);assert.equal(launch.draft.text,undefined);
    assert.equal(launch.exportFiles,files);assert.deepEqual([...new Uint8Array(await launch.exportFiles[1].arrayBuffer())],[0,255,80,75]);
    assert.deepEqual(requests,['/api/desktop/mailer/state','/api/desktop/mailer/state']);assert.equal(state.windowCleanups.get('a').length,0);
});
await check('chooses the last send-capable account and respects account settings',async()=>{
    mail.rememberAccount('two');body.account_settings={one:{read_only:true}};
    await mail.open('a',options());assert.equal(opened[0].options.accountId,'two');assert.equal(opened[0].options.state.accounts[1].read_only,true);
});
for(const variant of ['disabled','readonly','no-account']) await check(variant+' is refused before export',async()=>{
    if(variant==='disabled')body.enabled=false;if(variant==='readonly')readonly=true;if(variant==='no-account')body.accounts=[];
    let exported=false;await mail.open('a',options({createFiles:async()=>{exported=true;return [text()];}}));
    assert.equal(exported,false);assert.equal(opened.length,0);
});
await check('setup is an explicit navigation action, without enabling or sending',async()=>{
    body.enabled=false;choice='setup';await mail.open('a',options());assert.equal(opened[0].options.showSettings,true);assert.equal(opened[0].options.compose,undefined);assert.equal(requests.length,1);
});
await check('format cancellation and exporter cancellation create no draft',async()=>{
    choice=false;await mail.open('a',options({formats:['txt','html']}));await mail.open('a',options({createFiles:async()=>false}));
    assert.equal(opened.length,0);assert.equal(notices.length,0);
});
await check('permissions are rechecked after generation',async()=>{
    await mail.open('a',options({createFiles:async()=>{body.readonly=true;return [text()];}}));assert.equal(opened.length,0);
});
await check('per-file and aggregate limits use server budgets',async()=>{
    body.attachment_limits={max_file_bytes:8,max_message_bytes:10};
    await mail.open('a',options({createFiles:async()=>[new File(['123456789'],'large')]}));assert.equal(opened.length,0);
    await mail.open('a',options({createFiles:async()=>[new File(['123456'],'one'),new File(['12345'],'two')]}));assert.equal(opened.length,0);
    await mail.open('a',options({createFiles:async()=>[new File(['12345'],'one'),new File(['12345'],'two')]}));assert.equal(opened.length,1);
});
await check('source changes, cancellation and disposal drop late exports',async()=>{
    let current=true;await mail.open('a',options({isCurrent:()=>current,createFiles:async()=>{current=false;return [text()];}}));
    const life=new AbortController();await mail.open('a',options({signal:life.signal,createFiles:async()=>{life.abort();return [text()];}}));
    assert.equal(opened.length,0);assert.equal(notices.length,0);
});
await check('double clicks are coalesced and different windows remain independent',async()=>{
    let release;const gate=new Promise(resolve=>release=resolve);
    const first=mail.open('a',options({createFiles:async()=>{await gate;return [text('a.txt')];}}));
    assert.equal(await mail.open('a',options()),false);await mail.open('b',options({subject:'B'}));release();await first;
    assert.equal(opened.length,2);assert.equal(opened[0].options.draft.subject,'B');assert.equal(opened[1].options.exportFiles[0].name,'a.txt');
});
await check('Tresor and Store windows cannot invoke the handoff',async()=>{
    await mail.open('vault',options());await mail.open('store',options());assert.equal(opened.length,0);assert.equal(requests.length,0);
});
await check('session restoration retains no transient files or compose launch data',async()=>{
    const session=await readFile(new URL('../ui/js/desktop/core/session-runtime.js',import.meta.url),'utf8');
    const c={launch:{path:'Documents/source.txt',compose:true,exportFiles:[text()],draft:{subject:'Private'}}};
    vm.createContext(c);vm.runInContext(session.slice(0,session.indexOf('    function captureSessionSnapshot()')),c);
    const restored=JSON.parse(vm.runInContext('JSON.stringify(sanitizeSessionContext(launch))',c));
    assert.deepEqual(restored,{path:'Documents/source.txt'});
});
await check('downloads reject foreign origins, arbitrary APIs, redirects and unbounded bodies',async()=>{
    let fetched=0;read=async(_url,opts)=>{fetched++;assert.equal(opts.redirect,'error');return new Response('abc',{headers:{'Content-Type':'application/pdf'}});};
    await assert.rejects(()=>mail.file('https://foreign.test/file','x'));
    await assert.rejects(()=>mail.file('/api/config','x'));assert.equal(fetched,0);
    const file=await mail.file('/api/desktop/download?path=Documents/a.pdf','a.pdf');assert.equal(file.type,'application/pdf');assert.equal(await file.text(),'abc');
    assert.equal(await (await mail.file('/api/game-maker/projects/game-1/export','game.zip')).text(),'abc');
    await assert.rejects(()=>mail.file('/api/game-maker/config','x'));
    read=async()=>new Response('abc',{headers:{'Content-Length':String(26<<20)}});await assert.rejects(()=>mail.file('/files/a','a'));
    read=async()=>new Response(new ReadableStream({start(controller){controller.enqueue(new Uint8Array(26<<20));controller.close();}}));
    await assert.rejects(()=>mail.file('/files/a','a'));
});
await check('NASSCAD capture is scoped and never falls through to a download after cancellation',async()=>{
    const sample="async function _nasDownload(filename, blobOrString, mimeType){ regularDownloads++; }\nfunction saveProject(list){\n  showSpinner('Save Project','Preparing…');\n  requestAnimationFrame(()=>{_nasDownload('scene.nasscad','current scene','application/json');});}\n</body>";
    const patched=adaptNasscadEmail('index.html',sample);const script=patched.match(/<script>([\s\S]*?)<\/script>/)[1];
    const c={objs:[{}],File,queueMicrotask,requestAnimationFrame:queueMicrotask,showSpinner:()=>{},regularDownloads:0};c.window=c;vm.createContext(c);
    vm.runInContext(patched.split('</body>')[0].split('<script>')[0],c);vm.runInContext(script,c);
    const api=c.window.AuraNasscadEmail,files=await api.createFiles('project');assert.equal(await files[0].text(),'current scene');assert.equal(c.regularDownloads,0);
    vm.runInContext("_nasDownload('manual.txt','manual','text/plain')",c);assert.equal(c.regularDownloads,1);
    let resume;const wait=new Promise(resolve=>resume=resolve);c.expSTL=async()=>{await wait;await c._nasDownload('shape.stl','solid','model/stl');};
    const life=new AbortController(),pending=api.createFiles('stl',life.signal);life.abort();resume();await assert.rejects(()=>pending);assert.equal(c.regularDownloads,1);assert.equal(api.active,false);
});

console.log(`desktop-email: ${count} checks passed`);
