import assert from 'node:assert/strict'; import { loadEngine, fc } from './harness.mjs';
const F = () => loadEngine(['hd-core.js', 'hd-format.js']).E.format;
const DASH = fc(0x2013), NBSP = fc(0xa0), DEG = fc(0xb0);
export const tests = [
  ['number and split are locale-correct and tabular-ready', () => { const f = F();
    assert.equal(f.number(1234.5, { decimals: 1, lang: 'en' }), '1,234.5'); assert.equal(f.number(21.53), '21.5'); assert.equal(f.number(150.7), '151'); assert.equal(f.number(5), '5'); assert.equal(f.number(null), '');
    assert.deepEqual({ ...f.split(1234.5, { decimals: 1, lang: 'en', unit: 'W' }) }, { int: '1,234', frac: '.5', unit: 'W', empty: false });
    assert.deepEqual({ ...f.split(1234.5, { decimals: 1, lang: 'de' }) }, { int: '1.234', frac: ',5', unit: '', empty: false });
    assert.equal(f.split(-3.2, { decimals: 1 }).int, '-3'); assert.deepEqual({ ...f.split(null, { unit: '%' }) }, { int: DASH, frac: '', unit: '%', empty: true }); }],
  ['valueText, percent, relTime, duration', () => { const f = F();
    assert.equal(f.valueText(21.5, { unit: DEG + 'C' }), '21.5' + NBSP + DEG + 'C'); assert.equal(f.valueText(null), DASH);
    assert.equal(f.valueText(4000, { decimals: 0, grouping: false, unit: 'K' }), '4000' + NBSP + 'K'); assert.equal(f.percent(0.5, { lang: 'en' }), '50%');
    const now = 1_800_000_000_000; assert.equal(f.relTime(now - 120000, now, 'en'), '2 minutes ago'); assert.equal(f.relTime(now, now, 'en'), 'now');
    const u = (unit, n) => new Intl.NumberFormat('en', { style: 'unit', unit, unitDisplay: 'narrow' }).format(n);
    assert.equal(f.duration(3725, 'en'), u('hour', 1) + ' ' + u('minute', 2)); assert.equal(f.duration(0, 'en'), u('second', 0)); }],
  ['isOn, stateWord, stateText', () => { const f = F();
    assert.equal(f.isOn({ value: 'on' }), true); assert.equal(f.isOn({ value: 'closed' }), false); assert.equal(f.isOn({ value: 21 }), null);
    const w = v => f.stateWord(v);
    assert.equal(w(null), 'unavailable'); assert.equal(w({ available: false }), 'unavailable'); assert.equal(w({ status: 'unknown' }), 'unknown');
    assert.equal(w({ status: 'error', value: 'on' }), 'error'); assert.equal(w({ pending: true, value: 'on' }), 'pending'); assert.equal(w({ value: 'off' }), 'off'); assert.equal(w({ value: 3 }), 'value');
    assert.equal(f.stateText({ value: 'on' }, k => '[' + k + ']'), '[homedash.state.on]'); assert.equal(f.stateText({ value: 3 }, k => k), ''); }]
];
