'use strict';

self.onmessage = function (event) {
    const payload = event && event.data ? event.data : {};
    try {
        const regex = new RegExp(String(payload.pattern || ''), 'g');
        const replacement = String(payload.replacement || '');
        const bases = Array.isArray(payload.bases) ? payload.bases : [];
        self.postMessage({ ok: true, bases: bases.map(base => String(base).replace(regex, replacement)) });
    } catch (error) {
        self.postMessage({ ok: false, error: error && error.message ? error.message : 'invalid_regular_expression' });
    }
};
