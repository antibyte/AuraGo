/* AuraGo Code Studio preview helper, injected into previewed HTML and SVG documents.
   - Long-polls the workspace version and reloads the page; when only stylesheets changed it swaps the
     <link rel="stylesheet"> hrefs instead, so the page keeps its state.
   - Forwards console.error, uncaught errors, failed resources and unhandled rejections to Code Studio via
     postMessage ({auragoPreview: 1, kind, text}); the parent accepts them only from its own iframe.
   The page runs sandboxed with an opaque origin: it has no access to AuraGo cookies, storage or APIs. */
(function () {
    'use strict';
    if (window.__auragoPreviewLive) return;
    window.__auragoPreviewLive = true;
    var script = document.currentScript;
    if (!script) return;
    var source = script.getAttribute('src') || script.getAttribute('href') || '';
    var marker = '__aurago/live.js';
    var cut = source.lastIndexOf(marker);
    if (cut < 0) return;
    var base = new URL(source.slice(0, cut), location.href).href;
    var targetOrigin = new URL(base).origin;
    var version = Number(script.getAttribute('data-aurago-version')) || 0;
    var embedded = window.parent && window.parent !== window;
    var MAX_TEXT = 2000;

    function post(kind, text) {
        if (!embedded) return;
        try {
            window.parent.postMessage({ auragoPreview: 1, kind: kind, text: String(text == null ? '' : text).slice(0, MAX_TEXT) }, targetOrigin);
        } catch (_) {}
    }

    function describe(value) {
        if (value && value.stack) return String(value.stack);
        if (value && value.message) return String(value.message);
        if (typeof value === 'string') return value;
        try { return JSON.stringify(value); } catch (_) { return String(value); }
    }

    function relative(url) {
        var text = String(url || '');
        return text.indexOf(base) === 0 ? text.slice(base.length) : text;
    }

    var originalError = console.error;
    console.error = function () {
        var parts = [];
        for (var i = 0; i < arguments.length; i++) parts.push(describe(arguments[i]));
        post('console', parts.join(' '));
        return originalError.apply(console, arguments);
    };
    window.addEventListener('error', function (event) {
        var element = event.target;
        if (element && element !== window && element.nodeType === 1) {
            post('resource', 'Failed to load ' + relative(element.currentSrc || element.src || element.href || element.tagName));
            return;
        }
        var where = event.filename ? ' (' + relative(event.filename) + ':' + (event.lineno || 0) + ')' : '';
        post('error', (event.error ? describe(event.error) : (event.message || 'Error')) + where);
    }, true);
    window.addEventListener('unhandledrejection', function (event) {
        post('rejection', 'Unhandled rejection: ' + describe(event.reason));
    });
    post('location', relative(location.href.split('#')[0]));

    function swapStylesheets() {
        var links = document.querySelectorAll('link[rel~="stylesheet"][href]');
        for (var i = 0; i < links.length; i++) {
            (function (link) {
                var url = new URL(link.getAttribute('href'), location.href);
                url.searchParams.set('__aurago', String(version));
                var next = link.cloneNode(false);
                next.href = url.href;
                next.addEventListener('load', function () { if (link.parentNode) link.parentNode.removeChild(link); });
                next.addEventListener('error', function () { if (next.parentNode) next.parentNode.removeChild(next); });
                link.parentNode.insertBefore(next, link.nextSibling);
            })(links[i]);
        }
    }

    function poll(delay) {
        setTimeout(function () {
            fetch(base + '__aurago/version?since=' + version, { cache: 'no-store', credentials: 'omit' })
                .then(function (response) {
                    if (response.status === 401) { post('expired', ''); return null; }
                    if (!response.ok) throw new Error('HTTP ' + response.status);
                    return response.json();
                })
                .then(function (data) {
                    if (!data) return;
                    var next = Number(data.version) || 0;
                    if (next === version) { poll(0); return; }
                    var paths = Array.isArray(data.paths) ? data.paths : [];
                    var cssOnly = next > version && !data.full && paths.length > 0 && paths.every(function (p) { return /\.css$/i.test(String(p)); });
                    version = next;
                    if (cssOnly && document.querySelector('link[rel~="stylesheet"][href]')) {
                        swapStylesheets();
                        post('reloaded', 'css');
                        poll(0);
                        return;
                    }
                    location.reload();
                })
                .catch(function () { poll(Math.min(30000, Math.max(1000, delay * 2))); });
        }, delay);
    }
    poll(0);
})();
