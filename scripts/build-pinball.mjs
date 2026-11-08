import { build } from 'esbuild';
import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';

process.chdir(fileURLToPath(new URL('../', import.meta.url)));
const check = process.argv.includes('--check'), output = 'ui/js/vendor/pinball';
const three = JSON.parse(await fs.readFile('node_modules/three/package.json'));
if (three.version !== '0.186.1') throw Error('AuraPinball requires pinned Three.js 0.186.1');
const result = await build({ entryPoints: ['ui/js/desktop/apps/pinball/game.js'], bundle: true, format: 'esm', target: 'es2022', minify: true, legalComments: 'eof', write: false });
const bytes = result.outputFiles[0].contents;
if (bytes.length > 3 * 1024 * 1024) throw Error('Pinball runtime exceeds 3 MiB');
const license = ('Three.js (MIT)\n\n' + await fs.readFile('node_modules/three/LICENSE', 'utf8')).replace(/\r\n?/g, '\n');
const files = { 'game.esm.js': bytes, 'LICENSE.txt': license, 'manifest.json': JSON.stringify({ three: three.version, entry: 'ui/js/desktop/apps/pinball/game.js', bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }, null, 2) + '\n' };
for (const [name, content] of Object.entries(files)) {
    const filename = output + '/' + name;
    if (check) { if (!(await fs.readFile(filename)).equals(Buffer.from(content))) throw Error('Rebuild ' + filename); }
    else { await fs.mkdir(output, { recursive: true }); await fs.writeFile(filename, content); }
}
console.log('AuraPinball runtime ' + (check ? 'verified' : 'built') + ': ' + bytes.length + ' bytes');
