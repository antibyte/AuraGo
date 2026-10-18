(function () {
    'use strict';

    // Placeholder for the Crazy theme effects: the real implementation arrives in a later package.
    // The host (fx-core.js) already provides the layer, the AbortSignal and the hidden-tab pause.
    window.AuraDesktopThemeFx.register('crazy', { start() { return { pause() {}, resume() {}, stop() {} }; } });
})();
