// CYD Creator code editor: file tree (your files under src/app, generated and template files
// read-only with a hint), CodeMirror 6 with C++ (textarea fallback), 1 s autosave with If-Match
// sha and conflict choices, new/rename/delete under src/app, problem markers next to the lines,
// and ensureHook() for the designer's "Create hook".
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const HOOK_FILE = 'src/app/app_events.cpp';
    const APP_DIR = 'src/app/';
    // The server's IsEditablePath (internal/cydcreator/diagnostics.go): at most four folders below
    // src/app, 160 bytes, ASCII segments that start with a letter, digit or _, do not end with a dot
    // and are no Windows device name, and a .cpp, .h, .hpp or .c file.
    const SEGMENT = /^[A-Za-z0-9_](?:[A-Za-z0-9_.-]{0,62}[A-Za-z0-9_-])?$/;
    const DEVICE = /^(?:con|prn|aux|nul|com[0-9]|lpt[0-9])$/;
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

    function isEditablePath(rel) {
        const p = String(rel || '');
        const segs = p.split('/');
        if (p.length > 160 || !p.startsWith(APP_DIR) || segs.length < 3 || segs.length > 7) return false;
        if (!segs.every(s => SEGMENT.test(s) && !DEVICE.test(s.split('.')[0].toLowerCase()))) return false;
        return /\.(?:cpp|h|hpp|c)$/.test(segs[segs.length - 1]);
    }

    // hookLine returns the line of the definition (or declaration) void fn(…), 0 when there is none;
    // a call or a mention of the bare name does not count.
    function hookLine(text, fn) {
        const s = String(text);
        const m = new RegExp('\\bvoid\\s+' + fn + '\\s*\\(').exec(s);
        return m ? s.slice(0, m.index).split('\n').length : 0;
    }

    // folderPrefixes('src/app/a/b/x.cpp') → ['src/app/a/', 'src/app/a/b/'].
    function folderPrefixes(rel) {
        const parts = String(rel).split('/');
        const out = [];
        for (let i = 3; i < parts.length; i++) out.push(parts.slice(0, i).join('/') + '/');
        return out;
    }

    // severityOf maps a diagnostic like the dock does: error, warning, everything else a note.
    function severityOf(d) {
        return d.severity === 'error' ? 'error' : (d.severity === 'warning' ? 'warning' : 'note');
    }

    function create(inst) {
        const { el, bag } = P.util;
        const t = inst.t;
        const b = bag();
        const state = {
            files: [],
            openPath: '',
            openInfo: null,
            openSha: '',
            missing: '',
            switching: 0,
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
        const newBtn = el('button', { type: 'button', class: 'cydc-icon-btn', dataset: { ccCodeAction: 'new' }, 'aria-label': t('cyd_creator.code.new_file'), title: t('cyd_creator.code.new_file'), text: '+' });
        const groups = el('div', { class: 'cydc-tree-groups' });
        const tree = el('nav', { class: 'cydc-tree', 'aria-label': t('cyd_creator.code.files') },
            el('div', { class: 'cydc-tree-head' }, el('span', { text: t('cyd_creator.code.files') }), newBtn),
            groups);
        const pathLabel = el('span', { class: 'cydc-code-path', dataset: { ccCodePath: '' } });
        const chip = el('span', { class: 'cydc-pill', role: 'status', dataset: { ccCodeState: '' }, hidden: true });
        const saveBtn = el('button', { type: 'button', class: 'cydc-btn cydc-btn--small', dataset: { ccCodeAction: 'save' }, hidden: true, text: t('cyd_creator.code.save_now') });
        const hint = el('p', { class: 'cydc-code-hint', dataset: { ccCodeHint: '' }, hidden: true });
        const editorHost = el('div', { class: 'cydc-code-editor', dataset: { ccCodeEditor: '' } });
        const markers = el('div', { class: 'cydc-code-markers', 'aria-hidden': 'true' });
        const empty = el('div', { class: 'cydc-empty cydc-code-empty', text: t('cyd_creator.code.choose_file') });
        const main = el('section', { class: 'cydc-code-main' },
            el('div', { class: 'cydc-code-bar' }, pathLabel, chip, el('span', { class: 'cydc-dock-spacer' }), saveBtn),
            hint,
            el('div', { class: 'cydc-code-surface' }, editorHost, markers),
            empty);
        const root = el('div', { class: 'cydc-code', dataset: { ccCode: '' } }, tree, main);

        // The editor is read-only while it switches or renames a file, so nothing typed between the
        // flush and the new document is lost.
        function readOnlyFor(info) {
            return !info || !info.editable || state.switching > 0 || inst.readonly || inst.isLocked();
        }

        function setSwitching(on) {
            state.switching = Math.max(0, state.switching + (on ? 1 : -1));
            updateReadOnly();
        }

        // ── tree ──
        function treeFocusKey() {
            const active = document.activeElement;
            if (!active || !active.dataset || !groups.contains(active)) return null;
            return { file: active.dataset.ccFile, action: active.dataset.ccCodeAction, path: active.dataset.path };
        }

        function restoreTreeFocus(key) {
            if (!key) return;
            const buttons = Array.from(groups.querySelectorAll('button'));
            const hit = (key.action && buttons.find(n => n.dataset.ccCodeAction === key.action && n.dataset.path === key.path)) ||
                buttons.find(n => n.dataset.ccFile === (key.file || key.path));
            (hit || buttons[0] || newBtn).focus();
        }

        function renderTree() {
            const byGroup = { app: [], generated: [], template: [] };
            state.files.slice().sort((x, y) => String(x.path).localeCompare(String(y.path))).forEach(info => byGroup[fileGroup(info)].push(info));
            const counts = new Map();
            const rank = { note: 0, warning: 1, error: 2 };
            state.problems.forEach(d => {
                const c = counts.get(d.file) || { n: 0, worst: 'note' };
                const s = severityOf(d);
                counts.set(d.file, { n: c.n + 1, worst: rank[s] > rank[c.worst] ? s : c.worst });
            });
            const tones = { error: 'danger', warning: 'warning', note: null };
            const canEdit = !inst.readonly && !inst.isLocked();
            const focusKey = treeFocusKey();
            const sections = [['app', true], ['generated', false], ['template', false]].map(([group, open]) => {
                const list = byGroup[group];
                if (!list.length && group !== 'app') return null;
                const items = list.map(info => {
                    const label = group === 'app' ? info.path.slice(APP_DIR.length) : info.path;
                    const item = el('button', {
                        type: 'button', class: 'cydc-tree-item', dataset: { ccFile: info.path },
                        'aria-current': info.path === state.openPath ? 'true' : null, title: info.path
                    }, el('span', { class: 'cydc-tree-name', text: label }),
                    counts.get(info.path) ? el('span', { class: 'cydc-tag', dataset: { tone: tones[counts.get(info.path).worst] }, text: String(counts.get(info.path).n) }) : null);
                    item.addEventListener('click', () => openFile(info.path));
                    const tools = group === 'app' && canEdit ? el('span', { class: 'cydc-tree-tools' },
                        el('button', { type: 'button', class: 'cydc-icon-btn', dataset: { ccCodeAction: 'rename', path: info.path }, 'aria-label': t('cyd_creator.code.rename_file', { name: label }), title: t('cyd_creator.code.rename'), text: '✎', onClick: () => renameFile(info) }),
                        el('button', { type: 'button', class: 'cydc-icon-btn', dataset: { ccCodeAction: 'delete', path: info.path }, 'aria-label': t('cyd_creator.code.delete_file', { name: label }), title: t('cyd_creator.code.delete'), text: '×', onClick: () => deleteFile(info) })) : null;
                    return el('li', { class: 'cydc-tree-row' }, item, tools);
                });
                const details = el('details', { class: 'cydc-tree-group', dataset: { group }, open },
                    el('summary', { text: t('cyd_creator.code.group_' + group) }),
                    list.length ? el('ul', {}, items) : el('p', { class: 'cydc-empty', text: t('cyd_creator.code.no_app_files') }));
                return details;
            });
            const openGroups = new Set(Array.from(groups.querySelectorAll('details[open]')).map(d => d.dataset.group));
            groups.replaceChildren(...sections.filter(Boolean));
            if (openGroups.size) groups.querySelectorAll('details').forEach(d => { d.open = openGroups.has(d.dataset.group) || d.dataset.group === 'app'; });
            newBtn.disabled = !canEdit;
            restoreTreeFocus(focusKey);
        }

        // openIsStale: a read-only (generated or template) file whose listed sha moved on.
        function openIsStale() {
            const info = state.files.find(f => f.path === state.openPath);
            return !!(info && !state.saver && state.openSha && info.sha256 && info.sha256 !== state.openSha);
        }

        async function refreshFiles() {
            try {
                // A save that lands while the list loads changes the token: the list may predate it.
                const saver = state.saver;
                const before = saver ? saver.token() : null;
                const files = await inst.api.listFiles(inst.projectId);
                if (state.disposed) return;
                state.files = files;
                renderTree();
                const info = files.find(f => f.path === state.openPath);
                if (state.openPath && !info) {
                    // Deleted or renamed elsewhere (agent, restore, another window). Unsaved text stays
                    // open; saving it ends in the conflict choice: keep editing, drop it or recreate the file.
                    if (state.saver && state.saver.dirty()) {
                        if (state.missing !== state.openPath && state.saver.state() !== 'conflict') {
                            state.missing = state.openPath;
                            inst.notify('error', t('cyd_creator.error.CYD_FILE_CONFLICT'));
                            state.saver.flush();
                        }
                        return;
                    }
                    closeEditor();
                    return;
                }
                if (info) {
                    state.missing = '';
                    state.openInfo = info;
                    // Changed elsewhere and nothing local to keep: reload.
                    if (saver && saver === state.saver) {
                        if (!saver.dirty() && saver.token() === before && info.sha256 && info.sha256 !== before) await reloadOpen();
                    } else if (openIsStale()) {
                        await reloadOpen();
                    }
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
                cm.EditorView.contentAttributes.of({ 'aria-label': t('cyd_creator.code.editor_label') }),
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
                state.textarea = el('textarea', { class: 'cydc-code-fallback', spellcheck: 'false', 'aria-label': t('cyd_creator.code.editor_label'), dataset: { ccCodeFallback: '' } });
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
                    else refreshFiles(); // recreated after it was deleted elsewhere
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
                ],
                owner: b
            });
            const choice = await d.promise;
            if (choice !== 'reload' && choice !== 'overwrite') return { action: 'keep' };
            let server = null;
            try {
                server = await inst.api.readFile(inst.projectId, path);
            } catch (err) {
                if (!err || err.status !== 404) {
                    inst.notify('error', P.errors.message(t, err));
                    return { action: 'keep' };
                }
            }
            if (!server) {
                // Deleted elsewhere: Overwrite recreates it (If-Match * creates only), the saved
                // version is no file at all.
                if (choice === 'overwrite') return { action: 'overwrite', token: '*' };
                if (path === state.openPath) closeEditor();
                refreshFiles();
                return { action: 'reload' };
            }
            if (choice === 'reload' && path === state.openPath) setText(server.text, readOnlyFor(state.openInfo));
            return { action: choice, token: server.sha };
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
            state.openSha = sha || '';
            state.missing = '';
            pathLabel.textContent = path;
            empty.hidden = true;
            editorHost.hidden = false;
            setText(text, readOnlyFor(state.openInfo));
            renderHint();
            renderChip();
            renderTree();
            inst.bus.emit('code:state', state.saver ? state.saver.state() : 'saved');
        }

        function closeEditor() {
            if (state.saver) state.saver.dispose();
            state.saver = null;
            state.openPath = '';
            state.openInfo = null;
            state.openSha = '';
            state.missing = '';
            pathLabel.textContent = '';
            setText('', true);
            empty.hidden = false;
            editorHost.hidden = true;
            renderHint();
            renderChip('');
            renderTree();
            inst.bus.emit('code:state', 'saved');
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
        // A conflict answered with the saved version of a deleted file closes the document: then
        // nothing is left to save.
        function flush() {
            if (!state.saver) return Promise.resolve(true);
            return state.saver.flush().then(ok => ok || !state.saver);
        }

        async function leaveCurrent() {
            if (!state.saver || !state.saver.dirty()) return true;
            if (await flush()) return true;
            return inst.confirm({ titleKey: 'cyd_creator.code.unsaved_title', bodyKey: 'cyd_creator.code.unsaved_body', vars: { path: state.openPath }, okKey: 'cyd_creator.code.discard', danger: true });
        }

        async function openFile(path, line) {
            await ensureEditor();
            if (state.disposed) return false;
            if (path === state.openPath && !openIsStale()) {
                // The open file wins over another file that is still loading.
                state.loadToken++;
                root.classList.remove('is-loading');
                if (line) gotoLine(line);
                return true;
            }
            const token = ++state.loadToken;
            setSwitching(true);
            try {
                if (!(await leaveCurrent()) || token !== state.loadToken || state.disposed) return false;
                root.classList.add('is-loading');
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
                if (!state.disposed) setSwitching(false);
            }
        }

        // ── file actions ──
        // nameError checks a new name against the server's rules; existing names and their folders
        // compare without letter case (the server refuses a second spelling of either), ignoring the
        // renamed file itself.
        function nameError(path, ignorePath) {
            if (!isEditablePath(path)) return t('cyd_creator.code.file_name_invalid');
            const others = state.files.map(f => String(f.path)).filter(p => p !== ignorePath);
            const lower = path.toLowerCase();
            const folders = new Map();
            others.forEach(p => folderPrefixes(p).forEach(d => folders.set(d.toLowerCase(), d)));
            const folderClash = folderPrefixes(path).some(d => folders.has(d.toLowerCase()) && folders.get(d.toLowerCase()) !== d);
            if (folderClash || others.some(p => p.toLowerCase() === lower)) return t('cyd_creator.code.file_exists');
            return '';
        }

        function nameDialog(titleKey, value, ignorePath) {
            const input = el('input', { class: 'cydc-input', type: 'text', value: value || '', autofocus: true, spellcheck: 'false', 'aria-label': t('cyd_creator.code.file_name'), dataset: { ccFileName: '' } });
            const help = el('p', { class: 'cydc-form-help', text: t('cyd_creator.code.file_name_help') });
            const d = P.dialog.open(inst.root, {
                title: t(titleKey),
                body: [el('label', { class: 'cydc-form-field' }, el('span', { text: t('cyd_creator.code.file_name') }), input), help],
                actions: [{ id: 'cancel', label: t('cyd_creator.app.cancel') }, { id: 'ok', label: t('cyd_creator.app.ok'), kind: 'primary' }],
                onAction(id, dialog) {
                    if (id !== 'ok') { dialog.close(null); return; }
                    const path = APP_DIR + input.value.trim().replace(/^src\/app\//, '');
                    const error = nameError(path, ignorePath);
                    if (error) { dialog.setError(error); return; }
                    dialog.close(path);
                },
                owner: b
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

        // renameFile copies and deletes (the API has no rename) and undoes the first step when the
        // second fails. The server refuses a second spelling of an existing name, so a change of
        // letter case goes through a temporary copy: the server holds the text at every step, even
        // if the browser goes away meanwhile. The copy is a .h file, which a build running meanwhile
        // never compiles (no duplicate symbols). It stays, and opens, if neither name can be written.
        async function renameFile(info) {
            const target = await nameDialog('cyd_creator.code.rename', info.path.slice(APP_DIR.length), info.path);
            if (!target || target === info.path) return;
            const id = inst.projectId;
            const wasOpen = state.openPath === info.path;
            let renamed = false;
            let kept = '';
            if (wasOpen) setSwitching(true);
            try {
                if (wasOpen && !(await flush())) return;
                const current = await inst.api.readFile(id, info.path);
                if (target.toLowerCase() === info.path.toLowerCase()) {
                    const tmp = APP_DIR + P.util.uid('cyd_rename') + '.h';
                    const tmpSha = await inst.api.writeFile(id, tmp, current.text, '*');
                    try {
                        await inst.api.deleteFile(id, info.path, current.sha);
                    } catch (err) {
                        await inst.api.deleteFile(id, tmp, tmpSha).catch(() => {});
                        throw err;
                    }
                    try {
                        await inst.api.writeFile(id, target, current.text, '*');
                    } catch (err) {
                        const restored = await inst.api.writeFile(id, info.path, current.text, '*').then(() => true, () => false);
                        if (restored) await inst.api.deleteFile(id, tmp, tmpSha).catch(() => {});
                        else kept = tmp;
                        throw err;
                    }
                    await inst.api.deleteFile(id, tmp, tmpSha).catch(() => {});
                } else {
                    const sha = await inst.api.writeFile(id, target, current.text, '*');
                    try {
                        await inst.api.deleteFile(id, info.path, current.sha);
                    } catch (err) {
                        await inst.api.deleteFile(id, target, sha).catch(() => {});
                        throw err;
                    }
                }
                renamed = true;
                if (state.openPath === info.path) closeEditor();
            } catch (err) {
                inst.notify('error', P.errors.message(t, err));
            } finally {
                if (wasOpen && !state.disposed) setSwitching(false);
            }
            await refreshFiles();
            if (renamed && wasOpen && !state.openPath) await openFile(target, 1);
            else if (kept && !state.openPath) await openFile(kept, 1);
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
            const wasOpen = state.openPath === HOOK_FILE;
            if (wasOpen) setSwitching(true);
            let text = null;
            let sha = '*';
            try {
                if (wasOpen && !(await flush())) return false;
                try {
                    const current = await inst.api.readFile(inst.projectId, HOOK_FILE);
                    text = current.text;
                    sha = current.sha;
                } catch (err) {
                    if (!err || err.status !== 404) { inst.notify('error', P.errors.message(t, err)); return false; }
                }
                if (text === null || !hookLine(text, fn)) {
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
                if (state.disposed) return false;
                if (state.openPath === HOOK_FILE) installDocument(HOOK_FILE, text, sha);
            } finally {
                if (wasOpen && !state.disposed) setSwitching(false);
            }
            if (state.openPath !== HOOK_FILE && !(await openFile(HOOK_FILE))) return false;
            gotoLine(hookLine(text, fn) || 1);
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
                const severity = severityOf(d);
                markers.appendChild(el('div', {
                    class: 'cydc-code-marker is-' + severity,
                    title: (d.line ? d.line + ': ' : '') + String(d.message || ''),
                    dataset: { ccMarker: String(line), severity },
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
            // saveError is the error of the open file's last failed save (null once it saved).
            saveError: () => (state.saver ? state.saver.error() : null),
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
    // Pure rules, for the Node checks.
    P.code.isEditablePath = isEditablePath;
    P.code.hookLine = hookLine;
})();
