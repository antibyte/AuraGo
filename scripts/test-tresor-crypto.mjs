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

const { header, recoveryHex } = await c.createHeader('correct horse battery staple');
const master = await c.unlockWithPassword(header, 'correct horse battery staple');
assert.deepEqual(await c.unlockWithRecovery(header, recoveryHex), master);
await assert.rejects(c.unlockWithPassword(header, 'wrong password that is long enough'));
await assert.rejects(c.unlockWithRecovery(header, '0'.repeat(64)));

const sealed = await c.seal(master, new TextEncoder().encode('confidential note'), 'tresor:v1:id:body');
assert.equal(new TextDecoder().decode(await c.open(master, sealed, 'tresor:v1:id:body')), 'confidential note');
const tampered = sealed.slice();
tampered[15] ^= 1;
await assert.rejects(c.open(master, tampered, 'tresor:v1:id:body'));
await assert.rejects(c.open(master, sealed, 'tresor:v1:other:body'));

const changed = await c.changePassword(header, master, 'another sufficiently strong password');
assert.deepEqual(await c.unlockWithPassword(changed, 'another sufficiently strong password'), master);
assert.deepEqual(await c.unlockWithRecovery(changed, recoveryHex), master);
console.log('Tresor crypto vectors, recovery, rewrap, and integrity checks passed.');
