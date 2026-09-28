(function () {
    'use strict';

    // Loads the isolated Three.js 0.185.1 renderer only when the Tiefsee scene starts.
    const BUNDLE = '/js/vendor/screensaver-abyss/abyss.esm.js';

    async function createAbyssTheme(env) {
        const url = env.versionedURL ? env.versionedURL(BUNDLE) : BUNDLE;
        const module = await import(url);
        return module.createAbyss(env);
    }

    window.AuraScreensavers.register('abyss', createAbyssTheme);
})();
