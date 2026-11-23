/* Code Studio LSP protocol helpers (IDE package 2b): language mapping, workspace URIs, LSP diagnostics to Code
   Studio problems and LSP positions. Pure functions without DOM access. The same file runs inside the code-studio
   bundle (window.CodeStudioLSPProtocol) and under node (lsp-protocol.test.cjs). */
(function (root, factory) {
    'use strict';
    var api = factory();
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioLSPProtocol = api;
})(typeof window !== 'undefined' ? window : null, function () {
    'use strict';

    var ROOT_URI = 'file:///workspace';
    var LANGUAGES = ['go', 'typescript', 'python', 'cpp'];
    // One problem source per server, distinct from the container checks (go build, tsc, py_compile, gcc, ...).
    var SOURCES = { go: 'gopls', typescript: 'typescript', python: 'pyright', cpp: 'clangd' };
    var LANGUAGE_BY_EXTENSION = {
        go: 'go',
        ts: 'typescript', tsx: 'typescript', js: 'typescript', jsx: 'typescript', mjs: 'typescript', cjs: 'typescript',
        py: 'python',
        c: 'cpp', cc: 'cpp', cpp: 'cpp', cxx: 'cpp', h: 'cpp', hpp: 'cpp'
    };
    // LSP language IDs. typescript-language-server takes the script kind (TS, TSX, JS, JSX) from them.
    var LANGUAGE_ID_BY_EXTENSION = {
        go: 'go',
        ts: 'typescript', tsx: 'typescriptreact', js: 'javascript', jsx: 'javascriptreact', mjs: 'javascript', cjs: 'javascript',
        py: 'python',
        c: 'c', h: 'c', cc: 'cpp', cpp: 'cpp', cxx: 'cpp', hpp: 'cpp'
    };
    // LSP DiagnosticSeverity 1 Error, 2 Warning, 3 Information, 4 Hint; the Problems panel knows error, warning, info.
    var SEVERITIES = { 1: 'error', 2: 'warning', 3: 'info', 4: 'info' };

    function own(map, key) {
        return Object.prototype.hasOwnProperty.call(map, key);
    }

    function extensionOf(path) {
        var name = String(path == null ? '' : path).split('/').pop();
        var dot = name.lastIndexOf('.');
        return dot > 0 ? name.slice(dot + 1).toLowerCase() : '';
    }

    function languageForPath(path) {
        var ext = extensionOf(path);
        return own(LANGUAGE_BY_EXTENSION, ext) ? LANGUAGE_BY_EXTENSION[ext] : null;
    }

    function languageIdForPath(path) {
        var ext = extensionOf(path);
        return own(LANGUAGE_ID_BY_EXTENSION, ext) ? LANGUAGE_ID_BY_EXTENSION[ext] : null;
    }

    function sourceForLanguage(language) {
        return own(SOURCES, language) ? SOURCES[language] : '';
    }

    // An absolute container path below /workspace without dot segments, or null.
    function cleanPath(path) {
        var value = typeof path === 'string' ? path : '';
        if (!value || value.charAt(0) !== '/' || value.indexOf('\\') !== -1 || value.indexOf('\0') !== -1) return null;
        var parts = value.split('/').filter(function (part) { return part !== ''; });
        if (!parts.length || parts[0] !== 'workspace') return null;
        for (var i = 0; i < parts.length; i++) {
            if (parts[i] === '.' || parts[i] === '..') return null;
        }
        return '/' + parts.join('/');
    }

    // encodeURIComponent plus !'()*, the way vscode-uri (TypeScript and Pyright servers) writes file URIs.
    function encodeSegment(segment) {
        return encodeURIComponent(segment).replace(/[!'()*]/g, function (ch) {
            return '%' + ch.charCodeAt(0).toString(16).toUpperCase();
        });
    }

    function uriForPath(path) {
        var clean = cleanPath(path);
        return clean ? 'file://' + clean.split('/').map(encodeSegment).join('/') : null;
    }

    function pathForUri(uri) {
        var match = /^file:\/\/(?:localhost)?(\/[^?#]*)$/i.exec(typeof uri === 'string' ? uri : '');
        if (!match) return null;
        var segments = match[1].split('/');
        for (var i = 0; i < segments.length; i++) {
            var decoded;
            try {
                decoded = decodeURIComponent(segments[i]);
            } catch (err) {
                return null;
            }
            if (decoded.indexOf('/') !== -1) return null;
            segments[i] = decoded;
        }
        return cleanPath(segments.join('/'));
    }

    function count(value) {
        var number = Math.floor(Number(value));
        return isFinite(number) && number > 0 ? number : 0;
    }

    // The diagnostic as the server sent it: quick fixes (plan 2c) send it back in the textDocument/codeAction context.
    function rawDiagnostic(item, start, end) {
        var raw = {
            range: {
                start: { line: start.line, character: start.character },
                end: { line: end.line, character: end.character }
            },
            message: item.message
        };
        if (item.severity !== undefined) raw.severity = item.severity;
        if (item.code !== undefined) raw.code = item.code;
        if (item.source !== undefined) raw.source = item.source;
        if (item.data !== undefined) raw.data = item.data;
        return raw;
    }

    function diagnosticsToProblems(path, diagnostics, source) {
        var file = String(path == null ? '' : path);
        var origin = String(source == null ? '' : source);
        var problems = [];
        (Array.isArray(diagnostics) ? diagnostics : []).forEach(function (item) {
            if (!item || !item.range || !item.range.start) return;
            var message = String(item.message == null ? '' : item.message).trim();
            if (!message) return;
            var start = item.range.start;
            var end = item.range.end || start;
            problems.push({
                file: file,
                line: count(start.line) + 1,
                column: count(start.character) + 1,
                endLine: count(end.line) + 1,
                endColumn: count(end.character) + 1,
                severity: own(SEVERITIES, item.severity) ? SEVERITIES[item.severity] : 'error',
                message: message,
                source: origin,
                lsp: rawDiagnostic(item, start, end)
            });
        });
        return problems;
    }

    // LSP positions count UTF-16 code units, like CodeMirror offsets, so no conversion beyond the line lookup.
    function positionToOffset(doc, position) {
        if (!doc || !position) return 0;
        var line = Math.floor(Number(position.line));
        if (!isFinite(line) || line < 0) return 0;
        if (line >= doc.lines) return doc.length;
        var info = doc.line(line + 1);
        var character = Math.floor(Number(position.character));
        if (!isFinite(character) || character < 0) character = 0;
        return info.from + Math.min(character, info.to - info.from);
    }

    function offsetToPosition(doc, offset) {
        if (!doc) return { line: 0, character: 0 };
        var value = Math.floor(Number(offset));
        if (!isFinite(value) || value < 0) value = 0;
        if (value > doc.length) value = doc.length;
        var info = doc.lineAt(value);
        return { line: info.number - 1, character: value - info.from };
    }

    return {
        ROOT_URI: ROOT_URI,
        LANGUAGES: LANGUAGES.slice(),
        SOURCES: { go: SOURCES.go, typescript: SOURCES.typescript, python: SOURCES.python, cpp: SOURCES.cpp },
        languageForPath: languageForPath,
        languageIdForPath: languageIdForPath,
        sourceForLanguage: sourceForLanguage,
        uriForPath: uriForPath,
        pathForUri: pathForUri,
        diagnosticsToProblems: diagnosticsToProblems,
        positionToOffset: positionToOffset,
        offsetToPosition: offsetToPosition
    };
});
