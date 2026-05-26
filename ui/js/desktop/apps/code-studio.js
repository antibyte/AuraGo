(function () {
    'use strict';

    if (!window.AuraDesktopModules || typeof window.AuraDesktopModules.loadScriptParts !== 'function') {
        throw new Error('Aura desktop module loader is not available for code-studio');
    }
    var v = window.BUILD_VERSION || 'dev';
    window.AuraDesktopModules.loadScriptParts('code-studio', [
        '/js/desktop/apps/code-studio/core-shell-files.js?v=' + v,
        '/js/desktop/apps/code-studio/editor-terminal-files.js?v=' + v,
        '/js/desktop/apps/code-studio/actions-agent-editor.js?v=' + v,
        '/js/desktop/apps/code-studio/command-palette.js?v=' + v
    ]).catch(err => console.error('Failed to load Aura desktop code-studio bundle', err));
})();
