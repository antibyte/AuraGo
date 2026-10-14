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
  for (const name of ['nasscad-manifold-wasm.js','web-ifc-api-iife.js','nasscad-ifc-wasm.js']) assert(requests.includes(name), `${name} did not load`);
  assert.deepEqual(denied, [], 'dynamic assets lost the workspace access token');
  assert.deepEqual(errors, [], 'NASSCAD raised browser errors');
  console.log('NASSCAD browser: startup, cube, Manifold, IFC and authenticated dynamic assets passed');
} finally {
  await browser.close();
  await new Promise(resolve => server.close(resolve));
}
