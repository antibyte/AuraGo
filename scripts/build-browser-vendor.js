import { build, transform } from 'esbuild';
import { readFile, writeFile, mkdir, readdir } from 'node:fs/promises';
import path from 'node:path';
import { createHash } from 'node:crypto';
import * as three from 'three';

// All inputs are installed by npm ci; production never downloads browser code.
const check = process.argv.includes('--check');
const outputs = new Map(), packages = new Set();
const vendor = 'ui/js/vendor/';
const digest = data => createHash('sha256').update(data).digest('hex');
async function copy(name, source, target) {
    packages.add(name);
    outputs.set(target, await readFile(`node_modules/${name}/${source}`));
}
async function tree(name, source, target) {
    for (const entry of await readdir(`node_modules/${name}/${source}`, { withFileTypes: true })) {
        if (entry.isDirectory()) await tree(name, `${source}/${entry.name}`, `${target}/${entry.name}`);
        else if (!entry.name.endsWith('.map')) await copy(name, `${source}/${entry.name}`, `${target}/${entry.name}`);
    }
}
const sharedThree = {
    name: 'shared-three', setup(builder) {
        builder.onResolve({ filter: /^three$/ }, () => ({ path: 'three', namespace: 'global-three' }));
        builder.onLoad({ filter: /.*/, namespace: 'global-three' }, () => ({ contents:
            Object.keys(three).map(name => `export const ${name}=window.THREE.${name};`).join('\n') }));
    },
};
async function bundle(name, contents, target, plugins = [], format = 'iife') {
    packages.add(name);
    const result = await build({ stdin: { contents, resolveDir: process.cwd() }, bundle: true,
        format, target: 'es2022', minify: true, legalComments: 'inline', write: false,
        metafile: true, plugins, define: { 'process.env.NODE_ENV': '"production"',
            ...(format === 'iife' ? { 'import.meta.url': '__auragoVendorURL' } : {}) },
        ...(format === 'iife' ? { banner: { js: '(()=>{const __auragoVendorURL=document.currentScript.src;' }, footer: { js: '})();' } } : {}) });
    outputs.set(target, Buffer.from(result.outputFiles[0].contents));
    for (const file of Object.keys(result.metafile.inputs)) {
        const rest = file.replaceAll('\\', '/').split('node_modules/')[1];
        if (rest) packages.add(rest.startsWith('@') ? rest.split('/').slice(0, 2).join('/') : rest.split('/')[0]);
    }
}

for (const [name, source, target] of [
    ['chart.js', 'dist/chart.umd.min.js', 'ui/chart.min.js'],
    ['marked', 'lib/marked.umd.js', vendor + 'marked.min.js'],
    ['@highlightjs/cdn-assets', 'highlight.min.js', vendor + 'highlight.min.js'],
    ['@highlightjs/cdn-assets', 'styles/github.min.css', 'ui/css/hljs-github.min.css'],
    ['@highlightjs/cdn-assets', 'styles/github-dark.min.css', 'ui/css/hljs-github-dark.min.css'],
    ['dompurify', 'dist/purify.min.js', vendor + 'purify.min.js'],
    ['hls.js', 'dist/hls.min.js', vendor + 'hls.min.js'],
    ['force-graph', 'dist/force-graph.min.js', vendor + 'force-graph.min.js'],
    ['@xterm/xterm', 'lib/xterm.js', vendor + 'xterm.min.js'],
    ['@xterm/xterm', 'css/xterm.css', 'ui/css/xterm.css'],
    ['@xterm/addon-fit', 'lib/addon-fit.js', vendor + 'xterm-addon-fit.min.js'],
    ['@xterm/addon-webgl', 'lib/addon-webgl.js', vendor + 'xterm-addon-webgl.min.js'],
    ['@rive-app/canvas', 'rive.js', vendor + 'rive/rive.js'],
    ['@rive-app/canvas', 'rive.wasm', vendor + 'rive/rive.wasm'],
    ['webamp', 'built/webamp.bundle.min.mjs', vendor + 'webamp/webamp.bundle.min.mjs'],
    ['pdfjs-dist', 'build/pdf.min.mjs', vendor + 'pdf.module.js'],
    ['pdfjs-dist', 'build/pdf.worker.min.mjs', vendor + 'pdf.worker.min.js'],
]) await copy(name, source, target);
for (const [source, target] of [['three.module.js', 'three-0.186.1.module.min.js'], ['three.core.js', 'three-0.186.1.core.min.js']]) {
    const input = (await readFile('node_modules/three/build/' + source, 'utf8')).replaceAll('./three.core.js', './three-0.186.1.core.min.js');
    const result = await transform(input, { minify: true, format: 'esm', legalComments: 'inline' });
    outputs.set('internal/gamemaker/runtime/' + target, Buffer.from(result.code));
}
await tree('three', 'examples/jsm/libs/draco/gltf', vendor + 'draco');
await tree('esp-web-tools', 'dist/web', vendor + 'esp-web-tools');
for (const dir of ['cmaps', 'standard_fonts', 'wasm', 'iccs']) await tree('pdfjs-dist', dir, vendor + 'pdf/' + dir);
await bundle('three', "import * as THREE from 'three'; window.THREE={...THREE};", vendor + 'three.min.js');
await bundle('markdown-it', "import markdownit from 'markdown-it'; window.markdownit=markdownit;", vendor + 'markdown-it.min.js');
for (const [name, dir] of [['GLTFLoader', 'loaders'], ['STLLoader', 'loaders'], ['DRACOLoader', 'loaders'], ['OrbitControls', 'controls']]) {
    await bundle('three', `import {${name}} from 'three/addons/${dir}/${name}.js'; window.THREE.${name}=${name};`, vendor + name + '.min.js', [sharedThree]);
}
await bundle('3d-force-graph', "import ForceGraph3D from '3d-force-graph'; window.ForceGraph3D=ForceGraph3D;", vendor + '3d-force-graph.min.js', [sharedThree]);
await bundle('mermaid', "import mermaid from 'mermaid'; window.mermaid=mermaid;", vendor + 'mermaid.min.js');
await bundle('@novnc/novnc', "import RFB from '@novnc/novnc'; window.RFB=RFB;", vendor + 'novnc.min.js', [], 'esm');

// Await module initialization before Desktop loads the app that consumes it.
outputs.set(vendor + 'pdf.min.js', Buffer.from(`const source = new URL(import.meta.url);
const url = new URL('pdf.module.js', source); url.search = source.search;
const worker = new URL('pdf.worker.min.js', source); worker.search = source.search;
const lib = await import(url.href);
lib.GlobalWorkerOptions.workerSrc = worker.href;
window.pdfjsLib = lib;\n`));
const lock = JSON.parse(await readFile('package-lock.json', 'utf8'));
for (const [name, target, assets] of [
    ['@rive-app/canvas', 'rive', ['rive.js', 'rive.wasm']],
    ['webamp', 'webamp', ['webamp.bundle.min.mjs']],
    ['3d-force-graph', '3d-force-graph', ['../3d-force-graph.min.js']],
]) {
    const pkg = JSON.parse(await readFile(`node_modules/${name}/package.json`, 'utf8'));
    const files = {};
    for (const asset of assets) {
        const data = outputs.get(path.posix.normalize(`${vendor}${target}/${asset}`));
        files[asset] = { bytes: data.length, sha256: digest(data) };
    }
    outputs.set(`${vendor}${target}/manifest.json`, Buffer.from(JSON.stringify({
        name, version: pkg.version, license: pkg.license,
        source: name === 'webamp' ? 'https://github.com/captbaritone/webamp' : `https://www.npmjs.com/package/${name}/v/${pkg.version}`,
        npm_integrity: lock.packages['node_modules/' + name].integrity,
        ...(name === 'webamp' ? { bundle: assets[0], bundle_type: 'core-esm' } : {}),
        ...(name === '3d-force-graph' ? { shared_three: '0.186.1' } : {}), files,
    }, null, 2) + '\n'));
}
outputs.set(vendor + 'esp-web-tools/README.md', Buffer.from(
    `Vendored [ESP Web Tools](https://github.com/esphome/esp-web-tools) ${lock.packages['node_modules/esp-web-tools'].version} (\`dist/web\`).\n` +
    'Used by the Cheap Yellow Display config flasher. Apache-2.0, see LICENSE.txt.\n' +
    'Regenerate with `npm run build:browser-vendor`.\n'));
const versions = {}, notices = ['# Browser vendor licenses\n\nRebuild with `npm run build:browser-vendor`. Versions are locked in package-lock.json.\n'];
for (const name of [...packages].sort()) {
    const root = 'node_modules/' + name;
    const pkg = JSON.parse(await readFile(root + '/package.json', 'utf8'));
    versions[name] = { version: pkg.version, license: pkg.license };
    notices.push(`\n## ${name} ${pkg.version} — ${pkg.license || 'See upstream notice'}\n`);
    for (const file of await readdir(root, { withFileTypes: true })) {
        if (file.isFile() && /^(license|copying|notice)/i.test(file.name)) notices.push(await readFile(root + '/' + file.name, 'utf8'));
    }
}
outputs.set(vendor + 'LICENSES.md', Buffer.from(notices.join('\n').replaceAll('\r\n', '\n')));
const files = {};
for (const [name, data] of outputs) files[name] = { bytes: data.length, sha256: digest(data) };
outputs.set(vendor + 'manifest.json', Buffer.from(JSON.stringify({ packages: versions, files }, null, 2) + '\n'));
for (const [name, data] of outputs) {
    const current = await readFile(name).catch(error => {
        if (error.code !== 'ENOENT') throw error;
        return null;
    });
    if (check) {
        if (!current || !data.equals(current)) throw Error('Stale browser vendor asset: ' + name);
    } else if (!current || !data.equals(current)) {
        await mkdir(path.dirname(name), { recursive: true });
        await writeFile(name, data);
    }
}
console.log(`Browser vendor ${check ? 'verified' : 'built'}: ${outputs.size} assets`);
