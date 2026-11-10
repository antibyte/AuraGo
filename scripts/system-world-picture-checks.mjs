#!/usr/bin/env node
// System World picture checks (beauty pass, phase 6). A minimal PNG codec (8-bit RGB or RGBA, not interlaced: what the
// survey's canvas captures are) and pure measurements on survey pictures: the clipped share and the largest clipped blob
// inside a rectangle, mask decoding, luma percentiles, the lit share and a coarse wall-tone count inside a mask; contact
// sheets and before/after pairs; the frame-time and triangle budget. Node only, no dependencies.
//   node scripts/system-world-picture-checks.mjs sheet --dir <dir> --name <sequence> [--cols 4] [--scale 2]
//   node scripts/system-world-picture-checks.mjs compare --before <dir> --after <dir> --out <dir> [--only a,b] [--scale 2]
//   node scripts/system-world-picture-checks.mjs lit-compare --a <dir> --b <dir> --scene <name> [--tol 0.10]
//   node scripts/system-world-picture-checks.mjs budget-init --reports <dir,dir> --out <file> [--margin 0.10] [--triangles 650000]
//   node scripts/system-world-picture-checks.mjs budget-check --budget <file> --reports <dir,dir>
import zlib from 'node:zlib';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const SIGNATURE = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
const CRC = new Uint32Array(256).map((_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
function crc32(buf) { let c = 0xffffffff; for (const b of buf) c = CRC[(c ^ b) & 255] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; }

export function decodePNG(buf) {
  if (!Buffer.isBuffer(buf)) buf = Buffer.from(buf);
  if (!buf.subarray(0, 8).equals(SIGNATURE)) throw Error('not a PNG');
  let off = 8, width = 0, height = 0, depth = 0, type = 0, interlace = 0; const idat = [];
  while (off + 8 <= buf.length) {
    const len = buf.readUInt32BE(off), kind = buf.toString('latin1', off + 4, off + 8), data = buf.subarray(off + 8, off + 8 + len);
    if (kind === 'IHDR') { width = data.readUInt32BE(0); height = data.readUInt32BE(4); depth = data[8]; type = data[9]; interlace = data[12]; }
    else if (kind === 'IDAT') idat.push(data);
    else if (kind === 'IEND') break;
    off += 12 + len;
  }
  if (depth !== 8 || (type !== 6 && type !== 2) || interlace) throw Error(`unsupported PNG (depth ${depth}, colour type ${type}, interlace ${interlace})`);
  const channels = type === 6 ? 4 : 3, stride = width * channels, raw = zlib.inflateSync(Buffer.concat(idat));
  const data = new Uint8Array(width * height * 4); let prev = new Uint8Array(stride), line = new Uint8Array(stride);
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)], src = raw.subarray(y * (stride + 1) + 1, (y + 1) * (stride + 1));
    for (let i = 0; i < stride; i++) {
      const a = i >= channels ? line[i - channels] : 0, b = prev[i], c = i >= channels ? prev[i - channels] : 0;
      let v = src[i];
      if (filter === 1) v += a; else if (filter === 2) v += b; else if (filter === 3) v += (a + b) >> 1;
      else if (filter === 4) { const p = a + b - c, pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c); v += pa <= pb && pa <= pc ? a : pb <= pc ? b : c; }
      line[i] = v & 255;
    }
    for (let x = 0; x < width; x++) { const o = (y * width + x) * 4, s = x * channels; data[o] = line[s]; data[o + 1] = line[s + 1]; data[o + 2] = line[s + 2]; data[o + 3] = channels === 4 ? line[s + 3] : 255; }
    [prev, line] = [line, prev];
  }
  return { width, height, data };
}

export function encodePNG({ width, height, data }) {
  const stride = width * 4, raw = Buffer.alloc((stride + 1) * height);
  for (let y = 0; y < height; y++) Buffer.from(data.buffer, data.byteOffset + y * stride, stride).copy(raw, y * (stride + 1) + 1);
  const chunk = (kind, body) => { const head = Buffer.alloc(8); head.writeUInt32BE(body.length, 0); head.write(kind, 4, 'latin1');
    const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([head.subarray(4), body])), 0); return Buffer.concat([head, body, crc]); };
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(width, 0); ihdr.writeUInt32BE(height, 4); ihdr[8] = 8; ihdr[9] = 6;
  return Buffer.concat([SIGNATURE, chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw, { level: 6 })), chunk('IEND', Buffer.alloc(0))]);
}

export const luma = (r, g, b) => .2126 * r + .7152 * g + .0722 * b;
// [x0, y0, x1, y1] in fractions of the frame -> pixel bounds clamped to the image.
export function pixelRect(img, rect = [0, 0, 1, 1]) {
  const c = (v, n) => Math.max(0, Math.min(n, Math.round(v * n)));
  return [c(rect[0], img.width), c(rect[1], img.height), c(rect[2], img.width), c(rect[3], img.height)];
}
// Pixels at or above `threshold` in all three channels: their share of the rectangle and the largest 4-connected blob.
export function clipped(img, rect, threshold = 250) {
  const [x0, y0, x1, y1] = pixelRect(img, rect), w = x1 - x0, h = y1 - y0;
  if (w <= 0 || h <= 0) return { share: 0, pixels: 0, largest: 0, area: 0 };
  const hit = new Uint8Array(w * h); let pixels = 0;
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
    const o = ((y0 + y) * img.width + x0 + x) * 4;
    if (img.data[o] >= threshold && img.data[o + 1] >= threshold && img.data[o + 2] >= threshold) { hit[y * w + x] = 1; pixels++; }
  }
  let largest = 0; const stack = [];
  for (let i = 0; i < hit.length; i++) {
    if (hit[i] !== 1) continue;
    let size = 0; hit[i] = 2; stack.push(i);
    while (stack.length) {
      const j = stack.pop(), x = j % w, y = (j - x) / w; size++;
      if (x > 0 && hit[j - 1] === 1) { hit[j - 1] = 2; stack.push(j - 1); }
      if (x < w - 1 && hit[j + 1] === 1) { hit[j + 1] = 2; stack.push(j + 1); }
      if (y > 0 && hit[j - w] === 1) { hit[j - w] = 2; stack.push(j - w); }
      if (y < h - 1 && hit[j + w] === 1) { hit[j + w] = 2; stack.push(j + w); }
    }
    if (size > largest) largest = size;
  }
  return { share: pixels / (w * h), pixels, largest, area: w * h };
}
export function maskOf(img, colour, tolerance = 60) {
  const out = new Uint8Array(img.width * img.height);
  for (let i = 0; i < out.length; i++) { const o = i * 4;
    out[i] = Math.abs(img.data[o] - colour[0]) <= tolerance && Math.abs(img.data[o + 1] - colour[1]) <= tolerance && Math.abs(img.data[o + 2] - colour[2]) <= tolerance ? 1 : 0; }
  return out;
}
export function maskLumas(img, mask) { const out = []; for (let i = 0; i < mask.length; i++) if (mask[i]) { const o = i * 4; out.push(luma(img.data[o], img.data[o + 1], img.data[o + 2])); } return out; }
export function percentile(values, p) { if (!values.length) return NaN; const s = Float64Array.from(values).sort(); return s[Math.min(s.length - 1, Math.max(0, Math.ceil(p * s.length) - 1))]; }
export function litShare(img, mask, lumaMin = 64) {
  let n = 0, lit = 0;
  for (let i = 0; i < mask.length; i++) if (mask[i]) { n++; const o = i * 4; if (luma(img.data[o], img.data[o + 1], img.data[o + 2]) >= lumaMin) lit++; }
  return n ? lit / n : NaN;
}
// Coarse wall tones: the mean colour of every block x block cell fully inside the mask, binned by luma (3 bins) and hue
// (12 bins, or grey below `greyChroma`); a tone is a bin that holds at least `minShare` of the blocks.
export function toneCount(img, mask, { block = 8, minShare = .03, greyChroma = 10 } = {}) {
  const bins = new Map(); let blocks = 0;
  for (let by = 0; by + block <= img.height; by += block) for (let bx = 0; bx + block <= img.width; bx += block) {
    let r = 0, g = 0, b = 0, full = true;
    for (let y = by; y < by + block && full; y++) for (let x = bx; x < bx + block; x++) {
      const i = y * img.width + x; if (!mask[i]) { full = false; break; }
      const o = i * 4; r += img.data[o]; g += img.data[o + 1]; b += img.data[o + 2];
    }
    if (!full) continue;
    const n = block * block; r /= n; g /= n; b /= n; blocks++;
    const l = luma(r, g, b), band = l < 40 ? 0 : l < 110 ? 1 : 2, chroma = Math.max(r, g, b) - Math.min(r, g, b);
    const hue = chroma < greyChroma ? 'grey' : Math.floor(((Math.atan2(Math.sqrt(3) * (g - b), 2 * r - g - b) / (2 * Math.PI) + 1) % 1) * 12);
    const key = band + ':' + hue; bins.set(key, (bins.get(key) || 0) + 1);
  }
  return { tones: [...bins.values()].filter(c => blocks && c / blocks >= minShare).length, blocks };
}

export function downscale(img, factor) {
  if (!(factor > 1)) return img;
  if (!Number.isInteger(factor)) throw Error('downscale needs an integer factor');
  const w = Math.floor(img.width / factor), h = Math.floor(img.height / factor), data = new Uint8Array(w * h * 4), n = factor * factor;
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) for (let c = 0; c < 4; c++) {
    let s = 0; for (let dy = 0; dy < factor; dy++) for (let dx = 0; dx < factor; dx++) s += img.data[((y * factor + dy) * img.width + x * factor + dx) * 4 + c];
    data[(y * w + x) * 4 + c] = Math.round(s / n);
  }
  return { width: w, height: h, data };
}
export function compose(images, columns, gap = 8) {
  const w = Math.max(...images.map(i => i.width)), h = Math.max(...images.map(i => i.height)), rows = Math.ceil(images.length / columns);
  const width = columns * w + (columns + 1) * gap, height = rows * h + (rows + 1) * gap, data = new Uint8Array(width * height * 4);
  for (let i = 0; i < data.length; i += 4) { data[i] = 16; data[i + 1] = 16; data[i + 2] = 16; data[i + 3] = 255; }
  images.forEach((img, n) => {
    const ox = gap + (n % columns) * (w + gap), oy = gap + Math.floor(n / columns) * (h + gap);
    for (let y = 0; y < img.height; y++) data.set(img.data.subarray(y * img.width * 4, (y + 1) * img.width * 4), ((oy + y) * width + ox) * 4);
  });
  return { width, height, data };
}
const readPNG = file => decodePNG(fs.readFileSync(file));
// Contact sheet of a sequence: <name>-00.png, <name>-01.png ... -> <name>-sheet.png in the same folder.
export function writeSheet(dir, name, { columns = 4, scale = 2 } = {}) {
  const frames = fs.readdirSync(dir).filter(f => f.startsWith(name + '-') && /^[0-9]{2}\.png$/.test(f.slice(name.length + 1))).sort();
  if (!frames.length) return null;
  const file = path.join(dir, name + '-sheet.png');
  fs.writeFileSync(file, encodePNG(compose(frames.map(f => downscale(readPNG(path.join(dir, f)), scale)), Math.min(columns, frames.length))));
  return file;
}

// Budget file: {reference, margin, triangles: {scene, tier, max}, frameMs: {<scene>: {<tier>: base median}}}.
export function budgetInit(reports, { margin = .1, triangles = 650000 } = {}) {
  const frameMs = {};
  for (const r of reports) for (const s of r.shots || []) if (s.frameMs) {
    const t = r.info.tier; frameMs[s.scene] ??= {}; frameMs[s.scene][t] = Math.max(frameMs[s.scene][t] || 0, s.frameMs.median);
  }
  return { reference: reports[0]?.info?.renderer || null, margin, triangles: { scene: 'street-agent', tier: 'high', max: triangles }, frameMs };
}
export function budgetCheck(budget, reports) {
  const rows = [], failures = [];
  for (const r of reports) for (const s of r.shots || []) {
    const t = r.info.tier, base = budget.frameMs?.[s.scene]?.[t];
    if (s.frameMs && base) {
      const limit = +(base * (1 + budget.margin)).toFixed(2), ok = s.frameMs.median <= limit;
      rows.push({ scene: s.scene, tier: t, median: s.frameMs.median, base, limit, ok });
      if (!ok) failures.push(`${s.scene}@${t}: median ${s.frameMs.median} ms > ${limit} ms`);
    }
    const tri = budget.triangles;
    if (tri && s.scene === tri.scene && t === tri.tier && s.stats) {
      const value = s.stats.viewTriangles ?? s.stats.triangles, ok = value < tri.max;
      rows.push({ scene: s.scene, tier: t, triangles: value, limit: tri.max, ok });
      if (!ok) failures.push(`${s.scene}@${t}: ${value} triangles >= ${tri.max}`);
    }
  }
  return { rows, failures };
}

const readReport = dir => JSON.parse(fs.readFileSync(path.join(dir, 'report.json'), 'utf8'));
function parse(argv) {
  const o = {};
  for (let i = 0; i < argv.length; i++) {
    if (!argv[i].startsWith('--')) throw Error('bad argument ' + argv[i]);
    const key = argv[i].slice(2), next = argv[i + 1];
    o[key] = next === undefined || next.startsWith('--') ? true : (i++, next);
  }
  return o;
}
function main([command, ...rest]) {
  const o = parse(rest), list = v => String(v).split(',').map(s => s.trim()).filter(Boolean);
  if (command === 'sheet') { const file = writeSheet(path.resolve(o.dir), o.name, { columns: Number(o.cols || 4), scale: Number(o.scale || 2) }); console.log(file ? 'sheet ' + file : 'no frames for ' + o.name); return 0; }
  if (command === 'compare') {
    const before = path.resolve(o.before), after = path.resolve(o.after), out = path.resolve(o.out), scale = Number(o.scale || 2), only = o.only ? list(o.only) : null;
    fs.mkdirSync(out, { recursive: true });
    const shots = dir => new Map((fs.existsSync(path.join(dir, 'report.json')) ? readReport(dir).shots || [] : []).map(s => [s.name, s]));
    const sb = shots(before), sa = shots(after), f = v => v === undefined || v === null ? '' : v;
    const rows = ['| Picture | Triangles before | after | Calls before | after | Glare before | after |', '| --- | ---: | ---: | ---: | ---: | ---: | ---: |'];
    const names = fs.readdirSync(after).filter(n => n.endsWith('.png') && !/-mask\.png$|-sheet\.png$/.test(n) && fs.existsSync(path.join(before, n)))
      .map(n => n.slice(0, -4)).filter(n => !only || only.includes(n)).sort();
    for (const name of names) {
      fs.writeFileSync(path.join(out, name + '.png'), encodePNG(compose([downscale(readPNG(path.join(before, name + '.png')), scale), downscale(readPNG(path.join(after, name + '.png')), scale)], 2)));
      const b = sb.get(name), a = sa.get(name);
      rows.push(`| ${name} | ${f(b?.stats?.viewTriangles ?? b?.stats?.triangles)} | ${f(a?.stats?.viewTriangles ?? a?.stats?.triangles)} | ${f(b?.stats?.calls)} | ${f(a?.stats?.calls)} | ${f(b?.metrics?.glare?.share)} | ${f(a?.metrics?.glare?.share)} |`);
    }
    fs.writeFileSync(path.join(out, 'compare.md'), '# Before (left) and after (right)\n\n' + rows.join('\n') + '\n');
    console.log(`compared ${names.length} pictures into ${out}`); return 0;
  }
  if (command === 'lit-compare') {
    const pick = dir => (readReport(path.resolve(dir)).shots || []).find(s => s.scene === o.scene && s.metrics?.panes)?.metrics.panes.litShare;
    const a = pick(o.a), b = pick(o.b), tol = Number(o.tol || .1);
    if (!(Number.isFinite(a) && Number.isFinite(b))) { console.error('lit share missing: ' + JSON.stringify({ a, b })); return 1; }
    console.log(`lit share ${o.scene}: ${a} vs ${b} (difference ${Math.abs(a - b).toFixed(3)}, tolerance ${tol})`); return Math.abs(a - b) <= tol ? 0 : 1;
  }
  if (command === 'budget-init') {
    const budget = budgetInit(list(o.reports).map(d => readReport(path.resolve(d))), { margin: Number(o.margin || .1), triangles: Number(o.triangles || 650000) });
    fs.writeFileSync(path.resolve(o.out), JSON.stringify(budget, null, 2) + '\n'); console.log('budget written: ' + o.out); return 0;
  }
  if (command === 'budget-check') {
    const budget = JSON.parse(fs.readFileSync(path.resolve(o.budget), 'utf8')), result = budgetCheck(budget, list(o.reports).map(d => readReport(path.resolve(d))));
    console.table(result.rows); for (const f of result.failures) console.log('FAIL ' + f); return result.failures.length ? 1 : 0;
  }
  console.error('commands: sheet | compare | lit-compare | budget-init | budget-check'); return 1;
}
const self = fileURLToPath(import.meta.url);
if (process.argv[1] && path.resolve(process.argv[1]).toLowerCase() === self.toLowerCase()) {
  try { process.exit(main(process.argv.slice(2))); } catch (e) { console.error(e.message || e); process.exit(1); }
}
