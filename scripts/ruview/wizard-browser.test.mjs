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
const server = createServer(async (req, res) => {
    try {
        const url = new URL(req.url, 'http://localhost');
        if (url.pathname.startsWith('/api/desktop/integrations/ruview/')) {
            res.setHeader('Content-Type', 'application/json');
            const route = url.pathname.split('/ruview/')[1];
            if (req.method === 'POST') posts++;
            if (route === 'board-setup') {
                const snapshot = structuredClone(setup);
                if (setupDelay) await new Promise(resolve => setTimeout(resolve, setupDelay));
                res.statusCode = setupError ? 503 : 200;
                return res.end(JSON.stringify(setupError ? { error:'unavailable' } : snapshot));
            }
            if (route === 'firmware') return res.end(await readFile('internal/ruview/artifacts.json'));
            if (route === 'status' && statusError) {res.statusCode=statusError;return res.end(JSON.stringify({error:'fixture status unavailable'}));}
            if (route === 'proxy/api/v1/nodes') return res.end(JSON.stringify({nodes:[{node_id:7,csi_status:'active',csi_last_seen_ms:42,csi_sequence:3}]}));
            return res.end(JSON.stringify({enabled:true,readonly:false,mode:'docker',source:'esp32',fresh,signal:'CSI',data_state:'unchecked'}));
        }
        if (url.pathname === '/') {
            const lang = locales.includes(url.searchParams.get('lang')) ? url.searchParams.get('lang') : 'de';
            const dict = JSON.parse(await readFile(`ui/lang/ruview/${lang}.json`));
            const theme = url.searchParams.get('theme') === 'standard' ? 'standard' : 'fruity';
            const mode = url.searchParams.get('mode') === 'dark' ? 'dark' : 'light';
            res.setHeader('Content-Type','text/html; charset=utf-8');
            return res.end(`<!doctype html><html data-theme="dark"><head><meta charset="utf-8"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/ruview.css"></head><body class="desktop-body" data-theme="${theme}" data-fruity-mode="${mode}"><main id="app"></main><script src="/js/desktop/apps/ruview.js"></script><script>const dict=${JSON.stringify(dict)};RuViewApp.render(document.getElementById('app'),'fixture',{t:k=>dict[k]||k,esc:s=>String(s).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';')});</script></body></html>`);
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
        window.serialFixture = {cancel:false, fail:false, mismatch:false, requests:0, configured:null};
        Object.defineProperty(navigator, 'serial', {value:{requestPort:async () => {
            const state = window.serialFixture;
            state.requests++;
            if (state.cancel) throw new DOMException('No port selected', 'NotFoundError');
            let receiver;
            const port = {
                getInfo:() => ({usbVendorId:0x303a,usbProductId:0x1001}),
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
    await open(); await ready();
    const input = name => page.locator(`dialog [name="${name}"]`);
    const action = name => page.locator(`dialog [data-action="${name}"]`);
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
    await page.evaluate(async () => {
        serialFixture.cancel=false;
        const { ESPLoader, Transport } = await import('/js/vendor/ruview-esptool.js');
        const catalog=await (await fetch('/api/desktop/integrations/ruview/firmware')).json();
        const profile=catalog.firmware.find(p=>p.id==='esp32s3-16mb-cdc');
        const images=new Map(await Promise.all(profile.parts.map(async p=>[p.offset,new Uint8Array(await (await fetch(`/js/vendor/ruview-firmware/${catalog.revision}/${profile.id}/${p.name}`)).arrayBuffer())])));
        serialFixture.flashWrites=0;
        ESPLoader.prototype.main=async function(){this.chip={CHIP_NAME:'ESP32-S3'};};
        ESPLoader.prototype.readFlashId=async()=>24<<16;
        ESPLoader.prototype.readFlash=async address=>images.get(address);
        ESPLoader.prototype.writeFlash=async()=>{serialFixture.flashWrites++;};
        ESPLoader.prototype.after=async()=>{};
        Transport.prototype.disconnect=async()=>{};
    });
    await input('profile').selectOption('esp32s3-8mb-cdc');
    await action('flash').click(); await ready();
    assert.match(await page.locator('dialog section').first().innerText(),/Erkannt: 16 MB Flash.*Profil: 8 MB/);
    assert.equal(await page.evaluate(()=>serialFixture.flashWrites),0);
    assert.equal(await input('password').inputValue(),'fixture-password');
    await input('profile').selectOption('esp32s3-16mb-cdc');
    await action('flash').click(); await ready();
    assert.equal(await page.evaluate(()=>serialFixture.flashWrites),1);
    assert.ok((await page.locator('dialog section').first().innerText()).includes('Firmware geschrieben und geprüft'));
    assert.equal(await input('password').inputValue(),'fixture-password','flashing must preserve WLAN drafts for step 2');
    await page.evaluate(() => {serialFixture.cancel=false;serialFixture.fail=true;});
    await action('configure').click();
    await page.waitForFunction(() => [...document.querySelectorAll('dialog [role="status"]')].some(p=>p.textContent.includes('Fixture port busy')));
    assert.equal(await input('password').inputValue(),'fixture-password');
    await page.evaluate(() => serialFixture.fail=false);
    await action('configure').click();
    await page.waitForFunction(() => !document.querySelector('dialog [data-action="readback"]').disabled);
    assert.equal(await input('password').inputValue(),'');
    assert.equal(await action('verify').isDisabled(),true);
    await page.evaluate(() => serialFixture.mismatch=true);
    await action('readback').click(); await ready();
    assert.equal(await action('verify').isDisabled(),true,'a different board cannot confirm the settings');
    await page.evaluate(() => serialFixture.mismatch=false);
    await action('readback').click();
    await page.waitForFunction(() => !document.querySelector('dialog [data-action="verify"]').disabled);
    await input('ssid').fill('next-draft'); await input('password').fill('next-password');
    fresh=false; await action('verify').click(); await ready();
    assert.ok((await page.locator('dialog section').last().innerText()).includes('Noch keine frischen CSI-Daten'));
    assert.equal(await input('password').inputValue(),'next-password','sensor tests must not clear WLAN drafts');
    fresh=true; await action('verify').click(); await ready();
    assert.ok((await page.locator('dialog section').last().innerText()).includes('Frische Sensordaten empfangen'));
    assert.equal(posts,2);
    assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
    await page.evaluate(()=>{window.closedInputs=[...document.querySelectorAll('dialog input')];});
    await page.locator('dialog button[value="close"]').click();
    await page.locator('dialog').waitFor({state:'detached'});
    assert.equal(await page.evaluate(()=>closedInputs.filter(i=>['ssid','password'].includes(i.name)).every(i=>i.value==='')),true);

    await open(); await ready();
    await input('ssid').fill('reconnect-draft'); await input('password').fill('fixture-password');
    statusError=503;
    await page.waitForFunction(()=>document.querySelector('dialog > [role="status"]')?.textContent.includes('Verbindung unterbrochen'));
    assert.equal(await input('password').inputValue(),'fixture-password','a transient status failure must preserve the open draft');
    await action('configure').click(); await ready();
    assert.equal(await input('password').inputValue(),'fixture-password','failed authorization refresh must never provision or discard the draft');
    statusError=403;
    await page.locator('dialog').waitFor({state:'detached'});
    statusError=0;

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
    console.log('RuView wizard: 96 locale/theme/size cases; native checkboxes; numbered steps; saved and detected receivers; cancellation, serial readback, fresh CSI checks and secret cleanup passed');
} finally {
    await browser.close();
    await new Promise(resolve=>server.close(resolve));
}
