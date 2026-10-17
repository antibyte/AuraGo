import { createServer } from 'node:http';
import { readFile, mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import assert from 'node:assert/strict';

const require = createRequire(import.meta.url);
const { chromium } = process.env.AURAGO_PLAYWRIGHT ? require(process.env.AURAGO_PLAYWRIGHT) : require('playwright');
const locales = ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
let setup = { mode:'docker', receiver_ip:'192.168.4.20', udp_port:5010, udp_enabled:false, candidates:['192.168.4.20'] };
let setupDelay = 0, setupError = false, fresh = true, posts = 0, statusError = 0;
const knownNodes=[{node_id:1,csi_status:'stale'},{node_id:7,csi_status:'active',csi_last_seen_ms:42,csi_sequence:3}];
let nodeData={nodes:knownNodes},nodeError=false,nodeDelay=0;
const registrations=[];
let registrationError=false;
let catalogFault = '';
const buildVersion = 'ruview-browser-fixture', resourceRequests = [];
const server = createServer(async (req, res) => {
    try {
        const url = new URL(req.url, 'http://localhost');
        if (url.pathname.startsWith('/js/ruview/') || url.pathname.startsWith('/js/vendor/ruview-')) resourceRequests.push(url);
        if (url.pathname.startsWith('/api/desktop/integrations/ruview/') || url.pathname.startsWith('/api/ruview/')) {
            res.setHeader('Content-Type', 'application/json');
            const route = url.pathname.split('/ruview/')[1];
            if (req.method === 'POST') posts++;
            if (route === 'board-setup') {
                const snapshot = structuredClone(setup);
                if (setupDelay) await new Promise(resolve => setTimeout(resolve, setupDelay));
                res.statusCode = setupError ? 503 : 200;
                return res.end(JSON.stringify(setupError ? { error:'unavailable' } : snapshot));
            }
            if (route === 'firmware') {
                const catalog = JSON.parse(await readFile('internal/ruview/artifacts.json'));
                if (catalogFault === 'missing-offset') for (const profile of catalog.firmware) delete profile.parts[0].offset;
                if (catalogFault === 'revision') catalog.revision = '0'.repeat(40);
                return res.end(JSON.stringify(catalog));
            }
            if (route === 'status' && statusError) {res.statusCode=statusError;return res.end(JSON.stringify({error:'fixture status unavailable'}));}
            if (route === 'nodes' && req.method === 'POST') {
                let body='';for await(const chunk of req)body+=chunk;
                const payload=JSON.parse(body);
                assert.deepEqual(Object.keys(payload),['node_id'],'only the node ID may reach AuraGo');
                if(registrationError){res.statusCode=503;return res.end('{}');}
                registrations.push(payload.node_id);
                return res.end('{}');
            }
            if (route === 'proxy/api/v1/nodes' || route === 'nodes') {
                const snapshot=structuredClone(nodeData);
                if(nodeDelay)await new Promise(resolve=>setTimeout(resolve,nodeDelay));
                res.statusCode=nodeError?503:200;
                return res.end(JSON.stringify(nodeError?{error:'unavailable'}:snapshot));
            }
            return res.end(JSON.stringify({enabled:true,readonly:false,mode:'docker',source:'esp32',fresh,signal:'CSI',data_state:'unchecked'}));
        }
        if (url.pathname === '/') {
            const lang = locales.includes(url.searchParams.get('lang')) ? url.searchParams.get('lang') : 'de';
            const dict = JSON.parse(await readFile(`ui/lang/ruview/${lang}.json`));
            const theme = url.searchParams.get('theme') === 'standard' ? 'standard' : 'fruity';
            const mode = url.searchParams.get('mode') === 'dark' ? 'dark' : 'light';
            res.setHeader('Content-Type','text/html; charset=utf-8');
            return res.end(`<!doctype html><html data-theme="dark"><head><meta charset="utf-8"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/ruview.css"></head><body class="desktop-body" data-theme="${theme}" data-fruity-mode="${mode}"><main id="app"></main><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/apps/ruview.js"></script><script>window.BUILD_VERSION=${JSON.stringify(buildVersion)};const dict=${JSON.stringify(dict)};RuViewApp.render(document.getElementById('app'),'fixture',{t:k=>dict[k]||k,esc:s=>String(s).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';')});</script></body></html>`);
        }
        const relative = url.pathname.slice(1);
        if (!relative || relative.includes('..')) { res.writeHead(404).end(); return; }
        const data = await readFile(path.join('ui', relative));
        res.setHeader('Content-Type', ({'.js':'application/javascript','.mjs':'application/javascript','.css':'text/css','.json':'application/json'})[path.extname(relative)] || 'application/octet-stream');
        res.end(data);
    } catch { res.writeHead(404).end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const browser = await chromium.launch({headless:true,channel:'chrome'});
await mkdir('reports/ruview-wizard', {recursive:true});
try {
    const page = await browser.newPage();
    page.setDefaultTimeout(15000);
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.addInitScript(() => {
        window.serialFixture = {cancel:false, fail:false, mismatch:false, requests:0, configured:null, usbProductId:0x1001};
        Object.defineProperty(navigator, 'serial', {value:{requestPort:async () => {
            const state = window.serialFixture;
            state.requests++;
            if (state.cancel) throw new DOMException('No port selected', 'NotFoundError');
            let receiver;
            const port = {
                getInfo:() => ({usbVendorId:0x303a,usbProductId:state.usbProductId}),
                async open() {
                    if (state.fail) throw Error('Fixture port busy');
                    this.readable = new ReadableStream({start(controller){receiver=controller;}});
                    this.writable = new WritableStream({write(bytes){
                        const parts = new TextDecoder().decode(bytes).trim().split(' ');
                        const nonce = parts[1];
                        let response;
                        if (parts[0] === 'RUVIEW_HELLO_V1') {
                            const c = state.configured || {node:'0',ip:'0.0.0.0',port:'0'};
                            response = `RUVIEW_HELLO_OK_V1 nonce=${nonce} chip=esp32s3 version=0.8.12 device_digest=${state.mismatch?'0123456789abcdef':'abcdef0123456789'} configured=${state.configured?'1':'0'} node_id=${c.node} target_ip=${c.ip} target_port=${c.port}\n`;
                        } else {
                            state.configured = {node:parts[2],ip:parts[3],port:parts[4]};
                            response = `RUVIEW_CONFIG_OK_V1 nonce=${nonce} node_id=${parts[2]} rebooting=1\n`;
                        }
                        receiver.enqueue(new TextEncoder().encode(response));
                    }});
                },
                async close() {}
            };
            return port;
        }}});
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    const open = async (query='theme=standard&lang=de') => {
        await page.goto(origin+'/?'+query);
        await page.locator('[data-action="flash"]').click();
        await page.locator('dialog[open]').waitFor();
    };
    const ready = async () => { await page.waitForFunction(() => !document.querySelector('dialog [data-action="configure"]').disabled); };
    for (const theme of ['standard','fruity-light','fruity-dark']) for (const width of [1100,390]) for (const lang of locales) {
        await page.setViewportSize({width,height:900});
        await open(`theme=${theme==='standard'?'standard':'fruity'}&mode=${theme.endsWith('dark')?'dark':'light'}&lang=${lang}`);
        await ready();
        assert.equal(await page.locator('dialog [name="node"]').inputValue(),'2','known node IDs, including stale boards, must not be suggested again');
        assert.ok(resourceRequests.every(url=>url.searchParams.get('v')===buildVersion),'wizard dependencies must use the same resource version as the Desktop');
        assert.equal(await page.locator('dialog section h3').count(), 3);
        assert.ok(!/ruview\.[a-z_]+/.test(await page.locator('dialog').innerText()), `translations ${lang}`);
        const metrics = await page.locator('dialog').evaluate(dialog => {
            const c = dialog.querySelector('[name="keep"]'), box=c.getBoundingClientRect();
            const colors = getComputedStyle(dialog).backgroundColor.match(/[\d.]+/g).map(Number);
            return {width:box.width,height:box.height,appearance:getComputedStyle(c).appearance,scheme:getComputedStyle(dialog).colorScheme,brightness:colors.slice(0,3).reduce((a,b)=>a+b)/3,overflow:dialog.scrollWidth>dialog.clientWidth+1};
        });
        assert.deepEqual([metrics.width,metrics.height,metrics.appearance],[20,20,'auto'],`${theme}: checkbox geometry`);
        assert.equal(metrics.overflow,false,`${theme}/${width}/${lang}: overflow`);
        assert.equal(metrics.scheme,theme==='fruity-light'?'light':'dark');
        assert.ok(theme==='fruity-light'?metrics.brightness>150:metrics.brightness<100,`${theme}: wrong dialog surface`);
        await page.locator('dialog [name="ssid"]').fill('memory-only');
        await page.locator('dialog [name="password"]').fill('fixture-password');
        await page.locator('dialog .ruview-check').last().click();
        assert.equal(await page.locator('dialog [name="keep"]').isChecked(),true);
        assert.equal(await page.locator('dialog [name="ssid"]').isDisabled(),true);
        await page.locator('dialog [name="keep"]').press('Space');
        assert.equal(await page.locator('dialog [name="ssid"]').inputValue(),'memory-only');
        if (lang==='de') {
            await page.locator('dialog .ruview-fields').scrollIntoViewIfNeeded();
            await page.screenshot({path:`reports/ruview-wizard/${theme}-${width}.png`});
        }
        await page.locator('dialog button[value="close"]').click();
    }
    assert.equal(posts,0,'opening the wizard must never test, start or configure the runtime');
    await page.setViewportSize({width:1100,height:900});
    for(const [data,expected] of [
        [{nodes:[]},'1'],
        [[{id:'1'},{node_id:3,csi_status:'stale'},{node_id:2},{node_id:1},{node_id:0}],'4'],
        [{nodes:Array.from({length:254},(_,i)=>({node_id:i+1}))},'255']
    ]){
        nodeData=data;await open();await ready();
        assert.equal(await page.getByRole('spinbutton',{name:'Knoten-ID',exact:true}).inputValue(),expected);
        assert.ok((await page.locator('#ruview-node-hint').innerText()).includes(`ID: ${expected}.`));
    }
    for(const data of [{nodes:Array.from({length:255},(_,i)=>({node_id:i+1}))},{},{nodes:[{}]},{nodes:[{node_id:true}]}]){
        nodeData=data;await open();
        await page.waitForFunction(()=>!document.querySelector('#ruview-node-hint').textContent.includes('werden geprüft'));
        assert.equal(await page.locator('dialog [name="node"]').inputValue(),'');
        assert.equal(await page.locator('dialog [data-action="configure"]').isDisabled(),true);
        assert.equal(await page.locator('dialog [name="node"]').isEnabled(),true);
        await page.locator('dialog [name="node"]').fill('1');await ready();
        assert.equal(await page.locator('dialog [name="node"]').inputValue(),'1','existing IDs remain selectable for deliberate reconfiguration');
    }
    nodeData={nodes:knownNodes};nodeError=true;
    await open();await page.waitForFunction(()=>document.querySelector('#ruview-node-hint').textContent.includes('nicht geprüft'));
    assert.equal(await page.locator('dialog [name="node"]').inputValue(),'','failed discovery must not silently suggest 1');
    await page.locator('dialog [name="node"]').fill('42');await ready();
    nodeError=false;nodeDelay=800;
    await open();await page.locator('dialog [name="node"]').fill('42');await ready();
    assert.equal(await page.locator('dialog [name="node"]').inputValue(),'42','late discovery must preserve a manually entered ID');
    await open();await page.locator('dialog button[value="close"]').click();
    await page.locator('dialog').waitFor({state:'detached'});
    nodeDelay=0;
    await page.evaluate(async()=>{
        const {openWizard}=await import(window.AuraLazyAssets.versionedURL('/js/ruview/wizard.mjs'));
        await openWizard({base:'/api/ruview/',t:key=>dict[key]||key});
    });
    await ready();assert.equal(await page.locator('dialog [name="node"]').inputValue(),'2','the Config API entry must receive the same proposal');
    assert.equal(posts,0,'ID discovery must only read existing nodes');
    await page.locator('dialog button[value="close"]').click();
    await page.locator('dialog').waitFor({state:'detached'});
    await page.evaluate(async()=>{
        const {openWizard}=await import(window.AuraLazyAssets.versionedURL('/js/ruview/wizard.mjs'));
        await openWizard({base:'/api/ruview/',t:key=>dict[key]||key,nodeID:9});
    });
    await ready();
    assert.equal(await page.locator('dialog [name="node"]').inputValue(),'9','an explicit node id stays when discovery would suggest another');
    catalogFault='missing-offset';
    await open(); await ready();
    catalogFault=''; // The server is repaired while this dialog remains open.
    const input = name => page.locator(`dialog [name="${name}"]`);
    const action = name => page.locator(`dialog [data-action="${name}"]`);
    const wifiDraft = async () => [await input('ssid').inputValue(),await input('password').inputValue()];
    assert.equal(await input('receiver').inputValue(),'192.168.4.20');
    assert.equal(await input('port').inputValue(),'5010');
    assert.equal(await page.locator('.ruview-udp-warning').isVisible(),true);
    assert.equal(await action('readback').isDisabled(),true);
    assert.equal(await action('verify').isDisabled(),true);
    await input('ssid').fill('memory-only'); await input('password').fill('fixture-password'); await input('node').fill('7');
    await page.evaluate(() => serialFixture.cancel=true);
    await action('flash').click();
    await page.waitForFunction(() => document.querySelector('dialog section [role="status"]').textContent.includes('Abgebrochen'));
    assert.equal(await input('ssid').inputValue(),'memory-only');
    assert.equal(await input('password').inputValue(),'fixture-password');
    assert.equal(await page.locator('.ruview-flash-progress').isVisible(),false,'cancelled flashing must not leave a running progress bar');
    assert.equal(await action('flash').getAttribute('aria-busy'),'false');
    await page.evaluate(async () => {
        serialFixture.cancel=false;
        const { ESPLoader, Transport } = await import(window.AuraLazyAssets.versionedURL('/js/vendor/ruview-esptool.js'));
        const catalog=await (await fetch('/api/desktop/integrations/ruview/firmware')).json();
        const profile=catalog.firmware.find(p=>p.id==='esp32s3-16mb-cdc');
        const images=new Map(await Promise.all(profile.parts.map(async p=>[p.offset,new Uint8Array(await (await fetch(window.AuraLazyAssets.versionedURL(`/js/vendor/ruview-firmware/${catalog.revision}/${profile.id}/${p.name}`))).arrayBuffer())])));
        serialFixture.flashWrites=0;
        const hold=async phase=>{if(!serialFixture.hold?.[phase])return;delete serialFixture.hold[phase];await new Promise(resolve=>serialFixture.resume=resolve);};
        ESPLoader.prototype.main=async function(){this.chip={CHIP_NAME:'ESP32-S3'};await hold('connecting');};
        ESPLoader.prototype.readFlashId=async()=>24<<16;
        ESPLoader.prototype.readFlash=async address=>{if(serialFixture.flashWrites){await hold('verifying');if(serialFixture.verifyFailure)return new Uint8Array(images.get(address).length);}return images.get(address);};
        ESPLoader.prototype.writeFlash=async options=>{serialFixture.flashWrites++;const i=options.fileArray.length-1,total=options.fileArray[i].data.length;options.reportProgress(i,total/2,total);await hold('writing');options.reportProgress(i,total,total);};
        ESPLoader.prototype.after=async()=>hold('restarting');
        Transport.prototype.disconnect=async()=>{};
    });
    await input('profile').selectOption('esp32s3-8mb-cdc');
    await action('flash').click(); await ready();
    assert.match(await page.locator('dialog section').first().innerText(),/Erkannt: 16 MB Flash.*Profil: 8 MB/);
    assert.equal(await page.evaluate(()=>serialFixture.flashWrites),0);
    assert.equal(await input('password').inputValue(),'fixture-password');
    await input('profile').selectOption('esp32s3-16mb-cdc');
    await page.evaluate(()=>serialFixture.hold={connecting:true,writing:true,verifying:true,restarting:true});
    await action('flash').click();
    const flashStatus=page.locator('#ruview-flash-status'),next=page.locator('[data-flash-next]'),progress=page.locator('dialog progress');
    await page.waitForFunction(()=>document.querySelector('#ruview-flash-status').textContent.includes('Verbindung zum Board'));
    assert.equal(await progress.getAttribute('value'),null);
    assert.equal(await action('flash').getAttribute('aria-busy'),'true');
    assert.equal(await page.locator('dialog > [role="status"]').innerText(),'','idle readiness must not compete with an active phase');
    assert.equal(await next.isVisible(),false);
    await page.evaluate(()=>serialFixture.resume());
    await page.waitForFunction(()=>document.querySelector('#ruview-flash-status').textContent.includes('Firmware wird geschrieben'));
    assert.ok(await progress.evaluate(p=>p.value>0&&p.value<100));
    assert.match(await page.locator('.ruview-flash-progress output').innerText(),/^\d+ %$/);
    const captureFlashState=async state=>{
        for(const theme of ['standard','fruity-light','fruity-dark'])for(const width of [1100,390]){
            await page.setViewportSize({width,height:900});
            await page.evaluate(theme=>{document.body.dataset.theme=theme==='standard'?'standard':'fruity';document.body.dataset.fruityMode=theme.endsWith('dark')?'dark':'light';},theme);
            await (state==='writing'?action('flash'):next).scrollIntoViewIfNeeded();
            assert.equal(await page.locator('dialog').evaluate(d=>d.scrollWidth>d.clientWidth+1),false,`${state}/${theme}/${width}: overflow`);
            await page.screenshot({path:`reports/ruview-wizard/${state}-${theme}-${width}.png`});
        }
    };
    await captureFlashState('writing');
    assert.equal(await action('flash').evaluate(b=>getComputedStyle(b,'::before').animationName),'ruview-spin');
    await page.emulateMedia({reducedMotion:'reduce'});
    assert.equal(await action('flash').evaluate(b=>getComputedStyle(b,'::before').animationName),'none');
    await page.emulateMedia({reducedMotion:'no-preference'});
    await page.evaluate(()=>serialFixture.resume());
    await page.waitForFunction(()=>document.querySelector('#ruview-flash-status').textContent.includes('Geschriebene Firmware'));
    assert.equal(await progress.getAttribute('value'),null,'verification must remain visibly pending after writing reaches 100%');
    assert.equal(await next.isVisible(),false);
    await page.evaluate(()=>serialFixture.resume());
    await page.waitForFunction(()=>document.querySelector('#ruview-flash-status').textContent.includes('Board wird neu gestartet'));
    assert.equal(await action('flash').getAttribute('aria-busy'),'true');
    assert.equal(await next.isVisible(),false);
    await page.evaluate(()=>serialFixture.resume());await ready();
    assert.equal(await page.evaluate(()=>serialFixture.flashWrites),1);
    assert.match(await flashStatus.innerText(),/Schritt 1 abgeschlossen.*Firmware geschrieben und geprüft.*Schritt 2/);
    assert.equal(await progress.getAttribute('value'),'100');
    assert.equal(await action('flash').getAttribute('aria-busy'),'false');
    assert.equal(await next.isVisible(),true);
    assert.equal(await next.evaluate(b=>b===document.activeElement),true);
    await captureFlashState('complete');
    const requests=await page.evaluate(()=>serialFixture.requests);
    await next.click();
    assert.equal(await input('ssid').evaluate(i=>i===document.activeElement),true);
    assert.equal(await page.evaluate(()=>serialFixture.requests),requests,'continuing to step 2 must not start another serial operation');
    await input('keep').check();await next.click();
    assert.equal(await input('receiver').evaluate(i=>i===document.activeElement),true,'preserved Wi-Fi skips the disabled credentials');
    await input('keep').uncheck();
    assert.equal(await input('password').inputValue(),'fixture-password','flashing must preserve WLAN drafts for step 2');
    await page.evaluate(()=>serialFixture.usbProductId=0x4001);
    await action('flash').click();await ready();
    assert.equal(await page.evaluate(()=>serialFixture.flashWrites),2,'a second S3 16 MiB board using Espressif CDC 303a:4001 must flash in the same open dialog');
    assert.equal(await flashStatus.getAttribute('data-state'),'success');
    for (const fault of ['missing-offset','revision']) {
        catalogFault=fault;
        await action('flash').click();await ready();
        assert.equal(await flashStatus.getAttribute('data-state'),'error','each attempt must validate the current catalog');
        assert.equal(await page.evaluate(()=>serialFixture.flashWrites),2,'invalid current metadata must stop before writing');
        assert.equal(await input('password').inputValue(),'fixture-password');
    }
    catalogFault='';
    await action('flash').click();await ready();
    assert.equal(await page.evaluate(()=>serialFixture.flashWrites),3,'an explicit new attempt must recover after catalog repair');
    assert.equal(await flashStatus.getAttribute('data-state'),'success');
    assert.ok(resourceRequests.every(url=>url.searchParams.get('v')===buildVersion),'firmware images and the serial loader must retain the wizard resource version');
    await page.evaluate(()=>{serialFixture.verifyFailure=true;serialFixture.flashWrites=0;});
    await action('flash').click();await ready();
    assert.equal(await flashStatus.getAttribute('data-state'),'error');
    assert.equal(await next.isVisible(),false,'failed retries must clear the prior success action');
    assert.equal(await progress.isVisible(),false);
    assert.equal(await action('flash').getAttribute('aria-busy'),'false');
    assert.equal(await input('password').inputValue(),'fixture-password');
    await page.evaluate(()=>serialFixture.verifyFailure=false);
    await page.evaluate(() => {serialFixture.cancel=false;serialFixture.fail=true;});
    await action('configure').click();
    await page.waitForFunction(() => [...document.querySelectorAll('dialog [role="status"]')].some(p=>p.textContent.includes('Fixture port busy')));
    assert.equal(await input('password').inputValue(),'fixture-password');
    await page.evaluate(() => serialFixture.fail=false);
    await action('configure').click();
    await page.waitForFunction(() => !document.querySelector('dialog [data-action="readback"]').disabled);
    assert.equal(await input('node').inputValue(),'7','setup and retries must preserve the chosen node ID');
    assert.equal(await page.evaluate(()=>serialFixture.configured.node),'7','the selected ID must reach the board');
    assert.deepEqual(await wifiDraft(),['memory-only','fixture-password'],'acknowledged setup must retain both fields for retries after restart');
    assert.equal(await action('verify').isDisabled(),true);
    const setupRequests=await page.evaluate(()=>serialFixture.requests);
    await action('configure').click();await ready();
    assert.equal(await page.evaluate(()=>serialFixture.requests),setupRequests+1,'setup can be explicitly repeated without retyping credentials');
    assert.equal(await action('readback').isDisabled(),false);
    assert.deepEqual(await wifiDraft(),['memory-only','fixture-password']);
    await page.evaluate(()=>serialFixture.fail=true);
    await action('readback').click();await ready();
    assert.deepEqual(await wifiDraft(),['memory-only','fixture-password'],'a failed restart connection must retain the setup draft');
    await page.evaluate(()=>serialFixture.fail=false);
    await page.evaluate(() => serialFixture.mismatch=true);
    await action('readback').click(); await ready();
    assert.equal(await action('verify').isDisabled(),true,'a different board cannot confirm the settings');
    assert.deepEqual(await wifiDraft(),['memory-only','fixture-password'],'a mismatched board must not discard the draft');
    await page.evaluate(() => serialFixture.mismatch=false);
    assert.deepEqual(registrations,[],'cancelled, failed and mismatched readbacks must not register boards');
    registrationError=true;
    await action('readback').click();await ready();
    assert.ok((await page.locator('dialog section').nth(1).innerText()).includes('Geräteliste konnte nicht gespeichert'));
    assert.deepEqual(registrations,[]);
    registrationError=false;
    await action('readback').click();
    await page.waitForFunction(() => !document.querySelector('dialog [data-action="verify"]').disabled);
    assert.deepEqual(registrations,[7],'matching readback records the board independently of UDP reception');
    assert.deepEqual(await wifiDraft(),['memory-only','fixture-password'],'successful restart confirmation keeps the draft until dialog close');
    await input('ssid').fill('next-draft'); await input('password').fill('next-password');
    fresh=false; await action('verify').click(); await ready();
    assert.ok((await page.locator('dialog section').last().innerText()).includes('Noch keine frischen CSI-Daten'));
    assert.equal(await input('password').inputValue(),'next-password','sensor tests must not clear WLAN drafts');
    fresh=true; await action('verify').click(); await ready();
    assert.ok((await page.locator('dialog section').last().innerText()).includes('Frische Sensordaten empfangen'));
    assert.equal(posts,4,'two registration attempts and two explicit sensor tests');
    fresh=false;await action('verify').click();await ready();
    assert.equal(await page.locator('.ruview-steps li').nth(2).evaluate(el=>el.classList.contains('complete')),false,'failed repeated sensor check clears success');
    fresh=true;await action('verify').click();await ready();
    await page.evaluate(()=>serialFixture.mismatch=true);
    await action('readback').click();await ready();
    assert.equal(await action('verify').isDisabled(),true,'failed readback revokes the previous confirmation');
    assert.equal(await page.locator('.ruview-steps li').evaluateAll(steps=>steps.slice(1).every(step=>!step.classList.contains('complete'))),true,'failed readback clears configuration and sensor success');
    await page.evaluate(()=>serialFixture.mismatch=false);
    await action('readback').click();await ready();
    await action('verify').click();await ready();
    await page.evaluate(()=>serialFixture.cancel=true);
    await action('readback').click();await ready();
    assert.equal(await action('verify').isDisabled(),true,'cancelled readback revokes the previous confirmation');
    assert.equal(await page.locator('.ruview-steps li').evaluateAll(steps=>steps.slice(1).every(step=>!step.classList.contains('complete'))),true);
    assert.equal(await input('password').inputValue(),'next-password');
    assert.deepEqual(await wifiDraft(),['next-draft','next-password'],'cancelled repeated confirmation retains both fields');
    await page.evaluate(()=>serialFixture.cancel=false);
    assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
    await page.evaluate(()=>{window.closedInputs=[...document.querySelectorAll('dialog input')];});
    await page.locator('dialog button[value="close"]').click();
    await page.locator('dialog').waitFor({state:'detached'});
    assert.equal(await page.evaluate(()=>closedInputs.filter(i=>['ssid','password'].includes(i.name)).every(i=>i.value==='')),true);

    await open(); await ready();
    assert.deepEqual(await wifiDraft(),['',''],'closed drafts must not reappear when reopening');
    await input('ssid').fill('reconnect-draft'); await input('password').fill('fixture-password');
    statusError=503;
    await page.waitForFunction(()=>document.querySelector('dialog > [role="status"]')?.textContent.includes('Verbindung unterbrochen'));
    assert.equal(await input('password').inputValue(),'fixture-password','a transient status failure must preserve the open draft');
    await action('configure').click(); await ready();
    assert.equal(await input('password').inputValue(),'fixture-password','failed authorization refresh must never provision or discard the draft');
    await page.evaluate(()=>{window.closedInputs=[...document.querySelectorAll('dialog input')];});
    statusError=403;
    await page.locator('dialog').waitFor({state:'detached'});
    assert.equal(await page.evaluate(()=>closedInputs.filter(i=>['ssid','password'].includes(i.name)).every(i=>i.value==='')),true,'revoked access clears the retained draft');
    statusError=0;

    for(const event of ['aurago:auth-ended','aurago:desktop-policy']){
        await open();await ready();
        await input('ssid').fill('memory-only');await input('password').fill('fixture-password');
        await action('configure').click();await ready();
        assert.deepEqual(await wifiDraft(),['memory-only','fixture-password']);
        await page.evaluate(event=>{
            window.closedInputs=[...document.querySelectorAll('dialog input')];
            window.dispatchEvent(new CustomEvent(event,{detail:{readonly:true}}));
        },event);
        await page.locator('dialog').waitFor({state:'detached'});
        assert.equal(await page.evaluate(()=>closedInputs.filter(i=>['ssid','password'].includes(i.name)).every(i=>i.value==='')),true,`${event} clears acknowledged credentials`);
    }

    setup={...setup,receiver_ip:'',candidates:['192.168.4.20','10.1.0.2','127.0.0.1'],udp_enabled:true};
    await open(); await ready();
    assert.equal(await input('receiver').inputValue(),'','unverified addresses must be explicitly chosen');
    assert.equal(await page.locator('[data-use-receiver]').isDisabled(),true);
    await input('receiver_choice').selectOption('10.1.0.2');
    await page.locator('[data-use-receiver]').click();
    assert.equal(await input('receiver').inputValue(),'10.1.0.2');
    assert.equal(await input('receiver_choice').locator('option[value="127.0.0.1"]').count(),0);
    setup={...setup,receiver_ip:'192.168.4.20',candidates:[]};setupDelay=800;
    await open(); await input('receiver').fill('10.4.0.8'); await input('port').fill('5020'); await ready();
    assert.equal(await input('receiver').inputValue(),'10.4.0.8','late defaults must preserve typing');
    assert.equal(await input('port').inputValue(),'5020');
    assert.ok((await page.locator('#ruview-receiver-lookup').innerText()).includes('Router'));
    setupDelay=0;setupError=true;
    await open(); await ready();
    assert.ok((await page.locator('#ruview-receiver-lookup').innerText()).includes('nicht geladen'));
    assert.equal(await input('receiver').isEnabled(),true);
    assert.deepEqual(errors,[]);
    console.log('RuView wizard: 96 locale/theme/size cases; free node IDs, discovery failures and manual drafts; versioned modules and firmware; repeated boards and live catalog repair; cancellation, serial readback, fresh CSI checks and secret cleanup passed');
} finally {
    await browser.close();
    await new Promise(resolve=>server.close(resolve));
}
