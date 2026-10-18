import { build } from 'esbuild';
import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';

const check = process.argv.includes('--check');
const output = 'ui/js/vendor/minigolf';
const three = JSON.parse(await fs.readFile('node_modules/three/package.json'));
const rapier = JSON.parse(await fs.readFile('node_modules/@dimforge/rapier3d-compat/package.json'));
if (three.version !== '0.186.1' || rapier.version !== '0.21.0') throw Error('AuraGolf requires pinned Three.js 0.186.1 and Rapier 0.21.0');
const result = await build({ entryPoints: ['ui/js/desktop/apps/minigolf-game.js'], bundle: true, format: 'esm', target: 'es2022', minify: true, legalComments: 'eof', write: false });
const bytes = result.outputFiles[0].contents;
if (bytes.length > 5 * 1024 * 1024) throw Error('AuraGolf runtime exceeds 5 MiB');
const license = 'Three.js (MIT)\n\n' + await fs.readFile('node_modules/three/LICENSE', 'utf8') + '\n\nRapier (Apache-2.0)\n\n' + await fs.readFile('node_modules/@dimforge/rapier3d-compat/LICENSE', 'utf8');
const files = { 'game.esm.js': bytes, 'LICENSE.txt': license, 'manifest.json': JSON.stringify({ three: three.version, rapier: rapier.version, entry: 'ui/js/desktop/apps/minigolf-game.js', bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }, null, 2) + '\n' };
for (const [name, content] of Object.entries(files)) {
    const filename = output + '/' + name;
    if (check) { if (!(await fs.readFile(filename)).equals(Buffer.from(content))) throw Error('Rebuild ' + filename); }
    else { await fs.mkdir(output, { recursive: true }); await fs.writeFile(filename, content); }
}
console.log('AuraGolf runtime ' + (check ? 'verified' : 'built') + ': ' + bytes.length + ' bytes');
