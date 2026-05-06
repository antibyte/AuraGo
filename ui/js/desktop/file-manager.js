(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for file-manager');
    }
    window.AuraDesktopModules.loadScriptParts('file-manager', [
        '/js/desktop/file-manager/core-render.js?v=2',
        '/js/desktop/file-manager/actions-input.js?v=1',
        '/js/desktop/file-manager/lifecycle-export.js?v=1'
    ]);
})();
