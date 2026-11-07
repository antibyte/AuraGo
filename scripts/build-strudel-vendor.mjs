import fs from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { zipSync } from 'fflate';
import { build } from 'esbuild';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const out = path.join(root, 'ui/js/vendor/strudel');
const check = process.argv.includes('--check');
const packages = new Map(), sources = {}, notices = [];
async function collect(name, from = path.join(root, 'package.json')) {
    const require = createRequire(from);
    let dir;
    try { dir = path.dirname(require.resolve(name + '/package.json')); }
    catch { dir = path.dirname(require.resolve(name)); while (JSON.parse(await fs.readFile(path.join(dir, 'package.json')).catch(() => '{}')).name !== name) dir = path.dirname(dir); }
    const pkg = JSON.parse(await fs.readFile(path.join(dir, 'package.json')));
    const key = name + '@' + pkg.version;
    if (packages.has(key)) return;
    packages.set(key, pkg.license || 'See package notices');
    const relativeName = entry => path.relative(dir, path.join(entry.parentPath, entry.name)).replaceAll('\\', '/');
    for (const entry of (await fs.readdir(dir, { recursive: true, withFileTypes: true })).sort((a, b) => relativeName(a) < relativeName(b) ? -1 : relativeName(a) > relativeName(b) ? 1 : 0)) {
        if (!entry.isFile()) continue;
        const file = path.join(entry.parentPath, entry.name), relative = path.relative(dir, file).replaceAll('\\', '/');
        if (relative.startsWith('node_modules/')) continue;
        const bytes = await fs.readFile(file);
        sources[key + '/' + relative] = [bytes, { mtime: new Date('2000-01-01T00:00:00Z') }];
        if (/^(license|licence|copying|notice)/i.test(entry.name)) notices.push('## ' + key + ' — ' + relative + '\n\n' + bytes.toString('utf8'));
    }
    for (const dep of Object.keys(pkg.dependencies || {}).sort()) await collect(dep, path.join(dir, 'package.json'));
}
await collect('@strudel/web');
const pkg = path.join(root, 'node_modules/@strudel/web');
const bundle = await build({ absWorkingDir: root, entryPoints: [path.join(pkg, 'web.mjs')], bundle: true, format: 'iife', globalName: 'strudel', write: false, minify: true, legalComments: 'inline', target: 'es2022', define: { 'process.env.NODE_ENV': '"production"' }, logLevel: 'silent' });
const assets = { 'strudel.js': Buffer.from(bundle.outputFiles[0].contents) };
const instructions = '# Strudel 1.3.0 sources\n\nNpm distributions and their complete shipped sources, including transitive dependencies. Versions and licenses are in manifest.json. Reproduce the browser runtime with npm ci in the AuraGo source checkout, then node scripts/build-strudel-vendor.mjs. Upstream repository: https://codeberg.org/uzu/strudel. AuraGo rebundles web.mjs with esbuild; no upstream source is changed. Shared-worker clock synchronization is disabled in the integration.\n';
for (const name of ['scripts/build-strudel-vendor.mjs', 'ui/js/desktop/apps/synth-studio-strudel.js', 'ui/js/desktop/apps/synth-studio-strudel-player.js', 'internal/server/synth_studio_strudel.go']) sources['aurago/' + name] = [Buffer.from((await fs.readFile(path.join(root, name), 'utf8')).replaceAll('\r\n', '\n')), { mtime: new Date('2000-01-01T00:00:00Z') }];
sources['README.md'] = [Buffer.from(instructions), { mtime: new Date('2000-01-01T00:00:00Z') }];
assets['sources.zip'] = Buffer.from(zipSync(sources, { level: 6 }));
assets['LICENSES.md'] = Buffer.from('# Strudel — AGPL-3.0-or-later\n\nCopyright Strudel contributors, including Alex McLean and Felix Roos.\nUpstream source is unchanged, locally rebundled with esbuild. AuraGo integration: 2026-11-07, AGPL-3.0-or-later.\nComplete shipped package and integration sources: [sources.zip](sources.zip).\nIntegration source: https://github.com/antibyte/AuraGo\n\n' + notices.join('\n\n'));
assets['manifest.json'] = Buffer.from(JSON.stringify({ package: '@strudel/web', version: '1.3.0', packages: Object.fromEntries(packages), files: Object.fromEntries(Object.entries(assets).map(([name, data]) => [name, { bytes: data.length, sha256: createHash('sha256').update(data).digest('hex') }])) }, null, 2) + '\n');
for (const [name, data] of Object.entries(assets)) {
    const target = path.join(out, name);
    if (check) {
        if (!data.equals(await fs.readFile(target))) throw new Error('Stale Strudel asset: ' + name);
    } else { await fs.mkdir(path.dirname(target), { recursive: true }); await fs.writeFile(target, data); }
}
console.log('Strudel 1.3.0: runtime, source archive and ' + packages.size + ' package notices ' + (check ? 'verified' : 'vendored'));
