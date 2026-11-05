import fs from 'node:fs';
import zlib from 'node:zlib';
const all = JSON.parse(fs.readFileSync('out/cmaps.json', 'utf8'));
const same = ['Bold', 'Light', 'Medium', 'SemiBold'].every(w => JSON.stringify(all['ui/fonts/Geist-' + w + '.woff2']) === JSON.stringify(all['ui/fonts/Geist-Regular.woff2']));
console.log('Geist weights identical cmap:', same);
function ranges(cps) { const r = []; for (const c of cps) { const l = r[r.length - 1]; if (l && c === l[1] + 1) l[1] = c; else r.push([c, c]); } return r; }
const merge = (...keys) => [...new Set(keys.flatMap(k => all[k]))].sort((a, b) => a - b);
const faces = {
  'geist': all['ui/fonts/Geist-Regular.woff2'],
  'inter-greek': merge('ui/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2', 'ui/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2ZL7SUc.woff2'),
  'noto-devanagari': all['ui/fonts/qr/NotoSansDevanagari.ttf'],
  'noto-arabic': all['ui/fonts/qr/NotoSansArabic.ttf'],
  'noto-cjk': all['ui/fonts/qr/NotoSansCJKsc.otf'],
};
const out = { schema: 'storymaker-glyphs/1', faces: {} };
const flat = { schema: 'storymaker-glyphs/1', faces: {} };
for (const [k, v] of Object.entries(faces)) {
  const r = ranges(v);
  // delta format: [start0, len0, gap1, len1, ...] with len = end-start
  const d = []; let prev = 0; for (const [s, e] of r) { d.push(s - prev, e - s); prev = e; }
  out.faces[k] = { ranges: r.length, codepoints: v.length, d };
  flat.faces[k] = r.flat();
  console.log(k, 'cps', v.length, 'ranges', r.length);
}
const a = JSON.stringify(flat), b = JSON.stringify(out);
fs.writeFileSync('out/glyphs-flat.json', a);
fs.writeFileSync('out/glyphs-delta.json', b);
for (const [n, s] of [['flat start,end pairs', a], ['delta start/len', b]]) console.log(n, 'raw', Buffer.byteLength(s), 'gzip', zlib.gzipSync(s, { level: 9 }).length);
// without CJK
const noCjk = JSON.stringify({ ...flat, faces: Object.fromEntries(Object.entries(flat.faces).filter(([k]) => k !== 'noto-cjk')) });
console.log('flat without cjk raw', noCjk.length, 'gzip', zlib.gzipSync(noCjk).length);
