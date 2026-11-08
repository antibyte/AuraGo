// Runs the pinball suites in order. Missing suites are skipped with a note; the first failure stops the run.
import { existsSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const SUITES = [
    'config', 'rules', 'input', 'physics', 'devices', 'paths', 'tables', 'reach',
    'layout-neon', 'layout-leviathan', 'layout-temple', 'pack-neon', 'pack-leviathan', 'pack-temple',
    'lights', 'dmd', 'audio', 'music', 'session', 'assets'
];

for (const name of SUITES) {
    const file = fileURLToPath(new URL('./test-pinball-' + name + '.mjs', import.meta.url));
    if (!existsSync(file)) { console.log('skip  ' + name + ' (scripts/test-pinball-' + name + '.mjs not present)'); continue; }
    console.log('run   ' + name);
    const result = spawnSync(process.execPath, [file], { stdio: 'inherit' });
    if (result.status !== 0) {
        console.error('FAIL  ' + name + (result.error ? ': ' + result.error.message : ' (exit ' + result.status + ')'));
        process.exit(result.status || 1);
    }
}
console.log('pinball suites ok');
