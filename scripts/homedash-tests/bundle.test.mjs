import assert from 'node:assert/strict'; import { spawnSync } from 'node:child_process'; import fs from 'node:fs'; import path from 'node:path'; import { read, root } from './harness.mjs';
export const PARTS = ['tokens', 'bridge', 'base', 'primitives', 'family-glass', 'family-tactile', 'family-minimal', 'family-instrument', 'widgets', 'page'].map(p => `ui/css/homedash-${p}.css`);
export const tests = [
  ['the bundle entry lists the ten parts in order and the parts are LF without BOM', () => {
    const m = /out: 'ui\/css\/homedash\.bundle\.css',\s*parts: \[([^\]]*)\]/.exec(read('scripts/build-ui-bundles.js')); assert.ok(m, 'bundle entry missing');
    assert.deepEqual([...m[1].matchAll(/'([^']+)'/g)].map(x => x[1]), PARTS);
    for (const p of PARTS) { const t = read(p); assert.ok(!t.startsWith(String.fromCodePoint(0xfeff)), p + ' BOM'); assert.ok(!t.includes('\r'), p + ' CRLF'); } }],
  ['the generated bundle is current and carries the bridge and the font faces', () => {
    const r = spawnSync(process.execPath, ['scripts/build-ui-bundles.js', '--check'], { cwd: root, encoding: 'utf8' });
    assert.equal(r.status, 0, r.stdout + r.stderr); assert.match(r.stdout, /Checked ui\/css\/homedash\.bundle\.css \(10 parts\)/);
    const b = read('ui/css/homedash.bundle.css'); for (const s of ['--vd-mat-card-color', '"HomeDash Barlow Condensed"', '"HomeDash Share Tech Mono"', '--hd-n-ink']) assert.ok(b.includes(s), s);
    for (const f of ['BarlowCondensed-SemiBold.ttf', 'ShareTechMono-Regular.ttf']) assert.ok(fs.existsSync(path.join(root, 'ui/fonts', f)), f); }],
  ['base CSS carries the backdrops, rotation groups and the easing fallback', () => { const base = read('ui/css/homedash-base.css'), tok = read('ui/css/homedash-tokens.css');
    for (const s of ['[data-hd-backdrop="theme"]', '[data-hd-backdrop="solid"]', '[data-hd-backdrop="gradient"]', '.hd-dial-handle', '.hd-gauge-needle', '.hd-gauge-cap', '.hd-dial-current', 'transform-origin: 100px 100px']) assert.ok(base.includes(s), s);
    assert.ok(tok.includes('.hd-root[data-hd-ease="fallback"]'), 'ease fallback'); }],
  ['page CSS places widgets from the render variables and declares no token', () => { const page = read('ui/css/homedash-page.css');
    for (const s of ['.hd-page > .hd-widget', 'var(--hd-x', 'var(--hd-colw', '.hd-tab[aria-selected="true"]', '.hd-page-empty', '.hd-widget.hd-unsupported']) assert.ok(page.includes(s), s);
    assert.ok(!/^\s*--hd-[\w-]+\s*:/m.test(page), 'the page part reads tokens only'); }]
];
