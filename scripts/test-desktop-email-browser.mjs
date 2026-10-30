import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {Readable} from 'node:stream';
import {readFile,mkdir} from 'node:fs/promises';
import {createRequire} from 'node:module';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const require=createRequire(import.meta.url);
const {chromium}=require(process.env.AURAGO_PLAYWRIGHT||'playwright');
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const words={...JSON.parse(await readFile(path.join(root,'ui/lang/desktop/en.json'))),...JSON.parse(await readFile(path.join(root,'ui/lang/mailer/en.json')))};
const runtime=await readFile(path.join(root,'ui/js/desktop/core/email-export-runtime.js'),'utf8');
const routing=await readFile(path.join(root,'ui/js/desktop/core/menus-and-routing.js'),'utf8');
const modal=routing.slice(routing.indexOf('    function modalDialog(options)'),routing.indexOf('    async function createFileInPath'));
let drafts=new Map(),uploads=[],sendRequests=0,failUpload=false;
const mailState={enabled:true,readonly:false,accounts:[{id:'one',name:'Work',address:'owner@example.test',capabilities:['send']},{id:'two',name:'Home',address:'home@example.test',capabilities:['send']}],contacts:[],attachment_limits:{max_file_bytes:25<<20,max_message_bytes:25<<20}};
const server=createServer(async(req,res)=>{
    try {
        const url=new URL(req.url,'http://localhost'),p=url.pathname;
        const json=value=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify(value));};
        if(p==='/') {res.setHeader('Content-Type','text/html');res.end('<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-common.css"><link rel="stylesheet" href="/css/desktop-app-mailer.css"><style>body{margin:0;font:14px system-ui}#a{padding:12px}#composers>div{position:absolute;inset:70px 0 0;background:var(--vd-theme-app-bg)}</style><div id="a"><div data-toolbar></div><textarea>Unsaved document — äöü</textarea></div><main id="composers"></main>');return;}
        if(p==='/api/desktop/mailer/state')return json(mailState);
        if(p.startsWith('/api/desktop/mailer/outbox')){sendRequests++;throw Error('Unexpected send');}
        if(p.includes('/attachments')&&req.method==='POST'){
            if(failUpload){failUpload=false;res.statusCode=503;return json({error:'upload_failed'});}
            const id=p.split('/')[5],draft=drafts.get(id);
            assert.equal(Number(url.searchParams.get('revision')),draft.revision);
            const form=await new Request('http://localhost'+p,{method:'POST',headers:req.headers,body:Readable.toWeb(req),duplex:'half'}).formData();
            const file=form.get('file');assert(file instanceof File);
            const bytes=[...new Uint8Array(await file.arrayBuffer())];uploads.push({name:file.name,type:file.type,bytes});
            draft.attachments.push({id:'attachment-'+uploads.length,filename:file.name,content_type:file.type,size:file.size});draft.revision++;
            return json(draft);
        }
        if(p.startsWith('/api/desktop/mailer/drafts')&&['POST','PUT'].includes(req.method)){
            let data='';for await(const chunk of req)data+=chunk;
            const payload=JSON.parse(data),id=payload.id||'draft-'+(drafts.size+1),previous=drafts.get(id);
            if(previous)assert.equal(payload.revision,previous.revision);
            const draft={...payload,id,revision:(previous?.revision||0)+1,attachments:previous?.attachments||[]};drafts.set(id,draft);return json(draft);
        }
        if(p.startsWith('/api/desktop/mailer/'))return json({items:[]});
        const file=path.resolve(root,'ui','.'+p);if(!file.startsWith(path.join(root,'ui')+path.sep))throw Error('path');
        res.setHeader('Content-Type',p.endsWith('.css')?'text/css':p.endsWith('.js')?'text/javascript':'application/octet-stream');res.end(await readFile(file));
    }catch(error){res.statusCode=500;res.setHeader('Content-Type','application/json');res.end(JSON.stringify({error:error.message}));}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const origin='http://127.0.0.1:'+server.address().port;
const browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHANNEL?{channel:process.env.PLAYWRIGHT_CHANNEL}:{})});
const report=path.join(root,'ui/reports/desktop-email');await mkdir(report,{recursive:true});
try {
    for(const theme of ['standard','fruity'])for(const mode of ['light','dark'])for(const width of [390,1280]){
        drafts=new Map();uploads=[];const page=await browser.newPage({viewport:{width,height:900}}),errors=[];
        page.on('pageerror',error=>errors.push(error.message));await page.goto(origin);
        for(const file of ['marked.min.js','purify.min.js'])await page.addScriptTag({url:origin+'/js/vendor/'+file});
        for(const file of ['mailer-api','mailer-views','mailer-compose'])await page.addScriptTag({url:origin+'/js/desktop/apps/'+file+'.js'});
        await page.evaluate(({words,theme,mode})=>{
            document.body.className='desktop-body';document.body.dataset.theme=theme;document.documentElement.dataset.theme=mode;
            const css=document.documentElement.style;for(const[name,value]of Object.entries({'--vd-theme-app-bg':mode==='light'?'#f5f6fa':'#111823','--vd-theme-panel-bg':mode==='light'?'#fff':'#1a2331','--vd-theme-control-bg':mode==='light'?'#edf0f5':'#263044','--vd-theme-border':mode==='light'?'#ccd1db':'#374357','--vd-text':mode==='light'?'#202b3a':'#edf2f8','--vd-accent':'#3679d3'}))css.setProperty(name,value);
            window.state={windows:new Map([['a',{appId:'writer'}]]),windowCleanups:new Map()};
            window.desktopModalQueue=Promise.resolve();window.closeContextMenu=()=>{};window.desktopSound=()=>{};
            window.t=(key,params={})=>Object.entries(params).reduce((text,[k,v])=>text.replaceAll('{{'+k+'}}',v),words[key]||key);
            window.esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
            window.mountDesktopDialog=el=>{document.body.append(el);return()=>el.remove();};
            window.desktopReadonly=()=>false;window.isBuiltinApp=()=>true;window.contentEl=id=>document.getElementById(id);
            window.registerWindowCleanup=(id,fn)=>{const list=state.windowCleanups.get(id)||[];list.push(fn);state.windowCleanups.set(id,list);};
            window.api=async(url,options={})=>{const response=await fetch(url,options),body=await response.json();if(!response.ok)throw Object.assign(Error(body.error),{status:response.status});return body;};
            window.notifications=[];window.showDesktopNotification=item=>notifications.push(item);window.composers=[];window.launches=[];
            window.openApp=(app,options)=>{
                assertEmailApp(app);const id='composer-'+composers.length,host=document.createElement('div');host.id=id;document.querySelector('#composers').append(host);state.windows.set(id,{appId:'mailer'});launches.push(options);
                const ctx={...options,t,api,readonly:false,confirmDialog:async()=>true,setWindowBeforeClose:()=>{},registerWindowCleanup};
                composers.push(MailerCompose.render(host,id,ctx,options));
            };
            function assertEmailApp(app){if(app!=='mailer')throw Error('unexpected app');}
        },{words,theme,mode});
        await page.addScriptTag({content:modal});await page.addScriptTag({content:runtime});
        await page.evaluate(()=>{
            localStorage.setItem('aurago.desktop.mailer.prefs',JSON.stringify({accountId:'two'}));
            AuraDesktopEmail.attach('a','[data-toolbar]',()=>({key:document.querySelector('#a textarea'),subject:'Document',formats:['txt','bin'],createFiles:async format=>[
                format==='txt'?new File([document.querySelector('#a textarea').value],'Überblick.txt',{type:'text/plain;charset=utf-8'}):new File([new Uint8Array([0,255,80,75])],'Model.3mf',{type:'model/3mf'})
            ]}));
        });
        const button=page.locator('#a [data-email-export]');await button.click();
        await page.locator('.vd-modal select').selectOption('bin');await page.locator('.vd-modal button[type="submit"]').click();
        await page.waitForFunction(()=>composers[0]?.draft.attachments.length===1&&!composers[0].attachmentBusy);
        assert.deepEqual(uploads[0],{name:'Model.3mf',type:'model/3mf',bytes:[0,255,80,75]});
        assert.equal(await page.evaluate(()=>composers[0].accountId),'two');
        assert.equal(await page.evaluate(()=>launches[0].exportFiles),undefined);
        assert.deepEqual(await page.evaluate(()=>({to:composers[0].recipients.to,subject:composers[0].draft.subject,text:composers[0].draft.text})),{to:[],subject:'Document',text:''});
        await page.evaluate(()=>{document.querySelector('#composers').hidden=true;document.querySelector('#a textarea').value='Changed after the first handoff';});
        await button.click();await page.locator('.vd-modal select').selectOption('txt');await page.locator('.vd-modal button[type="submit"]').click();
        await page.waitForFunction(()=>composers[1]?.draft.attachments.length===1&&!composers[1].attachmentBusy);
        assert.equal(Buffer.from(uploads[1].bytes).toString(),'Changed after the first handoff');
        assert.equal(await page.evaluate(()=>composers[0].draft.attachments[0].filename),'Model.3mf');
        await page.evaluate(()=>{document.querySelector('#composers').hidden=false;document.querySelector('#composers>div:first-child').hidden=true;});
        await page.screenshot({path:path.join(report,`${theme}-${mode}-${width}.png`)});
        failUpload=true;await page.evaluate(()=>{document.querySelector('#composers').hidden=true;});
        await button.click();await page.locator('.vd-modal select').selectOption('bin');await page.locator('.vd-modal button[type="submit"]').click();
        await page.waitForFunction(()=>composers[2]?.failedUploads.size===1&&!composers[2].attachmentBusy);
        assert.equal(await page.evaluate(()=>composers[2].host.querySelector('[data-send]').disabled),true);
        await page.evaluate(()=>composers[2].host.querySelector('[data-retry-upload]').click());
        await page.waitForFunction(()=>composers[2]?.draft.attachments.length===1&&!composers[2].failedUploads.size);
        assert.equal(sendRequests,0);assert.deepEqual(errors,[]);
        for(const inst of await page.evaluate(()=>composers.map(c=>c.windowId)))await page.evaluate(id=>MailerCompose.dispose(id),inst);
        await page.close();console.log(`ok desktop email ${theme}/${mode}/${width}`);
    }
}finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
console.log('Desktop email browser handoff, native format dialog, snapshots and retry passed; no email sent.');
