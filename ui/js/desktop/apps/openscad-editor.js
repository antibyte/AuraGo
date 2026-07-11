(function () {
    'use strict';

    var codeMirrorModulePromise = null;

    function loadCodeMirror() {
        if (!codeMirrorModulePromise) {
            codeMirrorModulePromise = import('/js/vendor/codemirror-bundle.esm.js');
        }
        return codeMirrorModulePromise;
    }

    function create(hostState, mountEl, onChangeCallback) {
        var editorView = null;
        var currentErrors = [];
        var cmModule = null;
        var disposed = false;
        var isFallback = false;
        var fallbackTextarea = null;

        function safeOnChange(text) {
            if (onChangeCallback && !disposed) {
                try { onChangeCallback(text); } catch (_) {}
            }
        }

        function applyErrorStyles() {
            if (!editorView || isFallback || !cmModule) return;
            var wrapper = editorView.dom;
            if (!wrapper) return;
            wrapper.querySelectorAll('.oscad-error-line, .oscad-warning-line').forEach(function (el) {
                el.classList.remove('oscad-error-line', 'oscad-warning-line');
            });
            if (!currentErrors.length) return;
            var doc = editorView.state.doc;
            currentErrors.forEach(function (err) {
                if (!err || typeof err.line !== 'number' || err.line < 1 || err.line > doc.lines) return;
                var line = doc.line(err.line);
                if (!line) return;
                var lineEl = wrapper.querySelector('.cm-line[data-line="' + (err.line - 1) + '"]');
                if (lineEl) {
                    lineEl.classList.add(err.severity === 'warning' ? 'oscad-warning-line' : 'oscad-error-line');
                    lineEl.title = err.message || '';
                }
            });
        }

        function setupCodeMirror(cm) {
            if (disposed) return;
            cmModule = cm;

            var extension = [
                cm.lineNumbers && cm.lineNumbers(),
                cm.highlightActiveLineGutter && cm.highlightActiveLineGutter(),
                cm.highlightSpecialChars && cm.highlightSpecialChars(),
                cm.history && cm.history(),
                cm.closeBrackets && cm.closeBrackets(),
                cm.drawSelection && cm.drawSelection(),
                cm.dropCursor && cm.dropCursor(),
                cm.highlightActiveLine && cm.highlightActiveLine(),
                cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
                cm.rectangularSelection && cm.rectangularSelection(),
                cm.crosshairCursor && cm.crosshairCursor(),
                cm.javascript && cm.javascript(),
                cm.EditorView.lineWrapping,
                cm.EditorView.theme({
                    '&': {
                        fontSize: '13px',
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                        backgroundColor: 'rgba(5,11,18,0.68)',
                        color: '#eef7f7',
                        height: '100%'
                    },
                    '.cm-scroller': {
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                        overflow: 'auto'
                    },
                    '.cm-gutters': {
                        backgroundColor: 'rgba(5,11,18,0.68)',
                        borderRight: '1px solid rgba(148,184,202,0.18)',
                        color: '#9fb0bd'
                    },
                    '.cm-activeLineGutter': { backgroundColor: 'rgba(66,215,200,0.08)' },
                    '.cm-activeLine': { backgroundColor: 'rgba(66,215,200,0.06)' },
                    '.cm-cursor': { borderLeftColor: '#42d7c8', borderLeftWidth: '2px' },
                    '.cm-selectionBackground, ::selection': { backgroundColor: 'rgba(66,215,200,0.18) !important' },
                    '.cm-focused .cm-selectionBackground': { backgroundColor: 'rgba(66,215,200,0.22) !important' },
                    '.oscad-error-line': { backgroundColor: 'rgba(255,113,128,0.16) !important', borderBottom: '1px dashed rgba(255,113,128,0.45)' },
                    '.oscad-warning-line': { backgroundColor: 'rgba(255,184,92,0.12) !important', borderBottom: '1px dashed rgba(255,184,92,0.40)' }
                })
            ];

            if (cm.keymap) {
                var keys = []
                    .concat(cm.closeBracketsKeymap || [])
                    .concat(cm.defaultKeymap || [])
                    .concat(cm.searchKeymap || [])
                    .concat(cm.historyKeymap || [])
                    .concat(cm.completionKeymap || []);
                keys.push(cm.indentWithTab);
                extension.push(cm.keymap.of(keys.filter(Boolean)));
            }

            extension.push(cm.EditorView.updateListener.of(function (update) {
                if (!update.docChanged || disposed) return;
                safeOnChange(update.state.doc.toString());
            }));

            extension = extension.filter(Boolean);

            try {
                var state = cm.EditorState.create({
                    doc: hostState.source || '',
                    extensions: extension
                });
                editorView = new cm.EditorView({
                    state: state,
                    parent: mountEl
                });
                setTimeout(function () {
                    if (!disposed) applyErrorStyles();
                }, 100);
            } catch (e) {
                createFallback();
            }
        }

        function createFallback() {
            if (disposed) return;
            isFallback = true;
            var textarea = document.createElement('textarea');
            textarea.className = 'oscad-source oscad-source-fallback';
            textarea.setAttribute('data-oscad-source', '');
            textarea.spellcheck = false;
            textarea.value = hostState.source || '';
            mountEl.innerHTML = '';
            mountEl.appendChild(textarea);
            fallbackTextarea = textarea;
            textarea.addEventListener('input', function () {
                safeOnChange(textarea.value);
            });
        }

        loadCodeMirror().then(setupCodeMirror).catch(function () {
            if (!disposed && !isFallback) createFallback();
        });

        return {
            getValue: function () {
                if (isFallback) return fallbackTextarea ? fallbackTextarea.value : (hostState.source || '');
                if (!editorView) return hostState.source || '';
                try { return editorView.state.doc.toString(); } catch (_) { return hostState.source || ''; }
            },
            setValue: function (value) {
                var newValue = value != null ? String(value) : '';
                if (isFallback) {
                    if (fallbackTextarea) fallbackTextarea.value = newValue;
                    return;
                }
                if (!editorView || !cmModule) return;
                try {
                    var current = editorView.state.doc.toString();
                    if (current !== newValue) {
                        editorView.dispatch({
                            changes: { from: 0, to: current.length, insert: newValue }
                        });
                    }
                } catch (_) {}
            },
            setErrors: function (errors) {
                currentErrors = Array.isArray(errors) ? errors.slice() : [];
                if (!isFallback && editorView) applyErrorStyles();
            },
            clearErrors: function () {
                currentErrors = [];
                if (!isFallback && editorView) applyErrorStyles();
            },
            dispose: function () {
                disposed = true;
                if (isFallback) {
                    if (fallbackTextarea && fallbackTextarea.parentNode) {
                        try { fallbackTextarea.parentNode.removeChild(fallbackTextarea); } catch (_) {}
                    }
                    fallbackTextarea = null;
                    return;
                }
                if (editorView && typeof editorView.destroy === 'function') {
                    try { editorView.destroy(); } catch (_) {}
                }
                editorView = null;
            }
        };
    }

    function parseOpenSCADErrors(stderr) {
        if (!stderr) return [];
        var results = [];
        var re = /^(ERROR|WARNING):\s*(.+?)\s*(?:in file\s+.+?,\s*)?(?:on\s+)?line\s+(\d+)/gim;
        var match;
        while ((match = re.exec(stderr)) !== null) {
            var severity = (match[1] || '').toUpperCase() === 'WARNING' ? 'warning' : 'error';
            results.push({
                line: Number(match[3]) || 0,
                message: String(match[2] || '').trim(),
                severity: severity
            });
        }
        return results;
    }

    window.OpenSCADEditor = { create: create, parse: parseOpenSCADErrors };
})();