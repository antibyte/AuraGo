(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for main');
    }
    window.AuraDesktopModules.loadScriptParts('main', [
        '/js/desktop/core/desktop-foundation.js?v=9',
        '/js/desktop/core/window-shell-runtime.js?v=5',
        '/js/desktop/core/lifecycle-cleanup.js?v=1',
        '/js/desktop/core/widget-autosize-runtime.js?v=2',
        '/js/desktop/core/shortcut-runtime.js?v=1',
        '/js/desktop/core/menus-and-routing.js?v=3',
        '/js/desktop/apps/settings-calculator.js?v=5',
        '/js/desktop/apps/planning-gallery-music.js?v=3',
        '/js/desktop/apps/quickconnect-launchpad-chat.js?v=1',
        '/js/desktop/core/sdk-events-bootstrap.js?v=3'
    ]).catch(err => console.error('Failed to load Aura desktop main bundle', err));
})();
