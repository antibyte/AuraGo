// Serves ui/ plus /harness (ui/testdata/aura-drift-harness.html) for real-screen Aura Drift play.
// Usage: node scripts/aura-drift-dev-server.mjs [port]   (default 8791)
import http from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const uiRoot = path.join(root, 'ui');
const port = Number(process.argv[2] || process.env.PORT || 8791);
const types = {
    '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.mjs': 'text/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8', '.json': 'application/json; charset=utf-8', '.svg': 'image/svg+xml',
    '.png': 'image/png', '.jpg': 'image/jpeg', '.webp': 'image/webp', '.glb': 'model/gltf-binary',
    '.ogg': 'audio/ogg', '.mp3': 'audio/mpeg', '.wav': 'audio/wav', '.woff2': 'font/woff2', '.txt': 'text/plain; charset=utf-8'
};

const server = http.createServer(async (req, res) => {
    try {
        const url = new URL(req.url, 'http://localhost');
        let rel = decodeURIComponent(url.pathname);
        if (rel === '/harness' || rel === '/harness/') rel = '/testdata/aura-drift-harness.html';
        const file = path.resolve(uiRoot, '.' + rel);
        if (file !== uiRoot && !file.startsWith(uiRoot + path.sep)) {
            res.writeHead(403).end('forbidden');
            return;
        }
        const info = await stat(file);
        if (!info.isFile()) throw new Error('not a file');
        const body = await readFile(file);
        res.writeHead(200, {
            'Content-Type': types[path.extname(file).toLowerCase()] || 'application/octet-stream',
            'Cache-Control': 'no-store'
        });
        res.end(body);
    } catch (_) {
        res.writeHead(404, { 'Cache-Control': 'no-store' }).end('not found');
    }
});
server.listen(port, '127.0.0.1', () => {
    console.log(`Aura Drift dev server: http://127.0.0.1:${port}/harness?track=alpenpass&autostart=1`);
});
