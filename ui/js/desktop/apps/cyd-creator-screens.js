// CYD Creator designer screens panel: the screen list (open, add, inline rename, duplicate, delete,
// reorder, start marker, issue count), the template's Wi-Fi setup screen as a locked system entry,
// settings of the active screen (title, transition, background) and the layer list of its widgets.
// The layer list is the keyboard/screen-reader way to reach every widget (roving tab stop).
// Every edit is a designer command (store origin local). Read-only or locked by an agent job (the
// designer's 'lock' signal) every tool, field and layer flag is disabled; screens still open and
// layers still select. Keys: F2 renames and Alt+ArrowUp/Down moves the focused screen; in the layer
// list ArrowUp/Down, Home and End move the focus (they win over nudging), the other designer keys
// (Delete, arrows left/right, undo …) go to interact.handleKey. Enter, Delete, Backspace and F2 never
// leave the panel: the desktop opens, deletes or renames its selected icons from them. Typing in the
// screen settings updates the entry in place: a re-render at blur would drop the focus to the page
// and lose the click that caused the blur; a rename's blur commit waits for a press in the panel to
// end (and its click to run) for the same reason.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const PAGE_KEYS = new Set(['Enter', 'Delete', 'Backspace', 'F2']);
    const HEX = /^#[0-9A-Fa-f]{6}$/;

    // pretty turns a catalog id into an English fallback label ("slide_left" → "Slide left").
    function pretty(id) {
        const s = String(id || '').replace(/_/g, ' ');
        return s.charAt(0).toUpperCase() + s.slice(1);
    }

    function mount(d, host) {
        const { h, dom, icon, swap } = P.render;
        const M = P.model;
        const t = d.t;
        const bag = P.util.bag();
        const SYSTEM = d.SYSTEM_WIFI;
        let renaming = '';
        let draft = '';
        let renameError = '';
        let editing = false;
        let disposed = false;
        let blurTimer = 0;
        let held = false; // a pointer is pressed inside the panel
        let blurAfterRelease = false;
        let shownSelection = null;
        const errorId = P.util.uid('cc_rename_err');
        const root = dom(h('div', { class: 'cydc-screens', tabindex: '-1' }));
        host.appendChild(root);
        const str = v => (typeof v === 'string' ? v : v == null ? '' : String(v));
        const issueCount = count => t.or('cyd_creator.designer.issue_count', 'Issues: {count}', { count });

        const tool = (cmd, name, label, id, disabled) => h('button', { type: 'button', class: 'cydc-tool cydc-tool-sm', 'data-cc-cmd': cmd, 'data-cc-id': id, 'data-cc-focus': cmd + ':' + id, title: label, 'aria-label': label, disabled: !!disabled }, icon(name));
        const field = (label, control) => h('label', { class: 'cydc-field' }, h('span', { class: 'cydc-field-label' }, label), control);
        const renameMessage = code => (code === 'SCREEN_NAME_DUPLICATE'
            ? t.or('cyd_creator.issue.SCREEN_NAME_DUPLICATE', 'Another screen already uses this name.')
            : t.or('cyd_creator.issue.SCREEN_NAME_INVALID', 'Screen names use lowercase letters, digits and _, start with a letter and have at most 32 characters.'));
        // byFocus finds an element by its data-cc-focus key (ids are compared, never put into a selector).
        const byFocus = key => Array.from(root.querySelectorAll('[data-cc-focus]')).find(el => el.getAttribute('data-cc-focus') === key) || null;
        const layerButtons = () => root.querySelectorAll('[data-cc-layer-id]');
        const openKey = id => 'open:' + (id === SYSTEM ? 'system' : id);

        function screenRow(s, i, n, p, ro) {
            const active = s.id === d.screenId;
            // start_screen holds a screen id (internal/cydcreator/project.go).
            const start = p.start_screen === s.id;
            const issues = d.issuesForScreen(s).length;
            const renameLabel = t.or('cyd_creator.designer.screen_rename', 'Rename screen');
            if (renaming === s.id) {
                // The reason is spoken once when it changes (commitRename); re-renders stay silent.
                return h('li', { class: 'cydc-screen-row is-renaming', 'data-cc-screen-id': s.id },
                    h('input', { type: 'text', class: 'cydc-screen-rename', value: draft, maxlength: '32', spellcheck: 'false', autocomplete: 'off', 'data-cc-rename': s.id, 'data-cc-focus': 'rename:' + s.id, 'aria-label': renameLabel, 'aria-invalid': renameError ? 'true' : null, 'aria-describedby': renameError ? errorId : null }),
                    renameError ? h('span', { class: 'cydc-field-error', id: errorId }, renameMessage(renameError)) : null);
            }
            const startLabel = t.or('cyd_creator.designer.screen_start', 'Start screen');
            return h('li', { class: 'cydc-screen-row' + (active ? ' is-active' : ''), 'data-cc-screen-id': s.id },
                h('button', { type: 'button', class: 'cydc-screen-open', 'data-cc-cmd': 'screen-open', 'data-cc-id': s.id, 'data-cc-focus': openKey(s.id), 'aria-current': active ? 'true' : null },
                    start ? h('span', { class: 'cydc-start', title: startLabel, 'aria-label': startLabel }, icon('star')) : null,
                    h('span', { class: 'cydc-screen-name' }, str(s.name)),
                    // Always present (empty without a title): typing a title updates it in place.
                    h('span', { class: 'cydc-screen-title', 'data-cc-title': s.id }, str(s.title)),
                    issues ? h('span', { class: 'cydc-count is-error', title: issueCount(issues) },
                        h('span', { 'aria-hidden': 'true' }, String(issues)), h('span', { class: 'cydc-sr-only' }, issueCount(issues))) : null),
                h('span', { class: 'cydc-row-tools' },
                    tool('screen-start', 'star', t.or('cyd_creator.designer.screen_set_start', 'Make start screen'), s.id, ro || start),
                    tool('screen-rename', 'edit', renameLabel, s.id, ro),
                    tool('screen-duplicate', 'copy', t.or('cyd_creator.designer.screen_duplicate', 'Duplicate screen'), s.id, ro || n >= M.LIMITS.screens),
                    tool('screen-up', 'up', t.or('cyd_creator.designer.screen_up', 'Move screen up'), s.id, ro || i === 0),
                    tool('screen-down', 'down', t.or('cyd_creator.designer.screen_down', 'Move screen down'), s.id, ro || i === n - 1),
                    tool('screen-delete', 'trash', t.or('cyd_creator.designer.screen_delete', 'Delete screen'), s.id, ro || n <= 1)));
        }

        function systemRow() {
            const active = d.isSystem();
            return h('li', { class: 'cydc-screen-row is-system' + (active ? ' is-active' : ''), 'data-cc-screen-id': SYSTEM },
                h('button', { type: 'button', class: 'cydc-screen-open', 'data-cc-cmd': 'screen-open', 'data-cc-id': SYSTEM, 'data-cc-focus': openKey(SYSTEM), 'aria-current': active ? 'true' : null, title: t.or('cyd_creator.designer.system_hint', 'This screen comes from the template and appears while Wi-Fi still needs to be set up. It follows the theme and cannot be edited.') },
                    icon('lock'), h('span', { class: 'cydc-screen-name' }, t.or('cyd_creator.designer.system_wifi', 'Wi-Fi setup')), h('span', { class: 'cydc-badge' }, t.or('cyd_creator.designer.system_badge', 'System'))));
        }

        function screenProps(screen, ro) {
            if (!screen) return null;
            const bg = str(screen.style && screen.style.bg_color);
            const themeBg = str((d.theme().tokens || {}).bg);
            const bgLabel = t.or('cyd_creator.designer.screen_background', 'Background');
            const settingsLabel = t.or('cyd_creator.designer.screen_settings', 'Screen settings');
            return h('section', { class: 'cydc-screen-props', 'aria-label': settingsLabel },
                h('h3', {}, settingsLabel),
                h('fieldset', { class: 'cydc-fieldset', disabled: ro },
                    field(t.or('cyd_creator.designer.screen_title', 'Title'), h('input', { type: 'text', maxlength: '64', value: str(screen.title), 'data-cc-field': 'title', 'data-cc-focus': 'screen-title' })),
                    field(t.or('cyd_creator.designer.screen_transition', 'Transition'), h('select', { 'data-cc-field': 'transition', 'data-cc-focus': 'screen-transition' },
                        M.TRANSITIONS.map(x => h('option', { value: x, selected: (screen.transition || 'none') === x }, t.or('cyd_creator.transition.' + x, pretty(x)))))),
                    h('div', { class: 'cydc-field' }, h('span', { class: 'cydc-field-label' }, bgLabel),
                        h('div', { class: 'cydc-color-row' },
                            // A colour input shows only #rrggbb: an invalid stored value shows the theme's.
                            h('input', { type: 'color', value: (HEX.test(bg) ? bg : HEX.test(themeBg) ? themeBg : '#000000').toLowerCase(), 'data-cc-field': 'bg', 'data-cc-focus': 'screen-bg', 'aria-label': bgLabel }),
                            h('button', { type: 'button', class: 'cydc-chip', 'data-cc-cmd': 'screen-bg-reset', 'aria-pressed': bg ? 'false' : 'true', 'data-cc-focus': 'screen-bg-reset', disabled: ro }, t.or('cyd_creator.designer.use_theme', 'Use theme'))))));
        }

        function layerRows(screen, ro) {
            const rows = [];
            const selected = new Set(d.selection);
            (function visit(list, depth, parent) {
                list.slice().reverse().forEach(w => {
                    if (!w || typeof w !== 'object') return;
                    const name = str(w.name) || d.label(w.type);
                    const issues = d.issuesFor(w);
                    const state = [w.locked ? t.or('cyd_creator.designer.layer_locked', 'locked') : '', w.hidden ? t.or('cyd_creator.designer.layer_hidden', 'hidden') : '',
                        parent && parent.type === 'tabview' ? t.or('cyd_creator.designer.layer_tab', 'tab {n}', { n: (Number(w.tab) || 0) + 1 }) : '', issues.length ? issueCount(issues.length) : ''].filter(Boolean).join(', ');
                    const lockLabel = w.locked ? t.or('cyd_creator.designer.unlock', 'Unlock') : t.or('cyd_creator.designer.lock', 'Lock');
                    const hideLabel = w.hidden ? t.or('cyd_creator.designer.show', 'Show') : t.or('cyd_creator.designer.hide', 'Hide');
                    // aria-level carries the nesting the indentation shows.
                    rows.push(h('li', { class: 'cydc-layer' + (selected.has(w.id) ? ' is-selected' : '') + (w.hidden ? ' is-hidden' : ''), 'aria-level': String(depth + 1), style: { '--cc-depth': String(depth) } },
                        h('button', { type: 'button', class: 'cydc-layer-select', 'data-cc-layer-id': w.id, 'data-cc-focus': 'layer:' + w.id, tabindex: '-1', 'aria-pressed': selected.has(w.id) ? 'true' : 'false', 'aria-label': [name, d.label(w.type), state].filter(Boolean).join(', ') },
                            h('span', { class: 'cydc-layer-glyph', 'aria-hidden': 'true' }, P.designerPalette ? (P.designerPalette.GLYPHS[w.type] || '◆') : '◆'),
                            h('span', { class: 'cydc-layer-name' }, name),
                            h('span', { class: 'cydc-layer-type' }, d.label(w.type)),
                            issues.length ? h('span', { class: 'cydc-issue-mark ' + (issues.some(i => i.severity === 'error') ? 'is-error' : 'is-warning'), title: issues.map(i => d.issueText(i)).join('\n') }, '!') : null),
                        h('button', { type: 'button', class: 'cydc-layer-flag', 'data-cc-cmd': 'layer-lock', 'data-cc-id': w.id, 'data-cc-focus': 'layer-lock:' + w.id, tabindex: '-1', disabled: ro, 'aria-pressed': w.locked ? 'true' : 'false', title: lockLabel, 'aria-label': lockLabel }, icon(w.locked ? 'lock' : 'unlock')),
                        h('button', { type: 'button', class: 'cydc-layer-flag', 'data-cc-cmd': 'layer-hide', 'data-cc-id': w.id, 'data-cc-focus': 'layer-hide:' + w.id, tabindex: '-1', disabled: ro, 'aria-pressed': w.hidden ? 'true' : 'false', title: hideLabel, 'aria-label': hideLabel }, icon(w.hidden ? 'eye-off' : 'eye'))));
                    if (Array.isArray(w.children) && w.children.length) visit(w.children, depth + 1, w);
                });
            })(Array.isArray(screen.widgets) ? screen.widgets : [], 0, null);
            return rows;
        }

        function layers(ro) {
            const screen = d.isSystem() ? null : d.screen();
            const rows = screen ? layerRows(screen, ro) : [];
            const title = t.or('cyd_creator.designer.layers_title', 'Layers');
            return h('section', { class: 'cydc-layers', 'aria-label': title },
                h('div', { class: 'cydc-panel-head' }, h('h3', {}, title)),
                rows.length ? h('ul', { class: 'cydc-layer-list', role: 'list' }, rows) : h('p', { class: 'cydc-panel-empty' }, d.isSystem()
                    ? t.or('cyd_creator.designer.system_hint', 'This screen comes from the template and appears while Wi-Fi still needs to be set up. It follows the theme and cannot be edited.')
                    : t.or('cyd_creator.designer.layers_empty', 'No widgets on this screen yet.')));
        }

        // tabStop is the layer that takes Tab: the focused one, else the first selected, else the first.
        function tabStop() {
            const all = Array.from(layerButtons());
            const active = document.activeElement;
            return all.find(b => b === active) || all.find(b => b.getAttribute('aria-pressed') === 'true') || all[0] || null;
        }
        // keepFocus: a focused control that is gone or disabled after a re-render hands the focus to the
        // layer tab stop (for a layer) or its screen's entry, then the active screen's entry, then the
        // panel — never to the page (the desktop acts on its selected icons from the page's keys).
        function keepFocus(key) {
            const ro = d.readonly();
            const usable = el => !!el && el !== root && root.contains(el) && !el.disabled &&
                (!ro || /^(open|layer):/.test(el.getAttribute('data-cc-focus') || ''));
            if (usable(document.activeElement)) return;
            const k = String(key || '');
            const id = k.slice(k.indexOf(':') + 1);
            const next = [k.startsWith('layer') ? tabStop() : null, byFocus(openKey(id)), byFocus(openKey(d.screenId))].find(usable);
            (next || root).focus({ preventScroll: true });
        }

        function render() {
            if (editing || disposed) return;
            const p = d.project();
            const ro = d.readonly();
            const screens = Array.isArray(p.screens) ? p.screens : [];
            // A rename ends when its screen is gone or editing stops being allowed.
            if (renaming && (ro || !screens.some(s => s.id === renaming))) { renaming = ''; renameError = ''; }
            const wifiOn = !!(p.settings && p.settings.wifi !== 'off');
            const active = document.activeElement;
            const hadFocus = !!active && active !== root && root.contains(active);
            const key = hadFocus ? active.getAttribute('data-cc-focus') || '' : '';
            const addLabel = t.or('cyd_creator.designer.screen_add', 'Add screen');
            const title = t.or('cyd_creator.designer.screens_title', 'Screens');
            swap(root, [
                h('section', { class: 'cydc-screen-section', 'aria-label': title },
                    h('div', { class: 'cydc-panel-head' }, h('h3', {}, title),
                        h('button', { type: 'button', class: 'cydc-tool', 'data-cc-cmd': 'screen-add', 'data-cc-focus': 'screen-add', disabled: ro || screens.length >= M.LIMITS.screens, title: addLabel, 'aria-label': addLabel }, icon('plus'))),
                    h('ul', { class: 'cydc-screen-list', role: 'list' }, screens.map((s, i) => screenRow(s, i, screens.length, p, ro)), wifiOn ? systemRow() : null)),
                d.isSystem() ? null : screenProps(d.screen(), ro),
                layers(ro)
            ]);
            shownSelection = d.selection.join(' ');
            root.classList.toggle('is-readonly', ro);
            // Roving tab stop: the focused layer, else the first selected layer, else the first layer.
            const stop = tabStop();
            if (stop) stop.setAttribute('tabindex', '0');
            if (hadFocus) keepFocus(key);
        }

        // ── actions ─────────────────────────────────────────────────────────
        function addScreen() {
            const res = d.commands.run('add-screen', [{ op: 'add_screen', screen_data: { name: M.proposeScreenName(d.project(), 'screen'), title: '' } }]);
            if (!res) return;
            d.setScreen(res[0].id);
            d.announce(t.or('cyd_creator.designer.screen_added', 'Screen added'));
        }
        // duplicateScreen adds a copy (fresh ids and names) right after the original in one undo step.
        function duplicateScreen(id) {
            const p = d.project();
            const f = M.findScreen(p, id);
            if (!f) return;
            const names = M.usedNames(p);
            const data = {
                name: M.proposeScreenName(p, f.screen.name), title: str(f.screen.title), transition: f.screen.transition || 'none',
                style: P.util.deepClone(f.screen.style || {}), widgets: (f.screen.widgets || []).map(w => M.cloneSubtree(p, w, names))
            };
            const copyId = d.commands.transact('duplicate-screen', (doc, apply) => {
                const added = apply([{ op: 'add_screen', screen_data: data }])[0].id;
                apply([{ op: 'reorder_screen', id: added, index: f.index + 1 }]);
                return added;
            });
            if (copyId) d.setScreen(copyId);
        }
        // deleteScreen asks first (the shell's dialog); the screen must still exist and not be the last.
        // delete_screen moves the start screen to the first remaining one.
        async function deleteScreen(id) {
            const f = M.findScreen(d.project(), id);
            if (!f || d.readonly() || typeof d.inst.confirm !== 'function') return;
            const ok = await d.inst.confirm({ titleKey: 'cyd_creator.designer.screen_delete_title', bodyKey: 'cyd_creator.designer.screen_delete_body', vars: { name: str(f.screen.name) }, okKey: 'cyd_creator.designer.delete', danger: true });
            if (disposed || d.disposed) return;
            const screens = d.project().screens || [];
            if (ok && screens.length > 1 && screens.some(s => s.id === f.screen.id)) d.commands.run('delete-screen', [{ op: 'delete_screen', id: f.screen.id }]);
            keepFocus('screen-delete:' + f.screen.id);
        }
        // startRename opens the name field of a screen. A rename of another screen still open (its
        // blur commit waits for the click that got here) is committed first; when its name is
        // refused, its field stays with the reason and this one does not start.
        function startRename(id) {
            const f = M.findScreen(d.project(), id);
            if (!f || d.readonly()) return;
            if (renaming && renaming !== f.screen.id) { commitRename(false, true); if (renaming) return; }
            renaming = f.screen.id; draft = str(f.screen.name); renameError = '';
            render();
            const input = byFocus('rename:' + renaming);
            if (input) { input.focus(); input.select(); }
        }
        // commitRename stores a valid new name (Enter, blur) or drops the entry (cancel, Escape). An
        // invalid name keeps the field open with the reason. From the keyboard the focus goes back to
        // the screen's entry; a blur leaves it where the user put it.
        function commitRename(cancel, blur) {
            if (!renaming) return;
            const id = renaming;
            const f = M.findScreen(d.project(), id);
            const name = draft.trim();
            const changed = !cancel && !!f && f.screen.id === id && name !== f.screen.name;
            if (changed) {
                const issue = M.screenNameIssue(d.project(), name, id);
                if (issue) {
                    if (issue !== renameError) d.announce(renameMessage(issue));
                    renameError = issue;
                    render();
                    const input = byFocus('rename:' + id);
                    if (input && !blur) input.focus();
                    return;
                }
            }
            renaming = ''; renameError = '';
            if (!changed || !d.commands.run('rename-screen', [{ op: 'update_screen', id, set: { name } }])) render();
            if (!blur) { const btn = byFocus(openKey(id)); if (btn) btn.focus({ preventScroll: true }); else keepFocus(''); }
        }
        // editScreen changes the active screen; typing=true keeps the panel as it is (the caret stays)
        // and updates the entry's title and the "Use theme" chip in place.
        function editScreen(set, coalesce, typing) {
            const screen = d.screen();
            if (!screen) return;
            editing = !!typing;
            try { d.commands.run('screen', [{ op: 'update_screen', id: screen.id, set }], coalesce); } finally { editing = false; }
            if (typing) syncEntry();
        }
        function syncEntry() {
            const screen = d.screen();
            if (!screen) return;
            const title = Array.from(root.querySelectorAll('[data-cc-title]')).find(el => el.getAttribute('data-cc-title') === screen.id);
            if (title) title.replaceChildren(document.createTextNode(str(screen.title)));
            const chip = byFocus('screen-bg-reset');
            if (chip) chip.setAttribute('aria-pressed', screen.style && screen.style.bg_color ? 'false' : 'true');
        }
        // showTabs opens the tab pages on a widget's path (as the designer's reveal does), so a layer
        // picked from the list is visible on the canvas.
        function showTabs(id) {
            const f = d.find(id);
            if (!f) return;
            const chain = f.ancestors.concat([f.widget]);
            chain.forEach((a, i) => {
                const tab = chain[i + 1] ? Number(chain[i + 1].tab) || 0 : 0;
                if (a.type === 'tabview' && chain[i + 1] && d.activeTab(a.id) !== tab) d.setTab(a.id, tab);
            });
        }
        // blurCommit commits a rename whose field lost the focus, unless the focus is back in it (a
        // re-render made a new one). A press inside the panel (another entry, a tool, a layer) gets its
        // click first: re-rendering between press and release would lose that click.
        function blurCommit() {
            const now = document.activeElement;
            if (disposed || !renaming || (now && now.dataset && now.dataset.ccRename === renaming && root.contains(now))) return;
            if (held) { blurAfterRelease = true; return; }
            commitRename(false, true);
        }
        function release() {
            held = false;
            if (!blurAfterRelease) return;
            blurAfterRelease = false;
            clearTimeout(blurTimer);
            blurTimer = setTimeout(blurCommit, 0);
        }
        function moveScreen(id, step) {
            const f = M.findScreen(d.project(), id);
            if (f) d.commands.run('reorder-screen', [{ op: 'reorder_screen', id: f.screen.id, index: f.index + step }]);
        }
        function moveFocus(from, step) {
            const all = Array.from(layerButtons());
            const i = all.indexOf(from);
            const next = step === 'home' ? all[0] : step === 'end' ? all[all.length - 1] : all[i + step];
            if (!next) return;
            all.forEach(b => b.setAttribute('tabindex', '-1'));
            next.setAttribute('tabindex', '0');
            next.focus();
        }

        bag.on(root, 'click', event => {
            const target = event.target;
            if (!target || typeof target.closest !== 'function') return;
            const layer = target.closest('[data-cc-layer-id]');
            if (layer) {
                const id = layer.dataset.ccLayerId;
                d.select([id], { toggle: event.shiftKey || event.ctrlKey || event.metaKey });
                if (d.selection.includes(id)) showTabs(id);
                return;
            }
            const b = target.closest('button[data-cc-cmd]');
            if (!b || b.disabled) return;
            const id = b.dataset.ccId;
            const cmd = b.dataset.ccCmd;
            if (cmd === 'screen-open') d.setScreen(id);
            else if (cmd === 'screen-add') addScreen();
            else if (cmd === 'screen-start') d.commands.run('start-screen', [{ op: 'set_project', set: { start_screen: id } }]);
            else if (cmd === 'screen-rename') startRename(id);
            else if (cmd === 'screen-duplicate') duplicateScreen(id);
            else if (cmd === 'screen-up' || cmd === 'screen-down') moveScreen(id, cmd === 'screen-up' ? -1 : 1);
            else if (cmd === 'screen-delete') deleteScreen(id);
            else if (cmd === 'screen-bg-reset') editScreen({ style: { bg_color: null } });
            else if (cmd === 'layer-lock' || cmd === 'layer-hide') {
                const f = d.find(id);
                const key = cmd === 'layer-lock' ? 'locked' : 'hidden';
                if (f) d.commands.update(id, { [key]: !f.widget[key] });
            }
        });
        bag.on(root, 'input', event => {
            const el = event.target;
            if (!el || !el.dataset) return;
            if (el.dataset.ccRename !== undefined) { draft = str(el.value); return; }
            if (el.dataset.ccField === 'title') editScreen({ title: str(el.value) }, 'screen-title:' + d.screenId, true);
            if (el.dataset.ccField === 'bg' && HEX.test(el.value)) editScreen({ style: { bg_color: el.value.toUpperCase() } }, 'screen-bg:' + d.screenId, true);
        });
        // A select changes while it has the focus (the re-render keeps it). Title and colour edits are
        // stored on input; their change event (at blur) re-renders nothing.
        bag.on(root, 'change', event => {
            const el = event.target;
            if (!el || !el.dataset) return;
            if (el.dataset.ccField === 'transition' && M.TRANSITIONS.includes(el.value)) editScreen({ transition: el.value });
        });
        bag.on(root, 'keydown', event => {
            const el = event.target;
            const ix = d.interact;
            if (!el || typeof event.key !== 'string') return;
            // During a gesture (a palette drag) the keys go to it: Escape cancels it.
            if (ix && ix.gestureKey(event)) { event.preventDefault(); event.stopPropagation(); return; }
            if (PAGE_KEYS.has(event.key)) event.stopPropagation();
            const plain = !event.ctrlKey && !event.metaKey && !event.altKey;
            if (el.dataset && el.dataset.ccRename !== undefined) {
                // Enter and Escape during an IME composition belong to the composition.
                if (event.isComposing || event.keyCode === 229) return;
                if (event.key === 'Enter') { event.preventDefault(); commitRename(false, false); }
                if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); commitRename(true, false); }
                return;
            }
            const open = typeof el.closest === 'function' ? el.closest('[data-cc-cmd="screen-open"]') : null;
            if (open && open.dataset.ccId !== SYSTEM) {
                if (event.key === 'F2' && plain && !event.shiftKey) { event.preventDefault(); startRename(open.dataset.ccId); return; }
                // Alt+ArrowUp/Down moves the screen; Ctrl+Alt+arrows stay with the desktop (workspaces).
                if (event.altKey && !event.ctrlKey && !event.metaKey && !event.shiftKey && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
                    event.preventDefault();
                    event.stopPropagation();
                    moveScreen(open.dataset.ccId, event.key === 'ArrowUp' ? -1 : 1);
                    return;
                }
            }
            if (el.dataset && el.dataset.ccLayerId !== undefined) {
                // ArrowUp/Down, Home and End move through the layer list (they win over nudging).
                const step = plain ? { ArrowUp: -1, ArrowDown: 1, Home: 'home', End: 'end' }[event.key] : undefined;
                if (step !== undefined) { event.preventDefault(); moveFocus(el, step); return; }
                if (ix && ix.handleKey(event)) { event.preventDefault(); event.stopPropagation(); }
            }
        });
        // Blur commits a rename (blurCommit); a press anywhere tells whether it started in the panel.
        bag.on(root, 'focusout', event => {
            const el = event.target;
            if (!renaming || !el || !el.dataset || el.dataset.ccRename === undefined) return;
            clearTimeout(blurTimer);
            blurTimer = setTimeout(blurCommit, 0);
        });
        bag.on(window, 'pointerdown', event => { held = !!event.target && root.contains(event.target); }, true);
        bag.on(window, 'pointerup', release, true);
        bag.on(window, 'pointercancel', release, true);
        // A release outside the browser may never arrive: leaving the window ends the press too.
        bag.on(window, 'blur', release);
        bag.add(() => clearTimeout(blurTimer));
        // A store change that switched the screen rendered on 'screen' already; a selection already
        // shown (the 'selection' after 'screen') renders nothing.
        bag.add(d.bus.on('project', info => { if (!(info && info.screenChanged)) render(); }));
        bag.add(d.bus.on('selection', () => { if (d.selection.join(' ') !== shownSelection) render(); }));
        ['screen', 'issues', 'lock'].forEach(type => bag.add(d.bus.on(type, render)));
        render();
        return { root, render, dispose() { disposed = true; bag.dispose(); root.remove(); } };
    }

    P.designerScreens = { mount };
})();
