// Run against the reviewed local image: ZIRO_TEST_URL=http://127.0.0.1:14174.
// AURAGO_PLAYWRIGHT_PATH can point to an already installed playwright directory.
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.AURAGO_PLAYWRIGHT_PATH || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const appURL = process.env.ZIRO_TEST_URL || 'http://127.0.0.1:14174';
const disk = new Map();
const directories = new Set(['Shared']);
const version = data => '"' + createHash('sha256').update(data).digest('hex') + '"';
const locale = JSON.parse(await readFile(path.join(root, 'ui/lang/desktop/de.json')));
let selected = '', readonly = false, uploadFailure = '', uploads = 0, initialAnswers = [];
const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, 'http://localhost');
    const json = (body, code = 200) => { res.writeHead(code, { 'Content-Type': 'application/json' }); res.end(JSON.stringify(body)); };
    if (url.pathname === '/fixture') {
      res.writeHead(200, { 'Content-Type': 'text/html' });
      res.end(`<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-common.css"><link rel="stylesheet" href="/css/desktop-app-software-store.css"></head><body style="margin:0"><div id="host" style="height:100vh"></div>
      <script src="/js/desktop/apps/ziro-files.js"></script><script src="/js/desktop/apps/ziro-designer.js"></script>
      <script>const tr=${JSON.stringify(locale)};window.answers=${JSON.stringify(initialAnswers)};window.cleanups=[];window.ctx={};window.ro=false;
      const deps={t:k=>tr[k]||k,esc:v=>String(v).replace(/[&<>"']/g,'_'),
      api:async(u,o)=>{const r=await fetch(u,o);const b=await r.json();if(!r.ok)throw Object.assign(new Error(b.error||'HTTP '+r.status),{status:r.status,body:b});return b;},
      modalDialog:async options=>{window.lastModal=options;return window.answers.shift()||false;},
      openFileDialog:async()=>({path:window.pick,canceled:!window.pick}),saveFileDialog:async options=>{const r=await fetch('/api/desktop/files?path='+encodeURIComponent(options.initialPath));if(!r.ok)throw new Error('Missing initial save folder');return {path:window.pick,canceled:!window.pick};},
      makeSandboxedFrame:url=>{const f=document.createElement('iframe');f.sandbox='allow-scripts allow-same-origin allow-downloads allow-pointer-lock';f.src=url;return f;},
      registerWindowCleanup:(_,fn)=>window.cleanups.push(fn),setBeforeClose:fn=>window.closeCheck=fn,
      isReadonly:()=>window.ro,draftKey:${JSON.stringify('browser-test-window-' + (url.searchParams.get('window') || '1'))},updateContext:c=>Object.assign(window.ctx,c),openProjectWindow:c=>{window.nextWindow=c;},openURL:'/open-url'};
      ZiroDesignerApp.render(document.querySelector('#host'),'test',{id:'store-ziro-designer'},${JSON.stringify(selected ? { path: selected } : {})},deps);</script></body></html>`);
      return;
    }
    if (url.pathname === '/open-url') return json({ url: appURL });
    if (url.pathname.startsWith('/js/') || url.pathname.startsWith('/css/')) {
      const source = await readFile(path.join(root, 'ui', url.pathname));
      res.writeHead(200, { 'Content-Type': url.pathname.startsWith('/css/') ? 'text/css' : 'text/javascript' }); res.end(source); return;
    }
    if (url.pathname === '/api/desktop/files') {
      const folder = url.searchParams.get('path');
      if (!directories.has(folder)) return json({ error: 'missing directory' }, 400);
      return json({ files: [...disk].filter(([key]) => key.startsWith(folder + '/')).map(([path, data]) => ({ path, size: data.length, type: 'file' })) });
    }
    if (url.pathname === '/api/desktop/download') {
      const data = disk.get(url.searchParams.get('path')); assert.ok(data);
      res.writeHead(200, { ETag: version(data), 'Content-Type': 'application/octet-stream' }); res.end(data); return;
    }
    if (readonly) return json({ code: 'desktop_readonly', error: 'readonly' }, 403);
    const chunks = []; for await (const chunk of req) chunks.push(chunk);
    const body = Buffer.concat(chunks);
    if (url.pathname === '/api/desktop/directory') { directories.add(JSON.parse(body).path); return json({ status: 'ok' }); }
    if (url.pathname === '/api/desktop/upload') {
      const request = new Request('http://localhost', { method: 'POST', headers: req.headers, body });
      const form = await request.formData(); const file = form.get('file');
      const target = form.get('path') + '/' + file.name;
      if (target === uploadFailure) return json({ error: 'simulated disk error' }, 500);
      const old = disk.get(target);
      if ((old && req.headers['if-match'] !== version(old)) || (!old && req.headers['if-none-match'] !== '*')) {
        return json({ code: 'file_conflict', conflict: { path: target, version: old ? version(old) : '' } }, 412);
      }
      const data = Buffer.from(await file.arrayBuffer()); disk.set(target, data); uploads++;
      return json({ path: target, version: version(data) });
    }
    json({ error: 'unknown route: ' + url.pathname }, 404);
  } catch (error) { res.writeHead(500); res.end(String(error)); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = 'http://127.0.0.1:' + server.address().port;
const browser = await chromium.launch({ headless: true, executablePath: process.env.AURAGO_BROWSER_PATH || undefined, args: ['--enable-webgl','--use-angle=swiftshader','--enable-unsafe-swiftshader'] });
const context = await browser.newContext({ viewport: { width: 1440, height: 960 }, serviceWorkers: 'block' });
const external = [], errors = [];
const until = async check => {
  const deadline = Date.now() + 15000;
  while (!check()) { if (Date.now() > deadline) throw new Error('Ziro save did not publish the expected bytes'); await new Promise(resolve => setTimeout(resolve, 25)); }
};
await context.route('**/*', route => {
  const url = new URL(route.request().url());
  if (url.protocol === 'http:' && [new URL(origin).host, new URL(appURL).host].includes(url.host)) return route.continue();
  external.push(url.href); return route.abort();
});
const page = await context.newPage();
page.on('pageerror', error => errors.push(error.message));
try {
  await page.goto(origin + '/fixture');
  await page.locator('[data-ziro="new"]').waitFor();
  await page.waitForFunction(() => !document.querySelector('[data-ziro="new"]').disabled, { timeout: 60000 });
  await page.evaluate(() => { window.answers.push('Board'); window.pick = 'Shared/Ziro Designer/Board.kicad_pro'; });
  await page.locator('[data-ziro="new"]').click();
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'Nicht in Shared gespeichert');
  await page.locator('[data-ziro="save"]').click();
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert', { timeout: 60000 });
  assert.ok(disk.has('Shared/Ziro Designer/Board/Board.kicad_pro'));
  assert.ok(disk.has('Shared/Ziro Designer/Board/Board.kicad_pcb'));
  const schematic = 'Shared/Ziro Designer/Board/Board.kicad_sch';
  assert.match(disk.get(schematic).toString(), /kicad_sch/);
  console.log('Real Ziro bridge: create and save native project passed.');
  const frame = page.frames().find(frame => frame.url().startsWith(appURL));
  await frame.locator('canvas').first().waitFor({ timeout: 60000 });
  const libraries = await frame.evaluate(async () => {
    const symbols = await (await fetch('/symbols/index.json')).json();
    const footprints = await (await fetch('/footprints/index.json')).json();
    const device = await (await fetch('/symbols/Device.kicad_sym')).text();
    return { resistor: symbols.some(lib => lib.name === 'Device' && lib.symbols.includes('R')),
      footprints: footprints.reduce((count, lib) => count + lib.footprints.length, 0), native: device.startsWith('(kicad_symbol_lib') };
  });
  assert.equal(libraries.resistor, true);
  assert.equal(libraries.native, true);
  assert.ok(libraries.footprints > 0, 'the offline footprint subset must be available');
  if (process.env.ZIRO_INSPECT === '1') {
    console.log('EDITOR_BUTTONS', await frame.locator('button').evaluateAll(nodes => nodes.map(node => ({ title: node.title, text: node.textContent?.slice(0, 70) })).filter(node => node.title || node.text).slice(0, 100)));
  }
  async function titleInEditor(frame, title) {
    await frame.page().bringToFront();
    for (let attempt = 0; attempt < 3; attempt++) {
      await frame.locator('button[title^="Page Settings"]:visible').click();
      try { await frame.getByRole('textbox', { name: 'Title', exact: true }).waitFor({ timeout: 1500 }); break; }
      catch (error) { if (attempt === 2) throw error; }
    }
    await frame.getByRole('textbox', { name: 'Title', exact: true }).fill(title);
    await frame.getByRole('button', { name: 'OK', exact: true }).click();
  }
  await titleInEditor(frame, 'Shared schematic acceptance');
  await page.keyboard.press('Control+s');
  await until(() => disk.get(schematic).toString().includes('Shared schematic acceptance'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert', { timeout: 60000 });
  assert.match(disk.get(schematic).toString(), /Shared schematic acceptance/, 'a real schematic edit must reach Shared');
  await frame.locator('button[title^="Switch to PCB Editor"]:visible').click();
  await titleInEditor(frame, 'Shared PCB acceptance');
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get('Shared/Ziro Designer/Board/Board.kicad_pcb').toString().includes('Shared PCB acceptance'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert');
  assert.match(disk.get('Shared/Ziro Designer/Board/Board.kicad_pcb').toString(), /Shared PCB acceptance/, 'a real PCB edit must reach Shared');
  const binaryPath = 'Shared/Ziro Designer/Board/sub/model.bin';
  directories.add('Shared/Ziro Designer/Board/sub'); disk.set(binaryPath, Buffer.from([255, 128, 1]));
  const childPath = 'Shared/Ziro Designer/Board/sub/Board.kicad_sch';
  disk.set(childPath, Buffer.from(disk.get(schematic).toString().replace(/\(uuid "[^"]+"\)/, '(uuid "5d092d6b-f60b-4e8d-821f-c672a148ea19")')));
  const grandchildPath = 'Shared/Ziro Designer/Board/Grandchild.kicad_sch';
  disk.set(grandchildPath, Buffer.from(disk.get(childPath)));
  disk.set(childPath, Buffer.from(disk.get(childPath).toString().replace(/\)\s*$/, `(sheet (at 100 50) (size 40 30)
    (stroke (width 0.1524) (type solid)) (fill (color 0 0 0 0))
    (uuid "54fc6cc9-0c27-4d37-a129-6a182a499719")
    (property "Sheetname" "Grandchild" (at 100 49 0) (effects (font (size 1.27 1.27))))
    (property "Sheetfile" "../Grandchild.kicad_sch" (at 100 81 0) (effects (font (size 1.27 1.27)))))
  )`)));
  disk.set(schematic, Buffer.from(disk.get(schematic).toString().replace(/\)\s*$/, `(sheet (at 100 50) (size 40 30)
    (stroke (width 0.1524) (type solid)) (fill (color 0 0 0 0))
    (uuid "35348ec7-7fc1-4594-9a67-b0a3d85c924b")
    (property "Sheetname" "Child" (at 100 49 0) (effects (font (size 1.27 1.27))))
    (property "Sheetfile" "sub/Board.kicad_sch" (at 100 81 0) (effects (font (size 1.27 1.27)))))
  )`)));
  const localLibrary = 'Shared/Ziro Designer/Board/sub/LocalParts.kicad_sym';
  disk.set(localLibrary, Buffer.from(`(kicad_symbol_lib (version 20241209) (generator kicad_symbol_editor)
    (symbol "Probe" (in_bom yes) (on_board yes)
      (property "Reference" "U" (at 0 2.54 0) (effects (font (size 1.27 1.27))))
      (property "Value" "Probe" (at 0 -2.54 0) (effects (font (size 1.27 1.27))))
      (symbol "Probe_0_1" (rectangle (start -1 -1) (end 1 1) (stroke (width 0) (type default)) (fill (type background))))))`));
  disk.set('Shared/Ziro Designer/Board/sym-lib-table', Buffer.from('(sym_lib_table (version 7) (lib (name "LocalParts") (type "KiCad") (uri "${KIPRJMOD}/sub/LocalParts.kicad_sym") (options "") (descr "")))'));
  const footprintLibrary = 'Shared/Ziro Designer/Board/LocalFootprints.pretty/ProbeFP.kicad_mod';
  directories.add('Shared/Ziro Designer/Board/LocalFootprints.pretty');
  disk.set(footprintLibrary, Buffer.from('(footprint "ProbeFP" (version 20240108) (generator "pcbnew") (layer "F.Cu") (attr smd) (pad "1" smd rect (at 0 0) (size 1 1) (layers "F.Cu" "F.Paste" "F.Mask")))'));
  disk.set('Shared/Ziro Designer/Board/fp-lib-table', Buffer.from('(fp_lib_table (version 7) (lib (name "LocalFootprints") (type "KiCad") (uri "${KIPRJMOD}/LocalFootprints.pretty") (options "") (descr "")))'));
  selected = 'Shared/Ziro Designer/Board/Board.kicad_pro';
  if (process.env.ZIRO_TEST_CONTAINER) {
    await promisify(execFile)('docker', ['restart', process.env.ZIRO_TEST_CONTAINER]);
    const deadline = Date.now() + 30000;
    while (!await fetch(appURL + '/health').then(response => response.ok).catch(() => false)) {
      if (Date.now() > deadline) throw new Error('Ziro did not restart');
      await new Promise(resolve => setTimeout(resolve, 100));
    }
  }
  await page.reload();
  await page.waitForFunction(() => document.querySelector('[data-ziro-path]')?.textContent === 'Shared/Ziro Designer/Board', { timeout: 60000 });
  const hierarchyFrame = page.frames().find(frame => frame.url().startsWith(appURL));
  await hierarchyFrame.locator('.ze-hier-row[title="sub/Board.kicad_sch"]').click({ timeout: 60000 });
  await titleInEditor(hierarchyFrame, 'Saved hierarchical child');
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get(childPath).toString().includes('Saved hierarchical child'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert', { timeout: 60000 });
  await hierarchyFrame.locator('.ze-hier-row[title="Grandchild.kicad_sch"]').click();
  await titleInEditor(hierarchyFrame, 'Saved relative grandchild');
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get(grandchildPath).toString().includes('Saved relative grandchild'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert');
  await hierarchyFrame.locator('.ze-hier-row[title="Board.kicad_sch"]').click();
  await hierarchyFrame.locator('button[title^="Place Symbols"]:visible').click();
  await hierarchyFrame.locator('.ze-symbol-chooser input[placeholder="Filter"]').fill('Probe');
  await hierarchyFrame.locator('.ze-symbol-chooser [title="LocalParts:Probe"]').first().waitFor();
  await hierarchyFrame.locator('.ze-symbol-chooser').getByRole('button', { name: 'Cancel', exact: true }).click();
  await hierarchyFrame.locator('button[title^="Symbol Editor"]:visible').click();
  await hierarchyFrame.locator('.ze-leftdock input[placeholder="Filter"]:visible').fill('Probe');
  await hierarchyFrame.locator('.ze-leftdock [title="LocalParts:Probe"]:visible').dblclick();
  await hierarchyFrame.locator('button[title^="Edit symbol properties"]:visible').click();
  await hierarchyFrame.getByRole('textbox', { name: /Keywords/ }).fill('Shared symbol acceptance');
  await hierarchyFrame.getByRole('button', { name: 'OK', exact: true }).click();
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get(localLibrary).toString().includes('Shared symbol acceptance'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert');
  await hierarchyFrame.locator('[title="Back to project manager"]:visible').click();
  await hierarchyFrame.getByRole('button', { name: 'Footprint Editor', exact: true }).click();
  await hierarchyFrame.locator('input[placeholder="Filter"]:visible').fill('ProbeFP');
  await hierarchyFrame.locator('[title="LocalFootprints:ProbeFP"]:visible').dblclick();
  await hierarchyFrame.locator('button[title^="Edit footprint properties"]:visible').click();
  await hierarchyFrame.getByRole('textbox', { name: /Description/ }).fill('Shared footprint acceptance');
  await hierarchyFrame.getByRole('button', { name: 'OK', exact: true }).click();
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get(footprintLibrary).toString().includes('Shared footprint acceptance'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert');
  await hierarchyFrame.locator('[title="Back to project manager"]:visible').click();
  await hierarchyFrame.getByRole('button', { name: 'Schematic Editor', exact: true }).click();
  assert.deepEqual([...disk.get(binaryPath)], [255,128,1]);
  assert.match(disk.get(childPath).toString(), /kicad_sch/);
  assert.match(disk.get(localLibrary).toString(), /symbol "Probe"/);
  const other = await context.newPage();
  await other.goto(origin + '/fixture?window=2');
  await other.waitForFunction(() => document.querySelector('[data-ziro-path]')?.textContent === 'Shared/Ziro Designer/Board');
  const otherFrame = other.frames().find(frame => frame.url().startsWith(appURL));
  await titleInEditor(otherFrame, 'Saved by the other window');
  await other.locator('[data-ziro="save"]').click();
  await until(() => disk.get(schematic).toString().includes('Saved by the other window'));
  const currentFrame = page.frames().find(frame => frame.url().startsWith(appURL));
  await titleInEditor(currentFrame, 'Keep my conflicting edit');
  await page.evaluate(() => { window.answers.push('copy'); window.pick = 'Shared/Ziro Designer/BoardCopy.kicad_pro'; });
  await page.locator('[data-ziro="save"]').click();
  const copyFolder = 'Shared/Ziro Designer/BoardCopy';
  await until(() => disk.get(copyFolder + '/Board.kicad_sch')?.toString().includes('Keep my conflicting edit'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert');
  assert.match(disk.get(schematic).toString(), /Saved by the other window/);
  assert.deepEqual([...disk.get(copyFolder + '/sub/model.bin')], [255,128,1]);
  assert.deepEqual(disk.get(copyFolder + '/sub/LocalParts.kicad_sym'), disk.get(localLibrary));
  assert.deepEqual(disk.get(copyFolder + '/LocalFootprints.pretty/ProbeFP.kicad_mod'), disk.get(footprintLibrary));
  assert.match(disk.get(copyFolder + '/sub/Board.kicad_sch').toString(), /Saved hierarchical child/);
  assert.match(disk.get(copyFolder + '/Grandchild.kicad_sch').toString(), /Saved relative grandchild/);
  await titleInEditor(currentFrame, 'Recover after a failed save');
  await currentFrame.locator('button[title^="Switch to PCB Editor"]:visible').click();
  await titleInEditor(currentFrame, 'Recover the PCB too');
  uploadFailure = copyFolder + '/Board.kicad_pcb';
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get(copyFolder + '/Board.kicad_sch').toString().includes('Recover after a failed save'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'simulated disk error');
  assert.doesNotMatch(disk.get(uploadFailure).toString(), /Recover the PCB too/);
  assert.equal(await page.evaluate(() => window.closeCheck()), false, 'cancelled close must preserve a failed-save draft');
  selected = copyFolder + '/Board.kicad_pro'; initialAnswers = ['recover'];
  await page.reload();
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]')?.textContent === 'Nicht in Shared gespeichert');
  uploadFailure = '';
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get(copyFolder + '/Board.kicad_pcb').toString().includes('Recover the PCB too'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert');
  const restoredFrame = page.frames().find(frame => frame.url().startsWith(appURL));
  await restoredFrame.locator('button[title^="Page Settings"]:visible').waitFor();
  const switchToSchematic = restoredFrame.locator('button[title^="Switch to Schematic Editor"]:visible');
  if (await switchToSchematic.count()) await switchToSchematic.click();
  await titleInEditor(restoredFrame, 'Readonly draft');
  readonly = true;
  await page.evaluate(() => { window.ro=true; document.dispatchEvent(new CustomEvent('aurago:desktop-policy')); });
  assert.equal(await page.locator('[data-ziro="save"]').isDisabled(), true);
  assert.equal(await page.locator('[data-ziro="open"]').isDisabled(), false);
  await page.evaluate(() => { window.ro=false; document.dispatchEvent(new CustomEvent('aurago:desktop-policy')); });
  await page.locator('[data-ziro="save"]').click();
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'readonly');
  assert.doesNotMatch(disk.get(copyFolder + '/Board.kicad_sch').toString(), /Readonly draft/);
  readonly = false;
  await page.locator('[data-ziro="save"]').click();
  await until(() => disk.get(copyFolder + '/Board.kicad_sch').toString().includes('Readonly draft'));
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'In Shared gespeichert');
  const sharedCount = uploads;
  await page.evaluate(() => window.answers.push('AnotherBoard'));
  await page.locator('[data-ziro="new"]').click();
  await page.waitForFunction(() => window.nextWindow?.ziro_new === 'AnotherBoard');
  assert.equal(await page.locator('[data-ziro-path]').textContent(), copyFolder);
  await page.evaluate(path => { window.pick = path; }, selected);
  await page.locator('[data-ziro="open"]').click();
  await page.waitForFunction(() => !!window.nextWindow?.path);
  assert.equal(uploads, sharedCount, 'New/Open in another window never publishes the current project');
  await restoredFrame.evaluate(origin => window.parent.postMessage({ ziro: 1, type: 'save-request', id: 9000 }, origin), origin);
  await page.evaluate(() => window.postMessage({ ziro: 1, type: 'ready', challenge: 'forged' }, location.origin));
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  assert.equal(uploads, sharedCount, 'window messages cannot bypass the private document channel');
  assert.equal(external.length, 0, 'external runtime requests: ' + external.join(', '));
  assert.deepEqual(errors, [], 'browser script errors');
  await page.screenshot({ path: process.env.ZIRO_SCREENSHOT || path.join(root, '../ziro-browser.png'), fullPage: true });
  await page.setViewportSize({ width: 820, height: 700 });
  await page.locator('iframe').evaluate((frame, url) => { frame.src = url; }, appURL + '/');
  await page.waitForFunction(() => document.querySelector('[data-ziro-status]').textContent === 'Die Verbindung zum Editor wurde unterbrochen. Öffne das Fenster erneut.');
  assert.equal(await page.locator('[data-ziro="save"]').isDisabled(), true);
  await page.evaluate(() => window.cleanups.forEach(fn => fn()));
  assert.equal(await page.locator('iframe').count(), 0);
  await other.close();
  console.log('Ziro browser: four editors, hierarchical sheets, custom libraries, two-window conflict copy, partial failure/recovery, readonly, binary preservation, separate windows, disposal and zero external requests passed; uploads=' + uploads);
} catch (error) {
  console.error('Ziro browser failure:', await page.locator('[data-ziro-status]').textContent(), errors, external);
  for (const [index, item] of context.pages().entries()) {
    console.error('Page ' + index, (await item.locator('iframe').contentFrame().locator('body').innerText().catch(() => '')).slice(-1800));
    await item.screenshot({ path: path.join(root, '../ziro-browser-failure-' + index + '.png'), fullPage: true });
  }
  await page.screenshot({ path: path.join(root, '../ziro-browser-failure.png'), fullPage: true });
  throw error;
} finally { await browser.close(); await new Promise(resolve => server.close(resolve)); }
