import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile, readdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const dir = path.join(root, 'ui/img/infra');
const manifest = JSON.parse(await readFile(path.join(dir, 'manifest.json'), 'utf8'));
const provenance = JSON.parse(await readFile(path.join(dir, 'provenance.json'), 'utf8'));
assert.equal(manifest.version, 1);
assert.equal(manifest.icons.length, 66);
assert.equal(new Set(manifest.icons.map(icon => icon.id)).size, 66);
assert.deepEqual(['device', 'application', 'structure', 'app'].map(family => manifest.icons.filter(icon => icon.family === family).length), [32, 24, 8, 2]);
assert.equal(provenance.assets.length, 66);
let total = 0;
for (const icon of manifest.icons) {
    assert.match(icon.id, /^[a-z0-9-]+$/);
    assert.equal(icon.path, `/img/infra/${icon.id}.webp`);
    const data = await readFile(path.join(dir, `${icon.id}.webp`));
    assert.equal(createHash('sha256').update(data).digest('hex'), icon.sha256, icon.id);
    assert.equal(data.toString('ascii', 0, 4), 'RIFF', icon.id);
    assert.equal(data.toString('ascii', 8, 12), 'WEBP', icon.id);
    assert.equal(data.toString('ascii', 12, 16), 'VP8X', icon.id);
    assert.ok(data[20] & 0x10, `${icon.id} must retain generated alpha`);
    assert.equal(data.readUIntLE(24, 3) + 1, icon.width, icon.id);
    assert.equal(data.readUIntLE(27, 3) + 1, icon.height, icon.id);
    assert.ok(icon.width <= 768 && icon.height <= 768, icon.id);
    const origin = provenance.assets.find(asset => asset.id === icon.id);
    assert.ok(origin?.transparent_background, icon.id);
    assert.match(origin.source_sha256, /^[a-f0-9]{64}$/);
    assert.ok(icon.id === 'rack-server' ? origin.style_brief : origin.prompt, icon.id);
    assert.equal(origin.style_reference, icon.id === 'rack-server' ? null : 'rack-server');
    total += data.length;
}
assert.ok(total < 2_500_000, `Infra raster budget exceeded: ${total}`);
assert.equal((await readdir(dir)).filter(name => name.endsWith('.webp')).length, 66);
for (const locale of ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']) {
    const labels = JSON.parse(await readFile(path.join(root, `ui/lang/desktop/${locale}.json`), 'utf8'));
    for (const icon of manifest.icons.filter(icon => icon.family !== 'app')) {
        assert.ok(typeof labels[icon.label_key] === 'string' && labels[icon.label_key].trim(), `${locale}: ${icon.label_key}`);
    }
}
console.log(`Infra assets: 64 localized motifs + 2 illustrations, alpha/hashes/dimensions valid, ${total} bytes`);
