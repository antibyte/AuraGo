import assert from 'node:assert/strict';
import { loadDesktopSounds } from './desktop-sound-loader.mjs';
const { DSP: D, Instruments: I } = loadDesktopSounds({ themes: 'none' });
const SR = D.SR;
const NAMES = ['glass', 'bar', 'bell', 'string', 'analog', 'chip', 'drop', 'blip', 'pad', 'metal', 'click', 'thud', 'noise', 'squelch', 'debris'];
assert.ok(I, 'DesktopSoundInstruments is defined');
assert.deepEqual(Object.keys(I.MODELS).sort(), NAMES.slice().sort());
const TONAL = ['glass', 'bar', 'bell', 'string', 'analog', 'chip', 'blip'];
// pitch of the typical default note per model (level calibration)
const TYPICAL = { glass: 880, bell: 880, thud: 220, squelch: 220 };
function pitch(buf, expect) { // autocorrelation around the expected period, parabolic refinement
  const start = Math.round(0.02 * SR), len = Math.round(0.06 * SR);
  const lo = Math.floor(SR / (expect * 1.25)), hi = Math.ceil(SR / (expect / 1.25));
  const r = l => { let s = 0; for (let i = start; i < start + len; i++) s += buf[i] * buf[i + l]; return s; };
  let best = lo; for (let l = lo; l <= hi; l++) if (r(l) > r(best)) best = l;
  const a = r(best - 1), b = r(best), c = r(best + 1);
  return SR / (best + 0.5 * (a - c) / (a - 2 * b + c));
}
const peakOf = a => a.reduce((m, x) => Math.max(m, Math.abs(x)), 0);
const db = x => 20 * Math.log10(Math.max(x, 1e-12));
const fp = o => { let h = 0x811c9dc5; for (const ch of [o.L, o.R]) { const b = new Uint8Array(ch.buffer, ch.byteOffset, ch.byteLength); for (let i = 0; i < b.length; i++) h = Math.imul(h ^ b[i], 0x01000193) >>> 0; } return h; };
for (const name of NAMES) {
  const m = I.MODELS[name];
  for (const [k, s] of Object.entries(m.params)) { // every default lies inside its own range
    if (Array.isArray(s)) assert.ok(s[2] >= s[0] && s[2] <= s[1], `${name}.${k} default`);
    else if (s.values) assert.ok(s.values.includes(s.def), `${name}.${k} default`);
  }
  const p = I.resolveParams(name, {});
  const note = { freq: 440, dur: Math.min(m.duration(p), 2), gain: 1, pan: 0, at: 0, detune: 0 };
  const a = I.render(name, {}, note, D.makeRng(11)), b = I.render(name, {}, note, D.makeRng(11));
  assert.equal(a.L.length, Math.ceil(note.dur * D.SR), `${name} length`);
  assert.ok(a.L.every(Number.isFinite) && a.R.every(Number.isFinite), `${name} finite`);
  assert.ok(Math.abs(a.L[0]) < 1e-4 && Math.abs(a.R[0]) < 1e-4, `${name} starts at zero`);
  assert.ok(a.L.some(x => Math.abs(x) > 1e-3), `${name} not silent`);
  assert.equal(fp(a), fp(b), `${name} deterministic`);
  if (TONAL.includes(name) && name !== 'bell') { const mono = a.L.map((x, i) => x + a.R[i]); assert.ok(Math.abs(1200 * Math.log2(pitch(mono, 440) / 440)) < 10, `${name} pitch`); }
  // level calibration: a typical default note peaks between -12 and -3 dBFS
  const tn = { freq: TYPICAL[name] ?? 440, dur: m.duration(p), gain: 1, pan: 0, at: 0, detune: 0 };
  const t = I.render(name, {}, tn, D.makeRng(5));
  const peakDb = db(Math.max(peakOf(t.L), peakOf(t.R)));
  assert.ok(peakDb >= -12 && peakDb <= -3, `${name} default peak ${peakDb.toFixed(2)} dBFS`);
  // no clicks: the last 2 ms of a default-duration render are below -50 dB relative to the peak
  const tail = Math.round(0.002 * SR);
  const tailPeak = Math.max(peakOf(t.L.subarray(t.L.length - tail)), peakOf(t.R.subarray(t.R.length - tail)));
  assert.ok(db(tailPeak) - peakDb < -50, `${name} tail ${(db(tailPeak) - peakDb).toFixed(1)} dB`);
  // a note shorter than the natural duration is faded out at note.dur
  const sn = { freq: TYPICAL[name] ?? 440, dur: Math.max(0.03, m.duration(p) * 0.5), gain: 1, pan: 0, at: 0, detune: 0 };
  const sh = I.render(name, {}, sn, D.makeRng(5));
  const shTail = Math.max(peakOf(sh.L.subarray(sh.L.length - tail)), peakOf(sh.R.subarray(sh.R.length - tail)));
  const fin = Math.round(0.0002 * SR); // the 5 ms raised-cosine fade leaves the last 0.2 ms below -40 dB
  const shEnd = Math.max(peakOf(sh.L.subarray(sh.L.length - fin)), peakOf(sh.R.subarray(sh.R.length - fin)));
  assert.ok(db(shEnd) - db(Math.max(peakOf(sh.L), peakOf(sh.R))) < -40, `${name} short note tail`);
}
// bell: the default kind has half-integer FM ratios (3.5, 1.41), so autocorrelation cannot track it; check the
// carrier alone and the harmonic kinds instead
for (const params of [{ index: 0, air: 0 }, { kind: 'celesta' }, { kind: 'tine' }]) {
  const o = I.render('bell', params, { freq: 440, dur: 0.5, gain: 1, pan: 0, at: 0, detune: 0 }, D.makeRng(11));
  assert.ok(Math.abs(1200 * Math.log2(pitch(o.L.map((x, i) => x + o.R[i]), 440) / 440)) < 10, 'bell pitch ' + JSON.stringify(params));
}
// every enum value of every preset resolves and renders
for (const name of NAMES) {
  const m = I.MODELS[name];
  for (const [k, s] of Object.entries(m.params)) {
    if (Array.isArray(s) || !s.values) continue;
    for (const v of s.values) {
      const p = I.resolveParams(name, { [k]: v });
      const o = I.render(name, { [k]: v }, { freq: 440, dur: Math.min(m.duration(p), 1.5), gain: 1, pan: 0, at: 0, detune: 0 }, D.makeRng(2));
      assert.ok(o.L.every(Number.isFinite) && o.R.every(Number.isFinite) && peakOf(o.L) + peakOf(o.R) > 1e-3, `${name}.${v} renders`);
    }
  }
}
// pan: hard left puts the energy on the left for panned mono models
{ const o = I.render('thud', {}, { freq: 220, dur: 0.2, gain: 1, pan: -1, at: 0, detune: 0 }, D.makeRng(1));
  assert.ok(peakOf(o.L) > 10 * peakOf(o.R), 'pan left'); }
// gain scales the output
{ const n1 = { freq: 440, dur: 0.2, gain: 1, pan: 0, at: 0, detune: 0 };
  const g1 = I.render('blip', {}, n1, D.makeRng(1)), g2 = I.render('blip', {}, { ...n1, gain: 0.5 }, D.makeRng(1));
  assert.ok(Math.abs(peakOf(g2.L) / peakOf(g1.L) - 0.5) < 1e-3, 'gain'); }
// the glass strike parameter adds a transient
{ const nt = { freq: 880, dur: 0.3, gain: 1, pan: 0, at: 0, detune: 0 };
  const x0 = I.render('glass', { strike: 0 }, nt, D.makeRng(3)), x1 = I.render('glass', { strike: 1 }, nt, D.makeRng(3));
  assert.notEqual(fp(x0), fp(x1), 'glass strike changes the onset'); }
// air bed: differs from no bed, finite, starts at zero
for (const name of ['glass', 'bar', 'bell', 'metal', 'string']) {
  const an = { freq: 440, dur: 0.5, gain: 1, pan: 0, at: 0, detune: 0 };
  const x0 = I.render(name, { air: 0 }, an, D.makeRng(4)), x1 = I.render(name, { air: 0.04 }, an, D.makeRng(4));
  assert.notEqual(fp(x0), fp(x1), name + ' air bed changes the render');
  assert.ok(x1.L.every(Number.isFinite) && x1.R.every(Number.isFinite), name + ' air finite');
  assert.ok(Math.abs(x1.L[0]) < 1e-4 && Math.abs(x1.R[0]) < 1e-4, name + ' air starts at zero');
}
assert.notEqual(fp(I.render('glass', {}, { freq: 880, dur: 0.5, gain: 1, pan: 0, at: 0, detune: 4 }, D.makeRng(1))),
  fp(I.render('glass', {}, { freq: 880, dur: 0.5, gain: 1, pan: 0, at: 0, detune: 0 }, D.makeRng(1))), 'variant detune changes partials');
assert.throws(() => I.render('kazoo', {}, { freq: 440, dur: 0.1, gain: 1, pan: 0, at: 0 }, D.makeRng(1)));
console.log('desktop sound instruments: ok');
