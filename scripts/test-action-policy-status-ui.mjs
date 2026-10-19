import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Source renderers, real theme CSS, synthetic records only. No running AuraGo is used.
// Run: node scripts/test-action-policy-status-ui.mjs
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const reports = path.join(root, 'reports/audit_harness_20261018/action-policy-status');
const locales = ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
const families = ['dashboard','desktop','missions','easydrag'];
const keys = {
    dashboard: ['dashboard.activity_task_blocked','dashboard.activity_action_rule_required','dashboard.activity_action_rules'],
    desktop: ['desktop.mc_state_blocked','desktop.mc_action_rule_required','desktop.mc_action_rules'],
    missions: ['missions.card_blocked','missions.action_rule_required','missions.action_rules'],
    easydrag: ['easydrag.ui.error_flow_action_rule_required']
};
const dictionaries = {};
for (const lang of locales) {
    dictionaries[lang] = {};
    for (const family of families) Object.assign(dictionaries[lang], JSON.parse(await readFile(path.join(root, `ui/lang/${family}/${lang}.json`), 'utf8')));
}
const report = { started_at: new Date().toISOString(), passed: [], failures: [], screenshots: [] };
await mkdir(reports, { recursive: true });
function playwright() {
    const require = createRequire(import.meta.url);
    for (const candidate of [process.env.AURAGO_PLAYWRIGHT_MODULE, 'playwright', path.join(os.homedir(), '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright')].filter(Boolean)) {
        try { return require(candidate); } catch (error) { if (error.code !== 'MODULE_NOT_FOUND') throw error; }
    }
    throw new Error('Set AURAGO_PLAYWRIGHT_MODULE to an installed Playwright module.');
}
const server = createServer(async (request, response) => {
    try {
        const pathname = new URL(request.url, 'http://127.0.0.1').pathname;
        if (pathname === '/') { response.setHeader('Content-Type', 'text/html'); response.end('<!doctype html><html data-theme="dark"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head><body></body></html>'); return; }
        const relative = decodeURIComponent(pathname).slice(1);
        if (!relative || relative.includes('\\') || relative.split('/').includes('..')) { response.writeHead(404).end(); return; }
        const data = await readFile(path.join(root, 'ui', relative));
        response.setHeader('Content-Type', ({'.js':'application/javascript','.css':'text/css','.json':'application/json','.woff2':'font/woff2'})[path.extname(relative)] || 'application/octet-stream');
        response.end(data);
    } catch { response.writeHead(404).end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
let browser;
async function test(name, run) {
    try { await run(); report.passed.push(name); console.log(`PASS ${name}`); }
    catch (error) { report.failures.push({name,error:error.stack || String(error)}); console.error(`FAIL ${name}: ${error.message}`); }
}
try {
    await test('all 16 locales contain translated blocked-state diagnostics', () => {
        for (const lang of locales) for (const family of families) for (const key of keys[family]) {
            assert.equal(typeof dictionaries[lang][key], 'string', `${lang}: ${key}`);
            assert.ok(dictionaries[lang][key].trim(), `${lang}: empty ${key}`);
            if (lang !== 'en') assert.notEqual(dictionaries[lang][key], dictionaries.en[key], `${lang}: English copy ${key}`);
        }
    });
    browser = await playwright().chromium.launch({headless:true,channel:process.env.AURAGO_BROWSER_CHANNEL || 'chrome',chromiumSandbox:true,args:['--disable-background-networking','--disable-component-update']});
    for (const lang of locales) for (const theme of ['standard','fruity']) {
        await test(`${lang}/${theme}: blocked background, mission list/detail/history, mission cards, flow diagnosis`, async () => {
            const page = await browser.newPage({viewport:{width:1280,height:1000},reducedMotion:'reduce'});
            const errors = [], api = [];
            page.on('pageerror', error => errors.push(error.message));
            await page.route('**/api/**', route => { api.push(route.request().url()); return route.fulfill({json:{}}); });
            try {
                await page.goto(origin);
                for (const css of ['shared-variables.css','shared-components.css','css/tokens.css','css/desktop-shell.bundle.css','css/desktop-app-mission-control.css','css/missions.css','css/precision-workspace.css','css/dashboard.css']) await page.addStyleTag({url:`${origin}/${css}`});
                await page.evaluate(({lang,theme,text}) => {
                    document.documentElement.lang=lang;
                    document.body.className='desktop-body';
                    document.body.dataset.theme=theme;
                    document.body.dataset.fruityMode='dark';
                    document.body.innerHTML='<main style="padding:20px;overflow:auto"><section class="pw-page" data-workspace-page="dashboard"><h2>Background</h2><div id="activity-stats"></div><div id="activity-details"></div></section><section class="vd-mc" style="height:auto"><div id="list"></div><div id="detail"></div></section><section class="pw-page" data-workspace-page="missions"><div id="cards"></div><div id="compact"></div></section><p id="flow"></p></main>';
                    window.t=(key,params={})=>Object.entries(params).reduce((s,[k,v])=>s.replaceAll('{'+k+'}',String(v)),text[key] || key);
                    window.esc=value=>String(value ?? '').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&#39;');
                    window.truncate=(value,n)=>String(value).slice(0,n);
                    window.DashSections={register(){}};
                },{lang,theme,text:dictionaries[lang]});
                // Loading after DOMContentLoaded avoids the real page's fetch/SSE initialization.
                for (const source of ['js/dashboard/sections/tools.js','js/missions/main.js','js/desktop/apps/mission-control-triggers.js','js/desktop/apps/mission-control-list.js','js/desktop/apps/mission-control-detail.js','js/desktop/apps/easydrag-core.js']) await page.addScriptTag({url:`${origin}/${source}`});
                const observed = await page.evaluate(() => {
                    const attack='<img src=x onerror="window.injected=true">';
                    const mission={id:'fixture-blocked',name:attack,prompt:attack,execution_type:'manual',enabled:true,last_result:'blocked',last_output:'action_rule_required',status:'idle',run_count:1,last_run:'2026-10-18T12:00:00Z'};
                    renderActivity({background_task_summary:{blocked:2},background_tasks:[{id:'task-blocked',description:attack,type:'follow_up',status:'blocked',last_error:'action_rule_required'},{id:'isolated-blocked',description:'Isolated code',type:'isolated_code',status:'blocked',last_error:'action_rule_required: background action identity or policy gate unavailable'}]});
                    const deps={esc,t,lang:document.documentElement.lang,svg:{},triggers:MissionControlTriggers,schedule:{},readonly:false,fmt:{relative:()=>'',dateTime:value=>value,duration:value=>String(value)}};
                    const list=MissionControlList.create(deps); document.getElementById('list').append(list.element); list.setData({missions:[mission],queue:{running:'',items:[]}}); list.setFilter('errors');
                    const detail=MissionControlDetail.create(deps); document.getElementById('detail').append(detail.element); detail.setMission(mission); detail.setHistory({items:[{id:'blocked-run',status:'blocked',started_at:mission.last_run,output:'action_rule_required',error_msg:'action_rule_required'}],total:1});
                    document.getElementById('cards').innerHTML=renderMissionGrid(mission,false);
                    document.getElementById('compact').innerHTML=renderMissionCompact(mission);
                    document.getElementById('flow').textContent=EasyDrag.core.stepErrorText(t,{status:'error',error_code:'FLOW_ACTION_RULE_REQUIRED',error_message:''});
                    bindMissionUI();
                    window.statusFixture={list,detail};
                    return {listState:list.element.querySelector('[data-mc-id]').dataset.state,counts:list.counts(),listIds:list.visibleIds(),injected:!!window.injected,images:document.querySelectorAll('img').length,background:document.getElementById('activity-details').textContent,flow:document.getElementById('flow').textContent,overview:detail.element.querySelector('[data-mc-rule-blocked]').textContent,chip:document.querySelector('#cards .mc-status-chip').textContent};
                });
                const text=dictionaries[lang];
                assert.equal(observed.listState,'blocked');
                assert.equal(observed.counts.running,0); assert.equal(observed.counts.waiting,0);
                assert.deepEqual(observed.listIds,['fixture-blocked']);
                assert.ok(observed.background.includes(text['dashboard.activity_task_blocked']));
                assert.ok(observed.background.includes(text['dashboard.activity_action_rule_required']));
                assert.equal(observed.background.includes('action_rule_required'),false,'real isolated job reason must use the friendly diagnosis');
                assert.ok(observed.overview.includes(text['desktop.mc_action_rule_required']));
                assert.ok(observed.chip.includes(text['missions.card_blocked']));
                assert.equal(observed.flow,text['easydrag.ui.error_flow_action_rule_required']);
                assert.equal(observed.injected,false); assert.equal(observed.images,0);
                assert.equal(await page.locator('#cards .mc-rule-blocked').isVisible(),true);
                assert.equal(await page.locator('#detail [data-mc-rule-blocked] a').getAttribute('href'),'/config#danger');
                // The compact card's delegated edit handler must leave the configuration link alone.
                const popupPromise=page.waitForEvent('popup');
                await page.locator('#compact a[href="/config#danger"]').click();
                const popup=await popupPromise; await popup.close();
                await page.evaluate(()=>statusFixture.detail.setTab('history'));
                assert.equal(await page.locator('#detail .vd-mc-run [data-state="blocked"]').count(),2);
                await page.locator('#detail [data-mc-run-toggle]').click();
                assert.ok((await page.locator('#detail .vd-mc-run-body').textContent()).includes(text['desktop.mc_action_rule_required']));
                await page.evaluate(()=>statusFixture.detail.setTab('overview'));
                if(lang==='de') {
                    const screenshot=path.join(reports,`${theme}-dark.png`); await page.screenshot({path:screenshot,fullPage:true}); report.screenshots.push(screenshot);
                    await page.evaluate(()=>document.body.dataset.fruityMode='light');
                    await page.setViewportSize({width:390,height:900});
                    assert.equal(await page.locator('#detail [data-mc-rule-blocked]').isVisible(),true);
                    const narrow=path.join(reports,`${theme}-narrow.png`); await page.screenshot({path:narrow,fullPage:true}); report.screenshots.push(narrow);
                }
                assert.deepEqual(api,[],'status rendering or opening settings must not run/retry a task');
                assert.deepEqual(errors,[],'browser JavaScript errors');
            } finally { await page.close(); }
        });
    }
} finally {
    if(browser) await browser.close();
    await new Promise(resolve=>server.close(resolve));
    report.finished_at=new Date().toISOString();
    await writeFile(path.join(reports,'results.json'),JSON.stringify(report,null,2)+'\n');
    console.log(`${report.passed.length} passed; ${report.failures.length} failed. Evidence: ${reports}`);
    if(report.failures.length) process.exitCode=1;
}
