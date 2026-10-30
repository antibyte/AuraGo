import assert from 'node:assert/strict';
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import {createRequire} from 'node:module';

const require = createRequire(import.meta.url);
const {chromium} = require(process.env.AURAGO_PLAYWRIGHT || 'playwright');
const root = path.resolve('internal/desktop/bundled_apps/nasscad');
const manifest = JSON.parse(fs.readFileSync(path.join(root, 'provenance.json')));
// Exercise the monolithic workspace entry used by the Desktop seeder.
const html = fs.readFileSync(path.join(root, 'index.html'), 'utf8').replace(/<script\b[^>]*\bsrc=["']([^"']+)["'][^>]*>\s*<\/script>/gi,
  (_, name) => `<script>\n${fs.readFileSync(path.join(root, name), 'utf8')}\n</script>`);
const requests = [], denied = [], errors = [];
const token = 'nasscad-browser-fixture';
const server = http.createServer((req,res) => {
  const url = new URL(req.url, 'http://localhost');
  if (url.pathname === '/favicon.ico') { res.writeHead(204).end(); return; }
  if (url.searchParams.get('embed_token') !== token) { denied.push(url.pathname); res.writeHead(403).end(); return; }
  const name = url.pathname.slice(1);
  if (!(name in manifest.files)) { res.writeHead(404).end(); return; }
  requests.push(name);
  res.setHeader('Content-Type', name.endsWith('.js') ? 'text/javascript' : name.endsWith('.wasm') ? 'application/wasm' : name.endsWith('.html') ? 'text/html' : 'application/octet-stream');
  res.end(name === 'index.html' ? html : fs.readFileSync(path.join(root, name)));
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const browser = await chromium.launch({channel:'chrome',headless:true});
try {
  const page = await browser.newPage({viewport:{width:1365,height:900}});
  page.on('pageerror', error => errors.push(error.message));
  await page.route('http://127.0.0.1:8765/**', route => route.fulfill({status:503,body:'fixture: no native engine'}));
  await page.goto(`http://127.0.0.1:${server.address().port}/index.html?embed_token=${token}`);
  await page.waitForFunction(() => typeof addPrimitive === 'function' && typeof scene !== 'undefined');
  assert.equal(await page.evaluate(() => {addPrimitive('cube'); return objs.length;}), 1);
  await page.evaluate(() => _wasmCsgLoadScript());
  assert.equal(await page.evaluate(() => !!globalThis._NASSCAD_MANIFOLD_WASM), true);
  await page.evaluate(() => _ifcGetApi());
  const downloads=[];page.on('download',item=>downloads.push(item.suggestedFilename()));
  const emailExports=await page.evaluate(async()=>{
    const rows=[];
    for(const format of ['project','stl','stl-ascii','3mf','glb','glb-draco','obj','ply','ifc']){
      const files=await AuraNasscadEmail.createFiles(format);
      if(files.length!==1)throw Error('missing '+format+' mail artifact');
      const file=files[0],bytes=new Uint8Array(await file.arrayBuffer());
      rows.push({format,name:file.name,type:file.type,size:file.size,head:Array.from(bytes.slice(0,32))});
    }
    return rows;
  });
  for(const format of ['step','svg','dxf']){
    await page.evaluate(format=>{window.emailTask=AuraNasscadEmail.createFiles(format);},format);
    const modal=page.locator(format==='step'?'#step-export-modal':'#sxdlg');
    await modal.waitFor({state:'visible'});
    if(format!=='step')await page.locator('#sxval').fill('5');
    const row=await page.evaluate(async format=>{
      if(format==='step')await doStepExport();else await _doSectionExp(format);
      const files=await window.emailTask;
      if(files.length!==1)throw Error('missing '+format+' mail artifact');
      const file=files[0],bytes=new Uint8Array(await file.arrayBuffer());
      return {format,name:file.name,type:file.type,size:file.size,head:Array.from(bytes.slice(0,32))};
    },format);
    emailExports.push(row);
  }
  await page.evaluate(()=>{window.emailTask=AuraNasscadEmail.createFiles('step');});
  await page.locator('#step-export-modal').waitFor({state:'visible'});
  await page.evaluate(()=>cancelStepExportModal());
  assert.equal(await page.evaluate(()=>window.emailTask),false,'cancelled export created files');
  assert.equal(emailExports.length,12);
  for(const row of emailExports)assert(row.size>0&&row.name&&row.type,'incomplete '+row.format+' artifact');
  const header=format=>Buffer.from(emailExports.find(row=>row.format===format).head);
  assert.equal(header('3mf').subarray(0,2).toString(),'PK');
  assert.equal(header('glb').subarray(0,4).toString(),'glTF');
  assert.equal(header('glb-draco').subarray(0,4).toString(),'glTF');
  assert.equal(header('ply').subarray(0,3).toString(),'ply');
  assert(header('ifc').toString().startsWith('ISO-10303-21;'));
  assert(header('step').toString().startsWith('ISO-10303-21;'));
  assert(header('svg').toString().startsWith('<?xml'));
  assert(header('dxf').toString().includes('SECTION'));
  assert(emailExports.find(row=>row.format==='svg').name.includes('Y5mm'));
  assert.deepEqual(downloads,[],'preparing email must never start a browser download');
  for (const name of ['nasscad-manifold-wasm.js','web-ifc-api-iife.js','nasscad-ifc-wasm.js']) assert(requests.includes(name), `${name} did not load`);
  assert.deepEqual(denied, [], 'dynamic assets lost the workspace access token');
  assert.deepEqual(errors, [], 'NASSCAD raised browser errors');
  console.log('NASSCAD browser: startup, cube, 12 native email formats, settings, cancellation and authenticated assets passed');
} finally {
  await browser.close();
  await new Promise(resolve => server.close(resolve));
}
