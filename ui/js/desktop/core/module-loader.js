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
            styles: appStyles('/css/quill.snow.css', '/css/desktop-app-office.css'),
            scripts: ['/js/vendor/quill.js', '/js/desktop/apps/writer.js']
        },
        'sheets': {
            styles: appStyles('/css/desktop-app-office.css'),
            scripts: [
                '/js/desktop/apps/sheets-formulas.js',
                '/js/desktop/apps/sheets-format.js',
                '/js/desktop/apps/sheets-search.js',
                '/js/desktop/apps/sheets.js'
            ]
        },
        'settings': {
            styles: appStyles('/css/desktop-app-settings.css'),
            scripts: ['/js/desktop/apps/settings.js']
        },
        'calculator': {
            styles: appStyles('/css/desktop-app-calculator.css'),
            scripts: ['/js/desktop/apps/calculator.js']
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
        'live-speech': {
            styles: appStyles('/css/realtime-speech.css', '/css/desktop-app-live-speech.css'),
            scripts: [
                '/js/realtime-speech/vendor/ort.wasm.min.js',
                '/js/realtime-speech/silero-vad.js',
                '/js/realtime-speech/audio-engine.js',
                '/js/realtime-speech/provider-common.js',
                '/js/realtime-speech/provider-openai.js',
                '/js/realtime-speech/provider-xai.js',
                '/js/realtime-speech/provider-gemini.js',
                '/js/realtime-speech/core.js',
                '/js/realtime-speech/panel.js',
                '/js/desktop/apps/live-speech.js'
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
        'teevee': {
            styles: appStyles('/css/teevee.css'),
            scripts: ['/js/vendor/hls.min.js', '/js/desktop/core/media-helpers.js', '/js/desktop/apps/teevee.js']
        },
        'looper': {
            styles: appStyles('/css/desktop-app-looper.css'),
            scripts: ['/js/desktop/apps/looper.js']
        },
        'viewer': {
            styles: appStyles('/css/desktop-app-viewer.css'),
            scripts: ['/js/vendor/pdf.min.js', '/js/vendor/markdown-it.min.js', '/js/vendor/purify.min.js', '/js/desktop/apps/viewer.js']
        },
        'camera': {
            styles: appStyles('/css/camera.css'),
            scripts: ['/js/desktop/apps/camera.js']
        },
        'sip-phone': {
            styles: appStyles('/css/desktop-app-sip-phone.css'),
            scripts: ['/js/desktop/apps/sip-phone.js']
        },
        'network-cameras': {
            styles: appStyles('/css/desktop-app-network-cameras.css'),
            scripts: ['/js/desktop/apps/network-cameras.js']
        },
        'noisemaker': {
            styles: appStyles('/css/desktop-app-noisemaker.css'),
            scripts: ['/js/desktop/apps/noisemaker-library.js', '/js/desktop/apps/noisemaker.js']
        },
        'zipper': {
            styles: appStyles('/css/zipper.css'),
            scripts: ['/js/desktop/apps/zipper.js']
        },
        'pixel': {
            styles: appStyles('/css/pixel.css'),
            scripts: [
                '/js/desktop/apps/pixel-state.js',
                '/js/desktop/apps/pixel-view.js',
                '/js/desktop/apps/pixel-canvas.js',
                '/js/desktop/apps/pixel-tools.js',
                '/js/desktop/apps/pixel-actions.js',
                '/js/desktop/apps/pixel-events.js',
                '/js/desktop/apps/pixel.js'
            ]
        },
        'galaxa-deluxe': {
            styles: appStyles('/css/galaxa-deluxe.css'),
            scripts: [
                '/js/desktop/apps/galaxa-constants.js',
                '/js/desktop/apps/galaxa-tweens.js',
                '/js/desktop/apps/galaxa-audio.js',
                '/js/desktop/apps/galaxa-sprites.js',
                '/js/desktop/apps/galaxa-background.js',
                '/js/desktop/apps/galaxa-entities-core.js',
                '/js/desktop/apps/galaxa-entities-spawning.js',
                '/js/desktop/apps/galaxa-entities-behaviors.js',
                '/js/desktop/apps/galaxa-entities.js',
                '/js/desktop/apps/galaxa-render-effects.js',
                '/js/desktop/apps/galaxa-render-stage.js',
                '/js/desktop/apps/galaxa-render-hud.js',
                '/js/desktop/apps/galaxa-render.js',
                '/js/desktop/apps/galaxa-shop.js',
                '/js/desktop/apps/galaxa-relics.js',
                '/js/desktop/apps/galaxa-demo.js',
                '/js/desktop/apps/galaxa-supers.js',
                '/js/desktop/apps/galaxa-biome-transitions.js',
                '/js/desktop/apps/galaxa-combo-ladder.js',
                '/js/desktop/apps/galaxa-adaptive-music.js',
                '/js/desktop/apps/galaxa-fx.js',
                '/js/desktop/apps/galaxa-game.js',
                '/js/desktop/apps/galaxa-deluxe.js'
            ]
        },
        'chess': {
            styles: appStyles('/css/cm-chessboard.css', '/css/desktop-app-chess.css'),
            scripts: [
                '/js/desktop/apps/chess-engine.js',
                '/js/desktop/apps/chess-agent.js',
                '/js/desktop/apps/chess-fx.js',
                '/js/desktop/apps/chess.js'
            ]
        },
        'nasscad': {
            styles: appStyles('/css/desktop-app-nasscad.css'),
            scripts: ['/js/desktop/apps/nasscad.js']
        },
        'openscad': {
            styles: appStyles('/css/desktop-app-openscad.css', '/css/stl-viewer.css'),
            scripts: [
                '/js/vendor/three.min.js',
                '/js/vendor/STLLoader.min.js',
                '/js/vendor/OrbitControls.min.js',
                '/js/desktop/apps/openscad-defines.js',
                '/js/desktop/apps/openscad-editor.js',
                '/js/desktop/apps/openscad.js'
            ]
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
        'virtual-computers': {
            styles: appStyles('/css/xterm.css', '/css/desktop-app-virtual-computers.css'),
            scripts: [
                '/js/vendor/xterm.min.js',
                '/js/vendor/xterm-addon-fit.min.js',
                '/js/vendor/novnc.min.js',
                '/js/desktop/apps/virtual-computers-terminal.js',
                '/js/desktop/apps/virtual-computers-vnc.js',
                '/js/desktop/apps/virtual-computers.js'
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
            scripts: ['/js/desktop/apps/mission-control-modal.js', '/js/desktop/apps/mission-control.js']
        },
        'cheater': {
            styles: appStyles('/css/desktop-app-cheater.css', '/css/hljs-github.min.css'),
            scripts: [
                '/js/vendor/marked.min.js',
                '/js/vendor/purify.min.js',
                '/js/vendor/highlight.min.js',
                '/js/desktop/apps/cheater.js',
                '/js/desktop/apps/cheater-templates.js',
                '/js/desktop/apps/cheater-toolbar.js',
                '/js/desktop/apps/cheater-spotlight.js',
                '/js/desktop/apps/cheater-attachments.js'
            ]
        },
        'homepage-studio': {
            styles: appStyles('/css/desktop-app-homepage-studio.css', '/css/chat-modules.css', '/css/hljs-github-dark.min.css'),
            scripts: [
                '/js/vendor/markdown-it.min.js',
                '/js/vendor/highlight.min.js',
                '/js/shared/render-markdown.js',
                '/js/shared/chat-core.js',
                '/js/shared/chat-stream-parser.js',
                '/js/desktop/chat-renderer.js',
                '/js/desktop/apps/homepage-studio.js'
            ]
        },
        'pet-picker': {
            styles: appStyles('/css/desktop-app-pet-picker.css'),
            scripts: ['/js/desktop/apps/pet-picker.js']
        },
        'system-world': {
            styles: appStyles('/css/desktop-app-sysworld.css'),
            scripts: [
                '/js/vendor/three.min.js',
                '/js/vendor/OrbitControls.min.js',
                '/js/desktop/apps/sysworld-effects.js',
                '/js/desktop/apps/sysworld-scene.js',
                '/js/desktop/apps/sysworld-core.js',
                '/js/desktop/apps/sysworld-orbit.js',
                '/js/desktop/apps/sysworld-graph.js',
                '/js/desktop/apps/sysworld-fleet.js',
                '/js/desktop/apps/sysworld-hud.js',
                '/js/desktop/apps/sysworld.js'
            ]
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

    // App id → i18n section prefixes to merge into window.I18N before scripts run.
    // Shell only embeds desktop.*; apps load their own sections on demand.
    const APP_I18N_SECTIONS = {
        'agent-chat': ['chat'],
        'cheater': ['cheater'],
        'chess': [],
        'code-studio': ['codeStudio'],
        'galaxa-deluxe': ['galaxa'],
        'homepage-studio': ['homepage_studio'],
        'live-speech': [],
        'sip-phone': ['sip_phone'],
        'mission-control': ['missions'],
        'pixel': ['pixel'],
        'system-world': ['sysworld'],
        'viewer': ['viewer'],
        'viewer-3d': ['viewer'],
        'zipper': ['zipper']
    };
    const loadedI18nSections = new Set();

    function loadAppI18nSections(appId) {
        const sections = APP_I18N_SECTIONS[appId];
        if (!sections || !sections.length) return Promise.resolve();
        const pending = sections.filter(s => s && !loadedI18nSections.has(s));
        if (!pending.length) return Promise.resolve();
        const lang = (window.SYSTEM_LANG || document.documentElement.lang || 'en').toString();
        const url = '/api/i18n?lang=' + encodeURIComponent(lang) +
            '&sections=' + encodeURIComponent(pending.join(','));
        return fetch(url, { credentials: 'same-origin' })
            .then(resp => {
                if (!resp.ok) throw new Error('i18n sections HTTP ' + resp.status);
                return resp.json();
            })
            .then(body => {
                const data = (body && body.data) || {};
                window.I18N = Object.assign({}, window.I18N || {}, data);
                pending.forEach(s => loadedI18nSections.add(s));
            })
            .catch(err => {
                console.warn('Desktop app i18n sections failed to load', appId, pending, err);
            });
    }

    function loadAppAssets(appId) {
        const assets = DESKTOP_APP_ASSETS[appId];
        if (!assets) {
            // Still try i18n for apps without dedicated asset bundles.
            return loadAppI18nSections(appId).then(() => {
                readyApps.add(appId);
            });
        }
        if (readyApps.has(appId)) return Promise.resolve();
        if (appPromises.has(appId)) return appPromises.get(appId);
        const promise = loadAppI18nSections(appId)
            .then(() => loadStyles(assets.styles))
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
    window.AuraDesktopModules.loadAppI18nSections = loadAppI18nSections;
    window.AuraDesktopModules.loadAppScript = loadAppScript;
    window.AuraDesktopModules.appAssetsReady = appAssetsReady;
})();
