import { build } from 'esbuild';
import fs from 'node:fs/promises';
import { createHash } from 'node:crypto';

const check = process.argv.includes('--check');
const pkg = JSON.parse(await fs.readFile('node_modules/three/package.json', 'utf8'));
if (pkg.version !== '0.185.1') throw Error('Game Maker requires Three.js 0.185.1');
const result = await build({
    entryPoints: ['assets/game-maker-low-poly/runtime.js'], bundle: true, format: 'esm',
    target: 'es2022', minify: true, legalComments: 'eof', write: false,
    plugins: [{ name: 'shared-three', setup(builder) {
        builder.onResolve({ filter: /^three$/ }, () => ({ path: './three-0.185.1.module.min.js', external: true }));
    } }],
});
const bytes = result.outputFiles[0].contents;
const path = 'internal/gamemaker/runtime/aurago-three-assets-1.js';
if (check) {
    if (!(await fs.readFile(path)).equals(Buffer.from(bytes))) throw Error('Rebuild ' + path);
} else await fs.writeFile(path, bytes);
console.log('Game Maker 3D runtime ' + (check ? 'verified' : 'built') + ': ' + bytes.length + ' bytes, ' + createHash('sha256').update(bytes).digest('hex'));
