/* Curate stationary spectator clips from the existing versioned Desktop pet assets. */
import assert from 'node:assert/strict';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadSim } from './aura-drift-sim-loader.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const checkOnly = process.argv.includes('--check');
assert.ok(process.argv.slice(2).every(x => x === '--check'), 'Unknown build argument');
const AD = loadSim(['pet-decor']);
const catalog = JSON.parse(await readFile(path.join(root, 'assets/desktop-pets/catalog.json'), 'utf8'));
const output = path.join(root, 'ui/3d/aura-drift/pets');
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const manifest = { version: 1, models: {}, biomes: AD.PET_DECOR.biomes };
if (!checkOnly) await mkdir(output, { recursive: true });
async function emit(name, body) {
    assert.ok(Buffer.byteLength(body) < 512 * 1024, name + ' exceeds the curated motion budget');
    if (checkOnly) assert.equal(await readFile(path.join(output, name), 'utf8'), body, name + ' is stale');
    else await writeFile(path.join(output, name), body, 'utf8');
}
for (const [folder, spec] of Object.entries(AD.PET_DECOR.pets)) {
    assert.ok(Object.hasOwn(catalog, folder), 'Mascots must be active Desktop pets');
    const model = await readFile(path.join(root, 'ui/3d/pets', folder, 'model.glb'));
    const source = await readFile(path.join(root, 'ui/3d/pets', folder, 'motions.json'));
    const pack = JSON.parse(source);
    assert.equal(pack.model_sha256, sha(model));
    const clips = spec.clips.map(id => {
        const item = structuredClone(pack.clips.find(clip => clip.id === id));
        assert.ok(item, 'Missing mascot clip: ' + folder + '/' + id);
        for (const track of item.clip.tracks) if (track.name.endsWith('.position')) {
            assert.equal(track.type, 'vector');
            // Spectators keep their horizontal anchor; vertical breathing and limb rotations remain.
            for (let i = 3; i < track.values.length; i += 3) { track.values[i] = track.values[0]; track.values[i + 2] = track.values[2]; }
        }
        return item;
    });
    const body = JSON.stringify({ version: 1, folder, model_sha256: pack.model_sha256, source_sha256: sha(source), license: pack.license, clips }) + '\n';
    await emit(folder + '.json', body);
    manifest.models[folder] = { pet_id: catalog[folder].id, model_sha256: pack.model_sha256, source_sha256: sha(source), motion_sha256: sha(body), bytes: Buffer.byteLength(body), license: pack.license, clips: spec.clips };
}
await emit('manifest.json', JSON.stringify(manifest, null, 2) + '\n');
console.log(`${checkOnly ? 'Checked' : 'Built'} ${Object.keys(manifest.models).length} Aura Drift mascot packs (three clips each; original models shared).`);
