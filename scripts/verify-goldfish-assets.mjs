import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, resolve, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { inflateSync } from 'node:zlib';
import vm from 'node:vm';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const art = resolve(root, 'ui/img/goldfish-adventure');
const json = file => JSON.parse(readFileSync(resolve(root, file), 'utf8'));
const manifest = json('ui/img/goldfish-adventure/manifest.json');
const context = { window: {}, TextEncoder };
vm.createContext(context);
for (const name of ['content', 'content-act1', 'content-act2', 'content-act3', 'content-act4'])
    vm.runInContext(readFileSync(resolve(root, 'ui/js/desktop/apps/goldfish-adventure/' + name + '.js'), 'utf8'), context);
const content = context.window.GoldfishContent;
const images = new Map();

function png(file) {
    if (images.has(file)) return images.get(file);
    const bytes = readFileSync(file);
    assert.equal(bytes.subarray(0, 8).toString('hex'), '89504e470d0a1a0a', file);
    const width = bytes.readUInt32BE(16), height = bytes.readUInt32BE(20);
    assert.equal(bytes[24], 8, 'Only authored 8-bit PNGs are expected');
    assert.equal(bytes[28], 0, 'Interlaced atlas export is not supported');
    const color = bytes[25], channels = { 2: 3, 6: 4 }[color];
    assert.ok(channels, 'PNG must be RGB or RGBA: ' + file);
    const parts = [];
    for (let at = 8; at < bytes.length;) {
        const size = bytes.readUInt32BE(at), type = bytes.toString('ascii', at + 4, at + 8);
        if (type === 'IDAT') parts.push(bytes.subarray(at + 8, at + 8 + size));
        at += size + 12;
    }
    const raw = inflateSync(Buffer.concat(parts)), stride = width * channels;
    assert.equal(raw.length, (stride + 1) * height);
    let previous = Buffer.alloc(stride), cursor = 0, transparent = 0, opaque = 0;
    const paeth = (a, b, c) => {
        const p = a + b - c, pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
        return pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
    };
    for (let y = 0; y < height; y++) {
        const filter = raw[cursor++], row = Buffer.from(raw.subarray(cursor, cursor + stride)); cursor += stride;
        assert.ok(filter <= 4);
        for (let x = 0; x < stride; x++) {
            const left = x >= channels ? row[x - channels] : 0, up = previous[x], prior = x >= channels ? previous[x - channels] : 0;
            row[x] = (row[x] + (filter === 1 ? left : filter === 2 ? up : filter === 3 ? Math.floor((left + up) / 2) : filter === 4 ? paeth(left, up, prior) : 0)) & 255;
        }
        if (channels === 4) for (let x = 3; x < stride; x += 4) {
            if (row[x] === 0) transparent++;
            if (row[x] === 255) opaque++;
        }
        previous = row;
    }
    const result = { width, height, bytes: bytes.length, alpha: channels === 4, transparent, opaque,
        sha256: createHash('sha256').update(bytes).digest('hex') };
    images.set(file, result);
    return result;
}

function local(url) {
    assert.ok(typeof url === 'string' && url.startsWith('/img/goldfish-adventure/') && !/[?#\\]/.test(url) && !url.includes('..'), url);
    const file = resolve(root, 'ui', url.slice(1));
    assert.ok(!relative(art, file).startsWith('..'));
    assert.ok(statSync(file).isFile(), url);
    return file;
}

assert.equal(Object.keys(manifest.rooms).length, 16);
for (const room of content.rooms) {
    const stage = manifest.rooms[room.id];
    assert.equal(stage.background, room.background);
    const files = new Set([stage.background, ...(stage.layers || []).map(layer => layer.src)]);
    for (const actor of [{ id: 'r0st' }, ...(stage.actors || [])]) {
        const sprite = manifest.sprites[actor.sprite || actor.id];
        assert.ok(sprite, room.id + ' actor ' + actor.id);
        files.add(sprite.src);
        for (const frame of sprite.frames) if (frame.src) files.add(frame.src);
    }
    const decoded = [...files].reduce((sum, file) => {
        const image = png(local(file)); return sum + image.width * image.height * 4;
    }, 0);
    assert.ok(decoded <= 192 * 1024 * 1024, room.id + ' decoded image budget');
    if (stage.audio) local(stage.audio);
    assert.equal(stage.audio, '/img/goldfish-adventure/audio/song-' + room.id.toLowerCase() + '.ogg', room.id + ' plays its own song');
}
assert.ok(manifest.audioVolume >= 0.05 && manifest.audioVolume <= 0.25, 'Room music must stay quiet (audioVolume 0.05-0.25)');
let clips = 0, frames = 0;
for (const [id, sprite] of Object.entries(manifest.sprites)) {
    for (const frame of sprite.frames) {
        const image = png(local(frame.src || sprite.src));
        assert.ok(image.alpha && image.transparent > image.width * image.height * .08, id + ' needs genuine cutout alpha');
        const [x, y, w, h] = frame.rect;
        assert.ok([x, y, w, h].every(Number.isInteger) && x >= 0 && y >= 0 && w > 0 && h > 0 && x + w <= image.width && y + h <= image.height, id + ' frame bounds');
        assert.ok(frame.pivot.every(value => Number.isFinite(value) && value >= 0 && value <= 1));
        frames++;
    }
    for (const clip of Object.values(sprite.clips)) {
        assert.ok(clip.frames.length > 0 && clip.fps > 0);
        for (const frame of clip.frames) assert.ok(Number.isInteger(frame) && frame >= 0 && frame < sprite.frames.length, id + ' clip index');
        clips++;
    }
}
for (const item of content.items) assert.ok(png(local(item.icon)).alpha, item.id + ' inventory alpha');
for (const entry of json('assets/goldfish-adventure/background-manifest.json').rooms) {
    const master = resolve(root, entry.master), runtime = resolve(root, entry.runtime);
    assert.equal(png(master).sha256, entry.sha256);
    assert.equal(png(runtime).sha256, entry.sha256);
    assert.equal(png(runtime).width, entry.width);
    assert.equal(png(runtime).height, entry.height);
}
for (const entry of json('assets/goldfish-adventure/character-manifest.json').sheets) {
    const source = resolve(root, 'assets/goldfish-adventure', entry.source);
    assert.equal(png(source).sha256, entry.sha256);
    if (entry.runtime) assert.equal(png(resolve(art, entry.runtime)).sha256, entry.sha256);
}
for (const [actionID, beats] of Object.entries(manifest.moments)) {
    const action = content.actions.find(record => record.id === actionID);
    assert.ok(action, 'Unknown cinematic action ' + actionID);
    assert.ok(beats.length >= 2 && beats.length <= 24);
    const actors = new Map([{ id: 'r0st' }, ...manifest.rooms[action.room].actors].map(actor => [actor.id, actor]));
    assert.ok(beats.reduce((total, beat) => total + beat.duration, 0) <= 60000);
    for (const beat of beats) {
        for (const [id, pose] of Object.entries(beat.actors || {})) {
            const actor = actors.get(id);
            assert.ok(actor, actionID + ' stages an absent actor ' + id);
            assert.ok(manifest.sprites[actor.sprite || id].clips[pose.clip], actionID + ' missing clip ' + pose.clip);
        }
        if (beat.sfx) local(beat.sfx);
    }
}
assert.ok(Object.keys(manifest.moments).length >= 12, 'Bespoke story moments missing');

for (const entry of json('assets/goldfish-adventure/audio-manifest.json').assets) {
    const bytes = readFileSync(local('/img/goldfish-adventure/' + entry.path));
    assert.equal(createHash('sha256').update(bytes).digest('hex'), entry.sha256, entry.id + ' audio hash');
}
for (const take of json('assets/goldfish-adventure/audio-takes.json').takes) {
    const bytes = readFileSync(resolve(root, 'assets/goldfish-adventure/masters/audio/samples', take.name));
    assert.equal(createHash('sha256').update(bytes).digest('hex'), take.sha256, take.name + ' instrument hash');
}
assert.equal(readFileSync(resolve(root, 'assets/goldfish-adventure/audio-notices.txt'), 'utf8'),
    readFileSync(resolve(art, 'audio/audio-notices.txt'), 'utf8'), 'Audio notices must ship unchanged');

const locales = ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
const placeholders = value => [...value.matchAll(/\{[a-zA-Z_]+\}/g)].map(match => match[0]).sort();
for (const section of ['goldfish', 'goldfish-interface']) {
    const source = json('ui/lang/' + section + '/de.json'), keys = Object.keys(source).sort();
    for (const locale of locales) {
        const filename = 'ui/lang/' + section + '/' + locale + '.json';
        const bytes = readFileSync(resolve(root, filename));
        assert.notEqual(bytes.subarray(0, 3).toString('hex'), 'efbbbf', filename + ' BOM');
        assert.ok(!bytes.includes(13), filename + ' CRLF');
        const values = JSON.parse(bytes.toString('utf8'));
        assert.deepEqual(Object.keys(values).sort(), keys, filename + ' incomplete keys');
        for (const key of keys) {
            assert.ok(typeof values[key] === 'string' && values[key].trim() && values[key].length <= 4096, key);
            assert.deepEqual(placeholders(values[key]), placeholders(source[key]), filename + ' ' + key);
        }
    }
}
function size(directory) {
    return readdirSync(directory, { withFileTypes: true }).reduce((sum, item) => {
        const path = resolve(directory, item.name);
        return sum + (item.isDirectory() ? size(path) : statSync(path).size);
    }, 0);
}
const runtimeBytes = size(art) + size(resolve(root, 'ui/lang/goldfish')) + size(resolve(root, 'ui/lang/goldfish-interface')) + size(resolve(root, 'ui/js/desktop/apps/goldfish-adventure')) + statSync(resolve(root, 'ui/css/desktop-app-goldfish-adventure.css')).size;
assert.ok(runtimeBytes <= 180 * 1024 * 1024, 'Full game payload exceeds authored budget');
console.log(JSON.stringify({ rooms: 16, spriteDefinitions: Object.keys(manifest.sprites).length, frameReferences: frames, clipDefinitions: clips,
    storyMoments: Object.keys(manifest.moments).length, locales: locales.length, runtimeBytes, verifiedImages: images.size }, null, 2));
