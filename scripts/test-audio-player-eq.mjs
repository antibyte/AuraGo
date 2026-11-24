import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function loadBrowserModule(relPath) {
  const code = readFileSync(relPath, 'utf8');
  const mod = { exports: {} };
  const sandbox = {
    window: null,
    globalThis: null,
    module: mod,
    exports: mod.exports,
    console,
    Math,
    Date,
    Number,
    String,
    Array,
    Object,
    Map,
    Set,
    JSON,
    Float32Array,
    Uint8Array
  };
  sandbox.window = sandbox;
  sandbox.globalThis = sandbox;
  vm.runInNewContext(code, sandbox, { filename: relPath });
  return sandbox;
}

const eqEnv = loadBrowserModule('ui/js/desktop/apps/audio-player-eq.js');
const vizEnv = loadBrowserModule('ui/js/desktop/apps/audio-player-visualizer.js');
const eq = eqEnv.AudioPlayerEQ;
const viz = vizEnv.AudioPlayerVisualizer;

function makeMemoryStorage() {
  const map = new Map();
  return {
    getItem(key) { return map.has(key) ? map.get(key) : null; },
    setItem(key, val) { map.set(key, String(val)); },
    removeItem(key) { map.delete(key); }
  };
}

// 1. Visualizer modes contract
assert.deepEqual([...viz.MODES], ['bars', 'radial', 'wave', 'nebula', 'horizon']);

// 2. EQ band & preset invariants
assert.equal(eq.BANDS.length, 8);
assert.equal(eq.PRESETS.length, 10);
assert.deepEqual(
  Array.from(eq.BANDS, b => b.freq),
  [60, 170, 350, 1000, 2400, 6000, 12000, 16000]
);
for (const preset of eq.PRESETS) {
  assert.equal(preset.bands.length, 8);
  assert.ok(preset.id.startsWith('preset:'));
}

// 3. Clamping & 0.5 dB snapping
assert.deepEqual(
  Array.from(eq.normalizeBands([99, -99, 3.24, -2.76, NaN, '4.5', null, 0.25])),
  [12, -12, 3, -3, 0, 4.5, 0, 0.5]
);

// 4. RBJ transfer curve accuracy
const flatCurve = eq.computeCurve([0, 0, 0, 0, 0, 0, 0, 0], true, 480, 68, 64);
assert.equal(flatCurve.points.length, 65);
assert.equal(flatCurve.bandDots.length, 8);
assert.ok(Math.abs(eq.responseDbAt(60, [0, 0, 0, 0, 0, 0, 0, 0], true)) < 1e-6);

const bassBoost = eq.PRESETS.find(p => p.key === 'bass_boost').bands;
const lowDb = eq.responseDbAt(60, bassBoost, true);
const midDb = eq.responseDbAt(1000, bassBoost, true);
const bypassedLowDb = eq.responseDbAt(60, bassBoost, false);
assert.ok(lowDb > 5.0 && lowDb < 12.0, `Expected 60Hz bass boost > 5 dB, got ${lowDb}`);
assert.ok(Math.abs(midDb) < 1.5, `Expected 1kHz near flat, got ${midDb}`);
assert.equal(bypassedLowDb, 0);

// 5. Store persistence & profile CRUD
const storage = makeMemoryStorage();
const store = eq.createStore(storage);
assert.equal(store.snapshot().activeId, 'preset:flat');
assert.equal(store.isEffective(), false);

assert.equal(store.selectProfile('preset:electronic'), true);
assert.equal(store.snapshot().activeId, 'preset:electronic');
assert.equal(store.isEffective(), true);

// Reject empty name
assert.equal(store.saveProfile('   ').ok, false);
assert.equal(store.saveProfile('   ').reason, 'empty_name');

// Adjust band -> becomes custom
store.setBand(3, 4.5);
assert.equal(store.snapshot().activeId, 'custom');
assert.equal(store.snapshot().bands[3], 4.5);

// Save custom profile & overwrite case-insensitively
const saved = store.saveProfile('  Studio Night  ');
assert.equal(saved.ok, true);
assert.equal(saved.updated, false);
assert.equal(saved.profile.name, 'Studio Night');
assert.equal(store.snapshot().activeId, saved.profile.id);

store.setBand(0, 7.5);
const updated = store.saveProfile('studio night');
assert.equal(updated.ok, true);
assert.equal(updated.updated, true);
assert.equal(updated.profile.id, saved.profile.id);
assert.equal(updated.profile.bands[0], 7.5);

// Reload from same storage verifies persistence
const reloaded = eq.createStore(storage);
assert.equal(reloaded.snapshot().profiles.length, 1);
assert.equal(reloaded.snapshot().activeId, saved.profile.id);
assert.equal(reloaded.snapshot().bands[0], 7.5);

// Delete profile keeps current curve as custom & removes only that user profile
assert.equal(reloaded.deleteProfile('preset:rock').ok, false);
assert.equal(reloaded.deleteProfile(saved.profile.id).ok, true);
assert.equal(reloaded.snapshot().profiles.length, 0);
assert.equal(reloaded.snapshot().activeId, 'custom');

// Limit enforcement
for (let i = 0; i < eq.MAX_PROFILES; i++) {
  const r = reloaded.saveProfile(`Profile ${i + 1}`);
  assert.equal(r.ok, true);
}
assert.equal(reloaded.saveProfile('Overflow').ok, false);
assert.equal(reloaded.saveProfile('Overflow').reason, 'limit_reached');

// Corrupted JSON recovery
storage.setItem(eq.STORAGE_KEY, '{corrupt json');
const recovered = eq.loadState(storage);
assert.equal(recovered.activeId, 'preset:flat');
assert.deepEqual(Array.from(recovered.bands), [0, 0, 0, 0, 0, 0, 0, 0]);

// 6. Web Audio graph chain builder & bypass application
const mockCalls = [];
const makeParam = (initial = 0) => ({
  value: initial,
  cancelScheduledValues(t) { mockCalls.push(['cancel', t]); },
  setTargetAtTime(v, t, c) { this.value = v; mockCalls.push(['target', v, t, c]); },
  setValueAtTime(v, t) { this.value = v; mockCalls.push(['value', v, t]); }
});
const mockCtx = {
  currentTime: 2.5,
  createBiquadFilter() {
    return {
      type: 'peaking',
      frequency: { value: 0 },
      Q: { value: 1 },
      gain: makeParam(0),
      connect() {}
    };
  },
  createDynamicsCompressor() {
    return {
      threshold: { value: 0 },
      knee: { value: 0 },
      ratio: { value: 1 },
      attack: { value: 0 },
      release: { value: 0 },
      connect() {}
    };
  }
};
const chain = eq.buildChain(mockCtx);
assert.equal(chain.filters.length, 8);
assert.ok(chain.limiter);
eq.applyToChain(chain, mockCtx, { enabled: true, bands: [6, 3, 0, -2, 1, 2, 4, 5] }, false);
assert.deepEqual(Array.from(chain.filters, f => f.gain.value), [6, 3, 0, -2, 1, 2, 4, 5]);
eq.applyToChain(chain, mockCtx, { enabled: false, bands: [6, 3, 0, -2, 1, 2, 4, 5] }, true);
assert.deepEqual(Array.from(chain.filters, f => f.gain.value), [0, 0, 0, 0, 0, 0, 0, 0]);

console.log('OK: AudioPlayerEQ and 5-mode visualizer assertions passed.');
