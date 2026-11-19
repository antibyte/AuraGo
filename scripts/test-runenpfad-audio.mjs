// Runenpfad audio checks: cue coverage, every synth recipe on a strict fake AudioContext, music for every theme.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const sandbox = { console, Math, JSON, Object, Array, Float32Array, Promise, Map, Set, Uint8Array };
sandbox.globalThis = sandbox;
sandbox.fetch = () => Promise.reject(new Error('offline test'));
vm.createContext(sandbox);
const load = (rel) => vm.runInContext(fs.readFileSync(path.join(root, 'ui/js/desktop/apps/runenpfad', rel), 'utf8'), sandbox, { filename: rel });
['biomes.js', 'fx.js', 'physics.js', 'behaviors.js', 'bosses.js', 'sfx.js', 'audio.js', 'music.js', 'duel-music.js', 'place-music.js', 'story-music.js'].forEach(load);
const Audio = sandbox.RunenpfadAudio;
const Sfx = sandbox.RunenpfadSfx;
const Music = sandbox.RunenpfadMusic;

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
['createGain', 'createOscillator', 'createBiquadFilter', 'createBufferSource', 'createDynamicsCompressor', 'createConvolver', 'createStereoPanner', 'createWaveShaper']
    .forEach((m) => { FakeContext.prototype[m] = function () { return node(m); }; });
sandbox.AudioContext = FakeContext;

// ---- 1. contract coverage ----
const CONTRACT_EVENTS = [
    'enemy_block', 'enemy_phase', 'enemy_windup', 'enemy_slam', 'spore_burst', 'spore_lob', 'stomp', 'chest_open', 'land',
    'rune_pickup', 'rune_expire', 'rune_shield_break', 'rune_zap', 'resonance_start', 'resonance_rune', 'resonance_complete', 'resonance_expire', 'sky_ray_board', 'sky_ray_leap',
    'geyser_burst', 'switch_on', 'gate_open', 'jet_warn', 'jet_fire',
    'boss_intro', 'boss_phase', 'boss_defeat', 'boss_slam', 'boss_telegraph', 'boss_attack', 'enemy_attack', 'enemy_dive'
];
CONTRACT_EVENTS.forEach((type) => assert.ok(Audio.CUE_MAP[type], `cue missing for ${type}`));
assert.equal(Audio.CUE_MAP.jump.synth, 'whoosh', 'the common jump keeps its sample and adds a soft lift layer');
const PATTERNS = ['charge', 'ground_pound', 'leap', 'volley', 'summon', 'ring', 'laser', 'dive', 'missiles', 'fire_strip'];
const table = (sandbox.RunenpfadBosses && sandbox.RunenpfadBosses.PATTERNS) || {};
const bossPatterns = [].concat(...Object.keys(table).map((id) => [].concat(table[id].p1 || [], table[id].p2 || [])));
bossPatterns.forEach((p) => assert.ok(Audio.CUE_MAP.boss_attack.by.map[p], `boss attack sound missing for ${p}`));
PATTERNS.forEach((p) => assert.ok(Audio.CUE_MAP.boss_attack.by.map[p], `boss attack sound missing for ${p}`));
['charge', 'pound', 'leap', 'volley', 'summon', 'ring', 'beam', 'dive', 'missiles', 'fire'].forEach((k) => assert.ok(Audio.CUE_MAP.boss_telegraph.by.map[k], `boss telegraph sound missing for ${k}`));
['fire', 'frost', 'storm', 'magnet'].forEach((k) => assert.ok(Audio.CUE_MAP.rune_pickup.by.map[k], `rune pickup sound missing for ${k}`));
assert.ok(Audio.CUE_MAP.hazard.by.map.spikes && !Audio.CUE_MAP.hazard.by.map.spike, 'hazard uses the spikes kind');
assert.ok(Audio.REQUIRED_SOUNDS.includes('rain-light'));

// Every referenced sample id and synth name must exist.
const manifest = { assets: Audio.REQUIRED_SOUNDS.map((id) => ({ id, gain: 0.8, files: [{ file: 'sounds/' + id + '.wav' }] })) };
const sampleIds = new Set(Audio.REQUIRED_SOUNDS);
function walkCue(name, cue) {
    if (typeof cue.sound === 'string') assert.ok(sampleIds.has(cue.sound), `${name}: unknown sample ${cue.sound}`);
    if (cue.synth) assert.ok(Audio.hasSynth(cue.synth), `${name}: unknown synth ${cue.synth}`);
    (cue.also || []).forEach((a) => assert.ok(Audio.hasSynth(a.synth), `${name}: unknown also-synth ${a.synth}`));
    if (cue.by) Object.keys(cue.by.map).forEach((k) => walkCue(name + '/' + k, cue.by.map[k]));
    if (cue.byEnemy) Object.keys(cue.byEnemy).forEach((k) => walkCue(name + '/' + k, cue.byEnemy[k]));
}
Object.keys(Audio.CUE_MAP).forEach((n) => walkCue(n, Audio.CUE_MAP[n]));
Object.keys(Audio.BOSS_VOICES).forEach((b) => Audio.BOSS_VOICES[b].forEach((v) => assert.ok(Audio.hasSynth(v.synth), `voice ${v.synth}`)));

// ---- 2. every synth recipe builds without throwing on a strict context ----
const ac = new FakeContext();
const noise = node('noise');
Object.keys(Sfx.SYNTHS).forEach((name) => {
    [0.5, 1, 2.6].forEach((sr) => {
        [false, true].forEach((short) => {
            const res = Sfx.SYNTHS[name](ac, noise, node('dest'), 1, { gain: 0.5, sr, v: 0.37, short, count: 3, freq: 1046 });
            assert.ok(Array.isArray(res) && res[0] && Array.isArray(res[1]), `${name} returns [anchor, nodes]`);
        });
    });
});

// ---- 3. the controller routes every cue; unknown events stay silent ----
const controller = Audio.createAudioController({ manifest });
controller.unlock();
const detailsFor = (type) => {
    const base = { x: 100, y: 100, id: 1 };
    const cue = Audio.CUE_MAP[type];
    if (!cue.by) return [base];
    const keys = Object.keys(cue.by.map).filter((k) => k !== '*').concat(['unknown_kind']);
    return keys.map((k) => Object.assign({}, base, { [cue.by.key]: k }));
};
let clock = 0;
Object.keys(Audio.CUE_MAP).forEach((type) => {
    detailsFor(type).forEach((detail) => {
        clock += 5;
        sandbox.__t = clock;
        controller.handleEvents([{ type, detail }], null, { x: 0, w: 800 });
    });
});
controller.handleEvents([{ type: 'enemy_attack', detail: { kind: 'shot', enemy: 'crystal_bat', x: 1, y: 1 } }], null, { x: 0, w: 800 });
controller.handleEvents([{ type: 'enemy_phase', detail: { phased: true, x: 1, y: 1 } }, { type: 'enemy_phase', detail: { phased: false, x: 1, y: 1 } }], null, null);
controller.setAmbience({ primary: 'forest-night', secondary: 'space-hum' });
sandbox.__t += 10;
const loud = nodeCount;
controller.handleEvents([{ type: 'boss_attack', detail: { pattern: 'fire_strip', x: 1, y: 1 } }], null, null);
assert.ok(nodeCount > loud, 'fire_strip builds synth layers');
sandbox.__t += 10;
const before = nodeCount;
controller.handleEvents([{ type: 'totally_unknown_event', detail: { x: 1 } }, null, {}], null, null);
assert.equal(nodeCount, before, 'unknown events are silent');
assert.ok(controller.liveSynths() > 0 && controller.liveSynths() <= 40, 'live synth voices stay under the hard cap: ' + controller.liveSynths());
controller.dispose();

// Landing layers use the world surface and impact speed; legacy callers still get the land sample.
assert.equal(Audio.CUE_MAP.land.sound, null);
assert.equal(Audio.CUE_MAP.land.speed, 'impactSpeed');
['step-grass', 'step-stone', 'step-gravel', 'step-wood', 'step-metal'].forEach((surface) => {
    assert.ok(Audio.CUE_MAP.land.by.map[surface], 'landing tail for ' + surface);
});
const requestedSamples = [];
const savedFetch = sandbox.fetch;
sandbox.fetch = (url) => { requestedSamples.push(url); return Promise.reject(new Error('offline test')); };
const landingSpeeds = [];
const originalLanding = Sfx.SYNTHS.landing;
Sfx.SYNTHS.landing = function (ctx, noise, dest, time, options) {
    landingSpeeds.push(options.speed);
    return originalLanding(ctx, noise, dest, time, options);
};
const lander = Audio.createAudioController({ manifest });
lander.unlock();
const landBefore = nodeCount;
lander.handleEvents([{ type: 'land', detail: { x: 20, y: 30, sound: 'step-metal', impactSpeed: 740 } }], null, null);
assert.ok(nodeCount > landBefore, 'landing creates the impact and material layers');
assert.ok(requestedSamples.some((url) => url.includes('step-metal.wav')), 'landing loads its metal surface sample');
assert.deepEqual(landingSpeeds, [740], 'landing impact keeps its incoming speed');
assert.ok(Audio.CUE_MAP.land.by.map['step-metal'].also.some((layer) => layer.synth === 'clank'), 'metal landing rings after the thump');
const raySurfaceBefore = nodeCount;
const raySamplesBefore = requestedSamples.length;
lander.handleEvents([
    { type: 'step', detail: { sound: 'step-grass', skyRay: true } },
    { type: 'land', detail: { sound: 'step-grass', impactSpeed: 500, skyRay: true } }
], null, null);
assert.equal(nodeCount, raySurfaceBefore, 'Windray riding suppresses terrain and ordinary landing synths');
assert.equal(requestedSamples.length, raySamplesBefore, 'Windray riding does not request terrain samples');
const slamBefore = nodeCount;
lander.handleEvents([{ type: 'land', detail: { x: 20, y: 30, sound: 'step-metal', impactSpeed: 740, slam: true } }], null, null);
assert.equal(nodeCount, slamBefore, 'slam landing does not double-play the ordinary landing cue');
lander.dispose();
sandbox.fetch = savedFetch;
Sfx.SYNTHS.landing = originalLanding;

// Resonance notes follow each biome scale and keep every contact distinct.
const resonanceFreqs = [];
const resonancePhrases = [];
const originalResonance = Sfx.SYNTHS.resonance;
Sfx.SYNTHS.resonance = function (ctx, noise, dest, time, options) {
    resonanceFreqs.push(options.freq);
    resonancePhrases.push(options.notes || null);
    return originalResonance(ctx, noise, dest, time, options);
};
const resonance = Audio.createAudioController({ manifest });
resonance.unlock();
resonance.setAmbience('forest-night');
[
    { type: 'resonance_start', detail: { index: 0 } },
    { type: 'resonance_rune', detail: { index: 1 } },
    { type: 'resonance_rune', detail: { index: 2 } },
    { type: 'resonance_complete', detail: { index: 3 } }
].forEach((event) => {
    sandbox.__t += 100;
    resonance.handleEvents([event], null, null);
});
const cavern = Music.THEMES.cavern;
const expectedResonanceFreqs = [0, 1, 2, 3].map((index) => 440 * Math.pow(2, (cavern.root + 12 + cavern.scale[index] - 69) / 12));
assert.deepEqual(resonanceFreqs, expectedResonanceFreqs, 'resonance contacts follow the current biome scale');
assert.equal(new Set(resonanceFreqs).size, 4, 'each resonance stage has its own pitch');
assert.deepEqual(resonancePhrases.slice(0, 3), [null, null, null], 'rune contacts do not add extra musical phrases');
assert.deepEqual(Array.from(resonancePhrases[3]), [0, 2, 4].map((degree) => 440 * Math.pow(2, (cavern.root + 12 + cavern.scale[degree] - 69) / 12)), 'the final cue resolves in the biome scale');
resonance.dispose();
Sfx.SYNTHS.resonance = originalResonance;

// Windray cues use two synth layers and take stable pitches from each biome scale.
const rayPitches = [];
const rayLayers = [];
const originalRayBoard = Sfx.SYNTHS.ray_board;
const originalRayLeap = Sfx.SYNTHS.ray_leap;
const originalRayGlint = Sfx.SYNTHS.ray_glint;
['ray_board', 'ray_leap', 'ray_glint'].forEach((name) => {
    const original = Sfx.SYNTHS[name];
    Sfx.SYNTHS[name] = function (ctx, buffer, dest, time, options) {
        rayLayers.push(name);
        rayPitches.push([name, options.freq]);
        return original(ctx, buffer, dest, time, options);
    };
});
const windray = Audio.createAudioController({ manifest });
windray.unlock();
const biomeAmbience = { woodland: 'forest-day', cavern: 'forest-night', desert: 'wind-strong', glacier: 'wind-gentle', citadel: 'industrial' };
Object.keys(biomeAmbience).forEach((biome) => {
    windray.setAmbience(biomeAmbience[biome]);
    ['sky_ray_board', 'sky_ray_leap'].forEach((type) => {
        sandbox.__t += 350;
        windray.handleEvents([{ type, detail: { x: 120, y: 80 } }], null, null);
    });
});
const expectedRayPitches = Object.keys(biomeAmbience).flatMap((biome) => [4, 5].flatMap((index) => {
    const freq = Sfx.resonanceNotes(Music.THEMES[biome], index).freq;
    return [[index === 4 ? 'ray_board' : 'ray_leap', freq], ['ray_glint', freq]];
}));
assert.deepEqual(rayPitches, expectedRayPitches, 'Windray layers stay in each biome scale and differ between boarding and leaping');
assert.equal(rayLayers.filter((name) => name === 'ray_glint').length, 10, 'each Windray event has one soft harmonic layer');
windray.dispose();
[ ['ray_board', originalRayBoard], ['ray_leap', originalRayLeap], ['ray_glint', originalRayGlint] ].forEach(([name, original]) => { Sfx.SYNTHS[name] = original; });
const mutedResonance = Audio.createAudioController({ manifest, enabled: false });
const mutedBefore = nodeCount;
mutedResonance.handleEvents([{ type: 'resonance_start', detail: { index: 0 } }], null, null);
assert.equal(nodeCount, mutedBefore, 'the sound mute silences resonance contacts');
mutedResonance.dispose();
const mutedRay = Audio.createAudioController({ manifest, enabled: false });
const mutedRayBefore = nodeCount;
mutedRay.handleEvents([{ type: 'sky_ray_board', detail: { x: 1, y: 1 } }, { type: 'sky_ray_leap', detail: { x: 1, y: 1 } }], null, null);
assert.equal(nodeCount, mutedRayBefore, 'the sound mute silences Windray cues');
mutedRay.dispose();
const independentSfx = Audio.createAudioController({ manifest });
independentSfx.unlock();
const mutedMusic = Music.createMusicController(independentSfx.musicPort());
mutedMusic.setEnabled(false);
const sfxWithMusicMuted = nodeCount;
independentSfx.handleEvents([{ type: 'sky_ray_board', detail: { x: 1, y: 1 } }], null, null);
assert.ok(nodeCount > sfxWithMusicMuted, 'the music mute leaves Windray sound effects enabled');
mutedMusic.dispose();
independentSfx.dispose();

// ---- 3b. round 4 events: every new event, glyph and pattern maps to a cue and plays ----
const R4_EVENTS = ['swing_grab', 'swing_release', 'seq_show', 'seq_hit', 'seq_fail', 'seq_solved', 'resonance_start', 'resonance_rune', 'resonance_complete', 'resonance_expire', 'lever_pull', 'bridge_toggle', 'bridge_done',
    'plate_down', 'stairs_up', 'stairs_down', 'brazier_lit', 'ritual_done', 'ritual_fail', 'dragon_roar', 'dragon_warn', 'dragon_breath', 'dragon_pass', 'fireball_hit'];
R4_EVENTS.forEach((type) => assert.ok(Audio.CUE_MAP[type], 'cue missing for ' + type));
[0, 1, 2, 3].forEach((g) => ['seq_show', 'seq_hit'].forEach((type) => assert.ok(Audio.CUE_MAP[type].by.map[g], type + ' glyph ' + g)));
['strafe', 'fireballs', 'roar'].forEach((p) => {
    assert.ok(Audio.CUE_MAP.boss_attack.by.map[p], 'dragon boss attack sound missing for ' + p);
    assert.ok(Audio.CUE_MAP.boss_telegraph.by.map[p], 'dragon boss telegraph sound missing for ' + p);
});
assert.ok(Audio.CUE_MAP.enemy_attack.by.map.fireball, 'drakeling fireball spit');
const R4_CASES = [];
R4_EVENTS.forEach((type) => R4_CASES.push({ type, detail: { x: 300, y: 200 } }, { type, detail: {} }, { type }));
[0, 1, 2, 3].forEach((glyph) => ['seq_show', 'seq_hit'].forEach((type) => R4_CASES.push({ type, detail: { glyph, index: glyph, step: glyph, x: 50, y: 50 } })));
R4_CASES.push({ type: 'seq_show', detail: { glyph: 9 } }, { type: 'swing_release', detail: { speed: 0 } }, { type: 'swing_release', detail: { speed: 900, x: 5 } }, { type: 'swing_release', detail: { speed: 'fast' } },
    { type: 'bridge_toggle', detail: { on: true } }, { type: 'bridge_toggle', detail: { on: false } }, { type: 'brazier_lit', detail: { order: 0 } }, { type: 'brazier_lit', detail: { order: 4 } },
    { type: 'dragon_roar', detail: { kind: 'ember' } }, { type: 'dragon_roar', detail: { kind: 'frost' } }, { type: 'dragon_breath', detail: { kind: 'frost', x: 1, y: 1 } },
    { type: 'dragon_warn', detail: { x1: 0, x2: 900, laneY: 400, laneH: 60 } }, { type: 'enemy_attack', detail: { id: 4, defId: 'drakeling', kind: 'fireball', x: 10, y: 10 } });
['strafe', 'fireballs', 'roar'].forEach((pattern) => ['boss_dragon_ember', 'boss_dragon_frost', 'boss_forest'].forEach((defId) => {
    R4_CASES.push({ type: 'boss_attack', detail: { pattern, defId, x: 10, y: 10 } }, { type: 'boss_telegraph', detail: { kind: pattern, defId, x: 10, y: 10 } });
}));
R4_CASES.push({ type: 'boss_intro', detail: { defId: 'boss_dragon_frost' } });
R4_CASES.forEach((ev) => {
    const r4 = Audio.createAudioController({ manifest }); // fresh: the fake never ends voices, so the live-voice cap would fill up
    r4.unlock();
    sandbox.__t += 5;
    const nodesBefore = nodeCount;
    r4.handleEvents([ev], null, { x: 0, w: 800 });
    assert.ok(nodeCount > nodesBefore, 'event builds sound: ' + JSON.stringify(ev));
    assert.ok(nodeCount - nodesBefore < 700, 'event stays within a sane node budget: ' + JSON.stringify(ev) + ' ' + (nodeCount - nodesBefore));
    assert.ok(r4.liveSynths() <= 40);
    r4.dispose();
});
const r4 = Audio.createAudioController({ manifest });
r4.unlock();
sandbox.__t += 5;
const dragonNodes = nodeCount;
r4.handleEvents([{ type: 'boss_attack', detail: { pattern: 'strafe', defId: 'boss_dragon_frost', x: 1, y: 1 } }], null, null);
assert.ok(nodeCount > dragonNodes, 'dragon voice plays');
sandbox.__t += 5;
const silent = nodeCount;
r4.handleEvents([{ type: 'seq_wobble', detail: { glyph: 1 } }, { type: 'dragon_sneeze' }], null, null);
assert.equal(nodeCount, silent, 'unknown round 4 events are silent');
// Burst: many simultaneous big cues never exceed the hard voice cap.
sandbox.__t += 50;
for (let i = 0; i < 80; i++) {
    sandbox.__t += 2;
    r4.handleEvents([{ type: 'dragon_roar', detail: { kind: i % 2 ? 'frost' : 'ember', x: i } }, { type: 'dragon_breath', detail: { kind: 'ember', x: i } }, { type: 'dragon_pass', detail: { x: i } }, { type: 'seq_solved', detail: { x: i } }], null, null);
    assert.ok(r4.liveSynths() <= 40, 'burst stays under the hard cap: ' + r4.liveSynths());
}
r4.dispose();

// ---- 4. music builds every biome and boss theme ----
const themeIds = Object.keys(Music.THEMES);
['woodland', 'cavern', 'desert', 'glacier', 'citadel'].forEach((b) => assert.ok(Music.THEMES[b] && Music.THEMES['boss_' + b]));
const bossInstr = new Set(['woodland', 'cavern', 'desert', 'glacier', 'citadel'].map((b) => Music.THEMES['boss_' + b].leadInstr));
assert.equal(bossInstr.size, 5, 'each boss has its own lead instrument');
assert.ok(Music.THEMES.boss_desert.scale.includes(1) && Music.THEMES.boss_desert.scale.includes(3), 'desert boss stays dark (flat 2nd, minor 3rd)');
const stage = { id: 3, biome: { id: 'woodland' }, enemies: [], bossId: null };
const sim = { stage, player: { x: 0, y: 0 }, state: 'playing', events: [], bossIntro: null };
const bossSim = { stage, player: { x: 0, y: 0 }, state: 'playing', events: [], bossIntro: null };
themeIds.forEach((id) => {
    const port = { ctx: new FakeContext(), bus: node('bus'), reverb: node('rev') };
    const music = Music.createMusicController(port);
    assert.ok(music);
    music.setTheme(id);
    for (let i = 0; i < 360; i++) {
        sandbox.__t = i / 30;
        music.update({ danger: (i % 120) / 120, boss: id.startsWith('boss_'), bossAlive: id.startsWith('boss_'), state: 'playing' }, i === 100 ? Object.assign(bossSim, { events: [{ type: 'rune_pickup' }] }) : sim);
    }
    assert.equal(music.getState().theme, id, `${id} is playing`);
    assert.ok(music.getState().voices <= Music.VOICE_CAP);
    ['victory', 'defeat', 'boss_intro', 'rune', 'checkpoint'].forEach((k) => music.stinger(k));
    music.dispose();
});

// ---- 5. NPC encounters: every pushed event has a cue, duel patterns/themes/stingers exist ----
const NPC_EVENTS = ['npc_near', 'npc_talk', 'npc_close', 'npc_text', 'npc_choice', 'npc_rep', 'npc_trade', 'npc_quest', 'quest_pickup', 'npc_provoke',
    'duel_start', 'duel_tell', 'duel_attack', 'duel_hit', 'duel_block', 'duel_roll', 'duel_phase', 'duel_special', 'duel_end'];
NPC_EVENTS.forEach((type) => assert.ok(Audio.CUE_MAP[type], 'cue missing for ' + type));
['npcs.js', 'dialogue.js', 'trade.js', 'duel.js', 'duel-patterns.js'].forEach((rel) => {
    const file = path.join(root, 'ui/js/desktop/apps/runenpfad', rel);
    if (!fs.existsSync(file)) return;
    const src = fs.readFileSync(file, 'utf8');
    for (const m of src.matchAll(/pushEvent\x28sim, '([a-z_]+)'/g)) assert.ok(Audio.CUE_MAP[m[1]], rel + ' pushes ' + m[1] + ' without an audio cue');
});
const NPC_IDS = ['witch', 'wizard', 'merchant', 'chief', 'seer', 'alchemist'];
NPC_IDS.forEach((npc) => ['a', 'b', 'c'].forEach((p) => {
    assert.ok(Audio.CUE_MAP.duel_tell.by.map[npc + '_' + p], 'duel tell sound missing for ' + npc + '_' + p);
    assert.ok(Audio.CUE_MAP.duel_attack.by.map[npc + '_' + p], 'duel attack sound missing for ' + npc + '_' + p);
}));
const DUEL_CASES = [];
NPC_IDS.forEach((npc) => ['a', 'b', 'c'].forEach((pattern) => ['duel_tell', 'duel_attack'].forEach((type) => DUEL_CASES.push({ type, detail: { npcId: npc, pattern: npc + '_' + pattern } }))));
DUEL_CASES.push({ type: 'duel_tell', detail: { npcId: 'x', pattern: 'x_a' } }, { type: 'duel_start', detail: { npcId: 'witch' } }, { type: 'duel_roll' }, { type: 'duel_phase', detail: { npcId: 'witch' } },
    { type: 'duel_hit', detail: { target: 'hero', dmg: 2 } }, { type: 'duel_hit', detail: { target: 'npc', dmg: 4 } }, { type: 'duel_block', detail: { by: 'hero' } }, { type: 'duel_block', detail: { by: 'npc' } },
    { type: 'duel_end', detail: { npcId: 'witch', result: 'won' } }, { type: 'duel_end', detail: { npcId: 'witch', result: 'lost' } },
    { type: 'npc_rep', detail: { faction: 'coven', delta: 1, dir: 'up' } }, { type: 'npc_rep', detail: { faction: 'coven', delta: -1, dir: 'down' } },
    { type: 'npc_quest', detail: { questId: 'q', state: 'start' } }, { type: 'npc_quest', detail: { questId: 'q', state: 'done' } },
    { type: 'npc_text' }, { type: 'npc_close' }, { type: 'npc_trade', detail: { npcId: 'witch', offerId: 'potion', fake: false } });
['fire', 'frost', 'storm', 'magnet'].forEach((kind) => DUEL_CASES.push({ type: 'duel_special', detail: { kind } }));
NPC_EVENTS.forEach((type) => DUEL_CASES.push({ type, detail: {} }, { type }));
DUEL_CASES.forEach((ev) => {
    const dc = Audio.createAudioController({ manifest });
    dc.unlock();
    sandbox.__t += 5;
    const nodesBefore = nodeCount;
    dc.handleEvents([ev], null, { x: 0, w: 800 });
    assert.ok(nodeCount > nodesBefore, 'duel/npc event builds sound: ' + JSON.stringify(ev));
    assert.ok(nodeCount - nodesBefore < 700, 'duel/npc event stays within a sane node budget: ' + JSON.stringify(ev));
    dc.dispose();
});
// Typewriter blips are rate-limited and pitch-varied deterministically.
const tw = Audio.createAudioController({ manifest });
tw.unlock();
sandbox.__t += 5;
const twBase = nodeCount;
for (let i = 0; i < 10; i++) tw.handleEvents([{ type: 'npc_text' }], null, null);
assert.ok(nodeCount > twBase && nodeCount - twBase < 40, 'npc_text cooldown limits a burst: ' + (nodeCount - twBase));
tw.dispose();
// Duel themes: one per NPC, boss-style, deterministic, dark, and selected during a duel.
const DuelMusic = sandbox.RunenpfadDuelMusic;
assert.equal(DuelMusic.THEME_IDS.length, 6);
NPC_IDS.forEach((npc) => {
    const th = Music.THEMES['duel_' + npc];
    assert.ok(th && th.boss && th.bpm >= 90, 'duel theme for ' + npc);
    assert.ok(!th.scale.includes(4), npc + ' duel theme has no major third');
    assert.equal(th.prog[0].length, 4);
    Music.LAYERS.forEach((layer) => assert.equal(JSON.stringify(Music.composeBar('duel_' + npc, layer, 2, 9)), JSON.stringify(Music.composeBar('duel_' + npc, layer, 2, 9))));
    assert.ok(Music.composeBar('duel_' + npc, 'perc', 0, 1).length > 0 && Music.composeBar('duel_' + npc, 'boss', 0, 1).length > 0, npc + ' has drums and an ostinato');
});
assert.equal(new Set(NPC_IDS.map((n) => Music.THEMES['duel_' + n].leadInstr + '/' + Music.THEMES['duel_' + n].bpm)).size, 6, 'every duel loop sounds different');
const duelSim = { stage, player: { x: 0, y: 0 }, state: 'playing', events: [], bossIntro: null, encounter: { kind: 'duel', npcId: 'seer', duel: { npcId: 'seer' } } };
const di = Music.intensityFor(duelSim);
assert.ok(di.boss && di.bossAlive && di.danger === 1, 'intensityFor reports boss during a duel');
assert.equal(Music.themeIdFor(duelSim, true), 'duel_seer');
assert.equal(Music.themeIdFor(Object.assign({}, duelSim, { encounter: null }), true), 'boss_woodland', 'no duel, no duel theme');
{
    const port = { ctx: new FakeContext(), bus: node('bus'), reverb: node('rev') };
    const music = Music.createMusicController(port);
    for (let i = 0; i < 120; i++) { sandbox.__t = i / 30; music.update(Music.intensityFor(duelSim), duelSim); }
    assert.equal(music.getState().theme, 'duel_seer', 'duel switches the music to the NPC loop');
    const stingSim = Object.assign({}, duelSim, { events: [{ type: 'duel_start', detail: { npcId: 'seer' } }, { type: 'duel_end', detail: { npcId: 'seer', result: 'won' } }] });
    music.update(Music.intensityFor(stingSim), stingSim);
    const after = Object.assign({}, sim, { events: [] });
    for (let i = 120; i < 240; i++) { sandbox.__t = i / 30; music.update(Music.intensityFor(after), after); }
    music.dispose();
}
// ---- 6. places and story: interior events, UI cues, village / interior / story themes, music override ----
const PLACE_EVENTS = ['place_near', 'place_enter', 'place_leave', 'place_room', 'place_ko', 'in_step', 'in_attack', 'in_dash', 'in_hit', 'in_mob_tell',
    'in_mob_attack', 'in_mob_die', 'in_chest', 'in_pickup', 'in_clear', 'in_locked'];
PLACE_EVENTS.forEach((type) => assert.ok(Audio.CUE_MAP[type], 'cue missing for ' + type));
['interior.js', 'interior-mobs.js', 'places.js', 'village.js', 'folk.js'].forEach((rel) => {
    const file = path.join(root, 'ui/js/desktop/apps/runenpfad', rel);
    if (!fs.existsSync(file)) return;
    for (const m of fs.readFileSync(file, 'utf8').matchAll(/pushEvent\x28sim, '((?:place|in)_[a-z_]+)'/g)) assert.ok(Audio.CUE_MAP[m[1]], rel + ' pushes ' + m[1] + ' without an audio cue');
});
const MOBS = ['rat', 'slime', 'bat', 'spider', 'mummy', 'icewisp', 'clockbug'];
MOBS.forEach((k) => ['in_mob_tell', 'in_mob_attack'].forEach((type) => assert.ok(Audio.CUE_MAP[type].by.map[k], type + ' sound missing for ' + k)));
assert.equal(new Set(MOBS.map((k) => Audio.CUE_MAP.in_mob_tell.by.map[k].sr)).size, 7, 'every creature has its own warning pitch');
const PLACE_CASES = [];
PLACE_EVENTS.forEach((type) => PLACE_CASES.push({ type }, { type, detail: {} }));
PLACE_CASES.push({ type: 'place_near', detail: { placeId: 'inn', kind: 'house' } }, { type: 'place_enter', detail: { placeId: 'inn', kind: 'house' } }, { type: 'place_enter', detail: { placeId: 'mine', kind: 'cave' } },
    { type: 'place_leave', detail: { placeId: 'inn', reason: 'exit' } }, { type: 'place_leave', detail: { placeId: 'inn', reason: 'ko' } }, { type: 'place_room', detail: { roomId: 'cellar' } },
    { type: 'in_attack', detail: { hero: 'knight' } }, { type: 'in_attack', detail: { hero: 'ranger' } }, { type: 'in_hit', detail: { target: 'mob', kind: 'rat' } }, { type: 'in_hit', detail: { target: 'hero', kind: 'slime' } },
    { type: 'in_chest', detail: { lootId: 'a', empty: false } }, { type: 'in_chest', detail: { lootId: 'a', empty: true } }, { type: 'in_clear', detail: { roomId: 'cellar' } });
MOBS.forEach((kind) => ['in_mob_tell', 'in_mob_attack', 'in_mob_die'].forEach((type) => PLACE_CASES.push({ type, detail: { kind } })));
['coin', 'heart', 'potion', 'key', 'unknown'].forEach((kind) => PLACE_CASES.push({ type: 'in_pickup', detail: { kind } }));
let panners = 0;
const makePanner = FakeContext.prototype.createStereoPanner;
FakeContext.prototype.createStereoPanner = function () { panners++; return makePanner.call(this); };
PLACE_CASES.forEach((ev) => {
    const pc = Audio.createAudioController({ manifest });
    pc.unlock();
    sandbox.__t += 5;
    const nodesBefore = nodeCount;
    pc.handleEvents([ev], null, { x: 4000, w: 800 }); // far-off camera: an x-less event must stay centred
    assert.ok(nodeCount > nodesBefore, 'place event builds sound: ' + JSON.stringify(ev));
    assert.ok(nodeCount - nodesBefore < 700, 'place event stays within a sane node budget: ' + JSON.stringify(ev));
    pc.dispose();
});
assert.equal(panners, 0, 'interior events are never panned by the world camera');
FakeContext.prototype.createStereoPanner = makePanner;
// Footsteps are rate-limited: a walking burst plays only a few steps.
const stepper = Audio.createAudioController({ manifest });
stepper.unlock();
sandbox.__t += 5;
const stepBase = nodeCount;
for (let i = 0; i < 12; i++) { sandbox.__t += 0.05; stepper.handleEvents([{ type: 'in_step' }], null, null); }
assert.ok(nodeCount > stepBase && nodeCount - stepBase < 12 * 25 / 2, 'in_step is rate-limited: ' + (nodeCount - stepBase));
stepper.dispose();
// Empty chests click instead of creaking open; the hero attack sounds differ.
assert.notEqual(Audio.CUE_MAP.in_chest.by.map.true.synth, Audio.CUE_MAP.in_chest.synth);
assert.ok(Audio.CUE_MAP.in_attack.by.map.knight.sr < Audio.CUE_MAP.in_attack.by.map.ranger.sr, 'knight swings lower than the ranger');
// UI cues play without a simulation through audio.play(cueId).
['cut_open', 'cut_next', 'cut_close'].forEach((cue) => {
    assert.ok(Audio.UI_CUES[cue] && Audio.hasSynth(Audio.UI_CUES[cue].synth), 'UI cue ' + cue);
    const uc = Audio.createAudioController({ manifest });
    uc.unlock();
    uc.setActive(false); // overlays pause the game; the cue must still sound
    sandbox.__t += 5;
    const before = nodeCount;
    uc.play(cue);
    assert.ok(nodeCount > before, 'play(' + cue + ') builds sound');
    sandbox.__t += 5;
    const quiet = nodeCount;
    uc.play('cut_unknown');
    assert.equal(nodeCount, quiet, 'unknown UI cue is silent');
    uc.dispose();
});
{
    const muted = Audio.createAudioController({ manifest, enabled: false });
    const before = nodeCount;
    muted.play('cut_open');
    assert.equal(nodeCount, before, 'play() respects mute');
    muted.dispose();
}
// Themes: three place loops and three story loops, deterministic, dark ones without a major third.
const PlaceMusic = sandbox.RunenpfadPlaceMusic;
const StoryMusic = sandbox.RunenpfadStoryMusic;
assert.equal(PlaceMusic.THEME_IDS.join(), 'village,in_cozy,in_cave');
assert.equal(StoryMusic.THEME_IDS.join(), 'story_dawn,story_dark,story_triumph');
const NEW_THEMES = PlaceMusic.THEME_IDS.concat(StoryMusic.THEME_IDS);
NEW_THEMES.forEach((id) => {
    const th = Music.THEMES[id];
    assert.ok(th && !th.boss && th.prog.length === 2 && th.prog[0].length === 4, id + ' is registered');
    Music.LAYERS.forEach((layer) => {
        for (let bar = 0; bar < 8; bar++) {
            const notes = Music.composeBar(id, layer, bar, 7);
            assert.equal(JSON.stringify(notes), JSON.stringify(Music.composeBar(id, layer, bar, 7)), id + ' composes deterministically');
            notes.forEach((n) => assert.ok(Number.isFinite(n.t) && Number.isFinite(n.dur) && (n.midi === null || Number.isFinite(n.midi)), id + ' note is finite'));
        }
    });
    const bars8 = (layer) => [0, 1, 2, 3, 4, 5, 6, 7].reduce((n, b) => n + Music.composeBar(id, layer, b, 7).length, 0);
    assert.ok(bars8('melody') > 0 && bars8('pad') > 0, id + ' has a melody and a pad or drone');
});
['in_cave', 'story_dark'].forEach((id) => assert.ok(!Music.THEMES[id].scale.includes(4), id + ' has no major third'));
['village', 'in_cozy', 'story_dawn', 'story_triumph'].forEach((id) => assert.ok(Music.THEMES[id].scale.includes(4), id + ' is a major or mixolydian tune'));
assert.ok(Music.THEMES.in_cave.bpm < 70 && Music.THEMES.story_dark.bpm < 70 && Music.THEMES.story_dawn.bpm < 80, 'dark and dawn loops are slow');
assert.ok(Music.THEMES.in_cozy.bpm > Music.THEMES.village.bpm, 'the tavern jig is livelier than the village');
assert.ok(Music.THEMES.story_triumph.vol > Music.THEMES.story_dawn.vol && Music.THEMES.story_triumph.vol > Music.THEMES.story_dark.vol && Music.THEMES.story_triumph.bpm >= 116, 'the fanfare is the loudest story loop');
assert.ok(Music.composeBar('in_cozy', 'pad', 0, 1).every((n) => n.dur <= 0.5) && Music.composeBar('in_cozy', 'melody', 0, 1).every((n) => n.dur <= 1), 'the tavern plays short plucked notes');
assert.ok(Music.composeBar('in_cozy', 'perc', 0, 1).some((n) => n.type === 'tamb') && Music.composeBar('in_cozy', 'perc', 0, 1).some((n) => n.type === 'tom'), 'stomp and tambourine');
assert.ok(Music.composeBar('story_dark', 'perc', 0, 1).some((n) => n.type === 'clock'), 'story_dark ticks like a clock');
assert.ok(Music.composeBar('story_triumph', 'perc', 0, 1).length >= 4, 'story_triumph marches');
assert.ok(Music.composeBar('in_cave', 'perc', 3, 1).length === 0 && Music.composeBar('in_cave', 'perc', 5, 1).some((n) => n.type === 'taiko'), 'the cave has only an occasional low hit');
['lute', 'harp', 'flute', 'fiddle', 'plink', 'cello', 'brass', 'growl', 'metal', 'clock', 'tamb'].forEach((v) => assert.equal(typeof PlaceMusic.VOICES[v], 'function', 'voice ' + v));
// themeFor: interior room music, the hub stage, otherwise nothing.
const interiorSim = (music) => ({ stage: { id: 1, biome: { id: 'woodland' }, enemies: [], bossId: null }, player: { x: 0, y: 0 }, state: 'playing', events: [], interior: { room: { def: { music } } } });
assert.equal(PlaceMusic.themeFor(interiorSim('in_cave')), 'in_cave');
assert.equal(PlaceMusic.themeFor(interiorSim('in_cozy')), 'in_cozy');
assert.equal(PlaceMusic.themeFor(interiorSim('nonsense')), 'in_cozy', 'an unknown room music id falls back to the cozy loop');
const hubSim = Object.assign({}, sim, { stage: Object.assign({}, stage, { hub: true }) });
assert.equal(PlaceMusic.themeFor(hubSim), 'village');
assert.equal(PlaceMusic.themeFor(Object.assign({}, hubSim, { interior: { room: { def: { music: 'in_cave' } } } })), 'in_cave', 'inside a hub house the room loop wins');
assert.equal(PlaceMusic.themeFor(sim), null);
assert.equal(PlaceMusic.themeFor(null), null);
assert.equal(Music.themeIdFor(interiorSim('in_cave'), false), 'in_cave');
assert.equal(Music.themeIdFor(hubSim, false), 'village');
assert.equal(Music.themeIdFor(sim, false), 'woodland', 'ordinary levels keep the biome theme');
assert.equal(Music.themeIdFor(Object.assign({}, duelSim, { interior: { room: { def: { music: 'in_cave' } } } }), true), 'duel_seer', 'a duel wins over the place loop');
{
    const port = { ctx: new FakeContext(), bus: node('bus'), reverb: node('rev') };
    const music = Music.createMusicController(port);
    const tick = (n, s) => { for (let i = 0; i < n; i++) { sandbox.__t += 1 / 30; music.update(Music.intensityFor(s), s); } };
    sandbox.__t = 1;
    tick(60, hubSim);
    assert.equal(music.getState().theme, 'village', 'the hub plays the village loop');
    tick(60, interiorSim('in_cave'));
    assert.equal(music.getState().theme, 'in_cave', 'entering a cave switches the loop');
    // setOverride wins over the normal choice, also with a won stage and no sim at all.
    music.setOverride('story_dawn');
    assert.equal(music.getState().theme, 'story_dawn', 'override plays at once');
    assert.ok(music.getState().voices > 0, 'override deck is scheduled without an update call');
    tick(60, interiorSim('in_cave'));
    assert.equal(music.getState().theme, 'story_dawn', 'override beats the interior loop');
    tick(30, Object.assign({}, sim, { state: 'won' }));
    music.update(null, null);
    assert.equal(music.getState().theme, 'story_dawn', 'override survives a won stage and a missing sim');
    music.setOverride('story_dark');
    assert.equal(music.getState().theme, 'story_dark', 'override can change theme');
    music.setOverride('no_such_theme'); // unknown ids clear the override
    assert.equal(music.getState().theme, null, 'clearing fades the cutscene deck out');
    tick(90, sim);
    assert.equal(music.getState().theme, 'woodland', 'clearing the override returns to normal selection');
    music.setOverride('story_triumph');
    music.setOverride(null);
    tick(90, sim);
    assert.equal(music.getState().theme, 'woodland');
    assert.ok(music.getState().voices <= Music.VOICE_CAP);
    music.dispose();
    music.setOverride('story_dawn'); // after dispose: harmless
}
console.log('Runenpfad audio checks passed: ' + Object.keys(Audio.CUE_MAP).length + ' cues, ' + Object.keys(Sfx.SYNTHS).length + ' synth recipes, ' + themeIds.length + ' music themes.');
