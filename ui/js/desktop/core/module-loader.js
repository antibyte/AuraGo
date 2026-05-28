(function () {
    'use strict';

    window.AuraDesktopModules = window.AuraDesktopModules || {};

    const modulePromises = new Map();
    const appPromises = new Map();
    const readyApps = new Set();
    const bundlePaths = {
        main: '/js/desktop/bundles/main.bundle.js',
        'file-manager': '/js/desktop/bundles/file-manager.bundle.js',
        'code-studio': '/js/desktop/bundles/code-studio.bundle.js'
    };

    const DESKTOP_APP_ASSETS = {
        'files': {
            scripts: ['/js/desktop/bundles/file-manager.bundle.js']
        },
        'writer': {
            styles: ['/css/quill.snow.css'],
            scripts: ['/js/vendor/quill.js', '/js/desktop/apps/writer.js']
        },
        'sheets': {
            scripts: ['/js/desktop/apps/sheets.js']
        },
        'agent-chat': {
            styles: ['/css/chat-modules.css', '/css/stt-overlay.css', '/css/hljs-github-dark.min.css'],
            scripts: [
                '/js/vendor/markdown-it.min.js',
                '/js/vendor/highlight.min.js',
                '/js/shared/render-markdown.js',
                '/js/chat/ui-icons.js',
                '/js/chat/modules/voice-recorder.js',
                '/js/chat/modules/speech-to-text.js',
                '/js/chat/modules/mermaid-loader.js',
                '/js/desktop/chat-renderer.js',
                '/js/desktop/apps/agent-chat.js'
            ]
        },
        'code-studio': {
            styles: ['/css/code-studio.css', '/css/xterm.css', '/css/hljs-github-dark.min.css'],
            scripts: [
                '/js/vendor/xterm.min.js',
                '/js/vendor/xterm-addon-fit.min.js',
                '/js/vendor/highlight.min.js',
                '/js/desktop/bundles/code-studio.bundle.js'
            ]
        },
        'radio': {
            styles: ['/css/radio.css'],
            scripts: ['/js/desktop/apps/radio.js']
        },
        'looper': {
            scripts: ['/js/desktop/apps/looper.js']
        },
        'viewer': {
            scripts: ['/js/vendor/pdf.min.js', '/js/desktop/apps/viewer.js']
        },
        'camera': {
            styles: ['/css/camera.css'],
            scripts: ['/js/desktop/apps/camera.js']
        },
        'zipper': {
            styles: ['/css/zipper.css'],
            scripts: ['/js/desktop/apps/zipper.js']
        },
        'pixel': {
            styles: ['/css/pixel.css'],
            scripts: ['/js/desktop/apps/pixel.js']
        },
        'galaxa-deluxe': {
            styles: ['/css/galaxa-deluxe.css'],
            scripts: ['/js/desktop/apps/galaxa-deluxe.js']
        },
        'viewer-3d': {
            styles: ['/css/stl-viewer.css'],
            scripts: [
                '/js/vendor/three.min.js',
                '/js/vendor/STLLoader.min.js',
                '/js/vendor/OrbitControls.min.js',
                '/js/desktop/apps/viewer-3d.js'
            ]
        },
        'system-info': {
            scripts: ['/chart.min.js', '/js/desktop/apps/system-info.js']
        },
        'quick-connect': {
            styles: ['/css/xterm.css'],
            scripts: [
                '/js/vendor/xterm.min.js',
                '/js/vendor/xterm-addon-fit.min.js',
                '/js/vendor/novnc.min.js'
            ]
        },
        'software-store': {
            scripts: ['/js/desktop/apps/software-store.js']
        }
    };

    function versionedURL(url) {
        if (window.AuraLazyAssets && typeof window.AuraLazyAssets.versionedURL === 'function') {
            return window.AuraLazyAssets.versionedURL(url);
        }
        if (!url || /^(?:data:|blob:|https?:)?\/\//i.test(url)) return url;
        const separator = url.includes('?') ? '&' : '?';
        return url + separator + 'v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
    }

    function fallbackLoadScript(src) {
        const versioned = versionedURL(src);
        const existing = document.querySelector('script[data-aurago-lazy-src="' + src + '"],script[src="' + versioned + '"]');
        if (existing && existing.dataset.auragoLoaded === '1') return Promise.resolve(existing);
        return new Promise((resolve, reject) => {
            const script = existing || document.createElement('script');
            script.dataset.auragoLazySrc = src;
            script.onload = () => {
                script.dataset.auragoLoaded = '1';
                resolve(script);
            };
            script.onerror = () => reject(new Error('Failed to load script: ' + src));
            if (!existing) {
                script.src = versioned;
                document.head.appendChild(script);
            }
        });
    }

    function fallbackLoadStyle(href) {
        const versioned = versionedURL(href);
        const existing = document.querySelector('link[data-aurago-lazy-href="' + href + '"],link[href="' + versioned + '"]');
        if (existing) return Promise.resolve(existing);
        return new Promise((resolve, reject) => {
            const link = document.createElement('link');
            link.rel = 'stylesheet';
            link.dataset.auragoLazyHref = href;
            link.href = versioned;
            link.onload = () => resolve(link);
            link.onerror = () => reject(new Error('Failed to load stylesheet: ' + href));
            document.head.appendChild(link);
        });
    }

    function assetLoader() {
        if (window.AuraLazyAssets) return window.AuraLazyAssets;
        return { loadScript: fallbackLoadScript, loadStyle: fallbackLoadStyle };
    }

    function loadScript(src) {
        return assetLoader().loadScript(src);
    }

    function loadStyle(href) {
        return assetLoader().loadStyle(href);
    }

    function loadScriptsInOrder(scripts) {
        return (scripts || []).reduce((promise, src) => {
            return promise.then(() => {
                const bundleLabel = Object.keys(bundlePaths).find(label => bundlePaths[label] === src);
                return bundleLabel ? loadBundle(bundleLabel, src) : loadScript(src);
            });
        }, Promise.resolve());
    }

    function loadStyles(styles) {
        return Promise.all((styles || []).map(href => loadStyle(href)));
    }

    function loadBundle(label, src) {
        label = String(label || 'module');
        src = src || bundlePaths[label];
        if (!src) return Promise.reject(new Error('Desktop bundle is not prebuilt: ' + label));
        const cacheKey = 'bundle:' + label + ':' + src;
        if (modulePromises.has(cacheKey)) return modulePromises.get(cacheKey);
        const promise = assetLoader().loadScript(src)
            .then(() => {
                window.dispatchEvent(new CustomEvent('aurago:module-loaded', { detail: { label } }));
            })
            .catch(err => {
                modulePromises.delete(cacheKey);
                console.error('Failed to load desktop bundle', label, err);
                window.dispatchEvent(new CustomEvent('aurago:module-load-error', { detail: { label, error: err } }));
                throw err;
            });
        modulePromises.set(cacheKey, promise);
        return promise;
    }

    function loadScriptParts(label, _parts) {
        return loadBundle(label, bundlePaths[String(label || '')]);
    }

    function loadAppAssets(appId) {
        const assets = DESKTOP_APP_ASSETS[appId];
        if (!assets) {
            readyApps.add(appId);
            return Promise.resolve();
        }
        if (readyApps.has(appId)) return Promise.resolve();
        if (appPromises.has(appId)) return appPromises.get(appId);
        const promise = loadStyles(assets.styles)
            .then(() => loadScriptsInOrder(assets.scripts))
            .then(() => {
                readyApps.add(appId);
            })
            .catch(err => {
                appPromises.delete(appId);
                console.error('Failed to load desktop app assets', appId, err);
                throw err;
            });
        appPromises.set(appId, promise);
        return promise;
    }

    function loadAppScript(appId) {
        return loadAppAssets(appId);
    }

    function appAssetsReady(appId) {
        return readyApps.has(appId) || !DESKTOP_APP_ASSETS[appId];
    }

    window.AuraDesktopModules.DESKTOP_APP_ASSETS = DESKTOP_APP_ASSETS;
    window.AuraDesktopModules.loadBundle = loadBundle;
    window.AuraDesktopModules.loadScriptParts = loadScriptParts;
    window.AuraDesktopModules.loadAppAssets = loadAppAssets;
    window.AuraDesktopModules.loadAppScript = loadAppScript;
    window.AuraDesktopModules.appAssetsReady = appAssetsReady;
})();
