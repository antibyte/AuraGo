(function () {
    'use strict';

    const effectPromises = new Map();
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
            scripts: ['/js/chat/dark-sun-shader.js', '/js/chat/dark-sun-embers.js']
        },
        'ocean': {
            scripts: ['/js/chat/ocean-shader.js']
        },
        'sandstorm': {
            scripts: ['/js/chat/sandstorm-particles.js']
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

    function currentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'dark';
    }

    window.AuraChatThemeEffects = { ensure };

    window.addEventListener('aurago:themechange', event => {
        const theme = event && event.detail && event.detail.theme ? event.detail.theme : currentTheme();
        ensure(theme).catch(() => { });
    });

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => ensure(currentTheme()).catch(() => { }), { once: true });
    } else {
        ensure(currentTheme()).catch(() => { });
    }
})();
