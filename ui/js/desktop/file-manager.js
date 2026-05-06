(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for file-manager');
    }
    window.AuraDesktopModules.loadScriptParts('file-manager', [
        '/js/desktop/file-manager-part-01.js?v=1',
        '/js/desktop/file-manager-part-02.js?v=1',
        '/js/desktop/file-manager-part-03.js?v=1'
    ]);
})();
