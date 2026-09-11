import {execFileSync} from 'node:child_process';
import {statSync} from 'node:fs';
import {join} from 'node:path';

const inventory = execFileSync('go', ['list', '-deps', '-f', '{{.ImportPath}}|{{.Dir}}|{{join .EmbedFiles ";"}}', './cmd/aurago'], {encoding:'utf8'});
let embedded = 0;
for (const line of inventory.trim().split(/\r?\n/)) {
    const [pkg, dir, names] = line.split('|');
    if (!pkg.startsWith('aurago/') || !names) continue;
    const size = names.split(';').reduce((sum, name) => sum + statSync(join(dir,name)).size, 0);
    embedded += size;
    console.log(`${pkg}: ${size} embedded bytes`);
    if (pkg === 'aurago/ui' || (pkg === 'aurago/internal/server' && size > 1_000_000)) throw new Error('Full UI or oversized recovery page embedded');
}
if (embedded > 10_000_000) throw new Error(`Embedded first-party resources exceed 10 MB: ${embedded}`);
for (const binary of process.argv.slice(2)) {
    const size = statSync(binary).size;
    console.log(`${binary}: ${(size/1e6).toFixed(2)} MB`);
    if (size > 120_000_000) throw new Error(`Stripped binary exceeds 120 MB: ${binary}`);
}
console.log(`Embedded resource budget OK: ${(embedded/1e6).toFixed(2)} MB`);
