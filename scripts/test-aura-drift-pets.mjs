/* Mascot placement, source preservation and curated-motion checks, without a browser. */
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadSim } from './aura-drift-sim-loader.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const AD = loadSim(['look', 'terrain', 'props', 'pet-decor']);
const manifest = JSON.parse(readFileSync(path.join(root, 'ui/3d/aura-drift/v1/manifest.json'), 'utf8'));
const tracks = AD.TRACKS.map(t => AD.buildTrack(t)).concat([AD.buildTrack(AD.dailyTrack(20261113))]);
const retired = ['mcp', 'snoopy', 'wall-e', 'webbi'];
for (const track of tracks) {
    const before = JSON.stringify({ samples: track.samples, seed: track.seed });
    const pets = AD.petDecorPlacements(track);
    assert.equal(JSON.stringify(pets), JSON.stringify(AD.petDecorPlacements(track)), 'Mascot placement must be deterministic');
    assert.equal(pets.length, 6, track.id + ' needs six clear mascot spots');
    const furniture = AD.furnitureProps(track);
    for (const pet of pets) {
        assert.ok(!retired.includes(pet.folder));
        assert.ok(AD.PET_DECOR.biomes[track.biomeId].includes(pet.folder));
        for (const key of ['x', 'y', 'z', 'yaw', 'r', 'h']) assert.ok(Number.isFinite(pet[key]), key);
        let nearest = Infinity, width = 0;
        for (const p of track.samples) { const d = Math.hypot(pet.x - p.x, pet.y - p.y); if (d < nearest) { nearest = d; width = p.halfWidth; } }
        assert.ok(nearest >= width + pet.r + 2.8, 'The complete mascot must remain outside the driving corridor');
        for (const prop of furniture) assert.ok(Math.hypot(pet.x - prop.x, pet.y - prop.y) >= pet.r + prop.r + 1.99, 'Mascot overlaps furniture');
    }
    const scatter = AD.scatterProps(track, track.biomeId, manifest.props[track.biomeId]);
    for (const pet of pets) for (const prop of scatter) assert.ok(Math.hypot(pet.x - prop.x, pet.y - prop.y) >= pet.r + prop.r + 1.99, 'Scatter invaded a reserved mascot spot: ' + track.id + '/' + prop.name);
    assert.equal(JSON.stringify({ samples: track.samples, seed: track.seed }), before, 'Decoration must not mutate track simulation');
}
let bytes = 0;
const sha = data => createHash('sha256').update(data).digest('hex');
for (const [folder, spec] of Object.entries(AD.PET_DECOR.pets)) {
    const raw = readFileSync(path.join(root, 'ui/3d/aura-drift/pets', folder + '.json'));
    const pack = JSON.parse(raw); bytes += raw.length;
    assert.equal(pack.model_sha256, sha(readFileSync(path.join(root, 'ui/3d/pets', folder, 'model.glb'))));
    assert.equal(pack.source_sha256, sha(readFileSync(path.join(root, 'ui/3d/pets', folder, 'motions.json'))));
    assert.deepEqual(pack.clips.map(c => c.id), Array.from(spec.clips));
    for (const item of pack.clips) for (const track of item.clip.tracks) {
        assert.ok(track.values.every(Number.isFinite));
        if (track.name.endsWith('.position')) for (let i = 3; i < track.values.length; i += 3) {
            assert.equal(track.values[i], track.values[0], 'Spectator must not walk into the road');
            assert.equal(track.values[i + 2], track.values[2], 'Spectator must not walk into the road');
        }
    }
}
assert.ok(bytes < 2 * 1024 * 1024, 'Curated motions must stay below 2 MiB');
console.log(`Aura Drift mascots: ${tracks.length} tracks, six clear spots each, nine active pets, three anchored clips each (${bytes} bytes).`);
