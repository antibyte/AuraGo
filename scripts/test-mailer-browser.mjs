import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFile, mkdir} from 'node:fs/promises';
import {createRequire} from 'node:module';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const require = createRequire(import.meta.url);
const runtimeEnv = globalThis.process?.env || {};
const {chromium} = require(runtimeEnv.AURAGO_PLAYWRIGHT || globalThis.AURAGO_PLAYWRIGHT || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const words = {...JSON.parse(await readFile(path.join(root, 'ui/lang/desktop/en.json'), 'utf8')), ...JSON.parse(await readFile(path.join(root, 'ui/lang/mailer/en.json'), 'utf8'))};
const report = path.join(root, 'reports/mailer/browser');
await mkdir(report, {recursive:true});
const server = createServer(async (req,res) => {
    try {
        const name = new URL(req.url,'http://localhost').pathname;
        if (name === '/') { res.setHeader('Content-Type','text/html'); res.end('<!doctype html><meta charset="utf-8"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden}#host{position:absolute;inset:0}#compose{position:absolute;inset:0;z-index:5}</style><div id="host"></div><div id="compose" hidden></div>'); return; }
        const file = path.resolve(root,'ui','.'+name);
        if (!file.startsWith(path.join(root,'ui')+path.sep)) throw Error('invalid path');
        res.setHeader('Content-Type',name.endsWith('.css')?'text/css':name.endsWith('.svg')?'image/svg+xml':name.endsWith('.woff2')?'font/woff2':'text/javascript');
        res.end(await readFile(file));
    } catch {res.statusCode=404;res.end();}
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({headless:true, ...((runtimeEnv.PLAYWRIGHT_CHANNEL || globalThis.PLAYWRIGHT_CHANNEL)?{channel:runtimeEnv.PLAYWRIGHT_CHANNEL || globalThis.PLAYWRIGHT_CHANNEL}:{}), ...((runtimeEnv.AURAGO_CHROMIUM_EXECUTABLE || globalThis.AURAGO_CHROMIUM_EXECUTABLE)?{executablePath:runtimeEnv.AURAGO_CHROMIUM_EXECUTABLE || globalThis.AURAGO_CHROMIUM_EXECUTABLE}:{})});
try {
    for (const theme of ['standard','fruity']) for (const mode of ['dark','light']) for (const width of [1440,900,390]) {
        const page = await browser.newPage({viewport:{width,height:900}});
        const errors=[], writes=[], network=[];
        page.on('pageerror',e=>errors.push(e.message));
        page.on('request',r=>network.push(r.url()));
        const accounts=[{id:'email:work',source:'email',protocol:'imap',name:'Work',address:'owner@example.test',linked:true,reauthorize_supported:true,capabilities:['read','send','read_state','star','move','archive','trash','restore','delete','folders_read','folders_create']},{id:'email:private',source:'email',protocol:'pop3',name:'Personal',address:'personal@example.test',linked:true,capabilities:['read','search','folders_read']}];
        const folders=accounts.flatMap(a=>['inbox','archive','trash','sent'].map(kind=>({id:a.id+':'+kind,account_id:a.id,provider_id:kind,name:kind[0].toUpperCase()+kind.slice(1),kind})));
        const state={enabled:true,settings:{enabled:true},accounts,sources:accounts,folders,account_settings:{},templates:[],signatures:[],rules:[],contacts:[{name:'Alice Example',email:'alice@example.test'}],providers:[{id:'fixture',name:'Local assistant',model:'test-model'}],default_provider_id:'fixture',assist_guardian:{id:'scanner',name:'Security scanner',model:'scan-model'},assist_policy:'fixture-policy',readonly:false};
        const rows=Array.from({length:40},(_,i)=>({id:'thread-'+i,message_id:'message-'+i,account_id:accounts[i%2].id,folder_id:accounts[i%2].id+':inbox',from:i%2?'Mara Example <mara@example.test>':'Alice Example <alice@example.test>',to:['owner@example.test'],participants:i%2?['owner@example.test','mara@example.test']:['"Alice Example" <alice@example.test>','owner@example.test','notifications@example.test'],subject:i===0?'Project update and next steps':'Mailbox fixture '+i,preview:'A clear preview of the selected conversation.',date:new Date(Date.UTC(2026,9,16,12,0,-i)).toISOString(),unread:i<8,starred:i===2,message_count:1,attachment_count:0}));
        let draft=null, outbox=null, conflictNextSave=false, conflictLatest=null, uploadFailNext=false, uploadWaitEnabled=false, releaseUpload=null, uploadStartResolve=null;
        let outboxRecords=Array.from({length:54},(_,i)=>({id:`job-${i}`,status:i===0?'failed':i===1?'scheduled':'sent',updated_at:new Date(Date.UTC(2026,9,16,12,0,-i)).toISOString()}));
        let ruleRuns=[{message_id:'message-0',rule_id:'rule-failed',rule_revision:2,action_index:1,action_count:3,status:'failed',error:'Move action failed',updated_at:new Date().toISOString()},{message_id:'message-0',rule_id:'rule-unknown',rule_revision:1,action_index:0,action_count:2,status:'unknown',error:'Provider response was lost',updated_at:new Date().toISOString()}];
        const uploadStarted=new Promise(resolve=>uploadStartResolve=resolve);
        let uploadGate=new Promise(resolve=>releaseUpload=resolve);
        const json=value=>({status:200,contentType:'application/json',body:JSON.stringify(value)});
        await page.route('**/api/desktop/mailer/**',async route=>{
            const request=route.request(), u=new URL(request.url()), p=u.pathname.replace('/api/desktop/mailer/',''), method=request.method();
            const postData=request.postData();
            const data=postData&&request.headers()['content-type']?.includes('application/json')?JSON.parse(postData):{};
            if(method!=='GET') writes.push({p,method,data});
            if(p==='state') return route.fulfill(json(state));
            if(p==='threads') {
                if(u.searchParams.has('cursor')) { assert.equal(u.searchParams.get('cursor'),'next-page'); return route.fulfill(json({items:[],total:40000,next_cursor:'',search_scope:'local'})); }
                return route.fulfill(json({items:rows,total:40000,next_cursor:'next-page',search_scope:'local'}));
            }
            if(/^threads\/thread-\d+$/.test(p)) {
                const row=rows[Number(p.split('-').at(-1))];
                return route.fulfill(json({...row,messages:[{id:row.message_id,account_id:row.account_id,from:row.from,to:['owner@example.test'],cc:['cc@example.test'],subject:row.subject,message_id:'<fixture@example.test>',date:row.date,text:'Please review the next steps.\n\n> Previous message',html:'<p>Please review the next steps.</p><img src="https://tracker.invalid/pixel"><script>parent.__mailInjection=true</script><blockquote>Previous message</blockquote>',attachments:[]}]}));
            }
            if(p.startsWith('threads/')) return route.fulfill(json({ok:true}));
            if(p==='drafts'&&method==='GET') return route.fulfill(json({items:draft?[draft]:[]}));
            if(/^drafts\/[^/]+$/.test(p)&&method==='GET') return route.fulfill(json(draft?{...draft}:{}));
            if(/^drafts\/[^/]+\/attachments$/.test(p)&&method==='POST') {
                uploadStartResolve();
                if(uploadFailNext){uploadFailNext=false;return route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'upload_failed'})});}
                if(uploadWaitEnabled) await uploadGate;
                const attachment={id:`attachment-${(draft?.attachments||[]).length+1}`,filename:'fixture.txt',size:7};
                draft={...draft,revision:(draft?.revision||0)+1,attachments:[...(draft?.attachments||[]),attachment]};
                return route.fulfill(json(draft));
            }
            if((p==='drafts'||p.startsWith('drafts/'))&&(method==='POST'||method==='PUT')) {
                if(method==='PUT'&&conflictNextSave){conflictNextSave=false;draft=conflictLatest;return route.fulfill({status:409,contentType:'application/json',body:JSON.stringify({error:'draft_conflict'})});}
                draft={...data,id:'draft-fixture',revision:(draft?.revision||0)+1,attachments:draft?.attachments||[],updated_at:new Date().toISOString()}; return route.fulfill(json(draft));
            }
            if(p==='outbox'&&method==='POST') {outbox={id:'outbox-fixture',...data,status:'scheduled',undo_until:new Date(Date.now()+10000).toISOString()};outboxRecords.unshift(outbox);return route.fulfill(json(outbox));}
            if(p==='outbox'&&method==='GET') {const cursor=u.searchParams.get('cursor');return route.fulfill(json({items:cursor==='outbox-next'?outboxRecords.slice(50):outboxRecords.slice(0,50),next_cursor:cursor?'':'outbox-next',has_more:!cursor}));}
            if(p==='outbox/outbox-fixture'&&method==='DELETE') {outbox=null;outboxRecords=outboxRecords.filter(item=>item.id!=='outbox-fixture');return route.fulfill(json({ok:true}));}
            if(/^outbox\/job-\d+\/retry$/.test(p)&&method==='POST') return route.fulfill(json({ok:true}));
            if(/^outbox\/job-\d+$/.test(p)&&method==='DELETE') {outboxRecords=outboxRecords.filter(item=>item.id!==p.split('/')[1]);return route.fulfill(json({ok:true}));}
            if(/^messages\/message-\d+\/rule-runs$/.test(p)&&method==='GET') return route.fulfill(json({items:ruleRuns}));
            if(/^messages\/message-\d+\/rule-runs\/[^/]+\/retry$/.test(p)&&method==='POST') {const id=p.split('/').at(-2);ruleRuns=ruleRuns.map(run=>run.rule_id===id?{...run,status:'executing',error:''}:run);return route.fulfill(json({ok:true}));}
            if(p==='assist') { assert.equal(data.assist_policy,'fixture-policy'); return route.fulfill(json({preview:{text:'Suggested text',html:'<p>Suggested text</p>'},source_revision:data.revision,provider_id:'fixture',model:'test-model'})); }
            if(['templates','signatures','rules'].includes(p)) return route.fulfill(json({items:[]}));
            return route.fulfill(json({ok:true}));
        });
        await page.goto(origin);
        await page.addStyleTag({url:origin+'/css/desktop-shell.bundle.css'});
        await page.addStyleTag({url:origin+'/css/desktop-app-mailer.css'});
        await page.addStyleTag({url:origin+'/css/desktop-stickers.css'});
        await page.addScriptTag({url:origin+'/js/vendor/marked.min.js'});
        await page.addScriptTag({url:origin+'/js/vendor/purify.min.js'});
        for(const file of ['sticker-catalog','sticker-library']) await page.addScriptTag({url:origin+'/js/desktop/shared/'+file+'.js'});
        for(const file of ['mailer-api','mailer-views','mailer-stickers','mailer-compose','mailer-settings','mailer']) await page.addScriptTag({url:origin+'/js/desktop/apps/'+file+'.js'});
        await page.evaluate(({words,theme,mode,state})=>{
            document.body.className='desktop-body'; document.body.dataset.theme=theme; document.documentElement.dataset.theme=mode;
            document.documentElement.style.setProperty('--surface-color',mode==='light'?'#f5f6fa':'#111823');
            document.documentElement.style.setProperty('--text-color',mode==='light'?'#202b3a':'#edf2f8');
            document.documentElement.style.setProperty('--text-muted',mode==='light'?'#526174':'#9aa8ba');
            window.confirmations=[];
            window.ctx={state,readonly:false,t:(key,vars={})=>{let text=words[key]||key;for(const[k,v]of Object.entries(vars))text=text.replaceAll('{'+k+'}',v);return text;},api:async(url,options)=>{const r=await fetch(url,options);const value=await r.json();if(!r.ok)throw Object.assign(new Error(value.error),value,{status:r.status});return value;},confirmDialog:async(title,message)=>{confirmations.push({title,message});return true;},promptDialog:async()=>'',iconMarkup:()=>'<span aria-hidden="true">✉</span>',registerWindowCleanup:()=>{},setWindowBeforeClose:(id,cb)=>window.beforeClose=cb,closeWindow:()=>{document.querySelector('#compose').hidden=true;MailerCompose.dispose('composer');},openApp:(app,options)=>{const host=document.querySelector('#compose');host.hidden=false;MailerCompose.render(host,'composer',{...ctx,state:options.state||state},options);}};
            ctx.providers=[];
            MailerApp.render(document.querySelector('#host'),'mail',ctx,{});
        },{words,theme,mode,state});
        await page.locator('[data-thread-id]').first().waitFor();
        assert.equal(await page.locator('[data-thread-id]').count(),40,'large mailbox must only render the returned page');
        const avatarLetters=await page.locator('.vd-mailer-thread .vd-mailer-avatar').allTextContents();
        assert(avatarLetters.length===40,'every thread row must render a sender avatar');
        assert(avatarLetters.every(text=>/^[A-Z][A-Z]?$/.test(text.trim())),'avatar circles must only carry letters of the sender, got '+avatarLetters.map(text=>text.trim()).join(','));
        assert.equal(avatarLetters[0].trim(),'AE','the avatar must show the sender of the thread, not an arbitrary participant');
        assert.equal((await page.locator('.vd-mailer-thread-line b').first().textContent()).trim(),'Alice Example','the row label must name the sender');
        await page.locator('[data-thread-id]').first().focus(); await page.keyboard.press('Enter');
        await page.locator('[data-reader] iframe').first().waitFor({state:'visible'});
        const readerActions=await page.locator('.vd-mailer-reader-actions [data-reader-action]').count();
        assert(readerActions>=6,'the reader must offer its response and file actions');
        assert.equal(await page.locator('.vd-mailer-reader-actions [data-reader-action] svg.vd-mailer-icon').count(),readerActions,'every reader action must carry its icon');
        assert(await page.locator('.vd-mailer-reader-action-group').count()>=2,'the reader actions must be grouped instead of one flat pile');
        assert.equal(await page.locator('.vd-mailer-reader-actions [data-reader-action="reply"] span').count(),1,'the reply action must keep its label');
        assert.equal(await page.locator('.vd-mailer-reader-actions [data-reader-action="print"] span').count(),0,'utility actions must collapse into icon buttons');
        assert.equal(await page.locator('.vd-mailer-reader-actions [data-reader-action="print"]').getAttribute('aria-label'),words['mailer.print'],'icon-only actions must stay accessible');
        assert.equal((await page.locator('.vd-mailer-participant-chip').first().textContent()).trim(),'3 participants','the reader must report the thread roster');
        await page.locator('[data-rule-runs-for="message-0"] li').first().waitFor();
        assert.equal(await page.locator('[data-retry-rule-run]').count(),2,'failed and unknown rule runs must offer explicit retry controls');
        await page.locator('[data-retry-rule-run="rule-unknown"]').focus(); await page.keyboard.press('Enter');
        assert((await page.evaluate(()=>window.confirmations)).some(item=>item.title===words['mailer.rule_run_retry_unknown_title']&&item.message.includes('could duplicate')),'unknown rule retry must require an accessible duplicate-risk confirmation');
        assert(writes.some(item=>item.p==='messages/message-0/rule-runs/rule-unknown/retry'&&item.method==='POST'),'confirmed rule retry must use the per-message retry endpoint');
        await page.locator('[data-rule-runs-for="message-0"]').getByText(words['mailer.rule_run_executing'],{exact:true}).waitFor();
        assert(!(await page.locator('[data-rule-runs-for="message-0"]').innerText()).includes(words['mailer.outbox_sending']),'rule actions must not use the outbox sending label');
        ruleRuns=ruleRuns.map(run=>run.rule_id==='rule-failed'?{...run,status:'completed',action_index:run.action_count,error:''}:run);
        await page.evaluate(()=>document.dispatchEvent(new CustomEvent('aurago:mailer-change',{detail:{type:'mailer_changed',payload:{type:'rule_run',account_id:'email:work',id:'message-0',status:'completed'}}})));
        await page.locator('[data-rule-runs-for="message-0"]').getByText(words['mailer.rule_run_completed'],{exact:true}).waitFor();
        try {
            await page.frameLocator('[data-reader] iframe').first().getByText('Please review the next steps.', {exact:true}).waitFor({timeout:5000});
        } catch (error) {
            console.error(await page.frameLocator('[data-reader] iframe').first().locator('body').evaluate(node => ({html:node.innerHTML,style:getComputedStyle(node).cssText,rect:node.getBoundingClientRect().toJSON(),p:node.querySelector('p')?.getBoundingClientRect().toJSON(),display:getComputedStyle(node).display,visibility:getComputedStyle(node).visibility})));
            throw error;
        }
        assert.equal(await page.evaluate(()=>!!window.__mailInjection),false);
        assert(!network.some(url=>url.includes('tracker.invalid')),'reader leaked remote image request');
        const sandbox = (await page.locator('[data-reader] iframe').first().getAttribute('sandbox')).split(/\s+/);
        assert(!sandbox.includes('allow-scripts') && !sandbox.includes('allow-same-origin') && !sandbox.includes('allow-forms'), 'reader must isolate active mail content');
        assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'page overflow');
        if(width===390) {
            assert.equal(await page.locator('.vd-mailer-folders').isVisible(),false,'folder rail must not overlap the mobile reader');
            await page.locator('[data-reader-back]').click();
            await page.locator('.vd-mailer-folders').waitFor({state:'visible'});
            const nav=await page.locator('.vd-mailer-folders').boundingBox(), list=await page.locator('.vd-mailer-list-panel').boundingBox();
            assert(list.y>=nav.y+nav.height-1,'mobile list covers folder navigation');
            await page.locator('[data-thread-id]').first().focus(); await page.keyboard.press('Enter');
            await page.frameLocator('[data-reader] iframe').first().getByText('Please review the next steps.',{exact:true}).waitFor();
        }
        await page.screenshot({path:path.join(report,`mail-${theme}-${mode}-${width}.png`)});
        if(width===1440&&mode==='dark') {
            await page.locator('[data-thread-id="thread-1"]').click();
            await page.locator('[data-reader] iframe').waitFor({state:'visible'});
            await page.frameLocator('[data-reader] iframe').getByText('Please review the next steps.',{exact:true}).waitFor();
            assert(!writes.some(item=>item.p==='threads/thread-1/read'),'read-only provider grant must not mark mail read');
            assert.equal(await page.locator('[data-reader-action="trash"]').count(),0,'unavailable provider mutation must not be offered');
            await page.locator('[data-load-more]').click();
            await page.waitForFunction(()=>document.querySelector('[data-load-more]').hidden);
            assert.equal(await page.locator('[data-thread-id]').count(),40,'cursor pagination duplicated the previous page');
            await page.locator('[data-folder-id="email:work:archive"]').click();
            await page.locator('[data-search]').fill('project');
            await page.locator('[data-provider-search]').waitFor({state:'visible'});
            state.accounts[0].sync_error='rule_error';
            await page.evaluate(()=>document.dispatchEvent(new CustomEvent('aurago:mailer-change',{detail:{type:'mailer_changed',payload:{type:'account_sync',account_id:'email:work',status:'error'}}})));
            await page.locator('[data-status]').getByText(words['mailer.reason_rule_error'],{exact:true}).waitFor();
            await page.locator('[data-view="outbox"]').click();
            await page.locator('[data-retry="job-0"]').waitFor();
            assert.equal(await page.locator('.vd-mailer-outbox-item').count(),50,'outbox first page must show the server page only');
            const retriedOutbox = page.waitForResponse(response => response.url().endsWith('/outbox?limit=50'));
            await page.locator('[data-retry="job-0"]').click();
            await retriedOutbox;
            await page.locator('[data-mailer-workspace][aria-busy="false"]').waitFor();
            assert(writes.some(item=>item.p==='outbox/job-0/retry'&&item.method==='POST'),'outbox retry must address an item by id');
            await page.locator('[data-load-more]').click();
            await page.waitForFunction(()=>document.querySelectorAll('.vd-mailer-outbox-item').length===54);
            assert(network.some(url=>url.includes('/api/desktop/mailer/outbox?limit=50&cursor=outbox-next')||url.includes('/api/desktop/mailer/outbox?cursor=outbox-next&limit=50')),'outbox load-more must request the opaque cursor');
            await page.locator('[data-cancel-send="job-1"]').click();
            assert(writes.some(item=>item.p==='outbox/job-1'&&item.method==='DELETE'),'outbox cancel must address an item by id after pagination');
        }
        if(width===1440&&mode==='light') {
            await page.locator('[data-compose]').click();
            const editor=page.locator('[data-mailer-richtext]'); await editor.waitFor({timeout:20000});
            assert.equal(await page.locator('[data-chips="to"]').textContent(), '', 'new message must not inherit the selected sender');
            assert.equal(await page.locator('[data-send-undo]').isVisible(), false, 'undo must remain hidden before sending');
            await page.locator('[data-recipient-input="to"]').fill('recipient@example.test');await page.locator('[data-recipient-input="to"]').press('Enter');
            await page.locator('[data-subject]').fill('Reviewed draft');await editor.fill('A complete draft body.');
            await editor.focus(); await page.keyboard.press('ControlOrMeta+A');
            await page.locator('[data-format="bold"]').click();
            await page.locator('[data-save-now]').click(); await page.waitForFunction(()=>document.querySelector('[data-save-state]')?.textContent.includes('saved'));
            assert(draft?.to.includes('recipient@example.test'));assert(draft?.html.includes('A complete draft body.'));
            assert(draft?.html.includes('<strong>'), 'formatting toolbar must produce formatted outgoing HTML');
            await page.screenshot({path:path.join(report,`compose-${theme}.png`)});
            await page.locator('[data-assist-open]').click();
            const disclosure=await page.locator('[data-assist-provider-review]').textContent();
            assert(disclosure.includes('Local assistant')&&disclosure.includes('test-model')&&disclosure.includes('Security scanner')&&disclosure.includes('scan-model'),'all provider destinations must be disclosed');
            await page.locator('[data-assist-action]').selectOption('rewrite');
            await page.locator('[data-assist-run]').click();
            await page.locator('[data-assist-preview]:not([hidden])').waitFor();
            assert((await editor.textContent()).includes('A complete draft body.'),'preview must not replace the draft');
            await editor.fill('Edited after the preview.');
            await page.locator('[data-assist-apply]').click();
            assert((await editor.textContent()).includes('Edited after the preview.'),'stale preview overwrote new edits');
            await page.locator('[data-assist-run]').click();
            await page.locator('[data-assist-preview]:not([hidden])').waitFor();
            await page.locator('[data-assist-apply]').click();
            assert((await editor.textContent()).includes('Suggested text'),'approved preview was not applied');
            assert((await page.locator('[data-chips="to"]').textContent()).includes('recipient@example.test'),'assist changed recipients');
            await page.locator('[data-assist-run]').click();
            await page.locator('[data-assist-preview]:not([hidden])').waitFor();
            uploadWaitEnabled=true;
            await page.locator('[data-file-input]').setInputFiles({name:'deferred.txt',mimeType:'text/plain',buffer:Buffer.from('pending')});
            await uploadStarted;
            await page.locator('[data-send]').click();
            await page.waitForTimeout(80);
            assert.equal(writes.filter(x=>x.p==='outbox'&&x.method==='POST').length,0,'send must wait for a deferred upload to settle');
            for(const selector of ['[data-insert-template]','[data-insert-signature]','[data-assist-open]','[data-assist-apply]']) assert.equal(await page.locator(selector).isDisabled(),true,`${selector} must stay locked while send waits for attachment work`);
            releaseUpload(); uploadWaitEnabled=false;
            await page.locator('[data-send-undo]:not([hidden])').waitFor();
            assert.equal(draft.attachments.length,1,'successful upload must be saved before enqueue');
            await page.locator('[data-undo-send]').click();assert.equal(outbox,null,'undo must cancel persistent job');
            await page.waitForFunction(()=>document.querySelector('[data-send]')?.disabled===false);
            assert.equal(await page.locator('[data-send]').isDisabled(),false,'undo must restore the send control');
            assert.equal(await editor.isEditable(),true,'undo must restore body editing');
            await page.locator('[data-assist-discard]').click();
            await editor.fill('Edited after undo and ready to resend.');
            await page.waitForFunction(()=>document.querySelector('[data-save-state]')?.textContent.includes('saved'));
            await page.locator('[data-send]').click();await page.locator('[data-send-undo]:not([hidden])').waitFor();
            assert.equal(writes.filter(x=>x.p==='outbox'&&x.method==='POST').length,2,'editing after undo must allow a second send');
            await page.locator('[data-undo-send]').click();
            await page.waitForFunction(()=>document.querySelector('[data-send]')?.disabled===false);
            await page.waitForFunction(()=>document.querySelector('[data-save-state]')?.textContent.includes('saved'));
            uploadFailNext=true;
            assert.equal(await page.locator('[data-file-input]').isEnabled(),true,'undo must restore attachment selection');
            await page.locator('[data-file-input]').setInputFiles({name:'retry.txt',mimeType:'text/plain',buffer:Buffer.from('retry')});
            await page.locator('[data-upload-failed]').waitFor();
            assert.equal(await page.locator('[data-send]').isDisabled(),true,'failed upload must block sending');
            await page.locator('[data-retry-upload]').click();
            await page.waitForFunction(()=>document.querySelectorAll('[data-upload-failed]').length===0&&document.querySelector('[data-send]')?.disabled===false);
            await page.locator('[data-send]').click();await page.locator('[data-send-undo]:not([hidden])').waitFor();
            assert.equal(writes.filter(x=>x.p==='outbox'&&x.method==='POST').length,3,'send after explicit upload retry should enqueue once for each send attempt');
            await page.locator('[data-undo-send]').click();
            await page.waitForFunction(()=>document.querySelector('[data-send]')?.disabled===false);
            await page.waitForFunction(()=>document.querySelector('[data-save-state]')?.textContent.includes('saved'));
            conflictLatest={...draft,revision:draft.revision+1,subject:'Other window title',text:'Other window body',html:'<p>Other window body</p>',attachments:[]};
            conflictNextSave=true;
            const localText='My local text must survive conflict recovery.';
            await editor.fill(localText);
            await page.locator('[data-draft-conflict]:not([hidden])').waitFor();
            assert((await page.locator('[data-subject]').inputValue())==='Reviewed draft','conflict must not silently replace the subject');
            await page.locator('[data-resolve-draft-conflict]').focus(); await page.keyboard.press('Enter');
            await page.locator('[data-remove-conflict-attachment]').first().waitFor();
            await page.waitForFunction(()=>document.querySelector('[data-save-state]')?.textContent.includes('saved'));
            assert((await editor.textContent()).includes(localText),'conflict recovery must preserve the local body');
            assert((await page.locator('[data-chips="to"]').textContent()).includes('recipient@example.test'),'conflict recovery must preserve local recipients');
            assert.equal(await page.locator('[data-send]').isDisabled(),true,'changed attachment references must block sending until reviewed');
            while(await page.locator('[data-remove-conflict-attachment]').count()) await page.locator('[data-remove-conflict-attachment]').first().click();
            assert.equal(await page.locator('[data-send]').isDisabled(),false,'explicitly removing unavailable references should unblock send');
            await page.evaluate(()=>ctx.closeWindow());
            const imported={...draft,id:'draft-html-fixture',revision:draft.revision+1,account_id:'email:work',to:['local@example.test'],cc:[],bcc:[],subject:'Imported local subject',reply_remote_id:'local-reply-id',in_reply_to:'<local-in-reply-to@example.test>',references:['<local-reference@example.test>'],text:'Imported fallback text',html:'<p><strong>Imported local HTML</strong></p>'};
            draft=imported;
            conflictLatest={...imported,revision:imported.revision+1,account_id:'email:private',subject:'Other window subject',reply_remote_id:'other-reply-id',in_reply_to:'<other-in-reply-to@example.test>',references:['<other-reference@example.test>'],text:'Other window text',html:'<p>Other window HTML</p>'};
            conflictNextSave=true;
            await page.evaluate(value=>ctx.openApp('mailer',{draft:value,accountId:'email:work'}),imported);
            await page.locator('[data-subject]').fill('Local subject kept after conflict');
            await page.locator('[data-draft-conflict]:not([hidden])').waitFor();
            await page.locator('[data-resolve-draft-conflict]').click();
            await page.waitForFunction(()=>document.querySelector('[data-save-state]')?.textContent.includes('saved'));
            assert.equal(draft.html,'<p><strong>Imported local HTML</strong></p>','conflict rebase must preserve imported local HTML');
            assert.equal(draft.text,'Imported fallback text','conflict rebase must preserve imported local text');
            assert.equal(draft.subject,'Local subject kept after conflict','conflict rebase must preserve local subject');
            assert.equal(draft.reply_remote_id,'local-reply-id','conflict rebase must preserve local reply metadata');
            assert.equal(draft.in_reply_to,'<local-in-reply-to@example.test>','conflict rebase must preserve local In-Reply-To metadata');
            assert.deepEqual(draft.references,['<local-reference@example.test>'],'conflict rebase must preserve local References metadata');
            assert.equal(draft.account_id,'email:work','conflict rebase must preserve the still-available local account');
            await page.evaluate(()=>ctx.closeWindow());
            await page.evaluate(()=>ctx.closeWindow());
            await page.locator('[data-reader-action="reply_all"]').click();
            await page.locator('[data-mailer-richtext]').waitFor({timeout:20000});
            assert((await page.locator('[data-chips="to"]').textContent()).includes('alice@example.test'),'reply-all lost the original sender');
            assert(!(await page.locator('[data-chips="to"]').textContent()).includes('owner@example.test'),'reply-all included the receiving account');
            assert((await page.locator('[data-chips="cc"]').textContent()).includes('cc@example.test'),'reply-all lost Cc');
            assert.equal(await page.locator('[data-subject]').inputValue(),'Re: Project update and next steps');
            await page.evaluate(()=>ctx.closeWindow());
            await page.locator('[data-open-settings]').click();
            assert.equal(await page.locator('[data-test-account="email:work"][data-direction="receive"]').isEnabled(),true,'receive test must use read capability');
            assert(await page.locator('[data-reauthorize="email:work"]').count()>0,'OAuth accounts must support authorization again');
            for (const tab of ['accounts','templates','signatures','rules']) {
                await page.locator(`[data-setting-tab="${tab}"]`).click();
                assert(!/\bmailer\.[a-z_]+/.test(await page.locator('[data-settings]').innerText()), `${tab} settings exposed missing translations`);
            }
        }
        assert.deepEqual(errors,[],`${theme}/${mode}/${width} browser errors`);
        await page.close();
    }
    console.log('Mailer browser: 12 theme/mode/width cases, large list, keyboard, isolated HTML, composer and undo passed.');
} finally {await browser.close();await new Promise(resolve=>server.close(resolve));}
