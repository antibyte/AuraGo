import assert from 'node:assert/strict';
import { EVENT_TYPES, CUES } from '../ui/js/desktop/apps/pinball/contracts.js';
import { soundFor, cueSoundFor, createVoiceLimiter, panFor, SILENT_EVENTS, MAX_VOICES } from '../ui/js/desktop/apps/pinball/audio/sfx.js';
import { RECIPES } from '../ui/js/desktop/apps/pinball/audio/recipes.js';
import { pickVoice } from '../ui/js/desktop/apps/pinball/audio/voice.js';
import { SAMPLE_FILES, SAMPLE_NAMES, TAKES, LOOPS, makeLoop } from '../ui/js/desktop/apps/pinball/audio/samples.js';
import { ceilingCurve, CEILING, busGain } from '../ui/js/desktop/apps/pinball/audio/engine.js';
import fs from 'node:fs';

const sample = { impact: { speed: 2, material: 'wood' }, magnet: { state: 'grab' }, flipper: { up: true }, launch: { speed: 5 }, spinner: { rate: 20 } };
for (const type of EVENT_TYPES) {
    const snd = soundFor({ type, id: 'x', ball: 0, tick: 7, ...(sample[type] || {}) });
    if (SILENT_EVENTS.includes(type)) { assert.equal(snd, null, type + ' is silent'); continue; }
    assert.ok(snd, type + ' maps to a recipe'); assert.ok(RECIPES[snd.recipe], type + ' -> unknown recipe ' + snd.recipe);
    assert.ok(snd.gain > 0 && snd.gain <= 1 && Number.isFinite(snd.rate) && Math.abs(snd.pan) <= .8);
}
for (const cue of CUES) { const snd = cueSoundFor(cue); assert.ok(snd && RECIPES[snd.recipe], 'cue ' + cue); }
assert.equal(cueSoundFor('nope'), null);

assert.equal(soundFor({ type: 'impact', material: 'metal', speed: 0.14 }), null, 'impact below 0.15 is silent');
assert.ok(soundFor({ type: 'impact', material: 'metal', speed: 0.16 }));
assert.ok(soundFor({ type: 'impact', material: 'wood', speed: 6 }).gain > soundFor({ type: 'impact', material: 'wood', speed: 1 }).gain, 'impacts scale with speed');

assert.equal(panFor(10), .8); assert.equal(panFor(-10), -.8); assert.ok(Math.abs(panFor(.13) - .4) < 1e-9); assert.equal(panFor(undefined), 0);
assert.equal(soundFor({ type: 'bumper', id: 'b1', x: 0.5 }).pan, .8);

const lim = createVoiceLimiter(3);
const a = lim.acquire(5, 10, 0), b = lim.acquire(1, 10, 0), c = lim.acquire(3, 10, 0);
let stopped = 0; b.stop = () => stopped++;
assert.ok(a && b && c);
assert.equal(lim.acquire(0, 10, 0), null, 'lowest priority newcomer is dropped');
assert.ok(lim.acquire(4, 10, 0)); assert.equal(stopped, 1, 'lowest-priority voice was stolen');
assert.equal(lim.size(0), 3); assert.equal(lim.size(11), 0, 'ended voices free their slot');
assert.equal(MAX_VOICES, 24);

const voices = [{ lang: 'de-DE', localService: false }, { lang: 'de-DE', localService: true, name: 'a' }, { lang: 'en-US', localService: true }];
assert.equal(pickVoice(voices, 'de').name, 'a'); assert.equal(pickVoice(voices.slice(0, 1), 'de'), null, 'remote voices are never used'); assert.equal(pickVoice(voices, 'fr'), null);
// Sample map: every take/loop references a shipped, manifest-recorded CC0 recording and a trim inside it.
const manifest = JSON.parse(fs.readFileSync('ui/3d/pinball/v1/manifest.json', 'utf8'));
assert.deepEqual(manifest.audio.map(a => a.path).sort(), SAMPLE_NAMES.map(n => 'audio/' + n + '.ogg').sort(), 'manifest audio = sample map');
const seconds = Object.fromEntries(manifest.audio.map(a => [a.path.slice(6, -4), a.seconds]));
for (const a of manifest.audio) { assert.equal(a.license, 'CC0-1.0'); assert.ok(['minigolf', 'aura-drift'].includes(a.pack), a.path + ' pack'); assert.equal(SAMPLE_FILES[a.path.slice(6, -4)].pack, a.pack); assert.ok(manifest.sources.some(x => x.id === a.source && x.license === 'CC0-1.0'), a.path + ' source'); }
for (const [role, list] of Object.entries(TAKES)) {
    assert.ok(list.length > 0, role);
    for (const t of list) { assert.ok(SAMPLE_FILES[t.s], role + ': unknown sample ' + t.s); assert.ok(t.off >= 0 && t.g > 0, role); if (t.dur !== undefined) assert.ok(t.dur > .02 && t.off + t.dur <= seconds[t.s] + .01, role + ': trim outside ' + t.s); }
}
for (const [kind, l] of Object.entries(LOOPS)) { assert.ok(seconds[l.s], kind); assert.ok(l.off + l.dur <= seconds[l.s] + .01 && l.xf * 2 <= l.dur, kind + ' loop region'); }
for (const n of ['hard', 'soft', 'scrape']) assert.ok(LOOPS[n]);

// Every recipe runs against a recording kit; every take role it asks for exists, and nothing rings as a pure high sine.
for (const [name, def] of Object.entries(RECIPES)) {
    const calls = [], rec = new Proxy({}, { get: (_, fn) => (...a) => { calls.push([fn, ...a]); return fn === 'take' ? true : undefined; } });
    def.run(rec, 0, 1, {});
    for (const c of calls) {
        if (c[0] === 'take') assert.ok(TAKES[c[1]], name + ': missing take role ' + c[1]);
        if (c[0] === 'osc' && c[1] === 'sine' && ['flipperUp', 'bumper', 'sling', 'drop', 'scoop', 'plungerRelease', 'eject', 'kickback'].includes(name)) assert.ok(c[2] <= 130, name + ': high sine ring at ' + c[2]);
    }
    if (def.duck !== undefined) assert.ok(def.duck < 0 && def.duck >= -12, name + ' duck');
}
for (const name of ['flipperUp', 'bumper', 'sling', 'drop', 'knocker', 'plungerRelease', 'scoop']) assert.ok(RECIPES[name], name);

// Seamless loop: length shrinks by the crossfade and the wrap point is continuous for a steady tone.
const sr = 8000, tone = Float32Array.from({ length: sr * 2 }, (_, i) => Math.sin(2 * Math.PI * 100 * i / sr));
const loop = makeLoop(tone, sr, .1, 1.5, .2);
assert.equal(loop.length, Math.floor(1.5 * sr) - Math.floor(.2 * sr));
assert.ok(Math.abs(loop[0] - loop[loop.length - 1]) < .25, 'loop seam');
const curve = ceilingCurve();
assert.ok(Math.max(...curve) <= CEILING + 1e-6 && Math.min(...curve) >= -CEILING - 1e-6 && curve[1024] === 0, 'ceiling curve');
for (let i = 1; i < curve.length; i++) assert.ok(curve[i] >= curve[i - 1], 'ceiling curve monotonic');
assert.ok(busGain('effects', .65) > busGain('music', .65), 'effects bus is trimmed above music'); assert.equal(busGain('effects', 0), 0);
console.log('pinball audio ok');
