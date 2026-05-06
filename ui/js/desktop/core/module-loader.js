(function () {
    'use strict';

    window.AuraDesktopModules = window.AuraDesktopModules || {};

    window.AuraDesktopModules.loadScriptParts = window.AuraDesktopModules.loadScriptParts || function loadScriptParts(label, parts) {
        if (!Array.isArray(parts) || parts.length === 0) {
            throw new Error('Desktop module has no script parts: ' + label);
        }
        let source = '';
        for (const part of parts) {
            const xhr = new XMLHttpRequest();
            xhr.open('GET', part, false);
            xhr.send(null);
            if ((xhr.status < 200 || xhr.status >= 300) && xhr.status !== 0) {
                throw new Error('Failed to load desktop module part ' + part + ': HTTP ' + xhr.status);
            }
            source += '\n;' + xhr.responseText;
        }
        source += '\n//# sourceURL=/js/desktop/' + String(label || 'module') + '.bundle.js';
        (0, eval)(source);
    };
})();
