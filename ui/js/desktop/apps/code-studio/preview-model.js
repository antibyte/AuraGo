/* Code Studio HTML preview helpers (Plan B): previewable files, workspace-relative addresses, preview URLs, the
   pane width, device scaling and the message shape of the sandboxed preview. Pure functions without DOM access.
   The same file runs inside the code-studio bundle (window.CodeStudioPreviewModel) and under node
   (preview-model.test.cjs). */
(function (root, factory) {
    'use strict';
    var api = factory();
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioPreviewModel = api;
})(typeof window !== 'undefined' ? window : null, function () {
    'use strict';

    var WORKSPACE_ROOT = '/workspace';
    var PREVIEWABLE = { html: true, htm: true, svg: true };
    var VIEWPORTS = { tablet: { width: 768, height: 1024 }, mobile: { width: 375, height: 812 } };
    var MIN_WIDTH = 280;
    var MIN_EDITOR_WIDTH = 320;
    var DEFAULT_WIDTH = 480;
    var NO_CONTAINER_MAX = 1600;
    var MESSAGE_KINDS = { console: true, error: true, rejection: true, resource: true, location: true, reloaded: true, expired: true };
    var ERROR_KINDS = { console: true, error: true, rejection: true, resource: true };
    var MAX_TEXT = 4000;
    var MAX_ENTRIES = 200;
    var REFRESH_MARGIN_MS = 10 * 60 * 1000;

    function own(map, key) {
        return Object.prototype.hasOwnProperty.call(map, key);
    }

    function extensionOf(path) {
        var name = String(path == null ? '' : path).split('?')[0].split('#')[0].split('/').pop();
        var dot = name.lastIndexOf('.');
        return dot > 0 ? name.slice(dot + 1).toLowerCase() : '';
    }

    function isPreviewablePath(path) {
        return own(PREVIEWABLE, extensionOf(path));
    }

    function relativePath(vpath) {
        var value = String(vpath == null ? '' : vpath);
        if (value === WORKSPACE_ROOT) return '';
        if (value.indexOf(WORKSPACE_ROOT + '/') === 0) return value.slice(WORKSPACE_ROOT.length + 1);
        return null;
    }

    function safeDecode(segment) {
        try { return decodeURIComponent(segment); } catch (_) { return null; }
    }

    // An address typed into the preview toolbar or reported by live.js: a workspace-relative path with an optional
    // ?query and #hash. Returns { path, directory, suffix } or null when it is empty, names another origin or scheme,
    // or contains a ".." segment (also percent-encoded).
    function parseAddress(input) {
        var raw = String(input == null ? '' : input).trim();
        if (!raw || raw.indexOf('\\') >= 0 || raw.indexOf('//') === 0 || /^[a-z][a-z0-9+.-]*:/i.test(raw)) return null;
        var cut = raw.search(/[?#]/);
        var pathPart = cut >= 0 ? raw.slice(0, cut) : raw;
        var suffix = cut >= 0 ? raw.slice(cut) : '';
        if (pathPart === WORKSPACE_ROOT || pathPart.indexOf(WORKSPACE_ROOT + '/') === 0) pathPart = pathPart.slice(WORKSPACE_ROOT.length);
        var directory = pathPart === '' || pathPart.charAt(pathPart.length - 1) === '/';
        var parts = [];
        var segments = pathPart.split('/');
        for (var i = 0; i < segments.length; i++) {
            var segment = safeDecode(segments[i]);
            if (segment === null || segment.indexOf('/') >= 0 || segment.indexOf('\\') >= 0 || segment.indexOf('\0') >= 0) return null;
            if (segment === '' || segment === '.') continue;
            if (segment === '..') return null;
            parts.push(segment);
        }
        return { path: WORKSPACE_ROOT + (parts.length ? '/' + parts.join('/') : ''), directory: directory || parts.length === 0, suffix: suffix };
    }

    function addressText(vpath, directory, suffix) {
        var rel = relativePath(vpath) || '';
        return rel + (directory && rel ? '/' : '') + String(suffix || '');
    }

    function encodePath(rel) {
        return rel ? rel.split('/').map(encodeURIComponent).join('/') : '';
    }

    function buildUrl(base, vpath, directory, suffix) {
        var rel = relativePath(vpath) || '';
        return String(base || '') + encodePath(rel) + (directory && rel ? '/' : '') + String(suffix || '');
    }

    // The pane may grow until the editor keeps MIN_EDITOR_WIDTH of the space both share (available).
    function clampWidth(width, available) {
        var numeric = Number(width);
        if (!Number.isFinite(numeric)) numeric = DEFAULT_WIDTH;
        var max = available > 0 ? Math.max(MIN_WIDTH, available - MIN_EDITOR_WIDTH) : NO_CONTAINER_MAX;
        return Math.round(Math.min(max, Math.max(MIN_WIDTH, numeric)));
    }

    function deviceScale(viewport, stageWidth, stageHeight, inset) {
        var size = own(VIEWPORTS, viewport) ? VIEWPORTS[viewport] : null;
        if (!size) return 1;
        var pad = inset == null ? 24 : inset;
        var scale = Math.min(1, (stageWidth - 2 * pad) / size.width, (stageHeight - 2 * pad) / size.height);
        if (!Number.isFinite(scale)) return 1;
        return Math.max(0.25, Math.round(scale * 100) / 100);
    }

    function normalizeMessage(data) {
        if (!data || typeof data !== 'object' || data.auragoPreview !== 1) return null;
        if (typeof data.kind !== 'string' || !own(MESSAGE_KINDS, data.kind)) return null;
        if (data.text !== undefined && typeof data.text !== 'string') return null;
        return { kind: data.kind, text: (data.text || '').slice(0, MAX_TEXT) };
    }

    function isErrorKind(kind) {
        return own(ERROR_KINDS, String(kind));
    }

    function pushEntry(list, entry, max) {
        var next = Array.isArray(list) ? list.slice() : [];
        next.push(entry);
        var limit = max || MAX_ENTRIES;
        return next.length > limit ? next.slice(next.length - limit) : next;
    }

    function tokenNeedsRefresh(expiresAt, now) {
        var expires = Date.parse(String(expiresAt || ''));
        return !Number.isFinite(expires) || expires - now < REFRESH_MARGIN_MS;
    }

    return {
        WORKSPACE_ROOT: WORKSPACE_ROOT,
        VIEWPORTS: VIEWPORTS,
        isPreviewablePath: isPreviewablePath,
        relativePath: relativePath,
        parseAddress: parseAddress,
        addressText: addressText,
        buildUrl: buildUrl,
        clampWidth: clampWidth,
        deviceScale: deviceScale,
        normalizeMessage: normalizeMessage,
        isErrorKind: isErrorKind,
        pushEntry: pushEntry,
        tokenNeedsRefresh: tokenNeedsRefresh
    };
});
