import { readFileSync, writeFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import assert from 'node:assert/strict';

const write = process.argv.includes('--write');
const root = new URL('../', import.meta.url), read = path => JSON.parse(readFileSync(new URL(path, root), 'utf8'));
const atlas = read('ui/img/galaxa/atlas.json'), provenance = read('ui/img/galaxa/provenance.json');
provenance.production = 'Original image_gen PNG outputs, unchanged pixels. Alpha-derived frame rectangles and source anchors live in atlas.json. Gameplay uses nearest-neighbor sprite crops; optional planet sheets draw directly with smoothed scaling and frame blending.';
const motifs = read('assets/galaxa-expansion/catalog.json').motifs;
assert.equal(motifs.length, 50); assert.equal(new Set(motifs.map(m => m.kind + '.' + m.id)).size, 50);
const metadata = JSON.parse(execFileSync('go', ['run', './scripts/galaxa-atlas-metadata.go'], { cwd: new URL('.', root), maxBuffer: 8 * 1024 * 1024, encoding: 'utf8' }));
for (const [id, sheet] of Object.entries(atlas.sheets)) if (sheet.file.startsWith('exp-')) delete atlas.sheets[id];
for (const key of Object.keys(atlas.animations)) if (/^(asteroid|alien|weapon|planet|effect)\./.test(key)) delete atlas.animations[key];
provenance.files = provenance.files.filter(f => !f.file.startsWith('exp-'));
atlas.expansion = { id: 'deluxe-50', motifs: [] };
for (const [motifIndex, m] of motifs.entries()) {
    const ids = m.kind === 'planet' ? ['a', 'b', 'c'].map(part => `exp-${m.kind}-${m.id}-${part}`) : [`exp-${m.kind}-${m.id}`];
    const frames = [], anchors = [], sheets = []; let sourceSize = 0;
    for (const id of ids) {
        const file = id + '.png', meta = metadata[file]; assert.ok(meta, 'Missing source: ' + file);
        const source = read(`assets/galaxa-expansion/sources/${id.slice(4)}.json`);
        const bytes = readFileSync(new URL('ui/img/galaxa/' + file, root));
        atlas.sheets[id] = { file, width: meta.width, height: meta.height, ...(m.kind === 'planet' ? { optional: 'planet' } : {}) };
        provenance.files.push({ file, source: source.source, sha256: createHash('sha256').update(bytes).digest('hex'), bytes: bytes.length, prompt: source.prompt, ...(source.reference ? { reference: source.reference } : {}) });
        frames.push(...meta.frames); anchors.push(...meta.anchors); sheets.push(...meta.frames.map(() => id)); sourceSize = Math.max(sourceSize, meta.sourceSize);
        if (meta.edgePixels.some(n => n > 12)) console.warn('Review frame edge:', file, meta.edgePixels);
        if (m.kind === 'planet') for (const f of meta.frames) assert.ok(f[2] >= 512 && f[3] >= 512, 'Planet frame below 512px: ' + file);
    }
    const keys = [];
    const add = (key, start, count, size, ms, loop = true) => {
        atlas.animations[key] = { sheet: ids[0], frames: frames.slice(start, start + count), size, origin: size, pivot: [0.5, 0.5], ms, loop,
            sourceSize, sourceAnchors: anchors.slice(start, start + count), ...(ids.length > 1 ? { frameSheets: sheets.slice(start, start + count) } : {}) };
        keys.push(key); return atlas.animations[key];
    };
    const key = m.kind + '.' + m.id;
    if (m.kind === 'alien') {
        const size = atlas.animations[m.role + '.idle'].size;
        add(key + '.idle', 0, 8, size, 105); add(key + '.attack', 8, 6, size, 100); add(key + '.damage', 14, 2, size, 75);
    } else if (m.kind === 'weapon') {
        const a = add(key, 0, 8, 26, 80);
        if (m.id.includes('mine')) add(key + '.arming', 8, 4, 26, 112, false);
        atlas.animations['pickup.' + m.powerup] = { ...a, size: 32, origin: 32, ms: 130 };
        if (m.powerup === 'mine_layer') atlas.animations['pickup.mega_mine_layer'] = { ...atlas.animations['pickup.mine_layer'] };
        if (m.powerup === 'rocket_launcher') atlas.animations['pickup.mega_rocket'] = { ...atlas.animations['pickup.rocket_launcher'] };
    } else if (m.kind === 'planet') add(key, 0, 12, 280, 2000 + (motifIndex - 30) * 200);
    else if (m.kind === 'asteroid') add(key, 0, 16, 48, 100).blendFrames = true;
    else add(key, 0, 12, 64, 55, ['target-lock', 'rocket-exhaust'].includes(m.id));
    atlas.expansion.motifs.push({ number: motifIndex + 1, id: m.id, kind: m.kind, animations: keys, usage: m.role || m.powerup || m.biome || m.effect || 'combat-effect' });
}
for (const [path, value] of [['ui/img/galaxa/atlas.json', atlas], ['ui/img/galaxa/provenance.json', provenance]]) {
    const text = JSON.stringify(value, null, 2) + '\n';
    if (write) writeFileSync(new URL(path, root), text);
    else assert.equal(readFileSync(new URL(path, root), 'utf8').replace(/\r\n/g, '\n'), text, path + ' is stale; run --write');
}
console.log(`Galaxa: ${motifs.length} motifs, ${provenance.files.filter(f => f.file.startsWith('exp-')).length} original sheets, atlas/provenance ${write ? 'written' : 'verified'}.`);
