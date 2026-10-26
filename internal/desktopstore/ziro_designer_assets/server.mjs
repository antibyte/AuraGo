// SPDX-License-Identifier: GPL-3.0-or-later
import { createServer } from 'node:http';
import { open, stat } from 'node:fs/promises';
import path from 'node:path';
const root = path.resolve(process.env.ZIRO_ROOT || '/app');
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml', '.json': 'application/json', '.wasm': 'application/wasm', '.png': 'image/png', '.woff2': 'font/woff2', '.glb': 'model/gltf-binary', '.webmanifest': 'application/manifest+json', '.gz': 'application/gzip' };
export const policy = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' blob: data:; worker-src 'self' blob:; frame-src 'none'; object-src 'none'; base-uri 'self'; form-action 'none'";
export function createAppServer() {
  return createServer(async (req, res) => {
    res.setHeader('Content-Security-Policy', policy);
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Referrer-Policy', 'no-referrer');
    if (!['GET', 'HEAD'].includes(req.method)) { res.writeHead(405); res.end(); return; }
    let file;
    try {
      const pathname = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
      file = path.resolve(root, '.' + pathname);
      if (file !== root && !file.startsWith(root + path.sep)) throw new Error('path');
      if (pathname === '/health') { res.writeHead(200); res.end('ok'); return; }
      try { if (!(await stat(file)).isFile()) throw new Error('missing'); }
      catch { if (path.extname(pathname)) { res.writeHead(404); res.end(); return; } file = path.join(root, 'index.html'); }
      const handle = await open(file, 'r');
      const info = await handle.stat();
      res.setHeader('Content-Type', types[path.extname(file)] || 'application/octet-stream');
      res.setHeader('Content-Length', info.size);
      res.setHeader('Cache-Control', /\/assets\//.test(file.replaceAll('\\', '/')) ? 'public, max-age=31536000, immutable' : 'no-cache');
      if (req.method === 'HEAD') { await handle.close(); res.end(); return; }
      const stream = handle.createReadStream();
      res.on('close', () => stream.destroy());
      stream.on('error', () => res.destroy()); stream.pipe(res);
    } catch { if (!res.headersSent) res.writeHead(400); res.end(); }
  });
}
if (process.argv[1] && path.resolve(process.argv[1]) === new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1')) {
  createAppServer().listen(Number(process.env.PORT || 8080), '0.0.0.0');
}
