// Runenpfad round 11 audio checks: every event of the deeps has a voice and a TRIM key, Brandur's seven moves, the new mob voices,
// the mine and glut room tones, and the in_mine / in_lair themes. Loads only the audio add-on; the event scan reads source text,
// so files that do not exist yet are skipped.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const dir = path.join(root, 'ui/js/desktop/apps/runenpfad');
const sandbox = { console, Math, JSON, Object, Array, Float32Array, Promise, Map, Set, Uint8Array };
sandbox.globalThis = sandbox;
sandbox.fetch = () => Promise.reject(new Error('offline test'));
vm.createContext(sandbox);
const load = (rel) => vm.runInContext(fs.readFileSync(path.join(dir, rel), 'utf8'), sandbox, { filename: rel });
['biomes.js', 'fx.js', 'physics.js', 'behaviors.js', 'bosses.js', 'audio-dsp.js', 'sfx.js', 'sfx-rooms.js', 'sfx-deeps.js', 'audio-mix.js', 'audio.js',
    'music-voices.js', 'music.js', 'music-scores.js', 'duel-music.js', 'place-music.js', 'story-music.js', 'deeps-music.js'].forEach(load);
const Mix = sandbox.RunenpfadAudioMix;
const Audio = sandbox.RunenpfadAudio;
const Sfx = sandbox.RunenpfadSfx;
const Music = sandbox.RunenpfadMusic;
const PlaceMusic = sandbox.RunenpfadPlaceMusic;
const DeepsMusic = sandbox.RunenpfadDeepsMusic;
assert.ok(Sfx.ROOM_TOOLS && Sfx.MOB_VOICES && DeepsMusic, 'the deeps audio modules loaded');

// ---- strict fake Web Audio ----
let nodeCount = 0;
function param(name) {
    const check = (v) => assert.ok(Number.isFinite(v), `${name}: non-finite value ${v}`);
    return {
        value: 0,
        setValueAtTime(v, t) { check(v); check(t); },
        linearRampToValueAtTime(v, t) { check(v); check(t); },
        exponentialRampToValueAtTime(v, t) { check(v); check(t); assert.ok(v > 0, `${name}: exponential ramp to ${v}`); },
        setTargetAtTime(v, t, k) { check(v); check(t); check(k); },
        cancelScheduledValues() {}
    };
}
function node(kind) {
    nodeCount++;
    const target = { kind, connect: (n) => n, disconnect() {}, start() {}, stop() {}, getChannelData: () => new Float32Array(64) };
    return new Proxy(target, {
        get(t, k) { if (!(k in t)) t[k] = param(kind + '.' + String(k)); return t[k]; },
        set(t, k, v) { t[k] = v; return true; }
    });
}
class FakeContext {
    get currentTime() { return sandbox.__t || 0; }
    constructor() { this.state = 'running'; this.sampleRate = 8000; this.destination = node('dest'); }
    resume() { return Promise.resolve(); }
    suspend() { return Promise.resolve(); }
    close() { return Promise.resolve(); }
    decodeAudioData() { return Promise.reject(new Error('no decode')); }
    createBuffer() { return node('buffer'); }
}
['createGain', 'createOscillator', 'createBiquadFilter', 'createBufferSource', 'createDynamicsCompressor', 'createConvolver', 'createStereoPanner', 'createWaveShaper', 'createDelay', 'createPeriodicWave']
    .forEach((m) => { FakeContext.prototype[m] = function () { return node(m); }; });
sandbox.AudioContext = FakeContext;
const manifest = { assets: Audio.REQUIRED_SOUNDS.map((id) => ({ id, gain: 0.8, files: [{ file: 'sounds/' + id + '.wav' }] })) };
const cueOf = (type) => Audio.CUE_MAP[type] || Sfx.CUES[type];
const hasTrim = (key) => Object.prototype.hasOwnProperty.call(Mix.TRIM, key);

// ---- 1. every event of the deeps has a cue and a TRIM key (and a TRIM key per variant) ----
const EVENTS = ['in_tablet', 'dr_bell', 'dr_outcome', 'dr_blessing', 'dr_wake', 'dr_scale', 'dr_drowsy', 'dr_impact', 'craft_cook', 'craft_strike', 'craft_done',
    'in_boss_tell', 'in_boss_attack', 'in_boss_phase', 'in_boss_die', 'in_mob_tell', 'in_mob_attack', 'in_mob_die', 'in_hit', 'in_brazier'];
function checkCue(type) {
    const cue = cueOf(type);
    assert.ok(cue, 'cue missing for ' + type);
    assert.ok(cue.synth || (cue.by && Object.keys(cue.by.map).length), type + ' has a synthesized voice');
    assert.ok(hasTrim(type), 'TRIM key missing for ' + type);
    if (cue.by && cue.by.key !== 'pattern' && cue.by.key !== 'kind' && cue.by.key !== 'target' && cue.by.key !== 'hero') {
        Object.keys(cue.by.map).filter((k) => k !== '*').forEach((k) => assert.ok(hasTrim(type + ':' + k), 'TRIM key missing for ' + type + ':' + k));
    }
}
EVENTS.forEach(checkCue);
// Variant keys for the new kind tables.
['stalactite', 'rain', 'land', 'vent'].forEach((k) => assert.ok(Sfx.CUES.dr_impact.by.map[k] && hasTrim('dr_impact:' + k), 'dr_impact voice and TRIM for ' + k));
['slain', 'sealed'].forEach((k) => assert.ok(Sfx.CUES.dr_outcome.by.map[k]));
assert.notEqual(Sfx.CUES.dr_bell.by.map.true.synth, Sfx.CUES.dr_bell.by.map.false.synth, 'the mended bell tolls, the cracked one rattles');
assert.notEqual(Sfx.CUES.craft_strike.by.map.good.synth, Sfx.CUES.craft_strike.by.map.bad.synth, 'a good strike rings, a bad one clanks');
assert.notEqual(Sfx.CUES.craft_done.by.map.true.synth, Sfx.CUES.craft_done.by.map.false.synth, 'a done craft and a failed one differ');
assert.notEqual(Sfx.CUES.dr_drowsy.by.map.true.synth, Sfx.CUES.dr_drowsy.by.map.false.synth, 'falling asleep and waking up differ');
// Every pushEvent type of the new modules has a cue and a TRIM key (files that do not exist yet are skipped).
const pushed = {};
['brandur.js', 'deep-mobs.js', 'ember.js', 'crafts.js', 'deeps-rooms.js', 'chronicle.js'].forEach((rel) => {
    const file = path.join(dir, rel);
    if (!fs.existsSync(file)) return;
    for (const m of fs.readFileSync(file, 'utf8').matchAll(/pushEvent\x28sim, '([a-z_]+)'/g)) {
        pushed[m[1]] = rel;
        assert.ok(cueOf(m[1]), rel + ' pushes ' + m[1] + ' without an audio cue');
        assert.ok(hasTrim(m[1]), rel + ' pushes ' + m[1] + ' without a TRIM key');
    }
});

// ---- 2. Brandur: seven moves, each with its own tell and attack voice and TRIM keys ----
const MOVES = ['breath', 'tail', 'stomp', 'takeoff', 'buffet', 'roar', 'vents'];
assert.equal(Sfx.DEEP_MOVES.join(), MOVES.join());
assert.ok(Sfx.ROOM_GUARDIANS.brandur && Object.keys(Sfx.ROOM_GUARDIANS.brandur.moves).length === 7, 'Brandur is in the guardian table');
['in_boss_tell', 'in_boss_attack'].forEach((type) => {
    const map = Sfx.CUES[type].by.map;
    assert.ok(map.brandur, type + ' default for brandur');
    MOVES.forEach((m) => {
        assert.ok(map['brandur_' + m], type + ' voice for brandur_' + m);
        assert.ok(Audio.hasSynth(map['brandur_' + m].synth), type + ' recipe for brandur_' + m);
        assert.ok(hasTrim(type + ':brandur_' + m), 'TRIM key for ' + type + ':brandur_' + m);
    });
    assert.ok(hasTrim(type + ':brandur'), 'TRIM key for ' + type + ':brandur');
    assert.equal(new Set(MOVES.map((m) => map['brandur_' + m].synth)).size, 7, type + ' gives every move its own recipe');
});
MOVES.forEach((m) => assert.notEqual(Sfx.CUES.in_boss_tell.by.map['brandur_' + m].synth, Sfx.CUES.in_boss_attack.by.map['brandur_' + m].synth, m + ' tell differs from its attack'));
['in_boss_phase', 'in_boss_die'].forEach((type) => {
    const v = Sfx.CUES[type].by.map.brandur;
    assert.ok(v && Audio.hasSynth(v.synth), type + ' voice for brandur');
    assert.ok(hasTrim(type + ':brandur'), 'TRIM key for ' + type + ':brandur');
    assert.notEqual(v.synth, Sfx.CUES[type].by.map.brood.synth, type + ' brandur has his own voice');
});
// The seven moves resolve to their own recipe when played, and an unknown move falls back to Brandur's default (not the counter).
{
    const played = [];
    const originals = {};
    Object.keys(Sfx.SYNTHS).filter((k) => /^dp_/.test(k)).forEach((name) => {
        originals[name] = Sfx.SYNTHS[name];
        Sfx.SYNTHS[name] = function (ctx, buf, dest, time, options) { played.push(name); return originals[name](ctx, buf, dest, time, options); };
    });
    sandbox.__t = 20;
    const rc = Audio.createAudioController({ manifest });
    rc.unlock();
    MOVES.forEach((m) => {
        sandbox.__t += 5;
        played.length = 0;
        rc.handleEvents([{ type: 'in_boss_attack', detail: { kind: 'brandur', move: m, pattern: 'brandur_' + m } }], null, null);
        assert.ok(played.includes(Sfx.CUES.in_boss_attack.by.map['brandur_' + m].synth), 'attack ' + m + ' plays its recipe');
        sandbox.__t += 5;
        played.length = 0;
        rc.handleEvents([{ type: 'in_boss_tell', detail: { kind: 'brandur', move: m, pattern: 'brandur_' + m } }], null, null);
        assert.ok(played.includes(Sfx.CUES.in_boss_tell.by.map['brandur_' + m].synth), 'tell ' + m + ' plays its recipe');
    });
    sandbox.__t += 5;
    played.length = 0;
    rc.handleEvents([{ type: 'in_boss_attack', detail: { kind: 'brandur', move: 'x', pattern: 'brandur_unknown' } }], null, null);
    assert.ok(played.includes('dp_a_breath'), 'an unknown Brandur move still sounds like Brandur');
    rc.dispose();
    Object.keys(originals).forEach((name) => { Sfx.SYNTHS[name] = originals[name]; });
}

// ---- 3. the new mob voices ----
['emberling', 'drakeling'].forEach((kind) => {
    ['in_mob_tell', 'in_mob_attack'].forEach((type) => {
        const v = Audio.CUE_MAP[type].by.map[kind];
        assert.ok(v && Audio.hasSynth(v.synth) && /^dp_/.test(v.synth), type + ' has its own voice for ' + kind);
        assert.ok(hasTrim(type + ':' + kind), 'TRIM key for ' + type + ':' + kind);
    });
});
assert.notEqual(Audio.CUE_MAP.in_mob_tell.by.map.emberling.synth, Audio.CUE_MAP.in_mob_tell.by.map.drakeling.synth, 'the two mobs sound different');
assert.ok(Audio.CUE_MAP.in_mob_tell.by.map.rat && Audio.CUE_MAP.in_mob_attack.by.map.rat, 'the old mob voices are kept');

// ---- 4. every deeps recipe runs on the strict fake context, for several seeds and rates; every new cue makes sound for every variant ----
const dpSynths = Object.keys(Sfx.SYNTHS).filter((k) => /^dp_/.test(k));
assert.ok(dpSynths.length >= 30, 'the deeps recipes exist (' + dpSynths.length + ')');
{
    const ctx = new FakeContext();
    dpSynths.forEach((name) => {
        [[0.4, 1, 0], [0.8, 0.7, 0.37], [0.2, 1.6, 0.91]].forEach(([gain, sr, v]) => {
            const before = nodeCount;
            const res = Sfx.SYNTHS[name](ctx, node('noise'), node('dest'), 1, { gain, sr, v });
            assert.ok(Array.isArray(res) && nodeCount > before, name + ' builds nodes');
        });
    });
}
{
    const cues = ['in_tablet', 'dr_bell', 'dr_outcome', 'dr_blessing', 'dr_wake', 'dr_scale', 'dr_drowsy', 'dr_impact', 'craft_cook', 'craft_strike', 'craft_done', 'in_boss_phase', 'in_boss_die', 'in_mob_tell', 'in_mob_attack'];
    cues.forEach((type) => {
        const cue = cueOf(type);
        const keys = cue.by ? Object.keys(cue.by.map) : [''];
        keys.forEach((k) => {
            sandbox.__t += 5;
            const each = Audio.createAudioController({ manifest });
            each.unlock();
            const detail = {};
            if (cue.by) detail[cue.by.key] = k === 'true' ? true : k === 'false' ? false : k;
            const before = nodeCount;
            each.handleEvents([{ type, detail }], null, null);
            assert.ok(nodeCount > before, type + (k ? ' [' + k + ']' : '') + ' makes sound');
            each.dispose();
        });
    });
}

// ---- 5. room tones for the mine and the glut ----
const outdoor = { stage: { ambience: { primary: 'forest-day', secondary: 'stream' } } };
['mine', 'glut'].forEach((theme) => {
    const spec = Mix.ambienceFor({ stage: outdoor.stage, interior: { room: { def: { theme } } } });
    assert.ok(spec && spec.primary && Audio.REQUIRED_SOUNDS.includes(spec.primary), theme + ' tone has a known primary');
    [spec.secondary, spec.tertiary].forEach((id) => assert.ok(!id || Audio.REQUIRED_SOUNDS.includes(id), theme + ' tone uses known sounds'));
    assert.ok(spec.level > 0 && spec.level <= 1, theme + ' tone has a level');
    assert.notEqual(JSON.stringify(spec), JSON.stringify(Mix.ambienceFor({ stage: outdoor.stage, interior: { room: { def: { theme: 'cave' } } } })), theme + ' differs from the cave tone');
});
assert.equal(Mix.ambienceFor({ stage: outdoor.stage, interior: { room: { def: { theme: 'mine' } } } }).primary, 'rain-light', 'the mine drips');
assert.ok(['engine', 'fire'].includes(Mix.ambienceFor({ stage: outdoor.stage, interior: { room: { def: { theme: 'glut' } } } }).primary), 'the glut rumbles and burns');
// Neither tone maps its primary to a biome, so entering the mine does not swap the reverb.
assert.ok(!['wind-gentle', 'wind-strong', 'forest-night', 'industrial', 'forest-day'].includes(Mix.ambienceFor({ interior: { room: { def: { theme: 'mine' } } } }).primary));

// ---- 6. music: in_mine and in_lair ----
assert.equal(PlaceMusic.THEME_IDS.join(), 'village,in_cozy,in_cave,in_obs', 'PlaceMusic.THEME_IDS is unchanged');
assert.equal(DeepsMusic.THEME_IDS.join(), 'in_mine,in_lair');
const interiorSim = (music) => ({ stage: { id: 1, biome: { id: 'woodland' }, enemies: [], bossId: null }, player: { x: 0, y: 0 }, state: 'playing', events: [], interior: { room: { def: { music } } } });
DeepsMusic.THEME_IDS.forEach((id) => {
    const th = Music.THEMES[id];
    assert.ok(th && !th.boss && th.prog.length === 2 && th.prog[0].length === 4, id + ' is registered');
    assert.ok(!th.scale.includes(4), id + ' has no major third');
    Music.LAYERS.forEach((layer) => {
        for (let bar = 0; bar < 16; bar++) {
            const notes = Music.composeBar(id, layer, bar, 7);
            assert.equal(JSON.stringify(notes), JSON.stringify(Music.composeBar(id, layer, bar, 7)), id + ' composes deterministically');
            notes.forEach((n) => assert.ok(Number.isFinite(n.t) && n.t >= 0 && n.t < 4 && Number.isFinite(n.dur) && n.dur > 0 && (n.midi === null || Number.isFinite(n.midi)), id + ' ' + layer + ' note is finite'));
        }
    });
    const count = (layer) => { let n = 0; for (let b = 0; b < 8; b++) n += Music.composeBar(id, layer, b, 7).length; return n; };
    assert.ok(count('pad') > 0 && count('perc') > 0, id + ' has a drone and percussion');
    assert.equal(PlaceMusic.themeFor(interiorSim(id)), id, 'themeFor returns ' + id);
    assert.equal(Music.themeIdFor(interiorSim(id), false), id);
});
assert.equal(PlaceMusic.themeFor(interiorSim('nonsense')), 'in_cozy', 'an unknown id still falls back to the cozy loop');
assert.ok(Music.composeBar('in_lair', 'drama', 0, 1).length >= 8, 'in_lair has a drama layer');
assert.ok(Music.composeBar('in_lair', 'drama', 0, 1).some((n) => n.type === 'taiko') && Music.composeBar('in_lair', 'drama', 0, 1).some((n) => n.type === 'cello'), 'the drama layer drives with cello and taiko');
assert.ok(Music.THEMES.in_lair.bpm > Music.THEMES.in_cave.bpm && Music.THEMES.in_lair.bpm >= 80, 'the lair is a faster boss loop');
assert.ok(Music.THEMES.in_mine.bpm < Music.THEMES.in_cave.bpm && Music.THEMES.in_mine.bpm <= 60, 'the mine is slower than the cave');
const mineNotes = (layer, bars) => { let all = []; for (let b = 0; b < bars; b++) all = all.concat(Music.composeBar('in_mine', layer, b, 3)); return all; };
assert.ok(mineNotes('perc', 8).some((n) => n.type === 'clock') && mineNotes('perc', 8).some((n) => n.type === 'swell'), 'the mine has picks and wooden creaks');
assert.ok(mineNotes('melody', 8).length <= 8 && mineNotes('pad', 8).length <= 8, 'the mine is sparse');
{
    const port = { ctx: new FakeContext(), bus: node('bus'), reverb: node('rev') };
    const music = Music.createMusicController(port);
    const tick = (n, s) => { for (let i = 0; i < n; i++) { sandbox.__t += 1 / 30; music.update(Music.intensityFor(s), s); } };
    sandbox.__t = 100;
    tick(60, interiorSim('in_mine'));
    assert.equal(music.getState().theme, 'in_mine', 'entering the mine switches the loop');
    const lair = interiorSim('in_lair');
    lair.interior.hero = { x: 100, y: 100 };
    lair.interior.mobs = [];
    lair.interior.guardian = { state: 'chase', x: 600, y: 600 };
    tick(120, lair);
    assert.equal(music.getState().theme, 'in_lair', 'the lair loop keeps playing during the fight');
    assert.ok(music.getState().voices > 0, 'the lair schedules voices');
    music.dispose();
}

// ---- 7. determinism, no URLs, line budget ----
['sfx-deeps.js', 'deeps-music.js'].forEach((rel) => {
    const src = fs.readFileSync(path.join(dir, rel), 'utf8');
    assert.ok(!src.includes('Math.random') && !src.includes('Date.now'), rel + ' stays deterministic');
    assert.ok(!/https?:\/\//.test(src), rel + ' has no URL strings');
    assert.ok(src.split('\n').length < 1100, rel + ' stays below the line budget');
});
['audio.js', 'audio-mix.js', 'place-music.js', 'sfx-rooms.js'].forEach((rel) => assert.ok(fs.readFileSync(path.join(dir, rel), 'utf8').split('\n').length < 1000, rel + ' stays below 1000 lines'));
console.log('Runenpfad deeps audio checks passed: ' + dpSynths.length + ' recipes, ' + Object.keys(pushed).length + ' pushed events scanned, ' + DeepsMusic.THEME_IDS.length + ' themes.');
