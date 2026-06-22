import { nodeResolve } from '@rollup/plugin-node-resolve';
import { rollup } from 'rollup';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

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

async function copyPackageFile(name, relPath, outputPath) {
  const filePath = await assertPackage(name, relPath);
  await fs.copyFile(filePath, outputPath);
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
await bundle.write({
  file: path.join(vendorDir, 'chess-vendor.esm.js'),
  format: 'es',
  sourcemap: false,
});
await bundle.close();

const chessVendorPath = path.join(vendorDir, 'chess-vendor.esm.js');
const chessVendorSource = await fs.readFile(chessVendorPath, 'utf8');
await fs.writeFile(chessVendorPath, normalizeGeneratedIndent(chessVendorSource), 'utf8');

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
await fs.writeFile(path.join(cssOutDir, 'cm-chessboard.css'), `${css.join('\n\n')}\n`, 'utf8');

const standardPieces = await readPackageText('cm-chessboard', 'assets/pieces/standard.svg');
await fs.writeFile(
  path.join(imageOutDir, 'standard.svg'),
  standardPieces.replace('LICENSE\n=======', 'License\n-------'),
  'utf8',
);
await copyPackageFile('cm-chessboard', 'assets/extensions/markers/markers.svg', path.join(imageOutDir, 'markers.svg'));
await copyPackageFile('stockfish', 'bin/stockfish-18-lite-single.js', path.join(stockfishOutDir, 'stockfish-18-lite-single.js'));
await copyPackageFile('stockfish', 'bin/stockfish-18-lite-single.wasm', path.join(stockfishOutDir, 'stockfish-18-lite-single.wasm'));
await copyPackageFile('stockfish', 'Copying.txt', path.join(stockfishOutDir, 'Copying.txt'));

console.log('Built chess vendor assets.');
