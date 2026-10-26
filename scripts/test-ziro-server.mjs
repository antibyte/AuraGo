import assert from 'node:assert/strict';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
const directory = await mkdtemp(path.join(tmpdir(), 'aurago-ziro-static-'));
assert.ok(path.resolve(directory).startsWith(path.resolve(tmpdir()) + path.sep));
process.env.ZIRO_ROOT = directory;
const { createAppServer } = await import('../internal/desktopstore/ziro_designer_assets/server.mjs');
await writeFile(path.join(directory, 'index.html'), '<!doctype html>offline app');
await writeFile(path.join(directory, 'worker.wasm'), Uint8Array.of(0,97,115,109));
const server = createAppServer();
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = 'http://127.0.0.1:' + server.address().port;
try {
  const response = await fetch(origin + '/project/route');
  assert.equal(response.status, 200);
  assert.equal(await response.text(), '<!doctype html>offline app');
  assert.match(response.headers.get('content-security-policy'), /connect-src 'self' blob: data:/);
  assert.equal((await fetch(origin + '/health')).status, 200);
  assert.equal((await fetch(origin + '/worker.wasm')).headers.get('content-type'), 'application/wasm');
  const head = await fetch(origin + '/worker.wasm', { method: 'HEAD' });
  assert.equal(head.headers.get('content-length'), '4');
  assert.equal((await head.arrayBuffer()).byteLength, 0);
  assert.equal((await fetch(origin + '/missing.js')).status, 404);
  assert.equal((await fetch(origin + '/index.html', { method: 'POST' })).status, 405);
  console.log('Ziro static server: SPA routing, health, WASM/HEAD, missing files, read-only methods and CSP passed.');
} finally { await new Promise(resolve => server.close(resolve)); await rm(directory, { recursive: true, force: true }); }
