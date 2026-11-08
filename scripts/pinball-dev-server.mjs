// Serves ui/ plus /node_modules/three/* (only) and /harness (ui/testdata/pinball-harness.html) for AuraPinball dev work.
// Usage: node scripts/pinball-dev-server.mjs   (port 8792, override with PORT or argv[2])
import http from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const uiRoot = path.join(root, 'ui');
const threeRoot = path.join(root, 'node_modules', 'three');
const port = Number(process.argv[2] || process.env.PORT || 8792);
const types = {
    '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.mjs': 'text/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8', '.json': 'application/json; charset=utf-8', '.svg': 'image/svg+xml',
    '.png': 'image/png', '.jpg': 'image/jpeg', '.webp': 'image/webp', '.glb': 'model/gltf-binary',
    '.ogg': 'audio/ogg', '.mp3': 'audio/mpeg', '.wav': 'audio/wav', '.woff2': 'font/woff2',
    '.txt': 'text/plain; charset=utf-8', '.wasm': 'application/wasm'
};

function resolveFile(pathname) {
    let rel = decodeURIComponent(pathname);
    if (rel.includes('\0')) return null;
    if (rel === '/harness' || rel === '/harness/') rel = '/testdata/pinball-harness.html';
    let base = uiRoot;
    if (rel.startsWith('/node_modules/three/')) { base = threeRoot; rel = rel.slice('/node_modules/three'.length); }
    else if (rel.startsWith('/node_modules')) return null;
    const file = path.resolve(base, '.' + rel);
    if (file !== base && !file.startsWith(base + path.sep)) return null;
    return file;
}

const server = http.createServer(async (req, res) => {
    const headers = { 'Cache-Control': 'no-store' };
    try {
        const file = resolveFile(new URL(req.url, 'http://localhost').pathname);
        if (!file) { res.writeHead(403, headers).end('forbidden'); return; }
        const info = await stat(file);
        if (!info.isFile()) throw new Error('not a file');
        const body = await readFile(file);
        res.writeHead(200, { ...headers, 'Content-Type': types[path.extname(file).toLowerCase()] || 'application/octet-stream' });
        res.end(body);
    } catch (_) {
        res.writeHead(404, headers).end('not found');
    }
});
server.listen(port, '127.0.0.1', () => {
    console.log(`AuraPinball dev server: http://127.0.0.1:${port}/harness?table=neon&mode=2d`);
});
