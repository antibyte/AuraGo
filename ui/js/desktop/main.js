(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for main');
    }
    var v = window.BUILD_VERSION || 'dev';
    var assetV = v + '-desktop-20260525-window-ai-context';
    window.AuraDesktopModules.loadScriptParts('main', [
        '/js/desktop/core/desktop-foundation.js?v=' + assetV,
        '/js/desktop/core/icon-selection-runtime.js?v=' + assetV,
        '/js/desktop/core/window-shell-runtime.js?v=' + assetV,
        '/js/desktop/core/window-ai-context.js?v=' + assetV,
        '/js/desktop/core/lifecycle-cleanup.js?v=' + assetV,
        '/js/desktop/core/widget-autosize-runtime.js?v=' + assetV,
        '/js/desktop/core/shortcut-runtime.js?v=' + assetV,
        '/js/desktop/core/desktop-file-drops.js?v=' + assetV,
        '/js/desktop/apps/agent-chat.js?v=' + assetV,
        '/js/desktop/apps/software-store.js?v=' + assetV,
        '/js/desktop/core/menus-and-routing.js?v=' + assetV,
        '/js/desktop/apps/settings-calculator.js?v=' + assetV,
        '/js/desktop/apps/planning-gallery-music.js?v=' + assetV,
        '/js/desktop/apps/quickconnect-launchpad-chat.js?v=' + assetV,
        '/js/desktop/core/sdk-events-bootstrap.js?v=' + assetV
    ]).catch(err => console.error('Failed to load Aura desktop main bundle', err));
})();
