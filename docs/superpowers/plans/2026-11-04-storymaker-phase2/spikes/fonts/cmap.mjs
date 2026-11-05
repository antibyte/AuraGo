// usage: node cmap.mjs <repoRoot> <outDir>
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';

const root = process.argv[2], out = process.argv[3];

function sfntFromWoff2(buf) {
  if (buf.toString('latin1', 0, 4) !== 'wOF2') throw new Error('not woff2');
  const numTables = buf.readUInt16BE(12), totalCompressed = buf.readUInt32BE(20);
  let p = 48;
  const known = ['cmap','head','hhea','hmtx','maxp','name','OS/2','post','cvt ','fpgm','glyf','loca','prep','CFF ','VORG','EBDT','EBLC','gasp','hdmx','kern','LTSH','PCLT','VDMX','vhea','vmtx','BASE','GDEF','GPOS','GSUB','EBSC','JSTF','MATH','CBDT','CBLC','COLR','CPAL','SVG ','sbix','acnt','avar','bdat','bloc','bsln','cvar','fdsc','feat','fmtx','fvar','gvar','hsty','just','lcar','mort','morx','opbd','prop','trak','Zapf','Silf','Glat','Gloc','Feat','Sill'];
  const tables = [];
  const b128 = () => { let a = 0; for (let i = 0; i < 5; i++) { const b = buf[p++]; a = a * 128 + (b & 127); if (!(b & 128)) return a; } throw new Error('b128'); };
  for (let i = 0; i < numTables; i++) {
    const flags = buf[p++]; let tag;
    if ((flags & 63) === 63) { tag = buf.toString('latin1', p, p + 4); p += 4; } else tag = known[flags & 63];
    const ver = flags >> 6, orig = b128();
    const transformed = (tag === 'glyf' || tag === 'loca') ? ver === 0 : ver !== 0;
    const len = transformed ? b128() : orig;
    tables.push({ tag, orig, len });
  }
  const data = zlib.brotliDecompressSync(buf.subarray(p, p + totalCompressed));
  let o = 0; const map = {};
  for (const t of tables) { map[t.tag] = data.subarray(o, o + t.len); o += t.len; }
  return map;
}

function sfntTables(buf) {
  let base = 0;
  if (buf.toString('latin1', 0, 4) === 'ttcf') base = buf.readUInt32BE(12);
  const n = buf.readUInt16BE(base + 4), map = {};
  for (let i = 0; i < n; i++) {
    const e = base + 12 + i * 16;
    const tag = buf.toString('latin1', e, e + 4), off = buf.readUInt32BE(e + 8), len = buf.readUInt32BE(e + 12);
    map[tag] = buf.subarray(off, off + len);
  }
  return map;
}

function parseCmap(c) {
  const set = new Set();
  const n = c.readUInt16BE(2);
  for (let i = 0; i < n; i++) {
    const plat = c.readUInt16BE(4 + i * 8), enc = c.readUInt16BE(6 + i * 8), off = c.readUInt32BE(8 + i * 8);
    const uni = plat === 0 || (plat === 3 && (enc === 1 || enc === 10));
    if (!uni) continue;
    const fmt = c.readUInt16BE(off);
    if (fmt === 4) {
      const segX2 = c.readUInt16BE(off + 6), segs = segX2 / 2;
      const endO = off + 14, startO = endO + segX2 + 2, deltaO = startO + segX2, rangeO = deltaO + segX2;
      for (let s = 0; s < segs; s++) {
        const end = c.readUInt16BE(endO + s * 2), start = c.readUInt16BE(startO + s * 2);
        const delta = c.readInt16BE(deltaO + s * 2), ro = c.readUInt16BE(rangeO + s * 2);
        for (let cp = start; cp <= end && cp !== 0xFFFF; cp++) {
          let g;
          if (ro === 0) g = (cp + delta) & 0xFFFF;
          else { const gi = rangeO + s * 2 + ro + (cp - start) * 2; g = c.readUInt16BE(gi); if (g) g = (g + delta) & 0xFFFF; }
          if (g) set.add(cp);
        }
      }
    } else if (fmt === 12 || fmt === 13) {
      const ng = c.readUInt32BE(off + 12);
      for (let g = 0; g < ng; g++) {
        const s = c.readUInt32BE(off + 16 + g * 12), e = c.readUInt32BE(off + 20 + g * 12), gid = c.readUInt32BE(off + 24 + g * 12);
        if (fmt === 12 && gid === 0 && s === 0) { /* may include notdef */ }
        for (let cp = s; cp <= e; cp++) set.add(cp);
      }
    } else if (fmt === 6) {
      const first = c.readUInt16BE(off + 6), cnt = c.readUInt16BE(off + 8);
      for (let k = 0; k < cnt; k++) if (c.readUInt16BE(off + 10 + k * 2)) set.add(first + k);
    } else if (fmt === 0) {
      for (let k = 0; k < 256; k++) if (c[off + 6 + k]) set.add(k);
    }
  }
  set.delete(0);
  return set;
}

const files = [];
for (const f of fs.readdirSync(path.join(root, 'ui/fonts'))) if (/\.woff2$/.test(f)) files.push(['ui/fonts/' + f]);
for (const f of fs.readdirSync(path.join(root, 'ui/fonts/qr'))) if (/\.(ttf|otf)$/.test(f)) files.push(['ui/fonts/qr/' + f]);

const blocks = {
  'Latin (0020-007E,00A0-00FF)': [[0x20, 0x7E], [0xA0, 0xFF]],
  'Latin Ext-A/B (0100-024F)': [[0x100, 0x24F]],
  'Latin Ext Additional (1E00-1EFF)': [[0x1E00, 0x1EFF]],
  'Greek (0370-03FF)': [[0x370, 0x3FF]],
  'Greek Extended (1F00-1FFF)': [[0x1F00, 0x1FFF]],
  'Cyrillic (0400-04FF)': [[0x400, 0x4FF]],
  'Devanagari (0900-097F)': [[0x900, 0x97F]],
  'Arabic (0600-06FF)': [[0x600, 0x6FF]],
  'Arabic Pres-A/B (FB50-FDFF,FE70-FEFF)': [[0xFB50, 0xFDFF], [0xFE70, 0xFEFF]],
  'Han (4E00-9FFF)': [[0x4E00, 0x9FFF]],
  'Hiragana+Katakana (3040-30FF)': [[0x3040, 0x30FF]],
  'Hangul syll (AC00-D7A3)': [[0xAC00, 0xD7A3]],
  'CJK punct (3000-303F)': [[0x3000, 0x303F]],
  'Emoji (1F300-1FAFF)': [[0x1F300, 0x1FAFF]],
};
function cover(set, rs) { let have = 0, tot = 0; for (const [a, b] of rs) for (let c = a; c <= b; c++) { tot++; if (set.has(c)) have++; } return [have, tot]; }

fs.mkdirSync(out, { recursive: true });
const all = {}; const rows = [];
for (const [rel] of files) {
  const buf = fs.readFileSync(path.join(root, rel));
  let tabs;
  try { tabs = rel.endsWith('.woff2') ? sfntFromWoff2(buf) : sfntTables(buf); } catch (e) { rows.push(rel + ' ERROR ' + e.message); continue; }
  if (!tabs.cmap) { rows.push(rel + ' no cmap'); continue; }
  const set = parseCmap(tabs.cmap);
  all[rel] = [...set].sort((a, b) => a - b);
  const parts = [];
  for (const [name, rs] of Object.entries(blocks)) { const [h, t] = cover(set, rs); if (h) parts.push(`${name.split(' (')[0]} ${h}/${t}`); }
  rows.push(`${rel} | glyphs-cps ${set.size} | ` + parts.join('; '));
}
fs.writeFileSync(path.join(out, 'cmaps.json'), JSON.stringify(all));
fs.writeFileSync(path.join(out, 'coverage.txt'), rows.join('\n') + '\n');
console.log(rows.join('\n'));
