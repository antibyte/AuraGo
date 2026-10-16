import { build } from 'esbuild';
import { readFile, writeFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';

const check = process.argv.includes('--check');
const root = 'ui/js/vendor/foliate/';
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
const result = await build({ entryPoints: ['ui/js/desktop/apps/bookshelf/reader-engine.js'],
    bundle: true, format: 'iife', target: 'es2022', minify: true, write: false,
    legalComments: 'inline', plugins: [{ name: 'dedicated-pdf-reader', setup(builder) {
        builder.onResolve({ filter: /^\.\/pdf\.js$/ }, () => ({ path: 'pdf', namespace: 'bookshelf' }));
        builder.onLoad({ filter: /.*/, namespace: 'bookshelf' }, () => ({ contents: 'export function makePDF(){throw new Error("Use the dedicated PDF reader")}' }));
    } }] });
const data = Buffer.from(result.outputFiles[0].contents);
const target = root + 'reader.bundle.js';
const files = {};
async function hashes(directory) {
    for (const entry of await readdir(root + directory, { withFileTypes: true })) {
        const name = directory + entry.name;
        if (entry.isDirectory()) await hashes(name + '/');
        else files[name] = digest(await readFile(root + name));
    }
}
await hashes('source/');
files['reader.bundle.js'] = digest(data);
const manifest = JSON.stringify({ source: 'https://github.com/johnfactotum/foliate-js',
    revision: '78914aef4466eb960965702401634c2cb348e9b1', license: 'MIT',
    dependencies: { '@zip.js/zip.js': '2.8.22', fflate: '0.8.2', dompurify: '3.4.17' }, files }, null, 2) + '\n';
if (check) {
    if (!(await readFile(target)).equals(data) || await readFile(root + 'manifest.json', 'utf8') !== manifest) {
        throw new Error('Bookshelf reader assets are stale; run node scripts/build-bookshelf-reader.mjs');
    }
    console.log('Bookshelf reader assets verified');
} else {
    await writeFile(target, data); await writeFile(root + 'manifest.json', manifest);
    console.log(`Bookshelf reader: ${data.length} bytes`);
}
