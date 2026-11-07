(function () {
    'use strict';

    const effectPromises = new Map();
    let opticsPromise = null;
    const EFFECTS = {
        'retro-crt': {
            scripts: [
                '/js/crt-persistence-shader.js',
                '/js/crt-shader.js'
            ]
        },
        '8bit': {
            scripts: ['/js/chat/8bit-pixelate.js']
        },
        'cyberwar': {
            scripts: [
                '/js/chat/cyberwar-shader.js',
                '/js/chat/cyberwar-hud.js'
            ]
        },
        'lollipop': {
            scripts: ['/js/chat/lollipop-petals.js']
        },
        'dark-sun': {
            scripts: ['/js/chat/dark-sun-shader.js']
        },
        'ocean': {
            scripts: ['/js/chat/ocean-shader.js']
        },
        'sandstorm': {
            scripts: ['/js/chat/sandstorm-particles.js']
        },
        'galaxy': {
            scripts: ['/js/chat/galaxy-interface.js', '/js/vendor/three.min.js', '/js/chat/galaxy-scene.js']
        },
        'threedee': {
            scripts: [
                '/js/vendor/three.min.js',
                '/js/vendor/GLTFLoader.min.js',
                '/js/vendor/DRACOLoader.min.js',
                '/js/chat/threedee-shader.js',
                '/js/chat/threedee-fold.js'
            ]
        },
        'black-matrix': {
            scripts: ['/js/chat/black-matrix-shader.js']
        }
    };

    // Theme optics are a shared WebGL backdrop that every chat theme gets. The
    // scripts load once, independent of the theme: the controller swaps the
    // recipe when the theme changes, so switching themes never reloads a shader.
    const OPTICS_SCRIPTS = [
        '/js/shared/theme-optics-core.js',
        '/js/chat/theme-optics-recipes.js',
        '/js/chat/theme-optics.js'
    ];

    function assets() {
        if (window.AuraLazyAssets) return window.AuraLazyAssets;
        return {
            loadAll(spec) {
                const scripts = spec && Array.isArray(spec.scripts) ? spec.scripts : [];
                let chain = Promise.resolve();
                scripts.forEach(src => {
                    chain = chain.then(() => new Promise((resolve, reject) => {
                        const script = document.createElement('script');
                        script.src = src;
                        script.onload = resolve;
                        script.onerror = () => reject(new Error('Failed to load script: ' + src));
                        document.head.appendChild(script);
                    }));
                });
                return chain;
            }
        };
    }

    function ensure(theme) {
        const key = String(theme || '').trim();
        const spec = EFFECTS[key];
        if (!spec) return Promise.resolve(false);
        if (effectPromises.has(key)) return effectPromises.get(key);
        const promise = assets().loadAll(spec).then(() => {
            window.dispatchEvent(new CustomEvent('aurago:theme-effects-loaded', { detail: { theme: key } }));
            return true;
        }).catch(err => {
            effectPromises.delete(key);
            console.error('[AuraGo] Failed to load chat theme effect', key, err);
            throw err;
        });
        effectPromises.set(key, promise);
        return promise;
    }

    // The optics backdrop is theme independent, so it loads once for the page.
    function ensureOptics() {
        if (opticsPromise) return opticsPromise;
        opticsPromise = assets().loadAll({ scripts: OPTICS_SCRIPTS }).then(() => {
            window.dispatchEvent(new CustomEvent('aurago:theme-optics-loaded', { detail: { theme: currentTheme() } }));
            return true;
        }).catch(err => {
            opticsPromise = null;
            console.warn('[AuraGo] Failed to load chat theme optics', err);
            return false;
        });
        return opticsPromise;
    }

    function currentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'dark';
    }

    window.AuraChatThemeEffects = { ensure, ensureOptics };

    window.addEventListener('aurago:themechange', event => {
        const theme = event && event.detail && event.detail.theme ? event.detail.theme : currentTheme();
        ensure(theme).catch(() => { });
        ensureOptics();
    });

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            ensure(currentTheme()).catch(() => { });
            ensureOptics();
        }, { once: true });
    } else {
        ensure(currentTheme()).catch(() => { });
        ensureOptics();
    }
})();
