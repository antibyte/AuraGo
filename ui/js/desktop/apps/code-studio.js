(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for code-studio');
    }
    window.AuraDesktopModules.loadScriptParts('code-studio', [
        '/js/desktop/apps/code-studio-part-01.js?v=1',
        '/js/desktop/apps/code-studio-part-02.js?v=1'
    ]);
})();
