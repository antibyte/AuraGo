(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadBundle !== 'function') {
        throw new Error('Aura desktop module loader is not available for code-studio');
    }
    window.AuraDesktopModules.loadBundle('code-studio', '/js/desktop/bundles/code-studio.bundle.js')
        .catch(err => console.error('Failed to load Aura desktop code-studio bundle', err));
})();
