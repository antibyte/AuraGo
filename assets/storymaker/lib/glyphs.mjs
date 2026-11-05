// Fallback font coverage, schema storymaker-glyphs/1. The catalog build reads the cmap of each face and writes
// ui/img/storymaker/glyphs.json. Go and the player read only that file, so neither needs a font parser.
// Ported from docs/superpowers/plans/2026-11-04-storymaker-phase2/spikes/fonts/cmap.mjs and glyphs.mjs.

import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export const GLYPHS_SCHEMA = 'storymaker-glyphs/1';

const MIB = 1048576;
const DEFAULT_FONT_CAP = 8 * MIB;

// Faces in fallback order. Greek is first with its range, because Geist has only 6 Greek letters and Chrome would draw
// omega as a capital Omega from them. CJK is last, because it also covers Latin.
export const GLYPH_FACES = [
  { id: 'sm-greek', family: 'SMGreek', file: '/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2', weight: '300 700', unicode_range: 'U+0370-03FF' },
  { id: 'sm-greek-ext', family: 'SMGreek', file: '/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2ZL7SUc.woff2', weight: '300 700', unicode_range: 'U+1F00-1FFF' },
  { id: 'geist', family: 'Geist', file: '/fonts/Geist-Regular.woff2', weight: '400', unicode_range: '', exclude: [[0x0370, 0x03ff]] },
  { id: 'sm-deva', family: 'SMDeva', file: '/fonts/qr/NotoSansDevanagari.ttf', weight: '100 900', unicode_range: '' },
  { id: 'sm-arab', family: 'SMArab', file: '/fonts/qr/NotoSansArabic.ttf', weight: '100 900', unicode_range: '' },
  { id: 'sm-cjk', family: 'SMCJK', file: '/fonts/qr/NotoSansCJKsc.otf', weight: '400', unicode_range: '', max_bytes: 17 * MIB },
];

// The known table tags of the WOFF2 format, in the order of its specification.
const WOFF2_TAGS = ['cmap', 'head', 'hhea', 'hmtx', 'maxp', 'name', 'OS/2', 'post', 'cvt ', 'fpgm', 'glyf', 'loca', 'prep', 'CFF ', 'VORG', 'EBDT', 'EBLC', 'gasp', 'hdmx', 'kern', 'LTSH', 'PCLT', 'VDMX', 'vhea', 'vmtx', 'BASE', 'GDEF', 'GPOS', 'GSUB', 'EBSC', 'JSTF', 'MATH', 'CBDT', 'CBLC', 'COLR', 'CPAL', 'SVG ', 'sbix', 'acnt', 'avar', 'bdat', 'bloc', 'bsln', 'cvar', 'fdsc', 'feat', 'fmtx', 'fvar', 'gvar', 'hsty', 'just', 'lcar', 'mort', 'morx', 'opbd', 'prop', 'trak', 'Zapf', 'Silf', 'Glat', 'Gloc', 'Feat', 'Sill'];

/** The tables of a WOFF2 file by tag. Transformed tables stay as stored; only cmap and name are read. */
export function woff2Tables(buf) {
  if (buf.toString('latin1', 0, 4) !== 'wOF2') throw new Error('not a WOFF2 file');
  const numTables = buf.readUInt16BE(12);
  const totalCompressed = buf.readUInt32BE(20);
  let p = 48;
  const base128 = () => {
    let value = 0;
    for (let i = 0; i < 5; i++) {
      const b = buf[p++];
      value = value * 128 + (b & 127);
      if (!(b & 128)) return value;
    }
    throw new Error('bad base128 value');
  };
  const entries = [];
  for (let i = 0; i < numTables; i++) {
    const flags = buf[p++];
    let tag;
    if ((flags & 63) === 63) {
      tag = buf.toString('latin1', p, p + 4);
      p += 4;
    } else {
      tag = WOFF2_TAGS[flags & 63];
    }
    const version = flags >> 6;
    const orig = base128();
    const transformed = tag === 'glyf' || tag === 'loca' ? version === 0 : version !== 0;
    entries.push({ tag, length: transformed ? base128() : orig });
  }
  const data = zlib.brotliDecompressSync(buf.subarray(p, p + totalCompressed));
  const tables = {};
  let offset = 0;
  for (const e of entries) {
    tables[e.tag] = data.subarray(offset, offset + e.length);
    offset += e.length;
  }
  return tables;
}

/** The tables of a TTF, OTF or font collection by tag. */
export function sfntTables(buf) {
  const base = buf.toString('latin1', 0, 4) === 'ttcf' ? buf.readUInt32BE(12) : 0;
  const count = buf.readUInt16BE(base + 4);
  const tables = {};
  for (let i = 0; i < count; i++) {
    const e = base + 12 + i * 16;
    const offset = buf.readUInt32BE(e + 8);
    tables[buf.toString('latin1', e, e + 4)] = buf.subarray(offset, offset + buf.readUInt32BE(e + 12));
  }
  return tables;
}

function readCmapFormat4(c, off, set) {
  const segX2 = c.readUInt16BE(off + 6);
  const segs = segX2 / 2;
  const endO = off + 14;
  const startO = endO + segX2 + 2;
  const deltaO = startO + segX2;
  const rangeO = deltaO + segX2;
  for (let s = 0; s < segs; s++) {
    const end = c.readUInt16BE(endO + s * 2);
    const start = c.readUInt16BE(startO + s * 2);
    const delta = c.readInt16BE(deltaO + s * 2);
    const ro = c.readUInt16BE(rangeO + s * 2);
    for (let cp = start; cp <= end && cp !== 0xffff; cp++) {
      let g;
      if (ro === 0) {
        g = (cp + delta) & 0xffff;
      } else {
        g = c.readUInt16BE(rangeO + s * 2 + ro + (cp - start) * 2);
        if (g) g = (g + delta) & 0xffff;
      }
      if (g) set.add(cp);
    }
  }
}

/** The Unicode code points of a cmap table, without U+0000. */
export function parseCmap(c) {
  const set = new Set();
  const n = c.readUInt16BE(2);
  for (let i = 0; i < n; i++) {
    const plat = c.readUInt16BE(4 + i * 8);
    const enc = c.readUInt16BE(6 + i * 8);
    const off = c.readUInt32BE(8 + i * 8);
    if (plat !== 0 && !(plat === 3 && (enc === 1 || enc === 10))) continue;
    const fmt = c.readUInt16BE(off);
    if (fmt === 4) {
      readCmapFormat4(c, off, set);
    } else if (fmt === 12 || fmt === 13) {
      const groups = c.readUInt32BE(off + 12);
      for (let g = 0; g < groups; g++) {
        const first = c.readUInt32BE(off + 16 + g * 12);
        const last = c.readUInt32BE(off + 20 + g * 12);
        for (let cp = first; cp <= last; cp++) set.add(cp);
      }
    } else if (fmt === 6) {
      const first = c.readUInt16BE(off + 6);
      const count = c.readUInt16BE(off + 8);
      for (let k = 0; k < count; k++) if (c.readUInt16BE(off + 10 + k * 2)) set.add(first + k);
    } else if (fmt === 0) {
      for (let k = 0; k < 256; k++) if (c[off + 6 + k]) set.add(k);
    }
  }
  set.delete(0);
  return set;
}

function parseUnicodeRange(text) {
  if (!text) return null;
  const m = /^U\+([0-9A-F]+)-([0-9A-F]+)$/.exec(text);
  if (!m) throw new Error(`bad unicode_range ${text}`);
  return [parseInt(m[1], 16), parseInt(m[2], 16)];
}

// A code point is covered if it is in the face's cmap, inside its unicode_range and outside its exclusions.
function effectiveCodepoints(face) {
  const buf = fs.readFileSync(path.join(ROOT, 'ui', ...face.file.split('/')));
  const cap = face.max_bytes || DEFAULT_FONT_CAP;
  if (buf.length > cap) throw new Error(`${face.file}: ${buf.length} bytes is over its cap of ${cap}`);
  const tables = face.file.endsWith('.woff2') ? woff2Tables(buf) : sfntTables(buf);
  if (!tables.cmap) throw new Error(`${face.file}: no cmap table`);
  const only = parseUnicodeRange(face.unicode_range);
  const exclude = face.exclude || [];
  return [...parseCmap(tables.cmap)]
    .filter((cp) => (!only || (cp >= only[0] && cp <= only[1])) && !exclude.some(([a, b]) => cp >= a && cp <= b))
    .sort((a, b) => a - b);
}

/** Sorted code points to the delta format: first value the start, then gap = start - previous end, len = end - start. */
export function encodeRanges(cps) {
  const ranges = [];
  for (const cp of cps) {
    const last = ranges[ranges.length - 1];
    if (last && cp === last[1] + 1) last[1] = cp;
    else ranges.push([cp, cp]);
  }
  const d = [];
  let prevEnd = 0;
  for (const [start, end] of ranges) {
    d.push(start - prevEnd, end - start);
    prevEnd = end;
  }
  return d;
}

/** The delta format to inclusive [start, end] pairs. */
export function decodeRanges(d) {
  const out = [];
  let end = 0;
  for (let i = 0; i < d.length; i += 2) {
    const start = end + d[i];
    end = start + d[i + 1];
    out.push([start, end]);
  }
  return out;
}

export function rangesCover(ranges, cp) {
  return ranges.some(([a, b]) => cp >= a && cp <= b);
}

let cached = null;

/** The storymaker-glyphs/1 document. Reading the CJK font takes a moment, so one process reads it once. */
export function buildGlyphs() {
  if (cached) return cached;
  const faces = GLYPH_FACES.map((face) => {
    const { exclude, ...row } = face;
    return { ...row, d: encodeRanges(effectiveCodepoints(face)) };
  });
  cached = { schema: GLYPHS_SCHEMA, faces };
  return cached;
}

/** The file text: one face per line, LF line endings. */
export function glyphsText(data) {
  const rows = data.faces.map((f) => '  ' + JSON.stringify(f));
  return `{"schema":${JSON.stringify(data.schema)},"faces":[\n${rows.join(',\n')}\n]}\n`;
}
