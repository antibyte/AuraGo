import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFile, mkdir} from 'node:fs/promises';
import {createRequire} from 'node:module';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const require = createRequire(import.meta.url);
const {chromium} = require(process.env.AURAGO_PLAYWRIGHT || 'playwright');
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
const browser = await chromium.launch({headless:true, ...(process.env.PLAYWRIGHT_CHANNEL?{channel:process.env.PLAYWRIGHT_CHANNEL}:{})});
try {
    for (const theme of ['standard','fruity']) for (const mode of ['dark','light']) for (const width of [1440,900,390]) {
        const page = await browser.newPage({viewport:{width,height:900}});
        const errors=[], writes=[], network=[];
        page.on('pageerror',e=>errors.push(e.message));
        page.on('request',r=>network.push(r.url()));
        const accounts=[{id:'email:work',source:'email',protocol:'imap',name:'Work',address:'owner@example.test',linked:true,reauthorize_supported:true,capabilities:['read','send','read_state','star','move','archive','trash','restore','delete','folders_read','folders_create']},{id:'email:private',source:'email',protocol:'pop3',name:'Personal',address:'personal@example.test',linked:true,capabilities:['read','search','folders_read']}];
        const folders=accounts.flatMap(a=>['inbox','archive','trash','sent'].map(kind=>({id:a.id+':'+kind,account_id:a.id,provider_id:kind,name:kind[0].toUpperCase()+kind.slice(1),kind})));
        const state={enabled:true,settings:{enabled:true},accounts,sources:accounts,folders,account_settings:{},templates:[],signatures:[],rules:[],contacts:[{name:'Alice Example',email:'alice@example.test'}],providers:[{id:'fixture',name:'Local assistant',model:'test-model'}],default_provider_id:'fixture',assist_guardian:{id:'scanner',name:'Security scanner',model:'scan-model'},assist_policy:'fixture-policy',readonly:false};
        const rows=Array.from({length:40},(_,i)=>({id:'thread-'+i,message_id:'message-'+i,account_id:accounts[i%2].id,folder_id:accounts[i%2].id+':inbox',from:i%2?'Mara <mara@example.test>':'Alice <alice@example.test>',subject:i===0?'Project update and next steps':'Mailbox fixture '+i,preview:'A clear preview of the selected conversation.',date:new Date(Date.UTC(2026,9,16,12,0,-i)).toISOString(),unread:i<8,starred:i===2,message_count:1,attachment_count:0}));
        let draft=null, outbox=null;
        const json=value=>({status:200,contentType:'application/json',body:JSON.stringify(value)});
        await page.route('**/api/desktop/mailer/**',async route=>{
            const request=route.request(), u=new URL(request.url()), p=u.pathname.replace('/api/desktop/mailer/',''), method=request.method();
            const data=request.postData()?JSON.parse(request.postData()):{};
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
            if((p==='drafts'||p.startsWith('drafts/'))&&(method==='POST'||method==='PUT')) { draft={...data,id:'draft-fixture',revision:(draft?.revision||0)+1,updated_at:new Date().toISOString()}; return route.fulfill(json(draft)); }
            if(p==='outbox'&&method==='POST') {outbox={id:'outbox-fixture',...data,status:'scheduled',undo_until:new Date(Date.now()+10000).toISOString()};return route.fulfill(json(outbox));}
            if(p==='outbox'&&method==='GET') return route.fulfill(json({items:outbox?[outbox]:[]}));
            if(p==='outbox/outbox-fixture'&&method==='DELETE') {outbox=null;return route.fulfill(json({ok:true}));}
            if(p==='assist') { assert.equal(data.assist_policy,'fixture-policy'); return route.fulfill(json({preview:{text:'Suggested text',html:'<p>Suggested text</p>'},source_revision:data.revision,provider_id:'fixture',model:'test-model'})); }
            if(['templates','signatures','rules'].includes(p)) return route.fulfill(json({items:[]}));
            return route.fulfill(json({ok:true}));
        });
        await page.goto(origin);
        await page.addStyleTag({url:origin+'/css/desktop-shell.bundle.css'});
        await page.addStyleTag({url:origin+'/css/desktop-app-mailer.css'});
        await page.addScriptTag({url:origin+'/js/vendor/marked.min.js'});
        await page.addScriptTag({url:origin+'/js/vendor/purify.min.js'});
        for(const file of ['mailer-api','mailer-compose','mailer-settings','mailer']) await page.addScriptTag({url:origin+'/js/desktop/apps/'+file+'.js'});
        await page.evaluate(({words,theme,mode,state})=>{
            document.body.className='desktop-body'; document.body.dataset.theme=theme; document.documentElement.dataset.theme=mode;
            document.documentElement.style.setProperty('--surface-color',mode==='light'?'#f5f6fa':'#111823');
            document.documentElement.style.setProperty('--text-color',mode==='light'?'#202b3a':'#edf2f8');
            document.documentElement.style.setProperty('--text-muted',mode==='light'?'#526174':'#9aa8ba');
            window.ctx={state,readonly:false,t:(key,vars={})=>{let text=words[key]||key;for(const[k,v]of Object.entries(vars))text=text.replaceAll('{'+k+'}',v);return text;},api:async(url,options)=>{const r=await fetch(url,options);const value=await r.json();if(!r.ok)throw Object.assign(new Error(value.error),value);return value;},confirmDialog:async()=>true,promptDialog:async()=>'',iconMarkup:()=>'<span aria-hidden="true">✉</span>',registerWindowCleanup:()=>{},setWindowBeforeClose:(id,cb)=>window.beforeClose=cb,closeWindow:()=>{document.querySelector('#compose').hidden=true;MailerCompose.dispose('composer');},openApp:(app,options)=>{const host=document.querySelector('#compose');host.hidden=false;MailerCompose.render(host,'composer',{...ctx,state:options.state},options);}};
            ctx.providers=[];
            MailerApp.render(document.querySelector('#host'),'mail',ctx,{});
        },{words,theme,mode,state});
        await page.locator('[data-thread-id]').first().waitFor();
        assert.equal(await page.locator('[data-thread-id]').count(),40,'large mailbox must only render the returned page');
        await page.locator('[data-thread-id]').first().focus(); await page.keyboard.press('Enter');
        await page.locator('[data-reader] iframe').first().waitFor({state:'visible'});
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
            await page.locator('[data-assist-open]').click();
            await page.locator('[data-send]').click();await page.locator('[data-send-undo]:not([hidden])').waitFor();
            assert.equal(writes.filter(x=>x.p==='outbox'&&x.method==='POST').length,1);
            await page.locator('[data-undo-send]').click();assert.equal(outbox,null,'undo must cancel persistent job');
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
