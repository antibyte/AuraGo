import { build } from 'esbuild';
import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';

const check = process.argv.includes('--check');
const pkg = JSON.parse(await fs.readFile('node_modules/three/package.json', 'utf8'));
if (pkg.version !== '0.185.1') throw Error('Tiefsee screensaver requires Three.js 0.185.1');
const output = 'ui/js/vendor/screensaver-abyss';
const entry = 'ui/js/desktop/screensavers/abyss-scene.js';
const result = await build({
  entryPoints: [entry], bundle: true, format: 'esm',
  target: 'es2022', minify: true, legalComments: 'eof', write: false,
});
const bytes = result.outputFiles[0].contents;
const license = await fs.readFile('node_modules/three/LICENSE', 'utf8');
const files = {
  'abyss.esm.js': bytes,
  'LICENSE.txt': license,
  'manifest.json': JSON.stringify({ three: pkg.version, license: 'MIT', entry, bytes: bytes.length,
    sha256: createHash('sha256').update(bytes).digest('hex') }, null, 2) + '\n',
};
for (const [name, content] of Object.entries(files)) {
  const path = output + '/' + name;
  if (check) {
    if (!(await fs.readFile(path)).equals(Buffer.from(content))) throw Error('Rebuild ' + path);
  } else {
    await fs.mkdir(output, { recursive: true }); await fs.writeFile(path, content);
  }
}
console.log('Tiefsee screensaver renderer ' + (check ? 'verified' : 'built') + ': ' + bytes.length + ' bytes');
