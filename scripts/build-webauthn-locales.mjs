import assert from 'node:assert/strict';
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const locales = ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh'];
const messages = {};
let keys;
for (const locale of locales) {
    const raw = await readFile(path.join(root, 'ui/lang/common', locale + '.json'), 'utf8');
    const names = [...raw.matchAll(/^\s*"(webauthn\.[^"]+)":/gm)].map(match => match[1]);
    assert.equal(new Set(names).size, names.length, locale + ' duplicate WebAuthn keys');
    const source = JSON.parse(raw);
    const entries = Object.entries(source).filter(([key]) => key.startsWith('webauthn.') || key === 'common.btn_cancel').sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0);
    assert.ok(entries.length > 0, 'WebAuthn translations are required');
    keys ??= entries.map(([key]) => key);
    assert.deepEqual(entries.map(([key]) => key), keys, locale + ' translation keys');
    for (const [key, value] of entries) assert.ok(typeof value === 'string' && value.trim(), locale + ': ' + key);
    messages[locale] = Object.fromEntries(entries);
}
const target = path.join(root, 'internal/server/auth_webauthn_locales.json');
const output = JSON.stringify(messages) + '\n';
if (process.argv.includes('--check')) {
    assert.equal(await readFile(target, 'utf8'), output, 'Run node scripts/build-webauthn-locales.mjs');
} else {
    await writeFile(target, output);
}
process.stdout.write('WebAuthn recovery translations: 16 locales verified.\n');
