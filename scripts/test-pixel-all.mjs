// Runs every scripts/test-pixel-*.mjs (except this file) one after the other and reports all failures at the end.
// New work packages only add their own scripts/test-pixel-<area>.mjs; nothing here needs to change.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const dir = path.dirname(fileURLToPath(import.meta.url));
const self = path.basename(fileURLToPath(import.meta.url));
const tests = fs.readdirSync(dir)
    .filter(name => /^test-pixel-.+\.mjs$/.test(name) && name !== self)
    .sort();

if (!tests.length) {
    console.error('test-pixel-all: no scripts/test-pixel-*.mjs found');
    process.exit(1);
}

const failed = [];
for (const name of tests) {
    console.log('--- ' + name);
    const result = spawnSync(process.execPath, [path.join(dir, name)], { stdio: 'inherit' });
    if (result.status !== 0) {
        const why = result.error ? result.error.message : 'exit ' + result.status;
        console.error('test-pixel-all: ' + name + ' failed (' + why + ')');
        failed.push(name + ' (' + why + ')');
    }
}
if (failed.length) {
    console.error('test-pixel-all: ' + failed.length + ' of ' + tests.length + ' scripts failed:');
    failed.forEach(entry => console.error('  - ' + entry));
    process.exit(1);
}
console.log('test-pixel-all: ' + tests.length + ' scripts passed');
