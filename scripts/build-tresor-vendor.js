import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = path.join(root, 'node_modules', 'argon2id');
const target = path.join(root, 'ui', 'js', 'vendor', 'tresor-argon2');
const files = ['lib/setup.js', 'lib/argon2id.js', 'lib/blake2b.js', 'dist/simd.wasm', 'dist/no-simd.wasm', 'LICENSE'];
const pkg = JSON.parse(fs.readFileSync(path.join(source, 'package.json'), 'utf8'));
if (pkg.version !== '1.0.1') throw new Error('Tresor Argon2id version must be 1.0.1');
const check = process.argv.includes('--check');
for (const file of files) {
    const src = fs.readFileSync(path.join(source, file));
    const dest = path.join(target, file === 'LICENSE' ? 'LICENSE.txt' : path.basename(file));
    if (check) {
        if (!fs.existsSync(dest) || !src.equals(fs.readFileSync(dest))) throw new Error(`Tresor vendor drift: ${file}`);
    } else {
        fs.mkdirSync(target, { recursive: true });
        fs.writeFileSync(dest, src);
    }
}
