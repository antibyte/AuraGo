(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for file-manager');
    }
    var v = window.BUILD_VERSION || 'dev';
    window.AuraDesktopModules.loadScriptParts('file-manager', [
        '/js/desktop/file-manager/core-render.js?v=' + v,
        '/js/desktop/file-manager/core-render-components.js?v=' + v,
        '/js/desktop/file-manager/actions-input.js?v=' + v,
        '/js/desktop/file-manager/actions-operations.js?v=' + v,
        '/js/desktop/file-manager/tabs.js?v=' + v,
        '/js/desktop/file-manager/preview-panel.js?v=' + v,
        '/js/desktop/file-manager/advanced-actions.js?v=' + v,
        '/js/desktop/file-manager/lifecycle-export.js?v=' + v
    ]).catch(err => console.error('Failed to load Aura desktop file-manager bundle', err));
})();
