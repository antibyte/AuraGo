// Storymaker app code tab: story.json and the code scenes in the vendored CodeMirror (the shared ESM bundle, a dynamic import;
// when it cannot load, a plain text area stays). Saving story.json sends the text with If-Match and takes the story the server
// answers with (defaults applied) back into the editor; saving a code scene sends the source with If-Match (or
// If-None-Match: * for a new file) and shows the bundler's errors at their line and column. Validation issues are shown inline:
// the JSON Pointer of an issue is walked through the text (S.model.locate), so the mark sits on the field. Drafts survive tab
// and file switches. Issue texts are server data and appear as text only. Classic script, publishes S.createCode.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const CODEMIRROR_URL = '/js/vendor/codemirror-bundle.esm.js';
    const PROBLEMS_SHOWN = 12;
    const TEMPLATE = [
        'export default function scene(api) {',
        '  api.duration(5);',
        "  api.text('Hello', 'title');",
        '}',
        ''
    ].join('\n');
    let codeMirrorPromise = null;

    function loadCodeMirror() {
        if (!codeMirrorPromise) {
            codeMirrorPromise = import(CODEMIRROR_URL).catch(error => {
                codeMirrorPromise = null;
                throw error;
            });
        }
        return codeMirrorPromise;
    }

    // The shell theme decides between the light and the dark syntax palette, like Code Studio.
    function lightScheme() {
        const body = document.body;
        if (!body) return false;
        if (body.dataset.scheme) return body.dataset.scheme === 'light';
        if (body.dataset.theme === 'fruity') return body.dataset.fruityMode !== 'dark';
        return body.dataset.theme === 'light';
    }

    // createEditor(host, options) -> handle. A text area at once, CodeMirror swapped in when its bundle has loaded.
    // options: {label, onChange(text), onSave(), load}. Diagnostics are {from, to, severity: error|warning|info, message}.
    function createEditor(host, options) {
        const o = options || {};
        const { el } = S;
        const area = el('div', { class: 'stm-editor-area' });
        const textarea = el('textarea', { class: 'stm-editor-text', spellcheck: 'false', autocapitalize: 'off', autocomplete: 'off', wrap: 'off', ariaLabel: o.label || '', dataset: { stm: 'editor-text' } });
        area.appendChild(textarea);
        host.appendChild(area);
        let view = null, cm = null, languageSlot = null, readSlot = null, schemeSlot = null, light = lightScheme(), applying = false, disposed = false, observer = null;
        let language = 'json', readOnly = false, diagnostics = [];
        const handle = { kind: 'textarea' };

        const getText = () => (view ? view.state.doc.toString() : textarea.value);
        const changed = () => { if (!applying && !disposed && o.onChange) o.onChange(getText()); };
        textarea.addEventListener('input', changed);
        textarea.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 's') {
                event.preventDefault();
                if (o.onSave) o.onSave();
            }
        });

        function languageExtension() {
            if (!cm) return [];
            return language === 'javascript' ? cm.javascript() : cm.json();
        }
        function readExtensions() { return [cm.EditorState.readOnly.of(readOnly), cm.EditorView.editable.of(!readOnly)]; }
        function schemeExtensions() {
            if (light) return cm.syntaxHighlighting && cm.defaultHighlightStyle ? [cm.syntaxHighlighting(cm.defaultHighlightStyle)] : [];
            return cm.oneDark ? [cm.oneDark] : [];
        }
        function theme() {
            return cm.EditorView.theme({
                '&': { height: '100%', fontSize: '13px', backgroundColor: 'var(--vd-mat-well-color, var(--vd-theme-app-bg))', color: 'var(--vd-text)' },
                '.cm-scroller': { fontFamily: 'var(--vd-mat-mono-font, var(--ds-font-mono, ui-monospace, SFMono-Regular, Consolas, monospace))', lineHeight: '1.5' },
                '.cm-content': { caretColor: 'var(--vd-accent)' },
                '.cm-cursor': { borderLeftColor: 'var(--vd-accent)', borderLeftWidth: '2px' },
                '.cm-gutters': { backgroundColor: 'var(--vd-mat-side-color, var(--vd-theme-panel-bg))', color: 'var(--vd-theme-muted)', borderRight: '1px solid var(--vd-theme-border)' },
                '.cm-activeLine': { backgroundColor: 'color-mix(in srgb, var(--vd-accent) 8%, transparent)' },
                '.cm-activeLineGutter': { backgroundColor: 'color-mix(in srgb, var(--vd-accent) 12%, transparent)' },
                '&.cm-focused': { outline: 'var(--ds-focus-ring-width, 2px) solid var(--ds-color-border-focus)', outlineOffset: '-2px' },
                '.cm-tooltip': { backgroundColor: 'var(--vd-theme-panel-bg-strong)', color: 'var(--vd-text)', border: '1px solid var(--vd-theme-border)' }
            }, { dark: !light });
        }
        function applyMarks() {
            if (!view) return;
            const length = view.state.doc.length;
            const items = diagnostics.map(d => {
                const from = Math.min(Math.max(0, d.from), length);
                return { from, to: Math.min(Math.max(from, d.to), length), severity: d.severity, message: d.message, source: 'storymaker' };
            });
            view.dispatch(cm.setDiagnostics(view.state, items));
        }

        function swap(module) {
            if (disposed) return;
            cm = module;
            languageSlot = new cm.Compartment();
            readSlot = new cm.Compartment();
            schemeSlot = new cm.Compartment();
            const keys = [
                { key: 'Mod-s', preventDefault: true, run: () => { if (o.onSave) o.onSave(); return true; } },
                { key: 'Escape', run: () => { if (!o.onEscape) return false; o.onEscape(); return true; } },
                cm.indentWithTab, ...(cm.closeBracketsKeymap || []), ...(cm.defaultKeymap || []), ...(cm.searchKeymap || []), ...(cm.historyKeymap || []), ...(cm.lintKeymap || [])
            ].filter(Boolean);
            const extensions = [
                cm.lineNumbers(), cm.highlightActiveLineGutter(), cm.highlightSpecialChars(), cm.history(), cm.drawSelection(), cm.highlightActiveLine(),
                cm.indentUnit.of('  '), cm.EditorState.tabSize.of(2), cm.closeBrackets(), cm.highlightSelectionMatches(),
                cm.lintGutter(), cm.linter(null),
                languageSlot.of(languageExtension()), schemeSlot.of(schemeExtensions()), theme(), readSlot.of(readExtensions()),
                cm.EditorView.contentAttributes.of({ 'aria-label': o.label || '' }),
                cm.keymap.of(keys),
                cm.EditorView.updateListener.of(update => { if (update.docChanged && !applying) changed(); })
            ];
            const wasFocused = document.activeElement === textarea;
            const caret = textarea.selectionStart || 0;
            const cmHost = el('div', { class: 'stm-editor-cm' });
            area.appendChild(cmHost);
            view = new cm.EditorView({ state: cm.EditorState.create({ doc: textarea.value, extensions }), parent: cmHost });
            textarea.hidden = true;
            handle.kind = 'codemirror';
            host.dataset.editor = 'codemirror';
            applyMarks();
            if (wasFocused) { view.focus(); view.dispatch({ selection: { anchor: Math.min(caret, view.state.doc.length) } }); }
            if (typeof MutationObserver === 'function' && document.body) {
                observer = new MutationObserver(() => {
                    const next = lightScheme();
                    if (next === light || !view) return;
                    light = next;
                    view.dispatch({ effects: schemeSlot.reconfigure(schemeExtensions()) });
                });
                observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-fruity-mode', 'data-scheme'] });
            }
            if (o.onReady) o.onReady(handle);
        }

        Object.assign(handle, {
            getText,
            setText(next) {
                const value = String(next == null ? '' : next);
                applying = true;
                try {
                    if (view) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } });
                    else textarea.value = value;
                } finally { applying = false; }
            },
            setLanguage(next) {
                language = next === 'javascript' ? 'javascript' : 'json';
                if (view) view.dispatch({ effects: languageSlot.reconfigure(languageExtension()) });
            },
            setReadOnly(next) {
                readOnly = !!next;
                textarea.readOnly = readOnly;
                if (view) view.dispatch({ effects: readSlot.reconfigure(readExtensions()) });
            },
            setDiagnostics(list) { diagnostics = Array.isArray(list) ? list : []; applyMarks(); },
            gotoOffset(offset) {
                if (view) {
                    const pos = Math.min(Math.max(0, offset), view.state.doc.length);
                    view.dispatch({ selection: { anchor: pos }, effects: cm.EditorView.scrollIntoView(pos, { y: 'center' }) });
                    view.focus();
                } else {
                    textarea.focus();
                    textarea.setSelectionRange(Math.min(offset, textarea.value.length), Math.min(offset, textarea.value.length));
                }
            },
            focus() { if (view) view.focus(); else textarea.focus(); },
            dispose() {
                if (disposed) return;
                disposed = true;
                if (observer) observer.disconnect();
                if (view) view.destroy();
                view = null;
                S.clear(host);
            }
        });
        handle.ready = (o.load || loadCodeMirror)().then(swap, error => { console.warn('Storymaker: CodeMirror failed to load, using a plain text area', error); });
        return handle;
    }

    // inst: {t, api, bus, story, etag, issues, report, readonly, projectId, codeStatus, saveStoryText(text), saveCodeScene(id, text, etag), flushEdits(), focusIssue(issue), notify(text)}
    function createCode(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const files = new Map();       // key -> {key, label, kind, sceneId, text, base, etag, loaded, loading, isNew, errors}
        let current = 'story';
        let editor = null;
        let busy = false;
        let staleMarks = false;
        let disposed = false;

        const fileList = el('ul', { class: 'stm-code-files', dataset: { stm: 'code-files' }, ariaLabel: t('storymaker.code.files') });
        const fileLabel = el('strong', { class: 'stm-code-file', dataset: { stm: 'code-file' } });
        const stateLabel = el('span', { class: 'stm-code-state', role: 'status', dataset: { stm: 'code-state' } });
        const saveButton = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'code-save' }, onClick: () => save() }, icon('check'), el('span', { text: t('storymaker.code.save') }));
        const reloadButton = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'code-reload' }, onClick: () => reloadFile() }, icon('refresh'), el('span', { text: t('storymaker.code.reload') }));
        const editorHost = el('div', { class: 'stm-editor', dataset: { stm: 'code-editor' } });
        const problems = el('div', { class: 'stm-problems', dataset: { stm: 'code-problems' } });
        const root = el('section', { class: 'stm-code', ariaLabel: t('storymaker.code.title'), dataset: { stm: 'code' } },
            el('nav', { class: 'stm-code-nav' }, el('h3', { class: 'stm-panel-title', text: t('storymaker.code.files') }), fileList),
            el('div', { class: 'stm-code-main' },
                el('div', { class: 'stm-code-bar' }, fileLabel, stateLabel, el('span', { class: 'stm-spacer' }), reloadButton, saveButton),
                editorHost, problems));

        function fileFor(key) {
            if (files.has(key)) return files.get(key);
            const sceneId = key.indexOf('scene:') === 0 ? key.slice(6) : '';
            const file = { key, label: sceneId ? sceneId + '.js' : 'story.json', kind: sceneId ? 'code' : 'story', sceneId, text: '', base: '', etag: '', loaded: false, loading: false, isNew: false, errors: [] };
            files.set(key, file);
            return file;
        }

        function isDirty(file) { return file.loaded && file.text !== file.base; }

        function codeSceneIds() {
            return S.model.scenesOf(inst.story).filter(scene => scene && scene.kind === 'code').map(scene => scene.id);
        }

        function paintFiles() {
            S.clear(fileList);
            const keys = ['story'].concat(codeSceneIds().map(id => 'scene:' + id));
            keys.forEach(key => {
                const file = fileFor(key);
                const status = file.sceneId ? (inst.codeStatus || []).find(item => item.scene_id === file.sceneId) : null;
                const failed = !!file.errors.length || (status && status.ok === false);
                fileList.appendChild(el('li', {}, el('button', {
                    type: 'button', class: 'stm-code-file-btn', dataset: { stm: 'code-file-btn', key, current: String(key === current), failed: String(!!failed), dirty: String(isDirty(file)) },
                    ariaCurrent: key === current ? 'true' : null,
                    onClick: () => show(key)
                }, icon(file.kind === 'story' ? 'film' : 'code'), el('span', { class: 'stm-code-file-name', text: file.label }), isDirty(file) ? el('span', { class: 'stm-dot', title: t('storymaker.code.unsaved') }) : null, failed ? el('span', { class: 'stm-dot stm-dot-bad', title: t('storymaker.code.bundle_failed') }) : null)));
            });
        }

        function paintState() {
            const file = fileFor(current);
            fileLabel.textContent = file.label;
            const dirty = isDirty(file);
            let text = '';
            if (file.loading) text = t('storymaker.loading');
            else if (staleMarks) text = t('storymaker.code.stale');
            else if (dirty) text = t('storymaker.code.unsaved');
            else if (file.isNew) text = t('storymaker.code.new_file');
            else if (file.loaded) text = t('storymaker.code.saved');
            stateLabel.textContent = text;
            stateLabel.dataset.state = file.loading ? 'loading' : dirty ? 'dirty' : 'saved';
            saveButton.disabled = inst.readonly || busy || !file.loaded || (!dirty && !file.isNew);
            reloadButton.disabled = busy || file.loading;
            if (editor) editor.setReadOnly(inst.readonly || file.loading);
            paintFiles();
        }

        // ---- problems ----------------------------------------------------------------------------------------------------

        function storyDiagnostics(file) {
            const text = file.text;
            const list = [];
            if (staleMarks) return list;
            const issues = S.model.visibleIssues(S.model.mergeIssues(inst.issues, inst.report ? inst.report.issues : []));
            issues.filter(issue => issue.code !== 'narration_missing').forEach(issue => {
                let from = 0, to = 0, found = false;
                const where = S.model.positionFromMessage(issue.message);
                if (!issue.path && !(issue.code === 'invalid_json' && where)) return;
                if ((issue.code === 'invalid_json') && where) {
                    from = S.model.offsetOf(text, where.line, where.column);
                    to = from + 1;
                    found = true;
                } else {
                    const hit = S.model.locate(text, issue.path);
                    from = hit.from;
                    to = hit.to;
                    found = hit.found || hit.depth > 0;
                }
                if (!found && issue.path) return;
                const lineEnd = text.indexOf('\n', from);
                to = Math.min(to, lineEnd < 0 ? text.length : lineEnd);
                list.push({ from, to: Math.max(to, from), severity: issue.severity === 'error' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info', message: S.errors.issueTitle(t, issue.code) + ': ' + issue.message, issue });
            });
            return list;
        }

        function codeDiagnostics(file) {
            return file.errors.map(error => {
                const from = S.model.offsetOf(file.text, error.line || 1, error.column || 1);
                const lineEnd = file.text.indexOf('\n', from);
                return { from, to: lineEnd < 0 ? file.text.length : Math.max(from, lineEnd), severity: 'error', message: error.message, line: error.line || 1, column: error.column || 1 };
            });
        }

        function paintProblems() {
            const file = fileFor(current);
            const list = file.kind === 'story' ? storyDiagnostics(file) : codeDiagnostics(file);
            if (editor) editor.setDiagnostics(list);
            S.clear(problems);
            problems.dataset.count = String(list.length);
            if (!list.length) {
                problems.appendChild(el('p', { class: 'stm-muted stm-problems-none', text: t('storymaker.code.no_problems') }));
                return;
            }
            problems.appendChild(el('p', { class: 'stm-problems-summary', role: 'status', text: t('storymaker.code.problem_count', { count: list.length }) }));
            const rows = el('ul', { class: 'stm-problems-list' });
            list.slice(0, PROBLEMS_SHOWN).forEach(item => {
                const position = S.model.lineColumn(file.text, item.from);
                rows.appendChild(el('li', { class: 'stm-problem', dataset: { severity: item.severity } },
                    el('button', { type: 'button', class: 'stm-problem-go', onClick: () => { if (editor) editor.gotoOffset(item.from); } },
                        icon(item.severity === 'error' ? 'alert' : 'info'),
                        el('span', { class: 'stm-problem-line', text: t('storymaker.code.line_col', { line: position.line, column: position.column }) }),
                        el('span', { class: 'stm-problem-text', text: item.message }))));
            });
            if (list.length > PROBLEMS_SHOWN) rows.appendChild(el('li', { class: 'stm-muted', text: t('storymaker.code.more', { count: list.length - PROBLEMS_SHOWN }) }));
            problems.appendChild(rows);
        }

        // ---- files -------------------------------------------------------------------------------------------------------

        function storyText() { return S.model.canonicalText(inst.story); }

        async function loadFile(file) {
            if (file.kind === 'story') {
                file.text = storyText();
                file.base = file.text;
                file.loaded = true;
                return;
            }
            file.loading = true;
            paintState();
            try {
                const result = await inst.api.getCode(inst.projectId, file.sceneId);
                file.text = result.text;
                file.base = result.text;
                file.etag = result.etag;
                file.isNew = false;
            } catch (err) {
                if (err && err.status === 404) {
                    file.text = TEMPLATE;
                    file.base = '';
                    file.etag = '';
                    file.isNew = true;
                } else {
                    file.loading = false;
                    inst.notify(S.errors.describe(t, err), 'error');
                    paintState();
                    return;
                }
            }
            file.loading = false;
            file.loaded = true;
            const status = (inst.codeStatus || []).find(item => item.scene_id === file.sceneId);
            file.errors = status && status.ok === false && Array.isArray(status.errors) ? status.errors : [];
        }

        function ensureEditor() {
            if (editor || disposed) return;
            editor = createEditor(editorHost, {
                label: t('storymaker.code.editor_label'),
                onChange(text) {
                    const file = fileFor(current);
                    if (!file.loaded) return;
                    file.text = text;
                    if (!staleMarks) { staleMarks = true; paintProblems(); }
                    paintState();
                },
                onSave: () => save(),
                onEscape: () => { const button = fileList.querySelector('[aria-current="true"]'); if (button) button.focus(); },
                onReady: () => { editor.setLanguage(fileFor(current).kind === 'code' ? 'javascript' : 'json'); editor.setReadOnly(inst.readonly); paintProblems(); }
            });
        }

        async function show(key) {
            const file = fileFor(key || current);
            current = file.key;
            ensureEditor();
            if (!file.loaded) {
                paintState();
                await loadFile(file);
                if (disposed || current !== file.key) return;
            } else if (file.kind === 'story' && !isDirty(file)) {
                // the story may have changed in the inspector meanwhile
                file.text = storyText();
                file.base = file.text;
            }
            staleMarks = false;
            editor.setLanguage(file.kind === 'code' ? 'javascript' : 'json');
            editor.setText(file.text);
            editor.setReadOnly(inst.readonly);
            paintState();
            paintProblems();
        }

        async function reloadFile() {
            const file = fileFor(current);
            file.loaded = false;
            file.errors = [];
            await show(file.key);
        }

        async function save() {
            const file = fileFor(current);
            if (busy || inst.readonly || !file.loaded) return false;
            busy = true;
            paintState();
            const sent = file.text;
            const projectId = inst.projectId;
            try {
                if (file.kind === 'story') {
                    const result = await inst.saveStoryText(sent);
                    if (disposed || inst.projectId !== projectId || result.cancelled) return false;
                    if (result.ok) {
                        staleMarks = false;
                        file.base = typeof result.text === 'string' ? result.text : sent;
                        if (file.text === sent) file.text = file.base;
                        staleMarks = isDirty(file);
                        if (fileFor(current) === file && editor && editor.getText() === sent) editor.setText(file.text);
                    } else {
                        staleMarks = file.text !== sent;
                    }
                } else {
                    const result = await inst.saveCodeScene(file.sceneId, sent, file.isNew ? null : file.etag);
                    if (disposed || inst.projectId !== projectId || result.cancelled) return false;
                    file.etag = result.etag || file.etag;
                    file.base = sent;
                    file.isNew = false;
                    file.errors = result.ok ? [] : (Array.isArray(result.errors) ? result.errors : []);
                    staleMarks = isDirty(file);
                }
            } catch (err) {
                if (!disposed && inst.projectId === projectId) inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                busy = false;
                if (!disposed && inst.projectId === projectId) { paintState(); paintProblems(); }
            }
            return true;
        }

        const off = [
            inst.bus.on('story', () => { if (!disposed) { paintFiles(); if (root.dataset.visible === 'true') paintProblems(); } }),
            inst.bus.on('issues', () => { if (!disposed && root.dataset.visible === 'true') paintProblems(); }),
            inst.bus.on('readonly', () => paintState())
        ];

        return {
            el: root,
            show(key) { root.dataset.visible = 'true'; return show(key); },
            hide() { root.dataset.visible = 'false'; },
            current: () => current,
            dirty: () => Array.from(files.values()).some(isDirty),
            // a save elsewhere changed the story: an untouched story.json follows, a draft stays
            storyChanged() {
                const file = fileFor('story');
                if (file.loaded && !busy && !isDirty(file)) { file.text = storyText(); file.base = file.text; if (editor && current === 'story' && root.dataset.visible === 'true') editor.setText(file.text); }
                paintFiles();
            },
            // put the caret on the field an issue names (story.json must be the open file)
            gotoPath(issue) {
                const file = fileFor('story');
                if (!editor || current !== 'story' || !file.loaded) return false;
                const where = S.model.positionFromMessage(issue.message);
                const offset = issue.code === 'invalid_json' && where ? S.model.offsetOf(file.text, where.line, where.column) : S.model.locate(file.text, issue.path).from;
                editor.gotoOffset(offset);
                return true;
            },
            forget(sceneId) { files.delete('scene:' + sceneId); if (current === 'scene:' + sceneId) current = 'story'; paintFiles(); },
            paint() { paintState(); paintProblems(); },
            dispose() {
                disposed = true;
                off.forEach(fn => fn());
                if (editor) editor.dispose();
                editor = null;
            }
        };
    }

    S.createCode = createCode;
    S.codeTemplate = TEMPLATE;
    S.createEditor = createEditor;
})();
