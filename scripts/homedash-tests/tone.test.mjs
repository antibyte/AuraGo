import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const T = () => loadEngine(['hd-core.js', 'hd-format.js', 'hd-tone.js']).E.tone;
const view = (value, extra) => Object.assign({ id: 'system:cpu', kind: 'sensor', value, available: true, status: 'ok' }, extra);
export const tests = [
  ['tone: rising thresholds give warn and crit, boundaries included', () => { const t = T(), o = { warn: 80, alert: 95 };
    assert.equal(t.of(view(79.9), o), null); assert.equal(t.of(view(80), o), 'warn'); assert.equal(t.of(view(94.9), o), 'warn'); assert.equal(t.of(view(95), o), 'crit');
    assert.equal(t.of(view('96.5'), o), 'crit'); assert.equal(t.of(view(99), { alert: 95 }), 'crit'); assert.equal(t.of(view(90), { alert: 95 }), null);
    assert.equal(t.of(view(81), { warn: 80 }), 'warn'); assert.equal(t.of(view(90), { warn: 90, alert: 90 }), 'crit'); assert.equal(t.direction({ warn: 90, alert: 90 }), 'rising'); }],
  ['tone: warn above alert means falling (low battery)', () => { const t = T(), o = { warn: 30, alert: 15 };
    assert.equal(t.direction(o), 'falling'); assert.equal(t.of(view(31), o), null); assert.equal(t.of(view(30), o), 'warn'); assert.equal(t.of(view(15), o), 'crit');
    assert.equal(t.of(view(2), o), 'crit'); assert.equal(t.of(view(5), { warn: 30, alert: 15, min: 100, max: 0 }), 'crit'); }],
  ['tone: no tone without thresholds, for text, unavailable and unknown entities', () => { const t = T(), o = { warn: 80, alert: 95 };
    assert.equal(t.of(view(99), {}), null); assert.equal(t.of(view('on'), o), null); assert.equal(t.of(view(null), o), null);
    assert.equal(t.of(view(99, { available: false }), o), null); assert.equal(t.of(view(99, { status: 'unknown' }), o), null); assert.equal(t.of(null, o), null); }],
  ['tone: safety binary sensors are crit while on, other classes never', () => { const t = T();
    for (const dc of ['smoke', 'gas', 'moisture', 'problem', 'safety', 'carbon_monoxide']) {
      assert.equal(t.of(view('on', { kind: 'binary_sensor', device_class: dc })), 'crit', dc); assert.equal(t.of(view(true, { kind: 'binary_sensor', device_class: dc })), 'crit', dc);
      assert.equal(t.of(view('off', { kind: 'binary_sensor', device_class: dc })), null, dc); }
    assert.equal(t.of(view('on', { kind: 'binary_sensor', device_class: 'door' })), null);
    assert.equal(t.of(view('on', { kind: 'binary_sensor', device_class: 'smoke', available: false })), null); }],
  ['tone: gauge zones follow the thresholds inside the range', () => { const t = T();
    eq(t.zones({ min: 0, max: 100, warn: 80, alert: 95 }), [{ from: 80, to: 95, tone: 'warn' }, { from: 95, to: 100, tone: 'crit' }]);
    eq(t.zones({ min: 0, max: 100, warn: 30, alert: 15 }), [{ from: 0, to: 15, tone: 'crit' }, { from: 15, to: 30, tone: 'warn' }]);
    eq(t.zones({ min: 0, max: 100, warn: 80 }), [{ from: 80, to: 100, tone: 'warn' }]); eq(t.zones({ min: 0, max: 50, alert: 95 }), []); eq(t.zones({}), []); }],
  ['icons: the M2 host glyphs exist', () => { const E = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true }).E;
    for (const name of ['arrow-left', 'trash', 'dashboard', 'signal-off', 'info', 'refresh']) { assert.ok(E.icons.has(name), name); assert.ok(E.icons.icon(name).querySelectorAll('path').length > 0, name); } }]
];
