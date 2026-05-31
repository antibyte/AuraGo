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
    const commonAppStyles = ['/css/desktop-app-common.css'];

    function appStyles(...styles) {
        return commonAppStyles.concat(styles);
    }

    const DESKTOP_APP_ASSETS = {
        'files': {
            styles: appStyles('/css/desktop-app-file-manager.css'),
            scripts: ['/js/desktop/bundles/file-manager.bundle.js']
        },
        'editor': {
            styles: appStyles('/css/desktop-app-office.css')
        },
        'writer': {
            styles: appStyles('/css/desktop-app-office.css', '/css/quill.snow.css'),
            scripts: ['/js/vendor/quill.js', '/js/desktop/apps/writer.js']
        },
        'sheets': {
            styles: appStyles('/css/desktop-app-office.css'),
            scripts: ['/js/desktop/apps/sheets.js']
        },
        'settings': {
            styles: appStyles('/css/desktop-app-settings.css')
        },
        'calculator': {
            styles: appStyles('/css/desktop-app-calculator.css')
        },
        'todo': {
            styles: appStyles('/css/desktop-app-planning.css')
        },
        'calendar': {
            styles: appStyles('/css/desktop-app-planning.css')
        },
        'music-player': {
            styles: appStyles('/css/desktop-app-planning.css')
        },
        'gallery': {
            styles: appStyles('/css/desktop-app-gallery.css')
        },
        'agent-chat': {
            styles: appStyles('/css/desktop-app-chat.css', '/css/chat-modules.css', '/css/stt-overlay.css', '/css/hljs-github-dark.min.css'),
            scripts: [
                '/js/vendor/markdown-it.min.js',
                '/js/vendor/highlight.min.js',
                '/js/shared/render-markdown.js',
                '/js/shared/chat-core.js',
                '/js/shared/chat-stream-parser.js',
                '/js/chat/ui-icons.js',
                '/js/chat/modules/voice-recorder.js',
                '/js/chat/modules/speech-to-text.js',
                '/js/chat/modules/mermaid-loader.js',
                '/js/desktop/chat-renderer.js',
                '/js/desktop/apps/agent-chat.js'
            ]
        },
        'code-studio': {
            styles: appStyles('/css/code-studio.css', '/css/xterm.css', '/css/hljs-github-dark.min.css'),
            scripts: [
                '/js/vendor/xterm.min.js',
                '/js/vendor/xterm-addon-fit.min.js',
                '/js/vendor/highlight.min.js',
                '/js/desktop/bundles/code-studio.bundle.js'
            ]
        },
        'radio': {
            styles: appStyles('/css/radio.css'),
            scripts: ['/js/desktop/apps/radio.js']
        },
        'looper': {
            styles: appStyles('/css/desktop-app-looper.css'),
            scripts: ['/js/desktop/apps/looper.js']
        },
        'viewer': {
            styles: appStyles('/css/desktop-app-viewer.css'),
            scripts: ['/js/vendor/pdf.min.js', '/js/desktop/apps/viewer.js']
        },
        'camera': {
            styles: appStyles('/css/camera.css'),
            scripts: ['/js/desktop/apps/camera.js']
        },
        'zipper': {
            styles: appStyles('/css/zipper.css'),
            scripts: ['/js/desktop/apps/zipper.js']
        },
        'pixel': {
            styles: appStyles('/css/pixel.css'),
            scripts: ['/js/desktop/apps/pixel.js']
        },
        'galaxa-deluxe': {
            styles: appStyles('/css/galaxa-deluxe.css'),
            scripts: ['/js/desktop/apps/galaxa-deluxe.js']
        },
        'viewer-3d': {
            styles: appStyles('/css/stl-viewer.css'),
            scripts: [
                '/js/vendor/three.min.js',
                '/js/vendor/STLLoader.min.js',
                '/js/vendor/OrbitControls.min.js',
                '/js/desktop/apps/viewer-3d.js'
            ]
        },
        'system-info': {
            styles: appStyles('/css/desktop-app-system-info.css'),
            scripts: ['/chart.min.js', '/js/desktop/apps/system-info.js']
        },
        'quick-connect': {
            styles: appStyles('/css/desktop-app-quick-connect.css', '/css/xterm.css'),
            scripts: [
                '/js/vendor/xterm.min.js',
                '/js/vendor/xterm-addon-fit.min.js',
                '/js/vendor/novnc.min.js'
            ]
        },
        'launchpad': {
            styles: appStyles('/css/desktop-app-launchpad.css')
        },
        'software-store': {
            styles: appStyles('/css/desktop-app-software-store.css'),
            scripts: ['/js/desktop/apps/software-store.js']
        },
        'people': {
            styles: appStyles('/css/desktop-app-people.css'),
            scripts: ['/js/desktop/apps/people.js']
        },
        'mission-control': {
            styles: appStyles('/css/desktop-app-mission-control.css'),
            scripts: ['/js/desktop/apps/mission-control.js']
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
        return Promise.all((styles || []).map(href => loadStyle(href).catch(err => {
            console.warn('Desktop app stylesheet failed to load; continuing without optional CSS', href, err);
            return null;
        })));
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
