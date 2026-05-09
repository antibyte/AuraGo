(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for main');
    }
    var v = window.BUILD_VERSION || 'dev';
    window.AuraDesktopModules.loadScriptParts('main', [
        '/js/desktop/core/desktop-foundation.js?v=' + v,
        '/js/desktop/core/window-shell-runtime.js?v=' + v,
        '/js/desktop/core/lifecycle-cleanup.js?v=' + v,
        '/js/desktop/core/widget-autosize-runtime.js?v=' + v,
        '/js/desktop/core/shortcut-runtime.js?v=' + v,
        '/js/desktop/core/menus-and-routing.js?v=' + v,
        '/js/desktop/apps/settings-calculator.js?v=' + v,
        '/js/desktop/apps/planning-gallery-music.js?v=' + v,
        '/js/desktop/apps/calendar.js?v=' + v,
        '/js/desktop/apps/quickconnect-launchpad-chat.js?v=' + v,
        '/js/desktop/core/sdk-events-bootstrap.js?v=' + v
    ]).catch(err => console.error('Failed to load Aura desktop main bundle', err));
})();
