/* Docker Manager editor: the YAML/.env editor and the workbench of the open stack. A classic script
   (window.DockerManagerEditor) with no DOM work at load.
   - mount(host, options) shows a plain textarea at once, loads CodeMirror (the shared vendored ESM bundle, a dynamic import
     like Code Studio) in the background and swaps it in; when the bundle cannot load, the textarea stays. Both have Ctrl/Cmd+S
     (save) and Ctrl/Cmd+Enter (save and deploy), the lint diagnostics as gutter marks (CodeMirror) and as a list below the
     editor (always), and a way to mark the line an operation failed on. Tab indents in CodeMirror, so Escape is the way out
     of it: it calls options.onEscape(), which moves the focus to the tab of the pane around the editor.
   - createLinter debounces the lint requests (400 ms) and drops the answer of a request that a newer one replaced.
   - createWorkbench keeps the drafts of the compose file and the .env of the open stack while the tabs switch, saves them with
     the revision they are based on, resolves a revision conflict through the dialogs and feeds the diagnostics to the editors. */
(function () {
    'use strict';

    const CODEMIRROR_URL = '/js/vendor/codemirror-bundle.esm.js';
    const LINT_DELAY_MS = 400;
    const PROBLEMS_SHOWN = 8;
    const FILES = ['compose', 'env'];
    const SEVERITY_TONE = { error: 'bad', warning: 'warn', hint: 'info' };
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

    // Same rule as Code Studio: the scheme of the desktop theme decides between the light and the dark syntax palette.
    function usesLightEditorTheme() {
        const body = document.body;
        if (!body) return false;
        if (body.dataset.scheme) return body.dataset.scheme === 'light';
        if (body.dataset.theme === 'fruity') return body.dataset.fruityMode !== 'dark';
        return body.dataset.theme === 'light';
    }

    // source() returns a promise of the lint answer; deliver(result) gets it, deliver(null, error) a failure. The newest
    // request wins: an answer that arrives after a newer schedule()/flush() is dropped, aborted requests are silent.
    function createLinter(source, deliver, delay) {
        let timer = 0, sequence = 0, disposed = false;
        async function run() {
            timer = 0;
            const mine = ++sequence;
            try {
                const result = await source();
                if (!disposed && mine === sequence) deliver(result);
            } catch (error) {
                if (disposed || mine !== sequence || (error && error.dm && error.dm.code === 'aborted')) return;
                deliver(null, error);
            }
        }
        return {
            schedule() { clearTimeout(timer); timer = setTimeout(run, delay == null ? LINT_DELAY_MS : delay); },
            flush() { clearTimeout(timer); return run(); },
            cancel() { clearTimeout(timer); timer = 0; sequence += 1; },
            dispose() { disposed = true; clearTimeout(timer); timer = 0; }
        };
    }

    function create(deps) {
        const { t, esc, stackModel: sm, views, uid } = deps;
        const load = deps.load || loadCodeMirror;
        let counter = 0;

        // options: {label, text, language: 'yaml'|'plain', readOnly, wrap, onChange(text), onSave(), onDeploy(), onEscape(), onFix(diagnostic)}
        // Diagnostics given to setDiagnostics are {line, column, severity, code, message, fix?}; message is already localized.
        function mount(host, options) {
            const o = options || {}, id = uid + '-ed' + (counter += 1), ac = new AbortController(), signal = ac.signal;
            const wrap = document.createElement('div'), area = document.createElement('div'), problems = document.createElement('div'), hint = document.createElement('p');
            wrap.className = 'dm-ed-wrap';
            area.className = 'dm-ed-area';
            problems.className = 'dm-ed-problems';
            problems.dataset.edProblems = '';
            hint.className = 'dm-hint dm-ed-hint';
            hint.id = id + '-hint';
            hint.textContent = t('editor_tab_hint');
            wrap.append(area, problems, hint);
            host.replaceChildren(wrap);
            const textarea = document.createElement('textarea');
            textarea.className = 'dm-ed-text';
            textarea.spellcheck = false;
            textarea.setAttribute('autocapitalize', 'off');
            textarea.setAttribute('autocomplete', 'off');
            textarea.setAttribute('wrap', o.wrap ? 'soft' : 'off');
            textarea.setAttribute('aria-label', o.label || '');
            textarea.setAttribute('aria-describedby', hint.id);
            textarea.value = String(o.text == null ? '' : o.text);
            textarea.readOnly = !!o.readOnly;
            area.append(textarea);
            const handle = {};
            let view = null, cm = null, schemeSlot = null, readSlot = null, light = usesLightEditorTheme(), diagnostics = [], marks = [], applying = false, disposed = false, observer = null;

            const getText = () => (view ? view.state.doc.toString() : textarea.value);
            const changed = () => { if (!applying && !disposed && o.onChange) o.onChange(getText()); };

            // ---- problems list (CodeMirror and fallback alike) ---------------------------------------------------------
            function paintProblems() {
                const counts = sm.lintCounts(diagnostics);
                if (!diagnostics.length) { problems.innerHTML = ''; problems.hidden = true; return; }
                problems.hidden = false;
                const items = diagnostics.slice(0, PROBLEMS_SHOWN).map((d, index) => {
                    const fix = d.fix && d.fix.kind === 'env_random' && o.onFix
                        ? '<button type="button" class="dm-btn dm-problem-fix" data-fix="' + index + '"><span>' + esc(t('fix_env_random', { name: d.fix.variable })) + '</span></button>' : '';
                    return '<li class="dm-problem" data-severity="' + esc(d.severity) + '"><button type="button" class="dm-problem-go" data-line="' + (Number(d.line) || 0) + '" data-col="' + (Number(d.column) || 0) + '">'
                        + views.dot(SEVERITY_TONE[d.severity] || 'idle') + '<span class="dm-problem-sev">' + esc(t('lint_sev_' + d.severity)) + '</span>'
                        + (d.line ? '<span class="dm-problem-line">' + esc(t('lint_line', { line: d.line })) + '</span>' : '') + '<span class="dm-problem-text">' + esc(d.message) + '</span></button>' + fix + '</li>';
                }).join('');
                const more = diagnostics.length > PROBLEMS_SHOWN ? '<li class="dm-hint">' + esc(t('lint_more', { count: diagnostics.length - PROBLEMS_SHOWN })) + '</li>' : '';
                problems.innerHTML = '<p class="dm-problems-summary" role="status">' + esc(t('lint_summary', { errors: counts.error, warnings: counts.warning, hints: counts.hint })) + '</p><ul class="dm-problems-list">' + items + more + '</ul>';
            }
            problems.addEventListener('click', event => {
                const go = event.target.closest('[data-line]'), fix = event.target.closest('[data-fix]');
                if (go) handle.gotoLine(Number(go.dataset.line), Number(go.dataset.col));
                else if (fix && o.onFix) o.onFix(diagnostics[Number(fix.dataset.fix)]);
            }, { signal });

            // ---- diagnostics on the gutter (CodeMirror) ----------------------------------------------------------------
            function applyMarks() {
                if (!view) return;
                const doc = view.state.doc;
                const items = diagnostics.concat(marks).map(d => {
                    const line = doc.line(Math.min(Math.max(Number(d.line) || 1, 1), doc.lines));
                    const offset = Number(d.column) > 0 ? Number(d.column) - 1 : line.text.length - line.text.trimStart().length;
                    const from = Math.min(line.from + offset, line.to);
                    return { from, to: line.to, severity: d.severity === 'hint' ? 'info' : d.severity, message: d.message, source: d.code ? 'lint' : 'operation' };
                });
                view.dispatch(cm.setDiagnostics(view.state, items));
            }

            // ---- keyboard (textarea) -----------------------------------------------------------------------------------
            const shortcut = event => {
                if (!(event.ctrlKey || event.metaKey) || event.altKey) return false;
                if (event.key.toLowerCase() === 's' && !event.shiftKey) { event.preventDefault(); if (o.onSave) o.onSave(); return true; }
                if (event.key === 'Enter') { event.preventDefault(); if (o.onDeploy) o.onDeploy(); return true; }
                return false;
            };
            textarea.addEventListener('keydown', shortcut, { signal });
            textarea.addEventListener('input', changed, { signal });

            // ---- CodeMirror --------------------------------------------------------------------------------------------
            function schemeExtensions() {
                const highlight = light ? (cm.syntaxHighlighting && cm.defaultHighlightStyle ? [cm.syntaxHighlighting(cm.defaultHighlightStyle)] : []) : (cm.oneDark ? [cm.oneDark] : []);
                return highlight;
            }
            function tokenTheme() {
                return cm.EditorView.theme({
                    '&': { height: '100%', fontSize: '13px', backgroundColor: 'var(--vd-theme-app-bg)', color: 'var(--vd-text)' },
                    '.cm-scroller': { fontFamily: 'var(--ds-font-mono, ui-monospace, SFMono-Regular, Consolas, monospace)', lineHeight: '1.5' },
                    '.cm-content': { caretColor: 'var(--vd-accent)' },
                    '.cm-cursor': { borderLeftColor: 'var(--vd-accent)', borderLeftWidth: '2px' },
                    '.cm-gutters': { backgroundColor: 'var(--vd-theme-panel-bg)', color: 'var(--vd-theme-muted)', borderRight: '1px solid var(--vd-theme-border)' },
                    '.cm-activeLine': { backgroundColor: 'color-mix(in srgb, var(--vd-accent) 8%, transparent)' },
                    '.cm-activeLineGutter': { backgroundColor: 'color-mix(in srgb, var(--vd-accent) 12%, transparent)' },
                    '&.cm-focused': { outline: 'var(--ds-focus-ring-width) solid var(--ds-color-border-focus)', outlineOffset: '-2px' },
                    '.cm-tooltip': { backgroundColor: 'var(--vd-theme-panel-bg-strong)', color: 'var(--vd-text)', border: '1px solid var(--vd-theme-border)' }
                }, { dark: !light });
            }
            function readExtensions(readOnly) { return [cm.EditorState.readOnly.of(!!readOnly), cm.EditorView.editable.of(!readOnly)]; }
            function swapToCodeMirror(module) {
                if (disposed) return;
                cm = module;
                const wasFocused = document.activeElement === textarea, caret = textarea.selectionStart || 0;
                schemeSlot = new cm.Compartment();
                readSlot = new cm.Compartment();
                const keys = [
                    { key: 'Mod-s', preventDefault: true, run: () => { if (o.onSave) o.onSave(); return true; } },
                    { key: 'Mod-Enter', preventDefault: true, run: () => { if (o.onDeploy) o.onDeploy(); return true; } },
                    // Tab indents, so without this the keyboard could never leave the editor.
                    { key: 'Escape', run: () => { if (!o.onEscape) return false; o.onEscape(); return true; } },
                    cm.indentWithTab, ...(cm.closeBracketsKeymap || []), ...(cm.defaultKeymap || []), ...(cm.searchKeymap || []), ...(cm.historyKeymap || []), ...(cm.lintKeymap || [])
                ].filter(Boolean);
                const extensions = [
                    cm.lineNumbers(), cm.highlightActiveLineGutter(), cm.highlightSpecialChars(), cm.history(), cm.drawSelection(), cm.dropCursor(), cm.highlightActiveLine(),
                    cm.indentUnit.of('  '), cm.EditorState.tabSize.of(2), cm.closeBrackets(), cm.highlightSelectionMatches(),
                    o.language === 'yaml' ? cm.yaml() : [], o.wrap ? cm.EditorView.lineWrapping : [],
                    cm.lintGutter(), cm.linter(null), schemeSlot.of(schemeExtensions()), tokenTheme(), readSlot.of(readExtensions(o.readOnly)),
                    cm.EditorView.contentAttributes.of({ 'aria-label': o.label || '', 'aria-describedby': hint.id }),
                    cm.keymap.of(keys),
                    cm.EditorView.updateListener.of(update => {
                        if (!update.docChanged || applying) return;
                        if (marks.length) { marks = []; applyMarks(); }
                        changed();
                    })
                ];
                const cmHost = document.createElement('div');
                cmHost.className = 'dm-ed-cm';
                area.append(cmHost);
                view = new cm.EditorView({ state: cm.EditorState.create({ doc: textarea.value, extensions }), parent: cmHost });
                textarea.hidden = true;
                handle.kind = 'codemirror';
                applyMarks();
                if (wasFocused) { view.focus(); view.dispatch({ selection: { anchor: Math.min(caret, view.state.doc.length) } }); }
                if (typeof MutationObserver === 'function' && document.body) {
                    observer = new MutationObserver(() => {
                        const next = usesLightEditorTheme();
                        if (next === light || !view) return;
                        light = next;
                        view.dispatch({ effects: schemeSlot.reconfigure(schemeExtensions()) });
                    });
                    observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-fruity-mode', 'data-scheme'] });
                }
            }

            Object.assign(handle, {
                kind: 'textarea',
                getText,
                setText(next) {
                    const value = String(next == null ? '' : next);
                    applying = true;
                    try {
                        if (view) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } });
                        else textarea.value = value;
                    } finally { applying = false; }
                    if (marks.length) { marks = []; applyMarks(); }
                },
                focus() { if (view) view.focus(); else textarea.focus(); },
                // Moves the caret to a line (1-based) and scrolls it into view.
                gotoLine(line, column) {
                    if (view) {
                        const doc = view.state.doc, found = doc.line(Math.min(Math.max(Number(line) || 1, 1), doc.lines));
                        const pos = Math.min(found.from + Math.max(0, (Number(column) || 1) - 1), found.to);
                        view.dispatch({ selection: { anchor: pos }, effects: cm.EditorView.scrollIntoView(pos, { y: 'center' }) });
                        view.focus();
                    } else {
                        const range = sm.lineRange(sm.lineIndex(textarea.value), line, column);
                        textarea.focus();
                        textarea.setSelectionRange(range.from, range.to);
                    }
                },
                setDiagnostics(list) { diagnostics = Array.isArray(list) ? list : []; paintProblems(); applyMarks(); },
                // The line an operation failed on: an error mark (and a list entry) until the text changes.
                markLine(line, message) {
                    marks = [{ line: Number(line) || 1, severity: 'error', message: String(message || '') }];
                    applyMarks();
                    handle.gotoLine(line);
                },
                setReadOnly(readOnly) {
                    if (!!o.readOnly === !!readOnly) return;
                    o.readOnly = !!readOnly;
                    textarea.readOnly = !!readOnly;
                    if (view) view.dispatch({ effects: readSlot.reconfigure(readExtensions(readOnly)) });
                },
                refresh() { if (view) view.requestMeasure(); },
                dispose() {
                    if (disposed) return;
                    disposed = true;
                    ac.abort();
                    observer?.disconnect();
                    view?.destroy();
                    view = null;
                    host.replaceChildren();
                }
            });
            handle.ready = load().then(swapToCodeMirror, error => {
                console.warn('Docker Manager: CodeMirror failed to load, using a plain text area', error);
            });
            paintProblems();
            return handle;
        }

        // ---- the workbench of the open stack -------------------------------------------------------------------------

        // w = {store, dialogs, toast, announce, canMutate(), onChange(), onDeploy(), onEscape()}. One workbench lives while a
        // stack is open; open(pane, stack) binds it to the pane built by Stacks.shell().
        function createWorkbench(w) {
            let pane = null, key = '', ids = { hostId: '', name: '' }, linter = null, saving = false, disposed = false;
            const drafts = {}, editors = {};
            const baseOf = file => ({ content: String(file && file.content || ''), revision: String(file && file.revision || ''), exists: !!(file && file.exists) });
            const detailOf = () => {
                const stack = w.store.state.stack;
                return stack && stack.key === key && stack.detail ? stack.detail : null;
            };
            const dirty = file => {
                const d = drafts[file];
                return !!d && d.text !== d.base.content;
            };
            const anyDirty = () => FILES.some(dirty);
            const q = selector => (pane ? pane.querySelector(selector) : null);
            const messageOf = (d, text) => Object.assign({}, d, { message: text });

            // Localizes the lint answer and hands every editor the diagnostics of its file.
            function deliver(result) {
                if (disposed || !result) return;
                const split = sm.splitDiagnostics(result.diagnostics);
                for (const file of FILES) {
                    if (editors[file]) editors[file].setDiagnostics(split[file].map(d => messageOf(d, sm.diagnosticText(d, t))));
                }
            }
            function lintSource() {
                return w.store.lint(ids.hostId, { compose: drafts.compose ? drafts.compose.text : '', env: drafts.env ? drafts.env.text : '', stack: ids.name }, 'stack');
            }

            function paintState() {
                const can = w.canMutate();
                for (const file of FILES) {
                    const d = drafts[file], isDirty = dirty(file);
                    const state = q('[data-ed-state="' + file + '"]');
                    if (state) state.textContent = !d ? '' : d.stale ? t('editor_stale') : isDirty ? t('editor_unsaved') : t('editor_saved');
                    for (const button of pane ? pane.querySelectorAll('[data-file="' + file + '"]') : []) {
                        button.hidden = !can;
                        button.disabled = saving || !can || (button.dataset.action === 'save-file' ? !isDirty : false);
                    }
                    if (editors[file]) editors[file].setReadOnly(!can);
                }
                const note = q('[data-env-note]'), detail = detailOf();
                if (note) note.textContent = detail && !detail.env.exists ? t('env_missing_note') : '';
            }

            function ensureEditor(file) {
                if (editors[file] || !pane || !drafts[file]) return;
                const surface = q('[data-ed-surface="' + file + '"]');
                if (!surface) return;
                editors[file] = mount(surface, {
                    label: file === 'compose' ? t('editor_compose_label') : t('editor_env_label'), text: drafts[file].text, language: file === 'compose' ? 'yaml' : 'plain', wrap: file === 'env',
                    readOnly: !w.canMutate(),
                    onChange: text => { drafts[file].text = text; if (linter) linter.schedule(); paintState(); w.onChange(); },
                    onSave: () => { void save([file], false); },
                    onDeploy: () => { void save(FILES.filter(dirty), true); },
                    onEscape: () => { if (w.onEscape) w.onEscape(); },
                    onFix: file === 'compose' ? diagnostic => fixVariable(diagnostic) : undefined
                });
                if (linter) linter.flush();
            }

            function fixVariable(diagnostic) {
                const name = diagnostic && diagnostic.fix && diagnostic.fix.variable;
                if (!name || !drafts.env || !w.canMutate()) return;
                const next = sm.appendEnvVariable(drafts.env.text, name, sm.randomValue(32, window.crypto));
                if (next === drafts.env.text) return;
                drafts.env.text = next;
                if (editors.env) editors.env.setText(next);
                if (linter) linter.schedule();
                paintState();
                w.onChange();
                w.announce(t('fix_env_done', { name }));
            }

            // Reads the stack as the store has it. A draft that equals the host's text is clean whatever the revision; a clean
            // draft follows a changed file; an edited draft whose file changed on the host is marked stale and meets the
            // revision conflict when it is saved.
            function sync() {
                const detail = detailOf();
                if (disposed || !detail) return;
                for (const file of FILES) {
                    const current = detail[file] || {}, d = drafts[file];
                    if (!d) { drafts[file] = { text: String(current.content || ''), base: baseOf(current), stale: false }; continue; }
                    if (d.text === String(current.content || '')) { d.base = baseOf(current); d.stale = false; }
                    else if (String(current.revision || '') !== d.base.revision) {
                        if (d.text === d.base.content) { d.base = baseOf(current); d.text = d.base.content; d.stale = false; if (editors[file]) editors[file].setText(d.text); }
                        else d.stale = true;
                    }
                }
                if (!linter) { linter = createLinter(lintSource, deliver); linter.flush(); }
                paintState();
            }

            // files: the file names to save together; deploy: run "up" after a successful save. Resolves true when everything
            // asked for is on the host (nothing dirty counts as saved).
            async function save(files, deploy) {
                if (saving || disposed || !w.canMutate()) return false;
                const names = files.filter(dirty);
                if (!names.length) { if (deploy) w.onDeploy(); return true; }
                saving = true;
                paintState();
                try {
                    for (let attempt = 0; attempt < 4; attempt += 1) {
                        const payload = {};
                        for (const file of names) payload[file] = { content: drafts[file].text, revision: drafts[file].base.revision };
                        try {
                            const revisions = await w.store.saveFiles(ids.hostId, ids.name, payload);
                            if (disposed) return false;
                            for (const file of names) {
                                drafts[file].base = { content: payload[file].content, revision: String(revisions[file] || ''), exists: true };
                                drafts[file].stale = false;
                            }
                            w.toast(t('editor_saved_toast', { files: names.map(file => (file === 'env' ? '.env' : 'compose')).join(', ') }), 'success');
                            w.announce(t('editor_saved'));
                            if (deploy) w.onDeploy();
                            return true;
                        } catch (error) {
                            const info = (error && error.dm) || {};
                            if (info.code !== 'revision_conflict' || !info.data || !info.data.current) { w.toast(views.errorText(info), 'error'); return false; }
                            const file = FILES.includes(info.data.file) ? info.data.file : names[0], current = info.data.current;
                            const choice = await w.dialogs.confirmConflict({ file, mine: drafts[file].text, theirs: String(current.content || ''), name: file === 'env' ? '.env' : String(current.name || 'compose.yaml') });
                            if (disposed) return false;
                            if (choice === 'theirs') {
                                drafts[file].base = baseOf(current);
                                drafts[file].text = drafts[file].base.content;
                                drafts[file].stale = false;
                                if (editors[file]) editors[file].setText(drafts[file].text);
                                return false;
                            }
                            if (choice !== 'mine') return false;
                            drafts[file].base = { content: String(current.content || ''), revision: String(current.revision || ''), exists: !!current.exists };
                        }
                    }
                    return false;
                } finally {
                    saving = false;
                    if (!disposed) { paintState(); w.onChange(); }
                }
            }

            return {
                // Binds to the pane of a stack; the first sync() fills the drafts.
                open(element, stack) {
                    this.close();
                    pane = element;
                    key = stack.key;
                    ids = { hostId: stack.hostId, name: stack.name };
                    disposed = false;
                    sync();
                },
                close() {
                    disposed = true;
                    linter?.dispose();
                    linter = null;
                    for (const file of FILES) { editors[file]?.dispose(); delete editors[file]; delete drafts[file]; }
                    pane = null;
                    key = '';
                    saving = false;
                },
                sync,
                // Mounts the editor of a tab the first time it is shown (a hidden CodeMirror cannot measure itself).
                showTab(tab) {
                    if (!FILES.includes(tab)) return;
                    ensureEditor(tab);
                    editors[tab]?.refresh();
                    paintState();
                },
                focusEditor(tab) { editors[tab]?.focus(); },
                // An operation failed on a line of the compose file: show the Compose tab's editor with the line marked.
                markLine(line, message) { ensureEditor('compose'); editors.compose?.markLine(line, message); },
                dirty,
                anyDirty,
                dirtyState: () => ({ compose: dirty('compose'), env: dirty('env') }),
                save: (files, deploy) => save(files || FILES, !!deploy),
                isOpen: () => !!pane && !disposed,
                key: () => key
            };
        }

        return { mount, createWorkbench };
    }

    window.DockerManagerEditor = { create, createLinter, usesLightEditorTheme, LINT_DELAY_MS };
})();
