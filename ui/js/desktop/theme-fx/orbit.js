(function () {
    'use strict';

    // Orbit effect (registered as "orbit" with AuraDesktopThemeFx). The orchestrator of the theme: it owns the lifecycle of
    // the live scene, the particle layer and the window bridge, renders one still frame instead of a loop when motion is off
    // (the theme declares staticFx), pauses while the tab is hidden, and answers inspect() and command(). This file is the
    // scaffold: it registers an effect that does nothing yet.
    const fx = window.AuraDesktopThemeFx;
    if (!fx) return;
    fx.register('orbit', { start() { return { pause() {}, resume() {}, stop() {} }; } });
}());
