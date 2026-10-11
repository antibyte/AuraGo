// CYD Creator code editor: file tree (your files under src/app, generated and template files
// read-only with a hint), CodeMirror 6 with C++ (textarea fallback), 1 s autosave with If-Match
// sha and conflict choices, new/rename/delete under src/app, problem markers next to the lines,
// and ensureHook() for the designer's "Create hook".
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const HOOK_FILE = 'src/app/app_events.cpp';
    const APP_DIR = 'src/app/';
    const FILE_NAME = /^(?:[A-Za-z0-9_][A-Za-z0-9_-]*\/)*[A-Za-z0-9_][A-Za-z0-9_.-]*\.(?:cpp|h|hpp|c)$/;
    const HOOK_HEADER = [
        '// Event hooks of the CYD Creator layout. "Create hook" in the designer adds a function',
        '// here for every logic event; the prototypes live in src/generated/ui.h.',
        '#include <cyd.h>',
        '#include "../generated/ui.h"',
        ''
    ].join('\n');

    let codeMirrorPromise = null;
    function loadCodeMirror() {
        if (!codeMirrorPromise) {
            codeMirrorPromise = import('/js/vendor/codemirror-bundle.esm.js').catch(err => {
                codeMirrorPromise = null;
                throw err;
            });
        }
        return codeMirrorPromise;
    }

    function usesLightEditorTheme() {
        const body = document.body;
        if (!body) return false;
        if (body.dataset.theme === 'fruity') return body.dataset.fruityMode !== 'dark';
        return body.dataset.theme === 'light';
    }

    function fileGroup(info) {
        if (info.editable) return 'app';
        if (String(info.path).startsWith('lib/cyd_template/')) return 'template';
        return 'generated';
    }

    function hookStub(fn) {
        return [
            'void ' + fn + '(lv_event_t *e) {',
            '  (void)e;',
            '  cyd::log::info("' + fn + '");',
            '}',
            ''
        ].join('\n');
    }

    function lineOf(text, needle) {
        const index = String(text).indexOf(needle);
        if (index < 0) return 1;
        return String(text).slice(0, index).split('\n').length;
    }

    function create(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const state = {
            files: [],
            openPath: '',
            openInfo: null,
            saver: null,
            cm: null,
            view: null,
            textarea: null,
            roCompartment: null,
            themeCompartment: null,
            light: false,
            applying: false,
            loadToken: 0,
            problems: [],
            markerFrame: 0,
            disposed: false
        };

        // ── DOM ──
        const newBtn = el('button', { type: 'button', class: 'cc-icon-btn', dataset: { ccCodeAction: 'new' }, 'aria-label': t('cyd_creator.code.new_file'), title: t('cyd_creator.code.new_file'), text: '+' });
        const groups = el('div', { class: 'cc-tree-groups' });
        const tree = el('nav', { class: 'cc-tree', 'aria-label': t('cyd_creator.code.files') },
            el('div', { class: 'cc-tree-head' }, el('span', { text: t('cyd_creator.code.files') }), newBtn),
            groups);
        const pathLabel = el('span', { class: 'cc-code-path', dataset: { ccCodePath: '' } });
        const chip = el('span', { class: 'cc-pill', role: 'status', dataset: { ccCodeState: '' }, hidden: true });
        const saveBtn = el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccCodeAction: 'save' }, hidden: true, text: t('cyd_creator.code.save_now') });
        const hint = el('p', { class: 'cc-code-hint', dataset: { ccCodeHint: '' }, hidden: true });
        const editorHost = el('div', { class: 'cc-code-editor', dataset: { ccCodeEditor: '' } });
        const markers = el('div', { class: 'cc-code-markers', 'aria-hidden': 'true' });
        const empty = el('div', { class: 'cc-empty cc-code-empty', text: t('cyd_creator.code.choose_file') });
        const main = el('section', { class: 'cc-code-main' },
            el('div', { class: 'cc-code-bar' }, pathLabel, chip, el('span', { class: 'cc-dock-spacer' }), saveBtn),
            hint,
            el('div', { class: 'cc-code-surface' }, editorHost, markers),
            empty);
        const root = el('div', { class: 'cc-code', dataset: { ccCode: '' } }, tree, main);

        function readOnlyFor(info) {
            return !info || !info.editable || inst.readonly || inst.isLocked();
        }

        // ── tree ──
        function renderTree() {
            const byGroup = { app: [], generated: [], template: [] };
            state.files.slice().sort((x, y) => String(x.path).localeCompare(String(y.path))).forEach(info => byGroup[fileGroup(info)].push(info));
            const counts = new Map();
            state.problems.forEach(d => { if (d.file) counts.set(d.file, (counts.get(d.file) || 0) + 1); });
            const canEdit = !inst.readonly && !inst.isLocked();
            const sections = [['app', true], ['generated', false], ['template', false]].map(([group, open]) => {
                const list = byGroup[group];
                if (!list.length && group !== 'app') return null;
                const items = list.map(info => {
                    const label = group === 'app' ? info.path.slice(APP_DIR.length) : info.path;
                    const item = el('button', {
                        type: 'button', class: 'cc-tree-item', dataset: { ccFile: info.path },
                        'aria-current': info.path === state.openPath ? 'true' : null, title: info.path
                    }, el('span', { class: 'cc-tree-name', text: label }),
                    counts.get(info.path) ? el('span', { class: 'cc-tag', dataset: { tone: 'danger' }, text: String(counts.get(info.path)) }) : null);
                    item.addEventListener('click', () => openFile(info.path));
                    const tools = group === 'app' && canEdit ? el('span', { class: 'cc-tree-tools' },
                        el('button', { type: 'button', class: 'cc-icon-btn', dataset: { ccCodeAction: 'rename', path: info.path }, 'aria-label': t('cyd_creator.code.rename_file', { name: label }), title: t('cyd_creator.code.rename'), text: '✎', onClick: () => renameFile(info) }),
                        el('button', { type: 'button', class: 'cc-icon-btn', dataset: { ccCodeAction: 'delete', path: info.path }, 'aria-label': t('cyd_creator.code.delete_file', { name: label }), title: t('cyd_creator.code.delete'), text: '×', onClick: () => deleteFile(info) })) : null;
                    return el('li', { class: 'cc-tree-row' }, item, tools);
                });
                const details = el('details', { class: 'cc-tree-group', dataset: { group }, open },
                    el('summary', { text: t('cyd_creator.code.group_' + group) }),
                    list.length ? el('ul', {}, items) : el('p', { class: 'cc-empty', text: t('cyd_creator.code.no_app_files') }));
                return details;
            });
            const openGroups = new Set(Array.from(groups.querySelectorAll('details[open]')).map(d => d.dataset.group));
            groups.replaceChildren(...sections.filter(Boolean));
            if (openGroups.size) groups.querySelectorAll('details').forEach(d => { d.open = openGroups.has(d.dataset.group) || d.dataset.group === 'app'; });
            newBtn.disabled = !canEdit;
        }

        async function refreshFiles() {
            try {
                const files = await inst.api.listFiles(inst.projectId);
                if (state.disposed) return;
                state.files = files;
                renderTree();
                const info = files.find(f => f.path === state.openPath);
                if (state.openPath && !info) { closeEditor(); return; }
                if (info) {
                    state.openInfo = info;
                    // Changed elsewhere (agent, restore, another window) and nothing local to keep: reload.
                    if (state.saver && !state.saver.dirty() && info.sha256 && info.sha256 !== state.saver.token()) await reloadOpen();
                }
            } catch (err) {
                if (!state.disposed) inst.notify('error', P.errors.message(t, err));
            }
        }

        // ── editor ──
        function editorTheme(cm, light) {
            return cm.EditorView.theme({
                '&': { height: '100%', fontSize: '13px', backgroundColor: 'var(--cc-code-bg)', color: 'var(--cc-text)' },
                '.cm-scroller': { fontFamily: 'var(--cc-mono)', lineHeight: '1.5' },
                '.cm-gutters': { backgroundColor: 'var(--cc-panel)', color: 'var(--cc-muted)', borderRight: '1px solid var(--cc-border)' },
                '.cm-activeLine': { backgroundColor: 'var(--cc-accent-faint)' },
                '.cm-activeLineGutter': { backgroundColor: 'var(--cc-accent-faint)' },
                '.cm-cursor': { borderLeftColor: 'var(--cc-accent)', borderLeftWidth: '2px' },
                '&.cm-focused': { outline: 'none' }
            }, { dark: !light });
        }

        function readOnlyExtension(cm, ro) {
            return [cm.EditorState.readOnly.of(ro), cm.EditorView.editable.of(!ro)];
        }

        // themeExtensions follows the desktop theme (Fruity light vs. dark themes), like Code Studio.
        function themeExtensions(cm, light) {
            const highlight = light
                ? (cm.syntaxHighlighting && cm.defaultHighlightStyle ? [cm.syntaxHighlighting(cm.defaultHighlightStyle)] : [])
                : (cm.oneDark ? [cm.oneDark] : []);
            return [...highlight, editorTheme(cm, light)];
        }

        function extensions(cm, ro) {
            state.light = usesLightEditorTheme();
            return [
                cm.lineNumbers(),
                cm.highlightActiveLineGutter(),
                cm.highlightSpecialChars(),
                cm.history(),
                cm.drawSelection(),
                cm.dropCursor(),
                cm.highlightActiveLine(),
                cm.indentUnit.of('  '),
                cm.closeBrackets(),
                cm.highlightSelectionMatches(),
                cm.cpp(),
                state.themeCompartment.of(themeExtensions(cm, state.light)),
                cm.keymap.of([
                    { key: 'Mod-s', preventDefault: true, run: () => { flush(); return true; } },
                    cm.indentWithTab,
                    ...(cm.closeBracketsKeymap || []),
                    ...(cm.defaultKeymap || []),
                    ...(cm.searchKeymap || []),
                    ...(cm.historyKeymap || [])
                ].filter(Boolean)),
                state.roCompartment.of(readOnlyExtension(cm, ro)),
                cm.EditorView.updateListener.of(update => {
                    if (update.docChanged && !state.applying) onEdit(update.state.doc.toString());
                    if (update.docChanged || update.geometryChanged || update.viewportChanged) scheduleMarkers();
                })
            ];
        }

        function syncTheme() {
            if (!state.view || !state.themeCompartment) return;
            const light = usesLightEditorTheme();
            if (light === state.light) return;
            state.light = light;
            state.view.dispatch({ effects: state.themeCompartment.reconfigure(themeExtensions(state.cm, light)) });
        }

        function ensureEditor() {
            if (state.view || state.textarea) return Promise.resolve();
            return loadCodeMirror().then(cm => {
                if (state.disposed || state.view) return;
                state.cm = cm;
                state.roCompartment = new cm.Compartment();
                state.themeCompartment = new cm.Compartment();
                state.view = new cm.EditorView({ state: cm.EditorState.create({ doc: '', extensions: extensions(cm, true) }), parent: editorHost });
                b.on(state.view.scrollDOM, 'scroll', scheduleMarkers, { passive: true });
            }).catch(err => {
                console.warn('CYD Creator: CodeMirror failed to load, using a plain text area', err);
                if (state.disposed || state.textarea) return;
                state.textarea = el('textarea', { class: 'cc-code-fallback', spellcheck: 'false', 'aria-label': t('cyd_creator.code.editor_label'), dataset: { ccCodeFallback: '' } });
                b.on(state.textarea, 'input', () => { if (!state.applying) onEdit(state.textarea.value); });
                b.on(state.textarea, 'keydown', event => {
                    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') { event.preventDefault(); flush(); }
                });
                editorHost.appendChild(state.textarea);
            });
        }

        function setText(text, ro) {
            state.applying = true;
            try {
                if (state.view) {
                    state.view.setState(state.cm.EditorState.create({ doc: text, extensions: extensions(state.cm, ro) }));
                } else if (state.textarea) {
                    state.textarea.value = text;
                    state.textarea.readOnly = ro;
                }
            } finally {
                state.applying = false;
            }
            scheduleMarkers();
        }

        function currentText() {
            if (state.view) return state.view.state.doc.toString();
            if (state.textarea) return state.textarea.value;
            return '';
        }

        function gotoLine(line) {
            const n = Math.max(1, Number(line) || 1);
            if (state.view) {
                const doc = state.view.state.doc;
                const pos = doc.line(Math.min(n, doc.lines)).from;
                state.view.dispatch({ selection: { anchor: pos }, effects: state.cm.EditorView.scrollIntoView(pos, { y: 'center' }) });
                state.view.focus();
            } else if (state.textarea) {
                const lines = state.textarea.value.split('\n');
                const pos = lines.slice(0, Math.min(n, lines.length) - 1).reduce((sum, l) => sum + l.length + 1, 0);
                state.textarea.focus();
                state.textarea.setSelectionRange(pos, pos);
            }
        }

        function updateReadOnly() {
            const ro = readOnlyFor(state.openInfo);
            if (state.view && state.roCompartment) {
                state.view.dispatch({ effects: state.roCompartment.reconfigure(readOnlyExtension(state.cm, ro)) });
            } else if (state.textarea) {
                state.textarea.readOnly = ro;
            }
            renderHint();
            renderTree();
        }

        function renderHint() {
            const info = state.openInfo;
            let text = '';
            if (info && !info.editable) text = t(fileGroup(info) === 'template' ? 'cyd_creator.code.template_hint' : 'cyd_creator.code.generated_hint');
            hint.textContent = text;
            hint.hidden = !text;
        }

        function renderChip(saverState) {
            const s = saverState || (state.saver ? state.saver.state() : '');
            chip.hidden = !s || !state.openPath;
            chip.textContent = s ? t('cyd_creator.code.state_' + s) : '';
            chip.dataset.state = s || '';
            // The saver's classification decides the state; the reason (e.g. a project fenced for
            // publication review) is the chip's tooltip.
            const err = (s === 'failed' || s === 'offline') && state.saver ? state.saver.error() : null;
            chip.title = err ? P.errors.message(t, err) : '';
            saveBtn.hidden = !(s === 'failed' || s === 'conflict' || s === 'offline');
        }

        function makeSaver(path, sha) {
            return P.autosave.create({
                token: sha,
                save: (text, token) => inst.api.writeFile(inst.projectId, path, text, token),
                onState: s => { if (path === state.openPath) renderChip(s); inst.bus.emit('code:state', s); },
                onSaved: token => {
                    const info = state.files.find(f => f.path === path);
                    if (info) info.sha256 = token;
                },
                onConflict: () => resolveConflict(path)
            });
        }

        async function resolveConflict(path) {
            const d = P.dialog.open(inst.root, {
                title: t('cyd_creator.code.conflict_title'),
                body: el('p', { text: t('cyd_creator.code.conflict_body', { path }) }),
                actions: [
                    { id: 'keep', label: t('cyd_creator.code.conflict_keep') },
                    { id: 'reload', label: t('cyd_creator.code.conflict_reload') },
                    { id: 'overwrite', label: t('cyd_creator.code.conflict_overwrite'), kind: 'danger' }
                ]
            });
            const choice = await d.promise;
            if (choice !== 'reload' && choice !== 'overwrite') return { action: 'keep' };
            try {
                const server = await inst.api.readFile(inst.projectId, path);
                if (choice === 'reload' && path === state.openPath) setText(server.text, readOnlyFor(state.openInfo));
                return { action: choice, token: server.sha };
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
                return { action: 'keep' };
            }
        }

        function onEdit(text) {
            if (!state.saver || readOnlyFor(state.openInfo)) return;
            state.saver.change(text);
        }

        function installDocument(path, text, sha) {
            if (state.saver) state.saver.dispose();
            state.openPath = path;
            state.openInfo = state.files.find(f => f.path === path) || { path, editable: false, generated: true };
            state.saver = state.openInfo.editable ? makeSaver(path, sha) : null;
            pathLabel.textContent = path;
            empty.hidden = true;
            editorHost.hidden = false;
            setText(text, readOnlyFor(state.openInfo));
            renderHint();
            renderChip();
            renderTree();
        }

        function closeEditor() {
            if (state.saver) state.saver.dispose();
            state.saver = null;
            state.openPath = '';
            state.openInfo = null;
            pathLabel.textContent = '';
            setText('', true);
            empty.hidden = false;
            editorHost.hidden = true;
            renderHint();
            renderChip('');
            renderTree();
        }

        async function reloadOpen() {
            const path = state.openPath;
            if (!path) return;
            const server = await inst.api.readFile(inst.projectId, path);
            if (state.disposed || path !== state.openPath) return;
            if (state.saver && state.saver.dirty()) return;
            const line = state.view ? state.view.state.doc.lineAt(state.view.state.selection.main.head).number : 1;
            installDocument(path, server.text, server.sha);
            if (line > 1) gotoLine(line);
        }

        // flush saves the open file now. False when it could not be saved (offline, conflict, locked).
        function flush() {
            return state.saver ? state.saver.flush() : Promise.resolve(true);
        }

        async function leaveCurrent() {
            if (!state.saver || !state.saver.dirty()) return true;
            if (await flush()) return true;
            return inst.confirm({ titleKey: 'cyd_creator.code.unsaved_title', bodyKey: 'cyd_creator.code.unsaved_body', vars: { path: state.openPath }, okKey: 'cyd_creator.code.discard', danger: true });
        }

        async function openFile(path, line) {
            await ensureEditor();
            if (state.disposed) return false;
            if (path === state.openPath) {
                if (line) gotoLine(line);
                return true;
            }
            if (!(await leaveCurrent())) return false;
            const token = ++state.loadToken;
            root.classList.add('is-loading');
            try {
                if (!state.files.some(f => f.path === path)) await refreshFiles();
                const server = await inst.api.readFile(inst.projectId, path);
                if (token !== state.loadToken || state.disposed) return false;
                installDocument(path, server.text, server.sha);
                if (line) gotoLine(line);
                return true;
            } catch (err) {
                if (token === state.loadToken && !state.disposed) inst.notify('error', P.errors.message(t, err));
                return false;
            } finally {
                if (token === state.loadToken) root.classList.remove('is-loading');
            }
        }

        // ── file actions ──
        function nameDialog(titleKey, value) {
            const input = el('input', { class: 'cc-input', type: 'text', value: value || '', autofocus: true, spellcheck: 'false', 'aria-label': t('cyd_creator.code.file_name'), dataset: { ccFileName: '' } });
            const help = el('p', { class: 'cc-form-help', text: t('cyd_creator.code.file_name_help') });
            const d = P.dialog.open(inst.root, {
                title: t(titleKey),
                body: [el('label', { class: 'cc-form-field' }, el('span', { text: t('cyd_creator.code.file_name') }), input), help],
                actions: [{ id: 'cancel', label: t('cyd_creator.app.cancel') }, { id: 'ok', label: t('cyd_creator.app.ok'), kind: 'primary' }],
                onAction(id, dialog) {
                    if (id !== 'ok') { dialog.close(null); return; }
                    const name = input.value.trim().replace(/^src\/app\//, '');
                    if (!FILE_NAME.test(name) || name.split('/').includes('..')) { dialog.setError(t('cyd_creator.code.file_name_invalid')); return; }
                    if (state.files.some(f => f.path === APP_DIR + name)) { dialog.setError(t('cyd_creator.code.file_exists')); return; }
                    dialog.close(APP_DIR + name);
                }
            });
            input.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); d.el.querySelector('[data-cc-dialog-action="ok"]').click(); } });
            input.select();
            return d.promise;
        }

        async function newFile() {
            const path = await nameDialog('cyd_creator.code.new_file');
            if (!path) return;
            const content = /\.(h|hpp)$/.test(path) ? '#pragma once\n\n#include <cyd.h>\n' : '#include <cyd.h>\n';
            try {
                await inst.api.writeFile(inst.projectId, path, content, '*');
                await refreshFiles();
                await openFile(path, 1);
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            }
        }

        async function renameFile(info) {
            const target = await nameDialog('cyd_creator.code.rename', info.path.slice(APP_DIR.length));
            if (!target || target === info.path) return;
            try {
                if (state.openPath === info.path && !(await flush())) return;
                const current = await inst.api.readFile(inst.projectId, info.path);
                await inst.api.writeFile(inst.projectId, target, current.text, '*');
                await inst.api.deleteFile(inst.projectId, info.path, current.sha);
                const wasOpen = state.openPath === info.path;
                if (wasOpen) closeEditor();
                await refreshFiles();
                if (wasOpen) await openFile(target, 1);
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            }
        }

        async function deleteFile(info) {
            const ok = await inst.confirm({ titleKey: 'cyd_creator.code.delete', bodyKey: 'cyd_creator.code.delete_confirm', vars: { path: info.path }, okKey: 'cyd_creator.code.delete', danger: true });
            if (!ok) return;
            try {
                const sha = state.openPath === info.path && state.saver ? state.saver.token() : info.sha256;
                await inst.api.deleteFile(inst.projectId, info.path, sha || '*');
                if (state.openPath === info.path) closeEditor();
                await refreshFiles();
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            }
        }

        // ensureHook creates void on_<name>_<event>(lv_event_t *e) in src/app/app_events.cpp
        // unless it exists, switches to Code mode and shows the function.
        async function ensureHook(spec) {
            const name = String((spec && spec.name) || '');
            const event = String((spec && spec.event) || '');
            if (!/^[a-z][a-z0-9_]{0,31}$/.test(name) || !/^[a-z_]{1,32}$/.test(event)) throw new Error('invalid hook name');
            const fn = 'on_' + name + '_' + event;
            if (state.openPath === HOOK_FILE && !(await flush())) return false;
            let text = null;
            let sha = '*';
            try {
                const current = await inst.api.readFile(inst.projectId, HOOK_FILE);
                text = current.text;
                sha = current.sha;
            } catch (err) {
                if (!err || err.status !== 404) { inst.notify('error', P.errors.message(t, err)); return false; }
            }
            if (text === null || !new RegExp('\\b' + fn + '\\s*\\(').test(text)) {
                if (inst.readonly || inst.isLocked()) { inst.notify('error', t(inst.readonly ? 'cyd_creator.app.readonly_hint' : 'cyd_creator.app.locked_banner')); return false; }
                text = (text === null ? HOOK_HEADER : text.replace(/\s*$/, '\n')) + '\n' + hookStub(fn);
                try {
                    sha = await inst.api.writeFile(inst.projectId, HOOK_FILE, text, sha);
                } catch (err) {
                    inst.notify('error', P.errors.message(t, err));
                    return false;
                }
                await refreshFiles();
            }
            inst.setMode('code');
            await ensureEditor();
            if (state.openPath === HOOK_FILE) installDocument(HOOK_FILE, text, sha);
            else if (!(await openFile(HOOK_FILE))) return false;
            gotoLine(lineOf(text, 'void ' + fn + '('));
            return true;
        }

        // ── problem markers ──
        function scheduleMarkers() {
            if (state.markerFrame || state.disposed) return;
            state.markerFrame = requestAnimationFrame(() => { state.markerFrame = 0; paintMarkers(); });
        }

        function paintMarkers() {
            markers.replaceChildren();
            const view = state.view;
            if (!view || !state.openPath) return;
            const list = state.problems.filter(d => d.file === state.openPath && Number(d.line) >= 1);
            if (!list.length) return;
            const box = markers.getBoundingClientRect();
            const doc = view.state.doc;
            list.forEach(d => {
                const line = Math.min(Number(d.line), doc.lines);
                const block = view.lineBlockAt(doc.line(line).from);
                const top = block.top + view.documentTop - box.top;
                if (top + block.height < 0 || top > box.height) return;
                const severity = d.severity === 'error' ? 'error' : 'warning';
                markers.appendChild(el('div', {
                    class: 'cc-code-marker is-' + severity,
                    title: (d.line ? d.line + ': ' : '') + String(d.message || ''),
                    dataset: { ccMarker: String(line) },
                    style: { top: top + 'px', height: block.height + 'px' }
                }));
            });
        }

        function setProblems(list) {
            state.problems = Array.isArray(list) ? list.filter(d => d && d.file) : [];
            renderTree();
            scheduleMarkers();
        }

        // ── wiring ──
        b.on(newBtn, 'click', newFile);
        b.on(saveBtn, 'click', () => flush());
        b.add(inst.bus.on('lock:changed', locked => {
            updateReadOnly();
            if (!locked && state.saver) state.saver.unlock();
        }));
        b.add(() => { if (state.markerFrame) cancelAnimationFrame(state.markerFrame); });
        if (typeof MutationObserver === 'function' && document.body) {
            const themeObserver = new MutationObserver(syncTheme);
            themeObserver.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-fruity-mode'] });
            b.add(() => themeObserver.disconnect());
        }
        editorHost.hidden = true;

        return {
            el: root,
            mount(host) {
                host.appendChild(root);
                ensureEditor();
                return refreshFiles();
            },
            openFile,
            ensureHook,
            refreshFiles,
            reloadOpen,
            setProblems,
            updateReadOnly,
            flush,
            isDirty: () => !!(state.saver && state.saver.dirty()),
            openPath: () => state.openPath,
            text: currentText,
            dispose() {
                state.disposed = true;
                state.loadToken++;
                if (state.saver) state.saver.dispose();
                if (state.view) state.view.destroy();
                b.dispose();
                root.remove();
            }
        };
    }

    P.code = { create, HOOK_FILE };
})();
