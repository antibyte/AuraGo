(function () {
    'use strict';

    var codeMirrorModulePromise = null;

    function loadCodeMirror() {
        if (!codeMirrorModulePromise) {
            codeMirrorModulePromise = import('/js/vendor/codemirror-bundle.esm.js');
        }
        return codeMirrorModulePromise;
    }

    function translate(ctx, key, fallback) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key, fallback) : fallback;
    }

    function formatErrorTitle(ctx, err) {
        var message = err && err.message ? String(err.message) : '';
        var line = err && typeof err.line === 'number' ? err.line : 0;
        var template = translate(ctx, 'desktop.openscad.error_line', 'Line {line}: {message}');
        return String(template).replace('{line}', String(line)).replace('{message}', message);
    }

    function create(hostState, mountEl, onChangeCallback) {
        var editorView = null;
        var currentErrors = [];
        var cmModule = null;
        var disposed = false;
        var isFallback = false;
        var fallbackTextarea = null;
        var editableCompartment = null;
        var readOnly = !!(hostState && hostState.ctx && hostState.ctx.readonly);
        var ctx = hostState && hostState.ctx;

        function safeOnChange(text) {
            if (readOnly || disposed || !onChangeCallback) return;
            try { onChangeCallback(text); } catch (_) {}
        }

        function applyErrorStyles() {
            if (!editorView || isFallback || !cmModule) return;
            var wrapper = editorView.dom;
            if (!wrapper) return;
            wrapper.querySelectorAll('.oscad-error-line, .oscad-warning-line').forEach(function (el) {
                el.classList.remove('oscad-error-line', 'oscad-warning-line');
                el.removeAttribute('title');
            });
            wrapper.querySelectorAll('.cm-gutterElement.oscad-error-gutter, .cm-gutterElement.oscad-warning-gutter').forEach(function (el) {
                el.classList.remove('oscad-error-gutter', 'oscad-warning-gutter');
                el.removeAttribute('title');
            });
            if (!currentErrors.length) return;
            var lines = wrapper.querySelectorAll('.cm-content .cm-line');
            var gutters = wrapper.querySelectorAll('.cm-lineNumbers .cm-gutterElement');
            var gutterOffset = gutters.length > lines.length ? gutters.length - lines.length : 0;
            currentErrors.forEach(function (err) {
                if (!err || typeof err.line !== 'number' || err.line < 1 || err.line > lines.length) return;
                var lineEl = lines[err.line - 1];
                var title = formatErrorTitle(ctx, err);
                var warning = err.severity === 'warning';
                if (lineEl) {
                    lineEl.classList.add(warning ? 'oscad-warning-line' : 'oscad-error-line');
                    lineEl.title = title;
                }
                var gutterEl = gutters[gutterOffset + err.line - 1];
                if (gutterEl) {
                    gutterEl.classList.add(warning ? 'oscad-warning-gutter' : 'oscad-error-gutter');
                    gutterEl.title = translate(ctx, 'desktop.openscad.error_gutter', 'Error on line') + ' ' + err.line;
                }
            });
        }

        function setupCodeMirror(cm) {
            if (disposed) return;
            cmModule = cm;
            editableCompartment = cm.Compartment ? new cm.Compartment() : null;

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
                    '.oscad-warning-line': { backgroundColor: 'rgba(255,184,92,0.12) !important', borderBottom: '1px dashed rgba(255,184,92,0.40)' },
                    '.oscad-error-gutter': { color: '#ff7180 !important', fontWeight: '700' },
                    '.oscad-warning-gutter': { color: '#ffb85c !important', fontWeight: '700' }
                })
            ];

            if (editableCompartment && cm.EditorView && cm.EditorView.editable) {
                extension.push(editableCompartment.of(cm.EditorView.editable.of(!readOnly)));
            }

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
                if (!update.docChanged || disposed || readOnly) return;
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
            textarea.readOnly = readOnly;
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
            setReadOnly: function (value) {
                readOnly = !!value;
                if (fallbackTextarea) fallbackTextarea.readOnly = readOnly;
                if (!editorView || !cmModule || !editableCompartment || !cmModule.EditorView || !cmModule.EditorView.editable) return;
                try {
                    editorView.dispatch({
                        effects: editableCompartment.reconfigure(cmModule.EditorView.editable.of(!readOnly))
                    });
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
