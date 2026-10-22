/* Shared by the full UI and the self-contained recovery login. */
(function () {
    'use strict';
    const decode = value => Uint8Array.from(atob(value.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0));
    const encode = value => btoa(Array.from(new Uint8Array(value), c => String.fromCharCode(c)).join('')).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    function options(value, registration) {
        const json = value.publicKey;
        const parse = registration ? PublicKeyCredential.parseCreationOptionsFromJSON : PublicKeyCredential.parseRequestOptionsFromJSON;
        if (typeof parse === 'function') return parse.call(PublicKeyCredential, json);
        const key = { ...json, challenge: decode(json.challenge) };
        if (registration) key.user = { ...json.user, id: decode(json.user.id) };
        for (const field of ['allowCredentials', 'excludeCredentials']) {
            if (json[field]) key[field] = json[field].map(c => ({ ...c, id: decode(c.id) }));
        }
        return key;
    }
    function serialize(credential) {
        if (typeof credential.toJSON === 'function') return credential.toJSON();
        const response = {};
        for (const key of ['clientDataJSON', 'attestationObject', 'authenticatorData', 'signature', 'userHandle']) {
            if (credential.response[key] != null) response[key] = encode(credential.response[key]);
        }
        if (typeof credential.response.getTransports === 'function') response.transports = credential.response.getTransports();
        return { id: credential.id, rawId: encode(credential.rawId), type: credential.type, response,
            authenticatorAttachment: credential.authenticatorAttachment || undefined,
            clientExtensionResults: credential.getClientExtensionResults() };
    }
    async function send(url, body, method = 'POST', signal) {
        let response, data;
        try {
            response = await fetch(url, { method, credentials: 'same-origin', cache: 'no-store', signal,
                headers: { 'Content-Type': 'application/json' }, body: method === 'GET' ? undefined : JSON.stringify(body || {}) });
            data = await response.json();
        } catch (error) {
            if (url.endsWith('/finish') || ['PUT', 'PATCH', 'DELETE'].includes(method)) throw Object.assign(new Error('webauthn_uncertain', { cause: error }), { code: 'webauthn_uncertain' });
            throw error;
        }
        if (!response.ok) { const error = new Error(data.code || data.error || 'webauthn_failed'); error.code = data.code || data.error || 'webauthn_failed'; throw error; }
        return data;
    }
    function available() {
        return globalThis.isSecureContext === true && typeof globalThis.PublicKeyCredential !== 'undefined' && !!globalThis.navigator?.credentials;
    }
    async function ceremony(registration, body, signal) {
        if (!available()) { const error = new Error('webauthn_browser_unavailable'); error.code = error.message; throw error; }
        const base = registration ? '/api/auth/webauthn/register/' : '/auth/webauthn/login/';
        const begin = await send(base + 'begin', body, 'POST', signal);
        const request = { publicKey: options(begin.options, registration), signal };
        const credential = registration ? await navigator.credentials.create(request) : await navigator.credentials.get(request);
        if (!credential) throw new DOMException('No credential', 'NotAllowedError');
        return send(base + 'finish', { transaction: begin.transaction, credential: serialize(credential) }, 'POST', signal);
    }
    globalThis.AuraWebAuthn = {
        available, send,
        login: (password, redirect, signal) => ceremony(false, { password: password || '', redirect: redirect || '/' }, signal),
        register: (label, stepUp, signal) => ceremony(true, { label, ...stepUp }, signal),
        errorKey(error) {
            if (error.code === 'webauthn_uncertain') return 'webauthn.uncertain';
            if (error.name === 'AbortError') return 'webauthn.cancelled';
            if (error.name === 'NotAllowedError') return 'webauthn.cancelled_or_timeout';
            if (error.name === 'InvalidStateError') return 'webauthn.duplicate';
            if (error.name === 'SecurityError') return 'webauthn.origin_error';
            const keys = { webauthn_browser_unavailable: 'unsupported', webauthn_invalid_credentials: 'invalid_credentials',
                webauthn_origin_rejected: 'origin_error', webauthn_invalid_ceremony: 'expired', webauthn_save_failed: 'save_failed',
                webauthn_registry_unavailable: 'storage_error', webauthn_invalid_settings: 'settings_error', webauthn_credentials_required: 'enroll_first',
                webauthn_last_factor: 'last_factor', webauthn_reset_required: 'reset_required', webauthn_rate_limited: 'rate_limited',
                webauthn_credential_limit: 'limit', current_password_required: 'step_up', current_totp_required: 'step_up',
                invalid_credentials: 'invalid_credentials', too_many_attempts: 'rate_limited', webauthn_required: 'second_factor_hint' };
            return 'webauthn.' + (keys[error.code] || 'failed');
        }
    };
})();
