(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createAudio = function (ctx) {
        GC.createAudioCore(ctx);
        GC.createAudioSfx(ctx);
        GC.createAudioMusic(ctx);
    };
})();
