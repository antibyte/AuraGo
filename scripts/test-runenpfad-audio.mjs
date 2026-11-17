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
['biomes.js', 'fx.js', 'physics.js', 'behaviors.js', 'bosses.js', 'sfx.js', 'audio.js', 'music.js'].forEach(load);
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
    'enemy_block', 'enemy_phase', 'enemy_windup', 'enemy_slam', 'spore_burst', 'spore_lob', 'stomp', 'chest_open',
    'rune_pickup', 'rune_expire', 'rune_shield_break', 'rune_zap', 'geyser_burst', 'switch_on', 'gate_open', 'jet_warn', 'jet_fire',
    'boss_intro', 'boss_phase', 'boss_defeat', 'boss_slam', 'boss_telegraph', 'boss_attack', 'enemy_attack', 'enemy_dive'
];
CONTRACT_EVENTS.forEach((type) => assert.ok(Audio.CUE_MAP[type], `cue missing for ${type}`));
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
controller.dispose();

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
console.log('Runenpfad audio checks passed: ' + Object.keys(Audio.CUE_MAP).length + ' cues, ' + Object.keys(Sfx.SYNTHS).length + ' synth recipes, ' + themeIds.length + ' music themes.');
