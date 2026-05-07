(function () {
    'use strict';

    window.AuraDesktopModules = window.AuraDesktopModules || {};
    const modulePromises = new Map();

    function fetchScriptPart(part) {
        return fetch(part, { credentials: 'same-origin' }).then(response => {
            if (!response.ok) {
                throw new Error('Failed to load desktop module part ' + part + ': HTTP ' + response.status);
            }
            return response.text();
        });
    }

    window.AuraDesktopModules.loadScriptParts = window.AuraDesktopModules.loadScriptParts || function loadScriptParts(label, parts) {
        if (!Array.isArray(parts) || parts.length === 0) {
            return Promise.reject(new Error('Desktop module has no script parts: ' + label));
        }
        const cacheKey = String(label || 'module') + ':' + parts.join('|');
        if (modulePromises.has(cacheKey)) return modulePromises.get(cacheKey);
        const promise = Promise.all(parts.map(fetchScriptPart))
            .then(sources => {
                const source = sources.map(source => '\n;' + source).join('')
                    + '\n//# sourceURL=/js/desktop/' + String(label || 'module') + '.bundle.js';
                (0, eval)(source);
                window.dispatchEvent(new CustomEvent('auradesktop:module-loaded', { detail: { label } }));
            })
            .catch(err => {
                console.error('Failed to load desktop module', label, err);
                window.dispatchEvent(new CustomEvent('auradesktop:module-load-error', { detail: { label, error: err } }));
                throw err;
            });
        modulePromises.set(cacheKey, promise);
        return promise;
    };
})();
