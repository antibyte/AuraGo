import assert from 'node:assert/strict';
import { loadDesktopSounds } from './desktop-sound-loader.mjs';
const { DSP: D } = loadDesktopSounds({ themes: 'none' });
const SR = D.SR;
const sine = (f, sec, amp, phase = 0) => Float32Array.from({ length: Math.round(sec * SR) }, (_, i) => amp * Math.sin(2 * Math.PI * f * i / SR + phase));
const rms = a => Math.sqrt(a.reduce((s, x) => s + x * x, 0) / a.length);
const peakOf = a => a.reduce((m, x) => Math.max(m, Math.abs(x)), 0);
const cents = (f, ref) => 1200 * Math.log2(f / ref);
function pitch(buf, expect) { // autocorrelation around the expected period, parabolic refinement
  const start = Math.round(0.02 * SR), len = Math.round(0.06 * SR);
  const lo = Math.floor(SR / (expect * 1.25)), hi = Math.ceil(SR / (expect / 1.25));
  const r = l => { let s = 0; for (let i = start; i < start + len; i++) s += buf[i] * buf[i + l]; return s; };
  let best = lo; for (let l = lo; l <= hi; l++) if (r(l) > r(best)) best = l;
  const a = r(best - 1), b = r(best), c = r(best + 1);
  return SR / (best + 0.5 * (a - c) / (a - 2 * b + c));
}
// random generator and seeds
{ const a = D.makeRng(7), b = D.makeRng(7); for (let i = 0; i < 1000; i++) { const x = a(); assert.equal(x, b()); assert.ok(x >= 0 && x < 1); } }
assert.equal(D.hashSeed('crystal', 'window.open', '0'), D.hashSeed('crystal', 'window.open', '0'));
assert.notEqual(D.hashSeed('crystal', 'window.open', '0'), D.hashSeed('crystal', 'window.open', '1'));
// envelope starts at zero
{ const a = new Float32Array(480).fill(1); D.applyEnv(a, { attack: 0.004, decay: 0.2 }); assert.equal(a[0], 0); }
// modal partial: pitch, and -60 dB at t60
{ const { a } = D.modal({ freq: 1000, dur: 0.6, t60: 0.5, damp: 0, attack: 0.001, partials: [{ r: 1, g: 1 }] }, D.makeRng(1));
  assert.ok(Math.abs(cents(pitch(a, 1000), 1000)) < 3, 'modal pitch');
  const drop = 20 * Math.log10(peakOf(a.subarray(Math.round(0.49 * SR), Math.round(0.51 * SR))) / peakOf(a.subarray(0, Math.round(0.02 * SR))));
  assert.ok(drop > -61.5 && drop < -58.5, 'modal decay ' + drop); }
// plucked string: tuned within 5 cents, no DC
for (const f of [220, 440, 1000]) { const s = D.pluck({ freq: f, dur: 0.4, t60: 0.8, brightness: 0.5 }, D.makeRng(3));
  assert.ok(Math.abs(cents(pitch(s, f), f)) < 5, 'pluck pitch ' + f);
  assert.ok(Math.abs(s.reduce((x, y) => x + y, 0) / s.length) < 1e-3, 'pluck DC'); }
// filters
{ const x = sine(1000, 0.5, 0.5); D.svf(x, { type: 'lp', cutoff: 1000, q: Math.SQRT1_2 });
  const g = 20 * Math.log10(rms(x.subarray(4800)) / (0.5 / Math.SQRT2)); assert.ok(Math.abs(g + 3.01) < 0.5, 'svf -3 dB ' + g);
  assert.throws(() => D.svf(new Float32Array(8), { type: 'notch?', cutoff: 1000 })); }
{ const lo = sine(55, 1, 0.5), hi = sine(1000, 1, 0.5);
  for (const b of [lo, hi]) { D.highpass(b, 110, 0.5412); D.highpass(b, 110, 1.3066); }
  assert.ok(20 * Math.log10(rms(lo.subarray(SR / 2)) / (0.5 / Math.SQRT2)) < -20, '4th order low cut at 55 Hz');
  assert.ok(Math.abs(20 * Math.log10(rms(hi.subarray(SR / 2)) / (0.5 / Math.SQRT2))) < 0.1, 'low cut passes 1 kHz'); }
// BS.1770: a stereo 1 kHz sine of amplitude 0.1 reads -20 LUFS
{ const s = sine(1000, 1, 0.1); assert.ok(Math.abs(D.loudness({ L: s, R: s, n: s.length }) + 20) < 0.2); }
// true peak finds the inter-sample peak of a 12 kHz sine sampled at +-45 degrees
{ const s = sine(12000, 0.1, 0.5, Math.PI / 4); assert.ok(peakOf(s) < 0.36);
  assert.ok(Math.abs(20 * Math.log10(D.truePeak({ L: s, R: s, n: s.length }) / 0.5)) < 0.25, 'true peak'); }
// master: loudness target, zero ends, length cap
{ const make = () => { const m = D.createMix(1.2); const { a } = D.modal({ freq: 660, dur: 1.2, t60: 1, partials: [{ r: 1, g: 1 }] }, D.makeRng(2)); D.addMono(m, a, 0, 1, 0); return m; };
  const out = D.master(make(), { lufs: -23, ceiling: -3, maxSeconds: 2 });
  assert.ok(out.limited || Math.abs(D.loudness(out.mix) + 23) < 0.05);
  assert.equal(out.mix.L[0], 0); assert.equal(out.mix.R[out.mix.n - 1], 0);
  const capped = D.master(make(), { lufs: -23, ceiling: -3, maxSeconds: 0.2 });
  assert.ok(capped.seconds <= 0.2 + 1e-9 && capped.naturalSeconds > 0.2); }
// reverb and echo extend the sound, deterministically
{ const m = D.createMix(0.1); m.L[10] = m.R[10] = 1;
  const r1 = D.reverb(m, { size: 1, rt60: 0.5, damping: 6000, mix: 0.3 }), r2 = D.reverb(m, { size: 1, rt60: 0.5, damping: 6000, mix: 0.3 });
  assert.ok(r1.n >= m.n + 0.5 * SR); assert.deepEqual(r1.L, r2.L);
  const e = D.echo(m, { time: 0.1, feedback: 0.5, tone: 4000, mix: 0.3 }); assert.ok(e.n > m.n + 0.3 * SR); }
// noise colours are finite and seeded
for (const c of ['white', 'pink', 'brown']) { const a = D.noise(0.1, c, D.makeRng(5)), b = D.noise(0.1, c, D.makeRng(5));
  assert.deepEqual(a, b); assert.ok(a.every(Number.isFinite) && rms(a) > 0.01); }
console.log('desktop sound DSP kernel: ok');
