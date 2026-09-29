import { nodeResolve } from '@rollup/plugin-node-resolve';
import { rollup } from 'rollup';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const check = process.argv.includes('--check');
const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, '..');
const nodeModules = path.join(repoRoot, 'node_modules');
const disposableDir = path.join(repoRoot, 'disposable', 'chess-vendor');
const vendorDir = path.join(repoRoot, 'ui', 'js', 'vendor');
const stockfishOutDir = path.join(vendorDir, 'stockfish');
const cssOutDir = path.join(repoRoot, 'ui', 'css');
const imageOutDir = path.join(repoRoot, 'ui', 'img', 'chess');

async function assertPackage(name, relPath = '') {
  const packagePath = path.join(nodeModules, name, relPath);
  try {
    await fs.access(packagePath);
  } catch {
    throw new Error(`Missing ${name}. Run npm install before building chess vendor assets.`);
  }
  return packagePath;
}

function stripSourceMapComment(source) {
  return source.replace(/\n?\/\*# sourceMappingURL=.*?\*\/\s*$/u, '').trimEnd();
}

function normalizeGeneratedIndent(source) {
  return source.replace(/^[ \t]+/gmu, (indent) => indent.replace(/\t/gu, '  '));
}

async function readPackageText(name, relPath) {
  const filePath = await assertPackage(name, relPath);
  return fs.readFile(filePath, 'utf8');
}

async function writeAsset(outputPath, content) {
  const data = Buffer.from(content);
  const current = await fs.readFile(outputPath).catch(error => { if (error.code !== 'ENOENT') throw error; return Buffer.alloc(0); });
  if (data.equals(current)) return;
  if (check) throw Error('Stale chess vendor asset: ' + outputPath);
  const temporary = outputPath + '.tmp';
  try { await fs.writeFile(temporary, data); await fs.rename(temporary, outputPath); }
  finally { await fs.rm(temporary, { force: true }); }
}

async function copyPackageFile(name, relPath, outputPath) {
  const filePath = await assertPackage(name, relPath);
  await writeAsset(outputPath, await fs.readFile(filePath));
}

await fs.mkdir(disposableDir, { recursive: true });
await fs.mkdir(vendorDir, { recursive: true });
await fs.mkdir(stockfishOutDir, { recursive: true });
await fs.mkdir(cssOutDir, { recursive: true });
await fs.mkdir(imageOutDir, { recursive: true });

const entryPath = path.join(disposableDir, 'entry.js');
await fs.writeFile(
  entryPath,
  [
    "export { Chessboard, COLOR, INPUT_EVENT_TYPE, BORDER_TYPE } from 'cm-chessboard/src/Chessboard.js';",
    "export { Markers, MARKER_TYPE } from 'cm-chessboard/src/extensions/markers/Markers.js';",
    "export { PromotionDialog } from 'cm-chessboard/src/extensions/promotion-dialog/PromotionDialog.js';",
    "export { Chess } from 'chess.js';",
    '',
  ].join('\n'),
  'utf8',
);

const bundle = await rollup({
  input: entryPath,
  plugins: [nodeResolve({ browser: true })],
});
const generated = await bundle.generate({ format: 'es', sourcemap: false });
await bundle.close();
await writeAsset(path.join(vendorDir, 'chess-vendor.esm.js'), normalizeGeneratedIndent(generated.output[0].code));

const cssParts = [
  ['cm-chessboard', 'assets/chessboard.css'],
  ['cm-chessboard markers', 'assets/extensions/markers/markers.css'],
  ['cm-chessboard promotion dialog', 'assets/extensions/promotion-dialog/promotion-dialog.css'],
];
const css = [];
for (const [label, relPath] of cssParts) {
  const source = await readPackageText('cm-chessboard', relPath);
  css.push(`/* ${label}: ${relPath} */\n${stripSourceMapComment(source)}`);
}
await writeAsset(path.join(cssOutDir, 'cm-chessboard.css'), `${css.join('\n\n')}\n`);

const standardPieces = await readPackageText('cm-chessboard', 'assets/pieces/standard.svg');
await writeAsset(
  path.join(imageOutDir, 'standard.svg'),
  standardPieces.replace('LICENSE\n=======', 'License\n-------'),
);
await copyPackageFile('cm-chessboard', 'assets/extensions/markers/markers.svg', path.join(imageOutDir, 'markers.svg'));
await copyPackageFile('stockfish', 'bin/stockfish-19-lite-single.js', path.join(stockfishOutDir, 'stockfish-19-lite-single.js'));
await copyPackageFile('stockfish', 'bin/stockfish-19-lite-single.wasm', path.join(stockfishOutDir, 'stockfish-19-lite-single.wasm'));
await writeAsset(path.join(stockfishOutDir, 'Copying.txt'), (await readPackageText('stockfish', 'Copying.txt')).replaceAll('\r\n', '\n'));

console.log(`Chess vendor ${check ? 'verified' : 'built'}.`);
