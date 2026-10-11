import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { readFile, mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const locales = JSON.parse(await readFile(path.join(root, 'ui/lang/desktop/en.json'), 'utf8'));
const configLocales = JSON.parse(await readFile(path.join(root, 'ui/lang/config/gog/en.json'), 'utf8'));
const storageSource = await readFile(path.join(root,'ui/js/desktop/file-manager/storage.js'),'utf8');
const server = createServer(async (req, res) => {
    try {
        const name = new URL(req.url, 'http://localhost').pathname;
        if (name === '/') { res.setHeader('Content-Type','text/html'); res.end('<!doctype html><meta charset="utf-8"><title>gog regression fixture</title><body class="desktop-body"><div id="host" style="position:absolute;inset:0"><div><input id="local-draft" value="Local draft"></div></div>'); return; }
        const file = path.resolve(root, 'ui', '.' + name);
        if (!file.startsWith(path.join(root, 'ui') + path.sep)) throw Error('path');
        res.setHeader('Content-Type', name.endsWith('.css') ? 'text/css' : name.endsWith('.woff2') ? 'font/woff2' : 'text/javascript'); res.end(await readFile(file));
    } catch { res.statusCode = 404; res.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const browser = await chromium.launch({ headless: true, ...(process.env.PLAYWRIGHT_CHANNEL ? {channel:process.env.PLAYWRIGHT_CHANNEL} : {}) });
const origin = `http://127.0.0.1:${server.address().port}`;
const report = path.join(root, 'reports/gog'); await mkdir(report, { recursive: true });
try {
    for (const theme of ['standard', 'fruity']) for (const width of [1365, 390]) {
        const page = await browser.newPage({ viewport: { width, height: 844 } }); const errors = [], calls = [], writes = [];
        page.on('pageerror', e => errors.push(e.message));
        const grants = { read:true, create:true, update:true, delete:true };
        let readonly = false, unknown = false, pendingWrite = null, pendingRead = null, failWrite = false, recurrence = [];
        const accounts = ['first','second'].map(id => ({ id, name: id, services: Object.fromEntries(['calendar','tasks','contacts','drive','docs','sheets'].map(s => [s, grants])) }));
        const json = data => ({ contentType:'application/json', body:JSON.stringify(data) });
        await page.route('**/api/desktop/integrations/gog/**', async route => {
            if (route.request().method() === 'GET') { await route.fulfill(json({accounts,readonly})); return; }
            const req = route.request().postDataJSON(); calls.push(req);
            const read = ['calendars','events','lists','list','get','search'].includes(req.operation);
            if (!read) { writes.push(req); if (pendingWrite) await pendingWrite; if (failWrite) { await route.abort('connectionreset'); return; } await route.fulfill(json(unknown ? {status:'external_status_unknown',code:'write_outcome_unknown'} : {status:'ok',data:{}})); return; }
            const data = req.operation === 'calendars' ? {calendars:[{id:'calendar-id',summary:'Team'}]} : req.operation === 'lists' ? {tasklists:[{id:'tasks-id',title:'Project'}]} : req.service === 'calendar' ? {events:[{id:'event-id',summary:'Planning',recurringEventId:'series-id',start:{dateTime:'2026-10-11T10:00:00+02:00',timeZone:'Europe/Berlin'},end:{dateTime:'2026-10-11T11:00:00+02:00'},originalStartTime:{dateTime:'2026-10-11T10:00:00+02:00'}}]} : req.service === 'tasks' ? {tasks:[{id:'task-id',title:'Fixture task',status:'needsAction'}]} : req.operation === 'get' ? {contact:{resourceName:'people/fixture',names:[{givenName:'Example',familyName:'Person'}],emailAddresses:[{value:'fixture@example.invalid'}]}} : {contacts:[{resource:'people/fixture',name:'Example <img src=x onerror=alert(1)>',email:req.account_id+'@example.invalid'}]};
            if (data.tasks) data.tasks[0].due = '2026-10-12T00:00:00Z';
            if (data.events && recurrence.length) { data.events[0].recurrence = recurrence; delete data.events[0].recurringEventId; }
            if (req.operation === 'get' && pendingRead) await pendingRead;
            await route.fulfill(json({status:'ok',data}));
        });
        await page.goto(origin); for (const css of ['/fonts/fonts.css','/shared-variables.css','/shared-utilities.css','/shared-components.css','/css/desktop-shell.bundle.css','/css/desktop-gog.css']) await page.addStyleTag({url:origin+css});
        await page.addScriptTag({url:origin+'/js/desktop/apps/gog-sources.js'});
        await page.evaluate(({locales,theme}) => { document.body.dataset.theme=theme;window.ctx={t:key=>locales[key]||key};window.mount=service=>{window.disposeGog?.();document.querySelector('#host').innerHTML='<div><input id="local-draft" value="Local draft"></div>';window.disposeGog=AuraGogDesktop.attachSources(document.querySelector('#host'),service,ctx);}; }, {locales,theme});
        await page.evaluate(() => mount('contacts')); await page.waitForSelector('.gog-source-bar:not([hidden])'); assert.equal(calls.length,0,'status must not contact Google');
        await page.locator('.gog-source-bar select').selectOption('first'); await page.waitForSelector('.gog-record'); assert.equal(await page.locator('.gog-record img').count(),0);
        let releaseRead; pendingRead = new Promise(resolve=>{releaseRead=resolve;});
        const reading = page.waitForRequest(req=>req.method()==='POST'&&req.postDataJSON().operation==='get');
        await page.locator('.gog-record').click(); await reading;
        assert(await page.locator('.gog-record').isDisabled(),'contact reads must not race to replace an editor'); releaseRead(); pendingRead=null;
        await page.getByLabel('Given name',{exact:true}).fill('Changed');
        await page.getByRole('button',{name:'Save',exact:true}).click(); await page.waitForFunction(()=>!document.querySelector('.gog-editor form'));
        assert.deepEqual(writes.at(-1).args,{resource_name:'people/fixture',given:'Changed'},'editing a name must not replace unchanged email/phone collections');
        await page.locator('.gog-record').click(); await page.getByLabel('Given name',{exact:true}).fill('Unsaved');
        await page.locator('.gog-source-bar select').selectOption('second'); await page.waitForSelector('dialog[open]'); await page.keyboard.press('Escape'); await page.waitForFunction(()=>document.querySelector('.gog-source-bar select').value==='first');
        assert.equal(await page.locator('.gog-source-bar select').inputValue(),'first'); assert.equal(await page.getByLabel('Given name',{exact:true}).inputValue(),'Unsaved');
        await page.locator('.gog-source-bar select').selectOption('second'); await page.getByRole('button',{name:'Continue',exact:true}).click(); await page.waitForFunction(()=>document.querySelector('.gog-record span')?.textContent.includes('second@'));
        await page.locator('.gog-source-bar select').selectOption(''); assert.equal(await page.locator('#local-draft').inputValue(),'Local draft');
        await page.evaluate(() => mount('calendar')); await page.locator('.gog-source-bar select').selectOption('first'); await page.locator('.gog-record').click();
        const before=writes.length; await page.getByRole('button',{name:'Save',exact:true}).click(); assert.equal(writes.length,before,'series scope must be explicit');
        await page.getByLabel('Edit series',{exact:true}).selectOption('single'); unknown=true; await page.getByRole('button',{name:'Save',exact:true}).click(); await page.waitForFunction(()=>document.querySelector('.gog-status')?.textContent.includes('may have completed'));
        assert.equal(writes.length,before+1); assert.equal(writes.at(-1).args.original_start,'2026-10-11T10:00:00+02:00'); assert.equal(writes.at(-1).account_id,'first'); unknown=false;
        await page.screenshot({path:path.join(report,`calendar-${theme}-${width}.png`)});
        recurrence=['RRULE:FREQ=WEEKLY']; await page.evaluate(() => mount('calendar')); await page.locator('.gog-source-bar select').selectOption('first'); await page.locator('.gog-record').click();
        await page.getByLabel('Edit series',{exact:true}).selectOption('all'); await page.getByLabel('Recurrence rules (RRULE, one per line)',{exact:true}).fill('');
        await page.getByRole('button',{name:'Save',exact:true}).click(); await page.waitForFunction(()=>!document.querySelector('.gog-editor form'));
        assert.deepEqual(writes.at(-1).args.rrule,[''],'clearing recurrence must send an explicit empty flag'); recurrence=[];
        await page.evaluate(() => mount('tasks')); await page.locator('.gog-source-bar select').selectOption('first'); await page.locator('.gog-record').click(); await page.getByRole('button',{name:'Complete',exact:true}).click(); await page.waitForFunction(()=>!document.querySelector('.gog-editor form'));
        assert.equal(writes.at(-1).operation,'complete'); assert.equal(writes.at(-1).args.tasklist_id,'tasks-id');
        await page.locator('.gog-record').click(); await page.getByLabel('Due date',{exact:true}).fill('');
        await page.getByRole('button',{name:'Save',exact:true}).click(); await page.waitForFunction(()=>!document.querySelector('.gog-editor form'));
        assert.equal(writes.at(-1).args.due,'','clearing a due date must reach Google as an explicit empty value');
        await page.locator('.gog-record').click();
        let releaseWrite; pendingWrite = new Promise(resolve => { releaseWrite = resolve; });
        const started = page.waitForRequest(req=>req.method()==='POST'&&req.postDataJSON().operation==='complete');
        await page.getByRole('button',{name:'Complete',exact:true}).click(); await started;
        assert(await page.locator('[data-refresh]').isDisabled(),'refresh must not cancel a pending mutation');
        assert(await page.locator('.gog-record').isDisabled(),'opening another editor must not overlap a pending mutation');
        assert(await page.locator('.gog-source-bar select').isDisabled(),'account switch must wait for a pending mutation');
        failWrite=true; releaseWrite(); pendingWrite=null;
        await page.waitForFunction(()=>document.querySelector('.gog-status')?.textContent.includes('may have completed'));
        assert(await page.getByRole('button',{name:'Complete',exact:true}).isDisabled(),'connection loss must not invite a duplicate write'); failWrite=false;
        await page.locator('[data-refresh]').click(); await page.waitForFunction(()=>!document.querySelector('[data-list-create]').disabled);
        pendingWrite = new Promise(resolve => { releaseWrite = resolve; });
        await page.locator('[data-list-create]').click(); await page.getByLabel('Title',{exact:true}).fill('Created list');
        const listStarted = page.waitForRequest(req=>req.method()==='POST'&&req.postDataJSON().operation==='create_list');
        await page.getByRole('button',{name:'Continue',exact:true}).click(); await listStarted;
        assert(await page.locator('[data-refresh]').isDisabled()); assert(await page.locator('.gog-source-bar select').isDisabled(),'list creation must keep its account stable');
        releaseWrite(); pendingWrite=null; await page.waitForFunction(()=>!document.querySelector('[data-list-create]').disabled);
        readonly=true; await page.evaluate(()=>document.dispatchEvent(new Event('aurago:desktop-policy'))); await page.waitForFunction(()=>document.querySelector('[data-create]').disabled);
        assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'unexpected horizontal page scrolling');
        await page.route('**/api/desktop/integrations/gog/office/import',route=>route.fulfill(json({status:'ok',file:{path:'copies/fixture.docx'}})));
        const imported=await page.evaluate(async()=>{ctx.saveFileDialog=async()=>({path:'copies/fixture.docx'});return AuraGogDesktop.office(ctx,'docs','import','',undefined,{account_id:'first',resource_id:'doc-id'});});
        assert.equal(imported.file.path,'copies/fixture.docx','read-only Google account must allow an explicit local copy');
        const identity=await page.evaluate(source=>{
            const fm={currentPath:'folder',storage:{provider:'gog',account_id:'first',item_id:'folder'},gogLocations:new Map()};
            const api=new Function('fm','joinPath',source+';return {storageLocation,storageChildLocation,locationKey,savedLocation};')(fm,(a,b)=>[a,b].filter(Boolean).join('/'));
            const a={provider:'gog',account_id:'first',item_id:'one',path:'folder/one',name:'same.txt'},b={...a,item_id:'two',path:'folder/two'};
            const child=api.storageChildLocation({...a,item_id:'folder',path:'folder'},'copy.txt');
            return {distinct:api.locationKey(a)!==api.locationKey(b),account:api.locationKey(a)!==api.locationKey({...a,account_id:'second'}),saved:api.savedLocation(b),child:api.storageLocation(child)};
        },storageSource);
        assert(identity.distinct&&identity.account); assert.equal(identity.saved.item_id,'two'); assert.equal(identity.child.item_id,''); assert.equal(identity.child.parent_id,'folder');

        const admin=[];
        await page.route('**/api/gog/**',async route=>{const name=new URL(route.request().url()).pathname.split('/').at(-1);admin.push({name,method:route.request().method()});await route.fulfill(json(name==='status'?{version:'0.43.0-aurago1',installed:true,accounts:[]}:{status:'ok'}));});
        await page.evaluate(words=>{disposeGog();document.querySelector('#host').innerHTML='<main id="content"></main>';window.configData={gog:{enabled:true,readonly:true,accounts:[{id:'first',name:'First',enabled:true,client_id:'fixture-client',services:{docs:{read:true}}}]}};window.t=k=>words[k]||k;window.escapeHtml=s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');window.isDirty=false;window.setDirty=v=>isDirty=v;window.attachChangeListeners=()=>AuraConfigState.bind(document.querySelector('#content'));},configLocales);
        await page.addScriptTag({url:origin+'/js/config/state.js'}); await page.addScriptTag({url:origin+'/cfg/form-builder.js'}); await page.addScriptTag({url:origin+'/cfg/gog.js'});
        await page.evaluate(async()=>{AuraConfigState.init(configData);await renderGogSection({label:'Google Workspace via gog'});});
        assert.deepEqual(admin,[{name:'status',method:'GET'}]);
        await page.getByRole('button',{name:'Test connection',exact:true}).click(); await page.waitForFunction(()=>document.querySelector('#gog-message').textContent==='Done');
        assert.equal(admin.filter(x=>x.name==='test').length,1);assert(admin.every(x=>x.name==='status'||x.method==='POST'));
        await page.getByRole('button',{name:'Add account',exact:true}).click();
        const patch=await page.evaluate(()=>{AuraConfigState.syncFromDOM();return AuraConfigState.buildPatch();});
        assert.equal(patch.gog.accounts.length,2); assert.match(patch.gog.accounts[1].id,/^g[a-f0-9]+$/);
        await page.getByRole('button',{name:'Test connection',exact:true}).first().click(); assert.equal(admin.filter(x=>x.name==='test').length,1,'unsaved accounts must not trigger a probe');
        assert.deepEqual(errors,[]); await page.close();
        console.log(`gog Desktop: ${theme} ${width}px passed`);
    }
} finally { await browser.close(); await new Promise(resolve=>server.close(resolve)); }
