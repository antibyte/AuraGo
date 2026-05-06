(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for code-studio');
    }
    window.AuraDesktopModules.loadScriptParts('code-studio', [
        '/js/desktop/apps/code-studio/core-shell-files.js?v=1',
        '/js/desktop/apps/code-studio/actions-agent-editor.js?v=1'
    ]);
})();
