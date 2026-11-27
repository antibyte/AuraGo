import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
globalThis.fetch = async url => new Response(await fs.readFile(path.join(root, 'ui', url)));
const instantiate = WebAssembly.instantiate;
let wasmMemory;
WebAssembly.instantiate = (bytes, imports) => {
    wasmMemory = imports.env.memory;
    return instantiate(bytes, imports);
};
const c = await import('../ui/js/desktop/apps/tresor-crypto.js');

const salt = Uint8Array.from({ length: 32 }, (_, i) => i);
const key = await c.derivePasswordKey('correct horse battery staple', salt);
assert.equal(Buffer.from(key).toString('hex'), '0b220637b2f1ea0425e3c3e4ea5c5c14fd91b0f26b5726839a04f2e387f75f73');
assert.ok(new Uint8Array(wasmMemory.buffer).every(byte => byte === 0), 'Argon2 workspace must be wiped');

// The derivation label is the only thing that keeps an older vault openable: an
// absent label must fall back to the frozen v1 parameters, a hostile one is refused.
const V1 = { parallelism: 4, passes: 3, memorySize: 65536, tagLength: 32 };
assert.deepEqual(c.parseKdf(''), V1);
assert.deepEqual(c.parseKdf(undefined), V1);
assert.deepEqual(c.parseKdf('argon2id$v=19$m=65536,t=3,p=4'), V1);
assert.deepEqual(await c.derivePasswordKey('correct horse battery staple', salt, V1), key);
for (const bad of ['bcrypt$v=19$m=65536,t=3,p=4', 'argon2id$v=19$m=65536,t=3', 'argon2id$v=19$m=1048577,t=3,p=4', 'argon2id$v=19$m=65536,t=0,p=4', 'argon2id$v=19$m=65536,t=3,p=65', 'argon2id$v=19$m=65536,t=3,p=4 ']) {
    assert.throws(() => c.parseKdf(bad), /invalid_kdf/, bad);
}

const { header, recoveryHex } = await c.createHeader('correct horse battery staple');
assert.equal(header.kdf, 'argon2id$v=19$m=65536,t=3,p=4');
const master = await c.unlockWithPassword(header, 'correct horse battery staple');
assert.deepEqual(await c.unlockWithRecovery(header, recoveryHex), master);
const legacy = { salt: header.salt, password_envelope: header.password_envelope, recovery_envelope: header.recovery_envelope };
assert.deepEqual(await c.unlockWithPassword(legacy, 'correct horse battery staple'), master);
await assert.rejects(c.unlockWithPassword(header, 'wrong password that is long enough'));
await assert.rejects(c.unlockWithRecovery(header, '0'.repeat(64)));

const sealed = await c.seal(master, new TextEncoder().encode('confidential note'), 'tresor:v1:id:body');
assert.equal(new TextDecoder().decode(await c.open(master, sealed, 'tresor:v1:id:body')), 'confidential note');
const tampered = sealed.slice();
tampered[15] ^= 1;
await assert.rejects(c.open(master, tampered, 'tresor:v1:id:body'));
await assert.rejects(c.open(master, sealed, 'tresor:v1:other:body'));

const changed = await c.changePassword(header, master, 'another sufficiently strong password');
assert.equal(changed.kdf, 'argon2id$v=19$m=65536,t=3,p=4');
assert.deepEqual(await c.unlockWithPassword(changed, 'another sufficiently strong password'), master);
assert.deepEqual(await c.unlockWithRecovery(changed, recoveryHex), master);
console.log('Tresor crypto vectors, recovery, rewrap, and integrity checks passed.');
