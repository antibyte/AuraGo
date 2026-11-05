// Builds the Storymaker starter catalog from the code-authored generators in assets/storymaker/.
//
//   node scripts/build-storymaker-catalog.mjs                  write ui/img/storymaker/** and ui/lang/storymaker_catalog/*.json
//   node scripts/build-storymaker-catalog.mjs --check          exit 1 when a committed output differs from the generators
//   node scripts/build-storymaker-catalog.mjs --review <dir>   render the review sheets (SVG + PNG through headless Chrome)
//                                                              optional: --chrome <path-to-chrome>
//
// The build is deterministic: no clocks, no randomness outside seeded generators, LF line endings.

import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { buildOutputs, sameOutput } from '../assets/storymaker/lib/catalog.mjs';
import { localeTables, LANGS } from '../assets/storymaker/lib/names.mjs';
import { validateOutputs } from '../assets/storymaker/lib/validate.mjs';
import { castSheet, varietySheet, envPropSheet } from '../assets/storymaker/lib/review.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.join(root, 'ui', 'img', 'storymaker');
const langDir = path.join(root, 'ui', 'lang', 'storymaker_catalog');
const SUBDIRS = ['rig', 'props', 'environments', 'sfx', 'thumbs'];


function expectedFiles() {
  const built = buildOutputs();
  const files = new Map();
  for (const [p, text] of built.files) files.set(path.join(outDir, ...p.split('/')), text);
  const tables = localeTables();
  for (const lang of LANGS) files.set(path.join(langDir, `${lang}.json`), JSON.stringify(tables[lang], null, 2) + '\n');
  return { files, built };
}

function listExisting() {
  const found = [];
  const walk = (dir) => {
    if (!fs.existsSync(dir)) return;
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) walk(p);
      else found.push(p);
    }
  };
  for (const d of SUBDIRS) walk(path.join(outDir, d));
  for (const f of ['catalog.json', 'glyphs.json']) if (fs.existsSync(path.join(outDir, f))) found.push(path.join(outDir, f));
  walk(langDir);
  return found;
}

function write() {
  const { files, built } = expectedFiles();
  const problems = validateOutputs(built);
  if (problems.length) {
    console.error(problems.join('\n'));
    process.exit(1);
  }
  for (const [p, text] of files) {
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, text); // a string is written as UTF-8, a Buffer byte for byte
  }
  for (const p of listExisting()) if (!files.has(p)) fs.rmSync(p);
  let bytes = 0;
  for (const text of built.files.values()) bytes += Buffer.byteLength(text);
  console.log(`storymaker catalog ${built.manifest.version}: ${built.files.size} files, ${(bytes / 1048576).toFixed(2)} MiB in ui/img/storymaker, ${LANGS.length} locale files`);
}

function check() {
  const { files, built } = expectedFiles();
  const problems = validateOutputs(built);
  for (const [p, text] of files) {
    if (!fs.existsSync(p)) problems.push(`missing: ${path.relative(root, p)}`);
    else if (!sameOutput(fs.readFileSync(p), text)) problems.push(`differs: ${path.relative(root, p)}`);
  }
  for (const p of listExisting()) if (!files.has(p)) problems.push(`stale: ${path.relative(root, p)}`);
  if (problems.length) {
    console.error(problems.join('\n'));
    console.error('Run: node scripts/build-storymaker-catalog.mjs');
    process.exit(1);
  }
  console.log(`storymaker catalog ${built.manifest.version} is up to date`);
}

function findChrome(explicit) {
  const candidates = [
    explicit, process.env.CHROME_PATH,
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
    '/usr/bin/google-chrome', '/usr/bin/chromium', '/usr/bin/chromium-browser',
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  ].filter(Boolean);
  return candidates.find((c) => fs.existsSync(c));
}

function review(dir, chromeArg) {
  fs.mkdirSync(dir, { recursive: true });
  const { built } = expectedFiles();
  const sheets = [['cast-sheet', castSheet(built.rig)], ['variety-sheet', varietySheet(built.rig)], ['env-prop-sheet', envPropSheet(built.rig)]];
  const chrome = findChrome(chromeArg);
  for (const [name, sheet] of sheets) {
    const svgPath = path.resolve(dir, `${name}.svg`);
    fs.writeFileSync(svgPath, sheet.svg, 'utf8');
    console.log(`wrote ${svgPath} (${sheet.width} x ${sheet.height})`);
    if (!chrome) continue;
    const png = path.resolve(dir, `${name}.png`);
    const r = spawnSync(chrome, ['--headless=new', '--disable-gpu', '--hide-scrollbars', '--force-device-scale-factor=1', `--window-size=${sheet.width},${sheet.height}`, `--screenshot=${png}`, `file:///${svgPath.replace(/\\/g, '/')}`], { stdio: 'ignore' });
    console.log(r.status === 0 && fs.existsSync(png) ? `wrote ${png}` : `chrome failed for ${name}`);
  }
  if (!chrome) console.log('no Chrome found: pass --chrome <path> or set CHROME_PATH to also render PNGs');
}

const args = process.argv.slice(2);
if (args.includes('--check')) check();
else if (args.includes('--review')) review(path.resolve(args[args.indexOf('--review') + 1] || 'disposable/storymaker-review'), args.includes('--chrome') ? args[args.indexOf('--chrome') + 1] : undefined);
else write();
