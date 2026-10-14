// Reproduce AuraGo's browser-only adaptation of the pinned NASSCAD release.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';

const revision = '279da7c0f0ca28dfae19c11c8be231f326dfda4f';
const out = 'internal/desktop/bundled_apps/nasscad';
const manifestPath = `${out}/provenance.json`;
const digest = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const inputHash = digest(fs.readFileSync('scripts/vendor-nasscad.mjs', 'utf8').replaceAll('\r\n', '\n'));
if (process.argv.includes('--check')) {
  const manifest = JSON.parse(fs.readFileSync(manifestPath));
  if (manifest.revision !== revision || manifest.adapter_sha256 !== inputHash) throw Error('NASSCAD provenance drift');
  const names = fs.readdirSync(out).filter(n => n !== 'provenance.json').sort();
  if (JSON.stringify(names) !== JSON.stringify(Object.keys(manifest.files).sort())) throw Error('NASSCAD file list drift');
  for (const [name, hash] of Object.entries(manifest.files)) if (digest(fs.readFileSync(`${out}/${name}`)) !== hash) throw Error(`NASSCAD checksum mismatch: ${name}`);
  console.log(`Verified ${names.length} NASSCAD files at ${revision}`);
  process.exit(0);
}
const source = process.argv.find(a => a.startsWith('--source='))?.slice(9);
if (!source) throw Error(`Pass --source=<NASSCAD ${revision} source directory>`);
const names = [
  'NASSCAD_V4_7_0.htm', 'LICENSE', 'THIRD-PARTY.md', 'web-ifc-LICENSE.md',
  'three.js', 'nasscad_logs.js', 'nasscad_occt_wasm.js', 'opencascade.wasm.wasm',
  'nasscad-draco.js', 'nasscad-fonts.js', 'nasscad-gens.js', 'nasscad-io.js',
  'nasscad-materials.js', 'nassscript.js', 'occt-import-js.js', 'occt-loader-b64.js',
  'quick-fillet.js', 'step-export.js', 'step-import.js', 'step-xcaf.js', 'step-declare.js',
  'ifc-export.js', 'ifc-import.js', 'web-ifc-api-iife.js', 'nasscad-ifc-wasm.js',
  'nasscad-manifold-wasm.js', 'nasscad-medusa-bridge.js', 'nasscad-desktop.js',
  'nasscad-buildvolume.js', 'nasscad-audit-fixes.js', 'nasscad-object-range.js',
  'nass-demo-engine.js', 'nass-demo-solar.js', 'nasscad_user_manual.html',
  'nasscript_doc.html', 'nassscript-engine-preview.png', 'solar-system-snapshot.png',
];
const outputs = new Map(), upstream = {};
function patch(text, before, after) {
  if (!text.includes(before)) throw Error(`NASSCAD patch anchor missing: ${before}`);
  return text.replaceAll(before, after);
}
for (const name of names) {
  const bytes = fs.readFileSync(path.join(source, name));
  upstream[name] = digest(bytes);
  let text = bytes.toString('utf8'), target = name;
  if (name === 'NASSCAD_V4_7_0.htm') {
    target = 'index.html';
    text = text.replace(/<!-- GA4[^]*?<\/script>/, '<!-- AuraGo: no analytics. -->');
    if (text.includes('googletagmanager.com')) throw Error('NASSCAD analytics patch failed');
    text = patch(text, 'nassscript.js?v=20261004-stop', 'nassscript.js');
    text = patch(text, "sc.src = _WASM_CSG_FILE;", "sc.src = _WASM_CSG_FILE + location.search;");
    text = patch(text, 'new URL(demoSources[key],document.baseURI)', 'new URL(demoSources[key]+location.search,document.baseURI)');
    // Signed workspace access must also cover the local illustrated guides.
    text = patch(text, '</head>', `<script>document.addEventListener('DOMContentLoaded',()=>{for(const link of document.querySelectorAll('a[href]')){const url=new URL(link.href);if(url.origin===location.origin&&url.pathname.startsWith(new URL('.',location.href).pathname)) {url.search=location.search;link.href=url.href;}}});</script>\n</head>`);
  } else if (name === 'nasscript_doc.html') {
    // The guide must not reach Google: no analytics, and its font stacks fall back to local fonts.
    text = text.replace(/<!-- GA4[^]*?<\/script>/, '<!-- AuraGo: no analytics. -->');
    text = patch(text, '<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=IBM+Plex+Sans:wght@400;500;600&family=IBM+Plex+Sans+Condensed:wght@600;700&display=swap">\n', '');
    if (/googletagmanager\.com|fonts\.googleapis\.com/.test(text)) throw Error('NASSCAD guide patch failed');
  } else if (name === 'step-import.js') {
    text = patch(text, "s.src = 'nasscad_occt_wasm.js';", "s.src = 'nasscad_occt_wasm.js' + location.search;");
  } else if (name === 'quick-fillet.js') {
    text = patch(text, "fetch('opencascade.wasm.wasm')", "fetch('opencascade.wasm.wasm'+location.search)");
  } else if (name === 'ifc-import.js') {
    text = patch(text, "new URL('.', document.currentScript.src)", "new URL('.', document.baseURI)");
    text = patch(text, 'new URL(name, _ifcAssetBase).href', 'new URL(name, _ifcAssetBase).href + location.search');
  }
  outputs.set(target, name.endsWith('.js') || name.endsWith('.htm') || name === 'nasscript_doc.html' ? Buffer.from(text) : bytes);
}
fs.mkdirSync(out, {recursive:true});
for (const [name, bytes] of outputs) fs.writeFileSync(`${out}/${name}`, bytes);
const manifest = {version:'4.7.0', revision, source:`https://github.com/Nx-Nass/NASSCAD/tree/${revision}`, adapter_sha256:inputHash, upstream, files:Object.fromEntries([...outputs].map(([n,b])=>[n,digest(b)]))};
fs.writeFileSync(manifestPath, JSON.stringify(manifest,null,2)+'\n');
console.log(`Vendored ${outputs.size} NASSCAD browser assets`);
