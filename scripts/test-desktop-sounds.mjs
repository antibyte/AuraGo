// Measured gates of the desktop sound system (spec 2026-11-23 §12).
//   node scripts/test-desktop-sounds.mjs                 all 16 themes, every event, variant and effect
//   node scripts/test-desktop-sounds.mjs --theme water   authoring run for one or more themes (no cross-theme checks)
import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';
import { loadDesktopSounds } from './desktop-sound-loader.mjs';

const args = process.argv.slice(2), only = [];
for (let i = 0; i < args.length; i++) if (args[i] === '--theme') only.push(...String(args[++i] || '').split(',').filter(Boolean));
const full = only.length === 0;
const budgetMs = Number(process.env.DESKTOP_SOUNDS_THEME_BUDGET_MS || 6000);
const { DSP, Engine } = loadDesktopSounds({ themes: full ? 'all' : only, requireAll: full });
const FX_OWNERS = { voxel_world: ['window.fracture', 'window.impact'], green_slime: ['slime.pop', 'slime.snap'], orbit: ['orbit.ping'] };
const MIRRORED = [['window.open', 'window.close'], ['window.restore', 'window.minimize'], ['menu.open', 'menu.close'], ['dialog.confirm', 'dialog.cancel'], ['ui.toggle.on', 'ui.toggle.off']];
const failures = [], warnings = [], rows = [];
const check = (ok, message) => { if (!ok) failures.push(message); };

// Catalogue and registration
assert.equal(Engine.EVENTS.length, 26);
assert.deepEqual(Object.keys(Engine.ROLES).sort(), Engine.EVENTS.slice().sort());
for (const id of Engine.EVENTS) { const r = Engine.ROLES[id]; assert.ok(Engine.CATEGORIES.includes(r.category) && r.variants >= 1 && r.variants <= 3, id); }
const themes = Engine.themes();
assert.deepEqual(themes.slice().sort(), (full ? Engine.THEME_IDS : only).slice().sort(), 'registered themes');
for (const t of themes) assert.deepEqual(Object.keys(self.DesktopSoundThemes[t].fx || {}).sort(), (FX_OWNERS[t] || []).slice().sort(), t + ' effects');

// Validation rejects broken definitions
const base = self.DesktopSoundThemes[themes[0]];
const clone = () => JSON.parse(JSON.stringify(base));
const voice0 = Object.keys(base.voices)[0];
assert.deepEqual(Engine.validateTheme(clone()), [], 'a registered theme validates');
for (const [name, mutate] of [
  ['unknown model', d => { d.voices[voice0].model = 'kazoo'; }],
  ['unknown parameter', d => { d.voices[voice0].sparkle = 1; }],
  ['unknown voice', d => { d.events['window.open'].notes = [{ v: 'nope', n: 0, t: 0, g: 1 }]; }],
  ['unknown event', d => { d.events['window.wobble'] = d.events['window.open']; }],
  ['missing event', d => { delete d.events['ui.tick']; }],
  ['value out of range', d => { d.space.rt60 = 99; }],
  ['effect of another theme', d => { const s = d.events['window.snap']; d.id = d.id === 'orbit' ? 'crystal' : d.id; d.fx = { 'orbit.ping': { small: s, medium: s, large: s } }; }]
]) { const d = clone(); mutate(d); check(Engine.validateTheme(d).length > 0, 'validation accepted a broken theme: ' + name); }

const fingerprint = (L, R) => { let h = 0x811c9dc5; for (const ch of [L, R]) { const b = new Uint8Array(ch.buffer, ch.byteOffset, ch.byteLength); for (let i = 0; i < b.length; i++) h = Math.imul(h ^ b[i], 0x01000193) >>> 0; } return h.toString(16) + ':' + L.length; };
const energy = a => { let e = 0; for (let i = 0; i < a.length; i++) e += a[i] * a[i]; return e; };
const below90 = ch => { const b = Float32Array.from(ch); DSP.lowpass(b, 90, 0.5412); DSP.lowpass(b, 90, 1.3066); return energy(b); };
const mean = ch => { let s = 0; for (let i = 0; i < ch.length; i++) s += ch[i]; return s / ch.length; };

function gate(label, out, r) {
  const { L, R } = out, n = L.length;
  check(n > 0 && R.length === n && out.sampleRate === 48000 && Math.abs(out.seconds - n / 48000) < 1e-6, `${label}: bad buffer shape`);
  for (let i = 0; i < n; i++) if (!Number.isFinite(L[i]) || !Number.isFinite(R[i])) { failures.push(`${label}: NaN or Infinity`); return; }
  if (out.limited) {
    check(out.lufs <= r.lufs + 0.5, `${label}: ${out.lufs.toFixed(2)} LUFS above target ${r.lufs} while peak-limited`);
    check(out.lufs >= r.lufs - 3, `${label}: peak-limited ${(r.lufs - out.lufs).toFixed(1)} LU under target (transient too sharp: add a tonal body or soften the attack)`);
  } else check(Math.abs(out.lufs - r.lufs) <= 0.5, `${label}: ${out.lufs.toFixed(2)} LUFS, target ${r.lufs} ±0.5`);
  const measured = DSP.loudness({ L, R, n });
  check(Math.abs(measured - out.lufs) <= 0.05, `${label}: reports ${out.lufs.toFixed(2)} LUFS, measures ${measured.toFixed(2)}`);
  const tp = DSP.gainToDb(DSP.truePeak({ L, R, n }));
  check(tp <= -3 + 0.05, `${label}: true peak ${tp.toFixed(2)} dBTP above -3`);
  check(out.naturalSeconds <= r.maxSeconds + 1e-9, `${label}: natural length ${out.naturalSeconds.toFixed(3)} s over ${r.maxSeconds} s`);
  check(L[0] === 0 && R[0] === 0 && L[n - 1] === 0 && R[n - 1] === 0, `${label}: first or last sample not zero`);
  check(Math.abs(mean(L)) < 1e-3 && Math.abs(mean(R)) < 1e-3, `${label}: DC offset`);
  const low = 10 * Math.log10(Math.max(below90(L) + below90(R), 1e-20) / Math.max(energy(L) + energy(R), 1e-20));
  check(low <= -30, `${label}: energy below 90 Hz only ${(-low).toFixed(1)} dB under total`);
}

const prints = {};
for (const theme of themes) {
  const t0 = performance.now(); let renders = 0, worst = 0;
  prints[theme] = {};
  for (const eventId of Engine.EVENTS) {
    const r = Engine.ROLES[eventId], outs = [];
    for (let v = 0; v < r.variants; v++) { const out = Engine.render(theme, eventId, v); renders++; gate(`${theme}/${eventId}#${v}`, out, r); outs.push(out); if (!out.limited) worst = Math.max(worst, Math.abs(out.lufs - r.lufs)); }
    prints[theme][eventId] = fingerprint(outs[0].L, outs[0].R);
    for (let a = 0; a < outs.length; a++) for (let b = a + 1; b < outs.length; b++) {
      check(fingerprint(outs[a].L, outs[a].R) !== fingerprint(outs[b].L, outs[b].R), `${theme}/${eventId}: variants ${a} and ${b} are identical`);
      check(Math.abs(outs[a].lufs - outs[b].lufs) <= 1, `${theme}/${eventId}: variants ${a} and ${b} differ by more than 1 LU`);
    }
  }
  for (const eventId of FX_OWNERS[theme] || []) for (const name of Engine.FX_ROLES[eventId].names) { gate(`${theme}/${eventId}:${name}`, Engine.render(theme, eventId, name), Engine.FX_ROLES[eventId]); renders++; }
  const ms = performance.now() - t0;
  check(ms <= budgetMs, `${theme}: full render took ${ms.toFixed(0)} ms, budget ${budgetMs} ms`);
  for (const eventId of ['window.open', 'notify.message', 'ui.tick']) {
    const a = Engine.render(theme, eventId, 0), b = Engine.render(theme, eventId, 0);
    check(fingerprint(a.L, a.R) === fingerprint(b.L, b.R), `${theme}/${eventId}: render is not deterministic`);
  }
  for (const [x, y] of MIRRORED) check(prints[theme][x] !== prints[theme][y], `${theme}: mirrored pair ${x}/${y} renders identical audio`);
  rows.push({ theme, renders, ms: Math.round(ms), worstLU: worst.toFixed(2) });
}
if (full) for (const eventId of Engine.EVENTS) {
  const seen = new Set(themes.map(t => prints[t][eventId]));
  check(seen.size === themes.length, `${eventId}: two themes render identical audio`);
}
console.table(rows);
warnings.forEach(w => console.warn('warning: ' + w));
if (failures.length) { console.error(failures.length + ' gate failure(s):\n  ' + failures.join('\n  ')); process.exit(1); }
console.log('desktop sound gates: ok (' + themes.length + ' theme(s))');
