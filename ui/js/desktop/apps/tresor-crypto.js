import setupWasm from '../../vendor/tresor-argon2/setup.js';

const enc = new TextEncoder();
const KDF = Object.freeze({ parallelism: 4, passes: 3, memorySize: 65536, tagLength: 32 });
let argon2Promise;

const random = n => crypto.getRandomValues(new Uint8Array(n));
const text = value => enc.encode(value);

export function toBase64(bytes) {
    let value = '';
    for (let i = 0; i < bytes.length; i += 32768) {
        value += String.fromCharCode(...bytes.subarray(i, i + 32768));
    }
    return btoa(value);
}

export function fromBase64(value) {
    const raw = atob(value);
    return Uint8Array.from(raw, c => c.charCodeAt(0));
}

export function recoveryFromHex(value) {
    const hex = String(value || '').replace(/[\s-]/g, '').toLowerCase();
    if (!/^[0-9a-f]{64}$/.test(hex)) throw new Error('invalid_recovery_key');
    return Uint8Array.from(hex.match(/../g), pair => parseInt(pair, 16));
}

async function loadArgon2() {
    if (!argon2Promise) {
        let memory;
        const instantiate = name => async imports => {
            memory = imports.env.memory;
            const response = await fetch(`/js/vendor/tresor-argon2/${name}.wasm`, { cache: 'no-store' });
            if (!response.ok) throw new Error('argon2_unavailable');
            return WebAssembly.instantiate(await response.arrayBuffer(), imports);
        };
        argon2Promise = setupWasm(instantiate('simd'), instantiate('no-simd')).then(hash => params => {
            try { return hash(params); }
            finally { new Uint8Array(memory.buffer).fill(0); }
        });
    }
    return argon2Promise;
}

export async function derivePasswordKey(password, salt, loader = loadArgon2) {
    if (typeof password !== 'string' || password.length < 16 || salt.length !== 32) throw new Error('invalid_password');
    const bytes = text(password);
    try {
        return (await loader())({ password: bytes, salt, ...KDF });
    } finally {
        bytes.fill(0);
    }
}

async function aesKey(raw) {
    return crypto.subtle.importKey('raw', raw, 'AES-GCM', false, ['encrypt', 'decrypt']);
}

export async function seal(rawKey, plaintext, context) {
    const nonce = random(12);
    const ciphertext = new Uint8Array(await crypto.subtle.encrypt(
        { name: 'AES-GCM', iv: nonce, additionalData: text(context) }, await aesKey(rawKey), plaintext
    ));
    const result = new Uint8Array(nonce.length + ciphertext.length);
    result.set(nonce);
    result.set(ciphertext, nonce.length);
    return result;
}

export async function open(rawKey, sealed, context) {
    if (sealed.length < 28) throw new Error('invalid_ciphertext');
    return new Uint8Array(await crypto.subtle.decrypt(
        { name: 'AES-GCM', iv: sealed.subarray(0, 12), additionalData: text(context) },
        await aesKey(rawKey), sealed.subarray(12)
    ));
}

export async function createHeader(password) {
    const salt = random(32);
    const master = random(32);
    const recovery = random(32);
    let passwordKey;
    try {
        passwordKey = await derivePasswordKey(password, salt);
        const header = {
            salt: toBase64(salt),
            password_envelope: toBase64(await seal(passwordKey, master, 'tresor:v1:password')),
            recovery_envelope: toBase64(await seal(recovery, master, 'tresor:v1:recovery'))
        };
        return { header, recoveryHex: Array.from(recovery, b => b.toString(16).padStart(2, '0')).join('').toUpperCase() };
    } finally {
        master.fill(0);
        passwordKey?.fill(0);
        recovery.fill(0);
    }
}

export async function unlockWithPassword(header, password) {
    const key = await derivePasswordKey(password, fromBase64(header.salt));
    try {
        return await open(key, fromBase64(header.password_envelope), 'tresor:v1:password');
    } finally { key.fill(0); }
}

export async function unlockWithRecovery(header, recoveryText) {
    const key = recoveryFromHex(recoveryText);
    try {
        return await open(key, fromBase64(header.recovery_envelope), 'tresor:v1:recovery');
    } finally { key.fill(0); }
}

export async function changePassword(header, master, password) {
    const salt = random(32);
    const key = await derivePasswordKey(password, salt);
    try {
        return {
            salt: toBase64(salt),
            password_envelope: toBase64(await seal(key, master, 'tresor:v1:password')),
            recovery_envelope: header.recovery_envelope
        };
    } finally { key.fill(0); }
}

export const recordContext = (id, field) => `tresor:v1:${id}:${field}`;
