import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Runs the shipped action-policy module and styles against synthetic APIs only.
// No AuraGo instance, local configuration, provider or administrator token is used.
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const reports = path.join(root, 'reports/audit_harness_20261018/action-policy');
const locales = ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
const dictionaries = Object.fromEntries(await Promise.all(locales.map(async lang => [lang, JSON.parse(await readFile(path.join(root, `ui/lang/action_policy/${lang}.json`), 'utf8'))])));
const report = { started_at: new Date().toISOString(), passed: [], failures: [], screenshots: [] };
await mkdir(reports, { recursive: true });

function playwright() {
    const require = createRequire(import.meta.url);
    const candidates = [process.env.AURAGO_PLAYWRIGHT_MODULE, 'playwright', path.join(os.homedir(), '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright')].filter(Boolean);
    for (const candidate of candidates) {
        try { return require(candidate); } catch (error) { if (error.code !== 'MODULE_NOT_FOUND') throw error; }
    }
    throw new Error('Playwright is unavailable; set AURAGO_PLAYWRIGHT_MODULE to its installed module path.');
}

const server = createServer(async (request, response) => {
    try {
        const url = new URL(request.url, 'http://127.0.0.1');
        if (url.pathname === '/') {
            const lang = locales.includes(url.searchParams.get('lang')) ? url.searchParams.get('lang') : 'de';
            const theme = url.searchParams.get('theme') === 'fruity' ? 'fruity' : 'standard';
            const mode = url.searchParams.get('mode') === 'light' ? 'light' : 'dark';
            const configSurface = url.searchParams.get('surface') === 'config';
            const rootMode = configSurface ? mode : 'dark';
            const density = url.searchParams.get('density') === 'compact' ? 'compact' : 'comfortable';
            const policy = url.searchParams.get('policy') || 'on';
            const initialConfig = {agent: policy === 'missing' ? {} : {action_policy: {allow:[],deny:[]}}};
            if (!['missing','unset'].includes(policy)) initialConfig.agent.action_policy.enabled = policy === 'on';
            const bodyAttributes = configSurface ? `class="pw-page" data-workspace-page="config" data-density="${density}"` : `class="desktop-body" data-theme="${theme}" data-fruity-mode="${mode}"`;
            const surfaceStyles = configSurface ? '<link rel="stylesheet" href="/css/tokens.css"><link rel="stylesheet" href="/css/config.css"><link rel="stylesheet" href="/css/precision-workspace.css"><link rel="stylesheet" href="/css/precision-pages.css"><link rel="stylesheet" href="/css/config-workspace.css">' : '<link rel="stylesheet" href="/css/desktop-shell.bundle.css">';
            response.setHeader('Content-Type', 'text/html; charset=utf-8');
            response.end(`<!doctype html><html lang="${lang}" data-theme="${rootMode}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="/fonts/fonts.css"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/action-policy.css">${surfaceStyles}<style>body{overflow:auto}.fixture{position:relative;max-width:960px;margin:0 auto;padding:20px;min-height:100vh;box-sizing:border-box;background:var(--card-bg,transparent);color:var(--text-primary,#eee)}.fixture h1{font-size:1.5rem}</style></head><body ${bodyAttributes}><main class="fixture"><h1>AuraGo · Danger Zone</h1><div id="settings" class="cfg-section active"></div></main><script>window.SYSTEM_LANG=${JSON.stringify(lang)};window.fixture={config:${JSON.stringify(initialConfig)},changes:[],atomic:[],dirty:false};window.setDirty=value=>fixture.dirty=value;window.AuraConfigState={markAtomic:key=>fixture.atomic.push(key),set:(key,value)=>fixture.changes.push({key,value:structuredClone(value)})};</script><script src="/js/shared/action-policy.js"></script><script>window.fixture.ready=AuraActionPolicy.mountSettings(document.getElementById('settings'),fixture.config);</script></body></html>`);
            return;
        }
        const relative = decodeURIComponent(url.pathname).slice(1);
        if (!relative || relative.split('/').some(segment => segment === '..') || relative.includes('\\')) { response.writeHead(404).end(); return; }
        const data = await readFile(path.join(root, 'ui', relative));
        response.setHeader('Content-Type', ({'.js':'application/javascript','.css':'text/css','.json':'application/json','.woff2':'font/woff2'})[path.extname(relative)] || 'application/octet-stream');
        response.end(data);
    } catch { response.writeHead(404).end(); }
});
await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await playwright().chromium.launch({ headless: true, channel: process.env.AURAGO_BROWSER_CHANNEL || 'chrome', chromiumSandbox: true, args: ['--disable-background-networking', '--disable-component-update'] });
const catalog = [
    {type:'function',function:{name:'filesystem',parameters:{properties:{operation:{enum:['read_file','write_file','delete']},path:{type:'string'}}}}},
    {type:'function',function:{name:'send_email',parameters:{properties:{operation:{enum:['send']}}}}}
];
const attack = '<img src=x onerror="window.fixture.injected=true"><script>window.fixture.injected=true</script>';

async function test(name, run) {
    try { await run(); report.passed.push(name); console.log(`PASS ${name}`); }
    catch (error) { report.failures.push({name,error:error.stack || String(error)}); console.error(`FAIL ${name}: ${error.message}`); }
}

async function fixture(options = {}) {
    const {lang='de',theme='standard',mode='dark',width=1280,admin=true,surface='desktop',policy='on',density='comfortable'} = options;
    const page = await browser.newPage({ viewport: {width,height:900}, reducedMotion: 'reduce' });
    page.setDefaultTimeout(8000);
    const state = { errors: [], previews: [], decisions: [], pendingRequests: [], pendingQueries: [], pending: [], decisionDelay: 0, decisionStatus: 200, decisionResult: {status:'success',result:'Fixture completed'}, abortDecision: false, catalog:structuredClone(catalog), catalogRequests:0 };
    page.on('pageerror', error => state.errors.push(error.message));
    await page.route('**/api/**', async route => {
        const request = route.request();
        const url = new URL(request.url());
        if (url.pathname === '/api/agent/action-policy/catalog') {
            state.catalogRequests++;
            return route.fulfill({status:admin?200:403,json:admin?state.catalog:{error:'administrator_required'}});
        }
        if (url.pathname === '/api/agent/action-policy/preview') {
            state.previews.push(request.postDataJSON());
            return route.fulfill({json:{state:'denied',action:{tool:'filesystem',operation:'delete',target:attack}}});
        }
        if (url.pathname === '/api/agent/actions' && request.method() === 'GET') {
            state.pendingRequests.push(url.searchParams.get('session_id'));
            state.pendingQueries.push(Object.fromEntries(url.searchParams));
            return route.fulfill({json:{pending:state.pending}});
        }
        if (url.pathname === '/api/agent/actions' && request.method() === 'POST') {
            state.decisions.push({body:request.postDataJSON(),session:request.headers()['x-session-id']});
            if (state.decisionDelay) await new Promise(resolve => setTimeout(resolve,state.decisionDelay));
            if (state.abortDecision) return route.abort('failed');
            return route.fulfill({status:state.decisionStatus,json:state.decisionResult});
        }
        throw new Error(`Unexpected fixture API: ${request.method()} ${url.pathname}`);
    });
    await page.goto(`${origin}/?lang=${lang}&theme=${theme}&mode=${mode}&surface=${surface}&policy=${policy}&density=${density}`);
    await page.evaluate(() => fixture.ready);
    const text = Object.fromEntries(Object.entries(dictionaries[lang]).map(([key,value]) => [key.replace(/^action_policy\./,''),value]));
    return { page, state, text, close:async () => { await page.close(); assert.deepEqual(state.errors, [], 'browser JavaScript errors'); } };
}

function pending(id, extra = {}) {
    return {id,session_id:'fixture/session-A',expires_at:new Date(Date.now()+60000).toISOString(),action:{tool:'filesystem',operation:'delete',target:attack},arguments_preview:JSON.stringify({path:attack}),...extra};
}

async function show(page, item) { await page.evaluate(item => { void AuraActionPolicy.show(item); }, item); await page.locator('dialog[open]').waitFor(); }
async function closeDialog(page, text) { await page.getByRole('button',{name:text.close,exact:true}).click(); await page.locator('dialog').waitFor({state:'detached'}); }
async function screenshot(page, name) { const file=path.join(reports,`${name}.png`); await page.screenshot({path:file,fullPage:true}); report.screenshots.push(path.relative(root,file).replaceAll('\\','/')); }
async function noInjection(page) { assert.equal(await page.evaluate(() => !!fixture.injected), false); assert.equal(await page.locator('.action-policy-panel img, .action-policy-dialog img, .action-policy-dialog script').count(),0); }

try {
    await test('all 16 locale keys match and prose is translated', async () => {
        function leaves(value,prefix='') {
            return Object.fromEntries(Object.entries(value).flatMap(([key,entry])=>{
                const name=prefix?`${prefix}.${key}`:key;
                return entry && typeof entry==='object' ? Object.entries(leaves(entry,name)) : [[name,entry]];
            }));
        }
        const english=leaves(dictionaries.en);
        const keys=Object.keys(english).sort();
        for (const lang of locales) {
            const dictionary=leaves(dictionaries[lang]);
            assert.deepEqual(Object.keys(dictionary).sort(),keys,`${lang} locale key parity`);
            for (const key of keys) {
                const value=dictionary[key];
                assert.equal(typeof value,'string',`${lang}.${key}`);
                assert.ok(value.trim(),`${lang}.${key} is empty`);
                if(lang!=='en' && english[key].length>14) assert.notEqual(value,english[key],`${lang}.${key} copies English prose`);
            }
        }
    });

    await test('actual Config, Chat and Desktop consumers are connected',async()=>{
        for(const file of ['ui/config.html','ui/index.html','ui/desktop.html']) {
            const source=await readFile(path.join(root,file),'utf8');
            assert.match(source,/action-policy\.js\?v=\{\{\.BuildVersion\}\}/,`${file} missing versioned runtime`);
            assert.match(source,/action-policy\.css\?v=\{\{\.BuildVersion\}\}/,`${file} missing versioned stylesheet`);
        }
        assert.match(await readFile(path.join(root,'ui/cfg/danger.js'),'utf8'),/AuraActionPolicy\.mountSettings/);
        for(const file of ['ui/js/chat/chat-streaming.js','ui/js/desktop/apps/agent-chat.js']) assert.match(await readFile(path.join(root,file),'utf8'),/action_approval[\s\S]*AuraActionPolicy\.show/);
    });

    for (const policy of ['missing','unset','off']) await test(`Config policy defaults off: ${policy}`,async()=>{
        const f=await fixture({surface:'config',policy});
        try {
            assert.equal(await f.page.getByLabel(f.text.enabled,{exact:true}).isChecked(),false);
            assert.equal(await f.page.locator('.action-policy-editor').isVisible(),false);
            assert.equal(await f.page.getByRole('button',{name:f.text.pending,exact:true}).isVisible(),true);
            assert.equal(await f.page.evaluate(()=>fixture.dirty || fixture.changes.length>0),false,'mount changed the saved policy');
            if (policy==='missing') await screenshot(f.page,'config-policy-default-off');
        } finally { await f.close(); }
    });

    await test('Config native switch saves booleans and retains rules while off',async()=>{
        const f=await fixture({surface:'config',policy:'off'});
        try {
            await f.page.addScriptTag({url:origin+'/js/config/state.js'});
            await f.page.evaluate(()=>{
                AuraConfigState.init(fixture.config);
                AuraConfigState.bind(document.getElementById('settings'));
                window.setDirty=()=>AuraConfigState.syncFromDOM();
            });
            await f.page.locator('label.toggle').click();
            assert.equal(await f.page.locator('.action-policy-editor').isVisible(),true);
            assert.equal(await f.page.evaluate(()=>AuraConfigState.buildPatch().agent.action_policy.enabled),true);
            await f.page.getByRole('button',{name:f.text.add,exact:true}).first().click();
            const rule=f.page.locator('.action-policy-rule').first();
            await rule.locator('select').first().selectOption('filesystem');
            await rule.locator('input').first().fill('fixture.txt');
            await rule.locator('input').first().blur();
            await f.page.locator('label.toggle').click();
            assert.equal(await f.page.locator('.action-policy-editor').isVisible(),false);
            const patch=await f.page.evaluate(()=>JSON.parse(JSON.stringify(AuraConfigState.buildPatch())));
            assert.equal(patch.agent.action_policy.enabled,false);
            assert.equal(patch.agent.action_policy.allow[0].target,'fixture.txt');
            await f.page.evaluate(async()=>{
                fixture.config=AuraConfigState.snapshot().draft;
                AuraConfigState.commit(fixture.config);
                const host=document.getElementById('settings');
                host.replaceChildren();
                await AuraActionPolicy.mountSettings(host,fixture.config);
            });
            assert.equal(await f.page.getByLabel(f.text.enabled,{exact:true}).isChecked(),false);
            assert.equal(await f.page.evaluate(()=>AuraConfigState.isDirty()),false);
            await f.page.getByLabel(f.text.enabled,{exact:true}).press('Space');
            assert.equal(await f.page.locator('.action-policy-rule').first().locator('input').first().inputValue(),'fixture.txt');
        } finally { await f.close(); }
    });

    await test('actual Danger Zone renderer contains the policy card',async()=>{
        const f=await fixture({surface:'config',policy:'off'});
        try {
            await f.page.addScriptTag({url:origin+'/cfg/danger.js'});
            await f.page.evaluate(()=>{
                window.configData=fixture.config;
                window.runtimeData={}; window.helpTexts={}; window.lang='de';
                window.t=key=>key; window.attachChangeListeners=()=>{};
                document.querySelector('main').id='content';
                renderDangerZoneSection({label:'Danger Zone',desc:''});
            });
            await f.page.getByRole('button',{name:f.text.pending,exact:true}).waitFor();
            assert.equal(await f.page.locator('#content > .cfg-section > .action-policy-panel.cfg-topic > .pw-panel-body').count(),1);
        } finally { await f.close(); }
    });

    for (const mode of ['dark','light']) for (const width of [1280,390]) for (const density of ['comfortable','compact']) {
        await test(`Config card and rule controls ${mode}-${width}-${density}`,async()=>{
            const f=await fixture({surface:'config',mode,width,density});
            try {
                await f.page.getByRole('button',{name:f.text.add,exact:true}).first().click();
                const metrics=await f.page.locator('.action-policy-panel').evaluate(panel=>{
                    const body=panel.querySelector('.pw-panel-body');
                    const heading=panel.querySelector('.pw-panel-heading');
                    return {
                        radius:getComputedStyle(panel).borderRadius,
                        padding:getComputedStyle(panel).paddingTop,
                        inset:getComputedStyle(body).paddingLeft,
                        heading:getComputedStyle(heading).fontSize,
                        dropdowns:[...panel.querySelectorAll('select')].every(control=>getComputedStyle(control).backgroundImage!=='none' && parseFloat(getComputedStyle(control).paddingRight)>=32),
                        controls:[...panel.querySelectorAll('.action-policy-field input, select, textarea, button')].map(control=>({font:parseFloat(getComputedStyle(control).fontSize),height:control.getBoundingClientRect().height})),
                        overflow:document.documentElement.scrollWidth>innerWidth+1
                    };
                });
                assert.equal(metrics.radius,'16px'); assert.equal(metrics.padding,'0px'); assert.equal(metrics.heading,'18px');
                assert.equal(metrics.inset,width<700 || density==='compact'?'16px':'24px');
                assert.equal(metrics.dropdowns,true,'Config dropdown indicators are missing');
                assert.ok(metrics.controls.every(control=>control.height>=44 && control.font>=16),JSON.stringify(metrics));
                assert.equal(metrics.overflow,false,'Config policy controls overflow');
                await screenshot(f.page,`rules-config-${mode}-${width}-${density}`);
            } finally { await f.close(); }
        });
    }

    await test('reopened settings reload tools after capability changes',async()=>{
        const f=await fixture();
        try {
            assert.equal(f.state.catalogRequests,1);
            f.state.catalog=[{type:'function',function:{name:'execute_shell',parameters:{properties:{command:{type:'string'}}}}}];
            await f.page.evaluate(async()=>{
                const host=document.getElementById('settings');
                host.replaceChildren();
                await AuraActionPolicy.mountSettings(host,fixture.config);
            });
            assert.equal(f.state.catalogRequests,2,'settings reused the catalog from before the config change');
            const options=await f.page.locator('.action-policy-rule').last().locator('select option').evaluateAll(nodes=>nodes.map(node=>node.value));
            assert.ok(options.includes('execute_shell'),'newly enabled tool is unavailable');
            assert.ok(!options.includes('filesystem'),'disabled tool remains in the preview');
        } finally { await f.close(); }
    });

    for(const theme of ['standard','fruity']) for(const mode of ['dark','light']) for(const width of [1280,390]) {
        const label=`${theme}-${mode}-${width}`;
        await test(`admin rules, preview, literal text and layout ${label}`,async()=>{
            const f=await fixture({theme,mode,width});
            try {
                const {page,text,state}=f;
                await page.getByRole('button',{name:text.add,exact:true}).first().click();
                const rule=page.locator('.action-policy-rule').first();
                await rule.locator('select').nth(0).selectOption('filesystem');
                await rule.locator('select').nth(1).selectOption('write_file');
                await rule.locator('input').nth(0).fill(attack); await rule.locator('input').nth(0).blur();
                await rule.locator('select').nth(2).selectOption('mission');
                await rule.locator('input').nth(1).fill('mission-fixture'); await rule.locator('input').nth(1).blur();
                const draft=await page.evaluate(()=>({policy:fixture.config.agent.action_policy,dirty:fixture.dirty,atomic:fixture.atomic}));
                assert.equal(draft.policy.allow[0].target,attack); assert.equal(draft.policy.allow[0].operation,'write_file'); assert.equal(draft.policy.allow[0].run_type,'mission'); assert.equal(draft.policy.allow[0].scope_id,'mission-fixture'); assert.ok(draft.dirty); assert.ok(draft.atomic.includes('agent.action_policy'));
                const preview=page.locator('.action-policy-rule').last();
                await preview.locator('select').selectOption('filesystem');
                await page.locator('textarea').fill(JSON.stringify({operation:'delete',path:'fixture.txt'}));
                await page.getByRole('button',{name:text.check,exact:true}).click();
                await page.waitForFunction(()=>document.querySelector('.action-policy-state').textContent.includes('filesystem / delete'));
                assert.equal(state.previews.length,1); assert.equal(state.previews[0].arguments.operation,'delete');
                assert.ok(await page.locator('.action-policy-state').textContent().then(value=>value.includes(text.denied)));
                await noInjection(page);
                await screenshot(page,`rules-${label}`);
                const bounds=await page.evaluate(()=>({scroll:document.documentElement.scrollWidth,viewport:innerWidth}));
                assert.ok(bounds.scroll<=bounds.viewport+1,`horizontal overflow ${JSON.stringify(bounds)}`);
                await page.getByRole('button',{name:text.remove,exact:true}).click();
                assert.equal(await page.evaluate(()=>fixture.config.agent.action_policy.allow.length),0);
                await page.locator('textarea').fill('{invalid');
                await page.getByRole('button',{name:text.check,exact:true}).click();
                assert.equal(state.previews.length,1,'invalid JSON reached API');
            } finally { await f.close(); }
        });

        await test(`pending exact approval, double click and text safety ${label}`,async()=>{
            const f=await fixture({theme,mode,width});
            try {
                const {page,text,state}=f;
                state.decisionDelay=350; state.decisionResult={status:'success',result:attack};
                const item=pending(`approve-${label}`);
                state.pending=[item,item];
                await page.evaluate(()=>AuraActionPolicy.load('fixture/session-A'));
                await page.locator('dialog[open]').waitFor();
                assert.equal(await page.locator('dialog').count(),1,'duplicate pending dialog');
                assert.deepEqual(state.pendingRequests,['fixture/session-A']);
                assert.ok(await page.locator('dialog dd').last().textContent().then(value=>value===attack));
                await noInjection(page);
                await screenshot(page,`approval-${label}`);
                const box=await page.locator('dialog').boundingBox(); assert.ok(box.x>=0 && box.x+box.width<=width+1,'approval dialog leaves viewport');
                await page.getByRole('button',{name:text.approve,exact:true}).dblclick();
                assert.equal(state.decisions.length,1,'double click sent duplicate decision');
                assert.equal(state.decisions[0].session,'fixture/session-A');
                assert.deepEqual(state.decisions[0].body,{id:item.id,decision:'approve'});
                assert.equal(await page.getByRole('button',{name:text.reject,exact:true}).isDisabled(),true);
                await page.waitForFunction(done=>document.querySelector('dialog [role=status]').textContent.startsWith(done),text.done);
                await noInjection(page);
                await closeDialog(page,text);
            } finally { await f.close(); }
        });
    }

    for(const theme of ['standard','fruity']) {
        await test(`reject, expiry, denied admin and uncertain transport ${theme}`,async()=>{
            const f=await fixture({theme});
            try {
                const {page,text,state}=f;
                state.decisionResult={status:'denied',result:'Rejected fixture'};
                await show(page,pending(`reject-${theme}`));
                await page.getByRole('button',{name:text.reject,exact:true}).click();
                await page.waitForFunction(denied=>document.querySelector('dialog [role=status]').textContent.startsWith(denied),text.denied);
                assert.equal(state.decisions[0].body.decision,'reject'); await closeDialog(page,text);
                await show(page,pending(`expire-${theme}`,{expires_at:new Date(Date.now()+400).toISOString()}));
                await page.waitForFunction(expired=>document.querySelector('dialog [role=status]').textContent===expired,text.expired);
                assert.equal(await page.getByRole('button',{name:text.approve,exact:true}).isDisabled(),true);
                assert.equal(state.decisions.length,1); await closeDialog(page,text);
                state.decisionStatus=403;
                await show(page,pending(`forbidden-${theme}`)); await page.getByRole('button',{name:text.approve,exact:true}).click();
                await page.waitForFunction(message=>document.querySelector('dialog [role=status]').textContent===message,text.admin_only); await closeDialog(page,text);
                state.decisionStatus=200; state.abortDecision=true;
                await show(page,pending(`unknown-${theme}`)); await page.getByRole('button',{name:text.approve,exact:true}).click();
                await page.waitForFunction(message=>document.querySelector('dialog [role=status]').textContent===message,text.unknown);
                assert.equal(state.decisions.length,3,'uncertain mutation automatically repeated');
                assert.equal(await page.getByRole('button',{name:text.approve,exact:true}).isDisabled(),true); await closeDialog(page,text);
                await page.evaluate(item=>{void AuraActionPolicy.show(item);},pending(`already-expired-${theme}`,{expires_at:new Date(Date.now()-1000).toISOString()}));
                await page.evaluate(()=>Promise.resolve()); assert.equal(await page.locator('dialog').count(),0);
            } finally { await f.close(); }
        });
    }

    for(const lang of locales) await test(`localized real settings and approval ${lang}`,async()=>{
        const f=await fixture({lang,theme:locales.indexOf(lang)%2?'fruity':'standard'});
        try {
            await f.page.getByRole('heading',{name:f.text.title,exact:true}).waitFor();
            await show(f.page,pending(`locale-${lang}`));
            await f.page.getByRole('heading',{name:f.text.approval_title,exact:true}).waitFor();
            await f.page.getByRole('button',{name:f.text.approve,exact:true}).waitFor();
            await closeDialog(f.page,f.text);
        } finally { await f.close(); }
    });

    await test('non-administrator cannot edit rules',async()=>{
        const f=await fixture({admin:false});
        try { assert.equal(await f.page.locator('input[type=checkbox]').isDisabled(),true); assert.equal(await f.page.getByRole('button',{name:f.text.add,exact:true}).count(),0); assert.equal(await f.page.locator('.action-policy-state').textContent(),f.text.admin_only); }
        finally { await f.close(); }
    });

    await test('administrator pending refresh covers all sessions and keeps decision session',async()=>{
        const f=await fixture();
        try {
            const item=pending('admin-all-sessions',{session_id:'fixture/other-session'});
            f.state.pending=[item];
            await f.page.getByRole('button',{name:f.text.pending,exact:true}).click();
            await f.page.locator('dialog[open]').waitFor();
            assert.equal(f.state.pendingQueries[0].all,'1','administrator refresh is restricted to the default session');
            await f.page.getByRole('button',{name:f.text.reject,exact:true}).click();
            await f.page.waitForFunction(done=>document.querySelector('dialog [role=status]').textContent.startsWith(done),f.text.done);
            assert.equal(f.state.decisions[0].session,item.session_id,'decision changed the pending action session');
            await closeDialog(f.page,f.text);
        } finally { await f.close(); }
    });

    await test('German run-type options are translated',async()=>{
        const f=await fixture();
        try {
            await f.page.getByRole('button',{name:f.text.add,exact:true}).first().click();
            const options=await f.page.locator('.action-policy-rule').first().locator('select').nth(2).locator('option').allTextContents();
            for(const value of ['interactive','background','coagent','maintenance']) assert.ok(!options.includes(value),`untranslated run type: ${value}`);
        } finally { await f.close(); }
    });

    for(const mode of ['dark','light']) await test(`real Config ${mode} dialog contrast`,async()=>{
        const f=await fixture({surface:'config',mode});
        try {
            await show(f.page,pending(`config-${mode}`));
            await screenshot(f.page,`approval-config-${mode}`);
            const colors=await f.page.locator('dialog').evaluate(dialog=>({foreground:getComputedStyle(dialog).color,background:getComputedStyle(dialog).backgroundColor}));
            const luminance=color=>{
                const values=color.match(/[\d.]+/g).slice(0,3).map(Number).map(value=>{value/=255;return value<=0.04045?value/12.92:((value+0.055)/1.055)**2.4;});
                return .2126*values[0]+.7152*values[1]+.0722*values[2];
            };
            const a=luminance(colors.foreground),b=luminance(colors.background);
            const contrast=(Math.max(a,b)+.05)/(Math.min(a,b)+.05);
            assert.ok(contrast>=4.5,`dialog text contrast ${contrast.toFixed(2)}:1 ${JSON.stringify(colors)}`);
            await closeDialog(f.page,f.text);
        } finally { await f.close(); }
    });
} finally {
    await browser.close(); await new Promise(resolve=>server.close(resolve));
    report.finished_at=new Date().toISOString();
    await writeFile(path.join(reports,'results.json'),JSON.stringify(report,null,2)+'\n');
}
console.log(`${report.passed.length} passed, ${report.failures.length} failed; ${reports}`);
if(report.failures.length) process.exitCode=1;
