import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Round 11 art: the 26 sheets of the ember wyrm exist in the atlas, are lazy and carry the clips the code names.
const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const atlasPath = path.join(rootDir, 'ui/img/runenpfad/atlas.json');
const atlas = JSON.parse(fs.readFileSync(atlasPath, 'utf8'));
const artSrc = fs.readFileSync(path.join(rootDir, 'ui/js/desktop/apps/runenpfad/art.js'), 'utf8');

const CLIPS = {
    'td-boss-brandur': ['walk', 'breath_tell', 'breath', 'hurt', 'down'],
    'td-brandur-b': ['tail_tell', 'tail', 'roar', 'buffet', 'stomp_tell', 'stomp', 'drowsy', 'asleep'],
    'td-brandur-air': ['fly', 'takeoff', 'dive', 'land', 'glide'],
    'td-fx-breath': ['ignite', 'loop', 'fade'],
    'td-fx-hazards': ['stalactite', 'shards', 'ring', 'vent', 'flames'],
    'td-drakeling': ['move', 'tell', 'attack', 'hurt', 'death'],
    'td-emberling': ['move', 'tell', 'attack', 'hurt', 'death'],
    'fac-deep': ['f0', 'f1', 'f2', 'f3'],
};
const DEFAULTS = { 'td-boss-brandur': 'walk', 'td-brandur-b': 'drowsy', 'td-brandur-air': 'fly', 'td-fx-breath': 'loop', 'td-fx-hazards': 'flames' };
const IDS = Object.keys(CLIPS).concat(['td-folk-deep'],
    ['mira', 'nokk', 'bram', 'brandur'].map((n) => 'portrait-' + n),
    ['tex-mine', 'tex-glut'],
    ['mine', 'lair', 'home', 'house'].map((n) => 'tdp-' + n),
    [1, 2, 3, 4, 5, 6, 7].map((n) => 'cut-dragon-' + n));

let checks = 0;
function check(name, fn) { fn(); checks++; console.log('ok  ' + name); }

check('26 round-11 sheets exist and are lazy', () => {
    assert.equal(new Set(IDS).size, 26);
    const lazyRe = /\/\^\(([^)]*)\)-\/;/.exec(artSrc.slice(artSrc.indexOf('const LAZY')));
    assert.ok(lazyRe, 'art.js LAZY pattern');
    const lazy = new RegExp('^(' + lazyRe[1] + ')-');
    for (const id of IDS) {
        const s = atlas.sheets[id];
        assert.ok(s, id + ' in atlas');
        assert.equal(s.lazy, true, id + ' lazy flag');
        assert.ok(lazy.test(id), id + ' matches art.js LAZY');
        assert.ok(fs.existsSync(path.join(rootDir, 'ui/img/runenpfad', s.file)), id + ' file');
        assert.ok(s.cw > 0 && s.ch > 0 && s.cols > 0 && s.rows > 0, id + ' grid');
    }
});

check('the clip names are the real ones', () => {
    for (const [id, names] of Object.entries(CLIPS)) {
        const clips = atlas.sheets[id].clips;
        for (const n of names) assert.ok(clips[n], id + ' clip ' + n);
        if (DEFAULTS[id]) assert.equal(atlas.sheets[id].default, DEFAULTS[id], id + ' default clip');
    }
    assert.deepEqual(atlas.sheets['td-boss-brandur'].clips.walk.frames, [0, 1, 2, 3]);
    assert.deepEqual(atlas.sheets['td-brandur-air'].clips.fly.frames, [0, 1, 2, 3]);
    assert.deepEqual(atlas.sheets['td-fx-breath'].clips.loop.frames, [2, 3, 4, 5]);
    assert.deepEqual(atlas.sheets['td-fx-hazards'].clips.flames.frames, [4, 5, 6, 7]);
    for (const n of ['mira', 'nokk', 'bram', 'brandur']) assert.equal(atlas.sheets['portrait-' + n].cols, 3, n + ' has 3 moods');
    assert.equal(atlas.sheets['td-folk-deep'].rows, 4, 'four folk rows');
});

check('breath pivots left, dragon sizes as built', () => {
    assert.equal(atlas.sheets['td-fx-breath'].pivot, 'left');
    for (const id of ['td-boss-brandur', 'td-brandur-b', 'td-brandur-air']) assert.equal(atlas.sheets[id].cw, 444, id + ' cell width');
});

check('the atlas stays below 1 MiB', () => {
    assert.ok(fs.statSync(atlasPath).size < 1024 * 1024, 'atlas.json size ' + fs.statSync(atlasPath).size);
});

console.log('runenpfad art11: ' + checks + ' checks passed');
