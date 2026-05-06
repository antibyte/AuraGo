(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for main');
    }
    window.AuraDesktopModules.loadScriptParts('main', [
        '/js/desktop/core/main-part-01.js?v=1',
        '/js/desktop/core/main-part-02.js?v=1',
        '/js/desktop/core/main-part-03.js?v=1',
        '/js/desktop/core/main-part-04.js?v=1',
        '/js/desktop/core/main-part-05.js?v=1',
        '/js/desktop/core/main-part-06.js?v=1',
        '/js/desktop/core/main-part-07.js?v=1'
    ]);
})();
