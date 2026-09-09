import { mkdir, readFile, writeFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { rollup } from 'rollup';
import { nodeResolve } from '@rollup/plugin-node-resolve';

const out = 'ui/js/vendor/writer';
const check = process.argv.includes('--check');
const outputs = new Map();
const bundle = await rollup({
    input: 'scripts/writer-engine-entry.js',
    plugins: [nodeResolve({ browser: true }), {
        name: 'writer-local-assets',
        resolveImportMeta(property, { moduleId }) {
            if (property !== 'url') return null;
            const base = moduleId.replaceAll('\\', '/').includes('/fonts/') ? 'fonts/module.js' : 'engine.js';
            return `new URL('/js/vendor/writer/${base}', document.baseURI).href`;
        },
    }],
    onwarn(warning, warn) {
        if (['CIRCULAR_DEPENDENCY', 'MODULE_LEVEL_DIRECTIVE'].includes(warning.code)) return;
        if (warning.code === 'UNRESOLVED_IMPORT') throw new Error(warning.message);
        warn(warning);
    },
});
try {
    const generated = await bundle.generate({ format: 'es', sourcemap: false, inlineDynamicImports: true });
    outputs.set('engine.js', Buffer.from(generated.output[0].code.replace(/[ \t]+$/gm, '')));
    outputs.set('engine.css', await readFile('node_modules/@docx-editor.dev/core/dist/editor.css'));
    outputs.set('harfbuzz.wasm', await readFile('node_modules/@docx-editor.dev/core/dist/harfbuzz.wasm'));
    for (const file of await readdir('node_modules/@docx-editor.dev/fonts/assets')) {
        outputs.set(`assets/${file}`, await readFile(`node_modules/@docx-editor.dev/fonts/assets/${file}`));
    }
    const packages = new Set(['@docx-editor.dev/core', '@docx-editor.dev/fonts']);
    for (const id of bundle.watchFiles) {
        const rest = id.replaceAll('\\', '/').split('/node_modules/')[1];
        if (rest) packages.add(rest.startsWith('@') ? rest.split('/').slice(0, 2).join('/') : rest.split('/')[0]);
    }
    const notices = ['# Autor editor: third-party licenses\n\nAuraGo host code remains MIT. No Pro or AGPL packages are included.\n'];
    const versions = {};
    for (const name of [...packages].sort()) {
        const root = `node_modules/${name}`;
        const pkg = JSON.parse(await readFile(`${root}/package.json`, 'utf8'));
        if (/AGPL|GPL-|commercial|proprietary/i.test(pkg.license || '')) throw new Error(`Disallowed license: ${name}: ${pkg.license}`);
        versions[name] = { version: pkg.version, license: pkg.license };
        notices.push(`\n## ${name} ${pkg.version} — ${pkg.license}\n`);
        for (const file of await readdir(root, { withFileTypes: true })) {
            if (file.isFile() && /^(license|copying|third.party.notices)/i.test(file.name)) notices.push(await readFile(`${root}/${file.name}`, 'utf8'));
        }
        try {
            for (const file of await readdir(`${root}/licenses`)) notices.push(await readFile(`${root}/licenses/${file}`, 'utf8'));
        } catch (error) { if (error.code !== 'ENOENT') throw error; }
    }
    outputs.set('LICENSES.md', Buffer.from(notices.join('\n')));
    const files = {};
    for (const [name, data] of outputs) files[name] = { bytes: data.length, sha256: createHash('sha256').update(data).digest('hex') };
    outputs.set('manifest.json', Buffer.from(JSON.stringify({ version: 1, packages: versions, files }, null, 2) + '\n'));
    for (const [name, data] of outputs) {
        const file = path.join(out, name);
        if (check) {
            if (!data.equals(await readFile(file))) throw new Error(`Stale Writer asset: ${file}`);
        } else {
            await mkdir(path.dirname(file), { recursive: true });
            await writeFile(file, data);
        }
    }
    console.log(`Writer vendor ${check ? 'verified' : 'built'}: ${outputs.size} local assets`);
} finally { await bundle.close(); }
