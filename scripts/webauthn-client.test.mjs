import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('../internal/server/auth_webauthn_client.js', import.meta.url), 'utf8');
function client(overrides = {}) {
    const context = vm.createContext({ atob, btoa, Uint8Array, DOMException, isSecureContext: true, PublicKeyCredential: {}, navigator: { credentials: {} }, ...overrides });
    vm.runInContext(source, context);
    return context.AuraWebAuthn;
}
for (const registration of [false, true]) {
    for (const native of [false, true]) {
        const calls = [];
        let requested;
        const json = { challenge: 'AQI', user: { id: 'AwQ', name: 'owner', displayName: 'Owner' }, allowCredentials: [{ type: 'public-key', id: 'BQY' }], excludeCredentials: [{ type: 'public-key', id: 'Bwg' }] };
        const credential = { id: 'BQY', rawId: Uint8Array.of(5, 6).buffer, type: 'public-key', response: { clientDataJSON: Uint8Array.of(1, 2).buffer, attestationObject: Uint8Array.of(3, 4).buffer, getTransports: () => ['usb'] }, getClientExtensionResults: () => ({}) };
        if (native) credential.toJSON = () => ({ id: 'native-json' });
        const api = client({
            PublicKeyCredential: native ? { [registration ? 'parseCreationOptionsFromJSON' : 'parseRequestOptionsFromJSON']: value => { assert.equal(value.challenge, 'AQI'); return { native: true }; } } : {},
            navigator: { credentials: { [registration ? 'create' : 'get']: async request => { requested = request; return credential; } } },
            fetch: async (url, request) => {
                calls.push({ url, request });
                assert.equal(request.credentials, 'same-origin'); assert.equal(request.cache, 'no-store');
                return { ok: true, json: async () => url.endsWith('begin') ? { transaction: 'tx', options: { publicKey: json } } : { ok: true } };
            }
        });
        const signal = new AbortController().signal;
        const result = registration ? await api.register('key', { current_password: 'fixture' }, signal) : await api.login('fixture', '/', signal);
        assert.equal(result.ok, true); assert.equal(calls.length, 2); assert.equal(requested.signal, signal);
        if (native) assert.equal(requested.publicKey.native, true);
        else {
            assert.deepEqual([...requested.publicKey.challenge], [1, 2]);
            assert.deepEqual([...requested.publicKey.allowCredentials[0].id], [5, 6]);
            if (registration) assert.deepEqual([...requested.publicKey.user.id], [3, 4]);
        }
        const finish = JSON.parse(calls[1].request.body);
        assert.equal(finish.transaction, 'tx');
        assert.equal(finish.credential.id, native ? 'native-json' : 'BQY');
        if (!native) { assert.equal(finish.credential.response.clientDataJSON, 'AQI'); assert.deepEqual(finish.credential.response.transports, ['usb']); }
    }
}
for (const overrides of [{ isSecureContext: false }, { PublicKeyCredential: undefined }, { navigator: {} }]) {
    const api = client(overrides);
    assert.equal(api.available(), false);
    await assert.rejects(api.login(), error => api.errorKey(error) === 'webauthn.unsupported');
}
let attempts = 0;
const failing = client({ fetch: async () => { attempts++; throw new TypeError('Lost connection'); } });
await assert.rejects(failing.send('/api/auth/webauthn/settings', {}, 'PUT'), error => failing.errorKey(error) === 'webauthn.uncertain');
assert.equal(attempts, 1);
const abortedWrite = client({ fetch: async () => { throw new DOMException('Cancelled', 'AbortError'); } });
await assert.rejects(abortedWrite.send('/api/auth/webauthn/settings', {}, 'PUT'), error => abortedWrite.errorKey(error) === 'webauthn.uncertain');
const lostBody = client({ fetch: async () => ({ ok: true, json: async () => { throw new TypeError('Lost response'); } }) });
await assert.rejects(lostBody.send('/auth/webauthn/login/finish', {}), error => lostBody.errorKey(error) === 'webauthn.uncertain');
const cancelled = client({ fetch: async () => ({ ok: true, json: async () => ({ transaction: 'tx', options: { publicKey: { challenge: 'AQI' } } }) }), navigator: { credentials: { get: async () => { throw new DOMException('Cancelled', 'AbortError'); } } } });
await assert.rejects(cancelled.login(), error => cancelled.errorKey(error) === 'webauthn.cancelled');
console.log('WebAuthn client: native/legacy encoding, capability checks, cancellation and no-retry failure handling passed.');
