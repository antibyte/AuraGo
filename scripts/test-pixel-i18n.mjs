// Guards the English bundle: every Pixel key the scripts use (literal keys and the keys built at run time) must have its
// English text in ui/lang/desktop/en.json. Fix a failure by adding the key to en.json (plain t('pixel.key') calls carry no
// inline text); node scripts/pixel-i18n-extract.mjs --write inserts keys whose English text a registry object or helper still carries.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const dir = path.dirname(fileURLToPath(import.meta.url));
const result = spawnSync(process.execPath, [path.join(dir, 'pixel-i18n-extract.mjs')], { encoding: 'utf8' });
assert.equal(result.status, 0, 'Pixel i18n keys are missing from en.json:\n' + result.stdout + result.stderr);
assert.match(result.stdout, /missing with English: 0, missing without English: 0/);

// The key list for the translators stays a sorted key -> English object.
const listFile = path.join(dir, '..', 'docs', 'superpowers', 'plans', '2026-10-21-pixel-pro-new-keys.json');
if (fs.existsSync(listFile)) {
    const list = JSON.parse(fs.readFileSync(listFile, 'utf8'));
    const keys = Object.keys(list);
    assert.deepEqual(keys, keys.slice().sort(), 'new-keys.json is sorted');
    assert.ok(keys.every(key => typeof list[key] === 'string' && list[key].length > 0), 'every new key has an English text');
}
console.log('test-pixel-i18n: en.json covers every Pixel key');
