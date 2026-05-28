(function () {
    'use strict';

    window.AuraDesktopModules = window.AuraDesktopModules || {};
    const modulePromises = new Map();

    function fetchScriptPart(part) {
        return fetch(part, { credentials: 'same-origin', cache: 'no-store' }).then(response => {
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
                window.dispatchEvent(new CustomEvent('aurago:module-loaded', { detail: { label } }));
            })
            .catch(err => {
                modulePromises.delete(cacheKey);
                console.error('Failed to load desktop module', label, err);
                window.dispatchEvent(new CustomEvent('aurago:module-load-error', { detail: { label, error: err } }));
                throw err;
            });
        modulePromises.set(cacheKey, promise);
        return promise;
    };

    const LAZY_APP_SCRIPTS = {
        'system-info': '/js/desktop/apps/system-info.js',
        'camera': '/js/desktop/apps/camera.js',
        'zipper': '/js/desktop/apps/zipper.js',
        'pixel': '/js/desktop/apps/pixel.js',
        'galaxa-deluxe': '/js/desktop/apps/galaxa-deluxe.js',
        'viewer-3d': '/js/desktop/apps/viewer-3d.js'
    };

    window.AuraDesktopModules.loadAppScript = window.AuraDesktopModules.loadAppScript || function loadAppScript(appId) {
        const src = LAZY_APP_SCRIPTS[appId];
        if (!src) return Promise.resolve();
        const versioned = src + (src.includes('?') ? '&' : '?') + 'v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
        if (modulePromises.has(appId)) return modulePromises.get(appId);
        const promise = new Promise((resolve, reject) => {
            const script = document.createElement('script');
            script.src = versioned;
            script.onload = () => {
                modulePromises.set(appId, Promise.resolve());
                resolve();
            };
            script.onerror = () => {
                modulePromises.delete(appId);
                reject(new Error('Failed to load app script: ' + appId));
            };
            document.head.appendChild(script);
        });
        modulePromises.set(appId, promise);
        return promise;
    };
})();
