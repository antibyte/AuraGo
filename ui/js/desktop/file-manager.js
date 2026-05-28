(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadBundle !== 'function') {
        throw new Error('Aura desktop module loader is not available for file-manager');
    }
    window.AuraDesktopModules.loadBundle('file-manager', '/js/desktop/bundles/file-manager.bundle.js')
        .catch(err => console.error('Failed to load Aura desktop file-manager bundle', err));
})();
