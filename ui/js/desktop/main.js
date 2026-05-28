(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadBundle !== 'function') {
        throw new Error('Aura desktop module loader is not available for main');
    }
    window.AuraDesktopModules.loadBundle('main', '/js/desktop/bundles/main.bundle.js')
        .catch(err => console.error('Failed to load Aura desktop main bundle', err));
})();
