import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

// Pure math of the Synth Studio read-outs: master dB label, LED meter, EQ and
// filter response curves, envelope shape. The DOM painting is covered by the browser test.
const runtime = { console };
runtime.globalThis = runtime;
runtime.window = runtime;
vm.runInNewContext(fs.readFileSync('ui/js/desktop/apps/synth-studio-curves.js', 'utf8'), runtime, { filename: 'synth-studio-curves.js' });
const C = runtime.SynthStudioCurves;
assert.ok(C, 'module exports its namespace');
const near = (a, b, eps, message) => assert.ok(Math.abs(a - b) <= eps, `${message}: ${a} vs ${b}`);

// dB label
assert.equal(C.formatDb(0), '-inf');
assert.equal(C.formatDb(-1), '-inf');
assert.equal(C.formatDb(NaN), '-inf');
assert.equal(C.formatDb(1), '0.0 dB');
assert.equal(C.formatDb(0.5), '-6.0 dB');
assert.equal(C.formatDb(0.78), '-2.2 dB');
assert.equal(C.formatDb(1.5), '+3.5 dB');
assert.equal(C.gainToDb(0.1), -20);

// LED meter
assert.equal(C.meterLit(0), 0);
assert.equal(C.meterLit(0.001), 0, 'below the -48 dB floor stays dark');
assert.equal(C.meterLit(1), 16, 'full scale lights every segment');
assert.equal(C.meterLit(2), 16, 'clipping stays clamped');
assert.equal(C.meterLit(0.5), 14, '-6 dB sits at segment 14');
assert.ok(C.meterLit(0.1) < C.meterLit(0.5), 'louder lights more');
assert.equal(C.meterFollow(10, 14), 14, 'rises at once');
near(C.meterFollow(14, 2), 13.65, 1e-9, 'falls slowly');
assert.equal(C.meterFollow(0.2, 0), 0, 'never below target');
const tones = Array.from({ length: 16 }, (_, i) => C.segmentTone(i, 16)).join('');
assert.equal(tones, 'ggggggggggaaaarr', 'green, amber, red');
assert.equal((C.meterMarkup('x').match(/ss-led-seg/g) || []).length, 16);

// Filter coefficients: a lowpass passes DC and rejects the top octave
const lp = C.biquad('lowpass', 1000, 0, 0.7);
near(C.magnitudeDb(lp, 20), 0, 0.2, 'lowpass passband');
assert.ok(C.magnitudeDb(lp, 10000) < -30, 'lowpass stopband');
near(C.magnitudeDb(lp, 1000), 0.7, 0.2, 'lowpass resonance Q is in dB, so Q 0.7 peaks +0.7 dB at the cutoff');
const hp = C.biquad('highpass', 1000, 0, 0.7);
assert.ok(C.magnitudeDb(hp, 40) < -30 && Math.abs(C.magnitudeDb(hp, 15000)) < 0.5, 'highpass');
const bp = C.biquad('bandpass', 1000, 0, 2);
near(C.magnitudeDb(bp, 1000), 0, 0.1, 'bandpass peaks at 0 dB');
assert.ok(C.magnitudeDb(bp, 100) < -15);
const notch = C.biquad('notch', 1000, 0, 2);
assert.ok(C.magnitudeDb(notch, 1000) < -40 && Math.abs(C.magnitudeDb(notch, 100)) < 1, 'notch');
near(C.magnitudeDb(C.biquad('peaking', 1000, 6, 0.9), 1000), 6, 0.05, 'peaking hits its gain');
near(C.magnitudeDb(C.biquad('lowshelf', 110, 9, 0), 20), 9, 0.3, 'lowshelf boosts below');
near(C.magnitudeDb(C.biquad('lowshelf', 110, 9, 0), 10000), 0, 0.1, 'lowshelf leaves the top');
near(C.magnitudeDb(C.biquad('highshelf', 5000, -9, 0), 20000), -9, 0.5, 'highshelf cuts above');

// Master EQ
const flat = C.eqResponse(0, 0, 0);
assert.equal(flat.length, 96);
assert.ok(flat.every(v => Math.abs(v) < 1e-6), 'flat EQ is a flat line');
const eq = C.eqResponse(12, -6, 6);
near(eq[0], 12, 0.6, 'low shelf shows at 20 Hz');
near(eq[eq.length - 1], 6, 0.6, 'high shelf shows at 20 kHz');
const mid = C.frequencies(96).findIndex(f => f >= 1000);
assert.ok(eq[mid] < 0, 'mid cut shows around 1 kHz');
assert.notDeepEqual(C.eqResponse(3, 0, 0), C.eqResponse(-3, 0, 0), 'response follows the gains');
assert.deepEqual(C.eqResponse(99, 0, 0), C.eqResponse(18, 0, 0), 'gains clamp to the engine range');

// Filter response and cutoff mapping
near(C.cutoffHz(0.05), 180 + Math.pow(0.05, 1.7) * 9500, 1e-6, 'cutoff mapping low end');
near(C.cutoffHz(1), 9680, 1e-6, 'cutoff mapping high end');
assert.ok(C.cutoffHz(0.3) < C.cutoffHz(0.7), 'cutoff is monotone');
const fr = C.filterResponse('lowpass', 0.65, 1);
assert.ok(fr[0] > -1 && fr[fr.length - 1] < -25, 'lowpass curve falls');
const frq = C.filterResponse('lowpass', 0.65, 12);
assert.ok(Math.max(...frq) > Math.max(...fr) + 6, 'resonance raises the peak');
const frh = C.filterResponse('highpass', 0.65, 1);
assert.ok(frh[0] < -25 && frh[frh.length - 1] > -1, 'highpass curve rises');
assert.deepEqual(C.filterResponse('nonsense', 0.65, 1), fr, 'unknown type falls back to lowpass');

// Path geometry
const paths = C.eqPaths(0, 0, 0);
assert.ok(paths.line.startsWith('M0.0 45.0L') && paths.fill.endsWith('Z'), 'flat EQ runs through the middle');
assert.equal(C.eqPaths(6, 0, 0).line === paths.line, false);
assert.ok(!/NaN|Infinity/.test(C.filterPaths('notch', 0.2, 15).line), 'paths are finite');
near(C.freqX(20), 0, 1e-9, 'x axis starts at 20 Hz');
near(C.freqX(20000), C.VIEW_W, 1e-9, 'x axis ends at 20 kHz');

// Envelope shape
const pts = C.adsrPoints(0.01, 0.25, 0.5, 0.2);
assert.equal(pts.length, 5);
assert.equal(pts[0][0], 0);
near(pts[4][0], C.VIEW_W, 1e-9, 'envelope fills the width');
assert.ok(pts[1][1] < pts[2][1], 'decay falls from the peak to the sustain level');
assert.equal(pts[2][1], pts[3][1], 'sustain is flat');
assert.equal(pts[0][1], pts[4][1], 'envelope starts and ends silent');
assert.ok(C.adsrPoints(2, 0.25, 0.5, 0.2)[0][0] === 0 && C.adsrPoints(2, 0.25, 0.5, 0.2)[1][0] > pts[1][0], 'longer attack stretches the first segment');
assert.ok(C.adsrPoints(0.01, 0.25, 0.9, 0.2)[2][1] < pts[2][1], 'higher sustain sits higher');
assert.ok(!/NaN/.test(C.adsrPaths(NaN, undefined, null, -1).line), 'bad input stays finite');
console.log('synth-studio curves ok');
