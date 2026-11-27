// Builds assets/storymaker/icons/{lucide.json,LICENSE.txt,NOTICE.txt} from a lucide-static package and the allow-list.
//   node scripts/build-storymaker-icons.mjs --verify <lucide-static.tgz>     checks the sha512 integrity
//   node scripts/build-storymaker-icons.mjs --package <dir>                  writes the files
//   node scripts/build-storymaker-icons.mjs --check --package <dir>          compares without writing
// The package is data: only icon-nodes.json, tags.json and LICENSE are read.

import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildIconData, LUCIDE_INTEGRITY } from '../assets/storymaker/lib/icons.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const outDir = path.join(root, 'assets', 'storymaker', 'icons');

const args = process.argv.slice(2);
const arg = (name) => { const i = args.indexOf(name); return i < 0 ? null : args[i + 1] ?? ''; };
const fail = (msg) => { console.error(msg); process.exit(1); };

if (args.includes('--verify')) {
    const tgz = arg('--verify');
    if (!tgz) fail('--verify needs the path of the tarball');
    const got = `sha512-${crypto.createHash('sha512').update(fs.readFileSync(tgz)).digest('base64')}`;
    if (got !== LUCIDE_INTEGRITY) fail(`integrity mismatch: ${got}`);
    console.log('integrity ok');
    if (!arg('--package')) process.exit(0);
}

const pkg = arg('--package');
if (!pkg) fail('usage: --package <dir> [--check] | --verify <tgz>');

const allowList = JSON.parse(fs.readFileSync(path.join(outDir, 'allow-list.json'), 'utf8'));
const built = buildIconData(pkg, allowList);
if (built.problems.length) fail(`icon build failed:\n${built.problems.join('\n')}`);
const outputs = { 'lucide.json': built.json, 'LICENSE.txt': built.license, 'NOTICE.txt': built.notice };

let stale = 0;
for (const [name, text] of Object.entries(outputs)) {
    const file = path.join(outDir, name);
    if (args.includes('--check')) {
        const have = fs.existsSync(file) ? fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n') : null;
        if (have !== text) { console.error(`stale: assets/storymaker/icons/${name}`); stale++; }
    } else {
        fs.writeFileSync(file, text);
    }
}
if (stale) process.exit(1);
console.log(args.includes('--check') ? `icons up to date (${built.count})` : `wrote ${built.count} icons`);
