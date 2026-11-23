// Authoring tool: renders desktop sound themes to 24-bit WAV files, a contact sheet and a levels table.
//   node scripts/render-desktop-sounds.mjs --theme crystal --out C:\disposable\desktop-sounds\render [--event window.open] [--variant 0|name|all] [--json]
// Output: <out>/<theme>/<event>.v<n>.wav (effects: <event>.<name>.wav), <theme>-contact-sheet.wav, levels.json.
import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { performance } from 'node:perf_hooks';
import { loadDesktopSounds, repoRoot } from './desktop-sound-loader.mjs';
import { desktopSoundThemeParts } from './desktop-sound-bundle-parts.js';

const args = process.argv.slice(2);
const opt = { theme: '', event: '', variant: 'all', out: '', json: false };
for (let i = 0; i < args.length; i++) {
  const a = args[i];
  if (a === '--theme') opt.theme = String(args[++i] || '');
  else if (a === '--event') opt.event = String(args[++i] || '');
  else if (a === '--variant') opt.variant = String(args[++i] || 'all');
  else if (a === '--out') opt.out = String(args[++i] || '');
  else if (a === '--json') opt.json = true;
  else { console.error('unknown argument ' + a); process.exit(2); }
}
if (!opt.out) { console.error('--out <dir> is required'); process.exit(2); }
const available = Object.keys(desktopSoundThemeParts).filter(id => existsSync(path.join(repoRoot, desktopSoundThemeParts[id])));
const ids = !opt.theme || opt.theme === 'all' ? available : opt.theme.split(',').filter(Boolean);
const { DSP, Engine } = loadDesktopSounds({ themes: ids });
const SR = DSP.SR;

// 24-bit stereo little-endian PCM WAV
function encodeWav(L, R, sampleRate = SR) {
  const n = L.length, data = n * 6, buf = Buffer.alloc(44 + data);
  buf.write('RIFF', 0); buf.writeUInt32LE(36 + data, 4); buf.write('WAVE', 8); buf.write('fmt ', 12);
  buf.writeUInt32LE(16, 16); buf.writeUInt16LE(1, 20); buf.writeUInt16LE(2, 22); buf.writeUInt32LE(sampleRate, 24);
  buf.writeUInt32LE(sampleRate * 6, 28); buf.writeUInt16LE(6, 32); buf.writeUInt16LE(24, 34); buf.write('data', 36); buf.writeUInt32LE(data, 40);
  let o = 44;
  const put = x => { const v = Math.max(-8388608, Math.min(8388607, Math.round(x * 8388607))); buf.writeIntLE(v, o, 3); o += 3; };
  for (let i = 0; i < n; i++) { put(L[i]); put(R[i]); }
  return buf;
}

// radix-2 in-place FFT
function fft(re, im) {
  const n = re.length;
  for (let i = 1, j = 0; i < n; i++) {
    let bit = n >> 1;
    for (; j & bit; bit >>= 1) j ^= bit;
    j ^= bit;
    if (i < j) { [re[i], re[j]] = [re[j], re[i]]; [im[i], im[j]] = [im[j], im[i]]; }
  }
  for (let len = 2; len <= n; len <<= 1) {
    const ang = -2 * Math.PI / len, wr = Math.cos(ang), wi = Math.sin(ang);
    for (let i = 0; i < n; i += len) {
      let cr = 1, ci = 0;
      for (let k = 0; k < len / 2; k++) {
        const a = i + k, b = a + len / 2;
        const tr = re[b] * cr - im[b] * ci, ti = re[b] * ci + im[b] * cr;
        re[b] = re[a] - tr; im[b] = im[a] - ti; re[a] += tr; im[a] += ti;
        const nr = cr * wr - ci * wi; ci = cr * wi + ci * wr; cr = nr;
      }
    }
  }
}
// energy-weighted mean over 2048-sample Hann frames (hop 1024) of sum(f*|X|) / sum(|X|), on (L+R)/2
function centroid(L, R) {
  const N = 2048, hop = 1024, win = new Float64Array(N);
  for (let i = 0; i < N; i++) win[i] = 0.5 - 0.5 * Math.cos(2 * Math.PI * i / (N - 1));
  let num = 0, den = 0;
  const frames = Math.max(1, Math.ceil((L.length - N) / hop) + 1);
  for (let f = 0; f < frames; f++) {
    const re = new Float64Array(N), im = new Float64Array(N);
    let e = 0;
    for (let i = 0; i < N; i++) { const k = f * hop + i; const s = k < L.length ? (L[k] + R[k]) / 2 : 0; re[i] = s * win[i]; e += re[i] * re[i]; }
    if (e <= 0) continue;
    fft(re, im);
    let sf = 0, sm = 0;
    for (let b = 1; b < N / 2; b++) { const m = Math.hypot(re[b], im[b]); sf += b * SR / N * m; sm += m; }
    if (sm > 0) { num += e * sf / sm; den += e; }
  }
  return den > 0 ? num / den : 0;
}
// time from the first sample above -40 dB of the peak to the first 1 ms RMS frame within 3 dB of the loudest 1 ms frame
function attackMs(L, R) {
  const n = L.length;
  let peak = 0;
  for (let i = 0; i < n; i++) peak = Math.max(peak, Math.abs(L[i]), Math.abs(R[i]));
  const thr = peak * Math.pow(10, -40 / 20);
  let first = 0;
  while (first < n && Math.abs(L[first]) <= thr && Math.abs(R[first]) <= thr) first++;
  const w = Math.round(SR / 1000), rms = [];
  for (let s = 0; s + w <= n; s += w) { let e = 0; for (let i = 0; i < w; i++) e += L[s + i] * L[s + i] + R[s + i] * R[s + i]; rms.push(Math.sqrt(e / (2 * w))); }
  const top = Math.max(...rms, 1e-12), lim = top * Math.pow(10, -3 / 20);
  const frame = rms.findIndex(v => v >= lim);
  return Math.max(0, (frame * w - first) / SR * 1000);
}

const rows = [], round = (x, d) => Number(x.toFixed(d));
const list = theme => {
  const events = opt.event ? [opt.event] : Engine.EVENTS.concat(Object.keys(Engine.FX_ROLES).filter(id => Engine.FX_ROLES[id].theme === theme));
  const out = [];
  for (const id of events) {
    const fx = Engine.FX_ROLES[id];
    if (fx && fx.theme !== theme) continue;
    const variants = fx ? fx.names : Array.from({ length: Engine.ROLES[id].variants }, (_, i) => i);
    const pick = opt.variant === 'all' ? variants : variants.filter(v => String(v) === opt.variant);
    for (const v of pick) out.push({ id, v, fx: !!fx, role: fx || Engine.ROLES[id] });
  }
  return out;
};
let totalMs = 0;
for (const theme of Engine.themes()) {
  const dir = path.join(opt.out, theme);
  mkdirSync(dir, { recursive: true });
  const sheet = [], gap = Math.round(0.5 * SR), themeRows = [];
  const items = list(theme);
  let themeMs = 0;
  for (const it of items) {
    const t0 = performance.now();
    const o = Engine.render(theme, it.id, it.v);
    const renderMs = performance.now() - t0;
    themeMs += renderMs;
    const file = it.fx ? `${it.id}.${it.v}.wav` : `${it.id}.v${it.v}.wav`;
    writeFileSync(path.join(dir, file), encodeWav(o.L, o.R, o.sampleRate));
    const row = { theme, event: it.id, variant: it.v, seconds: round(o.seconds, 3), natural: round(o.naturalSeconds, 3), lufs: round(o.lufs, 2), target: it.role.lufs,
      deltaLU: round(o.lufs - it.role.lufs, 2), peakDb: round(o.peakDb, 2), limited: o.limited, centroidHz: Math.round(centroid(o.L, o.R)), attackMs: round(attackMs(o.L, o.R), 1), renderMs: Math.round(renderMs) };
    themeRows.push(row); rows.push(row);
    if ((it.fx || it.v === 0) && opt.variant === 'all' && !opt.event) sheet.push(o);
  }
  totalMs += themeMs;
  if (sheet.length) {
    const total = sheet.reduce((s, o) => s + o.L.length + gap, 0), L = new Float32Array(total), R = new Float32Array(total);
    let at = 0;
    for (const o of sheet) { L.set(o.L, at); R.set(o.R, at); at += o.L.length + gap; }
    writeFileSync(path.join(dir, `${theme}-contact-sheet.wav`), encodeWav(L, R));
  }
  writeFileSync(path.join(dir, 'levels.json'), JSON.stringify({ theme, renderMs: Math.round(themeMs), rows: themeRows }, null, 2));
}
if (opt.json) { console.log(JSON.stringify(rows, null, 2)); console.error('total render time ' + Math.round(totalMs) + ' ms'); }
else { console.table(rows); console.log('total render time ' + Math.round(totalMs) + ' ms'); }
